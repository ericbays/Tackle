package auth

import (
	"context"
	"database/sql"
	"fmt"

	"golang.org/x/crypto/bcrypt"
)

// HistoryChecker enforces password reuse policy against stored password history.
type HistoryChecker struct {
	db *sql.DB
}

// NewHistoryChecker creates a HistoryChecker backed by the given database.
func NewHistoryChecker(db *sql.DB) *HistoryChecker {
	return &HistoryChecker{db: db}
}

// IsReused reports whether newPassword matches any of the last `limit` passwords
// stored in the password history for userID.
func (h *HistoryChecker) IsReused(ctx context.Context, userID, newPassword string, limit int) (bool, error) {
	const q = `
		SELECT password_hash
		FROM password_history
		WHERE user_id = $1
		ORDER BY created_at DESC
		LIMIT $2`

	rows, err := h.db.QueryContext(ctx, q, userID, limit)
	if err != nil {
		return false, fmt.Errorf("password history: query: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var storedHash string
		if err := rows.Scan(&storedHash); err != nil {
			return false, fmt.Errorf("password history: scan: %w", err)
		}
		if bcrypt.CompareHashAndPassword([]byte(storedHash), []byte(newPassword)) == nil {
			return true, nil
		}
	}
	if err := rows.Err(); err != nil {
		return false, fmt.Errorf("password history: rows: %w", err)
	}
	return false, nil
}

// Record inserts a new password hash into the history for userID.
func (h *HistoryChecker) Record(ctx context.Context, userID, hash string) error {
	const q = `INSERT INTO password_history (user_id, password_hash) VALUES ($1, $2)`
	if _, err := h.db.ExecContext(ctx, q, userID, hash); err != nil {
		return fmt.Errorf("password history: record: %w", err)
	}
	return nil
}
