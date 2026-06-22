package auth

import (
	"testing"
	"time"
)

func TestHashAPIKeyStable(t *testing.T) {
	if HashAPIKey("ctv_abc") != HashAPIKey("ctv_abc") {
		t.Fatal("hash not stable")
	}
	if HashAPIKey("a") == HashAPIKey("b") {
		t.Fatal("distinct keys hash equal")
	}
}

func TestGenerateKey(t *testing.T) {
	key, hash, prefix, err := GenerateKey("ctv")
	if err != nil {
		t.Fatal(err)
	}
	if HashAPIKey(key) != hash {
		t.Fatal("hash does not match key")
	}
	if prefix != key[:12] {
		t.Fatalf("prefix %q not derived from key", prefix)
	}
}

func TestDeviceSignatureRoundTrip(t *testing.T) {
	secret := "s3cr3t"
	body := []byte(`{"a":1}`)
	sig := DeviceSignature(secret, "1700000000", "nonce1", body)
	if !ConstantTimeEqual(sig, DeviceSignature(secret, "1700000000", "nonce1", body)) {
		t.Fatal("signature not reproducible")
	}
	if ConstantTimeEqual(sig, DeviceSignature("other", "1700000000", "nonce1", body)) {
		t.Fatal("signature valid under wrong secret")
	}
}

// Cross-language known-answer vectors. The device-signing and webhook-signing
// protocols MUST be byte-identical across every client (Go, Python/Pi, Node, PHP,
// Kotlin). These pinned hex digests are mirrored verbatim in the other languages'
// test suites (see receiver-pi, sdk-server-node, sdk-server-php). A round-trip
// test cannot catch cross-language drift because both sides move together; a fixed
// digest can. Do not change these without updating every mirrored test.
const (
	katDeviceSecret = "s3cr3t"
	katTS           = "1700000000"
	katNonce        = "nonce1"
	katDeviceBody   = `{"a":1}`
	katDeviceSig    = "93cffdba929d8f1c542790a0b59ca1fd239a0a2a1f909f18f25ee401e484fc24"

	katWebhookSecret = "whsec_test"
	// A fixed, compact JSON byte string (no inter-token spaces). The signature is
	// over these exact bytes; re-serializing this object in another language must
	// reproduce the same bytes for the digest to match.
	katWebhookBody = `{"event":"verification.verified","session_id":"sess1","verified_msisdn":"+8801712345678","channel":"sms","ts":"2026-01-01T00:00:00Z"}`
	katWebhookSig  = "e665a75f0e93afe2a7a77b832e826d2ec3654f3d519aec12f54b5ae558086694"
)

func TestDeviceSignatureKAT(t *testing.T) {
	got := DeviceSignature(katDeviceSecret, katTS, katNonce, []byte(katDeviceBody))
	if got != katDeviceSig {
		t.Fatalf("device-signing KAT drifted:\n got  %s\n want %s\n(cross-language protocol break)", got, katDeviceSig)
	}
}

func TestWebhookSignatureKAT(t *testing.T) {
	got := HMACHex(katWebhookSecret, []byte(katWebhookBody))
	if got != katWebhookSig {
		t.Fatalf("webhook-signing KAT drifted:\n got  %s\n want %s\n(cross-language protocol break)", got, katWebhookSig)
	}
}

func TestNonceCache(t *testing.T) {
	c := NewNonceCache(time.Minute)
	now := time.Unix(1700000000, 0)
	if c.Seen("n1", now) {
		t.Fatal("first use should not be seen")
	}
	if !c.Seen("n1", now) {
		t.Fatal("replay should be detected")
	}
	if c.Seen("n2", now) {
		t.Fatal("distinct nonce should be fresh")
	}
}
