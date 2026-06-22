package auth

import (
	"context"
	"time"

	"github.com/redis/go-redis/v9"
)

// RedisNonceCache rejects replayed device nonces using Redis, so replay protection
// is shared across instances. It satisfies the same Seen contract as NonceCache.
type RedisNonceCache struct {
	client *redis.Client
	ttl    time.Duration
}

func NewRedisNonceCache(client *redis.Client, ttl time.Duration) *RedisNonceCache {
	return &RedisNonceCache{client: client, ttl: ttl}
}

// Seen records the nonce and reports whether it had already been used within the
// TTL. It fails open (treats the nonce as fresh) if Redis is unreachable; the HMAC
// signature and timestamp skew still guard the request.
func (c *RedisNonceCache) Seen(nonce string, _ time.Time) bool {
	ctx := context.Background()
	ok, err := c.client.SetNX(ctx, "ctv:nonce:"+nonce, "1", c.ttl).Result()
	if err != nil {
		return false
	}
	return !ok
}
