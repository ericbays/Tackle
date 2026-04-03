package landingpage

import (
	"strings"
	"testing"
)

func TestRenderPreviewHTML(t *testing.T) {
	tests := []struct {
		name       string
		def        map[string]any
		pageIndex  int
		wantErr    bool
		wantContains []string
	}{
		{
			name:    "empty definition",
			def:     map[string]any{},
			wantErr: true,
		},
		{
			name: "minimal page with heading",
			def: map[string]any{
				"pages": []any{
					map[string]any{
						"page_id": "p1", "name": "Home", "route": "/",
						"title":      "Test Page",
						"page_styles": "body { color: red; }",
						"page_js":     "console.log('hello');",
						"component_tree": []any{
							map[string]any{
								"component_id": "c1",
								"type":         "heading",
								"properties":   map[string]any{"content": "Hello World", "level": "h1"},
							},
						},
					},
				},
				"global_styles": "* { margin: 0; }",
				"global_js":     "// global",
			},
			wantContains: []string{
				"<title>Test Page</title>",
				"PREVIEW MODE",
				"<h1>Hello World</h1>",
				"body { color: red; }",
				"console.log('hello');",
				"* { margin: 0; }",
			},
		},
		{
			name: "form elements",
			def: map[string]any{
				"pages": []any{
					map[string]any{
						"page_id": "p1", "name": "Login", "route": "/",
						"title": "Login",
						"component_tree": []any{
							map[string]any{
								"component_id": "c1",
								"type":         "email_input",
								"properties":   map[string]any{"name": "user_email", "placeholder": "Email"},
							},
							map[string]any{
								"component_id": "c2",
								"type":         "password_input",
								"properties":   map[string]any{"name": "user_pass", "placeholder": "Password"},
							},
							map[string]any{
								"component_id": "c3",
								"type":         "submit_button",
								"properties":   map[string]any{"content": "Sign In"},
							},
						},
					},
				},
			},
			wantContains: []string{
				`type="email"`,
				`name="user_email"`,
				`type="password"`,
				`name="user_pass"`,
				`type="submit"`,
				"Sign In",
			},
		},
		{
			name: "nested container",
			def: map[string]any{
				"pages": []any{
					map[string]any{
						"page_id": "p1", "name": "Home", "route": "/",
						"title": "Nested",
						"component_tree": []any{
							map[string]any{
								"component_id": "c1",
								"type":         "container",
								"properties":   map[string]any{"css_class": "outer"},
								"children": []any{
									map[string]any{
										"component_id": "c2",
										"type":         "paragraph",
										"properties":   map[string]any{"content": "Inside container"},
									},
								},
							},
						},
					},
				},
			},
			wantContains: []string{
				`class="outer"`,
				"Inside container",
			},
		},
		{
			name: "iframe component",
			def: map[string]any{
				"pages": []any{
					map[string]any{
						"page_id": "p1", "name": "Home", "route": "/",
						"title": "Iframe Test",
						"component_tree": []any{
							map[string]any{
								"component_id": "c1",
								"type":         "iframe",
								"properties":   map[string]any{"src": "https://example.com", "width": "100%", "height": "600", "sandbox": "allow-scripts"},
							},
						},
					},
				},
			},
			wantContains: []string{
				"<iframe",
				`src="https://example.com"`,
				`sandbox="allow-scripts"`,
			},
		},
		{
			name: "multi-page renders second page",
			def: map[string]any{
				"pages": []any{
					map[string]any{
						"page_id": "p1", "name": "Page 1", "route": "/",
						"title": "Page One",
						"component_tree": []any{
							map[string]any{"component_id": "c1", "type": "heading", "properties": map[string]any{"content": "First Page"}},
						},
					},
					map[string]any{
						"page_id": "p2", "name": "Page 2", "route": "/page2",
						"title": "Page Two",
						"component_tree": []any{
							map[string]any{"component_id": "c2", "type": "heading", "properties": map[string]any{"content": "Second Page"}},
						},
					},
				},
			},
			pageIndex: 1,
			wantContains: []string{
				"<title>Page Two</title>",
				"Second Page",
			},
		},
		{
			name: "viewport switching - responsive meta",
			def: map[string]any{
				"pages": []any{
					map[string]any{
						"page_id": "p1", "name": "Home", "route": "/",
						"title":          "Responsive",
						"component_tree": []any{},
					},
				},
			},
			wantContains: []string{
				`name="viewport"`,
				"width=device-width",
			},
		},
		{
			name: "raw html block",
			def: map[string]any{
				"pages": []any{
					map[string]any{
						"page_id": "p1", "name": "Home", "route": "/",
						"title": "Raw",
						"component_tree": []any{
							map[string]any{
								"component_id": "c1",
								"type":         "raw_html",
								"properties":   map[string]any{"content": "<div class='custom'>Raw Content</div>"},
							},
						},
					},
				},
			},
			wantContains: []string{
				"<div class='custom'>Raw Content</div>",
			},
		},
		{
			name: "meta tags rendered",
			def: map[string]any{
				"pages": []any{
					map[string]any{
						"page_id": "p1", "name": "Home", "route": "/",
						"title": "Meta Test",
						"meta_tags": []any{
							map[string]any{"name": "description", "content": "A test page"},
							map[string]any{"name": "author", "content": "Tackle"},
						},
						"component_tree": []any{},
					},
				},
			},
			wantContains: []string{
				`name="description" content="A test page"`,
				`name="author" content="Tackle"`,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			html, err := RenderPreviewHTML(tt.def, tt.pageIndex)
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			for _, want := range tt.wantContains {
				if !strings.Contains(html, want) {
					t.Errorf("expected HTML to contain %q, but it didn't.\nHTML:\n%s", want, html)
				}
			}
		})
	}
}

func TestPreviewHiddenComponentsExcluded(t *testing.T) {
	def := map[string]any{
		"pages": []any{
			map[string]any{
				"page_id": "p1", "name": "Home", "route": "/",
				"title": "Test",
				"component_tree": []any{
					map[string]any{
						"component_id": "c1",
						"type":         "heading",
						"properties":   map[string]any{"content": "Visible Heading"},
					},
					map[string]any{
						"component_id": "c2",
						"type":         "paragraph",
						"properties":   map[string]any{"content": "Hidden Para", "hidden": true},
					},
					map[string]any{
						"component_id": "c3",
						"type":         "container",
						"properties":   map[string]any{"hidden": true},
						"children": []any{
							map[string]any{
								"component_id": "c4",
								"type":         "heading",
								"properties":   map[string]any{"content": "Child Of Hidden"},
							},
						},
					},
				},
			},
		},
	}

	html, err := RenderPreviewHTML(def, 0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(html, "Visible Heading") {
		t.Error("expected visible heading in preview")
	}
	if strings.Contains(html, "Hidden Para") {
		t.Error("hidden component should NOT appear in preview HTML")
	}
	if strings.Contains(html, "Child Of Hidden") {
		t.Error("children of hidden container should NOT appear in preview HTML")
	}
}

func TestPreviewBannerNotInProduction(t *testing.T) {
	def := map[string]any{
		"pages": []any{
			map[string]any{
				"page_id": "p1", "name": "Home", "route": "/",
				"title":          "Test",
				"component_tree": []any{},
			},
		},
	}

	html, err := RenderPreviewHTML(def, 0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Preview mode should show the banner.
	if !strings.Contains(html, "PREVIEW MODE") {
		t.Error("expected PREVIEW MODE banner in preview output")
	}
}
