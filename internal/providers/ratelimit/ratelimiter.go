// Package ratelimit provides a simple token-bucket rate limiter for per-provider API calls.
package ratelimit

import (
	"sync"
	"time"
)

// RateLimiter is a simple token-bucket rate limiter for per-provider API calls.
// It is safe for concurrent use.
type RateLimiter struct {
	mu           sync.Mutex
	tokens       int
	maxTokens    int
	refillPeriod time.Duration
	lastRefill   time.Time
}

// NewRateLimiter creates a RateLimiter that allows at most ratePerMinute calls per minute.
func NewRateLimiter(ratePerMinute int) *RateLimiter {
	if ratePerMinute <= 0 {
		ratePerMinute = 10
	}
	return &RateLimiter{
		tokens:       ratePerMinute,
		maxTokens:    ratePerMinute,
		refillPeriod: time.Minute,
		lastRefill:   time.Now(),
	}
}

// Wait blocks until a token is available, then consumes one token.
func (r *RateLimiter) Wait() {
	for {
		r.mu.Lock()
		r.refill()
		if r.tokens > 0 {
			r.tokens--
			r.mu.Unlock()
			return
		}
		r.mu.Unlock()
		time.Sleep(50 * time.Millisecond)
	}
}

// refill adds tokens based on time elapsed since last refill. Must be called under lock.
func (r *RateLimiter) refill() {
	now := time.Now()
	elapsed := now.Sub(r.lastRefill)
	if elapsed >= r.refillPeriod {
		r.tokens = r.maxTokens
		r.lastRefill = now
	}
}
