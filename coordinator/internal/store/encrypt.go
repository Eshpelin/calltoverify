package store

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"strings"
)

// secretPrefix marks a value encrypted by this layer. Values without it are
// treated as legacy plaintext (so enabling encryption on an existing database is
// safe: old secrets keep working and are re-encrypted on their next write).
const secretPrefix = "ctvenc1:"

// DecodeSecretKey parses a 32-byte key from hex (64 chars) or base64.
func DecodeSecretKey(s string) ([]byte, error) {
	s = strings.TrimSpace(s)
	if len(s) == 64 {
		if b, err := hex.DecodeString(s); err == nil {
			return b, nil
		}
	}
	b, err := base64.StdEncoding.DecodeString(s)
	if err != nil {
		b, err = base64.RawStdEncoding.DecodeString(s)
	}
	if err != nil {
		return nil, fmt.Errorf("CTV_SECRET_KEY must be 32 bytes as hex or base64")
	}
	if len(b) != 32 {
		return nil, fmt.Errorf("CTV_SECRET_KEY must decode to 32 bytes, got %d", len(b))
	}
	return b, nil
}

type secretCipher struct{ aead cipher.AEAD }

func newSecretCipher(key []byte) (*secretCipher, error) {
	if len(key) != 32 {
		return nil, fmt.Errorf("secret key must be 32 bytes, got %d", len(key))
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	return &secretCipher{aead: aead}, nil
}

func (c *secretCipher) encrypt(plain string) (string, error) {
	nonce := make([]byte, c.aead.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return "", err
	}
	ct := c.aead.Seal(nonce, nonce, []byte(plain), nil)
	return secretPrefix + base64.RawStdEncoding.EncodeToString(ct), nil
}

func (c *secretCipher) decrypt(v string) (string, error) {
	if !strings.HasPrefix(v, secretPrefix) {
		return v, nil // legacy plaintext / never-encrypted -> passthrough
	}
	raw, err := base64.RawStdEncoding.DecodeString(strings.TrimPrefix(v, secretPrefix))
	if err != nil {
		return "", err
	}
	ns := c.aead.NonceSize()
	if len(raw) < ns {
		return "", fmt.Errorf("ciphertext too short")
	}
	pt, err := c.aead.Open(nil, raw[:ns], raw[ns:], nil)
	if err != nil {
		return "", err
	}
	return string(pt), nil
}

// encStore wraps a Store and transparently encrypts the two symmetric secrets
// (device_secret, webhook_secret) at rest with AES-256-GCM, so a database read
// alone does not yield forge-capable HMAC keys. It embeds the inner Store, so all
// other methods pass through unchanged and the wrapper stays correct as the
// interface grows.
type encStore struct {
	Store
	cipher *secretCipher
}

// NewEncrypted wraps inner so device/webhook secrets are encrypted at rest.
func NewEncrypted(inner Store, key []byte) (Store, error) {
	c, err := newSecretCipher(key)
	if err != nil {
		return nil, err
	}
	return &encStore{Store: inner, cipher: c}, nil
}

func (e *encStore) CreateApp(ctx context.Context, a App) (App, error) {
	plain := a.WebhookSecret
	enc, err := e.cipher.encrypt(plain)
	if err != nil {
		return App{}, err
	}
	a.WebhookSecret = enc
	out, err := e.Store.CreateApp(ctx, a)
	out.WebhookSecret = plain // hand the caller the cleartext it just supplied
	return out, err
}

func (e *encStore) EnsureApp(ctx context.Context, a App) (App, error) {
	enc, err := e.cipher.encrypt(a.WebhookSecret)
	if err != nil {
		return App{}, err
	}
	a.WebhookSecret = enc
	out, err := e.Store.EnsureApp(ctx, a)
	if err != nil {
		return out, err
	}
	out.WebhookSecret, err = e.cipher.decrypt(out.WebhookSecret)
	return out, err
}

func (e *encStore) GetAppByID(ctx context.Context, id string) (App, error) {
	return e.decApp(e.Store.GetAppByID(ctx, id))
}

func (e *encStore) GetAppByAPIKeyHash(ctx context.Context, hash string) (App, error) {
	return e.decApp(e.Store.GetAppByAPIKeyHash(ctx, hash))
}

func (e *encStore) decApp(a App, err error) (App, error) {
	if err != nil {
		return a, err
	}
	a.WebhookSecret, err = e.cipher.decrypt(a.WebhookSecret)
	return a, err
}

func (e *encStore) CreateDevice(ctx context.Context, d Device) (Device, error) {
	plain := d.DeviceSecret
	enc, err := e.cipher.encrypt(plain)
	if err != nil {
		return Device{}, err
	}
	d.DeviceSecret = enc
	out, err := e.Store.CreateDevice(ctx, d)
	out.DeviceSecret = plain
	return out, err
}

func (e *encStore) GetDeviceByID(ctx context.Context, id string) (Device, error) {
	d, err := e.Store.GetDeviceByID(ctx, id)
	if err != nil {
		return d, err
	}
	d.DeviceSecret, err = e.cipher.decrypt(d.DeviceSecret)
	return d, err
}
