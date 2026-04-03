package providers

import (
	"fmt"

	"tackle/internal/providers/azuredns"
	"tackle/internal/providers/credentials"
	"tackle/internal/providers/godaddy"
	"tackle/internal/providers/namecheap"
	r53 "tackle/internal/providers/route53"
)

// ProviderType identifies a supported domain provider.
type ProviderType string

const (
	// ProviderTypeNamecheap is the Namecheap domain registrar.
	ProviderTypeNamecheap ProviderType = "namecheap"
	// ProviderTypeGoDaddy is the GoDaddy domain registrar.
	ProviderTypeGoDaddy ProviderType = "godaddy"
	// ProviderTypeRoute53 is the AWS Route 53 DNS service.
	ProviderTypeRoute53 ProviderType = "route53"
	// ProviderTypeAzureDNS is the Azure DNS service.
	ProviderTypeAzureDNS ProviderType = "azure_dns"
)

// ValidProviderType returns true if t is a recognized provider type.
func ValidProviderType(t ProviderType) bool {
	switch t {
	case ProviderTypeNamecheap, ProviderTypeGoDaddy, ProviderTypeRoute53, ProviderTypeAzureDNS:
		return true
	}
	return false
}

// ConnectionForTest holds the provider type and encrypted credentials needed to build a client.
// Callers decrypt credentials externally and pass the typed struct to NewClientFromDecrypted.
type ConnectionForTest struct {
	Type             ProviderType
	RatePerMinute    int
	NamecheapCreds   *credentials.NamecheapCredentials
	GoDaddyCreds     *credentials.GoDaddyCredentials
	Route53Creds     *credentials.Route53Credentials
	AzureDNSCreds    *credentials.AzureDNSCredentials
}

// NewClientFromDecrypted constructs the appropriate ProviderClient given decrypted credentials.
// Returns an error if the provider type is unknown or required credentials are missing.
func NewClientFromDecrypted(conn ConnectionForTest) (ProviderClient, error) {
	switch conn.Type {
	case ProviderTypeNamecheap:
		if conn.NamecheapCreds == nil {
			return nil, fmt.Errorf("factory: namecheap credentials are required")
		}
		return namecheap.NewClient(*conn.NamecheapCreds, conn.RatePerMinute), nil

	case ProviderTypeGoDaddy:
		if conn.GoDaddyCreds == nil {
			return nil, fmt.Errorf("factory: godaddy credentials are required")
		}
		return godaddy.NewClient(*conn.GoDaddyCreds, conn.RatePerMinute), nil

	case ProviderTypeRoute53:
		if conn.Route53Creds == nil {
			return nil, fmt.Errorf("factory: route53 credentials are required")
		}
		return r53.NewClient(*conn.Route53Creds, conn.RatePerMinute)

	case ProviderTypeAzureDNS:
		if conn.AzureDNSCreds == nil {
			return nil, fmt.Errorf("factory: azure_dns credentials are required")
		}
		return azuredns.NewClient(*conn.AzureDNSCreds, conn.RatePerMinute), nil

	default:
		return nil, fmt.Errorf("factory: unknown provider type %q", conn.Type)
	}
}
