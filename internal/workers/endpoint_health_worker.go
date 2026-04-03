package workers

import (
	"context"
	"database/sql"
	"log/slog"
	"sync"
	"time"

	"tackle/internal/services/endpointmgmt"
)

// EndpointHealthWorker periodically checks for heartbeat timeouts on active endpoints,
// runs complementary health checks, and cleans up old heartbeat records.
type EndpointHealthWorker struct {
	svc      *endpointmgmt.Service
	db       *sql.DB
	interval time.Duration
	logger   *slog.Logger

	// Consecutive failure tracking.
	mu           sync.Mutex
	failureCount map[string]int // endpoint ID → consecutive failure count
	failureThreshold int       // default: 3
}

// NewEndpointHealthWorker creates an EndpointHealthWorker.
func NewEndpointHealthWorker(svc *endpointmgmt.Service, db *sql.DB, interval time.Duration, logger *slog.Logger) *EndpointHealthWorker {
	if interval <= 0 {
		interval = 30 * time.Second
	}
	return &EndpointHealthWorker{
		svc:              svc,
		db:               db,
		interval:         interval,
		logger:           logger,
		failureCount:     make(map[string]int),
		failureThreshold: 3,
	}
}

// Start runs the worker in a blocking loop until ctx is cancelled.
func (w *EndpointHealthWorker) Start(ctx context.Context) {
	w.logger.Info("endpoint health worker starting", slog.Duration("interval", w.interval))

	// Short startup delay.
	select {
	case <-time.After(10 * time.Second):
	case <-ctx.Done():
		return
	}

	ticker := time.NewTicker(w.interval)
	defer ticker.Stop()

	// Run heartbeat cleanup once per hour.
	cleanupTicker := time.NewTicker(1 * time.Hour)
	defer cleanupTicker.Stop()

	for {
		select {
		case <-ticker.C:
			w.runHealthCycle(ctx)
		case <-cleanupTicker.C:
			w.cleanupOldHeartbeats(ctx)
		case <-ctx.Done():
			w.logger.Info("endpoint health worker stopping")
			return
		}
	}
}

// runHealthCycle checks heartbeat timeouts with consecutive failure tracking.
func (w *EndpointHealthWorker) runHealthCycle(ctx context.Context) {
	// Get heartbeat timeout results from the service.
	timedOut, healthy, err := w.svc.CheckHeartbeatTimeoutsDetailed(ctx)
	if err != nil {
		w.logger.Error("endpoint health worker: check failed", "error", err)
		return
	}

	w.mu.Lock()
	defer w.mu.Unlock()

	// Reset failure count for healthy endpoints.
	for _, epID := range healthy {
		delete(w.failureCount, epID)
	}

	// Increment failure count for timed-out endpoints.
	for _, epID := range timedOut {
		w.failureCount[epID]++
		count := w.failureCount[epID]

		if count < w.failureThreshold {
			w.logger.Warn("endpoint heartbeat timeout",
				"endpoint_id", epID,
				"consecutive_failures", count,
				"threshold", w.failureThreshold,
			)
			continue
		}

		// Threshold reached — transition to error.
		w.logger.Error("endpoint heartbeat timeout threshold reached",
			"endpoint_id", epID,
			"consecutive_failures", count,
		)
		if err := w.svc.TransitionToError(ctx, epID, "heartbeat timeout: 3 consecutive failures"); err != nil {
			w.logger.Error("endpoint health worker: transition to error failed", "endpoint_id", epID, "error", err)
		}
		delete(w.failureCount, epID) // Reset after transition.
	}
}

// cleanupOldHeartbeats removes heartbeat records older than 7 days.
func (w *EndpointHealthWorker) cleanupOldHeartbeats(ctx context.Context) {
	result, err := w.db.ExecContext(ctx,
		`DELETE FROM endpoint_heartbeats WHERE created_at < now() - interval '7 days'`)
	if err != nil {
		w.logger.Error("endpoint health worker: cleanup failed", "error", err)
		return
	}
	n, _ := result.RowsAffected()
	if n > 0 {
		w.logger.Info("endpoint health worker: cleaned up old heartbeats", "deleted", n)
	}
}
