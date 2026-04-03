package repositories

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"

	"github.com/google/uuid"
)

// TargetImportRepository provides database operations for target_imports.
type TargetImportRepository struct {
	db *sql.DB
}

// NewTargetImportRepository creates a new TargetImportRepository.
func NewTargetImportRepository(db *sql.DB) *TargetImportRepository {
	return &TargetImportRepository{db: db}
}

// Create inserts a new target import record.
func (r *TargetImportRepository) Create(ctx context.Context, ti TargetImport) (TargetImport, error) {
	id := uuid.New().String()

	headersJSON, err := json.Marshal(ti.Headers)
	if err != nil {
		return TargetImport{}, fmt.Errorf("target imports: create: marshal headers: %w", err)
	}
	previewJSON, err := json.Marshal(ti.PreviewRows)
	if err != nil {
		return TargetImport{}, fmt.Errorf("target imports: create: marshal preview: %w", err)
	}

	const q = `
		INSERT INTO target_imports
			(id, filename, row_count, column_count, headers, preview_rows, raw_data, status, uploaded_by)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9)
		RETURNING id, filename, row_count, column_count, headers, preview_rows,
		          mapping, status, validation_result, imported_count, rejected_count,
		          uploaded_by, created_at, updated_at`

	return r.scanOne(r.db.QueryRowContext(ctx, q,
		id, ti.Filename, ti.RowCount, ti.ColumnCount, headersJSON, previewJSON,
		ti.RawData, "uploaded", ti.UploadedBy,
	))
}

// GetByID retrieves a target import by UUID.
func (r *TargetImportRepository) GetByID(ctx context.Context, id string) (TargetImport, error) {
	const q = `
		SELECT id, filename, row_count, column_count, headers, preview_rows,
		       mapping, status, validation_result, imported_count, rejected_count,
		       uploaded_by, created_at, updated_at
		FROM target_imports WHERE id = $1`
	return r.scanOne(r.db.QueryRowContext(ctx, q, id))
}

// GetRawData retrieves just the raw CSV data for processing.
func (r *TargetImportRepository) GetRawData(ctx context.Context, id string) ([]byte, error) {
	var data []byte
	err := r.db.QueryRowContext(ctx,
		"SELECT raw_data FROM target_imports WHERE id = $1", id).Scan(&data)
	if err != nil {
		return nil, fmt.Errorf("target imports: get raw data: %w", err)
	}
	return data, nil
}

// UpdateMapping saves the column mapping for an import.
func (r *TargetImportRepository) UpdateMapping(ctx context.Context, id string, mapping map[string]string) (TargetImport, error) {
	mappingJSON, err := json.Marshal(mapping)
	if err != nil {
		return TargetImport{}, fmt.Errorf("target imports: update mapping: marshal: %w", err)
	}

	const q = `
		UPDATE target_imports SET mapping = $1, status = 'mapped'
		WHERE id = $2
		RETURNING id, filename, row_count, column_count, headers, preview_rows,
		          mapping, status, validation_result, imported_count, rejected_count,
		          uploaded_by, created_at, updated_at`
	return r.scanOne(r.db.QueryRowContext(ctx, q, mappingJSON, id))
}

// UpdateValidation saves the validation result.
func (r *TargetImportRepository) UpdateValidation(ctx context.Context, id string, result map[string]any) (TargetImport, error) {
	resultJSON, err := json.Marshal(result)
	if err != nil {
		return TargetImport{}, fmt.Errorf("target imports: update validation: marshal: %w", err)
	}

	const q = `
		UPDATE target_imports SET validation_result = $1, status = 'validated'
		WHERE id = $2
		RETURNING id, filename, row_count, column_count, headers, preview_rows,
		          mapping, status, validation_result, imported_count, rejected_count,
		          uploaded_by, created_at, updated_at`
	return r.scanOne(r.db.QueryRowContext(ctx, q, resultJSON, id))
}

// UpdateCommitResult saves the commit outcome.
func (r *TargetImportRepository) UpdateCommitResult(ctx context.Context, id string, imported, rejected int, status string) (TargetImport, error) {
	const q = `
		UPDATE target_imports SET imported_count = $1, rejected_count = $2, status = $3
		WHERE id = $4
		RETURNING id, filename, row_count, column_count, headers, preview_rows,
		          mapping, status, validation_result, imported_count, rejected_count,
		          uploaded_by, created_at, updated_at`
	return r.scanOne(r.db.QueryRowContext(ctx, q, imported, rejected, status, id))
}

// --- Import Mapping Templates ---

// MappingTemplateRepository provides database operations for import_mapping_templates.
type MappingTemplateRepository struct {
	db *sql.DB
}

// NewMappingTemplateRepository creates a new MappingTemplateRepository.
func NewMappingTemplateRepository(db *sql.DB) *MappingTemplateRepository {
	return &MappingTemplateRepository{db: db}
}

// Create inserts a new mapping template.
func (r *MappingTemplateRepository) Create(ctx context.Context, name string, mapping map[string]string, createdBy string) (ImportMappingTemplate, error) {
	id := uuid.New().String()
	mappingJSON, err := json.Marshal(mapping)
	if err != nil {
		return ImportMappingTemplate{}, fmt.Errorf("mapping templates: create: marshal: %w", err)
	}

	var t ImportMappingTemplate
	var mJSON []byte
	err = r.db.QueryRowContext(ctx, `
		INSERT INTO import_mapping_templates (id, name, mapping, created_by)
		VALUES ($1,$2,$3,$4)
		RETURNING id, name, mapping, created_by, created_at, updated_at`,
		id, name, mappingJSON, createdBy,
	).Scan(&t.ID, &t.Name, &mJSON, &t.CreatedBy, &t.CreatedAt, &t.UpdatedAt)
	if err != nil {
		return ImportMappingTemplate{}, fmt.Errorf("mapping templates: create: %w", err)
	}
	t.Mapping = map[string]string{}
	if len(mJSON) > 0 {
		_ = json.Unmarshal(mJSON, &t.Mapping)
	}
	return t, nil
}

// List returns all mapping templates.
func (r *MappingTemplateRepository) List(ctx context.Context) ([]ImportMappingTemplate, error) {
	rows, err := r.db.QueryContext(ctx,
		"SELECT id, name, mapping, created_by, created_at, updated_at FROM import_mapping_templates ORDER BY name")
	if err != nil {
		return nil, fmt.Errorf("mapping templates: list: %w", err)
	}
	defer rows.Close()

	var results []ImportMappingTemplate
	for rows.Next() {
		var t ImportMappingTemplate
		var mJSON []byte
		if err := rows.Scan(&t.ID, &t.Name, &mJSON, &t.CreatedBy, &t.CreatedAt, &t.UpdatedAt); err != nil {
			return nil, fmt.Errorf("mapping templates: list scan: %w", err)
		}
		t.Mapping = map[string]string{}
		if len(mJSON) > 0 {
			_ = json.Unmarshal(mJSON, &t.Mapping)
		}
		results = append(results, t)
	}
	return results, rows.Err()
}

// GetByID retrieves a mapping template by UUID.
func (r *MappingTemplateRepository) GetByID(ctx context.Context, id string) (ImportMappingTemplate, error) {
	var t ImportMappingTemplate
	var mJSON []byte
	err := r.db.QueryRowContext(ctx,
		"SELECT id, name, mapping, created_by, created_at, updated_at FROM import_mapping_templates WHERE id = $1", id,
	).Scan(&t.ID, &t.Name, &mJSON, &t.CreatedBy, &t.CreatedAt, &t.UpdatedAt)
	if err != nil {
		return ImportMappingTemplate{}, err
	}
	t.Mapping = map[string]string{}
	if len(mJSON) > 0 {
		_ = json.Unmarshal(mJSON, &t.Mapping)
	}
	return t, nil
}

// Update modifies an existing mapping template.
func (r *MappingTemplateRepository) Update(ctx context.Context, id, name string, mapping map[string]string) (ImportMappingTemplate, error) {
	mappingJSON, err := json.Marshal(mapping)
	if err != nil {
		return ImportMappingTemplate{}, fmt.Errorf("mapping templates: update: marshal: %w", err)
	}

	var t ImportMappingTemplate
	var mJSON []byte
	err = r.db.QueryRowContext(ctx, `
		UPDATE import_mapping_templates SET name = $1, mapping = $2
		WHERE id = $3
		RETURNING id, name, mapping, created_by, created_at, updated_at`,
		name, mappingJSON, id,
	).Scan(&t.ID, &t.Name, &mJSON, &t.CreatedBy, &t.CreatedAt, &t.UpdatedAt)
	if err != nil {
		return ImportMappingTemplate{}, fmt.Errorf("mapping templates: update: %w", err)
	}
	t.Mapping = map[string]string{}
	if len(mJSON) > 0 {
		_ = json.Unmarshal(mJSON, &t.Mapping)
	}
	return t, nil
}

// Delete removes a mapping template.
func (r *MappingTemplateRepository) Delete(ctx context.Context, id string) error {
	result, err := r.db.ExecContext(ctx, "DELETE FROM import_mapping_templates WHERE id = $1", id)
	if err != nil {
		return fmt.Errorf("mapping templates: delete: %w", err)
	}
	n, _ := result.RowsAffected()
	if n == 0 {
		return fmt.Errorf("mapping templates: delete: %w", sql.ErrNoRows)
	}
	return nil
}

// --- scan helpers for TargetImport ---

func (r *TargetImportRepository) scanOne(row *sql.Row) (TargetImport, error) {
	var ti TargetImport
	var headersJSON, previewJSON, mappingJSON, validationJSON []byte
	err := row.Scan(
		&ti.ID, &ti.Filename, &ti.RowCount, &ti.ColumnCount,
		&headersJSON, &previewJSON, &mappingJSON, &ti.Status,
		&validationJSON, &ti.ImportedCount, &ti.RejectedCount,
		&ti.UploadedBy, &ti.CreatedAt, &ti.UpdatedAt,
	)
	if err != nil {
		return TargetImport{}, err
	}

	ti.Headers = []string{}
	if len(headersJSON) > 0 {
		_ = json.Unmarshal(headersJSON, &ti.Headers)
	}
	ti.PreviewRows = [][]string{}
	if len(previewJSON) > 0 {
		_ = json.Unmarshal(previewJSON, &ti.PreviewRows)
	}
	if len(mappingJSON) > 0 {
		ti.Mapping = map[string]string{}
		_ = json.Unmarshal(mappingJSON, &ti.Mapping)
	}
	if len(validationJSON) > 0 {
		ti.ValidationResult = map[string]any{}
		_ = json.Unmarshal(validationJSON, &ti.ValidationResult)
	}

	return ti, nil
}
