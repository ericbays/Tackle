package users

import (
	"encoding/json"
	"net/http"

	"tackle/internal/middleware"
	"tackle/pkg/response"
)

// UserPreferences holds user display and workflow preferences.
type UserPreferences struct {
	Timezone        string `json:"timezone"`
	DateFormat      string `json:"date_format"`
	ItemsPerPage    int    `json:"items_per_page"`
	DefaultLanding  string `json:"default_landing"`
}

// GetPreferences handles GET /api/v1/users/me/preferences.
func (d *Deps) GetPreferences(w http.ResponseWriter, r *http.Request) {
	correlationID := middleware.GetCorrelationID(r.Context())
	claims := middleware.ClaimsFromContext(r.Context())
	if claims == nil {
		response.Error(w, "UNAUTHORIZED", "authentication required", http.StatusUnauthorized, correlationID)
		return
	}

	var raw []byte
	err := d.DB.QueryRowContext(r.Context(),
		`SELECT COALESCE(preferences, '{}') FROM users WHERE id = $1`, claims.Subject,
	).Scan(&raw)
	if err != nil {
		response.Error(w, "INTERNAL_ERROR", "failed to fetch preferences", http.StatusInternalServerError, correlationID)
		return
	}

	var prefs UserPreferences
	if err := json.Unmarshal(raw, &prefs); err != nil {
		// Return defaults on parse failure.
		prefs = UserPreferences{}
	}

	// Apply defaults for zero values.
	if prefs.Timezone == "" {
		prefs.Timezone = "UTC"
	}
	if prefs.DateFormat == "" {
		prefs.DateFormat = "YYYY-MM-DD"
	}
	if prefs.ItemsPerPage == 0 {
		prefs.ItemsPerPage = 25
	}
	if prefs.DefaultLanding == "" {
		prefs.DefaultLanding = "/"
	}

	response.Success(w, prefs)
}

// UpdatePreferences handles PUT /api/v1/users/me/preferences.
func (d *Deps) UpdatePreferences(w http.ResponseWriter, r *http.Request) {
	correlationID := middleware.GetCorrelationID(r.Context())
	claims := middleware.ClaimsFromContext(r.Context())
	if claims == nil {
		response.Error(w, "UNAUTHORIZED", "authentication required", http.StatusUnauthorized, correlationID)
		return
	}

	var prefs UserPreferences
	if err := json.NewDecoder(r.Body).Decode(&prefs); err != nil {
		response.Error(w, "BAD_REQUEST", "invalid request body", http.StatusBadRequest, correlationID)
		return
	}

	// Validate items_per_page range.
	if prefs.ItemsPerPage < 10 {
		prefs.ItemsPerPage = 10
	}
	if prefs.ItemsPerPage > 100 {
		prefs.ItemsPerPage = 100
	}

	raw, err := json.Marshal(prefs)
	if err != nil {
		response.Error(w, "INTERNAL_ERROR", "failed to encode preferences", http.StatusInternalServerError, correlationID)
		return
	}

	_, err = d.DB.ExecContext(r.Context(),
		`UPDATE users SET preferences = $1, updated_at = now() WHERE id = $2`,
		raw, claims.Subject,
	)
	if err != nil {
		response.Error(w, "INTERNAL_ERROR", "failed to save preferences", http.StatusInternalServerError, correlationID)
		return
	}

	response.Success(w, prefs)
}
