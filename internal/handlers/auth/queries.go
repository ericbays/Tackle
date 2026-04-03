package auth

import (
	"context"
	"database/sql"
	"fmt"

	"tackle/internal/models"
)

// userRow holds everything needed for auth from a single DB query.
type userRow struct {
	user        models.User
	roleName    string
	permissions []string
}

// findUserByUsernameOrEmail looks up a user by username or email, their assigned role,
// and resolves their full permission list from the database.
func findUserByUsernameOrEmail(ctx context.Context, db *sql.DB, usernameOrEmail string) (*userRow, error) {
	const q = `
		SELECT u.id, u.email, u.username, u.password_hash, u.display_name,
		       u.is_initial_admin, u.auth_provider, u.external_id, u.status,
		       u.force_password_change, u.created_at, u.updated_at,
		       COALESCE(ro.name, '') AS role_name
		FROM users u
		LEFT JOIN user_roles ur ON ur.user_id = u.id
		LEFT JOIN roles ro ON ro.id = ur.role_id
		WHERE u.username = $1 OR u.email = $1
		LIMIT 1`

	row := db.QueryRowContext(ctx, q, usernameOrEmail)
	var ur userRow
	var pwHash sql.NullString
	var extID sql.NullString
	err := row.Scan(
		&ur.user.ID, &ur.user.Email, &ur.user.Username, &pwHash,
		&ur.user.DisplayName, &ur.user.IsInitialAdmin, &ur.user.AuthProvider,
		&extID, &ur.user.Status, &ur.user.ForcePasswordChange,
		&ur.user.CreatedAt, &ur.user.UpdatedAt, &ur.roleName,
	)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("user not found")
	}
	if err != nil {
		return nil, fmt.Errorf("find user: %w", err)
	}
	if pwHash.Valid {
		ur.user.PasswordHash = &pwHash.String
	}
	if extID.Valid {
		ur.user.ExternalID = &extID.String
	}

	ur.permissions, err = resolvePermissions(ctx, db, ur.roleName)
	if err != nil {
		return nil, fmt.Errorf("find user: resolve permissions: %w", err)
	}
	return &ur, nil
}

// findUserByID looks up a user by UUID.
func findUserByID(ctx context.Context, db *sql.DB, userID string) (*userRow, error) {
	const q = `
		SELECT u.id, u.email, u.username, u.password_hash, u.display_name,
		       u.is_initial_admin, u.auth_provider, u.external_id, u.status,
		       u.force_password_change, u.created_at, u.updated_at,
		       COALESCE(ro.name, '') AS role_name
		FROM users u
		LEFT JOIN user_roles ur ON ur.user_id = u.id
		LEFT JOIN roles ro ON ro.id = ur.role_id
		WHERE u.id = $1`

	row := db.QueryRowContext(ctx, q, userID)
	var ur userRow
	var pwHash sql.NullString
	var extID sql.NullString
	err := row.Scan(
		&ur.user.ID, &ur.user.Email, &ur.user.Username, &pwHash,
		&ur.user.DisplayName, &ur.user.IsInitialAdmin, &ur.user.AuthProvider,
		&extID, &ur.user.Status, &ur.user.ForcePasswordChange,
		&ur.user.CreatedAt, &ur.user.UpdatedAt, &ur.roleName,
	)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("user not found")
	}
	if err != nil {
		return nil, fmt.Errorf("find user by id: %w", err)
	}
	if pwHash.Valid {
		ur.user.PasswordHash = &pwHash.String
	}
	if extID.Valid {
		ur.user.ExternalID = &extID.String
	}

	ur.permissions, err = resolvePermissions(ctx, db, ur.roleName)
	if err != nil {
		return nil, fmt.Errorf("find user by id: resolve permissions: %w", err)
	}
	return &ur, nil
}

// resolvePermissions returns the permission strings for a role.
// Admin role returns all permissions (represented as a wildcard sentinel, handled by caller).
func resolvePermissions(ctx context.Context, db *sql.DB, roleName string) ([]string, error) {
	if roleName == "" {
		return nil, nil
	}
	if roleName == "admin" {
		// Admin short-circuit: return all permissions from registry.
		// We query the permissions table to stay consistent with the DB.
		return allPermissionsFromDB(ctx, db)
	}
	const q = `
		SELECT p.resource_type || ':' || p.action
		FROM role_permissions rp
		JOIN roles ro ON ro.id = rp.role_id
		JOIN permissions p ON p.id = rp.permission_id
		WHERE ro.name = $1`
	rows, err := db.QueryContext(ctx, q, roleName)
	if err != nil {
		return nil, fmt.Errorf("resolve permissions: %w", err)
	}
	defer rows.Close()
	var perms []string
	for rows.Next() {
		var p string
		if err := rows.Scan(&p); err != nil {
			return nil, fmt.Errorf("resolve permissions: scan: %w", err)
		}
		perms = append(perms, p)
	}
	return perms, rows.Err()
}

func allPermissionsFromDB(ctx context.Context, db *sql.DB) ([]string, error) {
	const q = `SELECT resource_type || ':' || action FROM permissions ORDER BY resource_type, action`
	rows, err := db.QueryContext(ctx, q)
	if err != nil {
		return nil, fmt.Errorf("all permissions: %w", err)
	}
	defer rows.Close()
	var perms []string
	for rows.Next() {
		var p string
		if err := rows.Scan(&p); err != nil {
			return nil, fmt.Errorf("all permissions: scan: %w", err)
		}
		perms = append(perms, p)
	}
	return perms, rows.Err()
}
