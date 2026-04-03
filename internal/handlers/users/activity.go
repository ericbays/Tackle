package users

import (
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"

	"tackle/internal/middleware"
	"tackle/pkg/response"
)

type activityEntry struct {
	ID            string     `json:"id"`
	Category      string     `json:"category"`
	Severity      string     `json:"severity"`
	Action        string     `json:"action"`
	ResourceType  *string    `json:"resource_type,omitempty"`
	ResourceID    *string    `json:"resource_id,omitempty"`
	CorrelationID *string    `json:"correlation_id,omitempty"`
	SourceIP      *string    `json:"source_ip,omitempty"`
	CreatedAt     time.Time  `json:"created_at"`
}

type activityResponse struct {
	Data  []activityEntry `json:"data"`
	Total int             `json:"total"`
}

// GetActivity handles GET /api/v1/users/{id}/activity.
func (d *Deps) GetActivity(w http.ResponseWriter, r *http.Request) {
	correlationID := middleware.GetCorrelationID(r.Context())
	targetID := chi.URLParam(r, "id")

	limit := 50
	if l, err := strconv.Atoi(r.URL.Query().Get("limit")); err == nil && l > 0 && l <= 200 {
		limit = l
	}
	offset := 0
	if o, err := strconv.Atoi(r.URL.Query().Get("offset")); err == nil && o >= 0 {
		offset = o
	}

	var total int
	if err := d.DB.QueryRowContext(r.Context(), `SELECT COUNT(*) FROM audit_logs WHERE actor_id = $1`, targetID).Scan(&total); err != nil {
		response.Error(w, "INTERNAL_ERROR", "failed to count activity", http.StatusInternalServerError, correlationID)
		return
	}

	rows, err := d.DB.QueryContext(r.Context(), `
		SELECT id, category, severity, action,
		       resource_type, resource_id, correlation_id,
		       source_ip::text,
		       created_at
		FROM audit_logs
		WHERE actor_id = $1
		ORDER BY created_at DESC
		LIMIT $2 OFFSET $3`,
		targetID, limit, offset)
	if err != nil {
		response.Error(w, "INTERNAL_ERROR", "failed to fetch activity", http.StatusInternalServerError, correlationID)
		return
	}
	defer rows.Close()

	var entries []activityEntry
	for rows.Next() {
		var e activityEntry
		if err := rows.Scan(
			&e.ID, &e.Category, &e.Severity, &e.Action,
			&e.ResourceType, &e.ResourceID, &e.CorrelationID,
			&e.SourceIP, &e.CreatedAt,
		); err != nil {
			response.Error(w, "INTERNAL_ERROR", "failed to scan activity", http.StatusInternalServerError, correlationID)
			return
		}
		entries = append(entries, e)
	}
	if err := rows.Err(); err != nil {
		response.Error(w, "INTERNAL_ERROR", "activity query error", http.StatusInternalServerError, correlationID)
		return
	}
	if entries == nil {
		entries = []activityEntry{}
	}

	response.Success(w, activityResponse{Data: entries, Total: total})
}
