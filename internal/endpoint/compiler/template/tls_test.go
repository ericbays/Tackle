package main

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/json"
	"encoding/pem"
	"math/big"
	"net"
	"net/http"
	"strings"
	"testing"
	"time"
)

func TestCertManagerInitialCert(t *testing.T) {
	cm, err := newCertManager("example.com")
	if err != nil {
		t.Fatalf("newCertManager: %v", err)
	}

	cert, err := cm.GetCertificate(nil)
	if err != nil {
		t.Fatalf("GetCertificate: %v", err)
	}
	if cert == nil {
		t.Fatal("cert is nil")
	}
	if cert.Leaf == nil {
		t.Fatal("cert.Leaf is nil")
	}
	if cert.Leaf.Subject.CommonName != "example.com" {
		t.Errorf("CN = %q, want example.com", cert.Leaf.Subject.CommonName)
	}
}

func TestCertManagerUpdateFromPEM(t *testing.T) {
	cm, err := newCertManager("")
	if err != nil {
		t.Fatalf("newCertManager: %v", err)
	}

	// Generate a new cert/key pair as PEM.
	certPEM, keyPEM := generateTestPEM(t, "new-domain.com")

	if err := cm.UpdateFromPEM(certPEM, keyPEM); err != nil {
		t.Fatalf("UpdateFromPEM: %v", err)
	}

	cert := cm.Current()
	if cert.Leaf.Subject.CommonName != "new-domain.com" {
		t.Errorf("CN after update = %q, want new-domain.com", cert.Leaf.Subject.CommonName)
	}
}

func TestCertManagerUpdateInvalidPEM(t *testing.T) {
	cm, err := newCertManager("")
	if err != nil {
		t.Fatalf("newCertManager: %v", err)
	}

	err = cm.UpdateFromPEM([]byte("not-a-cert"), []byte("not-a-key"))
	if err == nil {
		t.Error("expected error for invalid PEM")
	}
}

func TestCertManagerHotReload(t *testing.T) {
	cm, err := newCertManager("original.com")
	if err != nil {
		t.Fatalf("newCertManager: %v", err)
	}

	// Verify initial cert.
	cert1, _ := cm.GetCertificate(nil)
	if cert1.Leaf.Subject.CommonName != "original.com" {
		t.Fatalf("initial CN = %q", cert1.Leaf.Subject.CommonName)
	}

	// Update to new cert.
	certPEM, keyPEM := generateTestPEM(t, "updated.com")
	cm.UpdateFromPEM(certPEM, keyPEM)

	// GetCertificate should now return the new cert.
	cert2, _ := cm.GetCertificate(nil)
	if cert2.Leaf.Subject.CommonName != "updated.com" {
		t.Errorf("updated CN = %q, want updated.com", cert2.Leaf.Subject.CommonName)
	}
}

func TestBuildTLSConfig(t *testing.T) {
	cm, _ := newCertManager("")
	cfg := buildTLSConfig(cm)

	if cfg.MinVersion != tls.VersionTLS12 {
		t.Errorf("MinVersion = %d, want TLS 1.2", cfg.MinVersion)
	}
	if cfg.MaxVersion != tls.VersionTLS13 {
		t.Errorf("MaxVersion = %d, want TLS 1.3", cfg.MaxVersion)
	}
	if cfg.GetCertificate == nil {
		t.Error("GetCertificate is nil")
	}

	// All cipher suites should be ECDHE.
	for _, s := range cfg.CipherSuites {
		name := tls.CipherSuiteName(s)
		if !strings.Contains(name, "ECDHE") {
			t.Errorf("weak cipher suite: %s", name)
		}
	}
}

func TestGenerateSelfSignedCertForDomain(t *testing.T) {
	cert, err := generateSelfSignedCertForDomain("phish.example.com")
	if err != nil {
		t.Fatalf("generateSelfSignedCertForDomain: %v", err)
	}
	if cert.Leaf.Subject.CommonName != "phish.example.com" {
		t.Errorf("CN = %q, want phish.example.com", cert.Leaf.Subject.CommonName)
	}

	found := false
	for _, name := range cert.Leaf.DNSNames {
		if name == "phish.example.com" {
			found = true
		}
	}
	if !found {
		t.Error("phish.example.com not in DNSNames")
	}
}

func TestParsePEMCertificate(t *testing.T) {
	certPEM, _ := generateTestPEM(t, "parse-test.com")

	parsed, err := parsePEMCertificate(certPEM)
	if err != nil {
		t.Fatalf("parsePEMCertificate: %v", err)
	}
	if parsed.Subject.CommonName != "parse-test.com" {
		t.Errorf("CN = %q, want parse-test.com", parsed.Subject.CommonName)
	}
}

func TestParsePEMCertificateInvalid(t *testing.T) {
	_, err := parsePEMCertificate([]byte("not pem"))
	if err == nil {
		t.Error("expected error for invalid PEM")
	}

	// Wrong block type.
	block := pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: []byte("fake")})
	_, err = parsePEMCertificate(block)
	if err == nil {
		t.Error("expected error for non-CERTIFICATE PEM")
	}
}

func TestCertUpdateViaControlChannel(t *testing.T) {
	cm, _ := newCertManager("")
	relay, ts := newTestRelay(t, "tok")
	relay.certMgr = cm

	certPEM, keyPEM := generateTestPEM(t, "control-test.com")

	body := `{"cert_pem":` + jsonQuote(string(certPEM)) + `,"key_pem":` + jsonQuote(string(keyPEM)) + `}`

	req, _ := newAuthRequest(t, "POST", ts.URL+"/relay/cert", body, "tok")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		t.Errorf("status = %d, want 200", resp.StatusCode)
	}

	// Verify cert was updated.
	cert := cm.Current()
	if cert.Leaf.Subject.CommonName != "control-test.com" {
		t.Errorf("CN = %q, want control-test.com", cert.Leaf.Subject.CommonName)
	}
}

func TestCertUpdateUnauthorized(t *testing.T) {
	cm, _ := newCertManager("")
	relay, ts := newTestRelay(t, "secret")
	relay.certMgr = cm

	req, _ := newAuthRequest(t, "POST", ts.URL+"/relay/cert", `{"cert_pem":"x","key_pem":"y"}`, "wrong")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 401 {
		t.Errorf("status = %d, want 401", resp.StatusCode)
	}
}

// --- helpers ---

func generateTestPEM(t *testing.T, domain string) (certPEM, keyPEM []byte) {
	t.Helper()
	priv, _ := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	serial, _ := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 128))
	tmpl := x509.Certificate{
		SerialNumber: serial,
		Subject:      pkix.Name{CommonName: domain},
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     time.Now().Add(24 * time.Hour),
		KeyUsage:     x509.KeyUsageDigitalSignature,
		DNSNames:     []string{domain},
		IPAddresses:  []net.IP{net.ParseIP("127.0.0.1")},
	}
	certDER, _ := x509.CreateCertificate(rand.Reader, &tmpl, &tmpl, &priv.PublicKey, priv)
	certPEM = pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: certDER})
	privDER, _ := x509.MarshalECPrivateKey(priv)
	keyPEM = pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: privDER})
	return
}

func jsonQuote(s string) string {
	b, _ := json.Marshal(s)
	return string(b)
}

func newAuthRequest(t *testing.T, method, url, body, token string) (*http.Request, error) {
	t.Helper()
	req, err := http.NewRequest(method, url, strings.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	return req, nil
}
