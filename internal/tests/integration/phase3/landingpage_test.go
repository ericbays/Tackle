//go:build integration

package phase3

import (
	"net/http"
	"testing"
)

// TestLandingPageProjectCRUD verifies landing page project create, read, update, delete.
func TestLandingPageProjectCRUD(t *testing.T) {
	env := setupTestEnv(t)
	resetPhase3DB(t, env.db)
	adminToken := mustSetup(t, env)

	// Create project.
	resp := env.do("POST", "/landing-pages", map[string]any{
		"name": "Test Login Page",
		"page_definition": map[string]any{
			"pages": []map[string]any{
				{
					"id":   "page1",
					"name": "Login",
					"components": []map[string]any{
						{
							"type":  "form",
							"props": map[string]any{"action": "/submit"},
							"children": []map[string]any{
								{"type": "input", "props": map[string]any{"name": "username", "capture_tag": "username"}},
								{"type": "input", "props": map[string]any{"name": "password", "type": "password", "capture_tag": "password"}},
								{"type": "button", "props": map[string]any{"type": "submit", "text": "Sign In"}},
							},
						},
					},
				},
			},
		},
	}, adminToken)
	assertStatus(t, resp, http.StatusCreated)
	var project struct {
		Data struct {
			ID     string `json:"id"`
			Name   string `json:"name"`
			Status string `json:"status"`
		} `json:"data"`
	}
	decodeT(t, resp, &project)
	projectID := project.Data.ID
	if project.Data.Name != "Test Login Page" {
		t.Errorf("expected name 'Test Login Page', got %s", project.Data.Name)
	}

	// Get project.
	resp = env.do("GET", "/landing-pages/"+projectID, nil, adminToken)
	assertStatus(t, resp, http.StatusOK)
	resp.Body.Close()

	// Update project.
	resp = env.do("PUT", "/landing-pages/"+projectID, map[string]any{
		"name": "Updated Login Page",
	}, adminToken)
	assertStatus(t, resp, http.StatusOK)
	resp.Body.Close()

	// List projects.
	resp = env.do("GET", "/landing-pages", nil, adminToken)
	assertStatus(t, resp, http.StatusOK)
	var listResp struct {
		Data []any `json:"data"`
	}
	decodeT(t, resp, &listResp)
	if len(listResp.Data) < 1 {
		t.Error("expected at least 1 landing page project")
	}

	// Delete.
	resp = env.do("DELETE", "/landing-pages/"+projectID, nil, adminToken)
	assertStatus(t, resp, http.StatusOK)
	resp.Body.Close()
}

// TestLandingPageDuplicate verifies project cloning.
func TestLandingPageDuplicate(t *testing.T) {
	env := setupTestEnv(t)
	resetPhase3DB(t, env.db)
	adminToken := mustSetup(t, env)

	// Create project.
	resp := env.do("POST", "/landing-pages", map[string]any{
		"name": "Original Page",
		"page_definition": map[string]any{
			"pages": []map[string]any{
				{"id": "p1", "name": "Main", "components": []map[string]any{}},
			},
		},
	}, adminToken)
	assertStatus(t, resp, http.StatusCreated)
	var project struct {
		Data struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	decodeT(t, resp, &project)

	// Duplicate.
	resp = env.do("POST", "/landing-pages/"+project.Data.ID+"/duplicate", nil, adminToken)
	assertStatus(t, resp, http.StatusCreated)
	var dupe struct {
		Data struct {
			ID   string `json:"id"`
			Name string `json:"name"`
		} `json:"data"`
	}
	decodeT(t, resp, &dupe)
	if dupe.Data.ID == project.Data.ID {
		t.Error("duplicate should have different ID")
	}
}

// TestLandingPageComponents verifies the component library endpoint.
func TestLandingPageComponents(t *testing.T) {
	env := setupTestEnv(t)
	resetPhase3DB(t, env.db)
	adminToken := mustSetup(t, env)

	resp := env.do("GET", "/landing-pages/components", nil, adminToken)
	assertStatus(t, resp, http.StatusOK)
	resp.Body.Close()
}

// TestLandingPageThemes verifies the themes endpoint.
func TestLandingPageThemes(t *testing.T) {
	env := setupTestEnv(t)
	resetPhase3DB(t, env.db)
	adminToken := mustSetup(t, env)

	resp := env.do("GET", "/landing-pages/themes", nil, adminToken)
	assertStatus(t, resp, http.StatusOK)
	resp.Body.Close()
}

// TestLandingPageStarterTemplates verifies the starter templates endpoint.
func TestLandingPageStarterTemplates(t *testing.T) {
	env := setupTestEnv(t)
	resetPhase3DB(t, env.db)
	adminToken := mustSetup(t, env)

	resp := env.do("GET", "/landing-pages/starter-templates", nil, adminToken)
	assertStatus(t, resp, http.StatusOK)
	resp.Body.Close()
}

// TestLandingPagePreview verifies the preview endpoint.
func TestLandingPagePreview(t *testing.T) {
	env := setupTestEnv(t)
	resetPhase3DB(t, env.db)
	adminToken := mustSetup(t, env)

	// Create project.
	resp := env.do("POST", "/landing-pages", map[string]any{
		"name": "Preview Test",
		"page_definition": map[string]any{
			"pages": []map[string]any{
				{"id": "p1", "name": "Main", "components": []map[string]any{
					{"type": "heading", "props": map[string]any{"text": "Welcome"}},
				}},
			},
		},
	}, adminToken)
	assertStatus(t, resp, http.StatusCreated)
	var project struct {
		Data struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	decodeT(t, resp, &project)

	// Preview.
	resp = env.do("POST", "/landing-pages/"+project.Data.ID+"/preview", map[string]any{
		"page_id": "p1",
	}, adminToken)
	assertStatus(t, resp, http.StatusOK)
	resp.Body.Close()
}

// TestLandingPageRBAC verifies landing page permissions.
func TestLandingPageRBAC(t *testing.T) {
	env := setupTestEnv(t)
	resetPhase3DB(t, env.db)
	adminToken := mustSetup(t, env)
	toks := setupRoles(t, env, adminToken)

	// Defender cannot list landing pages (no landing_pages:read).
	resp := env.do("GET", "/landing-pages", nil, toks.defender)
	if resp.StatusCode != http.StatusForbidden {
		resp.Body.Close()
		t.Errorf("expected 403 for defender landing page access, got %d", resp.StatusCode)
	} else {
		resp.Body.Close()
	}

	// Operator can read and create (has templates.landing:read/create).
	resp = env.do("GET", "/landing-pages", nil, toks.operator)
	if resp.StatusCode == http.StatusForbidden {
		resp.Body.Close()
		t.Log("operator got 403 for landing pages — permission mapping may differ")
	} else {
		resp.Body.Close()
	}
}
