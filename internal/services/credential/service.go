// Package credential implements the credential capture business logic layer.
package credential

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"tackle/internal/crypto"
	"tackle/internal/repositories"
	auditsvc "tackle/internal/services/audit"
	notifsvc "tackle/internal/services/notification"
)


const (
	// PurposeCredentialEncryption is the HKDF purpose for credential field encryption.
	PurposeCredentialEncryption = "tackle/credential-encryption"
)

// Service provides credential capture business logic.
type Service struct {
	db        *sql.DB
	repo      *repositories.CaptureEventRepository
	encSvc    *crypto.EncryptionService
	auditSvc  *auditsvc.AuditService
	notifSvc  *notifsvc.NotificationService
	campRepo  *repositories.CampaignRepository
}

// NewService creates a new credential capture service.
func NewService(
	db *sql.DB,
	repo *repositories.CaptureEventRepository,
	encSvc *crypto.EncryptionService,
	auditSvc *auditsvc.AuditService,
	notifSvc *notifsvc.NotificationService,
	campRepo *repositories.CampaignRepository,
) *Service {
	return &Service{
		db:       db,
		repo:     repo,
		encSvc:   encSvc,
		auditSvc: auditSvc,
		notifSvc: notifSvc,
		campRepo: campRepo,
	}
}

// CaptureInput contains the data received from a landing page form submission.
type CaptureInput struct {
	CampaignID        string
	TrackingToken     string
	TargetID          *string
	TemplateVariantID *string
	EndpointID        *string
	Fields            map[string]string
	SourceIP          string
	UserAgent         string
	AcceptLanguage    string
	Referer           string
	URLPath           string
	HTTPMethod        string
	IsCanary          bool
	SessionData       []SessionDataInput
}

// SessionDataInput represents session artifact data from the landing page.
type SessionDataInput struct {
	DataType        repositories.SessionDataType
	Key             string
	Value           string
	Metadata        map[string]any
	IsTimeSensitive bool
}

// CaptureEventDTO is the API representation of a capture event.
type CaptureEventDTO struct {
	ID                string    `json:"id"`
	CampaignID        string    `json:"campaign_id"`
	TargetID          *string   `json:"target_id"`
	TemplateVariantID *string   `json:"template_variant_id,omitempty"`
	EndpointID        *string   `json:"endpoint_id,omitempty"`
	TrackingToken     *string   `json:"tracking_token,omitempty"`
	SourceIP          *string   `json:"source_ip,omitempty"`
	UserAgent         *string   `json:"user_agent,omitempty"`
	HTTPMethod        string    `json:"http_method"`
	SubmissionSeq     int       `json:"submission_sequence"`
	IsUnattributed    bool      `json:"is_unattributed"`
	IsCanary          bool      `json:"is_canary"`
	CapturedAt        time.Time `json:"captured_at"`
	FieldsCaptured    []string  `json:"fields_captured"`
	FieldCategories   map[string]string `json:"field_categories,omitempty"`
}

// CaptureFieldDTO is the API representation of a capture field.
type CaptureFieldDTO struct {
	FieldName  string `json:"field_name"`
	FieldValue string `json:"field_value"` // Masked or revealed.
	Category   string `json:"category"`
}

// RevealedCapture contains decrypted capture data.
type RevealedCapture struct {
	EventID string            `json:"event_id"`
	Fields  []CaptureFieldDTO `json:"fields"`
}

// ListInput controls filtering and pagination for capture event lists.
type ListInput struct {
	CampaignID     string
	TargetID       string
	DateAfter      *time.Time
	DateBefore     *time.Time
	IsUnattributed *bool
	Page           int
	PerPage        int
}

// Capture processes and stores a credential capture from a landing page app.
func (s *Service) Capture(ctx context.Context, input CaptureInput) (CaptureEventDTO, error) {
	// Determine target attribution.
	targetID := input.TargetID
	isUnattributed := targetID == nil || *targetID == ""
	if isUnattributed {
		targetID = nil
	}

	// Get next submission sequence for this target+campaign.
	seq, err := s.repo.NextSubmissionSequence(ctx, input.CampaignID, targetID)
	if err != nil {
		return CaptureEventDTO{}, fmt.Errorf("capture: %w", err)
	}

	capturedAt := time.Now().UTC()

	// Create the capture event.
	event, err := s.repo.CreateEvent(ctx, repositories.CaptureEvent{
		CampaignID:        input.CampaignID,
		TargetID:          targetID,
		TemplateVariantID: input.TemplateVariantID,
		EndpointID:        input.EndpointID,
		TrackingToken:     strPtr(input.TrackingToken),
		SourceIP:          strPtrNonEmpty(input.SourceIP),
		UserAgent:         strPtrNonEmpty(input.UserAgent),
		AcceptLanguage:    strPtrNonEmpty(input.AcceptLanguage),
		Referer:           strPtrNonEmpty(input.Referer),
		URLPath:           strPtrNonEmpty(input.URLPath),
		HTTPMethod:        defaultStr(input.HTTPMethod, "POST"),
		SubmissionSeq:     seq,
		IsUnattributed:    isUnattributed,
		IsCanary:          input.IsCanary,
		CapturedAt:        capturedAt,
	})
	if err != nil {
		return CaptureEventDTO{}, fmt.Errorf("capture: create event: %w", err)
	}

	// Get categorization rules for field classification.
	rules, _ := s.repo.ListCategorizationRules(ctx, nil) // global rules

	// Encrypt and store each field.
	fieldNames := make([]string, 0, len(input.Fields))
	fieldCategories := make(map[string]string, len(input.Fields))
	for name, value := range input.Fields {
		cat := categorizeField(name, rules)

		// Encrypt the field value. The EncryptionService prepends a random nonce.
		encrypted, err := s.encSvc.EncryptString(value)
		if err != nil {
			return CaptureEventDTO{}, fmt.Errorf("capture: encrypt field %q: %w", name, err)
		}

		// The IV is embedded in the encrypted bytes (first 12 bytes), but we also
		// store it separately for the schema contract. Extract the nonce.
		iv := encrypted[:12]

		_, err = s.repo.CreateField(ctx, repositories.CaptureField{
			CaptureEventID:       event.ID,
			FieldName:            name,
			FieldValueEncrypted:  encrypted,
			FieldCategory:        cat,
			EncryptionKeyVersion: 1,
			IV:                   iv,
		})
		if err != nil {
			return CaptureEventDTO{}, fmt.Errorf("capture: store field %q: %w", name, err)
		}
		fieldNames = append(fieldNames, name)
		fieldCategories[name] = string(cat)
	}

	// Store session data if provided.
	for _, sd := range input.SessionData {
		keyEnc, err := s.encSvc.EncryptString(sd.Key)
		if err != nil {
			return CaptureEventDTO{}, fmt.Errorf("capture: encrypt session key: %w", err)
		}
		valEnc, err := s.encSvc.EncryptString(sd.Value)
		if err != nil {
			return CaptureEventDTO{}, fmt.Errorf("capture: encrypt session value: %w", err)
		}
		var metaJSON json.RawMessage
		if sd.Metadata != nil {
			metaJSON, _ = json.Marshal(sd.Metadata)
		}
		_, err = s.repo.CreateSessionCapture(ctx, repositories.SessionCapture{
			CaptureEventID:  event.ID,
			DataType:        sd.DataType,
			KeyEncrypted:    keyEnc,
			ValueEncrypted:  valEnc,
			Metadata:        metaJSON,
			CapturedAt:      capturedAt,
			IsTimeSensitive: sd.IsTimeSensitive,
		})
		if err != nil {
			return CaptureEventDTO{}, fmt.Errorf("capture: store session data: %w", err)
		}
	}

	// Audit log.
	sysActor := "landing-page-app"
	resType := "capture_event"
	_ = s.auditSvc.Log(ctx, auditsvc.LogEntry{
		Timestamp:    time.Now().UTC(),
		Category:     auditsvc.CategoryCredentialCapture,
		Severity:     auditsvc.SeverityInfo,
		ActorType:    auditsvc.ActorTypeSystem,
		ActorID:      &sysActor,
		ActorLabel:   "landing-page-app",
		Action:       "credential_submitted",
		ResourceType: &resType,
		ResourceID:   &event.ID,
		Details: map[string]any{
			"campaign_id": event.CampaignID,
			"target_id":   event.TargetID,
			"field_count": len(input.Fields),
			"sequence":    seq,
		},
	})

	// Send real-time notification (within 3 seconds).
	s.sendCaptureNotification(ctx, event, fieldNames)

	return CaptureEventDTO{
		ID:                event.ID,
		CampaignID:        event.CampaignID,
		TargetID:          event.TargetID,
		TemplateVariantID: event.TemplateVariantID,
		EndpointID:        event.EndpointID,
		TrackingToken:     event.TrackingToken,
		SourceIP:          event.SourceIP,
		UserAgent:         event.UserAgent,
		HTTPMethod:        event.HTTPMethod,
		SubmissionSeq:     event.SubmissionSeq,
		IsUnattributed:    event.IsUnattributed,
		IsCanary:          event.IsCanary,
		CapturedAt:        event.CapturedAt,
		FieldsCaptured:    fieldNames,
		FieldCategories:   fieldCategories,
	}, nil
}

// Get returns a capture event by ID with masked field information.
func (s *Service) Get(ctx context.Context, id string) (CaptureEventDTO, error) {
	event, err := s.repo.GetEvent(ctx, id)
	if err != nil {
		return CaptureEventDTO{}, fmt.Errorf("get capture: %w", err)
	}

	fields, err := s.repo.ListFieldsByEvent(ctx, id)
	if err != nil {
		return CaptureEventDTO{}, fmt.Errorf("get capture fields: %w", err)
	}

	fieldNames := make([]string, len(fields))
	fieldCategories := make(map[string]string, len(fields))
	for i, f := range fields {
		fieldNames[i] = f.FieldName
		fieldCategories[f.FieldName] = string(f.FieldCategory)
	}

	return CaptureEventDTO{
		ID:                event.ID,
		CampaignID:        event.CampaignID,
		TargetID:          event.TargetID,
		TemplateVariantID: event.TemplateVariantID,
		EndpointID:        event.EndpointID,
		TrackingToken:     event.TrackingToken,
		SourceIP:          event.SourceIP,
		UserAgent:         event.UserAgent,
		HTTPMethod:        event.HTTPMethod,
		SubmissionSeq:     event.SubmissionSeq,
		IsUnattributed:    event.IsUnattributed,
		IsCanary:          event.IsCanary,
		CapturedAt:        event.CapturedAt,
		FieldsCaptured:    fieldNames,
		FieldCategories:   fieldCategories,
	}, nil
}

// List returns paginated capture events.
func (s *Service) List(ctx context.Context, input ListInput) ([]CaptureEventDTO, int, error) {
	events, total, err := s.repo.ListEvents(ctx, repositories.CaptureEventFilters{
		CampaignID:     input.CampaignID,
		TargetID:       input.TargetID,
		DateAfter:      input.DateAfter,
		DateBefore:     input.DateBefore,
		IsUnattributed: input.IsUnattributed,
		Page:           input.Page,
		PerPage:        input.PerPage,
	})
	if err != nil {
		return nil, 0, fmt.Errorf("list captures: %w", err)
	}

	dtos := make([]CaptureEventDTO, len(events))
	for i, e := range events {
		// Get field names for each event.
		fields, _ := s.repo.ListFieldsByEvent(ctx, e.ID)
		fieldNames := make([]string, len(fields))
		fieldCategories := make(map[string]string, len(fields))
		for j, f := range fields {
			fieldNames[j] = f.FieldName
			fieldCategories[f.FieldName] = string(f.FieldCategory)
		}

		dtos[i] = CaptureEventDTO{
			ID:                e.ID,
			CampaignID:        e.CampaignID,
			TargetID:          e.TargetID,
			TemplateVariantID: e.TemplateVariantID,
			EndpointID:        e.EndpointID,
			TrackingToken:     e.TrackingToken,
			SourceIP:          e.SourceIP,
			UserAgent:         e.UserAgent,
			HTTPMethod:        e.HTTPMethod,
			SubmissionSeq:     e.SubmissionSeq,
			IsUnattributed:    e.IsUnattributed,
			IsCanary:          e.IsCanary,
			CapturedAt:        e.CapturedAt,
			FieldsCaptured:    fieldNames,
			FieldCategories:   fieldCategories,
		}
	}
	return dtos, total, nil
}

// Reveal decrypts all field values for a capture event. Requires credentials:reveal permission.
// Creates an audit log entry.
func (s *Service) Reveal(ctx context.Context, eventID string, userID, username, clientIP, correlationID string) (RevealedCapture, error) {
	fields, err := s.repo.ListFieldsByEvent(ctx, eventID)
	if err != nil {
		return RevealedCapture{}, fmt.Errorf("reveal: list fields: %w", err)
	}

	result := RevealedCapture{
		EventID: eventID,
		Fields:  make([]CaptureFieldDTO, len(fields)),
	}

	for i, f := range fields {
		plaintext, err := s.encSvc.DecryptString(f.FieldValueEncrypted)
		if err != nil {
			return RevealedCapture{}, fmt.Errorf("reveal: decrypt field %q: %w", f.FieldName, err)
		}
		result.Fields[i] = CaptureFieldDTO{
			FieldName:  f.FieldName,
			FieldValue: plaintext,
			Category:   string(f.FieldCategory),
		}
	}

	// Audit log the reveal action.
	revealResType := "capture_event"
	_ = s.auditSvc.Log(ctx, auditsvc.LogEntry{
		Timestamp:     time.Now().UTC(),
		Category:      auditsvc.CategoryCredentialCapture,
		Severity:      auditsvc.SeverityWarning,
		ActorType:     auditsvc.ActorTypeUser,
		ActorID:       &userID,
		ActorLabel:    username,
		Action:        "credential_reveal",
		ResourceType:  &revealResType,
		ResourceID:    &eventID,
		CorrelationID: correlationID,
		SourceIP:      &clientIP,
		Details: map[string]any{
			"field_count": len(fields),
		},
	})

	return result, nil
}

// Delete removes a single capture event. Returns error if campaign is archived.
func (s *Service) Delete(ctx context.Context, eventID string, userID, username, clientIP, correlationID string) error {
	// Check if campaign is archived.
	event, err := s.repo.GetEvent(ctx, eventID)
	if err != nil {
		return fmt.Errorf("delete capture: %w", err)
	}

	if err := s.checkNotArchived(ctx, event.CampaignID); err != nil {
		return err
	}

	ok, err := s.repo.DeleteEvent(ctx, eventID)
	if err != nil {
		return fmt.Errorf("delete capture: %w", err)
	}
	if !ok {
		return &NotFoundError{Msg: "capture event not found"}
	}

	delResType := "capture_event"
	_ = s.auditSvc.Log(ctx, auditsvc.LogEntry{
		Timestamp:     time.Now().UTC(),
		Category:      auditsvc.CategoryCredentialCapture,
		Severity:      auditsvc.SeverityWarning,
		ActorType:     auditsvc.ActorTypeUser,
		ActorID:       &userID,
		ActorLabel:    username,
		Action:        "credential_delete",
		ResourceType:  &delResType,
		ResourceID:    &eventID,
		CorrelationID: correlationID,
		SourceIP:      &clientIP,
		Details: map[string]any{
			"campaign_id": event.CampaignID,
		},
	})

	return nil
}

// PurgeInput controls bulk deletion of capture events.
type PurgeInput struct {
	CampaignID   string
	DateAfter    *time.Time
	DateBefore   *time.Time
	Confirmation string
}

// Purge bulk-deletes capture events matching filters. Requires "PURGE" confirmation.
func (s *Service) Purge(ctx context.Context, input PurgeInput, userID, username, clientIP, correlationID string) (int, error) {
	if input.Confirmation != "PURGE" {
		return 0, &ValidationError{Msg: "confirmation must be the literal string \"PURGE\""}
	}

	if input.CampaignID != "" {
		if err := s.checkNotArchived(ctx, input.CampaignID); err != nil {
			return 0, err
		}
	}

	var count int
	var err error

	if input.CampaignID != "" && input.DateAfter == nil && input.DateBefore == nil {
		count, err = s.repo.PurgeByCampaign(ctx, input.CampaignID)
	} else if input.DateAfter != nil && input.DateBefore != nil {
		count, err = s.repo.PurgeByDateRange(ctx, *input.DateAfter, *input.DateBefore)
	} else {
		return 0, &ValidationError{Msg: "purge requires either campaign_id or both date_after and date_before"}
	}

	if err != nil {
		return 0, fmt.Errorf("purge captures: %w", err)
	}

	purgeResType := "capture_event"
	_ = s.auditSvc.Log(ctx, auditsvc.LogEntry{
		Timestamp:     time.Now().UTC(),
		Category:      auditsvc.CategoryCredentialCapture,
		Severity:      auditsvc.SeverityCritical,
		ActorType:     auditsvc.ActorTypeUser,
		ActorID:       &userID,
		ActorLabel:    username,
		Action:        "credential_purge",
		ResourceType:  &purgeResType,
		CorrelationID: correlationID,
		SourceIP:      &clientIP,
		Details: map[string]any{
			"campaign_id": input.CampaignID,
			"count":       count,
		},
	})

	return count, nil
}

// AssociateTarget links an unattributed capture to a target.
func (s *Service) AssociateTarget(ctx context.Context, eventID, targetID string, userID, username, clientIP, correlationID string) error {
	if err := s.repo.AssociateTarget(ctx, eventID, targetID); err != nil {
		return fmt.Errorf("associate target: %w", err)
	}

	assocResType := "capture_event"
	_ = s.auditSvc.Log(ctx, auditsvc.LogEntry{
		Timestamp:     time.Now().UTC(),
		Category:      auditsvc.CategoryCredentialCapture,
		Severity:      auditsvc.SeverityInfo,
		ActorType:     auditsvc.ActorTypeUser,
		ActorID:       &userID,
		ActorLabel:    username,
		Action:        "capture_associate_target",
		ResourceType:  &assocResType,
		ResourceID:    &eventID,
		CorrelationID: correlationID,
		SourceIP:      &clientIP,
		Details: map[string]any{
			"target_id": targetID,
		},
	})

	return nil
}

// GetMetrics returns aggregate capture metrics for a campaign.
func (s *Service) GetMetrics(ctx context.Context, campaignID string) (CaptureMetricsDTO, error) {
	base, err := s.repo.GetCaptureMetrics(ctx, campaignID)
	if err != nil {
		return CaptureMetricsDTO{}, fmt.Errorf("get metrics: %w", err)
	}

	variants, err := s.repo.GetVariantMetrics(ctx, campaignID)
	if err != nil {
		return CaptureMetricsDTO{}, fmt.Errorf("get variant metrics: %w", err)
	}

	timeline, err := s.repo.GetCaptureTimeline(ctx, campaignID, 60) // hourly buckets
	if err != nil {
		return CaptureMetricsDTO{}, fmt.Errorf("get timeline: %w", err)
	}

	fieldRates, err := s.repo.GetFieldCompletionRates(ctx, campaignID)
	if err != nil {
		return CaptureMetricsDTO{}, fmt.Errorf("get field rates: %w", err)
	}

	return CaptureMetricsDTO{
		CampaignID:           campaignID,
		TotalCaptures:        base.TotalCaptures,
		UniqueTargets:        base.UniqueTargets,
		RepeatSubmitters:     base.RepeatSubmitters,
		UnattributedCount:    base.UnattributedCount,
		VariantMetrics:       variants,
		Timeline:             timeline,
		FieldCompletionRates: fieldRates,
	}, nil
}

// CaptureMetricsDTO is the API representation of capture metrics.
type CaptureMetricsDTO struct {
	CampaignID           string                          `json:"campaign_id"`
	TotalCaptures        int                             `json:"total_captures"`
	UniqueTargets        int                             `json:"unique_targets"`
	RepeatSubmitters     int                             `json:"repeat_submitters"`
	UnattributedCount    int                             `json:"unattributed_count"`
	VariantMetrics       []repositories.VariantMetric    `json:"variant_metrics,omitempty"`
	Timeline             []repositories.TimelineBucket   `json:"timeline,omitempty"`
	FieldCompletionRates []repositories.FieldCompletionRate `json:"field_completion_rates,omitempty"`
}

// ExportInput controls export filtering.
type ExportInput struct {
	CampaignID  string
	DateAfter   *time.Time
	DateBefore  *time.Time
	IncludeRaw  bool // Requires credentials:reveal
	Format      string // "csv" or "json"
}

// ExportRow represents a single row in a credential export.
type ExportRow struct {
	EventID       string            `json:"event_id"`
	CampaignID    string            `json:"campaign_id"`
	TargetID      *string           `json:"target_id"`
	SourceIP      *string           `json:"source_ip"`
	UserAgent     *string           `json:"user_agent"`
	CapturedAt    time.Time         `json:"captured_at"`
	FieldNames    []string          `json:"field_names"`
	FieldValues   map[string]string `json:"field_values,omitempty"` // Only if IncludeRaw
	FieldCategories map[string]string `json:"field_categories"`
}

// Export returns capture data for export. If includeRaw, decrypts field values.
func (s *Service) Export(ctx context.Context, input ExportInput, userID, username, clientIP, correlationID string) ([]ExportRow, error) {
	events, _, err := s.repo.ListEvents(ctx, repositories.CaptureEventFilters{
		CampaignID: input.CampaignID,
		DateAfter:  input.DateAfter,
		DateBefore: input.DateBefore,
		Page:       1,
		PerPage:    10000, // Large limit for export.
	})
	if err != nil {
		return nil, fmt.Errorf("export: list events: %w", err)
	}

	rows := make([]ExportRow, len(events))
	for i, e := range events {
		fields, _ := s.repo.ListFieldsByEvent(ctx, e.ID)
		fieldNames := make([]string, len(fields))
		fieldCategories := make(map[string]string, len(fields))
		var fieldValues map[string]string

		if input.IncludeRaw {
			fieldValues = make(map[string]string, len(fields))
		}

		for j, f := range fields {
			fieldNames[j] = f.FieldName
			fieldCategories[f.FieldName] = string(f.FieldCategory)
			if input.IncludeRaw {
				val, err := s.encSvc.DecryptString(f.FieldValueEncrypted)
				if err != nil {
					fieldValues[f.FieldName] = "[decrypt error]"
				} else {
					fieldValues[f.FieldName] = val
				}
			}
		}

		rows[i] = ExportRow{
			EventID:         e.ID,
			CampaignID:      e.CampaignID,
			TargetID:        e.TargetID,
			SourceIP:        e.SourceIP,
			UserAgent:       e.UserAgent,
			CapturedAt:      e.CapturedAt,
			FieldNames:      fieldNames,
			FieldValues:     fieldValues,
			FieldCategories: fieldCategories,
		}
	}

	// Audit log the export.
	exportResType := "campaign"
	exportResID := input.CampaignID
	_ = s.auditSvc.Log(ctx, auditsvc.LogEntry{
		Timestamp:     time.Now().UTC(),
		Category:      auditsvc.CategoryCredentialCapture,
		Severity:      auditsvc.SeverityWarning,
		ActorType:     auditsvc.ActorTypeUser,
		ActorID:       &userID,
		ActorLabel:    username,
		Action:        "credential_export",
		ResourceType:  &exportResType,
		ResourceID:    &exportResID,
		CorrelationID: correlationID,
		SourceIP:      &clientIP,
		Details: map[string]any{
			"format":      input.Format,
			"count":       len(rows),
			"include_raw": input.IncludeRaw,
		},
	})

	return rows, nil
}

// GetCategorizationRules returns field categorization rules for a landing page.
func (s *Service) GetCategorizationRules(ctx context.Context, landingPageID *string) ([]repositories.FieldCategorizationRule, error) {
	return s.repo.ListCategorizationRules(ctx, landingPageID)
}

// UpsertCategorizationRule creates or updates a field categorization rule.
func (s *Service) UpsertCategorizationRule(ctx context.Context, rule repositories.FieldCategorizationRule) (repositories.FieldCategorizationRule, error) {
	return s.repo.UpsertCategorizationRule(ctx, rule)
}

// DeleteCategorizationRule removes a custom categorization rule.
func (s *Service) DeleteCategorizationRule(ctx context.Context, id string) error {
	return s.repo.DeleteCategorizationRule(ctx, id)
}

// sendCaptureNotification sends WebSocket notifications for a new capture.
// Notifies operators and engineers (roles that have credentials:read).
func (s *Service) sendCaptureNotification(ctx context.Context, event repositories.CaptureEvent, fieldNames []string) {
	if s.notifSvc == nil {
		return
	}

	body := fmt.Sprintf("Credential submission captured — %d fields", len(fieldNames))
	if len(fieldNames) > 0 {
		body += ": " + strings.Join(fieldNames, ", ")
	}

	// Send to both operator and engineer roles (both have credentials:read).
	for _, role := range []string{"operator", "engineer"} {
		s.notifSvc.Create(ctx, notifsvc.CreateNotificationParams{
			Category:     "credential_capture",
			Severity:     "info",
			Title:        "New credential submission",
			Body:         body,
			ResourceType: "campaign",
			ResourceID:   event.CampaignID,
			ActionURL:    fmt.Sprintf("/campaigns/%s/captures", event.CampaignID),
			Recipients: notifsvc.RecipientSpec{
				Role: role,
			},
		})
	}
}

// checkNotArchived returns an error if the campaign is archived.
func (s *Service) checkNotArchived(ctx context.Context, campaignID string) error {
	camp, err := s.campRepo.GetByID(ctx, campaignID)
	if err != nil {
		return fmt.Errorf("check archive status: %w", err)
	}
	if camp.CurrentState == "archived" {
		return &ForbiddenError{Msg: "cannot modify captures for archived campaign"}
	}
	return nil
}

// categorizeField determines the category for a field based on rules.
func categorizeField(fieldName string, rules []repositories.FieldCategorizationRule) repositories.FieldCategory {
	lower := strings.ToLower(fieldName)
	for _, rule := range rules {
		if strings.Contains(lower, strings.ToLower(rule.FieldPattern)) {
			return rule.Category
		}
	}
	return repositories.FieldCategoryCustom
}

// Error types.

// ValidationError indicates invalid input.
type ValidationError struct{ Msg string }

func (e *ValidationError) Error() string { return e.Msg }

// NotFoundError indicates a resource was not found.
type NotFoundError struct{ Msg string }

func (e *NotFoundError) Error() string { return e.Msg }

// ForbiddenError indicates the action is not allowed.
type ForbiddenError struct{ Msg string }

func (e *ForbiddenError) Error() string { return e.Msg }

// Helper functions.

func strPtr(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

func strPtrNonEmpty(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

func defaultStr(s, def string) string {
	if s == "" {
		return def
	}
	return s
}
