package api

import (
	"errors"
	"net/http"
	"time"

	"github.com/Eshpelin/calltoverify/coordinator/internal/store"
	"github.com/Eshpelin/calltoverify/coordinator/internal/verify"
)

type startVerificationReq struct {
	Channel       string `json:"channel"`
	BindingMode   string `json:"binding_mode"`
	ClaimedMSISDN string `json:"claimed_msisdn"`
}

func (s *Server) handleStartVerification(w http.ResponseWriter, r *http.Request) {
	app := appFromCtx(r)
	var req startVerificationReq
	if !decodeJSON(w, r, &req) {
		return
	}

	res, err := s.svc.Start(r.Context(), app, verify.StartRequest{
		Channel:       req.Channel,
		BindingMode:   req.BindingMode,
		ClaimedMSISDN: req.ClaimedMSISDN,
	})
	if err != nil {
		var ve *verify.ValidationError
		var be *verify.BusyError
		switch {
		case errors.As(err, &ve):
			writeErr(w, http.StatusBadRequest, "bad_request", ve.Error())
		case errors.As(err, &be):
			writeJSON(w, http.StatusTooManyRequests, map[string]any{"error": "busy", "detail": be.Error(), "position": be.Position})
		case errors.Is(err, verify.ErrNoCapacity):
			writeErr(w, http.StatusServiceUnavailable, "no_capacity", "no available number for this channel; try again shortly")
		default:
			s.logger.Error("start verification", "err", err)
			writeErr(w, http.StatusInternalServerError, "internal", "could not start verification")
		}
		return
	}
	writeJSON(w, http.StatusCreated, res)
}

type verificationStatusResp struct {
	SessionID      string    `json:"session_id"`
	Status         string    `json:"status"`
	Channel        string    `json:"channel"`
	VerifiedMSISDN string    `json:"verified_msisdn,omitempty"`
	ExpiresAt      time.Time `json:"expires_at"`
}

func (s *Server) handleGetVerification(w http.ResponseWriter, r *http.Request) {
	app := appFromCtx(r)
	id := r.PathValue("id")

	sess, err := s.store.GetSession(r.Context(), app.ID, id)
	if errors.Is(err, store.ErrNotFound) {
		writeErr(w, http.StatusNotFound, "not_found", "no such verification")
		return
	}
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "internal", "lookup failed")
		return
	}

	resp := verificationStatusResp{
		SessionID: sess.ID,
		Status:    sess.Status,
		Channel:   sess.Channel,
		ExpiresAt: sess.ExpiresAt,
	}
	if sess.VerifiedMSISDN != nil {
		resp.VerifiedMSISDN = *sess.VerifiedMSISDN
	}
	writeJSON(w, http.StatusOK, resp)
}
