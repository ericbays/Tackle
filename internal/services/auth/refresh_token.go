package auth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"errors"
	"fmt"
	"net"
	"time"
)

// ErrTokenNotFound is returned when a refresh token hash is not found in the database.
var ErrTokenNotFound = errors.New("refresh token not found")

// ErrTokenReuse is returned when a previously invalidated refresh token is presented,
// indicating a potential token theft. All tokens for the user are revoked.
var ErrTokenReuse = errors.New("refresh token already used: potential token theft detected")

// Session represents an active user session stored in the database.
type Session struct {
	ID               string
	UserID           string
	TokenHash        string
	RefreshTokenHash string
	IPAddress        string
	UserAgent        string
	ExpiresAt        time.Time
	LastUsedAt       *time.Time
	Revoked          bool
	CreatedAt        time.Time
}

// RefreshTokenService manages opaque refresh tokens with rotation and theft detection.
type RefreshTokenService struct {
	db *sql.DB
}

// NewRefreshTokenService creates a RefreshTokenService backed by the given database.
func NewRefreshTokenService(db *sql.DB) *RefreshTokenService {
	return &RefreshTokenService{db: db}
}

// Issue generates a cryptographically random refresh token, stores its SHA-256 hash
// in the sessions table, and returns the raw (unhashed) token to the caller.
// The raw token is never stored.
func (r *RefreshTokenService) Issue(ctx context.Context, userID, accessTokenHash string, ipAddress, userAgent string, lifetime time.Duration) (rawToken string, err error) {
	raw, err := generateToken()
	if err != nil {
		return "", fmt.Errorf("refresh token issue: %w", err)
	}
	hash := hashToken(raw)

	// Strip port from ipAddress if present (r.RemoteAddr includes host:port).
	if host, _, splitErr := net.SplitHostPort(ipAddress); splitErr == nil {
		ipAddress = host
	}

	const q = `
		INSERT INTO sessions (user_id, token_hash, refresh_token_hash, ip_address, user_agent, expires_at)
		VALUES ($1, $2, $3, $4, $5, $6)`
	if _, err := r.db.ExecContext(ctx, q, userID, accessTokenHash, hash, ipAddress, userAgent, time.Now().Add(lifetime)); err != nil {
		return "", fmt.Errorf("refresh token issue: insert: %w", err)
	}
	return raw, nil
}

// Consume validates the refresh token and marks the session as revoked.
// On success it returns the Session so the caller can issue a new token pair.
// If the token was already revoked, ErrTokenReuse is returned and all sessions for
// the user are immediately revoked (theft detection).
func (r *RefreshTokenService) Consume(ctx context.Context, rawToken string) (*Session, error) {
	hash := hashToken(rawToken)

	const q = `
		SELECT id, user_id, token_hash, refresh_token_hash, COALESCE(ip_address::text,''),
		       COALESCE(user_agent,''), expires_at, last_used_at, revoked, created_at
		FROM sessions
		WHERE refresh_token_hash = $1`

	var s Session
	var lastUsed sql.NullTime
	err := r.db.QueryRowContext(ctx, q, hash).Scan(
		&s.ID, &s.UserID, &s.TokenHash, &s.RefreshTokenHash,
		&s.IPAddress, &s.UserAgent, &s.ExpiresAt, &lastUsed,
		&s.Revoked, &s.CreatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, ErrTokenNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("refresh token consume: query: %w", err)
	}
	if lastUsed.Valid {
		s.LastUsedAt = &lastUsed.Time
	}

	if s.Revoked {
		// Potential theft: invalidate all sessions for this user.
		_ = r.RevokeAll(ctx, s.UserID)
		return nil, ErrTokenReuse
	}
	if time.Now().After(s.ExpiresAt) {
		return nil, ErrTokenNotFound
	}

	// Mark this session as revoked (single-use rotation).
	if _, err := r.db.ExecContext(ctx, `UPDATE sessions SET revoked = TRUE, last_used_at = now() WHERE id = $1`, s.ID); err != nil {
		return nil, fmt.Errorf("refresh token consume: revoke: %w", err)
	}
	return &s, nil
}

// RevokeAll marks all active sessions for userID as revoked.
func (r *RefreshTokenService) RevokeAll(ctx context.Context, userID string) error {
	if _, err := r.db.ExecContext(ctx, `UPDATE sessions SET revoked = TRUE WHERE user_id = $1 AND revoked = FALSE`, userID); err != nil {
		return fmt.Errorf("refresh token revoke all: %w", err)
	}
	return nil
}

// CountActiveSessions returns the number of active (non-revoked, non-expired) sessions for a user.
func (r *RefreshTokenService) CountActiveSessions(ctx context.Context, userID string) (int, error) {
	var count int
	err := r.db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM sessions WHERE user_id = $1 AND revoked = FALSE AND expires_at > now()`,
		userID,
	).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("count active sessions: %w", err)
	}
	return count, nil
}

// RevokeOldest revokes the oldest active session for a user.
func (r *RefreshTokenService) RevokeOldest(ctx context.Context, userID string) error {
	_, err := r.db.ExecContext(ctx,
		`UPDATE sessions SET revoked = TRUE
		 WHERE id = (
			SELECT id FROM sessions
			WHERE user_id = $1 AND revoked = FALSE AND expires_at > now()
			ORDER BY created_at ASC
			LIMIT 1
		 )`,
		userID,
	)
	if err != nil {
		return fmt.Errorf("revoke oldest session: %w", err)
	}
	return nil
}

// GetLastUsedAt returns the last_used_at timestamp for a session.
func (r *RefreshTokenService) GetLastUsedAt(ctx context.Context, sessionID string) (*time.Time, error) {
	var lastUsed sql.NullTime
	err := r.db.QueryRowContext(ctx,
		`SELECT last_used_at FROM sessions WHERE id = $1`,
		sessionID,
	).Scan(&lastUsed)
	if err != nil {
		return nil, fmt.Errorf("get last used at: %w", err)
	}
	if lastUsed.Valid {
		return &lastUsed.Time, nil
	}
	return nil, nil
}

// TouchSession updates last_used_at on a session (debounced by caller).
func (r *RefreshTokenService) TouchSession(ctx context.Context, sessionID string) error {
	_, err := r.db.ExecContext(ctx,
		`UPDATE sessions SET last_used_at = now() WHERE id = $1`,
		sessionID,
	)
	if err != nil {
		return fmt.Errorf("touch session: %w", err)
	}
	return nil
}

// HashTokenPublic returns the hex-encoded SHA-256 hash of rawToken.
// Used by callers that need to store a hash of an access token in the sessions table.
func HashTokenPublic(rawToken string) string {
	return hashToken(rawToken)
}

// generateToken produces 64 cryptographically random bytes, base64url-encoded.
func generateToken() (string, error) {
	b := make([]byte, 64)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("generate token: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

// hashToken returns the hex-encoded SHA-256 hash of rawToken.
func hashToken(rawToken string) string {
	sum := sha256.Sum256([]byte(rawToken))
	return fmt.Sprintf("%x", sum)
}
