# Changelog

All notable changes to this project are documented here. The format is based on
[Keep a Changelog](https://keepachangelog.com/en/1.1.0/), and the project aims to follow
[Semantic Versioning](https://semver.org/) once it reaches `1.0`.

## [Unreleased]

### Added
- **Ecosystem clients**: backend SDKs for Node/TypeScript, Python, and PHP (Laravel-friendly);
  front-end for the web widget, React, and Flutter; receivers for Android (Kotlin) and Raspberry
  Pi (Python, with gammu + Asterisk). All share one HMAC device protocol, pinned by cross-language
  known-answer vectors.
- **Self-hosted console** (`coordinator/examples/dashboard`): a guided onboarding wizard (served at
  `/` and `/setup`) — pair a phone via QR, run a test verification — plus a live ops dashboard of
  receivers and recent verifications. Plus a non-Go `examples/node-web` (Node SDK + browser UI).
- **Per-SIM voice queueing**: at most one pending call/DTMF verification per number (SMS
  multiplexes), made atomic by a partial unique index; all-busy returns a `BusyError` with a queue
  position.
- **Abuse auto-block**: senders that brute-force codes or flood inbound are auto-blocked for an
  hour.
- **Redis-backed rate-limit + replay-nonce** (`CTV_REDIS_URL`) for sharing limits across instances;
  falls back to in-process.
- **Embeddable Go engine** (`coordinator/engine`): add CallToVerify to your own Go backend with
  no separate service. `New` / `StartVerification` / `Status` / `DeviceHandler` / `NewPairing`
  (QR pairing). SQLite is the zero-infra default; Postgres via `PostgresDSN`.
- `Store` is now an interface with **Postgres and SQLite** backends (pure-Go `modernc.org/sqlite`).
- Shared `deviceapi` package for the device-facing routes (used by both the standalone
  Coordinator and the embedded engine).
- Runnable embedded example at `coordinator/examples/embedded`.
- Initial public scaffold of the CallToVerify monorepo.
- **Coordinator Phase 1**: full verification implementation.
  - pgx-backed persistence with an embedded, self-applying migration runner.
  - Domain logic: start verification (number-pool selection, code generation,
    channel/binding validation) and inbound matching for SMS, missed call, and DTMF.
  - Provisioning API (`/admin/apps`, `/admin/devices`, `/admin/numbers`) behind an admin token.
  - Developer API (`POST /v1/verifications`, `GET /v1/verifications/{id}`) with bearer API keys.
  - Device API (`/v1/devices/register`, `/v1/devices/heartbeat`, `/v1/inbound`) with HMAC +
    timestamp + nonce request signing and replay protection.
  - Signed webhooks on verification, configurable binding modes (`derive`/`claim`), in-process
    rate limiting, abuse blocks, and a background session-expiry sweep.
  - Unit tests plus Postgres-backed integration tests; CI now runs a Postgres service.
- Initial Postgres schema: apps, devices, numbers, sessions, inbound_events, blocks.
- `docker compose` setup for Coordinator + Postgres + Redis.
- Open-source project hygiene: Apache-2.0 license, contributing guide, code of conduct,
  security policy, issue/PR templates, and CI.
- Architecture, channel, security, self-hosting, and end-to-end getting-started documentation;
  design blueprint in `DESIGN.md`.

[Unreleased]: https://github.com/Eshpelin/calltoverify/commits/main
