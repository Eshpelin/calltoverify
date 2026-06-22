package api

import (
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Eshpelin/calltoverify/coordinator/internal/config"
)

func newTestRouter() http.Handler {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	return NewRouter(logger, config.Config{Env: "test"})
}

func TestHealthz(t *testing.T) {
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)

	newTestRouter().ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("healthz: got status %d, want %d", rr.Code, http.StatusOK)
	}
	if !strings.Contains(rr.Body.String(), `"status":"ok"`) {
		t.Fatalf("healthz: unexpected body %q", rr.Body.String())
	}
}

func TestVerificationsStubbed(t *testing.T) {
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/v1/verifications", nil)

	newTestRouter().ServeHTTP(rr, req)

	if rr.Code != http.StatusNotImplemented {
		t.Fatalf("verifications: got status %d, want %d", rr.Code, http.StatusNotImplemented)
	}
}
