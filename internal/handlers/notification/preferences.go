package notification

import (
	"database/sql"
	"encoding/json"
	"net/http"

	"tackle/internal/middleware"
	"tackle/pkg/response"
)

// preferenceRow is the JSON shape for a single notification preference entry.
type preferenceRow struct {
	ID             string `json:"id"`
	Category       string `json:"category"`
	EmailEnabled   bool   `json:"email_enabled"`
	InAppEnabled   bool   `json:"in_app_enabled"` // always true
	EmailMode      string `json:"email_mode"`
	DigestInterval string `json:"digest_interval"`
}

// GetPreferences handles GET /api/v1/notifications/preferences.
func (d *Deps) GetPreferences(w http.ResponseWriter, r *http.Request) {
	correlationID := middleware.GetCorrelationID(r.Context())
	claims := middleware.ClaimsFromContext(r.Context())
	if claims == nil {
		response.Error(w, "UNAUTHORIZED", "authentication required", http.StatusUnauthorized, correlationID)
		return
	}

	rows, err := d.DB.QueryContext(r.Context(), `
		SELECT id, category, email_enabled, email_mode, digest_interval
		FROM notification_preferences
		WHERE user_id = $1::uuid
		ORDER BY category`,
		claims.Subject,
	)
	if err != nil {
		response.Error(w, "INTERNAL_ERROR", "failed to query preferences", http.StatusInternalServerError, correlationID)
		return
	}
	defer rows.Close()

	prefs := make([]preferenceRow, 0)
	for rows.Next() {
		var p preferenceRow
		if err := rows.Scan(&p.ID, &p.Category, &p.EmailEnabled, &p.EmailMode, &p.DigestInterval); err != nil {
			response.Error(w, "INTERNAL_ERROR", "failed to read preference", http.StatusInternalServerError, correlationID)
			return
		}
		p.InAppEnabled = true // always true
		prefs = append(prefs, p)
	}
	if err := rows.Err(); err != nil {
		response.Error(w, "INTERNAL_ERROR", "failed to iterate preferences", http.StatusInternalServerError, correlationID)
		return
	}

	response.Success(w, prefs)
}

// preferenceInput is one item in the PUT request body.
type preferenceInput struct {
	Category       string `json:"category"`
	EmailEnabled   bool   `json:"email_enabled"`
	EmailMode      string `json:"email_mode"`
	DigestInterval string `json:"digest_interval"`
}

// preferencesRequest is the request body for PUT /api/v1/notifications/preferences.
type preferencesRequest struct {
	Preferences []preferenceInput `json:"preferences"`
}

// UpdatePreferences handles PUT /api/v1/notifications/preferences.
func (d *Deps) UpdatePreferences(w http.ResponseWriter, r *http.Request) {
	correlationID := middleware.GetCorrelationID(r.Context())
	claims := middleware.ClaimsFromContext(r.Context())
	if claims == nil {
		response.Error(w, "UNAUTHORIZED", "authentication required", http.StatusUnauthorized, correlationID)
		return
	}

	var req preferencesRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.Error(w, "BAD_REQUEST", "invalid request body", http.StatusBadRequest, correlationID)
		return
	}

	tx, err := d.DB.BeginTx(r.Context(), nil)
	if err != nil {
		response.Error(w, "INTERNAL_ERROR", "failed to begin transaction", http.StatusInternalServerError, correlationID)
		return
	}
	defer tx.Rollback() //nolint:errcheck

	for _, pref := range req.Preferences {
		if pref.Category == "" {
			continue
		}
		emailMode := pref.EmailMode
		if emailMode == "" {
			emailMode = "digest"
		}
		digestInterval := pref.DigestInterval
		if digestInterval == "" {
			digestInterval = "daily"
		}

		_, err := tx.ExecContext(r.Context(), `
			INSERT INTO notification_preferences (user_id, category, email_enabled, email_mode, digest_interval)
			VALUES ($1::uuid, $2, $3, $4, $5)
			ON CONFLICT (user_id, category) DO UPDATE
			SET email_enabled = EXCLUDED.email_enabled,
			    email_mode = EXCLUDED.email_mode,
			    digest_interval = EXCLUDED.digest_interval`,
			claims.Subject, pref.Category, pref.EmailEnabled, emailMode, digestInterval,
		)
		if err != nil {
			response.Error(w, "INTERNAL_ERROR", "failed to upsert preference", http.StatusInternalServerError, correlationID)
			return
		}
	}

	if err := tx.Commit(); err != nil {
		response.Error(w, "INTERNAL_ERROR", "failed to commit preferences", http.StatusInternalServerError, correlationID)
		return
	}

	// Fetch and return updated preferences.
	rows, err := d.DB.QueryContext(r.Context(), `
		SELECT id, category, email_enabled, email_mode, digest_interval
		FROM notification_preferences
		WHERE user_id = $1::uuid
		ORDER BY category`,
		claims.Subject,
	)
	if err != nil {
		response.Error(w, "INTERNAL_ERROR", "failed to fetch updated preferences", http.StatusInternalServerError, correlationID)
		return
	}
	defer rows.Close()

	prefs := make([]preferenceRow, 0)
	for rows.Next() {
		var p preferenceRow
		if err := rows.Scan(&p.ID, &p.Category, &p.EmailEnabled, &p.EmailMode, &p.DigestInterval); err != nil {
			response.Error(w, "INTERNAL_ERROR", "failed to read preference", http.StatusInternalServerError, correlationID)
			return
		}
		p.InAppEnabled = true
		prefs = append(prefs, p)
	}
	if err := rows.Err(); err != nil {
		response.Error(w, "INTERNAL_ERROR", "failed to iterate preferences", http.StatusInternalServerError, correlationID)
		return
	}

	response.Success(w, prefs)
}

// scanNotificationRow scans a single notification row (used in read.go).
func scanNotificationRow(row *sql.Row) (map[string]any, error) {
	var (
		id, userID, category, severity, title, body string
		resourceType, resourceID, actionURL          sql.NullString
		isRead                                       bool
		expiresAt                                    sql.NullTime
		createdAt                                    sql.NullTime
	)
	err := row.Scan(&id, &userID, &category, &severity, &title, &body,
		&resourceType, &resourceID, &actionURL, &isRead, &expiresAt, &createdAt)
	if err != nil {
		return nil, err
	}

	out := map[string]any{
		"id":         id,
		"user_id":    userID,
		"category":   category,
		"severity":   severity,
		"title":      title,
		"body":       body,
		"is_read":    isRead,
		"created_at": createdAt.Time,
	}
	if resourceType.Valid {
		out["resource_type"] = resourceType.String
	}
	if resourceID.Valid {
		out["resource_id"] = resourceID.String
	}
	if actionURL.Valid {
		out["action_url"] = actionURL.String
	}
	if expiresAt.Valid {
		out["expires_at"] = expiresAt.Time
	}
	return out, nil
}
