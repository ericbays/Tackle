// Package cloudcredential implements business logic for cloud credential management.
package cloudcredential

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

// ConflictError is returned when an operation would violate a uniqueness or dependency constraint.
type ConflictError struct{ msg string }

// Error implements the error interface.
func (e *ConflictError) Error() string { return e.msg }

// CreateInput holds the fields needed to create a cloud credential set.
type CreateInput struct {
	ProviderType  string
	DisplayName   string
	DefaultRegion string
	// One of the following must be set based on ProviderType.
	AWSCreds     *credentials.AWSCredentials
	AzureCreds   *credentials.AzureCredentials
	ProxmoxCreds *credentials.ProxmoxCredentials
}

// UpdateInput holds the mutable fields for updating a credential set.
type UpdateInput struct {
	DisplayName   *string
	DefaultRegion *string
	// Optional: re-supply credentials to rotate them.
	AWSCreds     *credentials.AWSCredentials
	AzureCreds   *credentials.AzureCredentials
	ProxmoxCreds *credentials.ProxmoxCredentials
}

// CloudCredentialDTO is the API-safe representation (credentials are masked).
type CloudCredentialDTO struct {
	ID            string `json:"id"`
	ProviderType  string `json:"provider_type"`
	DisplayName   string `json:"display_name"`
	DefaultRegion string `json:"default_region"`
	Status        string `json:"status"`
	StatusMessage string `json:"status_message,omitempty"`
	LastTestedAt  string `json:"last_tested_at,omitempty"`
	CreatedBy     string `json:"created_by"`
	CreatedAt     string `json:"created_at"`
	UpdatedAt     string `json:"updated_at"`
}

// TestResult is the result of a credential test.
type TestResult struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
}

// Service implements cloud credential business logic.
type Service struct {
	repo     *repositories.CloudCredentialRepository
	encSvc   *credentials.EncryptionService
	auditSvc *audit.AuditService
}

// NewService creates a new cloud credential Service.
func NewService(
	repo *repositories.CloudCredentialRepository,
	encSvc *credentials.EncryptionService,
	auditSvc *audit.AuditService,
) *Service {
	return &Service{repo: repo, encSvc: encSvc, auditSvc: auditSvc}
}

// CreateCredential validates, encrypts, and persists a new cloud credential set.
func (s *Service) CreateCredential(ctx context.Context, input CreateInput, actorID, actorName, ip, correlationID string) (CloudCredentialDTO, error) {
	if input.DisplayName == "" {
		return CloudCredentialDTO{}, &ValidationError{msg: "display_name is required"}
	}
	if input.DefaultRegion == "" {
		return CloudCredentialDTO{}, &ValidationError{msg: "default_region is required"}
	}

	var encrypted []byte
	var err error

	switch input.ProviderType {
	case "aws":
		if input.AWSCreds == nil {
			return CloudCredentialDTO{}, &ValidationError{msg: "aws credentials are required for provider_type=aws"}
		}
		if input.AWSCreds.AccessKeyID == "" || input.AWSCreds.SecretAccessKey == "" {
			return CloudCredentialDTO{}, &ValidationError{msg: "access_key_id and secret_access_key are required"}
		}
		encrypted, err = s.encSvc.Encrypt(*input.AWSCreds)
	case "azure":
		if input.AzureCreds == nil {
			return CloudCredentialDTO{}, &ValidationError{msg: "azure credentials are required for provider_type=azure"}
		}
		if input.AzureCreds.TenantID == "" || input.AzureCreds.ClientID == "" || input.AzureCreds.ClientSecret == "" || input.AzureCreds.SubscriptionID == "" {
			return CloudCredentialDTO{}, &ValidationError{msg: "tenant_id, client_id, client_secret, and subscription_id are required"}
		}
		encrypted, err = s.encSvc.Encrypt(*input.AzureCreds)
	case "proxmox":
		if input.ProxmoxCreds == nil {
			return CloudCredentialDTO{}, &ValidationError{msg: "proxmox credentials are required for provider_type=proxmox"}
		}
		if input.ProxmoxCreds.Host == "" || input.ProxmoxCreds.TokenID == "" || input.ProxmoxCreds.TokenSecret == "" || input.ProxmoxCreds.Node == "" {
			return CloudCredentialDTO{}, &ValidationError{msg: "host, token_id, token_secret, and node are required"}
		}
		encrypted, err = s.encSvc.Encrypt(*input.ProxmoxCreds)
	default:
		return CloudCredentialDTO{}, &ValidationError{msg: fmt.Sprintf("unsupported provider_type %q; must be aws, azure, or proxmox", input.ProviderType)}
	}
	if err != nil {
		return CloudCredentialDTO{}, fmt.Errorf("credential service: encrypt: %w", err)
	}

	cred, err := s.repo.Create(ctx, repositories.CloudCredential{
		ProviderType:         repositories.CloudProviderType(input.ProviderType),
		DisplayName:          input.DisplayName,
		CredentialsEncrypted: encrypted,
		DefaultRegion:        input.DefaultRegion,
		CreatedBy:            actorID,
	})
	if err != nil {
		if isUniqueViolation(err) {
			return CloudCredentialDTO{}, &ConflictError{msg: fmt.Sprintf("display_name %q is already in use", input.DisplayName)}
		}
		return CloudCredentialDTO{}, fmt.Errorf("credential service: create: %w", err)
	}

	resourceType := "cloud_credential"
	resourceID := cred.ID
	_ = s.auditSvc.Log(ctx, audit.LogEntry{
		Category:      audit.CategoryInfrastructure,
		Severity:      audit.SeverityInfo,
		ActorType:     audit.ActorTypeUser,
		ActorID:       &actorID,
		ActorLabel:    actorName,
		Action:        "cloud_credential.created",
		ResourceType:  &resourceType,
		ResourceID:    &resourceID,
		CorrelationID: correlationID,
		SourceIP:      &ip,
		Details: map[string]any{
			"provider_type": input.ProviderType,
			"display_name":  input.DisplayName,
		},
	})

	return toDTO(cred), nil
}

// GetCredential returns the masked DTO for a single credential set.
func (s *Service) GetCredential(ctx context.Context, id string) (CloudCredentialDTO, error) {
	cred, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return CloudCredentialDTO{}, err
	}
	return toDTO(cred), nil
}

// ListCredentials returns masked DTOs for all credential sets, with optional filtering.
func (s *Service) ListCredentials(ctx context.Context, providerType, status string) ([]CloudCredentialDTO, error) {
	creds, err := s.repo.List(ctx, repositories.CloudCredentialFilters{
		ProviderType: providerType,
		Status:       status,
	})
	if err != nil {
		return nil, fmt.Errorf("credential service: list: %w", err)
	}
	dtos := make([]CloudCredentialDTO, 0, len(creds))
	for _, c := range creds {
		dtos = append(dtos, toDTO(c))
	}
	return dtos, nil
}

// UpdateCredential updates mutable fields. If new credentials are supplied they are re-encrypted.
func (s *Service) UpdateCredential(ctx context.Context, id string, input UpdateInput, actorID, actorName, ip, correlationID string) (CloudCredentialDTO, error) {
	existing, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return CloudCredentialDTO{}, err
	}

	upd := repositories.CloudCredentialUpdate{
		DisplayName:   input.DisplayName,
		DefaultRegion: input.DefaultRegion,
	}

	if input.AWSCreds != nil {
		if existing.ProviderType != repositories.CloudProviderAWS {
			return CloudCredentialDTO{}, &ValidationError{msg: "cannot supply aws credentials for a non-aws credential set"}
		}
		enc, err := s.encSvc.Encrypt(*input.AWSCreds)
		if err != nil {
			return CloudCredentialDTO{}, fmt.Errorf("credential service: encrypt aws: %w", err)
		}
		upd.CredentialsEncrypted = enc
	}
	if input.AzureCreds != nil {
		if existing.ProviderType != repositories.CloudProviderAzure {
			return CloudCredentialDTO{}, &ValidationError{msg: "cannot supply azure credentials for a non-azure credential set"}
		}
		enc, err := s.encSvc.Encrypt(*input.AzureCreds)
		if err != nil {
			return CloudCredentialDTO{}, fmt.Errorf("credential service: encrypt azure: %w", err)
		}
		upd.CredentialsEncrypted = enc
	}
	if input.ProxmoxCreds != nil {
		if existing.ProviderType != repositories.CloudProviderProxmox {
			return CloudCredentialDTO{}, &ValidationError{msg: "cannot supply proxmox credentials for a non-proxmox credential set"}
		}
		enc, err := s.encSvc.Encrypt(*input.ProxmoxCreds)
		if err != nil {
			return CloudCredentialDTO{}, fmt.Errorf("credential service: encrypt proxmox: %w", err)
		}
		upd.CredentialsEncrypted = enc
	}

	updated, err := s.repo.Update(ctx, id, upd)
	if err != nil {
		if isUniqueViolation(err) {
			return CloudCredentialDTO{}, &ConflictError{msg: "display_name is already in use"}
		}
		if errors.Is(err, sql.ErrNoRows) {
			return CloudCredentialDTO{}, err
		}
		return CloudCredentialDTO{}, fmt.Errorf("credential service: update: %w", err)
	}

	resourceType := "cloud_credential"
	_ = s.auditSvc.Log(ctx, audit.LogEntry{
		Category:      audit.CategoryInfrastructure,
		Severity:      audit.SeverityInfo,
		ActorType:     audit.ActorTypeUser,
		ActorID:       &actorID,
		ActorLabel:    actorName,
		Action:        "cloud_credential.updated",
		ResourceType:  &resourceType,
		ResourceID:    &id,
		CorrelationID: correlationID,
		SourceIP:      &ip,
		Details: map[string]any{
			"provider_type": string(existing.ProviderType),
			"display_name":  updated.DisplayName,
		},
	})

	return toDTO(updated), nil
}

// DeleteCredential removes a credential set. Fails if any instance templates reference it.
func (s *Service) DeleteCredential(ctx context.Context, id string, actorID, actorName, ip, correlationID string) error {
	cred, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return err
	}

	hasRefs, err := s.repo.HasTemplateReferences(ctx, id)
	if err != nil {
		return fmt.Errorf("credential service: check references: %w", err)
	}
	if hasRefs {
		return &ConflictError{msg: "credential set is referenced by one or more instance templates and cannot be deleted"}
	}

	if err := s.repo.Delete(ctx, id); err != nil {
		return fmt.Errorf("credential service: delete: %w", err)
	}

	resourceType := "cloud_credential"
	_ = s.auditSvc.Log(ctx, audit.LogEntry{
		Category:      audit.CategoryInfrastructure,
		Severity:      audit.SeverityInfo,
		ActorType:     audit.ActorTypeUser,
		ActorID:       &actorID,
		ActorLabel:    actorName,
		Action:        "cloud_credential.deleted",
		ResourceType:  &resourceType,
		ResourceID:    &id,
		CorrelationID: correlationID,
		SourceIP:      &ip,
		Details: map[string]any{
			"provider_type": string(cred.ProviderType),
			"display_name":  cred.DisplayName,
		},
	})
	return nil
}

// TestCredential decrypts credentials, tests the connection, updates the status, and returns the result.
func (s *Service) TestCredential(ctx context.Context, id string, actorID, actorName, ip, correlationID string) (TestResult, error) {
	cred, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return TestResult{}, err
	}

	client, err := providers.NewCloudClientFromCredential(ctx, cred, s.encSvc)
	if err != nil {
		return TestResult{}, fmt.Errorf("credential service: build client: %w", err)
	}

	testErr := client.TestConnection(ctx)

	var status repositories.CloudCredentialStatus
	var statusMsg *string
	var auditAction string
	var result TestResult

	if testErr == nil {
		status = repositories.CloudCredentialStatusHealthy
		result = TestResult{Success: true, Message: "connection successful"}
		auditAction = "cloud_credential.test_success"
	} else {
		status = repositories.CloudCredentialStatusError
		msg := testErr.Error()
		statusMsg = &msg
		result = TestResult{Success: false, Message: msg}
		auditAction = "cloud_credential.test_failure"
	}

	if updErr := s.repo.UpdateStatus(ctx, id, status, statusMsg); updErr != nil {
		return result, fmt.Errorf("credential service: update status: %w", updErr)
	}

	resourceType := "cloud_credential"
	_ = s.auditSvc.Log(ctx, audit.LogEntry{
		Category:      audit.CategoryInfrastructure,
		Severity:      audit.SeverityInfo,
		ActorType:     audit.ActorTypeUser,
		ActorID:       &actorID,
		ActorLabel:    actorName,
		Action:        auditAction,
		ResourceType:  &resourceType,
		ResourceID:    &id,
		CorrelationID: correlationID,
		SourceIP:      &ip,
		Details: map[string]any{
			"provider_type": string(cred.ProviderType),
			"success":       result.Success,
		},
	})

	return result, nil
}

func toDTO(c repositories.CloudCredential) CloudCredentialDTO {
	dto := CloudCredentialDTO{
		ID:            c.ID,
		ProviderType:  string(c.ProviderType),
		DisplayName:   c.DisplayName,
		DefaultRegion: c.DefaultRegion,
		Status:        string(c.Status),
		CreatedBy:     c.CreatedBy,
		CreatedAt:     c.CreatedAt.Format("2006-01-02T15:04:05Z07:00"),
		UpdatedAt:     c.UpdatedAt.Format("2006-01-02T15:04:05Z07:00"),
	}
	if c.StatusMessage != nil {
		dto.StatusMessage = *c.StatusMessage
	}
	if c.LastTestedAt != nil {
		dto.LastTestedAt = c.LastTestedAt.Format("2006-01-02T15:04:05Z07:00")
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
