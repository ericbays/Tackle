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

// Campaign is the DB model for a campaigns row.
type Campaign struct {
	ID                string
	Name              string
	Description       string
	CurrentState      string
	StateChangedAt    time.Time
	LandingPageID     *string
	CloudProvider     *string
	Region            *string
	InstanceType      *string
	EndpointDomainID  *string
	ThrottleRate      *int
	InterEmailDelayMin *int
	InterEmailDelayMax *int
	SendOrder         string
	ScheduledLaunchAt *time.Time
	GracePeriodHours  int
	StartDate         *time.Time
	EndDate           *time.Time
	ApprovedBy        *string
	ApprovalComment   *string
	LaunchedAt        *time.Time
	CompletedAt       *time.Time
	ArchivedAt        *time.Time
	Configuration     map[string]any
	CreatedBy         string
	DeletedAt         *time.Time
	CreatedAt         time.Time
	UpdatedAt         time.Time
}

// CampaignUpdate holds mutable fields for campaign update.
type CampaignUpdate struct {
	Name              *string
	Description       *string
	LandingPageID     *string
	CloudProvider     *string
	Region            *string
	InstanceType      *string
	EndpointDomainID  *string
	ThrottleRate      *int
	InterEmailDelayMin *int
	InterEmailDelayMax *int
	SendOrder         *string
	ScheduledLaunchAt *time.Time
	GracePeriodHours  *int
	StartDate         *time.Time
	EndDate           *time.Time
	Configuration     map[string]any
}

// CampaignFilters controls optional filtering for campaign list operations.
type CampaignFilters struct {
	States     []string // filter by current_state (OR)
	Name       string   // partial, case-insensitive
	CreatedBy  string   // exact UUID match
	DateFrom   *time.Time
	DateTo     *time.Time
	IncludeArchived bool
	Page       int
	PerPage    int
	// OwnerID restricts to campaigns created by or shared with this user (operator scoping).
	OwnerID string
}

// CampaignListResult holds paginated campaign results.
type CampaignListResult struct {
	Campaigns []Campaign
	Total     int
}

// CampaignStateTransition is the DB model for campaign_state_transitions.
type CampaignStateTransition struct {
	ID         string
	CampaignID string
	FromState  string
	ToState    string
	ActorID    *string
	Reason     string
	CreatedAt  time.Time
}

// CampaignBuildLog is the DB model for campaign_build_logs.
type CampaignBuildLog struct {
	ID           string
	CampaignID   string
	StepName     string
	StepOrder    int
	Status       string
	StartedAt    *time.Time
	CompletedAt  *time.Time
	ErrorDetails *string
	CreatedAt    time.Time
}

// CampaignEmail is the DB model for campaign_emails.
type CampaignEmail struct {
	ID                string
	CampaignID        string
	TargetID          string
	VariantID         *string
	SMTPConfigID      *string
	Status            string
	MessageID         *string
	SentAt            *time.Time
	DeliveredAt       *time.Time
	BouncedAt         *time.Time
	BounceType        *string
	BounceMessage     *string
	RetryCount        int
	NextRetryAt       *time.Time
	SendOrderPosition *int
	VariantLabel      *string
	TrackingToken     *string
	CreatedAt         time.Time
	UpdatedAt         time.Time
}

// EmailDeliveryEvent is the DB model for email_delivery_events.
type EmailDeliveryEvent struct {
	ID        string
	EmailID   string
	EventType string
	EventData map[string]any
	CreatedAt time.Time
}

// CampaignSMTPSendCount tracks per-profile send counts for round-robin.
type CampaignSMTPSendCount struct {
	ID            string
	CampaignID    string
	SMTPProfileID string
	SendCount     int
	ErrorCount    int
	LastSentAt    *time.Time
	CreatedAt     time.Time
	UpdatedAt     time.Time
}

// CampaignTemplateVariant is the DB model for campaign_template_variants.
type CampaignTemplateVariant struct {
	ID         string
	CampaignID string
	TemplateID string
	SplitRatio int
	Label      string
	CreatedAt  time.Time
}

// CampaignSendWindow is the DB model for campaign_send_windows.
type CampaignSendWindow struct {
	ID         string
	CampaignID string
	Days       []string
	StartTime  string
	EndTime    string
	Timezone   string
	CreatedAt  time.Time
}

// CampaignTargetsSnapshot is the DB model for campaign_targets_snapshot.
type CampaignTargetsSnapshot struct {
	ID                string
	CampaignID        string
	TargetID          string
	VariantLabel      *string
	SendOrderPosition *int
	IsCanary          bool
	SnapshottedAt     time.Time
}

// CampaignConfigTemplate is the DB model for campaign_config_templates.
type CampaignConfigTemplate struct {
	ID          string
	Name        string
	Description string
	ConfigJSON  map[string]any
	CreatedBy   string
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

// CampaignRepository handles campaign-related database operations.
type CampaignRepository struct {
	db *sql.DB
}

// NewCampaignRepository creates a new CampaignRepository.
func NewCampaignRepository(db *sql.DB) *CampaignRepository {
	return &CampaignRepository{db: db}
}

// ---------- Campaign CRUD ----------

const campaignColumns = `id, name, description, current_state, state_changed_at,
	landing_page_id, cloud_provider, region, instance_type, endpoint_domain_id,
	throttle_rate, inter_email_delay_min, inter_email_delay_max, send_order,
	scheduled_launch_at, grace_period_hours, start_date, end_date,
	approved_by, approval_comment, launched_at, completed_at, archived_at,
	configuration, created_by, deleted_at, created_at, updated_at`

func scanCampaign(row interface{ Scan(...any) error }) (Campaign, error) {
	var c Campaign
	var cfgJSON []byte
	err := row.Scan(
		&c.ID, &c.Name, &c.Description, &c.CurrentState, &c.StateChangedAt,
		&c.LandingPageID, &c.CloudProvider, &c.Region, &c.InstanceType, &c.EndpointDomainID,
		&c.ThrottleRate, &c.InterEmailDelayMin, &c.InterEmailDelayMax, &c.SendOrder,
		&c.ScheduledLaunchAt, &c.GracePeriodHours, &c.StartDate, &c.EndDate,
		&c.ApprovedBy, &c.ApprovalComment, &c.LaunchedAt, &c.CompletedAt, &c.ArchivedAt,
		&cfgJSON, &c.CreatedBy, &c.DeletedAt, &c.CreatedAt, &c.UpdatedAt,
	)
	if err != nil {
		return Campaign{}, err
	}
	if cfgJSON != nil {
		_ = json.Unmarshal(cfgJSON, &c.Configuration)
	}
	if c.Configuration == nil {
		c.Configuration = make(map[string]any)
	}
	return c, nil
}

// Create inserts a new campaign in Draft state and returns it.
func (r *CampaignRepository) Create(ctx context.Context, c Campaign) (Campaign, error) {
	c.ID = uuid.New().String()
	cfgJSON, err := json.Marshal(c.Configuration)
	if err != nil {
		return Campaign{}, fmt.Errorf("campaigns: create: marshal config: %w", err)
	}

	q := fmt.Sprintf(`
		INSERT INTO campaigns (id, name, description, current_state, landing_page_id,
			cloud_provider, region, instance_type, endpoint_domain_id,
			throttle_rate, inter_email_delay_min, inter_email_delay_max, send_order,
			scheduled_launch_at, grace_period_hours, start_date, end_date,
			configuration, created_by)
		VALUES ($1,$2,$3,'draft',$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17,$18)
		RETURNING %s`, campaignColumns)

	return scanCampaign(r.db.QueryRowContext(ctx, q,
		c.ID, c.Name, c.Description, c.LandingPageID,
		c.CloudProvider, c.Region, c.InstanceType, c.EndpointDomainID,
		c.ThrottleRate, c.InterEmailDelayMin, c.InterEmailDelayMax, c.SendOrder,
		c.ScheduledLaunchAt, c.GracePeriodHours, c.StartDate, c.EndDate,
		cfgJSON, c.CreatedBy,
	))
}

// GetByID returns a campaign by ID (excluding soft-deleted).
func (r *CampaignRepository) GetByID(ctx context.Context, id string) (Campaign, error) {
	q := fmt.Sprintf(`SELECT %s FROM campaigns WHERE id = $1 AND deleted_at IS NULL`, campaignColumns)
	c, err := scanCampaign(r.db.QueryRowContext(ctx, q, id))
	if err == sql.ErrNoRows {
		return Campaign{}, fmt.Errorf("campaigns: not found")
	}
	if err != nil {
		return Campaign{}, fmt.Errorf("campaigns: get by id: %w", err)
	}
	return c, nil
}

// GetByIDForUpdate returns a campaign with a row-level lock for state transitions.
func (r *CampaignRepository) GetByIDForUpdate(ctx context.Context, tx *sql.Tx, id string) (Campaign, error) {
	q := fmt.Sprintf(`SELECT %s FROM campaigns WHERE id = $1 AND deleted_at IS NULL FOR UPDATE`, campaignColumns)
	c, err := scanCampaign(tx.QueryRowContext(ctx, q, id))
	if err == sql.ErrNoRows {
		return Campaign{}, fmt.Errorf("campaigns: not found")
	}
	if err != nil {
		return Campaign{}, fmt.Errorf("campaigns: get for update: %w", err)
	}
	return c, nil
}

// Update modifies a campaign. Only non-nil fields in the update struct are changed.
func (r *CampaignRepository) Update(ctx context.Context, id string, u CampaignUpdate) (Campaign, error) {
	sets := []string{}
	args := []any{}
	idx := 1

	addSet := func(col string, val any) {
		sets = append(sets, fmt.Sprintf("%s = $%d", col, idx))
		args = append(args, val)
		idx++
	}

	if u.Name != nil {
		addSet("name", *u.Name)
	}
	if u.Description != nil {
		addSet("description", *u.Description)
	}
	if u.LandingPageID != nil {
		addSet("landing_page_id", *u.LandingPageID)
	}
	if u.CloudProvider != nil {
		addSet("cloud_provider", *u.CloudProvider)
	}
	if u.Region != nil {
		addSet("region", *u.Region)
	}
	if u.InstanceType != nil {
		addSet("instance_type", *u.InstanceType)
	}
	if u.EndpointDomainID != nil {
		addSet("endpoint_domain_id", *u.EndpointDomainID)
	}
	if u.ThrottleRate != nil {
		addSet("throttle_rate", *u.ThrottleRate)
	}
	if u.InterEmailDelayMin != nil {
		addSet("inter_email_delay_min", *u.InterEmailDelayMin)
	}
	if u.InterEmailDelayMax != nil {
		addSet("inter_email_delay_max", *u.InterEmailDelayMax)
	}
	if u.SendOrder != nil {
		addSet("send_order", *u.SendOrder)
	}
	if u.ScheduledLaunchAt != nil {
		addSet("scheduled_launch_at", *u.ScheduledLaunchAt)
	}
	if u.GracePeriodHours != nil {
		addSet("grace_period_hours", *u.GracePeriodHours)
	}
	if u.StartDate != nil {
		addSet("start_date", *u.StartDate)
	}
	if u.EndDate != nil {
		addSet("end_date", *u.EndDate)
	}
	if u.Configuration != nil {
		cfgJSON, err := json.Marshal(u.Configuration)
		if err != nil {
			return Campaign{}, fmt.Errorf("campaigns: update: marshal config: %w", err)
		}
		addSet("configuration", cfgJSON)
	}

	if len(sets) == 0 {
		return r.GetByID(ctx, id)
	}

	args = append(args, id)
	q := fmt.Sprintf(`UPDATE campaigns SET %s WHERE id = $%d AND deleted_at IS NULL RETURNING %s`,
		strings.Join(sets, ", "), idx, campaignColumns)

	c, err := scanCampaign(r.db.QueryRowContext(ctx, q, args...))
	if err == sql.ErrNoRows {
		return Campaign{}, fmt.Errorf("campaigns: not found")
	}
	if err != nil {
		return Campaign{}, fmt.Errorf("campaigns: update: %w", err)
	}
	return c, nil
}

// Delete hard-deletes a draft campaign.
func (r *CampaignRepository) Delete(ctx context.Context, id string) error {
	res, err := r.db.ExecContext(ctx,
		`DELETE FROM campaigns WHERE id = $1 AND current_state = 'draft' AND deleted_at IS NULL`, id)
	if err != nil {
		return fmt.Errorf("campaigns: delete: %w", err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return fmt.Errorf("campaigns: not found or not in draft state")
	}
	return nil
}

// List returns paginated, filtered campaigns.
func (r *CampaignRepository) List(ctx context.Context, f CampaignFilters) (CampaignListResult, error) {
	where := []string{}
	args := []any{}
	idx := 1

	addWhere := func(clause string, val any) {
		where = append(where, fmt.Sprintf(clause, idx))
		args = append(args, val)
		idx++
	}

	if !f.IncludeArchived {
		where = append(where, "current_state != 'archived'")
	}
	where = append(where, "deleted_at IS NULL")

	if len(f.States) > 0 {
		placeholders := make([]string, len(f.States))
		for i, s := range f.States {
			placeholders[i] = fmt.Sprintf("$%d", idx)
			args = append(args, s)
			idx++
		}
		where = append(where, fmt.Sprintf("current_state IN (%s)", strings.Join(placeholders, ",")))
	}

	if f.Name != "" {
		addWhere("LOWER(name) LIKE LOWER('%%' || $%d || '%%')", f.Name)
	}
	if f.CreatedBy != "" {
		addWhere("created_by = $%d", f.CreatedBy)
	}
	if f.DateFrom != nil {
		addWhere("created_at >= $%d", *f.DateFrom)
	}
	if f.DateTo != nil {
		addWhere("created_at <= $%d", *f.DateTo)
	}
	if f.OwnerID != "" {
		ownerClause := fmt.Sprintf("(created_by = $%d OR EXISTS (SELECT 1 FROM campaign_shares cs WHERE cs.campaign_id = campaigns.id AND cs.user_id = $%d))", idx, idx+1)
		args = append(args, f.OwnerID, f.OwnerID)
		idx += 2
		where = append(where, ownerClause)
	}

	whereClause := ""
	if len(where) > 0 {
		whereClause = "WHERE " + strings.Join(where, " AND ")
	}

	// Count total.
	countQ := fmt.Sprintf("SELECT COUNT(*) FROM campaigns %s", whereClause)
	var total int
	if err := r.db.QueryRowContext(ctx, countQ, args...).Scan(&total); err != nil {
		return CampaignListResult{}, fmt.Errorf("campaigns: list count: %w", err)
	}

	if f.Page < 1 {
		f.Page = 1
	}
	if f.PerPage < 1 {
		f.PerPage = 25
	}
	offset := (f.Page - 1) * f.PerPage

	args = append(args, f.PerPage, offset)
	q := fmt.Sprintf(`SELECT %s FROM campaigns %s ORDER BY created_at DESC LIMIT $%d OFFSET $%d`,
		campaignColumns, whereClause, idx, idx+1)

	rows, err := r.db.QueryContext(ctx, q, args...)
	if err != nil {
		return CampaignListResult{}, fmt.Errorf("campaigns: list: %w", err)
	}
	defer rows.Close()

	var campaigns []Campaign
	for rows.Next() {
		c, err := scanCampaign(rows)
		if err != nil {
			return CampaignListResult{}, fmt.Errorf("campaigns: list scan: %w", err)
		}
		campaigns = append(campaigns, c)
	}
	if campaigns == nil {
		campaigns = []Campaign{}
	}

	return CampaignListResult{Campaigns: campaigns, Total: total}, nil
}

// ---------- State Transitions ----------

// TransitionState updates the campaign state atomically within a transaction.
func (r *CampaignRepository) TransitionState(ctx context.Context, tx *sql.Tx, id, fromState, toState string, actorID *string, reason string) error {
	// Update campaign state.
	res, err := tx.ExecContext(ctx,
		`UPDATE campaigns SET current_state = $1, state_changed_at = now()
		 WHERE id = $2 AND current_state = $3 AND deleted_at IS NULL`,
		toState, id, fromState)
	if err != nil {
		return fmt.Errorf("campaigns: transition state: %w", err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return fmt.Errorf("campaigns: transition state: concurrent modification or not found")
	}

	// Update timestamp fields based on target state.
	switch toState {
	case "active":
		_, _ = tx.ExecContext(ctx, `UPDATE campaigns SET launched_at = COALESCE(launched_at, now()) WHERE id = $1`, id)
	case "completed":
		_, _ = tx.ExecContext(ctx, `UPDATE campaigns SET completed_at = now() WHERE id = $1`, id)
	case "archived":
		_, _ = tx.ExecContext(ctx, `UPDATE campaigns SET archived_at = now() WHERE id = $1`, id)
	case "draft":
		// On unlock/reject/rollback, clear approval fields.
		_, _ = tx.ExecContext(ctx, `UPDATE campaigns SET approved_by = NULL, approval_comment = NULL WHERE id = $1`, id)
	}

	// Record transition in history.
	_, err = tx.ExecContext(ctx,
		`INSERT INTO campaign_state_transitions (id, campaign_id, from_state, to_state, actor_id, reason)
		 VALUES ($1, $2, $3, $4, $5, $6)`,
		uuid.New().String(), id, fromState, toState, actorID, reason)
	if err != nil {
		return fmt.Errorf("campaigns: record transition: %w", err)
	}

	return nil
}

// SetApproval sets the approved_by and approval_comment fields.
func (r *CampaignRepository) SetApproval(ctx context.Context, tx *sql.Tx, campaignID, approvedBy, comment string) error {
	_, err := tx.ExecContext(ctx,
		`UPDATE campaigns SET approved_by = $1, approval_comment = $2 WHERE id = $3`,
		approvedBy, comment, campaignID)
	if err != nil {
		return fmt.Errorf("campaigns: set approval: %w", err)
	}
	return nil
}

// ListTransitions returns state transition history for a campaign.
func (r *CampaignRepository) ListTransitions(ctx context.Context, campaignID string) ([]CampaignStateTransition, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT id, campaign_id, from_state, to_state, actor_id, reason, created_at
		 FROM campaign_state_transitions WHERE campaign_id = $1 ORDER BY created_at ASC`, campaignID)
	if err != nil {
		return nil, fmt.Errorf("campaigns: list transitions: %w", err)
	}
	defer rows.Close()

	var result []CampaignStateTransition
	for rows.Next() {
		var t CampaignStateTransition
		if err := rows.Scan(&t.ID, &t.CampaignID, &t.FromState, &t.ToState, &t.ActorID, &t.Reason, &t.CreatedAt); err != nil {
			return nil, fmt.Errorf("campaigns: scan transition: %w", err)
		}
		result = append(result, t)
	}
	if result == nil {
		result = []CampaignStateTransition{}
	}
	return result, nil
}

// ---------- Template Variants ----------

// CreateTemplateVariant adds a template variant to a campaign.
func (r *CampaignRepository) CreateTemplateVariant(ctx context.Context, v CampaignTemplateVariant) (CampaignTemplateVariant, error) {
	v.ID = uuid.New().String()
	err := r.db.QueryRowContext(ctx,
		`INSERT INTO campaign_template_variants (id, campaign_id, template_id, split_ratio, label)
		 VALUES ($1,$2,$3,$4,$5) RETURNING id, campaign_id, template_id, split_ratio, label, created_at`,
		v.ID, v.CampaignID, v.TemplateID, v.SplitRatio, v.Label,
	).Scan(&v.ID, &v.CampaignID, &v.TemplateID, &v.SplitRatio, &v.Label, &v.CreatedAt)
	if err != nil {
		return CampaignTemplateVariant{}, fmt.Errorf("campaigns: create template variant: %w", err)
	}
	return v, nil
}

// ListTemplateVariants returns all template variants for a campaign.
func (r *CampaignRepository) ListTemplateVariants(ctx context.Context, campaignID string) ([]CampaignTemplateVariant, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT id, campaign_id, template_id, split_ratio, label, created_at
		 FROM campaign_template_variants WHERE campaign_id = $1 ORDER BY label`, campaignID)
	if err != nil {
		return nil, fmt.Errorf("campaigns: list template variants: %w", err)
	}
	defer rows.Close()

	var result []CampaignTemplateVariant
	for rows.Next() {
		var v CampaignTemplateVariant
		if err := rows.Scan(&v.ID, &v.CampaignID, &v.TemplateID, &v.SplitRatio, &v.Label, &v.CreatedAt); err != nil {
			return nil, fmt.Errorf("campaigns: scan template variant: %w", err)
		}
		result = append(result, v)
	}
	if result == nil {
		result = []CampaignTemplateVariant{}
	}
	return result, nil
}

// DeleteTemplateVariants removes all template variants for a campaign.
func (r *CampaignRepository) DeleteTemplateVariants(ctx context.Context, campaignID string) error {
	_, err := r.db.ExecContext(ctx, `DELETE FROM campaign_template_variants WHERE campaign_id = $1`, campaignID)
	if err != nil {
		return fmt.Errorf("campaigns: delete template variants: %w", err)
	}
	return nil
}

// ---------- Send Windows ----------

// CreateSendWindow adds a send window to a campaign.
func (r *CampaignRepository) CreateSendWindow(ctx context.Context, w CampaignSendWindow) (CampaignSendWindow, error) {
	w.ID = uuid.New().String()
	daysJSON, err := json.Marshal(w.Days)
	if err != nil {
		return CampaignSendWindow{}, fmt.Errorf("campaigns: create send window: marshal days: %w", err)
	}
	err = r.db.QueryRowContext(ctx,
		`INSERT INTO campaign_send_windows (id, campaign_id, days, start_time, end_time, timezone)
		 VALUES ($1,$2,$3,$4,$5,$6) RETURNING id, campaign_id, days, start_time, end_time, timezone, created_at`,
		w.ID, w.CampaignID, daysJSON, w.StartTime, w.EndTime, w.Timezone,
	).Scan(&w.ID, &w.CampaignID, &daysJSON, &w.StartTime, &w.EndTime, &w.Timezone, &w.CreatedAt)
	if err != nil {
		return CampaignSendWindow{}, fmt.Errorf("campaigns: create send window: %w", err)
	}
	_ = json.Unmarshal(daysJSON, &w.Days)
	return w, nil
}

// ListSendWindows returns all send windows for a campaign.
func (r *CampaignRepository) ListSendWindows(ctx context.Context, campaignID string) ([]CampaignSendWindow, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT id, campaign_id, days, start_time, end_time, timezone, created_at
		 FROM campaign_send_windows WHERE campaign_id = $1 ORDER BY start_time`, campaignID)
	if err != nil {
		return nil, fmt.Errorf("campaigns: list send windows: %w", err)
	}
	defer rows.Close()

	var result []CampaignSendWindow
	for rows.Next() {
		var w CampaignSendWindow
		var daysJSON []byte
		if err := rows.Scan(&w.ID, &w.CampaignID, &daysJSON, &w.StartTime, &w.EndTime, &w.Timezone, &w.CreatedAt); err != nil {
			return nil, fmt.Errorf("campaigns: scan send window: %w", err)
		}
		_ = json.Unmarshal(daysJSON, &w.Days)
		if w.Days == nil {
			w.Days = []string{}
		}
		result = append(result, w)
	}
	if result == nil {
		result = []CampaignSendWindow{}
	}
	return result, nil
}

// DeleteSendWindows removes all send windows for a campaign.
func (r *CampaignRepository) DeleteSendWindows(ctx context.Context, campaignID string) error {
	_, err := r.db.ExecContext(ctx, `DELETE FROM campaign_send_windows WHERE campaign_id = $1`, campaignID)
	if err != nil {
		return fmt.Errorf("campaigns: delete send windows: %w", err)
	}
	return nil
}

// ---------- Build Logs ----------

// CreateBuildLog inserts a new build log entry.
func (r *CampaignRepository) CreateBuildLog(ctx context.Context, l CampaignBuildLog) (CampaignBuildLog, error) {
	l.ID = uuid.New().String()
	err := r.db.QueryRowContext(ctx,
		`INSERT INTO campaign_build_logs (id, campaign_id, step_name, step_order, status, started_at, completed_at, error_details)
		 VALUES ($1,$2,$3,$4,$5,$6,$7,$8)
		 RETURNING id, campaign_id, step_name, step_order, status, started_at, completed_at, error_details, created_at`,
		l.ID, l.CampaignID, l.StepName, l.StepOrder, l.Status, l.StartedAt, l.CompletedAt, l.ErrorDetails,
	).Scan(&l.ID, &l.CampaignID, &l.StepName, &l.StepOrder, &l.Status, &l.StartedAt, &l.CompletedAt, &l.ErrorDetails, &l.CreatedAt)
	if err != nil {
		return CampaignBuildLog{}, fmt.Errorf("campaigns: create build log: %w", err)
	}
	return l, nil
}

// UpdateBuildLogStatus updates the status and timestamps of a build log entry.
func (r *CampaignRepository) UpdateBuildLogStatus(ctx context.Context, id, status string, completedAt *time.Time, errorDetails *string) error {
	_, err := r.db.ExecContext(ctx,
		`UPDATE campaign_build_logs SET status = $1, completed_at = $2, error_details = $3 WHERE id = $4`,
		status, completedAt, errorDetails, id)
	if err != nil {
		return fmt.Errorf("campaigns: update build log: %w", err)
	}
	return nil
}

// ListBuildLogs returns build logs for a campaign ordered by step.
func (r *CampaignRepository) ListBuildLogs(ctx context.Context, campaignID string) ([]CampaignBuildLog, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT id, campaign_id, step_name, step_order, status, started_at, completed_at, error_details, created_at
		 FROM campaign_build_logs WHERE campaign_id = $1 ORDER BY step_order ASC`, campaignID)
	if err != nil {
		return nil, fmt.Errorf("campaigns: list build logs: %w", err)
	}
	defer rows.Close()

	var result []CampaignBuildLog
	for rows.Next() {
		var l CampaignBuildLog
		if err := rows.Scan(&l.ID, &l.CampaignID, &l.StepName, &l.StepOrder, &l.Status, &l.StartedAt, &l.CompletedAt, &l.ErrorDetails, &l.CreatedAt); err != nil {
			return nil, fmt.Errorf("campaigns: scan build log: %w", err)
		}
		result = append(result, l)
	}
	if result == nil {
		result = []CampaignBuildLog{}
	}
	return result, nil
}

// DeleteBuildLogs removes all build logs for a campaign (for clean rebuild).
func (r *CampaignRepository) DeleteBuildLogs(ctx context.Context, campaignID string) error {
	_, err := r.db.ExecContext(ctx, `DELETE FROM campaign_build_logs WHERE campaign_id = $1`, campaignID)
	if err != nil {
		return fmt.Errorf("campaigns: delete build logs: %w", err)
	}
	return nil
}

// ---------- Campaign Emails ----------

// CreateCampaignEmail inserts a new email dispatch record.
func (r *CampaignRepository) CreateCampaignEmail(ctx context.Context, e CampaignEmail) (CampaignEmail, error) {
	e.ID = uuid.New().String()
	err := r.db.QueryRowContext(ctx,
		`INSERT INTO campaign_emails (id, campaign_id, target_id, variant_id, smtp_config_id, status, send_order_position, variant_label)
		 VALUES ($1,$2,$3,$4,$5,$6,$7,$8)
		 RETURNING id, campaign_id, target_id, variant_id, smtp_config_id, status, message_id,
		           sent_at, delivered_at, bounced_at, bounce_type, bounce_message, retry_count, next_retry_at,
		           send_order_position, variant_label, created_at, updated_at`,
		e.ID, e.CampaignID, e.TargetID, e.VariantID, e.SMTPConfigID, e.Status, e.SendOrderPosition, e.VariantLabel,
	).Scan(&e.ID, &e.CampaignID, &e.TargetID, &e.VariantID, &e.SMTPConfigID, &e.Status, &e.MessageID,
		&e.SentAt, &e.DeliveredAt, &e.BouncedAt, &e.BounceType, &e.BounceMessage, &e.RetryCount, &e.NextRetryAt,
		&e.SendOrderPosition, &e.VariantLabel, &e.CreatedAt, &e.UpdatedAt)
	if err != nil {
		return CampaignEmail{}, fmt.Errorf("campaigns: create email: %w", err)
	}
	return e, nil
}

// CreateCampaignEmailsBatch inserts multiple email records efficiently.
func (r *CampaignRepository) CreateCampaignEmailsBatch(ctx context.Context, emails []CampaignEmail) error {
	if len(emails) == 0 {
		return nil
	}

	// Build batch insert using multi-row VALUES.
	var b strings.Builder
	b.WriteString(`INSERT INTO campaign_emails (id, campaign_id, target_id, variant_id, smtp_config_id, status, send_order_position, variant_label, tracking_token) VALUES `)

	args := make([]any, 0, len(emails)*9)
	for i, e := range emails {
		if i > 0 {
			b.WriteString(", ")
		}
		base := i * 9
		fmt.Fprintf(&b, "($%d,$%d,$%d,$%d,$%d,$%d,$%d,$%d,$%d)", base+1, base+2, base+3, base+4, base+5, base+6, base+7, base+8, base+9)
		id := uuid.New().String()
		emails[i].ID = id
		args = append(args, id, e.CampaignID, e.TargetID, e.VariantID, e.SMTPConfigID, e.Status, e.SendOrderPosition, e.VariantLabel, e.TrackingToken)
	}

	_, err := r.db.ExecContext(ctx, b.String(), args...)
	if err != nil {
		return fmt.Errorf("campaigns: create emails batch: %w", err)
	}
	return nil
}

// UpdateEmailStatus updates an email's status and related timestamps.
func (r *CampaignRepository) UpdateEmailStatus(ctx context.Context, id, status string, messageID *string, sentAt, deliveredAt, bouncedAt *time.Time, bounceType, bounceMessage *string) error {
	_, err := r.db.ExecContext(ctx,
		`UPDATE campaign_emails SET status = $1, message_id = COALESCE($2, message_id),
		 sent_at = COALESCE($3, sent_at), delivered_at = COALESCE($4, delivered_at),
		 bounced_at = COALESCE($5, bounced_at), bounce_type = COALESCE($6, bounce_type),
		 bounce_message = COALESCE($7, bounce_message)
		 WHERE id = $8`,
		status, messageID, sentAt, deliveredAt, bouncedAt, bounceType, bounceMessage, id)
	if err != nil {
		return fmt.Errorf("campaigns: update email status: %w", err)
	}
	return nil
}

// UpdateEmailRetry updates retry_count, next_retry_at, and optionally the SMTP config.
func (r *CampaignRepository) UpdateEmailRetry(ctx context.Context, id string, retryCount int, nextRetryAt *time.Time, smtpConfigID *string) error {
	_, err := r.db.ExecContext(ctx,
		`UPDATE campaign_emails SET retry_count = $1, next_retry_at = $2,
		 smtp_config_id = COALESCE($3, smtp_config_id), status = CASE WHEN $4::timestamptz IS NOT NULL THEN 'deferred' ELSE status END
		 WHERE id = $5`,
		retryCount, nextRetryAt, smtpConfigID, nextRetryAt, id)
	if err != nil {
		return fmt.Errorf("campaigns: update email retry: %w", err)
	}
	return nil
}

// GetEmailsForRetry returns emails whose next_retry_at is in the past.
func (r *CampaignRepository) GetEmailsForRetry(ctx context.Context, campaignID string) ([]CampaignEmail, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT id, campaign_id, target_id, variant_id, smtp_config_id, status, message_id,
		        sent_at, delivered_at, bounced_at, bounce_type, bounce_message, retry_count, next_retry_at,
		        send_order_position, variant_label, tracking_token, created_at, updated_at
		 FROM campaign_emails
		 WHERE campaign_id = $1 AND next_retry_at IS NOT NULL AND next_retry_at <= now()
		   AND status IN ('deferred', 'failed')
		 ORDER BY next_retry_at ASC`, campaignID)
	if err != nil {
		return nil, fmt.Errorf("campaigns: get emails for retry: %w", err)
	}
	defer rows.Close()

	var result []CampaignEmail
	for rows.Next() {
		var e CampaignEmail
		if err := rows.Scan(&e.ID, &e.CampaignID, &e.TargetID, &e.VariantID, &e.SMTPConfigID, &e.Status, &e.MessageID,
			&e.SentAt, &e.DeliveredAt, &e.BouncedAt, &e.BounceType, &e.BounceMessage, &e.RetryCount, &e.NextRetryAt,
			&e.SendOrderPosition, &e.VariantLabel, &e.TrackingToken, &e.CreatedAt, &e.UpdatedAt); err != nil {
			return nil, fmt.Errorf("campaigns: scan retry email: %w", err)
		}
		result = append(result, e)
	}
	return result, nil
}

// ListCampaignEmails returns all emails for a campaign.
func (r *CampaignRepository) ListCampaignEmails(ctx context.Context, campaignID string) ([]CampaignEmail, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT id, campaign_id, target_id, variant_id, smtp_config_id, status, message_id,
		        sent_at, delivered_at, bounced_at, bounce_type, bounce_message, retry_count, next_retry_at,
		        send_order_position, variant_label, created_at, updated_at
		 FROM campaign_emails WHERE campaign_id = $1 ORDER BY send_order_position ASC NULLS LAST`, campaignID)
	if err != nil {
		return nil, fmt.Errorf("campaigns: list emails: %w", err)
	}
	defer rows.Close()

	var result []CampaignEmail
	for rows.Next() {
		var e CampaignEmail
		if err := rows.Scan(&e.ID, &e.CampaignID, &e.TargetID, &e.VariantID, &e.SMTPConfigID, &e.Status, &e.MessageID,
			&e.SentAt, &e.DeliveredAt, &e.BouncedAt, &e.BounceType, &e.BounceMessage, &e.RetryCount, &e.NextRetryAt,
			&e.SendOrderPosition, &e.VariantLabel, &e.CreatedAt, &e.UpdatedAt); err != nil {
			return nil, fmt.Errorf("campaigns: scan email: %w", err)
		}
		result = append(result, e)
	}
	return result, nil
}

// GetCampaignEmailByID returns a single email by ID.
func (r *CampaignRepository) GetCampaignEmailByID(ctx context.Context, id string) (CampaignEmail, error) {
	var e CampaignEmail
	err := r.db.QueryRowContext(ctx,
		`SELECT id, campaign_id, target_id, variant_id, smtp_config_id, status, message_id,
		        sent_at, delivered_at, bounced_at, bounce_type, bounce_message, retry_count, next_retry_at,
		        send_order_position, variant_label, created_at, updated_at
		 FROM campaign_emails WHERE id = $1`, id,
	).Scan(&e.ID, &e.CampaignID, &e.TargetID, &e.VariantID, &e.SMTPConfigID, &e.Status, &e.MessageID,
		&e.SentAt, &e.DeliveredAt, &e.BouncedAt, &e.BounceType, &e.BounceMessage, &e.RetryCount, &e.NextRetryAt,
		&e.SendOrderPosition, &e.VariantLabel, &e.CreatedAt, &e.UpdatedAt)
	if err != nil {
		return CampaignEmail{}, fmt.Errorf("campaigns: get email by id: %w", err)
	}
	return e, nil
}

// DeleteCampaignEmails removes all emails for a campaign.
func (r *CampaignRepository) DeleteCampaignEmails(ctx context.Context, campaignID string) error {
	_, err := r.db.ExecContext(ctx, `DELETE FROM campaign_emails WHERE campaign_id = $1`, campaignID)
	if err != nil {
		return fmt.Errorf("campaigns: delete emails: %w", err)
	}
	return nil
}

// CancelUnsentEmails marks all queued emails for a campaign as cancelled.
func (r *CampaignRepository) CancelUnsentEmails(ctx context.Context, campaignID string) (int64, error) {
	res, err := r.db.ExecContext(ctx,
		`UPDATE campaign_emails SET status = 'cancelled' WHERE campaign_id = $1 AND status = 'queued'`, campaignID)
	if err != nil {
		return 0, fmt.Errorf("campaigns: cancel unsent emails: %w", err)
	}
	n, _ := res.RowsAffected()
	return n, nil
}

// CountEmailsByStatus returns email counts grouped by status for a campaign.
func (r *CampaignRepository) CountEmailsByStatus(ctx context.Context, campaignID string) (map[string]int, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT status, COUNT(*) FROM campaign_emails WHERE campaign_id = $1 GROUP BY status`, campaignID)
	if err != nil {
		return nil, fmt.Errorf("campaigns: count emails: %w", err)
	}
	defer rows.Close()

	counts := make(map[string]int)
	for rows.Next() {
		var status string
		var count int
		if err := rows.Scan(&status, &count); err != nil {
			return nil, fmt.Errorf("campaigns: scan email count: %w", err)
		}
		counts[status] = count
	}
	return counts, nil
}

// GetNextQueuedEmail returns the next queued email by send order position.
func (r *CampaignRepository) GetNextQueuedEmail(ctx context.Context, campaignID string) (CampaignEmail, error) {
	var e CampaignEmail
	err := r.db.QueryRowContext(ctx,
		`SELECT id, campaign_id, target_id, variant_id, smtp_config_id, status, message_id,
		        sent_at, delivered_at, bounced_at, bounce_type, bounce_message, retry_count, next_retry_at,
		        send_order_position, variant_label, tracking_token, created_at, updated_at
		 FROM campaign_emails WHERE campaign_id = $1 AND status = 'queued'
		 ORDER BY send_order_position ASC NULLS LAST, created_at ASC LIMIT 1`, campaignID,
	).Scan(&e.ID, &e.CampaignID, &e.TargetID, &e.VariantID, &e.SMTPConfigID, &e.Status, &e.MessageID,
		&e.SentAt, &e.DeliveredAt, &e.BouncedAt, &e.BounceType, &e.BounceMessage, &e.RetryCount, &e.NextRetryAt,
		&e.SendOrderPosition, &e.VariantLabel, &e.TrackingToken, &e.CreatedAt, &e.UpdatedAt)
	if err == sql.ErrNoRows {
		return CampaignEmail{}, fmt.Errorf("campaigns: no queued emails")
	}
	if err != nil {
		return CampaignEmail{}, fmt.Errorf("campaigns: get next queued email: %w", err)
	}
	return e, nil
}

// ---------- Target Snapshots ----------

// CreateTargetSnapshot inserts a snapshot row.
func (r *CampaignRepository) CreateTargetSnapshot(ctx context.Context, s CampaignTargetsSnapshot) error {
	s.ID = uuid.New().String()
	_, err := r.db.ExecContext(ctx,
		`INSERT INTO campaign_targets_snapshot (id, campaign_id, target_id, variant_label, send_order_position, is_canary)
		 VALUES ($1,$2,$3,$4,$5,$6)`,
		s.ID, s.CampaignID, s.TargetID, s.VariantLabel, s.SendOrderPosition, s.IsCanary)
	if err != nil {
		return fmt.Errorf("campaigns: create target snapshot: %w", err)
	}
	return nil
}

// ListTargetSnapshots returns all snapshot rows for a campaign.
func (r *CampaignRepository) ListTargetSnapshots(ctx context.Context, campaignID string) ([]CampaignTargetsSnapshot, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT id, campaign_id, target_id, variant_label, send_order_position, is_canary, snapshotted_at
		 FROM campaign_targets_snapshot WHERE campaign_id = $1 ORDER BY send_order_position ASC NULLS LAST`, campaignID)
	if err != nil {
		return nil, fmt.Errorf("campaigns: list target snapshots: %w", err)
	}
	defer rows.Close()

	var result []CampaignTargetsSnapshot
	for rows.Next() {
		var s CampaignTargetsSnapshot
		if err := rows.Scan(&s.ID, &s.CampaignID, &s.TargetID, &s.VariantLabel, &s.SendOrderPosition, &s.IsCanary, &s.SnapshottedAt); err != nil {
			return nil, fmt.Errorf("campaigns: scan snapshot: %w", err)
		}
		result = append(result, s)
	}
	if result == nil {
		result = []CampaignTargetsSnapshot{}
	}
	return result, nil
}

// DeleteTargetSnapshots removes all snapshots for a campaign.
func (r *CampaignRepository) DeleteTargetSnapshots(ctx context.Context, campaignID string) error {
	_, err := r.db.ExecContext(ctx, `DELETE FROM campaign_targets_snapshot WHERE campaign_id = $1`, campaignID)
	if err != nil {
		return fmt.Errorf("campaigns: delete target snapshots: %w", err)
	}
	return nil
}

// SetTrackingToken sets the tracking token for a campaign target.
func (r *CampaignRepository) SetTrackingToken(ctx context.Context, campaignID, targetID, token string) error {
	_, err := r.db.ExecContext(ctx,
		`UPDATE campaign_targets SET tracking_token = $1 WHERE campaign_id = $2 AND target_id = $3`,
		token, campaignID, targetID)
	if err != nil {
		return fmt.Errorf("campaigns: set tracking token: %w", err)
	}
	return nil
}

// SetTrackingTokenOnEmail sets the tracking token on a campaign email record.
func (r *CampaignRepository) SetTrackingTokenOnEmail(ctx context.Context, emailID, token string) error {
	_, err := r.db.ExecContext(ctx,
		`UPDATE campaign_emails SET tracking_token = $1 WHERE id = $2`,
		token, emailID)
	if err != nil {
		return fmt.Errorf("campaigns: set email tracking token: %w", err)
	}
	return nil
}

// GetTrackingToken retrieves the tracking token for a campaign target.
func (r *CampaignRepository) GetTrackingToken(ctx context.Context, campaignID, targetID string) (string, error) {
	var token sql.NullString
	err := r.db.QueryRowContext(ctx,
		`SELECT tracking_token FROM campaign_targets WHERE campaign_id = $1 AND target_id = $2`,
		campaignID, targetID).Scan(&token)
	if err != nil {
		return "", fmt.Errorf("campaigns: get tracking token: %w", err)
	}
	if !token.Valid {
		return "", nil
	}
	return token.String, nil
}

// ---------- Variant Assignments ----------

// CreateVariantAssignment assigns a target to a template variant.
func (r *CampaignRepository) CreateVariantAssignment(ctx context.Context, campaignID, targetID, variantID string) error {
	_, err := r.db.ExecContext(ctx,
		`INSERT INTO campaign_target_variant_assignments (campaign_id, target_id, variant_id) VALUES ($1,$2,$3)`,
		campaignID, targetID, variantID)
	if err != nil {
		return fmt.Errorf("campaigns: create variant assignment: %w", err)
	}
	return nil
}

// DeleteVariantAssignments removes all variant assignments for a campaign.
func (r *CampaignRepository) DeleteVariantAssignments(ctx context.Context, campaignID string) error {
	_, err := r.db.ExecContext(ctx, `DELETE FROM campaign_target_variant_assignments WHERE campaign_id = $1`, campaignID)
	if err != nil {
		return fmt.Errorf("campaigns: delete variant assignments: %w", err)
	}
	return nil
}

// ---------- Campaign Config Templates ----------

// CreateConfigTemplate inserts a reusable campaign config template.
func (r *CampaignRepository) CreateConfigTemplate(ctx context.Context, t CampaignConfigTemplate) (CampaignConfigTemplate, error) {
	t.ID = uuid.New().String()
	cfgJSON, err := json.Marshal(t.ConfigJSON)
	if err != nil {
		return CampaignConfigTemplate{}, fmt.Errorf("campaigns: create config template: marshal: %w", err)
	}
	err = r.db.QueryRowContext(ctx,
		`INSERT INTO campaign_config_templates (id, name, description, config_json, created_by)
		 VALUES ($1,$2,$3,$4,$5) RETURNING id, name, description, config_json, created_by, created_at, updated_at`,
		t.ID, t.Name, t.Description, cfgJSON, t.CreatedBy,
	).Scan(&t.ID, &t.Name, &t.Description, &cfgJSON, &t.CreatedBy, &t.CreatedAt, &t.UpdatedAt)
	if err != nil {
		return CampaignConfigTemplate{}, fmt.Errorf("campaigns: create config template: %w", err)
	}
	_ = json.Unmarshal(cfgJSON, &t.ConfigJSON)
	return t, nil
}

// GetConfigTemplateByID returns a config template by ID.
func (r *CampaignRepository) GetConfigTemplateByID(ctx context.Context, id string) (CampaignConfigTemplate, error) {
	var t CampaignConfigTemplate
	var cfgJSON []byte
	err := r.db.QueryRowContext(ctx,
		`SELECT id, name, description, config_json, created_by, created_at, updated_at
		 FROM campaign_config_templates WHERE id = $1`, id,
	).Scan(&t.ID, &t.Name, &t.Description, &cfgJSON, &t.CreatedBy, &t.CreatedAt, &t.UpdatedAt)
	if err == sql.ErrNoRows {
		return CampaignConfigTemplate{}, fmt.Errorf("campaigns: config template not found")
	}
	if err != nil {
		return CampaignConfigTemplate{}, fmt.Errorf("campaigns: get config template: %w", err)
	}
	_ = json.Unmarshal(cfgJSON, &t.ConfigJSON)
	if t.ConfigJSON == nil {
		t.ConfigJSON = make(map[string]any)
	}
	return t, nil
}

// ListConfigTemplates returns all campaign config templates.
func (r *CampaignRepository) ListConfigTemplates(ctx context.Context) ([]CampaignConfigTemplate, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT id, name, description, config_json, created_by, created_at, updated_at
		 FROM campaign_config_templates ORDER BY name`)
	if err != nil {
		return nil, fmt.Errorf("campaigns: list config templates: %w", err)
	}
	defer rows.Close()

	var result []CampaignConfigTemplate
	for rows.Next() {
		var t CampaignConfigTemplate
		var cfgJSON []byte
		if err := rows.Scan(&t.ID, &t.Name, &t.Description, &cfgJSON, &t.CreatedBy, &t.CreatedAt, &t.UpdatedAt); err != nil {
			return nil, fmt.Errorf("campaigns: scan config template: %w", err)
		}
		_ = json.Unmarshal(cfgJSON, &t.ConfigJSON)
		if t.ConfigJSON == nil {
			t.ConfigJSON = make(map[string]any)
		}
		result = append(result, t)
	}
	if result == nil {
		result = []CampaignConfigTemplate{}
	}
	return result, nil
}

// UpdateConfigTemplate updates name, description, and config_json.
func (r *CampaignRepository) UpdateConfigTemplate(ctx context.Context, id string, name, description string, configJSON map[string]any) (CampaignConfigTemplate, error) {
	cfgBytes, err := json.Marshal(configJSON)
	if err != nil {
		return CampaignConfigTemplate{}, fmt.Errorf("campaigns: update config template: marshal: %w", err)
	}
	var t CampaignConfigTemplate
	var cfgJSON2 []byte
	err = r.db.QueryRowContext(ctx,
		`UPDATE campaign_config_templates SET name = $1, description = $2, config_json = $3
		 WHERE id = $4 RETURNING id, name, description, config_json, created_by, created_at, updated_at`,
		name, description, cfgBytes, id,
	).Scan(&t.ID, &t.Name, &t.Description, &cfgJSON2, &t.CreatedBy, &t.CreatedAt, &t.UpdatedAt)
	if err == sql.ErrNoRows {
		return CampaignConfigTemplate{}, fmt.Errorf("campaigns: config template not found")
	}
	if err != nil {
		return CampaignConfigTemplate{}, fmt.Errorf("campaigns: update config template: %w", err)
	}
	_ = json.Unmarshal(cfgJSON2, &t.ConfigJSON)
	return t, nil
}

// DeleteConfigTemplate removes a config template by ID.
func (r *CampaignRepository) DeleteConfigTemplate(ctx context.Context, id string) error {
	res, err := r.db.ExecContext(ctx, `DELETE FROM campaign_config_templates WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("campaigns: delete config template: %w", err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return fmt.Errorf("campaigns: config template not found")
	}
	return nil
}

// ---------- Domain Exclusivity ----------

// IsDomainInUse checks if a domain is used by any campaign in active-like states.
// Excludes the campaign identified by excludeID (used when updating own campaign).
func (r *CampaignRepository) IsDomainInUse(ctx context.Context, domainID, excludeID string) (bool, error) {
	var count int
	err := r.db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM campaigns
		 WHERE endpoint_domain_id = $1
		   AND current_state IN ('building', 'ready', 'active', 'paused')
		   AND deleted_at IS NULL
		   AND id != $2`, domainID, excludeID,
	).Scan(&count)
	if err != nil {
		return false, fmt.Errorf("campaigns: domain in use check: %w", err)
	}
	return count > 0, nil
}

// CountActiveCampaigns returns the number of campaigns in active/paused state.
func (r *CampaignRepository) CountActiveCampaigns(ctx context.Context) (int, error) {
	var count int
	err := r.db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM campaigns WHERE current_state IN ('active', 'paused') AND deleted_at IS NULL`,
	).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("campaigns: count active: %w", err)
	}
	return count, nil
}

// ListReadyForAutoLaunch returns campaigns in Ready state with scheduled_launch_at in the past.
func (r *CampaignRepository) ListReadyForAutoLaunch(ctx context.Context) ([]Campaign, error) {
	q := fmt.Sprintf(`SELECT %s FROM campaigns
		WHERE current_state = 'ready'
		  AND scheduled_launch_at IS NOT NULL
		  AND scheduled_launch_at <= now()
		  AND deleted_at IS NULL`, campaignColumns)

	rows, err := r.db.QueryContext(ctx, q)
	if err != nil {
		return nil, fmt.Errorf("campaigns: list ready for auto-launch: %w", err)
	}
	defer rows.Close()

	var result []Campaign
	for rows.Next() {
		c, err := scanCampaign(rows)
		if err != nil {
			return nil, fmt.Errorf("campaigns: scan auto-launch: %w", err)
		}
		result = append(result, c)
	}
	if result == nil {
		result = []Campaign{}
	}
	return result, nil
}

// ListByState returns all campaigns in the given state.
func (r *CampaignRepository) ListByState(ctx context.Context, state string) ([]Campaign, error) {
	q := fmt.Sprintf(`SELECT %s FROM campaigns
		WHERE current_state = $1
		  AND deleted_at IS NULL`, campaignColumns)

	rows, err := r.db.QueryContext(ctx, q, state)
	if err != nil {
		return nil, fmt.Errorf("campaigns: list by state: %w", err)
	}
	defer rows.Close()

	var result []Campaign
	for rows.Next() {
		c, err := scanCampaign(rows)
		if err != nil {
			return nil, fmt.Errorf("campaigns: scan by state: %w", err)
		}
		result = append(result, c)
	}
	if result == nil {
		result = []Campaign{}
	}
	return result, nil
}

// BeginTx starts a new database transaction.
func (r *CampaignRepository) BeginTx(ctx context.Context) (*sql.Tx, error) {
	return r.db.BeginTx(ctx, nil)
}

// ---------- Email Delivery Events ----------

// CreateDeliveryEvent inserts an email delivery event.
func (r *CampaignRepository) CreateDeliveryEvent(ctx context.Context, e EmailDeliveryEvent) (EmailDeliveryEvent, error) {
	e.ID = uuid.New().String()
	eventJSON, err := json.Marshal(e.EventData)
	if err != nil {
		return EmailDeliveryEvent{}, fmt.Errorf("campaigns: create delivery event: marshal: %w", err)
	}
	err = r.db.QueryRowContext(ctx,
		`INSERT INTO email_delivery_events (id, email_id, event_type, event_data)
		 VALUES ($1,$2,$3,$4) RETURNING id, email_id, event_type, event_data, created_at`,
		e.ID, e.EmailID, e.EventType, eventJSON,
	).Scan(&e.ID, &e.EmailID, &e.EventType, &eventJSON, &e.CreatedAt)
	if err != nil {
		return EmailDeliveryEvent{}, fmt.Errorf("campaigns: create delivery event: %w", err)
	}
	_ = json.Unmarshal(eventJSON, &e.EventData)
	return e, nil
}

// ListDeliveryEvents returns all delivery events for an email.
func (r *CampaignRepository) ListDeliveryEvents(ctx context.Context, emailID string) ([]EmailDeliveryEvent, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT id, email_id, event_type, event_data, created_at
		 FROM email_delivery_events WHERE email_id = $1 ORDER BY created_at ASC`, emailID)
	if err != nil {
		return nil, fmt.Errorf("campaigns: list delivery events: %w", err)
	}
	defer rows.Close()

	var result []EmailDeliveryEvent
	for rows.Next() {
		var ev EmailDeliveryEvent
		var eventJSON []byte
		if err := rows.Scan(&ev.ID, &ev.EmailID, &ev.EventType, &eventJSON, &ev.CreatedAt); err != nil {
			return nil, fmt.Errorf("campaigns: scan delivery event: %w", err)
		}
		_ = json.Unmarshal(eventJSON, &ev.EventData)
		if ev.EventData == nil {
			ev.EventData = make(map[string]any)
		}
		result = append(result, ev)
	}
	if result == nil {
		result = []EmailDeliveryEvent{}
	}
	return result, nil
}

// ---------- Campaign SMTP Send Counts ----------

// IncrementSMTPSendCount atomically increments the send count for a profile.
func (r *CampaignRepository) IncrementSMTPSendCount(ctx context.Context, campaignID, smtpProfileID string, isError bool) error {
	errorInc := 0
	if isError {
		errorInc = 1
	}
	_, err := r.db.ExecContext(ctx,
		`INSERT INTO campaign_smtp_send_counts (id, campaign_id, smtp_profile_id, send_count, error_count, last_sent_at)
		 VALUES (gen_random_uuid(), $1, $2, 1, $3, now())
		 ON CONFLICT (campaign_id, smtp_profile_id)
		 DO UPDATE SET send_count = campaign_smtp_send_counts.send_count + 1,
		               error_count = campaign_smtp_send_counts.error_count + $3,
		               last_sent_at = now()`,
		campaignID, smtpProfileID, errorInc)
	if err != nil {
		return fmt.Errorf("campaigns: increment smtp send count: %w", err)
	}
	return nil
}

// GetSMTPSendCounts returns send counts for all SMTP profiles in a campaign.
func (r *CampaignRepository) GetSMTPSendCounts(ctx context.Context, campaignID string) ([]CampaignSMTPSendCount, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT id, campaign_id, smtp_profile_id, send_count, error_count, last_sent_at, created_at, updated_at
		 FROM campaign_smtp_send_counts WHERE campaign_id = $1 ORDER BY send_count ASC`, campaignID)
	if err != nil {
		return nil, fmt.Errorf("campaigns: get smtp send counts: %w", err)
	}
	defer rows.Close()

	var result []CampaignSMTPSendCount
	for rows.Next() {
		var sc CampaignSMTPSendCount
		if err := rows.Scan(&sc.ID, &sc.CampaignID, &sc.SMTPProfileID, &sc.SendCount, &sc.ErrorCount, &sc.LastSentAt, &sc.CreatedAt, &sc.UpdatedAt); err != nil {
			return nil, fmt.Errorf("campaigns: scan smtp send count: %w", err)
		}
		result = append(result, sc)
	}
	if result == nil {
		result = []CampaignSMTPSendCount{}
	}
	return result, nil
}

// GetLeastUsedSMTPProfile returns the SMTP profile ID with the lowest send count for round-robin.
func (r *CampaignRepository) GetLeastUsedSMTPProfile(ctx context.Context, campaignID string, profileIDs []string) (string, error) {
	if len(profileIDs) == 0 {
		return "", fmt.Errorf("campaigns: no SMTP profiles provided")
	}
	if len(profileIDs) == 1 {
		return profileIDs[0], nil
	}

	// Use a subquery to find the profile with lowest count, defaulting to 0 for profiles with no sends yet.
	placeholders := make([]string, len(profileIDs))
	args := make([]any, 0, len(profileIDs)+1)
	args = append(args, campaignID)
	for i, pid := range profileIDs {
		placeholders[i] = fmt.Sprintf("$%d", i+2)
		args = append(args, pid)
	}

	q := fmt.Sprintf(`
		SELECT p.id
		FROM (SELECT unnest(ARRAY[%s]::text[]) AS id) p
		LEFT JOIN campaign_smtp_send_counts sc ON sc.smtp_profile_id = p.id AND sc.campaign_id = $1
		ORDER BY COALESCE(sc.send_count, 0) ASC, p.id ASC
		LIMIT 1`, strings.Join(placeholders, ","))

	var profileID string
	err := r.db.QueryRowContext(ctx, q, args...).Scan(&profileID)
	if err != nil {
		return "", fmt.Errorf("campaigns: get least used smtp profile: %w", err)
	}
	return profileID, nil
}

// CountEmailsByStatusAndVariant returns email counts grouped by status and variant_label.
func (r *CampaignRepository) CountEmailsByStatusAndVariant(ctx context.Context, campaignID string) (map[string]map[string]int, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT COALESCE(variant_label, ''), status, COUNT(*)
		 FROM campaign_emails WHERE campaign_id = $1
		 GROUP BY variant_label, status`, campaignID)
	if err != nil {
		return nil, fmt.Errorf("campaigns: count emails by variant: %w", err)
	}
	defer rows.Close()

	result := make(map[string]map[string]int)
	for rows.Next() {
		var variant, status string
		var count int
		if err := rows.Scan(&variant, &status, &count); err != nil {
			return nil, fmt.Errorf("campaigns: scan variant count: %w", err)
		}
		if result[variant] == nil {
			result[variant] = make(map[string]int)
		}
		result[variant][status] = count
	}
	return result, nil
}

// AllEmailsTerminal returns true if all campaign emails are in a terminal state (sent, delivered, bounced, failed, cancelled).
func (r *CampaignRepository) AllEmailsTerminal(ctx context.Context, campaignID string) (bool, error) {
	var nonTerminal int
	err := r.db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM campaign_emails
		 WHERE campaign_id = $1 AND status NOT IN ('sent', 'delivered', 'bounced', 'failed', 'cancelled')`, campaignID,
	).Scan(&nonTerminal)
	if err != nil {
		return false, fmt.Errorf("campaigns: all emails terminal: %w", err)
	}
	return nonTerminal == 0, nil
}

// GetLastSentPosition returns the highest send_order_position that has been sent.
func (r *CampaignRepository) GetLastSentPosition(ctx context.Context, campaignID string) (*int, error) {
	var pos *int
	err := r.db.QueryRowContext(ctx,
		`SELECT MAX(send_order_position) FROM campaign_emails
		 WHERE campaign_id = $1 AND status NOT IN ('queued', 'cancelled')`, campaignID,
	).Scan(&pos)
	if err != nil {
		return nil, fmt.Errorf("campaigns: get last sent position: %w", err)
	}
	return pos, nil
}

// SetCanaryTargets replaces canary target designations for a campaign.
func (r *CampaignRepository) SetCanaryTargets(ctx context.Context, campaignID string, targetIDs []string, designatedBy string) error {
	// Delete existing canaries.
	if _, err := r.db.ExecContext(ctx, `DELETE FROM campaign_canary_targets WHERE campaign_id = $1`, campaignID); err != nil {
		return fmt.Errorf("campaigns: clear canary targets: %w", err)
	}
	if len(targetIDs) == 0 {
		return nil
	}
	// Insert new canaries.
	var b strings.Builder
	b.WriteString(`INSERT INTO campaign_canary_targets (campaign_id, target_id, designated_by) VALUES `)
	args := make([]any, 0, len(targetIDs)*3)
	for i, tid := range targetIDs {
		if i > 0 {
			b.WriteString(", ")
		}
		base := i * 3
		fmt.Fprintf(&b, "($%d,$%d,$%d)", base+1, base+2, base+3)
		args = append(args, campaignID, tid, designatedBy)
	}
	if _, err := r.db.ExecContext(ctx, b.String(), args...); err != nil {
		return fmt.Errorf("campaigns: set canary targets: %w", err)
	}
	return nil
}

// ListCanaryTargets returns canary target IDs for a campaign.
func (r *CampaignRepository) ListCanaryTargets(ctx context.Context, campaignID string) ([]string, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT target_id FROM campaign_canary_targets WHERE campaign_id = $1`, campaignID)
	if err != nil {
		return nil, fmt.Errorf("campaigns: list canary targets: %w", err)
	}
	defer rows.Close()
	var ids []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, fmt.Errorf("campaigns: scan canary target: %w", err)
		}
		ids = append(ids, id)
	}
	return ids, nil
}

// GetTargetCountsBatch returns target email counts for a set of campaign IDs.
func (r *CampaignRepository) GetTargetCountsBatch(ctx context.Context, campaignIDs []string) (map[string]int, error) {
	if len(campaignIDs) == 0 {
		return map[string]int{}, nil
	}
	// Build placeholder list.
	placeholders := make([]string, len(campaignIDs))
	args := make([]any, len(campaignIDs))
	for i, id := range campaignIDs {
		placeholders[i] = fmt.Sprintf("$%d", i+1)
		args[i] = id
	}
	q := fmt.Sprintf(
		`SELECT campaign_id, COUNT(*) FROM campaign_emails WHERE campaign_id IN (%s) GROUP BY campaign_id`,
		strings.Join(placeholders, ","),
	)
	rows, err := r.db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, fmt.Errorf("campaigns: target counts batch: %w", err)
	}
	defer rows.Close()

	result := make(map[string]int)
	for rows.Next() {
		var cid string
		var count int
		if err := rows.Scan(&cid, &count); err != nil {
			return nil, fmt.Errorf("campaigns: scan target count: %w", err)
		}
		result[cid] = count
	}
	return result, nil
}

// GetCreatorNamesBatch returns display_name for user IDs from the users table.
func (r *CampaignRepository) GetCreatorNamesBatch(ctx context.Context, userIDs []string) (map[string]string, error) {
	if len(userIDs) == 0 {
		return map[string]string{}, nil
	}
	placeholders := make([]string, len(userIDs))
	args := make([]any, len(userIDs))
	for i, id := range userIDs {
		placeholders[i] = fmt.Sprintf("$%d", i+1)
		args[i] = id
	}
	q := fmt.Sprintf(
		`SELECT id, COALESCE(display_name, username) FROM users WHERE id IN (%s)`,
		strings.Join(placeholders, ","),
	)
	rows, err := r.db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, fmt.Errorf("campaigns: creator names batch: %w", err)
	}
	defer rows.Close()

	result := make(map[string]string)
	for rows.Next() {
		var uid, name string
		if err := rows.Scan(&uid, &name); err != nil {
			return nil, fmt.Errorf("campaigns: scan creator name: %w", err)
		}
		result[uid] = name
	}
	return result, nil
}
