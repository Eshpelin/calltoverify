package store

import (
	"context"
	"errors"
	"strconv"
	"testing"
	"time"
)

// TestMaxPendingPerNumber verifies a number is skipped once it holds the cap of
// pending sessions, bounding flood/exhaustion abuse. Runs against every backend
// with the cap configured via WithMaxPending so the cap SQL is exercised on both.
func TestMaxPendingPerNumber(t *testing.T) {
	runParity(t, func(t *testing.T, st Store) {
		ctx := context.Background()
		app, _, num := seed(t, st, []string{"sms"}, "+8801700000001")
		exp := time.Now().Add(time.Minute)

		// Below the cap, the number is pickable.
		if _, err := st.PickNumber(ctx, "sms"); err != nil {
			t.Fatalf("number should be pickable when empty: %v", err)
		}

		// Fill it to the cap of 3.
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
	}, WithMaxPending(3))
}
