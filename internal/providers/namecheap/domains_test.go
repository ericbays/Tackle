package namecheap

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"tackle/internal/providers/credentials"
	"tackle/internal/providers/ratelimit"
)


func TestCheckAvailability_Available(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/xml")
		w.Write([]byte(`<?xml version="1.0"?>
<ApiResponse Status="OK" xmlns="http://api.namecheap.com/xml.response">
  <Errors/>
  <CommandResponse>
    <DomainCheckResult Domain="example.com" Available="true" IsPremiumName="false" PremiumRegistrationPrice="" PriceCurrency=""/>
  </CommandResponse>
</ApiResponse>`))
	}))
	defer srv.Close()

	c := clientWithServer(t, srv.URL)
	result, err := c.CheckAvailability("example.com")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.Available {
		t.Error("expected Available=true")
	}
	if result.Premium {
		t.Error("expected Premium=false")
	}
}

func TestCheckAvailability_Unavailable(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`<?xml version="1.0"?>
<ApiResponse Status="OK">
  <Errors/>
  <CommandResponse>
    <DomainCheckResult Domain="taken.com" Available="false" IsPremiumName="false" PremiumRegistrationPrice="" PriceCurrency=""/>
  </CommandResponse>
</ApiResponse>`))
	}))
	defer srv.Close()

	c := clientWithServer(t, srv.URL)
	result, err := c.CheckAvailability("taken.com")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Available {
		t.Error("expected Available=false")
	}
}

func TestCheckAvailability_Premium(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`<?xml version="1.0"?>
<ApiResponse Status="OK">
  <Errors/>
  <CommandResponse>
    <DomainCheckResult Domain="premium.com" Available="true" IsPremiumName="true" PremiumRegistrationPrice="499.99" PriceCurrency="USD"/>
  </CommandResponse>
</ApiResponse>`))
	}))
	defer srv.Close()

	c := clientWithServer(t, srv.URL)
	result, err := c.CheckAvailability("premium.com")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.Premium {
		t.Error("expected Premium=true")
	}
	if result.Price != 499.99 {
		t.Errorf("expected Price=499.99, got %v", result.Price)
	}
	if result.Currency != "USD" {
		t.Errorf("expected Currency=USD, got %q", result.Currency)
	}
}

func TestCheckAvailability_APIError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`<?xml version="1.0"?>
<ApiResponse Status="ERROR">
  <Errors>
    <Error Number="1011102">API Key is invalid</Error>
  </Errors>
  <CommandResponse/>
</ApiResponse>`))
	}))
	defer srv.Close()

	c := clientWithServer(t, srv.URL)
	_, err := c.CheckAvailability("example.com")
	if err == nil {
		t.Fatal("expected error for invalid API key")
	}
}

func TestGetDomainInfo(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`<?xml version="1.0"?>
<ApiResponse Status="OK">
  <Errors/>
  <CommandResponse>
    <DomainGetInfoResult DomainName="example.com">
      <DomainDetails AutoRenew="true">
        <CreatedDate>01/15/2022</CreatedDate>
        <ExpiredDate>01/15/2025</ExpiredDate>
      </DomainDetails>
      <DnsDetails>
        <Nameserver>ns1.example.com</Nameserver>
        <Nameserver>ns2.example.com</Nameserver>
      </DnsDetails>
    </DomainGetInfoResult>
  </CommandResponse>
</ApiResponse>`))
	}))
	defer srv.Close()

	c := clientWithServer(t, srv.URL)
	info, err := c.GetDomainInfo("example.com")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if info.Domain != "example.com" {
		t.Errorf("expected domain=example.com, got %q", info.Domain)
	}
	if len(info.Nameservers) != 2 {
		t.Errorf("expected 2 nameservers, got %d", len(info.Nameservers))
	}
}

func TestRenewDomain(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`<?xml version="1.0"?>
<ApiResponse Status="OK">
  <Errors/>
  <CommandResponse>
    <DomainRenewResult DomainName="example.com" Renewed="true" OrderID="12345" ExpireDate="01/15/2026"/>
  </CommandResponse>
</ApiResponse>`))
	}))
	defer srv.Close()

	c := clientWithServer(t, srv.URL)
	result, err := c.RenewDomain("example.com", 1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Domain != "example.com" {
		t.Errorf("expected domain=example.com, got %q", result.Domain)
	}
	if result.OrderID != "12345" {
		t.Errorf("expected order ID 12345, got %q", result.OrderID)
	}
}

func TestRenewDomain_NotRenewed(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`<?xml version="1.0"?>
<ApiResponse Status="OK">
  <Errors/>
  <CommandResponse>
    <DomainRenewResult DomainName="example.com" Renewed="false" OrderID="" ExpireDate=""/>
  </CommandResponse>
</ApiResponse>`))
	}))
	defer srv.Close()

	c := clientWithServer(t, srv.URL)
	_, err := c.RenewDomain("example.com", 1)
	if err == nil {
		t.Fatal("expected error when Renewed=false")
	}
}

// clientWithServer returns a Client whose HTTP calls go to the given test server URL.
func clientWithServer(t *testing.T, serverURL string) *Client {
	t.Helper()
	creds := credentials.NamecheapCredentials{
		APIUser:  "testuser",
		APIKey:   "testapikey",
		Username: "testuser",
		ClientIP: "1.2.3.4",
	}
	// Override the baseURL constant by constructing requests against serverURL.
	// We do this by patching the Client to use a custom transport that rewrites the host.
	transport := &rewriteTransport{targetURL: serverURL}
	return &Client{
		creds:       creds,
		httpClient:  &http.Client{Transport: transport},
		rateLimiter: ratelimit.NewRateLimiter(100),
	}
}

// rewriteTransport redirects all requests to targetURL (ignoring scheme/host).
type rewriteTransport struct {
	targetURL string
}

func (t *rewriteTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	// Parse the target and swap scheme+host.
	newReq := req.Clone(req.Context())
	newReq.URL.Scheme = "http"
	// Extract host from targetURL.
	u := req.URL
	u.Scheme = "http"
	u.Host = hostFrom(t.targetURL)
	newReq.URL = u
	return http.DefaultTransport.RoundTrip(newReq)
}

func hostFrom(u string) string {
	// Strip scheme.
	if len(u) > 7 && u[:7] == "http://" {
		return u[7:]
	}
	return u
}
