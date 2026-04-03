package rbac

import (
	"testing"
)

func TestIsBuiltin(t *testing.T) {
	cases := []struct {
		name     string
		expected bool
	}{
		{RoleAdmin, true},
		{RoleEngineer, true},
		{RoleOperator, true},
		{RoleDefender, true},
		{"custom", false},
		{"", false},
		{"ADMIN", false}, // case-sensitive
	}
	for _, tc := range cases {
		got := IsBuiltin(tc.name)
		if got != tc.expected {
			t.Errorf("IsBuiltin(%q) = %v, want %v", tc.name, got, tc.expected)
		}
	}
}

func TestIsAdmin(t *testing.T) {
	if !IsAdmin(RoleAdmin) {
		t.Error("IsAdmin(admin) should be true")
	}
	if IsAdmin(RoleEngineer) {
		t.Error("IsAdmin(engineer) should be false")
	}
}

func TestBuiltinPermissions_AdminShortCircuits(t *testing.T) {
	perms, err := BuiltinPermissions(RoleAdmin)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if perms != nil {
		t.Error("admin permissions should be nil (short-circuit)")
	}
}

func TestBuiltinPermissions_UnknownRole(t *testing.T) {
	_, err := BuiltinPermissions("unknown")
	if err == nil {
		t.Error("expected error for unknown role")
	}
}

func TestBuiltinPermissions_Engineer(t *testing.T) {
	perms, err := BuiltinPermissions(RoleEngineer)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(perms) == 0 {
		t.Fatal("engineer should have permissions")
	}

	permSet := make(map[Permission]struct{}, len(perms))
	for _, p := range perms {
		permSet[p] = struct{}{}
	}

	// Engineer should NOT have user management or role management.
	forbidden := []Permission{
		"users:create", "users:update", "users:delete",
		"roles:create", "roles:update", "roles:delete",
		"settings.auth:read", "settings.auth:update",
		"campaigns:create", "campaigns:update", "campaigns:delete", "campaigns:execute",
	}
	for _, p := range forbidden {
		if _, ok := permSet[p]; ok {
			t.Errorf("engineer should NOT have permission %q", p)
		}
	}

	// Engineer must have infrastructure permissions.
	required := []Permission{
		"endpoints:create", "endpoints:read", "endpoints:update", "endpoints:delete", "endpoints:execute",
		"domains:create", "domains:read", "domains:update", "domains:delete",
		"infrastructure.requests:approve",
		"cloud:create", "cloud:read",
		"settings:read",
	}
	for _, p := range required {
		if _, ok := permSet[p]; !ok {
			t.Errorf("engineer should have permission %q", p)
		}
	}
}

func TestBuiltinPermissions_Operator(t *testing.T) {
	perms, err := BuiltinPermissions(RoleOperator)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	permSet := make(map[Permission]struct{}, len(perms))
	for _, p := range perms {
		permSet[p] = struct{}{}
	}

	// Operator CANNOT approve infrastructure requests.
	if _, ok := permSet["infrastructure.requests:approve"]; ok {
		t.Error("operator should NOT have infrastructure.requests:approve")
	}

	// Operator CANNOT manage users.
	if _, ok := permSet["users:create"]; ok {
		t.Error("operator should NOT have users:create")
	}

	// Operator must have campaign management.
	required := []Permission{
		"campaigns:create", "campaigns:read", "campaigns:execute", "campaigns:export",
		"targets:create", "targets:read",
		"schedules:create", "schedules:execute",
	}
	for _, p := range required {
		if _, ok := permSet[p]; !ok {
			t.Errorf("operator should have permission %q", p)
		}
	}
}

func TestBuiltinPermissions_Defender(t *testing.T) {
	perms, err := BuiltinPermissions(RoleDefender)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(perms) != 2 {
		t.Fatalf("defender should have exactly 2 permissions, got %d", len(perms))
	}
	permSet := make(map[Permission]struct{}, len(perms))
	for _, p := range perms {
		permSet[p] = struct{}{}
	}
	if _, ok := permSet["metrics:read"]; !ok {
		t.Error("defender should have metrics:read")
	}
	if _, ok := permSet["notifications:read"]; !ok {
		t.Error("defender should have notifications:read")
	}
}
