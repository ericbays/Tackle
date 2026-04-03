package htmlgen

import (
	"strings"
	"testing"
)

func makeSimpleDef(pages ...map[string]any) map[string]any {
	ps := make([]any, len(pages))
	for i, p := range pages {
		ps[i] = p
	}
	return map[string]any{
		"schema_version": 1,
		"pages":          ps,
		"global_styles":  "body { margin: 0; }",
		"global_js":      "console.log('loaded');",
	}
}

func makePage(route, title string, components ...map[string]any) map[string]any {
	tree := make([]any, len(components))
	for i, c := range components {
		tree[i] = c
	}
	return map[string]any{
		"page_id":        "page-1",
		"name":           title,
		"route":          route,
		"title":          title,
		"component_tree": tree,
	}
}

func makeComp(cType string, props map[string]any) map[string]any {
	return map[string]any{
		"type":       cType,
		"properties": props,
	}
}

func makeCompID(cType, compID string, props map[string]any) map[string]any {
	return map[string]any{
		"component_id": compID,
		"type":         cType,
		"properties":   props,
	}
}

func makeCompIDWithChildren(cType, compID string, props map[string]any, children ...map[string]any) map[string]any {
	ch := make([]any, len(children))
	for i, c := range children {
		ch[i] = c
	}
	return map[string]any{
		"component_id": compID,
		"type":         cType,
		"properties":   props,
		"children":     ch,
	}
}

func makeCompWithChildren(cType string, props map[string]any, children ...map[string]any) map[string]any {
	ch := make([]any, len(children))
	for i, c := range children {
		ch[i] = c
	}
	return map[string]any{
		"type":       cType,
		"properties": props,
		"children":   ch,
	}
}

func TestGeneratePageAssets_SinglePage(t *testing.T) {
	def := makeSimpleDef(
		makePage("/", "Login",
			makeComp("heading", map[string]any{"content": "Sign In", "level": "h1"}),
			makeComp("paragraph", map[string]any{"content": "Enter your credentials"}),
		),
	)

	outputs, err := GeneratePageAssets(def, PageConfig{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(outputs) != 1 {
		t.Fatalf("expected 1 page, got %d", len(outputs))
	}

	out := outputs[0]
	if out.Route != "/" {
		t.Errorf("expected route /, got %s", out.Route)
	}
	if out.Filename != "index.html" {
		t.Errorf("expected filename index.html, got %s", out.Filename)
	}
	if !strings.Contains(out.HTML, "<h1") {
		t.Error("expected h1 heading in output")
	}
	if !strings.Contains(out.HTML, "Sign In") {
		t.Error("expected heading content in output")
	}
	if !strings.Contains(out.HTML, "Enter your credentials") {
		t.Error("expected paragraph content in output")
	}
	if !strings.Contains(out.HTML, "body { margin: 0; }") {
		t.Error("expected global styles in output")
	}
	if !strings.Contains(out.HTML, "console.log") {
		t.Error("expected global JS in output")
	}
	// Should NOT have preview banner.
	if strings.Contains(out.HTML, "PREVIEW MODE") {
		t.Error("production output should not contain PREVIEW MODE banner")
	}
}

func TestGeneratePageAssets_MultiPage(t *testing.T) {
	def := makeSimpleDef(
		makePage("/", "Home", makeComp("heading", map[string]any{"content": "Home"})),
		makePage("/login", "Login", makeComp("heading", map[string]any{"content": "Login"})),
		makePage("/mfa", "MFA", makeComp("heading", map[string]any{"content": "MFA"})),
	)

	outputs, err := GeneratePageAssets(def, PageConfig{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(outputs) != 3 {
		t.Fatalf("expected 3 pages, got %d", len(outputs))
	}

	expectedRoutes := []string{"/", "/login", "/mfa"}
	expectedFiles := []string{"index.html", "login.html", "mfa.html"}
	for i, out := range outputs {
		if out.Route != expectedRoutes[i] {
			t.Errorf("page %d: expected route %s, got %s", i, expectedRoutes[i], out.Route)
		}
		if out.Filename != expectedFiles[i] {
			t.Errorf("page %d: expected filename %s, got %s", i, expectedFiles[i], out.Filename)
		}
	}
}

func TestGeneratePageAssets_FormWithCaptureTags(t *testing.T) {
	def := makeSimpleDef(
		makePage("/", "Login",
			makeCompWithChildren("container", map[string]any{},
				makeComp("email_input", map[string]any{"name": "email", "capture_tag": "email", "placeholder": "Email"}),
				makeComp("password_input", map[string]any{"name": "password", "capture_tag": "password", "placeholder": "Password"}),
				makeComp("submit_button", map[string]any{"content": "Sign In"}),
			),
		),
	)

	outputs, err := GeneratePageAssets(def, PageConfig{
		CaptureEndpoint: "/capture",
		PostCaptureAction: "redirect",
		PostCaptureRedirectURL: "https://example.com",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	html := outputs[0].HTML
	if !strings.Contains(html, `data-capture="true"`) {
		t.Error("expected form with data-capture attribute")
	}
	if !strings.Contains(html, `type="email"`) {
		t.Error("expected email input")
	}
	if !strings.Contains(html, `type="password"`) {
		t.Error("expected password input")
	}
	if !strings.Contains(html, `type="submit"`) {
		t.Error("expected submit button")
	}
	// Should include capture script.
	if !strings.Contains(html, "/capture") {
		t.Error("expected capture endpoint in script")
	}
}

func TestGeneratePageAssets_TrackingPixel(t *testing.T) {
	def := makeSimpleDef(
		makePage("/", "Home", makeComp("heading", map[string]any{"content": "Hello"})),
	)

	outputs, err := GeneratePageAssets(def, PageConfig{
		TrackingEndpoint:   "/track",
		TrackingTokenParam: "tk",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	html := outputs[0].HTML
	if !strings.Contains(html, "/track") {
		t.Error("expected tracking endpoint in output")
	}
	if !strings.Contains(html, "'tk'") {
		t.Error("expected tracking token param in output")
	}
}

func TestGeneratePageAssets_ComponentTypes(t *testing.T) {
	tests := []struct {
		name     string
		comp     map[string]any
		contains string
	}{
		{"heading", makeComp("heading", map[string]any{"content": "Title", "level": "h1"}), "<h1"},
		{"paragraph", makeComp("paragraph", map[string]any{"content": "Text"}), "<p"},
		{"image", makeComp("image", map[string]any{"src": "logo.png", "alt": "Logo"}), "<img"},
		{"button", makeComp("button", map[string]any{"content": "Click"}), "<button"},
		{"link", makeComp("link", map[string]any{"href": "/next", "content": "Next"}), "<a"},
		{"divider", makeComp("divider", map[string]any{}), "<hr"},
		{"spacer", makeComp("spacer", map[string]any{"height": "40px"}), "40px"},
		{"alert", makeComp("alert", map[string]any{"content": "Warning"}), "role=\"alert\""},
		{"video", makeComp("video", map[string]any{"src": "vid.mp4"}), "<video"},
		{"iframe", makeComp("iframe", map[string]any{"src": "https://example.com"}), "<iframe"},
		{"raw_html", makeComp("raw_html", map[string]any{"content": "<marquee>test</marquee>"}), "<marquee>"},
		{"checkbox", makeComp("checkbox", map[string]any{"name": "agree", "label": "I agree"}), "type=\"checkbox\""},
		{"radio", makeComp("radio", map[string]any{"name": "choice", "value": "a", "label": "A"}), "type=\"radio\""},
		{"textarea", makeComp("textarea", map[string]any{"name": "notes", "placeholder": "Notes"}), "<textarea"},
		{"select", makeComp("select", map[string]any{"name": "country", "options": []any{
			map[string]any{"value": "us", "label": "US"},
		}}), "<select"},
		{"spinner", makeComp("spinner", map[string]any{}), "spinner"},
		{"progress", makeComp("progress_bar", map[string]any{"value": float64(75)}), "<progress"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			def := makeSimpleDef(makePage("/", "Test", tt.comp))
			outputs, err := GeneratePageAssets(def, PageConfig{})
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if !strings.Contains(outputs[0].HTML, tt.contains) {
				t.Errorf("expected HTML to contain %q", tt.contains)
			}
		})
	}
}

func TestGeneratePageAssets_EmptyDefinition(t *testing.T) {
	_, err := GeneratePageAssets(map[string]any{}, PageConfig{})
	if err == nil {
		t.Fatal("expected error for empty definition")
	}
}

func TestRouteToFilename(t *testing.T) {
	tests := []struct {
		route    string
		expected string
	}{
		{"/", "index.html"},
		{"/login", "login.html"},
		{"/mfa/verify", "mfa_verify.html"},
		{"", "index.html"},
	}
	for _, tt := range tests {
		got := routeToFilename(tt.route)
		if got != tt.expected {
			t.Errorf("routeToFilename(%q) = %q, want %q", tt.route, got, tt.expected)
		}
	}
}

func TestCountComponents(t *testing.T) {
	def := makeSimpleDef(
		makePage("/", "Home",
			makeCompWithChildren("container", map[string]any{},
				makeComp("heading", map[string]any{"content": "Title"}),
				makeComp("paragraph", map[string]any{"content": "Text"}),
			),
		),
	)
	count := CountComponents(def)
	if count != 3 { // container + heading + paragraph
		t.Errorf("expected 3 components, got %d", count)
	}
}

func TestCountCaptureFields(t *testing.T) {
	def := makeSimpleDef(
		makePage("/", "Login",
			makeCompWithChildren("container", map[string]any{},
				makeComp("email_input", map[string]any{"name": "email", "capture_tag": "email"}),
				makeComp("password_input", map[string]any{"name": "pass", "capture_tag": "password"}),
				makeComp("text_input", map[string]any{"name": "name"}), // no capture tag
			),
		),
	)
	count := CountCaptureFields(def)
	if count != 2 {
		t.Errorf("expected 2 capture fields, got %d", count)
	}
}

func TestPostCaptureActions(t *testing.T) {
	tests := []struct {
		name    string
		config  PageConfig
		check   string
	}{
		{"redirect", PageConfig{PostCaptureAction: "redirect", PostCaptureRedirectURL: "https://example.com"}, "window.location.href"},
		{"delay_redirect", PageConfig{PostCaptureAction: "delay_redirect", PostCaptureRedirectURL: "https://example.com", PostCaptureDelayMs: 5000}, "setTimeout"},
		{"display_page", PageConfig{PostCaptureAction: "display_page", PostCapturePageRoute: "/success"}, "success.html"},
		{"no_action", PageConfig{PostCaptureAction: "no_action"}, ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			def := makeSimpleDef(
				makePage("/", "Login",
					makeCompWithChildren("container", map[string]any{},
						makeComp("password_input", map[string]any{"name": "pass", "capture_tag": "password"}),
						makeComp("submit_button", map[string]any{}),
					),
				),
			)
			outputs, err := GeneratePageAssets(def, tt.config)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			html := outputs[0].HTML
			if tt.check != "" && !strings.Contains(html, tt.check) {
				t.Errorf("expected HTML to contain %q for action %s", tt.check, tt.name)
			}
		})
	}
}

func TestEventBindings(t *testing.T) {
	comp := map[string]any{
		"type": "button",
		"properties": map[string]any{
			"id":      "btn1",
			"content": "Click",
		},
		"event_bindings": []any{
			map[string]any{
				"event":   "onClick",
				"handler": "alert('clicked')",
			},
		},
	}

	def := makeSimpleDef(makePage("/", "Test", comp))
	outputs, err := GeneratePageAssets(def, PageConfig{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	html := outputs[0].HTML
	if !strings.Contains(html, "addEventListener('click'") {
		t.Error("expected click event listener in output")
	}
	if !strings.Contains(html, "alert('clicked')") {
		t.Error("expected handler code in output")
	}
}

func TestMultiStepFormUsesDataCompID(t *testing.T) {
	// Multi-step form with 2 steps should generate JS that selects fields by data-comp-id.
	form := makeCompIDWithChildren("form", "form-1111-2222", map[string]any{
		"id": "loginForm",
		"form_steps": []any{
			map[string]any{
				"field_ids":   []any{"field-aaaa-1111", "field-aaaa-2222"},
				"progression": "immediate",
				"delay_ms":    float64(0),
			},
			map[string]any{
				"field_ids":   []any{"field-bbbb-1111"},
				"progression": "immediate",
				"delay_ms":    float64(0),
			},
		},
	},
		makeCompID("email_input", "field-aaaa-1111", map[string]any{"name": "email", "capture_tag": "email", "placeholder": "Email"}),
		makeCompID("password_input", "field-aaaa-2222", map[string]any{"name": "password", "capture_tag": "password", "placeholder": "Password"}),
		makeCompID("text_input", "field-bbbb-1111", map[string]any{"name": "mfa", "capture_tag": "mfa_token", "placeholder": "MFA Code"}),
		makeCompID("submit_button", "field-cccc-1111", map[string]any{"content": "Submit"}),
	)

	def := makeSimpleDef(makePage("/", "Login", form))
	outputs, err := GeneratePageAssets(def, PageConfig{CaptureEndpoint: "/capture"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	html := outputs[0].HTML

	// Multi-step JS should use data-comp-id, NOT data-component-id.
	if strings.Contains(html, "data-component-id") {
		t.Error("multi-step JS should NOT use data-component-id")
	}
	if !strings.Contains(html, "data-comp-id") {
		t.Error("multi-step JS should use data-comp-id for field selection")
	}
	// Verify the field IDs appear in the generated script.
	if !strings.Contains(html, "field-aaaa-1111") {
		t.Error("expected first field ID in multi-step script")
	}
	if !strings.Contains(html, "field-bbbb-1111") {
		t.Error("expected second step field ID in multi-step script")
	}
}

func TestHiddenComponentsExcluded(t *testing.T) {
	t.Run("hidden component not rendered", func(t *testing.T) {
		def := makeSimpleDef(makePage("/", "Test",
			makeComp("heading", map[string]any{"content": "Visible"}),
			makeComp("paragraph", map[string]any{"content": "Secret Text", "hidden": true}),
		))
		outputs, err := GeneratePageAssets(def, PageConfig{})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		html := outputs[0].HTML
		if !strings.Contains(html, "Visible") {
			t.Error("expected visible heading in output")
		}
		if strings.Contains(html, "Secret Text") {
			t.Error("hidden component should NOT appear in production HTML")
		}
	})

	t.Run("hidden container children also excluded", func(t *testing.T) {
		def := makeSimpleDef(makePage("/", "Test",
			makeCompWithChildren("container", map[string]any{"hidden": true},
				makeComp("heading", map[string]any{"content": "Child Of Hidden"}),
				makeComp("paragraph", map[string]any{"content": "Also Hidden"}),
			),
		))
		outputs, err := GeneratePageAssets(def, PageConfig{})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		html := outputs[0].HTML
		if strings.Contains(html, "Child Of Hidden") {
			t.Error("children of hidden container should NOT appear in production HTML")
		}
		if strings.Contains(html, "Also Hidden") {
			t.Error("children of hidden container should NOT appear in production HTML")
		}
	})
}

func TestEventBindingAutoID(t *testing.T) {
	t.Run("no explicit id generates auto id", func(t *testing.T) {
		comp := map[string]any{
			"component_id": "abcdef12-3456-7890-abcd-ef1234567890",
			"type":         "button",
			"properties":   map[string]any{"content": "Click Me"},
			"event_bindings": []any{
				map[string]any{"event": "onClick", "handler": "alert('hi')"},
			},
		}
		def := makeSimpleDef(makePage("/", "Test", comp))
		outputs, err := GeneratePageAssets(def, PageConfig{})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		html := outputs[0].HTML
		// Should auto-generate id="comp-abcdef12" from first 8 chars of component_id.
		if !strings.Contains(html, `id="comp-abcdef12"`) {
			t.Error("expected auto-generated id attribute comp-abcdef12")
		}
		if !strings.Contains(html, "getElementById('comp-abcdef12')") {
			t.Error("expected event binding JS to use auto-generated id")
		}
		if !strings.Contains(html, "alert('hi')") {
			t.Error("expected handler code in output")
		}
	})

	t.Run("explicit id takes priority", func(t *testing.T) {
		comp := map[string]any{
			"component_id": "abcdef12-3456-7890-abcd-ef1234567890",
			"type":         "button",
			"properties":   map[string]any{"id": "myBtn", "content": "Click Me"},
			"event_bindings": []any{
				map[string]any{"event": "onClick", "handler": "doStuff()"},
			},
		}
		def := makeSimpleDef(makePage("/", "Test", comp))
		outputs, err := GeneratePageAssets(def, PageConfig{})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		html := outputs[0].HTML
		if !strings.Contains(html, `id="myBtn"`) {
			t.Error("expected explicit id attribute myBtn")
		}
		if !strings.Contains(html, "getElementById('myBtn')") {
			t.Error("expected event binding JS to use explicit id")
		}
		// Should NOT contain auto-generated id.
		if strings.Contains(html, "comp-abcdef12") {
			t.Error("should not contain auto-generated id when explicit id is set")
		}
	})
}

func TestNestedFormPrevention(t *testing.T) {
	t.Run("explicit form with capture children - single form", func(t *testing.T) {
		// form > text_input(capture) + password_input(capture) should produce exactly ONE <form>.
		comp := makeCompWithChildren("form", map[string]any{},
			makeComp("text_input", map[string]any{"name": "email", "capture_tag": "email", "placeholder": "Email"}),
			makeComp("password_input", map[string]any{"name": "pass", "capture_tag": "password", "placeholder": "Password"}),
			makeComp("submit_button", map[string]any{"content": "Login"}),
		)
		def := makeSimpleDef(makePage("/", "Login", comp))
		outputs, err := GeneratePageAssets(def, PageConfig{CaptureEndpoint: "/capture"})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		formCount := strings.Count(outputs[0].HTML, "<form")
		if formCount != 1 {
			t.Errorf("expected exactly 1 <form>, got %d", formCount)
		}
	})

	t.Run("container with capture children no explicit form - auto-wrap", func(t *testing.T) {
		// container > text_input(capture) should auto-wrap with ONE <form>.
		comp := makeCompWithChildren("container", map[string]any{},
			makeComp("email_input", map[string]any{"name": "email", "capture_tag": "email", "placeholder": "Email"}),
			makeComp("submit_button", map[string]any{"content": "Submit"}),
		)
		def := makeSimpleDef(makePage("/", "Login", comp))
		outputs, err := GeneratePageAssets(def, PageConfig{CaptureEndpoint: "/capture"})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		formCount := strings.Count(outputs[0].HTML, "<form")
		if formCount != 1 {
			t.Errorf("expected exactly 1 auto-wrapped <form>, got %d", formCount)
		}
	})

	t.Run("form wrapping inside container with nested containers", func(t *testing.T) {
		// section > row > column > text_input(capture) — only top-level section should auto-wrap.
		comp := makeCompWithChildren("section", map[string]any{},
			makeCompWithChildren("row", map[string]any{"inline_style": "display:flex;gap:16px;"},
				makeCompWithChildren("column", map[string]any{"inline_style": "flex-basis:50%;"},
					makeComp("email_input", map[string]any{"name": "email", "capture_tag": "email", "placeholder": "Email"}),
				),
				makeCompWithChildren("column", map[string]any{"inline_style": "flex-basis:50%;"},
					makeComp("password_input", map[string]any{"name": "pass", "capture_tag": "password", "placeholder": "Password"}),
				),
			),
			makeComp("submit_button", map[string]any{"content": "Login"}),
		)
		def := makeSimpleDef(makePage("/", "Login", comp))
		outputs, err := GeneratePageAssets(def, PageConfig{CaptureEndpoint: "/capture"})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		formCount := strings.Count(outputs[0].HTML, "<form")
		if formCount != 1 {
			t.Errorf("expected exactly 1 auto-wrapped <form>, got %d", formCount)
		}
	})

	t.Run("form > container > capture fields - no nested form", func(t *testing.T) {
		// form > container > text_input(capture) — container should NOT get auto-wrapped.
		comp := makeCompWithChildren("form", map[string]any{},
			makeCompWithChildren("container", map[string]any{},
				makeComp("text_input", map[string]any{"name": "email", "capture_tag": "email", "placeholder": "Email"}),
				makeComp("password_input", map[string]any{"name": "pass", "capture_tag": "password", "placeholder": "Password"}),
			),
			makeComp("submit_button", map[string]any{"content": "Login"}),
		)
		def := makeSimpleDef(makePage("/", "Login", comp))
		outputs, err := GeneratePageAssets(def, PageConfig{CaptureEndpoint: "/capture"})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		formCount := strings.Count(outputs[0].HTML, "<form")
		if formCount != 1 {
			t.Errorf("expected exactly 1 <form> (no nested forms), got %d", formCount)
		}
	})
}

func TestCompoundBlockFlexRendering(t *testing.T) {
	// section > row > 2 columns — should produce correct flex layout HTML.
	section := makeCompIDWithChildren("section", "sec-1111",
		map[string]any{"inline_style": "display:flex;flex-direction:column;"},
		makeCompIDWithChildren("row", "row-1111",
			map[string]any{"inline_style": "display:flex;flex-direction:row;gap:16px;"},
			makeCompIDWithChildren("column", "col-1111",
				map[string]any{"inline_style": "display:flex;flex-direction:column;flex-basis:50%;"},
				makeCompID("heading", "h-1111", map[string]any{"content": "Left Column", "level": "h2"}),
			),
			makeCompIDWithChildren("column", "col-2222",
				map[string]any{"inline_style": "display:flex;flex-direction:column;flex-basis:50%;"},
				makeCompID("paragraph", "p-1111", map[string]any{"content": "Right Column"}),
			),
		),
	)

	def := makeSimpleDef(makePage("/", "Test", section))
	outputs, err := GeneratePageAssets(def, PageConfig{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	html := outputs[0].HTML

	// Verify section renders with its inline_style.
	if !strings.Contains(html, "flex-direction:column") {
		t.Error("expected section to have flex-direction:column")
	}
	// Verify row has display:flex and gap.
	if !strings.Contains(html, "gap:16px") {
		t.Error("expected row to have gap:16px")
	}
	// Verify columns have flex-basis.
	if !strings.Contains(html, "flex-basis:50%") {
		t.Error("expected columns to have flex-basis:50%")
	}
	// Verify content renders.
	if !strings.Contains(html, "Left Column") {
		t.Error("expected left column content")
	}
	if !strings.Contains(html, "Right Column") {
		t.Error("expected right column content")
	}
	// Verify data-comp-id attributes present.
	if !strings.Contains(html, `data-comp-id="row-1111"`) {
		t.Error("expected data-comp-id on row")
	}
	if !strings.Contains(html, `data-comp-id="col-1111"`) {
		t.Error("expected data-comp-id on first column")
	}
}

func TestResponsiveFlexOverrides(t *testing.T) {
	// Column with responsive_styles that override flex-basis on mobile.
	col := map[string]any{
		"component_id": "col-resp-1111",
		"type":         "column",
		"properties": map[string]any{
			"inline_style": "display:flex;flex-direction:column;flex-basis:50%;",
			"responsive_styles": map[string]any{
				"tablet": "flex-basis:100%;",
				"mobile": "flex-basis:100%;flex-direction:column;",
			},
		},
		"children": []any{
			map[string]any{
				"component_id": "p-resp-1",
				"type":         "paragraph",
				"properties":   map[string]any{"content": "Responsive Content"},
			},
		},
	}

	def := makeSimpleDef(makePage("/", "Test", col))
	outputs, err := GeneratePageAssets(def, PageConfig{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	html := outputs[0].HTML

	// Verify @media tablet block targets the column.
	if !strings.Contains(html, "@media (max-width: 1024px)") {
		t.Error("expected tablet @media query")
	}
	if !strings.Contains(html, `[data-comp-id="col-resp-1111"]`) {
		t.Error("expected responsive rule to target column by data-comp-id")
	}
	// Verify @media mobile block.
	if !strings.Contains(html, "@media (max-width: 768px)") {
		t.Error("expected mobile @media query")
	}
	// Verify content still renders.
	if !strings.Contains(html, "Responsive Content") {
		t.Error("expected column content")
	}
}
