<div align="center">

# CallToVerify

**Free, self-hosted phone number verification. The user contacts you, not the other way around.**

[![License](https://img.shields.io/badge/license-Apache--2.0-blue.svg)](LICENSE)
[![Status](https://img.shields.io/badge/status-alpha-orange.svg)](#project-status)
[![CI](https://img.shields.io/badge/CI-pending-lightgrey.svg)](.github/workflows/ci.yml)
[![PRs welcome](https://img.shields.io/badge/PRs-welcome-brightgreen.svg)](CONTRIBUTING.md)

</div>

---

## The problem

Sending OTP SMS to verify a phone number is expensive, slow, unencrypted, and a standing
cost-and-DDoS liability. Automated abuse can rack up thousands of OTP sends overnight, each one
billed to you. In Bangladesh and many other markets it is worse: cross-border delivery gets
blocked, gateways demand large upfront fees for a masked sender, and the APIs are poor.

## The idea

**Flip the direction.** Instead of *you* sending a code to the user, the user sends an inbound
signal to a phone number *you* control. A code shown on their screen binds that signal to a
verification session. You receive on a cheap spare Android phone or a Raspberry Pi with a SIM.

- **Receiving SMS and calls is free.** No gateway, no masked-sender fee, no send-side blocking.
- **The cost moves to the user** (one SMS, or a free missed call).
- **DDoS economics flip** onto the attacker, and abusive senders are auto-blocked.
- **One spare phone is enough to start.**

```
  End user's phone                 Your receiver                    Your backend
  ┌──────────────┐  SMS / call /  ┌──────────────┐   signed HTTPS  ┌──────────────┐
  │  enters code │ ─ missed call ─▶  Android app  │ ──────────────▶ │  Coordinator │
  │  from screen │    / DTMF       │  or Pi + SIM │                 │  + your SDK  │
  └──────────────┘                 └──────────────┘                 └──────────────┘
         ▲                                                                  │
         └──────────────────  "verified" pushed back to the UI  ◀───────────┘
```

## Screenshots

### End-user experience

What the person verifying sees: a channel chooser, a per-channel instruction with a tap-to-send
deep link and a live countdown, and a verified state. No code to wait for.

<p>
  <img src="https://raw.githubusercontent.com/Eshpelin/calltoverify/main/docs/screenshots/enduser-chooser.png" width="300" alt="Channel chooser: text us a code, or give a missed call" />
  <img src="https://raw.githubusercontent.com/Eshpelin/calltoverify/main/docs/screenshots/enduser-sms.png" width="300" alt="SMS instruction with the code as digit chips and an Open messages button" />
  <img src="https://raw.githubusercontent.com/Eshpelin/calltoverify/main/docs/screenshots/enduser-success.png" width="300" alt="Verified success state showing the verified number" />
</p>

### Self-hosted console

The implementer's view ([`coordinator/examples/dashboard`](coordinator/examples/dashboard)): a
guided onboarding wizard — pair a phone, run a test verification — plus a live dashboard of
receivers and recent verifications. Embeds the engine, so there is no separate service and no
database to run.

<p>
  <img src="https://raw.githubusercontent.com/Eshpelin/calltoverify/main/docs/screenshots/console.png" width="820" alt="Self-hosted console: onboarding wizard, receivers table, and recent verifications" />
</p>

### Android receiver app

The spare phone that holds the SIM. It scans a pairing QR, then shows a live connected status with
the endpoint, provisioned numbers, and recent inbound activity.

<p>
  <img src="https://raw.githubusercontent.com/Eshpelin/calltoverify/main/docs/screenshots/android-pairing.png" width="260" alt="Android scan-to-pair screen" />
  <img src="https://raw.githubusercontent.com/Eshpelin/calltoverify/main/docs/screenshots/android-connected.png" width="260" alt="Android connected status screen" />
</p>

## How verification works

1. Your backend calls the SDK: `startVerification()`.
2. The **Coordinator** picks an available number from your pool, generates a short code, and
   returns instructions.
3. The user's screen shows: *"Send `4729` to `017XXXXXXXX`"* with a tap-to-send deep link.
4. The user sends it. Your **receiver** captures the inbound signal and posts a signed event.
5. The Coordinator matches `(number, code)`, marks the session verified, and the result is
   pushed back to your UI and your backend webhook.

The code lets one number serve many simultaneous verifications. The verified phone number can
be **derived** from the sender, or **claimed** up front and matched. See
[`docs/channels.md`](docs/channels.md).

## Channels

| Channel | What the user does | Android | Raspberry Pi | Binding |
|---|---|---|---|---|
| **SMS + code** | Sends an SMS containing the code | Yes | Yes | derive or claim |
| **Missed call** | Gives a free missed call | Yes | Yes | claim only |
| **DTMF on call** | Calls and types the code | No (audio locked) | Yes (Asterisk) | derive or claim |

## Repository layout

| Path | What it is | Stack |
|---|---|---|
| [`coordinator/`](coordinator) | The brain: sessions, number pool, matching, webhooks | Go |
| [`receiver-android/`](receiver-android) | Receiver app for a spare Android phone | Kotlin |
| [`receiver-pi/`](receiver-pi) | Receiver daemon for Raspberry Pi + GSM modem | Python |
| [`sdk-server-node/`](sdk-server-node) | Backend SDK | Node / TS |
| [`sdk-server-php/`](sdk-server-php) | Backend SDK | PHP / Laravel |
| [`sdk-server-python/`](sdk-server-python) | Backend SDK | Python |
| [`widget-web/`](widget-web) | Embeddable verification UI | JS |
| [`sdk-client-flutter/`](sdk-client-flutter) | Mobile client SDK | Flutter |
| [`docs/`](docs) | Documentation | — |

## Quick start

### Embed it in your Go backend (no separate service, no database to run)

```go
import ctv "github.com/Eshpelin/calltoverify/coordinator/engine"

eng, _ := ctv.New(ctx, ctv.Options{
    OnVerified: func(ev ctv.Event) { /* mark the user verified */ },
})
mux.Handle("/ctv/", eng.DeviceHandler("/ctv")) // the receiver app posts here

// in your signup handler:
v, _ := eng.StartVerification(ctx, ctv.Params{Channel: "sms"})
// show v.Instructions to the user; poll eng.Status or use OnVerified
```

SQLite is the default, so there is nothing else to run. Enroll a spare phone with
`eng.NewPairing(...)`, which returns a QR payload the Android app scans.

Two runnable examples: a minimal API in [`coordinator/examples/embedded`](coordinator/examples/embedded),
and a full **self-hosted console** in [`coordinator/examples/dashboard`](coordinator/examples/dashboard)
(`go run ./examples/dashboard`) — a guided wizard to pair a phone, run a test verification, and a
live dashboard of receivers and recent verifications.

### Or run the standalone Coordinator (for non-Go backends)

```bash
git clone https://github.com/Eshpelin/calltoverify.git
cd calltoverify
docker compose up --build
curl http://localhost:8080/healthz
```

Then use a backend SDK ([Node](sdk-server-node), [Python](sdk-server-python)) against its REST
API. Full guide: [`docs/getting-started.md`](docs/getting-started.md).

## Project status

**Alpha, but the whole loop works end to end** for all three channels (SMS, missed call, DTMF).
A cross-language end-to-end test ([`scripts/e2e.sh`](scripts/e2e.sh)) drives the Go engine with
the Python receiver client and verifies a number over HTTP.

| Component | What | Verified |
|---|---|---|
| [`coordinator`](coordinator) + [`engine`](coordinator/engine) | Control plane: verification loop, auth, webhooks, embeddable engine (SQLite/Postgres), QR pairing | Unit + Postgres integration + E2E |
| [`sdk-server-node`](sdk-server-node) | Node/TS backend SDK | Unit (CI) |
| [`sdk-server-python`](sdk-server-python) | Python backend SDK | Unit (CI) |
| [`sdk-server-php`](sdk-server-php) | PHP backend SDK (Laravel-friendly) | PHPUnit (CI) |
| [`widget-web`](widget-web) | Vanilla JS verification widget | Unit (CI) |
| [`sdk-client-react`](sdk-client-react) | React component | SSR tests (CI) |
| [`sdk-client-flutter`](sdk-client-flutter) | Flutter client | `flutter test` (CI) |
| [`receiver-pi`](receiver-pi) | Raspberry Pi receiver (SMS/missed-call/DTMF) | Client/signing unit-tested (CI) + E2E |
| [`receiver-android`](receiver-android) | Android receiver (SMS/missed-call) | Reviewed; not compiled here (no Android SDK) |

Remaining roadmap: hardening (Play Integrity attestation, Redis-backed rate-limit/nonce for
multi-instance deployments, hosted SaaS). Per-SIM voice queueing, an ops dashboard, the
onboarding wizard, and sender auto-blocking are already implemented. See
[`DESIGN.md`](DESIGN.md) for the full blueprint.

## Security

Phone verification is a security primitive. Please read the threat model in
[`docs/security.md`](docs/security.md), and report vulnerabilities privately per
[`SECURITY.md`](SECURITY.md) rather than in public issues.

## Contributing

Contributions are very welcome. Start with [`CONTRIBUTING.md`](CONTRIBUTING.md) and our
[`CODE_OF_CONDUCT.md`](CODE_OF_CONDUCT.md).

## License

[Apache License 2.0](LICENSE). Copyright 2026 The CallToVerify Authors.
