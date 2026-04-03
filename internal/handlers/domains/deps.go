// Package domains provides HTTP handlers for domain lifecycle management.
package domains

import domainsvc "tackle/internal/services/domain"

// Deps holds the handler dependencies.
type Deps struct {
	Svc *domainsvc.Service
}
