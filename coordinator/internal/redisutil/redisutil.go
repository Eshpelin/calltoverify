// Package redisutil connects to Redis from a URL, verifying reachability.
package redisutil

import (
	"context"
	"time"

	"github.com/redis/go-redis/v9"
)

// Connect parses a redis:// URL, dials, and pings. It returns an error if Redis is
// unreachable so callers can fall back to in-process state.
func Connect(ctx context.Context, url string) (*redis.Client, error) {
	opt, err := redis.ParseURL(url)
	if err != nil {
		return nil, err
	}
	client := redis.NewClient(opt)
	pingCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	if err := client.Ping(pingCtx).Err(); err != nil {
		_ = client.Close()
		return nil, err
	}
	return client, nil
}
