package endpoint

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/hex"
	"encoding/pem"
	"fmt"
	"math/big"
	"net"
	"time"

	"tackle/internal/crypto"
	"tackle/internal/repositories"
	"tackle/internal/services/audit"
)

// CommCredentials holds the generated communication credentials for an endpoint.
type CommCredentials struct {
	TLSCertPEM    []byte // Self-signed TLS certificate for the endpoint
	TLSKeyPEM     []byte // TLS private key
	AuthToken     string // Pre-shared bearer token for the data channel
	ControlPort   int    // Assigned control channel port
}

// CommAuthService handles generation and lifecycle of endpoint communication credentials.
type CommAuthService struct {
	repo     *repositories.PhishingEndpointRepository
	encSvc   *crypto.EncryptionService
	auditSvc *audit.AuditService
}

// NewCommAuthService creates a new CommAuthService.
func NewCommAuthService(
	repo *repositories.PhishingEndpointRepository,
	encSvc *crypto.EncryptionService,
	auditSvc *audit.AuditService,
) *CommAuthService {
	return &CommAuthService{repo: repo, encSvc: encSvc, auditSvc: auditSvc}
}

// GenerateCredentials creates all communication credentials for an endpoint:
// a self-signed TLS certificate, a bearer token for the data channel, and assigns
// a control port. Credentials are stored encrypted in the database.
func (s *CommAuthService) GenerateCredentials(ctx context.Context, endpointID, domain, publicIP string) (CommCredentials, error) {
	// Generate TLS certificate and key.
	certPEM, keyPEM, err := generateSelfSignedCert(domain, publicIP)
	if err != nil {
		return CommCredentials{}, fmt.Errorf("comm auth: generate cert: %w", err)
	}

	// Generate bearer token for data channel.
	authToken, err := generateAuthToken()
	if err != nil {
		return CommCredentials{}, fmt.Errorf("comm auth: generate token: %w", err)
	}

	// Assign control port (fixed offset from base to avoid conflicts).
	controlPort := 9443

	// Encrypt and store auth token.
	encToken, err := s.encSvc.Encrypt([]byte(authToken))
	if err != nil {
		return CommCredentials{}, fmt.Errorf("comm auth: encrypt token: %w", err)
	}

	if err := s.repo.UpdateControlInfo(ctx, endpointID, controlPort, encToken); err != nil {
		return CommCredentials{}, fmt.Errorf("comm auth: store control info: %w", err)
	}

	// Audit log.
	resourceType := "phishing_endpoint"
	_ = s.auditSvc.Log(ctx, audit.LogEntry{
		Category:     audit.CategoryInfrastructure,
		Severity:     audit.SeverityInfo,
		ActorType:    audit.ActorTypeSystem,
		ActorLabel:   "system",
		Action:       "endpoint.credentials_generated",
		ResourceType: &resourceType,
		ResourceID:   &endpointID,
		Details: map[string]any{
			"control_port": controlPort,
			"domain":       domain,
		},
	})

	return CommCredentials{
		TLSCertPEM:  certPEM,
		TLSKeyPEM:   keyPEM,
		AuthToken:   authToken,
		ControlPort: controlPort,
	}, nil
}

// InvalidateCredentials clears the auth token and control info for a terminated endpoint.
func (s *CommAuthService) InvalidateCredentials(ctx context.Context, endpointID string) error {
	// Zero out the auth token and control port.
	if err := s.repo.UpdateControlInfo(ctx, endpointID, 0, nil); err != nil {
		return fmt.Errorf("comm auth: invalidate: %w", err)
	}

	resourceType := "phishing_endpoint"
	_ = s.auditSvc.Log(ctx, audit.LogEntry{
		Category:     audit.CategoryInfrastructure,
		Severity:     audit.SeverityInfo,
		ActorType:    audit.ActorTypeSystem,
		ActorLabel:   "system",
		Action:       "endpoint.credentials_invalidated",
		ResourceType: &resourceType,
		ResourceID:   &endpointID,
	})

	return nil
}

// ValidateAuthToken checks whether a request's bearer token matches the stored
// encrypted token for the given endpoint. Returns true if valid.
func (s *CommAuthService) ValidateAuthToken(ctx context.Context, endpointID, token string) (bool, error) {
	ep, err := s.repo.GetByID(ctx, endpointID)
	if err != nil {
		return false, fmt.Errorf("comm auth: get endpoint: %w", err)
	}

	if len(ep.AuthToken) == 0 {
		return false, nil // No token stored — credentials invalidated.
	}

	storedToken, err := s.encSvc.Decrypt(ep.AuthToken)
	if err != nil {
		return false, fmt.Errorf("comm auth: decrypt token: %w", err)
	}

	// Constant-time comparison to prevent timing attacks.
	tokenHash := sha256.Sum256([]byte(token))
	storedHash := sha256.Sum256(storedToken)
	return tokenHash == storedHash, nil
}

// generateSelfSignedCert creates a self-signed TLS certificate for the endpoint.
func generateSelfSignedCert(domain, ipAddr string) (certPEM, keyPEM []byte, err error) {
	priv, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return nil, nil, fmt.Errorf("generate key: %w", err)
	}

	serialNumber, err := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 128))
	if err != nil {
		return nil, nil, fmt.Errorf("generate serial: %w", err)
	}

	template := x509.Certificate{
		SerialNumber: serialNumber,
		Subject: pkix.Name{
			Organization: []string{"Self-Signed"},
			CommonName:   domain,
		},
		NotBefore:             time.Now().Add(-1 * time.Hour),
		NotAfter:              time.Now().Add(365 * 24 * time.Hour),
		KeyUsage:              x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		BasicConstraintsValid: true,
	}

	if domain != "" {
		template.DNSNames = []string{domain}
	}
	if ipAddr != "" {
		if ip := net.ParseIP(ipAddr); ip != nil {
			template.IPAddresses = []net.IP{ip}
		}
	}

	certDER, err := x509.CreateCertificate(rand.Reader, &template, &template, &priv.PublicKey, priv)
	if err != nil {
		return nil, nil, fmt.Errorf("create certificate: %w", err)
	}

	certPEM = pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: certDER})

	privDER, err := x509.MarshalECPrivateKey(priv)
	if err != nil {
		return nil, nil, fmt.Errorf("marshal private key: %w", err)
	}
	keyPEM = pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: privDER})

	return certPEM, keyPEM, nil
}

// generateAuthToken creates a cryptographically secure bearer token.
func generateAuthToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("read random bytes: %w", err)
	}
	return hex.EncodeToString(b), nil
}
