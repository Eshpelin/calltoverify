# Getting started

This guide gets you from nothing to a working phone verification, step by step. No prior knowledge assumed.

## First: which path are you on?

There are two ways to use CallToVerify. Pick one — the steps are different.

- **Your backend is written in Go** → you *embed* the engine. You do **not** create apps or API keys; you add receivers in the console and call the engine from your code. That's it. See [Embed the engine](#path-a-embed-the-engine-go) below (2 steps).
- **Your backend is anything else** (Node, PHP, Python, Ruby, …) → you run the **Coordinator** as a small service and call its REST API. This is most of this guide: [Run the Coordinator](#path-b-run-the-coordinator-any-backend) (5 steps).

> The self-hosted **console** (`go run ./coordinator/examples/dashboard`) is Path A — it embeds the engine, so it has no apps or API keys. Don't run the Path B `curl` commands against it; they belong to the standalone Coordinator you start in Path B, step 1.

---

## Path A: embed the engine (Go)

**Step 1 — add a receiver.** Start the console (`go run ./coordinator/examples/dashboard`, open http://localhost:8080), go to **Add a device**, and pair a spare phone or a Pi. No API key needed.

**Step 2 — call the engine from your code.**

```go
eng, _ := ctv.New(ctx, ctv.Options{
    OnVerified: func(ev ctv.Event) { /* mark ev.VerifiedMSISDN verified in your DB */ },
})
mux.Handle("/ctv/", eng.DeviceHandler("/ctv")) // the receiver posts here

v, _ := eng.StartVerification(ctx, ctv.Params{Channel: "sms"})
// show v.Instructions to the user; the OnVerified callback fires when they pass
```

That's the whole integration. The rest of this guide is **not** needed for Go.

---

## Path B: run the Coordinator (any backend)

### The 3 things you create once, and what they are

Before any verification can happen you create these one time, by calling the Coordinator's **admin API** (a few `curl`s). What each is:

| Thing | What it is | What creating it gives you |
|---|---|---|
| **App** | your application's identity in CallToVerify | an **`api_key`** (your backend uses this) + a **`webhook_secret`** |
| **Device** | one receiver — a spare phone or a Pi | a `device_id` + `device_secret` to pair the physical receiver with |
| **Number** | the SIM phone number that device answers on | the number end users will contact |

> **Two different secrets, don't mix them up.** The **admin token** (`CTV_ADMIN_TOKEN`) is the master key *you choose* when starting the Coordinator; it only creates apps/devices and never goes near your app code. The **`api_key`** is per-app and is what your backend sends on every request. Keep both server-side.

### Prerequisites

- Docker (easiest), or Go 1.25+ to run the Coordinator directly.
- `curl` and `jq` to follow along in a terminal.

### Step 1 — run the Coordinator *(in a terminal)*

```bash
git clone https://github.com/Eshpelin/calltoverify.git && cd calltoverify
docker compose up --build        # starts Postgres, Redis, and the Coordinator on :8080
```

`docker compose` sets the admin token to `dev-admin-change-me` for you (change it for production). Check it's alive:

```bash
curl -s http://localhost:8080/healthz        # → {"status":"ok"}
```

Set two shell variables you'll reuse:

```bash
API=http://localhost:8080
ADMIN=dev-admin-change-me                     # = CTV_ADMIN_TOKEN from docker-compose.yml
```

### Step 2 — create your app, and get your API key *(in a terminal, one time)*

This is **where the API key comes from.** Call `POST /admin/apps` with the admin token:

```bash
curl -s -X POST $API/admin/apps \
  -H "Authorization: Bearer $ADMIN" \
  -H 'Content-Type: application/json' \
  -d '{"name":"my-app","webhook_url":"https://my-app.com/webhooks/ctv"}' | jq
```

```json
{
  "app_id": "…",
  "api_key": "ctv_live_…",      ← your backend uses this (save it; shown once)
  "webhook_secret": "whsec_…",  ← verifies webhooks (save it; shown once)
  "api_key_prefix": "…"
}
```

Save the values into variables for the next steps (`webhook_url` is optional — omit it to poll instead):

```bash
API_KEY=ctv_live_…
APP_ID=…
```

### Step 3 — add a receiver *(in a terminal, one time per phone/Pi)*

Register the device, then its number:

```bash
# the device → returns the credentials you pair the physical receiver with
curl -s -X POST $API/admin/devices -H "Authorization: Bearer $ADMIN" -H 'Content-Type: application/json' \
  -d "{\"app_id\":\"$APP_ID\",\"name\":\"Front desk phone\",\"type\":\"android\",\"capabilities\":[\"sms\"]}" | jq
# → { "device_id": "…", "device_secret": "…" }

# the SIM number that device answers on
curl -s -X POST $API/admin/numbers -H "Authorization: Bearer $ADMIN" -H 'Content-Type: application/json' \
  -d "{\"device_id\":\"DEVICE_ID\",\"msisdn\":\"+8801700000001\",\"channels\":[\"sms\"]}"
```

Now pair the **physical** receiver with `{"endpoint":"http://<your-coordinator>/v1","device_id":"…","device_secret":"…"}`:

- **Android:** encode that JSON as a QR and scan it in the app.
- **Raspberry Pi:** `ctv-pi pair '<that JSON>' --msisdn "+8801700000001"` then `ctv-pi register`.

The receiver heartbeats and goes **online** — it's now ready to report inbound SMS/calls.

### Step 4 — start a verification *(this is your backend code)*

Your server makes exactly two calls, using the **`api_key`** (not the admin token). Shown as `curl`; the [Node](../sdk-server-node), [Python](../sdk-server-python), and [PHP](../sdk-server-php) SDKs wrap them.

```bash
curl -s -X POST $API/v1/verifications -H "Authorization: Bearer $API_KEY" -H 'Content-Type: application/json' \
  -d '{"channel":"sms"}' | jq
```

```json
{ "session_id": "…",
  "instructions": { "number": "+8801700000001", "code": "4729",
                    "action": "Text 4729 to +8801700000001", "deep_link": "sms:…",
                    "expires_at": "…" } }
```

Show `instructions` to the user (the [web widget](../widget-web) / [React](../sdk-client-react) / [Flutter](../sdk-client-flutter) components render this for you).

### Step 5 — get the result *(your backend code)*

Either **poll**:

```bash
curl -s $API/v1/verifications/SESSION_ID -H "Authorization: Bearer $API_KEY" | jq
# → { "status": "verified", "verified_msisdn": "+8801712345678", … }
```

…or, if you set a `webhook_url`, the Coordinator **POSTs** you the result. Verify the signature before trusting it:

```http
POST https://my-app.com/webhooks/ctv
X-CTV-Event: verification.verified
X-CTV-Signature: <hex HMAC-SHA256 of the raw body, keyed with your webhook_secret>

{ "event":"verification.verified", "session_id":"…", "verified_msisdn":"+8801712345678", "channel":"sms", "ts":"…" }
```

Recompute `HMAC-SHA256(webhook_secret, raw_body)`, compare it (constant-time) to the header, then mark the user verified.

---

## Test the whole loop without a phone (optional)

Don't have a receiver paired yet? You can play the part of the phone from a terminal. **You never write this code** — it's what the Android app / Pi does for you — but it lets you see the full loop. It signs requests as the device using the `device_secret` from step 3:

```bash
DEVICE_ID=…; DEVICE_SECRET=…
ctv_device_post() {  # $1=path  $2=json body
  local ts nonce sig; ts=$(date +%s); nonce="n-$RANDOM-$ts"
  sig=$(printf '%s\n%s\n%s' "$ts" "$nonce" "$2" | openssl dgst -sha256 -hmac "$DEVICE_SECRET" | awk '{print $NF}')
  curl -s -X POST "$API$1" -H "X-CTV-Device-Id: $DEVICE_ID" -H "X-CTV-Timestamp: $ts" \
    -H "X-CTV-Nonce: $nonce" -H "X-CTV-Signature: $sig" -H 'Content-Type: application/json' --data "$2"
}

ctv_device_post /v1/devices/heartbeat '{}'                                   # bring the device online
# after starting a verification (step 4) and getting CODE, deliver the "SMS":
ctv_device_post /v1/inbound '{"number":"+8801700000001","type":"sms","sender":"+8801712345678","body":"4729"}'
# → {"matched":true,…}; now step 5 shows "verified".
```

## Where to go next

- [`channels.md`](channels.md) — missed-call and DTMF variants, and binding modes.
- [`../coordinator/README.md`](../coordinator/README.md) — full endpoint and auth reference.
- [`self-hosting.md`](self-hosting.md) — production config (env vars, Postgres, Redis, webhooks).
