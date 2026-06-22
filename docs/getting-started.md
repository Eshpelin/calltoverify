# Getting started

This walks through a full SMS verification end to end against a local Coordinator. In
production the inbound step is performed by a receiver (the Android app or the Pi daemon); here
we simulate it with `curl` + `openssl` so you can see the whole loop.

## Prerequisites

- Docker (for the quick start), or Go 1.25+ to run the Coordinator directly.
- `curl`, `jq`, and `openssl` for the demo below.

## 1. Run the stack

```bash
git clone https://github.com/Eshpelin/calltoverify.git
cd calltoverify
docker compose up --build
```

Postgres, Redis, and the Coordinator come up. The Coordinator applies its schema migrations on
startup. Check it is alive:

```bash
curl -s http://localhost:8080/healthz   # {"status":"ok"}
```

The compose file sets `CTV_ADMIN_TOKEN=dev-admin-change-me` to enable provisioning.

## 2. Provision an app, a device, and a number

```bash
ADMIN="dev-admin-change-me"
API=http://localhost:8080

APP=$(curl -s -X POST $API/admin/apps -H "Authorization: Bearer $ADMIN" \
  -d '{"name":"demo"}')
API_KEY=$(echo "$APP" | jq -r .api_key)
APP_ID=$(echo "$APP" | jq -r .app_id)

DEV=$(curl -s -X POST $API/admin/devices -H "Authorization: Bearer $ADMIN" \
  -d "{\"app_id\":\"$APP_ID\",\"name\":\"my-phone\",\"type\":\"android\",\"capabilities\":[\"sms\"]}")
DEVICE_ID=$(echo "$DEV" | jq -r .device_id)
DEVICE_SECRET=$(echo "$DEV" | jq -r .device_secret)

curl -s -X POST $API/admin/numbers -H "Authorization: Bearer $ADMIN" \
  -d "{\"device_id\":\"$DEVICE_ID\",\"msisdn\":\"+8801700000001\",\"channels\":[\"sms\"]}" >/dev/null
```

## 3. A helper to send signed device requests

```bash
ctv_device_post() {  # $1=path  $2=json body
  local path="$1" body="$2"
  local ts nonce sig
  ts=$(date +%s); nonce=$(uuidgen 2>/dev/null || echo "n-$RANDOM-$ts")
  sig=$(printf '%s\n%s\n%s' "$ts" "$nonce" "$body" \
        | openssl dgst -sha256 -hmac "$DEVICE_SECRET" | awk '{print $NF}')
  curl -s -X POST "$API$path" \
    -H "X-CTV-Device-Id: $DEVICE_ID" -H "X-CTV-Timestamp: $ts" \
    -H "X-CTV-Nonce: $nonce" -H "X-CTV-Signature: $sig" \
    -H "Content-Type: application/json" --data "$body"
}

# Bring the device online.
ctv_device_post /v1/devices/heartbeat '{}'
```

## 4. Start a verification

```bash
START=$(curl -s -X POST $API/v1/verifications -H "Authorization: Bearer $API_KEY" \
  -d '{"channel":"sms"}')
echo "$START" | jq
SESSION_ID=$(echo "$START" | jq -r .session_id)
CODE=$(echo "$START" | jq -r .instructions.code)
echo "User is told: send $CODE to +8801700000001"
```

## 5. Simulate the inbound SMS, then check status

```bash
ctv_device_post /v1/inbound \
  "{\"number\":\"+8801700000001\",\"type\":\"sms\",\"sender\":\"+8801712345678\",\"body\":\"$CODE\"}"
# {"matched":true,"session_id":"..."}

curl -s $API/v1/verifications/$SESSION_ID -H "Authorization: Bearer $API_KEY" | jq
# {"status":"verified","channel":"sms","verified_msisdn":"+8801712345678", ...}
```

That is the entire loop. Swap step 5 for a real receiver and the user's own phone, and you have
free phone verification. See [`channels.md`](channels.md) for missed-call and DTMF variants, and
[`../coordinator/README.md`](../coordinator/README.md) for the full API and auth reference.
