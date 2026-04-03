package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"tackle/internal/models"
	"tackle/internal/services/auth"
)

func newTestAuthMiddleware(t *testing.T) (*auth.JWTService, *auth.TokenBlacklist) {
	t.Helper()
	svc := auth.NewJWTService([]byte("test-signing-key-32-bytes-padded!"), 15)
	bl := auth.NewTokenBlacklist()
	return svc, bl
}

func issueTestToken(t *testing.T, svc *auth.JWTService, role string) string {
	t.Helper()
	user := &models.User{ID: "user-1", Username: "alice", Email: "alice@example.com"}
	token, err := svc.Issue(user, role, []string{"campaigns:read"})
	if err != nil {
		t.Fatalf("Issue: %v", err)
	}
	return token
}

func okHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
}

func TestRequireAuth_ValidToken(t *testing.T) {
	svc, bl := newTestAuthMiddleware(t)
	mw := RequireAuth(svc, bl)
	token := issueTestToken(t, svc, "operator")

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()

	mw(okHandler()).ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want 200", w.Code)
	}
}

func TestRequireAuth_MissingHeader(t *testing.T) {
	svc, bl := newTestAuthMiddleware(t)
	mw := RequireAuth(svc, bl)

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	w := httptest.NewRecorder()

	mw(okHandler()).ServeHTTP(w, req)
	if w.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want 401", w.Code)
	}
}

func TestRequireAuth_InvalidToken(t *testing.T) {
	svc, bl := newTestAuthMiddleware(t)
	mw := RequireAuth(svc, bl)

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "Bearer notavalidtoken")
	w := httptest.NewRecorder()

	mw(okHandler()).ServeHTTP(w, req)
	if w.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want 401", w.Code)
	}
}

func TestRequireAuth_RevokedToken(t *testing.T) {
	svc, bl := newTestAuthMiddleware(t)
	mw := RequireAuth(svc, bl)
	token := issueTestToken(t, svc, "operator")

	// Parse claims to get JTI.
	claims, err := svc.Validate(token)
	if err != nil {
		t.Fatalf("Validate: %v", err)
	}
	bl.Revoke(claims.JTI, time.Now().Add(15*time.Minute))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()

	mw(okHandler()).ServeHTTP(w, req)
	if w.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want 401 for revoked token", w.Code)
	}
}

func TestClaimsFromContext_StoredByMiddleware(t *testing.T) {
	svc, bl := newTestAuthMiddleware(t)
	mw := RequireAuth(svc, bl)
	token := issueTestToken(t, svc, "admin")

	var captured *auth.Claims
	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		captured = ClaimsFromContext(r.Context())
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	mw(inner).ServeHTTP(w, req)

	if captured == nil {
		t.Fatal("expected claims in context, got nil")
	}
	if captured.Role != "admin" {
		t.Errorf("role = %q, want admin", captured.Role)
	}
}
