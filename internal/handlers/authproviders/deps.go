// Package authproviders provides HTTP handlers for external auth provider management
// and OIDC/FusionAuth login flows.
package authproviders

import (
	"tackle/internal/services/authprovider"
	authsvc "tackle/internal/services/auth"
)

// Deps holds the shared dependencies for all auth provider handlers.
type Deps struct {
	Svc         *authprovider.Service
	ProvSvc     *authprovider.ProvisioningService
	LinkSvc     *authprovider.LinkingService
	JWTSvc      *authsvc.JWTService
	RefreshSvc  *authsvc.RefreshTokenService
}
