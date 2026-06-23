package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	_ "modernc.org/sqlite"
)

type sqliteStore struct {
	db *sql.DB
}

// NewSQLite opens (creating if needed) a SQLite database at path. Use ":memory:"
// for an ephemeral store. This is the zero-infra default for the embedded engine.
func NewSQLite(path string) (Store, error) {
	dsn := path + "?_pragma=busy_timeout(5000)&_pragma=foreign_keys(1)"
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, err
	}
	// SQLite serialises writers; a single connection avoids "database is locked".
	db.SetMaxOpenConns(1)
	return &sqliteStore{db: db}, nil
}

func (s *sqliteStore) Close()                         { _ = s.db.Close() }
func (s *sqliteStore) Ping(ctx context.Context) error { return s.db.PingContext(ctx) }

const sqliteSchema = `
CREATE TABLE IF NOT EXISTS apps (
  id TEXT PRIMARY KEY, name TEXT NOT NULL, api_key_hash TEXT NOT NULL UNIQUE,
  api_key_prefix TEXT NOT NULL, webhook_url TEXT NOT NULL DEFAULT '', webhook_secret TEXT NOT NULL,
  config TEXT NOT NULL DEFAULT '{}', created_at TEXT NOT NULL);
CREATE TABLE IF NOT EXISTS devices (
  id TEXT PRIMARY KEY, app_id TEXT NOT NULL REFERENCES apps(id) ON DELETE CASCADE,
  name TEXT NOT NULL, device_secret TEXT NOT NULL, type TEXT NOT NULL,
  capabilities TEXT NOT NULL DEFAULT '[]', status TEXT NOT NULL DEFAULT 'offline',
  last_heartbeat TEXT, created_at TEXT NOT NULL);
CREATE TABLE IF NOT EXISTS numbers (
  id TEXT PRIMARY KEY, device_id TEXT NOT NULL REFERENCES devices(id) ON DELETE CASCADE,
  msisdn TEXT NOT NULL UNIQUE, channels TEXT NOT NULL DEFAULT '[]',
  status TEXT NOT NULL DEFAULT 'active', created_at TEXT NOT NULL);
CREATE TABLE IF NOT EXISTS sessions (
  id TEXT PRIMARY KEY, app_id TEXT NOT NULL REFERENCES apps(id) ON DELETE CASCADE,
  channel TEXT NOT NULL, binding_mode TEXT NOT NULL, status TEXT NOT NULL DEFAULT 'pending',
  number_id TEXT, code TEXT, claimed_msisdn TEXT, verified_msisdn TEXT,
  attempts INTEGER NOT NULL DEFAULT 0, created_at TEXT NOT NULL, expires_at TEXT NOT NULL);
CREATE UNIQUE INDEX IF NOT EXISTS sessions_active_code_per_number
  ON sessions(number_id, code) WHERE status='pending' AND code IS NOT NULL;
CREATE UNIQUE INDEX IF NOT EXISTS sessions_one_voice_per_number
  ON sessions(number_id) WHERE status='pending' AND channel IN ('call','dtmf');
CREATE INDEX IF NOT EXISTS sessions_app_status ON sessions(app_id, status);
CREATE TABLE IF NOT EXISTS inbound_events (
  id TEXT PRIMARY KEY, number_id TEXT, type TEXT NOT NULL, sender TEXT NOT NULL,
  body TEXT, matched_session_id TEXT, received_at TEXT NOT NULL);
CREATE INDEX IF NOT EXISTS inbound_events_sender_time ON inbound_events (sender, received_at);
CREATE TABLE IF NOT EXISTS blocks (
  id TEXT PRIMARY KEY, target TEXT NOT NULL, kind TEXT NOT NULL, reason TEXT,
  until TEXT, created_at TEXT NOT NULL);
`

func (s *sqliteStore) Migrate(ctx context.Context) error {
	_, err := s.db.ExecContext(ctx, sqliteSchema)
	return err
}

func (s *sqliteStore) Reset(ctx context.Context) error {
	_, err := s.db.ExecContext(ctx,
		`DELETE FROM inbound_events; DELETE FROM sessions; DELETE FROM numbers; DELETE FROM devices; DELETE FROM blocks; DELETE FROM apps;`)
	return err
}

// --- apps ---

func (s *sqliteStore) CreateApp(ctx context.Context, a App) (App, error) {
	a.ID = newID()
	a.CreatedAt = nowUTC()
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO apps (id, name, api_key_hash, api_key_prefix, webhook_url, webhook_secret, config, created_at)
		 VALUES (?,?,?,?,?,?,?,?)`,
		a.ID, a.Name, a.APIKeyHash, a.APIKeyPrefix, a.WebhookURL, a.WebhookSecret, encConfig(a.Config), fmtTime(a.CreatedAt))
	return a, err
}

func (s *sqliteStore) EnsureApp(ctx context.Context, a App) (App, error) {
	_, err := s.db.ExecContext(ctx,
		`INSERT OR IGNORE INTO apps (id, name, api_key_hash, api_key_prefix, webhook_url, webhook_secret, config, created_at)
		 VALUES (?,?,?,?,?,?,?,?)`,
		a.ID, a.Name, a.APIKeyHash, a.APIKeyPrefix, a.WebhookURL, a.WebhookSecret, encConfig(a.Config), fmtTime(nowUTC()))
	if err != nil {
		return App{}, err
	}
	return s.GetAppByID(ctx, a.ID)
}

const sqliteAppCols = `id, name, api_key_hash, api_key_prefix, webhook_url, webhook_secret, config, created_at`

func (s *sqliteStore) GetAppByAPIKeyHash(ctx context.Context, hash string) (App, error) {
	return scanApp(s.db.QueryRowContext(ctx, `SELECT `+sqliteAppCols+` FROM apps WHERE api_key_hash = ?`, hash))
}

func (s *sqliteStore) GetAppByID(ctx context.Context, id string) (App, error) {
	return scanApp(s.db.QueryRowContext(ctx, `SELECT `+sqliteAppCols+` FROM apps WHERE id = ?`, id))
}

func scanApp(sc scanner) (App, error) {
	var a App
	var cfg, createdAt string
	err := sc.Scan(&a.ID, &a.Name, &a.APIKeyHash, &a.APIKeyPrefix, &a.WebhookURL, &a.WebhookSecret, &cfg, &createdAt)
	if errors.Is(err, sql.ErrNoRows) {
		return App{}, ErrNotFound
	}
	if err != nil {
		return App{}, err
	}
	if cfg != "" {
		_ = json.Unmarshal([]byte(cfg), &a.Config)
	}
	a.CreatedAt = parseTime(createdAt)
	return a, nil
}

// --- devices ---

func (s *sqliteStore) CreateDevice(ctx context.Context, d Device) (Device, error) {
	d.ID = newID()
	d.Status = "offline"
	d.CreatedAt = nowUTC()
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO devices (id, app_id, name, device_secret, type, capabilities, status, created_at)
		 VALUES (?,?,?,?,?,?,?,?)`,
		d.ID, d.AppID, d.Name, d.DeviceSecret, d.Type, encStrs(d.Capabilities), d.Status, fmtTime(d.CreatedAt))
	return d, err
}

func (s *sqliteStore) GetDeviceByID(ctx context.Context, id string) (Device, error) {
	var d Device
	var caps, createdAt string
	var heartbeat sql.NullString
	err := s.db.QueryRowContext(ctx,
		`SELECT id, app_id, name, device_secret, type, capabilities, status, last_heartbeat, created_at FROM devices WHERE id = ?`, id,
	).Scan(&d.ID, &d.AppID, &d.Name, &d.DeviceSecret, &d.Type, &caps, &d.Status, &heartbeat, &createdAt)
	if errors.Is(err, sql.ErrNoRows) {
		return Device{}, ErrNotFound
	}
	if err != nil {
		return Device{}, err
	}
	d.Capabilities = decStrs(caps)
	d.CreatedAt = parseTime(createdAt)
	if heartbeat.Valid {
		t := parseTime(heartbeat.String)
		d.LastHeartbeat = &t
	}
	return d, nil
}

func (s *sqliteStore) SetHeartbeat(ctx context.Context, deviceID string) error {
	res, err := s.db.ExecContext(ctx,
		`UPDATE devices SET status='online', last_heartbeat=? WHERE id=?`, fmtTime(nowUTC()), deviceID)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *sqliteStore) DeleteDevice(ctx context.Context, id string) error {
	res, err := s.db.ExecContext(ctx, `DELETE FROM devices WHERE id=?`, id)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *sqliteStore) ListNumbersByDevice(ctx context.Context, deviceID string) ([]Number, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, device_id, msisdn, channels, status, created_at FROM numbers WHERE device_id = ?`, deviceID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Number
	for rows.Next() {
		n, err := scanNumber(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, n)
	}
	return out, rows.Err()
}

// --- numbers ---

func (s *sqliteStore) CreateNumber(ctx context.Context, n Number) (Number, error) {
	n.ID = newID()
	n.Status = "active"
	n.CreatedAt = nowUTC()
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO numbers (id, device_id, msisdn, channels, status, created_at) VALUES (?,?,?,?,?,?)`,
		n.ID, n.DeviceID, n.MSISDN, encStrs(n.Channels), n.Status, fmtTime(n.CreatedAt))
	return n, err
}

func (s *sqliteStore) GetNumberByMSISDN(ctx context.Context, msisdn string) (Number, error) {
	return scanNumber(s.db.QueryRowContext(ctx,
		`SELECT id, device_id, msisdn, channels, status, created_at FROM numbers WHERE msisdn = ?`, msisdn))
}

func (s *sqliteStore) PickNumber(ctx context.Context, channel string) (Number, error) {
	excl := ""
	if isVoiceChannel(channel) {
		excl = ` AND NOT EXISTS (SELECT 1 FROM sessions sv WHERE sv.number_id = n.id AND sv.status='pending' AND sv.channel IN ('call','dtmf'))`
	}
	capFilter := fmt.Sprintf(` AND (SELECT count(*) FROM sessions sc WHERE sc.number_id = n.id AND sc.status='pending') < %d`, MaxPendingPerNumber)
	return scanNumber(s.db.QueryRowContext(ctx,
		`SELECT n.id, n.device_id, n.msisdn, n.channels, n.status, n.created_at
		 FROM numbers n JOIN devices d ON d.id = n.device_id
		 WHERE n.status='active' AND d.status='online'
		   AND EXISTS (SELECT 1 FROM json_each(n.channels) WHERE value = ?)`+excl+capFilter+`
		 ORDER BY (SELECT count(*) FROM sessions s WHERE s.number_id=n.id AND s.status='pending') ASC, random()
		 LIMIT 1`, channel))
}

func (s *sqliteStore) CountAvailableNumbers(ctx context.Context, channel string) (int, error) {
	var c int
	err := s.db.QueryRowContext(ctx,
		`SELECT count(*) FROM numbers n JOIN devices d ON d.id = n.device_id
		 WHERE n.status='active' AND d.status='online'
		   AND EXISTS (SELECT 1 FROM json_each(n.channels) WHERE value = ?)`, channel).Scan(&c)
	return c, err
}

func scanNumber(sc scanner) (Number, error) {
	var n Number
	var channels, createdAt string
	err := sc.Scan(&n.ID, &n.DeviceID, &n.MSISDN, &channels, &n.Status, &createdAt)
	if errors.Is(err, sql.ErrNoRows) {
		return Number{}, ErrNotFound
	}
	if err != nil {
		return Number{}, err
	}
	n.Channels = decStrs(channels)
	n.CreatedAt = parseTime(createdAt)
	return n, nil
}

// --- sessions ---

func (s *sqliteStore) CreateSession(ctx context.Context, sess Session) (Session, error) {
	sess.ID = newID()
	sess.Status = "pending"
	sess.Attempts = 0
	sess.CreatedAt = nowUTC()
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO sessions (id, app_id, channel, binding_mode, status, number_id, code, claimed_msisdn, attempts, created_at, expires_at)
		 VALUES (?,?,?,?, 'pending', ?,?,?, 0, ?, ?)`,
		sess.ID, sess.AppID, sess.Channel, sess.BindingMode, nullable(sess.NumberID), nullable(sess.Code),
		nullable(sess.ClaimedMSISDN), fmtTime(sess.CreatedAt), fmtTime(sess.ExpiresAt))
	if err != nil {
		if strings.Contains(err.Error(), "UNIQUE constraint failed") {
			return Session{}, ErrConflict
		}
		return Session{}, err
	}
	return sess, nil
}

const sqliteSessionCols = `id, app_id, channel, binding_mode, status, number_id, code, claimed_msisdn, verified_msisdn, attempts, created_at, expires_at`

func (s *sqliteStore) GetSession(ctx context.Context, appID, id string) (Session, error) {
	return scanSessionRow(s.db.QueryRowContext(ctx,
		`SELECT `+sqliteSessionCols+` FROM sessions WHERE id = ? AND app_id = ?`, id, appID))
}

func (s *sqliteStore) FindPendingByCode(ctx context.Context, numberID, code, channel string) (Session, error) {
	return scanSessionRow(s.db.QueryRowContext(ctx,
		`SELECT `+sqliteSessionCols+` FROM sessions
		 WHERE number_id=? AND code=? AND channel=? AND status='pending' AND expires_at > ?
		 ORDER BY created_at DESC LIMIT 1`, numberID, code, channel, fmtTime(nowUTC())))
}

func (s *sqliteStore) FindPendingMissedCall(ctx context.Context, numberID, claimed string) (Session, error) {
	return scanSessionRow(s.db.QueryRowContext(ctx,
		`SELECT `+sqliteSessionCols+` FROM sessions
		 WHERE number_id=? AND channel='call' AND claimed_msisdn=? AND status='pending' AND expires_at > ?
		 ORDER BY created_at DESC LIMIT 1`, numberID, claimed, fmtTime(nowUTC())))
}

func scanSessionRow(sc scanner) (Session, error) {
	var sess Session
	var numID, code, claimed, verified sql.NullString
	var createdAt, expiresAt string
	err := sc.Scan(&sess.ID, &sess.AppID, &sess.Channel, &sess.BindingMode, &sess.Status,
		&numID, &code, &claimed, &verified, &sess.Attempts, &createdAt, &expiresAt)
	if errors.Is(err, sql.ErrNoRows) {
		return Session{}, ErrNotFound
	}
	if err != nil {
		return Session{}, err
	}
	sess.NumberID = toPtr(numID)
	sess.Code = toPtr(code)
	sess.ClaimedMSISDN = toPtr(claimed)
	sess.VerifiedMSISDN = toPtr(verified)
	sess.CreatedAt = parseTime(createdAt)
	sess.ExpiresAt = parseTime(expiresAt)
	return sess, nil
}

func (s *sqliteStore) VerifySession(ctx context.Context, id, verifiedMSISDN string) (bool, error) {
	res, err := s.db.ExecContext(ctx,
		`UPDATE sessions SET status='verified', verified_msisdn=? WHERE id=? AND status='pending'`, verifiedMSISDN, id)
	if err != nil {
		return false, err
	}
	n, _ := res.RowsAffected()
	return n == 1, nil
}

func (s *sqliteStore) IncrementAttempts(ctx context.Context, id string) error {
	_, err := s.db.ExecContext(ctx, `UPDATE sessions SET attempts = attempts + 1 WHERE id = ?`, id)
	return err
}

func (s *sqliteStore) ExpireDue(ctx context.Context) (int64, error) {
	res, err := s.db.ExecContext(ctx,
		`UPDATE sessions SET status='expired' WHERE status='pending' AND expires_at <= ?`, fmtTime(nowUTC()))
	if err != nil {
		return 0, err
	}
	return res.RowsAffected()
}

func (s *sqliteStore) DeleteInboundEventsBefore(ctx context.Context, cutoff time.Time) (int64, error) {
	res, err := s.db.ExecContext(ctx, `DELETE FROM inbound_events WHERE received_at < ?`, fmtTime(cutoff.UTC()))
	if err != nil {
		return 0, err
	}
	return res.RowsAffected()
}

// --- inbound events & blocks ---

func (s *sqliteStore) CreateInboundEvent(ctx context.Context, ev InboundEvent) error {
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO inbound_events (id, number_id, type, sender, body, matched_session_id, received_at)
		 VALUES (?,?,?,?,?,?,?)`,
		newID(), nullableStr(ev.NumberID), ev.Type, ev.Sender, ev.Body, nullable(ev.MatchedSessionID), fmtTime(nowUTC()))
	return err
}

func (s *sqliteStore) CountInboundBySender(ctx context.Context, sender string, since time.Time, unmatchedOnly bool) (int, error) {
	q := `SELECT count(*) FROM inbound_events WHERE sender = ? AND received_at > ?`
	if unmatchedOnly {
		q += ` AND matched_session_id IS NULL`
	}
	var c int
	err := s.db.QueryRowContext(ctx, q, sender, fmtTime(since)).Scan(&c)
	return c, err
}

func (s *sqliteStore) IsBlocked(ctx context.Context, target string) (bool, error) {
	var n int
	err := s.db.QueryRowContext(ctx,
		`SELECT count(*) FROM blocks WHERE target=? AND (until IS NULL OR until > ?)`, target, fmtTime(nowUTC())).Scan(&n)
	return n > 0, err
}

func (s *sqliteStore) CreateBlock(ctx context.Context, target, kind, reason string, until *time.Time) error {
	var untilStr any
	if until != nil {
		untilStr = fmtTime(*until)
	}
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO blocks (id, target, kind, reason, until, created_at) VALUES (?,?,?,?,?,?)`,
		newID(), target, kind, reason, untilStr, fmtTime(nowUTC()))
	return err
}

func (s *sqliteStore) ListDevices(ctx context.Context) ([]Device, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, app_id, name, device_secret, type, capabilities, status, last_heartbeat, created_at FROM devices ORDER BY created_at`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Device
	for rows.Next() {
		var d Device
		var caps, createdAt string
		var hb sql.NullString
		if err := rows.Scan(&d.ID, &d.AppID, &d.Name, &d.DeviceSecret, &d.Type, &caps, &d.Status, &hb, &createdAt); err != nil {
			return nil, err
		}
		d.Capabilities = decStrs(caps)
		d.CreatedAt = parseTime(createdAt)
		if hb.Valid {
			t := parseTime(hb.String)
			d.LastHeartbeat = &t
		}
		out = append(out, d)
	}
	return out, rows.Err()
}

func (s *sqliteStore) ListRecentSessions(ctx context.Context, appID string, limit int) ([]Session, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT `+sqliteSessionCols+` FROM sessions WHERE app_id=? ORDER BY created_at DESC LIMIT ?`, appID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Session
	for rows.Next() {
		sess, err := scanSessionRow(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, sess)
	}
	return out, rows.Err()
}

// --- helpers ---

type scanner interface {
	Scan(dest ...any) error
}

func encStrs(ss []string) string {
	if ss == nil {
		ss = []string{}
	}
	b, _ := json.Marshal(ss)
	return string(b)
}

func decStrs(s string) []string {
	var out []string
	if s != "" {
		_ = json.Unmarshal([]byte(s), &out)
	}
	return out
}

func encConfig(c AppConfig) string {
	b, _ := json.Marshal(c)
	return string(b)
}

func fmtTime(t time.Time) string { return t.UTC().Format(tsLayout) }

func parseTime(s string) time.Time {
	t, _ := time.Parse(tsLayout, s)
	return t
}

func toPtr(ns sql.NullString) *string {
	if ns.Valid {
		v := ns.String
		return &v
	}
	return nil
}

// nullable returns the string value or nil for a *string column.
func nullable(p *string) any {
	if p == nil {
		return nil
	}
	return *p
}

// nullableStr maps "" to nil for an optional column stored from a plain string.
func nullableStr(s string) any {
	if s == "" {
		return nil
	}
	return s
}
