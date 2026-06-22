package api

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/Eshpelin/calltoverify/coordinator/internal/auth"
	"github.com/Eshpelin/calltoverify/coordinator/internal/config"
	"github.com/Eshpelin/calltoverify/coordinator/internal/ratelimit"
	"github.com/Eshpelin/calltoverify/coordinator/internal/store"
	"github.com/Eshpelin/calltoverify/coordinator/internal/verify"
)

type fakeNotifier struct {
	mu    sync.Mutex
	calls []store.Session
}

func (f *fakeNotifier) VerificationVerified(sess store.Session, _ store.App) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls = append(f.calls, sess)
}

func (f *fakeNotifier) count() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return len(f.calls)
}

func testServer(t *testing.T) (*httptest.Server, *fakeNotifier) {
	t.Helper()
	dsn := os.Getenv("CTV_TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("set CTV_TEST_DATABASE_URL to run Coordinator integration tests")
	}
	ctx := context.Background()
	st, err := store.New(ctx, dsn)
	if err != nil {
		t.Fatalf("store: %v", err)
	}
	if err := st.Migrate(ctx); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	if err := st.Reset(ctx); err != nil {
		t.Fatalf("reset: %v", err)
	}

	notifier := &fakeNotifier{}
	svc := verify.NewService(st, notifier, ratelimit.New(6000, 1000), 6, 90*time.Second)
	cfg := config.Config{AdminToken: "test-admin", DefaultCodeLen: 6, DefaultTTL: 90 * time.Second}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	srv := NewServer(logger, cfg, st, svc, auth.NewNonceCache(time.Minute))

	ts := httptest.NewServer(srv.Routes())
	t.Cleanup(func() {
		ts.Close()
		st.Close()
	})
	return ts, notifier
}

// --- HTTP helpers ---

func doReq(t *testing.T, method, url string, body any, headers map[string]string) (int, map[string]any) {
	t.Helper()
	var reader io.Reader
	if body != nil {
		raw, _ := json.Marshal(body)
		reader = bytes.NewReader(raw)
	}
	req, err := http.NewRequest(method, url, reader)
	if err != nil {
		t.Fatal(err)
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	var m map[string]any
	if len(raw) > 0 {
		_ = json.Unmarshal(raw, &m)
	}
	return resp.StatusCode, m
}

func adminPost(t *testing.T, ts *httptest.Server, path string, body any) map[string]any {
	t.Helper()
	code, m := doReq(t, http.MethodPost, ts.URL+path, body, map[string]string{"Authorization": "Bearer test-admin"})
	if code < 200 || code >= 300 {
		t.Fatalf("admin %s: status %d: %v", path, code, m)
	}
	return m
}

func deviceReq(t *testing.T, ts *httptest.Server, deviceID, secret, path string, body any) (int, map[string]any) {
	t.Helper()
	raw, _ := json.Marshal(body)
	tss := strconv.FormatInt(time.Now().Unix(), 10)
	nonce := strconv.FormatInt(time.Now().UnixNano(), 10)
	sig := auth.DeviceSignature(secret, tss, nonce, raw)
	req, _ := http.NewRequest(http.MethodPost, ts.URL+path, bytes.NewReader(raw))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-CTV-Device-Id", deviceID)
	req.Header.Set("X-CTV-Timestamp", tss)
	req.Header.Set("X-CTV-Nonce", nonce)
	req.Header.Set("X-CTV-Signature", sig)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	var m map[string]any
	if len(b) > 0 {
		_ = json.Unmarshal(b, &m)
	}
	return resp.StatusCode, m
}

func str(m map[string]any, key string) string {
	if v, ok := m[key].(string); ok {
		return v
	}
	return ""
}

// provision creates an app, a device with the given capabilities, a number, and
// brings the device online via a heartbeat. Returns api key, device id/secret, msisdn.
func provision(t *testing.T, ts *httptest.Server, caps []string, channels []string, msisdn string) (apiKey, deviceID, deviceSecret string) {
	t.Helper()
	app := adminPost(t, ts, "/admin/apps", map[string]any{"name": "test"})
	apiKey = str(app, "api_key")

	dev := adminPost(t, ts, "/admin/devices", map[string]any{
		"app_id": str(app, "app_id"), "name": "dev", "type": "pi", "capabilities": caps,
	})
	deviceID = str(dev, "device_id")
	deviceSecret = str(dev, "device_secret")

	adminPost(t, ts, "/admin/numbers", map[string]any{
		"device_id": deviceID, "msisdn": msisdn, "channels": channels,
	})

	if code, m := deviceReq(t, ts, deviceID, deviceSecret, "/v1/devices/heartbeat", map[string]any{}); code != 200 {
		t.Fatalf("heartbeat: %d %v", code, m)
	}
	return apiKey, deviceID, deviceSecret
}

// --- tests ---

func TestHealthAndReady(t *testing.T) {
	ts, _ := testServer(t)
	if code, _ := doReq(t, http.MethodGet, ts.URL+"/healthz", nil, nil); code != 200 {
		t.Fatalf("healthz: %d", code)
	}
	if code, _ := doReq(t, http.MethodGet, ts.URL+"/readyz", nil, nil); code != 200 {
		t.Fatalf("readyz: %d", code)
	}
}

func TestSMSHappyPath(t *testing.T) {
	ts, notifier := testServer(t)
	apiKey, deviceID, deviceSecret := provision(t, ts, []string{"sms"}, []string{"sms"}, "+8801700000001")

	code, start := doReq(t, http.MethodPost, ts.URL+"/v1/verifications",
		map[string]any{"channel": "sms"}, map[string]string{"Authorization": "Bearer " + apiKey})
	if code != 201 {
		t.Fatalf("start: %d %v", code, start)
	}
	sessionID := str(start, "session_id")
	instr, _ := start["instructions"].(map[string]any)
	otp := str(instr, "code")
	if len(otp) != 6 {
		t.Fatalf("expected 6-digit code, got %q", otp)
	}

	sender := "+8801711111111"
	st, in := deviceReq(t, ts, deviceID, deviceSecret, "/v1/inbound",
		map[string]any{"number": "+8801700000001", "type": "sms", "sender": sender, "body": otp})
	if st != 200 || in["matched"] != true {
		t.Fatalf("inbound: %d %v", st, in)
	}

	_, got := doReq(t, http.MethodGet, ts.URL+"/v1/verifications/"+sessionID, nil,
		map[string]string{"Authorization": "Bearer " + apiKey})
	if str(got, "status") != "verified" || str(got, "verified_msisdn") != sender {
		t.Fatalf("status: %v", got)
	}
	if notifier.count() != 1 {
		t.Fatalf("expected 1 webhook notification, got %d", notifier.count())
	}
}

func TestClaimMismatch(t *testing.T) {
	ts, _ := testServer(t)
	apiKey, deviceID, deviceSecret := provision(t, ts, []string{"sms"}, []string{"sms"}, "+8801700000002")

	_, start := doReq(t, http.MethodPost, ts.URL+"/v1/verifications",
		map[string]any{"channel": "sms", "binding_mode": "claim", "claimed_msisdn": "+8801799999999"},
		map[string]string{"Authorization": "Bearer " + apiKey})
	sessionID := str(start, "session_id")
	otp := str(start["instructions"].(map[string]any), "code")

	// Correct code but from the wrong sender.
	_, in := deviceReq(t, ts, deviceID, deviceSecret, "/v1/inbound",
		map[string]any{"number": "+8801700000002", "type": "sms", "sender": "+8801700000000", "body": otp})
	if in["matched"] != false || str(in, "reason") != "claim_mismatch" {
		t.Fatalf("expected claim_mismatch, got %v", in)
	}

	_, got := doReq(t, http.MethodGet, ts.URL+"/v1/verifications/"+sessionID, nil,
		map[string]string{"Authorization": "Bearer " + apiKey})
	if str(got, "status") != "pending" {
		t.Fatalf("session should still be pending, got %v", got)
	}
}

func TestMissedCallHappyPath(t *testing.T) {
	ts, _ := testServer(t)
	apiKey, deviceID, deviceSecret := provision(t, ts, []string{"call"}, []string{"call"}, "+8801700000003")

	claimed := "+8801722222222"
	_, start := doReq(t, http.MethodPost, ts.URL+"/v1/verifications",
		map[string]any{"channel": "call", "binding_mode": "claim", "claimed_msisdn": claimed},
		map[string]string{"Authorization": "Bearer " + apiKey})
	sessionID := str(start, "session_id")

	st, in := deviceReq(t, ts, deviceID, deviceSecret, "/v1/inbound",
		map[string]any{"number": "+8801700000003", "type": "call", "sender": claimed, "body": ""})
	if st != 200 || in["matched"] != true {
		t.Fatalf("missed-call inbound: %d %v", st, in)
	}

	_, got := doReq(t, http.MethodGet, ts.URL+"/v1/verifications/"+sessionID, nil,
		map[string]string{"Authorization": "Bearer " + apiKey})
	if str(got, "status") != "verified" || str(got, "verified_msisdn") != claimed {
		t.Fatalf("status: %v", got)
	}
}

func TestNoCapacityWhenDeviceOffline(t *testing.T) {
	ts, _ := testServer(t)
	// Provision but do NOT heartbeat: device stays offline, so no number is pickable.
	app := adminPost(t, ts, "/admin/apps", map[string]any{"name": "test"})
	dev := adminPost(t, ts, "/admin/devices", map[string]any{
		"app_id": str(app, "app_id"), "name": "dev", "type": "pi", "capabilities": []string{"sms"},
	})
	adminPost(t, ts, "/admin/numbers", map[string]any{
		"device_id": str(dev, "device_id"), "msisdn": "+8801700000004", "channels": []string{"sms"},
	})

	code, m := doReq(t, http.MethodPost, ts.URL+"/v1/verifications",
		map[string]any{"channel": "sms"}, map[string]string{"Authorization": "Bearer " + str(app, "api_key")})
	if code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503 no_capacity, got %d %v", code, m)
	}
}

func TestDevAuthRejectsBadKey(t *testing.T) {
	ts, _ := testServer(t)
	code, _ := doReq(t, http.MethodPost, ts.URL+"/v1/verifications",
		map[string]any{"channel": "sms"}, map[string]string{"Authorization": "Bearer nope"})
	if code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", code)
	}
}
