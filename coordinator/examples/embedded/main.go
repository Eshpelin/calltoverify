// Command embedded is a minimal backend that embeds the CallToVerify engine.
// It needs no separate service and no external database: SQLite is the default.
//
//	go run ./examples/embedded
//	# pair a phone (returns the QR payload the Android app scans):
//	curl 'http://localhost:8080/pair?name=my-phone&msisdn=%2B8801700000001&channels=sms'
//	# start a verification:
//	curl -X POST 'http://localhost:8080/start?channel=sms'
//	# check status:
//	curl 'http://localhost:8080/status?id=SESSION_ID'
package main

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"os"
	"strings"

	ctv "github.com/Eshpelin/calltoverify/coordinator/engine"
)

func main() {
	ctx := context.Background()

	// Address to listen on (default :8080). Overridable so multiple instances
	// (or CI's e2e) can run without colliding on a fixed port.
	addr := os.Getenv("CTV_EXAMPLE_ADDR")
	if addr == "" {
		addr = ":8080"
	}
	// The public base URL the receiver app reaches; defaults to the listen addr.
	baseURL := os.Getenv("CTV_EXAMPLE_BASE_URL")
	if baseURL == "" {
		baseURL = "http://localhost" + addr
	}

	eng, err := ctv.New(ctx, ctv.Options{
		SQLitePath: "calltoverify.db",
		OnVerified: func(ev ctv.Event) {
			log.Printf("verified: %s via %s (session %s)", ev.VerifiedMSISDN, ev.Channel, ev.SessionID)
		},
	})
	if err != nil {
		log.Fatal(err)
	}
	defer eng.Close()

	mux := http.NewServeMux()

	// The receiver app (paired via /pair) posts inbound events here.
	mux.Handle("/ctv/", eng.DeviceHandler("/ctv"))

	// Admin-only in a real app: enroll a phone and return the QR payload to scan.
	mux.HandleFunc("GET /pair", func(w http.ResponseWriter, r *http.Request) {
		p, err := eng.NewPairing(ctx, ctv.PairingParams{
			Endpoint: baseURL + "/ctv",
			Name:     r.URL.Query().Get("name"),
			MSISDN:   r.URL.Query().Get("msisdn"),
			Channels: strings.Split(r.URL.Query().Get("channels"), ","),
		})
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		writeJSON(w, map[string]string{"scan_this": p.QRPayload})
	})

	// Call this from your signup flow, then show the instructions to the user.
	mux.HandleFunc("POST /start", func(w http.ResponseWriter, r *http.Request) {
		v, err := eng.StartVerification(ctx, ctv.Params{
			Channel:       r.URL.Query().Get("channel"),
			BindingMode:   r.URL.Query().Get("binding_mode"),
			ClaimedMSISDN: r.URL.Query().Get("claimed_msisdn"),
		})
		if err != nil {
			http.Error(w, err.Error(), http.StatusServiceUnavailable)
			return
		}
		writeJSON(w, v)
	})

	mux.HandleFunc("GET /status", func(w http.ResponseWriter, r *http.Request) {
		st, err := eng.Status(ctx, r.URL.Query().Get("id"))
		if err != nil {
			http.Error(w, err.Error(), http.StatusNotFound)
			return
		}
		writeJSON(w, st)
	})

	log.Printf("embedded CallToVerify example listening on %s", addr)
	log.Fatal(http.ListenAndServe(addr, mux))
}

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(v)
}
