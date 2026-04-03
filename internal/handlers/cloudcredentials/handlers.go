package cloudcredentials

import (
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"

	"tackle/internal/middleware"
	"tackle/internal/providers/credentials"
	credsvc "tackle/internal/services/cloudcredential"
	tmplsvc "tackle/internal/services/instancetemplate"
	"tackle/pkg/response"
)

// --- Cloud Credential Handlers ---

// CreateCredential handles POST /api/v1/settings/cloud-credentials.
func (d *Deps) CreateCredential(w http.ResponseWriter, r *http.Request) {
	correlationID := middleware.GetCorrelationID(r.Context())
	claims := middleware.ClaimsFromContext(r.Context())
	if claims == nil {
		response.Error(w, "UNAUTHORIZED", "authentication required", http.StatusUnauthorized, correlationID)
		return
	}

	var body struct {
		ProviderType  string          `json:"provider_type"`
		DisplayName   string          `json:"display_name"`
		DefaultRegion string          `json:"default_region"`
		AWS           *awsCredInput   `json:"aws,omitempty"`
		Azure         *azureCredInput `json:"azure,omitempty"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		response.Error(w, "BAD_REQUEST", "invalid request body", http.StatusBadRequest, correlationID)
		return
	}

	input := credsvc.CreateInput{
		ProviderType:  body.ProviderType,
		DisplayName:   body.DisplayName,
		DefaultRegion: body.DefaultRegion,
	}
	if body.AWS != nil {
		c := credentials.AWSCredentials{
			AccessKeyID:     body.AWS.AccessKeyID,
			SecretAccessKey: body.AWS.SecretAccessKey,
			IAMRoleARN:      body.AWS.IAMRoleARN,
		}
		input.AWSCreds = &c
	}
	if body.Azure != nil {
		c := credentials.AzureCredentials{
			TenantID:       body.Azure.TenantID,
			ClientID:       body.Azure.ClientID,
			ClientSecret:   body.Azure.ClientSecret,
			SubscriptionID: body.Azure.SubscriptionID,
		}
		input.AzureCreds = &c
	}

	dto, err := d.CredSvc.CreateCredential(r.Context(), input, claims.Subject, claims.Username, clientIP(r), correlationID)
	if err != nil {
		writeCredError(w, err, correlationID)
		return
	}
	response.Created(w, dto)
}

// ListCredentials handles GET /api/v1/settings/cloud-credentials.
func (d *Deps) ListCredentials(w http.ResponseWriter, r *http.Request) {
	correlationID := middleware.GetCorrelationID(r.Context())
	q := r.URL.Query()
	dtos, err := d.CredSvc.ListCredentials(r.Context(), q.Get("provider_type"), q.Get("status"))
	if err != nil {
		response.Error(w, "INTERNAL_ERROR", "failed to list cloud credentials", http.StatusInternalServerError, correlationID)
		return
	}
	response.Success(w, dtos)
}

// GetCredential handles GET /api/v1/settings/cloud-credentials/{id}.
func (d *Deps) GetCredential(w http.ResponseWriter, r *http.Request) {
	correlationID := middleware.GetCorrelationID(r.Context())
	id := chi.URLParam(r, "id")
	dto, err := d.CredSvc.GetCredential(r.Context(), id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			response.Error(w, "NOT_FOUND", "cloud credential not found", http.StatusNotFound, correlationID)
			return
		}
		response.Error(w, "INTERNAL_ERROR", "failed to get cloud credential", http.StatusInternalServerError, correlationID)
		return
	}
	response.Success(w, dto)
}

// UpdateCredential handles PUT /api/v1/settings/cloud-credentials/{id}.
func (d *Deps) UpdateCredential(w http.ResponseWriter, r *http.Request) {
	correlationID := middleware.GetCorrelationID(r.Context())
	claims := middleware.ClaimsFromContext(r.Context())
	if claims == nil {
		response.Error(w, "UNAUTHORIZED", "authentication required", http.StatusUnauthorized, correlationID)
		return
	}
	id := chi.URLParam(r, "id")

	var body struct {
		DisplayName   *string         `json:"display_name"`
		DefaultRegion *string         `json:"default_region"`
		AWS           *awsCredInput   `json:"aws,omitempty"`
		Azure         *azureCredInput `json:"azure,omitempty"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		response.Error(w, "BAD_REQUEST", "invalid request body", http.StatusBadRequest, correlationID)
		return
	}

	input := credsvc.UpdateInput{
		DisplayName:   body.DisplayName,
		DefaultRegion: body.DefaultRegion,
	}
	if body.AWS != nil {
		c := credentials.AWSCredentials{
			AccessKeyID:     body.AWS.AccessKeyID,
			SecretAccessKey: body.AWS.SecretAccessKey,
			IAMRoleARN:      body.AWS.IAMRoleARN,
		}
		input.AWSCreds = &c
	}
	if body.Azure != nil {
		c := credentials.AzureCredentials{
			TenantID:       body.Azure.TenantID,
			ClientID:       body.Azure.ClientID,
			ClientSecret:   body.Azure.ClientSecret,
			SubscriptionID: body.Azure.SubscriptionID,
		}
		input.AzureCreds = &c
	}

	dto, err := d.CredSvc.UpdateCredential(r.Context(), id, input, claims.Subject, claims.Username, clientIP(r), correlationID)
	if err != nil {
		writeCredError(w, err, correlationID)
		return
	}
	response.Success(w, dto)
}

// DeleteCredential handles DELETE /api/v1/settings/cloud-credentials/{id}.
func (d *Deps) DeleteCredential(w http.ResponseWriter, r *http.Request) {
	correlationID := middleware.GetCorrelationID(r.Context())
	claims := middleware.ClaimsFromContext(r.Context())
	if claims == nil {
		response.Error(w, "UNAUTHORIZED", "authentication required", http.StatusUnauthorized, correlationID)
		return
	}
	id := chi.URLParam(r, "id")

	err := d.CredSvc.DeleteCredential(r.Context(), id, claims.Subject, claims.Username, clientIP(r), correlationID)
	if err != nil {
		writeCredError(w, err, correlationID)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// TestCredential handles POST /api/v1/settings/cloud-credentials/{id}/test.
func (d *Deps) TestCredential(w http.ResponseWriter, r *http.Request) {
	correlationID := middleware.GetCorrelationID(r.Context())
	claims := middleware.ClaimsFromContext(r.Context())
	if claims == nil {
		response.Error(w, "UNAUTHORIZED", "authentication required", http.StatusUnauthorized, correlationID)
		return
	}
	id := chi.URLParam(r, "id")

	result, err := d.CredSvc.TestCredential(r.Context(), id, claims.Subject, claims.Username, clientIP(r), correlationID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			response.Error(w, "NOT_FOUND", "cloud credential not found", http.StatusNotFound, correlationID)
			return
		}
		response.Error(w, "INTERNAL_ERROR", "test failed", http.StatusInternalServerError, correlationID)
		return
	}
	response.Success(w, result)
}

// --- Instance Template Handlers ---

// CreateTemplate handles POST /api/v1/instance-templates.
func (d *Deps) CreateTemplate(w http.ResponseWriter, r *http.Request) {
	correlationID := middleware.GetCorrelationID(r.Context())
	claims := middleware.ClaimsFromContext(r.Context())
	if claims == nil {
		response.Error(w, "UNAUTHORIZED", "authentication required", http.StatusUnauthorized, correlationID)
		return
	}

	var input tmplsvc.CreateInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		response.Error(w, "BAD_REQUEST", "invalid request body", http.StatusBadRequest, correlationID)
		return
	}

	dto, err := d.TmplSvc.CreateTemplate(r.Context(), input, claims.Subject, claims.Username, clientIP(r), correlationID)
	if err != nil {
		writeTmplError(w, err, correlationID)
		return
	}
	response.Created(w, dto)
}

// ListTemplates handles GET /api/v1/instance-templates.
func (d *Deps) ListTemplates(w http.ResponseWriter, r *http.Request) {
	correlationID := middleware.GetCorrelationID(r.Context())
	dtos, err := d.TmplSvc.ListTemplates(r.Context(), r.URL.Query().Get("provider_type"))
	if err != nil {
		response.Error(w, "INTERNAL_ERROR", "failed to list templates", http.StatusInternalServerError, correlationID)
		return
	}
	response.Success(w, dtos)
}

// GetTemplate handles GET /api/v1/instance-templates/{id}.
func (d *Deps) GetTemplate(w http.ResponseWriter, r *http.Request) {
	correlationID := middleware.GetCorrelationID(r.Context())
	id := chi.URLParam(r, "id")
	dto, err := d.TmplSvc.GetTemplate(r.Context(), id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) || isNotFound(err) {
			response.Error(w, "NOT_FOUND", "instance template not found", http.StatusNotFound, correlationID)
			return
		}
		response.Error(w, "INTERNAL_ERROR", "failed to get template", http.StatusInternalServerError, correlationID)
		return
	}
	response.Success(w, dto)
}

// UpdateTemplate handles PUT /api/v1/instance-templates/{id} — creates a new version.
func (d *Deps) UpdateTemplate(w http.ResponseWriter, r *http.Request) {
	correlationID := middleware.GetCorrelationID(r.Context())
	claims := middleware.ClaimsFromContext(r.Context())
	if claims == nil {
		response.Error(w, "UNAUTHORIZED", "authentication required", http.StatusUnauthorized, correlationID)
		return
	}
	id := chi.URLParam(r, "id")

	var input tmplsvc.UpdateInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		response.Error(w, "BAD_REQUEST", "invalid request body", http.StatusBadRequest, correlationID)
		return
	}

	dto, err := d.TmplSvc.UpdateTemplate(r.Context(), id, input, claims.Subject, claims.Username, clientIP(r), correlationID)
	if err != nil {
		writeTmplError(w, err, correlationID)
		return
	}
	response.Success(w, dto)
}

// DeleteTemplate handles DELETE /api/v1/instance-templates/{id}.
func (d *Deps) DeleteTemplate(w http.ResponseWriter, r *http.Request) {
	correlationID := middleware.GetCorrelationID(r.Context())
	claims := middleware.ClaimsFromContext(r.Context())
	if claims == nil {
		response.Error(w, "UNAUTHORIZED", "authentication required", http.StatusUnauthorized, correlationID)
		return
	}
	id := chi.URLParam(r, "id")

	err := d.TmplSvc.DeleteTemplate(r.Context(), id, claims.Subject, claims.Username, clientIP(r), correlationID)
	if err != nil {
		writeTmplError(w, err, correlationID)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// ListTemplateVersions handles GET /api/v1/instance-templates/{id}/versions.
func (d *Deps) ListTemplateVersions(w http.ResponseWriter, r *http.Request) {
	correlationID := middleware.GetCorrelationID(r.Context())
	id := chi.URLParam(r, "id")
	dtos, err := d.TmplSvc.ListTemplateVersions(r.Context(), id)
	if err != nil {
		response.Error(w, "INTERNAL_ERROR", "failed to list template versions", http.StatusInternalServerError, correlationID)
		return
	}
	response.Success(w, dtos)
}

// GetTemplateVersion handles GET /api/v1/instance-templates/{id}/versions/{version}.
func (d *Deps) GetTemplateVersion(w http.ResponseWriter, r *http.Request) {
	correlationID := middleware.GetCorrelationID(r.Context())
	id := chi.URLParam(r, "id")
	versionStr := chi.URLParam(r, "version")
	version, err := strconv.Atoi(versionStr)
	if err != nil || version < 1 {
		response.Error(w, "BAD_REQUEST", "invalid version number", http.StatusBadRequest, correlationID)
		return
	}
	dto, err := d.TmplSvc.GetTemplateVersion(r.Context(), id, version)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) || isNotFound(err) {
			response.Error(w, "NOT_FOUND", "template version not found", http.StatusNotFound, correlationID)
			return
		}
		response.Error(w, "INTERNAL_ERROR", "failed to get template version", http.StatusInternalServerError, correlationID)
		return
	}
	response.Success(w, dto)
}

// ValidateTemplate handles POST /api/v1/instance-templates/validate.
func (d *Deps) ValidateTemplate(w http.ResponseWriter, r *http.Request) {
	correlationID := middleware.GetCorrelationID(r.Context())

	var input tmplsvc.CreateInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		response.Error(w, "BAD_REQUEST", "invalid request body", http.StatusBadRequest, correlationID)
		return
	}

	result, err := d.TmplSvc.ValidateTemplate(r.Context(), input)
	if err != nil {
		response.Error(w, "INTERNAL_ERROR", "validation failed", http.StatusInternalServerError, correlationID)
		return
	}
	response.Success(w, result)
}

// --- helpers ---

type awsCredInput struct {
	AccessKeyID     string `json:"access_key_id"`
	SecretAccessKey string `json:"secret_access_key"`
	IAMRoleARN      string `json:"iam_role_arn,omitempty"`
}

type azureCredInput struct {
	TenantID       string `json:"tenant_id"`
	ClientID       string `json:"client_id"`
	ClientSecret   string `json:"client_secret"`
	SubscriptionID string `json:"subscription_id"`
}

func writeCredError(w http.ResponseWriter, err error, correlationID string) {
	var ve *credsvc.ValidationError
	if errors.As(err, &ve) {
		response.Error(w, "VALIDATION_ERROR", ve.Error(), http.StatusBadRequest, correlationID)
		return
	}
	var ce *credsvc.ConflictError
	if errors.As(err, &ce) {
		response.Error(w, "CONFLICT", ce.Error(), http.StatusConflict, correlationID)
		return
	}
	if errors.Is(err, sql.ErrNoRows) {
		response.Error(w, "NOT_FOUND", "cloud credential not found", http.StatusNotFound, correlationID)
		return
	}
	response.Error(w, "INTERNAL_ERROR", "internal server error", http.StatusInternalServerError, correlationID)
}

func writeTmplError(w http.ResponseWriter, err error, correlationID string) {
	var ve *tmplsvc.ValidationError
	if errors.As(err, &ve) {
		response.Error(w, "VALIDATION_ERROR", ve.Error(), http.StatusBadRequest, correlationID)
		return
	}
	var ce *tmplsvc.ConflictError
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

func isNotFound(err error) bool {
	if err == nil {
		return false
	}
	return strings.Contains(err.Error(), "not found")
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
