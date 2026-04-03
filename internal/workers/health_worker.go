// Package workers contains background goroutine workers for scheduled tasks.
package workers

import (
	"context"
	"log/slog"
	"math/rand"
	"sync"
	"time"

	healthsvc "tackle/internal/services/health"
)

// HealthCheckWorker runs scheduled domain health checks.
type HealthCheckWorker struct {
	svc      *healthsvc.Service
	interval time.Duration
	logger   *slog.Logger
}

// NewHealthCheckWorker creates a HealthCheckWorker with the given interval.
func NewHealthCheckWorker(svc *healthsvc.Service, interval time.Duration, logger *slog.Logger) *HealthCheckWorker {
	if interval <= 0 {
		interval = 6 * time.Hour
	}
	return &HealthCheckWorker{svc: svc, interval: interval, logger: logger}
}

// Start runs the worker in a blocking loop until ctx is cancelled.
func (w *HealthCheckWorker) Start(ctx context.Context) {
	w.logger.Info("health check worker starting", slog.Duration("interval", w.interval))

	// Run once at startup after a short delay to avoid thundering herd.
	select {
	case <-time.After(30 * time.Second):
		w.runBatch(ctx)
	case <-ctx.Done():
		return
	}

	ticker := time.NewTicker(w.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			w.runBatch(ctx)
		case <-ctx.Done():
			w.logger.Info("health check worker stopping")
			return
		}
	}
}

// runBatch fetches all active domain IDs and runs health checks with staggering.
func (w *HealthCheckWorker) runBatch(ctx context.Context) {
	ids, err := w.svc.GetAllActiveProfileIDs(ctx)
	if err != nil {
		w.logger.Error("health check worker: failed to fetch active domains", "error", err)
		return
	}
	if len(ids) == 0 {
		return
	}

	w.logger.Info("health check worker: running batch", slog.Int("domain_count", len(ids)))

	// Stagger checks: spread evenly across the interval.
	stagger := w.interval / time.Duration(len(ids)+1)
	if stagger > 5*time.Minute {
		stagger = 5 * time.Minute
	}

	const maxConcurrency = 3
	sem := make(chan struct{}, maxConcurrency)
	var wg sync.WaitGroup

	for _, id := range ids {
		select {
		case <-ctx.Done():
			return
		default:
		}

		wg.Add(1)
		go func(domainID string) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			if err := w.svc.RunHealthCheckForScheduler(ctx, domainID); err != nil {
				w.logger.Error("health check worker: check failed", "domain_id", domainID, "error", err)
			}
		}(id)

		// Stagger.
		select {
		case <-time.After(stagger):
		case <-ctx.Done():
			wg.Wait()
			return
		}
	}

	wg.Wait()
	w.logger.Info("health check worker: batch complete", slog.Int("domain_count", len(ids)))
}

// CategorizationWorker runs scheduled domain categorization checks.
type CategorizationWorker struct {
	svc      categorizationRunner
	interval time.Duration
	logger   *slog.Logger
}

// categorizationRunner is the minimal interface needed by CategorizationWorker.
type categorizationRunner interface {
	GetAllActiveProfileIDs(ctx context.Context) ([]string, error)
	RunCategorizationForScheduler(ctx context.Context, domainProfileID string) error
}

// NewCategorizationWorker creates a CategorizationWorker with the given interval.
func NewCategorizationWorker(svc categorizationRunner, interval time.Duration, logger *slog.Logger) *CategorizationWorker {
	if interval <= 0 {
		interval = 24 * time.Hour
	}
	return &CategorizationWorker{svc: svc, interval: interval, logger: logger}
}

// Start runs the worker in a blocking loop until ctx is cancelled.
func (w *CategorizationWorker) Start(ctx context.Context) {
	w.logger.Info("categorization worker starting", slog.Duration("interval", w.interval))

	// Run once at startup after a delay.
	select {
	case <-time.After(2 * time.Minute):
		w.runBatch(ctx)
	case <-ctx.Done():
		return
	}

	ticker := time.NewTicker(w.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			w.runBatch(ctx)
		case <-ctx.Done():
			w.logger.Info("categorization worker stopping")
			return
		}
	}
}

// runBatch fetches all active domain IDs and runs categorization checks with jitter.
func (w *CategorizationWorker) runBatch(ctx context.Context) {
	ids, err := w.svc.GetAllActiveProfileIDs(ctx)
	if err != nil {
		w.logger.Error("categorization worker: failed to fetch active domains", "error", err)
		return
	}
	if len(ids) == 0 {
		return
	}

	w.logger.Info("categorization worker: running batch", slog.Int("domain_count", len(ids)))

	for _, id := range ids {
		select {
		case <-ctx.Done():
			return
		default:
		}

		if err := w.svc.RunCategorizationForScheduler(ctx, id); err != nil {
			w.logger.Error("categorization worker: check failed", "domain_id", id, "error", err)
		}

		// Random jitter: 1-5 seconds between each domain to avoid abuse detection.
		jitter := time.Duration(1+rand.Intn(4)) * time.Second //nolint:gosec
		select {
		case <-time.After(jitter):
		case <-ctx.Done():
			return
		}
	}

	w.logger.Info("categorization worker: batch complete", slog.Int("domain_count", len(ids)))
}
