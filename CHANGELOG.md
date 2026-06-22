# Changelog

All notable changes to this project are documented here. The format is based on
[Keep a Changelog](https://keepachangelog.com/en/1.1.0/), and the project aims to follow
[Semantic Versioning](https://semver.org/) once it reaches `1.0`.

## [Unreleased]

### Added
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
