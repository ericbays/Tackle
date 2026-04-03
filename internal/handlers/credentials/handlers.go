package credentials

import (
	"encoding/csv"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"

	"tackle/internal/middleware"
	"tackle/internal/repositories"
	credsvc "tackle/internal/services/credential"
	"tackle/pkg/response"
)

// ListCaptures handles GET /api/v1/captures.
func (d *Deps) ListCaptures(w http.ResponseWriter, r *http.Request) {
	correlationID := middleware.GetCorrelationID(r.Context())
	q := r.URL.Query()

	input := credsvc.ListInput{
		CampaignID: q.Get("campaign_id"),
		TargetID:   q.Get("target_id"),
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
	if da := q.Get("date_after"); da != "" {
		if t, err := time.Parse(time.RFC3339, da); err == nil {
			input.DateAfter = &t
		}
	}
	if db := q.Get("date_before"); db != "" {
		if t, err := time.Parse(time.RFC3339, db); err == nil {
			input.DateBefore = &t
		}
	}
	if ua := q.Get("is_unattributed"); ua != "" {
		b := ua == "true"
		input.IsUnattributed = &b
	}

	dtos, total, err := d.Svc.List(r.Context(), input)
	if err != nil {
		response.Error(w, "INTERNAL_ERROR", "failed to list captures", http.StatusInternalServerError, correlationID)
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

// GetCapture handles GET /api/v1/captures/{id}.
func (d *Deps) GetCapture(w http.ResponseWriter, r *http.Request) {
	correlationID := middleware.GetCorrelationID(r.Context())
	id := chi.URLParam(r, "id")

	dto, err := d.Svc.Get(r.Context(), id)
	if err != nil {
		writeError(w, err, correlationID)
		return
	}

	response.Success(w, dto)
}

// ListCampaignCaptures handles GET /api/v1/campaigns/{id}/captures.
func (d *Deps) ListCampaignCaptures(w http.ResponseWriter, r *http.Request) {
	correlationID := middleware.GetCorrelationID(r.Context())
	campaignID := chi.URLParam(r, "id")
	q := r.URL.Query()

	input := credsvc.ListInput{
		CampaignID: campaignID,
		TargetID:   q.Get("target_id"),
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
		response.Error(w, "INTERNAL_ERROR", "failed to list captures", http.StatusInternalServerError, correlationID)
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

// RevealCapture handles POST /api/v1/captures/{id}/reveal.
// Requires credentials:reveal permission; decrypts and returns field values with audit trail.
func (d *Deps) RevealCapture(w http.ResponseWriter, r *http.Request) {
	correlationID := middleware.GetCorrelationID(r.Context())
	claims := middleware.ClaimsFromContext(r.Context())
	if claims == nil {
		response.Error(w, "UNAUTHORIZED", "authentication required", http.StatusUnauthorized, correlationID)
		return
	}
	id := chi.URLParam(r, "id")

	revealed, err := d.Svc.Reveal(r.Context(), id, claims.Subject, claims.Username, clientIP(r), correlationID)
	if err != nil {
		writeError(w, err, correlationID)
		return
	}

	response.Success(w, revealed)
}

// DeleteCapture handles DELETE /api/v1/captures/{id}.
func (d *Deps) DeleteCapture(w http.ResponseWriter, r *http.Request) {
	correlationID := middleware.GetCorrelationID(r.Context())
	claims := middleware.ClaimsFromContext(r.Context())
	if claims == nil {
		response.Error(w, "UNAUTHORIZED", "authentication required", http.StatusUnauthorized, correlationID)
		return
	}
	id := chi.URLParam(r, "id")

	if err := d.Svc.Delete(r.Context(), id, claims.Subject, claims.Username, clientIP(r), correlationID); err != nil {
		writeError(w, err, correlationID)
		return
	}

	response.Success(w, map[string]string{"status": "deleted"})
}

// PurgeCaptures handles POST /api/v1/captures/purge.
func (d *Deps) PurgeCaptures(w http.ResponseWriter, r *http.Request) {
	correlationID := middleware.GetCorrelationID(r.Context())
	claims := middleware.ClaimsFromContext(r.Context())
	if claims == nil {
		response.Error(w, "UNAUTHORIZED", "authentication required", http.StatusUnauthorized, correlationID)
		return
	}

	var input struct {
		CampaignID   string  `json:"campaign_id"`
		DateAfter    *string `json:"date_after"`
		DateBefore   *string `json:"date_before"`
		Confirmation string  `json:"confirmation"`
	}
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		response.Error(w, "BAD_REQUEST", "invalid request body", http.StatusBadRequest, correlationID)
		return
	}

	purgeInput := credsvc.PurgeInput{
		CampaignID:   input.CampaignID,
		Confirmation: input.Confirmation,
	}
	if input.DateAfter != nil {
		if t, err := time.Parse(time.RFC3339, *input.DateAfter); err == nil {
			purgeInput.DateAfter = &t
		}
	}
	if input.DateBefore != nil {
		if t, err := time.Parse(time.RFC3339, *input.DateBefore); err == nil {
			purgeInput.DateBefore = &t
		}
	}

	count, err := d.Svc.Purge(r.Context(), purgeInput, claims.Subject, claims.Username, clientIP(r), correlationID)
	if err != nil {
		writeError(w, err, correlationID)
		return
	}

	response.Success(w, map[string]any{"purged_count": count})
}

// ExportCaptures handles GET /api/v1/captures/export.
func (d *Deps) ExportCaptures(w http.ResponseWriter, r *http.Request) {
	correlationID := middleware.GetCorrelationID(r.Context())
	claims := middleware.ClaimsFromContext(r.Context())
	if claims == nil {
		response.Error(w, "UNAUTHORIZED", "authentication required", http.StatusUnauthorized, correlationID)
		return
	}

	q := r.URL.Query()
	format := q.Get("format")
	if format == "" {
		format = "json"
	}

	// Check if user has reveal permission for including raw values.
	includeRaw := q.Get("include_raw") == "true"
	if includeRaw {
		hasReveal := false
		for _, p := range claims.Permissions {
			if p == "credentials:reveal" {
				hasReveal = true
				break
			}
		}
		// Admin has all permissions implicitly.
		if claims.Role == "admin" {
			hasReveal = true
		}
		if !hasReveal {
			response.Error(w, "FORBIDDEN", "credentials:reveal permission required to include raw values", http.StatusForbidden, correlationID)
			return
		}
	}

	exportInput := credsvc.ExportInput{
		CampaignID: q.Get("campaign_id"),
		IncludeRaw: includeRaw,
		Format:     format,
	}
	if da := q.Get("date_after"); da != "" {
		if t, err := time.Parse(time.RFC3339, da); err == nil {
			exportInput.DateAfter = &t
		}
	}
	if db := q.Get("date_before"); db != "" {
		if t, err := time.Parse(time.RFC3339, db); err == nil {
			exportInput.DateBefore = &t
		}
	}

	rows, err := d.Svc.Export(r.Context(), exportInput, claims.Subject, claims.Username, clientIP(r), correlationID)
	if err != nil {
		writeError(w, err, correlationID)
		return
	}

	switch format {
	case "csv":
		writeCSVExport(w, rows, includeRaw)
	default:
		response.Success(w, rows)
	}
}

// GetCaptureMetrics handles GET /api/v1/campaigns/{id}/capture-metrics.
func (d *Deps) GetCaptureMetrics(w http.ResponseWriter, r *http.Request) {
	correlationID := middleware.GetCorrelationID(r.Context())
	campaignID := chi.URLParam(r, "id")

	metrics, err := d.Svc.GetMetrics(r.Context(), campaignID)
	if err != nil {
		response.Error(w, "INTERNAL_ERROR", "failed to compute metrics", http.StatusInternalServerError, correlationID)
		return
	}

	response.Success(w, metrics)
}

// AssociateTarget handles POST /api/v1/captures/{id}/associate.
func (d *Deps) AssociateTarget(w http.ResponseWriter, r *http.Request) {
	correlationID := middleware.GetCorrelationID(r.Context())
	claims := middleware.ClaimsFromContext(r.Context())
	if claims == nil {
		response.Error(w, "UNAUTHORIZED", "authentication required", http.StatusUnauthorized, correlationID)
		return
	}
	id := chi.URLParam(r, "id")

	var input struct {
		TargetID string `json:"target_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		response.Error(w, "BAD_REQUEST", "invalid request body", http.StatusBadRequest, correlationID)
		return
	}
	if input.TargetID == "" {
		response.Error(w, "VALIDATION_ERROR", "target_id is required", http.StatusBadRequest, correlationID)
		return
	}

	if err := d.Svc.AssociateTarget(r.Context(), id, input.TargetID, claims.Subject, claims.Username, clientIP(r), correlationID); err != nil {
		writeError(w, err, correlationID)
		return
	}

	response.Success(w, map[string]string{"status": "associated"})
}

// GetCategorizationRules handles GET /api/v1/landing-pages/{id}/field-categories.
func (d *Deps) GetCategorizationRules(w http.ResponseWriter, r *http.Request) {
	correlationID := middleware.GetCorrelationID(r.Context())
	lpID := chi.URLParam(r, "id")

	var lpIDPtr *string
	if lpID != "" {
		lpIDPtr = &lpID
	}

	rules, err := d.Svc.GetCategorizationRules(r.Context(), lpIDPtr)
	if err != nil {
		response.Error(w, "INTERNAL_ERROR", "failed to get categorization rules", http.StatusInternalServerError, correlationID)
		return
	}

	response.Success(w, rules)
}

// UpsertCategorizationRule handles POST /api/v1/landing-pages/{id}/field-categories.
func (d *Deps) UpsertCategorizationRule(w http.ResponseWriter, r *http.Request) {
	correlationID := middleware.GetCorrelationID(r.Context())
	lpID := chi.URLParam(r, "id")

	var input struct {
		FieldPattern string `json:"field_pattern"`
		Category     string `json:"category"`
		Priority     int    `json:"priority"`
	}
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		response.Error(w, "BAD_REQUEST", "invalid request body", http.StatusBadRequest, correlationID)
		return
	}

	rule, err := d.Svc.UpsertCategorizationRule(r.Context(), repositories.FieldCategorizationRule{
		LandingPageID: &lpID,
		FieldPattern:  input.FieldPattern,
		Category:      repositories.FieldCategory(input.Category),
		Priority:      input.Priority,
	})
	if err != nil {
		response.Error(w, "INTERNAL_ERROR", "failed to save rule", http.StatusInternalServerError, correlationID)
		return
	}

	response.Success(w, rule)
}

// DeleteCategorizationRule handles DELETE /api/v1/field-categories/{id}.
func (d *Deps) DeleteCategorizationRule(w http.ResponseWriter, r *http.Request) {
	correlationID := middleware.GetCorrelationID(r.Context())
	id := chi.URLParam(r, "id")

	if err := d.Svc.DeleteCategorizationRule(r.Context(), id); err != nil {
		response.Error(w, "INTERNAL_ERROR", "failed to delete rule", http.StatusInternalServerError, correlationID)
		return
	}

	response.Success(w, map[string]string{"status": "deleted"})
}

// writeCSVExport streams CSV export to the response.
func writeCSVExport(w http.ResponseWriter, rows []credsvc.ExportRow, includeRaw bool) {
	w.Header().Set("Content-Type", "text/csv")
	w.Header().Set("Content-Disposition", "attachment; filename=captures.csv")

	csvw := csv.NewWriter(w)
	defer csvw.Flush()

	// Header row.
	header := []string{"event_id", "campaign_id", "target_id", "source_ip", "user_agent", "captured_at", "field_names", "field_categories"}
	if includeRaw {
		header = append(header, "field_values")
	}
	_ = csvw.Write(header)

	for _, row := range rows {
		tid := ""
		if row.TargetID != nil {
			tid = *row.TargetID
		}
		sip := ""
		if row.SourceIP != nil {
			sip = *row.SourceIP
		}
		ua := ""
		if row.UserAgent != nil {
			ua = *row.UserAgent
		}

		catParts := make([]string, len(row.FieldNames))
		for i, fn := range row.FieldNames {
			catParts[i] = fn + "=" + row.FieldCategories[fn]
		}

		record := []string{
			row.EventID,
			row.CampaignID,
			tid,
			sip,
			ua,
			row.CapturedAt.Format(time.RFC3339),
			strings.Join(row.FieldNames, ";"),
			strings.Join(catParts, ";"),
		}
		if includeRaw {
			valParts := make([]string, len(row.FieldNames))
			for i, fn := range row.FieldNames {
				valParts[i] = fn + "=" + row.FieldValues[fn]
			}
			record = append(record, strings.Join(valParts, ";"))
		}
		_ = csvw.Write(record)
	}
}

// writeError maps service errors to HTTP responses.
func writeError(w http.ResponseWriter, err error, correlationID string) {
	var validationErr *credsvc.ValidationError
	var notFoundErr *credsvc.NotFoundError
	var forbiddenErr *credsvc.ForbiddenError

	switch {
	case errors.As(err, &validationErr):
		response.Error(w, "VALIDATION_ERROR", validationErr.Msg, http.StatusBadRequest, correlationID)
	case errors.As(err, &notFoundErr):
		response.Error(w, "NOT_FOUND", notFoundErr.Msg, http.StatusNotFound, correlationID)
	case errors.As(err, &forbiddenErr):
		response.Error(w, "FORBIDDEN", forbiddenErr.Msg, http.StatusForbidden, correlationID)
	default:
		response.Error(w, "INTERNAL_ERROR", fmt.Sprintf("internal error: %v", err), http.StatusInternalServerError, correlationID)
	}
}

// clientIP extracts the client IP from the request.
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
