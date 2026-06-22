# sdk-server-node

Backend SDK for Node.js / TypeScript. The thin client your server uses to talk to the
Coordinator.

> **Status: planned (Phase 1).** Placeholder directory.

## Planned API

```ts
const ctv = new CallToVerify({ baseUrl, apiKey, apiSecret });

// Start a verification (binding mode and channel optional; defaults from app config).
const v = await ctv.startVerification({ channel: "sms", claimedMsisdn });
// v.instructions -> { number, code, deepLink, expiresAt }

// Poll, or rely on the webhook.
const status = await ctv.checkStatus(v.sessionId);

// Verify a webhook signature in your HTTP handler.
ctv.verifyWebhook(rawBody, signatureHeader); // throws on mismatch
```

## Planned stack

TypeScript, zero/minimal dependencies, published to npm.
