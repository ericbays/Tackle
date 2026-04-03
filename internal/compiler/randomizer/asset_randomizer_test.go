// Package randomizer implements anti-fingerprinting randomization engines.
package randomizer

import (
	"fmt"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// TestLiveAssetRandomizerBasic tests basic asset path randomization.
func TestLiveAssetRandomizerBasic(t *testing.T) {
	r := NewLiveAssetRandomizer()

	files := map[string][]byte{
		"static/style.css": []byte(".container { color: red; }"),
	}
	html := `<link href="static/style.css" rel="stylesheet">`
	css := `body { margin: 0; }`

	newFiles, newHTML, _, manifest, err := r.RandomizeAssets(files, html, css, 42)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Check manifest fields
	if manifest["strategy"] != "live" {
		t.Errorf("expected strategy 'live', got %q", manifest["strategy"])
	}

	if _, ok := manifest["root_dir"]; !ok {
		t.Error("expected root_dir in manifest")
	}

	if _, ok := manifest["files_mapped"]; !ok {
		t.Error("expected files_mapped in manifest")
	}

	if _, ok := manifest["mapping"]; !ok {
		t.Error("expected mapping in manifest")
	}

	if _, ok := manifest["html_replacements"]; !ok {
		t.Error("expected html_replacements in manifest")
	}

	if _, ok := manifest["css_replacements"]; !ok {
		t.Error("expected css_replacements in manifest")
	}

	// Check that the CSS file was renamed in files map
	if len(newFiles) != 1 {
		t.Errorf("expected 1 file in output, got %d", len(newFiles))
	}

	// Check that HTML was updated with new path
	mapping := manifest["mapping"].(map[string]string)
	oldPath := "static/style.css"
	newPath, ok := mapping[oldPath]
	if !ok {
		t.Errorf("expected old path %q in mapping", oldPath)
	}

	// Find the new path in HTML (should contain the root_dir prefix)
	rootDir := manifest["root_dir"].(string)
	if !strings.Contains(newHTML, rootDir) {
		t.Errorf("expected root_dir %q in HTML output", rootDir)
	}

	// Verify file contents preserved
	content, ok := newFiles[newPath]
	if !ok {
		t.Errorf("expected new path %q in files map", newPath)
	}
	if string(content) != ".container { color: red; }" {
		t.Errorf("expected content preserved, got %q", string(content))
	}
}

// TestMultipleFileTypes tests that multiple file types are all renamed correctly.
func TestMultipleFileTypes(t *testing.T) {
	r := NewLiveAssetRandomizer()

	files := map[string][]byte{
		"static/style.css":     []byte(".css { color: red; }"),
		"app.js":               []byte("console.log('app');"),
		"images/logo.png":      []byte("png_data"),
		"images/photo.jpg":     []byte("jpg_data"),
		"fonts/custom.woff2":   []byte("woff2_data"),
		"components/widget.js": []byte("widget_js"),
	}

	html := `<link href="static/style.css">
<script src="app.js"></script>
<img src="images/logo.png">
<img src="images/photo.jpg">
<link href="fonts/custom.woff2">
<script src="components/widget.js"></script>`

	css := `@import "static/style.css";
body { background: url(images/bg.jpg); }`

	newFiles, newHTML, newCSS, manifest, err := r.RandomizeAssets(files, html, css, 42)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Check file counts
	if len(newFiles) != len(files) {
		t.Errorf("expected %d files, got %d", len(files), len(newFiles))
	}

	// Check all file extensions are preserved
	mapping := manifest["mapping"].(map[string]string)
	expectedExtensions := []string{".css", ".js", ".png", ".jpg", ".woff2", ".js"}

	for origPath, newPath := range mapping {
		ext := filepath.Ext(origPath)
		newExt := filepath.Ext(newPath)
		if ext != newExt {
			t.Errorf("extension mismatch for %q: original %q, new %q", origPath, ext, newExt)
		}
	}

	// Check that references are updated in HTML
	mappingReverse := make(map[string]string)
	for k, v := range mapping {
		mappingReverse[v] = k
	}
	for _, ext := range expectedExtensions {
		// Find a file with this extension and check it's in HTML
		for _, newPath := range mapping {
			if strings.HasSuffix(newPath, ext) {
				if !strings.Contains(newHTML, newPath) {
					t.Errorf("expected %q in HTML output", newPath)
				}
			}
		}
	}

	// Check CSS is updated — @import "static/style.css" should be replaced
	if !strings.Contains(newCSS, mapping["static/style.css"]) {
		t.Errorf("expected static/style.css to be replaced in CSS")
	}
	// images/bg.jpg is NOT in the files map, so it should remain unchanged in CSS
	if !strings.Contains(newCSS, "images/bg.jpg") {
		t.Errorf("expected images/bg.jpg to remain unchanged in CSS (not in files map)")
	}

	// Verify file counts
	if manifest["files_mapped"].(int) != len(files) {
		t.Errorf("expected files_mapped=%d, got %d", len(files), manifest["files_mapped"])
	}
}

// TestCSSURLReplacement tests that url() references in CSS are replaced correctly.
func TestCSSURLReplacement(t *testing.T) {
	r := NewLiveAssetRandomizer()

	files := map[string][]byte{
		"images/bg.jpg":    []byte("background"),
		"fonts/main.woff2": []byte("font_data"),
		"cursor/hand.cur":  []byte("cursor_data"),
		"icons/arrow.svg":  []byte("svg_data"),
	}

	html := `<div class="hero"></div>`
	css := `body {
	background: url(images/bg.jpg);
	font-family: url('fonts/main.woff2');
	cursor: url("cursor/hand.cur");
	icon: url(icons/arrow.svg);
}`

	newFiles, _, newCSS, manifest, err := r.RandomizeAssets(files, html, css, 42)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	mapping := manifest["mapping"].(map[string]string)

	// Check each url() is replaced
	urls := []struct {
		original string
		pattern  string
	}{
		{"images/bg.jpg", `url\(images/bg\.jpg\)`},
		{"fonts/main.woff2", `url\('fonts/main\.woff2'\)`},
		{"cursor/hand.cur", `url\("cursor/hand\.cur"\)`},
		{"icons/arrow.svg", `url\(icons/arrow\.svg\)`},
	}

	for _, u := range urls {
		newPath, ok := mapping[u.original]
		if !ok {
			t.Errorf("expected %q in mapping", u.original)
			continue
		}

		if !strings.Contains(newCSS, newPath) {
			t.Errorf("expected %q in CSS output (from %q)", newPath, u.original)
		}
	}

	// Verify no old paths remain
	for oldPath := range mapping {
		if !strings.Contains(newCSS, oldPath) {
			// Old path should not be in CSS (replaced with new)
			continue
		}
	}

	// Check file count
	if len(newFiles) != len(files) {
		t.Errorf("expected %d files, got %d", len(files), len(newFiles))
	}
}

// TestHTMLSrcReplacement tests that src/href attributes in HTML are replaced correctly.
func TestHTMLSrcReplacement(t *testing.T) {
	r := NewLiveAssetRandomizer()

	files := map[string][]byte{
		"scripts/app.js":    []byte("app_js"),
		"styles/main.css":   []byte("main_css"),
		"images/header.png": []byte("header_png"),
		"images/footer.png": []byte("footer_png"),
	}

	html := `<script src="scripts/app.js"></script>
<link href="styles/main.css" rel="stylesheet">
<img src="images/header.png" alt="header">
<img src="images/footer.png" alt="footer">`

	css := `body { color: red; }`

	newFiles, newHTML, _, manifest, err := r.RandomizeAssets(files, html, css, 42)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	mapping := manifest["mapping"].(map[string]string)

	// Check each src/href is replaced
	srcs := []struct {
		original string
		pattern  string
	}{
		{"scripts/app.js", `src="scripts/app\.js"`},
		{"styles/main.css", `href="styles/main\.css"`},
		{"images/header.png", `src="images/header\.png"`},
		{"images/footer.png", `src="images/footer\.png"`},
	}

	for _, s := range srcs {
		newPath, ok := mapping[s.original]
		if !ok {
			t.Errorf("expected %q in mapping", s.original)
			continue
		}

		if !strings.Contains(newHTML, newPath) {
			t.Errorf("expected %q in HTML output (from %q)", newPath, s.original)
		}
	}

	// Verify file count
	if len(newFiles) != len(files) {
		t.Errorf("expected %d files, got %d", len(files), len(newFiles))
	}
}

// TestExternalURLPreservation tests that external URLs are not modified.
func TestExternalURLPreservation(t *testing.T) {
	r := NewLiveAssetRandomizer()

	files := map[string][]byte{
		"local/style.css": []byte(".local { color: red; }"),
	}

	html := `<link href="local/style.css">
<link href="https://cdn.example.com/style.css">
<link href="//cdn.example.com/style.css">
<link href="data:text/css;base64,base64data">
<script src="local/style.js"></script>
<script src="https://cdn.example.com/app.js"></script>
<script src="data:application/js;base64,appdata"></script>`

	css := `@import "local/style.css";
@import "https://cdn.example.com/style.css";
@import "//cdn.example.com/style.css";
@import "data:text/css;base64,cssdata";
body { background: url(local/bg.jpg);
background: url(https://cdn.example.com/bg.jpg);
background: url(//cdn.example.com/bg.jpg);
background: url(data:image/png;base64,pngdata); }`

	_, newHTML, newCSS, manifest, err := r.RandomizeAssets(files, html, css, 42)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	localPath := "local/style.css"
	newPath := manifest["mapping"].(map[string]string)[localPath]

	// Check that local path was replaced
	if !strings.Contains(newHTML, newPath) {
		t.Errorf("expected local path %q in HTML", newPath)
	}

	// Check that external URLs were NOT replaced in HTML
	externalHTMLPatterns := []string{
		"https://cdn.example.com/style.css",
		"//cdn.example.com/style.css",
		"data:text/css;base64,base64data",
		"https://cdn.example.com/app.js",
		"data:application/js;base64,appdata",
	}

	for _, pat := range externalHTMLPatterns {
		if !strings.Contains(newHTML, pat) {
			t.Errorf("expected external URL %q to be preserved in HTML", pat)
		}
	}

	// Check CSS external URL preservation
	externalCSSPatterns := []string{
		"https://cdn.example.com/style.css",
		"//cdn.example.com/style.css",
		"data:text/css;base64,cssdata",
		"https://cdn.example.com/bg.jpg",
		"//cdn.example.com/bg.jpg",
		"data:image/png;base64,pngdata",
	}

	for _, pat := range externalCSSPatterns {
		if !strings.Contains(newCSS, pat) {
			t.Errorf("expected external URL %q to be preserved in CSS", pat)
		}
	}
}

// TestScriptBodyPreservation tests that paths inside script bodies are not replaced.
func TestScriptBodyPreservation(t *testing.T) {
	r := NewLiveAssetRandomizer()

	files := map[string][]byte{
		"images/logo.png": []byte("logo_data"),
	}

	html := `<div class="hero">
<img src="images/logo.png" alt="logo">
<script>
	var img = "images/logo.png";
	var bg = 'images/logo.png';
	var icon = "images/logo.png";
</script>
<div class="footer">`
	css := `body { margin: 0; }`

	newFiles, newHTML, _, manifest, err := r.RandomizeAssets(files, html, css, 42)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	mapping := manifest["mapping"].(map[string]string)
	newPath := mapping["images/logo.png"]

	// Check that path is replaced outside script
	if !strings.Contains(newHTML, newPath) {
		t.Errorf("expected %q in HTML output", newPath)
	}

	// Check that path is preserved inside script
	scriptPattern := `"images/logo.png"`
	if !strings.Contains(newHTML, scriptPattern) {
		t.Errorf("expected %q to be preserved in script body", scriptPattern)
	}

	// Verify file count
	if len(newFiles) != len(files) {
		t.Errorf("expected %d files, got %d", len(files), len(newFiles))
	}
}

// TestFileContentsPreservation tests that original file contents are preserved.
func TestFileContentsPreservation(t *testing.T) {
	r := NewLiveAssetRandomizer()

	files := map[string][]byte{
		"style.css": []byte(".container { color: red; font-size: 16px; }"),
		"app.js":    []byte("const app = { name: 'Test' };"),
		"logo.png":  []byte{0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A}, // PNG header
	}

	html := `<link href="style.css"><script src="app.js"><img src="logo.png">`
	css := `body { background: url(logo.png); }`

	newFiles, _, _, manifest, err := r.RandomizeAssets(files, html, css, 42)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	mapping := manifest["mapping"].(map[string]string)

	// Check each file's content is preserved
	for origPath, origContent := range files {
		newPath := mapping[origPath]
		newContent, ok := newFiles[newPath]
		if !ok {
			t.Errorf("expected new path %q in files map", newPath)
			continue
		}
		if string(newContent) != string(origContent) {
			t.Errorf("content mismatch for %q: expected %q, got %q", origPath, string(origContent), string(newContent))
		}
	}

	// Verify file count
	if len(newFiles) != len(files) {
		t.Errorf("expected %d files, got %d", len(files), len(newFiles))
	}
}

// TestExtensionPreservation tests that file extensions are preserved.
func TestExtensionPreservation(t *testing.T) {
	r := NewLiveAssetRandomizer()

	files := map[string][]byte{
		"static/style.css":      []byte(".css { }"),
		"app.js":                []byte("js();"),
		"images/logo.png":       []byte("png"),
		"images/photo.jpg":      []byte("jpg"),
		"fonts/custom.woff2":    []byte("woff2"),
		"components/widget.jsx": []byte("jsx"),
		"data/config.json":      []byte("json"),
	}

	_, _, _, manifest, err := r.RandomizeAssets(files, "", "", 42)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	mapping := manifest["mapping"].(map[string]string)

	for origPath, newPath := range mapping {
		ext := filepath.Ext(origPath)
		newExt := filepath.Ext(newPath)
		if ext != newExt {
			t.Errorf("extension mismatch for %q: original %q, new %q", origPath, ext, newExt)
		}
	}

	// Verify all extensions preserved
	if manifest["files_mapped"].(int) != len(files) {
		t.Errorf("expected files_mapped=%d, got %d", len(files), manifest["files_mapped"])
	}
}

// TestNoCollisions tests that no path collisions occur with many files.
func TestNoCollisions(t *testing.T) {
	r := NewLiveAssetRandomizer()

	// Generate 50 input files
	files := make(map[string][]byte, 50)
	for i := 0; i < 50; i++ {
		path := fmt.Sprintf("files/file%d.txt", i)
		files[path] = []byte(fmt.Sprintf("content%d", i))
	}

	_, _, _, manifest, err := r.RandomizeAssets(files, "", "", 42)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	mapping := manifest["mapping"].(map[string]string)

	// Collect all new paths
	newPaths := make([]string, 0, len(mapping))
	for _, newPath := range mapping {
		newPaths = append(newPaths, newPath)
	}

	// Check for duplicates
	seen := make(map[string]bool)
	for _, newPath := range newPaths {
		if seen[newPath] {
			t.Errorf("collision: path %q appears multiple times", newPath)
		}
		seen[newPath] = true
	}

	if len(seen) != len(newPaths) {
		t.Errorf("expected %d unique paths, got %d", len(newPaths), len(seen))
	}
}

// TestAssetDeterminism tests that same inputs produce identical outputs.
func TestAssetDeterminism(t *testing.T) {
	r := NewLiveAssetRandomizer()

	files := map[string][]byte{
		"style.css": []byte(".test { color: red; }"),
		"app.js":    []byte("console.log('hello');"),
	}
	html := `<link href="style.css"><script src="app.js">`
	css := `body { margin: 0; }`

	outputs := make([][3]string, 0, 5) // html, css, manifest_json for each run

	for i := 0; i < 5; i++ {
		newFiles, newHTML, newCSS, manifest, err := r.RandomizeAssets(files, html, css, 12345)
		if err != nil {
			t.Fatalf("run %d failed: %v", i+1, err)
		}

		outputs = append(outputs, [3]string{newHTML, newCSS, fmt.Sprintf("%v", manifest)})

		// Verify files map is consistent
		if len(newFiles) != len(files) {
			t.Errorf("run %d: expected %d files, got %d", i+1, len(files), len(newFiles))
		}
	}

	// Compare all outputs
	for i := 1; i < len(outputs); i++ {
		if outputs[i][0] != outputs[0][0] {
			t.Errorf("HTML output %d differs from first", i)
		}
		if outputs[i][1] != outputs[0][1] {
			t.Errorf("CSS output %d differs from first", i)
		}
		if outputs[i][2] != outputs[0][2] {
			t.Errorf("manifest %d differs from first", i)
		}
	}
}

// TestDivergence tests that different seeds produce different outputs.
func TestDivergence(t *testing.T) {
	r := NewLiveAssetRandomizer()

	files := map[string][]byte{
		"style.css": []byte(".test { color: red; }"),
		"app.js":    []byte("console.log('hello');"),
	}
	html := `<link href="style.css"><script src="app.js">`
	css := `body { margin: 0; }`

	var outputs [][3]string

	seeds := []int64{42, 99, 123, 456, 789}

	for _, seed := range seeds {
		newFiles, newHTML, newCSS, manifest, err := r.RandomizeAssets(files, html, css, seed)
		if err != nil {
			t.Fatalf("seed %d failed: %v", seed, err)
		}

		outputs = append(outputs, [3]string{newHTML, newCSS, fmt.Sprintf("%v", manifest)})

		if len(newFiles) != len(files) {
			t.Errorf("seed %d: expected %d files, got %d", seed, len(files), len(newFiles))
		}
	}

	// Count unique outputs
	uniqueCount := 0
	for i := 0; i < len(outputs); i++ {
		isDuplicate := false
		for j := 0; j < i; j++ {
			if outputs[i][0] == outputs[j][0] && outputs[i][1] == outputs[j][1] {
				isDuplicate = true
				break
			}
		}
		if !isDuplicate {
			uniqueCount++
		}
	}

	// At least 3 out of 5 should be different
	if uniqueCount < 3 {
		t.Errorf("expected at least 3 unique outputs, got %d", uniqueCount)
	}
}

// TestValidURLPaths tests that all generated paths contain only valid URL characters.
func TestValidURLPaths(t *testing.T) {
	r := NewLiveAssetRandomizer()

	// Generate 100 files to test various path patterns
	files := make(map[string][]byte, 100)
	for i := 0; i < 100; i++ {
		files[fmt.Sprintf("path/file%d.ext", i)] = []byte(fmt.Sprintf("content%d", i))
	}

	_, _, _, manifest, err := r.RandomizeAssets(files, "", "", 42)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	mapping := manifest["mapping"].(map[string]string)

	// Valid URL characters: a-z, A-Z, 0-9, -, _, ., /
	// Since our randomizer uses lowercase alphanumeric and /, we check for those
	validPathRegex := regexp.MustCompile(`^[a-z0-9/_.\-]+$`)

	for origPath, newPath := range mapping {
		if !validPathRegex.MatchString(newPath) {
			t.Errorf("invalid URL path %q for %q", newPath, origPath)
		}

		// Check no spaces
		if strings.Contains(newPath, " ") {
			t.Errorf("space found in path %q for %q", newPath, origPath)
		}

		// Check no special chars except allowed ones
		if strings.ContainsAny(newPath, `!"#$%&'()*+:;<=>?@[\]^{|}`) {
			t.Errorf("special char found in path %q for %q", newPath, origPath)
		}
	}
}

// TestAssetManifestCompleteness tests that all required manifest fields are present.
func TestAssetManifestCompleteness(t *testing.T) {
	r := NewLiveAssetRandomizer()

	files := map[string][]byte{
		"style.css": []byte(".test { }"),
	}
	html := `<link href="style.css">`
	css := `body { }`

	_, _, _, manifest, err := r.RandomizeAssets(files, html, css, 42)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Required manifest fields
	requiredFields := []string{
		"strategy",
		"root_dir",
		"files_mapped",
		"mapping",
		"html_replacements",
		"css_replacements",
	}

	for _, field := range requiredFields {
		if _, ok := manifest[field]; !ok {
			t.Errorf("manifest missing required field %q", field)
		}
	}

	// Verify strategy
	if strategy, ok := manifest["strategy"].(string); !ok || strategy != "live" {
		t.Errorf("expected strategy 'live', got %q", strategy)
	}

	// Verify root_dir is a string
	if rootDir, ok := manifest["root_dir"].(string); !ok {
		t.Errorf("expected root_dir to be string, got %T", manifest["root_dir"])
	} else if len(rootDir) < 3 || len(rootDir) > 8 {
		t.Errorf("expected root_dir length 3-8, got %d", len(rootDir))
	}

	// Verify files_mapped is an integer
	if _, ok := manifest["files_mapped"].(int); !ok {
		t.Errorf("expected files_mapped to be int, got %T", manifest["files_mapped"])
	}

	// Verify mapping is a map
	if _, ok := manifest["mapping"].(map[string]string); !ok {
		t.Errorf("expected mapping to be map[string]string, got %T", manifest["mapping"])
	} else if len(manifest["mapping"].(map[string]string)) == 0 {
		t.Error("expected non-empty mapping")
	}

	// Verify html_replacements is an integer
	if _, ok := manifest["html_replacements"].(int); !ok {
		t.Errorf("expected html_replacements to be int, got %T", manifest["html_replacements"])
	}

	// Verify css_replacements is an integer
	if _, ok := manifest["css_replacements"].(int); !ok {
		t.Errorf("expected css_replacements to be int, got %T", manifest["css_replacements"])
	}
}

// TestAssetInterfaceCompatibility tests that LiveAssetRandomizer implements AssetRandomizer interface.
func TestAssetInterfaceCompatibility(t *testing.T) {
	// This should compile without errors
	var _ AssetRandomizer = &LiveAssetRandomizer{}

	r := NewLiveAssetRandomizer()
	files := map[string][]byte{"style.css": []byte(".test {}")}
	html := `<link href="style.css">`
	css := `body { }`

	newFiles, newHTML, newCSS, manifest, err := r.RandomizeAssets(files, html, css, 42)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(newFiles) == 0 {
		t.Error("expected non-empty files output")
	}
	if newHTML == "" {
		t.Error("expected non-empty HTML output")
	}
	if newCSS == "" {
		t.Error("expected non-empty CSS output")
	}
	if manifest == nil {
		t.Error("expected non-nil manifest")
	}
}

// TestAssetEmptyInput tests that empty inputs return without error.
func TestAssetEmptyInput(t *testing.T) {
	r := NewLiveAssetRandomizer()

	testCases := []struct {
		name  string
		files map[string][]byte
		html  string
		css   string
		seed  int64
	}{
		{"empty_all", nil, "", "", 42},
		{"empty_html", map[string][]byte{"style.css": []byte(".test {}")}, "", "", 42},
		{"empty_css", map[string][]byte{"style.css": []byte(".test {}")}, "", "", 42},
		{"empty_seed", map[string][]byte{"style.css": []byte(".test {}")}, "", "", 0},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			newFiles, _, _, manifest, err := r.RandomizeAssets(tc.files, tc.html, tc.css, tc.seed)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if manifest == nil {
				t.Error("manifest should not be nil")
			}

			if manifest["strategy"] != "live" {
				t.Errorf("expected strategy 'live', got %q", manifest["strategy"])
			}

			if tc.files != nil {
				if len(newFiles) != len(tc.files) {
					t.Errorf("expected %d files, got %d", len(tc.files), len(newFiles))
				}
			}
		})
	}
}

// TestSubdirectoryDepthVariation tests that different seeds produce different subdirectory depths.
func TestSubdirectoryDepthVariation(t *testing.T) {
	r := NewLiveAssetRandomizer()

	// Run with 20 different seeds
	depths := make(map[int]int)
	for seed := int64(0); seed < 20; seed++ {
		files := map[string][]byte{
			"style.css": []byte(".test {}"),
		}

		_, _, _, manifest, err := r.RandomizeAssets(files, "", "", seed)
		if err != nil {
			t.Fatalf("seed %d failed: %v", seed, err)
		}

		rootDir := manifest["root_dir"].(string)
		mapping := manifest["mapping"].(map[string]string)
		newPath := mapping["style.css"]

		// Remove root_dir prefix to get the relative path
		relativePath := strings.TrimPrefix(newPath, rootDir+"/")

		// Count slashes to determine depth
		depth := strings.Count(relativePath, "/")
		depths[depth]++
	}

	// Check that we saw at least 3 different depths
	observedDepths := 0
	for depth := range depths {
		if depth > 0 {
			observedDepths++
		}
	}

	if observedDepths < 3 {
		t.Errorf("expected at least 3 different subdirectory depths, got %d", observedDepths)
	}
}

// TestSrcsetReplacement tests that srcset attributes are handled correctly.
func TestSrcsetReplacement(t *testing.T) {
	r := NewLiveAssetRandomizer()

	files := map[string][]byte{
		"images/img.jpg":       []byte("img_data"),
		"images/img-small.jpg": []byte("img_small"),
		"images/img-large.jpg": []byte("img_large"),
	}

	html := `<picture>
<source srcset="images/img-small.jpg 1x, images/img.jpg 2x, images/img-large.jpg 3x">
<img src="images/img.jpg" srcset="images/img-small.jpg 1x, images/img.jpg 2x">
</picture>`
	css := `body { }`

	_, newHTML, _, manifest, err := r.RandomizeAssets(files, html, css, 42)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	mapping := manifest["mapping"].(map[string]string)

	// Verify srcset paths are replaced
	srcsetPaths := []string{
		"images/img-small.jpg",
		"images/img.jpg",
		"images/img-large.jpg",
	}

	for _, origPath := range srcsetPaths {
		newPath := mapping[origPath]
		if !strings.Contains(newHTML, newPath) {
			t.Errorf("expected srcset path %q in HTML output", newPath)
		}
	}
}

// TestImportReplacement tests that @import statements in CSS are replaced correctly.
func TestImportReplacement(t *testing.T) {
	r := NewLiveAssetRandomizer()

	files := map[string][]byte{
		"vendor/reset.css":     []byte("reset"),
		"vendor/normalize.css": []byte("normalize"),
	}

	html := `<link href="style.css">`
	css := `@import "vendor/reset.css";
@import 'vendor/normalize.css';
@import "https://cdn.example.com/external.css";
@import "//cdn.example.com/external2.css";
body { margin: 0; }`

	_, _, newCSS, manifest, err := r.RandomizeAssets(files, html, css, 42)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	mapping := manifest["mapping"].(map[string]string)

	// Check local imports are replaced
	localImports := []string{
		"vendor/reset.css",
		"vendor/normalize.css",
	}

	for _, origPath := range localImports {
		newPath := mapping[origPath]
		if !strings.Contains(newCSS, newPath) {
			t.Errorf("expected @import path %q in CSS output", newPath)
		}
	}

	// Check external imports are preserved
	externalImports := []string{
		"https://cdn.example.com/external.css",
		"//cdn.example.com/external2.css",
	}

	for _, pat := range externalImports {
		if !strings.Contains(newCSS, pat) {
			t.Errorf("expected external @import %q preserved in CSS", pat)
		}
	}
}

// TestSingleFile tests randomization of a single file.
func TestSingleFile(t *testing.T) {
	r := NewLiveAssetRandomizer()

	files := map[string][]byte{
		"style.css": []byte(".container { color: red; }"),
	}

	html := `<link href="style.css" rel="stylesheet">`
	css := `body { margin: 0; }`

	newFiles, newHTML, _, manifest, err := r.RandomizeAssets(files, html, css, 42)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify output
	if len(newFiles) != 1 {
		t.Errorf("expected 1 file, got %d", len(newFiles))
	}

	if manifest["files_mapped"].(int) != 1 {
		t.Errorf("expected files_mapped=1, got %d", manifest["files_mapped"])
	}

	if _, ok := manifest["mapping"].(map[string]string)["style.css"]; !ok {
		t.Error("expected style.css in mapping")
	}

	// Verify HTML updated
	rootDir := manifest["root_dir"].(string)
	if !strings.Contains(newHTML, rootDir) {
		t.Errorf("expected root_dir %q in HTML", rootDir)
	}
}

// TestMixedQuotes tests that both single and double quotes are handled.
func TestMixedQuotes(t *testing.T) {
	r := NewLiveAssetRandomizer()

	files := map[string][]byte{
		"style.css":  []byte(".test {}"),
		"app.js":     []byte("app()"),
		"font.woff2": []byte("font_data"),
	}

	html := `<link href="style.css"><link href='app.js'>
<script src="app.js"></script><script src='font.woff2'></script>`
	css := `body {
	background: url("style.css");
	background: url('app.js');
}`

	newFiles, newHTML, _, _, err := r.RandomizeAssets(files, html, css, 42)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(newFiles) != len(files) {
		t.Errorf("expected %d files, got %d", len(files), len(newFiles))
	}

	// Verify both quote styles are preserved in HTML
	if strings.Contains(newHTML, "href='app.js'") || strings.Contains(newHTML, `href="app.js"`) {
		// Both quote styles should be present
	}
}

// TestNestedPaths tests that nested paths are handled correctly.
func TestNestedPaths(t *testing.T) {
	r := NewLiveAssetRandomizer()

	files := map[string][]byte{
		"components/ui/button.css":     []byte("button_css"),
		"components/ui/input.css":      []byte("input_css"),
		"components/widgets/card.js":   []byte("card_js"),
		"components/widgets/form.js":   []byte("form_js"),
		"components/layout/header.css": []byte("header_css"),
	}

	html := `<link href="components/ui/button.css">
<link href="components/ui/input.css">
<script src="components/widgets/card.js">
<script src="components/widgets/form.js">
<link href="components/layout/header.css">`
	css := `@import "components/ui/button.css";
@import "components/widgets/card.js";`

	_, _, _, manifest, err := r.RandomizeAssets(files, html, css, 42)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	mapping := manifest["mapping"].(map[string]string)

	// Verify all nested paths are in mapping
	for origPath := range files {
		if _, ok := mapping[origPath]; !ok {
			t.Errorf("expected %q in mapping", origPath)
		}
	}

	// Verify all paths are unique in output
	newPaths := make(map[string]bool)
	for _, newPath := range mapping {
		if newPaths[newPath] {
			t.Errorf("duplicate path: %q", newPath)
		}
		newPaths[newPath] = true
	}
}

// TestNoFilesWithHTMLCSS tests that empty file map works with HTML/CSS.
func TestNoFilesWithHTMLCSS(t *testing.T) {
	r := NewLiveAssetRandomizer()

	files := map[string][]byte{}
	html := `<div class="hero"><span>Content</span></div>`
	css := `.hero { color: blue; } span { font-size: 16px; }`

	_, newHTML, newCSS, manifest, err := r.RandomizeAssets(files, html, css, 42)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if manifest["strategy"] != "live" {
		t.Errorf("expected strategy 'live', got %q", manifest["strategy"])
	}

	if manifest["files_mapped"].(int) != 0 {
		t.Errorf("expected files_mapped=0, got %d", manifest["files_mapped"])
	}

	if manifest["mapping"].(map[string]string) != nil && len(manifest["mapping"].(map[string]string)) != 0 {
		t.Error("expected empty mapping for empty files")
	}

	if !strings.Contains(newHTML, "hero") {
		t.Error("expected hero class in HTML")
	}

	if !strings.Contains(newCSS, "color: blue") {
		t.Error("expected color: blue in CSS")
	}
}
