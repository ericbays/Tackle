// Package randomizer implements anti-fingerprinting randomization engines.
// Tests for build uniqueness across different seeds.
package randomizer

import (
	"strings"
	"testing"
)

// sampleHTML provides a realistic landing page HTML for uniqueness testing.
const sampleHTML = `<!DOCTYPE html>
<html>
<head>
<meta charset="utf-8">
<title>Login</title>
<style>
.container { max-width: 600px; margin: 0 auto; }
.header { background: #333; color: white; padding: 20px; }
.form-group { margin: 10px 0; }
.btn-primary { background: blue; color: white; padding: 10px 20px; }
.footer { text-align: center; padding: 10px; }
</style>
</head>
<body>
<div class="header" data-comp-id="comp-1">
<h1>Welcome</h1>
</div>
<div class="container" data-comp-id="comp-2">
<form data-capture="login-form" action="/capture" method="POST">
<div class="form-group">
<label>Email</label>
<input type="email" name="email" data-capture-field="email" required>
</div>
<div class="form-group">
<label>Password</label>
<input type="password" name="password" data-capture-field="password" required>
</div>
<button type="submit" class="btn-primary">Sign In</button>
</form>
</div>
<div class="footer" data-comp-id="comp-3">
<p>Copyright 2026</p>
</div>
</body>
</html>`

// TestBuildUniqueness_DOMDiffersBySeed verifies that DOM randomization produces
// different output for different seeds.
func TestBuildUniqueness_DOMDiffersBySeed(t *testing.T) {
	r := NewLiveDOMRandomizer()
	seed1 := int64(12345)
	seed2 := int64(67890)

	out1, manifest1, err := r.RandomizeDOM(sampleHTML, seed1)
	if err != nil {
		t.Fatalf("DOM seed1: %v", err)
	}
	out2, manifest2, err := r.RandomizeDOM(sampleHTML, seed2)
	if err != nil {
		t.Fatalf("DOM seed2: %v", err)
	}

	if out1 == out2 {
		t.Error("DOM randomization produced identical output for different seeds")
	}

	// Manifests should record different decisions (compare scalar fields only).
	nd1, _ := manifest1["nesting_depth"].(int)
	nd2, _ := manifest2["nesting_depth"].(int)
	wa1, _ := manifest1["wrappers_added"].(int)
	wa2, _ := manifest2["wrappers_added"].(int)
	nw1, _ := manifest1["neutral_wrappers_added"].(int)
	nw2, _ := manifest2["neutral_wrappers_added"].(int)
	if nd1 == nd2 && wa1 == wa2 && nw1 == nw2 {
		t.Error("DOM manifests are identical — randomization may not be seed-dependent")
	}
}

// TestBuildUniqueness_DOMPreservesDataAttributes verifies that data-capture,
// data-comp-id, and data-capture-field attributes survive DOM randomization.
func TestBuildUniqueness_DOMPreservesDataAttributes(t *testing.T) {
	r := NewLiveDOMRandomizer()

	out, _, err := r.RandomizeDOM(sampleHTML, 42)
	if err != nil {
		t.Fatalf("DOM: %v", err)
	}

	attrs := []string{
		`data-capture="login-form"`,
		`data-comp-id="comp-1"`,
		`data-comp-id="comp-2"`,
		`data-comp-id="comp-3"`,
		`data-capture-field="email"`,
		`data-capture-field="password"`,
	}

	for _, attr := range attrs {
		if !strings.Contains(out, attr) {
			t.Errorf("DOM randomization stripped %q from output", attr)
		}
	}
}

// TestBuildUniqueness_CSSClassNamesZeroOverlap verifies that CSS class names
// differ completely between two builds with different seeds.
func TestBuildUniqueness_CSSClassNamesZeroOverlap(t *testing.T) {
	r := NewLiveCSSRandomizer()
	seed1 := int64(11111)
	seed2 := int64(99999)

	html1, _, _, err := r.RandomizeCSS(sampleHTML, "", seed1)
	if err != nil {
		t.Fatalf("CSS seed1: %v", err)
	}
	html2, _, _, err := r.RandomizeCSS(sampleHTML, "", seed2)
	if err != nil {
		t.Fatalf("CSS seed2: %v", err)
	}

	classes1 := extractClassNames(html1)
	classes2 := extractClassNames(html2)

	// Original class names should not be present in either output.
	originals := []string{"container", "header", "form-group", "btn-primary", "footer"}
	for _, orig := range originals {
		if classes1[orig] {
			t.Errorf("seed1 output still contains original class %q", orig)
		}
		if classes2[orig] {
			t.Errorf("seed2 output still contains original class %q", orig)
		}
	}

	// The two outputs should have zero overlapping class names
	// (excluding any common framework classes).
	overlap := 0
	for name := range classes1 {
		if classes2[name] {
			overlap++
		}
	}
	if overlap > 0 {
		t.Errorf("found %d overlapping class names between seed1 and seed2 builds", overlap)
	}
}

// TestBuildUniqueness_HeadersDifferBySeed verifies that header profiles differ
// between two builds with different seeds and have at least 3 differing values.
func TestBuildUniqueness_HeadersDifferBySeed(t *testing.T) {
	r := NewLiveHeaderRandomizer()
	seed1 := int64(22222)
	seed2 := int64(88888)

	profile1, src1, manifest1, err := r.GenerateHeaderProfile(seed1)
	if err != nil {
		t.Fatalf("Header seed1: %v", err)
	}
	profile2, src2, manifest2, err := r.GenerateHeaderProfile(seed2)
	if err != nil {
		t.Fatalf("Header seed2: %v", err)
	}

	// Count differences in header values.
	differences := 0
	allKeys := make(map[string]bool)
	for k := range profile1 {
		allKeys[k] = true
	}
	for k := range profile2 {
		allKeys[k] = true
	}

	for k := range allKeys {
		v1, ok1 := profile1[k]
		v2, ok2 := profile2[k]
		if ok1 != ok2 || v1 != v2 {
			differences++
		}
	}

	if differences < 3 {
		t.Errorf("expected >= 3 header differences between seeds, got %d", differences)
	}

	// Middleware source should differ.
	if src1 == src2 {
		t.Error("header middleware source identical for different seeds")
	}

	// Manifests should record different decisions.
	if manifest1["server_header"] == manifest2["server_header"] &&
		manifest1["x_powered_by"] == manifest2["x_powered_by"] &&
		manifest1["cache_control"] == manifest2["cache_control"] {
		t.Error("header manifests are identical — randomization may not be seed-dependent")
	}
}

// TestBuildUniqueness_AssetPathsDiffer verifies that asset paths are randomized
// differently per seed with zero overlap.
func TestBuildUniqueness_AssetPathsDiffer(t *testing.T) {
	r := NewLiveAssetRandomizer()
	seed1 := int64(33333)
	seed2 := int64(77777)

	files := map[string][]byte{
		"static/index.html": []byte(sampleHTML),
		"static/style.css":  []byte(`.container { color: red; }`),
		"static/app.js":     []byte(`console.log("hello")`),
	}

	out1, _, _, manifest1, err := r.RandomizeAssets(files, "", "", seed1)
	if err != nil {
		t.Fatalf("Asset seed1: %v", err)
	}
	out2, _, _, manifest2, err := r.RandomizeAssets(files, "", "", seed2)
	if err != nil {
		t.Fatalf("Asset seed2: %v", err)
	}

	// Collect output filenames.
	names1 := make(map[string]bool)
	for k := range out1 {
		names1[k] = true
	}
	names2 := make(map[string]bool)
	for k := range out2 {
		names2[k] = true
	}

	// No original filenames should survive.
	for orig := range files {
		if names1[orig] {
			t.Errorf("seed1 output still has original path %q", orig)
		}
		if names2[orig] {
			t.Errorf("seed2 output still has original path %q", orig)
		}
	}

	// Zero overlap in randomized paths.
	overlap := 0
	for name := range names1 {
		if names2[name] {
			overlap++
		}
	}
	if overlap > 0 {
		t.Errorf("found %d overlapping asset paths between seed1 and seed2", overlap)
	}

	// Manifests should differ.
	if manifest1["root_dir"] == manifest2["root_dir"] {
		t.Error("asset root_dir identical for different seeds")
	}
}

// TestBuildUniqueness_DecoysDiffer verifies decoy injection varies by seed.
func TestBuildUniqueness_DecoysDiffer(t *testing.T) {
	r := NewLiveDecoyInjector()
	seed1 := int64(44444)
	seed2 := int64(55555)

	out1, _, _, _, err := r.InjectDecoys(sampleHTML, "", "", seed1)
	if err != nil {
		t.Fatalf("Decoy seed1: %v", err)
	}
	out2, _, _, _, err := r.InjectDecoys(sampleHTML, "", "", seed2)
	if err != nil {
		t.Fatalf("Decoy seed2: %v", err)
	}

	if out1 == out2 {
		t.Error("decoy injection produced identical output for different seeds")
	}

	// Both should be longer than original (decoys added).
	if len(out1) <= len(sampleHTML) {
		t.Error("seed1 decoy output not longer than input — decoys may not have been injected")
	}
	if len(out2) <= len(sampleHTML) {
		t.Error("seed2 decoy output not longer than input — decoys may not have been injected")
	}
}

// TestBuildUniqueness_FullPipeline runs all randomizers in sequence (mimicking engine.go)
// with two different seeds and verifies zero common long substrings.
func TestBuildUniqueness_FullPipeline(t *testing.T) {
	dom := NewLiveDOMRandomizer()
	css := NewLiveCSSRandomizer()
	decoy := NewLiveDecoyInjector()
	header := NewLiveHeaderRandomizer()

	runPipeline := func(seed int64) (string, map[string]string) {
		html := sampleHTML

		// DOM randomization.
		randomized, _, err := dom.RandomizeDOM(html, seed)
		if err != nil {
			t.Fatalf("DOM seed %d: %v", seed, err)
		}
		html = randomized

		// CSS randomization.
		randomized, _, _, err = css.RandomizeCSS(html, "", seed)
		if err != nil {
			t.Fatalf("CSS seed %d: %v", seed, err)
		}
		html = randomized

		// Decoy injection.
		randomized, _, _, _, err = decoy.InjectDecoys(html, "", "", seed)
		if err != nil {
			t.Fatalf("Decoy seed %d: %v", seed, err)
		}
		html = randomized

		// Header profile.
		profile, _, _, err := header.GenerateHeaderProfile(seed)
		if err != nil {
			t.Fatalf("Header seed %d: %v", seed, err)
		}

		return html, profile
	}

	html1, headers1 := runPipeline(100)
	html2, headers2 := runPipeline(200)

	// Verify no common substrings of 200+ chars (excluding visible text like "Welcome", "Copyright 2026").
	if hasCommonLongSubstring(html1, html2, 200) {
		t.Error("two builds share a 200+ character substring — fingerprinting risk")
	}

	// Headers must differ.
	headerDiffs := 0
	allKeys := make(map[string]bool)
	for k := range headers1 {
		allKeys[k] = true
	}
	for k := range headers2 {
		allKeys[k] = true
	}
	for k := range allKeys {
		if headers1[k] != headers2[k] {
			headerDiffs++
		}
	}
	if headerDiffs < 3 {
		t.Errorf("expected >= 3 header differences, got %d", headerDiffs)
	}
}

// extractClassNames extracts all class names from HTML class="..." attributes.
func extractClassNames(html string) map[string]bool {
	result := make(map[string]bool)
	remaining := html
	for {
		idx := strings.Index(remaining, "class=\"")
		if idx < 0 {
			break
		}
		remaining = remaining[idx+7:]
		end := strings.Index(remaining, "\"")
		if end < 0 {
			break
		}
		classes := remaining[:end]
		for _, c := range strings.Fields(classes) {
			result[c] = true
		}
		remaining = remaining[end+1:]
	}
	return result
}

// hasCommonLongSubstring checks if two strings share any common substring of
// length >= minLen. Uses a simple sliding window approach.
func hasCommonLongSubstring(a, b string, minLen int) bool {
	if len(a) < minLen || len(b) < minLen {
		return false
	}
	// Build a set of all minLen-length substrings of a.
	subs := make(map[string]bool)
	for i := 0; i <= len(a)-minLen; i++ {
		subs[a[i:i+minLen]] = true
	}
	// Check if any minLen-length substring of b exists in the set.
	for i := 0; i <= len(b)-minLen; i++ {
		if subs[b[i:i+minLen]] {
			return true
		}
	}
	return false
}
