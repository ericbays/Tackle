// Package users provides HTTP handlers for user management endpoints.
package users

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"

	"tackle/internal/middleware"
	"tackle/internal/services/audit"
	authsvc "tackle/internal/services/auth"
	"tackle/pkg/response"
)

// Deps holds shared dependencies for user handlers.
type Deps struct {
	DB         *sql.DB
	AuditSvc   *audit.AuditService
	RefreshSvc *authsvc.RefreshTokenService
}

type assignRoleRequest struct {
	RoleID string `json:"role_id"`
}

type userRoleResponse struct {
	ID          string    `json:"id"`
	Username    string    `json:"username"`
	Email       string    `json:"email"`
	DisplayName string    `json:"display_name"`
	RoleID      string    `json:"role_id"`
	RoleName    string    `json:"role_name"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// AssignRole handles PUT /api/v1/users/{id}/roles — assigns a role to a user.
func (d *Deps) AssignRole(w http.ResponseWriter, r *http.Request) {
	correlationID := middleware.GetCorrelationID(r.Context())
	targetUserID := chi.URLParam(r, "id")

	callerClaims := middleware.ClaimsFromContext(r.Context())
	if callerClaims == nil {
		response.Error(w, "UNAUTHORIZED", "authentication required", http.StatusUnauthorized, correlationID)
		return
	}

	var req assignRoleRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.Error(w, "BAD_REQUEST", "invalid request body", http.StatusBadRequest, correlationID)
		return
	}
	if req.RoleID == "" {
		response.Error(w, "VALIDATION_ERROR", "role_id is required", http.StatusBadRequest, correlationID)
		return
	}

	// Fetch the target user.
	var targetUser struct {
		ID             string
		Username       string
		Email          string
		DisplayName    string
		IsInitialAdmin bool
		UpdatedAt      time.Time
	}
	const userQ = `SELECT id, username, email, display_name, is_initial_admin, updated_at FROM users WHERE id = $1`
	err := d.DB.QueryRowContext(r.Context(), userQ, targetUserID).Scan(
		&targetUser.ID, &targetUser.Username, &targetUser.Email,
		&targetUser.DisplayName, &targetUser.IsInitialAdmin, &targetUser.UpdatedAt,
	)
	if err == sql.ErrNoRows {
		response.Error(w, "NOT_FOUND", "user not found", http.StatusNotFound, correlationID)
		return
	}
	if err != nil {
		response.Error(w, "INTERNAL_ERROR", "failed to fetch user", http.StatusInternalServerError, correlationID)
		return
	}

	// Fetch the target role.
	var roleName string
	const roleQ = `SELECT name FROM roles WHERE id = $1`
	err = d.DB.QueryRowContext(r.Context(), roleQ, req.RoleID).Scan(&roleName)
	if err == sql.ErrNoRows {
		response.Error(w, "NOT_FOUND", "role not found", http.StatusNotFound, correlationID)
		return
	}
	if err != nil {
		response.Error(w, "INTERNAL_ERROR", "failed to fetch role", http.StatusInternalServerError, correlationID)
		return
	}

	// Initial admin's role is immutable.
	if targetUser.IsInitialAdmin && roleName != "admin" {
		response.Error(w, "FORBIDDEN", "cannot change the initial administrator's role", http.StatusForbidden, correlationID)
		return
	}

	// An administrator cannot remove their own admin role.
	if callerClaims.Subject == targetUserID && callerClaims.Role == "admin" && roleName != "admin" {
		response.Error(w, "FORBIDDEN", "administrators cannot remove their own administrator role", http.StatusForbidden, correlationID)
		return
	}

	// Upsert user_roles (single role per user).
	tx, err := d.DB.BeginTx(r.Context(), nil)
	if err != nil {
		response.Error(w, "INTERNAL_ERROR", "failed to start transaction", http.StatusInternalServerError, correlationID)
		return
	}
	defer tx.Rollback() //nolint:errcheck

	if _, err := tx.ExecContext(r.Context(), `DELETE FROM user_roles WHERE user_id = $1`, targetUserID); err != nil {
		response.Error(w, "INTERNAL_ERROR", "failed to remove existing role", http.StatusInternalServerError, correlationID)
		return
	}
	if _, err := tx.ExecContext(r.Context(),
		`INSERT INTO user_roles (user_id, role_id) VALUES ($1, $2)`,
		targetUserID, req.RoleID,
	); err != nil {
		response.Error(w, "INTERNAL_ERROR", "failed to assign role", http.StatusInternalServerError, correlationID)
		return
	}

	if err := tx.Commit(); err != nil {
		response.Error(w, "INTERNAL_ERROR", "failed to commit", http.StatusInternalServerError, correlationID)
		return
	}

	if d.AuditSvc != nil {
		actorID := callerClaims.Subject
		resType := "user"
		_ = d.AuditSvc.Log(r.Context(), audit.LogEntry{
			Category:      audit.CategoryUserActivity,
			Severity:      audit.SeverityInfo,
			ActorType:     audit.ActorTypeUser,
			ActorID:       &actorID,
			ActorLabel:    callerClaims.Username,
			Action:        "rbac.role.assigned",
			ResourceType:  &resType,
			ResourceID:    &targetUserID,
			CorrelationID: correlationID,
			Details:       map[string]any{"role_id": req.RoleID, "role_name": roleName, "target_username": targetUser.Username},
		})
	}

	result := userRoleResponse{
		ID:          targetUser.ID,
		Username:    targetUser.Username,
		Email:       targetUser.Email,
		DisplayName: targetUser.DisplayName,
		RoleID:      req.RoleID,
		RoleName:    roleName,
		UpdatedAt:   targetUser.UpdatedAt,
	}
	response.Success(w, result)
}
