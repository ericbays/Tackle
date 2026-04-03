package middleware

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestSecurityHeaders(t *testing.T) {
	handler := SecurityHeaders(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	tests := []struct {
		header   string
		contains string
	}{
		{"Content-Security-Policy", "default-src 'self'"},
		{"Content-Security-Policy", "script-src 'self'"},
		{"Content-Security-Policy", "frame-src 'self' blob:"},
		{"X-Content-Type-Options", "nosniff"},
		{"X-Frame-Options", "DENY"},
		{"Referrer-Policy", "strict-origin-when-cross-origin"},
		{"Permissions-Policy", "camera=()"},
	}

	for _, tt := range tests {
		val := rec.Header().Get(tt.header)
		if val == "" {
			t.Errorf("missing header %s", tt.header)
			continue
		}
		if !strings.Contains(val, tt.contains) {
			t.Errorf("header %s = %q, want substring %q", tt.header, val, tt.contains)
		}
	}
}
