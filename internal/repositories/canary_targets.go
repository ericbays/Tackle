package repositories

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"
)

// CanaryTarget is the DB model for a campaign_canary_targets row.
type CanaryTarget struct {
	CampaignID   string
	TargetID     string
	DesignatedAt time.Time
	DesignatedBy string
	VerifiedAt   *time.Time
}

// CanaryTargetDetail includes target details alongside canary metadata.
type CanaryTargetDetail struct {
	CanaryTarget
	Email     string
	FirstName *string
	LastName  *string
}

// CanaryTargetRepository provides database operations for campaign_canary_targets.
type CanaryTargetRepository struct {
	db *sql.DB
}

// NewCanaryTargetRepository creates a new CanaryTargetRepository.
func NewCanaryTargetRepository(db *sql.DB) *CanaryTargetRepository {
	return &CanaryTargetRepository{db: db}
}

// Designate adds canary targets to a campaign. Idempotent.
func (r *CanaryTargetRepository) Designate(ctx context.Context, campaignID string, targetIDs []string, designatedBy string) (int, error) {
	if len(targetIDs) == 0 {
		return 0, nil
	}

	values := []string{}
	args := []any{campaignID, designatedBy}
	argIdx := 3
	for _, tid := range targetIDs {
		values = append(values, fmt.Sprintf("($1, $%d, now(), $2)", argIdx))
		args = append(args, tid)
		argIdx++
	}

	q := fmt.Sprintf(`
		INSERT INTO campaign_canary_targets (campaign_id, target_id, designated_at, designated_by)
		VALUES %s
		ON CONFLICT (campaign_id, target_id) DO NOTHING`, strings.Join(values, ", "))

	res, err := r.db.ExecContext(ctx, q, args...)
	if err != nil {
		return 0, fmt.Errorf("canary_targets: designate: %w", err)
	}
	n, _ := res.RowsAffected()
	return int(n), nil
}

// Undesignate removes canary designation from targets in a campaign.
func (r *CanaryTargetRepository) Undesignate(ctx context.Context, campaignID string, targetIDs []string) (int, error) {
	if len(targetIDs) == 0 {
		return 0, nil
	}

	placeholders := []string{}
	args := []any{campaignID}
	for i, tid := range targetIDs {
		placeholders = append(placeholders, fmt.Sprintf("$%d", i+2))
		args = append(args, tid)
	}

	q := fmt.Sprintf(`
		DELETE FROM campaign_canary_targets
		WHERE campaign_id = $1 AND target_id IN (%s)`, strings.Join(placeholders, ", "))

	res, err := r.db.ExecContext(ctx, q, args...)
	if err != nil {
		return 0, fmt.Errorf("canary_targets: undesignate: %w", err)
	}
	n, _ := res.RowsAffected()
	return int(n), nil
}

// List returns all canary targets for a campaign with target details.
func (r *CanaryTargetRepository) List(ctx context.Context, campaignID string) ([]CanaryTargetDetail, error) {
	const q = `
		SELECT cct.campaign_id, cct.target_id, cct.designated_at, cct.designated_by, cct.verified_at,
		       t.email, t.first_name, t.last_name
		FROM campaign_canary_targets cct
		INNER JOIN targets t ON t.id = cct.target_id
		WHERE cct.campaign_id = $1
		ORDER BY t.email ASC`

	rows, err := r.db.QueryContext(ctx, q, campaignID)
	if err != nil {
		return nil, fmt.Errorf("canary_targets: list: %w", err)
	}
	defer rows.Close()

	var targets []CanaryTargetDetail
	for rows.Next() {
		var d CanaryTargetDetail
		if err := rows.Scan(
			&d.CampaignID, &d.TargetID, &d.DesignatedAt, &d.DesignatedBy, &d.VerifiedAt,
			&d.Email, &d.FirstName, &d.LastName,
		); err != nil {
			return nil, fmt.Errorf("canary_targets: list scan: %w", err)
		}
		targets = append(targets, d)
	}
	return targets, rows.Err()
}

// MarkVerified records a verification timestamp for a canary target.
func (r *CanaryTargetRepository) MarkVerified(ctx context.Context, campaignID, targetID string) error {
	const q = `
		UPDATE campaign_canary_targets SET verified_at = now()
		WHERE campaign_id = $1 AND target_id = $2 AND verified_at IS NULL`
	_, err := r.db.ExecContext(ctx, q, campaignID, targetID)
	if err != nil {
		return fmt.Errorf("canary_targets: verify: %w", err)
	}
	return nil
}

// IsCanary checks if a target is a canary for a specific campaign.
func (r *CanaryTargetRepository) IsCanary(ctx context.Context, campaignID, targetID string) (bool, error) {
	const q = `
		SELECT EXISTS(
			SELECT 1 FROM campaign_canary_targets
			WHERE campaign_id = $1 AND target_id = $2
		)`
	var exists bool
	err := r.db.QueryRowContext(ctx, q, campaignID, targetID).Scan(&exists)
	if err != nil {
		return false, fmt.Errorf("canary_targets: is_canary: %w", err)
	}
	return exists, nil
}
