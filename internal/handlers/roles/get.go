package roles

import (
	"database/sql"
	"net/http"

	"github.com/go-chi/chi/v5"

	"tackle/internal/middleware"
	"tackle/pkg/response"
)

// Get handles GET /api/v1/roles/{id} — returns a role with its full permission list.
func (d *Deps) Get(w http.ResponseWriter, r *http.Request) {
	correlationID := middleware.GetCorrelationID(r.Context())
	id := chi.URLParam(r, "id")

	const roleQ = `
		SELECT id, name, COALESCE(description, ''), is_builtin, created_at, updated_at
		FROM roles
		WHERE id = $1`

	var role roleResponse
	err := d.DB.QueryRowContext(r.Context(), roleQ, id).Scan(
		&role.ID, &role.Name, &role.Description, &role.IsBuiltin,
		&role.CreatedAt, &role.UpdatedAt,
	)
	if err == sql.ErrNoRows {
		response.Error(w, "NOT_FOUND", "role not found", http.StatusNotFound, correlationID)
		return
	}
	if err != nil {
		response.Error(w, "INTERNAL_ERROR", "failed to get role", http.StatusInternalServerError, correlationID)
		return
	}

	perms, err := fetchRolePermissions(r.Context(), d.DB, role.ID)
	if err != nil {
		response.Error(w, "INTERNAL_ERROR", "failed to get permissions", http.StatusInternalServerError, correlationID)
		return
	}
	role.Permissions = perms
	role.PermissionCount = len(perms)

	response.Success(w, role)
}
