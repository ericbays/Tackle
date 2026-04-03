package cloud

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"tackle/internal/providers/credentials"
)

// ProxmoxProvider implements the Provider interface for Proxmox VE hypervisors.
// VMs are cloned from a cloud-init enabled template, configured with static IPs from a pool,
// and tagged with Tackle metadata for audit traceability.
type ProxmoxProvider struct {
	baseURL      string
	tokenID      string
	tokenSec     string
	node         string
	templateVMID int
	bridge       string
	gateway      string
	subnetMask   string
	client       *http.Client
}

// NewProxmoxProvider creates a Proxmox provider from the given credentials.
func NewProxmoxProvider(creds credentials.ProxmoxCredentials) (*ProxmoxProvider, error) {
	if creds.Host == "" {
		return nil, fmt.Errorf("proxmox provider: host is required")
	}
	if creds.Node == "" {
		return nil, fmt.Errorf("proxmox provider: node is required")
	}
	if creds.TemplateVMID == 0 {
		return nil, fmt.Errorf("proxmox provider: template_vmid is required")
	}

	port := creds.Port
	if port == 0 {
		port = 8006
	}
	bridge := creds.Bridge
	if bridge == "" {
		bridge = "vmbr0"
	}

	return &ProxmoxProvider{
		baseURL:      fmt.Sprintf("https://%s:%d/api2/json", creds.Host, port),
		tokenID:      creds.TokenID,
		tokenSec:     creds.TokenSecret,
		node:         creds.Node,
		templateVMID: creds.TemplateVMID,
		bridge:       bridge,
		gateway:      creds.Gateway,
		subnetMask:   creds.SubnetMask,
		client: &http.Client{
			Timeout: 120 * time.Second,
			Transport: &http.Transport{
				TLSClientConfig: &tls.Config{
					InsecureSkipVerify: true, // Proxmox typically uses self-signed certs in lab environments.
				},
			},
		},
	}, nil
}

// ProviderName returns "proxmox".
func (p *ProxmoxProvider) ProviderName() string { return "proxmox" }

// ProvisionInstance clones the template VM and configures it with cloud-init.
// The config.Tags must include "ip_address" for the cloud-init static IP configuration.
// Returns the new VMID as a string.
func (p *ProxmoxProvider) ProvisionInstance(ctx context.Context, config ProvisionConfig) (string, error) {
	// Get next available VMID.
	newVMID, err := p.getNextVMID(ctx)
	if err != nil {
		return "", fmt.Errorf("proxmox provider: get next vmid: %w", err)
	}

	// Clone from template.
	cloneData := url.Values{}
	cloneData.Set("newid", strconv.Itoa(newVMID))
	cloneData.Set("full", "1") // Full clone, not linked.
	vmName := "tackle-ep-" + generateShortID()
	cloneData.Set("name", vmName)

	// Build description with Tackle metadata.
	description := "managed-by: tackle"
	if campaignID, ok := config.Tags["campaign_id"]; ok {
		description += "\ncampaign_id: " + campaignID
	}
	if endpointID, ok := config.Tags["endpoint_id"]; ok {
		description += "\nendpoint_id: " + endpointID
	}
	cloneData.Set("description", description)

	clonePath := fmt.Sprintf("/nodes/%s/qemu/%d/clone", p.node, p.templateVMID)
	resp, err := p.doFormRequest(ctx, "POST", clonePath, cloneData)
	if err != nil {
		return "", fmt.Errorf("proxmox provider: clone vm: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("proxmox provider: clone failed (status %d): %s", resp.StatusCode, string(body))
	}

	// Wait for clone task to complete.
	taskID, err := p.extractTaskID(resp.Body)
	if err != nil {
		return "", fmt.Errorf("proxmox provider: parse clone task: %w", err)
	}
	if err := p.waitForTask(ctx, taskID); err != nil {
		return "", fmt.Errorf("proxmox provider: clone task: %w", err)
	}

	vmIDStr := strconv.Itoa(newVMID)

	// Configure cloud-init: static IP, gateway, DNS, SSH key.
	ciData := url.Values{}
	ipAddr := config.Tags["ip_address"]
	if ipAddr != "" && p.subnetMask != "" {
		cidrBits := subnetMaskToCIDR(p.subnetMask)
		ciData.Set("ipconfig0", fmt.Sprintf("ip=%s/%d,gw=%s", ipAddr, cidrBits, p.gateway))
	}
	if config.SSHPublicKey != "" {
		ciData.Set("sshkeys", url.QueryEscape(config.SSHPublicKey))
	}
	ciData.Set("nameserver", "8.8.8.8 8.8.4.4")

	// Parse and apply instance size (cores/memory).
	if config.InstanceSize != "" {
		cores, memMB := parseProxmoxSize(config.InstanceSize)
		if cores > 0 {
			ciData.Set("cores", strconv.Itoa(cores))
		}
		if memMB > 0 {
			ciData.Set("memory", strconv.Itoa(memMB))
		}
	}

	ciPath := fmt.Sprintf("/nodes/%s/qemu/%d/config", p.node, newVMID)
	ciResp, err := p.doFormRequest(ctx, "PUT", ciPath, ciData)
	if err != nil {
		return vmIDStr, fmt.Errorf("proxmox provider: configure cloud-init: %w", err)
	}
	defer ciResp.Body.Close()

	// Set PVE tags for Tackle management.
	tagData := url.Values{}
	tagParts := []string{"managed-by-tackle"}
	if cid, ok := config.Tags["campaign_id"]; ok {
		tagParts = append(tagParts, "campaign-"+cid[:8])
	}
	tagData.Set("tags", strings.Join(tagParts, ";"))
	tagResp, err := p.doFormRequest(ctx, "PUT", ciPath, tagData)
	if err == nil {
		tagResp.Body.Close()
	}

	// Start the VM.
	startPath := fmt.Sprintf("/nodes/%s/qemu/%d/status/start", p.node, newVMID)
	startResp, err := p.doFormRequest(ctx, "POST", startPath, nil)
	if err != nil {
		return vmIDStr, fmt.Errorf("proxmox provider: start vm: %w", err)
	}
	defer startResp.Body.Close()

	return vmIDStr, nil
}

// GetInstanceStatus returns the current status of a Proxmox VM.
func (p *ProxmoxProvider) GetInstanceStatus(ctx context.Context, instanceID string) (InstanceStatus, error) {
	path := fmt.Sprintf("/nodes/%s/qemu/%s/status/current", p.node, instanceID)
	resp, err := p.doRequest(ctx, "GET", path, nil)
	if err != nil {
		return InstanceStatus{}, fmt.Errorf("proxmox provider: get status: %w", err)
	}
	defer resp.Body.Close()

	var result struct {
		Data struct {
			Status string `json:"status"`
			VMID   int    `json:"vmid"`
		} `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return InstanceStatus{}, fmt.Errorf("proxmox provider: decode status: %w", err)
	}

	return InstanceStatus{
		InstanceID: instanceID,
		State:      normalizeProxmoxState(result.Data.Status),
	}, nil
}

// StopInstance sends a shutdown command to a Proxmox VM.
func (p *ProxmoxProvider) StopInstance(ctx context.Context, instanceID string) error {
	path := fmt.Sprintf("/nodes/%s/qemu/%s/status/shutdown", p.node, instanceID)
	resp, err := p.doFormRequest(ctx, "POST", path, nil)
	if err != nil {
		return fmt.Errorf("proxmox provider: shutdown vm: %w", err)
	}
	resp.Body.Close()
	return nil
}

// StartInstance starts a stopped Proxmox VM.
func (p *ProxmoxProvider) StartInstance(ctx context.Context, instanceID string) error {
	path := fmt.Sprintf("/nodes/%s/qemu/%s/status/start", p.node, instanceID)
	resp, err := p.doFormRequest(ctx, "POST", path, nil)
	if err != nil {
		return fmt.Errorf("proxmox provider: start vm: %w", err)
	}
	resp.Body.Close()
	return nil
}

// TerminateInstance destroys a Proxmox VM permanently.
func (p *ProxmoxProvider) TerminateInstance(ctx context.Context, instanceID string) error {
	// Stop first if running.
	status, err := p.GetInstanceStatus(ctx, instanceID)
	if err == nil && status.State == "running" {
		stopPath := fmt.Sprintf("/nodes/%s/qemu/%s/status/stop", p.node, instanceID)
		stopResp, err := p.doFormRequest(ctx, "POST", stopPath, nil)
		if err == nil {
			stopResp.Body.Close()
			// Wait briefly for stop.
			time.Sleep(3 * time.Second)
		}
	}

	path := fmt.Sprintf("/nodes/%s/qemu/%s", p.node, instanceID)
	data := url.Values{}
	data.Set("purge", "1")
	data.Set("destroy-unreferenced-disks", "1")
	resp, err := p.doFormRequest(ctx, "DELETE", path+"?"+data.Encode(), nil)
	if err != nil {
		return fmt.Errorf("proxmox provider: destroy vm: %w", err)
	}
	resp.Body.Close()
	return nil
}

// AllocateStaticIP is a no-op for Proxmox — IPs are allocated from the DB pool, not the cloud API.
// The actual allocation happens in the repository layer (PhishingEndpointRepository.AllocateProxmoxIP).
func (p *ProxmoxProvider) AllocateStaticIP(ctx context.Context) (StaticIPResult, error) {
	return StaticIPResult{}, fmt.Errorf("proxmox provider: static IP allocation is pool-based, use the IP pool allocator")
}

// AssociateStaticIP is a no-op for Proxmox — IP is configured via cloud-init during provisioning.
func (p *ProxmoxProvider) AssociateStaticIP(ctx context.Context, instanceID, allocationID string) error {
	return nil
}

// ReleaseStaticIP is a no-op for Proxmox — IP release is handled in the DB pool.
func (p *ProxmoxProvider) ReleaseStaticIP(ctx context.Context, allocationID string) error {
	return nil
}

// getNextVMID requests the next available VMID from the Proxmox cluster.
func (p *ProxmoxProvider) getNextVMID(ctx context.Context) (int, error) {
	resp, err := p.doRequest(ctx, "GET", "/cluster/nextid", nil)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()

	var result struct {
		Data json.RawMessage `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return 0, err
	}

	// PVE returns the ID as a quoted string.
	var idStr string
	if err := json.Unmarshal(result.Data, &idStr); err != nil {
		// Try as integer.
		var idInt int
		if err2 := json.Unmarshal(result.Data, &idInt); err2 != nil {
			return 0, fmt.Errorf("proxmox: cannot parse nextid: %w", err)
		}
		return idInt, nil
	}
	return strconv.Atoi(idStr)
}

// waitForTask polls a Proxmox task until completion or timeout.
func (p *ProxmoxProvider) waitForTask(ctx context.Context, taskID string) error {
	deadline := time.Now().Add(5 * time.Minute)
	for time.Now().Before(deadline) {
		path := fmt.Sprintf("/nodes/%s/tasks/%s/status", p.node, taskID)
		resp, err := p.doRequest(ctx, "GET", path, nil)
		if err != nil {
			return err
		}

		var result struct {
			Data struct {
				Status string `json:"status"`
			} `json:"data"`
		}
		json.NewDecoder(resp.Body).Decode(&result)
		resp.Body.Close()

		if result.Data.Status == "stopped" {
			return nil // Task completed.
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(2 * time.Second):
		}
	}
	return fmt.Errorf("proxmox: task %s timed out", taskID)
}

// extractTaskID extracts the UPID task identifier from a Proxmox API response.
func (p *ProxmoxProvider) extractTaskID(body io.Reader) (string, error) {
	var result struct {
		Data string `json:"data"`
	}
	if err := json.NewDecoder(body).Decode(&result); err != nil {
		return "", err
	}
	if result.Data == "" {
		return "", fmt.Errorf("proxmox: empty task ID in response")
	}
	return result.Data, nil
}

// doRequest makes an authenticated HTTP request to the Proxmox API.
func (p *ProxmoxProvider) doRequest(ctx context.Context, method, path string, body io.Reader) (*http.Response, error) {
	reqURL := p.baseURL + path
	req, err := http.NewRequestWithContext(ctx, method, reqURL, body)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", fmt.Sprintf("PVEAPIToken=%s=%s", p.tokenID, p.tokenSec))
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	return p.client.Do(req)
}

// doFormRequest makes an authenticated form-encoded HTTP request.
func (p *ProxmoxProvider) doFormRequest(ctx context.Context, method, path string, data url.Values) (*http.Response, error) {
	reqURL := p.baseURL + path
	var body io.Reader
	if data != nil {
		body = strings.NewReader(data.Encode())
	}
	req, err := http.NewRequestWithContext(ctx, method, reqURL, body)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", fmt.Sprintf("PVEAPIToken=%s=%s", p.tokenID, p.tokenSec))
	if data != nil {
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	}
	return p.client.Do(req)
}

func normalizeProxmoxState(status string) string {
	switch status {
	case "running":
		return "running"
	case "stopped":
		return "stopped"
	default:
		return "unknown"
	}
}

// subnetMaskToCIDR converts a dotted-decimal subnet mask to CIDR notation bits.
func subnetMaskToCIDR(mask string) int {
	parts := strings.Split(mask, ".")
	if len(parts) != 4 {
		return 24 // Default.
	}
	bits := 0
	for _, p := range parts {
		n, _ := strconv.Atoi(p)
		for n > 0 {
			bits += n & 1
			n >>= 1
		}
	}
	return bits
}

// parseProxmoxSize parses a size string like "2c-2g" into cores and memory in MB.
func parseProxmoxSize(size string) (cores, memMB int) {
	parts := strings.Split(size, "-")
	if len(parts) != 2 {
		return 0, 0
	}
	coreStr := strings.TrimSuffix(parts[0], "c")
	memStr := strings.TrimSuffix(parts[1], "g")
	cores, _ = strconv.Atoi(coreStr)
	memGB, _ := strconv.Atoi(memStr)
	return cores, memGB * 1024
}
