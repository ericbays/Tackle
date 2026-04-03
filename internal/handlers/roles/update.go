package roles

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"

	"tackle/internal/middleware"
	"tackle/internal/services/audit"
	"tackle/internal/services/rbac"
	"tackle/pkg/response"
)

type updateRoleRequest struct {
	Name        string   `json:"name"`
	Description string   `json:"description"`
	Permissions []string `json:"permissions"`
}

// Update handles PUT /api/v1/roles/{id} — updates a custom role.
func (d *Deps) Update(w http.ResponseWriter, r *http.Request) {
	correlationID := middleware.GetCorrelationID(r.Context())
	id := chi.URLParam(r, "id")

	// Fetch the role first to check is_builtin.
	var isBuiltin bool
	err := d.DB.QueryRowContext(r.Context(), `SELECT is_builtin FROM roles WHERE id = $1`, id).Scan(&isBuiltin)
	if err == sql.ErrNoRows {
		response.Error(w, "NOT_FOUND", "role not found", http.StatusNotFound, correlationID)
		return
	}
	if err != nil {
		response.Error(w, "INTERNAL_ERROR", "failed to fetch role", http.StatusInternalServerError, correlationID)
		return
	}
	if isBuiltin {
		response.Error(w, "CONFLICT", "built-in roles cannot be modified", http.StatusConflict, correlationID)
		return
	}

	var req updateRoleRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.Error(w, "BAD_REQUEST", "invalid request body", http.StatusBadRequest, correlationID)
		return
	}

	if !roleNameRe.MatchString(req.Name) {
		response.Error(w, "VALIDATION_ERROR", "name must be 1–64 characters and contain only letters, digits, underscores, or hyphens", http.StatusBadRequest, correlationID)
		return
	}

	if rbac.IsBuiltin(strings.ToLower(req.Name)) {
		response.Error(w, "VALIDATION_ERROR", "name conflicts with a built-in role", http.StatusBadRequest, correlationID)
		return
	}

	for _, p := range req.Permissions {
		if !rbac.Valid(rbac.Permission(p)) {
			response.Error(w, "VALIDATION_ERROR", "unknown permission: "+p, http.StatusBadRequest, correlationID)
			return
		}
	}

	tx, err := d.DB.BeginTx(r.Context(), nil)
	if err != nil {
		response.Error(w, "INTERNAL_ERROR", "failed to start transaction", http.StatusInternalServerError, correlationID)
		return
	}
	defer tx.Rollback() //nolint:errcheck

	const updateQ = `
		UPDATE roles SET name = $1, description = $2, updated_at = now()
		WHERE id = $3
		RETURNING id, name, COALESCE(description, ''), is_builtin, created_at, updated_at`

	var role roleResponse
	err = tx.QueryRowContext(r.Context(), updateQ, req.Name, req.Description, id).Scan(
		&role.ID, &role.Name, &role.Description, &role.IsBuiltin,
		&role.CreatedAt, &role.UpdatedAt,
	)
	if err != nil {
		if isUniqueViolation(err) {
			response.Error(w, "CONFLICT", "a role with that name already exists", http.StatusConflict, correlationID)
			return
		}
		response.Error(w, "INTERNAL_ERROR", "failed to update role", http.StatusInternalServerError, correlationID)
		return
	}

	// Replace permission set.
	if _, err := tx.ExecContext(r.Context(), `DELETE FROM role_permissions WHERE role_id = $1`, id); err != nil {
		response.Error(w, "INTERNAL_ERROR", "failed to clear permissions", http.StatusInternalServerError, correlationID)
		return
	}

	if err := insertRolePermissions(r.Context(), tx, role.ID, req.Permissions); err != nil {
		response.Error(w, "INTERNAL_ERROR", "failed to assign permissions", http.StatusInternalServerError, correlationID)
		return
	}

	if err := tx.Commit(); err != nil {
		response.Error(w, "INTERNAL_ERROR", "failed to commit", http.StatusInternalServerError, correlationID)
		return
	}

	role.Permissions = req.Permissions
	if role.Permissions == nil {
		role.Permissions = []string{}
	}
	role.PermissionCount = len(role.Permissions)

	if d.AuditSvc != nil {
		claims := middleware.ClaimsFromContext(r.Context())
		entry := audit.LogEntry{
			Category:     audit.CategoryUserActivity,
			Severity:     audit.SeverityInfo,
			ActorType:    audit.ActorTypeUser,
			Action:       "rbac.permission.changed",
			ResourceType: strPtr("role"),
			ResourceID:   &role.ID,
			Details:      map[string]any{"role_name": role.Name, "permissions": role.Permissions},
		}
		if claims != nil {
			entry.ActorID = &claims.Subject
			entry.ActorLabel = claims.Username
			entry.CorrelationID = correlationID
		}
		_ = d.AuditSvc.Log(r.Context(), entry)
	}

	response.Success(w, role)
}
