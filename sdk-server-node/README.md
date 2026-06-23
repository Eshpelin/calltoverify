# @calltoverify/sdk

Backend SDK for Node.js / TypeScript. A thin, dependency-free client for the CallToVerify
Coordinator's developer API.

> **Status: alpha (Phase 1 implemented).** Start verifications, poll status, verify webhooks.

## Install

```bash
npm install @calltoverify/sdk
```

Requires Node 18+ (uses the global `fetch`).

## Usage

```ts
import { CallToVerify } from "@calltoverify/sdk";

const ctv = new CallToVerify({
  baseUrl: "https://verify.example.com", // your Coordinator
  apiKey: process.env.CTV_API_KEY!,
  webhookSecret: process.env.CTV_WEBHOOK_SECRET, // only needed for verifyWebhook
});

// Start a verification. Returns instructions to show the user.
const v = await ctv.startVerification({ channel: "sms" });
// v.instructions -> { number, code, channel, action, deepLink, expiresAt }

// Poll, or rely on the webhook.
const status = await ctv.checkStatus(v.sessionId);
if (status.status === "verified") {
  console.log("verified number:", status.verifiedMsisdn);
}
```

### Claim mode

```ts
const v = await ctv.startVerification({
  channel: "sms",
  bindingMode: "claim",
  claimedMsisdn: "+8801712345678",
});
```

### Verifying webhooks

```ts
// In your HTTP handler, pass the RAW request body and the X-CTV-Signature header.
try {
  const event = ctv.verifyWebhook(rawBody, req.headers["x-ctv-signature"]);
  // event -> { event, sessionId, verifiedMsisdn, channel, ts }
} catch (err) {
  // CallToVerifyError on signature mismatch -> reject the request
}
```

## API

- `new CallToVerify({ baseUrl, apiKey, webhookSecret?, fetch?, timeoutMs? })` (`timeoutMs` defaults to 10000)
- `startVerification(params?) => Promise<Verification>`
- `checkStatus(sessionId) => Promise<VerificationStatus>`
- `verifyWebhook(rawBody, signature, maxAgeSeconds?) => WebhookEvent` (throws `CallToVerifyError` on signature mismatch; pass `maxAgeSeconds` to also reject events whose `ts` is older than the window)

Non-2xx responses, timeouts, network errors, and non-JSON error bodies all throw `CallToVerifyError` with `status` and `code`.

## Develop

```bash
npm install
npm test     # builds, then runs node:test against the compiled output
```
