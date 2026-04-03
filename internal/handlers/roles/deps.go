// Package roles provides HTTP handlers for role management endpoints.
package roles

import (
	"database/sql"

	"tackle/internal/services/audit"
)

// Deps holds shared dependencies for role handlers.
type Deps struct {
	DB       *sql.DB
	AuditSvc *audit.AuditService
}
