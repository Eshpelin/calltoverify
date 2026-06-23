package webhook

import (
	"net"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestIsDisallowedIP(t *testing.T) {
	cases := map[string]bool{
		"127.0.0.1":       true, // loopback
		"::1":             true,
		"10.1.2.3":        true, // RFC1918
		"192.168.1.1":     true,
		"172.16.0.1":      true,
		"169.254.169.254": true, // cloud metadata (link-local)
		"0.0.0.0":         true, // unspecified
		"fc00::1":         true, // ULA
		"8.8.8.8":         false,
		"1.1.1.1":         false,
		"93.184.216.34":   false,
	}
	for s, want := range cases {
		ip := net.ParseIP(s)
		if ip == nil {
			t.Fatalf("bad test ip %q", s)
		}
		if got := isDisallowedIP(ip); got != want {
			t.Errorf("isDisallowedIP(%s) = %v, want %v", s, got, want)
		}
	}
}

func TestValidateURL(t *testing.T) {
	good := []string{"https://example.com/hook", "http://example.com:8080/x"}
	bad := []string{"ftp://example.com", "file:///etc/passwd", "https://", "notaurl", ""}
	for _, u := range good {
		if err := ValidateURL(u); err != nil {
			t.Errorf("ValidateURL(%q) = %v, want nil", u, err)
		}
	}
	for _, u := range bad {
		if err := ValidateURL(u); err == nil {
			t.Errorf("ValidateURL(%q) = nil, want error", u)
		}
	}
}

// TestSafeClientBlocksLoopback proves the SSRF guard: webhook delivery to a
// loopback address is refused by default, and reachable only with the explicit
// allowPrivate opt-out.
func TestSafeClientBlocksLoopback(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	if _, err := safeClient(false).Get(srv.URL); err == nil {
		t.Fatalf("safeClient(false) reached loopback %s; expected an SSRF block", srv.URL)
	}

	resp, err := safeClient(true).Get(srv.URL)
	if err != nil {
		t.Fatalf("safeClient(true) should reach loopback for self-host opt-out: %v", err)
	}
	_ = resp.Body.Close()
}
