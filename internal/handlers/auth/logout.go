package auth

import (
	"log/slog"
	"net/http"
	"time"

	"tackle/internal/middleware"
	"tackle/internal/services/audit"
	"tackle/pkg/response"
)

// Logout handles POST /api/v1/auth/logout.
func (d *Deps) Logout(w http.ResponseWriter, r *http.Request) {
	correlationID := middleware.GetCorrelationID(r.Context())
	claims := middleware.ClaimsFromContext(r.Context())
	if claims == nil {
		response.Error(w, "UNAUTHORIZED", "authentication required", http.StatusUnauthorized, correlationID)
		return
	}

	// Blacklist the access token JTI for the remainder of its lifetime.
	if claims.ExpiresAt != nil {
		d.Blacklist.Revoke(claims.JTI, claims.ExpiresAt.Time)
	} else {
		d.Blacklist.Revoke(claims.JTI, time.Now().Add(15*time.Minute))
	}

	// Revoke the refresh token from cookie or body.
	rawRefresh := ""
	if c, err := r.Cookie("refresh_token"); err == nil {
		rawRefresh = c.Value
	}
	if rawRefresh != "" {
		if _, err := d.RefreshSvc.Consume(r.Context(), rawRefresh); err != nil {
			// Token may already be expired or not found — still proceed with logout.
			slog.Info("logout: refresh token consume error", "err", err, "correlation_id", correlationID)
		}
	}

	// Clear the cookie.
	http.SetCookie(w, &http.Cookie{
		Name:     "refresh_token",
		Value:    "",
		Path:     "/api/v1/auth/refresh",
		HttpOnly: true,
		Secure:   true,
		SameSite: http.SameSiteStrictMode,
		MaxAge:   -1,
	})

	slog.Info("logout.success", "user_id", claims.Subject, "correlation_id", correlationID)
	if d.AuditSvc != nil {
		userID := claims.Subject
		_ = d.AuditSvc.Log(r.Context(), audit.LogEntry{
			Category:      audit.CategoryUserActivity,
			Severity:      audit.SeverityInfo,
			ActorType:     audit.ActorTypeUser,
			ActorID:       &userID,
			ActorLabel:    claims.Username,
			Action:        "auth.logout",
			CorrelationID: correlationID,
		})
	}
	w.WriteHeader(http.StatusNoContent)
}
