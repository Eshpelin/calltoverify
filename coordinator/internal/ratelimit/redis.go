package ratelimit

import (
	"context"
	"fmt"
	"log/slog"
	"sync/atomic"
	"time"

	"github.com/redis/go-redis/v9"
)

// RedisLimiter is a fixed-window per-key limiter backed by Redis, so multiple
// Coordinator/engine instances share one budget. It satisfies the same Allow(key)
// contract as the in-process Limiter.
type RedisLimiter struct {
	client   *redis.Client
	rate     int // requests allowed per 60s window
	logger   *slog.Logger
	degraded atomic.Bool // true while Redis is unreachable, to log the transition once
}

func NewRedis(client *redis.Client, ratePerMinute int, logger *slog.Logger) *RedisLimiter {
	if logger == nil {
		logger = slog.Default()
	}
	return &RedisLimiter{client: client, rate: ratePerMinute, logger: logger}
}

// Allow consumes one request for the key in the current minute window. It fails
// open (allows) if Redis is unreachable, so an outage never locks out real users
// — but it logs the open/recover transition so the degraded state is never silent.
func (l *RedisLimiter) Allow(key string) bool {
	ctx := context.Background()
	window := time.Now().Unix() / 60
	rk := fmt.Sprintf("ctv:rl:%s:%d", key, window)
	n, err := l.client.Incr(ctx, rk).Result()
	if err != nil {
		if l.degraded.CompareAndSwap(false, true) {
			l.logger.Warn("redis rate-limiter unreachable; failing open (rate limiting disabled until it recovers)", "err", err)
		}
		return true
	}
	if l.degraded.CompareAndSwap(true, false) {
		l.logger.Info("redis rate-limiter recovered")
	}
	if n == 1 {
		// Set the window TTL only on the first increment. If this fails the key
		// would never expire and the window would never reset, so log it.
		if err := l.client.Expire(ctx, rk, 70*time.Second).Err(); err != nil {
			l.logger.Warn("redis rate-limiter: failed to set window expiry", "key", rk, "err", err)
		}
	}
	return n <= int64(l.rate)
}
