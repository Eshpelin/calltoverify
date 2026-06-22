# Architecture

CallToVerify has four moving parts. They communicate over signed HTTPS.

```
  End user phone ──(SMS / missed call / DTMF)──▶ Receiver ──(signed HTTPS)──▶ Coordinator
                                                                                   │
                                                       webhook / poll ────────────┤
                                                                                   ▼
                                                              Developer backend + SDK
                                                                                   │
                                                                  renders / status ▼
                                                                        Client widget
```

## Components

### Coordinator (`coordinator/`, Go)
The control plane. Owns:
- **Session lifecycle:** `pending → verified | expired | failed`.
- **Number pool + routing:** picks an available number for each session.
- **Matching:** maps an inbound signal to exactly one live session.
- **Abuse control:** rate limits, per-sender attempt caps, auto-blocking.
- **Webhooks:** notifies the developer backend on `verification.verified`.

State: Postgres for durable records, Redis for hot session/queue/rate state.

### Receiver (`receiver-android/`, `receiver-pi/`)
Holds the SIM(s) and listens for inbound SMS and calls. Auto-rejects calls so a missed call is
free for the user. Reports inbound events to the Coordinator with an HMAC signature, a nonce,
and a timestamp. Buffers and retries when offline. Sends periodic heartbeats.

### Backend SDKs (`sdk-server-*`)
Thin wrappers the developer's backend calls: `startVerification()`, `checkStatus()`, and a
webhook signature verifier.

### Client widget / SDKs (`widget-web/`, `sdk-client-flutter/`)
Render the instruction UI (`Send 4729 to 017…`) with tap-to-send deep links and show live
status via server-sent events.

## Data model

See [`coordinator/migrations/0001_init.sql`](../coordinator/migrations/0001_init.sql). Core
tables: `apps`, `devices`, `numbers`, `sessions`, `inbound_events`, `blocks`.

Key invariant: **a code is unique among pending sessions on a given number**, enforced by a
partial unique index, so an inbound signal resolves to exactly one live session.

## Request flow (SMS + code)

1. Developer backend → `POST /v1/verifications`.
2. Coordinator picks number **N**, generates code **C**, stores a pending session, returns
   `{number: N, code: C, deep_link, expires_at}`.
3. Widget shows the instruction; the user sends an SMS with **C** to **N**.
4. Receiver on **N** posts `POST /v1/inbound {sender, body, ...}` (signed).
5. Coordinator matches `(N, C)`, sets the session verified, records the verified number per the
   binding mode, and fires the webhook. The widget flips to success.

## API surface

| Audience | Endpoint | Purpose |
|---|---|---|
| Developer | `POST /v1/verifications` | start a verification |
| Developer | `GET /v1/verifications/{id}` | poll status |
| Device | `POST /v1/devices/register` | register a receiver |
| Device | `POST /v1/devices/heartbeat` | liveness + commands |
| Device | `POST /v1/inbound` | report an inbound signal |
| Ops | `GET /healthz`, `GET /readyz` | health / readiness |
