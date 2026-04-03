// Package landingpage provides business logic for landing page project management.
package landingpage

import (
	"archive/zip"
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"tackle/internal/repositories"
	auditsvc "tackle/internal/services/audit"
)

// ValidationError indicates invalid input.
type ValidationError struct{ Msg string }

func (e *ValidationError) Error() string { return e.Msg }

// ConflictError indicates a resource conflict (e.g., duplicate name).
type ConflictError struct{ Msg string }

func (e *ConflictError) Error() string { return e.Msg }

// NotFoundError indicates a resource was not found.
type NotFoundError struct{ Msg string }

func (e *NotFoundError) Error() string { return e.Msg }

// Service handles landing page business logic.
type Service struct {
	repo     *repositories.LandingPageRepository
	auditSvc *auditsvc.AuditService
}

// NewService creates a new landing page Service.
func NewService(repo *repositories.LandingPageRepository, auditSvc *auditsvc.AuditService) *Service {
	return &Service{repo: repo, auditSvc: auditSvc}
}

// ---------- DTOs ----------

// ProjectDTO is the API representation of a landing page project.
type ProjectDTO struct {
	ID             string         `json:"id"`
	Name           string         `json:"name"`
	Description    string         `json:"description"`
	DefinitionJSON map[string]any `json:"definition_json"`
	CreatedBy      string         `json:"created_by"`
	CreatedAt      time.Time      `json:"created_at"`
	UpdatedAt      time.Time      `json:"updated_at"`
}

// TemplateDTO is the API representation of a landing page template.
type TemplateDTO struct {
	ID             string         `json:"id"`
	Name           string         `json:"name"`
	Description    string         `json:"description"`
	Category       string         `json:"category"`
	DefinitionJSON map[string]any `json:"definition_json"`
	CreatedBy      string         `json:"created_by"`
	IsShared       bool           `json:"is_shared"`
	CreatedAt      time.Time      `json:"created_at"`
}

// BuildDTO is the API representation of a landing page build.
type BuildDTO struct {
	ID                string         `json:"id"`
	ProjectID         string         `json:"project_id"`
	CampaignID        *string        `json:"campaign_id"`
	Seed              int64          `json:"seed"`
	Strategy          string         `json:"strategy"`
	BuildManifestJSON map[string]any `json:"build_manifest_json"`
	BuildLog          string         `json:"build_log"`
	BinaryPath        *string        `json:"binary_path"`
	BinaryHash        *string        `json:"binary_hash"`
	Status            string         `json:"status"`
	Port              *int           `json:"port"`
	CreatedAt         time.Time      `json:"created_at"`
}

// ComponentTypeDTO describes an available component type for the UI.
type ComponentTypeDTO struct {
	Type       string `json:"type"`
	Category   string `json:"category"`
	Label      string `json:"label"`
	CanNest    bool   `json:"can_nest"`
	HasCapture bool   `json:"has_capture"`
}

// ---------- Input Types ----------

// CreateProjectInput holds input for creating a project.
type CreateProjectInput struct {
	Name           string         `json:"name"`
	Description    string         `json:"description"`
	DefinitionJSON map[string]any `json:"definition_json"`
	TemplateID     string         `json:"template_id"`
}

// UpdateProjectInput holds input for updating a project.
type UpdateProjectInput struct {
	Name           *string        `json:"name"`
	Description    *string        `json:"description"`
	DefinitionJSON map[string]any `json:"definition_json"`
}

// ListProjectInput holds input for listing projects.
type ListProjectInput struct {
	Name    string `json:"name"`
	Page    int    `json:"page"`
	PerPage int    `json:"per_page"`
}

// SaveTemplateInput holds input for saving a project as template.
type SaveTemplateInput struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Category    string `json:"category"`
	IsShared    bool   `json:"is_shared"`
}

// PreviewInput holds input for generating preview HTML.
type PreviewInput struct {
	PageIndex int `json:"page_index"`
}

// ImportHTMLInput holds input for HTML import.
type ImportHTMLInput struct {
	HTML     string `json:"html"`
	Mode     string `json:"mode"` // "builder" or "raw"
	Filename string `json:"filename"`
}

// CloneURLInput holds input for cloning a page from URL.
type CloneURLInput struct {
	URL           string `json:"url"`
	IncludeJS     bool   `json:"include_js"`
	StripTracking bool   `json:"strip_tracking"`
}

// ---------- Project CRUD ----------

// Create creates a new landing page project.
func (s *Service) Create(ctx context.Context, input CreateProjectInput, actorID, actorName string) (ProjectDTO, error) {
	if strings.TrimSpace(input.Name) == "" {
		return ProjectDTO{}, &ValidationError{Msg: "name is required"}
	}
	if len(input.Name) > 255 {
		return ProjectDTO{}, &ValidationError{Msg: "name must be 255 characters or fewer"}
	}

	definition := input.DefinitionJSON
	if definition == nil {
		definition = defaultDefinition()
	}

	// If a template ID is provided, load its definition.
	if input.TemplateID != "" {
		tmpl, err := s.repo.GetTemplateByID(ctx, input.TemplateID)
		if err != nil {
			if strings.Contains(err.Error(), "not found") {
				return ProjectDTO{}, &NotFoundError{Msg: "template not found"}
			}
			return ProjectDTO{}, fmt.Errorf("landing_pages: load template: %w", err)
		}
		definition = tmpl.DefinitionJSON
	}

	if err := ValidateDefinition(definition); err != nil {
		return ProjectDTO{}, &ValidationError{Msg: fmt.Sprintf("invalid definition: %s", err.Error())}
	}

	p, err := s.repo.CreateProject(ctx, repositories.LandingPageProject{
		Name:           strings.TrimSpace(input.Name),
		Description:    input.Description,
		DefinitionJSON: definition,
		CreatedBy:      actorID,
	})
	if err != nil {
		if strings.Contains(err.Error(), "name already exists") {
			return ProjectDTO{}, &ConflictError{Msg: "a landing page with this name already exists"}
		}
		return ProjectDTO{}, fmt.Errorf("landing_pages: create: %w", err)
	}

	resType := "landing_page"
	s.logAudit(ctx, "landing_page.create", &actorID, actorName, &resType, &p.ID, map[string]any{"name": p.Name})
	return toProjectDTO(p), nil
}

// Get returns a project by ID.
func (s *Service) Get(ctx context.Context, id string) (ProjectDTO, error) {
	p, err := s.repo.GetProjectByID(ctx, id)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			return ProjectDTO{}, &NotFoundError{Msg: "landing page not found"}
		}
		return ProjectDTO{}, fmt.Errorf("landing_pages: get: %w", err)
	}
	return toProjectDTO(p), nil
}

// Update modifies a project.
func (s *Service) Update(ctx context.Context, id string, input UpdateProjectInput, actorID, actorName string) (ProjectDTO, error) {
	if input.Name != nil {
		n := strings.TrimSpace(*input.Name)
		if n == "" {
			return ProjectDTO{}, &ValidationError{Msg: "name cannot be empty"}
		}
		if len(n) > 255 {
			return ProjectDTO{}, &ValidationError{Msg: "name must be 255 characters or fewer"}
		}
		input.Name = &n
	}

	if input.DefinitionJSON != nil {
		if err := ValidateDefinition(input.DefinitionJSON); err != nil {
			return ProjectDTO{}, &ValidationError{Msg: fmt.Sprintf("invalid definition: %s", err.Error())}
		}
	}

	p, err := s.repo.UpdateProject(ctx, id, repositories.LandingPageProjectUpdate{
		Name:           input.Name,
		Description:    input.Description,
		DefinitionJSON: input.DefinitionJSON,
	})
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			return ProjectDTO{}, &NotFoundError{Msg: "landing page not found"}
		}
		if strings.Contains(err.Error(), "name already exists") {
			return ProjectDTO{}, &ConflictError{Msg: "a landing page with this name already exists"}
		}
		return ProjectDTO{}, fmt.Errorf("landing_pages: update: %w", err)
	}

	resType := "landing_page"
	s.logAudit(ctx, "landing_page.update", &actorID, actorName, &resType, &p.ID, map[string]any{"name": p.Name})
	return toProjectDTO(p), nil
}

// Delete soft-deletes a project.
func (s *Service) Delete(ctx context.Context, id string, actorID, actorName string) error {
	// Verify exists first for the audit log.
	p, err := s.repo.GetProjectByID(ctx, id)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			return &NotFoundError{Msg: "landing page not found"}
		}
		return fmt.Errorf("landing_pages: delete: %w", err)
	}

	if err := s.repo.DeleteProject(ctx, id); err != nil {
		return fmt.Errorf("landing_pages: delete: %w", err)
	}

	resType := "landing_page"
	s.logAudit(ctx, "landing_page.delete", &actorID, actorName, &resType, &p.ID, map[string]any{"name": p.Name})
	return nil
}

// List returns paginated projects.
func (s *Service) List(ctx context.Context, input ListProjectInput) ([]ProjectDTO, int, error) {
	result, err := s.repo.ListProjects(ctx, repositories.LandingPageProjectFilters{
		Name:    input.Name,
		Page:    input.Page,
		PerPage: input.PerPage,
	})
	if err != nil {
		return nil, 0, fmt.Errorf("landing_pages: list: %w", err)
	}

	dtos := make([]ProjectDTO, len(result.Projects))
	for i, p := range result.Projects {
		dtos[i] = toProjectDTO(p)
	}
	return dtos, result.Total, nil
}

// Duplicate creates a copy of an existing project.
func (s *Service) Duplicate(ctx context.Context, id string, actorID, actorName string) (ProjectDTO, error) {
	src, err := s.repo.GetProjectByID(ctx, id)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			return ProjectDTO{}, &NotFoundError{Msg: "landing page not found"}
		}
		return ProjectDTO{}, fmt.Errorf("landing_pages: duplicate: %w", err)
	}

	newName := fmt.Sprintf("%s (Copy)", src.Name)
	if len(newName) > 255 {
		newName = newName[:255]
	}

	p, err := s.repo.CreateProject(ctx, repositories.LandingPageProject{
		Name:           newName,
		Description:    src.Description,
		DefinitionJSON: src.DefinitionJSON,
		CreatedBy:      actorID,
	})
	if err != nil {
		return ProjectDTO{}, fmt.Errorf("landing_pages: duplicate: %w", err)
	}

	resType := "landing_page"
	s.logAudit(ctx, "landing_page.duplicate", &actorID, actorName, &resType, &p.ID, map[string]any{
		"source_id": id,
		"name":      p.Name,
	})
	return toProjectDTO(p), nil
}

// ---------- Templates ----------

// ListTemplates returns available templates for the user.
func (s *Service) ListTemplates(ctx context.Context, userID string) ([]TemplateDTO, error) {
	templates, err := s.repo.ListTemplates(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("landing_pages: list templates: %w", err)
	}

	dtos := make([]TemplateDTO, len(templates))
	for i, t := range templates {
		dtos[i] = toTemplateDTO(t)
	}
	return dtos, nil
}

// SaveAsTemplate saves a project as a reusable template.
func (s *Service) SaveAsTemplate(ctx context.Context, projectID string, input SaveTemplateInput, actorID, actorName string) (TemplateDTO, error) {
	if strings.TrimSpace(input.Name) == "" {
		return TemplateDTO{}, &ValidationError{Msg: "template name is required"}
	}
	if len(input.Name) > 255 {
		return TemplateDTO{}, &ValidationError{Msg: "template name must be 255 characters or fewer"}
	}

	project, err := s.repo.GetProjectByID(ctx, projectID)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			return TemplateDTO{}, &NotFoundError{Msg: "landing page not found"}
		}
		return TemplateDTO{}, fmt.Errorf("landing_pages: save as template: %w", err)
	}

	category := input.Category
	if category == "" {
		category = "custom"
	}

	t, err := s.repo.CreateTemplate(ctx, repositories.LandingPageTemplate{
		Name:           strings.TrimSpace(input.Name),
		Description:    input.Description,
		Category:       category,
		DefinitionJSON: project.DefinitionJSON,
		CreatedBy:      actorID,
		IsShared:       input.IsShared,
	})
	if err != nil {
		return TemplateDTO{}, fmt.Errorf("landing_pages: save as template: %w", err)
	}

	resType := "landing_page_template"
	s.logAudit(ctx, "landing_page.template.create", &actorID, actorName, &resType, &t.ID, map[string]any{
		"name":       t.Name,
		"project_id": projectID,
	})
	return toTemplateDTO(t), nil
}

// UpdateTemplateShared toggles the is_shared flag on a template.
func (s *Service) UpdateTemplateShared(ctx context.Context, id string, isShared bool, actorID, actorName string) (TemplateDTO, error) {
	// Verify ownership: only the creator can toggle sharing.
	t, err := s.repo.GetTemplateByID(ctx, id)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			return TemplateDTO{}, &NotFoundError{Msg: "template not found"}
		}
		return TemplateDTO{}, fmt.Errorf("landing_pages: update template shared: %w", err)
	}
	if t.CreatedBy != actorID {
		return TemplateDTO{}, &ValidationError{Msg: "only the template creator can change sharing"}
	}

	updated, err := s.repo.UpdateTemplateShared(ctx, id, isShared)
	if err != nil {
		return TemplateDTO{}, fmt.Errorf("landing_pages: update template shared: %w", err)
	}

	resType := "landing_page_template"
	s.logAudit(ctx, "landing_page.template.update_shared", &actorID, actorName, &resType, &id, map[string]any{
		"is_shared": isShared,
	})
	return toTemplateDTO(updated), nil
}

// DeleteTemplate deletes a template owned by the actor.
func (s *Service) DeleteTemplate(ctx context.Context, id string, actorID, actorName string) error {
	t, err := s.repo.GetTemplateByID(ctx, id)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			return &NotFoundError{Msg: "template not found"}
		}
		return fmt.Errorf("landing_pages: delete template: %w", err)
	}
	if t.CreatedBy != actorID {
		return &ValidationError{Msg: "only the template creator can delete a template"}
	}

	if err := s.repo.DeleteTemplate(ctx, id); err != nil {
		return fmt.Errorf("landing_pages: delete template: %w", err)
	}

	resType := "landing_page_template"
	s.logAudit(ctx, "landing_page.template.delete", &actorID, actorName, &resType, &id, nil)
	return nil
}

// ---------- Preview ----------

// GeneratePreview generates preview HTML for a project.
func (s *Service) GeneratePreview(ctx context.Context, id string, input PreviewInput) (string, error) {
	project, err := s.repo.GetProjectByID(ctx, id)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			return "", &NotFoundError{Msg: "landing page not found"}
		}
		return "", fmt.Errorf("landing_pages: preview: %w", err)
	}

	html, err := RenderPreviewHTML(project.DefinitionJSON, input.PageIndex)
	if err != nil {
		return "", &ValidationError{Msg: fmt.Sprintf("preview generation failed: %s", err.Error())}
	}
	return html, nil
}

// ---------- HTML Import ----------

// ImportHTML imports HTML content into a project.
func (s *Service) ImportHTML(ctx context.Context, id string, input ImportHTMLInput, actorID, actorName string) (ProjectDTO, error) {
	if strings.TrimSpace(input.HTML) == "" {
		return ProjectDTO{}, &ValidationError{Msg: "HTML content is required"}
	}
	if input.Mode != "builder" && input.Mode != "raw" {
		return ProjectDTO{}, &ValidationError{Msg: "mode must be 'builder' or 'raw'"}
	}
	if len(input.HTML) > 5*1024*1024 {
		return ProjectDTO{}, &ValidationError{Msg: "HTML content exceeds 5 MB limit"}
	}

	project, err := s.repo.GetProjectByID(ctx, id)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			return ProjectDTO{}, &NotFoundError{Msg: "landing page not found"}
		}
		return ProjectDTO{}, fmt.Errorf("landing_pages: import: %w", err)
	}

	definition := project.DefinitionJSON
	if input.Mode == "builder" {
		definition, err = ParseHTMLToDefinition(input.HTML, definition)
	} else {
		definition, err = ImportRawHTML(input.HTML, definition)
	}
	if err != nil {
		return ProjectDTO{}, &ValidationError{Msg: fmt.Sprintf("import failed: %s", err.Error())}
	}

	p, err := s.repo.UpdateProject(ctx, id, repositories.LandingPageProjectUpdate{
		DefinitionJSON: definition,
	})
	if err != nil {
		return ProjectDTO{}, fmt.Errorf("landing_pages: import update: %w", err)
	}

	resType := "landing_page"
	s.logAudit(ctx, "landing_page.import", &actorID, actorName, &resType, &p.ID, map[string]any{
		"mode":     input.Mode,
		"filename": input.Filename,
	})
	return toProjectDTO(p), nil
}

// ImportZIP imports a ZIP archive containing HTML + assets into a project.
func (s *Service) ImportZIP(ctx context.Context, id string, zipReader *zip.Reader, filename string, actorID, actorName string) (ProjectDTO, error) {
	project, err := s.repo.GetProjectByID(ctx, id)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			return ProjectDTO{}, &NotFoundError{Msg: "landing page not found"}
		}
		return ProjectDTO{}, fmt.Errorf("landing_pages: import zip: %w", err)
	}

	definition, err := ImportFromZIP(zipReader, project.DefinitionJSON)
	if err != nil {
		return ProjectDTO{}, &ValidationError{Msg: fmt.Sprintf("ZIP import failed: %s", err.Error())}
	}

	p, err := s.repo.UpdateProject(ctx, id, repositories.LandingPageProjectUpdate{
		DefinitionJSON: definition,
	})
	if err != nil {
		return ProjectDTO{}, fmt.Errorf("landing_pages: import zip update: %w", err)
	}

	resType := "landing_page"
	s.logAudit(ctx, "landing_page.import_zip", &actorID, actorName, &resType, &p.ID, map[string]any{
		"filename": filename,
	})
	return toProjectDTO(p), nil
}

// ---------- URL Cloning ----------

// CloneFromURL clones a web page by URL into a project.
func (s *Service) CloneFromURL(ctx context.Context, id string, input CloneURLInput, actorID, actorName string) (ProjectDTO, error) {
	if strings.TrimSpace(input.URL) == "" {
		return ProjectDTO{}, &ValidationError{Msg: "URL is required"}
	}
	if !strings.HasPrefix(input.URL, "http://") && !strings.HasPrefix(input.URL, "https://") {
		return ProjectDTO{}, &ValidationError{Msg: "URL must start with http:// or https://"}
	}

	_, err := s.repo.GetProjectByID(ctx, id)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			return ProjectDTO{}, &NotFoundError{Msg: "landing page not found"}
		}
		return ProjectDTO{}, fmt.Errorf("landing_pages: clone url: %w", err)
	}

	definition, err := ClonePageFromURL(input.URL, input.IncludeJS, input.StripTracking)
	if err != nil {
		return ProjectDTO{}, &ValidationError{Msg: fmt.Sprintf("clone failed: %s", err.Error())}
	}

	p, err := s.repo.UpdateProject(ctx, id, repositories.LandingPageProjectUpdate{
		DefinitionJSON: definition,
	})
	if err != nil {
		return ProjectDTO{}, fmt.Errorf("landing_pages: clone url update: %w", err)
	}

	resType := "landing_page"
	s.logAudit(ctx, "landing_page.clone_url", &actorID, actorName, &resType, &p.ID, map[string]any{
		"source_url": input.URL,
	})
	return toProjectDTO(p), nil
}

// ---------- Component Types ----------

// ListComponentTypes returns all available component types for the UI.
func (s *Service) ListComponentTypes() []ComponentTypeDTO {
	return GetComponentTypes()
}

// ---------- Helpers ----------

func toProjectDTO(p repositories.LandingPageProject) ProjectDTO {
	return ProjectDTO{
		ID:             p.ID,
		Name:           p.Name,
		Description:    p.Description,
		DefinitionJSON: p.DefinitionJSON,
		CreatedBy:      p.CreatedBy,
		CreatedAt:      p.CreatedAt,
		UpdatedAt:      p.UpdatedAt,
	}
}

func toTemplateDTO(t repositories.LandingPageTemplate) TemplateDTO {
	return TemplateDTO{
		ID:             t.ID,
		Name:           t.Name,
		Description:    t.Description,
		Category:       t.Category,
		DefinitionJSON: t.DefinitionJSON,
		CreatedBy:      t.CreatedBy,
		IsShared:       t.IsShared,
		CreatedAt:      t.CreatedAt,
	}
}

func toBuildDTO(b repositories.LandingPageBuild) BuildDTO {
	return BuildDTO{
		ID:                b.ID,
		ProjectID:         b.ProjectID,
		CampaignID:        b.CampaignID,
		Seed:              b.Seed,
		Strategy:          b.Strategy,
		BuildManifestJSON: b.BuildManifestJSON,
		BuildLog:          b.BuildLog,
		BinaryPath:        b.BinaryPath,
		BinaryHash:        b.BinaryHash,
		Status:            b.Status,
		Port:              b.Port,
		CreatedAt:         b.CreatedAt,
	}
}

func (s *Service) logAudit(ctx context.Context, action string, actorID *string, actorName string, resType, resID *string, details map[string]any) {
	entry := auditsvc.LogEntry{
		Category:     auditsvc.CategoryUserActivity,
		Severity:     auditsvc.SeverityInfo,
		ActorType:    auditsvc.ActorTypeUser,
		ActorID:      actorID,
		ActorLabel:   actorName,
		Action:       action,
		ResourceType: resType,
		ResourceID:   resID,
		Details:      details,
	}
	if actorID == nil {
		entry.ActorType = auditsvc.ActorTypeSystem
	}
	_ = s.auditSvc.Log(ctx, entry)
}

// defaultDefinition returns the default empty page definition.
func defaultDefinition() map[string]any {
	return map[string]any{
		"schema_version": 1,
		"pages": []any{
			map[string]any{
				"page_id":        "page-1",
				"name":           "Landing Page",
				"route":          "/",
				"title":          "Landing Page",
				"favicon":        "",
				"meta_tags":      []any{},
				"component_tree": []any{},
				"page_styles":    "",
				"page_js":        "",
			},
		},
		"global_styles": "",
		"global_js":     "",
		"theme":         map[string]any{},
		"navigation":    []any{},
	}
}

// Suppress unused warning for toBuildDTO (used in future compilation session).
var _ = toBuildDTO

// Suppress unused warning for json import.
var _ = json.Marshal
