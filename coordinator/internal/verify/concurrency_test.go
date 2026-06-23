package verify

import (
	"context"
	"errors"
	"fmt"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/Eshpelin/calltoverify/coordinator/internal/store"
)

// noopNotifier satisfies Notifier; Start never notifies, so it is never called.
type noopNotifier struct{}

func (noopNotifier) VerificationVerified(store.Session, store.App) {}

// allowAllLimiter satisfies Limiter; Start does not consult the limiter.
type allowAllLimiter struct{}

func (allowAllLimiter) Allow(string) bool { return true }

// verifyBackends returns the store implementations to exercise. SQLite (in-memory)
// always runs; Postgres runs only when CTV_TEST_DATABASE_URL is set. SQLite
// serialises writers (one connection), so the real TOCTOU window is exercised on
// the Postgres backend, where concurrent Start calls run truly in parallel.
func verifyBackends(t *testing.T) map[string]func(t *testing.T) store.Store {
	t.Helper()
	m := map[string]func(t *testing.T) store.Store{
		"sqlite": func(t *testing.T) store.Store {
			st, err := store.NewSQLite(":memory:")
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
		m["postgres"] = func(t *testing.T) store.Store {
			st, err := store.NewPostgres(context.Background(), dsn)
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

// seedOneVoiceNumber creates an app + one online device + one number that serves
// the given voice channel, returning the app to start verifications against.
func seedOneVoiceNumber(t *testing.T, st store.Store, channel, msisdn string) store.App {
	t.Helper()
	ctx := context.Background()
	app, err := st.CreateApp(ctx, store.App{Name: "conc", APIKeyHash: "h-" + msisdn, APIKeyPrefix: "p", WebhookSecret: "w"})
	if err != nil {
		t.Fatalf("CreateApp: %v", err)
	}
	dev, err := st.CreateDevice(ctx, store.Device{AppID: app.ID, Name: "d", DeviceSecret: "s", Type: "pi", Capabilities: []string{channel}})
	if err != nil {
		t.Fatalf("CreateDevice: %v", err)
	}
	if err := st.SetHeartbeat(ctx, dev.ID); err != nil { // brings the device online
		t.Fatalf("SetHeartbeat: %v", err)
	}
	if _, err := st.CreateNumber(ctx, store.Number{DeviceID: dev.ID, MSISDN: msisdn, Channels: []string{channel}}); err != nil {
		t.Fatalf("CreateNumber: %v", err)
	}
	return app
}

// TestStartVoiceConcurrencyOneWinner is the regression test for the pick/create
// TOCTOU race: PickNumber and CreateSession are separate statements, so without a
// DB-level guard two concurrent voice starts on the same SIM could both pass the
// pick and both create a pending voice session, breaking the one-call-per-SIM
// guarantee. With the unique partial index in place plus Start's re-pick-on-
// conflict loop, exactly one of N concurrent starts wins and the rest get a
// BusyError, and the store ends with a single pending voice session on the SIM.
func TestStartVoiceConcurrencyOneWinner(t *testing.T) {
	for name, open := range verifyBackends(t) {
		t.Run(name, func(t *testing.T) {
			st := open(t)
			app := seedOneVoiceNumber(t, st, "call", "+8801700000010")
			svc := NewService(st, noopNotifier{}, allowAllLimiter{}, 6, time.Minute, nil)

			const n = 32
			results := make([]error, n)
			var wg sync.WaitGroup
			start := make(chan struct{}) // release all goroutines at once to maximise contention
			for i := 0; i < n; i++ {
				wg.Add(1)
				go func(i int) {
					defer wg.Done()
					<-start
					_, err := svc.Start(context.Background(), app, StartRequest{
						Channel:       "call",
						BindingMode:   "claim",
						ClaimedMSISDN: fmt.Sprintf("+88017111%05d", i),
					})
					results[i] = err
				}(i)
			}
			close(start)
			wg.Wait()

			var wins, busy int
			for i, err := range results {
				switch {
				case err == nil:
					wins++
				case isBusy(err):
					busy++
				default:
					t.Errorf("goroutine %d: unexpected error %v", i, err)
				}
			}
			if wins != 1 {
				t.Fatalf("wins = %d, want exactly 1", wins)
			}
			if busy != n-1 {
				t.Fatalf("busy = %d, want %d", busy, n-1)
			}

			// The SIM must hold exactly one pending voice session.
			if pv := pendingVoiceCount(t, st, app.ID); pv != 1 {
				t.Fatalf("pending voice sessions = %d, want 1", pv)
			}
			// And it now reads as busy, not free: a further pick finds nothing while
			// the line still counts as available capacity.
			if _, err := st.PickNumber(context.Background(), "call"); !errors.Is(err, store.ErrNotFound) {
				t.Fatalf("PickNumber(call) after winner: err=%v, want ErrNotFound", err)
			}
			if avail, err := st.CountAvailableNumbers(context.Background(), "call"); err != nil || avail != 1 {
				t.Fatalf("CountAvailableNumbers(call) = %d err %v, want 1", avail, err)
			}
		})
	}
}

func isBusy(err error) bool {
	var be *BusyError
	return errors.As(err, &be)
}

func pendingVoiceCount(t *testing.T, st store.Store, appID string) int {
	t.Helper()
	sessions, err := st.ListRecentSessions(context.Background(), appID, 1000)
	if err != nil {
		t.Fatalf("ListRecentSessions: %v", err)
	}
	var c int
	for _, s := range sessions {
		if s.Status == "pending" && (s.Channel == "call" || s.Channel == "dtmf") {
			c++
		}
	}
	return c
}
