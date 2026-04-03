package authprovider

import (
	"context"
	"fmt"

	"tackle/internal/repositories"
)

// LocalAuthFunc is a function that attempts local authentication.
// Returns (userID, roleName, permissions, error).
type LocalAuthFunc func(ctx context.Context, username, password string) (string, string, []string, error)

// LoginRouter routes login attempts between local and LDAP auth based on the
// configured auth_order for the LDAP provider.
type LoginRouter struct {
	ldap         *LDAPProvider
	providerRepo *repositories.AuthProviderRepository
	provSvc      *ProvisioningService
	enc          interface {
		Decrypt([]byte, any) error
	}
}

// NewLoginRouter creates a LoginRouter.
func NewLoginRouter(
	ldap *LDAPProvider,
	providerRepo *repositories.AuthProviderRepository,
	provSvc *ProvisioningService,
	enc interface{ Decrypt([]byte, any) error },
) *LoginRouter {
	return &LoginRouter{ldap: ldap, providerRepo: providerRepo, provSvc: provSvc, enc: enc}
}

// RouteLogin attempts authentication against configured providers.
// It tries local auth and LDAP auth in the configured order.
// Always returns a generic error to prevent provider enumeration.
func (r *LoginRouter) RouteLogin(ctx context.Context, username, password string, localAuth LocalAuthFunc) (ProvisionedUser, string, error) {
	// Fetch LDAP providers.
	var ldapProviders []repositories.AuthProvider
	if r.providerRepo != nil {
		providers, err := r.providerRepo.GetByProviderType(ctx, repositories.AuthProviderLDAP)
		if err == nil {
			ldapProviders = providers
		}
	}

	// Determine auth order: use first enabled LDAP provider's setting (or default to local_first).
	authOrder := repositories.AuthOrderLocalFirst
	for _, p := range ldapProviders {
		if p.Enabled {
			authOrder = p.AuthOrder
			break
		}
	}

	type authResult struct {
		user  ProvisionedUser
		token string
		err   error
	}

	tryLocal := func() authResult {
		userID, roleName, perms, err := localAuth(ctx, username, password)
		if err != nil {
			return authResult{err: err}
		}
		return authResult{user: ProvisionedUser{
			UserID:      userID,
			RoleName:    roleName,
			Permissions: perms,
		}}
	}

	tryLDAP := func() authResult {
		for _, p := range ldapProviders {
			if !p.Enabled {
				continue
			}
			var cfg LDAPConfig
			if decErr := r.enc.Decrypt(p.Configuration, &cfg); decErr != nil {
				continue
			}
			claims, ldapErr := r.ldap.Authenticate(ctx, cfg, username, password)
			if ldapErr != nil {
				continue
			}
			provisioned, provErr := r.provSvc.ResolveExternalUser(ctx, p, claims)
			if provErr != nil {
				continue
			}
			return authResult{user: provisioned}
		}
		return authResult{err: fmt.Errorf("ldap authentication failed")}
	}

	var primary, secondary func() authResult
	if authOrder == repositories.AuthOrderLDAPFirst {
		primary, secondary = tryLDAP, tryLocal
	} else {
		primary, secondary = tryLocal, tryLDAP
	}

	if res := primary(); res.err == nil {
		return res.user, "local", nil
	}

	if len(ldapProviders) > 0 || authOrder == repositories.AuthOrderLocalFirst {
		if res := secondary(); res.err == nil {
			return res.user, "external", nil
		}
	}

	return ProvisionedUser{}, "", fmt.Errorf("invalid credentials")
}
