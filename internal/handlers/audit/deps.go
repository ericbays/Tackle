// Package audit provides HTTP handlers for the audit log query API.
package audit

import (
	"database/sql"

	auditsvc "tackle/internal/services/audit"
)

// Deps holds the shared dependencies for all audit log handlers.
type Deps struct {
	DB       *sql.DB
	AuditSvc *auditsvc.AuditService
	HMACSvc  *auditsvc.HMACService
}
