package endpointmgmt

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"net"
	"testing"
	"time"
)

func generateTestCert(t *testing.T, domain string, notBefore, notAfter time.Time) (certPEM, keyPEM []byte) {
	t.Helper()
	priv, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}

	serial, _ := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 128))
	tmpl := x509.Certificate{
		SerialNumber: serial,
		Subject:      pkix.Name{CommonName: domain, Organization: []string{"Test"}},
		NotBefore:    notBefore,
		NotAfter:     notAfter,
		KeyUsage:     x509.KeyUsageDigitalSignature,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		DNSNames:     []string{domain},
	}
	if ip := net.ParseIP(domain); ip != nil {
		tmpl.IPAddresses = []net.IP{ip}
		tmpl.DNSNames = nil
	}

	certDER, err := x509.CreateCertificate(rand.Reader, &tmpl, &tmpl, &priv.PublicKey, priv)
	if err != nil {
		t.Fatalf("create cert: %v", err)
	}

	certPEM = pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: certDER})
	privDER, _ := x509.MarshalECPrivateKey(priv)
	keyPEM = pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: privDER})
	return certPEM, keyPEM
}

func TestParseTLSCertificate_Valid(t *testing.T) {
	certPEM, _ := generateTestCert(t, "phish.example.com",
		time.Now().Add(-1*time.Hour), time.Now().Add(365*24*time.Hour))

	parsed, err := parseTLSCertificate(certPEM)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if parsed.fingerprint == "" {
		t.Error("fingerprint should not be empty")
	}
	if len(parsed.dnsNames) == 0 {
		t.Error("dnsNames should not be empty")
	}
	if parsed.dnsNames[0] != "phish.example.com" {
		t.Errorf("expected DNS name phish.example.com, got %s", parsed.dnsNames[0])
	}
	if parsed.notAfter.Before(time.Now()) {
		t.Error("certificate should not be expired")
	}
}

func TestParseTLSCertificate_InvalidPEM(t *testing.T) {
	_, err := parseTLSCertificate([]byte("not a pem"))
	if err == nil {
		t.Error("expected error for invalid PEM")
	}
}

func TestParseTLSCertificate_WrongBlockType(t *testing.T) {
	block := pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: []byte("fake")})
	_, err := parseTLSCertificate(block)
	if err == nil {
		t.Error("expected error for non-certificate PEM block")
	}
}

func TestParsedCert_MatchesDomain(t *testing.T) {
	tests := []struct {
		name     string
		dnsNames []string
		domain   string
		match    bool
	}{
		{"exact match", []string{"example.com"}, "example.com", true},
		{"case insensitive", []string{"Example.COM"}, "example.com", true},
		{"no match", []string{"other.com"}, "example.com", false},
		{"wildcard match", []string{"*.example.com"}, "sub.example.com", true},
		{"wildcard no match apex", []string{"*.example.com"}, "example.com", false},
		{"multiple names", []string{"a.com", "b.com", "c.com"}, "b.com", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := &parsedCert{dnsNames: tt.dnsNames}
			if got := c.matchesDomain(tt.domain); got != tt.match {
				t.Errorf("matchesDomain(%q) = %v, want %v", tt.domain, got, tt.match)
			}
		})
	}
}

func TestEncodeDecodeJSON(t *testing.T) {
	original := map[string]string{"Host": "example.com", "User-Agent": "test/1.0"}
	data, err := encodeJSON(original)
	if err != nil {
		t.Fatalf("encodeJSON: %v", err)
	}

	var decoded map[string]string
	if err := decodeJSON(data, &decoded); err != nil {
		t.Fatalf("decodeJSON: %v", err)
	}

	if decoded["Host"] != "example.com" {
		t.Errorf("expected Host=example.com, got %s", decoded["Host"])
	}
}
