// Package providers contains factory functions for building cloud provider clients.
package providers

import (
	"context"
	"fmt"

	awscloud "tackle/internal/providers/aws"
	azurecloud "tackle/internal/providers/azure"
	"tackle/internal/providers/credentials"
	proxmoxcloud "tackle/internal/providers/proxmox"
	"tackle/internal/repositories"
)

// CloudClient defines the interface for cloud provider operations needed by the credential
// and template services.
type CloudClient interface {
	// TestConnection performs a non-mutating API call to verify credentials.
	TestConnection(ctx context.Context) error
	// ValidateRegion returns true if the region string is valid for this provider.
	ValidateRegion(region string) bool
	// ValidateInstanceSize returns true if the instance size is valid for this provider.
	ValidateInstanceSize(size string) bool
}

// NewCloudClientFromCredential decrypts the stored credential and returns the appropriate CloudClient.
func NewCloudClientFromCredential(ctx context.Context, cred repositories.CloudCredential, encSvc *credentials.EncryptionService) (CloudClient, error) {
	switch cred.ProviderType {
	case repositories.CloudProviderAWS:
		var awsCreds credentials.AWSCredentials
		if err := encSvc.Decrypt(cred.CredentialsEncrypted, &awsCreds); err != nil {
			return nil, fmt.Errorf("cloud factory: decrypt aws credentials: %w", err)
		}
		client, err := awscloud.NewCloudClient(ctx, awsCreds, cred.DefaultRegion)
		if err != nil {
			return nil, fmt.Errorf("cloud factory: create aws client: %w", err)
		}
		return client, nil

	case repositories.CloudProviderAzure:
		var azCreds credentials.AzureCredentials
		if err := encSvc.Decrypt(cred.CredentialsEncrypted, &azCreds); err != nil {
			return nil, fmt.Errorf("cloud factory: decrypt azure credentials: %w", err)
		}
		client, err := azurecloud.NewCloudClient(azCreds)
		if err != nil {
			return nil, fmt.Errorf("cloud factory: create azure client: %w", err)
		}
		return client, nil

	case repositories.CloudProviderProxmox:
		var pxCreds credentials.ProxmoxCredentials
		if err := encSvc.Decrypt(cred.CredentialsEncrypted, &pxCreds); err != nil {
			return nil, fmt.Errorf("cloud factory: decrypt proxmox credentials: %w", err)
		}
		client, err := proxmoxcloud.NewCloudClient(pxCreds)
		if err != nil {
			return nil, fmt.Errorf("cloud factory: create proxmox client: %w", err)
		}
		return client, nil

	default:
		return nil, fmt.Errorf("cloud factory: unknown provider type %q", cred.ProviderType)
	}
}
