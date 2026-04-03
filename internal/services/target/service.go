// Package target implements business logic for target management.
package target

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net/mail"
	"regexp"
	"strings"
	"time"

	"tackle/internal/repositories"
	auditsvc "tackle/internal/services/audit"
)

// ValidationError is returned when input fails validation.
type ValidationError struct{ Msg string }

// Error implements the error interface.
func (e *ValidationError) Error() string { return e.Msg }

// ConflictError is returned when a uniqueness constraint would be violated.
type ConflictError struct{ Msg string }

// Error implements the error interface.
func (e *ConflictError) Error() string { return e.Msg }

// ForbiddenError is returned when the caller lacks permission.
type ForbiddenError struct{ Msg string }

// Error implements the error interface.
func (e *ForbiddenError) Error() string { return e.Msg }

const (
	maxCustomFieldKeys     = 50
	maxCustomFieldKeyLen   = 64
	maxCustomFieldValueLen = 1024
)

// secretPatterns detects values that look like secrets.
var secretPatterns = []*regexp.Regexp{
	regexp.MustCompile(`(?i)(api[_-]?key|secret|password|token|bearer)\s*[:=]\s*\S+`),
	regexp.MustCompile(`(?i)^(sk|pk|ak|rk)[-_][a-zA-Z0-9]{16,}`),
	regexp.MustCompile(`(?i)^eyJ[a-zA-Z0-9_-]+\.[a-zA-Z0-9_-]+`), // JWT
}

// CreateInput holds fields for creating a target.
type CreateInput struct {
	Email        string         `json:"email"`
	FirstName    *string        `json:"first_name,omitempty"`
	LastName     *string        `json:"last_name,omitempty"`
	Department   *string        `json:"department,omitempty"`
	Title        *string        `json:"title,omitempty"`
	CustomFields map[string]any `json:"custom_fields,omitempty"`
}

// UpdateInput holds mutable fields for an update.
type UpdateInput struct {
	Email        *string        `json:"email,omitempty"`
	FirstName    *string        `json:"first_name,omitempty"`
	LastName     *string        `json:"last_name,omitempty"`
	Department   *string        `json:"department,omitempty"`
	Title        *string        `json:"title,omitempty"`
	CustomFields map[string]any `json:"custom_fields,omitempty"`
}

// TargetDTO is the API-safe representation of a target.
type TargetDTO struct {
	ID           string         `json:"id"`
	Email        string         `json:"email"`
	FirstName    *string        `json:"first_name,omitempty"`
	LastName     *string        `json:"last_name,omitempty"`
	Department   *string        `json:"department,omitempty"`
	Title        *string        `json:"title,omitempty"`
	CustomFields map[string]any `json:"custom_fields"`
	CreatedBy    string         `json:"created_by"`
	DeletedAt    *string        `json:"deleted_at,omitempty"`
	CreatedAt    string         `json:"created_at"`
	UpdatedAt    string         `json:"updated_at"`
}

// BulkEditInput holds fields for a bulk edit operation.
type BulkEditInput struct {
	TargetIDs []string `json:"target_ids"`
	Field     string   `json:"field"`
	Value     string   `json:"value"`
}

// BulkDeleteInput holds fields for a bulk delete operation.
type BulkDeleteInput struct {
	TargetIDs []string `json:"target_ids"`
	Confirm   bool     `json:"confirm"`
}

// BulkExportInput holds fields for a bulk export operation.
type BulkExportInput struct {
	TargetIDs []string `json:"target_ids,omitempty"`
}

// CampaignHistoryDTO represents a target's participation in a campaign.
type CampaignHistoryDTO struct {
	CampaignID string  `json:"campaign_id"`
	Status     string  `json:"status"`
	Reported   bool    `json:"reported"`
	AssignedAt string  `json:"assigned_at"`
	RemovedAt  *string `json:"removed_at,omitempty"`
}

// TimelineEventDTO represents a single event in a target's timeline.
type TimelineEventDTO struct {
	ID        string         `json:"id"`
	EventType string         `json:"event_type"`
	EventData map[string]any `json:"event_data"`
	IPAddress *string        `json:"ip_address,omitempty"`
	UserAgent *string        `json:"user_agent,omitempty"`
	CreatedAt string         `json:"created_at"`
}

// BlocklistChecker checks whether an email is blocked by the blocklist.
type BlocklistChecker interface {
	CheckEmail(ctx context.Context, email string) (BlocklistCheckResult, error)
}

// BlocklistCheckResult is a simplified result from a blocklist check.
type BlocklistCheckResult struct {
	Blocked bool   `json:"blocked"`
	Pattern string `json:"pattern,omitempty"`
}

// Service implements target management business logic.
type Service struct {
	repo       *repositories.TargetRepository
	importRepo *repositories.TargetImportRepository
	mappingRepo *repositories.MappingTemplateRepository
	eventRepo  *repositories.CampaignTargetEventRepository
	auditSvc   *auditsvc.AuditService
	blocklist  BlocklistChecker
}

// NewService creates a new target Service.
func NewService(
	repo *repositories.TargetRepository,
	importRepo *repositories.TargetImportRepository,
	mappingRepo *repositories.MappingTemplateRepository,
	eventRepo *repositories.CampaignTargetEventRepository,
	auditSvc *auditsvc.AuditService,
) *Service {
	return &Service{
		repo:       repo,
		importRepo: importRepo,
		mappingRepo: mappingRepo,
		eventRepo:  eventRepo,
		auditSvc:   auditSvc,
	}
}

// SetBlocklistChecker sets the blocklist checker for import validation.
func (s *Service) SetBlocklistChecker(bl BlocklistChecker) {
	s.blocklist = bl
}

// Create validates and persists a new target.
func (s *Service) Create(ctx context.Context, input CreateInput, actorID, actorName, ip, correlationID string) (TargetDTO, error) {
	if err := validateEmail(input.Email); err != nil {
		return TargetDTO{}, err
	}
	if err := validateCustomFields(input.CustomFields); err != nil {
		return TargetDTO{}, err
	}

	email := strings.ToLower(strings.TrimSpace(input.Email))

	cf := input.CustomFields
	if cf == nil {
		cf = map[string]any{}
	}

	t := repositories.Target{
		Email:        email,
		FirstName:    input.FirstName,
		LastName:     input.LastName,
		Department:   input.Department,
		Title:        input.Title,
		CustomFields: cf,
		CreatedBy:    actorID,
	}

	created, err := s.repo.Create(ctx, t)
	if err != nil {
		if isUniqueViolation(err) {
			return TargetDTO{}, &ConflictError{Msg: fmt.Sprintf("target with email %q already exists", email)}
		}
		return TargetDTO{}, fmt.Errorf("target service: create: %w", err)
	}

	resourceType := "target"
	resourceID := created.ID
	_ = s.auditSvc.Log(ctx, auditsvc.LogEntry{
		Category:      auditsvc.CategoryUserActivity,
		Severity:      auditsvc.SeverityInfo,
		ActorType:     auditsvc.ActorTypeUser,
		ActorID:       &actorID,
		ActorLabel:    actorName,
		Action:        "target.created",
		ResourceType:  &resourceType,
		ResourceID:    &resourceID,
		CorrelationID: correlationID,
		SourceIP:      &ip,
		Details: map[string]any{
			"email": maskEmail(email),
		},
	})

	return toDTO(created), nil
}

// Get returns the DTO for a single target.
func (s *Service) Get(ctx context.Context, id string) (TargetDTO, error) {
	t, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return TargetDTO{}, err
	}
	return toDTO(t), nil
}

// List returns DTOs for targets with optional filtering and pagination.
func (s *Service) List(ctx context.Context, filters repositories.TargetFilters) ([]TargetDTO, int, error) {
	result, err := s.repo.List(ctx, filters)
	if err != nil {
		return nil, 0, fmt.Errorf("target service: list: %w", err)
	}
	dtos := make([]TargetDTO, 0, len(result.Targets))
	for _, t := range result.Targets {
		dtos = append(dtos, toDTO(t))
	}
	return dtos, result.Total, nil
}

// ListCursor returns DTOs for targets using cursor-based pagination.
func (s *Service) ListCursor(ctx context.Context, filters repositories.TargetFilters) ([]TargetDTO, bool, error) {
	result, err := s.repo.ListCursor(ctx, filters)
	if err != nil {
		return nil, false, fmt.Errorf("target service: list cursor: %w", err)
	}
	dtos := make([]TargetDTO, 0, len(result.Targets))
	for _, t := range result.Targets {
		dtos = append(dtos, toDTO(t))
	}
	return dtos, result.HasMore, nil
}

// Update applies changes to a target.
func (s *Service) Update(ctx context.Context, id string, input UpdateInput, actorID, actorName, ip, correlationID string) (TargetDTO, error) {
	if input.Email != nil {
		if err := validateEmail(*input.Email); err != nil {
			return TargetDTO{}, err
		}
		normalized := strings.ToLower(strings.TrimSpace(*input.Email))
		input.Email = &normalized
	}
	if input.CustomFields != nil {
		if err := validateCustomFields(input.CustomFields); err != nil {
			return TargetDTO{}, err
		}
	}

	upd := repositories.TargetUpdate{
		Email:        input.Email,
		FirstName:    input.FirstName,
		LastName:     input.LastName,
		Department:   input.Department,
		Title:        input.Title,
		CustomFields: input.CustomFields,
	}

	updated, err := s.repo.Update(ctx, id, upd)
	if err != nil {
		if isUniqueViolation(err) {
			return TargetDTO{}, &ConflictError{Msg: "target with this email already exists"}
		}
		if errors.Is(err, sql.ErrNoRows) {
			return TargetDTO{}, err
		}
		return TargetDTO{}, fmt.Errorf("target service: update: %w", err)
	}

	resourceType := "target"
	_ = s.auditSvc.Log(ctx, auditsvc.LogEntry{
		Category:      auditsvc.CategoryUserActivity,
		Severity:      auditsvc.SeverityInfo,
		ActorType:     auditsvc.ActorTypeUser,
		ActorID:       &actorID,
		ActorLabel:    actorName,
		Action:        "target.updated",
		ResourceType:  &resourceType,
		ResourceID:    &id,
		CorrelationID: correlationID,
		SourceIP:      &ip,
		Details: map[string]any{
			"email": maskEmail(updated.Email),
		},
	})

	return toDTO(updated), nil
}

// SoftDelete marks a target as deleted.
func (s *Service) SoftDelete(ctx context.Context, id, actorID, actorName, ip, correlationID string) error {
	if err := s.repo.SoftDelete(ctx, id); err != nil {
		return err
	}

	resourceType := "target"
	_ = s.auditSvc.Log(ctx, auditsvc.LogEntry{
		Category:      auditsvc.CategoryUserActivity,
		Severity:      auditsvc.SeverityInfo,
		ActorType:     auditsvc.ActorTypeUser,
		ActorID:       &actorID,
		ActorLabel:    actorName,
		Action:        "target.soft_deleted",
		ResourceType:  &resourceType,
		ResourceID:    &id,
		CorrelationID: correlationID,
		SourceIP:      &ip,
	})
	return nil
}

// Restore reactivates a soft-deleted target. Admin only (enforced at handler layer via RBAC).
func (s *Service) Restore(ctx context.Context, id, actorID, actorName, ip, correlationID string) error {
	// Check the target exists and is deleted.
	t, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return err
	}
	if t.DeletedAt == nil {
		return &ValidationError{Msg: "target is not deleted"}
	}

	if err := s.repo.Restore(ctx, id); err != nil {
		return err
	}

	resourceType := "target"
	_ = s.auditSvc.Log(ctx, auditsvc.LogEntry{
		Category:      auditsvc.CategoryUserActivity,
		Severity:      auditsvc.SeverityInfo,
		ActorType:     auditsvc.ActorTypeUser,
		ActorID:       &actorID,
		ActorLabel:    actorName,
		Action:        "target.restored",
		ResourceType:  &resourceType,
		ResourceID:    &id,
		CorrelationID: correlationID,
		SourceIP:      &ip,
	})
	return nil
}

// CheckEmail returns whether an active target with the given email exists.
func (s *Service) CheckEmail(ctx context.Context, email string) (string, bool, error) {
	return s.repo.CheckEmailExists(ctx, email)
}

// GetDepartments returns distinct department values for filter dropdowns.
func (s *Service) GetDepartments(ctx context.Context) ([]string, error) {
	return s.repo.GetDepartments(ctx)
}

// GetHistory returns cross-campaign history for a target.
func (s *Service) GetHistory(ctx context.Context, targetID string) ([]CampaignHistoryDTO, error) {
	cts, err := s.eventRepo.GetCrossTargetHistory(ctx, targetID)
	if err != nil {
		return nil, fmt.Errorf("target service: get history: %w", err)
	}
	dtos := make([]CampaignHistoryDTO, 0, len(cts))
	for _, ct := range cts {
		dto := CampaignHistoryDTO{
			CampaignID: ct.CampaignID,
			Status:     ct.Status,
			Reported:   ct.Reported,
			AssignedAt: ct.AssignedAt.Format(time.RFC3339),
		}
		if ct.RemovedAt != nil {
			s := ct.RemovedAt.Format(time.RFC3339)
			dto.RemovedAt = &s
		}
		dtos = append(dtos, dto)
	}
	return dtos, nil
}

// GetTimeline returns the event timeline for a target in a specific campaign.
func (s *Service) GetTimeline(ctx context.Context, campaignID, targetID, eventType string, includeSensitive bool) ([]TimelineEventDTO, error) {
	events, err := s.eventRepo.GetTimeline(ctx, campaignID, targetID, eventType)
	if err != nil {
		return nil, fmt.Errorf("target service: get timeline: %w", err)
	}
	dtos := make([]TimelineEventDTO, 0, len(events))
	for _, evt := range events {
		dto := TimelineEventDTO{
			ID:        evt.ID,
			EventType: evt.EventType,
			EventData: evt.EventData,
			CreatedAt: evt.CreatedAt.Format(time.RFC3339),
		}
		// Strip credential values from event data — show field names only.
		if evt.EventType == "credential_submitted" {
			sanitized := map[string]any{}
			if ed, ok := evt.EventData["fields"]; ok {
				if fields, ok := ed.([]any); ok {
					names := make([]string, 0, len(fields))
					for _, f := range fields {
						if fm, ok := f.(map[string]any); ok {
							if name, ok := fm["name"].(string); ok {
								names = append(names, name)
							}
						}
					}
					sanitized["field_names"] = names
				}
			}
			dto.EventData = sanitized
		}
		// Only include IP and user-agent for authorized roles.
		if includeSensitive {
			dto.IPAddress = evt.IPAddress
			dto.UserAgent = evt.UserAgent
		}
		dtos = append(dtos, dto)
	}
	return dtos, nil
}

// CrossCampaignEventDTO represents an event from any campaign for a target.
type CrossCampaignEventDTO struct {
	ID         string         `json:"id"`
	CampaignID string         `json:"campaign_id"`
	EventType  string         `json:"event_type"`
	EventData  map[string]any `json:"event_data"`
	IPAddress  *string        `json:"ip_address,omitempty"`
	UserAgent  *string        `json:"user_agent,omitempty"`
	CreatedAt  string         `json:"created_at"`
}

// TargetStatsDTO holds summary statistics for a target.
type TargetStatsDTO struct {
	TotalCampaigns int     `json:"total_campaigns"`
	OpenRate       float64 `json:"open_rate"`
	ClickRate      float64 `json:"click_rate"`
	SubmitRate     float64 `json:"submit_rate"`
}

// GetEvents returns cross-campaign events for a target.
func (s *Service) GetEvents(ctx context.Context, targetID string, limit, offset int, includeSensitive bool) ([]CrossCampaignEventDTO, int, error) {
	events, total, err := s.eventRepo.GetCrossTargetEvents(ctx, targetID, limit, offset)
	if err != nil {
		return nil, 0, fmt.Errorf("target service: get events: %w", err)
	}
	dtos := make([]CrossCampaignEventDTO, 0, len(events))
	for _, evt := range events {
		dto := CrossCampaignEventDTO{
			ID:         evt.ID,
			CampaignID: evt.CampaignID,
			EventType:  evt.EventType,
			EventData:  evt.EventData,
			CreatedAt:  evt.CreatedAt.Format(time.RFC3339),
		}
		if evt.EventType == "credential_submitted" {
			sanitized := map[string]any{}
			if ed, ok := evt.EventData["fields"]; ok {
				if fields, ok := ed.([]any); ok {
					names := make([]string, 0, len(fields))
					for _, f := range fields {
						if fm, ok := f.(map[string]any); ok {
							if name, ok := fm["name"].(string); ok {
								names = append(names, name)
							}
						}
					}
					sanitized["field_names"] = names
				}
			}
			dto.EventData = sanitized
		}
		if includeSensitive {
			dto.IPAddress = evt.IPAddress
			dto.UserAgent = evt.UserAgent
		}
		dtos = append(dtos, dto)
	}
	return dtos, total, nil
}

// GetStats returns summary statistics for a target across all campaigns.
func (s *Service) GetStats(ctx context.Context, targetID string) (TargetStatsDTO, error) {
	cts, err := s.eventRepo.GetCrossTargetHistory(ctx, targetID)
	if err != nil {
		return TargetStatsDTO{}, fmt.Errorf("target service: get stats: %w", err)
	}
	stats := TargetStatsDTO{TotalCampaigns: len(cts)}
	if len(cts) == 0 {
		return stats, nil
	}
	var opened, clicked, submitted int
	for _, ct := range cts {
		switch ct.Status {
		case "email_opened":
			opened++
		case "link_clicked":
			opened++
			clicked++
		case "credential_submitted":
			opened++
			clicked++
			submitted++
		}
	}
	n := float64(len(cts))
	stats.OpenRate = float64(opened) / n
	stats.ClickRate = float64(clicked) / n
	stats.SubmitRate = float64(submitted) / n
	return stats, nil
}

// PurgeExpired permanently removes PII from targets soft-deleted before the retention cutoff.
func (s *Service) PurgeExpired(ctx context.Context, retentionDays int) (int64, error) {
	if retentionDays <= 0 {
		retentionDays = 365
	}
	cutoff := time.Now().UTC().AddDate(0, 0, -retentionDays)
	count, err := s.repo.PurgeExpired(ctx, cutoff)
	if err != nil {
		return 0, fmt.Errorf("target service: purge expired: %w", err)
	}
	if count > 0 {
		_ = s.auditSvc.Log(ctx, auditsvc.LogEntry{
			Category:  auditsvc.CategoryUserActivity,
			Severity:  auditsvc.SeverityWarning,
			ActorType: auditsvc.ActorTypeSystem,
			Action:    "target.purge_expired",
			Details: map[string]any{
				"purged_count":   count,
				"retention_days": retentionDays,
			},
		})
	}
	return count, nil
}

// BulkDelete soft-deletes multiple targets.
func (s *Service) BulkDelete(ctx context.Context, input BulkDeleteInput, actorID, actorName, ip, correlationID string) (int64, error) {
	if !input.Confirm {
		return 0, &ValidationError{Msg: "bulk delete requires confirm: true"}
	}
	if len(input.TargetIDs) == 0 {
		return 0, &ValidationError{Msg: "no target IDs provided"}
	}

	count, err := s.repo.BulkSoftDelete(ctx, input.TargetIDs)
	if err != nil {
		return 0, fmt.Errorf("target service: bulk delete: %w", err)
	}

	resourceType := "target"
	_ = s.auditSvc.Log(ctx, auditsvc.LogEntry{
		Category:      auditsvc.CategoryUserActivity,
		Severity:      auditsvc.SeverityWarning,
		ActorType:     auditsvc.ActorTypeUser,
		ActorID:       &actorID,
		ActorLabel:    actorName,
		Action:        "target.bulk_deleted",
		ResourceType:  &resourceType,
		CorrelationID: correlationID,
		SourceIP:      &ip,
		Details: map[string]any{
			"requested_count": len(input.TargetIDs),
			"deleted_count":   count,
		},
	})

	return count, nil
}

// BulkEdit sets a single field value for multiple targets.
func (s *Service) BulkEdit(ctx context.Context, input BulkEditInput, actorID, actorName, ip, correlationID string) (int64, error) {
	if len(input.TargetIDs) == 0 {
		return 0, &ValidationError{Msg: "no target IDs provided"}
	}

	count, err := s.repo.BulkUpdateField(ctx, input.TargetIDs, input.Field, input.Value)
	if err != nil {
		return 0, fmt.Errorf("target service: bulk edit: %w", err)
	}

	resourceType := "target"
	_ = s.auditSvc.Log(ctx, auditsvc.LogEntry{
		Category:      auditsvc.CategoryUserActivity,
		Severity:      auditsvc.SeverityInfo,
		ActorType:     auditsvc.ActorTypeUser,
		ActorID:       &actorID,
		ActorLabel:    actorName,
		Action:        "target.bulk_edited",
		ResourceType:  &resourceType,
		CorrelationID: correlationID,
		SourceIP:      &ip,
		Details: map[string]any{
			"field":          input.Field,
			"target_count":   len(input.TargetIDs),
			"updated_count":  count,
		},
	})

	return count, nil
}

// BulkExport retrieves targets for CSV export.
func (s *Service) BulkExport(ctx context.Context, ids []string) ([]TargetDTO, error) {
	if len(ids) == 0 {
		// Export all active targets.
		result, err := s.repo.List(ctx, repositories.TargetFilters{PerPage: 100})
		if err != nil {
			return nil, fmt.Errorf("target service: bulk export: %w", err)
		}
		dtos := make([]TargetDTO, 0, len(result.Targets))
		for _, t := range result.Targets {
			dtos = append(dtos, toDTO(t))
		}
		return dtos, nil
	}
	// Export specific targets.
	dtos := make([]TargetDTO, 0, len(ids))
	for _, id := range ids {
		t, err := s.repo.GetByID(ctx, id)
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				continue
			}
			return nil, fmt.Errorf("target service: bulk export: %w", err)
		}
		dtos = append(dtos, toDTO(t))
	}
	return dtos, nil
}

// --- validation helpers ---

func validateEmail(email string) error {
	email = strings.TrimSpace(email)
	if email == "" {
		return &ValidationError{Msg: "email is required"}
	}
	if _, err := mail.ParseAddress(email); err != nil {
		return &ValidationError{Msg: fmt.Sprintf("invalid email address: %s", err.Error())}
	}
	// Additional length check.
	if len(email) > 320 {
		return &ValidationError{Msg: "email exceeds maximum length of 320 characters"}
	}
	return nil
}

func validateCustomFields(cf map[string]any) error {
	if cf == nil {
		return nil
	}
	if len(cf) > maxCustomFieldKeys {
		return &ValidationError{Msg: fmt.Sprintf("custom_fields exceeds maximum of %d keys", maxCustomFieldKeys)}
	}
	for k, v := range cf {
		if len(k) > maxCustomFieldKeyLen {
			return &ValidationError{Msg: fmt.Sprintf("custom field key %q exceeds maximum length of %d", k, maxCustomFieldKeyLen)}
		}
		valStr := fmt.Sprintf("%v", v)
		if len(valStr) > maxCustomFieldValueLen {
			return &ValidationError{Msg: fmt.Sprintf("custom field value for key %q exceeds maximum length of %d", k, maxCustomFieldValueLen)}
		}
		// Warn about secret-like values (return error to prevent storage).
		for _, pat := range secretPatterns {
			if pat.MatchString(valStr) {
				return &ValidationError{Msg: fmt.Sprintf("custom field %q appears to contain secret material; do not store credentials in target custom fields", k)}
			}
		}
	}
	return nil
}

// maskEmail masks the local part of an email for audit logs.
func maskEmail(email string) string {
	parts := strings.SplitN(email, "@", 2)
	if len(parts) != 2 {
		return "***"
	}
	local := parts[0]
	if len(local) <= 1 {
		return local + "**@" + parts[1]
	}
	return string(local[0]) + "**@" + parts[1]
}

func isUniqueViolation(err error) bool {
	return err != nil && strings.Contains(err.Error(), "duplicate key value violates unique constraint")
}

func toDTO(t repositories.Target) TargetDTO {
	dto := TargetDTO{
		ID:           t.ID,
		Email:        t.Email,
		FirstName:    t.FirstName,
		LastName:     t.LastName,
		Department:   t.Department,
		Title:        t.Title,
		CustomFields: t.CustomFields,
		CreatedBy:    t.CreatedBy,
		CreatedAt:    t.CreatedAt.Format(time.RFC3339),
		UpdatedAt:    t.UpdatedAt.Format(time.RFC3339),
	}
	if t.DeletedAt != nil {
		s := t.DeletedAt.Format(time.RFC3339)
		dto.DeletedAt = &s
	}
	if dto.CustomFields == nil {
		dto.CustomFields = map[string]any{}
	}
	return dto
}

// marshalJSON is a helper to convert an object to JSON bytes.
func marshalJSON(v any) []byte {
	b, _ := json.Marshal(v)
	return b
}
