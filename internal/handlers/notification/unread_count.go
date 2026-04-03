package notification

import (
	"net/http"

	"tackle/internal/middleware"
	"tackle/pkg/response"
)

// UnreadCount handles GET /api/v1/notifications/unread-count — returns the caller's unread count.
func (d *Deps) UnreadCount(w http.ResponseWriter, r *http.Request) {
	correlationID := middleware.GetCorrelationID(r.Context())
	claims := middleware.ClaimsFromContext(r.Context())
	if claims == nil {
		response.Error(w, "UNAUTHORIZED", "authentication required", http.StatusUnauthorized, correlationID)
		return
	}

	var count int
	err := d.DB.QueryRowContext(r.Context(), `
		SELECT COUNT(*) FROM notifications
		WHERE user_id = $1::uuid AND is_read = FALSE`,
		claims.Subject,
	).Scan(&count)
	if err != nil {
		response.Error(w, "INTERNAL_ERROR", "failed to count notifications", http.StatusInternalServerError, correlationID)
		return
	}

	response.Success(w, map[string]int{"count": count})
}
