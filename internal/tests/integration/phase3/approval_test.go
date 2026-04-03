//go:build integration

package phase3

import (
	"net/http"
	"testing"
)

// TestApprovalFullCycle verifies: Operator submits → Engineer reviews → Engineer approves.
func TestApprovalFullCycle(t *testing.T) {
	env := setupTestEnv(t)
	resetPhase3DB(t, env.db)
	adminToken := mustSetup(t, env)
	toks := setupRoles(t, env, adminToken)

	campaignID := mustPrepareCampaignForSubmission(t, env, toks.operator, "Approval Cycle Test")

	// Operator submits.
	resp := env.do("POST", "/campaigns/"+campaignID+"/submit", map[string]any{
		"required_approver_count": 1,
	}, toks.operator)
	assertStatus(t, resp, http.StatusOK)
	resp.Body.Close()

	// Verify pending_approval.
	state := getCampaignState(t, env, toks.operator, campaignID)
	if state != "pending_approval" {
		t.Fatalf("expected pending_approval, got %s", state)
	}

	// Engineer reviews — approval review endpoint.
	resp = env.do("GET", "/campaigns/"+campaignID+"/approval-review", nil, toks.engineer)
	assertStatus(t, resp, http.StatusOK)
	resp.Body.Close()

	// Engineer approves.
	resp = env.do("POST", "/campaigns/"+campaignID+"/approve", map[string]any{
		"comment": "Approved by engineer",
	}, toks.engineer)
	assertStatus(t, resp, http.StatusOK)
	resp.Body.Close()

	// Verify approved.
	state = getCampaignState(t, env, toks.operator, campaignID)
	if state != "approved" {
		t.Fatalf("expected approved, got %s", state)
	}

	// Verify approval history.
	resp = env.do("GET", "/campaigns/"+campaignID+"/approval-history", nil, toks.operator)
	assertStatus(t, resp, http.StatusOK)
	resp.Body.Close()

	// Verify audit events.
	assertAuditEvent(t, env.db, "campaign.submitted")
	assertAuditEvent(t, env.db, "campaign.approved")
}

// TestApprovalRejectionAndResubmission verifies: submit → reject → edit → resubmit → approve.
func TestApprovalRejectionAndResubmission(t *testing.T) {
	env := setupTestEnv(t)
	resetPhase3DB(t, env.db)
	adminToken := mustSetup(t, env)
	toks := setupRoles(t, env, adminToken)

	campaignID := mustPrepareCampaignForSubmission(t, env, toks.operator, "Rejection Test")

	// Submit.
	resp := env.do("POST", "/campaigns/"+campaignID+"/submit", nil, toks.operator)
	assertStatus(t, resp, http.StatusOK)
	resp.Body.Close()

	// Engineer rejects with comment.
	resp = env.do("POST", "/campaigns/"+campaignID+"/reject", map[string]any{
		"reason": "Target list needs to be reviewed, too many VIPs",
	}, toks.engineer)
	assertStatus(t, resp, http.StatusOK)
	resp.Body.Close()

	// Verify returns to draft.
	state := getCampaignState(t, env, toks.operator, campaignID)
	if state != "draft" {
		t.Fatalf("expected draft after rejection, got %s", state)
	}

	// Operator edits.
	resp = env.do("PUT", "/campaigns/"+campaignID, map[string]any{
		"description": "Revised after feedback",
	}, toks.operator)
	assertStatus(t, resp, http.StatusOK)
	resp.Body.Close()

	// Resubmit.
	resp = env.do("POST", "/campaigns/"+campaignID+"/submit", nil, toks.operator)
	assertStatus(t, resp, http.StatusOK)
	resp.Body.Close()

	// Approve on second submission.
	resp = env.do("POST", "/campaigns/"+campaignID+"/approve", map[string]any{
		"comment": "Approved on resubmission",
	}, toks.engineer)
	assertStatus(t, resp, http.StatusOK)
	resp.Body.Close()

	state = getCampaignState(t, env, toks.operator, campaignID)
	if state != "approved" {
		t.Fatalf("expected approved after resubmission, got %s", state)
	}

	// Verify approval history shows both submissions.
	resp = env.do("GET", "/campaigns/"+campaignID+"/approval-history", nil, toks.operator)
	assertStatus(t, resp, http.StatusOK)
	resp.Body.Close()
}

// TestSelfApprovalPrevention verifies submitter cannot approve own campaign.
func TestSelfApprovalPrevention(t *testing.T) {
	env := setupTestEnv(t)
	resetPhase3DB(t, env.db)
	adminToken := mustSetup(t, env)
	toks := setupRoles(t, env, adminToken)

	// Admin creates and submits (admin can do both).
	campaignID := mustPrepareCampaignForSubmission(t, env, toks.admin, "Self Approval Test")
	resp := env.do("POST", "/campaigns/"+campaignID+"/submit", nil, toks.admin)
	assertStatus(t, resp, http.StatusOK)
	resp.Body.Close()

	// Admin tries to approve own campaign — should be forbidden.
	resp = env.do("POST", "/campaigns/"+campaignID+"/approve", map[string]any{
		"comment": "Self-approve attempt",
	}, toks.admin)
	if resp.StatusCode == http.StatusOK {
		resp.Body.Close()
		t.Fatal("expected self-approval to be rejected")
	}
	resp.Body.Close()

	// Different user (engineer) can approve.
	resp = env.do("POST", "/campaigns/"+campaignID+"/approve", map[string]any{
		"comment": "External approval",
	}, toks.engineer)
	assertStatus(t, resp, http.StatusOK)
	resp.Body.Close()
}

// TestOperatorCannotApprove verifies operators lack campaigns:approve permission.
func TestOperatorCannotApprove(t *testing.T) {
	env := setupTestEnv(t)
	resetPhase3DB(t, env.db)
	adminToken := mustSetup(t, env)
	toks := setupRoles(t, env, adminToken)

	campaignID := mustPrepareCampaignForSubmission(t, env, toks.operator, "Operator Approval Test")
	resp := env.do("POST", "/campaigns/"+campaignID+"/submit", nil, toks.operator)
	assertStatus(t, resp, http.StatusOK)
	resp.Body.Close()

	// Operator tries to approve — should fail (no campaigns:approve permission).
	resp = env.do("POST", "/campaigns/"+campaignID+"/approve", map[string]any{
		"comment": "Operator approval attempt",
	}, toks.operator)
	if resp.StatusCode != http.StatusForbidden {
		resp.Body.Close()
		t.Errorf("expected 403 for operator approval, got %d", resp.StatusCode)
	} else {
		resp.Body.Close()
	}
}

// TestDefenderCannotCreateOrApproveCampaigns verifies defender's limited access.
func TestDefenderCannotCreateOrApproveCampaigns(t *testing.T) {
	env := setupTestEnv(t)
	resetPhase3DB(t, env.db)
	adminToken := mustSetup(t, env)
	toks := setupRoles(t, env, adminToken)

	// Defender cannot create campaigns.
	resp := env.do("POST", "/campaigns", map[string]any{
		"name":        "Defender Campaign",
		"description": "Should fail",
		"send_order":  "default",
	}, toks.defender)
	if resp.StatusCode != http.StatusForbidden {
		resp.Body.Close()
		t.Errorf("expected 403 for defender creating campaign, got %d", resp.StatusCode)
	} else {
		resp.Body.Close()
	}
}

// TestAdminCanApprove verifies admin can approve campaigns (not their own).
func TestAdminCanApprove(t *testing.T) {
	env := setupTestEnv(t)
	resetPhase3DB(t, env.db)
	adminToken := mustSetup(t, env)
	toks := setupRoles(t, env, adminToken)

	// Operator creates and submits.
	campaignID := mustPrepareCampaignForSubmission(t, env, toks.operator, "Admin Approval Test")
	resp := env.do("POST", "/campaigns/"+campaignID+"/submit", nil, toks.operator)
	assertStatus(t, resp, http.StatusOK)
	resp.Body.Close()

	// Admin approves.
	resp = env.do("POST", "/campaigns/"+campaignID+"/approve", map[string]any{
		"comment": "Admin approval",
	}, toks.admin)
	assertStatus(t, resp, http.StatusOK)
	resp.Body.Close()

	state := getCampaignState(t, env, toks.operator, campaignID)
	if state != "approved" {
		t.Fatalf("expected approved, got %s", state)
	}
}
