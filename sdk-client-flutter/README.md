# calltoverify (Flutter)

Flutter client for CallToVerify. A drop-in `CallToVerify` widget renders the multi-channel
verification UX (chooser, per-channel instructions, waiting + countdown, success / expiry) in the
bold branded style, plus a headless `VerificationController` if you want your own UI.

> **Status: alpha.**

## Install

```yaml
dependencies:
  calltoverify: ^0.1.0
```

## Usage

```dart
import 'package:calltoverify/calltoverify.dart';
// optionally: import 'package:url_launcher/url_launcher.dart';

CallToVerify(
  // Priority order: the first is the primary (shown first), the rest are alternatives.
  channels: const [Channel.sms, Channel.call],
  start: (channel) async {
    final res = await myApi.startVerification(channel?.wire); // your backend
    return StartResult.fromJson(res);
  },
  status: (sessionId) async {
    final res = await myApi.checkStatus(sessionId);
    return StatusResult.fromJson(res);
  },
  onVerified: (msisdn) {
    // advance your flow
  },
  // Wire the tap-to-send deep link to url_launcher:
  onOpenLink: (url) => launchUrl(Uri.parse(url)),
);
```

`start` and `status` call **your** backend, never the Coordinator directly, so the API key stays
server-side.

## Headless controller

```dart
final c = VerificationController(start: start, status: status, channels: [Channel.sms]);
c.addListener(() { /* rebuild on c.phase */ });
await c.begin(Channel.sms);
await c.poll();
```

## Develop

```bash
flutter pub get
flutter test
```

Tests cover the controller state machine, snake_case JSON parsing, and widget rendering.
