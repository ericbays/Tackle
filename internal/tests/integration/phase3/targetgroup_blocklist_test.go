//go:build integration

package phase3

import (
	"net/http"
	"testing"
)

// TestTargetGroupLifecycle verifies create, list, member management, and delete.
func TestTargetGroupLifecycle(t *testing.T) {
	env := setupTestEnv(t)
	resetPhase3DB(t, env.db)
	adminToken := mustSetup(t, env)

	// Create targets.
	t1 := mustCreateTarget(t, env, adminToken, "g1@example.com", "Group1", "User", "IT")
	t2 := mustCreateTarget(t, env, adminToken, "g2@example.com", "Group2", "User", "IT")
	t3 := mustCreateTarget(t, env, adminToken, "g3@example.com", "Group3", "User", "IT")

	// Create group.
	groupID := mustCreateTargetGroup(t, env, adminToken, "Test Group", "A test target group")

	// Add members.
	resp := env.do("POST", "/target-groups/"+groupID+"/members", map[string]any{
		"target_ids": []string{t1, t2, t3},
	}, adminToken)
	assertStatus(t, resp, http.StatusOK)
	resp.Body.Close()

	// Get group — verify member count.
	resp = env.do("GET", "/target-groups/"+groupID, nil, adminToken)
	assertStatus(t, resp, http.StatusOK)
	var group struct {
		Data struct {
			ID          string `json:"id"`
			Name        string `json:"name"`
			MemberCount int    `json:"member_count"`
		} `json:"data"`
	}
	decodeT(t, resp, &group)
	if group.Data.MemberCount != 3 {
		t.Errorf("expected member_count=3, got %d", group.Data.MemberCount)
	}

	// Remove one member.
	resp = env.do("DELETE", "/target-groups/"+groupID+"/members", map[string]any{
		"target_ids": []string{t3},
	}, adminToken)
	assertStatus(t, resp, http.StatusOK)
	resp.Body.Close()

	// Verify member count decreased.
	resp = env.do("GET", "/target-groups/"+groupID, nil, adminToken)
	assertStatus(t, resp, http.StatusOK)
	decodeT(t, resp, &group)
	if group.Data.MemberCount != 2 {
		t.Errorf("expected member_count=2 after removal, got %d", group.Data.MemberCount)
	}

	// Update group name.
	resp = env.do("PUT", "/target-groups/"+groupID, map[string]any{
		"name":        "Renamed Group",
		"description": "Updated description",
	}, adminToken)
	assertStatus(t, resp, http.StatusOK)
	resp.Body.Close()

	// List groups — verify the renamed group appears.
	resp = env.do("GET", "/target-groups?name=Renamed", nil, adminToken)
	assertStatus(t, resp, http.StatusOK)
	var listResp struct {
		Pagination struct {
			Total int `json:"total"`
		} `json:"pagination"`
	}
	decodeT(t, resp, &listResp)
	if listResp.Pagination.Total != 1 {
		t.Errorf("expected 1 group matching 'Renamed', got %d", listResp.Pagination.Total)
	}

	// Delete group.
	resp = env.do("DELETE", "/target-groups/"+groupID, nil, adminToken)
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNoContent {
		body := mustBody(resp)
		t.Fatalf("delete group: expected 200 or 204, got %d: %s", resp.StatusCode, body)
	}
	resp.Body.Close()

	// Verify gone.
	resp = env.do("GET", "/target-groups/"+groupID, nil, adminToken)
	if resp.StatusCode != http.StatusNotFound {
		resp.Body.Close()
		t.Errorf("expected 404 for deleted group, got %d", resp.StatusCode)
	} else {
		resp.Body.Close()
	}
}

// TestBlocklistCRUD verifies block list entry create, list, deactivate, and reactivate.
func TestBlocklistCRUD(t *testing.T) {
	env := setupTestEnv(t)
	resetPhase3DB(t, env.db)
	adminToken := mustSetup(t, env)

	// Create block list entries.
	id1 := mustCreateBlocklistEntry(t, env, adminToken, "*@blocked.com", "Known malicious domain")
	_ = mustCreateBlocklistEntry(t, env, adminToken, "ceo@example.com", "Executive protection")

	// List — should have 2.
	resp := env.do("GET", "/blocklist", nil, adminToken)
	assertStatus(t, resp, http.StatusOK)
	var listResp struct {
		Pagination struct {
			Total int `json:"total"`
		} `json:"pagination"`
	}
	decodeT(t, resp, &listResp)
	if listResp.Pagination.Total != 2 {
		t.Errorf("expected 2 blocklist entries, got %d", listResp.Pagination.Total)
	}

	// Deactivate one entry.
	resp = env.do("PUT", "/blocklist/"+id1+"/deactivate", nil, adminToken)
	assertStatus(t, resp, http.StatusOK)
	resp.Body.Close()

	// Get entry — verify deactivated.
	resp = env.do("GET", "/blocklist/"+id1, nil, adminToken)
	assertStatus(t, resp, http.StatusOK)
	var entry struct {
		Data struct {
			IsActive bool `json:"is_active"`
		} `json:"data"`
	}
	decodeT(t, resp, &entry)
	if entry.Data.IsActive {
		t.Error("expected entry to be deactivated")
	}

	// Reactivate.
	resp = env.do("PUT", "/blocklist/"+id1+"/reactivate", nil, adminToken)
	assertStatus(t, resp, http.StatusOK)
	resp.Body.Close()

	// Verify active again.
	resp = env.do("GET", "/blocklist/"+id1, nil, adminToken)
	assertStatus(t, resp, http.StatusOK)
	decodeT(t, resp, &entry)
	if !entry.Data.IsActive {
		t.Error("expected entry to be reactivated")
	}
}

// TestBlocklistCheck verifies the check endpoint matches email patterns.
func TestBlocklistCheck(t *testing.T) {
	env := setupTestEnv(t)
	resetPhase3DB(t, env.db)
	adminToken := mustSetup(t, env)

	mustCreateBlocklistEntry(t, env, adminToken, "*@blocked.com", "Domain block")
	mustCreateBlocklistEntry(t, env, adminToken, "specific@example.com", "Individual block")

	// Check blocked domain.
	resp := env.do("GET", "/blocklist/check?email=anyone@blocked.com", nil, adminToken)
	assertStatus(t, resp, http.StatusOK)
	var check struct {
		Data struct {
			Blocked bool `json:"blocked"`
		} `json:"data"`
	}
	decodeT(t, resp, &check)
	if !check.Data.Blocked {
		t.Error("expected anyone@blocked.com to be blocked")
	}

	// Check specific blocked email.
	resp = env.do("GET", "/blocklist/check?email=specific@example.com", nil, adminToken)
	assertStatus(t, resp, http.StatusOK)
	decodeT(t, resp, &check)
	if !check.Data.Blocked {
		t.Error("expected specific@example.com to be blocked")
	}

	// Check non-blocked email.
	resp = env.do("GET", "/blocklist/check?email=safe@example.com", nil, adminToken)
	assertStatus(t, resp, http.StatusOK)
	decodeT(t, resp, &check)
	if check.Data.Blocked {
		t.Error("expected safe@example.com to NOT be blocked")
	}
}

// TestBlocklistRBACAdminOnly verifies only Admin can manage block list.
func TestBlocklistRBACAdminOnly(t *testing.T) {
	env := setupTestEnv(t)
	resetPhase3DB(t, env.db)
	adminToken := mustSetup(t, env)
	toks := setupRoles(t, env, adminToken)

	// Operator cannot create block list entries.
	resp := env.do("POST", "/blocklist", map[string]any{
		"pattern": "*@test.com",
		"reason":  "test",
	}, toks.operator)
	if resp.StatusCode != http.StatusForbidden {
		resp.Body.Close()
		t.Errorf("expected 403 for operator creating blocklist entry, got %d", resp.StatusCode)
	} else {
		resp.Body.Close()
	}

	// Defender cannot create block list entries.
	resp = env.do("POST", "/blocklist", map[string]any{
		"pattern": "*@test.com",
		"reason":  "test",
	}, toks.defender)
	if resp.StatusCode != http.StatusForbidden {
		resp.Body.Close()
		t.Errorf("expected 403 for defender creating blocklist entry, got %d", resp.StatusCode)
	} else {
		resp.Body.Close()
	}

	// Admin can create.
	resp = env.do("POST", "/blocklist", map[string]any{
		"pattern": "*@admin-blocked.com",
		"reason":  "Admin-only entry",
	}, toks.admin)
	assertStatus(t, resp, http.StatusCreated)
	resp.Body.Close()
}

// TestCampaignTargetGroupAssignment verifies assigning target groups to campaigns.
func TestCampaignTargetGroupAssignment(t *testing.T) {
	env := setupTestEnv(t)
	resetPhase3DB(t, env.db)
	adminToken := mustSetup(t, env)

	// Create targets and group.
	t1 := mustCreateTarget(t, env, adminToken, "cg1@example.com", "CG1", "User", "IT")
	t2 := mustCreateTarget(t, env, adminToken, "cg2@example.com", "CG2", "User", "IT")
	groupID := mustCreateTargetGroup(t, env, adminToken, "Campaign Group", "For campaign assignment")
	resp := env.do("POST", "/target-groups/"+groupID+"/members", map[string]any{
		"target_ids": []string{t1, t2},
	}, adminToken)
	assertStatus(t, resp, http.StatusOK)
	resp.Body.Close()

	// Create campaign.
	campaignID := mustCreateCampaign(t, env, adminToken, "Group Assignment Test")

	// Assign group to campaign.
	resp = env.do("POST", "/campaigns/"+campaignID+"/target-groups", map[string]any{
		"group_id": groupID,
	}, adminToken)
	assertStatus(t, resp, http.StatusOK)
	resp.Body.Close()

	// List campaign groups.
	resp = env.do("GET", "/campaigns/"+campaignID+"/target-groups", nil, adminToken)
	assertStatus(t, resp, http.StatusOK)
	var groups struct {
		Data []struct {
			ID   string `json:"id"`
			Name string `json:"name"`
		} `json:"data"`
	}
	decodeT(t, resp, &groups)
	if len(groups.Data) != 1 {
		t.Fatalf("expected 1 campaign group, got %d", len(groups.Data))
	}

	// Resolve targets.
	resp = env.do("GET", "/campaigns/"+campaignID+"/resolve-targets", nil, adminToken)
	assertStatus(t, resp, http.StatusOK)
	var resolved struct {
		Data struct {
			TotalTargets int `json:"total_targets"`
		} `json:"data"`
	}
	decodeT(t, resp, &resolved)
	if resolved.Data.TotalTargets != 2 {
		t.Errorf("expected 2 resolved targets, got %d", resolved.Data.TotalTargets)
	}

	// Unassign group.
	resp = env.do("DELETE", "/campaigns/"+campaignID+"/target-groups/"+groupID, nil, adminToken)
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNoContent {
		body := mustBody(resp)
		t.Fatalf("unassign group: expected 200 or 204, got %d: %s", resp.StatusCode, body)
	}
	resp.Body.Close()
}
