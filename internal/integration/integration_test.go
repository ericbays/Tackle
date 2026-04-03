//go:build integration

// Package integration contains end-to-end tests for Phase 1.
// Requires a real PostgreSQL database. Set TEST_DATABASE_URL to run.
// Run with: go test -tags=integration ./internal/integration/...
package integration

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	_ "github.com/lib/pq"

	"tackle/internal/config"
	"tackle/internal/crypto"
	auditsvc "tackle/internal/services/audit"
	"tackle/internal/server"
)

// testEnv holds state shared across integration tests.
type testEnv struct {
	srv       *httptest.Server
	db        *sql.DB
	masterKey []byte
	auditHMAC *auditsvc.HMACService
}

func setupTestEnv(t *testing.T) *testEnv {
	t.Helper()

	dbURL := os.Getenv("TEST_DATABASE_URL")
	if dbURL == "" {
		t.Skip("TEST_DATABASE_URL not set — skipping integration tests")
	}

	db, err := sql.Open("postgres", dbURL)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	db.SetMaxOpenConns(10)
	if err := db.PingContext(context.Background()); err != nil {
		t.Fatalf("ping db: %v", err)
	}

	masterKey := testMasterKey()

	auditHMACKey, err := crypto.DeriveSubkey(masterKey, crypto.PurposeHMACAudit)
	if err != nil {
		t.Fatalf("derive audit hmac key: %v", err)
	}

	cfg := &config.Config{
		ListenAddr:         ":0",
		DatabaseURL:        dbURL,
		Env:                "test",
		CORSAllowedOrigins: []string{"http://localhost:5173"},
	}

	logger := newTestLogger(t)
	httpSrv := server.New(cfg, db, masterKey, logger)

	ts := httptest.NewServer(httpSrv.Handler)
	t.Cleanup(func() {
		ts.Close()
		db.Close()
	})

	return &testEnv{
		srv:       ts,
		db:        db,
		masterKey: masterKey,
		auditHMAC: auditsvc.NewHMACService(auditHMACKey),
	}
}

func (e *testEnv) url(path string) string {
	return e.srv.URL + "/api/v1" + path
}

func (e *testEnv) do(method, path string, body any, token string) *http.Response {
	var bodyR io.Reader
	if body != nil {
		b, _ := json.Marshal(body)
		bodyR = bytes.NewReader(b)
	}
	req, _ := http.NewRequest(method, e.url(path), bodyR)
	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		panic(fmt.Sprintf("http %s %s: %v", method, path, err))
	}
	return resp
}

// decodeT reads and JSON-decodes a response body in a test context.
func decodeT(t *testing.T, resp *http.Response, dst any) {
	t.Helper()
	defer resp.Body.Close()
	if err := json.NewDecoder(resp.Body).Decode(dst); err != nil {
		t.Fatalf("decode response: %v", err)
	}
}

// TestSetupFlow tests: fresh DB → setup wizard → login → access protected endpoint.
func TestSetupFlow(t *testing.T) {
	env := setupTestEnv(t)
	resetDB(t, env.db)

	resp := env.do("POST", "/setup", map[string]string{
		"username":              "admin",
		"email":                 "admin@example.com",
		"display_name":          "Administrator",
		"password":              "S3cur3P@ssw0rd!XYZ",
		"password_confirmation": "S3cur3P@ssw0rd!XYZ",
	}, "")
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("setup: expected 201, got %d", resp.StatusCode)
	}

	var setupResp struct {
		Data struct {
			AccessToken string `json:"access_token"`
		} `json:"data"`
	}
	decodeT(t, resp, &setupResp)
	token := setupResp.Data.AccessToken
	if token == "" {
		t.Fatal("setup: expected non-empty access token")
	}

	resp = env.do("GET", "/auth/me", nil, token)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("auth/me: expected 200, got %d", resp.StatusCode)
	}
	resp.Body.Close()

	// Setup endpoint should now return 403.
	resp = env.do("POST", "/setup", map[string]string{
		"username":              "second",
		"email":                 "second@example.com",
		"display_name":          "Second",
		"password":              "S3cur3P@ssw0rd!XYZ",
		"password_confirmation": "S3cur3P@ssw0rd!XYZ",
	}, "")
	if resp.StatusCode != http.StatusForbidden {
		resp.Body.Close()
		t.Fatalf("second setup: expected 403, got %d", resp.StatusCode)
	}
	resp.Body.Close()
}

// TestLoginJWTClaims tests login → JWT contains role + permissions → API access works.
func TestLoginJWTClaims(t *testing.T) {
	env := setupTestEnv(t)
	resetDB(t, env.db)
	mustSetup(t, env)

	resp := env.do("POST", "/auth/login", map[string]string{
		"username": "admin",
		"password": "S3cur3P@ssw0rd!XYZ",
	}, "")
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("login: expected 200, got %d", resp.StatusCode)
	}
	var loginResp struct {
		Data struct {
			AccessToken string `json:"access_token"`
			User        struct {
				Roles       []string `json:"roles"`
				Permissions []string `json:"permissions"`
			} `json:"user"`
		} `json:"data"`
	}
	decodeT(t, resp, &loginResp)

	if len(loginResp.Data.User.Roles) == 0 {
		t.Fatal("login: expected roles in response")
	}
	if len(loginResp.Data.User.Permissions) == 0 {
		t.Fatal("login: expected permissions in response")
	}

	resp = env.do("GET", "/users", nil, loginResp.Data.AccessToken)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("list users: expected 200, got %d", resp.StatusCode)
	}
	resp.Body.Close()
}

// TestRefreshToken tests login → refresh → new access token issued.
func TestRefreshToken(t *testing.T) {
	env := setupTestEnv(t)
	resetDB(t, env.db)
	mustSetup(t, env)

	resp := env.do("POST", "/auth/login", map[string]string{
		"username": "admin", "password": "S3cur3P@ssw0rd!XYZ",
	}, "")
	var lr struct {
		Data struct {
			RefreshToken string `json:"refresh_token"`
		} `json:"data"`
	}
	decodeT(t, resp, &lr)
	refreshToken := lr.Data.RefreshToken
	if refreshToken == "" {
		t.Skip("refresh token not in body — cookie-only client needed")
	}

	resp = env.do("POST", "/auth/refresh", map[string]string{"refresh_token": refreshToken}, "")
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("refresh: expected 200, got %d", resp.StatusCode)
	}
	var rr struct {
		Data struct {
			AccessToken string `json:"access_token"`
		} `json:"data"`
	}
	decodeT(t, resp, &rr)
	if rr.Data.AccessToken == "" {
		t.Fatal("refresh: expected non-empty access token")
	}
}

// TestLogoutInvalidatesRefresh tests login → logout → refresh returns 401.
func TestLogoutInvalidatesRefresh(t *testing.T) {
	env := setupTestEnv(t)
	resetDB(t, env.db)
	mustSetup(t, env)

	resp := env.do("POST", "/auth/login", map[string]string{
		"username": "admin", "password": "S3cur3P@ssw0rd!XYZ",
	}, "")
	var lr struct {
		Data struct {
			AccessToken  string `json:"access_token"`
			RefreshToken string `json:"refresh_token"`
		} `json:"data"`
	}
	decodeT(t, resp, &lr)
	token := lr.Data.AccessToken
	refreshToken := lr.Data.RefreshToken
	if refreshToken == "" {
		t.Skip("refresh token not in body")
	}

	resp = env.do("POST", "/auth/logout", nil, token)
	if resp.StatusCode != http.StatusNoContent {
		resp.Body.Close()
		t.Fatalf("logout: expected 204, got %d", resp.StatusCode)
	}
	resp.Body.Close()

	resp = env.do("POST", "/auth/refresh", map[string]string{"refresh_token": refreshToken}, "")
	if resp.StatusCode != http.StatusUnauthorized {
		resp.Body.Close()
		t.Fatalf("refresh after logout: expected 401, got %d", resp.StatusCode)
	}
	resp.Body.Close()
}

// TestRBACOperatorLimitedAccess tests Operator role cannot access user management.
func TestRBACOperatorLimitedAccess(t *testing.T) {
	env := setupTestEnv(t)
	resetDB(t, env.db)
	adminToken := mustSetup(t, env)

	operatorRoleID := getOperatorRoleID(t, env, adminToken)

	resp := env.do("POST", "/users", map[string]string{
		"username":     "operator1",
		"email":        "operator1@example.com",
		"display_name": "Operator One",
		"password":     "S3cur3P@ssw0rd!XYZ",
		"role_id":      operatorRoleID,
	}, adminToken)
	if resp.StatusCode != http.StatusCreated {
		resp.Body.Close()
		t.Fatalf("create operator: expected 201, got %d", resp.StatusCode)
	}
	resp.Body.Close()

	resp = env.do("POST", "/auth/login", map[string]string{
		"username": "operator1", "password": "S3cur3P@ssw0rd!XYZ",
	}, "")
	var opLogin struct {
		Data struct {
			AccessToken string `json:"access_token"`
		} `json:"data"`
	}
	decodeT(t, resp, &opLogin)
	opToken := opLogin.Data.AccessToken

	resp = env.do("GET", "/users", nil, opToken)
	if resp.StatusCode != http.StatusForbidden {
		resp.Body.Close()
		t.Fatalf("operator GET /users: expected 403, got %d", resp.StatusCode)
	}
	resp.Body.Close()

	resp = env.do("GET", "/roles", nil, opToken)
	if resp.StatusCode != http.StatusForbidden {
		resp.Body.Close()
		t.Fatalf("operator GET /roles: expected 403, got %d", resp.StatusCode)
	}
	resp.Body.Close()
}

// TestAccountLockout tests: lock account → login rejected → unlock → login works.
func TestAccountLockout(t *testing.T) {
	env := setupTestEnv(t)
	resetDB(t, env.db)
	adminToken := mustSetup(t, env)

	operatorRoleID := getOperatorRoleID(t, env, adminToken)
	resp := env.do("POST", "/users", map[string]string{
		"username":     "locktest",
		"email":        "locktest@example.com",
		"display_name": "Lock Test",
		"password":     "S3cur3P@ssw0rd!XYZ",
		"role_id":      operatorRoleID,
	}, adminToken)
	var created struct {
		Data struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	decodeT(t, resp, &created)
	userID := created.Data.ID

	status := "locked"
	resp = env.do("PUT", "/users/"+userID, map[string]any{"status": &status}, adminToken)
	if resp.StatusCode != http.StatusNoContent {
		resp.Body.Close()
		t.Fatalf("lock user: expected 204, got %d", resp.StatusCode)
	}
	resp.Body.Close()

	resp = env.do("POST", "/auth/login", map[string]string{
		"username": "locktest", "password": "S3cur3P@ssw0rd!XYZ",
	}, "")
	if resp.StatusCode != http.StatusUnauthorized {
		resp.Body.Close()
		t.Fatalf("locked login: expected 401, got %d", resp.StatusCode)
	}
	resp.Body.Close()

	activeStatus := "active"
	resp = env.do("PUT", "/users/"+userID, map[string]any{"status": &activeStatus}, adminToken)
	if resp.StatusCode != http.StatusNoContent {
		resp.Body.Close()
		t.Fatalf("unlock user: expected 204, got %d", resp.StatusCode)
	}
	resp.Body.Close()

	resp = env.do("POST", "/auth/login", map[string]string{
		"username": "locktest", "password": "S3cur3P@ssw0rd!XYZ",
	}, "")
	if resp.StatusCode != http.StatusOK {
		resp.Body.Close()
		t.Fatalf("unlocked login: expected 200, got %d", resp.StatusCode)
	}
	resp.Body.Close()
}

// TestAdminPasswordResetSetsForceChange tests admin resets password → force_password_change set.
func TestAdminPasswordResetSetsForceChange(t *testing.T) {
	env := setupTestEnv(t)
	resetDB(t, env.db)
	adminToken := mustSetup(t, env)

	operatorRoleID := getOperatorRoleID(t, env, adminToken)
	resp := env.do("POST", "/users", map[string]string{
		"username":     "pwresettest",
		"email":        "pwreset@example.com",
		"display_name": "PW Reset Test",
		"password":     "S3cur3P@ssw0rd!XYZ",
		"role_id":      operatorRoleID,
	}, adminToken)
	var created struct {
		Data struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	decodeT(t, resp, &created)
	userID := created.Data.ID

	resp = env.do("PUT", "/users/"+userID+"/password", map[string]string{
		"new_password": "N3wSecureP@ssw0rd!9",
	}, adminToken)
	if resp.StatusCode != http.StatusNoContent {
		resp.Body.Close()
		t.Fatalf("admin reset password: expected 204, got %d", resp.StatusCode)
	}
	resp.Body.Close()

	var forceChange bool
	err := env.db.QueryRowContext(context.Background(),
		`SELECT force_password_change FROM users WHERE id = $1`, userID,
	).Scan(&forceChange)
	if err != nil {
		t.Fatalf("query force_password_change: %v", err)
	}
	if !forceChange {
		t.Fatal("expected force_password_change = TRUE after admin password reset")
	}
}

// TestAuditLogEntriesCreated tests that auth events produce audit log entries.
func TestAuditLogEntriesCreated(t *testing.T) {
	env := setupTestEnv(t)
	resetDB(t, env.db)
	mustSetup(t, env)

	time.Sleep(200 * time.Millisecond)

	var count int
	err := env.db.QueryRowContext(context.Background(),
		`SELECT COUNT(*) FROM audit_logs WHERE action = 'system.setup.complete'`,
	).Scan(&count)
	if err != nil {
		t.Fatalf("count audit logs: %v", err)
	}
	if count == 0 {
		t.Fatal("expected at least one audit log entry for system.setup.complete")
	}
}

// TestAuditLogHMACIntegrity verifies all audit log entries have valid HMAC checksums.
func TestAuditLogHMACIntegrity(t *testing.T) {
	env := setupTestEnv(t)
	resetDB(t, env.db)
	mustSetup(t, env)

	resp := env.do("POST", "/auth/login", map[string]string{
		"username": "admin", "password": "S3cur3P@ssw0rd!XYZ",
	}, "")
	resp.Body.Close()

	time.Sleep(200 * time.Millisecond)

	rows, err := env.db.QueryContext(context.Background(), `
		SELECT id, timestamp, category, severity, actor_type,
		       actor_id, actor_label, action, resource_type, resource_id,
		       details, correlation_id, source_ip, session_id, campaign_id, checksum
		FROM audit_logs LIMIT 100`)
	if err != nil {
		t.Fatalf("query audit logs: %v", err)
	}
	defer rows.Close()

	checked := 0
	for rows.Next() {
		entry := scanAuditEntry(t, rows)
		if !env.auditHMAC.Verify(entry) {
			t.Errorf("audit log entry %s (action=%s) has invalid HMAC checksum", entry.ID, entry.Action)
		}
		checked++
	}
	if checked == 0 {
		t.Fatal("no audit log entries found to verify")
	}
}

// TestInitialAdminProtected tests initial admin cannot be deleted or deactivated.
func TestInitialAdminProtected(t *testing.T) {
	env := setupTestEnv(t)
	resetDB(t, env.db)
	adminToken := mustSetup(t, env)

	resp := env.do("GET", "/users", nil, adminToken)
	var usersResp struct {
		Data struct {
			Data []struct {
				ID             string `json:"id"`
				IsInitialAdmin bool   `json:"is_initial_admin"`
			} `json:"data"`
		} `json:"data"`
	}
	decodeT(t, resp, &usersResp)
	var adminID string
	for _, u := range usersResp.Data.Data {
		if u.IsInitialAdmin {
			adminID = u.ID
			break
		}
	}
	if adminID == "" {
		t.Fatal("initial admin user not found")
	}

	resp = env.do("DELETE", "/users/"+adminID, nil, adminToken)
	if resp.StatusCode == http.StatusNoContent {
		resp.Body.Close()
		t.Fatal("expected delete of initial admin to fail, but got 204")
	}
	resp.Body.Close()

	resp = env.do("PUT", "/users/"+adminID, map[string]any{"status": "inactive"}, adminToken)
	if resp.StatusCode == http.StatusNoContent {
		resp.Body.Close()
		t.Fatal("expected deactivation of initial admin to fail, but got 204")
	}
	resp.Body.Close()
}

// TestUnauthenticatedRejected tests protected endpoints return 401 without token.
func TestUnauthenticatedRejected(t *testing.T) {
	env := setupTestEnv(t)
	resetDB(t, env.db)
	mustSetup(t, env)

	endpoints := []struct{ method, path string }{
		{"GET", "/users"},
		{"GET", "/roles"},
		{"GET", "/logs/audit"},
		{"GET", "/auth/me"},
		{"GET", "/notifications"},
	}
	for _, ep := range endpoints {
		resp := env.do(ep.method, ep.path, nil, "")
		if resp.StatusCode != http.StatusUnauthorized {
			resp.Body.Close()
			t.Errorf("%s %s: expected 401, got %d", ep.method, ep.path, resp.StatusCode)
			continue
		}
		resp.Body.Close()
	}
}

// TestResponseNeverExposesPasswordHash verifies no endpoint returns password_hash.
func TestResponseNeverExposesPasswordHash(t *testing.T) {
	env := setupTestEnv(t)
	resetDB(t, env.db)
	adminToken := mustSetup(t, env)

	endpoints := []struct{ method, path string }{
		{"GET", "/users"},
		{"GET", "/auth/me"},
		{"GET", "/roles"},
	}
	for _, ep := range endpoints {
		resp := env.do(ep.method, ep.path, nil, adminToken)
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		if bytes.Contains(body, []byte(`"password_hash"`)) {
			t.Errorf("%s %s: response body contains 'password_hash'", ep.method, ep.path)
		}
	}
}

// TestErrorResponseNoStackTrace verifies error responses don't leak internal details.
func TestErrorResponseNoStackTrace(t *testing.T) {
	env := setupTestEnv(t)
	resetDB(t, env.db)
	mustSetup(t, env)

	resp := env.do("GET", "/users", nil, "not.a.valid.token")
	body, _ := io.ReadAll(resp.Body)
	resp.Body.Close()

	forbidden := []string{"goroutine", "runtime/debug", ".go:", "panic:"}
	for _, s := range forbidden {
		if bytes.Contains(body, []byte(s)) {
			t.Errorf("error response contains internal detail %q: %s", s, body)
		}
	}
}

// TestRateLimitHeaders verifies X-RateLimit-* headers are present.
func TestRateLimitHeaders(t *testing.T) {
	env := setupTestEnv(t)
	resetDB(t, env.db)
	adminToken := mustSetup(t, env)

	resp := env.do("GET", "/users", nil, adminToken)
	resp.Body.Close()

	for _, h := range []string{"X-RateLimit-Limit", "X-RateLimit-Remaining", "X-RateLimit-Reset"} {
		if resp.Header.Get(h) == "" {
			t.Errorf("missing header %s", h)
		}
	}
}

// TestCorrelationIDHeader verifies X-Correlation-ID and X-Request-ID are present.
func TestCorrelationIDHeader(t *testing.T) {
	env := setupTestEnv(t)
	resetDB(t, env.db)

	resp := env.do("GET", "/health", nil, "")
	resp.Body.Close()

	if resp.Header.Get("X-Correlation-ID") == "" {
		t.Error("missing X-Correlation-ID header")
	}
	if resp.Header.Get("X-Request-ID") == "" {
		t.Error("missing X-Request-ID header")
	}
	if resp.Header.Get("X-Correlation-ID") != resp.Header.Get("X-Request-ID") {
		t.Error("X-Correlation-ID and X-Request-ID should match")
	}
}

// TestTokenBlacklistBlocksRevoked verifies after logout the access token is rejected.
func TestTokenBlacklistBlocksRevoked(t *testing.T) {
	env := setupTestEnv(t)
	resetDB(t, env.db)
	mustSetup(t, env)

	resp := env.do("POST", "/auth/login", map[string]string{
		"username": "admin", "password": "S3cur3P@ssw0rd!XYZ",
	}, "")
	var lr struct {
		Data struct {
			AccessToken string `json:"access_token"`
		} `json:"data"`
	}
	decodeT(t, resp, &lr)
	token := lr.Data.AccessToken

	resp = env.do("GET", "/auth/me", nil, token)
	if resp.StatusCode != http.StatusOK {
		resp.Body.Close()
		t.Fatalf("pre-logout auth/me: expected 200, got %d", resp.StatusCode)
	}
	resp.Body.Close()

	resp = env.do("POST", "/auth/logout", nil, token)
	resp.Body.Close()

	resp = env.do("GET", "/auth/me", nil, token)
	if resp.StatusCode != http.StatusUnauthorized {
		resp.Body.Close()
		t.Fatalf("post-logout auth/me: expected 401, got %d", resp.StatusCode)
	}
	resp.Body.Close()
}

// ----- private helpers -----

func mustSetup(t *testing.T, env *testEnv) string {
	t.Helper()
	resp := env.do("POST", "/setup", map[string]string{
		"username":              "admin",
		"email":                 "admin@example.com",
		"display_name":          "Administrator",
		"password":              "S3cur3P@ssw0rd!XYZ",
		"password_confirmation": "S3cur3P@ssw0rd!XYZ",
	}, "")
	if resp.StatusCode != http.StatusCreated {
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		t.Fatalf("mustSetup: expected 201, got %d: %s", resp.StatusCode, body)
	}
	var sr struct {
		Data struct {
			AccessToken string `json:"access_token"`
		} `json:"data"`
	}
	decodeT(t, resp, &sr)
	return sr.Data.AccessToken
}

func getOperatorRoleID(t *testing.T, env *testEnv, adminToken string) string {
	t.Helper()
	resp := env.do("GET", "/roles", nil, adminToken)
	var rolesResp struct {
		Data struct {
			Data []struct {
				ID   string `json:"id"`
				Name string `json:"name"`
			} `json:"data"`
		} `json:"data"`
	}
	decodeT(t, resp, &rolesResp)
	for _, r := range rolesResp.Data.Data {
		if r.Name == "operator" {
			return r.ID
		}
	}
	t.Fatal("operator role not found")
	return ""
}

func scanAuditEntry(t *testing.T, rows *sql.Rows) *auditsvc.LogEntry {
	t.Helper()
	var (
		e          auditsvc.LogEntry
		category   string
		severity   string
		actorType  string
		detailsRaw []byte
		corrID     sql.NullString
		sourceIP   sql.NullString
		sessionID  sql.NullString
		campaignID sql.NullString
		actorID    sql.NullString
		actorLabel sql.NullString
		resType    sql.NullString
		resID      sql.NullString
	)
	err := rows.Scan(
		&e.ID, &e.Timestamp, &category, &severity, &actorType,
		&actorID, &actorLabel, &e.Action, &resType, &resID,
		&detailsRaw, &corrID, &sourceIP, &sessionID, &campaignID, &e.Checksum,
	)
	if err != nil {
		t.Fatalf("scan audit entry: %v", err)
	}
	e.Category = auditsvc.Category(category)
	e.Severity = auditsvc.Severity(severity)
	e.ActorType = auditsvc.ActorType(actorType)
	if actorID.Valid {
		s := actorID.String
		e.ActorID = &s
	}
	if actorLabel.Valid {
		e.ActorLabel = actorLabel.String
	}
	if resType.Valid {
		s := resType.String
		e.ResourceType = &s
	}
	if resID.Valid {
		s := resID.String
		e.ResourceID = &s
	}
	if corrID.Valid {
		e.CorrelationID = corrID.String
	}
	if sourceIP.Valid {
		s := sourceIP.String
		e.SourceIP = &s
	}
	if sessionID.Valid {
		s := sessionID.String
		e.SessionID = &s
	}
	if campaignID.Valid {
		s := campaignID.String
		e.CampaignID = &s
	}
	if len(detailsRaw) > 0 {
		json.Unmarshal(detailsRaw, &e.Details) //nolint:errcheck
	}
	return &e
}
