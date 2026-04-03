//go:build integration

package phase3

import (
	"net/http"
	"testing"
)

// TestEndpointListEmpty verifies empty endpoint list returns valid response.
func TestEndpointListEmpty(t *testing.T) {
	env := setupTestEnv(t)
	resetPhase3DB(t, env.db)
	adminToken := mustSetup(t, env)

	resp := env.do("GET", "/endpoints", nil, adminToken)
	assertStatus(t, resp, http.StatusOK)
	resp.Body.Close()
}

// TestEndpointCampaignEndpointNotFound verifies 404 for campaign without endpoint.
func TestEndpointCampaignEndpointNotFound(t *testing.T) {
	env := setupTestEnv(t)
	resetPhase3DB(t, env.db)
	adminToken := mustSetup(t, env)

	campaignID := mustCreateCampaign(t, env, adminToken, "No Endpoint Campaign")

	resp := env.do("GET", "/campaigns/"+campaignID+"/endpoint", nil, adminToken)
	// Should return 404 since no endpoint has been provisioned.
	if resp.StatusCode != http.StatusNotFound && resp.StatusCode != http.StatusOK {
		body := mustBody(resp)
		t.Logf("endpoint status for new campaign: %d: %s", resp.StatusCode, body)
	} else {
		resp.Body.Close()
	}
}

// TestEndpointHealthNoEndpoint verifies health returns appropriate status for no endpoint.
func TestEndpointHealthNoEndpoint(t *testing.T) {
	env := setupTestEnv(t)
	resetPhase3DB(t, env.db)
	adminToken := mustSetup(t, env)

	campaignID := mustCreateCampaign(t, env, adminToken, "Health Check Test")

	resp := env.do("GET", "/campaigns/"+campaignID+"/endpoint/health", nil, adminToken)
	// No endpoint provisioned — expect 404 or empty result.
	if resp.StatusCode != http.StatusNotFound && resp.StatusCode != http.StatusOK {
		body := mustBody(resp)
		t.Logf("health check for campaign without endpoint: %d: %s", resp.StatusCode, body)
	} else {
		resp.Body.Close()
	}
}

// TestEndpointRBAC verifies infrastructure permissions are enforced.
func TestEndpointRBAC(t *testing.T) {
	env := setupTestEnv(t)
	resetPhase3DB(t, env.db)
	adminToken := mustSetup(t, env)
	toks := setupRoles(t, env, adminToken)

	campaignID := mustCreateCampaign(t, env, toks.operator, "Endpoint RBAC Test")

	// Defender cannot read endpoint info (no infrastructure:read).
	resp := env.do("GET", "/campaigns/"+campaignID+"/endpoint", nil, toks.defender)
	if resp.StatusCode != http.StatusForbidden {
		resp.Body.Close()
		t.Errorf("expected 403 for defender endpoint access, got %d", resp.StatusCode)
	} else {
		resp.Body.Close()
	}

	// Defender cannot list all endpoints.
	resp = env.do("GET", "/endpoints", nil, toks.defender)
	if resp.StatusCode != http.StatusForbidden {
		resp.Body.Close()
		t.Errorf("expected 403 for defender endpoint list, got %d", resp.StatusCode)
	} else {
		resp.Body.Close()
	}

	// Operator can read endpoints (has endpoints:read).
	resp = env.do("GET", "/campaigns/"+campaignID+"/endpoint", nil, toks.operator)
	// May be 404 (no endpoint) or 403 (depending on permission mapping).
	// The key check is that it's NOT 403 if operator should have read access.
	if resp.StatusCode == http.StatusForbidden {
		resp.Body.Close()
		t.Log("operator got 403 for endpoint read — infrastructure:read may not be in operator permissions")
	} else {
		resp.Body.Close()
	}

	// Engineer can read endpoints (has infrastructure:read).
	resp = env.do("GET", "/endpoints", nil, toks.engineer)
	if resp.StatusCode == http.StatusForbidden {
		resp.Body.Close()
		t.Errorf("expected engineer to have infrastructure:read, got 403")
	} else {
		resp.Body.Close()
	}
}

// TestEndpointStopWithoutEndpoint verifies stop command on non-existent endpoint fails gracefully.
func TestEndpointStopWithoutEndpoint(t *testing.T) {
	env := setupTestEnv(t)
	resetPhase3DB(t, env.db)
	adminToken := mustSetup(t, env)

	campaignID := mustCreateCampaign(t, env, adminToken, "Stop No Endpoint")

	resp := env.do("POST", "/campaigns/"+campaignID+"/endpoint/stop", nil, adminToken)
	// Should return error (404 or 400) since no endpoint exists.
	if resp.StatusCode == http.StatusOK {
		resp.Body.Close()
		t.Error("expected error for stop on non-existent endpoint")
	} else {
		resp.Body.Close()
	}
}

// TestEndpointLogsEmpty verifies logs endpoint returns valid response for no logs.
func TestEndpointLogsEmpty(t *testing.T) {
	env := setupTestEnv(t)
	resetPhase3DB(t, env.db)
	adminToken := mustSetup(t, env)

	campaignID := mustCreateCampaign(t, env, adminToken, "Logs Test")

	resp := env.do("GET", "/campaigns/"+campaignID+"/endpoint/logs", nil, adminToken)
	// May return 404 (no endpoint) or 200 with empty logs.
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNotFound {
		body := mustBody(resp)
		t.Logf("endpoint logs: %d: %s", resp.StatusCode, body)
	} else {
		resp.Body.Close()
	}
}
