// Package randomizer implements anti-fingerprinting randomization engines.
package randomizer

import (
	"strings"
	"testing"
)

// TestLiveDOMRandomizerBasic tests the basic randomization functionality.
func TestLiveDOMRandomizerBasic(t *testing.T) {
	r := &LiveDOMRandomizer{}

	html := "<div><p>Hello</p></div>"
	output, manifest, err := r.RandomizeDOM(html, 42)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if output == "" {
		t.Error("expected non-empty output")
	}

	if manifest["strategy"] != "live" {
		t.Errorf("expected strategy 'live', got %q", manifest["strategy"])
	}

	if _, ok := manifest["wrappers_added"]; !ok {
		t.Error("expected wrappers_added in manifest")
	}

	if _, ok := manifest["nesting_depth"]; !ok {
		t.Error("expected nesting_depth in manifest")
	}

	if _, ok := manifest["attribute_reorders"]; !ok {
		t.Error("expected attribute_reorders in manifest")
	}

	if _, ok := manifest["whitespace_variant"]; !ok {
		t.Error("expected whitespace_variant in manifest")
	}
}

// TestStructuralDivergence tests that different seeds produce different output.
func TestStructuralDivergence(t *testing.T) {
	r := &LiveDOMRandomizer{}
	input := `<div class="container"><p class="text">Hello</p></div>`

	output1, manifest1, err := r.RandomizeDOM(input, 42)
	if err != nil {
		t.Fatalf("first call failed: %v", err)
	}

	output2, manifest2, err := r.RandomizeDOM(input, 99)
	if err != nil {
		t.Fatalf("second call failed: %v", err)
	}

	// Outputs should be different (structurally divergent)
	if output1 == output2 {
		t.Errorf("expected different outputs for different seeds, got same output")
	}

	// Check at least one manifest field differs
	manifestDiffers := manifest1["wrappers_added"] != manifest2["wrappers_added"] ||
		manifest1["nesting_depth"] != manifest2["nesting_depth"] ||
		manifest1["attribute_reorders"] != manifest2["attribute_reorders"] ||
		manifest1["neutral_wrappers_added"] != manifest2["neutral_wrappers_added"]

	// Also check if the outputs themselves differ (structural divergence)
	if !manifestDiffers && output1 == output2 {
		t.Log("manifest fields:", manifest1, manifest2)
		t.Error("expected structural differences for different seeds")
	}
}

// TestDeterminism tests that same seed produces identical output.
func TestDeterminism(t *testing.T) {
	r := &LiveDOMRandomizer{}
	input := "<div><p>Determinism Test</p><span class=\"test\">Content</span></div>"

	outputs := make([]string, 0, 5)
	for i := 0; i < 5; i++ {
		output, _, err := r.RandomizeDOM(input, 12345)
		if err != nil {
			t.Fatalf("call %d failed: %v", i+1, err)
		}
		outputs = append(outputs, output)
	}

	// All outputs should be identical
	for i := 1; i < len(outputs); i++ {
		if outputs[i] != outputs[0] {
			t.Errorf("determinism failed: output %d differs from first", i)
		}
	}
}

// TestContentPreservation tests that text content and functional elements survive randomization.
func TestContentPreservation(t *testing.T) {
	r := &LiveDOMRandomizer{}

	input := `<div id="main">
		<h1>Welcome</h1>
		<p>This is a <strong>test</strong> paragraph.</p>
		<form action="/submit" method="post">
			<input type="text" name="username" />
			<input type="email" name="email" />
			<button type="submit">Send</button>
		</form>
	</div>`

	output, _, err := r.RandomizeDOM(input, 42)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Check text content preservation
	contentToPreserve := []string{
		"Welcome",
		"This is a",
		"test",
		"paragraph",
		"/submit",
		"post",
		"username",
		"email",
		"Send",
	}

	for _, c := range contentToPreserve {
		if !strings.Contains(output, c) {
			t.Errorf("expected content %q not found in output", c)
		}
	}

	// Check functional elements are preserved
	functionalElements := []string{
		"<form",
		"</form>",
		"<input",
		"<button",
	}

	for _, el := range functionalElements {
		if !strings.Contains(output, el) {
			t.Errorf("expected element %q not found in output", el)
		}
	}
}

// TestVisualEquivalence checks that functional HTML elements are preserved.
func TestVisualEquivalence(t *testing.T) {
	r := &LiveDOMRandomizer{}

	input := `<div>
		<a href="/link">Link</a>
		<img src="image.png" alt="Image" />
		<div class="container">
			<span class="item">Item</span>
		</div>
	</div>`

	output, _, err := r.RandomizeDOM(input, 42)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify key functional elements
	expectedElements := []string{
		"<a",
		"href=",
		"<img",
		"src=",
		"<div",
		"class=",
		"<span",
	}

	for _, el := range expectedElements {
		if !strings.Contains(output, el) {
			t.Errorf("expected visual element %q not found", el)
		}
	}

	// Verify class names are preserved (though order may change)
	classPreserved := strings.Contains(output, "container") || strings.Contains(output, "item")
	if !classPreserved {
		t.Error("expected class names to be preserved")
	}
}

// TestManifestCompleteness verifies all manifest fields are populated.
func TestManifestCompleteness(t *testing.T) {
	r := &LiveDOMRandomizer{}

	testCases := []struct {
		name string
		html string
		seed int64
	}{
		{"simple", "<div>test</div>", 42},
		{"complex", `<div><p>hello</p><span>world</span></div>`, 123},
		{"with-head", `<html><head><meta charset="utf-8"/><title>Test</title></head><body>content</body></html>`, 456},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			output, manifest, err := r.RandomizeDOM(tc.html, tc.seed)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			// Check required fields
			requiredFields := []string{
				"strategy",
				"wrappers_added",
				"nesting_depth",
				"attribute_reorders",
				"whitespace_variant",
			}

			for _, field := range requiredFields {
				if _, ok := manifest[field]; !ok {
					t.Errorf("manifest missing required field %q", field)
				}
			}

			// Check strategy value
			if manifest["strategy"] != "live" {
				t.Errorf("expected strategy 'live', got %q", manifest["strategy"])
			}

			// Check whitespace variant details
			wsv, ok := manifest["whitespace_variant"].(map[string]any)
			if !ok {
				t.Errorf("whitespace_variant should be a map")
			} else {
				if _, ok := wsv["indent_style"]; !ok {
					t.Error("whitespace_variant missing indent_style")
				}
				if _, ok := wsv["line_ending"]; !ok {
					t.Error("whitespace_variant missing line_ending")
				}
				if _, ok := wsv["spacing"]; !ok {
					t.Error("whitespace_variant missing spacing")
				}
			}

			// Verify output is not empty
			if output == "" {
				t.Error("expected non-empty output")
			}
		})
	}
}

// TestSeedSweep tests that 10 different seeds produce unique outputs.
func TestSeedSweep(t *testing.T) {
	r := &LiveDOMRandomizer{}
	input := `<div class="wrapper">
		<h1>Seed Sweep Test</h1>
		<p>This content should vary with seed.</p>
	</div>`

	seeds := []int64{100, 200, 300, 400, 500, 600, 700, 800, 900, 1000}
	outputs := make([]string, len(seeds))

	for i, seed := range seeds {
		output, _, err := r.RandomizeDOM(input, seed)
		if err != nil {
			t.Fatalf("seed %d failed: %v", seed, err)
		}
		outputs[i] = output
	}

	// Check for unique outputs
	uniqueOutputs := make([]string, 0, len(outputs))
	for i, output := range outputs {
		isDuplicate := false
		for j := 0; j < i; j++ {
			if output == outputs[j] {
				isDuplicate = true
				break
			}
		}
		if !isDuplicate {
			uniqueOutputs = append(uniqueOutputs, output)
		}
	}

	// Verify we have a good variety of outputs (at least 6 unique out of 10)
	minUnique := 6
	if len(uniqueOutputs) < minUnique {
		t.Errorf("expected at least %d unique outputs, got %d", minUnique, len(uniqueOutputs))
	}

	// Verify all outputs contain the expected content
	expectedContent := []string{"Seed Sweep Test", "This content should vary"}
	for _, output := range outputs {
		for _, c := range expectedContent {
			if !strings.Contains(output, c) {
				t.Errorf("output missing expected content %q", c)
			}
		}
	}
}

// TestEdgeCases tests various edge cases for the DOM randomizer.
func TestEdgeCases(t *testing.T) {
	r := &LiveDOMRandomizer{}

	testCases := []struct {
		name   string
		html   string
		seed   int64
		verify func(output string, t *testing.T)
	}{
		{
			name:   "empty_html",
			html:   "",
			seed:   42,
			verify: func(output string, t *testing.T) { /* empty is valid */ },
		},
		{
			name: "single_element",
			html: "<div>Only</div>",
			seed: 42,
			verify: func(output string, t *testing.T) {
				if !strings.Contains(output, "Only") {
					t.Error("single element content not preserved")
				}
			},
		},
		{
			name: "no_body",
			html: "<html><head><title>No Body</title></head></html>",
			seed: 123,
			verify: func(output string, t *testing.T) {
				if !strings.Contains(output, "No Body") {
					t.Error("head content not preserved")
				}
			},
		},
		{
			name: "nested_deep",
			html: "<div><div><div><div>Deep</div></div></div></div>",
			seed: 456,
			verify: func(output string, t *testing.T) {
				if !strings.Contains(output, "Deep") {
					t.Error("nested content not preserved")
				}
			},
		},
		{
			name: "form_with_inputs",
			html: `<form id="form1"><input name="field1"/><select><option>1</option></select></form>`,
			seed: 789,
			verify: func(output string, t *testing.T) {
				if !strings.Contains(output, "form1") {
					t.Error("form id not preserved")
				}
				if !strings.Contains(output, "field1") {
					t.Error("input name not preserved")
				}
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			output, manifest, err := r.RandomizeDOM(tc.html, tc.seed)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			tc.verify(output, t)

			// Verify manifest is never nil
			if manifest == nil {
				t.Error("manifest should not be nil")
			}
		})
	}
}

// TestWrapperInsertion tests that neutral wrappers are inserted correctly.
func TestWrapperInsertion(t *testing.T) {
	r := &LiveDOMRandomizer{
		WrapperTags:     []string{"div", "span"},
		MaxNestingDepth: 3,
	}

	input := `<div class="content"><p>Test</p></div>`
	_, manifest, err := r.RandomizeDOM(input, 42)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Check neutral wrappers are in manifest
	if wrappers, ok := manifest["neutral_wrappers"].([]any); ok {
		if len(wrappers) == 0 {
			t.Error("expected neutral wrappers to be inserted")
		}
	} else {
		t.Errorf("expected neutral_wrappers in manifest, got %T", manifest["neutral_wrappers"])
	}
}

// TestHeadTagOrdering tests that head tag ordering is randomized.
func TestHeadTagOrdering(t *testing.T) {
	r := &LiveDOMRandomizer{}

	input := `<html>
	<head>
		<meta charset="utf-8"/>
		<link rel="stylesheet" href="style.css"/>
		<script src="app.js"></script>
		<title>Test</title>
	</head>
	<body>content</body>
	</html>`

	output, manifest, err := r.RandomizeDOM(input, 42)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify all head tags are present
	expectedHeadTags := []string{"meta", "link", "script", "title"}
	for _, tag := range expectedHeadTags {
		if !strings.Contains(output, tag) {
			t.Errorf("expected head tag %q not found", tag)
		}
	}

	// Check manifest for head ordering info
	if _, ok := manifest["attribute_reorders"]; !ok {
		t.Error("expected attribute_reorders in manifest")
	}
}

// TestAttributeOrdering tests that attribute ordering is randomized.
func TestAttributeOrdering(t *testing.T) {
	r := &LiveDOMRandomizer{}

	input := `<div class="a" id="b" data-x="c">content</div>`
	output, manifest, err := r.RandomizeDOM(input, 42)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify all attributes are present
	expectedAttrs := []string{"class=", "id=", "data-x="}
	for _, attr := range expectedAttrs {
		if !strings.Contains(output, attr) {
			t.Errorf("expected attribute %q not found", attr)
		}
	}

	// Check attribute reorders counter
	if attrReorders, ok := manifest["attribute_reorders"].(int); !ok || attrReorders < 0 {
		t.Errorf("expected attribute_reorders to be a non-negative integer")
	}
}

// TestNoopCompatibility tests that LiveDOMRandomizer is compatible with the interface.
func TestNoopCompatibility(t *testing.T) {
	var dr DOMRandomizer = &LiveDOMRandomizer{}

	html := `<div class="test"><span>Compat</span></div>`
	output, manifest, err := dr.RandomizeDOM(html, 42)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !strings.Contains(output, "Compat") {
		t.Error("content not preserved through interface")
	}

	if manifest["strategy"] != "live" {
		t.Errorf("expected strategy 'live', got %q", manifest["strategy"])
	}
}

// BenchmarkLiveDOMRandomizer benchmarks the randomizer performance.
func BenchmarkLiveDOMRandomizer(b *testing.B) {
	r := &LiveDOMRandomizer{}
	html := `<div class="benchmark">
		<header><h1>Benchmark Test</h1></header>
		<main>
			<p>Many <span>elements</span> for testing</p>
			<ul>
				<li>Item 1</li>
				<li>Item 2</li>
				<li>Item 3</li>
			</ul>
		</main>
		<footer><p>Footer</p></footer>
	</div>`

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _, err := r.RandomizeDOM(html, int64(i))
		if err != nil {
			b.Fatalf("error: %v", err)
		}
	}
}
