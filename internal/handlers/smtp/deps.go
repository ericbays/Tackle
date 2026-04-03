// Package smtp provides HTTP handlers for SMTP profile management.
package smtp

import smtpsvc "tackle/internal/services/smtpprofile"

// Deps holds the handler dependencies.
type Deps struct {
	Svc *smtpsvc.Service
}
