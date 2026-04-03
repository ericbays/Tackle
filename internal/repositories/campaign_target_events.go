package repositories

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"

	"github.com/google/uuid"
)

// CampaignTargetEventRepository provides database operations for campaign_target_events.
type CampaignTargetEventRepository struct {
	db *sql.DB
}

// NewCampaignTargetEventRepository creates a new CampaignTargetEventRepository.
func NewCampaignTargetEventRepository(db *sql.DB) *CampaignTargetEventRepository {
	return &CampaignTargetEventRepository{db: db}
}

// RecordEvent inserts a new event and updates the campaign_target status if applicable.
func (r *CampaignTargetEventRepository) RecordEvent(ctx context.Context, evt CampaignTargetEvent) (CampaignTargetEvent, error) {
	id := uuid.New().String()

	edJSON, err := json.Marshal(evt.EventData)
	if err != nil {
		return CampaignTargetEvent{}, fmt.Errorf("campaign target events: record: marshal event_data: %w", err)
	}

	var created CampaignTargetEvent
	var eventDataJSON []byte
	err = r.db.QueryRowContext(ctx, `
		INSERT INTO campaign_target_events (id, campaign_id, target_id, event_type, event_data, ip_address, user_agent)
		VALUES ($1,$2,$3,$4,$5,$6,$7)
		RETURNING id, campaign_id, target_id, event_type, event_data, ip_address, user_agent, created_at`,
		id, evt.CampaignID, evt.TargetID, evt.EventType, edJSON, evt.IPAddress, evt.UserAgent,
	).Scan(
		&created.ID, &created.CampaignID, &created.TargetID, &created.EventType,
		&eventDataJSON, &created.IPAddress, &created.UserAgent, &created.CreatedAt,
	)
	if err != nil {
		return CampaignTargetEvent{}, fmt.Errorf("campaign target events: record: %w", err)
	}
	created.EventData = map[string]any{}
	if len(eventDataJSON) > 0 {
		_ = json.Unmarshal(eventDataJSON, &created.EventData)
	}

	// Update campaign_target status and corresponding timestamp if the event type maps to a higher status.
	statusMap := map[string]int{
		"pending":              0,
		"email_sent":           1,
		"email_opened":         2,
		"link_clicked":         3,
		"credential_submitted": 4,
	}
	timestampCol := map[string]string{
		"email_sent":           "sent_at",
		"email_opened":         "opened_at",
		"link_clicked":         "clicked_at",
		"credential_submitted": "submitted_at",
	}
	if newOrder, ok := statusMap[evt.EventType]; ok {
		tsCol := timestampCol[evt.EventType]
		tsSet := ""
		if tsCol != "" {
			tsSet = fmt.Sprintf(", %s = COALESCE(%s, now())", tsCol, tsCol)
		}
		_, _ = r.db.ExecContext(ctx, fmt.Sprintf(`
			UPDATE campaign_targets
			SET status = $1%s
			WHERE campaign_id = $2 AND target_id = $3
			  AND CASE status
			        WHEN 'pending' THEN 0
			        WHEN 'email_sent' THEN 1
			        WHEN 'email_opened' THEN 2
			        WHEN 'link_clicked' THEN 3
			        WHEN 'credential_submitted' THEN 4
			      END < $4`, tsSet),
			evt.EventType, evt.CampaignID, evt.TargetID, newOrder,
		)
	}

	// Handle reported as a boolean flag + timestamp on campaign_targets.
	if evt.EventType == "reported" {
		_, _ = r.db.ExecContext(ctx,
			"UPDATE campaign_targets SET reported = true, reported_at = COALESCE(reported_at, now()) WHERE campaign_id = $1 AND target_id = $2",
			evt.CampaignID, evt.TargetID,
		)
	}

	return created, nil
}

// GetTimeline retrieves the event timeline for a target in a campaign.
func (r *CampaignTargetEventRepository) GetTimeline(ctx context.Context, campaignID, targetID string, eventType string) ([]CampaignTargetEvent, error) {
	args := []any{campaignID, targetID}
	where := "WHERE campaign_id = $1 AND target_id = $2"
	argIdx := 3

	if eventType != "" {
		where += fmt.Sprintf(" AND event_type = $%d", argIdx)
		args = append(args, eventType)
	}

	q := fmt.Sprintf(`
		SELECT id, campaign_id, target_id, event_type, event_data, ip_address, user_agent, created_at
		FROM campaign_target_events %s ORDER BY created_at DESC`, where)

	rows, err := r.db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, fmt.Errorf("campaign target events: timeline: %w", err)
	}
	defer rows.Close()

	var events []CampaignTargetEvent
	for rows.Next() {
		var evt CampaignTargetEvent
		var edJSON []byte
		if err := rows.Scan(
			&evt.ID, &evt.CampaignID, &evt.TargetID, &evt.EventType,
			&edJSON, &evt.IPAddress, &evt.UserAgent, &evt.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("campaign target events: timeline scan: %w", err)
		}
		evt.EventData = map[string]any{}
		if len(edJSON) > 0 {
			_ = json.Unmarshal(edJSON, &evt.EventData)
		}
		events = append(events, evt)
	}
	return events, rows.Err()
}

// GetCrossTargetEvents retrieves all events for a target across all campaigns.
func (r *CampaignTargetEventRepository) GetCrossTargetEvents(ctx context.Context, targetID string, limit, offset int) ([]CampaignTargetEvent, int, error) {
	if limit <= 0 || limit > 100 {
		limit = 50
	}

	var total int
	if err := r.db.QueryRowContext(ctx,
		"SELECT COUNT(*) FROM campaign_target_events WHERE target_id = $1", targetID,
	).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("campaign target events: cross events count: %w", err)
	}

	rows, err := r.db.QueryContext(ctx, `
		SELECT id, campaign_id, target_id, event_type, event_data, ip_address, user_agent, created_at
		FROM campaign_target_events WHERE target_id = $1
		ORDER BY created_at DESC LIMIT $2 OFFSET $3`, targetID, limit, offset)
	if err != nil {
		return nil, 0, fmt.Errorf("campaign target events: cross events: %w", err)
	}
	defer rows.Close()

	var events []CampaignTargetEvent
	for rows.Next() {
		var evt CampaignTargetEvent
		var edJSON []byte
		if err := rows.Scan(
			&evt.ID, &evt.CampaignID, &evt.TargetID, &evt.EventType,
			&edJSON, &evt.IPAddress, &evt.UserAgent, &evt.CreatedAt,
		); err != nil {
			return nil, 0, fmt.Errorf("campaign target events: cross events scan: %w", err)
		}
		evt.EventData = map[string]any{}
		if len(edJSON) > 0 {
			_ = json.Unmarshal(edJSON, &evt.EventData)
		}
		events = append(events, evt)
	}
	return events, total, rows.Err()
}

// GetCrossTargetHistory retrieves all campaigns a target has participated in.
func (r *CampaignTargetEventRepository) GetCrossTargetHistory(ctx context.Context, targetID string) ([]CampaignTarget, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT id, campaign_id, target_id, status, reported, assigned_at, assigned_by,
		       removed_at, sent_at, opened_at, clicked_at, submitted_at, reported_at,
		       created_at, updated_at
		FROM campaign_targets WHERE target_id = $1 ORDER BY assigned_at DESC`, targetID)
	if err != nil {
		return nil, fmt.Errorf("campaign target events: cross history: %w", err)
	}
	defer rows.Close()

	var results []CampaignTarget
	for rows.Next() {
		var ct CampaignTarget
		if err := rows.Scan(
			&ct.ID, &ct.CampaignID, &ct.TargetID, &ct.Status, &ct.Reported,
			&ct.AssignedAt, &ct.AssignedBy, &ct.RemovedAt,
			&ct.SentAt, &ct.OpenedAt, &ct.ClickedAt, &ct.SubmittedAt, &ct.ReportedAt,
			&ct.CreatedAt, &ct.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("campaign target events: cross history scan: %w", err)
		}
		results = append(results, ct)
	}
	return results, rows.Err()
}
