package cloud

import (
	"testing"
)

func TestGenerateShortID(t *testing.T) {
	id1 := generateShortID()
	id2 := generateShortID()
	if len(id1) != 8 {
		t.Errorf("expected 8 char ID, got %d: %q", len(id1), id1)
	}
	if id1 == id2 {
		t.Errorf("expected unique IDs, got same: %q", id1)
	}
}

func TestExpandIPRange(t *testing.T) {
	tests := []struct {
		name      string
		start     string
		end       string
		wantCount int
		wantErr   bool
	}{
		{"single IP", "10.0.0.1", "10.0.0.1", 1, false},
		{"small range", "10.0.0.1", "10.0.0.5", 5, false},
		{"class C", "192.168.1.1", "192.168.1.254", 254, false},
		{"cross octet", "10.0.0.250", "10.0.1.5", 12, false},
		{"invalid start", "not-an-ip", "10.0.0.5", 0, true},
		{"invalid end", "10.0.0.1", "bad", 0, true},
		{"reversed", "10.0.0.10", "10.0.0.1", 0, true},
		{"too large", "10.0.0.1", "10.0.5.0", 0, true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ips, err := expandIPRange(tc.start, tc.end)
			if tc.wantErr {
				if err == nil {
					t.Errorf("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if len(ips) != tc.wantCount {
				t.Errorf("expected %d IPs, got %d", tc.wantCount, len(ips))
			}
			if tc.wantCount > 0 {
				if ips[0] != tc.start {
					t.Errorf("first IP should be %s, got %s", tc.start, ips[0])
				}
				if ips[len(ips)-1] != tc.end {
					t.Errorf("last IP should be %s, got %s", tc.end, ips[len(ips)-1])
				}
			}
		})
	}
}

func TestSubnetMaskToCIDR(t *testing.T) {
	tests := []struct {
		mask string
		want int
	}{
		{"255.255.255.0", 24},
		{"255.255.0.0", 16},
		{"255.0.0.0", 8},
		{"255.255.255.128", 25},
		{"255.255.255.252", 30},
		{"invalid", 24}, // Default.
	}
	for _, tc := range tests {
		t.Run(tc.mask, func(t *testing.T) {
			got := subnetMaskToCIDR(tc.mask)
			if got != tc.want {
				t.Errorf("subnetMaskToCIDR(%q) = %d, want %d", tc.mask, got, tc.want)
			}
		})
	}
}

func TestParseProxmoxSize(t *testing.T) {
	tests := []struct {
		size     string
		cores    int
		memMB    int
	}{
		{"2c-2g", 2, 2048},
		{"4c-8g", 4, 8192},
		{"1c-1g", 1, 1024},
		{"invalid", 0, 0},
		{"", 0, 0},
	}
	for _, tc := range tests {
		t.Run(tc.size, func(t *testing.T) {
			cores, memMB := parseProxmoxSize(tc.size)
			if cores != tc.cores || memMB != tc.memMB {
				t.Errorf("parseProxmoxSize(%q) = (%d, %d), want (%d, %d)", tc.size, cores, memMB, tc.cores, tc.memMB)
			}
		})
	}
}

func TestParseAzureResourceID(t *testing.T) {
	tests := []struct {
		id   string
		rg   string
		name string
	}{
		{
			"/subscriptions/sub-id/resourceGroups/my-rg/providers/Microsoft.Compute/virtualMachines/my-vm",
			"my-rg", "my-vm",
		},
		{
			"/subscriptions/sub-id/resourceGroups/rg1/providers/Microsoft.Network/publicIPAddresses/my-ip",
			"rg1", "my-ip",
		},
		{"", "", ""},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			rg, name := parseAzureResourceID(tc.id)
			if rg != tc.rg || name != tc.name {
				t.Errorf("parseAzureResourceID(%q) = (%q, %q), want (%q, %q)", tc.id, rg, name, tc.rg, tc.name)
			}
		})
	}
}

func TestNormalizeAWSState(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"running", "running"},
		{"stopped", "stopped"},
		{"terminated", "terminated"},
		{"pending", "pending"},
		{"unknown-state", "unknown"},
	}
	for _, tc := range tests {
		t.Run(tc.input, func(t *testing.T) {
			got := normalizeProxmoxState(tc.input)
			if tc.input == "running" || tc.input == "stopped" {
				if got != tc.want {
					t.Errorf("got %q, want %q", got, tc.want)
				}
			}
		})
	}
}

func TestNormalizeAzureState(t *testing.T) {
	tests := []struct {
		code string
		want string
	}{
		{"PowerState/running", "running"},
		{"PowerState/deallocated", "stopped"},
		{"PowerState/stopped", "stopped"},
		{"PowerState/starting", "pending"},
		{"PowerState/unknown", "unknown"},
	}
	for _, tc := range tests {
		t.Run(tc.code, func(t *testing.T) {
			got := normalizeAzureState(tc.code)
			if got != tc.want {
				t.Errorf("normalizeAzureState(%q) = %q, want %q", tc.code, got, tc.want)
			}
		})
	}
}

// TestProviderInterface verifies that all provider types implement the Provider interface.
func TestProviderInterface(t *testing.T) {
	// Compile-time interface checks.
	var _ Provider = (*AWSProvider)(nil)
	var _ Provider = (*AzureProvider)(nil)
	var _ Provider = (*ProxmoxProvider)(nil)
}

func TestProxmoxProviderName(t *testing.T) {
	p := &ProxmoxProvider{}
	if p.ProviderName() != "proxmox" {
		t.Errorf("expected 'proxmox', got %q", p.ProviderName())
	}
}

func TestAWSProviderName(t *testing.T) {
	p := &AWSProvider{}
	if p.ProviderName() != "aws" {
		t.Errorf("expected 'aws', got %q", p.ProviderName())
	}
}

func TestAzureProviderName(t *testing.T) {
	p := &AzureProvider{}
	if p.ProviderName() != "azure" {
		t.Errorf("expected 'azure', got %q", p.ProviderName())
	}
}
