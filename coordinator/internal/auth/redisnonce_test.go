package auth

import (
	"context"
	"os"
	"strconv"
	"testing"
	"time"

	"github.com/Eshpelin/calltoverify/coordinator/internal/redisutil"
)

func TestRedisNonceCache(t *testing.T) {
	url := os.Getenv("CTV_TEST_REDIS_URL")
	if url == "" {
		t.Skip("set CTV_TEST_REDIS_URL to run the Redis nonce test")
	}
	rc, err := redisutil.Connect(context.Background(), url)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	defer rc.Close()

	c := NewRedisNonceCache(rc, time.Minute, false, nil)
	now := time.Now()
	nonce := "nonce:" + strconv.FormatInt(now.UnixNano(), 10)

	if c.Seen(nonce, now) {
		t.Fatal("first use should not be seen")
	}
	if !c.Seen(nonce, now) {
		t.Fatal("replay should be detected")
	}
	if c.Seen(nonce+"-other", now) {
		t.Fatal("a distinct nonce should be fresh")
	}
}
