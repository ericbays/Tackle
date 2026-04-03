//go:build integration

package phase2

import (
	"net/http"
	"testing"
)

// TestCloudCredentialAWSLifecycle verifies creating, reading, updating, and
// deleting an AWS credential set. Verifies credentials are masked in responses.
func TestCloudCredentialAWSLifecycle(t *testing.T) {
	env := setupTestEnv(t)
	resetPhase2DB(t, env.db)
	adminToken := mustSetup(t, env)

	// Create AWS credentials.
	resp := env.do("POST", "/settings/cloud-credentials", map[string]any{
		"provider_type":  "aws",
		"display_name":   "Test AWS Creds",
		"default_region": "us-east-1",
		"aws": map[string]string{
			"access_key_id":     "AKIAIOSFODNN7EXAMPLE",
			"secret_access_key": "wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY",
		},
	}, adminToken)
	assertStatus(t, resp, http.StatusCreated)

	var created struct {
		Data struct {
			ID           string `json:"id"`
			ProviderType string `json:"provider_type"`
			DisplayName  string `json:"display_name"`
			Status       string `json:"status"`
		} `json:"data"`
	}
	decodeT(t, resp, &created)
	id := created.Data.ID
	if id == "" {
		t.Fatal("expected non-empty credential ID")
	}
	if created.Data.ProviderType != "aws" {
		t.Errorf("expected provider_type=aws, got %s", created.Data.ProviderType)
	}

	// GET — verify no plaintext secret.
	resp = env.do("GET", "/settings/cloud-credentials/"+id, nil, adminToken)
	assertStatus(t, resp, http.StatusOK)
	body := mustBody(resp)
	if bodyContains(body, "wJalrXUtnFEMI") {
		t.Error("secret_access_key leaked in GET response")
	}
	if bodyContains(body, "AKIAIOSFODNN7EXAMPLE") {
		t.Error("access_key_id leaked in GET response")
	}

	// Update display name.
	resp = env.do("PUT", "/settings/cloud-credentials/"+id, map[string]any{
		"display_name": "Test AWS Creds Updated",
	}, adminToken)
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNoContent {
		body := mustBody(resp)
		t.Fatalf("update credential: expected 200/204, got %d: %s", resp.StatusCode, body)
	}
	resp.Body.Close()

	// Delete.
	resp = env.do("DELETE", "/settings/cloud-credentials/"+id, nil, adminToken)
	if resp.StatusCode != http.StatusNoContent && resp.StatusCode != http.StatusOK {
		body := mustBody(resp)
		t.Fatalf("delete credential: expected 200/204, got %d: %s", resp.StatusCode, body)
	}
	resp.Body.Close()

	waitForAudit()
	assertAuditEvent(t, env.db, "cloud_credential.created")
	assertAuditEvent(t, env.db, "cloud_credential.deleted")
}

// TestCloudCredentialAzureLifecycle verifies Azure credential management.
func TestCloudCredentialAzureLifecycle(t *testing.T) {
	env := setupTestEnv(t)
	resetPhase2DB(t, env.db)
	adminToken := mustSetup(t, env)

	resp := env.do("POST", "/settings/cloud-credentials", map[string]any{
		"provider_type":  "azure",
		"display_name":   "Test Azure Creds",
		"default_region": "eastus",
		"azure": map[string]string{
			"tenant_id":       "test-tenant-id",
			"client_id":       "test-client-id",
			"client_secret":   "supersecretazurecred",
			"subscription_id": "test-sub-id",
		},
	}, adminToken)
	assertStatus(t, resp, http.StatusCreated)

	var created struct {
		Data struct{ ID string `json:"id"` } `json:"data"`
	}
	decodeT(t, resp, &created)
	id := created.Data.ID

	// Verify credentials not in GET response.
	resp = env.do("GET", "/settings/cloud-credentials/"+id, nil, adminToken)
	assertStatus(t, resp, http.StatusOK)
	body := mustBody(resp)
	if bodyContains(body, "supersecretazurecred") {
		t.Error("Azure client_secret leaked in GET response")
	}

	assertAuditEvent(t, env.db, "cloud_credential.created")
}

// TestCloudCredentialTestConnection verifies the test connectivity endpoint.
func TestCloudCredentialTestConnection(t *testing.T) {
	env := setupTestEnv(t)
	resetPhase2DB(t, env.db)
	adminToken := mustSetup(t, env)

	resp := env.do("POST", "/settings/cloud-credentials", map[string]any{
		"provider_type":  "aws",
		"display_name":   "Test Conn AWS",
		"default_region": "us-west-2",
		"aws": map[string]string{
			"access_key_id":     "AKIAIOSFODNN7EXAMPLE",
			"secret_access_key": "wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY",
		},
	}, adminToken)
	assertStatus(t, resp, http.StatusCreated)
	var created struct {
		Data struct{ ID string `json:"id"` } `json:"data"`
	}
	decodeT(t, resp, &created)
	id := created.Data.ID

	// Test connection — will fail (no real AWS), but should not 500.
	resp = env.do("POST", "/settings/cloud-credentials/"+id+"/test", nil, adminToken)
	if resp.StatusCode == http.StatusInternalServerError {
		body := mustBody(resp)
		t.Fatalf("cloud credential test returned 500: %s", body)
	}
	resp.Body.Close()
}

// TestInstanceTemplateLifecycle verifies create, version, list, and delete.
func TestInstanceTemplateLifecycle(t *testing.T) {
	env := setupTestEnv(t)
	resetPhase2DB(t, env.db)
	adminToken := mustSetup(t, env)

	// Create AWS credential first.
	credID := createTestAWSCredential(t, env, adminToken, "Template Cred")

	// Create instance template.
	resp := env.do("POST", "/instance-templates", map[string]any{
		"display_name":        "Test Instance Template",
		"cloud_credential_id": credID,
		"region":              "us-east-1",
		"instance_size":       "t3.micro",
		"os_image":            "ami-0abcdef1234567890",
		"security_groups":     []string{"sg-12345"},
		"tags":                map[string]string{"env": "test"},
	}, adminToken)
	assertStatus(t, resp, http.StatusCreated)

	var created struct {
		Data struct {
			ID             string `json:"id"`
			CurrentVersion int    `json:"current_version"`
		} `json:"data"`
	}
	decodeT(t, resp, &created)
	id := created.Data.ID
	if id == "" {
		t.Fatal("expected non-empty template ID")
	}
	if created.Data.CurrentVersion != 1 {
		t.Errorf("expected version 1, got %d", created.Data.CurrentVersion)
	}

	// Update (creates version 2).
	resp = env.do("PUT", "/instance-templates/"+id, map[string]any{
		"display_name":        "Test Instance Template",
		"cloud_credential_id": credID,
		"region":              "us-west-2",
		"instance_size":       "t3.small",
		"os_image":            "ami-0abcdef1234567890",
		"security_groups":     []string{"sg-12345"},
		"tags":                map[string]string{"env": "test", "updated": "true"},
	}, adminToken)
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNoContent {
		body := mustBody(resp)
		t.Fatalf("update template: expected 200/204, got %d: %s", resp.StatusCode, body)
	}
	resp.Body.Close()

	// List versions.
	resp = env.do("GET", "/instance-templates/"+id+"/versions", nil, adminToken)
	assertStatus(t, resp, http.StatusOK)
	var versions struct {
		Data []map[string]any `json:"data"`
	}
	decodeT(t, resp, &versions)
	if len(versions.Data) < 2 {
		t.Errorf("expected >=2 versions, got %d", len(versions.Data))
	}

	// Delete.
	resp = env.do("DELETE", "/instance-templates/"+id, nil, adminToken)
	if resp.StatusCode != http.StatusNoContent && resp.StatusCode != http.StatusOK {
		body := mustBody(resp)
		t.Fatalf("delete template: expected 200/204, got %d: %s", resp.StatusCode, body)
	}
	resp.Body.Close()

	waitForAudit()
	assertAuditEvent(t, env.db, "instance_template.created")
	assertAuditEvent(t, env.db, "instance_template.updated")
	assertAuditEvent(t, env.db, "instance_template.deleted")
}

// TestInstanceTemplateValidation verifies template validation endpoint.
func TestInstanceTemplateValidation(t *testing.T) {
	env := setupTestEnv(t)
	resetPhase2DB(t, env.db)
	adminToken := mustSetup(t, env)

	credID := createTestAWSCredential(t, env, adminToken, "Validation Cred")

	// Valid template.
	resp := env.do("POST", "/instance-templates/validate", map[string]any{
		"cloud_credential_id": credID,
		"region":              "us-east-1",
		"instance_size":       "t3.micro",
		"os_image":            "ami-0abcdef1234567890",
		"security_groups":     []string{"sg-12345"},
	}, adminToken)
	if resp.StatusCode != http.StatusOK {
		body := mustBody(resp)
		t.Fatalf("validate template (valid): expected 200, got %d: %s", resp.StatusCode, body)
	}
	resp.Body.Close()

	// Invalid region.
	resp = env.do("POST", "/instance-templates/validate", map[string]any{
		"cloud_credential_id": credID,
		"region":              "not-a-valid-region-xyz",
		"instance_size":       "t3.micro",
		"os_image":            "ami-0abcdef1234567890",
		"security_groups":     []string{"sg-12345"},
	}, adminToken)
	// Should return 200 with valid=false or 400.
	if resp.StatusCode == http.StatusInternalServerError {
		body := mustBody(resp)
		t.Fatalf("validate template (invalid region): returned 500: %s", body)
	}
	resp.Body.Close()
}

// TestCloudCredentialRBACEnforcement verifies operators cannot create/delete credentials.
func TestCloudCredentialRBACEnforcement(t *testing.T) {
	env := setupTestEnv(t)
	resetPhase2DB(t, env.db)
	adminToken := mustSetup(t, env)
	toks := setupRoles(t, env, adminToken)

	resp := env.do("POST", "/settings/cloud-credentials", map[string]any{
		"provider_type":  "aws",
		"display_name":   "Operator Should Fail",
		"default_region": "us-east-1",
		"aws": map[string]string{
			"access_key_id": "X", "secret_access_key": "Y",
		},
	}, toks.operator)
	if resp.StatusCode != http.StatusForbidden {
		body := mustBody(resp)
		t.Errorf("operator create cloud-credential: expected 403, got %d: %s", resp.StatusCode, body)
	}
	resp.Body.Close()

	// Operator can list credentials.
	resp = env.do("GET", "/settings/cloud-credentials", nil, toks.operator)
	assertStatusQuiet(t, resp, http.StatusOK)
}

// TestCloudCredentialDeleteInUseBlocked verifies that a credential referenced
// by an instance template cannot be deleted.
func TestCloudCredentialDeleteInUseBlocked(t *testing.T) {
	env := setupTestEnv(t)
	resetPhase2DB(t, env.db)
	adminToken := mustSetup(t, env)

	credID := createTestAWSCredential(t, env, adminToken, "In-Use Cred")

	// Create a template referencing the credential.
	resp := env.do("POST", "/instance-templates", map[string]any{
		"display_name":        "In-Use Template",
		"cloud_credential_id": credID,
		"region":              "us-east-1",
		"instance_size":       "t3.micro",
		"os_image":            "ami-0abcdef1234567890",
		"security_groups":     []string{"sg-12345"},
	}, adminToken)
	assertStatus(t, resp, http.StatusCreated)
	resp.Body.Close()

	// Attempt to delete the credential — should be blocked.
	resp = env.do("DELETE", "/settings/cloud-credentials/"+credID, nil, adminToken)
	if resp.StatusCode == http.StatusNoContent || resp.StatusCode == http.StatusOK {
		resp.Body.Close()
		t.Error("expected credential delete to be blocked (in use by template), but it succeeded")
		return
	}
	// Should return 409 Conflict or 422 Unprocessable Entity.
	if resp.StatusCode != http.StatusConflict && resp.StatusCode != http.StatusUnprocessableEntity &&
		resp.StatusCode != http.StatusBadRequest {
		body := mustBody(resp)
		t.Errorf("expected 409/422/400 when deleting in-use credential, got %d: %s", resp.StatusCode, body)
	}
	resp.Body.Close()
}

// createTestAWSCredential is a helper that creates an AWS credential and returns its ID.
func createTestAWSCredential(t *testing.T, env *testEnv, token, name string) string {
	t.Helper()
	resp := env.do("POST", "/settings/cloud-credentials", map[string]any{
		"provider_type":  "aws",
		"display_name":   name,
		"default_region": "us-east-1",
		"aws": map[string]string{
			"access_key_id":     "AKIAIOSFODNN7EXAMPLE",
			"secret_access_key": "wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY",
		},
	}, token)
	if resp.StatusCode != http.StatusCreated {
		body := mustBody(resp)
		t.Fatalf("createTestAWSCredential(%s): expected 201, got %d: %s", name, resp.StatusCode, body)
	}
	var cr struct {
		Data struct{ ID string `json:"id"` } `json:"data"`
	}
	decodeT(t, resp, &cr)
	return cr.Data.ID
}
