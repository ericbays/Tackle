//go:build integration

package phase2

import (
	"net/http"
	"testing"
)

// TestDomainDNSEmailAuthFullPipeline runs the complete domain + DNS + email auth pipeline:
// provider connection → domain creation → DNS records → SPF + DKIM + DMARC → validate.
func TestDomainDNSEmailAuthFullPipeline(t *testing.T) {
	env := setupTestEnv(t)
	resetPhase2DB(t, env.db)
	adminToken := mustSetup(t, env)

	// 1. Create domain provider connection.
	resp := env.do("POST", "/settings/domain-providers", map[string]any{
		"provider_type": "namecheap",
		"display_name":  "Pipeline Namecheap",
		"namecheap_credentials": map[string]string{
			"api_user": "u", "api_key": "k", "username": "u", "client_ip": "1.1.1.1",
		},
	}, adminToken)
	assertStatus(t, resp, http.StatusCreated)
	var prov struct {
		Data struct{ ID string `json:"id"` } `json:"data"`
	}
	decodeT(t, resp, &prov)

	// 2. Create domain profile.
	resp = env.do("POST", "/domains", map[string]any{
		"name":                 "pipeline-test.com",
		"registrar":            "namecheap",
		"dns_provider":         "namecheap",
		"domain_provider_id":   prov.Data.ID,
		"purchase_date":        "2025-01-01",
		"expiry_date":          "2026-01-01",
		"purpose":              "phishing",
	}, adminToken)
	assertStatus(t, resp, http.StatusCreated)
	var domain struct {
		Data struct{ ID string `json:"id"` } `json:"data"`
	}
	decodeT(t, resp, &domain)
	domainID := domain.Data.ID

	// 3. Create A record pointing to endpoint IP.
	resp = env.do("POST", "/domains/"+domainID+"/dns-records", map[string]any{
		"type":  "A",
		"name":  "@",
		"value": "10.0.0.1",
		"ttl":   300,
	}, adminToken)
	assertStatus(t, resp, http.StatusCreated)
	resp.Body.Close()

	// 4. Configure SPF.
	resp = env.do("POST", "/domains/"+domainID+"/email-auth/spf", map[string]any{
		"mechanisms": []string{"ip4:10.0.0.1"},
		"qualifier":  "~all",
	}, adminToken)
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		body := mustBody(resp)
		t.Fatalf("SPF: got %d: %s", resp.StatusCode, body)
	}
	resp.Body.Close()

	// 5. Generate DKIM.
	resp = env.do("POST", "/domains/"+domainID+"/email-auth/dkim", map[string]any{
		"selector":  "mail",
		"algorithm": "rsa",
		"key_bits":  2048,
	}, adminToken)
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		body := mustBody(resp)
		t.Fatalf("DKIM: got %d: %s", resp.StatusCode, body)
	}
	resp.Body.Close()

	// 6. Configure DMARC.
	resp = env.do("POST", "/domains/"+domainID+"/email-auth/dmarc", map[string]any{
		"policy": "none",
		"pct":    100,
	}, adminToken)
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		body := mustBody(resp)
		t.Fatalf("DMARC: got %d: %s", resp.StatusCode, body)
	}
	resp.Body.Close()

	// 7. Check email auth status.
	resp = env.do("GET", "/domains/"+domainID+"/email-auth", nil, adminToken)
	assertStatus(t, resp, http.StatusOK)
	resp.Body.Close()

	// 8. Verify audit log has all pipeline steps.
	waitForAudit()
	for _, action := range []string{
		"domain_provider.created",
		"domain.created",
		"dns_record.created",
		"email_auth.spf.configured",
		"email_auth.dkim.generated",
		"email_auth.dmarc.configured",
	} {
		if countAuditEvents(t, env.db, action) == 0 {
			t.Errorf("missing audit event: %s", action)
		}
	}
}

// TestSMTPEmailTemplatePipeline runs the SMTP + email template + preview pipeline.
func TestSMTPEmailTemplatePipeline(t *testing.T) {
	env := setupTestEnv(t)
	resetPhase2DB(t, env.db)
	adminToken := mustSetup(t, env)

	// 1. Create SMTP profile.
	smtpID := createTestSMTPProfile(t, env, adminToken, "Pipeline SMTP")

	// 2. Create email template with variables.
	resp := env.do("POST", "/email-templates", map[string]any{
		"name":      "Pipeline Template",
		"subject":   "Security Notice for {{.FirstName}}",
		"html_body": "<html><body><p>Dear {{.FirstName}} {{.LastName}},</p><p><a href='{{.TrackingURL}}'>Click here</a></p></body></html>",
		"text_body": "Dear {{.FirstName}}, visit {{.TrackingURL}}",
		"category":  "credential_harvest",
	}, adminToken)
	assertStatus(t, resp, http.StatusCreated)
	var tmpl struct {
		Data struct{ ID string `json:"id"` } `json:"data"`
	}
	decodeT(t, resp, &tmpl)
	tmplID := tmpl.Data.ID

	// 3. Preview template.
	resp = env.do("POST", "/email-templates/"+tmplID+"/preview", map[string]any{
		"variables": map[string]string{
			"FirstName":   "Bob",
			"LastName":    "Builder",
			"TrackingURL": "https://t.example.com/track/x",
		},
	}, adminToken)
	if resp.StatusCode != http.StatusOK {
		body := mustBody(resp)
		t.Fatalf("preview: got %d: %s", resp.StatusCode, body)
	}
	resp.Body.Close()

	// 4. Verify SMTP profile and template are independently retrievable.
	resp = env.do("GET", "/smtp-profiles/"+smtpID, nil, adminToken)
	assertStatus(t, resp, http.StatusOK)
	resp.Body.Close()

	resp = env.do("GET", "/email-templates/"+tmplID, nil, adminToken)
	assertStatus(t, resp, http.StatusOK)
	resp.Body.Close()
}

// TestCloudCredentialInstanceTemplatePipeline runs the cloud credential +
// instance template lifecycle pipeline.
func TestCloudCredentialInstanceTemplatePipeline(t *testing.T) {
	env := setupTestEnv(t)
	resetPhase2DB(t, env.db)
	adminToken := mustSetup(t, env)

	// 1. Create AWS credential.
	credID := createTestAWSCredential(t, env, adminToken, "Pipeline AWS Cred")

	// 2. Verify credential is retrievable (no secret).
	resp := env.do("GET", "/settings/cloud-credentials/"+credID, nil, adminToken)
	assertStatus(t, resp, http.StatusOK)
	body := mustBody(resp)
	if bodyContains(body, "wJalrXUtnFEMI") {
		t.Error("secret_access_key leaked in response")
	}

	// 3. Create instance template.
	resp = env.do("POST", "/instance-templates", map[string]any{
		"display_name":        "Pipeline Template",
		"cloud_credential_id": credID,
		"region":              "us-east-1",
		"instance_size":       "t3.micro",
		"os_image":            "ami-0abcdef1234567890",
		"security_groups":     []string{"sg-12345"},
		"tags":                map[string]string{"pipeline": "test"},
	}, adminToken)
	assertStatus(t, resp, http.StatusCreated)
	var tmpl struct {
		Data struct {
			ID             string `json:"id"`
			CurrentVersion int    `json:"current_version"`
		} `json:"data"`
	}
	decodeT(t, resp, &tmpl)

	// 4. Version the template.
	resp = env.do("PUT", "/instance-templates/"+tmpl.Data.ID, map[string]any{
		"display_name":        "Pipeline Template",
		"cloud_credential_id": credID,
		"region":              "us-west-2",
		"instance_size":       "t3.small",
		"os_image":            "ami-0abcdef1234567890",
		"security_groups":     []string{"sg-12345"},
	}, adminToken)
	resp.Body.Close()

	// 5. Verify version history.
	resp = env.do("GET", "/instance-templates/"+tmpl.Data.ID+"/versions", nil, adminToken)
	assertStatus(t, resp, http.StatusOK)
	var versions struct {
		Data []map[string]any `json:"data"`
	}
	decodeT(t, resp, &versions)
	if len(versions.Data) < 2 {
		t.Errorf("expected >=2 versions, got %d", len(versions.Data))
	}

	waitForAudit()
	assertAuditEvent(t, env.db, "cloud_credential.created")
	assertAuditEvent(t, env.db, "instance_template.created")
	assertAuditEvent(t, env.db, "instance_template.updated")
}

// TestFullRBACMatrix verifies all Phase 2 endpoints enforce RBAC correctly
// for each role (Admin, Engineer, Operator, Defender).
func TestFullRBACMatrix(t *testing.T) {
	env := setupTestEnv(t)
	resetPhase2DB(t, env.db)
	adminToken := mustSetup(t, env)
	toks := setupRoles(t, env, adminToken)

	type check struct {
		method string
		path   string
		token  string
		want   int // expected status; 0 = any 2xx
	}

	checks := []check{
		// Domain providers — create requires domains:create
		{"POST", "/settings/domain-providers", toks.operator, http.StatusForbidden},
		{"POST", "/settings/domain-providers", toks.defender, http.StatusForbidden},
		{"GET", "/settings/domain-providers", toks.operator, http.StatusOK},
		{"GET", "/settings/domain-providers", toks.defender, http.StatusOK},

		// Cloud credentials — create requires infrastructure:create
		{"POST", "/settings/cloud-credentials", toks.operator, http.StatusForbidden},
		{"POST", "/settings/cloud-credentials", toks.defender, http.StatusForbidden},
		{"GET", "/settings/cloud-credentials", toks.operator, http.StatusOK},
		{"GET", "/settings/cloud-credentials", toks.defender, http.StatusOK},

		// SMTP profiles — create requires infrastructure:create
		{"POST", "/smtp-profiles", toks.operator, http.StatusForbidden},
		{"POST", "/smtp-profiles", toks.defender, http.StatusForbidden},
		{"GET", "/smtp-profiles", toks.operator, http.StatusOK},
		{"GET", "/smtp-profiles", toks.defender, http.StatusOK},

		// Auth providers — create requires settings:update
		{"POST", "/settings/auth-providers", toks.operator, http.StatusForbidden},
		{"POST", "/settings/auth-providers", toks.defender, http.StatusForbidden},
		{"GET", "/settings/auth-providers", toks.operator, http.StatusForbidden},
		{"GET", "/settings/auth-providers", toks.engineer, http.StatusOK},

		// Email templates — read requires campaigns:read
		{"GET", "/email-templates", toks.operator, http.StatusOK},
		{"GET", "/email-templates", toks.defender, http.StatusOK},
	}

	for _, c := range checks {
		t.Run(c.method+" "+c.path+"(role)", func(t *testing.T) {
			resp := env.do(c.method, c.path, nil, c.token)
			body := mustBody(resp)
			if resp.StatusCode != c.want {
				t.Errorf("expected %d, got %d: %s", c.want, resp.StatusCode, body)
			}
		})
	}
}

// TestAuditLogCompletenessSweep performs one create+update+delete in each
// Phase 2 subsystem and verifies the audit log captures all events.
func TestAuditLogCompletenessSweep(t *testing.T) {
	env := setupTestEnv(t)
	resetPhase2DB(t, env.db)
	adminToken := mustSetup(t, env)

	// Domain provider.
	resp := env.do("POST", "/settings/domain-providers", map[string]any{
		"provider_type": "namecheap",
		"display_name":  "Sweep Provider",
		"namecheap_credentials": map[string]string{
			"api_user": "u", "api_key": "k", "username": "u", "client_ip": "1.1.1.1",
		},
	}, adminToken)
	assertStatus(t, resp, http.StatusCreated)
	var dp struct {
		Data struct{ ID string `json:"id"` } `json:"data"`
	}
	decodeT(t, resp, &dp)
	env.do("PUT", "/settings/domain-providers/"+dp.Data.ID, map[string]any{"display_name": "Updated"}, adminToken).Body.Close()
	env.do("DELETE", "/settings/domain-providers/"+dp.Data.ID, nil, adminToken).Body.Close()

	// Cloud credential.
	credID := createTestAWSCredential(t, env, adminToken, "Sweep Cred")
	env.do("PUT", "/settings/cloud-credentials/"+credID, map[string]any{"display_name": "Updated"}, adminToken).Body.Close()
	env.do("DELETE", "/settings/cloud-credentials/"+credID, nil, adminToken).Body.Close()

	// SMTP profile.
	smtpID := createTestSMTPProfile(t, env, adminToken, "Sweep SMTP")
	env.do("PUT", "/smtp-profiles/"+smtpID, map[string]any{"host": "smtp2.example.com"}, adminToken).Body.Close()
	env.do("DELETE", "/smtp-profiles/"+smtpID, nil, adminToken).Body.Close()

	// Email template.
	tmplID := createTestEmailTemplate(t, env, adminToken, "Sweep Template")
	env.do("PUT", "/email-templates/"+tmplID, map[string]any{"subject": "Updated"}, adminToken).Body.Close()
	env.do("DELETE", "/email-templates/"+tmplID, nil, adminToken).Body.Close()

	waitForAudit()

	required := []string{
		"domain_provider.created", "domain_provider.updated", "domain_provider.deleted",
		"cloud_credential.created", "cloud_credential.deleted",
		"smtp_profile.created", "smtp_profile.updated", "smtp_profile.deleted",
		"email_template.created", "email_template.updated", "email_template.deleted",
	}
	for _, action := range required {
		if countAuditEvents(t, env.db, action) == 0 {
			t.Errorf("missing audit event: %s", action)
		}
	}
}

// TestCredentialMaskingConsistency verifies no plaintext credentials appear
// in any API response across all credential-bearing entity types.
func TestCredentialMaskingConsistency(t *testing.T) {
	env := setupTestEnv(t)
	resetPhase2DB(t, env.db)
	adminToken := mustSetup(t, env)

	secretValues := []string{
		"SUPERSECRETVALUE99",
		"my-very-secret-api-key",
		"azure-client-secret-xyz",
		"ldap-bind-password-xyz",
	}

	// Domain provider.
	resp := env.do("POST", "/settings/domain-providers", map[string]any{
		"provider_type": "namecheap",
		"display_name":  "Mask Test NC",
		"namecheap_credentials": map[string]string{
			"api_user": "u", "api_key": secretValues[0], "username": "u", "client_ip": "1.1.1.1",
		},
	}, adminToken)
	assertStatus(t, resp, http.StatusCreated)
	var dp struct {
		Data struct{ ID string `json:"id"` } `json:"data"`
	}
	decodeT(t, resp, &dp)
	resp = env.do("GET", "/settings/domain-providers/"+dp.Data.ID, nil, adminToken)
	body := mustBody(resp)
	if bodyContains(body, secretValues[0]) {
		t.Errorf("domain provider: %q leaked in GET", secretValues[0])
	}

	// Cloud credential.
	resp = env.do("POST", "/settings/cloud-credentials", map[string]any{
		"provider_type": "godaddy",
		"display_name":  "Mask Test GD",
		"godaddy_credentials": map[string]string{
			"api_key": "gdk", "api_secret": secretValues[1], "environment": "ote",
		},
	}, adminToken)
	// godaddy credentials may map to domain provider, not cloud credential; adjust if needed.
	// Try AWS instead.
	resp = env.do("POST", "/settings/cloud-credentials", map[string]any{
		"provider_type":  "aws",
		"display_name":   "Mask Test AWS",
		"default_region": "us-east-1",
		"aws": map[string]string{
			"access_key_id": "AKID", "secret_access_key": secretValues[1],
		},
	}, adminToken)
	assertStatus(t, resp, http.StatusCreated)
	var cc struct {
		Data struct{ ID string `json:"id"` } `json:"data"`
	}
	decodeT(t, resp, &cc)
	resp = env.do("GET", "/settings/cloud-credentials/"+cc.Data.ID, nil, adminToken)
	body = mustBody(resp)
	if bodyContains(body, secretValues[1]) {
		t.Errorf("cloud credential: %q leaked in GET", secretValues[1])
	}

	// SMTP profile.
	resp = env.do("POST", "/smtp-profiles", map[string]any{
		"name": "Mask Test SMTP", "host": "smtp.example.com", "port": 587,
		"auth_type": "plain", "username": "user", "password": "masktest-smtp-pass-xyz",
		"tls_mode": "starttls", "from_address": "f@example.com",
		"max_connections": 5, "timeout_connect": 10, "timeout_send": 30,
	}, adminToken)
	assertStatus(t, resp, http.StatusCreated)
	var sp struct {
		Data struct{ ID string `json:"id"` } `json:"data"`
	}
	decodeT(t, resp, &sp)
	resp = env.do("GET", "/smtp-profiles/"+sp.Data.ID, nil, adminToken)
	body = mustBody(resp)
	if bodyContains(body, "masktest-smtp-pass-xyz") {
		t.Errorf("smtp profile: password leaked in GET")
	}

	// Auth provider.
	oidc := newOIDCMock(t)
	resp = env.do("POST", "/settings/auth-providers", map[string]any{
		"type": "oidc", "name": "Mask Test OIDC", "enabled": false,
		"auto_provision": false, "auth_order": "local_first",
		"config": map[string]any{
			"issuer_url": oidc.issuer, "client_id": "cid",
			"client_secret": "masktest-oidc-secret-xyz",
		},
	}, adminToken)
	assertStatus(t, resp, http.StatusCreated)
	var ap struct {
		Data struct{ ID string `json:"id"` } `json:"data"`
	}
	decodeT(t, resp, &ap)
	resp = env.do("GET", "/settings/auth-providers/"+ap.Data.ID, nil, adminToken)
	body = mustBody(resp)
	if bodyContains(body, "masktest-oidc-secret-xyz") {
		t.Errorf("auth provider: client_secret leaked in GET")
	}
}
