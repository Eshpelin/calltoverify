package ratelimit

import (
	"strconv"
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

// TestIdleBucketsEvicted proves the bucket map does not grow unboundedly: once a
// bucket has refilled to full burst and the GC interval has elapsed, it is removed.
// Without this, every distinct (attacker-chosen) key leaks a map entry forever.
func TestIdleBucketsEvicted(t *testing.T) {
	l := New(60, 3) // 1 token/sec, burst 3
	base := time.Unix(1700000000, 0)
	l.now = func() time.Time { return base }

	for i := 0; i < 1000; i++ {
		l.Allow(strconv.Itoa(i)) // each key consumes 1 of its 3 tokens, leaving 2
	}
	if got := l.size(); got != 1000 {
		t.Fatalf("expected 1000 live buckets, got %d", got)
	}

	// Jump past the GC interval and well past the refill time (2 tokens at 1/sec
	// refill in 2s; the GC sweeps on the next Allow). All idle buckets are full again.
	l.now = func() time.Time { return base.Add(gcInterval + 5*time.Second) }
	l.Allow("trigger") // triggers gc; "trigger" itself stays (just used a token)
	if got := l.size(); got != 1 {
		t.Fatalf("expected stale buckets evicted down to 1, got %d", got)
	}
}

// TestGCKeepsActiveBuckets proves eviction does not reset a bucket that is still
// being limited. We use a slow refill so that after the GC interval the bucket has
// NOT refilled to full burst; it must survive the sweep and keep blocking.
func TestGCKeepsActiveBuckets(t *testing.T) {
	l := New(6, 2) // 0.1 token/sec, burst 2: a token takes 10s, full refill 20s
	base := time.Unix(1700000000, 0)
	l.now = func() time.Time { return base }

	if !l.Allow("k") || !l.Allow("k") {
		t.Fatal("first two within burst should pass")
	}
	if l.Allow("k") {
		t.Fatal("third should be blocked")
	}

	// Advance exactly one GC interval (60s). At 0.1/sec that refills 6 tokens, but
	// the bucket caps at burst=2, so it IS full and gets evicted -- and would also
	// legitimately allow. To test survival of a still-limited bucket, use a key
	// touched right before the sweep so it has not refilled.
	l.now = func() time.Time { return base.Add(gcInterval - time.Second) }
	if !l.Allow("fresh") || !l.Allow("fresh") {
		t.Fatal("fresh burst should pass")
	}
	if l.Allow("fresh") {
		t.Fatal("fresh third should be blocked")
	}
	// Trigger the GC: only ~1s has passed for "fresh", so it is still depleted and
	// must NOT be evicted -- the next request stays blocked.
	l.now = func() time.Time { return base.Add(gcInterval) }
	l.Allow("trigger") // fires gc
	if l.Allow("fresh") {
		t.Fatal("still-limited bucket must survive GC and keep blocking")
	}
}
