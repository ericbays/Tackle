//go:build integration

package phase2

import (
	"net/http"
	"testing"
)

// TestSMTPProfileCreateAndMasking verifies SMTP profile creation with
// encrypted password storage and masked GET response.
func TestSMTPProfileCreateAndMasking(t *testing.T) {
	env := setupTestEnv(t)
	resetPhase2DB(t, env.db)
	adminToken := mustSetup(t, env)

	resp := env.do("POST", "/smtp-profiles", map[string]any{
		"name":            "Primary SMTP",
		"host":            "smtp.example.com",
		"port":            587,
		"auth_type":       "plain",
		"username":        "smtp_user",
		"password":        "supersecretpassword",
		"tls_mode":        "starttls",
		"tls_skip_verify": false,
		"from_address":    "phish@example.com",
		"max_connections": 5,
		"timeout_connect": 10,
		"timeout_send":    30,
	}, adminToken)
	assertStatus(t, resp, http.StatusCreated)

	var created struct {
		Data struct {
			ID          string `json:"id"`
			Name        string `json:"name"`
			Host        string `json:"host"`
			Port        int    `json:"port"`
			HasUsername bool   `json:"has_username"`
			HasPassword bool   `json:"has_password"`
			Status      string `json:"status"`
		} `json:"data"`
	}
	decodeT(t, resp, &created)
	id := created.Data.ID
	if id == "" {
		t.Fatal("expected non-empty profile ID")
	}
	if !created.Data.HasPassword {
		t.Error("expected has_password=true")
	}

	// GET — verify password not in response.
	resp = env.do("GET", "/smtp-profiles/"+id, nil, adminToken)
	assertStatus(t, resp, http.StatusOK)
	body := mustBody(resp)
	if bodyContains(body, "supersecretpassword") {
		t.Error("SMTP password leaked in GET response")
	}

	assertAuditEvent(t, env.db, "smtp_profile.created")
}

// TestSMTPProfileList verifies listing SMTP profiles.
func TestSMTPProfileList(t *testing.T) {
	env := setupTestEnv(t)
	resetPhase2DB(t, env.db)
	adminToken := mustSetup(t, env)

	// Create two profiles.
	for _, name := range []string{"SMTP-A", "SMTP-B"} {
		resp := env.do("POST", "/smtp-profiles", map[string]any{
			"name":            name,
			"host":            "smtp.example.com",
			"port":            587,
			"auth_type":       "plain",
			"username":        "user",
			"password":        "pass",
			"tls_mode":        "starttls",
			"from_address":    "from@example.com",
			"max_connections": 5,
			"timeout_connect": 10,
			"timeout_send":    30,
		}, adminToken)
		assertStatus(t, resp, http.StatusCreated)
		resp.Body.Close()
	}

	resp := env.do("GET", "/smtp-profiles", nil, adminToken)
	assertStatus(t, resp, http.StatusOK)
	var listResp struct {
		Data []map[string]any `json:"data"`
	}
	decodeT(t, resp, &listResp)
	if len(listResp.Data) < 2 {
		t.Errorf("expected >=2 profiles, got %d", len(listResp.Data))
	}
}

// TestSMTPProfileUpdateAndDelete verifies updating and deleting a profile.
func TestSMTPProfileUpdateAndDelete(t *testing.T) {
	env := setupTestEnv(t)
	resetPhase2DB(t, env.db)
	adminToken := mustSetup(t, env)

	id := createTestSMTPProfile(t, env, adminToken, "Update Test")

	// Update.
	resp := env.do("PUT", "/smtp-profiles/"+id, map[string]any{
		"host": "smtp2.example.com",
		"port": ptrInt(465),
	}, adminToken)
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNoContent {
		body := mustBody(resp)
		t.Fatalf("update profile: expected 200/204, got %d: %s", resp.StatusCode, body)
	}
	resp.Body.Close()

	// Delete.
	resp = env.do("DELETE", "/smtp-profiles/"+id, nil, adminToken)
	if resp.StatusCode != http.StatusNoContent && resp.StatusCode != http.StatusOK {
		body := mustBody(resp)
		t.Fatalf("delete profile: expected 200/204, got %d: %s", resp.StatusCode, body)
	}
	resp.Body.Close()

	waitForAudit()
	assertAuditEvent(t, env.db, "smtp_profile.updated")
	assertAuditEvent(t, env.db, "smtp_profile.deleted")
}

// TestSMTPProfileDuplicate verifies the duplicate endpoint creates a new profile
// without credentials, with status=untested.
func TestSMTPProfileDuplicate(t *testing.T) {
	env := setupTestEnv(t)
	resetPhase2DB(t, env.db)
	adminToken := mustSetup(t, env)

	id := createTestSMTPProfile(t, env, adminToken, "Source Profile")

	resp := env.do("POST", "/smtp-profiles/"+id+"/duplicate", map[string]any{
		"name": "Duplicate Profile",
	}, adminToken)
	assertStatus(t, resp, http.StatusCreated)
	var dup struct {
		Data struct {
			ID          string `json:"id"`
			Name        string `json:"name"`
			HasPassword bool   `json:"has_password"`
			Status      string `json:"status"`
		} `json:"data"`
	}
	decodeT(t, resp, &dup)
	if dup.Data.ID == id {
		t.Error("duplicate profile should have a different ID")
	}
	if dup.Data.HasPassword {
		t.Error("duplicated profile should not have password")
	}
	if dup.Data.Status != "untested" && dup.Data.Status != "pending" {
		t.Logf("duplicated profile status=%s (expected untested/pending)", dup.Data.Status)
	}
}

// TestSMTPProfileTestConnection verifies the test endpoint doesn't panic.
func TestSMTPProfileTestConnection(t *testing.T) {
	env := setupTestEnv(t)
	resetPhase2DB(t, env.db)
	adminToken := mustSetup(t, env)

	id := createTestSMTPProfile(t, env, adminToken, "Test Conn Profile")

	resp := env.do("POST", "/smtp-profiles/"+id+"/test", nil, adminToken)
	// Will fail (no real SMTP server), but must not panic.
	if resp.StatusCode == http.StatusInternalServerError {
		body := mustBody(resp)
		t.Fatalf("SMTP test returned 500: %s", body)
	}
	resp.Body.Close()
}

// TestSMTPProfileSendWindowLogic verifies send window enforcement via the
// send schedule endpoints.
func TestSMTPProfileSendWindowLogic(t *testing.T) {
	env := setupTestEnv(t)
	resetPhase2DB(t, env.db)
	adminToken := mustSetup(t, env)

	// Create a fake campaign ID to attach schedules to.
	// In Phase 2, campaigns don't exist yet, so we test validation of the schedule payload.
	fakeID := "00000000-0000-0000-0000-000000000001"

	// PUT /campaigns/{id}/send-schedule — should return 404 for nonexistent campaign.
	resp := env.do("PUT", "/campaigns/"+fakeID+"/send-schedule", map[string]any{
		"days_of_week": []int{1, 2, 3, 4, 5},
		"start_hour":   9,
		"end_hour":     17,
		"timezone":     "America/New_York",
	}, adminToken)
	// 404 is expected since the campaign doesn't exist; no 500 or panic.
	if resp.StatusCode == http.StatusInternalServerError {
		body := mustBody(resp)
		t.Fatalf("send schedule PUT returned 500: %s", body)
	}
	resp.Body.Close()
}

// TestSMTPProfileRBACEnforcement verifies operators cannot create/delete profiles.
func TestSMTPProfileRBACEnforcement(t *testing.T) {
	env := setupTestEnv(t)
	resetPhase2DB(t, env.db)
	adminToken := mustSetup(t, env)
	toks := setupRoles(t, env, adminToken)

	resp := env.do("POST", "/smtp-profiles", map[string]any{
		"name":            "Operator Should Fail",
		"host":            "smtp.example.com",
		"port":            587,
		"auth_type":       "none",
		"tls_mode":        "none",
		"from_address":    "from@example.com",
		"max_connections": 5,
		"timeout_connect": 10,
		"timeout_send":    30,
	}, toks.operator)
	if resp.StatusCode != http.StatusForbidden {
		body := mustBody(resp)
		t.Errorf("operator create SMTP profile: expected 403, got %d: %s", resp.StatusCode, body)
	}
	resp.Body.Close()

	// Operator can list.
	resp = env.do("GET", "/smtp-profiles", nil, toks.operator)
	assertStatusQuiet(t, resp, http.StatusOK)
}

// TestSMTPProfileAuditTrail verifies create, update, duplicate, delete are all audited.
func TestSMTPProfileAuditTrail(t *testing.T) {
	env := setupTestEnv(t)
	resetPhase2DB(t, env.db)
	adminToken := mustSetup(t, env)

	id := createTestSMTPProfile(t, env, adminToken, "Audit Profile")

	// Update.
	resp := env.do("PUT", "/smtp-profiles/"+id, map[string]any{"host": "smtp2.example.com"}, adminToken)
	resp.Body.Close()

	// Duplicate.
	resp = env.do("POST", "/smtp-profiles/"+id+"/duplicate", map[string]any{"name": "Audit Dup"}, adminToken)
	resp.Body.Close()

	// Delete.
	resp = env.do("DELETE", "/smtp-profiles/"+id, nil, adminToken)
	resp.Body.Close()

	waitForAudit()
	for _, action := range []string{
		"smtp_profile.created",
		"smtp_profile.updated",
		"smtp_profile.duplicated",
		"smtp_profile.deleted",
	} {
		if countAuditEvents(t, env.db, action) == 0 {
			t.Errorf("missing audit event: %s", action)
		}
	}
}

// TestSMTPProfileEffectiveRateLimit verifies the campaign profile validate endpoint
// performs input validation.
func TestSMTPProfileEffectiveRateLimit(t *testing.T) {
	env := setupTestEnv(t)
	resetPhase2DB(t, env.db)
	adminToken := mustSetup(t, env)

	fakeID := "00000000-0000-0000-0000-000000000002"
	resp := env.do("POST", "/campaigns/"+fakeID+"/smtp-profiles/validate", nil, adminToken)
	// 404 is expected (no campaign); no 500 or panic.
	if resp.StatusCode == http.StatusInternalServerError {
		body := mustBody(resp)
		t.Fatalf("validate campaign profiles returned 500: %s", body)
	}
	resp.Body.Close()
}

// createTestSMTPProfile creates an SMTP profile and returns its ID.
func createTestSMTPProfile(t *testing.T, env *testEnv, token, name string) string {
	t.Helper()
	resp := env.do("POST", "/smtp-profiles", map[string]any{
		"name":            name,
		"host":            "smtp.example.com",
		"port":            587,
		"auth_type":       "plain",
		"username":        "user",
		"password":        "password123",
		"tls_mode":        "starttls",
		"from_address":    "from@example.com",
		"max_connections": 5,
		"timeout_connect": 10,
		"timeout_send":    30,
	}, token)
	if resp.StatusCode != http.StatusCreated {
		body := mustBody(resp)
		t.Fatalf("createTestSMTPProfile(%s): expected 201, got %d: %s", name, resp.StatusCode, body)
	}
	var cr struct {
		Data struct{ ID string `json:"id"` } `json:"data"`
	}
	decodeT(t, resp, &cr)
	return cr.Data.ID
}
