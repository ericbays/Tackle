package campaign

import (
	"context"
	"fmt"
	"log/slog"

	"tackle/internal/compiler/hosting"
	"tackle/internal/repositories"
	auditsvc "tackle/internal/services/audit"
)

// TeardownDeps holds all dependencies needed for infrastructure teardown.
type TeardownDeps struct {
	CampaignRepo    *repositories.CampaignRepository
	EndpointRepo    *repositories.PhishingEndpointRepository
	LandingPageRepo *repositories.LandingPageRepository
	AppMgr          *hosting.AppManager
	AuditSvc        *auditsvc.AuditService
}

// TeardownService handles infrastructure cleanup when campaigns complete, are archived, or are unlocked.
type TeardownService struct {
	deps TeardownDeps
}

// NewTeardownService creates a new TeardownService.
func NewTeardownService(deps TeardownDeps) *TeardownService {
	return &TeardownService{deps: deps}
}

// TeardownInfrastructure performs full infrastructure cleanup for a campaign.
// This includes: stop landing page apps, clean up builds, destroy SSH keys,
// and release domain association. Endpoint VM termination is handled separately
// by endpointmgmt.HandleCampaignCompletion.
func (t *TeardownService) TeardownInfrastructure(ctx context.Context, campaignID string) error {
	slog.Info("teardown: starting infrastructure teardown", "campaign_id", campaignID)

	// 1. Stop any running landing page apps for this campaign.
	if err := t.stopLandingPageApps(ctx, campaignID); err != nil {
		slog.Error("teardown: stop landing page apps failed", "campaign_id", campaignID, "error", err)
		// Continue teardown despite errors.
	}

	// 2. Destroy SSH keys associated with the campaign.
	if err := t.destroySSHKeys(ctx, campaignID); err != nil {
		slog.Error("teardown: destroy ssh keys failed", "campaign_id", campaignID, "error", err)
	}

	// 3. Release domain association by clearing endpoint_domain_id.
	if err := t.releaseDomainAssociation(ctx, campaignID); err != nil {
		slog.Error("teardown: release domain failed", "campaign_id", campaignID, "error", err)
	}

	// Audit log all teardown actions.
	resType := "campaign"
	_ = t.deps.AuditSvc.Log(ctx, auditsvc.LogEntry{
		Category:     auditsvc.CategoryInfrastructure,
		Severity:     auditsvc.SeverityInfo,
		ActorType:    auditsvc.ActorTypeSystem,
		ActorLabel:   "system",
		Action:       "campaign.infrastructure_teardown",
		ResourceType: &resType,
		ResourceID:   &campaignID,
		Details: map[string]any{
			"campaign_id": campaignID,
		},
	})

	slog.Info("teardown: infrastructure teardown complete", "campaign_id", campaignID)
	return nil
}

// stopLandingPageApps stops all running landing page applications for the campaign.
func (t *TeardownService) stopLandingPageApps(ctx context.Context, campaignID string) error {
	builds, err := t.deps.LandingPageRepo.ListBuildsByCampaign(ctx, campaignID)
	if err != nil {
		return fmt.Errorf("list builds: %w", err)
	}

	for _, build := range builds {
		if build.Status != "running" {
			continue
		}
		if err := t.deps.AppMgr.StopApp(ctx, build.ID); err != nil {
			slog.Warn("teardown: stop app failed",
				"build_id", build.ID, "campaign_id", campaignID, "error", err)
			continue
		}
		slog.Info("teardown: stopped landing page app",
			"build_id", build.ID, "campaign_id", campaignID)
	}
	return nil
}

// destroySSHKeys marks all SSH keys for the campaign as destroyed.
func (t *TeardownService) destroySSHKeys(ctx context.Context, campaignID string) error {
	keys, err := t.deps.EndpointRepo.ListSSHKeysByCampaign(ctx, campaignID)
	if err != nil {
		return fmt.Errorf("list ssh keys: %w", err)
	}

	for _, key := range keys {
		if err := t.deps.EndpointRepo.DestroySSHKey(ctx, key.ID); err != nil {
			slog.Warn("teardown: destroy ssh key failed",
				"key_id", key.ID, "campaign_id", campaignID, "error", err)
			continue
		}
		slog.Info("teardown: destroyed ssh key",
			"key_id", key.ID, "campaign_id", campaignID)
	}
	return nil
}

// releaseDomainAssociation clears the endpoint_domain_id on the campaign.
func (t *TeardownService) releaseDomainAssociation(ctx context.Context, campaignID string) error {
	emptyDomain := ""
	_, err := t.deps.CampaignRepo.Update(ctx, campaignID, repositories.CampaignUpdate{
		EndpointDomainID: &emptyDomain,
	})
	if err != nil {
		return fmt.Errorf("release domain: %w", err)
	}
	return nil
}
