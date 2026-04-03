package campaign

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"

	"tackle/internal/campaign"
	"tackle/internal/repositories"
	auditsvc "tackle/internal/services/audit"
	blocklistsvc "tackle/internal/services/blocklist"
	notifsvc "tackle/internal/services/notification"
	targetgroupsvc "tackle/internal/services/targetgroup"
)

// ForbiddenError indicates the caller lacks required authorization.
type ForbiddenError struct{ Msg string }

func (e *ForbiddenError) Error() string { return e.Msg }

// ---------- Approval DTOs ----------

// ApprovalDTO is the API representation of an approval record.
type ApprovalDTO struct {
	ID                     string         `json:"id"`
	CampaignID             string         `json:"campaign_id"`
	SubmissionID           string         `json:"submission_id"`
	ActorID                string         `json:"actor_id"`
	Action                 string         `json:"action"`
	Comments               string         `json:"comments"`
	BlockListAcknowledged  bool           `json:"block_list_acknowledged"`
	BlockListJustification *string        `json:"block_list_justification,omitempty"`
	ConfigSnapshotJSON     map[string]any `json:"config_snapshot_json,omitempty"`
	CreatedAt              time.Time      `json:"created_at"`
}

// ApprovalRequirementDTO is the API representation of approval requirements.
type ApprovalRequirementDTO struct {
	CampaignID            string `json:"campaign_id"`
	SubmissionID          string `json:"submission_id"`
	RequiredApproverCount int    `json:"required_approver_count"`
	RequiresAdminApproval bool   `json:"requires_admin_approval"`
	CurrentApprovalCount  int    `json:"current_approval_count"`
}

// SubmitResult is the output of a campaign submission.
type SubmitResult struct {
	Campaign    CampaignDTO            `json:"campaign"`
	Requirement ApprovalRequirementDTO `json:"requirement"`
	BlockedTargets []BlockedTargetDTO  `json:"blocked_targets,omitempty"`
}

// BlockedTargetDTO represents a target that matches the block list.
type BlockedTargetDTO struct {
	TargetID string `json:"target_id"`
	Email    string `json:"email"`
	Pattern  string `json:"pattern"`
	Reason   string `json:"reason"`
}

// ApproveInput is the input for an approval action.
type ApproveInput struct {
	Comments string `json:"comments"`
}

// RejectInput is the input for a rejection action.
type RejectInput struct {
	Comments string `json:"comments"`
}

// BlocklistOverrideInput is the input for a block list override action.
type BlocklistOverrideInput struct {
	Action          string  `json:"action"` // "approve" or "reject"
	Acknowledged    bool    `json:"acknowledged"`
	Justification   string  `json:"justification"`
	RejectionReason *string `json:"rejection_reason,omitempty"`
}

// SMTPProfileSummaryDTO is a brief summary of an SMTP profile for approval review.
type SMTPProfileSummaryDTO struct {
	ID           string  `json:"id"`
	Name         string  `json:"name"`
	Host         string  `json:"host"`
	Port         int     `json:"port"`
	FromAddress  string  `json:"from_address"`
	Status       string  `json:"status"`
	Priority     int     `json:"priority"`
}

// LandingPageSummaryDTO is a brief summary of a landing page for approval review.
type LandingPageSummaryDTO struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// EndpointConfigSummaryDTO is the endpoint configuration for approval review.
type EndpointConfigSummaryDTO struct {
	CloudProvider string  `json:"cloud_provider"`
	Region        string  `json:"region"`
	InstanceType  string  `json:"instance_type"`
	DomainName    string  `json:"domain_name"`
	DomainStatus  string  `json:"domain_status"`
}

// ApprovalReviewDTO provides all data needed for an approver to review a campaign.
type ApprovalReviewDTO struct {
	Campaign           CampaignDTO              `json:"campaign"`
	TemplateVariants   []TemplateVariantDTO     `json:"template_variants"`
	SendWindows        []SendWindowDTO          `json:"send_windows"`
	Requirement        *ApprovalRequirementDTO  `json:"requirement,omitempty"`
	BlockedTargets     []BlockedTargetDTO       `json:"blocked_targets,omitempty"`
	TargetCount        int                      `json:"target_count"`
	SubmissionMetadata *ApprovalDTO             `json:"submission_metadata,omitempty"`
	RejectionHistory   []ApprovalDTO            `json:"rejection_history"`
	SMTPProfiles       []SMTPProfileSummaryDTO  `json:"smtp_profiles"`
	LandingPage        *LandingPageSummaryDTO   `json:"landing_page,omitempty"`
	EndpointConfig     *EndpointConfigSummaryDTO `json:"endpoint_config,omitempty"`
}

// ApprovalService handles the campaign approval workflow.
type ApprovalService struct {
	campaignRepo    *repositories.CampaignRepository
	approvalRepo    *repositories.CampaignApprovalRepository
	auditSvc        *auditsvc.AuditService
	notifSvc        *notifsvc.NotificationService
	blocklistSvc    *blocklistsvc.Service
	targetGroupSvc  *targetgroupsvc.Service
	smtpRepo        *repositories.SMTPProfileRepository
	domainRepo      *repositories.DomainProfileRepository
	landingPageRepo *repositories.LandingPageRepository
	campaignSvc     *Service
	maxConcurrent   int
}

// NewApprovalService creates a new ApprovalService.
func NewApprovalService(
	campaignRepo *repositories.CampaignRepository,
	approvalRepo *repositories.CampaignApprovalRepository,
	auditSvc *auditsvc.AuditService,
	notifSvc *notifsvc.NotificationService,
	blocklistSvc *blocklistsvc.Service,
	targetGroupSvc *targetgroupsvc.Service,
) *ApprovalService {
	return &ApprovalService{
		campaignRepo:   campaignRepo,
		approvalRepo:   approvalRepo,
		auditSvc:       auditSvc,
		notifSvc:       notifSvc,
		blocklistSvc:   blocklistSvc,
		targetGroupSvc: targetGroupSvc,
		maxConcurrent:  defaultMaxConcurrent,
	}
}

// SetValidationDeps sets additional dependencies needed for submission validation.
// Called after construction to break initialization cycles.
func (s *ApprovalService) SetValidationDeps(
	smtpRepo *repositories.SMTPProfileRepository,
	domainRepo *repositories.DomainProfileRepository,
	landingPageRepo *repositories.LandingPageRepository,
	campaignSvc *Service,
) {
	s.smtpRepo = smtpRepo
	s.domainRepo = domainRepo
	s.landingPageRepo = landingPageRepo
	s.campaignSvc = campaignSvc
}

// ---------- APPR-02: Campaign Submission ----------

// Submit validates campaign configuration and transitions from Draft to PendingApproval.
func (s *ApprovalService) Submit(ctx context.Context, campaignID string, requiredApproverCount int, actorID, actorName, actorRole, ip, correlationID string) (SubmitResult, error) {
	// Validate required approver count.
	if requiredApproverCount < 1 {
		requiredApproverCount = 1
	}
	if requiredApproverCount > 5 {
		return SubmitResult{}, &ValidationError{Msg: "required approver count must be between 1 and 5"}
	}

	// Begin transaction for atomic submit.
	tx, err := s.campaignRepo.BeginTx(ctx)
	if err != nil {
		return SubmitResult{}, fmt.Errorf("approval: submit: begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	// Lock campaign for update.
	locked, err := s.campaignRepo.GetByIDForUpdate(ctx, tx, campaignID)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			return SubmitResult{}, &NotFoundError{Msg: "campaign not found"}
		}
		return SubmitResult{}, fmt.Errorf("approval: submit: lock: %w", err)
	}

	// Validate current state is Draft.
	currentState := campaign.State(locked.CurrentState)
	_, err = campaign.ValidateTransition(currentState, campaign.StatePendingApproval, actorRole)
	if err != nil {
		if _, ok := err.(*campaign.StateError); ok {
			return SubmitResult{}, &ConflictError{Msg: err.Error()}
		}
		if _, ok := err.(*campaign.RoleError); ok {
			return SubmitResult{}, &ConflictError{Msg: err.Error()}
		}
		return SubmitResult{}, fmt.Errorf("approval: submit: validate transition: %w", err)
	}

	// Validate all required configuration fields (APPR-02).
	if err := s.validateSubmission(ctx, locked); err != nil {
		return SubmitResult{}, err
	}

	// Validate template variants exist and sum to 100.
	variants, err := s.campaignRepo.ListTemplateVariants(ctx, campaignID)
	if err != nil {
		return SubmitResult{}, fmt.Errorf("approval: submit: list variants: %w", err)
	}
	if len(variants) == 0 {
		return SubmitResult{}, &ValidationError{Msg: "at least one template variant is required before submission"}
	}
	totalRatio := 0
	for _, v := range variants {
		totalRatio += v.SplitRatio
	}
	if totalRatio != 100 {
		return SubmitResult{}, &ValidationError{Msg: fmt.Sprintf("template variant split_ratio values must sum to 100, got %d", totalRatio)}
	}

	// Resolve effective target list and check block list.
	resolution, err := s.targetGroupSvc.ResolveTargets(ctx, campaignID)
	if err != nil {
		return SubmitResult{}, fmt.Errorf("approval: submit: resolve targets: %w", err)
	}
	if resolution.TotalTargets == 0 {
		return SubmitResult{}, &ValidationError{Msg: "at least one target is required before submission"}
	}

	// Check all resolved targets against block list.
	emails := make([]string, 0, len(resolution.Targets))
	emailToTarget := make(map[string]targetgroupsvc.ResolvedTargetDTO, len(resolution.Targets))
	for _, t := range resolution.Targets {
		emails = append(emails, t.Email)
		emailToTarget[strings.ToLower(t.Email)] = t
	}

	blockResults, err := s.blocklistSvc.CheckEmails(ctx, emails)
	if err != nil {
		return SubmitResult{}, fmt.Errorf("approval: submit: check blocklist: %w", err)
	}

	var blockedTargets []BlockedTargetDTO
	requiresAdminApproval := false
	for email, result := range blockResults {
		if result.Blocked {
			requiresAdminApproval = true
			target := emailToTarget[strings.ToLower(email)]
			for _, match := range result.Matches {
				blockedTargets = append(blockedTargets, BlockedTargetDTO{
					TargetID: target.ID,
					Email:    email,
					Pattern:  match.Pattern,
					Reason:   match.Reason,
				})
			}
		}
	}

	// Build configuration snapshot.
	configSnapshot := s.buildConfigSnapshot(locked, variants)

	// Generate new submission ID.
	submissionID := uuid.New().String()

	// Create approval record.
	_, err = s.approvalRepo.CreateApprovalInTx(ctx, tx, repositories.CampaignApproval{
		CampaignID:         campaignID,
		SubmissionID:       submissionID,
		ActorID:            actorID,
		Action:             "submitted",
		Comments:           "",
		ConfigSnapshotJSON: configSnapshot,
	})
	if err != nil {
		return SubmitResult{}, fmt.Errorf("approval: submit: create approval record: %w", err)
	}

	// Create approval requirement.
	reqRecord, err := s.approvalRepo.CreateRequirement(ctx, repositories.CampaignApprovalRequirement{
		CampaignID:            campaignID,
		SubmissionID:          submissionID,
		RequiredApproverCount: requiredApproverCount,
		RequiresAdminApproval: requiresAdminApproval,
		CurrentApprovalCount:  0,
	})
	if err != nil {
		return SubmitResult{}, fmt.Errorf("approval: submit: create requirement: %w", err)
	}

	// If block list matches found, create override request via blocklist service.
	if requiresAdminApproval && len(blockedTargets) > 0 {
		blockedInfos := make([]repositories.BlockedTargetInfo, len(blockedTargets))
		for i, bt := range blockedTargets {
			blockedInfos[i] = repositories.BlockedTargetInfo{
				TargetID: bt.TargetID,
				Email:    bt.Email,
				Pattern:  bt.Pattern,
				Reason:   bt.Reason,
			}
		}
		if _, err := s.blocklistSvc.CreateOverride(ctx, campaignID, blockedInfos); err != nil {
			return SubmitResult{}, fmt.Errorf("approval: submit: create blocklist override: %w", err)
		}
	}

	// Transition state.
	actorIDPtr := &actorID
	if err := s.campaignRepo.TransitionState(ctx, tx, campaignID, string(currentState), string(campaign.StatePendingApproval), actorIDPtr, "submitted for approval"); err != nil {
		return SubmitResult{}, fmt.Errorf("approval: submit: transition: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return SubmitResult{}, fmt.Errorf("approval: submit: commit: %w", err)
	}

	// Re-fetch updated campaign.
	updated, err := s.campaignRepo.GetByID(ctx, campaignID)
	if err != nil {
		return SubmitResult{}, fmt.Errorf("approval: submit: re-fetch: %w", err)
	}

	// Audit log.
	resType := "campaign"
	details := map[string]any{
		"submission_id":          submissionID,
		"requires_admin":        requiresAdminApproval,
		"blocked_target_count":  len(blockedTargets),
		"target_count":          resolution.TotalTargets,
		"required_approver_count": requiredApproverCount,
	}
	s.logAudit(ctx, "campaign.submitted", actorIDPtr, actorName, &resType, &campaignID, ip, correlationID, details, auditsvc.SeverityInfo)

	// Notify eligible reviewers.
	s.notifySubmission(ctx, campaignID, locked.Name, actorName, requiresAdminApproval)

	return SubmitResult{
		Campaign: toCampaignDTO(updated),
		Requirement: ApprovalRequirementDTO{
			CampaignID:            reqRecord.CampaignID,
			SubmissionID:          reqRecord.SubmissionID,
			RequiredApproverCount: reqRecord.RequiredApproverCount,
			RequiresAdminApproval: reqRecord.RequiresAdminApproval,
			CurrentApprovalCount:  reqRecord.CurrentApprovalCount,
		},
		BlockedTargets: blockedTargets,
	}, nil
}

// validateSubmission checks all required fields and cross-references.
func (s *ApprovalService) validateSubmission(ctx context.Context, c repositories.Campaign) error {
	var errs []string
	if strings.TrimSpace(c.Name) == "" {
		errs = append(errs, "campaign name is required")
	}
	if c.StartDate == nil {
		errs = append(errs, "start_date is required")
	}
	if c.EndDate == nil {
		errs = append(errs, "end_date is required")
	}
	if c.StartDate != nil && c.EndDate != nil && !c.EndDate.After(*c.StartDate) {
		errs = append(errs, "end_date must be after start_date")
	}
	if c.CloudProvider == nil || *c.CloudProvider == "" {
		errs = append(errs, "cloud_provider is required")
	}
	if c.Region == nil || *c.Region == "" {
		errs = append(errs, "region is required")
	}
	if c.InstanceType == nil || *c.InstanceType == "" {
		errs = append(errs, "instance_type is required")
	}
	if c.EndpointDomainID == nil || *c.EndpointDomainID == "" {
		errs = append(errs, "endpoint domain is required")
	}

	// Landing page must be set and reference an existing project.
	if c.LandingPageID == nil || *c.LandingPageID == "" {
		errs = append(errs, "landing page is required")
	} else if s.landingPageRepo != nil {
		_, err := s.landingPageRepo.GetProjectByID(ctx, *c.LandingPageID)
		if err != nil {
			errs = append(errs, "landing page not found or deleted")
		}
	}

	// At least one SMTP profile must be associated and all must be healthy.
	if s.smtpRepo != nil {
		assocs, err := s.smtpRepo.ListCampaignAssociations(ctx, c.ID)
		if err != nil {
			errs = append(errs, fmt.Sprintf("failed to check SMTP profiles: %v", err))
		} else if len(assocs) == 0 {
			errs = append(errs, "at least one SMTP profile is required")
		} else {
			for _, a := range assocs {
				profile, err := s.smtpRepo.GetByID(ctx, a.SMTPProfileID)
				if err != nil {
					errs = append(errs, fmt.Sprintf("SMTP profile %s not found", a.SMTPProfileID))
					continue
				}
				if profile.Status != repositories.SMTPStatusHealthy {
					errs = append(errs, fmt.Sprintf("SMTP profile %q has status %q (must be healthy)", profile.Name, profile.Status))
				}
			}
		}
	}

	// Domain must be active.
	if c.EndpointDomainID != nil && *c.EndpointDomainID != "" && s.domainRepo != nil {
		domain, err := s.domainRepo.GetByID(ctx, *c.EndpointDomainID)
		if err != nil {
			errs = append(errs, "endpoint domain not found")
		} else if domain.Status != repositories.DomainStatusActive {
			errs = append(errs, fmt.Sprintf("endpoint domain must be active (current status: %s)", domain.Status))
		}
	}

	// Domain must not be in use by another active campaign.
	if c.EndpointDomainID != nil && *c.EndpointDomainID != "" && s.campaignRepo != nil {
		inUse, err := s.campaignRepo.IsDomainInUse(ctx, *c.EndpointDomainID, c.ID)
		if err != nil {
			errs = append(errs, fmt.Sprintf("failed to check domain availability: %v", err))
		} else if inUse {
			errs = append(errs, "domain is already in use by another active campaign")
		}
	}

	// Concurrent campaign limit.
	if s.campaignSvc != nil {
		if err := s.campaignSvc.CheckConcurrentLimit(ctx, s.maxConcurrent); err != nil {
			errs = append(errs, err.Error())
		}
	}

	if len(errs) > 0 {
		return &ValidationError{Msg: "submission validation failed: " + strings.Join(errs, "; ")}
	}
	return nil
}

// buildConfigSnapshot creates an immutable snapshot of the campaign config.
func (s *ApprovalService) buildConfigSnapshot(c repositories.Campaign, variants []repositories.CampaignTemplateVariant) map[string]any {
	variantInfo := make([]map[string]any, len(variants))
	for i, v := range variants {
		variantInfo[i] = map[string]any{
			"template_id": v.TemplateID,
			"split_ratio": v.SplitRatio,
			"label":       v.Label,
		}
	}

	return map[string]any{
		"name":              c.Name,
		"description":       c.Description,
		"start_date":        c.StartDate,
		"end_date":          c.EndDate,
		"landing_page_id":   c.LandingPageID,
		"cloud_provider":    c.CloudProvider,
		"region":            c.Region,
		"instance_type":     c.InstanceType,
		"endpoint_domain_id": c.EndpointDomainID,
		"throttle_rate":     c.ThrottleRate,
		"send_order":        c.SendOrder,
		"template_variants": variantInfo,
		"snapshotted_at":    time.Now().UTC().Format(time.RFC3339),
	}
}

// ---------- APPR-03: Approval Review Data API ----------

// GetApprovalReview returns all data needed for an approver to review a campaign.
func (s *ApprovalService) GetApprovalReview(ctx context.Context, campaignID string) (ApprovalReviewDTO, error) {
	c, err := s.campaignRepo.GetByID(ctx, campaignID)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			return ApprovalReviewDTO{}, &NotFoundError{Msg: "campaign not found"}
		}
		return ApprovalReviewDTO{}, fmt.Errorf("approval: review: get campaign: %w", err)
	}

	variants, err := s.campaignRepo.ListTemplateVariants(ctx, campaignID)
	if err != nil {
		return ApprovalReviewDTO{}, fmt.Errorf("approval: review: list variants: %w", err)
	}
	variantDTOs := make([]TemplateVariantDTO, len(variants))
	for i, v := range variants {
		variantDTOs[i] = toVariantDTO(v)
	}

	windows, err := s.campaignRepo.ListSendWindows(ctx, campaignID)
	if err != nil {
		return ApprovalReviewDTO{}, fmt.Errorf("approval: review: list windows: %w", err)
	}
	windowDTOs := make([]SendWindowDTO, len(windows))
	for i, w := range windows {
		windowDTOs[i] = toSendWindowDTO(w)
	}

	// Get target count.
	resolution, err := s.targetGroupSvc.ResolveTargets(ctx, campaignID)
	if err != nil {
		return ApprovalReviewDTO{}, fmt.Errorf("approval: review: resolve targets: %w", err)
	}

	// Check block list.
	emails := make([]string, 0, len(resolution.Targets))
	emailToTarget := make(map[string]targetgroupsvc.ResolvedTargetDTO, len(resolution.Targets))
	for _, t := range resolution.Targets {
		emails = append(emails, t.Email)
		emailToTarget[strings.ToLower(t.Email)] = t
	}

	var blockedTargets []BlockedTargetDTO
	if len(emails) > 0 {
		blockResults, err := s.blocklistSvc.CheckEmails(ctx, emails)
		if err != nil {
			return ApprovalReviewDTO{}, fmt.Errorf("approval: review: check blocklist: %w", err)
		}
		for email, result := range blockResults {
			if result.Blocked {
				target := emailToTarget[strings.ToLower(email)]
				for _, match := range result.Matches {
					blockedTargets = append(blockedTargets, BlockedTargetDTO{
						TargetID: target.ID,
						Email:    email,
						Pattern:  match.Pattern,
						Reason:   match.Reason,
					})
				}
			}
		}
	}

	// Get approval requirement (may not exist if not yet submitted).
	var reqDTO *ApprovalRequirementDTO
	subID, subErr := s.approvalRepo.GetLatestSubmissionID(ctx, campaignID)
	if subErr == nil {
		req, err := s.approvalRepo.GetRequirement(ctx, campaignID, subID)
		if err == nil {
			reqDTO = &ApprovalRequirementDTO{
				CampaignID:            req.CampaignID,
				SubmissionID:          req.SubmissionID,
				RequiredApproverCount: req.RequiredApproverCount,
				RequiresAdminApproval: req.RequiresAdminApproval,
				CurrentApprovalCount:  req.CurrentApprovalCount,
			}
		}
	}

	// Get submission metadata.
	var submissionMeta *ApprovalDTO
	if subErr == nil {
		approvals, err := s.approvalRepo.ListBySubmission(ctx, campaignID, subID)
		if err == nil {
			for _, a := range approvals {
				if a.Action == "submitted" {
					dto := toApprovalDTO(a)
					submissionMeta = &dto
					break
				}
			}
		}
	}

	// Get rejection history.
	rejections, err := s.approvalRepo.GetRejectionHistory(ctx, campaignID)
	if err != nil {
		return ApprovalReviewDTO{}, fmt.Errorf("approval: review: rejection history: %w", err)
	}
	rejectionDTOs := make([]ApprovalDTO, len(rejections))
	for i, r := range rejections {
		rejectionDTOs[i] = toApprovalDTO(r)
	}

	// Get SMTP profile summaries.
	var smtpSummaries []SMTPProfileSummaryDTO
	if s.smtpRepo != nil {
		assocs, assocErr := s.smtpRepo.ListCampaignAssociations(ctx, campaignID)
		if assocErr == nil {
			for _, a := range assocs {
				profile, pErr := s.smtpRepo.GetByID(ctx, a.SMTPProfileID)
				if pErr != nil {
					continue
				}
				smtpSummaries = append(smtpSummaries, SMTPProfileSummaryDTO{
					ID:          profile.ID,
					Name:        profile.Name,
					Host:        profile.Host,
					Port:        profile.Port,
					FromAddress: profile.FromAddress,
					Status:      string(profile.Status),
					Priority:    a.Priority,
				})
			}
		}
	}

	// Get landing page summary.
	var landingPage *LandingPageSummaryDTO
	cDTO := toCampaignDTO(c)
	if cDTO.LandingPageID != nil && s.landingPageRepo != nil {
		project, lpErr := s.landingPageRepo.GetProjectByID(ctx, *cDTO.LandingPageID)
		if lpErr == nil {
			landingPage = &LandingPageSummaryDTO{
				ID:   project.ID,
				Name: project.Name,
			}
		}
	}

	// Get endpoint config summary.
	var endpointConfig *EndpointConfigSummaryDTO
	if cDTO.CloudProvider != nil || cDTO.Region != nil || cDTO.InstanceType != nil {
		ec := EndpointConfigSummaryDTO{}
		if cDTO.CloudProvider != nil {
			ec.CloudProvider = *cDTO.CloudProvider
		}
		if cDTO.Region != nil {
			ec.Region = *cDTO.Region
		}
		if cDTO.InstanceType != nil {
			ec.InstanceType = *cDTO.InstanceType
		}
		if cDTO.EndpointDomainID != nil && s.domainRepo != nil {
			domain, dErr := s.domainRepo.GetByID(ctx, *cDTO.EndpointDomainID)
			if dErr == nil {
				ec.DomainName = domain.DomainName
				ec.DomainStatus = string(domain.Status)
			}
		}
		endpointConfig = &ec
	}

	return ApprovalReviewDTO{
		Campaign:           cDTO,
		TemplateVariants:   variantDTOs,
		SendWindows:        windowDTOs,
		Requirement:        reqDTO,
		BlockedTargets:     blockedTargets,
		TargetCount:        resolution.TotalTargets,
		SubmissionMetadata: submissionMeta,
		RejectionHistory:   rejectionDTOs,
		SMTPProfiles:       smtpSummaries,
		LandingPage:        landingPage,
		EndpointConfig:     endpointConfig,
	}, nil
}

// ---------- APPR-04: Approve Action ----------

// Approve processes an approval for a campaign in PendingApproval state.
func (s *ApprovalService) Approve(ctx context.Context, campaignID string, input ApproveInput, actorID, actorName, actorRole, ip, correlationID string) (CampaignDTO, error) {
	// Get the latest submission.
	subID, err := s.approvalRepo.GetLatestSubmissionID(ctx, campaignID)
	if err != nil {
		return CampaignDTO{}, &NotFoundError{Msg: "no pending submission found"}
	}

	// Check if actor already approved in this cycle.
	alreadyApproved, err := s.approvalRepo.HasApprovedInSubmission(ctx, campaignID, subID, actorID)
	if err != nil {
		return CampaignDTO{}, fmt.Errorf("approval: approve: check duplicate: %w", err)
	}
	if alreadyApproved {
		return CampaignDTO{}, &ConflictError{Msg: "you have already approved this submission"}
	}

	// Begin transaction.
	tx, err := s.campaignRepo.BeginTx(ctx)
	if err != nil {
		return CampaignDTO{}, fmt.Errorf("approval: approve: begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	// Lock campaign.
	locked, err := s.campaignRepo.GetByIDForUpdate(ctx, tx, campaignID)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			return CampaignDTO{}, &NotFoundError{Msg: "campaign not found"}
		}
		return CampaignDTO{}, fmt.Errorf("approval: approve: lock: %w", err)
	}

	if campaign.State(locked.CurrentState) != campaign.StatePendingApproval {
		return CampaignDTO{}, &ConflictError{Msg: fmt.Sprintf("campaign must be in pending_approval state; current state is %q", locked.CurrentState)}
	}

	// Check RBAC: if requires admin approval, only admin can approve.
	req, err := s.approvalRepo.GetRequirementForUpdate(ctx, tx, campaignID, subID)
	if err != nil {
		return CampaignDTO{}, fmt.Errorf("approval: approve: get requirement: %w", err)
	}

	if req.RequiresAdminApproval && actorRole != "admin" {
		return CampaignDTO{}, &ForbiddenError{Msg: "this campaign requires Administrator approval due to block list matches"}
	}

	// Record approval.
	_, err = s.approvalRepo.CreateApprovalInTx(ctx, tx, repositories.CampaignApproval{
		CampaignID:   campaignID,
		SubmissionID: subID,
		ActorID:      actorID,
		Action:       "approved",
		Comments:     input.Comments,
	})
	if err != nil {
		return CampaignDTO{}, fmt.Errorf("approval: approve: create record: %w", err)
	}

	// Increment approval count.
	updatedReq, err := s.approvalRepo.IncrementApprovalCount(ctx, tx, campaignID, subID)
	if err != nil {
		return CampaignDTO{}, fmt.Errorf("approval: approve: increment count: %w", err)
	}

	// If all required approvals received, transition to Approved.
	if updatedReq.CurrentApprovalCount >= updatedReq.RequiredApproverCount {
		if err := s.campaignRepo.TransitionState(ctx, tx, campaignID, string(campaign.StatePendingApproval), string(campaign.StateApproved), &actorID, "all required approvals received"); err != nil {
			return CampaignDTO{}, fmt.Errorf("approval: approve: transition: %w", err)
		}
		if err := s.campaignRepo.SetApproval(ctx, tx, campaignID, actorID, input.Comments); err != nil {
			return CampaignDTO{}, fmt.Errorf("approval: approve: set approval: %w", err)
		}
	}

	if err := tx.Commit(); err != nil {
		return CampaignDTO{}, fmt.Errorf("approval: approve: commit: %w", err)
	}

	// Re-fetch.
	updated, err := s.campaignRepo.GetByID(ctx, campaignID)
	if err != nil {
		return CampaignDTO{}, fmt.Errorf("approval: approve: re-fetch: %w", err)
	}

	// Audit log.
	resType := "campaign"
	details := map[string]any{
		"submission_id":    subID,
		"comments":         input.Comments,
		"approval_count":   updatedReq.CurrentApprovalCount,
		"required_count":   updatedReq.RequiredApproverCount,
		"fully_approved":   updatedReq.CurrentApprovalCount >= updatedReq.RequiredApproverCount,
	}
	s.logAudit(ctx, "campaign.approved", &actorID, actorName, &resType, &campaignID, ip, correlationID, details, auditsvc.SeverityCritical)

	// Notify operator.
	if updatedReq.CurrentApprovalCount >= updatedReq.RequiredApproverCount {
		s.notifyApproved(ctx, campaignID, locked.Name, locked.CreatedBy)
	} else {
		// Notify operator of partial progress.
		s.notifyApprovalProgress(ctx, campaignID, locked.Name, locked.CreatedBy, updatedReq.CurrentApprovalCount, updatedReq.RequiredApproverCount)
	}

	return toCampaignDTO(updated), nil
}

// ---------- APPR-05: Reject Action ----------

// Reject processes a rejection for a campaign in PendingApproval state.
func (s *ApprovalService) Reject(ctx context.Context, campaignID string, input RejectInput, actorID, actorName, actorRole, ip, correlationID string) (CampaignDTO, error) {
	if strings.TrimSpace(input.Comments) == "" {
		return CampaignDTO{}, &ValidationError{Msg: "rejection comments are mandatory"}
	}

	subID, err := s.approvalRepo.GetLatestSubmissionID(ctx, campaignID)
	if err != nil {
		return CampaignDTO{}, &NotFoundError{Msg: "no pending submission found"}
	}

	// Begin transaction.
	tx, err := s.campaignRepo.BeginTx(ctx)
	if err != nil {
		return CampaignDTO{}, fmt.Errorf("approval: reject: begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	// Lock campaign.
	locked, err := s.campaignRepo.GetByIDForUpdate(ctx, tx, campaignID)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			return CampaignDTO{}, &NotFoundError{Msg: "campaign not found"}
		}
		return CampaignDTO{}, fmt.Errorf("approval: reject: lock: %w", err)
	}

	if campaign.State(locked.CurrentState) != campaign.StatePendingApproval {
		return CampaignDTO{}, &ConflictError{Msg: fmt.Sprintf("campaign must be in pending_approval state; current state is %q", locked.CurrentState)}
	}

	// Check RBAC for block list campaigns.
	req, err := s.approvalRepo.GetRequirementForUpdate(ctx, tx, campaignID, subID)
	if err != nil {
		return CampaignDTO{}, fmt.Errorf("approval: reject: get requirement: %w", err)
	}
	if req.RequiresAdminApproval && actorRole != "admin" {
		return CampaignDTO{}, &ForbiddenError{Msg: "this campaign requires Administrator action due to block list matches"}
	}

	// Record rejection.
	_, err = s.approvalRepo.CreateApprovalInTx(ctx, tx, repositories.CampaignApproval{
		CampaignID:   campaignID,
		SubmissionID: subID,
		ActorID:      actorID,
		Action:       "rejected",
		Comments:     input.Comments,
	})
	if err != nil {
		return CampaignDTO{}, fmt.Errorf("approval: reject: create record: %w", err)
	}

	// Transition back to Draft — single rejection voids all.
	if err := s.campaignRepo.TransitionState(ctx, tx, campaignID, string(campaign.StatePendingApproval), string(campaign.StateDraft), &actorID, "rejected: "+input.Comments); err != nil {
		return CampaignDTO{}, fmt.Errorf("approval: reject: transition: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return CampaignDTO{}, fmt.Errorf("approval: reject: commit: %w", err)
	}

	// Re-fetch.
	updated, err := s.campaignRepo.GetByID(ctx, campaignID)
	if err != nil {
		return CampaignDTO{}, fmt.Errorf("approval: reject: re-fetch: %w", err)
	}

	// Audit log.
	resType := "campaign"
	details := map[string]any{
		"submission_id": subID,
		"comments":      input.Comments,
	}
	s.logAudit(ctx, "campaign.rejected", &actorID, actorName, &resType, &campaignID, ip, correlationID, details, auditsvc.SeverityInfo)

	// Notify operator of rejection.
	s.notifyRejected(ctx, campaignID, locked.Name, locked.CreatedBy, input.Comments)

	return toCampaignDTO(updated), nil
}

// ---------- APPR-06: Block List Override ----------

// ProcessBlocklistOverride handles Admin block list override approval or rejection.
func (s *ApprovalService) ProcessBlocklistOverride(ctx context.Context, campaignID string, input BlocklistOverrideInput, actorID, actorName, actorRole, ip, correlationID string) (CampaignDTO, error) {
	if actorRole != "admin" {
		return CampaignDTO{}, &ForbiddenError{Msg: "only Administrators can approve or reject block list overrides"}
	}

	if input.Action != "approve" && input.Action != "reject" {
		return CampaignDTO{}, &ValidationError{Msg: "action must be 'approve' or 'reject'"}
	}

	if input.Action == "approve" {
		if !input.Acknowledged {
			return CampaignDTO{}, &ValidationError{Msg: "block list acknowledgment is required"}
		}
		if strings.TrimSpace(input.Justification) == "" {
			return CampaignDTO{}, &ValidationError{Msg: "justification is required for block list override approval"}
		}
	}

	// Get campaign.
	c, err := s.campaignRepo.GetByID(ctx, campaignID)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			return CampaignDTO{}, &NotFoundError{Msg: "campaign not found"}
		}
		return CampaignDTO{}, fmt.Errorf("approval: blocklist override: get campaign: %w", err)
	}

	if campaign.State(c.CurrentState) != campaign.StatePendingApproval {
		return CampaignDTO{}, &ConflictError{Msg: "campaign must be in pending_approval state for block list override"}
	}

	subID, err := s.approvalRepo.GetLatestSubmissionID(ctx, campaignID)
	if err != nil {
		return CampaignDTO{}, &NotFoundError{Msg: "no pending submission found"}
	}

	// Record the override action.
	action := "blocklist_override_approved"
	if input.Action == "reject" {
		action = "blocklist_override_rejected"
	}

	_, err = s.approvalRepo.CreateApproval(ctx, repositories.CampaignApproval{
		CampaignID:             campaignID,
		SubmissionID:           subID,
		ActorID:                actorID,
		Action:                 action,
		Comments:               input.Justification,
		BlockListAcknowledged:  input.Acknowledged,
		BlockListJustification: &input.Justification,
	})
	if err != nil {
		return CampaignDTO{}, fmt.Errorf("approval: blocklist override: create record: %w", err)
	}

	// Get the existing override for this campaign and process it.
	override, err := s.blocklistSvc.GetOverride(ctx, campaignID)
	if err != nil {
		return CampaignDTO{}, fmt.Errorf("approval: blocklist override: get override: %w", err)
	}

	overrideInput := blocklistsvc.OverrideInput{
		Action:         input.Action,
		Acknowledgment: input.Acknowledged,
		Justification:  input.Justification,
	}
	if input.Action == "reject" && input.RejectionReason != nil {
		overrideInput.RejectionReason = input.RejectionReason
	}

	if input.Action == "approve" {
		if _, err := s.blocklistSvc.ApproveOverride(ctx, override.ID, overrideInput, actorID, actorName, ip, correlationID); err != nil {
			return CampaignDTO{}, fmt.Errorf("approval: blocklist override: approve: %w", err)
		}
	} else {
		if _, err := s.blocklistSvc.RejectOverride(ctx, override.ID, overrideInput, actorID, actorName, ip, correlationID); err != nil {
			return CampaignDTO{}, fmt.Errorf("approval: blocklist override: reject: %w", err)
		}
	}

	// If rejected, transition campaign back to draft.
	if input.Action == "reject" {
		tx, err := s.campaignRepo.BeginTx(ctx)
		if err != nil {
			return CampaignDTO{}, fmt.Errorf("approval: blocklist override reject: begin tx: %w", err)
		}
		defer func() { _ = tx.Rollback() }()

		if _, err := s.campaignRepo.GetByIDForUpdate(ctx, tx, campaignID); err != nil {
			return CampaignDTO{}, fmt.Errorf("approval: blocklist override reject: lock: %w", err)
		}

		reason := "block list override rejected"
		if input.RejectionReason != nil {
			reason += ": " + *input.RejectionReason
		}
		if err := s.campaignRepo.TransitionState(ctx, tx, campaignID, string(campaign.StatePendingApproval), string(campaign.StateDraft), &actorID, reason); err != nil {
			return CampaignDTO{}, fmt.Errorf("approval: blocklist override reject: transition: %w", err)
		}
		if err := tx.Commit(); err != nil {
			return CampaignDTO{}, fmt.Errorf("approval: blocklist override reject: commit: %w", err)
		}
	}

	// Re-fetch.
	updated, err := s.campaignRepo.GetByID(ctx, campaignID)
	if err != nil {
		return CampaignDTO{}, fmt.Errorf("approval: blocklist override: re-fetch: %w", err)
	}

	// Audit log with CRITICAL severity for block list overrides.
	resType := "campaign"
	severity := auditsvc.SeverityCritical
	details := map[string]any{
		"action":        input.Action,
		"acknowledged":  input.Acknowledged,
		"justification": input.Justification,
	}
	s.logAudit(ctx, "campaign.blocklist_override", &actorID, actorName, &resType, &campaignID, ip, correlationID, details, severity)

	// Notify campaign owner and admins.
	s.notifyBlocklistOverrideResult(ctx, campaignID, c.Name, c.CreatedBy, input.Action, actorName)

	return toCampaignDTO(updated), nil
}

// ---------- APPR-07: Unlock Action ----------

// Unlock transitions a campaign from Approved or Ready back to Draft.
func (s *ApprovalService) Unlock(ctx context.Context, campaignID string, reason string, actorID, actorName, actorRole, ip, correlationID string) (CampaignDTO, error) {
	tx, err := s.campaignRepo.BeginTx(ctx)
	if err != nil {
		return CampaignDTO{}, fmt.Errorf("approval: unlock: begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	locked, err := s.campaignRepo.GetByIDForUpdate(ctx, tx, campaignID)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			return CampaignDTO{}, &NotFoundError{Msg: "campaign not found"}
		}
		return CampaignDTO{}, fmt.Errorf("approval: unlock: lock: %w", err)
	}

	currentState := campaign.State(locked.CurrentState)
	_, err = campaign.ValidateTransition(currentState, campaign.StateDraft, actorRole)
	if err != nil {
		if _, ok := err.(*campaign.StateError); ok {
			return CampaignDTO{}, &ConflictError{Msg: err.Error()}
		}
		if _, ok := err.(*campaign.RoleError); ok {
			return CampaignDTO{}, &ConflictError{Msg: err.Error()}
		}
		return CampaignDTO{}, fmt.Errorf("approval: unlock: validate: %w", err)
	}

	// If Ready state, log teardown steps (infrastructure teardown placeholder).
	if currentState == campaign.StateReady {
		teardownSteps := []string{
			"terminate_phishing_endpoint",
			"release_static_ip",
			"revert_dns_records",
			"stop_landing_page",
			"cleanup_tls_certificates",
		}
		for i, step := range teardownSteps {
			now := time.Now()
			_, _ = s.campaignRepo.CreateBuildLog(ctx, repositories.CampaignBuildLog{
				CampaignID: campaignID,
				StepName:   "teardown_" + step,
				StepOrder:  i + 1,
				Status:     "completed",
				StartedAt:  &now,
				CompletedAt: &now,
			})
		}
	}

	// Record unlock in approval history.
	subID, _ := s.approvalRepo.GetLatestSubmissionID(ctx, campaignID)
	if subID == "" {
		subID = uuid.New().String() // fallback if no submission exists
	}
	_, _ = s.approvalRepo.CreateApprovalInTx(ctx, tx, repositories.CampaignApproval{
		CampaignID:   campaignID,
		SubmissionID: subID,
		ActorID:      actorID,
		Action:       "unlock",
		Comments:     reason,
	})

	// Transition to Draft (clears approved_by and approval_comment in TransitionState).
	unlockReason := "campaign unlocked for editing"
	if reason != "" {
		unlockReason = "unlocked: " + reason
	}
	if err := s.campaignRepo.TransitionState(ctx, tx, campaignID, string(currentState), string(campaign.StateDraft), &actorID, unlockReason); err != nil {
		return CampaignDTO{}, fmt.Errorf("approval: unlock: transition: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return CampaignDTO{}, fmt.Errorf("approval: unlock: commit: %w", err)
	}

	// Re-fetch.
	updated, err := s.campaignRepo.GetByID(ctx, campaignID)
	if err != nil {
		return CampaignDTO{}, fmt.Errorf("approval: unlock: re-fetch: %w", err)
	}

	// Audit log.
	resType := "campaign"
	details := map[string]any{
		"from_state":       string(currentState),
		"reason":           reason,
		"teardown_required": currentState == campaign.StateReady,
	}
	s.logAudit(ctx, "campaign.unlocked", &actorID, actorName, &resType, &campaignID, ip, correlationID, details, auditsvc.SeverityInfo)

	return toCampaignDTO(updated), nil
}

// ---------- APPR-05 continued: Approval History ----------

// GetApprovalHistory returns all approval records for a campaign.
func (s *ApprovalService) GetApprovalHistory(ctx context.Context, campaignID string) ([]ApprovalDTO, error) {
	records, err := s.approvalRepo.ListByCampaign(ctx, campaignID)
	if err != nil {
		return nil, fmt.Errorf("approval: history: %w", err)
	}
	dtos := make([]ApprovalDTO, len(records))
	for i, r := range records {
		dtos[i] = toApprovalDTO(r)
	}
	return dtos, nil
}

// ---------- Helpers ----------

func toApprovalDTO(a repositories.CampaignApproval) ApprovalDTO {
	return ApprovalDTO{
		ID:                     a.ID,
		CampaignID:             a.CampaignID,
		SubmissionID:           a.SubmissionID,
		ActorID:                a.ActorID,
		Action:                 a.Action,
		Comments:               a.Comments,
		BlockListAcknowledged:  a.BlockListAcknowledged,
		BlockListJustification: a.BlockListJustification,
		ConfigSnapshotJSON:     a.ConfigSnapshotJSON,
		CreatedAt:              a.CreatedAt,
	}
}

func (s *ApprovalService) logAudit(ctx context.Context, action string, actorID *string, actorName string, resType, resID *string, ip, correlationID string, details map[string]any, severity auditsvc.Severity) {
	entry := auditsvc.LogEntry{
		Category:      auditsvc.CategoryUserActivity,
		Severity:      severity,
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
	_ = s.auditSvc.Log(ctx, entry)
}

// ---------- APPR-09: Notifications ----------

func (s *ApprovalService) notifySubmission(ctx context.Context, campaignID, campaignName, submitterName string, requiresAdmin bool) {
	recipientRole := "engineer"
	if requiresAdmin {
		recipientRole = "admin"
	}
	s.notifSvc.Create(ctx, notifsvc.CreateNotificationParams{
		Category:     "campaign",
		Severity:     "info",
		Title:        "Campaign submitted for approval",
		Body:         fmt.Sprintf("%s submitted campaign %q for approval.", submitterName, campaignName),
		ResourceType: "campaign",
		ResourceID:   campaignID,
		ActionURL:    fmt.Sprintf("/campaigns/%s/approval-review", campaignID),
		Recipients:   notifsvc.RecipientSpec{Role: recipientRole},
	})
}

func (s *ApprovalService) notifyApproved(ctx context.Context, campaignID, campaignName, operatorID string) {
	s.notifSvc.Create(ctx, notifsvc.CreateNotificationParams{
		Category:     "campaign",
		Severity:     "info",
		Title:        "Campaign approved",
		Body:         fmt.Sprintf("Campaign %q has been fully approved and is ready to build.", campaignName),
		ResourceType: "campaign",
		ResourceID:   campaignID,
		ActionURL:    fmt.Sprintf("/campaigns/%s", campaignID),
		Recipients:   notifsvc.RecipientSpec{UserIDs: []string{operatorID}},
	})
}

func (s *ApprovalService) notifyRejected(ctx context.Context, campaignID, campaignName, operatorID, comments string) {
	s.notifSvc.Create(ctx, notifsvc.CreateNotificationParams{
		Category:     "campaign",
		Severity:     "warning",
		Title:        "Campaign rejected",
		Body:         fmt.Sprintf("Campaign %q was rejected: %s", campaignName, comments),
		ResourceType: "campaign",
		ResourceID:   campaignID,
		ActionURL:    fmt.Sprintf("/campaigns/%s", campaignID),
		Recipients:   notifsvc.RecipientSpec{UserIDs: []string{operatorID}},
	})
}

func (s *ApprovalService) notifyApprovalProgress(ctx context.Context, campaignID, campaignName, operatorID string, current, required int) {
	s.notifSvc.Create(ctx, notifsvc.CreateNotificationParams{
		Category:     "campaign",
		Severity:     "info",
		Title:        "Campaign approval progress",
		Body:         fmt.Sprintf("Campaign %q: %d of %d approvals received.", campaignName, current, required),
		ResourceType: "campaign",
		ResourceID:   campaignID,
		ActionURL:    fmt.Sprintf("/campaigns/%s/approval-review", campaignID),
		Recipients:   notifsvc.RecipientSpec{UserIDs: []string{operatorID}},
	})
}

func (s *ApprovalService) notifyBlocklistOverrideResult(ctx context.Context, campaignID, campaignName, operatorID, action, adminName string) {
	severity := "info"
	title := "Block list override approved"
	body := fmt.Sprintf("Administrator %s approved the block list override for campaign %q.", adminName, campaignName)
	if action == "reject" {
		severity = "warning"
		title = "Block list override rejected"
		body = fmt.Sprintf("Administrator %s rejected the block list override for campaign %q.", adminName, campaignName)
	}

	// Notify both operator and all admins.
	s.notifSvc.Create(ctx, notifsvc.CreateNotificationParams{
		Category:     "campaign",
		Severity:     severity,
		Title:        title,
		Body:         body,
		ResourceType: "campaign",
		ResourceID:   campaignID,
		ActionURL:    fmt.Sprintf("/campaigns/%s", campaignID),
		Recipients:   notifsvc.RecipientSpec{UserIDs: []string{operatorID}},
	})
	s.notifSvc.Create(ctx, notifsvc.CreateNotificationParams{
		Category:     "campaign",
		Severity:     severity,
		Title:        title,
		Body:         body,
		ResourceType: "campaign",
		ResourceID:   campaignID,
		ActionURL:    fmt.Sprintf("/campaigns/%s", campaignID),
		Recipients:   notifsvc.RecipientSpec{Role: "admin"},
	})
}
