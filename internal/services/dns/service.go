package dns

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"tackle/internal/providers"
	"tackle/internal/providers/credentials"
	dnsiface "tackle/internal/providers/dns"
	"tackle/internal/repositories"
	auditsvc "tackle/internal/services/audit"
)

// Service implements DNS record management, propagation checking, and email auth.
type Service struct {
	dnsRepo      *repositories.DNSRecordRepository
	profileRepo  *repositories.DomainProfileRepository
	providerRepo *repositories.DomainProviderRepository
	credEnc      *credentials.EncryptionService
	audit        *auditsvc.AuditService
}

// NewService creates a DNS Service.
func NewService(
	dnsRepo *repositories.DNSRecordRepository,
	profileRepo *repositories.DomainProfileRepository,
	providerRepo *repositories.DomainProviderRepository,
	credEnc *credentials.EncryptionService,
	audit *auditsvc.AuditService,
) *Service {
	return &Service{
		dnsRepo:      dnsRepo,
		profileRepo:  profileRepo,
		providerRepo: providerRepo,
		credEnc:      credEnc,
		audit:        audit,
	}
}

// --- DTOs ---

// RecordDTO is the API-safe representation of a DNS record.
type RecordDTO struct {
	ID       string `json:"id"`
	Type     string `json:"type"`
	Name     string `json:"name"`
	Value    string `json:"value"`
	TTL      int    `json:"ttl"`
	Priority int    `json:"priority,omitempty"`
}

// PropagationCheckDTO is the API-safe representation of a propagation check result.
type PropagationCheckDTO struct {
	ID              string                                `json:"id"`
	DomainProfileID string                                `json:"domain_profile_id"`
	RecordType      string                                `json:"record_type"`
	RecordName      string                                `json:"record_name"`
	ExpectedValue   string                                `json:"expected_value"`
	OverallStatus   string                                `json:"overall_status"`
	Results         []repositories.PropagationCheckResult `json:"results"`
	CheckedAt       string                                `json:"checked_at"`
}

// EmailAuthStatusDTO is the API representation of email auth panel status.
type EmailAuthStatusDTO struct {
	DomainProfileID string          `json:"domain_profile_id"`
	SPFStatus       string          `json:"spf_status"`
	DKIMStatus      string          `json:"dkim_status"`
	DMARCStatus     string          `json:"dmarc_status"`
	LastCheckedAt   *string         `json:"last_checked_at,omitempty"`
	Details         json.RawMessage `json:"details"`
}

// --- DNS record CRUD ---

// ListRecords fetches DNS records from the provider and returns them.
func (s *Service) ListRecords(ctx context.Context, domainProfileID string) ([]RecordDTO, error) {
	provider, zone, err := s.resolveProvider(ctx, domainProfileID)
	if err != nil {
		return nil, err
	}

	records, err := provider.ListRecords(ctx, zone)
	if err != nil {
		return nil, fmt.Errorf("dns service: list records: %w", err)
	}

	// Sync results to local cache (best-effort).
	now := time.Now()
	for _, r := range records {
		rec := repositories.DNSRecord{
			DomainProfileID: domainProfileID,
			RecordType:      string(r.Type),
			RecordName:      r.Name,
			RecordValue:     r.Value,
			TTL:             r.TTL,
			Priority:        r.Priority,
			SyncedAt:        &now,
		}
		if r.ID != "" {
			rec.ProviderRecordID = strPtr(r.ID)
		}
		_, _ = s.dnsRepo.UpsertRecord(ctx, rec)
	}

	return recordsToDTO(records), nil
}

// CreateRecord validates, creates a DNS record, emits an audit event, and triggers propagation check.
func (s *Service) CreateRecord(ctx context.Context, domainProfileID string, record dnsiface.Record, actorID, actorLabel, sourceIP, correlationID string) (RecordDTO, error) {
	if result := ValidateRecord(record); !result.OK() {
		return RecordDTO{}, result.Errors[0]
	}

	provider, zone, err := s.resolveProvider(ctx, domainProfileID)
	if err != nil {
		return RecordDTO{}, err
	}

	created, err := provider.CreateRecord(ctx, zone, record)
	if err != nil {
		return RecordDTO{}, fmt.Errorf("dns service: create record: %w", err)
	}

	_ = s.audit.Log(ctx, auditsvc.LogEntry{
		Category: auditsvc.CategoryInfrastructure, Severity: auditsvc.SeverityInfo,
		ActorType: auditsvc.ActorTypeUser, ActorID: &actorID, ActorLabel: actorLabel,
		Action: "dns.record_created", ResourceType: strPtr("dns_record"),
		CorrelationID: correlationID, SourceIP: &sourceIP,
		Details: map[string]any{
			"domain_profile_id": domainProfileID,
			"record_type":       string(created.Type),
			"record_name":       created.Name,
			"record_value":      created.Value,
			"ttl":               created.TTL,
		},
	})

	go s.asyncPropagationCheck(context.Background(), domainProfileID, zone, created, actorID, actorLabel, sourceIP, correlationID)

	return recordToDTO(created), nil
}

// UpdateRecord validates, updates a DNS record, emits an audit event with before/after values.
func (s *Service) UpdateRecord(ctx context.Context, domainProfileID, recordID string, record dnsiface.Record, actorID, actorLabel, sourceIP, correlationID string) (RecordDTO, error) {
	if result := ValidateRecord(record); !result.OK() {
		return RecordDTO{}, result.Errors[0]
	}

	provider, zone, err := s.resolveProvider(ctx, domainProfileID)
	if err != nil {
		return RecordDTO{}, err
	}

	// Fetch before value for audit.
	var before *dnsiface.Record
	if current, listErr := provider.ListRecords(ctx, zone); listErr == nil {
		for _, r := range current {
			if r.ID == recordID {
				cp := r
				before = &cp
				break
			}
		}
	}

	updated, err := provider.UpdateRecord(ctx, zone, recordID, record)
	if err != nil {
		return RecordDTO{}, fmt.Errorf("dns service: update record: %w", err)
	}

	details := map[string]any{
		"domain_profile_id": domainProfileID,
		"record_id":         recordID,
		"after": map[string]any{
			"record_type":  string(updated.Type),
			"record_name":  updated.Name,
			"record_value": updated.Value,
			"ttl":          updated.TTL,
		},
	}
	if before != nil {
		details["before"] = map[string]any{
			"record_type":  string(before.Type),
			"record_name":  before.Name,
			"record_value": before.Value,
			"ttl":          before.TTL,
		}
	}

	_ = s.audit.Log(ctx, auditsvc.LogEntry{
		Category: auditsvc.CategoryInfrastructure, Severity: auditsvc.SeverityInfo,
		ActorType: auditsvc.ActorTypeUser, ActorID: &actorID, ActorLabel: actorLabel,
		Action: "dns.record_updated", ResourceType: strPtr("dns_record"),
		CorrelationID: correlationID, SourceIP: &sourceIP,
		Details: details,
	})

	go s.asyncPropagationCheck(context.Background(), domainProfileID, zone, updated, actorID, actorLabel, sourceIP, correlationID)

	return recordToDTO(updated), nil
}

// DeleteRecord removes a DNS record and emits an audit event.
func (s *Service) DeleteRecord(ctx context.Context, domainProfileID, recordID string, actorID, actorLabel, sourceIP, correlationID string) error {
	provider, zone, err := s.resolveProvider(ctx, domainProfileID)
	if err != nil {
		return err
	}

	// Fetch before value for audit.
	var before *dnsiface.Record
	if current, listErr := provider.ListRecords(ctx, zone); listErr == nil {
		for _, r := range current {
			if r.ID == recordID {
				cp := r
				before = &cp
				break
			}
		}
	}

	if err := provider.DeleteRecord(ctx, zone, recordID); err != nil {
		return fmt.Errorf("dns service: delete record: %w", err)
	}

	details := map[string]any{
		"domain_profile_id": domainProfileID,
		"record_id":         recordID,
	}
	if before != nil {
		details["before"] = map[string]any{
			"record_type":  string(before.Type),
			"record_name":  before.Name,
			"record_value": before.Value,
			"ttl":          before.TTL,
		}
	}

	_ = s.audit.Log(ctx, auditsvc.LogEntry{
		Category: auditsvc.CategoryInfrastructure, Severity: auditsvc.SeverityInfo,
		ActorType: auditsvc.ActorTypeUser, ActorID: &actorID, ActorLabel: actorLabel,
		Action: "dns.record_deleted", ResourceType: strPtr("dns_record"),
		CorrelationID: correlationID, SourceIP: &sourceIP,
		Details: details,
	})

	return nil
}

// GetSOA returns the SOA record for the domain.
func (s *Service) GetSOA(ctx context.Context, domainProfileID string) (RecordDTO, error) {
	provider, zone, err := s.resolveProvider(ctx, domainProfileID)
	if err != nil {
		return RecordDTO{}, err
	}
	soa, err := provider.GetSOA(ctx, zone)
	if err != nil {
		return RecordDTO{}, fmt.Errorf("dns service: get soa: %w", err)
	}
	return recordToDTO(soa), nil
}

// --- Email auth ---

// ConfigureSPF builds and publishes an SPF TXT record at the domain apex.
func (s *Service) ConfigureSPF(ctx context.Context, domainProfileID string, cfg SPFConfig, actorID, actorLabel, sourceIP, correlationID string) (RecordDTO, error) {
	recordValue, _, err := BuildSPFRecord(cfg)
	if err != nil {
		return RecordDTO{}, fmt.Errorf("dns service: build spf: %w", err)
	}

	record := dnsiface.Record{
		Type:  dnsiface.RecordTypeTXT,
		Name:  "@",
		Value: recordValue,
		TTL:   300,
	}

	dto, err := s.CreateRecord(ctx, domainProfileID, record, actorID, actorLabel, sourceIP, correlationID)
	if err != nil {
		return RecordDTO{}, fmt.Errorf("dns service: publish spf: %w", err)
	}

	_ = s.audit.Log(ctx, auditsvc.LogEntry{
		Category: auditsvc.CategoryInfrastructure, Severity: auditsvc.SeverityInfo,
		ActorType: auditsvc.ActorTypeUser, ActorID: &actorID, ActorLabel: actorLabel,
		Action: "dns.spf_published", ResourceType: strPtr("domain"),
		CorrelationID: correlationID, SourceIP: &sourceIP,
		Details: map[string]any{
			"domain_profile_id": domainProfileID,
			"spf_record":        recordValue,
		},
	})

	go s.triggerEmailAuthValidation(context.Background(), domainProfileID)
	return dto, nil
}

// GenerateDKIM generates a DKIM key pair, stores it encrypted, and publishes the DNS TXT record.
func (s *Service) GenerateDKIM(ctx context.Context, domainProfileID, selector string, algorithm DKIMAlgorithm, keySize int, actorID, actorLabel, sourceIP, correlationID string) (RecordDTO, error) {
	profile, err := s.profileRepo.GetByID(ctx, domainProfileID)
	if err != nil {
		return RecordDTO{}, fmt.Errorf("dns service: get profile: %w", err)
	}

	kp, err := GenerateDKIMKeyPair(algorithm, keySize)
	if err != nil {
		return RecordDTO{}, fmt.Errorf("dns service: generate dkim key: %w", err)
	}

	// Encrypt private key PEM before storage.
	encryptedPriv, err := s.credEnc.Encrypt(kp.PrivateKeyPEM)
	if err != nil {
		return RecordDTO{}, fmt.Errorf("dns service: encrypt dkim private key: %w", err)
	}

	dbKey := repositories.DKIMKey{
		DomainProfileID:     domainProfileID,
		Selector:            selector,
		Algorithm:           string(algorithm),
		PrivateKeyEncrypted: encryptedPriv,
		PublicKey:           kp.PublicKeyBase64,
	}
	if kp.KeySize > 0 {
		ks := kp.KeySize
		dbKey.KeySize = &ks
	}
	if _, err := s.dnsRepo.CreateDKIMKey(ctx, dbKey); err != nil {
		return RecordDTO{}, fmt.Errorf("dns service: store dkim key: %w", err)
	}

	_, recordValue, err := BuildDKIMRecord(profile.DomainName, selector, kp)
	if err != nil {
		return RecordDTO{}, fmt.Errorf("dns service: build dkim record: %w", err)
	}

	record := dnsiface.Record{
		Type:  dnsiface.RecordTypeTXT,
		Name:  selector + "._domainkey",
		Value: recordValue,
		TTL:   300,
	}

	dto, err := s.CreateRecord(ctx, domainProfileID, record, actorID, actorLabel, sourceIP, correlationID)
	if err != nil {
		return RecordDTO{}, fmt.Errorf("dns service: publish dkim: %w", err)
	}

	_ = s.audit.Log(ctx, auditsvc.LogEntry{
		Category: auditsvc.CategoryInfrastructure, Severity: auditsvc.SeverityInfo,
		ActorType: auditsvc.ActorTypeUser, ActorID: &actorID, ActorLabel: actorLabel,
		Action: "dns.dkim_published", ResourceType: strPtr("domain"),
		CorrelationID: correlationID, SourceIP: &sourceIP,
		Details: map[string]any{
			"domain_profile_id": domainProfileID,
			"selector":          selector,
			// Private key is NOT logged.
		},
	})

	go s.triggerEmailAuthValidation(context.Background(), domainProfileID)
	return dto, nil
}

// ConfigureDMARC builds and publishes a DMARC TXT record.
func (s *Service) ConfigureDMARC(ctx context.Context, domainProfileID string, cfg DMARCConfig, actorID, actorLabel, sourceIP, correlationID string) (RecordDTO, error) {
	profile, err := s.profileRepo.GetByID(ctx, domainProfileID)
	if err != nil {
		return RecordDTO{}, fmt.Errorf("dns service: get profile: %w", err)
	}

	_, recordValue, err := BuildDMARCRecord(profile.DomainName, cfg)
	if err != nil {
		return RecordDTO{}, fmt.Errorf("dns service: build dmarc: %w", err)
	}

	record := dnsiface.Record{
		Type:  dnsiface.RecordTypeTXT,
		Name:  "_dmarc",
		Value: recordValue,
		TTL:   300,
	}

	dto, err := s.CreateRecord(ctx, domainProfileID, record, actorID, actorLabel, sourceIP, correlationID)
	if err != nil {
		return RecordDTO{}, fmt.Errorf("dns service: publish dmarc: %w", err)
	}

	_ = s.audit.Log(ctx, auditsvc.LogEntry{
		Category: auditsvc.CategoryInfrastructure, Severity: auditsvc.SeverityInfo,
		ActorType: auditsvc.ActorTypeUser, ActorID: &actorID, ActorLabel: actorLabel,
		Action: "dns.dmarc_published", ResourceType: strPtr("domain"),
		CorrelationID: correlationID, SourceIP: &sourceIP,
		Details: map[string]any{
			"domain_profile_id": domainProfileID,
			"dmarc_record":      recordValue,
		},
	})

	go s.triggerEmailAuthValidation(context.Background(), domainProfileID)
	return dto, nil
}

// GetEmailAuthStatus returns the stored email auth panel status for a domain.
func (s *Service) GetEmailAuthStatus(ctx context.Context, domainProfileID string) (EmailAuthStatusDTO, error) {
	status, err := s.dnsRepo.GetEmailAuthStatus(ctx, domainProfileID)
	if err != nil {
		// Return blank "missing" status if not yet computed.
		return EmailAuthStatusDTO{
			DomainProfileID: domainProfileID,
			SPFStatus:       "missing",
			DKIMStatus:      "missing",
			DMARCStatus:     "missing",
			Details:         json.RawMessage("{}"),
		}, nil
	}

	dto := EmailAuthStatusDTO{
		DomainProfileID: domainProfileID,
		SPFStatus:       status.SPFStatus,
		DKIMStatus:      status.DKIMStatus,
		DMARCStatus:     status.DMARCStatus,
		Details:         json.RawMessage(status.DetailsJSON),
	}
	if status.LastCheckedAt != nil {
		t := status.LastCheckedAt.UTC().Format(time.RFC3339)
		dto.LastCheckedAt = &t
	}
	return dto, nil
}

// ValidateEmailAuth queries public DNS and updates the stored email auth status.
func (s *Service) ValidateEmailAuth(ctx context.Context, domainProfileID, actorID, actorLabel, sourceIP, correlationID string) (EmailAuthStatusDTO, error) {
	profile, err := s.profileRepo.GetByID(ctx, domainProfileID)
	if err != nil {
		return EmailAuthStatusDTO{}, fmt.Errorf("dns service: get profile: %w", err)
	}

	type mechanismDetail struct {
		Status   string `json:"status"`
		Expected string `json:"expected,omitempty"`
		Actual   string `json:"actual,omitempty"`
		Diff     string `json:"diff,omitempty"`
	}

	// SPF: check for v=spf1 at the apex.
	spfResult, _ := CheckPropagation(ctx, profile.DomainName, dnsiface.RecordTypeTXT, "v=spf1")
	spfStatus := emailAuthMechanismStatus(spfResult)
	spfDetail := mechanismDetail{Status: spfStatus}
	if len(spfResult.Results) > 0 {
		spfDetail.Actual = spfResult.Results[0].Response
	}

	// DMARC: check _dmarc.<domain>.
	dmarcResult, _ := CheckPropagation(ctx, "_dmarc."+profile.DomainName, dnsiface.RecordTypeTXT, "v=DMARC1")
	dmarcStatus := emailAuthMechanismStatus(dmarcResult)
	dmarcDetail := mechanismDetail{Status: dmarcStatus}
	if len(dmarcResult.Results) > 0 {
		dmarcDetail.Actual = dmarcResult.Results[0].Response
	}

	// DKIM: check all known selectors.
	dkimKeys, _ := s.dnsRepo.ListDKIMKeys(ctx, domainProfileID)
	dkimStatus := "missing"
	if len(dkimKeys) > 0 {
		anyConfigured := false
		for _, k := range dkimKeys {
			dkimDomain := k.Selector + "._domainkey." + profile.DomainName
			dkimResult, _ := CheckPropagation(ctx, dkimDomain, dnsiface.RecordTypeTXT, "v=DKIM1")
			if dkimResult.OverallStatus == PropagationStatusPropagated {
				anyConfigured = true
				break
			}
		}
		if anyConfigured {
			dkimStatus = "configured"
		} else {
			dkimStatus = "misconfigured"
		}
	}

	details := map[string]any{
		"spf":   spfDetail,
		"dkim":  mechanismDetail{Status: dkimStatus},
		"dmarc": dmarcDetail,
	}
	detailsJSON, _ := json.Marshal(details)

	now := time.Now().UTC()
	_, _ = s.dnsRepo.UpsertEmailAuthStatus(ctx, repositories.EmailAuthStatus{
		DomainProfileID: domainProfileID,
		SPFStatus:       spfStatus,
		DKIMStatus:      dkimStatus,
		DMARCStatus:     dmarcStatus,
		LastCheckedAt:   &now,
		DetailsJSON:     detailsJSON,
	})

	_ = s.audit.Log(ctx, auditsvc.LogEntry{
		Category: auditsvc.CategoryInfrastructure, Severity: auditsvc.SeverityInfo,
		ActorType: auditsvc.ActorTypeUser, ActorID: &actorID, ActorLabel: actorLabel,
		Action: "dns.validation_completed", ResourceType: strPtr("domain"),
		CorrelationID: correlationID, SourceIP: &sourceIP,
		Details: map[string]any{
			"domain_profile_id": domainProfileID,
			"spf_status":        spfStatus,
			"dkim_status":       dkimStatus,
			"dmarc_status":      dmarcStatus,
		},
	})

	nowStr := now.Format(time.RFC3339)
	return EmailAuthStatusDTO{
		DomainProfileID: domainProfileID,
		SPFStatus:       spfStatus,
		DKIMStatus:      dkimStatus,
		DMARCStatus:     dmarcStatus,
		LastCheckedAt:   &nowStr,
		Details:         json.RawMessage(detailsJSON),
	}, nil
}

// GetPropagationChecks returns recent propagation check results for a domain.
func (s *Service) GetPropagationChecks(ctx context.Context, domainProfileID string, limit int) ([]PropagationCheckDTO, error) {
	checks, err := s.dnsRepo.ListPropagationChecks(ctx, domainProfileID, limit)
	if err != nil {
		return nil, fmt.Errorf("dns service: list propagation checks: %w", err)
	}

	out := make([]PropagationCheckDTO, 0, len(checks))
	for _, c := range checks {
		out = append(out, PropagationCheckDTO{
			ID:              c.ID,
			DomainProfileID: c.DomainProfileID,
			RecordType:      c.RecordType,
			RecordName:      c.RecordName,
			ExpectedValue:   c.ExpectedValue,
			OverallStatus:   c.OverallStatus,
			Results:         c.Results,
			CheckedAt:       c.CheckedAt.UTC().Format(time.RFC3339),
		})
	}
	return out, nil
}

// --- internal helpers ---

func (s *Service) resolveProvider(ctx context.Context, domainProfileID string) (dnsiface.Provider, string, error) {
	profile, err := s.profileRepo.GetByID(ctx, domainProfileID)
	if err != nil {
		return nil, "", fmt.Errorf("dns service: get profile: %w", err)
	}

	connID := profile.DNSProviderConnectionID
	if connID == nil {
		connID = profile.RegistrarConnectionID
	}
	if connID == nil {
		return nil, "", fmt.Errorf("dns service: domain %q has no DNS provider connection configured", profile.DomainName)
	}

	conn, err := s.providerRepo.GetByID(ctx, *connID)
	if err != nil {
		return nil, "", fmt.Errorf("dns service: get provider connection: %w", err)
	}

	provider, err := providers.BuildDNSProviderFromEncrypted(conn.ProviderType, conn.CredentialsEncrypted, s.credEnc, 0)
	if err != nil {
		return nil, "", fmt.Errorf("dns service: build provider: %w", err)
	}

	return provider, profile.DomainName, nil
}

// asyncPropagationCheck runs a propagation check in the background and stores the result.
func (s *Service) asyncPropagationCheck(ctx context.Context, domainProfileID, zone string, record dnsiface.Record, actorID, actorLabel, sourceIP, correlationID string) {
	result, err := CheckPropagation(ctx, zone, record.Type, record.Value)
	if err != nil {
		return
	}

	repoResults := make([]repositories.PropagationCheckResult, 0, len(result.Results))
	for _, r := range result.Results {
		repoResults = append(repoResults, repositories.PropagationCheckResult{
			Resolver:  r.Resolver,
			Response:  r.Response,
			Matches:   r.Matches,
			LatencyMs: r.LatencyMs,
		})
	}

	_, _ = s.dnsRepo.CreatePropagationCheck(ctx, repositories.PropagationCheck{
		DomainProfileID: domainProfileID,
		RecordType:      string(record.Type),
		RecordName:      record.Name,
		ExpectedValue:   record.Value,
		OverallStatus:   string(result.OverallStatus),
		Results:         repoResults,
		CheckedAt:       time.Now().UTC(),
	})

	_ = s.audit.Log(ctx, auditsvc.LogEntry{
		Category: auditsvc.CategoryInfrastructure, Severity: auditsvc.SeverityInfo,
		ActorType: auditsvc.ActorTypeUser, ActorID: &actorID, ActorLabel: actorLabel,
		Action: "dns.propagation_check", ResourceType: strPtr("dns_record"),
		CorrelationID: correlationID, SourceIP: &sourceIP,
		Details: map[string]any{
			"domain_profile_id": domainProfileID,
			"record_type":       string(record.Type),
			"overall_status":    string(result.OverallStatus),
		},
	})
}

// triggerEmailAuthValidation re-validates email auth in the background after a change.
func (s *Service) triggerEmailAuthValidation(ctx context.Context, domainProfileID string) {
	_, _ = s.ValidateEmailAuth(ctx, domainProfileID, "system", "background-validator", "", "")
}

// emailAuthMechanismStatus maps a PropagationResult to configured/misconfigured/missing.
func emailAuthMechanismStatus(result PropagationResult) string {
	switch result.OverallStatus {
	case PropagationStatusPropagated:
		return "configured"
	case PropagationStatusPartial:
		return "misconfigured"
	default:
		return "missing"
	}
}

// recordToDTO converts a provider record to a DTO.
func recordToDTO(r dnsiface.Record) RecordDTO {
	return RecordDTO{
		ID:       r.ID,
		Type:     string(r.Type),
		Name:     r.Name,
		Value:    r.Value,
		TTL:      r.TTL,
		Priority: r.Priority,
	}
}

// recordsToDTO converts a slice of provider records to DTOs.
func recordsToDTO(records []dnsiface.Record) []RecordDTO {
	out := make([]RecordDTO, 0, len(records))
	for _, r := range records {
		out = append(out, recordToDTO(r))
	}
	return out
}

func strPtr(s string) *string { return &s }
