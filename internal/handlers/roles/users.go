package roles

import (
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"

	"tackle/internal/middleware"
	"tackle/pkg/response"
)

// userSummary is the abbreviated user representation returned for role membership lists.
type userSummary struct {
	ID          string    `json:"id"`
	Username    string    `json:"username"`
	Email       string    `json:"email"`
	DisplayName string    `json:"display_name"`
	CreatedAt   time.Time `json:"created_at"`
}

// Users handles GET /api/v1/roles/{id}/users — lists users assigned to a role.
func (d *Deps) Users(w http.ResponseWriter, r *http.Request) {
	correlationID := middleware.GetCorrelationID(r.Context())
	id := chi.URLParam(r, "id")

	// Verify the role exists.
	var exists bool
	err := d.DB.QueryRowContext(r.Context(), `SELECT EXISTS(SELECT 1 FROM roles WHERE id = $1)`, id).Scan(&exists)
	if err != nil {
		response.Error(w, "INTERNAL_ERROR", "failed to check role", http.StatusInternalServerError, correlationID)
		return
	}
	if !exists {
		response.Error(w, "NOT_FOUND", "role not found", http.StatusNotFound, correlationID)
		return
	}

	const q = `
		SELECT u.id, u.username, u.email, u.display_name, u.created_at
		FROM users u
		JOIN user_roles ur ON ur.user_id = u.id
		WHERE ur.role_id = $1
		ORDER BY u.username`

	rows, err := d.DB.QueryContext(r.Context(), q, id)
	if err != nil {
		response.Error(w, "INTERNAL_ERROR", "failed to list users", http.StatusInternalServerError, correlationID)
		return
	}
	defer rows.Close()

	var users []userSummary
	for rows.Next() {
		var u userSummary
		if err := rows.Scan(&u.ID, &u.Username, &u.Email, &u.DisplayName, &u.CreatedAt); err != nil {
			response.Error(w, "INTERNAL_ERROR", "failed to scan user", http.StatusInternalServerError, correlationID)
			return
		}
		users = append(users, u)
	}
	if err := rows.Err(); err != nil {
		response.Error(w, "INTERNAL_ERROR", "failed to iterate users", http.StatusInternalServerError, correlationID)
		return
	}

	if users == nil {
		users = []userSummary{}
	}
	response.Success(w, users)
}
