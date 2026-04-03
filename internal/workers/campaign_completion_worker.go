package workers

import (
	"context"
	"log/slog"
	"time"

	"tackle/internal/campaign"
	"tackle/internal/repositories"
	campaignsvc "tackle/internal/services/campaign"
	auditsvc "tackle/internal/services/audit"
)

// CampaignCompletionWorker periodically checks for active campaigns where all
// emails have reached a terminal state and the end date (or grace period) has
// passed, then auto-transitions them to completed.
type CampaignCompletionWorker struct {
	svc          *campaignsvc.Service
	campaignRepo *repositories.CampaignRepository
	auditSvc     *auditsvc.AuditService
	interval     time.Duration
	logger       *slog.Logger
}

// NewCampaignCompletionWorker creates a completion detection worker.
func NewCampaignCompletionWorker(
	svc *campaignsvc.Service,
	campaignRepo *repositories.CampaignRepository,
	auditSvc *auditsvc.AuditService,
	interval time.Duration,
	logger *slog.Logger,
) *CampaignCompletionWorker {
	if interval <= 0 {
		interval = 60 * time.Second
	}
	return &CampaignCompletionWorker{
		svc:          svc,
		campaignRepo: campaignRepo,
		auditSvc:     auditSvc,
		interval:     interval,
		logger:       logger,
	}
}

// Start runs the worker in a blocking loop until ctx is cancelled.
func (w *CampaignCompletionWorker) Start(ctx context.Context) {
	w.logger.Info("campaign completion worker starting", slog.Duration("interval", w.interval))

	// Short startup delay.
	select {
	case <-time.After(15 * time.Second):
	case <-ctx.Done():
		return
	}

	ticker := time.NewTicker(w.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			w.check(ctx)
		case <-ctx.Done():
			w.logger.Info("campaign completion worker stopping")
			return
		}
	}
}

// check examines all active campaigns for completion eligibility.
func (w *CampaignCompletionWorker) check(ctx context.Context) {
	campaigns, err := w.svc.ListActiveCampaigns(ctx)
	if err != nil {
		w.logger.Error("completion worker: failed to list active campaigns", "error", err)
		return
	}

	now := time.Now().UTC()

	for _, c := range campaigns {
		allTerminal, err := w.campaignRepo.AllEmailsTerminal(ctx, c.ID)
		if err != nil {
			w.logger.Error("completion worker: failed to check email status",
				"campaign_id", c.ID, "error", err)
			continue
		}

		if !allTerminal {
			continue
		}

		// All emails are terminal. Check end_date and grace period.
		if !w.shouldComplete(c, now) {
			continue
		}

		// Transition to completed.
		w.logger.Info("completion worker: auto-completing campaign",
			"campaign_id", c.ID, "campaign_name", c.Name)

		_, err = w.svc.Transition(ctx, c.ID, campaign.StateCompleted,
			"auto-completed: all emails terminal and end date/grace period elapsed",
			"", "system", "system", "", "")
		if err != nil {
			w.logger.Error("completion worker: failed to transition campaign",
				"campaign_id", c.ID, "error", err)
			continue
		}

		// Audit log with actor=system.
		_ = w.auditSvc.Log(ctx, auditsvc.LogEntry{
			Category:     auditsvc.CategoryUserActivity,
			Severity:     auditsvc.SeverityInfo,
			ActorType:    auditsvc.ActorTypeSystem,
			Action:       "campaign.auto_completed",
			ResourceType: strPtr("campaign"),
			ResourceID:   strPtr(c.ID),
			Details: map[string]any{
				"campaign_name":     c.Name,
				"grace_period_hours": c.GracePeriodHours,
			},
		})
	}
}

// shouldComplete determines if a campaign with all-terminal emails should be completed.
func (w *CampaignCompletionWorker) shouldComplete(c campaignsvc.CampaignDTO, now time.Time) bool {
	// If end_date has passed, complete immediately.
	if c.EndDate != nil && now.After(*c.EndDate) {
		return true
	}

	// If no end_date, complete when grace period has elapsed since state_changed_at.
	gracePeriod := time.Duration(c.GracePeriodHours) * time.Hour
	if gracePeriod <= 0 {
		gracePeriod = 72 * time.Hour // default grace period
	}

	return now.After(c.StateChangedAt.Add(gracePeriod))
}

func strPtr(s string) *string { return &s }
