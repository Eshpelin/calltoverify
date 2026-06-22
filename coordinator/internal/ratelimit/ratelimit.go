// Package ratelimit provides a simple in-process token-bucket limiter keyed by an
// arbitrary string (for example, a sender MSISDN). A multi-instance deployment
// should replace this with a Redis-backed limiter.
package ratelimit

import (
	"sync"
	"time"
)

// gcInterval bounds how often Allow sweeps idle buckets. A bucket is "idle" once
// it has refilled to full burst: deleting it is behaviourally identical to never
// having created it, so the sweep cannot change limiting decisions.
const gcInterval = time.Minute

type bucket struct {
	tokens float64
	last   time.Time
}

// Limiter is a keyed token-bucket limiter, safe for concurrent use.
type Limiter struct {
	mu      sync.Mutex
	buckets map[string]*bucket
	rate    float64 // tokens per second
	burst   float64
	now     func() time.Time
	lastGC  time.Time
}

// New returns a limiter that allows up to burst requests, refilling at
// ratePerMinute tokens per minute.
func New(ratePerMinute, burst int) *Limiter {
	return &Limiter{
		buckets: make(map[string]*bucket),
		rate:    float64(ratePerMinute) / 60.0,
		burst:   float64(burst),
		now:     time.Now,
	}
}

// Allow reports whether the key has budget for one request and consumes a token if so.
func (l *Limiter) Allow(key string) bool {
	l.mu.Lock()
	defer l.mu.Unlock()

	now := l.now()
	l.gc(now)

	b, ok := l.buckets[key]
	if !ok {
		l.buckets[key] = &bucket{tokens: l.burst - 1, last: now}
		return true
	}

	b.tokens += now.Sub(b.last).Seconds() * l.rate
	if b.tokens > l.burst {
		b.tokens = l.burst
	}
	b.last = now

	if b.tokens >= 1 {
		b.tokens--
		return true
	}
	return false
}

// gc periodically evicts buckets that have refilled to full burst. Without it the
// map grows once per distinct key (for example, every attacker-chosen sender
// MSISDN) and never shrinks, an unbounded-memory DoS vector. Callers hold l.mu.
func (l *Limiter) gc(now time.Time) {
	if now.Sub(l.lastGC) < gcInterval {
		return
	}
	l.lastGC = now
	for k, b := range l.buckets {
		if b.tokens+now.Sub(b.last).Seconds()*l.rate >= l.burst {
			delete(l.buckets, k)
		}
	}
}

// size reports the number of live buckets. Used by tests to assert eviction.
func (l *Limiter) size() int {
	l.mu.Lock()
	defer l.mu.Unlock()
	return len(l.buckets)
}
