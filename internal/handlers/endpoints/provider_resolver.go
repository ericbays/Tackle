package endpoints

import (
	"context"
	"database/sql"
	"fmt"

	"tackle/internal/endpoint/cloud"
	"tackle/internal/providers/credentials"
	"tackle/internal/repositories"
)

// DBProviderResolver resolves cloud providers by looking up the endpoint's cloud credential
// in the database, decrypting it, and instantiating the appropriate provider.
type DBProviderResolver struct {
	endpointRepo  *repositories.PhishingEndpointRepository
	cloudCredRepo *repositories.CloudCredentialRepository
	credEncSvc    *credentials.EncryptionService
}

// NewDBProviderResolver creates a DBProviderResolver.
func NewDBProviderResolver(
	endpointRepo *repositories.PhishingEndpointRepository,
	cloudCredRepo *repositories.CloudCredentialRepository,
	credEncSvc *credentials.EncryptionService,
) *DBProviderResolver {
	return &DBProviderResolver{
		endpointRepo:  endpointRepo,
		cloudCredRepo: cloudCredRepo,
		credEncSvc:    credEncSvc,
	}
}

// ResolveForEndpoint returns a cloud.Provider for the given endpoint by looking up its
// cloud credential, decrypting it, and creating the appropriate provider instance.
func (r *DBProviderResolver) ResolveForEndpoint(endpointID string) (cloud.Provider, error) {
	ctx := context.Background()

	ep, err := r.endpointRepo.GetByID(ctx, endpointID)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("endpoint not found: %s", endpointID)
		}
		return nil, fmt.Errorf("resolve provider: get endpoint: %w", err)
	}

	if ep.CloudCredentialID == nil || *ep.CloudCredentialID == "" {
		return nil, fmt.Errorf("endpoint %s has no cloud credential configured", endpointID)
	}

	cred, err := r.cloudCredRepo.GetByID(ctx, *ep.CloudCredentialID)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("cloud credential not found: %s", *ep.CloudCredentialID)
		}
		return nil, fmt.Errorf("resolve provider: get credential: %w", err)
	}

	provider, err := cloud.NewProviderFromCredential(ctx, cred, r.credEncSvc, ep.Region, "")
	if err != nil {
		return nil, fmt.Errorf("resolve provider: create provider: %w", err)
	}

	return provider, nil
}
