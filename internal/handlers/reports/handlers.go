// Package reports provides HTTP handlers for report generation and management.
package reports

import (
	"encoding/json"
	"net/http"
	"path/filepath"

	"github.com/go-chi/chi/v5"

	"tackle/internal/middleware"
	"tackle/internal/services/report"
	"tackle/pkg/response"
)

// Deps holds handler dependencies.
type Deps struct {
	TemplateSvc *report.TemplateService
	Generator   *report.Generator
}

// ListTemplates handles GET /api/v1/report-templates.
func (d *Deps) ListTemplates(w http.ResponseWriter, r *http.Request) {
	correlationID := middleware.GetCorrelationID(r.Context())

	templates, err := d.TemplateSvc.ListTemplates(r.Context())
	if err != nil {
		response.Error(w, "INTERNAL_ERROR", "failed to list report templates", http.StatusInternalServerError, correlationID)
		return
	}
	response.Success(w, templates)
}

// CreateTemplate handles POST /api/v1/report-templates.
func (d *Deps) CreateTemplate(w http.ResponseWriter, r *http.Request) {
	correlationID := middleware.GetCorrelationID(r.Context())
	claims := middleware.ClaimsFromContext(r.Context())
	if claims == nil {
		response.Error(w, "UNAUTHORIZED", "authentication required", http.StatusUnauthorized, correlationID)
		return
	}

	var input report.CreateTemplateInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		response.Error(w, "BAD_REQUEST", "invalid request body", http.StatusBadRequest, correlationID)
		return
	}

	tmpl, err := d.TemplateSvc.CreateTemplate(r.Context(), input, claims.Subject)
	if err != nil {
		response.Error(w, "INTERNAL_ERROR", "failed to create report template", http.StatusInternalServerError, correlationID)
		return
	}
	response.Created(w, tmpl)
}

// GenerateReport handles POST /api/v1/reports/generate.
func (d *Deps) GenerateReport(w http.ResponseWriter, r *http.Request) {
	correlationID := middleware.GetCorrelationID(r.Context())
	claims := middleware.ClaimsFromContext(r.Context())
	if claims == nil {
		response.Error(w, "UNAUTHORIZED", "authentication required", http.StatusUnauthorized, correlationID)
		return
	}

	var input report.GenerateInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		response.Error(w, "BAD_REQUEST", "invalid request body", http.StatusBadRequest, correlationID)
		return
	}

	if len(input.CampaignIDs) == 0 {
		response.Error(w, "BAD_REQUEST", "at least one campaign_id is required", http.StatusBadRequest, correlationID)
		return
	}
	if input.Title == "" {
		input.Title = "Campaign Report"
	}

	rpt, err := d.Generator.GenerateReport(r.Context(), input, claims.Subject)
	if err != nil {
		response.Error(w, "INTERNAL_ERROR", "failed to start report generation", http.StatusInternalServerError, correlationID)
		return
	}
	response.Accepted(w, rpt)
}

// ListReports handles GET /api/v1/reports.
func (d *Deps) ListReports(w http.ResponseWriter, r *http.Request) {
	correlationID := middleware.GetCorrelationID(r.Context())

	reports, err := d.Generator.ListReports(r.Context())
	if err != nil {
		response.Error(w, "INTERNAL_ERROR", "failed to list reports", http.StatusInternalServerError, correlationID)
		return
	}
	response.Success(w, reports)
}

// GetReport handles GET /api/v1/reports/{id}.
func (d *Deps) GetReport(w http.ResponseWriter, r *http.Request) {
	correlationID := middleware.GetCorrelationID(r.Context())
	id := chi.URLParam(r, "id")

	rpt, err := d.Generator.GetReport(r.Context(), id)
	if err != nil {
		response.Error(w, "NOT_FOUND", "report not found", http.StatusNotFound, correlationID)
		return
	}
	response.Success(w, rpt)
}

// DownloadReport handles GET /api/v1/reports/{id}/download.
func (d *Deps) DownloadReport(w http.ResponseWriter, r *http.Request) {
	correlationID := middleware.GetCorrelationID(r.Context())
	id := chi.URLParam(r, "id")

	filePath, format, err := d.Generator.GetFilePath(r.Context(), id)
	if err != nil {
		response.Error(w, "NOT_FOUND", "report file not found", http.StatusNotFound, correlationID)
		return
	}

	contentType := "application/octet-stream"
	switch format {
	case "csv":
		contentType = "text/csv"
	case "json":
		contentType = "application/json"
	case "html":
		contentType = "text/html"
	}

	w.Header().Set("Content-Type", contentType)
	w.Header().Set("Content-Disposition", "attachment; filename="+filepath.Base(filePath))
	http.ServeFile(w, r, filePath)
}

// DeleteReport handles DELETE /api/v1/reports/{id}.
func (d *Deps) DeleteReport(w http.ResponseWriter, r *http.Request) {
	correlationID := middleware.GetCorrelationID(r.Context())
	id := chi.URLParam(r, "id")

	if err := d.Generator.DeleteReport(r.Context(), id); err != nil {
		response.Error(w, "INTERNAL_ERROR", "failed to delete report", http.StatusInternalServerError, correlationID)
		return
	}
	response.Success(w, map[string]string{"message": "report deleted"})
}
