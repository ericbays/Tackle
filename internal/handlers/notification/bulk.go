package notification

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"tackle/internal/middleware"
	"tackle/pkg/response"
)

// DeleteRead handles POST /api/v1/notifications/delete-read — deletes all read notifications for the caller.
func (d *Deps) DeleteRead(w http.ResponseWriter, r *http.Request) {
	correlationID := middleware.GetCorrelationID(r.Context())
	claims := middleware.ClaimsFromContext(r.Context())
	if claims == nil {
		response.Error(w, "UNAUTHORIZED", "authentication required", http.StatusUnauthorized, correlationID)
		return
	}

	result, err := d.DB.ExecContext(r.Context(), `
		DELETE FROM notifications
		WHERE user_id = $1::uuid AND is_read = TRUE`,
		claims.Subject,
	)
	if err != nil {
		response.Error(w, "INTERNAL_ERROR", "failed to delete read notifications", http.StatusInternalServerError, correlationID)
		return
	}

	n, _ := result.RowsAffected()
	response.Success(w, map[string]int64{"deleted": n})
}

// deleteSelectedRequest is the request body for DeleteSelected.
type deleteSelectedRequest struct {
	IDs []string `json:"ids"`
}

// DeleteSelected handles POST /api/v1/notifications/delete-selected — deletes notifications by ID for the caller.
func (d *Deps) DeleteSelected(w http.ResponseWriter, r *http.Request) {
	correlationID := middleware.GetCorrelationID(r.Context())
	claims := middleware.ClaimsFromContext(r.Context())
	if claims == nil {
		response.Error(w, "UNAUTHORIZED", "authentication required", http.StatusUnauthorized, correlationID)
		return
	}

	var req deleteSelectedRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.Error(w, "BAD_REQUEST", "invalid request body", http.StatusBadRequest, correlationID)
		return
	}
	if len(req.IDs) == 0 {
		response.Success(w, map[string]int64{"deleted": 0})
		return
	}
	if len(req.IDs) > 100 {
		response.Error(w, "BAD_REQUEST", "maximum 100 IDs per request", http.StatusBadRequest, correlationID)
		return
	}

	// Build parameterized IN clause: $2, $3, ...
	placeholders := make([]string, len(req.IDs))
	args := make([]any, 0, len(req.IDs)+1)
	args = append(args, claims.Subject)
	for i, id := range req.IDs {
		placeholders[i] = fmt.Sprintf("$%d::uuid", i+2)
		args = append(args, id)
	}

	query := fmt.Sprintf(`DELETE FROM notifications WHERE user_id = $1::uuid AND id IN (%s)`,
		strings.Join(placeholders, ", "))

	result, err := d.DB.ExecContext(r.Context(), query, args...)
	if err != nil {
		response.Error(w, "INTERNAL_ERROR", "failed to delete selected notifications", http.StatusInternalServerError, correlationID)
		return
	}

	n, _ := result.RowsAffected()
	response.Success(w, map[string]int64{"deleted": n})
}
