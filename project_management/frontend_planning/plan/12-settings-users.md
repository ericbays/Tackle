# 12 — Settings, User Management, and User Preferences

This document specifies the **Settings** page (Admin-only), **User Management** page, **Role Management**, **API Key Management**, **Webhook Management**, and the **User Preferences** panel. Settings govern platform-wide configuration. User management covers CRUD operations on users, roles, and sessions. User preferences are per-user personalization options that apply immediately and persist server-side.

---

## 1. Settings Page

### 1.1 Purpose

The Settings page is the central administration surface for platform-wide configuration. Only users with the `admin` role can access it. Each settings section saves independently — there is no global "Save all" button. Changes take effect immediately upon successful save.

### 1.2 Navigation

- Sidebar location: **Administration > Settings**.
- Requires permission: `settings:read` (view), `settings:write` (modify).
- Route: `/settings` redirects to `/settings/general`.
- Page title: "Settings".

### 1.3 Layout

The Settings page uses a vertical tab layout with the tab list on the left and the active section's content on the right.

```
┌──────────────────────────────────────────────────────────────────────────┐
│  Settings                                                                │
├──────────────┬───────────────────────────────────────────────────────────┤
│              │                                                           │
│  General     │  (Active tab content)                                     │
│  Security    │                                                           │
│  Auth        │  Section heading                                          │
│  Providers   │  Description text                                         │
│  Sessions    │                                                           │
│  Password    │  ┌─────────────────────────────────────────────────┐      │
│  Policy      │  │  Field Label                                    │      │
│  Email/SMTP  │  │  [input value                              ]    │      │
│  Notifications│ │  Help text                                      │      │
│  Webhooks    │  │                                                  │      │
│  API Keys    │  │  Field Label                                    │      │
│              │  │  [input value                              ]    │      │
│              │  │                                                  │      │
│              │  │                    [Cancel]  [Save Section]      │      │
│              │  └─────────────────────────────────────────────────┘      │
│              │                                                           │
├──────────────┴───────────────────────────────────────────────────────────┤
```

### 1.4 Tab Navigation

| Tab | Route | Section |
|-----|-------|---------|
| General | `/settings/general` | App name, timezone, retention, branding |
| Security | `/settings/security` | Rate limits, account lockout |
| Auth Providers | `/settings/auth-providers` | OIDC, FusionAuth, LDAP configuration |
| Sessions | `/settings/sessions` | JWT lifetimes, max concurrent sessions |
| Password Policy | `/settings/password-policy` | Complexity rules, expiry, history |
| Email / SMTP | `/settings/email` | Notification SMTP configuration |
| Notifications | `/settings/notifications` | System notification defaults, retention |
| Webhooks | `/settings/webhooks` | Outbound webhook management |
| API Keys | `/settings/api-keys` | API key creation and revocation |

- The active tab has a left border in `--accent-primary` and `--accent-primary-muted` background.
- Tab list width: 200px, fixed. Content area fills remaining width.
- Tab switching updates the URL. Direct-linking to a tab route works.
- On viewports below 768px, the tab list collapses into a `<select>` dropdown above the content area.

### 1.5 Section Save Pattern

Every settings section follows this save pattern:

1. Fields load current values from `GET /api/v1/settings` (the response is a flat key-value map organized by section).
2. Editing any field enables the "Save Section" button and shows "Cancel" alongside it.
3. "Cancel" reverts all fields in the section to their last-saved values.
4. "Save Section" calls `PUT /api/v1/settings` with only the changed keys.
5. On success: a success toast ("Settings updated"), button returns to disabled state.
6. On validation error: inline field errors appear below the offending field(s), the toast reads "Please fix the errors below."
7. On server error: danger toast with the error message.
8. If the user attempts to navigate away with unsaved changes, a confirmation modal appears: "You have unsaved changes in [Section Name]. Discard changes?"

### 1.6 Sensitive Field Handling

Fields containing secrets (API keys, SMTP passwords, HMAC secrets) follow a masked display pattern:

- The field displays a masked value: `••••••••••••` (12 bullet characters regardless of actual length).
- A toggle button (eye icon) on the right side of the input reveals the actual value. The toggle calls no API — the full value is loaded on page mount but masked in the UI.
- When editing, the field clears the masked value and accepts new input. Submitting an empty value means "keep the existing secret."
- Copy button (clipboard icon) copies the actual value to the clipboard without revealing it visually.

---

## 2. General Settings

### 2.1 Route

`/settings/general`

### 2.2 Fields

| Field | Type | Validation | Default | Description |
|-------|------|------------|---------|-------------|
| Application Name | Text input | 1-100 chars | "Tackle" | Displayed in the sidebar header and browser tab title |
| System Timezone | Timezone select | Must be valid IANA timezone | "UTC" | Default timezone for scheduled operations and display when user has no preference |
| Data Retention Period | Number + unit select | 30-3650 days | 365 days | How long to retain campaign data, logs, and metrics before automatic purge |
| Notification Retention | Number + unit select | 7-365 days | 90 days | How long to retain notification records |
| Max Upload Size | Number (MB) | 1-100 | 25 MB | Maximum file upload size for templates, attachments, images |
| Default Landing Page Protocol | Select (HTTP/HTTPS) | — | HTTPS | Default protocol for new landing pages |

### 2.3 Layout

```
┌─────────────────────────────────────────────────────────────┐
│  General Settings                                            │
│  Configure basic platform settings.                          │
│                                                              │
│  Application Name                                            │
│  [Tackle                                                ]    │
│  Displayed in the sidebar and browser tab.                   │
│                                                              │
│  System Timezone                                             │
│  [UTC (Coordinated Universal Time)                    ▾]     │
│  Used for scheduled campaigns and system timestamps.         │
│                                                              │
│  Data Retention                                              │
│  [365] [days ▾]                                              │
│  Campaign data and audit logs older than this are purged.    │
│                                                              │
│  Notification Retention                                      │
│  [90] [days ▾]                                               │
│  Notification records older than this are purged.            │
│                                                              │
│  Max Upload Size                                             │
│  [25] MB                                                     │
│  Maximum size for file uploads (templates, images).          │
│                                                              │
│  Default Landing Page Protocol                               │
│  [HTTPS ▾]                                                   │
│                                                              │
│                              [Cancel]  [Save General]        │
└─────────────────────────────────────────────────────────────┘
```

---

## 3. Security Settings

### 3.1 Route

`/settings/security`

### 3.2 Rate Limiting

| Field | Type | Validation | Default | Description |
|-------|------|------------|---------|-------------|
| Login Rate Limit | Number | 1-100 | 5 | Max login attempts per window |
| Login Rate Window | Number (seconds) | 60-3600 | 300 | Time window for rate limiting |
| API Rate Limit | Number | 10-10000 | 1000 | Max API requests per minute per user |
| API Rate Window | Number (seconds) | 60-3600 | 60 | Time window for API rate limiting |

### 3.3 Account Lockout

| Field | Type | Validation | Default | Description |
|-------|------|------------|---------|-------------|
| Enable Account Lockout | Toggle | — | Enabled | Lock accounts after failed login attempts |
| Max Failed Attempts | Number | 3-20 | 5 | Failed attempts before lockout |
| Lockout Duration | Number (minutes) | 1-1440 | 30 | Duration of account lockout |
| Lockout Notification | Toggle | — | Enabled | Send email notification on lockout |

### 3.4 Layout

```
┌─────────────────────────────────────────────────────────────┐
│  Security Settings                                           │
│  Configure rate limiting and account lockout policies.       │
│                                                              │
│  ── Rate Limiting ──────────────────────────────────────     │
│                                                              │
│  Login Rate Limit                                            │
│  [5] attempts per [300] seconds                              │
│                                                              │
│  API Rate Limit                                              │
│  [1000] requests per [60] seconds                            │
│                                                              │
│  ── Account Lockout ────────────────────────────────────     │
│                                                              │
│  Enable Account Lockout  [====●]                             │
│                                                              │
│  Max Failed Attempts                                         │
│  [5]                                                         │
│                                                              │
│  Lockout Duration                                            │
│  [30] minutes                                                │
│                                                              │
│  Lockout Notification  [====●]                               │
│  Send email to user and admins when an account is locked.    │
│                                                              │
│                              [Cancel]  [Save Security]       │
└─────────────────────────────────────────────────────────────┘
```

- When "Enable Account Lockout" is toggled off, the fields beneath it (Max Failed Attempts, Lockout Duration, Lockout Notification) are visually disabled with `--text-muted` labels and non-interactive inputs.

---

## 4. Authentication Provider Configuration

### 4.1 Route

`/settings/auth-providers`

### 4.2 Purpose

Administrators can configure external authentication providers that users can use to log in. Multiple providers can be configured, but each must be individually enabled. The local username/password authentication is always available and cannot be disabled.

### 4.3 Provider List View

```
┌─────────────────────────────────────────────────────────────────────┐
│  Authentication Providers                           [+ Add Provider]│
│  Configure external identity providers for user authentication.     │
│                                                                     │
│  ┌───────────────────────────────────────────────────────────────┐  │
│  │  ● Local Authentication                          Built-in    │  │
│  │  Username and password authentication     Always enabled      │  │
│  └───────────────────────────────────────────────────────────────┘  │
│                                                                     │
│  ┌───────────────────────────────────────────────────────────────┐  │
│  │  ● Okta OIDC                              [====●] Enabled   │  │
│  │  OpenID Connect  ·  Last synced 2h ago      [Test] [Edit] [⋮]│  │
│  └───────────────────────────────────────────────────────────────┘  │
│                                                                     │
│  ┌───────────────────────────────────────────────────────────────┐  │
│  │  ○ Corporate LDAP                         [●====] Disabled   │  │
│  │  LDAP  ·  Never tested                     [Test] [Edit] [⋮]│  │
│  └───────────────────────────────────────────────────────────────┘  │
│                                                                     │
│  ┌───────────────────────────────────────────────────────────────┐  │
│  │  ● FusionAuth SSO                         [====●] Enabled   │  │
│  │  FusionAuth  ·  Last synced 15m ago         [Test] [Edit] [⋮]│  │
│  └───────────────────────────────────────────────────────────────┘  │
│                                                                     │
└─────────────────────────────────────────────────────────────────────┘
```

- Each provider card shows: status dot (green=enabled, gray=disabled), provider name, provider type badge, last sync/test time, and action buttons.
- The `[⋮]` overflow menu contains: "View logs", "Delete". Delete requires confirmation modal.
- The enable/disable toggle calls `PUT /api/v1/auth-providers/{id}/toggle`.
- "Test" calls `POST /api/v1/auth-providers/{id}/test` and shows a result toast (success or failure with details).

### 4.4 Add/Edit Provider Slide-Over

Clicking "+ Add Provider" or "Edit" opens a slide-over panel from the right (480px width). The slide-over contains a form whose fields change based on the selected provider type.

**Step 1 — Choose Provider Type** (only shown for new providers):

```
┌──────────────────────────────────────────┐
│  Add Authentication Provider          [✕]│
├──────────────────────────────────────────┤
│                                          │
│  Select provider type:                   │
│                                          │
│  ┌────────────────────────────────────┐  │
│  │  🔑 OpenID Connect (OIDC)         │  │
│  │  Okta, Auth0, Azure AD, Google    │  │
│  └────────────────────────────────────┘  │
│                                          │
│  ┌────────────────────────────────────┐  │
│  │  🔐 FusionAuth                    │  │
│  │  FusionAuth identity platform     │  │
│  └────────────────────────────────────┘  │
│                                          │
│  ┌────────────────────────────────────┐  │
│  │  📁 LDAP / Active Directory       │  │
│  │  LDAP v3 or Microsoft AD          │  │
│  └────────────────────────────────────┘  │
│                                          │
└──────────────────────────────────────────┘
```

**Step 2 — Configuration Form** (fields vary by type):

#### 4.4.1 OIDC Configuration Fields

| Field | Type | Validation | Description |
|-------|------|------------|-------------|
| Display Name | Text | Required, 1-100 chars | Human-readable name shown on login page |
| Issuer URL | URL input | Required, valid URL | OIDC issuer URL (e.g., `https://company.okta.com`) |
| Client ID | Text | Required | OAuth client ID |
| Client Secret | Sensitive text | Required | OAuth client secret (masked display) |
| Scopes | Tag input | At least `openid` | OAuth scopes (default: `openid profile email`) |
| Auto-discover | Toggle | — | Use `.well-known/openid-configuration` for endpoints |
| Authorization URL | URL (shown if auto-discover off) | Required | Authorization endpoint |
| Token URL | URL (shown if auto-discover off) | Required | Token endpoint |
| UserInfo URL | URL (shown if auto-discover off) | Required | UserInfo endpoint |
| Username Claim | Text | Required | JWT claim to map to username (default: `preferred_username`) |
| Email Claim | Text | Required | JWT claim to map to email (default: `email`) |
| Auto-create Users | Toggle | — | Create Tackle users on first SSO login |
| Default Role | Role select (shown if auto-create on) | Required | Role assigned to auto-created users |

#### 4.4.2 FusionAuth Configuration Fields

| Field | Type | Validation | Description |
|-------|------|------------|-------------|
| Display Name | Text | Required | Human-readable name |
| FusionAuth URL | URL | Required | Base URL of FusionAuth instance |
| Application ID | Text | Required | FusionAuth application ID |
| API Key | Sensitive text | Required | FusionAuth API key (masked display) |
| Tenant ID | Text | Optional | FusionAuth tenant ID (multi-tenant setups) |
| Auto-create Users | Toggle | — | Create Tackle users on first login |
| Default Role | Role select | Required if auto-create on | Default role for new users |

#### 4.4.3 LDAP Configuration Fields

| Field | Type | Validation | Description |
|-------|------|------------|-------------|
| Display Name | Text | Required | Human-readable name |
| Server URL | URL | Required | LDAP server URL (`ldap://` or `ldaps://`) |
| Bind DN | Text | Required | Distinguished name for binding |
| Bind Password | Sensitive text | Required | Bind password (masked display) |
| User Search Base | Text | Required | Base DN for user search (e.g., `ou=users,dc=company,dc=com`) |
| User Search Filter | Text | Required | LDAP filter (default: `(uid={{username}})`) |
| Group Search Base | Text | Optional | Base DN for group search |
| Group Search Filter | Text | Optional | Filter for group membership |
| Username Attribute | Text | Required | LDAP attribute for username (default: `uid`) |
| Email Attribute | Text | Required | LDAP attribute for email (default: `mail`) |
| Use StartTLS | Toggle | — | Upgrade connection to TLS |
| Skip TLS Verify | Toggle | — | Skip TLS certificate verification (shows warning badge) |
| Auto-create Users | Toggle | — | Create Tackle users on first LDAP login |
| Default Role | Role select | Required if auto-create on | Default role for new users |

### 4.5 Role Mapping

Each provider configuration includes a role mapping section at the bottom of the slide-over form. Role mappings translate external provider groups/roles to Tackle roles.

```
┌──────────────────────────────────────────┐
│  Role Mappings                           │
│  Map external groups to Tackle roles.    │
│                                          │
│  External Group/Role    Tackle Role      │
│  [admins            ]   [Admin      ▾]   │
│  [security-team     ]   [Engineer   ▾]   │
│  [phishing-ops      ]   [Operator   ▾]   │
│                             [+ Add Row]  │
│                                          │
│  If no mapping matches, user gets:       │
│  [Defender ▾] (fallback role)            │
│                                          │
└──────────────────────────────────────────┘
```

- Each row maps an external group name (text input) to a Tackle role (select dropdown).
- Rows can be removed with an `[✕]` button on the right.
- "+ Add Row" appends a new blank mapping row.
- The fallback role is applied when a user's external groups do not match any mapping.

### 4.6 Provider Test

The "Test" button on a provider card initiates a test connection:

1. For OIDC: Attempts to fetch the discovery document and validates the client credentials.
2. For FusionAuth: Calls the FusionAuth health endpoint and validates the API key.
3. For LDAP: Attempts a bind with the configured credentials and performs a sample search.

Test results appear in a modal:

```
┌─────────────────────────────────────────┐
│  Connection Test: Okta OIDC          [✕]│
├─────────────────────────────────────────┤
│                                         │
│  ✓  Discovery document fetched          │
│  ✓  Client credentials valid            │
│  ✓  Token endpoint reachable            │
│  ✕  UserInfo endpoint returned 403      │
│                                         │
│  Result: Partial Success                │
│  The UserInfo endpoint rejected the     │
│  test token. Verify scopes include      │
│  "profile" and "email".                 │
│                                         │
│                              [Close]    │
└─────────────────────────────────────────┘
```

- Each test step shows a checkmark (`✓` in `--success`) or cross (`✕` in `--danger`).
- A summary message with actionable advice is shown below the steps.

---

## 5. Session Settings

### 5.1 Route

`/settings/sessions`

### 5.2 Fields

| Field | Type | Validation | Default | Description |
|-------|------|------------|---------|-------------|
| Access Token Lifetime | Number (minutes) | 5-1440 | 15 | JWT access token expiry |
| Refresh Token Lifetime | Number (hours) | 1-720 | 168 (7 days) | Refresh token expiry |
| Max Concurrent Sessions | Number | 1-20 | 5 | Max active sessions per user |
| Session Idle Timeout | Number (minutes) | 5-480 | 30 | Inactivity timeout before session expiry |
| Enforce Single Session | Toggle | — | Disabled | If enabled, new login terminates all other sessions for that user |
| Remember Me Duration | Number (days) | 1-90 | 30 | Duration of "Remember me" persistent sessions |

### 5.3 Layout

```
┌─────────────────────────────────────────────────────────────┐
│  Session Settings                                            │
│  Configure token lifetimes and session behavior.             │
│                                                              │
│  ── Token Lifetimes ────────────────────────────────────     │
│                                                              │
│  Access Token Lifetime                                       │
│  [15] minutes                                                │
│  Short-lived token for API authentication.                   │
│                                                              │
│  Refresh Token Lifetime                                      │
│  [168] hours (7 days)                                        │
│  Long-lived token used to obtain new access tokens.          │
│                                                              │
│  ── Session Behavior ───────────────────────────────────     │
│                                                              │
│  Max Concurrent Sessions                                     │
│  [5]                                                         │
│  Oldest session is terminated when limit is exceeded.        │
│                                                              │
│  Session Idle Timeout                                        │
│  [30] minutes                                                │
│                                                              │
│  Enforce Single Session  [●====]                             │
│  When enabled, logging in terminates all other sessions.     │
│                                                              │
│  Remember Me Duration                                        │
│  [30] days                                                   │
│                                                              │
│  ⚠ Changing token lifetimes does not affect existing         │
│    sessions. Users must re-authenticate for new values       │
│    to apply.                                                 │
│                                                              │
│                              [Cancel]  [Save Sessions]       │
└─────────────────────────────────────────────────────────────┘
```

- The warning notice at the bottom uses `--warning-muted` background with `--warning` left border.

---

## 6. Password Policy Settings

### 6.1 Route

`/settings/password-policy`

### 6.2 Fields

| Field | Type | Validation | Default | Description |
|-------|------|------------|---------|-------------|
| Minimum Length | Number | 8-128 | 12 | Minimum password character count |
| Require Uppercase | Toggle | — | Enabled | At least one uppercase letter |
| Require Lowercase | Toggle | — | Enabled | At least one lowercase letter |
| Require Number | Toggle | — | Enabled | At least one digit |
| Require Special Character | Toggle | — | Enabled | At least one special character |
| Password Expiry | Number (days) | 0-365 (0=never) | 90 | Days before password must be changed |
| Password History | Number | 0-24 | 5 | Number of previous passwords to remember (prevents reuse) |
| Expire on First Login | Toggle | — | Enabled | Force password change on first login for admin-created accounts |

### 6.3 Layout

```
┌─────────────────────────────────────────────────────────────┐
│  Password Policy                                             │
│  Define password complexity and expiration rules.            │
│                                                              │
│  ── Complexity Requirements ────────────────────────────     │
│                                                              │
│  Minimum Length                                               │
│  [12] characters                                             │
│                                                              │
│  Require Uppercase Letter  [====●]                           │
│  Require Lowercase Letter  [====●]                           │
│  Require Number            [====●]                           │
│  Require Special Character [====●]                           │
│                                                              │
│  ── Expiration ─────────────────────────────────────────     │
│                                                              │
│  Password Expiry                                             │
│  [90] days  (0 = never expires)                              │
│                                                              │
│  Password History                                            │
│  [5] previous passwords remembered                           │
│  Users cannot reuse their last 5 passwords.                  │
│                                                              │
│  Expire on First Login  [====●]                              │
│  Admin-created accounts must change password on first login. │
│                                                              │
│                              [Cancel]  [Save Password Policy]│
└─────────────────────────────────────────────────────────────┘
```

---

## 7. Email / SMTP Configuration

### 7.1 Route

`/settings/email`

### 7.2 Purpose

Configures the SMTP server used for **system notifications** (password resets, lockout alerts, notification digest emails). This is separate from the SMTP profiles used for phishing campaigns (managed in Infrastructure > SMTP Profiles).

### 7.3 Fields

| Field | Type | Validation | Default | Description |
|-------|------|------------|---------|-------------|
| SMTP Host | Text | Required | — | SMTP server hostname |
| SMTP Port | Number | 1-65535 | 587 | SMTP server port |
| Encryption | Select | None / STARTTLS / TLS | STARTTLS | Connection encryption method |
| Username | Text | Optional | — | SMTP authentication username |
| Password | Sensitive text | Optional | — | SMTP authentication password (masked) |
| From Address | Email | Required, valid email | — | Default "From" address for system emails |
| From Name | Text | Optional | "Tackle Platform" | Display name for system emails |
| Reply-To Address | Email | Optional | — | Reply-to address for system emails |

### 7.4 Layout

```
┌─────────────────────────────────────────────────────────────┐
│  Email / SMTP Configuration                                  │
│  Configure the SMTP server for system notification emails.   │
│  This is NOT used for phishing campaign emails — those are   │
│  configured in Infrastructure > SMTP Profiles.               │
│                                                              │
│  ── Server ─────────────────────────────────────────────     │
│                                                              │
│  SMTP Host                                                   │
│  [smtp.company.com                                     ]     │
│                                                              │
│  SMTP Port              Encryption                           │
│  [587]                  [STARTTLS ▾]                         │
│                                                              │
│  ── Authentication ─────────────────────────────────────     │
│                                                              │
│  Username                                                    │
│  [notifications@company.com                            ]     │
│                                                              │
│  Password                                                    │
│  [••••••••••••                                  👁 📋]       │
│                                                              │
│  ── Sender ─────────────────────────────────────────────     │
│                                                              │
│  From Address                                                │
│  [noreply@company.com                                  ]     │
│                                                              │
│  From Name                                                   │
│  [Tackle Platform                                      ]     │
│                                                              │
│  Reply-To Address                                            │
│  [security-team@company.com                            ]     │
│                                                              │
│                    [Send Test Email]  [Cancel]  [Save Email]  │
└─────────────────────────────────────────────────────────────┘
```

### 7.5 Test Email

"Send Test Email" opens a small modal:

```
┌───────────────────────────────────┐
│  Send Test Email               [✕]│
├───────────────────────────────────┤
│                                   │
│  Recipient                        │
│  [admin@company.com          ]    │
│  (defaults to current user email) │
│                                   │
│            [Cancel]  [Send Test]  │
└───────────────────────────────────┘
```

- Sends via `POST /api/v1/settings/email/test` with the recipient address.
- While sending, the button shows a spinner with "Sending...".
- On success: success toast "Test email sent successfully."
- On failure: danger toast with the SMTP error message (e.g., "Connection refused", "Authentication failed").

---

## 8. System Notification Settings

### 8.1 Route

`/settings/notifications`

### 8.2 Purpose

Configures system-wide defaults for notification behavior. Individual users can override some of these settings in their personal preferences (section 16). This section controls what the platform generates, not what individual users receive.

### 8.3 Fields

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| Enable Email Notifications | Toggle | Enabled | Master switch for all email notifications |
| Enable In-App Notifications | Toggle | Enabled | Master switch for in-app notification panel |
| Default Digest Mode | Select | Immediate | Default digest mode for new users (immediate, hourly, daily, weekly) |
| Campaign Event Notifications | Toggle | Enabled | Generate notifications for campaign events |
| Infrastructure Event Notifications | Toggle | Enabled | Generate notifications for infrastructure events |
| Approval Notifications | Toggle | Enabled | Generate notifications for approval workflows |
| System Alert Notifications | Toggle | Enabled | Generate notifications for system alerts |
| Admin Action Notifications | Toggle | Enabled | Generate notifications for admin actions |

### 8.4 Layout

```
┌─────────────────────────────────────────────────────────────┐
│  System Notifications                                        │
│  Control which notification categories the system generates. │
│  Users can override their personal delivery preferences.     │
│                                                              │
│  ── Global Switches ────────────────────────────────────     │
│                                                              │
│  Enable Email Notifications   [====●]                        │
│  Enable In-App Notifications  [====●]                        │
│                                                              │
│  Default Digest Mode                                         │
│  [Immediate ▾]                                               │
│  Applied to new user accounts. Users can override.           │
│                                                              │
│  ── Notification Categories ────────────────────────────     │
│                                                              │
│  Campaign Events         [====●]                             │
│  Campaign started, completed, credential captured, etc.      │
│                                                              │
│  Infrastructure Events   [====●]                             │
│  Endpoint provisioned, endpoint down, domain expiring, etc.  │
│                                                              │
│  Approval Requests       [====●]                             │
│  Pending approval, approved, rejected.                       │
│                                                              │
│  System Alerts           [====●]                             │
│  SMTP failures, rate limit warnings, health check failures.  │
│                                                              │
│  Admin Actions           [====●]                             │
│  User created, role changed, settings modified.              │
│                                                              │
│                        [Cancel]  [Save Notifications]        │
└─────────────────────────────────────────────────────────────┘
```

- When "Enable Email Notifications" is toggled off, a warning appears: "Email notifications are disabled. Users will not receive any email notifications, regardless of their personal preferences."
- Same pattern for in-app notifications toggle.

---

## 9. Webhook Management

### 9.1 Route

`/settings/webhooks`

### 9.2 Purpose

Administrators can configure outbound webhooks that send HTTP POST requests to external URLs when events occur in Tackle. Webhooks are used for integrations with SIEM, ticketing systems, Slack, or custom automation.

### 9.3 Webhook List View

```
┌─────────────────────────────────────────────────────────────────────┐
│  Webhooks                                            [+ Add Webhook]│
│  Send event notifications to external services.                     │
│                                                                     │
│  ┌─────────────────────────────────────────────────────────────┐    │
│  │  ● Slack Alerts                              [====●] Active │    │
│  │  https://hooks.slack.com/services/T.../B.../xxx              │    │
│  │  Events: campaign.*, infrastructure.down                     │    │
│  │  Auth: HMAC-SHA256  ·  Last delivery: 2m ago  ·  98% success│    │
│  │                                     [Deliveries] [Edit] [⋮] │    │
│  └─────────────────────────────────────────────────────────────┘    │
│                                                                     │
│  ┌─────────────────────────────────────────────────────────────┐    │
│  │  ○ SIEM Integration                          [●====] Paused │    │
│  │  https://siem.internal/api/webhooks/tackle                   │    │
│  │  Events: audit.*                                             │    │
│  │  Auth: Bearer Token  ·  Last delivery: 3d ago  ·  85% success│   │
│  │                                     [Deliveries] [Edit] [⋮] │    │
│  └─────────────────────────────────────────────────────────────┘    │
│                                                                     │
│  ┌─────────────────────────────────────────────────────────────┐    │
│  │  ● Jira Ticket Creator                       [====●] Active │    │
│  │  https://jira.company.com/rest/webhooks/tackle               │    │
│  │  Events: approval.requested                                  │    │
│  │  Auth: Basic  ·  Last delivery: 1h ago  ·  100% success     │    │
│  │                                     [Deliveries] [Edit] [⋮] │    │
│  └─────────────────────────────────────────────────────────────┘    │
│                                                                     │
└─────────────────────────────────────────────────────────────────────┘
```

- Status dot: green (`--success`) for active, gray (`--text-muted`) for paused.
- Success rate uses color coding: 95-100% green, 80-94% yellow, below 80% red.
- The `[⋮]` overflow menu contains: "Send test event", "Delete". Delete requires confirmation modal.
- Enable/disable toggle calls `PUT /api/v1/webhooks/{id}/toggle`.

### 9.4 Add/Edit Webhook Slide-Over

Clicking "+ Add Webhook" or "Edit" opens a slide-over panel (480px width).

| Field | Type | Validation | Description |
|-------|------|------------|-------------|
| Name | Text | Required, 1-100 chars | Human-readable name |
| URL | URL | Required, valid HTTPS URL | Endpoint to receive POST requests |
| Description | Textarea | Optional, max 500 chars | Purpose description |
| Authentication Type | Select | Required | none, hmac, bearer, basic |
| Secret (HMAC) | Sensitive text | Required if HMAC | Shared secret for HMAC-SHA256 signing |
| Token (Bearer) | Sensitive text | Required if Bearer | Bearer token value |
| Username (Basic) | Text | Required if Basic | HTTP Basic username |
| Password (Basic) | Sensitive text | Required if Basic | HTTP Basic password |
| Events | Multi-select checkbox list | At least one | Event types to subscribe to |
| Custom Headers | Key-value pairs | Optional | Additional HTTP headers to include |
| Timeout | Number (seconds) | 1-30, default 10 | Request timeout |
| Retry Count | Number | 0-5, default 3 | Number of retry attempts on failure |
| Enabled | Toggle | — | Active/paused state |

**Event types available for subscription:**

```
┌──────────────────────────────────────────┐
│  Events to subscribe to:                 │
│                                          │
│  ── Campaign ───────────────────────     │
│  ☑ campaign.created                      │
│  ☑ campaign.started                      │
│  ☑ campaign.completed                    │
│  ☑ campaign.paused                       │
│  ☐ campaign.credential_captured          │
│  ☐ campaign.email_opened                 │
│  ☐ campaign.link_clicked                 │
│                                          │
│  ── Infrastructure ─────────────────     │
│  ☑ infrastructure.endpoint_provisioned   │
│  ☑ infrastructure.endpoint_down          │
│  ☐ infrastructure.domain_expiring        │
│  ☐ infrastructure.smtp_failure           │
│                                          │
│  ── Approval ───────────────────────     │
│  ☐ approval.requested                    │
│  ☐ approval.approved                     │
│  ☐ approval.rejected                     │
│                                          │
│  ── System ─────────────────────────     │
│  ☐ system.user_created                   │
│  ☐ system.role_changed                   │
│  ☐ system.settings_modified              │
│                                          │
│  [Select All]  [Deselect All]            │
└──────────────────────────────────────────┘
```

### 9.5 Delivery History

Clicking "Deliveries" on a webhook card navigates to a delivery log for that webhook.

Route: `/settings/webhooks/{id}/deliveries`

```
┌──────────────────────────────────────────────────────────────────────────┐
│  ← Back to Webhooks     Deliveries: Slack Alerts                        │
├──────────────────────────────────────────────────────────────────────────┤
│  Filter: [Status ▾] [Event ▾] [Date range ···]        [Clear filters]  │
├──────────────────────────────────────────────────────────────────────────┤
│  Timestamp              Event                  Status     Duration      │
├──────────────────────────────────────────────────────────────────────────┤
│  2026-04-03 14:30:01    campaign.started       ✓ 200      145ms        │
│  2026-04-03 14:25:12    infrastructure.down    ✓ 200      230ms        │
│  2026-04-03 12:00:00    campaign.completed     ✕ 500      10,002ms  🔄 │
│  2026-04-03 11:58:45    campaign.started       ✓ 200      98ms         │
│  ...                                                                    │
├──────────────────────────────────────────────────────────────────────────┤
│  Page 1 of 12                              [← Prev]  [Next →]          │
└──────────────────────────────────────────────────────────────────────────┘
```

- Status column: green checkmark + HTTP status for success (2xx), red cross + HTTP status for failure.
- The `🔄` retry icon on failed deliveries calls `POST /api/v1/webhooks/{id}/deliveries/{delivery_id}/retry`.
- Clicking a row expands an inline detail panel showing the request payload (JSON), response body, and response headers.
- Deliveries are paginated server-side, 25 per page.

---

## 10. API Key Management

### 10.1 Route

`/settings/api-keys`

### 10.2 Purpose

Administrators can create API keys for programmatic access to the Tackle API. Each key has scoped permissions and an optional expiry date. Keys are displayed only once at creation time.

### 10.3 API Key List

```
┌──────────────────────────────────────────────────────────────────────────┐
│  API Keys                                                 [+ Create Key]│
│  Manage API keys for programmatic access.                               │
│                                                                         │
│  Name             Prefix       Permissions      Expires     Created     │
├──────────────────────────────────────────────────────────────────────────┤
│  CI/CD Pipeline   tk_abc1...   Full Access      2026-12-31  2026-01-15  │
│                                                          [Revoke]       │
│  Metrics Export   tk_def2...   Read Only        Never       2026-03-01  │
│                                                          [Revoke]       │
│  SIEM Feed        tk_ghi3...   Logs + Metrics   2026-06-30  2026-02-20  │
│                                                          [Revoke]       │
│                                                                         │
│  Revoked keys are hidden.  [Show revoked (2)]                           │
└──────────────────────────────────────────────────────────────────────────┘
```

- The "Prefix" column shows only the first 8 characters of the key followed by `...`. The full key is never displayed after creation.
- "Revoke" opens a confirmation modal: "Revoke API key '[Name]'? This action cannot be undone. Any systems using this key will lose access immediately."
- Revoked keys are hidden by default. "Show revoked" toggles their visibility. Revoked keys display with `--text-muted` styling and a "Revoked" badge with strikethrough on the name.

### 10.4 Create API Key Modal

Clicking "+ Create Key" opens a modal:

```
┌─────────────────────────────────────────────────┐
│  Create API Key                              [✕]│
├─────────────────────────────────────────────────┤
│                                                 │
│  Name                                           │
│  [CI/CD Pipeline                           ]    │
│  A descriptive name for this key.               │
│                                                 │
│  Expiry                                         │
│  ○ Never expires                                │
│  ● Expires on: [2026-12-31         📅]          │
│                                                 │
│  Permissions                                    │
│  ○ Full Access (all permissions)                │
│  ● Scoped (select specific permissions)         │
│                                                 │
│  ┌────────────────────────────────────────────┐ │
│  │  Resource          Read  Write  Delete     │ │
│  │  Campaigns          ☑     ☐      ☐        │ │
│  │  Targets            ☑     ☑      ☐        │ │
│  │  Templates          ☑     ☐      ☐        │ │
│  │  Infrastructure     ☑     ☐      ☐        │ │
│  │  Metrics            ☑     ☐      ☐        │ │
│  │  Audit Logs         ☑     ☐      ☐        │ │
│  │  Users              ☐     ☐      ☐        │ │
│  │  Settings           ☐     ☐      ☐        │ │
│  └────────────────────────────────────────────┘ │
│                                                 │
│                    [Cancel]  [Create Key]        │
└─────────────────────────────────────────────────┘
```

- The permission matrix is only visible when "Scoped" is selected.
- Checking "Write" automatically checks "Read" for that resource. Checking "Delete" automatically checks "Write" and "Read".

### 10.5 Key Display (Post-Creation)

After successful creation, the modal transforms to show the key:

```
┌─────────────────────────────────────────────────┐
│  API Key Created                             [✕]│
├─────────────────────────────────────────────────┤
│                                                 │
│  ⚠ Copy this key now. It will not be shown      │
│    again.                                       │
│                                                 │
│  ┌────────────────────────────────────────────┐ │
│  │ tk_abc123def456ghi789jkl012mno345pqr678    │ │
│  │                                    [Copy]  │ │
│  └────────────────────────────────────────────┘ │
│                                                 │
│  Name: CI/CD Pipeline                           │
│  Expires: 2026-12-31                            │
│  Permissions: Scoped (3 resources)              │
│                                                 │
│                                       [Done]    │
└─────────────────────────────────────────────────┘
```

- The key is displayed in a monospace font on a `--bg-primary` background.
- The warning banner uses `--warning-muted` background with `--warning` left border.
- "Copy" copies the full key to the clipboard and changes to "Copied!" for 2 seconds.
- The modal cannot be closed by clicking outside — only via the "Done" button or the close icon. This prevents accidental dismissal before copying.

---

## 11. User Management Page

### 11.1 Navigation

- Sidebar location: **Administration > Users & Roles**.
- Route: `/admin/users` (default tab).
- Requires permission: `users:read` (view), `users:write` (create/edit), `users:delete` (delete).
- Page title: "Users & Roles".

### 11.2 Tab Layout

The Users & Roles page uses a horizontal tab bar at the top.

```
┌──────────────────────────────────────────────────────────────────────────┐
│  Users & Roles                                                          │
├──────────────────────────────────────────────────────────────────────────┤
│  [Users]  [Roles]                                                       │
├──────────────────────────────────────────────────────────────────────────┤
│                                                                          │
│  (Active tab content)                                                    │
│                                                                          │
└──────────────────────────────────────────────────────────────────────────┘
```

| Tab | Route | Content |
|-----|-------|---------|
| Users | `/admin/users` | User list and management (this section) |
| Roles | `/admin/roles` | Role list and permission management (section 12) |

### 11.3 User List

```
┌──────────────────────────────────────────────────────────────────────────┐
│  [Search users...                            🔍]        [+ Create User] │
│  [Role ▾] [Status ▾] [Auth Provider ▾]                 [Clear filters] │
├──────────────────────────────────────────────────────────────────────────┤
│  ☐  User                    Email                Role      Status    ⋮  │
├──────────────────────────────────────────────────────────────────────────┤
│  ☐  ██ John Administrator   john@company.com     Admin     ● Active  ⋮  │
│  ☐  ██ Sarah Engineer       sarah@company.com    Engineer  ● Active  ⋮  │
│  ☐  ██ Mike Operator        mike@company.com     Operator  ● Active  ⋮  │
│  ☐  ██ Jane Defender        jane@company.com     Defender  ○ Locked  ⋮  │
│  ☐  ██ Bob Analyst          bob@company.com      Operator  ◐ Invited ⋮  │
│  ...                                                                    │
├──────────────────────────────────────────────────────────────────────────┤
│  Showing 1-25 of 47                        [← Prev]  [Next →]          │
└──────────────────────────────────────────────────────────────────────────┘
```

**Table columns:**

| Column | Content | Sortable |
|--------|---------|----------|
| Checkbox | Row selection for bulk actions | No |
| User | Avatar (initials) + full name + username below in muted text | Yes (by name) |
| Email | Email address | Yes |
| Role | Role badge with role-specific color | Yes |
| Status | Status indicator dot + label | Yes |
| Actions | Overflow menu `[⋮]` | No |

**Status values:**

| Status | Indicator | Color |
|--------|-----------|-------|
| Active | ● | `--success` |
| Locked | ○ | `--danger` |
| Invited | ◐ | `--warning` |
| Disabled | ○ | `--text-muted` |

**Overflow menu actions:**

| Action | Behavior |
|--------|----------|
| Edit | Opens edit slide-over |
| Lock / Unlock | Toggles account lock. Lock shows confirmation modal |
| Reset Password | Sends password reset email. Confirmation modal |
| View Sessions | Opens session list modal for this user |
| Disable / Enable | Toggles account active status. Confirmation modal |
| Delete | Confirmation modal with username typed confirmation |

**Bulk actions** (appear when checkboxes are selected):
- A toolbar appears above the table: "[N] selected — [Lock] [Unlock] [Disable] [Delete]"
- Each bulk action requires a confirmation modal.
- Cannot bulk-delete the last admin user. The UI disables the delete button and shows a tooltip: "Cannot delete the last admin user."

### 11.4 Create/Edit User Slide-Over

Clicking "+ Create User" or "Edit" opens a slide-over panel (480px width).

**Create User fields:**

| Field | Type | Validation | Description |
|-------|------|------------|-------------|
| First Name | Text | Required, 1-50 chars | User's first name |
| Last Name | Text | Required, 1-50 chars | User's last name |
| Username | Text | Required, 3-30 chars, alphanumeric + underscore, unique | Login username |
| Email | Email | Required, valid email, unique | User's email address |
| Role | Select | Required | Role assignment |
| Auth Provider | Select | Required (default: Local) | Authentication provider |
| Set Password | Password + confirm | Required if provider is Local | Initial password |
| Force Password Change | Toggle | Default: on | Require password change on first login |
| Send Welcome Email | Toggle | Default: on | Send invitation email with login details |

**Edit User fields** — same as create except:
- Username is read-only (cannot be changed after creation).
- Password fields are replaced with a "Reset Password" button.
- Shows "Created" and "Last Login" timestamps as read-only info.
- Shows "Auth Provider" as read-only if the user was created via SSO.

```
┌──────────────────────────────────────────┐
│  Create User                          [✕]│
├──────────────────────────────────────────┤
│                                          │
│  First Name           Last Name          │
│  [                ]    [                ]│
│                                          │
│  Username                                │
│  [                                  ]    │
│                                          │
│  Email                                   │
│  [                                  ]    │
│                                          │
│  Role                                    │
│  [Operator ▾]                            │
│                                          │
│  Auth Provider                           │
│  [Local ▾]                               │
│                                          │
│  ── Password ───────────────────────     │
│                                          │
│  Password                                │
│  [                                  ]    │
│  Strength: ████░░░░░░ Fair               │
│                                          │
│  Confirm Password                        │
│  [                                  ]    │
│                                          │
│  Force Password Change  [====●]          │
│  Send Welcome Email     [====●]          │
│                                          │
│              [Cancel]  [Create User]     │
└──────────────────────────────────────────┘
```

**Password strength indicator** — a horizontal bar that fills and changes color:
- 0-25%: Red (`--danger`), label "Weak"
- 26-50%: Orange (`--warning`), label "Fair"
- 51-75%: Yellow-green, label "Good"
- 76-100%: Green (`--success`), label "Strong"
- Strength is calculated client-side based on the current password policy settings (length, character requirements).

### 11.5 User Sessions Modal

The "View Sessions" action opens a modal showing all active sessions for a user.

```
┌──────────────────────────────────────────────────────────┐
│  Active Sessions: John Administrator                  [✕]│
├──────────────────────────────────────────────────────────┤
│                                                          │
│  ┌────────────────────────────────────────────────────┐  │
│  │  🖥 Chrome on Windows                  ★ Current   │  │
│  │  IP: 192.168.1.100  ·  Last active: now            │  │
│  │  Created: 2026-04-03 09:15                         │  │
│  └────────────────────────────────────────────────────┘  │
│                                                          │
│  ┌────────────────────────────────────────────────────┐  │
│  │  📱 Safari on macOS                                │  │
│  │  IP: 10.0.0.55  ·  Last active: 2h ago             │  │
│  │  Created: 2026-04-02 16:30           [Terminate]   │  │
│  └────────────────────────────────────────────────────┘  │
│                                                          │
│  ┌────────────────────────────────────────────────────┐  │
│  │  💻 API Key: CI/CD Pipeline                        │  │
│  │  IP: 172.16.0.10  ·  Last active: 15m ago          │  │
│  │  Created: 2026-04-01 08:00           [Terminate]   │  │
│  └────────────────────────────────────────────────────┘  │
│                                                          │
│  3 active sessions (max 5)                               │
│                                                          │
│                    [Terminate All Others]  [Close]       │
└──────────────────────────────────────────────────────────┘
```

- The current session (if viewing own sessions) is marked with a star badge and cannot be terminated.
- "Terminate" on individual sessions calls `DELETE /api/v1/users/{id}/sessions/{session_id}`.
- "Terminate All Others" calls `DELETE /api/v1/users/{id}/sessions` (preserving the current session).
- Session count shows current vs. max as configured in session settings.

---

## 12. Role Management

### 12.1 Route

`/admin/roles`

### 12.2 Role List

```
┌──────────────────────────────────────────────────────────────────────────┐
│  Roles                                                [+ Create Role]   │
│                                                                         │
│  ┌──────────────────────────────────────────────────────────────────┐   │
│  │  🔒 Admin                                        Built-in       │   │
│  │  Full system access. Cannot be modified or deleted.              │   │
│  │  4 users assigned                                                │   │
│  └──────────────────────────────────────────────────────────────────┘   │
│                                                                         │
│  ┌──────────────────────────────────────────────────────────────────┐   │
│  │  🔒 Engineer                                     Built-in       │   │
│  │  Infrastructure management and campaign configuration.           │   │
│  │  6 users assigned                                   [View]       │   │
│  └──────────────────────────────────────────────────────────────────┘   │
│                                                                         │
│  ┌──────────────────────────────────────────────────────────────────┐   │
│  │  🔒 Operator                                     Built-in       │   │
│  │  Campaign operations and target management.                      │   │
│  │  12 users assigned                                  [View]       │   │
│  └──────────────────────────────────────────────────────────────────┘   │
│                                                                         │
│  ┌──────────────────────────────────────────────────────────────────┐   │
│  │  🔒 Defender                                     Built-in       │   │
│  │  Read-only access to metrics and reports.                        │   │
│  │  8 users assigned                                   [View]       │   │
│  └──────────────────────────────────────────────────────────────────┘   │
│                                                                         │
│  ┌──────────────────────────────────────────────────────────────────┐   │
│  │  ✏ Security Analyst                              Custom         │   │
│  │  Extended metrics access with campaign read permissions.         │   │
│  │  3 users assigned                          [Edit] [Delete]       │   │
│  └──────────────────────────────────────────────────────────────────┘   │
│                                                                         │
└──────────────────────────────────────────────────────────────────────────┘
```

- Built-in roles display a lock icon and "Built-in" badge. They cannot be edited or deleted. They only have a "View" button that opens the permission matrix in read-only mode.
- Custom roles show an edit icon and have "Edit" and "Delete" buttons.
- "Delete" requires a confirmation modal. If users are assigned to the role, the modal includes a reassignment step: "3 users are assigned to this role. Reassign them to: [Select role ▾]".
- User count is a clickable link that opens the Users tab filtered by that role.

### 12.3 Create/Edit Role Slide-Over

Clicking "+ Create Role" or "Edit" on a custom role opens a slide-over (560px width, wider than typical to accommodate the permission matrix).

```
┌────────────────────────────────────────────────────────┐
│  Create Custom Role                                 [✕]│
├────────────────────────────────────────────────────────┤
│                                                        │
│  Role Name                                             │
│  [Security Analyst                                ]    │
│                                                        │
│  Description                                           │
│  [Extended metrics access with campaign read      ]    │
│  [permissions.                                    ]    │
│                                                        │
│  ── Permissions ───────────────────────────────────    │
│                                                        │
│  Resource          Read   Write  Delete  Admin         │
│  ─────────────────────────────────────────────────     │
│  Campaigns          ☑      ☐      ☐      ☐           │
│  Targets            ☑      ☐      ☐      ☐           │
│  Email Templates    ☑      ☐      ☐      ☐           │
│  Landing Pages      ☑      ☐      ☐      ☐           │
│  Domains            ☑      ☐      ☐      ☐           │
│  SMTP Profiles      ☑      ☐      ☐      ☐           │
│  Cloud Credentials  ☐      ☐      ☐      ☐           │
│  Instance Templates ☐      ☐      ☐      ☐           │
│  Metrics            ☑      ☐      ☐      ☐           │
│  Reports            ☑      ☑      ☐      ☐           │
│  Audit Logs         ☑      ☐      ☐      ☐           │
│  Users              ☐      ☐      ☐      ☐           │
│  Roles              ☐      ☐      ☐      ☐           │
│  Settings           ☐      ☐      ☐      ☐           │
│  API Keys           ☐      ☐      ☐      ☐           │
│  Webhooks           ☐      ☐      ☐      ☐           │
│                                                        │
│  [Select All Read]  [Clear All]                        │
│                                                        │
│              [Cancel]  [Create Role]                   │
└────────────────────────────────────────────────────────┘
```

**Permission matrix rules:**
- Columns represent actions: Read, Write, Delete, Admin.
- Rows represent resource types.
- Checking "Write" automatically checks "Read" for that resource.
- Checking "Delete" automatically checks "Write" and "Read".
- Checking "Admin" automatically checks all other columns for that resource.
- Unchecking "Read" automatically unchecks all other columns for that resource.
- "Select All Read" checks the Read column for every resource. "Clear All" unchecks everything.
- The permission matrix is the data structure: `{ resource_type: string, actions: string[] }[]`.

### 12.4 View Built-in Role

"View" on a built-in role opens the same slide-over in read-only mode:
- All fields are disabled and non-interactive.
- The header reads "View Role: Admin" (or whatever the role name is).
- A notice bar at the top: "Built-in roles cannot be modified."
- No save/cancel buttons. Only a "Close" button.

---

## 13. Cloud Provider Credentials (Cross-Reference)

### 13.1 Route

`/infrastructure/cloud-credentials`

### 13.2 Cross-Reference Note

Cloud provider credential management (AWS, Azure, GCP, DigitalOcean, Linode) is fully specified in **Document 09 — Infrastructure Management**, section 8. The Settings page does NOT duplicate this functionality. The sidebar navigation for "Cloud & Endpoints" under Infrastructure is the canonical location.

The Settings page may display a read-only summary card in the General tab linking to the Infrastructure section:

```
┌─────────────────────────────────────────────────────────────┐
│  Cloud Providers                                             │
│  3 providers configured  ·  2 healthy  ·  1 needs attention  │
│                          [Manage in Infrastructure →]        │
└─────────────────────────────────────────────────────────────┘
```

---

## 14. User Preferences Panel

### 14.1 Navigation

- Accessed via: Top bar user menu > "Profile & Preferences".
- Route: `/profile/preferences`.
- Available to all authenticated users (no special permissions required).
- Page title: "Profile & Preferences".

### 14.2 Purpose

User preferences are per-user settings that control the UI experience. Preferences are stored server-side via `PUT /api/v1/users/me/preferences`, loaded on session start, and applied immediately when changed. There is no "Save" button — each preference change auto-saves with a debounced 500ms delay and a subtle "Saved" indicator.

### 14.3 Layout

```
┌──────────────────────────────────────────────────────────────────────────┐
│  Profile & Preferences                                                   │
├──────────────────────────────────────────────────────────────────────────┤
│                                                                          │
│  ┌──────────────────────────────────────────────────────────────────┐   │
│  │  Profile                                                         │   │
│  │                                                                  │   │
│  │  ██  John Administrator                                          │   │
│  │      john@company.com  ·  Admin  ·  Joined Mar 2026              │   │
│  │                                                                  │   │
│  │  First Name           Last Name                                  │   │
│  │  [John            ]   [Administrator  ]                          │   │
│  │                                                                  │   │
│  │  Email                                                           │   │
│  │  [john@company.com                                ]              │   │
│  │                                                                  │   │
│  │                                       [Update Profile]           │   │
│  └──────────────────────────────────────────────────────────────────┘   │
│                                                                          │
│  ┌──────────────────────────────────────────────────────────────────┐   │
│  │  Notification Preferences                                  Saved │   │
│  │  (section 15 content)                                            │   │
│  └──────────────────────────────────────────────────────────────────┘   │
│                                                                          │
│  ┌──────────────────────────────────────────────────────────────────┐   │
│  │  Display Preferences                                             │   │
│  │  (section 16 content)                                            │   │
│  └──────────────────────────────────────────────────────────────────┘   │
│                                                                          │
│  ┌──────────────────────────────────────────────────────────────────┐   │
│  │  Security                                                        │   │
│  │                                                                  │   │
│  │  Password                                                        │   │
│  │  Last changed: 45 days ago                                       │   │
│  │                                        [Change Password]         │   │
│  │                                                                  │   │
│  │  Active Sessions                                                 │   │
│  │  3 active sessions (this device + 2 others)                      │   │
│  │                                        [Manage Sessions]         │   │
│  └──────────────────────────────────────────────────────────────────┘   │
│                                                                          │
└──────────────────────────────────────────────────────────────────────────┘
```

### 14.4 Profile Section

- "Update Profile" calls `PUT /api/v1/users/me` with the changed fields.
- Email changes may require re-verification depending on auth provider.
- If the user was created via an external auth provider, the name and email fields are read-only with a note: "Managed by [Provider Name]. Contact your administrator to update."

### 14.5 Security Section

- "Change Password" opens a modal (same as described in Document 02, section 3.4).
- "Manage Sessions" opens the sessions modal (same as section 11.5, but scoped to the current user via `/api/v1/users/me/sessions`).

---

## 15. Notification Preferences

### 15.1 Location

Rendered as a card within the Profile & Preferences page (section 14).

### 15.2 Purpose

Users control which notification categories they receive and via which channels (in-app panel, email). They also choose their digest mode. These preferences only apply to the current user and override system defaults (section 8) for opt-out only — users cannot enable categories that an admin has disabled system-wide.

### 15.3 Layout

```
┌──────────────────────────────────────────────────────────────────┐
│  Notification Preferences                                  Saved │
│                                                                  │
│  Digest Mode                                                     │
│  [Immediate ▾]                                                   │
│  How often batched notifications are delivered.                   │
│  Options: Immediate, Hourly, Daily (9:00 AM), Weekly (Monday)    │
│                                                                  │
│  ── Category Preferences ───────────────────────────────────     │
│                                                                  │
│  Category                    In-App    Email                     │
│  ────────────────────────────────────────────                    │
│  Campaign Events              [✓]       [✓]                     │
│  Campaign started, completed, credential captured                │
│                                                                  │
│  Infrastructure Events        [✓]       [✓]                     │
│  Endpoint provisioned, down, domain expiring                     │
│                                                                  │
│  Approval Requests            [✓]       [✓]                     │
│  Pending approval, approved, rejected                            │
│                                                                  │
│  System Alerts                [✓]       [ ]                     │
│  SMTP failures, rate limit warnings                              │
│                                                                  │
│  Admin Actions                [✓]       [ ]                     │
│  User created, role changed, settings modified                   │
│                                                                  │
└──────────────────────────────────────────────────────────────────┘
```

**Behavior:**

- Each checkbox change auto-saves (debounced 500ms) via `PUT /api/v1/users/me/preferences`.
- The "Saved" indicator in the card header appears briefly (fades in, stays for 2 seconds, fades out) after each successful save.
- If a category is disabled system-wide by an admin, the row is grayed out with a lock icon and tooltip: "Disabled by administrator."
- If email notifications are globally disabled by an admin, the Email column header shows a warning icon with tooltip: "Email notifications are disabled system-wide."
- Digest mode select updates immediately on change. The description text below the select updates to reflect the chosen mode.

### 15.4 API

- `GET /api/v1/users/me/preferences` — returns all user preferences including notification settings.
- `PUT /api/v1/users/me/preferences` — updates preferences. Only changed keys need to be sent.

Notification preference structure:
```
{
  "notifications": {
    "digest_mode": "immediate" | "hourly" | "daily" | "weekly",
    "categories": {
      "campaign_events":        { "in_app": true, "email": true },
      "infrastructure_events":  { "in_app": true, "email": true },
      "approval_requests":      { "in_app": true, "email": true },
      "system_alerts":          { "in_app": true, "email": false },
      "admin_actions":          { "in_app": true, "email": false }
    }
  }
}
```

---

## 16. Display Preferences

### 16.1 Location

Rendered as a card within the Profile & Preferences page (section 14).

### 16.2 Layout

```
┌──────────────────────────────────────────────────────────────────┐
│  Display Preferences                                       Saved │
│                                                                  │
│  ── Date & Time ─────────────────────────────────────────────    │
│                                                                  │
│  Timezone                                                        │
│  [America/New_York (Eastern Time)                         ▾]     │
│  Overrides the system timezone for your session.                  │
│                                                                  │
│  Date Format                                                     │
│  [YYYY-MM-DD ▾]                                                  │
│  Options: YYYY-MM-DD, MM/DD/YYYY, DD/MM/YYYY, DD.MM.YYYY        │
│                                                                  │
│  Time Format                                                     │
│  ○ 24-hour (14:30)                                               │
│  ● 12-hour (2:30 PM)                                             │
│                                                                  │
│  ── Dashboard ───────────────────────────────────────────────    │
│                                                                  │
│  Default Dashboard View                                          │
│  [Overview ▾]                                                    │
│  The view shown when you navigate to the Dashboard.              │
│  Options: Overview, Campaign Focus, Infrastructure Focus         │
│                                                                  │
│  Default Date Range                                              │
│  [Last 30 days ▾]                                                │
│  Options: Last 7 days, Last 30 days, Last 90 days, This month,  │
│  This quarter, This year                                         │
│                                                                  │
│  ── Tables ──────────────────────────────────────────────────    │
│                                                                  │
│  Default Page Size                                               │
│  [25 ▾]                                                          │
│  Options: 10, 25, 50, 100                                        │
│                                                                  │
│  Default Sort Order                                              │
│  [Newest first ▾]                                                │
│  Options: Newest first, Oldest first, Alphabetical (A-Z),        │
│  Alphabetical (Z-A)                                              │
│                                                                  │
│  Compact Table Rows  [●====]                                     │
│  Reduce row height for denser data display.                      │
│                                                                  │
└──────────────────────────────────────────────────────────────────┘
```

### 16.3 Preference Application

- All display preferences are applied immediately upon change. No page reload required.
- Timezone changes re-render all visible timestamps across the application.
- Date/time format changes re-render all formatted dates across the application.
- Table preferences apply as defaults when a table is first loaded. Per-table overrides (column visibility, sort order) are stored separately and take precedence.
- Dashboard view preference determines which view loads when navigating to `/dashboard`.

### 16.4 Preference Data Structure

```
{
  "display": {
    "timezone": "America/New_York",
    "date_format": "YYYY-MM-DD",
    "time_format": "24h" | "12h",
    "dashboard_default_view": "overview" | "campaign_focus" | "infrastructure_focus",
    "dashboard_default_range": "7d" | "30d" | "90d" | "this_month" | "this_quarter" | "this_year",
    "table_page_size": 10 | 25 | 50 | 100,
    "table_sort_order": "newest" | "oldest" | "alpha_asc" | "alpha_desc",
    "table_compact": false
  }
}
```

### 16.5 Per-Table Column Preferences

Beyond the global defaults, each table in the application supports per-table column configuration. This is managed via a column toggle dropdown in each table header (not in the preferences page).

When a user toggles column visibility or reorders columns in any table, the configuration is saved to their preferences under:

```
{
  "tables": {
    "users_list": {
      "visible_columns": ["name", "email", "role", "status", "last_login"],
      "column_order": ["name", "email", "role", "status", "last_login"],
      "sort_by": "name",
      "sort_direction": "asc"
    },
    "campaigns_list": { ... },
    "targets_list": { ... }
  }
}
```

This data is auto-saved on change and loaded on each table mount. If no per-table config exists, the global defaults from display preferences are used.

---

## 17. Error States

### 17.1 Settings Load Failure

If `GET /api/v1/settings` fails:

```
┌──────────────────────────────────────────────────────────────────┐
│  Settings                                                        │
├──────────────┬───────────────────────────────────────────────────┤
│              │                                                   │
│  General     │  ┌─────────────────────────────────────────────┐  │
│  Security    │  │                                              │  │
│  ...         │  │     ⚠ Unable to load settings               │  │
│              │  │                                              │  │
│              │  │     Could not connect to the server.         │  │
│              │  │     Check your network connection and        │  │
│              │  │     try again.                               │  │
│              │  │                                              │  │
│              │  │              [Retry]                         │  │
│              │  │                                              │  │
│              │  └─────────────────────────────────────────────┘  │
│              │                                                   │
├──────────────┴───────────────────────────────────────────────────┤
```

- The tab navigation remains visible so the user can attempt to load a different section.
- The error message adapts based on the HTTP status: 403 shows "You do not have permission to view settings.", 500 shows "An unexpected error occurred.", network error shows "Could not connect to the server."

### 17.2 Settings Save Failure

- On save failure, the "Save Section" button re-enables and the fields retain their edited (unsaved) values.
- A danger toast appears with the error message.
- If the failure is a 409 (conflict — another admin changed the setting concurrently), a modal appears:
  ```
  "Settings conflict: These settings were modified by another
   administrator. Do you want to overwrite their changes or
   reload the current values?"
   [Reload]  [Overwrite]
  ```

### 17.3 User Management Errors

| Scenario | Behavior |
|----------|----------|
| Create user with duplicate email | Inline error under email field: "A user with this email already exists." |
| Create user with duplicate username | Inline error under username field: "This username is taken." |
| Delete last admin user | Button disabled, tooltip: "Cannot delete the last admin user." |
| Lock own account | Button disabled, tooltip: "You cannot lock your own account." |
| Delete own account | Button disabled, tooltip: "You cannot delete your own account." |
| Role assignment with no permissions | Warning toast: "This role has no permissions. The user will not be able to access any features." |
| Password does not meet policy | Live validation below password field showing unmet requirements in red |
| Session termination failure | Danger toast: "Could not terminate session. It may have already expired." |

### 17.4 Auth Provider Errors

| Scenario | Behavior |
|----------|----------|
| Test connection fails | Test result modal shows failed steps with error details |
| Save provider with invalid URL | Inline error: "Invalid URL format." |
| Delete provider with active users | Confirmation modal warns: "N users authenticate via this provider. They will need to use another method or be reassigned." |
| Enable provider that fails validation | Toast: "Cannot enable provider. Run a connection test first and resolve any issues." |

### 17.5 Webhook Errors

| Scenario | Behavior |
|----------|----------|
| Invalid URL | Inline error: "URL must be a valid HTTPS endpoint." |
| Test delivery fails | Toast with failure reason and HTTP status code |
| Retry delivery fails | Toast: "Retry failed. The endpoint may be unreachable." |
| Delete webhook with pending deliveries | Confirmation modal: "This webhook has N pending deliveries that will be canceled." |

### 17.6 API Key Errors

| Scenario | Behavior |
|----------|----------|
| Create with duplicate name | Inline error: "An API key with this name already exists." |
| Revoke fails | Toast: "Could not revoke key. Please try again." |
| Max keys reached | "+ Create Key" button disabled, tooltip: "Maximum number of API keys reached (20)." |

### 17.7 Preferences Save Failure

- Since preferences auto-save, a failure shows a brief inline error indicator in the card header where "Saved" normally appears — the text changes to "Save failed" in `--danger` color.
- After 3 seconds, the indicator fades and the preference value reverts to its last successfully saved state.
- If the failure persists (3 consecutive failures), a persistent warning bar appears at the top of the preferences page: "Preferences are not saving. Check your connection." with a "Retry" button.

### 17.8 Permission Denied

If a non-admin user attempts to access `/settings` or `/admin/users` directly:

- The page renders an empty state with a shield icon: "You do not have permission to access this page."
- A "Go to Dashboard" button navigates back to the dashboard.
- No sidebar navigation items for Settings or Users & Roles are rendered for non-admin users (as specified in Document 02, section 2.6), so this state should only occur via direct URL entry.

---

## 18. Keyboard Shortcuts

| Shortcut | Context | Action |
|----------|---------|--------|
| `Ctrl+S` | Any settings section with changes | Save the active section |
| `Escape` | Slide-over or modal open | Close the slide-over or modal |
| `Tab` / `Shift+Tab` | Permission matrix | Navigate between checkboxes |
| `Space` | Focused checkbox or toggle | Toggle the value |
| `/` | User list focused | Focus the search input |

---

## 19. API Reference Summary

### 19.1 Settings

| Method | Endpoint | Purpose |
|--------|----------|---------|
| GET | `/api/v1/settings` | Load all settings (grouped by section) |
| PUT | `/api/v1/settings` | Update specific setting keys |
| POST | `/api/v1/settings/email/test` | Send a test email |

### 19.2 Auth Providers

| Method | Endpoint | Purpose |
|--------|----------|---------|
| GET | `/api/v1/auth-providers` | List all configured providers |
| POST | `/api/v1/auth-providers` | Create a new provider |
| GET | `/api/v1/auth-providers/{id}` | Get provider details |
| PUT | `/api/v1/auth-providers/{id}` | Update provider configuration |
| DELETE | `/api/v1/auth-providers/{id}` | Delete a provider |
| PUT | `/api/v1/auth-providers/{id}/toggle` | Enable/disable provider |
| POST | `/api/v1/auth-providers/{id}/test` | Test provider connection |

### 19.3 Users

| Method | Endpoint | Purpose |
|--------|----------|---------|
| GET | `/api/v1/users` | List users (paginated, filterable) |
| POST | `/api/v1/users` | Create a new user |
| GET | `/api/v1/users/{id}` | Get user details |
| PUT | `/api/v1/users/{id}` | Update user |
| DELETE | `/api/v1/users/{id}` | Delete user |
| PUT | `/api/v1/users/{id}/lock` | Lock/unlock user account |
| POST | `/api/v1/users/{id}/reset-password` | Send password reset email |
| GET | `/api/v1/users/{id}/sessions` | List user's active sessions |
| DELETE | `/api/v1/users/{id}/sessions` | Terminate all sessions |
| DELETE | `/api/v1/users/{id}/sessions/{sid}` | Terminate specific session |
| GET | `/api/v1/users/me` | Get current user profile |
| PUT | `/api/v1/users/me` | Update current user profile |
| PUT | `/api/v1/users/me/password` | Change own password |
| GET | `/api/v1/users/me/preferences` | Get current user preferences |
| PUT | `/api/v1/users/me/preferences` | Update current user preferences |
| GET | `/api/v1/users/me/sessions` | List own sessions |
| DELETE | `/api/v1/users/me/sessions` | Terminate all own sessions except current |

### 19.4 Roles

| Method | Endpoint | Purpose |
|--------|----------|---------|
| GET | `/api/v1/roles` | List all roles |
| POST | `/api/v1/roles` | Create a custom role |
| GET | `/api/v1/roles/{id}` | Get role details with permissions |
| PUT | `/api/v1/roles/{id}` | Update custom role |
| DELETE | `/api/v1/roles/{id}` | Delete custom role |

### 19.5 API Keys

| Method | Endpoint | Purpose |
|--------|----------|---------|
| GET | `/api/v1/api-keys` | List all API keys (keys are masked) |
| POST | `/api/v1/api-keys` | Create a new API key (returns full key once) |
| DELETE | `/api/v1/api-keys/{id}` | Revoke an API key |

### 19.6 Webhooks

| Method | Endpoint | Purpose |
|--------|----------|---------|
| GET | `/api/v1/webhooks` | List all webhooks |
| POST | `/api/v1/webhooks` | Create a new webhook |
| GET | `/api/v1/webhooks/{id}` | Get webhook details |
| PUT | `/api/v1/webhooks/{id}` | Update webhook |
| DELETE | `/api/v1/webhooks/{id}` | Delete webhook |
| PUT | `/api/v1/webhooks/{id}/toggle` | Enable/disable webhook |
| GET | `/api/v1/webhooks/{id}/deliveries` | List delivery history |
| POST | `/api/v1/webhooks/{id}/deliveries/{did}/retry` | Retry a failed delivery |

### 19.7 Notifications

| Method | Endpoint | Purpose |
|--------|----------|---------|
| GET | `/api/v1/notifications` | List notifications (paginated) |
| GET | `/api/v1/notifications/unread-count` | Get unread count |
| POST | `/api/v1/notifications/read-all` | Mark all as read |
| PUT | `/api/v1/notifications/{id}/read` | Mark single as read |
