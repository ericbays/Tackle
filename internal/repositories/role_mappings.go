package repositories

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/google/uuid"
)

// RoleMapping maps an external group name from a provider to a Tackle role.
type RoleMapping struct {
	ID               string
	ProviderConfigID string
	ExternalGroup    string
	RoleID           string
	CreatedAt        time.Time
	UpdatedAt        time.Time
}

// RoleMappingRepository handles database access for role_mappings.
type RoleMappingRepository struct {
	db *sql.DB
}

// NewRoleMappingRepository creates a new RoleMappingRepository.
func NewRoleMappingRepository(db *sql.DB) *RoleMappingRepository {
	return &RoleMappingRepository{db: db}
}

// GetMappingsForProvider returns all role mappings for a given provider config.
func (r *RoleMappingRepository) GetMappingsForProvider(ctx context.Context, providerConfigID string) ([]RoleMapping, error) {
	const q = `
		SELECT id, provider_config_id, external_group, role_id, created_at, updated_at
		FROM role_mappings WHERE provider_config_id = $1 ORDER BY external_group`
	rows, err := r.db.QueryContext(ctx, q, providerConfigID)
	if err != nil {
		return nil, fmt.Errorf("role mappings get for provider: %w", err)
	}
	defer rows.Close()
	var out []RoleMapping
	for rows.Next() {
		var m RoleMapping
		if err := rows.Scan(&m.ID, &m.ProviderConfigID, &m.ExternalGroup, &m.RoleID, &m.CreatedAt, &m.UpdatedAt); err != nil {
			return nil, fmt.Errorf("role mappings scan: %w", err)
		}
		out = append(out, m)
	}
	return out, rows.Err()
}

// SetMappings atomically replaces all role mappings for a provider.
func (r *RoleMappingRepository) SetMappings(ctx context.Context, providerConfigID string, mappings []RoleMapping) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("role mappings set: begin tx: %w", err)
	}
	defer tx.Rollback() //nolint:errcheck

	if _, err := tx.ExecContext(ctx, `DELETE FROM role_mappings WHERE provider_config_id = $1`, providerConfigID); err != nil {
		return fmt.Errorf("role mappings set: delete: %w", err)
	}

	for _, m := range mappings {
		id := uuid.New().String()
		if _, err := tx.ExecContext(ctx,
			`INSERT INTO role_mappings (id, provider_config_id, external_group, role_id) VALUES ($1, $2, $3, $4)`,
			id, providerConfigID, m.ExternalGroup, m.RoleID,
		); err != nil {
			return fmt.Errorf("role mappings set: insert: %w", err)
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("role mappings set: commit: %w", err)
	}
	return nil
}

// ResolveRole returns the role ID for the first matching external group,
// or defaultRoleID if no mapping matches.
func (r *RoleMappingRepository) ResolveRole(ctx context.Context, providerConfigID string, externalGroups []string, defaultRoleID string) (string, error) {
	if len(externalGroups) == 0 {
		return defaultRoleID, nil
	}

	mappings, err := r.GetMappingsForProvider(ctx, providerConfigID)
	if err != nil {
		return "", fmt.Errorf("resolve role: %w", err)
	}

	// Build lookup map from external group to role ID.
	lookup := make(map[string]string, len(mappings))
	for _, m := range mappings {
		lookup[m.ExternalGroup] = m.RoleID
	}

	// Return first match in the order groups are provided.
	for _, g := range externalGroups {
		if roleID, ok := lookup[g]; ok {
			return roleID, nil
		}
	}

	return defaultRoleID, nil
}
