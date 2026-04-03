package middleware

import (
	"database/sql"
	"net/http"

	"tackle/internal/services/setup"
	"tackle/pkg/response"
)

// RequireSetupComplete blocks requests with 503 if initial setup has not been completed.
// Checks the database on every request — no in-memory state (REQ-AUTH-002).
func RequireSetupComplete(db *sql.DB) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			correlationID := GetCorrelationID(r.Context())

			required, err := setup.IsSetupRequired(r.Context(), db)
			if err != nil {
				response.Error(w, "INTERNAL_ERROR", "failed to check setup status", http.StatusServiceUnavailable, correlationID)
				return
			}
			if required {
				response.Error(w, "SETUP_REQUIRED",
					"Initial setup has not been completed. Please complete setup at POST /api/v1/setup.",
					http.StatusServiceUnavailable, correlationID)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

// RequireSetupPending blocks the setup endpoint with 403 if setup is already complete.
// Checks the database on every request — no in-memory state (REQ-AUTH-002).
func RequireSetupPending(db *sql.DB) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			correlationID := GetCorrelationID(r.Context())

			required, err := setup.IsSetupRequired(r.Context(), db)
			if err != nil {
				response.Error(w, "INTERNAL_ERROR", "failed to check setup status", http.StatusInternalServerError, correlationID)
				return
			}
			if !required {
				response.Error(w, "SETUP_COMPLETE",
					"Initial setup has already been completed.",
					http.StatusForbidden, correlationID)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}
