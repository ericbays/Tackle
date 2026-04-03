package rbac

import "fmt"

// BuiltinRoleName constants match the seed data in migration 002.
const (
	RoleAdmin    = "admin"
	RoleEngineer = "engineer"
	RoleOperator = "operator"
	RoleDefender = "defender"
)

var builtinNames = map[string]struct{}{
	RoleAdmin:    {},
	RoleEngineer: {},
	RoleOperator: {},
	RoleDefender: {},
}

// IsBuiltin reports whether the given role name is a built-in role.
func IsBuiltin(name string) bool {
	_, ok := builtinNames[name]
	return ok
}

// IsAdmin reports whether the given role name is the administrator role.
func IsAdmin(name string) bool {
	return name == RoleAdmin
}

// engineerPermissions are loaded from the permission matrix (REQ-RBAC-013).
var engineerPermissions = []Permission{
	"campaigns:read",
	"campaigns:approve",
	"targets:read",
	"templates.email:read",
	"templates.landing:create",
	"templates.landing:read",
	"templates.landing:update",
	"templates.landing:delete",
	"templates.landing:execute",
	"templates.landing:export",
	"domains:create",
	"domains:read",
	"domains:update",
	"domains:delete",
	"endpoints:create",
	"endpoints:read",
	"endpoints:update",
	"endpoints:delete",
	"endpoints:execute",
	"smtp:create",
	"smtp:read",
	"smtp:update",
	"smtp:delete",
	"smtp:execute",
	"credentials:read",
	"credentials:reveal",
	"credentials:delete",
	"credentials:export",
	"infrastructure:create",
	"infrastructure:read",
	"infrastructure:update",
	"infrastructure:delete",
	"landing_pages:create",
	"landing_pages:read",
	"landing_pages:update",
	"landing_pages:delete",
	"reports:read",
	"metrics:read",
	"metrics:export",
	"logs.audit:read",
	"logs.audit:export",
	"logs.campaign:read",
	"logs.campaign:export",
	"logs.system:read",
	"logs.system:export",
	"settings:read",
	"cloud:create",
	"cloud:read",
	"cloud:update",
	"cloud:delete",
	"infrastructure.requests:create",
	"infrastructure.requests:read",
	"infrastructure.requests:update",
	"infrastructure.requests:approve",
	"schedules:read",
	"notifications:read",
	"notifications:create",
	"notifications:update",
	"notifications:delete",
	"api_keys:create",
	"api_keys:read",
	"api_keys:delete",
}

// operatorPermissions are loaded from the permission matrix (REQ-RBAC-013).
var operatorPermissions = []Permission{
	"campaigns:create",
	"campaigns:read",
	"campaigns:update",
	"campaigns:delete",
	"campaigns:execute",
	"campaigns:export",
	"targets:create",
	"targets:read",
	"targets:update",
	"targets:delete",
	"targets:export",
	"templates.email:create",
	"templates.email:read",
	"templates.email:update",
	"templates.email:delete",
	"templates.email:export",
	"templates.landing:create",
	"templates.landing:read",
	"templates.landing:update",
	"templates.landing:delete",
	"templates.landing:execute",
	"templates.landing:export",
	"endpoints:read",
	"smtp:read",
	"smtp:execute",
	"credentials:read",
	"credentials:delete",
	"credentials:export",
	"infrastructure:read",
	"landing_pages:create",
	"landing_pages:read",
	"landing_pages:update",
	"landing_pages:delete",
	"reports:create",
	"reports:read",
	"reports:delete",
	"reports:export",
	"metrics:read",
	"metrics:export",
	"logs.campaign:read",
	"logs.campaign:export",
	"infrastructure.requests:create",
	"infrastructure.requests:read",
	"infrastructure.requests:update",
	"schedules:create",
	"schedules:read",
	"schedules:update",
	"schedules:delete",
	"schedules:execute",
	"notifications:read",
	"notifications:create",
	"notifications:update",
	"notifications:delete",
	"api_keys:create",
	"api_keys:read",
	"api_keys:delete",
}

// defenderPermissions are loaded from the permission matrix (REQ-RBAC-013).
var defenderPermissions = []Permission{
	"metrics:read",
	"notifications:read",
}

// BuiltinPermissions returns the permission set for a built-in role.
// Returns nil for RoleAdmin (short-circuit: all permissions implicitly granted).
// Returns an error if name is not a built-in role.
func BuiltinPermissions(name string) ([]Permission, error) {
	switch name {
	case RoleAdmin:
		return nil, nil
	case RoleEngineer:
		out := make([]Permission, len(engineerPermissions))
		copy(out, engineerPermissions)
		return out, nil
	case RoleOperator:
		out := make([]Permission, len(operatorPermissions))
		copy(out, operatorPermissions)
		return out, nil
	case RoleDefender:
		out := make([]Permission, len(defenderPermissions))
		copy(out, defenderPermissions)
		return out, nil
	default:
		return nil, fmt.Errorf("rbac: %q is not a built-in role", name)
	}
}
