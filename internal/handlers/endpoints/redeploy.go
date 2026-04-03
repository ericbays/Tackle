package endpoints

import (
	"context"
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"

	"tackle/internal/endpoint"
	"tackle/internal/middleware"
	"tackle/internal/services/audit"
	"tackle/internal/services/endpointmgmt"
	"tackle/pkg/response"
)

// RedeployDeps holds dependencies for the redeploy handler.
type RedeployDeps struct {
	Svc      *endpointmgmt.Service
	Deployer *endpoint.SSHDeployer
	CommAuth *endpoint.CommAuthService
	AuditSvc *audit.AuditService
}

// RedeployCampaignEndpoint handles POST /api/v1/campaigns/{id}/endpoint/redeploy.
// Re-deploys the proxy binary and configuration to an existing endpoint.
func (d *RedeployDeps) RedeployCampaignEndpoint(w http.ResponseWriter, r *http.Request) {
	correlationID := middleware.GetCorrelationID(r.Context())
	claims := middleware.ClaimsFromContext(r.Context())
	if claims == nil {
		response.Error(w, "UNAUTHORIZED", "authentication required", http.StatusUnauthorized, correlationID)
		return
	}
	campaignID := chi.URLParam(r, "id")

	ep, err := d.Svc.GetCampaignEndpoint(r.Context(), campaignID)
	if err != nil {
		writeEndpointError(w, err, correlationID)
		return
	}

	// Redeploy is only valid for active, stopped, or error endpoints.
	if ep.State != "active" && ep.State != "stopped" && ep.State != "error" {
		response.Error(w, "CONFLICT", "endpoint must be active, stopped, or in error state to redeploy", http.StatusConflict, correlationID)
		return
	}

	if ep.PublicIP == nil || *ep.PublicIP == "" {
		response.Error(w, "CONFLICT", "endpoint has no public IP — cannot SSH deploy", http.StatusConflict, correlationID)
		return
	}

	if ep.SSHKeyID == nil || *ep.SSHKeyID == "" {
		response.Error(w, "CONFLICT", "endpoint has no SSH key — cannot redeploy", http.StatusConflict, correlationID)
		return
	}

	actor := claims.Subject

	// Audit log the redeploy request.
	resourceType := "phishing_endpoint"
	_ = d.AuditSvc.Log(r.Context(), audit.LogEntry{
		Category:     audit.CategoryInfrastructure,
		Severity:     audit.SeverityInfo,
		ActorType:    audit.ActorTypeUser,
		ActorID:      &actor,
		ActorLabel:   actor,
		Action:       "endpoint.redeploy_requested",
		ResourceType: &resourceType,
		ResourceID:   &ep.ID,
		Details: map[string]any{
			"campaign_id": campaignID,
		},
	})

	// Generate fresh comm credentials.
	domain := ""
	if ep.Domain != nil {
		domain = *ep.Domain
	}
	creds, err := d.CommAuth.GenerateCredentials(r.Context(), ep.ID, domain, *ep.PublicIP)
	if err != nil {
		response.Error(w, "INTERNAL_ERROR", "failed to generate credentials", http.StatusInternalServerError, correlationID)
		return
	}

	sshKeyID := *ep.SSHKeyID
	publicIP := *ep.PublicIP
	endpointID := ep.ID

	// Run redeploy asynchronously.
	go func() {
		ctx := context.Background()

		sshKey, err := d.Svc.GetEndpointSSHKey(ctx, sshKeyID)
		if err != nil {
			slog.Error("redeploy: failed to get SSH key", "endpoint_id", endpointID, "error", err)
			return
		}

		deployConfig := endpoint.SSHDeployConfig{
			EndpointID:  endpointID,
			CampaignID:  campaignID,
			PublicIP:    publicIP,
			SSHPort:     22,
			TLSCertPEM:  creds.TLSCertPEM,
			TLSKeyPEM:   creds.TLSKeyPEM,
			ControlPort: creds.ControlPort,
			AuthToken:   creds.AuthToken,
		}

		if err := d.Deployer.Deploy(ctx, sshKey, deployConfig); err != nil {
			slog.Error("redeploy failed", "endpoint_id", endpointID, "error", err)
			return
		}

		slog.Info("redeploy complete", "endpoint_id", endpointID, "campaign_id", campaignID)
	}()

	response.Accepted(w, map[string]string{
		"status":  "redeploying",
		"message": "redeploy started — monitor endpoint health for completion",
	})
}
