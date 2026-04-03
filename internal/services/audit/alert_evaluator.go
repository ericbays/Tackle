package audit

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"
)

// AlertNotifier is called when an alert rule fires and notify is enabled.
type AlertNotifier interface {
	Create(ctx context.Context, params AlertNotification)
}

// AlertNotification contains the data passed to the notifier when an alert fires.
type AlertNotification struct {
	RuleName    string
	RuleID      string
	Severity    string
	Category    string
	Action      string
	ActorLabel  string
	Description string
}

// AlertRule represents a persisted alert rule from the database.
type AlertRule struct {
	ID              string
	Name            string
	Description     string
	Conditions      AlertConditions
	Actions         AlertActions
	IsEnabled       bool
	CooldownMinutes int
	LastTriggeredAt *time.Time
	CreatedBy       string
}

// AlertConditions defines what audit events match this rule.
type AlertConditions struct {
	Severity      string `json:"severity,omitempty"`
	Category      string `json:"category,omitempty"`
	ActionPattern string `json:"action_pattern,omitempty"`
	Threshold     int    `json:"threshold,omitempty"`
	WindowMinutes int    `json:"window_minutes,omitempty"`
}

// AlertActions defines what happens when the rule fires.
type AlertActions struct {
	Notify bool `json:"notify"`
	Email  bool `json:"email,omitempty"`
}

// AlertEvaluator checks incoming audit entries against active alert rules.
type AlertEvaluator struct {
	db       *sql.DB
	notifier AlertNotifier

	// mu protects rules cache.
	mu          sync.RWMutex
	rules       []AlertRule
	lastRefresh time.Time
	cacheTTL    time.Duration
}

// NewAlertEvaluator creates an AlertEvaluator that checks audit entries against rules.
func NewAlertEvaluator(db *sql.DB, notifier AlertNotifier) *AlertEvaluator {
	return &AlertEvaluator{
		db:       db,
		notifier: notifier,
		cacheTTL: 30 * time.Second,
	}
}

// Evaluate checks a new audit entry against all active alert rules.
// Called asynchronously after each audit log write.
func (ae *AlertEvaluator) Evaluate(entry LogEntry) {
	rules, err := ae.loadRules()
	if err != nil {
		slog.Error("alert_evaluator: load rules", "error", err)
		return
	}

	for _, rule := range rules {
		if ae.matches(rule, entry) {
			if ae.cooldownRespected(rule) {
				ae.fire(rule, entry)
			}
		}
	}
}

// loadRules returns cached rules or refreshes from the database.
func (ae *AlertEvaluator) loadRules() ([]AlertRule, error) {
	ae.mu.RLock()
	if time.Since(ae.lastRefresh) < ae.cacheTTL && ae.rules != nil {
		rules := ae.rules
		ae.mu.RUnlock()
		return rules, nil
	}
	ae.mu.RUnlock()

	ae.mu.Lock()
	defer ae.mu.Unlock()

	// Double-check after acquiring write lock.
	if time.Since(ae.lastRefresh) < ae.cacheTTL && ae.rules != nil {
		return ae.rules, nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	rows, err := ae.db.QueryContext(ctx, `
		SELECT id, name, COALESCE(description, ''), conditions, actions,
		       is_enabled, cooldown_minutes, last_triggered_at, created_by
		FROM alert_rules WHERE is_enabled = TRUE`)
	if err != nil {
		return nil, fmt.Errorf("query alert rules: %w", err)
	}
	defer rows.Close()

	var rules []AlertRule
	for rows.Next() {
		var (
			r              AlertRule
			conditionsRaw  []byte
			actionsRaw     []byte
			lastTriggered  sql.NullTime
		)
		if err := rows.Scan(&r.ID, &r.Name, &r.Description, &conditionsRaw, &actionsRaw,
			&r.IsEnabled, &r.CooldownMinutes, &lastTriggered, &r.CreatedBy); err != nil {
			return nil, fmt.Errorf("scan alert rule: %w", err)
		}
		if err := json.Unmarshal(conditionsRaw, &r.Conditions); err != nil {
			slog.Warn("alert_evaluator: bad conditions JSON", "rule_id", r.ID, "error", err)
			continue
		}
		if err := json.Unmarshal(actionsRaw, &r.Actions); err != nil {
			slog.Warn("alert_evaluator: bad actions JSON", "rule_id", r.ID, "error", err)
			continue
		}
		if lastTriggered.Valid {
			t := lastTriggered.Time
			r.LastTriggeredAt = &t
		}
		rules = append(rules, r)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate alert rules: %w", err)
	}

	ae.rules = rules
	ae.lastRefresh = time.Now()
	return rules, nil
}

// matches checks whether an audit entry matches a rule's conditions.
func (ae *AlertEvaluator) matches(rule AlertRule, entry LogEntry) bool {
	c := rule.Conditions

	if c.Severity != "" && string(entry.Severity) != c.Severity {
		return false
	}
	if c.Category != "" && string(entry.Category) != c.Category {
		return false
	}
	if c.ActionPattern != "" {
		if strings.HasSuffix(c.ActionPattern, "*") {
			prefix := strings.TrimSuffix(c.ActionPattern, "*")
			if !strings.HasPrefix(entry.Action, prefix) {
				return false
			}
		} else if entry.Action != c.ActionPattern {
			return false
		}
	}

	// Threshold-based rules: check recent event count in the database.
	if c.Threshold > 0 && c.WindowMinutes > 0 {
		count, err := ae.countRecentEvents(entry, c.WindowMinutes)
		if err != nil {
			slog.Error("alert_evaluator: count recent events", "error", err)
			return false
		}
		if count < c.Threshold {
			return false
		}
	}

	return true
}

// countRecentEvents counts how many matching events occurred in the last windowMinutes.
func (ae *AlertEvaluator) countRecentEvents(entry LogEntry, windowMinutes int) (int, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	since := time.Now().UTC().Add(-time.Duration(windowMinutes) * time.Minute)

	var clauses []string
	var args []any
	idx := 1

	clauses = append(clauses, fmt.Sprintf("timestamp >= $%d", idx))
	args = append(args, since)
	idx++

	clauses = append(clauses, fmt.Sprintf("action = $%d", idx))
	args = append(args, entry.Action)
	idx++

	query := "SELECT COUNT(*) FROM audit_logs WHERE " + strings.Join(clauses, " AND ")

	var count int
	if err := ae.db.QueryRowContext(ctx, query, args...).Scan(&count); err != nil {
		return 0, fmt.Errorf("count recent events: %w", err)
	}
	return count, nil
}

// cooldownRespected returns true if enough time has passed since the last trigger.
func (ae *AlertEvaluator) cooldownRespected(rule AlertRule) bool {
	if rule.CooldownMinutes <= 0 {
		return true
	}
	if rule.LastTriggeredAt == nil {
		return true
	}
	return time.Since(*rule.LastTriggeredAt) >= time.Duration(rule.CooldownMinutes)*time.Minute
}

// fire executes the alert actions and updates last_triggered_at.
func (ae *AlertEvaluator) fire(rule AlertRule, entry LogEntry) {
	slog.Info("alert_evaluator: rule fired",
		"rule_id", rule.ID,
		"rule_name", rule.Name,
		"action", entry.Action,
	)

	// Update last_triggered_at.
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_, err := ae.db.ExecContext(ctx,
		`UPDATE alert_rules SET last_triggered_at = now(), updated_at = now() WHERE id = $1`, rule.ID)
	if err != nil {
		slog.Error("alert_evaluator: update last_triggered_at", "rule_id", rule.ID, "error", err)
	}

	// Invalidate cache so the updated last_triggered_at is reflected.
	ae.mu.Lock()
	ae.lastRefresh = time.Time{}
	ae.mu.Unlock()

	// Send notification if configured.
	if rule.Actions.Notify && ae.notifier != nil {
		ae.notifier.Create(ctx, AlertNotification{
			RuleName:    rule.Name,
			RuleID:      rule.ID,
			Severity:    string(entry.Severity),
			Category:    string(entry.Category),
			Action:      entry.Action,
			ActorLabel:  entry.ActorLabel,
			Description: rule.Description,
		})
	}
}
