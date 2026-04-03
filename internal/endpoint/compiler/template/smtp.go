package main

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	"mime/multipart"
	"net/smtp"
)

const (
	smtpDefaultPort = 9443
	smtpUserAgent   = "" // No identifying user agent
)

// EmailSendCommand represents a command to send an email received from the framework.
type EmailSendCommand struct {
	ID               string `json:"id"`
	Recipient        string `json:"recipient"`
	Subject          string `json:"subject"`
	BodyHTML         string `json:"body_html"`
	BodyText         string `json:"body_text"`
	SMTPHost         string `json:"smtp_host"`
	SMTPPort         int    `json:"smtp_port"`
	SMTPUsername     string `json:"smtp_username"`
	SMTPPassword     string `json:"smtp_password"`
	EnvelopeSender   string `json:"envelope_sender"`
	CampaignID       string `json:"campaign_id"`
	RateLimit        int    `json:"rate_limit,omitempty"`
	MaxConcurrent    int    `json:"max_concurrent,omitempty"`
	// Sanitization is always maximally applied. These fields are reserved
	// for future per-profile override support.
	_ [0]byte `json:"-"` // placeholder to keep struct layout stable
}

// EmailSendResult represents the result of an email send operation.
type EmailSendResult struct {
	ID           string `json:"id"`
	Success      bool   `json:"success"`
	SMTPCode     int    `json:"smtp_code"`
	SMTPMessage  string `json:"smtp_message"`
	ErrorMessage string `json:"error_message"`
	DeliveredAt  string `json:"delivered_at"`
}

// SMTPLogEntry represents a log entry sent to the framework after each send.
type SMTPLogEntry struct {
	EndpointID string `json:"endpoint_id"`
	CampaignID string `json:"campaign_id"`
	CommandID  string `json:"command_id"`
	Sender     string `json:"sender"`
	Recipient  string `json:"recipient"`
	Status     string `json:"status"`
	SMTPCode   int    `json:"smtp_code"`
	SMTPHost   string `json:"smtp_host"`
	Timestamp  string `json:"timestamp"`
}

// smtpRelay manages the control channel for email relay commands and cert updates.
type smtpRelay struct {
	frameworkHost string
	authToken     string
	controlPort   string
	certMgr       *certManager

	httpServer   *http.Server
	shutdownCh   chan struct{}
	shutdownOnce sync.Once

	// Token bucket for rate limiting
	rateMu     sync.Mutex
	tokens     float64
	lastRefill time.Time
	maxTokens  float64
	refillRate float64 // tokens per minute

	// Worker pool for concurrent sends
	workerCh chan *EmailSendCommand

	// In-memory credential storage (cleared after use)
	credentialStore *credentialStore
}

// credentialStore holds SMTP credentials in memory with clearing support.
type credentialStore struct {
	mu          sync.RWMutex
	credentials map[string]*smtpCredentials
}

// smtpCredentials holds auth credentials for an SMTP server.
type smtpCredentials struct {
	Username string
	Password []byte
}

// NewSMTPRelay creates a new SMTP relay instance.
func newSMTPRelay(endpointID, frameworkHost, authToken, controlPort string) *smtpRelay {
	relay := &smtpRelay{
		frameworkHost: frameworkHost,
		authToken:     authToken,
		controlPort:   controlPort,
		shutdownCh:    make(chan struct{}),
		credentialStore: &credentialStore{
			credentials: make(map[string]*smtpCredentials),
		},
	}

	// Set up worker pool
	maxConcurrent := relay.getMaxConcurrent()
	relay.workerCh = make(chan *EmailSendCommand, maxConcurrent)

	// Initialize token bucket with default rate limit
	relay.maxTokens = float64(maxConcurrent)
	relay.refillRate = float64(maxConcurrent) // 1 token per concurrent send per minute
	relay.tokens = relay.maxTokens
	relay.lastRefill = time.Now()

	return relay
}

// Start begins the SMTP relay control channel.
func (r *smtpRelay) Start() error {
	log.Printf("starting SMTP relay control channel on port %s", r.controlPort)

	mux := http.NewServeMux()

	// Control endpoint for sending emails.
	mux.HandleFunc("/relay/send", r.handleSend)

	// Certificate update endpoint.
	mux.HandleFunc("/relay/cert", r.handleCertUpdate)

	// Health endpoint.
	mux.HandleFunc("/relay/health", r.handleHealth)

	r.httpServer = &http.Server{
		Addr:    ":" + r.controlPort,
		Handler: mux,
		BaseContext: func(net.Listener) context.Context {
			return context.WithValue(context.Background(), "relay", r)
		},
	}

	// Start token bucket refiller
	go r.refillTokens()

	// Start worker pool
	maxConcurrent := r.getMaxConcurrent()
	for i := 0; i < maxConcurrent; i++ {
		go r.worker(i)
	}

	// Start server in background
	go func() {
		if err := r.httpServer.ListenAndServe(); err != http.ErrServerClosed {
			log.Printf("SMTP relay error: %v", err)
		}
	}()

	return nil
}

// Stop shuts down the SMTP relay control channel.
func (r *smtpRelay) Stop() error {
	r.shutdownOnce.Do(func() {
		close(r.shutdownCh)
		close(r.workerCh) // Unblock workers waiting on channel.

		if r.httpServer != nil {
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()
			_ = r.httpServer.Shutdown(ctx)
		}
	})

	return nil
}

// handleSend handles POST requests to /relay/send with email commands.
func (r *smtpRelay) handleSend(w http.ResponseWriter, req *http.Request) {
	if req.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Authenticate request
	authToken := req.Header.Get("Authorization")
	if !r.authenticate(authToken) {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	// Parse email command
	var cmd EmailSendCommand
	if err := json.NewDecoder(req.Body).Decode(&cmd); err != nil {
		log.Printf("failed to decode email command: %v", err)
		http.Error(w, "invalid JSON", http.StatusBadRequest)
		return
	}

	// Validate required fields
	if err := r.validateCommand(&cmd); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	// Store credentials for this send
	if cmd.SMTPUsername != "" || cmd.SMTPPassword != "" {
		r.credentialStore.Store(cmd.SMTPHost, cmd.SMTPUsername, cmd.SMTPPassword)
		defer r.credentialStore.Clear(cmd.SMTPHost)
	}

	// Enqueue command for processing
	r.workerCh <- &cmd

	// Return immediate acknowledgment
	result := EmailSendResult{
		ID:          cmd.ID,
		Success:     true,
		SMTPCode:    202,
		SMTPMessage: "command accepted",
		DeliveredAt: time.Now().Format(time.RFC3339),
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(result); err != nil {
		log.Printf("failed to encode response: %v", err)
	}
}

// handleHealth handles health check requests.
func (r *smtpRelay) handleHealth(w http.ResponseWriter, req *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	fmt.Fprintf(w, `{"status":"ok","controlPort":%q}`, r.controlPort)
}

// handleCertUpdate handles POST /relay/cert to update the TLS certificate at runtime.
func (r *smtpRelay) handleCertUpdate(w http.ResponseWriter, req *http.Request) {
	if req.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	if !r.authenticate(req.Header.Get("Authorization")) {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	if r.certMgr == nil {
		http.Error(w, "certificate manager not configured", http.StatusInternalServerError)
		return
	}

	var body struct {
		CertPEM string `json:"cert_pem"`
		KeyPEM  string `json:"key_pem"`
	}
	if err := json.NewDecoder(req.Body).Decode(&body); err != nil {
		http.Error(w, "invalid JSON", http.StatusBadRequest)
		return
	}
	if body.CertPEM == "" || body.KeyPEM == "" {
		http.Error(w, "cert_pem and key_pem are required", http.StatusBadRequest)
		return
	}

	if err := r.certMgr.UpdateFromPEM([]byte(body.CertPEM), []byte(body.KeyPEM)); err != nil {
		log.Printf("cert update failed: %v", err)
		http.Error(w, fmt.Sprintf("certificate update failed: %v", err), http.StatusBadRequest)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	fmt.Fprintf(w, `{"status":"updated"}`)
}

// authenticate validates the Bearer token in the Authorization header.
func (r *smtpRelay) authenticate(authHeader string) bool {
	if r.authToken == "" {
		return true // no auth required
	}

	const bearerPrefix = "Bearer "
	if !strings.HasPrefix(authHeader, bearerPrefix) {
		return false
	}

	token := strings.TrimPrefix(authHeader, bearerPrefix)
	return token == r.authToken
}

// validateCommand validates an email send command.
func (r *smtpRelay) validateCommand(cmd *EmailSendCommand) error {
	if cmd.ID == "" {
		return fmt.Errorf("id is required")
	}
	if cmd.Recipient == "" {
		return fmt.Errorf("recipient is required")
	}
	if cmd.SMTPHost == "" {
		return fmt.Errorf("smtp_host is required")
	}
	if cmd.Subject == "" {
		return fmt.Errorf("subject is required")
	}
	if cmd.BodyHTML == "" && cmd.BodyText == "" {
		return fmt.Errorf("body_html or body_text is required")
	}
	if cmd.SMTPPort == 0 {
		cmd.SMTPPort = 587 // default port
	}
	if cmd.MaxConcurrent == 0 {
		cmd.MaxConcurrent = 5 // default
	}
	if cmd.RateLimit == 0 {
		cmd.RateLimit = 0 // no limit
	}
	return nil
}

// worker is a worker goroutine that processes email commands.
func (r *smtpRelay) worker(id int) {
	log.Printf("smtp worker %d started", id)

	for cmd := range r.workerCh {
		r.processCommand(cmd)
	}
}

// processCommand processes an email send command.
func (r *smtpRelay) processCommand(cmd *EmailSendCommand) {
	log.Printf("processing email command %s", cmd.ID)

	// Apply rate limiting if configured
	if cmd.RateLimit > 0 {
		r.rateMu.Lock()
		if r.tokens < 1 {
			// Wait for tokens to refill
			needed := 1 - r.tokens
			waitMs := int(60000 * needed / r.refillRate)
			r.rateMu.Unlock()
			time.Sleep(time.Duration(waitMs) * time.Millisecond)
			r.rateMu.Lock()
		}
		r.tokens--
		r.rateMu.Unlock()
	}

	// Perform SMTP delivery
	result := r.sendEmail(cmd)

	// Send log entry to framework
	go r.logDelivery(cmd, result)

	// If send failed, include error in response (if response channel exists)
	if !result.Success && result.ErrorMessage != "" {
		log.Printf("email %s delivery failed: %s", cmd.ID, result.ErrorMessage)
	}
}

// sendEmail sends an email to the specified SMTP server.
func (r *smtpRelay) sendEmail(cmd *EmailSendCommand) *EmailSendResult {
	result := &EmailSendResult{
		ID:          cmd.ID,
		DeliveredAt: time.Now().Format(time.RFC3339),
	}

	// Get sanitization configuration for this email
	sanitizeConfig := getSanitizationConfig(cmd)

	// Connect to SMTP server
	smtpAddr := fmt.Sprintf("%s:%d", cmd.SMTPHost, cmd.SMTPPort)

	var client *smtp.Client
	var err error

	switch cmd.SMTPPort {
	case 465:
		// Implicit TLS for port 465
		client, err = r.connectImplicitTLS(smtpAddr)
	default:
		// STARTTLS for port 587 or port 25
		client, err = r.connectWithSTARTTLS(smtpAddr)
	}

	if err != nil {
		result.Success = false
		result.SMTPCode = 0
		result.SMTPMessage = err.Error()
		result.ErrorMessage = fmt.Sprintf("connection failed: %v", err)
		return result
	}
	defer client.Close()

	// If we have credentials for this host, use them
	if creds := r.credentialStore.Load(cmd.SMTPHost); creds != nil {
		auth := smtp.PlainAuth("", creds.Username, string(creds.Password), cmd.SMTPHost)
		if err := client.Auth(auth); err != nil {
			result.Success = false
			result.SMTPCode = 500
			result.SMTPMessage = err.Error()
			result.ErrorMessage = fmt.Sprintf("authentication failed: %v", err)
			return result
		}
	}

	// Set envelope sender
	if err := client.Mail(cmd.EnvelopeSender); err != nil {
		result.Success = false
		result.SMTPCode = 500
		result.SMTPMessage = err.Error()
		result.ErrorMessage = fmt.Sprintf("MAIL FROM failed: %v", err)
		return result
	}

	// Set envelope recipient
	if err := client.Rcpt(cmd.Recipient); err != nil {
		result.Success = false
		result.SMTPCode = 500
		result.SMTPMessage = err.Error()
		result.ErrorMessage = fmt.Sprintf("RCPT TO failed: %v", err)
		return result
	}

	// Send DATA (email body)
	body, err := r.buildMIMEMessage(cmd)
	if err != nil {
		result.Success = false
		result.SMTPCode = 500
		result.SMTPMessage = err.Error()
		result.ErrorMessage = fmt.Sprintf("body construction failed: %v", err)
		return result
	}

	// Sanitize headers before sending
	sanitizedBody, err := sanitizeHeaders(body, sanitizeConfig)
	if err != nil {
		result.Success = false
		result.SMTPCode = 500
		result.SMTPMessage = err.Error()
		result.ErrorMessage = fmt.Sprintf("header sanitization failed: %v", err)
		return result
	}

	w, err := client.Data()
	if err != nil {
		result.Success = false
		result.SMTPCode = 500
		result.SMTPMessage = err.Error()
		result.ErrorMessage = fmt.Sprintf("DATA failed: %v", err)
		return result
	}

	if _, err := w.Write(sanitizedBody); err != nil {
		result.Success = false
		result.SMTPCode = 500
		result.SMTPMessage = err.Error()
		result.ErrorMessage = fmt.Sprintf("data write failed: %v", err)
		return result
	}

	if err := w.Close(); err != nil {
		result.Success = false
		result.SMTPCode = 500
		result.SMTPMessage = err.Error()
		result.ErrorMessage = fmt.Sprintf("data close failed: %v", err)
		return result
	}

	// Get final response code from DATA
	if err := client.Quit(); err != nil {
		result.Success = false
		result.SMTPCode = 500
		result.SMTPMessage = err.Error()
		result.ErrorMessage = fmt.Sprintf("QUIT failed: %v", err)
		return result
	}

	result.Success = true
	result.SMTPCode = 250
	result.SMTPMessage = "delivery complete"

	return result
}

// getSanitizationConfig builds a sanitization configuration from the email command.
// The command's bool fields act as explicit overrides — when false (default), the
// maximally-sanitizing default (true) is used. When the command explicitly sets a
// field to false, we honour that by checking the command struct directly.
func getSanitizationConfig(cmd *EmailSendCommand) SanitizationConfig {
	// Derive synthetic domain from envelope sender if available.
	syntheticDomain := cmd.SMTPHost
	if cmd.EnvelopeSender != "" {
		if idx := strings.Index(cmd.EnvelopeSender, "@"); idx != -1 {
			syntheticDomain = cmd.EnvelopeSender[idx+1:]
		}
	}

	return SanitizationConfig{
		StripXHeaders:           true,
		RewriteMessageID:        true,
		StripReceived:           true,
		SyntheticReceivedDomain: syntheticDomain,
	}
}

// sanitizeHeaders applies header sanitization to email bytes.
func sanitizeHeaders(email []byte, config SanitizationConfig) ([]byte, error) {
	sanitizer := NewSanitizer(config)
	return sanitizer.SanitizeHeaders(email)
}

// connectImplicitTLS connects to an SMTP server using implicit TLS (port 465).
func (r *smtpRelay) connectImplicitTLS(addr string) (*smtp.Client, error) {
	// Create TLS config
	tlsConfig := &tls.Config{
		ServerName:         "",
		InsecureSkipVerify: false,
		MinVersion:         tls.VersionTLS12,
	}

	// Dial with TLS
	conn, err := tls.Dial("tcp", addr, tlsConfig)
	if err != nil {
		return nil, fmt.Errorf("tls dial: %w", err)
	}

	// Create SMTP client
	client, err := smtp.NewClient(conn, addr)
	if err != nil {
		conn.Close()
		return nil, fmt.Errorf("smtp client: %w", err)
	}

	return client, nil
}

// connectWithSTARTTLS connects to an SMTP server with STARTTLS (port 587 or 25).
func (r *smtpRelay) connectWithSTARTTLS(addr string) (*smtp.Client, error) {
	// Dial without TLS first
	conn, err := net.Dial("tcp", addr)
	if err != nil {
		return nil, fmt.Errorf("dial: %w", err)
	}

	client, err := smtp.NewClient(conn, addr)
	if err != nil {
		conn.Close()
		return nil, fmt.Errorf("smtp client: %w", err)
	}

	// Check if STARTTLS is supported
	if ok, _ := client.Extension("STARTTLS"); ok {
		tlsConfig := &tls.Config{
			ServerName:         "",
			InsecureSkipVerify: false,
			MinVersion:         tls.VersionTLS12,
		}
		if err := client.StartTLS(tlsConfig); err != nil {
			return nil, fmt.Errorf("starttls: %w", err)
		}
	}

	return client, nil
}

// buildMIMEMessage builds a MIME-formatted email message.
func (r *smtpRelay) buildMIMEMessage(cmd *EmailSendCommand) ([]byte, error) {
	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)

	// Set content type for multipart
	buf.WriteString(fmt.Sprintf("From: %s\r\n", cmd.EnvelopeSender))
	buf.WriteString(fmt.Sprintf("To: %s\r\n", cmd.Recipient))
	buf.WriteString(fmt.Sprintf("Subject: %s\r\n", cmd.Subject))
	buf.WriteString(fmt.Sprintf("Date: %s\r\n", time.Now().Format(time.RFC1123Z)))
	buf.WriteString(fmt.Sprintf("Message-ID: <%s@%s>\r\n", cmd.ID, cmd.SMTPHost))
	buf.WriteString("MIME-Version: 1.0\r\n")
	buf.WriteString(fmt.Sprintf("Content-Type: multipart/alternative; boundary=%s\r\n", w.Boundary()))
	buf.WriteString("\r\n")

	// Write text part if available
	if cmd.BodyText != "" {
		buf.WriteString(fmt.Sprintf("--%s\r\n", w.Boundary()))
		buf.WriteString("Content-Type: text/plain; charset=UTF-8\r\n")
		buf.WriteString("Content-Transfer-Encoding: quoted-printable\r\n")
		buf.WriteString("\r\n")
		buf.WriteString(cmd.BodyText)
		buf.WriteString("\r\n")
	}

	// Write HTML part
	buf.WriteString(fmt.Sprintf("--%s\r\n", w.Boundary()))
	buf.WriteString("Content-Type: text/html; charset=UTF-8\r\n")
	buf.WriteString("Content-Transfer-Encoding: quoted-printable\r\n")
	buf.WriteString("\r\n")
	buf.WriteString(cmd.BodyHTML)
	buf.WriteString("\r\n")

	// Close multipart
	buf.WriteString(fmt.Sprintf("--%s--\r\n", w.Boundary()))
	buf.WriteString("\r\n")

	return buf.Bytes(), nil
}

// logDelivery sends a log entry to the framework after each send.
func (r *smtpRelay) logDelivery(cmd *EmailSendCommand, result *EmailSendResult) {
	// Determine status string
	status := "delivered"
	if !result.Success {
		status = "failed"
		if result.SMTPCode >= 500 {
			status = "rejected"
		}
	}

	// Build log entry
	entry := SMTPLogEntry{
		EndpointID: endpointID,
		CampaignID: cmd.CampaignID,
		CommandID:  cmd.ID,
		Sender:     cmd.EnvelopeSender,
		Recipient:  cmd.Recipient,
		Status:     status,
		SMTPCode:   result.SMTPCode,
		SMTPHost:   cmd.SMTPHost,
		Timestamp:  time.Now().Format(time.RFC3339),
	}

	// Send to framework
	if err := r.sendLogToFramework(entry); err != nil {
		log.Printf("failed to send SMTP log: %v", err)
	}
}

// sendLogToFramework sends an SMTP log entry to the framework.
func (r *smtpRelay) sendLogToFramework(entry SMTPLogEntry) error {
	if r.frameworkHost == "" {
		return nil // framework not configured
	}

	// Build URL
	url := fmt.Sprintf("http://%s/endpoint-data/smtp-log", r.frameworkHost)

	// Marshal entry
	body, err := json.Marshal(entry)
	if err != nil {
		return fmt.Errorf("marshal entry: %w", err)
	}

	// Create request
	req, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", smtpUserAgent)

	// Add auth token if available
	if r.authToken != "" {
		req.Header.Set("Authorization", "Bearer "+r.authToken)
	}

	// Send request
	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("send request: %w", err)
	}
	defer resp.Body.Close()

	// Read response body (ignore errors for logging)
	_, _ = io.ReadAll(resp.Body)

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		return fmt.Errorf("framework returned status %d", resp.StatusCode)
	}

	return nil
}

// refillTokens refills the token bucket for rate limiting.
func (r *smtpRelay) refillTokens() {
	ticker := time.NewTicker(1 * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			r.rateMu.Lock()
			r.tokens += r.refillRate
			if r.tokens > r.maxTokens {
				r.tokens = r.maxTokens
			}
			r.lastRefill = time.Now()
			r.rateMu.Unlock()
		case <-r.shutdownCh:
			return
		}
	}
}

// getMaxConcurrent returns the max concurrent sends.
func (r *smtpRelay) getMaxConcurrent() int {
	return 5 // Default max concurrent connections
}

// Store adds credentials to the store.
func (cs *credentialStore) Store(host, username, password string) {
	cs.mu.Lock()
	defer cs.mu.Unlock()

	cs.credentials[host] = &smtpCredentials{
		Username: username,
		Password: []byte(password),
	}
}

// Load retrieves credentials for a host.
func (cs *credentialStore) Load(host string) *smtpCredentials {
	cs.mu.RLock()
	defer cs.mu.RUnlock()

	return cs.credentials[host]
}

// Clear removes credentials for a host and clears sensitive data from memory.
func (cs *credentialStore) Clear(host string) {
	cs.mu.Lock()
	defer cs.mu.Unlock()

	if creds, ok := cs.credentials[host]; ok {
		// Clear password from memory (best effort)
		if len(creds.Password) > 0 {
			for i := range creds.Password {
				creds.Password[i] = 0
			}
		}
		delete(cs.credentials, host)
	}
}

// newCredentialStore creates a new credential store with an initialized map.
func newCredentialStore() *credentialStore {
	return &credentialStore{
		credentials: make(map[string]*smtpCredentials),
	}
}

// ClearAll clears all credentials from the store.
func (cs *credentialStore) ClearAll() {
	cs.mu.Lock()
	defer cs.mu.Unlock()

	for host, creds := range cs.credentials {
		// Clear password
		if len(creds.Password) > 0 {
			for i := range creds.Password {
				creds.Password[i] = 0
			}
		}
		delete(cs.credentials, host)
	}
}
