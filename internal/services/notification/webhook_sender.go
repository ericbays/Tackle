package notification

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/google/uuid"
)

// WebhookEndpoint represents a configured webhook target.
type WebhookEndpoint struct {
	ID         string   `json:"id"`
	Name       string   `json:"name"`
	URL        string   `json:"url"`
	AuthType   string   `json:"auth_type"` // none, hmac, bearer, basic
	AuthConfig []byte   `json:"-"`
	Categories []string `json:"categories"`
	IsEnabled  bool     `json:"is_enabled"`
	CreatedBy  string   `json:"created_by"`
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`
}

// WebhookPayload is the JSON payload sent to webhook endpoints.
type WebhookPayload struct {
	EventID    string       `json:"event_id"`
	EventType  string       `json:"event_type"`
	Timestamp  time.Time    `json:"timestamp"`
	Notification *Notification `json:"notification"`
}

// WebhookSender delivers notifications to configured webhook endpoints.
type WebhookSender struct {
	db     *sql.DB
	encSvc Decryptor
	client *http.Client
}

// NewWebhookSender creates a WebhookSender.
func NewWebhookSender(db *sql.DB, encSvc Decryptor) *WebhookSender {
	return &WebhookSender{
		db:     db,
		encSvc: encSvc,
		client: &http.Client{Timeout: 10 * time.Second},
	}
}

// DeliverToMatchingEndpoints sends a notification to all enabled webhook endpoints
// whose category filter matches. Runs asynchronously — errors are logged, not returned.
func (s *WebhookSender) DeliverToMatchingEndpoints(ctx context.Context, n *Notification) {
	endpoints, err := s.getMatchingEndpoints(ctx, n.Category)
	if err != nil {
		slog.Error("webhook_sender: get endpoints", "error", err)
		return
	}

	for _, ep := range endpoints {
		go func(ep WebhookEndpoint) {
			if err := s.deliver(context.Background(), ep, n); err != nil {
				slog.Error("webhook_sender: deliver failed", "webhook_id", ep.ID, "error", err)
			}
		}(ep)
	}
}

// getMatchingEndpoints returns enabled endpoints where the category filter matches
// (empty categories = all categories).
func (s *WebhookSender) getMatchingEndpoints(ctx context.Context, category string) ([]WebhookEndpoint, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, name, url, auth_type, auth_config, categories, is_enabled, created_by, created_at, updated_at
		FROM webhook_endpoints
		WHERE is_enabled = TRUE
		  AND (categories IS NULL OR categories = '{}' OR $1 = ANY(categories))`,
		category,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var endpoints []WebhookEndpoint
	for rows.Next() {
		var ep WebhookEndpoint
		var authConfig []byte
		var categories sql.NullString
		err := rows.Scan(&ep.ID, &ep.Name, &ep.URL, &ep.AuthType, &authConfig, &categories, &ep.IsEnabled, &ep.CreatedBy, &ep.CreatedAt, &ep.UpdatedAt)
		if err != nil {
			return nil, err
		}
		ep.AuthConfig = authConfig
		if categories.Valid && categories.String != "" {
			ep.Categories = parsePostgresArray(categories.String)
		}
		endpoints = append(endpoints, ep)
	}
	return endpoints, rows.Err()
}

// deliver sends a single webhook delivery with retry logic.
func (s *WebhookSender) deliver(ctx context.Context, ep WebhookEndpoint, n *Notification) error {
	// SSRF prevention: reject private IPs.
	if err := validateWebhookURL(ep.URL); err != nil {
		return s.recordDelivery(ctx, ep.ID, n.ID, "failed", 0, err.Error(), 0)
	}

	payload := WebhookPayload{
		EventID:      uuid.New().String(),
		EventType:    "notification." + n.Category,
		Timestamp:    time.Now().UTC(),
		Notification: n,
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal payload: %w", err)
	}

	// Retry up to 3 times with exponential backoff.
	delays := []time.Duration{0, 5 * time.Second, 15 * time.Second}
	var lastErr error
	for attempt, delay := range delays {
		if delay > 0 {
			time.Sleep(delay)
		}

		statusCode, respBody, err := s.doRequest(ctx, ep, body)
		if err == nil && statusCode >= 200 && statusCode < 300 {
			return s.recordDelivery(ctx, ep.ID, n.ID, "success", statusCode, respBody, attempt)
		}
		if err != nil {
			lastErr = err
		} else {
			lastErr = fmt.Errorf("http %d: %s", statusCode, truncate(respBody, 500))
		}
	}

	_ = s.recordDelivery(ctx, ep.ID, n.ID, "failed", 0, lastErr.Error(), len(delays)-1)
	return lastErr
}

// doRequest sends the HTTP POST with appropriate auth headers.
func (s *WebhookSender) doRequest(ctx context.Context, ep WebhookEndpoint, body []byte) (int, string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, ep.URL, bytes.NewReader(body))
	if err != nil {
		return 0, "", err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "Tackle-Webhook/1.0")

	switch ep.AuthType {
	case "hmac":
		secret := s.decryptAuthConfig(ep.AuthConfig)
		mac := hmac.New(sha256.New, []byte(secret))
		mac.Write(body)
		sig := hex.EncodeToString(mac.Sum(nil))
		req.Header.Set("X-Webhook-Signature", "sha256="+sig)
	case "bearer":
		token := s.decryptAuthConfig(ep.AuthConfig)
		req.Header.Set("Authorization", "Bearer "+token)
	case "basic":
		// auth_config stores "user:pass" encrypted
		creds := s.decryptAuthConfig(ep.AuthConfig)
		req.SetBasicAuth(strings.SplitN(creds, ":", 2)[0], strings.SplitN(creds, ":", 2)[1])
	}

	resp, err := s.client.Do(req)
	if err != nil {
		return 0, "", err
	}
	defer resp.Body.Close()

	respBytes, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
	return resp.StatusCode, string(respBytes), nil
}

func (s *WebhookSender) decryptAuthConfig(enc []byte) string {
	if len(enc) == 0 || s.encSvc == nil {
		return ""
	}
	plain, err := s.encSvc.DecryptString(enc)
	if err != nil {
		slog.Error("webhook_sender: decrypt auth_config", "error", err)
		return ""
	}
	return plain
}

// recordDelivery inserts a webhook_deliveries row.
func (s *WebhookSender) recordDelivery(ctx context.Context, webhookID, notifID, status string, code int, respBody string, retryCount int) error {
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO webhook_deliveries (id, webhook_id, notification_id, status, response_code, response_body, retry_count)
		VALUES ($1, $2::uuid, $3::uuid, $4, $5, $6, $7)`,
		uuid.New().String(), webhookID, notifID, status, code, truncate(respBody, 2000), retryCount,
	)
	return err
}

// validateWebhookURL rejects URLs that target private/internal IPs (SSRF prevention).
func validateWebhookURL(rawURL string) error {
	u, err := url.Parse(rawURL)
	if err != nil {
		return fmt.Errorf("invalid url: %w", err)
	}
	if u.Scheme != "https" && u.Scheme != "http" {
		return fmt.Errorf("scheme %q not allowed, must be http or https", u.Scheme)
	}
	host := u.Hostname()
	ips, err := net.LookupIP(host)
	if err != nil {
		return fmt.Errorf("dns lookup failed for %s: %w", host, err)
	}
	for _, ip := range ips {
		if ip.IsLoopback() || ip.IsPrivate() || ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() {
			return fmt.Errorf("private/internal IP %s not allowed (SSRF prevention)", ip)
		}
	}
	return nil
}

func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen]
}

// parsePostgresArray parses a simple PostgreSQL text array literal like {a,b,c}.
func parsePostgresArray(s string) []string {
	s = strings.TrimPrefix(s, "{")
	s = strings.TrimSuffix(s, "}")
	if s == "" {
		return nil
	}
	return strings.Split(s, ",")
}
