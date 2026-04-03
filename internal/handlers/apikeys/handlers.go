// Package apikeys provides HTTP handlers for API key management endpoints.
package apikeys

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"

	"tackle/internal/middleware"
	"tackle/internal/services/apikey"
	"tackle/internal/services/audit"
	"tackle/pkg/response"
)

// Deps holds shared dependencies for API key handlers.
type Deps struct {
	Svc      *apikey.Service
	AuditSvc *audit.AuditService
}

type createKeyRequest struct {
	Name      string  `json:"name"`
	ExpiresIn *int    `json:"expires_in_days"`
}

type keyResponse struct {
	ID         string     `json:"id"`
	Name       string     `json:"name"`
	Prefix     string     `json:"prefix"`
	ExpiresAt  *time.Time `json:"expires_at"`
	LastUsedAt *time.Time `json:"last_used_at"`
	Revoked    bool       `json:"revoked"`
	CreatedAt  time.Time  `json:"created_at"`
}

type createKeyResponse struct {
	keyResponse
	RawKey  string `json:"raw_key"`
	Warning string `json:"warning"`
}

// Create handles POST /api/v1/api-keys — generates a new API key.
func (d *Deps) Create(w http.ResponseWriter, r *http.Request) {
	correlationID := middleware.GetCorrelationID(r.Context())
	claims := middleware.ClaimsFromContext(r.Context())
	if claims == nil {
		response.Error(w, "UNAUTHORIZED", "authentication required", http.StatusUnauthorized, correlationID)
		return
	}

	var req createKeyRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.Error(w, "BAD_REQUEST", "invalid request body", http.StatusBadRequest, correlationID)
		return
	}
	if req.Name == "" {
		response.Error(w, "VALIDATION_ERROR", "name is required", http.StatusBadRequest, correlationID)
		return
	}

	var expiresAt *time.Time
	if req.ExpiresIn != nil && *req.ExpiresIn > 0 {
		t := time.Now().Add(time.Duration(*req.ExpiresIn) * 24 * time.Hour)
		expiresAt = &t
	}

	result, err := d.Svc.Create(r.Context(), claims.Subject, req.Name, expiresAt)
	if err != nil {
		response.Error(w, "INTERNAL_ERROR", "failed to create API key", http.StatusInternalServerError, correlationID)
		return
	}

	if d.AuditSvc != nil {
		actorID := claims.Subject
		resType := "api_key"
		_ = d.AuditSvc.Log(r.Context(), audit.LogEntry{
			Category:      audit.CategoryUserActivity,
			Severity:      audit.SeverityInfo,
			ActorType:     audit.ActorTypeUser,
			ActorID:       &actorID,
			ActorLabel:    claims.Username,
			Action:        "api_key.created",
			ResourceType:  &resType,
			ResourceID:    &result.Key.ID,
			CorrelationID: correlationID,
			Details:       map[string]any{"name": req.Name},
		})
	}

	response.Created(w, createKeyResponse{
		keyResponse: toKeyResponse(result.Key),
		RawKey:      result.RawKey,
		Warning:     "This key will not be shown again. Store it securely.",
	})
}

// List handles GET /api/v1/api-keys — returns all keys for the authenticated user.
func (d *Deps) List(w http.ResponseWriter, r *http.Request) {
	correlationID := middleware.GetCorrelationID(r.Context())
	claims := middleware.ClaimsFromContext(r.Context())
	if claims == nil {
		response.Error(w, "UNAUTHORIZED", "authentication required", http.StatusUnauthorized, correlationID)
		return
	}

	keys, err := d.Svc.List(r.Context(), claims.Subject)
	if err != nil {
		response.Error(w, "INTERNAL_ERROR", "failed to list API keys", http.StatusInternalServerError, correlationID)
		return
	}

	resp := make([]keyResponse, 0, len(keys))
	for _, k := range keys {
		resp = append(resp, toKeyResponse(k))
	}
	response.Success(w, resp)
}

// Revoke handles DELETE /api/v1/api-keys/{id} — revokes an API key.
func (d *Deps) Revoke(w http.ResponseWriter, r *http.Request) {
	correlationID := middleware.GetCorrelationID(r.Context())
	claims := middleware.ClaimsFromContext(r.Context())
	if claims == nil {
		response.Error(w, "UNAUTHORIZED", "authentication required", http.StatusUnauthorized, correlationID)
		return
	}

	keyID := chi.URLParam(r, "id")

	// Verify the key belongs to the authenticated user.
	key, err := d.Svc.Get(r.Context(), keyID)
	if err != nil {
		response.Error(w, "NOT_FOUND", "API key not found", http.StatusNotFound, correlationID)
		return
	}
	if key.UserID != claims.Subject {
		response.Error(w, "FORBIDDEN", "not authorized to revoke this key", http.StatusForbidden, correlationID)
		return
	}

	if err := d.Svc.Revoke(r.Context(), keyID); err != nil {
		response.Error(w, "INTERNAL_ERROR", "failed to revoke API key", http.StatusInternalServerError, correlationID)
		return
	}

	if d.AuditSvc != nil {
		actorID := claims.Subject
		resType := "api_key"
		_ = d.AuditSvc.Log(r.Context(), audit.LogEntry{
			Category:      audit.CategoryUserActivity,
			Severity:      audit.SeverityInfo,
			ActorType:     audit.ActorTypeUser,
			ActorID:       &actorID,
			ActorLabel:    claims.Username,
			Action:        "api_key.revoked",
			ResourceType:  &resType,
			ResourceID:    &keyID,
			CorrelationID: correlationID,
		})
	}

	w.WriteHeader(http.StatusNoContent)
}

func toKeyResponse(k apikey.APIKey) keyResponse {
	return keyResponse{
		ID:         k.ID,
		Name:       k.Name,
		Prefix:     k.KeyPrefix,
		ExpiresAt:  k.ExpiresAt,
		LastUsedAt: k.LastUsedAt,
		Revoked:    k.Revoked,
		CreatedAt:  k.CreatedAt,
	}
}
