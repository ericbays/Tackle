// Package randomizer defines interfaces for anti-fingerprinting randomization engines.
// Implementation: LiveHeaderRandomizer for HTTP response header randomization.
package randomizer

import (
	"fmt"
	"math/rand"
	"sort"
	"strings"
)

// LiveHeaderRandomizer implements the HeaderRandomizer interface for generating
// randomized HTTP response header profiles.
type LiveHeaderRandomizer struct {
	rd *rand.Rand
}

// headerSet represents a single header and its value.
type headerSet struct {
	name  string
	value string
}

// NewLiveHeaderRandomizer creates a new LiveHeaderRandomizer instance.
func NewLiveHeaderRandomizer() *LiveHeaderRandomizer {
	return &LiveHeaderRandomizer{}
}

// GenerateHeaderProfile produces a set of HTTP headers and Go middleware source code.
// Returns the header map, the Go source for a middleware function, and a manifest.
func (hr *LiveHeaderRandomizer) GenerateHeaderProfile(seed int64) (map[string]string, string, map[string]any, error) {
	// Create seeded random source for deterministic results
	hr.rd = rand.New(rand.NewSource(seed))

	profile := make(map[string]string)
	manifest := make(map[string]any)

	// Generate server header
	serverHeader, serverOmitted := hr.generateServerHeader()
	if !serverOmitted {
		profile["Server"] = serverHeader
	}
	manifest["strategy"] = "live"
	manifest["server_header"] = func() any {
		if serverOmitted {
			return "omitted"
		}
		return serverHeader
	}()

	// Generate X-Powered-By header
	pxbHeader, pxbOmitted := hr.generateXPoweredBy()
	if !pxbOmitted {
		profile["X-Powered-By"] = pxbHeader
	}
	manifest["x_powered_by"] = func() any {
		if pxbOmitted {
			return "omitted"
		}
		return pxbHeader
	}()

	// Generate Cache-Control header (always present)
	cacheControl := hr.generateCacheControl()
	profile["Cache-Control"] = cacheControl
	manifest["cache_control"] = cacheControl

	// Generate security headers
	securityHeaders := hr.generateSecurityHeaders()
	for k, v := range securityHeaders {
		profile[k] = v
	}
	manifest["security_headers_included"] = func() []string {
		keys := make([]string, 0, len(securityHeaders))
		for k := range securityHeaders {
			keys = append(keys, k)
		}
		return keys
	}()

	// Generate decoy headers
	decoyHeaders := hr.generateDecoyHeaders()
	for k, v := range decoyHeaders {
		profile[k] = v
	}
	manifest["decoy_headers_count"] = len(decoyHeaders)

	// Generate middleware source code
	middlewareSource := hr.generateMiddlewareSource(profile)

	// Calculate total headers
	manifest["total_headers"] = len(profile)

	return profile, middlewareSource, manifest, nil
}

// generateServerHeader selects a server header from the pool.
// Returns the header value and whether it was omitted.
func (hr *LiveHeaderRandomizer) generateServerHeader() (string, bool) {
	// 15% chance of omission
	if hr.rd.Intn(100) < 15 {
		return "", true
	}

	serverPool := []string{
		"nginx/1.18.0", "nginx/1.20.2", "nginx/1.22.1", "nginx/1.24.0",
		"Apache/2.4.41", "Apache/2.4.51", "Apache/2.4.57",
		"cloudflare",
		"Microsoft-IIS/10.0", "Microsoft-IIS/8.5",
	}

	idx := hr.rd.Intn(len(serverPool))
	return serverPool[idx], false
}

// generateXPoweredBy selects an X-Powered-By header from the pool.
// Returns the header value and whether it was omitted.
func (hr *LiveHeaderRandomizer) generateXPoweredBy() (string, bool) {
	// 20% chance of omission
	if hr.rd.Intn(100) < 20 {
		return "", true
	}

	pxbPool := []string{
		"Express",
		"ASP.NET",
		"PHP/7.4.33",
		"PHP/8.1.27",
		"Next.js",
		"Kestrel",
	}

	idx := hr.rd.Intn(len(pxbPool))
	return pxbPool[idx], false
}

// generateCacheControl selects a cache control directive.
func (hr *LiveHeaderRandomizer) generateCacheControl() string {
	cachePool := []string{
		"no-cache, no-store, must-revalidate",
		"public, max-age=3600",
		"public, max-age=86400",
		"private, max-age=0",
		"no-cache",
		"max-age=604800, stale-while-revalidate=86400",
	}

	idx := hr.rd.Intn(len(cachePool))
	return cachePool[idx]
}

// generateSecurityHeaders generates security headers with 50-70% inclusion rate.
func (hr *LiveHeaderRandomizer) generateSecurityHeaders() map[string]string {
	securityHeaders := make(map[string]string)

	// X-Content-Type-Options: 50-70% chance
	if hr.shouldIncludeSecurityHeader() {
		securityHeaders["X-Content-Type-Options"] = "nosniff"
	}

	// X-Frame-Options: 50-70% chance
	if hr.shouldIncludeSecurityHeader() {
		if hr.rd.Intn(2) == 0 {
			securityHeaders["X-Frame-Options"] = "DENY"
		} else {
			securityHeaders["X-Frame-Options"] = "SAMEORIGIN"
		}
	}

	// X-XSS-Protection: 50-70% chance
	if hr.shouldIncludeSecurityHeader() {
		if hr.rd.Intn(2) == 0 {
			securityHeaders["X-XSS-Protection"] = "1; mode=block"
		} else {
			securityHeaders["X-XSS-Protection"] = "0"
		}
	}

	// Referrer-Policy: 50-70% chance
	if hr.shouldIncludeSecurityHeader() {
		referrerPolicies := []string{
			"no-referrer",
			"strict-origin",
			"same-origin",
		}
		idx := hr.rd.Intn(len(referrerPolicies))
		securityHeaders["Referrer-Policy"] = referrerPolicies[idx]
	}

	// Permissions-Policy: 50-70% chance (may be omitted)
	if hr.shouldIncludeSecurityHeader() {
		// Only include 50% of the time
		if hr.rd.Intn(2) == 0 {
			securityHeaders["Permissions-Policy"] = "interest-cohort=()"
		}
	}

	return securityHeaders
}

// shouldIncludeSecurityHeader returns true if a security header should be included.
func (hr *LiveHeaderRandomizer) shouldIncludeSecurityHeader() bool {
	// 50-70% chance: generate random between 50 and 70
	minChance := 50
	maxChance := 70
	chance := minChance + hr.rd.Intn(maxChance-minChance+1)
	return hr.rd.Intn(100) < chance
}

// generateDecoyHeaders generates 1-3 custom decoy headers.
func (hr *LiveHeaderRandomizer) generateDecoyHeaders() map[string]string {
	decoyHeaders := make(map[string]string)

	// Determine number of decoy headers (1-3)
	numDecoys := 1 + hr.rd.Intn(3)

	// Define available decoy header types
	decoyTypes := []struct {
		name  string
		value func() string
	}{
		{
			name: "X-Request-ID",
			value: func() string {
				return hr.generateUUIDLike()
			},
		},
		{
			name: "X-Trace-ID",
			value: func() string {
				return hr.generateHexID()
			},
		},
		{
			name: "X-Cache",
			value: func() string {
				if hr.rd.Intn(2) == 0 {
					return "HIT"
				}
				return "MISS"
			},
		},
		{
			name: "X-Served-By",
			value: func() string {
				hosts := []string{
					"proxy-1.local",
					"gateway-2.internal",
					"web-frontend-01",
					"api-gateway-02",
				}
				return hosts[hr.rd.Intn(len(hosts))]
			},
		},
		{
			name: "X-Runtime",
			value: func() string {
				// Generate a random float like 0.042
				ms := 10 + hr.rd.Intn(190) // 10-200ms
				return fmt.Sprintf("0.%03d", ms)
			},
		},
		{
			name: "Via",
			value: func() string {
				hosts := []string{
					"1.1 proxy-1.local",
					"1.1 gateway-2.internal",
					"1.1 web-frontend-01",
				}
				return hosts[hr.rd.Intn(len(hosts))]
			},
		},
	}

	// Shuffle the decoy types
	hr.shuffleDecoyTypes(decoyTypes)

	// Select and add decoy headers
	for i := 0; i < numDecoys; i++ {
		decoy := decoyTypes[i]
		decoyHeaders[decoy.name] = decoy.value()
	}

	return decoyHeaders
}

// shuffleDecoyTypes shuffles the decoy types using Fisher-Yates.
func (hr *LiveHeaderRandomizer) shuffleDecoyTypes(decoyTypes []struct {
	name  string
	value func() string
}) {
	for i := len(decoyTypes) - 1; i > 0; i-- {
		j := hr.rd.Intn(i + 1)
		decoyTypes[i], decoyTypes[j] = decoyTypes[j], decoyTypes[i]
	}
}

// generateUUIDLike generates a UUID-like string.
func (hr *LiveHeaderRandomizer) generateUUIDLike() string {
	const hex = "0123456789abcdef"
	result := make([]byte, 36)
	for i := range result {
		if i == 8 || i == 13 || i == 18 || i == 23 {
			result[i] = '-'
		} else if i == 14 {
			result[i] = '4'
		} else if i == 19 {
			result[i] = byte((hr.rd.Intn(4) + 8) + '0') // 8-11
		} else {
			result[i] = hex[hr.rd.Intn(16)]
		}
	}
	return string(result)
}

// generateHexID generates a random hex string.
func (hr *LiveHeaderRandomizer) generateHexID() string {
	const hex = "0123456789abcdef"
	const length = 16
	result := make([]byte, length)
	for i := range result {
		result[i] = hex[hr.rd.Intn(16)]
	}
	return string(result)
}

// generateMiddlewareSource generates Go source code for the header middleware.
func (hr *LiveHeaderRandomizer) generateMiddlewareSource(profile map[string]string) string {
	var builder strings.Builder

	// Collect keys in sorted order for determinism, then shuffle
	keys := make([]string, 0, len(profile))
	for k := range profile {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	headerSets := make([]headerSet, 0, len(profile))
	for _, k := range keys {
		headerSets = append(headerSets, headerSet{name: k, value: profile[k]})
	}

	// Shuffle the header set order using the seeded random source
	hr.shuffleHeaderSets(headerSets)

	// Generate middleware source
	builder.WriteString("func ApplyHeaders(next http.Handler) http.Handler {\n")
	builder.WriteString("    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {\n")

	// Add header set calls
	for _, hs := range headerSets {
		builder.WriteString(fmt.Sprintf("        w.Header().Set(%q, %q)\n", hs.name, hs.value))
	}

	builder.WriteString("        next.ServeHTTP(w, r)\n")
	builder.WriteString("    })\n")
	builder.WriteString("}\n")

	return builder.String()
}

// shuffleHeaderSets shuffles the header sets using Fisher-Yates.
func (hr *LiveHeaderRandomizer) shuffleHeaderSets(headerSets []headerSet) {
	for i := len(headerSets) - 1; i > 0; i-- {
		j := hr.rd.Intn(i + 1)
		headerSets[i], headerSets[j] = headerSets[j], headerSets[i]
	}
}
