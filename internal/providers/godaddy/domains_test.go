package godaddy

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"tackle/internal/providers/credentials"
	"tackle/internal/providers/ratelimit"
)

func clientWithServer(t *testing.T, srv *httptest.Server) *Client {
	t.Helper()
	creds := credentials.GoDaddyCredentials{
		APIKey:      "testkey",
		APISecret:   "testsecret",
		Environment: credentials.GoDaddyProduction,
	}
	return &Client{
		creds:       creds,
		baseURL:     srv.URL,
		httpClient:  srv.Client(),
		rateLimiter: ratelimit.NewRateLimiter(100),
	}
}

func TestCheckAvailability_Available(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/domains/available" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"available":true,"currency":"USD","price":12000000,"domain":"example.com"}`))
	}))
	defer srv.Close()

	c := clientWithServer(t, srv)
	result, err := c.CheckAvailability("example.com")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.Available {
		t.Error("expected Available=true")
	}
	if result.Currency != "USD" {
		t.Errorf("expected Currency=USD, got %q", result.Currency)
	}
}

func TestCheckAvailability_Unavailable(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"available":false,"domain":"taken.com"}`))
	}))
	defer srv.Close()

	c := clientWithServer(t, srv)
	result, err := c.CheckAvailability("taken.com")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Available {
		t.Error("expected Available=false")
	}
}

func TestCheckAvailability_AuthError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnauthorized)
		w.Write([]byte(`{"code":"UNABLE_TO_AUTHENTICATE","message":"Unauthorized"}`))
	}))
	defer srv.Close()

	c := clientWithServer(t, srv)
	_, err := c.CheckAvailability("example.com")
	if err == nil {
		t.Fatal("expected error on 401")
	}
}

func TestGetDomainInfo(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/domains/example.com" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{
			"domain":"example.com",
			"status":"ACTIVE",
			"expires":"2025-01-15T00:00:00.000Z",
			"createdAt":"2022-01-15T00:00:00.000Z",
			"renewAuto":true,
			"nameServers":["ns1.example.com","ns2.example.com"]
		}`))
	}))
	defer srv.Close()

	c := clientWithServer(t, srv)
	info, err := c.GetDomainInfo("example.com")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if info.Domain != "example.com" {
		t.Errorf("expected domain=example.com, got %q", info.Domain)
	}
	if !info.AutoRenew {
		t.Error("expected AutoRenew=true")
	}
	if len(info.Nameservers) != 2 {
		t.Errorf("expected 2 nameservers, got %d", len(info.Nameservers))
	}
}

func TestRenewDomain(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("expected POST, got %s", r.Method)
		}
		if r.URL.Path != "/v1/domains/example.com/renew" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"orderId":99,"itemCount":1,"total":12.0,"currency":"USD"}`))
	}))
	defer srv.Close()

	c := clientWithServer(t, srv)
	result, err := c.RenewDomain("example.com", 1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Domain != "example.com" {
		t.Errorf("expected domain=example.com, got %q", result.Domain)
	}
	if result.OrderID != "99" {
		t.Errorf("expected OrderID=99, got %q", result.OrderID)
	}
}

func TestRenewDomain_Error(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusTooManyRequests)
		w.Write([]byte(`{"code":"RATE_LIMITED","message":"Too many requests"}`))
	}))
	defer srv.Close()

	c := clientWithServer(t, srv)
	_, err := c.RenewDomain("example.com", 1)
	if err == nil {
		t.Fatal("expected error on 429")
	}
}
