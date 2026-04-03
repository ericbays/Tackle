# 02 — Authentication & Authorization

## 1. Overview

This document defines the authentication and authorization requirements for the Tackle platform. Authentication governs how users prove their identity, and authorization (via RBAC) governs what authenticated users are permitted to do. These systems are foundational — every other feature module depends on them.

Tackle supports multiple authentication providers operating simultaneously and implements a role-based access control (RBAC) system with four built-in roles and support for administrator-defined custom roles.

---

## 2. Initial Setup Flow

### REQ-AUTH-001 — First-Launch Setup Detection

On application startup, the backend MUST check whether any user accounts exist in the database. If the `users` table is empty (fresh installation), the system MUST enter a one-time setup mode.

- The setup mode MUST expose a dedicated endpoint (`POST /api/v1/setup`) that accepts the initial administrator's credentials.
- The setup endpoint MUST accept: username, email address, password, and password confirmation.
- Password MUST meet the platform's password policy (see REQ-AUTH-020).
- No other API endpoints (except health check) are accessible until setup is complete.
- The frontend MUST present a setup wizard guiding the administrator through initial account creation.

### REQ-AUTH-002 — Setup Endpoint Permanent Deactivation

Once the initial administrator account is created, the setup endpoint MUST be permanently disabled.

- The backend MUST reject all subsequent requests to `/api/v1/setup` with HTTP `403 Forbidden` and a response body indicating setup has already been completed.
- This check MUST be performed at the middleware level before any handler logic executes.
- The check MUST query a persistent flag (database or the existence of any user account) — it MUST NOT rely on in-memory state alone, so that the protection survives application restarts.
- There is no mechanism to re-enable the setup endpoint. Re-initialization requires direct database intervention outside the application.

### REQ-AUTH-003 — Initial Administrator Immutability

The first user account created via the setup flow is designated as the **initial administrator**. This designation is permanent and irrevocable.

- The initial administrator MUST be flagged in the database with an `is_initial_admin` boolean column set to `true`. No other account may have this flag set to `true`.
- No API endpoint — including those accessible to other administrators — may remove the Administrator role from this account.
- No API endpoint may delete, deactivate, or suspend the initial administrator account.
- The initial administrator account itself MUST NOT be able to remove its own Administrator role or delete itself.
- The backend MUST enforce these constraints at the service layer, independent of any UI restrictions.
- Audit log entries MUST be generated for any attempt (successful or rejected) to modify the initial administrator's role or account status.

---

## 3. Authentication Providers

Tackle supports four authentication providers. All four can be active simultaneously. Users authenticate through any configured provider, and the system resolves their identity to a single internal user account.

### 3.1 Local Accounts

#### REQ-AUTH-010 — Local Account Availability

Local account authentication MUST always be available, regardless of whether any external authentication providers are configured or active.

- Local authentication cannot be disabled through any configuration or UI setting.
- This guarantees that the initial administrator (and any locally-created accounts) can always log in, even if external providers experience outages or misconfigurations.

#### REQ-AUTH-011 — Local Account Creation

Administrators MUST be able to create local accounts via the API and the admin UI.

- Required fields: username (unique), email address (unique), display name, password, role assignment.
- Passwords MUST be hashed using bcrypt with a minimum cost factor of 12.
- Plaintext passwords MUST never be stored, logged, or included in API responses.
- The API MUST return the created user object (without password hash) upon successful creation.

#### REQ-AUTH-012 — Local Account Login

The local login endpoint (`POST /api/v1/auth/login`) MUST accept username (or email) and password.

- The backend MUST perform constant-time comparison of password hashes to prevent timing attacks.
- On successful authentication, the backend MUST issue a JWT access token and a refresh token (see Section 4).
- On failure, the backend MUST return a generic `401 Unauthorized` response. The error message MUST NOT indicate whether the username or the password was incorrect.
- Rate limiting MUST be enforced on the login endpoint (see REQ-AUTH-025).

#### REQ-AUTH-013 — Local Account Password Change

Authenticated users MUST be able to change their own local password.

- The endpoint MUST require the current password before accepting the new password.
- The new password MUST meet the password policy (see REQ-AUTH-020).
- On password change, all existing sessions for that user (except the current session) MUST be invalidated.

#### REQ-AUTH-014 — Administrator Password Reset

Administrators MUST be able to reset any local user's password without knowing the current password.

- The administrator sets a temporary password.
- The account MUST be flagged so the user is required to change their password on next login.
- An audit log entry MUST be recorded for the reset, including which administrator performed it.

### 3.2 OIDC (OpenID Connect)

#### REQ-AUTH-030 — OIDC Provider Configuration

Administrators MUST be able to configure one or more generic OIDC providers via the admin UI and API.

- Configuration fields per provider: provider name (display label), issuer URL, client ID, client secret, scopes (default: `openid profile email`), redirect URI, user info endpoint (optional override), token endpoint (optional override), authorization endpoint (optional override).
- Client secrets MUST be encrypted at rest using the application's secret encryption key.
- Administrators MUST be able to enable, disable, or delete individual OIDC provider configurations.
- Changes to OIDC configuration MUST take effect without application restart.

#### REQ-AUTH-031 — OIDC Authentication Flow

The system MUST implement the OIDC Authorization Code Flow.

- The frontend MUST present configured OIDC providers as login options alongside the local login form.
- Clicking an OIDC provider button MUST initiate the authorization code flow by redirecting the user to the provider's authorization endpoint.
- The backend callback endpoint (`GET /api/v1/auth/oidc/callback/{provider_id}`) MUST exchange the authorization code for tokens, validate the ID token (signature, issuer, audience, expiration), extract user claims (subject, email, name), and issue Tackle JWT tokens.
- The `state` parameter MUST be used and validated to prevent CSRF attacks.
- A `nonce` claim MUST be included and validated in the ID token.

#### REQ-AUTH-032 — OIDC User Provisioning

On first login via OIDC, the system MUST handle user provisioning.

- If no Tackle account is linked to the OIDC subject identifier for that provider, the system MUST check whether an existing account matches by email address.
- If a matching account exists, the OIDC identity MUST be linked to that existing account (after confirmation — see REQ-AUTH-060).
- If no matching account exists, a new Tackle user account MUST be automatically provisioned with a default role (configurable per OIDC provider, default: Operator).
- Auto-provisioned accounts MUST have no local password set (they authenticate exclusively via OIDC unless they later set a local password).

### 3.3 FusionAuth

#### REQ-AUTH-040 — FusionAuth Integration Configuration

Administrators MUST be able to configure a FusionAuth integration via the admin UI and API.

- Configuration fields: FusionAuth base URL, application ID, client ID, client secret, tenant ID (optional), API key (for user management operations).
- All sensitive fields (client secret, API key) MUST be encrypted at rest.
- The system MUST validate the configuration on save by performing a connectivity check against the FusionAuth instance.

#### REQ-AUTH-041 — FusionAuth Authentication Flow

The FusionAuth authentication flow MUST use FusionAuth's OAuth2/OIDC endpoints.

- The login flow follows the same Authorization Code Flow pattern as generic OIDC.
- The backend MUST additionally leverage FusionAuth-specific claims and user data (e.g., group memberships, registration data) if available.
- FusionAuth's refresh token endpoint MUST be used for token renewal when FusionAuth is the authentication source.

#### REQ-AUTH-042 — FusionAuth User Provisioning

FusionAuth user provisioning follows the same rules as OIDC (REQ-AUTH-032) with the following additions.

- If FusionAuth provides role or group information, administrators MUST be able to configure a mapping from FusionAuth roles/groups to Tackle roles.
- The role mapping is optional — if not configured, the default role is applied.
- Role mappings MUST be re-evaluated on each login (not just first login) so that changes in FusionAuth groups are reflected in Tackle.

### 3.4 LDAP

#### REQ-AUTH-050 — LDAP Configuration

Administrators MUST be able to configure LDAP authentication via the admin UI and API.

- Configuration fields: server URL(s) (supports `ldap://` and `ldaps://`), bind DN, bind password, base DN for user search, user search filter (default: `(sAMAccountName={{username}})` for AD, `(uid={{username}})` for standard LDAP), attribute mappings (username, email, display name, group membership), group base DN (optional), group search filter (optional), StartTLS toggle, certificate verification toggle (for lab environments with self-signed certificates).
- Bind password MUST be encrypted at rest.
- The system MUST support connection pooling for LDAP queries.
- The system MUST validate the configuration on save by attempting a test bind.

#### REQ-AUTH-051 — LDAP Authentication Flow

The LDAP authentication flow uses bind authentication.

- The user provides a username and password via the standard login form (same form as local login, with a selector or auto-detection).
- The backend MUST first search for the user using the configured search filter and the service account bind credentials.
- If the user DN is found, the backend MUST attempt to bind as that user with the provided password.
- On successful bind, the backend MUST extract the configured attributes (email, display name, group memberships) and issue Tackle JWT tokens.
- Failed binds MUST return the same generic `401 Unauthorized` response as local login failures.

#### REQ-AUTH-052 — LDAP User Provisioning

LDAP user provisioning follows the same rules as OIDC (REQ-AUTH-032) with the following additions.

- If LDAP group memberships are available and a group-to-role mapping is configured, the user's Tackle role MUST be set according to the mapping.
- Group-to-role mappings MUST be re-evaluated on each login.
- Administrators MUST be able to configure whether LDAP authentication is attempted before, after, or instead of local authentication for users who provide username/password credentials. The default order is: local first, then LDAP.

### 3.5 Multi-Provider Account Linking

#### REQ-AUTH-060 — Account Linking

Users MUST be able to link multiple authentication methods to a single Tackle account.

- A user account maintains a list of linked identities (stored in an `auth_identities` table), each referencing the provider type, provider ID, and external subject identifier.
- Linking an OIDC/FusionAuth/LDAP identity to an existing account MUST require the user to first authenticate with the existing account (proving ownership).
- The account linking flow is: (1) user is logged in, (2) user initiates "Link Account" from settings, (3) user authenticates with the external provider, (4) the external identity is linked to the current account.
- When an unlinked external identity's email matches an existing account, the system MUST prompt the user to either link to the existing account (requires re-authentication) or create a new account.

#### REQ-AUTH-061 — Identity Unlinking

Users MUST be able to unlink external authentication identities from their account, with the following constraints.

- A user MUST NOT be able to unlink their last remaining authentication method. At least one method (local password or external identity) must remain active.
- Unlinking an identity MUST NOT delete any data or permissions associated with the user account.
- An audit log entry MUST be recorded for link and unlink actions.

---

## 4. Session Management

### REQ-AUTH-070 — JWT Access Tokens

The system MUST use JSON Web Tokens (JWT) for API authentication.

- Access tokens MUST be signed using HMAC-SHA256 (HS256) with a server-side secret, or RS256 with an RSA key pair. The signing method MUST be configurable.
- Access token payload MUST include: `sub` (user ID), `username`, `email`, `role` (current role name), `permissions` (list of granted permissions), `iat` (issued at), `exp` (expiration), `jti` (unique token identifier).
- Default access token lifetime: 15 minutes. Configurable by administrators between 5 minutes and 60 minutes.
- Access tokens MUST be sent in the `Authorization: Bearer <token>` header.
- The backend MUST validate the token signature, expiration, and issuer on every request.

### REQ-AUTH-071 — Refresh Tokens

The system MUST implement refresh tokens for session continuity.

- Refresh tokens MUST be opaque (non-JWT), randomly generated, and stored in the database (hashed).
- Default refresh token lifetime: 7 days. Configurable by administrators between 1 hour and 30 days.
- Refresh tokens are single-use — issuing a new access token MUST also issue a new refresh token and invalidate the old one (rotation).
- If a previously-used (invalidated) refresh token is presented, the system MUST treat this as a potential token theft, invalidate all refresh tokens for that user, and log a security event.
- Refresh tokens MUST be returned in an HTTP-only, Secure, SameSite=Strict cookie for browser clients, or in the response body for API clients.

### REQ-AUTH-072 — Session Termination

Users and administrators MUST be able to terminate sessions.

- Users MUST be able to log out (invalidates current access and refresh tokens).
- Users MUST be able to view and terminate their other active sessions (list of refresh tokens with metadata: IP address, user agent, creation time, last used time).
- Administrators MUST be able to terminate any user's active sessions.
- On logout, the refresh token MUST be deleted from the database and the access token's `jti` MUST be added to a short-lived blacklist (in-memory, expires when the access token would naturally expire).

### REQ-AUTH-073 — Configurable Session Duration

Administrators MUST be able to configure session parameters via the admin UI.

- Configurable parameters: access token lifetime, refresh token lifetime, maximum concurrent sessions per user (default: unlimited), idle session timeout (optional — revoke refresh token if not used within N minutes).
- Changes to session configuration MUST apply to new sessions only. Existing sessions continue with their original parameters until they naturally expire or are explicitly revoked.

---

## 5. Password Policy

### REQ-AUTH-020 — Password Complexity Requirements

The system MUST enforce a configurable password policy for local accounts.

- Default minimum requirements: minimum length of 12 characters, at least one uppercase letter, at least one lowercase letter, at least one digit, at least one special character.
- Administrators MUST be able to adjust these requirements via the admin UI.
- The system MUST reject passwords that appear in a list of commonly breached passwords (embedded list, minimum 10,000 entries).
- Password policy MUST be enforced on account creation, password change, and administrator-initiated password reset.

### REQ-AUTH-021 — Password History

The system MUST maintain a password history for local accounts.

- The system MUST store the last N password hashes (configurable, default: 5).
- Users MUST NOT be able to reuse any password in their history.

---

## 6. Security Controls

### REQ-AUTH-025 — Login Rate Limiting

The system MUST enforce rate limiting on authentication endpoints.

- Per-IP rate limit: maximum 10 failed login attempts per minute. After exceeding the limit, subsequent attempts from that IP MUST be rejected with HTTP `429 Too Many Requests` for a configurable lockout duration (default: 5 minutes).
- Per-account rate limit: maximum 5 failed login attempts per minute for a given username. After exceeding the limit, the account MUST be temporarily locked for a configurable duration (default: 15 minutes).
- Rate limiting MUST apply to local and LDAP login. OIDC/FusionAuth flows are rate-limited by the external provider.
- Successful login MUST reset the per-account failure counter.
- Account lockout events MUST be logged with full context (IP, username, timestamp, failure count).

### REQ-AUTH-026 — Account Lockout

Administrators MUST be able to manually lock and unlock user accounts.

- A locked account MUST NOT be able to authenticate via any provider.
- A locked account's existing sessions MUST be immediately invalidated.
- The initial administrator account MUST NOT be lockable (to prevent complete lockout of the system).
- Lockout and unlock actions MUST be recorded in the audit log.

### REQ-AUTH-027 — Secure Token Storage

All secrets related to authentication MUST follow secure storage practices.

- JWT signing keys MUST be stored as environment variables or in the database encrypted with the application encryption key. They MUST NOT be hardcoded.
- OIDC/FusionAuth client secrets and LDAP bind passwords MUST be encrypted at rest in the database.
- Refresh token values stored in the database MUST be hashed (SHA-256) — the raw token value is never persisted.
- Captured credentials from phishing campaigns are handled separately (see 08-credential-capture.md) and MUST NOT be stored in or near the authentication tables.

### REQ-AUTH-028 — Audit Logging for Authentication Events

All authentication-related events MUST be recorded in the audit log.

- Events to log: successful login (with provider type), failed login (with provider type and reason), logout, session refresh, password change, password reset, account creation, account deletion, account lock/unlock, role change, permission change, auth provider configuration change, account link/unlink, rate limit triggered, setup flow completion.
- Each log entry MUST include: timestamp, user ID (if applicable), IP address, user agent, event type, event outcome (success/failure), additional context.

---

## 7. Role-Based Access Control (RBAC)

### 7.1 Built-in Roles

The system ships with four immutable built-in roles. These roles cannot be renamed, deleted, or have their permission sets modified.

#### REQ-RBAC-001 — Administrator Role

The **Administrator** role grants unrestricted access to all platform features.

- Administrators have implicit permission to perform any action on any resource.
- Permission checks for Administrators MUST short-circuit to "allowed" — they are never denied access by the RBAC system.
- The only restriction on an Administrator is REQ-AUTH-003: the initial administrator's role assignment is immutable.
- Administrators can create, modify, and delete other user accounts (including other Administrators, except the initial admin).
- Administrators can configure all system settings, authentication providers, and create custom roles.

#### REQ-RBAC-002 — Engineer Role

The **Engineer** role grants full access to infrastructure and phishing endpoint management.

- Engineers have full CRUD access to: phishing endpoints (create, configure, deploy, monitor, stop, terminate), domain management (register, configure DNS, manage certificates), SMTP server configurations, landing page templates and builds, cloud provider credential configuration.
- Engineers can approve or reject infrastructure requests submitted by Operators.
- Engineers have read access to: campaigns, targets, captured credentials, metrics, reports, audit logs.
- Engineers CANNOT: manage user accounts, configure authentication providers, modify system-wide settings, manage roles and permissions.

#### REQ-RBAC-003 — Operator Role

The **Operator** role is designed for red team members who plan and execute campaigns.

- Operators have full CRUD access to: campaigns (create, configure, schedule, execute, pause, stop), target lists and target management, email templates, landing page design (via landing page builder), credential capture configuration (per campaign), reports (generate and export).
- Operators can request infrastructure changes, but these requests require Engineer approval: provisioning new phishing endpoints, modifying domain DNS records, changing SMTP routing.
- Operators have read access to: phishing endpoint status, domain status, SMTP server status, metrics dashboards, audit logs (own actions only).
- Operators CANNOT: directly manage infrastructure, manage user accounts, configure authentication providers, approve their own infrastructure requests.

#### REQ-RBAC-004 — Defender Role

The **Defender** role provides read-only access for blue team members reviewing campaign results.

- Defenders have read access to: metrics dashboards, campaign results and statistics, aggregate reports.
- Defenders CANNOT: view raw captured credentials, view individual target PII, view detailed email templates, view landing page source, modify any resource, access audit logs, access system configuration.
- The Defender dashboard MUST present an appropriately scoped view — it MUST NOT expose navigation or UI elements for features the Defender cannot access.

### 7.2 Permissions System

#### REQ-RBAC-010 — Permission Model

The RBAC system MUST implement a granular permission model based on resource types and actions.

- Each permission is expressed as `resource:action` (e.g., `campaigns:create`, `targets:read`).
- Permissions are assigned to roles. Users inherit permissions from their assigned role.
- The system MUST evaluate permissions on every API request via middleware.
- If a user lacks the required permission for an endpoint, the API MUST return HTTP `403 Forbidden`.

#### REQ-RBAC-011 — Resource Types

The following resource types MUST be defined in the permission system:

| Resource Type | Description |
|---------------|-------------|
| `users` | User account management |
| `roles` | Role and permission management |
| `campaigns` | Campaign lifecycle management |
| `targets` | Target individuals and target lists |
| `templates.email` | Email templates |
| `templates.landing` | Landing page templates and builds |
| `domains` | Domain registration and DNS management |
| `endpoints` | Phishing endpoint infrastructure |
| `smtp` | SMTP server configuration |
| `credentials` | Captured credentials from campaigns |
| `reports` | Report generation and export |
| `metrics` | Dashboard metrics and statistics |
| `logs.audit` | Audit log access |
| `logs.campaign` | Campaign activity logs |
| `logs.system` | System event logs |
| `settings` | System-wide configuration |
| `settings.auth` | Authentication provider configuration |
| `cloud` | Cloud provider credential and integration management |
| `infrastructure.requests` | Infrastructure change request workflow |
| `schedules` | Campaign scheduling |
| `notifications` | System notification configuration |
| `api_keys` | API key management for programmatic access |

#### REQ-RBAC-012 — Action Types

The following action types MUST be defined for each resource type:

| Action | Description |
|--------|-------------|
| `create` | Create a new instance of the resource |
| `read` | View/retrieve resource data |
| `update` | Modify an existing resource |
| `delete` | Remove/destroy a resource |
| `execute` | Trigger an operational action (e.g., launch a campaign, send test email, deploy endpoint) |
| `approve` | Approve a pending request or workflow item (e.g., infrastructure requests) |
| `export` | Export/download resource data |

Not every resource type requires every action. The following matrix defines the applicable permissions:

| Resource | create | read | update | delete | execute | approve | export |
|----------|--------|------|--------|--------|---------|---------|--------|
| `users` | X | X | X | X | | | X |
| `roles` | X | X | X | X | | | |
| `campaigns` | X | X | X | X | X | | X |
| `targets` | X | X | X | X | | | X |
| `templates.email` | X | X | X | X | | | X |
| `templates.landing` | X | X | X | X | X | | X |
| `domains` | X | X | X | X | | | |
| `endpoints` | X | X | X | X | X | | |
| `smtp` | X | X | X | X | X | | |
| `credentials` | | X | | X | | | X |
| `reports` | X | X | | X | | | X |
| `metrics` | | X | | | | | X |
| `logs.audit` | | X | | | | | X |
| `logs.campaign` | | X | | | | | X |
| `logs.system` | | X | | | | | X |
| `settings` | | X | X | | | | |
| `settings.auth` | | X | X | | | | |
| `cloud` | X | X | X | X | | | |
| `infrastructure.requests` | X | X | X | | | X | |
| `schedules` | X | X | X | X | X | | |
| `notifications` | X | X | X | X | | | |
| `api_keys` | X | X | | X | | | |

#### REQ-RBAC-013 — Built-in Role Permission Assignments

The following table defines the default permission assignments for each built-in role. Administrators are omitted as they have all permissions implicitly.

| Permission | Engineer | Operator | Defender |
|------------|----------|----------|----------|
| `users:*` | | | |
| `roles:*` | | | |
| `campaigns:read` | X | X | |
| `campaigns:create` | | X | |
| `campaigns:update` | | X | |
| `campaigns:delete` | | X | |
| `campaigns:execute` | | X | |
| `campaigns:export` | | X | |
| `targets:read` | X | X | |
| `targets:create` | | X | |
| `targets:update` | | X | |
| `targets:delete` | | X | |
| `targets:export` | | X | |
| `templates.email:read` | X | X | |
| `templates.email:create` | | X | |
| `templates.email:update` | | X | |
| `templates.email:delete` | | X | |
| `templates.email:export` | | X | |
| `templates.landing:*` | X | X | |
| `domains:*` | X | | |
| `endpoints:*` | X | | |
| `endpoints:read` | X | X | |
| `smtp:*` | X | | |
| `smtp:read` | X | X | |
| `smtp:execute` | X | X | |
| `credentials:read` | X | X | |
| `credentials:delete` | X | X | |
| `credentials:export` | X | X | |
| `reports:*` | | X | |
| `reports:read` | X | X | |
| `metrics:read` | X | X | X |
| `metrics:export` | X | X | |
| `logs.audit:read` | X | | |
| `logs.audit:export` | X | | |
| `logs.campaign:read` | X | X | |
| `logs.campaign:export` | X | X | |
| `logs.system:read` | X | | |
| `logs.system:export` | X | | |
| `settings:read` | X | | |
| `settings.auth:*` | | | |
| `cloud:*` | X | | |
| `infrastructure.requests:create` | X | X | |
| `infrastructure.requests:read` | X | X | |
| `infrastructure.requests:update` | X | X | |
| `infrastructure.requests:approve` | X | | |
| `schedules:*` | | X | |
| `schedules:read` | X | X | |
| `notifications:read` | X | X | X |
| `notifications:create` | X | X | |
| `notifications:update` | X | X | |
| `notifications:delete` | X | X | |
| `api_keys:create` | X | X | |
| `api_keys:read` | X | X | |
| `api_keys:delete` | X | X | |

**Note on Operator audit log access:** Operators can view their own audit log entries via a filtered endpoint. They do NOT have the `logs.audit:read` permission, which grants access to all audit logs. Own-action audit visibility is enforced at the API layer separately from RBAC permissions.

### 7.3 Custom Roles

#### REQ-RBAC-020 — Custom Role Creation

Administrators MUST be able to create custom roles via the admin UI and API.

- A custom role has a name (unique, alphanumeric with hyphens/underscores, max 64 characters), a description (max 255 characters), and a set of permissions selected from the full permission matrix (REQ-RBAC-012).
- Custom roles MUST NOT be named the same as any built-in role (case-insensitive comparison).
- There is no limit on the number of custom roles that can be created.
- Custom roles can combine any subset of permissions from the permission matrix. This allows creating roles that span traditional boundaries (e.g., a "Campaign Reviewer" role with read access to campaigns, targets, and credentials but no write access).

#### REQ-RBAC-021 — Custom Role Modification

Administrators MUST be able to modify the permissions assigned to a custom role.

- When a custom role's permissions are modified, the change MUST take effect on the next API request for users assigned to that role (permissions are evaluated per-request, not cached in the JWT beyond the access token lifetime).
- However, since the `permissions` claim is included in the JWT (REQ-AUTH-070), the effective permission update occurs when the user's current access token expires and a new one is issued with the updated permissions. For immediate enforcement, administrators can force-revoke active sessions.
- An audit log entry MUST record every permission change, including the before and after permission sets.

#### REQ-RBAC-022 — Custom Role Deletion

Administrators MUST be able to delete custom roles that are no longer needed.

- A custom role MUST NOT be deleted if any users are currently assigned to it. The API MUST return an error listing the affected users.
- Administrators must reassign affected users to a different role before deletion.
- Built-in roles MUST NOT be deletable.

#### REQ-RBAC-023 — Role Assignment

Administrators MUST be able to assign roles to user accounts.

- Each user has exactly one role at any time (no multi-role assignment).
- Changing a user's role MUST be reflected in the next issued JWT.
- Administrators can change any user's role except: the initial administrator (REQ-AUTH-003) and their own role (an administrator cannot remove their own Administrator role; another administrator must do it).
- Role changes MUST be logged in the audit log.

---

## 8. Permission Enforcement

### REQ-RBAC-030 — Middleware Enforcement

Every API endpoint (except public endpoints: login, OIDC callback, health check, setup) MUST be protected by authentication and authorization middleware.

- The middleware MUST: validate the JWT access token, extract the user's role and permissions, check that the user has the required permission for the requested endpoint and action, return `401 Unauthorized` if the token is invalid or expired, return `403 Forbidden` if the user lacks the required permission.
- Each API route MUST declare its required permission(s) as part of its route definition.
- Endpoints that require multiple permissions (e.g., approving an infrastructure request requires both `infrastructure.requests:read` and `infrastructure.requests:approve`) MUST require ALL listed permissions.

### REQ-RBAC-031 — Frontend Permission Enforcement

The frontend MUST enforce permissions for UI rendering purposes.

- On login, the frontend receives the user's permissions list (from the JWT or a dedicated `/api/v1/auth/me` endpoint).
- Navigation items, buttons, and UI elements MUST be conditionally rendered based on the user's permissions.
- Frontend enforcement is for UX only — the backend MUST always be the authoritative enforcement point. A missing backend check is a security vulnerability regardless of frontend behavior.

### REQ-RBAC-032 — Resource-Level Access Control

In addition to action-level permissions, certain resources MUST enforce ownership or scope-based access.

- Operators MUST only be able to view and manage campaigns they created, unless granted explicit access by another Operator or an Administrator.
- Campaign sharing MUST be supported: an Operator can grant another Operator read or read-write access to their campaign.
- Audit log access for Operators is scoped to their own actions (see note under REQ-RBAC-013).
- API keys are scoped to the user who created them — a user can only view and manage their own API keys.

---

## 9. API Key Authentication

### REQ-AUTH-080 — API Key Support

The system MUST support API key authentication for programmatic/headless access.

- Users can generate API keys from the admin UI or via the API.
- Each API key is associated with the creating user and inherits that user's role and permissions.
- API key format: a 64-character cryptographically random string, prefixed with `tk_` (e.g., `tk_a1b2c3...`).
- The API key MUST only be displayed once upon creation. The system stores only a SHA-256 hash of the key.
- API keys can be given a name/label and an optional expiration date.
- API key authentication uses the `X-API-Key` header.
- API key requests are subject to the same RBAC enforcement as JWT-authenticated requests.
- Administrators can view and revoke any user's API keys. Users can view and revoke their own.

---

## 10. Acceptance Criteria

### Authentication

- [ ] On a fresh database, the application presents the setup wizard and no other functionality is accessible.
- [ ] After the initial administrator is created, the setup endpoint returns `403` for all subsequent requests, including after application restart.
- [ ] The initial administrator's account cannot be deleted, deactivated, or have its Administrator role removed — by any user, including itself.
- [ ] Local authentication works with username or email, uses bcrypt hashing, and returns generic error messages on failure.
- [ ] At least one OIDC provider can be configured and used for login simultaneously with local auth.
- [ ] FusionAuth integration supports login, auto-provisioning, and group-to-role mapping.
- [ ] LDAP integration supports bind authentication, auto-provisioning, and group-to-role mapping.
- [ ] Users can link multiple auth providers to one account and unlink them (while retaining at least one).
- [ ] JWT access tokens expire after the configured duration and are renewable via refresh tokens.
- [ ] Refresh token rotation is enforced — reuse of an old refresh token invalidates all tokens for that user.
- [ ] Users can view and revoke their own active sessions.
- [ ] Administrators can revoke any user's sessions.
- [ ] Password policy is enforced on all password creation and change operations.
- [ ] Rate limiting blocks excessive login attempts by IP and by account.
- [ ] All authentication events are recorded in the audit log.

### Authorization

- [ ] Administrators have unrestricted access to all endpoints.
- [ ] Engineers can manage infrastructure, endpoints, domains, and SMTP but cannot manage users or system settings.
- [ ] Engineers can approve Operator infrastructure requests.
- [ ] Operators can fully manage campaigns, targets, and templates but cannot directly manage infrastructure.
- [ ] Operator infrastructure changes require Engineer approval; Operators cannot approve their own requests.
- [ ] Defenders see only the metrics dashboard with aggregate data — no access to raw credentials, target PII, templates, or configuration.
- [ ] Custom roles can be created with any combination of permissions from the permission matrix.
- [ ] Modifying a custom role's permissions takes effect for assigned users on their next token refresh.
- [ ] Deleting a custom role is blocked if users are assigned to it.
- [ ] API keys inherit the creating user's permissions and are subject to RBAC enforcement.
- [ ] Every API endpoint (except public endpoints) returns `403` when accessed without the required permission.
- [ ] Frontend hides UI elements the user lacks permission to use, but backend enforcement is the authority.

---

## 11. Security Considerations

1. **Credential storage** — Passwords are hashed with bcrypt (cost 12+). Refresh tokens are hashed with SHA-256. API keys are hashed with SHA-256. No reversible credential storage.
2. **Token security** — JWTs are signed with HS256 or RS256. Refresh tokens are HTTP-only cookies with Secure and SameSite=Strict flags. Access tokens have short lifetimes.
3. **Timing attacks** — Password comparison uses constant-time functions. Username enumeration is prevented by generic error messages.
4. **CSRF protection** — OIDC state parameter validation. SameSite cookie attributes. All state-changing operations require authentication.
5. **Rate limiting** — Per-IP and per-account rate limiting on login endpoints prevents brute-force attacks.
6. **Privilege escalation** — Users cannot modify their own role. Custom roles cannot exceed Administrator permissions (inherently, since Administrator bypasses checks). Permission changes are audited.
7. **Session fixation** — New tokens are issued on every login. Refresh token rotation prevents replay.
8. **Initial admin protection** — The initial administrator account is immune to deletion, deactivation, lockout, and role removal, ensuring the system always has a recovery path.
9. **Separation of auth data** — Authentication tables (users, sessions, auth_identities) are logically separated from campaign data (captured credentials). No query or API endpoint can conflate the two.
10. **External provider failures** — If an external auth provider (OIDC, FusionAuth, LDAP) is unavailable, local auth remains functional. The system degrades gracefully.

---

## 12. Dependencies

| Dependency | Document | Nature |
|------------|----------|--------|
| PostgreSQL database | [14-database-schema.md](14-database-schema.md) | Auth tables (users, roles, sessions, auth_identities, api_keys) defined in DB schema |
| Audit logging system | [11-audit-logging.md](11-audit-logging.md) | All auth events feed into the audit log |
| Frontend admin UI | [16-frontend-architecture.md](16-frontend-architecture.md) | Login pages, setup wizard, user management UI, role management UI |
| System configuration | [01-system-overview.md](01-system-overview.md) | Bootstrap config (encryption key, DB connection) via environment variables |
| Phishing endpoint proxy | [07-phishing-endpoints.md](07-phishing-endpoints.md) | Endpoints do NOT use Tackle auth — they serve unauthenticated target-facing traffic |
| Campaign management | [06-campaign-management.md](06-campaign-management.md) | Campaign ownership and sharing model depends on RBAC user identity |
| Credential capture | [08-credential-capture.md](08-credential-capture.md) | Captured credentials are NOT stored in auth tables; access governed by RBAC |

---

## 13. Data Model Summary

The following entities are introduced or referenced by this module. Full schema definitions are in [14-database-schema.md](14-database-schema.md).

| Entity | Key Fields | Notes |
|--------|------------|-------|
| `users` | id, username, email, display_name, password_hash, role_id, is_initial_admin, is_locked, force_password_change, created_at, updated_at | Core user account |
| `auth_identities` | id, user_id, provider_type, provider_id, external_subject, created_at | Links external auth providers to user accounts |
| `roles` | id, name, description, is_builtin, created_at, updated_at | Built-in and custom roles |
| `role_permissions` | role_id, permission | Permission assignments for custom roles (built-in role perms are hardcoded) |
| `refresh_tokens` | id, user_id, token_hash, ip_address, user_agent, created_at, expires_at, last_used_at, revoked | Active refresh tokens / sessions |
| `password_history` | id, user_id, password_hash, created_at | Previous password hashes for reuse prevention |
| `api_keys` | id, user_id, name, key_hash, expires_at, created_at, last_used_at, revoked | API keys for programmatic access |
| `auth_provider_configs` | id, provider_type, name, config_json (encrypted), is_enabled, default_role_id, created_at, updated_at | OIDC/FusionAuth/LDAP provider configurations |
| `role_mappings` | id, auth_provider_config_id, external_group, role_id | Maps external groups to Tackle roles |
