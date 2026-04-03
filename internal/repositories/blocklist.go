package repositories

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
)

// BlocklistEntry is the DB model for a blocklist_entries row.
type BlocklistEntry struct {
	ID       string
	Pattern  string
	Reason   string
	IsActive bool
	AddedBy  string
	AddedAt  time.Time
}

// BlocklistFilters controls optional filtering for block list queries.
type BlocklistFilters struct {
	Pattern  string // partial, case-insensitive
	Reason   string // partial, case-insensitive
	IsActive *bool  // nil = all, true = active only, false = inactive only
	Page     int
	PerPage  int
}

// BlocklistOverride is the DB model for a blocklist_overrides row.
type BlocklistOverride struct {
	ID              string
	CampaignID      string
	Status          string // pending, approved, rejected
	BlockedTargets  []BlockedTargetInfo
	TargetHash      string
	Acknowledgment  bool
	Justification   *string
	RejectionReason *string
	DecidedBy       *string
	DecidedAt       *time.Time
	CreatedAt       time.Time
}

// BlockedTargetInfo stores info about a blocked target in an override request.
type BlockedTargetInfo struct {
	TargetID string `json:"target_id"`
	Email    string `json:"email"`
	Pattern  string `json:"pattern"`
	Reason   string `json:"reason"`
}

// BlocklistRepository provides database operations for blocklist_entries.
type BlocklistRepository struct {
	db *sql.DB
}

// NewBlocklistRepository creates a new BlocklistRepository.
func NewBlocklistRepository(db *sql.DB) *BlocklistRepository {
	return &BlocklistRepository{db: db}
}

// Create inserts a new block list entry.
func (r *BlocklistRepository) Create(ctx context.Context, e BlocklistEntry) (BlocklistEntry, error) {
	id := uuid.New().String()

	const q = `
		INSERT INTO blocklist_entries (id, pattern, reason, is_active, added_by)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING id, pattern, reason, is_active, added_by, added_at`

	var out BlocklistEntry
	err := r.db.QueryRowContext(ctx, q, id, e.Pattern, e.Reason, true, e.AddedBy).Scan(
		&out.ID, &out.Pattern, &out.Reason, &out.IsActive, &out.AddedBy, &out.AddedAt,
	)
	if err != nil {
		return BlocklistEntry{}, fmt.Errorf("blocklist: create: %w", err)
	}
	return out, nil
}

// GetByID retrieves a block list entry by ID.
func (r *BlocklistRepository) GetByID(ctx context.Context, id string) (BlocklistEntry, error) {
	const q = `
		SELECT id, pattern, reason, is_active, added_by, added_at
		FROM blocklist_entries WHERE id = $1`

	var out BlocklistEntry
	err := r.db.QueryRowContext(ctx, q, id).Scan(
		&out.ID, &out.Pattern, &out.Reason, &out.IsActive, &out.AddedBy, &out.AddedAt,
	)
	if err != nil {
		return BlocklistEntry{}, fmt.Errorf("blocklist: get: %w", err)
	}
	return out, nil
}

// Deactivate sets is_active = false for an entry.
func (r *BlocklistRepository) Deactivate(ctx context.Context, id string) (BlocklistEntry, error) {
	const q = `
		UPDATE blocklist_entries SET is_active = false
		WHERE id = $1
		RETURNING id, pattern, reason, is_active, added_by, added_at`

	var out BlocklistEntry
	err := r.db.QueryRowContext(ctx, q, id).Scan(
		&out.ID, &out.Pattern, &out.Reason, &out.IsActive, &out.AddedBy, &out.AddedAt,
	)
	if err != nil {
		return BlocklistEntry{}, fmt.Errorf("blocklist: deactivate: %w", err)
	}
	return out, nil
}

// Reactivate sets is_active = true for an entry.
func (r *BlocklistRepository) Reactivate(ctx context.Context, id string) (BlocklistEntry, error) {
	const q = `
		UPDATE blocklist_entries SET is_active = true
		WHERE id = $1
		RETURNING id, pattern, reason, is_active, added_by, added_at`

	var out BlocklistEntry
	err := r.db.QueryRowContext(ctx, q, id).Scan(
		&out.ID, &out.Pattern, &out.Reason, &out.IsActive, &out.AddedBy, &out.AddedAt,
	)
	if err != nil {
		return BlocklistEntry{}, fmt.Errorf("blocklist: reactivate: %w", err)
	}
	return out, nil
}

// List returns block list entries filtered and paginated.
func (r *BlocklistRepository) List(ctx context.Context, f BlocklistFilters) ([]BlocklistEntry, int, error) {
	where := []string{}
	args := []any{}
	argIdx := 1

	if f.Pattern != "" {
		where = append(where, fmt.Sprintf("LOWER(pattern) LIKE LOWER($%d)", argIdx))
		args = append(args, "%"+f.Pattern+"%")
		argIdx++
	}
	if f.Reason != "" {
		where = append(where, fmt.Sprintf("LOWER(reason) LIKE LOWER($%d)", argIdx))
		args = append(args, "%"+f.Reason+"%")
		argIdx++
	}
	if f.IsActive != nil {
		where = append(where, fmt.Sprintf("is_active = $%d", argIdx))
		args = append(args, *f.IsActive)
		argIdx++
	}

	whereClause := ""
	if len(where) > 0 {
		whereClause = "WHERE " + strings.Join(where, " AND ")
	}

	var total int
	countQ := fmt.Sprintf(`SELECT COUNT(*) FROM blocklist_entries %s`, whereClause)
	if err := r.db.QueryRowContext(ctx, countQ, args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("blocklist: list count: %w", err)
	}

	page := f.Page
	if page < 1 {
		page = 1
	}
	perPage := f.PerPage
	if perPage < 1 || perPage > 100 {
		perPage = 25
	}
	offset := (page - 1) * perPage

	args = append(args, perPage, offset)
	q := fmt.Sprintf(`
		SELECT id, pattern, reason, is_active, added_by, added_at
		FROM blocklist_entries %s
		ORDER BY added_at DESC
		LIMIT $%d OFFSET $%d`, whereClause, argIdx, argIdx+1)

	rows, err := r.db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("blocklist: list: %w", err)
	}
	defer rows.Close()

	var entries []BlocklistEntry
	for rows.Next() {
		var e BlocklistEntry
		if err := rows.Scan(&e.ID, &e.Pattern, &e.Reason, &e.IsActive, &e.AddedBy, &e.AddedAt); err != nil {
			return nil, 0, fmt.Errorf("blocklist: list scan: %w", err)
		}
		entries = append(entries, e)
	}
	return entries, total, rows.Err()
}

// ListActive returns all active block list entries (for caching/matching).
func (r *BlocklistRepository) ListActive(ctx context.Context) ([]BlocklistEntry, error) {
	const q = `
		SELECT id, pattern, reason, is_active, added_by, added_at
		FROM blocklist_entries WHERE is_active = true
		ORDER BY pattern ASC`

	rows, err := r.db.QueryContext(ctx, q)
	if err != nil {
		return nil, fmt.Errorf("blocklist: list_active: %w", err)
	}
	defer rows.Close()

	var entries []BlocklistEntry
	for rows.Next() {
		var e BlocklistEntry
		if err := rows.Scan(&e.ID, &e.Pattern, &e.Reason, &e.IsActive, &e.AddedBy, &e.AddedAt); err != nil {
			return nil, fmt.Errorf("blocklist: list_active scan: %w", err)
		}
		entries = append(entries, e)
	}
	return entries, rows.Err()
}

// GetByPattern retrieves a block list entry by pattern (case-insensitive).
func (r *BlocklistRepository) GetByPattern(ctx context.Context, pattern string) (BlocklistEntry, error) {
	const q = `
		SELECT id, pattern, reason, is_active, added_by, added_at
		FROM blocklist_entries WHERE LOWER(pattern) = LOWER($1)`

	var out BlocklistEntry
	err := r.db.QueryRowContext(ctx, q, pattern).Scan(
		&out.ID, &out.Pattern, &out.Reason, &out.IsActive, &out.AddedBy, &out.AddedAt,
	)
	if err != nil {
		return BlocklistEntry{}, fmt.Errorf("blocklist: get_by_pattern: %w", err)
	}
	return out, nil
}

// --- Blocklist Override Repository ---

// BlocklistOverrideRepository provides database operations for blocklist_overrides.
type BlocklistOverrideRepository struct {
	db *sql.DB
}

// NewBlocklistOverrideRepository creates a new BlocklistOverrideRepository.
func NewBlocklistOverrideRepository(db *sql.DB) *BlocklistOverrideRepository {
	return &BlocklistOverrideRepository{db: db}
}

// Create inserts a new override request.
func (r *BlocklistOverrideRepository) Create(ctx context.Context, o BlocklistOverride) (BlocklistOverride, error) {
	id := uuid.New().String()

	btJSON, err := json.Marshal(o.BlockedTargets)
	if err != nil {
		return BlocklistOverride{}, fmt.Errorf("blocklist_overrides: marshal: %w", err)
	}

	const q = `
		INSERT INTO blocklist_overrides (id, campaign_id, status, blocked_targets, target_hash)
		VALUES ($1, $2, 'pending', $3, $4)
		RETURNING id, campaign_id, status, blocked_targets, target_hash, acknowledgment,
		          justification, rejection_reason, decided_by, decided_at, created_at`

	return r.scanOne(r.db.QueryRowContext(ctx, q, id, o.CampaignID, btJSON, o.TargetHash))
}

// GetByCampaignID returns the most recent override for a campaign.
func (r *BlocklistOverrideRepository) GetByCampaignID(ctx context.Context, campaignID string) (BlocklistOverride, error) {
	const q = `
		SELECT id, campaign_id, status, blocked_targets, target_hash, acknowledgment,
		       justification, rejection_reason, decided_by, decided_at, created_at
		FROM blocklist_overrides
		WHERE campaign_id = $1
		ORDER BY created_at DESC
		LIMIT 1`

	return r.scanOne(r.db.QueryRowContext(ctx, q, campaignID))
}

// GetByID returns an override by ID.
func (r *BlocklistOverrideRepository) GetByID(ctx context.Context, id string) (BlocklistOverride, error) {
	const q = `
		SELECT id, campaign_id, status, blocked_targets, target_hash, acknowledgment,
		       justification, rejection_reason, decided_by, decided_at, created_at
		FROM blocklist_overrides WHERE id = $1`

	return r.scanOne(r.db.QueryRowContext(ctx, q, id))
}

// Approve marks an override as approved with justification.
func (r *BlocklistOverrideRepository) Approve(ctx context.Context, id, decidedBy, justification string) (BlocklistOverride, error) {
	const q = `
		UPDATE blocklist_overrides
		SET status = 'approved', acknowledgment = true, justification = $2,
		    decided_by = $3, decided_at = now()
		WHERE id = $1
		RETURNING id, campaign_id, status, blocked_targets, target_hash, acknowledgment,
		          justification, rejection_reason, decided_by, decided_at, created_at`

	return r.scanOne(r.db.QueryRowContext(ctx, q, id, justification, decidedBy))
}

// Reject marks an override as rejected.
func (r *BlocklistOverrideRepository) Reject(ctx context.Context, id, decidedBy string, reason *string) (BlocklistOverride, error) {
	const q = `
		UPDATE blocklist_overrides
		SET status = 'rejected', rejection_reason = $2, decided_by = $3, decided_at = now()
		WHERE id = $1
		RETURNING id, campaign_id, status, blocked_targets, target_hash, acknowledgment,
		          justification, rejection_reason, decided_by, decided_at, created_at`

	return r.scanOne(r.db.QueryRowContext(ctx, q, id, reason, decidedBy))
}

// ListPending returns all pending override requests.
func (r *BlocklistOverrideRepository) ListPending(ctx context.Context) ([]BlocklistOverride, error) {
	const q = `
		SELECT id, campaign_id, status, blocked_targets, target_hash, acknowledgment,
		       justification, rejection_reason, decided_by, decided_at, created_at
		FROM blocklist_overrides
		WHERE status = 'pending'
		ORDER BY created_at DESC`

	rows, err := r.db.QueryContext(ctx, q)
	if err != nil {
		return nil, fmt.Errorf("blocklist_overrides: list pending: %w", err)
	}
	defer rows.Close()

	var results []BlocklistOverride
	for rows.Next() {
		var o BlocklistOverride
		var btJSON []byte
		if err := rows.Scan(
			&o.ID, &o.CampaignID, &o.Status, &btJSON, &o.TargetHash,
			&o.Acknowledgment, &o.Justification, &o.RejectionReason,
			&o.DecidedBy, &o.DecidedAt, &o.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("blocklist_overrides: list pending scan: %w", err)
		}
		if btJSON != nil {
			_ = json.Unmarshal(btJSON, &o.BlockedTargets)
		}
		results = append(results, o)
	}
	return results, rows.Err()
}

// ListAll returns all override requests ordered by date descending.
func (r *BlocklistOverrideRepository) ListAll(ctx context.Context) ([]BlocklistOverride, error) {
	const q = `
		SELECT id, campaign_id, status, blocked_targets, target_hash, acknowledgment,
		       justification, rejection_reason, decided_by, decided_at, created_at
		FROM blocklist_overrides
		ORDER BY created_at DESC`

	rows, err := r.db.QueryContext(ctx, q)
	if err != nil {
		return nil, fmt.Errorf("blocklist_overrides: list all: %w", err)
	}
	defer rows.Close()

	var results []BlocklistOverride
	for rows.Next() {
		var o BlocklistOverride
		var btJSON []byte
		if err := rows.Scan(
			&o.ID, &o.CampaignID, &o.Status, &btJSON, &o.TargetHash,
			&o.Acknowledgment, &o.Justification, &o.RejectionReason,
			&o.DecidedBy, &o.DecidedAt, &o.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("blocklist_overrides: list all scan: %w", err)
		}
		if btJSON != nil {
			_ = json.Unmarshal(btJSON, &o.BlockedTargets)
		}
		results = append(results, o)
	}
	return results, rows.Err()
}

// InvalidatePending invalidates all pending overrides for a campaign (used when blocked set changes).
func (r *BlocklistOverrideRepository) InvalidatePending(ctx context.Context, campaignID string) error {
	const q = `
		UPDATE blocklist_overrides
		SET status = 'rejected', rejection_reason = 'Invalidated: blocked target set changed'
		WHERE campaign_id = $1 AND status = 'pending'`
	_, err := r.db.ExecContext(ctx, q, campaignID)
	if err != nil {
		return fmt.Errorf("blocklist_overrides: invalidate: %w", err)
	}
	return nil
}

func (r *BlocklistOverrideRepository) scanOne(row *sql.Row) (BlocklistOverride, error) {
	var o BlocklistOverride
	var btJSON []byte
	err := row.Scan(
		&o.ID, &o.CampaignID, &o.Status, &btJSON, &o.TargetHash,
		&o.Acknowledgment, &o.Justification, &o.RejectionReason,
		&o.DecidedBy, &o.DecidedAt, &o.CreatedAt,
	)
	if err != nil {
		return BlocklistOverride{}, fmt.Errorf("blocklist_overrides: scan: %w", err)
	}
	if btJSON != nil {
		_ = json.Unmarshal(btJSON, &o.BlockedTargets)
	}
	return o, nil
}
