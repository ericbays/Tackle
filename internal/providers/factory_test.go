package providers

import (
	"testing"

	"tackle/internal/providers/credentials"
)

func TestNewClientFromDecrypted_Namecheap(t *testing.T) {
	c, err := NewClientFromDecrypted(ConnectionForTest{
		Type:           ProviderTypeNamecheap,
		RatePerMinute:  20,
		NamecheapCreds: &credentials.NamecheapCredentials{APIUser: "u", APIKey: "k", Username: "u", ClientIP: "1.2.3.4"},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if c == nil {
		t.Fatal("expected non-nil client")
	}
}

func TestNewClientFromDecrypted_GoDaddy(t *testing.T) {
	c, err := NewClientFromDecrypted(ConnectionForTest{
		Type:          ProviderTypeGoDaddy,
		RatePerMinute: 60,
		GoDaddyCreds:  &credentials.GoDaddyCredentials{APIKey: "k", APISecret: "s", Environment: credentials.GoDaddyProduction},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if c == nil {
		t.Fatal("expected non-nil client")
	}
}

func TestNewClientFromDecrypted_AzureDNS(t *testing.T) {
	c, err := NewClientFromDecrypted(ConnectionForTest{
		Type:          ProviderTypeAzureDNS,
		AzureDNSCreds: &credentials.AzureDNSCredentials{TenantID: "t", ClientID: "c", ClientSecret: "s", SubscriptionID: "sub", ResourceGroup: "rg"},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if c == nil {
		t.Fatal("expected non-nil client")
	}
}

func TestNewClientFromDecrypted_MissingCreds(t *testing.T) {
	_, err := NewClientFromDecrypted(ConnectionForTest{Type: ProviderTypeNamecheap})
	if err == nil {
		t.Fatal("expected error for missing creds")
	}
}

func TestNewClientFromDecrypted_UnknownType(t *testing.T) {
	_, err := NewClientFromDecrypted(ConnectionForTest{Type: "unknown"})
	if err == nil {
		t.Fatal("expected error for unknown provider")
	}
}

func TestValidProviderType(t *testing.T) {
	cases := []struct {
		t     ProviderType
		valid bool
	}{
		{ProviderTypeNamecheap, true},
		{ProviderTypeGoDaddy, true},
		{ProviderTypeRoute53, true},
		{ProviderTypeAzureDNS, true},
		{"", false},
		{"unknown", false},
	}
	for _, tc := range cases {
		if got := ValidProviderType(tc.t); got != tc.valid {
			t.Errorf("ValidProviderType(%q) = %v, want %v", tc.t, got, tc.valid)
		}
	}
}
