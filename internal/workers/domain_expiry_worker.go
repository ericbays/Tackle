package workers

import (
	"context"
	"log/slog"
	"time"

	domainsvc "tackle/internal/services/domain"
)

// DomainExpiryWorker periodically syncs domain expiry dates from registrars
// and checks for upcoming expirations to generate notifications.
type DomainExpiryWorker struct {
	svc      *domainsvc.Service
	interval time.Duration
	logger   *slog.Logger
}

// NewDomainExpiryWorker creates a domain expiry sync worker.
func NewDomainExpiryWorker(svc *domainsvc.Service, interval time.Duration, logger *slog.Logger) *DomainExpiryWorker {
	if interval <= 0 {
		interval = 24 * time.Hour
	}
	return &DomainExpiryWorker{svc: svc, interval: interval, logger: logger}
}

// Start runs the worker in a blocking loop until ctx is cancelled.
func (w *DomainExpiryWorker) Start(ctx context.Context) {
	w.logger.Info("domain expiry worker starting", slog.Duration("interval", w.interval))

	// Run once on startup after a short delay.
	select {
	case <-time.After(30 * time.Second):
		w.run(ctx)
	case <-ctx.Done():
		return
	}

	ticker := time.NewTicker(w.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			w.run(ctx)
		case <-ctx.Done():
			w.logger.Info("domain expiry worker stopping")
			return
		}
	}
}

// run executes a single sync + notification cycle.
func (w *DomainExpiryWorker) run(ctx context.Context) {
	if err := w.svc.SyncExpiryDates(ctx); err != nil {
		w.logger.Error("domain expiry worker: sync failed", "error", err)
	} else {
		w.logger.Debug("domain expiry worker: sync complete")
	}

	if err := w.svc.CheckExpiryNotifications(ctx); err != nil {
		w.logger.Error("domain expiry worker: notification check failed", "error", err)
	} else {
		w.logger.Debug("domain expiry worker: notification check complete")
	}
}
