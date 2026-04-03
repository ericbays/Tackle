// Package endpoints provides HTTP handlers for endpoint management operations:
// heartbeat ingestion, health monitoring, lifecycle (stop/restart/terminate),
// request log queries, TLS certificate upload, and phishing report webhooks.
package endpoints

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"

	"tackle/internal/endpoint"
	"tackle/internal/endpoint/cloud"
	"tackle/internal/middleware"
	"tackle/internal/services/endpointmgmt"
	"tackle/pkg/response"
)

// CloudProviderResolver resolves a cloud.Provider for a given endpoint.
type CloudProviderResolver interface {
	ResolveForEndpoint(endpointID string) (cloud.Provider, error)
}

// Deps holds the dependencies for endpoint management handlers.
type Deps struct {
	Svc              *endpointmgmt.Service
	ProviderResolver CloudProviderResolver
	DNSUpdater       endpoint.DNSUpdater
}

func writeEndpointError(w http.ResponseWriter, err error, correlationID string) {
	switch err.(type) {
	case *endpointmgmt.ValidationError:
		response.Error(w, "BAD_REQUEST", err.Error(), http.StatusBadRequest, correlationID)
	case *endpointmgmt.NotFoundError:
		response.Error(w, "NOT_FOUND", err.Error(), http.StatusNotFound, correlationID)
	case *endpointmgmt.ConflictError:
		response.Error(w, "CONFLICT", err.Error(), http.StatusConflict, correlationID)
	default:
		response.Error(w, "INTERNAL_ERROR", "internal server error", http.StatusInternalServerError, correlationID)
	}
}

// --- MGMT-06: Heartbeat Receiver ---

// ReceiveHeartbeat handles POST /api/v1/endpoints/heartbeat from endpoint binaries.
// Authenticated via bearer token (comm auth), not user JWT.
func (d *Deps) ReceiveHeartbeat(w http.ResponseWriter, r *http.Request) {
	correlationID := middleware.GetCorrelationID(r.Context())

	var payload endpointmgmt.HeartbeatPayload
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		response.Error(w, "BAD_REQUEST", "invalid JSON payload", http.StatusBadRequest, correlationID)
		return
	}

	if err := d.Svc.ProcessHeartbeat(r.Context(), payload); err != nil {
		writeEndpointError(w, err, correlationID)
		return
	}

	response.Success(w, map[string]string{"status": "ok"})
}

// --- Request Log Ingestion ---

// ReceiveRequestLogs handles POST /api/v1/endpoints/logs from endpoint binaries.
func (d *Deps) ReceiveRequestLogs(w http.ResponseWriter, r *http.Request) {
	correlationID := middleware.GetCorrelationID(r.Context())

	var body struct {
		EndpointID string                         `json:"endpoint_id"`
		Logs       []endpointmgmt.RequestLogEntry `json:"logs"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		response.Error(w, "BAD_REQUEST", "invalid JSON payload", http.StatusBadRequest, correlationID)
		return
	}

	if body.EndpointID == "" {
		response.Error(w, "BAD_REQUEST", "endpoint_id is required", http.StatusBadRequest, correlationID)
		return
	}

	if err := d.Svc.IngestRequestLogs(r.Context(), body.EndpointID, body.Logs); err != nil {
		writeEndpointError(w, err, correlationID)
		return
	}

	response.Success(w, map[string]int{"ingested": len(body.Logs)})
}

// --- MGMT-07: Health Status ---

// GetCampaignEndpointHealth handles GET /api/v1/campaigns/:id/endpoint/health.
func (d *Deps) GetCampaignEndpointHealth(w http.ResponseWriter, r *http.Request) {
	correlationID := middleware.GetCorrelationID(r.Context())
	campaignID := chi.URLParam(r, "id")

	ep, err := d.Svc.GetCampaignEndpoint(r.Context(), campaignID)
	if err != nil {
		writeEndpointError(w, err, correlationID)
		return
	}

	// Get health history.
	sinceStr := r.URL.Query().Get("since")
	since := time.Now().Add(-24 * time.Hour)
	if sinceStr != "" {
		if t, err := time.Parse(time.RFC3339, sinceStr); err == nil {
			since = t
		}
	}

	limitStr := r.URL.Query().Get("limit")
	limit := 100
	if limitStr != "" {
		if l, err := strconv.Atoi(limitStr); err == nil {
			limit = l
		}
	}

	history, err := d.Svc.GetHealthHistory(r.Context(), ep.ID, since, limit)
	if err != nil {
		writeEndpointError(w, err, correlationID)
		return
	}

	healthStatus, err := d.Svc.GetHealthStatus(r.Context(), ep.ID)
	if err != nil {
		writeEndpointError(w, err, correlationID)
		return
	}

	response.Success(w, map[string]any{
		"status":  healthStatus,
		"history": history,
	})
}

// --- MGMT-08: Stop and Restart ---

// StopCampaignEndpoint handles POST /api/v1/campaigns/:id/endpoint/stop.
func (d *Deps) StopCampaignEndpoint(w http.ResponseWriter, r *http.Request) {
	correlationID := middleware.GetCorrelationID(r.Context())
	claims := middleware.ClaimsFromContext(r.Context())
	if claims == nil {
		response.Error(w, "UNAUTHORIZED", "authentication required", http.StatusUnauthorized, correlationID)
		return
	}
	campaignID := chi.URLParam(r, "id")

	ep, err := d.Svc.GetCampaignEndpoint(r.Context(), campaignID)
	if err != nil {
		writeEndpointError(w, err, correlationID)
		return
	}

	provider, err := d.resolveProvider(ep.ID, correlationID, w)
	if err != nil {
		return
	}

	if err := d.Svc.StopEndpoint(r.Context(), ep.ID, claims.Subject, provider); err != nil {
		writeEndpointError(w, err, correlationID)
		return
	}

	response.Success(w, map[string]string{"status": "stopped"})
}

// RestartCampaignEndpoint handles POST /api/v1/campaigns/:id/endpoint/restart.
func (d *Deps) RestartCampaignEndpoint(w http.ResponseWriter, r *http.Request) {
	correlationID := middleware.GetCorrelationID(r.Context())
	claims := middleware.ClaimsFromContext(r.Context())
	if claims == nil {
		response.Error(w, "UNAUTHORIZED", "authentication required", http.StatusUnauthorized, correlationID)
		return
	}
	campaignID := chi.URLParam(r, "id")

	ep, err := d.Svc.GetCampaignEndpoint(r.Context(), campaignID)
	if err != nil {
		writeEndpointError(w, err, correlationID)
		return
	}

	provider, err := d.resolveProvider(ep.ID, correlationID, w)
	if err != nil {
		return
	}

	if err := d.Svc.RestartEndpoint(r.Context(), ep.ID, claims.Subject, provider); err != nil {
		writeEndpointError(w, err, correlationID)
		return
	}

	response.Success(w, map[string]string{"status": "restarted"})
}

// --- MGMT-09: Terminate ---

// TerminateCampaignEndpoint handles DELETE /api/v1/campaigns/:id/endpoint.
func (d *Deps) TerminateCampaignEndpoint(w http.ResponseWriter, r *http.Request) {
	correlationID := middleware.GetCorrelationID(r.Context())
	claims := middleware.ClaimsFromContext(r.Context())
	if claims == nil {
		response.Error(w, "UNAUTHORIZED", "authentication required", http.StatusUnauthorized, correlationID)
		return
	}
	campaignID := chi.URLParam(r, "id")

	ep, err := d.Svc.GetCampaignEndpoint(r.Context(), campaignID)
	if err != nil {
		writeEndpointError(w, err, correlationID)
		return
	}

	provider, err := d.resolveProvider(ep.ID, correlationID, w)
	if err != nil {
		return
	}

	if d.DNSUpdater == nil {
		response.Error(w, "SERVICE_UNAVAILABLE", "DNS updater not configured — configure cloud credentials in Settings first", http.StatusServiceUnavailable, correlationID)
		return
	}

	if err := d.Svc.TerminateEndpoint(r.Context(), ep.ID, claims.Subject, provider, d.DNSUpdater, "", ""); err != nil {
		writeEndpointError(w, err, correlationID)
		return
	}

	response.Success(w, map[string]string{"status": "terminated"})
}

// --- MGMT-11: Retry ---

// RetryCampaignEndpoint handles POST /api/v1/campaigns/:id/endpoint/retry.
func (d *Deps) RetryCampaignEndpoint(w http.ResponseWriter, r *http.Request) {
	correlationID := middleware.GetCorrelationID(r.Context())
	claims := middleware.ClaimsFromContext(r.Context())
	if claims == nil {
		response.Error(w, "UNAUTHORIZED", "authentication required", http.StatusUnauthorized, correlationID)
		return
	}
	campaignID := chi.URLParam(r, "id")

	ep, err := d.Svc.GetCampaignEndpoint(r.Context(), campaignID)
	if err != nil {
		writeEndpointError(w, err, correlationID)
		return
	}

	if err := d.Svc.RetryEndpoint(r.Context(), ep.ID, claims.Subject); err != nil {
		writeEndpointError(w, err, correlationID)
		return
	}

	response.Success(w, map[string]string{"status": "retrying"})
}

// --- MGMT-14: Status and List ---

// GetCampaignEndpoint handles GET /api/v1/campaigns/:id/endpoint.
func (d *Deps) GetCampaignEndpoint(w http.ResponseWriter, r *http.Request) {
	correlationID := middleware.GetCorrelationID(r.Context())
	campaignID := chi.URLParam(r, "id")

	ep, err := d.Svc.GetCampaignEndpoint(r.Context(), campaignID)
	if err != nil {
		writeEndpointError(w, err, correlationID)
		return
	}

	response.Success(w, ep)
}

// GetCampaignEndpointLogs handles GET /api/v1/campaigns/:id/endpoint/logs.
func (d *Deps) GetCampaignEndpointLogs(w http.ResponseWriter, r *http.Request) {
	correlationID := middleware.GetCorrelationID(r.Context())
	campaignID := chi.URLParam(r, "id")

	ep, err := d.Svc.GetCampaignEndpoint(r.Context(), campaignID)
	if err != nil {
		writeEndpointError(w, err, correlationID)
		return
	}

	filter := endpointmgmt.RequestLogFilter{
		EndpointID: ep.ID,
		CampaignID: campaignID,
	}

	if v := r.URL.Query().Get("source_ip"); v != "" {
		filter.SourceIP = v
	}
	if v := r.URL.Query().Get("method"); v != "" {
		filter.Method = v
	}
	if v := r.URL.Query().Get("path"); v != "" {
		filter.Path = v
	}
	if v := r.URL.Query().Get("since"); v != "" {
		if t, err := time.Parse(time.RFC3339, v); err == nil {
			filter.Since = t
		}
	}
	if v := r.URL.Query().Get("limit"); v != "" {
		if l, err := strconv.Atoi(v); err == nil {
			filter.Limit = l
		}
	}
	if v := r.URL.Query().Get("offset"); v != "" {
		if o, err := strconv.Atoi(v); err == nil {
			filter.Offset = o
		}
	}

	logs, total, err := d.Svc.GetRequestLogs(r.Context(), filter)
	if err != nil {
		writeEndpointError(w, err, correlationID)
		return
	}

	response.Success(w, map[string]any{
		"logs":  logs,
		"total": total,
	})
}

// ListAllEndpoints handles GET /api/v1/endpoints.
func (d *Deps) ListAllEndpoints(w http.ResponseWriter, r *http.Request) {
	correlationID := middleware.GetCorrelationID(r.Context())

	stateFilter := r.URL.Query().Get("state")

	eps, err := d.Svc.ListAllEndpoints(r.Context(), stateFilter)
	if err != nil {
		writeEndpointError(w, err, correlationID)
		return
	}

	dtos := make([]endpointmgmt.EndpointDTO, len(eps))
	for i, ep := range eps {
		dtos[i] = endpointmgmt.ToDTO(ep)
	}

	response.Success(w, dtos)
}

// --- MGMT-12: TLS Certificate Upload ---

// UploadTLSCertificate handles POST /api/v1/campaigns/:id/endpoint/tls.
func (d *Deps) UploadTLSCertificate(w http.ResponseWriter, r *http.Request) {
	correlationID := middleware.GetCorrelationID(r.Context())
	claims := middleware.ClaimsFromContext(r.Context())
	if claims == nil {
		response.Error(w, "UNAUTHORIZED", "authentication required", http.StatusUnauthorized, correlationID)
		return
	}
	campaignID := chi.URLParam(r, "id")

	ep, err := d.Svc.GetCampaignEndpoint(r.Context(), campaignID)
	if err != nil {
		writeEndpointError(w, err, correlationID)
		return
	}

	var body struct {
		CertPEM string `json:"cert_pem"`
		KeyPEM  string `json:"key_pem"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		response.Error(w, "BAD_REQUEST", "invalid JSON payload", http.StatusBadRequest, correlationID)
		return
	}

	if body.CertPEM == "" || body.KeyPEM == "" {
		response.Error(w, "BAD_REQUEST", "cert_pem and key_pem are required", http.StatusBadRequest, correlationID)
		return
	}

	certInfo, err := d.Svc.UploadTLSCertificate(r.Context(), ep.ID, claims.Subject, []byte(body.CertPEM), []byte(body.KeyPEM))
	if err != nil {
		writeEndpointError(w, err, correlationID)
		return
	}

	response.Created(w, certInfo)
}

// --- MGMT-13: Phishing Report Webhook ---

// ReceivePhishingReport handles POST /api/v1/webhooks/phishing-reports.
func (d *Deps) ReceivePhishingReport(w http.ResponseWriter, r *http.Request) {
	correlationID := middleware.GetCorrelationID(r.Context())

	var payload endpointmgmt.PhishingReportPayload
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		response.Error(w, "BAD_REQUEST", "invalid JSON payload", http.StatusBadRequest, correlationID)
		return
	}

	report, err := d.Svc.ProcessPhishingReport(r.Context(), payload, "webhook", "")
	if err != nil {
		writeEndpointError(w, err, correlationID)
		return
	}

	response.Created(w, report)
}

// ManualPhishingReport handles POST /api/v1/campaigns/:id/phishing-reports.
func (d *Deps) ManualPhishingReport(w http.ResponseWriter, r *http.Request) {
	correlationID := middleware.GetCorrelationID(r.Context())
	claims := middleware.ClaimsFromContext(r.Context())
	if claims == nil {
		response.Error(w, "UNAUTHORIZED", "authentication required", http.StatusUnauthorized, correlationID)
		return
	}

	var payload endpointmgmt.PhishingReportPayload
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		response.Error(w, "BAD_REQUEST", "invalid JSON payload", http.StatusBadRequest, correlationID)
		return
	}

	report, err := d.Svc.ProcessPhishingReport(r.Context(), payload, "manual", claims.Subject)
	if err != nil {
		writeEndpointError(w, err, correlationID)
		return
	}

	response.Created(w, report)
}

func (d *Deps) resolveProvider(endpointID, correlationID string, w http.ResponseWriter) (cloud.Provider, error) {
	if d.ProviderResolver == nil {
		response.Error(w, "SERVICE_UNAVAILABLE", "cloud provider not configured for this endpoint — configure cloud credentials in Settings first", http.StatusServiceUnavailable, correlationID)
		return nil, fmt.Errorf("no provider resolver configured")
	}
	provider, err := d.ProviderResolver.ResolveForEndpoint(endpointID)
	if err != nil {
		response.Error(w, "INTERNAL_ERROR", "failed to resolve cloud provider", http.StatusInternalServerError, correlationID)
		return nil, err
	}
	return provider, nil
}
