package typosquat

import (
	"context"
	"fmt"
	"sort"
	"sync"
	"time"

	"tackle/internal/providers"
	"tackle/internal/providers/credentials"
	"tackle/internal/providers/godaddy"
	"tackle/internal/providers/namecheap"
	"tackle/internal/repositories"
	auditsvc "tackle/internal/services/audit"
	domainsvc "tackle/internal/services/domain"
)

// AvailabilityStatus is the result of an availability check.
type AvailabilityStatus string

const (
	// AvailabilityStatusAvailable means the domain is available to register.
	AvailabilityStatusAvailable AvailabilityStatus = "available"
	// AvailabilityStatusTaken means the domain is already registered.
	AvailabilityStatusTaken AvailabilityStatus = "taken"
	// AvailabilityStatusUnknown means the check failed or timed out.
	AvailabilityStatusUnknown AvailabilityStatus = "unknown"
)

// TyposquatResult is a fully enriched typosquat candidate.
type TyposquatResult struct {
	Domain       string             `json:"domain"`
	Technique    string             `json:"technique"`
	Similarity   float64            `json:"similarity"`
	Availability AvailabilityStatus `json:"availability"`
	Premium      bool               `json:"premium,omitempty"`
	Price        float64            `json:"price,omitempty"`
	Currency     string             `json:"currency,omitempty"`
	Registrar    string             `json:"registrar,omitempty"`
	Error        string             `json:"error,omitempty"`
}

// GenerateAndCheckInput is the input to GenerateAndCheck.
type GenerateAndCheckInput struct {
	TargetDomain          string
	RegistrarConnectionIDs []string
	CheckAvailability     bool
	SortBy                string // "similarity" | "technique" | "availability"
	FilterTechnique       string // filter by technique, empty = all
	FilterAvailability    string // "available" | "taken" | "unknown" | empty = all
}

// GenerateAndCheckResult is the paginated output of GenerateAndCheck.
type GenerateAndCheckResult struct {
	TargetDomain string            `json:"target_domain"`
	Candidates   []TyposquatResult `json:"candidates"`
	Total        int               `json:"total"`
	GeneratedAt  string            `json:"generated_at"`
}

// availabilityCache is a short-lived in-memory cache for availability results.
var (
	cacheMu    sync.Mutex
	cache      = make(map[string]cachedAvail)
)

type cachedAvail struct {
	result    domainsvc.AvailabilityResult
	expiresAt time.Time
}

// Service orchestrates typosquat generation and availability checking.
type Service struct {
	providerRepo *repositories.DomainProviderRepository
	profileRepo  *repositories.DomainProfileRepository
	credEnc      *credentials.EncryptionService
	domainSvc    *domainsvc.Service
	audit        *auditsvc.AuditService
}

// NewService creates a typosquat Service.
func NewService(
	providerRepo *repositories.DomainProviderRepository,
	profileRepo *repositories.DomainProfileRepository,
	credEnc *credentials.EncryptionService,
	domainSvc *domainsvc.Service,
	audit *auditsvc.AuditService,
) *Service {
	return &Service{
		providerRepo: providerRepo,
		profileRepo:  profileRepo,
		credEnc:      credEnc,
		domainSvc:    domainSvc,
		audit:        audit,
	}
}

// GenerateAndCheck generates typosquat candidates, scores them, optionally checks
// availability, and returns a filtered/sorted result set.
func (s *Service) GenerateAndCheck(ctx context.Context, input GenerateAndCheckInput, actorID, actorLabel string) (GenerateAndCheckResult, error) {
	candidates, err := GenerateTyposquats(input.TargetDomain)
	if err != nil {
		return GenerateAndCheckResult{}, fmt.Errorf("typosquat: generate: %w", err)
	}

	// Score similarity.
	for i := range candidates {
		candidates[i].Similarity = CalculateSimilarity(input.TargetDomain, candidates[i].Domain)
	}

	// Availability checking.
	results := make([]TyposquatResult, 0, len(candidates))
	if input.CheckAvailability && len(input.RegistrarConnectionIDs) > 0 {
		domains := make([]string, len(candidates))
		for i, c := range candidates {
			domains[i] = c.Domain
		}
		availMap := s.checkBulkAvailability(ctx, domains, input.RegistrarConnectionIDs)

		for _, c := range candidates {
			avail, ok := availMap[c.Domain]
			r := TyposquatResult{
				Domain:    c.Domain,
				Technique: c.Technique,
				Similarity: c.Similarity,
			}
			if !ok {
				r.Availability = AvailabilityStatusUnknown
			} else if avail.err != nil {
				r.Availability = AvailabilityStatusUnknown
				r.Error = avail.err.Error()
			} else if avail.result.Available {
				r.Availability = AvailabilityStatusAvailable
				r.Premium = avail.result.Premium
				r.Price = avail.result.Price
				r.Currency = avail.result.Currency
				r.Registrar = avail.registrar
			} else {
				r.Availability = AvailabilityStatusTaken
			}
			results = append(results, r)
		}
	} else {
		for _, c := range candidates {
			results = append(results, TyposquatResult{
				Domain:    c.Domain,
				Technique: c.Technique,
				Similarity: c.Similarity,
				Availability: AvailabilityStatusUnknown,
			})
		}
	}

	// Filter.
	if input.FilterTechnique != "" {
		filtered := results[:0]
		for _, r := range results {
			if r.Technique == input.FilterTechnique {
				filtered = append(filtered, r)
			}
		}
		results = filtered
	}
	if input.FilterAvailability != "" {
		filtered := results[:0]
		for _, r := range results {
			if string(r.Availability) == input.FilterAvailability {
				filtered = append(filtered, r)
			}
		}
		results = filtered
	}

	// Sort.
	switch input.SortBy {
	case "technique":
		sort.Slice(results, func(i, j int) bool {
			if results[i].Technique != results[j].Technique {
				return results[i].Technique < results[j].Technique
			}
			return results[i].Similarity > results[j].Similarity
		})
	case "availability":
		order := map[AvailabilityStatus]int{
			AvailabilityStatusAvailable: 0,
			AvailabilityStatusTaken:     1,
			AvailabilityStatusUnknown:   2,
		}
		sort.Slice(results, func(i, j int) bool {
			oi, oj := order[results[i].Availability], order[results[j].Availability]
			if oi != oj {
				return oi < oj
			}
			return results[i].Similarity > results[j].Similarity
		})
	default: // similarity (default)
		sort.Slice(results, func(i, j int) bool {
			return results[i].Similarity > results[j].Similarity
		})
	}

	_ = s.audit.Log(ctx, auditsvc.LogEntry{
		Category:   auditsvc.CategoryInfrastructure,
		Severity:   auditsvc.SeverityInfo,
		ActorType:  auditsvc.ActorTypeUser,
		ActorID:    &actorID,
		ActorLabel: actorLabel,
		Action:     "domain.typosquat_generated",
		ResourceType: strPtr("tool"),
		Details: map[string]any{
			"target_domain":   input.TargetDomain,
			"candidate_count": len(results),
		},
	})

	return GenerateAndCheckResult{
		TargetDomain: input.TargetDomain,
		Candidates:   results,
		Total:        len(results),
		GeneratedAt:  time.Now().UTC().Format("2006-01-02T15:04:05Z07:00"),
	}, nil
}

// RegisterFromTyposquat submits a domain registration request for a typosquat candidate.
// Delegates to the domain registration service's approval workflow.
func (s *Service) RegisterFromTyposquat(ctx context.Context, candidateDomain, registrarConnectionID string, actorID, actorLabel, role, sourceIP, correlationID string) (domainsvc.DomainProfileDTO, bool, error) {
	input := domainsvc.RegisterDomainInput{
		RegistrarConnectionID: registrarConnectionID,
		Domain:                candidateDomain,
		Years:                 1,
	}
	dto, pending, err := s.domainSvc.RegisterDomain(ctx, input, actorID, actorLabel, role, sourceIP, correlationID)
	if err != nil {
		return domainsvc.DomainProfileDTO{}, false, fmt.Errorf("typosquat: register: %w", err)
	}

	_ = s.audit.Log(ctx, auditsvc.LogEntry{
		Category:   auditsvc.CategoryInfrastructure,
		Severity:   auditsvc.SeverityInfo,
		ActorType:  auditsvc.ActorTypeUser,
		ActorID:    &actorID,
		ActorLabel: actorLabel,
		Action:     "domain.typosquat_registered",
		ResourceType: strPtr("domain"),
		Details: map[string]any{
			"candidate_domain":        candidateDomain,
			"registrar_connection_id": registrarConnectionID,
			"pending":                 pending,
		},
	})

	return dto, pending, nil
}

// availResult holds the outcome of a single availability check.
type availResult struct {
	result    domainsvc.AvailabilityResult
	registrar string
	err       error
}

// checkBulkAvailability checks availability for multiple domains using configured registrar APIs.
// Uses a semaphore to limit concurrency and caches results for 1 hour.
func (s *Service) checkBulkAvailability(ctx context.Context, domains []string, registrarConnIDs []string) map[string]availResult {
	const maxConcurrency = 5
	sem := make(chan struct{}, maxConcurrency)

	var mu sync.Mutex
	results := make(map[string]availResult, len(domains))

	var wg sync.WaitGroup
	for _, domain := range domains {
		wg.Add(1)
		go func(d string) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			// Check cache.
			cacheMu.Lock()
			if cached, ok := cache[d]; ok && time.Now().Before(cached.expiresAt) {
				cacheMu.Unlock()
				mu.Lock()
				results[d] = availResult{result: cached.result}
				mu.Unlock()
				return
			}
			cacheMu.Unlock()

			// Try each registrar connection in order.
			for _, connID := range registrarConnIDs {
				r, registrar, err := s.checkAvailabilityViaConn(ctx, d, connID)
				if err == nil {
					// Cache for 1 hour.
					cacheMu.Lock()
					cache[d] = cachedAvail{result: r, expiresAt: time.Now().Add(time.Hour)}
					cacheMu.Unlock()

					mu.Lock()
					results[d] = availResult{result: r, registrar: registrar}
					mu.Unlock()
					return
				}
			}
			mu.Lock()
			results[d] = availResult{err: fmt.Errorf("no registrar returned a result")}
			mu.Unlock()
		}(domain)
	}
	wg.Wait()
	return results
}

// checkAvailabilityViaConn checks availability for a single domain using one registrar connection.
func (s *Service) checkAvailabilityViaConn(ctx context.Context, domain, connID string) (domainsvc.AvailabilityResult, string, error) {
	conn, err := s.providerRepo.GetByID(ctx, connID)
	if err != nil {
		return domainsvc.AvailabilityResult{}, "", fmt.Errorf("get provider: %w", err)
	}

	switch providers.ProviderType(conn.ProviderType) {
	case providers.ProviderTypeNamecheap:
		var creds credentials.NamecheapCredentials
		if err := s.credEnc.Decrypt(conn.CredentialsEncrypted, &creds); err != nil {
			return domainsvc.AvailabilityResult{}, "", fmt.Errorf("decrypt namecheap: %w", err)
		}
		client := namecheap.NewClient(creds, 0)
		r, err := client.CheckAvailability(domain)
		if err != nil {
			return domainsvc.AvailabilityResult{}, "", err
		}
		return domainsvc.AvailabilityResult{
			Domain: domain, Available: r.Available, Premium: r.Premium, Price: r.Price, Currency: r.Currency,
		}, "namecheap", nil

	case providers.ProviderTypeGoDaddy:
		var creds credentials.GoDaddyCredentials
		if err := s.credEnc.Decrypt(conn.CredentialsEncrypted, &creds); err != nil {
			return domainsvc.AvailabilityResult{}, "", fmt.Errorf("decrypt godaddy: %w", err)
		}
		client := godaddy.NewClient(creds, 0)
		r, err := client.CheckAvailability(domain)
		if err != nil {
			return domainsvc.AvailabilityResult{}, "", err
		}
		return domainsvc.AvailabilityResult{
			Domain: domain, Available: r.Available, Premium: r.Premium, Price: r.Price, Currency: r.Currency,
		}, "godaddy", nil

	default:
		return domainsvc.AvailabilityResult{}, "", fmt.Errorf("provider type %q does not support availability checks", conn.ProviderType)
	}
}

func strPtr(s string) *string { return &s }
