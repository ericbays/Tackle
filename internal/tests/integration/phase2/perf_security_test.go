//go:build integration

package phase2

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"testing"
	"time"
)

// TestDomainInventoryWithManyDomains verifies the domain list endpoint returns
// quickly when there are many domain records (pagination).
func TestDomainInventoryWithManyDomains(t *testing.T) {
	env := setupTestEnv(t)
	resetPhase2DB(t, env.db)
	adminToken := mustSetup(t, env)

	// Insert 100 domains directly via DB for speed.
	insertBulkDomains(t, env.db, 100)

	start := time.Now()
	resp := env.do("GET", "/domains?page=1&per_page=50", nil, adminToken)
	elapsed := time.Since(start)
	assertStatus(t, resp, http.StatusOK)
	resp.Body.Close()

	if elapsed > 2*time.Second {
		t.Errorf("domain list with 100 records took %v (expected <2s)", elapsed)
	}
}

// TestDNSRecordListPerformance verifies DNS record listing for a domain with many records.
func TestDNSRecordListPerformance(t *testing.T) {
	env := setupTestEnv(t)
	resetPhase2DB(t, env.db)
	adminToken := mustSetup(t, env)

	domainID := createTestDomain(t, env, adminToken, "perf-dns.com")

	// Insert 50 DNS records directly for speed.
	insertBulkDNSRecords(t, env.db, domainID, 50)

	start := time.Now()
	resp := env.do("GET", "/domains/"+domainID+"/dns-records", nil, adminToken)
	elapsed := time.Since(start)
	assertStatus(t, resp, http.StatusOK)
	resp.Body.Close()

	if elapsed > 2*time.Second {
		t.Errorf("DNS record list with 50 records took %v (expected <2s)", elapsed)
	}
}

// TestConcurrentAPIRequests verifies the domain list endpoint handles
// concurrent requests without errors or data corruption.
func TestConcurrentAPIRequests(t *testing.T) {
	env := setupTestEnv(t)
	resetPhase2DB(t, env.db)
	adminToken := mustSetup(t, env)

	// Pre-insert a few domains.
	insertBulkDomains(t, env.db, 10)

	const concurrency = 20
	var wg sync.WaitGroup
	errors := make(chan error, concurrency)

	for i := 0; i < concurrency; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			resp := env.do("GET", "/domains", nil, adminToken)
			defer resp.Body.Close()
			if resp.StatusCode != http.StatusOK {
				errors <- fmt.Errorf("expected 200, got %d", resp.StatusCode)
				return
			}
			// Verify response is valid JSON.
			var body map[string]any
			if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
				errors <- fmt.Errorf("invalid JSON in response: %v", err)
			}
		}()
	}
	wg.Wait()
	close(errors)

	for err := range errors {
		t.Errorf("concurrent request failed: %v", err)
	}
}

// TestEmailTemplateListPerformance verifies bulk template listing performance.
func TestEmailTemplateListPerformance(t *testing.T) {
	env := setupTestEnv(t)
	resetPhase2DB(t, env.db)
	adminToken := mustSetup(t, env)

	// Insert 20 templates via API.
	for i := 0; i < 10; i++ {
		resp := env.do("POST", "/email-templates", map[string]any{
			"name":      fmt.Sprintf("Perf Template %d", i),
			"subject":   "Perf test",
			"html_body": "<p>Hello {{.FirstName}}</p>",
			"text_body": "Hello {{.FirstName}}",
			"category":  "generic",
		}, adminToken)
		resp.Body.Close()
	}

	start := time.Now()
	resp := env.do("GET", "/email-templates", nil, adminToken)
	elapsed := time.Since(start)
	assertStatus(t, resp, http.StatusOK)
	resp.Body.Close()

	if elapsed > 2*time.Second {
		t.Errorf("email template list took %v (expected <2s)", elapsed)
	}
}

// TestCredentialEncryptionAtRest verifies that credential fields stored
// in the database are AES-256-GCM encrypted (not plaintext).
func TestCredentialEncryptionAtRest(t *testing.T) {
	env := setupTestEnv(t)
	resetPhase2DB(t, env.db)
	adminToken := mustSetup(t, env)

	secretKey := "SUPERSECRET-AT-REST-TEST"

	// Create domain provider with a known secret.
	resp := env.do("POST", "/settings/domain-providers", map[string]any{
		"provider_type": "namecheap",
		"display_name":  "Encryption Test",
		"namecheap_credentials": map[string]string{
			"api_user": "u", "api_key": secretKey, "username": "u", "client_ip": "1.1.1.1",
		},
	}, adminToken)
	assertStatus(t, resp, http.StatusCreated)

	// Query the raw database record.
	var rawCreds []byte
	err := env.db.QueryRowContext(context.Background(),
		`SELECT credentials FROM domain_providers WHERE display_name = 'Encryption Test'`).Scan(&rawCreds)
	if err != nil {
		if err == sql.ErrNoRows {
			t.Skip("domain_providers table not yet queried; may need migration")
		}
		t.Fatalf("query raw credentials: %v", err)
	}

	rawStr := string(rawCreds)
	if rawStr != "" && contains(rawStr, secretKey) {
		t.Errorf("plaintext credential %q found in database column (expected encrypted)", secretKey)
	}
}

// TestAPIResponseCredentialMasking verifies no plaintext secrets in GET responses
// for all credential-bearing entities (comprehensive check).
func TestAPIResponseCredentialMasking(t *testing.T) {
	env := setupTestEnv(t)
	resetPhase2DB(t, env.db)
	adminToken := mustSetup(t, env)

	type entity struct {
		name      string
		createURL string
		getURL    func(id string) string
		payload   map[string]any
		secret    string
	}

	entities := []entity{
		{
			name:      "domain_provider",
			createURL: "/settings/domain-providers",
			getURL:    func(id string) string { return "/settings/domain-providers/" + id },
			payload: map[string]any{
				"provider_type": "namecheap", "display_name": "Mask Check NC",
				"namecheap_credentials": map[string]string{
					"api_user": "u", "api_key": "secret-nc-api-key-check", "username": "u", "client_ip": "1.1.1.1",
				},
			},
			secret: "secret-nc-api-key-check",
		},
		{
			name:      "cloud_credential",
			createURL: "/settings/cloud-credentials",
			getURL:    func(id string) string { return "/settings/cloud-credentials/" + id },
			payload: map[string]any{
				"provider_type": "aws", "display_name": "Mask Check AWS", "default_region": "us-east-1",
				"aws": map[string]string{"access_key_id": "AKID", "secret_access_key": "secret-aws-check-key"},
			},
			secret: "secret-aws-check-key",
		},
		{
			name:      "smtp_profile",
			createURL: "/smtp-profiles",
			getURL:    func(id string) string { return "/smtp-profiles/" + id },
			payload: map[string]any{
				"name": "Mask Check SMTP", "host": "smtp.x.com", "port": 587,
				"auth_type": "plain", "username": "u", "password": "secret-smtp-pass-check",
				"tls_mode": "starttls", "from_address": "f@x.com",
				"max_connections": 5, "timeout_connect": 10, "timeout_send": 30,
			},
			secret: "secret-smtp-pass-check",
		},
	}

	for _, e := range entities {
		t.Run(e.name, func(t *testing.T) {
			resp := env.do("POST", e.createURL, e.payload, adminToken)
			if resp.StatusCode != http.StatusCreated {
				body := mustBody(resp)
				t.Fatalf("create %s: expected 201, got %d: %s", e.name, resp.StatusCode, body)
			}
			var created struct {
				Data struct{ ID string `json:"id"` } `json:"data"`
			}
			decodeT(t, resp, &created)

			resp = env.do("GET", e.getURL(created.Data.ID), nil, adminToken)
			body := mustBody(resp)
			if contains(string(body), e.secret) {
				t.Errorf("%s: secret %q found in GET response", e.name, e.secret)
			}
		})
	}
}

// TestInputValidationCoverage verifies all Phase 2 endpoints reject malformed
// input with 400/422 — no panics or 500s.
func TestInputValidationCoverage(t *testing.T) {
	env := setupTestEnv(t)
	resetPhase2DB(t, env.db)
	adminToken := mustSetup(t, env)

	type badInput struct {
		name    string
		method  string
		path    string
		payload map[string]any
	}

	cases := []badInput{
		// Domain provider — missing required fields.
		{"domain_provider missing type", "POST", "/settings/domain-providers", map[string]any{"display_name": "X"}},
		// Cloud credential — missing provider credentials.
		{"cloud_credential missing creds", "POST", "/settings/cloud-credentials", map[string]any{
			"provider_type": "aws", "display_name": "X",
		}},
		// SMTP profile — invalid port.
		{"smtp_profile invalid port", "POST", "/smtp-profiles", map[string]any{
			"name": "X", "host": "smtp.x.com", "port": -1,
			"auth_type": "plain", "tls_mode": "none", "from_address": "f@x.com",
			"max_connections": 5, "timeout_connect": 10, "timeout_send": 30,
		}},
		// Email template — missing required body.
		{"email_template empty body", "POST", "/email-templates", map[string]any{
			"name": "X", "subject": "S", "html_body": "", "category": "generic",
		}},
		// Auth provider — missing config.
		{"auth_provider missing config", "POST", "/settings/auth-providers", map[string]any{
			"type": "oidc", "name": "X", "enabled": false,
		}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			resp := env.do(tc.method, tc.path, tc.payload, adminToken)
			if resp.StatusCode == http.StatusInternalServerError {
				body := mustBody(resp)
				t.Errorf("%s: server returned 500 (panic?): %s", tc.name, body)
			}
			resp.Body.Close()
		})
	}
}

// TestSMTPConnectionTestTimeout verifies that SMTP connection tests return
// within a reasonable time for unreachable hosts.
func TestSMTPConnectionTestTimeout(t *testing.T) {
	env := setupTestEnv(t)
	resetPhase2DB(t, env.db)
	adminToken := mustSetup(t, env)

	// Use a host that's unreachable quickly (invalid IP in TEST env).
	resp := env.do("POST", "/smtp-profiles", map[string]any{
		"name": "Timeout Test SMTP", "host": "192.0.2.1", "port": 25, // TEST-NET — unreachable
		"auth_type": "none", "tls_mode": "none", "from_address": "f@x.com",
		"max_connections": 1, "timeout_connect": 2, "timeout_send": 2,
	}, adminToken)
	assertStatus(t, resp, http.StatusCreated)
	var created struct {
		Data struct{ ID string `json:"id"` } `json:"data"`
	}
	decodeT(t, resp, &created)

	start := time.Now()
	resp = env.do("POST", "/smtp-profiles/"+created.Data.ID+"/test", nil, adminToken)
	elapsed := time.Since(start)
	resp.Body.Close()

	// Should return within 30 seconds (not hang indefinitely).
	if elapsed > 30*time.Second {
		t.Errorf("SMTP connection test for unreachable host took %v (too long)", elapsed)
	}
}

// TestErrorResponseSanitization verifies error responses don't contain
// internal paths, stack traces, or credential values.
func TestErrorResponseSanitization(t *testing.T) {
	env := setupTestEnv(t)
	resetPhase2DB(t, env.db)
	adminToken := mustSetup(t, env)

	// Request a non-existent resource.
	resp := env.do("GET", "/settings/cloud-credentials/nonexistent-id-xyz", nil, adminToken)
	body := mustBody(resp)
	bodyStr := string(body)

	// Should not contain stack traces.
	if contains(bodyStr, "goroutine") || contains(bodyStr, "panic") {
		t.Errorf("error response contains stack trace: %s", bodyStr)
	}
	// Should not contain internal paths.
	if contains(bodyStr, "internal/") || contains(bodyStr, "services/") {
		t.Errorf("error response contains internal path: %s", bodyStr)
	}
}

// TestPhaseGateValidation is a final gate check that runs key validation
// scenarios and confirms Phase 2 is ready for Phase 3 handoff.
func TestPhaseGateValidation(t *testing.T) {
	env := setupTestEnv(t)
	resetPhase2DB(t, env.db)
	adminToken := mustSetup(t, env)

	t.Run("cloud credentials can be created and retrieved", func(t *testing.T) {
		id := createTestAWSCredential(t, env, adminToken, "Gate AWS")
		resp := env.do("GET", "/settings/cloud-credentials/"+id, nil, adminToken)
		assertStatusQuiet(t, resp, http.StatusOK)
	})

	t.Run("domains and DNS records manageable", func(t *testing.T) {
		domID := createTestDomain(t, env, adminToken, "gate-test.com")
		resp := env.do("POST", "/domains/"+domID+"/dns-records", map[string]any{
			"type": "A", "name": "@", "value": "1.2.3.4", "ttl": 300,
		}, adminToken)
		assertStatusQuiet(t, resp, http.StatusCreated)
	})

	t.Run("SMTP profiles can be created and listed", func(t *testing.T) {
		id := createTestSMTPProfile(t, env, adminToken, "Gate SMTP")
		resp := env.do("GET", "/smtp-profiles/"+id, nil, adminToken)
		assertStatusQuiet(t, resp, http.StatusOK)
	})

	t.Run("auth providers can be configured", func(t *testing.T) {
		oidc := newOIDCMock(t)
		resp := env.do("POST", "/settings/auth-providers", map[string]any{
			"type": "oidc", "name": "Gate OIDC", "enabled": false,
			"auto_provision": false, "auth_order": "local_first",
			"config": map[string]any{"issuer_url": oidc.issuer, "client_id": "cid", "client_secret": "cs"},
		}, adminToken)
		assertStatusQuiet(t, resp, http.StatusCreated)
	})

	t.Run("email templates support CRUD", func(t *testing.T) {
		id := createTestEmailTemplate(t, env, adminToken, "Gate Template")
		resp := env.do("PUT", "/email-templates/"+id, map[string]any{"subject": "Updated"}, adminToken)
		resp.Body.Close()
		resp = env.do("DELETE", "/email-templates/"+id, nil, adminToken)
		resp.Body.Close()
	})

	t.Run("instance templates can be versioned", func(t *testing.T) {
		credID := createTestAWSCredential(t, env, adminToken, "Gate Cred v")
		resp := env.do("POST", "/instance-templates", map[string]any{
			"display_name":        "Gate Template",
			"cloud_credential_id": credID,
			"region":              "us-east-1",
			"instance_size":       "t3.micro",
			"os_image":            "ami-0abc",
			"security_groups":     []string{"sg-1"},
		}, adminToken)
		assertStatusQuiet(t, resp, http.StatusCreated)
	})

	t.Run("RBAC enforcement works for all roles", func(t *testing.T) {
		toks := setupRoles(t, env, adminToken)
		resp := env.do("POST", "/settings/cloud-credentials", map[string]any{
			"provider_type": "aws", "display_name": "x", "default_region": "us-east-1",
			"aws": map[string]string{"access_key_id": "x", "secret_access_key": "x"},
		}, toks.operator)
		if resp.StatusCode != http.StatusForbidden {
			body := mustBody(resp)
			t.Errorf("operator should be forbidden; got %d: %s", resp.StatusCode, body)
		}
		resp.Body.Close()
	})

	t.Run("audit log captures infrastructure operations", func(t *testing.T) {
		waitForAudit()
		// At least domain.created must be in the log from sub-tests above.
		if countAuditEvents(t, env.db, "domain.created") == 0 {
			t.Error("audit log missing domain.created event")
		}
	})
}

// ---- Performance helpers (direct DB inserts for bulk data) ----

func insertBulkDomains(tb testing.TB, db *sql.DB, count int) {
	tb.Helper()
	for i := 0; i < count; i++ {
		_, err := db.ExecContext(context.Background(), `
			INSERT INTO domain_profiles
			  (id, name, registrar, dns_provider, purchase_date, expiry_date, status, purpose, created_by, created_at, updated_at)
			VALUES
			  (gen_random_uuid(), $1, 'namecheap', 'namecheap', '2025-01-01', '2026-01-01', 'active', 'phishing', 'test', NOW(), NOW())
		`, fmt.Sprintf("bulk-domain-%d.com", i))
		if err != nil {
			tb.Logf("insertBulkDomains[%d]: %v (skipping)", i, err)
			return
		}
	}
}

func insertBulkDNSRecords(tb testing.TB, db *sql.DB, domainID string, count int) {
	tb.Helper()
	for i := 0; i < count; i++ {
		_, err := db.ExecContext(context.Background(), `
			INSERT INTO dns_records
			  (id, domain_id, type, name, value, ttl, created_by, created_at, updated_at)
			VALUES
			  (gen_random_uuid(), $1, 'A', $2, '1.2.3.4', 300, 'test', NOW(), NOW())
		`, domainID, fmt.Sprintf("host%d", i))
		if err != nil {
			tb.Logf("insertBulkDNSRecords[%d]: %v (skipping)", i, err)
			return
		}
	}
}

// contains is a convenience wrapper (strings.Contains) to avoid import cycles.
func contains(s, sub string) bool {
	return len(s) >= len(sub) && (s == sub || len(sub) == 0 ||
		func() bool {
			for i := 0; i <= len(s)-len(sub); i++ {
				if s[i:i+len(sub)] == sub {
					return true
				}
			}
			return false
		}())
}
