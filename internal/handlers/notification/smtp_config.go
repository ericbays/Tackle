package notification

import (
	"encoding/json"
	"net/http"
	"time"

	"tackle/internal/middleware"
	"tackle/pkg/response"
)

// smtpConfigRow is the JSON shape for the notification SMTP config.
type smtpConfigRow struct {
	ID          string    `json:"id"`
	Host        string    `json:"host"`
	Port        int       `json:"port"`
	AuthType    string    `json:"auth_type"`
	Username    string    `json:"username,omitempty"`
	TLSMode     string    `json:"tls_mode"`
	FromAddress string    `json:"from_address"`
	FromName    string    `json:"from_name"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// GetSMTPConfig handles GET /api/v1/notifications/smtp-config.
func (d *Deps) GetSMTPConfig(w http.ResponseWriter, r *http.Request) {
	var cfg smtpConfigRow
	err := d.DB.QueryRowContext(r.Context(), `
		SELECT id, host, port, auth_type, COALESCE(username, ''), tls_mode, from_address, from_name, created_at, updated_at
		FROM notification_smtp_config
		ORDER BY created_at DESC LIMIT 1`,
	).Scan(&cfg.ID, &cfg.Host, &cfg.Port, &cfg.AuthType, &cfg.Username, &cfg.TLSMode, &cfg.FromAddress, &cfg.FromName, &cfg.CreatedAt, &cfg.UpdatedAt)
	if err != nil {
		// No config yet — return null.
		response.Success(w, nil)
		return
	}
	response.Success(w, cfg)
}

// upsertSMTPConfigRequest is the request body for UpsertSMTPConfig.
type upsertSMTPConfigRequest struct {
	Host        string `json:"host"`
	Port        int    `json:"port"`
	AuthType    string `json:"auth_type"`
	Username    string `json:"username"`
	Password    string `json:"password"`
	TLSMode     string `json:"tls_mode"`
	FromAddress string `json:"from_address"`
	FromName    string `json:"from_name"`
}

// UpsertSMTPConfig handles PUT /api/v1/notifications/smtp-config.
func (d *Deps) UpsertSMTPConfig(w http.ResponseWriter, r *http.Request) {
	correlationID := middleware.GetCorrelationID(r.Context())

	var req upsertSMTPConfigRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.Error(w, "BAD_REQUEST", "invalid request body", http.StatusBadRequest, correlationID)
		return
	}
	if req.Host == "" || req.Port == 0 || req.FromAddress == "" {
		response.Error(w, "BAD_REQUEST", "host, port, and from_address are required", http.StatusBadRequest, correlationID)
		return
	}
	if req.AuthType == "" {
		req.AuthType = "plain"
	}
	if req.TLSMode == "" {
		req.TLSMode = "starttls"
	}
	if req.FromName == "" {
		req.FromName = "Tackle"
	}

	// For password storage: in a real deployment, this would be encrypted.
	// Using plaintext bytes for now — the notification_smtp_config.password column is BYTEA.
	var passwordBytes []byte
	if req.Password != "" {
		passwordBytes = []byte(req.Password)
	}

	// Delete existing config (only one row).
	_, _ = d.DB.ExecContext(r.Context(), `DELETE FROM notification_smtp_config`)

	_, err := d.DB.ExecContext(r.Context(), `
		INSERT INTO notification_smtp_config (host, port, auth_type, username, password, tls_mode, from_address, from_name)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)`,
		req.Host, req.Port, req.AuthType, req.Username, passwordBytes, req.TLSMode, req.FromAddress, req.FromName,
	)
	if err != nil {
		response.Error(w, "INTERNAL_ERROR", "failed to save SMTP config", http.StatusInternalServerError, correlationID)
		return
	}

	response.Success(w, map[string]string{"status": "saved"})
}
