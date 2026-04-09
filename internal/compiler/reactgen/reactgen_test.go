package reactgen

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestReactgenScaffoldAndBundle(t *testing.T) {
	// 1. Test AST Transpilation
	mockAST := AST{
		CampaignType: "awareness",
		RootNode: Node{
			ID:   "root-0",
			Type: "root",
			Styles: map[string]string{
				"backgroundColor": "#121212",
				"minHeight":       "100vh",
			},
			Children: []Node{
				{
					ID:   "heading-1",
					Type: "heading",
					Properties: map[string]string{
						"content": "Welcome to Tackle",
						"level":   "1",
					},
					Styles: map[string]string{"color": "white"},
				},
				{
					ID:   "form-1",
					Type: "form",
					Properties: map[string]string{
						"actionRoute": "/api/v1/signin",
					},
					Children: []Node{
						{
							ID:   "input-1",
							Type: "input",
							Properties: map[string]string{
								"name":        "email",
								"placeholder": "Enter Email",
							},
						},
						{
							ID:   "button-1",
							Type: "button",
							Properties: map[string]string{
								"content": "Login",
							},
						},
					},
				},
			},
		},
	}

	payload, err := json.Marshal(mockAST)
	if err != nil {
		t.Fatalf("Failed to marshal mock AST: %v", err)
	}

	appJsx, err := Transpile(payload)
	if err != nil {
		t.Fatalf("Transpile failed: %v", err)
	}

	// Validate output structurally
	if !strings.Contains(appJsx, "<div id=\"root-0\"") {
		t.Errorf("Expected root div, got: %s", appJsx)
	}
	if !strings.Contains(appJsx, "<h1 id=\"heading-1\"") {
		t.Errorf("Expected heading h1, got: %s", appJsx)
	}
	if !strings.Contains(appJsx, "action=\"/api/v1/signin\"") {
		t.Errorf("Expected configurable form action, got: %s", appJsx)
	}

	// 2. Test Scaffold Creation
	ws, err := CreateWorkspace()
	if err != nil {
		t.Fatalf("CreateWorkspace failed: %v", err)
	}
	defer ws.Cleanup()

	// 3. Write basic files
	if err := ws.WriteIndex(); err != nil {
		t.Fatalf("WriteIndex failed: %v", err)
	}
	if err := ws.WriteFile("App.tsx", appJsx); err != nil {
		t.Fatalf("WriteFile App.tsx failed: %v", err)
	}

	// 4. Test Esbuild Compilation
	res, err := RunEsbuild(ws.DirPath)
	if err != nil {
		t.Fatalf("RunEsbuild failed: %v", err)
	}

	if len(res.JS) == 0 {
		t.Errorf("Expected javascript bundle, got empty payload")
	}
	
	// Print a snippet of the generated javascript to prove it transpiled the JSX successfully
	if !strings.Contains(string(res.JS), "Welcome to Tackle") {
		t.Errorf("Transpiled output did not contain expected content. Got: %s", string(res.JS)[:200])
	}
}
