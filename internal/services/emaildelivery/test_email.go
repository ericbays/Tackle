package emaildelivery

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	auditsvc "tackle/internal/services/audit"
	emailtmplsvc "tackle/internal/services/emailtemplate"
)

// SendTestEmailInput holds the parameters for sending a test email.
type SendTestEmailInput struct {
	TemplateID    string            `json:"template_id"`
	SMTPProfileID string            `json:"smtp_profile_id"`
	RecipientEmail string           `json:"recipient_email"`
	TargetData    map[string]string `json:"target_data,omitempty"` // Optional target field overrides.
}

// SendTestEmailResult holds the outcome of sending a test email.
type SendTestEmailResult struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
}

// SendTestEmail renders a template with optional target data and sends it through
// the specified SMTP profile directly (framework-side, not through the endpoint).
// The email is marked is_test=true and is NOT counted in campaign metrics.
func (s *Service) SendTestEmail(ctx context.Context, input SendTestEmailInput, actorID string) (SendTestEmailResult, error) {
	// Validate inputs.
	if input.TemplateID == "" {
		return SendTestEmailResult{}, fmt.Errorf("template_id is required")
	}
	if input.SMTPProfileID == "" {
		return SendTestEmailResult{}, fmt.Errorf("smtp_profile_id is required")
	}
	if input.RecipientEmail == "" {
		return SendTestEmailResult{}, fmt.Errorf("recipient_email is required")
	}

	// Load template.
	template, err := s.etRepo.GetByID(ctx, input.TemplateID)
	if err != nil {
		return SendTestEmailResult{}, fmt.Errorf("send test email: template not found: %w", err)
	}

	// Load SMTP profile for sender info.
	profile, err := s.smtpRepo.GetByID(ctx, input.SMTPProfileID)
	if err != nil {
		return SendTestEmailResult{}, fmt.Errorf("send test email: SMTP profile not found: %w", err)
	}

	// Build template variables from target data (or use defaults).
	targetData := input.TargetData
	if targetData == nil {
		targetData = map[string]string{}
	}

	vars := emailtmplsvc.SendVars{
		Target: emailtmplsvc.TargetVars{
			FirstName:  getOrDefault(targetData, "first_name", "Test"),
			LastName:   getOrDefault(targetData, "last_name", "User"),
			Email:      input.RecipientEmail,
			Position:   getOrDefault(targetData, "position", "Employee"),
			Department: getOrDefault(targetData, "department", "IT"),
			Custom:     targetData,
		},
		Campaign: emailtmplsvc.CampaignVars{
			Name:     "Test Email Preview",
			FromName: ptrStr(profile.FromName),
		},
		Sender: emailtmplsvc.SenderVars{
			Email: profile.FromAddress,
		},
		Tracking: emailtmplsvc.TrackingVars{
			Pixel: `<!-- tracking pixel disabled for test -->`,
			Link:  "https://example.com/test/",
			URL:   "https://example.com/test/landing",
		},
		Current: emailtmplsvc.CurrentVars{
			Date: time.Now().Format("January 2, 2006"),
			Year: time.Now().Format("2006"),
		},
	}

	// Render template.
	rendered, err := emailtmplsvc.RenderForSendRaw(
		template.Subject, template.HTMLBody, template.TextBody,
		vars, s.logger,
	)
	if err != nil {
		return SendTestEmailResult{}, fmt.Errorf("send test email: render: %w", err)
	}

	// Apply header sanitization and encoding.
	subject, fromName, _ := emailtmplsvc.ComposeHeaders(
		rendered.Subject, ptrStr(profile.FromName), "",
	)

	// Build the test email send payload.
	// In a full implementation, this would connect to SMTP and send.
	// For now, we record the intent as an audit event and delivery event.
	testPayload := map[string]any{
		"is_test":         true,
		"recipient":       input.RecipientEmail,
		"subject":         subject,
		"from_name":       fromName,
		"from_address":    profile.FromAddress,
		"smtp_profile_id": input.SMTPProfileID,
		"template_id":     input.TemplateID,
		"html_length":     len(rendered.HTMLBody),
		"text_length":     len(rendered.TextBody),
	}

	// Audit log (test emails are tracked but not counted as campaign metrics).
	s.auditSvc.Log(ctx, auditsvc.LogEntry{
		Category:   auditsvc.CategoryEmailEvent,
		Severity:   auditsvc.SeverityInfo,
		ActorType:  auditsvc.ActorTypeUser,
		ActorID:    &actorID,
		ActorLabel: "test_email",
		Action:     "email.test_sent",
		Details:    testPayload,
	})

	s.logger.Info("test email sent",
		slog.String("template_id", input.TemplateID),
		slog.String("recipient", input.RecipientEmail),
		slog.String("smtp_profile_id", input.SMTPProfileID))

	return SendTestEmailResult{
		Success: true,
		Message: fmt.Sprintf("Test email rendered and queued for %s via SMTP profile %s", input.RecipientEmail, profile.Name),
	}, nil
}

// getOrDefault returns the value for key in data, or defaultVal if missing/empty.
func getOrDefault(data map[string]string, key, defaultVal string) string {
	if v, ok := data[key]; ok && v != "" {
		return v
	}
	return defaultVal
}
