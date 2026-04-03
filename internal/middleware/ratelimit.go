package middleware

import (
	"fmt"
	"net/http"
	"sync"
	"time"

	"tackle/pkg/response"
)

// rateLimitBucket tracks a sliding-window counter per key.
type rateLimitBucket struct {
	mu          sync.Mutex
	count       int
	windowStart time.Time
}

// RateLimitStore is a concurrency-safe map of per-key buckets.
type RateLimitStore struct {
	mu      sync.RWMutex
	buckets map[string]*rateLimitBucket
}

// NewRateLimitStore creates an empty store.
func NewRateLimitStore() *RateLimitStore {
	return &RateLimitStore{buckets: make(map[string]*rateLimitBucket)}
}

func (s *RateLimitStore) bucket(key string) *rateLimitBucket {
	s.mu.RLock()
	b := s.buckets[key]
	s.mu.RUnlock()
	if b != nil {
		return b
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if b = s.buckets[key]; b != nil {
		return b
	}
	b = &rateLimitBucket{windowStart: time.Now()}
	s.buckets[key] = b
	return b
}

// check increments the counter and returns (allowed, remaining, resetTime).
func (s *RateLimitStore) check(key string, limit int, window time.Duration) (bool, int, time.Time) {
	b := s.bucket(key)
	b.mu.Lock()
	defer b.mu.Unlock()

	now := time.Now()
	if now.Sub(b.windowStart) >= window {
		b.count = 0
		b.windowStart = now
	}

	resetAt := b.windowStart.Add(window)
	b.count++
	remaining := limit - b.count
	if remaining < 0 {
		remaining = 0
	}
	return b.count <= limit, remaining, resetAt
}

// RateLimit returns middleware that enforces a sliding-window rate limit.
// keyFn derives the per-request limit key (e.g., IP or user ID).
// limit is the maximum requests per window.
func RateLimit(store *RateLimitStore, limit int, window time.Duration, keyFn func(r *http.Request) string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			key := keyFn(r)
			allowed, remaining, resetAt := store.check(key, limit, window)

			w.Header().Set("X-RateLimit-Limit", fmt.Sprintf("%d", limit))
			w.Header().Set("X-RateLimit-Remaining", fmt.Sprintf("%d", remaining))
			w.Header().Set("X-RateLimit-Reset", fmt.Sprintf("%d", resetAt.Unix()))

			if !allowed {
				correlationID := GetCorrelationID(r.Context())
				response.Error(w, "TOO_MANY_REQUESTS", "rate limit exceeded", http.StatusTooManyRequests, correlationID)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

// IPKey returns a keyFn that uses the request's remote address as the limit key.
func IPKey(r *http.Request) string {
	return "ip:" + r.RemoteAddr
}

// UserKey returns a keyFn that uses the authenticated user's subject claim as the limit key.
// Falls back to IP if no auth claims are present.
func UserKey(r *http.Request) string {
	claims := ClaimsFromContext(r.Context())
	if claims != nil {
		return "user:" + claims.Subject
	}
	return "ip:" + r.RemoteAddr
}
