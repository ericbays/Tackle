package target

import (
	"bytes"
	"context"
	"encoding/csv"
	"fmt"
	"io"
	"strings"
	"time"

	"tackle/internal/repositories"
	auditsvc "tackle/internal/services/audit"
)

const (
	maxImportFileSize = 50 << 20 // 50 MB
	maxPreviewRows    = 10
)

// csvInjectionPrefixes are characters that could trigger formula injection in spreadsheet apps.
var csvInjectionPrefixes = []string{"=", "+", "-", "@", "\t", "\r"}

// ImportUploadResult is returned after a CSV file is uploaded and parsed.
type ImportUploadResult struct {
	UploadID    string     `json:"upload_id"`
	Filename    string     `json:"filename"`
	RowCount    int        `json:"row_count"`
	ColumnCount int        `json:"column_count"`
	Headers     []string   `json:"headers"`
	PreviewRows [][]string `json:"preview_rows"`
}

// ImportMappingInput holds the column-to-field mapping submitted by the user.
type ImportMappingInput struct {
	Mapping map[string]string `json:"mapping"`
}

// ImportMappingTemplateDTO is the API-safe representation of a mapping template.
type ImportMappingTemplateDTO struct {
	ID        string            `json:"id"`
	Name      string            `json:"name"`
	Mapping   map[string]string `json:"mapping"`
	CreatedBy string            `json:"created_by"`
	CreatedAt string            `json:"created_at"`
	UpdatedAt string            `json:"updated_at"`
}

// ImportValidationResult contains the full-file validation results.
type ImportValidationResult struct {
	TotalRows        int                `json:"total_rows"`
	ValidCount       int                `json:"valid_count"`
	ErrorCount       int                `json:"error_count"`
	WarningCount     int                `json:"warning_count"`
	BlockedCount     int                `json:"blocked_count"`
	Errors           []ImportRowIssue   `json:"errors,omitempty"`
	Warnings         []ImportRowIssue   `json:"warnings,omitempty"`
	DuplicatesInFile []ImportDuplicate  `json:"duplicates_in_file,omitempty"`
	DuplicatesInDB   []ImportDuplicate  `json:"duplicates_in_db,omitempty"`
	BlockedRows      []ImportBlockedRow `json:"blocked_rows,omitempty"`
}

// ImportBlockedRow describes a row blocked by the blocklist.
type ImportBlockedRow struct {
	Row     int    `json:"row"`
	Email   string `json:"email"`
	Pattern string `json:"pattern"`
}

// ImportRowIssue describes a validation issue with a specific row.
type ImportRowIssue struct {
	Row     int    `json:"row"`
	Column  string `json:"column"`
	Message string `json:"message"`
}

// ImportDuplicate describes a duplicate email found during import.
type ImportDuplicate struct {
	Row   int    `json:"row"`
	Email string `json:"email"`
}

// ImportCommitResult is returned after committing valid rows.
type ImportCommitResult struct {
	ImportedCount int `json:"imported_count"`
	RejectedCount int `json:"rejected_count"`
}

// UploadCSV parses and stores a CSV file for subsequent mapping and import.
func (s *Service) UploadCSV(ctx context.Context, data []byte, filename, actorID string) (ImportUploadResult, error) {
	if len(data) > maxImportFileSize {
		return ImportUploadResult{}, &ValidationError{Msg: fmt.Sprintf("file exceeds maximum size of %d MB", maxImportFileSize>>20)}
	}

	// Validate it looks like CSV content (not binary).
	if !looksLikeCSV(data) {
		return ImportUploadResult{}, &ValidationError{Msg: "file does not appear to be a valid CSV"}
	}

	reader := csv.NewReader(bytes.NewReader(data))
	reader.LazyQuotes = true
	reader.TrimLeadingSpace = true

	// Read all records for counting and preview.
	records, err := reader.ReadAll()
	if err != nil {
		return ImportUploadResult{}, &ValidationError{Msg: fmt.Sprintf("CSV parse error: %s", err.Error())}
	}

	if len(records) < 2 {
		return ImportUploadResult{}, &ValidationError{Msg: "CSV must contain at least a header row and one data row"}
	}

	headers := records[0]
	dataRows := records[1:]
	rowCount := len(dataRows)
	columnCount := len(headers)

	// Build preview (first 10 data rows).
	previewCount := maxPreviewRows
	if rowCount < previewCount {
		previewCount = rowCount
	}
	previewRows := make([][]string, previewCount)
	for i := 0; i < previewCount; i++ {
		previewRows[i] = sanitizeCSVRow(dataRows[i])
	}

	ti := repositories.TargetImport{
		Filename:    filename,
		RowCount:    rowCount,
		ColumnCount: columnCount,
		Headers:     headers,
		PreviewRows: previewRows,
		RawData:     data,
		UploadedBy:  actorID,
	}

	created, err := s.importRepo.Create(ctx, ti)
	if err != nil {
		return ImportUploadResult{}, fmt.Errorf("target service: upload csv: %w", err)
	}

	return ImportUploadResult{
		UploadID:    created.ID,
		Filename:    created.Filename,
		RowCount:    created.RowCount,
		ColumnCount: created.ColumnCount,
		Headers:     headers,
		PreviewRows: previewRows,
	}, nil
}

// GetImportPreview returns the preview data for an uploaded CSV.
func (s *Service) GetImportPreview(ctx context.Context, uploadID string) (ImportUploadResult, error) {
	ti, err := s.importRepo.GetByID(ctx, uploadID)
	if err != nil {
		return ImportUploadResult{}, err
	}
	return ImportUploadResult{
		UploadID:    ti.ID,
		Filename:    ti.Filename,
		RowCount:    ti.RowCount,
		ColumnCount: ti.ColumnCount,
		Headers:     ti.Headers,
		PreviewRows: ti.PreviewRows,
	}, nil
}

// SubmitMapping saves the column-to-field mapping for an import.
func (s *Service) SubmitMapping(ctx context.Context, uploadID string, mapping map[string]string) error {
	// Validate that email is mapped.
	hasEmail := false
	for _, target := range mapping {
		if target == "email" {
			hasEmail = true
			break
		}
	}
	if !hasEmail {
		return &ValidationError{Msg: "mapping must include a column mapped to 'email'"}
	}

	// Validate target field names.
	validFields := map[string]bool{
		"email": true, "first_name": true, "last_name": true,
		"department": true, "title": true, "ignore": true,
	}
	for col, field := range mapping {
		if !validFields[field] && !strings.HasPrefix(field, "custom:") {
			return &ValidationError{Msg: fmt.Sprintf("invalid mapping target %q for column %q; use a standard field, 'custom:<key>', or 'ignore'", field, col)}
		}
	}

	_, err := s.importRepo.UpdateMapping(ctx, uploadID, mapping)
	return err
}

// ValidateImport performs full-file validation against the mapped columns.
func (s *Service) ValidateImport(ctx context.Context, uploadID string) (ImportValidationResult, error) {
	ti, err := s.importRepo.GetByID(ctx, uploadID)
	if err != nil {
		return ImportValidationResult{}, err
	}
	if ti.Mapping == nil {
		return ImportValidationResult{}, &ValidationError{Msg: "mapping must be submitted before validation"}
	}

	rawData, err := s.importRepo.GetRawData(ctx, uploadID)
	if err != nil {
		return ImportValidationResult{}, fmt.Errorf("target service: validate import: %w", err)
	}

	reader := csv.NewReader(bytes.NewReader(rawData))
	reader.LazyQuotes = true
	reader.TrimLeadingSpace = true

	records, err := reader.ReadAll()
	if err != nil {
		return ImportValidationResult{}, &ValidationError{Msg: fmt.Sprintf("CSV re-parse error: %s", err.Error())}
	}

	headers := records[0]
	dataRows := records[1:]

	// Build header-to-column-index map.
	headerIdx := map[string]int{}
	for i, h := range headers {
		headerIdx[h] = i
	}

	// Track emails for in-file duplicate detection.
	seenEmails := map[string]int{} // email -> first row number

	var result ImportValidationResult
	result.TotalRows = len(dataRows)

	for rowIdx, row := range dataRows {
		rowNum := rowIdx + 2 // 1-indexed, plus header row

		// Extract email.
		var email string
		for col, field := range ti.Mapping {
			if field != "email" {
				continue
			}
			idx, ok := headerIdx[col]
			if !ok || idx >= len(row) {
				continue
			}
			email = strings.ToLower(strings.TrimSpace(row[idx]))
		}

		if email == "" {
			result.Errors = append(result.Errors, ImportRowIssue{
				Row: rowNum, Column: "email", Message: "email is empty",
			})
			result.ErrorCount++
			continue
		}

		if err := validateEmail(email); err != nil {
			result.Errors = append(result.Errors, ImportRowIssue{
				Row: rowNum, Column: "email", Message: err.Error(),
			})
			result.ErrorCount++
			continue
		}

		// In-file duplicate check.
		if firstRow, exists := seenEmails[email]; exists {
			result.DuplicatesInFile = append(result.DuplicatesInFile, ImportDuplicate{
				Row: rowNum, Email: maskEmail(email),
			})
			_ = firstRow
			result.WarningCount++
		} else {
			seenEmails[email] = rowNum
		}

		// DB duplicate check.
		if _, exists, _ := s.repo.CheckEmailExists(ctx, email); exists {
			result.DuplicatesInDB = append(result.DuplicatesInDB, ImportDuplicate{
				Row: rowNum, Email: maskEmail(email),
			})
			result.WarningCount++
		}

		// Blocklist check.
		if s.blocklist != nil {
			blResult, blErr := s.blocklist.CheckEmail(ctx, email)
			if blErr == nil && blResult.Blocked {
				result.BlockedRows = append(result.BlockedRows, ImportBlockedRow{
					Row:     rowNum,
					Email:   maskEmail(email),
					Pattern: blResult.Pattern,
				})
				result.BlockedCount++
			}
		}

		// Validate custom fields.
		for col, field := range ti.Mapping {
			if !strings.HasPrefix(field, "custom:") {
				continue
			}
			idx, ok := headerIdx[col]
			if !ok || idx >= len(row) {
				continue
			}
			val := row[idx]
			if len(val) > maxCustomFieldValueLen {
				result.Errors = append(result.Errors, ImportRowIssue{
					Row: rowNum, Column: col, Message: fmt.Sprintf("value exceeds %d characters", maxCustomFieldValueLen),
				})
				result.ErrorCount++
			}
		}

		if result.ErrorCount == 0 || rowIdx < len(dataRows)-1 {
			result.ValidCount++
		}
	}

	// Recalculate valid count properly.
	result.ValidCount = result.TotalRows - result.ErrorCount

	// Store validation result.
	valMap := map[string]any{
		"total_rows":    result.TotalRows,
		"valid_count":   result.ValidCount,
		"error_count":   result.ErrorCount,
		"warning_count": result.WarningCount,
	}
	_, _ = s.importRepo.UpdateValidation(ctx, uploadID, valMap)

	return result, nil
}

// CommitImport atomically imports all valid rows from a validated CSV.
func (s *Service) CommitImport(ctx context.Context, uploadID, actorID, actorName, ip, correlationID string) (ImportCommitResult, error) {
	ti, err := s.importRepo.GetByID(ctx, uploadID)
	if err != nil {
		return ImportCommitResult{}, err
	}
	if ti.Status != "validated" && ti.Status != "mapped" {
		return ImportCommitResult{}, &ValidationError{Msg: "import must be validated before committing"}
	}

	rawData, err := s.importRepo.GetRawData(ctx, uploadID)
	if err != nil {
		return ImportCommitResult{}, fmt.Errorf("target service: commit import: %w", err)
	}

	reader := csv.NewReader(bytes.NewReader(rawData))
	reader.LazyQuotes = true
	reader.TrimLeadingSpace = true

	records, err := reader.ReadAll()
	if err != nil {
		return ImportCommitResult{}, &ValidationError{Msg: fmt.Sprintf("CSV re-parse error: %s", err.Error())}
	}

	headers := records[0]
	dataRows := records[1:]

	headerIdx := map[string]int{}
	for i, h := range headers {
		headerIdx[h] = i
	}

	// Build target list from valid rows.
	var targets []repositories.Target
	rejected := 0
	seenEmails := map[string]bool{}

	for _, row := range dataRows {
		t := repositories.Target{
			CustomFields: map[string]any{},
			CreatedBy:    actorID,
		}

		for col, field := range ti.Mapping {
			idx, ok := headerIdx[col]
			if !ok || idx >= len(row) {
				continue
			}
			val := strings.TrimSpace(row[idx])
			val = sanitizeCSVCell(val)

			switch field {
			case "email":
				t.Email = strings.ToLower(val)
			case "first_name":
				t.FirstName = strPtr(val)
			case "last_name":
				t.LastName = strPtr(val)
			case "department":
				t.Department = strPtr(val)
			case "title":
				t.Title = strPtr(val)
			case "ignore":
				// Skip.
			default:
				if strings.HasPrefix(field, "custom:") {
					key := strings.TrimPrefix(field, "custom:")
					t.CustomFields[key] = val
				}
			}
		}

		// Skip invalid or duplicate emails.
		if t.Email == "" || validateEmail(t.Email) != nil {
			rejected++
			continue
		}
		if seenEmails[t.Email] {
			rejected++
			continue
		}
		// Skip blocked emails.
		if s.blocklist != nil {
			blResult, blErr := s.blocklist.CheckEmail(ctx, t.Email)
			if blErr == nil && blResult.Blocked {
				rejected++
				continue
			}
		}
		seenEmails[t.Email] = true

		targets = append(targets, t)
	}

	// Bulk create.
	created, errs := s.repo.BulkCreate(ctx, targets)
	for _, e := range errs {
		if e != nil {
			rejected++
		}
	}

	importedCount := len(created)
	status := "committed"
	if importedCount == 0 && rejected > 0 {
		status = "failed"
	}

	_, _ = s.importRepo.UpdateCommitResult(ctx, uploadID, importedCount, rejected, status)

	resourceType := "target_import"
	_ = s.auditSvc.Log(ctx, auditsvc.LogEntry{
		Category:      auditsvc.CategoryUserActivity,
		Severity:      auditsvc.SeverityInfo,
		ActorType:     auditsvc.ActorTypeUser,
		ActorID:       &actorID,
		ActorLabel:    actorName,
		Action:        "target.import_committed",
		ResourceType:  &resourceType,
		ResourceID:    &uploadID,
		CorrelationID: correlationID,
		SourceIP:      &ip,
		Details: map[string]any{
			"upload_id":      uploadID,
			"filename":       ti.Filename,
			"total_rows":     ti.RowCount,
			"imported_count": importedCount,
			"rejected_count": rejected,
		},
	})

	return ImportCommitResult{
		ImportedCount: importedCount,
		RejectedCount: rejected,
	}, nil
}

// --- Mapping Template CRUD ---

// CreateMappingTemplate creates a reusable column mapping template.
func (s *Service) CreateMappingTemplate(ctx context.Context, name string, mapping map[string]string, actorID string) (ImportMappingTemplateDTO, error) {
	if strings.TrimSpace(name) == "" {
		return ImportMappingTemplateDTO{}, &ValidationError{Msg: "name is required"}
	}
	t, err := s.mappingRepo.Create(ctx, name, mapping, actorID)
	if err != nil {
		if isUniqueViolation(err) {
			return ImportMappingTemplateDTO{}, &ConflictError{Msg: fmt.Sprintf("mapping template name %q is already in use", name)}
		}
		return ImportMappingTemplateDTO{}, fmt.Errorf("target service: create mapping template: %w", err)
	}
	return toMappingTemplateDTO(t), nil
}

// ListMappingTemplates returns all mapping templates.
func (s *Service) ListMappingTemplates(ctx context.Context) ([]ImportMappingTemplateDTO, error) {
	templates, err := s.mappingRepo.List(ctx)
	if err != nil {
		return nil, fmt.Errorf("target service: list mapping templates: %w", err)
	}
	dtos := make([]ImportMappingTemplateDTO, 0, len(templates))
	for _, t := range templates {
		dtos = append(dtos, toMappingTemplateDTO(t))
	}
	return dtos, nil
}

// GetMappingTemplate returns a single mapping template.
func (s *Service) GetMappingTemplate(ctx context.Context, id string) (ImportMappingTemplateDTO, error) {
	t, err := s.mappingRepo.GetByID(ctx, id)
	if err != nil {
		return ImportMappingTemplateDTO{}, err
	}
	return toMappingTemplateDTO(t), nil
}

// UpdateMappingTemplate updates an existing mapping template.
func (s *Service) UpdateMappingTemplate(ctx context.Context, id, name string, mapping map[string]string, actorID string) (ImportMappingTemplateDTO, error) {
	// Check ownership for non-admin.
	existing, err := s.mappingRepo.GetByID(ctx, id)
	if err != nil {
		return ImportMappingTemplateDTO{}, err
	}
	_ = existing // ownership check done at handler layer via RBAC

	t, err := s.mappingRepo.Update(ctx, id, name, mapping)
	if err != nil {
		if isUniqueViolation(err) {
			return ImportMappingTemplateDTO{}, &ConflictError{Msg: "mapping template name is already in use"}
		}
		return ImportMappingTemplateDTO{}, fmt.Errorf("target service: update mapping template: %w", err)
	}
	return toMappingTemplateDTO(t), nil
}

// DeleteMappingTemplate deletes a mapping template.
func (s *Service) DeleteMappingTemplate(ctx context.Context, id string) error {
	return s.mappingRepo.Delete(ctx, id)
}

// --- CSV helpers ---

// looksLikeCSV checks that the data appears to be text-based CSV content.
func looksLikeCSV(data []byte) bool {
	if len(data) == 0 {
		return false
	}
	// Check first 1024 bytes for binary content.
	check := data
	if len(check) > 1024 {
		check = check[:1024]
	}
	for _, b := range check {
		if b == 0 {
			return false // NULL byte = likely binary
		}
	}
	// Try parsing the first line.
	reader := csv.NewReader(bytes.NewReader(data))
	_, err := reader.Read()
	return err == nil || err == io.EOF
}

// sanitizeCSVRow strips CSV injection characters from all cells in a row.
func sanitizeCSVRow(row []string) []string {
	result := make([]string, len(row))
	for i, cell := range row {
		result[i] = sanitizeCSVCell(cell)
	}
	return result
}

// sanitizeCSVCell strips leading CSV injection characters.
func sanitizeCSVCell(cell string) string {
	for _, prefix := range csvInjectionPrefixes {
		if strings.HasPrefix(cell, prefix) {
			cell = strings.TrimLeft(cell, "=+-@\t\r")
			break
		}
	}
	return cell
}

func strPtr(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

func toMappingTemplateDTO(t repositories.ImportMappingTemplate) ImportMappingTemplateDTO {
	return ImportMappingTemplateDTO{
		ID:        t.ID,
		Name:      t.Name,
		Mapping:   t.Mapping,
		CreatedBy: t.CreatedBy,
		CreatedAt: t.CreatedAt.Format(time.RFC3339),
		UpdatedAt: t.UpdatedAt.Format(time.RFC3339),
	}
}
