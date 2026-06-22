// Command dashboard is a self-hosted console for CallToVerify: a guided
// onboarding wizard (pair a phone, run a test verification, copy the integration
// snippet) plus an ops dashboard of receivers and recent verifications. It embeds
// the engine, so it needs no separate service and no database (SQLite default).
//
//	go run ./examples/dashboard      # then open http://localhost:8080
package main

import (
	"context"
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"

	ctv "github.com/Eshpelin/calltoverify/coordinator/engine"
)

//go:embed index.html
var indexHTML []byte

func main() {
	ctx := context.Background()
	eng, err := ctv.New(ctx, ctv.Options{
		SQLitePath: "calltoverify.db",
		OnVerified: func(ev ctv.Event) { log.Printf("verified %s via %s", ev.VerifiedMSISDN, ev.Channel) },
	})
	if err != nil {
		log.Fatal(err)
	}
	defer eng.Close()

	mux := http.NewServeMux()

	// The receiver app posts here (pair it with the QR from the wizard).
	mux.Handle("/ctv/", eng.DeviceHandler("/ctv"))

	// Root catch-all serves the console SPA (also at /setup, the guided onboarding
	// entry point). Registered without a method so it does not conflict with the
	// all-method "/ctv/" subtree.
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" && r.URL.Path != "/setup" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write(indexHTML)
	})

	mux.HandleFunc("POST /api/pair", func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			Name     string
			Type     string
			MSISDN   string
			Channels []string
		}
		_ = json.NewDecoder(r.Body).Decode(&body)
		p, err := eng.NewPairing(ctx, ctv.PairingParams{
			Endpoint: "http://" + r.Host + "/ctv", Name: body.Name, Type: body.Type, MSISDN: body.MSISDN, Channels: body.Channels,
		})
		if err != nil {
			httpErr(w, http.StatusInternalServerError, err)
			return
		}
		writeJSON(w, map[string]string{"payload": p.QRPayload, "device_id": p.DeviceID})
	})

	mux.HandleFunc("POST /api/start", func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			Channel       string
			BindingMode   string
			ClaimedMSISDN string
		}
		_ = json.NewDecoder(r.Body).Decode(&body)
		v, err := eng.StartVerification(ctx, ctv.Params{
			Channel: body.Channel, BindingMode: body.BindingMode, ClaimedMSISDN: body.ClaimedMSISDN,
		})
		if err != nil {
			httpErr(w, http.StatusServiceUnavailable, err)
			return
		}
		writeJSON(w, v)
	})

	mux.HandleFunc("GET /api/status", func(w http.ResponseWriter, r *http.Request) {
		st, err := eng.Status(ctx, r.URL.Query().Get("id"))
		if err != nil {
			httpErr(w, http.StatusNotFound, err)
			return
		}
		writeJSON(w, st)
	})

	mux.HandleFunc("GET /api/devices", func(w http.ResponseWriter, r *http.Request) {
		d, err := eng.Devices(ctx)
		if err != nil {
			httpErr(w, http.StatusInternalServerError, err)
			return
		}
		writeJSON(w, d)
	})

	mux.HandleFunc("DELETE /api/devices", func(w http.ResponseWriter, r *http.Request) {
		id := r.URL.Query().Get("id")
		if id == "" {
			httpErr(w, http.StatusBadRequest, fmt.Errorf("missing id"))
			return
		}
		if err := eng.RemoveDevice(ctx, id); err != nil {
			if errors.Is(err, ctv.ErrNotFound) {
				httpErr(w, http.StatusNotFound, err)
				return
			}
			httpErr(w, http.StatusInternalServerError, err)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	})

	mux.HandleFunc("GET /api/sessions", func(w http.ResponseWriter, r *http.Request) {
		s, err := eng.Sessions(ctx, 20)
		if err != nil {
			httpErr(w, http.StatusInternalServerError, err)
			return
		}
		writeJSON(w, s)
	})

	addr := os.Getenv("CTV_EXAMPLE_ADDR")
	if addr == "" {
		addr = ":8080"
	}
	log.Printf("CallToVerify console on http://localhost%s", addr)
	log.Fatal(http.ListenAndServe(addr, mux))
}

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(v)
}

func httpErr(w http.ResponseWriter, code int, err error) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
}
