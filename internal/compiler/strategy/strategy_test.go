package strategy

import (
	"strings"
	"testing"
)

func testDefinition() PageDefinition {
	return PageDefinition{
		Pages: []PageDef{
			{
				ID: "home", Title: "Welcome", Path: "/", IsDefault: true,
				HTML: `<h1>Welcome</h1><p>Please log in.</p>`,
				CSS: "h1 { color: blue; }", JS: "console.log('home');",
				Forms: nil,
			},
			{
				ID: "login", Title: "Login", Path: "/login",
				HTML: `<h1>Login</h1>`,
				CSS: "form { margin: 20px; }", JS: "console.log('login');",
				Forms: []FormDef{
					{ID: "loginForm", Action: "/submit-login", Method: "POST", Fields: []string{"username", "password"}},
				},
			},
			{
				ID: "success", Title: "Success", Path: "/success",
				HTML: `<h1>Thank you!</h1>`,
				CSS: "", JS: "",
				Forms: nil,
			},
		},
		GlobalCSS:  "body { font-family: sans-serif; }",
		GlobalJS:   "console.log('global');",
		CampaignID: "campaign-123",
		BuildToken: "secret-build-token-456",
	}
}

// TestInterfaceCompatibility verifies all 3 strategy types implement CodeGenerator.
func TestInterfaceCompatibility(t *testing.T) {
	var _ CodeGenerator = &SPAGenerator{}
	var _ CodeGenerator = &MultiFileGenerator{}
	var _ CodeGenerator = &HybridGenerator{}
}

// TestSPAGeneratesExpectedFiles verifies SPA output contains main.go and index.html.
func TestSPAGeneratesExpectedFiles(t *testing.T) {
	gen := &SPAGenerator{}
	output, err := gen.Generate(testDefinition(), 42)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	requiredFiles := []string{"main.go", "index.html"}
	for _, f := range requiredFiles {
		if _, ok := output.Files[f]; !ok {
			t.Errorf("SPA missing expected file %q", f)
		}
	}

	if output.Strategy != StrategySPA {
		t.Errorf("expected strategy %q, got %q", StrategySPA, output.Strategy)
	}
}

// TestMultiFileGeneratesExpectedFiles verifies multi-file output has per-page HTML and static files.
func TestMultiFileGeneratesExpectedFiles(t *testing.T) {
	gen := &MultiFileGenerator{}
	output, err := gen.Generate(testDefinition(), 42)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	requiredFiles := []string{"main.go", "static/styles.css", "static/app.js"}
	for _, f := range requiredFiles {
		if _, ok := output.Files[f]; !ok {
			t.Errorf("MultiFile missing expected file %q", f)
		}
	}

	// Check per-page HTML files
	for _, page := range testDefinition().Pages {
		htmlPath := "pages/" + page.ID + ".html"
		// On Windows filepath.Join uses backslash, check both
		found := false
		for k := range output.Files {
			if strings.Replace(k, "\\", "/", -1) == htmlPath {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("MultiFile missing page HTML %q", htmlPath)
		}
	}

	if output.Strategy != StrategyMultiFile {
		t.Errorf("expected strategy %q, got %q", StrategyMultiFile, output.Strategy)
	}
}

// TestHybridGeneratesExpectedFiles verifies hybrid output has expected files.
func TestHybridGeneratesExpectedFiles(t *testing.T) {
	gen := &HybridGenerator{}
	output, err := gen.Generate(testDefinition(), 42)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	requiredFiles := []string{"main.go", "index.html", "static/app.js", "static/extra.css"}
	for _, f := range requiredFiles {
		if _, ok := output.Files[f]; !ok {
			t.Errorf("Hybrid missing expected file %q", f)
		}
	}

	if output.Strategy != StrategyHybrid {
		t.Errorf("expected strategy %q, got %q", StrategyHybrid, output.Strategy)
	}
}

// TestDeterminism verifies same definition + same seed = identical output.
func TestStrategyDeterminism(t *testing.T) {
	strategies := []struct {
		name string
		gen  CodeGenerator
	}{
		{"spa", &SPAGenerator{}},
		{"multifile", &MultiFileGenerator{}},
		{"hybrid", &HybridGenerator{}},
	}

	def := testDefinition()

	for _, s := range strategies {
		t.Run(s.name, func(t *testing.T) {
			var outputs [3]*BuildOutput
			for i := 0; i < 3; i++ {
				out, err := s.gen.Generate(def, 12345)
				if err != nil {
					t.Fatalf("run %d: unexpected error: %v", i, err)
				}
				outputs[i] = out
			}

			for i := 1; i < 3; i++ {
				if len(outputs[i].Files) != len(outputs[0].Files) {
					t.Errorf("run %d: file count mismatch: %d vs %d", i, len(outputs[i].Files), len(outputs[0].Files))
				}
				for path, content := range outputs[0].Files {
					if string(outputs[i].Files[path]) != string(content) {
						t.Errorf("run %d: file %q content mismatch", i, path)
					}
				}
			}
		})
	}
}

// TestValidGoServer verifies each main.go contains required Go constructs.
func TestValidGoServer(t *testing.T) {
	strategies := []struct {
		name string
		gen  CodeGenerator
	}{
		{"spa", &SPAGenerator{}},
		{"multifile", &MultiFileGenerator{}},
		{"hybrid", &HybridGenerator{}},
	}

	def := testDefinition()

	for _, s := range strategies {
		t.Run(s.name, func(t *testing.T) {
			output, err := s.gen.Generate(def, 42)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			mainGo := string(output.Files["main.go"])

			required := []string{"package main", "func main()", "http.ListenAndServe"}
			for _, r := range required {
				if !strings.Contains(mainGo, r) {
					t.Errorf("%s main.go missing %q", s.name, r)
				}
			}
		})
	}
}

// TestFormHandlingPresent verifies each strategy handles POST for form actions.
func TestFormHandlingPresent(t *testing.T) {
	strategies := []struct {
		name string
		gen  CodeGenerator
	}{
		{"spa", &SPAGenerator{}},
		{"multifile", &MultiFileGenerator{}},
		{"hybrid", &HybridGenerator{}},
	}

	def := testDefinition()

	for _, s := range strategies {
		t.Run(s.name, func(t *testing.T) {
			output, err := s.gen.Generate(def, 42)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			mainGo := string(output.Files["main.go"])

			if !strings.Contains(mainGo, "formHandler") {
				t.Errorf("%s main.go missing formHandler", s.name)
			}
			if !strings.Contains(mainGo, "/submit-login") {
				t.Errorf("%s main.go missing /submit-login route", s.name)
			}
			if !strings.Contains(mainGo, "r.ParseForm()") {
				t.Errorf("%s main.go missing form parsing", s.name)
			}
		})
	}
}

// TestHealthCheckPresent verifies each main.go includes /health endpoint.
func TestHealthCheckPresent(t *testing.T) {
	strategies := []struct {
		name string
		gen  CodeGenerator
	}{
		{"spa", &SPAGenerator{}},
		{"multifile", &MultiFileGenerator{}},
		{"hybrid", &HybridGenerator{}},
	}

	def := testDefinition()

	for _, s := range strategies {
		t.Run(s.name, func(t *testing.T) {
			output, err := s.gen.Generate(def, 42)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			mainGo := string(output.Files["main.go"])

			if !strings.Contains(mainGo, `"/health"`) {
				t.Errorf("%s main.go missing /health route", s.name)
			}
			if !strings.Contains(mainGo, "healthHandler") {
				t.Errorf("%s main.go missing healthHandler", s.name)
			}
		})
	}
}

// TestSelectStrategyRandom verifies empty name selects all 3 strategies across seeds.
func TestSelectStrategyRandom(t *testing.T) {
	seen := make(map[string]bool)

	for seed := int64(0); seed < 20; seed++ {
		gen, err := SelectStrategy("", seed)
		if err != nil {
			t.Fatalf("seed %d: unexpected error: %v", seed, err)
		}

		output, err := gen.Generate(testDefinition(), seed)
		if err != nil {
			t.Fatalf("seed %d: generate error: %v", seed, err)
		}
		seen[output.Strategy] = true
	}

	if len(seen) < 3 {
		t.Errorf("expected all 3 strategies seen across 20 seeds, got %d: %v", len(seen), seen)
	}
}

// TestSelectStrategyNamed verifies named strategy selection.
func TestSelectStrategyNamed(t *testing.T) {
	tests := []struct {
		name     string
		expected string
	}{
		{StrategySPA, StrategySPA},
		{StrategyMultiFile, StrategyMultiFile},
		{StrategyHybrid, StrategyHybrid},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			gen, err := SelectStrategy(tc.name, 0)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			output, err := gen.Generate(testDefinition(), 42)
			if err != nil {
				t.Fatalf("generate error: %v", err)
			}

			if output.Strategy != tc.expected {
				t.Errorf("expected strategy %q, got %q", tc.expected, output.Strategy)
			}
		})
	}
}

// TestSelectStrategyInvalid verifies invalid name returns error.
func TestSelectStrategyInvalid(t *testing.T) {
	_, err := SelectStrategy("invalid", 0)
	if err == nil {
		t.Error("expected error for invalid strategy name")
	}
}

// TestManifestCompleteness verifies each strategy's manifest includes required fields.
func TestStrategyManifestCompleteness(t *testing.T) {
	strategies := []struct {
		name     string
		gen      CodeGenerator
		required []string
	}{
		{"spa", &SPAGenerator{}, []string{"strategy", "pages_generated", "routing", "css_mode", "js_mode"}},
		{"multifile", &MultiFileGenerator{}, []string{"strategy", "pages_generated", "routing", "css_mode", "js_mode", "html_files"}},
		{"hybrid", &HybridGenerator{}, []string{"strategy", "pages_generated", "routing", "css_mode", "js_mode"}},
	}

	def := testDefinition()

	for _, s := range strategies {
		t.Run(s.name, func(t *testing.T) {
			output, err := s.gen.Generate(def, 42)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			for _, field := range s.required {
				if _, ok := output.Manifest[field]; !ok {
					t.Errorf("%s manifest missing field %q", s.name, field)
				}
			}
		})
	}
}

// TestMultiPageSupport verifies 3-page definition produces correct output.
func TestMultiPageSupport(t *testing.T) {
	strategies := []struct {
		name string
		gen  CodeGenerator
	}{
		{"spa", &SPAGenerator{}},
		{"multifile", &MultiFileGenerator{}},
		{"hybrid", &HybridGenerator{}},
	}

	def := testDefinition()

	for _, s := range strategies {
		t.Run(s.name, func(t *testing.T) {
			output, err := s.gen.Generate(def, 42)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			pagesGen := output.Manifest["pages_generated"]
			if pagesGen != 3 {
				t.Errorf("%s: expected pages_generated=3, got %v", s.name, pagesGen)
			}
		})
	}
}

// TestBuildTokenNotInHTML verifies build token appears in Go code but not in HTML/JS/CSS.
func TestBuildTokenNotInHTML(t *testing.T) {
	strategies := []struct {
		name string
		gen  CodeGenerator
	}{
		{"spa", &SPAGenerator{}},
		{"multifile", &MultiFileGenerator{}},
		{"hybrid", &HybridGenerator{}},
	}

	def := testDefinition()
	token := def.BuildToken

	for _, s := range strategies {
		t.Run(s.name, func(t *testing.T) {
			output, err := s.gen.Generate(def, 42)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			// Build token MUST be in main.go
			mainGo := string(output.Files["main.go"])
			if !strings.Contains(mainGo, token) {
				t.Errorf("%s: build token missing from main.go", s.name)
			}

			// Build token must NOT be in any HTML, JS, or CSS files
			for path, content := range output.Files {
				if path == "main.go" {
					continue
				}
				if strings.Contains(string(content), token) {
					t.Errorf("%s: build token found in %q — must not be exposed to frontend", s.name, path)
				}
			}
		})
	}
}

// TestAllStrategiesFunc verifies AllStrategies returns 3 strategy names.
func TestAllStrategiesFunc(t *testing.T) {
	strategies := AllStrategies()
	if len(strategies) != 3 {
		t.Errorf("expected 3 strategies, got %d", len(strategies))
	}

	expected := map[string]bool{StrategySPA: true, StrategyMultiFile: true, StrategyHybrid: true}
	for _, s := range strategies {
		if !expected[s] {
			t.Errorf("unexpected strategy name %q", s)
		}
	}
}

// TestEntryPointSet verifies EntryPoint is set for all strategies.
func TestEntryPointSet(t *testing.T) {
	strategies := []struct {
		name string
		gen  CodeGenerator
	}{
		{"spa", &SPAGenerator{}},
		{"multifile", &MultiFileGenerator{}},
		{"hybrid", &HybridGenerator{}},
	}

	for _, s := range strategies {
		t.Run(s.name, func(t *testing.T) {
			output, err := s.gen.Generate(testDefinition(), 42)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if output.EntryPoint != "main.go" {
				t.Errorf("expected EntryPoint 'main.go', got %q", output.EntryPoint)
			}
		})
	}
}

// TestMultipleSeeds runs all strategies across multiple seeds to verify no panics.
func TestMultipleSeeds(t *testing.T) {
	strategies := []CodeGenerator{&SPAGenerator{}, &MultiFileGenerator{}, &HybridGenerator{}}
	def := testDefinition()

	for _, gen := range strategies {
		for seed := int64(0); seed < 10; seed++ {
			output, err := gen.Generate(def, seed)
			if err != nil {
				t.Fatalf("seed %d: unexpected error: %v", seed, err)
			}
			if len(output.Files) == 0 {
				t.Errorf("seed %d: no files generated", seed)
			}
		}
	}
}
