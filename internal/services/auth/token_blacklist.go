package auth

import (
	"sync"
	"time"
)

// TokenBlacklist is an in-memory store of revoked JWT JTI values.
// Entries expire automatically once the corresponding token would have naturally expired.
type TokenBlacklist struct {
	mu      sync.RWMutex
	entries map[string]time.Time // jti -> expiry
}

// NewTokenBlacklist creates an empty TokenBlacklist.
func NewTokenBlacklist() *TokenBlacklist {
	return &TokenBlacklist{
		entries: make(map[string]time.Time),
	}
}

// Revoke adds jti to the blacklist. It is automatically removed after expiry.
func (b *TokenBlacklist) Revoke(jti string, expiry time.Time) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.entries[jti] = expiry
}

// IsRevoked reports whether jti has been revoked and the revocation has not yet expired.
// Expired entries are cleaned up lazily during this call.
func (b *TokenBlacklist) IsRevoked(jti string) bool {
	b.mu.Lock()
	defer b.mu.Unlock()

	expiry, exists := b.entries[jti]
	if !exists {
		return false
	}
	if time.Now().After(expiry) {
		delete(b.entries, jti)
		return false
	}
	return true
}
