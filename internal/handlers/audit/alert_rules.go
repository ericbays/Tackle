package audit

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"

	"tackle/internal/middleware"
	auditsvc "tackle/internal/services/audit"
	"tackle/pkg/response"
)

// alertRuleResponse is the JSON representation of an alert rule.
type alertRuleResponse struct {
	ID              string                  `json:"id"`
	Name            string                  `json:"name"`
	Description     string                  `json:"description"`
	Conditions      auditsvc.AlertConditions `json:"conditions"`
	Actions         auditsvc.AlertActions    `json:"actions"`
	IsEnabled       bool                    `json:"is_enabled"`
	CooldownMinutes int                     `json:"cooldown_minutes"`
	LastTriggeredAt *string                 `json:"last_triggered_at"`
	CreatedBy       string                  `json:"created_by"`
	CreatedAt       string                  `json:"created_at"`
	UpdatedAt       string                  `json:"updated_at"`
}

// ListAlertRules handles GET /api/v1/alert-rules — returns all alert rules.
func (d *Deps) ListAlertRules(w http.ResponseWriter, r *http.Request) {
	correlationID := middleware.GetCorrelationID(r.Context())

	rows, err := d.DB.QueryContext(r.Context(), `
		SELECT id, name, COALESCE(description, ''), conditions, actions,
		       is_enabled, cooldown_minutes, last_triggered_at, created_by,
		       created_at, updated_at
		FROM alert_rules ORDER BY created_at DESC`)
	if err != nil {
		response.Error(w, "INTERNAL_ERROR", "failed to query alert rules", http.StatusInternalServerError, correlationID)
		return
	}
	defer rows.Close()

	rules := make([]alertRuleResponse, 0)
	for rows.Next() {
		rule, err := scanAlertRule(rows)
		if err != nil {
			response.Error(w, "INTERNAL_ERROR", "failed to read alert rule", http.StatusInternalServerError, correlationID)
			return
		}
		rules = append(rules, *rule)
	}
	if err := rows.Err(); err != nil {
		response.Error(w, "INTERNAL_ERROR", "failed to iterate alert rules", http.StatusInternalServerError, correlationID)
		return
	}

	response.Success(w, rules)
}

// createAlertRuleRequest is the expected body for creating an alert rule.
type createAlertRuleRequest struct {
	Name            string                  `json:"name"`
	Description     string                  `json:"description"`
	Conditions      auditsvc.AlertConditions `json:"conditions"`
	Actions         auditsvc.AlertActions    `json:"actions"`
	IsEnabled       *bool                   `json:"is_enabled"`
	CooldownMinutes *int                    `json:"cooldown_minutes"`
}

// CreateAlertRule handles POST /api/v1/alert-rules — creates a new alert rule.
func (d *Deps) CreateAlertRule(w http.ResponseWriter, r *http.Request) {
	correlationID := middleware.GetCorrelationID(r.Context())
	claims := middleware.ClaimsFromContext(r.Context())
	if claims == nil {
		response.Error(w, "UNAUTHORIZED", "authentication required", http.StatusUnauthorized, correlationID)
		return
	}

	var req createAlertRuleRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.Error(w, "INVALID_INPUT", "invalid request body", http.StatusBadRequest, correlationID)
		return
	}
	if req.Name == "" {
		response.Error(w, "INVALID_INPUT", "name is required", http.StatusBadRequest, correlationID)
		return
	}

	isEnabled := true
	if req.IsEnabled != nil {
		isEnabled = *req.IsEnabled
	}
	cooldownMinutes := 60
	if req.CooldownMinutes != nil {
		cooldownMinutes = *req.CooldownMinutes
	}

	conditionsJSON, err := json.Marshal(req.Conditions)
	if err != nil {
		response.Error(w, "INTERNAL_ERROR", "failed to encode conditions", http.StatusInternalServerError, correlationID)
		return
	}
	actionsJSON, err := json.Marshal(req.Actions)
	if err != nil {
		response.Error(w, "INTERNAL_ERROR", "failed to encode actions", http.StatusInternalServerError, correlationID)
		return
	}

	var id, createdAt, updatedAt string
	err = d.DB.QueryRowContext(r.Context(), `
		INSERT INTO alert_rules (name, description, conditions, actions, is_enabled, cooldown_minutes, created_by)
		VALUES ($1, $2, $3::jsonb, $4::jsonb, $5, $6, $7)
		RETURNING id, created_at, updated_at`,
		req.Name, req.Description, string(conditionsJSON), string(actionsJSON),
		isEnabled, cooldownMinutes, claims.Subject,
	).Scan(&id, &createdAt, &updatedAt)
	if err != nil {
		response.Error(w, "INTERNAL_ERROR", "failed to create alert rule", http.StatusInternalServerError, correlationID)
		return
	}

	result := alertRuleResponse{
		ID:              id,
		Name:            req.Name,
		Description:     req.Description,
		Conditions:      req.Conditions,
		Actions:         req.Actions,
		IsEnabled:       isEnabled,
		CooldownMinutes: cooldownMinutes,
		CreatedBy:       claims.Subject,
		CreatedAt:       createdAt,
		UpdatedAt:       updatedAt,
	}
	response.Created(w, result)
}

// UpdateAlertRule handles PUT /api/v1/alert-rules/{id} — updates an existing alert rule.
func (d *Deps) UpdateAlertRule(w http.ResponseWriter, r *http.Request) {
	correlationID := middleware.GetCorrelationID(r.Context())
	id := chi.URLParam(r, "id")

	var req createAlertRuleRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.Error(w, "INVALID_INPUT", "invalid request body", http.StatusBadRequest, correlationID)
		return
	}
	if req.Name == "" {
		response.Error(w, "INVALID_INPUT", "name is required", http.StatusBadRequest, correlationID)
		return
	}

	isEnabled := true
	if req.IsEnabled != nil {
		isEnabled = *req.IsEnabled
	}
	cooldownMinutes := 60
	if req.CooldownMinutes != nil {
		cooldownMinutes = *req.CooldownMinutes
	}

	conditionsJSON, err := json.Marshal(req.Conditions)
	if err != nil {
		response.Error(w, "INTERNAL_ERROR", "failed to encode conditions", http.StatusInternalServerError, correlationID)
		return
	}
	actionsJSON, err := json.Marshal(req.Actions)
	if err != nil {
		response.Error(w, "INTERNAL_ERROR", "failed to encode actions", http.StatusInternalServerError, correlationID)
		return
	}

	result, err := d.DB.ExecContext(r.Context(), `
		UPDATE alert_rules
		SET name = $1, description = $2, conditions = $3::jsonb, actions = $4::jsonb,
		    is_enabled = $5, cooldown_minutes = $6, updated_at = now()
		WHERE id = $7`,
		req.Name, req.Description, string(conditionsJSON), string(actionsJSON),
		isEnabled, cooldownMinutes, id,
	)
	if err != nil {
		response.Error(w, "INTERNAL_ERROR", "failed to update alert rule", http.StatusInternalServerError, correlationID)
		return
	}
	rows, _ := result.RowsAffected()
	if rows == 0 {
		response.Error(w, "NOT_FOUND", "alert rule not found", http.StatusNotFound, correlationID)
		return
	}

	response.Success(w, map[string]string{"id": id, "status": "updated"})
}

// DeleteAlertRule handles DELETE /api/v1/alert-rules/{id} — deletes an alert rule.
func (d *Deps) DeleteAlertRule(w http.ResponseWriter, r *http.Request) {
	correlationID := middleware.GetCorrelationID(r.Context())
	id := chi.URLParam(r, "id")

	result, err := d.DB.ExecContext(r.Context(), `DELETE FROM alert_rules WHERE id = $1`, id)
	if err != nil {
		response.Error(w, "INTERNAL_ERROR", "failed to delete alert rule", http.StatusInternalServerError, correlationID)
		return
	}
	rows, _ := result.RowsAffected()
	if rows == 0 {
		response.Error(w, "NOT_FOUND", "alert rule not found", http.StatusNotFound, correlationID)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// scanAlertRule scans a single alert_rules row.
func scanAlertRule(rows *sql.Rows) (*alertRuleResponse, error) {
	var (
		r             alertRuleResponse
		condRaw       []byte
		actRaw        []byte
		lastTriggered sql.NullTime
		createdAt     time.Time
		updatedAt     time.Time
	)
	if err := rows.Scan(&r.ID, &r.Name, &r.Description, &condRaw, &actRaw,
		&r.IsEnabled, &r.CooldownMinutes, &lastTriggered, &r.CreatedBy,
		&createdAt, &updatedAt); err != nil {
		return nil, err
	}
	if err := json.Unmarshal(condRaw, &r.Conditions); err != nil {
		return nil, err
	}
	if err := json.Unmarshal(actRaw, &r.Actions); err != nil {
		return nil, err
	}
	if lastTriggered.Valid {
		s := lastTriggered.Time.Format(time.RFC3339)
		r.LastTriggeredAt = &s
	}
	r.CreatedAt = createdAt.Format(time.RFC3339)
	r.UpdatedAt = updatedAt.Format(time.RFC3339)
	return &r, nil
}
