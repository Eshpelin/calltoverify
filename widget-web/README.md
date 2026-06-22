# widget-web

Embeddable verification UI for the web. Renders the instruction ("Send `4729` to `017…`") with
a tap-to-send deep link and shows live status.

> **Status: planned (Phase 1).** Placeholder directory.

## Scope

- Render channel instructions and a countdown to expiry.
- Deep links: `sms:<number>?body=<code>` and `tel:<number>`.
- Live status via server-sent events: `pending → verified`, auto-advance on success.
- Fallback channel switcher (for example, offer missed call if SMS does not arrive).

## Security note

By default the widget talks to your backend, which proxies to the Coordinator, so the
Coordinator need not be public. A direct session-token mode is also planned.

## Planned stack

Framework-agnostic JS, distributed as a small embeddable script and an npm package.
