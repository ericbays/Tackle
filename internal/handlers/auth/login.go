package auth

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"

	"tackle/internal/middleware"
	"tackle/internal/models"
	"tackle/internal/services/audit"
	authsvc "tackle/internal/services/auth"
	"tackle/pkg/response"
)

type loginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

type userSummary struct {
	ID          string   `json:"id"`
	Username    string   `json:"username"`
	DisplayName string   `json:"display_name"`
	Roles       []string `json:"roles"`
	Permissions []string `json:"permissions"`
}

type loginResponse struct {
	AccessToken  string      `json:"access_token"`
	RefreshToken string      `json:"refresh_token"`
	TokenType    string      `json:"token_type"`
	ExpiresIn    int         `json:"expires_in"`
	User         userSummary `json:"user"`
}

// Login handles POST /api/v1/auth/login.
func (d *Deps) Login(w http.ResponseWriter, r *http.Request) {
	correlationID := middleware.GetCorrelationID(r.Context())

	var req loginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.Error(w, "BAD_REQUEST", "invalid request body", http.StatusBadRequest, correlationID)
		return
	}
	if req.Username == "" || req.Password == "" {
		response.Error(w, "UNAUTHORIZED", "invalid credentials", http.StatusUnauthorized, correlationID)
		return
	}

	// Check IP rate limit.
	ipKey := "ip:" + r.RemoteAddr
	if locked, _ := d.RateLimiter.IsLocked(ipKey); locked {
		response.Error(w, "TOO_MANY_REQUESTS", "too many login attempts, try again later", http.StatusTooManyRequests, correlationID)
		return
	}

	// Check per-account rate limit.
	accountKey := "user:" + req.Username
	if locked, _ := d.RateLimiter.IsLocked(accountKey); locked {
		response.Error(w, "TOO_MANY_REQUESTS", "too many login attempts, try again later", http.StatusTooManyRequests, correlationID)
		return
	}

	// Try routing through LoginRouter (supports LDAP + local), fall back to local-only.
	var (
		userID      string
		username    string
		displayName string
		roleName    string
		permissions []string
		provider    string
	)

	if d.LoginRouter != nil {
		// Define the local auth callback for the LoginRouter.
		localAuth := func(ctx context.Context, uname, pass string) (string, string, []string, error) {
			return d.authenticateLocal(ctx, uname, pass)
		}

		provisioned, prov, err := d.LoginRouter.RouteLogin(r.Context(), req.Username, req.Password, localAuth)
		if err != nil {
			d.RateLimiter.RecordFailure(ipKey)
			d.RateLimiter.RecordFailure(accountKey)
			slog.Info("login.failed", "reason", "authentication_failed", "username", req.Username, "correlation_id", correlationID)
			d.auditLoginFailure(r, correlationID, req.Username, nil, "authentication_failed")
			response.Error(w, "UNAUTHORIZED", "invalid credentials", http.StatusUnauthorized, correlationID)
			return
		}

		userID = provisioned.UserID
		username = provisioned.Username
		displayName = provisioned.DisplayName
		roleName = provisioned.RoleName
		permissions = provisioned.Permissions
		provider = prov
	} else {
		// No LoginRouter — local-only authentication.
		uid, rname, perms, err := d.authenticateLocal(r.Context(), req.Username, req.Password)
		if err != nil {
			d.RateLimiter.RecordFailure(ipKey)
			d.RateLimiter.RecordFailure(accountKey)
			slog.Info("login.failed", "reason", err.Error(), "username", req.Username, "correlation_id", correlationID)
			d.auditLoginFailure(r, correlationID, req.Username, nil, err.Error())
			response.Error(w, "UNAUTHORIZED", "invalid credentials", http.StatusUnauthorized, correlationID)
			return
		}

		// Re-fetch user details for the response.
		ur, _ := findUserByUsernameOrEmail(r.Context(), d.DB, req.Username)
		if ur != nil {
			displayName = ur.user.DisplayName
			username = ur.user.Username
		} else {
			username = req.Username
		}
		userID = uid
		roleName = rname
		permissions = perms
		provider = "local"
	}

	// Success — reset rate limit counters.
	d.RateLimiter.Reset(ipKey)
	d.RateLimiter.Reset(accountKey)

	// Load session config for token lifetimes.
	sessCfg := authsvc.DefaultSessionConfig()
	if d.SessionConfigLoader != nil {
		sessCfg = d.SessionConfigLoader.Load(r.Context())
	}

	// Enforce max concurrent sessions: revoke oldest if over limit.
	if sessCfg.MaxConcurrentSessions > 0 {
		count, err := d.RefreshSvc.CountActiveSessions(r.Context(), userID)
		if err == nil && count >= sessCfg.MaxConcurrentSessions {
			_ = d.RefreshSvc.RevokeOldest(r.Context(), userID)
		}
	}

	// Issue JWT with configurable lifetime.
	accessToken, err := d.JWTSvc.IssueWithExpiry(userID, username, "", roleName, permissions, sessCfg.AccessTokenLifetime)
	if err != nil {
		response.Error(w, "INTERNAL_ERROR", "failed to issue token", http.StatusInternalServerError, correlationID)
		return
	}

	accessHash := authsvc.HashTokenPublic(accessToken)
	rawRefresh, err := d.RefreshSvc.Issue(r.Context(), userID, accessHash, r.RemoteAddr, r.UserAgent(), sessCfg.RefreshTokenLifetime)
	if err != nil {
		response.Error(w, "INTERNAL_ERROR", "failed to issue session", http.StatusInternalServerError, correlationID)
		return
	}

	refreshMaxAge := int(sessCfg.RefreshTokenLifetime.Seconds())
	// Set refresh token as HTTP-only cookie for browser clients.
	http.SetCookie(w, &http.Cookie{
		Name:     "refresh_token",
		Value:    rawRefresh,
		Path:     "/api/v1/auth/refresh",
		HttpOnly: true,
		Secure:   true,
		SameSite: http.SameSiteStrictMode,
		MaxAge:   refreshMaxAge,
	})

	slog.Info("login.success", "user_id", userID, "provider", provider, "correlation_id", correlationID)
	if d.AuditSvc != nil {
		ip := r.RemoteAddr
		_ = d.AuditSvc.Log(r.Context(), audit.LogEntry{
			Category:      audit.CategoryUserActivity,
			Severity:      audit.SeverityInfo,
			ActorType:     audit.ActorTypeUser,
			ActorID:       &userID,
			ActorLabel:    username,
			Action:        "auth.login.success",
			CorrelationID: correlationID,
			SourceIP:      &ip,
			Details:       map[string]any{"provider": provider},
		})
	}

	response.Success(w, loginResponse{
		AccessToken:  accessToken,
		RefreshToken: rawRefresh,
		TokenType:    "Bearer",
		ExpiresIn:    int(sessCfg.AccessTokenLifetime.Seconds()),
		User: userSummary{
			ID:          userID,
			Username:    username,
			DisplayName: displayName,
			Roles:       []string{roleName},
			Permissions: permissions,
		},
	})
}

// authenticateLocal performs local bcrypt authentication.
// Returns (userID, roleName, permissions, error).
func (d *Deps) authenticateLocal(ctx context.Context, username, password string) (string, string, []string, error) {
	ur, err := findUserByUsernameOrEmail(ctx, d.DB, username)
	if err != nil {
		return "", "", nil, err
	}

	// Check account status.
	if ur.user.Status == "locked" {
		return "", "", nil, &models.ErrAccountLocked{}
	}

	// Verify password.
	if ur.user.PasswordHash == nil {
		return "", "", nil, &models.ErrInvalidCredentials{}
	}
	if err := authsvc.ComparePassword(*ur.user.PasswordHash, password); err != nil {
		return "", "", nil, &models.ErrInvalidCredentials{}
	}

	return ur.user.ID, ur.roleName, ur.permissions, nil
}

// auditLoginFailure logs a login failure to the audit service.
func (d *Deps) auditLoginFailure(r *http.Request, correlationID, username string, userID *string, reason string) {
	if d.AuditSvc == nil {
		return
	}
	ip := r.RemoteAddr
	_ = d.AuditSvc.Log(r.Context(), audit.LogEntry{
		Category:      audit.CategoryUserActivity,
		Severity:      audit.SeverityWarning,
		ActorType:     audit.ActorTypeUser,
		ActorID:       userID,
		ActorLabel:    username,
		Action:        "auth.login.failure",
		CorrelationID: correlationID,
		SourceIP:      &ip,
		Details:       map[string]any{"reason": reason, "username": username},
	})
}
