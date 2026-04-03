package godaddy

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// AvailabilityResult is the result of a domain availability check.
type AvailabilityResult struct {
	Available bool
	Premium   bool
	Price     float64
	Currency  string
}

// RegistrantInfo holds contact data required for domain registration.
type RegistrantInfo struct {
	FirstName    string
	LastName     string
	Address      string
	City         string
	State        string
	PostalCode   string
	Country      string // 2-letter ISO code
	Phone        string // +1.5551234567
	EmailAddress string
}

// RegistrationResult is returned after a successful domain registration.
type RegistrationResult struct {
	Domain           string
	OrderID          string
	RegistrationDate time.Time
	ExpiryDate       time.Time
}

// DomainInfo holds details about an existing domain from the registrar.
type DomainInfo struct {
	Domain           string
	RegistrationDate time.Time
	ExpiryDate       time.Time
	AutoRenew        bool
	Nameservers      []string
	Status           string
}

// RenewalResult is returned after a successful domain renewal.
type RenewalResult struct {
	Domain     string
	ExpiryDate time.Time
	OrderID    string
}

// --- API response types ---

type availabilityResp struct {
	Available  bool    `json:"available"`
	Currency   string  `json:"currency"`
	Definitive bool    `json:"definitive"`
	Domain     string  `json:"domain"`
	Period     int     `json:"period"`
	Price      float64 `json:"price"`
}

type purchaseResp struct {
	OrderID  int    `json:"orderId"`
	ItemCount int   `json:"itemCount"`
	Total    float64 `json:"total"`
	Currency string  `json:"currency"`
}

type domainDetailResp struct {
	Domain      string   `json:"domain"`
	Status      string   `json:"status"`
	Expires     string   `json:"expires"`
	CreatedAt   string   `json:"createdAt"`
	RenewAuto   bool     `json:"renewAuto"`
	NameServers []string `json:"nameServers"`
}

type renewResp struct {
	OrderID  int     `json:"orderId"`
	ItemCount int    `json:"itemCount"`
	Total    float64 `json:"total"`
	Currency string  `json:"currency"`
}

// CheckAvailability checks whether a domain is available for registration.
func (c *Client) CheckAvailability(domain string) (AvailabilityResult, error) {
	c.rateLimiter.Wait()

	req, err := http.NewRequest(http.MethodGet, c.baseURL+"/v1/domains/available?domain="+domain, nil)
	if err != nil {
		return AvailabilityResult{}, fmt.Errorf("godaddy: check availability: build request: %w", err)
	}
	c.setAuthHeader(req)

	body, statusCode, err := c.doRequest(req)
	if err != nil {
		return AvailabilityResult{}, err
	}
	if statusCode != http.StatusOK {
		return AvailabilityResult{}, c.translateError(statusCode, body)
	}

	var resp availabilityResp
	if err := json.Unmarshal(body, &resp); err != nil {
		return AvailabilityResult{}, fmt.Errorf("godaddy: check availability: parse response: %w", err)
	}

	// GoDaddy does not have a separate "premium" flag in the availability endpoint;
	// a higher price indicates a premium domain.
	premium := resp.Price > 0 && resp.Price > 15.0
	return AvailabilityResult{
		Available: resp.Available,
		Premium:   premium,
		Price:     resp.Price / 1_000_000, // GoDaddy returns price in micros
		Currency:  resp.Currency,
	}, nil
}

// RegisterDomain purchases a domain via the GoDaddy API.
func (c *Client) RegisterDomain(domain string, years int, contacts RegistrantInfo) (RegistrationResult, error) {
	c.rateLimiter.Wait()

	contact := map[string]any{
		"nameFirst": contacts.FirstName,
		"nameLast":  contacts.LastName,
		"addressMailing": map[string]any{
			"address1":   contacts.Address,
			"city":       contacts.City,
			"state":      contacts.State,
			"postalCode": contacts.PostalCode,
			"country":    contacts.Country,
		},
		"phone": contacts.Phone,
		"email": contacts.EmailAddress,
	}

	payload := map[string]any{
		"domain": domain,
		"period": years,
		"consent": map[string]any{
			"agreedAt":      time.Now().UTC().Format(time.RFC3339),
			"agreedBy":      contacts.EmailAddress,
			"agreementKeys": []string{"DNRA"},
		},
		"contactAdmin":    contact,
		"contactBilling":  contact,
		"contactRegistrant": contact,
		"contactTech":     contact,
	}

	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return RegistrationResult{}, fmt.Errorf("godaddy: register domain: marshal: %w", err)
	}

	req, err := http.NewRequest(http.MethodPost, c.baseURL+"/v1/domains/purchase", bytes.NewReader(payloadBytes))
	if err != nil {
		return RegistrationResult{}, fmt.Errorf("godaddy: register domain: build request: %w", err)
	}
	c.setAuthHeader(req)
	req.Header.Set("Content-Type", "application/json")

	body, statusCode, err := c.doRequest(req)
	if err != nil {
		return RegistrationResult{}, err
	}
	if statusCode != http.StatusOK && statusCode != http.StatusAccepted {
		return RegistrationResult{}, c.translatePurchaseError(statusCode, body)
	}

	var resp purchaseResp
	if err := json.Unmarshal(body, &resp); err != nil {
		return RegistrationResult{}, fmt.Errorf("godaddy: register domain: parse response: %w", err)
	}

	now := time.Now().UTC()
	return RegistrationResult{
		Domain:           domain,
		OrderID:          fmt.Sprintf("%d", resp.OrderID),
		RegistrationDate: now,
		ExpiryDate:       now.AddDate(years, 0, 0),
	}, nil
}

// GetDomainInfo retrieves registration and expiry details for a domain.
func (c *Client) GetDomainInfo(domain string) (DomainInfo, error) {
	c.rateLimiter.Wait()

	req, err := http.NewRequest(http.MethodGet, c.baseURL+"/v1/domains/"+domain, nil)
	if err != nil {
		return DomainInfo{}, fmt.Errorf("godaddy: get domain info: build request: %w", err)
	}
	c.setAuthHeader(req)

	body, statusCode, err := c.doRequest(req)
	if err != nil {
		return DomainInfo{}, err
	}
	if statusCode != http.StatusOK {
		return DomainInfo{}, c.translateError(statusCode, body)
	}

	var resp domainDetailResp
	if err := json.Unmarshal(body, &resp); err != nil {
		return DomainInfo{}, fmt.Errorf("godaddy: get domain info: parse response: %w", err)
	}

	created, _ := time.Parse(time.RFC3339, resp.CreatedAt)
	expiry, _ := time.Parse(time.RFC3339, resp.Expires)

	return DomainInfo{
		Domain:           resp.Domain,
		RegistrationDate: created,
		ExpiryDate:       expiry,
		AutoRenew:        resp.RenewAuto,
		Nameservers:      resp.NameServers,
		Status:           resp.Status,
	}, nil
}

// RenewDomain renews a domain for the given number of years.
func (c *Client) RenewDomain(domain string, years int) (RenewalResult, error) {
	c.rateLimiter.Wait()

	payload := map[string]any{"period": years}
	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return RenewalResult{}, fmt.Errorf("godaddy: renew domain: marshal: %w", err)
	}

	req, err := http.NewRequest(http.MethodPost, c.baseURL+"/v1/domains/"+domain+"/renew", bytes.NewReader(payloadBytes))
	if err != nil {
		return RenewalResult{}, fmt.Errorf("godaddy: renew domain: build request: %w", err)
	}
	c.setAuthHeader(req)
	req.Header.Set("Content-Type", "application/json")

	body, statusCode, err := c.doRequest(req)
	if err != nil {
		return RenewalResult{}, err
	}
	if statusCode != http.StatusOK && statusCode != http.StatusNoContent {
		return RenewalResult{}, c.translateError(statusCode, body)
	}

	var resp renewResp
	_ = json.Unmarshal(body, &resp) // best-effort; may be empty on 204

	// GoDaddy renew does not return a new expiry in the response body.
	// Caller should re-query GetDomainInfo to get updated expiry.
	return RenewalResult{
		Domain:     domain,
		ExpiryDate: time.Time{}, // zero — caller must sync
		OrderID:    fmt.Sprintf("%d", resp.OrderID),
	}, nil
}

// setAuthHeader adds the GoDaddy API key authorization header.
func (c *Client) setAuthHeader(req *http.Request) {
	req.Header.Set("Authorization", fmt.Sprintf("sso-key %s:%s", c.creds.APIKey, c.creds.APISecret))
	req.Header.Set("Accept", "application/json")
}

// doRequest executes an HTTP request and returns the response body and status code.
func (c *Client) doRequest(req *http.Request) ([]byte, int, error) {
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, 0, fmt.Errorf("godaddy: request failed: %w", err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<16))
	if err != nil {
		return nil, resp.StatusCode, fmt.Errorf("godaddy: read response: %w", err)
	}
	return body, resp.StatusCode, nil
}

// translatePurchaseError converts purchase-specific error codes into actionable messages.
func (c *Client) translatePurchaseError(statusCode int, body []byte) error {
	var apiErr godaddyError
	_ = json.Unmarshal(body, &apiErr)

	switch apiErr.Code {
	case "UNABLE_TO_AUTHENTICATE":
		return fmt.Errorf("godaddy: authentication failed during purchase. Verify API credentials")
	case "DUPLICATE_ORDER":
		return fmt.Errorf("godaddy: domain was already purchased (duplicate order)")
	case "NOT_AVAILABLE":
		return fmt.Errorf("godaddy: domain is no longer available (race condition — another registrant just registered it)")
	case "INSUFFICIENT_FUNDS":
		return fmt.Errorf("godaddy: insufficient account funds to complete purchase")
	default:
		return c.translateError(statusCode, body)
	}
}
