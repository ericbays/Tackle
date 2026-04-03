package workers

import (
	"context"
	"log/slog"
	"time"

	targetsvc "tackle/internal/services/target"
)

// TargetPurgeWorker periodically purges PII from soft-deleted targets
// that have exceeded the configured retention period.
type TargetPurgeWorker struct {
	svc           *targetsvc.Service
	retentionDays int
	interval      time.Duration
	logger        *slog.Logger
}

// NewTargetPurgeWorker creates a target purge worker.
func NewTargetPurgeWorker(
	svc *targetsvc.Service,
	retentionDays int,
	interval time.Duration,
	logger *slog.Logger,
) *TargetPurgeWorker {
	if retentionDays <= 0 {
		retentionDays = 365
	}
	if interval <= 0 {
		interval = 24 * time.Hour
	}
	return &TargetPurgeWorker{
		svc:           svc,
		retentionDays: retentionDays,
		interval:      interval,
		logger:        logger,
	}
}

// Start runs the worker in a blocking loop until ctx is cancelled.
func (w *TargetPurgeWorker) Start(ctx context.Context) {
	w.logger.Info("target purge worker starting",
		slog.Duration("interval", w.interval),
		slog.Int("retention_days", w.retentionDays))

	// Delay startup to avoid running immediately on boot.
	select {
	case <-time.After(5 * time.Minute):
	case <-ctx.Done():
		return
	}

	// Run once immediately after startup delay.
	w.purge(ctx)

	ticker := time.NewTicker(w.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			w.purge(ctx)
		case <-ctx.Done():
			w.logger.Info("target purge worker stopping")
			return
		}
	}
}

// purge executes the purge operation.
func (w *TargetPurgeWorker) purge(ctx context.Context) {
	count, err := w.svc.PurgeExpired(ctx, w.retentionDays)
	if err != nil {
		w.logger.Error("target purge worker: failed to purge", "error", err)
		return
	}
	if count > 0 {
		w.logger.Info("target purge worker: purged expired targets",
			slog.Int64("count", count),
			slog.Int("retention_days", w.retentionDays))
	}
}
