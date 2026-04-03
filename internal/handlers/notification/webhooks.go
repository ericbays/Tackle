package notification

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"tackle/internal/middleware"
	"tackle/pkg/response"
)

// webhookRow is the JSON shape for a webhook endpoint.
type webhookRow struct {
	ID         string    `json:"id"`
	Name       string    `json:"name"`
	URL        string    `json:"url"`
	AuthType   string    `json:"auth_type"`
	Categories []string  `json:"categories"`
	IsEnabled  bool      `json:"is_enabled"`
	CreatedBy  string    `json:"created_by"`
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`
}

// ListWebhooks handles GET /api/v1/webhooks.
func (d *Deps) ListWebhooks(w http.ResponseWriter, r *http.Request) {
	correlationID := middleware.GetCorrelationID(r.Context())

	rows, err := d.DB.QueryContext(r.Context(), `
		SELECT id, name, url, auth_type, categories, is_enabled, created_by, created_at, updated_at
		FROM webhook_endpoints
		ORDER BY created_at DESC`)
	if err != nil {
		response.Error(w, "INTERNAL_ERROR", "failed to query webhooks", http.StatusInternalServerError, correlationID)
		return
	}
	defer rows.Close()

	webhooks := make([]webhookRow, 0)
	for rows.Next() {
		var wh webhookRow
		var categories sql.NullString
		if err := rows.Scan(&wh.ID, &wh.Name, &wh.URL, &wh.AuthType, &categories, &wh.IsEnabled, &wh.CreatedBy, &wh.CreatedAt, &wh.UpdatedAt); err != nil {
			response.Error(w, "INTERNAL_ERROR", "failed to read webhook", http.StatusInternalServerError, correlationID)
			return
		}
		if categories.Valid && categories.String != "" {
			wh.Categories = parseTextArray(categories.String)
		}
		webhooks = append(webhooks, wh)
	}
	if err := rows.Err(); err != nil {
		response.Error(w, "INTERNAL_ERROR", "failed to iterate webhooks", http.StatusInternalServerError, correlationID)
		return
	}

	response.Success(w, webhooks)
}

// createWebhookRequest is the request body for CreateWebhook.
type createWebhookRequest struct {
	Name       string   `json:"name"`
	URL        string   `json:"url"`
	AuthType   string   `json:"auth_type"`
	Secret     string   `json:"secret"`
	Categories []string `json:"categories"`
	IsEnabled  bool     `json:"is_enabled"`
}

// CreateWebhook handles POST /api/v1/webhooks.
func (d *Deps) CreateWebhook(w http.ResponseWriter, r *http.Request) {
	correlationID := middleware.GetCorrelationID(r.Context())
	claims := middleware.ClaimsFromContext(r.Context())
	if claims == nil {
		response.Error(w, "UNAUTHORIZED", "authentication required", http.StatusUnauthorized, correlationID)
		return
	}

	var req createWebhookRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.Error(w, "BAD_REQUEST", "invalid request body", http.StatusBadRequest, correlationID)
		return
	}
	if req.Name == "" || req.URL == "" {
		response.Error(w, "BAD_REQUEST", "name and url are required", http.StatusBadRequest, correlationID)
		return
	}
	if req.AuthType == "" {
		req.AuthType = "none"
	}

	id := uuid.New().String()
	var authConfig []byte
	// Secret is stored as-is for simplicity — in production this would be encrypted.
	if req.Secret != "" {
		authConfig = []byte(req.Secret)
	}

	cats := formatTextArray(req.Categories)

	_, err := d.DB.ExecContext(r.Context(), `
		INSERT INTO webhook_endpoints (id, name, url, auth_type, auth_config, categories, is_enabled, created_by)
		VALUES ($1, $2, $3, $4, $5, $6::text[], $7, $8::uuid)`,
		id, req.Name, req.URL, req.AuthType, authConfig, cats, req.IsEnabled, claims.Subject,
	)
	if err != nil {
		response.Error(w, "INTERNAL_ERROR", "failed to create webhook", http.StatusInternalServerError, correlationID)
		return
	}

	response.Created(w, map[string]string{"id": id})
}

// DeleteWebhook handles DELETE /api/v1/webhooks/{id}.
func (d *Deps) DeleteWebhook(w http.ResponseWriter, r *http.Request) {
	correlationID := middleware.GetCorrelationID(r.Context())
	id := chi.URLParam(r, "id")

	result, err := d.DB.ExecContext(r.Context(), `DELETE FROM webhook_endpoints WHERE id = $1::uuid`, id)
	if err != nil {
		response.Error(w, "INTERNAL_ERROR", "failed to delete webhook", http.StatusInternalServerError, correlationID)
		return
	}
	n, _ := result.RowsAffected()
	if n == 0 {
		response.Error(w, "NOT_FOUND", "webhook not found", http.StatusNotFound, correlationID)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// ToggleWebhook handles PUT /api/v1/webhooks/{id}/toggle.
func (d *Deps) ToggleWebhook(w http.ResponseWriter, r *http.Request) {
	correlationID := middleware.GetCorrelationID(r.Context())
	id := chi.URLParam(r, "id")

	result, err := d.DB.ExecContext(r.Context(), `
		UPDATE webhook_endpoints SET is_enabled = NOT is_enabled WHERE id = $1::uuid`, id)
	if err != nil {
		response.Error(w, "INTERNAL_ERROR", "failed to toggle webhook", http.StatusInternalServerError, correlationID)
		return
	}
	n, _ := result.RowsAffected()
	if n == 0 {
		response.Error(w, "NOT_FOUND", "webhook not found", http.StatusNotFound, correlationID)
		return
	}
	response.Success(w, map[string]string{"status": "toggled"})
}

// WebhookDeliveries handles GET /api/v1/webhooks/{id}/deliveries.
func (d *Deps) WebhookDeliveries(w http.ResponseWriter, r *http.Request) {
	correlationID := middleware.GetCorrelationID(r.Context())
	id := chi.URLParam(r, "id")

	rows, err := d.DB.QueryContext(r.Context(), `
		SELECT id, webhook_id, notification_id, status, response_code, response_body, attempted_at, retry_count
		FROM webhook_deliveries
		WHERE webhook_id = $1::uuid
		ORDER BY attempted_at DESC
		LIMIT 50`, id)
	if err != nil {
		response.Error(w, "INTERNAL_ERROR", "failed to query deliveries", http.StatusInternalServerError, correlationID)
		return
	}
	defer rows.Close()

	type delivery struct {
		ID             string    `json:"id"`
		WebhookID      string    `json:"webhook_id"`
		NotificationID string    `json:"notification_id"`
		Status         string    `json:"status"`
		ResponseCode   int       `json:"response_code"`
		ResponseBody   string    `json:"response_body"`
		AttemptedAt    time.Time `json:"attempted_at"`
		RetryCount     int       `json:"retry_count"`
	}
	deliveries := make([]delivery, 0)
	for rows.Next() {
		var d delivery
		var respBody sql.NullString
		var respCode sql.NullInt32
		if err := rows.Scan(&d.ID, &d.WebhookID, &d.NotificationID, &d.Status, &respCode, &respBody, &d.AttemptedAt, &d.RetryCount); err != nil {
			response.Error(w, "INTERNAL_ERROR", "failed to read delivery", http.StatusInternalServerError, correlationID)
			return
		}
		if respCode.Valid {
			d.ResponseCode = int(respCode.Int32)
		}
		if respBody.Valid {
			d.ResponseBody = respBody.String
		}
		deliveries = append(deliveries, d)
	}
	if err := rows.Err(); err != nil {
		response.Error(w, "INTERNAL_ERROR", "failed to iterate deliveries", http.StatusInternalServerError, correlationID)
		return
	}

	response.Success(w, deliveries)
}

// parseTextArray parses a PostgreSQL text array literal {a,b,c}.
func parseTextArray(s string) []string {
	s = trimBraces(s)
	if s == "" {
		return nil
	}
	return splitArray(s)
}

func trimBraces(s string) string {
	if len(s) >= 2 && s[0] == '{' && s[len(s)-1] == '}' {
		return s[1 : len(s)-1]
	}
	return s
}

func splitArray(s string) []string {
	parts := make([]string, 0)
	for _, p := range split(s, ',') {
		parts = append(parts, p)
	}
	return parts
}

func split(s string, sep byte) []string {
	var result []string
	start := 0
	for i := 0; i < len(s); i++ {
		if s[i] == sep {
			result = append(result, s[start:i])
			start = i + 1
		}
	}
	result = append(result, s[start:])
	return result
}

func formatTextArray(items []string) string {
	if len(items) == 0 {
		return "{}"
	}
	s := "{"
	for i, item := range items {
		if i > 0 {
			s += ","
		}
		s += item
	}
	s += "}"
	return s
}
