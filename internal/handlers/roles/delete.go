package roles

import (
	"database/sql"
	"net/http"

	"github.com/go-chi/chi/v5"

	"tackle/internal/middleware"
	"tackle/pkg/response"
)

type deleteConflictDetail struct {
	Code      string   `json:"code"`
	Message   string   `json:"message"`
	UserCount int      `json:"user_count"`
	UserIDs   []string `json:"user_ids"`
}

// Delete handles DELETE /api/v1/roles/{id} — deletes a custom role.
func (d *Deps) Delete(w http.ResponseWriter, r *http.Request) {
	correlationID := middleware.GetCorrelationID(r.Context())
	id := chi.URLParam(r, "id")

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
		response.Error(w, "CONFLICT", "built-in roles cannot be deleted", http.StatusConflict, correlationID)
		return
	}

	// Check for assigned users.
	const usersQ = `SELECT user_id FROM user_roles WHERE role_id = $1`
	rows, err := d.DB.QueryContext(r.Context(), usersQ, id)
	if err != nil {
		response.Error(w, "INTERNAL_ERROR", "failed to check assigned users", http.StatusInternalServerError, correlationID)
		return
	}
	defer rows.Close()

	var userIDs []string
	for rows.Next() {
		var uid string
		if err := rows.Scan(&uid); err != nil {
			response.Error(w, "INTERNAL_ERROR", "failed to scan user ID", http.StatusInternalServerError, correlationID)
			return
		}
		userIDs = append(userIDs, uid)
	}
	if err := rows.Err(); err != nil {
		response.Error(w, "INTERNAL_ERROR", "failed to iterate users", http.StatusInternalServerError, correlationID)
		return
	}

	if len(userIDs) > 0 {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusConflict)
		detail := map[string]interface{}{
			"code":       "CONFLICT",
			"message":    "role has assigned users; reassign them before deleting",
			"user_count": len(userIDs),
			"user_ids":   userIDs,
		}
		encodeJSON(w, map[string]interface{}{"error": detail})
		return
	}

	if _, err := d.DB.ExecContext(r.Context(), `DELETE FROM roles WHERE id = $1`, id); err != nil {
		response.Error(w, "INTERNAL_ERROR", "failed to delete role", http.StatusInternalServerError, correlationID)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}
