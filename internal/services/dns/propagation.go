package dns

import (
	"context"
	"fmt"
	"net"
	"strings"
	"time"

	dnsiface "tackle/internal/providers/dns"
)

// PropagationStatus summarises how many resolvers agree with the expected value.
type PropagationStatus string

const (
	// PropagationStatusPropagated means all resolvers returned the expected value.
	PropagationStatusPropagated PropagationStatus = "propagated"
	// PropagationStatusPartial means some but not all resolvers returned the expected value.
	PropagationStatusPartial PropagationStatus = "partial"
	// PropagationStatusNotPropagated means no resolver returned the expected value.
	PropagationStatusNotPropagated PropagationStatus = "not_propagated"
)

// ResolverResult is the outcome of querying one DNS resolver.
type ResolverResult struct {
	Resolver  string        `json:"resolver"`
	Response  string        `json:"response"`
	Matches   bool          `json:"matches"`
	LatencyMs int64         `json:"latency_ms"`
	Error     string        `json:"error,omitempty"`
}

// PropagationResult is the aggregated result across all resolvers.
type PropagationResult struct {
	OverallStatus PropagationStatus
	Results       []ResolverResult
}

// publicResolvers is the set of geographically distributed DNS resolvers used
// for propagation checks. All queries use port 53 UDP/TCP via net.Resolver.
var publicResolvers = []string{
	"8.8.8.8",      // Google
	"8.8.4.4",      // Google secondary
	"1.1.1.1",      // Cloudflare
	"1.0.0.1",      // Cloudflare secondary
	"208.67.222.222", // OpenDNS
	"9.9.9.9",      // Quad9
}

// CheckPropagation queries each public resolver and determines whether the
// expected DNS record value has propagated.
//
// domain is the bare domain/hostname to query (e.g. "www.example.com").
// recordType must be one of the supported types (A, AAAA, CNAME, MX, TXT, NS).
// expectedValue is the record value to match against.
func CheckPropagation(ctx context.Context, domain string, recordType dnsiface.RecordType, expectedValue string) (PropagationResult, error) {
	results := make([]ResolverResult, 0, len(publicResolvers))

	for _, resolverIP := range publicResolvers {
		result := queryResolver(ctx, resolverIP, domain, recordType, expectedValue)
		results = append(results, result)
	}

	matched := 0
	for _, r := range results {
		if r.Matches {
			matched++
		}
	}

	var status PropagationStatus
	switch {
	case matched == len(results):
		status = PropagationStatusPropagated
	case matched > 0:
		status = PropagationStatusPartial
	default:
		status = PropagationStatusNotPropagated
	}

	return PropagationResult{
		OverallStatus: status,
		Results:       results,
	}, nil
}

// queryResolver queries a single resolver for the given domain and record type.
func queryResolver(ctx context.Context, resolverIP, domain string, recordType dnsiface.RecordType, expectedValue string) ResolverResult {
	start := time.Now()

	r := &net.Resolver{
		PreferGo: true,
		Dial: func(ctx context.Context, network, address string) (net.Conn, error) {
			d := net.Dialer{Timeout: 5 * time.Second}
			return d.DialContext(ctx, "udp", resolverIP+":53")
		},
	}

	response, err := lookupRecord(ctx, r, domain, recordType)
	latency := time.Since(start).Milliseconds()

	if err != nil {
		return ResolverResult{
			Resolver:  resolverIP,
			Response:  "",
			Matches:   false,
			LatencyMs: latency,
			Error:     err.Error(),
		}
	}

	matches := valuesMatch(response, expectedValue, recordType)

	return ResolverResult{
		Resolver:  resolverIP,
		Response:  response,
		Matches:   matches,
		LatencyMs: latency,
	}
}

// lookupRecord performs a DNS lookup using net.Resolver and returns the first
// matching value as a string.
func lookupRecord(ctx context.Context, r *net.Resolver, domain string, recordType dnsiface.RecordType) (string, error) {
	switch recordType {
	case dnsiface.RecordTypeA:
		addrs, err := r.LookupHost(ctx, domain)
		if err != nil {
			return "", fmt.Errorf("lookup A: %w", err)
		}
		for _, a := range addrs {
			if ip := net.ParseIP(a); ip != nil && ip.To4() != nil {
				return a, nil
			}
		}
		return "", fmt.Errorf("no A record found")

	case dnsiface.RecordTypeAAAA:
		addrs, err := r.LookupHost(ctx, domain)
		if err != nil {
			return "", fmt.Errorf("lookup AAAA: %w", err)
		}
		for _, a := range addrs {
			if ip := net.ParseIP(a); ip != nil && ip.To4() == nil {
				return a, nil
			}
		}
		return "", fmt.Errorf("no AAAA record found")

	case dnsiface.RecordTypeCNAME:
		cname, err := r.LookupCNAME(ctx, domain)
		if err != nil {
			return "", fmt.Errorf("lookup CNAME: %w", err)
		}
		return strings.TrimSuffix(cname, "."), nil

	case dnsiface.RecordTypeMX:
		mxs, err := r.LookupMX(ctx, domain)
		if err != nil {
			return "", fmt.Errorf("lookup MX: %w", err)
		}
		if len(mxs) == 0 {
			return "", fmt.Errorf("no MX records found")
		}
		return fmt.Sprintf("%d %s", mxs[0].Pref, strings.TrimSuffix(mxs[0].Host, ".")), nil

	case dnsiface.RecordTypeTXT:
		txts, err := r.LookupTXT(ctx, domain)
		if err != nil {
			return "", fmt.Errorf("lookup TXT: %w", err)
		}
		return strings.Join(txts, ""), nil

	case dnsiface.RecordTypeNS:
		nss, err := r.LookupNS(ctx, domain)
		if err != nil {
			return "", fmt.Errorf("lookup NS: %w", err)
		}
		if len(nss) == 0 {
			return "", fmt.Errorf("no NS records found")
		}
		return strings.TrimSuffix(nss[0].Host, "."), nil

	default:
		return "", fmt.Errorf("propagation check: unsupported record type %q", recordType)
	}
}

// valuesMatch compares the resolved response against the expected value.
// Comparison is case-insensitive for hostnames and normalises TXT quoting.
func valuesMatch(response, expected string, recordType dnsiface.RecordType) bool {
	response = strings.TrimSpace(response)
	expected = strings.TrimSpace(expected)

	switch recordType {
	case dnsiface.RecordTypeCNAME, dnsiface.RecordTypeNS, dnsiface.RecordTypeMX:
		return strings.EqualFold(
			strings.TrimSuffix(response, "."),
			strings.TrimSuffix(expected, "."),
		)
	case dnsiface.RecordTypeTXT:
		// Strip surrounding quotes that some resolvers add.
		response = strings.Trim(response, "\"")
		expected = strings.Trim(expected, "\"")
		return response == expected
	default:
		return response == expected
	}
}
