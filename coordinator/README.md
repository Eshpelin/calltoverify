# Coordinator

The CallToVerify control plane. It owns verification sessions, the number pool, inbound
matching, abuse blocking, and developer webhooks. Written in Go with Postgres for durable state
and Redis for hot session/queue/rate state.

> **Status: alpha.** The HTTP surface is scaffolded; endpoints return `501 Not Implemented`
> until Phase 1 lands the session lifecycle and matching.

## Run

```bash
# from the repo root, with Postgres + Redis:
docker compose up --build

# or run just the Coordinator (expects Postgres/Redis reachable, or use defaults):
cd coordinator
go run ./cmd/coordinator
curl http://localhost:8080/healthz
```

## Configuration

All settings come from the environment (defaults shown):

| Variable | Default | Purpose |
|---|---|---|
| `CTV_LISTEN_ADDR` | `:8080` | HTTP listen address |
| `CTV_DATABASE_URL` | `postgres://calltoverify:calltoverify@localhost:5432/calltoverify?sslmode=disable` | Postgres DSN |
| `CTV_REDIS_URL` | `redis://localhost:6379` | Redis URL |
| `CTV_ENV` | `development` | `development` or `production` |

## HTTP surface

**Operational**

- `GET /healthz` — liveness.
- `GET /readyz` — readiness (will check Postgres + Redis).

**Developer-facing** (SDK → Coordinator)

- `POST /v1/verifications` — start a verification, returns instructions (number, code, deep link).
- `GET /v1/verifications/{id}` — poll status.

**Device-facing** (Receiver ↔ Coordinator)

- `POST /v1/devices/register`
- `POST /v1/devices/heartbeat`
- `POST /v1/inbound` — report an inbound SMS/call for matching.

## Layout

```
coordinator/
  cmd/coordinator/      # main entrypoint
  internal/
    api/                # HTTP router + handlers
    config/             # environment configuration
  migrations/           # Postgres schema (applied by docker compose on first boot)
  Dockerfile
```

## Develop

```bash
go vet ./...
go test ./...
go build ./...
```

No third-party dependencies yet, so it builds offline. Keep the standard library first;
justify new dependencies in the PR.
