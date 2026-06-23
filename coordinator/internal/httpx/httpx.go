// Package httpx holds small HTTP helpers shared by the developer-facing API and
// the device-facing handlers: a JSON writer and one consistent error envelope, so
// the wire shape lives in a single place.
package httpx

import (
	"encoding/json"
	"net/http"
)

// MaxBodyBytes caps the request body a handler will read (1 MiB).
const MaxBodyBytes = 1 << 20

// WriteJSON writes v as a JSON response with the given status code.
func WriteJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

// WriteErr writes the standard {"error","detail"} envelope with the given status.
func WriteErr(w http.ResponseWriter, status int, code, detail string) {
	WriteJSON(w, status, map[string]string{"error": code, "detail": detail})
}
