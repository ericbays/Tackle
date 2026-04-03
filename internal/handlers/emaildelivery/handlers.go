// Package emaildelivery provides HTTP handlers for email delivery operations.
package emaildelivery

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"

	"tackle/internal/middleware"
	deliverysvc "tackle/internal/services/emaildelivery"
	"tackle/pkg/response"
)

// Deps holds dependencies for email delivery handlers.
type Deps struct {
	Svc *deliverysvc.Service
}

// GetDeliveryStatus handles GET /api/v1/campaigns/{id}/delivery-status.
func (d *Deps) GetDeliveryStatus(w http.ResponseWriter, r *http.Request) {
	correlationID := middleware.GetCorrelationID(r.Context())
	campaignID := chi.URLParam(r, "id")

	metrics, err := d.Svc.GetDeliveryMetrics(r.Context(), campaignID)
	if err != nil {
		response.Error(w, "INTERNAL_ERROR", "failed to get delivery metrics", http.StatusInternalServerError, correlationID)
		return
	}

	_ = correlationID
	response.Success(w, metrics)
}

// HandleDeliveryResult handles POST /internal/delivery-result.
// Receives delivery status callbacks from phishing endpoints.
func (d *Deps) HandleDeliveryResult(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	correlationID := middleware.GetCorrelationID(r.Context())

	var result deliverysvc.DeliveryResult
	if err := json.NewDecoder(r.Body).Decode(&result); err != nil {
		response.Error(w, "VALIDATION_ERROR", "invalid JSON payload", http.StatusBadRequest, correlationID)
		return
	}

	if result.EmailID == "" || result.CampaignID == "" || result.Status == "" {
		response.Error(w, "VALIDATION_ERROR", "email_id, campaign_id, and status are required", http.StatusBadRequest, correlationID)
		return
	}

	if err := d.Svc.ProcessDeliveryResult(r.Context(), result); err != nil {
		response.Error(w, "INTERNAL_ERROR", "failed to process delivery result", http.StatusInternalServerError, correlationID)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(`{"status":"ok"}`))
}
