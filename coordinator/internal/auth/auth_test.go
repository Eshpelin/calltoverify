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
