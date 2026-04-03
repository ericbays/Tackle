// Package report provides report template management and report generation.
package report

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/google/uuid"
)

// ReportTemplate is the DB model for a report template.
type ReportTemplate struct {
	ID             string         `json:"id"`
	Name           string         `json:"name"`
	Description    string         `json:"description"`
	TemplateType   string         `json:"template_type"`
	TemplateConfig map[string]any `json:"template_config"`
	LayoutConfig   map[string]any `json:"layout_config"`
	IsDefault      bool           `json:"is_default"`
	CreatedBy      string         `json:"created_by"`
	CreatedAt      time.Time      `json:"created_at"`
	UpdatedAt      time.Time      `json:"updated_at"`
}

// GeneratedReport is the DB model for a generated report.
type GeneratedReport struct {
	ID            string         `json:"id"`
	TemplateID    *string        `json:"template_id"`
	CampaignIDs   []string       `json:"campaign_ids"`
	Title         string         `json:"title"`
	Format        string         `json:"format"`
	Status        string         `json:"status"`
	FilePath      *string        `json:"file_path"`
	FileSizeBytes *int64         `json:"file_size_bytes"`
	Parameters    map[string]any `json:"parameters"`
	ErrorMessage  *string        `json:"error_message"`
	GeneratedBy   string         `json:"generated_by"`
	StartedAt     *time.Time     `json:"started_at"`
	CompletedAt   *time.Time     `json:"completed_at"`
	CreatedAt     time.Time      `json:"created_at"`
	UpdatedAt     time.Time      `json:"updated_at"`
}

// TemplateService handles report template CRUD.
type TemplateService struct {
	db *sql.DB
}

// NewTemplateService creates a new template service.
func NewTemplateService(db *sql.DB) *TemplateService {
	return &TemplateService{db: db}
}

// ListTemplates returns all active report templates.
func (s *TemplateService) ListTemplates(ctx context.Context) ([]ReportTemplate, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, name, description, template_type, template_config, layout_config, is_default, created_by, created_at, updated_at
		FROM report_templates WHERE deleted_at IS NULL ORDER BY is_default DESC, name`)
	if err != nil {
		return nil, fmt.Errorf("report templates: list: %w", err)
	}
	defer rows.Close()

	var templates []ReportTemplate
	for rows.Next() {
		var t ReportTemplate
		var configJSON, layoutJSON []byte
		if err := rows.Scan(&t.ID, &t.Name, &t.Description, &t.TemplateType,
			&configJSON, &layoutJSON, &t.IsDefault, &t.CreatedBy, &t.CreatedAt, &t.UpdatedAt); err != nil {
			return nil, fmt.Errorf("report templates: list scan: %w", err)
		}
		t.TemplateConfig = jsonToMap(configJSON)
		t.LayoutConfig = jsonToMap(layoutJSON)
		templates = append(templates, t)
	}
	return templates, rows.Err()
}

// GetTemplate returns a single report template.
func (s *TemplateService) GetTemplate(ctx context.Context, id string) (ReportTemplate, error) {
	var t ReportTemplate
	var configJSON, layoutJSON []byte
	err := s.db.QueryRowContext(ctx, `
		SELECT id, name, description, template_type, template_config, layout_config, is_default, created_by, created_at, updated_at
		FROM report_templates WHERE id = $1 AND deleted_at IS NULL`, id,
	).Scan(&t.ID, &t.Name, &t.Description, &t.TemplateType,
		&configJSON, &layoutJSON, &t.IsDefault, &t.CreatedBy, &t.CreatedAt, &t.UpdatedAt)
	if err != nil {
		return t, fmt.Errorf("report templates: get: %w", err)
	}
	t.TemplateConfig = jsonToMap(configJSON)
	t.LayoutConfig = jsonToMap(layoutJSON)
	return t, nil
}

// CreateTemplateInput holds input for creating a report template.
type CreateTemplateInput struct {
	Name           string         `json:"name"`
	Description    string         `json:"description"`
	TemplateType   string         `json:"template_type"`
	TemplateConfig map[string]any `json:"template_config"`
	LayoutConfig   map[string]any `json:"layout_config"`
}

// CreateTemplate creates a new report template.
func (s *TemplateService) CreateTemplate(ctx context.Context, input CreateTemplateInput, userID string) (ReportTemplate, error) {
	id := uuid.New().String()

	configJSON := mapToJSON(input.TemplateConfig)
	layoutJSON := mapToJSON(input.LayoutConfig)

	var t ReportTemplate
	var resConfig, resLayout []byte
	err := s.db.QueryRowContext(ctx, `
		INSERT INTO report_templates (id, name, description, template_type, template_config, layout_config, created_by)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		RETURNING id, name, description, template_type, template_config, layout_config, is_default, created_by, created_at, updated_at`,
		id, input.Name, input.Description, input.TemplateType, configJSON, layoutJSON, userID,
	).Scan(&t.ID, &t.Name, &t.Description, &t.TemplateType,
		&resConfig, &resLayout, &t.IsDefault, &t.CreatedBy, &t.CreatedAt, &t.UpdatedAt)
	if err != nil {
		return t, fmt.Errorf("report templates: create: %w", err)
	}
	t.TemplateConfig = jsonToMap(resConfig)
	t.LayoutConfig = jsonToMap(resLayout)
	return t, nil
}

// SeedDefaultTemplates creates default templates if none exist.
func (s *TemplateService) SeedDefaultTemplates(ctx context.Context, systemUserID string) error {
	var count int
	if err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM report_templates WHERE is_default = true`).Scan(&count); err != nil {
		return fmt.Errorf("report templates: seed check: %w", err)
	}
	if count > 0 {
		return nil // Already seeded.
	}

	defaults := []struct {
		name        string
		desc        string
		tmplType    string
		config      string
		layout      string
	}{
		{
			name:     "Campaign Summary",
			desc:     "Detailed metrics and timeline for a single campaign",
			tmplType: "campaign",
			config:   `{"sections": ["overview", "funnel", "timeline", "variants", "targets"]}`,
			layout:   `{"orientation": "portrait", "header": true, "footer": true, "charts": true}`,
		},
		{
			name:     "Campaign Comparison",
			desc:     "Side-by-side metrics comparison across multiple campaigns",
			tmplType: "comparison",
			config:   `{"sections": ["comparison_table", "funnel_overlay", "timeline_overlay"]}`,
			layout:   `{"orientation": "landscape", "header": true, "footer": true, "charts": true}`,
		},
		{
			name:     "Executive Summary",
			desc:     "High-level metrics with susceptibility scores and recommendations",
			tmplType: "executive",
			config:   `{"sections": ["kpi_cards", "department_breakdown", "trend_analysis", "recommendations"]}`,
			layout:   `{"orientation": "portrait", "header": true, "footer": true, "charts": true}`,
		},
	}

	for _, d := range defaults {
		id := uuid.New().String()
		_, err := s.db.ExecContext(ctx, `
			INSERT INTO report_templates (id, name, description, template_type, template_config, layout_config, is_default, created_by)
			VALUES ($1, $2, $3, $4, $5::jsonb, $6::jsonb, true, $7)
			ON CONFLICT DO NOTHING`,
			id, d.name, d.desc, d.tmplType, d.config, d.layout, systemUserID)
		if err != nil {
			return fmt.Errorf("report templates: seed %s: %w", d.name, err)
		}
	}

	return nil
}
