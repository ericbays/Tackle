package gogen

import (
	"strings"
	"testing"

	"tackle/internal/compiler/htmlgen"
)

func TestGenerateGoSource_Basic(t *testing.T) {
	config := GoSourceConfig{
		ModuleName:       "testapp",
		BuildToken:       "test-token-123",
		CampaignID:       "campaign-456",
		FrameworkBaseURL: "http://127.0.0.1:8080",
		Pages: []htmlgen.PageOutput{
			{Route: "/", Filename: "index.html"},
			{Route: "/login", Filename: "login.html"},
		},
	}

	src, err := GenerateGoSource(config)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(src.Files) != 3 {
		t.Fatalf("expected 3 files (go.mod, main.go, handlers.go), got %d", len(src.Files))
	}

	// Check go.mod.
	goMod := src.Files["go.mod"]
	if !strings.Contains(goMod, "module testapp") {
		t.Error("go.mod should contain module name")
	}
	if !strings.Contains(goMod, "go 1.22") {
		t.Error("go.mod should specify go 1.22")
	}

	// Check main.go.
	mainGo := src.Files["main.go"]
	if !strings.Contains(mainGo, "package main") {
		t.Error("main.go should have package main")
	}
	if !strings.Contains(mainGo, `buildToken   = "test-token-123"`) {
		t.Error("main.go should contain build token constant")
	}
	if !strings.Contains(mainGo, `campaignID   = "campaign-456"`) {
		t.Error("main.go should contain campaign ID constant")
	}
	if !strings.Contains(mainGo, `frameworkURL = "http://127.0.0.1:8080"`) {
		t.Error("main.go should contain framework URL constant")
	}
	if !strings.Contains(mainGo, "//go:embed static/*") {
		t.Error("main.go should contain embed directive")
	}
	if !strings.Contains(mainGo, `"/capture"`) {
		t.Error("main.go should register capture route")
	}
	if !strings.Contains(mainGo, `"/track"`) {
		t.Error("main.go should register tracking route")
	}
	if !strings.Contains(mainGo, `"/health"`) {
		t.Error("main.go should register health route")
	}

	// Check page routes are registered.
	if !strings.Contains(mainGo, `"/"`) {
		t.Error("main.go should register root route")
	}
	if !strings.Contains(mainGo, `"/login"`) {
		t.Error("main.go should register login route")
	}

	// Check handlers.go.
	handlersGo := src.Files["handlers.go"]
	if !strings.Contains(handlersGo, "func serveStaticPage") {
		t.Error("handlers.go should contain serveStaticPage")
	}
	if !strings.Contains(handlersGo, "func handleCapture") {
		t.Error("handlers.go should contain handleCapture")
	}
	if !strings.Contains(handlersGo, "func handleTracking") {
		t.Error("handlers.go should contain handleTracking")
	}
	if !strings.Contains(handlersGo, "func handleTelemetry") {
		t.Error("handlers.go should contain handleTelemetry")
	}
	if !strings.Contains(handlersGo, "func handleHealth") {
		t.Error("handlers.go should contain handleHealth")
	}
	if !strings.Contains(handlersGo, "X-Build-Token") {
		t.Error("handlers.go should include build token header in forwarding")
	}
	if !strings.Contains(handlersGo, "trackPixelGIF") {
		t.Error("handlers.go should contain tracking pixel GIF data")
	}
}

func TestGenerateGoSource_MissingBuildToken(t *testing.T) {
	config := GoSourceConfig{
		FrameworkBaseURL: "http://127.0.0.1:8080",
	}
	_, err := GenerateGoSource(config)
	if err == nil {
		t.Fatal("expected error for missing build token")
	}
	if !strings.Contains(err.Error(), "build token") {
		t.Errorf("expected build token error, got: %v", err)
	}
}

func TestGenerateGoSource_MissingFrameworkURL(t *testing.T) {
	config := GoSourceConfig{
		BuildToken: "test-token",
	}
	_, err := GenerateGoSource(config)
	if err == nil {
		t.Fatal("expected error for missing framework URL")
	}
	if !strings.Contains(err.Error(), "framework base URL") {
		t.Errorf("expected framework URL error, got: %v", err)
	}
}

func TestGenerateGoSource_HeaderMiddleware(t *testing.T) {
	customMiddleware := `func headerMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Server", "nginx/1.21")
		next.ServeHTTP(w, r)
	})
}`

	config := GoSourceConfig{
		BuildToken:          "test-token",
		FrameworkBaseURL:    "http://127.0.0.1:8080",
		HeaderMiddlewareSrc: customMiddleware,
		Pages:               []htmlgen.PageOutput{{Route: "/", Filename: "index.html"}},
	}

	src, err := GenerateGoSource(config)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	handlersGo := src.Files["handlers.go"]
	if !strings.Contains(handlersGo, "nginx/1.21") {
		t.Error("handlers.go should contain custom header middleware")
	}
}

func TestGenerateGoSource_DefaultHeaderMiddleware(t *testing.T) {
	config := GoSourceConfig{
		BuildToken:       "test-token",
		FrameworkBaseURL: "http://127.0.0.1:8080",
		Pages:            []htmlgen.PageOutput{{Route: "/", Filename: "index.html"}},
	}

	src, err := GenerateGoSource(config)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	handlersGo := src.Files["handlers.go"]
	if !strings.Contains(handlersGo, "func headerMiddleware") {
		t.Error("handlers.go should contain default header middleware")
	}
}

func TestGenerateGoSource_BuildTokenNotInHTML(t *testing.T) {
	config := GoSourceConfig{
		BuildToken:       "super-secret-token",
		FrameworkBaseURL: "http://127.0.0.1:8080",
		Pages:            []htmlgen.PageOutput{{Route: "/", Filename: "index.html", HTML: "<html><body>Hello</body></html>"}},
	}

	src, err := GenerateGoSource(config)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Build token should be in Go constants but NOT served to browser.
	mainGo := src.Files["main.go"]
	if !strings.Contains(mainGo, "super-secret-token") {
		t.Error("build token should be in main.go as constant")
	}

	// The HTML template itself should not contain the build token.
	handlersGo := src.Files["handlers.go"]
	// forwardJSON uses the constant, but it's not in any served HTML.
	if strings.Contains(handlersGo, "super-secret-token") {
		t.Error("build token should NOT be hardcoded in handlers (uses constant)")
	}
}
