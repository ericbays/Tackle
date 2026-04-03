// Package middleware provides HTTP middleware for the Tackle server.
package middleware

import (
	"context"
	"net/http"

	"github.com/google/uuid"
)

type contextKey string

// CorrelationIDKey is the context key used to store the correlation ID.
const CorrelationIDKey contextKey = "correlation_id"

// CorrelationIDHeader is the HTTP header name for the correlation ID.
const CorrelationIDHeader = "X-Correlation-ID"

// CorrelationID generates a UUID v4 correlation ID for each request,
// sets it in the response header, and stores it in the request context.
func CorrelationID(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id := r.Header.Get(CorrelationIDHeader)
		if id == "" {
			id = uuid.New().String()
		}

		ctx := context.WithValue(r.Context(), CorrelationIDKey, id)
		w.Header().Set(CorrelationIDHeader, id)
		w.Header().Set("X-Request-ID", id)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// GetCorrelationID retrieves the correlation ID from the request context.
// Returns an empty string if not set.
func GetCorrelationID(ctx context.Context) string {
	if id, ok := ctx.Value(CorrelationIDKey).(string); ok {
		return id
	}
	return ""
}
