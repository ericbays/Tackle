package repositories

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
)

// DNSRecord is the DB model for a cached DNS record.
type DNSRecord struct {
	ID               string
	DomainProfileID  string
	ProviderRecordID *string
	RecordType       string
	RecordName       string
	RecordValue      string
	TTL              int
	Priority         int
	ManagedBySystem  bool
	SyncedAt         *time.Time
	CreatedAt        time.Time
	UpdatedAt        time.Time
}

// PropagationCheckResult holds per-resolver data for a propagation check.
type PropagationCheckResult struct {
	Resolver string `json:"resolver"`
	Response string `json:"response"`
	Matches  bool   `json:"matches"`
	LatencyMs int64 `json:"latency_ms"`
}

// PropagationCheck is the DB model for a dns_propagation_checks row.
type PropagationCheck struct {
	ID              string
	DomainProfileID string
	RecordType      string
	RecordName      string
	ExpectedValue   string
	OverallStatus   string // propagated | partial | not_propagated
	Results         []PropagationCheckResult
	CheckedAt       time.Time
	CreatedAt       time.Time
}

// DKIMKey is the DB model for a dkim_keys row.
type DKIMKey struct {
	ID                  string
	DomainProfileID     string
	Selector            string
	Algorithm           string
	KeySize             *int
	PrivateKeyEncrypted []byte
	PublicKey           string
	RotatedAt           *time.Time
	CreatedAt           time.Time
	UpdatedAt           time.Time
}

// EmailAuthStatus is the DB model for a domain_email_auth_status row.
type EmailAuthStatus struct {
	ID              string
	DomainProfileID string
	SPFStatus       string // configured | misconfigured | missing
	DKIMStatus      string // configured | misconfigured | missing
	DMARCStatus     string // configured | misconfigured | missing
	LastCheckedAt   *time.Time
	DetailsJSON     []byte
	CreatedAt       time.Time
	UpdatedAt       time.Time
}

// DNSRecordRepository provides database operations for DNS records, propagation
// checks, DKIM keys, and email auth status.
type DNSRecordRepository struct {
	db *sql.DB
}

// NewDNSRecordRepository creates a new DNSRecordRepository.
func NewDNSRecordRepository(db *sql.DB) *DNSRecordRepository {
	return &DNSRecordRepository{db: db}
}

// --- DNS record cache ---

// UpsertRecord inserts or updates a cached DNS record (match by domain+type+name).
func (r *DNSRecordRepository) UpsertRecord(ctx context.Context, rec DNSRecord) (DNSRecord, error) {
	const q = `
		INSERT INTO dns_records
			(id, domain_profile_id, provider_record_id, record_type, record_name,
			 record_value, ttl, priority, managed_by_system, synced_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10)
		ON CONFLICT (domain_profile_id, record_type, record_name)
		DO UPDATE SET
			provider_record_id = EXCLUDED.provider_record_id,
			record_value       = EXCLUDED.record_value,
			ttl                = EXCLUDED.ttl,
			priority           = EXCLUDED.priority,
			managed_by_system  = EXCLUDED.managed_by_system,
			synced_at          = EXCLUDED.synced_at,
			updated_at         = now()
		RETURNING id, domain_profile_id, provider_record_id, record_type, record_name,
		          record_value, ttl, priority, managed_by_system, synced_at, created_at, updated_at`

	id := uuid.New().String()
	var out DNSRecord
	err := r.db.QueryRowContext(ctx, q,
		id, rec.DomainProfileID, rec.ProviderRecordID, rec.RecordType, rec.RecordName,
		rec.RecordValue, rec.TTL, rec.Priority, rec.ManagedBySystem, rec.SyncedAt,
	).Scan(
		&out.ID, &out.DomainProfileID, &out.ProviderRecordID,
		&out.RecordType, &out.RecordName, &out.RecordValue,
		&out.TTL, &out.Priority, &out.ManagedBySystem, &out.SyncedAt, &out.CreatedAt, &out.UpdatedAt,
	)
	if err != nil {
		return DNSRecord{}, fmt.Errorf("dns records: upsert: %w", err)
	}
	return out, nil
}

// ListRecords returns all cached DNS records for a domain profile.
func (r *DNSRecordRepository) ListRecords(ctx context.Context, domainProfileID string) ([]DNSRecord, error) {
	const q = `
		SELECT id, domain_profile_id, provider_record_id, record_type, record_name,
		       record_value, ttl, priority, managed_by_system, synced_at, created_at, updated_at
		FROM dns_records
		WHERE domain_profile_id = $1
		ORDER BY record_type, record_name`

	rows, err := r.db.QueryContext(ctx, q, domainProfileID)
	if err != nil {
		return nil, fmt.Errorf("dns records: list: %w", err)
	}
	defer rows.Close()

	return scanDNSRecords(rows)
}

// DeleteRecord removes a cached DNS record by ID.
func (r *DNSRecordRepository) DeleteRecord(ctx context.Context, id string) error {
	const q = `DELETE FROM dns_records WHERE id = $1`
	result, err := r.db.ExecContext(ctx, q, id)
	if err != nil {
		return fmt.Errorf("dns records: delete: %w", err)
	}
	n, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("dns records: delete rows: %w", err)
	}
	if n == 0 {
		return fmt.Errorf("dns records: delete: %w", sql.ErrNoRows)
	}
	return nil
}

// DeleteAllRecords removes all cached DNS records for a domain profile.
func (r *DNSRecordRepository) DeleteAllRecords(ctx context.Context, domainProfileID string) error {
	const q = `DELETE FROM dns_records WHERE domain_profile_id = $1`
	_, err := r.db.ExecContext(ctx, q, domainProfileID)
	if err != nil {
		return fmt.Errorf("dns records: delete all: %w", err)
	}
	return nil
}

// scanDNSRecords scans rows from dns_records into a slice.
func scanDNSRecords(rows *sql.Rows) ([]DNSRecord, error) {
	var out []DNSRecord
	for rows.Next() {
		var rec DNSRecord
		if err := rows.Scan(
			&rec.ID, &rec.DomainProfileID, &rec.ProviderRecordID,
			&rec.RecordType, &rec.RecordName, &rec.RecordValue,
			&rec.TTL, &rec.Priority, &rec.ManagedBySystem, &rec.SyncedAt,
			&rec.CreatedAt, &rec.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("dns records: scan: %w", err)
		}
		out = append(out, rec)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("dns records: rows: %w", err)
	}
	return out, nil
}

// --- Propagation checks ---

// CreatePropagationCheck inserts a propagation check result.
func (r *DNSRecordRepository) CreatePropagationCheck(ctx context.Context, check PropagationCheck) (PropagationCheck, error) {
	resultsJSON, err := json.Marshal(check.Results)
	if err != nil {
		return PropagationCheck{}, fmt.Errorf("dns propagation: marshal results: %w", err)
	}

	id := uuid.New().String()
	const q = `
		INSERT INTO dns_propagation_checks
			(id, domain_profile_id, record_type, record_name, expected_value,
			 overall_status, results_json, checked_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8)
		RETURNING id, domain_profile_id, record_type, record_name, expected_value,
		          overall_status, results_json, checked_at, created_at`

	var raw []byte
	var out PropagationCheck
	err = r.db.QueryRowContext(ctx, q,
		id, check.DomainProfileID, check.RecordType, check.RecordName,
		check.ExpectedValue, check.OverallStatus, resultsJSON, check.CheckedAt,
	).Scan(
		&out.ID, &out.DomainProfileID, &out.RecordType, &out.RecordName,
		&out.ExpectedValue, &out.OverallStatus, &raw, &out.CheckedAt, &out.CreatedAt,
	)
	if err != nil {
		return PropagationCheck{}, fmt.Errorf("dns propagation: create: %w", err)
	}
	if err := json.Unmarshal(raw, &out.Results); err != nil {
		return PropagationCheck{}, fmt.Errorf("dns propagation: parse results: %w", err)
	}
	return out, nil
}

// ListPropagationChecks returns recent propagation checks for a domain profile.
func (r *DNSRecordRepository) ListPropagationChecks(ctx context.Context, domainProfileID string, limit int) ([]PropagationCheck, error) {
	if limit <= 0 {
		limit = 20
	}
	const q = `
		SELECT id, domain_profile_id, record_type, record_name, expected_value,
		       overall_status, results_json, checked_at, created_at
		FROM dns_propagation_checks
		WHERE domain_profile_id = $1
		ORDER BY checked_at DESC
		LIMIT $2`

	rows, err := r.db.QueryContext(ctx, q, domainProfileID, limit)
	if err != nil {
		return nil, fmt.Errorf("dns propagation: list: %w", err)
	}
	defer rows.Close()

	var out []PropagationCheck
	for rows.Next() {
		var check PropagationCheck
		var raw []byte
		if err := rows.Scan(
			&check.ID, &check.DomainProfileID, &check.RecordType, &check.RecordName,
			&check.ExpectedValue, &check.OverallStatus, &raw, &check.CheckedAt, &check.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("dns propagation: scan: %w", err)
		}
		if err := json.Unmarshal(raw, &check.Results); err != nil {
			return nil, fmt.Errorf("dns propagation: parse results: %w", err)
		}
		out = append(out, check)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("dns propagation: rows: %w", err)
	}
	return out, nil
}

// --- DKIM keys ---

// CreateDKIMKey inserts a new DKIM key record.
func (r *DNSRecordRepository) CreateDKIMKey(ctx context.Context, key DKIMKey) (DKIMKey, error) {
	id := uuid.New().String()
	const q = `
		INSERT INTO dkim_keys
			(id, domain_profile_id, selector, algorithm, key_size,
			 private_key_encrypted, public_key)
		VALUES ($1,$2,$3,$4,$5,$6,$7)
		RETURNING id, domain_profile_id, selector, algorithm, key_size,
		          private_key_encrypted, public_key, rotated_at, created_at, updated_at`

	var out DKIMKey
	err := r.db.QueryRowContext(ctx, q,
		id, key.DomainProfileID, key.Selector, key.Algorithm, key.KeySize,
		key.PrivateKeyEncrypted, key.PublicKey,
	).Scan(
		&out.ID, &out.DomainProfileID, &out.Selector, &out.Algorithm, &out.KeySize,
		&out.PrivateKeyEncrypted, &out.PublicKey, &out.RotatedAt,
		&out.CreatedAt, &out.UpdatedAt,
	)
	if err != nil {
		return DKIMKey{}, fmt.Errorf("dkim keys: create: %w", err)
	}
	return out, nil
}

// ListDKIMKeys returns all DKIM keys for a domain profile.
func (r *DNSRecordRepository) ListDKIMKeys(ctx context.Context, domainProfileID string) ([]DKIMKey, error) {
	const q = `
		SELECT id, domain_profile_id, selector, algorithm, key_size,
		       private_key_encrypted, public_key, rotated_at, created_at, updated_at
		FROM dkim_keys
		WHERE domain_profile_id = $1
		ORDER BY created_at DESC`

	rows, err := r.db.QueryContext(ctx, q, domainProfileID)
	if err != nil {
		return nil, fmt.Errorf("dkim keys: list: %w", err)
	}
	defer rows.Close()

	var out []DKIMKey
	for rows.Next() {
		var k DKIMKey
		if err := rows.Scan(
			&k.ID, &k.DomainProfileID, &k.Selector, &k.Algorithm, &k.KeySize,
			&k.PrivateKeyEncrypted, &k.PublicKey, &k.RotatedAt,
			&k.CreatedAt, &k.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("dkim keys: scan: %w", err)
		}
		out = append(out, k)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("dkim keys: rows: %w", err)
	}
	return out, nil
}

// GetDKIMKeyBySelector retrieves a DKIM key by domain profile + selector.
func (r *DNSRecordRepository) GetDKIMKeyBySelector(ctx context.Context, domainProfileID, selector string) (DKIMKey, error) {
	const q = `
		SELECT id, domain_profile_id, selector, algorithm, key_size,
		       private_key_encrypted, public_key, rotated_at, created_at, updated_at
		FROM dkim_keys
		WHERE domain_profile_id = $1 AND selector = $2`

	var k DKIMKey
	err := r.db.QueryRowContext(ctx, q, domainProfileID, selector).Scan(
		&k.ID, &k.DomainProfileID, &k.Selector, &k.Algorithm, &k.KeySize,
		&k.PrivateKeyEncrypted, &k.PublicKey, &k.RotatedAt,
		&k.CreatedAt, &k.UpdatedAt,
	)
	if err != nil {
		return DKIMKey{}, fmt.Errorf("dkim keys: get by selector: %w", err)
	}
	return k, nil
}

// --- Email auth status ---

// UpsertEmailAuthStatus inserts or updates the email auth status for a domain.
func (r *DNSRecordRepository) UpsertEmailAuthStatus(ctx context.Context, status EmailAuthStatus) (EmailAuthStatus, error) {
	detailsJSON := status.DetailsJSON
	if detailsJSON == nil {
		detailsJSON = []byte("{}")
	}

	id := uuid.New().String()
	const q = `
		INSERT INTO domain_email_auth_status
			(id, domain_profile_id, spf_status, dkim_status, dmarc_status,
			 last_checked_at, details_json)
		VALUES ($1,$2,$3,$4,$5,$6,$7)
		ON CONFLICT (domain_profile_id) DO UPDATE SET
			spf_status      = EXCLUDED.spf_status,
			dkim_status     = EXCLUDED.dkim_status,
			dmarc_status    = EXCLUDED.dmarc_status,
			last_checked_at = EXCLUDED.last_checked_at,
			details_json    = EXCLUDED.details_json,
			updated_at      = now()
		RETURNING id, domain_profile_id, spf_status, dkim_status, dmarc_status,
		          last_checked_at, details_json, created_at, updated_at`

	var out EmailAuthStatus
	err := r.db.QueryRowContext(ctx, q,
		id, status.DomainProfileID, status.SPFStatus, status.DKIMStatus, status.DMARCStatus,
		status.LastCheckedAt, detailsJSON,
	).Scan(
		&out.ID, &out.DomainProfileID, &out.SPFStatus, &out.DKIMStatus, &out.DMARCStatus,
		&out.LastCheckedAt, &out.DetailsJSON, &out.CreatedAt, &out.UpdatedAt,
	)
	if err != nil {
		return EmailAuthStatus{}, fmt.Errorf("email auth status: upsert: %w", err)
	}
	return out, nil
}

// GetEmailAuthStatus retrieves the email auth status for a domain profile.
func (r *DNSRecordRepository) GetEmailAuthStatus(ctx context.Context, domainProfileID string) (EmailAuthStatus, error) {
	const q = `
		SELECT id, domain_profile_id, spf_status, dkim_status, dmarc_status,
		       last_checked_at, details_json, created_at, updated_at
		FROM domain_email_auth_status
		WHERE domain_profile_id = $1`

	var out EmailAuthStatus
	err := r.db.QueryRowContext(ctx, q, domainProfileID).Scan(
		&out.ID, &out.DomainProfileID, &out.SPFStatus, &out.DKIMStatus, &out.DMARCStatus,
		&out.LastCheckedAt, &out.DetailsJSON, &out.CreatedAt, &out.UpdatedAt,
	)
	if err != nil {
		return EmailAuthStatus{}, fmt.Errorf("email auth status: get: %w", err)
	}
	return out, nil
}
