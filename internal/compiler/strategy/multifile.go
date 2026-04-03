package strategy

import (
	"fmt"
	"math/rand"
	"path/filepath"
	"strings"
)

// MultiFileGenerator generates separate HTML files for each page with external CSS/JS.
type MultiFileGenerator struct{}

// Generate produces all files for a landing page build using Multi-File strategy.
func (g *MultiFileGenerator) Generate(definition PageDefinition, seed int64) (*BuildOutput, error) {
	r := rand.New(rand.NewSource(seed))

	output := &BuildOutput{
		Files:      make(map[string][]byte),
		EntryPoint: "main.go",
		Strategy:   StrategyMultiFile,
		Manifest: map[string]any{
			"strategy":        StrategyMultiFile,
			"pages_generated": len(definition.Pages),
			"routing":         "server",
			"css_mode":        "external",
			"js_mode":         "external",
			"html_files":      make([]string, 0, len(definition.Pages)),
		},
	}

	// Generate main.go
	output.Files["main.go"] = []byte(g.generateMultiFileGo(definition, r))

	// Generate per-page HTML files
	for _, page := range definition.Pages {
		htmlPath := filepath.Join("pages", page.ID+".html")
		output.Files[htmlPath] = []byte(g.generatePageHTML(page, definition, r))
		output.Manifest["html_files"] = append(output.Manifest["html_files"].([]string), htmlPath)
	}

	// Generate combined CSS
	cssContent := g.generateCombinedCSS(definition, r)
	output.Files["static/styles.css"] = []byte(cssContent)

	// Generate combined JS
	jsContent := g.generateCombinedJS(definition, r)
	output.Files["static/app.js"] = []byte(jsContent)

	return output, nil
}

// generateMultiFileGo generates the Go server for Multi-File strategy.
func (g *MultiFileGenerator) generateMultiFileGo(definition PageDefinition, r *rand.Rand) string {
	var sb strings.Builder

	sb.WriteString("package main\n\n")
	sb.WriteString(`import (
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
)
`)

	// Write build token as a Go constant
	sb.WriteString(fmt.Sprintf("const buildToken = \"%s\"\n\n", definition.BuildToken))

	// Combined JS for static files
	combinedJS := g.generateCombinedJS(definition, r)
	combinedCSS := g.generateCombinedCSS(definition, r)

	// Static files map (CSS and JS)
	sb.WriteString(fmt.Sprintf(`var staticFiles = map[string]string{
	"styles.css": %q,
	"app.js": %q,
}`, combinedCSS, combinedJS))

	// HTML template (embedded)
	sb.WriteString(`var pageTemplates = map[string]string{
`)

	for _, page := range definition.Pages {
		escapedHTML := fmt.Sprintf("%q", page.HTML)
		sb.WriteString(fmt.Sprintf("\t%q: %s,\n", page.Path, escapedHTML))
	}
	sb.WriteString("}\n\n")

	// Health check handler
	sb.WriteString(`// healthHandler returns a health check response.
func healthHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	fmt.Fprintf(w, ` + "`" + `{"status":"ok","build_token":"%s"}` + "`" + `, buildToken)
}
`)

	// Main page handler with server-side routing
	sb.WriteString(`// pageHandler serves individual HTML pages based on path.
func pageHandler(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/")
	if path == "" {
		path = "/"
	}

	// Find matching template or use default
	templateContent, found := pageTemplates[path]
	if !found {
		// Try to find by matching path prefix
		for p, t := range pageTemplates {
			if strings.HasPrefix(path, p) {
				templateContent = t
				found = true
				break
			}
		}
	}
	
	if !found && pageTemplates["/"] != "" {
		templateContent = pageTemplates["/"]
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	fmt.Fprint(w, templateContent)
}
`)

	// Static file handler for CSS and JS
	sb.WriteString(`// staticHandler serves static files (CSS, JS).
func staticHandler(w http.ResponseWriter, r *http.Request) {
	relPath := strings.TrimPrefix(r.URL.Path, "/static/")
	
	// Determine content type based on file extension
	switch filepath.Ext(relPath) {
	case ".css":
		w.Header().Set("Content-Type", "text/css")
	case ".js":
		w.Header().Set("Content-Type", "application/javascript")
	}
	
	if content, ok := staticFiles[relPath]; ok {
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, content)
	} else {
		http.Error(w, "Not Found", http.StatusNotFound)
	}
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
	
	// Static file handler
	mux.Handle("/static/", http.StripPrefix("/static/", http.HandlerFunc(staticHandler)))
	
	// Page routes
	mux.HandleFunc("/", pageHandler)
	mux.HandleFunc("/login", pageHandler)
	mux.HandleFunc("/success", pageHandler)
	
	// Form submission endpoint
	mux.HandleFunc("/submit-login", formHandler)

	log.Printf("Starting Multi-File server on port %s", port)
	log.Printf("Build token: %s", buildToken)
	if err := http.ListenAndServe(":" + port, mux); err != nil {
		log.Fatalf("server failed: %v", err)
	}
}
`)

	return sb.String()
}

// generatePageHTML generates HTML for a single page.
func (g *MultiFileGenerator) generatePageHTML(page PageDef, definition PageDefinition, r *rand.Rand) string {
	var sb strings.Builder

	sb.WriteString("<!DOCTYPE html>\n<html lang=\"en\">\n<head>\n")
	sb.WriteString(g.generateHead(page.Title, definition, r))
	sb.WriteString("</head>\n<body>\n")
	sb.WriteString(page.HTML)

	// Link to external CSS
	sb.WriteString("<link rel=\"stylesheet\" href=\"static/styles.css\">\n")

	// Link to external JS
	sb.WriteString("<script src=\"static/app.js\"></script>\n")

	sb.WriteString("</body>\n</html>\n")

	return sb.String()
}

// generateHead creates the <head> section.
func (g *MultiFileGenerator) generateHead(title string, definition PageDefinition, r *rand.Rand) string {
	var sb strings.Builder

	sb.WriteString(fmt.Sprintf("<meta charset=\"UTF-8\">\n"))
	sb.WriteString(fmt.Sprintf("<meta name=\"viewport\" content=\"width=device-width, initial-scale=1.0\">\n"))
	sb.WriteString(fmt.Sprintf("<title>%s</title>\n", title))

	return sb.String()
}

// generateCombinedCSS generates combined CSS for all pages.
func (g *MultiFileGenerator) generateCombinedCSS(definition PageDefinition, r *rand.Rand) string {
	var sb strings.Builder

	// Global styles
	sb.WriteString(definition.GlobalCSS)

	// Per-page styles
	for _, page := range definition.Pages {
		if page.CSS != "" {
			sb.WriteString(fmt.Sprintf("\n/* Page: %s */\n%s\n", page.ID, page.CSS))
		}
	}

	return sb.String()
}

// generateCombinedJS generates combined JavaScript for all pages.
func (g *MultiFileGenerator) generateCombinedJS(definition PageDefinition, r *rand.Rand) string {
	var sb strings.Builder

	sb.WriteString(definition.GlobalJS)

	// Per-page scripts
	for _, page := range definition.Pages {
		if page.JS != "" {
			sb.WriteString(fmt.Sprintf("\n// Page: %s\n%s\n", page.ID, page.JS))
		}
	}

	return sb.String()
}
