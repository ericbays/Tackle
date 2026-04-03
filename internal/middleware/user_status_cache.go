package middleware

import (
	"context"
	"database/sql"
	"sync"
	"time"
)

const userStatusCacheTTL = 30 * time.Second

type statusEntry struct {
	status    string
	fetchedAt time.Time
}

// UserStatusCache provides a short-lived cache of user account statuses
// to avoid hitting the database on every authenticated request.
type UserStatusCache struct {
	db    *sql.DB
	mu    sync.RWMutex
	cache map[string]statusEntry
}

// NewUserStatusCache creates a UserStatusCache backed by the given database.
func NewUserStatusCache(db *sql.DB) *UserStatusCache {
	return &UserStatusCache{
		db:    db,
		cache: make(map[string]statusEntry),
	}
}

// IsLocked returns true if the user's account status is "locked" or "inactive".
func (c *UserStatusCache) IsLocked(ctx context.Context, userID string) bool {
	c.mu.RLock()
	entry, ok := c.cache[userID]
	c.mu.RUnlock()

	if ok && time.Since(entry.fetchedAt) < userStatusCacheTTL {
		return entry.status == "locked" || entry.status == "inactive"
	}

	// Cache miss or expired — query DB.
	var status string
	err := c.db.QueryRowContext(ctx, `SELECT status FROM users WHERE id = $1`, userID).Scan(&status)
	if err != nil {
		// If user not found, treat as locked (safe default).
		return true
	}

	c.mu.Lock()
	c.cache[userID] = statusEntry{status: status, fetchedAt: time.Now()}
	c.mu.Unlock()

	return status == "locked" || status == "inactive"
}

// Invalidate removes a user from the cache, forcing a fresh DB lookup next time.
func (c *UserStatusCache) Invalidate(userID string) {
	c.mu.Lock()
	delete(c.cache, userID)
	c.mu.Unlock()
}
