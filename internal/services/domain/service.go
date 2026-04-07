// Package domain implements business logic for domain lifecycle management.
// This covers domain registration (direct and approval-gated), domain import,
// profile CRUD, renewal tracking, and scheduled expiry sync/notifications.
package domain

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"

	"tackle/internal/providers"
	"tackle/internal/providers/credentials"
	"tackle/internal/providers/godaddy"
	"tackle/internal/providers/namecheap"
	r53 "tackle/internal/providers/route53"
	"tackle/internal/providers/azuredns"
	"tackle/internal/repositories"
	auditsvc "tackle/internal/services/audit"
	notifsvc "tackle/internal/services/notification"
)

// fqdnRE validates a basic FQDN (labels separated by dots, no leading/trailing dots).
var fqdnRE = regexp.MustCompile(`^(?:[a-zA-Z0-9](?:[a-zA-Z0-9\-]{0,61}[a-zA-Z0-9])?\.)+[a-zA-Z]{2,}$`)

// engineerRoleName is the role that bypasses the approval gate for domain registration.
const engineerRoleName = "Engineer"

// expiryWarningDays is the 30-day window for expiry notifications.
const expiryWarningDays = 30

// expiryCriticalDays is the 7-day window for critical expiry notifications.
const expiryCriticalDays = 7

// ValidationError is returned when input validation fails.
type ValidationError struct {
	Field   string
	Message string
}

func (e *ValidationError) Error() string {
	return fmt.Sprintf("validation: %s: %s", e.Field, e.Message)
}

// ConflictError is returned when an operation conflicts with existing state.
type ConflictError struct{ Message string }

func (e *ConflictError) Error() string { return e.Message }

// Service provides domain lifecycle business logic.
type Service struct {
	profileRepo  *repositories.DomainProfileRepository
	providerRepo *repositories.DomainProviderRepository
	credEnc      *credentials.EncryptionService
	audit        *auditsvc.AuditService
	notif        *notifsvc.NotificationService
	db           *sql.DB
}

// NewService creates a Service.
func NewService(
	profileRepo *repositories.DomainProfileRepository,
	providerRepo *repositories.DomainProviderRepository,
	credEnc *credentials.EncryptionService,
	audit *auditsvc.AuditService,
	notif *notifsvc.NotificationService,
	db *sql.DB,
) *Service {
	return &Service{
		profileRepo:  profileRepo,
		providerRepo: providerRepo,
		credEnc:      credEnc,
		audit:        audit,
		notif:        notif,
		db:           db,
	}
}

// --- DTOs ---

// DomainProfileDTO is the API-safe representation of a domain profile.
type DomainProfileDTO struct {
	ID                      string  `json:"id"`
	DomainName              string  `json:"domain_name"`
	RegistrarConnectionID   *string `json:"registrar_connection_id,omitempty"`
	DNSProviderConnectionID *string `json:"dns_provider_connection_id,omitempty"`
	Status                  string  `json:"status"`
	RegistrationDate        *string `json:"registration_date,omitempty"`
	ExpiryDate              *string `json:"expiry_date,omitempty"`
	Tags                    []string `json:"tags"`
	Notes                   *string `json:"notes,omitempty"`
	CreatedBy               string  `json:"created_by"`
	CreatedAt               string  `json:"created_at"`
	UpdatedAt               string  `json:"updated_at"`
	CampaignCount           int     `json:"campaign_count"`
}

// RenewalRecordDTO is the API representation of a renewal history record.
type RenewalRecordDTO struct {
	ID                    string   `json:"id"`
	DomainProfileID       string   `json:"domain_profile_id"`
	RenewalDate           string   `json:"renewal_date"`
	DurationYears         int      `json:"duration_years"`
	CostAmount            *float64 `json:"cost_amount,omitempty"`
	CostCurrency          *string  `json:"cost_currency,omitempty"`
	RegistrarConnectionID *string  `json:"registrar_connection_id,omitempty"`
	InitiatedBy           *string  `json:"initiated_by,omitempty"`
	CreatedAt             string   `json:"created_at"`
}

// RegistrationRequestDTO is the API representation of a pending registration request.
type RegistrationRequestDTO struct {
	ID                    string  `json:"id"`
	DomainName            string  `json:"domain_name"`
	RegistrarConnectionID string  `json:"registrar_connection_id"`
	Years                 int     `json:"years"`
	Status                string  `json:"status"`
	RequestedBy           string  `json:"requested_by"`
	ReviewedBy            *string `json:"reviewed_by,omitempty"`
	ReviewedAt            *string `json:"reviewed_at,omitempty"`
	RejectionReason       *string `json:"rejection_reason,omitempty"`
	DomainProfileID       *string `json:"domain_profile_id,omitempty"`
	CreatedAt             string  `json:"created_at"`
}

// AvailabilityResult is the result of a domain availability check.
type AvailabilityResult struct {
	Domain    string  `json:"domain"`
	Available bool    `json:"available"`
	Premium   bool    `json:"premium"`
	Price     float64 `json:"price,omitempty"`
	Currency  string  `json:"currency,omitempty"`
}

// RegistrantInfo holds contact data for domain registration.
type RegistrantInfo struct {
	FirstName    string `json:"first_name"`
	LastName     string `json:"last_name"`
	Address      string `json:"address"`
	City         string `json:"city"`
	State        string `json:"state"`
	PostalCode   string `json:"postal_code"`
	Country      string `json:"country"`
	Phone        string `json:"phone"`
	EmailAddress string `json:"email_address"`
}

// ListResult wraps paginated list results.
type ListResult struct {
	Items  []DomainProfileDTO `json:"items"`
	Total  int                `json:"total"`
	Limit  int                `json:"limit"`
	Offset int                `json:"offset"`
}

// --- Input types ---

// CheckAvailabilityInput is the input for an availability check.
type CheckAvailabilityInput struct {
	RegistrarConnectionID string
	Domain                string
}

// RegisterDomainInput is the input for a domain registration.
type RegisterDomainInput struct {
	RegistrarConnectionID   string
	DNSProviderConnectionID string
	Domain                  string
	Years                   int
	Registrant              RegistrantInfo
	Tags                    []string
	Notes                   string
}

// ImportDomainInput is the input for importing an existing domain.
type ImportDomainInput struct {
	DomainName              string
	RegistrarConnectionID   string
	DNSProviderConnectionID string
	RegistrationDate        *time.Time
	ExpiryDate              *time.Time
	Tags                    []string
	Notes                   string
	SyncExpiry              bool
}

// UpdateDomainInput is the input for updating a domain profile.
type UpdateDomainInput struct {
	DNSProviderConnectionID *string
	Tags                    *[]string
	Notes                   *string
	Status                  *string
}

// RenewDomainInput is the input for a manual renewal.
type RenewDomainInput struct {
	Years int
}

// ListDomainsInput is the input for a filtered domain list.
type ListDomainsInput struct {
	Status                  string
	RegistrarConnectionID   string
	DNSProviderConnectionID string
	Tag                     string
	ExpiryBefore            *time.Time
	ExpiryAfter             *time.Time
	CampaignID              string
	Search                  string
	SortField               string
	SortDesc                bool
	Limit                   int
	Offset                  int
}

// --- Operations ---

// CheckAvailability delegates an availability check to the appropriate registrar provider.
func (s *Service) CheckAvailability(ctx context.Context, input CheckAvailabilityInput) (AvailabilityResult, error) {
	if err := validateDomainName(input.Domain); err != nil {
		return AvailabilityResult{}, err
	}
	conn, err := s.providerRepo.GetByID(ctx, input.RegistrarConnectionID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return AvailabilityResult{}, &ValidationError{Field: "registrar_connection_id", Message: "provider connection not found"}
		}
		return AvailabilityResult{}, fmt.Errorf("domain: check availability: get provider: %w", err)
	}

	switch providers.ProviderType(conn.ProviderType) {
	case providers.ProviderTypeNamecheap:
		var creds credentials.NamecheapCredentials
		if err := s.credEnc.Decrypt(conn.CredentialsEncrypted, &creds); err != nil {
			return AvailabilityResult{}, fmt.Errorf("domain: check availability: decrypt: %w", err)
		}
		client := namecheap.NewClient(creds, 0)
		r, err := client.CheckAvailability(input.Domain)
		if err != nil {
			return AvailabilityResult{}, fmt.Errorf("domain: check availability: namecheap: %w", err)
		}
		return AvailabilityResult{Domain: input.Domain, Available: r.Available, Premium: r.Premium, Price: r.Price, Currency: r.Currency}, nil

	case providers.ProviderTypeGoDaddy:
		var creds credentials.GoDaddyCredentials
		if err := s.credEnc.Decrypt(conn.CredentialsEncrypted, &creds); err != nil {
			return AvailabilityResult{}, fmt.Errorf("domain: check availability: decrypt: %w", err)
		}
		client := godaddy.NewClient(creds, 0)
		r, err := client.CheckAvailability(input.Domain)
		if err != nil {
			return AvailabilityResult{}, fmt.Errorf("domain: check availability: godaddy: %w", err)
		}
		return AvailabilityResult{Domain: input.Domain, Available: r.Available, Premium: r.Premium, Price: r.Price, Currency: r.Currency}, nil

	default:
		return AvailabilityResult{}, &ValidationError{Field: "registrar_connection_id", Message: fmt.Sprintf("provider type %q does not support domain availability checks", conn.ProviderType)}
	}
}

// RegisterDomain orchestrates domain registration.
// Engineers register directly; all other roles create a pending approval request.
func (s *Service) RegisterDomain(ctx context.Context, input RegisterDomainInput, actorID, actorLabel, actorRole, sourceIP, correlationID string) (DomainProfileDTO, bool, error) {
	if err := validateDomainName(input.Domain); err != nil {
		return DomainProfileDTO{}, false, err
	}
	if input.Years < 1 || input.Years > 10 {
		return DomainProfileDTO{}, false, &ValidationError{Field: "years", Message: "must be between 1 and 10"}
	}

	conn, err := s.providerRepo.GetByID(ctx, input.RegistrarConnectionID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return DomainProfileDTO{}, false, &ValidationError{Field: "registrar_connection_id", Message: "provider connection not found"}
		}
		return DomainProfileDTO{}, false, fmt.Errorf("domain: register: get provider: %w", err)
	}

	// Non-Engineers go through approval flow.
	if !strings.EqualFold(actorRole, engineerRoleName) {
		regInfo, _ := json.Marshal(input.Registrant)
		req := repositories.RegistrationRequest{
			DomainName:            input.Domain,
			RegistrarConnectionID: input.RegistrarConnectionID,
			Years:                 input.Years,
			RegistrantInfo:        regInfo,
			RequestedBy:           actorID,
		}
		_, err := s.profileRepo.CreateRegistrationRequest(ctx, req)
		if err != nil {
			return DomainProfileDTO{}, false, fmt.Errorf("domain: register: create request: %w", err)
		}
		_ = s.audit.Log(ctx, auditsvc.LogEntry{
			Category: auditsvc.CategoryInfrastructure, Severity: auditsvc.SeverityInfo,
			ActorType: auditsvc.ActorTypeUser, ActorID: &actorID, ActorLabel: actorLabel,
			Action: "domain.registration_requested", ResourceType: strPtr("domain"),
			CorrelationID: correlationID, SourceIP: &sourceIP,
			Details: map[string]any{"domain": input.Domain, "years": input.Years},
		})
		// Return empty DTO with isPending=true
		return DomainProfileDTO{DomainName: input.Domain, Status: "pending_registration", Tags: []string{}}, true, nil
	}

	// Engineer — register directly.
	profile, err := s.executeRegistration(ctx, conn, input, actorID, actorLabel, sourceIP, correlationID)
	if err != nil {
		return DomainProfileDTO{}, false, err
	}
	return profile, false, nil
}

// ApproveRegistration allows an Engineer to approve a pending registration request.
func (s *Service) ApproveRegistration(ctx context.Context, requestID, actorID, actorLabel, sourceIP, correlationID string) (DomainProfileDTO, error) {
	req, err := s.profileRepo.GetRegistrationRequest(ctx, requestID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return DomainProfileDTO{}, fmt.Errorf("domain: approve registration: request not found")
		}
		return DomainProfileDTO{}, fmt.Errorf("domain: approve registration: %w", err)
	}
	if req.Status != "pending" {
		return DomainProfileDTO{}, &ConflictError{Message: fmt.Sprintf("registration request is already %s", req.Status)}
	}

	conn, err := s.providerRepo.GetByID(ctx, req.RegistrarConnectionID)
	if err != nil {
		return DomainProfileDTO{}, fmt.Errorf("domain: approve registration: get provider: %w", err)
	}

	var registrant RegistrantInfo
	_ = json.Unmarshal(req.RegistrantInfo, &registrant)

	regInput := RegisterDomainInput{
		RegistrarConnectionID: req.RegistrarConnectionID,
		Domain:                req.DomainName,
		Years:                 req.Years,
		Registrant:            registrant,
	}

	profile, err := s.executeRegistration(ctx, conn, regInput, actorID, actorLabel, sourceIP, correlationID)
	if err != nil {
		_ = s.profileRepo.UpdateRegistrationRequest(ctx, requestID, "rejected", actorID, strPtr(err.Error()), nil)
		return DomainProfileDTO{}, err
	}

	if err := s.profileRepo.UpdateRegistrationRequest(ctx, requestID, "approved", actorID, nil, &profile.ID); err != nil {
		// Non-fatal — profile already created.
		_ = err
	}

	_ = s.audit.Log(ctx, auditsvc.LogEntry{
		Category: auditsvc.CategoryInfrastructure, Severity: auditsvc.SeverityInfo,
		ActorType: auditsvc.ActorTypeUser, ActorID: &actorID, ActorLabel: actorLabel,
		Action: "domain.registration_approved", ResourceType: strPtr("domain"),
		ResourceID: &profile.ID, CorrelationID: correlationID, SourceIP: &sourceIP,
		Details: map[string]any{"domain": req.DomainName, "request_id": requestID},
	})
	return profile, nil
}

// RejectRegistration allows an Engineer to reject a pending registration request.
func (s *Service) RejectRegistration(ctx context.Context, requestID, reason, actorID, actorLabel, sourceIP, correlationID string) error {
	req, err := s.profileRepo.GetRegistrationRequest(ctx, requestID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return fmt.Errorf("domain: reject registration: request not found")
		}
		return fmt.Errorf("domain: reject registration: %w", err)
	}
	if req.Status != "pending" {
		return &ConflictError{Message: fmt.Sprintf("registration request is already %s", req.Status)}
	}

	if err := s.profileRepo.UpdateRegistrationRequest(ctx, requestID, "rejected", actorID, &reason, nil); err != nil {
		return fmt.Errorf("domain: reject registration: %w", err)
	}

	_ = s.audit.Log(ctx, auditsvc.LogEntry{
		Category: auditsvc.CategoryInfrastructure, Severity: auditsvc.SeverityInfo,
		ActorType: auditsvc.ActorTypeUser, ActorID: &actorID, ActorLabel: actorLabel,
		Action: "domain.registration_rejected", ResourceType: strPtr("domain_registration_request"),
		ResourceID: &requestID, CorrelationID: correlationID, SourceIP: &sourceIP,
		Details: map[string]any{"domain": req.DomainName, "reason": reason},
	})
	return nil
}

// ImportDomain creates a domain profile from an existing domain (no registration via API).
func (s *Service) ImportDomain(ctx context.Context, input ImportDomainInput, actorID, actorLabel, sourceIP, correlationID string) (DomainProfileDTO, error) {
	if err := validateDomainName(input.DomainName); err != nil {
		return DomainProfileDTO{}, err
	}
	if input.RegistrarConnectionID == "" && input.DNSProviderConnectionID == "" {
		return DomainProfileDTO{}, &ValidationError{Field: "registrar_connection_id", Message: "at least one of registrar_connection_id or dns_provider_connection_id is required"}
	}

	// Validate referenced connections exist.
	if err := s.validateConnectionRef(ctx, input.RegistrarConnectionID); err != nil {
		return DomainProfileDTO{}, &ValidationError{Field: "registrar_connection_id", Message: err.Error()}
	}
	if err := s.validateConnectionRef(ctx, input.DNSProviderConnectionID); err != nil {
		return DomainProfileDTO{}, &ValidationError{Field: "dns_provider_connection_id", Message: err.Error()}
	}

	p := repositories.DomainProfile{
		DomainName:       input.DomainName,
		Status:           repositories.DomainStatusActive,
		Tags:             input.Tags,
		CreatedBy:        actorID,
		RegistrationDate: input.RegistrationDate,
		ExpiryDate:       input.ExpiryDate,
	}
	if input.Notes != "" {
		p.Notes = &input.Notes
	}
	if input.RegistrarConnectionID != "" {
		p.RegistrarConnectionID = &input.RegistrarConnectionID
	}
	if input.DNSProviderConnectionID != "" {
		p.DNSProviderConnectionID = &input.DNSProviderConnectionID
	}
	if p.Tags == nil {
		p.Tags = []string{}
	}

	profile, err := s.profileRepo.Create(ctx, p)
	if err != nil {
		return DomainProfileDTO{}, fmt.Errorf("domain: import: create profile: %w", err)
	}

	_ = s.audit.Log(ctx, auditsvc.LogEntry{
		Category: auditsvc.CategoryInfrastructure, Severity: auditsvc.SeverityInfo,
		ActorType: auditsvc.ActorTypeUser, ActorID: &actorID, ActorLabel: actorLabel,
		Action: "domain.imported", ResourceType: strPtr("domain"), ResourceID: &profile.ID,
		CorrelationID: correlationID, SourceIP: &sourceIP,
		Details: map[string]any{"domain": profile.DomainName},
	})

	// Optionally sync expiry from registrar.
	if input.SyncExpiry && input.RegistrarConnectionID != "" {
		go s.syncSingleDomain(context.Background(), profile)
	}

	return toDTO(profile, 0), nil
}

// GetDomainProfile returns a domain profile DTO with campaign count.
func (s *Service) GetDomainProfile(ctx context.Context, id string) (DomainProfileDTO, error) {
	profile, err := s.profileRepo.GetByID(ctx, id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return DomainProfileDTO{}, fmt.Errorf("domain: not found")
		}
		return DomainProfileDTO{}, fmt.Errorf("domain: get: %w", err)
	}

	assocs, _ := s.profileRepo.CheckCampaignAssociations(ctx, id)
	return toDTO(profile, len(assocs)), nil
}

// ListDomainProfiles returns a paginated, filtered list of domain profiles.
func (s *Service) ListDomainProfiles(ctx context.Context, input ListDomainsInput) (ListResult, error) {
	if input.Limit <= 0 {
		input.Limit = 50
	}
	if input.Limit > 200 {
		input.Limit = 200
	}

	filters := repositories.DomainProfileFilters{
		Status:                  input.Status,
		RegistrarConnectionID:   input.RegistrarConnectionID,
		DNSProviderConnectionID: input.DNSProviderConnectionID,
		Tag:                     input.Tag,
		ExpiryBefore:            input.ExpiryBefore,
		ExpiryAfter:             input.ExpiryAfter,
		CampaignID:              input.CampaignID,
		Search:                  input.Search,
	}
	sort := repositories.DomainProfileSort{
		Field: input.SortField,
		Desc:  input.SortDesc,
	}

	profiles, total, err := s.profileRepo.List(ctx, filters, sort, input.Limit, input.Offset)
	if err != nil {
		return ListResult{}, fmt.Errorf("domain: list: %w", err)
	}

	items := make([]DomainProfileDTO, len(profiles))
	for i, p := range profiles {
		items[i] = toDTO(p, 0)
	}
	return ListResult{Items: items, Total: total, Limit: input.Limit, Offset: input.Offset}, nil
}

// UpdateDomainProfile updates mutable fields on a domain profile.
func (s *Service) UpdateDomainProfile(ctx context.Context, id string, input UpdateDomainInput, actorID, actorLabel, sourceIP, correlationID string) (DomainProfileDTO, error) {
	_, err := s.profileRepo.GetByID(ctx, id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return DomainProfileDTO{}, fmt.Errorf("domain: not found")
		}
		return DomainProfileDTO{}, fmt.Errorf("domain: update: get: %w", err)
	}

	updates := repositories.DomainProfileUpdate{}
	if input.DNSProviderConnectionID != nil {
		if *input.DNSProviderConnectionID != "" {
			if err := s.validateConnectionRef(ctx, *input.DNSProviderConnectionID); err != nil {
				return DomainProfileDTO{}, &ValidationError{Field: "dns_provider_connection_id", Message: err.Error()}
			}
		}
		updates.DNSProviderConnectionID = input.DNSProviderConnectionID
	}
	if input.Tags != nil {
		updates.Tags = input.Tags
	}
	if input.Notes != nil {
		updates.Notes = input.Notes
	}
	if input.Status != nil {
		s := repositories.DomainStatus(*input.Status)
		updates.Status = &s
	}

	profile, err := s.profileRepo.Update(ctx, id, updates)
	if err != nil {
		return DomainProfileDTO{}, fmt.Errorf("domain: update: %w", err)
	}

	_ = s.audit.Log(ctx, auditsvc.LogEntry{
		Category: auditsvc.CategoryInfrastructure, Severity: auditsvc.SeverityInfo,
		ActorType: auditsvc.ActorTypeUser, ActorID: &actorID, ActorLabel: actorLabel,
		Action: "domain.updated", ResourceType: strPtr("domain"), ResourceID: &id,
		CorrelationID: correlationID, SourceIP: &sourceIP,
		Details: map[string]any{"domain": profile.DomainName},
	})

	assocs, _ := s.profileRepo.CheckCampaignAssociations(ctx, id)
	return toDTO(profile, len(assocs)), nil
}

// DecommissionDomain soft-deletes a domain profile.
func (s *Service) DecommissionDomain(ctx context.Context, id string, actorID, actorLabel, sourceIP, correlationID string) error {
	profile, err := s.profileRepo.GetByID(ctx, id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return fmt.Errorf("domain: not found")
		}
		return fmt.Errorf("domain: decommission: get: %w", err)
	}

	assocs, err := s.profileRepo.CheckCampaignAssociations(ctx, id)
	if err != nil {
		return fmt.Errorf("domain: decommission: check campaigns: %w", err)
	}
	if len(assocs) > 0 {
		return &ConflictError{Message: fmt.Sprintf("domain %q has %d active campaign association(s); remove associations before decommissioning", profile.DomainName, len(assocs))}
	}

	if err := s.profileRepo.SoftDelete(ctx, id); err != nil {
		return fmt.Errorf("domain: decommission: %w", err)
	}

	_ = s.audit.Log(ctx, auditsvc.LogEntry{
		Category: auditsvc.CategoryInfrastructure, Severity: auditsvc.SeverityInfo,
		ActorType: auditsvc.ActorTypeUser, ActorID: &actorID, ActorLabel: actorLabel,
		Action: "domain.decommissioned", ResourceType: strPtr("domain"), ResourceID: &id,
		CorrelationID: correlationID, SourceIP: &sourceIP,
		Details: map[string]any{"domain": profile.DomainName},
	})
	return nil
}

// RenewDomain executes a manual domain renewal via the registrar API.
func (s *Service) RenewDomain(ctx context.Context, domainID string, years int, actorID, actorLabel, sourceIP, correlationID string) (RenewalRecordDTO, error) {
	if years < 1 || years > 10 {
		return RenewalRecordDTO{}, &ValidationError{Field: "years", Message: "must be between 1 and 10"}
	}

	profile, err := s.profileRepo.GetByID(ctx, domainID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return RenewalRecordDTO{}, fmt.Errorf("domain: not found")
		}
		return RenewalRecordDTO{}, fmt.Errorf("domain: renew: get profile: %w", err)
	}
	if profile.RegistrarConnectionID == nil {
		return RenewalRecordDTO{}, &ConflictError{Message: fmt.Sprintf("domain %q has no registrar connection; please renew manually at the registrar console", profile.DomainName)}
	}

	conn, err := s.providerRepo.GetByID(ctx, *profile.RegistrarConnectionID)
	if err != nil {
		return RenewalRecordDTO{}, fmt.Errorf("domain: renew: get provider: %w", err)
	}

	var newExpiry time.Time
	var orderID string

	switch providers.ProviderType(conn.ProviderType) {
	case providers.ProviderTypeNamecheap:
		var creds credentials.NamecheapCredentials
		if err := s.credEnc.Decrypt(conn.CredentialsEncrypted, &creds); err != nil {
			return RenewalRecordDTO{}, fmt.Errorf("domain: renew: decrypt: %w", err)
		}
		client := namecheap.NewClient(creds, 0)
		result, err := client.RenewDomain(profile.DomainName, years)
		if err != nil {
			_ = s.audit.Log(ctx, auditsvc.LogEntry{
				Category: auditsvc.CategoryInfrastructure, Severity: auditsvc.SeverityWarning,
				ActorType: auditsvc.ActorTypeUser, ActorID: &actorID, ActorLabel: actorLabel,
				Action: "domain.renewal_failed", ResourceType: strPtr("domain"), ResourceID: &domainID,
				CorrelationID: correlationID, SourceIP: &sourceIP,
				Details: map[string]any{"domain": profile.DomainName, "error": err.Error()},
			})
			return RenewalRecordDTO{}, fmt.Errorf("domain: renew: namecheap: %w", err)
		}
		newExpiry = result.ExpiryDate
		orderID = result.OrderID

	case providers.ProviderTypeGoDaddy:
		var creds credentials.GoDaddyCredentials
		if err := s.credEnc.Decrypt(conn.CredentialsEncrypted, &creds); err != nil {
			return RenewalRecordDTO{}, fmt.Errorf("domain: renew: decrypt: %w", err)
		}
		client := godaddy.NewClient(creds, 0)
		result, err := client.RenewDomain(profile.DomainName, years)
		if err != nil {
			_ = s.audit.Log(ctx, auditsvc.LogEntry{
				Category: auditsvc.CategoryInfrastructure, Severity: auditsvc.SeverityWarning,
				ActorType: auditsvc.ActorTypeUser, ActorID: &actorID, ActorLabel: actorLabel,
				Action: "domain.renewal_failed", ResourceType: strPtr("domain"), ResourceID: &domainID,
				CorrelationID: correlationID, SourceIP: &sourceIP,
				Details: map[string]any{"domain": profile.DomainName, "error": err.Error()},
			})
			return RenewalRecordDTO{}, fmt.Errorf("domain: renew: godaddy: %w", err)
		}
		orderID = result.OrderID
		// GoDaddy renew doesn't return new expiry — sync it.
		if info, err := client.GetDomainInfo(profile.DomainName); err == nil {
			newExpiry = info.ExpiryDate
		}

	default:
		msg := fmt.Sprintf("registrar %q does not support renewal via API. Please visit your registrar console to renew %q manually.", conn.DisplayName, profile.DomainName)
		return RenewalRecordDTO{}, &ConflictError{Message: msg}
	}

	// Update domain profile expiry.
	if !newExpiry.IsZero() {
		_, _ = s.profileRepo.Update(ctx, domainID, repositories.DomainProfileUpdate{ExpiryDate: &newExpiry})
	}

	// Record renewal history.
	connID := conn.ID
	renewalRec, err := s.profileRepo.AddRenewalRecord(ctx, repositories.DomainRenewalRecord{
		DomainProfileID:       domainID,
		RenewalDate:           time.Now().UTC(),
		DurationYears:         years,
		RegistrarConnectionID: &connID,
		InitiatedBy:           &actorID,
	})
	if err != nil {
		return RenewalRecordDTO{}, fmt.Errorf("domain: renew: record history: %w", err)
	}

	_ = s.audit.Log(ctx, auditsvc.LogEntry{
		Category: auditsvc.CategoryInfrastructure, Severity: auditsvc.SeverityInfo,
		ActorType: auditsvc.ActorTypeUser, ActorID: &actorID, ActorLabel: actorLabel,
		Action: "domain.renewed", ResourceType: strPtr("domain"), ResourceID: &domainID,
		CorrelationID: correlationID, SourceIP: &sourceIP,
		Details: map[string]any{"domain": profile.DomainName, "years": years, "order_id": orderID},
	})

	return toRenewalDTO(renewalRec), nil
}

// GetRenewalHistory returns the renewal history for a domain.
func (s *Service) GetRenewalHistory(ctx context.Context, domainID string) ([]RenewalRecordDTO, error) {
	if _, err := s.profileRepo.GetByID(ctx, domainID); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, fmt.Errorf("domain: not found")
		}
		return nil, fmt.Errorf("domain: renewal history: get profile: %w", err)
	}

	records, err := s.profileRepo.ListRenewalHistory(ctx, domainID)
	if err != nil {
		return nil, fmt.Errorf("domain: renewal history: %w", err)
	}

	dtos := make([]RenewalRecordDTO, len(records))
	for i, r := range records {
		dtos[i] = toRenewalDTO(r)
	}
	return dtos, nil
}

// --- Scheduled jobs ---

// SyncExpiryDates fetches current expiry dates from registrar APIs for all active domains
// and updates local records. Intended to be called by a scheduler.
func (s *Service) SyncExpiryDates(ctx context.Context) error {
	profiles, err := s.profileRepo.GetActiveWithRegistrar(ctx)
	if err != nil {
		return fmt.Errorf("domain: sync expiry: list: %w", err)
	}

	synced := 0
	changed := 0
	for _, p := range profiles {
		updated, err := s.syncSingleDomain(ctx, p)
		if err != nil {
			// Log but continue with other domains.
			_ = s.audit.Log(ctx, auditsvc.LogEntry{
				Category: auditsvc.CategorySystem, Severity: auditsvc.SeverityWarning,
				ActorType: auditsvc.ActorTypeSystem,
				Action:    "domain.expiry_sync_error",
				Details:   map[string]any{"domain": p.DomainName, "error": err.Error()},
			})
			continue
		}
		synced++
		if updated {
			changed++
		}
	}

	_ = s.audit.Log(ctx, auditsvc.LogEntry{
		Category: auditsvc.CategorySystem, Severity: auditsvc.SeverityInfo,
		ActorType: auditsvc.ActorTypeSystem,
		Action:    "domain.expiry_sync",
		Details:   map[string]any{"synced": synced, "changed": changed},
	})
	return nil
}

// SyncAllProviders fetches all active domain provider connections, retrieves their domain lists,
// and upserts any missing domains into the local database as active domain profiles.
func (s *Service) SyncAllProviders(ctx context.Context, actorID, actorLabel, sourceIP, correlationID string) error {
	conns, err := s.providerRepo.List(ctx, repositories.DomainProviderFilters{})
	if err != nil {
		return fmt.Errorf("domain: sync all providers: list connections: %w", err)
	}

	totalSynced := 0
	totalNew := 0

	for _, conn := range conns {
		if conn.Status == repositories.ProviderStatusError {
			continue // Skip known broken connections, try untested or healthy
		}
		
		var client providers.ProviderClient
		
		switch providers.ProviderType(conn.ProviderType) {
		case providers.ProviderTypeNamecheap:
			var creds credentials.NamecheapCredentials
			if err := s.credEnc.Decrypt(conn.CredentialsEncrypted, &creds); err != nil {
				continue
			}
			client = namecheap.NewClient(creds, 0)
			
		case providers.ProviderTypeGoDaddy:
			var creds credentials.GoDaddyCredentials
			if err := s.credEnc.Decrypt(conn.CredentialsEncrypted, &creds); err != nil {
				continue
			}
			client = godaddy.NewClient(creds, 0)
			
		case providers.ProviderTypeRoute53:
			var creds credentials.Route53Credentials
			if err := s.credEnc.Decrypt(conn.CredentialsEncrypted, &creds); err != nil {
				continue
			}
			c, err := r53.NewClient(creds, 0)
			if err != nil {
				continue
			}
			client = c
			
		case providers.ProviderTypeAzureDNS:
			var creds credentials.AzureDNSCredentials
			if err := s.credEnc.Decrypt(conn.CredentialsEncrypted, &creds); err != nil {
				continue
			}
			client = azuredns.NewClient(creds, 0)
			
		default:
			continue
		}
		
		domains, err := client.ListDomains()
		if err != nil {
			_ = s.audit.Log(ctx, auditsvc.LogEntry{
				Category: auditsvc.CategorySystem, Severity: auditsvc.SeverityWarning,
				ActorType: auditsvc.ActorTypeSystem, Action: "domain.sync_provider_error",
				Details: map[string]any{"provider_id": conn.ID, "error": err.Error()},
			})
			continue
		}
		
		for _, rawDomain := range domains {
			domainName := strings.ToLower(strings.TrimSpace(rawDomain))
			if err := validateDomainName(domainName); err != nil {
				continue
			}
			
			// Check if exists
			_, err := s.profileRepo.GetByDomainName(ctx, domainName)
			if err == nil {
				totalSynced++
				continue
			}
			
			if !errors.Is(err, sql.ErrNoRows) {
				continue
			}
			
			// Does not exist, create it
			connID := conn.ID
			p := repositories.DomainProfile{
				DomainName: domainName,
				Status: repositories.DomainStatusActive,
				Tags: []string{"auto-synced"},
				CreatedBy: actorID,
			}
			
			if providers.ProviderType(conn.ProviderType) == providers.ProviderTypeNamecheap || providers.ProviderType(conn.ProviderType) == providers.ProviderTypeGoDaddy {
				p.RegistrarConnectionID = &connID
			} else {
				p.DNSProviderConnectionID = &connID
			}

			_, crErr := s.profileRepo.Create(ctx, p)
			if crErr == nil {
				totalNew++
			}
			totalSynced++
		}
		
		// Update LastTestedAt to signal it was just successfully talked to.
		_ = s.providerRepo.UpdateStatus(ctx, conn.ID, repositories.ProviderStatusHealthy, nil)
	}
	
	_ = s.audit.Log(ctx, auditsvc.LogEntry{
		Category: auditsvc.CategorySystem, Severity: auditsvc.SeverityInfo,
		ActorType: auditsvc.ActorTypeUser, ActorID: &actorID, ActorLabel: actorLabel,
		Action: "domain.providers_synced",
		CorrelationID: correlationID, SourceIP: &sourceIP,
		Details: map[string]any{"total_domains_synced": totalSynced, "total_new_imported": totalNew},
	})

	return nil
}

// CheckExpiryNotifications fires notifications for domains entering the 30-day and 7-day windows.
// Intended to be called by a scheduler.
func (s *Service) CheckExpiryNotifications(ctx context.Context) error {
	warning, err := s.profileRepo.GetExpiring(ctx, expiryWarningDays)
	if err != nil {
		return fmt.Errorf("domain: expiry notifications: get 30-day: %w", err)
	}
	critical, err := s.profileRepo.GetExpiring(ctx, expiryCriticalDays)
	if err != nil {
		return fmt.Errorf("domain: expiry notifications: get 7-day: %w", err)
	}

	criticalIDs := make(map[string]bool, len(critical))
	for _, p := range critical {
		criticalIDs[p.ID] = true
	}

	for _, p := range warning {
		if criticalIDs[p.ID] {
			// Emit critical notification.
			s.notif.Create(ctx, notifsvc.CreateNotificationParams{
				Category:     "infrastructure",
				Severity:     "critical",
				Title:        fmt.Sprintf("Domain expiring in 7 days: %s", p.DomainName),
				Body:         fmt.Sprintf("Domain %q expires on %s. Renew immediately or decommission to avoid disruption.", p.DomainName, p.ExpiryDate.Format("2006-01-02")),
				ResourceType: "domain",
				ResourceID:   p.ID,
				ActionURL:    fmt.Sprintf("/settings/domains/%s", p.ID),
				Recipients:   notifsvc.RecipientSpec{Role: engineerRoleName},
			})
			_ = s.audit.Log(ctx, auditsvc.LogEntry{
				Category: auditsvc.CategorySystem, Severity: auditsvc.SeverityCritical,
				ActorType: auditsvc.ActorTypeSystem, Action: "domain.expiry_critical",
				ResourceType: strPtr("domain"), ResourceID: &p.ID,
				Details: map[string]any{"domain": p.DomainName, "expiry_date": p.ExpiryDate},
			})
		} else {
			// Emit 30-day warning notification.
			s.notif.Create(ctx, notifsvc.CreateNotificationParams{
				Category:     "infrastructure",
				Severity:     "warning",
				Title:        fmt.Sprintf("Domain expiring in 30 days: %s", p.DomainName),
				Body:         fmt.Sprintf("Domain %q expires on %s. Consider renewing soon.", p.DomainName, p.ExpiryDate.Format("2006-01-02")),
				ResourceType: "domain",
				ResourceID:   p.ID,
				ActionURL:    fmt.Sprintf("/settings/domains/%s", p.ID),
				Recipients:   notifsvc.RecipientSpec{Role: engineerRoleName},
			})
			_ = s.audit.Log(ctx, auditsvc.LogEntry{
				Category: auditsvc.CategorySystem, Severity: auditsvc.SeverityWarning,
				ActorType: auditsvc.ActorTypeSystem, Action: "domain.expiry_warning",
				ResourceType: strPtr("domain"), ResourceID: &p.ID,
				Details: map[string]any{"domain": p.DomainName, "expiry_date": p.ExpiryDate},
			})
		}
	}
	return nil
}

// --- Internal helpers ---

// executeRegistration runs the actual provider API registration and creates the domain profile.
func (s *Service) executeRegistration(ctx context.Context, conn repositories.DomainProviderConnection, input RegisterDomainInput, actorID, actorLabel, sourceIP, correlationID string) (DomainProfileDTO, error) {
	var regDate, expiryDate time.Time

	switch providers.ProviderType(conn.ProviderType) {
	case providers.ProviderTypeNamecheap:
		var creds credentials.NamecheapCredentials
		if err := s.credEnc.Decrypt(conn.CredentialsEncrypted, &creds); err != nil {
			return DomainProfileDTO{}, fmt.Errorf("domain: register: decrypt: %w", err)
		}
		client := namecheap.NewClient(creds, 0)
		result, err := client.RegisterDomain(input.Domain, input.Years, namecheap.RegistrantInfo{
			FirstName: input.Registrant.FirstName, LastName: input.Registrant.LastName,
			Address: input.Registrant.Address, City: input.Registrant.City,
			StateProvince: input.Registrant.State, PostalCode: input.Registrant.PostalCode,
			Country: input.Registrant.Country, Phone: input.Registrant.Phone,
			EmailAddress: input.Registrant.EmailAddress,
		})
		if err != nil {
			_ = s.audit.Log(ctx, auditsvc.LogEntry{
				Category: auditsvc.CategoryInfrastructure, Severity: auditsvc.SeverityWarning,
				ActorType: auditsvc.ActorTypeUser, ActorID: &actorID, ActorLabel: actorLabel,
				Action: "domain.registration_failed", ResourceType: strPtr("domain"),
				CorrelationID: correlationID, SourceIP: &sourceIP,
				Details: map[string]any{"domain": input.Domain, "error": err.Error()},
			})
			return DomainProfileDTO{}, fmt.Errorf("domain: register: namecheap: %w", err)
		}
		regDate = result.RegistrationDate
		expiryDate = result.ExpiryDate

	case providers.ProviderTypeGoDaddy:
		var creds credentials.GoDaddyCredentials
		if err := s.credEnc.Decrypt(conn.CredentialsEncrypted, &creds); err != nil {
			return DomainProfileDTO{}, fmt.Errorf("domain: register: decrypt: %w", err)
		}
		client := godaddy.NewClient(creds, 0)
		result, err := client.RegisterDomain(input.Domain, input.Years, godaddy.RegistrantInfo{
			FirstName: input.Registrant.FirstName, LastName: input.Registrant.LastName,
			Address: input.Registrant.Address, City: input.Registrant.City,
			State: input.Registrant.State, PostalCode: input.Registrant.PostalCode,
			Country: input.Registrant.Country, Phone: input.Registrant.Phone,
			EmailAddress: input.Registrant.EmailAddress,
		})
		if err != nil {
			_ = s.audit.Log(ctx, auditsvc.LogEntry{
				Category: auditsvc.CategoryInfrastructure, Severity: auditsvc.SeverityWarning,
				ActorType: auditsvc.ActorTypeUser, ActorID: &actorID, ActorLabel: actorLabel,
				Action: "domain.registration_failed", ResourceType: strPtr("domain"),
				CorrelationID: correlationID, SourceIP: &sourceIP,
				Details: map[string]any{"domain": input.Domain, "error": err.Error()},
			})
			return DomainProfileDTO{}, fmt.Errorf("domain: register: godaddy: %w", err)
		}
		regDate = result.RegistrationDate
		expiryDate = result.ExpiryDate

	default:
		return DomainProfileDTO{}, &ValidationError{Field: "registrar_connection_id", Message: fmt.Sprintf("provider type %q does not support domain registration via API", conn.ProviderType)}
	}

	tags := input.Tags
	if tags == nil {
		tags = []string{}
	}
	p := repositories.DomainProfile{
		DomainName:            input.Domain,
		RegistrarConnectionID: &conn.ID,
		Status:                repositories.DomainStatusActive,
		Tags:                  tags,
		CreatedBy:             actorID,
	}
	if input.DNSProviderConnectionID != "" {
		p.DNSProviderConnectionID = &input.DNSProviderConnectionID
	}
	if !regDate.IsZero() {
		p.RegistrationDate = &regDate
	}
	if !expiryDate.IsZero() {
		p.ExpiryDate = &expiryDate
	}
	if input.Notes != "" {
		p.Notes = &input.Notes
	}

	profile, err := s.profileRepo.Create(ctx, p)
	if err != nil {
		return DomainProfileDTO{}, fmt.Errorf("domain: register: create profile: %w", err)
	}

	_ = s.audit.Log(ctx, auditsvc.LogEntry{
		Category: auditsvc.CategoryInfrastructure, Severity: auditsvc.SeverityInfo,
		ActorType: auditsvc.ActorTypeUser, ActorID: &actorID, ActorLabel: actorLabel,
		Action: "domain.registered", ResourceType: strPtr("domain"), ResourceID: &profile.ID,
		CorrelationID: correlationID, SourceIP: &sourceIP,
		Details: map[string]any{"domain": profile.DomainName, "registrar": conn.DisplayName},
	})

	return toDTO(profile, 0), nil
}

// syncSingleDomain fetches the current expiry from the registrar for one domain.
// Returns true if the expiry date was changed.
func (s *Service) syncSingleDomain(ctx context.Context, p repositories.DomainProfile) (bool, error) {
	if p.RegistrarConnectionID == nil {
		return false, nil
	}
	conn, err := s.providerRepo.GetByID(ctx, *p.RegistrarConnectionID)
	if err != nil {
		return false, fmt.Errorf("sync domain %s: get provider: %w", p.DomainName, err)
	}

	var info *domainInfoResult

	switch providers.ProviderType(conn.ProviderType) {
	case providers.ProviderTypeNamecheap:
		var creds credentials.NamecheapCredentials
		if err := s.credEnc.Decrypt(conn.CredentialsEncrypted, &creds); err != nil {
			return false, fmt.Errorf("sync domain %s: decrypt: %w", p.DomainName, err)
		}
		r, err := namecheap.NewClient(creds, 0).GetDomainInfo(p.DomainName)
		if err != nil {
			return false, fmt.Errorf("sync domain %s: namecheap: %w", p.DomainName, err)
		}
		info = &domainInfoResult{ExpiryDate: r.ExpiryDate, RegistrationDate: r.RegistrationDate}

	case providers.ProviderTypeGoDaddy:
		var creds credentials.GoDaddyCredentials
		if err := s.credEnc.Decrypt(conn.CredentialsEncrypted, &creds); err != nil {
			return false, fmt.Errorf("sync domain %s: decrypt: %w", p.DomainName, err)
		}
		r, err := godaddy.NewClient(creds, 0).GetDomainInfo(p.DomainName)
		if err != nil {
			return false, fmt.Errorf("sync domain %s: godaddy: %w", p.DomainName, err)
		}
		info = &domainInfoResult{ExpiryDate: r.ExpiryDate, RegistrationDate: r.RegistrationDate}

	default:
		return false, nil // Provider doesn't support domain info
	}

	if info == nil || info.ExpiryDate.IsZero() {
		return false, nil
	}

	// Check if expiry date changed.
	changed := p.ExpiryDate == nil || !p.ExpiryDate.Equal(info.ExpiryDate)

	updates := repositories.DomainProfileUpdate{ExpiryDate: &info.ExpiryDate}
	if !info.RegistrationDate.IsZero() {
		updates.RegistrationDate = &info.RegistrationDate
	}
	// Check if domain has expired.
	if info.ExpiryDate.Before(time.Now().UTC()) {
		status := repositories.DomainStatusExpired
		updates.Status = &status
	}

	if _, err := s.profileRepo.Update(ctx, p.ID, updates); err != nil {
		return false, fmt.Errorf("sync domain %s: update: %w", p.DomainName, err)
	}
	return changed, nil
}

type domainInfoResult struct {
	ExpiryDate       time.Time
	RegistrationDate time.Time
}

// validateConnectionRef checks that a connection ID exists (or is empty, which is allowed).
func (s *Service) validateConnectionRef(ctx context.Context, id string) error {
	if id == "" {
		return nil
	}
	_, err := s.providerRepo.GetByID(ctx, id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return fmt.Errorf("provider connection %q not found", id)
		}
		return fmt.Errorf("provider connection lookup failed: %w", err)
	}
	return nil
}

// validateDomainName checks that a domain name is a valid FQDN.
func validateDomainName(name string) error {
	if name == "" {
		return &ValidationError{Field: "domain_name", Message: "required"}
	}
	if !fqdnRE.MatchString(name) {
		return &ValidationError{Field: "domain_name", Message: "must be a valid domain name (e.g. example.com)"}
	}
	return nil
}

// --- DTO converters ---

func toDTO(p repositories.DomainProfile, campaignCount int) DomainProfileDTO {
	dto := DomainProfileDTO{
		ID:                      p.ID,
		DomainName:              p.DomainName,
		RegistrarConnectionID:   p.RegistrarConnectionID,
		DNSProviderConnectionID: p.DNSProviderConnectionID,
		Status:                  string(p.Status),
		Notes:                   p.Notes,
		Tags:                    p.Tags,
		CreatedBy:               p.CreatedBy,
		CreatedAt:               p.CreatedAt.UTC().Format(time.RFC3339),
		UpdatedAt:               p.UpdatedAt.UTC().Format(time.RFC3339),
		CampaignCount:           campaignCount,
	}
	if dto.Tags == nil {
		dto.Tags = []string{}
	}
	if p.RegistrationDate != nil {
		s := p.RegistrationDate.Format("2006-01-02")
		dto.RegistrationDate = &s
	}
	if p.ExpiryDate != nil {
		s := p.ExpiryDate.Format("2006-01-02")
		dto.ExpiryDate = &s
	}
	return dto
}

func toRenewalDTO(r repositories.DomainRenewalRecord) RenewalRecordDTO {
	return RenewalRecordDTO{
		ID:                    r.ID,
		DomainProfileID:       r.DomainProfileID,
		RenewalDate:           r.RenewalDate.Format("2006-01-02"),
		DurationYears:         r.DurationYears,
		CostAmount:            r.CostAmount,
		CostCurrency:          r.CostCurrency,
		RegistrarConnectionID: r.RegistrarConnectionID,
		InitiatedBy:           r.InitiatedBy,
		CreatedAt:             r.CreatedAt.UTC().Format(time.RFC3339),
	}
}

func toRegistrationRequestDTO(r repositories.RegistrationRequest) RegistrationRequestDTO {
	dto := RegistrationRequestDTO{
		ID:                    r.ID,
		DomainName:            r.DomainName,
		RegistrarConnectionID: r.RegistrarConnectionID,
		Years:                 r.Years,
		Status:                r.Status,
		RequestedBy:           r.RequestedBy,
		ReviewedBy:            r.ReviewedBy,
		RejectionReason:       r.RejectionReason,
		DomainProfileID:       r.DomainProfileID,
		CreatedAt:             r.CreatedAt.UTC().Format(time.RFC3339),
	}
	if r.ReviewedAt != nil {
		s := r.ReviewedAt.UTC().Format(time.RFC3339)
		dto.ReviewedAt = &s
	}
	return dto
}

// GetRegistrationRequest returns a registration request DTO by ID.
func (s *Service) GetRegistrationRequest(ctx context.Context, requestID string) (RegistrationRequestDTO, error) {
	req, err := s.profileRepo.GetRegistrationRequest(ctx, requestID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return RegistrationRequestDTO{}, fmt.Errorf("domain: registration request not found")
		}
		return RegistrationRequestDTO{}, fmt.Errorf("domain: get registration request: %w", err)
	}
	return toRegistrationRequestDTO(req), nil
}

func strPtr(s string) *string { return &s }
