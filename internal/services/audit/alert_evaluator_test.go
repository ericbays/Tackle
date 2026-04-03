package audit

import (
	"context"
	"testing"
	"time"
)

type mockAlertNotifier struct {
	called []AlertNotification
}

func (m *mockAlertNotifier) Create(_ context.Context, params AlertNotification) {
	m.called = append(m.called, params)
}

func TestAlertConditionsMatch(t *testing.T) {
	notifier := &mockAlertNotifier{}
	// Use a nil db — we only test the matches() logic which doesn't hit DB
	// for non-threshold rules.
	ae := &AlertEvaluator{notifier: notifier, cacheTTL: time.Hour}

	tests := []struct {
		name    string
		rule    AlertRule
		entry   LogEntry
		want    bool
	}{
		{
			name: "severity match",
			rule: AlertRule{Conditions: AlertConditions{Severity: "critical"}},
			entry: LogEntry{Severity: SeverityCritical, Action: "test"},
			want:  true,
		},
		{
			name: "severity mismatch",
			rule: AlertRule{Conditions: AlertConditions{Severity: "critical"}},
			entry: LogEntry{Severity: SeverityInfo, Action: "test"},
			want:  false,
		},
		{
			name: "category match",
			rule: AlertRule{Conditions: AlertConditions{Category: "system"}},
			entry: LogEntry{Category: CategorySystem, Severity: SeverityInfo, Action: "test"},
			want:  true,
		},
		{
			name: "action pattern exact",
			rule: AlertRule{Conditions: AlertConditions{ActionPattern: "auth.login.failure"}},
			entry: LogEntry{Severity: SeverityWarning, Action: "auth.login.failure"},
			want:  true,
		},
		{
			name: "action pattern prefix",
			rule: AlertRule{Conditions: AlertConditions{ActionPattern: "auth.*"}},
			entry: LogEntry{Severity: SeverityInfo, Action: "auth.login.success"},
			want:  true,
		},
		{
			name: "action pattern prefix no match",
			rule: AlertRule{Conditions: AlertConditions{ActionPattern: "campaign.*"}},
			entry: LogEntry{Severity: SeverityInfo, Action: "auth.login.success"},
			want:  false,
		},
		{
			name: "multiple conditions all match",
			rule: AlertRule{Conditions: AlertConditions{Severity: "error", Category: "infrastructure", ActionPattern: "endpoint.*"}},
			entry: LogEntry{Severity: SeverityError, Category: CategoryInfrastructure, Action: "endpoint.state_change"},
			want:  true,
		},
		{
			name: "multiple conditions partial match",
			rule: AlertRule{Conditions: AlertConditions{Severity: "error", Category: "infrastructure"}},
			entry: LogEntry{Severity: SeverityError, Category: CategorySystem, Action: "test"},
			want:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ae.matches(tt.rule, tt.entry)
			if got != tt.want {
				t.Errorf("matches() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestCooldownRespected(t *testing.T) {
	ae := &AlertEvaluator{}

	t.Run("no cooldown", func(t *testing.T) {
		rule := AlertRule{CooldownMinutes: 0}
		if !ae.cooldownRespected(rule) {
			t.Error("expected cooldown respected with 0 minutes")
		}
	})

	t.Run("never triggered", func(t *testing.T) {
		rule := AlertRule{CooldownMinutes: 60}
		if !ae.cooldownRespected(rule) {
			t.Error("expected cooldown respected with nil last_triggered_at")
		}
	})

	t.Run("within cooldown", func(t *testing.T) {
		recent := time.Now().Add(-5 * time.Minute)
		rule := AlertRule{CooldownMinutes: 60, LastTriggeredAt: &recent}
		if ae.cooldownRespected(rule) {
			t.Error("expected cooldown NOT respected within window")
		}
	})

	t.Run("cooldown expired", func(t *testing.T) {
		old := time.Now().Add(-120 * time.Minute)
		rule := AlertRule{CooldownMinutes: 60, LastTriggeredAt: &old}
		if !ae.cooldownRespected(rule) {
			t.Error("expected cooldown respected after expiry")
		}
	})
}
