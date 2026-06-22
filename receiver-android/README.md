# receiver-android

Receiver app for a spare Android phone. Holds the SIM, listens for inbound SMS and missed calls,
and reports signed inbound events to the Coordinator.

The app is a single-Activity Jetpack Compose UI backed by a foreground service that keeps the
device "online" with periodic heartbeats and drains a durable retry buffer.

## What it does

- **SMS channel.** Reads incoming SMS (sender + body) and reports them to the Coordinator.
- **Missed-call channel.** Detects a ringing call, reads the caller number, auto-rejects the
  call before it connects (so the user is not charged), and reports it.
- **Pairing.** Scans a pairing QR from the dashboard, stores the credentials in encrypted
  preferences, and registers the device.
- **Liveness.** A foreground service registers on start, then heartbeats every 60s and flushes
  any inbound events that were buffered while offline. It restarts on boot if still paired.
- **Signing.** Every device request is HMAC-SHA256 signed (timestamp + nonce + raw body); failed
  inbound reports are buffered to a durable FIFO queue and retried.

## Build

Standard Gradle Android build (Kotlin DSL, Compose):

```sh
./gradlew :app:assembleDebug      # debug APK
./gradlew :app:assembleRelease    # release APK (un-minified by default)
```

Requires JDK 17+ and the Android SDK (compileSdk 34, minSdk 24). The Gradle wrapper is checked in.

## Test it on an emulator

You can run the whole flow without a SIM. Start a backend (e.g. `go run ./coordinator/examples/dashboard`),
boot an emulator, then:

```sh
./gradlew :app:assembleDebug
adb install -r -g app/build/outputs/apk/debug/app-debug.apk

# Pair without a camera. The pairing JSON is what the dashboard QR encodes; use the
# host alias 10.0.2.2 so the emulator can reach your machine:
adb shell am start -n org.calltoverify.receiver/.ui.MainActivity \
  --es ctv_pairing '{"endpoint":"http://10.0.2.2:8080/ctv","device_id":"...","device_secret":"..."}'

# Start a verification in the dashboard, then deliver a fake inbound SMS with the code:
adb emu sms send +8801712345678 918604
```

The app reports the SMS to the backend, which verifies it. (Cleartext HTTP is permitted only to
`localhost` / `10.0.2.2` for this local testing — production traffic is HTTPS; see
`res/xml/network_security_config.xml`.)

## Sideload only

`RECEIVE_SMS` / `READ_CALL_LOG` are **restricted permissions on Google Play**, so this app is
distributed by **sideloaded APK**, not the Play Store. Install the APK directly on the receiver
phone (enable "install unknown apps" for your file manager / browser).

## Pairing

1. Open the app. It shows a **Scan to pair** screen.
2. Grant camera access when prompted.
3. Scan the pairing QR from your CallToVerify dashboard. The QR encodes:

   ```json
   {"endpoint":"https://your-backend/ctv","device_id":"<uuid>","device_secret":"<secret>"}
   ```

4. The app stores the credentials, registers the device, and switches to the **Connected** screen
   showing the endpoint, device id, provisioned number(s), online status, and a log of recent
   inbound events. Use **Unpair this device** to forget the credentials and stop the service.

## Permissions

| Permission | Why |
| --- | --- |
| `INTERNET`, `ACCESS_NETWORK_STATE` | Talk to the backend. |
| `RECEIVE_SMS` | SMS channel: read inbound texts. |
| `READ_PHONE_STATE`, `READ_CALL_LOG` | Missed-call channel: observe ringing calls and read the caller number. |
| `ANSWER_PHONE_CALLS` | Auto-reject incoming calls (`TelecomManager.endCall()`, API 28+). |
| `CAMERA` | Scan the pairing QR. |
| `FOREGROUND_SERVICE`, `FOREGROUND_SERVICE_DATA_SYNC`, `POST_NOTIFICATIONS` | Persistent heartbeat service + its ongoing notification. |
| `RECEIVE_BOOT_COMPLETED` | Restart the service after a reboot. |

The runtime permissions (phone state, call log, answer calls, SMS, notifications) are requested
once after pairing. The caller number is only available when `READ_CALL_LOG` is granted; auto-reject
additionally requires `ANSWER_PHONE_CALLS` and API 28+. Without those, the app still reports the
call but cannot reject it.

## Notes

- DTMF capture is **not** possible here (in-call audio is locked since Android 10). Use
  [`receiver-pi`](../receiver-pi) for DTMF.
- The device learns its own provisioned MSISDN from the server's register/heartbeat response, not
  from the SIM (reading the SIM number off Android is unreliable).

## Stack

Kotlin, Jetpack Compose (Material 3), CameraX + ML Kit barcode scanning, OkHttp, AndroidX Security
(EncryptedSharedPreferences). Gradle Kotlin DSL.
