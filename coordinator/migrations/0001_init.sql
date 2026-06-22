-- 0001_init.sql — CallToVerify Coordinator initial schema.
-- Target: PostgreSQL 14+. Requires pgcrypto for gen_random_uuid().

CREATE EXTENSION IF NOT EXISTS pgcrypto;

-- Tenants / API keys. A single self-hosted instance can serve multiple apps.
CREATE TABLE apps (
    id             UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name           TEXT NOT NULL,
    api_key        TEXT NOT NULL UNIQUE,
    api_secret     TEXT NOT NULL,                       -- store a hash, never the raw secret
    webhook_url    TEXT,
    webhook_secret TEXT,
    -- config: { binding_mode, code_len, ttl_seconds, channels_enabled[] }
    config         JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at     TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- Receiver devices: a spare Android phone or a Raspberry Pi + GSM modem.
CREATE TABLE devices (
    id             UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    app_id         UUID REFERENCES apps(id) ON DELETE CASCADE,
    name           TEXT NOT NULL,
    device_secret  TEXT NOT NULL,                       -- store a hash
    type           TEXT NOT NULL CHECK (type IN ('android', 'pi')),
    capabilities   TEXT[] NOT NULL DEFAULT '{}',         -- subset of: sms, call, dtmf
    status         TEXT NOT NULL DEFAULT 'offline'
                       CHECK (status IN ('online', 'offline', 'disabled')),
    last_heartbeat TIMESTAMPTZ,
    created_at     TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- MSISDNs hosted by a device, forming the number pool.
CREATE TABLE numbers (
    id         UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    device_id  UUID NOT NULL REFERENCES devices(id) ON DELETE CASCADE,
    msisdn     TEXT NOT NULL UNIQUE,
    channels   TEXT[] NOT NULL DEFAULT '{}',
    status     TEXT NOT NULL DEFAULT 'active'
                   CHECK (status IN ('active', 'paused', 'disabled')),
    rate_caps  JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- Verification sessions.
CREATE TABLE sessions (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    app_id          UUID NOT NULL REFERENCES apps(id) ON DELETE CASCADE,
    channel         TEXT NOT NULL CHECK (channel IN ('sms', 'call', 'dtmf')),
    binding_mode    TEXT NOT NULL CHECK (binding_mode IN ('derive', 'claim')),
    status          TEXT NOT NULL DEFAULT 'pending'
                        CHECK (status IN ('pending', 'verified', 'expired', 'failed')),
    number_id       UUID REFERENCES numbers(id) ON DELETE SET NULL,
    code            TEXT,
    claimed_msisdn  TEXT,
    verified_msisdn TEXT,
    attempts        INT NOT NULL DEFAULT 0,
    metadata        JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    expires_at      TIMESTAMPTZ NOT NULL
);

-- A code must be unique among *pending* sessions on a given number, so inbound
-- signals match exactly one live session.
CREATE UNIQUE INDEX sessions_active_code_per_number
    ON sessions (number_id, code)
    WHERE status = 'pending';

CREATE INDEX sessions_app_status ON sessions (app_id, status);
CREATE INDEX sessions_expires_at ON sessions (expires_at) WHERE status = 'pending';

-- Raw inbound signals from receivers, retained for matching and audit.
CREATE TABLE inbound_events (
    id                 UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    number_id          UUID REFERENCES numbers(id) ON DELETE SET NULL,
    type               TEXT NOT NULL CHECK (type IN ('sms', 'call')),
    sender             TEXT NOT NULL,
    body               TEXT,                            -- SMS body or captured DTMF digits
    matched_session_id UUID REFERENCES sessions(id) ON DELETE SET NULL,
    received_at        TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX inbound_events_number_time ON inbound_events (number_id, received_at);

-- Abuse blocks: numbers or IPs that are rate-limited out.
CREATE TABLE blocks (
    id         UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    target     TEXT NOT NULL,
    kind       TEXT NOT NULL CHECK (kind IN ('msisdn', 'ip')),
    reason     TEXT,
    until      TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX blocks_target ON blocks (target);
