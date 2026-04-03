package auth

import (
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"

	"tackle/internal/middleware"
	"tackle/internal/services/audit"
	"tackle/pkg/response"
)

type sessionInfo struct {
	ID          string     `json:"id"`
	IPAddress   string     `json:"ip_address"`
	UserAgent   string     `json:"user_agent"`
	CreatedAt   time.Time  `json:"created_at"`
	LastUsedAt  *time.Time `json:"last_used_at"`
	ExpiresAt   time.Time  `json:"expires_at"`
}

// ListSessions handles GET /api/v1/users/me/sessions.
func (d *Deps) ListSessions(w http.ResponseWriter, r *http.Request) {
	correlationID := middleware.GetCorrelationID(r.Context())
	claims := middleware.ClaimsFromContext(r.Context())
	if claims == nil {
		response.Error(w, "UNAUTHORIZED", "authentication required", http.StatusUnauthorized, correlationID)
		return
	}

	const q = `
		SELECT id, COALESCE(ip_address::text,''), COALESCE(user_agent,''),
		       created_at, last_used_at, expires_at
		FROM sessions
		WHERE user_id = $1 AND revoked = FALSE AND expires_at > now()
		ORDER BY created_at DESC`

	rows, err := d.DB.QueryContext(r.Context(), q, claims.Subject)
	if err != nil {
		response.Error(w, "INTERNAL_ERROR", "failed to list sessions", http.StatusInternalServerError, correlationID)
		return
	}
	defer rows.Close()

	var sessions []sessionInfo
	for rows.Next() {
		var s sessionInfo
		var lastUsed *time.Time
		if err := rows.Scan(&s.ID, &s.IPAddress, &s.UserAgent, &s.CreatedAt, &lastUsed, &s.ExpiresAt); err != nil {
			response.Error(w, "INTERNAL_ERROR", "failed to scan session", http.StatusInternalServerError, correlationID)
			return
		}
		s.LastUsedAt = lastUsed
		sessions = append(sessions, s)
	}
	if err := rows.Err(); err != nil {
		response.Error(w, "INTERNAL_ERROR", "session query error", http.StatusInternalServerError, correlationID)
		return
	}

	if sessions == nil {
		sessions = []sessionInfo{}
	}
	response.Success(w, sessions)
}

// DeleteSession handles DELETE /api/v1/users/me/sessions/{id}.
func (d *Deps) DeleteSession(w http.ResponseWriter, r *http.Request) {
	correlationID := middleware.GetCorrelationID(r.Context())
	claims := middleware.ClaimsFromContext(r.Context())
	if claims == nil {
		response.Error(w, "UNAUTHORIZED", "authentication required", http.StatusUnauthorized, correlationID)
		return
	}

	sessionID := chi.URLParam(r, "id")

	result, err := d.DB.ExecContext(r.Context(),
		`UPDATE sessions SET revoked = TRUE WHERE id = $1 AND user_id = $2 AND revoked = FALSE`,
		sessionID, claims.Subject)
	if err != nil {
		response.Error(w, "INTERNAL_ERROR", "failed to revoke session", http.StatusInternalServerError, correlationID)
		return
	}
	rows, _ := result.RowsAffected()
	if rows == 0 {
		response.Error(w, "NOT_FOUND", "session not found", http.StatusNotFound, correlationID)
		return
	}

	if d.AuditSvc != nil {
		userID := claims.Subject
		resType := "session"
		_ = d.AuditSvc.Log(r.Context(), audit.LogEntry{
			Category:      audit.CategoryUserActivity,
			Severity:      audit.SeverityInfo,
			ActorType:     audit.ActorTypeUser,
			ActorID:       &userID,
			ActorLabel:    claims.Username,
			Action:        "user.session.terminated",
			ResourceType:  &resType,
			ResourceID:    &sessionID,
			CorrelationID: correlationID,
		})
	}

	w.WriteHeader(http.StatusNoContent)
}
