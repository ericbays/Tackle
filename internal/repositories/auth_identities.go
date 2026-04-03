package repositories

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/google/uuid"
)

// AuthIdentity links an external provider identity to a local user account.
type AuthIdentity struct {
	ID               string
	UserID           string
	ProviderType     AuthProviderType
	ProviderConfigID string
	ExternalSubject  string
	ExternalEmail    *string
	CreatedAt        time.Time
}

// AuthIdentityRepository handles database access for auth_identities.
type AuthIdentityRepository struct {
	db *sql.DB
}

// NewAuthIdentityRepository creates a new AuthIdentityRepository.
func NewAuthIdentityRepository(db *sql.DB) *AuthIdentityRepository {
	return &AuthIdentityRepository{db: db}
}

// Create links a new external identity to a user.
func (r *AuthIdentityRepository) Create(ctx context.Context, identity AuthIdentity) (AuthIdentity, error) {
	identity.ID = uuid.New().String()
	const q = `
		INSERT INTO auth_identities (id, user_id, provider_type, provider_config_id, external_subject, external_email)
		VALUES ($1, $2, $3, $4, $5, $6)
		RETURNING id, user_id, provider_type, provider_config_id, external_subject, external_email, created_at`
	return r.scanOne(r.db.QueryRowContext(ctx, q,
		identity.ID, identity.UserID, identity.ProviderType, identity.ProviderConfigID,
		identity.ExternalSubject, identity.ExternalEmail,
	))
}

// GetByExternalSubject returns the identity linked to a given (providerConfigID, externalSubject) pair.
func (r *AuthIdentityRepository) GetByExternalSubject(ctx context.Context, providerConfigID, externalSubject string) (AuthIdentity, error) {
	const q = `
		SELECT id, user_id, provider_type, provider_config_id, external_subject, external_email, created_at
		FROM auth_identities WHERE provider_config_id = $1 AND external_subject = $2`
	return r.scanOne(r.db.QueryRowContext(ctx, q, providerConfigID, externalSubject))
}

// GetByUserID returns all identities linked to a user.
func (r *AuthIdentityRepository) GetByUserID(ctx context.Context, userID string) ([]AuthIdentity, error) {
	const q = `
		SELECT id, user_id, provider_type, provider_config_id, external_subject, external_email, created_at
		FROM auth_identities WHERE user_id = $1 ORDER BY created_at`
	rows, err := r.db.QueryContext(ctx, q, userID)
	if err != nil {
		return nil, fmt.Errorf("auth identities get by user: %w", err)
	}
	defer rows.Close()
	return r.scanMany(rows)
}

// GetByEmail returns the first identity whose external_email matches (for email-match linking).
func (r *AuthIdentityRepository) GetByEmail(ctx context.Context, email string) (AuthIdentity, error) {
	const q = `
		SELECT id, user_id, provider_type, provider_config_id, external_subject, external_email, created_at
		FROM auth_identities WHERE external_email = $1 LIMIT 1`
	return r.scanOne(r.db.QueryRowContext(ctx, q, email))
}

// Delete removes a linked identity.
func (r *AuthIdentityRepository) Delete(ctx context.Context, id string) error {
	_, err := r.db.ExecContext(ctx, `DELETE FROM auth_identities WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("auth identities delete: %w", err)
	}
	return nil
}

// CountByUserID returns the number of linked identities for a user.
func (r *AuthIdentityRepository) CountByUserID(ctx context.Context, userID string) (int, error) {
	var count int
	err := r.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM auth_identities WHERE user_id = $1`, userID).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("auth identities count: %w", err)
	}
	return count, nil
}

func (r *AuthIdentityRepository) scanOne(row *sql.Row) (AuthIdentity, error) {
	var i AuthIdentity
	var email sql.NullString
	err := row.Scan(&i.ID, &i.UserID, &i.ProviderType, &i.ProviderConfigID, &i.ExternalSubject, &email, &i.CreatedAt)
	if err == sql.ErrNoRows {
		return AuthIdentity{}, fmt.Errorf("auth identity not found")
	}
	if err != nil {
		return AuthIdentity{}, fmt.Errorf("auth identities scan: %w", err)
	}
	if email.Valid {
		i.ExternalEmail = &email.String
	}
	return i, nil
}

func (r *AuthIdentityRepository) scanMany(rows *sql.Rows) ([]AuthIdentity, error) {
	var out []AuthIdentity
	for rows.Next() {
		var i AuthIdentity
		var email sql.NullString
		if err := rows.Scan(&i.ID, &i.UserID, &i.ProviderType, &i.ProviderConfigID, &i.ExternalSubject, &email, &i.CreatedAt); err != nil {
			return nil, fmt.Errorf("auth identities scan row: %w", err)
		}
		if email.Valid {
			i.ExternalEmail = &email.String
		}
		out = append(out, i)
	}
	return out, rows.Err()
}
