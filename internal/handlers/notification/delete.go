package notification

import (
	"net/http"

	"github.com/go-chi/chi/v5"

	"tackle/internal/middleware"
	"tackle/pkg/response"
)

// Delete handles DELETE /api/v1/notifications/{id} — removes a notification owned by the caller.
func (d *Deps) Delete(w http.ResponseWriter, r *http.Request) {
	correlationID := middleware.GetCorrelationID(r.Context())
	claims := middleware.ClaimsFromContext(r.Context())
	if claims == nil {
		response.Error(w, "UNAUTHORIZED", "authentication required", http.StatusUnauthorized, correlationID)
		return
	}

	id := chi.URLParam(r, "id")

	result, err := d.DB.ExecContext(r.Context(), `
		DELETE FROM notifications
		WHERE id = $1::uuid AND user_id = $2::uuid`,
		id, claims.Subject,
	)
	if err != nil {
		response.Error(w, "INTERNAL_ERROR", "failed to delete notification", http.StatusInternalServerError, correlationID)
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

	w.WriteHeader(http.StatusNoContent)
}
