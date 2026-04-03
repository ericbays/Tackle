package middleware

import (
	"log/slog"
	"net/http"
	"runtime/debug"

	"tackle/pkg/response"
)

// Recovery catches panics, logs the stack trace, and returns a 500 response.
func Recovery(logger *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			defer func() {
				if rec := recover(); rec != nil {
					correlationID := GetCorrelationID(r.Context())
					logger.ErrorContext(r.Context(), "panic recovered",
						slog.String("component", "recovery"),
						slog.Any("panic", rec),
						slog.String("stack", string(debug.Stack())),
						slog.String("correlation_id", correlationID),
					)
					response.Error(w, "INTERNAL_ERROR", "an unexpected error occurred", http.StatusInternalServerError, correlationID)
				}
			}()
			next.ServeHTTP(w, r)
		})
	}
}
