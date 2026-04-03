# Tackle Database Schema Reference

This document describes all database tables, their relationships, and key data flows. The database is PostgreSQL 16 with 60 migrations.

---

## Connection Configuration

| Variable | Default | Description |
|----------|---------|-------------|
| `DATABASE_URL` | (required) | PostgreSQL connection string |
| `TACKLE_DB_SSLMODE` | `require` | TLS mode (`disable` for dev) |
| `TACKLE_DB_SSLROOTCERT` | — | CA cert path for TLS |
| `TACKLE_DB_MAX_OPEN_CONNS` | 25 | Max open connections |
| `TACKLE_DB_MAX_IDLE_CONNS` | 10 | Max idle connections |
| `TACKLE_DB_CONN_MAX_LIFETIME` | 30m | Connection max lifetime |

**Dev connection**: `postgres://tackle_dev:tackle_dev_password@localhost:5800/tackle_dev?sslmode=disable`

---

## PostgreSQL Extensions

- **pgcrypto** — `gen_random_uuid()` for UUID primary keys
- **pg_trgm** — trigram similarity for fuzzy text search

## Common Patterns

- **Primary keys**: UUIDs via `gen_random_uuid()`
- **Timestamps**: All tables use `TIMESTAMPTZ` with `NOW()` defaults
- **Updated timestamps**: Trigger function `set_updated_at()` auto-updates `updated_at`
- **Immutable tables**: Trigger function `reject_modification()` prevents updates (e.g., audit_logs)
- **Soft deletes**: `deleted_at` column on campaigns

---

## Table Groups

### 1. Authentication & Authorization

#### `users`
Core user accounts.

| Column | Type | Notes |
|--------|------|-------|
| id | UUID PK | |
| username | TEXT UNIQUE | |
| email | TEXT UNIQUE | |
| password_hash | TEXT | bcrypt hash |
| display_name | TEXT | |
| status | TEXT | `active`, `inactive`, `locked` |
| force_password_change | BOOLEAN | |
| last_login_at | TIMESTAMPTZ | |
| created_at / updated_at | TIMESTAMPTZ | |

#### `roles`
RBAC role definitions. Seeded with built-in roles.

| Column | Type | Notes |
|--------|------|-------|
| id | UUID PK | |
| name | TEXT UNIQUE | e.g., `admin`, `operator`, `viewer` |
| description | TEXT | |
| is_system | BOOLEAN | Cannot be deleted if true |

#### `permissions`
Permission definitions.

| Column | Type | Notes |
|--------|------|-------|
| id | UUID PK | |
| name | TEXT UNIQUE | e.g., `campaigns:create` |
| description | TEXT | |
| category | TEXT | Grouping category |

#### `role_permissions`
Maps roles to permissions (many-to-many).

#### `user_roles`
Maps users to roles (many-to-many).

#### `auth_providers`
External authentication provider configurations (LDAP, OIDC).

| Column | Type | Notes |
|--------|------|-------|
| id | UUID PK | |
| name | TEXT | |
| type | TEXT | `ldap`, `oidc` |
| config | JSONB | Provider-specific config (encrypted fields) |
| enabled | BOOLEAN | |
| default_role_id | UUID FK → roles | |

#### `auth_identities`
Links external identity provider accounts to local users.

| Column | Type | Notes |
|--------|------|-------|
| id | UUID PK | |
| user_id | UUID FK → users | |
| provider_id | UUID FK → auth_providers | |
| external_id | TEXT | ID from external provider |
| external_username | TEXT | |

#### `role_mappings`
Maps external provider groups/roles to local roles.

#### `sessions`
User session tracking.

#### `password_history`
Tracks previous password hashes to prevent reuse.

#### `api_keys`
API key management for programmatic access.

| Column | Type | Notes |
|--------|------|-------|
| id | UUID PK | |
| user_id | UUID FK → users | |
| name | TEXT | |
| key_hash | TEXT | bcrypt hash of key |
| key_prefix | TEXT | First 8 chars for identification |
| permissions | TEXT[] | Scoped permissions |
| expires_at | TIMESTAMPTZ | |
| last_used_at | TIMESTAMPTZ | |

---

### 2. Campaign Management

#### `campaigns`
Core campaign table with lifecycle state machine.

| Column | Type | Notes |
|--------|------|-------|
| id | UUID PK | |
| name | TEXT | |
| description | TEXT | |
| current_state | TEXT | See state machine below |
| created_by | UUID FK → users | |
| start_date | TIMESTAMPTZ | Scheduled start |
| end_date | TIMESTAMPTZ | Scheduled end |
| email_template_id | UUID FK → email_templates | |
| landing_page_id | UUID FK → landing_pages | |
| deleted_at | TIMESTAMPTZ | Soft delete |
| created_at / updated_at | TIMESTAMPTZ | |

**Campaign State Machine**:
```
draft → pending_approval → approved → building → ready → active → completed
                ↓                                   ↓
             rejected                             paused → active
                                                          → completed
completed → archived
```

#### `campaign_state_transitions`
Append-only log of all state changes.

| Column | Type | Notes |
|--------|------|-------|
| id | UUID PK | |
| campaign_id | UUID FK → campaigns | |
| from_state | TEXT | |
| to_state | TEXT | |
| changed_by | UUID FK → users | |
| reason | TEXT | |
| created_at | TIMESTAMPTZ | |

#### `campaign_approvals`
Approval workflow state.

| Column | Type | Notes |
|--------|------|-------|
| id | UUID PK | |
| campaign_id | UUID FK → campaigns | |
| reviewer_id | UUID FK → users | |
| status | TEXT | `pending`, `approved`, `rejected` |
| comments | TEXT | |
| reviewed_at | TIMESTAMPTZ | |

#### `campaign_shares`
Campaign sharing between users.

#### `campaign_template_variants`
A/B testing variants for campaigns.

| Column | Type | Notes |
|--------|------|-------|
| id | UUID PK | |
| campaign_id | UUID FK → campaigns | |
| email_template_id | UUID FK → email_templates | |
| weight | INTEGER | Distribution weight |
| name | TEXT | Variant label |

#### `campaign_send_windows`
Time windows when emails can be sent.

#### `campaign_config_templates`
Reusable campaign configuration templates.

#### `campaign_build_logs`
Build progress tracking during campaign building phase.

#### `campaign_emails`
Email dispatch tracking per campaign.

---

### 3. Target Management

#### `targets`
Individual phishing recipients.

| Column | Type | Notes |
|--------|------|-------|
| id | UUID PK | |
| email | TEXT | |
| first_name | TEXT | |
| last_name | TEXT | |
| department | TEXT | |
| title | TEXT | |
| phone | TEXT | |
| location | TEXT | |
| custom_fields | JSONB | Extensible metadata |
| created_at / updated_at | TIMESTAMPTZ | |

#### `target_groups`
Logical grouping of targets.

| Column | Type | Notes |
|--------|------|-------|
| id | UUID PK | |
| name | TEXT UNIQUE | |
| description | TEXT | |

#### `target_group_members`
Many-to-many: targets ↔ groups.

#### `campaign_targets`
Associates targets with campaigns and tracks per-target status.

| Column | Type | Notes |
|--------|------|-------|
| id | UUID PK | |
| campaign_id | UUID FK → campaigns | |
| target_id | UUID FK → targets | |
| status | TEXT | `pending`, `sent`, `delivered`, `opened`, `clicked`, `submitted`, `reported` |
| variant_id | UUID FK | Which A/B variant was used |
| sent_at | TIMESTAMPTZ | |
| delivered_at | TIMESTAMPTZ | |
| opened_at | TIMESTAMPTZ | |
| clicked_at | TIMESTAMPTZ | |
| submitted_at | TIMESTAMPTZ | |

#### `campaign_target_events`
Per-target event timeline (append-only).

| Column | Type | Notes |
|--------|------|-------|
| id | UUID PK | |
| campaign_target_id | UUID FK → campaign_targets | |
| event_type | TEXT | `sent`, `delivered`, `opened`, `clicked`, `submitted`, `reported` |
| metadata | JSONB | Event-specific data |
| created_at | TIMESTAMPTZ | |

#### `campaign_target_groups`
Associates target groups with campaigns.

#### `blocklist`
Email addresses/patterns blocked from campaigns.

| Column | Type | Notes |
|--------|------|-------|
| id | UUID PK | |
| pattern | TEXT | Email or wildcard pattern |
| reason | TEXT | |
| is_active | BOOLEAN | |
| created_by | UUID FK → users | |

---

### 4. Email System

#### `email_templates`
Phishing email templates.

| Column | Type | Notes |
|--------|------|-------|
| id | UUID PK | |
| name | TEXT | |
| subject | TEXT | |
| body_html | TEXT | HTML content |
| body_text | TEXT | Plaintext fallback |
| sender_name | TEXT | |
| sender_email | TEXT | |
| version | INTEGER | Versioned |
| created_by | UUID FK → users | |

#### `email_template_attachments`
File attachments for email templates.

#### `smtp_profiles`
SMTP server configurations.

| Column | Type | Notes |
|--------|------|-------|
| id | UUID PK | |
| name | TEXT | |
| host | TEXT | |
| port | INTEGER | |
| username | TEXT | |
| password_encrypted | BYTEA | AES-256-GCM encrypted |
| tls_mode | TEXT | `none`, `starttls`, `tls` |
| daily_limit | INTEGER | |

#### `campaign_smtp_profiles`
Associates SMTP profiles with campaigns (supports multiple).

#### `campaign_send_schedules`
Scheduling configuration for email delivery.

---

### 5. Landing Pages

#### `landing_pages`
Landing page project definitions.

| Column | Type | Notes |
|--------|------|-------|
| id | UUID PK | |
| name | TEXT | |
| description | TEXT | |
| page_config | JSONB | Page builder configuration |
| capture_config | JSONB | What fields to capture |
| assigned_port | INTEGER | Port for serving |
| created_by | UUID FK → users | |

#### `landing_page_builds`
Build artifacts for landing pages.

| Column | Type | Notes |
|--------|------|-------|
| id | UUID PK | |
| landing_page_id | UUID FK → landing_pages | |
| status | TEXT | `pending`, `building`, `success`, `failed` |
| build_log | TEXT | |
| created_at | TIMESTAMPTZ | |

---

### 6. Credential Capture

#### `capture_events`
Records credential submissions from landing pages.

| Column | Type | Notes |
|--------|------|-------|
| id | UUID PK | |
| campaign_id | UUID FK → campaigns | |
| campaign_target_id | UUID FK → campaign_targets | |
| landing_page_id | UUID FK → landing_pages | |
| submitted_data | BYTEA | AES-256-GCM encrypted JSON |
| ip_address | TEXT | |
| user_agent | TEXT | |
| created_at | TIMESTAMPTZ | |

#### `tracking_tokens`
Email tracking tokens for open/click tracking.

| Column | Type | Notes |
|--------|------|-------|
| id | UUID PK | |
| campaign_target_id | UUID FK → campaign_targets | |
| token | TEXT UNIQUE | HMAC-derived deterministic token |

---

### 7. Domain & Infrastructure

#### `domain_profiles`
Domain configuration for phishing infrastructure.

| Column | Type | Notes |
|--------|------|-------|
| id | UUID PK | |
| name | TEXT | Domain name |
| provider_id | UUID FK → domain_provider_connections | |
| status | TEXT | |
| registration_date | DATE | |
| expiry_date | DATE | |

#### `domain_provider_connections`
Connections to domain registrar APIs (GoDaddy, Namecheap, Route53, Azure DNS).

| Column | Type | Notes |
|--------|------|-------|
| id | UUID PK | |
| name | TEXT | |
| provider_type | TEXT | `godaddy`, `namecheap`, `route53`, `azure_dns` |
| credentials_encrypted | BYTEA | AES-256-GCM encrypted |
| is_active | BOOLEAN | |

#### `dns_records`
DNS records managed for domains.

| Column | Type | Notes |
|--------|------|-------|
| id | UUID PK | |
| domain_id | UUID FK → domain_profiles | |
| record_type | TEXT | `A`, `CNAME`, `TXT`, `MX`, etc. |
| name | TEXT | |
| value | TEXT | |
| ttl | INTEGER | |

#### `domain_health_checks`
Health monitoring results for domains.

#### `domain_categorizations`
Domain categorization status tracking.

#### `domain_renewal_history`
Domain renewal event log.

#### `domain_campaign_associations`
Links domains to campaigns.

#### `domain_registration_requests`
Domain registration workflow (request → approve → register).

---

### 8. Cloud & Endpoint Infrastructure

#### `cloud_credentials`
Cloud provider credentials for VM provisioning.

| Column | Type | Notes |
|--------|------|-------|
| id | UUID PK | |
| name | TEXT | |
| provider | TEXT | `aws`, `azure` |
| credentials_encrypted | BYTEA | AES-256-GCM encrypted |

#### `instance_templates`
Templates for provisioning cloud VMs.

#### `phishing_endpoints`
Active phishing endpoint infrastructure.

| Column | Type | Notes |
|--------|------|-------|
| id | UUID PK | |
| campaign_id | UUID FK → campaigns | |
| cloud_credential_id | UUID FK → cloud_credentials | |
| instance_template_id | UUID FK → instance_templates | |
| landing_page_id | UUID FK → landing_pages | |
| status | TEXT | State machine managed |
| ip_address | TEXT | |
| domain | TEXT | |
| port | INTEGER | |
| tls_enabled | BOOLEAN | |

---

### 9. Audit & System

#### `audit_logs`
Immutable audit trail with HMAC chain integrity.

| Column | Type | Notes |
|--------|------|-------|
| id | UUID PK | |
| action | TEXT | e.g., `campaign.created`, `user.login` |
| actor_id | UUID FK → users | |
| resource_type | TEXT | |
| resource_id | UUID | |
| details | JSONB | Sanitized event details |
| hmac | TEXT | HMAC-SHA256 of entry |
| previous_hmac | TEXT | Chain link to prior entry |
| ip_address | TEXT | |
| created_at | TIMESTAMPTZ | |

**Immutability enforced** by `reject_modification()` trigger — no UPDATE or DELETE allowed.

#### `alert_rules`
Configurable alert rules evaluated against audit events.

#### `notifications`
Notification queue for in-app and email notifications.

| Column | Type | Notes |
|--------|------|-------|
| id | UUID PK | |
| user_id | UUID FK → users | |
| type | TEXT | |
| title | TEXT | |
| body | TEXT | |
| is_read | BOOLEAN | |
| created_at | TIMESTAMPTZ | |
| expires_at | TIMESTAMPTZ | Cleaned up by background worker |

#### `user_preferences`
Per-user preference storage (notification settings, UI preferences).

#### `system_config`
Global system configuration key-value store.

---

## Entity Relationship Overview

```
users ──< user_roles >── roles ──< role_permissions >── permissions
  │
  ├──< campaigns
  │       │
  │       ├──< campaign_targets ──> targets ──< target_group_members >── target_groups
  │       │       │
  │       │       ├──< campaign_target_events
  │       │       └──< capture_events
  │       │
  │       ├──< campaign_state_transitions
  │       ├──< campaign_approvals
  │       ├──< campaign_template_variants ──> email_templates
  │       ├──< campaign_smtp_profiles ──> smtp_profiles
  │       ├──< campaign_send_windows
  │       ├──< campaign_shares
  │       ├──> email_templates ──< email_template_attachments
  │       ├──> landing_pages ──< landing_page_builds
  │       └──< phishing_endpoints ──> cloud_credentials
  │                                ──> instance_templates
  │
  ├──< audit_logs (HMAC chain)
  ├──< notifications
  ├──< api_keys
  └──< auth_identities ──> auth_providers ──< role_mappings

domain_profiles ──< dns_records
               ──< domain_health_checks
               ──< domain_categorizations
               ──> domain_provider_connections
```

---

## Encryption at Rest

The following columns store AES-256-GCM encrypted data:

| Table | Column | Content |
|-------|--------|---------|
| smtp_profiles | password_encrypted | SMTP passwords |
| cloud_credentials | credentials_encrypted | AWS/Azure secrets |
| domain_provider_connections | credentials_encrypted | Registrar API keys |
| capture_events | submitted_data | Captured credentials |

All encryption uses a master key (`TACKLE_ENCRYPTION_KEY`) with HKDF-SHA256 domain separation to derive independent subkeys per purpose.

---

## Seed Data

Development seed data is in `scripts/seed.sql`:
- Admin user + 4 test users (sarah.chen, mike.ross, jen.martinez, tom.baker)
- 25 target records across departments
- 5 target groups
- Campaign templates, SMTP profiles, landing pages, endpoints
- All passwords are bcrypt-hashed
