// Package notification provides HTTP handlers for the notification API.
package notification

import (
	"database/sql"

	authsvc "tackle/internal/services/auth"
	notifsvc "tackle/internal/services/notification"
)

// Deps holds shared dependencies for all notification handlers.
type Deps struct {
	DB       *sql.DB
	NotifSvc *notifsvc.NotificationService
	Hub      *notifsvc.Hub
	JWTSvc   *authsvc.JWTService
}
