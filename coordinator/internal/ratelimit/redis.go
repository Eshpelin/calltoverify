package ratelimit

import (
	"context"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

// RedisLimiter is a fixed-window per-key limiter backed by Redis, so multiple
// Coordinator/engine instances share one budget. It satisfies the same Allow(key)
// contract as the in-process Limiter.
type RedisLimiter struct {
	client *redis.Client
	rate   int // requests allowed per 60s window
}

func NewRedis(client *redis.Client, ratePerMinute int) *RedisLimiter {
	return &RedisLimiter{client: client, rate: ratePerMinute}
}

// Allow consumes one request for the key in the current minute window. It fails
// open (allows) if Redis is unreachable, so an outage never locks out real users.
func (l *RedisLimiter) Allow(key string) bool {
	ctx := context.Background()
	window := time.Now().Unix() / 60
	rk := fmt.Sprintf("ctv:rl:%s:%d", key, window)
	n, err := l.client.Incr(ctx, rk).Result()
	if err != nil {
		return true
	}
	if n == 1 {
		l.client.Expire(ctx, rk, 70*time.Second)
	}
	return n <= int64(l.rate)
}
