# calltoverify (Python)

Backend SDK for Python. A dependency-free client (standard library only) for the CallToVerify
Coordinator's developer API.

> **Status: alpha (Phase 1 implemented).**

## Install

```bash
pip install calltoverify
```

Requires Python 3.9+.

## Usage

```python
from calltoverify import CallToVerify

ctv = CallToVerify(
    base_url="https://verify.example.com",   # your Coordinator
    api_key=os.environ["CTV_API_KEY"],
    webhook_secret=os.environ.get("CTV_WEBHOOK_SECRET"),  # only for verify_webhook
)

# Start a verification; show the instructions to the user.
v = ctv.start_verification(channel="sms")
print(v.instructions.action)   # "Send 123456 to +8801700000001"

# Poll, or rely on the webhook.
status = ctv.check_status(v.session_id)
if status.status == "verified":
    print("verified:", status.verified_msisdn)
```

### Claim mode

```python
v = ctv.start_verification(channel="sms", binding_mode="claim", claimed_msisdn="+8801712345678")
```

### Verifying webhooks

```python
# Pass the RAW request body and the X-CTV-Signature header.
try:
    event = ctv.verify_webhook(raw_body, signature)
    # event.session_id, event.verified_msisdn, event.channel, ...
except CallToVerifyError:
    ...  # signature mismatch -> reject
```

## API

- `CallToVerify(base_url, api_key, webhook_secret=None, *, timeout=10.0)`
- `start_verification(*, channel=None, binding_mode=None, claimed_msisdn=None) -> Verification`
- `check_status(session_id) -> VerificationStatus`
- `verify_webhook(raw_body, signature, max_age_seconds=None) -> WebhookEvent` (raises `CallToVerifyError` on signature mismatch; pass `max_age_seconds` to also reject events whose `ts` is older than the window)

Non-2xx responses raise `CallToVerifyError` with `.status` and `.code`.

## Develop

```bash
python -m unittest discover -s tests -t .
```
