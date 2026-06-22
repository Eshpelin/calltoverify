// Package ratelimit provides a simple in-process token-bucket limiter keyed by an
// arbitrary string (for example, a sender MSISDN). A multi-instance deployment
// should replace this with a Redis-backed limiter.
package ratelimit

import (
	"sync"
	"time"
)

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
