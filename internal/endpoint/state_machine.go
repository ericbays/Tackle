// Package endpoint implements the phishing endpoint lifecycle state machine and orchestration.
package endpoint

import (
	"context"
	"fmt"

	"tackle/internal/repositories"
	"tackle/internal/services/audit"
)

// validTransitions defines which state transitions are allowed.
// Each key is the source state; the value is the set of valid target states.
var validTransitions = map[repositories.EndpointState]map[repositories.EndpointState]bool{
	repositories.EndpointStateRequested: {
		repositories.EndpointStateProvisioning: true,
	},
	repositories.EndpointStateProvisioning: {
		repositories.EndpointStateConfiguring: true,
		repositories.EndpointStateError:       true,
	},
	repositories.EndpointStateConfiguring: {
		repositories.EndpointStateActive: true,
		repositories.EndpointStateError:  true,
	},
	repositories.EndpointStateActive: {
		repositories.EndpointStateStopped:    true,
		repositories.EndpointStateError:      true,
		repositories.EndpointStateTerminated: true,
	},
	repositories.EndpointStateStopped: {
		repositories.EndpointStateActive:     true,
		repositories.EndpointStateTerminated: true,
	},
	repositories.EndpointStateError: {
		repositories.EndpointStateConfiguring: true,
		repositories.EndpointStateTerminated:  true,
	},
	repositories.EndpointStateTerminated: {},
}

// InvalidTransitionError is returned when an invalid state transition is attempted.
type InvalidTransitionError struct {
	From repositories.EndpointState
	To   repositories.EndpointState
}

// Error implements the error interface.
func (e *InvalidTransitionError) Error() string {
	return fmt.Sprintf("invalid endpoint state transition from %s to %s", e.From, e.To)
}

// StateMachine manages phishing endpoint state transitions with audit logging.
type StateMachine struct {
	repo     *repositories.PhishingEndpointRepository
	auditSvc *audit.AuditService
}

// NewStateMachine creates a new endpoint StateMachine.
func NewStateMachine(repo *repositories.PhishingEndpointRepository, auditSvc *audit.AuditService) *StateMachine {
	return &StateMachine{repo: repo, auditSvc: auditSvc}
}

// IsValidTransition returns true if the transition from → to is allowed.
func IsValidTransition(from, to repositories.EndpointState) bool {
	targets, ok := validTransitions[from]
	if !ok {
		return false
	}
	return targets[to]
}

// ValidTargets returns the set of valid target states for a given source state.
func ValidTargets(from repositories.EndpointState) []repositories.EndpointState {
	targets, ok := validTransitions[from]
	if !ok {
		return nil
	}
	result := make([]repositories.EndpointState, 0, len(targets))
	for t := range targets {
		result = append(result, t)
	}
	return result
}

// Transition atomically moves an endpoint from its current state to the target state.
// It validates the transition, persists it in the DB (with the state_transitions audit row),
// and logs to the audit service.
func (sm *StateMachine) Transition(ctx context.Context, endpointID string, toState repositories.EndpointState, actor, reason string) (repositories.PhishingEndpoint, error) {
	// Get current state.
	ep, err := sm.repo.GetByID(ctx, endpointID)
	if err != nil {
		return repositories.PhishingEndpoint{}, fmt.Errorf("state machine: get endpoint: %w", err)
	}

	// Validate the transition.
	if !IsValidTransition(ep.State, toState) {
		return repositories.PhishingEndpoint{}, &InvalidTransitionError{From: ep.State, To: toState}
	}

	// Perform the atomic state update + transition record.
	updated, err := sm.repo.UpdateState(ctx, endpointID, ep.State, toState, actor, reason)
	if err != nil {
		return repositories.PhishingEndpoint{}, fmt.Errorf("state machine: transition: %w", err)
	}

	// Log to audit service.
	resourceType := "phishing_endpoint"
	_ = sm.auditSvc.Log(ctx, audit.LogEntry{
		Category:     audit.CategoryInfrastructure,
		Severity:     audit.SeverityInfo,
		ActorType:    audit.ActorTypeUser,
		ActorID:      &actor,
		ActorLabel:   actor,
		Action:       fmt.Sprintf("endpoint.state.%s_to_%s", ep.State, toState),
		ResourceType: &resourceType,
		ResourceID:   &endpointID,
		Details: map[string]any{
			"from_state": string(ep.State),
			"to_state":   string(toState),
			"reason":     reason,
		},
	})

	return updated, nil
}

// TransitionSystem is like Transition but uses "system" as the actor (for automated transitions).
func (sm *StateMachine) TransitionSystem(ctx context.Context, endpointID string, toState repositories.EndpointState, reason string) (repositories.PhishingEndpoint, error) {
	return sm.Transition(ctx, endpointID, toState, "system", reason)
}

// CreateEndpoint creates a new endpoint in the Requested state and logs the creation.
func (sm *StateMachine) CreateEndpoint(ctx context.Context, campaignID *string, provider repositories.CloudProviderType, region, actor string) (repositories.PhishingEndpoint, error) {
	ep, err := sm.repo.Create(ctx, repositories.PhishingEndpoint{
		CampaignID:    campaignID,
		CloudProvider: provider,
		Region:        region,
	})
	if err != nil {
		return repositories.PhishingEndpoint{}, fmt.Errorf("state machine: create endpoint: %w", err)
	}

	resourceType := "phishing_endpoint"
	_ = sm.auditSvc.Log(ctx, audit.LogEntry{
		Category:     audit.CategoryInfrastructure,
		Severity:     audit.SeverityInfo,
		ActorType:    audit.ActorTypeUser,
		ActorID:      &actor,
		ActorLabel:   actor,
		Action:       "endpoint.created",
		ResourceType: &resourceType,
		ResourceID:   &ep.ID,
		Details: map[string]any{
			"cloud_provider": string(provider),
			"region":         region,
			"campaign_id":    campaignID,
		},
	})

	return ep, nil
}
