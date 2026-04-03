// Package categorization implements web filtering service checks for domain categorization.
// Checks are performed against Bluecoat/Symantec, Zscaler, Fortiguard, and Palo Alto
// URL filtering services via HTTP scraping.
package categorization

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// CategoryResult is the result of a single categorization check.
type CategoryResult struct {
	Service     string `json:"service"`
	Category    string `json:"category"`
	Status      string `json:"status"` // categorized, uncategorized, flagged, unknown
	RawResponse string `json:"raw_response,omitempty"`
	Error       string `json:"error,omitempty"`
}

// CategorizationChecker is the interface for web filtering service checkers.
type CategorizationChecker interface {
	ServiceName() string
	CheckCategory(ctx context.Context, domain string) (CategoryResult, error)
}

var httpClient = &http.Client{
	Timeout: 10 * time.Second,
}

// flaggedKeywords are category names that indicate a domain is flagged as suspicious/malicious.
var flaggedKeywords = []string{
	"phishing", "malware", "spam", "suspicious", "malicious", "botnet",
	"spyware", "adware", "hacking", "proxy avoidance", "anonymizer",
	"weapons", "violence", "illegal",
}

func isFlagged(category string) bool {
	lower := strings.ToLower(category)
	for _, kw := range flaggedKeywords {
		if strings.Contains(lower, kw) {
			return true
		}
	}
	return false
}

func mapCategory(raw string) string {
	if raw == "" || strings.EqualFold(raw, "uncategorized") || strings.EqualFold(raw, "not categorized") {
		return "uncategorized"
	}
	if isFlagged(raw) {
		return "flagged"
	}
	return "categorized"
}

// --- Bluecoat/Symantec WebPulse ---

// BluecoatChecker checks Bluecoat/Symantec WebPulse.
type BluecoatChecker struct{}

// ServiceName returns the service identifier.
func (c *BluecoatChecker) ServiceName() string { return "bluecoat" }

// CheckCategory checks the domain against Bluecoat/Symantec WebPulse.
func (c *BluecoatChecker) CheckCategory(ctx context.Context, domain string) (CategoryResult, error) {
	url := "https://sitereview.bluecoat.com/resource/lookup?url=" + domain

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return unknownResult("bluecoat", fmt.Sprintf("build request: %v", err)), nil
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "Mozilla/5.0 (compatible; DomainHealthChecker/1.0)")

	resp, err := httpClient.Do(req)
	if err != nil {
		return unknownResult("bluecoat", fmt.Sprintf("http: %v", err)), nil
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return unknownResult("bluecoat", fmt.Sprintf("http status %d", resp.StatusCode)), nil
	}

	body, _ := io.ReadAll(io.LimitReader(resp.Body, 8192))
	raw := string(body)

	// Best-effort JSON parsing for "categorization" field.
	category := extractJSONField(raw, "categorization", "name")
	if category == "" {
		category = extractJSONField(raw, "categories", "")
	}
	if category == "" {
		return unknownResult("bluecoat", "could not parse category"), nil
	}

	status := mapCategory(category)
	return CategoryResult{
		Service:     "bluecoat",
		Category:    category,
		Status:      status,
		RawResponse: truncate(raw, 512),
	}, nil
}

// --- Zscaler ---

// ZscalerChecker checks the Zscaler URL category lookup.
type ZscalerChecker struct{}

// ServiceName returns the service identifier.
func (c *ZscalerChecker) ServiceName() string { return "zscaler" }

// CheckCategory checks the domain against Zscaler URL filtering.
func (c *ZscalerChecker) CheckCategory(ctx context.Context, domain string) (CategoryResult, error) {
	url := "https://sitereview.zscaler.com/api/getsitecategory?url=" + domain

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return unknownResult("zscaler", fmt.Sprintf("build request: %v", err)), nil
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "Mozilla/5.0 (compatible; DomainHealthChecker/1.0)")

	resp, err := httpClient.Do(req)
	if err != nil {
		return unknownResult("zscaler", fmt.Sprintf("http: %v", err)), nil
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return unknownResult("zscaler", fmt.Sprintf("http status %d", resp.StatusCode)), nil
	}

	body, _ := io.ReadAll(io.LimitReader(resp.Body, 8192))
	raw := string(body)

	// Look for "primaryCategory" or "categories" in the JSON.
	category := extractJSONField(raw, "primaryCategory", "")
	if category == "" {
		category = extractJSONField(raw, "categories", "")
	}
	if category == "" {
		return unknownResult("zscaler", "could not parse category"), nil
	}

	status := mapCategory(category)
	return CategoryResult{
		Service:     "zscaler",
		Category:    category,
		Status:      status,
		RawResponse: truncate(raw, 512),
	}, nil
}

// --- Fortiguard ---

// FortiguardChecker checks the Fortiguard web filter lookup.
type FortiguardChecker struct{}

// ServiceName returns the service identifier.
func (c *FortiguardChecker) ServiceName() string { return "fortiguard" }

// CheckCategory checks the domain against Fortiguard URL filtering.
func (c *FortiguardChecker) CheckCategory(ctx context.Context, domain string) (CategoryResult, error) {
	url := "https://www.fortiguard.com/webfilter?q=" + domain

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return unknownResult("fortiguard", fmt.Sprintf("build request: %v", err)), nil
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (compatible; DomainHealthChecker/1.0)")

	resp, err := httpClient.Do(req)
	if err != nil {
		return unknownResult("fortiguard", fmt.Sprintf("http: %v", err)), nil
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return unknownResult("fortiguard", fmt.Sprintf("http status %d", resp.StatusCode)), nil
	}

	body, _ := io.ReadAll(io.LimitReader(resp.Body, 32768))
	raw := string(body)

	// Parse HTML: look for category name between specific tags.
	category := extractHTMLCategory(raw)
	if category == "" {
		return unknownResult("fortiguard", "could not parse category"), nil
	}

	status := mapCategory(category)
	return CategoryResult{
		Service:     "fortiguard",
		Category:    category,
		Status:      status,
		RawResponse: truncate(raw, 512),
	}, nil
}

// --- Palo Alto URL Filtering ---

// PaloAltoChecker checks the Palo Alto URL Filtering service.
type PaloAltoChecker struct{}

// ServiceName returns the service identifier.
func (c *PaloAltoChecker) ServiceName() string { return "paloalto" }

// CheckCategory checks the domain against Palo Alto URL filtering.
func (c *PaloAltoChecker) CheckCategory(ctx context.Context, domain string) (CategoryResult, error) {
	url := "https://urlfiltering.paloaltonetworks.com/query/?category=any&url=" + domain

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return unknownResult("paloalto", fmt.Sprintf("build request: %v", err)), nil
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "Mozilla/5.0 (compatible; DomainHealthChecker/1.0)")

	resp, err := httpClient.Do(req)
	if err != nil {
		return unknownResult("paloalto", fmt.Sprintf("http: %v", err)), nil
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return unknownResult("paloalto", fmt.Sprintf("http status %d", resp.StatusCode)), nil
	}

	body, _ := io.ReadAll(io.LimitReader(resp.Body, 8192))
	raw := string(body)

	category := extractJSONField(raw, "category", "")
	if category == "" {
		category = extractJSONField(raw, "categories", "")
	}
	if category == "" {
		return unknownResult("paloalto", "could not parse category"), nil
	}

	status := mapCategory(category)
	return CategoryResult{
		Service:     "paloalto",
		Category:    category,
		Status:      status,
		RawResponse: truncate(raw, 512),
	}, nil
}

// --- Helpers ---

// unknownResult returns a CategoryResult with status "unknown".
func unknownResult(service, reason string) CategoryResult {
	return CategoryResult{
		Service:  service,
		Category: "",
		Status:   "unknown",
		Error:    reason,
	}
}

// extractJSONField is a simple string-based extraction of a JSON field value.
// It handles the common case of `"key":"value"` or `"key": "value"`.
func extractJSONField(body, key, subkey string) string {
	needle := `"` + key + `"`
	idx := strings.Index(body, needle)
	if idx < 0 {
		return ""
	}
	rest := body[idx+len(needle):]
	// Skip whitespace and colon.
	rest = strings.TrimLeft(rest, ": \t\r\n")
	if len(rest) == 0 {
		return ""
	}

	if rest[0] == '"' {
		// String value.
		end := strings.IndexByte(rest[1:], '"')
		if end < 0 {
			return ""
		}
		return rest[1 : end+1]
	}

	if rest[0] == '[' || rest[0] == '{' {
		// Nested object/array — look for subkey inside.
		if subkey != "" {
			return extractJSONField(rest, subkey, "")
		}
		// Return the raw inner content up to closing bracket.
		end := strings.IndexAny(rest[1:], "]}")
		if end >= 0 {
			inner := strings.Trim(rest[1:end+1], `"[] `)
			return inner
		}
	}
	return ""
}

// extractHTMLCategory parses the Fortiguard HTML response for a category name.
func extractHTMLCategory(body string) string {
	// Fortiguard puts category in a <div class="info_head">Category</div>
	// followed by a <div class="info_data">...</div>.
	const marker = `class="info_data"`
	idx := strings.Index(body, marker)
	if idx < 0 {
		return ""
	}
	rest := body[idx+len(marker):]
	start := strings.IndexByte(rest, '>')
	if start < 0 {
		return ""
	}
	rest = rest[start+1:]
	end := strings.IndexByte(rest, '<')
	if end < 0 {
		return ""
	}
	return strings.TrimSpace(rest[:end])
}

// truncate limits a string to at most n bytes.
func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n]
}

// AllCheckers returns one instance of each categorization checker.
func AllCheckers() []CategorizationChecker {
	return []CategorizationChecker{
		&BluecoatChecker{},
		&ZscalerChecker{},
		&FortiguardChecker{},
		&PaloAltoChecker{},
	}
}
