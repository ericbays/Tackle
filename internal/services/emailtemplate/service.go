// Package emailtemplate implements business logic for email template management.
package emailtemplate

import (
	"bytes"
	"context"
	"database/sql"
	"errors"
	"fmt"
	"html/template"
	"strings"
	texttemplate "text/template"
	"time"

	"tackle/internal/repositories"
	auditsvc "tackle/internal/services/audit"
)

// ValidationError is returned when input fails validation.
type ValidationError struct{ msg string }

// Error implements the error interface.
func (e *ValidationError) Error() string { return e.msg }

// ConflictError is returned when a uniqueness constraint would be violated.
type ConflictError struct{ msg string }

// Error implements the error interface.
func (e *ConflictError) Error() string { return e.msg }

// TemplateVars holds all variables available during template rendering.
type TemplateVars struct {
	FirstName    string
	LastName     string
	Email        string
	TrackingURL  string
	TargetURL    string
	CampaignName string
	// Custom holds per-campaign extra variables.
	Custom map[string]string
}

// sampleVars is used when rendering previews.
var sampleVars = TemplateVars{
	FirstName:    "Jane",
	LastName:     "Smith",
	Email:        "jane.smith@example.com",
	TrackingURL:  "https://track.example.com/t/abc123",
	TargetURL:    "https://landing.example.com/l/abc123",
	CampaignName: "Q1 Security Assessment",
	Custom:       map[string]string{"Department": "Engineering"},
}

// CreateInput holds fields for creating an email template.
type CreateInput struct {
	Name        string   `json:"name"`
	Description *string  `json:"description,omitempty"`
	Subject     string   `json:"subject"`
	HTMLBody    string   `json:"html_body"`
	TextBody    string   `json:"text_body"`
	Category    string   `json:"category"`
	Tags        []string `json:"tags,omitempty"`
	IsShared    bool     `json:"is_shared"`
}

// UpdateInput holds mutable fields for an update.
type UpdateInput struct {
	Name        *string  `json:"name,omitempty"`
	Description *string  `json:"description,omitempty"`
	Subject     *string  `json:"subject,omitempty"`
	HTMLBody    *string  `json:"html_body,omitempty"`
	TextBody    *string  `json:"text_body,omitempty"`
	Category    *string  `json:"category,omitempty"`
	Tags        []string `json:"tags,omitempty"`
	IsShared    *bool    `json:"is_shared,omitempty"`
	ChangeNote  string   `json:"change_note,omitempty"`
}

// CloneInput holds fields for cloning a template.
type CloneInput struct {
	NewName string `json:"new_name"`
}

// PreviewInput holds the optional custom vars for preview rendering.
type PreviewInput struct {
	Custom map[string]string `json:"custom,omitempty"`
}

// EmailTemplateDTO is the API-safe representation of an email template.
type EmailTemplateDTO struct {
	ID          string   `json:"id"`
	Name        string   `json:"name"`
	Description string   `json:"description,omitempty"`
	Subject     string   `json:"subject"`
	HTMLBody    string   `json:"html_body"`
	TextBody    string   `json:"text_body"`
	Category    string   `json:"category"`
	Tags        []string `json:"tags"`
	IsShared    bool     `json:"is_shared"`
	CreatedBy   string   `json:"created_by"`
	CreatedAt   string   `json:"created_at"`
	UpdatedAt   string   `json:"updated_at"`
}

// EmailTemplateVersionDTO is the API-safe representation of a template version.
type EmailTemplateVersionDTO struct {
	ID            string `json:"id"`
	TemplateID    string `json:"template_id"`
	VersionNumber int    `json:"version_number"`
	Subject       string `json:"subject"`
	HTMLBody      string `json:"html_body"`
	TextBody      string `json:"text_body"`
	ChangeNote    string `json:"change_note,omitempty"`
	CreatedBy     string `json:"created_by"`
	CreatedAt     string `json:"created_at"`
}

// PreviewResult holds the rendered output for a template preview.
type PreviewResult struct {
	Subject  string `json:"subject"`
	HTMLBody string `json:"html_body"`
	TextBody string `json:"text_body"`
}

// ValidateResult reports template parse and variable errors.
type ValidateResult struct {
	Valid    bool     `json:"valid"`
	Errors   []string `json:"errors,omitempty"`
	Warnings []string `json:"warnings,omitempty"`
}

// Service implements email template business logic.
type Service struct {
	repo     *repositories.EmailTemplateRepository
	auditSvc *auditsvc.AuditService
}

// NewService creates a new email template Service.
func NewService(
	repo *repositories.EmailTemplateRepository,
	auditSvc *auditsvc.AuditService,
) *Service {
	return &Service{repo: repo, auditSvc: auditSvc}
}

// Create validates and persists a new email template.
func (s *Service) Create(ctx context.Context, input CreateInput, actorID, actorName, ip, correlationID string) (EmailTemplateDTO, error) {
	if err := validateCreate(input); err != nil {
		return EmailTemplateDTO{}, err
	}

	category := input.Category
	if category == "" {
		category = "general"
	}
	tags := input.Tags
	if tags == nil {
		tags = []string{}
	}

	t := repositories.EmailTemplate{
		Name:      input.Name,
		Subject:   input.Subject,
		HTMLBody:  input.HTMLBody,
		TextBody:  input.TextBody,
		Category:  category,
		Tags:      tags,
		IsShared:  input.IsShared,
		CreatedBy: actorID,
	}
	if input.Description != nil {
		t.Description = input.Description
	}

	created, err := s.repo.Create(ctx, t)
	if err != nil {
		if isUniqueViolation(err) {
			return EmailTemplateDTO{}, &ConflictError{msg: fmt.Sprintf("email template name %q is already in use", input.Name)}
		}
		return EmailTemplateDTO{}, fmt.Errorf("email template service: create: %w", err)
	}

	resourceType := "email_template"
	resourceID := created.ID
	_ = s.auditSvc.Log(ctx, auditsvc.LogEntry{
		Category:      auditsvc.CategoryUserActivity,
		Severity:      auditsvc.SeverityInfo,
		ActorType:     auditsvc.ActorTypeUser,
		ActorID:       &actorID,
		ActorLabel:    actorName,
		Action:        "email_template.created",
		ResourceType:  &resourceType,
		ResourceID:    &resourceID,
		CorrelationID: correlationID,
		SourceIP:      &ip,
		Details: map[string]any{
			"name":     input.Name,
			"category": category,
		},
	})

	return toDTO(created), nil
}

// Get returns the DTO for a single email template.
func (s *Service) Get(ctx context.Context, id string) (EmailTemplateDTO, error) {
	t, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return EmailTemplateDTO{}, err
	}
	return toDTO(t), nil
}

// List returns DTOs for all email templates with optional filtering.
func (s *Service) List(ctx context.Context, category, nameSearch, tag string, isShared *bool, createdBy string) ([]EmailTemplateDTO, error) {
	templates, err := s.repo.List(ctx, repositories.EmailTemplateFilters{
		Category:   category,
		NameSearch: nameSearch,
		Tag:        tag,
		IsShared:   isShared,
		CreatedBy:  createdBy,
	})
	if err != nil {
		return nil, fmt.Errorf("email template service: list: %w", err)
	}
	dtos := make([]EmailTemplateDTO, 0, len(templates))
	for _, t := range templates {
		dtos = append(dtos, toDTO(t))
	}
	return dtos, nil
}

// Update applies changes, snapshots a version, and audits.
func (s *Service) Update(ctx context.Context, id string, input UpdateInput, actorID, actorName, ip, correlationID string) (EmailTemplateDTO, error) {
	if _, err := s.repo.GetByID(ctx, id); err != nil {
		return EmailTemplateDTO{}, err
	}

	upd := repositories.EmailTemplateUpdate{
		Name:        input.Name,
		Description: input.Description,
		Subject:     input.Subject,
		HTMLBody:    input.HTMLBody,
		TextBody:    input.TextBody,
		Category:    input.Category,
		Tags:        input.Tags,
		IsShared:    input.IsShared,
	}

	updated, err := s.repo.Update(ctx, id, upd, actorID, input.ChangeNote)
	if err != nil {
		if isUniqueViolation(err) {
			return EmailTemplateDTO{}, &ConflictError{msg: "email template name is already in use"}
		}
		if errors.Is(err, sql.ErrNoRows) {
			return EmailTemplateDTO{}, err
		}
		return EmailTemplateDTO{}, fmt.Errorf("email template service: update: %w", err)
	}

	resourceType := "email_template"
	_ = s.auditSvc.Log(ctx, auditsvc.LogEntry{
		Category:      auditsvc.CategoryUserActivity,
		Severity:      auditsvc.SeverityInfo,
		ActorType:     auditsvc.ActorTypeUser,
		ActorID:       &actorID,
		ActorLabel:    actorName,
		Action:        "email_template.updated",
		ResourceType:  &resourceType,
		ResourceID:    &id,
		CorrelationID: correlationID,
		SourceIP:      &ip,
		Details: map[string]any{
			"name":        updated.Name,
			"change_note": input.ChangeNote,
		},
	})

	return toDTO(updated), nil
}

// Delete removes an email template.
func (s *Service) Delete(ctx context.Context, id string, actorID, actorName, ip, correlationID string) error {
	t, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return err
	}

	if err := s.repo.Delete(ctx, id); err != nil {
		return fmt.Errorf("email template service: delete: %w", err)
	}

	resourceType := "email_template"
	_ = s.auditSvc.Log(ctx, auditsvc.LogEntry{
		Category:      auditsvc.CategoryUserActivity,
		Severity:      auditsvc.SeverityInfo,
		ActorType:     auditsvc.ActorTypeUser,
		ActorID:       &actorID,
		ActorLabel:    actorName,
		Action:        "email_template.deleted",
		ResourceType:  &resourceType,
		ResourceID:    &id,
		CorrelationID: correlationID,
		SourceIP:      &ip,
		Details: map[string]any{
			"name": t.Name,
		},
	})
	return nil
}

// Clone copies a template with a new name.
func (s *Service) Clone(ctx context.Context, id string, input CloneInput, actorID, actorName, ip, correlationID string) (EmailTemplateDTO, error) {
	if input.NewName == "" {
		return EmailTemplateDTO{}, &ValidationError{msg: "new_name is required"}
	}

	cloned, err := s.repo.Clone(ctx, id, input.NewName, actorID)
	if err != nil {
		if isUniqueViolation(err) {
			return EmailTemplateDTO{}, &ConflictError{msg: fmt.Sprintf("email template name %q is already in use", input.NewName)}
		}
		if errors.Is(err, sql.ErrNoRows) {
			return EmailTemplateDTO{}, err
		}
		return EmailTemplateDTO{}, fmt.Errorf("email template service: clone: %w", err)
	}

	resourceType := "email_template"
	clonedID := cloned.ID
	_ = s.auditSvc.Log(ctx, auditsvc.LogEntry{
		Category:      auditsvc.CategoryUserActivity,
		Severity:      auditsvc.SeverityInfo,
		ActorType:     auditsvc.ActorTypeUser,
		ActorID:       &actorID,
		ActorLabel:    actorName,
		Action:        "email_template.cloned",
		ResourceType:  &resourceType,
		ResourceID:    &clonedID,
		CorrelationID: correlationID,
		SourceIP:      &ip,
		Details: map[string]any{
			"source_id": id,
			"new_name":  input.NewName,
		},
	})

	return toDTO(cloned), nil
}

// Preview renders a template with sample (or provided custom) data.
func (s *Service) Preview(ctx context.Context, id string, input PreviewInput) (PreviewResult, error) {
	t, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return PreviewResult{}, err
	}

	vars := sampleVars
	if input.Custom != nil {
		vars.Custom = input.Custom
	}

	subject, err := renderText(t.Subject, vars)
	if err != nil {
		return PreviewResult{}, &ValidationError{msg: fmt.Sprintf("subject template error: %v", err)}
	}

	htmlOut, err := renderHTML(t.HTMLBody, vars)
	if err != nil {
		return PreviewResult{}, &ValidationError{msg: fmt.Sprintf("html_body template error: %v", err)}
	}

	textOut, err := renderText(t.TextBody, vars)
	if err != nil {
		return PreviewResult{}, &ValidationError{msg: fmt.Sprintf("text_body template error: %v", err)}
	}

	return PreviewResult{
		Subject:  subject,
		HTMLBody: htmlOut,
		TextBody: textOut,
	}, nil
}

// Validate checks template syntax without rendering.
func (s *Service) Validate(ctx context.Context, id string) (ValidateResult, error) {
	t, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return ValidateResult{}, err
	}

	var errs []string
	var warns []string

	if _, err := texttemplate.New("subject").Parse(t.Subject); err != nil {
		errs = append(errs, fmt.Sprintf("subject: %v", err))
	}
	if _, err := template.New("html").Parse(t.HTMLBody); err != nil {
		errs = append(errs, fmt.Sprintf("html_body: %v", err))
	}
	if _, err := texttemplate.New("text").Parse(t.TextBody); err != nil {
		errs = append(errs, fmt.Sprintf("text_body: %v", err))
	}

	hasTracking := strings.Contains(t.HTMLBody, "{{.TrackingURL}}") ||
		strings.Contains(t.TextBody, "{{.TrackingURL}}") ||
		strings.Contains(t.HTMLBody, "{{.Tracking.URL}}") ||
		strings.Contains(t.TextBody, "{{.Tracking.URL}}") ||
		strings.Contains(t.HTMLBody, "{{phishing.url}}") ||
		strings.Contains(t.TextBody, "{{phishing.url}}")
	if !hasTracking {
		warns = append(warns, "template does not include a tracking URL token — click tracking will not work")
	}

	return ValidateResult{
		Valid:    len(errs) == 0,
		Errors:   errs,
		Warnings: warns,
	}, nil
}

// ListVersions returns version history for a template.
func (s *Service) ListVersions(ctx context.Context, templateID string) ([]EmailTemplateVersionDTO, error) {
	versions, err := s.repo.ListVersions(ctx, templateID)
	if err != nil {
		return nil, fmt.Errorf("email template service: list versions: %w", err)
	}
	dtos := make([]EmailTemplateVersionDTO, 0, len(versions))
	for _, v := range versions {
		dtos = append(dtos, toVersionDTO(v))
	}
	return dtos, nil
}

// GetVersion returns a specific version snapshot.
func (s *Service) GetVersion(ctx context.Context, templateID string, versionNumber int) (EmailTemplateVersionDTO, error) {
	v, err := s.repo.GetVersion(ctx, templateID, versionNumber)
	if err != nil {
		return EmailTemplateVersionDTO{}, err
	}
	return toVersionDTO(v), nil
}

// Export returns the template body fields suitable for file download.
func (s *Service) Export(ctx context.Context, id string) (EmailTemplateDTO, error) {
	return s.Get(ctx, id)
}

// --- rendering helpers ---

func renderText(tmplStr string, vars TemplateVars) (string, error) {
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

func renderHTML(tmplStr string, vars TemplateVars) (string, error) {
	tmpl, err := template.New("h").Option("missingkey=zero").Parse(tmplStr)
	if err != nil {
		return "", err
	}
	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, vars); err != nil {
		return "", err
	}
	return buf.String(), nil
}

// --- DTO converters ---

func toDTO(t repositories.EmailTemplate) EmailTemplateDTO {
	dto := EmailTemplateDTO{
		ID:        t.ID,
		Name:      t.Name,
		Subject:   t.Subject,
		HTMLBody:  t.HTMLBody,
		TextBody:  t.TextBody,
		Category:  t.Category,
		Tags:      t.Tags,
		IsShared:  t.IsShared,
		CreatedBy: t.CreatedBy,
		CreatedAt: t.CreatedAt.Format(time.RFC3339),
		UpdatedAt: t.UpdatedAt.Format(time.RFC3339),
	}
	if dto.Tags == nil {
		dto.Tags = []string{}
	}
	if t.Description != nil {
		dto.Description = *t.Description
	}
	return dto
}

func toVersionDTO(v repositories.EmailTemplateVersion) EmailTemplateVersionDTO {
	dto := EmailTemplateVersionDTO{
		ID:            v.ID,
		TemplateID:    v.TemplateID,
		VersionNumber: v.VersionNumber,
		Subject:       v.Subject,
		HTMLBody:      v.HTMLBody,
		TextBody:      v.TextBody,
		CreatedBy:     v.CreatedBy,
		CreatedAt:     v.CreatedAt.Format(time.RFC3339),
	}
	if v.ChangeNote != nil {
		dto.ChangeNote = *v.ChangeNote
	}
	return dto
}

// --- validation helpers ---

func validateCreate(input CreateInput) error {
	if strings.TrimSpace(input.Name) == "" {
		return &ValidationError{msg: "name is required"}
	}
	if strings.TrimSpace(input.Subject) == "" {
		return &ValidationError{msg: "subject is required"}
	}
	return nil
}

func isUniqueViolation(err error) bool {
	if err == nil {
		return false
	}
	msg := err.Error()
	return strings.Contains(msg, "unique") || strings.Contains(msg, "duplicate") || strings.Contains(msg, "23505")
}
