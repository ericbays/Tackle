package endpoint

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"time"

	"tackle/internal/repositories"
	"tackle/internal/services/audit"
)

// ACMEService handles Let's Encrypt certificate acquisition for phishing endpoints.
type ACMEService struct {
	repo     *repositories.PhishingEndpointRepository
	auditSvc *audit.AuditService

	// ACMEDirectory is the ACME directory URL (default: Let's Encrypt production).
	ACMEDirectory string
	// HTTPClient is used for ACME API calls. Default: http.DefaultClient.
	HTTPClient *http.Client
}

// NewACMEService creates a new ACMEService.
func NewACMEService(repo *repositories.PhishingEndpointRepository, auditSvc *audit.AuditService) *ACMEService {
	return &ACMEService{
		repo:          repo,
		auditSvc:      auditSvc,
		ACMEDirectory: "https://acme-v02.api.letsencrypt.org/directory",
		HTTPClient:    &http.Client{Timeout: 30 * time.Second},
	}
}

// ACMECertResult holds the result of a certificate acquisition.
type ACMECertResult struct {
	CertPEM    []byte    // Full certificate chain in PEM format.
	KeyPEM     []byte    // Private key in PEM format.
	Domain     string    // Domain the cert was issued for.
	Issuer     string    // Certificate issuer.
	NotBefore  time.Time // Validity start.
	NotAfter   time.Time // Validity end.
	SerialHex  string    // Serial number as hex string.
}

// TLSCertInfoJSON is the structure stored in phishing_endpoints.tls_cert_info JSONB.
type TLSCertInfoJSON struct {
	Domain    string    `json:"domain"`
	Issuer    string    `json:"issuer"`
	NotBefore time.Time `json:"not_before"`
	NotAfter  time.Time `json:"not_after"`
	Serial    string    `json:"serial"`
	Source    string    `json:"source"` // "acme", "upload", "self_signed"
	AcquiredAt time.Time `json:"acquired_at"`
}

// AcquireCertificate obtains a TLS certificate from Let's Encrypt via HTTP-01 challenge.
// The endpoint must be reachable on port 80 and serve the ACME challenge at
// /.well-known/acme-challenge/{token}.
//
// Steps:
//  1. Generate ECDSA private key
//  2. Create ACME account (or reuse)
//  3. Create order for the domain
//  4. Send challenge token to endpoint via management API
//  5. Tell ACME server to validate
//  6. Wait for validation
//  7. Finalize order with CSR
//  8. Download certificate
//  9. Store cert info in endpoint record
func (a *ACMEService) AcquireCertificate(ctx context.Context, domain string, endpointID string, mgmtSendChallenge func(ctx context.Context, endpointID, token, keyAuth string) error) (*ACMECertResult, error) {
	if domain == "" {
		return nil, fmt.Errorf("acme: domain is required")
	}
	if endpointID == "" {
		return nil, fmt.Errorf("acme: endpoint_id is required")
	}

	slog.Info("acme: starting certificate acquisition", "domain", domain, "endpoint_id", endpointID)

	// Generate ECDSA P-256 private key for the certificate.
	certKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return nil, fmt.Errorf("acme: generate cert key: %w", err)
	}

	keyDER, err := x509.MarshalECPrivateKey(certKey)
	if err != nil {
		return nil, fmt.Errorf("acme: marshal cert key: %w", err)
	}
	keyPEM := pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: keyDER})

	// Generate ACME account key.
	accountKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return nil, fmt.Errorf("acme: generate account key: %w", err)
	}

	// Step 1: Get ACME directory.
	directory, err := a.getDirectory(ctx)
	if err != nil {
		return nil, fmt.Errorf("acme: get directory: %w", err)
	}

	// Step 2: Get nonce.
	nonce, err := a.getNonce(ctx, directory.NewNonce)
	if err != nil {
		return nil, fmt.Errorf("acme: get nonce: %w", err)
	}

	// Step 3: Create account.
	accountURL, nonce, err := a.createAccount(ctx, directory.NewAccount, accountKey, nonce)
	if err != nil {
		return nil, fmt.Errorf("acme: create account: %w", err)
	}

	// Step 4: Create order.
	orderURL, finalizeURL, authzURLs, nonce, err := a.createOrder(ctx, directory.NewOrder, accountKey, accountURL, nonce, domain)
	if err != nil {
		return nil, fmt.Errorf("acme: create order: %w", err)
	}
	_ = orderURL

	// Step 5: Get authorization and challenge.
	if len(authzURLs) == 0 {
		return nil, fmt.Errorf("acme: no authorization URLs returned")
	}
	challengeURL, token, keyAuth, nonce, err := a.getHTTP01Challenge(ctx, authzURLs[0], accountKey, accountURL, nonce)
	if err != nil {
		return nil, fmt.Errorf("acme: get challenge: %w", err)
	}

	// Step 6: Send challenge to endpoint so it serves the response.
	if mgmtSendChallenge != nil {
		if err := mgmtSendChallenge(ctx, endpointID, token, keyAuth); err != nil {
			return nil, fmt.Errorf("acme: send challenge to endpoint: %w", err)
		}
	}

	// Step 7: Tell ACME to validate the challenge.
	nonce, err = a.respondToChallenge(ctx, challengeURL, accountKey, accountURL, nonce)
	if err != nil {
		return nil, fmt.Errorf("acme: respond to challenge: %w", err)
	}

	// Step 8: Poll for validation (max 60 seconds).
	pollCtx, cancel := context.WithTimeout(ctx, 60*time.Second)
	defer cancel()
	nonce, err = a.pollAuthorization(pollCtx, authzURLs[0], accountKey, accountURL, nonce)
	if err != nil {
		return nil, fmt.Errorf("acme: validation failed: %w", err)
	}

	// Step 9: Finalize order with CSR.
	certURL, nonce, err := a.finalizeOrder(ctx, finalizeURL, accountKey, accountURL, nonce, domain, certKey)
	if err != nil {
		return nil, fmt.Errorf("acme: finalize order: %w", err)
	}
	_ = nonce

	// Step 10: Download certificate.
	certPEM, err := a.downloadCertificate(ctx, certURL, accountKey, accountURL)
	if err != nil {
		return nil, fmt.Errorf("acme: download certificate: %w", err)
	}

	// Parse certificate for metadata.
	block, _ := pem.Decode(certPEM)
	if block == nil {
		return nil, fmt.Errorf("acme: failed to decode certificate PEM")
	}
	cert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		return nil, fmt.Errorf("acme: parse certificate: %w", err)
	}

	result := &ACMECertResult{
		CertPEM:   certPEM,
		KeyPEM:    keyPEM,
		Domain:    domain,
		Issuer:    cert.Issuer.CommonName,
		NotBefore: cert.NotBefore,
		NotAfter:  cert.NotAfter,
		SerialHex: cert.SerialNumber.Text(16),
	}

	// Store cert info in endpoint record.
	certInfo := TLSCertInfoJSON{
		Domain:     domain,
		Issuer:     cert.Issuer.CommonName,
		NotBefore:  cert.NotBefore,
		NotAfter:   cert.NotAfter,
		Serial:     cert.SerialNumber.Text(16),
		Source:     "acme",
		AcquiredAt: time.Now().UTC(),
	}
	certInfoJSON, _ := json.Marshal(certInfo)
	if err := a.repo.UpdateTLSCertInfo(ctx, endpointID, certInfoJSON); err != nil {
		slog.Warn("acme: failed to store cert info", "error", err, "endpoint_id", endpointID)
	}

	// Audit log.
	resourceType := "phishing_endpoint"
	_ = a.auditSvc.Log(ctx, audit.LogEntry{
		Category:     audit.CategoryInfrastructure,
		Severity:     audit.SeverityInfo,
		ActorType:    audit.ActorTypeSystem,
		ActorLabel:   "system",
		Action:       "endpoint.tls_cert_acquired",
		ResourceType: &resourceType,
		ResourceID:   &endpointID,
		Details: map[string]any{
			"domain":    domain,
			"issuer":    cert.Issuer.CommonName,
			"not_after": cert.NotAfter.Format(time.RFC3339),
			"source":    "acme",
		},
	})

	slog.Info("acme: certificate acquired", "domain", domain, "endpoint_id", endpointID, "expires", cert.NotAfter)
	return result, nil
}

// ShouldRenew checks if an endpoint's TLS cert should be renewed (< 7 days to expiry).
func (a *ACMEService) ShouldRenew(ctx context.Context, endpointID string) (bool, error) {
	ep, err := a.repo.GetByID(ctx, endpointID)
	if err != nil {
		return false, err
	}
	if len(ep.TLSCertInfo) == 0 {
		return false, nil // No cert info stored.
	}

	var info TLSCertInfoJSON
	if err := json.Unmarshal(ep.TLSCertInfo, &info); err != nil {
		return false, fmt.Errorf("acme: parse cert info: %w", err)
	}

	return time.Until(info.NotAfter) < 7*24*time.Hour, nil
}

// --- ACME protocol implementation ---

type acmeDirectory struct {
	NewNonce   string `json:"newNonce"`
	NewAccount string `json:"newAccount"`
	NewOrder   string `json:"newOrder"`
}

func (a *ACMEService) getDirectory(ctx context.Context) (*acmeDirectory, error) {
	req, _ := http.NewRequestWithContext(ctx, "GET", a.ACMEDirectory, nil)
	resp, err := a.HTTPClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	var dir acmeDirectory
	if err := json.NewDecoder(resp.Body).Decode(&dir); err != nil {
		return nil, err
	}
	return &dir, nil
}

func (a *ACMEService) getNonce(ctx context.Context, url string) (string, error) {
	req, _ := http.NewRequestWithContext(ctx, "HEAD", url, nil)
	resp, err := a.HTTPClient.Do(req)
	if err != nil {
		return "", err
	}
	resp.Body.Close()
	return resp.Header.Get("Replay-Nonce"), nil
}

// The remaining ACME protocol methods (createAccount, createOrder, getHTTP01Challenge,
// respondToChallenge, pollAuthorization, finalizeOrder, downloadCertificate) implement
// the full ACME v2 protocol using JWS-signed requests. They are intentionally stubbed
// to their interface signatures here, with the full JWS implementation deferred to
// a dedicated ACME client library (e.g., golang.org/x/crypto/acme) when integrating
// with real Let's Encrypt infrastructure.

func (a *ACMEService) createAccount(ctx context.Context, url string, key *ecdsa.PrivateKey, nonce string) (string, string, error) {
	// Full implementation would send JWS-signed POST to newAccount URL.
	// Returns: accountURL, newNonce, error.
	return "", nonce, fmt.Errorf("acme: account creation requires JWS implementation — use golang.org/x/crypto/acme in production")
}

func (a *ACMEService) createOrder(ctx context.Context, url string, key *ecdsa.PrivateKey, accountURL, nonce, domain string) (string, string, []string, string, error) {
	// Returns: orderURL, finalizeURL, authzURLs, newNonce, error.
	return "", "", nil, nonce, fmt.Errorf("acme: order creation requires JWS implementation")
}

func (a *ACMEService) getHTTP01Challenge(ctx context.Context, authzURL string, key *ecdsa.PrivateKey, accountURL, nonce string) (string, string, string, string, error) {
	// Returns: challengeURL, token, keyAuthorization, newNonce, error.
	return "", "", "", nonce, fmt.Errorf("acme: challenge retrieval requires JWS implementation")
}

func (a *ACMEService) respondToChallenge(ctx context.Context, challengeURL string, key *ecdsa.PrivateKey, accountURL, nonce string) (string, error) {
	// Returns: newNonce, error.
	return nonce, fmt.Errorf("acme: challenge response requires JWS implementation")
}

func (a *ACMEService) pollAuthorization(ctx context.Context, authzURL string, key *ecdsa.PrivateKey, accountURL, nonce string) (string, error) {
	// Returns: newNonce, error.
	return nonce, fmt.Errorf("acme: authorization polling requires JWS implementation")
}

func (a *ACMEService) finalizeOrder(ctx context.Context, finalizeURL string, key *ecdsa.PrivateKey, accountURL, nonce, domain string, certKey *ecdsa.PrivateKey) (string, string, error) {
	// Returns: certificateURL, newNonce, error.
	return "", nonce, fmt.Errorf("acme: order finalization requires JWS implementation")
}

func (a *ACMEService) downloadCertificate(ctx context.Context, certURL string, key *ecdsa.PrivateKey, accountURL string) ([]byte, error) {
	req, _ := http.NewRequestWithContext(ctx, "GET", certURL, nil)
	resp, err := a.HTTPClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	return io.ReadAll(resp.Body)
}
