package landingpage

import (
	"testing"
)

func TestValidateDefinition(t *testing.T) {
	tests := []struct {
		name    string
		def     map[string]any
		wantErr bool
		errMsg  string
	}{
		{
			name:    "nil definition",
			def:     nil,
			wantErr: true,
			errMsg:  "definition cannot be nil",
		},
		{
			name:    "missing pages",
			def:     map[string]any{},
			wantErr: true,
			errMsg:  "pages array is required",
		},
		{
			name:    "empty pages array",
			def:     map[string]any{"pages": []any{}},
			wantErr: true,
			errMsg:  "at least one page is required",
		},
		{
			name: "valid minimal definition",
			def: map[string]any{
				"schema_version": 1,
				"pages": []any{
					map[string]any{
						"page_id": "p1", "name": "Home", "route": "/",
						"component_tree": []any{},
					},
				},
			},
			wantErr: false,
		},
		{
			name: "page missing page_id",
			def: map[string]any{
				"pages": []any{
					map[string]any{"name": "Home", "route": "/"},
				},
			},
			wantErr: true,
			errMsg:  "page_id is required",
		},
		{
			name: "page missing name",
			def: map[string]any{
				"pages": []any{
					map[string]any{"page_id": "p1", "route": "/"},
				},
			},
			wantErr: true,
			errMsg:  "name is required",
		},
		{
			name: "page missing route",
			def: map[string]any{
				"pages": []any{
					map[string]any{"page_id": "p1", "name": "Home"},
				},
			},
			wantErr: true,
			errMsg:  "route is required",
		},
		{
			name: "duplicate routes",
			def: map[string]any{
				"pages": []any{
					map[string]any{"page_id": "p1", "name": "Home", "route": "/"},
					map[string]any{"page_id": "p2", "name": "Also Home", "route": "/"},
				},
			},
			wantErr: true,
			errMsg:  "duplicate route",
		},
		{
			name: "valid with components",
			def: map[string]any{
				"schema_version": 1,
				"pages": []any{
					map[string]any{
						"page_id": "p1", "name": "Home", "route": "/",
						"component_tree": []any{
							map[string]any{
								"component_id": "c1",
								"type":         "container",
								"properties":   map[string]any{},
								"children": []any{
									map[string]any{
										"component_id": "c2",
										"type":         "heading",
										"properties":   map[string]any{"content": "Hello"},
										"children":     []any{},
									},
								},
							},
						},
					},
				},
			},
			wantErr: false,
		},
		{
			name: "unknown component type",
			def: map[string]any{
				"pages": []any{
					map[string]any{
						"page_id": "p1", "name": "Home", "route": "/",
						"component_tree": []any{
							map[string]any{
								"component_id": "c1",
								"type":         "nonexistent_widget",
								"properties":   map[string]any{},
							},
						},
					},
				},
			},
			wantErr: true,
			errMsg:  "unknown component type",
		},
		{
			name: "children on non-nestable component",
			def: map[string]any{
				"pages": []any{
					map[string]any{
						"page_id": "p1", "name": "Home", "route": "/",
						"component_tree": []any{
							map[string]any{
								"component_id": "c1",
								"type":         "heading",
								"properties":   map[string]any{},
								"children": []any{
									map[string]any{
										"component_id": "c2",
										"type":         "paragraph",
										"properties":   map[string]any{},
									},
								},
							},
						},
					},
				},
			},
			wantErr: true,
			errMsg:  "cannot have children",
		},
		{
			name: "valid capture tag",
			def: map[string]any{
				"pages": []any{
					map[string]any{
						"page_id": "p1", "name": "Home", "route": "/",
						"component_tree": []any{
							map[string]any{
								"component_id": "c1",
								"type":         "password_input",
								"properties":   map[string]any{"capture_tag": "password"},
							},
						},
					},
				},
			},
			wantErr: false,
		},
		{
			name: "invalid capture tag",
			def: map[string]any{
				"pages": []any{
					map[string]any{
						"page_id": "p1", "name": "Home", "route": "/",
						"component_tree": []any{
							map[string]any{
								"component_id": "c1",
								"type":         "text_input",
								"properties":   map[string]any{"capture_tag": "invalid_tag"},
							},
						},
					},
				},
			},
			wantErr: true,
			errMsg:  "unknown capture_tag",
		},
		{
			name: "valid event binding",
			def: map[string]any{
				"pages": []any{
					map[string]any{
						"page_id": "p1", "name": "Home", "route": "/",
						"component_tree": []any{
							map[string]any{
								"component_id": "c1",
								"type":         "button",
								"properties":   map[string]any{},
								"event_bindings": []any{
									map[string]any{"event": "onClick", "handler": "alert('hi')"},
								},
							},
						},
					},
				},
			},
			wantErr: false,
		},
		{
			name: "invalid event binding",
			def: map[string]any{
				"pages": []any{
					map[string]any{
						"page_id": "p1", "name": "Home", "route": "/",
						"component_tree": []any{
							map[string]any{
								"component_id": "c1",
								"type":         "button",
								"properties":   map[string]any{},
								"event_bindings": []any{
									map[string]any{"event": "onFakeEvent", "handler": ""},
								},
							},
						},
					},
				},
			},
			wantErr: true,
			errMsg:  "unknown event type",
		},
		{
			name: "multi-page with navigation",
			def: map[string]any{
				"schema_version": 1,
				"pages": []any{
					map[string]any{"page_id": "p1", "name": "Login", "route": "/"},
					map[string]any{"page_id": "p2", "name": "MFA", "route": "/mfa"},
					map[string]any{"page_id": "p3", "name": "Dashboard", "route": "/dashboard"},
				},
				"navigation": []any{
					map[string]any{"source_page": "p1", "trigger": "form_submit", "target_page": "p2"},
					map[string]any{"source_page": "p2", "trigger": "redirect", "target_page": "p3", "delay_ms": 3000},
				},
			},
			wantErr: false,
		},
		{
			name: "invalid navigation trigger",
			def: map[string]any{
				"pages": []any{
					map[string]any{"page_id": "p1", "name": "Home", "route": "/"},
				},
				"navigation": []any{
					map[string]any{"source_page": "p1", "trigger": "teleport", "target_page": "p2"},
				},
			},
			wantErr: true,
			errMsg:  "unknown trigger",
		},
		{
			name: "component missing component_id",
			def: map[string]any{
				"pages": []any{
					map[string]any{
						"page_id": "p1", "name": "Home", "route": "/",
						"component_tree": []any{
							map[string]any{"type": "heading", "properties": map[string]any{}},
						},
					},
				},
			},
			wantErr: true,
			errMsg:  "component_id is required",
		},
		{
			name: "component missing type",
			def: map[string]any{
				"pages": []any{
					map[string]any{
						"page_id": "p1", "name": "Home", "route": "/",
						"component_tree": []any{
							map[string]any{"component_id": "c1", "properties": map[string]any{}},
						},
					},
				},
			},
			wantErr: true,
			errMsg:  "type is required",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateDefinition(tt.def)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("expected error containing %q, got nil", tt.errMsg)
				}
				if tt.errMsg != "" && !contains(err.Error(), tt.errMsg) {
					t.Fatalf("expected error containing %q, got %q", tt.errMsg, err.Error())
				}
			} else {
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
			}
		})
	}
}

func TestGetComponentTypes(t *testing.T) {
	types := GetComponentTypes()
	if len(types) == 0 {
		t.Fatal("expected component types, got none")
	}

	// Verify all categories are represented.
	cats := map[string]bool{}
	for _, ct := range types {
		cats[ct.Category] = true
	}
	expectedCats := []string{"layout", "navigation", "text", "media", "form", "interactive", "feedback", "special"}
	for _, c := range expectedCats {
		if !cats[c] {
			t.Errorf("missing category %q", c)
		}
	}

	// Verify nestable components have CanNest=true.
	for _, ct := range types {
		if NestableComponentTypes[ct.Type] && !ct.CanNest {
			t.Errorf("component %q should have CanNest=true", ct.Type)
		}
	}

	// Verify form capture components have HasCapture=true.
	captureTypes := map[string]bool{
		"text_input": true, "password_input": true, "email_input": true,
		"textarea": true, "select": true, "hidden_field": true,
	}
	for _, ct := range types {
		if captureTypes[ct.Type] && !ct.HasCapture {
			t.Errorf("component %q should have HasCapture=true", ct.Type)
		}
	}
}

func TestNestingDepthLimit(t *testing.T) {
	// Build a definition that exceeds max nesting depth.
	deepComp := map[string]any{
		"component_id": "deep",
		"type":         "paragraph",
		"properties":   map[string]any{},
	}
	// Wrap 25 times in containers (exceeds MaxNestingDepth of 20).
	for i := 0; i < 25; i++ {
		deepComp = map[string]any{
			"component_id": "wrap",
			"type":         "container",
			"properties":   map[string]any{},
			"children":     []any{deepComp},
		}
	}

	def := map[string]any{
		"pages": []any{
			map[string]any{
				"page_id": "p1", "name": "Home", "route": "/",
				"component_tree": []any{deepComp},
			},
		},
	}

	err := ValidateDefinition(def)
	if err == nil {
		t.Fatal("expected nesting depth error, got nil")
	}
	if !contains(err.Error(), "nesting depth") {
		t.Fatalf("expected nesting depth error, got %q", err.Error())
	}
}

func TestValidateDefinition_NewProperties(t *testing.T) {
	tests := []struct {
		name string
		def  map[string]any
	}{
		{
			name: "hidden property accepted",
			def: map[string]any{
				"pages": []any{
					map[string]any{
						"page_id": "p1", "name": "Home", "route": "/",
						"component_tree": []any{
							map[string]any{
								"component_id": "c1",
								"type":         "paragraph",
								"properties":   map[string]any{"content": "Hidden text", "hidden": true},
							},
						},
					},
				},
			},
		},
		{
			name: "label property accepted",
			def: map[string]any{
				"pages": []any{
					map[string]any{
						"page_id": "p1", "name": "Home", "route": "/",
						"component_tree": []any{
							map[string]any{
								"component_id": "c1",
								"type":         "container",
								"properties":   map[string]any{"label": "Hero Section"},
								"children":     []any{},
							},
						},
					},
				},
			},
		},
		{
			name: "flex child properties accepted",
			def: map[string]any{
				"pages": []any{
					map[string]any{
						"page_id": "p1", "name": "Home", "route": "/",
						"component_tree": []any{
							map[string]any{
								"component_id": "c1",
								"type":         "column",
								"properties": map[string]any{
									"flex_grow":   1.0,
									"flex_shrink": 0.0,
									"flex_basis":  "50%",
									"align_self":  "center",
									"order":       2.0,
								},
								"children": []any{},
							},
						},
					},
				},
			},
		},
		{
			name: "row with column children accepted",
			def: map[string]any{
				"pages": []any{
					map[string]any{
						"page_id": "p1", "name": "Home", "route": "/",
						"component_tree": []any{
							map[string]any{
								"component_id": "c1",
								"type":         "row",
								"properties":   map[string]any{},
								"children": []any{
									map[string]any{
										"component_id": "c2",
										"type":         "column",
										"properties":   map[string]any{"flex_basis": "50%"},
										"children":     []any{},
									},
									map[string]any{
										"component_id": "c3",
										"type":         "column",
										"properties":   map[string]any{"flex_basis": "50%"},
										"children":     []any{},
									},
								},
							},
						},
					},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateDefinition(tt.def)
			if err != nil {
				t.Fatalf("expected valid definition, got error: %v", err)
			}
		})
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && searchSubstring(s, substr)
}

func searchSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
