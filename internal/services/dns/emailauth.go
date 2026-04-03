package dns

import (
	"crypto/ed25519"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/base64"
	"encoding/pem"
	"fmt"
	"strings"
)

// --- SPF ---

// SPFQualifier is the qualifier for an SPF mechanism (+, -, ~, ?).
type SPFQualifier string

const (
	SPFQualifierPass     SPFQualifier = "+"
	SPFQualifierFail     SPFQualifier = "-"
	SPFQualifierSoftFail SPFQualifier = "~"
	SPFQualifierNeutral  SPFQualifier = "?"
)

// SPFMechanism represents a single SPF mechanism entry.
type SPFMechanism struct {
	// Type is one of: ip4, ip6, include, a, mx, exists, all.
	Type      string       `json:"type"`
	Value     string       `json:"value"`     // IP, hostname, or empty (for "all")
	Qualifier SPFQualifier `json:"qualifier"` // defaults to "+"
}

// SPFConfig holds the parsed representation of an SPF record.
type SPFConfig struct {
	Mechanisms []SPFMechanism `json:"mechanisms"`
	// AllPolicy is the qualifier for the trailing "all" mechanism.
	// Defaults to "~" (SoftFail) if not set.
	AllPolicy SPFQualifier `json:"all_policy"`
}

// SPFWarning is an advisory message about an SPF record.
type SPFWarning struct {
	Message string `json:"message"`
}

// dnsLookupMechanisms lists SPF mechanism types that consume a DNS lookup.
var dnsLookupMechanisms = map[string]bool{
	"include": true,
	"a":       true,
	"mx":      true,
	"exists":  true,
	"ptr":     true,
	"redirect": true,
}

// BuildSPFRecord assembles a TXT record value from an SPFConfig.
// Returns the record string, any warnings (e.g. lookup limit), and an error.
func BuildSPFRecord(cfg SPFConfig) (string, []SPFWarning, error) {
	if len(cfg.AllPolicy) == 0 {
		cfg.AllPolicy = SPFQualifierSoftFail
	}

	var parts []string
	parts = append(parts, "v=spf1")

	lookupCount := 0
	for _, m := range cfg.Mechanisms {
		if dnsLookupMechanisms[strings.ToLower(m.Type)] {
			lookupCount++
		}
		qualifier := string(m.Qualifier)
		if qualifier == "" {
			qualifier = "+"
		}
		// "+" qualifier is implicit and usually omitted in practice.
		prefix := ""
		if qualifier != "+" {
			prefix = qualifier
		}
		if m.Value != "" {
			parts = append(parts, fmt.Sprintf("%s%s:%s", prefix, strings.ToLower(m.Type), m.Value))
		} else {
			parts = append(parts, fmt.Sprintf("%s%s", prefix, strings.ToLower(m.Type)))
		}
	}

	parts = append(parts, string(cfg.AllPolicy)+"all")

	record := strings.Join(parts, " ")

	var warnings []SPFWarning
	if lookupCount > 10 {
		warnings = append(warnings, SPFWarning{
			Message: fmt.Sprintf("SPF record has %d DNS lookups; the RFC 7208 limit is 10. Exceeding this causes SPF to permanently fail.", lookupCount),
		})
	}
	if len(record) > 255 {
		warnings = append(warnings, SPFWarning{
			Message: fmt.Sprintf("SPF record is %d characters; values over 255 must be split across multiple TXT strings.", len(record)),
		})
	}

	return record, warnings, nil
}

// ParseSPFRecord parses an SPF TXT record value back into an SPFConfig.
func ParseSPFRecord(txt string) (SPFConfig, error) {
	txt = strings.TrimSpace(txt)
	if !strings.HasPrefix(txt, "v=spf1") {
		return SPFConfig{}, fmt.Errorf("spf: not a valid SPF record (missing v=spf1)")
	}

	tokens := strings.Fields(txt)
	var cfg SPFConfig

	for _, tok := range tokens[1:] { // skip "v=spf1"
		if strings.HasSuffix(strings.ToLower(tok), "all") {
			q := SPFQualifier(tok[0])
			if tok[0] != '-' && tok[0] != '~' && tok[0] != '?' {
				q = SPFQualifierPass
			}
			cfg.AllPolicy = q
			continue
		}

		qualifier := SPFQualifierPass
		mechanism := tok
		if len(tok) > 0 && (tok[0] == '+' || tok[0] == '-' || tok[0] == '~' || tok[0] == '?') {
			qualifier = SPFQualifier(tok[0:1])
			mechanism = tok[1:]
		}

		mType, mValue, _ := strings.Cut(mechanism, ":")
		cfg.Mechanisms = append(cfg.Mechanisms, SPFMechanism{
			Type:      mType,
			Value:     mValue,
			Qualifier: qualifier,
		})
	}

	return cfg, nil
}

// --- DKIM ---

// DKIMAlgorithm represents the signing algorithm for a DKIM key pair.
type DKIMAlgorithm string

const (
	DKIMAlgorithmRSA     DKIMAlgorithm = "rsa-sha256"
	DKIMAlgorithmEd25519 DKIMAlgorithm = "ed25519-sha256"
)

// DKIMKeyPair holds a generated DKIM key pair and the associated DNS record data.
type DKIMKeyPair struct {
	Algorithm        DKIMAlgorithm
	KeySize          int    // RSA key size in bits; 0 for Ed25519
	PrivateKeyPEM    []byte // PKCS#8 PEM-encoded private key (to be encrypted before storage)
	PublicKeyBase64  string // base64-encoded public key for the DNS TXT p= field
}

// GenerateDKIMKeyPair generates a new DKIM key pair for the given algorithm.
// For RSA, keySize must be ≥ 2048. For Ed25519, keySize is ignored.
func GenerateDKIMKeyPair(algorithm DKIMAlgorithm, keySize int) (DKIMKeyPair, error) {
	switch algorithm {
	case DKIMAlgorithmRSA:
		return generateRSADKIMKey(keySize)
	case DKIMAlgorithmEd25519:
		return generateEd25519DKIMKey()
	default:
		return DKIMKeyPair{}, fmt.Errorf("dkim: unsupported algorithm %q", algorithm)
	}
}

func generateRSADKIMKey(keySize int) (DKIMKeyPair, error) {
	if keySize < 2048 {
		keySize = 2048
	}

	priv, err := rsa.GenerateKey(rand.Reader, keySize)
	if err != nil {
		return DKIMKeyPair{}, fmt.Errorf("dkim rsa: generate key: %w", err)
	}

	privDER, err := x509.MarshalPKCS8PrivateKey(priv)
	if err != nil {
		return DKIMKeyPair{}, fmt.Errorf("dkim rsa: marshal private key: %w", err)
	}

	privPEM := pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: privDER})

	pubDER, err := x509.MarshalPKIXPublicKey(&priv.PublicKey)
	if err != nil {
		return DKIMKeyPair{}, fmt.Errorf("dkim rsa: marshal public key: %w", err)
	}

	return DKIMKeyPair{
		Algorithm:       DKIMAlgorithmRSA,
		KeySize:         keySize,
		PrivateKeyPEM:   privPEM,
		PublicKeyBase64: base64.StdEncoding.EncodeToString(pubDER),
	}, nil
}

func generateEd25519DKIMKey() (DKIMKeyPair, error) {
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return DKIMKeyPair{}, fmt.Errorf("dkim ed25519: generate key: %w", err)
	}

	privDER, err := x509.MarshalPKCS8PrivateKey(priv)
	if err != nil {
		return DKIMKeyPair{}, fmt.Errorf("dkim ed25519: marshal private key: %w", err)
	}

	privPEM := pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: privDER})

	// Ed25519 public key for DNS: raw public key bytes, base64-encoded.
	pubBytes := []byte(pub)
	return DKIMKeyPair{
		Algorithm:       DKIMAlgorithmEd25519,
		KeySize:         0,
		PrivateKeyPEM:   privPEM,
		PublicKeyBase64: base64.StdEncoding.EncodeToString(pubBytes),
	}, nil
}

// BuildDKIMRecord constructs the DNS TXT record name and value for a DKIM key.
// Returns (recordName, recordValue, error).
//
// recordName will be "{selector}._domainkey.{domain}".
// recordValue will be "v=DKIM1; k={k}; p={base64_public_key}".
func BuildDKIMRecord(domain, selector string, kp DKIMKeyPair) (recordName, recordValue string, err error) {
	if selector == "" {
		return "", "", fmt.Errorf("dkim: selector must not be empty")
	}
	if domain == "" {
		return "", "", fmt.Errorf("dkim: domain must not be empty")
	}

	var k string
	switch kp.Algorithm {
	case DKIMAlgorithmRSA:
		k = "rsa"
	case DKIMAlgorithmEd25519:
		k = "ed25519"
	default:
		return "", "", fmt.Errorf("dkim: unsupported algorithm %q", kp.Algorithm)
	}

	recordName = fmt.Sprintf("%s._domainkey.%s", selector, domain)
	recordValue = fmt.Sprintf("v=DKIM1; k=%s; p=%s", k, kp.PublicKeyBase64)
	return recordName, recordValue, nil
}

// --- DMARC ---

// DMARCPolicy is the DMARC enforcement policy.
type DMARCPolicy string

const (
	DMARCPolicyNone       DMARCPolicy = "none"
	DMARCPolicyQuarantine DMARCPolicy = "quarantine"
	DMARCPolicyReject     DMARCPolicy = "reject"
)

// DMARCAlignment is the DKIM/SPF identifier alignment mode.
type DMARCAlignment string

const (
	DMARCAlignmentRelaxed DMARCAlignment = "r"
	DMARCAlignmentStrict  DMARCAlignment = "s"
)

// DMARCConfig holds the full DMARC policy configuration.
type DMARCConfig struct {
	// Policy is the top-level DMARC policy. Default is none (REQ-EMAIL-029).
	Policy DMARCPolicy `json:"policy"`
	// SubdomainPolicy overrides Policy for subdomains if set.
	SubdomainPolicy *DMARCPolicy `json:"subdomain_policy,omitempty"`
	// Percentage is the percentage of messages to which the policy applies (0–100).
	Percentage int `json:"percentage"`
	// RUA is a list of aggregate report URIs (e.g. "mailto:dmarc@example.com").
	RUA []string `json:"rua,omitempty"`
	// RUF is a list of forensic report URIs.
	RUF []string `json:"ruf,omitempty"`
	// ASPF is the SPF alignment mode (relaxed or strict).
	ASPF DMARCAlignment `json:"aspf,omitempty"`
	// ADKIM is the DKIM alignment mode (relaxed or strict).
	ADKIM DMARCAlignment `json:"adkim,omitempty"`
	// RI is the reporting interval in seconds.
	RI *int `json:"ri,omitempty"`
	// FO is the failure reporting options ("0", "1", "d", "s").
	FO string `json:"fo,omitempty"`
}

// BuildDMARCRecord constructs the DNS TXT record name and value for a DMARC config.
// Returns (recordName, recordValue, error).
//
// recordName will be "_dmarc.{domain}".
func BuildDMARCRecord(domain string, cfg DMARCConfig) (recordName, recordValue string, err error) {
	if domain == "" {
		return "", "", fmt.Errorf("dmarc: domain must not be empty")
	}

	// Default policy is none (REQ-EMAIL-029).
	if cfg.Policy == "" {
		cfg.Policy = DMARCPolicyNone
	}

	pct := cfg.Percentage
	if pct == 0 {
		pct = 100
	}

	var parts []string
	parts = append(parts, "v=DMARC1")
	parts = append(parts, fmt.Sprintf("p=%s", cfg.Policy))

	if cfg.SubdomainPolicy != nil {
		parts = append(parts, fmt.Sprintf("sp=%s", *cfg.SubdomainPolicy))
	}

	if pct != 100 {
		parts = append(parts, fmt.Sprintf("pct=%d", pct))
	}

	if len(cfg.RUA) > 0 {
		parts = append(parts, fmt.Sprintf("rua=%s", strings.Join(cfg.RUA, ",")))
	}

	if len(cfg.RUF) > 0 {
		parts = append(parts, fmt.Sprintf("ruf=%s", strings.Join(cfg.RUF, ",")))
	}

	if cfg.ASPF != "" && cfg.ASPF != DMARCAlignmentRelaxed {
		parts = append(parts, fmt.Sprintf("aspf=%s", cfg.ASPF))
	}

	if cfg.ADKIM != "" && cfg.ADKIM != DMARCAlignmentRelaxed {
		parts = append(parts, fmt.Sprintf("adkim=%s", cfg.ADKIM))
	}

	if cfg.RI != nil {
		parts = append(parts, fmt.Sprintf("ri=%d", *cfg.RI))
	}

	if cfg.FO != "" {
		parts = append(parts, fmt.Sprintf("fo=%s", cfg.FO))
	}

	recordName = "_dmarc." + domain
	recordValue = strings.Join(parts, "; ")
	return recordName, recordValue, nil
}

// ParseDMARCRecord parses a DMARC TXT record value back into a DMARCConfig.
func ParseDMARCRecord(txt string) (DMARCConfig, error) {
	txt = strings.TrimSpace(txt)
	if !strings.HasPrefix(txt, "v=DMARC1") {
		return DMARCConfig{}, fmt.Errorf("dmarc: not a valid DMARC record (missing v=DMARC1)")
	}

	var cfg DMARCConfig
	cfg.Percentage = 100

	for _, token := range strings.Split(txt, ";") {
		token = strings.TrimSpace(token)
		if token == "" || token == "v=DMARC1" {
			continue
		}
		key, value, ok := strings.Cut(token, "=")
		if !ok {
			continue
		}
		key = strings.TrimSpace(key)
		value = strings.TrimSpace(value)

		switch key {
		case "p":
			cfg.Policy = DMARCPolicy(value)
		case "sp":
			sp := DMARCPolicy(value)
			cfg.SubdomainPolicy = &sp
		case "pct":
			fmt.Sscanf(value, "%d", &cfg.Percentage)
		case "rua":
			cfg.RUA = splitCSV(value)
		case "ruf":
			cfg.RUF = splitCSV(value)
		case "aspf":
			cfg.ASPF = DMARCAlignment(value)
		case "adkim":
			cfg.ADKIM = DMARCAlignment(value)
		case "ri":
			var ri int
			if _, err := fmt.Sscanf(value, "%d", &ri); err == nil {
				cfg.RI = &ri
			}
		case "fo":
			cfg.FO = value
		}
	}

	return cfg, nil
}

// splitCSV splits a comma-separated list and trims whitespace from each element.
func splitCSV(s string) []string {
	parts := strings.Split(s, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}
