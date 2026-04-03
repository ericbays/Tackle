package settings

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
)

// loadSettings reads the current system settings from the system_settings table.
// Returns defaults for any settings not yet stored.
func loadSettings(ctx context.Context, db *sql.DB) (settingsResponse, error) {
	settings := settingsResponse{
		SessionTimeoutMinutes:      60,
		MaxLoginAttempts:           5,
		PasswordMinLength:          12,
		PasswordRequireUppercase:   true,
		PasswordRequireLowercase:   true,
		PasswordRequireDigit:       true,
		PasswordRequireSpecial:     false,
		PasswordHistoryCount:       0,
		MaintenanceMode:            false,
		SiteName:                   "Tackle",
		AccessTokenLifetimeMinutes: 15,
		RefreshTokenLifetimeDays:   7,
		MaxConcurrentSessions:      0,
		IdleTimeoutMinutes:         0,
	}

	rows, err := db.QueryContext(ctx, `SELECT key, value FROM system_settings`)
	if err != nil {
		// Table may not exist in early setup; return defaults.
		return settings, nil //nolint:nilerr
	}
	defer rows.Close()

	for rows.Next() {
		var key, val string
		if err := rows.Scan(&key, &val); err != nil {
			return settings, fmt.Errorf("scan setting: %w", err)
		}
		switch key {
		case "session_timeout_minutes":
			var v int
			if err := json.Unmarshal([]byte(val), &v); err == nil {
				settings.SessionTimeoutMinutes = v
			}
		case "max_login_attempts":
			var v int
			if err := json.Unmarshal([]byte(val), &v); err == nil {
				settings.MaxLoginAttempts = v
			}
		case "password_min_length":
			var v int
			if err := json.Unmarshal([]byte(val), &v); err == nil {
				settings.PasswordMinLength = v
			}
		case "password_require_uppercase":
			var v bool
			if err := json.Unmarshal([]byte(val), &v); err == nil {
				settings.PasswordRequireUppercase = v
			}
		case "password_require_lowercase":
			var v bool
			if err := json.Unmarshal([]byte(val), &v); err == nil {
				settings.PasswordRequireLowercase = v
			}
		case "password_require_digit":
			var v bool
			if err := json.Unmarshal([]byte(val), &v); err == nil {
				settings.PasswordRequireDigit = v
			}
		case "password_require_special":
			var v bool
			if err := json.Unmarshal([]byte(val), &v); err == nil {
				settings.PasswordRequireSpecial = v
			}
		case "password_history_count":
			var v int
			if err := json.Unmarshal([]byte(val), &v); err == nil {
				settings.PasswordHistoryCount = v
			}
		case "maintenance_mode":
			var v bool
			if err := json.Unmarshal([]byte(val), &v); err == nil {
				settings.MaintenanceMode = v
			}
		case "site_name":
			var v string
			if err := json.Unmarshal([]byte(val), &v); err == nil {
				settings.SiteName = v
			}
		case "jwt_access_token_lifetime_minutes":
			var v int
			if err := json.Unmarshal([]byte(val), &v); err == nil {
				settings.AccessTokenLifetimeMinutes = v
			}
		case "jwt_refresh_token_lifetime_days":
			var v int
			if err := json.Unmarshal([]byte(val), &v); err == nil {
				settings.RefreshTokenLifetimeDays = v
			}
		case "max_concurrent_sessions":
			var v int
			if err := json.Unmarshal([]byte(val), &v); err == nil {
				settings.MaxConcurrentSessions = v
			}
		case "idle_timeout_minutes":
			var v int
			if err := json.Unmarshal([]byte(val), &v); err == nil {
				settings.IdleTimeoutMinutes = v
			}
		}
	}
	return settings, rows.Err()
}

// upsertSetting inserts or updates a single key-value setting.
func upsertSetting(ctx context.Context, db *sql.DB, key string, value any) error {
	b, err := json.Marshal(value)
	if err != nil {
		return fmt.Errorf("marshal setting value: %w", err)
	}
	_, err = db.ExecContext(ctx,
		`INSERT INTO system_settings (key, value, updated_at)
		 VALUES ($1, $2, now())
		 ON CONFLICT (key) DO UPDATE SET value = EXCLUDED.value, updated_at = now()`,
		key, string(b),
	)
	if err != nil {
		return fmt.Errorf("upsert setting %q: %w", key, err)
	}
	return nil
}

// ensure system_settings table exists (fallback for missing migration).
// Not used at runtime — table is created by migrations.
var _ = sql.ErrNoRows
