package health

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"tackle/internal/repositories"
	auditsvc "tackle/internal/services/audit"
	dnssvc "tackle/internal/services/dns"
	notifsvc "tackle/internal/services/notification"
)

// HealthCheckResults holds the per-check results for a full health check run.
type HealthCheckResults struct {
	DomainProfileID string                 `json:"domain_profile_id"`
	DomainName      string                 `json:"domain_name"`
	OverallStatus   repositories.HealthOverallStatus `json:"overall_status"`
	Propagation     *PropagationCheckResult `json:"propagation,omitempty"`
	Blocklist       *BlocklistCheckResult   `json:"blocklist,omitempty"`
	EmailAuth       *EmailAuthCheckResult   `json:"email_auth,omitempty"`
	MX              *MXCheckResult          `json:"mx,omitempty"`
	CheckedAt       time.Time              `json:"checked_at"`
}

// Service orchestrates domain health checks.
type Service struct {
	healthRepo  *repositories.DomainHealthRepository
	profileRepo *repositories.DomainProfileRepository
	dnsRepo     *repositories.DNSRecordRepository
	audit       *auditsvc.AuditService
	notif       *notifsvc.NotificationService
}

// NewService creates a health check Service.
func NewService(
	healthRepo *repositories.DomainHealthRepository,
	profileRepo *repositories.DomainProfileRepository,
	dnsRepo *repositories.DNSRecordRepository,
	audit *auditsvc.AuditService,
	notif *notifsvc.NotificationService,
) *Service {
	return &Service{
		healthRepo:  healthRepo,
		profileRepo: profileRepo,
		dnsRepo:     dnsRepo,
		audit:       audit,
		notif:       notif,
	}
}

// RunFullHealthCheck runs all checks concurrently, persists the result, and
// emits audit events and notifications as needed.
func (s *Service) RunFullHealthCheck(ctx context.Context, domainProfileID, triggeredByActor, actorLabel string, trigger repositories.HealthTrigger) (HealthCheckResults, error) {
	profile, err := s.profileRepo.GetByID(ctx, domainProfileID)
	if err != nil {
		return HealthCheckResults{}, fmt.Errorf("health service: get profile: %w", err)
	}

	// Fetch DKIM selectors for email auth check.
	dkimKeys, _ := s.dnsRepo.ListDKIMKeys(ctx, domainProfileID)
	selectors := make([]string, 0, len(dkimKeys))
	for _, k := range dkimKeys {
		selectors = append(selectors, k.Selector)
	}

	return s.runChecks(ctx, profile.ID, profile.DomainName, selectors, repositories.HealthCheckTypeFull, trigger, triggeredByActor, actorLabel, true, true, true, true)
}

// RunPartialHealthCheck runs only the specified check types.
// checkTypes values: "propagation", "blocklist", "email_auth", "mx".
func (s *Service) RunPartialHealthCheck(ctx context.Context, domainProfileID string, checkTypes []string, trigger repositories.HealthTrigger, triggeredByActor, actorLabel string) (HealthCheckResults, error) {
	profile, err := s.profileRepo.GetByID(ctx, domainProfileID)
	if err != nil {
		return HealthCheckResults{}, fmt.Errorf("health service: get profile: %w", err)
	}

	dkimKeys, _ := s.dnsRepo.ListDKIMKeys(ctx, domainProfileID)
	selectors := make([]string, 0, len(dkimKeys))
	for _, k := range dkimKeys {
		selectors = append(selectors, k.Selector)
	}

	var runProp, runBL, runEmail, runMX bool
	checkTypeEnum := repositories.HealthCheckTypeFull
	for _, ct := range checkTypes {
		switch ct {
		case "propagation":
			runProp = true
			checkTypeEnum = repositories.HealthCheckTypePropagationOnly
		case "blocklist":
			runBL = true
			checkTypeEnum = repositories.HealthCheckTypeBlocklistOnly
		case "email_auth":
			runEmail = true
			checkTypeEnum = repositories.HealthCheckTypeEmailAuthOnly
		case "mx":
			runMX = true
			checkTypeEnum = repositories.HealthCheckTypeMXOnly
		}
	}
	if runProp && runBL && runEmail && runMX {
		checkTypeEnum = repositories.HealthCheckTypeFull
	}

	return s.runChecks(ctx, profile.ID, profile.DomainName, selectors, checkTypeEnum, trigger, triggeredByActor, actorLabel, runProp, runBL, runEmail, runMX)
}

// GetHealthCheckHistory returns paginated health check history for a domain.
func (s *Service) GetHealthCheckHistory(ctx context.Context, domainProfileID string, limit, offset int) ([]HealthCheckResults, int, error) {
	checks, total, err := s.healthRepo.ListHealthChecks(ctx, domainProfileID, limit, offset)
	if err != nil {
		return nil, 0, fmt.Errorf("health service: list history: %w", err)
	}

	profile, _ := s.profileRepo.GetByID(ctx, domainProfileID)

	results := make([]HealthCheckResults, 0, len(checks))
	for _, c := range checks {
		r, _ := unmarshalCheckResults(c, profile.DomainName)
		results = append(results, r)
	}
	return results, total, nil
}

// GetLatestHealthCheck returns the most recent health check for a domain.
func (s *Service) GetLatestHealthCheck(ctx context.Context, domainProfileID string) (HealthCheckResults, error) {
	check, err := s.healthRepo.GetLatestHealthCheck(ctx, domainProfileID)
	if err != nil {
		return HealthCheckResults{}, fmt.Errorf("health service: get latest: %w", err)
	}

	profile, _ := s.profileRepo.GetByID(ctx, domainProfileID)
	return unmarshalCheckResults(check, profile.DomainName)
}

// runChecks is the core implementation that runs selected checks concurrently.
func (s *Service) runChecks(
	ctx context.Context,
	domainProfileID, domainName string,
	dkimSelectors []string,
	checkType repositories.HealthCheckType,
	trigger repositories.HealthTrigger,
	triggeredByActor, actorLabel string,
	runProp, runBL, runEmail, runMX bool,
) (HealthCheckResults, error) {
	checkCtx, cancel := context.WithTimeout(ctx, 60*time.Second)
	defer cancel()

	var (
		mu         sync.Mutex
		wg         sync.WaitGroup
		propResult *PropagationCheckResult
		blResult   *BlocklistCheckResult
		emailResult *EmailAuthCheckResult
		mxResult   *MXCheckResult
	)

	if runProp {
		wg.Add(1)
		go func() {
			defer wg.Done()
			r, _ := CheckDNSPropagation(checkCtx, domainName)
			mu.Lock()
			propResult = &r
			mu.Unlock()
		}()
	}

	if runBL {
		wg.Add(1)
		go func() {
			defer wg.Done()
			r, _ := CheckBlocklists(checkCtx, domainName)
			mu.Lock()
			blResult = &r
			mu.Unlock()
		}()
	}

	if runEmail {
		wg.Add(1)
		go func() {
			defer wg.Done()
			r, _ := CheckEmailAuth(checkCtx, domainName, dkimSelectors)
			mu.Lock()
			emailResult = &r
			mu.Unlock()
		}()
	}

	if runMX {
		wg.Add(1)
		go func() {
			defer wg.Done()
			r, _ := CheckMXResolution(checkCtx, domainName)
			mu.Lock()
			mxResult = &r
			mu.Unlock()
		}()
	}

	wg.Wait()

	// Aggregate overall status.
	statuses := make([]CheckStatus, 0, 4)
	if propResult != nil {
		statuses = append(statuses, propResult.Status)
	}
	if blResult != nil {
		statuses = append(statuses, blResult.Status)
	}
	if emailResult != nil {
		statuses = append(statuses, emailResult.Status)
	}
	if mxResult != nil {
		statuses = append(statuses, mxResult.Status)
	}

	worstCheck := AggregateStatus(statuses...)
	var overallStatus repositories.HealthOverallStatus
	switch worstCheck {
	case CheckStatusCritical:
		overallStatus = repositories.HealthStatusCritical
	case CheckStatusWarning:
		overallStatus = repositories.HealthStatusWarning
	default:
		overallStatus = repositories.HealthStatusHealthy
	}

	out := HealthCheckResults{
		DomainProfileID: domainProfileID,
		DomainName:      domainName,
		OverallStatus:   overallStatus,
		Propagation:     propResult,
		Blocklist:       blResult,
		EmailAuth:       emailResult,
		MX:              mxResult,
		CheckedAt:       time.Now().UTC(),
	}

	// Persist result.
	resultsJSON, _ := json.Marshal(out)
	stored, err := s.healthRepo.CreateHealthCheck(ctx, repositories.DomainHealthCheck{
		DomainProfileID: domainProfileID,
		CheckType:       checkType,
		OverallStatus:   overallStatus,
		ResultsJSON:     resultsJSON,
		TriggeredBy:     trigger,
	})
	if err != nil {
		return out, fmt.Errorf("health service: persist check: %w", err)
	}
	out.CheckedAt = stored.CreatedAt

	// Audit.
	_ = s.audit.Log(ctx, auditsvc.LogEntry{
		Category:  auditsvc.CategoryInfrastructure,
		Severity:  auditsvc.SeverityInfo,
		ActorType: auditsvc.ActorTypeUser,
		ActorID:   &triggeredByActor,
		ActorLabel: actorLabel,
		Action:    "domain.health_check_completed",
		ResourceType: strPtr("domain"),
		Details: map[string]any{
			"domain_profile_id": domainProfileID,
			"domain_name":       domainName,
			"overall_status":    string(overallStatus),
			"check_type":        string(checkType),
		},
	})

	// If any blocklist hit, emit critical notification.
	if blResult != nil && blResult.Status == CheckStatusCritical {
		listedOn := make([]string, 0)
		for _, e := range blResult.Results {
			if e.Listed {
				listedOn = append(listedOn, e.Name)
			}
		}
		_ = s.audit.Log(ctx, auditsvc.LogEntry{
			Category:  auditsvc.CategoryInfrastructure,
			Severity:  auditsvc.SeverityCritical,
			ActorType: auditsvc.ActorTypeSystem,
			Action:    "domain.blocklist_detected",
			ResourceType: strPtr("domain"),
			Details: map[string]any{
				"domain_profile_id": domainProfileID,
				"domain_name":       domainName,
				"listed_on":         listedOn,
			},
		})

		s.notif.Create(ctx, notifsvc.CreateNotificationParams{
			Category:     "security",
			Severity:     "critical",
			Title:        fmt.Sprintf("Domain blocklisted: %s", domainName),
			Body:         fmt.Sprintf("Domain %s has been detected on %d blocklist(s): %v", domainName, len(listedOn), listedOn),
			ResourceType: "domain",
			ResourceID:   domainProfileID,
			Recipients:   notifsvc.RecipientSpec{Role: "Engineer"},
		})
	}

	return out, nil
}

// unmarshalCheckResults reconstructs a HealthCheckResults from a stored DB record.
func unmarshalCheckResults(c repositories.DomainHealthCheck, domainName string) (HealthCheckResults, error) {
	var r HealthCheckResults
	if err := json.Unmarshal(c.ResultsJSON, &r); err != nil {
		// Return a minimal struct on parse failure.
		return HealthCheckResults{
			DomainProfileID: c.DomainProfileID,
			DomainName:      domainName,
			OverallStatus:   c.OverallStatus,
			CheckedAt:       c.CreatedAt,
		}, nil
	}
	r.DomainProfileID = c.DomainProfileID
	r.CheckedAt = c.CreatedAt
	return r, nil
}

func strPtr(s string) *string { return &s }

// GetAllActiveProfileIDs returns IDs of all active domain profiles.
func (s *Service) GetAllActiveProfileIDs(ctx context.Context) ([]string, error) {
	return s.healthRepo.GetAllActiveProfileIDs(ctx)
}

// RunHealthCheckForScheduler is the entry point used by the scheduler worker.
func (s *Service) RunHealthCheckForScheduler(ctx context.Context, domainProfileID string) error {
	_, err := s.RunFullHealthCheck(ctx, domainProfileID, "system", "health-scheduler", repositories.HealthTriggerScheduled)
	return err
}

// HealthCheckDTO is the API-safe representation of a health check result.
type HealthCheckDTO struct {
	ID              string                 `json:"id,omitempty"`
	DomainProfileID string                 `json:"domain_profile_id"`
	DomainName      string                 `json:"domain_name"`
	CheckType       string                 `json:"check_type,omitempty"`
	OverallStatus   string                 `json:"overall_status"`
	Propagation     *PropagationCheckResult `json:"propagation,omitempty"`
	Blocklist       *BlocklistCheckResult   `json:"blocklist,omitempty"`
	EmailAuth       *EmailAuthCheckResult   `json:"email_auth,omitempty"`
	MX              *MXCheckResult          `json:"mx,omitempty"`
	TriggeredBy     string                 `json:"triggered_by,omitempty"`
	CheckedAt       string                 `json:"checked_at"`
}

// ToDTO converts a HealthCheckResults to its API DTO form.
func (r HealthCheckResults) ToDTO() HealthCheckDTO {
	return HealthCheckDTO{
		DomainProfileID: r.DomainProfileID,
		DomainName:      r.DomainName,
		OverallStatus:   string(r.OverallStatus),
		Propagation:     r.Propagation,
		Blocklist:       r.Blocklist,
		EmailAuth:       r.EmailAuth,
		MX:              r.MX,
		CheckedAt:       r.CheckedAt.UTC().Format("2006-01-02T15:04:05Z07:00"),
	}
}

// DBCheckToDTO converts a raw DB record to a DTO.
func DBCheckToDTO(c repositories.DomainHealthCheck, domainName string) HealthCheckDTO {
	dto := HealthCheckDTO{
		ID:              c.ID,
		DomainProfileID: c.DomainProfileID,
		DomainName:      domainName,
		CheckType:       string(c.CheckType),
		OverallStatus:   string(c.OverallStatus),
		TriggeredBy:     string(c.TriggeredBy),
		CheckedAt:       c.CreatedAt.UTC().Format("2006-01-02T15:04:05Z07:00"),
	}
	var details struct {
		Propagation *PropagationCheckResult `json:"propagation,omitempty"`
		Blocklist   *BlocklistCheckResult   `json:"blocklist,omitempty"`
		EmailAuth   *EmailAuthCheckResult   `json:"email_auth,omitempty"`
		MX          *MXCheckResult          `json:"mx,omitempty"`
	}
	if err := json.Unmarshal(c.ResultsJSON, &details); err == nil {
		dto.Propagation = details.Propagation
		dto.Blocklist = details.Blocklist
		dto.EmailAuth = details.EmailAuth
		dto.MX = details.MX
	}
	return dto
}

// ResolverResult re-exports the dns package type for use in JSON serialization.
// This avoids import cycles — the check functions return dnssvc.ResolverResult
// which is embedded directly here.
var _ = dnssvc.ResolverResult{}
