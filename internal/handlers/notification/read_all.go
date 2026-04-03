package notification

import (
	"net/http"

	"tackle/internal/middleware"
	"tackle/pkg/response"
)

// ReadAll handles POST /api/v1/notifications/read-all — bulk marks all unread notifications as read.
func (d *Deps) ReadAll(w http.ResponseWriter, r *http.Request) {
	correlationID := middleware.GetCorrelationID(r.Context())
	claims := middleware.ClaimsFromContext(r.Context())
	if claims == nil {
		response.Error(w, "UNAUTHORIZED", "authentication required", http.StatusUnauthorized, correlationID)
		return
	}

	result, err := d.DB.ExecContext(r.Context(), `
		UPDATE notifications
		SET is_read = TRUE
		WHERE user_id = $1::uuid AND is_read = FALSE`,
		claims.Subject,
	)
	if err != nil {
		response.Error(w, "INTERNAL_ERROR", "failed to mark notifications as read", http.StatusInternalServerError, correlationID)
		return
	}

	n, _ := result.RowsAffected()
	response.Success(w, map[string]int64{"updated": n})
}
