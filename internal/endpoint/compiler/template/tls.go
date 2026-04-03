package main

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"log"
	"math/big"
	"net"
	"sync"
	"time"
)

// cryptoRandRead wraps crypto/rand.Read for use by other files.
var cryptoRandRead = rand.Read

// secondDuration is time.Second, declared here to avoid import cycles.
const secondDuration = time.Second

// certManager manages TLS certificates with hot-reload support.
// It starts with a self-signed cert and can be updated at runtime
// with operator-provided or ACME-obtained certificates.
type certManager struct {
	mu   sync.RWMutex
	cert *tls.Certificate
}

// newCertManager creates a certManager initialized with a self-signed certificate.
func newCertManager(domain string) (*certManager, error) {
	cert, err := generateSelfSignedCertForDomain(domain)
	if err != nil {
		return nil, fmt.Errorf("initial cert: %w", err)
	}
	return &certManager{cert: &cert}, nil
}

// GetCertificate returns the current certificate for use in tls.Config.GetCertificate.
func (cm *certManager) GetCertificate(hello *tls.ClientHelloInfo) (*tls.Certificate, error) {
	cm.mu.RLock()
	defer cm.mu.RUnlock()
	return cm.cert, nil
}

// UpdateFromPEM replaces the current certificate with one parsed from PEM-encoded cert and key.
func (cm *certManager) UpdateFromPEM(certPEM, keyPEM []byte) error {
	tlsCert, err := tls.X509KeyPair(certPEM, keyPEM)
	if err != nil {
		return fmt.Errorf("parse cert/key pair: %w", err)
	}

	// Parse the leaf for metadata.
	if len(tlsCert.Certificate) > 0 {
		leaf, err := x509.ParseCertificate(tlsCert.Certificate[0])
		if err == nil {
			tlsCert.Leaf = leaf
		}
	}

	cm.mu.Lock()
	cm.cert = &tlsCert
	cm.mu.Unlock()

	log.Printf("tls: certificate updated, expires %s", tlsCert.Leaf.NotAfter.Format(time.RFC3339))
	return nil
}

// Current returns the current certificate (for inspection).
func (cm *certManager) Current() *tls.Certificate {
	cm.mu.RLock()
	defer cm.mu.RUnlock()
	return cm.cert
}

// generateSelfSignedCert generates a self-signed ECDSA certificate for localhost.
func generateSelfSignedCert() (tls.Certificate, error) {
	return generateSelfSignedCertForDomain("")
}

// generateSelfSignedCertForDomain generates a self-signed ECDSA certificate.
func generateSelfSignedCertForDomain(domain string) (tls.Certificate, error) {
	priv, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return tls.Certificate{}, fmt.Errorf("generate key: %w", err)
	}

	serialNumber, err := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 128))
	if err != nil {
		return tls.Certificate{}, fmt.Errorf("generate serial: %w", err)
	}

	cn := "localhost"
	dnsNames := []string{"localhost"}
	ips := []net.IP{net.ParseIP("127.0.0.1")}

	if domain != "" {
		cn = domain
		dnsNames = append(dnsNames, domain)
	}

	template := x509.Certificate{
		SerialNumber: serialNumber,
		Subject: pkix.Name{
			CommonName: cn,
		},
		NotBefore:             time.Now().Add(-1 * time.Hour),
		NotAfter:              time.Now().Add(365 * 24 * time.Hour),
		KeyUsage:              x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		BasicConstraintsValid: true,
		DNSNames:              dnsNames,
		IPAddresses:           ips,
	}

	certDER, err := x509.CreateCertificate(rand.Reader, &template, &template, &priv.PublicKey, priv)
	if err != nil {
		return tls.Certificate{}, fmt.Errorf("create certificate: %w", err)
	}

	leaf, err := x509.ParseCertificate(certDER)
	if err != nil {
		return tls.Certificate{}, fmt.Errorf("parse certificate: %w", err)
	}

	return tls.Certificate{
		Certificate: [][]byte{certDER},
		PrivateKey:  priv,
		Leaf:        leaf,
	}, nil
}

// parsePEMCertificate parses a PEM-encoded certificate and returns metadata.
func parsePEMCertificate(certPEM []byte) (*x509.Certificate, error) {
	block, _ := pem.Decode(certPEM)
	if block == nil {
		return nil, fmt.Errorf("no PEM block found")
	}
	if block.Type != "CERTIFICATE" {
		return nil, fmt.Errorf("PEM block type %q is not CERTIFICATE", block.Type)
	}
	return x509.ParseCertificate(block.Bytes)
}

// getSupportedCipherSuites returns strong cipher suites only (TLS 1.2).
// TLS 1.3 cipher suites are always included by Go and cannot be configured.
func getSupportedCipherSuites() []uint16 {
	return []uint16{
		tls.TLS_ECDHE_ECDSA_WITH_AES_256_GCM_SHA384,
		tls.TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256,
		tls.TLS_ECDHE_ECDSA_WITH_CHACHA20_POLY1305_SHA256,
		tls.TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384,
		tls.TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256,
		tls.TLS_ECDHE_RSA_WITH_CHACHA20_POLY1305_SHA256,
	}
}

// buildTLSConfig creates a TLS configuration using the certManager for dynamic cert loading.
func buildTLSConfig(cm *certManager) *tls.Config {
	return &tls.Config{
		MinVersion:     tls.VersionTLS12,
		MaxVersion:     tls.VersionTLS13,
		CipherSuites:   getSupportedCipherSuites(),
		GetCertificate: cm.GetCertificate,
		NextProtos:     []string{"h2", "http/1.1"},
	}
}
