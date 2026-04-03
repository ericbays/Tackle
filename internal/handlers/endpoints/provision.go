package endpoints

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"

	"tackle/internal/endpoint"
	"tackle/internal/endpoint/cloud"
	"tackle/internal/middleware"
	"tackle/internal/providers/credentials"
	"tackle/internal/repositories"
	"tackle/internal/services/audit"
	"tackle/pkg/response"
)

// ProvisionDeps holds dependencies for the provisioning handler, which requires
// additional repositories beyond what Deps provides.
type ProvisionDeps struct {
	Provisioner   *endpoint.Provisioner
	SSHDeployer   *endpoint.SSHDeployer
	CommAuth      *endpoint.CommAuthService
	CampaignRepo  *repositories.CampaignRepository
	CloudCredRepo *repositories.CloudCredentialRepository
	TemplateRepo  *repositories.InstanceTemplateRepository
	CredEncSvc    *credentials.EncryptionService
	AuditSvc      *audit.AuditService
	DNSUpdater    endpoint.DNSUpdater
}

// provisionRequest is the JSON body for POST /api/v1/endpoints/provision.
type provisionRequest struct {
	CampaignID    string `json:"campaign_id"`
	CloudCredID   string `json:"cloud_credential_id"`
	TemplateID    string `json:"template_id"`
	Domain        string `json:"domain,omitempty"`
	Zone          string `json:"zone,omitempty"`
	Subdomain     string `json:"subdomain,omitempty"`
	ResourceGroup string `json:"resource_group,omitempty"`
}

// ProvisionEndpoint handles POST /api/v1/endpoints/provision.
// Validates inputs, then runs the provisioning workflow asynchronously.
// Returns 202 Accepted with the campaign ID immediately.
func (d *ProvisionDeps) ProvisionEndpoint(w http.ResponseWriter, r *http.Request) {
	correlationID := middleware.GetCorrelationID(r.Context())
	claims := middleware.ClaimsFromContext(r.Context())
	if claims == nil {
		response.Error(w, "UNAUTHORIZED", "authentication required", http.StatusUnauthorized, correlationID)
		return
	}

	var req provisionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.Error(w, "BAD_REQUEST", "invalid JSON payload", http.StatusBadRequest, correlationID)
		return
	}

	// Validate required fields.
	if req.CampaignID == "" {
		response.Error(w, "BAD_REQUEST", "campaign_id is required", http.StatusBadRequest, correlationID)
		return
	}
	if req.CloudCredID == "" {
		response.Error(w, "BAD_REQUEST", "cloud_credential_id is required", http.StatusBadRequest, correlationID)
		return
	}
	if req.TemplateID == "" {
		response.Error(w, "BAD_REQUEST", "template_id is required", http.StatusBadRequest, correlationID)
		return
	}

	// Validate campaign exists and is in a provisionable state.
	campaign, err := d.CampaignRepo.GetByID(r.Context(), req.CampaignID)
	if err != nil {
		response.Error(w, "NOT_FOUND", "campaign not found", http.StatusNotFound, correlationID)
		return
	}
	if campaign.CurrentState != "approved" && campaign.CurrentState != "building" {
		response.Error(w, "CONFLICT", fmt.Sprintf("campaign is in %s state; must be approved or building", campaign.CurrentState), http.StatusConflict, correlationID)
		return
	}

	// Validate cloud credential exists and is healthy.
	cred, err := d.CloudCredRepo.GetByID(r.Context(), req.CloudCredID)
	if err != nil {
		if err == sql.ErrNoRows {
			response.Error(w, "NOT_FOUND", "cloud credential not found", http.StatusNotFound, correlationID)
			return
		}
		response.Error(w, "INTERNAL_ERROR", "internal server error", http.StatusInternalServerError, correlationID)
		return
	}
	if cred.Status == repositories.CloudCredentialStatusError {
		response.Error(w, "CONFLICT", "cloud credential is in error state — test it first", http.StatusConflict, correlationID)
		return
	}

	// Validate instance template exists.
	tmpl, err := d.TemplateRepo.GetByID(r.Context(), req.TemplateID)
	if err != nil {
		if err == sql.ErrNoRows {
			response.Error(w, "NOT_FOUND", "instance template not found", http.StatusNotFound, correlationID)
			return
		}
		response.Error(w, "INTERNAL_ERROR", "internal server error", http.StatusInternalServerError, correlationID)
		return
	}

	// Build cloud provider from credential.
	provider, err := cloud.NewProviderFromCredential(r.Context(), cred, d.CredEncSvc, tmpl.Version.Region, req.ResourceGroup)
	if err != nil {
		response.Error(w, "INTERNAL_ERROR", "failed to create cloud provider", http.StatusInternalServerError, correlationID)
		return
	}

	// Build provision config from template version.
	config := endpoint.ProvisionConfig{
		CampaignID:      req.CampaignID,
		CloudCredential: cred,
		Region:          tmpl.Version.Region,
		InstanceSize:    tmpl.Version.InstanceSize,
		OSImage:         tmpl.Version.OSImage,
		Domain:          req.Domain,
		Zone:            req.Zone,
		Subdomain:       req.Subdomain,
		ResourceGroup:   req.ResourceGroup,
		SecurityGroups:  tmpl.Version.SecurityGroups,
	}
	if tmpl.Version.UserData != nil {
		config.UserData = *tmpl.Version.UserData
	}

	actor := claims.Subject

	// Audit log the provisioning request.
	resourceType := "phishing_endpoint"
	_ = d.AuditSvc.Log(r.Context(), audit.LogEntry{
		Category:     audit.CategoryInfrastructure,
		Severity:     audit.SeverityInfo,
		ActorType:    audit.ActorTypeUser,
		ActorID:      &actor,
		ActorLabel:   actor,
		Action:       "endpoint.provision_requested",
		ResourceType: &resourceType,
		Details: map[string]any{
			"campaign_id":      req.CampaignID,
			"cloud_credential": req.CloudCredID,
			"template_id":      req.TemplateID,
			"domain":           req.Domain,
		},
	})

	// Run provisioning asynchronously — return 202 immediately.
	go func() {
		ctx := context.Background()
		ep, err := d.Provisioner.ProvisionEndpoint(ctx, provider, d.DNSUpdater, config, actor)
		if err != nil {
			slog.Error("async provisioning failed", "campaign_id", req.CampaignID, "error", err)
			return
		}
		slog.Info("async provisioning complete", "endpoint_id", ep.ID, "campaign_id", req.CampaignID)
	}()

	response.Accepted(w, map[string]any{
		"status":      "provisioning",
		"campaign_id": req.CampaignID,
		"message":     "endpoint provisioning started — monitor status via campaign endpoint API",
	})
}
