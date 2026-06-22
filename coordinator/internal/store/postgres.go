// Package store is the Coordinator's persistence layer, backed by PostgreSQL via pgx.
package store

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// ErrNotFound is returned when a lookup matches no row.
var ErrNotFound = errors.New("not found")

type Store struct {
	pool *pgxpool.Pool
}

// New opens a connection pool to the given DSN.
func New(ctx context.Context, dsn string) (*Store, error) {
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		return nil, fmt.Errorf("connect: %w", err)
	}
	return &Store{pool: pool}, nil
}

func (s *Store) Close() { s.pool.Close() }

func (s *Store) Ping(ctx context.Context) error { return s.pool.Ping(ctx) }

// --- apps ---

func (s *Store) CreateApp(ctx context.Context, a App) (App, error) {
	cfg, err := json.Marshal(a.Config)
	if err != nil {
		return App{}, err
	}
	err = s.pool.QueryRow(ctx,
		`INSERT INTO apps (name, api_key_hash, api_key_prefix, webhook_url, webhook_secret, config)
		 VALUES ($1, $2, $3, $4, $5, $6::jsonb)
		 RETURNING id, created_at`,
		a.Name, a.APIKeyHash, a.APIKeyPrefix, a.WebhookURL, a.WebhookSecret, string(cfg),
	).Scan(&a.ID, &a.CreatedAt)
	return a, err
}

func (s *Store) GetAppByAPIKeyHash(ctx context.Context, hash string) (App, error) {
	return s.scanApp(s.pool.QueryRow(ctx, appSelect+` WHERE api_key_hash = $1`, hash))
}

func (s *Store) GetAppByID(ctx context.Context, id string) (App, error) {
	return s.scanApp(s.pool.QueryRow(ctx, appSelect+` WHERE id = $1`, id))
}

const appSelect = `SELECT id, name, api_key_hash, api_key_prefix, COALESCE(webhook_url, ''), webhook_secret, config, created_at FROM apps`

func (s *Store) scanApp(row pgx.Row) (App, error) {
	var a App
	var cfg []byte
	if err := row.Scan(&a.ID, &a.Name, &a.APIKeyHash, &a.APIKeyPrefix, &a.WebhookURL, &a.WebhookSecret, &cfg, &a.CreatedAt); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return App{}, ErrNotFound
		}
		return App{}, err
	}
	if len(cfg) > 0 {
		_ = json.Unmarshal(cfg, &a.Config)
	}
	return a, nil
}

// --- devices ---

func (s *Store) CreateDevice(ctx context.Context, d Device) (Device, error) {
	err := s.pool.QueryRow(ctx,
		`INSERT INTO devices (app_id, name, device_secret, type, capabilities)
		 VALUES ($1, $2, $3, $4, $5)
		 RETURNING id, status, created_at`,
		d.AppID, d.Name, d.DeviceSecret, d.Type, d.Capabilities,
	).Scan(&d.ID, &d.Status, &d.CreatedAt)
	return d, err
}

func (s *Store) GetDeviceByID(ctx context.Context, id string) (Device, error) {
	var d Device
	err := s.pool.QueryRow(ctx,
		`SELECT id, app_id, name, device_secret, type, capabilities, status, last_heartbeat, created_at
		 FROM devices WHERE id = $1`, id,
	).Scan(&d.ID, &d.AppID, &d.Name, &d.DeviceSecret, &d.Type, &d.Capabilities, &d.Status, &d.LastHeartbeat, &d.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return Device{}, ErrNotFound
	}
	return d, err
}

// SetHeartbeat marks a device online and stamps last_heartbeat.
func (s *Store) SetHeartbeat(ctx context.Context, deviceID string) error {
	tag, err := s.pool.Exec(ctx,
		`UPDATE devices SET status = 'online', last_heartbeat = now() WHERE id = $1`, deviceID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *Store) ListNumbersByDevice(ctx context.Context, deviceID string) ([]Number, error) {
	rows, err := s.pool.Query(ctx,
		`SELECT id, device_id, msisdn, channels, status, created_at FROM numbers WHERE device_id = $1`, deviceID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Number
	for rows.Next() {
		var n Number
		if err := rows.Scan(&n.ID, &n.DeviceID, &n.MSISDN, &n.Channels, &n.Status, &n.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, n)
	}
	return out, rows.Err()
}

// --- numbers ---

func (s *Store) CreateNumber(ctx context.Context, n Number) (Number, error) {
	err := s.pool.QueryRow(ctx,
		`INSERT INTO numbers (device_id, msisdn, channels) VALUES ($1, $2, $3)
		 RETURNING id, status, created_at`,
		n.DeviceID, n.MSISDN, n.Channels,
	).Scan(&n.ID, &n.Status, &n.CreatedAt)
	return n, err
}

func (s *Store) GetNumberByMSISDN(ctx context.Context, msisdn string) (Number, error) {
	var n Number
	err := s.pool.QueryRow(ctx,
		`SELECT id, device_id, msisdn, channels, status, created_at FROM numbers WHERE msisdn = $1`, msisdn,
	).Scan(&n.ID, &n.DeviceID, &n.MSISDN, &n.Channels, &n.Status, &n.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return Number{}, ErrNotFound
	}
	return n, err
}

// PickNumber chooses an active number that supports the channel and belongs to an
// online device, preferring the one with the fewest pending sessions.
func (s *Store) PickNumber(ctx context.Context, channel string) (Number, error) {
	var n Number
	err := s.pool.QueryRow(ctx,
		`SELECT n.id, n.device_id, n.msisdn, n.channels, n.status, n.created_at
		 FROM numbers n
		 JOIN devices d ON d.id = n.device_id
		 WHERE n.status = 'active'
		   AND d.status = 'online'
		   AND $1 = ANY(n.channels)
		 ORDER BY (SELECT count(*) FROM sessions s WHERE s.number_id = n.id AND s.status = 'pending') ASC,
		          random()
		 LIMIT 1`, channel,
	).Scan(&n.ID, &n.DeviceID, &n.MSISDN, &n.Channels, &n.Status, &n.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return Number{}, ErrNotFound
	}
	return n, err
}

// --- sessions ---

func (s *Store) CreateSession(ctx context.Context, sess Session) (Session, error) {
	err := s.pool.QueryRow(ctx,
		`INSERT INTO sessions (app_id, channel, binding_mode, status, number_id, code, claimed_msisdn, expires_at)
		 VALUES ($1, $2, $3, 'pending', $4, $5, $6, $7)
		 RETURNING id, status, attempts, created_at`,
		sess.AppID, sess.Channel, sess.BindingMode, sess.NumberID, sess.Code, sess.ClaimedMSISDN, sess.ExpiresAt,
	).Scan(&sess.ID, &sess.Status, &sess.Attempts, &sess.CreatedAt)
	return sess, err
}

func (s *Store) GetSession(ctx context.Context, appID, id string) (Session, error) {
	return s.scanSession(s.pool.QueryRow(ctx, sessionSelect+` WHERE id = $1 AND app_id = $2`, id, appID))
}

const sessionSelect = `SELECT id, app_id, channel, binding_mode, status, number_id, code, claimed_msisdn, verified_msisdn, attempts, created_at, expires_at FROM sessions`

func (s *Store) scanSession(row pgx.Row) (Session, error) {
	var sess Session
	err := row.Scan(&sess.ID, &sess.AppID, &sess.Channel, &sess.BindingMode, &sess.Status,
		&sess.NumberID, &sess.Code, &sess.ClaimedMSISDN, &sess.VerifiedMSISDN, &sess.Attempts, &sess.CreatedAt, &sess.ExpiresAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return Session{}, ErrNotFound
	}
	return sess, err
}

// FindPendingByCode returns the live (pending, unexpired) session on a number with
// the given code and channel.
func (s *Store) FindPendingByCode(ctx context.Context, numberID, code, channel string) (Session, error) {
	return s.scanSession(s.pool.QueryRow(ctx,
		sessionSelect+` WHERE number_id = $1 AND code = $2 AND channel = $3 AND status = 'pending' AND expires_at > now()
		 ORDER BY created_at DESC LIMIT 1`, numberID, code, channel))
}

// FindPendingMissedCall returns the live missed-call (channel 'call') session on a
// number whose claimed number matches the caller.
func (s *Store) FindPendingMissedCall(ctx context.Context, numberID, claimed string) (Session, error) {
	return s.scanSession(s.pool.QueryRow(ctx,
		sessionSelect+` WHERE number_id = $1 AND channel = 'call' AND claimed_msisdn = $2 AND status = 'pending' AND expires_at > now()
		 ORDER BY created_at DESC LIMIT 1`, numberID, claimed))
}

// VerifySession atomically transitions a pending session to verified. It returns
// false if the session was already resolved (lost the race), guaranteeing a single
// verification.
func (s *Store) VerifySession(ctx context.Context, id, verifiedMSISDN string) (bool, error) {
	tag, err := s.pool.Exec(ctx,
		`UPDATE sessions SET status = 'verified', verified_msisdn = $2
		 WHERE id = $1 AND status = 'pending'`, id, verifiedMSISDN)
	if err != nil {
		return false, err
	}
	return tag.RowsAffected() == 1, nil
}

func (s *Store) IncrementAttempts(ctx context.Context, id string) error {
	_, err := s.pool.Exec(ctx, `UPDATE sessions SET attempts = attempts + 1 WHERE id = $1`, id)
	return err
}

// ExpireDue marks pending sessions past their expiry as expired and returns the count.
func (s *Store) ExpireDue(ctx context.Context) (int64, error) {
	tag, err := s.pool.Exec(ctx,
		`UPDATE sessions SET status = 'expired' WHERE status = 'pending' AND expires_at <= now()`)
	if err != nil {
		return 0, err
	}
	return tag.RowsAffected(), nil
}

// --- inbound events & blocks ---

func (s *Store) CreateInboundEvent(ctx context.Context, ev InboundEvent) error {
	_, err := s.pool.Exec(ctx,
		`INSERT INTO inbound_events (number_id, type, sender, body, matched_session_id)
		 VALUES ($1, $2, $3, $4, $5)`,
		ev.NumberID, ev.Type, ev.Sender, ev.Body, ev.MatchedSessionID)
	return err
}

// IsBlocked reports whether a target (msisdn or ip) currently has an active block.
func (s *Store) IsBlocked(ctx context.Context, target string) (bool, error) {
	var exists bool
	err := s.pool.QueryRow(ctx,
		`SELECT EXISTS(SELECT 1 FROM blocks WHERE target = $1 AND (until IS NULL OR until > now()))`, target,
	).Scan(&exists)
	return exists, err
}

func (s *Store) CreateBlock(ctx context.Context, target, kind, reason string, until *time.Time) error {
	_, err := s.pool.Exec(ctx,
		`INSERT INTO blocks (target, kind, reason, until) VALUES ($1, $2, $3, $4)`,
		target, kind, reason, until)
	return err
}

// Reset truncates all data tables. Intended for tests only.
func (s *Store) Reset(ctx context.Context) error {
	_, err := s.pool.Exec(ctx,
		`TRUNCATE apps, devices, numbers, sessions, inbound_events, blocks RESTART IDENTITY CASCADE`)
	return err
}
