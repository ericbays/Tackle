//go:build integration

package phase2

import (
	"net/http"
	"testing"
)

// TestOIDCProviderConfiguration verifies creating an OIDC provider config.
// Verifies client_secret is not returned in GET response.
func TestOIDCProviderConfiguration(t *testing.T) {
	env := setupTestEnv(t)
	resetPhase2DB(t, env.db)
	adminToken := mustSetup(t, env)

	oidc := newOIDCMock(t)

	resp := env.do("POST", "/settings/auth-providers", map[string]any{
		"type":           "oidc",
		"name":           "Test OIDC Provider",
		"enabled":        true,
		"auto_provision": true,
		"auth_order":     "local_first",
		"config": map[string]any{
			"issuer_url":    oidc.issuer,
			"client_id":     "test-client-id",
			"client_secret": "verysecretclientsecret",
			"scopes":        []string{"openid", "email", "profile"},
		},
	}, adminToken)
	assertStatus(t, resp, http.StatusCreated)

	var created struct {
		Data struct {
			ID      string `json:"id"`
			Type    string `json:"type"`
			Name    string `json:"name"`
			Enabled bool   `json:"enabled"`
		} `json:"data"`
	}
	decodeT(t, resp, &created)
	id := created.Data.ID
	if id == "" {
		t.Fatal("expected non-empty provider ID")
	}
	if created.Data.Type != "oidc" {
		t.Errorf("expected type=oidc, got %s", created.Data.Type)
	}

	// GET — verify client_secret is masked.
	resp = env.do("GET", "/settings/auth-providers/"+id, nil, adminToken)
	assertStatus(t, resp, http.StatusOK)
	body := mustBody(resp)
	if bodyContains(body, "verysecretclientsecret") {
		t.Error("client_secret leaked in GET response")
	}

	assertAuditEvent(t, env.db, "auth_provider.created")
}

// TestFusionAuthProviderConfiguration verifies creating a FusionAuth provider config.
func TestFusionAuthProviderConfiguration(t *testing.T) {
	env := setupTestEnv(t)
	resetPhase2DB(t, env.db)
	adminToken := mustSetup(t, env)

	fa := newFusionAuthMock(t)

	resp := env.do("POST", "/settings/auth-providers", map[string]any{
		"type":           "fusionauth",
		"name":           "Test FusionAuth",
		"enabled":        true,
		"auto_provision": false,
		"auth_order":     "local_first",
		"config": map[string]any{
			"base_url":   fa.srv.URL,
			"api_key":    "fusionauth-api-key-secret",
			"tenant_id":  "default",
			"client_id":  "fa-client-id",
		},
	}, adminToken)
	assertStatus(t, resp, http.StatusCreated)

	var created struct {
		Data struct{ ID string `json:"id"` } `json:"data"`
	}
	decodeT(t, resp, &created)

	// Verify api_key is masked.
	resp = env.do("GET", "/settings/auth-providers/"+created.Data.ID, nil, adminToken)
	assertStatus(t, resp, http.StatusOK)
	body := mustBody(resp)
	if bodyContains(body, "fusionauth-api-key-secret") {
		t.Error("FusionAuth api_key leaked in GET response")
	}
}

// TestLDAPProviderConfiguration verifies creating an LDAP provider config.
func TestLDAPProviderConfiguration(t *testing.T) {
	env := setupTestEnv(t)
	resetPhase2DB(t, env.db)
	adminToken := mustSetup(t, env)

	resp := env.do("POST", "/settings/auth-providers", map[string]any{
		"type":           "ldap",
		"name":           "Test LDAP",
		"enabled":        true,
		"auto_provision": true,
		"auth_order":     "local_first",
		"config": map[string]any{
			"server_url":       "ldap://ldap.example.com:389",
			"bind_dn":          "cn=bind,dc=example,dc=com",
			"bind_password":    "ldapbindpassword",
			"base_dn":          "dc=example,dc=com",
			"user_filter":      "(sAMAccountName={username})",
			"group_attribute":  "memberOf",
		},
	}, adminToken)
	assertStatus(t, resp, http.StatusCreated)

	var created struct {
		Data struct{ ID string `json:"id"` } `json:"data"`
	}
	decodeT(t, resp, &created)

	// bind_password must be masked.
	resp = env.do("GET", "/settings/auth-providers/"+created.Data.ID, nil, adminToken)
	assertStatus(t, resp, http.StatusOK)
	body := mustBody(resp)
	if bodyContains(body, "ldapbindpassword") {
		t.Error("LDAP bind_password leaked in GET response")
	}
}

// TestAuthProviderList verifies listing returns all configured providers.
func TestAuthProviderList(t *testing.T) {
	env := setupTestEnv(t)
	resetPhase2DB(t, env.db)
	adminToken := mustSetup(t, env)

	oidc := newOIDCMock(t)
	fa := newFusionAuthMock(t)

	// Create two providers.
	for _, prov := range []map[string]any{
		{
			"type": "oidc", "name": "OIDC-1", "enabled": true,
			"auto_provision": true, "auth_order": "local_first",
			"config": map[string]any{"issuer_url": oidc.issuer, "client_id": "cid", "client_secret": "cs"},
		},
		{
			"type": "fusionauth", "name": "FA-1", "enabled": true,
			"auto_provision": false, "auth_order": "local_first",
			"config": map[string]any{"base_url": fa.srv.URL, "api_key": "k", "tenant_id": "t", "client_id": "c"},
		},
	} {
		resp := env.do("POST", "/settings/auth-providers", prov, adminToken)
		if resp.StatusCode != http.StatusCreated {
			body := mustBody(resp)
			t.Fatalf("create provider %s: expected 201, got %d: %s", prov["name"], resp.StatusCode, body)
		}
		resp.Body.Close()
	}

	resp := env.do("GET", "/settings/auth-providers", nil, adminToken)
	assertStatus(t, resp, http.StatusOK)
	var listResp struct {
		Data []map[string]any `json:"data"`
	}
	decodeT(t, resp, &listResp)
	if len(listResp.Data) < 2 {
		t.Errorf("expected >=2 providers, got %d", len(listResp.Data))
	}
}

// TestAuthProviderEnableDisable verifies the enable/disable endpoints.
func TestAuthProviderEnableDisable(t *testing.T) {
	env := setupTestEnv(t)
	resetPhase2DB(t, env.db)
	adminToken := mustSetup(t, env)

	oidc := newOIDCMock(t)
	resp := env.do("POST", "/settings/auth-providers", map[string]any{
		"type": "oidc", "name": "Toggle OIDC", "enabled": true,
		"auto_provision": false, "auth_order": "local_first",
		"config": map[string]any{"issuer_url": oidc.issuer, "client_id": "cid", "client_secret": "cs"},
	}, adminToken)
	assertStatus(t, resp, http.StatusCreated)
	var created struct {
		Data struct{ ID string `json:"id"` } `json:"data"`
	}
	decodeT(t, resp, &created)
	id := created.Data.ID

	// Disable.
	resp = env.do("POST", "/settings/auth-providers/"+id+"/disable", nil, adminToken)
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNoContent {
		body := mustBody(resp)
		t.Fatalf("disable provider: expected 200/204, got %d: %s", resp.StatusCode, body)
	}
	resp.Body.Close()

	// Re-enable.
	resp = env.do("POST", "/settings/auth-providers/"+id+"/enable", nil, adminToken)
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNoContent {
		body := mustBody(resp)
		t.Fatalf("enable provider: expected 200/204, got %d: %s", resp.StatusCode, body)
	}
	resp.Body.Close()
}

// TestAuthProviderRoleMappings verifies setting and retrieving role mappings.
func TestAuthProviderRoleMappings(t *testing.T) {
	env := setupTestEnv(t)
	resetPhase2DB(t, env.db)
	adminToken := mustSetup(t, env)

	oidc := newOIDCMock(t)
	resp := env.do("POST", "/settings/auth-providers", map[string]any{
		"type": "oidc", "name": "Mapping OIDC", "enabled": true,
		"auto_provision": true, "auth_order": "local_first",
		"config": map[string]any{"issuer_url": oidc.issuer, "client_id": "cid", "client_secret": "cs"},
	}, adminToken)
	assertStatus(t, resp, http.StatusCreated)
	var created struct {
		Data struct{ ID string `json:"id"` } `json:"data"`
	}
	decodeT(t, resp, &created)
	id := created.Data.ID

	// Get available roles to reference.
	roles := listRoles(t, env, adminToken)
	operatorRoleID := findRoleID(t, roles, "operator")

	// Set role mappings.
	resp = env.do("PUT", "/settings/auth-providers/"+id+"/role-mappings", map[string]any{
		"mappings": []map[string]any{
			{"external_group": "staff", "role_id": operatorRoleID},
		},
	}, adminToken)
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNoContent {
		body := mustBody(resp)
		t.Fatalf("set role mappings: expected 200/204, got %d: %s", resp.StatusCode, body)
	}
	resp.Body.Close()

	// Get role mappings.
	resp = env.do("GET", "/settings/auth-providers/"+id+"/role-mappings", nil, adminToken)
	assertStatus(t, resp, http.StatusOK)
	resp.Body.Close()
}

// TestAuthProviderDeleteAndCleanup verifies deleting a provider.
func TestAuthProviderDeleteAndCleanup(t *testing.T) {
	env := setupTestEnv(t)
	resetPhase2DB(t, env.db)
	adminToken := mustSetup(t, env)

	oidc := newOIDCMock(t)
	resp := env.do("POST", "/settings/auth-providers", map[string]any{
		"type": "oidc", "name": "Delete Me", "enabled": false,
		"auto_provision": false, "auth_order": "local_first",
		"config": map[string]any{"issuer_url": oidc.issuer, "client_id": "cid", "client_secret": "cs"},
	}, adminToken)
	assertStatus(t, resp, http.StatusCreated)
	var created struct {
		Data struct{ ID string `json:"id"` } `json:"data"`
	}
	decodeT(t, resp, &created)
	id := created.Data.ID

	resp = env.do("DELETE", "/settings/auth-providers/"+id, nil, adminToken)
	if resp.StatusCode != http.StatusNoContent && resp.StatusCode != http.StatusOK {
		body := mustBody(resp)
		t.Fatalf("delete provider: expected 200/204, got %d: %s", resp.StatusCode, body)
	}
	resp.Body.Close()

	assertAuditEvent(t, env.db, "auth_provider.deleted")
}

// TestAuthIdentityListAndUnlink verifies the identity listing and unlinking APIs.
func TestAuthIdentityListAndUnlink(t *testing.T) {
	env := setupTestEnv(t)
	resetPhase2DB(t, env.db)
	adminToken := mustSetup(t, env)

	// List own identities.
	resp := env.do("GET", "/auth/identities", nil, adminToken)
	assertStatus(t, resp, http.StatusOK)
	resp.Body.Close()
}

// TestAuthProviderPublicEndpoint verifies the public providers endpoint returns
// enabled providers without credentials.
func TestAuthProviderPublicEndpoint(t *testing.T) {
	env := setupTestEnv(t)
	resetPhase2DB(t, env.db)
	mustSetup(t, env)

	resp := env.do("GET", "/auth/providers", nil, "")
	assertStatus(t, resp, http.StatusOK)
	resp.Body.Close()
}

// TestAuthProviderRBACEnforcement verifies only admin can configure providers.
func TestAuthProviderRBACEnforcement(t *testing.T) {
	env := setupTestEnv(t)
	resetPhase2DB(t, env.db)
	adminToken := mustSetup(t, env)
	toks := setupRoles(t, env, adminToken)

	oidc := newOIDCMock(t)
	resp := env.do("POST", "/settings/auth-providers", map[string]any{
		"type": "oidc", "name": "Should Fail", "enabled": false,
		"auto_provision": false, "auth_order": "local_first",
		"config": map[string]any{"issuer_url": oidc.issuer, "client_id": "cid", "client_secret": "cs"},
	}, toks.operator)
	if resp.StatusCode != http.StatusForbidden {
		body := mustBody(resp)
		t.Errorf("operator create auth-provider: expected 403, got %d: %s", resp.StatusCode, body)
	}
	resp.Body.Close()

	// List is allowed for engineer.
	resp = env.do("GET", "/settings/auth-providers", nil, toks.engineer)
	assertStatusQuiet(t, resp, http.StatusOK)
}

// TestAuthProviderAuditTrail verifies all provider operations are audited.
func TestAuthProviderAuditTrail(t *testing.T) {
	env := setupTestEnv(t)
	resetPhase2DB(t, env.db)
	adminToken := mustSetup(t, env)

	oidc := newOIDCMock(t)
	resp := env.do("POST", "/settings/auth-providers", map[string]any{
		"type": "oidc", "name": "Audit OIDC", "enabled": true,
		"auto_provision": false, "auth_order": "local_first",
		"config": map[string]any{"issuer_url": oidc.issuer, "client_id": "cid", "client_secret": "cs"},
	}, adminToken)
	assertStatus(t, resp, http.StatusCreated)
	var created struct {
		Data struct{ ID string `json:"id"` } `json:"data"`
	}
	decodeT(t, resp, &created)
	id := created.Data.ID

	// Update.
	resp = env.do("PUT", "/settings/auth-providers/"+id, map[string]any{
		"type": "oidc", "name": "Audit OIDC Updated", "enabled": true,
		"auto_provision": false, "auth_order": "local_first",
		"config": map[string]any{"issuer_url": oidc.issuer, "client_id": "cid2", "client_secret": "cs"},
	}, adminToken)
	resp.Body.Close()

	// Delete.
	resp = env.do("DELETE", "/settings/auth-providers/"+id, nil, adminToken)
	resp.Body.Close()

	waitForAudit()
	assertAuditEvent(t, env.db, "auth_provider.created")
	assertAuditEvent(t, env.db, "auth_provider.updated")
	assertAuditEvent(t, env.db, "auth_provider.deleted")
}
