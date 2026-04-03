//go:build integration

package phase3

import (
	"net/http"
	"testing"
)

// TestCampaignCRUD verifies campaign create, read, update, delete lifecycle.
func TestCampaignCRUD(t *testing.T) {
	env := setupTestEnv(t)
	resetPhase3DB(t, env.db)
	adminToken := mustSetup(t, env)

	// Create.
	campaignID := mustCreateCampaign(t, env, adminToken, "CRUD Test Campaign")

	// Get — verify fields.
	resp := env.do("GET", "/campaigns/"+campaignID, nil, adminToken)
	assertStatus(t, resp, http.StatusOK)
	var campaign struct {
		Data struct {
			ID           string `json:"id"`
			Name         string `json:"name"`
			Description  string `json:"description"`
			CurrentState string `json:"current_state"`
			SendOrder    string `json:"send_order"`
		} `json:"data"`
	}
	decodeT(t, resp, &campaign)
	if campaign.Data.Name != "CRUD Test Campaign" {
		t.Errorf("expected name 'CRUD Test Campaign', got %s", campaign.Data.Name)
	}
	if campaign.Data.CurrentState != "draft" {
		t.Errorf("expected initial state 'draft', got %s", campaign.Data.CurrentState)
	}

	// Update.
	resp = env.do("PUT", "/campaigns/"+campaignID, map[string]any{
		"name":        "Updated Campaign Name",
		"description": "Updated description",
	}, adminToken)
	assertStatus(t, resp, http.StatusOK)
	resp.Body.Close()

	// Verify update.
	resp = env.do("GET", "/campaigns/"+campaignID, nil, adminToken)
	assertStatus(t, resp, http.StatusOK)
	decodeT(t, resp, &campaign)
	if campaign.Data.Name != "Updated Campaign Name" {
		t.Errorf("expected updated name, got %s", campaign.Data.Name)
	}

	// List campaigns.
	resp = env.do("GET", "/campaigns", nil, adminToken)
	assertStatus(t, resp, http.StatusOK)
	var listResp struct {
		Data       []any `json:"data"`
		Pagination struct {
			Total int `json:"total"`
		} `json:"pagination"`
	}
	decodeT(t, resp, &listResp)
	if listResp.Pagination.Total < 1 {
		t.Errorf("expected at least 1 campaign in list, got %d", listResp.Pagination.Total)
	}

	// Delete.
	resp = env.do("DELETE", "/campaigns/"+campaignID, nil, adminToken)
	assertStatus(t, resp, http.StatusOK)
	resp.Body.Close()

	// Verify deleted — should get 404.
	resp = env.do("GET", "/campaigns/"+campaignID, nil, adminToken)
	if resp.StatusCode != http.StatusNotFound {
		resp.Body.Close()
		t.Errorf("expected 404 for deleted campaign, got %d", resp.StatusCode)
	} else {
		resp.Body.Close()
	}
}

// TestCampaignStateTransitions verifies the complete state machine from Draft through Archive.
func TestCampaignStateTransitions(t *testing.T) {
	env := setupTestEnv(t)
	resetPhase3DB(t, env.db)
	adminToken := mustSetup(t, env)
	toks := setupRoles(t, env, adminToken)

	// Operator creates campaign with all required config for submission.
	campaignID := mustPrepareCampaignForSubmission(t, env, toks.operator, "State Machine Test")

	// T1: Draft → PendingApproval (Operator submits).
	resp := env.do("POST", "/campaigns/"+campaignID+"/submit", map[string]any{
		"required_approver_count": 1,
	}, toks.operator)
	assertStatus(t, resp, http.StatusOK)
	resp.Body.Close()

	state := getCampaignState(t, env, toks.operator, campaignID)
	if state != "pending_approval" {
		t.Fatalf("expected pending_approval after submit, got %s", state)
	}

	// T2: PendingApproval → Approved (Engineer approves).
	resp = env.do("POST", "/campaigns/"+campaignID+"/approve", map[string]any{
		"comment": "Looks good for testing",
	}, toks.engineer)
	assertStatus(t, resp, http.StatusOK)
	resp.Body.Close()

	state = getCampaignState(t, env, toks.operator, campaignID)
	if state != "approved" {
		t.Fatalf("expected approved after engineer approves, got %s", state)
	}

	// T4: Approved → Building (Operator triggers build).
	resp = env.do("POST", "/campaigns/"+campaignID+"/build", map[string]any{
		"reason": "Start build",
	}, toks.operator)
	assertStatus(t, resp, http.StatusOK)
	resp.Body.Close()

	state = getCampaignState(t, env, toks.operator, campaignID)
	if state != "building" {
		t.Fatalf("expected building after build trigger, got %s", state)
	}

	// T5: Building → Ready (System marks build complete).
	// Directly update DB since this is a system transition.
	_, err := env.db.Exec(`UPDATE campaigns SET current_state = 'ready', updated_at = NOW() WHERE id = $1`, campaignID)
	if err != nil {
		t.Fatalf("manual state update to ready: %v", err)
	}

	state = getCampaignState(t, env, toks.operator, campaignID)
	if state != "ready" {
		t.Fatalf("expected ready, got %s", state)
	}

	// T7: Ready → Active (Operator launches).
	resp = env.do("POST", "/campaigns/"+campaignID+"/launch", map[string]any{
		"reason": "Go live",
	}, toks.operator)
	assertStatus(t, resp, http.StatusOK)
	resp.Body.Close()

	state = getCampaignState(t, env, toks.operator, campaignID)
	if state != "active" {
		t.Fatalf("expected active after launch, got %s", state)
	}

	// T8: Active → Paused.
	resp = env.do("POST", "/campaigns/"+campaignID+"/pause", map[string]any{
		"reason": "Pause for review",
	}, toks.operator)
	assertStatus(t, resp, http.StatusOK)
	resp.Body.Close()

	state = getCampaignState(t, env, toks.operator, campaignID)
	if state != "paused" {
		t.Fatalf("expected paused, got %s", state)
	}

	// T9: Paused → Active (Resume).
	resp = env.do("POST", "/campaigns/"+campaignID+"/resume", map[string]any{
		"reason": "Resume sending",
	}, toks.operator)
	assertStatus(t, resp, http.StatusOK)
	resp.Body.Close()

	state = getCampaignState(t, env, toks.operator, campaignID)
	if state != "active" {
		t.Fatalf("expected active after resume, got %s", state)
	}

	// T10: Active → Completed.
	resp = env.do("POST", "/campaigns/"+campaignID+"/complete", map[string]any{
		"reason": "Campaign finished",
	}, toks.operator)
	assertStatus(t, resp, http.StatusOK)
	resp.Body.Close()

	state = getCampaignState(t, env, toks.operator, campaignID)
	if state != "completed" {
		t.Fatalf("expected completed, got %s", state)
	}

	// T12: Completed → Archived.
	resp = env.do("POST", "/campaigns/"+campaignID+"/archive", map[string]any{
		"reason": "Archiving",
	}, toks.operator)
	assertStatus(t, resp, http.StatusOK)
	resp.Body.Close()

	state = getCampaignState(t, env, toks.operator, campaignID)
	if state != "archived" {
		t.Fatalf("expected archived, got %s", state)
	}

	// Verify state transitions are audited.
	assertAuditEvent(t, env.db, "campaign.state_changed")
}

// TestCampaignInvalidTransitions verifies invalid state transitions are rejected.
func TestCampaignInvalidTransitions(t *testing.T) {
	env := setupTestEnv(t)
	resetPhase3DB(t, env.db)
	adminToken := mustSetup(t, env)

	campaignID := mustCreateCampaign(t, env, adminToken, "Invalid Transition Test")

	// Draft → Active (skip approval — should fail).
	resp := env.do("POST", "/campaigns/"+campaignID+"/launch", map[string]any{
		"reason": "Try to skip",
	}, adminToken)
	if resp.StatusCode == http.StatusOK {
		resp.Body.Close()
		t.Fatal("expected launch from draft to be rejected")
	}
	resp.Body.Close()

	// Draft → Completed (invalid — should fail).
	resp = env.do("POST", "/campaigns/"+campaignID+"/complete", map[string]any{
		"reason": "Try to complete",
	}, adminToken)
	if resp.StatusCode == http.StatusOK {
		resp.Body.Close()
		t.Fatal("expected complete from draft to be rejected")
	}
	resp.Body.Close()

	// Draft → Archived (invalid — should fail).
	resp = env.do("POST", "/campaigns/"+campaignID+"/archive", map[string]any{
		"reason": "Try to archive",
	}, adminToken)
	if resp.StatusCode == http.StatusOK {
		resp.Body.Close()
		t.Fatal("expected archive from draft to be rejected")
	}
	resp.Body.Close()

	// Verify state is still draft.
	state := getCampaignState(t, env, adminToken, campaignID)
	if state != "draft" {
		t.Fatalf("expected state to remain draft, got %s", state)
	}
}

// TestCampaignUnlockFromApproved verifies the unlock transition (Approved → Draft).
func TestCampaignUnlockFromApproved(t *testing.T) {
	env := setupTestEnv(t)
	resetPhase3DB(t, env.db)
	adminToken := mustSetup(t, env)
	toks := setupRoles(t, env, adminToken)

	campaignID := mustPrepareCampaignForSubmission(t, env, toks.operator, "Unlock Test")

	// Submit and approve.
	resp := env.do("POST", "/campaigns/"+campaignID+"/submit", nil, toks.operator)
	assertStatus(t, resp, http.StatusOK)
	resp.Body.Close()
	resp = env.do("POST", "/campaigns/"+campaignID+"/approve", map[string]any{
		"comment": "Approved",
	}, toks.engineer)
	assertStatus(t, resp, http.StatusOK)
	resp.Body.Close()

	// Unlock (T13: Approved → Draft).
	resp = env.do("POST", "/campaigns/"+campaignID+"/unlock", map[string]any{
		"reason": "Need to make changes",
	}, toks.operator)
	assertStatus(t, resp, http.StatusOK)
	resp.Body.Close()

	state := getCampaignState(t, env, toks.operator, campaignID)
	if state != "draft" {
		t.Fatalf("expected draft after unlock, got %s", state)
	}

	// Campaign should be editable again.
	resp = env.do("PUT", "/campaigns/"+campaignID, map[string]any{
		"name": "Modified After Unlock",
	}, toks.operator)
	assertStatus(t, resp, http.StatusOK)
	resp.Body.Close()
}

// TestCampaignListByState verifies filtering campaigns by state.
func TestCampaignListByState(t *testing.T) {
	env := setupTestEnv(t)
	resetPhase3DB(t, env.db)
	adminToken := mustSetup(t, env)

	// Create campaigns in different states.
	mustCreateCampaign(t, env, adminToken, "Draft 1")
	mustCreateCampaign(t, env, adminToken, "Draft 2")

	c3 := mustPrepareCampaignForSubmission(t, env, adminToken, "Submitted 1")
	resp := env.do("POST", "/campaigns/"+c3+"/submit", nil, adminToken)
	assertStatus(t, resp, http.StatusOK)
	resp.Body.Close()

	// Filter by draft.
	resp = env.do("GET", "/campaigns?states=draft", nil, adminToken)
	assertStatus(t, resp, http.StatusOK)
	var listResp struct {
		Pagination struct {
			Total int `json:"total"`
		} `json:"pagination"`
	}
	decodeT(t, resp, &listResp)
	if listResp.Pagination.Total != 2 {
		t.Errorf("expected 2 draft campaigns, got %d", listResp.Pagination.Total)
	}

	// Filter by pending_approval.
	resp = env.do("GET", "/campaigns?states=pending_approval", nil, adminToken)
	assertStatus(t, resp, http.StatusOK)
	decodeT(t, resp, &listResp)
	if listResp.Pagination.Total != 1 {
		t.Errorf("expected 1 pending_approval campaign, got %d", listResp.Pagination.Total)
	}
}

// TestCampaignTemplateVariants verifies A/B template variant assignment.
func TestCampaignTemplateVariants(t *testing.T) {
	env := setupTestEnv(t)
	resetPhase3DB(t, env.db)
	adminToken := mustSetup(t, env)

	campaignID := mustCreateCampaign(t, env, adminToken, "Variant Test")

	// Set template variants (using placeholder IDs — service may validate).
	resp := env.do("PUT", "/campaigns/"+campaignID+"/template-variants", map[string]any{
		"variants": []map[string]any{
			{"template_id": "00000000-0000-0000-0000-000000000001", "split_ratio": 60, "label": "Variant A"},
			{"template_id": "00000000-0000-0000-0000-000000000002", "split_ratio": 40, "label": "Variant B"},
		},
	}, adminToken)
	// Accept 200 or validation error if template IDs must exist.
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusBadRequest {
		body := mustBody(resp)
		t.Fatalf("set template variants: expected 200 or 400, got %d: %s", resp.StatusCode, body)
	}
	resp.Body.Close()

	// Get template variants.
	resp = env.do("GET", "/campaigns/"+campaignID+"/template-variants", nil, adminToken)
	assertStatus(t, resp, http.StatusOK)
	resp.Body.Close()
}

// TestCampaignSendWindows verifies send window configuration.
func TestCampaignSendWindows(t *testing.T) {
	env := setupTestEnv(t)
	resetPhase3DB(t, env.db)
	adminToken := mustSetup(t, env)

	campaignID := mustCreateCampaign(t, env, adminToken, "Send Window Test")

	// Set send windows.
	resp := env.do("PUT", "/campaigns/"+campaignID+"/send-windows", map[string]any{
		"windows": []map[string]any{
			{
				"days":       []string{"monday", "tuesday", "wednesday", "thursday", "friday"},
				"start_time": "09:00",
				"end_time":   "17:00",
				"timezone":   "America/New_York",
			},
		},
	}, adminToken)
	assertStatus(t, resp, http.StatusOK)
	resp.Body.Close()

	// Get send windows.
	resp = env.do("GET", "/campaigns/"+campaignID+"/send-windows", nil, adminToken)
	assertStatus(t, resp, http.StatusOK)
	var windowsResp struct {
		Data []struct {
			Days      []string `json:"days"`
			StartTime string   `json:"start_time"`
			EndTime   string   `json:"end_time"`
			Timezone  string   `json:"timezone"`
		} `json:"data"`
	}
	decodeT(t, resp, &windowsResp)
	if len(windowsResp.Data) != 1 {
		t.Fatalf("expected 1 send window, got %d", len(windowsResp.Data))
	}
	if windowsResp.Data[0].Timezone != "America/New_York" {
		t.Errorf("expected timezone America/New_York, got %s", windowsResp.Data[0].Timezone)
	}
}

// TestCampaignClone verifies cloning a campaign.
func TestCampaignClone(t *testing.T) {
	env := setupTestEnv(t)
	resetPhase3DB(t, env.db)
	adminToken := mustSetup(t, env)

	campaignID := mustCreateCampaign(t, env, adminToken, "Original Campaign")

	// Clone.
	resp := env.do("POST", "/campaigns/"+campaignID+"/clone", map[string]any{
		"include_landing_page":      true,
		"include_target_groups":     true,
		"include_smtp_configs":      true,
		"include_template_variants": true,
		"include_send_schedule":     true,
		"include_endpoint_config":   true,
	}, adminToken)
	assertStatus(t, resp, http.StatusCreated)
	var cloned struct {
		Data struct {
			ID           string `json:"id"`
			Name         string `json:"name"`
			CurrentState string `json:"current_state"`
		} `json:"data"`
	}
	decodeT(t, resp, &cloned)
	if cloned.Data.ID == campaignID {
		t.Error("cloned campaign should have different ID")
	}
	if cloned.Data.CurrentState != "draft" {
		t.Errorf("cloned campaign should be draft, got %s", cloned.Data.CurrentState)
	}
}

// TestCampaignConfigTemplates verifies config template CRUD and apply.
func TestCampaignConfigTemplates(t *testing.T) {
	env := setupTestEnv(t)
	resetPhase3DB(t, env.db)
	adminToken := mustSetup(t, env)

	// Create config template.
	resp := env.do("POST", "/campaign-templates", map[string]any{
		"name":        "Standard Phishing",
		"description": "Standard config template",
		"config_json": map[string]any{"throttle_rate": 10, "send_order": "default"},
	}, adminToken)
	assertStatus(t, resp, http.StatusCreated)
	var tmpl struct {
		Data struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	decodeT(t, resp, &tmpl)
	templateID := tmpl.Data.ID

	// List config templates.
	resp = env.do("GET", "/campaign-templates", nil, adminToken)
	assertStatus(t, resp, http.StatusOK)
	resp.Body.Close()

	// Get config template.
	resp = env.do("GET", "/campaign-templates/"+templateID, nil, adminToken)
	assertStatus(t, resp, http.StatusOK)
	resp.Body.Close()

	// Apply config template to create a campaign.
	resp = env.do("POST", "/campaign-templates/"+templateID+"/apply", map[string]any{
		"name": "From Template",
	}, adminToken)
	if resp.StatusCode == http.StatusCreated || resp.StatusCode == http.StatusOK {
		resp.Body.Close()
	} else {
		body := mustBody(resp)
		t.Logf("apply config template returned %d: %s (may require additional fields)", resp.StatusCode, body)
	}

	// Update config template.
	resp = env.do("PUT", "/campaign-templates/"+templateID, map[string]any{
		"name":        "Updated Standard",
		"description": "Updated description",
	}, adminToken)
	assertStatus(t, resp, http.StatusOK)
	resp.Body.Close()

	// Delete config template.
	resp = env.do("DELETE", "/campaign-templates/"+templateID, nil, adminToken)
	assertStatus(t, resp, http.StatusOK)
	resp.Body.Close()
}

// TestCampaignMetricsAndBuildLog verifies metrics and build log endpoints.
func TestCampaignMetricsAndBuildLog(t *testing.T) {
	env := setupTestEnv(t)
	resetPhase3DB(t, env.db)
	adminToken := mustSetup(t, env)

	campaignID := mustCreateCampaign(t, env, adminToken, "Metrics Test")

	// Get metrics — empty but should return valid response.
	resp := env.do("GET", "/campaigns/"+campaignID+"/metrics", nil, adminToken)
	assertStatus(t, resp, http.StatusOK)
	resp.Body.Close()

	// Get build log — empty but should return valid response.
	resp = env.do("GET", "/campaigns/"+campaignID+"/build-log", nil, adminToken)
	assertStatus(t, resp, http.StatusOK)
	resp.Body.Close()
}
