package endpoint

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"time"

	"tackle/internal/endpoint/cloud"
	dnsiface "tackle/internal/providers/dns"
	"tackle/internal/repositories"
	"tackle/internal/services/audit"
)

// DNSUpdater is the interface for DNS record operations needed by the provisioner.
// Implemented by the DNS service layer.
type DNSUpdater interface {
	// CreateARecord creates an A record pointing the domain to the given IP.
	CreateARecord(ctx context.Context, zone, subdomain, ip string) error
	// DeleteARecord removes the A record for the given subdomain.
	DeleteARecord(ctx context.Context, zone, subdomain string) error
	// CheckPropagation verifies that DNS has propagated for the given domain/IP.
	CheckPropagation(ctx context.Context, domain, expectedIP string) (bool, error)
}

// SimpleDNSUpdater implements DNSUpdater using a dns.Provider directly.
type SimpleDNSUpdater struct {
	provider dnsiface.Provider
}

// NewSimpleDNSUpdater creates a SimpleDNSUpdater wrapping a DNS provider.
func NewSimpleDNSUpdater(provider dnsiface.Provider) *SimpleDNSUpdater {
	return &SimpleDNSUpdater{provider: provider}
}

// CreateARecord creates an A record in the zone.
func (u *SimpleDNSUpdater) CreateARecord(ctx context.Context, zone, subdomain, ip string) error {
	_, err := u.provider.CreateRecord(ctx, zone, dnsiface.Record{
		Type:  dnsiface.RecordTypeA,
		Name:  subdomain,
		Value: ip,
		TTL:   300,
	})
	return err
}

// DeleteARecord removes an A record from the zone.
func (u *SimpleDNSUpdater) DeleteARecord(ctx context.Context, zone, subdomain string) error {
	records, err := u.provider.ListRecords(ctx, zone)
	if err != nil {
		return fmt.Errorf("list records for deletion: %w", err)
	}
	for _, r := range records {
		if r.Type == dnsiface.RecordTypeA && r.Name == subdomain {
			return u.provider.DeleteRecord(ctx, zone, r.ID)
		}
	}
	return nil // Record doesn't exist — nothing to delete.
}

// CheckPropagation verifies DNS resolution matches the expected IP.
func (u *SimpleDNSUpdater) CheckPropagation(ctx context.Context, domain, expectedIP string) (bool, error) {
	ips, err := net.DefaultResolver.LookupHost(ctx, domain)
	if err != nil {
		return false, nil // Not yet propagated.
	}
	for _, ip := range ips {
		if ip == expectedIP {
			return true, nil
		}
	}
	return false, nil
}

// ProvisionConfig holds the parameters for a full endpoint provisioning workflow.
type ProvisionConfig struct {
	CampaignID      string
	CloudCredential repositories.CloudCredential
	Region          string
	InstanceSize    string
	OSImage         string
	Domain          string
	Zone            string
	Subdomain       string
	ResourceGroup   string
	SecurityGroups  []string
	SubnetID        string
	SSHPublicKey    string
	UserData        string
}

// Provisioner orchestrates the full endpoint provisioning workflow:
// endpoint creation → cloud VM provisioning → static IP allocation → DNS update → SSH deploy → active.
type Provisioner struct {
	sm          *StateMachine
	repo        *repositories.PhishingEndpointRepository
	ipPool      *cloud.IPPoolAllocator
	auditSvc    *audit.AuditService
	deployer    *SSHDeployer    // Optional: if set, auto-deploy after provisioning.
	commAuth    *CommAuthService // Optional: generates comm credentials for deployment.
	onIPChange  IPChangeFunc    // Optional: called when endpoint public IP changes.
}

// IPChangeFunc is called when an endpoint's public IP changes.
type IPChangeFunc func(ctx context.Context, endpointID, oldIP, newIP string)

// NewProvisioner creates a new Provisioner.
func NewProvisioner(
	sm *StateMachine,
	repo *repositories.PhishingEndpointRepository,
	ipPool *cloud.IPPoolAllocator,
	auditSvc *audit.AuditService,
) *Provisioner {
	return &Provisioner{sm: sm, repo: repo, ipPool: ipPool, auditSvc: auditSvc}
}

// SetDeployer configures the SSH deployer for full provisioning workflows.
func (p *Provisioner) SetDeployer(deployer *SSHDeployer, commAuth *CommAuthService) {
	p.deployer = deployer
	p.commAuth = commAuth
}

// SetIPChangeCallback sets a function to be called when an endpoint's public IP changes.
func (p *Provisioner) SetIPChangeCallback(fn IPChangeFunc) {
	p.onIPChange = fn
}

// ProvisionEndpoint runs the full async provisioning workflow for an endpoint.
// It creates the endpoint, transitions through states, provisions the VM, allocates IP, and updates DNS.
// This is designed to be called in a background goroutine.
func (p *Provisioner) ProvisionEndpoint(ctx context.Context, provider cloud.Provider, dnsUpdater DNSUpdater, config ProvisionConfig, actor string) (repositories.PhishingEndpoint, error) {
	// Step 1: Create endpoint in Requested state.
	campaignIDPtr := &config.CampaignID
	ep, err := p.sm.CreateEndpoint(ctx, campaignIDPtr, repositories.CloudProviderType(provider.ProviderName()), config.Region, actor)
	if err != nil {
		return repositories.PhishingEndpoint{}, fmt.Errorf("provisioner: create endpoint: %w", err)
	}

	// Step 2: Transition to Provisioning.
	ep, err = p.sm.TransitionSystem(ctx, ep.ID, repositories.EndpointStateProvisioning, "starting cloud VM provisioning")
	if err != nil {
		return ep, fmt.Errorf("provisioner: transition to provisioning: %w", err)
	}

	// Step 3: Allocate static IP.
	var publicIP string
	var allocationID string

	if provider.ProviderName() == "proxmox" {
		// Proxmox: allocate from DB pool.
		ip, err := p.ipPool.Allocate(ctx, config.CloudCredential.ID, ep.ID)
		if err != nil {
			p.transitionToError(ctx, ep.ID, "IP pool exhausted: "+err.Error())
			return ep, fmt.Errorf("provisioner: allocate proxmox ip: %w", err)
		}
		publicIP = ip
		allocationID = ep.ID // Proxmox uses endpoint ID as allocation reference.
	} else {
		// AWS/Azure: allocate from cloud provider.
		result, err := provider.AllocateStaticIP(ctx)
		if err != nil {
			p.transitionToError(ctx, ep.ID, "failed to allocate static IP: "+err.Error())
			return ep, fmt.Errorf("provisioner: allocate static ip: %w", err)
		}
		publicIP = result.IP
		allocationID = result.AllocationID
	}

	// Step 4: Provision the cloud VM.
	tags := map[string]string{
		"managed-by":  "tackle",
		"campaign_id": config.CampaignID,
		"endpoint_id": ep.ID,
		"ip_address":  publicIP,
	}

	instanceID, err := provider.ProvisionInstance(ctx, cloud.ProvisionConfig{
		Region:         config.Region,
		InstanceSize:   config.InstanceSize,
		OSImage:        config.OSImage,
		SSHPublicKey:   config.SSHPublicKey,
		Tags:           tags,
		UserData:       config.UserData,
		SecurityGroups: config.SecurityGroups,
		SubnetID:       config.SubnetID,
		ResourceGroup:  config.ResourceGroup,
	})
	if err != nil {
		// Release IP on failure.
		p.releaseIP(ctx, provider, allocationID, ep.ID)
		p.transitionToError(ctx, ep.ID, "VM provisioning failed: "+err.Error())
		return ep, fmt.Errorf("provisioner: provision instance: %w", err)
	}

	// Step 5: Associate static IP (AWS/Azure only — Proxmox does it via cloud-init).
	if provider.ProviderName() != "proxmox" {
		if err := provider.AssociateStaticIP(ctx, instanceID, allocationID); err != nil {
			slog.Warn("provisioner: associate static IP failed", "error", err, "endpoint_id", ep.ID)
		}
	}

	// Step 6: Record instance info in DB.
	oldIP := ""
	if ep.PublicIP != nil {
		oldIP = *ep.PublicIP
	}
	if err := p.repo.UpdateInstanceInfo(ctx, ep.ID, instanceID, publicIP); err != nil {
		slog.Error("provisioner: update instance info", "error", err, "endpoint_id", ep.ID)
	}
	if p.onIPChange != nil && oldIP != publicIP {
		p.onIPChange(ctx, ep.ID, oldIP, publicIP)
	}

	// Store IP allocation ID for proper release on termination (AWS/Azure).
	if allocationID != "" && provider.ProviderName() != "proxmox" {
		if err := p.repo.UpdateIPAllocationID(ctx, ep.ID, allocationID); err != nil {
			slog.Warn("provisioner: store allocation ID", "error", err, "endpoint_id", ep.ID)
		}
	}

	// Step 7: Update DNS A record.
	if dnsUpdater != nil && config.Zone != "" && config.Subdomain != "" {
		if err := dnsUpdater.CreateARecord(ctx, config.Zone, config.Subdomain, publicIP); err != nil {
			slog.Warn("provisioner: DNS update failed", "error", err, "endpoint_id", ep.ID)
			// DNS failure is not fatal — surface as warning, continue to Configuring.
		}

		// Update domain in endpoint record.
		domain := config.Domain
		if domain == "" {
			domain = config.Subdomain + "." + config.Zone
		}
		if err := p.repo.UpdateDomain(ctx, ep.ID, domain); err != nil {
			slog.Error("provisioner: update domain", "error", err, "endpoint_id", ep.ID)
		}

		// Verify propagation (non-blocking — log warning if not propagated yet).
		propagationCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
		defer cancel()
		propagated, _ := dnsUpdater.CheckPropagation(propagationCtx, domain, publicIP)
		if !propagated {
			slog.Info("provisioner: DNS not yet propagated", "domain", domain, "ip", publicIP)
		}
	}

	// Step 8: Transition to Configuring.
	ep, err = p.sm.TransitionSystem(ctx, ep.ID, repositories.EndpointStateConfiguring, "VM provisioned, ready for configuration deployment")
	if err != nil {
		return ep, fmt.Errorf("provisioner: transition to configuring: %w", err)
	}

	// Log provisioning complete.
	resourceType := "phishing_endpoint"
	_ = p.auditSvc.Log(ctx, audit.LogEntry{
		Category:     audit.CategoryInfrastructure,
		Severity:     audit.SeverityInfo,
		ActorType:    audit.ActorTypeSystem,
		ActorLabel:   "system",
		Action:       "endpoint.provisioned",
		ResourceType: &resourceType,
		ResourceID:   &ep.ID,
		Details: map[string]any{
			"cloud_provider": provider.ProviderName(),
			"instance_id":    instanceID,
			"public_ip":      publicIP,
			"region":         config.Region,
		},
	})

	// Re-read from DB to get final state.
	ep, _ = p.repo.GetByID(ctx, ep.ID)
	return ep, nil
}

// DeployConfig holds additional parameters needed for SSH deployment after provisioning.
type DeployConfig struct {
	FrameworkHost string // Base URL of the framework server (for proxy config).
	BinaryPath    string // Path to the compiled proxy binary.
	BinaryHash    string // SHA-256 hash of the proxy binary.
}

// ProvisionAndDeploy runs the full provisioning workflow including SSH deployment.
// After VM provisioning (ProvisionEndpoint), it generates comm credentials, deploys
// the proxy binary via SSH, waits for heartbeat, and transitions to Active.
// This is designed to be called in a background goroutine.
func (p *Provisioner) ProvisionAndDeploy(ctx context.Context, provider cloud.Provider, dnsUpdater DNSUpdater, config ProvisionConfig, deployConfig DeployConfig, actor string) (repositories.PhishingEndpoint, error) {
	if p.deployer == nil || p.commAuth == nil {
		return repositories.PhishingEndpoint{}, fmt.Errorf("provisioner: deployer and commAuth must be set for full provisioning")
	}

	// Step 1-8: Provision VM (creates endpoint, allocates IP, provisions VM, updates DNS, transitions to Configuring).
	ep, err := p.ProvisionEndpoint(ctx, provider, dnsUpdater, config, actor)
	if err != nil {
		return ep, fmt.Errorf("provisioner: provision: %w", err)
	}

	// Step 9: Generate SSH key pair for this deployment.
	sshKey, pubKeyStr, err := p.deployer.GenerateSSHKeyPair(ctx, config.CampaignID)
	if err != nil {
		p.transitionToError(ctx, ep.ID, "SSH key generation failed: "+err.Error())
		return ep, fmt.Errorf("provisioner: generate ssh key: %w", err)
	}
	_ = pubKeyStr // Public key was injected via UserData during VM creation.

	// Step 10: Generate communication credentials (TLS cert + auth token).
	domain := ""
	if ep.Domain != nil {
		domain = *ep.Domain
	}
	publicIP := ""
	if ep.PublicIP != nil {
		publicIP = *ep.PublicIP
	}
	creds, err := p.commAuth.GenerateCredentials(ctx, ep.ID, domain, publicIP)
	if err != nil {
		p.transitionToError(ctx, ep.ID, "credential generation failed: "+err.Error())
		return ep, fmt.Errorf("provisioner: generate credentials: %w", err)
	}

	// Step 11: Generate hardening script.
	hardeningScript, err := GenerateHardeningScript(HardeningConfig{
		ProxyPort:   443,
		ControlPort: creds.ControlPort,
		BinaryUser:  "tackle",
	})
	if err != nil {
		slog.Warn("provisioner: hardening script generation failed", "error", err, "endpoint_id", ep.ID)
		hardeningScript = nil // Non-fatal — deploy without hardening.
	}

	// Step 12: Wait for SSH connectivity and deploy binary + config.
	sshConfig := SSHDeployConfig{
		EndpointID:      ep.ID,
		CampaignID:      config.CampaignID,
		PublicIP:         publicIP,
		SSHPort:         22,
		BinaryPath:      deployConfig.BinaryPath,
		BinaryHash:      deployConfig.BinaryHash,
		TLSCertPEM:      creds.TLSCertPEM,
		TLSKeyPEM:       creds.TLSKeyPEM,
		ProxyConfig:     []byte(fmt.Sprintf(`{"campaign_id":"%s","framework_host":"%s","control_port":%d,"auth_token":"%s"}`, config.CampaignID, deployConfig.FrameworkHost, creds.ControlPort, creds.AuthToken)),
		ControlPort:     creds.ControlPort,
		AuthToken:       creds.AuthToken,
		HardeningScript: hardeningScript,
	}

	if err := p.deployer.Deploy(ctx, sshKey, sshConfig); err != nil {
		// Deploy handles its own error transitions.
		return ep, fmt.Errorf("provisioner: deploy: %w", err)
	}

	// Re-read from DB to get final state (should be Active).
	ep, _ = p.repo.GetByID(ctx, ep.ID)
	return ep, nil
}

// TeardownEndpoint terminates a cloud VM, releases its IP, reverts DNS, and transitions to Terminated.
func (p *Provisioner) TeardownEndpoint(ctx context.Context, provider cloud.Provider, dnsUpdater DNSUpdater, endpointID, zone, subdomain, actor string) error {
	ep, err := p.repo.GetByID(ctx, endpointID)
	if err != nil {
		return fmt.Errorf("provisioner: get endpoint: %w", err)
	}

	// Step 1: Terminate the cloud VM.
	if ep.InstanceID != nil && *ep.InstanceID != "" {
		if err := provider.TerminateInstance(ctx, *ep.InstanceID); err != nil {
			slog.Warn("provisioner: terminate instance failed", "error", err, "endpoint_id", endpointID)
		}
	}

	// Step 2: Release static IP.
	if provider.ProviderName() == "proxmox" {
		if err := p.ipPool.Release(ctx, endpointID); err != nil {
			slog.Warn("provisioner: release proxmox ip failed", "error", err, "endpoint_id", endpointID)
		}
	} else if ep.PublicIP != nil && *ep.PublicIP != "" {
		// Retrieve stored allocation ID for proper IP release.
		allocID, err := p.repo.GetIPAllocationID(ctx, endpointID)
		if err != nil {
			slog.Warn("provisioner: get allocation ID failed", "error", err, "endpoint_id", endpointID)
		} else if allocID != "" {
			if err := provider.ReleaseStaticIP(ctx, allocID); err != nil {
				slog.Warn("provisioner: release static IP failed", "error", err, "endpoint_id", endpointID, "allocation_id", allocID)
			} else {
				slog.Info("provisioner: static IP released", "endpoint_id", endpointID, "allocation_id", allocID)
			}
		} else {
			slog.Info("provisioner: no allocation ID stored, IP release skipped", "endpoint_id", endpointID)
		}
	}

	// Step 3: Revert DNS.
	if dnsUpdater != nil && zone != "" && subdomain != "" {
		if err := dnsUpdater.DeleteARecord(ctx, zone, subdomain); err != nil {
			slog.Warn("provisioner: DNS cleanup failed", "error", err, "endpoint_id", endpointID)
		}
	}

	// Step 4: Transition to Terminated.
	_, err = p.sm.Transition(ctx, endpointID, repositories.EndpointStateTerminated, actor, "endpoint teardown")
	if err != nil {
		return fmt.Errorf("provisioner: transition to terminated: %w", err)
	}

	// Log teardown.
	resourceType := "phishing_endpoint"
	_ = p.auditSvc.Log(ctx, audit.LogEntry{
		Category:     audit.CategoryInfrastructure,
		Severity:     audit.SeverityInfo,
		ActorType:    audit.ActorTypeUser,
		ActorID:      &actor,
		ActorLabel:   actor,
		Action:       "endpoint.terminated",
		ResourceType: &resourceType,
		ResourceID:   &endpointID,
		Details: map[string]any{
			"cloud_provider": provider.ProviderName(),
		},
	})

	return nil
}

func (p *Provisioner) transitionToError(ctx context.Context, endpointID, reason string) {
	_, err := p.sm.TransitionSystem(ctx, endpointID, repositories.EndpointStateError, reason)
	if err != nil {
		slog.Error("provisioner: failed to transition to error state", "error", err, "endpoint_id", endpointID)
	}
}

func (p *Provisioner) releaseIP(ctx context.Context, provider cloud.Provider, allocationID, endpointID string) {
	if provider.ProviderName() == "proxmox" {
		if err := p.ipPool.Release(ctx, endpointID); err != nil {
			slog.Warn("provisioner: release IP on failure", "error", err, "endpoint_id", endpointID)
		}
	} else {
		if err := provider.ReleaseStaticIP(ctx, allocationID); err != nil {
			slog.Warn("provisioner: release static IP on failure", "error", err, "endpoint_id", endpointID)
		}
	}
}
