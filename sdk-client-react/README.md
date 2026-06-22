# @calltoverify/react

React component for CallToVerify. Renders the full multi-channel verification UX (chooser,
per-channel instructions, waiting + countdown, success / expiry) in the same bold, branded,
themeable style as the vanilla widget.

> **Status: alpha.**

## Install

```bash
npm install @calltoverify/react
```

`react` (18+) is a peer dependency.

## Usage

```tsx
import { CallToVerify } from "@calltoverify/react";

export function VerifyStep() {
  return (
    <CallToVerify
      channels={["sms", "call"]}
      start={async (channel) => {
        const res = await fetch("/api/verify/start", {
          method: "POST",
          headers: { "content-type": "application/json" },
          body: JSON.stringify({ channel }),
        });
        return res.json(); // { sessionId, instructions: { number, code, channel, action, deepLink, expiresAt } }
      }}
      status={async (sessionId) => {
        const res = await fetch(`/api/verify/status?id=${encodeURIComponent(sessionId)}`);
        return res.json(); // { status, verifiedMsisdn? }
      }}
      onVerified={(msisdn) => {
        // advance your flow
      }}
    />
  );
}
```

`start` and `status` call **your** backend, never the Coordinator directly, so your API key stays
on the server.

## Theming and i18n

```tsx
<CallToVerify
  /* ... */
  theme={{ "--ctv-brand": "#0ea5e9", "--ctv-radius": "12px" }}
  labels={{ title: "Verifica tu número", smsButton: "Abrir mensajes" }}
/>
```

Re-theme via the `--ctv-*` CSS variables; re-voice or translate via `labels`. Dark mode is
automatic.

## Props

| Prop | Type | |
|---|---|---|
| `channels` | `("sms"\|"call"\|"dtmf")[]` | channels to offer (chooser shown if > 1) |
| `start` | `(channel?) => Promise<StartResult>` | start a verification via your backend |
| `status` | `(sessionId) => Promise<StatusResult>` | poll status via your backend |
| `onVerified` | `(msisdn?) => void` | called on success |
| `onExpired` | `() => void` | called on expiry |
| `pollIntervalMs` | `number` | poll cadence (default 2500) |
| `labels` | `Partial<Labels>` | copy overrides |
| `theme` | `Record<string,string>` | `--ctv-*` variable overrides |

## Develop

```bash
npm install
npm test   # builds, then renders states with react-dom/server and asserts the markup
```
