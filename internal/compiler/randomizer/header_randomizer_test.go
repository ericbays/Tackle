// Package randomizer defines interfaces for anti-fingerprinting randomization engines.
// Tests for LiveHeaderRandomizer.
package randomizer

import (
	"regexp"
	"strings"
	"testing"
)

var _ HeaderRandomizer = &LiveHeaderRandomizer{}

// TestHeaderProfilePopulated verifies the returned map has at least 2 entries.
func TestHeaderProfilePopulated(t *testing.T) {
	rand := NewLiveHeaderRandomizer()

	profile, _, manifest, err := rand.GenerateHeaderProfile(42)

	if err != nil {
		t.Fatalf("GenerateHeaderProfile returned error: %v", err)
	}

	if len(profile) < 2 {
		t.Errorf("Expected at least 2 headers, got %d", len(profile))
	}

	// Verify manifest fields exist
	if _, ok := manifest["strategy"]; !ok {
		t.Error("Manifest missing 'strategy' field")
	}
	if _, ok := manifest["server_header"]; !ok {
		t.Error("Manifest missing 'server_header' field")
	}
	if _, ok := manifest["x_powered_by"]; !ok {
		t.Error("Manifest missing 'x_powered_by' field")
	}
	if _, ok := manifest["cache_control"]; !ok {
		t.Error("Manifest missing 'cache_control' field")
	}
}

// TestServerHeaderPool verifies Server header comes from the specified pool or is omitted.
func TestServerHeaderPool(t *testing.T) {
	rand := NewLiveHeaderRandomizer()

	serverPool := []string{
		"nginx/1.18.0", "nginx/1.20.2", "nginx/1.22.1", "nginx/1.24.0",
		"Apache/2.4.41", "Apache/2.4.51", "Apache/2.4.57",
		"cloudflare",
		"Microsoft-IIS/10.0", "Microsoft-IIS/8.5",
	}

	// Test across multiple seeds
	for i := 0; i < 20; i++ {
		profile, _, manifest, err := rand.GenerateHeaderProfile(int64(i))

		if err != nil {
			t.Fatalf("Seed %d: GenerateHeaderProfile returned error: %v", i, err)
		}

		serverHeader, ok := manifest["server_header"]
		if !ok {
			t.Errorf("Seed %d: manifest missing 'server_header'", i)
			continue
		}

		serverVal, ok := serverHeader.(string)
		if !ok {
			t.Errorf("Seed %d: server_header is not a string: %v", i, serverHeader)
			continue
		}

		// Verify it's either in the pool or "omitted"
		if serverVal != "omitted" {
			found := false
			for _, val := range serverPool {
				if val == serverVal {
					found = true
					break
				}
			}
			if !found {
				t.Errorf("Seed %d: server header %q not in pool", i, serverVal)
			}
		}

		// Check if Server header is present in profile when not omitted
		if serverVal != "omitted" {
			if _, ok := profile["Server"]; !ok {
				t.Errorf("Seed %d: Server header %q not in profile", i, serverVal)
			}
		} else {
			if _, ok := profile["Server"]; ok {
				t.Errorf("Seed %d: Server header should be omitted but present in profile", i)
			}
		}
	}
}

// TestCacheControlAlwaysPresent verifies Cache-Control is present in every profile.
func TestCacheControlAlwaysPresent(t *testing.T) {
	rand := NewLiveHeaderRandomizer()

	cachePool := []string{
		"no-cache, no-store, must-revalidate",
		"public, max-age=3600",
		"public, max-age=86400",
		"private, max-age=0",
		"no-cache",
		"max-age=604800, stale-while-revalidate=86400",
	}

	// Test across multiple seeds
	for i := 0; i < 20; i++ {
		profile, _, manifest, err := rand.GenerateHeaderProfile(int64(i))

		if err != nil {
			t.Fatalf("Seed %d: GenerateHeaderProfile returned error: %v", i, err)
		}

		// Check profile
		if _, ok := profile["Cache-Control"]; !ok {
			t.Errorf("Seed %d: Cache-Control missing from profile", i)
		}

		// Check manifest
		cacheControl, ok := manifest["cache_control"]
		if !ok {
			t.Errorf("Seed %d: cache_control missing from manifest", i)
			continue
		}

		cacheVal, ok := cacheControl.(string)
		if !ok {
			t.Errorf("Seed %d: cache_control is not a string: %v", i, cacheControl)
			continue
		}

		// Verify it's one of the expected values
		found := false
		for _, val := range cachePool {
			if val == cacheVal {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("Seed %d: cache control %q not in pool", i, cacheVal)
		}
	}
}

// TestMiddlewareSourceValid verifies the source string contains expected elements.
func TestMiddlewareSourceValid(t *testing.T) {
	rand := NewLiveHeaderRandomizer()

	_, middlewareSource, _, err := rand.GenerateHeaderProfile(42)

	if err != nil {
		t.Fatalf("GenerateHeaderProfile returned error: %v", err)
	}

	// Verify required elements are present
	requiredElements := []string{
		"func ApplyHeaders",
		"http.Handler",
		"w.Header().Set",
		"next.ServeHTTP",
	}

	for _, elem := range requiredElements {
		if !strings.Contains(middlewareSource, elem) {
			t.Errorf("Middleware source missing %q", elem)
		}
	}

	// Verify it's valid Go syntax (basic check)
	importPattern := regexp.MustCompile(`func ApplyHeaders\(next http\.Handler\) http\.Handler \{`)
	if !importPattern.MatchString(middlewareSource) {
		t.Errorf("Middleware source does not match expected function signature")
	}
}

// TestHeaderDeterminism verifies same seed produces identical output.
func TestHeaderDeterminism(t *testing.T) {
	rand := NewLiveHeaderRandomizer()

	seed := int64(12345)

	// Run 5 times with same seed
	var profiles []map[string]string
	var sources []string
	var manifests []map[string]any

	for i := 0; i < 5; i++ {
		profile, source, manifest, err := rand.GenerateHeaderProfile(seed)

		if err != nil {
			t.Fatalf("Run %d: GenerateHeaderProfile returned error: %v", i, err)
		}

		profiles = append(profiles, profile)
		sources = append(sources, source)
		manifests = append(manifests, manifest)
	}

	// Compare all runs to the first run
	for i := 1; i < 5; i++ {
		// Compare profiles
		if len(profiles[0]) != len(profiles[i]) {
			t.Errorf("Run %d: profile length mismatch: %d vs %d", i, len(profiles[0]), len(profiles[i]))
		}
		for k, v := range profiles[0] {
			if profiles[i][k] != v {
				t.Errorf("Run %d: profile[%q] mismatch: %q vs %q", i, k, v, profiles[i][k])
			}
		}

		// Compare sources
		if sources[0] != sources[i] {
			t.Errorf("Run %d: middleware source mismatch", i)
		}

		// Compare manifests
		if len(manifests[0]) != len(manifests[i]) {
			t.Errorf("Run %d: manifest length mismatch: %d vs %d", i, len(manifests[0]), len(manifests[i]))
		}
		for k, v := range manifests[0] {
			vi := manifests[i][k]
			switch val := v.(type) {
			case string:
				if vi != val {
					t.Errorf("Run %d: manifest[%q] mismatch: %v vs %v", i, k, v, vi)
				}
			case int:
				if vi != val {
					t.Errorf("Run %d: manifest[%q] mismatch: %v vs %v", i, k, v, vi)
				}
			case []string:
				viSlice, ok := vi.([]string)
				if !ok || len(val) != len(viSlice) {
					t.Errorf("Run %d: manifest[%q] mismatch: %v vs %v", i, k, v, vi)
				}
			}
		}
	}
}

// TestHeaderDivergence verifies different seeds produce different profiles.
func TestHeaderDivergence(t *testing.T) {
	rand := NewLiveHeaderRandomizer()

	// Test with different seeds
	seeds := []int64{1, 100, 500, 1000}
	profiles := make([]map[string]string, len(seeds))

	for i, seed := range seeds {
		profile, _, _, err := rand.GenerateHeaderProfile(seed)
		if err != nil {
			t.Fatalf("Seed %d: GenerateHeaderProfile returned error: %v", seed, err)
		}
		profiles[i] = profile
	}

	// Compare profiles and count header differences
	for i := 1; i < len(profiles); i++ {
		diffCount := 0
		for k, v := range profiles[0] {
			if profiles[i][k] != v {
				diffCount++
			}
		}
		if diffCount < 3 {
			t.Errorf("Seed %d: expected at least 3 header differences, got %d", seeds[i], diffCount)
		}
	}
}

// TestHeaderManifestCompleteness verifies all required manifest fields are present.
func TestHeaderManifestCompleteness(t *testing.T) {
	rand := NewLiveHeaderRandomizer()

	_, _, manifest, err := rand.GenerateHeaderProfile(42)

	if err != nil {
		t.Fatalf("GenerateHeaderProfile returned error: %v", err)
	}

	// Required fields with expected types
	requiredFields := map[string]any{
		"strategy":                  "live",
		"server_header":             "",
		"x_powered_by":              "",
		"cache_control":             "",
		"security_headers_included": []string{},
		"decoy_headers_count":       0,
		"total_headers":             0,
	}

	for field, expectedVal := range requiredFields {
		val, ok := manifest[field]
		if !ok {
			t.Errorf("Manifest missing field %q", field)
			continue
		}

		switch v := val.(type) {
		case string:
			if expectedVal != "" {
				if v != expectedVal.(string) {
					t.Errorf("Manifest field %q value mismatch: got %q, expected %q", field, v, expectedVal)
				}
			}
		case int:
			if expectedVal != 0 {
				if v != expectedVal.(int) {
					t.Errorf("Manifest field %q value mismatch: got %d, expected %d", field, v, expectedVal)
				}
			}
		case []string:
			if len(expectedVal.([]string)) > 0 && len(v) == 0 {
				t.Errorf("Manifest field %q: expected non-empty array", field)
			}
		case []any:
			if len(expectedVal.([]any)) > 0 && len(v) == 0 {
				t.Errorf("Manifest field %q: expected non-empty array", field)
			}
		}
	}
}

// TestHeaderInterfaceCompatibility verifies the type implements HeaderRandomizer.
func TestHeaderInterfaceCompatibility(t *testing.T) {
	var _ HeaderRandomizer = &LiveHeaderRandomizer{}
	rand := NewLiveHeaderRandomizer()
	_ = rand
}

// TestDecoyHeadersPresent verifies at least 1 custom header in every profile.
func TestDecoyHeadersPresent(t *testing.T) {
	rand := NewLiveHeaderRandomizer()

	// Test across multiple seeds
	for i := 0; i < 20; i++ {
		_, _, manifest, err := rand.GenerateHeaderProfile(int64(i))

		if err != nil {
			t.Fatalf("Seed %d: GenerateHeaderProfile returned error: %v", i, err)
		}

		decoyCount, ok := manifest["decoy_headers_count"]
		if !ok {
			t.Errorf("Seed %d: manifest missing 'decoy_headers_count'", i)
			continue
		}

		count, ok := decoyCount.(int)
		if !ok {
			t.Errorf("Seed %d: decoy_headers_count is not an int: %v", i, decoyCount)
			continue
		}

		if count < 1 {
			t.Errorf("Seed %d: expected at least 1 decoy header, got %d", i, count)
		}
	}
}

// TestSecurityHeadersValid verifies all included security headers have valid values.
func TestSecurityHeadersValid(t *testing.T) {
	rand := NewLiveHeaderRandomizer()

	// Valid values for each security header
	securityValues := map[string][]string{
		"X-Content-Type-Options": {"nosniff"},
		"X-Frame-Options":        {"DENY", "SAMEORIGIN"},
		"X-XSS-Protection":       {"1; mode=block", "0"},
		"Referrer-Policy":        {"no-referrer", "strict-origin", "same-origin"},
		"Permissions-Policy":     {"interest-cohort=()"},
	}

	// Test across multiple seeds
	for i := 0; i < 20; i++ {
		profile, _, manifest, err := rand.GenerateHeaderProfile(int64(i))

		if err != nil {
			t.Fatalf("Seed %d: GenerateHeaderProfile returned error: %v", i, err)
		}

		securityHeaders, ok := manifest["security_headers_included"]
		if !ok {
			continue
		}

		securityHeaderList, ok := securityHeaders.([]string)
		if !ok {
			continue
		}

		for _, header := range securityHeaderList {
			val, ok := profile[header]
			if !ok {
				t.Errorf("Seed %d: security header %q missing from profile", i, header)
				continue
			}

			validValues, ok := securityValues[header]
			if !ok {
				t.Errorf("Seed %d: unknown security header %q", i, header)
				continue
			}

			valid := false
			for _, v := range validValues {
				if v == val {
					valid = true
					break
				}
			}
			if !valid {
				t.Errorf("Seed %d: security header %q has invalid value %q", i, header, val)
			}
		}
	}
}

// TestHeaderNamesValid verifies all header names match HTTP header format.
func TestHeaderNamesValid(t *testing.T) {
	rand := NewLiveHeaderRandomizer()

	// HTTP header names: visible ASCII, no spaces or colons
	headerNamePattern := regexp.MustCompile(`^[A-Za-z0-9\-]+$`)

	// Test across multiple seeds
	for i := 0; i < 20; i++ {
		profile, _, _, err := rand.GenerateHeaderProfile(int64(i))

		if err != nil {
			t.Fatalf("Seed %d: GenerateHeaderProfile returned error: %v", i, err)
		}

		for name := range profile {
			if !headerNamePattern.MatchString(name) {
				t.Errorf("Seed %d: invalid header name %q", i, name)
			}
			if strings.Contains(name, " ") {
				t.Errorf("Seed %d: header name contains space: %q", i, name)
			}
		}
	}
}

// TestTotalHeadersCount verifies manifest total_headers matches actual map length.
func TestTotalHeadersCount(t *testing.T) {
	rand := NewLiveHeaderRandomizer()

	// Test across multiple seeds
	for i := 0; i < 20; i++ {
		profile, _, manifest, err := rand.GenerateHeaderProfile(int64(i))

		if err != nil {
			t.Fatalf("Seed %d: GenerateHeaderProfile returned error: %v", i, err)
		}

		totalHeaders, ok := manifest["total_headers"]
		if !ok {
			t.Errorf("Seed %d: manifest missing 'total_headers'", i)
			continue
		}

		count, ok := totalHeaders.(int)
		if !ok {
			t.Errorf("Seed %d: total_headers is not an int: %v", i, totalHeaders)
			continue
		}

		if count != len(profile) {
			t.Errorf("Seed %d: total_headers (%d) != profile length (%d)", i, count, len(profile))
		}
	}
}

// TestMiddlewareSourceSyntax verifies the middleware source is valid Go code.
func TestMiddlewareSourceSyntax(t *testing.T) {
	rand := NewLiveHeaderRandomizer()

	_, middlewareSource, _, err := rand.GenerateHeaderProfile(42)

	if err != nil {
		t.Fatalf("GenerateHeaderProfile returned error: %v", err)
	}

	// Verify basic structure
	requiredPatterns := []struct {
		pattern string
		desc    string
	}{
		{`func ApplyHeaders\(next http\.Handler\) http\.Handler`, "function signature"},
		{`http\.HandlerFunc`, "HandlerFunc type"},
		{`w\.Header\(\)\.Set`, "Header set calls"},
		{`next\.ServeHTTP\(w, r\)`, "next handler call"},
	}

	for _, p := range requiredPatterns {
		re := regexp.MustCompile(p.pattern)
		if !re.MatchString(middlewareSource) {
			t.Errorf("Middleware source missing pattern for %s: %s", p.desc, p.pattern)
		}
	}
}

// BenchmarkGenerateHeaderProfile benchmarks the header profile generation.
func BenchmarkGenerateHeaderProfile(b *testing.B) {
	rand := NewLiveHeaderRandomizer()

	for i := 0; i < b.N; i++ {
		_, _, _, _ = rand.GenerateHeaderProfile(int64(i))
	}
}
