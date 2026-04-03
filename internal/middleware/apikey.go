package middleware

import (
	"context"
	"net/http"

	"tackle/internal/services/apikey"
	"tackle/internal/services/auth"
)

// apiKeyUserKey is the context key for storing API key auth results.
const apiKeyUserKey contextKey = "api_key_user"

// APIKeyAuth checks for an X-API-Key header and validates the key.
// If valid, it sets JWT-equivalent claims in the request context so that
// downstream handlers and RequirePermission work transparently.
// If the header is absent, the request passes through for JWT auth.
func APIKeyAuth(svc *apikey.Service) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			rawKey := r.Header.Get("X-API-Key")
			if rawKey == "" {
				// No API key — fall through to JWT auth.
				next.ServeHTTP(w, r)
				return
			}

			result, err := svc.Validate(r.Context(), rawKey)
			if err != nil {
				// Invalid key — let RequireAuth handle the 401.
				next.ServeHTTP(w, r)
				return
			}

			// Build claims equivalent to JWT auth.
			claims := &auth.Claims{
				Username:    result.Username,
				Email:       result.Email,
				Role:        result.Role,
				Permissions: result.Permissions,
				JTI:         "apikey",
			}
			claims.Subject = result.UserID

			// Set claims in context — same key as JWT auth.
			ctx := context.WithValue(r.Context(), claimsKey, claims)

			// Also mark as API key auth so RequireAuth can skip JWT validation.
			ctx = context.WithValue(ctx, apiKeyUserKey, true)

			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// IsAPIKeyAuth returns true if the request was authenticated via API key.
func IsAPIKeyAuth(ctx context.Context) bool {
	v, _ := ctx.Value(apiKeyUserKey).(bool)
	return v
}
