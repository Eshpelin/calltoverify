// Package api wires the Coordinator's HTTP surface across three audiences:
//   - /admin/*    provisioning (apps, devices, numbers), guarded by an admin token
//   - /v1/verif*  developer API (bearer API key)
//   - /v1/devices/*, /v1/inbound  device API (HMAC-signed)
package api

import (
	"log/slog"
	"net/http"
	"time"

	"github.com/Eshpelin/calltoverify/coordinator/internal/auth"
	"github.com/Eshpelin/calltoverify/coordinator/internal/config"
	"github.com/Eshpelin/calltoverify/coordinator/internal/store"
	"github.com/Eshpelin/calltoverify/coordinator/internal/verify"
)

type Server struct {
	logger *slog.Logger
	cfg    config.Config
	store  *store.Store
	svc    *verify.Service
	nonces *auth.NonceCache
}

func NewServer(logger *slog.Logger, cfg config.Config, st *store.Store, svc *verify.Service, nonces *auth.NonceCache) *Server {
	return &Server{logger: logger, cfg: cfg, store: st, svc: svc, nonces: nonces}
}

// Routes returns the fully wired HTTP handler.
func (s *Server) Routes() http.Handler {
	mux := http.NewServeMux()

	// Operational.
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	})
	mux.HandleFunc("GET /readyz", s.handleReady)

	// Admin / provisioning.
	mux.HandleFunc("POST /admin/apps", s.adminAuth(s.handleCreateApp))
	mux.HandleFunc("POST /admin/devices", s.adminAuth(s.handleCreateDevice))
	mux.HandleFunc("POST /admin/numbers", s.adminAuth(s.handleCreateNumber))

	// Developer API.
	mux.HandleFunc("POST /v1/verifications", s.devAuth(s.handleStartVerification))
	mux.HandleFunc("GET /v1/verifications/{id}", s.devAuth(s.handleGetVerification))

	// Device API.
	mux.HandleFunc("POST /v1/devices/register", s.deviceAuth(s.handleDeviceRegister))
	mux.HandleFunc("POST /v1/devices/heartbeat", s.deviceAuth(s.handleDeviceHeartbeat))
	mux.HandleFunc("POST /v1/inbound", s.deviceAuth(s.handleInbound))

	return withLogging(s.logger, mux)
}

func (s *Server) handleReady(w http.ResponseWriter, r *http.Request) {
	if err := s.store.Ping(r.Context()); err != nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"status": "not_ready", "detail": "database unreachable"})
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ready"})
}

func withLogging(logger *slog.Logger, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		next.ServeHTTP(w, r)
		logger.Info("request", "method", r.Method, "path", r.URL.Path, "dur", time.Since(start).String())
	})
}
