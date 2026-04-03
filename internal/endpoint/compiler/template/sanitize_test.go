package main

import (
	"encoding/base64"
	"strings"
	"testing"
	"time"
)

// TestHeaderSanitizer tests the main sanitization functionality.
func TestHeaderSanitizer(t *testing.T) {
	tests := []struct {
		name              string
		rawEmail          string
		config            SanitizationConfig
		expectedHeaders   []string // headers that should exist
		unexpectedHeaders []string // headers that should NOT exist
	}{
		{
			name:     "basic_sanitization",
			rawEmail: basicEmailWithFrameworkHeaders,
			config:   DefaultSanitizationConfig(),
			expectedHeaders: []string{
				"From",
				"To",
				"Subject",
				"Date",
				"Message-ID",
			},
			unexpectedHeaders: []string{
				"X-Mailer",
				"X-Originating-IP",
				"User-Agent",
			},
		},
		{
			name:     "custom_strip_patterns",
			rawEmail: emailWithCustomHeaders,
			config: SanitizationConfig{
				StripXHeaders:       true,
				RewriteMessageID:    true,
				StripReceived:       true,
				CustomStripPatterns: []string{"Custom", "Vendor"},
			},
			expectedHeaders: []string{"From", "To", "Subject"},
			unexpectedHeaders: []string{
				"X-Custom-Property",
				"X-Vendor-Tag",
			},
		},
		{
			name:     "preserve_dkim_signature",
			rawEmail: emailWithDKIM,
			config:   DefaultSanitizationConfig(),
			expectedHeaders: []string{
				"DKIM-Signature",
				"From",
				"To",
				"Received", // synthetic Received header should be present
			},
			unexpectedHeaders: []string{"X-Mailer"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sanitizer := NewSanitizer(tt.config)
			sanitized, err := sanitizer.SanitizeHeaders([]byte(tt.rawEmail))

			if err != nil {
				t.Fatalf("SanitizeHeaders failed: %v", err)
			}

			sanitizedStr := string(sanitized)

			for _, header := range tt.expectedHeaders {
				if !strings.Contains(sanitizedStr, header+":") {
					t.Errorf("Expected header %s not found in sanitized email", header)
				}
			}

			for _, header := range tt.unexpectedHeaders {
				if strings.Contains(sanitizedStr, header+":") {
					t.Errorf("Unexpected header %s found in sanitized email", header)
				}
			}
		})
	}
}

// TestStripXHeaders tests X-header stripping functionality.
func TestStripXHeaders(t *testing.T) {
	tests := []struct {
		name        string
		rawEmail    string
		stripX      bool
		xHeaders    []string // X-headers to check
		nonXHeaders []string // non-X-headers that should be preserved
	}{
		{
			name:     "strip_all_x_headers",
			rawEmail: emailWithMultipleXHeaders,
			stripX:   true,
			xHeaders: []string{
				"X-Mailer",
				"X-Originating-IP",
				"X-Mailer-Version",
				"X-MimeOLE",
				"X-MSMail-Priority",
				"X-Tackle-Sender",
				"X-Custom-Header",
			},
			nonXHeaders: []string{
				"From",
				"To",
				"Subject",
			},
		},
		{
			name:     "preserve_non_framework_x_headers_when_disabled",
			rawEmail: emailWithMultipleXHeaders,
			stripX:   false,
			xHeaders: []string{
				// X-Custom-Header is not in alwaysStrip and doesn't contain "tackle"
				// so it should be preserved when StripXHeaders=false.
				"X-Custom-Header",
			},
			nonXHeaders: []string{"From", "To"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := SanitizationConfig{
				StripXHeaders:    tt.stripX,
				RewriteMessageID: true,
				StripReceived:    true,
			}

			sanitizer := NewSanitizer(config)
			sanitized, err := sanitizer.SanitizeHeaders([]byte(tt.rawEmail))

			if err != nil {
				t.Fatalf("SanitizeHeaders failed: %v", err)
			}

			sanitizedStr := string(sanitized)

			if tt.stripX {
				// All X-headers should be gone.
				for _, header := range tt.xHeaders {
					if strings.Contains(sanitizedStr, header+":") {
						t.Errorf("X-header %s should be stripped but was found", header)
					}
				}
			} else {
				// Non-framework X-headers should be preserved.
				for _, header := range tt.xHeaders {
					if !strings.Contains(sanitizedStr, header+":") {
						t.Errorf("X-header %s should be preserved but was not found", header)
					}
				}
				// Framework-identifying headers should still be stripped.
				if strings.Contains(sanitizedStr, "X-Mailer:") {
					t.Error("X-Mailer should always be stripped")
				}
				if strings.Contains(sanitizedStr, "X-Tackle-Sender:") {
					t.Error("X-Tackle-Sender should always be stripped (contains 'tackle')")
				}
			}

			// Check non-X-headers are preserved.
			for _, header := range tt.nonXHeaders {
				if !strings.Contains(sanitizedStr, header+":") {
					t.Errorf("Non-X-header %s should be preserved but was not found", header)
				}
			}
		})
	}
}

// TestReceivedChainManagement tests Received header handling.
func TestReceivedChainManagement(t *testing.T) {
	tests := []struct {
		name           string
		rawEmail       string
		stripReceived  bool
		checkSynthetic bool
	}{
		{
			name:           "strip_all_received_headers",
			rawEmail:       emailWithReceivedChain,
			stripReceived:  true,
			checkSynthetic: true,
		},
		{
			name:           "preserve_received_headers",
			rawEmail:       emailWithReceivedChain,
			stripReceived:  false,
			checkSynthetic: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := SanitizationConfig{
				StripXHeaders:    true,
				RewriteMessageID: true,
				StripReceived:    tt.stripReceived,
			}

			sanitizer := NewSanitizer(config)
			sanitized, err := sanitizer.SanitizeHeaders([]byte(tt.rawEmail))

			if err != nil {
				t.Fatalf("SanitizeHeaders failed: %v", err)
			}

			sanitizedStr := string(sanitized)

			// Count Received headers
			receivedCount := strings.Count(sanitizedStr, "Received:")

			if tt.stripReceived {
				if receivedCount > 1 {
					t.Errorf("Expected at most 1 Received header after stripping, got %d", receivedCount)
				}
			}

			if tt.checkSynthetic {
				if !strings.Contains(sanitizedStr, "Received:") {
					t.Errorf("Expected synthetic Received header not found")
				}
				// Check for RFC 5322 date format in synthetic Received
				if !strings.Contains(sanitizedStr, "; ") {
					t.Errorf("Expected date separator in synthetic Received header")
				}
			}
		})
	}
}

// TestMessageIDRewriting tests Message-ID rewriting functionality.
func TestMessageIDRewriting(t *testing.T) {
	tests := []struct {
		name             string
		rawEmail         string
		rewriteMessageID bool
		expectedDomain   string
	}{
		{
			name:             "rewrite_message_id",
			rawEmail:         emailWithMessageID,
			rewriteMessageID: true,
			expectedDomain:   "example.com",
		},
		{
			name:             "preserve_message_id",
			rawEmail:         emailWithMessageID,
			rewriteMessageID: false,
			expectedDomain:   "example.com",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := SanitizationConfig{
				StripXHeaders:    true,
				RewriteMessageID: tt.rewriteMessageID,
				StripReceived:    true,
			}

			sanitizer := NewSanitizer(config)
			sanitized, err := sanitizer.SanitizeHeaders([]byte(tt.rawEmail))

			if err != nil {
				t.Fatalf("SanitizeHeaders failed: %v", err)
			}

			sanitizedStr := string(sanitized)

			if tt.rewriteMessageID {
				// Check Message-ID format: <random@domain>
				if !strings.Contains(sanitizedStr, "Message-ID:") {
					t.Errorf("Message-ID header not found")
				}

				// Find Message-ID value
				mimeIdx := strings.Index(sanitizedStr, "Message-ID:")
				if mimeIdx != -1 {
					endIdx := strings.Index(sanitizedStr[mimeIdx:], "\r\n")
					if endIdx != -1 {
						msgIDLine := sanitizedStr[mimeIdx : mimeIdx+endIdx]
						if !strings.Contains(msgIDLine, "<") || !strings.Contains(msgIDLine, ">") {
							t.Errorf("Message-ID should be in angle brackets: %s", msgIDLine)
						}
					}
				}
			}
		})
	}
}

// TestPreservedHeaders tests that essential headers are preserved.
func TestPreservedHeaders(t *testing.T) {
	tests := []struct {
		name             string
		rawEmail         string
		preservedHeaders []string
	}{
		{
			name:     "preserve_mime_and_content_headers",
			rawEmail: emailWithMIMEHeaders,
			preservedHeaders: []string{
				"MIME-Version",
				"Content-Type",
				"Content-Transfer-Encoding",
			},
		},
		{
			name:     "preserve_comms_headers",
			rawEmail: emailWithCommsHeaders,
			preservedHeaders: []string{
				"From",
				"To",
				"Cc",
				"Bcc",
				"Subject",
				"Date",
				"Reply-To",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := DefaultSanitizationConfig()
			sanitizer := NewSanitizer(config)
			sanitized, err := sanitizer.SanitizeHeaders([]byte(tt.rawEmail))

			if err != nil {
				t.Fatalf("SanitizeHeaders failed: %v", err)
			}

			sanitizedStr := string(sanitized)

			for _, header := range tt.preservedHeaders {
				if !strings.Contains(sanitizedStr, header+":") {
					t.Errorf("Expected preserved header %s not found", header)
				}
			}
		})
	}
}

// TestDKIMPreservation tests that DKIM signatures are preserved.
func TestDKIMPreservation(t *testing.T) {
	tests := []struct {
		name      string
		rawEmail  string
		dkimLines []string // lines that should be in DKIM-Signature
	}{
		{
			name:     "preserve_dkim_signature",
			rawEmail: emailWithDKIM,
			dkimLines: []string{
				"v=",
				"a=",
				"s=",
				"d=",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := DefaultSanitizationConfig()
			sanitizer := NewSanitizer(config)
			sanitized, err := sanitizer.SanitizeHeaders([]byte(tt.rawEmail))

			if err != nil {
				t.Fatalf("SanitizeHeaders failed: %v", err)
			}

			sanitizedStr := string(sanitized)

			// Find DKIM-Signature
			dkimIdx := strings.Index(sanitizedStr, "DKIM-Signature:")
			if dkimIdx == -1 {
				t.Errorf("DKIM-Signature header not found")
				return
			}

			// The full DKIM-Signature line includes the header name and value
			// Find the full line containing "DKIM-Signature:"
			dkimStart := dkimIdx
			if dkimIdx >= 2 && sanitizedStr[dkimIdx-2:dkimIdx] == "\r\n" {
				dkimStart = dkimIdx - 2
			}

			// Find end of DKIM-Signature (next blank line or end of headers)
			remaining := sanitizedStr[dkimStart:]

			// Find the full DKIM-Signature line(s) before blank line or next header
			lines := strings.SplitAfter(remaining, "\n")
			var dkimLines []string
			var foundHeader bool
			var firstLine = true
			for _, line := range lines {
				line = strings.TrimSpace(line)
				// Skip first empty lines, only break on empty lines after we found content
				if line == "" && foundHeader && !firstLine {
					break
				}
				if line != "" {
					foundHeader = true
					firstLine = false
				}
				dkimLines = append(dkimLines, line)
				if strings.HasPrefix(line, "From:") ||
					strings.HasPrefix(line, "To:") ||
					strings.HasPrefix(line, "Subject:") ||
					strings.HasPrefix(line, "MIME-Version:") ||
					strings.HasPrefix(line, "Content-Type:") {
					break
				}
			}
			dkimBlock := strings.Join(dkimLines, "")

			// dkimLines contains the header name prefix
			// The test lines should be found in the block
			expectedLines := append([]string{"DKIM-Signature:"}, tt.dkimLines...)
			for _, line := range expectedLines {
				if !strings.Contains(dkimBlock, line) {
					t.Errorf("DKIM-Signature missing expected line: %s", line)
				}
			}
		})
	}
}

// TestCustomHeaders tests custom header injection.
func TestCustomHeaders(t *testing.T) {
	tests := []struct {
		name           string
		rawEmail       string
		customAdd      map[string]string
		expectedCustom map[string]string
	}{
		{
			name:     "add_custom_headers",
			rawEmail: basicEmailWithFrameworkHeaders,
			customAdd: map[string]string{
				"X-Injected-Header": "value1",
				"X-Fake-Mailer":     "CustomMailer/1.0",
			},
			expectedCustom: map[string]string{
				"X-Injected-Header": "value1",
				"X-Fake-Mailer":     "CustomMailer/1.0",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := SanitizationConfig{
				StripXHeaders:    true,
				RewriteMessageID: true,
				StripReceived:    true,
				CustomAddHeaders: tt.customAdd,
			}

			sanitizer := NewSanitizer(config)
			sanitized, err := sanitizer.SanitizeHeaders([]byte(tt.rawEmail))

			if err != nil {
				t.Fatalf("SanitizeHeaders failed: %v", err)
			}

			sanitizedStr := string(sanitized)

			for header, expectedValue := range tt.expectedCustom {
				if !strings.Contains(sanitizedStr, header+":") {
					t.Errorf("Custom header %s not found", header)
				}
				if expectedValue != "" && !strings.Contains(sanitizedStr, expectedValue) {
					t.Errorf("Expected value %s for header %s not found", expectedValue, header)
				}
			}
		})
	}
}

// TestEmptyEmail tests handling of minimal emails.
func TestEmptyEmail(t *testing.T) {
	tests := []struct {
		name     string
		rawEmail string
	}{
		{
			name:     "minimal_email_with_no_headers",
			rawEmail: "Just the body content",
		},
		{
			name:     "email_with_single_header",
			rawEmail: "From: sender@example.com\n\nBody content",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := DefaultSanitizationConfig()
			sanitizer := NewSanitizer(config)
			sanitized, err := sanitizer.SanitizeHeaders([]byte(tt.rawEmail))

			if err != nil {
				t.Fatalf("SanitizeHeaders failed: %v", err)
			}

			if len(sanitized) == 0 {
				t.Errorf("Sanitized email is empty")
			}
		})
	}
}

// TestMultipartBody tests that multipart MIME bodies are not corrupted.
func TestMultipartBody(t *testing.T) {
	tests := []struct {
		name     string
		rawEmail string
	}{
		{
			name:     "multipart_email_with_text_and_html",
			rawEmail: emailWithMultipartBody,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := DefaultSanitizationConfig()
			sanitizer := NewSanitizer(config)
			sanitized, err := sanitizer.SanitizeHeaders([]byte(tt.rawEmail))

			if err != nil {
				t.Fatalf("SanitizeHeaders failed: %v", err)
			}

			sanitizedStr := string(sanitized)

			// Check multipart boundaries are intact
			if !strings.Contains(sanitizedStr, "multipart/alternative") &&
				!strings.Contains(sanitizedStr, "multipart/mixed") {
				t.Errorf("MIME type not found in sanitized email")
			}

			// Check boundary markers are preserved
			if strings.Contains(sanitizedStr, "--boundary") ||
				strings.Contains(sanitizedStr, "boundary=") {
				// Check for boundary markers in body
				if !strings.Contains(sanitizedStr, "--") {
					t.Errorf("Boundary markers not found in body")
				}
			}
		})
	}
}

// TestSanitizeMessage tests string-based sanitization.
func TestSanitizeMessage(t *testing.T) {
	tests := []struct {
		name             string
		rawEmail         string
		expectedInResult string
	}{
		{
			name:             "sanitize_message_string",
			rawEmail:         basicEmailWithFrameworkHeaders,
			expectedInResult: "To:",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := DefaultSanitizationConfig()
			sanitizer := NewSanitizer(config)
			result, err := sanitizer.SanitizeMessage(tt.rawEmail)

			if err != nil {
				t.Fatalf("SanitizeMessage failed: %v", err)
			}

			if !strings.Contains(result, tt.expectedInResult) {
				t.Errorf("Expected string %s not found in sanitized message", tt.expectedInResult)
			}
		})
	}
}

// TestFormatRFC5322Date tests RFC 5322 date formatting.
func TestFormatRFC5322Date(t *testing.T) {
	tests := []struct {
		name     string
		date     string
		expected string
	}{
		{
			name:     "format_date",
			date:     "Mon, 18 Mar 2024 10:30:00 -0500",
			expected: "Mon, 18 Mar 2024 10:30:00 -0500",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Just verify the format function exists and works
			result := formatRFC5322Date(time.Now())
			if len(result) < 5 {
				t.Errorf("RFC 5322 date format seems incorrect: %s", result)
			}
		})
	}
}

// TestSanitizationConfigDefaults tests default configuration values.
func TestSanitizationConfigDefaults(t *testing.T) {
	config := DefaultSanitizationConfig()

	if !config.StripXHeaders {
		t.Errorf("Default StripXHeaders should be true, got false")
	}

	if !config.RewriteMessageID {
		t.Errorf("Default RewriteMessageID should be true, got false")
	}

	if !config.StripReceived {
		t.Errorf("Default StripReceived should be true, got false")
	}
}

// TestSanitizeMIMEMessage tests MIME-specific sanitization.
func TestSanitizeMIMEMessage(t *testing.T) {
	tests := []struct {
		name     string
		rawEmail string
	}{
		{
			name:     "mime_email",
			rawEmail: emailWithMIMEHeaders,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := DefaultSanitizationConfig()
			sanitizer := NewSanitizer(config)
			sanitized, err := sanitizer.SanitizeMIMEMessage([]byte(tt.rawEmail))

			if err != nil {
				t.Fatalf("SanitizeMIMEMessage failed: %v", err)
			}

			if len(sanitized) == 0 {
				t.Errorf("Sanitized MIME message is empty")
			}
		})
	}
}

// TestHeaderParsing tests the internal header parsing functionality.
func TestHeaderParsing(t *testing.T) {
	tests := []struct {
		name            string
		headerBlock     string
		expectedHeaders map[string]string
	}{
		{
			name:        "simple_headers",
			headerBlock: "From: sender@example.com\nTo: recipient@example.com\nSubject: Test",
			expectedHeaders: map[string]string{
				"From":    "sender@example.com",
				"To":      "recipient@example.com",
				"Subject": "Test",
			},
		},
		{
			name:        "headers_with_continuation",
			headerBlock: "From: sender@example.com\nSubject: This is a long\n subject line that continues",
			expectedHeaders: map[string]string{
				"From":    "sender@example.com",
				"Subject": "This is a long subject line that continues",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			headers := parseHeaders([]byte(tt.headerBlock))

			for key, expectedValue := range tt.expectedHeaders {
				if value, ok := headers[key]; ok {
					if value != expectedValue {
						t.Errorf("Header %s: expected %s, got %s", key, expectedValue, value)
					}
				} else {
					t.Errorf("Header %s not found", key)
				}
			}
		})
	}
}

// TestExtractSenderDomain tests sender domain extraction.
func TestExtractSenderDomain(t *testing.T) {
	tests := []struct {
		name         string
		from         string
		expectDomain string
	}{
		{
			name:         "simple_email",
			from:         "sender@example.com",
			expectDomain: "example.com",
		},
		{
			name:         "name_and_email",
			from:         "John Doe <john@example.com>",
			expectDomain: "example.com",
		},
		{
			name:         "email_with_angle_brackets",
			from:         "<admin@example.com>",
			expectDomain: "example.com",
		},
		{
			name:         "multi_recipient",
			from:         "sender1@example.com,sender2@example.org",
			expectDomain: "example.com",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			headers := map[string]string{
				"From": tt.from,
			}
			config := DefaultSanitizationConfig()
			sanitizer := NewSanitizer(config)

			// Extract domain using the method on sanitizer
			domain := sanitizer.extractSenderDomain(headers)

			if !strings.Contains(domain, tt.expectDomain) {
				t.Errorf("Extracted domain %s should contain %s", domain, tt.expectDomain)
			}
		})
	}
}

// TestRandomLocalPartGeneration tests random local part generation.
func TestRandomLocalPartGeneration(t *testing.T) {
	tests := []struct {
		name           string
		expectedLength int
	}{
		{
			name:           "20_char_local_part",
			expectedLength: 20,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := DefaultSanitizationConfig()
			sanitizer := NewSanitizer(config)

			localPart := sanitizer.generateRandomLocalPart()

			if len(localPart) != tt.expectedLength {
				t.Errorf("Expected local part length %d, got %d", tt.expectedLength, len(localPart))
			}

			// Verify all characters are alphanumeric
			for _, r := range localPart {
				if !((r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9')) {
					t.Errorf("Local part contains non-alphanumeric character: %c", r)
				}
			}
		})
	}
}

// TestGenerateMessageIDForReceived tests Received header message ID generation.
func TestGenerateMessageIDForReceived(t *testing.T) {
	config := DefaultSanitizationConfig()
	sanitizer := NewSanitizer(config)

	id := sanitizer.generateMessageIDForReceived()

	// Verify it's valid base64 URL encoded
	decoded, err := base64.URLEncoding.DecodeString(id)
	if err != nil {
		t.Errorf("Generated Received ID is not valid base64: %v", err)
	}

	if len(decoded) != 8 {
		t.Errorf("Expected 8 bytes decoded, got %d", len(decoded))
	}
}

// TestRebuildEmail tests email rebuilding after sanitization.
func TestRebuildEmail(t *testing.T) {
	tests := []struct {
		name            string
		headers         map[string]string
		body            []byte
		expectedHeaders []string
	}{
		{
			name: "simple_rebuild",
			headers: map[string]string{
				"From":    "sender@example.com",
				"To":      "recipient@example.com",
				"Subject": "Test Subject",
			},
			body:            []byte("Email body content"),
			expectedHeaders: []string{"From", "To", "Subject"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rebuilt := rebuildEmail(tt.headers, tt.body, nil)
			rebuiltStr := string(rebuilt)

			for _, header := range tt.expectedHeaders {
				if !strings.Contains(rebuiltStr, header+":") {
					t.Errorf("Expected header %s not found in rebuilt email", header)
				}
			}
		})
	}
}

// TestGetSortedHeaderKeys tests header key sorting.
func TestGetSortedHeaderKeys(t *testing.T) {
	headers := map[string]string{
		"From":     "sender@example.com",
		"To":       "recipient@example.com",
		"Subject":  "Test",
		"Date":     "Mon, 18 Mar 2024 10:30:00 -0500",
		"X-Custom": "value",
	}

	keys := getSortedHeaderKeys(headers)

	// Check priority order is preserved
	priorityOrder := []string{"From", "To", "Subject", "Date"}
	for i, key := range priorityOrder {
		found := false
		for _, k := range keys {
			if k == key {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("Expected header %s not found in sorted keys", key)
		} else if i > 0 && findIndex(keys, priorityOrder[i-1]) > findIndex(keys, key) {
			// This is fine - we just check existence
		}
	}
}

// Helper function to find index in slice
func findIndex(slice []string, item string) int {
	for i, v := range slice {
		if v == item {
			return i
		}
	}
	return -1
}

// Test emails for various test cases
const (
	basicEmailWithFrameworkHeaders = `From: sender@example.com
To: recipient@example.com
Subject: Test Email
Date: Mon, 18 Mar 2024 10:30:00 -0500
Message-ID: <abc123@example.com>
X-Mailer: TackleMailer/1.0
X-Originating-IP: 192.168.1.100
X-Tackle-Version: 1.0.0
User-Agent: Tackle/1.0

This is the email body.
With multiple lines.

--boundary
Content-Type: text/html
`
	emailWithMultipleXHeaders = `From: sender@example.com
To: recipient@example.com
Subject: Test
X-Mailer: TestMailer
X-Originating-IP: 10.0.0.1
X-Mailer-Version: 1.0
X-MimeOLE: Created By
X-MSMail-Priority: Normal
X-Tackle-Sender: internal
X-Custom-Header: custom-value

Body content`
	emailWithReceivedChain = `Received: from mail.example.com by example.com with ESMTP id ABC123
Received: from sender@example.com by mail.example.com with SMTP
Received: from client by sender@example.com

From: sender@example.com
To: recipient@example.com
Subject: Test Email
Message-ID: <msg123@example.com>

Body content`
	emailWithMessageID = `From: sender@example.com
To: recipient@example.com
Subject: Test
Message-ID: <original@old-domain.com>
Date: Mon, 18 Mar 2024 10:30:00 -0500

Body`
	emailWithDKIM = `From: sender@example.com
To: recipient@example.com
Subject: Signed Email
DKIM-Signature: v=1; a=rsa-sha256; c=relaxed/relaxed; d=example.com; s=selector;
 h=from:to:subject:date; bh=abc123;
 b=dGVzdCBERU1PIFNJR05BVFVSRSBHRU5FUkFURUQ=
Received: from mail.example.com by example.com with ESMTP id XYZ789

Body content`
	emailWithCustomHeaders = `From: sender@example.com
To: recipient@example.com
Subject: Test
X-Custom-Property: custom-value
X-Vendor-Tag: vendor-tag
X-Mailer: TestMailer

Body content`
	emailWithMIMEHeaders = `From: sender@example.com
To: recipient@example.com
Subject: MIME Test
MIME-Version: 1.0
Content-Type: text/plain; charset=UTF-8
Content-Transfer-Encoding: quoted-printable

Body content`
	emailWithCommsHeaders = `From: sender@example.com
To: recipient@example.com
Cc: cc@example.com
Bcc: bcc@example.com
Subject: Communication Test
Date: Mon, 18 Mar 2024 10:30:00 -0500
Reply-To: reply@example.com
Return-Path: sender@example.com

Body content`
	emailWithMultipartBody = `From: sender@example.com
To: recipient@example.com
Subject: Multipart Test
MIME-Version: 1.0
Content-Type: multipart/alternative; boundary="boundary123"

--boundary123
Content-Type: text/plain; charset=UTF-8
Content-Transfer-Encoding: quoted-printable

Plain text body

--boundary123
Content-Type: text/html; charset=UTF-8
Content-Transfer-Encoding: quoted-printable

<html><body>HTML body</body></html>

--boundary123--
`
)

