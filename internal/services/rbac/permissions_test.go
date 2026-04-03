package rbac

import (
	"sort"
	"testing"
)

func TestRegistry_ContainsExpectedPermissions(t *testing.T) {
	expected := []Permission{
		"users:create", "users:read", "users:update", "users:delete", "users:export",
		"roles:create", "roles:read", "roles:update", "roles:delete",
		"campaigns:create", "campaigns:read", "campaigns:update", "campaigns:delete",
		"campaigns:execute", "campaigns:export",
		"targets:create", "targets:read", "targets:update", "targets:delete", "targets:export",
		"templates.email:create", "templates.email:read", "templates.email:update",
		"templates.email:delete", "templates.email:export",
		"templates.landing:create", "templates.landing:read", "templates.landing:update",
		"templates.landing:delete", "templates.landing:execute", "templates.landing:export",
		"domains:create", "domains:read", "domains:update", "domains:delete",
		"endpoints:create", "endpoints:read", "endpoints:update", "endpoints:delete",
		"endpoints:execute",
		"smtp:create", "smtp:read", "smtp:update", "smtp:delete", "smtp:execute",
		"credentials:read", "credentials:delete", "credentials:export",
		"reports:create", "reports:read", "reports:delete", "reports:export",
		"metrics:read", "metrics:export",
		"logs.audit:read", "logs.audit:export",
		"logs.campaign:read", "logs.campaign:export",
		"logs.system:read", "logs.system:export",
		"settings:read", "settings:update",
		"settings.auth:read", "settings.auth:update",
		"cloud:create", "cloud:read", "cloud:update", "cloud:delete",
		"infrastructure.requests:create", "infrastructure.requests:read",
		"infrastructure.requests:update", "infrastructure.requests:approve",
		"schedules:create", "schedules:read", "schedules:update", "schedules:delete",
		"schedules:execute",
		"notifications:create", "notifications:read", "notifications:update",
		"notifications:delete",
		"api_keys:create", "api_keys:read", "api_keys:delete",
	}

	for _, p := range expected {
		if _, ok := Registry[p]; !ok {
			t.Errorf("expected permission %q not in registry", p)
		}
	}
}

func TestValid_RejectsUnknown(t *testing.T) {
	cases := []struct {
		perm  Permission
		valid bool
	}{
		{"campaigns:read", true},
		{"users:export", true},
		{"nope:nope", false},
		{"campaigns:hack", false},
		{"", false},
		{"roles:execute", false}, // execute not in roles matrix
	}
	for _, tc := range cases {
		got := Valid(tc.perm)
		if got != tc.valid {
			t.Errorf("Valid(%q) = %v, want %v", tc.perm, got, tc.valid)
		}
	}
}

func TestAll_IsSorted(t *testing.T) {
	all := All()
	if len(all) == 0 {
		t.Fatal("All() returned empty slice")
	}
	if !sort.SliceIsSorted(all, func(i, j int) bool { return all[i] < all[j] }) {
		t.Error("All() is not sorted")
	}
}

func TestAll_MatchesRegistry(t *testing.T) {
	all := All()
	if len(all) != len(Registry) {
		t.Errorf("All() len=%d, registry len=%d", len(all), len(Registry))
	}
	for _, p := range all {
		if _, ok := Registry[p]; !ok {
			t.Errorf("All() contains %q not in registry", p)
		}
	}
}
