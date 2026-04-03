// Package instancetemplate implements business logic for instance template management.
package instancetemplate

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"

	"tackle/internal/providers"
	"tackle/internal/providers/credentials"
	"tackle/internal/repositories"
	"tackle/internal/services/audit"
)

// ValidationError is returned when input fails validation.
type ValidationError struct{ msg string }

// Error implements the error interface.
func (e *ValidationError) Error() string { return e.msg }

// ConflictError is returned when an operation would violate a constraint.
type ConflictError struct{ msg string }

// Error implements the error interface.
func (e *ConflictError) Error() string { return e.msg }

// CreateInput holds the fields needed to create a new instance template.
type CreateInput struct {
	DisplayName       string
	CloudCredentialID string
	Region            string
	InstanceSize      string
	OSImage           string
	SecurityGroups    []string
	SSHKeyReference   *string
	UserData          *string
	Tags              map[string]string
	Notes             *string
}

// UpdateInput holds the mutable fields for creating a new template version.
type UpdateInput = CreateInput

// TemplateDTO is the API-safe representation of a template with its current version.
type TemplateDTO struct {
	ID                string     `json:"id"`
	DisplayName       string     `json:"display_name"`
	CloudCredentialID string     `json:"cloud_credential_id"`
	ProviderType      string     `json:"provider_type"`
	CurrentVersion    int        `json:"current_version"`
	CreatedBy         string     `json:"created_by"`
	CreatedAt         string     `json:"created_at"`
	UpdatedAt         string     `json:"updated_at"`
	Version           VersionDTO `json:"version"`
}

// VersionDTO is the API-safe representation of a template version.
type VersionDTO struct {
	ID              string            `json:"id"`
	VersionNumber   int               `json:"version_number"`
	Region          string            `json:"region"`
	InstanceSize    string            `json:"instance_size"`
	OSImage         string            `json:"os_image"`
	SecurityGroups  []string          `json:"security_groups"`
	SSHKeyReference string            `json:"ssh_key_reference,omitempty"`
	UserData        string            `json:"user_data,omitempty"`
	Tags            map[string]string `json:"tags"`
	Notes           string            `json:"notes,omitempty"`
	CreatedBy       string            `json:"created_by"`
	CreatedAt       string            `json:"created_at"`
}

// ValidationResult holds the result of a template field validation.
type ValidationResult struct {
	Valid  bool              `json:"valid"`
	Errors map[string]string `json:"errors,omitempty"`
}

// Service implements instance template business logic.
type Service struct {
	templateRepo *repositories.InstanceTemplateRepository
	credRepo     *repositories.CloudCredentialRepository
	encSvc       *credentials.EncryptionService
	auditSvc     *audit.AuditService
}

// NewService creates a new instance template Service.
func NewService(
	templateRepo *repositories.InstanceTemplateRepository,
	credRepo *repositories.CloudCredentialRepository,
	encSvc *credentials.EncryptionService,
	auditSvc *audit.AuditService,
) *Service {
	return &Service{
		templateRepo: templateRepo,
		credRepo:     credRepo,
		encSvc:       encSvc,
		auditSvc:     auditSvc,
	}
}

// CreateTemplate validates and creates a new template with version 1.
func (s *Service) CreateTemplate(ctx context.Context, input CreateInput, actorID, actorName, ip, correlationID string) (TemplateDTO, error) {
	if err := validateInput(input); err != nil {
		return TemplateDTO{}, err
	}

	cred, err := s.credRepo.GetByID(ctx, input.CloudCredentialID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return TemplateDTO{}, &ValidationError{msg: "cloud_credential_id not found"}
		}
		return TemplateDTO{}, fmt.Errorf("template service: get credential: %w", err)
	}

	if input.Tags == nil {
		input.Tags = map[string]string{}
	}

	header := repositories.InstanceTemplate{
		DisplayName:       input.DisplayName,
		CloudCredentialID: input.CloudCredentialID,
		ProviderType:      cred.ProviderType,
		CreatedBy:         actorID,
	}
	version := repositories.InstanceTemplateVersion{
		Region:          input.Region,
		InstanceSize:    input.InstanceSize,
		OSImage:         input.OSImage,
		SecurityGroups:  input.SecurityGroups,
		SSHKeyReference: input.SSHKeyReference,
		UserData:        input.UserData,
		Tags:            input.Tags,
		Notes:           input.Notes,
		CreatedBy:       actorID,
	}

	result, err := s.templateRepo.Create(ctx, header, version)
	if err != nil {
		if isUniqueViolation(err) {
			return TemplateDTO{}, &ConflictError{msg: fmt.Sprintf("display_name %q is already in use", input.DisplayName)}
		}
		return TemplateDTO{}, fmt.Errorf("template service: create: %w", err)
	}

	resourceType := "instance_template"
	resourceID := result.ID
	_ = s.auditSvc.Log(ctx, audit.LogEntry{
		Category:      audit.CategoryInfrastructure,
		Severity:      audit.SeverityInfo,
		ActorType:     audit.ActorTypeUser,
		ActorID:       &actorID,
		ActorLabel:    actorName,
		Action:        "instance_template.created",
		ResourceType:  &resourceType,
		ResourceID:    &resourceID,
		CorrelationID: correlationID,
		SourceIP:      &ip,
		Details: map[string]any{
			"display_name":  result.DisplayName,
			"provider_type": string(result.ProviderType),
			"region":        input.Region,
			"version":       1,
		},
	})

	return toTemplateDTO(result), nil
}

// UpdateTemplate creates a new version for an existing template (does NOT modify old versions).
func (s *Service) UpdateTemplate(ctx context.Context, id string, input UpdateInput, actorID, actorName, ip, correlationID string) (TemplateDTO, error) {
	if err := validateInput(input); err != nil {
		return TemplateDTO{}, err
	}

	existing, err := s.templateRepo.GetByID(ctx, id)
	if err != nil {
		return TemplateDTO{}, err
	}

	// If credential is changing, verify it exists and matches provider type.
	credID := input.CloudCredentialID
	if credID == "" {
		credID = existing.CloudCredentialID
	} else if credID != existing.CloudCredentialID {
		cred, err := s.credRepo.GetByID(ctx, credID)
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return TemplateDTO{}, &ValidationError{msg: "cloud_credential_id not found"}
			}
			return TemplateDTO{}, fmt.Errorf("template service: get credential: %w", err)
		}
		if cred.ProviderType != existing.ProviderType {
			return TemplateDTO{}, &ValidationError{msg: fmt.Sprintf("credential provider_type %q does not match template provider_type %q", cred.ProviderType, existing.ProviderType)}
		}
	}

	if input.Tags == nil {
		input.Tags = map[string]string{}
	}

	oldVersion := existing.CurrentVersion

	version := repositories.InstanceTemplateVersion{
		Region:          input.Region,
		InstanceSize:    input.InstanceSize,
		OSImage:         input.OSImage,
		SecurityGroups:  input.SecurityGroups,
		SSHKeyReference: input.SSHKeyReference,
		UserData:        input.UserData,
		Tags:            input.Tags,
		Notes:           input.Notes,
		CreatedBy:       actorID,
	}

	result, err := s.templateRepo.AddVersion(ctx, id, version)
	if err != nil {
		return TemplateDTO{}, fmt.Errorf("template service: add version: %w", err)
	}

	resourceType := "instance_template"
	_ = s.auditSvc.Log(ctx, audit.LogEntry{
		Category:      audit.CategoryInfrastructure,
		Severity:      audit.SeverityInfo,
		ActorType:     audit.ActorTypeUser,
		ActorID:       &actorID,
		ActorLabel:    actorName,
		Action:        "instance_template.updated",
		ResourceType:  &resourceType,
		ResourceID:    &id,
		CorrelationID: correlationID,
		SourceIP:      &ip,
		Details: map[string]any{
			"display_name": result.DisplayName,
			"old_version":  oldVersion,
			"new_version":  result.CurrentVersion,
		},
	})

	return toTemplateDTO(result), nil
}

// GetTemplate returns the current version of a template.
func (s *Service) GetTemplate(ctx context.Context, id string) (TemplateDTO, error) {
	result, err := s.templateRepo.GetByID(ctx, id)
	if err != nil {
		return TemplateDTO{}, err
	}
	return toTemplateDTO(result), nil
}

// GetTemplateVersion returns a specific version of a template.
func (s *Service) GetTemplateVersion(ctx context.Context, id string, version int) (VersionDTO, error) {
	v, err := s.templateRepo.GetVersion(ctx, id, version)
	if err != nil {
		return VersionDTO{}, err
	}
	return toVersionDTO(v), nil
}

// ListTemplates returns all templates with their current versions.
func (s *Service) ListTemplates(ctx context.Context, providerType string) ([]TemplateDTO, error) {
	results, err := s.templateRepo.List(ctx, providerType)
	if err != nil {
		return nil, fmt.Errorf("template service: list: %w", err)
	}
	dtos := make([]TemplateDTO, 0, len(results))
	for _, r := range results {
		dtos = append(dtos, toTemplateDTO(r))
	}
	return dtos, nil
}

// ListTemplateVersions returns all versions of a template.
func (s *Service) ListTemplateVersions(ctx context.Context, id string) ([]VersionDTO, error) {
	versions, err := s.templateRepo.ListVersions(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("template service: list versions: %w", err)
	}
	dtos := make([]VersionDTO, 0, len(versions))
	for _, v := range versions {
		dtos = append(dtos, toVersionDTO(v))
	}
	return dtos, nil
}

// DeleteTemplate removes a template and all its versions.
func (s *Service) DeleteTemplate(ctx context.Context, id string, actorID, actorName, ip, correlationID string) error {
	existing, err := s.templateRepo.GetByID(ctx, id)
	if err != nil {
		return err
	}

	if err := s.templateRepo.Delete(ctx, id); err != nil {
		return fmt.Errorf("template service: delete: %w", err)
	}

	resourceType := "instance_template"
	_ = s.auditSvc.Log(ctx, audit.LogEntry{
		Category:      audit.CategoryInfrastructure,
		Severity:      audit.SeverityInfo,
		ActorType:     audit.ActorTypeUser,
		ActorID:       &actorID,
		ActorLabel:    actorName,
		Action:        "instance_template.deleted",
		ResourceType:  &resourceType,
		ResourceID:    &id,
		CorrelationID: correlationID,
		SourceIP:      &ip,
		Details: map[string]any{
			"display_name": existing.DisplayName,
		},
	})
	return nil
}

// ValidateTemplate runs cloud-side validations against the credential's provider.
func (s *Service) ValidateTemplate(ctx context.Context, input CreateInput) (ValidationResult, error) {
	errs := map[string]string{}

	if err := validateInput(input); err != nil {
		if ve, ok := err.(*ValidationError); ok {
			return ValidationResult{Valid: false, Errors: map[string]string{"input": ve.msg}}, nil
		}
	}

	cred, err := s.credRepo.GetByID(ctx, input.CloudCredentialID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return ValidationResult{Valid: false, Errors: map[string]string{"cloud_credential_id": "not found"}}, nil
		}
		return ValidationResult{}, fmt.Errorf("template service: get credential: %w", err)
	}

	client, err := providers.NewCloudClientFromCredential(ctx, cred, s.encSvc)
	if err != nil {
		return ValidationResult{Valid: false, Errors: map[string]string{"credentials": err.Error()}}, nil
	}

	if !client.ValidateRegion(input.Region) {
		errs["region"] = fmt.Sprintf("%q is not a valid region for provider %q", input.Region, cred.ProviderType)
	}
	if !client.ValidateInstanceSize(input.InstanceSize) {
		errs["instance_size"] = fmt.Sprintf("%q is not a recognized instance size for provider %q", input.InstanceSize, cred.ProviderType)
	}

	return ValidationResult{Valid: len(errs) == 0, Errors: errs}, nil
}

func validateInput(input CreateInput) error {
	if input.DisplayName == "" {
		return &ValidationError{msg: "display_name is required"}
	}
	if input.CloudCredentialID == "" {
		return &ValidationError{msg: "cloud_credential_id is required"}
	}
	if input.Region == "" {
		return &ValidationError{msg: "region is required"}
	}
	if input.InstanceSize == "" {
		return &ValidationError{msg: "instance_size is required"}
	}
	if input.OSImage == "" {
		return &ValidationError{msg: "os_image is required"}
	}
	return nil
}

func toTemplateDTO(r repositories.InstanceTemplateWithVersion) TemplateDTO {
	return TemplateDTO{
		ID:                r.ID,
		DisplayName:       r.DisplayName,
		CloudCredentialID: r.CloudCredentialID,
		ProviderType:      string(r.ProviderType),
		CurrentVersion:    r.CurrentVersion,
		CreatedBy:         r.CreatedBy,
		CreatedAt:         r.CreatedAt.Format("2006-01-02T15:04:05Z07:00"),
		UpdatedAt:         r.UpdatedAt.Format("2006-01-02T15:04:05Z07:00"),
		Version:           toVersionDTO(r.Version),
	}
}

func toVersionDTO(v repositories.InstanceTemplateVersion) VersionDTO {
	dto := VersionDTO{
		ID:             v.ID,
		VersionNumber:  v.VersionNumber,
		Region:         v.Region,
		InstanceSize:   v.InstanceSize,
		OSImage:        v.OSImage,
		SecurityGroups: v.SecurityGroups,
		Tags:           v.Tags,
		CreatedBy:      v.CreatedBy,
		CreatedAt:      v.CreatedAt.Format("2006-01-02T15:04:05Z07:00"),
	}
	if v.SSHKeyReference != nil {
		dto.SSHKeyReference = *v.SSHKeyReference
	}
	if v.UserData != nil {
		dto.UserData = *v.UserData
	}
	if v.Notes != nil {
		dto.Notes = *v.Notes
	}
	if dto.SecurityGroups == nil {
		dto.SecurityGroups = []string{}
	}
	if dto.Tags == nil {
		dto.Tags = map[string]string{}
	}
	return dto
}

func isUniqueViolation(err error) bool {
	if err == nil {
		return false
	}
	msg := err.Error()
	return strings.Contains(msg, "unique") || strings.Contains(msg, "duplicate") || strings.Contains(msg, "23505")
}
