package proxmox

import (
	"testing"

	"tackle/internal/providers/credentials"
)

func TestNewCloudClient_RequiresHost(t *testing.T) {
	_, err := NewCloudClient(credentials.ProxmoxCredentials{})
	if err == nil {
		t.Error("expected error for empty host")
	}
}

func TestNewCloudClient_DefaultPort(t *testing.T) {
	c, err := NewCloudClient(credentials.ProxmoxCredentials{
		Host:        "pve.example.com",
		TokenID:     "user@pam!token",
		TokenSecret: "secret",
		Node:        "pve1",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if c.baseURL != "https://pve.example.com:8006/api2/json" {
		t.Errorf("unexpected baseURL: %s", c.baseURL)
	}
}

func TestNewCloudClient_CustomPort(t *testing.T) {
	c, err := NewCloudClient(credentials.ProxmoxCredentials{
		Host: "pve.example.com",
		Port: 9006,
		Node: "pve1",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if c.baseURL != "https://pve.example.com:9006/api2/json" {
		t.Errorf("unexpected baseURL: %s", c.baseURL)
	}
}

func TestValidateInstanceSize(t *testing.T) {
	c := &CloudClient{}
	tests := []struct {
		size  string
		valid bool
	}{
		{"2c-2g", true},
		{"4c-8g", true},
		{"1c-1g", true},
		{"16c-32g", true},
		{"invalid", false},
		{"2-2", false},
		{"c-g", false},
		{"", false},
		{"2c-", false},
	}
	for _, tc := range tests {
		t.Run(tc.size, func(t *testing.T) {
			if got := c.ValidateInstanceSize(tc.size); got != tc.valid {
				t.Errorf("ValidateInstanceSize(%q) = %v, want %v", tc.size, got, tc.valid)
			}
		})
	}
}
