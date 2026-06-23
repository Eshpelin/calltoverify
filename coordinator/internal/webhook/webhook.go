// Package webhook delivers signed verification events to developer backends.
package webhook

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"net/url"
	"time"

	"github.com/Eshpelin/calltoverify/coordinator/internal/auth"
	"github.com/Eshpelin/calltoverify/coordinator/internal/store"
)

// Sender posts signed webhooks. It satisfies verify.Notifier.
type Sender struct {
	client *http.Client
	logger *slog.Logger
}

// New builds a webhook sender. Unless allowPrivate is true, delivery refuses to
// connect to loopback/private/link-local addresses (SSRF defense: webhook_url is
// developer-supplied and the coordinator dereferences it from inside its own
// network). It also never follows redirects.
func New(logger *slog.Logger, allowPrivate bool) *Sender {
	return &Sender{
		client: safeClient(allowPrivate),
		logger: logger,
	}
}

// ValidateURL is a cheap, write-time check on a developer-supplied webhook URL.
// The authoritative SSRF guard is the dial-time IP check in safeClient; this just
// rejects obviously-wrong values early with a clear message.
func ValidateURL(raw string) error {
	u, err := url.Parse(raw)
	if err != nil {
		return fmt.Errorf("not a valid URL")
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return fmt.Errorf("scheme must be http or https")
	}
	if u.Hostname() == "" {
		return fmt.Errorf("missing host")
	}
	return nil
}

// safeClient returns an HTTP client whose dialer resolves the target and refuses
// to connect to non-public addresses (defeating SSRF and DNS-rebinding by dialing
// the exact IP it validated), and which does not follow redirects.
func safeClient(allowPrivate bool) *http.Client {
	dialer := &net.Dialer{Timeout: 5 * time.Second}
	tr := &http.Transport{
		ForceAttemptHTTP2: true,
		DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
			host, port, err := net.SplitHostPort(addr)
			if err != nil {
				return nil, err
			}
			ips, err := net.DefaultResolver.LookupIPAddr(ctx, host)
			if err != nil || len(ips) == 0 {
				return nil, fmt.Errorf("webhook: cannot resolve %s", host)
			}
			ip := ips[0].IP
			if !allowPrivate && isDisallowedIP(ip) {
				return nil, fmt.Errorf("webhook: refusing to connect to non-public address %s", ip)
			}
			// Dial the exact IP we validated (TLS still verifies the original host).
			return dialer.DialContext(ctx, network, net.JoinHostPort(ip.String(), port))
		},
	}
	return &http.Client{
		Timeout:   10 * time.Second,
		Transport: tr,
		// Do not follow redirects — a 3xx to an internal address would bypass the
		// dial-time check on the original URL.
		CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
	}
}

// isDisallowedIP reports whether ip is a non-public address a webhook must not
// reach: loopback, RFC1918/ULA private, link-local (incl. 169.254.169.254 cloud
// metadata), multicast, or unspecified.
func isDisallowedIP(ip net.IP) bool {
	return ip.IsLoopback() || ip.IsPrivate() || ip.IsLinkLocalUnicast() ||
		ip.IsLinkLocalMulticast() || ip.IsInterfaceLocalMulticast() ||
		ip.IsMulticast() || ip.IsUnspecified()
}

type payload struct {
	Event          string `json:"event"`
	SessionID      string `json:"session_id"`
	VerifiedMSISDN string `json:"verified_msisdn"`
	Channel        string `json:"channel"`
	Timestamp      string `json:"ts"`
}

// VerificationVerified fires a best-effort, signed webhook asynchronously.
func (s *Sender) VerificationVerified(sess store.Session, app store.App) {
	if app.WebhookURL == "" {
		return
	}
	verified := ""
	if sess.VerifiedMSISDN != nil {
		verified = *sess.VerifiedMSISDN
	}
	body, err := json.Marshal(payload{
		Event:          "verification.verified",
		SessionID:      sess.ID,
		VerifiedMSISDN: verified,
		Channel:        sess.Channel,
		Timestamp:      time.Now().UTC().Format(time.RFC3339),
	})
	if err != nil {
		s.logger.Error("webhook marshal", "err", err)
		return
	}
	sig := auth.HMACHex(app.WebhookSecret, body)
	go s.deliver(app.WebhookURL, body, sig, sess.ID)
}

func (s *Sender) deliver(url string, body []byte, sig, sessionID string) {
	backoff := 500 * time.Millisecond
	for attempt := 1; attempt <= 3; attempt++ {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
		if err != nil {
			cancel()
			s.logger.Error("webhook request", "err", err, "session", sessionID)
			return
		}
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-CTV-Event", "verification.verified")
		req.Header.Set("X-CTV-Signature", sig)

		resp, err := s.client.Do(req)
		cancel()
		if err == nil {
			_ = resp.Body.Close()
			if resp.StatusCode < 300 {
				return
			}
			err = errorStatus(resp.StatusCode)
		}
		s.logger.Warn("webhook delivery failed", "attempt", attempt, "session", sessionID, "err", err)
		if attempt < 3 {
			time.Sleep(backoff)
			backoff *= 2
		}
	}
	s.logger.Error("webhook gave up", "session", sessionID)
}

type errorStatus int

func (e errorStatus) Error() string { return "non-2xx status: " + http.StatusText(int(e)) }
