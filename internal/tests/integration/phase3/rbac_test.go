//go:build integration

package phase3

import (
	"net/http"
	"testing"
)

// TestCrossRoleTargetAccess verifies RBAC for target endpoints across all roles.
func TestCrossRoleTargetAccess(t *testing.T) {
	env := setupTestEnv(t)
	resetPhase3DB(t, env.db)
	adminToken := mustSetup(t, env)
	toks := setupRoles(t, env, adminToken)

	// Admin creates a target for testing read access.
	targetID := mustCreateTarget(t, env, toks.admin, "rbac-target@example.com", "RBAC", "Target", "IT")

	tests := []struct {
		name     string
		method   string
		path     string
		body     any
		token    string
		wantCode int
	}{
		// Admin — full access.
		{"admin read targets", "GET", "/targets", nil, toks.admin, http.StatusOK},
		{"admin create target", "POST", "/targets", map[string]any{"email": "admin-new@example.com", "first_name": "New"}, toks.admin, http.StatusCreated},
		{"admin read single", "GET", "/targets/" + targetID, nil, toks.admin, http.StatusOK},

		// Engineer — read only for targets.
		{"engineer read targets", "GET", "/targets", nil, toks.engineer, http.StatusOK},
		{"engineer read single", "GET", "/targets/" + targetID, nil, toks.engineer, http.StatusOK},

		// Operator — full CRUD for targets.
		{"operator read targets", "GET", "/targets", nil, toks.operator, http.StatusOK},
		{"operator create target", "POST", "/targets", map[string]any{"email": "op-new@example.com", "first_name": "Op"}, toks.operator, http.StatusCreated},

		// Defender — no target access.
		{"defender read targets", "GET", "/targets", nil, toks.defender, http.StatusForbidden},
		{"defender read single", "GET", "/targets/" + targetID, nil, toks.defender, http.StatusForbidden},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			resp := env.do(tc.method, tc.path, tc.body, tc.token)
			if resp.StatusCode != tc.wantCode {
				body := mustBody(resp)
				t.Errorf("expected %d, got %d: %s", tc.wantCode, resp.StatusCode, body)
			} else {
				resp.Body.Close()
			}
		})
	}
}

// TestCrossRoleCampaignAccess verifies RBAC for campaign endpoints across all roles.
func TestCrossRoleCampaignAccess(t *testing.T) {
	env := setupTestEnv(t)
	resetPhase3DB(t, env.db)
	adminToken := mustSetup(t, env)
	toks := setupRoles(t, env, adminToken)

	// Operator creates campaign for read tests.
	campaignID := mustCreateCampaign(t, env, toks.operator, "RBAC Campaign Test")

	tests := []struct {
		name     string
		method   string
		path     string
		body     any
		token    string
		wantCode int
	}{
		// Admin — full access.
		{"admin read campaigns", "GET", "/campaigns", nil, toks.admin, http.StatusOK},
		{"admin read single", "GET", "/campaigns/" + campaignID, nil, toks.admin, http.StatusOK},

		// Engineer — read only (campaigns:read but not campaigns:create).
		{"engineer read campaigns", "GET", "/campaigns", nil, toks.engineer, http.StatusOK},
		{"engineer read single", "GET", "/campaigns/" + campaignID, nil, toks.engineer, http.StatusOK},
		{"engineer create", "POST", "/campaigns", map[string]any{"name": "Eng Campaign", "send_order": "default"}, toks.engineer, http.StatusForbidden},

		// Operator — create and manage.
		{"operator read campaigns", "GET", "/campaigns", nil, toks.operator, http.StatusOK},
		{"operator create", "POST", "/campaigns", map[string]any{"name": "Op Campaign", "send_order": "default"}, toks.operator, http.StatusCreated},

		// Defender — no campaign access (only metrics:read).
		{"defender read campaigns", "GET", "/campaigns", nil, toks.defender, http.StatusForbidden},
		{"defender create", "POST", "/campaigns", map[string]any{"name": "Def Campaign", "send_order": "default"}, toks.defender, http.StatusForbidden},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			resp := env.do(tc.method, tc.path, tc.body, tc.token)
			if resp.StatusCode != tc.wantCode {
				body := mustBody(resp)
				t.Errorf("expected %d, got %d: %s", tc.wantCode, resp.StatusCode, body)
			} else {
				resp.Body.Close()
			}
		})
	}
}

// TestCrossRoleCredentialAccess verifies RBAC tiers for credential endpoints.
func TestCrossRoleCredentialAccess(t *testing.T) {
	env := setupTestEnv(t)
	resetPhase3DB(t, env.db)
	adminToken := mustSetup(t, env)
	toks := setupRoles(t, env, adminToken)

	tests := []struct {
		name     string
		method   string
		path     string
		body     any
		token    string
		wantCode int
	}{
		// Admin — full credential access.
		{"admin list captures", "GET", "/captures", nil, toks.admin, http.StatusOK},

		// Engineer — read + reveal.
		{"engineer list captures", "GET", "/captures", nil, toks.engineer, http.StatusOK},

		// Operator — read only (no reveal).
		{"operator list captures", "GET", "/captures", nil, toks.operator, http.StatusOK},

		// Defender — no credential access.
		{"defender list captures", "GET", "/captures", nil, toks.defender, http.StatusForbidden},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			resp := env.do(tc.method, tc.path, tc.body, tc.token)
			if resp.StatusCode != tc.wantCode {
				body := mustBody(resp)
				t.Errorf("expected %d, got %d: %s", tc.wantCode, resp.StatusCode, body)
			} else {
				resp.Body.Close()
			}
		})
	}
}

// TestCrossRoleApprovalAccess verifies approval endpoint RBAC.
func TestCrossRoleApprovalAccess(t *testing.T) {
	env := setupTestEnv(t)
	resetPhase3DB(t, env.db)
	adminToken := mustSetup(t, env)
	toks := setupRoles(t, env, adminToken)

	// Create campaign in pending_approval for approval tests.
	campaignID := mustPrepareCampaignForSubmission(t, env, toks.operator, "Approval RBAC Test")
	resp := env.do("POST", "/campaigns/"+campaignID+"/submit", nil, toks.operator)
	assertStatus(t, resp, http.StatusOK)
	resp.Body.Close()

	// Operator cannot approve (no campaigns:approve).
	resp = env.do("POST", "/campaigns/"+campaignID+"/approve", map[string]any{"comment": "test"}, toks.operator)
	if resp.StatusCode != http.StatusForbidden {
		resp.Body.Close()
		t.Errorf("expected 403 for operator approve, got %d", resp.StatusCode)
	} else {
		resp.Body.Close()
	}

	// Defender cannot approve.
	resp = env.do("POST", "/campaigns/"+campaignID+"/approve", map[string]any{"comment": "test"}, toks.defender)
	if resp.StatusCode != http.StatusForbidden {
		resp.Body.Close()
		t.Errorf("expected 403 for defender approve, got %d", resp.StatusCode)
	} else {
		resp.Body.Close()
	}

	// Engineer can approve.
	resp = env.do("POST", "/campaigns/"+campaignID+"/approve", map[string]any{"comment": "engineer approved"}, toks.engineer)
	assertStatus(t, resp, http.StatusOK)
	resp.Body.Close()
}

// TestUnauthenticatedAccessRejected verifies all Phase 3 endpoints reject unauthenticated requests.
func TestUnauthenticatedAccessRejected(t *testing.T) {
	env := setupTestEnv(t)
	resetPhase3DB(t, env.db)
	_ = mustSetup(t, env)

	endpoints := []struct {
		method string
		path   string
	}{
		{"GET", "/targets"},
		{"POST", "/targets"},
		{"GET", "/target-groups"},
		{"GET", "/blocklist"},
		{"GET", "/campaigns"},
		{"POST", "/campaigns"},
		{"GET", "/captures"},
		{"GET", "/endpoints"},
		{"GET", "/landing-pages"},
		{"GET", "/campaign-templates"},
	}

	for _, ep := range endpoints {
		t.Run(ep.method+" "+ep.path, func(t *testing.T) {
			resp := env.do(ep.method, ep.path, nil, "")
			if resp.StatusCode != http.StatusUnauthorized {
				resp.Body.Close()
				t.Errorf("expected 401, got %d", resp.StatusCode)
			} else {
				resp.Body.Close()
			}
		})
	}
}
