// Package namecheap implements a client for the Namecheap XML API.
package namecheap

import (
	"crypto/tls"
	"encoding/xml"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"

	"tackle/internal/providers/credentials"
	"tackle/internal/providers/ratelimit"
)

const (
	baseURL          = "https://api.namecheap.com/xml.response"
	defaultRateLimit = 20 // 20 requests/minute per Namecheap API limits
	requestTimeout   = 30 * time.Second
)

// Client is a Namecheap XML API client.
type Client struct {
	creds       credentials.NamecheapCredentials
	httpClient  *http.Client
	rateLimiter *ratelimit.RateLimiter
}

// NewClient creates a Namecheap API client with TLS enforcement and rate limiting.
// ratePerMinute overrides the default of 20 if > 0.
func NewClient(creds credentials.NamecheapCredentials, ratePerMinute int) *Client {
	if ratePerMinute <= 0 {
		ratePerMinute = defaultRateLimit
	}
	return &Client{
		creds: creds,
		httpClient: &http.Client{
			Timeout: requestTimeout,
			Transport: &http.Transport{
				TLSClientConfig: &tls.Config{MinVersion: tls.VersionTLS12},
			},
		},
		rateLimiter: ratelimit.NewRateLimiter(ratePerMinute),
	}
}

// apiResponse is the top-level Namecheap XML response envelope.
type apiResponse struct {
	Status string  `xml:"Status,attr"`
	Errors []Error `xml:"Errors>Error"`
}

// Error represents a Namecheap API error element.
type Error struct {
	Number  string `xml:"Number,attr"`
	Message string `xml:",chardata"`
}

// TestConnection validates credentials by calling namecheap.domains.getList with PageSize=1.
// Returns nil on success or a descriptive, actionable error on failure.
func (c *Client) TestConnection() error {
	c.rateLimiter.Wait()

	params := url.Values{
		"ApiUser":   {c.creds.APIUser},
		"ApiKey":    {c.creds.APIKey},
		"UserName":  {c.creds.Username},
		"ClientIp":  {c.creds.ClientIP},
		"Command":   {"namecheap.domains.getList"},
		"PageSize":  {"1"},
	}

	reqURL := baseURL + "?" + params.Encode()
	resp, err := c.httpClient.Get(reqURL)
	if err != nil {
		return fmt.Errorf("namecheap: connection failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<16)) // 64 KB limit
	if err != nil {
		return fmt.Errorf("namecheap: read response: %w", err)
	}

	var apiResp apiResponse
	if err := xml.Unmarshal(body, &apiResp); err != nil {
		return fmt.Errorf("namecheap: parse response: %w", err)
	}

	if apiResp.Status == "OK" {
		return nil
	}

	return c.translateError(apiResp.Errors)
}

// parseXML is an internal helper for unmarshaling XML API responses, exposed for testing.
func parseXML(data []byte, target *apiResponse) error {
	return xml.Unmarshal(data, target)
}

// translateError converts Namecheap error codes into actionable messages.
func (c *Client) translateError(errs []Error) error {
	if len(errs) == 0 {
		return fmt.Errorf("namecheap: unknown error from API")
	}
	switch errs[0].Number {
	case "1011102":
		return fmt.Errorf("namecheap: API key is invalid. Verify your Namecheap API credentials")
	case "1011150":
		return fmt.Errorf("namecheap: your IP address (%s) is not whitelisted in Namecheap. Add it to your API whitelist in Account > Profile > API Access", c.creds.ClientIP)
	case "1011306":
		return fmt.Errorf("namecheap: account is locked. Check your Namecheap account status")
	default:
		return fmt.Errorf("namecheap: API error %s: %s", errs[0].Number, errs[0].Message)
	}
}
