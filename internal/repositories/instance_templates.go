package repositories

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/lib/pq"
)

// InstanceTemplate is the DB model for an instance_templates header row.
type InstanceTemplate struct {
	ID                string
	DisplayName       string
	CloudCredentialID string
	ProviderType      CloudProviderType
	CurrentVersion    int
	CreatedBy         string
	CreatedAt         time.Time
	UpdatedAt         time.Time
}

// InstanceTemplateVersion is the DB model for an instance_template_versions row.
type InstanceTemplateVersion struct {
	ID               string
	TemplateID       string
	VersionNumber    int
	Region           string
	InstanceSize     string
	OSImage          string
	SecurityGroups   []string
	SSHKeyReference  *string
	UserData         *string
	Tags             map[string]string
	Notes            *string
	CreatedBy        string
	CreatedAt        time.Time
}

// InstanceTemplateWithVersion combines the header and the current version for API responses.
type InstanceTemplateWithVersion struct {
	InstanceTemplate
	Version InstanceTemplateVersion
}

// InstanceTemplateRepository provides database operations for instance templates.
type InstanceTemplateRepository struct {
	db *sql.DB
}

// NewInstanceTemplateRepository creates a new InstanceTemplateRepository.
func NewInstanceTemplateRepository(db *sql.DB) *InstanceTemplateRepository {
	return &InstanceTemplateRepository{db: db}
}

// Create inserts a new template header and version 1 in a transaction.
func (r *InstanceTemplateRepository) Create(ctx context.Context, tmpl InstanceTemplate, v InstanceTemplateVersion) (InstanceTemplateWithVersion, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return InstanceTemplateWithVersion{}, fmt.Errorf("instance template: begin tx: %w", err)
	}
	defer tx.Rollback() //nolint:errcheck

	tmplID := uuid.New().String()
	vID := uuid.New().String()

	const qHeader = `
		INSERT INTO instance_templates (id, display_name, cloud_credential_id, provider_type, current_version, created_by)
		VALUES ($1,$2,$3,$4,1,$5)
		RETURNING id, display_name, cloud_credential_id, provider_type, current_version, created_by, created_at, updated_at`

	var out InstanceTemplateWithVersion
	err = tx.QueryRowContext(ctx, qHeader,
		tmplID, tmpl.DisplayName, tmpl.CloudCredentialID, string(tmpl.ProviderType), tmpl.CreatedBy,
	).Scan(
		&out.ID, &out.DisplayName, &out.CloudCredentialID, &out.ProviderType,
		&out.CurrentVersion, &out.CreatedBy, &out.CreatedAt, &out.UpdatedAt,
	)
	if err != nil {
		return InstanceTemplateWithVersion{}, fmt.Errorf("instance template: create header: %w", err)
	}

	tagsJSON, err := json.Marshal(v.Tags)
	if err != nil {
		return InstanceTemplateWithVersion{}, fmt.Errorf("instance template: marshal tags: %w", err)
	}

	const qVersion = `
		INSERT INTO instance_template_versions
			(id, template_id, version_number, region, instance_size, os_image, security_groups,
			 ssh_key_reference, user_data, tags, notes, created_by)
		VALUES ($1,$2,1,$3,$4,$5,$6,$7,$8,$9,$10,$11)
		RETURNING id, template_id, version_number, region, instance_size, os_image, security_groups,
		          ssh_key_reference, user_data, tags, notes, created_by, created_at`

	ver, err := scanVersionRow(tx.QueryRowContext(ctx, qVersion,
		vID, tmplID, v.Region, v.InstanceSize, v.OSImage,
		pq.Array(v.SecurityGroups), v.SSHKeyReference, v.UserData,
		tagsJSON, v.Notes, v.CreatedBy,
	))
	if err != nil {
		return InstanceTemplateWithVersion{}, fmt.Errorf("instance template: create version: %w", err)
	}
	out.Version = ver

	if err := tx.Commit(); err != nil {
		return InstanceTemplateWithVersion{}, fmt.Errorf("instance template: commit: %w", err)
	}
	return out, nil
}

// GetByID retrieves the template header and its current version. Returns sql.ErrNoRows if not found.
func (r *InstanceTemplateRepository) GetByID(ctx context.Context, id string) (InstanceTemplateWithVersion, error) {
	const q = `
		SELECT t.id, t.display_name, t.cloud_credential_id, t.provider_type, t.current_version,
		       t.created_by, t.created_at, t.updated_at,
		       v.id, v.template_id, v.version_number, v.region, v.instance_size, v.os_image,
		       v.security_groups, v.ssh_key_reference, v.user_data, v.tags, v.notes,
		       v.created_by, v.created_at
		FROM instance_templates t
		JOIN instance_template_versions v ON v.template_id = t.id AND v.version_number = t.current_version
		WHERE t.id = $1`

	return r.scanWithVersion(r.db.QueryRowContext(ctx, q, id))
}

// GetVersion retrieves a specific version of a template.
func (r *InstanceTemplateRepository) GetVersion(ctx context.Context, templateID string, version int) (InstanceTemplateVersion, error) {
	const q = `
		SELECT id, template_id, version_number, region, instance_size, os_image, security_groups,
		       ssh_key_reference, user_data, tags, notes, created_by, created_at
		FROM instance_template_versions
		WHERE template_id = $1 AND version_number = $2`

	return scanVersionRow(r.db.QueryRowContext(ctx, q, templateID, version))
}

// List returns all templates with their current versions.
func (r *InstanceTemplateRepository) List(ctx context.Context, providerType string) ([]InstanceTemplateWithVersion, error) {
	where := "WHERE 1=1"
	args := []any{}
	if providerType != "" {
		where += " AND t.provider_type = $1"
		args = append(args, providerType)
	}

	q := fmt.Sprintf(`
		SELECT t.id, t.display_name, t.cloud_credential_id, t.provider_type, t.current_version,
		       t.created_by, t.created_at, t.updated_at,
		       v.id, v.template_id, v.version_number, v.region, v.instance_size, v.os_image,
		       v.security_groups, v.ssh_key_reference, v.user_data, v.tags, v.notes,
		       v.created_by, v.created_at
		FROM instance_templates t
		JOIN instance_template_versions v ON v.template_id = t.id AND v.version_number = t.current_version
		%s
		ORDER BY t.display_name ASC`, where)

	rows, err := r.db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, fmt.Errorf("instance templates: list: %w", err)
	}
	defer rows.Close()

	var results []InstanceTemplateWithVersion
	for rows.Next() {
		var out InstanceTemplateWithVersion
		var tagsJSON []byte
		var sgArr pq.StringArray
		err := rows.Scan(
			&out.ID, &out.DisplayName, &out.CloudCredentialID, &out.ProviderType,
			&out.CurrentVersion, &out.CreatedBy, &out.CreatedAt, &out.UpdatedAt,
			&out.Version.ID, &out.Version.TemplateID, &out.Version.VersionNumber,
			&out.Version.Region, &out.Version.InstanceSize, &out.Version.OSImage,
			&sgArr, &out.Version.SSHKeyReference, &out.Version.UserData,
			&tagsJSON, &out.Version.Notes, &out.Version.CreatedBy, &out.Version.CreatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("instance templates: list scan: %w", err)
		}
		out.Version.SecurityGroups = []string(sgArr)
		if err := json.Unmarshal(tagsJSON, &out.Version.Tags); err != nil {
			out.Version.Tags = map[string]string{}
		}
		results = append(results, out)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("instance templates: list rows: %w", err)
	}
	return results, nil
}

// ListVersions returns all versions of a template ordered by version_number descending.
func (r *InstanceTemplateRepository) ListVersions(ctx context.Context, templateID string) ([]InstanceTemplateVersion, error) {
	const q = `
		SELECT id, template_id, version_number, region, instance_size, os_image, security_groups,
		       ssh_key_reference, user_data, tags, notes, created_by, created_at
		FROM instance_template_versions
		WHERE template_id = $1
		ORDER BY version_number DESC`

	rows, err := r.db.QueryContext(ctx, q, templateID)
	if err != nil {
		return nil, fmt.Errorf("instance template versions: list: %w", err)
	}
	defer rows.Close()

	var results []InstanceTemplateVersion
	for rows.Next() {
		var v InstanceTemplateVersion
		var tagsJSON []byte
		var sgArr pq.StringArray
		err := rows.Scan(
			&v.ID, &v.TemplateID, &v.VersionNumber, &v.Region, &v.InstanceSize, &v.OSImage,
			&sgArr, &v.SSHKeyReference, &v.UserData,
			&tagsJSON, &v.Notes, &v.CreatedBy, &v.CreatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("instance template versions: list scan: %w", err)
		}
		v.SecurityGroups = []string(sgArr)
		if err := json.Unmarshal(tagsJSON, &v.Tags); err != nil {
			v.Tags = map[string]string{}
		}
		results = append(results, v)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("instance template versions: list rows: %w", err)
	}
	return results, nil
}

// AddVersion inserts a new version and updates current_version on the header — all in one transaction.
func (r *InstanceTemplateRepository) AddVersion(ctx context.Context, templateID string, v InstanceTemplateVersion) (InstanceTemplateWithVersion, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return InstanceTemplateWithVersion{}, fmt.Errorf("instance template: begin tx: %w", err)
	}
	defer tx.Rollback() //nolint:errcheck

	// Get next version number.
	var nextVer int
	if err := tx.QueryRowContext(ctx,
		"SELECT COALESCE(MAX(version_number),0)+1 FROM instance_template_versions WHERE template_id = $1",
		templateID).Scan(&nextVer); err != nil {
		return InstanceTemplateWithVersion{}, fmt.Errorf("instance template: get next version: %w", err)
	}

	vID := uuid.New().String()
	tagsJSON, err := json.Marshal(v.Tags)
	if err != nil {
		return InstanceTemplateWithVersion{}, fmt.Errorf("instance template: marshal tags: %w", err)
	}

	const qVersion = `
		INSERT INTO instance_template_versions
			(id, template_id, version_number, region, instance_size, os_image, security_groups,
			 ssh_key_reference, user_data, tags, notes, created_by)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12)
		RETURNING id, template_id, version_number, region, instance_size, os_image, security_groups,
		          ssh_key_reference, user_data, tags, notes, created_by, created_at`

	ver, err := scanVersionRow(tx.QueryRowContext(ctx, qVersion,
		vID, templateID, nextVer, v.Region, v.InstanceSize, v.OSImage,
		pq.Array(v.SecurityGroups), v.SSHKeyReference, v.UserData,
		tagsJSON, v.Notes, v.CreatedBy,
	))
	if err != nil {
		return InstanceTemplateWithVersion{}, fmt.Errorf("instance template: insert version: %w", err)
	}

	// Update header current_version.
	const qHeader = `
		UPDATE instance_templates SET current_version = $1
		WHERE id = $2
		RETURNING id, display_name, cloud_credential_id, provider_type, current_version,
		          created_by, created_at, updated_at`

	var out InstanceTemplateWithVersion
	err = tx.QueryRowContext(ctx, qHeader, nextVer, templateID).Scan(
		&out.ID, &out.DisplayName, &out.CloudCredentialID, &out.ProviderType,
		&out.CurrentVersion, &out.CreatedBy, &out.CreatedAt, &out.UpdatedAt,
	)
	if err != nil {
		return InstanceTemplateWithVersion{}, fmt.Errorf("instance template: update header version: %w", err)
	}
	out.Version = ver

	if err := tx.Commit(); err != nil {
		return InstanceTemplateWithVersion{}, fmt.Errorf("instance template: commit: %w", err)
	}
	return out, nil
}

// Delete hard-deletes a template and cascades to all its versions. Caller checks references.
func (r *InstanceTemplateRepository) Delete(ctx context.Context, id string) error {
	result, err := r.db.ExecContext(ctx, "DELETE FROM instance_templates WHERE id = $1", id)
	if err != nil {
		return fmt.Errorf("instance template: delete: %w", err)
	}
	n, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("instance template: delete rows affected: %w", err)
	}
	if n == 0 {
		return fmt.Errorf("instance template: delete: %w", sql.ErrNoRows)
	}
	return nil
}

// rowScanner is implemented by both *sql.Row and *sql.Rows for unified scan helpers.
type rowScanner interface {
	Scan(dest ...any) error
}

func scanVersionRow(row rowScanner) (InstanceTemplateVersion, error) {
	var v InstanceTemplateVersion
	var tagsJSON []byte
	var sgArr pq.StringArray
	err := row.Scan(
		&v.ID, &v.TemplateID, &v.VersionNumber, &v.Region, &v.InstanceSize, &v.OSImage,
		&sgArr, &v.SSHKeyReference, &v.UserData,
		&tagsJSON, &v.Notes, &v.CreatedBy, &v.CreatedAt,
	)
	if err != nil {
		return InstanceTemplateVersion{}, fmt.Errorf("instance template version: scan: %w", err)
	}
	v.SecurityGroups = []string(sgArr)
	if err := json.Unmarshal(tagsJSON, &v.Tags); err != nil {
		v.Tags = map[string]string{}
	}
	return v, nil
}

func (r *InstanceTemplateRepository) scanWithVersion(row *sql.Row) (InstanceTemplateWithVersion, error) {
	var out InstanceTemplateWithVersion
	var tagsJSON []byte
	var sgArr pq.StringArray
	err := row.Scan(
		&out.ID, &out.DisplayName, &out.CloudCredentialID, &out.ProviderType,
		&out.CurrentVersion, &out.CreatedBy, &out.CreatedAt, &out.UpdatedAt,
		&out.Version.ID, &out.Version.TemplateID, &out.Version.VersionNumber,
		&out.Version.Region, &out.Version.InstanceSize, &out.Version.OSImage,
		&sgArr, &out.Version.SSHKeyReference, &out.Version.UserData,
		&tagsJSON, &out.Version.Notes, &out.Version.CreatedBy, &out.Version.CreatedAt,
	)
	if err != nil {
		return InstanceTemplateWithVersion{}, fmt.Errorf("instance template: scan: %w", err)
	}
	out.Version.SecurityGroups = []string(sgArr)
	if err := json.Unmarshal(tagsJSON, &out.Version.Tags); err != nil {
		out.Version.Tags = map[string]string{}
	}
	return out, nil
}
