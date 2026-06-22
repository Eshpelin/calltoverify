# CallToVerify — Design Blueprint

**Reverse / inbound OTP.** Instead of the developer sending an OTP, the end user proves
control of their phone by sending an inbound signal (SMS, missed call, or DTMF) to a
self-hosted receiver (a cheap Android phone or a Raspberry Pi + GSM modem). An on-screen
code binds the inbound signal to a verification session. Cost moves to the user, the
developer needs no SMS gateway, and there is no send-side carrier/BTRC blocking.

## Decisions (locked)
- Build the **whole ecosystem**: all three channels — SMS+code, missed-call, DTMF (Pi only).
- Ship **open-source self-host first**. Hosted SaaS is a later layer on the same core.
- **Configurable binding** per app (`derive` vs `claim`), optional per-session override.
- Working name: **CallToVerify**.

## Reality checks
- **DTMF during a call is not feasible on stock Android** — in-call voice audio (`VOICE_CALL`)
  is locked for non-system apps since Android 10. DTMF capture only on Raspberry Pi + GSM
  modem via Asterisk `chan_dongle`. So SMS+code is the primary phone channel; DTMF is Pi-only.
- **Google Play restricts `READ_SMS`/`RECEIVE_SMS`.** Fine for self-host (sideload APK or
  default-SMS role); blocks a frictionless Play Store listing.
- **Voice concurrency = 1 call per SIM** → DTMF/missed-call need a queue + SIM pool. SMS does
  not (the code multiplexes one number across many concurrent sessions).
- **Missed-call has no code in the call → caller-ID is the binder → missed-call implies `claim`
  mode.** SMS and DTMF carry a code and work in either binding mode.

## Components (OSS monorepo)
```
calltoverify/
  coordinator/        # brain. Node/TS or Go + Postgres + Redis. Docker.
  receiver-android/   # Kotlin. SMS read, call-state, auto-reject, heartbeat.
  receiver-pi/        # Python daemon. gammu-smsd (SMS) + Asterisk chan_dongle (DTMF/call).
  sdk-server-node/
  sdk-server-php/     # Laravel — BD market
  sdk-server-python/
  widget-web/         # embeddable JS client
  sdk-client-flutter/
  docs/
  docker-compose.yml  # one-command self-host
```

## Data model
- **app** (tenant/API key): `api_key, api_secret, webhook_url, webhook_secret,
  config{binding_mode, code_len=6, ttl=90s, channels_enabled[]}`.
- **device**: `device_secret, type(android|pi), status, last_heartbeat, capabilities[]`.
- **number** (MSISDN): `device_id, msisdn, channels[], status, rate_caps`.
- **session**: `app_id, channel, status(pending|verified|expired|failed), number_id, code,
  binding_mode, claimed_msisdn?, verified_msisdn?, created_at, expires_at, attempts`.
- **inbound_event**: `{number_id, type, sender, body|dtmf, ts, matched_session_id}`.
- **block**: `target(msisdn|ip), reason, until`.

## Binding modes
- `derive` → `verified_msisdn = inbound sender`. No number typed. Best for signup. Needs a code.
- `claim` → require `inbound sender == claimed_msisdn` else fail. Needs number known first.
- Per-app default + optional per-session override. Coordinator rejects invalid
  channel/binding combos early (e.g. missed-call + derive).

## Channel mechanics

| Channel | Receiver signal | Android | Pi | Binding | Concurrency/SIM |
|---|---|---|---|---|---|
| SMS + code | incoming SMS sender+body | `RECEIVE_SMS`/default-SMS role | gammu-smsd | derive or claim | high |
| Missed-call | caller-ID via call-state, auto-reject | `READ_PHONE_STATE`/`READ_CALL_LOG` | chan_dongle / gammu | claim only | low (timing window) |
| DTMF on call | answer + capture tones | not feasible | Asterisk `chan_dongle` IVR | derive or claim | 1 voice call/SIM |

Missed-call binding: assign a specific pool number N per session + short window; screen says
"missed-call N from your number now"; `caller-ID == claimed` within window → verified. Rotate N
across the pool to blunt blind spam. Same N + same window for two users = collision → queue.

## API contracts

Dev-facing (SDK → Coordinator, auth = api_key + HMAC):
- `POST /v1/verifications` `{channel?, binding_mode?, claimed_msisdn?, metadata}` →
  `{session_id, channel, instructions{number, code?, action, deep_link, expires_at}, status}`.
- `GET /v1/verifications/{id}` → `{status, verified_msisdn?}`.
- Webhook → dev backend: `{event:"verification.verified", session_id, verified_msisdn,
  channel, ts}` + HMAC sig.

Device-facing (Receiver ↔ Coordinator, auth = device_secret HMAC + nonce, replay-protected):
- `POST /v1/devices/register` → config + active numbers.
- `POST /v1/devices/heartbeat` `{status, signal, queue_depth}` → commands (block-list,
  expected sessions, queue positions).
- `POST /v1/inbound` `{number, type, sender, body|dtmf, ts, nonce, sig}` → `{matched,
  session_id?}`. Coordinator does the match, not the device.
- Realtime: device long-poll/WS for "expect call for session X" + queue updates.

Client widget: deep links `sms:N?body=C`, `tel:N`; status via SSE. Self-host default proxies
client → dev-backend → Coordinator (no public Coordinator). Direct session-token mode optional.

## Security / abuse
- Brute force: 6-digit code, TTL 60–90s, single-use, cap active sessions/number, per-sender
  attempt cap, auto-block.
- Spoofing: SMS sender / caller-ID spoofable by sophisticated actors (same class as SMS-OTP).
  Document the threat model; offer optional 2nd factor for high-security apps.
- Rogue receiver: HMAC + nonce + timestamp on every inbound; Play Integrity attestation
  (Android); mTLS option. Self-host = dev owns the device → trust is acceptable.
- DDOS: cost now sits on the attacker. Still protect the SIM with a multi-SIM pool +
  auto-block on flooding senders.
- Privacy: hash MSISDN at rest; self-host keeps numbers on dev infra.

## Scaling
- The code multiplexes one SMS number across many concurrent sessions → SMS needs few SIMs.
- DTMF/voice = 1 call/SIM → queue + SIM pool mandatory. Widget shows queue position.
- Pool routing + failover; heartbeat-driven; alert the dev when a device goes offline.

## Build phases (delivers all 3 channels)
1. **Core + SMS**: coordinator (sessions/numbers/devices/rate-limit/webhooks/docker-compose),
   Android receiver SMS-mode, Node SDK, web widget. Both binding modes. End-to-end on one phone.
2. **Missed-call**: Android call-state + auto-reject + caller-ID inbound; claim-mode flow +
   per-session number assignment + queue scaffold; basic dashboard (device health, sessions).
3. **Pi + DTMF**: receiver-pi → gammu-smsd (SMS parity) + Asterisk `chan_dongle` IVR (DTMF);
   voice queue; pool routing + failover.
4. **Harden + ecosystem**: Play Integrity attestation, abuse intelligence/auto-block,
   PHP/Laravel + Flutter SDKs, metrics/alerting, optional hosted-SaaS layer.
