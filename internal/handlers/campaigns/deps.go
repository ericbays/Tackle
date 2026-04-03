// Package campaigns provides HTTP handlers for campaign lifecycle management.
package campaigns

import (
	"context"
	"database/sql"

	campaignsvc "tackle/internal/services/campaign"
)

// Deps holds handler dependencies.
type Deps struct {
	DB          *sql.DB
	Svc         *campaignsvc.Service
	ApprovalSvc *campaignsvc.ApprovalService
	Builder     *campaignsvc.CampaignBuilder
}

// canAccessCampaign checks whether an operator can access a campaign.
// Admins and engineers bypass this check. Returns true if the user
// created the campaign or it was shared with them.
func (d *Deps) canAccessCampaign(ctx context.Context, role, userID, campaignID string) bool {
	if role != "operator" {
		return true // admin/engineer see everything
	}
	// Check ownership.
	var createdBy string
	err := d.DB.QueryRowContext(ctx, `SELECT created_by FROM campaigns WHERE id = $1`, campaignID).Scan(&createdBy)
	if err != nil {
		return false
	}
	if createdBy == userID {
		return true
	}
	// Check campaign_shares.
	var exists bool
	_ = d.DB.QueryRowContext(ctx,
		`SELECT EXISTS(SELECT 1 FROM campaign_shares WHERE campaign_id = $1 AND user_id = $2)`,
		campaignID, userID,
	).Scan(&exists)
	return exists
}
