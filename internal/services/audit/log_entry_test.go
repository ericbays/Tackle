package audit

import "testing"

func TestValidate_MissingCategory(t *testing.T) {
	e := LogEntry{Severity: SeverityInfo, ActorType: ActorTypeUser, Action: "test.action"}
	if err := e.Validate(); err == nil {
		t.Fatal("expected error for missing category")
	}
}

func TestValidate_InvalidCategory(t *testing.T) {
	e := LogEntry{Category: "bogus", Severity: SeverityInfo, ActorType: ActorTypeUser, Action: "test.action"}
	if err := e.Validate(); err == nil {
		t.Fatal("expected error for invalid category")
	}
}

func TestValidate_MissingSeverity(t *testing.T) {
	e := LogEntry{Category: CategorySystem, ActorType: ActorTypeUser, Action: "test.action"}
	if err := e.Validate(); err == nil {
		t.Fatal("expected error for missing severity")
	}
}

func TestValidate_InvalidSeverity(t *testing.T) {
	e := LogEntry{Category: CategorySystem, Severity: "bogus", ActorType: ActorTypeUser, Action: "test.action"}
	if err := e.Validate(); err == nil {
		t.Fatal("expected error for invalid severity")
	}
}

func TestValidate_MissingActorType(t *testing.T) {
	e := LogEntry{Category: CategorySystem, Severity: SeverityInfo, Action: "test.action"}
	if err := e.Validate(); err == nil {
		t.Fatal("expected error for missing actor_type")
	}
}

func TestValidate_InvalidActorType(t *testing.T) {
	e := LogEntry{Category: CategorySystem, Severity: SeverityInfo, ActorType: "bogus", Action: "test.action"}
	if err := e.Validate(); err == nil {
		t.Fatal("expected error for invalid actor_type")
	}
}

func TestValidate_MissingAction(t *testing.T) {
	e := LogEntry{Category: CategorySystem, Severity: SeverityInfo, ActorType: ActorTypeUser}
	if err := e.Validate(); err == nil {
		t.Fatal("expected error for missing action")
	}
}

func TestValidate_Valid(t *testing.T) {
	e := LogEntry{
		Category:  CategoryUserActivity,
		Severity:  SeverityInfo,
		ActorType: ActorTypeUser,
		Action:    "auth.login.success",
	}
	if err := e.Validate(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestValidate_AllCategories(t *testing.T) {
	categories := []Category{
		CategoryUserActivity, CategoryEmailEvent, CategoryInfrastructure,
		CategoryRequest, CategorySystem,
	}
	for _, c := range categories {
		e := LogEntry{Category: c, Severity: SeverityInfo, ActorType: ActorTypeSystem, Action: "x"}
		if err := e.Validate(); err != nil {
			t.Errorf("category %q: unexpected error: %v", c, err)
		}
	}
}

func TestValidate_AllSeverities(t *testing.T) {
	severities := []Severity{SeverityDebug, SeverityInfo, SeverityWarning, SeverityError, SeverityCritical}
	for _, s := range severities {
		e := LogEntry{Category: CategorySystem, Severity: s, ActorType: ActorTypeSystem, Action: "x"}
		if err := e.Validate(); err != nil {
			t.Errorf("severity %q: unexpected error: %v", s, err)
		}
	}
}
