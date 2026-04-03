package providers

import (
	"fmt"

	dnsiface "tackle/internal/providers/dns"
	"tackle/internal/providers/azuredns"
	"tackle/internal/providers/credentials"
	"tackle/internal/providers/godaddy"
	"tackle/internal/providers/namecheap"
	r53 "tackle/internal/providers/route53"
)

// NewDNSClientFromDecrypted constructs the appropriate dns.Provider given decrypted credentials.
// Only providers that implement the DNS provider interface are supported.
func NewDNSClientFromDecrypted(conn ConnectionForTest) (dnsiface.Provider, error) {
	switch conn.Type {
	case ProviderTypeNamecheap:
		if conn.NamecheapCreds == nil {
			return nil, fmt.Errorf("dns factory: namecheap credentials are required")
		}
		return namecheap.NewClient(*conn.NamecheapCreds, conn.RatePerMinute), nil

	case ProviderTypeGoDaddy:
		if conn.GoDaddyCreds == nil {
			return nil, fmt.Errorf("dns factory: godaddy credentials are required")
		}
		return godaddy.NewClient(*conn.GoDaddyCreds, conn.RatePerMinute), nil

	case ProviderTypeRoute53:
		if conn.Route53Creds == nil {
			return nil, fmt.Errorf("dns factory: route53 credentials are required")
		}
		c, err := r53.NewClient(*conn.Route53Creds, conn.RatePerMinute)
		if err != nil {
			return nil, err
		}
		return c, nil

	case ProviderTypeAzureDNS:
		if conn.AzureDNSCreds == nil {
			return nil, fmt.Errorf("dns factory: azure_dns credentials are required")
		}
		return azuredns.NewClient(*conn.AzureDNSCreds, conn.RatePerMinute), nil

	default:
		return nil, fmt.Errorf("dns factory: unknown provider type %q", conn.Type)
	}
}

// BuildDNSProviderFromEncrypted decrypts the given credential blob and builds a dns.Provider.
// providerType is the raw string from the domain_provider_connections table.
func BuildDNSProviderFromEncrypted(providerType string, encryptedCreds []byte, enc *credentials.EncryptionService, ratePerMinute int) (dnsiface.Provider, error) {
	conn := ConnectionForTest{
		Type:          ProviderType(providerType),
		RatePerMinute: ratePerMinute,
	}

	switch ProviderType(providerType) {
	case ProviderTypeNamecheap:
		var c credentials.NamecheapCredentials
		if err := enc.Decrypt(encryptedCreds, &c); err != nil {
			return nil, fmt.Errorf("dns factory: decrypt namecheap credentials: %w", err)
		}
		conn.NamecheapCreds = &c

	case ProviderTypeGoDaddy:
		var c credentials.GoDaddyCredentials
		if err := enc.Decrypt(encryptedCreds, &c); err != nil {
			return nil, fmt.Errorf("dns factory: decrypt godaddy credentials: %w", err)
		}
		conn.GoDaddyCreds = &c

	case ProviderTypeRoute53:
		var c credentials.Route53Credentials
		if err := enc.Decrypt(encryptedCreds, &c); err != nil {
			return nil, fmt.Errorf("dns factory: decrypt route53 credentials: %w", err)
		}
		conn.Route53Creds = &c

	case ProviderTypeAzureDNS:
		var c credentials.AzureDNSCredentials
		if err := enc.Decrypt(encryptedCreds, &c); err != nil {
			return nil, fmt.Errorf("dns factory: decrypt azure_dns credentials: %w", err)
		}
		conn.AzureDNSCreds = &c

	default:
		return nil, fmt.Errorf("dns factory: unknown provider type %q", providerType)
	}

	return NewDNSClientFromDecrypted(conn)
}
