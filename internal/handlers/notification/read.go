package notification

import (
	"database/sql"
	"net/http"

	"github.com/go-chi/chi/v5"

	"tackle/internal/middleware"
	"tackle/pkg/response"
)

// Read handles PUT /api/v1/notifications/{id}/read — marks a single notification as read.
func (d *Deps) Read(w http.ResponseWriter, r *http.Request) {
	correlationID := middleware.GetCorrelationID(r.Context())
	claims := middleware.ClaimsFromContext(r.Context())
	if claims == nil {
		response.Error(w, "UNAUTHORIZED", "authentication required", http.StatusUnauthorized, correlationID)
		return
	}

	id := chi.URLParam(r, "id")

	result, err := d.DB.ExecContext(r.Context(), `
		UPDATE notifications
		SET is_read = TRUE
		WHERE id = $1::uuid AND user_id = $2::uuid`,
		id, claims.Subject,
	)
	if err != nil {
		response.Error(w, "INTERNAL_ERROR", "failed to update notification", http.StatusInternalServerError, correlationID)
		return
	}

	n, err := result.RowsAffected()
	if err != nil {
		response.Error(w, "INTERNAL_ERROR", "failed to check rows affected", http.StatusInternalServerError, correlationID)
		return
	}
	if n == 0 {
		response.Error(w, "NOT_FOUND", "notification not found", http.StatusNotFound, correlationID)
		return
	}

	// Fetch the updated record and return it.
	row := d.DB.QueryRowContext(r.Context(), `
		SELECT id, user_id, category, severity, title, body, resource_type, resource_id,
			action_url, is_read, expires_at, created_at
		FROM notifications
		WHERE id = $1::uuid`, id)

	notif, err := scanNotificationRow(row)
	if err != nil {
		if err == sql.ErrNoRows {
			response.Error(w, "NOT_FOUND", "notification not found", http.StatusNotFound, correlationID)
			return
		}
		response.Error(w, "INTERNAL_ERROR", "failed to fetch updated notification", http.StatusInternalServerError, correlationID)
		return
	}

	response.Success(w, notif)
}
