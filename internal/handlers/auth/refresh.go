package auth

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"time"

	"tackle/internal/middleware"
	"tackle/internal/services/audit"
	authsvc "tackle/internal/services/auth"
	"tackle/pkg/response"
)

type refreshRequest struct {
	RefreshToken string `json:"refresh_token"`
}

// Refresh handles POST /api/v1/auth/refresh.
func (d *Deps) Refresh(w http.ResponseWriter, r *http.Request) {
	correlationID := middleware.GetCorrelationID(r.Context())

	// Accept refresh token from cookie (browser) or request body (API clients).
	rawToken := ""
	if c, err := r.Cookie("refresh_token"); err == nil {
		rawToken = c.Value
	}
	if rawToken == "" {
		var req refreshRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err == nil {
			rawToken = req.RefreshToken
		}
	}
	if rawToken == "" {
		response.Error(w, "UNAUTHORIZED", "refresh token required", http.StatusUnauthorized, correlationID)
		return
	}

	session, err := d.RefreshSvc.Consume(r.Context(), rawToken)
	if err == authsvc.ErrTokenReuse {
		slog.Warn("refresh.token_reuse", "correlation_id", correlationID)
		response.Error(w, "UNAUTHORIZED", "token reuse detected: all sessions revoked", http.StatusUnauthorized, correlationID)
		return
	}
	if err != nil {
		response.Error(w, "UNAUTHORIZED", "invalid or expired refresh token", http.StatusUnauthorized, correlationID)
		return
	}

	// Load fresh user data and permissions for the new token.
	ur, err := findUserByID(r.Context(), d.DB, session.UserID)
	if err != nil {
		response.Error(w, "INTERNAL_ERROR", "user not found", http.StatusInternalServerError, correlationID)
		return
	}

	accessToken, err := d.JWTSvc.Issue(&ur.user, ur.roleName, ur.permissions)
	if err != nil {
		response.Error(w, "INTERNAL_ERROR", "failed to issue token", http.StatusInternalServerError, correlationID)
		return
	}

	accessHash := authsvc.HashTokenPublic(accessToken)
	newRawRefresh, err := d.RefreshSvc.Issue(r.Context(), ur.user.ID, accessHash, r.RemoteAddr, r.UserAgent(), 7*24*time.Hour)
	if err != nil {
		response.Error(w, "INTERNAL_ERROR", "failed to issue session", http.StatusInternalServerError, correlationID)
		return
	}

	http.SetCookie(w, &http.Cookie{
		Name:     "refresh_token",
		Value:    newRawRefresh,
		Path:     "/api/v1/auth/refresh",
		HttpOnly: true,
		Secure:   true,
		SameSite: http.SameSiteStrictMode,
		MaxAge:   7 * 24 * 60 * 60,
	})

	slog.Info("refresh.success", "user_id", ur.user.ID, "correlation_id", correlationID)
	if d.AuditSvc != nil {
		_ = d.AuditSvc.Log(r.Context(), audit.LogEntry{
			Category:      audit.CategoryUserActivity,
			Severity:      audit.SeverityInfo,
			ActorType:     audit.ActorTypeUser,
			ActorID:       &ur.user.ID,
			ActorLabel:    ur.user.Username,
			Action:        "auth.session.refresh",
			CorrelationID: correlationID,
		})
	}

	response.Success(w, loginResponse{
		AccessToken:  accessToken,
		RefreshToken: newRawRefresh,
		TokenType:    "Bearer",
		ExpiresIn:    900,
		User: userSummary{
			ID:          ur.user.ID,
			Username:    ur.user.Username,
			DisplayName: ur.user.DisplayName,
			Roles:       []string{ur.roleName},
			Permissions: ur.permissions,
		},
	})
}
