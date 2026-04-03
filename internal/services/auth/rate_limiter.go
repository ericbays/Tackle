package auth

import (
	"sync"
	"time"
)

type failRecord struct {
	count       int
	windowStart time.Time
	lockedUntil time.Time
}

// LockoutCallback is called when a rate limit lockout is triggered.
// key is the rate limit key (e.g., "ip:1.2.3.4" or "user:admin").
type LockoutCallback func(key string, attempts int, lockedUntil time.Time)

// RateLimiter is an in-memory per-key rate limiter with configurable thresholds and lock durations.
// Keys are caller-prefixed strings: "ip:<addr>" or "user:<username>".
type RateLimiter struct {
	mu              sync.RWMutex
	records         map[string]*failRecord
	maxPerIP        int
	ipLockDur       time.Duration
	maxPerAccount   int
	accountLockDur  time.Duration
	window          time.Duration
	onLockout       LockoutCallback
}

// NewRateLimiter creates a RateLimiter with the default thresholds from REQ-AUTH-025:
//   - per-IP: 10 failures/minute → 5-minute lock
//   - per-account: 5 failures/minute → 15-minute lock
func NewRateLimiter() *RateLimiter {
	return &RateLimiter{
		records:        make(map[string]*failRecord),
		maxPerIP:       10,
		ipLockDur:      5 * time.Minute,
		maxPerAccount:  5,
		accountLockDur: 15 * time.Minute,
		window:         time.Minute,
	}
}

// SetLockoutCallback sets a function that is called (outside the lock) when a
// lockout is triggered. Useful for audit logging rate limit events.
func (r *RateLimiter) SetLockoutCallback(cb LockoutCallback) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.onLockout = cb
}

// RecordFailure records a failed attempt for key and returns the updated attempt count,
// the lock expiry time (if now locked), and whether the key is currently locked.
// Keys prefixed with "ip:" use IP thresholds; all others use account thresholds.
func (r *RateLimiter) RecordFailure(key string) (attempts int, lockedUntil time.Time, locked bool) {
	r.mu.Lock()

	now := time.Now()
	rec := r.records[key]
	if rec == nil {
		rec = &failRecord{}
		r.records[key] = rec
	}

	// Reset window if expired.
	if now.Sub(rec.windowStart) > r.window {
		rec.count = 0
		rec.windowStart = now
		rec.lockedUntil = time.Time{}
	}

	rec.count++
	max, lockDur := r.thresholds(key)
	justLocked := false
	if rec.count >= max && rec.lockedUntil.IsZero() {
		rec.lockedUntil = now.Add(lockDur)
		justLocked = true
	}

	attempts = rec.count
	lockedUntil = rec.lockedUntil
	locked = !rec.lockedUntil.IsZero() && now.Before(rec.lockedUntil)
	cb := r.onLockout

	r.mu.Unlock()

	// Fire callback outside the lock.
	if justLocked && cb != nil {
		cb(key, attempts, lockedUntil)
	}

	return attempts, lockedUntil, locked
}

// Reset clears the failure record for key (called on successful authentication).
func (r *RateLimiter) Reset(key string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.records, key)
}

// IsLocked reports whether key is currently rate-limited, and if so, when the lock expires.
func (r *RateLimiter) IsLocked(key string) (bool, time.Time) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	rec := r.records[key]
	if rec == nil {
		return false, time.Time{}
	}
	now := time.Now()
	if rec.lockedUntil.IsZero() || now.After(rec.lockedUntil) {
		return false, time.Time{}
	}
	return true, rec.lockedUntil
}

func (r *RateLimiter) thresholds(key string) (max int, lockDur time.Duration) {
	if len(key) >= 3 && key[:3] == "ip:" {
		return r.maxPerIP, r.ipLockDur
	}
	return r.maxPerAccount, r.accountLockDur
}
