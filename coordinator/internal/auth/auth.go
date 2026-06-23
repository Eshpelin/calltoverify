// Package auth holds the Coordinator's credential primitives: API-key hashing for
// the developer API, HMAC request signing for the device API, and a replay-nonce
// cache.
package auth

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"sync"
	"time"
)

// HashAPIKey returns the hex SHA-256 of a bearer key. Keys are high-entropy, so a
// plain hash (not a slow KDF) is sufficient and constant-time on lookup via the
// unique index.
func HashAPIKey(key string) string {
	sum := sha256.Sum256([]byte(key))
	return hex.EncodeToString(sum[:])
}

// GenerateKey returns a new random secret of the form "<prefix>_<32 hex bytes>",
// its hash, and the non-secret display prefix (first 12 chars).
func GenerateKey(prefix string) (key, hash, display string, err error) {
	buf := make([]byte, 24)
	if _, err = rand.Read(buf); err != nil {
		return "", "", "", err
	}
	key = prefix + "_" + hex.EncodeToString(buf)
	hash = HashAPIKey(key)
	display = key[:min(12, len(key))]
	return key, hash, display, nil
}

// GenerateSecret returns a random hex secret for HMAC use (device or webhook).
func GenerateSecret() (string, error) {
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return hex.EncodeToString(buf), nil
}

// HMACHex computes the hex HMAC-SHA256 of body under secret.
func HMACHex(secret string, body []byte) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(body)
	return hex.EncodeToString(mac.Sum(nil))
}

// DeviceSignature is the canonical signature a receiver sends with each request:
// HMAC-SHA256(secret, ts "\n" nonce "\n" body).
func DeviceSignature(secret, ts, nonce string, body []byte) string {
	msg := make([]byte, 0, len(ts)+len(nonce)+len(body)+2)
	msg = append(msg, ts...)
	msg = append(msg, '\n')
	msg = append(msg, nonce...)
	msg = append(msg, '\n')
	msg = append(msg, body...)
	return HMACHex(secret, msg)
}

// ConstantTimeEqual compares two strings without leaking timing.
func ConstantTimeEqual(a, b string) bool {
	return subtle.ConstantTimeCompare([]byte(a), []byte(b)) == 1
}

// NonceCache remembers recently seen device nonces to reject replays. It is
// in-process; a multi-instance deployment should back this with Redis.
type NonceCache struct {
	mu     sync.Mutex
	seen   map[string]time.Time
	ttl    time.Duration
	lastGC time.Time
}

func NewNonceCache(ttl time.Duration) *NonceCache {
	return &NonceCache{seen: make(map[string]time.Time), ttl: ttl}
}

// Seen records the nonce and reports whether it had already been used within the TTL.
func (c *NonceCache) Seen(nonce string, now time.Time) bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	// GC at most once per TTL rather than scanning the whole map on every request
	// (which is O(n) under the lock on the hot path). Correctness is unchanged: the
	// replay check below still compares against the per-nonce timestamp.
	if now.Sub(c.lastGC) > c.ttl {
		for k, t := range c.seen {
			if now.Sub(t) > c.ttl {
				delete(c.seen, k)
			}
		}
		c.lastGC = now
	}
	if t, ok := c.seen[nonce]; ok && now.Sub(t) <= c.ttl {
		return true
	}
	c.seen[nonce] = now
	return false
}
