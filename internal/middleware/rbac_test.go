package middleware

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"tackle/internal/services/auth"
	"tackle/internal/services/rbac"
)

func claimsCtx(role string, perms []string) context.Context {
	claims := &auth.Claims{
		Role:        role,
		Permissions: perms,
	}
	return context.WithValue(context.Background(), claimsKey, claims)
}

func rbacOKHandler(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
}

func TestRequirePermission_AdminPassesAny(t *testing.T) {
	handler := RequirePermission("campaigns:read")(http.HandlerFunc(rbacOKHandler))
	req := httptest.NewRequest(http.MethodGet, "/", nil).WithContext(claimsCtx(rbac.RoleAdmin, nil))
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rr.Code)
	}
}

func TestRequirePermission_NonAdminMatchingPerm(t *testing.T) {
	handler := RequirePermission("campaigns:read")(http.HandlerFunc(rbacOKHandler))
	req := httptest.NewRequest(http.MethodGet, "/", nil).WithContext(
		claimsCtx(rbac.RoleOperator, []string{"campaigns:read", "targets:read"}),
	)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rr.Code)
	}
}

func TestRequirePermission_NonAdminMissingPerm(t *testing.T) {
	handler := RequirePermission("campaigns:delete")(http.HandlerFunc(rbacOKHandler))
	req := httptest.NewRequest(http.MethodGet, "/", nil).WithContext(
		claimsCtx(rbac.RoleDefender, []string{"metrics:read"}),
	)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)
	if rr.Code != http.StatusForbidden {
		t.Errorf("expected 403, got %d", rr.Code)
	}
}

func TestRequirePermission_MissingClaims(t *testing.T) {
	handler := RequirePermission("campaigns:read")(http.HandlerFunc(rbacOKHandler))
	req := httptest.NewRequest(http.MethodGet, "/", nil) // no claims in context
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)
	if rr.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", rr.Code)
	}
}
