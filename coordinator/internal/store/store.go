// Package store is the Coordinator's persistence layer. The Store interface is
// backed by two implementations: Postgres (for scale / the standalone Coordinator)
// and SQLite (zero-infra default for the embedded engine).
package store

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"time"
)

// ErrNotFound is returned when a lookup matches no row.
var ErrNotFound = errors.New("not found")

// ErrConflict is returned when an insert violates a unique constraint (used to
// retry verification-code generation on the rare collision).
var ErrConflict = errors.New("conflict")

// tsLayout is a fixed-width UTC timestamp format so SQLite text comparisons sort
// chronologically. Always store times as UTC in this layout.
const tsLayout = "2006-01-02T15:04:05.000Z"

// Store is the persistence contract shared by the Postgres and SQLite backends.
type Store interface {
	Migrate(ctx context.Context) error
	Ping(ctx context.Context) error
	Close()
	Reset(ctx context.Context) error // test helper: truncate all tables

	CreateApp(ctx context.Context, a App) (App, error)
	// EnsureApp upserts an app by its (caller-supplied) ID; used by the embedded
	// engine to maintain a single implicit app.
	EnsureApp(ctx context.Context, a App) (App, error)
	GetAppByAPIKeyHash(ctx context.Context, hash string) (App, error)
	GetAppByID(ctx context.Context, id string) (App, error)

	CreateDevice(ctx context.Context, d Device) (Device, error)
	GetDeviceByID(ctx context.Context, id string) (Device, error)
	SetHeartbeat(ctx context.Context, deviceID string) error
	ListNumbersByDevice(ctx context.Context, deviceID string) ([]Number, error)
	ListDevices(ctx context.Context) ([]Device, error)

	CreateNumber(ctx context.Context, n Number) (Number, error)
	GetNumberByMSISDN(ctx context.Context, msisdn string) (Number, error)
	// PickNumber chooses an online, active number for the channel. For voice
	// channels (call, dtmf) it excludes numbers that already have a pending voice
	// session, enforcing one voice verification per SIM at a time.
	PickNumber(ctx context.Context, channel string) (Number, error)
	// CountAvailableNumbers counts online, active numbers that support the channel,
	// ignoring whether they are currently busy. Used to tell "no capacity" apart
	// from "all voice lines busy".
	CountAvailableNumbers(ctx context.Context, channel string) (int, error)

	CreateSession(ctx context.Context, s Session) (Session, error) // ErrConflict on code collision
	GetSession(ctx context.Context, appID, id string) (Session, error)
	ListRecentSessions(ctx context.Context, appID string, limit int) ([]Session, error)
	FindPendingByCode(ctx context.Context, numberID, code, channel string) (Session, error)
	FindPendingMissedCall(ctx context.Context, numberID, claimed string) (Session, error)
	VerifySession(ctx context.Context, id, verifiedMSISDN string) (bool, error)
	IncrementAttempts(ctx context.Context, id string) error
	ExpireDue(ctx context.Context) (int64, error)

	CreateInboundEvent(ctx context.Context, ev InboundEvent) error
	IsBlocked(ctx context.Context, target string) (bool, error)
	CreateBlock(ctx context.Context, target, kind, reason string, until *time.Time) error
}

// newID returns a random UUIDv4 string. IDs are generated in Go so both backends
// share the same scheme without relying on a database extension.
func newID() string {
	var b [16]byte
	_, _ = rand.Read(b[:])
	b[6] = (b[6] & 0x0f) | 0x40
	b[8] = (b[8] & 0x3f) | 0x80
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:16])
}

func nowUTC() time.Time { return time.Now().UTC() }

// isVoiceChannel reports whether a channel occupies a SIM's single voice line.
func isVoiceChannel(channel string) bool { return channel == "call" || channel == "dtmf" }
