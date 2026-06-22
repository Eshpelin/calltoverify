// Package verify holds the Coordinator's domain logic: starting verifications
// (number-pool selection, code generation, channel/binding rules) and matching
// inbound signals to live sessions.
package verify

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/Eshpelin/calltoverify/coordinator/internal/store"
)

// ValidationError is a client-fault error mapped to HTTP 400.
type ValidationError struct {
	Field   string
	Message string
}

func (e *ValidationError) Error() string { return e.Field + ": " + e.Message }

// ErrNoCapacity means no online number could serve the requested channel.
var ErrNoCapacity = errors.New("no available number for channel")

// BusyError means online voice numbers exist but all are mid-verification (a SIM
// handles one voice call at a time). Position is how many voice lines are busy.
type BusyError struct {
	Position int
}

func (e *BusyError) Error() string {
	return fmt.Sprintf("all voice lines are busy (%d in progress); retry shortly", e.Position)
}

// ErrForbidden means the device tried to act on a number it does not own.
var ErrForbidden = errors.New("device does not own this number")

// Notifier is told when a session becomes verified (implemented by the webhook sender).
type Notifier interface {
	VerificationVerified(sess store.Session, app store.App)
}

// Limiter gates inbound by key (for example, sender MSISDN).
type Limiter interface {
	Allow(key string) bool
}

type Service struct {
	store          store.Store
	notifier       Notifier
	limiter        Limiter
	defaultCodeLen int
	defaultTTL     time.Duration
}

func NewService(st store.Store, n Notifier, l Limiter, codeLen int, ttl time.Duration) *Service {
	return &Service{store: st, notifier: n, limiter: l, defaultCodeLen: codeLen, defaultTTL: ttl}
}

// --- start ---

type StartRequest struct {
	Channel       string
	BindingMode   string
	ClaimedMSISDN string
}

type Instructions struct {
	Number    string    `json:"number"`
	Code      string    `json:"code,omitempty"`
	Channel   string    `json:"channel"`
	Action    string    `json:"action"`
	DeepLink  string    `json:"deep_link"`
	ExpiresAt time.Time `json:"expires_at"`
}

type StartResult struct {
	SessionID    string       `json:"session_id"`
	Status       string       `json:"status"`
	Instructions Instructions `json:"instructions"`
}

// Start creates a pending verification and returns the user-facing instructions.
func (s *Service) Start(ctx context.Context, app store.App, req StartRequest) (StartResult, error) {
	channel := req.Channel
	if channel == "" {
		channel = "sms"
	}
	binding := req.BindingMode
	if binding == "" {
		binding = app.Config.BindingMode
	}
	if binding == "" {
		binding = "derive"
	}
	if err := validateCombo(channel, binding); err != nil {
		return StartResult{}, err
	}
	if len(app.Config.ChannelsEnabled) > 0 && !contains(app.Config.ChannelsEnabled, channel) {
		return StartResult{}, &ValidationError{Field: "channel", Message: "not enabled for this app"}
	}

	claimed := strings.TrimSpace(req.ClaimedMSISDN)
	if binding == "claim" && claimed == "" {
		return StartResult{}, &ValidationError{Field: "claimed_msisdn", Message: "required for claim binding"}
	}

	num, err := s.store.PickNumber(ctx, channel)
	if errors.Is(err, store.ErrNotFound) {
		// For voice channels, distinguish "all lines busy" (queue) from "no capacity".
		if channel == "call" || channel == "dtmf" {
			if avail, _ := s.store.CountAvailableNumbers(ctx, channel); avail > 0 {
				return StartResult{}, &BusyError{Position: avail}
			}
		}
		return StartResult{}, ErrNoCapacity
	}
	if err != nil {
		return StartResult{}, err
	}

	codeLen := app.Config.CodeLen
	if codeLen == 0 {
		codeLen = s.defaultCodeLen
	}
	ttl := s.defaultTTL
	if app.Config.TTLSeconds > 0 {
		ttl = time.Duration(app.Config.TTLSeconds) * time.Second
	}

	base := store.Session{
		AppID:       app.ID,
		Channel:     channel,
		BindingMode: binding,
		NumberID:    &num.ID,
		ExpiresAt:   time.Now().Add(ttl),
	}
	if claimed != "" {
		base.ClaimedMSISDN = &claimed
	}

	var sess store.Session
	var code string
	if channelNeedsCode(channel) {
		// Retry on the rare code collision with another pending session on this number.
		for attempt := 0; attempt < 6; attempt++ {
			code, err = generateCode(codeLen)
			if err != nil {
				return StartResult{}, err
			}
			candidate := base
			candidate.Code = &code
			sess, err = s.store.CreateSession(ctx, candidate)
			if err == nil {
				break
			}
			if errors.Is(err, store.ErrConflict) {
				continue
			}
			return StartResult{}, err
		}
		if err != nil {
			return StartResult{}, err
		}
	} else {
		sess, err = s.store.CreateSession(ctx, base)
		if err != nil {
			return StartResult{}, err
		}
	}

	action, deepLink := buildInstructions(channel, num.MSISDN, code)
	return StartResult{
		SessionID: sess.ID,
		Status:    sess.Status,
		Instructions: Instructions{
			Number:    num.MSISDN,
			Code:      code,
			Channel:   channel,
			Action:    action,
			DeepLink:  deepLink,
			ExpiresAt: sess.ExpiresAt,
		},
	}, nil
}

// --- inbound ---

type InboundRequest struct {
	Number string // the receiver's own MSISDN
	Type   string // sms | call
	Sender string
	Body   string
}

type InboundResult struct {
	Matched   bool   `json:"matched"`
	SessionID string `json:"session_id,omitempty"`
	Reason    string `json:"reason,omitempty"`
}

// Inbound matches a reported inbound signal to a live session and verifies it.
func (s *Service) Inbound(ctx context.Context, device store.Device, req InboundRequest) (InboundResult, error) {
	if req.Type != "sms" && req.Type != "call" {
		return InboundResult{}, &ValidationError{Field: "type", Message: "must be sms or call"}
	}

	num, err := s.store.GetNumberByMSISDN(ctx, req.Number)
	if errors.Is(err, store.ErrNotFound) {
		return InboundResult{Matched: false, Reason: "unknown_number"}, nil
	}
	if err != nil {
		return InboundResult{}, err
	}
	if num.DeviceID != device.ID {
		return InboundResult{}, ErrForbidden
	}

	// From here on every outcome is recorded as an inbound_event for audit.
	var matchedID *string
	defer func() {
		_ = s.store.CreateInboundEvent(ctx, store.InboundEvent{
			NumberID:         num.ID,
			Type:             req.Type,
			Sender:           req.Sender,
			Body:             req.Body,
			MatchedSessionID: matchedID,
		})
	}()

	if blocked, _ := s.store.IsBlocked(ctx, req.Sender); blocked {
		return InboundResult{Matched: false, Reason: "blocked"}, nil
	}
	if !s.limiter.Allow("inbound:" + req.Sender) {
		return InboundResult{Matched: false, Reason: "rate_limited"}, nil
	}

	sess, err := s.matchSession(ctx, num.ID, req)
	if errors.Is(err, store.ErrNotFound) {
		return InboundResult{Matched: false, Reason: "no_match"}, nil
	}
	if err != nil {
		return InboundResult{}, err
	}

	// Apply binding.
	if sess.BindingMode == "claim" {
		if sess.ClaimedMSISDN == nil || *sess.ClaimedMSISDN != req.Sender {
			_ = s.store.IncrementAttempts(ctx, sess.ID)
			return InboundResult{Matched: false, Reason: "claim_mismatch"}, nil
		}
	}
	verified := req.Sender

	ok, err := s.store.VerifySession(ctx, sess.ID, verified)
	if err != nil {
		return InboundResult{}, err
	}
	if !ok {
		// Lost the race; another inbound already resolved it.
		return InboundResult{Matched: false, Reason: "already_resolved"}, nil
	}

	matchedID = &sess.ID
	sess.Status = "verified"
	sess.VerifiedMSISDN = &verified
	if app, err := s.store.GetAppByID(ctx, sess.AppID); err == nil {
		s.notifier.VerificationVerified(sess, app)
	}
	return InboundResult{Matched: true, SessionID: sess.ID}, nil
}

// matchSession resolves the live session an inbound signal targets:
//   - sms with a code -> sms session by code
//   - call with digits -> dtmf session by code
//   - call without digits -> missed-call session by caller ID
func (s *Service) matchSession(ctx context.Context, numberID string, req InboundRequest) (store.Session, error) {
	if req.Type == "sms" {
		code := extractCode(req.Body)
		if code == "" {
			return store.Session{}, store.ErrNotFound
		}
		return s.store.FindPendingByCode(ctx, numberID, code, "sms")
	}
	// req.Type == "call"
	if strings.TrimSpace(req.Body) != "" {
		code := extractCode(req.Body)
		if code == "" {
			return store.Session{}, store.ErrNotFound
		}
		return s.store.FindPendingByCode(ctx, numberID, code, "dtmf")
	}
	return s.store.FindPendingMissedCall(ctx, numberID, req.Sender)
}

func contains(ss []string, v string) bool {
	for _, s := range ss {
		if s == v {
			return true
		}
	}
	return false
}
