package domainproviders

import (
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"

	"tackle/internal/middleware"
	"tackle/internal/services/domainprovider"
	"tackle/pkg/response"
)

// Create handles POST /api/v1/settings/domain-providers.
func (d *Deps) Create(w http.ResponseWriter, r *http.Request) {
	correlationID := middleware.GetCorrelationID(r.Context())
	claims := middleware.ClaimsFromContext(r.Context())
	if claims == nil {
		response.Error(w, "UNAUTHORIZED", "authentication required", http.StatusUnauthorized, correlationID)
		return
	}

	var input domainprovider.CreateInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		response.Error(w, "BAD_REQUEST", "invalid request body", http.StatusBadRequest, correlationID)
		return
	}

	dto, err := d.Svc.CreateConnection(r.Context(), input, claims.Subject, claims.Username, clientIP(r), correlationID)
	if err != nil {
		writeServiceError(w, err, correlationID)
		return
	}

	response.Created(w, dto)
}

// List handles GET /api/v1/settings/domain-providers.
func (d *Deps) List(w http.ResponseWriter, r *http.Request) {
	correlationID := middleware.GetCorrelationID(r.Context())

	providerType := r.URL.Query().Get("provider_type")
	status := r.URL.Query().Get("status")

	dtos, err := d.Svc.ListConnections(r.Context(), providerType, status)
	if err != nil {
		response.Error(w, "INTERNAL_ERROR", "failed to list provider connections", http.StatusInternalServerError, correlationID)
		return
	}

	response.Success(w, dtos)
}

// Get handles GET /api/v1/settings/domain-providers/{id}.
func (d *Deps) Get(w http.ResponseWriter, r *http.Request) {
	correlationID := middleware.GetCorrelationID(r.Context())
	id := chi.URLParam(r, "id")

	dto, err := d.Svc.GetConnection(r.Context(), id)
	if err != nil {
		if isNotFound(err) {
			response.Error(w, "NOT_FOUND", "provider connection not found", http.StatusNotFound, correlationID)
			return
		}
		response.Error(w, "INTERNAL_ERROR", "failed to get provider connection", http.StatusInternalServerError, correlationID)
		return
	}

	response.Success(w, dto)
}

// Update handles PUT /api/v1/settings/domain-providers/{id}.
func (d *Deps) Update(w http.ResponseWriter, r *http.Request) {
	correlationID := middleware.GetCorrelationID(r.Context())
	claims := middleware.ClaimsFromContext(r.Context())
	if claims == nil {
		response.Error(w, "UNAUTHORIZED", "authentication required", http.StatusUnauthorized, correlationID)
		return
	}
	id := chi.URLParam(r, "id")

	var input domainprovider.UpdateInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		response.Error(w, "BAD_REQUEST", "invalid request body", http.StatusBadRequest, correlationID)
		return
	}

	dto, err := d.Svc.UpdateConnection(r.Context(), id, input, claims.Subject, claims.Username, clientIP(r), correlationID)
	if err != nil {
		if isNotFound(err) {
			response.Error(w, "NOT_FOUND", "provider connection not found", http.StatusNotFound, correlationID)
			return
		}
		writeServiceError(w, err, correlationID)
		return
	}

	response.Success(w, dto)
}

// Delete handles DELETE /api/v1/settings/domain-providers/{id}.
func (d *Deps) Delete(w http.ResponseWriter, r *http.Request) {
	correlationID := middleware.GetCorrelationID(r.Context())
	claims := middleware.ClaimsFromContext(r.Context())
	if claims == nil {
		response.Error(w, "UNAUTHORIZED", "authentication required", http.StatusUnauthorized, correlationID)
		return
	}
	id := chi.URLParam(r, "id")

	err := d.Svc.DeleteConnection(r.Context(), id, claims.Subject, claims.Username, clientIP(r), correlationID)
	if err != nil {
		if isNotFound(err) {
			response.Error(w, "NOT_FOUND", "provider connection not found", http.StatusNotFound, correlationID)
			return
		}
		writeServiceError(w, err, correlationID)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// TestConn handles POST /api/v1/settings/domain-providers/{id}/test.
func (d *Deps) TestConn(w http.ResponseWriter, r *http.Request) {
	correlationID := middleware.GetCorrelationID(r.Context())
	claims := middleware.ClaimsFromContext(r.Context())
	if claims == nil {
		response.Error(w, "UNAUTHORIZED", "authentication required", http.StatusUnauthorized, correlationID)
		return
	}
	id := chi.URLParam(r, "id")

	result, err := d.Svc.TestConnection(r.Context(), id, claims.Subject, claims.Username, clientIP(r), correlationID)
	if err != nil {
		if isNotFound(err) {
			response.Error(w, "NOT_FOUND", "provider connection not found", http.StatusNotFound, correlationID)
			return
		}
		response.Error(w, "INTERNAL_ERROR", "failed to test provider connection", http.StatusInternalServerError, correlationID)
		return
	}

	response.Success(w, result)
}

// writeServiceError maps service errors to appropriate HTTP responses.
func writeServiceError(w http.ResponseWriter, err error, correlationID string) {
	var valErr *domainprovider.ValidationError
	if errors.As(err, &valErr) {
		response.Error(w, "VALIDATION_ERROR", valErr.Error(), http.StatusBadRequest, correlationID)
		return
	}
	if errors.Is(err, sql.ErrNoRows) {
		response.Error(w, "NOT_FOUND", "resource not found", http.StatusNotFound, correlationID)
		return
	}
	response.Error(w, "INTERNAL_ERROR", "internal server error", http.StatusInternalServerError, correlationID)
}

// isNotFound returns true if the error indicates a missing resource.
func isNotFound(err error) bool {
	if err == nil {
		return false
	}
	return strings.Contains(err.Error(), "not found") || errors.Is(err, sql.ErrNoRows)
}

// clientIP extracts the client IP from the request, checking X-Forwarded-For first.
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
