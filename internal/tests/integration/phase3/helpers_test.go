//go:build integration

// Package phase3 contains end-to-end integration tests for Phase 3 Campaign Engine subsystems.
// Requires a real PostgreSQL database with all migrations applied (through 044).
// Run with: go test -tags=integration ./internal/tests/integration/phase3/...
// Requires TEST_DATABASE_URL environment variable.
package phase3

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

// testEnv holds state shared across Phase 3 integration tests.
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

// resetPhase3DB clears all Phase 3 and supporting data for a clean test run.
// Truncates each table individually so a missing table doesn't block the rest.
func resetPhase3DB(tb testing.TB, db *sql.DB) {
	tb.Helper()

	// Tables in reverse dependency order. CASCADE handles FK constraints.
	tables := []string{
		// Credential capture.
		"session_captures",
		"capture_fields",
		"capture_events",
		"field_categorization_rules",
		// Email delivery.
		"email_delivery_events",
		"campaign_smtp_send_counts",
		// Endpoints.
		"phishing_reports",
		"endpoint_request_logs",
		"endpoint_heartbeats",
		"endpoint_state_transitions",
		"endpoint_tls_certificates",
		"endpoint_ssh_keys",
		"proxmox_ip_allocations",
		"phishing_endpoints",
		// Campaign approvals.
		"campaign_approval_requirements",
		"campaign_approvals",
		// Campaign internals.
		"campaign_build_logs",
		"campaign_state_transitions",
		"campaign_send_windows",
		"campaign_template_variants",
		"campaign_emails",
		"campaign_target_events",
		"campaign_targets",
		"campaign_targets_snapshot",
		"campaign_target_variant_assignments",
		"campaign_target_groups",
		"campaign_config_templates",
		"campaign_canary_targets",
		"campaign_send_schedules",
		"campaign_smtp_profiles",
		"campaigns",
		// Landing pages.
		"landing_page_health_checks",
		"landing_page_builds",
		"landing_page_templates",
		"landing_page_projects",
		// Targets & groups.
		"blocklist_overrides",
		"blocklist_entries",
		"target_group_members",
		"target_groups",
		"target_imports",
		"import_mapping_templates",
		"targets",
		// Auth.
		"auth_identities",
		"role_mappings",
		"auth_providers",
		// Infrastructure.
		"email_template_versions",
		"email_templates",
		"smtp_profiles",
		"instance_template_versions",
		"instance_templates",
		"cloud_credentials",
		// Domains.
		"domain_health_checks",
		"domain_categorizations",
		"domain_email_auth_status",
		"dkim_keys",
		"dns_propagation_checks",
		"dns_records",
		"domain_registration_requests",
		"domain_campaign_associations",
		"domain_renewal_history",
		"domain_profiles",
		"domain_provider_connections",
		// Notifications.
		"webhook_deliveries",
		"webhook_endpoints",
		"notification_preferences",
		"notification_smtp_config",
		"notifications",
		// Sessions & users.
		"password_history",
		"api_keys",
		"sessions",
		"user_roles",
		"users",
	}

	for _, tbl := range tables {
		_, _ = db.ExecContext(context.Background(), "TRUNCATE TABLE "+tbl+" CASCADE")
	}

	// Clear audit log partitions.
	rows, err := db.QueryContext(context.Background(),
		`SELECT tablename FROM pg_tables WHERE tablename LIKE 'audit_logs_%' AND schemaname = 'public'`)
	if err == nil {
		defer rows.Close()
		for rows.Next() {
			var tbl string
			rows.Scan(&tbl) //nolint:errcheck
			db.ExecContext(context.Background(), "TRUNCATE TABLE "+tbl) //nolint:errcheck
		}
	}

	// Reset system_config to allow re-setup.
	db.ExecContext(context.Background(), `DELETE FROM system_config WHERE key = 'setup_complete'`) //nolint:errcheck
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
		Data []roleInfo `json:"data"`
	}
	decodeT(t, resp, &rolesResp)
	return rolesResp.Data
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
		var id, resType, resID, actorLabel string
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
			"id":            id,
			"action":        action,
			"resource_type": resType,
			"resource_id":   resID,
			"actor_label":   actorLabel,
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

// bodyContains returns true if the response body bytes contain the substring s.
func bodyContains(body []byte, s string) bool {
	return strings.Contains(string(body), s)
}

// ptrStr returns a pointer to the given string.
func ptrStr(s string) *string { return &s }

// ptrInt returns a pointer to the given int.
func ptrInt(i int) *int { return &i }

// ptrBool returns a pointer to the given bool.
func ptrBool(b bool) *bool { return &b }

// --- Phase 3 specific helpers ---

// mustCreateTarget creates a target and returns its ID.
func mustCreateTarget(t *testing.T, env *testEnv, token string, email, firstName, lastName, department string) string {
	t.Helper()
	resp := env.do("POST", "/targets", map[string]any{
		"email":      email,
		"first_name": firstName,
		"last_name":  lastName,
		"department": department,
	}, token)
	if resp.StatusCode != http.StatusCreated {
		body := mustBody(resp)
		t.Fatalf("mustCreateTarget(%s): expected 201, got %d: %s", email, resp.StatusCode, body)
	}
	var cr struct {
		Data struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	decodeT(t, resp, &cr)
	return cr.Data.ID
}

// mustCreateTargetGroup creates a target group and returns its ID.
func mustCreateTargetGroup(t *testing.T, env *testEnv, token, name, description string) string {
	t.Helper()
	resp := env.do("POST", "/target-groups", map[string]any{
		"name":        name,
		"description": description,
	}, token)
	if resp.StatusCode != http.StatusCreated {
		body := mustBody(resp)
		t.Fatalf("mustCreateTargetGroup(%s): expected 201, got %d: %s", name, resp.StatusCode, body)
	}
	var cr struct {
		Data struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	decodeT(t, resp, &cr)
	return cr.Data.ID
}

// mustCreateDomainProfile creates a domain profile directly in the DB and returns its ID.
// Requires a valid user ID for created_by — retrieves the admin user.
func mustCreateDomainProfile(t *testing.T, env *testEnv) string {
	t.Helper()
	var adminUID string
	if err := env.db.QueryRow(`SELECT id FROM users WHERE username = 'admin' LIMIT 1`).Scan(&adminUID); err != nil {
		t.Fatalf("get admin for domain profile: %v", err)
	}
	var id string
	err := env.db.QueryRow(`
		INSERT INTO domain_profiles (id, domain_name, status, created_by, created_at, updated_at)
		VALUES (gen_random_uuid(), 'test-phishing-' || substr(gen_random_uuid()::text, 1, 8) || '.com', 'active', $1, NOW(), NOW())
		RETURNING id
	`, adminUID).Scan(&id)
	if err != nil {
		t.Fatalf("create domain profile: %v", err)
	}
	return id
}

// mustCreateCampaign creates a campaign in draft state and returns its ID.
func mustCreateCampaign(t *testing.T, env *testEnv, token, name string) string {
	t.Helper()
	domainID := mustCreateDomainProfile(t, env)
	resp := env.do("POST", "/campaigns", map[string]any{
		"name":               name,
		"description":        "Integration test campaign",
		"send_order":         "default",
		"cloud_provider":     "aws",
		"region":             "us-east-1",
		"instance_type":      "t3.micro",
		"endpoint_domain_id": domainID,
		"start_date":         "2026-04-01T00:00:00Z",
		"end_date":           "2026-04-30T23:59:59Z",
		"grace_period_hours": 72,
	}, token)
	if resp.StatusCode != http.StatusCreated {
		body := mustBody(resp)
		t.Fatalf("mustCreateCampaign(%s): expected 201, got %d: %s", name, resp.StatusCode, body)
	}
	var cr struct {
		Data struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	decodeT(t, resp, &cr)
	return cr.Data.ID
}

// mustCreateBlocklistEntry creates a block list entry and returns its ID.
func mustCreateBlocklistEntry(t *testing.T, env *testEnv, token, pattern, reason string) string {
	t.Helper()
	resp := env.do("POST", "/blocklist", map[string]any{
		"pattern": pattern,
		"reason":  reason,
	}, token)
	if resp.StatusCode != http.StatusCreated {
		body := mustBody(resp)
		t.Fatalf("mustCreateBlocklistEntry(%s): expected 201, got %d: %s", pattern, resp.StatusCode, body)
	}
	var cr struct {
		Data struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	decodeT(t, resp, &cr)
	return cr.Data.ID
}

// mustPrepareCampaignForSubmission creates a campaign with all required config
// (targets, template variant, send windows) so it can be submitted.
func mustPrepareCampaignForSubmission(t *testing.T, env *testEnv, token, name string) string {
	t.Helper()
	campaignID := mustCreateCampaign(t, env, token, name)

	// Create a target and assign it via a group.
	// Sanitize name for email by replacing spaces with dashes and lowercasing.
	emailSafe := strings.ToLower(strings.ReplaceAll(name, " ", "-"))
	targetID := mustCreateTarget(t, env, token, emailSafe+"@test.com", "Test", "User", "IT")
	groupID := mustCreateTargetGroup(t, env, token, name+" Group", "Test group")
	resp := env.do("POST", "/target-groups/"+groupID+"/members", map[string]any{
		"target_ids": []string{targetID},
	}, token)
	assertStatus(t, resp, http.StatusOK)
	resp.Body.Close()
	resp = env.do("POST", "/campaigns/"+campaignID+"/target-groups", map[string]any{
		"group_id": groupID,
	}, token)
	assertStatus(t, resp, http.StatusOK)
	resp.Body.Close()

	// Create an email template and assign as variant.
	var templateID string
	err := env.db.QueryRow(`
		SELECT id FROM email_templates LIMIT 1
	`).Scan(&templateID)
	if err != nil {
		// Create one.
		var adminUID string
		env.db.QueryRow(`SELECT id FROM users WHERE username = 'admin' LIMIT 1`).Scan(&adminUID)
		env.db.QueryRow(`
			INSERT INTO email_templates (id, name, subject, html_body, text_body, created_by, created_at, updated_at)
			VALUES (gen_random_uuid(), $1, 'Test Subject', '<p>Hello {{first_name}}</p>', 'Hello {{first_name}}', $2, NOW(), NOW())
			RETURNING id
		`, emailSafe+"-template", adminUID).Scan(&templateID)
	}

	resp = env.do("PUT", "/campaigns/"+campaignID+"/template-variants", map[string]any{
		"variants": []map[string]any{
			{"template_id": templateID, "split_ratio": 100, "label": "Primary"},
		},
	}, token)
	if resp.StatusCode != http.StatusOK {
		body := mustBody(resp)
		t.Fatalf("set template variants: %d: %s", resp.StatusCode, body)
	}
	resp.Body.Close()

	return campaignID
}

// getCampaignState fetches a campaign and returns its current state.
func getCampaignState(t *testing.T, env *testEnv, token, campaignID string) string {
	t.Helper()
	resp := env.do("GET", "/campaigns/"+campaignID, nil, token)
	assertStatus(t, resp, http.StatusOK)
	var gr struct {
		Data struct {
			CurrentState string `json:"current_state"`
		} `json:"data"`
	}
	decodeT(t, resp, &gr)
	return gr.Data.CurrentState
}
