# receiver-pi

Receiver daemon for a Raspberry Pi (or any Linux box) with a GSM modem. Supports all three
channels, including DTMF, which the Android receiver cannot do.

> **Status: planned (Phase 3).** Placeholder directory.

## Scope

- **SMS + code:** read inbound SMS via `gammu-smsd`.
- **Missed call:** read caller ID, auto-reject.
- **DTMF:** answer the call and capture keypad tones via Asterisk (`chan_dongle`), running an
  IVR prompt.
- Register, heartbeat, and report signed inbound events to the Coordinator, with offline
  buffering and retry.

## Planned stack

Python daemon, `gammu-smsd` for SMS, Asterisk + `chan_dongle` for voice/DTMF. Typical hardware:
Raspberry Pi + a USB GSM modem (for example a Huawei dongle) or a SIM800/SIM900 module.
