package landingpage

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestParseHTMLToDefinition(t *testing.T) {
	tests := []struct {
		name    string
		html    string
		wantErr bool
		check   func(t *testing.T, def map[string]any)
	}{
		{
			name:    "empty HTML",
			html:    "",
			wantErr: true,
		},
		{
			name:    "whitespace only",
			html:    "   \n\t  ",
			wantErr: true,
		},
		{
			name: "simple HTML body",
			html: `<html><body><h1>Hello World</h1><p>Test paragraph</p></body></html>`,
			check: func(t *testing.T, def map[string]any) {
				pages, ok := def["pages"].([]any)
				if !ok || len(pages) == 0 {
					t.Fatal("expected pages")
				}
				page := pages[0].(map[string]any)
				tree, ok := page["component_tree"].([]any)
				if !ok || len(tree) == 0 {
					t.Fatal("expected component tree")
				}
				// Should have at least one raw_html component.
				comp := tree[0].(map[string]any)
				if comp["type"] != "raw_html" {
					t.Errorf("expected raw_html component, got %v", comp["type"])
				}
			},
		},
		{
			name: "extracts styles",
			html: `<html><head><style>body { color: blue; }</style></head><body><p>Content</p></body></html>`,
			check: func(t *testing.T, def map[string]any) {
				pages := def["pages"].([]any)
				page := pages[0].(map[string]any)
				styles, _ := page["page_styles"].(string)
				if !strings.Contains(styles, "color: blue") {
					t.Errorf("expected extracted styles, got %q", styles)
				}
			},
		},
		{
			name: "extracts scripts",
			html: `<html><body><p>Hello</p><script>console.log('test');</script></body></html>`,
			check: func(t *testing.T, def map[string]any) {
				pages := def["pages"].([]any)
				page := pages[0].(map[string]any)
				scripts, _ := page["page_js"].(string)
				if !strings.Contains(scripts, "console.log") {
					t.Errorf("expected extracted scripts, got %q", scripts)
				}
			},
		},
		{
			name: "extracts title",
			html: `<html><head><title>My Login Page</title></head><body><p>Content</p></body></html>`,
			check: func(t *testing.T, def map[string]any) {
				pages := def["pages"].([]any)
				page := pages[0].(map[string]any)
				title, _ := page["title"].(string)
				if title != "My Login Page" {
					t.Errorf("expected title 'My Login Page', got %q", title)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			existing := defaultDefinition()
			def, err := ParseHTMLToDefinition(tt.html, existing)
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if tt.check != nil {
				tt.check(t, def)
			}
		})
	}
}

func TestImportRawHTML(t *testing.T) {
	tests := []struct {
		name    string
		html    string
		wantErr bool
	}{
		{name: "empty", html: "", wantErr: true},
		{name: "valid", html: "<div>Hello</div>"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			existing := defaultDefinition()
			def, err := ImportRawHTML(tt.html, existing)
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			// Verify the raw_html component was created.
			pages := def["pages"].([]any)
			page := pages[0].(map[string]any)
			tree := page["component_tree"].([]any)
			if len(tree) == 0 {
				t.Fatal("expected at least one component")
			}
			comp := tree[0].(map[string]any)
			if comp["type"] != "raw_html" {
				t.Errorf("expected raw_html, got %v", comp["type"])
			}
			props := comp["properties"].(map[string]any)
			if props["content"] != "<div>Hello</div>" {
				t.Errorf("expected original HTML content, got %v", props["content"])
			}
		})
	}
}

func TestClonePageFromURL(t *testing.T) {
	// Set up a test HTTP server serving sample HTML.
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/404" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "text/html")
		fmt.Fprint(w, `<html><head><title>Test</title></head><body><h1>Hello</h1></body></html>`)
	}))
	defer ts.Close()

	tests := []struct {
		name    string
		url     string
		wantErr bool
	}{
		{name: "invalid URL", url: "not-a-url", wantErr: true},
		{name: "valid URL", url: ts.URL + "/page"},
		{name: "404 URL", url: ts.URL + "/404", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			def, err := ClonePageFromURL(tt.url, false, true)
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			// Verify structure.
			if def["clone_source"] != tt.url {
				t.Errorf("expected clone_source %q, got %v", tt.url, def["clone_source"])
			}
			pages, ok := def["pages"].([]any)
			if !ok || len(pages) == 0 {
				t.Fatal("expected pages in cloned definition")
			}
		})
	}
}

func TestImportModes(t *testing.T) {
	htmlContent := `<html>
<head>
<title>Test Form</title>
<style>body { font-family: Arial; }</style>
</head>
<body>
<form action="/submit" method="post">
<input type="text" name="username" placeholder="Username">
<input type="password" name="password" placeholder="Password">
<button type="submit">Login</button>
</form>
<script>document.querySelector('form').addEventListener('submit', function(e){});</script>
</body>
</html>`

	t.Run("builder mode", func(t *testing.T) {
		def, err := ParseHTMLToDefinition(htmlContent, defaultDefinition())
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		pages := def["pages"].([]any)
		page := pages[0].(map[string]any)

		// Should have extracted title.
		if page["title"] != "Test Form" {
			t.Errorf("expected title 'Test Form', got %v", page["title"])
		}

		// Should have extracted styles.
		styles, _ := page["page_styles"].(string)
		if !strings.Contains(styles, "font-family: Arial") {
			t.Error("expected extracted styles")
		}

		// Should have extracted scripts.
		scripts, _ := page["page_js"].(string)
		if !strings.Contains(scripts, "addEventListener") {
			t.Error("expected extracted scripts")
		}
	})

	t.Run("raw mode", func(t *testing.T) {
		def, err := ImportRawHTML(htmlContent, defaultDefinition())
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		pages := def["pages"].([]any)
		page := pages[0].(map[string]any)
		tree := page["component_tree"].([]any)

		// Should have a single raw_html component with the full content.
		if len(tree) != 1 {
			t.Fatalf("expected 1 component, got %d", len(tree))
		}
		comp := tree[0].(map[string]any)
		props := comp["properties"].(map[string]any)
		content := props["content"].(string)
		if !strings.Contains(content, "<form") {
			t.Error("expected raw HTML to contain the form")
		}
	})
}

func TestCopyDefinitionDeepCopy(t *testing.T) {
	original := map[string]any{
		"schema_version": 1,
		"pages": []any{
			map[string]any{
				"page_id": "p1",
				"name":    "Page 1",
				"route":   "/",
				"title":   "Original Title",
				"meta_tags": []any{
					map[string]any{"name": "description", "content": "original"},
				},
			},
		},
		"global_styles": "body { margin: 0; }",
	}

	copied := copyDefinition(original)

	// Modify nested value in copy.
	copiedPages := copied["pages"].([]any)
	copiedPage := copiedPages[0].(map[string]any)
	copiedPage["title"] = "Modified Title"
	copiedMetas := copiedPage["meta_tags"].([]any)
	copiedMeta := copiedMetas[0].(map[string]any)
	copiedMeta["content"] = "modified"

	// Original should NOT be affected.
	origPages := original["pages"].([]any)
	origPage := origPages[0].(map[string]any)
	if origPage["title"] != "Original Title" {
		t.Errorf("deep copy failed: original title was modified to %v", origPage["title"])
	}
	origMetas := origPage["meta_tags"].([]any)
	origMeta := origMetas[0].(map[string]any)
	if origMeta["content"] != "original" {
		t.Errorf("deep copy failed: original meta content was modified to %v", origMeta["content"])
	}
}
