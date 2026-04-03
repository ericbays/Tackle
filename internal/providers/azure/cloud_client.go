// Package azure provides cloud infrastructure operations for Microsoft Azure.
package azure

import (
	"context"
	"fmt"
	"strings"

	"github.com/Azure/azure-sdk-for-go/sdk/azidentity"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/dns/armdns"

	tacklecreds "tackle/internal/providers/credentials"
)

// CloudClient performs non-mutating Azure operations for credential validation and template field validation.
type CloudClient struct {
	cred           *azidentity.ClientSecretCredential
	subscriptionID string
}

// NewCloudClient creates an Azure CloudClient from the provided credentials.
func NewCloudClient(creds tacklecreds.AzureCredentials) (*CloudClient, error) {
	cred, err := azidentity.NewClientSecretCredential(
		creds.TenantID, creds.ClientID, creds.ClientSecret, nil,
	)
	if err != nil {
		return nil, fmt.Errorf("azure cloud client: create credential: %w", err)
	}
	return &CloudClient{cred: cred, subscriptionID: creds.SubscriptionID}, nil
}

// TestConnection verifies credentials by listing DNS zones in the subscription (non-mutating).
// Returns a descriptive error if credentials or subscription are invalid.
func (c *CloudClient) TestConnection(ctx context.Context) error {
	client, err := armdns.NewZonesClient(c.subscriptionID, c.cred, nil)
	if err != nil {
		return fmt.Errorf("azure: create DNS zones client: %w", err)
	}

	pager := client.NewListPager(nil)
	// Fetch exactly one page to validate credentials without enumerating everything.
	_, err = pager.NextPage(ctx)
	if err != nil {
		return classifyAzureError(err)
	}
	return nil
}

// ValidateRegion returns true if region is a recognized Azure region identifier (location name).
func (c *CloudClient) ValidateRegion(region string) bool {
	return isValidAzureRegion(region)
}

// ValidateInstanceSize returns true if size is a recognized Azure VM size identifier.
// Uses a built-in static list for fast validation.
func (c *CloudClient) ValidateInstanceSize(size string) bool {
	_, ok := validAzureVMSizes[strings.ToLower(size)]
	return ok
}

// ValidateImageReference checks whether an Azure image reference string is syntactically valid.
// Accepted formats: "publisher:offer:sku:version" or a full resource ID.
func (c *CloudClient) ValidateImageReference(imageRef string) bool {
	if strings.HasPrefix(imageRef, "/subscriptions/") {
		return true // resource ID — trust format
	}
	parts := strings.Split(imageRef, ":")
	return len(parts) == 4 && parts[0] != "" && parts[1] != "" && parts[2] != "" && parts[3] != ""
}

// classifyAzureError returns a human-readable error from an Azure SDK error.
func classifyAzureError(err error) error {
	msg := err.Error()
	switch {
	case strings.Contains(msg, "AADSTS70011") || strings.Contains(msg, "invalid_client"):
		return fmt.Errorf("invalid Azure client ID or client secret")
	case strings.Contains(msg, "AADSTS90002") || (strings.Contains(msg, "tenant") && strings.Contains(msg, "not found")):
		return fmt.Errorf("invalid Azure tenant ID")
	case strings.Contains(msg, "AuthorizationFailed"):
		return fmt.Errorf("insufficient Azure permissions: DNS Zone Reader role is required")
	case strings.Contains(msg, "SubscriptionNotFound"):
		return fmt.Errorf("Azure subscription not found: check subscription ID")
	case strings.Contains(msg, "no such host") || strings.Contains(msg, "dial"):
		return fmt.Errorf("cannot reach Azure API: check network connectivity")
	default:
		return fmt.Errorf("Azure API error: %w", err)
	}
}

// isValidAzureRegion returns true if region is a recognized Azure location name.
func isValidAzureRegion(region string) bool {
	_, ok := validAzureRegions[strings.ToLower(region)]
	return ok
}

var validAzureRegions = map[string]struct{}{
	"eastus": {}, "eastus2": {}, "westus": {}, "westus2": {}, "westus3": {},
	"centralus": {}, "northcentralus": {}, "southcentralus": {}, "westcentralus": {},
	"canadacentral": {}, "canadaeast": {},
	"brazilsouth": {}, "brazilsoutheast": {},
	"northeurope": {}, "westeurope": {},
	"uksouth": {}, "ukwest": {},
	"francecentral": {}, "francesouth": {},
	"germanywestcentral": {}, "germanynorth": {},
	"switzerlandnorth": {}, "switzerlandwest": {},
	"norwayeast": {}, "norwaywest": {},
	"swedencentral": {},
	"italynorth": {},
	"polandcentral": {},
	"eastasia": {}, "southeastasia": {},
	"australiaeast": {}, "australiasoutheast": {}, "australiacentral": {}, "australiacentral2": {},
	"japaneast": {}, "japanwest": {},
	"koreacentral": {}, "koreasouth": {},
	"centralindia": {}, "southindia": {}, "westindia": {},
	"uaenorth": {}, "uaecentral": {},
	"southafricanorth": {}, "southafricawest": {},
	"qatarcentral": {},
	"israelcentral": {},
	"mexicocentral": {},
}

var validAzureVMSizes = map[string]struct{}{
	// B-series (burstable) — common for phishing endpoints
	"standard_b1s": {}, "standard_b1ms": {}, "standard_b2s": {}, "standard_b2ms": {},
	"standard_b4ms": {}, "standard_b8ms": {},
	// D-series general purpose
	"standard_d2s_v3": {}, "standard_d4s_v3": {}, "standard_d8s_v3": {},
	"standard_d2s_v4": {}, "standard_d4s_v4": {}, "standard_d8s_v4": {},
	"standard_d2s_v5": {}, "standard_d4s_v5": {}, "standard_d8s_v5": {},
	"standard_d2as_v5": {}, "standard_d4as_v5": {},
	// F-series compute optimized
	"standard_f2s_v2": {}, "standard_f4s_v2": {}, "standard_f8s_v2": {},
	// A-series basic
	"standard_a1_v2": {}, "standard_a2_v2": {}, "standard_a4_v2": {},
}
