// Package metrics provides HTTP handlers for metrics and defender dashboard endpoints.
package metrics

import (
	"net/http"

	"github.com/go-chi/chi/v5"

	"tackle/internal/middleware"
	metricssvc "tackle/internal/services/metrics"
	"tackle/pkg/response"
)

// Deps holds handler dependencies.
type Deps struct {
	Svc *metricssvc.Service
}

// GetCampaignMetrics handles GET /api/v1/campaigns/{id}/metrics.
func (d *Deps) GetCampaignMetrics(w http.ResponseWriter, r *http.Request) {
	correlationID := middleware.GetCorrelationID(r.Context())
	id := chi.URLParam(r, "id")

	metrics, err := d.Svc.GetCampaignMetrics(r.Context(), id)
	if err != nil {
		response.Error(w, "INTERNAL_ERROR", "failed to get campaign metrics", http.StatusInternalServerError, correlationID)
		return
	}
	response.Success(w, metrics)
}

// GetCampaignTimeline handles GET /api/v1/campaigns/{id}/metrics/timeline.
func (d *Deps) GetCampaignTimeline(w http.ResponseWriter, r *http.Request) {
	correlationID := middleware.GetCorrelationID(r.Context())
	id := chi.URLParam(r, "id")

	timeline, err := d.Svc.GetTimeline(r.Context(), id)
	if err != nil {
		response.Error(w, "INTERNAL_ERROR", "failed to get timeline", http.StatusInternalServerError, correlationID)
		return
	}
	response.Success(w, timeline)
}

// GetOrganizationMetrics handles GET /api/v1/metrics/organization.
func (d *Deps) GetOrganizationMetrics(w http.ResponseWriter, r *http.Request) {
	correlationID := middleware.GetCorrelationID(r.Context())

	metrics, err := d.Svc.GetOrganizationMetrics(r.Context())
	if err != nil {
		response.Error(w, "INTERNAL_ERROR", "failed to get organization metrics", http.StatusInternalServerError, correlationID)
		return
	}
	response.Success(w, metrics)
}

// GetDepartmentMetrics handles GET /api/v1/metrics/departments.
func (d *Deps) GetDepartmentMetrics(w http.ResponseWriter, r *http.Request) {
	correlationID := middleware.GetCorrelationID(r.Context())

	depts, err := d.Svc.GetDepartmentMetrics(r.Context())
	if err != nil {
		response.Error(w, "INTERNAL_ERROR", "failed to get department metrics", http.StatusInternalServerError, correlationID)
		return
	}
	response.Success(w, depts)
}

// GetTrends handles GET /api/v1/metrics/trends.
func (d *Deps) GetTrends(w http.ResponseWriter, r *http.Request) {
	correlationID := middleware.GetCorrelationID(r.Context())

	trends, err := d.Svc.GetTrends(r.Context())
	if err != nil {
		response.Error(w, "INTERNAL_ERROR", "failed to get trends", http.StatusInternalServerError, correlationID)
		return
	}
	response.Success(w, trends)
}
