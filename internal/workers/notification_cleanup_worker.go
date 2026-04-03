package workers

import (
	"context"
	"database/sql"
	"log/slog"
	"time"
)

// NotificationCleanupWorker periodically purges old notifications.
// - Deletes all notifications older than 90 days.
// - Deletes read notifications older than 30 days.
type NotificationCleanupWorker struct {
	db       *sql.DB
	interval time.Duration
	logger   *slog.Logger
}

// NewNotificationCleanupWorker creates a cleanup worker.
func NewNotificationCleanupWorker(db *sql.DB, logger *slog.Logger) *NotificationCleanupWorker {
	return &NotificationCleanupWorker{
		db:       db,
		interval: 24 * time.Hour,
		logger:   logger,
	}
}

// Start runs the cleanup loop. Blocks until ctx is cancelled.
func (w *NotificationCleanupWorker) Start(ctx context.Context) {
	w.logger.Info("notification_cleanup_worker: started", "interval", w.interval)
	// Run once immediately, then on interval.
	w.run(ctx)
	ticker := time.NewTicker(w.interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			w.logger.Info("notification_cleanup_worker: stopped")
			return
		case <-ticker.C:
			w.run(ctx)
		}
	}
}

func (w *NotificationCleanupWorker) run(ctx context.Context) {
	// Delete all notifications older than 90 days.
	cutoff90 := time.Now().UTC().Add(-90 * 24 * time.Hour)
	res, err := w.db.ExecContext(ctx, `
		DELETE FROM notifications WHERE created_at < $1`, cutoff90)
	if err != nil {
		w.logger.Error("notification_cleanup_worker: delete old", "error", err)
	} else if n, _ := res.RowsAffected(); n > 0 {
		w.logger.Info("notification_cleanup_worker: purged old notifications", "count", n)
	}

	// Delete read notifications older than 30 days.
	cutoff30 := time.Now().UTC().Add(-30 * 24 * time.Hour)
	res, err = w.db.ExecContext(ctx, `
		DELETE FROM notifications WHERE is_read = TRUE AND created_at < $1`, cutoff30)
	if err != nil {
		w.logger.Error("notification_cleanup_worker: delete read", "error", err)
	} else if n, _ := res.RowsAffected(); n > 0 {
		w.logger.Info("notification_cleanup_worker: purged old read notifications", "count", n)
	}

	// Also clean up expired notifications.
	res, err = w.db.ExecContext(ctx, `
		DELETE FROM notifications WHERE expires_at IS NOT NULL AND expires_at < now()`)
	if err != nil {
		w.logger.Error("notification_cleanup_worker: delete expired", "error", err)
	} else if n, _ := res.RowsAffected(); n > 0 {
		w.logger.Info("notification_cleanup_worker: purged expired notifications", "count", n)
	}
}
