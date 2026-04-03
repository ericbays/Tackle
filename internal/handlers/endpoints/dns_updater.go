package endpoints

import (
	"context"
	"fmt"
	"net"

	"tackle/internal/providers"
	"tackle/internal/providers/credentials"
	dnsiface "tackle/internal/providers/dns"
	"tackle/internal/repositories"
)

// LazyDNSUpdater implements endpoint.DNSUpdater by resolving the DNS provider
// on demand from domain provider connections in the database.
type LazyDNSUpdater struct {
	providerRepo *repositories.DomainProviderRepository
	credEncSvc   *credentials.EncryptionService
}

// NewLazyDNSUpdater creates a LazyDNSUpdater.
func NewLazyDNSUpdater(
	providerRepo *repositories.DomainProviderRepository,
	credEncSvc *credentials.EncryptionService,
) *LazyDNSUpdater {
	return &LazyDNSUpdater{
		providerRepo: providerRepo,
		credEncSvc:   credEncSvc,
	}
}

// CreateARecord creates an A record in the zone using the first available DNS provider.
func (u *LazyDNSUpdater) CreateARecord(ctx context.Context, zone, subdomain, ip string) error {
	provider, err := u.resolveProvider(ctx)
	if err != nil {
		return fmt.Errorf("dns updater: create A record: %w", err)
	}
	_, err = provider.CreateRecord(ctx, zone, dnsiface.Record{
		Type:  dnsiface.RecordTypeA,
		Name:  subdomain,
		Value: ip,
		TTL:   300,
	})
	return err
}

// DeleteARecord removes an A record from the zone.
func (u *LazyDNSUpdater) DeleteARecord(ctx context.Context, zone, subdomain string) error {
	provider, err := u.resolveProvider(ctx)
	if err != nil {
		return fmt.Errorf("dns updater: delete A record: %w", err)
	}
	records, err := provider.ListRecords(ctx, zone)
	if err != nil {
		return fmt.Errorf("dns updater: list records for deletion: %w", err)
	}
	for _, r := range records {
		if r.Type == dnsiface.RecordTypeA && r.Name == subdomain {
			return provider.DeleteRecord(ctx, zone, r.ID)
		}
	}
	return nil // Record doesn't exist — nothing to delete.
}

// CheckPropagation verifies DNS resolution matches the expected IP.
func (u *LazyDNSUpdater) CheckPropagation(ctx context.Context, domain, expectedIP string) (bool, error) {
	ips, err := net.DefaultResolver.LookupHost(ctx, domain)
	if err != nil {
		return false, nil // Not yet propagated.
	}
	for _, ip := range ips {
		if ip == expectedIP {
			return true, nil
		}
	}
	return false, nil
}

// resolveProvider finds the first available DNS provider connection and creates a client.
func (u *LazyDNSUpdater) resolveProvider(ctx context.Context) (dnsiface.Provider, error) {
	conns, err := u.providerRepo.List(ctx, repositories.DomainProviderFilters{})
	if err != nil {
		return nil, fmt.Errorf("list domain provider connections: %w", err)
	}

	for _, conn := range conns {
		p, err := providers.BuildDNSProviderFromEncrypted(
			conn.ProviderType, conn.CredentialsEncrypted, u.credEncSvc, 60,
		)
		if err != nil {
			continue // Skip connections that can't produce a DNS provider.
		}
		return p, nil
	}

	return nil, fmt.Errorf("no DNS provider connection configured")
}
