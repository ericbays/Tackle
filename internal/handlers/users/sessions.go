package users

import (
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"

	"tackle/internal/middleware"
	"tackle/internal/services/audit"
	"tackle/pkg/response"
)

type adminSessionInfo struct {
	ID         string     `json:"id"`
	IPAddress  string     `json:"ip_address"`
	UserAgent  string     `json:"user_agent"`
	CreatedAt  time.Time  `json:"created_at"`
	LastUsedAt *time.Time `json:"last_used_at"`
	ExpiresAt  time.Time  `json:"expires_at"`
}

// ListUserSessions handles GET /api/v1/users/{id}/sessions.
// Admins can view any user's sessions; users can view their own.
func (d *Deps) ListUserSessions(w http.ResponseWriter, r *http.Request) {
	correlationID := middleware.GetCorrelationID(r.Context())
	targetID := chi.URLParam(r, "id")
	callerClaims := middleware.ClaimsFromContext(r.Context())
	if callerClaims == nil {
		response.Error(w, "UNAUTHORIZED", "authentication required", http.StatusUnauthorized, correlationID)
		return
	}

	// Self-access is always allowed; otherwise requires users:read (enforced at router).
	rows, err := d.DB.QueryContext(r.Context(), `
		SELECT id, COALESCE(ip_address::text,''), COALESCE(user_agent,''),
		       created_at, last_used_at, expires_at
		FROM sessions
		WHERE user_id = $1 AND revoked = FALSE AND expires_at > now()
		ORDER BY created_at DESC`, targetID)
	if err != nil {
		response.Error(w, "INTERNAL_ERROR", "failed to list sessions", http.StatusInternalServerError, correlationID)
		return
	}
	defer rows.Close()

	var sessions []adminSessionInfo
	for rows.Next() {
		var s adminSessionInfo
		if err := rows.Scan(&s.ID, &s.IPAddress, &s.UserAgent, &s.CreatedAt, &s.LastUsedAt, &s.ExpiresAt); err != nil {
			response.Error(w, "INTERNAL_ERROR", "failed to scan session", http.StatusInternalServerError, correlationID)
			return
		}
		sessions = append(sessions, s)
	}
	if err := rows.Err(); err != nil {
		response.Error(w, "INTERNAL_ERROR", "session query error", http.StatusInternalServerError, correlationID)
		return
	}
	if sessions == nil {
		sessions = []adminSessionInfo{}
	}
	response.Success(w, sessions)
}

// TerminateUserSession handles DELETE /api/v1/users/{id}/sessions/{sid}.
func (d *Deps) TerminateUserSession(w http.ResponseWriter, r *http.Request) {
	correlationID := middleware.GetCorrelationID(r.Context())
	targetID := chi.URLParam(r, "id")
	sessionID := chi.URLParam(r, "sid")
	callerClaims := middleware.ClaimsFromContext(r.Context())
	if callerClaims == nil {
		response.Error(w, "UNAUTHORIZED", "authentication required", http.StatusUnauthorized, correlationID)
		return
	}

	result, err := d.DB.ExecContext(r.Context(),
		`UPDATE sessions SET revoked = TRUE WHERE id = $1 AND user_id = $2 AND revoked = FALSE`,
		sessionID, targetID)
	if err != nil {
		response.Error(w, "INTERNAL_ERROR", "failed to revoke session", http.StatusInternalServerError, correlationID)
		return
	}
	n, _ := result.RowsAffected()
	if n == 0 {
		response.Error(w, "NOT_FOUND", "session not found", http.StatusNotFound, correlationID)
		return
	}

	if d.AuditSvc != nil {
		actorID := callerClaims.Subject
		resType := "session"
		_ = d.AuditSvc.Log(r.Context(), audit.LogEntry{
			Category:      audit.CategoryUserActivity,
			Severity:      audit.SeverityInfo,
			ActorType:     audit.ActorTypeUser,
			ActorID:       &actorID,
			ActorLabel:    callerClaims.Username,
			Action:        "user.session.terminated",
			ResourceType:  &resType,
			ResourceID:    &sessionID,
			CorrelationID: correlationID,
			Details:       map[string]any{"target_user_id": targetID},
		})
	}

	w.WriteHeader(http.StatusNoContent)
}
