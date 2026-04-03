package randomizer

import (
	"testing"
)

func TestNoopDOMRandomizer(t *testing.T) {
	r := &NoopDOMRandomizer{}
	input := "<div>test</div>"
	out, manifest, err := r.RandomizeDOM(input, 42)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out != input {
		t.Errorf("expected passthrough, got %q", out)
	}
	if manifest["strategy"] != "noop" {
		t.Error("expected noop strategy in manifest")
	}
}

func TestNoopCSSRandomizer(t *testing.T) {
	r := &NoopCSSRandomizer{}
	html := "<div class=\"test\">x</div>"
	css := ".test { color: red; }"
	outHTML, outCSS, manifest, err := r.RandomizeCSS(html, css, 42)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if outHTML != html || outCSS != css {
		t.Error("expected passthrough")
	}
	if manifest["strategy"] != "noop" {
		t.Error("expected noop strategy in manifest")
	}
}

func TestNoopAssetRandomizer(t *testing.T) {
	r := &NoopAssetRandomizer{}
	files := map[string][]byte{"img.png": {0x89}}
	outFiles, outHTML, outCSS, manifest, err := r.RandomizeAssets(files, "<img>", "body{}", 42)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(outFiles) != 1 || outHTML != "<img>" || outCSS != "body{}" {
		t.Error("expected passthrough")
	}
	if manifest["strategy"] != "noop" {
		t.Error("expected noop strategy in manifest")
	}
}

func TestNoopDecoyInjector(t *testing.T) {
	r := &NoopDecoyInjector{}
	outHTML, outCSS, outJS, manifest, err := r.InjectDecoys("<div>", ".x{}", "var a;", 42)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if outHTML != "<div>" || outCSS != ".x{}" || outJS != "var a;" {
		t.Error("expected passthrough")
	}
	if manifest["strategy"] != "noop" {
		t.Error("expected noop strategy in manifest")
	}
}

func TestNoopHeaderRandomizer(t *testing.T) {
	r := &NoopHeaderRandomizer{}
	headers, middleware, manifest, err := r.GenerateHeaderProfile(42)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(headers) != 0 {
		t.Error("expected empty headers")
	}
	if middleware != "" {
		t.Error("expected empty middleware")
	}
	if manifest["strategy"] != "noop" {
		t.Error("expected noop strategy in manifest")
	}
}
