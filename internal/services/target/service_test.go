package target

import (
	"testing"
)

func TestValidateEmail(t *testing.T) {
	tests := []struct {
		name    string
		email   string
		wantErr bool
	}{
		{"valid simple", "user@example.com", false},
		{"valid plus addressing", "user+tag@example.com", false},
		{"valid dotted local", "first.last@example.com", false},
		{"valid subdomain", "user@sub.example.com", false},
		{"empty", "", true},
		{"no at sign", "userexample.com", true},
		{"no domain", "user@", true},
		{"no local", "@example.com", true},
		{"spaces", "user @example.com", true},
		{"double at", "user@@example.com", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateEmail(tt.email)
			if (err != nil) != tt.wantErr {
				t.Errorf("validateEmail(%q) error = %v, wantErr %v", tt.email, err, tt.wantErr)
			}
			if err != nil {
				if _, ok := err.(*ValidationError); !ok {
					t.Errorf("validateEmail(%q) returned %T, want *ValidationError", tt.email, err)
				}
			}
		})
	}
}

func TestValidateCustomFields(t *testing.T) {
	tests := []struct {
		name    string
		cf      map[string]any
		wantErr bool
	}{
		{"nil is valid", nil, false},
		{"empty is valid", map[string]any{}, false},
		{"simple fields", map[string]any{"dept": "Engineering", "floor": "3"}, false},
		{"too many keys", makeNKeys(51), true},
		{"long key", map[string]any{string(make([]byte, 65)): "val"}, true},
		{"long value", map[string]any{"key": string(make([]byte, 1025))}, true},
		{"secret-like api key", map[string]any{"key": "api_key: sk-abc123456789012345"}, true},
		{"secret-like jwt", map[string]any{"token": "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJzdWIiOiIxIn0"}, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateCustomFields(tt.cf)
			if (err != nil) != tt.wantErr {
				t.Errorf("validateCustomFields() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestMaskEmail(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"alice@example.com", "a**@example.com"},
		{"b@example.com", "b**@example.com"},
		{"charlie.brown@test.co", "c**@test.co"},
		{"noatsign", "***"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := maskEmail(tt.input)
			if got != tt.expected {
				t.Errorf("maskEmail(%q) = %q, want %q", tt.input, got, tt.expected)
			}
		})
	}
}

func TestSanitizeCSVCell(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"normal text", "normal text"},
		{"=SUM(A1:A10)", "SUM(A1:A10)"},
		{"+cmd('calc')", "cmd('calc')"},
		{"-10+20", "10+20"},
		{"@SUM(A1)", "SUM(A1)"},
		{"\tcmd", "cmd"},
		{"\rcmd", "cmd"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := sanitizeCSVCell(tt.input)
			if got != tt.expected {
				t.Errorf("sanitizeCSVCell(%q) = %q, want %q", tt.input, got, tt.expected)
			}
		})
	}
}

func TestLooksLikeCSV(t *testing.T) {
	tests := []struct {
		name     string
		data     []byte
		expected bool
	}{
		{"valid csv", []byte("name,email\njohn,john@test.com\n"), true},
		{"empty", []byte{}, false},
		{"binary null bytes", []byte{0x00, 0x01, 0x02}, false},
		{"single header line", []byte("col1,col2"), true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := looksLikeCSV(tt.data)
			if got != tt.expected {
				t.Errorf("looksLikeCSV(%q) = %v, want %v", tt.name, got, tt.expected)
			}
		})
	}
}

func TestStrPtr(t *testing.T) {
	// Non-empty returns pointer.
	s := strPtr("hello")
	if s == nil || *s != "hello" {
		t.Error("strPtr(\"hello\") should return pointer to \"hello\"")
	}
	// Empty returns nil.
	s = strPtr("")
	if s != nil {
		t.Error("strPtr(\"\") should return nil")
	}
}

// makeNKeys creates a map with n keys for testing limits.
func makeNKeys(n int) map[string]any {
	m := make(map[string]any, n)
	for i := 0; i < n; i++ {
		m[string(rune('a'+i%26))+string(rune('0'+i/26))] = "v"
	}
	return m
}
