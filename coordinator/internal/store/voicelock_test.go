package store

import (
	"context"
	"errors"
	"testing"
	"time"
)

// TestVoiceLockPerNumber verifies the one-pending-voice-session-per-number index:
// a second voice session on the same number conflicts, while SMS multiplexes. Runs
// against every backend so the partial unique index behaves identically.
func TestVoiceLockPerNumber(t *testing.T) {
	runParity(t, func(t *testing.T, st Store) {
		ctx := context.Background()
		app, _, num := seed(t, st, []string{"call", "dtmf"}, "+8801700000001")
		exp := time.Now().Add(time.Minute)

		if _, err := st.CreateSession(ctx, Session{AppID: app.ID, Channel: "call", BindingMode: "claim", NumberID: &num.ID, ExpiresAt: exp}); err != nil {
			t.Fatalf("first voice session: %v", err)
		}
		// A second voice session (dtmf) on the same number must conflict.
		dtmfCode := "123456"
		_, err := st.CreateSession(ctx, Session{AppID: app.ID, Channel: "dtmf", BindingMode: "derive", NumberID: &num.ID, Code: &dtmfCode, ExpiresAt: exp})
		if !errors.Is(err, ErrConflict) {
			t.Fatalf("second voice session on the same number must conflict, got %v", err)
		}
		// SMS multiplexes: an SMS session alongside the voice one is fine.
		smsCode := "654321"
		if _, err := st.CreateSession(ctx, Session{AppID: app.ID, Channel: "sms", BindingMode: "derive", NumberID: &num.ID, Code: &smsCode, ExpiresAt: exp}); err != nil {
			t.Fatalf("sms session should be allowed alongside voice: %v", err)
		}
	})
}
