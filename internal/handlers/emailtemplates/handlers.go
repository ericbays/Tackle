package emailtemplates

import (
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"

	"tackle/internal/middleware"
	emaildeliverysvc "tackle/internal/services/emaildelivery"
	emailtmplsvc "tackle/internal/services/emailtemplate"
	"tackle/pkg/response"
)

// ListTemplates handles GET /api/v1/email-templates.
func (d *Deps) ListTemplates(w http.ResponseWriter, r *http.Request) {
	correlationID := middleware.GetCorrelationID(r.Context())
	q := r.URL.Query()

	var isShared *bool
	if s := q.Get("is_shared"); s != "" {
		v := s == "true"
		isShared = &v
	}

	dtos, err := d.Svc.List(r.Context(),
		q.Get("category"),
		q.Get("name"),
		q.Get("tag"),
		isShared,
		q.Get("created_by"),
	)
	if err != nil {
		response.Error(w, "INTERNAL_ERROR", "failed to list email templates", http.StatusInternalServerError, correlationID)
		return
	}
	response.Success(w, dtos)
}

// GetTemplate handles GET /api/v1/email-templates/{id}.
func (d *Deps) GetTemplate(w http.ResponseWriter, r *http.Request) {
	correlationID := middleware.GetCorrelationID(r.Context())
	id := chi.URLParam(r, "id")

	dto, err := d.Svc.Get(r.Context(), id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			response.Error(w, "NOT_FOUND", "email template not found", http.StatusNotFound, correlationID)
			return
		}
		response.Error(w, "INTERNAL_ERROR", "failed to get email template", http.StatusInternalServerError, correlationID)
		return
	}
	response.Success(w, dto)
}

// CreateTemplate handles POST /api/v1/email-templates.
func (d *Deps) CreateTemplate(w http.ResponseWriter, r *http.Request) {
	correlationID := middleware.GetCorrelationID(r.Context())
	claims := middleware.ClaimsFromContext(r.Context())
	if claims == nil {
		response.Error(w, "UNAUTHORIZED", "authentication required", http.StatusUnauthorized, correlationID)
		return
	}

	var input emailtmplsvc.CreateInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		response.Error(w, "BAD_REQUEST", "invalid request body", http.StatusBadRequest, correlationID)
		return
	}

	dto, err := d.Svc.Create(r.Context(), input, claims.Subject, claims.Username, clientIP(r), correlationID)
	if err != nil {
		writeTemplateError(w, err, correlationID)
		return
	}
	response.Created(w, dto)
}

// UpdateTemplate handles PUT /api/v1/email-templates/{id}.
func (d *Deps) UpdateTemplate(w http.ResponseWriter, r *http.Request) {
	correlationID := middleware.GetCorrelationID(r.Context())
	claims := middleware.ClaimsFromContext(r.Context())
	if claims == nil {
		response.Error(w, "UNAUTHORIZED", "authentication required", http.StatusUnauthorized, correlationID)
		return
	}
	id := chi.URLParam(r, "id")

	var input emailtmplsvc.UpdateInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		response.Error(w, "BAD_REQUEST", "invalid request body", http.StatusBadRequest, correlationID)
		return
	}

	dto, err := d.Svc.Update(r.Context(), id, input, claims.Subject, claims.Username, clientIP(r), correlationID)
	if err != nil {
		writeTemplateError(w, err, correlationID)
		return
	}
	response.Success(w, dto)
}

// DeleteTemplate handles DELETE /api/v1/email-templates/{id}.
func (d *Deps) DeleteTemplate(w http.ResponseWriter, r *http.Request) {
	correlationID := middleware.GetCorrelationID(r.Context())
	claims := middleware.ClaimsFromContext(r.Context())
	if claims == nil {
		response.Error(w, "UNAUTHORIZED", "authentication required", http.StatusUnauthorized, correlationID)
		return
	}
	id := chi.URLParam(r, "id")

	if err := d.Svc.Delete(r.Context(), id, claims.Subject, claims.Username, clientIP(r), correlationID); err != nil {
		writeTemplateError(w, err, correlationID)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// CloneTemplate handles POST /api/v1/email-templates/{id}/clone.
func (d *Deps) CloneTemplate(w http.ResponseWriter, r *http.Request) {
	correlationID := middleware.GetCorrelationID(r.Context())
	claims := middleware.ClaimsFromContext(r.Context())
	if claims == nil {
		response.Error(w, "UNAUTHORIZED", "authentication required", http.StatusUnauthorized, correlationID)
		return
	}
	id := chi.URLParam(r, "id")

	var input emailtmplsvc.CloneInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		response.Error(w, "BAD_REQUEST", "invalid request body", http.StatusBadRequest, correlationID)
		return
	}

	dto, err := d.Svc.Clone(r.Context(), id, input, claims.Subject, claims.Username, clientIP(r), correlationID)
	if err != nil {
		writeTemplateError(w, err, correlationID)
		return
	}
	response.Created(w, dto)
}

// PreviewTemplate handles POST /api/v1/email-templates/{id}/preview.
func (d *Deps) PreviewTemplate(w http.ResponseWriter, r *http.Request) {
	correlationID := middleware.GetCorrelationID(r.Context())
	id := chi.URLParam(r, "id")

	var input emailtmplsvc.PreviewInput
	// Best-effort decode — empty body is fine.
	_ = json.NewDecoder(r.Body).Decode(&input)

	result, err := d.Svc.Preview(r.Context(), id, input)
	if err != nil {
		writeTemplateError(w, err, correlationID)
		return
	}
	response.Success(w, result)
}

// ValidateTemplate handles POST /api/v1/email-templates/{id}/validate.
func (d *Deps) ValidateTemplate(w http.ResponseWriter, r *http.Request) {
	correlationID := middleware.GetCorrelationID(r.Context())
	id := chi.URLParam(r, "id")

	result, err := d.Svc.Validate(r.Context(), id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			response.Error(w, "NOT_FOUND", "email template not found", http.StatusNotFound, correlationID)
			return
		}
		response.Error(w, "INTERNAL_ERROR", "validation failed", http.StatusInternalServerError, correlationID)
		return
	}
	response.Success(w, result)
}

// ListVersions handles GET /api/v1/email-templates/{id}/versions.
func (d *Deps) ListVersions(w http.ResponseWriter, r *http.Request) {
	correlationID := middleware.GetCorrelationID(r.Context())
	id := chi.URLParam(r, "id")

	dtos, err := d.Svc.ListVersions(r.Context(), id)
	if err != nil {
		response.Error(w, "INTERNAL_ERROR", "failed to list versions", http.StatusInternalServerError, correlationID)
		return
	}
	response.Success(w, dtos)
}

// GetVersion handles GET /api/v1/email-templates/{id}/versions/{version}.
func (d *Deps) GetVersion(w http.ResponseWriter, r *http.Request) {
	correlationID := middleware.GetCorrelationID(r.Context())
	id := chi.URLParam(r, "id")
	verStr := chi.URLParam(r, "version")

	ver, err := strconv.Atoi(verStr)
	if err != nil {
		response.Error(w, "BAD_REQUEST", "version must be an integer", http.StatusBadRequest, correlationID)
		return
	}

	dto, err := d.Svc.GetVersion(r.Context(), id, ver)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			response.Error(w, "NOT_FOUND", "version not found", http.StatusNotFound, correlationID)
			return
		}
		response.Error(w, "INTERNAL_ERROR", "failed to get version", http.StatusInternalServerError, correlationID)
		return
	}
	response.Success(w, dto)
}

// ExportTemplate handles GET /api/v1/email-templates/{id}/export.
func (d *Deps) ExportTemplate(w http.ResponseWriter, r *http.Request) {
	correlationID := middleware.GetCorrelationID(r.Context())
	id := chi.URLParam(r, "id")

	dto, err := d.Svc.Export(r.Context(), id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			response.Error(w, "NOT_FOUND", "email template not found", http.StatusNotFound, correlationID)
			return
		}
		response.Error(w, "INTERNAL_ERROR", "failed to export email template", http.StatusInternalServerError, correlationID)
		return
	}

	w.Header().Set("Content-Disposition", `attachment; filename="email-template.json"`)
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(dto)
}

// SendTestEmail handles POST /api/v1/email-templates/{id}/send-test.
func (d *Deps) SendTestEmail(w http.ResponseWriter, r *http.Request) {
	correlationID := middleware.GetCorrelationID(r.Context())

	if d.EmailDeliverySvc == nil {
		response.Error(w, "NOT_CONFIGURED", "test email sending is not configured", http.StatusServiceUnavailable, correlationID)
		return
	}

	claims := middleware.ClaimsFromContext(r.Context())
	if claims == nil {
		response.Error(w, "UNAUTHORIZED", "authentication required", http.StatusUnauthorized, correlationID)
		return
	}

	templateID := chi.URLParam(r, "id")

	var input emaildeliverysvc.SendTestEmailInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		response.Error(w, "BAD_REQUEST", "invalid JSON body", http.StatusBadRequest, correlationID)
		return
	}
	input.TemplateID = templateID

	result, err := d.EmailDeliverySvc.SendTestEmail(r.Context(), input, claims.Subject)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			response.Error(w, "NOT_FOUND", err.Error(), http.StatusNotFound, correlationID)
			return
		}
		response.Error(w, "INTERNAL_ERROR", err.Error(), http.StatusInternalServerError, correlationID)
		return
	}

	response.Success(w, result)
}

// --- helpers ---

func writeTemplateError(w http.ResponseWriter, err error, correlationID string) {
	var ve *emailtmplsvc.ValidationError
	if errors.As(err, &ve) {
		response.Error(w, "VALIDATION_ERROR", ve.Error(), http.StatusBadRequest, correlationID)
		return
	}
	var ce *emailtmplsvc.ConflictError
	if errors.As(err, &ce) {
		response.Error(w, "CONFLICT", ce.Error(), http.StatusConflict, correlationID)
		return
	}
	if errors.Is(err, sql.ErrNoRows) {
		response.Error(w, "NOT_FOUND", "resource not found", http.StatusNotFound, correlationID)
		return
	}
	response.Error(w, "INTERNAL_ERROR", "internal server error", http.StatusInternalServerError, correlationID)
}

// ---------- Attachments ----------

// ListAttachments handles GET /api/v1/email-templates/:id/attachments.
func (d *Deps) ListAttachments(w http.ResponseWriter, r *http.Request) {
	correlationID := middleware.GetCorrelationID(r.Context())
	templateID := chi.URLParam(r, "id")

	if d.AttachmentSvc == nil {
		response.Error(w, "NOT_IMPLEMENTED", "attachment service not configured", http.StatusNotImplemented, correlationID)
		return
	}

	dtos, err := d.AttachmentSvc.List(r.Context(), templateID)
	if err != nil {
		response.Error(w, "INTERNAL_ERROR", "failed to list attachments", http.StatusInternalServerError, correlationID)
		return
	}
	response.Success(w, dtos)
}

// UploadAttachment handles POST /api/v1/email-templates/:id/attachments.
func (d *Deps) UploadAttachment(w http.ResponseWriter, r *http.Request) {
	correlationID := middleware.GetCorrelationID(r.Context())
	templateID := chi.URLParam(r, "id")

	if d.AttachmentSvc == nil {
		response.Error(w, "NOT_IMPLEMENTED", "attachment service not configured", http.StatusNotImplemented, correlationID)
		return
	}

	// Parse multipart form (max 12 MB to account for overhead).
	if err := r.ParseMultipartForm(12 << 20); err != nil {
		response.Error(w, "BAD_REQUEST", "invalid multipart form", http.StatusBadRequest, correlationID)
		return
	}

	file, header, err := r.FormFile("file")
	if err != nil {
		response.Error(w, "BAD_REQUEST", "file field required", http.StatusBadRequest, correlationID)
		return
	}
	defer file.Close()

	contentType := header.Header.Get("Content-Type")
	if contentType == "" {
		contentType = "application/octet-stream"
	}

	dto, err := d.AttachmentSvc.Upload(r.Context(), templateID, header.Filename, contentType, header.Size, file)
	if err != nil {
		if strings.Contains(err.Error(), "not allowed") || strings.Contains(err.Error(), "exceeds maximum") {
			response.Error(w, "VALIDATION_ERROR", err.Error(), http.StatusBadRequest, correlationID)
			return
		}
		response.Error(w, "INTERNAL_ERROR", "failed to upload attachment", http.StatusInternalServerError, correlationID)
		return
	}
	response.Created(w, dto)
}

// DeleteAttachment handles DELETE /api/v1/email-templates/:id/attachments/:aid.
func (d *Deps) DeleteAttachment(w http.ResponseWriter, r *http.Request) {
	correlationID := middleware.GetCorrelationID(r.Context())
	aid := chi.URLParam(r, "aid")

	if d.AttachmentSvc == nil {
		response.Error(w, "NOT_IMPLEMENTED", "attachment service not configured", http.StatusNotImplemented, correlationID)
		return
	}

	if err := d.AttachmentSvc.Delete(r.Context(), aid); err != nil {
		if strings.Contains(err.Error(), "not found") {
			response.Error(w, "NOT_FOUND", "attachment not found", http.StatusNotFound, correlationID)
			return
		}
		response.Error(w, "INTERNAL_ERROR", "failed to delete attachment", http.StatusInternalServerError, correlationID)
		return
	}
	w.WriteHeader(http.StatusNoContent)
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
