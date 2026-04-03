// Package blocklist implements business logic for the global block list.
package blocklist

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	"tackle/internal/repositories"
	auditsvc "tackle/internal/services/audit"
	notifsvc "tackle/internal/services/notification"
)

// ValidationError is returned when input fails validation.
type ValidationError struct{ Msg string }

// Error implements the error interface.
func (e *ValidationError) Error() string { return e.Msg }

// NotFoundError is returned when a resource is not found.
type NotFoundError struct{ Msg string }

// Error implements the error interface.
func (e *NotFoundError) Error() string { return e.Msg }

// ConflictError is returned when a uniqueness constraint would be violated.
type ConflictError struct{ Msg string }

// Error implements the error interface.
func (e *ConflictError) Error() string { return e.Msg }

// EntryDTO is the API-safe representation of a block list entry.
type EntryDTO struct {
	ID       string `json:"id"`
	Pattern  string `json:"pattern"`
	Reason   string `json:"reason"`
	IsActive bool   `json:"is_active"`
	AddedBy  string `json:"added_by"`
	AddedAt  string `json:"added_at"`
}

// CheckResultDTO shows matching block list entries for an email.
type CheckResultDTO struct {
	Email   string     `json:"email"`
	Blocked bool       `json:"blocked"`
	Matches []EntryDTO `json:"matches"`
}

// OverrideDTO is the API-safe representation of a block list override.
type OverrideDTO struct {
	ID              string                         `json:"id"`
	CampaignID      string                         `json:"campaign_id"`
	Status          string                         `json:"status"`
	BlockedTargets  []repositories.BlockedTargetInfo `json:"blocked_targets"`
	Acknowledgment  bool                           `json:"acknowledgment"`
	Justification   *string                        `json:"justification,omitempty"`
	RejectionReason *string                        `json:"rejection_reason,omitempty"`
	DecidedBy       *string                        `json:"decided_by,omitempty"`
	DecidedAt       *string                        `json:"decided_at,omitempty"`
	CreatedAt       string                         `json:"created_at"`
}

// OverrideInput holds the input for approving or rejecting an override.
type OverrideInput struct {
	Action          string  `json:"action"` // "approve" or "reject"
	Acknowledgment  bool    `json:"acknowledgment"`
	Justification   string  `json:"justification"`
	RejectionReason *string `json:"rejection_reason,omitempty"`
}

// CreateInput holds fields for creating a block list entry.
type CreateInput struct {
	Pattern string `json:"pattern"`
	Reason  string `json:"reason"`
}

// Service implements block list business logic with in-memory pattern cache.
type Service struct {
	repo         *repositories.BlocklistRepository
	overrideRepo *repositories.BlocklistOverrideRepository
	auditSvc     *auditsvc.AuditService
	notifSvc     *notifsvc.NotificationService

	// In-memory cache for fast pattern matching.
	mu            sync.RWMutex
	cachedEntries []repositories.BlocklistEntry
	cacheLoaded   bool
}

// NewService creates a new block list Service.
func NewService(
	repo *repositories.BlocklistRepository,
	overrideRepo *repositories.BlocklistOverrideRepository,
	auditSvc *auditsvc.AuditService,
	notifSvc *notifsvc.NotificationService,
) *Service {
	return &Service{
		repo:         repo,
		overrideRepo: overrideRepo,
		auditSvc:     auditSvc,
		notifSvc:     notifSvc,
	}
}

// Create adds a new block list entry.
func (s *Service) Create(ctx context.Context, input CreateInput, actorID, actorName, ip, correlationID string) (EntryDTO, error) {
	pattern := strings.TrimSpace(strings.ToLower(input.Pattern))
	reason := strings.TrimSpace(input.Reason)

	if err := validatePattern(pattern); err != nil {
		return EntryDTO{}, err
	}
	if reason == "" {
		return EntryDTO{}, &ValidationError{Msg: "reason is required"}
	}
	if len(reason) > 2048 {
		return EntryDTO{}, &ValidationError{Msg: "reason must be 2048 characters or fewer"}
	}

	// Check uniqueness.
	_, err := s.repo.GetByPattern(ctx, pattern)
	if err == nil {
		return EntryDTO{}, &ConflictError{Msg: "a block list entry with this pattern already exists"}
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return EntryDTO{}, fmt.Errorf("check pattern: %w", err)
	}

	entry, err := s.repo.Create(ctx, repositories.BlocklistEntry{
		Pattern: pattern,
		Reason:  reason,
		AddedBy: actorID,
	})
	if err != nil {
		return EntryDTO{}, fmt.Errorf("create entry: %w", err)
	}

	s.invalidateCache()

	resourceType := "blocklist_entry"
	_ = s.auditSvc.Log(ctx, auditsvc.LogEntry{
		Category:      auditsvc.CategoryUserActivity,
		Severity:      auditsvc.SeverityWarning,
		ActorType:     auditsvc.ActorTypeUser,
		ActorID:       &actorID,
		ActorLabel:    actorName,
		Action:        "blocklist.create",
		ResourceType:  &resourceType,
		ResourceID:    &entry.ID,
		Details:       map[string]any{"pattern": pattern, "reason": reason},
		CorrelationID: correlationID,
		SourceIP:      &ip,
	})

	return entryToDTO(entry), nil
}

// Get retrieves a block list entry by ID.
func (s *Service) Get(ctx context.Context, id string) (EntryDTO, error) {
	entry, err := s.repo.GetByID(ctx, id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return EntryDTO{}, &NotFoundError{Msg: "block list entry not found"}
		}
		return EntryDTO{}, fmt.Errorf("get entry: %w", err)
	}
	return entryToDTO(entry), nil
}

// List returns block list entries filtered and paginated.
func (s *Service) List(ctx context.Context, f repositories.BlocklistFilters) ([]EntryDTO, int, error) {
	entries, total, err := s.repo.List(ctx, f)
	if err != nil {
		return nil, 0, fmt.Errorf("list entries: %w", err)
	}

	dtos := make([]EntryDTO, 0, len(entries))
	for _, e := range entries {
		dtos = append(dtos, entryToDTO(e))
	}
	return dtos, total, nil
}

// Deactivate deactivates a block list entry.
func (s *Service) Deactivate(ctx context.Context, id, actorID, actorName, ip, correlationID string) (EntryDTO, error) {
	entry, err := s.repo.Deactivate(ctx, id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return EntryDTO{}, &NotFoundError{Msg: "block list entry not found"}
		}
		return EntryDTO{}, fmt.Errorf("deactivate entry: %w", err)
	}

	s.invalidateCache()

	resourceType := "blocklist_entry"
	_ = s.auditSvc.Log(ctx, auditsvc.LogEntry{
		Category:      auditsvc.CategoryUserActivity,
		Severity:      auditsvc.SeverityWarning,
		ActorType:     auditsvc.ActorTypeUser,
		ActorID:       &actorID,
		ActorLabel:    actorName,
		Action:        "blocklist.deactivate",
		ResourceType:  &resourceType,
		ResourceID:    &id,
		Details:       map[string]any{"pattern": entry.Pattern},
		CorrelationID: correlationID,
		SourceIP:      &ip,
	})

	return entryToDTO(entry), nil
}

// Reactivate reactivates a block list entry.
func (s *Service) Reactivate(ctx context.Context, id, actorID, actorName, ip, correlationID string) (EntryDTO, error) {
	entry, err := s.repo.Reactivate(ctx, id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return EntryDTO{}, &NotFoundError{Msg: "block list entry not found"}
		}
		return EntryDTO{}, fmt.Errorf("reactivate entry: %w", err)
	}

	s.invalidateCache()

	resourceType := "blocklist_entry"
	_ = s.auditSvc.Log(ctx, auditsvc.LogEntry{
		Category:      auditsvc.CategoryUserActivity,
		Severity:      auditsvc.SeverityWarning,
		ActorType:     auditsvc.ActorTypeUser,
		ActorID:       &actorID,
		ActorLabel:    actorName,
		Action:        "blocklist.reactivate",
		ResourceType:  &resourceType,
		ResourceID:    &id,
		Details:       map[string]any{"pattern": entry.Pattern},
		CorrelationID: correlationID,
		SourceIP:      &ip,
	})

	return entryToDTO(entry), nil
}

// CheckEmail checks a single email against the active block list.
func (s *Service) CheckEmail(ctx context.Context, email string) (CheckResultDTO, error) {
	email = strings.ToLower(strings.TrimSpace(email))
	if email == "" {
		return CheckResultDTO{}, &ValidationError{Msg: "email is required"}
	}

	entries, err := s.getActiveEntries(ctx)
	if err != nil {
		return CheckResultDTO{}, err
	}

	matches := matchEmail(email, entries)
	dtos := make([]EntryDTO, 0, len(matches))
	for _, m := range matches {
		dtos = append(dtos, entryToDTO(m))
	}

	return CheckResultDTO{
		Email:   email,
		Blocked: len(matches) > 0,
		Matches: dtos,
	}, nil
}

// CheckEmails checks multiple emails against the active block list.
func (s *Service) CheckEmails(ctx context.Context, emails []string) (map[string]CheckResultDTO, error) {
	entries, err := s.getActiveEntries(ctx)
	if err != nil {
		return nil, err
	}

	results := make(map[string]CheckResultDTO, len(emails))
	for _, email := range emails {
		email = strings.ToLower(strings.TrimSpace(email))
		matches := matchEmail(email, entries)
		dtos := make([]EntryDTO, 0, len(matches))
		for _, m := range matches {
			dtos = append(dtos, entryToDTO(m))
		}
		results[email] = CheckResultDTO{
			Email:   email,
			Blocked: len(matches) > 0,
			Matches: dtos,
		}
	}
	return results, nil
}

// --- Override Workflow ---

// CreateOverride creates a pending override request for a campaign with blocked targets.
func (s *Service) CreateOverride(ctx context.Context, campaignID string, blockedTargets []repositories.BlockedTargetInfo) (OverrideDTO, error) {
	// Invalidate any existing pending overrides.
	if err := s.overrideRepo.InvalidatePending(ctx, campaignID); err != nil {
		return OverrideDTO{}, fmt.Errorf("invalidate pending overrides: %w", err)
	}

	hash := computeTargetHash(blockedTargets)

	override, err := s.overrideRepo.Create(ctx, repositories.BlocklistOverride{
		CampaignID:     campaignID,
		BlockedTargets: blockedTargets,
		TargetHash:     hash,
	})
	if err != nil {
		return OverrideDTO{}, fmt.Errorf("create override: %w", err)
	}

	return overrideToDTO(override), nil
}

// ListOverrides returns all override requests, optionally filtered to pending only.
func (s *Service) ListOverrides(ctx context.Context, pendingOnly bool) ([]OverrideDTO, error) {
	var overrides []repositories.BlocklistOverride
	var err error
	if pendingOnly {
		overrides, err = s.overrideRepo.ListPending(ctx)
	} else {
		overrides, err = s.overrideRepo.ListAll(ctx)
	}
	if err != nil {
		return nil, fmt.Errorf("list overrides: %w", err)
	}
	dtos := make([]OverrideDTO, 0, len(overrides))
	for _, o := range overrides {
		dtos = append(dtos, overrideToDTO(o))
	}
	return dtos, nil
}

// GetOverride returns the current override for a campaign.
func (s *Service) GetOverride(ctx context.Context, campaignID string) (OverrideDTO, error) {
	override, err := s.overrideRepo.GetByCampaignID(ctx, campaignID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return OverrideDTO{}, &NotFoundError{Msg: "no override found for campaign"}
		}
		return OverrideDTO{}, fmt.Errorf("get override: %w", err)
	}
	return overrideToDTO(override), nil
}

// ApproveOverride approves a block list override.
func (s *Service) ApproveOverride(ctx context.Context, overrideID string, input OverrideInput, actorID, actorName, ip, correlationID string) (OverrideDTO, error) {
	if !input.Acknowledgment {
		return OverrideDTO{}, &ValidationError{Msg: "acknowledgment is required"}
	}
	if strings.TrimSpace(input.Justification) == "" {
		return OverrideDTO{}, &ValidationError{Msg: "justification is required"}
	}

	existing, err := s.overrideRepo.GetByID(ctx, overrideID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return OverrideDTO{}, &NotFoundError{Msg: "override not found"}
		}
		return OverrideDTO{}, fmt.Errorf("get override: %w", err)
	}

	if existing.Status != "pending" {
		return OverrideDTO{}, &ValidationError{Msg: "override is not pending"}
	}

	override, err := s.overrideRepo.Approve(ctx, overrideID, actorID, strings.TrimSpace(input.Justification))
	if err != nil {
		return OverrideDTO{}, fmt.Errorf("approve override: %w", err)
	}

	// Audit at CRITICAL severity per GRP-08.
	emails := make([]string, 0, len(override.BlockedTargets))
	patterns := make([]string, 0, len(override.BlockedTargets))
	for _, bt := range override.BlockedTargets {
		emails = append(emails, bt.Email)
		patterns = append(patterns, bt.Pattern)
	}

	resourceType := "blocklist_override"
	_ = s.auditSvc.Log(ctx, auditsvc.LogEntry{
		Category:      auditsvc.CategoryUserActivity,
		Severity:      auditsvc.SeverityCritical,
		ActorType:     auditsvc.ActorTypeUser,
		ActorID:       &actorID,
		ActorLabel:    actorName,
		Action:        "blocklist.override.approve",
		ResourceType:  &resourceType,
		ResourceID:    &override.ID,
		Details: map[string]any{
			"campaign_id":     override.CampaignID,
			"overridden_emails": emails,
			"matching_patterns": patterns,
			"justification":   input.Justification,
		},
		CorrelationID: correlationID,
		SourceIP:      &ip,
	})

	// Notify all admins.
	if s.notifSvc != nil {
		s.notifSvc.Create(ctx, notifsvc.CreateNotificationParams{
			Category:     "blocklist",
			Severity:     "warning",
			Title:        "Block list override approved",
			Body:         fmt.Sprintf("Block list override approved for campaign %s", override.CampaignID),
			ResourceType: "campaign",
			ResourceID:   override.CampaignID,
			ActionURL:    fmt.Sprintf("/campaigns/%s", override.CampaignID),
			Recipients:   notifsvc.RecipientSpec{Role: "Administrator"},
		})
	}

	return overrideToDTO(override), nil
}

// RejectOverride rejects a block list override.
func (s *Service) RejectOverride(ctx context.Context, overrideID string, input OverrideInput, actorID, actorName, ip, correlationID string) (OverrideDTO, error) {
	existing, err := s.overrideRepo.GetByID(ctx, overrideID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return OverrideDTO{}, &NotFoundError{Msg: "override not found"}
		}
		return OverrideDTO{}, fmt.Errorf("get override: %w", err)
	}

	if existing.Status != "pending" {
		return OverrideDTO{}, &ValidationError{Msg: "override is not pending"}
	}

	override, err := s.overrideRepo.Reject(ctx, overrideID, actorID, input.RejectionReason)
	if err != nil {
		return OverrideDTO{}, fmt.Errorf("reject override: %w", err)
	}

	resourceType := "blocklist_override"
	_ = s.auditSvc.Log(ctx, auditsvc.LogEntry{
		Category:      auditsvc.CategoryUserActivity,
		Severity:      auditsvc.SeverityWarning,
		ActorType:     auditsvc.ActorTypeUser,
		ActorID:       &actorID,
		ActorLabel:    actorName,
		Action:        "blocklist.override.reject",
		ResourceType:  &resourceType,
		ResourceID:    &override.ID,
		Details: map[string]any{
			"campaign_id":      override.CampaignID,
			"rejection_reason": input.RejectionReason,
		},
		CorrelationID: correlationID,
		SourceIP:      &ip,
	})

	// Notify all admins.
	if s.notifSvc != nil {
		s.notifSvc.Create(ctx, notifsvc.CreateNotificationParams{
			Category:     "blocklist",
			Severity:     "info",
			Title:        "Block list override rejected",
			Body:         fmt.Sprintf("Block list override rejected for campaign %s", override.CampaignID),
			ResourceType: "campaign",
			ResourceID:   override.CampaignID,
			ActionURL:    fmt.Sprintf("/campaigns/%s", override.CampaignID),
			Recipients:   notifsvc.RecipientSpec{Role: "Administrator"},
		})
	}

	return overrideToDTO(override), nil
}

// --- Pattern Matching Engine (GRP-05) ---

// validatePattern checks that a pattern is a valid block list pattern.
func validatePattern(pattern string) error {
	if pattern == "" {
		return &ValidationError{Msg: "pattern is required"}
	}
	if pattern == "*" || pattern == "*@*" || pattern == "@@" {
		return &ValidationError{Msg: "invalid pattern: too broad or malformed"}
	}
	if !strings.Contains(pattern, "@") {
		return &ValidationError{Msg: "pattern must contain @"}
	}

	parts := strings.SplitN(pattern, "@", 2)
	local, domain := parts[0], parts[1]

	if domain == "" {
		return &ValidationError{Msg: "pattern must have a domain after @"}
	}

	// local part: either exact or "*"
	if local != "*" && local == "" {
		return &ValidationError{Msg: "pattern must have a local part or * before @"}
	}

	// domain part: must be valid-ish
	if domain == "*" {
		return &ValidationError{Msg: "invalid pattern: domain cannot be *"}
	}

	return nil
}

// matchEmail checks a single email against a list of active entries.
func matchEmail(email string, entries []repositories.BlocklistEntry) []repositories.BlocklistEntry {
	email = strings.ToLower(email)
	atIdx := strings.LastIndex(email, "@")
	if atIdx < 0 {
		return nil
	}
	domain := email[atIdx+1:]

	var matches []repositories.BlocklistEntry
	for _, e := range entries {
		if !e.IsActive {
			continue
		}
		p := strings.ToLower(e.Pattern)
		if matchesPattern(email, domain, p) {
			matches = append(matches, e)
		}
	}
	return matches
}

// matchesPattern checks if an email matches a block list pattern.
// Pattern types:
//   - "user@domain.com" — exact match
//   - "*@domain.com" — all emails at exact domain
//   - "*@*.domain.com" — all emails at any subdomain (NOT bare domain)
func matchesPattern(email, emailDomain, pattern string) bool {
	// Ensure case-insensitive comparison throughout.
	email = strings.ToLower(email)
	emailDomain = strings.ToLower(emailDomain)
	pattern = strings.ToLower(pattern)

	parts := strings.SplitN(pattern, "@", 2)
	if len(parts) != 2 {
		return false
	}
	pLocal, pDomain := parts[0], parts[1]

	if strings.HasPrefix(pDomain, "*.") {
		// Subdomain wildcard: *@*.domain.com
		baseDomain := pDomain[2:] // "domain.com"
		// Must match a subdomain, NOT the bare domain.
		if emailDomain == baseDomain {
			return false
		}
		return strings.HasSuffix(emailDomain, "."+baseDomain)
	}

	// Exact domain match required for remaining patterns.
	if emailDomain != pDomain {
		return false
	}

	if pLocal == "*" {
		// *@domain.com — all emails at exact domain.
		return true
	}

	// Exact email match.
	return email == pattern
}

// getActiveEntries returns cached active entries, refreshing if needed.
func (s *Service) getActiveEntries(ctx context.Context) ([]repositories.BlocklistEntry, error) {
	s.mu.RLock()
	if s.cacheLoaded {
		entries := s.cachedEntries
		s.mu.RUnlock()
		return entries, nil
	}
	s.mu.RUnlock()

	return s.refreshCache(ctx)
}

// refreshCache loads active entries from DB into the cache.
func (s *Service) refreshCache(ctx context.Context) ([]repositories.BlocklistEntry, error) {
	entries, err := s.repo.ListActive(ctx)
	if err != nil {
		return nil, fmt.Errorf("load active entries: %w", err)
	}

	s.mu.Lock()
	s.cachedEntries = entries
	s.cacheLoaded = true
	s.mu.Unlock()

	return entries, nil
}

// invalidateCache marks the cache as stale.
func (s *Service) invalidateCache() {
	s.mu.Lock()
	s.cacheLoaded = false
	s.mu.Unlock()
}

// computeTargetHash creates a deterministic hash of blocked targets for invalidation detection.
func computeTargetHash(targets []repositories.BlockedTargetInfo) string {
	sorted := make([]string, 0, len(targets))
	for _, t := range targets {
		sorted = append(sorted, t.TargetID+":"+t.Pattern)
	}
	sort.Strings(sorted)
	h := sha256.Sum256([]byte(strings.Join(sorted, "|")))
	return hex.EncodeToString(h[:])
}

func entryToDTO(e repositories.BlocklistEntry) EntryDTO {
	return EntryDTO{
		ID:       e.ID,
		Pattern:  e.Pattern,
		Reason:   e.Reason,
		IsActive: e.IsActive,
		AddedBy:  e.AddedBy,
		AddedAt:  e.AddedAt.Format(time.RFC3339),
	}
}

func overrideToDTO(o repositories.BlocklistOverride) OverrideDTO {
	dto := OverrideDTO{
		ID:              o.ID,
		CampaignID:      o.CampaignID,
		Status:          o.Status,
		BlockedTargets:  o.BlockedTargets,
		Acknowledgment:  o.Acknowledgment,
		Justification:   o.Justification,
		RejectionReason: o.RejectionReason,
		DecidedBy:       o.DecidedBy,
		CreatedAt:       o.CreatedAt.Format(time.RFC3339),
	}
	if o.DecidedAt != nil {
		s := o.DecidedAt.Format(time.RFC3339)
		dto.DecidedAt = &s
	}
	return dto
}
