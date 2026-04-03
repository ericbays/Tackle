// Package proxmox provides cloud infrastructure operations for Proxmox VE hypervisors.
package proxmox

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"tackle/internal/providers/credentials"
)

// CloudClient performs non-mutating Proxmox VE operations for credential and template validation.
type CloudClient struct {
	baseURL   string
	tokenID   string
	tokenSec  string
	node      string
	client    *http.Client
}

// NewCloudClient creates a Proxmox CloudClient from the provided credentials.
func NewCloudClient(creds credentials.ProxmoxCredentials) (*CloudClient, error) {
	if creds.Host == "" {
		return nil, fmt.Errorf("proxmox cloud client: host is required")
	}
	port := creds.Port
	if port == 0 {
		port = 8006
	}

	baseURL := fmt.Sprintf("https://%s:%d/api2/json", creds.Host, port)

	return &CloudClient{
		baseURL:  baseURL,
		tokenID:  creds.TokenID,
		tokenSec: creds.TokenSecret,
		node:     creds.Node,
		client: &http.Client{
			Timeout: 30 * time.Second,
			Transport: &http.Transport{
				TLSClientConfig: &tls.Config{
					InsecureSkipVerify: true, // Proxmox typically uses self-signed certs in lab environments.
				},
			},
		},
	}, nil
}

// TestConnection validates the API token by querying the cluster status endpoint.
func (c *CloudClient) TestConnection(ctx context.Context) error {
	resp, err := c.doRequest(ctx, "GET", "/cluster/status", nil)
	if err != nil {
		return fmt.Errorf("proxmox: connection test failed: %w", classifyProxmoxError(err))
	}
	defer resp.Body.Close()

	if resp.StatusCode == 401 || resp.StatusCode == 403 {
		return fmt.Errorf("proxmox: invalid API token or insufficient permissions")
	}
	if resp.StatusCode != 200 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("proxmox: unexpected status %d: %s", resp.StatusCode, string(body))
	}
	return nil
}

// ValidateRegion validates that the configured node name exists in the cluster.
func (c *CloudClient) ValidateRegion(region string) bool {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	resp, err := c.doRequest(ctx, "GET", "/nodes", nil)
	if err != nil {
		return false
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return false
	}

	var result struct {
		Data []struct {
			Node string `json:"node"`
		} `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return false
	}

	for _, n := range result.Data {
		if n.Node == region {
			return true
		}
	}
	return false
}

// ValidateInstanceSize validates a CPU/memory profile format (e.g., "2c-2g" for 2 cores, 2GB RAM).
func (c *CloudClient) ValidateInstanceSize(size string) bool {
	// Format: {cores}c-{memGB}g (e.g., "2c-2g", "4c-8g").
	parts := strings.Split(size, "-")
	if len(parts) != 2 {
		return false
	}
	if !strings.HasSuffix(parts[0], "c") || !strings.HasSuffix(parts[1], "g") {
		return false
	}
	coreStr := strings.TrimSuffix(parts[0], "c")
	memStr := strings.TrimSuffix(parts[1], "g")
	if coreStr == "" || memStr == "" {
		return false
	}
	for _, ch := range coreStr {
		if ch < '0' || ch > '9' {
			return false
		}
	}
	for _, ch := range memStr {
		if ch < '0' || ch > '9' {
			return false
		}
	}
	return true
}

// doRequest makes an authenticated HTTP request to the Proxmox API.
func (c *CloudClient) doRequest(ctx context.Context, method, path string, body io.Reader) (*http.Response, error) {
	url := c.baseURL + path
	req, err := http.NewRequestWithContext(ctx, method, url, body)
	if err != nil {
		return nil, fmt.Errorf("proxmox: create request: %w", err)
	}

	// PVE API token auth header format.
	req.Header.Set("Authorization", fmt.Sprintf("PVEAPIToken=%s=%s", c.tokenID, c.tokenSec))
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	return c.client.Do(req)
}

func classifyProxmoxError(err error) error {
	msg := err.Error()
	switch {
	case strings.Contains(msg, "no such host"):
		return fmt.Errorf("cannot resolve Proxmox host")
	case strings.Contains(msg, "connection refused"):
		return fmt.Errorf("Proxmox API connection refused: check host and port")
	case strings.Contains(msg, "dial tcp"):
		return fmt.Errorf("cannot reach Proxmox API: check network connectivity")
	case strings.Contains(msg, "tls"):
		return fmt.Errorf("TLS error connecting to Proxmox API")
	default:
		return err
	}
}
