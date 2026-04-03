package authprovider

import (
	"context"
	"fmt"
	"testing"

	"tackle/internal/repositories"
)

// mockProviderRepo implements just the subset of AuthProviderRepository we need.
type mockProviderRepo struct {
	providers []repositories.AuthProvider
	err       error
}

func (m *mockProviderRepo) GetByProviderType(ctx context.Context, pt repositories.AuthProviderType) ([]repositories.AuthProvider, error) {
	if m.err != nil {
		return nil, m.err
	}
	return m.providers, nil
}

// mockDecryptor satisfies the enc interface used by LoginRouter.
type mockDecryptor struct{}

func (m *mockDecryptor) Decrypt(data []byte, target any) error {
	return nil
}

func TestRouteLogin_LocalOnly(t *testing.T) {
	repo := &mockProviderRepo{providers: nil}
	router := NewLoginRouter(nil, nil, nil, &mockDecryptor{})
	// Override the repo via internal access — use the RouteLogin method directly
	// with no LDAP providers.
	_ = repo

	localAuth := func(ctx context.Context, username, password string) (string, string, []string, error) {
		if username == "admin" && password == "secret" {
			return "uid-1", "admin", []string{"users:read"}, nil
		}
		return "", "", nil, fmt.Errorf("bad credentials")
	}

	// Test successful local auth.
	user, prov, err := router.RouteLogin(context.Background(), "admin", "secret", localAuth)
	if err != nil {
		t.Fatalf("expected success, got: %v", err)
	}
	if user.UserID != "uid-1" {
		t.Errorf("expected user ID uid-1, got %s", user.UserID)
	}
	if prov != "local" {
		t.Errorf("expected provider local, got %s", prov)
	}

	// Test failed local auth with no LDAP.
	_, _, err = router.RouteLogin(context.Background(), "admin", "wrong", localAuth)
	if err == nil {
		t.Fatal("expected error for bad credentials")
	}
}

func TestRouteLogin_LocalAuthRejectedLockedAccount(t *testing.T) {
	router := NewLoginRouter(nil, nil, nil, &mockDecryptor{})

	localAuth := func(ctx context.Context, username, password string) (string, string, []string, error) {
		return "", "", nil, fmt.Errorf("account locked")
	}

	_, _, err := router.RouteLogin(context.Background(), "locked-user", "secret", localAuth)
	if err == nil {
		t.Fatal("expected error for locked account")
	}
}
