package middleware

import (
	"net/http"

	"tackle/pkg/response"
	"tackle/internal/services/rbac"
)

// RequirePermission returns middleware that enforces a required permission.
// Must be used AFTER RequireAuth in the middleware chain.
// If the authenticated user is admin, the check short-circuits to allowed.
// Returns 403 Forbidden if the user lacks the permission.
func RequirePermission(permission string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			correlationID := GetCorrelationID(r.Context())

			claims := ClaimsFromContext(r.Context())
			if claims == nil {
				response.Error(w, "UNAUTHORIZED", "authentication required", http.StatusUnauthorized, correlationID)
				return
			}

			// Administrator short-circuits all permission checks.
			if rbac.IsAdmin(claims.Role) {
				next.ServeHTTP(w, r)
				return
			}

			// Check that the permission is in the claims.
			for _, p := range claims.Permissions {
				if p == permission {
					next.ServeHTTP(w, r)
					return
				}
			}

			response.Error(w, "FORBIDDEN", "insufficient permissions", http.StatusForbidden, correlationID)
		})
	}
}
