package landingpage

import (
	"archive/zip"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"mime"
	"net/http"
	"net/url"
	"path"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/google/uuid"
)

const (
	maxZIPSize      = 50 * 1024 * 1024 // 50 MB
	maxZIPFileCount = 500
)

// ParseHTMLToDefinition parses HTML content into builder components (best-effort).
// Unmapped elements become raw_html blocks. Merges into the first page of the existing definition.
func ParseHTMLToDefinition(htmlContent string, existingDef map[string]any) (map[string]any, error) {
	if strings.TrimSpace(htmlContent) == "" {
		return nil, fmt.Errorf("empty HTML content")
	}

	components := parseHTMLElements(htmlContent)

	def := copyDefinition(existingDef)
	pages := ensurePages(def)
	if len(pages) == 0 {
		return nil, fmt.Errorf("no pages in definition")
	}

	page, ok := pages[0].(map[string]any)
	if !ok {
		return nil, fmt.Errorf("invalid page structure")
	}

	page["component_tree"] = components
	pages[0] = page
	def["pages"] = pages

	// Extract any <style> blocks into page_styles.
	if styles := extractStyles(htmlContent); styles != "" {
		page["page_styles"] = styles
	}

	// Extract any <script> blocks into page_js.
	if scripts := extractScripts(htmlContent); scripts != "" {
		page["page_js"] = scripts
	}

	// Extract title.
	if title := extractTitle(htmlContent); title != "" {
		page["title"] = title
	}

	return def, nil
}

// ImportRawHTML imports HTML as a single raw_html block.
func ImportRawHTML(htmlContent string, existingDef map[string]any) (map[string]any, error) {
	if strings.TrimSpace(htmlContent) == "" {
		return nil, fmt.Errorf("empty HTML content")
	}

	component := map[string]any{
		"component_id": uuid.New().String(),
		"type":         "raw_html",
		"properties": map[string]any{
			"content": htmlContent,
		},
		"children":       []any{},
		"event_bindings": []any{},
	}

	def := copyDefinition(existingDef)
	pages := ensurePages(def)
	if len(pages) == 0 {
		return nil, fmt.Errorf("no pages in definition")
	}

	page, ok := pages[0].(map[string]any)
	if !ok {
		return nil, fmt.Errorf("invalid page structure")
	}

	page["component_tree"] = []any{component}
	pages[0] = page
	def["pages"] = pages

	return def, nil
}

const (
	cloneAssetTimeout  = 10 * time.Second
	clonePageTimeout   = 15 * time.Second
	cloneMaxAssets     = 100
	cloneMaxAssetSize  = 10 * 1024 * 1024  // 10 MB per asset
	cloneMaxTotalSize  = 50 * 1024 * 1024  // 50 MB total
	cloneMaxImageSize  = 2 * 1024 * 1024   // 2 MB for base64 inlining
)

// ClonePageFromURL fetches a URL, downloads its HTML, localizes CSS/JS assets
// by inlining them, and converts images to data URIs where practical.
func ClonePageFromURL(sourceURL string, includeJS, stripTracking bool) (map[string]any, error) {
	if !strings.HasPrefix(sourceURL, "http://") && !strings.HasPrefix(sourceURL, "https://") {
		return nil, fmt.Errorf("invalid URL: must start with http:// or https://")
	}

	parsedURL, err := url.Parse(sourceURL)
	if err != nil {
		return nil, fmt.Errorf("invalid URL: %w", err)
	}

	// Fetch the HTML page.
	client := &http.Client{Timeout: clonePageTimeout}
	resp, err := client.Get(sourceURL)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch page: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("page returned HTTP %d", resp.StatusCode)
	}

	bodyBytes, err := io.ReadAll(io.LimitReader(resp.Body, cloneMaxTotalSize))
	if err != nil {
		return nil, fmt.Errorf("failed to read page: %w", err)
	}

	htmlContent := string(bodyBytes)

	// Strip tracking scripts if requested.
	if stripTracking {
		htmlContent = stripTrackingScripts(htmlContent)
	}

	// Strip scripts entirely if not including JS.
	if !includeJS {
		htmlContent = stripAllScripts(htmlContent)
	}

	// Localize CSS: download linked stylesheets and inline them.
	htmlContent = localizeLinkedCSS(htmlContent, parsedURL)

	// Localize JS (if including): download external scripts and inline them.
	if includeJS {
		htmlContent = localizeLinkedJSFromURL(htmlContent, parsedURL)
	}

	// Localize images: convert src to data URIs for small images.
	htmlContent = localizeImages(htmlContent, parsedURL)

	// Parse into definition.
	def := map[string]any{
		"schema_version": 1,
		"clone_source":   sourceURL,
		"clone_options": map[string]any{
			"include_js":     includeJS,
			"strip_tracking": stripTracking,
		},
		"pages":        []any{},
		"global_styles": "",
		"global_js":     "",
		"theme":         map[string]any{},
		"navigation":    []any{},
	}

	definition, err := ParseHTMLToDefinition(htmlContent, def)
	if err != nil {
		return nil, fmt.Errorf("failed to parse cloned HTML: %w", err)
	}

	// Set clone metadata on first page.
	if pages, ok := definition["pages"].([]any); ok && len(pages) > 0 {
		if page, ok := pages[0].(map[string]any); ok {
			page["name"] = "Cloned Page"
			if page["title"] == "" || page["title"] == nil {
				page["title"] = "Cloned Page"
			}
		}
	}

	return definition, nil
}

// fetchAsset downloads a single asset with timeout and size limit.
func fetchAsset(assetURL string) ([]byte, string, error) {
	client := &http.Client{Timeout: cloneAssetTimeout}
	resp, err := client.Get(assetURL)
	if err != nil {
		return nil, "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, "", fmt.Errorf("HTTP %d", resp.StatusCode)
	}

	data, err := io.ReadAll(io.LimitReader(resp.Body, cloneMaxAssetSize+1))
	if err != nil {
		return nil, "", err
	}
	if len(data) > cloneMaxAssetSize {
		return nil, "", fmt.Errorf("asset too large (>%d MB)", cloneMaxAssetSize/(1024*1024))
	}

	contentType := resp.Header.Get("Content-Type")
	if contentType == "" {
		contentType = http.DetectContentType(data)
	}
	return data, contentType, nil
}

// resolveURL resolves a relative href against the base page URL.
func resolveURL(base *url.URL, href string) string {
	if strings.HasPrefix(href, "//") {
		return base.Scheme + ":" + href
	}
	ref, err := url.Parse(href)
	if err != nil {
		return href
	}
	return base.ResolveReference(ref).String()
}

var imgSrcRegex = regexp.MustCompile(`(?i)(<img[^>]+src\s*=\s*["'])([^"']+)(["'])`)

// localizeImages converts small external images to base64 data URIs.
func localizeImages(htmlContent string, baseURL *url.URL) string {
	count := 0
	return imgSrcRegex.ReplaceAllStringFunc(htmlContent, func(match string) string {
		if count >= cloneMaxAssets {
			return match
		}
		sub := imgSrcRegex.FindStringSubmatch(match)
		if len(sub) < 4 {
			return match
		}
		src := sub[2]
		// Skip data URIs and anchors.
		if strings.HasPrefix(src, "data:") || strings.HasPrefix(src, "#") {
			return match
		}
		fullURL := resolveURL(baseURL, src)
		data, contentType, err := fetchAsset(fullURL)
		if err != nil || len(data) > cloneMaxImageSize {
			return match // Skip: keep original reference
		}
		count++

		// Determine MIME type.
		mimeType := contentType
		if idx := strings.Index(mimeType, ";"); idx >= 0 {
			mimeType = mimeType[:idx]
		}
		mimeType = strings.TrimSpace(mimeType)
		if mimeType == "" || mimeType == "application/octet-stream" {
			ext := path.Ext(src)
			mimeType = mime.TypeByExtension(ext)
			if mimeType == "" {
				mimeType = "image/png"
			}
		}

		dataURI := fmt.Sprintf("data:%s;base64,%s", mimeType, base64.StdEncoding.EncodeToString(data))
		return sub[1] + dataURI + sub[3]
	})
}

// localizeLinkedCSS downloads external CSS and inlines it.
func localizeLinkedCSS(htmlContent string, baseURL *url.URL) string {
	count := 0
	return linkStylesheetRegex.ReplaceAllStringFunc(htmlContent, func(tag string) string {
		if count >= cloneMaxAssets {
			return tag
		}
		hrefMatch := linkHrefRegex.FindStringSubmatch(tag)
		if len(hrefMatch) < 2 {
			return tag
		}
		href := hrefMatch[1]
		if strings.HasPrefix(href, "data:") {
			return tag
		}
		fullURL := resolveURL(baseURL, href)
		data, _, err := fetchAsset(fullURL)
		if err != nil {
			return tag // Keep original link
		}
		count++

		// Also localize url() references within the CSS.
		cssContent := localizeURLsInCSS(string(data), fullURL)
		return fmt.Sprintf("<style>/* %s */\n%s\n</style>", href, cssContent)
	})
}

var cssURLRegex = regexp.MustCompile(`url\(\s*["']?([^)"']+)["']?\s*\)`)

// localizeURLsInCSS resolves and inlines url() references in CSS content.
func localizeURLsInCSS(css, cssFileURL string) string {
	base, err := url.Parse(cssFileURL)
	if err != nil {
		return css
	}
	count := 0
	return cssURLRegex.ReplaceAllStringFunc(css, func(match string) string {
		if count >= cloneMaxAssets {
			return match
		}
		sub := cssURLRegex.FindStringSubmatch(match)
		if len(sub) < 2 {
			return match
		}
		href := sub[1]
		if strings.HasPrefix(href, "data:") || strings.HasPrefix(href, "#") {
			return match
		}
		fullURL := resolveURL(base, href)
		data, contentType, err := fetchAsset(fullURL)
		if err != nil || len(data) > cloneMaxImageSize {
			return match
		}
		count++
		mimeType := contentType
		if idx := strings.Index(mimeType, ";"); idx >= 0 {
			mimeType = mimeType[:idx]
		}
		mimeType = strings.TrimSpace(mimeType)
		if mimeType == "" {
			ext := path.Ext(href)
			mimeType = mime.TypeByExtension(ext)
		}
		if mimeType == "" {
			mimeType = "application/octet-stream"
		}
		dataURI := fmt.Sprintf("data:%s;base64,%s", mimeType, base64.StdEncoding.EncodeToString(data))
		return fmt.Sprintf("url(\"%s\")", dataURI)
	})
}

var externalScriptRegex = regexp.MustCompile(`(?i)<script[^>]+src\s*=\s*["']([^"']+)["'][^>]*>\s*</script>`)

// localizeLinkedJSFromURL downloads external scripts and inlines them.
func localizeLinkedJSFromURL(htmlContent string, baseURL *url.URL) string {
	count := 0
	return externalScriptRegex.ReplaceAllStringFunc(htmlContent, func(tag string) string {
		if count >= cloneMaxAssets {
			return tag
		}
		sub := externalScriptRegex.FindStringSubmatch(tag)
		if len(sub) < 2 {
			return tag
		}
		src := sub[1]
		if strings.HasPrefix(src, "data:") {
			return tag
		}
		fullURL := resolveURL(baseURL, src)
		data, _, err := fetchAsset(fullURL)
		if err != nil {
			return tag
		}
		count++
		return fmt.Sprintf("<script>/* %s */\n%s\n</script>", src, string(data))
	})
}

var trackingScriptPatterns = []string{
	`google-analytics\.com`, `googletagmanager\.com`, `analytics\.`,
	`facebook\.net`, `fbevents\.js`, `hotjar\.com`, `segment\.com`,
	`mixpanel\.com`, `amplitude\.com`, `heap-analytics`,
}

// stripTrackingScripts removes common analytics/tracking script tags.
func stripTrackingScripts(html string) string {
	for _, pattern := range trackingScriptPatterns {
		re := regexp.MustCompile(`(?is)<script[^>]*` + pattern + `[^>]*>.*?</script>`)
		html = re.ReplaceAllString(html, "")
	}
	return html
}

var allScriptRegex = regexp.MustCompile(`(?is)<script[^>]*>.*?</script>`)

// stripAllScripts removes all <script> tags from HTML.
func stripAllScripts(html string) string {
	return allScriptRegex.ReplaceAllString(html, "")
}

// parseHTMLElements does a best-effort mapping of HTML elements to builder components.
func parseHTMLElements(htmlContent string) []any {
	// Extract the body content if present.
	body := htmlContent
	if idx := strings.Index(strings.ToLower(body), "<body"); idx >= 0 {
		end := strings.Index(body[idx:], ">")
		if end >= 0 {
			body = body[idx+end+1:]
		}
		if closeIdx := strings.LastIndex(strings.ToLower(body), "</body>"); closeIdx >= 0 {
			body = body[:closeIdx]
		}
	}

	// For simplicity, wrap entire body as raw_html if it contains complex structure.
	// A full HTML parser would recursively convert elements, but that complexity
	// is better served by a proper HTML parsing library in a future enhancement.
	body = strings.TrimSpace(body)
	if body == "" {
		return []any{}
	}

	// Try to identify simple top-level elements.
	components := []any{}

	// Check for form elements.
	if containsFormElements(body) {
		components = append(components, map[string]any{
			"component_id": uuid.New().String(),
			"type":         "raw_html",
			"properties": map[string]any{
				"content": body,
			},
			"children":       []any{},
			"event_bindings": []any{},
		})
	} else {
		// Split by major block elements.
		components = append(components, map[string]any{
			"component_id": uuid.New().String(),
			"type":         "raw_html",
			"properties": map[string]any{
				"content": body,
			},
			"children":       []any{},
			"event_bindings": []any{},
		})
	}

	return components
}

var (
	styleRegex  = regexp.MustCompile(`(?is)<style[^>]*>(.*?)</style>`)
	scriptRegex = regexp.MustCompile(`(?is)<script[^>]*>(.*?)</script>`)
	titleRegex  = regexp.MustCompile(`(?is)<title[^>]*>(.*?)</title>`)
	formRegex   = regexp.MustCompile(`(?i)<(form|input|select|textarea|button)[\s>]`)
)

func extractStyles(html string) string {
	var styles []string
	matches := styleRegex.FindAllStringSubmatch(html, -1)
	for _, m := range matches {
		if len(m) > 1 && strings.TrimSpace(m[1]) != "" {
			styles = append(styles, strings.TrimSpace(m[1]))
		}
	}
	return strings.Join(styles, "\n\n")
}

func extractScripts(html string) string {
	var scripts []string
	matches := scriptRegex.FindAllStringSubmatch(html, -1)
	for _, m := range matches {
		if len(m) > 1 && strings.TrimSpace(m[1]) != "" {
			scripts = append(scripts, strings.TrimSpace(m[1]))
		}
	}
	return strings.Join(scripts, "\n\n")
}

func extractTitle(html string) string {
	match := titleRegex.FindStringSubmatch(html)
	if len(match) > 1 {
		return strings.TrimSpace(match[1])
	}
	return ""
}

func containsFormElements(html string) bool {
	return formRegex.MatchString(html)
}

func copyDefinition(def map[string]any) map[string]any {
	if def == nil {
		return defaultDefinition()
	}
	// Deep copy via JSON round-trip to prevent shared nested map/slice mutation.
	data, err := json.Marshal(def)
	if err != nil {
		// Fallback to shallow copy if marshal fails (should not happen for valid definitions).
		result := make(map[string]any, len(def))
		for k, v := range def {
			result[k] = v
		}
		return result
	}
	var result map[string]any
	if err := json.Unmarshal(data, &result); err != nil {
		result = make(map[string]any, len(def))
		for k, v := range def {
			result[k] = v
		}
		return result
	}
	return result
}

func ensurePages(def map[string]any) []any {
	pages, ok := def["pages"].([]any)
	if !ok || len(pages) == 0 {
		pages = []any{
			map[string]any{
				"page_id":        "page-1",
				"name":           "Page 1",
				"route":          "/",
				"title":          "Page 1",
				"favicon":        "",
				"meta_tags":      []any{},
				"component_tree": []any{},
				"page_styles":    "",
				"page_js":        "",
			},
		}
		def["pages"] = pages
	}
	return pages
}

// ImportFromZIP extracts a ZIP archive, finds the entry-point HTML file,
// inlines referenced CSS/JS assets, and returns the result through ParseHTMLToDefinition.
// Assets are inlined into the HTML to avoid needing separate asset storage.
func ImportFromZIP(zipReader *zip.Reader, existingDef map[string]any) (map[string]any, error) {
	if len(zipReader.File) > maxZIPFileCount {
		return nil, fmt.Errorf("ZIP contains too many files (max %d)", maxZIPFileCount)
	}

	// Read all files into memory (with path traversal protection).
	files := make(map[string][]byte)
	var totalSize int64
	for _, f := range zipReader.File {
		// Path traversal protection.
		name := filepath.ToSlash(f.Name)
		if strings.Contains(name, "..") {
			return nil, fmt.Errorf("ZIP contains path traversal entry: %s", name)
		}
		// Skip directories.
		if f.FileInfo().IsDir() {
			continue
		}
		// Size check.
		totalSize += int64(f.UncompressedSize64)
		if totalSize > maxZIPSize {
			return nil, fmt.Errorf("ZIP uncompressed content exceeds %d MB limit", maxZIPSize/(1024*1024))
		}

		rc, err := f.Open()
		if err != nil {
			return nil, fmt.Errorf("failed to read ZIP entry %s: %w", name, err)
		}
		data, err := io.ReadAll(io.LimitReader(rc, maxZIPSize))
		rc.Close()
		if err != nil {
			return nil, fmt.Errorf("failed to read ZIP entry %s: %w", name, err)
		}
		files[name] = data
	}

	// Find entry-point HTML file.
	entryPoint := findEntryHTML(files)
	if entryPoint == "" {
		return nil, fmt.Errorf("no HTML file found in ZIP (looked for index.html or any .html file)")
	}

	html := string(files[entryPoint])
	entryDir := filepath.ToSlash(filepath.Dir(entryPoint))
	if entryDir == "." {
		entryDir = ""
	}

	// Inline CSS files referenced by <link rel="stylesheet">.
	html = inlineLinkedCSS(html, entryDir, files)

	// Inline JS files referenced by <script src="...">.
	html = inlineLinkedJS(html, entryDir, files)

	return ParseHTMLToDefinition(html, existingDef)
}

// findEntryHTML finds the best entry-point HTML file in the ZIP.
func findEntryHTML(files map[string][]byte) string {
	// Prefer index.html at root.
	if _, ok := files["index.html"]; ok {
		return "index.html"
	}
	// Check for index.html in any subdirectory.
	for name := range files {
		if filepath.Base(name) == "index.html" {
			return name
		}
	}
	// Fall back to any .html file.
	for name := range files {
		if strings.HasSuffix(strings.ToLower(name), ".html") || strings.HasSuffix(strings.ToLower(name), ".htm") {
			return name
		}
	}
	return ""
}

var linkStylesheetRegex = regexp.MustCompile(`(?i)<link[^>]+rel\s*=\s*["']stylesheet["'][^>]*>`)
var linkHrefRegex = regexp.MustCompile(`(?i)href\s*=\s*["']([^"']+)["']`)

// inlineLinkedCSS replaces <link rel="stylesheet" href="local.css"> with inline <style> blocks.
func inlineLinkedCSS(html, baseDir string, files map[string][]byte) string {
	return linkStylesheetRegex.ReplaceAllStringFunc(html, func(tag string) string {
		hrefMatch := linkHrefRegex.FindStringSubmatch(tag)
		if len(hrefMatch) < 2 {
			return tag
		}
		href := hrefMatch[1]
		// Skip external URLs.
		if strings.HasPrefix(href, "http://") || strings.HasPrefix(href, "https://") || strings.HasPrefix(href, "//") {
			return tag
		}
		// Resolve relative path.
		resolved := resolveAssetPath(baseDir, href)
		data, ok := files[resolved]
		if !ok {
			return tag // Keep original reference if file not found.
		}
		return fmt.Sprintf("<style>/* %s */\n%s\n</style>", href, string(data))
	})
}

var scriptSrcRegex = regexp.MustCompile(`(?i)<script[^>]+src\s*=\s*["']([^"']+)["'][^>]*>\s*</script>`)

// inlineLinkedJS replaces <script src="local.js"></script> with inline <script> blocks.
func inlineLinkedJS(html, baseDir string, files map[string][]byte) string {
	return scriptSrcRegex.ReplaceAllStringFunc(html, func(tag string) string {
		match := scriptSrcRegex.FindStringSubmatch(tag)
		if len(match) < 2 {
			return tag
		}
		src := match[1]
		// Skip external URLs.
		if strings.HasPrefix(src, "http://") || strings.HasPrefix(src, "https://") || strings.HasPrefix(src, "//") {
			return tag
		}
		resolved := resolveAssetPath(baseDir, src)
		data, ok := files[resolved]
		if !ok {
			return tag
		}
		return fmt.Sprintf("<script>/* %s */\n%s\n</script>", src, string(data))
	})
}

// resolveAssetPath resolves a relative asset path against a base directory within the ZIP.
func resolveAssetPath(baseDir, href string) string {
	href = strings.TrimPrefix(href, "./")
	if baseDir == "" {
		return filepath.ToSlash(filepath.Clean(href))
	}
	return filepath.ToSlash(filepath.Clean(filepath.Join(baseDir, href)))
}
