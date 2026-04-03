package roles

import (
	"encoding/json"
	"net/http"
	"regexp"
	"strings"

	"tackle/internal/middleware"
	"tackle/internal/services/audit"
	"tackle/internal/services/rbac"
	"tackle/pkg/response"
)

var roleNameRe = regexp.MustCompile(`^[a-zA-Z0-9_-]{1,64}$`)

type createRoleRequest struct {
	Name        string   `json:"name"`
	Description string   `json:"description"`
	Permissions []string `json:"permissions"`
}

// Create handles POST /api/v1/roles — creates a new custom role.
func (d *Deps) Create(w http.ResponseWriter, r *http.Request) {
	correlationID := middleware.GetCorrelationID(r.Context())

	var req createRoleRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.Error(w, "BAD_REQUEST", "invalid request body", http.StatusBadRequest, correlationID)
		return
	}

	if !roleNameRe.MatchString(req.Name) {
		response.Error(w, "VALIDATION_ERROR", "name must be 1–64 characters and contain only letters, digits, underscores, or hyphens", http.StatusBadRequest, correlationID)
		return
	}

	// Built-in role name check (case-insensitive).
	if rbac.IsBuiltin(strings.ToLower(req.Name)) {
		response.Error(w, "VALIDATION_ERROR", "name conflicts with a built-in role", http.StatusBadRequest, correlationID)
		return
	}

	// Validate each permission against the registry.
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

	const insertRole = `
		INSERT INTO roles (name, description, is_builtin)
		VALUES ($1, $2, FALSE)
		RETURNING id, name, COALESCE(description, ''), is_builtin, created_at, updated_at`

	var role roleResponse
	err = tx.QueryRowContext(r.Context(), insertRole, req.Name, req.Description).Scan(
		&role.ID, &role.Name, &role.Description, &role.IsBuiltin,
		&role.CreatedAt, &role.UpdatedAt,
	)
	if err != nil {
		if isUniqueViolation(err) {
			response.Error(w, "CONFLICT", "a role with that name already exists", http.StatusConflict, correlationID)
			return
		}
		response.Error(w, "INTERNAL_ERROR", "failed to create role", http.StatusInternalServerError, correlationID)
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
			Action:       "rbac.role.created",
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

	response.Created(w, role)
}
