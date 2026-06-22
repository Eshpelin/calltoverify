# Changelog

All notable changes to this project are documented here. The format is based on
[Keep a Changelog](https://keepachangelog.com/en/1.1.0/), and the project aims to follow
[Semantic Versioning](https://semver.org/) once it reaches `1.0`.

## [Unreleased]

### Added
- Initial public scaffold of the CallToVerify monorepo.
- Coordinator service (Go) with health/readiness endpoints and a stubbed v1 API surface
  (`/v1/verifications`, device registration, heartbeat, inbound).
- Initial Postgres schema: apps, devices, numbers, sessions, inbound_events, blocks.
- `docker compose` setup for Coordinator + Postgres + Redis.
- Open-source project hygiene: Apache-2.0 license, contributing guide, code of conduct,
  security policy, issue/PR templates, and CI.
- Architecture, channel, security, and self-hosting documentation; design blueprint in
  `DESIGN.md`.

[Unreleased]: https://github.com/Eshpelin/calltoverify/commits/main
