package store

import (
	"context"
	"errors"
	"testing"
	"time"
)

// TestVoiceLockSQLite verifies the one-pending-voice-session-per-number index:
// a second voice session on the same number conflicts, while SMS multiplexes.
func TestVoiceLockSQLite(t *testing.T) {
	st, err := NewSQLite(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	ctx := context.Background()
	if err := st.Migrate(ctx); err != nil {
		t.Fatal(err)
	}

	app, err := st.CreateApp(ctx, App{Name: "a", APIKeyHash: "h", APIKeyPrefix: "p", WebhookSecret: "w"})
	if err != nil {
		t.Fatal(err)
	}
	dev, err := st.CreateDevice(ctx, Device{AppID: app.ID, Name: "d", DeviceSecret: "s", Type: "pi", Capabilities: []string{"call", "dtmf"}})
	if err != nil {
		t.Fatal(err)
	}
	num, err := st.CreateNumber(ctx, Number{DeviceID: dev.ID, MSISDN: "+8801700000001", Channels: []string{"call", "dtmf"}})
	if err != nil {
		t.Fatal(err)
	}
	exp := time.Now().Add(time.Minute)

	if _, err := st.CreateSession(ctx, Session{AppID: app.ID, Channel: "call", BindingMode: "claim", NumberID: &num.ID, ExpiresAt: exp}); err != nil {
		t.Fatalf("first voice session: %v", err)
	}
	// A second voice session (dtmf) on the same number must conflict.
	dtmfCode := "123456"
	_, err = st.CreateSession(ctx, Session{AppID: app.ID, Channel: "dtmf", BindingMode: "derive", NumberID: &num.ID, Code: &dtmfCode, ExpiresAt: exp})
	if !errors.Is(err, ErrConflict) {
		t.Fatalf("second voice session on the same number must conflict, got %v", err)
	}
	// SMS multiplexes: an SMS session alongside the voice one is fine.
	smsCode := "654321"
	if _, err := st.CreateSession(ctx, Session{AppID: app.ID, Channel: "sms", BindingMode: "derive", NumberID: &num.ID, Code: &smsCode, ExpiresAt: exp}); err != nil {
		t.Fatalf("sms session should be allowed alongside voice: %v", err)
	}
}
