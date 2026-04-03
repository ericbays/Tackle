// Package randomizer implements anti-fingerprinting randomization engines.
package randomizer

import (
	"fmt"
	"regexp"
	"strings"
	"testing"
)

// TestDecoyHTMLCommentsCount verifies 5-20 HTML comments are injected.
func TestDecoyHTMLCommentsCount(t *testing.T) {
	di := NewLiveDecoyInjector()

	html := `<html><head></head><body><div>Content</div><p>More</p><span>Text</span></body></html>`
	css := `body { margin: 0; }`
	js := `console.log("app");`

	// Count original comments
	origComments := strings.Count(html, "<!--")

	newHTML, _, _, manifest, err := di.InjectDecoys(html, css, js, 42)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	newComments := strings.Count(newHTML, "<!--")
	added := newComments - origComments

	if added < 5 || added > 20 {
		t.Errorf("expected 5-20 HTML comments injected, got %d", added)
	}

	count, ok := manifest["html_comments_injected"].(int)
	if !ok {
		t.Fatalf("expected html_comments_injected to be int, got %T", manifest["html_comments_injected"])
	}
	if count < 5 || count > 20 {
		t.Errorf("manifest html_comments_injected=%d, expected 5-20", count)
	}
}

// TestDecoyHiddenElementsCount verifies 3-10 hidden elements are injected.
func TestDecoyHiddenElementsCount(t *testing.T) {
	di := NewLiveDecoyInjector()

	html := `<html><head></head><body><div>Content</div><p>More</p><span>Text</span><section>Area</section><article>Post</article></body></html>`
	css := `body { margin: 0; }`
	js := `console.log("app");`

	_, _, _, manifest, err := di.InjectDecoys(html, css, js, 42)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	count, ok := manifest["hidden_elements_injected"].(int)
	if !ok {
		t.Fatalf("expected hidden_elements_injected to be int, got %T", manifest["hidden_elements_injected"])
	}
	if count < 3 || count > 10 {
		t.Errorf("manifest hidden_elements_injected=%d, expected 3-10", count)
	}
}

// TestDecoyUnusedCSSRulesCount verifies 10-30 unused CSS rules are injected.
func TestDecoyUnusedCSSRulesCount(t *testing.T) {
	di := NewLiveDecoyInjector()

	html := `<html><head></head><body><div>Content</div></body></html>`
	css := `body { margin: 0; }`
	js := `console.log("app");`

	// Count original rules (by counting '{')
	origRules := strings.Count(css, "{")

	_, newCSS, _, manifest, err := di.InjectDecoys(html, css, js, 42)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	newRules := strings.Count(newCSS, "{")
	added := newRules - origRules

	if added < 10 || added > 30 {
		t.Errorf("expected 10-30 CSS rules injected, got %d", added)
	}

	count, ok := manifest["unused_css_rules_injected"].(int)
	if !ok {
		t.Fatalf("expected unused_css_rules_injected to be int, got %T", manifest["unused_css_rules_injected"])
	}
	if count < 10 || count > 30 {
		t.Errorf("manifest unused_css_rules_injected=%d, expected 10-30", count)
	}
}

// TestDecoyJSDecoysCount verifies 3-10 JS decoys are injected.
func TestDecoyJSDecoysCount(t *testing.T) {
	di := NewLiveDecoyInjector()

	html := `<html><head></head><body><div>Content</div></body></html>`
	css := `body { margin: 0; }`
	js := `console.log("app");`

	_, _, newJS, manifest, err := di.InjectDecoys(html, css, js, 42)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Original JS should still be present
	if !strings.Contains(newJS, `console.log("app");`) {
		t.Error("expected original JS to be preserved")
	}

	// Check that new JS is longer than original
	if len(newJS) <= len(js) {
		t.Error("expected JS to be longer after injection")
	}

	count, ok := manifest["js_decoys_injected"].(int)
	if !ok {
		t.Fatalf("expected js_decoys_injected to be int, got %T", manifest["js_decoys_injected"])
	}
	if count < 3 || count > 10 {
		t.Errorf("manifest js_decoys_injected=%d, expected 3-10", count)
	}
}

// TestDecoyMetaTagsCount verifies 3-8 meta tags are injected.
func TestDecoyMetaTagsCount(t *testing.T) {
	di := NewLiveDecoyInjector()

	html := `<html><head><title>Test</title></head><body><div>Content</div></body></html>`
	css := `body { margin: 0; }`
	js := `console.log("app");`

	origMeta := strings.Count(html, "<meta")

	newHTML, _, _, manifest, err := di.InjectDecoys(html, css, js, 42)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	newMeta := strings.Count(newHTML, "<meta")
	added := newMeta - origMeta

	if added < 3 || added > 8 {
		t.Errorf("expected 3-8 meta tags injected, got %d", added)
	}

	count, ok := manifest["meta_tags_injected"].(int)
	if !ok {
		t.Fatalf("expected meta_tags_injected to be int, got %T", manifest["meta_tags_injected"])
	}
	if count < 3 || count > 8 {
		t.Errorf("manifest meta_tags_injected=%d, expected 3-8", count)
	}
}

// TestDecoyHiddenElementTechniques verifies all hidden elements use valid hiding techniques.
func TestDecoyHiddenElementTechniques(t *testing.T) {
	di := NewLiveDecoyInjector()

	html := `<html><head></head><body><div>A</div><div>B</div><div>C</div><div>D</div><div>E</div><div>F</div><div>G</div><div>H</div><div>I</div><div>J</div></body></html>`
	css := ``
	js := ``

	newHTML, _, _, _, err := di.InjectDecoys(html, css, js, 42)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Valid hiding techniques
	validTechniques := []string{
		`display:none`,
		`visibility:hidden`,
		`position:absolute;left:-9999px`,
		`width:0;height:0;overflow:hidden`,
		`opacity:0;pointer-events:none`,
	}

	// Find all decoy elements (they have decoy in class or id)
	decoyPattern := regexp.MustCompile(`<(div|span|section|aside)\s+[^>]*(decoy[^>]*)>`)
	matches := decoyPattern.FindAllString(newHTML, -1)

	for _, match := range matches {
		hasValidTechnique := false
		for _, technique := range validTechniques {
			if strings.Contains(match, technique) {
				hasValidTechnique = true
				break
			}
		}
		if !hasValidTechnique {
			t.Errorf("hidden element missing valid hiding technique: %s", match)
		}
	}
}

// TestDecoyDeterminism verifies same inputs + same seed = identical output.
func TestDecoyDeterminism(t *testing.T) {
	di := NewLiveDecoyInjector()

	html := `<html><head></head><body><div>Content</div><p>Text</p></body></html>`
	css := `body { margin: 0; }`
	js := `console.log("app");`

	var outputs [5][3]string

	for i := 0; i < 5; i++ {
		newHTML, newCSS, newJS, _, err := di.InjectDecoys(html, css, js, 12345)
		if err != nil {
			t.Fatalf("run %d failed: %v", i, err)
		}
		outputs[i] = [3]string{newHTML, newCSS, newJS}
	}

	for i := 1; i < 5; i++ {
		if outputs[i][0] != outputs[0][0] {
			t.Errorf("HTML output %d differs from first", i)
		}
		if outputs[i][1] != outputs[0][1] {
			t.Errorf("CSS output %d differs from first", i)
		}
		if outputs[i][2] != outputs[0][2] {
			t.Errorf("JS output %d differs from first", i)
		}
	}
}

// TestDecoyDivergence verifies different seeds produce different output with 20%+ comment count variation.
func TestDecoyDivergence(t *testing.T) {
	di := NewLiveDecoyInjector()

	html := `<html><head></head><body><div>Content</div><p>Text</p><span>More</span></body></html>`
	css := `body { margin: 0; }`
	js := `console.log("app");`

	commentCounts := make(map[int]bool)
	origComments := strings.Count(html, "<!--")

	for seed := int64(0); seed < 10; seed++ {
		newHTML, _, _, _, err := di.InjectDecoys(html, css, js, seed)
		if err != nil {
			t.Fatalf("seed %d failed: %v", seed, err)
		}
		count := strings.Count(newHTML, "<!--") - origComments
		commentCounts[count] = true
	}

	// At least 3 different comment counts across 10 seeds (20%+ variation)
	if len(commentCounts) < 3 {
		t.Errorf("expected at least 3 different comment counts across 10 seeds, got %d", len(commentCounts))
	}
}

// TestDecoyValidHTMLComments verifies all injected comments are properly formed.
func TestDecoyValidHTMLComments(t *testing.T) {
	di := NewLiveDecoyInjector()

	html := `<html><head></head><body><div>Content</div><p>Text</p></body></html>`
	css := ``
	js := ``

	newHTML, _, _, _, err := di.InjectDecoys(html, css, js, 42)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Every <!-- must have a matching -->
	openCount := strings.Count(newHTML, "<!--")
	closeCount := strings.Count(newHTML, "-->")
	if openCount != closeCount {
		t.Errorf("mismatched comment tags: %d opens, %d closes", openCount, closeCount)
	}

	// No nested comments
	commentRegex := regexp.MustCompile(`<!--[\s\S]*?-->`)
	comments := commentRegex.FindAllString(newHTML, -1)
	for _, c := range comments {
		inner := c[4 : len(c)-3] // strip <!-- and -->
		if strings.Contains(inner, "<!--") || strings.Contains(inner, "-->") {
			t.Errorf("nested comment detected: %s", c)
		}
	}
}

// TestDecoyValidCSSRules verifies injected CSS rules have selector + braces + properties.
func TestDecoyValidCSSRules(t *testing.T) {
	di := NewLiveDecoyInjector()

	html := `<html><head></head><body><div>Content</div></body></html>`
	css := ``
	js := ``

	_, newCSS, _, _, err := di.InjectDecoys(html, css, js, 42)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Each rule should have { and } and at least one property with :
	ruleRegex := regexp.MustCompile(`[.#][^\{]+\{[^}]+\}`)
	rules := ruleRegex.FindAllString(newCSS, -1)

	if len(rules) < 10 {
		t.Errorf("expected at least 10 CSS rules, found %d", len(rules))
	}

	for _, rule := range rules {
		if !strings.Contains(rule, ":") {
			t.Errorf("CSS rule missing property: %s", rule)
		}
		if !strings.Contains(rule, ";") {
			t.Errorf("CSS rule missing semicolon: %s", rule)
		}
	}
}

// TestDecoyManifestCompleteness verifies all required manifest fields are present.
func TestDecoyManifestCompleteness(t *testing.T) {
	di := NewLiveDecoyInjector()

	html := `<html><head></head><body><div>Content</div></body></html>`
	css := `body { margin: 0; }`
	js := `console.log("app");`

	_, _, _, manifest, err := di.InjectDecoys(html, css, js, 42)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	requiredFields := []string{
		"strategy",
		"html_comments_injected",
		"hidden_elements_injected",
		"unused_css_rules_injected",
		"js_decoys_injected",
		"meta_tags_injected",
	}

	for _, field := range requiredFields {
		if _, ok := manifest[field]; !ok {
			t.Errorf("manifest missing required field %q", field)
		}
	}

	if manifest["strategy"] != "live" {
		t.Errorf("expected strategy 'live', got %q", manifest["strategy"])
	}

	// Verify integer types
	intFields := []string{
		"html_comments_injected", "hidden_elements_injected",
		"unused_css_rules_injected", "js_decoys_injected", "meta_tags_injected",
	}
	for _, field := range intFields {
		if _, ok := manifest[field].(int); !ok {
			t.Errorf("expected %s to be int, got %T", field, manifest[field])
		}
	}
}

// TestDecoyInterfaceCompatibility verifies LiveDecoyInjector implements DecoyInjector.
func TestDecoyInterfaceCompatibility(t *testing.T) {
	var _ DecoyInjector = &LiveDecoyInjector{}

	di := NewLiveDecoyInjector()
	_, _, _, manifest, err := di.InjectDecoys("<html><body></body></html>", "", "", 42)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if manifest == nil {
		t.Error("expected non-nil manifest")
	}
}

// TestDecoyEmptyInput verifies empty inputs return without error.
func TestDecoyEmptyInput(t *testing.T) {
	di := NewLiveDecoyInjector()

	testCases := []struct {
		name string
		html string
		css  string
		js   string
	}{
		{"all_empty", "", "", ""},
		{"html_only", "<html><body><div>X</div></body></html>", "", ""},
		{"css_only", "", "body { margin: 0; }", ""},
		{"js_only", "", "", "console.log('x');"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			_, _, _, manifest, err := di.InjectDecoys(tc.html, tc.css, tc.js, 42)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if manifest == nil {
				t.Error("expected non-nil manifest")
			}
			if manifest["strategy"] != "live" {
				t.Errorf("expected strategy 'live', got %q", manifest["strategy"])
			}
		})
	}
}

// TestDecoyHiddenElementContent verifies hidden elements contain text, not empty tags.
func TestDecoyHiddenElementContent(t *testing.T) {
	di := NewLiveDecoyInjector()

	// Use enough elements to get several insertion points
	html := `<html><head></head><body><div>A</div><div>B</div><div>C</div><div>D</div><div>E</div><div>F</div><div>G</div><div>H</div></body></html>`
	css := ``
	js := ``

	newHTML, _, _, _, err := di.InjectDecoys(html, css, js, 42)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Find hidden elements by their hiding styles
	hiddenPatterns := []string{
		`display:none`,
		`visibility:hidden`,
		`position:absolute;left:-9999px`,
		`width:0;height:0;overflow:hidden`,
		`opacity:0;pointer-events:none`,
	}

	for _, pattern := range hiddenPatterns {
		if !strings.Contains(newHTML, pattern) {
			continue // This technique may not have been used with this seed
		}
		// Find the element containing this pattern
		idx := strings.Index(newHTML, pattern)
		if idx == -1 {
			continue
		}
		// Find the closing tag
		afterPattern := newHTML[idx:]
		closeIdx := strings.Index(afterPattern, "</")
		if closeIdx == -1 {
			continue
		}
		elementContent := afterPattern[:closeIdx]
		// The content between the style and the close tag should not be empty
		// (after stripping the opening tag)
		gtIdx := strings.Index(elementContent, ">")
		if gtIdx != -1 {
			innerContent := elementContent[gtIdx+1:]
			innerContent = strings.TrimSpace(innerContent)
			if len(innerContent) == 0 {
				t.Errorf("hidden element with %s has empty content", pattern)
			}
		}
	}
}

// TestDecoyMultipleSeeds runs across multiple seeds to verify range compliance.
func TestDecoyMultipleSeeds(t *testing.T) {
	di := NewLiveDecoyInjector()

	html := `<html><head></head><body><div>A</div><div>B</div><div>C</div><p>D</p><span>E</span></body></html>`
	css := `body { margin: 0; }`
	js := `var x = 1;`

	for seed := int64(0); seed < 20; seed++ {
		t.Run(fmt.Sprintf("seed_%d", seed), func(t *testing.T) {
			_, _, _, manifest, err := di.InjectDecoys(html, css, js, seed)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			comments := manifest["html_comments_injected"].(int)
			hidden := manifest["hidden_elements_injected"].(int)
			cssRules := manifest["unused_css_rules_injected"].(int)
			jsDecoys := manifest["js_decoys_injected"].(int)
			meta := manifest["meta_tags_injected"].(int)

			if comments < 5 || comments > 20 {
				t.Errorf("html_comments_injected=%d, expected 5-20", comments)
			}
			if hidden < 3 || hidden > 10 {
				t.Errorf("hidden_elements_injected=%d, expected 3-10", hidden)
			}
			if cssRules < 10 || cssRules > 30 {
				t.Errorf("unused_css_rules_injected=%d, expected 10-30", cssRules)
			}
			if jsDecoys < 3 || jsDecoys > 10 {
				t.Errorf("js_decoys_injected=%d, expected 3-10", jsDecoys)
			}
			if meta < 3 || meta > 8 {
				t.Errorf("meta_tags_injected=%d, expected 3-8", meta)
			}
		})
	}
}
