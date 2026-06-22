// Package api wires the Coordinator's HTTP surface.
//
// Two audiences share this server:
//   - Developer-facing API (SDK -> Coordinator): create and poll verifications.
//   - Device-facing API (Receiver <-> Coordinator): register, heartbeat, post inbound.
//
// Handlers are deliberately thin stubs returning 501 until Phase 1 implements
// the session lifecycle, matching, and persistence.
package api

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"time"

	"github.com/Eshpelin/calltoverify/coordinator/internal/config"
)

// NewRouter returns the fully wired HTTP handler.
func NewRouter(logger *slog.Logger, cfg config.Config) http.Handler {
	_ = cfg // reserved for DB/Redis wiring in Phase 1

	mux := http.NewServeMux()

	// Operational endpoints.
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	})
	mux.HandleFunc("GET /readyz", func(w http.ResponseWriter, r *http.Request) {
		// TODO(phase1): verify Postgres + Redis connectivity before reporting ready.
		writeJSON(w, http.StatusOK, map[string]string{"status": "ready"})
	})

	// Developer-facing API (SDK -> Coordinator).
	mux.HandleFunc("POST /v1/verifications", notImplemented)
	mux.HandleFunc("GET /v1/verifications/{id}", notImplemented)

	// Device-facing API (Receiver <-> Coordinator).
	mux.HandleFunc("POST /v1/devices/register", notImplemented)
	mux.HandleFunc("POST /v1/devices/heartbeat", notImplemented)
	mux.HandleFunc("POST /v1/inbound", notImplemented)

	return withLogging(logger, mux)
}

func notImplemented(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusNotImplemented, map[string]string{
		"error":  "not_implemented",
		"detail": "endpoint is scaffolded; implementation lands in Phase 1",
	})
}

func writeJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}

// withLogging emits one structured line per request.
func withLogging(logger *slog.Logger, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		next.ServeHTTP(w, r)
		logger.Info("request",
			"method", r.Method,
			"path", r.URL.Path,
			"dur", time.Since(start).String(),
		)
	})
}
