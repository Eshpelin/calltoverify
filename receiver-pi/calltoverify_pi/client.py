"""Signed HTTP client for the CallToVerify device API.

The signing scheme matches the Go Coordinator exactly:
    X-CTV-Signature = hex(HMAC_SHA256(device_secret, timestamp + "\\n" + nonce + "\\n" + body))
with X-CTV-Device-Id, X-CTV-Timestamp (unix seconds), and X-CTV-Nonce headers.
"""
from __future__ import annotations

import hashlib
import hmac
import json
import os
import time
import urllib.error
import urllib.request
from typing import Callable, Optional, Protocol, Tuple


class CtvError(Exception):
    def __init__(self, status: int, code: str, detail: str) -> None:
        super().__init__(f"{code}: {detail}")
        self.status = status
        self.code = code
        self.detail = detail


class Transport(Protocol):
    """Pluggable HTTP transport so the client is testable without a network."""

    def post(self, url: str, headers: dict, body: bytes) -> Tuple[int, bytes]:
        ...


class UrllibTransport:
    def __init__(self, timeout: float = 10.0) -> None:
        self._timeout = timeout

    def post(self, url: str, headers: dict, body: bytes) -> Tuple[int, bytes]:
        req = urllib.request.Request(url, data=body, headers=headers, method="POST")
        try:
            with urllib.request.urlopen(req, timeout=self._timeout) as resp:
                return resp.status, resp.read()
        except urllib.error.HTTPError as exc:
            return exc.code, exc.read()


def sign(device_secret: str, ts: str, nonce: str, body: bytes) -> str:
    msg = f"{ts}\n{nonce}\n".encode() + body
    return hmac.new(device_secret.encode(), msg, hashlib.sha256).hexdigest()


class CtvClient:
    def __init__(
        self,
        endpoint: str,
        device_id: str,
        device_secret: str,
        *,
        transport: Optional[Transport] = None,
        time_fn: Callable[[], int] = lambda: int(time.time()),
        nonce_fn: Callable[[], str] = lambda: os.urandom(12).hex(),
    ) -> None:
        self.endpoint = endpoint.rstrip("/")
        self.device_id = device_id
        self.device_secret = device_secret
        self._transport = transport or UrllibTransport()
        self._time_fn = time_fn
        self._nonce_fn = nonce_fn

    def register(self) -> dict:
        return self._post("/devices/register", {})

    def heartbeat(self) -> dict:
        return self._post("/devices/heartbeat", {})

    def inbound(self, number: str, kind: str, sender: str, body: str = "") -> dict:
        """Report an inbound signal. kind is 'sms' or 'call' (call with a non-empty
        body carries captured DTMF digits; empty body is a missed call)."""
        return self._post("/inbound", {"number": number, "type": kind, "sender": sender, "body": body})

    def _post(self, path: str, payload: dict) -> dict:
        body = json.dumps(payload).encode()
        ts = str(self._time_fn())
        nonce = self._nonce_fn()
        headers = {
            "Content-Type": "application/json",
            "X-CTV-Device-Id": self.device_id,
            "X-CTV-Timestamp": ts,
            "X-CTV-Nonce": nonce,
            "X-CTV-Signature": sign(self.device_secret, ts, nonce, body),
        }
        status, raw = self._transport.post(self.endpoint + path, headers, body)
        # A non-JSON body (e.g. an HTML error page from a proxy) must not mask the
        # real HTTP status with a JSONDecodeError.
        try:
            data = json.loads(raw.decode()) if raw else {}
        except (ValueError, UnicodeDecodeError):
            data = {}
        if not isinstance(data, dict):
            data = {}
        if status < 200 or status >= 300:
            raise CtvError(status, data.get("error", "error"), data.get("detail", "request failed"))
        return data
