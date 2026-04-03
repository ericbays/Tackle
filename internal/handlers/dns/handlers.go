package dns

import (
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"

	"tackle/internal/middleware"
	dnsiface "tackle/internal/providers/dns"
	dnssvc "tackle/internal/services/dns"
	"tackle/pkg/response"
)

// ListRecords handles GET /api/v1/domains/{id}/dns-records.
func (d *Deps) ListRecords(w http.ResponseWriter, r *http.Request) {
	correlationID := middleware.GetCorrelationID(r.Context())
	domainID := chi.URLParam(r, "id")

	records, err := d.Svc.ListRecords(r.Context(), domainID)
	if err != nil {
		writeServiceError(w, err, correlationID)
		return
	}
	response.Success(w, records)
}

// CreateRecord handles POST /api/v1/domains/{id}/dns-records.
func (d *Deps) CreateRecord(w http.ResponseWriter, r *http.Request) {
	correlationID := middleware.GetCorrelationID(r.Context())
	claims := middleware.ClaimsFromContext(r.Context())
	if claims == nil {
		response.Error(w, "UNAUTHORIZED", "authentication required", http.StatusUnauthorized, correlationID)
		return
	}
	domainID := chi.URLParam(r, "id")

	var body struct {
		Type     string `json:"type"`
		Name     string `json:"name"`
		Value    string `json:"value"`
		TTL      int    `json:"ttl"`
		Priority int    `json:"priority"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		response.Error(w, "BAD_REQUEST", "invalid request body", http.StatusBadRequest, correlationID)
		return
	}

	rType, err := dnsiface.RecordTypeFromString(strings.ToUpper(body.Type))
	if err != nil {
		response.Error(w, "VALIDATION_ERROR", err.Error(), http.StatusBadRequest, correlationID)
		return
	}

	record := dnsiface.Record{
		Type:     rType,
		Name:     body.Name,
		Value:    body.Value,
		TTL:      body.TTL,
		Priority: body.Priority,
	}

	dto, err := d.Svc.CreateRecord(r.Context(), domainID, record, claims.Subject, claims.Username, clientIP(r), correlationID)
	if err != nil {
		writeServiceError(w, err, correlationID)
		return
	}
	response.Created(w, dto)
}

// UpdateRecord handles PUT /api/v1/domains/{id}/dns-records/{recordId}.
func (d *Deps) UpdateRecord(w http.ResponseWriter, r *http.Request) {
	correlationID := middleware.GetCorrelationID(r.Context())
	claims := middleware.ClaimsFromContext(r.Context())
	if claims == nil {
		response.Error(w, "UNAUTHORIZED", "authentication required", http.StatusUnauthorized, correlationID)
		return
	}
	domainID := chi.URLParam(r, "id")
	recordID := chi.URLParam(r, "recordId")

	var body struct {
		Type     string `json:"type"`
		Name     string `json:"name"`
		Value    string `json:"value"`
		TTL      int    `json:"ttl"`
		Priority int    `json:"priority"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		response.Error(w, "BAD_REQUEST", "invalid request body", http.StatusBadRequest, correlationID)
		return
	}

	rType, err := dnsiface.RecordTypeFromString(strings.ToUpper(body.Type))
	if err != nil {
		response.Error(w, "VALIDATION_ERROR", err.Error(), http.StatusBadRequest, correlationID)
		return
	}

	record := dnsiface.Record{
		Type:     rType,
		Name:     body.Name,
		Value:    body.Value,
		TTL:      body.TTL,
		Priority: body.Priority,
	}

	dto, err := d.Svc.UpdateRecord(r.Context(), domainID, recordID, record, claims.Subject, claims.Username, clientIP(r), correlationID)
	if err != nil {
		writeServiceError(w, err, correlationID)
		return
	}
	response.Success(w, dto)
}

// DeleteRecord handles DELETE /api/v1/domains/{id}/dns-records/{recordId}.
func (d *Deps) DeleteRecord(w http.ResponseWriter, r *http.Request) {
	correlationID := middleware.GetCorrelationID(r.Context())
	claims := middleware.ClaimsFromContext(r.Context())
	if claims == nil {
		response.Error(w, "UNAUTHORIZED", "authentication required", http.StatusUnauthorized, correlationID)
		return
	}
	domainID := chi.URLParam(r, "id")
	recordID := chi.URLParam(r, "recordId")

	if err := d.Svc.DeleteRecord(r.Context(), domainID, recordID, claims.Subject, claims.Username, clientIP(r), correlationID); err != nil {
		writeServiceError(w, err, correlationID)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// GetSOA handles GET /api/v1/domains/{id}/dns-records/soa.
func (d *Deps) GetSOA(w http.ResponseWriter, r *http.Request) {
	correlationID := middleware.GetCorrelationID(r.Context())
	domainID := chi.URLParam(r, "id")

	soa, err := d.Svc.GetSOA(r.Context(), domainID)
	if err != nil {
		writeServiceError(w, err, correlationID)
		return
	}
	response.Success(w, soa)
}

// GetEmailAuth handles GET /api/v1/domains/{id}/email-auth.
func (d *Deps) GetEmailAuth(w http.ResponseWriter, r *http.Request) {
	correlationID := middleware.GetCorrelationID(r.Context())
	domainID := chi.URLParam(r, "id")

	status, err := d.Svc.GetEmailAuthStatus(r.Context(), domainID)
	if err != nil {
		writeServiceError(w, err, correlationID)
		return
	}
	response.Success(w, status)
}

// ConfigureSPF handles POST /api/v1/domains/{id}/email-auth/spf.
func (d *Deps) ConfigureSPF(w http.ResponseWriter, r *http.Request) {
	correlationID := middleware.GetCorrelationID(r.Context())
	claims := middleware.ClaimsFromContext(r.Context())
	if claims == nil {
		response.Error(w, "UNAUTHORIZED", "authentication required", http.StatusUnauthorized, correlationID)
		return
	}
	domainID := chi.URLParam(r, "id")

	var cfg dnssvc.SPFConfig
	if err := json.NewDecoder(r.Body).Decode(&cfg); err != nil {
		response.Error(w, "BAD_REQUEST", "invalid request body", http.StatusBadRequest, correlationID)
		return
	}

	dto, err := d.Svc.ConfigureSPF(r.Context(), domainID, cfg, claims.Subject, claims.Username, clientIP(r), correlationID)
	if err != nil {
		writeServiceError(w, err, correlationID)
		return
	}
	response.Created(w, dto)
}

// GenerateDKIM handles POST /api/v1/domains/{id}/email-auth/dkim.
func (d *Deps) GenerateDKIM(w http.ResponseWriter, r *http.Request) {
	correlationID := middleware.GetCorrelationID(r.Context())
	claims := middleware.ClaimsFromContext(r.Context())
	if claims == nil {
		response.Error(w, "UNAUTHORIZED", "authentication required", http.StatusUnauthorized, correlationID)
		return
	}
	domainID := chi.URLParam(r, "id")

	var body struct {
		Selector  string `json:"selector"`
		Algorithm string `json:"algorithm"` // rsa-sha256 | ed25519-sha256
		KeySize   int    `json:"key_size"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		response.Error(w, "BAD_REQUEST", "invalid request body", http.StatusBadRequest, correlationID)
		return
	}
	if body.Selector == "" {
		response.Error(w, "VALIDATION_ERROR", "selector is required", http.StatusBadRequest, correlationID)
		return
	}

	algo := dnssvc.DKIMAlgorithm(body.Algorithm)
	if algo == "" {
		algo = dnssvc.DKIMAlgorithmRSA
	}

	dto, err := d.Svc.GenerateDKIM(r.Context(), domainID, body.Selector, algo, body.KeySize, claims.Subject, claims.Username, clientIP(r), correlationID)
	if err != nil {
		writeServiceError(w, err, correlationID)
		return
	}
	response.Created(w, dto)
}

// ConfigureDMARC handles POST /api/v1/domains/{id}/email-auth/dmarc.
func (d *Deps) ConfigureDMARC(w http.ResponseWriter, r *http.Request) {
	correlationID := middleware.GetCorrelationID(r.Context())
	claims := middleware.ClaimsFromContext(r.Context())
	if claims == nil {
		response.Error(w, "UNAUTHORIZED", "authentication required", http.StatusUnauthorized, correlationID)
		return
	}
	domainID := chi.URLParam(r, "id")

	var cfg dnssvc.DMARCConfig
	if err := json.NewDecoder(r.Body).Decode(&cfg); err != nil {
		response.Error(w, "BAD_REQUEST", "invalid request body", http.StatusBadRequest, correlationID)
		return
	}

	dto, err := d.Svc.ConfigureDMARC(r.Context(), domainID, cfg, claims.Subject, claims.Username, clientIP(r), correlationID)
	if err != nil {
		writeServiceError(w, err, correlationID)
		return
	}
	response.Created(w, dto)
}

// ValidateEmailAuth handles POST /api/v1/domains/{id}/email-auth/validate.
func (d *Deps) ValidateEmailAuth(w http.ResponseWriter, r *http.Request) {
	correlationID := middleware.GetCorrelationID(r.Context())
	claims := middleware.ClaimsFromContext(r.Context())
	if claims == nil {
		response.Error(w, "UNAUTHORIZED", "authentication required", http.StatusUnauthorized, correlationID)
		return
	}
	domainID := chi.URLParam(r, "id")

	result, err := d.Svc.ValidateEmailAuth(r.Context(), domainID, claims.Subject, claims.Username, clientIP(r), correlationID)
	if err != nil {
		writeServiceError(w, err, correlationID)
		return
	}
	response.Success(w, result)
}

// GetPropagationChecks handles GET /api/v1/domains/{id}/propagation-checks.
func (d *Deps) GetPropagationChecks(w http.ResponseWriter, r *http.Request) {
	correlationID := middleware.GetCorrelationID(r.Context())
	domainID := chi.URLParam(r, "id")

	limitStr := r.URL.Query().Get("limit")
	limit := 20
	if limitStr != "" {
		if n, err := strconv.Atoi(limitStr); err == nil && n > 0 {
			limit = n
		}
	}

	checks, err := d.Svc.GetPropagationChecks(r.Context(), domainID, limit)
	if err != nil {
		writeServiceError(w, err, correlationID)
		return
	}
	response.Success(w, checks)
}

// --- internal helpers ---

func writeServiceError(w http.ResponseWriter, err error, correlationID string) {
	var valErr *dnssvc.ValidationError
	if errors.As(err, &valErr) {
		response.Error(w, "VALIDATION_ERROR", valErr.Error(), http.StatusBadRequest, correlationID)
		return
	}
	if errors.Is(err, dnsiface.ErrRecordNotFound) || errors.Is(err, sql.ErrNoRows) {
		response.Error(w, "NOT_FOUND", "resource not found", http.StatusNotFound, correlationID)
		return
	}
	if errors.Is(err, dnsiface.ErrZoneNotFound) {
		response.Error(w, "NOT_FOUND", "DNS zone not found on provider", http.StatusNotFound, correlationID)
		return
	}
	if errors.Is(err, dnsiface.ErrRecordConflict) {
		response.Error(w, "CONFLICT", "DNS record already exists", http.StatusConflict, correlationID)
		return
	}
	if errors.Is(err, dnsiface.ErrAPIRateLimit) {
		response.Error(w, "RATE_LIMITED", "provider API rate limit exceeded", http.StatusTooManyRequests, correlationID)
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
