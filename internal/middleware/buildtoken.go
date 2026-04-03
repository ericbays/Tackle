package middleware

import (
	"context"
	"net"
	"net/http"
	"strings"

	"tackle/internal/repositories"
	"tackle/pkg/response"
)

type buildTokenCtxKey struct{}

// BuildTokenInfo holds build information extracted from the X-Build-Token header.
type BuildTokenInfo struct {
	BuildID    string
	CampaignID string
}

// BuildTokenFromContext extracts build token info from the request context.
func BuildTokenFromContext(ctx context.Context) *BuildTokenInfo {
	info, _ := ctx.Value(buildTokenCtxKey{}).(*BuildTokenInfo)
	return info
}

// RequireBuildToken validates the X-Build-Token header against the database
// and ensures the request originates from localhost.
func RequireBuildToken(repo *repositories.LandingPageRepository) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			correlationID := GetCorrelationID(r.Context())

			// Check localhost origin.
			host, _, err := net.SplitHostPort(r.RemoteAddr)
			if err != nil {
				host = r.RemoteAddr
			}
			if !isLocalhost(host) {
				response.Error(w, "FORBIDDEN", "internal API only accessible from localhost", http.StatusForbidden, correlationID)
				return
			}

			// Extract build token.
			token := r.Header.Get("X-Build-Token")
			if token == "" {
				response.Error(w, "UNAUTHORIZED", "missing X-Build-Token header", http.StatusUnauthorized, correlationID)
				return
			}

			// Look up build.
			build, err := repo.GetBuildByToken(r.Context(), token)
			if err != nil {
				response.Error(w, "UNAUTHORIZED", "invalid build token", http.StatusUnauthorized, correlationID)
				return
			}

			// Verify build is in running state.
			if build.Status != "running" {
				response.Error(w, "UNAUTHORIZED", "build is not in running state", http.StatusUnauthorized, correlationID)
				return
			}

			// Set build info on context.
			campaignID := ""
			if build.CampaignID != nil {
				campaignID = *build.CampaignID
			}
			info := &BuildTokenInfo{
				BuildID:    build.ID,
				CampaignID: campaignID,
			}
			ctx := context.WithValue(r.Context(), buildTokenCtxKey{}, info)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// WithBuildToken returns a context with the given BuildTokenInfo set.
// Intended for use in tests.
func WithBuildToken(ctx context.Context, info *BuildTokenInfo) context.Context {
	return context.WithValue(ctx, buildTokenCtxKey{}, info)
}

func isLocalhost(host string) bool {
	host = strings.TrimSpace(host)
	if host == "127.0.0.1" || host == "::1" || host == "localhost" {
		return true
	}
	ip := net.ParseIP(host)
	if ip == nil {
		return false
	}
	return ip.IsLoopback()
}
