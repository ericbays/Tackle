package blocklist

import (
	"testing"

	"tackle/internal/repositories"
)

func TestValidatePattern(t *testing.T) {
	tests := []struct {
		name    string
		pattern string
		wantErr bool
	}{
		// Valid patterns.
		{"exact email", "user@domain.com", false},
		{"domain wildcard", "*@domain.com", false},
		{"subdomain wildcard", "*@*.domain.com", false},
		{"subdomain wildcard gov", "*@*.gov", false},

		// Invalid patterns.
		{"empty string", "", true},
		{"just star", "*", true},
		{"star at star", "*@*", true},
		{"double at", "@@", true},
		{"no domain", "user@", true},
		{"no at sign", "userdomain.com", true},
		{"no local part", "@domain.com", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validatePattern(tt.pattern)
			if (err != nil) != tt.wantErr {
				t.Errorf("validatePattern(%q) error = %v, wantErr %v", tt.pattern, err, tt.wantErr)
			}
		})
	}
}

func TestMatchesPattern(t *testing.T) {
	tests := []struct {
		name    string
		email   string
		pattern string
		want    bool
	}{
		// Exact email match.
		{"exact match", "ceo@company.com", "ceo@company.com", true},
		{"exact match case insensitive", "ceo@company.com", "CEO@Company.com", true},
		{"exact no match", "cto@company.com", "ceo@company.com", false},

		// Domain wildcard: *@domain.com
		{"domain wildcard match", "anyone@legal.company.com", "*@legal.company.com", true},
		{"domain wildcard match 2", "user@domain.com", "*@domain.com", true},
		{"domain wildcard no match wrong domain", "user@other.com", "*@domain.com", false},
		{"domain wildcard no match subdomain", "user@sub.domain.com", "*@domain.com", false},

		// Subdomain wildcard: *@*.domain.com
		{"subdomain wildcard match", "user@sub.company.com", "*@*.company.com", true},
		{"subdomain wildcard match deep", "user@deep.sub.company.com", "*@*.company.com", true},
		{"subdomain wildcard no match bare", "user@company.com", "*@*.company.com", false},
		{"subdomain wildcard no match other", "user@other.com", "*@*.company.com", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			email := tt.email
			domain := email[lastIndexByte(email, '@')+1:]
			pattern := tt.pattern

			got := matchesPattern(email, domain, pattern)
			if got != tt.want {
				t.Errorf("matchesPattern(%q, %q, %q) = %v, want %v", email, domain, pattern, got, tt.want)
			}
		})
	}
}

func TestMatchEmail(t *testing.T) {
	entries := []repositories.BlocklistEntry{
		{ID: "1", Pattern: "ceo@company.com", Reason: "C-suite", IsActive: true},
		{ID: "2", Pattern: "*@legal.company.com", Reason: "Legal dept", IsActive: true},
		{ID: "3", Pattern: "*@*.gov", Reason: "Government", IsActive: true},
		{ID: "4", Pattern: "*@inactive.com", Reason: "Inactive", IsActive: false},
	}

	tests := []struct {
		name      string
		email     string
		wantCount int
	}{
		{"exact match", "ceo@company.com", 1},
		{"domain match", "lawyer@legal.company.com", 1},
		{"subdomain match", "user@dept.gov", 1},
		{"no match", "user@safe.com", 0},
		{"inactive not matched", "user@inactive.com", 0},
		{"case insensitive", "CEO@Company.com", 1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			matches := matchEmail(tt.email, entries)
			if len(matches) != tt.wantCount {
				t.Errorf("matchEmail(%q) got %d matches, want %d", tt.email, len(matches), tt.wantCount)
			}
		})
	}
}

func TestComputeTargetHash(t *testing.T) {
	targets1 := []repositories.BlockedTargetInfo{
		{TargetID: "a", Pattern: "p1"},
		{TargetID: "b", Pattern: "p2"},
	}
	targets2 := []repositories.BlockedTargetInfo{
		{TargetID: "b", Pattern: "p2"},
		{TargetID: "a", Pattern: "p1"},
	}
	targets3 := []repositories.BlockedTargetInfo{
		{TargetID: "a", Pattern: "p1"},
		{TargetID: "c", Pattern: "p3"},
	}

	h1 := computeTargetHash(targets1)
	h2 := computeTargetHash(targets2)
	h3 := computeTargetHash(targets3)

	if h1 != h2 {
		t.Error("same targets in different order should produce same hash")
	}
	if h1 == h3 {
		t.Error("different targets should produce different hash")
	}
}

func lastIndexByte(s string, c byte) int {
	for i := len(s) - 1; i >= 0; i-- {
		if s[i] == c {
			return i
		}
	}
	return -1
}
