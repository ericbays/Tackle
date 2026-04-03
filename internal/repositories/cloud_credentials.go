package repositories

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
)

// CloudProviderType identifies a supported cloud infrastructure provider.
type CloudProviderType string

const (
	// CloudProviderAWS is Amazon Web Services.
	CloudProviderAWS CloudProviderType = "aws"
	// CloudProviderAzure is Microsoft Azure.
	CloudProviderAzure CloudProviderType = "azure"
	// CloudProviderProxmox is a Proxmox VE hypervisor.
	CloudProviderProxmox CloudProviderType = "proxmox"
)

// CloudCredentialStatus is the last-known test result for a credential set.
type CloudCredentialStatus string

const (
	// CloudCredentialStatusUntested means no test has been performed yet.
	CloudCredentialStatusUntested CloudCredentialStatus = "untested"
	// CloudCredentialStatusHealthy means the last test succeeded.
	CloudCredentialStatusHealthy CloudCredentialStatus = "healthy"
	// CloudCredentialStatusError means the last test failed.
	CloudCredentialStatusError CloudCredentialStatus = "error"
)

// CloudCredential is the DB model for a cloud_credentials row.
type CloudCredential struct {
	ID                   string
	ProviderType         CloudProviderType
	DisplayName          string
	CredentialsEncrypted []byte
	DefaultRegion        string
	Status               CloudCredentialStatus
	StatusMessage        *string
	LastTestedAt         *time.Time
	CreatedBy            string
	CreatedAt            time.Time
	UpdatedAt            time.Time
}

// CloudCredentialFilters controls optional filtering for List.
type CloudCredentialFilters struct {
	ProviderType string // empty = all
	Status       string // empty = all
}

// CloudCredentialRepository provides database operations for cloud_credentials.
type CloudCredentialRepository struct {
	db *sql.DB
}

// NewCloudCredentialRepository creates a new CloudCredentialRepository.
func NewCloudCredentialRepository(db *sql.DB) *CloudCredentialRepository {
	return &CloudCredentialRepository{db: db}
}

// Create inserts a new cloud credential record and returns the created row.
func (r *CloudCredentialRepository) Create(ctx context.Context, c CloudCredential) (CloudCredential, error) {
	id := uuid.New().String()
	const q = `
		INSERT INTO cloud_credentials
			(id, provider_type, display_name, credentials_encrypted, default_region, created_by)
		VALUES ($1,$2,$3,$4,$5,$6)
		RETURNING id, provider_type, display_name, credentials_encrypted, default_region,
		          status, status_message, last_tested_at, created_by, created_at, updated_at`
	return r.scanOne(r.db.QueryRowContext(ctx, q,
		id, string(c.ProviderType), c.DisplayName,
		c.CredentialsEncrypted, c.DefaultRegion, c.CreatedBy,
	))
}

// GetByID retrieves a cloud credential by UUID. Returns sql.ErrNoRows if not found.
func (r *CloudCredentialRepository) GetByID(ctx context.Context, id string) (CloudCredential, error) {
	const q = `
		SELECT id, provider_type, display_name, credentials_encrypted, default_region,
		       status, status_message, last_tested_at, created_by, created_at, updated_at
		FROM cloud_credentials WHERE id = $1`
	return r.scanOne(r.db.QueryRowContext(ctx, q, id))
}

// List returns cloud credentials with optional filtering.
func (r *CloudCredentialRepository) List(ctx context.Context, filters CloudCredentialFilters) ([]CloudCredential, error) {
	args := []any{}
	argIdx := 1
	where := "WHERE 1=1"

	if filters.ProviderType != "" {
		where += fmt.Sprintf(" AND provider_type = $%d", argIdx)
		args = append(args, filters.ProviderType)
		argIdx++
	}
	if filters.Status != "" {
		where += fmt.Sprintf(" AND status = $%d", argIdx)
		args = append(args, filters.Status)
		argIdx++
	}
	_ = argIdx

	q := fmt.Sprintf(`
		SELECT id, provider_type, display_name, credentials_encrypted, default_region,
		       status, status_message, last_tested_at, created_by, created_at, updated_at
		FROM cloud_credentials %s
		ORDER BY display_name ASC`, where)

	rows, err := r.db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, fmt.Errorf("cloud credentials: list: %w", err)
	}
	defer rows.Close()

	var results []CloudCredential
	for rows.Next() {
		c, err := r.scanRow(rows)
		if err != nil {
			return nil, fmt.Errorf("cloud credentials: list scan: %w", err)
		}
		results = append(results, c)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("cloud credentials: list rows: %w", err)
	}
	return results, nil
}

// CloudCredentialUpdate holds mutable fields for an update operation.
type CloudCredentialUpdate struct {
	DisplayName          *string
	CredentialsEncrypted []byte  // nil = no change
	DefaultRegion        *string
}

// Update applies the given changes to the credential record identified by id.
func (r *CloudCredentialRepository) Update(ctx context.Context, id string, upd CloudCredentialUpdate) (CloudCredential, error) {
	setClauses := []string{}
	args := []any{}
	argIdx := 1

	if upd.DisplayName != nil {
		setClauses = append(setClauses, fmt.Sprintf("display_name = $%d", argIdx))
		args = append(args, *upd.DisplayName)
		argIdx++
	}
	if upd.CredentialsEncrypted != nil {
		setClauses = append(setClauses, fmt.Sprintf("credentials_encrypted = $%d", argIdx))
		args = append(args, upd.CredentialsEncrypted)
		argIdx++
		// Reset status when credentials change.
		setClauses = append(setClauses, fmt.Sprintf("status = $%d", argIdx))
		args = append(args, string(CloudCredentialStatusUntested))
		argIdx++
		setClauses = append(setClauses, fmt.Sprintf("status_message = $%d", argIdx))
		args = append(args, nil)
		argIdx++
		setClauses = append(setClauses, fmt.Sprintf("last_tested_at = $%d", argIdx))
		args = append(args, nil)
		argIdx++
	}
	if upd.DefaultRegion != nil {
		setClauses = append(setClauses, fmt.Sprintf("default_region = $%d", argIdx))
		args = append(args, *upd.DefaultRegion)
		argIdx++
	}

	if len(setClauses) == 0 {
		return r.GetByID(ctx, id)
	}

	args = append(args, id)
	q := fmt.Sprintf(`
		UPDATE cloud_credentials SET %s
		WHERE id = $%d
		RETURNING id, provider_type, display_name, credentials_encrypted, default_region,
		          status, status_message, last_tested_at, created_by, created_at, updated_at`,
		strings.Join(setClauses, ", "), argIdx)

	return r.scanOne(r.db.QueryRowContext(ctx, q, args...))
}

// Delete hard-deletes the credential record. Caller must check references first.
func (r *CloudCredentialRepository) Delete(ctx context.Context, id string) error {
	result, err := r.db.ExecContext(ctx, "DELETE FROM cloud_credentials WHERE id = $1", id)
	if err != nil {
		return fmt.Errorf("cloud credentials: delete: %w", err)
	}
	n, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("cloud credentials: delete rows affected: %w", err)
	}
	if n == 0 {
		return fmt.Errorf("cloud credentials: delete: %w", sql.ErrNoRows)
	}
	return nil
}

// UpdateStatus updates the status, message, and last_tested_at timestamp.
func (r *CloudCredentialRepository) UpdateStatus(ctx context.Context, id string, status CloudCredentialStatus, message *string) error {
	result, err := r.db.ExecContext(ctx,
		"UPDATE cloud_credentials SET status = $1, status_message = $2, last_tested_at = now() WHERE id = $3",
		string(status), message, id)
	if err != nil {
		return fmt.Errorf("cloud credentials: update status: %w", err)
	}
	n, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("cloud credentials: update status rows: %w", err)
	}
	if n == 0 {
		return fmt.Errorf("cloud credentials: update status: %w", sql.ErrNoRows)
	}
	return nil
}

// HasTemplateReferences returns true if any instance templates reference the given credential.
func (r *CloudCredentialRepository) HasTemplateReferences(ctx context.Context, id string) (bool, error) {
	var count int
	if err := r.db.QueryRowContext(ctx,
		"SELECT COUNT(*) FROM instance_templates WHERE cloud_credential_id = $1", id).Scan(&count); err != nil {
		return false, fmt.Errorf("cloud credentials: check template references: %w", err)
	}
	return count > 0, nil
}

func (r *CloudCredentialRepository) scanOne(row *sql.Row) (CloudCredential, error) {
	var c CloudCredential
	err := row.Scan(
		&c.ID, &c.ProviderType, &c.DisplayName, &c.CredentialsEncrypted, &c.DefaultRegion,
		&c.Status, &c.StatusMessage, &c.LastTestedAt, &c.CreatedBy, &c.CreatedAt, &c.UpdatedAt,
	)
	if err != nil {
		return CloudCredential{}, fmt.Errorf("cloud credentials: scan: %w", err)
	}
	return c, nil
}

func (r *CloudCredentialRepository) scanRow(rows *sql.Rows) (CloudCredential, error) {
	var c CloudCredential
	err := rows.Scan(
		&c.ID, &c.ProviderType, &c.DisplayName, &c.CredentialsEncrypted, &c.DefaultRegion,
		&c.Status, &c.StatusMessage, &c.LastTestedAt, &c.CreatedBy, &c.CreatedAt, &c.UpdatedAt,
	)
	if err != nil {
		return CloudCredential{}, err
	}
	return c, nil
}
