// Package domainprovider implements business logic for domain provider connection management.
package domainprovider

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net"
	"regexp"
	"strings"

	"tackle/internal/providers"
	"tackle/internal/providers/credentials"
	"tackle/internal/repositories"
	auditsvc "tackle/internal/services/audit"
)

// awsRegionRE validates AWS region format (e.g. us-east-1).
var awsRegionRE = regexp.MustCompile(`^[a-z]{2}(-gov)?-[a-z]+-\d+$`)

// uuidRE is a loose UUID format check.
var uuidRE = regexp.MustCompile(`^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$`)

// Service provides business logic for domain provider connections.
type Service struct {
	repo     *repositories.DomainProviderRepository
	credEnc  *credentials.EncryptionService
	audit    *auditsvc.AuditService
}

// NewService creates a Service.
func NewService(
	repo *repositories.DomainProviderRepository,
	credEnc *credentials.EncryptionService,
	audit *auditsvc.AuditService,
) *Service {
	return &Service{repo: repo, credEnc: credEnc, audit: audit}
}

// ProviderConnectionDTO is the API-safe representation of a connection (credentials masked).
type ProviderConnectionDTO struct {
	ID            string  `json:"id"`
	ProviderType  string  `json:"provider_type"`
	DisplayName   string  `json:"display_name"`
	Status        string  `json:"status"`
	StatusMessage *string `json:"status_message,omitempty"`
	LastTestedAt  *string `json:"last_tested_at,omitempty"`
	CreatedBy     string  `json:"created_by"`
	CreatedAt     string  `json:"created_at"`
	UpdatedAt     string  `json:"updated_at"`
}

// TestResult holds the outcome of a test-connection call.
type TestResult struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
}

// CreateInput is the request payload for creating a provider connection.
type CreateInput struct {
	ProviderType  string `json:"provider_type"`
	DisplayName   string `json:"display_name"`
	RatePerMinute int    `json:"rate_per_minute,omitempty"`

	// Exactly one of the following should be populated, matching provider_type.
	NamecheapCreds *credentials.NamecheapCredentials `json:"namecheap_credentials,omitempty"`
	GoDaddyCreds   *credentials.GoDaddyCredentials   `json:"godaddy_credentials,omitempty"`
	Route53Creds   *credentials.Route53Credentials   `json:"route53_credentials,omitempty"`
	AzureDNSCreds  *credentials.AzureDNSCredentials  `json:"azure_dns_credentials,omitempty"`
}

// UpdateInput is the request payload for updating a provider connection.
type UpdateInput struct {
	DisplayName   *string `json:"display_name,omitempty"`
	RatePerMinute *int    `json:"rate_per_minute,omitempty"`

	// Credentials are optional — omit to leave them unchanged.
	NamecheapCreds *credentials.NamecheapCredentials `json:"namecheap_credentials,omitempty"`
	GoDaddyCreds   *credentials.GoDaddyCredentials   `json:"godaddy_credentials,omitempty"`
	Route53Creds   *credentials.Route53Credentials   `json:"route53_credentials,omitempty"`
	AzureDNSCreds  *credentials.AzureDNSCredentials  `json:"azure_dns_credentials,omitempty"`
}

// ValidationError is returned when input validation fails.
type ValidationError struct {
	Field   string
	Message string
}

func (e *ValidationError) Error() string {
	return fmt.Sprintf("validation error: %s: %s", e.Field, e.Message)
}

// CreateConnection validates the input, encrypts credentials, persists, and emits an audit event.
func (s *Service) CreateConnection(ctx context.Context, input CreateInput, actorID, actorLabel, sourceIP, correlationID string) (ProviderConnectionDTO, error) {
	if err := s.validateCreate(ctx, input); err != nil {
		return ProviderConnectionDTO{}, err
	}

	encrypted, err := s.encryptForType(input.ProviderType, input)
	if err != nil {
		return ProviderConnectionDTO{}, fmt.Errorf("domain provider: encrypt credentials: %w", err)
	}

	row, err := s.repo.Create(ctx, repositories.DomainProviderConnection{
		ProviderType:         input.ProviderType,
		DisplayName:          strings.TrimSpace(input.DisplayName),
		CredentialsEncrypted: encrypted,
		CreatedBy:            actorID,
	})
	if err != nil {
		return ProviderConnectionDTO{}, fmt.Errorf("domain provider: create: %w", err)
	}

	_ = s.audit.Log(ctx, auditsvc.LogEntry{
		Category:      auditsvc.CategoryInfrastructure,
		Severity:      auditsvc.SeverityInfo,
		ActorType:     auditsvc.ActorTypeUser,
		ActorID:       &actorID,
		ActorLabel:    actorLabel,
		Action:        "domain_provider.created",
		ResourceType:  strPtr("domain_provider_connection"),
		ResourceID:    &row.ID,
		CorrelationID: correlationID,
		SourceIP:      &sourceIP,
		Details: map[string]any{
			"provider_type": row.ProviderType,
			"display_name":  row.DisplayName,
		},
	})

	return toDTO(row), nil
}

// GetConnection returns a DTO (credentials masked) for a given connection ID.
func (s *Service) GetConnection(ctx context.Context, id string) (ProviderConnectionDTO, error) {
	row, err := s.repo.GetByID(ctx, id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return ProviderConnectionDTO{}, fmt.Errorf("domain provider: not found")
		}
		return ProviderConnectionDTO{}, fmt.Errorf("domain provider: get: %w", err)
	}
	return toDTO(row), nil
}

// ListConnections returns DTOs (credentials masked) with optional filtering.
func (s *Service) ListConnections(ctx context.Context, providerType, status string) ([]ProviderConnectionDTO, error) {
	rows, err := s.repo.List(ctx, repositories.DomainProviderFilters{
		ProviderType: providerType,
		Status:       status,
	})
	if err != nil {
		return nil, fmt.Errorf("domain provider: list: %w", err)
	}
	dtos := make([]ProviderConnectionDTO, len(rows))
	for i, r := range rows {
		dtos[i] = toDTO(r)
	}
	return dtos, nil
}

// UpdateConnection validates, re-encrypts if credentials changed, persists, and emits an audit event.
func (s *Service) UpdateConnection(ctx context.Context, id string, input UpdateInput, actorID, actorLabel, sourceIP, correlationID string) (ProviderConnectionDTO, error) {
	existing, err := s.repo.GetByID(ctx, id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return ProviderConnectionDTO{}, fmt.Errorf("domain provider: not found")
		}
		return ProviderConnectionDTO{}, fmt.Errorf("domain provider: get for update: %w", err)
	}

	updates := repositories.DomainProviderUpdate{}

	if input.DisplayName != nil {
		trimmed := strings.TrimSpace(*input.DisplayName)
		if err := s.validateDisplayName(ctx, trimmed, id); err != nil {
			return ProviderConnectionDTO{}, err
		}
		updates.DisplayName = &trimmed
	}

	// Re-encrypt credentials if any were provided.
	if newCreds := extractCredsForType(existing.ProviderType, input); newCreds != nil {
		encrypted, err := s.credEnc.Encrypt(newCreds)
		if err != nil {
			return ProviderConnectionDTO{}, fmt.Errorf("domain provider: encrypt updated credentials: %w", err)
		}
		updates.CredentialsEncrypted = encrypted
	}

	row, err := s.repo.Update(ctx, id, updates)
	if err != nil {
		return ProviderConnectionDTO{}, fmt.Errorf("domain provider: update: %w", err)
	}

	_ = s.audit.Log(ctx, auditsvc.LogEntry{
		Category:      auditsvc.CategoryInfrastructure,
		Severity:      auditsvc.SeverityInfo,
		ActorType:     auditsvc.ActorTypeUser,
		ActorID:       &actorID,
		ActorLabel:    actorLabel,
		Action:        "domain_provider.updated",
		ResourceType:  strPtr("domain_provider_connection"),
		ResourceID:    &id,
		CorrelationID: correlationID,
		SourceIP:      &sourceIP,
		Details: map[string]any{
			"display_name":        row.DisplayName,
			"credentials_updated": updates.CredentialsEncrypted != nil,
		},
	})

	return toDTO(row), nil
}

// DeleteConnection deletes a connection by ID after checking it's not in use.
func (s *Service) DeleteConnection(ctx context.Context, id string, actorID, actorLabel, sourceIP, correlationID string) error {
	existing, err := s.repo.GetByID(ctx, id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return fmt.Errorf("domain provider: not found")
		}
		return fmt.Errorf("domain provider: get for delete: %w", err)
	}

	// TODO(phase-2): check for references in domain profiles when that table exists.

	if err := s.repo.Delete(ctx, id); err != nil {
		return fmt.Errorf("domain provider: delete: %w", err)
	}

	_ = s.audit.Log(ctx, auditsvc.LogEntry{
		Category:      auditsvc.CategoryInfrastructure,
		Severity:      auditsvc.SeverityInfo,
		ActorType:     auditsvc.ActorTypeUser,
		ActorID:       &actorID,
		ActorLabel:    actorLabel,
		Action:        "domain_provider.deleted",
		ResourceType:  strPtr("domain_provider_connection"),
		ResourceID:    &id,
		CorrelationID: correlationID,
		SourceIP:      &sourceIP,
		Details: map[string]any{
			"provider_type": existing.ProviderType,
			"display_name":  existing.DisplayName,
		},
	})

	return nil
}

// TestConnection decrypts credentials, calls the provider API, updates status, and emits audit events.
func (s *Service) TestConnection(ctx context.Context, id string, actorID, actorLabel, sourceIP, correlationID string) (TestResult, error) {
	row, err := s.repo.GetByID(ctx, id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return TestResult{}, fmt.Errorf("domain provider: not found")
		}
		return TestResult{}, fmt.Errorf("domain provider: get for test: %w", err)
	}

	client, err := s.buildClient(row)
	if err != nil {
		return TestResult{}, fmt.Errorf("domain provider: build client: %w", err)
	}

	testErr := client.TestConnection()

	var (
		newStatus = repositories.ProviderStatusHealthy
		statusMsg *string
		result    TestResult
	)
	if testErr != nil {
		newStatus = repositories.ProviderStatusError
		msg := testErr.Error()
		statusMsg = &msg
		result = TestResult{Success: false, Message: msg}
	} else {
		result = TestResult{Success: true, Message: "Connection successful"}
	}

	_ = s.repo.UpdateStatus(ctx, id, newStatus, statusMsg)

	action := "domain_provider.test_success"
	severity := auditsvc.SeverityInfo
	details := map[string]any{"provider_type": row.ProviderType, "display_name": row.DisplayName}
	if testErr != nil {
		action = "domain_provider.test_failure"
		severity = auditsvc.SeverityWarning
		details["error"] = testErr.Error()
	}

	_ = s.audit.Log(ctx, auditsvc.LogEntry{
		Category:      auditsvc.CategoryInfrastructure,
		Severity:      severity,
		ActorType:     auditsvc.ActorTypeUser,
		ActorID:       &actorID,
		ActorLabel:    actorLabel,
		Action:        action,
		ResourceType:  strPtr("domain_provider_connection"),
		ResourceID:    &id,
		CorrelationID: correlationID,
		SourceIP:      &sourceIP,
		Details:       details,
	})

	return result, nil
}

// buildClient decrypts credentials and returns the appropriate ProviderClient.
func (s *Service) buildClient(row repositories.DomainProviderConnection) (providers.ProviderClient, error) {
	ptype := providers.ProviderType(row.ProviderType)
	conn := providers.ConnectionForTest{Type: ptype}

	switch ptype {
	case providers.ProviderTypeNamecheap:
		var c credentials.NamecheapCredentials
		if err := s.credEnc.Decrypt(row.CredentialsEncrypted, &c); err != nil {
			return nil, fmt.Errorf("decrypt namecheap credentials: %w", err)
		}
		conn.NamecheapCreds = &c
	case providers.ProviderTypeGoDaddy:
		var c credentials.GoDaddyCredentials
		if err := s.credEnc.Decrypt(row.CredentialsEncrypted, &c); err != nil {
			return nil, fmt.Errorf("decrypt godaddy credentials: %w", err)
		}
		conn.GoDaddyCreds = &c
	case providers.ProviderTypeRoute53:
		var c credentials.Route53Credentials
		if err := s.credEnc.Decrypt(row.CredentialsEncrypted, &c); err != nil {
			return nil, fmt.Errorf("decrypt route53 credentials: %w", err)
		}
		conn.Route53Creds = &c
	case providers.ProviderTypeAzureDNS:
		var c credentials.AzureDNSCredentials
		if err := s.credEnc.Decrypt(row.CredentialsEncrypted, &c); err != nil {
			return nil, fmt.Errorf("decrypt azuredns credentials: %w", err)
		}
		conn.AzureDNSCreds = &c
	default:
		return nil, fmt.Errorf("unknown provider type %q", row.ProviderType)
	}

	return providers.NewClientFromDecrypted(conn)
}

// --- validation helpers ---

func (s *Service) validateCreate(ctx context.Context, input CreateInput) error {
	if !providers.ValidProviderType(providers.ProviderType(input.ProviderType)) {
		return &ValidationError{Field: "provider_type", Message: fmt.Sprintf("must be one of: namecheap, godaddy, route53, azure_dns; got %q", input.ProviderType)}
	}
	if err := s.validateDisplayName(ctx, strings.TrimSpace(input.DisplayName), ""); err != nil {
		return err
	}
	return s.validateCredsForType(input.ProviderType, input)
}

func (s *Service) validateDisplayName(ctx context.Context, name, excludeID string) error {
	if name == "" {
		return &ValidationError{Field: "display_name", Message: "must not be empty"}
	}
	if len(name) > 255 {
		return &ValidationError{Field: "display_name", Message: "must not exceed 255 characters"}
	}
	exists, err := s.repo.ExistsDisplayName(ctx, name, excludeID)
	if err != nil {
		return fmt.Errorf("domain provider: check display name: %w", err)
	}
	if exists {
		return &ValidationError{Field: "display_name", Message: fmt.Sprintf("display name %q is already in use", name)}
	}
	return nil
}

func (s *Service) validateCredsForType(providerType string, input CreateInput) error {
	switch providers.ProviderType(providerType) {
	case providers.ProviderTypeNamecheap:
		c := input.NamecheapCreds
		if c == nil {
			return &ValidationError{Field: "namecheap_credentials", Message: "required for provider_type namecheap"}
		}
		if c.APIUser == "" {
			return &ValidationError{Field: "namecheap_credentials.api_user", Message: "required"}
		}
		if c.APIKey == "" {
			return &ValidationError{Field: "namecheap_credentials.api_key", Message: "required"}
		}
		if c.Username == "" {
			return &ValidationError{Field: "namecheap_credentials.username", Message: "required"}
		}
		if c.ClientIP == "" {
			return &ValidationError{Field: "namecheap_credentials.client_ip", Message: "required"}
		}
		if net.ParseIP(c.ClientIP) == nil {
			return &ValidationError{Field: "namecheap_credentials.client_ip", Message: "must be a valid IP address"}
		}

	case providers.ProviderTypeGoDaddy:
		c := input.GoDaddyCreds
		if c == nil {
			return &ValidationError{Field: "godaddy_credentials", Message: "required for provider_type godaddy"}
		}
		if c.APIKey == "" {
			return &ValidationError{Field: "godaddy_credentials.api_key", Message: "required"}
		}
		if c.APISecret == "" {
			return &ValidationError{Field: "godaddy_credentials.api_secret", Message: "required"}
		}
		if c.Environment != credentials.GoDaddyProduction && c.Environment != credentials.GoDaddyOTE {
			return &ValidationError{Field: "godaddy_credentials.environment", Message: "must be production or ote"}
		}

	case providers.ProviderTypeRoute53:
		c := input.Route53Creds
		if c == nil {
			return &ValidationError{Field: "route53_credentials", Message: "required for provider_type route53"}
		}
		if c.AccessKeyID == "" {
			return &ValidationError{Field: "route53_credentials.aws_access_key_id", Message: "required"}
		}
		if c.SecretAccessKey == "" {
			return &ValidationError{Field: "route53_credentials.aws_secret_access_key", Message: "required"}
		}
		if c.Region == "" {
			return &ValidationError{Field: "route53_credentials.region", Message: "required"}
		}
		if !awsRegionRE.MatchString(c.Region) {
			return &ValidationError{Field: "route53_credentials.region", Message: "must be a valid AWS region (e.g. us-east-1)"}
		}
		if c.IAMRoleARN != "" && !strings.HasPrefix(c.IAMRoleARN, "arn:aws:iam::") {
			return &ValidationError{Field: "route53_credentials.iam_role_arn", Message: "must be a valid IAM Role ARN (arn:aws:iam::...)"}
		}

	case providers.ProviderTypeAzureDNS:
		c := input.AzureDNSCreds
		if c == nil {
			return &ValidationError{Field: "azure_dns_credentials", Message: "required for provider_type azure_dns"}
		}
		if !uuidRE.MatchString(c.TenantID) {
			return &ValidationError{Field: "azure_dns_credentials.tenant_id", Message: "must be a valid UUID"}
		}
		if !uuidRE.MatchString(c.ClientID) {
			return &ValidationError{Field: "azure_dns_credentials.client_id", Message: "must be a valid UUID"}
		}
		if c.ClientSecret == "" {
			return &ValidationError{Field: "azure_dns_credentials.client_secret", Message: "required"}
		}
		if !uuidRE.MatchString(c.SubscriptionID) {
			return &ValidationError{Field: "azure_dns_credentials.subscription_id", Message: "must be a valid UUID"}
		}
		if c.ResourceGroup == "" {
			return &ValidationError{Field: "azure_dns_credentials.resource_group", Message: "required"}
		}
	}
	return nil
}

// encryptForType picks the correct credential struct and encrypts it.
func (s *Service) encryptForType(providerType string, input CreateInput) ([]byte, error) {
	switch providers.ProviderType(providerType) {
	case providers.ProviderTypeNamecheap:
		return s.credEnc.Encrypt(*input.NamecheapCreds)
	case providers.ProviderTypeGoDaddy:
		return s.credEnc.Encrypt(*input.GoDaddyCreds)
	case providers.ProviderTypeRoute53:
		return s.credEnc.Encrypt(*input.Route53Creds)
	case providers.ProviderTypeAzureDNS:
		return s.credEnc.Encrypt(*input.AzureDNSCreds)
	}
	return nil, fmt.Errorf("unsupported provider type: %s", providerType)
}

// extractCredsForType returns the credential struct from UpdateInput for the given provider type,
// or nil if no credentials were provided in the update.
func extractCredsForType(providerType string, input UpdateInput) any {
	switch providers.ProviderType(providerType) {
	case providers.ProviderTypeNamecheap:
		if input.NamecheapCreds != nil {
			return *input.NamecheapCreds
		}
	case providers.ProviderTypeGoDaddy:
		if input.GoDaddyCreds != nil {
			return *input.GoDaddyCreds
		}
	case providers.ProviderTypeRoute53:
		if input.Route53Creds != nil {
			return *input.Route53Creds
		}
	case providers.ProviderTypeAzureDNS:
		if input.AzureDNSCreds != nil {
			return *input.AzureDNSCreds
		}
	}
	return nil
}

// toDTO converts a repository row to an API DTO (credentials not included).
func toDTO(row repositories.DomainProviderConnection) ProviderConnectionDTO {
	dto := ProviderConnectionDTO{
		ID:            row.ID,
		ProviderType:  row.ProviderType,
		DisplayName:   row.DisplayName,
		Status:        string(row.Status),
		StatusMessage: row.StatusMessage,
		CreatedBy:     row.CreatedBy,
		CreatedAt:     row.CreatedAt.UTC().Format("2006-01-02T15:04:05Z"),
		UpdatedAt:     row.UpdatedAt.UTC().Format("2006-01-02T15:04:05Z"),
	}
	if row.LastTestedAt != nil {
		s := row.LastTestedAt.UTC().Format("2006-01-02T15:04:05Z")
		dto.LastTestedAt = &s
	}
	return dto
}

func strPtr(s string) *string { return &s }
