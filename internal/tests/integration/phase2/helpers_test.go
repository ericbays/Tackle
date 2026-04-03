//go:build integration

// Package phase2 contains end-to-end integration tests for Phase 2 infrastructure subsystems.
// Requires a real PostgreSQL database with all Phase 2 migrations applied.
// Run with: go test -tags=integration ./internal/tests/integration/phase2/...
// Requires TEST_DATABASE_URL environment variable.
package phase2

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	_ "github.com/lib/pq"

	"tackle/internal/config"
	"tackle/internal/crypto"
	auditsvc "tackle/internal/services/audit"
	"tackle/internal/server"
)

// testEnv holds state shared across Phase 2 integration tests.
type testEnv struct {
	srv       *httptest.Server
	db        *sql.DB
	masterKey []byte
	auditHMAC *auditsvc.HMACService
}

// tokens holds login tokens for the four roles used in tests.
type tokens struct {
	admin    string
	engineer string
	operator string
	defender string
}

// setupTestEnv creates a test environment with a real PostgreSQL database and an httptest server.
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

// testMasterKey returns a deterministic 32-byte key for tests.
func testMasterKey() []byte {
	key := make([]byte, 32)
	for i := range key {
		key[i] = byte(i + 1)
	}
	return key
}

// newTestLogger returns a slog.Logger that writes to the test log.
func newTestLogger(t *testing.T) *slog.Logger {
	t.Helper()
	return slog.New(slog.NewTextHandler(testWriter{t}, &slog.HandlerOptions{Level: slog.LevelWarn}))
}

type testWriter struct{ t *testing.T }

func (tw testWriter) Write(p []byte) (int, error) {
	tw.t.Log(string(p))
	return len(p), nil
}

// resetPhase2DB clears all Phase 2 data for a clean test run.
// Clears in reverse dependency order.
func resetPhase2DB(tb testing.TB, db *sql.DB) {
	tb.Helper()
	tables := []string{
		// Phase 2 tables
		"auth_identities",
		"role_mappings",
		"auth_providers",
		"email_template_versions",
		"email_templates",
		"campaign_smtp_profiles",
		"send_schedules",
		"smtp_profiles",
		"instance_template_versions",
		"instance_templates",
		"cloud_credentials",
		"domain_health_checks",
		"domain_categorizations",
		"typosquat_results",
		"dns_records",
		"domain_profiles",
		"domain_providers",
		// Phase 1 tables
		"audit_logs",
		"sessions",
		"user_roles",
		"notifications",
		"users",
		"settings",
	}
	for _, tbl := range tables {
		if _, err := db.ExecContext(context.Background(), "DELETE FROM "+tbl); err != nil {
			tb.Logf("resetPhase2DB: delete from %s: %v (may not exist)", tbl, err)
		}
	}
}

// mustSetup runs the setup wizard and returns the admin token.
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

// mustLogin logs in with the given credentials and returns the access token.
func mustLogin(t *testing.T, env *testEnv, username, password string) string {
	t.Helper()
	resp := env.do("POST", "/auth/login", map[string]string{
		"username": username,
		"password": password,
	}, "")
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		t.Fatalf("mustLogin(%s): expected 200, got %d: %s", username, resp.StatusCode, body)
	}
	var lr struct {
		Data struct {
			AccessToken string `json:"access_token"`
		} `json:"data"`
	}
	decodeT(t, resp, &lr)
	return lr.Data.AccessToken
}

// setupRoles creates users with each role and returns their tokens.
// adminToken must be the token of the initial admin created by mustSetup.
func setupRoles(t *testing.T, env *testEnv, adminToken string) tokens {
	t.Helper()

	roles := listRoles(t, env, adminToken)

	engineerRoleID := findRoleID(t, roles, "engineer")
	operatorRoleID := findRoleID(t, roles, "operator")
	defenderRoleID := findRoleID(t, roles, "defender")

	createUser(t, env, adminToken, "engineer1", "engineer1@example.com", "Engineer One", "S3cur3P@ssw0rd!XYZ", engineerRoleID)
	createUser(t, env, adminToken, "operator1", "operator1@example.com", "Operator One", "S3cur3P@ssw0rd!XYZ", operatorRoleID)
	createUser(t, env, adminToken, "defender1", "defender1@example.com", "Defender One", "S3cur3P@ssw0rd!XYZ", defenderRoleID)

	return tokens{
		admin:    adminToken,
		engineer: mustLogin(t, env, "engineer1", "S3cur3P@ssw0rd!XYZ"),
		operator: mustLogin(t, env, "operator1", "S3cur3P@ssw0rd!XYZ"),
		defender: mustLogin(t, env, "defender1", "S3cur3P@ssw0rd!XYZ"),
	}
}

type roleInfo struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

func listRoles(t *testing.T, env *testEnv, adminToken string) []roleInfo {
	t.Helper()
	resp := env.do("GET", "/roles", nil, adminToken)
	var rolesResp struct {
		Data struct {
			Data []roleInfo `json:"data"`
		} `json:"data"`
	}
	decodeT(t, resp, &rolesResp)
	return rolesResp.Data.Data
}

func findRoleID(t *testing.T, roles []roleInfo, name string) string {
	t.Helper()
	for _, r := range roles {
		if r.Name == name {
			return r.ID
		}
	}
	t.Fatalf("role %q not found", name)
	return ""
}

func createUser(t *testing.T, env *testEnv, adminToken, username, email, displayName, password, roleID string) string {
	t.Helper()
	resp := env.do("POST", "/users", map[string]string{
		"username":     username,
		"email":        email,
		"display_name": displayName,
		"password":     password,
		"role_id":      roleID,
	}, adminToken)
	if resp.StatusCode != http.StatusCreated {
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		t.Fatalf("createUser(%s): expected 201, got %d: %s", username, resp.StatusCode, body)
	}
	var cr struct {
		Data struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	decodeT(t, resp, &cr)
	return cr.Data.ID
}

// decodeT reads and JSON-decodes a response body in a test context.
func decodeT(t *testing.T, resp *http.Response, dst any) {
	t.Helper()
	defer resp.Body.Close()
	if err := json.NewDecoder(resp.Body).Decode(dst); err != nil {
		t.Fatalf("decode response: %v", err)
	}
}

// mustBody reads and returns the response body bytes, closing the body.
func mustBody(resp *http.Response) []byte {
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	return b
}

// assertStatus fails the test if the response status code doesn't match.
func assertStatus(t *testing.T, resp *http.Response, want int) {
	t.Helper()
	if resp.StatusCode != want {
		body := mustBody(resp)
		t.Fatalf("expected status %d, got %d: %s", want, resp.StatusCode, body)
	}
}

// assertStatusQuiet fails if status doesn't match, closing body without reading.
func assertStatusQuiet(t *testing.T, resp *http.Response, want int) {
	t.Helper()
	if resp.StatusCode != want {
		body := mustBody(resp)
		t.Fatalf("expected status %d, got %d: %s", want, resp.StatusCode, body)
	}
	resp.Body.Close()
}

// waitForAudit sleeps briefly to allow the async audit service to flush.
func waitForAudit() {
	time.Sleep(250 * time.Millisecond)
}

// queryAuditEvents returns audit log entries for a given action, ordered by timestamp desc.
func queryAuditEvents(t *testing.T, db *sql.DB, action string) []map[string]any {
	t.Helper()
	rows, err := db.QueryContext(context.Background(),
		`SELECT id, action, resource_type, resource_id, actor_label, details
		 FROM audit_logs WHERE action = $1 ORDER BY timestamp DESC`, action)
	if err != nil {
		t.Fatalf("query audit logs (action=%s): %v", action, err)
	}
	defer rows.Close()

	var results []map[string]any
	for rows.Next() {
		var id, action, resType, resID, actorLabel string
		var resTypeN, resIDN, actorN sql.NullString
		var detailsRaw []byte
		if err := rows.Scan(&id, &action, &resTypeN, &resIDN, &actorN, &detailsRaw); err != nil {
			t.Fatalf("scan audit row: %v", err)
		}
		if resTypeN.Valid {
			resType = resTypeN.String
		}
		if resIDN.Valid {
			resID = resIDN.String
		}
		if actorN.Valid {
			actorLabel = actorN.String
		}
		entry := map[string]any{
			"id":           id,
			"action":       action,
			"resource_type": resType,
			"resource_id":  resID,
			"actor_label":  actorLabel,
		}
		if len(detailsRaw) > 0 {
			var details map[string]any
			json.Unmarshal(detailsRaw, &details) //nolint:errcheck
			entry["details"] = details
		}
		results = append(results, entry)
	}
	return results
}

// countAuditEvents returns the count of audit entries matching the given action.
func countAuditEvents(t *testing.T, db *sql.DB, action string) int {
	t.Helper()
	var count int
	err := db.QueryRowContext(context.Background(),
		`SELECT COUNT(*) FROM audit_logs WHERE action = $1`, action).Scan(&count)
	if err != nil {
		t.Fatalf("count audit events (action=%s): %v", action, err)
	}
	return count
}

// assertAuditEvent fails if no audit entry with the given action exists.
func assertAuditEvent(t *testing.T, db *sql.DB, action string) {
	t.Helper()
	waitForAudit()
	count := countAuditEvents(t, db, action)
	if count == 0 {
		t.Fatalf("expected audit event %q but found none", action)
	}
}

// ptrStr returns a pointer to the given string.
func ptrStr(s string) *string { return &s }

// ptrInt returns a pointer to the given int.
func ptrInt(i int) *int { return &i }

// ptrBool returns a pointer to the given bool.
func ptrBool(b bool) *bool { return &b }

// bodyContains returns true if the response body bytes contain the substring s.
func bodyContains(body []byte, s string) bool {
	return strings.Contains(string(body), s)
}
