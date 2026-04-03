package audit

import (
	"net/http"

	"github.com/go-chi/chi/v5"

	"tackle/internal/middleware"
	"tackle/pkg/response"
)

type verifyResponse struct {
	Valid  bool    `json:"valid"`
	Reason *string `json:"reason,omitempty"`
}

// Verify handles POST /api/v1/logs/audit/{id}/verify.
// Fetches the entry, recomputes its HMAC checksum, and returns whether it is valid.
func (d *Deps) Verify(w http.ResponseWriter, r *http.Request) {
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

	svcEntry := toServiceEntry(entry)
	valid := d.HMACSvc.Verify(svcEntry)

	result := verifyResponse{Valid: valid}
	if !valid {
		reason := "checksum mismatch: entry may have been tampered with"
		result.Reason = &reason
	}

	response.Success(w, result)
}
