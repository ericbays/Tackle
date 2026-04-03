// Package smtp provides SMTP connection testing and sending utilities for Tackle.
package smtp

import (
	"context"
	"crypto/hmac"
	"crypto/md5" //nolint:gosec // CRAM-MD5 is an SMTP auth mechanism defined in RFC 2195; use is protocol-mandated, not security-critical storage
	"crypto/tls"
	"encoding/base64"
	"fmt"
	"net"
	"net/smtp"
	"strings"
	"time"

	"tackle/internal/repositories"
)

// TestStage identifies which stage of the SMTP connection test was reached.
type TestStage string

const (
	// TestStageTCP is the initial TCP dial stage.
	TestStageTCP TestStage = "tcp"
	// TestStageTLS is the TLS negotiation stage.
	TestStageTLS TestStage = "tls"
	// TestStageAuth is the authentication stage.
	TestStageAuth TestStage = "auth"
	// TestStageNOOP is the NOOP verification stage.
	TestStageNOOP TestStage = "noop"
)

// ConnectionTestResult is the structured result of an SMTP connection test.
type ConnectionTestResult struct {
	Success      bool      `json:"success"`
	StageReached TestStage `json:"stage_reached"`
	TLSVersion   string    `json:"tls_version,omitempty"`
	ServerBanner string    `json:"server_banner,omitempty"`
	ErrorDetail  string    `json:"error_detail,omitempty"`
}

// ConnectionTester runs the TCP → TLS → AUTH → NOOP test sequence.
type ConnectionTester struct{}

// NewConnectionTester creates a ConnectionTester.
func NewConnectionTester() *ConnectionTester { return &ConnectionTester{} }

// Test runs the SMTP connection test for the given profile.
// Plaintext credentials (username and password) must be supplied already decrypted.
func (t *ConnectionTester) Test(ctx context.Context, profile repositories.SMTPProfile, username, password string) ConnectionTestResult {
	addr := fmt.Sprintf("%s:%d", profile.Host, profile.Port)
	connectTimeout := time.Duration(profile.TimeoutConnect) * time.Second

	// Stage 1: TCP dial.
	dialer := &net.Dialer{Timeout: connectTimeout}
	var conn net.Conn
	var err error

	if profile.TLSMode == repositories.SMTPTLSTLS {
		// Implicit TLS (port 465): wrap immediately.
		tlsCfg := buildTLSConfig(profile.Host, profile.TLSSkipVerify)
		conn, err = tls.DialWithDialer(dialer, "tcp", addr, tlsCfg)
	} else {
		conn, err = dialer.DialContext(ctx, "tcp", addr)
	}
	if err != nil {
		return ConnectionTestResult{
			StageReached: TestStageTCP,
			ErrorDetail:  fmt.Sprintf("TCP connection failed: %v", err),
		}
	}
	defer conn.Close()

	// Create SMTP client from raw connection.
	smtpConn, err := smtp.NewClient(conn, profile.Host)
	if err != nil {
		return ConnectionTestResult{
			StageReached: TestStageTCP,
			ErrorDetail:  fmt.Sprintf("SMTP client creation failed: %v", err),
		}
	}
	defer smtpConn.Close()

	// Capture server banner via EHLO (banner is not directly exposed by stdlib,
	// but we can note the hostname from the connection).
	serverBanner := profile.Host

	// Apply custom HELO if set.
	helo := "localhost"
	if profile.CustomHELO != nil && *profile.CustomHELO != "" {
		helo = *profile.CustomHELO
	}
	if err := smtpConn.Hello(helo); err != nil {
		return ConnectionTestResult{
			StageReached: TestStageTCP,
			ServerBanner: serverBanner,
			ErrorDetail:  fmt.Sprintf("EHLO/HELO failed: %v", err),
		}
	}

	// Stage 2: TLS negotiation (STARTTLS path).
	tlsVersion := ""
	if profile.TLSMode == repositories.SMTPTLSStartTLS {
		ok, _ := smtpConn.Extension("STARTTLS")
		if !ok {
			return ConnectionTestResult{
				StageReached: TestStageTLS,
				ServerBanner: serverBanner,
				ErrorDetail:  "STARTTLS not supported by server",
			}
		}
		tlsCfg := buildTLSConfig(profile.Host, profile.TLSSkipVerify)
		if err := smtpConn.StartTLS(tlsCfg); err != nil {
			return ConnectionTestResult{
				StageReached: TestStageTLS,
				ServerBanner: serverBanner,
				ErrorDetail:  fmt.Sprintf("STARTTLS negotiation failed: %v", err),
			}
		}
		if state, ok := smtpConn.TLSConnectionState(); ok {
			tlsVersion = tlsVersionString(state.Version)
		}
	} else if profile.TLSMode == repositories.SMTPTLSTLS {
		// Already TLS (connected above); capture the version.
		if tc, ok := conn.(*tls.Conn); ok {
			tlsVersion = tlsVersionString(tc.ConnectionState().Version)
		}
	}

	// Stage 3: Authentication.
	if profile.AuthType != repositories.SMTPAuthNone {
		if err := authenticate(smtpConn, profile.AuthType, username, password); err != nil {
			return ConnectionTestResult{
				StageReached: TestStageAuth,
				TLSVersion:   tlsVersion,
				ServerBanner: serverBanner,
				ErrorDetail:  fmt.Sprintf("authentication failed: %v", err),
			}
		}
	}

	// Stage 4: NOOP.
	if err := smtpConn.Noop(); err != nil {
		return ConnectionTestResult{
			StageReached: TestStageNOOP,
			TLSVersion:   tlsVersion,
			ServerBanner: serverBanner,
			ErrorDetail:  fmt.Sprintf("NOOP failed: %v", err),
		}
	}

	_ = smtpConn.Quit()

	return ConnectionTestResult{
		Success:      true,
		StageReached: TestStageNOOP,
		TLSVersion:   tlsVersion,
		ServerBanner: serverBanner,
	}
}

// authenticate sends the appropriate AUTH command based on the auth type.
func authenticate(c *smtp.Client, authType repositories.SMTPAuthType, username, password string) error {
	switch authType {
	case repositories.SMTPAuthPlain:
		return c.Auth(smtp.PlainAuth("", username, password, ""))
	case repositories.SMTPAuthLogin:
		return c.Auth(loginAuth(username, password))
	case repositories.SMTPAuthCRAMMD5:
		return c.Auth(smtp.CRAMMD5Auth(username, password))
	case repositories.SMTPAuthXOAUTH2:
		return c.Auth(xoauth2Auth(username, password))
	default:
		return fmt.Errorf("unsupported auth type: %s", authType)
	}
}

// buildTLSConfig creates a TLS configuration for the given host.
func buildTLSConfig(host string, skipVerify bool) *tls.Config {
	return &tls.Config{
		ServerName:         host,
		InsecureSkipVerify: skipVerify, //nolint:gosec // skip verify is user-controlled and flagged in the UI
		MinVersion:         tls.VersionTLS12,
	}
}

// tlsVersionString returns a human-readable TLS version string.
func tlsVersionString(v uint16) string {
	switch v {
	case tls.VersionTLS10:
		return "TLS 1.0"
	case tls.VersionTLS11:
		return "TLS 1.1"
	case tls.VersionTLS12:
		return "TLS 1.2"
	case tls.VersionTLS13:
		return "TLS 1.3"
	default:
		return fmt.Sprintf("TLS 0x%04x", v)
	}
}

// --- AUTH LOGIN implementation ---

type loginAuthImpl struct {
	username string
	password string
}

func loginAuth(username, password string) smtp.Auth {
	return &loginAuthImpl{username: username, password: password}
}

func (a *loginAuthImpl) Start(_ *smtp.ServerInfo) (string, []byte, error) {
	return "LOGIN", nil, nil
}

func (a *loginAuthImpl) Next(fromServer []byte, more bool) ([]byte, error) {
	if !more {
		return nil, nil
	}
	challenge := strings.ToUpper(string(fromServer))
	switch {
	case strings.Contains(challenge, "USERNAME") || strings.Contains(challenge, "USER"):
		return []byte(a.username), nil
	case strings.Contains(challenge, "PASSWORD") || strings.Contains(challenge, "PASS"):
		return []byte(a.password), nil
	}
	return nil, fmt.Errorf("unexpected LOGIN challenge: %q", string(fromServer))
}

// --- XOAUTH2 implementation (RFC 6749 / Google/Microsoft SASL) ---

type xoauth2AuthImpl struct {
	username    string
	accessToken string
}

func xoauth2Auth(username, accessToken string) smtp.Auth {
	return &xoauth2AuthImpl{username: username, accessToken: accessToken}
}

func (a *xoauth2AuthImpl) Start(_ *smtp.ServerInfo) (string, []byte, error) {
	sasl := fmt.Sprintf("user=%s\x01auth=Bearer %s\x01\x01", a.username, a.accessToken)
	return "XOAUTH2", []byte(sasl), nil
}

func (a *xoauth2AuthImpl) Next(_ []byte, _ bool) ([]byte, error) {
	// XOAUTH2 is a single-step mechanism; if the server sends a challenge, it is an error payload.
	return nil, nil
}

// --- CRAM-MD5 helper (stdlib smtp.CRAMMD5Auth covers this, but we expose it explicitly) ---

// cramMD5Response computes the CRAM-MD5 response for a given challenge.
// This is used internally by authenticate via smtp.CRAMMD5Auth.
// The function exists for testability.
func cramMD5Response(username, password string, challenge []byte) string {
	mac := hmac.New(md5.New, []byte(password))
	mac.Write(challenge)
	digest := mac.Sum(nil)
	return fmt.Sprintf("%s %x", username, digest)
}

// encodeBase64 is used in XOAUTH2 for test helpers.
func encodeBase64(s string) string {
	return base64.StdEncoding.EncodeToString([]byte(s))
}
