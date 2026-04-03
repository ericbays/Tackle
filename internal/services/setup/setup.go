// Package setup provides first-launch detection for the Tackle platform.
package setup

import (
	"context"
	"database/sql"
	"fmt"
)

// IsSetupRequired returns true if no user accounts exist in the database,
// indicating a fresh installation that requires initial admin setup.
// Queries the database on every call — no in-memory caching (REQ-AUTH-002).
func IsSetupRequired(ctx context.Context, db *sql.DB) (bool, error) {
	var count int
	err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM users`).Scan(&count)
	if err != nil {
		return false, fmt.Errorf("setup: count users: %w", err)
	}
	return count == 0, nil
}
