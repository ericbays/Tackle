package middleware

import (
	"context"
	"crypto/sha256"
	"crypto/subtle"
	"log/slog"
	"net/http"
	"strings"
	"sync"
	"time"

	"tackle/internal/crypto"
	"tackle/internal/repositories"
	"tackle/pkg/response"
)

// endpointAuthKey is the context key for storing the authenticated endpoint ID.
const endpointAuthKey contextKey = "endpoint_auth_id"

// EndpointAuthFromContext returns the authenticated endpoint ID from the request context.
func EndpointAuthFromContext(ctx context.Context) string {
	v, _ := ctx.Value(endpointAuthKey).(string)
	return v
}

// WithEndpointAuth returns a context with the given endpoint ID set.
// Intended for use in tests.
func WithEndpointAuth(ctx context.Context, endpointID string) context.Context {
	return context.WithValue(ctx, endpointAuthKey, endpointID)
}

// endpointTokenEntry holds a cached token hash and endpoint ID.
type endpointTokenEntry struct {
	tokenHash  [32]byte
	endpointID string
}

// EndpointTokenCache maintains a cached mapping of auth token hashes to endpoint IDs.
type EndpointTokenCache struct {
	repo   *repositories.PhishingEndpointRepository
	encSvc *crypto.EncryptionService

	mu      sync.RWMutex
	entries []endpointTokenEntry
	loadedAt time.Time
	cacheTTL time.Duration
}

// NewEndpointTokenCache creates a new cache for endpoint auth tokens.
func NewEndpointTokenCache(repo *repositories.PhishingEndpointRepository, encSvc *crypto.EncryptionService) *EndpointTokenCache {
	return &EndpointTokenCache{
		repo:     repo,
		encSvc:   encSvc,
		cacheTTL: 60 * time.Second,
	}
}

// Validate checks if the given token matches any active endpoint's auth token.
// Returns the endpoint ID if valid, empty string if not.
func (c *EndpointTokenCache) Validate(ctx context.Context, token string) string {
	c.refreshIfNeeded(ctx)

	tokenHash := sha256.Sum256([]byte(token))

	c.mu.RLock()
	defer c.mu.RUnlock()

	for _, entry := range c.entries {
		if subtle.ConstantTimeCompare(tokenHash[:], entry.tokenHash[:]) == 1 {
			return entry.endpointID
		}
	}
	return ""
}

// refreshIfNeeded reloads the cache if it's stale.
func (c *EndpointTokenCache) refreshIfNeeded(ctx context.Context) {
	c.mu.RLock()
	fresh := time.Since(c.loadedAt) < c.cacheTTL
	c.mu.RUnlock()
	if fresh {
		return
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	// Double-check after acquiring write lock.
	if time.Since(c.loadedAt) < c.cacheTTL {
		return
	}

	// Load all active/configuring endpoints that have auth tokens.
	var entries []endpointTokenEntry

	for _, state := range []repositories.EndpointState{
		repositories.EndpointStateActive,
		repositories.EndpointStateConfiguring,
		repositories.EndpointStateStopped,
	} {
		eps, err := c.repo.ListByState(ctx, state)
		if err != nil {
			slog.Warn("endpoint token cache: list failed", "state", state, "error", err)
			continue
		}
		for _, ep := range eps {
			if len(ep.AuthToken) == 0 {
				continue
			}
			plainToken, err := c.encSvc.Decrypt(ep.AuthToken)
			if err != nil {
				continue
			}
			entries = append(entries, endpointTokenEntry{
				tokenHash:  sha256.Sum256(plainToken),
				endpointID: ep.ID,
			})
		}
	}

	c.entries = entries
	c.loadedAt = time.Now()
}

// RequireEndpointAuth returns middleware that validates the X-Build-Token or
// Authorization: Bearer header against active endpoint auth tokens.
// On success, sets the endpoint ID in the request context.
func RequireEndpointAuth(cache *EndpointTokenCache) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			token := extractEndpointToken(r)
			if token == "" {
				correlationID := GetCorrelationID(r.Context())
				response.Error(w, "UNAUTHORIZED", "endpoint authentication required", http.StatusUnauthorized, correlationID)
				return
			}

			endpointID := cache.Validate(r.Context(), token)
			if endpointID == "" {
				correlationID := GetCorrelationID(r.Context())
				response.Error(w, "UNAUTHORIZED", "invalid endpoint token", http.StatusUnauthorized, correlationID)
				return
			}

			ctx := context.WithValue(r.Context(), endpointAuthKey, endpointID)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// OptionalEndpointAuth returns middleware that attempts to validate endpoint auth
// but does NOT reject the request if the token is missing or invalid.
// If valid, sets the endpoint ID in context for downstream handlers.
func OptionalEndpointAuth(cache *EndpointTokenCache) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			token := extractEndpointToken(r)
			if token != "" {
				if endpointID := cache.Validate(r.Context(), token); endpointID != "" {
					ctx := context.WithValue(r.Context(), endpointAuthKey, endpointID)
					next.ServeHTTP(w, r.WithContext(ctx))
					return
				}
			}
			next.ServeHTTP(w, r)
		})
	}
}

// extractEndpointToken extracts the auth token from X-Endpoint-Token, X-Build-Token,
// or Authorization headers.
func extractEndpointToken(r *http.Request) string {
	// Check X-Endpoint-Token first (used by landing page apps alongside build token).
	if token := r.Header.Get("X-Endpoint-Token"); token != "" {
		return token
	}

	// Check X-Build-Token.
	if token := r.Header.Get("X-Build-Token"); token != "" {
		return token
	}

	// Check Authorization: Bearer header.
	auth := r.Header.Get("Authorization")
	if strings.HasPrefix(auth, "Bearer ") {
		return strings.TrimPrefix(auth, "Bearer ")
	}

	return ""
}

