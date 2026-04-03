// Package domainproviders provides HTTP handlers for domain provider connection endpoints.
package domainproviders

import (
	"tackle/internal/services/domainprovider"
)

// Deps holds shared dependencies for domain provider handlers.
type Deps struct {
	Svc *domainprovider.Service
}
