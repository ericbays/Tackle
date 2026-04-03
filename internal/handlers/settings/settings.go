// Package settings provides HTTP handlers for system settings endpoints.
package settings

import (
	"database/sql"
	"encoding/json"
	"net/http"

	"tackle/internal/middleware"
	"tackle/internal/services/audit"
	"tackle/pkg/response"
)

// Deps holds shared dependencies for settings handlers.
type Deps struct {
	DB       *sql.DB
	AuditSvc *audit.AuditService
}

type settingsResponse struct {
	SessionTimeoutMinutes        int    `json:"session_timeout_minutes"`
	MaxLoginAttempts             int    `json:"max_login_attempts"`
	PasswordMinLength            int    `json:"password_min_length"`
	PasswordRequireUppercase     bool   `json:"password_require_uppercase"`
	PasswordRequireLowercase     bool   `json:"password_require_lowercase"`
	PasswordRequireDigit         bool   `json:"password_require_digit"`
	PasswordRequireSpecial       bool   `json:"password_require_special"`
	PasswordHistoryCount         int    `json:"password_history_count"`
	MaintenanceMode              bool   `json:"maintenance_mode"`
	SiteName                     string `json:"site_name"`
	AccessTokenLifetimeMinutes   int    `json:"access_token_lifetime_minutes"`
	RefreshTokenLifetimeDays     int    `json:"refresh_token_lifetime_days"`
	MaxConcurrentSessions        int    `json:"max_concurrent_sessions"`
	IdleTimeoutMinutes           int    `json:"idle_timeout_minutes"`
}

type updateSettingsRequest struct {
	SessionTimeoutMinutes        *int    `json:"session_timeout_minutes"`
	MaxLoginAttempts             *int    `json:"max_login_attempts"`
	PasswordMinLength            *int    `json:"password_min_length"`
	PasswordRequireUppercase     *bool   `json:"password_require_uppercase"`
	PasswordRequireLowercase     *bool   `json:"password_require_lowercase"`
	PasswordRequireDigit         *bool   `json:"password_require_digit"`
	PasswordRequireSpecial       *bool   `json:"password_require_special"`
	PasswordHistoryCount         *int    `json:"password_history_count"`
	MaintenanceMode              *bool   `json:"maintenance_mode"`
	SiteName                     *string `json:"site_name"`
	AccessTokenLifetimeMinutes   *int    `json:"access_token_lifetime_minutes"`
	RefreshTokenLifetimeDays     *int    `json:"refresh_token_lifetime_days"`
	MaxConcurrentSessions        *int    `json:"max_concurrent_sessions"`
	IdleTimeoutMinutes           *int    `json:"idle_timeout_minutes"`
}

// Get handles GET /api/v1/settings.
func (d *Deps) Get(w http.ResponseWriter, r *http.Request) {
	correlationID := middleware.GetCorrelationID(r.Context())
	callerClaims := middleware.ClaimsFromContext(r.Context())
	if callerClaims == nil {
		response.Error(w, "UNAUTHORIZED", "authentication required", http.StatusUnauthorized, correlationID)
		return
	}

	settings, err := loadSettings(r.Context(), d.DB)
	if err != nil {
		response.Error(w, "INTERNAL_ERROR", "failed to load settings", http.StatusInternalServerError, correlationID)
		return
	}

	// Log sensitive read.
	if d.AuditSvc != nil {
		actorID := callerClaims.Subject
		resType := "settings"
		_ = d.AuditSvc.Log(r.Context(), audit.LogEntry{
			Category:      audit.CategoryUserActivity,
			Severity:      audit.SeverityInfo,
			ActorType:     audit.ActorTypeUser,
			ActorID:       &actorID,
			ActorLabel:    callerClaims.Username,
			Action:        "settings.read",
			ResourceType:  &resType,
			CorrelationID: correlationID,
		})
	}

	response.Success(w, settings)
}

// Update handles PUT /api/v1/settings.
func (d *Deps) Update(w http.ResponseWriter, r *http.Request) {
	correlationID := middleware.GetCorrelationID(r.Context())
	callerClaims := middleware.ClaimsFromContext(r.Context())
	if callerClaims == nil {
		response.Error(w, "UNAUTHORIZED", "authentication required", http.StatusUnauthorized, correlationID)
		return
	}

	var req updateSettingsRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.Error(w, "BAD_REQUEST", "invalid request body", http.StatusBadRequest, correlationID)
		return
	}

	if req.SessionTimeoutMinutes != nil {
		if err := upsertSetting(r.Context(), d.DB, "session_timeout_minutes", *req.SessionTimeoutMinutes); err != nil {
			response.Error(w, "INTERNAL_ERROR", "failed to update setting", http.StatusInternalServerError, correlationID)
			return
		}
	}
	if req.MaxLoginAttempts != nil {
		if err := upsertSetting(r.Context(), d.DB, "max_login_attempts", *req.MaxLoginAttempts); err != nil {
			response.Error(w, "INTERNAL_ERROR", "failed to update setting", http.StatusInternalServerError, correlationID)
			return
		}
	}
	if req.PasswordMinLength != nil {
		if err := upsertSetting(r.Context(), d.DB, "password_min_length", *req.PasswordMinLength); err != nil {
			response.Error(w, "INTERNAL_ERROR", "failed to update setting", http.StatusInternalServerError, correlationID)
			return
		}
	}
	if req.PasswordRequireUppercase != nil {
		if err := upsertSetting(r.Context(), d.DB, "password_require_uppercase", *req.PasswordRequireUppercase); err != nil {
			response.Error(w, "INTERNAL_ERROR", "failed to update setting", http.StatusInternalServerError, correlationID)
			return
		}
	}
	if req.PasswordRequireLowercase != nil {
		if err := upsertSetting(r.Context(), d.DB, "password_require_lowercase", *req.PasswordRequireLowercase); err != nil {
			response.Error(w, "INTERNAL_ERROR", "failed to update setting", http.StatusInternalServerError, correlationID)
			return
		}
	}
	if req.PasswordRequireDigit != nil {
		if err := upsertSetting(r.Context(), d.DB, "password_require_digit", *req.PasswordRequireDigit); err != nil {
			response.Error(w, "INTERNAL_ERROR", "failed to update setting", http.StatusInternalServerError, correlationID)
			return
		}
	}
	if req.PasswordRequireSpecial != nil {
		if err := upsertSetting(r.Context(), d.DB, "password_require_special", *req.PasswordRequireSpecial); err != nil {
			response.Error(w, "INTERNAL_ERROR", "failed to update setting", http.StatusInternalServerError, correlationID)
			return
		}
	}
	if req.PasswordHistoryCount != nil {
		if err := upsertSetting(r.Context(), d.DB, "password_history_count", *req.PasswordHistoryCount); err != nil {
			response.Error(w, "INTERNAL_ERROR", "failed to update setting", http.StatusInternalServerError, correlationID)
			return
		}
	}
	if req.MaintenanceMode != nil {
		if err := upsertSetting(r.Context(), d.DB, "maintenance_mode", *req.MaintenanceMode); err != nil {
			response.Error(w, "INTERNAL_ERROR", "failed to update setting", http.StatusInternalServerError, correlationID)
			return
		}
	}
	if req.SiteName != nil {
		if err := upsertSetting(r.Context(), d.DB, "site_name", *req.SiteName); err != nil {
			response.Error(w, "INTERNAL_ERROR", "failed to update setting", http.StatusInternalServerError, correlationID)
			return
		}
	}
	if req.AccessTokenLifetimeMinutes != nil {
		if err := upsertSetting(r.Context(), d.DB, "jwt_access_token_lifetime_minutes", *req.AccessTokenLifetimeMinutes); err != nil {
			response.Error(w, "INTERNAL_ERROR", "failed to update setting", http.StatusInternalServerError, correlationID)
			return
		}
	}
	if req.RefreshTokenLifetimeDays != nil {
		if err := upsertSetting(r.Context(), d.DB, "jwt_refresh_token_lifetime_days", *req.RefreshTokenLifetimeDays); err != nil {
			response.Error(w, "INTERNAL_ERROR", "failed to update setting", http.StatusInternalServerError, correlationID)
			return
		}
	}
	if req.MaxConcurrentSessions != nil {
		if err := upsertSetting(r.Context(), d.DB, "max_concurrent_sessions", *req.MaxConcurrentSessions); err != nil {
			response.Error(w, "INTERNAL_ERROR", "failed to update setting", http.StatusInternalServerError, correlationID)
			return
		}
	}
	if req.IdleTimeoutMinutes != nil {
		if err := upsertSetting(r.Context(), d.DB, "idle_timeout_minutes", *req.IdleTimeoutMinutes); err != nil {
			response.Error(w, "INTERNAL_ERROR", "failed to update setting", http.StatusInternalServerError, correlationID)
			return
		}
	}

	if d.AuditSvc != nil {
		actorID := callerClaims.Subject
		resType := "settings"
		_ = d.AuditSvc.Log(r.Context(), audit.LogEntry{
			Category:      audit.CategoryUserActivity,
			Severity:      audit.SeverityInfo,
			ActorType:     audit.ActorTypeUser,
			ActorID:       &actorID,
			ActorLabel:    callerClaims.Username,
			Action:        "settings.updated",
			ResourceType:  &resType,
			CorrelationID: correlationID,
		})
	}

	w.WriteHeader(http.StatusNoContent)
}
