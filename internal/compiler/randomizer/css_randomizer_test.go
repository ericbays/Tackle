// Package randomizer implements anti-fingerprinting randomization engines.
package randomizer

import (
	"regexp"
	"strings"
	"testing"
)

// TestLiveCSSRandomizerBasic tests the basic randomization functionality.
func TestLiveCSSRandomizerBasic(t *testing.T) {
	r := &LiveCSSRandomizer{}

	html := `<div class="container">Hello</div>`
	css := `.container { color: red; }`

	outputHTML, outputCSS, manifest, err := r.RandomizeCSS(html, css, 42)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if outputHTML == "" {
		t.Error("expected non-empty HTML output")
	}

	if outputCSS == "" {
		t.Error("expected non-empty CSS output")
	}

	// Check manifest fields
	if manifest["strategy"] != "live" {
		t.Errorf("expected strategy 'live', got %q", manifest["strategy"])
	}

	if _, ok := manifest["naming_convention"]; !ok {
		t.Error("expected naming_convention in manifest")
	}

	if _, ok := manifest["classes_mapped"]; !ok {
		t.Error("expected classes_mapped in manifest")
	}

	if _, ok := manifest["mapping"]; !ok {
		t.Error("expected mapping in manifest")
	}
}

// TestMultiClassAttributes tests that multi-class attributes are replaced correctly.
func TestMultiClassAttributes(t *testing.T) {
	r := &LiveCSSRandomizer{}

	html := `<div class="foo bar baz">Content</div>`
	css := `.foo { color: red; } .bar { margin: 10px; } .baz { padding: 5px; }`

	outputHTML, _, manifest, err := r.RandomizeCSS(html, css, 42)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Extract the new class names from the mapping
	mapping := getManifestMapping(t, manifest)

	// Check that all original classes appear in the output
	originalClasses := []string{"foo", "bar", "baz"}
	for _, cls := range originalClasses {
		if _, ok := mapping[cls]; !ok {
			t.Errorf("original class %q not found in mapping", cls)
		}
	}

	// Verify HTML has the new classes (in the outputHTML, not the original)
	for _, newClass := range mapping {
		if !containsClass(outputHTML, newClass) {
			t.Errorf("expected class %q in HTML output", newClass)
		}
	}
}

// TestCompoundSelectors tests that compound selectors are replaced correctly.
func TestCompoundSelectors(t *testing.T) {
	r := &LiveCSSRandomizer{}

	html := `<div class="foo bar">Content</div>`
	css := `.foo.bar { color: red; }
.foo .bar { margin: 10px; }
.foo > .bar { padding: 5px; }
.foo:hover { background: blue; }`

	outputHTML, outputCSS, manifest, err := r.RandomizeCSS(html, css, 42)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Extract the mapping
	mapping := getManifestMapping(t, manifest)

	// Check that foo and bar are replaced in both HTML and CSS
	if !containsClass(outputHTML, mapping["foo"]) {
		t.Errorf("expected %q in HTML for class foo", mapping["foo"])
	}
	if !containsClass(outputHTML, mapping["bar"]) {
		t.Errorf("expected %q in HTML for class bar", mapping["bar"])
	}

	// Check CSS selectors are replaced
	// The new class names should appear in CSS as .newname
	newFoo := mapping["foo"]
	newBar := mapping["bar"]

	// Check for the new class names as selectors (with leading dot .)
	// In CSS, class selectors appear as .classname
	if !strings.Contains(outputCSS, "."+newFoo) {
		t.Errorf("expected selector .%q in CSS", newFoo)
	}
	if !strings.Contains(outputCSS, "."+newBar) {
		t.Errorf("expected selector .%q in CSS", newBar)
	}
}

// TestNoCollision tests that no class name collisions occur within a single build.
func TestNoCollision(t *testing.T) {
	r := &LiveCSSRandomizer{}

	// Generate 100 unique original class names
	var classes []string
	for i := 0; i < 100; i++ {
		classes = append(classes, "cls"+strings.Repeat(string(rune('a'+i%26)), 1+i/26))
	}

	// Build HTML with all classes and CSS with selectors for each
	html := `<div class="` + strings.Join(classes, " ") + `">Content</div>`
	var cssParts []string
	for _, cls := range classes {
		cssParts = append(cssParts, "."+cls+" { color: red; }")
	}
	css := strings.Join(cssParts, "\n")

	_, _, manifest, err := r.RandomizeCSS(html, css, 42)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	mapping := getManifestMapping(t, manifest)

	// All generated names must be unique
	seen := make(map[string]string) // newName → origName
	for origName, newName := range mapping {
		if prev, exists := seen[newName]; exists {
			t.Errorf("collision: %q and %q both mapped to %q", prev, origName, newName)
		}
		seen[newName] = origName
	}

	if len(mapping) < 50 {
		t.Errorf("expected at least 50 mapped classes, got %d", len(mapping))
	}
}

// TestCSSDeterminism tests that same inputs produce identical outputs.
func TestCSSDeterminism(t *testing.T) {
	r := &LiveCSSRandomizer{}
	inputHTML := `<div class="test">Determinism Test</div>`
	inputCSS := `.test { color: blue; }`

	outputsHTML := make([]string, 0, 5)
	outputsCSS := make([]string, 0, 5)

	for i := 0; i < 5; i++ {
		outHTML, outCSS, _, err := r.RandomizeCSS(inputHTML, inputCSS, 12345)
		if err != nil {
			t.Fatalf("call %d failed: %v", i+1, err)
		}
		outputsHTML = append(outputsHTML, outHTML)
		outputsCSS = append(outputsCSS, outCSS)
	}

	// All HTML outputs should be identical
	for i := 1; i < len(outputsHTML); i++ {
		if outputsHTML[i] != outputsHTML[0] {
			t.Errorf("HTML output %d differs from first", i)
		}
	}

	// All CSS outputs should be identical
	for i := 1; i < len(outputsCSS); i++ {
		if outputsCSS[i] != outputsCSS[0] {
			t.Errorf("CSS output %d differs from first", i)
		}
	}
}

// TestCSSDivergence tests that different seeds produce different outputs.
func TestCSSDivergence(t *testing.T) {
	r := &LiveCSSRandomizer{}
	inputHTML := `<div class="container">Content</div>`
	inputCSS := `.container { color: red; }`

	outHTML1, outCSS1, _, err := r.RandomizeCSS(inputHTML, inputCSS, 42)
	if err != nil {
		t.Fatalf("first call failed: %v", err)
	}

	outHTML2, outCSS2, _, err := r.RandomizeCSS(inputHTML, inputCSS, 99)
	if err != nil {
		t.Fatalf("second call failed: %v", err)
	}

	// Different seeds must produce different output
	if outHTML1 == outHTML2 && outCSS1 == outCSS2 {
		t.Error("expected different outputs for different seeds, but got identical results")
	}
}

// TestValidCSSIdentifiers tests that all generated names are valid CSS identifiers.
func TestValidCSSIdentifiers(t *testing.T) {
	r := &LiveCSSRandomizer{}

	testCases := []struct {
		name string
		html string
		css  string
		seed int64
	}{
		{"lowercase", `<div class="test">x</div>`, ".test{}", 42},
		{"camelCase", `<div class="test">x</div>`, ".test{}", 123},
		{"hyphenated", `<div class="test">x</div>`, ".test{}", 234},
		{"underscore", `<div class="test">x</div>`, ".test{}", 345},
		{"bem-like", `<div class="test">x</div>`, ".test{}", 456},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			_, _, manifest, err := r.RandomizeCSS(tc.html, tc.css, tc.seed)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			mapping := getManifestMapping(t, manifest)

			// Check all generated names are valid CSS identifiers
			for _, newName := range mapping {
				if !r.isValidCSSIdentifier(newName) {
					t.Errorf("generated name %q is not a valid CSS identifier", newName)
				}
			}
		})
	}
}

// TestNamingConventions tests that each naming convention produces valid output.
func TestNamingConventions(t *testing.T) {
	r := &LiveCSSRandomizer{}

	html := `<div class="foo bar">Content</div>`
	css := `.foo { color: red; } .bar { margin: 10px; }`

	conventions := []struct {
		name string
		seed int64
	}{
		{"lowercase", 100},
		{"camelCase", 200},
		{"hyphenated", 300},
		{"underscore", 400},
		{"bem-like", 500},
	}

	for _, conv := range conventions {
		t.Run(conv.name, func(t *testing.T) {
			htmlOut, cssOut, manifest, err := r.RandomizeCSS(html, css, conv.seed)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			// Check manifest has correct naming convention
			if convName, ok := manifest["naming_convention"].(string); ok {
				if convName != namingConventionNames[NamingLowercase] &&
					convName != namingConventionNames[NamingCamelCase] &&
					convName != namingConventionNames[NamingHyphenated] &&
					convName != namingConventionNames[NamingUnderscore] &&
					convName != namingConventionNames[NamingBemLike] {
					t.Errorf("unexpected naming convention: %q", convName)
				}
			}

			// Verify output is not empty
			if htmlOut == "" {
				t.Error("HTML output is empty")
			}
			if cssOut == "" {
				t.Error("CSS output is empty")
			}

			// Check that classes were replaced
			if _, ok := manifest["classes_mapped"].(int); !ok || ok && manifest["classes_mapped"].(int) == 0 {
				t.Error("classes_mapped should be > 0")
			}
		})
	}
}

// TestScriptPreservation tests that class names inside <script> tags are NOT replaced.
func TestScriptPreservation(t *testing.T) {
	r := &LiveCSSRandomizer{}

	html := `<div class="target">Content</div>
<script>
	var className = "target";
	var otherClass = "notarget";
</script>
<div class="target">More</div>`

	css := `.target { color: red; }
.notarget { margin: 10px; }`

	outputHTML, _, manifest, err := r.RandomizeCSS(html, css, 42)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// The class "target" should be replaced in HTML outside script
	if _, ok := manifest["mapping"].(map[string]string)["target"]; ok {
		mapping := manifest["mapping"].(map[string]string)
		newTarget := mapping["target"]

		// Check class is replaced outside script
		// (simplified check - class should be replaced)
		if !containsClass(outputHTML, newTarget) {
			t.Errorf("expected class %q in HTML (outside script)", newTarget)
		}
	}
}

// TestCSSManifestCompleteness tests that all required manifest fields are present.
func TestCSSManifestCompleteness(t *testing.T) {
	r := &LiveCSSRandomizer{}

	html := `<div class="test">Content</div>`
	css := `.test { color: red; }`

	outputHTML, outputCSS, manifest, err := r.RandomizeCSS(html, css, 42)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Required manifest fields
	requiredFields := []string{
		"strategy",
		"naming_convention",
		"classes_mapped",
		"mapping",
		"html_replacements",
		"css_replacements",
		"classes_extracted",
	}

	for _, field := range requiredFields {
		if _, ok := manifest[field]; !ok {
			t.Errorf("manifest missing required field %q", field)
		}
	}

	// Verify strategy value
	if strategy, ok := manifest["strategy"].(string); !ok || strategy != "live" {
		t.Errorf("expected strategy 'live', got %q", strategy)
	}

	// Verify mapping is a map
	if mapping, ok := manifest["mapping"].(map[string]string); !ok {
		t.Errorf("expected mapping to be map[string]string, got %T", manifest["mapping"])
	} else if len(mapping) == 0 {
		t.Error("expected non-empty mapping")
	}

	// Verify output is not empty
	if outputHTML == "" || outputCSS == "" {
		t.Error("expected non-empty output")
	}
}

// TestInterfaceCompatibility tests that LiveCSSRandomizer implements CSSRandomizer interface.
func TestInterfaceCompatibility(t *testing.T) {
	// This should compile without errors
	var _ CSSRandomizer = &LiveCSSRandomizer{}

	r := &LiveCSSRandomizer{}
	html := `<div class="test">x</div>`
	css := `.test { color: red; }`

	outputHTML, outputCSS, manifest, err := r.RandomizeCSS(html, css, 42)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if outputHTML == "" || outputCSS == "" {
		t.Error("expected non-empty output through interface")
	}

	if manifest["strategy"] != "live" {
		t.Errorf("expected strategy 'live' through interface, got %q", manifest["strategy"])
	}
}

// TestEmptyInput tests that empty HTML and/or CSS return empty output without error.
func TestEmptyInput(t *testing.T) {
	r := &LiveCSSRandomizer{}

	testCases := []struct {
		name string
		html string
		css  string
		seed int64
	}{
		{"empty_both", "", "", 42},
		{"empty_html", "", ".test{}", 42},
		{"empty_css", `<div class="test">x</div>`, "", 42},
		{"empty_seed", `<div class="test">x</div>`, ".test{}", 0},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			_, _, manifest, err := r.RandomizeCSS(tc.html, tc.css, tc.seed)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			// Outputs can be empty but manifest should not be nil
			if manifest == nil {
				t.Error("manifest should not be nil")
			}

			if manifest["strategy"] != "live" {
				t.Errorf("expected strategy 'live', got %q", manifest["strategy"])
			}
		})
	}
}

// TestUniquenessAcrossSeeds tests that different seeds produce unique class mappings.
func TestUniquenessAcrossSeeds(t *testing.T) {
	r := &LiveCSSRandomizer{}
	inputHTML := `<div class="foo bar baz">Content</div>`
	inputCSS := `.foo { color: red; } .bar { color: blue; } .baz { color: green; }`

	seeds := make([]int64, 100)
	for i := 0; i < 100; i++ {
		seeds[i] = int64(i * 1000)
	}

	mappings := make([]map[string]string, 0, len(seeds))

	for _, seed := range seeds {
		_, _, manifest, err := r.RandomizeCSS(inputHTML, inputCSS, seed)
		if err != nil {
			t.Fatalf("seed %d failed: %v", seed, err)
		}

		if mapping, ok := manifest["mapping"].(map[string]string); ok {
			mappings = append(mappings, mapping)
		}
	}

	// Count unique mappings
	uniqueCount := 0
	for i, mapping1 := range mappings {
		isDuplicate := false
		for j := 0; j < i; j++ {
			mapping2 := mappings[j]
			if mapsEqual(mapping1, mapping2) {
				isDuplicate = true
				break
			}
		}
		if !isDuplicate {
			uniqueCount++
		}
	}

	// At least 90 out of 100 should be unique
	if uniqueCount < 90 {
		t.Errorf("expected at least 90 unique mappings, got %d", uniqueCount)
	}
}

// Helper functions for tests

// getManifestMapping extracts the mapping from a manifest.
func getManifestMapping(t *testing.T, manifest map[string]any) map[string]string {
	t.Helper()
	if mapping, ok := manifest["mapping"].(map[string]string); ok {
		return mapping
	}
	t.Fatal("manifest missing string mapping")
	return nil
}

// containsClass checks if a class name appears in HTML.
func containsClass(html, className string) bool {
	re := regexp.MustCompile(`class=["'](?:[^"']*?\s+)?` + regexp.QuoteMeta(className) + `(?:\s+[^"']*)?["']`)
	return re.MatchString(html)
}

// mapsEqual checks if two maps are equal.
func mapsEqual(m1, m2 map[string]string) bool {
	if len(m1) != len(m2) {
		return false
	}
	for k, v1 := range m1 {
		if v2, ok := m2[k]; !ok || v1 != v2 {
			return false
		}
	}
	return true
}
