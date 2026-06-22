# Coordinator

The CallToVerify control plane. It owns verification sessions, the number pool, inbound
matching, abuse blocking, and developer webhooks. Written in Go with Postgres for durable state.

> **Status: alpha.** Phase 1 is implemented: session lifecycle, inbound matching for all three
> channels, provisioning, auth, and signed webhooks. Rate limiting and the replay-nonce cache
> are in-process for now (Redis-backed versions come in Phase 4 for horizontal scale).

## Run

```bash
# from the repo root, with Postgres + Redis:
docker compose up --build

# or run just the Coordinator against a local Postgres:
cd coordinator
export CTV_DATABASE_URL="postgres://calltoverify:calltoverify@localhost:5432/calltoverify?sslmode=disable"
export CTV_ADMIN_TOKEN="dev-admin"   # enables the /admin provisioning API
go run ./cmd/coordinator
```

The Coordinator applies its own schema migrations on startup (tracked in `schema_migrations`).

## Configuration

| Variable | Default | Purpose |
|---|---|---|
| `CTV_LISTEN_ADDR` | `:8080` | HTTP listen address |
| `CTV_DATABASE_URL` | `postgres://calltoverify:calltoverify@localhost:5432/calltoverify?sslmode=disable` | Postgres DSN |
| `CTV_REDIS_URL` | `redis://localhost:6379` | Redis URL (reserved; not yet required) |
| `CTV_ENV` | `development` | `development` or `production` |
| `CTV_ADMIN_TOKEN` | _(empty)_ | Bearer token for `/admin/*`. Empty disables the admin API. |
| `CTV_DEFAULT_CODE_LEN` | `6` | Default verification code length |
| `CTV_DEFAULT_TTL_SECONDS` | `90` | Default session time-to-live |

## HTTP surface and auth

**Operational**

- `GET /healthz` — liveness.
- `GET /readyz` — readiness (pings Postgres).

**Admin / provisioning** — `Authorization: Bearer <CTV_ADMIN_TOKEN>`

- `POST /admin/apps` `{name, webhook_url?, config?}` → `{app_id, api_key, api_key_prefix, webhook_secret}` (secrets shown once).
- `POST /admin/devices` `{app_id, name, type, capabilities[]}` → `{device_id, device_secret}` (shown once).
- `POST /admin/numbers` `{device_id, msisdn, channels[]}` → `{number_id}`.

**Developer API** — `Authorization: Bearer <api_key>`

- `POST /v1/verifications` `{channel?, binding_mode?, claimed_msisdn?}` → instructions `{number, code?, action, deep_link, expires_at}`.
- `GET /v1/verifications/{id}` → `{status, channel, verified_msisdn?, expires_at}`.

**Device API** — HMAC signed. Each request carries headers:
`X-CTV-Device-Id`, `X-CTV-Timestamp` (unix seconds), `X-CTV-Nonce`,
`X-CTV-Signature` = hex HMAC-SHA256(`device_secret`, `timestamp\nnonce\nbody`).
Timestamp skew is capped at ±300s and nonces are single-use.

- `POST /v1/devices/register` → device config + its numbers (marks the device online).
- `POST /v1/devices/heartbeat` → liveness; returns the device's numbers.
- `POST /v1/inbound` `{number, type, sender, body}` → `{matched, session_id?, reason?}`.

Webhooks to the developer backend are signed with `X-CTV-Signature` =
hex HMAC-SHA256(`webhook_secret`, body).

## Layout

```
coordinator/
  cmd/coordinator/      # entrypoint: boot, migrate, serve, background expiry sweep
  internal/
    api/                # HTTP router, auth middleware, handlers
    auth/               # API-key hashing, device HMAC, nonce cache
    config/             # environment configuration
    ratelimit/          # in-process token-bucket limiter
    store/              # pgx-backed persistence + migration runner
    verify/             # domain: start verification, match inbound
    webhook/            # signed webhook delivery
  migrations/           # embedded SQL schema
  Dockerfile
```

## Develop

```bash
go vet ./...
go build ./...
go test ./...                       # unit tests; integration tests skip without a DB

# Integration tests need a Postgres:
docker run -d --name ctv-pg -e POSTGRES_USER=ctv -e POSTGRES_PASSWORD=ctv -e POSTGRES_DB=ctv_test -p 55432:5432 postgres:16-alpine
CTV_TEST_DATABASE_URL="postgres://ctv:ctv@localhost:55432/ctv_test?sslmode=disable" go test ./...
docker rm -f ctv-pg
```
