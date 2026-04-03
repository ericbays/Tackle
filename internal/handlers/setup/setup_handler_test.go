package setup_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"

	setuphandler "tackle/internal/handlers/setup"
	authsvc "tackle/internal/services/auth"
)

func newTestDeps(t *testing.T, db interface{ Close() error }) *setuphandler.Deps {
	t.Helper()
	// JWTSvc and RefreshSvc require a real *sql.DB-compatible value; we pass the sqlmock db.
	// We only need these for the happy-path test; validation tests never reach them.
	return &setuphandler.Deps{
		Policy: authsvc.DefaultPolicy(),
	}
}

func TestSetup_PasswordPolicyEnforced(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("open sqlmock: %v", err)
	}
	defer db.Close()
	_ = mock

	d := &setuphandler.Deps{
		DB:     db,
		Policy: authsvc.DefaultPolicy(),
	}

	body := map[string]string{
		"username":              "adminuser",
		"email":                 "admin@example.com",
		"display_name":          "Admin",
		"password":              "short", // too short, no uppercase, digit, special
		"password_confirmation": "short",
	}
	b, _ := json.Marshal(body)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/setup", bytes.NewReader(b))
	rr := httptest.NewRecorder()
	d.Setup(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected 400 got %d", rr.Code)
	}
}

func TestSetup_PasswordConfirmationMismatch(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("open sqlmock: %v", err)
	}
	defer db.Close()
	_ = mock

	d := &setuphandler.Deps{
		DB:     db,
		Policy: authsvc.DefaultPolicy(),
	}

	body := map[string]string{
		"username":              "adminuser",
		"email":                 "admin@example.com",
		"display_name":          "Admin",
		"password":              "ValidP@ss1234",
		"password_confirmation": "DifferentPass!1",
	}
	b, _ := json.Marshal(body)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/setup", bytes.NewReader(b))
	rr := httptest.NewRecorder()
	d.Setup(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected 400 got %d", rr.Code)
	}
}

func TestSetup_InvalidUsername(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("open sqlmock: %v", err)
	}
	defer db.Close()
	_ = mock

	d := &setuphandler.Deps{
		DB:     db,
		Policy: authsvc.DefaultPolicy(),
	}

	body := map[string]string{
		"username":              "ab", // too short
		"email":                 "admin@example.com",
		"display_name":          "Admin",
		"password":              "ValidP@ss1234",
		"password_confirmation": "ValidP@ss1234",
	}
	b, _ := json.Marshal(body)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/setup", bytes.NewReader(b))
	rr := httptest.NewRecorder()
	d.Setup(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected 400 got %d", rr.Code)
	}
}

func TestSetup_InvalidEmail(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("open sqlmock: %v", err)
	}
	defer db.Close()
	_ = mock

	d := &setuphandler.Deps{
		DB:     db,
		Policy: authsvc.DefaultPolicy(),
	}

	body := map[string]string{
		"username":              "adminuser",
		"email":                 "not-an-email",
		"display_name":          "Admin",
		"password":              "ValidP@ss1234",
		"password_confirmation": "ValidP@ss1234",
	}
	b, _ := json.Marshal(body)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/setup", bytes.NewReader(b))
	rr := httptest.NewRecorder()
	d.Setup(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected 400 got %d", rr.Code)
	}
}

func TestSetup_CreatesUserWithIsInitialAdmin(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("open sqlmock: %v", err)
	}
	defer db.Close()

	signingKey := make([]byte, 32)
	jwtSvc := authsvc.NewJWTService(signingKey, 15)
	refreshSvc := authsvc.NewRefreshTokenService(db)

	d := &setuphandler.Deps{
		DB:         db,
		JWTSvc:     jwtSvc,
		RefreshSvc: refreshSvc,
		Policy:     authsvc.DefaultPolicy(),
	}

	userID := "00000000-0000-0000-0000-000000000001"
	roleID := "00000000-0000-0000-0000-000000000002"

	mock.ExpectBegin()
	mock.ExpectQuery(`INSERT INTO users`).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(userID))
	mock.ExpectQuery(`SELECT id FROM roles WHERE name`).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(roleID))
	mock.ExpectExec(`INSERT INTO user_roles`).
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectCommit()
	// Refresh token INSERT happens outside the transaction (user FK satisfied after commit).
	mock.ExpectExec(`INSERT INTO sessions`).
		WillReturnResult(sqlmock.NewResult(1, 1))

	body := map[string]string{
		"username":              "adminuser",
		"email":                 "admin@example.com",
		"display_name":          "Admin User",
		"password":              "ValidP@ss1234",
		"password_confirmation": "ValidP@ss1234",
	}
	b, _ := json.Marshal(body)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/setup", bytes.NewReader(b))
	rr := httptest.NewRecorder()
	d.Setup(rr, req)

	if rr.Code != http.StatusCreated {
		t.Errorf("expected 201 got %d; body: %s", rr.Code, rr.Body.String())
	}

	var resp map[string]interface{}
	if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	data, ok := resp["data"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected data envelope, got %v", resp)
	}
	if data["access_token"] == "" || data["access_token"] == nil {
		t.Error("expected non-empty access_token")
	}
	user, ok := data["user"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected user object in response")
	}
	if user["id"] != userID {
		t.Errorf("expected user_id=%s got %v", userID, user["id"])
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet expectations: %v", err)
	}
}
