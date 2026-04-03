//go:build integration

package phase2

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
)

// ---- Mock Namecheap XML API ----

// namecheapMock simulates the Namecheap XML API for domain provider tests.
type namecheapMock struct {
	srv      *httptest.Server
	mu       sync.Mutex
	requests []string // recorded command names
	// overrideFn allows tests to customize response per command.
	overrideFn func(command string) string
}

func newNamecheapMock(t *testing.T) *namecheapMock {
	t.Helper()
	m := &namecheapMock{}
	m.srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		cmd := r.URL.Query().Get("Command")
		m.mu.Lock()
		m.requests = append(m.requests, cmd)
		fn := m.overrideFn
		m.mu.Unlock()

		var body string
		if fn != nil {
			body = fn(cmd)
		} else {
			body = namecheapDefaultResponse(cmd)
		}
		w.Header().Set("Content-Type", "text/xml")
		fmt.Fprint(w, body)
	}))
	t.Cleanup(m.srv.Close)
	return m
}

func namecheapDefaultResponse(cmd string) string {
	switch cmd {
	case "namecheap.users.getBalances":
		return `<?xml version="1.0" encoding="UTF-8"?>
<ApiResponse Status="OK" xmlns="http://api.namecheap.com/xml.response">
  <Errors/><RequestedCommand>namecheap.users.getBalances</RequestedCommand>
  <CommandResponse Type="namecheap.users.getBalances">
    <UserGetBalancesResult Currency="USD" AvailableBalance="100.00"/>
  </CommandResponse><Server>SERVER1</Server><GMTTimeDifference>+5</GMTTimeDifference><ExecutionTime>0.01</ExecutionTime>
</ApiResponse>`
	case "namecheap.domains.check":
		return `<?xml version="1.0" encoding="UTF-8"?>
<ApiResponse Status="OK" xmlns="http://api.namecheap.com/xml.response">
  <Errors/><RequestedCommand>namecheap.domains.check</RequestedCommand>
  <CommandResponse Type="namecheap.domains.check">
    <DomainCheckResult Domain="example.com" Available="true" IsPremiumName="false" PremiumRegistrationPrice="0"/>
  </CommandResponse><Server>SERVER1</Server><GMTTimeDifference>+5</GMTTimeDifference><ExecutionTime>0.01</ExecutionTime>
</ApiResponse>`
	case "namecheap.domains.create":
		return `<?xml version="1.0" encoding="UTF-8"?>
<ApiResponse Status="OK" xmlns="http://api.namecheap.com/xml.response">
  <Errors/><RequestedCommand>namecheap.domains.create</RequestedCommand>
  <CommandResponse Type="namecheap.domains.create">
    <DomainCreateResult Domain="example.com" Registered="true" ChargedAmount="10.98" DomainID="12345" OrderID="1001" TransactionID="2001" WhoisguardEnable="true" NonRealTimeDomain="false"/>
  </CommandResponse><Server>SERVER1</Server><GMTTimeDifference>+5</GMTTimeDifference><ExecutionTime>0.01</ExecutionTime>
</ApiResponse>`
	case "namecheap.domains.dns.getHosts":
		return `<?xml version="1.0" encoding="UTF-8"?>
<ApiResponse Status="OK" xmlns="http://api.namecheap.com/xml.response">
  <Errors/><RequestedCommand>namecheap.domains.dns.getHosts</RequestedCommand>
  <CommandResponse Type="namecheap.domains.dns.getHosts">
    <DomainDNSGetHostsResult Domain="example.com" IsUsingOurDNS="true">
      <host HostId="1" Name="@" Type="A" Address="1.2.3.4" MXPref="10" TTL="1800"/>
    </DomainDNSGetHostsResult>
  </CommandResponse><Server>SERVER1</Server><GMTTimeDifference>+5</GMTTimeDifference><ExecutionTime>0.01</ExecutionTime>
</ApiResponse>`
	case "namecheap.domains.dns.setHosts":
		return `<?xml version="1.0" encoding="UTF-8"?>
<ApiResponse Status="OK" xmlns="http://api.namecheap.com/xml.response">
  <Errors/><RequestedCommand>namecheap.domains.dns.setHosts</RequestedCommand>
  <CommandResponse Type="namecheap.domains.dns.setHosts">
    <DomainDNSSetHostsResult Domain="example.com" IsSuccess="true"/>
  </CommandResponse><Server>SERVER1</Server><GMTTimeDifference>+5</GMTTimeDifference><ExecutionTime>0.01</ExecutionTime>
</ApiResponse>`
	default:
		return `<?xml version="1.0" encoding="UTF-8"?>
<ApiResponse Status="ERROR" xmlns="http://api.namecheap.com/xml.response">
  <Errors><Error Number="1011102">Parameter Command is Missing</Error></Errors>
</ApiResponse>`
	}
}

// ---- Mock GoDaddy REST API ----

// godaddyMock simulates the GoDaddy REST API.
type godaddyMock struct {
	srv        *httptest.Server
	mu         sync.Mutex
	requests   []string
	overrideFn func(method, path string) (int, any)
}

func newGoDaddyMock(t *testing.T) *godaddyMock {
	t.Helper()
	m := &godaddyMock{}
	m.srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		m.mu.Lock()
		m.requests = append(m.requests, r.Method+" "+r.URL.Path)
		fn := m.overrideFn
		m.mu.Unlock()

		w.Header().Set("Content-Type", "application/json")
		if fn != nil {
			code, body := fn(r.Method, r.URL.Path)
			w.WriteHeader(code)
			if body != nil {
				json.NewEncoder(w).Encode(body) //nolint:errcheck
			}
			return
		}
		// Default successful responses.
		switch r.URL.Path {
		case "/v1/domains/available":
			json.NewEncoder(w).Encode(map[string]any{"available": true, "currency": "USD", "period": 1, "price": 1099}) //nolint:errcheck
		case "/v1/domains":
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(map[string]any{"orderId": 12345}) //nolint:errcheck
		default:
			if r.Method == http.MethodGet && len(r.URL.Path) > 15 {
				// /v1/domains/{domain}/records
				json.NewEncoder(w).Encode([]map[string]any{}) //nolint:errcheck
				return
			}
			w.WriteHeader(http.StatusOK)
		}
	}))
	t.Cleanup(m.srv.Close)
	return m
}

// ---- Mock AWS API (EC2/Route53) ----

// awsMock simulates AWS API endpoints.
type awsMock struct {
	srv        *httptest.Server
	mu         sync.Mutex
	requests   []string
	overrideFn func(method, path string) (int, string)
}

func newAWSMock(t *testing.T) *awsMock {
	t.Helper()
	m := &awsMock{}
	m.srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		m.mu.Lock()
		m.requests = append(m.requests, r.Method+" "+r.URL.Path)
		fn := m.overrideFn
		m.mu.Unlock()

		w.Header().Set("Content-Type", "application/json")
		if fn != nil {
			code, body := fn(r.Method, r.URL.Path)
			w.WriteHeader(code)
			fmt.Fprint(w, body)
			return
		}
		// Default: return success for describe-instances style calls.
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, `{"Reservations":[]}`)
	}))
	t.Cleanup(m.srv.Close)
	return m
}

// ---- Mock Azure API ----

// azureMock simulates Azure API endpoints.
type azureMock struct {
	srv        *httptest.Server
	mu         sync.Mutex
	requests   []string
	overrideFn func(method, path string) (int, any)
}

func newAzureMock(t *testing.T) *azureMock {
	t.Helper()
	m := &azureMock{}
	m.srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		m.mu.Lock()
		m.requests = append(m.requests, r.Method+" "+r.URL.Path)
		fn := m.overrideFn
		m.mu.Unlock()

		w.Header().Set("Content-Type", "application/json")
		if fn != nil {
			code, body := fn(r.Method, r.URL.Path)
			w.WriteHeader(code)
			if body != nil {
				json.NewEncoder(w).Encode(body) //nolint:errcheck
			}
			return
		}
		// Default: empty VM list.
		json.NewEncoder(w).Encode(map[string]any{"value": []any{}}) //nolint:errcheck
	}))
	t.Cleanup(m.srv.Close)
	return m
}

// ---- Mock SMTP Server ----

// smtpMock records SMTP-related test probes.
// Since the SMTP test only makes a TCP+TLS+auth sequence, we model it as
// an HTTP endpoint that the SMTP tester calls for status.
// The actual smtpprofile service dials the host:port for real SMTP traffic,
// so for integration tests we verify the service endpoints and status fields
// rather than a live SMTP dial (which would require a real SMTP listener).
type smtpMock struct {
	srv  *httptest.Server
	host string
	port int
}

func newSMTPMock(t *testing.T) *smtpMock {
	t.Helper()
	// The SMTP mock is a simple HTTP-based recording server.
	// The actual SMTP profile service tests use configured host/port values
	// pointing to this server; connection test logic is validated separately
	// via service-level unit tests.
	m := &smtpMock{}
	m.srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(m.srv.Close)
	return m
}

// ---- Mock OIDC Provider ----

// oidcMock simulates an OIDC identity provider.
type oidcMock struct {
	srv      *httptest.Server
	issuer   string
	clientID string
	// jwtKey is a shared secret for signing mock ID tokens.
	// (We use the mock's base URL as a stub — actual token validation
	// is not exercised since the OIDC callback requires a real browser flow.)
	mu         sync.Mutex
	requests   []string
	overrideFn func(path string) (int, any)
}

func newOIDCMock(t *testing.T) *oidcMock {
	t.Helper()
	m := &oidcMock{}
	m.srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		m.mu.Lock()
		m.requests = append(m.requests, r.URL.Path)
		fn := m.overrideFn
		m.mu.Unlock()

		w.Header().Set("Content-Type", "application/json")
		if fn != nil {
			code, body := fn(r.URL.Path)
			w.WriteHeader(code)
			if body != nil {
				json.NewEncoder(w).Encode(body) //nolint:errcheck
			}
			return
		}
		// Default OIDC discovery document.
		switch r.URL.Path {
		case "/.well-known/openid-configuration":
			json.NewEncoder(w).Encode(map[string]any{ //nolint:errcheck
				"issuer":                 m.srv.URL,
				"authorization_endpoint": m.srv.URL + "/authorize",
				"token_endpoint":         m.srv.URL + "/token",
				"userinfo_endpoint":      m.srv.URL + "/userinfo",
				"jwks_uri":               m.srv.URL + "/.well-known/jwks.json",
				"response_types_supported": []string{"code"},
				"subject_types_supported": []string{"public"},
				"id_token_signing_alg_values_supported": []string{"RS256"},
			})
		case "/.well-known/jwks.json":
			json.NewEncoder(w).Encode(map[string]any{"keys": []any{}}) //nolint:errcheck
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	m.issuer = m.srv.URL
	m.clientID = "test-client-id"
	t.Cleanup(m.srv.Close)
	return m
}

// ---- Mock FusionAuth Server ----

// fusionAuthMock simulates the FusionAuth API.
type fusionAuthMock struct {
	srv        *httptest.Server
	mu         sync.Mutex
	requests   []string
	overrideFn func(method, path string) (int, any)
}

func newFusionAuthMock(t *testing.T) *fusionAuthMock {
	t.Helper()
	m := &fusionAuthMock{}
	m.srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		m.mu.Lock()
		m.requests = append(m.requests, r.Method+" "+r.URL.Path)
		fn := m.overrideFn
		m.mu.Unlock()

		w.Header().Set("Content-Type", "application/json")
		if fn != nil {
			code, body := fn(r.Method, r.URL.Path)
			w.WriteHeader(code)
			if body != nil {
				json.NewEncoder(w).Encode(body) //nolint:errcheck
			}
			return
		}
		// Default: API ping success.
		switch r.URL.Path {
		case "/api/system-configuration":
			json.NewEncoder(w).Encode(map[string]any{"systemConfiguration": map[string]any{}}) //nolint:errcheck
		default:
			w.WriteHeader(http.StatusOK)
		}
	}))
	t.Cleanup(m.srv.Close)
	return m
}
