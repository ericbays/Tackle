package campaign

import (
	"testing"
)

func TestAllStatesAreValid(t *testing.T) {
	for _, s := range AllStates() {
		if !s.IsValid() {
			t.Errorf("state %q should be valid", s)
		}
	}
}

func TestInvalidState(t *testing.T) {
	if State("nonexistent").IsValid() {
		t.Error("nonexistent state should be invalid")
	}
}

func TestArchivedIsTerminal(t *testing.T) {
	if !StateArchived.IsTerminal() {
		t.Error("archived should be terminal")
	}
	for _, s := range AllStates() {
		if s != StateArchived && s.IsTerminal() {
			t.Errorf("state %q should not be terminal", s)
		}
	}
}

func TestOnlyDraftIsMutable(t *testing.T) {
	for _, s := range AllStates() {
		if s == StateDraft && !s.IsMutable() {
			t.Error("draft should be mutable")
		}
		if s != StateDraft && s.IsMutable() {
			t.Errorf("state %q should not be mutable", s)
		}
	}
}

func TestAllTransitionsCount(t *testing.T) {
	all := AllTransitions()
	if len(all) != 15 {
		t.Errorf("expected 15 transitions, got %d", len(all))
	}
}

func TestValidTransitions(t *testing.T) {
	tests := []struct {
		name      string
		from      State
		to        State
		role      string
		wantName  string
		wantErr   bool
	}{
		{"T1: submit", StateDraft, StatePendingApproval, "operator", "T1", false},
		{"T2: approve", StatePendingApproval, StateApproved, "engineer", "T2", false},
		{"T3: reject", StatePendingApproval, StateDraft, "engineer", "T3", false},
		{"T4: build", StateApproved, StateBuilding, "operator", "T4", false},
		{"T5: build complete", StateBuilding, StateReady, "system", "T5", false},
		{"T6: build fail", StateBuilding, StateDraft, "system", "T6", false},
		{"T7: launch", StateReady, StateActive, "operator", "T7", false},
		{"T8: pause", StateActive, StatePaused, "operator", "T8", false},
		{"T9: resume", StatePaused, StateActive, "operator", "T9", false},
		{"T10: complete active", StateActive, StateCompleted, "operator", "T10", false},
		{"T11: complete paused", StatePaused, StateCompleted, "operator", "T11", false},
		{"T12: archive", StateCompleted, StateArchived, "operator", "T12", false},
		{"T13: unlock approved", StateApproved, StateDraft, "operator", "T13", false},
		{"T14: unlock ready", StateReady, StateDraft, "operator", "T14", false},
		{"T15: auto-launch", StateReady, StateActive, "system", "T15", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tr, err := ValidateTransition(tt.from, tt.to, tt.role)
			if tt.wantErr {
				if err == nil {
					t.Error("expected error")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if tr.Name != tt.wantName {
				t.Errorf("got transition %q, want %q", tr.Name, tt.wantName)
			}
		})
	}
}

func TestInvalidTransitions(t *testing.T) {
	tests := []struct {
		name string
		from State
		to   State
		role string
	}{
		{"draft to active", StateDraft, StateActive, "operator"},
		{"active to draft", StateActive, StateDraft, "operator"},
		{"archived to anything", StateArchived, StateDraft, "admin"},
		{"completed to active", StateCompleted, StateActive, "operator"},
		{"ready to completed", StateReady, StateCompleted, "operator"},
		{"building to active", StateBuilding, StateActive, "system"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := ValidateTransition(tt.from, tt.to, tt.role)
			if err == nil {
				t.Error("expected StateError for invalid transition")
			}
			se, ok := err.(*StateError)
			if !ok {
				t.Fatalf("expected *StateError, got %T", err)
			}
			if se.CurrentState != tt.from {
				t.Errorf("StateError.CurrentState = %q, want %q", se.CurrentState, tt.from)
			}
			if se.RequestedState != tt.to {
				t.Errorf("StateError.RequestedState = %q, want %q", se.RequestedState, tt.to)
			}
		})
	}
}

func TestRoleAuthorization(t *testing.T) {
	tests := []struct {
		name    string
		from    State
		to      State
		role    string
		wantErr bool
	}{
		{"operator cannot approve", StatePendingApproval, StateApproved, "operator", true},
		{"viewer cannot submit", StateDraft, StatePendingApproval, "viewer", true},
		{"admin can approve", StatePendingApproval, StateApproved, "admin", false},
		{"admin can do anything", StateDraft, StatePendingApproval, "admin", false},
		{"engineer can submit", StateDraft, StatePendingApproval, "engineer", false},
		{"operator cannot do system transition", StateBuilding, StateReady, "operator", true},
		{"system cannot submit", StateDraft, StatePendingApproval, "system", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := ValidateTransition(tt.from, tt.to, tt.role)
			if tt.wantErr && err == nil {
				t.Error("expected error for unauthorized role")
			}
			if !tt.wantErr && err != nil {
				t.Errorf("unexpected error: %v", err)
			}
			if tt.wantErr && err != nil {
				// Check that it's a RoleError for valid transitions with wrong role
				if _, ok := err.(*RoleError); !ok {
					// Could also be StateError if the transition doesn't exist
					if _, ok2 := err.(*StateError); !ok2 {
						t.Errorf("expected *RoleError or *StateError, got %T", err)
					}
				}
			}
		})
	}
}

func TestValidTargets(t *testing.T) {
	targets := ValidTargets(StateDraft)
	if len(targets) != 1 {
		t.Fatalf("draft should have 1 target, got %d", len(targets))
	}
	if targets[0] != StatePendingApproval {
		t.Errorf("draft target should be pending_approval, got %q", targets[0])
	}

	// Archived has no targets
	targets = ValidTargets(StateArchived)
	if len(targets) != 0 {
		t.Errorf("archived should have 0 targets, got %d", len(targets))
	}

	// Ready has 3 targets: active (operator launch), draft (unlock), active (system auto-launch)
	// But unique targets are: active, draft
	targets = ValidTargets(StateReady)
	if len(targets) != 3 {
		t.Errorf("ready should have 3 transition entries, got %d", len(targets))
	}
}

func TestStateErrorMessage(t *testing.T) {
	err := &StateError{
		CurrentState:     StateDraft,
		RequestedState:   StateActive,
		ValidTransitions: []State{StatePendingApproval},
	}
	msg := err.Error()
	if msg == "" {
		t.Error("error message should not be empty")
	}
}

func TestRoleErrorMessage(t *testing.T) {
	err := &RoleError{
		Transition:   "T2",
		RequiredRole: RoleEngineer,
		ActorRole:    "operator",
	}
	msg := err.Error()
	if msg == "" {
		t.Error("error message should not be empty")
	}
}
