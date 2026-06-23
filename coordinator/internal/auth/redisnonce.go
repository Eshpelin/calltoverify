package auth

import (
	"context"
	"log/slog"
	"sync/atomic"
	"time"

	"github.com/redis/go-redis/v9"
)

// RedisNonceCache rejects replayed device nonces using Redis, so replay protection
// is shared across instances. It satisfies the same Seen contract as NonceCache.
type RedisNonceCache struct {
	client     *redis.Client
	ttl        time.Duration
	failClosed bool // on Redis error: reject (true) vs allow (false)
	logger     *slog.Logger
	degraded   atomic.Bool
}

// NewRedisNonceCache builds a shared replay cache. failClosed decides the Redis-
// outage behaviour: when true the request is rejected (replay protection preserved
// at the cost of availability), when false it is allowed (the HMAC signature and
// ±300s timestamp skew still guard it). Default deployments use false; security-
// critical ones can opt into fail-closed via CTV_REDIS_FAIL_CLOSED.
func NewRedisNonceCache(client *redis.Client, ttl time.Duration, failClosed bool, logger *slog.Logger) *RedisNonceCache {
	if logger == nil {
		logger = slog.Default()
	}
	return &RedisNonceCache{client: client, ttl: ttl, failClosed: failClosed, logger: logger}
}

// Seen records the nonce and reports whether it had already been used within the
// TTL. On a Redis outage it follows the failClosed policy and logs the transition
// so the degraded state is never silent.
func (c *RedisNonceCache) Seen(nonce string, _ time.Time) bool {
	ctx := context.Background()
	ok, err := c.client.SetNX(ctx, "ctv:nonce:"+nonce, "1", c.ttl).Result()
	if err != nil {
		if c.degraded.CompareAndSwap(false, true) {
			c.logger.Warn("redis nonce cache unreachable", "fail_closed", c.failClosed, "err", err)
		}
		// fail-closed -> report "seen" so the request is rejected as a replay;
		// fail-open -> report "fresh" so it proceeds.
		return c.failClosed
	}
	if c.degraded.CompareAndSwap(true, false) {
		c.logger.Info("redis nonce cache recovered")
	}
	return !ok
}
