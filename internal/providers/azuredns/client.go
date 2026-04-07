// Package azuredns implements a client for the Azure DNS API using the Azure SDK for Go.
package azuredns

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/azidentity"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/dns/armdns"

	"tackle/internal/providers/ratelimit"
	"tackle/internal/providers/credentials"
)

const (
	defaultRateLimit = 60 // 60 requests/minute
	testTimeout      = 15 * time.Second
)

// zonesLister defines the subset of the Azure DNS zones client used by this package.
type zonesLister interface {
	NewListByResourceGroupPager(resourceGroupName string, options *armdns.ZonesClientListByResourceGroupOptions) *fakeOrRealPager
}

// fakeOrRealPager is a stand-in type; in production we use the real pager.
// In tests we inject a mock via the zonesFetcher interface below.
type fakeOrRealPager = interface{}

// zonesFetcher abstracts the Azure DNS ListByResourceGroup call for test injection.
type zonesFetcher interface {
	List(ctx context.Context, subscriptionID, resourceGroup string) error
	ListAll(ctx context.Context, subscriptionID, resourceGroup string) ([]string, error)
}

// azureZonesFetcher is the production implementation.
type azureZonesFetcher struct {
	creds credentials.AzureDNSCredentials
}

func (f *azureZonesFetcher) List(ctx context.Context, subscriptionID, resourceGroup string) error {
	credential, err := azidentity.NewClientSecretCredential(
		f.creds.TenantID,
		f.creds.ClientID,
		f.creds.ClientSecret,
		nil,
	)
	if err != nil {
		return translateAzureError(err)
	}

	client, err := armdns.NewZonesClient(subscriptionID, credential, nil)
	if err != nil {
		return fmt.Errorf("azure dns: create zones client: %w", err)
	}

	pager := client.NewListByResourceGroupPager(resourceGroup, &armdns.ZonesClientListByResourceGroupOptions{
		Top: toInt32Ptr(1),
	})

	_, err = pager.NextPage(ctx)
	if err != nil {
		return translateAzureError(err)
	}
	return nil
}

func (f *azureZonesFetcher) ListAll(ctx context.Context, subscriptionID, resourceGroup string) ([]string, error) {
	credential, err := azidentity.NewClientSecretCredential(
		f.creds.TenantID,
		f.creds.ClientID,
		f.creds.ClientSecret,
		nil,
	)
	if err != nil {
		return nil, translateAzureError(err)
	}

	client, err := armdns.NewZonesClient(subscriptionID, credential, nil)
	if err != nil {
		return nil, fmt.Errorf("azure dns: create zones client: %w", err)
	}

	pager := client.NewListByResourceGroupPager(resourceGroup, &armdns.ZonesClientListByResourceGroupOptions{})
	
	var domains []string
	for pager.More() {
		resp, err := pager.NextPage(ctx)
		if err != nil {
			return nil, translateAzureError(err)
		}
		for _, zone := range resp.Value {
			if zone.Name != nil {
				domains = append(domains, *zone.Name)
			}
		}
	}
	return domains, nil
}

// Client is an Azure DNS API client.
type Client struct {
	creds       credentials.AzureDNSCredentials
	fetcher     zonesFetcher
	rateLimiter *ratelimit.RateLimiter
}

// NewClient creates an Azure DNS client from the provided credentials.
// ratePerMinute overrides the default of 60 if > 0.
func NewClient(creds credentials.AzureDNSCredentials, ratePerMinute int) *Client {
	if ratePerMinute <= 0 {
		ratePerMinute = defaultRateLimit
	}
	return &Client{
		creds:       creds,
		fetcher:     &azureZonesFetcher{creds: creds},
		rateLimiter: ratelimit.NewRateLimiter(ratePerMinute),
	}
}

// newClientWithFetcher creates a Client with a mock fetcher (used in tests).
func newClientWithFetcher(creds credentials.AzureDNSCredentials, fetcher zonesFetcher, ratePerMinute int) *Client {
	if ratePerMinute <= 0 {
		ratePerMinute = defaultRateLimit
	}
	return &Client{
		creds:       creds,
		fetcher:     fetcher,
		rateLimiter: ratelimit.NewRateLimiter(ratePerMinute),
	}
}

// TestConnection validates credentials by listing DNS zones in the configured resource group.
// Returns nil on success or a descriptive, actionable error.
func (c *Client) TestConnection() error {
	c.rateLimiter.Wait()

	ctx, cancel := context.WithTimeout(context.Background(), testTimeout)
	defer cancel()

	return c.fetcher.List(ctx, c.creds.SubscriptionID, c.creds.ResourceGroup)
}

// ListDomains retrieves a list of all domains associated with the provider.
func (c *Client) ListDomains() ([]string, error) {
	c.rateLimiter.Wait()

	ctx, cancel := context.WithTimeout(context.Background(), testTimeout)
	defer cancel()

	return c.fetcher.ListAll(ctx, c.creds.SubscriptionID, c.creds.ResourceGroup)
}

// translateAzureError converts Azure SDK errors into actionable messages.
func translateAzureError(err error) error {
	if err == nil {
		return nil
	}
	msg := err.Error()

	switch {
	case containsAny(msg, "AADSTS70011", "invalid_client"):
		return fmt.Errorf("azure dns: client ID or client secret is invalid. Verify your service principal credentials")
	case containsAny(msg, "AADSTS90002", "tenant not found"):
		return fmt.Errorf("azure dns: tenant ID not found. Verify the tenant ID in your Azure DNS configuration")
	case containsAny(msg, "AuthorizationFailed", "does not have authorization"):
		return fmt.Errorf("azure dns: service principal lacks required permissions on the subscription or resource group. Assign the DNS Zone Contributor role")
	case containsAny(msg, "SubscriptionNotFound"):
		return fmt.Errorf("azure dns: subscription ID not found. Verify the subscription ID in your configuration")
	case containsAny(msg, "ResourceGroupNotFound"):
		return fmt.Errorf("azure dns: resource group not found. Verify the resource group name in your configuration")
	default:
		return fmt.Errorf("azure dns: API error: %s", sanitize(msg))
	}
}

// containsAny returns true if s contains any of the substrings.
func containsAny(s string, substrings ...string) bool {
	for _, sub := range substrings {
		if strings.Contains(s, sub) {
			return true
		}
	}
	return false
}

// sanitize truncates long error messages to prevent log flooding.
func sanitize(msg string) string {
	if len(msg) > 200 {
		return msg[:200] + "..."
	}
	return msg
}

func toInt32Ptr(i int32) *int32 { return &i }
