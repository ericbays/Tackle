package endpoint

import (
	"testing"

	"tackle/internal/repositories"
)

func TestIsValidTransition_AllValid(t *testing.T) {
	tests := []struct {
		from repositories.EndpointState
		to   repositories.EndpointState
	}{
		{repositories.EndpointStateRequested, repositories.EndpointStateProvisioning},
		{repositories.EndpointStateProvisioning, repositories.EndpointStateConfiguring},
		{repositories.EndpointStateProvisioning, repositories.EndpointStateError},
		{repositories.EndpointStateConfiguring, repositories.EndpointStateActive},
		{repositories.EndpointStateConfiguring, repositories.EndpointStateError},
		{repositories.EndpointStateActive, repositories.EndpointStateStopped},
		{repositories.EndpointStateActive, repositories.EndpointStateError},
		{repositories.EndpointStateActive, repositories.EndpointStateTerminated},
		{repositories.EndpointStateStopped, repositories.EndpointStateActive},
		{repositories.EndpointStateStopped, repositories.EndpointStateTerminated},
		{repositories.EndpointStateError, repositories.EndpointStateConfiguring},
		{repositories.EndpointStateError, repositories.EndpointStateTerminated},
	}
	for _, tc := range tests {
		t.Run(string(tc.from)+"_to_"+string(tc.to), func(t *testing.T) {
			if !IsValidTransition(tc.from, tc.to) {
				t.Errorf("expected transition %s -> %s to be valid", tc.from, tc.to)
			}
		})
	}
}

func TestIsValidTransition_AllInvalid(t *testing.T) {
	tests := []struct {
		from repositories.EndpointState
		to   repositories.EndpointState
	}{
		// Terminated is a terminal state — no transitions out.
		{repositories.EndpointStateTerminated, repositories.EndpointStateActive},
		{repositories.EndpointStateTerminated, repositories.EndpointStateRequested},
		{repositories.EndpointStateTerminated, repositories.EndpointStateProvisioning},
		{repositories.EndpointStateTerminated, repositories.EndpointStateConfiguring},
		{repositories.EndpointStateTerminated, repositories.EndpointStateStopped},
		{repositories.EndpointStateTerminated, repositories.EndpointStateError},

		// Can't skip states.
		{repositories.EndpointStateRequested, repositories.EndpointStateActive},
		{repositories.EndpointStateRequested, repositories.EndpointStateConfiguring},
		{repositories.EndpointStateRequested, repositories.EndpointStateTerminated},
		{repositories.EndpointStateProvisioning, repositories.EndpointStateActive},
		{repositories.EndpointStateProvisioning, repositories.EndpointStateTerminated},

		// Can't go backwards.
		{repositories.EndpointStateActive, repositories.EndpointStateProvisioning},
		{repositories.EndpointStateActive, repositories.EndpointStateConfiguring},
		{repositories.EndpointStateConfiguring, repositories.EndpointStateProvisioning},
		{repositories.EndpointStateStopped, repositories.EndpointStateProvisioning},

		// Error can only go to Configuring (retry) or Terminated.
		{repositories.EndpointStateError, repositories.EndpointStateActive},
		{repositories.EndpointStateError, repositories.EndpointStateProvisioning},
		{repositories.EndpointStateError, repositories.EndpointStateStopped},

		// Self-transitions are not valid.
		{repositories.EndpointStateRequested, repositories.EndpointStateRequested},
		{repositories.EndpointStateActive, repositories.EndpointStateActive},
	}
	for _, tc := range tests {
		t.Run(string(tc.from)+"_to_"+string(tc.to), func(t *testing.T) {
			if IsValidTransition(tc.from, tc.to) {
				t.Errorf("expected transition %s -> %s to be invalid", tc.from, tc.to)
			}
		})
	}
}

func TestValidTargets(t *testing.T) {
	targets := ValidTargets(repositories.EndpointStateActive)
	if len(targets) != 3 {
		t.Fatalf("expected 3 valid targets from Active, got %d", len(targets))
	}

	found := make(map[repositories.EndpointState]bool)
	for _, s := range targets {
		found[s] = true
	}
	for _, expected := range []repositories.EndpointState{
		repositories.EndpointStateStopped,
		repositories.EndpointStateError,
		repositories.EndpointStateTerminated,
	} {
		if !found[expected] {
			t.Errorf("expected %s in valid targets from Active", expected)
		}
	}

	// Terminated has no valid targets.
	terminatedTargets := ValidTargets(repositories.EndpointStateTerminated)
	if len(terminatedTargets) != 0 {
		t.Errorf("expected 0 valid targets from Terminated, got %d", len(terminatedTargets))
	}
}

func TestInvalidTransitionError(t *testing.T) {
	err := &InvalidTransitionError{
		From: repositories.EndpointStateTerminated,
		To:   repositories.EndpointStateActive,
	}
	expected := "invalid endpoint state transition from terminated to active"
	if err.Error() != expected {
		t.Errorf("expected error %q, got %q", expected, err.Error())
	}
}
