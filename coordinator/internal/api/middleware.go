package api

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/Eshpelin/calltoverify/coordinator/internal/auth"
	"github.com/Eshpelin/calltoverify/coordinator/internal/store"
)

const maxBodyBytes = 1 << 20 // 1 MiB

type ctxKey int

const (
	ctxApp ctxKey = iota
	ctxDevice
	ctxBody
)

// --- admin auth ---

func (s *Server) adminAuth(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if s.cfg.AdminToken == "" {
			writeErr(w, http.StatusNotFound, "not_found", "admin API is disabled (set CTV_ADMIN_TOKEN)")
			return
		}
		if !auth.ConstantTimeEqual(bearer(r), s.cfg.AdminToken) {
			writeErr(w, http.StatusUnauthorized, "unauthorized", "invalid admin token")
			return
		}
		next(w, r)
	}
}

// --- developer auth (bearer API key) ---

func (s *Server) devAuth(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		key := bearer(r)
		if key == "" {
			writeErr(w, http.StatusUnauthorized, "unauthorized", "missing bearer API key")
			return
		}
		app, err := s.store.GetAppByAPIKeyHash(r.Context(), auth.HashAPIKey(key))
		if errors.Is(err, store.ErrNotFound) {
			writeErr(w, http.StatusUnauthorized, "unauthorized", "invalid API key")
			return
		}
		if err != nil {
			writeErr(w, http.StatusInternalServerError, "internal", "auth lookup failed")
			return
		}
		next(w, r.WithContext(context.WithValue(r.Context(), ctxApp, app)))
	}
}

// --- device auth (HMAC + timestamp + nonce) ---

func (s *Server) deviceAuth(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := r.Header.Get("X-CTV-Device-Id")
		ts := r.Header.Get("X-CTV-Timestamp")
		nonce := r.Header.Get("X-CTV-Nonce")
		sig := r.Header.Get("X-CTV-Signature")
		if id == "" || ts == "" || nonce == "" || sig == "" {
			writeErr(w, http.StatusUnauthorized, "unauthorized", "missing device auth headers")
			return
		}

		tsi, err := strconv.ParseInt(ts, 10, 64)
		if err != nil {
			writeErr(w, http.StatusUnauthorized, "unauthorized", "bad timestamp")
			return
		}
		now := time.Now()
		if d := now.Unix() - tsi; d > 300 || d < -300 {
			writeErr(w, http.StatusUnauthorized, "unauthorized", "timestamp outside allowed skew")
			return
		}

		body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, maxBodyBytes))
		if err != nil {
			writeErr(w, http.StatusRequestEntityTooLarge, "too_large", "request body too large")
			return
		}

		device, err := s.store.GetDeviceByID(r.Context(), id)
		if errors.Is(err, store.ErrNotFound) {
			writeErr(w, http.StatusUnauthorized, "unauthorized", "unknown device")
			return
		}
		if err != nil {
			writeErr(w, http.StatusInternalServerError, "internal", "auth lookup failed")
			return
		}

		expected := auth.DeviceSignature(device.DeviceSecret, ts, nonce, body)
		if !auth.ConstantTimeEqual(sig, expected) {
			writeErr(w, http.StatusUnauthorized, "unauthorized", "bad signature")
			return
		}
		if s.nonces.Seen(nonce, now) {
			writeErr(w, http.StatusUnauthorized, "unauthorized", "nonce replay")
			return
		}

		ctx := context.WithValue(r.Context(), ctxDevice, device)
		ctx = context.WithValue(ctx, ctxBody, body)
		next(w, r.WithContext(ctx))
	}
}

// --- helpers ---

func bearer(r *http.Request) string {
	h := r.Header.Get("Authorization")
	if strings.HasPrefix(h, "Bearer ") {
		return strings.TrimSpace(strings.TrimPrefix(h, "Bearer "))
	}
	return ""
}

func appFromCtx(r *http.Request) store.App       { return r.Context().Value(ctxApp).(store.App) }
func deviceFromCtx(r *http.Request) store.Device { return r.Context().Value(ctxDevice).(store.Device) }
func bodyFromCtx(r *http.Request) []byte         { return r.Context().Value(ctxBody).([]byte) }

// decodeJSON reads and decodes a (size-limited) JSON body for non-device routes.
func decodeJSON(w http.ResponseWriter, r *http.Request, dst any) bool {
	body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, maxBodyBytes))
	if err != nil {
		writeErr(w, http.StatusRequestEntityTooLarge, "too_large", "request body too large")
		return false
	}
	if len(body) == 0 {
		return true // allow empty body; defaults apply
	}
	if err := json.Unmarshal(body, dst); err != nil {
		writeErr(w, http.StatusBadRequest, "bad_request", "invalid JSON body")
		return false
	}
	return true
}

func writeJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}

func writeErr(w http.ResponseWriter, status int, code, detail string) {
	writeJSON(w, status, map[string]string{"error": code, "detail": detail})
}
