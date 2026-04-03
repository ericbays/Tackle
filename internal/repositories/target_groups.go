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

// TargetGroup is the DB model for a target_groups row.
type TargetGroup struct {
	ID          string
	Name        string
	Description string
	CreatedBy   string
	DeletedAt   *time.Time
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

// TargetGroupMember is the DB model for a target_group_members row.
type TargetGroupMember struct {
	GroupID  string
	TargetID string
	AddedAt  time.Time
	AddedBy  string
}

// TargetGroupWithCount holds a group and its member count.
type TargetGroupWithCount struct {
	TargetGroup
	MemberCount int
}

// TargetGroupFilters controls optional filtering for group list operations.
type TargetGroupFilters struct {
	Name    string // partial, case-insensitive
	Page    int
	PerPage int
}

// TargetGroupRepository provides database operations for the target_groups table.
type TargetGroupRepository struct {
	db *sql.DB
}

// NewTargetGroupRepository creates a new TargetGroupRepository.
func NewTargetGroupRepository(db *sql.DB) *TargetGroupRepository {
	return &TargetGroupRepository{db: db}
}

// Create inserts a new target group and returns the created row.
func (r *TargetGroupRepository) Create(ctx context.Context, g TargetGroup) (TargetGroup, error) {
	id := uuid.New().String()

	const q = `
		INSERT INTO target_groups (id, name, description, created_by)
		VALUES ($1, $2, $3, $4)
		RETURNING id, name, description, created_by, deleted_at, created_at, updated_at`

	var out TargetGroup
	err := r.db.QueryRowContext(ctx, q, id, g.Name, g.Description, g.CreatedBy).Scan(
		&out.ID, &out.Name, &out.Description, &out.CreatedBy, &out.DeletedAt, &out.CreatedAt, &out.UpdatedAt,
	)
	if err != nil {
		return TargetGroup{}, fmt.Errorf("target_groups: create: %w", err)
	}
	return out, nil
}

// GetByID retrieves a target group by UUID. Returns sql.ErrNoRows if not found.
func (r *TargetGroupRepository) GetByID(ctx context.Context, id string) (TargetGroup, error) {
	const q = `
		SELECT id, name, description, created_by, deleted_at, created_at, updated_at
		FROM target_groups WHERE id = $1 AND deleted_at IS NULL`

	var out TargetGroup
	err := r.db.QueryRowContext(ctx, q, id).Scan(
		&out.ID, &out.Name, &out.Description, &out.CreatedBy, &out.DeletedAt, &out.CreatedAt, &out.UpdatedAt,
	)
	if err != nil {
		return TargetGroup{}, fmt.Errorf("target_groups: get: %w", err)
	}
	return out, nil
}

// GetByName retrieves a target group by name (case-insensitive).
func (r *TargetGroupRepository) GetByName(ctx context.Context, name string) (TargetGroup, error) {
	const q = `
		SELECT id, name, description, created_by, deleted_at, created_at, updated_at
		FROM target_groups WHERE LOWER(name) = LOWER($1) AND deleted_at IS NULL`

	var out TargetGroup
	err := r.db.QueryRowContext(ctx, q, name).Scan(
		&out.ID, &out.Name, &out.Description, &out.CreatedBy, &out.DeletedAt, &out.CreatedAt, &out.UpdatedAt,
	)
	if err != nil {
		return TargetGroup{}, fmt.Errorf("target_groups: get_by_name: %w", err)
	}
	return out, nil
}

// Update applies changes to the target group identified by id.
func (r *TargetGroupRepository) Update(ctx context.Context, id string, name, description *string) (TargetGroup, error) {
	setClauses := []string{}
	args := []any{}
	argIdx := 1

	if name != nil {
		setClauses = append(setClauses, fmt.Sprintf("name = $%d", argIdx))
		args = append(args, *name)
		argIdx++
	}
	if description != nil {
		setClauses = append(setClauses, fmt.Sprintf("description = $%d", argIdx))
		args = append(args, *description)
		argIdx++
	}

	if len(setClauses) == 0 {
		return r.GetByID(ctx, id)
	}

	args = append(args, id)
	q := fmt.Sprintf(`
		UPDATE target_groups SET %s
		WHERE id = $%d AND deleted_at IS NULL
		RETURNING id, name, description, created_by, deleted_at, created_at, updated_at`,
		strings.Join(setClauses, ", "), argIdx)

	var out TargetGroup
	err := r.db.QueryRowContext(ctx, q, args...).Scan(
		&out.ID, &out.Name, &out.Description, &out.CreatedBy, &out.DeletedAt, &out.CreatedAt, &out.UpdatedAt,
	)
	if err != nil {
		return TargetGroup{}, fmt.Errorf("target_groups: update: %w", err)
	}
	return out, nil
}

// Delete soft-deletes a target group by setting deleted_at.
func (r *TargetGroupRepository) Delete(ctx context.Context, id string) error {
	const q = `UPDATE target_groups SET deleted_at = now() WHERE id = $1 AND deleted_at IS NULL`
	res, err := r.db.ExecContext(ctx, q, id)
	if err != nil {
		return fmt.Errorf("target_groups: delete: %w", err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return sql.ErrNoRows
	}
	return nil
}

// List returns target groups with member counts, filtered and paginated.
func (r *TargetGroupRepository) List(ctx context.Context, f TargetGroupFilters) ([]TargetGroupWithCount, int, error) {
	where := []string{}
	args := []any{}
	argIdx := 1

	if f.Name != "" {
		where = append(where, fmt.Sprintf("LOWER(tg.name) LIKE LOWER($%d)", argIdx))
		args = append(args, "%"+f.Name+"%")
		argIdx++
	}

	where = append(where, "tg.deleted_at IS NULL")

	whereClause := ""
	if len(where) > 0 {
		whereClause = "WHERE " + strings.Join(where, " AND ")
	}

	// Count.
	countQ := fmt.Sprintf(`SELECT COUNT(*) FROM target_groups tg %s`, whereClause)
	var total int
	if err := r.db.QueryRowContext(ctx, countQ, args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("target_groups: list count: %w", err)
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
		SELECT tg.id, tg.name, tg.description, tg.created_by, tg.deleted_at, tg.created_at, tg.updated_at,
		       COUNT(tgm.target_id) AS member_count
		FROM target_groups tg
		LEFT JOIN target_group_members tgm ON tgm.group_id = tg.id
		%s
		GROUP BY tg.id
		ORDER BY tg.name ASC
		LIMIT $%d OFFSET $%d`, whereClause, argIdx, argIdx+1)

	rows, err := r.db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("target_groups: list: %w", err)
	}
	defer rows.Close()

	var groups []TargetGroupWithCount
	for rows.Next() {
		var g TargetGroupWithCount
		if err := rows.Scan(&g.ID, &g.Name, &g.Description, &g.CreatedBy, &g.DeletedAt, &g.CreatedAt, &g.UpdatedAt, &g.MemberCount); err != nil {
			return nil, 0, fmt.Errorf("target_groups: list scan: %w", err)
		}
		groups = append(groups, g)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("target_groups: list rows: %w", err)
	}

	return groups, total, nil
}

// AddMembers adds targets to a group. Idempotent: existing memberships are ignored.
func (r *TargetGroupRepository) AddMembers(ctx context.Context, groupID string, targetIDs []string, addedBy string) (int, error) {
	if len(targetIDs) == 0 {
		return 0, nil
	}

	// Build batch insert with ON CONFLICT DO NOTHING for idempotency.
	values := []string{}
	args := []any{groupID, addedBy}
	argIdx := 3
	for _, tid := range targetIDs {
		values = append(values, fmt.Sprintf("($1, $%d, now(), $2)", argIdx))
		args = append(args, tid)
		argIdx++
	}

	q := fmt.Sprintf(`
		INSERT INTO target_group_members (group_id, target_id, added_at, added_by)
		VALUES %s
		ON CONFLICT (group_id, target_id) DO NOTHING`, strings.Join(values, ", "))

	res, err := r.db.ExecContext(ctx, q, args...)
	if err != nil {
		return 0, fmt.Errorf("target_group_members: add: %w", err)
	}
	n, _ := res.RowsAffected()
	return int(n), nil
}

// RemoveMembers removes targets from a group. Idempotent: missing memberships are ignored.
func (r *TargetGroupRepository) RemoveMembers(ctx context.Context, groupID string, targetIDs []string) (int, error) {
	if len(targetIDs) == 0 {
		return 0, nil
	}

	placeholders := []string{}
	args := []any{groupID}
	for i, tid := range targetIDs {
		placeholders = append(placeholders, fmt.Sprintf("$%d", i+2))
		args = append(args, tid)
	}

	q := fmt.Sprintf(`
		DELETE FROM target_group_members
		WHERE group_id = $1 AND target_id IN (%s)`, strings.Join(placeholders, ", "))

	res, err := r.db.ExecContext(ctx, q, args...)
	if err != nil {
		return 0, fmt.Errorf("target_group_members: remove: %w", err)
	}
	n, _ := res.RowsAffected()
	return int(n), nil
}

// ListMembers returns paginated members of a group.
func (r *TargetGroupRepository) ListMembers(ctx context.Context, groupID string, page, perPage int) ([]Target, int, error) {
	// Count.
	var total int
	if err := r.db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM target_group_members WHERE group_id = $1`, groupID,
	).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("target_group_members: count: %w", err)
	}

	if page < 1 {
		page = 1
	}
	if perPage < 1 || perPage > 100 {
		perPage = 25
	}
	offset := (page - 1) * perPage

	const q = `
		SELECT t.id, t.email, t.first_name, t.last_name, t.department, t.title,
		       t.custom_fields, t.created_by, t.deleted_at, t.created_at, t.updated_at
		FROM targets t
		INNER JOIN target_group_members tgm ON tgm.target_id = t.id
		WHERE tgm.group_id = $1
		ORDER BY t.email ASC
		LIMIT $2 OFFSET $3`

	rows, err := r.db.QueryContext(ctx, q, groupID, perPage, offset)
	if err != nil {
		return nil, 0, fmt.Errorf("target_group_members: list: %w", err)
	}
	defer rows.Close()

	var targets []Target
	for rows.Next() {
		var t Target
		var cfJSON []byte
		if err := rows.Scan(
			&t.ID, &t.Email, &t.FirstName, &t.LastName, &t.Department, &t.Title,
			&cfJSON, &t.CreatedBy, &t.DeletedAt, &t.CreatedAt, &t.UpdatedAt,
		); err != nil {
			return nil, 0, fmt.Errorf("target_group_members: list scan: %w", err)
		}
		if cfJSON != nil {
			_ = json.Unmarshal(cfJSON, &t.CustomFields)
		}
		if t.CustomFields == nil {
			t.CustomFields = map[string]any{}
		}
		targets = append(targets, t)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("target_group_members: list rows: %w", err)
	}

	return targets, total, nil
}

// MemberCount returns the number of members in a group.
func (r *TargetGroupRepository) MemberCount(ctx context.Context, groupID string) (int, error) {
	var count int
	err := r.db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM target_group_members WHERE group_id = $1`, groupID,
	).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("target_group_members: count: %w", err)
	}
	return count, nil
}

// AssignGroupToCampaign assigns a group to a campaign. Idempotent.
func (r *TargetGroupRepository) AssignGroupToCampaign(ctx context.Context, campaignID, groupID, assignedBy string) error {
	const q = `
		INSERT INTO campaign_target_groups (campaign_id, group_id, assigned_at, assigned_by)
		VALUES ($1, $2, now(), $3)
		ON CONFLICT (campaign_id, group_id) DO NOTHING`
	_, err := r.db.ExecContext(ctx, q, campaignID, groupID, assignedBy)
	if err != nil {
		return fmt.Errorf("campaign_target_groups: assign: %w", err)
	}
	return nil
}

// UnassignGroupFromCampaign removes a group assignment from a campaign.
func (r *TargetGroupRepository) UnassignGroupFromCampaign(ctx context.Context, campaignID, groupID string) error {
	const q = `DELETE FROM campaign_target_groups WHERE campaign_id = $1 AND group_id = $2`
	_, err := r.db.ExecContext(ctx, q, campaignID, groupID)
	if err != nil {
		return fmt.Errorf("campaign_target_groups: unassign: %w", err)
	}
	return nil
}

// ListCampaignGroups returns groups assigned to a campaign.
func (r *TargetGroupRepository) ListCampaignGroups(ctx context.Context, campaignID string) ([]TargetGroupWithCount, error) {
	const q = `
		SELECT tg.id, tg.name, tg.description, tg.created_by, tg.deleted_at, tg.created_at, tg.updated_at,
		       COUNT(tgm.target_id) AS member_count
		FROM target_groups tg
		INNER JOIN campaign_target_groups ctg ON ctg.group_id = tg.id
		LEFT JOIN target_group_members tgm ON tgm.group_id = tg.id
		WHERE ctg.campaign_id = $1 AND tg.deleted_at IS NULL
		GROUP BY tg.id
		ORDER BY tg.name ASC`

	rows, err := r.db.QueryContext(ctx, q, campaignID)
	if err != nil {
		return nil, fmt.Errorf("campaign_target_groups: list: %w", err)
	}
	defer rows.Close()

	var groups []TargetGroupWithCount
	for rows.Next() {
		var g TargetGroupWithCount
		if err := rows.Scan(&g.ID, &g.Name, &g.Description, &g.CreatedBy, &g.DeletedAt, &g.CreatedAt, &g.UpdatedAt, &g.MemberCount); err != nil {
			return nil, fmt.Errorf("campaign_target_groups: list scan: %w", err)
		}
		groups = append(groups, g)
	}
	return groups, rows.Err()
}

// ResolveTargetsForCampaign returns deduplicated targets from both direct assignment
// and group membership for a campaign. Each target includes its source (direct, group name, or both).
func (r *TargetGroupRepository) ResolveTargetsForCampaign(ctx context.Context, campaignID string) ([]ResolvedTarget, error) {
	// Combine directly assigned targets with group-member targets, deduplicated by target_id.
	const q = `
		WITH direct AS (
			SELECT ct.target_id, 'direct' AS source
			FROM campaign_targets ct
			WHERE ct.campaign_id = $1 AND ct.removed_at IS NULL
		),
		from_groups AS (
			SELECT DISTINCT tgm.target_id, tg.name AS source
			FROM campaign_target_groups ctg
			INNER JOIN target_group_members tgm ON tgm.group_id = ctg.group_id
			INNER JOIN target_groups tg ON tg.id = ctg.group_id
			WHERE ctg.campaign_id = $1
		),
		all_sources AS (
			SELECT target_id, source FROM direct
			UNION ALL
			SELECT target_id, source FROM from_groups
		),
		aggregated AS (
			SELECT target_id, array_agg(DISTINCT source ORDER BY source) AS sources
			FROM all_sources
			GROUP BY target_id
		)
		SELECT t.id, t.email, t.first_name, t.last_name, t.department, t.title,
		       a.sources
		FROM aggregated a
		INNER JOIN targets t ON t.id = a.target_id
		WHERE t.deleted_at IS NULL
		ORDER BY t.email ASC`

	rows, err := r.db.QueryContext(ctx, q, campaignID)
	if err != nil {
		return nil, fmt.Errorf("resolve_targets: %w", err)
	}
	defer rows.Close()

	var resolved []ResolvedTarget
	for rows.Next() {
		var rt ResolvedTarget
		var sources []byte // PostgreSQL text[] comes as a string
		if err := rows.Scan(
			&rt.ID, &rt.Email, &rt.FirstName, &rt.LastName, &rt.Department, &rt.Title,
			&sources,
		); err != nil {
			return nil, fmt.Errorf("resolve_targets: scan: %w", err)
		}
		rt.Sources = parsePostgresArray(string(sources))
		resolved = append(resolved, rt)
	}
	return resolved, rows.Err()
}

// ResolvedTarget is a target with its source information for campaign resolution.
type ResolvedTarget struct {
	ID         string
	Email      string
	FirstName  *string
	LastName   *string
	Department *string
	Title      *string
	Sources    []string // "direct", group name(s), or both
}

// parsePostgresArray parses a PostgreSQL text array literal like {a,b,c}.
func parsePostgresArray(s string) []string {
	s = strings.TrimPrefix(s, "{")
	s = strings.TrimSuffix(s, "}")
	if s == "" {
		return nil
	}
	parts := strings.Split(s, ",")
	result := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.Trim(p, `"`)
		if p != "" {
			result = append(result, p)
		}
	}
	return result
}
