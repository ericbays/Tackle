// Package smtpprofile implements business logic for SMTP profile management.
package smtpprofile

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"

	"tackle/internal/providers/credentials"
	"tackle/internal/repositories"
	auditsvc "tackle/internal/services/audit"
	smtptester "tackle/internal/services/smtp"
)

// ValidationError is returned when input fails validation.
type ValidationError struct{ msg string }

// Error implements the error interface.
func (e *ValidationError) Error() string { return e.msg }

// ConflictError is returned when an operation would violate a uniqueness or dependency constraint.
type ConflictError struct{ msg string }

// Error implements the error interface.
func (e *ConflictError) Error() string { return e.msg }

// CreateInput holds the fields needed to create an SMTP profile.
type CreateInput struct {
	Name           string  `json:"name"`
	Description    *string `json:"description,omitempty"`
	Host           string  `json:"host"`
	Port           int     `json:"port"`
	AuthType       string  `json:"auth_type"`
	Username       string  `json:"username,omitempty"`
	Password       string  `json:"password,omitempty"`
	TLSMode        string  `json:"tls_mode"`
	TLSSkipVerify  bool    `json:"tls_skip_verify"`
	FromAddress    string  `json:"from_address"`
	FromName       *string `json:"from_name,omitempty"`
	ReplyTo        *string `json:"reply_to,omitempty"`
	CustomHELO     *string `json:"custom_helo,omitempty"`
	MaxSendRate    *int    `json:"max_send_rate,omitempty"`
	MaxConnections int     `json:"max_connections"`
	TimeoutConnect int     `json:"timeout_connect"`
	TimeoutSend    int     `json:"timeout_send"`
}

// UpdateInput holds the mutable fields for updating an SMTP profile.
type UpdateInput struct {
	Name           *string `json:"name,omitempty"`
	Description    *string `json:"description,omitempty"`
	Host           *string `json:"host,omitempty"`
	Port           *int    `json:"port,omitempty"`
	AuthType       *string `json:"auth_type,omitempty"`
	Username       *string `json:"username,omitempty"` // nil = no change; empty string = clear
	Password       *string `json:"password,omitempty"` // nil = no change; empty string = clear
	TLSMode        *string `json:"tls_mode,omitempty"`
	TLSSkipVerify  *bool   `json:"tls_skip_verify,omitempty"`
	FromAddress    *string `json:"from_address,omitempty"`
	FromName       *string `json:"from_name,omitempty"`
	ReplyTo        *string `json:"reply_to,omitempty"`
	CustomHELO     *string `json:"custom_helo,omitempty"`
	MaxSendRate    *int    `json:"max_send_rate,omitempty"`
	MaxConnections *int    `json:"max_connections,omitempty"`
	TimeoutConnect *int    `json:"timeout_connect,omitempty"`
	TimeoutSend    *int    `json:"timeout_send,omitempty"`
}

// SMTPProfileDTO is the API-safe representation (credentials are masked).
type SMTPProfileDTO struct {
	ID             string  `json:"id"`
	Name           string  `json:"name"`
	Description    string  `json:"description,omitempty"`
	Host           string  `json:"host"`
	Port           int     `json:"port"`
	AuthType       string  `json:"auth_type"`
	HasUsername    bool    `json:"has_username"`
	HasPassword    bool    `json:"has_password"`
	TLSMode        string  `json:"tls_mode"`
	TLSSkipVerify  bool    `json:"tls_skip_verify"`
	FromAddress    string  `json:"from_address"`
	FromName       string  `json:"from_name,omitempty"`
	ReplyTo        string  `json:"reply_to,omitempty"`
	CustomHELO     string  `json:"custom_helo,omitempty"`
	MaxSendRate    *int    `json:"max_send_rate,omitempty"`
	MaxConnections int     `json:"max_connections"`
	TimeoutConnect int     `json:"timeout_connect"`
	TimeoutSend    int     `json:"timeout_send"`
	Status         string  `json:"status"`
	StatusMessage  string  `json:"status_message,omitempty"`
	LastTestedAt   string  `json:"last_tested_at,omitempty"`
	CreatedBy      string  `json:"created_by"`
	CreatedAt      string  `json:"created_at"`
	UpdatedAt      string  `json:"updated_at"`
}

// TestResult is the result of an SMTP connection test.
type TestResult struct {
	Success      bool   `json:"success"`
	StageReached string `json:"stage_reached"`
	TLSVersion   string `json:"tls_version,omitempty"`
	ServerBanner string `json:"server_banner,omitempty"`
	ErrorDetail  string `json:"error_detail,omitempty"`
}

// CampaignAssignInput holds the fields for assigning an SMTP profile to a campaign.
type CampaignAssignInput struct {
	SMTPProfileID       string  `json:"smtp_profile_id"`
	Priority            int     `json:"priority"`
	Weight              int     `json:"weight"`
	FromAddressOverride *string `json:"from_address_override,omitempty"`
	FromNameOverride    *string `json:"from_name_override,omitempty"`
	ReplyToOverride     *string `json:"reply_to_override,omitempty"`
	SegmentFilter       []byte  `json:"segment_filter,omitempty"`
}

// CampaignAssocDTO is the API representation of a campaign-SMTP association.
type CampaignAssocDTO struct {
	ID                  string  `json:"id"`
	CampaignID          string  `json:"campaign_id"`
	SMTPProfileID       string  `json:"smtp_profile_id"`
	Priority            int     `json:"priority"`
	Weight              int     `json:"weight"`
	FromAddressOverride string  `json:"from_address_override,omitempty"`
	FromNameOverride    string  `json:"from_name_override,omitempty"`
	ReplyToOverride     string  `json:"reply_to_override,omitempty"`
	SegmentFilter       string  `json:"segment_filter,omitempty"`
	CreatedAt           string  `json:"created_at"`
}

// SendScheduleInput holds the fields for configuring a campaign's send schedule.
type SendScheduleInput struct {
	SendingStrategy    string  `json:"sending_strategy"`
	SendWindowStart    *string `json:"send_window_start,omitempty"`
	SendWindowEnd      *string `json:"send_window_end,omitempty"`
	SendWindowTimezone *string `json:"send_window_timezone,omitempty"`
	SendWindowDays     []int   `json:"send_window_days,omitempty"`
	CampaignRateLimit  *int    `json:"campaign_rate_limit,omitempty"`
	MinDelayMs         int     `json:"min_delay_ms"`
	MaxDelayMs         int     `json:"max_delay_ms"`
	BatchSize          *int    `json:"batch_size,omitempty"`
	BatchPauseSeconds  *int    `json:"batch_pause_seconds,omitempty"`
}

// SendScheduleDTO is the API representation of a campaign send schedule.
type SendScheduleDTO struct {
	ID                 string  `json:"id"`
	CampaignID         string  `json:"campaign_id"`
	SendingStrategy    string  `json:"sending_strategy"`
	SendWindowStart    string  `json:"send_window_start,omitempty"`
	SendWindowEnd      string  `json:"send_window_end,omitempty"`
	SendWindowTimezone string  `json:"send_window_timezone,omitempty"`
	SendWindowDays     []int   `json:"send_window_days,omitempty"`
	CampaignRateLimit  *int    `json:"campaign_rate_limit,omitempty"`
	MinDelayMs         int     `json:"min_delay_ms"`
	MaxDelayMs         int     `json:"max_delay_ms"`
	BatchSize          *int    `json:"batch_size,omitempty"`
	BatchPauseSeconds  *int    `json:"batch_pause_seconds,omitempty"`
	CreatedAt          string  `json:"created_at"`
	UpdatedAt          string  `json:"updated_at"`
}

// Service implements SMTP profile business logic.
type Service struct {
	repo     *repositories.SMTPProfileRepository
	encSvc   *credentials.SMTPEncryptionService
	tester   *smtptester.ConnectionTester
	auditSvc *auditsvc.AuditService
}

// NewService creates a new SMTP profile Service.
func NewService(
	repo *repositories.SMTPProfileRepository,
	encSvc *credentials.SMTPEncryptionService,
	auditSvc *auditsvc.AuditService,
) *Service {
	return &Service{
		repo:     repo,
		encSvc:   encSvc,
		tester:   smtptester.NewConnectionTester(),
		auditSvc: auditSvc,
	}
}

// CreateProfile validates, encrypts credentials, persists, and audits a new SMTP profile.
func (s *Service) CreateProfile(ctx context.Context, input CreateInput, actorID, actorName, ip, correlationID string) (SMTPProfileDTO, error) {
	if err := validateCreateInput(input); err != nil {
		return SMTPProfileDTO{}, err
	}

	profile := repositories.SMTPProfile{
		Name:           input.Name,
		Description:    input.Description,
		Host:           input.Host,
		Port:           input.Port,
		AuthType:       repositories.SMTPAuthType(input.AuthType),
		TLSMode:        repositories.SMTPTLSMode(input.TLSMode),
		TLSSkipVerify:  input.TLSSkipVerify,
		FromAddress:    input.FromAddress,
		FromName:       input.FromName,
		ReplyTo:        input.ReplyTo,
		CustomHELO:     input.CustomHELO,
		MaxSendRate:    input.MaxSendRate,
		MaxConnections: defaultIfZero(input.MaxConnections, 5),
		TimeoutConnect: defaultIfZero(input.TimeoutConnect, 30),
		TimeoutSend:    defaultIfZero(input.TimeoutSend, 60),
		CreatedBy:      actorID,
	}

	var err error
	if input.Username != "" {
		profile.UsernameEncrypted, err = s.encSvc.Encrypt(input.Username)
		if err != nil {
			return SMTPProfileDTO{}, fmt.Errorf("smtp profile service: encrypt username: %w", err)
		}
	}
	if input.Password != "" {
		profile.PasswordEncrypted, err = s.encSvc.Encrypt(input.Password)
		if err != nil {
			return SMTPProfileDTO{}, fmt.Errorf("smtp profile service: encrypt password: %w", err)
		}
	}

	created, err := s.repo.Create(ctx, profile)
	if err != nil {
		if isUniqueViolation(err) {
			return SMTPProfileDTO{}, &ConflictError{msg: fmt.Sprintf("smtp profile name %q is already in use", input.Name)}
		}
		return SMTPProfileDTO{}, fmt.Errorf("smtp profile service: create: %w", err)
	}

	resourceType := "smtp_profile"
	resourceID := created.ID
	_ = s.auditSvc.Log(ctx, auditsvc.LogEntry{
		Category:      auditsvc.CategoryInfrastructure,
		Severity:      auditsvc.SeverityInfo,
		ActorType:     auditsvc.ActorTypeUser,
		ActorID:       &actorID,
		ActorLabel:    actorName,
		Action:        "smtp_profile.created",
		ResourceType:  &resourceType,
		ResourceID:    &resourceID,
		CorrelationID: correlationID,
		SourceIP:      &ip,
		Details: map[string]any{
			"name":      input.Name,
			"host":      input.Host,
			"port":      input.Port,
			"auth_type": input.AuthType,
		},
	})

	return toDTO(created), nil
}

// GetProfile returns the masked DTO for a single SMTP profile.
func (s *Service) GetProfile(ctx context.Context, id string) (SMTPProfileDTO, error) {
	p, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return SMTPProfileDTO{}, err
	}
	return toDTO(p), nil
}

// ListProfiles returns masked DTOs for all SMTP profiles with optional filtering.
func (s *Service) ListProfiles(ctx context.Context, status, nameSearch string) ([]SMTPProfileDTO, error) {
	profiles, err := s.repo.List(ctx, repositories.SMTPProfileFilters{
		Status:     status,
		NameSearch: nameSearch,
	})
	if err != nil {
		return nil, fmt.Errorf("smtp profile service: list: %w", err)
	}
	dtos := make([]SMTPProfileDTO, 0, len(profiles))
	for _, p := range profiles {
		dtos = append(dtos, toDTO(p))
	}
	return dtos, nil
}

// UpdateProfile validates, re-encrypts credentials if changed, and persists updates.
func (s *Service) UpdateProfile(ctx context.Context, id string, input UpdateInput, actorID, actorName, ip, correlationID string) (SMTPProfileDTO, error) {
	if _, err := s.repo.GetByID(ctx, id); err != nil {
		return SMTPProfileDTO{}, err
	}

	upd := repositories.SMTPProfileUpdate{
		Name:           input.Name,
		Description:    input.Description,
		Host:           input.Host,
		Port:           input.Port,
		TLSSkipVerify:  input.TLSSkipVerify,
		FromAddress:    input.FromAddress,
		FromName:       input.FromName,
		ReplyTo:        input.ReplyTo,
		CustomHELO:     input.CustomHELO,
		MaxSendRate:    input.MaxSendRate,
		MaxConnections: input.MaxConnections,
		TimeoutConnect: input.TimeoutConnect,
		TimeoutSend:    input.TimeoutSend,
	}

	if input.AuthType != nil {
		at := repositories.SMTPAuthType(*input.AuthType)
		upd.AuthType = &at
	}
	if input.TLSMode != nil {
		tm := repositories.SMTPTLSMode(*input.TLSMode)
		upd.TLSMode = &tm
	}

	if input.Username != nil {
		enc, err := s.encSvc.Encrypt(*input.Username)
		if err != nil {
			return SMTPProfileDTO{}, fmt.Errorf("smtp profile service: encrypt username: %w", err)
		}
		upd.UsernameEncrypted = enc
	}
	if input.Password != nil {
		enc, err := s.encSvc.Encrypt(*input.Password)
		if err != nil {
			return SMTPProfileDTO{}, fmt.Errorf("smtp profile service: encrypt password: %w", err)
		}
		upd.PasswordEncrypted = enc
	}

	updated, err := s.repo.Update(ctx, id, upd)
	if err != nil {
		if isUniqueViolation(err) {
			return SMTPProfileDTO{}, &ConflictError{msg: "smtp profile name is already in use"}
		}
		if errors.Is(err, sql.ErrNoRows) {
			return SMTPProfileDTO{}, err
		}
		return SMTPProfileDTO{}, fmt.Errorf("smtp profile service: update: %w", err)
	}

	resourceType := "smtp_profile"
	_ = s.auditSvc.Log(ctx, auditsvc.LogEntry{
		Category:      auditsvc.CategoryInfrastructure,
		Severity:      auditsvc.SeverityInfo,
		ActorType:     auditsvc.ActorTypeUser,
		ActorID:       &actorID,
		ActorLabel:    actorName,
		Action:        "smtp_profile.updated",
		ResourceType:  &resourceType,
		ResourceID:    &id,
		CorrelationID: correlationID,
		SourceIP:      &ip,
		Details: map[string]any{
			"name": updated.Name,
		},
	})

	return toDTO(updated), nil
}

// DeleteProfile deletes an SMTP profile. Blocked if referenced by any campaign.
func (s *Service) DeleteProfile(ctx context.Context, id string, actorID, actorName, ip, correlationID string) error {
	profile, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return err
	}

	assocs, err := s.repo.GetActiveCampaignAssociations(ctx, id)
	if err != nil {
		return fmt.Errorf("smtp profile service: check associations: %w", err)
	}
	if len(assocs) > 0 {
		campaignIDs := make([]string, 0, len(assocs))
		for _, a := range assocs {
			campaignIDs = append(campaignIDs, a.CampaignID)
		}
		resourceType := "smtp_profile"
		_ = s.auditSvc.Log(ctx, auditsvc.LogEntry{
			Category:      auditsvc.CategoryInfrastructure,
			Severity:      auditsvc.SeverityWarning,
			ActorType:     auditsvc.ActorTypeUser,
			ActorID:       &actorID,
			ActorLabel:    actorName,
			Action:        "smtp_profile.delete_blocked",
			ResourceType:  &resourceType,
			ResourceID:    &id,
			CorrelationID: correlationID,
			SourceIP:      &ip,
			Details: map[string]any{
				"name":        profile.Name,
				"campaign_ids": campaignIDs,
			},
		})
		return &ConflictError{msg: fmt.Sprintf("smtp profile is in use by campaigns: %s", strings.Join(campaignIDs, ", "))}
	}

	if err := s.repo.Delete(ctx, id); err != nil {
		return fmt.Errorf("smtp profile service: delete: %w", err)
	}

	resourceType := "smtp_profile"
	_ = s.auditSvc.Log(ctx, auditsvc.LogEntry{
		Category:      auditsvc.CategoryInfrastructure,
		Severity:      auditsvc.SeverityInfo,
		ActorType:     auditsvc.ActorTypeUser,
		ActorID:       &actorID,
		ActorLabel:    actorName,
		Action:        "smtp_profile.deleted",
		ResourceType:  &resourceType,
		ResourceID:    &id,
		CorrelationID: correlationID,
		SourceIP:      &ip,
		Details: map[string]any{
			"name": profile.Name,
		},
	})
	return nil
}

// DuplicateProfile copies all fields except credentials, sets status to untested.
func (s *Service) DuplicateProfile(ctx context.Context, id, newName string, actorID, actorName, ip, correlationID string) (SMTPProfileDTO, error) {
	if newName == "" {
		return SMTPProfileDTO{}, &ValidationError{msg: "new_name is required"}
	}

	src, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return SMTPProfileDTO{}, err
	}

	dup := repositories.SMTPProfile{
		Name:           newName,
		Description:    src.Description,
		Host:           src.Host,
		Port:           src.Port,
		AuthType:       src.AuthType,
		TLSMode:        src.TLSMode,
		TLSSkipVerify:  src.TLSSkipVerify,
		FromAddress:    src.FromAddress,
		FromName:       src.FromName,
		ReplyTo:        src.ReplyTo,
		CustomHELO:     src.CustomHELO,
		MaxSendRate:    src.MaxSendRate,
		MaxConnections: src.MaxConnections,
		TimeoutConnect: src.TimeoutConnect,
		TimeoutSend:    src.TimeoutSend,
		// Credentials intentionally omitted — status will be 'untested'.
		CreatedBy: actorID,
	}

	created, err := s.repo.Create(ctx, dup)
	if err != nil {
		if isUniqueViolation(err) {
			return SMTPProfileDTO{}, &ConflictError{msg: fmt.Sprintf("smtp profile name %q is already in use", newName)}
		}
		return SMTPProfileDTO{}, fmt.Errorf("smtp profile service: duplicate: %w", err)
	}

	resourceType := "smtp_profile"
	createdID := created.ID
	_ = s.auditSvc.Log(ctx, auditsvc.LogEntry{
		Category:      auditsvc.CategoryInfrastructure,
		Severity:      auditsvc.SeverityInfo,
		ActorType:     auditsvc.ActorTypeUser,
		ActorID:       &actorID,
		ActorLabel:    actorName,
		Action:        "smtp_profile.duplicated",
		ResourceType:  &resourceType,
		ResourceID:    &createdID,
		CorrelationID: correlationID,
		SourceIP:      &ip,
		Details: map[string]any{
			"source_name": src.Name,
			"new_name":    newName,
		},
	})

	return toDTO(created), nil
}

// TestProfile decrypts credentials, runs the connection test, updates status, and audits.
func (s *Service) TestProfile(ctx context.Context, id string, actorID, actorName, ip, correlationID string) (TestResult, error) {
	profile, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return TestResult{}, err
	}

	var username, password string
	if len(profile.UsernameEncrypted) > 0 {
		username, err = s.encSvc.Decrypt(profile.UsernameEncrypted)
		if err != nil {
			return TestResult{}, fmt.Errorf("smtp profile service: decrypt username: %w", err)
		}
	}
	if len(profile.PasswordEncrypted) > 0 {
		password, err = s.encSvc.Decrypt(profile.PasswordEncrypted)
		if err != nil {
			return TestResult{}, fmt.Errorf("smtp profile service: decrypt password: %w", err)
		}
	}

	raw := s.tester.Test(ctx, profile, username, password)

	result := TestResult{
		Success:      raw.Success,
		StageReached: string(raw.StageReached),
		TLSVersion:   raw.TLSVersion,
		ServerBanner: raw.ServerBanner,
		ErrorDetail:  raw.ErrorDetail,
	}

	var status repositories.SMTPProfileStatus
	var statusMsg *string
	var auditAction string

	if raw.Success {
		status = repositories.SMTPStatusHealthy
		auditAction = "smtp_profile.test_success"
	} else {
		status = repositories.SMTPStatusError
		msg := raw.ErrorDetail
		statusMsg = &msg
		auditAction = "smtp_profile.test_failure"
	}

	if updErr := s.repo.UpdateStatus(ctx, id, status, statusMsg); updErr != nil {
		return result, fmt.Errorf("smtp profile service: update status: %w", updErr)
	}

	resourceType := "smtp_profile"
	_ = s.auditSvc.Log(ctx, auditsvc.LogEntry{
		Category:      auditsvc.CategoryInfrastructure,
		Severity:      auditsvc.SeverityInfo,
		ActorType:     auditsvc.ActorTypeUser,
		ActorID:       &actorID,
		ActorLabel:    actorName,
		Action:        auditAction,
		ResourceType:  &resourceType,
		ResourceID:    &id,
		CorrelationID: correlationID,
		SourceIP:      &ip,
		Details: map[string]any{
			"name":          profile.Name,
			"host":          profile.Host,
			"tls_version":   raw.TLSVersion,
			"stage_reached": string(raw.StageReached),
			"error_detail":  raw.ErrorDetail,
		},
	})

	return result, nil
}

// --- Campaign SMTP association methods ---

// AssignProfile assigns an SMTP profile to a campaign.
func (s *Service) AssignProfile(ctx context.Context, campaignID string, input CampaignAssignInput, actorID, actorName, ip, correlationID string) (CampaignAssocDTO, error) {
	if input.SMTPProfileID == "" {
		return CampaignAssocDTO{}, &ValidationError{msg: "smtp_profile_id is required"}
	}

	// Verify the profile exists.
	if _, err := s.repo.GetByID(ctx, input.SMTPProfileID); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return CampaignAssocDTO{}, &ValidationError{msg: "smtp profile not found"}
		}
		return CampaignAssocDTO{}, fmt.Errorf("smtp profile service: verify profile: %w", err)
	}

	assoc, err := s.repo.CreateCampaignAssociation(ctx, repositories.CampaignSMTPProfile{
		CampaignID:          campaignID,
		SMTPProfileID:       input.SMTPProfileID,
		Priority:            input.Priority,
		Weight:              input.Weight,
		FromAddressOverride: input.FromAddressOverride,
		FromNameOverride:    input.FromNameOverride,
		ReplyToOverride:     input.ReplyToOverride,
		SegmentFilter:       input.SegmentFilter,
	})
	if err != nil {
		if isUniqueViolation(err) {
			return CampaignAssocDTO{}, &ConflictError{msg: "smtp profile is already assigned to this campaign"}
		}
		return CampaignAssocDTO{}, fmt.Errorf("smtp profile service: assign: %w", err)
	}

	resourceType := "campaign_smtp"
	assocID := assoc.ID
	_ = s.auditSvc.Log(ctx, auditsvc.LogEntry{
		Category:      auditsvc.CategoryInfrastructure,
		Severity:      auditsvc.SeverityInfo,
		ActorType:     auditsvc.ActorTypeUser,
		ActorID:       &actorID,
		ActorLabel:    actorName,
		Action:        "campaign_smtp.assigned",
		ResourceType:  &resourceType,
		ResourceID:    &assocID,
		CorrelationID: correlationID,
		SourceIP:      &ip,
		Details: map[string]any{
			"campaign_id":     campaignID,
			"smtp_profile_id": input.SMTPProfileID,
		},
	})

	return toAssocDTO(assoc), nil
}

// ListCampaignProfiles returns all SMTP associations for a campaign.
func (s *Service) ListCampaignProfiles(ctx context.Context, campaignID string) ([]CampaignAssocDTO, error) {
	assocs, err := s.repo.ListCampaignAssociations(ctx, campaignID)
	if err != nil {
		return nil, fmt.Errorf("smtp profile service: list campaign profiles: %w", err)
	}
	dtos := make([]CampaignAssocDTO, 0, len(assocs))
	for _, a := range assocs {
		dtos = append(dtos, toAssocDTO(a))
	}
	return dtos, nil
}

// UpdateCampaignAssociation updates mutable fields on a campaign-SMTP association.
func (s *Service) UpdateCampaignAssociation(ctx context.Context, assocID string, priority, weight *int, fromAddr, fromName, replyTo *string, segmentFilter []byte, actorID, actorName, ip, correlationID string) (CampaignAssocDTO, error) {
	assoc, err := s.repo.UpdateCampaignAssociation(ctx, assocID, priority, weight, fromAddr, fromName, replyTo, segmentFilter)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return CampaignAssocDTO{}, err
		}
		return CampaignAssocDTO{}, fmt.Errorf("smtp profile service: update association: %w", err)
	}

	resourceType := "campaign_smtp"
	_ = s.auditSvc.Log(ctx, auditsvc.LogEntry{
		Category:      auditsvc.CategoryInfrastructure,
		Severity:      auditsvc.SeverityInfo,
		ActorType:     auditsvc.ActorTypeUser,
		ActorID:       &actorID,
		ActorLabel:    actorName,
		Action:        "campaign_smtp.updated",
		ResourceType:  &resourceType,
		ResourceID:    &assocID,
		CorrelationID: correlationID,
		SourceIP:      &ip,
		Details: map[string]any{
			"campaign_id":     assoc.CampaignID,
			"smtp_profile_id": assoc.SMTPProfileID,
		},
	})

	return toAssocDTO(assoc), nil
}

// RemoveCampaignAssociation removes an SMTP profile from a campaign.
func (s *Service) RemoveCampaignAssociation(ctx context.Context, assocID string, actorID, actorName, ip, correlationID string) error {
	assoc, err := s.repo.GetCampaignAssociation(ctx, assocID)
	if err != nil {
		return err
	}

	if err := s.repo.DeleteCampaignAssociation(ctx, assocID); err != nil {
		return fmt.Errorf("smtp profile service: remove association: %w", err)
	}

	resourceType := "campaign_smtp"
	_ = s.auditSvc.Log(ctx, auditsvc.LogEntry{
		Category:      auditsvc.CategoryInfrastructure,
		Severity:      auditsvc.SeverityInfo,
		ActorType:     auditsvc.ActorTypeUser,
		ActorID:       &actorID,
		ActorLabel:    actorName,
		Action:        "campaign_smtp.removed",
		ResourceType:  &resourceType,
		ResourceID:    &assocID,
		CorrelationID: correlationID,
		SourceIP:      &ip,
		Details: map[string]any{
			"campaign_id":     assoc.CampaignID,
			"smtp_profile_id": assoc.SMTPProfileID,
		},
	})
	return nil
}

// UpsertSendSchedule sets or updates the send schedule for a campaign.
func (s *Service) UpsertSendSchedule(ctx context.Context, campaignID string, input SendScheduleInput, actorID, actorName, ip, correlationID string) (SendScheduleDTO, error) {
	if err := validateStrategy(input.SendingStrategy); err != nil {
		return SendScheduleDTO{}, err
	}

	schedule, err := s.repo.UpsertSendSchedule(ctx, repositories.CampaignSendSchedule{
		CampaignID:         campaignID,
		SendingStrategy:    input.SendingStrategy,
		SendWindowStart:    input.SendWindowStart,
		SendWindowEnd:      input.SendWindowEnd,
		SendWindowTimezone: input.SendWindowTimezone,
		SendWindowDays:     input.SendWindowDays,
		CampaignRateLimit:  input.CampaignRateLimit,
		MinDelayMs:         input.MinDelayMs,
		MaxDelayMs:         input.MaxDelayMs,
		BatchSize:          input.BatchSize,
		BatchPauseSeconds:  input.BatchPauseSeconds,
	})
	if err != nil {
		return SendScheduleDTO{}, fmt.Errorf("smtp profile service: upsert schedule: %w", err)
	}

	resourceType := "campaign_send_schedule"
	resourceID := schedule.ID
	_ = s.auditSvc.Log(ctx, auditsvc.LogEntry{
		Category:      auditsvc.CategoryInfrastructure,
		Severity:      auditsvc.SeverityInfo,
		ActorType:     auditsvc.ActorTypeUser,
		ActorID:       &actorID,
		ActorLabel:    actorName,
		Action:        "campaign_smtp.schedule_updated",
		ResourceType:  &resourceType,
		ResourceID:    &resourceID,
		CorrelationID: correlationID,
		SourceIP:      &ip,
		Details: map[string]any{
			"campaign_id":      campaignID,
			"sending_strategy": input.SendingStrategy,
		},
	})

	return toScheduleDTO(schedule), nil
}

// GetSendSchedule returns the send schedule for a campaign.
func (s *Service) GetSendSchedule(ctx context.Context, campaignID string) (SendScheduleDTO, error) {
	schedule, err := s.repo.GetSendSchedule(ctx, campaignID)
	if err != nil {
		return SendScheduleDTO{}, err
	}
	return toScheduleDTO(schedule), nil
}

// ValidateCampaignProfiles verifies all assigned SMTP profiles are reachable.
func (s *Service) ValidateCampaignProfiles(ctx context.Context, campaignID string) ([]TestResult, error) {
	assocs, err := s.repo.ListCampaignAssociations(ctx, campaignID)
	if err != nil {
		return nil, fmt.Errorf("smtp profile service: validate profiles: %w", err)
	}

	results := make([]TestResult, 0, len(assocs))
	for _, a := range assocs {
		profile, err := s.repo.GetByID(ctx, a.SMTPProfileID)
		if err != nil {
			results = append(results, TestResult{
				Success:     false,
				ErrorDetail: fmt.Sprintf("profile %s not found: %v", a.SMTPProfileID, err),
			})
			continue
		}

		var username, password string
		if len(profile.UsernameEncrypted) > 0 {
			username, err = s.encSvc.Decrypt(profile.UsernameEncrypted)
			if err != nil {
				results = append(results, TestResult{
					Success:     false,
					ErrorDetail: fmt.Sprintf("profile %s: decrypt username failed", a.SMTPProfileID),
				})
				continue
			}
		}
		if len(profile.PasswordEncrypted) > 0 {
			password, err = s.encSvc.Decrypt(profile.PasswordEncrypted)
			if err != nil {
				results = append(results, TestResult{
					Success:     false,
					ErrorDetail: fmt.Sprintf("profile %s: decrypt password failed", a.SMTPProfileID),
				})
				continue
			}
		}

		raw := s.tester.Test(ctx, profile, username, password)
		results = append(results, TestResult{
			Success:      raw.Success,
			StageReached: string(raw.StageReached),
			TLSVersion:   raw.TLSVersion,
			ServerBanner: raw.ServerBanner,
			ErrorDetail:  raw.ErrorDetail,
		})
	}
	return results, nil
}

// --- DTO converters ---

func toDTO(p repositories.SMTPProfile) SMTPProfileDTO {
	dto := SMTPProfileDTO{
		ID:             p.ID,
		Name:           p.Name,
		Host:           p.Host,
		Port:           p.Port,
		AuthType:       string(p.AuthType),
		HasUsername:    len(p.UsernameEncrypted) > 0,
		HasPassword:    len(p.PasswordEncrypted) > 0,
		TLSMode:        string(p.TLSMode),
		TLSSkipVerify:  p.TLSSkipVerify,
		FromAddress:    p.FromAddress,
		MaxConnections: p.MaxConnections,
		TimeoutConnect: p.TimeoutConnect,
		TimeoutSend:    p.TimeoutSend,
		Status:         string(p.Status),
		MaxSendRate:    p.MaxSendRate,
		CreatedBy:      p.CreatedBy,
		CreatedAt:      p.CreatedAt.Format("2006-01-02T15:04:05Z07:00"),
		UpdatedAt:      p.UpdatedAt.Format("2006-01-02T15:04:05Z07:00"),
	}
	if p.Description != nil {
		dto.Description = *p.Description
	}
	if p.FromName != nil {
		dto.FromName = *p.FromName
	}
	if p.ReplyTo != nil {
		dto.ReplyTo = *p.ReplyTo
	}
	if p.CustomHELO != nil {
		dto.CustomHELO = *p.CustomHELO
	}
	if p.StatusMessage != nil {
		dto.StatusMessage = *p.StatusMessage
	}
	if p.LastTestedAt != nil {
		dto.LastTestedAt = p.LastTestedAt.Format("2006-01-02T15:04:05Z07:00")
	}
	return dto
}

func toAssocDTO(a repositories.CampaignSMTPProfile) CampaignAssocDTO {
	dto := CampaignAssocDTO{
		ID:            a.ID,
		CampaignID:    a.CampaignID,
		SMTPProfileID: a.SMTPProfileID,
		Priority:      a.Priority,
		Weight:        a.Weight,
		CreatedAt:     a.CreatedAt.Format("2006-01-02T15:04:05Z07:00"),
	}
	if a.FromAddressOverride != nil {
		dto.FromAddressOverride = *a.FromAddressOverride
	}
	if a.FromNameOverride != nil {
		dto.FromNameOverride = *a.FromNameOverride
	}
	if a.ReplyToOverride != nil {
		dto.ReplyToOverride = *a.ReplyToOverride
	}
	if len(a.SegmentFilter) > 0 {
		dto.SegmentFilter = string(a.SegmentFilter)
	}
	return dto
}

func toScheduleDTO(s repositories.CampaignSendSchedule) SendScheduleDTO {
	dto := SendScheduleDTO{
		ID:                s.ID,
		CampaignID:        s.CampaignID,
		SendingStrategy:   s.SendingStrategy,
		SendWindowDays:    s.SendWindowDays,
		CampaignRateLimit: s.CampaignRateLimit,
		MinDelayMs:        s.MinDelayMs,
		MaxDelayMs:        s.MaxDelayMs,
		BatchSize:         s.BatchSize,
		BatchPauseSeconds: s.BatchPauseSeconds,
		CreatedAt:         s.CreatedAt.Format("2006-01-02T15:04:05Z07:00"),
		UpdatedAt:         s.UpdatedAt.Format("2006-01-02T15:04:05Z07:00"),
	}
	if s.SendWindowStart != nil {
		dto.SendWindowStart = *s.SendWindowStart
	}
	if s.SendWindowEnd != nil {
		dto.SendWindowEnd = *s.SendWindowEnd
	}
	if s.SendWindowTimezone != nil {
		dto.SendWindowTimezone = *s.SendWindowTimezone
	}
	return dto
}

// --- validation helpers ---

func validateCreateInput(input CreateInput) error {
	if input.Name == "" {
		return &ValidationError{msg: "name is required"}
	}
	if input.Host == "" {
		return &ValidationError{msg: "host is required"}
	}
	if input.Port <= 0 || input.Port > 65535 {
		return &ValidationError{msg: "port must be between 1 and 65535"}
	}
	if input.FromAddress == "" {
		return &ValidationError{msg: "from_address is required"}
	}
	if err := validateAuthType(input.AuthType); err != nil {
		return err
	}
	if err := validateTLSMode(input.TLSMode); err != nil {
		return err
	}
	return nil
}

func validateAuthType(s string) error {
	switch repositories.SMTPAuthType(s) {
	case repositories.SMTPAuthNone, repositories.SMTPAuthPlain,
		repositories.SMTPAuthLogin, repositories.SMTPAuthCRAMMD5, repositories.SMTPAuthXOAUTH2:
		return nil
	}
	return &ValidationError{msg: fmt.Sprintf("invalid auth_type %q; must be none, plain, login, cram_md5, or xoauth2", s)}
}

func validateTLSMode(s string) error {
	switch repositories.SMTPTLSMode(s) {
	case repositories.SMTPTLSNone, repositories.SMTPTLSStartTLS, repositories.SMTPTLSTLS:
		return nil
	}
	return &ValidationError{msg: fmt.Sprintf("invalid tls_mode %q; must be none, starttls, or tls", s)}
}

func validateStrategy(s string) error {
	switch s {
	case "round_robin", "random", "weighted", "failover", "segment":
		return nil
	}
	return &ValidationError{msg: fmt.Sprintf("invalid sending_strategy %q", s)}
}

func defaultIfZero(v, def int) int {
	if v == 0 {
		return def
	}
	return v
}

func isUniqueViolation(err error) bool {
	if err == nil {
		return false
	}
	msg := err.Error()
	return strings.Contains(msg, "unique") || strings.Contains(msg, "duplicate") || strings.Contains(msg, "23505")
}
