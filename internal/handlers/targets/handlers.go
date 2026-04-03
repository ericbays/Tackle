package targets

import (
	"database/sql"
	"encoding/csv"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"

	"tackle/internal/middleware"
	"tackle/internal/repositories"
	"tackle/internal/services/auth"
	targetsvc "tackle/internal/services/target"
	"tackle/pkg/response"
)

// ListTargets handles GET /api/v1/targets.
// Supports both page/offset (?page=&per_page=) and cursor (?cursor=&limit=) pagination.
func (d *Deps) ListTargets(w http.ResponseWriter, r *http.Request) {
	correlationID := middleware.GetCorrelationID(r.Context())
	q := r.URL.Query()

	filters := repositories.TargetFilters{
		Email:      q.Get("email"),
		FirstName:  q.Get("first_name"),
		LastName:   q.Get("last_name"),
		Department: q.Get("department"),
		Title:      q.Get("title"),
	}

	if v := q.Get("group_id"); v != "" {
		filters.GroupID = v
	}
	if v := q.Get("campaign_id"); v != "" {
		filters.CampaignID = v
	}
	if v := q.Get("name"); v != "" {
		filters.Name = v
	}
	if from := q.Get("created_from"); from != "" {
		if t, err := time.Parse(time.RFC3339, from); err == nil {
			filters.CreatedFrom = &t
		}
	}
	if to := q.Get("created_to"); to != "" {
		if t, err := time.Parse(time.RFC3339, to); err == nil {
			filters.CreatedTo = &t
		}
	}
	if q.Get("include_deleted") == "true" {
		filters.IncludeDeleted = true
	}

	// Cursor-based pagination when ?cursor= or ?limit= is present (without ?page=).
	cursorParam := q.Get("cursor")
	if cursorParam != "" || (q.Get("limit") != "" && q.Get("page") == "") {
		if cursorParam != "" {
			tok, err := response.DecodeCursor(cursorParam)
			if err != nil {
				response.Error(w, "BAD_REQUEST", "invalid cursor", http.StatusBadRequest, correlationID)
				return
			}
			filters.CursorID = tok.ID
			filters.CursorCreatedAt = &tok.CreatedAt
		}
		if v := q.Get("limit"); v != "" {
			if n, err := strconv.Atoi(v); err == nil {
				filters.Limit = n
			}
		}

		dtos, hasMore, err := d.Svc.ListCursor(r.Context(), filters)
		if err != nil {
			response.Error(w, "INTERNAL_ERROR", "failed to list targets", http.StatusInternalServerError, correlationID)
			return
		}

		var nextCursor string
		if hasMore && len(dtos) > 0 {
			last := dtos[len(dtos)-1]
			if t, err := time.Parse(time.RFC3339, last.CreatedAt); err == nil {
				nextCursor = response.EncodeCursor(last.ID, t)
			}
		}

		response.CursorList(w, dtos, response.CursorPagination{
			NextCursor: nextCursor,
			HasMore:    hasMore,
		})
		return
	}

	// Offset-based pagination (default).
	if p := q.Get("page"); p != "" {
		if v, err := strconv.Atoi(p); err == nil {
			filters.Page = v
		}
	}
	if p := q.Get("per_page"); p != "" {
		if v, err := strconv.Atoi(p); err == nil {
			filters.PerPage = v
		}
	}

	dtos, total, err := d.Svc.List(r.Context(), filters)
	if err != nil {
		response.Error(w, "INTERNAL_ERROR", "failed to list targets", http.StatusInternalServerError, correlationID)
		return
	}

	page := filters.Page
	if page < 1 {
		page = 1
	}
	perPage := filters.PerPage
	if perPage < 1 || perPage > 100 {
		perPage = 25
	}
	totalPages := (total + perPage - 1) / perPage

	response.List(w, dtos, response.Pagination{
		Page:       page,
		PerPage:    perPage,
		Total:      total,
		TotalPages: totalPages,
	})
}

// CreateTarget handles POST /api/v1/targets.
func (d *Deps) CreateTarget(w http.ResponseWriter, r *http.Request) {
	correlationID := middleware.GetCorrelationID(r.Context())
	claims := middleware.ClaimsFromContext(r.Context())
	if claims == nil {
		response.Error(w, "UNAUTHORIZED", "authentication required", http.StatusUnauthorized, correlationID)
		return
	}

	var input targetsvc.CreateInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		response.Error(w, "BAD_REQUEST", "invalid request body", http.StatusBadRequest, correlationID)
		return
	}

	// Field-level validation.
	if errs := validateTargetInput(input.Email); len(errs) > 0 {
		response.ValidationFailed(w, errs, correlationID)
		return
	}

	dto, err := d.Svc.Create(r.Context(), input, claims.Subject, claims.Username, clientIP(r), correlationID)
	if err != nil {
		writeTargetError(w, err, correlationID)
		return
	}
	response.Created(w, dto)
}

// GetTarget handles GET /api/v1/targets/{id}.
func (d *Deps) GetTarget(w http.ResponseWriter, r *http.Request) {
	correlationID := middleware.GetCorrelationID(r.Context())
	id := chi.URLParam(r, "id")

	dto, err := d.Svc.Get(r.Context(), id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			response.Error(w, "NOT_FOUND", "target not found", http.StatusNotFound, correlationID)
			return
		}
		response.Error(w, "INTERNAL_ERROR", "failed to get target", http.StatusInternalServerError, correlationID)
		return
	}
	response.Success(w, dto)
}

// UpdateTarget handles PUT /api/v1/targets/{id}.
func (d *Deps) UpdateTarget(w http.ResponseWriter, r *http.Request) {
	correlationID := middleware.GetCorrelationID(r.Context())
	claims := middleware.ClaimsFromContext(r.Context())
	if claims == nil {
		response.Error(w, "UNAUTHORIZED", "authentication required", http.StatusUnauthorized, correlationID)
		return
	}
	id := chi.URLParam(r, "id")

	var input targetsvc.UpdateInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		response.Error(w, "BAD_REQUEST", "invalid request body", http.StatusBadRequest, correlationID)
		return
	}

	dto, err := d.Svc.Update(r.Context(), id, input, claims.Subject, claims.Username, clientIP(r), correlationID)
	if err != nil {
		writeTargetError(w, err, correlationID)
		return
	}
	response.Success(w, dto)
}

// DeleteTarget handles DELETE /api/v1/targets/{id}.
func (d *Deps) DeleteTarget(w http.ResponseWriter, r *http.Request) {
	correlationID := middleware.GetCorrelationID(r.Context())
	claims := middleware.ClaimsFromContext(r.Context())
	if claims == nil {
		response.Error(w, "UNAUTHORIZED", "authentication required", http.StatusUnauthorized, correlationID)
		return
	}
	id := chi.URLParam(r, "id")

	if err := d.Svc.SoftDelete(r.Context(), id, claims.Subject, claims.Username, clientIP(r), correlationID); err != nil {
		writeTargetError(w, err, correlationID)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// RestoreTarget handles POST /api/v1/targets/{id}/restore.
func (d *Deps) RestoreTarget(w http.ResponseWriter, r *http.Request) {
	correlationID := middleware.GetCorrelationID(r.Context())
	claims := middleware.ClaimsFromContext(r.Context())
	if claims == nil {
		response.Error(w, "UNAUTHORIZED", "authentication required", http.StatusUnauthorized, correlationID)
		return
	}
	id := chi.URLParam(r, "id")

	if err := d.Svc.Restore(r.Context(), id, claims.Subject, claims.Username, clientIP(r), correlationID); err != nil {
		writeTargetError(w, err, correlationID)
		return
	}
	response.Success(w, map[string]string{"status": "restored"})
}

// GetHistory handles GET /api/v1/targets/{id}/history.
func (d *Deps) GetHistory(w http.ResponseWriter, r *http.Request) {
	correlationID := middleware.GetCorrelationID(r.Context())
	id := chi.URLParam(r, "id")

	history, err := d.Svc.GetHistory(r.Context(), id)
	if err != nil {
		response.Error(w, "INTERNAL_ERROR", "failed to get target history", http.StatusInternalServerError, correlationID)
		return
	}
	response.Success(w, history)
}

// CheckEmail handles GET /api/v1/targets/check-email.
func (d *Deps) CheckEmail(w http.ResponseWriter, r *http.Request) {
	correlationID := middleware.GetCorrelationID(r.Context())
	email := r.URL.Query().Get("email")
	if email == "" {
		response.Error(w, "BAD_REQUEST", "email query parameter is required", http.StatusBadRequest, correlationID)
		return
	}

	id, exists, err := d.Svc.CheckEmail(r.Context(), email)
	if err != nil {
		response.Error(w, "INTERNAL_ERROR", "failed to check email", http.StatusInternalServerError, correlationID)
		return
	}
	response.Success(w, map[string]any{"exists": exists, "target_id": id})
}

// GetDepartments handles GET /api/v1/targets/departments.
func (d *Deps) GetDepartments(w http.ResponseWriter, r *http.Request) {
	correlationID := middleware.GetCorrelationID(r.Context())
	depts, err := d.Svc.GetDepartments(r.Context())
	if err != nil {
		response.Error(w, "INTERNAL_ERROR", "failed to get departments", http.StatusInternalServerError, correlationID)
		return
	}
	response.Success(w, depts)
}

// --- CSV Import Endpoints ---

// UploadCSV handles POST /api/v1/targets/import/upload.
func (d *Deps) UploadCSV(w http.ResponseWriter, r *http.Request) {
	correlationID := middleware.GetCorrelationID(r.Context())
	claims := middleware.ClaimsFromContext(r.Context())
	if claims == nil {
		response.Error(w, "UNAUTHORIZED", "authentication required", http.StatusUnauthorized, correlationID)
		return
	}

	// Limit request body size to 50 MB.
	r.Body = http.MaxBytesReader(w, r.Body, 50<<20)

	if err := r.ParseMultipartForm(50 << 20); err != nil {
		response.Error(w, "BAD_REQUEST", "failed to parse multipart form; file may exceed 50 MB limit", http.StatusBadRequest, correlationID)
		return
	}

	file, header, err := r.FormFile("file")
	if err != nil {
		response.Error(w, "BAD_REQUEST", "file field is required", http.StatusBadRequest, correlationID)
		return
	}
	defer file.Close()

	// Check file extension.
	filename := header.Filename
	ext := strings.ToLower(filename)
	if !strings.HasSuffix(ext, ".csv") && !strings.HasSuffix(ext, ".txt") {
		response.Error(w, "BAD_REQUEST", "only .csv and .txt files are accepted", http.StatusBadRequest, correlationID)
		return
	}

	data, err := io.ReadAll(file)
	if err != nil {
		response.Error(w, "BAD_REQUEST", "failed to read uploaded file", http.StatusBadRequest, correlationID)
		return
	}

	result, err := d.Svc.UploadCSV(r.Context(), data, filename, claims.Subject)
	if err != nil {
		writeTargetError(w, err, correlationID)
		return
	}
	response.Created(w, result)
}

// GetImportPreview handles GET /api/v1/targets/import/{upload_id}/preview.
func (d *Deps) GetImportPreview(w http.ResponseWriter, r *http.Request) {
	correlationID := middleware.GetCorrelationID(r.Context())
	uploadID := chi.URLParam(r, "upload_id")

	result, err := d.Svc.GetImportPreview(r.Context(), uploadID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			response.Error(w, "NOT_FOUND", "import not found", http.StatusNotFound, correlationID)
			return
		}
		response.Error(w, "INTERNAL_ERROR", "failed to get import preview", http.StatusInternalServerError, correlationID)
		return
	}
	response.Success(w, result)
}

// SubmitMapping handles POST /api/v1/targets/import/{upload_id}/mapping.
func (d *Deps) SubmitMapping(w http.ResponseWriter, r *http.Request) {
	correlationID := middleware.GetCorrelationID(r.Context())
	uploadID := chi.URLParam(r, "upload_id")

	var input targetsvc.ImportMappingInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		response.Error(w, "BAD_REQUEST", "invalid request body", http.StatusBadRequest, correlationID)
		return
	}

	if err := d.Svc.SubmitMapping(r.Context(), uploadID, input.Mapping); err != nil {
		writeTargetError(w, err, correlationID)
		return
	}
	response.Success(w, map[string]string{"status": "mapped"})
}

// ValidateImport handles POST /api/v1/targets/import/{upload_id}/validate.
func (d *Deps) ValidateImport(w http.ResponseWriter, r *http.Request) {
	correlationID := middleware.GetCorrelationID(r.Context())
	uploadID := chi.URLParam(r, "upload_id")

	result, err := d.Svc.ValidateImport(r.Context(), uploadID)
	if err != nil {
		writeTargetError(w, err, correlationID)
		return
	}
	response.Success(w, result)
}

// CommitImport handles POST /api/v1/targets/import/{upload_id}/commit.
func (d *Deps) CommitImport(w http.ResponseWriter, r *http.Request) {
	correlationID := middleware.GetCorrelationID(r.Context())
	claims := middleware.ClaimsFromContext(r.Context())
	if claims == nil {
		response.Error(w, "UNAUTHORIZED", "authentication required", http.StatusUnauthorized, correlationID)
		return
	}
	uploadID := chi.URLParam(r, "upload_id")

	result, err := d.Svc.CommitImport(r.Context(), uploadID, claims.Subject, claims.Username, clientIP(r), correlationID)
	if err != nil {
		writeTargetError(w, err, correlationID)
		return
	}
	response.Success(w, result)
}

// --- Bulk Operations ---

// BulkDelete handles POST /api/v1/targets/bulk/delete.
func (d *Deps) BulkDelete(w http.ResponseWriter, r *http.Request) {
	correlationID := middleware.GetCorrelationID(r.Context())
	claims := middleware.ClaimsFromContext(r.Context())
	if claims == nil {
		response.Error(w, "UNAUTHORIZED", "authentication required", http.StatusUnauthorized, correlationID)
		return
	}

	var input targetsvc.BulkDeleteInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		response.Error(w, "BAD_REQUEST", "invalid request body", http.StatusBadRequest, correlationID)
		return
	}

	count, err := d.Svc.BulkDelete(r.Context(), input, claims.Subject, claims.Username, clientIP(r), correlationID)
	if err != nil {
		writeTargetError(w, err, correlationID)
		return
	}
	response.Success(w, map[string]int64{"deleted_count": count})
}

// BulkEdit handles POST /api/v1/targets/bulk/edit.
func (d *Deps) BulkEdit(w http.ResponseWriter, r *http.Request) {
	correlationID := middleware.GetCorrelationID(r.Context())
	claims := middleware.ClaimsFromContext(r.Context())
	if claims == nil {
		response.Error(w, "UNAUTHORIZED", "authentication required", http.StatusUnauthorized, correlationID)
		return
	}

	var input targetsvc.BulkEditInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		response.Error(w, "BAD_REQUEST", "invalid request body", http.StatusBadRequest, correlationID)
		return
	}

	count, err := d.Svc.BulkEdit(r.Context(), input, claims.Subject, claims.Username, clientIP(r), correlationID)
	if err != nil {
		writeTargetError(w, err, correlationID)
		return
	}
	response.Success(w, map[string]int64{"updated_count": count})
}

// BulkExport handles POST /api/v1/targets/bulk/export.
func (d *Deps) BulkExport(w http.ResponseWriter, r *http.Request) {
	correlationID := middleware.GetCorrelationID(r.Context())

	var input targetsvc.BulkExportInput
	// Best-effort decode — empty body exports all.
	_ = json.NewDecoder(r.Body).Decode(&input)

	targets, err := d.Svc.BulkExport(r.Context(), input.TargetIDs)
	if err != nil {
		response.Error(w, "INTERNAL_ERROR", "failed to export targets", http.StatusInternalServerError, correlationID)
		return
	}

	w.Header().Set("Content-Type", "text/csv")
	w.Header().Set("Content-Disposition", `attachment; filename="targets-export.csv"`)

	writer := csv.NewWriter(w)
	// Write header.
	_ = writer.Write([]string{"id", "email", "first_name", "last_name", "department", "title", "custom_fields", "created_at"})
	for _, t := range targets {
		firstName := ""
		if t.FirstName != nil {
			firstName = *t.FirstName
		}
		lastName := ""
		if t.LastName != nil {
			lastName = *t.LastName
		}
		dept := ""
		if t.Department != nil {
			dept = *t.Department
		}
		title := ""
		if t.Title != nil {
			title = *t.Title
		}
		cfBytes, _ := json.Marshal(t.CustomFields)
		_ = writer.Write([]string{t.ID, t.Email, firstName, lastName, dept, title, string(cfBytes), t.CreatedAt})
	}
	writer.Flush()
}

// --- Mapping Template Endpoints ---

// ListMappingTemplates handles GET /api/v1/targets/import/mapping-templates.
func (d *Deps) ListMappingTemplates(w http.ResponseWriter, r *http.Request) {
	correlationID := middleware.GetCorrelationID(r.Context())
	dtos, err := d.Svc.ListMappingTemplates(r.Context())
	if err != nil {
		response.Error(w, "INTERNAL_ERROR", "failed to list mapping templates", http.StatusInternalServerError, correlationID)
		return
	}
	response.Success(w, dtos)
}

// CreateMappingTemplate handles POST /api/v1/targets/import/mapping-templates.
func (d *Deps) CreateMappingTemplate(w http.ResponseWriter, r *http.Request) {
	correlationID := middleware.GetCorrelationID(r.Context())
	claims := middleware.ClaimsFromContext(r.Context())
	if claims == nil {
		response.Error(w, "UNAUTHORIZED", "authentication required", http.StatusUnauthorized, correlationID)
		return
	}

	var input struct {
		Name    string            `json:"name"`
		Mapping map[string]string `json:"mapping"`
	}
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		response.Error(w, "BAD_REQUEST", "invalid request body", http.StatusBadRequest, correlationID)
		return
	}

	dto, err := d.Svc.CreateMappingTemplate(r.Context(), input.Name, input.Mapping, claims.Subject)
	if err != nil {
		writeTargetError(w, err, correlationID)
		return
	}
	response.Created(w, dto)
}

// UpdateMappingTemplate handles PUT /api/v1/targets/import/mapping-templates/{id}.
func (d *Deps) UpdateMappingTemplate(w http.ResponseWriter, r *http.Request) {
	correlationID := middleware.GetCorrelationID(r.Context())
	claims := middleware.ClaimsFromContext(r.Context())
	if claims == nil {
		response.Error(w, "UNAUTHORIZED", "authentication required", http.StatusUnauthorized, correlationID)
		return
	}
	id := chi.URLParam(r, "id")

	var input struct {
		Name    string            `json:"name"`
		Mapping map[string]string `json:"mapping"`
	}
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		response.Error(w, "BAD_REQUEST", "invalid request body", http.StatusBadRequest, correlationID)
		return
	}

	dto, err := d.Svc.UpdateMappingTemplate(r.Context(), id, input.Name, input.Mapping, claims.Subject)
	if err != nil {
		writeTargetError(w, err, correlationID)
		return
	}
	response.Success(w, dto)
}

// DeleteMappingTemplate handles DELETE /api/v1/targets/import/mapping-templates/{id}.
func (d *Deps) DeleteMappingTemplate(w http.ResponseWriter, r *http.Request) {
	correlationID := middleware.GetCorrelationID(r.Context())
	id := chi.URLParam(r, "id")

	if err := d.Svc.DeleteMappingTemplate(r.Context(), id); err != nil {
		writeTargetError(w, err, correlationID)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// GetTimeline handles GET /api/v1/campaigns/{id}/targets/{target_id}/timeline.
func (d *Deps) GetTimeline(w http.ResponseWriter, r *http.Request) {
	correlationID := middleware.GetCorrelationID(r.Context())
	claims := middleware.ClaimsFromContext(r.Context())
	campaignID := chi.URLParam(r, "id")
	targetID := chi.URLParam(r, "target_id")
	eventType := r.URL.Query().Get("event_type")

	// Operator/Engineer/Admin can see IP and user-agent; Viewer cannot.
	includeSensitive := claims != nil && hasPermission(claims, "targets:read")

	events, err := d.Svc.GetTimeline(r.Context(), campaignID, targetID, eventType, includeSensitive)
	if err != nil {
		response.Error(w, "INTERNAL_ERROR", "failed to get timeline", http.StatusInternalServerError, correlationID)
		return
	}
	response.Success(w, events)
}

// GetEvents handles GET /api/v1/targets/{id}/events.
func (d *Deps) GetEvents(w http.ResponseWriter, r *http.Request) {
	correlationID := middleware.GetCorrelationID(r.Context())
	claims := middleware.ClaimsFromContext(r.Context())
	id := chi.URLParam(r, "id")

	limit := 50
	offset := 0
	if p := r.URL.Query().Get("limit"); p != "" {
		if v, err := strconv.Atoi(p); err == nil && v > 0 && v <= 100 {
			limit = v
		}
	}
	if p := r.URL.Query().Get("offset"); p != "" {
		if v, err := strconv.Atoi(p); err == nil && v >= 0 {
			offset = v
		}
	}

	includeSensitive := claims != nil && hasPermission(claims, "targets:read")

	events, total, err := d.Svc.GetEvents(r.Context(), id, limit, offset, includeSensitive)
	if err != nil {
		response.Error(w, "INTERNAL_ERROR", "failed to get target events", http.StatusInternalServerError, correlationID)
		return
	}
	response.Success(w, map[string]any{
		"events": events,
		"total":  total,
	})
}

// GetStats handles GET /api/v1/targets/{id}/stats.
func (d *Deps) GetStats(w http.ResponseWriter, r *http.Request) {
	correlationID := middleware.GetCorrelationID(r.Context())
	id := chi.URLParam(r, "id")

	stats, err := d.Svc.GetStats(r.Context(), id)
	if err != nil {
		response.Error(w, "INTERNAL_ERROR", "failed to get target stats", http.StatusInternalServerError, correlationID)
		return
	}
	response.Success(w, stats)
}

// --- helpers ---

var targetEmailRe = regexp.MustCompile(`^[^@\s]+@[^@\s]+\.[^@\s]+$`)

// validateTargetInput performs field-level validation on target input.
func validateTargetInput(email string) []response.FieldError {
	var errs []response.FieldError
	if email == "" {
		errs = append(errs, response.FieldError{Field: "email", Message: "email is required", Code: "required"})
	} else if !targetEmailRe.MatchString(email) {
		errs = append(errs, response.FieldError{Field: "email", Message: "invalid email format", Code: "invalid_format"})
	}
	return errs
}

func writeTargetError(w http.ResponseWriter, err error, correlationID string) {
	var ve *targetsvc.ValidationError
	if errors.As(err, &ve) {
		response.Error(w, "VALIDATION_ERROR", ve.Error(), http.StatusBadRequest, correlationID)
		return
	}
	var ce *targetsvc.ConflictError
	if errors.As(err, &ce) {
		response.Error(w, "CONFLICT", ce.Error(), http.StatusConflict, correlationID)
		return
	}
	var fe *targetsvc.ForbiddenError
	if errors.As(err, &fe) {
		response.Error(w, "FORBIDDEN", fe.Error(), http.StatusForbidden, correlationID)
		return
	}
	if errors.Is(err, sql.ErrNoRows) {
		response.Error(w, "NOT_FOUND", "resource not found", http.StatusNotFound, correlationID)
		return
	}
	response.Error(w, "INTERNAL_ERROR", "internal server error", http.StatusInternalServerError, correlationID)
}

func clientIP(r *http.Request) string {
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		parts := strings.SplitN(xff, ",", 2)
		return strings.TrimSpace(parts[0])
	}
	addr := r.RemoteAddr
	if i := strings.LastIndex(addr, ":"); i > 0 {
		return addr[:i]
	}
	return addr
}

func hasPermission(claims *auth.Claims, perm string) bool {
	if claims == nil {
		return false
	}
	for _, p := range claims.Permissions {
		if p == perm {
			return true
		}
	}
	// Admin role has implicit all-permissions.
	return claims.Role == "admin"
}

// ptrStr dereferences a *string for CSV export.
func ptrStr(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

// fmtInt converts an int to a string for CSV.
func fmtInt(n int) string {
	return fmt.Sprintf("%d", n)
}

// unused import guard.
var _ = strconv.Itoa
