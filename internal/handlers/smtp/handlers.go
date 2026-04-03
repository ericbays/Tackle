package smtp

import (
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"

	"tackle/internal/middleware"
	smtpsvc "tackle/internal/services/smtpprofile"
	"tackle/pkg/response"
)

// CreateProfile handles POST /api/v1/smtp-profiles.
func (d *Deps) CreateProfile(w http.ResponseWriter, r *http.Request) {
	correlationID := middleware.GetCorrelationID(r.Context())
	claims := middleware.ClaimsFromContext(r.Context())
	if claims == nil {
		response.Error(w, "UNAUTHORIZED", "authentication required", http.StatusUnauthorized, correlationID)
		return
	}

	var input smtpsvc.CreateInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		response.Error(w, "BAD_REQUEST", "invalid request body", http.StatusBadRequest, correlationID)
		return
	}

	// Field-level validation.
	if errs := validateSMTPInput(input); len(errs) > 0 {
		response.ValidationFailed(w, errs, correlationID)
		return
	}

	dto, err := d.Svc.CreateProfile(r.Context(), input, claims.Subject, claims.Username, clientIP(r), correlationID)
	if err != nil {
		writeSMTPError(w, err, correlationID)
		return
	}
	response.Created(w, dto)
}

// ListProfiles handles GET /api/v1/smtp-profiles.
func (d *Deps) ListProfiles(w http.ResponseWriter, r *http.Request) {
	correlationID := middleware.GetCorrelationID(r.Context())
	q := r.URL.Query()
	dtos, err := d.Svc.ListProfiles(r.Context(), q.Get("status"), q.Get("name"))
	if err != nil {
		response.Error(w, "INTERNAL_ERROR", "failed to list SMTP profiles", http.StatusInternalServerError, correlationID)
		return
	}
	response.Success(w, dtos)
}

// GetProfile handles GET /api/v1/smtp-profiles/{id}.
func (d *Deps) GetProfile(w http.ResponseWriter, r *http.Request) {
	correlationID := middleware.GetCorrelationID(r.Context())
	id := chi.URLParam(r, "id")
	dto, err := d.Svc.GetProfile(r.Context(), id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			response.Error(w, "NOT_FOUND", "SMTP profile not found", http.StatusNotFound, correlationID)
			return
		}
		response.Error(w, "INTERNAL_ERROR", "failed to get SMTP profile", http.StatusInternalServerError, correlationID)
		return
	}
	response.Success(w, dto)
}

// UpdateProfile handles PUT /api/v1/smtp-profiles/{id}.
func (d *Deps) UpdateProfile(w http.ResponseWriter, r *http.Request) {
	correlationID := middleware.GetCorrelationID(r.Context())
	claims := middleware.ClaimsFromContext(r.Context())
	if claims == nil {
		response.Error(w, "UNAUTHORIZED", "authentication required", http.StatusUnauthorized, correlationID)
		return
	}
	id := chi.URLParam(r, "id")

	var input smtpsvc.UpdateInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		response.Error(w, "BAD_REQUEST", "invalid request body", http.StatusBadRequest, correlationID)
		return
	}

	dto, err := d.Svc.UpdateProfile(r.Context(), id, input, claims.Subject, claims.Username, clientIP(r), correlationID)
	if err != nil {
		writeSMTPError(w, err, correlationID)
		return
	}
	response.Success(w, dto)
}

// DeleteProfile handles DELETE /api/v1/smtp-profiles/{id}.
func (d *Deps) DeleteProfile(w http.ResponseWriter, r *http.Request) {
	correlationID := middleware.GetCorrelationID(r.Context())
	claims := middleware.ClaimsFromContext(r.Context())
	if claims == nil {
		response.Error(w, "UNAUTHORIZED", "authentication required", http.StatusUnauthorized, correlationID)
		return
	}
	id := chi.URLParam(r, "id")

	if err := d.Svc.DeleteProfile(r.Context(), id, claims.Subject, claims.Username, clientIP(r), correlationID); err != nil {
		writeSMTPError(w, err, correlationID)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// TestProfile handles POST /api/v1/smtp-profiles/{id}/test.
func (d *Deps) TestProfile(w http.ResponseWriter, r *http.Request) {
	correlationID := middleware.GetCorrelationID(r.Context())
	claims := middleware.ClaimsFromContext(r.Context())
	if claims == nil {
		response.Error(w, "UNAUTHORIZED", "authentication required", http.StatusUnauthorized, correlationID)
		return
	}
	id := chi.URLParam(r, "id")

	result, err := d.Svc.TestProfile(r.Context(), id, claims.Subject, claims.Username, clientIP(r), correlationID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			response.Error(w, "NOT_FOUND", "SMTP profile not found", http.StatusNotFound, correlationID)
			return
		}
		response.Error(w, "INTERNAL_ERROR", "test failed", http.StatusInternalServerError, correlationID)
		return
	}
	response.Success(w, result)
}

// DuplicateProfile handles POST /api/v1/smtp-profiles/{id}/duplicate.
func (d *Deps) DuplicateProfile(w http.ResponseWriter, r *http.Request) {
	correlationID := middleware.GetCorrelationID(r.Context())
	claims := middleware.ClaimsFromContext(r.Context())
	if claims == nil {
		response.Error(w, "UNAUTHORIZED", "authentication required", http.StatusUnauthorized, correlationID)
		return
	}
	id := chi.URLParam(r, "id")

	var body struct {
		NewName string `json:"new_name"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		response.Error(w, "BAD_REQUEST", "invalid request body", http.StatusBadRequest, correlationID)
		return
	}

	dto, err := d.Svc.DuplicateProfile(r.Context(), id, body.NewName, claims.Subject, claims.Username, clientIP(r), correlationID)
	if err != nil {
		writeSMTPError(w, err, correlationID)
		return
	}
	response.Created(w, dto)
}

// --- Campaign SMTP association handlers ---

// AssignProfile handles POST /api/v1/campaigns/{id}/smtp-profiles.
func (d *Deps) AssignProfile(w http.ResponseWriter, r *http.Request) {
	correlationID := middleware.GetCorrelationID(r.Context())
	claims := middleware.ClaimsFromContext(r.Context())
	if claims == nil {
		response.Error(w, "UNAUTHORIZED", "authentication required", http.StatusUnauthorized, correlationID)
		return
	}
	campaignID := chi.URLParam(r, "id")

	var input smtpsvc.CampaignAssignInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		response.Error(w, "BAD_REQUEST", "invalid request body", http.StatusBadRequest, correlationID)
		return
	}

	dto, err := d.Svc.AssignProfile(r.Context(), campaignID, input, claims.Subject, claims.Username, clientIP(r), correlationID)
	if err != nil {
		writeSMTPError(w, err, correlationID)
		return
	}
	response.Created(w, dto)
}

// ListCampaignProfiles handles GET /api/v1/campaigns/{id}/smtp-profiles.
func (d *Deps) ListCampaignProfiles(w http.ResponseWriter, r *http.Request) {
	correlationID := middleware.GetCorrelationID(r.Context())
	campaignID := chi.URLParam(r, "id")

	dtos, err := d.Svc.ListCampaignProfiles(r.Context(), campaignID)
	if err != nil {
		response.Error(w, "INTERNAL_ERROR", "failed to list campaign SMTP profiles", http.StatusInternalServerError, correlationID)
		return
	}
	response.Success(w, dtos)
}

// UpdateCampaignAssociation handles PUT /api/v1/campaigns/{id}/smtp-profiles/{assocId}.
func (d *Deps) UpdateCampaignAssociation(w http.ResponseWriter, r *http.Request) {
	correlationID := middleware.GetCorrelationID(r.Context())
	claims := middleware.ClaimsFromContext(r.Context())
	if claims == nil {
		response.Error(w, "UNAUTHORIZED", "authentication required", http.StatusUnauthorized, correlationID)
		return
	}
	assocID := chi.URLParam(r, "assocId")

	var body struct {
		Priority      *int    `json:"priority,omitempty"`
		Weight        *int    `json:"weight,omitempty"`
		FromAddress   *string `json:"from_address_override,omitempty"`
		FromName      *string `json:"from_name_override,omitempty"`
		ReplyTo       *string `json:"reply_to_override,omitempty"`
		SegmentFilter []byte  `json:"segment_filter,omitempty"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		response.Error(w, "BAD_REQUEST", "invalid request body", http.StatusBadRequest, correlationID)
		return
	}

	dto, err := d.Svc.UpdateCampaignAssociation(r.Context(), assocID,
		body.Priority, body.Weight, body.FromAddress, body.FromName, body.ReplyTo, body.SegmentFilter,
		claims.Subject, claims.Username, clientIP(r), correlationID)
	if err != nil {
		writeSMTPError(w, err, correlationID)
		return
	}
	response.Success(w, dto)
}

// RemoveCampaignAssociation handles DELETE /api/v1/campaigns/{id}/smtp-profiles/{assocId}.
func (d *Deps) RemoveCampaignAssociation(w http.ResponseWriter, r *http.Request) {
	correlationID := middleware.GetCorrelationID(r.Context())
	claims := middleware.ClaimsFromContext(r.Context())
	if claims == nil {
		response.Error(w, "UNAUTHORIZED", "authentication required", http.StatusUnauthorized, correlationID)
		return
	}
	assocID := chi.URLParam(r, "assocId")

	if err := d.Svc.RemoveCampaignAssociation(r.Context(), assocID, claims.Subject, claims.Username, clientIP(r), correlationID); err != nil {
		writeSMTPError(w, err, correlationID)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// UpsertSendSchedule handles PUT /api/v1/campaigns/{id}/send-schedule.
func (d *Deps) UpsertSendSchedule(w http.ResponseWriter, r *http.Request) {
	correlationID := middleware.GetCorrelationID(r.Context())
	claims := middleware.ClaimsFromContext(r.Context())
	if claims == nil {
		response.Error(w, "UNAUTHORIZED", "authentication required", http.StatusUnauthorized, correlationID)
		return
	}
	campaignID := chi.URLParam(r, "id")

	var input smtpsvc.SendScheduleInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		response.Error(w, "BAD_REQUEST", "invalid request body", http.StatusBadRequest, correlationID)
		return
	}

	dto, err := d.Svc.UpsertSendSchedule(r.Context(), campaignID, input, claims.Subject, claims.Username, clientIP(r), correlationID)
	if err != nil {
		writeSMTPError(w, err, correlationID)
		return
	}
	response.Success(w, dto)
}

// GetSendSchedule handles GET /api/v1/campaigns/{id}/send-schedule.
func (d *Deps) GetSendSchedule(w http.ResponseWriter, r *http.Request) {
	correlationID := middleware.GetCorrelationID(r.Context())
	campaignID := chi.URLParam(r, "id")

	dto, err := d.Svc.GetSendSchedule(r.Context(), campaignID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			response.Error(w, "NOT_FOUND", "send schedule not configured", http.StatusNotFound, correlationID)
			return
		}
		response.Error(w, "INTERNAL_ERROR", "failed to get send schedule", http.StatusInternalServerError, correlationID)
		return
	}
	response.Success(w, dto)
}

// ValidateCampaignProfiles handles POST /api/v1/campaigns/{id}/smtp-profiles/validate.
func (d *Deps) ValidateCampaignProfiles(w http.ResponseWriter, r *http.Request) {
	correlationID := middleware.GetCorrelationID(r.Context())
	campaignID := chi.URLParam(r, "id")

	results, err := d.Svc.ValidateCampaignProfiles(r.Context(), campaignID)
	if err != nil {
		response.Error(w, "INTERNAL_ERROR", "validation failed", http.StatusInternalServerError, correlationID)
		return
	}
	response.Success(w, results)
}

// --- helpers ---

// validateSMTPInput performs field-level validation on SMTP profile input.
func validateSMTPInput(input smtpsvc.CreateInput) []response.FieldError {
	var errs []response.FieldError
	if strings.TrimSpace(input.Name) == "" {
		errs = append(errs, response.FieldError{Field: "name", Message: "name is required", Code: "required"})
	}
	if strings.TrimSpace(input.Host) == "" {
		errs = append(errs, response.FieldError{Field: "host", Message: "host is required", Code: "required"})
	}
	if input.Port < 1 || input.Port > 65535 {
		errs = append(errs, response.FieldError{Field: "port", Message: "port must be between 1 and 65535", Code: "invalid_value"})
	}
	if strings.TrimSpace(input.FromAddress) == "" {
		errs = append(errs, response.FieldError{Field: "from_address", Message: "from_address is required", Code: "required"})
	}
	return errs
}

func writeSMTPError(w http.ResponseWriter, err error, correlationID string) {
	var ve *smtpsvc.ValidationError
	if errors.As(err, &ve) {
		response.Error(w, "VALIDATION_ERROR", ve.Error(), http.StatusBadRequest, correlationID)
		return
	}
	var ce *smtpsvc.ConflictError
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
