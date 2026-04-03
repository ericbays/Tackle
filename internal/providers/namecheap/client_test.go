package namecheap

import (
	"strings"
	"testing"

	"tackle/internal/providers/credentials"
	"tackle/internal/providers/ratelimit"
)

func testCreds(clientIP string) credentials.NamecheapCredentials {
	return credentials.NamecheapCredentials{
		APIUser:  "testuser",
		APIKey:   "testkey",
		Username: "testlogin",
		ClientIP: clientIP,
	}
}

func newTestClient(ip string) *Client {
	return &Client{
		creds:       testCreds(ip),
		httpClient:  nil, // not used in unit tests below
		rateLimiter: ratelimit.NewRateLimiter(100),
	}
}

func TestParseXML_Success(t *testing.T) {
	data := []byte(`<ApiResponse Status="OK"><CommandResponse/></ApiResponse>`)
	var resp apiResponse
	if err := parseXML(data, &resp); err != nil {
		t.Fatalf("parseXML: %v", err)
	}
	if resp.Status != "OK" {
		t.Errorf("expected OK, got %q", resp.Status)
	}
}

func TestParseXML_ErrorWithCode(t *testing.T) {
	data := []byte(`<ApiResponse Status="ERROR"><Errors><Error Number="1011102">Invalid Key</Error></Errors></ApiResponse>`)
	var resp apiResponse
	if err := parseXML(data, &resp); err != nil {
		t.Fatalf("parseXML: %v", err)
	}
	if resp.Status != "ERROR" {
		t.Errorf("expected ERROR, got %q", resp.Status)
	}
	if len(resp.Errors) == 0 || resp.Errors[0].Number != "1011102" {
		t.Errorf("expected error 1011102, got %+v", resp.Errors)
	}
}

func TestTranslateError_InvalidAPIKey(t *testing.T) {
	c := newTestClient("1.2.3.4")
	err := c.translateError([]Error{{Number: "1011102", Message: "Invalid API Key"}})
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "API key is invalid") {
		t.Errorf("unexpected message: %v", err)
	}
}

func TestTranslateError_IPNotWhitelisted(t *testing.T) {
	c := newTestClient("5.6.7.8")
	err := c.translateError([]Error{{Number: "1011150", Message: "IP not whitelisted"}})
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "5.6.7.8") {
		t.Errorf("IP should appear in error message: %v", err)
	}
	if !strings.Contains(err.Error(), "whitelisted") {
		t.Errorf("expected whitelist message: %v", err)
	}
}

func TestTranslateError_AccountLocked(t *testing.T) {
	c := newTestClient("1.2.3.4")
	err := c.translateError([]Error{{Number: "1011306", Message: "Account locked"}})
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "locked") {
		t.Errorf("unexpected message: %v", err)
	}
}

func TestTranslateError_Unknown(t *testing.T) {
	c := newTestClient("1.2.3.4")
	err := c.translateError([]Error{{Number: "9999999", Message: "Some error"}})
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "9999999") {
		t.Errorf("error number should appear: %v", err)
	}
}

func TestTranslateError_NoErrors(t *testing.T) {
	c := newTestClient("1.2.3.4")
	err := c.translateError(nil)
	if err == nil {
		t.Fatal("expected error")
	}
}
