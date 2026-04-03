package repositories

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/lib/pq"
)

// DomainStatus represents the lifecycle status of a domain profile.
type DomainStatus string

const (
	// DomainStatusPendingRegistration indicates registration is in flight or queued.
	DomainStatusPendingRegistration DomainStatus = "pending_registration"
	// DomainStatusActive indicates the domain is live and in use.
	DomainStatusActive DomainStatus = "active"
	// DomainStatusExpired indicates the domain has expired.
	DomainStatusExpired DomainStatus = "expired"
	// DomainStatusSuspended indicates the domain is suspended.
	DomainStatusSuspended DomainStatus = "suspended"
	// DomainStatusDecommissioned is the soft-delete state.
	DomainStatusDecommissioned DomainStatus = "decommissioned"
)

// DomainProfile is the DB model for a domain profile row.
type DomainProfile struct {
	ID                       string
	DomainName               string
	RegistrarConnectionID    *string
	DNSProviderConnectionID  *string
	Status                   DomainStatus
	RegistrationDate         *time.Time
	ExpiryDate               *time.Time
	Tags                     []string
	Notes                    *string
	AutoRenew                bool
	CreatedBy                string
	DeletedAt                *time.Time
	CreatedAt                time.Time
	UpdatedAt                time.Time
}

// DomainProfileFilters are optional filters for listing domain profiles.
type DomainProfileFilters struct {
	Status                  string   // empty = all
	RegistrarConnectionID   string   // empty = all
	DNSProviderConnectionID string   // empty = all
	Tag                     string   // empty = all
	ExpiryBefore            *time.Time
	ExpiryAfter             *time.Time
	CampaignID              string // empty = all
	Search                  string // partial domain_name match
}

// DomainProfileSort controls ordering of list results.
type DomainProfileSort struct {
	Field string // expiry_date | domain_name | status | created_at
	Desc  bool
}

// DomainRenewalRecord is the DB model for a renewal history row.
type DomainRenewalRecord struct {
	ID                     string
	DomainProfileID        string
	RenewalDate            time.Time
	DurationYears          int
	CostAmount             *float64
	CostCurrency           *string
	RegistrarConnectionID  *string
	InitiatedBy            *string
	CreatedAt              time.Time
}

// CampaignAssoc represents a campaign-domain association.
type CampaignAssoc struct {
	ID              string
	DomainProfileID string
	CampaignID      string
	AssociationType string
}

// DomainProfileRepository provides database operations for domain profiles.
type DomainProfileRepository struct {
	db *sql.DB
}

// NewDomainProfileRepository creates a new DomainProfileRepository.
func NewDomainProfileRepository(db *sql.DB) *DomainProfileRepository {
	return &DomainProfileRepository{db: db}
}

// Create inserts a new domain profile and returns the created record.
func (r *DomainProfileRepository) Create(ctx context.Context, p DomainProfile) (DomainProfile, error) {
	id := uuid.New().String()
	const q = `
		INSERT INTO domain_profiles
			(id, domain_name, registrar_connection_id, dns_provider_connection_id,
			 status, registration_date, expiry_date, tags, notes, auto_renew, created_by)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11)
		RETURNING id, domain_name, registrar_connection_id, dns_provider_connection_id,
		          status, registration_date, expiry_date, tags, notes, auto_renew, created_by, deleted_at, created_at, updated_at`

	var out DomainProfile
	err := r.db.QueryRowContext(ctx, q,
		id,
		p.DomainName,
		p.RegistrarConnectionID,
		p.DNSProviderConnectionID,
		string(p.Status),
		p.RegistrationDate,
		p.ExpiryDate,
		pq.Array(p.Tags),
		p.Notes,
		p.AutoRenew,
		p.CreatedBy,
	).Scan(
		&out.ID,
		&out.DomainName,
		&out.RegistrarConnectionID,
		&out.DNSProviderConnectionID,
		&out.Status,
		&out.RegistrationDate,
		&out.ExpiryDate,
		pq.Array(&out.Tags),
		&out.Notes, &out.AutoRenew,
		&out.CreatedBy, &out.DeletedAt,
		&out.CreatedAt,
		&out.UpdatedAt,
	)
	if err != nil {
		return DomainProfile{}, fmt.Errorf("domain profile: create: %w", err)
	}
	return out, nil
}

// GetByID retrieves a domain profile by UUID. Returns sql.ErrNoRows if not found.
func (r *DomainProfileRepository) GetByID(ctx context.Context, id string) (DomainProfile, error) {
	const q = `
		SELECT id, domain_name, registrar_connection_id, dns_provider_connection_id,
		       status, registration_date, expiry_date, tags, notes, auto_renew, created_by, deleted_at, created_at, updated_at
		FROM domain_profiles WHERE id = $1 AND deleted_at IS NULL`
	return r.scanOne(r.db.QueryRowContext(ctx, q, id))
}

// GetByDomainName retrieves a domain profile by domain name. Returns sql.ErrNoRows if not found.
func (r *DomainProfileRepository) GetByDomainName(ctx context.Context, name string) (DomainProfile, error) {
	const q = `
		SELECT id, domain_name, registrar_connection_id, dns_provider_connection_id,
		       status, registration_date, expiry_date, tags, notes, auto_renew, created_by, deleted_at, created_at, updated_at
		FROM domain_profiles WHERE domain_name = $1 AND deleted_at IS NULL`
	return r.scanOne(r.db.QueryRowContext(ctx, q, name))
}

// List returns domain profiles with optional filtering, sorting, and pagination.
// Returns records, total count (without pagination), and any error.
func (r *DomainProfileRepository) List(ctx context.Context, filters DomainProfileFilters, sort DomainProfileSort, limit, offset int) ([]DomainProfile, int, error) {
	args := []any{}
	argIdx := 1

	whereClause := "WHERE dp.deleted_at IS NULL"

	if filters.Status != "" {
		whereClause += fmt.Sprintf(" AND dp.status = $%d", argIdx)
		args = append(args, filters.Status)
		argIdx++
	}
	if filters.RegistrarConnectionID != "" {
		whereClause += fmt.Sprintf(" AND dp.registrar_connection_id = $%d", argIdx)
		args = append(args, filters.RegistrarConnectionID)
		argIdx++
	}
	if filters.DNSProviderConnectionID != "" {
		whereClause += fmt.Sprintf(" AND dp.dns_provider_connection_id = $%d", argIdx)
		args = append(args, filters.DNSProviderConnectionID)
		argIdx++
	}
	if filters.Tag != "" {
		whereClause += fmt.Sprintf(" AND $%d = ANY(dp.tags)", argIdx)
		args = append(args, filters.Tag)
		argIdx++
	}
	if filters.ExpiryBefore != nil {
		whereClause += fmt.Sprintf(" AND dp.expiry_date <= $%d", argIdx)
		args = append(args, filters.ExpiryBefore)
		argIdx++
	}
	if filters.ExpiryAfter != nil {
		whereClause += fmt.Sprintf(" AND dp.expiry_date >= $%d", argIdx)
		args = append(args, filters.ExpiryAfter)
		argIdx++
	}
	if filters.CampaignID != "" {
		whereClause += fmt.Sprintf(" AND EXISTS (SELECT 1 FROM domain_campaign_associations dca WHERE dca.domain_profile_id = dp.id AND dca.campaign_id = $%d)", argIdx)
		args = append(args, filters.CampaignID)
		argIdx++
	}
	if filters.Search != "" {
		whereClause += fmt.Sprintf(" AND dp.domain_name ILIKE $%d", argIdx)
		args = append(args, "%"+filters.Search+"%")
		argIdx++
	}

	// Count query.
	countQ := "SELECT COUNT(*) FROM domain_profiles dp " + whereClause
	var total int
	if err := r.db.QueryRowContext(ctx, countQ, args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("domain profile: list count: %w", err)
	}

	// Sort.
	sortField := "dp.created_at"
	switch sort.Field {
	case "expiry_date":
		sortField = "dp.expiry_date"
	case "domain_name":
		sortField = "dp.domain_name"
	case "status":
		sortField = "dp.status"
	}
	sortDir := "ASC"
	if sort.Desc {
		sortDir = "DESC"
	}

	// Data query.
	dataArgs := append(args, limit, offset)
	dataQ := fmt.Sprintf(`
		SELECT dp.id, dp.domain_name, dp.registrar_connection_id, dp.dns_provider_connection_id,
		       dp.status, dp.registration_date, dp.expiry_date, dp.tags, dp.notes,
		       dp.created_by, dp.created_at, dp.updated_at
		FROM domain_profiles dp
		%s
		ORDER BY %s %s NULLS LAST
		LIMIT $%d OFFSET $%d`,
		whereClause, sortField, sortDir, argIdx, argIdx+1)

	rows, err := r.db.QueryContext(ctx, dataQ, dataArgs...)
	if err != nil {
		return nil, 0, fmt.Errorf("domain profile: list query: %w", err)
	}
	defer rows.Close()

	var results []DomainProfile
	for rows.Next() {
		var p DomainProfile
		if err := rows.Scan(
			&p.ID, &p.DomainName,
			&p.RegistrarConnectionID, &p.DNSProviderConnectionID,
			&p.Status, &p.RegistrationDate, &p.ExpiryDate,
			pq.Array(&p.Tags), &p.Notes,
			&p.CreatedBy, &p.CreatedAt, &p.UpdatedAt,
		); err != nil {
			return nil, 0, fmt.Errorf("domain profile: list scan: %w", err)
		}
		results = append(results, p)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("domain profile: list rows: %w", err)
	}
	return results, total, nil
}

// DomainProfileUpdate holds mutable fields for a partial update.
type DomainProfileUpdate struct {
	DNSProviderConnectionID *string  // nil = no change; &"" = clear
	Status                  *DomainStatus
	ExpiryDate              *time.Time
	RegistrationDate        *time.Time
	Tags                    *[]string
	Notes                   *string
}

// Update applies the given changes to the profile identified by id.
func (r *DomainProfileRepository) Update(ctx context.Context, id string, updates DomainProfileUpdate) (DomainProfile, error) {
	setClauses := []string{}
	args := []any{}
	argIdx := 1

	if updates.DNSProviderConnectionID != nil {
		if *updates.DNSProviderConnectionID == "" {
			setClauses = append(setClauses, fmt.Sprintf("dns_provider_connection_id = $%d", argIdx))
			args = append(args, nil)
		} else {
			setClauses = append(setClauses, fmt.Sprintf("dns_provider_connection_id = $%d", argIdx))
			args = append(args, *updates.DNSProviderConnectionID)
		}
		argIdx++
	}
	if updates.Status != nil {
		setClauses = append(setClauses, fmt.Sprintf("status = $%d", argIdx))
		args = append(args, string(*updates.Status))
		argIdx++
	}
	if updates.ExpiryDate != nil {
		setClauses = append(setClauses, fmt.Sprintf("expiry_date = $%d", argIdx))
		args = append(args, updates.ExpiryDate)
		argIdx++
	}
	if updates.RegistrationDate != nil {
		setClauses = append(setClauses, fmt.Sprintf("registration_date = $%d", argIdx))
		args = append(args, updates.RegistrationDate)
		argIdx++
	}
	if updates.Tags != nil {
		setClauses = append(setClauses, fmt.Sprintf("tags = $%d", argIdx))
		args = append(args, pq.Array(*updates.Tags))
		argIdx++
	}
	if updates.Notes != nil {
		setClauses = append(setClauses, fmt.Sprintf("notes = $%d", argIdx))
		args = append(args, updates.Notes)
		argIdx++
	}

	if len(setClauses) == 0 {
		return r.GetByID(ctx, id)
	}

	args = append(args, id)
	q := fmt.Sprintf(`
		UPDATE domain_profiles
		SET %s
		WHERE id = $%d AND deleted_at IS NULL
		RETURNING id, domain_name, registrar_connection_id, dns_provider_connection_id,
		          status, registration_date, expiry_date, tags, notes, auto_renew, created_by, deleted_at, created_at, updated_at`,
		strings.Join(setClauses, ", "), argIdx)

	return r.scanOne(r.db.QueryRowContext(ctx, q, args...))
}

// SoftDelete marks the domain profile as deleted.
func (r *DomainProfileRepository) SoftDelete(ctx context.Context, id string) error {
	const q = `UPDATE domain_profiles SET deleted_at = now(), status = 'decommissioned' WHERE id = $1 AND deleted_at IS NULL`
	result, err := r.db.ExecContext(ctx, q, id)
	if err != nil {
		return fmt.Errorf("domain profile: soft delete: %w", err)
	}
	n, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("domain profile: soft delete rows affected: %w", err)
	}
	if n == 0 {
		return fmt.Errorf("domain profile: soft delete: %w", sql.ErrNoRows)
	}
	return nil
}

// CheckCampaignAssociations returns all campaign associations for a domain.
func (r *DomainProfileRepository) CheckCampaignAssociations(ctx context.Context, id string) ([]CampaignAssoc, error) {
	const q = `
		SELECT id, domain_profile_id, campaign_id, association_type
		FROM domain_campaign_associations
		WHERE domain_profile_id = $1`

	rows, err := r.db.QueryContext(ctx, q, id)
	if err != nil {
		return nil, fmt.Errorf("domain profile: check campaign associations: %w", err)
	}
	defer rows.Close()

	var results []CampaignAssoc
	for rows.Next() {
		var a CampaignAssoc
		if err := rows.Scan(&a.ID, &a.DomainProfileID, &a.CampaignID, &a.AssociationType); err != nil {
			return nil, fmt.Errorf("domain profile: campaign associations scan: %w", err)
		}
		results = append(results, a)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("domain profile: campaign associations rows: %w", err)
	}
	return results, nil
}

// GetExpiring returns all active domain profiles with an expiry_date within the next withinDays days.
func (r *DomainProfileRepository) GetExpiring(ctx context.Context, withinDays int) ([]DomainProfile, error) {
	const q = `
		SELECT id, domain_name, registrar_connection_id, dns_provider_connection_id,
		       status, registration_date, expiry_date, tags, notes, auto_renew, created_by, deleted_at, created_at, updated_at
		FROM domain_profiles
		WHERE status = 'active'
		  AND expiry_date IS NOT NULL
		  AND expiry_date <= now() + ($1 || ' days')::interval
		  AND expiry_date >= now()
		ORDER BY expiry_date ASC`

	rows, err := r.db.QueryContext(ctx, q, fmt.Sprintf("%d", withinDays))
	if err != nil {
		return nil, fmt.Errorf("domain profile: get expiring: %w", err)
	}
	defer rows.Close()

	var results []DomainProfile
	for rows.Next() {
		var p DomainProfile
		if err := rows.Scan(
			&p.ID, &p.DomainName,
			&p.RegistrarConnectionID, &p.DNSProviderConnectionID,
			&p.Status, &p.RegistrationDate, &p.ExpiryDate,
			pq.Array(&p.Tags), &p.Notes,
			&p.CreatedBy, &p.CreatedAt, &p.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("domain profile: get expiring scan: %w", err)
		}
		results = append(results, p)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("domain profile: get expiring rows: %w", err)
	}
	return results, nil
}

// GetActiveWithRegistrar returns all active domain profiles that have a registrar connection.
func (r *DomainProfileRepository) GetActiveWithRegistrar(ctx context.Context) ([]DomainProfile, error) {
	const q = `
		SELECT id, domain_name, registrar_connection_id, dns_provider_connection_id,
		       status, registration_date, expiry_date, tags, notes, auto_renew, created_by, deleted_at, created_at, updated_at
		FROM domain_profiles
		WHERE status = 'active'
		  AND registrar_connection_id IS NOT NULL
		ORDER BY domain_name ASC`

	rows, err := r.db.QueryContext(ctx, q)
	if err != nil {
		return nil, fmt.Errorf("domain profile: get active with registrar: %w", err)
	}
	defer rows.Close()

	var results []DomainProfile
	for rows.Next() {
		var p DomainProfile
		if err := rows.Scan(
			&p.ID, &p.DomainName,
			&p.RegistrarConnectionID, &p.DNSProviderConnectionID,
			&p.Status, &p.RegistrationDate, &p.ExpiryDate,
			pq.Array(&p.Tags), &p.Notes,
			&p.CreatedBy, &p.CreatedAt, &p.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("domain profile: get active with registrar scan: %w", err)
		}
		results = append(results, p)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("domain profile: get active with registrar rows: %w", err)
	}
	return results, nil
}

// AddRenewalRecord inserts a renewal history record.
func (r *DomainProfileRepository) AddRenewalRecord(ctx context.Context, rec DomainRenewalRecord) (DomainRenewalRecord, error) {
	id := uuid.New().String()
	const q = `
		INSERT INTO domain_renewal_history
			(id, domain_profile_id, renewal_date, duration_years, cost_amount, cost_currency,
			 registrar_connection_id, initiated_by)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8)
		RETURNING id, domain_profile_id, renewal_date, duration_years, cost_amount, cost_currency,
		          registrar_connection_id, initiated_by, created_at`

	var out DomainRenewalRecord
	err := r.db.QueryRowContext(ctx, q,
		id,
		rec.DomainProfileID,
		rec.RenewalDate,
		rec.DurationYears,
		rec.CostAmount,
		rec.CostCurrency,
		rec.RegistrarConnectionID,
		rec.InitiatedBy,
	).Scan(
		&out.ID,
		&out.DomainProfileID,
		&out.RenewalDate,
		&out.DurationYears,
		&out.CostAmount,
		&out.CostCurrency,
		&out.RegistrarConnectionID,
		&out.InitiatedBy,
		&out.CreatedAt,
	)
	if err != nil {
		return DomainRenewalRecord{}, fmt.Errorf("domain profile: add renewal record: %w", err)
	}
	return out, nil
}

// ListRenewalHistory returns renewal records for a domain profile ordered by date descending.
func (r *DomainProfileRepository) ListRenewalHistory(ctx context.Context, domainProfileID string) ([]DomainRenewalRecord, error) {
	const q = `
		SELECT id, domain_profile_id, renewal_date, duration_years, cost_amount, cost_currency,
		       registrar_connection_id, initiated_by, created_at
		FROM domain_renewal_history
		WHERE domain_profile_id = $1
		ORDER BY renewal_date DESC`

	rows, err := r.db.QueryContext(ctx, q, domainProfileID)
	if err != nil {
		return nil, fmt.Errorf("domain profile: list renewal history: %w", err)
	}
	defer rows.Close()

	var results []DomainRenewalRecord
	for rows.Next() {
		var rec DomainRenewalRecord
		if err := rows.Scan(
			&rec.ID, &rec.DomainProfileID,
			&rec.RenewalDate, &rec.DurationYears,
			&rec.CostAmount, &rec.CostCurrency,
			&rec.RegistrarConnectionID, &rec.InitiatedBy,
			&rec.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("domain profile: renewal history scan: %w", err)
		}
		results = append(results, rec)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("domain profile: renewal history rows: %w", err)
	}
	return results, nil
}

// RegistrationRequest is the DB model for a domain registration request.
type RegistrationRequest struct {
	ID                     string
	DomainName             string
	RegistrarConnectionID  string
	Years                  int
	RegistrantInfo         []byte // JSONB
	Status                 string
	RequestedBy            string
	ReviewedBy             *string
	ReviewedAt             *time.Time
	RejectionReason        *string
	DomainProfileID        *string
	CreatedAt              time.Time
	UpdatedAt              time.Time
}

// CreateRegistrationRequest inserts a new pending registration request.
func (r *DomainProfileRepository) CreateRegistrationRequest(ctx context.Context, req RegistrationRequest) (RegistrationRequest, error) {
	id := uuid.New().String()
	const q = `
		INSERT INTO domain_registration_requests
			(id, domain_name, registrar_connection_id, years, registrant_info, requested_by)
		VALUES ($1,$2,$3,$4,$5,$6)
		RETURNING id, domain_name, registrar_connection_id, years, registrant_info,
		          status, requested_by, reviewed_by, reviewed_at, rejection_reason,
		          domain_profile_id, created_at, updated_at`

	var out RegistrationRequest
	err := r.db.QueryRowContext(ctx, q,
		id,
		req.DomainName,
		req.RegistrarConnectionID,
		req.Years,
		req.RegistrantInfo,
		req.RequestedBy,
	).Scan(
		&out.ID, &out.DomainName, &out.RegistrarConnectionID, &out.Years, &out.RegistrantInfo,
		&out.Status, &out.RequestedBy, &out.ReviewedBy, &out.ReviewedAt, &out.RejectionReason,
		&out.DomainProfileID, &out.CreatedAt, &out.UpdatedAt,
	)
	if err != nil {
		return RegistrationRequest{}, fmt.Errorf("domain profile: create registration request: %w", err)
	}
	return out, nil
}

// GetRegistrationRequest retrieves a registration request by ID.
func (r *DomainProfileRepository) GetRegistrationRequest(ctx context.Context, id string) (RegistrationRequest, error) {
	const q = `
		SELECT id, domain_name, registrar_connection_id, years, registrant_info,
		       status, requested_by, reviewed_by, reviewed_at, rejection_reason,
		       domain_profile_id, created_at, updated_at
		FROM domain_registration_requests
		WHERE id = $1`

	var out RegistrationRequest
	err := r.db.QueryRowContext(ctx, q, id).Scan(
		&out.ID, &out.DomainName, &out.RegistrarConnectionID, &out.Years, &out.RegistrantInfo,
		&out.Status, &out.RequestedBy, &out.ReviewedBy, &out.ReviewedAt, &out.RejectionReason,
		&out.DomainProfileID, &out.CreatedAt, &out.UpdatedAt,
	)
	if err != nil {
		return RegistrationRequest{}, fmt.Errorf("domain profile: get registration request: %w", err)
	}
	return out, nil
}

// UpdateRegistrationRequest updates status, reviewer, and outcome fields.
func (r *DomainProfileRepository) UpdateRegistrationRequest(ctx context.Context, id string, status, reviewerID string, rejectionReason *string, domainProfileID *string) error {
	const q = `
		UPDATE domain_registration_requests
		SET status = $1, reviewed_by = $2, reviewed_at = now(),
		    rejection_reason = $3, domain_profile_id = $4
		WHERE id = $5`
	result, err := r.db.ExecContext(ctx, q, status, reviewerID, rejectionReason, domainProfileID, id)
	if err != nil {
		return fmt.Errorf("domain profile: update registration request: %w", err)
	}
	n, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("domain profile: update registration request rows: %w", err)
	}
	if n == 0 {
		return fmt.Errorf("domain profile: update registration request: %w", sql.ErrNoRows)
	}
	return nil
}

// scanOne scans a single domain profile row.
func (r *DomainProfileRepository) scanOne(row *sql.Row) (DomainProfile, error) {
	var p DomainProfile
	err := row.Scan(
		&p.ID, &p.DomainName,
		&p.RegistrarConnectionID, &p.DNSProviderConnectionID,
		&p.Status, &p.RegistrationDate, &p.ExpiryDate,
		pq.Array(&p.Tags), &p.Notes, &p.AutoRenew,
		&p.CreatedBy, &p.DeletedAt,
		&p.CreatedAt, &p.UpdatedAt,
	)
	if err != nil {
		return DomainProfile{}, fmt.Errorf("domain profile: scan: %w", err)
	}
	return p, nil
}
