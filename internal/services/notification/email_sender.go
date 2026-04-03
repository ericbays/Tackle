package notification

import (
	"context"
	"crypto/tls"
	"database/sql"
	"fmt"
	"html/template"
	"log/slog"
	"net/smtp"
	"strings"
	"sync"
	"time"
)

// Decryptor decrypts stored credentials. Implemented by crypto.EncryptionService
// and providers/credentials.EncryptionService.
type Decryptor interface {
	DecryptString(ciphertext []byte) (string, error)
}

// EmailSender handles sending notification emails via the system SMTP config.
type EmailSender struct {
	db     *sql.DB
	encSvc Decryptor

	mu     sync.RWMutex
	config *smtpConfig
	loaded time.Time
}

// smtpConfig is the cached system SMTP configuration for notification emails.
type smtpConfig struct {
	Host        string
	Port        int
	AuthType    string
	Username    string
	Password    string
	TLSMode     string
	FromAddress string
	FromName    string
}

const smtpConfigCacheTTL = 60 * time.Second

// NewEmailSender creates an EmailSender that reads config from notification_smtp_config.
func NewEmailSender(db *sql.DB, encSvc Decryptor) *EmailSender {
	return &EmailSender{db: db, encSvc: encSvc}
}

// SendNotificationEmail sends a notification email to the given address if
// the system SMTP is configured. Returns nil if SMTP is not configured.
func (s *EmailSender) SendNotificationEmail(ctx context.Context, toEmail, title, body, actionURL string) error {
	cfg, err := s.getConfig(ctx)
	if err != nil {
		return fmt.Errorf("email_sender: get config: %w", err)
	}
	if cfg == nil {
		return nil // SMTP not configured — skip silently
	}

	htmlBody := renderEmailHTML(cfg.FromName, title, body, actionURL)
	return s.send(cfg, toEmail, title, htmlBody)
}

// getConfig returns the cached or freshly loaded SMTP config. Returns nil if no config exists.
func (s *EmailSender) getConfig(ctx context.Context) (*smtpConfig, error) {
	s.mu.RLock()
	if s.config != nil && time.Since(s.loaded) < smtpConfigCacheTTL {
		c := s.config
		s.mu.RUnlock()
		return c, nil
	}
	s.mu.RUnlock()

	s.mu.Lock()
	defer s.mu.Unlock()

	// Double-check after acquiring write lock.
	if s.config != nil && time.Since(s.loaded) < smtpConfigCacheTTL {
		return s.config, nil
	}

	var cfg smtpConfig
	var passwordEnc []byte
	var username sql.NullString
	err := s.db.QueryRowContext(ctx, `
		SELECT host, port, auth_type, username, password, tls_mode, from_address, from_name
		FROM notification_smtp_config
		ORDER BY created_at DESC LIMIT 1`,
	).Scan(&cfg.Host, &cfg.Port, &cfg.AuthType, &username, &passwordEnc, &cfg.TLSMode, &cfg.FromAddress, &cfg.FromName)
	if err == sql.ErrNoRows {
		s.config = nil
		s.loaded = time.Now()
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	if username.Valid {
		cfg.Username = username.String
	}
	if len(passwordEnc) > 0 && s.encSvc != nil {
		pw, err := s.encSvc.DecryptString(passwordEnc)
		if err != nil {
			return nil, fmt.Errorf("decrypt smtp password: %w", err)
		}
		cfg.Password = pw
	}

	s.config = &cfg
	s.loaded = time.Now()
	return &cfg, nil
}

// send performs the actual SMTP delivery.
func (s *EmailSender) send(cfg *smtpConfig, to, subject, htmlBody string) error {
	addr := fmt.Sprintf("%s:%d", cfg.Host, cfg.Port)
	from := cfg.FromAddress

	msg := buildMIME(from, cfg.FromName, to, subject, htmlBody)

	tlsCfg := &tls.Config{
		ServerName: cfg.Host,
		MinVersion: tls.VersionTLS12,
	}

	var c *smtp.Client
	var err error

	if cfg.TLSMode == "tls" {
		conn, connErr := tls.Dial("tcp", addr, tlsCfg)
		if connErr != nil {
			return fmt.Errorf("smtp tls dial: %w", connErr)
		}
		c, err = smtp.NewClient(conn, cfg.Host)
	} else {
		c, err = smtp.Dial(addr)
	}
	if err != nil {
		return fmt.Errorf("smtp connect: %w", err)
	}
	defer c.Close()

	if cfg.TLSMode == "starttls" {
		if err := c.StartTLS(tlsCfg); err != nil {
			return fmt.Errorf("smtp starttls: %w", err)
		}
	}

	if cfg.AuthType != "none" && cfg.Username != "" {
		auth := smtp.PlainAuth("", cfg.Username, cfg.Password, cfg.Host)
		if err := c.Auth(auth); err != nil {
			return fmt.Errorf("smtp auth: %w", err)
		}
	}

	if err := c.Mail(from); err != nil {
		return fmt.Errorf("smtp mail: %w", err)
	}
	if err := c.Rcpt(to); err != nil {
		return fmt.Errorf("smtp rcpt: %w", err)
	}
	w, err := c.Data()
	if err != nil {
		return fmt.Errorf("smtp data: %w", err)
	}
	if _, err := w.Write([]byte(msg)); err != nil {
		return fmt.Errorf("smtp write: %w", err)
	}
	if err := w.Close(); err != nil {
		return fmt.Errorf("smtp data close: %w", err)
	}
	return c.Quit()
}

func buildMIME(from, fromName, to, subject, htmlBody string) string {
	var b strings.Builder
	if fromName != "" {
		fmt.Fprintf(&b, "From: %s <%s>\r\n", fromName, from)
	} else {
		fmt.Fprintf(&b, "From: %s\r\n", from)
	}
	fmt.Fprintf(&b, "To: %s\r\n", to)
	fmt.Fprintf(&b, "Subject: %s\r\n", subject)
	b.WriteString("MIME-Version: 1.0\r\n")
	b.WriteString("Content-Type: text/html; charset=UTF-8\r\n")
	b.WriteString("\r\n")
	b.WriteString(htmlBody)
	return b.String()
}

var emailTmpl = template.Must(template.New("notif").Parse(`<!DOCTYPE html>
<html>
<head><meta charset="UTF-8"></head>
<body style="font-family:sans-serif;background:#1a1a2e;color:#e0e0e0;padding:20px;">
<div style="max-width:600px;margin:0 auto;background:#16213e;border-radius:8px;padding:24px;">
<h2 style="color:#e0e0e0;margin:0 0 12px;">{{.Title}}</h2>
<p style="color:#b0b0b0;line-height:1.6;">{{.Body}}</p>
{{if .ActionURL}}<p style="margin-top:16px;"><a href="{{.ActionURL}}" style="display:inline-block;padding:10px 20px;background:#3b82f6;color:#fff;text-decoration:none;border-radius:6px;">View Details</a></p>{{end}}
<hr style="border:0;border-top:1px solid #2a2a4a;margin:20px 0;">
<p style="color:#666;font-size:12px;">Sent by {{.FromName}} notification system</p>
</div>
</body>
</html>`))

func renderEmailHTML(fromName, title, body, actionURL string) string {
	var buf strings.Builder
	data := struct {
		FromName  string
		Title     string
		Body      string
		ActionURL string
	}{fromName, title, body, actionURL}
	if err := emailTmpl.Execute(&buf, data); err != nil {
		slog.Error("email_sender: render template", "error", err)
		return fmt.Sprintf("<p>%s</p><p>%s</p>", title, body)
	}
	return buf.String()
}

// CheckUserPreference checks if the user has email enabled for the given category.
func (s *EmailSender) CheckUserPreference(ctx context.Context, userID, category string) (enabled bool, mode string, err error) {
	var emailEnabled bool
	var emailMode string
	err = s.db.QueryRowContext(ctx, `
		SELECT email_enabled, email_mode
		FROM notification_preferences
		WHERE user_id = $1::uuid AND category = $2`,
		userID, category,
	).Scan(&emailEnabled, &emailMode)
	if err == sql.ErrNoRows {
		return false, "", nil
	}
	if err != nil {
		return false, "", err
	}
	return emailEnabled, emailMode, nil
}

// GetUserEmail fetches the user's email address.
func (s *EmailSender) GetUserEmail(ctx context.Context, userID string) (string, error) {
	var email string
	err := s.db.QueryRowContext(ctx, `SELECT email FROM users WHERE id = $1::uuid`, userID).Scan(&email)
	if err != nil {
		return "", err
	}
	return email, nil
}
