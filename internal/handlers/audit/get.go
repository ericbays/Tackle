package audit

import (
	"database/sql"
	"net/http"

	"github.com/go-chi/chi/v5"

	"tackle/internal/middleware"
	"tackle/pkg/response"
)

// Get handles GET /api/v1/logs/audit/{id} — fetches a single audit log entry.
// Operators may only retrieve entries where actor_id = their own user ID.
func (d *Deps) Get(w http.ResponseWriter, r *http.Request) {
	correlationID := middleware.GetCorrelationID(r.Context())
	claims := middleware.ClaimsFromContext(r.Context())
	if claims == nil {
		response.Error(w, "UNAUTHORIZED", "authentication required", http.StatusUnauthorized, correlationID)
		return
	}

	id := chi.URLParam(r, "id")

	const q = `SELECT id, timestamp, category, severity, actor_type, actor_id, actor_label, action,
		resource_type, resource_id, details, correlation_id, source_ip, session_id, campaign_id, checksum,
		previous_checksum
		FROM audit_logs WHERE id = $1`

	rows, err := d.DB.QueryContext(r.Context(), q, id)
	if err != nil {
		response.Error(w, "INTERNAL_ERROR", "failed to query audit log", http.StatusInternalServerError, correlationID)
		return
	}
	defer rows.Close()

	if !rows.Next() {
		if err := rows.Err(); err != nil {
			response.Error(w, "INTERNAL_ERROR", "query error", http.StatusInternalServerError, correlationID)
			return
		}
		response.Error(w, "NOT_FOUND", "audit log entry not found", http.StatusNotFound, correlationID)
		return
	}

	entry, err := scanRow(rows)
	if err != nil {
		response.Error(w, "INTERNAL_ERROR", "failed to read audit log entry", http.StatusInternalServerError, correlationID)
		return
	}

	// Operators can only view their own entries.
	if claims.Role == "operator" {
		if entry.ActorID == nil || *entry.ActorID != claims.Subject {
			response.Error(w, "FORBIDDEN", "access denied", http.StatusForbidden, correlationID)
			return
		}
	}

	if err := rows.Err(); err != nil && err != sql.ErrNoRows {
		response.Error(w, "INTERNAL_ERROR", "query error", http.StatusInternalServerError, correlationID)
		return
	}

	response.Success(w, entry)
}
