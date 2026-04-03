// Package endpoint provides SSH-based configuration deployment to phishing endpoints.
package endpoint

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/pem"
	"fmt"
	"log/slog"
	"time"

	"golang.org/x/crypto/ssh"

	"tackle/internal/crypto"
	"tackle/internal/repositories"
	"tackle/internal/services/audit"
)

// SSHDeployConfig holds all parameters needed to deploy configuration to an endpoint.
type SSHDeployConfig struct {
	EndpointID    string
	CampaignID    string
	PublicIP      string
	SSHPort       int
	BinaryPath    string // Local path to compiled proxy binary
	BinaryHash    string
	TLSCertPEM    []byte // TLS certificate for the endpoint
	TLSKeyPEM     []byte // TLS private key for the endpoint
	ProxyConfig   []byte // Proxy configuration file contents
	ControlPort   int    // Port for the control channel
	AuthToken     string // Pre-shared auth token for communication
	HardeningScript []byte // Script to harden the endpoint VM
}

// SSHDeployer handles SSH-based configuration deployment to phishing endpoints.
type SSHDeployer struct {
	repo      *repositories.PhishingEndpointRepository
	sm        *StateMachine
	encSvc    *crypto.EncryptionService
	auditSvc  *audit.AuditService
	sshClient SSHClientFactory // Abstraction for testing

	// Configurable timeouts.
	DeployTimeout    time.Duration
	HeartbeatTimeout time.Duration
}

// SSHClientFactory creates SSH client connections. Abstracted for testing.
type SSHClientFactory interface {
	// Dial establishes an SSH connection to the given address.
	Dial(ctx context.Context, addr string, config *ssh.ClientConfig) (SSHClient, error)
}

// SSHClient represents an SSH client session. Abstracted for testing.
type SSHClient interface {
	// Upload transfers a file to the remote host.
	Upload(ctx context.Context, remotePath string, data []byte, mode uint32) error
	// Run executes a command on the remote host and returns combined output.
	Run(ctx context.Context, cmd string) ([]byte, error)
	// Close closes the SSH connection.
	Close() error
}

// NewSSHDeployer creates a new SSHDeployer.
func NewSSHDeployer(
	repo *repositories.PhishingEndpointRepository,
	sm *StateMachine,
	encSvc *crypto.EncryptionService,
	auditSvc *audit.AuditService,
	sshClient SSHClientFactory,
) *SSHDeployer {
	return &SSHDeployer{
		repo:             repo,
		sm:               sm,
		encSvc:           encSvc,
		auditSvc:         auditSvc,
		sshClient:        sshClient,
		DeployTimeout:    5 * time.Minute,
		HeartbeatTimeout: 2 * time.Minute,
	}
}

// GenerateSSHKeyPair generates an Ed25519 SSH key pair, encrypts the private key,
// and stores both in the database. Returns the key record and raw public key string.
func (d *SSHDeployer) GenerateSSHKeyPair(ctx context.Context, campaignID string) (repositories.EndpointSSHKey, string, error) {
	// Generate Ed25519 key pair.
	pubKey, privKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return repositories.EndpointSSHKey{}, "", fmt.Errorf("ssh deployer: generate key: %w", err)
	}

	// Marshal public key to authorized_keys format.
	sshPub, err := ssh.NewPublicKey(pubKey)
	if err != nil {
		return repositories.EndpointSSHKey{}, "", fmt.Errorf("ssh deployer: marshal public key: %w", err)
	}
	pubKeyStr := string(ssh.MarshalAuthorizedKey(sshPub))

	// Marshal private key to PEM.
	privKeyBytes, err := ssh.MarshalPrivateKey(privKey, "")
	if err != nil {
		return repositories.EndpointSSHKey{}, "", fmt.Errorf("ssh deployer: marshal private key: %w", err)
	}
	privKeyPEM := pem.EncodeToMemory(privKeyBytes)

	// Encrypt private key for storage.
	encPrivKey, err := d.encSvc.Encrypt(privKeyPEM)
	if err != nil {
		return repositories.EndpointSSHKey{}, "", fmt.Errorf("ssh deployer: encrypt private key: %w", err)
	}

	// Store in database.
	key, err := d.repo.CreateSSHKey(ctx, repositories.EndpointSSHKey{
		CampaignID:          campaignID,
		PublicKey:           pubKeyStr,
		PrivateKeyEncrypted: encPrivKey,
	})
	if err != nil {
		return repositories.EndpointSSHKey{}, "", fmt.Errorf("ssh deployer: store key: %w", err)
	}

	// Audit log.
	resourceType := "endpoint_ssh_key"
	_ = d.auditSvc.Log(ctx, audit.LogEntry{
		Category:     audit.CategoryInfrastructure,
		Severity:     audit.SeverityInfo,
		ActorType:    audit.ActorTypeSystem,
		ActorLabel:   "system",
		Action:       "ssh_key.generated",
		ResourceType: &resourceType,
		ResourceID:   &key.ID,
		Details: map[string]any{
			"campaign_id": campaignID,
		},
	})

	return key, pubKeyStr, nil
}

// Deploy transfers the proxy binary, TLS certificates, configuration, and hardening
// script to the endpoint via SSH, then starts the proxy as a systemd service.
// It waits for the first heartbeat before returning success.
func (d *SSHDeployer) Deploy(ctx context.Context, sshKey repositories.EndpointSSHKey, config SSHDeployConfig) error {
	// Set overall deploy timeout.
	deployCtx, cancel := context.WithTimeout(ctx, d.DeployTimeout)
	defer cancel()

	// Decrypt the SSH private key.
	privKeyPEM, err := d.encSvc.Decrypt(sshKey.PrivateKeyEncrypted)
	if err != nil {
		d.transitionToError(ctx, config.EndpointID, "failed to decrypt SSH key")
		return fmt.Errorf("ssh deployer: decrypt key: %w", err)
	}

	// Parse the private key for SSH auth.
	signer, err := ssh.ParsePrivateKey(privKeyPEM)
	if err != nil {
		d.transitionToError(ctx, config.EndpointID, "failed to parse SSH key")
		return fmt.Errorf("ssh deployer: parse key: %w", err)
	}

	// Connect via SSH.
	sshConfig := &ssh.ClientConfig{
		User: "root",
		Auth: []ssh.AuthMethod{
			ssh.PublicKeys(signer),
		},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
		Timeout:         30 * time.Second,
	}

	sshPort := config.SSHPort
	if sshPort == 0 {
		sshPort = 22
	}
	addr := fmt.Sprintf("%s:%d", config.PublicIP, sshPort)

	client, err := d.sshClient.Dial(deployCtx, addr, sshConfig)
	if err != nil {
		d.transitionToError(ctx, config.EndpointID, "SSH connection failed: "+err.Error())
		return fmt.Errorf("ssh deployer: connect: %w", err)
	}
	defer client.Close()

	// Step 1: Create deploy directory.
	if _, err := client.Run(deployCtx, "mkdir -p /opt/tackle && mkdir -p /opt/tackle/certs"); err != nil {
		d.transitionToError(ctx, config.EndpointID, "failed to create deploy directory")
		return fmt.Errorf("ssh deployer: mkdir: %w", err)
	}

	// Step 2: Upload binary.
	binaryData, err := readFileBytes(config.BinaryPath)
	if err != nil {
		d.transitionToError(ctx, config.EndpointID, "failed to read binary: "+err.Error())
		return fmt.Errorf("ssh deployer: read binary: %w", err)
	}
	if err := client.Upload(deployCtx, "/opt/tackle/proxy", binaryData, 0755); err != nil {
		d.transitionToError(ctx, config.EndpointID, "failed to upload binary")
		return fmt.Errorf("ssh deployer: upload binary: %w", err)
	}

	// Step 3: Upload TLS cert and key.
	if err := client.Upload(deployCtx, "/opt/tackle/certs/server.crt", config.TLSCertPEM, 0644); err != nil {
		d.transitionToError(ctx, config.EndpointID, "failed to upload TLS cert")
		return fmt.Errorf("ssh deployer: upload cert: %w", err)
	}
	if err := client.Upload(deployCtx, "/opt/tackle/certs/server.key", config.TLSKeyPEM, 0600); err != nil {
		d.transitionToError(ctx, config.EndpointID, "failed to upload TLS key")
		return fmt.Errorf("ssh deployer: upload key: %w", err)
	}

	// Step 4: Upload proxy configuration.
	if err := client.Upload(deployCtx, "/opt/tackle/config.json", config.ProxyConfig, 0600); err != nil {
		d.transitionToError(ctx, config.EndpointID, "failed to upload proxy config")
		return fmt.Errorf("ssh deployer: upload config: %w", err)
	}

	// Step 5: Upload and run hardening script.
	if len(config.HardeningScript) > 0 {
		if err := client.Upload(deployCtx, "/opt/tackle/harden.sh", config.HardeningScript, 0755); err != nil {
			d.transitionToError(ctx, config.EndpointID, "failed to upload hardening script")
			return fmt.Errorf("ssh deployer: upload hardening script: %w", err)
		}
		if output, err := client.Run(deployCtx, "/opt/tackle/harden.sh"); err != nil {
			slog.Warn("ssh deployer: hardening script warnings", "output", string(output), "endpoint_id", config.EndpointID)
		}
	}

	// Step 6: Create systemd service and start it.
	serviceUnit := generateSystemdUnit(config.ControlPort)
	if err := client.Upload(deployCtx, "/etc/systemd/system/tackle-proxy.service", serviceUnit, 0644); err != nil {
		d.transitionToError(ctx, config.EndpointID, "failed to upload systemd unit")
		return fmt.Errorf("ssh deployer: upload service: %w", err)
	}

	if _, err := client.Run(deployCtx, "systemctl daemon-reload && systemctl enable tackle-proxy && systemctl start tackle-proxy"); err != nil {
		d.transitionToError(ctx, config.EndpointID, "failed to start proxy service")
		return fmt.Errorf("ssh deployer: start service: %w", err)
	}

	// Step 7: Update binary hash and control info in DB.
	if err := d.repo.UpdateBinaryHash(ctx, config.EndpointID, config.BinaryHash); err != nil {
		slog.Error("ssh deployer: update binary hash", "error", err, "endpoint_id", config.EndpointID)
	}
	authTokenEnc, err := d.encSvc.Encrypt([]byte(config.AuthToken))
	if err != nil {
		slog.Error("ssh deployer: encrypt auth token", "error", err, "endpoint_id", config.EndpointID)
	} else {
		if err := d.repo.UpdateControlInfo(ctx, config.EndpointID, config.ControlPort, authTokenEnc); err != nil {
			slog.Error("ssh deployer: update control info", "error", err, "endpoint_id", config.EndpointID)
		}
	}

	// Step 8: Link SSH key to endpoint.
	if err := d.repo.UpdateSSHKeyID(ctx, config.EndpointID, sshKey.ID); err != nil {
		slog.Error("ssh deployer: update ssh key id", "error", err, "endpoint_id", config.EndpointID)
	}

	// Step 9: Wait for first heartbeat.
	if err := d.waitForHeartbeat(deployCtx, config.EndpointID); err != nil {
		d.transitionToError(ctx, config.EndpointID, "heartbeat timeout: "+err.Error())
		return fmt.Errorf("ssh deployer: heartbeat: %w", err)
	}

	// Step 10: Remove SSH firewall rule (no SSH access after configuration).
	if _, err := client.Run(deployCtx, "ufw deny 22/tcp 2>/dev/null; iptables -D INPUT -p tcp --dport 22 -j ACCEPT 2>/dev/null; true"); err != nil {
		slog.Warn("ssh deployer: remove SSH rule failed", "error", err, "endpoint_id", config.EndpointID)
	}

	// Step 11: Transition to Active.
	if _, err := d.sm.TransitionSystem(ctx, config.EndpointID, repositories.EndpointStateActive, "configuration deployed, heartbeat received"); err != nil {
		return fmt.Errorf("ssh deployer: transition to active: %w", err)
	}

	// Audit log.
	resourceType := "phishing_endpoint"
	_ = d.auditSvc.Log(ctx, audit.LogEntry{
		Category:     audit.CategoryInfrastructure,
		Severity:     audit.SeverityInfo,
		ActorType:    audit.ActorTypeSystem,
		ActorLabel:   "system",
		Action:       "endpoint.configured",
		ResourceType: &resourceType,
		ResourceID:   &config.EndpointID,
		Details: map[string]any{
			"binary_hash":  config.BinaryHash,
			"control_port": config.ControlPort,
			"campaign_id":  config.CampaignID,
		},
	})

	return nil
}

// DestroySSHKey marks the SSH key as destroyed and logs the event.
func (d *SSHDeployer) DestroySSHKey(ctx context.Context, keyID string) error {
	if err := d.repo.DestroySSHKey(ctx, keyID); err != nil {
		return fmt.Errorf("ssh deployer: destroy key: %w", err)
	}

	resourceType := "endpoint_ssh_key"
	_ = d.auditSvc.Log(ctx, audit.LogEntry{
		Category:     audit.CategoryInfrastructure,
		Severity:     audit.SeverityInfo,
		ActorType:    audit.ActorTypeSystem,
		ActorLabel:   "system",
		Action:       "ssh_key.destroyed",
		ResourceType: &resourceType,
		ResourceID:   &keyID,
	})

	return nil
}

// waitForHeartbeat polls the database for a heartbeat from the endpoint.
func (d *SSHDeployer) waitForHeartbeat(ctx context.Context, endpointID string) error {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	timeout := time.After(d.HeartbeatTimeout)

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-timeout:
			return fmt.Errorf("no heartbeat received within %s", d.HeartbeatTimeout)
		case <-ticker.C:
			ep, err := d.repo.GetByID(ctx, endpointID)
			if err != nil {
				continue
			}
			if ep.LastHeartbeatAt != nil {
				return nil // Heartbeat received.
			}
		}
	}
}

func (d *SSHDeployer) transitionToError(ctx context.Context, endpointID, reason string) {
	if _, err := d.sm.TransitionSystem(ctx, endpointID, repositories.EndpointStateError, reason); err != nil {
		slog.Error("ssh deployer: failed to transition to error", "error", err, "endpoint_id", endpointID)
	}
}

// generateSystemdUnit creates a systemd unit file for the proxy binary.
func generateSystemdUnit(controlPort int) []byte {
	var buf bytes.Buffer
	buf.WriteString("[Unit]\n")
	buf.WriteString("Description=Tackle Proxy Service\n")
	buf.WriteString("After=network.target\n\n")
	buf.WriteString("[Service]\n")
	buf.WriteString("Type=simple\n")
	buf.WriteString("User=tackle\n")
	buf.WriteString("Group=tackle\n")
	buf.WriteString("WorkingDirectory=/opt/tackle\n")
	buf.WriteString("ExecStart=/opt/tackle/proxy\n")
	buf.WriteString("Restart=always\n")
	buf.WriteString("RestartSec=5\n")
	buf.WriteString("LimitNOFILE=65535\n")
	buf.WriteString(fmt.Sprintf("Environment=CONTROL_PORT=%d\n", controlPort))
	buf.WriteString("Environment=CONFIG_PATH=/opt/tackle/config.json\n")
	buf.WriteString("Environment=TLS_CERT=/opt/tackle/certs/server.crt\n")
	buf.WriteString("Environment=TLS_KEY=/opt/tackle/certs/server.key\n")
	buf.WriteString("NoNewPrivileges=true\n")
	buf.WriteString("ProtectSystem=strict\n")
	buf.WriteString("ReadWritePaths=/opt/tackle\n")
	buf.WriteString("PrivateTmp=true\n\n")
	buf.WriteString("[Install]\n")
	buf.WriteString("WantedBy=multi-user.target\n")
	return buf.Bytes()
}

// readFileBytes reads a file and returns its contents as bytes.
func readFileBytes(path string) ([]byte, error) {
	return readFileBytesFromDisk(path)
}
