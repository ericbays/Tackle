package landingpages

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"

	"tackle/internal/compiler"
	"tackle/internal/middleware"
	lpsvc "tackle/internal/services/landingpage"
	"tackle/pkg/response"
)

// ---------- Project CRUD ----------

// ListProjects handles GET /api/v1/landing-pages.
func (d *Deps) ListProjects(w http.ResponseWriter, r *http.Request) {
	correlationID := middleware.GetCorrelationID(r.Context())
	q := r.URL.Query()

	input := lpsvc.ListProjectInput{
		Name: q.Get("name"),
	}
	if p := q.Get("page"); p != "" {
		if v, err := strconv.Atoi(p); err == nil {
			input.Page = v
		}
	}
	if p := q.Get("per_page"); p != "" {
		if v, err := strconv.Atoi(p); err == nil {
			input.PerPage = v
		}
	}

	dtos, total, err := d.Svc.List(r.Context(), input)
	if err != nil {
		response.Error(w, "INTERNAL_ERROR", "failed to list landing pages", http.StatusInternalServerError, correlationID)
		return
	}

	page := input.Page
	if page < 1 {
		page = 1
	}
	perPage := input.PerPage
	if perPage < 1 || perPage > 100 {
		perPage = 25
	}
	totalPages := (total + perPage - 1) / perPage

	response.List(w, dtos, response.Pagination{
		Page: page, PerPage: perPage, Total: total, TotalPages: totalPages,
	})
}

// CreateProject handles POST /api/v1/landing-pages.
func (d *Deps) CreateProject(w http.ResponseWriter, r *http.Request) {
	correlationID := middleware.GetCorrelationID(r.Context())
	claims := middleware.ClaimsFromContext(r.Context())
	if claims == nil {
		response.Error(w, "UNAUTHORIZED", "authentication required", http.StatusUnauthorized, correlationID)
		return
	}

	var input lpsvc.CreateProjectInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		response.Error(w, "BAD_REQUEST", "invalid request body", http.StatusBadRequest, correlationID)
		return
	}

	dto, err := d.Svc.Create(r.Context(), input, claims.Subject, claims.Username)
	if err != nil {
		handleError(w, err, correlationID)
		return
	}

	response.Created(w, dto)
}

// GetProject handles GET /api/v1/landing-pages/:id.
func (d *Deps) GetProject(w http.ResponseWriter, r *http.Request) {
	correlationID := middleware.GetCorrelationID(r.Context())
	id := chi.URLParam(r, "id")

	dto, err := d.Svc.Get(r.Context(), id)
	if err != nil {
		handleError(w, err, correlationID)
		return
	}

	response.Success(w, dto)
}

// UpdateProject handles PUT /api/v1/landing-pages/:id.
func (d *Deps) UpdateProject(w http.ResponseWriter, r *http.Request) {
	correlationID := middleware.GetCorrelationID(r.Context())
	claims := middleware.ClaimsFromContext(r.Context())
	if claims == nil {
		response.Error(w, "UNAUTHORIZED", "authentication required", http.StatusUnauthorized, correlationID)
		return
	}

	id := chi.URLParam(r, "id")

	var input lpsvc.UpdateProjectInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		slog.Error("landing page update: decode failed", "error", err, "id", id)
		response.Error(w, "BAD_REQUEST", "invalid request body: "+err.Error(), http.StatusBadRequest, correlationID)
		return
	}

	dto, err := d.Svc.Update(r.Context(), id, input, claims.Subject, claims.Username)
	if err != nil {
		slog.Error("landing page update: service error", "error", err, "id", id)
		handleError(w, err, correlationID)
		return
	}

	response.Success(w, dto)
}

// DeleteProject handles DELETE /api/v1/landing-pages/:id.
func (d *Deps) DeleteProject(w http.ResponseWriter, r *http.Request) {
	correlationID := middleware.GetCorrelationID(r.Context())
	claims := middleware.ClaimsFromContext(r.Context())
	if claims == nil {
		response.Error(w, "UNAUTHORIZED", "authentication required", http.StatusUnauthorized, correlationID)
		return
	}

	id := chi.URLParam(r, "id")

	if err := d.Svc.Delete(r.Context(), id, claims.Subject, claims.Username); err != nil {
		handleError(w, err, correlationID)
		return
	}

	response.Success(w, map[string]string{"status": "deleted"})
}

// DuplicateProject handles POST /api/v1/landing-pages/:id/duplicate.
func (d *Deps) DuplicateProject(w http.ResponseWriter, r *http.Request) {
	correlationID := middleware.GetCorrelationID(r.Context())
	claims := middleware.ClaimsFromContext(r.Context())
	if claims == nil {
		response.Error(w, "UNAUTHORIZED", "authentication required", http.StatusUnauthorized, correlationID)
		return
	}

	id := chi.URLParam(r, "id")

	dto, err := d.Svc.Duplicate(r.Context(), id, claims.Subject, claims.Username)
	if err != nil {
		handleError(w, err, correlationID)
		return
	}

	response.Created(w, dto)
}

// ---------- Templates ----------

// ListTemplates handles GET /api/v1/landing-pages/templates.
func (d *Deps) ListTemplates(w http.ResponseWriter, r *http.Request) {
	correlationID := middleware.GetCorrelationID(r.Context())
	claims := middleware.ClaimsFromContext(r.Context())
	if claims == nil {
		response.Error(w, "UNAUTHORIZED", "authentication required", http.StatusUnauthorized, correlationID)
		return
	}

	dtos, err := d.Svc.ListTemplates(r.Context(), claims.Subject)
	if err != nil {
		response.Error(w, "INTERNAL_ERROR", "failed to list templates", http.StatusInternalServerError, correlationID)
		return
	}

	response.Success(w, dtos)
}

// SaveAsTemplate handles POST /api/v1/landing-pages/templates.
func (d *Deps) SaveAsTemplate(w http.ResponseWriter, r *http.Request) {
	correlationID := middleware.GetCorrelationID(r.Context())
	claims := middleware.ClaimsFromContext(r.Context())
	if claims == nil {
		response.Error(w, "UNAUTHORIZED", "authentication required", http.StatusUnauthorized, correlationID)
		return
	}

	var body struct {
		ProjectID   string `json:"project_id"`
		Name        string `json:"name"`
		Description string `json:"description"`
		Category    string `json:"category"`
		IsShared    bool   `json:"is_shared"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		response.Error(w, "BAD_REQUEST", "invalid request body", http.StatusBadRequest, correlationID)
		return
	}

	dto, err := d.Svc.SaveAsTemplate(r.Context(), body.ProjectID, lpsvc.SaveTemplateInput{
		Name:        body.Name,
		Description: body.Description,
		Category:    body.Category,
		IsShared:    body.IsShared,
	}, claims.Subject, claims.Username)
	if err != nil {
		handleError(w, err, correlationID)
		return
	}

	response.Created(w, dto)
}

// UpdateTemplate handles PUT /api/v1/landing-pages/templates/:templateId.
func (d *Deps) UpdateTemplate(w http.ResponseWriter, r *http.Request) {
	correlationID := middleware.GetCorrelationID(r.Context())
	claims := middleware.ClaimsFromContext(r.Context())
	if claims == nil {
		response.Error(w, "UNAUTHORIZED", "authentication required", http.StatusUnauthorized, correlationID)
		return
	}

	templateID := chi.URLParam(r, "templateId")

	var body struct {
		IsShared bool `json:"is_shared"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		response.Error(w, "BAD_REQUEST", "invalid request body", http.StatusBadRequest, correlationID)
		return
	}

	dto, err := d.Svc.UpdateTemplateShared(r.Context(), templateID, body.IsShared, claims.Subject, claims.Username)
	if err != nil {
		handleError(w, err, correlationID)
		return
	}

	response.Success(w, dto)
}

// DeleteTemplate handles DELETE /api/v1/landing-pages/templates/:templateId.
func (d *Deps) DeleteTemplate(w http.ResponseWriter, r *http.Request) {
	correlationID := middleware.GetCorrelationID(r.Context())
	claims := middleware.ClaimsFromContext(r.Context())
	if claims == nil {
		response.Error(w, "UNAUTHORIZED", "authentication required", http.StatusUnauthorized, correlationID)
		return
	}

	templateID := chi.URLParam(r, "templateId")

	if err := d.Svc.DeleteTemplate(r.Context(), templateID, claims.Subject, claims.Username); err != nil {
		handleError(w, err, correlationID)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// ---------- Components ----------

// ListComponents handles GET /api/v1/landing-pages/components.
func (d *Deps) ListComponents(w http.ResponseWriter, r *http.Request) {
	response.Success(w, d.Svc.ListComponentTypes())
}

// ---------- Themes ----------

// ListThemes handles GET /api/v1/landing-pages/themes.
func (d *Deps) ListThemes(w http.ResponseWriter, r *http.Request) {
	response.Success(w, lpsvc.GetBuiltInThemes())
}

// ---------- JS Snippets ----------

// ListJSSnippets handles GET /api/v1/landing-pages/js-snippets.
func (d *Deps) ListJSSnippets(w http.ResponseWriter, r *http.Request) {
	response.Success(w, lpsvc.GetJSSnippetTemplates())
}

// ---------- Starter Templates ----------

// ListStarterTemplates handles GET /api/v1/landing-pages/starter-templates.
func (d *Deps) ListStarterTemplates(w http.ResponseWriter, r *http.Request) {
	response.Success(w, lpsvc.GetStarterTemplates())
}

// ---------- Preview ----------

// PreviewProject handles POST /api/v1/landing-pages/:id/preview.
func (d *Deps) PreviewProject(w http.ResponseWriter, r *http.Request) {
	correlationID := middleware.GetCorrelationID(r.Context())
	id := chi.URLParam(r, "id")

	var input lpsvc.PreviewInput
	if r.ContentLength > 0 {
		if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
			response.Error(w, "BAD_REQUEST", "invalid request body", http.StatusBadRequest, correlationID)
			return
		}
	}

	html, err := d.Svc.GeneratePreview(r.Context(), id, input)
	if err != nil {
		handleError(w, err, correlationID)
		return
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(html))
}

// ---------- Import ----------

// ImportHTML handles POST /api/v1/landing-pages/:id/import.
func (d *Deps) ImportHTML(w http.ResponseWriter, r *http.Request) {
	correlationID := middleware.GetCorrelationID(r.Context())
	claims := middleware.ClaimsFromContext(r.Context())
	if claims == nil {
		response.Error(w, "UNAUTHORIZED", "authentication required", http.StatusUnauthorized, correlationID)
		return
	}

	id := chi.URLParam(r, "id")

	var input lpsvc.ImportHTMLInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		response.Error(w, "BAD_REQUEST", "invalid request body", http.StatusBadRequest, correlationID)
		return
	}

	dto, err := d.Svc.ImportHTML(r.Context(), id, input, claims.Subject, claims.Username)
	if err != nil {
		handleError(w, err, correlationID)
		return
	}

	response.Success(w, dto)
}

// ImportZIP handles POST /api/v1/landing-pages/:id/import-zip.
// Accepts a multipart form upload with a .zip file (max 50 MB).
func (d *Deps) ImportZIP(w http.ResponseWriter, r *http.Request) {
	correlationID := middleware.GetCorrelationID(r.Context())
	claims := middleware.ClaimsFromContext(r.Context())
	if claims == nil {
		response.Error(w, "UNAUTHORIZED", "authentication required", http.StatusUnauthorized, correlationID)
		return
	}

	id := chi.URLParam(r, "id")

	// Parse multipart form: 50 MB max.
	const maxUpload = 50 << 20
	if err := r.ParseMultipartForm(maxUpload); err != nil {
		response.Error(w, "BAD_REQUEST", "file too large or invalid multipart form (max 50 MB)", http.StatusBadRequest, correlationID)
		return
	}

	file, header, err := r.FormFile("file")
	if err != nil {
		response.Error(w, "BAD_REQUEST", "missing 'file' field in multipart form", http.StatusBadRequest, correlationID)
		return
	}
	defer file.Close()

	// Read file into memory for zip.NewReader.
	data, err := io.ReadAll(io.LimitReader(file, maxUpload+1))
	if err != nil {
		response.Error(w, "BAD_REQUEST", "failed to read uploaded file", http.StatusBadRequest, correlationID)
		return
	}
	if int64(len(data)) > maxUpload {
		response.Error(w, "BAD_REQUEST", "file exceeds 50 MB limit", http.StatusBadRequest, correlationID)
		return
	}

	zipReader, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		response.Error(w, "BAD_REQUEST", "invalid ZIP file", http.StatusBadRequest, correlationID)
		return
	}

	dto, err := d.Svc.ImportZIP(r.Context(), id, zipReader, header.Filename, claims.Subject, claims.Username)
	if err != nil {
		handleError(w, err, correlationID)
		return
	}

	response.Success(w, dto)
}

// ---------- Clone URL ----------

// CloneURL handles POST /api/v1/landing-pages/:id/clone-url.
func (d *Deps) CloneURL(w http.ResponseWriter, r *http.Request) {
	correlationID := middleware.GetCorrelationID(r.Context())
	claims := middleware.ClaimsFromContext(r.Context())
	if claims == nil {
		response.Error(w, "UNAUTHORIZED", "authentication required", http.StatusUnauthorized, correlationID)
		return
	}

	id := chi.URLParam(r, "id")

	var input lpsvc.CloneURLInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		response.Error(w, "BAD_REQUEST", "invalid request body", http.StatusBadRequest, correlationID)
		return
	}

	dto, err := d.Svc.CloneFromURL(r.Context(), id, input, claims.Subject, claims.Username)
	if err != nil {
		handleError(w, err, correlationID)
		return
	}

	response.Success(w, dto)
}

// ---------- Build & Hosting ----------

// TriggerBuild handles POST /api/v1/landing-pages/{id}/build.
func (d *Deps) TriggerBuild(w http.ResponseWriter, r *http.Request) {
	correlationID := middleware.GetCorrelationID(r.Context())
	claims := middleware.ClaimsFromContext(r.Context())
	if claims == nil {
		response.Error(w, "UNAUTHORIZED", "authentication required", http.StatusUnauthorized, correlationID)
		return
	}

	projectID := chi.URLParam(r, "id")

	var input compiler.BuildInput
	// Body is optional — all BuildInput fields have defaults.
	if r.Body != nil && r.ContentLength != 0 {
		if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
			response.Error(w, "BAD_REQUEST", "invalid request body", http.StatusBadRequest, correlationID)
			return
		}
	}

	buildID, err := d.Engine.Build(r.Context(), projectID, claims.Subject, claims.Username, input)
	if err != nil {
		response.Error(w, "INTERNAL_ERROR", err.Error(), http.StatusInternalServerError, correlationID)
		return
	}

	response.Accepted(w, map[string]string{
		"id":         buildID,
		"project_id": projectID,
		"status":     "pending",
		"status_url": "/api/v1/landing-pages/" + projectID + "/builds/" + buildID,
	})
}

// ListBuilds handles GET /api/v1/landing-pages/{id}/builds.
func (d *Deps) ListBuilds(w http.ResponseWriter, r *http.Request) {
	correlationID := middleware.GetCorrelationID(r.Context())
	projectID := chi.URLParam(r, "id")

	builds, err := d.Engine.ListBuilds(r.Context(), projectID)
	if err != nil {
		response.Error(w, "INTERNAL_ERROR", "failed to list builds", http.StatusInternalServerError, correlationID)
		return
	}

	response.Success(w, builds)
}

// GetBuild handles GET /api/v1/landing-pages/{id}/builds/{buildId}.
func (d *Deps) GetBuild(w http.ResponseWriter, r *http.Request) {
	correlationID := middleware.GetCorrelationID(r.Context())
	buildID := chi.URLParam(r, "buildId")

	build, err := d.Engine.GetBuild(r.Context(), buildID)
	if err != nil {
		response.Error(w, "NOT_FOUND", "build not found", http.StatusNotFound, correlationID)
		return
	}

	response.Success(w, build)
}

// StartApp handles POST /api/v1/landing-pages/{id}/builds/{buildId}/start.
func (d *Deps) StartApp(w http.ResponseWriter, r *http.Request) {
	correlationID := middleware.GetCorrelationID(r.Context())
	buildID := chi.URLParam(r, "buildId")

	if err := d.AppMgr.StartApp(r.Context(), buildID); err != nil {
		response.Error(w, "INTERNAL_ERROR", err.Error(), http.StatusInternalServerError, correlationID)
		return
	}

	response.Success(w, map[string]string{"status": "starting"})
}

// StopApp handles POST /api/v1/landing-pages/{id}/builds/{buildId}/stop.
func (d *Deps) StopApp(w http.ResponseWriter, r *http.Request) {
	correlationID := middleware.GetCorrelationID(r.Context())
	buildID := chi.URLParam(r, "buildId")

	if err := d.AppMgr.StopApp(r.Context(), buildID); err != nil {
		response.Error(w, "INTERNAL_ERROR", err.Error(), http.StatusInternalServerError, correlationID)
		return
	}

	response.Success(w, map[string]string{"status": "stopped"})
}

// GetAppHealth handles GET /api/v1/landing-pages/{id}/builds/{buildId}/health.
func (d *Deps) GetAppHealth(w http.ResponseWriter, r *http.Request) {
	correlationID := middleware.GetCorrelationID(r.Context())
	buildID := chi.URLParam(r, "buildId")

	healthy, port, err := d.AppMgr.GetHealth(buildID)
	if err != nil {
		response.Error(w, "NOT_FOUND", err.Error(), http.StatusNotFound, correlationID)
		return
	}

	response.Success(w, map[string]any{
		"build_id": buildID,
		"healthy":  healthy,
		"port":     port,
	})
}

// ---------- Error Handling ----------

func handleError(w http.ResponseWriter, err error, correlationID string) {
	var valErr *lpsvc.ValidationError
	if errors.As(err, &valErr) {
		response.Error(w, "VALIDATION_ERROR", valErr.Msg, http.StatusBadRequest, correlationID)
		return
	}
	var notFoundErr *lpsvc.NotFoundError
	if errors.As(err, &notFoundErr) {
		response.Error(w, "NOT_FOUND", notFoundErr.Msg, http.StatusNotFound, correlationID)
		return
	}
	var conflictErr *lpsvc.ConflictError
	if errors.As(err, &conflictErr) {
		response.Error(w, "CONFLICT", conflictErr.Msg, http.StatusConflict, correlationID)
		return
	}
	response.Error(w, "INTERNAL_ERROR", "an internal error occurred", http.StatusInternalServerError, correlationID)
}
