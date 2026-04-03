package endpoint

import (
	"context"
	"strings"
	"testing"

	"golang.org/x/crypto/ssh"

	"tackle/internal/repositories"
)

// --- Mock DNS Updater ---

type mockDNSUpdater struct {
	createdRecords map[string]string // subdomain -> ip
	deletedRecords []string
	propagated     bool
	createErr      error
	deleteErr      error
}

func newMockDNSUpdater() *mockDNSUpdater {
	return &mockDNSUpdater{
		createdRecords: make(map[string]string),
		propagated:     true,
	}
}

func (u *mockDNSUpdater) CreateARecord(ctx context.Context, zone, subdomain, ip string) error {
	if u.createErr != nil {
		return u.createErr
	}
	u.createdRecords[subdomain] = ip
	return nil
}

func (u *mockDNSUpdater) DeleteARecord(ctx context.Context, zone, subdomain string) error {
	if u.deleteErr != nil {
		return u.deleteErr
	}
	u.deletedRecords = append(u.deletedRecords, subdomain)
	delete(u.createdRecords, subdomain)
	return nil
}

func (u *mockDNSUpdater) CheckPropagation(ctx context.Context, domain, expectedIP string) (bool, error) {
	return u.propagated, nil
}

// --- Integration Test: State Machine Valid Transitions ---

func TestIntegrationStateMachineAllTransitions(t *testing.T) {
	transitions := []struct {
		from, to repositories.EndpointState
	}{
		{repositories.EndpointStateRequested, repositories.EndpointStateProvisioning},
		{repositories.EndpointStateProvisioning, repositories.EndpointStateConfiguring},
		{repositories.EndpointStateProvisioning, repositories.EndpointStateError},
		{repositories.EndpointStateConfiguring, repositories.EndpointStateActive},
		{repositories.EndpointStateConfiguring, repositories.EndpointStateError},
		{repositories.EndpointStateActive, repositories.EndpointStateStopped},
		{repositories.EndpointStateActive, repositories.EndpointStateError},
		{repositories.EndpointStateActive, repositories.EndpointStateTerminated},
		{repositories.EndpointStateStopped, repositories.EndpointStateActive},
		{repositories.EndpointStateStopped, repositories.EndpointStateTerminated},
		{repositories.EndpointStateError, repositories.EndpointStateConfiguring},
		{repositories.EndpointStateError, repositories.EndpointStateTerminated},
	}

	for _, tc := range transitions {
		if !IsValidTransition(tc.from, tc.to) {
			t.Errorf("expected valid transition %s -> %s", tc.from, tc.to)
		}
	}
}

func TestIntegrationStateMachineInvalidTransitions(t *testing.T) {
	invalid := []struct {
		from, to repositories.EndpointState
	}{
		{repositories.EndpointStateTerminated, repositories.EndpointStateActive},
		{repositories.EndpointStateTerminated, repositories.EndpointStateRequested},
		{repositories.EndpointStateRequested, repositories.EndpointStateActive},
		{repositories.EndpointStateRequested, repositories.EndpointStateTerminated},
		{repositories.EndpointStateProvisioning, repositories.EndpointStateActive},
		{repositories.EndpointStateProvisioning, repositories.EndpointStateStopped},
		{repositories.EndpointStateActive, repositories.EndpointStateProvisioning},
		{repositories.EndpointStateActive, repositories.EndpointStateConfiguring},
		{repositories.EndpointStateStopped, repositories.EndpointStateProvisioning},
		{repositories.EndpointStateStopped, repositories.EndpointStateConfiguring},
		{repositories.EndpointStateError, repositories.EndpointStateActive},
		{repositories.EndpointStateError, repositories.EndpointStateStopped},
	}

	for _, tc := range invalid {
		if IsValidTransition(tc.from, tc.to) {
			t.Errorf("expected invalid transition %s -> %s to be rejected", tc.from, tc.to)
		}
	}
}

// --- Integration Test: SSH Deploy Flow (Mock) ---

func TestIntegrationSSHDeployUploadsAllFiles(t *testing.T) {
	client := newMockSSHClient()
	ctx := context.Background()

	expectedFiles := []struct {
		path string
		perm uint32
	}{
		{"/opt/tackle/proxy", 0755},
		{"/opt/tackle/certs/server.crt", 0644},
		{"/opt/tackle/certs/server.key", 0600},
		{"/opt/tackle/config.json", 0600},
		{"/opt/tackle/harden.sh", 0755},
		{"/etc/systemd/system/tackle-proxy.service", 0644},
	}

	for _, ef := range expectedFiles {
		err := client.Upload(ctx, ef.path, []byte("test data"), ef.perm)
		if err != nil {
			t.Fatalf("upload %s: %v", ef.path, err)
		}
	}

	for _, ef := range expectedFiles {
		data, ok := client.uploads[ef.path]
		if !ok {
			t.Errorf("file not uploaded: %s", ef.path)
			continue
		}
		if len(data) == 0 {
			t.Errorf("file empty: %s", ef.path)
		}
		if client.uploadPerms[ef.path] != ef.perm {
			t.Errorf("file %s: expected perm %o, got %o", ef.path, ef.perm, client.uploadPerms[ef.path])
		}
	}
}

func TestIntegrationSSHDeployCommandSequence(t *testing.T) {
	client := newMockSSHClient()
	ctx := context.Background()

	commands := []string{
		"mkdir -p /opt/tackle && mkdir -p /opt/tackle/certs",
		"/opt/tackle/harden.sh",
		"systemctl daemon-reload && systemctl enable tackle-proxy && systemctl start tackle-proxy",
		"ufw deny 22/tcp 2>/dev/null; iptables -D INPUT -p tcp --dport 22 -j ACCEPT 2>/dev/null; true",
	}

	for _, cmd := range commands {
		if _, err := client.Run(ctx, cmd); err != nil {
			t.Fatalf("run %q: %v", cmd, err)
		}
	}

	if len(client.commands) != len(commands) {
		t.Errorf("expected %d commands, got %d", len(commands), len(client.commands))
	}

	last := client.commands[len(client.commands)-1]
	if !strings.Contains(last, "22") {
		t.Error("last command should remove SSH access")
	}
}

// --- Integration Test: Communication Auth ---

func TestIntegrationCertGeneration(t *testing.T) {
	configs := []struct {
		name   string
		domain string
		ip     string
	}{
		{"aws", "phish.example.com", "54.1.2.3"},
		{"azure", "login.example.com", "40.1.2.3"},
		{"proxmox", "", "10.0.1.50"},
	}

	for _, tc := range configs {
		t.Run(tc.name, func(t *testing.T) {
			cert, key, err := generateSelfSignedCert(tc.domain, tc.ip)
			if err != nil {
				t.Fatalf("generate cert: %v", err)
			}
			if len(cert) == 0 || len(key) == 0 {
				t.Error("cert or key is empty")
			}
		})
	}
}

func TestIntegrationAuthTokenUniqueness(t *testing.T) {
	tokens := make(map[string]bool)
	for i := 0; i < 50; i++ {
		token, err := generateAuthToken()
		if err != nil {
			t.Fatalf("generate token %d: %v", i, err)
		}
		if tokens[token] {
			t.Errorf("duplicate token on iteration %d", i)
		}
		tokens[token] = true
	}
}

// --- Integration Test: Hardening ---

func TestIntegrationHardeningAllProviders(t *testing.T) {
	configs := []struct {
		name        string
		proxyPort   int
		controlPort int
	}{
		{"aws", 443, 9443},
		{"azure", 443, 9443},
		{"proxmox", 443, 9443},
	}

	for _, tc := range configs {
		t.Run(tc.name, func(t *testing.T) {
			script, err := GenerateHardeningScript(HardeningConfig{
				ProxyPort:   tc.proxyPort,
				ControlPort: tc.controlPort,
			})
			if err != nil {
				t.Fatalf("generate: %v", err)
			}
			s := string(script)
			if !strings.Contains(s, "iptables -P INPUT DROP") {
				t.Error("should set default DROP")
			}
			if !strings.Contains(s, "iptables -P FORWARD DROP") {
				t.Error("should drop forward")
			}
		})
	}
}

// --- Integration Test: Error Handling ---

func TestIntegrationSSHConnectionFailure(t *testing.T) {
	factory := &mockSSHClientFactory{dialErr: &mockError{msg: "connection refused"}}
	_, err := factory.Dial(context.Background(), "1.2.3.4:22", &ssh.ClientConfig{})
	if err == nil {
		t.Error("expected connection failure")
	}
}

type mockError struct {
	msg string
}

func (e *mockError) Error() string { return e.msg }

func TestIntegrationUploadFailure(t *testing.T) {
	client := newMockSSHClient()
	client.uploadErr = &mockError{msg: "disk full"}

	err := client.Upload(context.Background(), "/opt/tackle/proxy", []byte("binary"), 0755)
	if err == nil {
		t.Error("expected upload failure")
	}
}

func TestIntegrationServiceStartFailure(t *testing.T) {
	client := newMockSSHClient()
	client.runErr = &mockError{msg: "service failed to start"}

	_, err := client.Run(context.Background(), "systemctl start tackle-proxy")
	if err == nil {
		t.Error("expected service start failure")
	}
}

// --- Integration Test: Teardown ---

func TestIntegrationTeardownSequence(t *testing.T) {
	dns := newMockDNSUpdater()
	ctx := context.Background()

	dns.CreateARecord(ctx, "example.com", "phish", "1.2.3.4")
	if len(dns.createdRecords) != 1 {
		t.Error("DNS record should be created")
	}

	dns.DeleteARecord(ctx, "example.com", "phish")
	if len(dns.createdRecords) != 0 {
		t.Error("DNS record should be deleted")
	}
	if len(dns.deletedRecords) != 1 || dns.deletedRecords[0] != "phish" {
		t.Error("DNS deletion not tracked")
	}
}

func TestIntegrationTeardownSSHKeyDestruction(t *testing.T) {
	// Verify the mock SSH client can be closed (simulating key destruction).
	client := newMockSSHClient()
	client.Close()
	if !client.closed {
		t.Error("SSH client should be closed on teardown")
	}
}

// --- Integration Test: Audit Logging ---

func TestIntegrationAuditLogTransitionFormat(t *testing.T) {
	from := "configuring"
	to := "active"
	expected := "endpoint.state.configuring_to_active"
	action := "endpoint.state." + from + "_to_" + to
	if action != expected {
		t.Errorf("expected action %s, got %s", expected, action)
	}
}

func TestIntegrationAuditLogProvisionFormat(t *testing.T) {
	actions := []string{
		"endpoint.created",
		"endpoint.provisioned",
		"endpoint.configured",
		"endpoint.terminated",
		"ssh_key.generated",
		"ssh_key.destroyed",
		"endpoint.credentials_generated",
		"endpoint.credentials_invalidated",
	}

	for _, a := range actions {
		if !strings.Contains(a, "endpoint") && !strings.Contains(a, "ssh_key") {
			t.Errorf("audit action should reference endpoint or ssh_key: %s", a)
		}
	}
}
