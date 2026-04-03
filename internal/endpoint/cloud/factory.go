package cloud

import (
	"context"
	"fmt"

	"tackle/internal/providers/credentials"
	"tackle/internal/repositories"
)

// NewProviderFromCredential decrypts a stored cloud credential and returns the appropriate
// endpoint Provider for provisioning operations.
func NewProviderFromCredential(ctx context.Context, cred repositories.CloudCredential, encSvc *credentials.EncryptionService, region, resourceGroup string) (Provider, error) {
	switch cred.ProviderType {
	case repositories.CloudProviderAWS:
		var awsCreds credentials.AWSCredentials
		if err := encSvc.Decrypt(cred.CredentialsEncrypted, &awsCreds); err != nil {
			return nil, fmt.Errorf("endpoint cloud factory: decrypt aws: %w", err)
		}
		r := region
		if r == "" {
			r = cred.DefaultRegion
		}
		return NewAWSProvider(ctx, awsCreds, r)

	case repositories.CloudProviderAzure:
		var azCreds credentials.AzureCredentials
		if err := encSvc.Decrypt(cred.CredentialsEncrypted, &azCreds); err != nil {
			return nil, fmt.Errorf("endpoint cloud factory: decrypt azure: %w", err)
		}
		r := region
		if r == "" {
			r = cred.DefaultRegion
		}
		return NewAzureProvider(azCreds, r, resourceGroup)

	case repositories.CloudProviderProxmox:
		var pxCreds credentials.ProxmoxCredentials
		if err := encSvc.Decrypt(cred.CredentialsEncrypted, &pxCreds); err != nil {
			return nil, fmt.Errorf("endpoint cloud factory: decrypt proxmox: %w", err)
		}
		return NewProxmoxProvider(pxCreds)

	default:
		return nil, fmt.Errorf("endpoint cloud factory: unsupported provider type %q", cred.ProviderType)
	}
}
