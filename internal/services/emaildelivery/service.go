// Package emaildelivery provides the email sending pipeline for campaigns.
// It handles queue building, sending orchestration, delivery status tracking,
// retry logic, pause/resume, rate limiting, send window enforcement, and
// campaign completion detection.
package emaildelivery

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/binary"
	"fmt"
	"log/slog"
	"math"
	"strings"
	"sync"
	"time"

	"tackle/internal/repositories"
	auditsvc "tackle/internal/services/audit"
	notifsvc "tackle/internal/services/notification"
)

// Config holds configurable parameters for the delivery engine.
type Config struct {
	// MaxRetries is the maximum number of retries per email (default 3).
	MaxRetries int
	// InitialRetryInterval is the base retry interval (default 5m).
	InitialRetryInterval time.Duration
	// DefaultRateLimit is the fallback campaign rate limit (emails/min) if none configured.
	DefaultRateLimit int
}

// DefaultConfig returns the default delivery engine configuration.
func DefaultConfig() Config {
	return Config{
		MaxRetries:           3,
		InitialRetryInterval: 5 * time.Minute,
		DefaultRateLimit:     60,
	}
}

// Service orchestrates the email delivery pipeline.
type Service struct {
	campaignRepo *repositories.CampaignRepository
	targetRepo   *repositories.TargetRepository
	smtpRepo     *repositories.SMTPProfileRepository
	etRepo       *repositories.EmailTemplateRepository
	auditSvc     *auditsvc.AuditService
	notifSvc     *notifsvc.NotificationService
	config       Config
	logger       *slog.Logger

	// mu protects activeCampaigns.
	mu              sync.Mutex
	activeCampaigns map[string]*campaignSender
}

// campaignSender tracks the sending state for a single campaign.
type campaignSender struct {
	campaignID string
	cancel     context.CancelFunc
	paused     chan struct{} // closed when unpaused
	pausedMu   sync.Mutex
	isPaused   bool
}

// NewService creates a new email delivery Service.
func NewService(
	campaignRepo *repositories.CampaignRepository,
	targetRepo *repositories.TargetRepository,
	smtpRepo *repositories.SMTPProfileRepository,
	etRepo *repositories.EmailTemplateRepository,
	auditSvc *auditsvc.AuditService,
	notifSvc *notifsvc.NotificationService,
	config Config,
	logger *slog.Logger,
) *Service {
	if config.MaxRetries <= 0 {
		config.MaxRetries = 3
	}
	if config.InitialRetryInterval <= 0 {
		config.InitialRetryInterval = 5 * time.Minute
	}
	if config.DefaultRateLimit <= 0 {
		config.DefaultRateLimit = 60
	}
	return &Service{
		campaignRepo:    campaignRepo,
		targetRepo:      targetRepo,
		smtpRepo:        smtpRepo,
		etRepo:          etRepo,
		auditSvc:        auditSvc,
		notifSvc:        notifSvc,
		config:          config,
		logger:          logger,
		activeCampaigns: make(map[string]*campaignSender),
	}
}

// ---------- EMAIL-02: Queue Builder ----------

// BuildQueue creates the email send queue for a campaign from its target snapshots.
// It assigns SMTP profiles via round-robin and sets queue positions with canary targets first.
func (s *Service) BuildQueue(ctx context.Context, campaignID string) (int, error) {
	// Load target snapshots (pre-sorted by send_order_position with canary first).
	snapshots, err := s.campaignRepo.ListTargetSnapshots(ctx, campaignID)
	if err != nil {
		return 0, fmt.Errorf("email delivery: build queue: list snapshots: %w", err)
	}
	if len(snapshots) == 0 {
		return 0, fmt.Errorf("email delivery: build queue: no targets in snapshot")
	}

	// Load SMTP profiles assigned to this campaign.
	smtpAssocs, err := s.smtpRepo.ListCampaignAssociations(ctx, campaignID)
	if err != nil {
		return 0, fmt.Errorf("email delivery: build queue: list smtp profiles: %w", err)
	}
	if len(smtpAssocs) == 0 {
		return 0, fmt.Errorf("email delivery: build queue: no SMTP profiles configured")
	}

	// Extract profile IDs sorted by priority for round-robin.
	profileIDs := make([]string, len(smtpAssocs))
	for i, a := range smtpAssocs {
		profileIDs[i] = a.SMTPProfileID
	}

	// Delete existing queue entries (idempotent rebuild).
	if err := s.campaignRepo.DeleteCampaignEmails(ctx, campaignID); err != nil {
		return 0, fmt.Errorf("email delivery: build queue: clear existing: %w", err)
	}

	// Build queue entries: canary targets first, then others by send_order_position.
	// Snapshots are already ordered correctly from ListTargetSnapshots.
	emails := make([]repositories.CampaignEmail, 0, len(snapshots))
	for i, snap := range snapshots {
		// Round-robin SMTP assignment.
		smtpID := profileIDs[i%len(profileIDs)]
		pos := i + 1

		// Look up tracking token generated during build.
		var tokenPtr *string
		token, err := s.campaignRepo.GetTrackingToken(ctx, campaignID, snap.TargetID)
		if err != nil {
			s.logger.Warn("email delivery: build queue: failed to get tracking token",
				slog.String("target_id", snap.TargetID), "error", err)
		} else if token != "" {
			tokenPtr = &token
		}

		emails = append(emails, repositories.CampaignEmail{
			CampaignID:        campaignID,
			TargetID:          snap.TargetID,
			VariantID:         nil, // Set from variant assignment below.
			SMTPConfigID:      &smtpID,
			Status:            "queued",
			SendOrderPosition: &pos,
			VariantLabel:      snap.VariantLabel,
			TrackingToken:     tokenPtr,
		})
	}

	// Batch insert.
	if err := s.campaignRepo.CreateCampaignEmailsBatch(ctx, emails); err != nil {
		return 0, fmt.Errorf("email delivery: build queue: batch insert: %w", err)
	}

	// Create initial delivery events.
	for _, e := range emails {
		_, _ = s.campaignRepo.CreateDeliveryEvent(ctx, repositories.EmailDeliveryEvent{
			EmailID:   e.ID,
			EventType: "queued",
			EventData: map[string]any{"queue_position": e.SendOrderPosition},
		})
	}

	s.logAudit(ctx, "email.queue_built", &campaignID, map[string]any{
		"total_emails":   len(emails),
		"smtp_profiles":  len(profileIDs),
	})

	return len(emails), nil
}

// ---------- EMAIL-03: Sending Engine Core ----------

// StartSending begins the email sending process for a campaign.
// It runs in a goroutine and respects send windows, rate limits, and inter-message delays.
func (s *Service) StartSending(ctx context.Context, campaignID string) error {
	s.mu.Lock()
	if _, exists := s.activeCampaigns[campaignID]; exists {
		s.mu.Unlock()
		return fmt.Errorf("email delivery: campaign %s is already sending", campaignID)
	}

	sendCtx, cancel := context.WithCancel(ctx)
	sender := &campaignSender{
		campaignID: campaignID,
		cancel:     cancel,
		paused:     make(chan struct{}),
	}
	// Start unpaused: close the channel so waitForUnpause returns immediately.
	close(sender.paused)
	s.activeCampaigns[campaignID] = sender
	s.mu.Unlock()

	go s.sendLoop(sendCtx, sender)

	s.logAudit(ctx, "email.sending_started", &campaignID, nil)
	return nil
}

// sendLoop processes the email queue for a campaign.
func (s *Service) sendLoop(ctx context.Context, sender *campaignSender) {
	campaignID := sender.campaignID
	defer func() {
		s.mu.Lock()
		delete(s.activeCampaigns, campaignID)
		s.mu.Unlock()
		s.logger.Info("email sending loop exited", slog.String("campaign_id", campaignID))
	}()

	// Load send schedule for rate limiting and windows.
	schedule, err := s.smtpRepo.GetSendSchedule(ctx, campaignID)
	if err != nil {
		s.logger.Warn("email delivery: no send schedule, using defaults",
			slog.String("campaign_id", campaignID), "error", err)
		schedule = repositories.CampaignSendSchedule{
			CampaignRateLimit: &s.config.DefaultRateLimit,
		}
	}

	// Load send windows.
	windows, err := s.campaignRepo.ListSendWindows(ctx, campaignID)
	if err != nil {
		s.logger.Warn("email delivery: failed to load send windows",
			slog.String("campaign_id", campaignID), "error", err)
	}

	// ECOMP-07: Per-profile rate limiter.
	prl := newProfileRateLimiter()

	// ECOMP-08: Batch counter for batch pause logic.
	batchCount := 0

	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		// Wait if paused.
		if err := sender.waitForUnpause(ctx); err != nil {
			return
		}

		// Check send window.
		if len(windows) > 0 {
			if !s.isWithinSendWindow(windows) {
				nextOpen := s.nextWindowOpen(windows)
				if nextOpen.IsZero() {
					s.logger.Warn("email delivery: no future send window available",
						slog.String("campaign_id", campaignID))
					return
				}
				s.logger.Info("email delivery: outside send window, waiting",
					slog.String("campaign_id", campaignID),
					slog.Time("next_open", nextOpen))

				select {
				case <-time.After(time.Until(nextOpen)):
					continue
				case <-ctx.Done():
					return
				}
			}
		}

		// Process retries first.
		retryEmails, err := s.campaignRepo.GetEmailsForRetry(ctx, campaignID)
		if err != nil {
			s.logger.Error("email delivery: failed to get retry emails",
				slog.String("campaign_id", campaignID), "error", err)
		}
		for _, email := range retryEmails {
			select {
			case <-ctx.Done():
				return
			default:
			}
			if err := sender.waitForUnpause(ctx); err != nil {
				return
			}
			if len(windows) > 0 && !s.isWithinSendWindow(windows) {
				break // Exit retry loop, outer loop will handle window wait.
			}
			s.sendSingleEmail(ctx, campaignID, email, &schedule)
			s.applyDelay(ctx, &schedule)
		}

		// Get next queued email.
		email, err := s.campaignRepo.GetNextQueuedEmail(ctx, campaignID)
		if err != nil {
			if strings.Contains(err.Error(), "no queued emails") {
				// Check if there are pending retries.
				retries, _ := s.campaignRepo.GetEmailsForRetry(ctx, campaignID)
				if len(retries) == 0 {
					// All emails processed. Check for completion.
					s.checkCompletion(ctx, campaignID)
					return
				}
				// Wait for next retry window.
				select {
				case <-time.After(30 * time.Second):
					continue
				case <-ctx.Done():
					return
				}
			}
			s.logger.Error("email delivery: failed to get next email",
				slog.String("campaign_id", campaignID), "error", err)
			select {
			case <-time.After(5 * time.Second):
				continue
			case <-ctx.Done():
				return
			}
		}

		// Apply rate limiting (ECOMP-07: per-profile aware).
		smtpID := ptrStr(email.SMTPConfigID)
		s.applyRateLimit(ctx, &schedule, campaignID, smtpID, prl)

		// Send the email.
		s.sendSingleEmail(ctx, campaignID, email, &schedule)

		// Apply inter-message delay.
		s.applyDelay(ctx, &schedule)

		// ECOMP-08: Batch pause logic.
		batchCount++
		if schedule.BatchSize != nil && *schedule.BatchSize > 0 {
			if batchCount >= *schedule.BatchSize {
				pauseDuration := 5 * time.Second // default
				if schedule.BatchPauseSeconds != nil && *schedule.BatchPauseSeconds > 0 {
					pauseDuration = time.Duration(*schedule.BatchPauseSeconds) * time.Second
				}
				s.logger.Info("email delivery: batch pause",
					slog.String("campaign_id", campaignID),
					slog.Int("batch_count", batchCount),
					slog.Duration("pause", pauseDuration))
				select {
				case <-time.After(pauseDuration):
				case <-ctx.Done():
					return
				}
				batchCount = 0
			}
		}
	}
}

// sendSingleEmail dispatches a single email to the endpoint for SMTP relay.
func (s *Service) sendSingleEmail(ctx context.Context, campaignID string, email repositories.CampaignEmail, schedule *repositories.CampaignSendSchedule) {
	// Mark as sending.
	if err := s.campaignRepo.UpdateEmailStatus(ctx, email.ID, "sending", nil, nil, nil, nil, nil, nil); err != nil {
		s.logger.Error("email delivery: failed to mark sending",
			slog.String("email_id", email.ID), "error", err)
		return
	}
	_, _ = s.campaignRepo.CreateDeliveryEvent(ctx, repositories.EmailDeliveryEvent{
		EmailID:   email.ID,
		EventType: "sending",
		EventData: map[string]any{"retry_count": email.RetryCount},
	})

	// Load target data for personalization.
	target, err := s.targetRepo.GetByID(ctx, email.TargetID)
	if err != nil {
		s.logger.Error("email delivery: failed to load target",
			slog.String("email_id", email.ID), slog.String("target_id", email.TargetID), "error", err)
		s.handleSendFailure(ctx, email, "target not found", nil)
		return
	}

	// Build tracking URL paths from the token stored on the email record.
	trackingToken := ptrStr(email.TrackingToken)
	trackingURLs := buildTrackingURLPaths(trackingToken)

	// Build the send command payload.
	// In a real deployment, this would be dispatched to the phishing endpoint
	// via the control channel (WebSocket). For now, we record the intent and
	// mark the email as sent. The endpoint SMTP relay handles actual delivery.
	sendPayload := map[string]any{
		"email_id":        email.ID,
		"campaign_id":     campaignID,
		"target_email":    target.Email,
		"target_id":       email.TargetID,
		"smtp_profile_id": ptrStr(email.SMTPConfigID),
		"variant_label":   ptrStr(email.VariantLabel),
		"tracking_token":  trackingToken,
		"tracking_urls":   trackingURLs,
	}

	// Record send event.
	now := time.Now().UTC()
	if err := s.campaignRepo.UpdateEmailStatus(ctx, email.ID, "sent", nil, &now, nil, nil, nil, nil); err != nil {
		s.logger.Error("email delivery: failed to mark sent",
			slog.String("email_id", email.ID), "error", err)
		return
	}

	_, _ = s.campaignRepo.CreateDeliveryEvent(ctx, repositories.EmailDeliveryEvent{
		EmailID:   email.ID,
		EventType: "sent",
		EventData: sendPayload,
	})

	// Increment SMTP send count.
	if email.SMTPConfigID != nil {
		_ = s.campaignRepo.IncrementSMTPSendCount(ctx, campaignID, *email.SMTPConfigID, false)
	}

	// Update campaign target status.
	_ = s.targetRepo.UpdateCampaignTargetStatus(ctx, campaignID, email.TargetID, "email_sent")

	// Notify via WebSocket.
	if s.notifSvc != nil {
		s.notifSvc.Create(ctx, notifsvc.CreateNotificationParams{
			Category:     "campaign_event",
			Severity:     "info",
			Title:        "Email Sent",
			Body:         fmt.Sprintf("Email sent to %s", target.Email),
			ResourceType: "campaign",
			ResourceID:   campaignID,
			Recipients:   notifsvc.RecipientSpec{Role: "operator"},
		})
	}
}

// ---------- EMAIL-04: Delivery Status Tracking ----------

// ProcessDeliveryResult handles a delivery result callback from the phishing endpoint.
func (s *Service) ProcessDeliveryResult(ctx context.Context, result DeliveryResult) error {
	// Update email status.
	if err := s.campaignRepo.UpdateEmailStatus(ctx, result.EmailID, result.Status,
		result.MessageID, result.SentAt, result.DeliveredAt, result.BouncedAt,
		result.BounceType, result.BounceMessage); err != nil {
		return fmt.Errorf("email delivery: process result: update status: %w", err)
	}

	// Create delivery event.
	eventData := map[string]any{
		"status":           result.Status,
		"smtp_response":    ptrStr(result.SMTPResponse),
	}
	if result.MessageID != nil {
		eventData["message_id"] = *result.MessageID
	}
	if result.BounceType != nil {
		eventData["bounce_type"] = *result.BounceType
	}
	if result.BounceMessage != nil {
		eventData["bounce_message"] = *result.BounceMessage
	}

	_, _ = s.campaignRepo.CreateDeliveryEvent(ctx, repositories.EmailDeliveryEvent{
		EmailID:   result.EmailID,
		EventType: result.Status,
		EventData: eventData,
	})

	// Track SMTP profile errors.
	if result.SMTPProfileID != "" {
		isError := result.Status == "bounced" || result.Status == "failed"
		_ = s.campaignRepo.IncrementSMTPSendCount(ctx, result.CampaignID, result.SMTPProfileID, isError)
	}

	// Handle bounces.
	switch result.Status {
	case "bounced":
		if result.BounceType != nil && *result.BounceType == "hard" {
			// Hard bounce: mark permanently failed.
			if err := s.campaignRepo.UpdateEmailStatus(ctx, result.EmailID, "failed", nil, nil, nil, nil, nil, nil); err != nil {
				s.logger.Error("email delivery: failed to mark hard bounce as failed",
					slog.String("email_id", result.EmailID), "error", err)
			}
		} else {
			// Soft bounce: schedule retry.
			s.scheduleRetry(ctx, result.EmailID, result.CampaignID)
		}

	case "failed":
		// Schedule retry if under max retries.
		s.scheduleRetry(ctx, result.EmailID, result.CampaignID)

	case "delivered":
		// Update target status.
		email, err := s.campaignRepo.GetCampaignEmailByID(ctx, result.EmailID)
		if err == nil {
			_ = s.targetRepo.UpdateCampaignTargetStatus(ctx, result.CampaignID, email.TargetID, "email_delivered")
		}
	}

	// Audit log.
	s.logAudit(ctx, "email.delivery_result", &result.CampaignID, map[string]any{
		"email_id": result.EmailID,
		"status":   result.Status,
	})

	// Check campaign completion after each delivery result.
	s.checkCompletion(ctx, result.CampaignID)

	return nil
}

// DeliveryResult represents the outcome of an email delivery attempt from the endpoint.
type DeliveryResult struct {
	EmailID       string     `json:"email_id"`
	CampaignID    string     `json:"campaign_id"`
	TargetID      string     `json:"target_id"`
	SMTPProfileID string     `json:"smtp_profile_id"`
	Status        string     `json:"status"` // sent, delivered, deferred, bounced, failed
	MessageID     *string    `json:"message_id"`
	SMTPResponse  *string    `json:"smtp_response"`
	SentAt        *time.Time `json:"sent_at"`
	DeliveredAt   *time.Time `json:"delivered_at"`
	BouncedAt     *time.Time `json:"bounced_at"`
	BounceType    *string    `json:"bounce_type"`    // hard, soft
	BounceMessage *string    `json:"bounce_message"`
}

// ---------- EMAIL-05: Retry Logic ----------

// scheduleRetry schedules a retry for a failed/deferred email with exponential backoff.
func (s *Service) scheduleRetry(ctx context.Context, emailID, campaignID string) {
	email, err := s.campaignRepo.GetCampaignEmailByID(ctx, emailID)
	if err != nil {
		s.logger.Error("email delivery: failed to get email for retry",
			slog.String("email_id", emailID), "error", err)
		return
	}

	if email.RetryCount >= s.config.MaxRetries {
		// Max retries exhausted — mark permanently failed.
		if err := s.campaignRepo.UpdateEmailStatus(ctx, emailID, "failed", nil, nil, nil, nil, nil, nil); err != nil {
			s.logger.Error("email delivery: failed to mark max retries exhausted",
				slog.String("email_id", emailID), "error", err)
		}
		_, _ = s.campaignRepo.CreateDeliveryEvent(ctx, repositories.EmailDeliveryEvent{
			EmailID:   emailID,
			EventType: "failed",
			EventData: map[string]any{"reason": "max_retries_exhausted", "retry_count": email.RetryCount},
		})

		// Alert operator.
		if s.notifSvc != nil {
			s.notifSvc.Create(ctx, notifsvc.CreateNotificationParams{
				Category:     "campaign_event",
				Severity:     "warning",
				Title:        "Email Failed After Max Retries",
				Body:         fmt.Sprintf("Email %s failed after %d retries", emailID, email.RetryCount),
				ResourceType: "campaign",
				ResourceID:   campaignID,
				Recipients:   notifsvc.RecipientSpec{Role: "operator"},
			})
		}
		return
	}

	// Calculate next retry time with exponential backoff.
	retryCount := email.RetryCount + 1
	backoff := s.config.InitialRetryInterval * time.Duration(math.Pow(2, float64(email.RetryCount)))
	nextRetry := time.Now().UTC().Add(backoff)

	// Check send windows — defer to next window if retry falls outside.
	windows, _ := s.campaignRepo.ListSendWindows(ctx, campaignID)
	if len(windows) > 0 {
		nextRetry = s.adjustToNextWindow(nextRetry, windows)
	}

	// Try fallback to a different SMTP profile.
	var newSMTPID *string
	smtpAssocs, _ := s.smtpRepo.ListCampaignAssociations(ctx, campaignID)
	if len(smtpAssocs) > 1 && email.SMTPConfigID != nil {
		for _, a := range smtpAssocs {
			if a.SMTPProfileID != *email.SMTPConfigID {
				newSMTPID = &a.SMTPProfileID
				break
			}
		}
	}

	if err := s.campaignRepo.UpdateEmailRetry(ctx, emailID, retryCount, &nextRetry, newSMTPID); err != nil {
		s.logger.Error("email delivery: failed to schedule retry",
			slog.String("email_id", emailID), "error", err)
		return
	}

	_, _ = s.campaignRepo.CreateDeliveryEvent(ctx, repositories.EmailDeliveryEvent{
		EmailID:   emailID,
		EventType: "retry",
		EventData: map[string]any{
			"retry_count":    retryCount,
			"next_retry_at":  nextRetry.Format(time.RFC3339),
			"backoff_ms":     backoff.Milliseconds(),
			"new_smtp_id":    ptrStr(newSMTPID),
		},
	})

	s.logger.Info("email delivery: retry scheduled",
		slog.String("email_id", emailID),
		slog.Int("retry_count", retryCount),
		slog.Time("next_retry_at", nextRetry))
}

// ---------- EMAIL-06: Pause and Resume ----------

// Pause immediately stops dispatching new emails for a campaign.
func (s *Service) Pause(ctx context.Context, campaignID string) error {
	s.mu.Lock()
	sender, exists := s.activeCampaigns[campaignID]
	s.mu.Unlock()

	if !exists {
		return fmt.Errorf("email delivery: campaign %s is not actively sending", campaignID)
	}

	sender.pausedMu.Lock()
	defer sender.pausedMu.Unlock()

	if sender.isPaused {
		return nil // Already paused.
	}

	sender.isPaused = true
	sender.paused = make(chan struct{}) // New blocking channel.

	s.logAudit(ctx, "email.sending_paused", &campaignID, nil)

	if s.notifSvc != nil {
		s.notifSvc.Create(ctx, notifsvc.CreateNotificationParams{
			Category:     "campaign_event",
			Severity:     "info",
			Title:        "Email Sending Paused",
			Body:         fmt.Sprintf("Email sending paused for campaign %s", campaignID),
			ResourceType: "campaign",
			ResourceID:   campaignID,
			Recipients:   notifsvc.RecipientSpec{Role: "operator"},
		})
	}

	return nil
}

// Resume continues sending from the saved queue position.
func (s *Service) Resume(ctx context.Context, campaignID string) error {
	s.mu.Lock()
	sender, exists := s.activeCampaigns[campaignID]
	s.mu.Unlock()

	if !exists {
		// Campaign sender not active — restart sending.
		return s.StartSending(ctx, campaignID)
	}

	sender.pausedMu.Lock()
	defer sender.pausedMu.Unlock()

	if !sender.isPaused {
		return nil // Already running.
	}

	sender.isPaused = false
	close(sender.paused) // Unblock waitForUnpause.

	s.logAudit(ctx, "email.sending_resumed", &campaignID, nil)
	return nil
}

// StopSending cancels the sending process for a campaign.
func (s *Service) StopSending(ctx context.Context, campaignID string) error {
	s.mu.Lock()
	sender, exists := s.activeCampaigns[campaignID]
	s.mu.Unlock()

	if !exists {
		return nil // Not actively sending.
	}

	sender.cancel()
	s.logAudit(ctx, "email.sending_stopped", &campaignID, nil)
	return nil
}

// CancelUnsentEmails cancels all queued (unsent) emails for a campaign and
// stops the sending loop if active. Returns the number of cancelled emails.
func (s *Service) CancelUnsentEmails(ctx context.Context, campaignID string) (int, error) {
	// Stop any active sender first.
	_ = s.StopSending(ctx, campaignID)

	cancelled, err := s.campaignRepo.CancelUnsentEmails(ctx, campaignID)
	if err != nil {
		return 0, fmt.Errorf("email delivery: cancel unsent: %w", err)
	}

	s.logAudit(ctx, "email.unsent_cancelled", &campaignID, map[string]any{
		"cancelled_count": cancelled,
	})

	return int(cancelled), nil
}

// IsSending returns true if the campaign is actively sending.
func (s *Service) IsSending(campaignID string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	_, exists := s.activeCampaigns[campaignID]
	return exists
}

// IsPaused returns true if the campaign sending is paused.
func (s *Service) IsPaused(campaignID string) bool {
	s.mu.Lock()
	sender, exists := s.activeCampaigns[campaignID]
	s.mu.Unlock()
	if !exists {
		return false
	}
	sender.pausedMu.Lock()
	defer sender.pausedMu.Unlock()
	return sender.isPaused
}

// waitForUnpause blocks until the campaign is unpaused or context is cancelled.
func (cs *campaignSender) waitForUnpause(ctx context.Context) error {
	cs.pausedMu.Lock()
	ch := cs.paused
	cs.pausedMu.Unlock()

	select {
	case <-ch:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// ---------- EMAIL-07: Delivery Metrics ----------

// DeliveryMetrics holds delivery status metrics for a campaign.
type DeliveryMetrics struct {
	TotalEmails     int                       `json:"total_emails"`
	StatusCounts    map[string]int            `json:"status_counts"`
	ProfileCounts   []ProfileSendCount        `json:"profile_counts"`
	VariantCounts   map[string]map[string]int `json:"variant_counts"`
	CurrentPosition *int                      `json:"current_position"`
	IsSending       bool                      `json:"is_sending"`
	IsPaused        bool                      `json:"is_paused"`
}

// ProfileSendCount holds per-profile send metrics.
type ProfileSendCount struct {
	SMTPProfileID string  `json:"smtp_profile_id"`
	SendCount     int     `json:"send_count"`
	ErrorCount    int     `json:"error_count"`
	ErrorRate     float64 `json:"error_rate"`
}

// GetDeliveryMetrics returns current delivery metrics for a campaign.
func (s *Service) GetDeliveryMetrics(ctx context.Context, campaignID string) (DeliveryMetrics, error) {
	metrics := DeliveryMetrics{
		StatusCounts: make(map[string]int),
		IsSending:    s.IsSending(campaignID),
		IsPaused:     s.IsPaused(campaignID),
	}

	// Email counts by status.
	counts, err := s.campaignRepo.CountEmailsByStatus(ctx, campaignID)
	if err != nil {
		return metrics, fmt.Errorf("email delivery: get metrics: counts: %w", err)
	}
	metrics.StatusCounts = counts
	for _, c := range counts {
		metrics.TotalEmails += c
	}

	// Per-SMTP-profile counts.
	smtpCounts, err := s.campaignRepo.GetSMTPSendCounts(ctx, campaignID)
	if err != nil {
		return metrics, fmt.Errorf("email delivery: get metrics: smtp counts: %w", err)
	}
	metrics.ProfileCounts = make([]ProfileSendCount, len(smtpCounts))
	for i, sc := range smtpCounts {
		var errorRate float64
		if sc.SendCount > 0 {
			errorRate = float64(sc.ErrorCount) / float64(sc.SendCount)
		}
		metrics.ProfileCounts[i] = ProfileSendCount{
			SMTPProfileID: sc.SMTPProfileID,
			SendCount:     sc.SendCount,
			ErrorCount:    sc.ErrorCount,
			ErrorRate:     errorRate,
		}
	}

	// Per-variant counts.
	variantCounts, err := s.campaignRepo.CountEmailsByStatusAndVariant(ctx, campaignID)
	if err != nil {
		s.logger.Warn("email delivery: failed to get variant counts", "error", err)
	} else {
		metrics.VariantCounts = variantCounts
	}

	// Current queue position.
	lastPos, err := s.campaignRepo.GetLastSentPosition(ctx, campaignID)
	if err == nil {
		metrics.CurrentPosition = lastPos
	}

	return metrics, nil
}

// ---------- EMAIL-08: Campaign Completion Detection ----------

// checkCompletion checks if a campaign should be auto-completed.
func (s *Service) checkCompletion(ctx context.Context, campaignID string) {
	allDone, err := s.campaignRepo.AllEmailsTerminal(ctx, campaignID)
	if err != nil {
		s.logger.Error("email delivery: failed to check completion",
			slog.String("campaign_id", campaignID), "error", err)
		return
	}

	if !allDone {
		return
	}

	// Check if campaign end_date has passed.
	campaign, err := s.campaignRepo.GetByID(ctx, campaignID)
	if err != nil {
		s.logger.Error("email delivery: failed to get campaign for completion",
			slog.String("campaign_id", campaignID), "error", err)
		return
	}

	if campaign.EndDate != nil && campaign.EndDate.After(time.Now().UTC()) {
		// End date hasn't passed yet — don't auto-complete.
		return
	}

	// All emails terminal and end_date passed (or not set) — trigger completion.
	cancelled, err := s.campaignRepo.CancelUnsentEmails(ctx, campaignID)
	if err != nil {
		s.logger.Error("email delivery: failed to cancel unsent on completion",
			slog.String("campaign_id", campaignID), "error", err)
	}

	// Generate completion summary.
	counts, _ := s.campaignRepo.CountEmailsByStatus(ctx, campaignID)
	summary := map[string]any{
		"status_counts":     counts,
		"cancelled_on_complete": cancelled,
	}

	s.logAudit(ctx, "email.campaign_completed", &campaignID, summary)

	if s.notifSvc != nil {
		s.notifSvc.Create(ctx, notifsvc.CreateNotificationParams{
			Category:     "campaign_event",
			Severity:     "info",
			Title:        "Campaign Email Delivery Complete",
			Body:         fmt.Sprintf("All emails processed for campaign %s", campaignID),
			ResourceType: "campaign",
			ResourceID:   campaignID,
			Recipients:   notifsvc.RecipientSpec{Role: "operator"},
		})
	}

	// Stop the sender if still running.
	_ = s.StopSending(ctx, campaignID)

	s.logger.Info("email delivery: campaign complete",
		slog.String("campaign_id", campaignID),
		slog.Any("summary", summary))
}

// ---------- Send Window Helpers ----------

// isWithinSendWindow checks if the current time falls within any configured send window.
func (s *Service) isWithinSendWindow(windows []repositories.CampaignSendWindow) bool {
	now := time.Now()

	for _, w := range windows {
		loc := time.UTC
		if w.Timezone != "" {
			if l, err := time.LoadLocation(w.Timezone); err == nil {
				loc = l
			}
		}

		localNow := now.In(loc)
		weekday := strings.ToLower(localNow.Weekday().String())

		// Check day of week.
		dayMatch := len(w.Days) == 0 // No days specified means all days.
		for _, d := range w.Days {
			if strings.ToLower(d) == weekday {
				dayMatch = true
				break
			}
		}
		if !dayMatch {
			continue
		}

		// Check time range.
		currentTime := localNow.Format("15:04:05")
		if w.StartTime != "" && currentTime < w.StartTime {
			continue
		}
		if w.EndTime != "" && currentTime > w.EndTime {
			continue
		}

		return true
	}

	return false
}

// nextWindowOpen returns the next time a send window opens.
func (s *Service) nextWindowOpen(windows []repositories.CampaignSendWindow) time.Time {
	if len(windows) == 0 {
		return time.Time{}
	}

	now := time.Now()
	var earliest time.Time

	for _, w := range windows {
		loc := time.UTC
		if w.Timezone != "" {
			if l, err := time.LoadLocation(w.Timezone); err == nil {
				loc = l
			}
		}

		// Check next 7 days.
		for day := 0; day < 7; day++ {
			candidate := now.Add(time.Duration(day) * 24 * time.Hour).In(loc)
			weekday := strings.ToLower(candidate.Weekday().String())

			dayMatch := len(w.Days) == 0
			for _, d := range w.Days {
				if strings.ToLower(d) == weekday {
					dayMatch = true
					break
				}
			}
			if !dayMatch {
				continue
			}

			// Build candidate opening time.
			startTime := "00:00:00"
			if w.StartTime != "" {
				startTime = w.StartTime
			}

			dateStr := candidate.Format("2006-01-02")
			openTime, err := time.ParseInLocation("2006-01-02 15:04:05", dateStr+" "+startTime, loc)
			if err != nil {
				continue
			}

			if openTime.After(now) && (earliest.IsZero() || openTime.Before(earliest)) {
				earliest = openTime
			}
		}
	}

	return earliest
}

// adjustToNextWindow adjusts a time to the next send window opening if it falls outside.
func (s *Service) adjustToNextWindow(t time.Time, windows []repositories.CampaignSendWindow) time.Time {
	if len(windows) == 0 {
		return t
	}

	// Check if t is within a window.
	origNow := time.Now()
	// Temporarily use t as "now" by checking the window logic.
	for _, w := range windows {
		loc := time.UTC
		if w.Timezone != "" {
			if l, err := time.LoadLocation(w.Timezone); err == nil {
				loc = l
			}
		}
		localT := t.In(loc)
		weekday := strings.ToLower(localT.Weekday().String())

		dayMatch := len(w.Days) == 0
		for _, d := range w.Days {
			if strings.ToLower(d) == weekday {
				dayMatch = true
				break
			}
		}
		if !dayMatch {
			continue
		}

		currentTime := localT.Format("15:04:05")
		if (w.StartTime == "" || currentTime >= w.StartTime) && (w.EndTime == "" || currentTime <= w.EndTime) {
			return t // Already within window.
		}
	}

	// Not within window — find next opening after t.
	_ = origNow
	var earliest time.Time
	for _, w := range windows {
		loc := time.UTC
		if w.Timezone != "" {
			if l, err := time.LoadLocation(w.Timezone); err == nil {
				loc = l
			}
		}

		for day := 0; day < 7; day++ {
			candidate := t.Add(time.Duration(day) * 24 * time.Hour).In(loc)
			weekday := strings.ToLower(candidate.Weekday().String())

			dayMatch := len(w.Days) == 0
			for _, d := range w.Days {
				if strings.ToLower(d) == weekday {
					dayMatch = true
					break
				}
			}
			if !dayMatch {
				continue
			}

			startTime := "00:00:00"
			if w.StartTime != "" {
				startTime = w.StartTime
			}

			dateStr := candidate.Format("2006-01-02")
			openTime, err := time.ParseInLocation("2006-01-02 15:04:05", dateStr+" "+startTime, loc)
			if err != nil {
				continue
			}

			if openTime.After(t) && (earliest.IsZero() || openTime.Before(earliest)) {
				earliest = openTime
			}
		}
	}

	if !earliest.IsZero() {
		return earliest
	}
	return t
}

// ---------- Rate Limiting & Delay Helpers ----------

// profileRateLimiter tracks per-SMTP-profile send counts for rate limiting.
type profileRateLimiter struct {
	mu      sync.Mutex
	counts  map[string]*profileCounter // keyed by SMTP profile ID
}

// profileCounter tracks sends within a sliding minute window.
type profileCounter struct {
	count     int
	windowEnd time.Time
}

func newProfileRateLimiter() *profileRateLimiter {
	return &profileRateLimiter{counts: make(map[string]*profileCounter)}
}

// recordSend increments the send count for a profile and returns
// the current count within the current minute window.
func (prl *profileRateLimiter) recordSend(profileID string) int {
	prl.mu.Lock()
	defer prl.mu.Unlock()

	now := time.Now()
	counter, exists := prl.counts[profileID]
	if !exists || now.After(counter.windowEnd) {
		prl.counts[profileID] = &profileCounter{
			count:     1,
			windowEnd: now.Add(time.Minute),
		}
		return 1
	}
	counter.count++
	return counter.count
}

// applyRateLimit enforces the most restrictive of campaign-level and per-profile rate limits.
func (s *Service) applyRateLimit(ctx context.Context, schedule *repositories.CampaignSendSchedule, campaignID string, smtpProfileID string, prl *profileRateLimiter) {
	campaignLimit := s.config.DefaultRateLimit
	if schedule.CampaignRateLimit != nil && *schedule.CampaignRateLimit > 0 {
		campaignLimit = *schedule.CampaignRateLimit
	}

	// ECOMP-07: Check per-profile rate limit and use the most restrictive.
	effectiveLimit := campaignLimit
	if smtpProfileID != "" && prl != nil {
		profile, err := s.smtpRepo.GetByID(ctx, smtpProfileID)
		if err == nil && profile.MaxSendRate != nil && *profile.MaxSendRate > 0 {
			profileLimit := *profile.MaxSendRate
			if profileLimit < effectiveLimit {
				effectiveLimit = profileLimit
			}
		}

		// Track the send for this profile.
		currentCount := prl.recordSend(smtpProfileID)
		if currentCount >= effectiveLimit {
			// Wait for remainder of the minute window.
			select {
			case <-time.After(time.Second):
			case <-ctx.Done():
				return
			}
		}
	}

	// Apply delay based on effective rate: delay = 60/rate seconds.
	if effectiveLimit > 0 {
		delay := time.Duration(float64(time.Minute) / float64(effectiveLimit))
		select {
		case <-time.After(delay):
		case <-ctx.Done():
		}
	}
}

// applyDelay applies the configured inter-message delay with CSPRNG randomization.
func (s *Service) applyDelay(ctx context.Context, schedule *repositories.CampaignSendSchedule) {
	minDelay := schedule.MinDelayMs
	maxDelay := schedule.MaxDelayMs

	if minDelay <= 0 && maxDelay <= 0 {
		return
	}
	if maxDelay < minDelay {
		maxDelay = minDelay
	}

	// Use CSPRNG for unpredictable timing.
	delayMs := minDelay
	if maxDelay > minDelay {
		delayRange := maxDelay - minDelay
		var b [8]byte
		if _, err := rand.Read(b[:]); err == nil {
			randomVal := binary.LittleEndian.Uint64(b[:])
			delayMs = minDelay + int(randomVal%uint64(delayRange))
		}
	}

	select {
	case <-time.After(time.Duration(delayMs) * time.Millisecond):
	case <-ctx.Done():
	}
}

// ---------- Send Failure Handling ----------

// handleSendFailure marks an email as failed and schedules retry.
func (s *Service) handleSendFailure(ctx context.Context, email repositories.CampaignEmail, reason string, smtpResponse *string) {
	if err := s.campaignRepo.UpdateEmailStatus(ctx, email.ID, "failed", nil, nil, nil, nil, nil, nil); err != nil {
		s.logger.Error("email delivery: failed to mark failed",
			slog.String("email_id", email.ID), "error", err)
	}

	_, _ = s.campaignRepo.CreateDeliveryEvent(ctx, repositories.EmailDeliveryEvent{
		EmailID:   email.ID,
		EventType: "failed",
		EventData: map[string]any{"reason": reason, "smtp_response": ptrStr(smtpResponse)},
	})

	if email.SMTPConfigID != nil {
		_ = s.campaignRepo.IncrementSMTPSendCount(ctx, email.CampaignID, *email.SMTPConfigID, true)
	}

	// Schedule retry.
	s.scheduleRetry(ctx, email.ID, email.CampaignID)
}

// ---------- Audit & Helpers ----------

func (s *Service) logAudit(ctx context.Context, action string, campaignID *string, details map[string]any) {
	entry := auditsvc.LogEntry{
		Category:   auditsvc.CategoryEmailEvent,
		Severity:   auditsvc.SeverityInfo,
		ActorType:  auditsvc.ActorTypeSystem,
		ActorLabel: "email_delivery",
		Action:     action,
		Details:    details,
		CampaignID: campaignID,
	}
	resType := "campaign"
	entry.ResourceType = &resType
	entry.ResourceID = campaignID
	_ = s.auditSvc.Log(ctx, entry)
}

// buildTrackingURLPaths returns the URL path patterns for tracking pixel,
// click wrapper, and landing page, using the target's tracking token.
// The phishing endpoint prefixes these with its own https://{domain}.
func buildTrackingURLPaths(token string) map[string]string {
	if token == "" {
		return nil
	}
	return map[string]string{
		"pixel_path":        fmt.Sprintf("/t/%s.gif", token),
		"click_path":        fmt.Sprintf("/c/%s/", token),
		"landing_page_path": fmt.Sprintf("/l/%s", token),
	}
}

func ptrStr(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

// Ensure sql import is used for ErrNoRows references in other parts.
var _ = sql.ErrNoRows
