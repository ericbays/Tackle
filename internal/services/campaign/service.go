// Package campaign provides business logic for campaign lifecycle management.
package campaign

import (
	"context"
	"crypto/sha256"
	"encoding/binary"
	"fmt"
	"log/slog"
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"

	"tackle/internal/campaign"
	"tackle/internal/repositories"
	auditsvc "tackle/internal/services/audit"
)

// EmailDeliveryHook is the interface the campaign service uses to trigger
// email delivery actions after state transitions. This avoids a circular
// import with the emaildelivery package.
type EmailDeliveryHook interface {
	BuildQueue(ctx context.Context, campaignID string) (int, error)
	StartSending(ctx context.Context, campaignID string) error
	Pause(ctx context.Context, campaignID string) error
	Resume(ctx context.Context, campaignID string) error
	CancelUnsentEmails(ctx context.Context, campaignID string) (int, error)
}

// ValidationError indicates invalid input.
type ValidationError struct{ Msg string }

func (e *ValidationError) Error() string { return e.Msg }

// ConflictError indicates a resource conflict (e.g., duplicate name, invalid state).
type ConflictError struct{ Msg string }

func (e *ConflictError) Error() string { return e.Msg }

// NotFoundError indicates a resource was not found.
type NotFoundError struct{ Msg string }

func (e *NotFoundError) Error() string { return e.Msg }

// Service handles campaign business logic.
type Service struct {
	repo             *repositories.CampaignRepository
	auditSvc         *auditsvc.AuditService
	emailDeliverySvc EmailDeliveryHook
	teardownSvc      *TeardownService
}

// NewService creates a new campaign Service.
func NewService(repo *repositories.CampaignRepository, auditSvc *auditsvc.AuditService) *Service {
	return &Service{repo: repo, auditSvc: auditSvc}
}

// SetEmailDeliveryHook sets the email delivery service for post-transition hooks.
// This is called after construction to break the initialization cycle.
func (s *Service) SetEmailDeliveryHook(hook EmailDeliveryHook) {
	s.emailDeliverySvc = hook
}

// SetTeardownService sets the teardown service for post-transition infrastructure cleanup.
func (s *Service) SetTeardownService(svc *TeardownService) {
	s.teardownSvc = svc
}

// ---------- DTOs ----------

// CampaignDTO is the API representation of a campaign.
type CampaignDTO struct {
	ID                string         `json:"id"`
	Name              string         `json:"name"`
	Description       string         `json:"description"`
	CurrentState      string         `json:"current_state"`
	StateChangedAt    time.Time      `json:"state_changed_at"`
	LandingPageID     *string        `json:"landing_page_id"`
	CloudProvider     *string        `json:"cloud_provider"`
	Region            *string        `json:"region"`
	InstanceType      *string        `json:"instance_type"`
	EndpointDomainID  *string        `json:"endpoint_domain_id"`
	ThrottleRate      *int           `json:"throttle_rate"`
	InterEmailDelayMin *int          `json:"inter_email_delay_min"`
	InterEmailDelayMax *int          `json:"inter_email_delay_max"`
	SendOrder         string         `json:"send_order"`
	ScheduledLaunchAt *time.Time     `json:"scheduled_launch_at"`
	GracePeriodHours  int            `json:"grace_period_hours"`
	StartDate         *time.Time     `json:"start_date"`
	EndDate           *time.Time     `json:"end_date"`
	ApprovedBy        *string        `json:"approved_by"`
	ApprovalComment   *string        `json:"approval_comment"`
	LaunchedAt        *time.Time     `json:"launched_at"`
	CompletedAt       *time.Time     `json:"completed_at"`
	ArchivedAt        *time.Time     `json:"archived_at"`
	Configuration     map[string]any `json:"configuration"`
	CreatedBy         string         `json:"created_by"`
	CreatedByName     string         `json:"created_by_name,omitempty"`
	TargetCount       int            `json:"target_count,omitempty"`
	CreatedAt         time.Time      `json:"created_at"`
	UpdatedAt         time.Time      `json:"updated_at"`
}

// TemplateVariantDTO is the API representation of a template variant.
type TemplateVariantDTO struct {
	ID         string    `json:"id"`
	CampaignID string    `json:"campaign_id"`
	TemplateID string    `json:"template_id"`
	SplitRatio int       `json:"split_ratio"`
	Label      string    `json:"label"`
	CreatedAt  time.Time `json:"created_at"`
}

// SendWindowDTO is the API representation of a send window.
type SendWindowDTO struct {
	ID         string   `json:"id"`
	CampaignID string   `json:"campaign_id"`
	Days       []string `json:"days"`
	StartTime  string   `json:"start_time"`
	EndTime    string   `json:"end_time"`
	Timezone   string   `json:"timezone"`
}

// BuildLogDTO is the API representation of a build log entry.
type BuildLogDTO struct {
	ID           string     `json:"id"`
	CampaignID   string     `json:"campaign_id"`
	StepName     string     `json:"step_name"`
	StepOrder    int        `json:"step_order"`
	Status       string     `json:"status"`
	StartedAt    *time.Time `json:"started_at"`
	CompletedAt  *time.Time `json:"completed_at"`
	ErrorDetails *string    `json:"error_details"`
	CreatedAt    time.Time  `json:"created_at"`
}

// StateTransitionDTO is the API representation of a state transition.
type StateTransitionDTO struct {
	ID         string    `json:"id"`
	CampaignID string    `json:"campaign_id"`
	FromState  string    `json:"from_state"`
	ToState    string    `json:"to_state"`
	ActorID    *string   `json:"actor_id"`
	Reason     string    `json:"reason"`
	CreatedAt  time.Time `json:"created_at"`
}

// ConfigTemplateDTO is the API representation of a campaign config template.
type ConfigTemplateDTO struct {
	ID          string         `json:"id"`
	Name        string         `json:"name"`
	Description string         `json:"description"`
	ConfigJSON  map[string]any `json:"config_json"`
	CreatedBy   string         `json:"created_by"`
	CreatedAt   time.Time      `json:"created_at"`
	UpdatedAt   time.Time      `json:"updated_at"`
}

// MetricsDTO provides campaign-level metrics.
type MetricsDTO struct {
	EmailCounts map[string]int `json:"email_counts"`
}

// ---------- Input Types ----------

// CreateInput is the input for creating a new campaign.
type CreateInput struct {
	Name              string         `json:"name"`
	Description       string         `json:"description"`
	LandingPageID     *string        `json:"landing_page_id"`
	CloudProvider     *string        `json:"cloud_provider"`
	Region            *string        `json:"region"`
	InstanceType      *string        `json:"instance_type"`
	EndpointDomainID  *string        `json:"endpoint_domain_id"`
	ThrottleRate      *int           `json:"throttle_rate"`
	InterEmailDelayMin *int          `json:"inter_email_delay_min"`
	InterEmailDelayMax *int          `json:"inter_email_delay_max"`
	SendOrder         string         `json:"send_order"`
	ScheduledLaunchAt *time.Time     `json:"scheduled_launch_at"`
	GracePeriodHours  *int           `json:"grace_period_hours"`
	StartDate         *time.Time     `json:"start_date"`
	EndDate           *time.Time     `json:"end_date"`
	Configuration     map[string]any `json:"configuration"`
}

// UpdateInput is the input for updating a campaign.
type UpdateInput struct {
	Name              *string        `json:"name"`
	Description       *string        `json:"description"`
	LandingPageID     *string        `json:"landing_page_id"`
	CloudProvider     *string        `json:"cloud_provider"`
	Region            *string        `json:"region"`
	InstanceType      *string        `json:"instance_type"`
	EndpointDomainID  *string        `json:"endpoint_domain_id"`
	ThrottleRate      *int           `json:"throttle_rate"`
	InterEmailDelayMin *int          `json:"inter_email_delay_min"`
	InterEmailDelayMax *int          `json:"inter_email_delay_max"`
	SendOrder         *string        `json:"send_order"`
	ScheduledLaunchAt *time.Time     `json:"scheduled_launch_at"`
	GracePeriodHours  *int           `json:"grace_period_hours"`
	StartDate         *time.Time     `json:"start_date"`
	EndDate           *time.Time     `json:"end_date"`
	Configuration     map[string]any `json:"configuration"`
}

// TemplateVariantInput is the input for adding template variants to a campaign.
type TemplateVariantInput struct {
	Variants []VariantEntry `json:"variants"`
}

// VariantEntry represents one variant in a TemplateVariantInput.
type VariantEntry struct {
	TemplateID string `json:"template_id"`
	SplitRatio int    `json:"split_ratio"`
	Label      string `json:"label"`
}

// SendWindowInput is the input for setting send windows.
type SendWindowInput struct {
	Windows []WindowEntry `json:"windows"`
}

// WindowEntry represents one window in a SendWindowInput.
type WindowEntry struct {
	Days      []string `json:"days"`
	StartTime string   `json:"start_time"`
	EndTime   string   `json:"end_time"`
	Timezone  string   `json:"timezone"`
}

// TransitionInput is the input for a state transition.
type TransitionInput struct {
	Reason string `json:"reason"`
}

// CloneInput configures what to clone.
type CloneInput struct {
	IncludeLandingPage     bool `json:"include_landing_page"`
	IncludeTargetGroups    bool `json:"include_target_groups"`
	IncludeSMTPConfigs     bool `json:"include_smtp_configs"`
	IncludeTemplateVariants bool `json:"include_template_variants"`
	IncludeSendSchedule    bool `json:"include_send_schedule"`
	IncludeEndpointConfig  bool `json:"include_endpoint_config"`
}

// ConfigTemplateInput is the input for creating/updating a config template.
type ConfigTemplateInput struct {
	Name        string         `json:"name"`
	Description string         `json:"description"`
	ConfigJSON  map[string]any `json:"config_json"`
}

// ListInput contains filters for listing campaigns.
type ListInput struct {
	States          []string   `json:"states"`
	Name            string     `json:"name"`
	CreatedBy       string     `json:"created_by"`
	DateFrom        *time.Time `json:"date_from"`
	DateTo          *time.Time `json:"date_to"`
	IncludeArchived bool       `json:"include_archived"`
	Page            int        `json:"page"`
	PerPage         int        `json:"per_page"`
	// OwnerID restricts results to campaigns created by or shared with this user.
	// Used for operator role scoping. Empty string means no filtering.
	OwnerID string `json:"-"`
}

// ---------- Conversion Helpers ----------

func toCampaignDTO(c repositories.Campaign) CampaignDTO {
	return CampaignDTO{
		ID: c.ID, Name: c.Name, Description: c.Description,
		CurrentState: c.CurrentState, StateChangedAt: c.StateChangedAt,
		LandingPageID: c.LandingPageID, CloudProvider: c.CloudProvider,
		Region: c.Region, InstanceType: c.InstanceType,
		EndpointDomainID: c.EndpointDomainID, ThrottleRate: c.ThrottleRate,
		InterEmailDelayMin: c.InterEmailDelayMin, InterEmailDelayMax: c.InterEmailDelayMax,
		SendOrder: c.SendOrder, ScheduledLaunchAt: c.ScheduledLaunchAt,
		GracePeriodHours: c.GracePeriodHours, StartDate: c.StartDate, EndDate: c.EndDate,
		ApprovedBy: c.ApprovedBy, ApprovalComment: c.ApprovalComment,
		LaunchedAt: c.LaunchedAt, CompletedAt: c.CompletedAt, ArchivedAt: c.ArchivedAt,
		Configuration: c.Configuration, CreatedBy: c.CreatedBy,
		CreatedAt: c.CreatedAt, UpdatedAt: c.UpdatedAt,
	}
}

func toVariantDTO(v repositories.CampaignTemplateVariant) TemplateVariantDTO {
	return TemplateVariantDTO{
		ID: v.ID, CampaignID: v.CampaignID, TemplateID: v.TemplateID,
		SplitRatio: v.SplitRatio, Label: v.Label, CreatedAt: v.CreatedAt,
	}
}

func toSendWindowDTO(w repositories.CampaignSendWindow) SendWindowDTO {
	return SendWindowDTO{
		ID: w.ID, CampaignID: w.CampaignID, Days: w.Days,
		StartTime: w.StartTime, EndTime: w.EndTime, Timezone: w.Timezone,
	}
}

func toBuildLogDTO(l repositories.CampaignBuildLog) BuildLogDTO {
	return BuildLogDTO{
		ID: l.ID, CampaignID: l.CampaignID, StepName: l.StepName,
		StepOrder: l.StepOrder, Status: l.Status, StartedAt: l.StartedAt,
		CompletedAt: l.CompletedAt, ErrorDetails: l.ErrorDetails, CreatedAt: l.CreatedAt,
	}
}

func toTransitionDTO(t repositories.CampaignStateTransition) StateTransitionDTO {
	return StateTransitionDTO{
		ID: t.ID, CampaignID: t.CampaignID, FromState: t.FromState,
		ToState: t.ToState, ActorID: t.ActorID, Reason: t.Reason, CreatedAt: t.CreatedAt,
	}
}

func toConfigTemplateDTO(t repositories.CampaignConfigTemplate) ConfigTemplateDTO {
	return ConfigTemplateDTO{
		ID: t.ID, Name: t.Name, Description: t.Description,
		ConfigJSON: t.ConfigJSON, CreatedBy: t.CreatedBy,
		CreatedAt: t.CreatedAt, UpdatedAt: t.UpdatedAt,
	}
}

// ---------- Campaign CRUD ----------

// Create creates a new campaign in Draft state.
func (s *Service) Create(ctx context.Context, input CreateInput, actorID, actorName, ip, correlationID string) (CampaignDTO, error) {
	name := strings.TrimSpace(input.Name)
	if name == "" {
		return CampaignDTO{}, &ValidationError{Msg: "campaign name is required"}
	}
	if len(name) > 255 {
		return CampaignDTO{}, &ValidationError{Msg: "campaign name must be 255 characters or less"}
	}

	if input.SendOrder == "" {
		input.SendOrder = "default"
	}
	if err := validateSendOrder(input.SendOrder); err != nil {
		return CampaignDTO{}, err
	}

	// Normalize dates to UTC.
	if input.StartDate != nil {
		utc := input.StartDate.UTC()
		input.StartDate = &utc
	}
	if input.EndDate != nil {
		utc := input.EndDate.UTC()
		input.EndDate = &utc
	}
	if input.ScheduledLaunchAt != nil {
		utc := input.ScheduledLaunchAt.UTC()
		input.ScheduledLaunchAt = &utc
	}

	if input.StartDate != nil && input.EndDate != nil && !input.EndDate.After(*input.StartDate) {
		return CampaignDTO{}, &ValidationError{Msg: "end_date must be after start_date"}
	}

	gracePeriod := 72
	if input.GracePeriodHours != nil {
		gracePeriod = *input.GracePeriodHours
	}

	c := repositories.Campaign{
		Name:              name,
		Description:       strings.TrimSpace(input.Description),
		LandingPageID:     input.LandingPageID,
		CloudProvider:     input.CloudProvider,
		Region:            input.Region,
		InstanceType:      input.InstanceType,
		EndpointDomainID:  input.EndpointDomainID,
		ThrottleRate:      input.ThrottleRate,
		InterEmailDelayMin: input.InterEmailDelayMin,
		InterEmailDelayMax: input.InterEmailDelayMax,
		SendOrder:         input.SendOrder,
		ScheduledLaunchAt: input.ScheduledLaunchAt,
		GracePeriodHours:  gracePeriod,
		StartDate:         input.StartDate,
		EndDate:           input.EndDate,
		Configuration:     input.Configuration,
		CreatedBy:         actorID,
	}
	if c.Configuration == nil {
		c.Configuration = make(map[string]any)
	}

	created, err := s.repo.Create(ctx, c)
	if err != nil {
		if strings.Contains(err.Error(), "idx_campaigns_name_active") {
			return CampaignDTO{}, &ConflictError{Msg: "a campaign with this name already exists"}
		}
		return CampaignDTO{}, fmt.Errorf("campaign service: create: %w", err)
	}

	resType := "campaign"
	s.logAudit(ctx, "campaign.created", &actorID, actorName, &resType, &created.ID, ip, correlationID, nil)

	return toCampaignDTO(created), nil
}

// Get returns a campaign by ID.
func (s *Service) Get(ctx context.Context, id string) (CampaignDTO, error) {
	c, err := s.repo.GetByID(ctx, id)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			return CampaignDTO{}, &NotFoundError{Msg: "campaign not found"}
		}
		return CampaignDTO{}, fmt.Errorf("campaign service: get: %w", err)
	}
	return toCampaignDTO(c), nil
}

// List returns paginated, filtered campaigns.
func (s *Service) List(ctx context.Context, input ListInput) ([]CampaignDTO, int, error) {
	result, err := s.repo.List(ctx, repositories.CampaignFilters{
		States:          input.States,
		Name:            input.Name,
		CreatedBy:       input.CreatedBy,
		DateFrom:        input.DateFrom,
		DateTo:          input.DateTo,
		IncludeArchived: input.IncludeArchived,
		Page:            input.Page,
		PerPage:         input.PerPage,
		OwnerID:         input.OwnerID,
	})
	if err != nil {
		return nil, 0, fmt.Errorf("campaign service: list: %w", err)
	}

	dtos := make([]CampaignDTO, len(result.Campaigns))
	campaignIDs := make([]string, len(result.Campaigns))
	creatorIDs := make([]string, 0, len(result.Campaigns))
	seen := make(map[string]bool)
	for i, c := range result.Campaigns {
		dtos[i] = toCampaignDTO(c)
		campaignIDs[i] = c.ID
		if !seen[c.CreatedBy] {
			creatorIDs = append(creatorIDs, c.CreatedBy)
			seen[c.CreatedBy] = true
		}
	}

	// Populate target counts and creator names for list view.
	if len(campaignIDs) > 0 {
		targetCounts, tcErr := s.repo.GetTargetCountsBatch(ctx, campaignIDs)
		if tcErr == nil {
			for i := range dtos {
				dtos[i].TargetCount = targetCounts[dtos[i].ID]
			}
		}
		creatorNames, cnErr := s.repo.GetCreatorNamesBatch(ctx, creatorIDs)
		if cnErr == nil {
			for i := range dtos {
				dtos[i].CreatedByName = creatorNames[dtos[i].CreatedBy]
			}
		}
	}

	return dtos, result.Total, nil
}

// Update modifies a campaign. Only allowed in Draft state.
func (s *Service) Update(ctx context.Context, id string, input UpdateInput, actorID, actorName, ip, correlationID string) (CampaignDTO, error) {
	existing, err := s.repo.GetByID(ctx, id)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			return CampaignDTO{}, &NotFoundError{Msg: "campaign not found"}
		}
		return CampaignDTO{}, fmt.Errorf("campaign service: update: %w", err)
	}

	if !campaign.State(existing.CurrentState).IsMutable() {
		return CampaignDTO{}, &ConflictError{Msg: fmt.Sprintf("campaign configuration can only be modified in draft state; current state is %q", existing.CurrentState)}
	}

	if input.Name != nil {
		name := strings.TrimSpace(*input.Name)
		if name == "" {
			return CampaignDTO{}, &ValidationError{Msg: "campaign name is required"}
		}
		if len(name) > 255 {
			return CampaignDTO{}, &ValidationError{Msg: "campaign name must be 255 characters or less"}
		}
		input.Name = &name
	}

	if input.SendOrder != nil {
		if err := validateSendOrder(*input.SendOrder); err != nil {
			return CampaignDTO{}, err
		}
	}

	// Normalize dates to UTC.
	if input.StartDate != nil {
		utc := (*input.StartDate).UTC()
		input.StartDate = &utc
	}
	if input.EndDate != nil {
		utc := (*input.EndDate).UTC()
		input.EndDate = &utc
	}
	if input.ScheduledLaunchAt != nil {
		utc := (*input.ScheduledLaunchAt).UTC()
		input.ScheduledLaunchAt = &utc
	}

	// Validate end_date > start_date (using updated or existing values).
	effectiveStart := existing.StartDate
	if input.StartDate != nil {
		effectiveStart = input.StartDate
	}
	effectiveEnd := existing.EndDate
	if input.EndDate != nil {
		effectiveEnd = input.EndDate
	}
	if effectiveStart != nil && effectiveEnd != nil && !effectiveEnd.After(*effectiveStart) {
		return CampaignDTO{}, &ValidationError{Msg: "end_date must be after start_date"}
	}

	// Check domain exclusivity if changing domain.
	if input.EndpointDomainID != nil && *input.EndpointDomainID != "" {
		inUse, err := s.repo.IsDomainInUse(ctx, *input.EndpointDomainID, id)
		if err != nil {
			return CampaignDTO{}, fmt.Errorf("campaign service: update: domain check: %w", err)
		}
		if inUse {
			return CampaignDTO{}, &ConflictError{Msg: "domain is already in use by another campaign"}
		}
	}

	updated, err := s.repo.Update(ctx, id, repositories.CampaignUpdate{
		Name: input.Name, Description: input.Description,
		LandingPageID: input.LandingPageID, CloudProvider: input.CloudProvider,
		Region: input.Region, InstanceType: input.InstanceType,
		EndpointDomainID: input.EndpointDomainID, ThrottleRate: input.ThrottleRate,
		InterEmailDelayMin: input.InterEmailDelayMin, InterEmailDelayMax: input.InterEmailDelayMax,
		SendOrder: input.SendOrder, ScheduledLaunchAt: input.ScheduledLaunchAt,
		GracePeriodHours: input.GracePeriodHours, StartDate: input.StartDate,
		EndDate: input.EndDate, Configuration: input.Configuration,
	})
	if err != nil {
		if strings.Contains(err.Error(), "idx_campaigns_name_active") {
			return CampaignDTO{}, &ConflictError{Msg: "a campaign with this name already exists"}
		}
		if strings.Contains(err.Error(), "not found") {
			return CampaignDTO{}, &NotFoundError{Msg: "campaign not found"}
		}
		return CampaignDTO{}, fmt.Errorf("campaign service: update: %w", err)
	}

	resType := "campaign"
	s.logAudit(ctx, "campaign.updated", &actorID, actorName, &resType, &id, ip, correlationID, nil)

	return toCampaignDTO(updated), nil
}

// Delete hard-deletes a campaign. Only allowed in Draft state.
func (s *Service) Delete(ctx context.Context, id string, actorID, actorName, ip, correlationID string) error {
	existing, err := s.repo.GetByID(ctx, id)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			return &NotFoundError{Msg: "campaign not found"}
		}
		return fmt.Errorf("campaign service: delete: %w", err)
	}

	if existing.CurrentState != string(campaign.StateDraft) {
		return &ConflictError{Msg: "only draft campaigns can be deleted"}
	}

	if err := s.repo.Delete(ctx, id); err != nil {
		return fmt.Errorf("campaign service: delete: %w", err)
	}

	resType := "campaign"
	s.logAudit(ctx, "campaign.deleted", &actorID, actorName, &resType, &id, ip, correlationID, nil)
	return nil
}

// ---------- State Transitions ----------

// Transition performs a validated state transition.
func (s *Service) Transition(ctx context.Context, id string, targetState campaign.State, reason string, actorID, actorName, actorRole, ip, correlationID string) (CampaignDTO, error) {
	tx, err := s.repo.BeginTx(ctx)
	if err != nil {
		return CampaignDTO{}, fmt.Errorf("campaign service: transition: begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	locked, err := s.repo.GetByIDForUpdate(ctx, tx, id)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			return CampaignDTO{}, &NotFoundError{Msg: "campaign not found"}
		}
		return CampaignDTO{}, fmt.Errorf("campaign service: transition: lock: %w", err)
	}

	currentState := campaign.State(locked.CurrentState)
	t, err := campaign.ValidateTransition(currentState, targetState, actorRole)
	if err != nil {
		if _, ok := err.(*campaign.StateError); ok {
			return CampaignDTO{}, &ConflictError{Msg: err.Error()}
		}
		if _, ok := err.(*campaign.RoleError); ok {
			return CampaignDTO{}, &ConflictError{Msg: err.Error()}
		}
		return CampaignDTO{}, fmt.Errorf("campaign service: transition: validate: %w", err)
	}

	actorIDPtr := &actorID
	if actorRole == "system" {
		actorIDPtr = nil
	}

	if err := s.repo.TransitionState(ctx, tx, id, string(currentState), string(targetState), actorIDPtr, reason); err != nil {
		if strings.Contains(err.Error(), "concurrent") {
			return CampaignDTO{}, &ConflictError{Msg: "concurrent state transition detected; please retry"}
		}
		return CampaignDTO{}, fmt.Errorf("campaign service: transition: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return CampaignDTO{}, fmt.Errorf("campaign service: transition: commit: %w", err)
	}

	// Re-fetch to get updated state.
	updated, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return CampaignDTO{}, fmt.Errorf("campaign service: transition: re-fetch: %w", err)
	}

	// Generate a workflow correlation ID that links all downstream audit entries
	// for this state transition (build steps, email delivery, teardown, etc.).
	workflowCorrelationID := correlationID
	if workflowCorrelationID == "" {
		workflowCorrelationID = uuid.New().String()
	}

	resType := "campaign"
	details := map[string]any{
		"transition":              t.Name,
		"from_state":             string(currentState),
		"to_state":               string(targetState),
		"reason":                 reason,
		"description":            t.Description,
		"workflow_correlation_id": workflowCorrelationID,
	}
	s.logAudit(ctx, "campaign.state_changed", actorIDPtr, actorName, &resType, &id, ip, workflowCorrelationID, details)

	// Post-transition email delivery hooks.
	s.handleEmailDeliveryHooks(ctx, id, currentState, targetState, workflowCorrelationID)

	// Post-transition infrastructure teardown hooks.
	s.handleTeardownHooks(ctx, id, currentState, targetState, workflowCorrelationID)

	return toCampaignDTO(updated), nil
}

// ---------- Template Variants ----------

// SetTemplateVariants replaces all template variants for a campaign.
func (s *Service) SetTemplateVariants(ctx context.Context, campaignID string, input TemplateVariantInput, actorID, actorName, ip, correlationID string) ([]TemplateVariantDTO, error) {
	existing, err := s.repo.GetByID(ctx, campaignID)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			return nil, &NotFoundError{Msg: "campaign not found"}
		}
		return nil, fmt.Errorf("campaign service: set variants: %w", err)
	}

	if !campaign.State(existing.CurrentState).IsMutable() {
		return nil, &ConflictError{Msg: "template variants can only be modified in draft state"}
	}

	if len(input.Variants) == 0 {
		return nil, &ValidationError{Msg: "at least one template variant is required"}
	}

	totalRatio := 0
	for _, v := range input.Variants {
		if v.SplitRatio < 1 || v.SplitRatio > 100 {
			return nil, &ValidationError{Msg: fmt.Sprintf("split_ratio must be between 1 and 100, got %d", v.SplitRatio)}
		}
		totalRatio += v.SplitRatio
	}
	if totalRatio != 100 {
		return nil, &ValidationError{Msg: fmt.Sprintf("split_ratio values must sum to 100, got %d", totalRatio)}
	}

	// Delete existing and insert new.
	if err := s.repo.DeleteTemplateVariants(ctx, campaignID); err != nil {
		return nil, fmt.Errorf("campaign service: set variants: delete: %w", err)
	}

	var dtos []TemplateVariantDTO
	for i, v := range input.Variants {
		label := v.Label
		if label == "" {
			label = fmt.Sprintf("Variant %c", 'A'+i)
		}
		created, err := s.repo.CreateTemplateVariant(ctx, repositories.CampaignTemplateVariant{
			CampaignID: campaignID,
			TemplateID: v.TemplateID,
			SplitRatio: v.SplitRatio,
			Label:      label,
		})
		if err != nil {
			return nil, fmt.Errorf("campaign service: set variants: create: %w", err)
		}
		dtos = append(dtos, toVariantDTO(created))
	}

	resType := "campaign"
	s.logAudit(ctx, "campaign.variants_updated", &actorID, actorName, &resType, &campaignID, ip, correlationID, nil)
	return dtos, nil
}

// GetTemplateVariants returns template variants for a campaign.
func (s *Service) GetTemplateVariants(ctx context.Context, campaignID string) ([]TemplateVariantDTO, error) {
	variants, err := s.repo.ListTemplateVariants(ctx, campaignID)
	if err != nil {
		return nil, fmt.Errorf("campaign service: get variants: %w", err)
	}
	dtos := make([]TemplateVariantDTO, len(variants))
	for i, v := range variants {
		dtos[i] = toVariantDTO(v)
	}
	return dtos, nil
}

// ---------- Send Windows ----------

// SetSendWindows replaces all send windows for a campaign.
func (s *Service) SetSendWindows(ctx context.Context, campaignID string, input SendWindowInput, actorID, actorName, ip, correlationID string) ([]SendWindowDTO, error) {
	existing, err := s.repo.GetByID(ctx, campaignID)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			return nil, &NotFoundError{Msg: "campaign not found"}
		}
		return nil, fmt.Errorf("campaign service: set windows: %w", err)
	}

	if !campaign.State(existing.CurrentState).IsMutable() {
		return nil, &ConflictError{Msg: "send windows can only be modified in draft state"}
	}

	validDays := map[string]bool{
		"monday": true, "tuesday": true, "wednesday": true, "thursday": true,
		"friday": true, "saturday": true, "sunday": true,
	}

	for _, w := range input.Windows {
		for _, d := range w.Days {
			if !validDays[strings.ToLower(d)] {
				return nil, &ValidationError{Msg: fmt.Sprintf("invalid day: %q", d)}
			}
		}
		if w.StartTime == "" || w.EndTime == "" {
			return nil, &ValidationError{Msg: "start_time and end_time are required"}
		}
		if w.Timezone == "" {
			return nil, &ValidationError{Msg: "timezone is required"}
		}
		if _, err := time.LoadLocation(w.Timezone); err != nil {
			return nil, &ValidationError{Msg: fmt.Sprintf("invalid IANA timezone: %q", w.Timezone)}
		}
	}

	if err := s.repo.DeleteSendWindows(ctx, campaignID); err != nil {
		return nil, fmt.Errorf("campaign service: set windows: delete: %w", err)
	}

	var dtos []SendWindowDTO
	for _, w := range input.Windows {
		created, err := s.repo.CreateSendWindow(ctx, repositories.CampaignSendWindow{
			CampaignID: campaignID,
			Days:       w.Days,
			StartTime:  w.StartTime,
			EndTime:    w.EndTime,
			Timezone:   w.Timezone,
		})
		if err != nil {
			return nil, fmt.Errorf("campaign service: set windows: create: %w", err)
		}
		dtos = append(dtos, toSendWindowDTO(created))
	}

	resType := "campaign"
	s.logAudit(ctx, "campaign.send_windows_updated", &actorID, actorName, &resType, &campaignID, ip, correlationID, nil)
	return dtos, nil
}

// GetSendWindows returns send windows for a campaign.
func (s *Service) GetSendWindows(ctx context.Context, campaignID string) ([]SendWindowDTO, error) {
	windows, err := s.repo.ListSendWindows(ctx, campaignID)
	if err != nil {
		return nil, fmt.Errorf("campaign service: get windows: %w", err)
	}
	dtos := make([]SendWindowDTO, len(windows))
	for i, w := range windows {
		dtos[i] = toSendWindowDTO(w)
	}
	return dtos, nil
}

// ---------- Build Logs ----------

// GetBuildLogs returns build logs for a campaign.
func (s *Service) GetBuildLogs(ctx context.Context, campaignID string) ([]BuildLogDTO, error) {
	logs, err := s.repo.ListBuildLogs(ctx, campaignID)
	if err != nil {
		return nil, fmt.Errorf("campaign service: get build logs: %w", err)
	}
	dtos := make([]BuildLogDTO, len(logs))
	for i, l := range logs {
		dtos[i] = toBuildLogDTO(l)
	}
	return dtos, nil
}

// ---------- Canary Targets ----------

// SetCanaryTargets designates canary targets for a campaign. Only allowed in Draft state.
func (s *Service) SetCanaryTargets(ctx context.Context, campaignID string, targetIDs []string, actorID string) error {
	c, err := s.repo.GetByID(ctx, campaignID)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			return &NotFoundError{Msg: "campaign not found"}
		}
		return fmt.Errorf("campaign service: set canary targets: %w", err)
	}
	if !campaign.State(c.CurrentState).IsMutable() {
		return &ConflictError{Msg: "canary targets can only be set in draft state"}
	}
	if err := s.repo.SetCanaryTargets(ctx, campaignID, targetIDs, actorID); err != nil {
		return fmt.Errorf("campaign service: set canary targets: %w", err)
	}
	return nil
}

// GetCanaryTargets returns canary target IDs for a campaign.
func (s *Service) GetCanaryTargets(ctx context.Context, campaignID string) ([]string, error) {
	ids, err := s.repo.ListCanaryTargets(ctx, campaignID)
	if err != nil {
		return nil, fmt.Errorf("campaign service: get canary targets: %w", err)
	}
	if ids == nil {
		ids = []string{}
	}
	return ids, nil
}

// ---------- Metrics ----------

// GetMetrics returns campaign email metrics.
func (s *Service) GetMetrics(ctx context.Context, campaignID string) (MetricsDTO, error) {
	counts, err := s.repo.CountEmailsByStatus(ctx, campaignID)
	if err != nil {
		return MetricsDTO{}, fmt.Errorf("campaign service: get metrics: %w", err)
	}
	return MetricsDTO{EmailCounts: counts}, nil
}

// ---------- State Transition History ----------

// GetTransitions returns state transition history for a campaign.
func (s *Service) GetTransitions(ctx context.Context, campaignID string) ([]StateTransitionDTO, error) {
	transitions, err := s.repo.ListTransitions(ctx, campaignID)
	if err != nil {
		return nil, fmt.Errorf("campaign service: get transitions: %w", err)
	}
	dtos := make([]StateTransitionDTO, len(transitions))
	for i, t := range transitions {
		dtos[i] = toTransitionDTO(t)
	}
	return dtos, nil
}

// ---------- Campaign Cloning ----------

// Clone creates a copy of a campaign in Draft state.
func (s *Service) Clone(ctx context.Context, sourceID string, input CloneInput, actorID, actorName, ip, correlationID string) (CampaignDTO, error) {
	source, err := s.repo.GetByID(ctx, sourceID)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			return CampaignDTO{}, &NotFoundError{Msg: "campaign not found"}
		}
		return CampaignDTO{}, fmt.Errorf("campaign service: clone: %w", err)
	}

	clone := repositories.Campaign{
		Name:             fmt.Sprintf("Copy of %s", source.Name),
		Description:      source.Description,
		SendOrder:        source.SendOrder,
		GracePeriodHours: source.GracePeriodHours,
		Configuration:    source.Configuration,
		CreatedBy:        actorID,
	}

	if input.IncludeLandingPage {
		clone.LandingPageID = source.LandingPageID
	}
	if input.IncludeEndpointConfig {
		clone.CloudProvider = source.CloudProvider
		clone.Region = source.Region
		clone.InstanceType = source.InstanceType
		clone.EndpointDomainID = source.EndpointDomainID
	}

	created, err := s.repo.Create(ctx, clone)
	if err != nil {
		return CampaignDTO{}, fmt.Errorf("campaign service: clone: create: %w", err)
	}

	// Clone template variants if requested.
	if input.IncludeTemplateVariants {
		variants, err := s.repo.ListTemplateVariants(ctx, sourceID)
		if err == nil {
			for _, v := range variants {
				_, _ = s.repo.CreateTemplateVariant(ctx, repositories.CampaignTemplateVariant{
					CampaignID: created.ID, TemplateID: v.TemplateID,
					SplitRatio: v.SplitRatio, Label: v.Label,
				})
			}
		}
	}

	// Clone send windows if requested.
	if input.IncludeSendSchedule {
		windows, err := s.repo.ListSendWindows(ctx, sourceID)
		if err == nil {
			for _, w := range windows {
				_, _ = s.repo.CreateSendWindow(ctx, repositories.CampaignSendWindow{
					CampaignID: created.ID, Days: w.Days,
					StartTime: w.StartTime, EndTime: w.EndTime, Timezone: w.Timezone,
				})
			}
		}
	}

	resType := "campaign"
	details := map[string]any{"source_campaign_id": sourceID}
	s.logAudit(ctx, "campaign.cloned", &actorID, actorName, &resType, &created.ID, ip, correlationID, details)

	return toCampaignDTO(created), nil
}

// ---------- Campaign Config Templates ----------

// CreateConfigTemplate creates a reusable campaign config template.
func (s *Service) CreateConfigTemplate(ctx context.Context, input ConfigTemplateInput, actorID, actorName, ip, correlationID string) (ConfigTemplateDTO, error) {
	name := strings.TrimSpace(input.Name)
	if name == "" {
		return ConfigTemplateDTO{}, &ValidationError{Msg: "template name is required"}
	}

	created, err := s.repo.CreateConfigTemplate(ctx, repositories.CampaignConfigTemplate{
		Name: name, Description: input.Description, ConfigJSON: input.ConfigJSON, CreatedBy: actorID,
	})
	if err != nil {
		if strings.Contains(err.Error(), "idx_cct_name") {
			return ConfigTemplateDTO{}, &ConflictError{Msg: "a template with this name already exists"}
		}
		return ConfigTemplateDTO{}, fmt.Errorf("campaign service: create config template: %w", err)
	}

	resType := "campaign_config_template"
	s.logAudit(ctx, "campaign_template.created", &actorID, actorName, &resType, &created.ID, ip, correlationID, nil)
	return toConfigTemplateDTO(created), nil
}

// GetConfigTemplate returns a config template by ID.
func (s *Service) GetConfigTemplate(ctx context.Context, id string) (ConfigTemplateDTO, error) {
	t, err := s.repo.GetConfigTemplateByID(ctx, id)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			return ConfigTemplateDTO{}, &NotFoundError{Msg: "campaign config template not found"}
		}
		return ConfigTemplateDTO{}, fmt.Errorf("campaign service: get config template: %w", err)
	}
	return toConfigTemplateDTO(t), nil
}

// ListConfigTemplates returns all config templates.
func (s *Service) ListConfigTemplates(ctx context.Context) ([]ConfigTemplateDTO, error) {
	templates, err := s.repo.ListConfigTemplates(ctx)
	if err != nil {
		return nil, fmt.Errorf("campaign service: list config templates: %w", err)
	}
	dtos := make([]ConfigTemplateDTO, len(templates))
	for i, t := range templates {
		dtos[i] = toConfigTemplateDTO(t)
	}
	return dtos, nil
}

// UpdateConfigTemplate updates a config template.
func (s *Service) UpdateConfigTemplate(ctx context.Context, id string, input ConfigTemplateInput, actorID, actorName, ip, correlationID string) (ConfigTemplateDTO, error) {
	name := strings.TrimSpace(input.Name)
	if name == "" {
		return ConfigTemplateDTO{}, &ValidationError{Msg: "template name is required"}
	}

	updated, err := s.repo.UpdateConfigTemplate(ctx, id, name, input.Description, input.ConfigJSON)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			return ConfigTemplateDTO{}, &NotFoundError{Msg: "campaign config template not found"}
		}
		if strings.Contains(err.Error(), "idx_cct_name") {
			return ConfigTemplateDTO{}, &ConflictError{Msg: "a template with this name already exists"}
		}
		return ConfigTemplateDTO{}, fmt.Errorf("campaign service: update config template: %w", err)
	}

	resType := "campaign_config_template"
	s.logAudit(ctx, "campaign_template.updated", &actorID, actorName, &resType, &id, ip, correlationID, nil)
	return toConfigTemplateDTO(updated), nil
}

// DeleteConfigTemplate removes a config template.
func (s *Service) DeleteConfigTemplate(ctx context.Context, id string, actorID, actorName, ip, correlationID string) error {
	if err := s.repo.DeleteConfigTemplate(ctx, id); err != nil {
		if strings.Contains(err.Error(), "not found") {
			return &NotFoundError{Msg: "campaign config template not found"}
		}
		return fmt.Errorf("campaign service: delete config template: %w", err)
	}

	resType := "campaign_config_template"
	s.logAudit(ctx, "campaign_template.deleted", &actorID, actorName, &resType, &id, ip, correlationID, nil)
	return nil
}

// ApplyConfigTemplate creates a new campaign pre-populated from a template.
func (s *Service) ApplyConfigTemplate(ctx context.Context, templateID string, actorID, actorName, ip, correlationID string) (CampaignDTO, error) {
	tmpl, err := s.repo.GetConfigTemplateByID(ctx, templateID)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			return CampaignDTO{}, &NotFoundError{Msg: "campaign config template not found"}
		}
		return CampaignDTO{}, fmt.Errorf("campaign service: apply template: %w", err)
	}

	input := CreateInput{
		Name:          fmt.Sprintf("From template: %s", tmpl.Name),
		Description:   tmpl.Description,
		Configuration: tmpl.ConfigJSON,
	}

	return s.Create(ctx, input, actorID, actorName, ip, correlationID)
}

// ---------- A/B Variant Assignment ----------

// AssignVariants partitions targets across template variants based on split ratios.
// Assignment is deterministic: seeded by campaign_id + target_id.
func (s *Service) AssignVariants(ctx context.Context, campaignID string) error {
	variants, err := s.repo.ListTemplateVariants(ctx, campaignID)
	if err != nil {
		return fmt.Errorf("campaign service: assign variants: list variants: %w", err)
	}
	if len(variants) == 0 {
		return &ValidationError{Msg: "no template variants configured"}
	}

	snapshots, err := s.repo.ListTargetSnapshots(ctx, campaignID)
	if err != nil {
		return fmt.Errorf("campaign service: assign variants: list snapshots: %w", err)
	}
	if len(snapshots) == 0 {
		return &ValidationError{Msg: "no targets in snapshot"}
	}

	// Clear existing assignments.
	if err := s.repo.DeleteVariantAssignments(ctx, campaignID); err != nil {
		return fmt.Errorf("campaign service: assign variants: clear: %w", err)
	}

	// Sort snapshots by deterministic hash for reproducible assignment.
	sort.Slice(snapshots, func(i, j int) bool {
		return deterministicHash(campaignID, snapshots[i].TargetID) <
			deterministicHash(campaignID, snapshots[j].TargetID)
	})

	// Build variant boundaries based on split ratios.
	totalTargets := len(snapshots)
	assigned := 0
	for vi, v := range variants {
		var count int
		if vi == len(variants)-1 {
			count = totalTargets - assigned // last variant gets remainder
		} else {
			count = totalTargets * v.SplitRatio / 100
			if count == 0 && v.SplitRatio > 0 {
				count = 1 // ensure at least 1 target per variant with non-zero ratio
			}
		}

		for i := 0; i < count && assigned < totalTargets; i++ {
			snap := snapshots[assigned]
			if err := s.repo.CreateVariantAssignment(ctx, campaignID, snap.TargetID, v.ID); err != nil {
				return fmt.Errorf("campaign service: assign variants: assign: %w", err)
			}
			assigned++
		}
	}

	return nil
}

// ValidateSplitRatios validates that variant split ratios sum to exactly 100.
func ValidateSplitRatios(ratios []int) error {
	total := 0
	for _, r := range ratios {
		if r < 0 {
			return fmt.Errorf("split ratio cannot be negative: %d", r)
		}
		total += r
	}
	if total != 100 {
		return fmt.Errorf("split ratios must sum to 100, got %d", total)
	}
	return nil
}

// deterministicHash produces a reproducible uint64 from campaign+target IDs.
func deterministicHash(campaignID, targetID string) uint64 {
	h := sha256.Sum256([]byte(campaignID + ":" + targetID))
	return binary.BigEndian.Uint64(h[:8])
}

// ---------- Scheduled Auto-Launch ----------

// GetReadyForAutoLaunch returns campaigns ready for scheduled auto-launch.
func (s *Service) GetReadyForAutoLaunch(ctx context.Context) ([]CampaignDTO, error) {
	campaigns, err := s.repo.ListReadyForAutoLaunch(ctx)
	if err != nil {
		return nil, fmt.Errorf("campaign service: auto-launch list: %w", err)
	}
	dtos := make([]CampaignDTO, len(campaigns))
	for i, c := range campaigns {
		dtos[i] = toCampaignDTO(c)
	}
	return dtos, nil
}

// ---------- Concurrent Campaign Isolation ----------

// CheckConcurrentLimit checks if launching a new campaign would exceed the max concurrent limit.
func (s *Service) CheckConcurrentLimit(ctx context.Context, maxConcurrent int) error {
	if maxConcurrent <= 0 {
		return nil // no limit
	}
	count, err := s.repo.CountActiveCampaigns(ctx)
	if err != nil {
		return fmt.Errorf("campaign service: concurrent limit: %w", err)
	}
	if count >= maxConcurrent {
		return &ConflictError{Msg: fmt.Sprintf("maximum concurrent active campaigns reached (%d)", maxConcurrent)}
	}
	return nil
}

// ---------- Campaign Completion ----------

// CompleteAndCancel handles completion: cancels unsent emails.
func (s *Service) CompleteAndCancel(ctx context.Context, campaignID string) (int64, error) {
	cancelled, err := s.repo.CancelUnsentEmails(ctx, campaignID)
	if err != nil {
		return 0, fmt.Errorf("campaign service: complete and cancel: %w", err)
	}
	return cancelled, nil
}

// ListActiveCampaigns returns all campaigns in the 'active' state.
func (s *Service) ListActiveCampaigns(ctx context.Context) ([]CampaignDTO, error) {
	campaigns, err := s.repo.ListByState(ctx, string(campaign.StateActive))
	if err != nil {
		return nil, fmt.Errorf("campaign service: list active: %w", err)
	}
	dtos := make([]CampaignDTO, len(campaigns))
	for i, c := range campaigns {
		dtos[i] = toCampaignDTO(c)
	}
	return dtos, nil
}

// ---------- Email Delivery Hooks ----------

// handleEmailDeliveryHooks triggers email delivery actions after a successful
// state transition. Errors are logged but do not fail the transition since
// the state change has already been committed.
func (s *Service) handleEmailDeliveryHooks(ctx context.Context, campaignID string, from, to campaign.State, correlationID string) {
	_ = correlationID // available for future per-operation audit logging
	if s.emailDeliverySvc == nil {
		return
	}

	switch to {
	case campaign.StateActive:
		if from == campaign.StateReady {
			// Fresh launch: build queue then start sending.
			count, err := s.emailDeliverySvc.BuildQueue(ctx, campaignID)
			if err != nil {
				slog.Error("campaign: email delivery: build queue failed",
					"campaign_id", campaignID, "error", err)
				return
			}
			slog.Info("campaign: email delivery: queue built",
				"campaign_id", campaignID, "email_count", count)

			if err := s.emailDeliverySvc.StartSending(ctx, campaignID); err != nil {
				slog.Error("campaign: email delivery: start sending failed",
					"campaign_id", campaignID, "error", err)
			}
		} else if from == campaign.StatePaused {
			// Resume from pause.
			if err := s.emailDeliverySvc.Resume(ctx, campaignID); err != nil {
				slog.Error("campaign: email delivery: resume failed",
					"campaign_id", campaignID, "error", err)
			}
		}

	case campaign.StatePaused:
		if err := s.emailDeliverySvc.Pause(ctx, campaignID); err != nil {
			slog.Error("campaign: email delivery: pause failed",
				"campaign_id", campaignID, "error", err)
		}

	case campaign.StateCompleted:
		cancelled, err := s.emailDeliverySvc.CancelUnsentEmails(ctx, campaignID)
		if err != nil {
			slog.Error("campaign: email delivery: cancel unsent failed",
				"campaign_id", campaignID, "error", err)
		} else if cancelled > 0 {
			slog.Info("campaign: email delivery: cancelled unsent emails",
				"campaign_id", campaignID, "cancelled", cancelled)
		}
	}
}

// ---------- Infrastructure Teardown Hooks ----------

// handleTeardownHooks triggers infrastructure teardown after state transitions
// to completed, archived, or draft (unlock). Runs teardown in a goroutine to
// avoid blocking the transition response. Errors are logged but do not fail the transition.
func (s *Service) handleTeardownHooks(ctx context.Context, campaignID string, from, to campaign.State, correlationID string) {
	_ = correlationID // available for future per-operation audit logging
	if s.teardownSvc == nil {
		return
	}

	shouldTeardown := false

	switch to {
	case campaign.StateCompleted:
		shouldTeardown = true

	case campaign.StateArchived:
		// Double-check infrastructure is torn down when archiving.
		shouldTeardown = true

	case campaign.StateDraft:
		// Unlock transitions (T13 from Approved, T14 from Ready) need teardown
		// since infrastructure must be reprovisioned on next build.
		if from == campaign.StateApproved || from == campaign.StateReady {
			shouldTeardown = true
		}
	}

	if shouldTeardown {
		go func() {
			teardownCtx := context.Background()
			if err := s.teardownSvc.TeardownInfrastructure(teardownCtx, campaignID); err != nil {
				slog.Error("campaign: infrastructure teardown failed",
					"campaign_id", campaignID, "error", err)
			}
		}()
	}
}

// ---------- Helpers ----------

func validateSendOrder(order string) error {
	valid := map[string]bool{"default": true, "alphabetical": true, "department": true, "custom": true, "randomized": true}
	if !valid[order] {
		return &ValidationError{Msg: fmt.Sprintf("invalid send_order: %q; valid options: default, alphabetical, department, custom, randomized", order)}
	}
	return nil
}

func (s *Service) logAudit(ctx context.Context, action string, actorID *string, actorName string, resType, resID *string, ip, correlationID string, details map[string]any) {
	entry := auditsvc.LogEntry{
		Category:      auditsvc.CategoryUserActivity,
		Severity:      auditsvc.SeverityInfo,
		ActorType:     auditsvc.ActorTypeUser,
		ActorID:       actorID,
		ActorLabel:    actorName,
		Action:        action,
		ResourceType:  resType,
		ResourceID:    resID,
		Details:       details,
		CorrelationID: correlationID,
		SourceIP:      &ip,
	}
	if actorID == nil {
		entry.ActorType = auditsvc.ActorTypeSystem
		entry.ActorLabel = "system"
	}
	_ = s.auditSvc.Log(ctx, entry)
}
