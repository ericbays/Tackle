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

// FieldCategory classifies a captured form field.
type FieldCategory string

const (
	FieldCategoryIdentity  FieldCategory = "identity"
	FieldCategorySensitive FieldCategory = "sensitive"
	FieldCategoryMFA       FieldCategory = "mfa"
	FieldCategoryCustom    FieldCategory = "custom"
	FieldCategoryHidden    FieldCategory = "hidden"
)

// SessionDataType classifies a captured session artifact.
type SessionDataType string

const (
	SessionDataCookie         SessionDataType = "cookie"
	SessionDataOAuthToken     SessionDataType = "oauth_token"
	SessionDataSessionToken   SessionDataType = "session_token"
	SessionDataAuthHeader     SessionDataType = "auth_header"
	SessionDataLocalStorage   SessionDataType = "local_storage"
	SessionDataSessionStorage SessionDataType = "session_storage"
)

// PostCaptureAction defines what happens after a form submission is captured.
type PostCaptureAction string

const (
	PostCaptureRedirect          PostCaptureAction = "redirect"
	PostCaptureDisplayPage       PostCaptureAction = "display_page"
	PostCaptureRedirectWithDelay PostCaptureAction = "redirect_with_delay"
	PostCaptureReplaySubmission  PostCaptureAction = "replay_submission"
	PostCaptureNoAction          PostCaptureAction = "no_action"
)

// CaptureEvent represents a single form submission capture.
type CaptureEvent struct {
	ID                string
	CampaignID        string
	TargetID          *string
	TemplateVariantID *string
	EndpointID        *string
	EmailSendID       *string
	TrackingToken     *string
	SourceIP          *string
	UserAgent         *string
	AcceptLanguage    *string
	Referer           *string
	URLPath           *string
	HTTPMethod        string
	SubmissionSeq     int
	IsUnattributed    bool
	IsCanary          bool
	CapturedAt        time.Time
	CreatedAt         time.Time
}

// CaptureField represents an individual form field value (encrypted).
type CaptureField struct {
	ID                   string
	CaptureEventID       string
	FieldName            string
	FieldValueEncrypted  []byte
	FieldCategory        FieldCategory
	EncryptionKeyVersion int
	IV                   []byte
}

// SessionCapture represents a captured session artifact (cookie, token, etc.).
type SessionCapture struct {
	ID              string
	CaptureEventID  string
	DataType        SessionDataType
	KeyEncrypted    []byte
	ValueEncrypted  []byte
	Metadata        json.RawMessage
	CapturedAt      time.Time
	IsTimeSensitive bool
}

// FieldCategorizationRule maps field name patterns to categories.
type FieldCategorizationRule struct {
	ID            string
	LandingPageID *string
	FieldPattern  string
	Category      FieldCategory
	IsDefault     bool
	Priority      int
	CreatedAt     time.Time
}

// CaptureEventFilters controls optional filtering for list queries.
type CaptureEventFilters struct {
	CampaignID     string
	TargetID       string
	DateAfter      *time.Time
	DateBefore     *time.Time
	IsUnattributed *bool
	Page           int
	PerPage        int
}

// CaptureEventRepository provides database operations for credential capture.
type CaptureEventRepository struct {
	db *sql.DB
}

// NewCaptureEventRepository creates a new CaptureEventRepository.
func NewCaptureEventRepository(db *sql.DB) *CaptureEventRepository {
	return &CaptureEventRepository{db: db}
}

// CreateEvent inserts a new capture event and returns it.
func (r *CaptureEventRepository) CreateEvent(ctx context.Context, e CaptureEvent) (CaptureEvent, error) {
	id := uuid.New().String()
	const q = `
		INSERT INTO capture_events
			(id, campaign_id, target_id, template_variant_id, endpoint_id,
			 email_send_id, tracking_token, source_ip, user_agent,
			 accept_language, referer, url_path, http_method,
			 submission_sequence, is_unattributed, is_canary, captured_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8::INET,$9,$10,$11,$12,$13,$14,$15,$16,$17)
		RETURNING id, campaign_id, target_id, template_variant_id, endpoint_id,
		          email_send_id, tracking_token, source_ip, user_agent,
		          accept_language, referer, url_path, http_method,
		          submission_sequence, is_unattributed, is_canary, captured_at, created_at`
	return r.scanEvent(r.db.QueryRowContext(ctx, q,
		id, e.CampaignID, e.TargetID, e.TemplateVariantID, e.EndpointID,
		e.EmailSendID, e.TrackingToken, e.SourceIP, e.UserAgent,
		e.AcceptLanguage, e.Referer, e.URLPath, e.HTTPMethod,
		e.SubmissionSeq, e.IsUnattributed, e.IsCanary, e.CapturedAt,
	))
}

// GetEvent returns a capture event by ID.
func (r *CaptureEventRepository) GetEvent(ctx context.Context, id string) (CaptureEvent, error) {
	const q = `
		SELECT id, campaign_id, target_id, template_variant_id, endpoint_id,
		       email_send_id, tracking_token, source_ip, user_agent,
		       accept_language, referer, url_path, http_method,
		       submission_sequence, is_unattributed, is_canary, captured_at, created_at
		FROM capture_events WHERE id = $1`
	return r.scanEvent(r.db.QueryRowContext(ctx, q, id))
}

// ListEvents returns paginated capture events matching filters.
func (r *CaptureEventRepository) ListEvents(ctx context.Context, f CaptureEventFilters) ([]CaptureEvent, int, error) {
	var where []string
	var args []any
	n := 1

	if f.CampaignID != "" {
		where = append(where, fmt.Sprintf("campaign_id = $%d", n))
		args = append(args, f.CampaignID)
		n++
	}
	if f.TargetID != "" {
		where = append(where, fmt.Sprintf("target_id = $%d", n))
		args = append(args, f.TargetID)
		n++
	}
	if f.DateAfter != nil {
		where = append(where, fmt.Sprintf("captured_at >= $%d", n))
		args = append(args, *f.DateAfter)
		n++
	}
	if f.DateBefore != nil {
		where = append(where, fmt.Sprintf("captured_at <= $%d", n))
		args = append(args, *f.DateBefore)
		n++
	}
	if f.IsUnattributed != nil {
		where = append(where, fmt.Sprintf("is_unattributed = $%d", n))
		args = append(args, *f.IsUnattributed)
		n++
	}

	whereClause := ""
	if len(where) > 0 {
		whereClause = "WHERE " + strings.Join(where, " AND ")
	}

	// Count total.
	countQ := "SELECT COUNT(*) FROM capture_events " + whereClause
	var total int
	if err := r.db.QueryRowContext(ctx, countQ, args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count capture events: %w", err)
	}

	page := f.Page
	if page < 1 {
		page = 1
	}
	perPage := f.PerPage
	if perPage < 1 {
		perPage = 25
	}
	if perPage > 100 {
		perPage = 100
	}
	offset := (page - 1) * perPage

	q := fmt.Sprintf(`
		SELECT id, campaign_id, target_id, template_variant_id, endpoint_id,
		       email_send_id, tracking_token, source_ip, user_agent,
		       accept_language, referer, url_path, http_method,
		       submission_sequence, is_unattributed, is_canary, captured_at, created_at
		FROM capture_events %s
		ORDER BY captured_at DESC
		LIMIT $%d OFFSET $%d`, whereClause, n, n+1)
	args = append(args, perPage, offset)

	rows, err := r.db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("list capture events: %w", err)
	}
	defer rows.Close()

	var events []CaptureEvent
	for rows.Next() {
		e, err := r.scanEventRow(rows)
		if err != nil {
			return nil, 0, err
		}
		events = append(events, e)
	}
	return events, total, rows.Err()
}

// NextSubmissionSequence returns the next sequence number for a target in a campaign.
func (r *CaptureEventRepository) NextSubmissionSequence(ctx context.Context, campaignID string, targetID *string) (int, error) {
	if targetID == nil {
		return 1, nil
	}
	const q = `SELECT COALESCE(MAX(submission_sequence), 0) + 1
		FROM capture_events WHERE campaign_id = $1 AND target_id = $2`
	var seq int
	if err := r.db.QueryRowContext(ctx, q, campaignID, *targetID).Scan(&seq); err != nil {
		return 0, fmt.Errorf("next submission sequence: %w", err)
	}
	return seq, nil
}

// CountByCampaign returns the number of capture events for a campaign.
func (r *CaptureEventRepository) CountByCampaign(ctx context.Context, campaignID string) (int, error) {
	const q = `SELECT COUNT(*) FROM capture_events WHERE campaign_id = $1`
	var count int
	if err := r.db.QueryRowContext(ctx, q, campaignID).Scan(&count); err != nil {
		return 0, fmt.Errorf("count captures by campaign: %w", err)
	}
	return count, nil
}

// CountByCampaignAndTarget returns the submission count for a specific target in a campaign.
func (r *CaptureEventRepository) CountByCampaignAndTarget(ctx context.Context, campaignID, targetID string) (int, error) {
	const q = `SELECT COUNT(*) FROM capture_events WHERE campaign_id = $1 AND target_id = $2`
	var count int
	if err := r.db.QueryRowContext(ctx, q, campaignID, targetID).Scan(&count); err != nil {
		return 0, fmt.Errorf("count captures by campaign+target: %w", err)
	}
	return count, nil
}

// DeleteEvent deletes a capture event by ID. Returns false if not found.
func (r *CaptureEventRepository) DeleteEvent(ctx context.Context, id string) (bool, error) {
	const q = `DELETE FROM capture_events WHERE id = $1`
	res, err := r.db.ExecContext(ctx, q, id)
	if err != nil {
		return false, fmt.Errorf("delete capture event: %w", err)
	}
	n, _ := res.RowsAffected()
	return n > 0, nil
}

// PurgeByCampaign deletes all capture events for a campaign. Returns count deleted.
func (r *CaptureEventRepository) PurgeByCampaign(ctx context.Context, campaignID string) (int, error) {
	const q = `DELETE FROM capture_events WHERE campaign_id = $1`
	res, err := r.db.ExecContext(ctx, q, campaignID)
	if err != nil {
		return 0, fmt.Errorf("purge captures by campaign: %w", err)
	}
	n, _ := res.RowsAffected()
	return int(n), nil
}

// PurgeByDateRange deletes capture events in a date range. Returns count deleted.
func (r *CaptureEventRepository) PurgeByDateRange(ctx context.Context, after, before time.Time) (int, error) {
	const q = `DELETE FROM capture_events WHERE captured_at >= $1 AND captured_at <= $2`
	res, err := r.db.ExecContext(ctx, q, after, before)
	if err != nil {
		return 0, fmt.Errorf("purge captures by date: %w", err)
	}
	n, _ := res.RowsAffected()
	return int(n), nil
}

// AssociateTarget links an unattributed capture to a target.
func (r *CaptureEventRepository) AssociateTarget(ctx context.Context, eventID, targetID string) error {
	const q = `UPDATE capture_events SET target_id = $1, is_unattributed = false WHERE id = $2 AND is_unattributed = true`
	res, err := r.db.ExecContext(ctx, q, targetID, eventID)
	if err != nil {
		return fmt.Errorf("associate target: %w", err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return fmt.Errorf("capture event %s not found or not unattributed", eventID)
	}
	return nil
}

// CreateField inserts a capture field record.
func (r *CaptureEventRepository) CreateField(ctx context.Context, f CaptureField) (CaptureField, error) {
	id := uuid.New().String()
	const q = `
		INSERT INTO capture_fields
			(id, capture_event_id, field_name, field_value_encrypted, field_category, encryption_key_version, iv)
		VALUES ($1,$2,$3,$4,$5,$6,$7)
		RETURNING id, capture_event_id, field_name, field_value_encrypted, field_category, encryption_key_version, iv`
	row := r.db.QueryRowContext(ctx, q, id, f.CaptureEventID, f.FieldName,
		f.FieldValueEncrypted, string(f.FieldCategory), f.EncryptionKeyVersion, f.IV)
	return r.scanField(row)
}

// ListFieldsByEvent returns all capture fields for a capture event.
func (r *CaptureEventRepository) ListFieldsByEvent(ctx context.Context, eventID string) ([]CaptureField, error) {
	const q = `
		SELECT id, capture_event_id, field_name, field_value_encrypted, field_category, encryption_key_version, iv
		FROM capture_fields WHERE capture_event_id = $1
		ORDER BY field_name`
	rows, err := r.db.QueryContext(ctx, q, eventID)
	if err != nil {
		return nil, fmt.Errorf("list capture fields: %w", err)
	}
	defer rows.Close()

	var fields []CaptureField
	for rows.Next() {
		var f CaptureField
		if err := rows.Scan(&f.ID, &f.CaptureEventID, &f.FieldName,
			&f.FieldValueEncrypted, &f.FieldCategory, &f.EncryptionKeyVersion, &f.IV); err != nil {
			return nil, fmt.Errorf("scan capture field: %w", err)
		}
		fields = append(fields, f)
	}
	return fields, rows.Err()
}

// CreateSessionCapture inserts a session capture record.
func (r *CaptureEventRepository) CreateSessionCapture(ctx context.Context, sc SessionCapture) (SessionCapture, error) {
	id := uuid.New().String()
	const q = `
		INSERT INTO session_captures
			(id, capture_event_id, data_type, key_encrypted, value_encrypted, metadata, captured_at, is_time_sensitive)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8)
		RETURNING id, capture_event_id, data_type, key_encrypted, value_encrypted, metadata, captured_at, is_time_sensitive`
	row := r.db.QueryRowContext(ctx, q, id, sc.CaptureEventID, string(sc.DataType),
		sc.KeyEncrypted, sc.ValueEncrypted, sc.Metadata, sc.CapturedAt, sc.IsTimeSensitive)
	return r.scanSession(row)
}

// ListSessionCapturesByEvent returns all session captures for a capture event.
func (r *CaptureEventRepository) ListSessionCapturesByEvent(ctx context.Context, eventID string) ([]SessionCapture, error) {
	const q = `
		SELECT id, capture_event_id, data_type, key_encrypted, value_encrypted, metadata, captured_at, is_time_sensitive
		FROM session_captures WHERE capture_event_id = $1
		ORDER BY captured_at`
	rows, err := r.db.QueryContext(ctx, q, eventID)
	if err != nil {
		return nil, fmt.Errorf("list session captures: %w", err)
	}
	defer rows.Close()

	var caps []SessionCapture
	for rows.Next() {
		sc, err := r.scanSessionRow(rows)
		if err != nil {
			return nil, err
		}
		caps = append(caps, sc)
	}
	return caps, rows.Err()
}

// ListCategorizationRules returns field categorization rules, merging global defaults with landing-page-specific overrides.
func (r *CaptureEventRepository) ListCategorizationRules(ctx context.Context, landingPageID *string) ([]FieldCategorizationRule, error) {
	const q = `
		SELECT id, landing_page_id, field_pattern, category, is_default, priority, created_at
		FROM field_categorization_rules
		WHERE landing_page_id IS NULL OR landing_page_id = $1
		ORDER BY priority DESC, is_default ASC`
	rows, err := r.db.QueryContext(ctx, q, landingPageID)
	if err != nil {
		return nil, fmt.Errorf("list categorization rules: %w", err)
	}
	defer rows.Close()

	var rules []FieldCategorizationRule
	for rows.Next() {
		var rule FieldCategorizationRule
		if err := rows.Scan(&rule.ID, &rule.LandingPageID, &rule.FieldPattern,
			&rule.Category, &rule.IsDefault, &rule.Priority, &rule.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan categorization rule: %w", err)
		}
		rules = append(rules, rule)
	}
	return rules, rows.Err()
}

// UpsertCategorizationRule creates or updates a field categorization rule.
func (r *CaptureEventRepository) UpsertCategorizationRule(ctx context.Context, rule FieldCategorizationRule) (FieldCategorizationRule, error) {
	if rule.ID == "" {
		rule.ID = uuid.New().String()
	}
	const q = `
		INSERT INTO field_categorization_rules
			(id, landing_page_id, field_pattern, category, is_default, priority)
		VALUES ($1,$2,$3,$4,$5,$6)
		ON CONFLICT (id) DO UPDATE SET
			field_pattern = EXCLUDED.field_pattern,
			category = EXCLUDED.category,
			priority = EXCLUDED.priority
		RETURNING id, landing_page_id, field_pattern, category, is_default, priority, created_at`
	var out FieldCategorizationRule
	err := r.db.QueryRowContext(ctx, q, rule.ID, rule.LandingPageID, rule.FieldPattern,
		string(rule.Category), rule.IsDefault, rule.Priority).Scan(
		&out.ID, &out.LandingPageID, &out.FieldPattern,
		&out.Category, &out.IsDefault, &out.Priority, &out.CreatedAt)
	if err != nil {
		return FieldCategorizationRule{}, fmt.Errorf("upsert categorization rule: %w", err)
	}
	return out, nil
}

// DeleteCategorizationRule deletes a non-default categorization rule.
func (r *CaptureEventRepository) DeleteCategorizationRule(ctx context.Context, id string) error {
	const q = `DELETE FROM field_categorization_rules WHERE id = $1 AND is_default = false`
	res, err := r.db.ExecContext(ctx, q, id)
	if err != nil {
		return fmt.Errorf("delete categorization rule: %w", err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return fmt.Errorf("categorization rule %s not found or is a default rule", id)
	}
	return nil
}

// GetCaptureMetrics computes aggregate metrics for a campaign.
func (r *CaptureEventRepository) GetCaptureMetrics(ctx context.Context, campaignID string) (CaptureMetrics, error) {
	var m CaptureMetrics
	m.CampaignID = campaignID

	// Total capture count.
	err := r.db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM capture_events WHERE campaign_id = $1`, campaignID).Scan(&m.TotalCaptures)
	if err != nil {
		return m, fmt.Errorf("metrics total captures: %w", err)
	}

	// Unique targets who submitted.
	err = r.db.QueryRowContext(ctx,
		`SELECT COUNT(DISTINCT target_id) FROM capture_events WHERE campaign_id = $1 AND target_id IS NOT NULL`, campaignID).Scan(&m.UniqueTargets)
	if err != nil {
		return m, fmt.Errorf("metrics unique targets: %w", err)
	}

	// Repeat submissions (targets with > 1 submission).
	err = r.db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM (
			SELECT target_id FROM capture_events
			WHERE campaign_id = $1 AND target_id IS NOT NULL
			GROUP BY target_id HAVING COUNT(*) > 1
		) sub`, campaignID).Scan(&m.RepeatSubmitters)
	if err != nil {
		return m, fmt.Errorf("metrics repeat submitters: %w", err)
	}

	// Unattributed count.
	err = r.db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM capture_events WHERE campaign_id = $1 AND is_unattributed = true`, campaignID).Scan(&m.UnattributedCount)
	if err != nil {
		return m, fmt.Errorf("metrics unattributed: %w", err)
	}

	return m, nil
}

// GetVariantMetrics returns capture counts per template variant for a campaign.
func (r *CaptureEventRepository) GetVariantMetrics(ctx context.Context, campaignID string) ([]VariantMetric, error) {
	const q = `
		SELECT COALESCE(template_variant_id::TEXT, 'unknown'), COUNT(*), COUNT(DISTINCT target_id)
		FROM capture_events WHERE campaign_id = $1
		GROUP BY template_variant_id`
	rows, err := r.db.QueryContext(ctx, q, campaignID)
	if err != nil {
		return nil, fmt.Errorf("variant metrics: %w", err)
	}
	defer rows.Close()

	var metrics []VariantMetric
	for rows.Next() {
		var vm VariantMetric
		if err := rows.Scan(&vm.VariantID, &vm.TotalCaptures, &vm.UniqueTargets); err != nil {
			return nil, fmt.Errorf("scan variant metric: %w", err)
		}
		metrics = append(metrics, vm)
	}
	return metrics, rows.Err()
}

// GetCaptureTimeline returns time-bucketed capture counts for a campaign.
func (r *CaptureEventRepository) GetCaptureTimeline(ctx context.Context, campaignID string, bucketMinutes int) ([]TimelineBucket, error) {
	if bucketMinutes < 1 {
		bucketMinutes = 60
	}
	q := fmt.Sprintf(`
		SELECT date_trunc('hour', captured_at) +
		       (EXTRACT(minute FROM captured_at)::INT / %d * %d) * INTERVAL '1 minute' AS bucket,
		       COUNT(*)
		FROM capture_events WHERE campaign_id = $1
		GROUP BY bucket ORDER BY bucket`, bucketMinutes, bucketMinutes)
	rows, err := r.db.QueryContext(ctx, q, campaignID)
	if err != nil {
		return nil, fmt.Errorf("capture timeline: %w", err)
	}
	defer rows.Close()

	var buckets []TimelineBucket
	for rows.Next() {
		var b TimelineBucket
		if err := rows.Scan(&b.Timestamp, &b.Count); err != nil {
			return nil, fmt.Errorf("scan timeline bucket: %w", err)
		}
		buckets = append(buckets, b)
	}
	return buckets, rows.Err()
}

// GetFieldCompletionRates returns field-level completion statistics for a campaign.
func (r *CaptureEventRepository) GetFieldCompletionRates(ctx context.Context, campaignID string) ([]FieldCompletionRate, error) {
	const q = `
		SELECT cf.field_name, COUNT(*) AS filled_count,
		       (SELECT COUNT(*) FROM capture_events WHERE campaign_id = $1) AS total_events
		FROM capture_fields cf
		JOIN capture_events ce ON ce.id = cf.capture_event_id
		WHERE ce.campaign_id = $1
		GROUP BY cf.field_name
		ORDER BY filled_count DESC`
	rows, err := r.db.QueryContext(ctx, q, campaignID)
	if err != nil {
		return nil, fmt.Errorf("field completion rates: %w", err)
	}
	defer rows.Close()

	var rates []FieldCompletionRate
	for rows.Next() {
		var fcr FieldCompletionRate
		if err := rows.Scan(&fcr.FieldName, &fcr.FilledCount, &fcr.TotalEvents); err != nil {
			return nil, fmt.Errorf("scan field completion: %w", err)
		}
		rates = append(rates, fcr)
	}
	return rates, rows.Err()
}

// CaptureMetrics holds aggregate capture metrics for a campaign.
type CaptureMetrics struct {
	CampaignID        string `json:"campaign_id"`
	TotalCaptures     int    `json:"total_captures"`
	UniqueTargets     int    `json:"unique_targets"`
	RepeatSubmitters  int    `json:"repeat_submitters"`
	UnattributedCount int    `json:"unattributed_count"`
}

// VariantMetric holds per-variant capture metrics.
type VariantMetric struct {
	VariantID     string `json:"variant_id"`
	TotalCaptures int    `json:"total_captures"`
	UniqueTargets int    `json:"unique_targets"`
}

// TimelineBucket holds a time-bucketed capture count.
type TimelineBucket struct {
	Timestamp time.Time `json:"timestamp"`
	Count     int       `json:"count"`
}

// FieldCompletionRate holds completion stats for a field.
type FieldCompletionRate struct {
	FieldName   string `json:"field_name"`
	FilledCount int    `json:"filled_count"`
	TotalEvents int    `json:"total_events"`
}

// scanEvent scans a single capture event from a QueryRow result.
func (r *CaptureEventRepository) scanEvent(row *sql.Row) (CaptureEvent, error) {
	var e CaptureEvent
	err := row.Scan(
		&e.ID, &e.CampaignID, &e.TargetID, &e.TemplateVariantID, &e.EndpointID,
		&e.EmailSendID, &e.TrackingToken, &e.SourceIP, &e.UserAgent,
		&e.AcceptLanguage, &e.Referer, &e.URLPath, &e.HTTPMethod,
		&e.SubmissionSeq, &e.IsUnattributed, &e.IsCanary, &e.CapturedAt, &e.CreatedAt,
	)
	if err != nil {
		return CaptureEvent{}, fmt.Errorf("scan capture event: %w", err)
	}
	return e, nil
}

// scanEventRow scans a single capture event from a Rows result.
func (r *CaptureEventRepository) scanEventRow(rows *sql.Rows) (CaptureEvent, error) {
	var e CaptureEvent
	err := rows.Scan(
		&e.ID, &e.CampaignID, &e.TargetID, &e.TemplateVariantID, &e.EndpointID,
		&e.EmailSendID, &e.TrackingToken, &e.SourceIP, &e.UserAgent,
		&e.AcceptLanguage, &e.Referer, &e.URLPath, &e.HTTPMethod,
		&e.SubmissionSeq, &e.IsUnattributed, &e.IsCanary, &e.CapturedAt, &e.CreatedAt,
	)
	if err != nil {
		return CaptureEvent{}, fmt.Errorf("scan capture event row: %w", err)
	}
	return e, nil
}

// scanField scans a capture field from a QueryRow result.
func (r *CaptureEventRepository) scanField(row *sql.Row) (CaptureField, error) {
	var f CaptureField
	err := row.Scan(&f.ID, &f.CaptureEventID, &f.FieldName,
		&f.FieldValueEncrypted, &f.FieldCategory, &f.EncryptionKeyVersion, &f.IV)
	if err != nil {
		return CaptureField{}, fmt.Errorf("scan capture field: %w", err)
	}
	return f, nil
}

// scanSession scans a session capture from a QueryRow result.
func (r *CaptureEventRepository) scanSession(row *sql.Row) (SessionCapture, error) {
	var sc SessionCapture
	err := row.Scan(&sc.ID, &sc.CaptureEventID, &sc.DataType,
		&sc.KeyEncrypted, &sc.ValueEncrypted, &sc.Metadata, &sc.CapturedAt, &sc.IsTimeSensitive)
	if err != nil {
		return SessionCapture{}, fmt.Errorf("scan session capture: %w", err)
	}
	return sc, nil
}

// scanSessionRow scans a session capture from a Rows result.
func (r *CaptureEventRepository) scanSessionRow(rows *sql.Rows) (SessionCapture, error) {
	var sc SessionCapture
	err := rows.Scan(&sc.ID, &sc.CaptureEventID, &sc.DataType,
		&sc.KeyEncrypted, &sc.ValueEncrypted, &sc.Metadata, &sc.CapturedAt, &sc.IsTimeSensitive)
	if err != nil {
		return SessionCapture{}, fmt.Errorf("scan session capture row: %w", err)
	}
	return sc, nil
}
