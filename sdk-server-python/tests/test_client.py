import hashlib
import hmac
import json
import threading
import unittest
from http.server import BaseHTTPRequestHandler, HTTPServer

from calltoverify import CallToVerify, CallToVerifyError


class _Handler(BaseHTTPRequestHandler):
    # Configured per-test via class attributes.
    responses: dict = {}
    captured: dict = {}

    def _send(self, code, obj):
        body = json.dumps(obj).encode()
        self.send_response(code)
        self.send_header("Content-Type", "application/json")
        self.send_header("Content-Length", str(len(body)))
        self.end_headers()
        self.wfile.write(body)

    def do_POST(self):
        length = int(self.headers.get("Content-Length", "0"))
        raw = self.rfile.read(length).decode()
        _Handler.captured = {"method": "POST", "path": self.path,
                             "auth": self.headers.get("Authorization"), "body": raw}
        code, obj = _Handler.responses.get(("POST", self.path), (404, {"error": "not_found"}))
        self._send(code, obj)

    def do_GET(self):
        _Handler.captured = {"method": "GET", "path": self.path,
                             "auth": self.headers.get("Authorization")}
        code, obj = _Handler.responses.get(("GET", self.path), (404, {"error": "not_found"}))
        self._send(code, obj)

    def log_message(self, *args):
        pass


class ClientTest(unittest.TestCase):
    def setUp(self):
        _Handler.responses = {}
        _Handler.captured = {}
        self.server = HTTPServer(("127.0.0.1", 0), _Handler)
        self.thread = threading.Thread(target=self.server.serve_forever, daemon=True)
        self.thread.start()
        self.base = f"http://127.0.0.1:{self.server.server_address[1]}"

    def tearDown(self):
        self.server.shutdown()
        self.server.server_close()

    def test_start_verification_maps_and_authenticates(self):
        _Handler.responses[("POST", "/v1/verifications")] = (201, {
            "session_id": "sess1",
            "status": "pending",
            "instructions": {
                "number": "+8801700000001",
                "code": "123456",
                "channel": "sms",
                "action": "Send 123456 to +8801700000001",
                "deep_link": "sms:+8801700000001?body=123456",
                "expires_at": "2026-01-01T00:00:00Z",
            },
        })
        ctv = CallToVerify(self.base, "k")
        v = ctv.start_verification(channel="sms", binding_mode="derive")

        self.assertEqual(_Handler.captured["auth"], "Bearer k")
        self.assertEqual(json.loads(_Handler.captured["body"])["channel"], "sms")
        self.assertEqual(v.session_id, "sess1")
        self.assertEqual(v.instructions.code, "123456")
        self.assertEqual(v.instructions.deep_link, "sms:+8801700000001?body=123456")

    def test_start_omits_none_fields(self):
        _Handler.responses[("POST", "/v1/verifications")] = (201, {
            "session_id": "s", "status": "pending",
            "instructions": {"number": "n", "channel": "sms", "action": "a",
                             "deep_link": "d", "expires_at": "e"},
        })
        CallToVerify(self.base, "k").start_verification(channel="sms")
        body = json.loads(_Handler.captured["body"])
        self.assertNotIn("binding_mode", body)
        self.assertNotIn("claimed_msisdn", body)

    def test_check_status(self):
        _Handler.responses[("GET", "/v1/verifications/sess1")] = (200, {
            "session_id": "sess1", "status": "verified", "channel": "sms",
            "verified_msisdn": "+8801712345678", "expires_at": "2026-01-01T00:00:00Z",
        })
        s = CallToVerify(self.base, "k").check_status("sess1")
        self.assertEqual(s.status, "verified")
        self.assertEqual(s.verified_msisdn, "+8801712345678")

    def test_http_error_maps_to_exception(self):
        _Handler.responses[("POST", "/v1/verifications")] = (401, {
            "error": "unauthorized", "detail": "invalid API key"})
        with self.assertRaises(CallToVerifyError) as ctx:
            CallToVerify(self.base, "bad").start_verification()
        self.assertEqual(ctx.exception.status, 401)
        self.assertEqual(ctx.exception.code, "unauthorized")

    def test_verify_webhook(self):
        secret = "whsec_test"
        body = json.dumps({
            "event": "verification.verified", "session_id": "sess1",
            "verified_msisdn": "+8801712345678", "channel": "sms", "ts": "2026-01-01T00:00:00Z",
        })
        sig = hmac.new(secret.encode(), body.encode(), hashlib.sha256).hexdigest()
        ctv = CallToVerify("http://unused", "k", webhook_secret=secret)

        ev = ctv.verify_webhook(body, sig)
        self.assertEqual(ev.session_id, "sess1")
        self.assertEqual(ev.verified_msisdn, "+8801712345678")

        with self.assertRaises(CallToVerifyError):
            ctv.verify_webhook(body, "deadbeef")

    def test_verify_webhook_known_answer_vector(self):
        # Cross-language known-answer vector for webhook signing (body-only HMAC).
        # This pinned digest is mirrored in the Go, Node, and PHP suites. The body
        # is a fixed compact JSON byte string -- the signature is over these exact
        # bytes, so it is asserted as a literal (json.dumps would add spaces and
        # change the bytes). Do not change without updating every mirrored test.
        secret = "whsec_test"
        body = (
            '{"event":"verification.verified","session_id":"sess1",'
            '"verified_msisdn":"+8801712345678","channel":"sms","ts":"2026-01-01T00:00:00Z"}'
        )
        expected = "e665a75f0e93afe2a7a77b832e826d2ec3654f3d519aec12f54b5ae558086694"
        self.assertEqual(
            hmac.new(secret.encode(), body.encode(), hashlib.sha256).hexdigest(), expected
        )
        ctv = CallToVerify("http://unused", "k", webhook_secret=secret)
        ev = ctv.verify_webhook(body, expected)
        self.assertEqual(ev.session_id, "sess1")

    def test_constructor_validation(self):
        with self.assertRaises(ValueError):
            CallToVerify("", "k")
        with self.assertRaises(ValueError):
            CallToVerify("http://x", "")


if __name__ == "__main__":
    unittest.main()
