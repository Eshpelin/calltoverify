// Package config loads Coordinator settings from the environment. All keys are
// prefixed CTV_ and have development-friendly defaults.
package config

import (
	"os"
	"strconv"
	"time"
)

type Config struct {
	ListenAddr     string        // CTV_LISTEN_ADDR
	DatabaseURL    string        // CTV_DATABASE_URL
	RedisURL       string        // CTV_REDIS_URL
	Env            string        // CTV_ENV: development | production
	AdminToken     string        // CTV_ADMIN_TOKEN: bearer token for /admin endpoints (disabled if empty)
	DefaultCodeLen int           // CTV_DEFAULT_CODE_LEN
	DefaultTTL     time.Duration // CTV_DEFAULT_TTL_SECONDS
	// WebhookAllowPrivate lets webhooks reach loopback/private addresses. Off by
	// default (SSRF defense); enable only for single-tenant self-hosts whose
	// webhook genuinely lives on a private/internal address. CTV_WEBHOOK_ALLOW_PRIVATE
	WebhookAllowPrivate bool
	// RedisFailClosed makes the shared nonce cache reject requests when Redis is
	// unreachable (preserves replay protection at the cost of availability) instead
	// of failing open. Off by default. CTV_REDIS_FAIL_CLOSED
	RedisFailClosed bool
	// MaxInFlight caps concurrent in-flight HTTP requests; excess get 503 so a
	// flood of slow connections can't exhaust goroutines/DB connections. CTV_MAX_INFLIGHT
	MaxInFlight int
	// InboundRetention is how long inbound_events audit rows are kept before the
	// sweep prunes them. CTV_INBOUND_RETENTION_DAYS
	InboundRetention time.Duration
	// SecretKey, when set, enables AES-256-GCM encryption of device_secret and
	// webhook_secret at rest. 32 bytes as hex or base64. Empty = plaintext (legacy,
	// still readable after the key is enabled). CTV_SECRET_KEY
	SecretKey string
	// MaxPendingPerNumber caps pending sessions per number (flood/exhaustion guard).
	// CTV_MAX_PENDING_PER_NUMBER
	MaxPendingPerNumber int
}

// Load reads configuration from the environment, applying defaults.
func Load() Config {
	return Config{
		ListenAddr:     getenv("CTV_LISTEN_ADDR", ":8080"),
		DatabaseURL:    getenv("CTV_DATABASE_URL", "postgres://calltoverify:calltoverify@localhost:5432/calltoverify?sslmode=disable"),
		RedisURL:       getenv("CTV_REDIS_URL", "redis://localhost:6379"),
		Env:            getenv("CTV_ENV", "development"),
		AdminToken:     getenv("CTV_ADMIN_TOKEN", ""),
		DefaultCodeLen: getenvInt("CTV_DEFAULT_CODE_LEN", 6),
		DefaultTTL:     time.Duration(getenvInt("CTV_DEFAULT_TTL_SECONDS", 90)) * time.Second,

		WebhookAllowPrivate: getenvBool("CTV_WEBHOOK_ALLOW_PRIVATE", false),
		RedisFailClosed:     getenvBool("CTV_REDIS_FAIL_CLOSED", false),
		MaxInFlight:         getenvInt("CTV_MAX_INFLIGHT", 512),
		InboundRetention:    time.Duration(getenvInt("CTV_INBOUND_RETENTION_DAYS", 30)) * 24 * time.Hour,
		SecretKey:           getenv("CTV_SECRET_KEY", ""),
		MaxPendingPerNumber: getenvInt("CTV_MAX_PENDING_PER_NUMBER", 100),
	}
}

func getenvBool(key string, def bool) bool {
	if v := os.Getenv(key); v != "" {
		if b, err := strconv.ParseBool(v); err == nil {
			return b
		}
	}
	return def
}

func getenv(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func getenvInt(key string, def int) int {
	if v := os.Getenv(key); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			return n
		}
	}
	return def
}
