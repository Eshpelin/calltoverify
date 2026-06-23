package verify

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/Eshpelin/calltoverify/coordinator/internal/ratelimit"
	"github.com/Eshpelin/calltoverify/coordinator/internal/store"
)

// seedSmsNumber creates an app + one online device + one SMS number, returning the device.
func seedSmsNumber(t *testing.T, st store.Store, msisdn string) store.Device {
	t.Helper()
	ctx := context.Background()
	app, err := st.CreateApp(ctx, store.App{Name: "rl", APIKeyHash: "h-" + msisdn, APIKeyPrefix: "p", WebhookSecret: "w"})
	if err != nil {
		t.Fatalf("CreateApp: %v", err)
	}
	dev, err := st.CreateDevice(ctx, store.Device{AppID: app.ID, Name: "d", DeviceSecret: "s", Type: "android", Capabilities: []string{"sms"}})
	if err != nil {
		t.Fatalf("CreateDevice: %v", err)
	}
	if err := st.SetHeartbeat(ctx, dev.ID); err != nil {
		t.Fatalf("SetHeartbeat: %v", err)
	}
	if _, err := st.CreateNumber(ctx, store.Number{DeviceID: dev.ID, MSISDN: msisdn, Channels: []string{"sms"}}); err != nil {
		t.Fatalf("CreateNumber: %v", err)
	}
	return dev
}

// panicNotifier panics on notification, modelling a user OnVerified callback that
// throws. The verification has already committed, so Inbound must still succeed.
type panicNotifier struct{}

func (panicNotifier) VerificationVerified(store.Session, store.App) { panic("boom in OnVerified") }

func TestNotifierPanicDoesNotBreakVerification(t *testing.T) {
	for name, open := range verifyBackends(t) {
		t.Run(name, func(t *testing.T) {
			st := open(t)
			ctx := context.Background()
			dev := seedSmsNumber(t, st, "+8801700000021")
			app, err := st.GetAppByID(ctx, dev.AppID)
			if err != nil {
				t.Fatalf("GetAppByID: %v", err)
			}
			svc := NewService(st, panicNotifier{}, ratelimit.New(60, 10), 6, time.Minute, nil)
			res, err := svc.Start(ctx, app, StartRequest{Channel: "sms"})
			if err != nil {
				t.Fatalf("Start: %v", err)
			}
			in, err := svc.Inbound(ctx, dev, InboundRequest{
				Number: "+8801700000021", Type: "sms", Sender: "+8801712345678", Body: res.Instructions.Code,
			})
			if err != nil {
				t.Fatalf("Inbound returned error (panic not recovered): %v", err)
			}
			if !in.Matched {
				t.Fatalf("expected Matched=true despite notifier panic, got %+v", in)
			}
		})
	}
}

// TestInboundThrottledDespiteSenderRotation is the regression test for the
// throttle-bypass finding: the per-sender limiter and auto-block are keyed on the
// device-supplied `sender`, so a compromised device that rotates it evades them.
// With the per-device and per-number rate limits in place, a rotated-sender flood
// from one device is bounded regardless of the sender values it claims.
func TestInboundThrottledDespiteSenderRotation(t *testing.T) {
	for name, open := range verifyBackends(t) {
		t.Run(name, func(t *testing.T) {
			st := open(t)
			dev := seedSmsNumber(t, st, "+8801700000020")
			// Burst of 3 so the 4th+ inbound from this device/number is throttled.
			svc := NewService(st, noopNotifier{}, ratelimit.New(60, 3), 6, time.Minute, nil)

			const n = 8
			reasons := make([]string, n)
			for i := 0; i < n; i++ {
				res, err := svc.Inbound(context.Background(), dev, InboundRequest{
					Number: "+8801700000020",
					Type:   "sms",
					Sender: fmt.Sprintf("+8801%09d", i), // a fresh, never-repeated sender each time
					Body:   "000000",
				})
				if err != nil {
					t.Fatalf("Inbound %d: %v", i, err)
				}
				reasons[i] = res.Reason
			}

			limited := 0
			for _, r := range reasons {
				if r == "rate_limited" {
					limited++
				}
			}
			if limited == 0 {
				t.Fatalf("rotated-sender flood was never rate-limited — per-device/number cap not enforced; reasons=%v", reasons)
			}
			if reasons[0] == "rate_limited" {
				t.Fatalf("first inbound should pass the burst, got rate_limited immediately; reasons=%v", reasons)
			}
		})
	}
}
