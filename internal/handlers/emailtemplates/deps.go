// Package emailtemplates provides HTTP handlers for email template management.
package emailtemplates

import (
	emaildeliverysvc "tackle/internal/services/emaildelivery"
	emailtmplsvc "tackle/internal/services/emailtemplate"
)

// Deps holds the handler dependencies.
type Deps struct {
	Svc              *emailtmplsvc.Service
	EmailDeliverySvc *emaildeliverysvc.Service  // Optional: for send-test functionality.
	AttachmentSvc    *emailtmplsvc.AttachmentService // Optional: for attachment management.
}
