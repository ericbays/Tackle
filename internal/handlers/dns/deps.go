// Package dns provides HTTP handlers for DNS record management and email authentication.
package dns

import dnssvc "tackle/internal/services/dns"

// Deps holds the handler dependencies.
type Deps struct {
	Svc *dnssvc.Service
}
