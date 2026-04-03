package credentials

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestNamecheapCredentials_MaskedJSON(t *testing.T) {
	c := NamecheapCredentials{
		APIUser:  "myuser",
		APIKey:   "supersecret",
		Username: "myusername",
		ClientIP: "1.2.3.4",
	}
	b, err := json.Marshal(c)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	s := string(b)
	if strings.Contains(s, "myuser") || strings.Contains(s, "supersecret") || strings.Contains(s, "myusername") {
		t.Errorf("sensitive data visible in JSON: %s", s)
	}
	if !strings.Contains(s, "1.2.3.4") {
		t.Errorf("client_ip should be visible: %s", s)
	}
}

func TestNamecheapCredentials_String(t *testing.T) {
	c := NamecheapCredentials{APIKey: "secret"}
	if strings.Contains(c.String(), "secret") {
		t.Error("String() must not contain credential values")
	}
}

func TestGoDaddyCredentials_MaskedJSON(t *testing.T) {
	c := GoDaddyCredentials{
		APIKey:      "key123",
		APISecret:   "sec456",
		Environment: GoDaddyProduction,
	}
	b, err := json.Marshal(c)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	s := string(b)
	if strings.Contains(s, "key123") || strings.Contains(s, "sec456") {
		t.Errorf("sensitive data visible in JSON: %s", s)
	}
	if !strings.Contains(s, "production") {
		t.Errorf("environment should be visible: %s", s)
	}
}

func TestGoDaddyCredentials_String(t *testing.T) {
	c := GoDaddyCredentials{APISecret: "secret"}
	if strings.Contains(c.String(), "secret") {
		t.Error("String() must not contain credential values")
	}
}

func TestRoute53Credentials_MaskedJSON(t *testing.T) {
	c := Route53Credentials{
		AccessKeyID:     "AKIAFAKE",
		SecretAccessKey: "fakesecret",
		Region:          "us-east-1",
		IAMRoleARN:      "arn:aws:iam::123456789012:role/MyRole",
	}
	b, err := json.Marshal(c)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	s := string(b)
	if strings.Contains(s, "AKIAFAKE") || strings.Contains(s, "fakesecret") {
		t.Errorf("sensitive data visible in JSON: %s", s)
	}
	if !strings.Contains(s, "us-east-1") {
		t.Errorf("region should be visible: %s", s)
	}
}

func TestRoute53Credentials_String(t *testing.T) {
	c := Route53Credentials{SecretAccessKey: "secret"}
	if strings.Contains(c.String(), "secret") {
		t.Error("String() must not contain credential values")
	}
}

func TestAzureDNSCredentials_MaskedJSON(t *testing.T) {
	c := AzureDNSCredentials{
		TenantID:       "tenant-uuid",
		ClientID:       "client-uuid",
		ClientSecret:   "supersecret",
		SubscriptionID: "sub-uuid",
		ResourceGroup:  "my-rg",
	}
	b, err := json.Marshal(c)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	s := string(b)
	if strings.Contains(s, "tenant-uuid") || strings.Contains(s, "client-uuid") || strings.Contains(s, "supersecret") {
		t.Errorf("sensitive data visible in JSON: %s", s)
	}
	if !strings.Contains(s, "sub-uuid") || !strings.Contains(s, "my-rg") {
		t.Errorf("non-sensitive fields should be visible: %s", s)
	}
}

func TestAzureDNSCredentials_String(t *testing.T) {
	c := AzureDNSCredentials{ClientSecret: "secret"}
	if strings.Contains(c.String(), "secret") {
		t.Error("String() must not contain credential values")
	}
}
