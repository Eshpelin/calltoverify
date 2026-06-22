import hashlib
import hmac
import json
import unittest

from calltoverify_pi.client import CtvClient, CtvError, sign


class FakeTransport:
    def __init__(self, status=200, body=b'{"matched":true}'):
        self.status = status
        self.body = body
        self.calls = []

    def post(self, url, headers, body):
        self.calls.append((url, headers, body))
        return self.status, self.body


def client(transport):
    return CtvClient(
        "https://x/ctv",
        "dev1",
        "secret",
        transport=transport,
        time_fn=lambda: 1700000000,
        nonce_fn=lambda: "nonce1",
    )


class ClientTest(unittest.TestCase):
    def test_inbound_signs_and_builds_payload(self):
        t = FakeTransport()
        res = client(t).inbound("+880170", "sms", "+880171", "123456")
        self.assertTrue(res["matched"])

        url, headers, body = t.calls[0]
        self.assertEqual(url, "https://x/ctv/inbound")
        self.assertEqual(headers["X-CTV-Device-Id"], "dev1")
        self.assertEqual(headers["X-CTV-Timestamp"], "1700000000")
        self.assertEqual(headers["X-CTV-Nonce"], "nonce1")
        self.assertEqual(headers["X-CTV-Signature"], sign("secret", "1700000000", "nonce1", body))
        self.assertEqual(
            json.loads(body),
            {"number": "+880170", "type": "sms", "sender": "+880171", "body": "123456"},
        )

    def test_sign_matches_ts_nonce_body_layout(self):
        body = b'{"a":1}'
        expected = hmac.new(b"secret", b"1700000000\nnonce1\n" + body, hashlib.sha256).hexdigest()
        self.assertEqual(sign("secret", "1700000000", "nonce1", body), expected)

    def test_device_signature_known_answer_vector(self):
        # Cross-language known-answer vector. This pinned digest is mirrored in the
        # Go (coordinator/internal/auth/auth_test.go), Node, and PHP suites. A
        # round-trip test cannot catch cross-language drift; a fixed digest can.
        # Do not change without updating every mirrored test.
        got = sign("s3cr3t", "1700000000", "nonce1", b'{"a":1}')
        self.assertEqual(got, "93cffdba929d8f1c542790a0b59ca1fd239a0a2a1f909f18f25ee401e484fc24")

    def test_non_2xx_raises(self):
        t = FakeTransport(status=401, body=b'{"error":"unauthorized","detail":"bad signature"}')
        with self.assertRaises(CtvError) as ctx:
            client(t).heartbeat()
        self.assertEqual(ctx.exception.status, 401)
        self.assertEqual(ctx.exception.code, "unauthorized")

    def test_endpoint_trailing_slash_trimmed(self):
        t = FakeTransport()
        c = CtvClient("https://x/ctv/", "d", "s", transport=t, time_fn=lambda: 1, nonce_fn=lambda: "n")
        c.heartbeat()
        self.assertEqual(t.calls[0][0], "https://x/ctv/devices/heartbeat")


if __name__ == "__main__":
    unittest.main()
