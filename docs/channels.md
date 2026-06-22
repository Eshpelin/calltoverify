# Channels and binding modes

A **channel** is how the user reaches your receiver. A **binding mode** decides which phone
number ends up verified.

## Channels

### SMS + code (primary)
The user sends an SMS containing the on-screen code. The receiver reads the sender and body.
- **Strongest practical option.** Works on a stock Android phone and on a Pi.
- One number serves many concurrent verifications because the code disambiguates.
- Android note: `READ_SMS`/`RECEIVE_SMS` are restricted by Google Play, so the app is
  distributed as a sideloaded APK or runs as the default SMS handler.

### Missed call
The user gives a free missed call. The receiver reads the caller ID and auto-rejects.
- **Zero cost to the user.**
- The call carries no code, so the caller ID itself is the binder. This means **missed call
  requires `claim` mode** (see below): the user's number is known up front and must match.
- One voice call per SIM at a time, so this channel needs a timing window and, at scale, a
  queue and a pool of numbers.

### DTMF on call
The user calls and types the code on the keypad.
- **Raspberry Pi only.** Stock Android cannot capture in-call audio (the `VOICE_CALL` source is
  locked since Android 10), so DTMF needs a Pi with a GSM modem and Asterisk (`chan_dongle`).
- One voice call per SIM at a time. Needs a voice queue and number pool.

| Channel | Android | Pi | Code in signal? | Concurrency / SIM |
|---|---|---|---|---|
| SMS + code | yes | yes | yes | high |
| Missed call | yes | yes | no | low |
| DTMF | no | yes | yes | low (1 call) |

## Binding modes

Set per app, with an optional per-session override.

### `derive`
The verified number is taken from the inbound **sender**. The user does not type their number;
whatever number the SMS/DTMF comes from is the verified identity. Best for sign-up. Requires a
code in the signal, so it works with SMS and DTMF, **not** missed call.

### `claim`
The app states the number up front (`claimed_msisdn`). The inbound sender must equal it, or the
session fails. Use when you need the number before verifying (for example, to look up an
existing account). This is the only mode that works for missed call.

### Validity matrix

| Channel | `derive` | `claim` |
|---|---|---|
| SMS + code | yes | yes |
| DTMF | yes | yes |
| Missed call | no | yes |

The Coordinator rejects invalid channel/mode combinations when a verification is created.
