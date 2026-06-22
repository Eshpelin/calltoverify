# sdk-server-python

Backend SDK for Python.

> **Status: planned (Phase 4).** Placeholder directory.

## Planned API

```python
ctv = CallToVerify(base_url=..., api_key=..., api_secret=...)

v = ctv.start_verification(channel="sms", claimed_msisdn=msisdn)
# v.instructions -> number, code, deep_link, expires_at

status = ctv.check_status(v.session_id)

ctv.verify_webhook(raw_body, signature_header)  # raises on mismatch
```

## Planned stack

Python 3.10+, published to PyPI. Sync and async clients.
