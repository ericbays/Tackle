// Package auth provides HTTP handlers for authentication endpoints.
package auth

import (
	"database/sql"

	"tackle/internal/services/audit"
	authsvc "tackle/internal/services/auth"
	"tackle/internal/services/authprovider"
)

// Deps holds the shared dependencies for all auth handlers.
type Deps struct {
	DB                  *sql.DB
	JWTSvc              *authsvc.JWTService
	RefreshSvc          *authsvc.RefreshTokenService
	Blacklist           *authsvc.TokenBlacklist
	HistoryChecker      *authsvc.HistoryChecker
	RateLimiter         *authsvc.RateLimiter
	Policy              authsvc.PasswordPolicy
	AuditSvc            *audit.AuditService
	LoginRouter         *authprovider.LoginRouter
	SessionConfigLoader *authsvc.SessionConfigLoader
}
