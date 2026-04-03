// Package setup provides HTTP handlers for the initial admin setup flow.
package setup

import (
	"database/sql"

	"tackle/internal/services/audit"
	authsvc "tackle/internal/services/auth"
)

// Deps holds shared dependencies for all setup handlers.
type Deps struct {
	DB         *sql.DB
	JWTSvc     *authsvc.JWTService
	RefreshSvc *authsvc.RefreshTokenService
	Policy     authsvc.PasswordPolicy
	AuditSvc   *audit.AuditService
}
