package users

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/go-chi/chi/v5"
	"github.com/golang-jwt/jwt/v5"

	"tackle/internal/middleware"
	"tackle/internal/services/auth"
)

func injectClaims(r *http.Request, sub, role string) *http.Request {
	claims := &auth.Claims{
		RegisteredClaims: jwt.RegisteredClaims{Subject: sub},
		Role:             role,
	}
	ctx := context.WithValue(r.Context(), middleware.ClaimsContextKey(), claims)
	return r.WithContext(ctx)
}

func newChiCtx(r *http.Request, id string) *http.Request {
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", id)
	return r.WithContext(context.WithValue(r.Context(), chi.RouteCtxKey, rctx))
}

func TestAssignRole_InitialAdminRoleImmutable(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	mock.ExpectQuery(`SELECT id, username, email, display_name, is_initial_admin, updated_at`).
		WithArgs("user-init-admin").
		WillReturnRows(sqlmock.NewRows([]string{"id", "username", "email", "display_name", "is_initial_admin", "updated_at"}).
			AddRow("user-init-admin", "admin", "admin@test.com", "Admin", true, time.Now()))

	mock.ExpectQuery(`SELECT name FROM roles`).
		WithArgs("role-operator").
		WillReturnRows(sqlmock.NewRows([]string{"name"}).AddRow("operator"))

	d := &Deps{DB: db}
	body := `{"role_id":"role-operator"}`
	req := httptest.NewRequest(http.MethodPut, "/api/v1/users/user-init-admin/roles", bytes.NewBufferString(body))
	req = newChiCtx(req, "user-init-admin")
	req = injectClaims(req, "caller-admin", "admin")

	rr := httptest.NewRecorder()
	d.AssignRole(rr, req)
	if rr.Code != http.StatusForbidden {
		t.Errorf("expected 403 for initial admin role change, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestAssignRole_AdminCannotRemoveOwnAdminRole(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	mock.ExpectQuery(`SELECT id, username, email, display_name, is_initial_admin, updated_at`).
		WithArgs("self-admin").
		WillReturnRows(sqlmock.NewRows([]string{"id", "username", "email", "display_name", "is_initial_admin", "updated_at"}).
			AddRow("self-admin", "selfadmin", "self@test.com", "Self", false, time.Now()))

	mock.ExpectQuery(`SELECT name FROM roles`).
		WithArgs("role-operator").
		WillReturnRows(sqlmock.NewRows([]string{"name"}).AddRow("operator"))

	d := &Deps{DB: db}
	body := `{"role_id":"role-operator"}`
	req := httptest.NewRequest(http.MethodPut, "/api/v1/users/self-admin/roles", bytes.NewBufferString(body))
	req = newChiCtx(req, "self-admin")
	req = injectClaims(req, "self-admin", "admin") // caller == target

	rr := httptest.NewRecorder()
	d.AssignRole(rr, req)
	if rr.Code != http.StatusForbidden {
		t.Errorf("expected 403 for self admin role removal, got %d", rr.Code)
	}
}
