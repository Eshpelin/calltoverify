// Package deviceapi holds the device-facing HTTP surface (register, heartbeat,
// inbound) and its HMAC + timestamp + nonce authentication. Both the standalone
// Coordinator and the embedded engine mount these routes.
package deviceapi

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net"
	"net/http"
	"strconv"
	"time"

	"github.com/Eshpelin/calltoverify/coordinator/internal/auth"
	"github.com/Eshpelin/calltoverify/coordinator/internal/httpx"
	"github.com/Eshpelin/calltoverify/coordinator/internal/ratelimit"
	"github.com/Eshpelin/calltoverify/coordinator/internal/store"
	"github.com/Eshpelin/calltoverify/coordinator/internal/verify"
)

const maxBodyBytes = httpx.MaxBodyBytes

type ctxKey int

const (
	ctxDevice ctxKey = iota
	ctxBody
)

// NonceStore rejects replayed device nonces. Implemented by auth.NonceCache
// (in-process) and auth.RedisNonceCache (shared across instances).
type NonceStore interface {
	Seen(nonce string, now time.Time) bool
}

type Handler struct {
	store   store.Store
	svc     *verify.Service
	nonces  NonceStore
	logger  *slog.Logger
	preauth *ratelimit.Limiter // bounds unauthenticated work (body read + device lookup) per source IP
}

func New(st store.Store, svc *verify.Service, nonces NonceStore, logger *slog.Logger) *Handler {
	// A generous per-IP budget: legitimate receivers heartbeat ~1/min, so this is
	// pure abuse protection — it caps how fast one source can force pre-signature
	// body reads + device lookups, without affecting real traffic. Per instance.
	return &Handler{store: st, svc: svc, nonces: nonces, logger: logger, preauth: ratelimit.New(600, 100)}
}

// clientIP is the source address used to key the pre-auth limiter. It uses the
// transport peer (RemoteAddr), not a spoofable X-Forwarded-For header.
func clientIP(r *http.Request) string {
	if host, _, err := net.SplitHostPort(r.RemoteAddr); err == nil {
		return host
	}
	return r.RemoteAddr
}

// Mux returns the device routes under relative paths, ready to be mounted with a
// StripPrefix at whatever base the host chooses.
func (h *Handler) Mux() *http.ServeMux {
	m := http.NewServeMux()
	m.HandleFunc("POST /devices/register", h.Auth(h.Register))
	m.HandleFunc("POST /devices/heartbeat", h.Auth(h.Heartbeat))
	m.HandleFunc("POST /inbound", h.Auth(h.Inbound))
	return m
}

// Auth verifies the device HMAC signature, timestamp skew, and nonce, then stashes
// the device and raw body in the request context.
func (h *Handler) Auth(next http.HandlerFunc) http.HandlerFunc {
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
		// Throttle before the (still unauthenticated) body read + device lookup so a
		// single source cannot drive that work without a valid signature.
		if !h.preauth.Allow("devauth:" + clientIP(r)) {
			writeErr(w, http.StatusTooManyRequests, "rate_limited", "too many requests")
			return
		}
		body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, maxBodyBytes))
		if err != nil {
			writeErr(w, http.StatusRequestEntityTooLarge, "too_large", "request body too large")
			return
		}
		// A single generic error for unknown-device and bad-signature: distinct
		// messages would let an unauthenticated caller enumerate valid device ids.
		device, err := h.store.GetDeviceByID(r.Context(), id)
		if errors.Is(err, store.ErrNotFound) {
			writeErr(w, http.StatusUnauthorized, "unauthorized", "invalid device credentials")
			return
		}
		if err != nil {
			writeErr(w, http.StatusInternalServerError, "internal", "auth lookup failed")
			return
		}
		if !auth.ConstantTimeEqual(sig, auth.DeviceSignature(device.DeviceSecret, ts, nonce, body)) {
			writeErr(w, http.StatusUnauthorized, "unauthorized", "invalid device credentials")
			return
		}
		if h.nonces.Seen(nonce, now) {
			writeErr(w, http.StatusUnauthorized, "unauthorized", "nonce replay")
			return
		}
		ctx := context.WithValue(r.Context(), ctxDevice, device)
		ctx = context.WithValue(ctx, ctxBody, body)
		next(w, r.WithContext(ctx))
	}
}

type numberView struct {
	MSISDN   string   `json:"msisdn"`
	Channels []string `json:"channels"`
	Status   string   `json:"status"`
}

func (h *Handler) numbers(r *http.Request, deviceID string) ([]numberView, error) {
	nums, err := h.store.ListNumbersByDevice(r.Context(), deviceID)
	if err != nil {
		return nil, err
	}
	views := make([]numberView, 0, len(nums))
	for _, n := range nums {
		views = append(views, numberView{MSISDN: n.MSISDN, Channels: n.Channels, Status: n.Status})
	}
	return views, nil
}

func (h *Handler) Register(w http.ResponseWriter, r *http.Request) {
	device := deviceFromCtx(r)
	if err := h.store.SetHeartbeat(r.Context(), device.ID); err != nil {
		writeErr(w, http.StatusInternalServerError, "internal", "register failed")
		return
	}
	numbers, err := h.numbers(r, device.ID)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "internal", "could not list numbers")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"device_id":    device.ID,
		"type":         device.Type,
		"capabilities": device.Capabilities,
		"numbers":      numbers,
	})
}

func (h *Handler) Heartbeat(w http.ResponseWriter, r *http.Request) {
	device := deviceFromCtx(r)
	if err := h.store.SetHeartbeat(r.Context(), device.ID); err != nil {
		writeErr(w, http.StatusInternalServerError, "internal", "heartbeat failed")
		return
	}
	numbers, err := h.numbers(r, device.ID)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "internal", "could not list numbers")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "numbers": numbers})
}

type inboundReq struct {
	Number string `json:"number"`
	Type   string `json:"type"`
	Sender string `json:"sender"`
	Body   string `json:"body"`
}

func (h *Handler) Inbound(w http.ResponseWriter, r *http.Request) {
	device := deviceFromCtx(r)
	var req inboundReq
	if body := bodyFromCtx(r); len(body) > 0 {
		if err := json.Unmarshal(body, &req); err != nil {
			writeErr(w, http.StatusBadRequest, "bad_request", "invalid JSON body")
			return
		}
	}
	if req.Number == "" || req.Sender == "" {
		writeErr(w, http.StatusBadRequest, "bad_request", "number and sender are required")
		return
	}
	res, err := h.svc.Inbound(r.Context(), device, verify.InboundRequest{
		Number: req.Number, Type: req.Type, Sender: req.Sender, Body: req.Body,
	})
	if err != nil {
		var ve *verify.ValidationError
		switch {
		case errors.As(err, &ve):
			writeErr(w, http.StatusBadRequest, "bad_request", ve.Error())
		case errors.Is(err, verify.ErrForbidden):
			writeErr(w, http.StatusForbidden, "forbidden", "device does not own this number")
		default:
			h.logger.Error("inbound", "err", err)
			writeErr(w, http.StatusInternalServerError, "internal", "inbound processing failed")
		}
		return
	}
	writeJSON(w, http.StatusOK, res)
}

func deviceFromCtx(r *http.Request) store.Device { return r.Context().Value(ctxDevice).(store.Device) }
func bodyFromCtx(r *http.Request) []byte         { return r.Context().Value(ctxBody).([]byte) }

// writeJSON and writeErr forward to the shared httpx helpers so the JSON writer
// and the {"error","detail"} envelope live in one place.
func writeJSON(w http.ResponseWriter, status int, body any) { httpx.WriteJSON(w, status, body) }

func writeErr(w http.ResponseWriter, status int, code, detail string) {
	httpx.WriteErr(w, status, code, detail)
}
