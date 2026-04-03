package middleware

import (
	"context"
	"net/http"
	"strings"

	"tackle/internal/services/auth"
	"tackle/pkg/response"
)

// claimsKey is the context key for storing authenticated JWT claims.
const claimsKey contextKey = "auth_claims"

// RequireAuth validates the Bearer JWT in the Authorization header and checks
// the JTI against the token blacklist. If a UserStatusCache is provided, it also
// checks whether the user's account is locked or inactive.
// On success it stores *auth.Claims in the request context for downstream handlers
// and middleware.
// Returns 401 Unauthorized if the token is missing, malformed, invalid, or revoked.
func RequireAuth(jwtSvc *auth.JWTService, blacklist *auth.TokenBlacklist, statusCache ...*UserStatusCache) func(http.Handler) http.Handler {
	var cache *UserStatusCache
	if len(statusCache) > 0 {
		cache = statusCache[0]
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			correlationID := GetCorrelationID(r.Context())

			// If already authenticated via API key, skip JWT validation.
			if IsAPIKeyAuth(r.Context()) {
				next.ServeHTTP(w, r)
				return
			}

			authHeader := r.Header.Get("Authorization")
			if !strings.HasPrefix(authHeader, "Bearer ") {
				response.Error(w, "UNAUTHORIZED", "missing or malformed authorization header", http.StatusUnauthorized, correlationID)
				return
			}
			tokenStr := strings.TrimPrefix(authHeader, "Bearer ")

			claims, err := jwtSvc.Validate(tokenStr)
			if err != nil {
				response.Error(w, "UNAUTHORIZED", "invalid or expired token", http.StatusUnauthorized, correlationID)
				return
			}
			if blacklist.IsRevoked(claims.JTI) {
				response.Error(w, "UNAUTHORIZED", "token has been revoked", http.StatusUnauthorized, correlationID)
				return
			}

			// Check if user account is locked or inactive (cached, 30s TTL).
			if cache != nil && cache.IsLocked(r.Context(), claims.Subject) {
				response.Error(w, "UNAUTHORIZED", "account locked", http.StatusUnauthorized, correlationID)
				return
			}

			ctx := context.WithValue(r.Context(), claimsKey, claims)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// ClaimsFromContext retrieves *auth.Claims from the request context.
// Returns nil if not present (unauthenticated request).
func ClaimsFromContext(ctx context.Context) *auth.Claims {
	claims, _ := ctx.Value(claimsKey).(*auth.Claims)
	return claims
}

// ClaimsContextKey returns the context key used to store JWT claims.
// Exported for use in tests that need to inject claims directly into a context.
func ClaimsContextKey() interface{} {
	return claimsKey
}
