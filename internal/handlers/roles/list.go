package roles

import (
	"net/http"
	"time"

	"tackle/internal/middleware"
	"tackle/pkg/response"
)

// roleResponse is the JSON representation of a role returned by the API.
type roleResponse struct {
	ID              string    `json:"id"`
	Name            string    `json:"name"`
	Description     string    `json:"description"`
	IsBuiltin       bool      `json:"is_builtin"`
	Permissions     []string  `json:"permissions"`
	PermissionCount int       `json:"permission_count"`
	CreatedAt       time.Time `json:"created_at"`
	UpdatedAt       time.Time `json:"updated_at"`
}

// List handles GET /api/v1/roles — returns all roles with permission counts.
func (d *Deps) List(w http.ResponseWriter, r *http.Request) {
	correlationID := middleware.GetCorrelationID(r.Context())

	const q = `
		SELECT r.id, r.name, COALESCE(r.description, ''), r.is_builtin,
		       r.created_at, r.updated_at,
		       COUNT(rp.permission_id) AS permission_count
		FROM roles r
		LEFT JOIN role_permissions rp ON rp.role_id = r.id
		GROUP BY r.id, r.name, r.description, r.is_builtin, r.created_at, r.updated_at
		ORDER BY r.is_builtin DESC, r.name`

	rows, err := d.DB.QueryContext(r.Context(), q)
	if err != nil {
		response.Error(w, "INTERNAL_ERROR", "failed to list roles", http.StatusInternalServerError, correlationID)
		return
	}
	defer rows.Close()

	var result []roleResponse
	for rows.Next() {
		var role roleResponse
		var permCount int
		if err := rows.Scan(
			&role.ID, &role.Name, &role.Description, &role.IsBuiltin,
			&role.CreatedAt, &role.UpdatedAt, &permCount,
		); err != nil {
			response.Error(w, "INTERNAL_ERROR", "failed to scan role", http.StatusInternalServerError, correlationID)
			return
		}
		role.PermissionCount = permCount
		role.Permissions = []string{}
		result = append(result, role)
	}
	if err := rows.Err(); err != nil {
		response.Error(w, "INTERNAL_ERROR", "failed to iterate roles", http.StatusInternalServerError, correlationID)
		return
	}

	if result == nil {
		result = []roleResponse{}
	}
	response.Success(w, result)
}
