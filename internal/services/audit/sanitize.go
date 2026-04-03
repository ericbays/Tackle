package audit

import "strings"

// secretFields is the set of field name substrings that indicate a secret value.
// Checked case-insensitively.
var secretFields = []string{
	"password",
	"token",
	"secret",
	"api_key",
	"apikey",
	"private_key",
	"privatekey",
	"client_secret",
	"refresh_token",
	"access_token",
}

// injectionReplacer encodes log-injection characters in string values.
var injectionReplacer = strings.NewReplacer(
	"\n", `\n`,
	"\r", `\r`,
	"\t", `\t`,
	"\x00", `\0`,
)

// SanitizeDetails returns a sanitized copy of details.
// Secret field names (case-insensitive) have their values replaced with "[REDACTED]".
// String values containing newline, carriage-return, tab, or NUL characters have
// those characters encoded to prevent log injection attacks.
// Nested maps are recursively sanitized.
func SanitizeDetails(details map[string]any) map[string]any {
	if details == nil {
		return nil
	}
	out := make(map[string]any, len(details))
	for k, v := range details {
		if isSecretField(k) {
			out[k] = "[REDACTED]"
			continue
		}
		out[k] = sanitizeValue(v)
	}
	return out
}

// isSecretField returns true if the key name contains a known secret substring.
func isSecretField(key string) bool {
	lower := strings.ToLower(key)
	for _, s := range secretFields {
		if strings.Contains(lower, s) {
			return true
		}
	}
	return false
}

// sanitizeValue recursively sanitizes a value.
func sanitizeValue(v any) any {
	switch val := v.(type) {
	case string:
		return injectionReplacer.Replace(val)
	case map[string]any:
		return SanitizeDetails(val)
	default:
		return v
	}
}
