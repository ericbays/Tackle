// Package emailtemplate implements business logic for email template management.
package emailtemplate

import (
	"bytes"
	"fmt"
	"log/slog"
	"mime"
	"regexp"
	"strings"
	texttemplate "text/template"
	"time"
)

// TargetVars holds target-specific template variables.
type TargetVars struct {
	FirstName  string
	LastName   string
	FullName   string
	Email      string
	Position   string
	Department string
	Custom     map[string]string
}

// CampaignVars holds campaign-level template variables.
type CampaignVars struct {
	Name     string
	FromName string
}

// SenderVars holds sender-related template variables.
type SenderVars struct {
	Email string
}

// TrackingVars holds tracking URLs for email engagement tracking.
type TrackingVars struct {
	Pixel string // <img> tag with tracking URL
	Link  string // base click tracking URL prefix
	URL   string // phishing landing page URL
}

// CurrentVars holds date/time template variables.
type CurrentVars struct {
	Date string // formatted date (e.g., "March 26, 2026")
	Year string // 4-digit year
}

// SendVars holds all variables available during email rendering for campaign sends.
type SendVars struct {
	Target   TargetVars
	Campaign CampaignVars
	Sender   SenderVars
	Tracking TrackingVars
	Current  CurrentVars
}

// RenderResult holds the rendered output for a template ready to send.
type RenderResult struct {
	Subject  string
	HTMLBody string
	TextBody string
}

// tokenMapping maps requirements-style tokens to Go template syntax.
var tokenMapping = map[string]string{
	"target.first_name":   ".Target.FirstName",
	"target.last_name":    ".Target.LastName",
	"target.full_name":    ".Target.FullName",
	"target.email":        ".Target.Email",
	"target.position":     ".Target.Position",
	"target.department":   ".Target.Department",
	"campaign.name":       ".Campaign.Name",
	"campaign.from_name":  ".Campaign.FromName",
	"sender.email":        ".Sender.Email",
	"tracking.pixel":      ".Tracking.Pixel",
	"tracking.link":       ".Tracking.Link",
	"phishing.url":        ".Tracking.URL",
	"current.date":        ".Current.Date",
	"current.year":        ".Current.Year",
}

// customFieldRegex matches {{target.custom.<field_name>}} tokens.
var customFieldRegex = regexp.MustCompile(`\{\{\s*target\.custom\.(\w+)\s*\}\}`)

// normalizeTokens converts requirements-style tokens (e.g., {{target.first_name}})
// to Go template syntax (e.g., {{.Target.FirstName}}) for rendering.
func normalizeTokens(tmplStr string) string {
	result := tmplStr

	// Replace custom field tokens first: {{target.custom.<field>}} → {{index .Target.Custom "<field>"}}
	result = customFieldRegex.ReplaceAllStringFunc(result, func(match string) string {
		sub := customFieldRegex.FindStringSubmatch(match)
		if len(sub) < 2 {
			return match
		}
		return fmt.Sprintf(`{{index .Target.Custom "%s"}}`, sub[1])
	})

	// Replace standard tokens.
	for reqToken, goToken := range tokenMapping {
		// Match {{token}} with optional whitespace inside braces.
		old := fmt.Sprintf("{{%s}}", reqToken)
		new := fmt.Sprintf("{{%s}}", goToken)
		result = strings.ReplaceAll(result, old, new)

		// Also handle with spaces: {{ token }}.
		old = fmt.Sprintf("{{ %s }}", reqToken)
		result = strings.ReplaceAll(result, old, new)
	}

	// Also convert old-style flat tokens for backward compatibility.
	// {{.FirstName}} → {{.Target.FirstName}}, etc.
	oldStyleMapping := map[string]string{
		"{{.FirstName}}":    "{{.Target.FirstName}}",
		"{{.LastName}}":     "{{.Target.LastName}}",
		"{{.Email}}":        "{{.Target.Email}}",
		"{{.TrackingURL}}":  "{{.Tracking.URL}}",
		"{{.TargetURL}}":    "{{.Tracking.URL}}",
		"{{.CampaignName}}": "{{.Campaign.Name}}",
	}
	for old, new := range oldStyleMapping {
		result = strings.ReplaceAll(result, old, new)
	}

	return result
}

// RenderForSend renders a template with full campaign/target variables for email sending.
// It normalizes requirement-style tokens, renders via Go templates, and logs warnings
// for empty token values.
func (s *Service) RenderForSend(templateID string, vars SendVars, logger *slog.Logger) (RenderResult, error) {
	tmpl, err := s.repo.GetByID(nil, templateID)
	if err != nil {
		return RenderResult{}, fmt.Errorf("render for send: get template: %w", err)
	}

	return RenderForSendRaw(tmpl.Subject, tmpl.HTMLBody, tmpl.TextBody, vars, logger)
}

// RenderForSendRaw renders raw template strings with full campaign/target variables.
func RenderForSendRaw(subject, htmlBody, textBody string, vars SendVars, logger *slog.Logger) (RenderResult, error) {
	// Populate current date/time vars if not already set.
	if vars.Current.Date == "" {
		vars.Current.Date = time.Now().Format("January 2, 2006")
	}
	if vars.Current.Year == "" {
		vars.Current.Year = time.Now().Format("2006")
	}

	// Ensure FullName is populated.
	if vars.Target.FullName == "" && (vars.Target.FirstName != "" || vars.Target.LastName != "") {
		vars.Target.FullName = strings.TrimSpace(vars.Target.FirstName + " " + vars.Target.LastName)
	}

	// Ensure Custom map is initialized.
	if vars.Target.Custom == nil {
		vars.Target.Custom = make(map[string]string)
	}

	// Normalize tokens in all template strings.
	normSubject := normalizeTokens(subject)
	normHTML := normalizeTokens(htmlBody)
	normText := normalizeTokens(textBody)

	// Render subject (text template).
	renderedSubject, err := renderSendText(normSubject, vars)
	if err != nil {
		return RenderResult{}, fmt.Errorf("render for send: subject: %w", err)
	}

	// Render HTML body.
	renderedHTML, err := renderSendHTML(normHTML, vars)
	if err != nil {
		return RenderResult{}, fmt.Errorf("render for send: html_body: %w", err)
	}

	// Render text body.
	renderedText, err := renderSendText(normText, vars)
	if err != nil {
		return RenderResult{}, fmt.Errorf("render for send: text_body: %w", err)
	}

	// Auto-inject tracking pixel if not already present in HTML body.
	if vars.Tracking.Pixel != "" && renderedHTML != "" {
		if !strings.Contains(renderedHTML, vars.Tracking.Pixel) {
			renderedHTML = injectTrackingPixel(renderedHTML, vars.Tracking.Pixel)
		}
	}

	// Log warnings for empty token values.
	if logger != nil {
		logEmptyVarWarnings(vars, logger)
	}

	return RenderResult{
		Subject:  renderedSubject,
		HTMLBody: renderedHTML,
		TextBody: renderedText,
	}, nil
}

// logEmptyVarWarnings logs warnings when important template variables are empty.
func logEmptyVarWarnings(vars SendVars, logger *slog.Logger) {
	if vars.Target.Email == "" {
		logger.Warn("template variable Target.Email is empty")
	}
	if vars.Target.FirstName == "" {
		logger.Warn("template variable Target.FirstName is empty")
	}
	if vars.Tracking.URL == "" {
		logger.Warn("template variable Tracking.URL is empty — landing page links will not work")
	}
	if vars.Tracking.Pixel == "" {
		logger.Warn("template variable Tracking.Pixel is empty — open tracking will not work")
	}
}

// injectTrackingPixel inserts a tracking pixel <img> tag into HTML content.
// If a </body> tag exists, the pixel is inserted just before it.
// Otherwise, it is appended to the end of the HTML.
func injectTrackingPixel(html, pixelTag string) string {
	idx := strings.LastIndex(strings.ToLower(html), "</body>")
	if idx >= 0 {
		return html[:idx] + pixelTag + "\n" + html[idx:]
	}
	return html + "\n" + pixelTag
}

// --- ECOMP-03: Header Injection Sanitization ---

// sanitizeHeaderValue strips CR and LF characters from header-bound values
// to prevent header injection attacks. Body content should NOT be sanitized.
func sanitizeHeaderValue(s string) string {
	s = strings.ReplaceAll(s, "\r", "")
	s = strings.ReplaceAll(s, "\n", "")
	return s
}

// --- ECOMP-04: RFC 2047 Encoded-Word for Non-ASCII Headers ---

// encodeHeaderIfNeeded applies RFC 2047 Q-encoding to a header value
// only if it contains non-ASCII characters.
func encodeHeaderIfNeeded(s string) string {
	for _, r := range s {
		if r > 127 {
			return mime.QEncoding.Encode("utf-8", s)
		}
	}
	return s
}

// --- ECOMP-05: Message-ID Domain Fix ---

// ExtractFromDomain extracts the domain part from an email address.
// Returns the domain (e.g., "example.com") or an empty string if invalid.
func ExtractFromDomain(fromAddress string) string {
	parts := strings.SplitN(fromAddress, "@", 2)
	if len(parts) == 2 && parts[1] != "" {
		return parts[1]
	}
	return ""
}

// GenerateMessageID creates an RFC 5322 compliant Message-ID using the
// From address domain instead of the SMTP server hostname.
func GenerateMessageID(uuid, fromAddress string) string {
	domain := ExtractFromDomain(fromAddress)
	if domain == "" {
		domain = "localhost"
	}
	return fmt.Sprintf("<%s@%s>", uuid, domain)
}

// --- Composition Helpers ---

// ComposeHeaders applies header sanitization and RFC 2047 encoding
// to email header values (subject, from name, reply-to).
// Returns sanitized and encoded values ready for SMTP headers.
func ComposeHeaders(subject, fromName, replyTo string) (encodedSubject, encodedFromName, encodedReplyTo string) {
	// Sanitize to prevent header injection.
	encodedSubject = sanitizeHeaderValue(subject)
	encodedFromName = sanitizeHeaderValue(fromName)
	encodedReplyTo = sanitizeHeaderValue(replyTo)

	// Apply RFC 2047 encoding for non-ASCII.
	encodedSubject = encodeHeaderIfNeeded(encodedSubject)
	encodedFromName = encodeHeaderIfNeeded(encodedFromName)
	encodedReplyTo = encodeHeaderIfNeeded(encodedReplyTo)

	return
}

// renderSendText renders a text template with SendVars.
func renderSendText(tmplStr string, vars SendVars) (string, error) {
	tmpl, err := texttemplate.New("t").Option("missingkey=zero").Parse(tmplStr)
	if err != nil {
		return "", err
	}
	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, vars); err != nil {
		return "", err
	}
	return buf.String(), nil
}

// renderSendHTML renders an HTML template with SendVars.
// Uses text/template (not html/template) because email templates intentionally
// contain raw HTML tokens like tracking pixels that must not be escaped.
func renderSendHTML(tmplStr string, vars SendVars) (string, error) {
	tmpl, err := texttemplate.New("h").Option("missingkey=zero").Parse(tmplStr)
	if err != nil {
		return "", err
	}
	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, vars); err != nil {
		return "", err
	}
	return buf.String(), nil
}
