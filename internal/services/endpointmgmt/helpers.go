package endpointmgmt

import (
	"crypto/sha256"
	"crypto/x509"
	"encoding/hex"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"strings"
	"time"
)

// encodeJSON marshals v to JSON bytes.
func encodeJSON(v any) ([]byte, error) {
	return json.Marshal(v)
}

// decodeJSON unmarshals JSON bytes into v.
func decodeJSON(data []byte, v any) error {
	return json.Unmarshal(data, v)
}

// parsedCert holds parsed certificate metadata.
type parsedCert struct {
	issuer      string
	notBefore   time.Time
	notAfter    time.Time
	fingerprint string
	dnsNames    []string
}

// matchesDomain returns true if the certificate covers the given domain.
func (c *parsedCert) matchesDomain(domain string) bool {
	for _, name := range c.dnsNames {
		if strings.EqualFold(name, domain) {
			return true
		}
		// Wildcard match: *.example.com matches sub.example.com
		if strings.HasPrefix(name, "*.") {
			suffix := name[1:] // .example.com
			if strings.HasSuffix(strings.ToLower(domain), strings.ToLower(suffix)) {
				return true
			}
		}
	}
	return false
}

// parseTLSCertificate parses a PEM-encoded certificate and extracts metadata.
func parseTLSCertificate(certPEM []byte) (*parsedCert, error) {
	block, _ := pem.Decode(certPEM)
	if block == nil {
		return nil, fmt.Errorf("failed to decode PEM block")
	}
	if block.Type != "CERTIFICATE" {
		return nil, fmt.Errorf("PEM block is not a certificate (type: %s)", block.Type)
	}

	cert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		return nil, fmt.Errorf("parse certificate: %w", err)
	}

	fingerprint := sha256.Sum256(cert.Raw)

	return &parsedCert{
		issuer:      cert.Issuer.String(),
		notBefore:   cert.NotBefore,
		notAfter:    cert.NotAfter,
		fingerprint: hex.EncodeToString(fingerprint[:]),
		dnsNames:    cert.DNSNames,
	}, nil
}
