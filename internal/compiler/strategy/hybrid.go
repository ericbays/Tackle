package strategy

import (
	"fmt"
	"math/rand"
	"strings"
)

// HybridGenerator generates a hybrid SPA with JS-driven page transitions.
type HybridGenerator struct{}

// Generate produces all files for a landing page build using Hybrid strategy.
func (g *HybridGenerator) Generate(definition PageDefinition, seed int64) (*BuildOutput, error) {
	r := rand.New(rand.NewSource(seed))

	output := &BuildOutput{
		Files:      make(map[string][]byte),
		EntryPoint: "main.go",
		Strategy:   StrategyHybrid,
		Manifest: map[string]any{
			"strategy":        StrategyHybrid,
			"pages_generated": len(definition.Pages),
			"routing":         "pushstate",
			"css_mode":        "mixed",
			"js_mode":         "external_bundle",
		},
	}

	// Generate main.go
	output.Files["main.go"] = []byte(g.generateHybridGo(definition, r))

	// Generate index.html (entry point)
	output.Files["index.html"] = []byte(g.generateIndexHTML(definition, r))

	// Generate JS bundle
	output.Files["static/app.js"] = []byte(g.generateJSBundle(definition, r))

	// Generate extra CSS (non-critical/page-specific)
	output.Files["static/extra.css"] = []byte(g.generateExtraCSS(definition, r))

	return output, nil
}

// generateHybridGo generates the Go server for Hybrid strategy.
func (g *HybridGenerator) generateHybridGo(definition PageDefinition, r *rand.Rand) string {
	var sb strings.Builder

	sb.WriteString("package main\n\n")
	sb.WriteString(`import (
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"
)
`)

	// Write build token as a Go constant
	sb.WriteString(fmt.Sprintf("const buildToken = \"%s\"\n\n", definition.BuildToken))

	// Generate content for embedding
	indexHTML := g.generateIndexHTML(definition, r)
	indexCSS := g.generateIndexCSS(definition, r)
	jsBundle := g.generateJSBundle(definition, r)
	extraCSS := g.generateExtraCSS(definition, r)

	// Health check handler
	healthHandler := `// healthHandler returns a health check response.
func healthHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	fmt.Fprintf(w, ` + "`" + `{"status":"ok","build_token":"%s"}` + "`" + `, buildToken)
}
`

	// HTML content (embedded)
	htmlContentDecl := fmt.Sprintf("var htmlContent = `%s`", indexHTML)

	// CSS content (embedded)
	cssContentDecl := fmt.Sprintf("var cssContent = `%s`", indexCSS)

	// JS bundle content (embedded)
	jsBundleDecl := fmt.Sprintf("var jsBundle = `%s`", jsBundle)

	// Extra CSS content (embedded)
	extraCSSDecl := fmt.Sprintf("var extraCSS = `%s`", extraCSS)

	sb.WriteString(htmlContentDecl + "\n\n")
	sb.WriteString(cssContentDecl + "\n\n")
	sb.WriteString(jsBundleDecl + "\n\n")
	sb.WriteString(extraCSSDecl + "\n\n")
	sb.WriteString(healthHandler)

	// Main page handler with pushState routing
	sb.WriteString(`// pageHandler serves the hybrid SPA.
func pageHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	fmt.Fprint(w, htmlContent)
}
`)

	// CSS handler
	sb.WriteString(`// cssHandler serves the CSS file.
func cssHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/css")
	w.WriteHeader(http.StatusOK)
	fmt.Fprint(w, cssContent)
}
`)

	// JS handler
	sb.WriteString(`// jsHandler serves the JavaScript bundle.
func jsHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/javascript")
	w.WriteHeader(http.StatusOK)
	fmt.Fprint(w, jsBundle)
}
`)

	// Extra CSS handler
	sb.WriteString(`// extraCSSHandler serves the extra CSS file.
func extraCSSHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/css")
	w.WriteHeader(http.StatusOK)
	fmt.Fprint(w, extraCSS)
}
`)

	// Form submission handler
	sb.WriteString(`// formHandler handles form POST submissions.
func formHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	
	// Parse form data
	if err := r.ParseForm(); err != nil {
		log.Printf("error parsing form: %v", err)
		http.Error(w, "Bad Request", http.StatusBadRequest)
		return
	}
	
	// Log submission (placeholder - real forwarding done elsewhere)
	log.Printf("Form submission received from %s", r.RemoteAddr)
	
	// Redirect to configurable URL (or same page)
	redirectURL := r.FormValue("redirect")
	if redirectURL == "" {
		redirectURL = "/"
	}
	http.Redirect(w, r, redirectURL, http.StatusFound)
}
`)

	// Main function
	sb.WriteString(`// main sets up HTTP routes and starts the server.
func main() {
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	mux := http.NewServeMux()
	
	// Health check endpoint
	mux.HandleFunc("/health", healthHandler)
	
	// Static file handlers
	mux.HandleFunc("/static/app.js", jsHandler)
	mux.HandleFunc("/static/app.css", cssHandler)
	mux.HandleFunc("/static/extra.css", extraCSSHandler)
	
	// SPA routes (all paths serve the same HTML for pushState)
	mux.HandleFunc("/", pageHandler)
	mux.HandleFunc("/login", pageHandler)
	mux.HandleFunc("/success", pageHandler)
	
	// Form submission endpoint
	mux.HandleFunc("/submit-login", formHandler)

	log.Printf("Starting Hybrid server on port %s", port)
	log.Printf("Build token: %s", buildToken)
	if err := http.ListenAndServe(":" + port, mux); err != nil {
		log.Fatalf("server failed: %v", err)
	}
}
`)

	return sb.String()
}

// generateIndexCSS generates the critical CSS to be inlined.
func (g *HybridGenerator) generateIndexCSS(definition PageDefinition, r *rand.Rand) string {
	var sb strings.Builder

	// Critical/base styles (inline in HTML)
	sb.WriteString(`
/* Critical CSS */
* {
    margin: 0;
    padding: 0;
    box-sizing: border-box;
}
body {
    font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, sans-serif;
    line-height: 1.6;
    color: #333;
}
`)

	// Global CSS
	sb.WriteString(definition.GlobalCSS)

	// Page-specific critical styles
	for _, page := range definition.Pages {
		if page.CSS != "" {
			sb.WriteString(fmt.Sprintf("\n/* Page: %s */\n%s\n", page.ID, page.CSS))
		}
	}

	return sb.String()
}

// generateIndexHTML generates the entry point HTML file.
func (g *HybridGenerator) generateIndexHTML(definition PageDefinition, r *rand.Rand) string {
	var sb strings.Builder

	sb.WriteString("<!DOCTYPE html>\n<html lang=\"en\">\n<head>\n")
	sb.WriteString(g.generateHead(definition, r))
	sb.WriteString("</head>\n<body>\n")

	// Write all pages as templates (hidden)
	sb.WriteString("<!-- Page Templates -->\n")
	for _, page := range definition.Pages {
		sb.WriteString(fmt.Sprintf("<template id=\"%s\">\n", page.ID))
		sb.WriteString(page.HTML)
		sb.WriteString(fmt.Sprintf("</template>\n<!-- /%s -->\n", page.ID))
	}

	// Write inline critical CSS
	sb.WriteString("<style>\n")
	sb.WriteString(g.generateIndexCSS(definition, r))
	sb.WriteString("</style>\n")

	// Write inline JS (small snippet to load external assets)
	sb.WriteString("<script>\n")
	sb.WriteString("// Hybrid SPA routing with pushState\n")
	sb.WriteString("var routes = {};\n")
	for _, page := range definition.Pages {
		sb.WriteString(fmt.Sprintf("routes[%q] = %q;\n", page.Path, page.ID))
	}
	sb.WriteString(`var pageTemplates = new Map();

function loadTemplates() {
    document.querySelectorAll('template').forEach(t => {
        pageTemplates.set(t.id, t.innerHTML);
    });
}

function navigate(path) {
    path = path || window.location.pathname;
    const pageId = routes[path] || 'home';
    const template = pageTemplates.get(pageId);
    
    if (template) {
        document.body.innerHTML = template;
        window.history.pushState({path}, document.title, path);
    }
}

function handlePopState(event) {
    navigate(event.state?.path || window.location.pathname);
}

window.addEventListener('popstate', handlePopState);
loadTemplates();
navigate();
`)

	sb.WriteString("</script>\n")

	// Link to external JS bundle
	sb.WriteString("<script src=\"static/app.js\"></script>\n")

	// Link to extra CSS
	sb.WriteString("<link rel=\"stylesheet\" href=\"static/extra.css\">\n")

	sb.WriteString("</body>\n</html>\n")

	return sb.String()
}

// generateHead creates the <head> section.
func (g *HybridGenerator) generateHead(definition PageDefinition, r *rand.Rand) string {
	var sb strings.Builder

	sb.WriteString(fmt.Sprintf("<meta charset=\"UTF-8\">\n"))
	sb.WriteString(fmt.Sprintf("<meta name=\"viewport\" content=\"width=device-width, initial-scale=1.0\">\n"))
	sb.WriteString(fmt.Sprintf("<title>%s</title>\n", definition.Pages[0].Title))

	return sb.String()
}

// generateJSBundle generates the combined JavaScript bundle.
func (g *HybridGenerator) generateJSBundle(definition PageDefinition, r *rand.Rand) string {
	var sb strings.Builder

	// Global JS
	sb.WriteString(definition.GlobalJS)

	// Per-page scripts
	for _, page := range definition.Pages {
		if page.JS != "" {
			sb.WriteString(fmt.Sprintf("\n// Page: %s\n%s\n", page.ID, page.JS))
		}
	}

	return sb.String()
}

// generateExtraCSS generates non-critical/page-specific CSS.
func (g *HybridGenerator) generateExtraCSS(definition PageDefinition, r *rand.Rand) string {
	var sb strings.Builder

	// Page-specific styles that don't need to be critical
	for _, page := range definition.Pages {
		sb.WriteString(fmt.Sprintf("/* Styles for %s page */\n", page.ID))
		if page.CSS != "" {
			sb.WriteString(page.CSS)
		}
		sb.WriteString("\n")
	}

	return sb.String()
}

// buildRouteMap creates a map of page paths to IDs for routing.
func (g *HybridGenerator) buildRouteMap(definition PageDefinition) map[string]string {
	routeMap := make(map[string]string)
	for _, page := range definition.Pages {
		routeMap[page.Path] = page.ID
	}
	return routeMap
}
