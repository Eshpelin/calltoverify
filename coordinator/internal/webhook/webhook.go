// Package webhook delivers signed verification events to developer backends.
package webhook

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"time"

	"github.com/Eshpelin/calltoverify/coordinator/internal/auth"
	"github.com/Eshpelin/calltoverify/coordinator/internal/store"
)

// Sender posts signed webhooks. It satisfies verify.Notifier.
type Sender struct {
	client *http.Client
	logger *slog.Logger
}

func New(logger *slog.Logger) *Sender {
	return &Sender{
		client: &http.Client{Timeout: 10 * time.Second},
		logger: logger,
	}
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
