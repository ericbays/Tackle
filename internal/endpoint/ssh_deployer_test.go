package endpoint

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	"golang.org/x/crypto/ssh"
)

// mockSSHClientFactory implements SSHClientFactory for testing.
type mockSSHClientFactory struct {
	dialErr error
	client  *mockSSHClient
}

func (f *mockSSHClientFactory) Dial(ctx context.Context, addr string, config *ssh.ClientConfig) (SSHClient, error) {
	if f.dialErr != nil {
		return nil, f.dialErr
	}
	return f.client, nil
}

// mockSSHClient implements SSHClient for testing.
type mockSSHClient struct {
	mu          sync.Mutex
	uploads     map[string][]byte
	uploadPerms map[string]uint32
	commands    []string
	runErr      error
	uploadErr   error
	closed      bool
}

func newMockSSHClient() *mockSSHClient {
	return &mockSSHClient{
		uploads:     make(map[string][]byte),
		uploadPerms: make(map[string]uint32),
	}
}

func (c *mockSSHClient) Upload(ctx context.Context, remotePath string, data []byte, mode uint32) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.uploadErr != nil {
		return c.uploadErr
	}
	c.uploads[remotePath] = data
	c.uploadPerms[remotePath] = mode
	return nil
}

func (c *mockSSHClient) Run(ctx context.Context, cmd string) ([]byte, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.commands = append(c.commands, cmd)
	if c.runErr != nil {
		return nil, c.runErr
	}
	return []byte("ok"), nil
}

func (c *mockSSHClient) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.closed = true
	return nil
}

func TestGenerateSystemdUnit(t *testing.T) {
	unit := generateSystemdUnit(9443)
	s := string(unit)

	checks := []string{
		"[Unit]",
		"[Service]",
		"[Install]",
		"User=tackle",
		"ExecStart=/opt/tackle/proxy",
		"Restart=always",
		"CONTROL_PORT=9443",
		"NoNewPrivileges=true",
		"ProtectSystem=strict",
		"WantedBy=multi-user.target",
	}
	for _, check := range checks {
		if !strings.Contains(s, check) {
			t.Errorf("systemd unit missing: %s", check)
		}
	}
}

func TestGenerateSystemdUnitDifferentPort(t *testing.T) {
	unit := generateSystemdUnit(8443)
	if !strings.Contains(string(unit), "CONTROL_PORT=8443") {
		t.Error("systemd unit should contain the specified control port")
	}
}

func TestMockSSHClientUpload(t *testing.T) {
	client := newMockSSHClient()
	ctx := context.Background()

	data := []byte("hello world")
	if err := client.Upload(ctx, "/tmp/test", data, 0644); err != nil {
		t.Fatalf("upload: %v", err)
	}

	if string(client.uploads["/tmp/test"]) != "hello world" {
		t.Error("upload data mismatch")
	}
	if client.uploadPerms["/tmp/test"] != 0644 {
		t.Error("upload permissions mismatch")
	}
}

func TestMockSSHClientRun(t *testing.T) {
	client := newMockSSHClient()
	ctx := context.Background()

	output, err := client.Run(ctx, "echo hello")
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	if string(output) != "ok" {
		t.Error("unexpected output")
	}
	if len(client.commands) != 1 || client.commands[0] != "echo hello" {
		t.Error("command not recorded")
	}
}

func TestMockSSHClientUploadError(t *testing.T) {
	client := newMockSSHClient()
	client.uploadErr = fmt.Errorf("disk full")
	ctx := context.Background()

	err := client.Upload(ctx, "/tmp/test", []byte("data"), 0644)
	if err == nil {
		t.Error("expected error")
	}
}

func TestMockSSHClientRunError(t *testing.T) {
	client := newMockSSHClient()
	client.runErr = fmt.Errorf("command failed")
	ctx := context.Background()

	_, err := client.Run(ctx, "bad command")
	if err == nil {
		t.Error("expected error")
	}
}

func TestMockSSHClientClose(t *testing.T) {
	client := newMockSSHClient()
	client.Close()
	if !client.closed {
		t.Error("client should be marked as closed")
	}
}

func TestMockSSHFactoryDialError(t *testing.T) {
	factory := &mockSSHClientFactory{dialErr: fmt.Errorf("connection refused")}
	_, err := factory.Dial(context.Background(), "1.2.3.4:22", nil)
	if err == nil {
		t.Error("expected dial error")
	}
}

func TestWaitForHeartbeatTimeout(t *testing.T) {
	// This test verifies the heartbeat timeout mechanism works
	// without needing a real DB. We use a short timeout.
	d := &SSHDeployer{
		HeartbeatTimeout: 100 * time.Millisecond,
	}

	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()

	// No repo set, so GetByID will fail, simulating no heartbeat
	// The function should return a timeout error
	err := d.waitForHeartbeat(ctx, "test-endpoint")
	if err == nil {
		t.Error("expected timeout error")
	}
}
