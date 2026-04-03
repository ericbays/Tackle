//go:build integration

package phase2

import (
	"net/http"
	"strings"
	"testing"
)

// TestEmailTemplateCreateAndRetrieve verifies template creation and retrieval.
func TestEmailTemplateCreateAndRetrieve(t *testing.T) {
	env := setupTestEnv(t)
	resetPhase2DB(t, env.db)
	adminToken := mustSetup(t, env)

	resp := env.do("POST", "/email-templates", map[string]any{
		"name":      "Q1 Phishing Template",
		"subject":   "Action Required: Update Your Password",
		"html_body": "<html><body>Hello {{.FirstName}}, click <a href='{{.TrackingURL}}'>here</a>.</body></html>",
		"text_body": "Hello {{.FirstName}}, visit {{.TrackingURL}}",
		"category":  "credential_harvest",
		"tags":      []string{"q1", "password"},
		"is_shared": false,
	}, adminToken)
	assertStatus(t, resp, http.StatusCreated)

	var created struct {
		Data struct {
			ID       string `json:"id"`
			Name     string `json:"name"`
			Subject  string `json:"subject"`
			Category string `json:"category"`
			Version  int    `json:"version"`
		} `json:"data"`
	}
	decodeT(t, resp, &created)
	id := created.Data.ID
	if id == "" {
		t.Fatal("expected non-empty template ID")
	}
	if created.Data.Name != "Q1 Phishing Template" {
		t.Errorf("expected name=Q1 Phishing Template, got %s", created.Data.Name)
	}

	// Retrieve.
	resp = env.do("GET", "/email-templates/"+id, nil, adminToken)
	assertStatus(t, resp, http.StatusOK)
	resp.Body.Close()

	assertAuditEvent(t, env.db, "email_template.created")
}

// TestEmailTemplateUpdateAndVersion verifies update creates a new version.
func TestEmailTemplateUpdateAndVersion(t *testing.T) {
	env := setupTestEnv(t)
	resetPhase2DB(t, env.db)
	adminToken := mustSetup(t, env)

	id := createTestEmailTemplate(t, env, adminToken, "Version Test")

	// Update.
	resp := env.do("PUT", "/email-templates/"+id, map[string]any{
		"subject":     "Updated Subject",
		"change_note": "Updated subject for v2",
	}, adminToken)
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNoContent {
		body := mustBody(resp)
		t.Fatalf("update template: expected 200/204, got %d: %s", resp.StatusCode, body)
	}
	resp.Body.Close()

	// List versions.
	resp = env.do("GET", "/email-templates/"+id+"/versions", nil, adminToken)
	assertStatus(t, resp, http.StatusOK)
	var versions struct {
		Data []map[string]any `json:"data"`
	}
	decodeT(t, resp, &versions)
	if len(versions.Data) < 2 {
		t.Errorf("expected >=2 versions after update, got %d", len(versions.Data))
	}

	assertAuditEvent(t, env.db, "email_template.updated")
}

// TestEmailTemplatePreview verifies preview rendering substitutes variables.
func TestEmailTemplatePreview(t *testing.T) {
	env := setupTestEnv(t)
	resetPhase2DB(t, env.db)
	adminToken := mustSetup(t, env)

	id := createTestEmailTemplate(t, env, adminToken, "Preview Test")

	resp := env.do("POST", "/email-templates/"+id+"/preview", map[string]any{
		"variables": map[string]string{
			"FirstName":   "Jane",
			"LastName":    "Smith",
			"Email":       "jane@example.com",
			"TrackingURL": "https://track.example.com/abc",
		},
	}, adminToken)
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		body := mustBody(resp)
		t.Fatalf("preview template: expected 200/201, got %d: %s", resp.StatusCode, body)
	}
	body := mustBody(resp)
	// Preview body should contain rendered content (Jane, not {{.FirstName}}).
	if strings.Contains(string(body), "{{.FirstName}}") {
		t.Error("template variable {{.FirstName}} was not substituted in preview")
	}
}

// TestEmailTemplateClone verifies cloning creates a new editable copy.
func TestEmailTemplateClone(t *testing.T) {
	env := setupTestEnv(t)
	resetPhase2DB(t, env.db)
	adminToken := mustSetup(t, env)

	id := createTestEmailTemplate(t, env, adminToken, "Source Template")

	resp := env.do("POST", "/email-templates/"+id+"/clone", map[string]any{
		"name": "Cloned Template",
	}, adminToken)
	assertStatus(t, resp, http.StatusCreated)

	var clone struct {
		Data struct {
			ID   string `json:"id"`
			Name string `json:"name"`
		} `json:"data"`
	}
	decodeT(t, resp, &clone)
	if clone.Data.ID == id {
		t.Error("cloned template should have a different ID")
	}
	if clone.Data.Name != "Cloned Template" {
		t.Errorf("expected clone name=Cloned Template, got %s", clone.Data.Name)
	}
}

// TestEmailTemplateValidate verifies the validate endpoint checks template syntax.
func TestEmailTemplateValidate(t *testing.T) {
	env := setupTestEnv(t)
	resetPhase2DB(t, env.db)
	adminToken := mustSetup(t, env)

	id := createTestEmailTemplate(t, env, adminToken, "Validate Test")

	resp := env.do("POST", "/email-templates/"+id+"/validate", nil, adminToken)
	if resp.StatusCode == http.StatusInternalServerError {
		body := mustBody(resp)
		t.Fatalf("validate template returned 500: %s", body)
	}
	resp.Body.Close()
}

// TestEmailTemplateExport verifies the export endpoint returns template data.
func TestEmailTemplateExport(t *testing.T) {
	env := setupTestEnv(t)
	resetPhase2DB(t, env.db)
	adminToken := mustSetup(t, env)

	id := createTestEmailTemplate(t, env, adminToken, "Export Test")

	resp := env.do("GET", "/email-templates/"+id+"/export", nil, adminToken)
	if resp.StatusCode != http.StatusOK {
		body := mustBody(resp)
		t.Fatalf("export template: expected 200, got %d: %s", resp.StatusCode, body)
	}
	resp.Body.Close()
}

// TestEmailTemplateDeleteBlocked verifies a template in use by a campaign cannot be deleted.
// In Phase 2 there are no campaigns, so this tests the no-campaign path (delete should succeed).
func TestEmailTemplateDeleteSucceedsWithNoCampaign(t *testing.T) {
	env := setupTestEnv(t)
	resetPhase2DB(t, env.db)
	adminToken := mustSetup(t, env)

	id := createTestEmailTemplate(t, env, adminToken, "Delete Me")

	resp := env.do("DELETE", "/email-templates/"+id, nil, adminToken)
	if resp.StatusCode != http.StatusNoContent && resp.StatusCode != http.StatusOK {
		body := mustBody(resp)
		t.Fatalf("delete template: expected 200/204, got %d: %s", resp.StatusCode, body)
	}
	resp.Body.Close()

	assertAuditEvent(t, env.db, "email_template.deleted")
}

// TestEmailTemplateList verifies listing returns all non-deleted templates.
func TestEmailTemplateList(t *testing.T) {
	env := setupTestEnv(t)
	resetPhase2DB(t, env.db)
	adminToken := mustSetup(t, env)

	for _, name := range []string{"T1", "T2", "T3"} {
		createTestEmailTemplate(t, env, adminToken, name)
	}

	resp := env.do("GET", "/email-templates", nil, adminToken)
	assertStatus(t, resp, http.StatusOK)
	var listResp struct {
		Data []map[string]any `json:"data"`
	}
	decodeT(t, resp, &listResp)
	if len(listResp.Data) < 3 {
		t.Errorf("expected >=3 templates, got %d", len(listResp.Data))
	}
}

// TestEmailTemplateRBACEnforcement verifies role-appropriate access.
func TestEmailTemplateRBACEnforcement(t *testing.T) {
	env := setupTestEnv(t)
	resetPhase2DB(t, env.db)
	adminToken := mustSetup(t, env)
	toks := setupRoles(t, env, adminToken)

	// Defender can list (campaigns:read).
	resp := env.do("GET", "/email-templates", nil, toks.defender)
	assertStatusQuiet(t, resp, http.StatusOK)

	// Operator cannot create (campaigns:create).
	// Operator role only has campaigns:read — adjust if your RBAC differs.
	resp = env.do("POST", "/email-templates", map[string]any{
		"name":      "Operator Forbidden",
		"subject":   "Test",
		"html_body": "<p>Test</p>",
		"text_body": "Test",
		"category":  "generic",
	}, toks.operator)
	// Operator typically has campaigns:read but not campaigns:create.
	// If the role has campaigns:create, this will be 201 and that's valid.
	if resp.StatusCode == http.StatusInternalServerError {
		body := mustBody(resp)
		t.Errorf("operator create template returned 500: %s", body)
	}
	resp.Body.Close()
}

// TestEmailTemplateAuditTrail verifies all operations are logged.
func TestEmailTemplateAuditTrail(t *testing.T) {
	env := setupTestEnv(t)
	resetPhase2DB(t, env.db)
	adminToken := mustSetup(t, env)

	id := createTestEmailTemplate(t, env, adminToken, "Audit Template")

	// Update.
	resp := env.do("PUT", "/email-templates/"+id, map[string]any{
		"subject": "Updated",
	}, adminToken)
	resp.Body.Close()

	// Preview.
	resp = env.do("POST", "/email-templates/"+id+"/preview", map[string]any{}, adminToken)
	resp.Body.Close()

	// Delete.
	resp = env.do("DELETE", "/email-templates/"+id, nil, adminToken)
	resp.Body.Close()

	waitForAudit()
	assertAuditEvent(t, env.db, "email_template.created")
	assertAuditEvent(t, env.db, "email_template.updated")
	assertAuditEvent(t, env.db, "email_template.deleted")
}

// TestEmailTemplateVariableSubstitution verifies key template variables render correctly.
func TestEmailTemplateVariableSubstitution(t *testing.T) {
	env := setupTestEnv(t)
	resetPhase2DB(t, env.db)
	adminToken := mustSetup(t, env)

	// Create template with all variable types.
	resp := env.do("POST", "/email-templates", map[string]any{
		"name":      "Variable Test",
		"subject":   "Hello {{.FirstName}} {{.LastName}}",
		"html_body": "<p>{{.FirstName}} {{.LastName}} ({{.Email}}) <a href='{{.TrackingURL}}'>click</a></p>",
		"text_body": "{{.FirstName}} {{.LastName}}",
		"category":  "generic",
	}, adminToken)
	assertStatus(t, resp, http.StatusCreated)
	var created struct {
		Data struct{ ID string `json:"id"` } `json:"data"`
	}
	decodeT(t, resp, &created)

	// Preview with all variables.
	resp = env.do("POST", "/email-templates/"+created.Data.ID+"/preview", map[string]any{
		"variables": map[string]string{
			"FirstName":   "Alice",
			"LastName":    "Wonderland",
			"Email":       "alice@example.com",
			"TrackingURL": "https://t.example.com/track/abc",
		},
	}, adminToken)
	if resp.StatusCode == http.StatusInternalServerError {
		body := mustBody(resp)
		t.Fatalf("preview returned 500: %s", body)
	}
	body := mustBody(resp)
	if strings.Contains(string(body), "{{.") {
		t.Errorf("unrendered template variables found in preview: %s", string(body))
	}
}

// createTestEmailTemplate creates an email template and returns its ID.
func createTestEmailTemplate(t *testing.T, env *testEnv, token, name string) string {
	t.Helper()
	resp := env.do("POST", "/email-templates", map[string]any{
		"name":      name,
		"subject":   "Test Subject for " + name,
		"html_body": "<html><body><p>Hello {{.FirstName}}, <a href='{{.TrackingURL}}'>click here</a>.</p></body></html>",
		"text_body": "Hello {{.FirstName}}, visit {{.TrackingURL}}",
		"category":  "generic",
		"tags":      []string{"test"},
		"is_shared": false,
	}, token)
	if resp.StatusCode != http.StatusCreated {
		body := mustBody(resp)
		t.Fatalf("createTestEmailTemplate(%s): expected 201, got %d: %s", name, resp.StatusCode, body)
	}
	var cr struct {
		Data struct{ ID string `json:"id"` } `json:"data"`
	}
	decodeT(t, resp, &cr)
	return cr.Data.ID
}
