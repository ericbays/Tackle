// Package campaign implements the campaign lifecycle state machine.
package campaign

import (
	"fmt"
	"strings"
)

// State represents a campaign lifecycle state.
type State string

const (
	StateDraft           State = "draft"
	StatePendingApproval State = "pending_approval"
	StateApproved        State = "approved"
	StateBuilding        State = "building"
	StateReady           State = "ready"
	StateActive          State = "active"
	StatePaused          State = "paused"
	StateCompleted       State = "completed"
	StateArchived        State = "archived"
)

// AllStates returns all valid campaign states.
func AllStates() []State {
	return []State{
		StateDraft, StatePendingApproval, StateApproved, StateBuilding,
		StateReady, StateActive, StatePaused, StateCompleted, StateArchived,
	}
}

// IsValid returns true if s is a recognized campaign state.
func (s State) IsValid() bool {
	for _, v := range AllStates() {
		if s == v {
			return true
		}
	}
	return false
}

// String implements fmt.Stringer.
func (s State) String() string { return string(s) }

// IsTerminal returns true if the state has no outbound transitions.
func (s State) IsTerminal() bool { return s == StateArchived }

// IsMutable returns true if campaign configuration can be modified in this state.
func (s State) IsMutable() bool { return s == StateDraft }

// IsActiveLike returns true for states where infrastructure is provisioned.
func (s State) IsActiveLike() bool {
	return s == StateBuilding || s == StateReady || s == StateActive || s == StatePaused
}

// RequiredRole specifies the minimum role to perform a transition.
type RequiredRole string

const (
	RoleOperator RequiredRole = "operator"
	RoleEngineer RequiredRole = "engineer"
	RoleAdmin    RequiredRole = "admin"
	RoleSystem   RequiredRole = "system"
)

// Transition represents a named state transition with metadata.
type Transition struct {
	Name         string
	From         State
	To           State
	RequiredRole RequiredRole
	Description  string
}

// transitionKey uniquely identifies a from→to pair.
type transitionKey struct {
	From State
	To   State
}

// transitions defines the 15 valid campaign state transitions (T1–T15).
var transitions = []Transition{
	{Name: "T1", From: StateDraft, To: StatePendingApproval, RequiredRole: RoleOperator, Description: "Submit for approval"},
	{Name: "T2", From: StatePendingApproval, To: StateApproved, RequiredRole: RoleEngineer, Description: "Approve campaign"},
	{Name: "T3", From: StatePendingApproval, To: StateDraft, RequiredRole: RoleEngineer, Description: "Reject campaign"},
	{Name: "T4", From: StateApproved, To: StateBuilding, RequiredRole: RoleOperator, Description: "Trigger build"},
	{Name: "T5", From: StateBuilding, To: StateReady, RequiredRole: RoleSystem, Description: "Build completed"},
	{Name: "T6", From: StateBuilding, To: StateDraft, RequiredRole: RoleSystem, Description: "Build failed (rollback)"},
	{Name: "T7", From: StateReady, To: StateActive, RequiredRole: RoleOperator, Description: "Launch campaign"},
	{Name: "T8", From: StateActive, To: StatePaused, RequiredRole: RoleOperator, Description: "Pause campaign"},
	{Name: "T9", From: StatePaused, To: StateActive, RequiredRole: RoleOperator, Description: "Resume campaign"},
	{Name: "T10", From: StateActive, To: StateCompleted, RequiredRole: RoleOperator, Description: "Complete campaign"},
	{Name: "T11", From: StatePaused, To: StateCompleted, RequiredRole: RoleOperator, Description: "Complete paused campaign"},
	{Name: "T12", From: StateCompleted, To: StateArchived, RequiredRole: RoleOperator, Description: "Archive campaign"},
	{Name: "T13", From: StateApproved, To: StateDraft, RequiredRole: RoleOperator, Description: "Unlock for editing"},
	{Name: "T14", From: StateReady, To: StateDraft, RequiredRole: RoleOperator, Description: "Unlock ready campaign"},
	{Name: "T15", From: StateReady, To: StateActive, RequiredRole: RoleSystem, Description: "Scheduled auto-launch"},
}

// transitionMap is built at init for O(1) lookup.
// A from→to pair may have multiple entries (e.g., Ready→Active for operator and system).
var transitionMap map[transitionKey][]Transition

func init() {
	transitionMap = make(map[transitionKey][]Transition, len(transitions))
	for _, t := range transitions {
		key := transitionKey{From: t.From, To: t.To}
		transitionMap[key] = append(transitionMap[key], t)
	}
}

// StateError is returned when a transition is invalid.
type StateError struct {
	CurrentState      State
	RequestedState    State
	ValidTransitions  []State
}

func (e *StateError) Error() string {
	valid := make([]string, len(e.ValidTransitions))
	for i, s := range e.ValidTransitions {
		valid[i] = string(s)
	}
	return fmt.Sprintf(
		"invalid transition from %q to %q; valid transitions: [%s]",
		e.CurrentState, e.RequestedState, strings.Join(valid, ", "),
	)
}

// RoleError is returned when the actor lacks the required role.
type RoleError struct {
	Transition   string
	RequiredRole RequiredRole
	ActorRole    string
}

func (e *RoleError) Error() string {
	return fmt.Sprintf(
		"transition %q requires role %q but actor has role %q",
		e.Transition, e.RequiredRole, e.ActorRole,
	)
}

// ValidateTransition checks whether transitioning from currentState to targetState
// is allowed and whether the actor has the required role. It returns the matching
// Transition on success. When multiple transitions share the same from→to pair
// (e.g., Ready→Active for both operator and system), the first matching role wins.
func ValidateTransition(currentState, targetState State, actorRole string) (Transition, error) {
	key := transitionKey{From: currentState, To: targetState}
	candidates, ok := transitionMap[key]
	if !ok || len(candidates) == 0 {
		return Transition{}, &StateError{
			CurrentState:     currentState,
			RequestedState:   targetState,
			ValidTransitions: ValidTargets(currentState),
		}
	}

	// Try each candidate; return the first one the actor is authorized for.
	for _, c := range candidates {
		if hasRequiredRole(c.RequiredRole, actorRole) {
			return c, nil
		}
	}

	// Actor doesn't satisfy any candidate's role requirement.
	// Report the first candidate's requirement for clarity.
	return Transition{}, &RoleError{
		Transition:   candidates[0].Name,
		RequiredRole: candidates[0].RequiredRole,
		ActorRole:    actorRole,
	}
}

// ValidTargets returns all states reachable from the given state.
func ValidTargets(from State) []State {
	var targets []State
	for _, t := range transitions {
		if t.From == from {
			targets = append(targets, t.To)
		}
	}
	return targets
}

// GetTransition returns the first transition definition for a from→to pair, or false.
func GetTransition(from, to State) (Transition, bool) {
	candidates, ok := transitionMap[transitionKey{From: from, To: to}]
	if !ok || len(candidates) == 0 {
		return Transition{}, false
	}
	return candidates[0], true
}

// AllTransitions returns all defined transitions.
func AllTransitions() []Transition {
	cp := make([]Transition, len(transitions))
	copy(cp, transitions)
	return cp
}

// hasRequiredRole checks if the actor's role is sufficient for the transition.
// Admin can perform any transition. System transitions are only for system actors.
func hasRequiredRole(required RequiredRole, actorRole string) bool {
	if actorRole == "admin" {
		return true
	}
	switch required {
	case RoleSystem:
		return actorRole == "system"
	case RoleEngineer:
		return actorRole == "engineer" || actorRole == "admin"
	case RoleOperator:
		return actorRole == "operator" || actorRole == "engineer" || actorRole == "admin"
	default:
		return false
	}
}
