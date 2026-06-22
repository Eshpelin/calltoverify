package engine

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/Eshpelin/calltoverify/coordinator/internal/store"
)

// TestVoiceConcurrencyRacePostgres drives many concurrent voice starts against
// one SIM on Postgres (real concurrent connections). The partial unique index must
// let exactly one succeed; the rest get BusyError. Gated on CTV_TEST_DATABASE_URL.
func TestVoiceConcurrencyRacePostgres(t *testing.T) {
	dsn := os.Getenv("CTV_TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("set CTV_TEST_DATABASE_URL to run the Postgres voice-race test")
	}
	ctx := context.Background()

	// Reset the database for an isolated run.
	st, err := store.NewPostgres(ctx, dsn)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	if err := st.Migrate(ctx); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	if err := st.Reset(ctx); err != nil {
		t.Fatalf("reset: %v", err)
	}
	st.Close()

	eng, err := New(ctx, Options{PostgresDSN: dsn})
	if err != nil {
		t.Fatalf("engine: %v", err)
	}
	defer eng.Close()

	mux := http.NewServeMux()
	mux.Handle("/ctv/", eng.DeviceHandler("/ctv"))
	ts := httptest.NewServer(mux)
	defer ts.Close()

	pairing, err := eng.NewPairing(ctx, PairingParams{
		Endpoint: ts.URL + "/ctv", Name: "phone", MSISDN: "+8801700009999", Channels: []string{"call"},
	})
	if err != nil {
		t.Fatal(err)
	}
	deviceReq(t, ts, pairing, "/ctv/devices/heartbeat", map[string]any{})

	const n = 12
	var success, busy int32
	var wg sync.WaitGroup
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			_, err := eng.StartVerification(ctx, Params{
				Channel: "call", BindingMode: "claim", ClaimedMSISDN: fmt.Sprintf("+8801700%06d", i),
			})
			if err == nil {
				atomic.AddInt32(&success, 1)
				return
			}
			var be *BusyError
			if errors.As(err, &be) {
				atomic.AddInt32(&busy, 1)
			} else {
				t.Errorf("unexpected error: %v", err)
			}
		}(i)
	}
	wg.Wait()

	if success != 1 {
		t.Fatalf("exactly one concurrent voice start should succeed, got success=%d busy=%d", success, busy)
	}
	if busy != n-1 {
		t.Fatalf("the rest should be busy, got success=%d busy=%d", success, busy)
	}
}
