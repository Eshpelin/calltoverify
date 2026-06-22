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

## Quick start (self-host)

> Requires Docker.

```bash
git clone https://github.com/Eshpelin/calltoverify.git
cd calltoverify
docker compose up --build
# Coordinator health check:
curl http://localhost:8080/healthz
```

Postgres and Redis come up alongside the Coordinator, and the schema migrations run
automatically on first boot. Full guide: [`docs/getting-started.md`](docs/getting-started.md).

## Project status

**Alpha. Under active construction.** Roadmap:

1. **Core + SMS** — Coordinator session lifecycle, Android SMS receiver, Node SDK, web widget.
   _Coordinator done: session lifecycle, inbound matching for all three channels, provisioning,
   bearer + HMAC auth, signed webhooks ([end-to-end guide](docs/getting-started.md))._
2. **Missed call** — caller-ID capture, claim-mode flow, queueing, device dashboard.
3. **Pi + DTMF** — Raspberry Pi receiver with Asterisk, voice queue, pool failover.
4. **Harden + ecosystem** — attestation, abuse intelligence, PHP/Flutter SDKs, hosted SaaS.

The Coordinator backend already implements the full verification loop for SMS, missed call, and
DTMF. The receivers and SDKs that drive it are next. See [`DESIGN.md`](DESIGN.md) for the full
blueprint.

## Security

Phone verification is a security primitive. Please read the threat model in
[`docs/security.md`](docs/security.md), and report vulnerabilities privately per
[`SECURITY.md`](SECURITY.md) rather than in public issues.

## Contributing

Contributions are very welcome. Start with [`CONTRIBUTING.md`](CONTRIBUTING.md) and our
[`CODE_OF_CONDUCT.md`](CODE_OF_CONDUCT.md).

## License

[Apache License 2.0](LICENSE). Copyright 2026 The CallToVerify Authors.
