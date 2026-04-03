// Package apikey provides API key management services.
package apikey

import (
	"context"
	"database/sql"
	"fmt"
	"time"
)

// APIKey represents a stored API key record (hash only, never the raw key).
type APIKey struct {
	ID         string
	UserID     string
	Name       string
	KeyPrefix  string
	KeyHash    string
	ExpiresAt  *time.Time
	LastUsedAt *time.Time
	Revoked    bool
	CreatedAt  time.Time
	UpdatedAt  time.Time
}

// Repository provides CRUD operations against the api_keys table.
type Repository struct {
	db *sql.DB
}

// NewRepository creates a new API key repository.
func NewRepository(db *sql.DB) *Repository {
	return &Repository{db: db}
}

// Create stores a new API key record.
func (r *Repository) Create(ctx context.Context, key APIKey) error {
	const q = `
		INSERT INTO api_keys (id, user_id, name, key_prefix, key_hash, expires_at)
		VALUES ($1, $2, $3, $4, $5, $6)`
	_, err := r.db.ExecContext(ctx, q, key.ID, key.UserID, key.Name, key.KeyPrefix, key.KeyHash, key.ExpiresAt)
	if err != nil {
		return fmt.Errorf("create api key: %w", err)
	}
	return nil
}

// FindByHash looks up an API key by its SHA-256 hash.
func (r *Repository) FindByHash(ctx context.Context, hash string) (*APIKey, error) {
	const q = `
		SELECT id, user_id, name, key_prefix, key_hash, expires_at, last_used_at, revoked, created_at, updated_at
		FROM api_keys WHERE key_hash = $1`
	var k APIKey
	var expiresAt, lastUsedAt sql.NullTime
	err := r.db.QueryRowContext(ctx, q, hash).Scan(
		&k.ID, &k.UserID, &k.Name, &k.KeyPrefix, &k.KeyHash,
		&expiresAt, &lastUsedAt, &k.Revoked, &k.CreatedAt, &k.UpdatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("api key not found")
	}
	if err != nil {
		return nil, fmt.Errorf("find api key by hash: %w", err)
	}
	if expiresAt.Valid {
		k.ExpiresAt = &expiresAt.Time
	}
	if lastUsedAt.Valid {
		k.LastUsedAt = &lastUsedAt.Time
	}
	return &k, nil
}

// ListByUser returns all API keys for a user.
func (r *Repository) ListByUser(ctx context.Context, userID string) ([]APIKey, error) {
	const q = `
		SELECT id, user_id, name, key_prefix, key_hash, expires_at, last_used_at, revoked, created_at, updated_at
		FROM api_keys WHERE user_id = $1 ORDER BY created_at DESC`
	rows, err := r.db.QueryContext(ctx, q, userID)
	if err != nil {
		return nil, fmt.Errorf("list api keys: %w", err)
	}
	defer rows.Close()

	var keys []APIKey
	for rows.Next() {
		var k APIKey
		var expiresAt, lastUsedAt sql.NullTime
		if err := rows.Scan(
			&k.ID, &k.UserID, &k.Name, &k.KeyPrefix, &k.KeyHash,
			&expiresAt, &lastUsedAt, &k.Revoked, &k.CreatedAt, &k.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("list api keys: scan: %w", err)
		}
		if expiresAt.Valid {
			k.ExpiresAt = &expiresAt.Time
		}
		if lastUsedAt.Valid {
			k.LastUsedAt = &lastUsedAt.Time
		}
		keys = append(keys, k)
	}
	return keys, rows.Err()
}

// Get returns a single API key by ID.
func (r *Repository) Get(ctx context.Context, keyID string) (*APIKey, error) {
	const q = `
		SELECT id, user_id, name, key_prefix, key_hash, expires_at, last_used_at, revoked, created_at, updated_at
		FROM api_keys WHERE id = $1`
	var k APIKey
	var expiresAt, lastUsedAt sql.NullTime
	err := r.db.QueryRowContext(ctx, q, keyID).Scan(
		&k.ID, &k.UserID, &k.Name, &k.KeyPrefix, &k.KeyHash,
		&expiresAt, &lastUsedAt, &k.Revoked, &k.CreatedAt, &k.UpdatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("api key not found")
	}
	if err != nil {
		return nil, fmt.Errorf("get api key: %w", err)
	}
	if expiresAt.Valid {
		k.ExpiresAt = &expiresAt.Time
	}
	if lastUsedAt.Valid {
		k.LastUsedAt = &lastUsedAt.Time
	}
	return &k, nil
}

// Revoke marks an API key as revoked.
func (r *Repository) Revoke(ctx context.Context, keyID string) error {
	_, err := r.db.ExecContext(ctx, `UPDATE api_keys SET revoked = TRUE WHERE id = $1`, keyID)
	if err != nil {
		return fmt.Errorf("revoke api key: %w", err)
	}
	return nil
}

// UpdateLastUsed updates the last_used_at timestamp.
func (r *Repository) UpdateLastUsed(ctx context.Context, keyID string) error {
	_, err := r.db.ExecContext(ctx, `UPDATE api_keys SET last_used_at = now() WHERE id = $1`, keyID)
	if err != nil {
		return fmt.Errorf("update last used: %w", err)
	}
	return nil
}
