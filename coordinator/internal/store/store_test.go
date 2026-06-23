package store

import (
	"context"
	"errors"
	"os"
	"testing"
	"time"
)

// backends returns the store implementations to run parity checks against. SQLite
// (in-memory) always runs; Postgres runs only when CTV_TEST_DATABASE_URL is set
// (the same gate the API integration tests use). Each scenario runs identically
// against every backend so divergence in the SQL surfaces immediately.
func backends(t *testing.T) map[string]func(t *testing.T) Store {
	t.Helper()
	m := map[string]func(t *testing.T) Store{
		"sqlite": func(t *testing.T) Store {
			st, err := NewSQLite(":memory:")
			if err != nil {
				t.Fatalf("sqlite open: %v", err)
			}
			if err := st.Migrate(context.Background()); err != nil {
				t.Fatalf("sqlite migrate: %v", err)
			}
			t.Cleanup(st.Close)
			return st
		},
	}
	if dsn := os.Getenv("CTV_TEST_DATABASE_URL"); dsn != "" {
		m["postgres"] = func(t *testing.T) Store {
			st, err := NewPostgres(context.Background(), dsn)
			if err != nil {
				t.Fatalf("postgres open: %v", err)
			}
			ctx := context.Background()
			if err := st.Migrate(ctx); err != nil {
				t.Fatalf("postgres migrate: %v", err)
			}
			if err := st.Reset(ctx); err != nil {
				t.Fatalf("postgres reset: %v", err)
			}
			t.Cleanup(st.Close)
			return st
		}
	}
	return m
}

// runParity runs fn against every available backend as a subtest.
func runParity(t *testing.T, fn func(t *testing.T, st Store)) {
	for name, open := range backends(t) {
		t.Run(name, func(t *testing.T) {
			fn(t, open(t))
		})
	}
}

// seed creates an app + online device + one number serving the given channels.
func seed(t *testing.T, st Store, channels []string, msisdn string) (App, Device, Number) {
	t.Helper()
	ctx := context.Background()
	app, err := st.CreateApp(ctx, App{Name: "parity", APIKeyHash: "h-" + msisdn, APIKeyPrefix: "p"})
	if err != nil {
		t.Fatalf("CreateApp: %v", err)
	}
	dev, err := st.CreateDevice(ctx, Device{AppID: app.ID, Name: "d", DeviceSecret: "s", Type: "pi", Capabilities: channels})
	if err != nil {
		t.Fatalf("CreateDevice: %v", err)
	}
	if err := st.SetHeartbeat(ctx, dev.ID); err != nil { // brings device online
		t.Fatalf("SetHeartbeat: %v", err)
	}
	num, err := st.CreateNumber(ctx, Number{DeviceID: dev.ID, MSISDN: msisdn, Channels: channels})
	if err != nil {
		t.Fatalf("CreateNumber: %v", err)
	}
	return app, dev, num
}

// TestParityDeleteDeviceCascade checks that removing a device deletes its
// numbers (ON DELETE CASCADE) while retaining past verifications.
func TestParityDeleteDeviceCascade(t *testing.T) {
	runParity(t, func(t *testing.T, st Store) {
		ctx := context.Background()
		app, dev, num := seed(t, st, []string{"sms"}, "+8801700000055")

		// A past, verified session referencing the device's number.
		code := "654321"
		sess, err := st.CreateSession(ctx, Session{
			AppID: app.ID, Channel: "sms", BindingMode: "derive",
			NumberID: &num.ID, Code: &code, ExpiresAt: time.Now().Add(time.Minute),
		})
		if err != nil {
			t.Fatalf("CreateSession: %v", err)
		}
		if _, err := st.VerifySession(ctx, sess.ID, "+8801712345678"); err != nil {
			t.Fatalf("VerifySession: %v", err)
		}

		if err := st.DeleteDevice(ctx, dev.ID); err != nil {
			t.Fatalf("DeleteDevice: %v", err)
		}

		// Device is gone.
		if _, err := st.GetDeviceByID(ctx, dev.ID); !errors.Is(err, ErrNotFound) {
			t.Fatalf("GetDeviceByID after delete = %v, want ErrNotFound", err)
		}
		// Its numbers cascaded away.
		nums, err := st.ListNumbersByDevice(ctx, dev.ID)
		if err != nil {
			t.Fatalf("ListNumbersByDevice: %v", err)
		}
		if len(nums) != 0 {
			t.Fatalf("numbers after device delete = %d, want 0", len(nums))
		}
		// The past verification is retained.
		recent, err := st.ListRecentSessions(ctx, app.ID, 10)
		if err != nil {
			t.Fatalf("ListRecentSessions: %v", err)
		}
		found := false
		for _, s := range recent {
			if s.ID == sess.ID {
				found = true
			}
		}
		if !found {
			t.Fatalf("verified session not retained after device delete")
		}
		// Deleting an unknown device is ErrNotFound.
		if err := st.DeleteDevice(ctx, dev.ID); !errors.Is(err, ErrNotFound) {
			t.Fatalf("DeleteDevice(unknown) = %v, want ErrNotFound", err)
		}
	})
}

// TestParityInboundEventRetention checks that DeleteInboundEventsBefore prunes
// audit rows older than the cutoff, on both backends.
func TestParityInboundEventRetention(t *testing.T) {
	runParity(t, func(t *testing.T, st Store) {
		ctx := context.Background()
		_, _, num := seed(t, st, []string{"sms"}, "+8801700000099")
		sender := "+8801712345678"
		for i := 0; i < 3; i++ {
			if err := st.CreateInboundEvent(ctx, InboundEvent{NumberID: num.ID, Type: "sms", Sender: sender, Body: "x"}); err != nil {
				t.Fatalf("CreateInboundEvent: %v", err)
			}
		}
		if n, err := st.CountInboundBySender(ctx, sender, time.Now().Add(-time.Hour), false); err != nil || n != 3 {
			t.Fatalf("pre-prune count = %d err %v, want 3", n, err)
		}
		// Cutoff just after "now" prunes everything created above.
		deleted, err := st.DeleteInboundEventsBefore(ctx, time.Now().Add(time.Minute))
		if err != nil || deleted != 3 {
			t.Fatalf("DeleteInboundEventsBefore = %d err %v, want 3", deleted, err)
		}
		if n, _ := st.CountInboundBySender(ctx, sender, time.Now().Add(-time.Hour), false); n != 0 {
			t.Fatalf("post-prune count = %d, want 0", n)
		}
	})
}

func TestParitySMSVerifyFlow(t *testing.T) {
	runParity(t, func(t *testing.T, st Store) {
		ctx := context.Background()
		app, _, num := seed(t, st, []string{"sms"}, "+8801700000001")

		// PickNumber returns our only online number.
		picked, err := st.PickNumber(ctx, "sms")
		if err != nil || picked.ID != num.ID {
			t.Fatalf("PickNumber: got %v err %v, want %s", picked.ID, err, num.ID)
		}

		code := "123456"
		sess, err := st.CreateSession(ctx, Session{
			AppID: app.ID, Channel: "sms", BindingMode: "derive",
			NumberID: &num.ID, Code: &code, ExpiresAt: time.Now().Add(time.Minute),
		})
		if err != nil {
			t.Fatalf("CreateSession: %v", err)
		}
		if sess.Status != "pending" {
			t.Fatalf("new session status = %q, want pending", sess.Status)
		}

		// Find it by code, verify once, second verify loses the race.
		found, err := st.FindPendingByCode(ctx, num.ID, code, "sms")
		if err != nil || found.ID != sess.ID {
			t.Fatalf("FindPendingByCode: got %v err %v", found.ID, err)
		}
		ok, err := st.VerifySession(ctx, sess.ID, "+8801712345678")
		if err != nil || !ok {
			t.Fatalf("VerifySession first: ok=%v err=%v, want true", ok, err)
		}
		ok, err = st.VerifySession(ctx, sess.ID, "+8809999999999")
		if err != nil || ok {
			t.Fatalf("VerifySession second: ok=%v err=%v, want false (single-winner)", ok, err)
		}

		got, err := st.GetSession(ctx, app.ID, sess.ID)
		if err != nil {
			t.Fatalf("GetSession: %v", err)
		}
		if got.Status != "verified" || got.VerifiedMSISDN == nil || *got.VerifiedMSISDN != "+8801712345678" {
			t.Fatalf("verified session = %+v", got)
		}
		// A verified session is no longer findable as pending.
		if _, err := st.FindPendingByCode(ctx, num.ID, code, "sms"); err != ErrNotFound {
			t.Fatalf("FindPendingByCode after verify: err=%v, want ErrNotFound", err)
		}
	})
}

func TestParityCodeUniquenessConflict(t *testing.T) {
	runParity(t, func(t *testing.T, st Store) {
		ctx := context.Background()
		app, _, num := seed(t, st, []string{"sms"}, "+8801700000002")
		code := "654321"
		mk := func() (Session, error) {
			return st.CreateSession(ctx, Session{
				AppID: app.ID, Channel: "sms", BindingMode: "derive",
				NumberID: &num.ID, Code: &code, ExpiresAt: time.Now().Add(time.Minute),
			})
		}
		if _, err := mk(); err != nil {
			t.Fatalf("first CreateSession: %v", err)
		}
		// Same (number, code) among pending sessions must conflict on both backends.
		if _, err := mk(); err != ErrConflict {
			t.Fatalf("duplicate pending code: err=%v, want ErrConflict", err)
		}
	})
}

func TestParityVoiceSerialisedPerSIM(t *testing.T) {
	runParity(t, func(t *testing.T, st Store) {
		ctx := context.Background()
		app, _, num := seed(t, st, []string{"call"}, "+8801700000003")

		// First voice pick succeeds; create its pending session.
		if _, err := st.PickNumber(ctx, "call"); err != nil {
			t.Fatalf("first PickNumber(call): %v", err)
		}
		claimed := "+8801722222222"
		if _, err := st.CreateSession(ctx, Session{
			AppID: app.ID, Channel: "call", BindingMode: "claim",
			NumberID: &num.ID, ClaimedMSISDN: &claimed, ExpiresAt: time.Now().Add(time.Minute),
		}); err != nil {
			t.Fatalf("CreateSession(call): %v", err)
		}

		// The SIM is now busy: PickNumber(call) must find nothing, but the line
		// still counts as available capacity (distinguishes busy from no-capacity).
		if _, err := st.PickNumber(ctx, "call"); err != ErrNotFound {
			t.Fatalf("PickNumber(call) while busy: err=%v, want ErrNotFound", err)
		}
		avail, err := st.CountAvailableNumbers(ctx, "call")
		if err != nil || avail != 1 {
			t.Fatalf("CountAvailableNumbers(call) = %d err %v, want 1", avail, err)
		}
	})
}

func TestParityExpireDue(t *testing.T) {
	runParity(t, func(t *testing.T, st Store) {
		ctx := context.Background()
		app, _, num := seed(t, st, []string{"sms"}, "+8801700000004")
		code := "111111"
		if _, err := st.CreateSession(ctx, Session{
			AppID: app.ID, Channel: "sms", BindingMode: "derive",
			NumberID: &num.ID, Code: &code, ExpiresAt: time.Now().Add(-time.Second), // already due
		}); err != nil {
			t.Fatalf("CreateSession: %v", err)
		}
		n, err := st.ExpireDue(ctx)
		if err != nil || n != 1 {
			t.Fatalf("ExpireDue = %d err %v, want 1", n, err)
		}
		// Idempotent: a second sweep expires nothing.
		if n, err := st.ExpireDue(ctx); err != nil || n != 0 {
			t.Fatalf("ExpireDue second = %d err %v, want 0", n, err)
		}
	})
}

func TestParityBlocksAndInboundCount(t *testing.T) {
	runParity(t, func(t *testing.T, st Store) {
		ctx := context.Background()
		app, _, num := seed(t, st, []string{"sms"}, "+8801700000005")
		sender := "+8801733333333"

		if blocked, err := st.IsBlocked(ctx, sender); err != nil || blocked {
			t.Fatalf("IsBlocked before: %v err %v, want false", blocked, err)
		}
		// Record two failed (unmatched) inbound events and one matched. The matched
		// event references a real session id: Postgres enforces matched_session_id
		// as a UUID FK, so a synthetic id would only pass on SQLite (untyped TEXT).
		for i := 0; i < 2; i++ {
			if err := st.CreateInboundEvent(ctx, InboundEvent{NumberID: num.ID, Type: "sms", Sender: sender, Body: "x"}); err != nil {
				t.Fatalf("CreateInboundEvent: %v", err)
			}
		}
		code := "222222"
		sess, err := st.CreateSession(ctx, Session{
			AppID: app.ID, Channel: "sms", BindingMode: "derive",
			NumberID: &num.ID, Code: &code, ExpiresAt: time.Now().Add(time.Minute),
		})
		if err != nil {
			t.Fatalf("CreateSession: %v", err)
		}
		if err := st.CreateInboundEvent(ctx, InboundEvent{NumberID: num.ID, Type: "sms", Sender: sender, Body: "y", MatchedSessionID: &sess.ID}); err != nil {
			t.Fatalf("CreateInboundEvent matched: %v", err)
		}
		since := time.Now().Add(-time.Hour)
		if n, err := st.CountInboundBySender(ctx, sender, since, false); err != nil || n != 3 {
			t.Fatalf("CountInboundBySender all = %d err %v, want 3", n, err)
		}
		if n, err := st.CountInboundBySender(ctx, sender, since, true); err != nil || n != 2 {
			t.Fatalf("CountInboundBySender unmatched = %d err %v, want 2", n, err)
		}

		until := time.Now().Add(time.Hour)
		if err := st.CreateBlock(ctx, sender, "msisdn", "test", &until); err != nil {
			t.Fatalf("CreateBlock: %v", err)
		}
		if blocked, err := st.IsBlocked(ctx, sender); err != nil || !blocked {
			t.Fatalf("IsBlocked after: %v err %v, want true", blocked, err)
		}
	})
}
