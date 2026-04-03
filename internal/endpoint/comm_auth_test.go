package endpoint

import (
	"crypto/x509"
	"encoding/pem"
	"net"
	"strings"
	"testing"
)

func TestGenerateSelfSignedCert(t *testing.T) {
	certPEM, keyPEM, err := generateSelfSignedCert("example.com", "1.2.3.4")
	if err != nil {
		t.Fatalf("generate cert: %v", err)
	}

	// Verify cert is valid PEM.
	block, _ := pem.Decode(certPEM)
	if block == nil || block.Type != "CERTIFICATE" {
		t.Fatal("certPEM is not a valid PEM CERTIFICATE block")
	}

	cert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		t.Fatalf("parse certificate: %v", err)
	}

	// Check domain.
	if len(cert.DNSNames) != 1 || cert.DNSNames[0] != "example.com" {
		t.Errorf("expected DNS name example.com, got %v", cert.DNSNames)
	}

	// Check IP.
	if len(cert.IPAddresses) != 1 || !cert.IPAddresses[0].Equal(net.ParseIP("1.2.3.4")) {
		t.Errorf("expected IP 1.2.3.4, got %v", cert.IPAddresses)
	}

	// Verify key is valid PEM.
	keyBlock, _ := pem.Decode(keyPEM)
	if keyBlock == nil || keyBlock.Type != "EC PRIVATE KEY" {
		t.Fatal("keyPEM is not a valid PEM EC PRIVATE KEY block")
	}

	// Verify key matches cert.
	key, err := x509.ParseECPrivateKey(keyBlock.Bytes)
	if err != nil {
		t.Fatalf("parse private key: %v", err)
	}

	if !key.PublicKey.Equal(cert.PublicKey) {
		t.Error("private key does not match certificate public key")
	}
}

func TestGenerateSelfSignedCertNoDomain(t *testing.T) {
	certPEM, _, err := generateSelfSignedCert("", "10.0.0.1")
	if err != nil {
		t.Fatalf("generate cert: %v", err)
	}

	block, _ := pem.Decode(certPEM)
	cert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		t.Fatalf("parse cert: %v", err)
	}

	if len(cert.DNSNames) != 0 {
		t.Errorf("expected no DNS names, got %v", cert.DNSNames)
	}
	if len(cert.IPAddresses) != 1 {
		t.Error("expected one IP address")
	}
}

func TestGenerateSelfSignedCertNoIP(t *testing.T) {
	certPEM, _, err := generateSelfSignedCert("phish.example.com", "")
	if err != nil {
		t.Fatalf("generate cert: %v", err)
	}

	block, _ := pem.Decode(certPEM)
	cert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		t.Fatalf("parse cert: %v", err)
	}

	if len(cert.DNSNames) != 1 || cert.DNSNames[0] != "phish.example.com" {
		t.Error("expected DNS name")
	}
	if len(cert.IPAddresses) != 0 {
		t.Error("expected no IP addresses")
	}
}

func TestGenerateSelfSignedCertUniqueness(t *testing.T) {
	cert1, _, err := generateSelfSignedCert("a.com", "1.1.1.1")
	if err != nil {
		t.Fatalf("first cert: %v", err)
	}

	cert2, _, err := generateSelfSignedCert("a.com", "1.1.1.1")
	if err != nil {
		t.Fatalf("second cert: %v", err)
	}

	// Each generation should produce a unique cert (different serial number).
	if string(cert1) == string(cert2) {
		t.Error("two certs should not be identical")
	}
}

func TestGenerateAuthToken(t *testing.T) {
	token1, err := generateAuthToken()
	if err != nil {
		t.Fatalf("generate token: %v", err)
	}

	if len(token1) != 64 { // 32 bytes = 64 hex chars
		t.Errorf("expected 64 char token, got %d", len(token1))
	}

	token2, err := generateAuthToken()
	if err != nil {
		t.Fatalf("generate token 2: %v", err)
	}

	if token1 == token2 {
		t.Error("tokens should be unique")
	}
}

func TestGenerateAuthTokenBatch(t *testing.T) {
	seen := make(map[string]bool)
	for i := 0; i < 100; i++ {
		token, err := generateAuthToken()
		if err != nil {
			t.Fatalf("generate token %d: %v", i, err)
		}
		if seen[token] {
			t.Errorf("duplicate token on iteration %d", i)
		}
		seen[token] = true

		// Verify hex format.
		for _, c := range token {
			if !strings.ContainsRune("0123456789abcdef", c) {
				t.Errorf("token contains non-hex char: %c", c)
			}
		}
	}
}
