package emailtemplate

import (
	"log/slog"
	"os"
	"strings"
	"testing"
)

func TestNormalizeTokens_StandardTokens(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "target first name",
			input: "Hello {{target.first_name}}",
			want:  "Hello {{.Target.FirstName}}",
		},
		{
			name:  "target last name",
			input: "Dear {{target.last_name}}",
			want:  "Dear {{.Target.LastName}}",
		},
		{
			name:  "target full name",
			input: "Hi {{target.full_name}}",
			want:  "Hi {{.Target.FullName}}",
		},
		{
			name:  "target email",
			input: "Sent to {{target.email}}",
			want:  "Sent to {{.Target.Email}}",
		},
		{
			name:  "target position",
			input: "Role: {{target.position}}",
			want:  "Role: {{.Target.Position}}",
		},
		{
			name:  "target department",
			input: "Dept: {{target.department}}",
			want:  "Dept: {{.Target.Department}}",
		},
		{
			name:  "campaign name",
			input: "Campaign: {{campaign.name}}",
			want:  "Campaign: {{.Campaign.Name}}",
		},
		{
			name:  "campaign from name",
			input: "From: {{campaign.from_name}}",
			want:  "From: {{.Campaign.FromName}}",
		},
		{
			name:  "sender email",
			input: "Reply: {{sender.email}}",
			want:  "Reply: {{.Sender.Email}}",
		},
		{
			name:  "tracking pixel",
			input: "{{tracking.pixel}}",
			want:  "{{.Tracking.Pixel}}",
		},
		{
			name:  "tracking link",
			input: "{{tracking.link}}",
			want:  "{{.Tracking.Link}}",
		},
		{
			name:  "phishing url",
			input: "Visit {{phishing.url}}",
			want:  "Visit {{.Tracking.URL}}",
		},
		{
			name:  "current date",
			input: "Date: {{current.date}}",
			want:  "Date: {{.Current.Date}}",
		},
		{
			name:  "current year",
			input: "© {{current.year}}",
			want:  "© {{.Current.Year}}",
		},
		{
			name:  "with spaces inside braces",
			input: "Hello {{ target.first_name }}",
			want:  "Hello {{.Target.FirstName}}",
		},
		{
			name:  "multiple tokens in one string",
			input: "Hi {{target.first_name}} {{target.last_name}}, welcome to {{campaign.name}}",
			want:  "Hi {{.Target.FirstName}} {{.Target.LastName}}, welcome to {{.Campaign.Name}}",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := normalizeTokens(tc.input)
			if got != tc.want {
				t.Errorf("normalizeTokens(%q) =\n  %q\nwant:\n  %q", tc.input, got, tc.want)
			}
		})
	}
}

func TestNormalizeTokens_CustomFields(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "custom field employee_id",
			input: `ID: {{target.custom.employee_id}}`,
			want:  `ID: {{index .Target.Custom "employee_id"}}`,
		},
		{
			name:  "custom field with underscores",
			input: `{{target.custom.cost_center_code}}`,
			want:  `{{index .Target.Custom "cost_center_code"}}`,
		},
		{
			name:  "multiple custom fields",
			input: `{{target.custom.office}} - {{target.custom.badge_number}}`,
			want:  `{{index .Target.Custom "office"}} - {{index .Target.Custom "badge_number"}}`,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := normalizeTokens(tc.input)
			if got != tc.want {
				t.Errorf("normalizeTokens(%q) =\n  %q\nwant:\n  %q", tc.input, got, tc.want)
			}
		})
	}
}

func TestNormalizeTokens_OldStyleBackcompat(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "old FirstName",
			input: "Hello {{.FirstName}}",
			want:  "Hello {{.Target.FirstName}}",
		},
		{
			name:  "old TrackingURL",
			input: "Visit {{.TrackingURL}}",
			want:  "Visit {{.Tracking.URL}}",
		},
		{
			name:  "old CampaignName",
			input: "Campaign: {{.CampaignName}}",
			want:  "Campaign: {{.Campaign.Name}}",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := normalizeTokens(tc.input)
			if got != tc.want {
				t.Errorf("normalizeTokens(%q) =\n  %q\nwant:\n  %q", tc.input, got, tc.want)
			}
		})
	}
}

func TestRenderForSendRaw_AllTokenTypes(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelWarn}))

	vars := SendVars{
		Target: TargetVars{
			FirstName:  "Jane",
			LastName:   "Smith",
			Email:      "jane@example.com",
			Position:   "Engineer",
			Department: "IT",
			Custom: map[string]string{
				"employee_id": "EMP-12345",
				"office":      "Building A",
			},
		},
		Campaign: CampaignVars{
			Name:     "Q1 Assessment",
			FromName: "IT Support",
		},
		Sender: SenderVars{
			Email: "support@company.com",
		},
		Tracking: TrackingVars{
			Pixel: `<img src="https://track.example.com/t/abc.gif" width="1" height="1" style="display:none" alt="" />`,
			Link:  "https://track.example.com/c/abc/",
			URL:   "https://landing.example.com/l/abc",
		},
		Current: CurrentVars{
			Date: "March 26, 2026",
			Year: "2026",
		},
	}

	subject := "Hi {{target.first_name}}, action required for {{campaign.name}}"
	htmlBody := `<html><body>
<p>Dear {{target.full_name}},</p>
<p>Position: {{target.position}}, Dept: {{target.department}}</p>
<p>Employee ID: {{target.custom.employee_id}}</p>
<p>Office: {{target.custom.office}}</p>
<p>From: {{campaign.from_name}} &lt;{{sender.email}}&gt;</p>
<p><a href="{{phishing.url}}">Click here</a></p>
<p>Date: {{current.date}} | &copy; {{current.year}}</p>
{{tracking.pixel}}
</body></html>`
	textBody := `Hi {{target.first_name}} {{target.last_name}},

Visit: {{phishing.url}}
Date: {{current.date}}
`

	result, err := RenderForSendRaw(subject, htmlBody, textBody, vars, logger)
	if err != nil {
		t.Fatalf("RenderForSendRaw() error: %v", err)
	}

	// Subject.
	if !strings.Contains(result.Subject, "Jane") {
		t.Error("subject should contain target first name 'Jane'")
	}
	if !strings.Contains(result.Subject, "Q1 Assessment") {
		t.Error("subject should contain campaign name")
	}

	// HTML body.
	if !strings.Contains(result.HTMLBody, "Jane Smith") {
		t.Error("html_body should contain full name 'Jane Smith'")
	}
	if !strings.Contains(result.HTMLBody, "Engineer") {
		t.Error("html_body should contain position")
	}
	if !strings.Contains(result.HTMLBody, "IT") {
		t.Error("html_body should contain department")
	}
	if !strings.Contains(result.HTMLBody, "EMP-12345") {
		t.Error("html_body should contain custom field employee_id")
	}
	if !strings.Contains(result.HTMLBody, "Building A") {
		t.Error("html_body should contain custom field office")
	}
	if !strings.Contains(result.HTMLBody, "IT Support") {
		t.Error("html_body should contain from_name")
	}
	if !strings.Contains(result.HTMLBody, "support@company.com") {
		t.Error("html_body should contain sender email")
	}
	if !strings.Contains(result.HTMLBody, "https://landing.example.com/l/abc") {
		t.Error("html_body should contain phishing URL")
	}
	if !strings.Contains(result.HTMLBody, "March 26, 2026") {
		t.Error("html_body should contain current date")
	}
	if !strings.Contains(result.HTMLBody, "2026") {
		t.Error("html_body should contain current year")
	}
	if !strings.Contains(result.HTMLBody, "track.example.com/t/abc.gif") {
		t.Error("html_body should contain tracking pixel")
	}

	// Text body.
	if !strings.Contains(result.TextBody, "Jane") {
		t.Error("text_body should contain first name")
	}
	if !strings.Contains(result.TextBody, "Smith") {
		t.Error("text_body should contain last name")
	}
}

func TestRenderForSendRaw_RequirementsStyleTokens(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))

	vars := SendVars{
		Target: TargetVars{
			FirstName: "Bob",
			Email:     "bob@test.com",
		},
		Current: CurrentVars{
			Date: "March 26, 2026",
			Year: "2026",
		},
	}

	// Use requirements-style tokens.
	subject := "Hello {{target.first_name}}"
	html := "<p>Hello {{target.first_name}}, your email is {{target.email}}</p>"
	text := "Hello {{target.first_name}}"

	result, err := RenderForSendRaw(subject, html, text, vars, logger)
	if err != nil {
		t.Fatalf("RenderForSendRaw() error: %v", err)
	}

	if result.Subject != "Hello Bob" {
		t.Errorf("subject = %q, want 'Hello Bob'", result.Subject)
	}
	if !strings.Contains(result.HTMLBody, "bob@test.com") {
		t.Error("html should contain target email")
	}
}

func TestRenderForSendRaw_CustomTargetFields(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))

	vars := SendVars{
		Target: TargetVars{
			FirstName: "Alice",
			Custom: map[string]string{
				"employee_id": "EMP-999",
				"badge":       "B42",
			},
		},
	}

	subject := "ID: {{target.custom.employee_id}}"
	html := "<p>Badge: {{target.custom.badge}}</p>"
	text := ""

	result, err := RenderForSendRaw(subject, html, text, vars, logger)
	if err != nil {
		t.Fatalf("error: %v", err)
	}

	if result.Subject != "ID: EMP-999" {
		t.Errorf("subject = %q, want 'ID: EMP-999'", result.Subject)
	}
	if !strings.Contains(result.HTMLBody, "B42") {
		t.Error("html should contain badge value")
	}
}

func TestRenderForSendRaw_FullNameAutoPopulated(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))

	vars := SendVars{
		Target: TargetVars{
			FirstName: "Jane",
			LastName:  "Doe",
		},
	}

	subject := "{{target.full_name}}"
	result, err := RenderForSendRaw(subject, "", "", vars, logger)
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	if result.Subject != "Jane Doe" {
		t.Errorf("subject = %q, want 'Jane Doe'", result.Subject)
	}
}

func TestRenderForSendRaw_CurrentDateAutoPopulated(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))

	vars := SendVars{
		Target: TargetVars{FirstName: "X"},
	}

	// Don't set Current — should auto-populate.
	subject := "Date: {{current.date}}, Year: {{current.year}}"
	result, err := RenderForSendRaw(subject, "", "", vars, logger)
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	if result.Subject == "Date: , Year: " {
		t.Error("current date/year should be auto-populated")
	}
	if !strings.Contains(result.Subject, "2026") {
		t.Errorf("subject should contain current year, got: %q", result.Subject)
	}
}

func TestInjectTrackingPixel_BeforeBody(t *testing.T) {
	html := `<html><body><p>Hello</p></body></html>`
	pixel := `<img src="https://track.example.com/t/abc.gif" width="1" height="1" style="display:none" alt="" />`

	result := injectTrackingPixel(html, pixel)
	if !strings.Contains(result, pixel) {
		t.Error("result should contain tracking pixel")
	}
	// Pixel should appear before </body>.
	pixelIdx := strings.Index(result, pixel)
	bodyIdx := strings.Index(result, "</body>")
	if pixelIdx > bodyIdx {
		t.Error("tracking pixel should appear before </body>")
	}
}

func TestInjectTrackingPixel_NoBodyTag(t *testing.T) {
	html := `<p>Hello world</p>`
	pixel := `<img src="https://track.example.com/t/abc.gif" />`

	result := injectTrackingPixel(html, pixel)
	if !strings.HasSuffix(result, pixel) {
		t.Errorf("pixel should be appended when no </body> tag, got: %q", result)
	}
}

func TestRenderForSendRaw_PixelAutoInjected(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))

	pixel := `<img src="https://track.example.com/t/abc.gif" width="1" height="1" style="display:none" alt="" />`
	vars := SendVars{
		Target:   TargetVars{FirstName: "Test"},
		Tracking: TrackingVars{Pixel: pixel},
	}

	// Template WITHOUT tracking pixel token — should auto-inject.
	html := `<html><body><p>Hello</p></body></html>`
	result, err := RenderForSendRaw("Sub", html, "", vars, logger)
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	if !strings.Contains(result.HTMLBody, pixel) {
		t.Error("tracking pixel should be auto-injected when not in template")
	}
	pixelIdx := strings.Index(result.HTMLBody, pixel)
	bodyIdx := strings.Index(result.HTMLBody, "</body>")
	if pixelIdx > bodyIdx {
		t.Error("auto-injected pixel should appear before </body>")
	}
}

func TestRenderForSendRaw_PixelFromToken(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))

	pixel := `<img src="https://track.example.com/t/abc.gif" width="1" height="1" />`
	vars := SendVars{
		Target:   TargetVars{FirstName: "Test"},
		Tracking: TrackingVars{Pixel: pixel},
	}

	// Template WITH tracking pixel token — should render at token location.
	html := `<html><body><p>Hello</p><div>{{tracking.pixel}}</div></body></html>`
	result, err := RenderForSendRaw("Sub", html, "", vars, logger)
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	if !strings.Contains(result.HTMLBody, pixel) {
		t.Error("tracking pixel should appear at token location")
	}
	// Should only appear once (at token location, not also auto-injected).
	count := strings.Count(result.HTMLBody, "track.example.com/t/abc.gif")
	if count != 1 {
		t.Errorf("pixel should appear exactly once, found %d times", count)
	}
}

// --- ECOMP-03: Header Sanitization Tests ---

func TestSanitizeHeaderValue(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{"clean string", "Hello World", "Hello World"},
		{"strip CR LF", "Hello\r\nBcc: evil@hacker.com", "HelloBcc: evil@hacker.com"},
		{"strip LF only", "Subject\nInjection", "SubjectInjection"},
		{"strip CR only", "Subject\rInjection", "SubjectInjection"},
		{"empty string", "", ""},
		{"multiple CRLF", "a\r\nb\r\nc", "abc"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := sanitizeHeaderValue(tc.input)
			if got != tc.want {
				t.Errorf("sanitizeHeaderValue(%q) = %q, want %q", tc.input, got, tc.want)
			}
		})
	}
}

func TestComposeHeaders_SanitizesAndEncodes(t *testing.T) {
	// Subject with CRLF injection attempt.
	subject, fromName, replyTo := ComposeHeaders(
		"Hello\r\nBcc: evil@hacker.com",
		"IT\nSupport",
		"reply@example.com",
	)

	if strings.Contains(subject, "\n") || strings.Contains(subject, "\r") {
		t.Error("subject should have CRLF stripped")
	}
	if strings.Contains(fromName, "\n") {
		t.Error("fromName should have LF stripped")
	}
	if replyTo != "reply@example.com" {
		t.Errorf("replyTo should be unchanged, got %q", replyTo)
	}
}

// --- ECOMP-04: RFC 2047 Encoding Tests ---

func TestEncodeHeaderIfNeeded(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		encoded  bool // Whether encoding should be applied.
	}{
		{"ASCII only", "Hello World", false},
		{"non-ASCII umlaut", "Über wichtig", true},
		{"non-ASCII accents", "José García", true},
		{"empty string", "", false},
		{"ASCII with numbers", "Meeting 2026-Q1", false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := encodeHeaderIfNeeded(tc.input)
			if tc.encoded {
				if !strings.HasPrefix(got, "=?utf-8?q?") {
					t.Errorf("expected RFC 2047 encoding, got %q", got)
				}
			} else {
				if got != tc.input {
					t.Errorf("ASCII input should not be encoded, got %q", got)
				}
			}
		})
	}
}

func TestComposeHeaders_RFC2047(t *testing.T) {
	subject, fromName, _ := ComposeHeaders("Über wichtig", "José", "reply@example.com")

	if !strings.HasPrefix(subject, "=?utf-8?q?") {
		t.Errorf("non-ASCII subject should be RFC 2047 encoded, got %q", subject)
	}
	if !strings.HasPrefix(fromName, "=?utf-8?q?") {
		t.Errorf("non-ASCII from name should be RFC 2047 encoded, got %q", fromName)
	}
}

// --- ECOMP-05: Message-ID Domain Tests ---

func TestExtractFromDomain(t *testing.T) {
	tests := []struct {
		name    string
		address string
		want    string
	}{
		{"normal email", "user@example.com", "example.com"},
		{"subdomain", "user@mail.example.com", "mail.example.com"},
		{"no domain", "user", ""},
		{"empty", "", ""},
		{"at sign only", "@", ""},
		{"trailing at", "user@", ""},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := ExtractFromDomain(tc.address)
			if got != tc.want {
				t.Errorf("ExtractFromDomain(%q) = %q, want %q", tc.address, got, tc.want)
			}
		})
	}
}

func TestGenerateMessageID(t *testing.T) {
	tests := []struct {
		name    string
		uuid    string
		from    string
		wantPfx string
		wantSfx string
	}{
		{
			name:    "normal",
			uuid:    "abc-123",
			from:    "sender@example.com",
			wantPfx: "<abc-123@",
			wantSfx: "example.com>",
		},
		{
			name:    "no domain falls back to localhost",
			uuid:    "abc-123",
			from:    "sender",
			wantPfx: "<abc-123@",
			wantSfx: "localhost>",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := GenerateMessageID(tc.uuid, tc.from)
			if !strings.HasPrefix(got, tc.wantPfx) {
				t.Errorf("GenerateMessageID() = %q, want prefix %q", got, tc.wantPfx)
			}
			if !strings.HasSuffix(got, tc.wantSfx) {
				t.Errorf("GenerateMessageID() = %q, want suffix %q", got, tc.wantSfx)
			}
		})
	}
}

func TestRenderForSendRaw_EmptyVarsNoError(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))

	vars := SendVars{} // All empty.

	subject := "Hello {{target.first_name}}"
	result, err := RenderForSendRaw(subject, "", "", vars, logger)
	if err != nil {
		t.Fatalf("should not error on empty vars: %v", err)
	}
	if result.Subject != "Hello " {
		t.Errorf("subject = %q, want 'Hello '", result.Subject)
	}
}
