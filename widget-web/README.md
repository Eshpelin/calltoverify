# @calltoverify/widget

Embeddable web widget for CallToVerify. It renders the verification instruction
("Send `4729` to `017…`") with a tap-to-send deep link and a countdown, polls status, and flips
to a success state when the number is verified.

> **Status: alpha (Phase 1 implemented).**

## How it talks to the backend

The widget calls **your** backend, never the Coordinator directly, so your API key stays on the
server. You provide two callbacks: `start` (your backend calls the Node SDK's `startVerification`)
and `status` (your backend calls `checkStatus`).

```ts
import { mount } from "@calltoverify/widget";

mount("#verify", {
  start: async () => {
    const res = await fetch("/api/verify/start", { method: "POST" });
    return res.json(); // -> { sessionId, instructions: { number, code, channel, action, deepLink, expiresAt } }
  },
  status: async (sessionId) => {
    const res = await fetch(`/api/verify/status?id=${encodeURIComponent(sessionId)}`);
    return res.json(); // -> { status, verifiedMsisdn? }
  },
  onVerified: (msisdn) => {
    console.log("verified:", msisdn);
    // advance your UI
  },
});
```

Style the `.ctv-widget`, `.ctv-action`, `.ctv-button`, `.ctv-countdown`, `.ctv-status`,
`.ctv-success`, and `.ctv-expired` classes to taste.

## Headless controller

If you want to build your own UI, use the controller directly:

```ts
import { createController } from "@calltoverify/widget/controller";

const ctrl = createController({ start, status });
ctrl.on("verified", (m) => {/* ... */});
await ctrl.begin();
// call ctrl.poll() on your own schedule
```

## Develop

```bash
npm install
npm test   # builds, then runs the controller unit tests
```
