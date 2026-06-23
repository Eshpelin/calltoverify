package main

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

// TestLimitInFlight verifies the concurrency cap: with one slot, a request that
// is in-flight holds the slot and a second concurrent request is rejected with 503.
func TestLimitInFlight(t *testing.T) {
	entered := make(chan struct{})
	release := make(chan struct{})
	h := limitInFlight(1, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		entered <- struct{}{}
		<-release
		w.WriteHeader(http.StatusOK)
	}))

	rr1 := httptest.NewRecorder()
	go h.ServeHTTP(rr1, httptest.NewRequest(http.MethodGet, "/", nil))
	<-entered // the first request now holds the only slot

	rr2 := httptest.NewRecorder()
	h.ServeHTTP(rr2, httptest.NewRequest(http.MethodGet, "/", nil))
	if rr2.Code != http.StatusServiceUnavailable {
		t.Fatalf("second request over the cap should be 503, got %d", rr2.Code)
	}

	close(release)
}

// TestLimitInFlightZeroIsPassthrough: max<=0 disables the cap.
func TestLimitInFlightZeroIsPassthrough(t *testing.T) {
	h := limitInFlight(0, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusOK) }))
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/", nil))
	if rr.Code != http.StatusOK {
		t.Fatalf("passthrough should be 200, got %d", rr.Code)
	}
}
