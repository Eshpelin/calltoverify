package ratelimit

import (
	"context"
	"os"
	"strconv"
	"testing"
	"time"

	"github.com/Eshpelin/calltoverify/coordinator/internal/redisutil"
)

func TestRedisLimiter(t *testing.T) {
	url := os.Getenv("CTV_TEST_REDIS_URL")
	if url == "" {
		t.Skip("set CTV_TEST_REDIS_URL to run the Redis limiter test")
	}
	rc, err := redisutil.Connect(context.Background(), url)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	defer rc.Close()

	l := NewRedis(rc, 3) // 3 per minute
	key := "test:" + strconv.FormatInt(time.Now().UnixNano(), 10)

	for i := 0; i < 3; i++ {
		if !l.Allow(key) {
			t.Fatalf("request %d within budget should pass", i)
		}
	}
	if l.Allow(key) {
		t.Fatal("4th request should be limited")
	}
}
