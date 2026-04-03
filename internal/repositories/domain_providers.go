// Package repositories provides database access layers for Tackle.
package repositories

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/google/uuid"
)

// ProviderStatus represents the health status of a domain provider connection.
type ProviderStatus string

const (
	// ProviderStatusUntested indicates the connection has never been tested.
	ProviderStatusUntested ProviderStatus = "untested"
	// ProviderStatusHealthy indicates the last test succeeded.
	ProviderStatusHealthy ProviderStatus = "healthy"
	// ProviderStatusError indicates the last test failed.
	ProviderStatusError ProviderStatus = "error"
)

// DomainProviderConnection is the DB model for a domain provider connection row.
type DomainProviderConnection struct {
	ID                   string
	ProviderType         string
	DisplayName          string
	CredentialsEncrypted []byte
	Status               ProviderStatus
	StatusMessage        *string
	LastTestedAt         *time.Time
	CreatedBy            string
	CreatedAt            time.Time
	UpdatedAt            time.Time
}

// DomainProviderFilters are optional filters for listing connections.
type DomainProviderFilters struct {
	ProviderType string // empty = all types
	Status       string // empty = all statuses
}

// DomainProviderRepository provides database operations for domain provider connections.
type DomainProviderRepository struct {
	db *sql.DB
}

// NewDomainProviderRepository creates a new DomainProviderRepository.
func NewDomainProviderRepository(db *sql.DB) *DomainProviderRepository {
	return &DomainProviderRepository{db: db}
}

// Create inserts a new domain provider connection row and returns the created record.
func (r *DomainProviderRepository) Create(ctx context.Context, conn DomainProviderConnection) (DomainProviderConnection, error) {
	id := uuid.New().String()
	const q = `
		INSERT INTO domain_provider_connections
			(id, provider_type, display_name, credentials_encrypted, status, created_by)
		VALUES ($1, $2, $3, $4, 'untested', $5)
		RETURNING id, provider_type, display_name, credentials_encrypted, status,
		          status_message, last_tested_at, created_by, created_at, updated_at`

	var out DomainProviderConnection
	err := r.db.QueryRowContext(ctx, q,
		id,
		conn.ProviderType,
		conn.DisplayName,
		conn.CredentialsEncrypted,
		conn.CreatedBy,
	).Scan(
		&out.ID,
		&out.ProviderType,
		&out.DisplayName,
		&out.CredentialsEncrypted,
		&out.Status,
		&out.StatusMessage,
		&out.LastTestedAt,
		&out.CreatedBy,
		&out.CreatedAt,
		&out.UpdatedAt,
	)
	if err != nil {
		return DomainProviderConnection{}, fmt.Errorf("domain provider: create: %w", err)
	}
	return out, nil
}

// GetByID retrieves a domain provider connection by its UUID.
// Returns sql.ErrNoRows if not found.
func (r *DomainProviderRepository) GetByID(ctx context.Context, id string) (DomainProviderConnection, error) {
	const q = `
		SELECT id, provider_type, display_name, credentials_encrypted, status,
		       status_message, last_tested_at, created_by, created_at, updated_at
		FROM domain_provider_connections
		WHERE id = $1`

	var out DomainProviderConnection
	err := r.db.QueryRowContext(ctx, q, id).Scan(
		&out.ID,
		&out.ProviderType,
		&out.DisplayName,
		&out.CredentialsEncrypted,
		&out.Status,
		&out.StatusMessage,
		&out.LastTestedAt,
		&out.CreatedBy,
		&out.CreatedAt,
		&out.UpdatedAt,
	)
	if err != nil {
		return DomainProviderConnection{}, fmt.Errorf("domain provider: get by id: %w", err)
	}
	return out, nil
}

// List returns all domain provider connections, optionally filtered.
func (r *DomainProviderRepository) List(ctx context.Context, filters DomainProviderFilters) ([]DomainProviderConnection, error) {
	q := `
		SELECT id, provider_type, display_name, credentials_encrypted, status,
		       status_message, last_tested_at, created_by, created_at, updated_at
		FROM domain_provider_connections
		WHERE 1=1`
	args := []any{}
	argIdx := 1

	if filters.ProviderType != "" {
		q += fmt.Sprintf(" AND provider_type = $%d", argIdx)
		args = append(args, filters.ProviderType)
		argIdx++
	}
	if filters.Status != "" {
		q += fmt.Sprintf(" AND status = $%d", argIdx)
		args = append(args, filters.Status)
		argIdx++
	}
	q += " ORDER BY display_name ASC"
	_ = argIdx

	rows, err := r.db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, fmt.Errorf("domain provider: list: %w", err)
	}
	defer rows.Close()

	var results []DomainProviderConnection
	for rows.Next() {
		var c DomainProviderConnection
		if err := rows.Scan(
			&c.ID,
			&c.ProviderType,
			&c.DisplayName,
			&c.CredentialsEncrypted,
			&c.Status,
			&c.StatusMessage,
			&c.LastTestedAt,
			&c.CreatedBy,
			&c.CreatedAt,
			&c.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("domain provider: list scan: %w", err)
		}
		results = append(results, c)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("domain provider: list rows: %w", err)
	}
	return results, nil
}

// Update modifies the display_name and/or credentials_encrypted fields of an existing connection.
// Only non-nil fields in the update struct are changed.
type DomainProviderUpdate struct {
	DisplayName          *string
	CredentialsEncrypted []byte // nil = do not update
}

// Update applies the given changes to the connection identified by id.
func (r *DomainProviderRepository) Update(ctx context.Context, id string, updates DomainProviderUpdate) (DomainProviderConnection, error) {
	// Build SET clause dynamically.
	setClauses := []string{}
	args := []any{}
	argIdx := 1

	if updates.DisplayName != nil {
		setClauses = append(setClauses, fmt.Sprintf("display_name = $%d", argIdx))
		args = append(args, *updates.DisplayName)
		argIdx++
	}
	if updates.CredentialsEncrypted != nil {
		setClauses = append(setClauses, fmt.Sprintf("credentials_encrypted = $%d", argIdx))
		args = append(args, updates.CredentialsEncrypted)
		argIdx++
		// When credentials change, reset status to untested.
		setClauses = append(setClauses, "status = 'untested'")
		setClauses = append(setClauses, "status_message = NULL")
		setClauses = append(setClauses, "last_tested_at = NULL")
	}

	if len(setClauses) == 0 {
		return r.GetByID(ctx, id)
	}

	// Build full query.
	setStr := ""
	for i, c := range setClauses {
		if i > 0 {
			setStr += ", "
		}
		setStr += c
	}
	args = append(args, id)
	q := fmt.Sprintf(`
		UPDATE domain_provider_connections
		SET %s
		WHERE id = $%d
		RETURNING id, provider_type, display_name, credentials_encrypted, status,
		          status_message, last_tested_at, created_by, created_at, updated_at`,
		setStr, argIdx)

	var out DomainProviderConnection
	err := r.db.QueryRowContext(ctx, q, args...).Scan(
		&out.ID,
		&out.ProviderType,
		&out.DisplayName,
		&out.CredentialsEncrypted,
		&out.Status,
		&out.StatusMessage,
		&out.LastTestedAt,
		&out.CreatedBy,
		&out.CreatedAt,
		&out.UpdatedAt,
	)
	if err != nil {
		return DomainProviderConnection{}, fmt.Errorf("domain provider: update: %w", err)
	}
	return out, nil
}

// Delete removes a domain provider connection by ID.
// Returns sql.ErrNoRows if not found.
func (r *DomainProviderRepository) Delete(ctx context.Context, id string) error {
	const q = `DELETE FROM domain_provider_connections WHERE id = $1`
	result, err := r.db.ExecContext(ctx, q, id)
	if err != nil {
		return fmt.Errorf("domain provider: delete: %w", err)
	}
	n, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("domain provider: delete rows affected: %w", err)
	}
	if n == 0 {
		return fmt.Errorf("domain provider: delete: %w", sql.ErrNoRows)
	}
	return nil
}

// UpdateStatus sets the status, status_message, and last_tested_at fields.
func (r *DomainProviderRepository) UpdateStatus(ctx context.Context, id string, status ProviderStatus, message *string) error {
	const q = `
		UPDATE domain_provider_connections
		SET status = $1, status_message = $2, last_tested_at = now()
		WHERE id = $3`
	result, err := r.db.ExecContext(ctx, q, string(status), message, id)
	if err != nil {
		return fmt.Errorf("domain provider: update status: %w", err)
	}
	n, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("domain provider: update status rows affected: %w", err)
	}
	if n == 0 {
		return fmt.Errorf("domain provider: update status: %w", sql.ErrNoRows)
	}
	return nil
}

// ExistsDisplayName returns true if any connection (excluding optional excludeID) uses the given name.
func (r *DomainProviderRepository) ExistsDisplayName(ctx context.Context, name string, excludeID string) (bool, error) {
	var q string
	var args []any
	if excludeID != "" {
		q = `SELECT EXISTS(SELECT 1 FROM domain_provider_connections WHERE display_name = $1 AND id != $2)`
		args = []any{name, excludeID}
	} else {
		q = `SELECT EXISTS(SELECT 1 FROM domain_provider_connections WHERE display_name = $1)`
		args = []any{name}
	}
	var exists bool
	if err := r.db.QueryRowContext(ctx, q, args...).Scan(&exists); err != nil {
		return false, fmt.Errorf("domain provider: exists display name: %w", err)
	}
	return exists, nil
}
