package engine

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/Eshpelin/calltoverify/coordinator/internal/auth"
)

// newTestEngine builds an engine on an in-memory SQLite store plus an httptest
// server mounting the device handler at /ctv.
func newTestEngine(t *testing.T) (*Engine, *httptest.Server, *recorder) {
	t.Helper()
	rec := &recorder{}
	eng, err := New(context.Background(), Options{
		SQLitePath: ":memory:",
		OnVerified: rec.add,
	})
	if err != nil {
		t.Fatalf("new engine: %v", err)
	}
	mux := http.NewServeMux()
	mux.Handle("/ctv/", eng.DeviceHandler("/ctv"))
	ts := httptest.NewServer(mux)
	t.Cleanup(func() {
		ts.Close()
		eng.Close()
	})
	return eng, ts, rec
}

type recorder struct {
	mu     sync.Mutex
	events []Event
}

func (r *recorder) add(ev Event) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.events = append(r.events, ev)
}

func (r *recorder) count() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return len(r.events)
}

func deviceReq(t *testing.T, ts *httptest.Server, p Pairing, path string, body any) (int, map[string]any) {
	t.Helper()
	raw, _ := json.Marshal(body)
	tss := strconv.FormatInt(time.Now().Unix(), 10)
	nonce := strconv.FormatInt(time.Now().UnixNano(), 10)
	sig := auth.DeviceSignature(p.DeviceSecret, tss, nonce, raw)
	req, _ := http.NewRequest(http.MethodPost, ts.URL+path, bytes.NewReader(raw))
	req.Header.Set("X-CTV-Device-Id", p.DeviceID)
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

func TestEmbeddedSMSFlow(t *testing.T) {
	ctx := context.Background()
	eng, ts, rec := newTestEngine(t)

	pairing, err := eng.NewPairing(ctx, PairingParams{
		Endpoint: ts.URL + "/ctv", Name: "phone", MSISDN: "+8801700000001", Channels: []string{"sms"},
	})
	if err != nil {
		t.Fatalf("pairing: %v", err)
	}
	if pairing.QRPayload == "" || pairing.DeviceSecret == "" {
		t.Fatal("pairing payload/secret missing")
	}

	// Bring the device online.
	if code, m := deviceReq(t, ts, pairing, "/ctv/devices/heartbeat", map[string]any{}); code != 200 {
		t.Fatalf("heartbeat: %d %v", code, m)
	}

	v, err := eng.StartVerification(ctx, Params{Channel: "sms"})
	if err != nil {
		t.Fatalf("start: %v", err)
	}
	if len(v.Instructions.Code) != 6 {
		t.Fatalf("want 6-digit code, got %q", v.Instructions.Code)
	}

	sender := "+8801712345678"
	code, in := deviceReq(t, ts, pairing, "/ctv/inbound",
		map[string]any{"number": "+8801700000001", "type": "sms", "sender": sender, "body": v.Instructions.Code})
	if code != 200 || in["matched"] != true {
		t.Fatalf("inbound: %d %v", code, in)
	}

	st, err := eng.Status(ctx, v.SessionID)
	if err != nil {
		t.Fatalf("status: %v", err)
	}
	if st.Status != "verified" || st.VerifiedMSISDN != sender {
		t.Fatalf("status = %+v", st)
	}
	if rec.count() != 1 {
		t.Fatalf("OnVerified called %d times, want 1", rec.count())
	}
}

func TestEmbeddedMissedCall(t *testing.T) {
	ctx := context.Background()
	eng, ts, _ := newTestEngine(t)

	pairing, err := eng.NewPairing(ctx, PairingParams{
		Endpoint: ts.URL + "/ctv", Name: "phone", MSISDN: "+8801700000002", Channels: []string{"call"},
	})
	if err != nil {
		t.Fatal(err)
	}
	deviceReq(t, ts, pairing, "/ctv/devices/heartbeat", map[string]any{})

	claimed := "+8801722222222"
	v, err := eng.StartVerification(ctx, Params{Channel: "call", BindingMode: "claim", ClaimedMSISDN: claimed})
	if err != nil {
		t.Fatal(err)
	}
	if v.Instructions.Code != "" {
		t.Fatalf("missed call should have no code, got %q", v.Instructions.Code)
	}

	code, in := deviceReq(t, ts, pairing, "/ctv/inbound",
		map[string]any{"number": "+8801700000002", "type": "call", "sender": claimed, "body": ""})
	if code != 200 || in["matched"] != true {
		t.Fatalf("missed-call inbound: %d %v", code, in)
	}

	st, _ := eng.Status(ctx, v.SessionID)
	if st.Status != "verified" {
		t.Fatalf("status = %+v", st)
	}
}

func TestInvalidComboRejected(t *testing.T) {
	eng, _, _ := newTestEngine(t)
	_, err := eng.StartVerification(context.Background(), Params{Channel: "call", BindingMode: "derive"})
	var e *Error
	if err == nil {
		t.Fatal("expected error for missed-call + derive")
	}
	if !asEngineError(err, &e) {
		t.Fatalf("want *engine.Error, got %T", err)
	}
}

func TestNoCapacity(t *testing.T) {
	eng, _, _ := newTestEngine(t)
	// No paired device, so no number can serve sms.
	_, err := eng.StartVerification(context.Background(), Params{Channel: "sms"})
	if err != ErrNoCapacity {
		t.Fatalf("want ErrNoCapacity, got %v", err)
	}
}

func TestStatusNotFound(t *testing.T) {
	eng, _, _ := newTestEngine(t)
	_, err := eng.Status(context.Background(), "nope")
	if err != ErrNotFound {
		t.Fatalf("want ErrNotFound, got %v", err)
	}
}

func TestVoiceConcurrencyOnePerSIM(t *testing.T) {
	ctx := context.Background()
	eng, ts, _ := newTestEngine(t)
	pairing, err := eng.NewPairing(ctx, PairingParams{
		Endpoint: ts.URL + "/ctv", Name: "phone", MSISDN: "+8801700000010", Channels: []string{"call"},
	})
	if err != nil {
		t.Fatal(err)
	}
	deviceReq(t, ts, pairing, "/ctv/devices/heartbeat", map[string]any{})

	// First voice verification occupies the SIM's single line.
	if _, err := eng.StartVerification(ctx, Params{Channel: "call", BindingMode: "claim", ClaimedMSISDN: "+8801711111111"}); err != nil {
		t.Fatalf("first voice start: %v", err)
	}
	// Second must report the line is busy (queued), not no-capacity.
	_, err = eng.StartVerification(ctx, Params{Channel: "call", BindingMode: "claim", ClaimedMSISDN: "+8801722222222"})
	var be *BusyError
	if !errors.As(err, &be) {
		t.Fatalf("want *BusyError, got %v", err)
	}
	if be.Position != 1 {
		t.Fatalf("want position 1, got %d", be.Position)
	}
}

func TestSMSNotSerialised(t *testing.T) {
	ctx := context.Background()
	eng, ts, _ := newTestEngine(t)
	pairing, err := eng.NewPairing(ctx, PairingParams{
		Endpoint: ts.URL + "/ctv", Name: "phone", MSISDN: "+8801700000011", Channels: []string{"sms"},
	})
	if err != nil {
		t.Fatal(err)
	}
	deviceReq(t, ts, pairing, "/ctv/devices/heartbeat", map[string]any{})

	// SMS multiplexes: many concurrent verifications on one number are fine.
	if _, err := eng.StartVerification(ctx, Params{Channel: "sms"}); err != nil {
		t.Fatal(err)
	}
	if _, err := eng.StartVerification(ctx, Params{Channel: "sms"}); err != nil {
		t.Fatalf("second sms should also start: %v", err)
	}
}

func TestAutoBlockBruteForce(t *testing.T) {
	ctx := context.Background()
	eng, ts, _ := newTestEngine(t)
	pairing, err := eng.NewPairing(ctx, PairingParams{
		Endpoint: ts.URL + "/ctv", Name: "phone", MSISDN: "+8801700000020", Channels: []string{"sms"},
	})
	if err != nil {
		t.Fatal(err)
	}
	deviceReq(t, ts, pairing, "/ctv/devices/heartbeat", map[string]any{})

	const attacker = "+8801799999999"
	body := func(b string) map[string]any {
		return map[string]any{"number": "+8801700000020", "type": "sms", "sender": attacker, "body": b}
	}

	// Five failed (wrong-code) inbounds trip the auto-block.
	for i := 0; i < 5; i++ {
		code, in := deviceReq(t, ts, pairing, "/ctv/inbound", body("000000"))
		if code != 200 || in["matched"] != false {
			t.Fatalf("attempt %d should not match: %d %v", i, code, in)
		}
	}
	// The next inbound from that sender is rejected as blocked.
	_, in := deviceReq(t, ts, pairing, "/ctv/inbound", body("000000"))
	if in["reason"] != "blocked" {
		t.Fatalf("expected blocked after brute force, got %v", in)
	}
}

func asEngineError(err error, target **Error) bool {
	e, ok := err.(*Error)
	if ok {
		*target = e
	}
	return ok
}
