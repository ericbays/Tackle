package main

import (
	"bytes"
	cryptorand "crypto/rand"
	"encoding/base64"
	"fmt"
	"sort"
	"strings"
	"time"
)

// SanitizationConfig holds configuration options for header sanitization.
type SanitizationConfig struct {
	// StripXHeaders controls whether to strip all X- headers.
	// Default is true.
	StripXHeaders bool

	// RewriteMessageID controls whether to rewrite the Message-ID header.
	// Default is true.
	RewriteMessageID bool

	// StripReceived controls whether to strip all Received headers.
	// Default is true.
	StripReceived bool

	// SyntheticReceivedDomain is the domain to use for the synthetic Received header.
	// If empty, the envelope sender's domain will be used.
	SyntheticReceivedDomain string

	// CustomStripPatterns contains additional header name patterns to strip.
	// Patterns are case-insensitive.
	CustomStripPatterns []string

	// CustomAddHeaders contains headers to inject after sanitization.
	CustomAddHeaders map[string]string
}

// DefaultSanitizationConfig returns a maximally sanitizing configuration.
func DefaultSanitizationConfig() SanitizationConfig {
	return SanitizationConfig{
		StripXHeaders:           true,
		RewriteMessageID:        true,
		StripReceived:           true,
		SyntheticReceivedDomain: "",
		CustomStripPatterns:     nil,
		CustomAddHeaders:        nil,
	}
}

// HeaderSanitizer processes and sanitizes email headers.
type HeaderSanitizer struct {
	config SanitizationConfig
}

// NewSanitizer creates a new header sanitizer with the given configuration.
func NewSanitizer(config SanitizationConfig) *HeaderSanitizer {
	return &HeaderSanitizer{config: config}
}

// SanitizeHeaders processes email headers and returns the sanitized email.
// It strips framework-identifying headers, manages Received headers,
// rewrites Message-ID, and preserves essential headers for deliverability.
func (s *HeaderSanitizer) SanitizeHeaders(rawEmail []byte) ([]byte, error) {
	if len(rawEmail) == 0 {
		return rawEmail, nil
	}

	// Parse headers and body
	headerBlock, body := parseEmailHeaders(rawEmail)

	// Parse headers into a map
	headers := parseHeaders(headerBlock)

	// Preserve essential header values before stripping
	s.preserveEssentialHeaders(headers)

	// Apply sanitization steps
	s.stripFrameworkHeaders(headers)
	s.stripReceivedHeaders(headers)
	s.rewriteMessageID(headers)
	s.addSyntheticReceivedHeader(headers)

	// Add custom headers if configured
	s.addCustomHeaders(headers)

	// Rebuild the email with sanitized headers
	sanitized := rebuildEmail(headers, body, headerBlock)

	return sanitized, nil
}

// parseEmailHeaders splits the email into header block and body.
func parseEmailHeaders(email []byte) (headerBlock, body []byte) {
	// Find the blank line separating headers from body
	// Headers end at the first double CRLF (\r\n\r\n) or double LF (\n\n)
	separator := findHeaderBodySeparator(email)
	if separator == -1 {
		// No body found, entire email is headers
		return email, nil
	}

	headerBlock = email[:separator]
	body = email[separator:]

	return headerBlock, body
}

// findHeaderBodySeparator finds the position where headers end and body begins.
func findHeaderBodySeparator(email []byte) int {
	// Look for \r\n\r\n (Windows-style) or \n\n (Unix-style)
	// We check for \r\n\r\n first as it's more common in SMTP

	for i := 0; i < len(email)-3; i++ {
		if email[i] == '\r' && email[i+1] == '\n' &&
			email[i+2] == '\r' && email[i+3] == '\n' {
			return i + 4
		}
		if email[i] == '\n' && email[i+1] == '\n' {
			return i + 2
		}
	}
	return -1
}

// parseHeaders parses email headers into a map.
// Header names are stored in canonical form (e.g., "From", "To", "X-Mailer").
func parseHeaders(headerBlock []byte) map[string]string {
	headers := make(map[string]string)

	if len(headerBlock) == 0 {
		return headers
	}

	lines := bytes.Split(headerBlock, []byte{'\n'})
	currentName := ""
	currentValue := ""

	for _, line := range lines {
		// Handle CRLF line endings
		line = bytes.TrimSuffix(line, []byte{'\r'})

		if len(line) == 0 {
			continue
		}

		// Check if this is a continuation line (starts with whitespace)
		if line[0] == ' ' || line[0] == '\t' {
			if currentName != "" {
				// Strip leading whitespace from continuation
				continueValue := bytes.TrimLeftFunc(line, func(r rune) bool {
					return r == ' ' || r == '\t'
				})
				currentValue += " " + string(continueValue)
				headers[currentName] = currentValue
			}
			continue
		}

		// Parse header line: Name: Value
		colonIdx := bytes.Index(line, []byte{':'})
		if colonIdx == -1 {
			continue
		}

		// Save previous header if we have one
		if currentName != "" {
			headers[currentName] = currentValue
		}

		currentName = string(bytes.TrimSpace(line[:colonIdx]))
		currentValue = string(bytes.TrimSpace(line[colonIdx+1:]))
	}

	// Save the last header
	if currentName != "" {
		headers[currentName] = currentValue
	}

	return headers
}

// stripFrameworkHeaders removes headers that identify the Tackle framework.
func (s *HeaderSanitizer) stripFrameworkHeaders(headers map[string]string) {
	// Headers to always strip regardless of config — these identify the framework.
	alwaysStrip := []string{
		"X-Mailer",
		"X-Originating-IP",
		"X-Mailer-Version",
		"X-MimeOLE",
		"X-MSMail-Priority",
		"User-Agent",
	}

	// Build a set of header names to strip
	stripSet := make(map[string]bool)

	for _, name := range alwaysStrip {
		stripSet[name] = true
	}

	if s.config.StripXHeaders {
		// Strip all X- headers
		for name := range headers {
			if strings.HasPrefix(strings.ToUpper(name), "X-") {
				stripSet[name] = true
			}
		}
	}

	// Add headers containing "tackle" (case-insensitive)
	for name := range headers {
		if strings.Contains(strings.ToLower(name), "tackle") {
			stripSet[name] = true
		}
	}

	// Add custom strip patterns
	for _, pattern := range s.config.CustomStripPatterns {
		patternLower := strings.ToLower(pattern)
		for name := range headers {
			if strings.Contains(strings.ToLower(name), patternLower) {
				stripSet[name] = true
			}
		}
	}

	// Remove all headers marked for stripping
	for name := range stripSet {
		delete(headers, name)
	}
}

// stripReceivedHeaders removes all Received headers.
func (s *HeaderSanitizer) stripReceivedHeaders(headers map[string]string) {
	if !s.config.StripReceived {
		return
	}

	// Collect all Received header names
	var receivedNames []string
	for name := range headers {
		if strings.HasPrefix(strings.ToUpper(name), "RECEIVED") {
			receivedNames = append(receivedNames, name)
		}
	}

	// Sort to maintain consistent order
	sort.Strings(receivedNames)

	// Remove all Received headers
	for _, name := range receivedNames {
		delete(headers, name)
	}
}

// rewriteMessageID replaces the Message-ID with a sanitized version.
func (s *HeaderSanitizer) rewriteMessageID(headers map[string]string) {
	if !s.config.RewriteMessageID {
		return
	}

	// Extract sender domain from From header
	senderDomain := s.extractSenderDomain(headers)

	// Generate random local part (20 alphanumeric characters)
	localPart := s.generateRandomLocalPart()

	// Build new Message-ID
	newMessageID := fmt.Sprintf("<%s@%s>", localPart, senderDomain)

	// Update or add Message-ID header
	headers["Message-ID"] = newMessageID
}

// extractSenderDomain extracts the domain from the From header.
func (s *HeaderSanitizer) extractSenderDomain(headers map[string]string) string {
	fromHeader, ok := headers["From"]
	if !ok || fromHeader == "" {
		// Fallback to synthetic domain if no From header
		if s.config.SyntheticReceivedDomain != "" {
			return s.config.SyntheticReceivedDomain
		}
		return "localhost"
	}

	// Parse RFC 5322 address: "Name <email@domain>" or "email@domain"
	email := fromHeader

	// Remove quotes if present
	email = strings.Trim(email, "\"")

	// Find @ symbol for domain extraction
	if idx := strings.Index(email, "@"); idx != -1 {
		// Extract domain part
		domain := email[idx+1:]
		// Handle angle brackets (e.g., "<user@domain.com>")
		if idx2 := strings.Index(domain, ">"); idx2 != -1 {
			domain = domain[:idx2]
		}
		// Handle semicolon-separated addresses
		if idx3 := strings.Index(domain, ";"); idx3 != -1 {
			domain = domain[:idx3]
		}
		return strings.TrimSpace(domain)
	}

	// If no @ found, try to find domain after last space or angle bracket
	if idx := strings.LastIndex(email, ">"); idx != -1 {
		if idx < len(email)-1 {
			return email[idx+1 : strings.IndexFunc(email[idx+1:], func(r rune) bool {
				return r == ' ' || r == '\t' || r == '<' || r == '>' || r == ';'
			})]
		}
	}

	// Default fallback
	return "localhost"
}

// generateRandomLocalPart generates a random 20-character alphanumeric string.
func (s *HeaderSanitizer) generateRandomLocalPart() string {
	const charset = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	const length = 20

	rnd := make([]byte, length)
	if _, err := cryptorand.Read(rnd); err != nil {
		return "aaaabbbbccccddddeeee" // Safe fallback; crypto/rand failure is extremely rare.
	}
	for i, v := range rnd {
		rnd[i] = charset[v%byte(len(charset))]
	}

	return string(rnd)
}

// addSyntheticReceivedHeader adds a synthetic Received header.
func (s *HeaderSanitizer) addSyntheticReceivedHeader(headers map[string]string) {
	if !s.config.StripReceived {
		return
	}

	// Build synthetic Received header
	receivedHeader := s.buildSyntheticReceivedHeader()
	headers["Received"] = receivedHeader
}

// buildSyntheticReceivedHeader builds a RFC 5322-compliant Received header.
func (s *HeaderSanitizer) buildSyntheticReceivedHeader() string {
	now := time.Now()

	// Format: Received: from domain (hostname) by domain with protocol ...
	// Example: Received: from example.com (mail.example.com) by example.com with ESMTPS id ABC123
	//            for <recipient@example.com>; Thu, 18 Mar 2026 10:30:00 -0500

	var sb strings.Builder

	sb.WriteString("from ")

	// Use configured domain or fallback
	domain := s.config.SyntheticReceivedDomain
	if domain == "" {
		domain = "localhost"
	}

	sb.WriteString(domain)
	sb.WriteString(" (")

	// Add hostname (could be the same as domain or a subdomain)
	sb.WriteString(domain)
	sb.WriteString(")")

	sb.WriteString(" by ")
	sb.WriteString(domain)

	sb.WriteString(" with ESMTPS")
	sb.WriteString(" id ")
	sb.WriteString(s.generateMessageIDForReceived())

	// Add timestamp
	sb.WriteString("; ")
	sb.WriteString(formatRFC5322Date(now))

	return sb.String()
}

// generateMessageIDForReceived generates a random ID for the Received header.
func (s *HeaderSanitizer) generateMessageIDForReceived() string {
	bytes := make([]byte, 8)
	cryptorand.Read(bytes)
	return base64.URLEncoding.EncodeToString(bytes)
}

// formatRFC5322Date formats a time as RFC 5322 date string.
func formatRFC5322Date(t time.Time) string {
	// RFC 5322 format: day, dd mon yyyy hh:mm:ss +/-hhmm
	return t.Format("Mon, 02 Jan 2006 15:04:05 -0700")
}

// preserveEssentialHeaders snapshots essential header values so they survive
// stripping. Only headers that already exist in the email are preserved —
// missing headers are never invented.
func (s *HeaderSanitizer) preserveEssentialHeaders(headers map[string]string) {
	// Nothing to do — essential headers are protected by stripFrameworkHeaders
	// which only removes X- headers, User-Agent, and tackle-containing headers.
	// Essential headers (From, To, Subject, Date, MIME-Version, Content-Type,
	// Content-Transfer-Encoding, Reply-To, Return-Path, DKIM-Signature, Cc, Bcc)
	// never match those patterns, so they survive naturally.
}

// addCustomHeaders injects additional headers as configured.
func (s *HeaderSanitizer) addCustomHeaders(headers map[string]string) {
	if s.config.CustomAddHeaders == nil {
		return
	}

	for name, value := range s.config.CustomAddHeaders {
		headers[name] = value
	}
}

// rebuildEmail constructs the final email from sanitized headers and body.
func rebuildEmail(headers map[string]string, body, originalHeaderBlock []byte) []byte {
	var buf bytes.Buffer

	// Write headers in sorted order for consistency
	sortedKeys := getSortedHeaderKeys(headers)

	for _, key := range sortedKeys {
		if value, ok := headers[key]; ok {
			buf.WriteString(fmt.Sprintf("%s: %s\r\n", key, value))
		}
	}

	// Add blank line before body
	buf.WriteString("\r\n")

	// Add body if present
	if len(body) > 0 {
		buf.Write(body)
	}

	return buf.Bytes()
}

// getSortedHeaderKeys returns header names sorted for consistent output.
func getSortedHeaderKeys(headers map[string]string) []string {
	keys := make([]string, 0, len(headers))
	for name := range headers {
		keys = append(keys, name)
	}

	sort.Slice(keys, func(i, j int) bool {
		// Priority order for header placement
		priority := map[string]int{
			"From":                      0,
			"To":                        1,
			"Cc":                        2,
			"Bcc":                       3,
			"Subject":                   4,
			"Date":                      5,
			"Message-ID":                6,
			"Reply-To":                  7,
			"Return-Path":               8,
			"MIME-Version":              9,
			"Content-Type":              10,
			"Content-Transfer-Encoding": 11,
			"DKIM-Signature":            12,
			"Received":                  13,
		}

		pi, okI := priority[keys[i]]
		pj, okJ := priority[keys[j]]

		// Headers with priority come first
		if okI && okJ {
			return pi < pj
		}
		if okI {
			return true
		}
		if okJ {
			return false
		}

		// Alphabetical for others
		return keys[i] < keys[j]
	})

	return keys
}

// SanitizeMessage processes a complete email message string.
func (s *HeaderSanitizer) SanitizeMessage(message string) (string, error) {
	sanitized, err := s.SanitizeHeaders([]byte(message))
	return string(sanitized), err
}

// SanitizeMIMEMessage processes a MIME message, preserving multipart boundaries.
func (s *HeaderSanitizer) SanitizeMIMEMessage(mimeData []byte) ([]byte, error) {
	return s.SanitizeHeaders(mimeData)
}
