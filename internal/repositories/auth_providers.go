// Package repositories provides database access for Tackle domain objects.
package repositories

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/google/uuid"
)

// AuthProviderType represents the type of an external auth provider.
type AuthProviderType string

const (
	// AuthProviderOIDC is a generic OIDC / OAuth2 provider.
	AuthProviderOIDC AuthProviderType = "oidc"
	// AuthProviderFusionAuth is a FusionAuth-specific OIDC provider.
	AuthProviderFusionAuth AuthProviderType = "fusionauth"
	// AuthProviderLDAP is an LDAP/Active Directory provider.
	AuthProviderLDAP AuthProviderType = "ldap"
)

// AuthOrder controls whether local or LDAP auth is attempted first.
type AuthOrder string

const (
	// AuthOrderLocalFirst tries local auth before LDAP.
	AuthOrderLocalFirst AuthOrder = "local_first"
	// AuthOrderLDAPFirst tries LDAP auth before local.
	AuthOrderLDAPFirst AuthOrder = "ldap_first"
)

// AuthProvider is a row from the auth_providers table.
type AuthProvider struct {
	ID              string
	Type            AuthProviderType
	Name            string
	Configuration   []byte // AES-256-GCM encrypted JSON
	Enabled         bool
	DefaultRoleID   *string
	AutoProvision   bool
	AuthOrder       AuthOrder
	CreatedAt       time.Time
	UpdatedAt       time.Time
}

// AuthProviderRepository handles database access for auth_providers.
type AuthProviderRepository struct {
	db *sql.DB
}

// NewAuthProviderRepository creates a new AuthProviderRepository.
func NewAuthProviderRepository(db *sql.DB) *AuthProviderRepository {
	return &AuthProviderRepository{db: db}
}

// Create inserts a new auth provider configuration.
func (r *AuthProviderRepository) Create(ctx context.Context, p AuthProvider) (AuthProvider, error) {
	p.ID = uuid.New().String()
	const q = `
		INSERT INTO auth_providers (id, type, name, configuration, enabled, default_role_id, auto_provision, auth_order)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		RETURNING id, type, name, configuration, enabled, default_role_id, auto_provision, auth_order, created_at, updated_at`
	return r.scanOne(r.db.QueryRowContext(ctx, q,
		p.ID, p.Type, p.Name, p.Configuration, p.Enabled, p.DefaultRoleID, p.AutoProvision, p.AuthOrder,
	))
}

// GetByID returns a single provider by UUID.
func (r *AuthProviderRepository) GetByID(ctx context.Context, id string) (AuthProvider, error) {
	const q = `
		SELECT id, type, name, configuration, enabled, default_role_id, auto_provision, auth_order, created_at, updated_at
		FROM auth_providers WHERE id = $1`
	return r.scanOne(r.db.QueryRowContext(ctx, q, id))
}

// GetByProviderType returns all providers of the given type.
func (r *AuthProviderRepository) GetByProviderType(ctx context.Context, t AuthProviderType) ([]AuthProvider, error) {
	const q = `
		SELECT id, type, name, configuration, enabled, default_role_id, auto_provision, auth_order, created_at, updated_at
		FROM auth_providers WHERE type = $1 ORDER BY name`
	rows, err := r.db.QueryContext(ctx, q, t)
	if err != nil {
		return nil, fmt.Errorf("auth providers get by type: %w", err)
	}
	defer rows.Close()
	return r.scanMany(rows)
}

// ListEnabled returns all enabled providers.
func (r *AuthProviderRepository) ListEnabled(ctx context.Context) ([]AuthProvider, error) {
	const q = `
		SELECT id, type, name, configuration, enabled, default_role_id, auto_provision, auth_order, created_at, updated_at
		FROM auth_providers WHERE enabled = TRUE ORDER BY name`
	rows, err := r.db.QueryContext(ctx, q)
	if err != nil {
		return nil, fmt.Errorf("auth providers list enabled: %w", err)
	}
	defer rows.Close()
	return r.scanMany(rows)
}

// List returns all providers.
func (r *AuthProviderRepository) List(ctx context.Context) ([]AuthProvider, error) {
	const q = `
		SELECT id, type, name, configuration, enabled, default_role_id, auto_provision, auth_order, created_at, updated_at
		FROM auth_providers ORDER BY name`
	rows, err := r.db.QueryContext(ctx, q)
	if err != nil {
		return nil, fmt.Errorf("auth providers list: %w", err)
	}
	defer rows.Close()
	return r.scanMany(rows)
}

// Update replaces mutable fields of a provider configuration.
func (r *AuthProviderRepository) Update(ctx context.Context, id string, p AuthProvider) (AuthProvider, error) {
	const q = `
		UPDATE auth_providers
		SET name = $2, configuration = $3, enabled = $4, default_role_id = $5,
		    auto_provision = $6, auth_order = $7
		WHERE id = $1
		RETURNING id, type, name, configuration, enabled, default_role_id, auto_provision, auth_order, created_at, updated_at`
	return r.scanOne(r.db.QueryRowContext(ctx, q,
		id, p.Name, p.Configuration, p.Enabled, p.DefaultRoleID, p.AutoProvision, p.AuthOrder,
	))
}

// Delete removes a provider configuration.
func (r *AuthProviderRepository) Delete(ctx context.Context, id string) error {
	_, err := r.db.ExecContext(ctx, `DELETE FROM auth_providers WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("auth providers delete: %w", err)
	}
	return nil
}

// Enable sets enabled = TRUE for a provider.
func (r *AuthProviderRepository) Enable(ctx context.Context, id string) error {
	_, err := r.db.ExecContext(ctx, `UPDATE auth_providers SET enabled = TRUE WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("auth providers enable: %w", err)
	}
	return nil
}

// Disable sets enabled = FALSE for a provider.
func (r *AuthProviderRepository) Disable(ctx context.Context, id string) error {
	_, err := r.db.ExecContext(ctx, `UPDATE auth_providers SET enabled = FALSE WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("auth providers disable: %w", err)
	}
	return nil
}

func (r *AuthProviderRepository) scanOne(row *sql.Row) (AuthProvider, error) {
	var p AuthProvider
	var defaultRoleID sql.NullString
	err := row.Scan(
		&p.ID, &p.Type, &p.Name, &p.Configuration, &p.Enabled,
		&defaultRoleID, &p.AutoProvision, &p.AuthOrder,
		&p.CreatedAt, &p.UpdatedAt,
	)
	if err == sql.ErrNoRows {
		return AuthProvider{}, fmt.Errorf("auth provider not found")
	}
	if err != nil {
		return AuthProvider{}, fmt.Errorf("auth providers scan: %w", err)
	}
	if defaultRoleID.Valid {
		p.DefaultRoleID = &defaultRoleID.String
	}
	return p, nil
}

func (r *AuthProviderRepository) scanMany(rows *sql.Rows) ([]AuthProvider, error) {
	var out []AuthProvider
	for rows.Next() {
		var p AuthProvider
		var defaultRoleID sql.NullString
		if err := rows.Scan(
			&p.ID, &p.Type, &p.Name, &p.Configuration, &p.Enabled,
			&defaultRoleID, &p.AutoProvision, &p.AuthOrder,
			&p.CreatedAt, &p.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("auth providers scan row: %w", err)
		}
		if defaultRoleID.Valid {
			p.DefaultRoleID = &defaultRoleID.String
		}
		out = append(out, p)
	}
	return out, rows.Err()
}
