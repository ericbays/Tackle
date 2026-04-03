package apikey

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"fmt"
	"time"

	"github.com/google/uuid"
)

// Service provides API key lifecycle management.
type Service struct {
	repo *Repository
	db   *sql.DB
}

// NewService creates a new API key service.
func NewService(db *sql.DB) *Service {
	return &Service{
		repo: NewRepository(db),
		db:   db,
	}
}

// CreateResult holds the result of creating an API key.
// The RawKey is only available at creation time.
type CreateResult struct {
	RawKey string
	Key    APIKey
}

// Create generates a new API key for the given user.
// Returns the raw key (shown once) and the stored record.
func (s *Service) Create(ctx context.Context, userID, name string, expiresAt *time.Time) (*CreateResult, error) {
	raw, err := generateRawKey()
	if err != nil {
		return nil, fmt.Errorf("create api key: %w", err)
	}

	hash := hashKey(raw)
	prefix := raw[:8]
	id := uuid.New().String()

	key := APIKey{
		ID:        id,
		UserID:    userID,
		Name:      name,
		KeyPrefix: prefix,
		KeyHash:   hash,
		ExpiresAt: expiresAt,
	}

	if err := s.repo.Create(ctx, key); err != nil {
		return nil, err
	}

	return &CreateResult{RawKey: raw, Key: key}, nil
}

// List returns all API keys for a user (hashes excluded from response).
func (s *Service) List(ctx context.Context, userID string) ([]APIKey, error) {
	return s.repo.ListByUser(ctx, userID)
}

// Get returns a single API key by ID.
func (s *Service) Get(ctx context.Context, keyID string) (*APIKey, error) {
	return s.repo.Get(ctx, keyID)
}

// Revoke marks an API key as revoked.
func (s *Service) Revoke(ctx context.Context, keyID string) error {
	return s.repo.Revoke(ctx, keyID)
}

// ValidateResult holds the user info associated with a validated API key.
type ValidateResult struct {
	UserID      string
	Username    string
	Email       string
	Role        string
	Permissions []string
}

// Validate checks a raw API key, returns the associated user info.
func (s *Service) Validate(ctx context.Context, rawKey string) (*ValidateResult, error) {
	hash := hashKey(rawKey)
	key, err := s.repo.FindByHash(ctx, hash)
	if err != nil {
		return nil, fmt.Errorf("validate api key: %w", err)
	}
	if key.Revoked {
		return nil, fmt.Errorf("api key revoked")
	}
	if key.ExpiresAt != nil && time.Now().After(*key.ExpiresAt) {
		return nil, fmt.Errorf("api key expired")
	}

	// Update last_used_at (fire-and-forget).
	go func() { _ = s.repo.UpdateLastUsed(context.Background(), key.ID) }() //nolint:errcheck

	// Load user with role and permissions.
	result, err := s.loadUserInfo(ctx, key.UserID)
	if err != nil {
		return nil, fmt.Errorf("validate api key: load user: %w", err)
	}
	return result, nil
}

func (s *Service) loadUserInfo(ctx context.Context, userID string) (*ValidateResult, error) {
	const q = `
		SELECT u.id, u.username, u.email, COALESCE(r.name, '') AS role_name
		FROM users u
		LEFT JOIN user_roles ur ON ur.user_id = u.id
		LEFT JOIN roles r ON r.id = ur.role_id
		WHERE u.id = $1 AND u.status = 'active'`

	var result ValidateResult
	err := s.db.QueryRowContext(ctx, q, userID).Scan(
		&result.UserID, &result.Username, &result.Email, &result.Role,
	)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("user not found or inactive")
	}
	if err != nil {
		return nil, fmt.Errorf("load user info: %w", err)
	}

	// Resolve permissions.
	result.Permissions, err = s.resolvePermissions(ctx, result.Role)
	if err != nil {
		return nil, fmt.Errorf("load user info: %w", err)
	}
	return &result, nil
}

func (s *Service) resolvePermissions(ctx context.Context, roleName string) ([]string, error) {
	if roleName == "" {
		return nil, nil
	}
	if roleName == "admin" {
		return s.allPermissions(ctx)
	}
	const q = `
		SELECT p.resource_type || ':' || p.action
		FROM role_permissions rp
		JOIN roles r ON r.id = rp.role_id
		JOIN permissions p ON p.id = rp.permission_id
		WHERE r.name = $1`
	rows, err := s.db.QueryContext(ctx, q, roleName)
	if err != nil {
		return nil, fmt.Errorf("resolve permissions: %w", err)
	}
	defer rows.Close()
	var perms []string
	for rows.Next() {
		var p string
		if err := rows.Scan(&p); err != nil {
			return nil, fmt.Errorf("resolve permissions: scan: %w", err)
		}
		perms = append(perms, p)
	}
	return perms, rows.Err()
}

func (s *Service) allPermissions(ctx context.Context) ([]string, error) {
	const q = `SELECT resource_type || ':' || action FROM permissions ORDER BY resource_type, action`
	rows, err := s.db.QueryContext(ctx, q)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var perms []string
	for rows.Next() {
		var p string
		if err := rows.Scan(&p); err != nil {
			return nil, err
		}
		perms = append(perms, p)
	}
	return perms, rows.Err()
}

// generateRawKey produces 32 cryptographically random bytes, hex-encoded.
func generateRawKey() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("generate raw key: %w", err)
	}
	return hex.EncodeToString(b), nil
}

// hashKey returns the hex-encoded SHA-256 hash of a raw key.
func hashKey(rawKey string) string {
	sum := sha256.Sum256([]byte(rawKey))
	return hex.EncodeToString(sum[:])
}
