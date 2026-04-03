package categorization

import (
	"context"
	"fmt"
	"sync"
	"time"

	"tackle/internal/repositories"
	auditsvc "tackle/internal/services/audit"
	notifsvc "tackle/internal/services/notification"
)

// CategorizationSummary is the API representation of the latest categorization.
type CategorizationSummary struct {
	DomainProfileID string            `json:"domain_profile_id"`
	Results         []CategoryResult  `json:"results"`
	CheckedAt       string            `json:"checked_at"`
}

// Service orchestrates domain categorization checks.
type Service struct {
	healthRepo  *repositories.DomainHealthRepository
	profileRepo *repositories.DomainProfileRepository
	audit       *auditsvc.AuditService
	notif       *notifsvc.NotificationService
	checkers    []CategorizationChecker
}

// NewService creates a categorization Service.
func NewService(
	healthRepo *repositories.DomainHealthRepository,
	profileRepo *repositories.DomainProfileRepository,
	audit *auditsvc.AuditService,
	notif *notifsvc.NotificationService,
) *Service {
	return &Service{
		healthRepo:  healthRepo,
		profileRepo: profileRepo,
		audit:       audit,
		notif:       notif,
		checkers:    AllCheckers(),
	}
}

// CheckCategorization runs all categorization checkers concurrently, persists
// results, detects status changes, and emits notifications on negative transitions.
func (s *Service) CheckCategorization(ctx context.Context, domainProfileID, triggeredByActor, actorLabel string) (CategorizationSummary, error) {
	profile, err := s.profileRepo.GetByID(ctx, domainProfileID)
	if err != nil {
		return CategorizationSummary{}, fmt.Errorf("categorization service: get profile: %w", err)
	}

	// Fetch previous results to detect changes.
	prevResults, _ := s.healthRepo.GetLatestCategorization(ctx, domainProfileID)
	prevByService := make(map[string]repositories.DomainCategorization, len(prevResults))
	for _, p := range prevResults {
		prevByService[p.Service] = p
	}

	// Run all checkers concurrently.
	type indexedResult struct {
		idx    int
		result CategoryResult
	}
	results := make([]CategoryResult, len(s.checkers))
	ch := make(chan indexedResult, len(s.checkers))

	for i, checker := range s.checkers {
		go func(idx int, c CategorizationChecker) {
			checkCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
			defer cancel()
			r, _ := c.CheckCategory(checkCtx, profile.DomainName)
			ch <- indexedResult{idx: idx, result: r}
		}(i, checker)
	}

	for range s.checkers {
		ir := <-ch
		results[ir.idx] = ir.result
	}

	now := time.Now().UTC()

	// Persist results and detect changes.
	for _, r := range results {
		raw := r.RawResponse
		var rawPtr *string
		if raw != "" {
			rawPtr = &raw
		}
		_, _ = s.healthRepo.UpsertCategorization(ctx, repositories.DomainCategorization{
			DomainProfileID: domainProfileID,
			Service:         r.Service,
			Category:        r.Category,
			Status:          repositories.CategorizationStatus(r.Status),
			RawResponse:     rawPtr,
			CheckedAt:       now,
		})

		// Detect changes.
		prev, hadPrev := prevByService[r.Service]
		if hadPrev && prev.Status != repositories.CategorizationStatus(r.Status) {
			_ = s.audit.Log(ctx, auditsvc.LogEntry{
				Category:   auditsvc.CategoryInfrastructure,
				Severity:   auditsvc.SeverityInfo,
				ActorType:  auditsvc.ActorTypeSystem,
				ActorID:    &triggeredByActor,
				ActorLabel: actorLabel,
				Action:     "domain.categorization_changed",
				ResourceType: strPtr("domain"),
				Details: map[string]any{
					"domain_profile_id": domainProfileID,
					"domain_name":       profile.DomainName,
					"service":           r.Service,
					"old_status":        string(prev.Status),
					"new_status":        r.Status,
					"old_category":      prev.Category,
					"new_category":      r.Category,
				},
			})

			// Notify on negative transition to flagged.
			if r.Status == "flagged" {
				_ = s.audit.Log(ctx, auditsvc.LogEntry{
					Category:  auditsvc.CategoryInfrastructure,
					Severity:  auditsvc.SeverityCritical,
					ActorType: auditsvc.ActorTypeSystem,
					Action:    "domain.categorization_flagged",
					ResourceType: strPtr("domain"),
					Details: map[string]any{
						"domain_profile_id": domainProfileID,
						"domain_name":       profile.DomainName,
						"service":           r.Service,
						"category":          r.Category,
					},
				})
				s.notif.Create(ctx, notifsvc.CreateNotificationParams{
					Category:     "security",
					Severity:     "critical",
					Title:        fmt.Sprintf("Domain flagged: %s (%s)", profile.DomainName, r.Service),
					Body:         fmt.Sprintf("Domain %s is now categorized as %q by %s", profile.DomainName, r.Category, r.Service),
					ResourceType: "domain",
					ResourceID:   domainProfileID,
					Recipients:   notifsvc.RecipientSpec{Role: "Engineer"},
				})
			}
		}
	}

	_ = s.audit.Log(ctx, auditsvc.LogEntry{
		Category:   auditsvc.CategoryInfrastructure,
		Severity:   auditsvc.SeverityInfo,
		ActorType:  auditsvc.ActorTypeUser,
		ActorID:    &triggeredByActor,
		ActorLabel: actorLabel,
		Action:     "domain.categorization_changed",
		ResourceType: strPtr("domain"),
		Details: map[string]any{
			"domain_profile_id": domainProfileID,
			"domain_name":       profile.DomainName,
			"service_count":     len(results),
		},
	})

	return CategorizationSummary{
		DomainProfileID: domainProfileID,
		Results:         results,
		CheckedAt:       now.Format("2006-01-02T15:04:05Z07:00"),
	}, nil
}

// GetLatestCategorization returns the most recent categorization per service.
func (s *Service) GetLatestCategorization(ctx context.Context, domainProfileID string) (CategorizationSummary, error) {
	records, err := s.healthRepo.GetLatestCategorization(ctx, domainProfileID)
	if err != nil {
		return CategorizationSummary{}, fmt.Errorf("categorization service: get latest: %w", err)
	}

	results := make([]CategoryResult, 0, len(records))
	var latestTime time.Time
	for _, rec := range records {
		rp := rec.RawResponse
		raw := ""
		if rp != nil {
			raw = *rp
		}
		results = append(results, CategoryResult{
			Service:     rec.Service,
			Category:    rec.Category,
			Status:      string(rec.Status),
			RawResponse: raw,
		})
		if rec.CheckedAt.After(latestTime) {
			latestTime = rec.CheckedAt
		}
	}

	checkedAt := ""
	if !latestTime.IsZero() {
		checkedAt = latestTime.UTC().Format("2006-01-02T15:04:05Z07:00")
	}

	return CategorizationSummary{
		DomainProfileID: domainProfileID,
		Results:         results,
		CheckedAt:       checkedAt,
	}, nil
}

// GetCategorizationHistory returns categorization history for a domain.
func (s *Service) GetCategorizationHistory(ctx context.Context, domainProfileID, service string, limit int) ([]repositories.DomainCategorization, error) {
	return s.healthRepo.ListCategorizationHistory(ctx, domainProfileID, service, limit)
}

// GetAllActiveProfileIDs returns IDs of all active domain profiles.
func (s *Service) GetAllActiveProfileIDs(ctx context.Context) ([]string, error) {
	return s.healthRepo.GetAllActiveProfileIDs(ctx)
}

// RunCategorizationForScheduler is the entry point used by the scheduler worker.
func (s *Service) RunCategorizationForScheduler(ctx context.Context, domainProfileID string) error {
	_, err := s.CheckCategorization(ctx, domainProfileID, "system", "categorization-scheduler")
	return err
}

// CheckWithJitter runs a categorization check with random jitter (used by the scheduler).
func (s *Service) CheckWithJitter(ctx context.Context, domainProfileID string, wg *sync.WaitGroup) {
	defer wg.Done()
	_ = s.RunCategorizationForScheduler(ctx, domainProfileID)
}

func strPtr(s string) *string { return &s }
