<div align="center">

# CallToVerify

**Self-hosted phone number verification. The person verifying contacts a phone you control, instead of you sending them an OTP.**

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

- **Receiving costs you nothing.** No SMS gateway, no masked-sender fee, no send-side blocking.
- **The per-verification cost moves to the end user** — one text message, or a missed call that never connects.
- **DDoS economics flip** onto the attacker, and abusive senders are auto-blocked.
- **One spare phone is enough to start.**

<p align="center">
  <img src="https://raw.githubusercontent.com/Eshpelin/calltoverify/main/docs/screenshots/flow.png" width="760" alt="Flow: the end user sends an SMS, missed call, or DTMF from the number being verified to your receiver (a spare Android phone or Pi); the receiver reports the sender and code to your backend over signed HTTPS; the verified result is pushed back to your app, and the verified number is the sender." />
</p>

## Screenshots

### End-user experience

What the person verifying sees: a channel chooser, then a per-channel instruction with a
tap-to-send deep link and a live countdown, and finally a verified state. The same widget renders
all three channels — text a code, give a missed call, or call and enter a code.

<p>
  <img src="https://raw.githubusercontent.com/Eshpelin/calltoverify/main/docs/screenshots/enduser-chooser.png" width="300" alt="Channel chooser: text us a code, or give a missed call" />
  <img src="https://raw.githubusercontent.com/Eshpelin/calltoverify/main/docs/screenshots/enduser-sms.png" width="300" alt="SMS instruction with the code as digit chips and an Open messages button" />
  <img src="https://raw.githubusercontent.com/Eshpelin/calltoverify/main/docs/screenshots/enduser-success.png" width="300" alt="Verified success state showing the verified number" />
</p>

The missed-call and DTMF flows. Missed call costs the user nothing (it never connects); DTMF has
the user enter the on-screen code on the keypad during a call.

<p>
  <img src="https://raw.githubusercontent.com/Eshpelin/calltoverify/main/docs/screenshots/enduser-call.png" width="300" alt="Missed-call instruction: give a quick missed call, with the number, three steps, and a Call now button" />
  <img src="https://raw.githubusercontent.com/Eshpelin/calltoverify/main/docs/screenshots/enduser-dtmf.png" width="300" alt="DTMF instruction: call the number and enter the code on the keypad" />
</p>

### Self-hosted console

The implementer's view ([`coordinator/examples/dashboard`](coordinator/examples/dashboard)): a
left-nav console to add and manage receivers, run a test verification, and watch a live dashboard
of receivers and recent verifications across all three channels. Embeds the engine, so there is no
separate service and no database to run.

<p>
  <img src="https://raw.githubusercontent.com/Eshpelin/calltoverify/main/docs/screenshots/console.png" width="820" alt="Self-hosted console overview: receivers (an Android phone and a Raspberry Pi) and recent SMS, missed-call, and DTMF verifications" />
</p>
<p>
  <img src="https://raw.githubusercontent.com/Eshpelin/calltoverify/main/docs/screenshots/console-devices.png" width="820" alt="Manage devices: each receiver's type, numbers, channels, and status, with a remove action" />
</p>

### Android receiver app

The spare phone that holds the SIM. It scans a pairing QR, then shows a live connected status with
the endpoint, provisioned numbers, and recent inbound activity.

<p>
  <img src="https://raw.githubusercontent.com/Eshpelin/calltoverify/main/docs/screenshots/android-pairing.png" width="260" alt="Android scan-to-pair screen" />
  <img src="https://raw.githubusercontent.com/Eshpelin/calltoverify/main/docs/screenshots/android-connected.png" width="260" alt="Android connected status screen" />
</p>

## How it works

You run a **receiver** — a spare Android phone or a Raspberry Pi with a SIM — and you tell
CallToVerify that SIM's phone number. From then on:

1. A user needs to verify their phone in your app. Your backend calls `startVerification()`.
2. CallToVerify picks one of your receiver numbers and a short code, and returns instructions
   like *"Text `4729` to `017XXXXXXXX`"* (with a tap-to-send deep link).
3. The user sends that text — or gives a missed call, or calls and types the code — from the
   phone they are verifying.
4. The receiver on that SIM captures the inbound signal (the sender's number and the code) and
   reports it to your backend over signed HTTPS.
5. Your backend matches `(number, code)`, marks the session **verified**, and you find out via a
   callback (embedded engine) or a webhook (standalone). The verified phone number is the sender.

You never send an SMS, and no code leaves your infrastructure. One number can run many SMS
verifications at once (the code disambiguates them).

## Concepts

| Term | Meaning |
|---|---|
| **Receiver** (device) | The spare Android phone or Raspberry Pi that holds a SIM and listens for inbound SMS/calls. |
| **Number** (MSISDN) | A receiver SIM's phone number, in your pool. **You provide it when you pair the receiver** — see [How you add a phone](#how-you-add-a-phone-and-where-the-sim-number-comes-from). |
| **Channel** | How the user reaches the receiver: `sms` (text a code), `call` (missed call), or `dtmf` (call and type the code). |
| **Binding mode** | `derive` — the verified number is whoever the inbound came from (best for sign-up). `claim` — you pass the number up front and the inbound must come from it. |
| **Session** | One verification attempt: `pending` → `verified` / `expired`. |
| **Pairing** | Linking a receiver to your backend by scanning a QR. |

## Channels

| Channel | What the user does | Cost to the user | Android receiver | Pi receiver |
|---|---|---|---|---|
| **SMS + code** | Texts the code to your number | one SMS | yes | yes |
| **Missed call** | Calls your number, hangs up before it connects | none | yes | yes |
| **DTMF on call** | Calls your number and types the code | one call | no | yes (via Asterisk) |

> **Why no DTMF on Android?** Reading the keypad tones means reading the live call audio, which
> Android blocks for apps (since Android 10). On a Raspberry Pi, Asterisk owns the call, so DTMF
> works there. SMS and missed call work on both receivers. Binding: missed call is `claim` only
> (no code travels with the call); SMS and DTMF support both `derive` and `claim`.

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

## Quick start — embed in your Go backend

The simplest path: add the engine to your own Go server. There is no separate service to run, and
**no database to set up** — it uses an embedded SQLite file by default (point it at Postgres when
you need scale).

**1. Add the dependency**

```bash
go get github.com/Eshpelin/calltoverify/coordinator/engine
```

**2. Create the engine and mount the receiver endpoint**

```go
import ctv "github.com/Eshpelin/calltoverify/coordinator/engine"

eng, err := ctv.New(ctx, ctv.Options{
    // SQLitePath defaults to "calltoverify.db". Set PostgresDSN for Postgres,
    // RedisURL to share rate-limit/nonce across multiple instances.
    OnVerified: func(ev ctv.Event) {
        // ev.VerifiedMSISDN is a verified phone number for this session — mark the user verified.
    },
})
defer eng.Close()

mux.Handle("/ctv/", eng.DeviceHandler("/ctv")) // the receiver app posts inbound signals here
```

**3. Pair a spare phone** — tell the engine the SIM's number and channels, then show the QR to
scan with the [Android app](receiver-android):

```go
p, _ := eng.NewPairing(ctx, ctv.PairingParams{
    Endpoint: "https://your-backend.example.com/ctv", // where you mounted DeviceHandler
    Name:     "front-desk phone",
    MSISDN:   "+8801700000001",         // the SIM's phone number — you provide it
    Channels: []string{"sms", "call"},
})
// Render p.QRPayload as a QR code; scan it with the app. The phone is now online and in your pool.
```

**4. Verify a user**

```go
v, err := eng.StartVerification(ctx, ctv.Params{Channel: "sms"})
// Show v.Instructions (number, code, a tap-to-send deep link) to the user.
// Render it with a front-end SDK (below) and poll eng.Status, or wait for OnVerified.
```

Don't want to wire anything yet? `go run ./coordinator/examples/dashboard` opens a **console** at
`http://localhost:8080/setup`: a form to pair a phone (enter the SIM number, scan the QR), a button
to run a test verification, and a live dashboard. A bare API example is in
[`coordinator/examples/embedded`](coordinator/examples/embedded).

## Quick start — non-Go backends (standalone Coordinator)

If your backend isn't Go, run the Coordinator as a service and talk to it with a language SDK.

**1. Run it**

```bash
git clone https://github.com/Eshpelin/calltoverify.git && cd calltoverify
docker compose up --build          # Coordinator + Postgres + Redis
```

**2. Create an app** (the admin token is set in `docker-compose.yml`):

```bash
curl -s -X POST localhost:8080/admin/apps \
  -H 'Authorization: Bearer dev-admin-change-me' -d '{"name":"my-app"}'
# -> { "app_id": "...", "api_key": "ctv_...", "webhook_secret": "..." }   (save the api_key)
```

**3. Register a receiver and its number**

```bash
curl -s -X POST localhost:8080/admin/devices -H 'Authorization: Bearer dev-admin-change-me' \
  -d '{"app_id":"APP_ID","name":"front-desk phone","type":"android","capabilities":["sms"]}'
# -> { "device_id": "...", "device_secret": "..." }

curl -s -X POST localhost:8080/admin/numbers -H 'Authorization: Bearer dev-admin-change-me' \
  -d '{"device_id":"DEVICE_ID","msisdn":"+8801700000001","channels":["sms"]}'
```

**4. Pair the phone** with the JSON `{"endpoint":"http://<host>/v1","device_id":"...","device_secret":"..."}`
(encode it as a QR and scan), then call the API from your app with a backend SDK:

```ts
import { CallToVerify } from "@calltoverify/sdk";          // Node — also Python and PHP
const ctv = new CallToVerify({ baseUrl: "http://localhost:8080", apiKey: "ctv_..." });
const v = await ctv.startVerification({ channel: "sms" }); // show v.instructions to the user
const s = await ctv.checkStatus(v.sessionId);              // or verify the signed webhook
```

Full setup walkthrough: [`docs/getting-started.md`](docs/getting-started.md). The actual front-end +
back-end code to wire it into your app: [`docs/integration.md`](docs/integration.md). A runnable
Node + browser example is in [`examples/node-web`](examples/node-web).

## How you add a phone (and where the SIM number comes from)

This is the part people ask about, so to be explicit: **you tell CallToVerify the SIM's phone
number — the app does not.** Reading a SIM's own number off Android is unreliable, so you provide
it when you pair the receiver:

- **Embedded (Go):** pass `MSISDN` to `eng.NewPairing(...)` (step 3 above).
- **Console wizard:** open `/setup`, type the SIM number into the pairing form, generate the QR.
- **Standalone:** `POST /admin/numbers` with the `msisdn`.

The pairing **QR contains only your backend URL and the receiver's credentials — not the number.**
When the app scans it, the phone authenticates to your backend; the number you already provided is
in the pool. From then on, `startVerification()` picks one of your numbers and tells the user to
text or call *that* number. A receiver can hold several numbers (multiple SIMs) — register each.

## What you can configure

**Embedded engine** (`ctv.Options`):

| Option | Default | What it does |
|---|---|---|
| `SQLitePath` | `calltoverify.db` | embedded SQLite file (zero infrastructure) |
| `PostgresDSN` | — | use Postgres instead of SQLite |
| `RedisURL` | — | share rate-limit + replay-nonce across instances |
| `CodeLen` | `6` | verification code length |
| `TTL` | `90s` | how long a code stays valid |
| `OnVerified` | — | callback fired when a number is verified |

**Standalone Coordinator** — environment variables: `CTV_DATABASE_URL`, `CTV_REDIS_URL`,
`CTV_ADMIN_TOKEN`, `CTV_DEFAULT_CODE_LEN`, `CTV_DEFAULT_TTL_SECONDS`, `CTV_LISTEN_ADDR`.

Safety controls are on by default: inbound is rate-limited per sender, a sender is auto-blocked
after repeated failures or flooding, voice channels are limited to one call per SIM at a time, and
each number caps how many pending sessions it will hold.

## Run it on a Raspberry Pi

A Pi + a USB GSM modem is the most reliable receiver, and the only one that can do DTMF. The
outline (full steps in [`receiver-pi/README.md`](receiver-pi)):

1. **Hardware:** a Raspberry Pi and a USB GSM modem (or HAT) with a SIM.
2. **Install:** `cd receiver-pi && pip install -e .` (gives you the `ctv-pi` command).
3. **Pair:** in the console, **Add a device → Raspberry Pi**. A Pi has no camera, so instead of a
   QR the console gives you a ready-to-paste command. SSH into the Pi and run it, then `ctv-pi register`:

   ```sh
   ctv-pi pair '{"endpoint":"https://verify.yourapp.com/ctv","device_id":"…","device_secret":"…"}' --msisdn "+8801700000001"
   ctv-pi register
   ```
4. **SMS** (via gammu): `sudo apt install gammu-smsd` and point its `RunOnReceive` at `ctv-pi on-sms`.
5. **Missed call / DTMF** (via Asterisk): add the provided dialplan + AGI script.
6. **Keep it online:** install the `ctv-pi run` systemd service (heartbeats + retry buffer).

The console gives you that exact command (and a copy button) when you add a Raspberry Pi:

<p>
  <img src="https://raw.githubusercontent.com/Eshpelin/calltoverify/main/docs/screenshots/console-add-pi.png" width="820" alt="Console Add-a-device with Raspberry Pi selected: a ready-to-paste ctv-pi pair command over SSH instead of a QR, with the QR available as a fallback." />
</p>

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
| [`receiver-android`](receiver-android) | Android receiver (SMS/missed-call) | Builds (Gradle); paired + SMS-verified on an Android emulator |

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
