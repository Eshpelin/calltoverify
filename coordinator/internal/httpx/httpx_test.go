package httpx

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestWriteErr(t *testing.T) {
	rr := httptest.NewRecorder()
	WriteErr(rr, http.StatusBadRequest, "bad_request", "nope")
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("code = %d, want 400", rr.Code)
	}
	if ct := rr.Header().Get("Content-Type"); ct != "application/json" {
		t.Fatalf("content-type = %q", ct)
	}
	var m map[string]string
	if err := json.Unmarshal(rr.Body.Bytes(), &m); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if m["error"] != "bad_request" || m["detail"] != "nope" {
		t.Fatalf("envelope = %v", m)
	}
}

func TestWriteJSON(t *testing.T) {
	rr := httptest.NewRecorder()
	WriteJSON(rr, http.StatusCreated, map[string]int{"n": 1})
	if rr.Code != http.StatusCreated {
		t.Fatalf("code = %d, want 201", rr.Code)
	}
	var m map[string]int
	if err := json.Unmarshal(rr.Body.Bytes(), &m); err != nil || m["n"] != 1 {
		t.Fatalf("body = %s err %v", rr.Body.String(), err)
	}
}
