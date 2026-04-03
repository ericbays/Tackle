package middleware_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"

	"tackle/internal/middleware"
)

func TestCorrelationID(t *testing.T) {
	t.Run("generates UUID when header is absent", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		w := httptest.NewRecorder()

		var capturedID string
		next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			capturedID = middleware.GetCorrelationID(r.Context())
		})

		middleware.CorrelationID(next).ServeHTTP(w, req)

		headerID := w.Header().Get(middleware.CorrelationIDHeader)
		if headerID == "" {
			t.Fatal("expected X-Correlation-ID header to be set")
		}
		if _, err := uuid.Parse(headerID); err != nil {
			t.Errorf("header value %q is not a valid UUID: %v", headerID, err)
		}
		if capturedID != headerID {
			t.Errorf("context ID %q does not match header ID %q", capturedID, headerID)
		}
	})

	t.Run("preserves existing correlation ID from request", func(t *testing.T) {
		existing := "550e8400-e29b-41d4-a716-446655440000"
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.Header.Set(middleware.CorrelationIDHeader, existing)
		w := httptest.NewRecorder()

		var capturedID string
		next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			capturedID = middleware.GetCorrelationID(r.Context())
		})

		middleware.CorrelationID(next).ServeHTTP(w, req)

		if capturedID != existing {
			t.Errorf("expected context ID %q, got %q", existing, capturedID)
		}
		if w.Header().Get(middleware.CorrelationIDHeader) != existing {
			t.Errorf("expected response header to echo %q", existing)
		}
	})
}
