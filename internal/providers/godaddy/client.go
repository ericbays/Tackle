// Package godaddy implements a client for the GoDaddy REST API v1.
package godaddy

import (
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"tackle/internal/providers/ratelimit"
	"tackle/internal/providers/credentials"
)

const (
	productionBaseURL = "https://api.godaddy.com"
	oteBaseURL        = "https://api.ote-godaddy.com"
	defaultRateLimit  = 60 // 60 requests/minute per GoDaddy API limits
	requestTimeout    = 30 * time.Second
)

// Client is a GoDaddy REST API v1 client.
type Client struct {
	creds       credentials.GoDaddyCredentials
	baseURL     string
	httpClient  *http.Client
	rateLimiter *ratelimit.RateLimiter
}

// NewClient creates a GoDaddy API client with TLS enforcement and rate limiting.
// ratePerMinute overrides the default of 60 if > 0.
func NewClient(creds credentials.GoDaddyCredentials, ratePerMinute int) *Client {
	if ratePerMinute <= 0 {
		ratePerMinute = defaultRateLimit
	}
	base := productionBaseURL
	if creds.Environment == credentials.GoDaddyOTE {
		base = oteBaseURL
	}
	return &Client{
		creds:   creds,
		baseURL: base,
		httpClient: &http.Client{
			Timeout: requestTimeout,
			Transport: &http.Transport{
				TLSClientConfig: &tls.Config{MinVersion: tls.VersionTLS12},
			},
		},
		rateLimiter: ratelimit.NewRateLimiter(ratePerMinute),
	}
}

// godaddyError represents a GoDaddy API error response.
type godaddyError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

// TestConnection validates credentials by calling GET /v1/domains?limit=1.
// Returns nil on success or a descriptive, actionable error on failure.
func (c *Client) TestConnection() error {
	c.rateLimiter.Wait()

	req, err := http.NewRequest(http.MethodGet, c.baseURL+"/v1/domains?limit=1", nil)
	if err != nil {
		return fmt.Errorf("godaddy: build request: %w", err)
	}
	req.Header.Set("Authorization", fmt.Sprintf("sso-key %s:%s", c.creds.APIKey, c.creds.APISecret))
	req.Header.Set("Accept", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("godaddy: connection failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<16))
	if err != nil {
		return fmt.Errorf("godaddy: read response: %w", err)
	}

	if resp.StatusCode == http.StatusOK {
		return nil
	}

	return c.translateError(resp.StatusCode, body)
}

// translateError converts GoDaddy HTTP status codes and error bodies into actionable messages.
func (c *Client) translateError(statusCode int, body []byte) error {
	var apiErr godaddyError
	_ = json.Unmarshal(body, &apiErr) // best-effort parse; ignore unmarshal errors

	switch statusCode {
	case http.StatusUnauthorized:
		return fmt.Errorf("godaddy: authentication failed. Verify your API key and secret are correct")
	case http.StatusForbidden:
		return fmt.Errorf("godaddy: access denied. Ensure your API key has the required permissions")
	case http.StatusTooManyRequests:
		return fmt.Errorf("godaddy: rate limit exceeded. Reduce the configured rate limit for this connection")
	default:
		if apiErr.Message != "" {
			return fmt.Errorf("godaddy: API error %d (%s): %s", statusCode, apiErr.Code, apiErr.Message)
		}
		return fmt.Errorf("godaddy: unexpected HTTP %d response", statusCode)
	}
}
