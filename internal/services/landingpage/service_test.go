package landingpage

import (
	"testing"
)

func TestDefaultDefinition(t *testing.T) {
	def := defaultDefinition()
	if def == nil {
		t.Fatal("expected non-nil default definition")
	}

	// Validate the default definition is valid.
	if err := ValidateDefinition(def); err != nil {
		t.Fatalf("default definition should be valid, got: %v", err)
	}

	// Check schema version.
	sv, ok := def["schema_version"]
	if !ok {
		t.Error("expected schema_version in default definition")
	}
	if sv != 1 {
		t.Errorf("expected schema_version 1, got %v", sv)
	}

	// Check pages.
	pages, ok := def["pages"].([]any)
	if !ok || len(pages) != 1 {
		t.Fatal("expected exactly 1 page in default definition")
	}

	page, ok := pages[0].(map[string]any)
	if !ok {
		t.Fatal("expected page to be a map")
	}
	if page["route"] != "/" {
		t.Errorf("expected default route '/', got %v", page["route"])
	}
}

func TestGetStarterTemplates(t *testing.T) {
	templates := GetStarterTemplates()
	if len(templates) != 5 {
		t.Fatalf("expected 5 starter templates, got %d", len(templates))
	}

	expectedNames := []string{
		"Login → Loading → Success",
		"Login → MFA → Dashboard",
		"SSO Login → Consent → Redirect",
		"File Share Login → Download Page",
		"Password Reset → Confirmation",
	}

	for i, tmpl := range templates {
		if tmpl.Name != expectedNames[i] {
			t.Errorf("template %d: expected name %q, got %q", i, expectedNames[i], tmpl.Name)
		}
		if tmpl.Category != "starter" {
			t.Errorf("template %d: expected category 'starter', got %q", i, tmpl.Category)
		}

		// Validate each template's definition.
		if err := ValidateDefinition(tmpl.Definition); err != nil {
			t.Errorf("template %q: invalid definition: %v", tmpl.Name, err)
		}
	}
}

func TestGetBuiltInThemes(t *testing.T) {
	themes := GetBuiltInThemes()
	if len(themes) != 5 {
		t.Fatalf("expected 5 built-in themes, got %d", len(themes))
	}

	expectedIDs := []string{
		"theme-microsoft",
		"theme-google",
		"theme-corporate",
		"theme-cloud",
		"theme-banking",
	}

	for i, theme := range themes {
		if theme.ID != expectedIDs[i] {
			t.Errorf("theme %d: expected ID %q, got %q", i, expectedIDs[i], theme.ID)
		}
		if theme.Category != "enterprise" {
			t.Errorf("theme %d: expected category 'enterprise', got %q", i, theme.Category)
		}
		if theme.Styles == nil {
			t.Errorf("theme %d: expected non-nil styles", i)
		}
		// Each theme should have a CSS field.
		if _, ok := theme.Styles["css"]; !ok {
			t.Errorf("theme %d %q: expected 'css' in styles", i, theme.Name)
		}
	}
}

func TestGetJSSnippetTemplates(t *testing.T) {
	snippets := GetJSSnippetTemplates()
	if len(snippets) != 7 {
		t.Fatalf("expected 7 JS snippets, got %d", len(snippets))
	}

	for _, s := range snippets {
		if s.ID == "" {
			t.Error("expected non-empty snippet ID")
		}
		if s.Name == "" {
			t.Error("expected non-empty snippet name")
		}
		if s.Code == "" {
			t.Error("expected non-empty snippet code")
		}
		if s.Category == "" {
			t.Error("expected non-empty snippet category")
		}
	}
}

func TestValidationErrorType(t *testing.T) {
	err := &ValidationError{Msg: "test error"}
	if err.Error() != "test error" {
		t.Errorf("expected 'test error', got %q", err.Error())
	}
}

func TestConflictErrorType(t *testing.T) {
	err := &ConflictError{Msg: "conflict"}
	if err.Error() != "conflict" {
		t.Errorf("expected 'conflict', got %q", err.Error())
	}
}

func TestNotFoundErrorType(t *testing.T) {
	err := &NotFoundError{Msg: "not found"}
	if err.Error() != "not found" {
		t.Errorf("expected 'not found', got %q", err.Error())
	}
}
