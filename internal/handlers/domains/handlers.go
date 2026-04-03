package domains

import (
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"

	"tackle/internal/middleware"
	domainsvc "tackle/internal/services/domain"
	"tackle/pkg/response"
)

// Create handles POST /api/v1/domains — register or import a domain.
// The action is determined by the "action" field: "register" or "import".
func (d *Deps) Create(w http.ResponseWriter, r *http.Request) {
	correlationID := middleware.GetCorrelationID(r.Context())
	claims := middleware.ClaimsFromContext(r.Context())
	if claims == nil {
		response.Error(w, "UNAUTHORIZED", "authentication required", http.StatusUnauthorized, correlationID)
		return
	}

	var body struct {
		Action string `json:"action"` // "register" or "import"

		// Register fields.
		RegistrarConnectionID   string                  `json:"registrar_connection_id"`
		DNSProviderConnectionID string                  `json:"dns_provider_connection_id"`
		Domain                  string                  `json:"domain"`
		Years                   int                     `json:"years"`
		Registrant              domainsvc.RegistrantInfo `json:"registrant"`
		Tags                    []string                `json:"tags"`
		Notes                   string                  `json:"notes"`

		// Import-specific fields.
		RegistrationDate *string `json:"registration_date"`
		ExpiryDate       *string `json:"expiry_date"`
		SyncExpiry       bool    `json:"sync_expiry"`
	}

	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		response.Error(w, "BAD_REQUEST", "invalid request body", http.StatusBadRequest, correlationID)
		return
	}

	switch body.Action {
	case "register":
		input := domainsvc.RegisterDomainInput{
			RegistrarConnectionID:   body.RegistrarConnectionID,
			DNSProviderConnectionID: body.DNSProviderConnectionID,
			Domain:                  body.Domain,
			Years:                   body.Years,
			Registrant:              body.Registrant,
			Tags:                    body.Tags,
			Notes:                   body.Notes,
		}
		// Determine actor role from claims (Engineer bypass).
		role := claims.Role
		dto, pending, err := d.Svc.RegisterDomain(r.Context(), input, claims.Subject, claims.Username, role, clientIP(r), correlationID)
		if err != nil {
			writeServiceError(w, err, correlationID)
			return
		}
		if pending {
			response.Success(w, map[string]any{"status": "pending_approval", "domain": dto.DomainName})
		} else {
			response.Created(w, dto)
		}

	case "import":
		input := domainsvc.ImportDomainInput{
			DomainName:              body.Domain,
			RegistrarConnectionID:   body.RegistrarConnectionID,
			DNSProviderConnectionID: body.DNSProviderConnectionID,
			Tags:                    body.Tags,
			Notes:                   body.Notes,
			SyncExpiry:              body.SyncExpiry,
		}
		if body.RegistrationDate != nil {
			t, err := time.Parse("2006-01-02", *body.RegistrationDate)
			if err == nil {
				input.RegistrationDate = &t
			}
		}
		if body.ExpiryDate != nil {
			t, err := time.Parse("2006-01-02", *body.ExpiryDate)
			if err == nil {
				input.ExpiryDate = &t
			}
		}

		dto, err := d.Svc.ImportDomain(r.Context(), input, claims.Subject, claims.Username, clientIP(r), correlationID)
		if err != nil {
			writeServiceError(w, err, correlationID)
			return
		}
		response.Created(w, dto)

	default:
		response.Error(w, "BAD_REQUEST", `"action" must be "register" or "import"`, http.StatusBadRequest, correlationID)
	}
}

// List handles GET /api/v1/domains.
func (d *Deps) List(w http.ResponseWriter, r *http.Request) {
	correlationID := middleware.GetCorrelationID(r.Context())

	q := r.URL.Query()
	input := domainsvc.ListDomainsInput{
		Status:                  q.Get("status"),
		RegistrarConnectionID:   q.Get("registrar_connection_id"),
		DNSProviderConnectionID: q.Get("dns_provider_connection_id"),
		Tag:                     q.Get("tag"),
		CampaignID:              q.Get("campaign_id"),
		Search:                  q.Get("search"),
		SortField:               q.Get("sort"),
		SortDesc:                q.Get("order") == "desc",
	}

	if v := q.Get("expiry_before"); v != "" {
		if t, err := time.Parse("2006-01-02", v); err == nil {
			input.ExpiryBefore = &t
		}
	}
	if v := q.Get("expiry_after"); v != "" {
		if t, err := time.Parse("2006-01-02", v); err == nil {
			input.ExpiryAfter = &t
		}
	}

	input.Limit, _ = strconv.Atoi(q.Get("limit"))
	input.Offset, _ = strconv.Atoi(q.Get("offset"))

	result, err := d.Svc.ListDomainProfiles(r.Context(), input)
	if err != nil {
		response.Error(w, "INTERNAL_ERROR", "failed to list domains", http.StatusInternalServerError, correlationID)
		return
	}

	response.Success(w, result)
}

// Get handles GET /api/v1/domains/{id}.
func (d *Deps) Get(w http.ResponseWriter, r *http.Request) {
	correlationID := middleware.GetCorrelationID(r.Context())
	id := chi.URLParam(r, "id")

	dto, err := d.Svc.GetDomainProfile(r.Context(), id)
	if err != nil {
		if isNotFound(err) {
			response.Error(w, "NOT_FOUND", "domain not found", http.StatusNotFound, correlationID)
			return
		}
		response.Error(w, "INTERNAL_ERROR", "failed to get domain", http.StatusInternalServerError, correlationID)
		return
	}

	response.Success(w, dto)
}

// Update handles PUT /api/v1/domains/{id}.
func (d *Deps) Update(w http.ResponseWriter, r *http.Request) {
	correlationID := middleware.GetCorrelationID(r.Context())
	claims := middleware.ClaimsFromContext(r.Context())
	if claims == nil {
		response.Error(w, "UNAUTHORIZED", "authentication required", http.StatusUnauthorized, correlationID)
		return
	}
	id := chi.URLParam(r, "id")

	var input domainsvc.UpdateDomainInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		response.Error(w, "BAD_REQUEST", "invalid request body", http.StatusBadRequest, correlationID)
		return
	}

	dto, err := d.Svc.UpdateDomainProfile(r.Context(), id, input, claims.Subject, claims.Username, clientIP(r), correlationID)
	if err != nil {
		if isNotFound(err) {
			response.Error(w, "NOT_FOUND", "domain not found", http.StatusNotFound, correlationID)
			return
		}
		writeServiceError(w, err, correlationID)
		return
	}

	response.Success(w, dto)
}

// Delete handles DELETE /api/v1/domains/{id} — soft-deletes (decommissions) a domain.
func (d *Deps) Delete(w http.ResponseWriter, r *http.Request) {
	correlationID := middleware.GetCorrelationID(r.Context())
	claims := middleware.ClaimsFromContext(r.Context())
	if claims == nil {
		response.Error(w, "UNAUTHORIZED", "authentication required", http.StatusUnauthorized, correlationID)
		return
	}
	id := chi.URLParam(r, "id")

	err := d.Svc.DecommissionDomain(r.Context(), id, claims.Subject, claims.Username, clientIP(r), correlationID)
	if err != nil {
		if isNotFound(err) {
			response.Error(w, "NOT_FOUND", "domain not found", http.StatusNotFound, correlationID)
			return
		}
		writeServiceError(w, err, correlationID)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// CheckAvailability handles POST /api/v1/domains/check-availability.
func (d *Deps) CheckAvailability(w http.ResponseWriter, r *http.Request) {
	correlationID := middleware.GetCorrelationID(r.Context())

	var body domainsvc.CheckAvailabilityInput
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		response.Error(w, "BAD_REQUEST", "invalid request body", http.StatusBadRequest, correlationID)
		return
	}

	result, err := d.Svc.CheckAvailability(r.Context(), body)
	if err != nil {
		writeServiceError(w, err, correlationID)
		return
	}

	response.Success(w, result)
}

// Renew handles POST /api/v1/domains/{id}/renew.
func (d *Deps) Renew(w http.ResponseWriter, r *http.Request) {
	correlationID := middleware.GetCorrelationID(r.Context())
	claims := middleware.ClaimsFromContext(r.Context())
	if claims == nil {
		response.Error(w, "UNAUTHORIZED", "authentication required", http.StatusUnauthorized, correlationID)
		return
	}
	id := chi.URLParam(r, "id")

	var body struct {
		Years int `json:"years"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.Years < 1 {
		response.Error(w, "BAD_REQUEST", `"years" is required and must be >= 1`, http.StatusBadRequest, correlationID)
		return
	}

	rec, err := d.Svc.RenewDomain(r.Context(), id, body.Years, claims.Subject, claims.Username, clientIP(r), correlationID)
	if err != nil {
		if isNotFound(err) {
			response.Error(w, "NOT_FOUND", "domain not found", http.StatusNotFound, correlationID)
			return
		}
		writeServiceError(w, err, correlationID)
		return
	}

	response.Success(w, rec)
}

// RenewalHistory handles GET /api/v1/domains/{id}/renewal-history.
func (d *Deps) RenewalHistory(w http.ResponseWriter, r *http.Request) {
	correlationID := middleware.GetCorrelationID(r.Context())
	id := chi.URLParam(r, "id")

	records, err := d.Svc.GetRenewalHistory(r.Context(), id)
	if err != nil {
		if isNotFound(err) {
			response.Error(w, "NOT_FOUND", "domain not found", http.StatusNotFound, correlationID)
			return
		}
		response.Error(w, "INTERNAL_ERROR", "failed to get renewal history", http.StatusInternalServerError, correlationID)
		return
	}

	response.Success(w, records)
}

// SubmitRegistrationRequest handles POST /api/v1/domains/registration-requests.
// Creates a pending registration request for non-Engineer users who cannot directly register.
func (d *Deps) SubmitRegistrationRequest(w http.ResponseWriter, r *http.Request) {
	correlationID := middleware.GetCorrelationID(r.Context())
	claims := middleware.ClaimsFromContext(r.Context())
	if claims == nil {
		response.Error(w, "UNAUTHORIZED", "authentication required", http.StatusUnauthorized, correlationID)
		return
	}

	var input domainsvc.RegisterDomainInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		response.Error(w, "BAD_REQUEST", "invalid request body", http.StatusBadRequest, correlationID)
		return
	}

	// Force through approval flow by passing a non-Engineer role.
	_, _, err := d.Svc.RegisterDomain(r.Context(), input, claims.Subject, claims.Username, "Operator", clientIP(r), correlationID)
	if err != nil {
		writeServiceError(w, err, correlationID)
		return
	}

	response.Success(w, map[string]string{"status": "pending_approval"})
}

// ApproveRegistration handles POST /api/v1/domains/registration-requests/{id}/approve.
func (d *Deps) ApproveRegistration(w http.ResponseWriter, r *http.Request) {
	correlationID := middleware.GetCorrelationID(r.Context())
	claims := middleware.ClaimsFromContext(r.Context())
	if claims == nil {
		response.Error(w, "UNAUTHORIZED", "authentication required", http.StatusUnauthorized, correlationID)
		return
	}
	id := chi.URLParam(r, "id")

	dto, err := d.Svc.ApproveRegistration(r.Context(), id, claims.Subject, claims.Username, clientIP(r), correlationID)
	if err != nil {
		if isNotFound(err) {
			response.Error(w, "NOT_FOUND", "registration request not found", http.StatusNotFound, correlationID)
			return
		}
		writeServiceError(w, err, correlationID)
		return
	}

	response.Created(w, dto)
}

// RejectRegistration handles POST /api/v1/domains/registration-requests/{id}/reject.
func (d *Deps) RejectRegistration(w http.ResponseWriter, r *http.Request) {
	correlationID := middleware.GetCorrelationID(r.Context())
	claims := middleware.ClaimsFromContext(r.Context())
	if claims == nil {
		response.Error(w, "UNAUTHORIZED", "authentication required", http.StatusUnauthorized, correlationID)
		return
	}
	id := chi.URLParam(r, "id")

	var body struct {
		Reason string `json:"reason"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.Reason == "" {
		response.Error(w, "BAD_REQUEST", `"reason" is required`, http.StatusBadRequest, correlationID)
		return
	}

	err := d.Svc.RejectRegistration(r.Context(), id, body.Reason, claims.Subject, claims.Username, clientIP(r), correlationID)
	if err != nil {
		if isNotFound(err) {
			response.Error(w, "NOT_FOUND", "registration request not found", http.StatusNotFound, correlationID)
			return
		}
		writeServiceError(w, err, correlationID)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// --- helpers ---

func writeServiceError(w http.ResponseWriter, err error, correlationID string) {
	var valErr *domainsvc.ValidationError
	if errors.As(err, &valErr) {
		response.Error(w, "VALIDATION_ERROR", valErr.Error(), http.StatusBadRequest, correlationID)
		return
	}
	var conflictErr *domainsvc.ConflictError
	if errors.As(err, &conflictErr) {
		response.Error(w, "CONFLICT", conflictErr.Error(), http.StatusConflict, correlationID)
		return
	}
	if errors.Is(err, sql.ErrNoRows) {
		response.Error(w, "NOT_FOUND", "resource not found", http.StatusNotFound, correlationID)
		return
	}
	response.Error(w, "INTERNAL_ERROR", "internal server error", http.StatusInternalServerError, correlationID)
}

func isNotFound(err error) bool {
	if err == nil {
		return false
	}
	return strings.Contains(err.Error(), "not found") || errors.Is(err, sql.ErrNoRows)
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
