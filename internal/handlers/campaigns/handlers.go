package campaigns

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"

	"tackle/internal/campaign"
	"tackle/internal/middleware"
	campaignsvc "tackle/internal/services/campaign"
	"tackle/pkg/response"
)

// ---------- Campaign CRUD ----------

// ListCampaigns handles GET /api/v1/campaigns.
func (d *Deps) ListCampaigns(w http.ResponseWriter, r *http.Request) {
	correlationID := middleware.GetCorrelationID(r.Context())
	q := r.URL.Query()

	input := campaignsvc.ListInput{
		Name:            q.Get("name"),
		CreatedBy:       q.Get("created_by"),
		IncludeArchived: q.Get("include_archived") == "true",
	}

	// Operators see only their own and shared campaigns.
	claims := middleware.ClaimsFromContext(r.Context())
	if claims != nil && claims.Role == "operator" {
		input.OwnerID = claims.Subject
	}

	if states := q.Get("states"); states != "" {
		input.States = strings.Split(states, ",")
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
		response.Error(w, "INTERNAL_ERROR", "failed to list campaigns", http.StatusInternalServerError, correlationID)
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

// CreateCampaign handles POST /api/v1/campaigns.
func (d *Deps) CreateCampaign(w http.ResponseWriter, r *http.Request) {
	correlationID := middleware.GetCorrelationID(r.Context())
	claims := middleware.ClaimsFromContext(r.Context())
	if claims == nil {
		response.Error(w, "UNAUTHORIZED", "authentication required", http.StatusUnauthorized, correlationID)
		return
	}

	var input campaignsvc.CreateInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		response.Error(w, "BAD_REQUEST", "invalid request body", http.StatusBadRequest, correlationID)
		return
	}

	// Field-level validation before calling service.
	if errs := validateCampaignInput(input.Name, input.StartDate, input.EndDate); len(errs) > 0 {
		response.ValidationFailed(w, errs, correlationID)
		return
	}

	dto, err := d.Svc.Create(r.Context(), input, claims.Subject, claims.Username, clientIP(r), correlationID)
	if err != nil {
		writeCampaignError(w, err, correlationID)
		return
	}
	response.Created(w, dto)
}

// GetCampaign handles GET /api/v1/campaigns/:id.
func (d *Deps) GetCampaign(w http.ResponseWriter, r *http.Request) {
	correlationID := middleware.GetCorrelationID(r.Context())
	claims := middleware.ClaimsFromContext(r.Context())
	id := chi.URLParam(r, "id")

	// Operator ownership check.
	if claims != nil && !d.canAccessCampaign(r.Context(), claims.Role, claims.Subject, id) {
		response.Error(w, "NOT_FOUND", "campaign not found", http.StatusNotFound, correlationID)
		return
	}

	dto, err := d.Svc.Get(r.Context(), id)
	if err != nil {
		writeCampaignError(w, err, correlationID)
		return
	}
	response.Success(w, dto)
}

// UpdateCampaign handles PUT /api/v1/campaigns/:id.
func (d *Deps) UpdateCampaign(w http.ResponseWriter, r *http.Request) {
	correlationID := middleware.GetCorrelationID(r.Context())
	claims := middleware.ClaimsFromContext(r.Context())
	if claims == nil {
		response.Error(w, "UNAUTHORIZED", "authentication required", http.StatusUnauthorized, correlationID)
		return
	}
	id := chi.URLParam(r, "id")

	if !d.canAccessCampaign(r.Context(), claims.Role, claims.Subject, id) {
		response.Error(w, "NOT_FOUND", "campaign not found", http.StatusNotFound, correlationID)
		return
	}

	var input campaignsvc.UpdateInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		response.Error(w, "BAD_REQUEST", "invalid request body", http.StatusBadRequest, correlationID)
		return
	}

	// Field-level validation for update.
	name := ""
	if input.Name != nil {
		name = *input.Name
	}
	if errs := validateCampaignInput(name, input.StartDate, input.EndDate); len(errs) > 0 {
		response.ValidationFailed(w, errs, correlationID)
		return
	}

	dto, err := d.Svc.Update(r.Context(), id, input, claims.Subject, claims.Username, clientIP(r), correlationID)
	if err != nil {
		writeCampaignError(w, err, correlationID)
		return
	}
	response.Success(w, dto)
}

// DeleteCampaign handles DELETE /api/v1/campaigns/:id.
func (d *Deps) DeleteCampaign(w http.ResponseWriter, r *http.Request) {
	correlationID := middleware.GetCorrelationID(r.Context())
	claims := middleware.ClaimsFromContext(r.Context())
	if claims == nil {
		response.Error(w, "UNAUTHORIZED", "authentication required", http.StatusUnauthorized, correlationID)
		return
	}
	id := chi.URLParam(r, "id")

	if !d.canAccessCampaign(r.Context(), claims.Role, claims.Subject, id) {
		response.Error(w, "NOT_FOUND", "campaign not found", http.StatusNotFound, correlationID)
		return
	}

	if err := d.Svc.Delete(r.Context(), id, claims.Subject, claims.Username, clientIP(r), correlationID); err != nil {
		writeCampaignError(w, err, correlationID)
		return
	}
	response.Success(w, map[string]string{"message": "campaign deleted"})
}

// ---------- State Transitions ----------

// SubmitCampaign handles POST /api/v1/campaigns/:id/submit.
func (d *Deps) SubmitCampaign(w http.ResponseWriter, r *http.Request) {
	d.handleTransition(w, r, campaign.StatePendingApproval)
}

// BuildCampaign handles POST /api/v1/campaigns/:id/build.
// It transitions to Building, starts the async build pipeline, and returns 202 Accepted.
func (d *Deps) BuildCampaign(w http.ResponseWriter, r *http.Request) {
	correlationID := middleware.GetCorrelationID(r.Context())
	claims := middleware.ClaimsFromContext(r.Context())
	if claims == nil {
		response.Error(w, "UNAUTHORIZED", "authentication required", http.StatusUnauthorized, correlationID)
		return
	}
	id := chi.URLParam(r, "id")

	// Check if campaign is already building (prevent duplicate builds).
	existing, err := d.Svc.Get(r.Context(), id)
	if err != nil {
		writeCampaignError(w, err, correlationID)
		return
	}
	if existing.CurrentState == string(campaign.StateBuilding) {
		response.Error(w, "CONFLICT", "campaign is already building", http.StatusConflict, correlationID)
		return
	}

	var input campaignsvc.TransitionInput
	_ = json.NewDecoder(r.Body).Decode(&input)

	// Transition to building state.
	dto, err := d.Svc.Transition(r.Context(), id, campaign.StateBuilding, input.Reason, claims.Subject, claims.Username, claims.Role, clientIP(r), correlationID)
	if err != nil {
		writeCampaignError(w, err, correlationID)
		return
	}

	// Start the build pipeline asynchronously.
	if d.Builder != nil {
		go d.Builder.Build(context.Background(), id)
	}

	response.Accepted(w, dto)
}

// LaunchCampaign handles POST /api/v1/campaigns/:id/launch.
// It transitions to Active and returns 202 Accepted since launching is async.
func (d *Deps) LaunchCampaign(w http.ResponseWriter, r *http.Request) {
	correlationID := middleware.GetCorrelationID(r.Context())
	claims := middleware.ClaimsFromContext(r.Context())
	if claims == nil {
		response.Error(w, "UNAUTHORIZED", "authentication required", http.StatusUnauthorized, correlationID)
		return
	}
	id := chi.URLParam(r, "id")

	var input campaignsvc.TransitionInput
	_ = json.NewDecoder(r.Body).Decode(&input)

	dto, err := d.Svc.Transition(r.Context(), id, campaign.StateActive, input.Reason, claims.Subject, claims.Username, claims.Role, clientIP(r), correlationID)
	if err != nil {
		writeCampaignError(w, err, correlationID)
		return
	}
	response.Accepted(w, dto)
}

// PauseCampaign handles POST /api/v1/campaigns/:id/pause.
func (d *Deps) PauseCampaign(w http.ResponseWriter, r *http.Request) {
	d.handleTransition(w, r, campaign.StatePaused)
}

// ResumeCampaign handles POST /api/v1/campaigns/:id/resume.
func (d *Deps) ResumeCampaign(w http.ResponseWriter, r *http.Request) {
	d.handleTransition(w, r, campaign.StateActive)
}

// CompleteCampaign handles POST /api/v1/campaigns/:id/complete.
func (d *Deps) CompleteCampaign(w http.ResponseWriter, r *http.Request) {
	d.handleTransition(w, r, campaign.StateCompleted)
}

// ArchiveCampaign handles POST /api/v1/campaigns/:id/archive.
func (d *Deps) ArchiveCampaign(w http.ResponseWriter, r *http.Request) {
	d.handleTransition(w, r, campaign.StateArchived)
}

// UnlockCampaign handles POST /api/v1/campaigns/:id/unlock.
func (d *Deps) UnlockCampaign(w http.ResponseWriter, r *http.Request) {
	d.handleTransition(w, r, campaign.StateDraft)
}

func (d *Deps) handleTransition(w http.ResponseWriter, r *http.Request, targetState campaign.State) {
	correlationID := middleware.GetCorrelationID(r.Context())
	claims := middleware.ClaimsFromContext(r.Context())
	if claims == nil {
		response.Error(w, "UNAUTHORIZED", "authentication required", http.StatusUnauthorized, correlationID)
		return
	}
	id := chi.URLParam(r, "id")

	var input campaignsvc.TransitionInput
	// Body is optional for transitions.
	_ = json.NewDecoder(r.Body).Decode(&input)

	dto, err := d.Svc.Transition(r.Context(), id, targetState, input.Reason, claims.Subject, claims.Username, claims.Role, clientIP(r), correlationID)
	if err != nil {
		writeCampaignError(w, err, correlationID)
		return
	}
	response.Success(w, dto)
}

// ---------- Template Variants ----------

// SetTemplateVariants handles PUT /api/v1/campaigns/:id/template-variants.
func (d *Deps) SetTemplateVariants(w http.ResponseWriter, r *http.Request) {
	correlationID := middleware.GetCorrelationID(r.Context())
	claims := middleware.ClaimsFromContext(r.Context())
	if claims == nil {
		response.Error(w, "UNAUTHORIZED", "authentication required", http.StatusUnauthorized, correlationID)
		return
	}
	id := chi.URLParam(r, "id")

	var input campaignsvc.TemplateVariantInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		response.Error(w, "BAD_REQUEST", "invalid request body", http.StatusBadRequest, correlationID)
		return
	}

	dtos, err := d.Svc.SetTemplateVariants(r.Context(), id, input, claims.Subject, claims.Username, clientIP(r), correlationID)
	if err != nil {
		writeCampaignError(w, err, correlationID)
		return
	}
	response.Success(w, dtos)
}

// GetTemplateVariants handles GET /api/v1/campaigns/:id/template-variants.
func (d *Deps) GetTemplateVariants(w http.ResponseWriter, r *http.Request) {
	correlationID := middleware.GetCorrelationID(r.Context())
	id := chi.URLParam(r, "id")

	dtos, err := d.Svc.GetTemplateVariants(r.Context(), id)
	if err != nil {
		writeCampaignError(w, err, correlationID)
		return
	}
	response.Success(w, dtos)
}

// ---------- Send Windows ----------

// SetSendWindows handles PUT /api/v1/campaigns/:id/send-windows.
func (d *Deps) SetSendWindows(w http.ResponseWriter, r *http.Request) {
	correlationID := middleware.GetCorrelationID(r.Context())
	claims := middleware.ClaimsFromContext(r.Context())
	if claims == nil {
		response.Error(w, "UNAUTHORIZED", "authentication required", http.StatusUnauthorized, correlationID)
		return
	}
	id := chi.URLParam(r, "id")

	var input campaignsvc.SendWindowInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		response.Error(w, "BAD_REQUEST", "invalid request body", http.StatusBadRequest, correlationID)
		return
	}

	dtos, err := d.Svc.SetSendWindows(r.Context(), id, input, claims.Subject, claims.Username, clientIP(r), correlationID)
	if err != nil {
		writeCampaignError(w, err, correlationID)
		return
	}
	response.Success(w, dtos)
}

// GetSendWindows handles GET /api/v1/campaigns/:id/send-windows.
func (d *Deps) GetSendWindows(w http.ResponseWriter, r *http.Request) {
	correlationID := middleware.GetCorrelationID(r.Context())
	id := chi.URLParam(r, "id")

	dtos, err := d.Svc.GetSendWindows(r.Context(), id)
	if err != nil {
		writeCampaignError(w, err, correlationID)
		return
	}
	response.Success(w, dtos)
}

// ---------- Metrics and Build Logs ----------

// GetMetrics handles GET /api/v1/campaigns/:id/metrics.
func (d *Deps) GetMetrics(w http.ResponseWriter, r *http.Request) {
	correlationID := middleware.GetCorrelationID(r.Context())
	id := chi.URLParam(r, "id")

	dto, err := d.Svc.GetMetrics(r.Context(), id)
	if err != nil {
		writeCampaignError(w, err, correlationID)
		return
	}
	response.Success(w, dto)
}

// GetBuildLog handles GET /api/v1/campaigns/:id/build-log.
func (d *Deps) GetBuildLog(w http.ResponseWriter, r *http.Request) {
	correlationID := middleware.GetCorrelationID(r.Context())
	id := chi.URLParam(r, "id")

	dtos, err := d.Svc.GetBuildLogs(r.Context(), id)
	if err != nil {
		writeCampaignError(w, err, correlationID)
		return
	}
	response.Success(w, dtos)
}

// GetVariantComparison handles GET /api/v1/campaigns/:id/variant-comparison.
func (d *Deps) GetVariantComparison(w http.ResponseWriter, r *http.Request) {
	correlationID := middleware.GetCorrelationID(r.Context())
	id := chi.URLParam(r, "id")

	dto, err := d.Svc.GetVariantComparison(r.Context(), id)
	if err != nil {
		writeCampaignError(w, err, correlationID)
		return
	}
	response.Success(w, dto)
}

// ---------- Clone ----------

// CloneCampaign handles POST /api/v1/campaigns/:id/clone.
func (d *Deps) CloneCampaign(w http.ResponseWriter, r *http.Request) {
	correlationID := middleware.GetCorrelationID(r.Context())
	claims := middleware.ClaimsFromContext(r.Context())
	if claims == nil {
		response.Error(w, "UNAUTHORIZED", "authentication required", http.StatusUnauthorized, correlationID)
		return
	}
	id := chi.URLParam(r, "id")

	var input campaignsvc.CloneInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		response.Error(w, "BAD_REQUEST", "invalid request body", http.StatusBadRequest, correlationID)
		return
	}

	dto, err := d.Svc.Clone(r.Context(), id, input, claims.Subject, claims.Username, clientIP(r), correlationID)
	if err != nil {
		writeCampaignError(w, err, correlationID)
		return
	}
	response.Created(w, dto)
}

// ---------- Campaign Config Templates ----------

// ListConfigTemplates handles GET /api/v1/campaign-templates.
func (d *Deps) ListConfigTemplates(w http.ResponseWriter, r *http.Request) {
	correlationID := middleware.GetCorrelationID(r.Context())

	dtos, err := d.Svc.ListConfigTemplates(r.Context())
	if err != nil {
		response.Error(w, "INTERNAL_ERROR", "failed to list templates", http.StatusInternalServerError, correlationID)
		return
	}
	response.Success(w, dtos)
}

// CreateConfigTemplate handles POST /api/v1/campaign-templates.
func (d *Deps) CreateConfigTemplate(w http.ResponseWriter, r *http.Request) {
	correlationID := middleware.GetCorrelationID(r.Context())
	claims := middleware.ClaimsFromContext(r.Context())
	if claims == nil {
		response.Error(w, "UNAUTHORIZED", "authentication required", http.StatusUnauthorized, correlationID)
		return
	}

	var input campaignsvc.ConfigTemplateInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		response.Error(w, "BAD_REQUEST", "invalid request body", http.StatusBadRequest, correlationID)
		return
	}

	dto, err := d.Svc.CreateConfigTemplate(r.Context(), input, claims.Subject, claims.Username, clientIP(r), correlationID)
	if err != nil {
		writeCampaignError(w, err, correlationID)
		return
	}
	response.Created(w, dto)
}

// GetConfigTemplate handles GET /api/v1/campaign-templates/:id.
func (d *Deps) GetConfigTemplate(w http.ResponseWriter, r *http.Request) {
	correlationID := middleware.GetCorrelationID(r.Context())
	id := chi.URLParam(r, "id")

	dto, err := d.Svc.GetConfigTemplate(r.Context(), id)
	if err != nil {
		writeCampaignError(w, err, correlationID)
		return
	}
	response.Success(w, dto)
}

// UpdateConfigTemplate handles PUT /api/v1/campaign-templates/:id.
func (d *Deps) UpdateConfigTemplate(w http.ResponseWriter, r *http.Request) {
	correlationID := middleware.GetCorrelationID(r.Context())
	claims := middleware.ClaimsFromContext(r.Context())
	if claims == nil {
		response.Error(w, "UNAUTHORIZED", "authentication required", http.StatusUnauthorized, correlationID)
		return
	}
	id := chi.URLParam(r, "id")

	var input campaignsvc.ConfigTemplateInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		response.Error(w, "BAD_REQUEST", "invalid request body", http.StatusBadRequest, correlationID)
		return
	}

	dto, err := d.Svc.UpdateConfigTemplate(r.Context(), id, input, claims.Subject, claims.Username, clientIP(r), correlationID)
	if err != nil {
		writeCampaignError(w, err, correlationID)
		return
	}
	response.Success(w, dto)
}

// DeleteConfigTemplate handles DELETE /api/v1/campaign-templates/:id.
func (d *Deps) DeleteConfigTemplate(w http.ResponseWriter, r *http.Request) {
	correlationID := middleware.GetCorrelationID(r.Context())
	claims := middleware.ClaimsFromContext(r.Context())
	if claims == nil {
		response.Error(w, "UNAUTHORIZED", "authentication required", http.StatusUnauthorized, correlationID)
		return
	}
	id := chi.URLParam(r, "id")

	if err := d.Svc.DeleteConfigTemplate(r.Context(), id, claims.Subject, claims.Username, clientIP(r), correlationID); err != nil {
		writeCampaignError(w, err, correlationID)
		return
	}
	response.Success(w, map[string]string{"message": "template deleted"})
}

// ApplyConfigTemplate handles POST /api/v1/campaign-templates/:id/apply.
func (d *Deps) ApplyConfigTemplate(w http.ResponseWriter, r *http.Request) {
	correlationID := middleware.GetCorrelationID(r.Context())
	claims := middleware.ClaimsFromContext(r.Context())
	if claims == nil {
		response.Error(w, "UNAUTHORIZED", "authentication required", http.StatusUnauthorized, correlationID)
		return
	}
	id := chi.URLParam(r, "id")

	dto, err := d.Svc.ApplyConfigTemplate(r.Context(), id, claims.Subject, claims.Username, clientIP(r), correlationID)
	if err != nil {
		writeCampaignError(w, err, correlationID)
		return
	}
	response.Created(w, dto)
}

// ---------- Helpers ----------

// ---------- Canary Targets ----------

// SetCanaryTargets handles POST /api/v1/campaigns/:id/canary-targets.
func (d *Deps) SetCanaryTargets(w http.ResponseWriter, r *http.Request) {
	correlationID := middleware.GetCorrelationID(r.Context())
	claims := middleware.ClaimsFromContext(r.Context())
	if claims == nil {
		response.Error(w, "UNAUTHORIZED", "authentication required", http.StatusUnauthorized, correlationID)
		return
	}
	id := chi.URLParam(r, "id")

	var input struct {
		TargetIDs []string `json:"target_ids"`
	}
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		response.Error(w, "BAD_REQUEST", "invalid request body", http.StatusBadRequest, correlationID)
		return
	}

	err := d.Svc.SetCanaryTargets(r.Context(), id, input.TargetIDs, claims.Subject)
	if err != nil {
		writeCampaignError(w, err, correlationID)
		return
	}
	response.Success(w, map[string]any{"canary_count": len(input.TargetIDs)})
}

// GetCanaryTargets handles GET /api/v1/campaigns/:id/canary-targets.
func (d *Deps) GetCanaryTargets(w http.ResponseWriter, r *http.Request) {
	correlationID := middleware.GetCorrelationID(r.Context())
	id := chi.URLParam(r, "id")

	targetIDs, err := d.Svc.GetCanaryTargets(r.Context(), id)
	if err != nil {
		writeCampaignError(w, err, correlationID)
		return
	}
	response.Success(w, targetIDs)
}

// validateCampaignInput performs field-level validation on campaign create/update input.
func validateCampaignInput(name string, startDate, endDate *time.Time) []response.FieldError {
	var errs []response.FieldError
	if name != "" && len(name) > 255 {
		errs = append(errs, response.FieldError{Field: "name", Message: "campaign name must be 255 characters or less", Code: "too_long"})
	}
	if startDate != nil && endDate != nil && !endDate.After(*startDate) {
		errs = append(errs, response.FieldError{Field: "end_date", Message: "end_date must be after start_date", Code: "invalid_value"})
	}
	return errs
}

func writeCampaignError(w http.ResponseWriter, err error, correlationID string) {
	var ve *campaignsvc.ValidationError
	if errors.As(err, &ve) {
		response.Error(w, "VALIDATION_ERROR", ve.Error(), http.StatusBadRequest, correlationID)
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
