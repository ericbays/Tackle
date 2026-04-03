package roles

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
)

// encodeJSON writes v as JSON to w (caller must set Content-Type and status).
func encodeJSON(w http.ResponseWriter, v interface{}) {
	_ = json.NewEncoder(w).Encode(v)
}

// fetchRolePermissions returns the list of "resource:action" strings for a role ID.
func fetchRolePermissions(ctx context.Context, db *sql.DB, roleID string) ([]string, error) {
	const q = `
		SELECT p.resource_type || ':' || p.action
		FROM role_permissions rp
		JOIN permissions p ON p.id = rp.permission_id
		WHERE rp.role_id = $1
		ORDER BY p.resource_type, p.action`

	rows, err := db.QueryContext(ctx, q, roleID)
	if err != nil {
		return nil, fmt.Errorf("query role permissions: %w", err)
	}
	defer rows.Close()

	var perms []string
	for rows.Next() {
		var p string
		if err := rows.Scan(&p); err != nil {
			return nil, fmt.Errorf("scan permission: %w", err)
		}
		perms = append(perms, p)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate permissions: %w", err)
	}
	if perms == nil {
		perms = []string{}
	}
	return perms, nil
}

// insertRolePermissions resolves permission strings to IDs and inserts into role_permissions.
// Runs within the provided transaction.
func insertRolePermissions(ctx context.Context, tx *sql.Tx, roleID string, permissions []string) error {
	for _, perm := range permissions {
		parts := strings.SplitN(perm, ":", 2)
		if len(parts) != 2 {
			return fmt.Errorf("invalid permission format: %s", perm)
		}
		resource, action := parts[0], parts[1]

		const q = `
			INSERT INTO role_permissions (role_id, permission_id)
			SELECT $1, p.id FROM permissions p
			WHERE p.resource_type = $2 AND p.action = $3
			ON CONFLICT DO NOTHING`

		_, err := tx.ExecContext(ctx, q, roleID, resource, action)
		if err != nil {
			return fmt.Errorf("insert permission %s: %w", perm, err)
		}
	}
	return nil
}

// strPtr returns a pointer to s.
func strPtr(s string) *string { return &s }

// isUniqueViolation reports whether err is a PostgreSQL unique constraint violation.
func isUniqueViolation(err error) bool {
	if err == nil {
		return false
	}
	return strings.Contains(err.Error(), "23505") || strings.Contains(err.Error(), "unique constraint")
}
