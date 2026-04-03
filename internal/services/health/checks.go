// Package health implements domain health checking: DNS propagation, blocklist status,
// email authentication validity, and MX resolution.
package health

import (
	"context"
	"fmt"
	"net"
	"strings"
	"time"

	dnsiface "tackle/internal/providers/dns"
	dnssvc "tackle/internal/services/dns"
)

// CheckStatus is the per-check result status.
type CheckStatus string

const (
	// CheckStatusHealthy means the check passed.
	CheckStatusHealthy CheckStatus = "healthy"
	// CheckStatusWarning means the check returned a non-critical issue.
	CheckStatusWarning CheckStatus = "warning"
	// CheckStatusCritical means the check returned a critical issue.
	CheckStatusCritical CheckStatus = "critical"
	// CheckStatusSkipped means the check was not run.
	CheckStatusSkipped CheckStatus = "skipped"
)

// PropagationCheckResult contains the result of DNS propagation health checks.
type PropagationCheckResult struct {
	Status         CheckStatus              `json:"status"`
	OverallStatus  string                   `json:"overall_status"`
	ResolverCount  int                      `json:"resolver_count"`
	MatchedCount   int                      `json:"matched_count"`
	ResolverChecks []dnssvc.ResolverResult  `json:"resolver_checks"`
}

// BlocklistEntry contains the result for a single blocklist lookup.
type BlocklistEntry struct {
	Name       string `json:"name"`
	QueryName  string `json:"query_name"`
	Listed     bool   `json:"listed"`
	ReturnCode string `json:"return_code,omitempty"`
	Error      string `json:"error,omitempty"`
}

// BlocklistCheckResult contains the aggregate result of all blocklist checks.
type BlocklistCheckResult struct {
	Status  CheckStatus      `json:"status"`
	Results []BlocklistEntry `json:"results"`
}

// EmailAuthCheckResult contains the result of SPF/DKIM/DMARC checks.
type EmailAuthCheckResult struct {
	Status    CheckStatus `json:"status"`
	SPFStatus string      `json:"spf_status"`
	DKIMStatus string     `json:"dkim_status"`
	DMARCStatus string    `json:"dmarc_status"`
}

// MXHostResult contains the result for a single MX host.
type MXHostResult struct {
	Host      string `json:"host"`
	Priority  uint16 `json:"priority"`
	Resolvable bool  `json:"resolvable"`
	Reachable  bool  `json:"reachable"`
	Error      string `json:"error,omitempty"`
}

// MXCheckResult contains the aggregate result of all MX checks.
type MXCheckResult struct {
	Status  CheckStatus    `json:"status"`
	Results []MXHostResult `json:"results"`
}

// blocklists is the list of DNS-based blocklist services to query.
var blocklists = []struct {
	Name   string
	Suffix string
}{
	{"Spamhaus DBL", "dbl.spamhaus.org"},
	{"SURBL", "multi.surbl.org"},
	{"URIBL", "multi.uribl.com"},
	{"SpamCop", "bl.spamcop.net"},
	{"Barracuda", "b.barracudacentral.org"},
}

// CheckDNSPropagation queries public resolvers for A/AAAA records and classifies
// the result as healthy, warning (partial propagation), or critical (no responses).
func CheckDNSPropagation(ctx context.Context, domain string) (PropagationCheckResult, error) {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	result, err := dnssvc.CheckPropagation(ctx, domain, dnsiface.RecordTypeA, "")
	if err != nil {
		// Non-fatal — return critical.
		return PropagationCheckResult{
			Status:        CheckStatusCritical,
			OverallStatus: "not_propagated",
		}, nil
	}

	matched := 0
	for _, r := range result.Results {
		if r.Matches || (r.Error == "" && r.Response != "") {
			matched++
		}
	}

	var status CheckStatus
	switch result.OverallStatus {
	case dnssvc.PropagationStatusPropagated:
		status = CheckStatusHealthy
	case dnssvc.PropagationStatusPartial:
		status = CheckStatusWarning
	default:
		// Check if any resolver responded at all (propagation but no expected value set)
		responded := 0
		for _, r := range result.Results {
			if r.Error == "" {
				responded++
			}
		}
		if responded >= 3 {
			status = CheckStatusHealthy
		} else if responded > 0 {
			status = CheckStatusWarning
		} else {
			status = CheckStatusCritical
		}
	}

	return PropagationCheckResult{
		Status:         status,
		OverallStatus:  string(result.OverallStatus),
		ResolverCount:  len(result.Results),
		MatchedCount:   matched,
		ResolverChecks: result.Results,
	}, nil
}

// CheckBlocklists queries DNS-based blocklists for the given domain.
// A returned A record means the domain is listed on that blocklist.
func CheckBlocklists(ctx context.Context, domain string) (BlocklistCheckResult, error) {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	type entryResult struct {
		idx   int
		entry BlocklistEntry
	}

	results := make([]BlocklistEntry, len(blocklists))
	ch := make(chan entryResult, len(blocklists))

	for i, bl := range blocklists {
		go func(idx int, name, suffix string) {
			queryName := domain + "." + suffix
			entry := BlocklistEntry{
				Name:      name,
				QueryName: queryName,
			}

			r := &net.Resolver{
				PreferGo: true,
				Dial: func(ctx context.Context, network, address string) (net.Conn, error) {
					d := net.Dialer{Timeout: 5 * time.Second}
					return d.DialContext(ctx, "udp", "8.8.8.8:53")
				},
			}

			addrs, err := r.LookupHost(ctx, queryName)
			if err != nil {
				// NXDOMAIN = not listed; other errors treated as unknown (not listed).
				entry.Listed = false
				if !strings.Contains(err.Error(), "no such host") &&
					!strings.Contains(err.Error(), "NXDOMAIN") {
					entry.Error = err.Error()
				}
			} else if len(addrs) > 0 {
				entry.Listed = true
				entry.ReturnCode = addrs[0]
			}
			ch <- entryResult{idx: idx, entry: entry}
		}(i, bl.Name, bl.Suffix)
	}

	for range blocklists {
		r := <-ch
		results[r.idx] = r.entry
	}

	anyListed := false
	for _, e := range results {
		if e.Listed {
			anyListed = true
			break
		}
	}

	status := CheckStatusHealthy
	if anyListed {
		status = CheckStatusCritical
	}

	return BlocklistCheckResult{
		Status:  status,
		Results: results,
	}, nil
}

// CheckEmailAuth validates SPF, DKIM, and DMARC status for a domain by
// delegating to the DNS propagation checker.
func CheckEmailAuth(ctx context.Context, domain string, knownSelectors []string) (EmailAuthCheckResult, error) {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	// SPF: look for v=spf1 prefix in TXT at apex.
	spfResult, _ := dnssvc.CheckPropagation(ctx, domain, dnsiface.RecordTypeTXT, "v=spf1")
	spfStatus := propagationToAuthStatus(spfResult)

	// DMARC: look for v=DMARC1 in _dmarc.<domain>.
	dmarcResult, _ := dnssvc.CheckPropagation(ctx, "_dmarc."+domain, dnsiface.RecordTypeTXT, "v=DMARC1")
	dmarcStatus := propagationToAuthStatus(dmarcResult)

	// DKIM: check each known selector.
	dkimStatus := "missing"
	if len(knownSelectors) > 0 {
		for _, sel := range knownSelectors {
			dkimDomain := sel + "._domainkey." + domain
			dkimResult, _ := dnssvc.CheckPropagation(ctx, dkimDomain, dnsiface.RecordTypeTXT, "v=DKIM1")
			if dkimResult.OverallStatus == dnssvc.PropagationStatusPropagated {
				dkimStatus = "configured"
				break
			}
		}
		if dkimStatus != "configured" {
			dkimStatus = "misconfigured"
		}
	}

	// Classify overall.
	var overallStatus CheckStatus
	switch {
	case spfStatus == "configured" && dkimStatus == "configured" && dmarcStatus == "configured":
		overallStatus = CheckStatusHealthy
	case spfStatus == "missing" || dkimStatus == "misconfigured":
		overallStatus = CheckStatusCritical
	default:
		overallStatus = CheckStatusWarning
	}

	return EmailAuthCheckResult{
		Status:      overallStatus,
		SPFStatus:   spfStatus,
		DKIMStatus:  dkimStatus,
		DMARCStatus: dmarcStatus,
	}, nil
}

// CheckMXResolution looks up MX records, resolves each host, and attempts a TCP
// connection to port 25 to verify reachability.
func CheckMXResolution(ctx context.Context, domain string) (MXCheckResult, error) {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	r := net.DefaultResolver
	mxs, err := r.LookupMX(ctx, domain)
	if err != nil || len(mxs) == 0 {
		return MXCheckResult{
			Status:  CheckStatusCritical,
			Results: nil,
		}, nil
	}

	results := make([]MXHostResult, 0, len(mxs))
	for _, mx := range mxs {
		host := strings.TrimSuffix(mx.Host, ".")
		entry := MXHostResult{
			Host:     host,
			Priority: mx.Pref,
		}

		addrs, resolveErr := r.LookupHost(ctx, host)
		if resolveErr != nil || len(addrs) == 0 {
			entry.Error = fmt.Sprintf("resolve failed: %v", resolveErr)
		} else {
			entry.Resolvable = true
			// Attempt TCP to port 25 with a short timeout.
			conn, dialErr := (&net.Dialer{Timeout: 5 * time.Second}).DialContext(ctx, "tcp", addrs[0]+":25")
			if dialErr == nil {
				entry.Reachable = true
				conn.Close()
			}
		}
		results = append(results, entry)
	}

	anyResolvable := false
	anyReachable := false
	for _, e := range results {
		if e.Resolvable {
			anyResolvable = true
		}
		if e.Reachable {
			anyReachable = true
		}
	}

	var status CheckStatus
	switch {
	case anyReachable:
		status = CheckStatusHealthy
	case anyResolvable:
		status = CheckStatusWarning
	default:
		status = CheckStatusCritical
	}

	return MXCheckResult{
		Status:  status,
		Results: results,
	}, nil
}

// propagationToAuthStatus maps a propagation result to an auth status string.
func propagationToAuthStatus(result dnssvc.PropagationResult) string {
	switch result.OverallStatus {
	case dnssvc.PropagationStatusPropagated:
		return "configured"
	case dnssvc.PropagationStatusPartial:
		return "misconfigured"
	default:
		return "missing"
	}
}

// AggregateStatus returns the worst status among the provided statuses.
func AggregateStatus(statuses ...CheckStatus) CheckStatus {
	worst := CheckStatusHealthy
	for _, s := range statuses {
		switch s {
		case CheckStatusCritical:
			return CheckStatusCritical
		case CheckStatusWarning:
			worst = CheckStatusWarning
		}
	}
	return worst
}
