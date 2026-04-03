package repositories

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/lib/pq"
)

// EmailTemplate is the DB model for an email_templates row.
type EmailTemplate struct {
	ID          string
	Name        string
	Description *string
	Subject     string
	HTMLBody    string
	TextBody    string
	Category    string
	Tags        []string
	IsShared    bool
	Variables   []byte // JSONB
	CreatedBy   string
	DeletedAt   *time.Time
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

// EmailTemplateVersion is the DB model for an email_template_versions row.
type EmailTemplateVersion struct {
	ID            string
	TemplateID    string
	VersionNumber int
	Subject       string
	HTMLBody      string
	TextBody      string
	ChangeNote    *string
	CreatedBy     string
	CreatedAt     time.Time
}

// EmailTemplateFilters controls optional filtering for List.
type EmailTemplateFilters struct {
	Category   string // empty = all
	NameSearch string // empty = all
	Tag        string // empty = all
	IsShared   *bool  // nil = all
	CreatedBy  string // empty = all
}

// EmailTemplateUpdate holds mutable fields for an update operation.
type EmailTemplateUpdate struct {
	Name        *string
	Description *string
	Subject     *string
	HTMLBody    *string
	TextBody    *string
	Category    *string
	Tags        []string // nil = no change
	IsShared    *bool
}

// EmailTemplateRepository provides database operations for email_templates.
type EmailTemplateRepository struct {
	db *sql.DB
}

// NewEmailTemplateRepository creates a new EmailTemplateRepository.
func NewEmailTemplateRepository(db *sql.DB) *EmailTemplateRepository {
	return &EmailTemplateRepository{db: db}
}

// Create inserts a new email template and returns the created row.
func (r *EmailTemplateRepository) Create(ctx context.Context, t EmailTemplate) (EmailTemplate, error) {
	id := uuid.New().String()
	tags := t.Tags
	if tags == nil {
		tags = []string{}
	}
	const q = `
		INSERT INTO email_templates
			(id, name, description, subject, html_body, text_body,
			 category, tags, is_shared, variables, created_by)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11)
		RETURNING id, name, description, subject, html_body, text_body,
		          category, tags, is_shared, variables, created_by, deleted_at, created_at, updated_at`
	return r.scanOne(r.db.QueryRowContext(ctx, q,
		id, t.Name, t.Description, t.Subject, t.HTMLBody, t.TextBody,
		t.Category, pq.Array(tags), t.IsShared, nullBytes(t.Variables), t.CreatedBy,
	))
}

// GetByID retrieves an email template by UUID. Returns sql.ErrNoRows if not found.
func (r *EmailTemplateRepository) GetByID(ctx context.Context, id string) (EmailTemplate, error) {
	const q = `
		SELECT id, name, description, subject, html_body, text_body,
		       category, tags, is_shared, variables, created_by, deleted_at, created_at, updated_at
		FROM email_templates WHERE id = $1 AND deleted_at IS NULL`
	return r.scanOne(r.db.QueryRowContext(ctx, q, id))
}

// List returns email templates with optional filtering.
func (r *EmailTemplateRepository) List(ctx context.Context, filters EmailTemplateFilters) ([]EmailTemplate, error) {
	args := []any{}
	argIdx := 1
	where := "WHERE 1=1"

	if filters.Category != "" {
		where += fmt.Sprintf(" AND category = $%d", argIdx)
		args = append(args, filters.Category)
		argIdx++
	}
	if filters.NameSearch != "" {
		where += fmt.Sprintf(" AND name ILIKE $%d", argIdx)
		args = append(args, "%"+filters.NameSearch+"%")
		argIdx++
	}
	if filters.Tag != "" {
		where += fmt.Sprintf(" AND $%d = ANY(tags)", argIdx)
		args = append(args, filters.Tag)
		argIdx++
	}
	if filters.IsShared != nil {
		where += fmt.Sprintf(" AND is_shared = $%d", argIdx)
		args = append(args, *filters.IsShared)
		argIdx++
	}
	if filters.CreatedBy != "" {
		where += fmt.Sprintf(" AND created_by = $%d", argIdx)
		args = append(args, filters.CreatedBy)
		argIdx++
	}
	_ = argIdx

	if !strings.Contains(where, "deleted_at") {
		where += " AND deleted_at IS NULL"
	}

	q := fmt.Sprintf(`
		SELECT id, name, description, subject, html_body, text_body,
		       category, tags, is_shared, variables, created_by, deleted_at, created_at, updated_at
		FROM email_templates %s ORDER BY name ASC`, where)

	rows, err := r.db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, fmt.Errorf("email templates: list: %w", err)
	}
	defer rows.Close()

	var results []EmailTemplate
	for rows.Next() {
		t, err := r.scanRow(rows)
		if err != nil {
			return nil, fmt.Errorf("email templates: list scan: %w", err)
		}
		results = append(results, t)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("email templates: list rows: %w", err)
	}
	return results, nil
}

// Update applies changes to the email template identified by id and snapshots a new version.
func (r *EmailTemplateRepository) Update(ctx context.Context, id string, upd EmailTemplateUpdate, actorID, changeNote string) (EmailTemplate, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return EmailTemplate{}, fmt.Errorf("email templates: update begin tx: %w", err)
	}
	defer tx.Rollback() //nolint:errcheck

	// Fetch current for versioning.
	var current EmailTemplate
	var tags pq.StringArray
	err = tx.QueryRowContext(ctx, `
		SELECT id, name, description, subject, html_body, text_body,
		       category, tags, is_shared, variables, created_by, deleted_at, created_at, updated_at
		FROM email_templates WHERE id = $1 AND deleted_at IS NULL`, id).Scan(
		&current.ID, &current.Name, &current.Description,
		&current.Subject, &current.HTMLBody, &current.TextBody,
		&current.Category, &tags, &current.IsShared, &current.Variables, &current.CreatedBy,
		&current.DeletedAt, &current.CreatedAt, &current.UpdatedAt,
	)
	if err != nil {
		return EmailTemplate{}, fmt.Errorf("email templates: update fetch: %w", err)
	}
	current.Tags = []string(tags)

	// Determine next version number.
	var maxVer int
	_ = tx.QueryRowContext(ctx,
		`SELECT COALESCE(MAX(version_number), 0) FROM email_template_versions WHERE template_id = $1`, id,
	).Scan(&maxVer)
	nextVer := maxVer + 1

	// Snapshot current state as a version.
	var notePtr *string
	if changeNote != "" {
		notePtr = &changeNote
	}
	_, err = tx.ExecContext(ctx, `
		INSERT INTO email_template_versions
			(id, template_id, version_number, subject, html_body, text_body, change_note, created_by)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8)`,
		uuid.New().String(), id, nextVer,
		current.Subject, current.HTMLBody, current.TextBody, notePtr, actorID,
	)
	if err != nil {
		return EmailTemplate{}, fmt.Errorf("email templates: snapshot version: %w", err)
	}

	// Build UPDATE query.
	setClauses := []string{}
	args := []any{}
	argIdx := 1

	if upd.Name != nil {
		setClauses = append(setClauses, fmt.Sprintf("name = $%d", argIdx))
		args = append(args, *upd.Name)
		argIdx++
	}
	if upd.Description != nil {
		setClauses = append(setClauses, fmt.Sprintf("description = $%d", argIdx))
		args = append(args, *upd.Description)
		argIdx++
	}
	if upd.Subject != nil {
		setClauses = append(setClauses, fmt.Sprintf("subject = $%d", argIdx))
		args = append(args, *upd.Subject)
		argIdx++
	}
	if upd.HTMLBody != nil {
		setClauses = append(setClauses, fmt.Sprintf("html_body = $%d", argIdx))
		args = append(args, *upd.HTMLBody)
		argIdx++
	}
	if upd.TextBody != nil {
		setClauses = append(setClauses, fmt.Sprintf("text_body = $%d", argIdx))
		args = append(args, *upd.TextBody)
		argIdx++
	}
	if upd.Category != nil {
		setClauses = append(setClauses, fmt.Sprintf("category = $%d", argIdx))
		args = append(args, *upd.Category)
		argIdx++
	}
	if upd.Tags != nil {
		setClauses = append(setClauses, fmt.Sprintf("tags = $%d", argIdx))
		args = append(args, pq.Array(upd.Tags))
		argIdx++
	}
	if upd.IsShared != nil {
		setClauses = append(setClauses, fmt.Sprintf("is_shared = $%d", argIdx))
		args = append(args, *upd.IsShared)
		argIdx++
	}

	if len(setClauses) == 0 {
		if err := tx.Commit(); err != nil {
			return EmailTemplate{}, fmt.Errorf("email templates: update commit: %w", err)
		}
		return current, nil
	}

	args = append(args, id)
	q := fmt.Sprintf(`
		UPDATE email_templates SET %s
		WHERE id = $%d AND deleted_at IS NULL
		RETURNING id, name, description, subject, html_body, text_body,
		          category, tags, is_shared, variables, created_by, deleted_at, created_at, updated_at`,
		strings.Join(setClauses, ", "), argIdx)

	updated, err := r.scanOneTx(tx.QueryRowContext(ctx, q, args...))
	if err != nil {
		return EmailTemplate{}, fmt.Errorf("email templates: update: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return EmailTemplate{}, fmt.Errorf("email templates: update commit: %w", err)
	}
	return updated, nil
}

// Delete soft-deletes the email template by setting deleted_at.
func (r *EmailTemplateRepository) Delete(ctx context.Context, id string) error {
	result, err := r.db.ExecContext(ctx,
		"UPDATE email_templates SET deleted_at = now() WHERE id = $1 AND deleted_at IS NULL", id)
	if err != nil {
		return fmt.Errorf("email templates: delete: %w", err)
	}
	n, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("email templates: delete rows affected: %w", err)
	}
	if n == 0 {
		return fmt.Errorf("email templates: delete: %w", sql.ErrNoRows)
	}
	return nil
}

// Clone creates a copy of the template with a new name.
func (r *EmailTemplateRepository) Clone(ctx context.Context, sourceID, newName, actorID string) (EmailTemplate, error) {
	src, err := r.GetByID(ctx, sourceID)
	if err != nil {
		return EmailTemplate{}, err
	}
	clone := EmailTemplate{
		Name:      newName,
		Subject:   src.Subject,
		HTMLBody:  src.HTMLBody,
		TextBody:  src.TextBody,
		Category:  src.Category,
		Tags:      append([]string{}, src.Tags...),
		IsShared:  false,
		CreatedBy: actorID,
	}
	if src.Description != nil {
		desc := *src.Description
		clone.Description = &desc
	}
	return r.Create(ctx, clone)
}

// ListVersions returns all version snapshots for a template, most recent first.
func (r *EmailTemplateRepository) ListVersions(ctx context.Context, templateID string) ([]EmailTemplateVersion, error) {
	const q = `
		SELECT id, template_id, version_number, subject, html_body, text_body,
		       change_note, created_by, created_at
		FROM email_template_versions
		WHERE template_id = $1
		ORDER BY version_number DESC`
	rows, err := r.db.QueryContext(ctx, q, templateID)
	if err != nil {
		return nil, fmt.Errorf("email templates: list versions: %w", err)
	}
	defer rows.Close()

	var results []EmailTemplateVersion
	for rows.Next() {
		var v EmailTemplateVersion
		if err := rows.Scan(
			&v.ID, &v.TemplateID, &v.VersionNumber,
			&v.Subject, &v.HTMLBody, &v.TextBody,
			&v.ChangeNote, &v.CreatedBy, &v.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("email templates: list versions scan: %w", err)
		}
		results = append(results, v)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("email templates: list versions rows: %w", err)
	}
	return results, nil
}

// GetVersion returns a specific version snapshot.
func (r *EmailTemplateRepository) GetVersion(ctx context.Context, templateID string, versionNumber int) (EmailTemplateVersion, error) {
	const q = `
		SELECT id, template_id, version_number, subject, html_body, text_body,
		       change_note, created_by, created_at
		FROM email_template_versions
		WHERE template_id = $1 AND version_number = $2`
	var v EmailTemplateVersion
	err := r.db.QueryRowContext(ctx, q, templateID, versionNumber).Scan(
		&v.ID, &v.TemplateID, &v.VersionNumber,
		&v.Subject, &v.HTMLBody, &v.TextBody,
		&v.ChangeNote, &v.CreatedBy, &v.CreatedAt,
	)
	if err != nil {
		return EmailTemplateVersion{}, fmt.Errorf("email templates: get version: %w", err)
	}
	return v, nil
}

// --- scan helpers ---

func (r *EmailTemplateRepository) scanOne(row *sql.Row) (EmailTemplate, error) {
	var t EmailTemplate
	var tags pq.StringArray
	err := row.Scan(
		&t.ID, &t.Name, &t.Description,
		&t.Subject, &t.HTMLBody, &t.TextBody,
		&t.Category, &tags, &t.IsShared, &t.Variables, &t.CreatedBy,
		&t.DeletedAt, &t.CreatedAt, &t.UpdatedAt,
	)
	if err != nil {
		return EmailTemplate{}, fmt.Errorf("email templates: scan: %w", err)
	}
	t.Tags = []string(tags)
	return t, nil
}

func (r *EmailTemplateRepository) scanOneTx(row *sql.Row) (EmailTemplate, error) {
	var t EmailTemplate
	var tags pq.StringArray
	err := row.Scan(
		&t.ID, &t.Name, &t.Description,
		&t.Subject, &t.HTMLBody, &t.TextBody,
		&t.Category, &tags, &t.IsShared, &t.Variables, &t.CreatedBy,
		&t.DeletedAt, &t.CreatedAt, &t.UpdatedAt,
	)
	if err != nil {
		return EmailTemplate{}, fmt.Errorf("email templates: scan: %w", err)
	}
	t.Tags = []string(tags)
	return t, nil
}

func (r *EmailTemplateRepository) scanRow(rows *sql.Rows) (EmailTemplate, error) {
	var t EmailTemplate
	var tags pq.StringArray
	err := rows.Scan(
		&t.ID, &t.Name, &t.Description,
		&t.Subject, &t.HTMLBody, &t.TextBody,
		&t.Category, &tags, &t.IsShared, &t.Variables, &t.CreatedBy,
		&t.DeletedAt, &t.CreatedAt, &t.UpdatedAt,
	)
	if err != nil {
		return EmailTemplate{}, err
	}
	t.Tags = []string(tags)
	return t, nil
}
