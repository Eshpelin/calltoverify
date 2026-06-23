package store

import (
	"bytes"
	"context"
	"encoding/hex"
	"strings"
	"testing"
)

func testKey() []byte { return bytes.Repeat([]byte{0x2a}, 32) }

func TestSecretCipherRoundTrip(t *testing.T) {
	c, err := newSecretCipher(testKey())
	if err != nil {
		t.Fatalf("newSecretCipher: %v", err)
	}
	enc, err := c.encrypt("s3cr3t")
	if err != nil {
		t.Fatalf("encrypt: %v", err)
	}
	if !strings.HasPrefix(enc, secretPrefix) {
		t.Fatalf("ciphertext missing marker prefix: %q", enc)
	}
	if enc == "s3cr3t" || strings.Contains(enc, "s3cr3t") {
		t.Fatalf("plaintext leaked into ciphertext: %q", enc)
	}
	got, err := c.decrypt(enc)
	if err != nil || got != "s3cr3t" {
		t.Fatalf("decrypt = %q err %v, want s3cr3t", got, err)
	}
	// Legacy plaintext (no prefix) passes through untouched.
	if got, _ := c.decrypt("legacy-plain"); got != "legacy-plain" {
		t.Fatalf("legacy passthrough = %q", got)
	}
	// Nonce is random: two encryptions of the same value differ.
	enc2, _ := c.encrypt("s3cr3t")
	if enc == enc2 {
		t.Fatalf("ciphertext not randomized")
	}
}

func TestDecodeSecretKey(t *testing.T) {
	hexKey := hex.EncodeToString(testKey())
	if b, err := DecodeSecretKey(hexKey); err != nil || len(b) != 32 {
		t.Fatalf("hex key: %v len %d", err, len(b))
	}
	b64 := "MDEyMzQ1Njc4OWFiY2RlZjAxMjM0NTY3ODlhYmNkZWY=" // 32 bytes base64
	if b, err := DecodeSecretKey(b64); err != nil || len(b) != 32 {
		t.Fatalf("base64 key: %v len %d", err, len(b))
	}
	if _, err := DecodeSecretKey("too-short"); err == nil {
		t.Fatalf("short key should error")
	}
	if _, err := DecodeSecretKey(hex.EncodeToString([]byte("16-byte-key-here"))); err == nil {
		t.Fatalf("wrong-length key should error")
	}
}

// TestEncryptedStoreAtRest proves the wrapper returns plaintext to callers while
// the underlying row holds ciphertext for both secret columns.
func TestEncryptedStoreAtRest(t *testing.T) {
	ctx := context.Background()
	inner, err := NewSQLite(":memory:")
	if err != nil {
		t.Fatalf("sqlite: %v", err)
	}
	t.Cleanup(inner.Close)
	if err := inner.Migrate(ctx); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	enc, err := NewEncrypted(inner, testKey())
	if err != nil {
		t.Fatalf("NewEncrypted: %v", err)
	}

	app, err := enc.CreateApp(ctx, App{Name: "a", APIKeyHash: "h", APIKeyPrefix: "p", WebhookSecret: "whsec_plain"})
	if err != nil {
		t.Fatalf("CreateApp: %v", err)
	}
	if app.WebhookSecret != "whsec_plain" {
		t.Fatalf("CreateApp returned %q, want plaintext", app.WebhookSecret)
	}
	// Caller-facing reads decrypt.
	got, _ := enc.GetAppByID(ctx, app.ID)
	if got.WebhookSecret != "whsec_plain" {
		t.Fatalf("GetAppByID via wrapper = %q, want plaintext", got.WebhookSecret)
	}
	gotByHash, _ := enc.GetAppByAPIKeyHash(ctx, "h")
	if gotByHash.WebhookSecret != "whsec_plain" {
		t.Fatalf("GetAppByAPIKeyHash via wrapper = %q", gotByHash.WebhookSecret)
	}
	// The row itself is ciphertext.
	raw, _ := inner.GetAppByID(ctx, app.ID)
	if !strings.HasPrefix(raw.WebhookSecret, secretPrefix) {
		t.Fatalf("webhook_secret stored in cleartext: %q", raw.WebhookSecret)
	}

	dev, err := enc.CreateDevice(ctx, Device{AppID: app.ID, Name: "d", DeviceSecret: "dev_plain", Type: "pi", Capabilities: []string{"sms"}})
	if err != nil {
		t.Fatalf("CreateDevice: %v", err)
	}
	if dev.DeviceSecret != "dev_plain" {
		t.Fatalf("CreateDevice returned %q", dev.DeviceSecret)
	}
	gotDev, _ := enc.GetDeviceByID(ctx, dev.ID)
	if gotDev.DeviceSecret != "dev_plain" {
		t.Fatalf("GetDeviceByID via wrapper = %q", gotDev.DeviceSecret)
	}
	rawDev, _ := inner.GetDeviceByID(ctx, dev.ID)
	if !strings.HasPrefix(rawDev.DeviceSecret, secretPrefix) {
		t.Fatalf("device_secret stored in cleartext: %q", rawDev.DeviceSecret)
	}
}

// TestEncryptedStoreReadsLegacyPlaintext: a row written before encryption was
// enabled (plaintext) is still readable through the wrapper.
func TestEncryptedStoreReadsLegacyPlaintext(t *testing.T) {
	ctx := context.Background()
	inner, err := NewSQLite(":memory:")
	if err != nil {
		t.Fatalf("sqlite: %v", err)
	}
	t.Cleanup(inner.Close)
	if err := inner.Migrate(ctx); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	// Write directly via the inner (plaintext) store.
	app, _ := inner.CreateApp(ctx, App{Name: "a", APIKeyHash: "h", APIKeyPrefix: "p", WebhookSecret: "legacy_plain"})
	dev, _ := inner.CreateDevice(ctx, Device{AppID: app.ID, Name: "d", DeviceSecret: "legacy_dev", Type: "pi", Capabilities: []string{"sms"}})

	enc, _ := NewEncrypted(inner, testKey())
	if got, _ := enc.GetAppByID(ctx, app.ID); got.WebhookSecret != "legacy_plain" {
		t.Fatalf("legacy webhook secret = %q", got.WebhookSecret)
	}
	if got, _ := enc.GetDeviceByID(ctx, dev.ID); got.DeviceSecret != "legacy_dev" {
		t.Fatalf("legacy device secret = %q", got.DeviceSecret)
	}
}
