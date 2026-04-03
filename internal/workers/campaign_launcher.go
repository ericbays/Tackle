package workers

import (
	"context"
	"log/slog"
	"time"

	"tackle/internal/campaign"
	campaignsvc "tackle/internal/services/campaign"
)

// CampaignAutoLaunchWorker periodically checks for Ready campaigns with scheduled
// launch times in the past and transitions them to Active.
type CampaignAutoLaunchWorker struct {
	svc      *campaignsvc.Service
	interval time.Duration
	logger   *slog.Logger
}

// NewCampaignAutoLaunchWorker creates a worker that checks for auto-launch candidates.
func NewCampaignAutoLaunchWorker(svc *campaignsvc.Service, interval time.Duration, logger *slog.Logger) *CampaignAutoLaunchWorker {
	if interval <= 0 {
		interval = 30 * time.Second
	}
	return &CampaignAutoLaunchWorker{svc: svc, interval: interval, logger: logger}
}

// Start runs the worker in a blocking loop until ctx is cancelled.
func (w *CampaignAutoLaunchWorker) Start(ctx context.Context) {
	w.logger.Info("campaign auto-launch worker starting", slog.Duration("interval", w.interval))

	ticker := time.NewTicker(w.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			w.check(ctx)
		case <-ctx.Done():
			w.logger.Info("campaign auto-launch worker stopping")
			return
		}
	}
}

// check fetches campaigns ready for auto-launch and transitions them.
func (w *CampaignAutoLaunchWorker) check(ctx context.Context) {
	campaigns, err := w.svc.GetReadyForAutoLaunch(ctx)
	if err != nil {
		w.logger.Error("auto-launch worker: failed to fetch candidates", "error", err)
		return
	}

	for _, c := range campaigns {
		w.logger.Info("auto-launch worker: launching campaign",
			slog.String("campaign_id", c.ID),
			slog.String("campaign_name", c.Name))

		_, err := w.svc.Transition(ctx, c.ID, campaign.StateActive,
			"scheduled auto-launch", "", "system", "system", "", "auto-launch")
		if err != nil {
			w.logger.Error("auto-launch worker: failed to launch",
				slog.String("campaign_id", c.ID), "error", err)
		}
	}
}
