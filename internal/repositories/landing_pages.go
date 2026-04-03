package repositories

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
)

// LandingPageProject is the DB model for a landing_page_projects row.
type LandingPageProject struct {
	ID             string
	Name           string
	Description    string
	DefinitionJSON map[string]any
	CreatedBy      string
	AssignedPort   *int
	DeletedAt      *time.Time
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

// LandingPageProjectUpdate holds mutable fields for update.
type LandingPageProjectUpdate struct {
	Name           *string
	Description    *string
	DefinitionJSON map[string]any // nil = no change
}

// LandingPageProjectFilters controls filtering for project list.
type LandingPageProjectFilters struct {
	Name      string
	CreatedBy string
	Page      int
	PerPage   int
}

// LandingPageProjectListResult holds paginated results.
type LandingPageProjectListResult struct {
	Projects []LandingPageProject
	Total    int
}

// LandingPageTemplate is the DB model for a landing_page_templates row.
type LandingPageTemplate struct {
	ID             string
	Name           string
	Description    string
	Category       string
	DefinitionJSON map[string]any
	CreatedBy      string
	IsShared       bool
	CreatedAt      time.Time
}

// LandingPageBuild is the DB model for a landing_page_builds row.
type LandingPageBuild struct {
	ID                string         `json:"id"`
	ProjectID         string         `json:"project_id"`
	CampaignID        *string        `json:"campaign_id"`
	Seed              int64          `json:"seed"`
	Strategy          string         `json:"strategy"`
	BuildManifestJSON map[string]any `json:"build_manifest_json"`
	BuildLog          string         `json:"build_log"`
	BinaryPath        *string        `json:"binary_path,omitempty"`
	BinaryHash        *string        `json:"binary_hash,omitempty"`
	Status            string         `json:"status"`
	Port              *int           `json:"port,omitempty"`
	BuildToken        *string        `json:"build_token,omitempty"`
	CreatedAt         time.Time      `json:"created_at"`
}

// LandingPageHealthCheck is the DB model for a landing_page_health_checks row.
type LandingPageHealthCheck struct {
	ID             string
	BuildID        string
	Status         string
	ResponseTimeMs int
	CheckedAt      time.Time
}

// LandingPageRepository handles landing page database operations.
type LandingPageRepository struct {
	db *sql.DB
}

// NewLandingPageRepository creates a new LandingPageRepository.
func NewLandingPageRepository(db *sql.DB) *LandingPageRepository {
	return &LandingPageRepository{db: db}
}

const lpProjectColumns = `id, name, description, definition_json, created_by, assigned_port, deleted_at, created_at, updated_at`

func scanLPProject(row interface{ Scan(...any) error }) (LandingPageProject, error) {
	var p LandingPageProject
	var defJSON []byte
	err := row.Scan(&p.ID, &p.Name, &p.Description, &defJSON, &p.CreatedBy, &p.AssignedPort, &p.DeletedAt, &p.CreatedAt, &p.UpdatedAt)
	if err != nil {
		return LandingPageProject{}, err
	}
	p.DefinitionJSON = make(map[string]any)
	if defJSON != nil {
		_ = json.Unmarshal(defJSON, &p.DefinitionJSON)
	}
	return p, nil
}

// ---------- Project CRUD ----------

// CreateProject inserts a new landing page project.
func (r *LandingPageRepository) CreateProject(ctx context.Context, p LandingPageProject) (LandingPageProject, error) {
	p.ID = uuid.New().String()
	defJSON, err := json.Marshal(p.DefinitionJSON)
	if err != nil {
		return LandingPageProject{}, fmt.Errorf("landing_pages: create: marshal definition: %w", err)
	}

	q := fmt.Sprintf(`
		INSERT INTO landing_page_projects (id, name, description, definition_json, created_by)
		VALUES ($1,$2,$3,$4,$5)
		RETURNING %s`, lpProjectColumns)

	result, err := scanLPProject(r.db.QueryRowContext(ctx, q, p.ID, p.Name, p.Description, defJSON, p.CreatedBy))
	if err != nil {
		if strings.Contains(err.Error(), "idx_lpp_name_active") {
			return LandingPageProject{}, fmt.Errorf("landing_pages: name already exists")
		}
		return LandingPageProject{}, fmt.Errorf("landing_pages: create: %w", err)
	}
	return result, nil
}

// GetProjectByID returns a project by ID (excluding soft-deleted).
func (r *LandingPageRepository) GetProjectByID(ctx context.Context, id string) (LandingPageProject, error) {
	q := fmt.Sprintf(`SELECT %s FROM landing_page_projects WHERE id = $1 AND deleted_at IS NULL`, lpProjectColumns)
	p, err := scanLPProject(r.db.QueryRowContext(ctx, q, id))
	if err == sql.ErrNoRows {
		return LandingPageProject{}, fmt.Errorf("landing_pages: not found")
	}
	if err != nil {
		return LandingPageProject{}, fmt.Errorf("landing_pages: get by id: %w", err)
	}
	return p, nil
}

// UpdateProject modifies a project. Only non-nil fields in the update struct are changed.
func (r *LandingPageRepository) UpdateProject(ctx context.Context, id string, u LandingPageProjectUpdate) (LandingPageProject, error) {
	sets := []string{}
	args := []any{}
	idx := 1

	addSet := func(col string, val any) {
		sets = append(sets, fmt.Sprintf("%s = $%d", col, idx))
		args = append(args, val)
		idx++
	}

	if u.Name != nil {
		addSet("name", *u.Name)
	}
	if u.Description != nil {
		addSet("description", *u.Description)
	}
	if u.DefinitionJSON != nil {
		defJSON, err := json.Marshal(u.DefinitionJSON)
		if err != nil {
			return LandingPageProject{}, fmt.Errorf("landing_pages: update: marshal definition: %w", err)
		}
		addSet("definition_json", defJSON)
	}

	if len(sets) == 0 {
		return r.GetProjectByID(ctx, id)
	}

	args = append(args, id)
	q := fmt.Sprintf(`UPDATE landing_page_projects SET %s WHERE id = $%d AND deleted_at IS NULL RETURNING %s`,
		strings.Join(sets, ", "), idx, lpProjectColumns)

	p, err := scanLPProject(r.db.QueryRowContext(ctx, q, args...))
	if err == sql.ErrNoRows {
		return LandingPageProject{}, fmt.Errorf("landing_pages: not found")
	}
	if err != nil {
		if strings.Contains(err.Error(), "idx_lpp_name_active") {
			return LandingPageProject{}, fmt.Errorf("landing_pages: name already exists")
		}
		return LandingPageProject{}, fmt.Errorf("landing_pages: update: %w", err)
	}
	return p, nil
}

// SetProjectPort assigns a persistent port to a landing page project.
func (r *LandingPageRepository) SetProjectPort(ctx context.Context, projectID string, port int) error {
	_, err := r.db.ExecContext(ctx,
		`UPDATE landing_page_projects SET assigned_port = $1 WHERE id = $2 AND deleted_at IS NULL`,
		port, projectID)
	if err != nil {
		return fmt.Errorf("landing_pages: set port: %w", err)
	}
	return nil
}

// GetProjectPort returns the assigned port for a project, or nil if none.
func (r *LandingPageRepository) GetProjectPort(ctx context.Context, projectID string) (*int, error) {
	var port *int
	err := r.db.QueryRowContext(ctx,
		`SELECT assigned_port FROM landing_page_projects WHERE id = $1 AND deleted_at IS NULL`,
		projectID).Scan(&port)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("landing_pages: not found")
	}
	if err != nil {
		return nil, fmt.Errorf("landing_pages: get port: %w", err)
	}
	return port, nil
}

// DeleteProject soft-deletes a project.
func (r *LandingPageRepository) DeleteProject(ctx context.Context, id string) error {
	res, err := r.db.ExecContext(ctx,
		`UPDATE landing_page_projects SET deleted_at = now() WHERE id = $1 AND deleted_at IS NULL`, id)
	if err != nil {
		return fmt.Errorf("landing_pages: delete: %w", err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return fmt.Errorf("landing_pages: not found")
	}
	return nil
}

// ListProjects returns paginated, filtered projects.
func (r *LandingPageRepository) ListProjects(ctx context.Context, f LandingPageProjectFilters) (LandingPageProjectListResult, error) {
	where := []string{"deleted_at IS NULL"}
	args := []any{}
	idx := 1

	if f.Name != "" {
		where = append(where, fmt.Sprintf("LOWER(name) LIKE LOWER('%%' || $%d || '%%')", idx))
		args = append(args, f.Name)
		idx++
	}
	if f.CreatedBy != "" {
		where = append(where, fmt.Sprintf("created_by = $%d", idx))
		args = append(args, f.CreatedBy)
		idx++
	}

	whereClause := "WHERE " + strings.Join(where, " AND ")

	countQ := fmt.Sprintf("SELECT COUNT(*) FROM landing_page_projects %s", whereClause)
	var total int
	if err := r.db.QueryRowContext(ctx, countQ, args...).Scan(&total); err != nil {
		return LandingPageProjectListResult{}, fmt.Errorf("landing_pages: list count: %w", err)
	}

	if f.Page < 1 {
		f.Page = 1
	}
	if f.PerPage < 1 || f.PerPage > 100 {
		f.PerPage = 25
	}
	offset := (f.Page - 1) * f.PerPage

	args = append(args, f.PerPage, offset)
	q := fmt.Sprintf(`SELECT %s FROM landing_page_projects %s ORDER BY created_at DESC LIMIT $%d OFFSET $%d`,
		lpProjectColumns, whereClause, idx, idx+1)

	rows, err := r.db.QueryContext(ctx, q, args...)
	if err != nil {
		return LandingPageProjectListResult{}, fmt.Errorf("landing_pages: list: %w", err)
	}
	defer rows.Close()

	var projects []LandingPageProject
	for rows.Next() {
		p, err := scanLPProject(rows)
		if err != nil {
			return LandingPageProjectListResult{}, fmt.Errorf("landing_pages: list scan: %w", err)
		}
		projects = append(projects, p)
	}
	if projects == nil {
		projects = []LandingPageProject{}
	}

	return LandingPageProjectListResult{Projects: projects, Total: total}, nil
}

// ---------- Templates ----------

// CreateTemplate inserts a new landing page template.
func (r *LandingPageRepository) CreateTemplate(ctx context.Context, t LandingPageTemplate) (LandingPageTemplate, error) {
	t.ID = uuid.New().String()
	defJSON, err := json.Marshal(t.DefinitionJSON)
	if err != nil {
		return LandingPageTemplate{}, fmt.Errorf("landing_pages: create template: marshal: %w", err)
	}

	err = r.db.QueryRowContext(ctx,
		`INSERT INTO landing_page_templates (id, name, description, category, definition_json, created_by, is_shared)
		 VALUES ($1,$2,$3,$4,$5,$6,$7)
		 RETURNING id, name, description, category, definition_json, created_by, is_shared, created_at`,
		t.ID, t.Name, t.Description, t.Category, defJSON, t.CreatedBy, t.IsShared,
	).Scan(&t.ID, &t.Name, &t.Description, &t.Category, &defJSON, &t.CreatedBy, &t.IsShared, &t.CreatedAt)
	if err != nil {
		return LandingPageTemplate{}, fmt.Errorf("landing_pages: create template: %w", err)
	}
	t.DefinitionJSON = make(map[string]any)
	_ = json.Unmarshal(defJSON, &t.DefinitionJSON)
	return t, nil
}

// ListTemplates returns all landing page templates (shared or owned by user).
func (r *LandingPageRepository) ListTemplates(ctx context.Context, userID string) ([]LandingPageTemplate, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT id, name, description, category, definition_json, created_by, is_shared, created_at
		 FROM landing_page_templates
		 WHERE is_shared = true OR created_by = $1
		 ORDER BY category, name`, userID)
	if err != nil {
		return nil, fmt.Errorf("landing_pages: list templates: %w", err)
	}
	defer rows.Close()

	var result []LandingPageTemplate
	for rows.Next() {
		var t LandingPageTemplate
		var defJSON []byte
		if err := rows.Scan(&t.ID, &t.Name, &t.Description, &t.Category, &defJSON, &t.CreatedBy, &t.IsShared, &t.CreatedAt); err != nil {
			return nil, fmt.Errorf("landing_pages: scan template: %w", err)
		}
		t.DefinitionJSON = make(map[string]any)
		_ = json.Unmarshal(defJSON, &t.DefinitionJSON)
		result = append(result, t)
	}
	if result == nil {
		result = []LandingPageTemplate{}
	}
	return result, nil
}

// GetTemplateByID returns a single template by ID.
func (r *LandingPageRepository) GetTemplateByID(ctx context.Context, id string) (LandingPageTemplate, error) {
	var t LandingPageTemplate
	var defJSON []byte
	err := r.db.QueryRowContext(ctx,
		`SELECT id, name, description, category, definition_json, created_by, is_shared, created_at
		 FROM landing_page_templates WHERE id = $1`, id,
	).Scan(&t.ID, &t.Name, &t.Description, &t.Category, &defJSON, &t.CreatedBy, &t.IsShared, &t.CreatedAt)
	if err == sql.ErrNoRows {
		return LandingPageTemplate{}, fmt.Errorf("landing_pages: template not found")
	}
	if err != nil {
		return LandingPageTemplate{}, fmt.Errorf("landing_pages: get template: %w", err)
	}
	t.DefinitionJSON = make(map[string]any)
	_ = json.Unmarshal(defJSON, &t.DefinitionJSON)
	return t, nil
}

// DeleteTemplate removes a template by ID.
func (r *LandingPageRepository) DeleteTemplate(ctx context.Context, id string) error {
	res, err := r.db.ExecContext(ctx, `DELETE FROM landing_page_templates WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("landing_pages: delete template: %w", err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return fmt.Errorf("landing_pages: template not found")
	}
	return nil
}

// UpdateTemplateShared updates the is_shared flag on a template.
func (r *LandingPageRepository) UpdateTemplateShared(ctx context.Context, id string, isShared bool) (LandingPageTemplate, error) {
	row := r.db.QueryRowContext(ctx, `
		UPDATE landing_page_templates
		SET is_shared = $2
		WHERE id = $1
		RETURNING id, name, description, category, definition_json, created_by, is_shared, created_at
	`, id, isShared)

	var t LandingPageTemplate
	var defJSON []byte
	if err := row.Scan(&t.ID, &t.Name, &t.Description, &t.Category, &defJSON, &t.CreatedBy, &t.IsShared, &t.CreatedAt); err != nil {
		return LandingPageTemplate{}, fmt.Errorf("landing_pages: update template shared: %w", err)
	}
	if err := json.Unmarshal(defJSON, &t.DefinitionJSON); err != nil {
		return LandingPageTemplate{}, fmt.Errorf("landing_pages: unmarshal template def: %w", err)
	}
	return t, nil
}

// ---------- Builds ----------

// CreateBuild inserts a new build record.
func (r *LandingPageRepository) CreateBuild(ctx context.Context, b LandingPageBuild) (LandingPageBuild, error) {
	b.ID = uuid.New().String()
	manifestJSON, err := json.Marshal(b.BuildManifestJSON)
	if err != nil {
		return LandingPageBuild{}, fmt.Errorf("landing_pages: create build: marshal manifest: %w", err)
	}

	err = r.db.QueryRowContext(ctx,
		`INSERT INTO landing_page_builds (id, project_id, campaign_id, seed, strategy, build_manifest_json, build_log, status, build_token)
		 VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9)
		 RETURNING id, project_id, campaign_id, seed, strategy, build_manifest_json, build_log,
		           binary_path, binary_hash, status, port, build_token, created_at`,
		b.ID, b.ProjectID, b.CampaignID, b.Seed, b.Strategy, manifestJSON, b.BuildLog, b.Status, b.BuildToken,
	).Scan(&b.ID, &b.ProjectID, &b.CampaignID, &b.Seed, &b.Strategy, &manifestJSON, &b.BuildLog,
		&b.BinaryPath, &b.BinaryHash, &b.Status, &b.Port, &b.BuildToken, &b.CreatedAt)
	if err != nil {
		return LandingPageBuild{}, fmt.Errorf("landing_pages: create build: %w", err)
	}
	b.BuildManifestJSON = make(map[string]any)
	_ = json.Unmarshal(manifestJSON, &b.BuildManifestJSON)
	return b, nil
}

// GetBuildByID returns a build by ID.
func (r *LandingPageRepository) GetBuildByID(ctx context.Context, id string) (LandingPageBuild, error) {
	var b LandingPageBuild
	var manifestJSON []byte
	err := r.db.QueryRowContext(ctx,
		`SELECT id, project_id, campaign_id, seed, strategy, build_manifest_json, build_log,
		        binary_path, binary_hash, status, port, build_token, created_at
		 FROM landing_page_builds WHERE id = $1`, id,
	).Scan(&b.ID, &b.ProjectID, &b.CampaignID, &b.Seed, &b.Strategy, &manifestJSON, &b.BuildLog,
		&b.BinaryPath, &b.BinaryHash, &b.Status, &b.Port, &b.BuildToken, &b.CreatedAt)
	if err == sql.ErrNoRows {
		return LandingPageBuild{}, fmt.Errorf("landing_pages: build not found")
	}
	if err != nil {
		return LandingPageBuild{}, fmt.Errorf("landing_pages: get build: %w", err)
	}
	b.BuildManifestJSON = make(map[string]any)
	_ = json.Unmarshal(manifestJSON, &b.BuildManifestJSON)
	return b, nil
}

// ListBuildsByProject returns builds for a project.
func (r *LandingPageRepository) ListBuildsByProject(ctx context.Context, projectID string) ([]LandingPageBuild, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT id, project_id, campaign_id, seed, strategy, build_manifest_json, build_log,
		        binary_path, binary_hash, status, port, build_token, created_at
		 FROM landing_page_builds WHERE project_id = $1 ORDER BY created_at DESC`, projectID)
	if err != nil {
		return nil, fmt.Errorf("landing_pages: list builds: %w", err)
	}
	defer rows.Close()

	var result []LandingPageBuild
	for rows.Next() {
		var b LandingPageBuild
		var manifestJSON []byte
		if err := rows.Scan(&b.ID, &b.ProjectID, &b.CampaignID, &b.Seed, &b.Strategy, &manifestJSON, &b.BuildLog,
			&b.BinaryPath, &b.BinaryHash, &b.Status, &b.Port, &b.BuildToken, &b.CreatedAt); err != nil {
			return nil, fmt.Errorf("landing_pages: scan build: %w", err)
		}
		b.BuildManifestJSON = make(map[string]any)
		_ = json.Unmarshal(manifestJSON, &b.BuildManifestJSON)
		result = append(result, b)
	}
	if result == nil {
		result = []LandingPageBuild{}
	}
	return result, nil
}

// ListBuildsByCampaign returns builds for a campaign.
func (r *LandingPageRepository) ListBuildsByCampaign(ctx context.Context, campaignID string) ([]LandingPageBuild, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT id, project_id, campaign_id, seed, strategy, build_manifest_json, build_log,
		        binary_path, binary_hash, status, port, build_token, created_at
		 FROM landing_page_builds WHERE campaign_id = $1 ORDER BY created_at DESC`, campaignID)
	if err != nil {
		return nil, fmt.Errorf("landing_pages: list builds by campaign: %w", err)
	}
	defer rows.Close()

	var result []LandingPageBuild
	for rows.Next() {
		var b LandingPageBuild
		var manifestJSON []byte
		if err := rows.Scan(&b.ID, &b.ProjectID, &b.CampaignID, &b.Seed, &b.Strategy, &manifestJSON, &b.BuildLog,
			&b.BinaryPath, &b.BinaryHash, &b.Status, &b.Port, &b.BuildToken, &b.CreatedAt); err != nil {
			return nil, fmt.Errorf("landing_pages: scan build by campaign: %w", err)
		}
		b.BuildManifestJSON = make(map[string]any)
		_ = json.Unmarshal(manifestJSON, &b.BuildManifestJSON)
		result = append(result, b)
	}
	if result == nil {
		result = []LandingPageBuild{}
	}
	return result, nil
}

// UpdateBuildStatus updates the status and optional fields of a build.
func (r *LandingPageRepository) UpdateBuildStatus(ctx context.Context, id, status string, binaryPath, binaryHash *string, port *int) error {
	_, err := r.db.ExecContext(ctx,
		`UPDATE landing_page_builds SET status = $1, binary_path = COALESCE($2, binary_path),
		 binary_hash = COALESCE($3, binary_hash), port = COALESCE($4, port)
		 WHERE id = $5`,
		status, binaryPath, binaryHash, port, id)
	if err != nil {
		return fmt.Errorf("landing_pages: update build status: %w", err)
	}
	return nil
}

// ResetStaleBuilds sets all builds with status 'running' or 'starting' to 'stopped'.
// This is used on server startup to clean up stale state from a previous process.
func (r *LandingPageRepository) ResetStaleBuilds(ctx context.Context) (int64, error) {
	result, err := r.db.ExecContext(ctx,
		`UPDATE landing_page_builds SET status = 'stopped' WHERE status IN ('running', 'starting')`)
	if err != nil {
		return 0, fmt.Errorf("landing_pages: reset stale builds: %w", err)
	}
	return result.RowsAffected()
}

// AppendBuildLog appends text to a build's log.
func (r *LandingPageRepository) AppendBuildLog(ctx context.Context, id, logText string) error {
	_, err := r.db.ExecContext(ctx,
		`UPDATE landing_page_builds SET build_log = build_log || $1 WHERE id = $2`, logText, id)
	if err != nil {
		return fmt.Errorf("landing_pages: append build log: %w", err)
	}
	return nil
}

// UpdateBuildManifest updates the build manifest JSON.
func (r *LandingPageRepository) UpdateBuildManifest(ctx context.Context, id string, manifest map[string]any) error {
	manifestJSON, err := json.Marshal(manifest)
	if err != nil {
		return fmt.Errorf("landing_pages: update manifest: marshal: %w", err)
	}
	_, err = r.db.ExecContext(ctx,
		`UPDATE landing_page_builds SET build_manifest_json = $1 WHERE id = $2`, manifestJSON, id)
	if err != nil {
		return fmt.Errorf("landing_pages: update manifest: %w", err)
	}
	return nil
}

// GetBuildByToken returns a build by its build token.
func (r *LandingPageRepository) GetBuildByToken(ctx context.Context, token string) (LandingPageBuild, error) {
	var b LandingPageBuild
	var manifestJSON []byte
	err := r.db.QueryRowContext(ctx,
		`SELECT id, project_id, campaign_id, seed, strategy, build_manifest_json, build_log,
		        binary_path, binary_hash, status, port, build_token, created_at
		 FROM landing_page_builds WHERE build_token = $1`, token,
	).Scan(&b.ID, &b.ProjectID, &b.CampaignID, &b.Seed, &b.Strategy, &manifestJSON, &b.BuildLog,
		&b.BinaryPath, &b.BinaryHash, &b.Status, &b.Port, &b.BuildToken, &b.CreatedAt)
	if err == sql.ErrNoRows {
		return LandingPageBuild{}, fmt.Errorf("landing_pages: build not found for token")
	}
	if err != nil {
		return LandingPageBuild{}, fmt.Errorf("landing_pages: get build by token: %w", err)
	}
	b.BuildManifestJSON = make(map[string]any)
	_ = json.Unmarshal(manifestJSON, &b.BuildManifestJSON)
	return b, nil
}

// ---------- Health Checks ----------

// CreateHealthCheck inserts a health check record.
func (r *LandingPageRepository) CreateHealthCheck(ctx context.Context, hc LandingPageHealthCheck) (LandingPageHealthCheck, error) {
	hc.ID = uuid.New().String()
	err := r.db.QueryRowContext(ctx,
		`INSERT INTO landing_page_health_checks (id, build_id, status, response_time_ms)
		 VALUES ($1,$2,$3,$4) RETURNING id, build_id, status, response_time_ms, checked_at`,
		hc.ID, hc.BuildID, hc.Status, hc.ResponseTimeMs,
	).Scan(&hc.ID, &hc.BuildID, &hc.Status, &hc.ResponseTimeMs, &hc.CheckedAt)
	if err != nil {
		return LandingPageHealthCheck{}, fmt.Errorf("landing_pages: create health check: %w", err)
	}
	return hc, nil
}

// ListHealthChecks returns health checks for a build.
func (r *LandingPageRepository) ListHealthChecks(ctx context.Context, buildID string) ([]LandingPageHealthCheck, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT id, build_id, status, response_time_ms, checked_at
		 FROM landing_page_health_checks WHERE build_id = $1 ORDER BY checked_at DESC LIMIT 50`, buildID)
	if err != nil {
		return nil, fmt.Errorf("landing_pages: list health checks: %w", err)
	}
	defer rows.Close()

	var result []LandingPageHealthCheck
	for rows.Next() {
		var hc LandingPageHealthCheck
		if err := rows.Scan(&hc.ID, &hc.BuildID, &hc.Status, &hc.ResponseTimeMs, &hc.CheckedAt); err != nil {
			return nil, fmt.Errorf("landing_pages: scan health check: %w", err)
		}
		result = append(result, hc)
	}
	if result == nil {
		result = []LandingPageHealthCheck{}
	}
	return result, nil
}
