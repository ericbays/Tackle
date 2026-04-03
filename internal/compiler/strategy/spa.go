package strategy

import (
	"fmt"
	"math/rand"
	"strings"
)

// SPAGenerator generates a Single Page Application with all pages in one HTML file.
type SPAGenerator struct{}

// Generate produces all files for a landing page build using SPA strategy.
func (g *SPAGenerator) Generate(definition PageDefinition, seed int64) (*BuildOutput, error) {
	r := rand.New(rand.NewSource(seed))

	// Generate the single HTML file with all pages
	htmlContent := g.generateSPAHTML(definition, r)

	// Generate the Go server
	goContent := g.generateSPAGo(definition, r)

	output := &BuildOutput{
		Files: map[string][]byte{
			"index.html": []byte(htmlContent),
			"main.go":    []byte(goContent),
		},
		EntryPoint: "main.go",
		Strategy:   StrategySPA,
		Manifest: map[string]any{
			"strategy":          StrategySPA,
			"pages_generated":   len(definition.Pages),
			"routing":           "hash",
			"css_mode":          "inline",
			"js_mode":           "inline",
		},
	}

	return output, nil
}

// generateSPAHTML creates a single HTML file with all pages as hidden sections.
func (g *SPAGenerator) generateSPAHTML(definition PageDefinition, r *rand.Rand) string {
	var sb strings.Builder

	// Start HTML document
	sb.WriteString("<!DOCTYPE html>\n<html lang=\"en\">\n<head>\n")
	sb.WriteString(g.generateHead(definition, r))
	sb.WriteString("</head>\n<body>\n")

	// Write all pages
	for _, page := range definition.Pages {
		sb.WriteString(fmt.Sprintf("<div id=\"%s\" class=\"page\" data-path=\"%s\">\n", page.ID, page.Path))
		sb.WriteString(page.HTML)
		sb.WriteString(fmt.Sprintf("</div>\n<!-- /%s -->\n", page.ID))
	}

	// Write inline CSS
	sb.WriteString("<style>\n")
	sb.WriteString(definition.GlobalCSS)
	for _, page := range definition.Pages {
		if page.CSS != "" {
			sb.WriteString(fmt.Sprintf("/* Page: %s */\n%s\n", page.ID, page.CSS))
		}
	}
	sb.WriteString(g.generateSPAStyles(r))
	sb.WriteString("</style>\n")

	// Write inline JS
	sb.WriteString("<script>\n")
	sb.WriteString(definition.GlobalJS)
	for _, page := range definition.Pages {
		if page.JS != "" {
			sb.WriteString(fmt.Sprintf("// Page: %s\n%s\n", page.ID, page.JS))
		}
	}
	sb.WriteString(g.generateSPAJS(definition, r))
	sb.WriteString("</script>\n")

	sb.WriteString("</body>\n</html>\n")

	return sb.String()
}

// generateHead creates the <head> section with title and meta tags.
func (g *SPAGenerator) generateHead(definition PageDefinition, r *rand.Rand) string {
	var sb strings.Builder

	// Randomized meta tags
	sb.WriteString(fmt.Sprintf("<meta charset=\"UTF-8\">\n"))
	sb.WriteString(fmt.Sprintf("<meta name=\"viewport\" content=\"width=device-width, initial-scale=1.0\">\n"))
	sb.WriteString(fmt.Sprintf("<title>%s</title>\n", definition.Pages[0].Title))

	return sb.String()
}

// generateSPAStyles generates additional SPA-specific styles.
func (g *SPAGenerator) generateSPAStyles(r *rand.Rand) string {
	return `
/* SPA Styles */
.page {
    display: none;
}
.page.active {
    display: block;
}
`
}

// generateSPAJS generates the hash routing JavaScript.
func (g *SPAGenerator) generateSPAJS(definition PageDefinition, r *rand.Rand) string {
	var sb strings.Builder

	sb.WriteString("\n// Hash-based routing for SPA\n")
	sb.WriteString("var routes = {};\n")
	for _, page := range definition.Pages {
		sb.WriteString(fmt.Sprintf("routes[%q] = %q;\n", page.Path, page.ID))
	}
	sb.WriteString(`
function showPage(path) {
    var pageId = routes[path];
    if (!pageId) return;
    var pages = document.querySelectorAll('.page');
    for (var i = 0; i < pages.length; i++) {
        pages[i].classList.remove('active');
    }
    var page = document.getElementById(pageId);
    if (page) {
        page.classList.add('active');
    }
}

function navigate() {
    var hash = window.location.hash.slice(1) || '/';
    showPage(hash);
}

window.addEventListener('hashchange', navigate);
navigate();
`)

	return sb.String()
}

// buildRouteMap creates a map of page paths to IDs for the routing JS.
func (g *SPAGenerator) buildRouteMap(definition PageDefinition) map[string]string {
	routeMap := make(map[string]string)
	for _, page := range definition.Pages {
		routeMap[page.Path] = page.ID
	}
	return routeMap
}

// generateSPAGo generates the Go server for SPA.
func (g *SPAGenerator) generateSPAGo(definition PageDefinition, r *rand.Rand) string {
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

	// Write build token as a Go constant (not exposed to frontend)
	sb.WriteString(fmt.Sprintf("const buildToken = \"%s\"\n\n", definition.BuildToken))

	// Routes for each page
	sb.WriteString(`var pages = []struct {
	Name  string
	Path  string
}{
`)

	for _, page := range definition.Pages {
		sb.WriteString(fmt.Sprintf("\t{Name: %q, Path: %q},\n", page.Title, page.Path))
	}
	sb.WriteString("}\n\n")

	// Health check handler
	sb.WriteString(`// healthHandler returns a health check response.
func healthHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	fmt.Fprintf(w, ` + "`" + `{"status":"ok","build_token":"%s"}` + "`" + `, buildToken)
}
`)

	// Main page handler with hash routing
	sb.WriteString(`// pageHandler serves the SPA at any route.
func pageHandler(w http.ResponseWriter, r *http.Request) {
	// Set content type based on path
	contentType := "text/html; charset=utf-8"
	w.Header().Set("Content-Type", contentType)
	w.WriteHeader(http.StatusOK)
	
	// Route based on path (without hash)
	path := strings.TrimPrefix(r.URL.Path, "/")
	if path == "" {
		path = "/"
	}
	
	fmt.Fprint(w, pages[0].Name)
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
	
	// SPA routes (all paths serve the same HTML)
	mux.HandleFunc("/", pageHandler)
	mux.HandleFunc("/login", pageHandler)
	mux.HandleFunc("/success", pageHandler)
	
	// Form submission endpoint
	mux.HandleFunc("/submit-login", formHandler)

	log.Printf("Starting SPA server on port %s", port)
	log.Printf("Build token: %s", buildToken)
	if err := http.ListenAndServe(":" + port, mux); err != nil {
		log.Fatalf("server failed: %v", err)
	}
}
`)

	return sb.String()
}