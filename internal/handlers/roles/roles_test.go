package roles

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/go-chi/chi/v5"
)

func newChiCtxWithID(r *http.Request, id string) *http.Request {
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", id)
	return r.WithContext(context.WithValue(r.Context(), chi.RouteCtxKey, rctx))
}

func TestCreate_InvalidPermission(t *testing.T) {
	db, _, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	d := &Deps{DB: db}
	body := `{"name":"custom","description":"test","permissions":["nonexistent:action"]}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/roles", bytes.NewBufferString(body))
	rr := httptest.NewRecorder()
	d.Create(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", rr.Code)
	}
}

func TestCreate_BuiltinNameConflict(t *testing.T) {
	db, _, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	d := &Deps{DB: db}
	for _, name := range []string{"admin", "Admin", "ADMIN", "engineer", "operator", "defender"} {
		body := `{"name":"` + name + `","description":"","permissions":[]}`
		req := httptest.NewRequest(http.MethodPost, "/api/v1/roles", bytes.NewBufferString(body))
		rr := httptest.NewRecorder()
		d.Create(rr, req)
		if rr.Code != http.StatusBadRequest {
			t.Errorf("name %q: expected 400, got %d", name, rr.Code)
		}
	}
}

func TestUpdate_BuiltinRoleConflict(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	mock.ExpectQuery(`SELECT is_builtin FROM roles`).
		WithArgs("role-id-1").
		WillReturnRows(sqlmock.NewRows([]string{"is_builtin"}).AddRow(true))

	d := &Deps{DB: db}
	body := `{"name":"newname","description":"","permissions":[]}`
	req := httptest.NewRequest(http.MethodPut, "/api/v1/roles/role-id-1", bytes.NewBufferString(body))
	req = newChiCtxWithID(req, "role-id-1")
	rr := httptest.NewRecorder()
	d.Update(rr, req)
	if rr.Code != http.StatusConflict {
		t.Errorf("expected 409, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestDelete_BuiltinRoleConflict(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	mock.ExpectQuery(`SELECT is_builtin FROM roles`).
		WithArgs("role-id-builtin").
		WillReturnRows(sqlmock.NewRows([]string{"is_builtin"}).AddRow(true))

	d := &Deps{DB: db}
	req := httptest.NewRequest(http.MethodDelete, "/api/v1/roles/role-id-builtin", nil)
	req = newChiCtxWithID(req, "role-id-builtin")
	rr := httptest.NewRecorder()
	d.Delete(rr, req)
	if rr.Code != http.StatusConflict {
		t.Errorf("expected 409, got %d", rr.Code)
	}
}

func TestDelete_AssignedUsersConflict(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	mock.ExpectQuery(`SELECT is_builtin FROM roles`).
		WithArgs("role-id-custom").
		WillReturnRows(sqlmock.NewRows([]string{"is_builtin"}).AddRow(false))

	mock.ExpectQuery(`SELECT user_id FROM user_roles`).
		WithArgs("role-id-custom").
		WillReturnRows(sqlmock.NewRows([]string{"user_id"}).AddRow("user-1").AddRow("user-2"))

	d := &Deps{DB: db}
	req := httptest.NewRequest(http.MethodDelete, "/api/v1/roles/role-id-custom", nil)
	req = newChiCtxWithID(req, "role-id-custom")
	rr := httptest.NewRecorder()
	d.Delete(rr, req)
	if rr.Code != http.StatusConflict {
		t.Errorf("expected 409, got %d", rr.Code)
	}

	var resp map[string]interface{}
	if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	errObj, ok := resp["error"].(map[string]interface{})
	if !ok {
		t.Fatal("expected error object in response")
	}
	userCount, ok := errObj["user_count"].(float64)
	if !ok || userCount != 2 {
		t.Errorf("expected user_count=2, got %v", errObj["user_count"])
	}
}
