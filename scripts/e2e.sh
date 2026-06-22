#!/usr/bin/env bash
#
# End-to-end smoke test for CallToVerify.
#
# Builds the embedded-engine example (Go), runs it, then drives a full verification
# through HTTP using the Python receiver client (receiver-pi). This exercises the
# real loop AND the cross-language device-signing interop (Python signer -> Go
# verifier). Covers: SMS happy path, missed-call (claim binding), and a negative
# claim-mismatch case.
#
# Requires: go, python3, curl. Run from the repo root:  ./scripts/e2e.sh
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
TMP="$(mktemp -d)"
export PYTHONPATH="$ROOT/receiver-pi"
API="http://localhost:8080"

echo "building embedded example..."
( cd "$ROOT/coordinator" && go build -o "$TMP/embedded" ./examples/embedded )

( cd "$TMP" && ./embedded >"$TMP/server.log" 2>&1 & echo $! > "$TMP/pid" )
SVPID="$(cat "$TMP/pid")"
cleanup() { kill "$SVPID" 2>/dev/null || true; rm -rf "$TMP"; }
trap cleanup EXIT

pair() {  # $1=msisdn(url-encoded) $2=channels
  curl --retry 40 --retry-connrefused --retry-delay 1 -s "$API/pair?name=e2e&msisdn=$1&channels=$2" \
    | python3 -c 'import sys,json;print(json.load(sys.stdin)["scan_this"])'
}
online() { python3 - "$1" <<'PY'
import sys, json
from calltoverify_pi.client import CtvClient
p = json.loads(sys.argv[1]); c = CtvClient(p["endpoint"], p["device_id"], p["device_secret"])
c.register(); c.heartbeat()
PY
}
inbound() {  # $1=payload $2=number $3=type $4=sender $5=body $6=want(true|false)
  python3 - "$@" <<'PY'
import sys, json
from calltoverify_pi.client import CtvClient
p = json.loads(sys.argv[1])
c = CtvClient(p["endpoint"], p["device_id"], p["device_secret"])
res = c.inbound(sys.argv[2], sys.argv[3], sys.argv[4], sys.argv[5])
want = sys.argv[6] == "true"
assert bool(res.get("matched")) is want, f"matched={res.get('matched')} want={want}: {res}"
print("   inbound ->", res)
PY
}
field() { python3 -c "import sys,json;print(json.load(sys.stdin)['$1'])"; }
assert_status() {  # $1=session $2=expected-status [$3=expected-msisdn]
  local d; d="$(curl -s "$API/status?id=$1")"
  echo "$d" | python3 -c "import sys,json; d=json.load(sys.stdin); assert d['Status']=='$2', d; ${3:+assert d['VerifiedMSISDN']=='$3', d;} print('   status ->', d['Status'], d.get('VerifiedMSISDN',''))"
}

echo "== SMS =="
P="$(pair "%2B8801700000001" sms)"; online "$P"
S="$(curl -s -X POST "$API/start?channel=sms")"
SID="$(echo "$S" | field SessionID)"; CODE="$(echo "$S" | python3 -c 'import sys,json;print(json.load(sys.stdin)["Instructions"]["Code"])')"
inbound "$P" "+8801700000001" sms "+8801712345678" "$CODE" true
assert_status "$SID" verified "+8801712345678"

echo "== missed call (claim) =="
P2="$(pair "%2B8801700000002" call)"; online "$P2"
S2="$(curl -s -X POST "$API/start?channel=call&binding_mode=claim&claimed_msisdn=%2B8801722222222")"
SID2="$(echo "$S2" | field SessionID)"
inbound "$P2" "+8801700000002" call "+8801722222222" "" true
assert_status "$SID2" verified "+8801722222222"

echo "== negative: claim mismatch stays pending =="
S3="$(curl -s -X POST "$API/start?channel=call&binding_mode=claim&claimed_msisdn=%2B8801799999999")"
SID3="$(echo "$S3" | field SessionID)"
inbound "$P2" "+8801700000002" call "+8801700000000" "" false
assert_status "$SID3" pending

echo ""
echo "PASS: CallToVerify end-to-end (SMS + missed-call + negative) verified."
