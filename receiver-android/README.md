# receiver-android

Receiver app for a spare Android phone. Holds the SIM, listens for inbound SMS and calls, and
reports signed inbound events to the Coordinator.

> **Status: planned (Phase 1 for SMS, Phase 2 for missed call).** This directory is a
> placeholder; the Kotlin app has not landed yet.

## Scope

- Read incoming SMS (sender + body) for the **SMS + code** channel.
- Observe call state and caller ID, auto-reject, for the **missed call** channel.
- Register with the Coordinator (device pairing), send heartbeats, report signal strength.
- Sign every inbound event (HMAC + nonce + timestamp); buffer and retry when offline.
- Optional Play Integrity attestation.

## Notes

- `READ_SMS`/`RECEIVE_SMS` are restricted on Google Play, so distribution is via sideloaded APK
  or by running as the default SMS handler.
- DTMF capture is **not** possible here (in-call audio is locked since Android 10). Use
  [`receiver-pi`](../receiver-pi) for DTMF.

## Planned stack

Kotlin, Android SDK, Gradle.
