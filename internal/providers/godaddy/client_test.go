package godaddy

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"tackle/internal/providers/credentials"
)

func testCreds(env credentials.GoDaddyEnvironment) credentials.GoDaddyCredentials {
	return credentials.GoDaddyCredentials{
		APIKey:      "testkey",
		APISecret:   "testsecret",
		Environment: env,
	}
}

func newTestClientWithServer(t *testing.T, env credentials.GoDaddyEnvironment, handler http.Handler) (*Client, *httptest.Server) {
	t.Helper()
	srv := httptest.NewTLSServer(handler)
	c := NewClient(testCreds(env), 0)
	c.httpClient = srv.Client()
	c.baseURL = srv.URL
	return c, srv
}

func TestTestConnection_Success(t *testing.T) {
	c, srv := newTestClientWithServer(t, credentials.GoDaddyProduction, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") == "" {
			http.Error(w, "no auth", http.StatusUnauthorized)
			return
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`[]`))
	}))
	defer srv.Close()
	if err := c.TestConnection(); err != nil {
		t.Errorf("expected nil, got %v", err)
	}
}

func TestTestConnection_Unauthorized(t *testing.T) {
	c, srv := newTestClientWithServer(t, credentials.GoDaddyProduction, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"code":"UNABLE_TO_AUTHENTICATE","message":"Unauthorized"}`))
	}))
	defer srv.Close()
	err := c.TestConnection()
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "authentication failed") {
		t.Errorf("unexpected message: %v", err)
	}
}

func TestTestConnection_Forbidden(t *testing.T) {
	c, srv := newTestClientWithServer(t, credentials.GoDaddyProduction, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
	}))
	defer srv.Close()
	err := c.TestConnection()
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "access denied") {
		t.Errorf("unexpected message: %v", err)
	}
}

func TestTestConnection_RateLimit(t *testing.T) {
	c, srv := newTestClientWithServer(t, credentials.GoDaddyProduction, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusTooManyRequests)
	}))
	defer srv.Close()
	err := c.TestConnection()
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "rate limit") {
		t.Errorf("unexpected message: %v", err)
	}
}

func TestNewClient_OTEBaseURL(t *testing.T) {
	c := NewClient(testCreds(credentials.GoDaddyOTE), 0)
	if c.baseURL != oteBaseURL {
		t.Errorf("expected OTE base URL %q, got %q", oteBaseURL, c.baseURL)
	}
}

func TestNewClient_ProductionBaseURL(t *testing.T) {
	c := NewClient(testCreds(credentials.GoDaddyProduction), 0)
	if c.baseURL != productionBaseURL {
		t.Errorf("expected production base URL %q, got %q", productionBaseURL, c.baseURL)
	}
}
