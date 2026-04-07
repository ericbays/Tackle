package azuredns

import (
	"context"
	"errors"
	"strings"
	"testing"

	"tackle/internal/providers/credentials"
)

type mockFetcher struct {
	err error
}

func (m *mockFetcher) List(ctx context.Context, subscriptionID, resourceGroup string) error {
	return m.err
}

func (m *mockFetcher) ListAll(ctx context.Context, subscriptionID, resourceGroup string) ([]string, error) {
	if m.err != nil {
		return nil, m.err
	}
	return []string{"example.com"}, nil
}

func testCreds() credentials.AzureDNSCredentials {
	return credentials.AzureDNSCredentials{
		TenantID:       "tenant-1234",
		ClientID:       "client-5678",
		ClientSecret:   "secret",
		SubscriptionID: "sub-0000",
		ResourceGroup:  "rg-test",
	}
}

func TestTestConnection_Success(t *testing.T) {
	c := newClientWithFetcher(testCreds(), &mockFetcher{}, 0)
	if err := c.TestConnection(); err != nil {
		t.Errorf("expected nil, got %v", err)
	}
}

func TestTestConnection_Error(t *testing.T) {
	c := newClientWithFetcher(testCreds(), &mockFetcher{err: errors.New("API failure")}, 0)
	err := c.TestConnection()
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestTranslateAzureError_Nil(t *testing.T) {
	if err := translateAzureError(nil); err != nil {
		t.Errorf("nil should return nil, got %v", err)
	}
}

func TestTranslateAzureError_InvalidClient(t *testing.T) {
	err := translateAzureError(errors.New("AADSTS70011: The provided value for the input parameter 'scope' is not valid"))
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "client ID or client secret") {
		t.Errorf("unexpected message: %v", err)
	}
}

func TestTranslateAzureError_TenantNotFound(t *testing.T) {
	err := translateAzureError(errors.New("AADSTS90002: Tenant 'fake-tenant' not found"))
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "tenant ID") {
		t.Errorf("unexpected message: %v", err)
	}
}

func TestTranslateAzureError_AuthorizationFailed(t *testing.T) {
	err := translateAzureError(errors.New("AuthorizationFailed: The client does not have authorization"))
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "permissions") {
		t.Errorf("unexpected message: %v", err)
	}
}

func TestTranslateAzureError_SubscriptionNotFound(t *testing.T) {
	err := translateAzureError(errors.New("SubscriptionNotFound"))
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "subscription") {
		t.Errorf("unexpected message: %v", err)
	}
}

func TestTranslateAzureError_ResourceGroupNotFound(t *testing.T) {
	err := translateAzureError(errors.New("ResourceGroupNotFound"))
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "resource group") {
		t.Errorf("unexpected message: %v", err)
	}
}
