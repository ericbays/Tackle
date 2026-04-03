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

// Target is the DB model for a targets row.
type Target struct {
	ID           string
	Email        string
	FirstName    *string
	LastName     *string
	Department   *string
	Title        *string
	CustomFields map[string]any
	CreatedBy    string
	DeletedAt    *time.Time
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

// CampaignTarget is the DB model for a campaign_targets row.
type CampaignTarget struct {
	ID          string
	CampaignID  string
	TargetID    string
	Status      string
	Reported    bool
	AssignedAt  time.Time
	AssignedBy  string
	RemovedAt   *time.Time
	SentAt      *time.Time
	OpenedAt    *time.Time
	ClickedAt   *time.Time
	SubmittedAt *time.Time
	ReportedAt  *time.Time
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

// CampaignTargetEvent is the DB model for a campaign_target_events row.
type CampaignTargetEvent struct {
	ID         string
	CampaignID string
	TargetID   string
	EventType  string
	EventData  map[string]any
	IPAddress  *string
	UserAgent  *string
	CreatedAt  time.Time
}

// TargetFilters controls optional filtering for target list operations.
type TargetFilters struct {
	Email       string // partial, case-insensitive
	FirstName   string // partial, case-insensitive
	LastName    string // partial, case-insensitive
	Department  string // exact match
	Title       string // partial, case-insensitive
	CreatedFrom *time.Time
	CreatedTo   *time.Time
	IncludeDeleted bool

	// Extended filters.
	GroupID    string // only targets in this group
	CampaignID string // only targets assigned to this campaign
	Name       string // partial match across first_name or last_name

	// Pagination (page/offset mode).
	Page    int
	PerPage int

	// Cursor pagination (keyset mode). When CursorID is set, page/offset is ignored.
	CursorID        string
	CursorCreatedAt *time.Time
	Limit           int // max results; 0 = use default (25)
}

// TargetUpdate holds mutable fields for an update operation.
type TargetUpdate struct {
	Email        *string
	FirstName    *string
	LastName     *string
	Department   *string
	Title        *string
	CustomFields map[string]any // nil = no change
}

// TargetListResult holds paginated target results.
type TargetListResult struct {
	Targets []Target
	Total   int
}

// TargetCursorResult is the result of a cursor-paginated list query.
type TargetCursorResult struct {
	Targets []Target
	HasMore bool
}

// TargetImport is the DB model for a target_imports row.
type TargetImport struct {
	ID               string
	Filename         string
	RowCount         int
	ColumnCount      int
	Headers          []string
	PreviewRows      [][]string
	RawData          []byte
	Mapping          map[string]string
	Status           string
	ValidationResult map[string]any
	ImportedCount    int
	RejectedCount    int
	UploadedBy       string
	CreatedAt        time.Time
	UpdatedAt        time.Time
}

// ImportMappingTemplate is the DB model for an import_mapping_templates row.
type ImportMappingTemplate struct {
	ID        string
	Name      string
	Mapping   map[string]string
	CreatedBy string
	CreatedAt time.Time
	UpdatedAt time.Time
}

// TargetRepository provides database operations for the targets table.
type TargetRepository struct {
	db *sql.DB
}

// NewTargetRepository creates a new TargetRepository.
func NewTargetRepository(db *sql.DB) *TargetRepository {
	return &TargetRepository{db: db}
}

// Create inserts a new target and returns the created row.
func (r *TargetRepository) Create(ctx context.Context, t Target) (Target, error) {
	id := uuid.New().String()

	cfJSON, err := json.Marshal(t.CustomFields)
	if err != nil {
		return Target{}, fmt.Errorf("targets: create: marshal custom_fields: %w", err)
	}

	const q = `
		INSERT INTO targets (id, email, first_name, last_name, department, title, custom_fields, created_by)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8)
		RETURNING id, email, first_name, last_name, department, title, custom_fields,
		          created_by, deleted_at, created_at, updated_at`
	return r.scanOne(r.db.QueryRowContext(ctx, q,
		id, t.Email, t.FirstName, t.LastName, t.Department, t.Title, cfJSON, t.CreatedBy,
	))
}

// GetByID retrieves a target by UUID. Returns sql.ErrNoRows if not found.
func (r *TargetRepository) GetByID(ctx context.Context, id string) (Target, error) {
	const q = `
		SELECT id, email, first_name, last_name, department, title, custom_fields,
		       created_by, deleted_at, created_at, updated_at
		FROM targets WHERE id = $1`
	return r.scanOne(r.db.QueryRowContext(ctx, q, id))
}

// GetByEmail retrieves a target by email (case-insensitive). Returns sql.ErrNoRows if not found.
func (r *TargetRepository) GetByEmail(ctx context.Context, email string) (Target, error) {
	const q = `
		SELECT id, email, first_name, last_name, department, title, custom_fields,
		       created_by, deleted_at, created_at, updated_at
		FROM targets WHERE LOWER(email) = LOWER($1) AND deleted_at IS NULL`
	return r.scanOne(r.db.QueryRowContext(ctx, q, email))
}

// Update applies changes to the target identified by id.
func (r *TargetRepository) Update(ctx context.Context, id string, upd TargetUpdate) (Target, error) {
	setClauses := []string{}
	args := []any{}
	argIdx := 1

	if upd.Email != nil {
		setClauses = append(setClauses, fmt.Sprintf("email = $%d", argIdx))
		args = append(args, *upd.Email)
		argIdx++
	}
	if upd.FirstName != nil {
		setClauses = append(setClauses, fmt.Sprintf("first_name = $%d", argIdx))
		args = append(args, *upd.FirstName)
		argIdx++
	}
	if upd.LastName != nil {
		setClauses = append(setClauses, fmt.Sprintf("last_name = $%d", argIdx))
		args = append(args, *upd.LastName)
		argIdx++
	}
	if upd.Department != nil {
		setClauses = append(setClauses, fmt.Sprintf("department = $%d", argIdx))
		args = append(args, *upd.Department)
		argIdx++
	}
	if upd.Title != nil {
		setClauses = append(setClauses, fmt.Sprintf("title = $%d", argIdx))
		args = append(args, *upd.Title)
		argIdx++
	}
	if upd.CustomFields != nil {
		cfJSON, err := json.Marshal(upd.CustomFields)
		if err != nil {
			return Target{}, fmt.Errorf("targets: update: marshal custom_fields: %w", err)
		}
		setClauses = append(setClauses, fmt.Sprintf("custom_fields = $%d", argIdx))
		args = append(args, cfJSON)
		argIdx++
	}

	if len(setClauses) == 0 {
		return r.GetByID(ctx, id)
	}

	args = append(args, id)
	q := fmt.Sprintf(`
		UPDATE targets SET %s
		WHERE id = $%d AND deleted_at IS NULL
		RETURNING id, email, first_name, last_name, department, title, custom_fields,
		          created_by, deleted_at, created_at, updated_at`,
		strings.Join(setClauses, ", "), argIdx)

	return r.scanOne(r.db.QueryRowContext(ctx, q, args...))
}

// SoftDelete sets the deleted_at timestamp on a target.
func (r *TargetRepository) SoftDelete(ctx context.Context, id string) error {
	result, err := r.db.ExecContext(ctx,
		"UPDATE targets SET deleted_at = now() WHERE id = $1 AND deleted_at IS NULL", id)
	if err != nil {
		return fmt.Errorf("targets: soft delete: %w", err)
	}
	n, _ := result.RowsAffected()
	if n == 0 {
		return fmt.Errorf("targets: soft delete: %w", sql.ErrNoRows)
	}
	return nil
}

// Restore clears the deleted_at timestamp on a soft-deleted target.
func (r *TargetRepository) Restore(ctx context.Context, id string) error {
	result, err := r.db.ExecContext(ctx,
		"UPDATE targets SET deleted_at = NULL WHERE id = $1 AND deleted_at IS NOT NULL", id)
	if err != nil {
		return fmt.Errorf("targets: restore: %w", err)
	}
	n, _ := result.RowsAffected()
	if n == 0 {
		return fmt.Errorf("targets: restore: %w", sql.ErrNoRows)
	}
	return nil
}

// List returns targets with optional filtering and pagination.
func (r *TargetRepository) List(ctx context.Context, filters TargetFilters) (TargetListResult, error) {
	args := []any{}
	argIdx := 1
	where := "WHERE 1=1"

	if !filters.IncludeDeleted {
		where += " AND deleted_at IS NULL"
	}
	if filters.Email != "" {
		where += fmt.Sprintf(" AND email ILIKE $%d", argIdx)
		args = append(args, "%"+filters.Email+"%")
		argIdx++
	}
	if filters.FirstName != "" {
		where += fmt.Sprintf(" AND first_name ILIKE $%d", argIdx)
		args = append(args, "%"+filters.FirstName+"%")
		argIdx++
	}
	if filters.LastName != "" {
		where += fmt.Sprintf(" AND last_name ILIKE $%d", argIdx)
		args = append(args, "%"+filters.LastName+"%")
		argIdx++
	}
	if filters.Department != "" {
		where += fmt.Sprintf(" AND department = $%d", argIdx)
		args = append(args, filters.Department)
		argIdx++
	}
	if filters.Title != "" {
		where += fmt.Sprintf(" AND title ILIKE $%d", argIdx)
		args = append(args, "%"+filters.Title+"%")
		argIdx++
	}
	if filters.CreatedFrom != nil {
		where += fmt.Sprintf(" AND created_at >= $%d", argIdx)
		args = append(args, *filters.CreatedFrom)
		argIdx++
	}
	if filters.CreatedTo != nil {
		where += fmt.Sprintf(" AND created_at <= $%d", argIdx)
		args = append(args, *filters.CreatedTo)
		argIdx++
	}
	if filters.Name != "" {
		where += fmt.Sprintf(" AND (first_name ILIKE $%d OR last_name ILIKE $%d)", argIdx, argIdx)
		args = append(args, "%"+filters.Name+"%")
		argIdx++
	}
	if filters.GroupID != "" {
		where += fmt.Sprintf(" AND id IN (SELECT target_id FROM target_group_members WHERE group_id = $%d)", argIdx)
		args = append(args, filters.GroupID)
		argIdx++
	}
	if filters.CampaignID != "" {
		where += fmt.Sprintf(" AND id IN (SELECT target_id FROM campaign_targets WHERE campaign_id = $%d)", argIdx)
		args = append(args, filters.CampaignID)
		argIdx++
	}

	// Count total.
	countQ := fmt.Sprintf("SELECT COUNT(*) FROM targets %s", where)
	var total int
	if err := r.db.QueryRowContext(ctx, countQ, args...).Scan(&total); err != nil {
		return TargetListResult{}, fmt.Errorf("targets: list count: %w", err)
	}

	// Pagination defaults.
	page := filters.Page
	if page < 1 {
		page = 1
	}
	perPage := filters.PerPage
	if perPage < 1 || perPage > 100 {
		perPage = 25
	}
	offset := (page - 1) * perPage

	limitClause := fmt.Sprintf(" ORDER BY created_at DESC LIMIT $%d OFFSET $%d", argIdx, argIdx+1)
	args = append(args, perPage, offset)

	q := fmt.Sprintf(`
		SELECT id, email, first_name, last_name, department, title, custom_fields,
		       created_by, deleted_at, created_at, updated_at
		FROM targets %s%s`, where, limitClause)

	rows, err := r.db.QueryContext(ctx, q, args...)
	if err != nil {
		return TargetListResult{}, fmt.Errorf("targets: list: %w", err)
	}
	defer rows.Close()

	var results []Target
	for rows.Next() {
		t, err := r.scanRow(rows)
		if err != nil {
			return TargetListResult{}, fmt.Errorf("targets: list scan: %w", err)
		}
		results = append(results, t)
	}
	if err := rows.Err(); err != nil {
		return TargetListResult{}, fmt.Errorf("targets: list rows: %w", err)
	}

	return TargetListResult{Targets: results, Total: total}, nil
}

// ListCursor returns targets using keyset (cursor) pagination.
// It fetches limit+1 rows to determine if more pages exist.
func (r *TargetRepository) ListCursor(ctx context.Context, filters TargetFilters) (TargetCursorResult, error) {
	args := []any{}
	argIdx := 1
	where := "WHERE 1=1"

	if !filters.IncludeDeleted {
		where += " AND deleted_at IS NULL"
	}
	if filters.Email != "" {
		where += fmt.Sprintf(" AND email ILIKE $%d", argIdx)
		args = append(args, "%"+filters.Email+"%")
		argIdx++
	}
	if filters.FirstName != "" {
		where += fmt.Sprintf(" AND first_name ILIKE $%d", argIdx)
		args = append(args, "%"+filters.FirstName+"%")
		argIdx++
	}
	if filters.LastName != "" {
		where += fmt.Sprintf(" AND last_name ILIKE $%d", argIdx)
		args = append(args, "%"+filters.LastName+"%")
		argIdx++
	}
	if filters.Department != "" {
		where += fmt.Sprintf(" AND department = $%d", argIdx)
		args = append(args, filters.Department)
		argIdx++
	}
	if filters.Title != "" {
		where += fmt.Sprintf(" AND title ILIKE $%d", argIdx)
		args = append(args, "%"+filters.Title+"%")
		argIdx++
	}
	if filters.CreatedFrom != nil {
		where += fmt.Sprintf(" AND created_at >= $%d", argIdx)
		args = append(args, *filters.CreatedFrom)
		argIdx++
	}
	if filters.CreatedTo != nil {
		where += fmt.Sprintf(" AND created_at <= $%d", argIdx)
		args = append(args, *filters.CreatedTo)
		argIdx++
	}
	if filters.Name != "" {
		where += fmt.Sprintf(" AND (first_name ILIKE $%d OR last_name ILIKE $%d)", argIdx, argIdx)
		args = append(args, "%"+filters.Name+"%")
		argIdx++
	}
	if filters.GroupID != "" {
		where += fmt.Sprintf(" AND id IN (SELECT target_id FROM target_group_members WHERE group_id = $%d)", argIdx)
		args = append(args, filters.GroupID)
		argIdx++
	}
	if filters.CampaignID != "" {
		where += fmt.Sprintf(" AND id IN (SELECT target_id FROM campaign_targets WHERE campaign_id = $%d)", argIdx)
		args = append(args, filters.CampaignID)
		argIdx++
	}

	// Cursor keyset condition: rows before the cursor position.
	if filters.CursorID != "" && filters.CursorCreatedAt != nil {
		where += fmt.Sprintf(" AND (created_at, id) < ($%d, $%d)", argIdx, argIdx+1)
		args = append(args, *filters.CursorCreatedAt, filters.CursorID)
		argIdx += 2
	}

	limit := filters.Limit
	if limit < 1 || limit > 100 {
		limit = 25
	}

	// Fetch limit+1 to detect hasMore.
	limitClause := fmt.Sprintf(" ORDER BY created_at DESC, id DESC LIMIT $%d", argIdx)
	args = append(args, limit+1)

	q := fmt.Sprintf(`
		SELECT id, email, first_name, last_name, department, title, custom_fields,
		       created_by, deleted_at, created_at, updated_at
		FROM targets %s%s`, where, limitClause)

	rows, err := r.db.QueryContext(ctx, q, args...)
	if err != nil {
		return TargetCursorResult{}, fmt.Errorf("targets: list cursor: %w", err)
	}
	defer rows.Close()

	var results []Target
	for rows.Next() {
		t, err := r.scanRow(rows)
		if err != nil {
			return TargetCursorResult{}, fmt.Errorf("targets: list cursor scan: %w", err)
		}
		results = append(results, t)
	}
	if err := rows.Err(); err != nil {
		return TargetCursorResult{}, fmt.Errorf("targets: list cursor rows: %w", err)
	}

	hasMore := len(results) > limit
	if hasMore {
		results = results[:limit]
	}

	return TargetCursorResult{Targets: results, HasMore: hasMore}, nil
}

// BulkCreate inserts multiple targets in a single transaction.
// Returns the created targets and any per-row errors.
func (r *TargetRepository) BulkCreate(ctx context.Context, targets []Target) ([]Target, []error) {
	created := make([]Target, 0, len(targets))
	errs := make([]error, len(targets))

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		for i := range targets {
			errs[i] = fmt.Errorf("targets: bulk create: begin tx: %w", err)
		}
		return nil, errs
	}
	defer tx.Rollback() //nolint:errcheck

	const q = `
		INSERT INTO targets (id, email, first_name, last_name, department, title, custom_fields, created_by)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8)
		RETURNING id, email, first_name, last_name, department, title, custom_fields,
		          created_by, deleted_at, created_at, updated_at`

	for i, t := range targets {
		id := uuid.New().String()
		cfJSON, err := json.Marshal(t.CustomFields)
		if err != nil {
			errs[i] = fmt.Errorf("row %d: marshal custom_fields: %w", i, err)
			continue
		}

		row := tx.QueryRowContext(ctx, q,
			id, t.Email, t.FirstName, t.LastName, t.Department, t.Title, cfJSON, t.CreatedBy,
		)
		result, err := r.scanOneTx(row)
		if err != nil {
			errs[i] = fmt.Errorf("row %d: %w", i, err)
			continue
		}
		created = append(created, result)
	}

	if err := tx.Commit(); err != nil {
		for i := range targets {
			errs[i] = fmt.Errorf("targets: bulk create: commit: %w", err)
		}
		return nil, errs
	}

	return created, errs
}

// BulkSoftDelete soft-deletes multiple targets by ID.
func (r *TargetRepository) BulkSoftDelete(ctx context.Context, ids []string) (int64, error) {
	if len(ids) == 0 {
		return 0, nil
	}

	placeholders := make([]string, len(ids))
	args := make([]any, len(ids))
	for i, id := range ids {
		placeholders[i] = fmt.Sprintf("$%d", i+1)
		args[i] = id
	}

	q := fmt.Sprintf(
		"UPDATE targets SET deleted_at = now() WHERE id IN (%s) AND deleted_at IS NULL",
		strings.Join(placeholders, ","))

	result, err := r.db.ExecContext(ctx, q, args...)
	if err != nil {
		return 0, fmt.Errorf("targets: bulk soft delete: %w", err)
	}
	n, _ := result.RowsAffected()
	return n, nil
}

// BulkUpdateField sets a single field to the same value for all specified targets.
func (r *TargetRepository) BulkUpdateField(ctx context.Context, ids []string, field, value string) (int64, error) {
	if len(ids) == 0 {
		return 0, nil
	}

	// Whitelist allowed fields.
	allowed := map[string]bool{
		"first_name": true, "last_name": true,
		"department": true, "title": true,
	}
	if !allowed[field] {
		return 0, fmt.Errorf("targets: bulk update: field %q not allowed", field)
	}

	placeholders := make([]string, len(ids))
	args := make([]any, len(ids)+1)
	args[0] = value
	for i, id := range ids {
		placeholders[i] = fmt.Sprintf("$%d", i+2)
		args[i+1] = id
	}

	q := fmt.Sprintf(
		"UPDATE targets SET %s = $1 WHERE id IN (%s) AND deleted_at IS NULL",
		field, strings.Join(placeholders, ","))

	result, err := r.db.ExecContext(ctx, q, args...)
	if err != nil {
		return 0, fmt.Errorf("targets: bulk update field: %w", err)
	}
	n, _ := result.RowsAffected()
	return n, nil
}

// GetDepartments returns the distinct department values for active targets.
func (r *TargetRepository) GetDepartments(ctx context.Context) ([]string, error) {
	rows, err := r.db.QueryContext(ctx,
		"SELECT DISTINCT department FROM targets WHERE department IS NOT NULL AND deleted_at IS NULL ORDER BY department")
	if err != nil {
		return nil, fmt.Errorf("targets: get departments: %w", err)
	}
	defer rows.Close()

	var depts []string
	for rows.Next() {
		var d string
		if err := rows.Scan(&d); err != nil {
			return nil, fmt.Errorf("targets: get departments scan: %w", err)
		}
		depts = append(depts, d)
	}
	return depts, rows.Err()
}

// CheckEmailExists checks if an active target with the given email exists. Returns the target ID if found.
func (r *TargetRepository) CheckEmailExists(ctx context.Context, email string) (string, bool, error) {
	var id string
	err := r.db.QueryRowContext(ctx,
		"SELECT id FROM targets WHERE LOWER(email) = LOWER($1) AND deleted_at IS NULL", email,
	).Scan(&id)
	if err != nil {
		if err == sql.ErrNoRows {
			return "", false, nil
		}
		return "", false, fmt.Errorf("targets: check email: %w", err)
	}
	return id, true, nil
}

// PurgeExpired permanently removes PII from targets soft-deleted before the cutoff time.
func (r *TargetRepository) PurgeExpired(ctx context.Context, cutoff time.Time) (int64, error) {
	result, err := r.db.ExecContext(ctx, `
		UPDATE targets
		SET email = '[purged]', first_name = NULL, last_name = NULL,
		    department = NULL, title = NULL, custom_fields = '{}'::jsonb
		WHERE deleted_at IS NOT NULL AND deleted_at < $1 AND email != '[purged]'`, cutoff)
	if err != nil {
		return 0, fmt.Errorf("targets: purge expired: %w", err)
	}
	n, _ := result.RowsAffected()
	return n, nil
}

// --- scan helpers ---

func (r *TargetRepository) scanOne(row *sql.Row) (Target, error) {
	var t Target
	var cfJSON []byte
	err := row.Scan(
		&t.ID, &t.Email, &t.FirstName, &t.LastName, &t.Department, &t.Title,
		&cfJSON, &t.CreatedBy, &t.DeletedAt, &t.CreatedAt, &t.UpdatedAt,
	)
	if err != nil {
		return Target{}, err
	}
	t.CustomFields = map[string]any{}
	if len(cfJSON) > 0 {
		_ = json.Unmarshal(cfJSON, &t.CustomFields)
	}
	return t, nil
}

func (r *TargetRepository) scanOneTx(row *sql.Row) (Target, error) {
	return r.scanOne(row)
}

// UpdateCampaignTargetStatus updates the status of a campaign_targets row.
func (r *TargetRepository) UpdateCampaignTargetStatus(ctx context.Context, campaignID, targetID, status string) error {
	_, err := r.db.ExecContext(ctx,
		`UPDATE campaign_targets SET status = $1 WHERE campaign_id = $2 AND target_id = $3`,
		status, campaignID, targetID)
	if err != nil {
		return fmt.Errorf("targets: update campaign target status: %w", err)
	}
	return nil
}

func (r *TargetRepository) scanRow(rows *sql.Rows) (Target, error) {
	var t Target
	var cfJSON []byte
	err := rows.Scan(
		&t.ID, &t.Email, &t.FirstName, &t.LastName, &t.Department, &t.Title,
		&cfJSON, &t.CreatedBy, &t.DeletedAt, &t.CreatedAt, &t.UpdatedAt,
	)
	if err != nil {
		return Target{}, err
	}
	t.CustomFields = map[string]any{}
	if len(cfJSON) > 0 {
		_ = json.Unmarshal(cfJSON, &t.CustomFields)
	}
	return t, nil
}
