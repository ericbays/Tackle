package authprovider

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	"github.com/google/uuid"

	"tackle/internal/repositories"
	auditsvc "tackle/internal/services/audit"
)

// ProvisionedUser holds the result of resolving an external user.
type ProvisionedUser struct {
	UserID      string
	Username    string
	Email       string
	DisplayName string
	RoleName    string
	Permissions []string
	IsNew       bool
	// NeedsLinking is set when an existing user was found by email match
	// but no auth identity link exists yet. The UI must prompt the user.
	NeedsLinking bool
	// MatchedUserID is populated when NeedsLinking is true.
	MatchedUserID string
}

// ProvisioningService resolves external identities into local user accounts.
type ProvisioningService struct {
	db     *sql.DB
	idRepo *repositories.AuthIdentityRepository
	rmRepo *repositories.RoleMappingRepository
	audit  *auditsvc.AuditService
}

// NewProvisioningService creates a ProvisioningService.
func NewProvisioningService(
	db *sql.DB,
	idRepo *repositories.AuthIdentityRepository,
	rmRepo *repositories.RoleMappingRepository,
	audit *auditsvc.AuditService,
) *ProvisioningService {
	return &ProvisioningService{db: db, idRepo: idRepo, rmRepo: rmRepo, audit: audit}
}

// ResolveExternalUser finds or provisions a local user for the given external identity.
func (s *ProvisioningService) ResolveExternalUser(
	ctx context.Context,
	provider repositories.AuthProvider,
	claims ExternalClaims,
) (ProvisionedUser, error) {
	// 1. Check if an auth identity already exists for this (providerConfigID, subject).
	identity, err := s.idRepo.GetByExternalSubject(ctx, provider.ID, claims.Subject)
	if err == nil {
		// Identity found — fetch user and re-evaluate role mapping.
		return s.refreshExistingUser(ctx, provider, identity, claims)
	}
	if !strings.Contains(err.Error(), "auth identity not found") {
		return ProvisionedUser{}, fmt.Errorf("resolve external user: lookup identity: %w", err)
	}

	// 2. No identity — check for email match.
	if claims.Email != "" {
		matched, err := s.findUserByEmail(ctx, claims.Email)
		if err == nil {
			// Email match found — signal linking required. Do NOT auto-link.
			return ProvisionedUser{NeedsLinking: true, MatchedUserID: matched}, nil
		}
	}

	// 3. Auto-provision if configured.
	if !provider.AutoProvision {
		return ProvisionedUser{}, fmt.Errorf("resolve external user: auto-provision disabled for this provider")
	}
	return s.provisionUser(ctx, provider, claims)
}

func (s *ProvisioningService) refreshExistingUser(
	ctx context.Context,
	provider repositories.AuthProvider,
	identity repositories.AuthIdentity,
	claims ExternalClaims,
) (ProvisionedUser, error) {
	row, err := s.queryUserWithRole(ctx, identity.UserID)
	if err != nil {
		return ProvisionedUser{}, fmt.Errorf("refresh external user: %w", err)
	}

	// Re-evaluate group-to-role mapping on every login.
	defaultRoleID := ""
	if provider.DefaultRoleID != nil {
		defaultRoleID = *provider.DefaultRoleID
	}
	newRoleID, err := s.rmRepo.ResolveRole(ctx, provider.ID, claims.Groups, defaultRoleID)
	if err != nil {
		// Non-fatal — keep existing role.
		newRoleID = row.roleID
	}
	if newRoleID != "" && newRoleID != row.roleID {
		if updateErr := s.updateUserRole(ctx, identity.UserID, newRoleID); updateErr == nil {
			s.emitAudit(ctx, "auth.role_changed_by_mapping", map[string]any{
				"user_id": identity.UserID, "provider_id": provider.ID,
				"old_role_id": row.roleID, "new_role_id": newRoleID,
			})
			// Re-fetch with updated role.
			row, _ = s.queryUserWithRole(ctx, identity.UserID)
		}
	}

	return ProvisionedUser{
		UserID:      identity.UserID,
		Username:    row.username,
		Email:       row.email,
		DisplayName: row.displayName,
		RoleName:    row.roleName,
		Permissions: row.permissions,
	}, nil
}

func (s *ProvisioningService) provisionUser(
	ctx context.Context,
	provider repositories.AuthProvider,
	claims ExternalClaims,
) (ProvisionedUser, error) {
	// Resolve role.
	defaultRoleID := ""
	if provider.DefaultRoleID != nil {
		defaultRoleID = *provider.DefaultRoleID
	}
	roleID, err := s.rmRepo.ResolveRole(ctx, provider.ID, claims.Groups, defaultRoleID)
	if err != nil {
		roleID = defaultRoleID
	}

	username := sanitizeUsername(claims.Username, claims.Email, claims.Subject)
	email := claims.Email
	if email == "" {
		email = username + "@external"
	}
	displayName := claims.Name
	if displayName == "" {
		displayName = username
	}

	userID := uuid.New().String()

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return ProvisionedUser{}, fmt.Errorf("provision user: begin tx: %w", err)
	}
	defer tx.Rollback() //nolint:errcheck

	// Insert user (no password_hash — external auth only).
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO users (id, email, username, display_name, auth_provider, status)
		VALUES ($1, $2, $3, $4, $5, 'active')`,
		userID, email, username, displayName, string(provider.Type),
	); err != nil {
		return ProvisionedUser{}, fmt.Errorf("provision user: insert: %w", err)
	}

	// Assign role if resolved.
	roleName := ""
	if roleID != "" {
		if _, err := tx.ExecContext(ctx,
			`INSERT INTO user_roles (id, user_id, role_id) VALUES ($1, $2, $3)`,
			uuid.New().String(), userID, roleID,
		); err != nil {
			return ProvisionedUser{}, fmt.Errorf("provision user: assign role: %w", err)
		}
		// Resolve role name.
		_ = tx.QueryRowContext(ctx, `SELECT name FROM roles WHERE id = $1`, roleID).Scan(&roleName)
	}

	if err := tx.Commit(); err != nil {
		return ProvisionedUser{}, fmt.Errorf("provision user: commit: %w", err)
	}

	// Create auth identity link.
	_, err = s.idRepo.Create(ctx, repositories.AuthIdentity{
		UserID:           userID,
		ProviderType:     provider.Type,
		ProviderConfigID: provider.ID,
		ExternalSubject:  claims.Subject,
		ExternalEmail:    &email,
	})
	if err != nil {
		// Log but don't fail — user exists, identity can be re-linked.
		s.emitAudit(ctx, "auth.user_auto_provisioned", map[string]any{
			"user_id": userID, "provider_id": provider.ID, "warning": "identity link failed: " + err.Error(),
		})
	} else {
		s.emitAudit(ctx, "auth.user_auto_provisioned", map[string]any{
			"user_id": userID, "provider_id": provider.ID, "role_id": roleID,
		})
	}

	return ProvisionedUser{
		UserID:      userID,
		Username:    username,
		Email:       email,
		DisplayName: displayName,
		RoleName:    roleName,
		IsNew:       true,
	}, nil
}

type userRow struct {
	username    string
	email       string
	displayName string
	roleID      string
	roleName    string
	permissions []string
}

func (s *ProvisioningService) queryUserWithRole(ctx context.Context, userID string) (userRow, error) {
	const q = `
		SELECT u.username, u.email, u.display_name,
		       COALESCE(ro.id, ''),
		       COALESCE(ro.name, '')
		FROM users u
		LEFT JOIN user_roles ur ON ur.user_id = u.id
		LEFT JOIN roles ro ON ro.id = ur.role_id
		WHERE u.id = $1`
	var r userRow
	if err := s.db.QueryRowContext(ctx, q, userID).Scan(
		&r.username, &r.email, &r.displayName, &r.roleID, &r.roleName,
	); err != nil {
		return userRow{}, fmt.Errorf("query user with role: %w", err)
	}
	r.permissions = s.resolvePermissions(ctx, r.roleName)
	return r, nil
}

func (s *ProvisioningService) resolvePermissions(ctx context.Context, roleName string) []string {
	if roleName == "" {
		return nil
	}
	var q string
	var args []any
	if roleName == "admin" {
		q = `SELECT resource_type || ':' || action FROM permissions ORDER BY resource_type, action`
	} else {
		q = `SELECT p.resource_type || ':' || p.action
		     FROM role_permissions rp
		     JOIN roles ro ON ro.id = rp.role_id
		     JOIN permissions p ON p.id = rp.permission_id
		     WHERE ro.name = $1`
		args = []any{roleName}
	}
	rows, err := s.db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil
	}
	defer rows.Close()
	var perms []string
	for rows.Next() {
		var p string
		if rows.Scan(&p) == nil {
			perms = append(perms, p)
		}
	}
	return perms
}

func (s *ProvisioningService) findUserByEmail(ctx context.Context, email string) (string, error) {
	var id string
	err := s.db.QueryRowContext(ctx, `SELECT id FROM users WHERE email = $1 LIMIT 1`, email).Scan(&id)
	if err != nil {
		return "", err
	}
	return id, nil
}

func (s *ProvisioningService) updateUserRole(ctx context.Context, userID, roleID string) error {
	// Upsert: delete existing assignment then insert new one.
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback() //nolint:errcheck
	if _, err := tx.ExecContext(ctx, `DELETE FROM user_roles WHERE user_id = $1`, userID); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx,
		`INSERT INTO user_roles (id, user_id, role_id) VALUES ($1, $2, $3)`,
		uuid.New().String(), userID, roleID,
	); err != nil {
		return err
	}
	return tx.Commit()
}

func (s *ProvisioningService) emitAudit(ctx context.Context, action string, details map[string]any) {
	if s.audit == nil {
		return
	}
	_ = s.audit.Log(ctx, auditsvc.LogEntry{
		Category:  auditsvc.CategoryUserActivity,
		Severity:  auditsvc.SeverityInfo,
		ActorType: auditsvc.ActorTypeSystem,
		Action:    action,
		Details:   details,
	})
}

func sanitizeUsername(preferred, email, subject string) string {
	if preferred != "" {
		return strings.ToLower(strings.ReplaceAll(preferred, " ", "_"))
	}
	if email != "" {
		parts := strings.SplitN(email, "@", 2)
		if parts[0] != "" {
			return strings.ToLower(parts[0])
		}
	}
	// Fall back to a short UUID fragment.
	return "ext_" + subject[:min(8, len(subject))]
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
