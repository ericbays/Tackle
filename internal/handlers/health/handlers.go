package health

import (
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"

	"tackle/internal/middleware"
	"tackle/internal/repositories"
	healthsvc "tackle/internal/services/health"
	typosvc "tackle/internal/services/typosquat"
	"tackle/pkg/response"
)

// TriggerHealthCheck handles POST /api/v1/domains/{id}/health-check.
func (d *Deps) TriggerHealthCheck(w http.ResponseWriter, r *http.Request) {
	correlationID := middleware.GetCorrelationID(r.Context())
	claims := middleware.ClaimsFromContext(r.Context())
	if claims == nil {
		response.Error(w, "UNAUTHORIZED", "authentication required", http.StatusUnauthorized, correlationID)
		return
	}
	id := chi.URLParam(r, "id")

	var body struct {
		CheckTypes []string `json:"check_types"` // empty = full
	}
	_ = json.NewDecoder(r.Body).Decode(&body)

	var result healthsvc.HealthCheckDTO
	if len(body.CheckTypes) == 0 {
		res, err := d.HealthSvc.RunFullHealthCheck(r.Context(), id, claims.Subject, claims.Username, repositories.HealthTriggerManual)
		if err != nil {
			writeErr(w, err, correlationID)
			return
		}
		result = res.ToDTO()
	} else {
		res, err := d.HealthSvc.RunPartialHealthCheck(r.Context(), id, body.CheckTypes, repositories.HealthTriggerManual, claims.Subject, claims.Username)
		if err != nil {
			writeErr(w, err, correlationID)
			return
		}
		result = res.ToDTO()
	}

	response.Success(w, result)
}

// ListHealthChecks handles GET /api/v1/domains/{id}/health-checks.
func (d *Deps) ListHealthChecks(w http.ResponseWriter, r *http.Request) {
	correlationID := middleware.GetCorrelationID(r.Context())
	id := chi.URLParam(r, "id")

	q := r.URL.Query()
	limit, _ := strconv.Atoi(q.Get("limit"))
	offset, _ := strconv.Atoi(q.Get("offset"))
	if limit <= 0 {
		limit = 20
	}

	results, total, err := d.HealthSvc.GetHealthCheckHistory(r.Context(), id, limit, offset)
	if err != nil {
		response.Error(w, "INTERNAL_ERROR", "failed to list health checks", http.StatusInternalServerError, correlationID)
		return
	}

	dtos := make([]healthsvc.HealthCheckDTO, 0, len(results))
	for _, r := range results {
		dtos = append(dtos, r.ToDTO())
	}

	response.Success(w, map[string]any{
		"items":  dtos,
		"total":  total,
		"limit":  limit,
		"offset": offset,
	})
}

// GetLatestHealthCheck handles GET /api/v1/domains/{id}/health-checks/latest.
func (d *Deps) GetLatestHealthCheck(w http.ResponseWriter, r *http.Request) {
	correlationID := middleware.GetCorrelationID(r.Context())
	id := chi.URLParam(r, "id")

	result, err := d.HealthSvc.GetLatestHealthCheck(r.Context(), id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) || strings.Contains(err.Error(), "no rows") {
			response.Error(w, "NOT_FOUND", "no health checks found for this domain", http.StatusNotFound, correlationID)
			return
		}
		response.Error(w, "INTERNAL_ERROR", "failed to get latest health check", http.StatusInternalServerError, correlationID)
		return
	}

	response.Success(w, result.ToDTO())
}

// GetLatestCategorization handles GET /api/v1/domains/{id}/categorization.
func (d *Deps) GetLatestCategorization(w http.ResponseWriter, r *http.Request) {
	correlationID := middleware.GetCorrelationID(r.Context())
	id := chi.URLParam(r, "id")

	summary, err := d.CatSvc.GetLatestCategorization(r.Context(), id)
	if err != nil {
		response.Error(w, "INTERNAL_ERROR", "failed to get categorization", http.StatusInternalServerError, correlationID)
		return
	}

	response.Success(w, summary)
}

// GetCategorizationHistory handles GET /api/v1/domains/{id}/categorization/history.
func (d *Deps) GetCategorizationHistory(w http.ResponseWriter, r *http.Request) {
	correlationID := middleware.GetCorrelationID(r.Context())
	id := chi.URLParam(r, "id")

	q := r.URL.Query()
	service := q.Get("service")
	limit, _ := strconv.Atoi(q.Get("limit"))
	if limit <= 0 {
		limit = 50
	}

	records, err := d.CatSvc.GetCategorizationHistory(r.Context(), id, service, limit)
	if err != nil {
		response.Error(w, "INTERNAL_ERROR", "failed to get categorization history", http.StatusInternalServerError, correlationID)
		return
	}

	type item struct {
		ID              string `json:"id"`
		Service         string `json:"service"`
		Category        string `json:"category"`
		Status          string `json:"status"`
		CheckedAt       string `json:"checked_at"`
	}
	items := make([]item, 0, len(records))
	for _, rec := range records {
		items = append(items, item{
			ID:        rec.ID,
			Service:   rec.Service,
			Category:  rec.Category,
			Status:    string(rec.Status),
			CheckedAt: rec.CheckedAt.UTC().Format("2006-01-02T15:04:05Z07:00"),
		})
	}

	response.Success(w, map[string]any{"items": items, "total": len(items)})
}

// TriggerCategorization handles POST /api/v1/domains/{id}/categorization/check.
func (d *Deps) TriggerCategorization(w http.ResponseWriter, r *http.Request) {
	correlationID := middleware.GetCorrelationID(r.Context())
	claims := middleware.ClaimsFromContext(r.Context())
	if claims == nil {
		response.Error(w, "UNAUTHORIZED", "authentication required", http.StatusUnauthorized, correlationID)
		return
	}
	id := chi.URLParam(r, "id")

	summary, err := d.CatSvc.CheckCategorization(r.Context(), id, claims.Subject, claims.Username)
	if err != nil {
		writeErr(w, err, correlationID)
		return
	}

	response.Success(w, summary)
}

// GenerateTyposquats handles POST /api/v1/tools/typosquat.
func (d *Deps) GenerateTyposquats(w http.ResponseWriter, r *http.Request) {
	correlationID := middleware.GetCorrelationID(r.Context())
	claims := middleware.ClaimsFromContext(r.Context())
	if claims == nil {
		response.Error(w, "UNAUTHORIZED", "authentication required", http.StatusUnauthorized, correlationID)
		return
	}

	var body struct {
		TargetDomain           string   `json:"target_domain"`
		RegistrarConnectionIDs []string `json:"registrar_connection_ids"`
		CheckAvailability      bool     `json:"check_availability"`
		SortBy                 string   `json:"sort_by"`
		FilterTechnique        string   `json:"filter_technique"`
		FilterAvailability     string   `json:"filter_availability"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		response.Error(w, "BAD_REQUEST", "invalid request body", http.StatusBadRequest, correlationID)
		return
	}
	if body.TargetDomain == "" {
		response.Error(w, "BAD_REQUEST", `"target_domain" is required`, http.StatusBadRequest, correlationID)
		return
	}

	input := typosvc.GenerateAndCheckInput{
		TargetDomain:           body.TargetDomain,
		RegistrarConnectionIDs: body.RegistrarConnectionIDs,
		CheckAvailability:      body.CheckAvailability,
		SortBy:                 body.SortBy,
		FilterTechnique:        body.FilterTechnique,
		FilterAvailability:     body.FilterAvailability,
	}

	result, err := d.TypoSvc.GenerateAndCheck(r.Context(), input, claims.Subject, claims.Username)
	if err != nil {
		writeErr(w, err, correlationID)
		return
	}

	response.Success(w, result)
}

// RegisterTyposquat handles POST /api/v1/tools/typosquat/register.
func (d *Deps) RegisterTyposquat(w http.ResponseWriter, r *http.Request) {
	correlationID := middleware.GetCorrelationID(r.Context())
	claims := middleware.ClaimsFromContext(r.Context())
	if claims == nil {
		response.Error(w, "UNAUTHORIZED", "authentication required", http.StatusUnauthorized, correlationID)
		return
	}

	var body struct {
		CandidateDomain       string `json:"candidate_domain"`
		RegistrarConnectionID string `json:"registrar_connection_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		response.Error(w, "BAD_REQUEST", "invalid request body", http.StatusBadRequest, correlationID)
		return
	}
	if body.CandidateDomain == "" || body.RegistrarConnectionID == "" {
		response.Error(w, "BAD_REQUEST", `"candidate_domain" and "registrar_connection_id" are required`, http.StatusBadRequest, correlationID)
		return
	}

	dto, pending, err := d.TypoSvc.RegisterFromTyposquat(r.Context(), body.CandidateDomain, body.RegistrarConnectionID,
		claims.Subject, claims.Username, claims.Role, clientIP(r), correlationID)
	if err != nil {
		writeErr(w, err, correlationID)
		return
	}

	if pending {
		response.Success(w, map[string]any{"status": "pending_approval", "domain": body.CandidateDomain})
	} else {
		response.Created(w, dto)
	}
}

// --- helpers ---

func writeErr(w http.ResponseWriter, err error, correlationID string) {
	if errors.Is(err, sql.ErrNoRows) || strings.Contains(err.Error(), "not found") {
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
