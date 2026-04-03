package namecheap

import (
	"encoding/xml"
	"fmt"
	"io"
	"net/url"
	"strconv"
	"time"
)

// AvailabilityResult is the result of a domain availability check.
type AvailabilityResult struct {
	Available bool
	Premium   bool
	Price     float64
	Currency  string
}

// RegistrantInfo holds registrant contact data required for domain registration.
type RegistrantInfo struct {
	FirstName   string
	LastName    string
	Address     string
	City        string
	StateProvince string
	PostalCode  string
	Country     string // 2-letter ISO code
	Phone       string // +1.5551234567
	EmailAddress string
}

// RegistrationResult is returned after a successful domain registration.
type RegistrationResult struct {
	Domain       string
	OrderID      string
	RegistrationDate time.Time
	ExpiryDate   time.Time
}

// DomainInfo holds details about an existing domain retrieved from the registrar.
type DomainInfo struct {
	Domain           string
	RegistrationDate time.Time
	ExpiryDate       time.Time
	AutoRenew        bool
	Nameservers      []string
}

// RenewalResult is returned after a successful domain renewal.
type RenewalResult struct {
	Domain     string
	ExpiryDate time.Time
	OrderID    string
}

// --- XML response structs ---

type checkResponse struct {
	apiResponse
	CommandResponse struct {
		DomainCheckResult []struct {
			Domain    string `xml:"Domain,attr"`
			Available string `xml:"Available,attr"`
			IsPremiumName string `xml:"IsPremiumName,attr"`
			PremiumRegistrationPrice string `xml:"PremiumRegistrationPrice,attr"`
			PriceCurrency string `xml:"PriceCurrency,attr"`
		} `xml:"DomainCheckResult"`
	} `xml:"CommandResponse"`
}

type createResponse struct {
	apiResponse
	CommandResponse struct {
		DomainCreateResult struct {
			Domain     string `xml:"Domain,attr"`
			Registered string `xml:"Registered,attr"`
			OrderID    string `xml:"OrderID,attr"`
		} `xml:"DomainCreateResult"`
	} `xml:"CommandResponse"`
}

type getInfoResponse struct {
	apiResponse
	CommandResponse struct {
		DomainGetInfoResult struct {
			DomainName string `xml:"DomainName,attr"`
			DomainDetails struct {
				CreatedDate  string `xml:"CreatedDate"`
				ExpiredDate  string `xml:"ExpiredDate"`
				AutoRenew    string `xml:"AutoRenew,attr"`
			} `xml:"DomainDetails"`
			DnsDetails struct {
				Nameserver []string `xml:"Nameserver"`
			} `xml:"DnsDetails"`
		} `xml:"DomainGetInfoResult"`
	} `xml:"CommandResponse"`
}

type renewResponse struct {
	apiResponse
	CommandResponse struct {
		DomainRenewResult struct {
			DomainName string `xml:"DomainName,attr"`
			Renewed    string `xml:"Renewed,attr"`
			OrderID    string `xml:"OrderID,attr"`
			ExpireDate string `xml:"ExpireDate,attr"`
		} `xml:"DomainRenewResult"`
	} `xml:"CommandResponse"`
}

// CheckAvailability checks whether a domain name is available for registration.
func (c *Client) CheckAvailability(domain string) (AvailabilityResult, error) {
	c.rateLimiter.Wait()

	params := url.Values{
		"ApiUser":  {c.creds.APIUser},
		"ApiKey":   {c.creds.APIKey},
		"UserName": {c.creds.Username},
		"ClientIp": {c.creds.ClientIP},
		"Command":  {"namecheap.domains.check"},
		"DomainList": {domain},
	}

	body, err := c.doGet(params)
	if err != nil {
		return AvailabilityResult{}, err
	}

	var resp checkResponse
	if err := xml.Unmarshal(body, &resp); err != nil {
		return AvailabilityResult{}, fmt.Errorf("namecheap: check availability: parse: %w", err)
	}
	if resp.Status != "OK" {
		return AvailabilityResult{}, c.translateError(resp.Errors)
	}
	if len(resp.CommandResponse.DomainCheckResult) == 0 {
		return AvailabilityResult{}, fmt.Errorf("namecheap: check availability: empty response")
	}

	r := resp.CommandResponse.DomainCheckResult[0]
	available := r.Available == "true"
	premium := r.IsPremiumName == "true"
	var price float64
	if r.PremiumRegistrationPrice != "" {
		price, _ = strconv.ParseFloat(r.PremiumRegistrationPrice, 64)
	}
	return AvailabilityResult{
		Available: available,
		Premium:   premium,
		Price:     price,
		Currency:  r.PriceCurrency,
	}, nil
}

// RegisterDomain registers a domain name via the Namecheap API.
// Returns a RegistrationResult on success.
func (c *Client) RegisterDomain(domain string, years int, contacts RegistrantInfo) (RegistrationResult, error) {
	c.rateLimiter.Wait()

	params := url.Values{
		"ApiUser":  {c.creds.APIUser},
		"ApiKey":   {c.creds.APIKey},
		"UserName": {c.creds.Username},
		"ClientIp": {c.creds.ClientIP},
		"Command":  {"namecheap.domains.create"},
		"DomainName": {domain},
		"Years":    {strconv.Itoa(years)},
		// Registrant contact fields.
		"RegistrantFirstName":   {contacts.FirstName},
		"RegistrantLastName":    {contacts.LastName},
		"RegistrantAddress1":    {contacts.Address},
		"RegistrantCity":        {contacts.City},
		"RegistrantStateProvince": {contacts.StateProvince},
		"RegistrantPostalCode":  {contacts.PostalCode},
		"RegistrantCountry":     {contacts.Country},
		"RegistrantPhone":       {contacts.Phone},
		"RegistrantEmailAddress": {contacts.EmailAddress},
		// Tech contact — same as registrant.
		"TechFirstName":   {contacts.FirstName},
		"TechLastName":    {contacts.LastName},
		"TechAddress1":    {contacts.Address},
		"TechCity":        {contacts.City},
		"TechStateProvince": {contacts.StateProvince},
		"TechPostalCode":  {contacts.PostalCode},
		"TechCountry":     {contacts.Country},
		"TechPhone":       {contacts.Phone},
		"TechEmailAddress": {contacts.EmailAddress},
		// Admin contact — same as registrant.
		"AdminFirstName":   {contacts.FirstName},
		"AdminLastName":    {contacts.LastName},
		"AdminAddress1":    {contacts.Address},
		"AdminCity":        {contacts.City},
		"AdminStateProvince": {contacts.StateProvince},
		"AdminPostalCode":  {contacts.PostalCode},
		"AdminCountry":     {contacts.Country},
		"AdminPhone":       {contacts.Phone},
		"AdminEmailAddress": {contacts.EmailAddress},
		// AuxBilling contact — same as registrant.
		"AuxBillingFirstName":   {contacts.FirstName},
		"AuxBillingLastName":    {contacts.LastName},
		"AuxBillingAddress1":    {contacts.Address},
		"AuxBillingCity":        {contacts.City},
		"AuxBillingStateProvince": {contacts.StateProvince},
		"AuxBillingPostalCode":  {contacts.PostalCode},
		"AuxBillingCountry":     {contacts.Country},
		"AuxBillingPhone":       {contacts.Phone},
		"AuxBillingEmailAddress": {contacts.EmailAddress},
	}

	body, err := c.doGet(params)
	if err != nil {
		return RegistrationResult{}, err
	}

	var resp createResponse
	if err := xml.Unmarshal(body, &resp); err != nil {
		return RegistrationResult{}, fmt.Errorf("namecheap: register domain: parse: %w", err)
	}
	if resp.Status != "OK" {
		return RegistrationResult{}, c.translateRegisterError(resp.Errors)
	}

	result := resp.CommandResponse.DomainCreateResult
	if result.Registered != "true" {
		return RegistrationResult{}, fmt.Errorf("namecheap: domain %q was not registered; check account balance and TLD availability", domain)
	}

	now := time.Now().UTC()
	return RegistrationResult{
		Domain:           result.Domain,
		OrderID:          result.OrderID,
		RegistrationDate: now,
		ExpiryDate:       now.AddDate(years, 0, 0),
	}, nil
}

// GetDomainInfo retrieves registration and expiry details for a domain.
func (c *Client) GetDomainInfo(domain string) (DomainInfo, error) {
	c.rateLimiter.Wait()

	params := url.Values{
		"ApiUser":    {c.creds.APIUser},
		"ApiKey":     {c.creds.APIKey},
		"UserName":   {c.creds.Username},
		"ClientIp":   {c.creds.ClientIP},
		"Command":    {"namecheap.domains.getInfo"},
		"DomainName": {domain},
	}

	body, err := c.doGet(params)
	if err != nil {
		return DomainInfo{}, err
	}

	var resp getInfoResponse
	if err := xml.Unmarshal(body, &resp); err != nil {
		return DomainInfo{}, fmt.Errorf("namecheap: get domain info: parse: %w", err)
	}
	if resp.Status != "OK" {
		return DomainInfo{}, c.translateError(resp.Errors)
	}

	details := resp.CommandResponse.DomainGetInfoResult.DomainDetails
	created, _ := time.Parse("01/02/2006", details.CreatedDate)
	expiry, _ := time.Parse("01/02/2006", details.ExpiredDate)
	autoRenew := details.AutoRenew == "true"

	return DomainInfo{
		Domain:           resp.CommandResponse.DomainGetInfoResult.DomainName,
		RegistrationDate: created,
		ExpiryDate:       expiry,
		AutoRenew:        autoRenew,
		Nameservers:      resp.CommandResponse.DomainGetInfoResult.DnsDetails.Nameserver,
	}, nil
}

// RenewDomain renews a domain for the given number of years.
func (c *Client) RenewDomain(domain string, years int) (RenewalResult, error) {
	c.rateLimiter.Wait()

	params := url.Values{
		"ApiUser":    {c.creds.APIUser},
		"ApiKey":     {c.creds.APIKey},
		"UserName":   {c.creds.Username},
		"ClientIp":   {c.creds.ClientIP},
		"Command":    {"namecheap.domains.renew"},
		"DomainName": {domain},
		"Years":      {strconv.Itoa(years)},
	}

	body, err := c.doGet(params)
	if err != nil {
		return RenewalResult{}, err
	}

	var resp renewResponse
	if err := xml.Unmarshal(body, &resp); err != nil {
		return RenewalResult{}, fmt.Errorf("namecheap: renew domain: parse: %w", err)
	}
	if resp.Status != "OK" {
		return RenewalResult{}, c.translateError(resp.Errors)
	}

	r := resp.CommandResponse.DomainRenewResult
	if r.Renewed != "true" {
		return RenewalResult{}, fmt.Errorf("namecheap: renewal for %q was not confirmed", domain)
	}

	expiry, _ := time.Parse("01/02/2006", r.ExpireDate)
	return RenewalResult{
		Domain:     r.DomainName,
		ExpiryDate: expiry,
		OrderID:    r.OrderID,
	}, nil
}

// doGet executes a GET request to the Namecheap API and returns the raw body.
func (c *Client) doGet(params url.Values) ([]byte, error) {
	reqURL := baseURL + "?" + params.Encode()
	resp, err := c.httpClient.Get(reqURL)
	if err != nil {
		return nil, fmt.Errorf("namecheap: request failed: %w", err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<16))
	if err != nil {
		return nil, fmt.Errorf("namecheap: read response: %w", err)
	}
	return body, nil
}

// translateRegisterError handles registration-specific error codes in addition to common ones.
func (c *Client) translateRegisterError(errs []Error) error {
	if len(errs) == 0 {
		return fmt.Errorf("namecheap: unknown registration error")
	}
	switch errs[0].Number {
	case "2033409":
		return fmt.Errorf("namecheap: domain %q is no longer available (race condition — another registrant just registered it)", "domain")
	case "4023271":
		return fmt.Errorf("namecheap: insufficient funds to register domain. Add funds to your Namecheap account")
	case "2033407":
		return fmt.Errorf("namecheap: TLD is not supported for registration via API")
	default:
		return c.translateError(errs)
	}
}
