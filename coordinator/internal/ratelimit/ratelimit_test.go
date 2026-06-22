package ratelimit

import (
	"testing"
	"time"
)

func TestTokenBucketBurstThenBlock(t *testing.T) {
	l := New(60, 3) // 1/sec, burst 3
	base := time.Unix(1700000000, 0)
	l.now = func() time.Time { return base }

	for i := 0; i < 3; i++ {
		if !l.Allow("k") {
			t.Fatalf("request %d within burst should be allowed", i)
		}
	}
	if l.Allow("k") {
		t.Fatal("4th request should be blocked")
	}

	// After ~1s one token refills.
	l.now = func() time.Time { return base.Add(time.Second) }
	if !l.Allow("k") {
		t.Fatal("token should refill after a second")
	}
}

func TestTokenBucketPerKey(t *testing.T) {
	l := New(60, 1)
	if !l.Allow("a") {
		t.Fatal("a first allowed")
	}
	if !l.Allow("b") {
		t.Fatal("b has its own bucket")
	}
}
