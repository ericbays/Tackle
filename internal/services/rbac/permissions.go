// Package rbac implements the role-based access control system for Tackle.
package rbac

import "sort"

// Resource constants — all resource names used in resource:action pairs.
const (
	ResourceUsers              = "users"
	ResourceRoles              = "roles"
	ResourceCampaigns          = "campaigns"
	ResourceTargets            = "targets"
	ResourceTemplatesEmail     = "templates.email"
	ResourceTemplatesLanding   = "templates.landing"
	ResourceDomains            = "domains"
	ResourceEndpoints          = "endpoints"
	ResourceSMTP               = "smtp"
	ResourceCredentials        = "credentials"
	ResourceReports            = "reports"
	ResourceMetrics            = "metrics"
	ResourceLogsAudit          = "logs.audit"
	ResourceLogsCampaign       = "logs.campaign"
	ResourceLogsSystem         = "logs.system"
	ResourceSettings           = "settings"
	ResourceSettingsAuth       = "settings.auth"
	ResourceCloud              = "cloud"
	ResourceInfrastructure     = "infrastructure"
	ResourceInfraRequests      = "infrastructure.requests"
	ResourceLandingPages       = "landing_pages"
	ResourceBlocklist          = "blocklist"
	ResourceSchedules          = "schedules"
	ResourceNotifications      = "notifications"
	ResourceAPIKeys            = "api_keys"
)

// Action constants.
const (
	ActionCreate  = "create"
	ActionRead    = "read"
	ActionUpdate  = "update"
	ActionDelete  = "delete"
	ActionExecute = "execute"
	ActionApprove = "approve"
	ActionExport  = "export"
	ActionReveal  = "reveal"
	ActionPurge   = "purge"
	ActionManage  = "manage"
)

// Permission represents a resource:action pair, e.g. "campaigns:read".
type Permission string

// Registry is the authoritative set of all valid permissions.
// Used to validate custom role permission assignments.
var Registry map[Permission]struct{}

func init() {
	// Full permission matrix from REQ-RBAC-012.
	pairs := []Permission{
		// users
		"users:create", "users:read", "users:update", "users:delete", "users:export",

		// roles
		"roles:create", "roles:read", "roles:update", "roles:delete",

		// campaigns
		"campaigns:create", "campaigns:read", "campaigns:update", "campaigns:delete",
		"campaigns:execute", "campaigns:export", "campaigns:approve",

		// targets
		"targets:create", "targets:read", "targets:update", "targets:delete", "targets:export",

		// templates.email
		"templates.email:create", "templates.email:read", "templates.email:update",
		"templates.email:delete", "templates.email:export",

		// templates.landing
		"templates.landing:create", "templates.landing:read", "templates.landing:update",
		"templates.landing:delete", "templates.landing:execute", "templates.landing:export",

		// domains
		"domains:create", "domains:read", "domains:update", "domains:delete",

		// endpoints
		"endpoints:create", "endpoints:read", "endpoints:update", "endpoints:delete",
		"endpoints:execute",

		// smtp
		"smtp:create", "smtp:read", "smtp:update", "smtp:delete", "smtp:execute",

		// credentials
		"credentials:read", "credentials:reveal", "credentials:delete",
		"credentials:export", "credentials:purge",

		// reports
		"reports:create", "reports:read", "reports:delete", "reports:export",

		// metrics
		"metrics:read", "metrics:export",

		// logs.audit
		"logs.audit:read", "logs.audit:export",

		// logs.campaign
		"logs.campaign:read", "logs.campaign:export",

		// logs.system
		"logs.system:read", "logs.system:export",

		// settings
		"settings:read", "settings:update",

		// settings.auth
		"settings.auth:read", "settings.auth:update",

		// cloud
		"cloud:create", "cloud:read", "cloud:update", "cloud:delete",

		// infrastructure (unified permission for cloud/SMTP/endpoints/instance templates)
		"infrastructure:create", "infrastructure:read", "infrastructure:update", "infrastructure:delete",

		// infrastructure.requests
		"infrastructure.requests:create", "infrastructure.requests:read",
		"infrastructure.requests:update", "infrastructure.requests:approve",

		// landing_pages
		"landing_pages:create", "landing_pages:read", "landing_pages:update", "landing_pages:delete",

		// blocklist
		"blocklist:manage",

		// schedules
		"schedules:create", "schedules:read", "schedules:update", "schedules:delete",
		"schedules:execute",

		// notifications
		"notifications:create", "notifications:read", "notifications:update",
		"notifications:delete",

		// api_keys
		"api_keys:create", "api_keys:read", "api_keys:delete",
	}

	Registry = make(map[Permission]struct{}, len(pairs))
	for _, p := range pairs {
		Registry[p] = struct{}{}
	}
}

// Valid reports whether p is a registered permission.
func Valid(p Permission) bool {
	_, ok := Registry[p]
	return ok
}

// All returns a sorted slice of every registered permission string.
func All() []Permission {
	out := make([]Permission, 0, len(Registry))
	for p := range Registry {
		out = append(out, p)
	}
	sort.Slice(out, func(i, j int) bool { return out[i] < out[j] })
	return out
}
