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

// chiCtx2 adds both "id" and a second param to the route context.
func chiCtxSid(r *http.Request, id, sid string) *http.Request {
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", id)
	rctx.URLParams.Add("sid", sid)
	return r.WithContext(context.WithValue(r.Context(), chi.RouteCtxKey, rctx))
}

func claimsCtx(r *http.Request, sub string) *http.Request {
	claims := &auth.Claims{
		RegisteredClaims: jwt.RegisteredClaims{Subject: sub},
		Role:             "admin",
	}
	ctx := context.WithValue(r.Context(), middleware.ClaimsContextKey(), claims)
	return r.WithContext(ctx)
}

// ---- ListUsers ----

func TestListUsers_Success(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	mock.ExpectQuery(`SELECT COUNT\(\*\) FROM users`).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(1))

	mock.ExpectQuery(`SELECT u.id`).
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "username", "email", "display_name", "status",
			"role_id", "role_name", "is_initial_admin", "force_password_change", "created_at",
		}).AddRow("uid1", "alice", "alice@test.com", "Alice", "active",
			"rid1", "admin", false, false, time.Now()))

	d := &Deps{DB: db}
	req := httptest.NewRequest(http.MethodGet, "/api/v1/users", nil)
	req = claimsCtx(req, "caller-1")
	rr := httptest.NewRecorder()
	d.ListUsers(rr, req)
	if rr.Code != http.StatusOK {
		t.Errorf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
}

// ---- CreateUser ----

func TestCreateUser_MissingUsername(t *testing.T) {
	d := &Deps{}
	body := `{"email":"a@b.com","display_name":"A","password":"supersecretpassword1","role_id":"rid"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/users", bytes.NewBufferString(body))
	req = claimsCtx(req, "caller-1")
	rr := httptest.NewRecorder()
	d.CreateUser(rr, req)
	if rr.Code != http.StatusUnprocessableEntity {
		t.Errorf("expected 422 for missing username, got %d", rr.Code)
	}
}

func TestCreateUser_ShortPassword(t *testing.T) {
	d := &Deps{}
	body := `{"username":"alice","email":"a@b.com","display_name":"A","password":"short","role_id":"rid"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/users", bytes.NewBufferString(body))
	req = claimsCtx(req, "caller-1")
	rr := httptest.NewRecorder()
	d.CreateUser(rr, req)
	if rr.Code != http.StatusUnprocessableEntity {
		t.Errorf("expected 422 for short password, got %d", rr.Code)
	}
}

func TestCreateUser_InvalidEmail(t *testing.T) {
	d := &Deps{}
	body := `{"username":"alice","email":"not-an-email","display_name":"A","password":"supersecretpassword1","role_id":"rid"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/users", bytes.NewBufferString(body))
	req = claimsCtx(req, "caller-1")
	rr := httptest.NewRecorder()
	d.CreateUser(rr, req)
	if rr.Code != http.StatusUnprocessableEntity {
		t.Errorf("expected 422 for invalid email, got %d", rr.Code)
	}
}

// ---- GetUser ----

func TestGetUser_NotFound(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	mock.ExpectQuery(`SELECT u.id`).
		WithArgs("missing-uid").
		WillReturnRows(sqlmock.NewRows(nil))

	d := &Deps{DB: db}
	req := httptest.NewRequest(http.MethodGet, "/api/v1/users/missing-uid", nil)
	req = newChiCtx(req, "missing-uid")
	req = claimsCtx(req, "caller-1")
	rr := httptest.NewRecorder()
	d.GetUser(rr, req)
	if rr.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d: %s", rr.Code, rr.Body.String())
	}
}

// ---- UpdateUser ----

func TestUpdateUser_InitialAdminStatusBlocked(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	mock.ExpectQuery(`SELECT is_initial_admin FROM users`).
		WithArgs("init-admin-id").
		WillReturnRows(sqlmock.NewRows([]string{"is_initial_admin"}).AddRow(true))

	d := &Deps{DB: db}
	body := `{"status":"inactive"}`
	req := httptest.NewRequest(http.MethodPut, "/api/v1/users/init-admin-id", bytes.NewBufferString(body))
	req = newChiCtx(req, "init-admin-id")
	req = claimsCtx(req, "caller-1")
	rr := httptest.NewRecorder()
	d.UpdateUser(rr, req)
	if rr.Code != http.StatusConflict {
		t.Errorf("expected 409 for initial admin status change, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestUpdateUser_InvalidStatus(t *testing.T) {
	d := &Deps{}
	body := `{"status":"suspended"}`
	req := httptest.NewRequest(http.MethodPut, "/api/v1/users/uid1", bytes.NewBufferString(body))
	req = newChiCtx(req, "uid1")
	req = claimsCtx(req, "caller-1")
	rr := httptest.NewRecorder()
	d.UpdateUser(rr, req)
	if rr.Code != http.StatusUnprocessableEntity {
		t.Errorf("expected 422 for invalid status, got %d: %s", rr.Code, rr.Body.String())
	}
}

// ---- DeleteUser ----

func TestDeleteUser_CannotDeleteSelf(t *testing.T) {
	d := &Deps{}
	req := httptest.NewRequest(http.MethodDelete, "/api/v1/users/self-id", nil)
	req = newChiCtx(req, "self-id")
	req = claimsCtx(req, "self-id") // caller == target
	rr := httptest.NewRecorder()
	d.DeleteUser(rr, req)
	if rr.Code != http.StatusForbidden {
		t.Errorf("expected 403 for self-delete, got %d", rr.Code)
	}
}

func TestDeleteUser_InitialAdminBlocked(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	mock.ExpectQuery(`SELECT is_initial_admin FROM users`).
		WithArgs("init-admin-id").
		WillReturnRows(sqlmock.NewRows([]string{"is_initial_admin"}).AddRow(true))

	d := &Deps{DB: db}
	req := httptest.NewRequest(http.MethodDelete, "/api/v1/users/init-admin-id", nil)
	req = newChiCtx(req, "init-admin-id")
	req = claimsCtx(req, "caller-other")
	rr := httptest.NewRecorder()
	d.DeleteUser(rr, req)
	if rr.Code != http.StatusConflict {
		t.Errorf("expected 409 for initial admin delete, got %d", rr.Code)
	}
}

// ---- TerminateUserSession ----

func TestTerminateUserSession_NotFound(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	mock.ExpectExec(`UPDATE sessions`).
		WithArgs("missing-sid", "uid1").
		WillReturnResult(sqlmock.NewResult(0, 0))

	d := &Deps{DB: db}
	req := httptest.NewRequest(http.MethodDelete, "/api/v1/users/uid1/sessions/missing-sid", nil)
	req = chiCtxSid(req, "uid1", "missing-sid")
	req = claimsCtx(req, "caller-1")
	rr := httptest.NewRecorder()
	d.TerminateUserSession(rr, req)
	if rr.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d", rr.Code)
	}
}
