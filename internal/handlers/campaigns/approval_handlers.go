package campaigns

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"

	"tackle/internal/middleware"
	campaignsvc "tackle/internal/services/campaign"
	"tackle/pkg/response"
)

// SubmitForApproval handles POST /api/v1/campaigns/:id/submit.
// This replaces the simple state transition with full approval validation.
func (d *Deps) SubmitForApproval(w http.ResponseWriter, r *http.Request) {
	correlationID := middleware.GetCorrelationID(r.Context())
	claims := middleware.ClaimsFromContext(r.Context())
	if claims == nil {
		response.Error(w, "UNAUTHORIZED", "authentication required", http.StatusUnauthorized, correlationID)
		return
	}
	id := chi.URLParam(r, "id")

	var input struct {
		RequiredApproverCount int `json:"required_approver_count"`
	}
	_ = json.NewDecoder(r.Body).Decode(&input) // optional body
	if input.RequiredApproverCount < 1 {
		input.RequiredApproverCount = 1
	}

	result, err := d.ApprovalSvc.Submit(r.Context(), id, input.RequiredApproverCount, claims.Subject, claims.Username, claims.Role, clientIP(r), correlationID)
	if err != nil {
		writeApprovalError(w, err, correlationID)
		return
	}
	response.Success(w, result)
}

// ApproveCampaign handles POST /api/v1/campaigns/:id/approve.
func (d *Deps) ApproveCampaign(w http.ResponseWriter, r *http.Request) {
	correlationID := middleware.GetCorrelationID(r.Context())
	claims := middleware.ClaimsFromContext(r.Context())
	if claims == nil {
		response.Error(w, "UNAUTHORIZED", "authentication required", http.StatusUnauthorized, correlationID)
		return
	}
	id := chi.URLParam(r, "id")

	var input campaignsvc.ApproveInput
	_ = json.NewDecoder(r.Body).Decode(&input) // optional comments

	dto, err := d.ApprovalSvc.Approve(r.Context(), id, input, claims.Subject, claims.Username, claims.Role, clientIP(r), correlationID)
	if err != nil {
		writeApprovalError(w, err, correlationID)
		return
	}
	response.Success(w, dto)
}

// RejectCampaign handles POST /api/v1/campaigns/:id/reject.
func (d *Deps) RejectCampaign(w http.ResponseWriter, r *http.Request) {
	correlationID := middleware.GetCorrelationID(r.Context())
	claims := middleware.ClaimsFromContext(r.Context())
	if claims == nil {
		response.Error(w, "UNAUTHORIZED", "authentication required", http.StatusUnauthorized, correlationID)
		return
	}
	id := chi.URLParam(r, "id")

	var input campaignsvc.RejectInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		response.Error(w, "BAD_REQUEST", "invalid request body", http.StatusBadRequest, correlationID)
		return
	}

	dto, err := d.ApprovalSvc.Reject(r.Context(), id, input, claims.Subject, claims.Username, claims.Role, clientIP(r), correlationID)
	if err != nil {
		writeApprovalError(w, err, correlationID)
		return
	}
	response.Success(w, dto)
}

// BlocklistOverrideCampaign handles POST /api/v1/campaigns/:id/blocklist-override.
func (d *Deps) BlocklistOverrideCampaign(w http.ResponseWriter, r *http.Request) {
	correlationID := middleware.GetCorrelationID(r.Context())
	claims := middleware.ClaimsFromContext(r.Context())
	if claims == nil {
		response.Error(w, "UNAUTHORIZED", "authentication required", http.StatusUnauthorized, correlationID)
		return
	}
	id := chi.URLParam(r, "id")

	var input campaignsvc.BlocklistOverrideInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		response.Error(w, "BAD_REQUEST", "invalid request body", http.StatusBadRequest, correlationID)
		return
	}

	dto, err := d.ApprovalSvc.ProcessBlocklistOverride(r.Context(), id, input, claims.Subject, claims.Username, claims.Role, clientIP(r), correlationID)
	if err != nil {
		writeApprovalError(w, err, correlationID)
		return
	}
	response.Success(w, dto)
}

// UnlockCampaignApproval handles POST /api/v1/campaigns/:id/unlock.
// This replaces the simple state transition with approval-aware unlock.
func (d *Deps) UnlockCampaignApproval(w http.ResponseWriter, r *http.Request) {
	correlationID := middleware.GetCorrelationID(r.Context())
	claims := middleware.ClaimsFromContext(r.Context())
	if claims == nil {
		response.Error(w, "UNAUTHORIZED", "authentication required", http.StatusUnauthorized, correlationID)
		return
	}
	id := chi.URLParam(r, "id")

	var input struct {
		Reason string `json:"reason"`
	}
	_ = json.NewDecoder(r.Body).Decode(&input)

	dto, err := d.ApprovalSvc.Unlock(r.Context(), id, input.Reason, claims.Subject, claims.Username, claims.Role, clientIP(r), correlationID)
	if err != nil {
		writeApprovalError(w, err, correlationID)
		return
	}
	response.Success(w, dto)
}

// GetApprovalReview handles GET /api/v1/campaigns/:id/approval-review.
func (d *Deps) GetApprovalReview(w http.ResponseWriter, r *http.Request) {
	correlationID := middleware.GetCorrelationID(r.Context())
	id := chi.URLParam(r, "id")

	review, err := d.ApprovalSvc.GetApprovalReview(r.Context(), id)
	if err != nil {
		writeApprovalError(w, err, correlationID)
		return
	}
	response.Success(w, review)
}

// GetApprovalHistory handles GET /api/v1/campaigns/:id/approval-history.
func (d *Deps) GetApprovalHistory(w http.ResponseWriter, r *http.Request) {
	correlationID := middleware.GetCorrelationID(r.Context())
	id := chi.URLParam(r, "id")

	history, err := d.ApprovalSvc.GetApprovalHistory(r.Context(), id)
	if err != nil {
		writeApprovalError(w, err, correlationID)
		return
	}
	response.Success(w, history)
}

// writeApprovalError maps service errors to HTTP responses including ForbiddenError.
func writeApprovalError(w http.ResponseWriter, err error, correlationID string) {
	var ve *campaignsvc.ValidationError
	if errors.As(err, &ve) {
		response.Error(w, "VALIDATION_ERROR", ve.Error(), http.StatusBadRequest, correlationID)
		return
	}
	var fe *campaignsvc.ForbiddenError
	if errors.As(err, &fe) {
		response.Error(w, "FORBIDDEN", fe.Error(), http.StatusForbidden, correlationID)
		return
	}
	var ce *campaignsvc.ConflictError
	if errors.As(err, &ce) {
		response.Error(w, "CONFLICT", ce.Error(), http.StatusConflict, correlationID)
		return
	}
	var nf *campaignsvc.NotFoundError
	if errors.As(err, &nf) {
		response.Error(w, "NOT_FOUND", nf.Error(), http.StatusNotFound, correlationID)
		return
	}
	response.Error(w, "INTERNAL_ERROR", "internal server error", http.StatusInternalServerError, correlationID)
}
