// Package engine is the embeddable CallToVerify control plane. Add it to your own
// Go backend, mount DeviceHandler for the receiver app to reach, and call
// StartVerification / Status in-process. SQLite is the zero-infra default.
//
//	eng, _ := engine.New(ctx, engine.Options{
//	    OnVerified: func(ev engine.Event) { /* mark the user verified */ },
//	})
//	mux.Handle("/ctv/", eng.DeviceHandler("/ctv"))
//	v, _ := eng.StartVerification(ctx, engine.Params{Channel: "sms"})
package engine

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/Eshpelin/calltoverify/coordinator/internal/auth"
	"github.com/Eshpelin/calltoverify/coordinator/internal/deviceapi"
	"github.com/Eshpelin/calltoverify/coordinator/internal/ratelimit"
	"github.com/Eshpelin/calltoverify/coordinator/internal/redisutil"
	"github.com/Eshpelin/calltoverify/coordinator/internal/store"
	"github.com/Eshpelin/calltoverify/coordinator/internal/verify"
)

// embeddedAppID is the fixed single-tenant app the embedded engine uses.
const embeddedAppID = "00000000-0000-4000-8000-000000000001"

// ErrNoCapacity means no online number could serve the requested channel.
var ErrNoCapacity = errors.New("no available number for channel")

// ErrNotFound means the verification does not exist.
var ErrNotFound = errors.New("not found")

// BusyError means all voice lines are mid-verification (one voice call per SIM).
// Position is how many are in progress. Retry shortly.
type BusyError struct {
	Position int
}

func (e *BusyError) Error() string {
	return fmt.Sprintf("all voice lines are busy (%d in progress); retry shortly", e.Position)
}

// Error is a client-fault error (for example, an invalid channel/binding combo).
type Error struct {
	Code    string
	Message string
}

func (e *Error) Error() string { return e.Code + ": " + e.Message }

// Options configures the engine. Leave PostgresDSN empty to use SQLite.
type Options struct {
	SQLitePath  string         // SQLite file path (default "calltoverify.db")
	PostgresDSN string         // if set, use Postgres instead of SQLite
	RedisURL    string         // optional; share rate-limit + nonce across instances
	OnVerified  func(ev Event) // called when a verification succeeds
	CodeLen     int            // verification code length (default 6)
	TTL         time.Duration  // session lifetime (default 90s)
	Logger      *slog.Logger   // optional; defaults to a no-op logger
}

// Event is delivered to OnVerified when a number is verified.
type Event struct {
	SessionID      string
	VerifiedMSISDN string
	Channel        string
}

// Params starts a verification.
type Params struct {
	Channel       string // sms | call | dtmf (default sms)
	BindingMode   string // derive | claim (default derive)
	ClaimedMSISDN string // required for claim binding
}

// Instructions is what to show the end user.
type Instructions struct {
	Number    string
	Code      string
	Channel   string
	Action    string
	DeepLink  string
	ExpiresAt time.Time
}

// Result is returned by StartVerification.
type Result struct {
	SessionID    string
	Status       string
	Instructions Instructions
}

// Status is returned by Status.
type Status struct {
	SessionID      string
	Status         string
	Channel        string
	VerifiedMSISDN string
	ExpiresAt      time.Time
}

// PairingParams describes a receiver to enroll.
type PairingParams struct {
	Endpoint     string   // public URL where DeviceHandler is mounted, e.g. https://app.example.com/ctv
	Name         string   // device label
	Type         string   // android | pi (default android)
	MSISDN       string   // the SIM's phone number
	Channels     []string // sms, call, dtmf this number serves
	Capabilities []string // device capabilities (defaults to Channels)
}

// Pairing is the result of NewPairing. QRPayload is the JSON the receiver app scans.
type Pairing struct {
	DeviceID     string
	DeviceSecret string
	Endpoint     string
	QRPayload    string
}

type Engine struct {
	store  store.Store
	svc    *verify.Service
	device *deviceapi.Handler
	app    store.App
	logger *slog.Logger
}

// New builds an engine, opening (and migrating) its store.
func New(ctx context.Context, opts Options) (*Engine, error) {
	logger := opts.Logger
	if logger == nil {
		logger = slog.New(slog.NewTextHandler(io.Discard, nil))
	}
	codeLen := opts.CodeLen
	if codeLen == 0 {
		codeLen = 6
	}
	ttl := opts.TTL
	if ttl == 0 {
		ttl = 90 * time.Second
	}

	var st store.Store
	var err error
	if opts.PostgresDSN != "" {
		st, err = store.NewPostgres(ctx, opts.PostgresDSN)
	} else {
		path := opts.SQLitePath
		if path == "" {
			path = "calltoverify.db"
		}
		st, err = store.NewSQLite(path)
	}
	if err != nil {
		return nil, err
	}
	if err := st.Migrate(ctx); err != nil {
		st.Close()
		return nil, err
	}

	whSecret, err := auth.GenerateSecret()
	if err != nil {
		st.Close()
		return nil, err
	}
	app, err := st.EnsureApp(ctx, store.App{
		ID:            embeddedAppID,
		Name:          "embedded",
		APIKeyHash:    auth.HashAPIKey("embedded-" + embeddedAppID),
		APIKeyPrefix:  "embedded",
		WebhookSecret: whSecret,
	})
	if err != nil {
		st.Close()
		return nil, err
	}

	var limiter verify.Limiter = ratelimit.New(60, 10)
	var nonces deviceapi.NonceStore = auth.NewNonceCache(10 * time.Minute)
	if opts.RedisURL != "" {
		if rc, rerr := redisutil.Connect(ctx, opts.RedisURL); rerr == nil {
			limiter = ratelimit.NewRedis(rc, 60)
			nonces = auth.NewRedisNonceCache(rc, 10*time.Minute)
			logger.Info("redis: sharing rate-limit + nonce across instances")
		} else {
			logger.Warn("redis unavailable; using in-process rate-limit + nonce", "err", rerr)
		}
	}

	svc := verify.NewService(st, callbackNotifier{cb: opts.OnVerified}, limiter, codeLen, ttl)
	device := deviceapi.New(st, svc, nonces, logger)
	return &Engine{store: st, svc: svc, device: device, app: app, logger: logger}, nil
}

// Close releases the engine's store.
func (e *Engine) Close() { e.store.Close() }

// DeviceHandler returns the receiver-facing HTTP handler, mounted at prefix.
//
//	mux.Handle("/ctv/", eng.DeviceHandler("/ctv"))
func (e *Engine) DeviceHandler(prefix string) http.Handler {
	return http.StripPrefix("/"+strings.Trim(prefix, "/"), e.device.Mux())
}

// StartVerification creates a verification and returns instructions for the user.
func (e *Engine) StartVerification(ctx context.Context, p Params) (Result, error) {
	res, err := e.svc.Start(ctx, e.app, verify.StartRequest{
		Channel: p.Channel, BindingMode: p.BindingMode, ClaimedMSISDN: p.ClaimedMSISDN,
	})
	if err != nil {
		var ve *verify.ValidationError
		var be *verify.BusyError
		switch {
		case errors.As(err, &ve):
			return Result{}, &Error{Code: "invalid", Message: ve.Error()}
		case errors.As(err, &be):
			return Result{}, &BusyError{Position: be.Position}
		case errors.Is(err, verify.ErrNoCapacity):
			return Result{}, ErrNoCapacity
		default:
			return Result{}, err
		}
	}
	return Result{
		SessionID: res.SessionID,
		Status:    res.Status,
		Instructions: Instructions{
			Number:    res.Instructions.Number,
			Code:      res.Instructions.Code,
			Channel:   res.Instructions.Channel,
			Action:    res.Instructions.Action,
			DeepLink:  res.Instructions.DeepLink,
			ExpiresAt: res.Instructions.ExpiresAt,
		},
	}, nil
}

// Status returns the current state of a verification.
func (e *Engine) Status(ctx context.Context, sessionID string) (Status, error) {
	sess, err := e.store.GetSession(ctx, e.app.ID, sessionID)
	if errors.Is(err, store.ErrNotFound) {
		return Status{}, ErrNotFound
	}
	if err != nil {
		return Status{}, err
	}
	out := Status{SessionID: sess.ID, Status: sess.Status, Channel: sess.Channel, ExpiresAt: sess.ExpiresAt}
	if sess.VerifiedMSISDN != nil {
		out.VerifiedMSISDN = *sess.VerifiedMSISDN
	}
	return out, nil
}

// DeviceInfo is a receiver's status for the ops dashboard.
type DeviceInfo struct {
	ID            string
	Name          string
	Type          string
	Status        string
	LastHeartbeat *time.Time
	Capabilities  []string
	Numbers       []string
}

// Devices lists enrolled receivers and their numbers.
func (e *Engine) Devices(ctx context.Context) ([]DeviceInfo, error) {
	devs, err := e.store.ListDevices(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]DeviceInfo, 0, len(devs))
	for _, d := range devs {
		nums, _ := e.store.ListNumbersByDevice(ctx, d.ID)
		msisdns := make([]string, 0, len(nums))
		for _, n := range nums {
			msisdns = append(msisdns, n.MSISDN)
		}
		out = append(out, DeviceInfo{
			ID: d.ID, Name: d.Name, Type: d.Type, Status: d.Status,
			LastHeartbeat: d.LastHeartbeat, Capabilities: d.Capabilities, Numbers: msisdns,
		})
	}
	return out, nil
}

// SessionInfo is a recent verification for the ops dashboard.
type SessionInfo struct {
	ID             string
	Channel        string
	Status         string
	VerifiedMSISDN string
	CreatedAt      time.Time
}

// Sessions returns the most recent verifications (newest first).
func (e *Engine) Sessions(ctx context.Context, limit int) ([]SessionInfo, error) {
	if limit <= 0 {
		limit = 20
	}
	ss, err := e.store.ListRecentSessions(ctx, e.app.ID, limit)
	if err != nil {
		return nil, err
	}
	out := make([]SessionInfo, 0, len(ss))
	for _, s := range ss {
		v := ""
		if s.VerifiedMSISDN != nil {
			v = *s.VerifiedMSISDN
		}
		out = append(out, SessionInfo{ID: s.ID, Channel: s.Channel, Status: s.Status, VerifiedMSISDN: v, CreatedAt: s.CreatedAt})
	}
	return out, nil
}

// NewPairing enrolls a receiver and its number, returning a payload the app scans.
func (e *Engine) NewPairing(ctx context.Context, p PairingParams) (Pairing, error) {
	secret, err := auth.GenerateSecret()
	if err != nil {
		return Pairing{}, err
	}
	devType := p.Type
	if devType == "" {
		devType = "android"
	}
	caps := p.Capabilities
	if len(caps) == 0 {
		caps = p.Channels
	}
	dev, err := e.store.CreateDevice(ctx, store.Device{
		AppID: e.app.ID, Name: p.Name, DeviceSecret: secret, Type: devType, Capabilities: caps,
	})
	if err != nil {
		return Pairing{}, err
	}
	if p.MSISDN != "" {
		if _, err := e.store.CreateNumber(ctx, store.Number{DeviceID: dev.ID, MSISDN: p.MSISDN, Channels: p.Channels}); err != nil {
			return Pairing{}, err
		}
	}
	payload, _ := json.Marshal(map[string]string{
		"endpoint": p.Endpoint, "device_id": dev.ID, "device_secret": secret,
	})
	return Pairing{DeviceID: dev.ID, DeviceSecret: secret, Endpoint: p.Endpoint, QRPayload: string(payload)}, nil
}

type callbackNotifier struct {
	cb func(ev Event)
}

func (n callbackNotifier) VerificationVerified(sess store.Session, _ store.App) {
	if n.cb == nil {
		return
	}
	verified := ""
	if sess.VerifiedMSISDN != nil {
		verified = *sess.VerifiedMSISDN
	}
	n.cb(Event{SessionID: sess.ID, VerifiedMSISDN: verified, Channel: sess.Channel})
}
