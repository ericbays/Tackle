package notification

import (
	"testing"
)

func TestValidateWebhookURL_RejectsPrivateIPs(t *testing.T) {
	cases := []struct {
		name    string
		url     string
		wantErr bool
	}{
		{"loopback", "http://127.0.0.1/hook", true},
		{"private 192", "http://192.168.1.1/hook", true},
		{"private 10", "http://10.0.0.1/hook", true},
		{"localhost", "http://localhost/hook", true},
		{"ftp scheme", "ftp://example.com/hook", true},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := validateWebhookURL(tc.url)
			if tc.wantErr && err == nil {
				t.Errorf("validateWebhookURL(%q) = nil, want error", tc.url)
			}
		})
	}
}

func TestParsePostgresArray(t *testing.T) {
	cases := []struct {
		input string
		want  int
	}{
		{"{}", 0},
		{"{campaign}", 1},
		{"{campaign,alert,system}", 3},
		{"", 0},
	}

	for _, tc := range cases {
		result := parsePostgresArray(tc.input)
		if len(result) != tc.want {
			t.Errorf("parsePostgresArray(%q) = %d items, want %d", tc.input, len(result), tc.want)
		}
	}
}

func TestTruncate(t *testing.T) {
	if got := truncate("hello world", 5); got != "hello" {
		t.Errorf("truncate(\"hello world\", 5) = %q, want \"hello\"", got)
	}
	if got := truncate("hi", 5); got != "hi" {
		t.Errorf("truncate(\"hi\", 5) = %q, want \"hi\"", got)
	}
}
