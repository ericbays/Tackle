package audit

import (
	"context"
	"net/http"

	"tackle/internal/middleware"
	authsvc "tackle/internal/services/auth"
)

// injectAdminClaims is a test middleware that injects admin JWT claims into the context.
func injectAdminClaims(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		claims := &authsvc.Claims{
			Username: "admin",
			Role:     "admin",
		}
		claims.Subject = "00000000-0000-0000-0000-000000000001"
		ctx := context.WithValue(r.Context(), middleware.ClaimsContextKey(), claims)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}
