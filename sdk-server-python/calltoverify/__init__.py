"""CallToVerify server SDK.

A dependency-free client for the Coordinator's developer API. Use it from your
backend to start verifications, poll their status, and verify webhooks.
"""
from __future__ import annotations

import hashlib
import hmac
import json
import urllib.error
import urllib.parse
import urllib.request
from dataclasses import dataclass
from typing import Optional, Union

__all__ = [
    "CallToVerify",
    "CallToVerifyError",
    "Instructions",
    "Verification",
    "VerificationStatus",
    "WebhookEvent",
]

__version__ = "0.1.0"


@dataclass
class Instructions:
    number: str
    channel: str
    action: str
    deep_link: str
    expires_at: str
    code: Optional[str] = None


@dataclass
class Verification:
    session_id: str
    status: str
    instructions: Instructions


@dataclass
class VerificationStatus:
    session_id: str
    status: str
    channel: str
    expires_at: str
    verified_msisdn: Optional[str] = None


@dataclass
class WebhookEvent:
    event: str
    session_id: str
    verified_msisdn: str
    channel: str
    ts: str


class CallToVerifyError(Exception):
    """Raised for non-2xx Coordinator responses and webhook signature failures."""

    def __init__(self, status: int, code: str, detail: str) -> None:
        super().__init__(f"{code}: {detail}")
        self.status = status
        self.code = code
        self.detail = detail


class CallToVerify:
    def __init__(
        self,
        base_url: str,
        api_key: str,
        webhook_secret: Optional[str] = None,
        *,
        timeout: float = 10.0,
    ) -> None:
        if not base_url:
            raise ValueError("base_url is required")
        if not api_key:
            raise ValueError("api_key is required")
        self._base = base_url.rstrip("/")
        self._key = api_key
        self._webhook_secret = webhook_secret
        self._timeout = timeout

    def start_verification(
        self,
        *,
        channel: Optional[str] = None,
        binding_mode: Optional[str] = None,
        claimed_msisdn: Optional[str] = None,
    ) -> Verification:
        """Start a verification and return the user-facing instructions."""
        data = self._request(
            "POST",
            "/v1/verifications",
            {"channel": channel, "binding_mode": binding_mode, "claimed_msisdn": claimed_msisdn},
        )
        return Verification(
            session_id=data["session_id"],
            status=data["status"],
            instructions=_instructions(data["instructions"]),
        )

    def check_status(self, session_id: str) -> VerificationStatus:
        """Poll a verification's current status."""
        data = self._request("GET", f"/v1/verifications/{urllib.parse.quote(session_id)}")
        return VerificationStatus(
            session_id=data["session_id"],
            status=data["status"],
            channel=data["channel"],
            expires_at=data["expires_at"],
            verified_msisdn=data.get("verified_msisdn") or None,
        )

    def verify_webhook(self, raw_body: Union[str, bytes], signature: str) -> WebhookEvent:
        """Verify and parse a webhook. Pass the RAW request body and the
        X-CTV-Signature header. Raises CallToVerifyError on signature mismatch."""
        if not self._webhook_secret:
            raise ValueError("webhook_secret is required to verify webhooks")
        raw = raw_body.encode() if isinstance(raw_body, str) else raw_body
        expected = hmac.new(self._webhook_secret.encode(), raw, hashlib.sha256).hexdigest()
        if not hmac.compare_digest(expected, signature):
            raise CallToVerifyError(401, "invalid_signature", "webhook signature mismatch")
        p = json.loads(raw.decode())
        return WebhookEvent(
            event=p["event"],
            session_id=p["session_id"],
            verified_msisdn=p.get("verified_msisdn", ""),
            channel=p["channel"],
            ts=p["ts"],
        )

    def _request(self, method: str, path: str, body: Optional[dict] = None) -> dict:
        data = None
        headers = {"Authorization": f"Bearer {self._key}"}
        if body is not None:
            payload = {k: v for k, v in body.items() if v is not None}
            data = json.dumps(payload).encode()
            headers["Content-Type"] = "application/json"
        req = urllib.request.Request(self._base + path, data=data, method=method, headers=headers)
        try:
            with urllib.request.urlopen(req, timeout=self._timeout) as resp:
                text = resp.read().decode()
                return json.loads(text) if text else {}
        except urllib.error.HTTPError as exc:
            text = exc.read().decode()
            try:
                j = json.loads(text)
            except ValueError:
                j = {}
            raise CallToVerifyError(exc.code, j.get("error", "error"), j.get("detail", str(exc))) from None


def _instructions(i: dict) -> Instructions:
    return Instructions(
        number=i["number"],
        channel=i["channel"],
        action=i["action"],
        deep_link=i["deep_link"],
        expires_at=i["expires_at"],
        code=i.get("code") or None,
    )
