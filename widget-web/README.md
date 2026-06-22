# @calltoverify/widget

The embeddable web widget for CallToVerify. It renders the full end-user experience: an optional
channel chooser, the per-channel instruction ("Send `4729` to `017…`", "give a missed call", or
"call and enter the code") with a tap-to-send deep link and a live countdown, status polling, and
the success / expired states. Bold, branded, and fully themeable.

> **Status: alpha (Phase 1 + multi-channel UX).**

## How it talks to the backend

The widget calls **your** backend, never the Coordinator directly, so your API key stays on the
server. You provide `start` (your backend calls the engine / SDK's start) and `status`.

```ts
import { mount } from "@calltoverify/widget";

mount("#verify", {
  // Channels in priority order: the first is the primary (shown first), the rest
  // are alternatives. With more than one, a chooser is shown.
  channels: ["sms", "call"],

  start: async (channel) => {
    const res = await fetch("/api/verify/start", {
      method: "POST",
      headers: { "content-type": "application/json" },
      body: JSON.stringify({ channel }),
    });
    return res.json(); // -> { sessionId, instructions: { number, code, channel, action, deepLink, expiresAt } }
  },
  status: async (sessionId) => {
    const res = await fetch(`/api/verify/status?id=${encodeURIComponent(sessionId)}`);
    return res.json(); // -> { status, verifiedMsisdn? }
  },

  onVerified: (msisdn) => {
    // advance your UI
  },
});
```

## Theming (bold & branded by default)

Re-theme by overriding `--ctv-*` CSS variables, either globally in your CSS or per-mount:

```ts
mount("#verify", {
  /* ... */
  theme: {
    "--ctv-brand": "#0ea5e9",
    "--ctv-radius": "12px",
  },
});
```

Key tokens: `--ctv-brand`, `--ctv-brand-strong`, `--ctv-on-brand`, `--ctv-bg`, `--ctv-surface`,
`--ctv-text`, `--ctv-muted`, `--ctv-success`, `--ctv-radius`. Dark mode is automatic via
`prefers-color-scheme`.

## Translating / re-voicing

Pass `labels` to override any string (each is either a string or a small function):

```ts
mount("#verify", {
  /* ... */
  labels: {
    title: "Verifica tu número",
    smsButton: "Abrir mensajes",
  },
});
```

## Headless controller

To build your own UI, use the controller directly:

```ts
import { createController } from "@calltoverify/widget/controller";

const ctrl = createController({ start, status });
ctrl.on("verified", (m) => {/* ... */});
await ctrl.begin("sms");
// call ctrl.poll() on your own schedule; ctrl.reset() to start over
```

## Develop

```bash
npm install
npm test   # builds, then runs the controller unit tests
```
