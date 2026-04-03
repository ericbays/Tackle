package rbac

import (
	"context"
	"database/sql"
	"fmt"
)

// Resolver resolves the full permission set for a user from the database.
type Resolver struct {
	db *sql.DB
}

// NewResolver creates a Resolver backed by the provided database connection.
func NewResolver(db *sql.DB) *Resolver {
	return &Resolver{db: db}
}

// RoleForUser returns the role name assigned to userID, or an error if none.
func (r *Resolver) RoleForUser(ctx context.Context, userID string) (string, error) {
	const q = `
		SELECT ro.name
		FROM user_roles ur
		JOIN roles ro ON ro.id = ur.role_id
		WHERE ur.user_id = $1
		LIMIT 1`

	var name string
	err := r.db.QueryRowContext(ctx, q, userID).Scan(&name)
	if err == sql.ErrNoRows {
		return "", fmt.Errorf("rbac: no role assigned to user %s", userID)
	}
	if err != nil {
		return "", fmt.Errorf("rbac: query role for user: %w", err)
	}
	return name, nil
}

// Resolve returns the full permission list for the user identified by userID.
// If the user has the admin role, it returns all registered permissions.
// Results are NOT cached here — caching is handled via JWT claims.
func (r *Resolver) Resolve(ctx context.Context, userID string) ([]Permission, error) {
	role, err := r.RoleForUser(ctx, userID)
	if err != nil {
		return nil, err
	}

	if IsAdmin(role) {
		return All(), nil
	}

	const q = `
		SELECT p.resource_type || ':' || p.action
		FROM role_permissions rp
		JOIN permissions p ON p.id = rp.permission_id
		JOIN roles ro ON ro.id = rp.role_id
		JOIN user_roles ur ON ur.role_id = ro.id
		WHERE ur.user_id = $1`

	rows, err := r.db.QueryContext(ctx, q, userID)
	if err != nil {
		return nil, fmt.Errorf("rbac: query permissions for user: %w", err)
	}
	defer rows.Close()

	var perms []Permission
	for rows.Next() {
		var p string
		if err := rows.Scan(&p); err != nil {
			return nil, fmt.Errorf("rbac: scan permission: %w", err)
		}
		perms = append(perms, Permission(p))
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("rbac: iterate permissions: %w", err)
	}
	return perms, nil
}
