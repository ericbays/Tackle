package endpointmgmt

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"time"
)

// HealthCheckType identifies a type of health check.
type HealthCheckType string

const (
	// HealthCheckReachability checks TCP connectivity to the control port.
	HealthCheckReachability HealthCheckType = "reachability"
	// HealthCheckEndpointHealth checks the HTTP health endpoint.
	HealthCheckEndpointHealth HealthCheckType = "endpoint_health"
	// HealthCheckTLSCert checks TLS certificate validity and expiry.
	HealthCheckTLSCert HealthCheckType = "tls_cert"
	// HealthCheckSMTPRelay checks SMTP EHLO connectivity.
	HealthCheckSMTPRelay HealthCheckType = "smtp_relay"
)

// HealthCheckResult holds the result of a single health check.
type HealthCheckResult struct {
	Type    HealthCheckType `json:"type"`
	Healthy bool            `json:"healthy"`
	Message string          `json:"message,omitempty"`
	Latency time.Duration   `json:"latency_ms"`
}

// CheckReachability tests TCP connectivity to the endpoint's control port.
func CheckReachability(ctx context.Context, ip string, port int) HealthCheckResult {
	start := time.Now()
	addr := fmt.Sprintf("%s:%d", ip, port)

	conn, err := net.DialTimeout("tcp", addr, 10*time.Second)
	latency := time.Since(start)

	if err != nil {
		return HealthCheckResult{
			Type:    HealthCheckReachability,
			Healthy: false,
			Message: fmt.Sprintf("TCP connect failed: %s", err.Error()),
			Latency: latency,
		}
	}
	conn.Close()

	return HealthCheckResult{
		Type:    HealthCheckReachability,
		Healthy: true,
		Message: "TCP connect successful",
		Latency: latency,
	}
}

// CheckEndpointHTTPHealth performs an HTTP GET to the endpoint's health endpoint.
func CheckEndpointHTTPHealth(ctx context.Context, ip string, controlPort int) HealthCheckResult {
	start := time.Now()
	url := fmt.Sprintf("https://%s:%d/health", ip, controlPort)

	client := &http.Client{
		Timeout: 10 * time.Second,
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true}, //nolint:gosec // Endpoint uses self-signed cert.
		},
	}

	req, _ := http.NewRequestWithContext(ctx, "GET", url, nil)
	resp, err := client.Do(req)
	latency := time.Since(start)

	if err != nil {
		return HealthCheckResult{
			Type:    HealthCheckEndpointHealth,
			Healthy: false,
			Message: fmt.Sprintf("HTTP health check failed: %s", err.Error()),
			Latency: latency,
		}
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return HealthCheckResult{
			Type:    HealthCheckEndpointHealth,
			Healthy: false,
			Message: fmt.Sprintf("HTTP health returned status %d", resp.StatusCode),
			Latency: latency,
		}
	}

	return HealthCheckResult{
		Type:    HealthCheckEndpointHealth,
		Healthy: true,
		Message: "HTTP health OK",
		Latency: latency,
	}
}

// TLSCertCheckResult extends HealthCheckResult with TLS-specific info.
type TLSCertCheckResult struct {
	HealthCheckResult
	Issuer    string    `json:"issuer,omitempty"`
	NotAfter  time.Time `json:"not_after,omitempty"`
	DaysLeft  int       `json:"days_left,omitempty"`
}

// CheckTLSCert performs a TLS handshake and checks certificate validity.
func CheckTLSCert(ctx context.Context, domain string, port int) TLSCertCheckResult {
	start := time.Now()
	addr := fmt.Sprintf("%s:%d", domain, port)

	conn, err := tls.DialWithDialer(
		&net.Dialer{Timeout: 10 * time.Second},
		"tcp", addr,
		&tls.Config{
			ServerName:         domain,
			InsecureSkipVerify: false,
		},
	)
	latency := time.Since(start)

	if err != nil {
		// Try with InsecureSkipVerify to still get cert info.
		conn2, err2 := tls.DialWithDialer(
			&net.Dialer{Timeout: 10 * time.Second},
			"tcp", addr,
			&tls.Config{InsecureSkipVerify: true}, //nolint:gosec
		)
		if err2 != nil {
			return TLSCertCheckResult{
				HealthCheckResult: HealthCheckResult{
					Type:    HealthCheckTLSCert,
					Healthy: false,
					Message: fmt.Sprintf("TLS handshake failed: %s", err.Error()),
					Latency: latency,
				},
			}
		}
		defer conn2.Close()

		certs := conn2.ConnectionState().PeerCertificates
		if len(certs) > 0 {
			cert := certs[0]
			daysLeft := int(time.Until(cert.NotAfter).Hours() / 24)
			return TLSCertCheckResult{
				HealthCheckResult: HealthCheckResult{
					Type:    HealthCheckTLSCert,
					Healthy: false,
					Message: fmt.Sprintf("TLS verification failed (self-signed): %s", err.Error()),
					Latency: latency,
				},
				Issuer:   cert.Issuer.CommonName,
				NotAfter: cert.NotAfter,
				DaysLeft: daysLeft,
			}
		}

		return TLSCertCheckResult{
			HealthCheckResult: HealthCheckResult{
				Type:    HealthCheckTLSCert,
				Healthy: false,
				Message: fmt.Sprintf("TLS handshake failed: %s", err.Error()),
				Latency: latency,
			},
		}
	}
	defer conn.Close()

	certs := conn.ConnectionState().PeerCertificates
	if len(certs) == 0 {
		return TLSCertCheckResult{
			HealthCheckResult: HealthCheckResult{
				Type:    HealthCheckTLSCert,
				Healthy: false,
				Message: "no peer certificates",
				Latency: latency,
			},
		}
	}

	cert := certs[0]
	daysLeft := int(time.Until(cert.NotAfter).Hours() / 24)
	healthy := time.Now().Before(cert.NotAfter) && daysLeft > 0

	return TLSCertCheckResult{
		HealthCheckResult: HealthCheckResult{
			Type:    HealthCheckTLSCert,
			Healthy: healthy,
			Message: fmt.Sprintf("cert valid, expires in %d days", daysLeft),
			Latency: latency,
		},
		Issuer:   cert.Issuer.CommonName,
		NotAfter: cert.NotAfter,
		DaysLeft: daysLeft,
	}
}

// CheckSMTPRelay performs an SMTP EHLO to the endpoint's SMTP port.
func CheckSMTPRelay(ctx context.Context, ip string, port int) HealthCheckResult {
	start := time.Now()
	if port == 0 {
		port = 25
	}
	addr := fmt.Sprintf("%s:%d", ip, port)

	conn, err := net.DialTimeout("tcp", addr, 10*time.Second)
	latency := time.Since(start)

	if err != nil {
		return HealthCheckResult{
			Type:    HealthCheckSMTPRelay,
			Healthy: false,
			Message: fmt.Sprintf("SMTP connect failed: %s", err.Error()),
			Latency: latency,
		}
	}
	defer conn.Close()

	// Read server banner.
	conn.SetDeadline(time.Now().Add(5 * time.Second)) //nolint:errcheck
	banner := make([]byte, 512)
	n, _ := conn.Read(banner)

	// Send EHLO.
	_, _ = conn.Write([]byte("EHLO tackle.local\r\n"))
	resp := make([]byte, 1024)
	rn, _ := conn.Read(resp)

	// Send QUIT.
	_, _ = conn.Write([]byte("QUIT\r\n"))

	if n == 0 && rn == 0 {
		return HealthCheckResult{
			Type:    HealthCheckSMTPRelay,
			Healthy: false,
			Message: "no SMTP response",
			Latency: time.Since(start),
		}
	}

	return HealthCheckResult{
		Type:    HealthCheckSMTPRelay,
		Healthy: true,
		Message: fmt.Sprintf("SMTP EHLO OK, banner: %.50s", string(banner[:n])),
		Latency: time.Since(start),
	}
}

// RunAllChecks runs all 4 health check types against an endpoint and returns the results.
func RunAllChecks(ctx context.Context, ip, domain string, controlPort, smtpPort int) []HealthCheckResult {
	results := make([]HealthCheckResult, 0, 4)

	results = append(results, CheckReachability(ctx, ip, controlPort))
	results = append(results, CheckEndpointHTTPHealth(ctx, ip, controlPort))

	if domain != "" {
		tlsResult := CheckTLSCert(ctx, domain, 443)
		results = append(results, tlsResult.HealthCheckResult)
	}

	if smtpPort > 0 {
		results = append(results, CheckSMTPRelay(ctx, ip, smtpPort))
	}

	return results
}

// MarshalCheckResults serializes check results to JSON for storage in heartbeat metrics.
func MarshalCheckResults(results []HealthCheckResult) json.RawMessage {
	data, _ := json.Marshal(results)
	return data
}
