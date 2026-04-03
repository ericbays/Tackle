// Package internalapi provides HTTP handlers for the framework's internal API.
// These endpoints receive forwarded data from generated landing page applications.
package internalapi

import (
	"database/sql"

	"tackle/internal/repositories"
	auditsvc "tackle/internal/services/audit"
	credsvc "tackle/internal/services/credential"
	emaildeliverysvc "tackle/internal/services/emaildelivery"
	"tackle/internal/tracking"
)

// Deps holds dependencies for internal API handlers.
type Deps struct {
	DB               *sql.DB
	EventRepo        *repositories.CampaignTargetEventRepository
	BuildRepo        *repositories.LandingPageRepository
	AuditSvc         *auditsvc.AuditService
	CredSvc          *credsvc.Service
	EmailDeliverySvc *emaildeliverysvc.Service
	TokenSvc         *tracking.TokenService
}
