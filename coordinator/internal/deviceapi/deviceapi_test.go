package deviceapi

import (
	"bytes"
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
	"time"

	"github.com/Eshpelin/calltoverify/coordinator/internal/auth"
	"github.com/Eshpelin/calltoverify/coordinator/internal/store"
)

// newAuthTestHandler builds a handler over an in-memory store with one device,
// returning the handler, the device id, and its secret. svc is nil — the Auth
// middleware never calls it.
func newAuthTestHandler(t *testing.T) (*Handler, string, string) {
	t.Helper()
	st, err := store.NewSQLite(":memory:")
	if err != nil {
		t.Fatalf("sqlite: %v", err)
	}
	t.Cleanup(st.Close)
	ctx := context.Background()
	if err := st.Migrate(ctx); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	app, err := st.CreateApp(ctx, store.App{Name: "t", APIKeyHash: "h", APIKeyPrefix: "p", WebhookSecret: "w"})
	if err != nil {
		t.Fatalf("app: %v", err)
	}
	dev, err := st.CreateDevice(ctx, store.Device{AppID: app.ID, Name: "d", DeviceSecret: "s3cr3t", Type: "pi", Capabilities: []string{"sms"}})
	if err != nil {
		t.Fatalf("device: %v", err)
	}
	h := New(st, nil, auth.NewNonceCache(time.Minute), slog.New(slog.NewTextHandler(io.Discard, nil)))
	return h, dev.ID, "s3cr3t"
}

func signedReq(deviceID, secret string, body []byte) *http.Request {
	ts := strconv.FormatInt(time.Now().Unix(), 10)
	nonce := strconv.FormatInt(time.Now().UnixNano(), 10)
	sig := auth.DeviceSignature(secret, ts, nonce, body)
	r := httptest.NewRequest(http.MethodPost, "/inbound", bytes.NewReader(body))
	r.RemoteAddr = "203.0.113.7:5555"
	r.Header.Set("X-CTV-Device-Id", deviceID)
	r.Header.Set("X-CTV-Timestamp", ts)
	r.Header.Set("X-CTV-Nonce", nonce)
	r.Header.Set("X-CTV-Signature", sig)
	return r
}

func TestAuthAcceptsValidSignature(t *testing.T) {
	h, id, secret := newAuthTestHandler(t)
	called := false
	handler := h.Auth(func(http.ResponseWriter, *http.Request) { called = true })

	rr := httptest.NewRecorder()
	handler(rr, signedReq(id, secret, []byte(`{}`)))
	if rr.Code != http.StatusOK || !called {
		t.Fatalf("valid signed request: code=%d called=%v", rr.Code, called)
	}
}

func TestAuthMissingHeaders(t *testing.T) {
	h, _, _ := newAuthTestHandler(t)
	handler := h.Auth(func(http.ResponseWriter, *http.Request) { t.Fatal("next should not run") })
	rr := httptest.NewRecorder()
	handler(rr, httptest.NewRequest(http.MethodPost, "/inbound", nil))
	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("missing headers: code=%d", rr.Code)
	}
}

// TestAuthErrorIsGeneric: an unknown device id and a bad signature must return
// the SAME message so a caller cannot enumerate valid device ids.
func TestAuthErrorIsGeneric(t *testing.T) {
	h, id, _ := newAuthTestHandler(t)
	handler := h.Auth(func(http.ResponseWriter, *http.Request) { t.Fatal("next should not run") })

	// Bad signature for a real device.
	rrBad := httptest.NewRecorder()
	handler(rrBad, signedReq(id, "wrong-secret", []byte(`{}`)))

	// Unknown device id.
	rrUnknown := httptest.NewRecorder()
	handler(rrUnknown, signedReq("11111111-1111-4111-8111-111111111111", "whatever", []byte(`{}`)))

	if rrBad.Code != http.StatusUnauthorized || rrUnknown.Code != http.StatusUnauthorized {
		t.Fatalf("both should be 401: bad=%d unknown=%d", rrBad.Code, rrUnknown.Code)
	}
	if rrBad.Body.String() != rrUnknown.Body.String() {
		t.Fatalf("error bodies differ — enumeration oracle:\n bad=%s\n unknown=%s", rrBad.Body.String(), rrUnknown.Body.String())
	}
}
