package users

import (
	"database/sql"
	"net/http"

	"github.com/go-chi/chi/v5"

	"tackle/internal/middleware"
	"tackle/internal/services/audit"
	"tackle/pkg/response"
)

// DeleteUser handles DELETE /api/v1/users/{id} — soft deactivates a user.
func (d *Deps) DeleteUser(w http.ResponseWriter, r *http.Request) {
	correlationID := middleware.GetCorrelationID(r.Context())
	targetID := chi.URLParam(r, "id")
	callerClaims := middleware.ClaimsFromContext(r.Context())
	if callerClaims == nil {
		response.Error(w, "UNAUTHORIZED", "authentication required", http.StatusUnauthorized, correlationID)
		return
	}

	// Cannot delete self.
	if callerClaims.Subject == targetID {
		response.Error(w, "FORBIDDEN", "cannot delete own account", http.StatusForbidden, correlationID)
		return
	}

	// Check initial admin guard.
	var isInitialAdmin bool
	if err := d.DB.QueryRowContext(r.Context(), `SELECT is_initial_admin FROM users WHERE id = $1`, targetID).Scan(&isInitialAdmin); err == sql.ErrNoRows {
		response.Error(w, "NOT_FOUND", "user not found", http.StatusNotFound, correlationID)
		return
	} else if err != nil {
		response.Error(w, "INTERNAL_ERROR", "failed to fetch user", http.StatusInternalServerError, correlationID)
		return
	}
	if isInitialAdmin {
		response.Error(w, "CONFLICT", "cannot modify initial admin", http.StatusConflict, correlationID)
		return
	}

	if _, err := d.DB.ExecContext(r.Context(), `UPDATE users SET status = 'inactive', updated_at = now() WHERE id = $1`, targetID); err != nil {
		response.Error(w, "INTERNAL_ERROR", "failed to deactivate user", http.StatusInternalServerError, correlationID)
		return
	}

	if d.AuditSvc != nil {
		actorID := callerClaims.Subject
		resType := "user"
		_ = d.AuditSvc.Log(r.Context(), audit.LogEntry{
			Category:      audit.CategoryUserActivity,
			Severity:      audit.SeverityInfo,
			ActorType:     audit.ActorTypeUser,
			ActorID:       &actorID,
			ActorLabel:    callerClaims.Username,
			Action:        "user.deactivated",
			ResourceType:  &resType,
			ResourceID:    &targetID,
			CorrelationID: correlationID,
		})
	}

	w.WriteHeader(http.StatusNoContent)
}
