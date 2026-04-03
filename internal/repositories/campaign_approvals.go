package repositories

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
)

// CampaignApproval is the DB model for campaign_approvals.
type CampaignApproval struct {
	ID                     string
	CampaignID             string
	SubmissionID           string
	ActorID                string
	Action                 string // submitted, approved, rejected, unlock, blocklist_override_approved, blocklist_override_rejected
	Comments               string
	BlockListAcknowledged  bool
	BlockListJustification *string
	ConfigSnapshotJSON     map[string]any
	CreatedAt              time.Time
}

// CampaignApprovalRequirement is the DB model for campaign_approval_requirements.
type CampaignApprovalRequirement struct {
	CampaignID            string
	SubmissionID          string
	RequiredApproverCount int
	RequiresAdminApproval bool
	CurrentApprovalCount  int
	CreatedAt             time.Time
}

// CampaignApprovalRepository handles approval-related database operations.
type CampaignApprovalRepository struct {
	db *sql.DB
}

// NewCampaignApprovalRepository creates a new CampaignApprovalRepository.
func NewCampaignApprovalRepository(db *sql.DB) *CampaignApprovalRepository {
	return &CampaignApprovalRepository{db: db}
}

// CreateApproval inserts an approval record.
func (r *CampaignApprovalRepository) CreateApproval(ctx context.Context, a CampaignApproval) (CampaignApproval, error) {
	a.ID = uuid.New().String()
	var cfgJSON []byte
	if a.ConfigSnapshotJSON != nil {
		var err error
		cfgJSON, err = json.Marshal(a.ConfigSnapshotJSON)
		if err != nil {
			return CampaignApproval{}, fmt.Errorf("campaign_approvals: marshal config: %w", err)
		}
	}

	err := r.db.QueryRowContext(ctx,
		`INSERT INTO campaign_approvals (id, campaign_id, submission_id, actor_id, action, comments,
		 block_list_acknowledged, block_list_justification, config_snapshot_json)
		 VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9)
		 RETURNING id, campaign_id, submission_id, actor_id, action, comments,
		           block_list_acknowledged, block_list_justification, config_snapshot_json, created_at`,
		a.ID, a.CampaignID, a.SubmissionID, a.ActorID, a.Action, a.Comments,
		a.BlockListAcknowledged, a.BlockListJustification, cfgJSON,
	).Scan(&a.ID, &a.CampaignID, &a.SubmissionID, &a.ActorID, &a.Action, &a.Comments,
		&a.BlockListAcknowledged, &a.BlockListJustification, &cfgJSON, &a.CreatedAt)
	if err != nil {
		return CampaignApproval{}, fmt.Errorf("campaign_approvals: create: %w", err)
	}
	if cfgJSON != nil {
		_ = json.Unmarshal(cfgJSON, &a.ConfigSnapshotJSON)
	}
	return a, nil
}

// CreateApprovalInTx inserts an approval record within a transaction.
func (r *CampaignApprovalRepository) CreateApprovalInTx(ctx context.Context, tx *sql.Tx, a CampaignApproval) (CampaignApproval, error) {
	a.ID = uuid.New().String()
	var cfgJSON []byte
	if a.ConfigSnapshotJSON != nil {
		var err error
		cfgJSON, err = json.Marshal(a.ConfigSnapshotJSON)
		if err != nil {
			return CampaignApproval{}, fmt.Errorf("campaign_approvals: marshal config: %w", err)
		}
	}

	err := tx.QueryRowContext(ctx,
		`INSERT INTO campaign_approvals (id, campaign_id, submission_id, actor_id, action, comments,
		 block_list_acknowledged, block_list_justification, config_snapshot_json)
		 VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9)
		 RETURNING id, campaign_id, submission_id, actor_id, action, comments,
		           block_list_acknowledged, block_list_justification, config_snapshot_json, created_at`,
		a.ID, a.CampaignID, a.SubmissionID, a.ActorID, a.Action, a.Comments,
		a.BlockListAcknowledged, a.BlockListJustification, cfgJSON,
	).Scan(&a.ID, &a.CampaignID, &a.SubmissionID, &a.ActorID, &a.Action, &a.Comments,
		&a.BlockListAcknowledged, &a.BlockListJustification, &cfgJSON, &a.CreatedAt)
	if err != nil {
		return CampaignApproval{}, fmt.Errorf("campaign_approvals: create in tx: %w", err)
	}
	if cfgJSON != nil {
		_ = json.Unmarshal(cfgJSON, &a.ConfigSnapshotJSON)
	}
	return a, nil
}

// ListByCampaign returns all approval records for a campaign, ordered by time.
func (r *CampaignApprovalRepository) ListByCampaign(ctx context.Context, campaignID string) ([]CampaignApproval, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT id, campaign_id, submission_id, actor_id, action, comments,
		        block_list_acknowledged, block_list_justification, config_snapshot_json, created_at
		 FROM campaign_approvals WHERE campaign_id = $1 ORDER BY created_at ASC`, campaignID)
	if err != nil {
		return nil, fmt.Errorf("campaign_approvals: list by campaign: %w", err)
	}
	defer rows.Close()

	var result []CampaignApproval
	for rows.Next() {
		a, err := scanApproval(rows)
		if err != nil {
			return nil, err
		}
		result = append(result, a)
	}
	if result == nil {
		result = []CampaignApproval{}
	}
	return result, nil
}

// ListBySubmission returns approval records for a specific submission cycle.
func (r *CampaignApprovalRepository) ListBySubmission(ctx context.Context, campaignID, submissionID string) ([]CampaignApproval, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT id, campaign_id, submission_id, actor_id, action, comments,
		        block_list_acknowledged, block_list_justification, config_snapshot_json, created_at
		 FROM campaign_approvals WHERE campaign_id = $1 AND submission_id = $2 ORDER BY created_at ASC`,
		campaignID, submissionID)
	if err != nil {
		return nil, fmt.Errorf("campaign_approvals: list by submission: %w", err)
	}
	defer rows.Close()

	var result []CampaignApproval
	for rows.Next() {
		a, err := scanApproval(rows)
		if err != nil {
			return nil, err
		}
		result = append(result, a)
	}
	if result == nil {
		result = []CampaignApproval{}
	}
	return result, nil
}

// HasApprovedInSubmission checks if an actor has already approved in a given submission cycle.
func (r *CampaignApprovalRepository) HasApprovedInSubmission(ctx context.Context, campaignID, submissionID, actorID string) (bool, error) {
	var count int
	err := r.db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM campaign_approvals
		 WHERE campaign_id = $1 AND submission_id = $2 AND actor_id = $3 AND action = 'approved'`,
		campaignID, submissionID, actorID).Scan(&count)
	if err != nil {
		return false, fmt.Errorf("campaign_approvals: has approved: %w", err)
	}
	return count > 0, nil
}

// GetLatestSubmissionID returns the most recent submission_id for a campaign.
func (r *CampaignApprovalRepository) GetLatestSubmissionID(ctx context.Context, campaignID string) (string, error) {
	var subID string
	err := r.db.QueryRowContext(ctx,
		`SELECT submission_id FROM campaign_approvals
		 WHERE campaign_id = $1 AND action = 'submitted'
		 ORDER BY created_at DESC LIMIT 1`, campaignID).Scan(&subID)
	if err == sql.ErrNoRows {
		return "", fmt.Errorf("campaign_approvals: no submission found")
	}
	if err != nil {
		return "", fmt.Errorf("campaign_approvals: get latest submission: %w", err)
	}
	return subID, nil
}

// GetRejectionHistory returns all rejection records for a campaign, newest first.
func (r *CampaignApprovalRepository) GetRejectionHistory(ctx context.Context, campaignID string) ([]CampaignApproval, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT id, campaign_id, submission_id, actor_id, action, comments,
		        block_list_acknowledged, block_list_justification, config_snapshot_json, created_at
		 FROM campaign_approvals WHERE campaign_id = $1 AND action = 'rejected'
		 ORDER BY created_at DESC`, campaignID)
	if err != nil {
		return nil, fmt.Errorf("campaign_approvals: rejection history: %w", err)
	}
	defer rows.Close()

	var result []CampaignApproval
	for rows.Next() {
		a, err := scanApproval(rows)
		if err != nil {
			return nil, err
		}
		result = append(result, a)
	}
	if result == nil {
		result = []CampaignApproval{}
	}
	return result, nil
}

// ---------- Approval Requirements ----------

// CreateRequirement inserts an approval requirement record.
func (r *CampaignApprovalRepository) CreateRequirement(ctx context.Context, req CampaignApprovalRequirement) (CampaignApprovalRequirement, error) {
	err := r.db.QueryRowContext(ctx,
		`INSERT INTO campaign_approval_requirements (campaign_id, submission_id, required_approver_count, requires_admin_approval, current_approval_count)
		 VALUES ($1,$2,$3,$4,$5)
		 RETURNING campaign_id, submission_id, required_approver_count, requires_admin_approval, current_approval_count, created_at`,
		req.CampaignID, req.SubmissionID, req.RequiredApproverCount, req.RequiresAdminApproval, req.CurrentApprovalCount,
	).Scan(&req.CampaignID, &req.SubmissionID, &req.RequiredApproverCount, &req.RequiresAdminApproval, &req.CurrentApprovalCount, &req.CreatedAt)
	if err != nil {
		return CampaignApprovalRequirement{}, fmt.Errorf("campaign_approval_requirements: create: %w", err)
	}
	return req, nil
}

// GetRequirement returns the approval requirement for a submission cycle.
func (r *CampaignApprovalRepository) GetRequirement(ctx context.Context, campaignID, submissionID string) (CampaignApprovalRequirement, error) {
	var req CampaignApprovalRequirement
	err := r.db.QueryRowContext(ctx,
		`SELECT campaign_id, submission_id, required_approver_count, requires_admin_approval, current_approval_count, created_at
		 FROM campaign_approval_requirements WHERE campaign_id = $1 AND submission_id = $2`,
		campaignID, submissionID,
	).Scan(&req.CampaignID, &req.SubmissionID, &req.RequiredApproverCount, &req.RequiresAdminApproval, &req.CurrentApprovalCount, &req.CreatedAt)
	if err == sql.ErrNoRows {
		return CampaignApprovalRequirement{}, fmt.Errorf("campaign_approval_requirements: not found")
	}
	if err != nil {
		return CampaignApprovalRequirement{}, fmt.Errorf("campaign_approval_requirements: get: %w", err)
	}
	return req, nil
}

// IncrementApprovalCount atomically increments the approval count and returns updated requirement.
func (r *CampaignApprovalRepository) IncrementApprovalCount(ctx context.Context, tx *sql.Tx, campaignID, submissionID string) (CampaignApprovalRequirement, error) {
	var req CampaignApprovalRequirement
	err := tx.QueryRowContext(ctx,
		`UPDATE campaign_approval_requirements
		 SET current_approval_count = current_approval_count + 1
		 WHERE campaign_id = $1 AND submission_id = $2
		 RETURNING campaign_id, submission_id, required_approver_count, requires_admin_approval, current_approval_count, created_at`,
		campaignID, submissionID,
	).Scan(&req.CampaignID, &req.SubmissionID, &req.RequiredApproverCount, &req.RequiresAdminApproval, &req.CurrentApprovalCount, &req.CreatedAt)
	if err == sql.ErrNoRows {
		return CampaignApprovalRequirement{}, fmt.Errorf("campaign_approval_requirements: not found")
	}
	if err != nil {
		return CampaignApprovalRequirement{}, fmt.Errorf("campaign_approval_requirements: increment: %w", err)
	}
	return req, nil
}

// GetRequirementForUpdate returns the requirement with a row lock in a transaction.
func (r *CampaignApprovalRepository) GetRequirementForUpdate(ctx context.Context, tx *sql.Tx, campaignID, submissionID string) (CampaignApprovalRequirement, error) {
	var req CampaignApprovalRequirement
	err := tx.QueryRowContext(ctx,
		`SELECT campaign_id, submission_id, required_approver_count, requires_admin_approval, current_approval_count, created_at
		 FROM campaign_approval_requirements WHERE campaign_id = $1 AND submission_id = $2 FOR UPDATE`,
		campaignID, submissionID,
	).Scan(&req.CampaignID, &req.SubmissionID, &req.RequiredApproverCount, &req.RequiresAdminApproval, &req.CurrentApprovalCount, &req.CreatedAt)
	if err == sql.ErrNoRows {
		return CampaignApprovalRequirement{}, fmt.Errorf("campaign_approval_requirements: not found")
	}
	if err != nil {
		return CampaignApprovalRequirement{}, fmt.Errorf("campaign_approval_requirements: get for update: %w", err)
	}
	return req, nil
}

// ---------- Helpers ----------

func scanApproval(rows *sql.Rows) (CampaignApproval, error) {
	var a CampaignApproval
	var cfgJSON []byte
	if err := rows.Scan(&a.ID, &a.CampaignID, &a.SubmissionID, &a.ActorID, &a.Action, &a.Comments,
		&a.BlockListAcknowledged, &a.BlockListJustification, &cfgJSON, &a.CreatedAt); err != nil {
		return CampaignApproval{}, fmt.Errorf("campaign_approvals: scan: %w", err)
	}
	if cfgJSON != nil {
		_ = json.Unmarshal(cfgJSON, &a.ConfigSnapshotJSON)
	}
	return a, nil
}
