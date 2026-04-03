package auth

import (
	"context"
	"database/sql"
	"encoding/json"
	"sync"
	"time"
)

// SessionConfig holds configurable session parameters loaded from system_settings.
type SessionConfig struct {
	AccessTokenLifetime  time.Duration
	RefreshTokenLifetime time.Duration
	MaxConcurrentSessions int
	IdleTimeout          time.Duration
}

// DefaultSessionConfig returns sensible defaults.
func DefaultSessionConfig() SessionConfig {
	return SessionConfig{
		AccessTokenLifetime:  15 * time.Minute,
		RefreshTokenLifetime: 7 * 24 * time.Hour,
		MaxConcurrentSessions: 0, // unlimited
		IdleTimeout:          0,  // disabled
	}
}

// SessionConfigLoader caches session config with a configurable TTL to avoid
// hitting the database on every token issuance.
type SessionConfigLoader struct {
	db        *sql.DB
	mu        sync.RWMutex
	config    SessionConfig
	loadedAt  time.Time
	cacheTTL  time.Duration
}

// NewSessionConfigLoader creates a loader that caches for the given TTL.
func NewSessionConfigLoader(db *sql.DB, cacheTTL time.Duration) *SessionConfigLoader {
	return &SessionConfigLoader{
		db:       db,
		config:   DefaultSessionConfig(),
		cacheTTL: cacheTTL,
	}
}

// Load returns the current session config, refreshing from DB if the cache has expired.
func (l *SessionConfigLoader) Load(ctx context.Context) SessionConfig {
	l.mu.RLock()
	if time.Since(l.loadedAt) < l.cacheTTL {
		cfg := l.config
		l.mu.RUnlock()
		return cfg
	}
	l.mu.RUnlock()

	// Cache expired — reload.
	l.mu.Lock()
	defer l.mu.Unlock()

	// Double-check after acquiring write lock.
	if time.Since(l.loadedAt) < l.cacheTTL {
		return l.config
	}

	cfg := DefaultSessionConfig()
	rows, err := l.db.QueryContext(ctx, `SELECT key, value FROM system_settings WHERE key IN (
		'jwt_access_token_lifetime_minutes',
		'jwt_refresh_token_lifetime_days',
		'max_concurrent_sessions',
		'idle_timeout_minutes'
	)`)
	if err != nil {
		// On error, keep using previous/default config.
		return l.config
	}
	defer rows.Close()

	for rows.Next() {
		var key, val string
		if err := rows.Scan(&key, &val); err != nil {
			continue
		}
		switch key {
		case "jwt_access_token_lifetime_minutes":
			var v int
			if json.Unmarshal([]byte(val), &v) == nil && v > 0 {
				cfg.AccessTokenLifetime = time.Duration(v) * time.Minute
			}
		case "jwt_refresh_token_lifetime_days":
			var v int
			if json.Unmarshal([]byte(val), &v) == nil && v > 0 {
				cfg.RefreshTokenLifetime = time.Duration(v) * 24 * time.Hour
			}
		case "max_concurrent_sessions":
			var v int
			if json.Unmarshal([]byte(val), &v) == nil && v >= 0 {
				cfg.MaxConcurrentSessions = v
			}
		case "idle_timeout_minutes":
			var v int
			if json.Unmarshal([]byte(val), &v) == nil && v >= 0 {
				cfg.IdleTimeout = time.Duration(v) * time.Minute
			}
		}
	}

	l.config = cfg
	l.loadedAt = time.Now()
	return cfg
}

// Refresh forces a cache refresh on next Load call.
func (l *SessionConfigLoader) Refresh() {
	l.mu.Lock()
	l.loadedAt = time.Time{}
	l.mu.Unlock()
}
