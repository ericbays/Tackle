// Package repositories provides database access for domain health checks and categorizations.
package repositories

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
)

// HealthCheckType is the type of health check performed.
type HealthCheckType string

const (
	// HealthCheckTypeFull runs all health checks.
	HealthCheckTypeFull HealthCheckType = "full"
	// HealthCheckTypePropagationOnly runs only DNS propagation checks.
	HealthCheckTypePropagationOnly HealthCheckType = "propagation_only"
	// HealthCheckTypeBlocklistOnly runs only blocklist checks.
	HealthCheckTypeBlocklistOnly HealthCheckType = "blocklist_only"
	// HealthCheckTypeEmailAuthOnly runs only email auth checks.
	HealthCheckTypeEmailAuthOnly HealthCheckType = "email_auth_only"
	// HealthCheckTypeMXOnly runs only MX resolution checks.
	HealthCheckTypeMXOnly HealthCheckType = "mx_only"
)

// HealthOverallStatus is the aggregate status of a health check.
type HealthOverallStatus string

const (
	// HealthStatusHealthy means all checks passed.
	HealthStatusHealthy HealthOverallStatus = "healthy"
	// HealthStatusWarning means at least one check returned a warning.
	HealthStatusWarning HealthOverallStatus = "warning"
	// HealthStatusCritical means at least one check returned a critical result.
	HealthStatusCritical HealthOverallStatus = "critical"
)

// HealthTrigger describes what initiated the health check.
type HealthTrigger string

const (
	// HealthTriggerManual means the check was triggered by a user.
	HealthTriggerManual HealthTrigger = "manual"
	// HealthTriggerScheduled means the check was triggered by the scheduler.
	HealthTriggerScheduled HealthTrigger = "scheduled"
	// HealthTriggerDNSChange means the check was triggered by a DNS record change.
	HealthTriggerDNSChange HealthTrigger = "dns_change"
)

// DomainHealthCheck is the DB model for a domain health check record.
type DomainHealthCheck struct {
	ID              string
	DomainProfileID string
	CheckType       HealthCheckType
	OverallStatus   HealthOverallStatus
	ResultsJSON     []byte
	TriggeredBy     HealthTrigger
	CreatedAt       time.Time
}

// CategorizationStatus is the status of a categorization result.
type CategorizationStatus string

const (
	// CategorizationStatusCategorized means the domain is categorized as benign.
	CategorizationStatusCategorized CategorizationStatus = "categorized"
	// CategorizationStatusUncategorized means the domain is new/unknown to the service.
	CategorizationStatusUncategorized CategorizationStatus = "uncategorized"
	// CategorizationStatusFlagged means the domain is flagged as suspicious/malicious.
	CategorizationStatusFlagged CategorizationStatus = "flagged"
	// CategorizationStatusUnknown means the check failed or was blocked.
	CategorizationStatusUnknown CategorizationStatus = "unknown"
)

// DomainCategorization is the DB model for a domain categorization record.
type DomainCategorization struct {
	ID              string
	DomainProfileID string
	Service         string
	Category        string
	Status          CategorizationStatus
	RawResponse     *string
	CheckedAt       time.Time
}

// DomainHealthRepository provides DB access for health checks and categorizations.
type DomainHealthRepository struct {
	db *sql.DB
}

// NewDomainHealthRepository creates a new DomainHealthRepository.
func NewDomainHealthRepository(db *sql.DB) *DomainHealthRepository {
	return &DomainHealthRepository{db: db}
}

// CreateHealthCheck inserts a new domain health check record.
func (r *DomainHealthRepository) CreateHealthCheck(ctx context.Context, check DomainHealthCheck) (DomainHealthCheck, error) {
	id := uuid.New().String()
	resultsJSON := check.ResultsJSON
	if resultsJSON == nil {
		resultsJSON = []byte("{}")
	}
	const q = `
		INSERT INTO domain_health_checks
			(id, domain_profile_id, check_type, overall_status, results_json, triggered_by)
		VALUES ($1, $2, $3, $4, $5, $6)
		RETURNING id, domain_profile_id, check_type, overall_status, results_json, triggered_by, created_at`

	var out DomainHealthCheck
	err := r.db.QueryRowContext(ctx, q,
		id,
		check.DomainProfileID,
		string(check.CheckType),
		string(check.OverallStatus),
		resultsJSON,
		string(check.TriggeredBy),
	).Scan(
		&out.ID, &out.DomainProfileID,
		&out.CheckType, &out.OverallStatus,
		&out.ResultsJSON, &out.TriggeredBy,
		&out.CreatedAt,
	)
	if err != nil {
		return DomainHealthCheck{}, fmt.Errorf("health repo: create health check: %w", err)
	}
	return out, nil
}

// ListHealthChecks returns health checks for a domain, newest first, with pagination.
func (r *DomainHealthRepository) ListHealthChecks(ctx context.Context, domainProfileID string, limit, offset int) ([]DomainHealthCheck, int, error) {
	if limit <= 0 {
		limit = 20
	}

	var total int
	const countQ = `SELECT COUNT(*) FROM domain_health_checks WHERE domain_profile_id = $1`
	if err := r.db.QueryRowContext(ctx, countQ, domainProfileID).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("health repo: count health checks: %w", err)
	}

	const q = `
		SELECT id, domain_profile_id, check_type, overall_status, results_json, triggered_by, created_at
		FROM domain_health_checks
		WHERE domain_profile_id = $1
		ORDER BY created_at DESC
		LIMIT $2 OFFSET $3`

	rows, err := r.db.QueryContext(ctx, q, domainProfileID, limit, offset)
	if err != nil {
		return nil, 0, fmt.Errorf("health repo: list health checks: %w", err)
	}
	defer rows.Close()

	var results []DomainHealthCheck
	for rows.Next() {
		var c DomainHealthCheck
		if err := rows.Scan(
			&c.ID, &c.DomainProfileID,
			&c.CheckType, &c.OverallStatus,
			&c.ResultsJSON, &c.TriggeredBy,
			&c.CreatedAt,
		); err != nil {
			return nil, 0, fmt.Errorf("health repo: scan health check: %w", err)
		}
		results = append(results, c)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("health repo: list health checks rows: %w", err)
	}
	return results, total, nil
}

// GetLatestHealthCheck returns the most recent health check for a domain.
func (r *DomainHealthRepository) GetLatestHealthCheck(ctx context.Context, domainProfileID string) (DomainHealthCheck, error) {
	const q = `
		SELECT id, domain_profile_id, check_type, overall_status, results_json, triggered_by, created_at
		FROM domain_health_checks
		WHERE domain_profile_id = $1
		ORDER BY created_at DESC
		LIMIT 1`

	var c DomainHealthCheck
	err := r.db.QueryRowContext(ctx, q, domainProfileID).Scan(
		&c.ID, &c.DomainProfileID,
		&c.CheckType, &c.OverallStatus,
		&c.ResultsJSON, &c.TriggeredBy,
		&c.CreatedAt,
	)
	if err != nil {
		return DomainHealthCheck{}, fmt.Errorf("health repo: get latest health check: %w", err)
	}
	return c, nil
}

// GetAllActiveProfileIDs returns the IDs of all active domain profiles.
func (r *DomainHealthRepository) GetAllActiveProfileIDs(ctx context.Context) ([]string, error) {
	const q = `SELECT id FROM domain_profiles WHERE status = 'active' ORDER BY domain_name ASC`
	rows, err := r.db.QueryContext(ctx, q)
	if err != nil {
		return nil, fmt.Errorf("health repo: get active profile IDs: %w", err)
	}
	defer rows.Close()

	var ids []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, fmt.Errorf("health repo: scan profile ID: %w", err)
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}

// UpsertCategorization inserts a new categorization record.
func (r *DomainHealthRepository) UpsertCategorization(ctx context.Context, cat DomainCategorization) (DomainCategorization, error) {
	id := uuid.New().String()
	const q = `
		INSERT INTO domain_categorizations
			(id, domain_profile_id, service, category, status, raw_response, checked_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		RETURNING id, domain_profile_id, service, category, status, raw_response, checked_at`

	var out DomainCategorization
	err := r.db.QueryRowContext(ctx, q,
		id,
		cat.DomainProfileID,
		cat.Service,
		cat.Category,
		string(cat.Status),
		cat.RawResponse,
		cat.CheckedAt,
	).Scan(
		&out.ID, &out.DomainProfileID,
		&out.Service, &out.Category,
		&out.Status, &out.RawResponse,
		&out.CheckedAt,
	)
	if err != nil {
		return DomainCategorization{}, fmt.Errorf("health repo: upsert categorization: %w", err)
	}
	return out, nil
}

// GetLatestCategorization returns the most recent categorization per service for a domain.
func (r *DomainHealthRepository) GetLatestCategorization(ctx context.Context, domainProfileID string) ([]DomainCategorization, error) {
	const q = `
		SELECT DISTINCT ON (service)
			id, domain_profile_id, service, category, status, raw_response, checked_at
		FROM domain_categorizations
		WHERE domain_profile_id = $1
		ORDER BY service, checked_at DESC`

	rows, err := r.db.QueryContext(ctx, q, domainProfileID)
	if err != nil {
		return nil, fmt.Errorf("health repo: get latest categorization: %w", err)
	}
	defer rows.Close()

	var results []DomainCategorization
	for rows.Next() {
		var c DomainCategorization
		if err := rows.Scan(
			&c.ID, &c.DomainProfileID,
			&c.Service, &c.Category,
			&c.Status, &c.RawResponse,
			&c.CheckedAt,
		); err != nil {
			return nil, fmt.Errorf("health repo: scan categorization: %w", err)
		}
		results = append(results, c)
	}
	return results, rows.Err()
}

// ListCategorizationHistory returns all categorization records for a domain and service, newest first.
func (r *DomainHealthRepository) ListCategorizationHistory(ctx context.Context, domainProfileID, service string, limit int) ([]DomainCategorization, error) {
	if limit <= 0 {
		limit = 50
	}
	args := []any{domainProfileID, limit}
	q := `
		SELECT id, domain_profile_id, service, category, status, raw_response, checked_at
		FROM domain_categorizations
		WHERE domain_profile_id = $1`
	if service != "" {
		q += ` AND service = $3`
		args = append(args, service)
	}
	q += ` ORDER BY checked_at DESC LIMIT $2`

	rows, err := r.db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, fmt.Errorf("health repo: list categorization history: %w", err)
	}
	defer rows.Close()

	var results []DomainCategorization
	for rows.Next() {
		var c DomainCategorization
		if err := rows.Scan(
			&c.ID, &c.DomainProfileID,
			&c.Service, &c.Category,
			&c.Status, &c.RawResponse,
			&c.CheckedAt,
		); err != nil {
			return nil, fmt.Errorf("health repo: scan categorization history: %w", err)
		}
		results = append(results, c)
	}
	return results, rows.Err()
}

// MarshalResults marshals an arbitrary value to JSON for storage in results_json.
func MarshalResults(v any) ([]byte, error) {
	b, err := json.Marshal(v)
	if err != nil {
		return nil, fmt.Errorf("marshal results: %w", err)
	}
	return b, nil
}
