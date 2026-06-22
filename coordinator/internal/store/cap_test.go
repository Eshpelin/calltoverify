package store

import (
	"context"
	"errors"
	"strconv"
	"testing"
	"time"
)

// TestMaxPendingPerNumber verifies a number is skipped once it holds the cap of
// pending sessions, bounding flood/exhaustion abuse.
func TestMaxPendingPerNumber(t *testing.T) {
	old := MaxPendingPerNumber
	MaxPendingPerNumber = 3
	defer func() { MaxPendingPerNumber = old }()

	st, err := NewSQLite(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	ctx := context.Background()
	if err := st.Migrate(ctx); err != nil {
		t.Fatal(err)
	}

	app, _ := st.CreateApp(ctx, App{Name: "a", APIKeyHash: "h", APIKeyPrefix: "p", WebhookSecret: "w"})
	dev, _ := st.CreateDevice(ctx, Device{AppID: app.ID, Name: "d", DeviceSecret: "s", Type: "pi", Capabilities: []string{"sms"}})
	num, _ := st.CreateNumber(ctx, Number{DeviceID: dev.ID, MSISDN: "+8801700000001", Channels: []string{"sms"}})
	if err := st.SetHeartbeat(ctx, dev.ID); err != nil {
		t.Fatal(err)
	}
	exp := time.Now().Add(time.Minute)

	// Below the cap, the number is pickable.
	if _, err := st.PickNumber(ctx, "sms"); err != nil {
		t.Fatalf("number should be pickable when empty: %v", err)
	}

	// Fill it to the cap.
	for i := 0; i < 3; i++ {
		code := strconv.Itoa(100000 + i)
		if _, err := st.CreateSession(ctx, Session{AppID: app.ID, Channel: "sms", BindingMode: "derive", NumberID: &num.ID, Code: &code, ExpiresAt: exp}); err != nil {
			t.Fatalf("create session %d: %v", i, err)
		}
	}

	// Now it is skipped.
	if _, err := st.PickNumber(ctx, "sms"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("a number at the pending cap should not be pickable, got %v", err)
	}
}
