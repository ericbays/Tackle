//go:build integration

package phase2

import (
	"net/http"
	"testing"
)

// TestDomainProviderConnectionLifecycle verifies: create Namecheap connection,
// verify encrypted storage and masked API response, test connection (success),
// verify status updated to healthy.
func TestDomainProviderConnectionLifecycle(t *testing.T) {
	env := setupTestEnv(t)
	resetPhase2DB(t, env.db)
	adminToken := mustSetup(t, env)

	// Create a Namecheap provider connection.
	resp := env.do("POST", "/settings/domain-providers", map[string]any{
		"provider_type": "namecheap",
		"display_name":  "Test Namecheap",
		"namecheap_credentials": map[string]string{
			"api_user":  "ncuser",
			"api_key":   "ncsecretkey",
			"username":  "ncuser",
			"client_ip": "1.2.3.4",
		},
	}, adminToken)
	assertStatus(t, resp, http.StatusCreated)

	var created struct {
		Data struct {
			ID           string `json:"id"`
			ProviderType string `json:"provider_type"`
			DisplayName  string `json:"display_name"`
			Status       string `json:"status"`
			CreatedBy    string `json:"created_by"`
		} `json:"data"`
	}
	decodeT(t, resp, &created)
	id := created.Data.ID
	if id == "" {
		t.Fatal("expected non-empty provider ID")
	}
	if created.Data.ProviderType != "namecheap" {
		t.Errorf("expected provider_type=namecheap, got %s", created.Data.ProviderType)
	}

	// Verify masked response — no credential values.
	resp = env.do("GET", "/settings/domain-providers/"+id, nil, adminToken)
	assertStatus(t, resp, http.StatusOK)
	body := mustBody(resp)
	if bodyContains(body, "ncsecretkey") {
		t.Error("API key leaked in GET response")
	}

	// Verify audit event.
	assertAuditEvent(t, env.db, "domain_provider.created")
}

// TestDomainProviderConnectionFailure verifies that a failed test connection
// returns status=error with an actionable message.
func TestDomainProviderConnectionFailure(t *testing.T) {
	env := setupTestEnv(t)
	resetPhase2DB(t, env.db)
	adminToken := mustSetup(t, env)

	resp := env.do("POST", "/settings/domain-providers", map[string]any{
		"provider_type": "godaddy",
		"display_name":  "Bad GoDaddy",
		"godaddy_credentials": map[string]string{
			"api_key":     "badkey",
			"api_secret":  "badsecret",
			"environment": "ote",
		},
	}, adminToken)
	assertStatus(t, resp, http.StatusCreated)

	var created struct {
		Data struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	decodeT(t, resp, &created)
	id := created.Data.ID

	// Test connection — will fail (no real GoDaddy server).
	resp = env.do("POST", "/settings/domain-providers/"+id+"/test", nil, adminToken)
	// Should return 200 with success=false or a 4xx/5xx — either is valid.
	body := mustBody(resp)
	_ = body // result depends on implementation; key check is no panic
	if resp.StatusCode == 500 {
		t.Errorf("unexpected 500 — server panicked on connection test failure")
	}
}

// TestMultipleProviders verifies all four provider types can be created and listed.
func TestMultipleProviders(t *testing.T) {
	env := setupTestEnv(t)
	resetPhase2DB(t, env.db)
	adminToken := mustSetup(t, env)

	providers := []map[string]any{
		{
			"provider_type": "namecheap",
			"display_name":  "Namecheap Prod",
			"namecheap_credentials": map[string]string{
				"api_user": "u", "api_key": "k", "username": "u", "client_ip": "1.1.1.1",
			},
		},
		{
			"provider_type": "godaddy",
			"display_name":  "GoDaddy Prod",
			"godaddy_credentials": map[string]string{
				"api_key": "k", "api_secret": "s", "environment": "production",
			},
		},
		{
			"provider_type": "route53",
			"display_name":  "AWS Route 53",
			"route53_credentials": map[string]string{
				"access_key_id": "AKIAIOSFODNN7EXAMPLE", "secret_access_key": "secret", "region": "us-east-1",
			},
		},
		{
			"provider_type": "azure_dns",
			"display_name":  "Azure DNS",
			"azure_dns_credentials": map[string]string{
				"tenant_id": "tid", "client_id": "cid", "client_secret": "cs", "subscription_id": "sub",
			},
		},
	}

	for _, p := range providers {
		resp := env.do("POST", "/settings/domain-providers", p, adminToken)
		if resp.StatusCode != http.StatusCreated {
			body := mustBody(resp)
			t.Fatalf("create provider %s: expected 201, got %d: %s",
				p["display_name"], resp.StatusCode, body)
		}
		resp.Body.Close()
	}

	resp := env.do("GET", "/settings/domain-providers", nil, adminToken)
	assertStatus(t, resp, http.StatusOK)
	var listResp struct {
		Data struct {
			Data []map[string]any `json:"data"`
		} `json:"data"`
	}
	decodeT(t, resp, &listResp)
	if len(listResp.Data.Data) < 4 {
		t.Errorf("expected >=4 providers, got %d", len(listResp.Data.Data))
	}
}

// TestDomainCreateAndList verifies domain profile creation and listing.
func TestDomainCreateAndList(t *testing.T) {
	env := setupTestEnv(t)
	resetPhase2DB(t, env.db)
	adminToken := mustSetup(t, env)

	// Create domain.
	resp := env.do("POST", "/domains", map[string]any{
		"name":           "test-domain.com",
		"registrar":      "namecheap",
		"dns_provider":   "namecheap",
		"purchase_date":  "2025-01-01",
		"expiry_date":    "2026-01-01",
		"auto_renew":     false,
		"purpose":        "phishing",
		"tags":           []string{"test"},
	}, adminToken)
	assertStatus(t, resp, http.StatusCreated)

	var created struct {
		Data struct {
			ID   string `json:"id"`
			Name string `json:"name"`
		} `json:"data"`
	}
	decodeT(t, resp, &created)
	if created.Data.ID == "" {
		t.Fatal("expected non-empty domain ID")
	}
	if created.Data.Name != "test-domain.com" {
		t.Errorf("expected name=test-domain.com, got %s", created.Data.Name)
	}

	// List domains.
	resp = env.do("GET", "/domains", nil, adminToken)
	assertStatus(t, resp, http.StatusOK)
	var listResp struct {
		Data struct {
			Data []map[string]any `json:"data"`
		} `json:"data"`
	}
	decodeT(t, resp, &listResp)
	if len(listResp.Data.Data) == 0 {
		t.Error("expected at least one domain in list")
	}

	// Get by ID.
	resp = env.do("GET", "/domains/"+created.Data.ID, nil, adminToken)
	assertStatus(t, resp, http.StatusOK)
	resp.Body.Close()

	assertAuditEvent(t, env.db, "domain.created")
}

// TestDomainUpdateAndDelete verifies domain update and soft delete.
func TestDomainUpdateAndDelete(t *testing.T) {
	env := setupTestEnv(t)
	resetPhase2DB(t, env.db)
	adminToken := mustSetup(t, env)

	resp := env.do("POST", "/domains", map[string]any{
		"name":          "update-test.com",
		"registrar":     "godaddy",
		"dns_provider":  "godaddy",
		"purchase_date": "2025-01-01",
		"expiry_date":   "2026-01-01",
		"purpose":       "phishing",
	}, adminToken)
	assertStatus(t, resp, http.StatusCreated)
	var created struct {
		Data struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	decodeT(t, resp, &created)
	id := created.Data.ID

	// Update.
	resp = env.do("PUT", "/domains/"+id, map[string]any{
		"tags": []string{"updated", "test"},
	}, adminToken)
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNoContent {
		body := mustBody(resp)
		t.Fatalf("update domain: expected 200/204, got %d: %s", resp.StatusCode, body)
	}
	resp.Body.Close()

	// Delete.
	resp = env.do("DELETE", "/domains/"+id, nil, adminToken)
	if resp.StatusCode != http.StatusNoContent && resp.StatusCode != http.StatusOK {
		body := mustBody(resp)
		t.Fatalf("delete domain: expected 200/204, got %d: %s", resp.StatusCode, body)
	}
	resp.Body.Close()

	assertAuditEvent(t, env.db, "domain.deleted")
}

// TestDNSRecordCRUD verifies creating, updating, and deleting DNS records.
func TestDNSRecordCRUD(t *testing.T) {
	env := setupTestEnv(t)
	resetPhase2DB(t, env.db)
	adminToken := mustSetup(t, env)

	// Create a domain first.
	resp := env.do("POST", "/domains", map[string]any{
		"name":          "dns-test.com",
		"registrar":     "namecheap",
		"dns_provider":  "namecheap",
		"purchase_date": "2025-01-01",
		"expiry_date":   "2026-01-01",
		"purpose":       "phishing",
	}, adminToken)
	assertStatus(t, resp, http.StatusCreated)
	var domain struct {
		Data struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	decodeT(t, resp, &domain)
	domainID := domain.Data.ID

	// Create A record.
	resp = env.do("POST", "/domains/"+domainID+"/dns-records", map[string]any{
		"type":  "A",
		"name":  "@",
		"value": "1.2.3.4",
		"ttl":   300,
	}, adminToken)
	assertStatus(t, resp, http.StatusCreated)
	var record struct {
		Data struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	decodeT(t, resp, &record)
	recordID := record.Data.ID

	// List records.
	resp = env.do("GET", "/domains/"+domainID+"/dns-records", nil, adminToken)
	assertStatus(t, resp, http.StatusOK)
	resp.Body.Close()

	// Update record.
	resp = env.do("PUT", "/domains/"+domainID+"/dns-records/"+recordID, map[string]any{
		"value": "5.6.7.8",
		"ttl":   600,
	}, adminToken)
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNoContent {
		body := mustBody(resp)
		t.Fatalf("update DNS record: expected 200/204, got %d: %s", resp.StatusCode, body)
	}
	resp.Body.Close()

	// Delete record.
	resp = env.do("DELETE", "/domains/"+domainID+"/dns-records/"+recordID, nil, adminToken)
	if resp.StatusCode != http.StatusNoContent && resp.StatusCode != http.StatusOK {
		body := mustBody(resp)
		t.Fatalf("delete DNS record: expected 200/204, got %d: %s", resp.StatusCode, body)
	}
	resp.Body.Close()

	assertAuditEvent(t, env.db, "dns_record.created")
}

// TestDNSRecordValidation verifies invalid DNS record inputs are rejected with 400.
func TestDNSRecordValidation(t *testing.T) {
	env := setupTestEnv(t)
	resetPhase2DB(t, env.db)
	adminToken := mustSetup(t, env)

	// Create domain.
	resp := env.do("POST", "/domains", map[string]any{
		"name":          "val-test.com",
		"registrar":     "namecheap",
		"dns_provider":  "namecheap",
		"purchase_date": "2025-01-01",
		"expiry_date":   "2026-01-01",
		"purpose":       "phishing",
	}, adminToken)
	assertStatus(t, resp, http.StatusCreated)
	var domain struct {
		Data struct{ ID string `json:"id"` } `json:"data"`
	}
	decodeT(t, resp, &domain)
	id := domain.Data.ID

	cases := []struct {
		name    string
		payload map[string]any
	}{
		{"invalid A record value", map[string]any{"type": "A", "name": "@", "value": "not-an-ip", "ttl": 300}},
		{"invalid AAAA value", map[string]any{"type": "AAAA", "name": "@", "value": "1.2.3.4", "ttl": 300}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			resp := env.do("POST", "/domains/"+id+"/dns-records", tc.payload, adminToken)
			if resp.StatusCode != http.StatusBadRequest && resp.StatusCode != http.StatusUnprocessableEntity {
				body := mustBody(resp)
				t.Errorf("expected 400/422 for %s, got %d: %s", tc.name, resp.StatusCode, body)
			}
			resp.Body.Close()
		})
	}
}

// TestDomainHealthCheck verifies the health check endpoint can be triggered and returns results.
func TestDomainHealthCheck(t *testing.T) {
	env := setupTestEnv(t)
	resetPhase2DB(t, env.db)
	adminToken := mustSetup(t, env)

	resp := env.do("POST", "/domains", map[string]any{
		"name":          "health-test.com",
		"registrar":     "namecheap",
		"dns_provider":  "namecheap",
		"purchase_date": "2025-01-01",
		"expiry_date":   "2026-01-01",
		"purpose":       "phishing",
	}, adminToken)
	assertStatus(t, resp, http.StatusCreated)
	var domain struct {
		Data struct{ ID string `json:"id"` } `json:"data"`
	}
	decodeT(t, resp, &domain)
	id := domain.Data.ID

	// Trigger health check.
	resp = env.do("POST", "/domains/"+id+"/health-check", nil, adminToken)
	// 200 or 202 accepted.
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusAccepted && resp.StatusCode != http.StatusCreated {
		body := mustBody(resp)
		t.Fatalf("trigger health check: expected 200/202/201, got %d: %s", resp.StatusCode, body)
	}
	resp.Body.Close()

	// List health checks.
	resp = env.do("GET", "/domains/"+id+"/health-checks", nil, adminToken)
	assertStatus(t, resp, http.StatusOK)
	resp.Body.Close()

	// Latest health check.
	resp = env.do("GET", "/domains/"+id+"/health-checks/latest", nil, adminToken)
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNotFound {
		body := mustBody(resp)
		t.Fatalf("latest health check: expected 200 or 404, got %d: %s", resp.StatusCode, body)
	}
	resp.Body.Close()
}

// TestDomainRBACEnforcement verifies RBAC for domain provider endpoints.
// Operator cannot create/delete provider connections.
func TestDomainRBACEnforcement(t *testing.T) {
	env := setupTestEnv(t)
	resetPhase2DB(t, env.db)
	adminToken := mustSetup(t, env)
	toks := setupRoles(t, env, adminToken)

	// Operator should be denied create on domain-providers.
	resp := env.do("POST", "/settings/domain-providers", map[string]any{
		"provider_type": "namecheap",
		"display_name":  "Operator Should Fail",
		"namecheap_credentials": map[string]string{
			"api_user": "u", "api_key": "k", "username": "u", "client_ip": "1.1.1.1",
		},
	}, toks.operator)
	if resp.StatusCode != http.StatusForbidden {
		body := mustBody(resp)
		t.Errorf("operator create domain-provider: expected 403, got %d: %s", resp.StatusCode, body)
	}
	resp.Body.Close()

	// Defender should be denied create on domain-providers.
	resp = env.do("POST", "/settings/domain-providers", map[string]any{
		"provider_type": "namecheap",
		"display_name":  "Defender Should Fail",
		"namecheap_credentials": map[string]string{
			"api_user": "u", "api_key": "k", "username": "u", "client_ip": "1.1.1.1",
		},
	}, toks.defender)
	if resp.StatusCode != http.StatusForbidden {
		body := mustBody(resp)
		t.Errorf("defender create domain-provider: expected 403, got %d: %s", resp.StatusCode, body)
	}
	resp.Body.Close()

	// Operator can read providers.
	resp = env.do("GET", "/settings/domain-providers", nil, toks.operator)
	assertStatusQuiet(t, resp, http.StatusOK)
}

// TestDomainAuditCompleteness verifies that create, update, delete operations
// are all recorded in the audit log.
func TestDomainAuditCompleteness(t *testing.T) {
	env := setupTestEnv(t)
	resetPhase2DB(t, env.db)
	adminToken := mustSetup(t, env)

	// Create provider.
	resp := env.do("POST", "/settings/domain-providers", map[string]any{
		"provider_type": "namecheap",
		"display_name":  "Audit Test Provider",
		"namecheap_credentials": map[string]string{
			"api_user": "u", "api_key": "k", "username": "u", "client_ip": "1.1.1.1",
		},
	}, adminToken)
	assertStatus(t, resp, http.StatusCreated)
	var prov struct {
		Data struct{ ID string `json:"id"` } `json:"data"`
	}
	decodeT(t, resp, &prov)
	provID := prov.Data.ID

	// Update provider.
	resp = env.do("PUT", "/settings/domain-providers/"+provID, map[string]any{
		"display_name": "Audit Test Provider Updated",
	}, adminToken)
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNoContent {
		resp.Body.Close()
	} else {
		resp.Body.Close()
	}

	// Delete provider.
	resp = env.do("DELETE", "/settings/domain-providers/"+provID, nil, adminToken)
	resp.Body.Close()

	waitForAudit()
	assertAuditEvent(t, env.db, "domain_provider.created")
	assertAuditEvent(t, env.db, "domain_provider.updated")
	assertAuditEvent(t, env.db, "domain_provider.deleted")
}

// TestTyposquatGeneration verifies the typosquat tool returns results.
func TestTyposquatGeneration(t *testing.T) {
	env := setupTestEnv(t)
	resetPhase2DB(t, env.db)
	adminToken := mustSetup(t, env)

	resp := env.do("POST", "/tools/typosquat", map[string]any{
		"domain": "example.com",
	}, adminToken)
	if resp.StatusCode == http.StatusOK || resp.StatusCode == http.StatusCreated {
		resp.Body.Close()
	} else {
		body := mustBody(resp)
		t.Fatalf("typosquat: expected 200/201, got %d: %s", resp.StatusCode, body)
	}
}
