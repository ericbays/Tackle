# Tackle API Reference

Complete list of all API endpoints the frontend can consume. All routes are under `/api/v1`.

---

## Authentication

All authenticated endpoints require `Authorization: Bearer <JWT>` header unless noted otherwise.

### Auth Endpoints

| Method | Path | Auth | Description |
|--------|------|------|-------------|
| POST | `/auth/login` | None | Login with username/password |
| POST | `/auth/refresh` | Refresh token | Refresh access token |
| POST | `/auth/logout` | JWT | Logout and blacklist token |
| GET | `/auth/me` | JWT | Get current user profile |
| GET | `/auth/providers` | None | List enabled auth providers |
| GET | `/auth/oidc/{providerID}/login` | None | Initiate OIDC login |
| GET | `/auth/oidc/callback/{providerID}` | None | OIDC callback |

### Account Linking

| Method | Path | Auth | Description |
|--------|------|------|-------------|
| GET | `/auth/identities` | JWT | List linked identities |
| POST | `/auth/link/{providerID}` | JWT | Initiate account linking |
| GET | `/auth/link/callback/{providerID}` | JWT | Link callback |
| DELETE | `/auth/identities/{identityID}` | JWT | Unlink identity |

---

## Setup

| Method | Path | Auth | Description |
|--------|------|------|-------------|
| GET | `/setup/status` | None | Check if initial setup is complete |
| POST | `/setup` | None | Complete initial setup (first-run only) |

---

## Users

### User Management (Admin)

| Method | Path | Permission | Description |
|--------|------|-----------|-------------|
| GET | `/users` | `users:read` | List all users |
| POST | `/users` | `users:create` | Create user |
| GET | `/users/{id}` | `users:read` | Get user details |
| PUT | `/users/{id}` | `users:update` | Update user |
| DELETE | `/users/{id}` | `users:delete` | Delete user |
| GET | `/users/{id}/activity` | `users:read` | Get user activity |
| GET | `/users/{id}/sessions` | `users:read` | List user sessions |
| DELETE | `/users/{id}/sessions/{sid}` | `users:update` | Terminate session |
| PUT | `/users/{id}/password` | `users:update` | Admin password reset |
| PUT | `/users/{id}/roles` | `users:update` | Assign roles |

### Current User (Self-service)

| Method | Path | Auth | Description |
|--------|------|------|-------------|
| GET | `/users/me/sessions` | JWT | List own sessions |
| PUT | `/users/me/password` | JWT | Change own password |
| DELETE | `/users/me/sessions/{id}` | JWT | Delete own session |
| PUT | `/users/me/profile` | JWT | Update own profile |
| GET | `/users/me/preferences` | JWT | Get preferences |
| PUT | `/users/me/preferences` | JWT | Update preferences |

---

## Roles & Permissions

| Method | Path | Permission | Description |
|--------|------|-----------|-------------|
| GET | `/roles` | `roles:read` | List roles |
| POST | `/roles` | `roles:create` | Create role |
| GET | `/roles/{id}` | `roles:read` | Get role |
| PUT | `/roles/{id}` | `roles:update` | Update role |
| DELETE | `/roles/{id}` | `roles:delete` | Delete role |
| GET | `/roles/{id}/users` | `roles:read` | Get users in role |
| GET | `/permissions` | `roles:read` | List all permissions |

---

## API Keys

| Method | Path | Permission | Description |
|--------|------|-----------|-------------|
| GET | `/api-keys` | JWT | List own API keys |
| POST | `/api-keys` | JWT | Create API key |
| DELETE | `/api-keys/{id}` | JWT | Revoke API key |

---

## Campaigns

### CRUD

| Method | Path | Permission | Description |
|--------|------|-----------|-------------|
| GET | `/campaigns` | `campaigns:read` | List campaigns |
| POST | `/campaigns` | `campaigns:create` | Create campaign |
| GET | `/campaigns/{id}` | `campaigns:read` | Get campaign |
| PUT | `/campaigns/{id}` | `campaigns:update` | Update campaign |
| DELETE | `/campaigns/{id}` | `campaigns:delete` | Delete campaign |
| POST | `/campaigns/{id}/clone` | `campaigns:create` | Clone campaign |

### State Transitions

| Method | Path | Permission | Description |
|--------|------|-----------|-------------|
| POST | `/campaigns/{id}/submit` | `campaigns:update` | Submit for approval |
| POST | `/campaigns/{id}/build` | `campaigns:update` | Start building |
| POST | `/campaigns/{id}/launch` | `campaigns:update` | Launch campaign |
| POST | `/campaigns/{id}/pause` | `campaigns:update` | Pause campaign |
| POST | `/campaigns/{id}/resume` | `campaigns:update` | Resume campaign |
| POST | `/campaigns/{id}/complete` | `campaigns:update` | Complete campaign |
| POST | `/campaigns/{id}/archive` | `campaigns:update` | Archive campaign |
| POST | `/campaigns/{id}/unlock` | `campaigns:approve` | Unlock from approval |

### Approval Workflow

| Method | Path | Permission | Description |
|--------|------|-----------|-------------|
| POST | `/campaigns/{id}/approve` | `campaigns:approve` | Approve campaign |
| POST | `/campaigns/{id}/reject` | `campaigns:approve` | Reject campaign |
| GET | `/campaigns/{id}/approval-review` | `campaigns:read` | Get approval review |
| GET | `/campaigns/{id}/approval-history` | `campaigns:read` | Get approval history |

### Campaign Configuration

| Method | Path | Permission | Description |
|--------|------|-----------|-------------|
| GET | `/campaigns/{id}/template-variants` | `campaigns:read` | Get A/B variants |
| PUT | `/campaigns/{id}/template-variants` | `campaigns:update` | Set A/B variants |
| GET | `/campaigns/{id}/send-windows` | `campaigns:read` | Get send windows |
| PUT | `/campaigns/{id}/send-windows` | `campaigns:update` | Set send windows |

### Campaign SMTP Profiles

| Method | Path | Permission | Description |
|--------|------|-----------|-------------|
| GET | `/campaigns/{id}/smtp-profiles` | `campaigns:read` | List assigned profiles |
| POST | `/campaigns/{id}/smtp-profiles` | `campaigns:update` | Assign SMTP profile |
| PUT | `/campaigns/{id}/smtp-profiles/{assocId}` | `campaigns:update` | Update assignment |
| DELETE | `/campaigns/{id}/smtp-profiles/{assocId}` | `campaigns:update` | Remove assignment |
| GET | `/campaigns/{id}/send-schedule` | `campaigns:read` | Get send schedule |
| PUT | `/campaigns/{id}/send-schedule` | `campaigns:update` | Set send schedule |
| POST | `/campaigns/{id}/smtp-profiles/validate` | `campaigns:update` | Validate profiles |

### Campaign Target Groups

| Method | Path | Permission | Description |
|--------|------|-----------|-------------|
| GET | `/campaigns/{id}/target-groups` | `campaigns:read` | List assigned groups |
| POST | `/campaigns/{id}/target-groups` | `campaigns:update` | Assign group |
| DELETE | `/campaigns/{id}/target-groups/{groupId}` | `campaigns:update` | Unassign group |
| GET | `/campaigns/{id}/resolve-targets` | `campaigns:read` | Resolve all targets |

### Campaign Blocklist

| Method | Path | Permission | Description |
|--------|------|-----------|-------------|
| GET | `/campaigns/{id}/blocklist-review` | `campaigns:read` | Review blocklist hits |
| POST | `/campaigns/{id}/blocklist-override` | `campaigns:approve` | Override blocklist |

### Campaign Canary Targets

| Method | Path | Permission | Description |
|--------|------|-----------|-------------|
| GET | `/campaigns/{id}/canary-targets` | `campaigns:read` | List canary targets |
| POST | `/campaigns/{id}/canary-targets` | `campaigns:update` | Designate canaries |
| DELETE | `/campaigns/{id}/canary-targets` | `campaigns:update` | Remove canaries |

### Campaign Metrics & Reporting

| Method | Path | Permission | Description |
|--------|------|-----------|-------------|
| GET | `/campaigns/{id}/metrics` | `campaigns:read` | Campaign metrics |
| GET | `/campaigns/{id}/metrics/timeline` | `campaigns:read` | Metrics timeline |
| GET | `/campaigns/{id}/build-log` | `campaigns:read` | Build log |
| GET | `/campaigns/{id}/variant-comparison` | `campaigns:read` | A/B comparison |
| GET | `/campaigns/{id}/delivery-status` | `campaigns:read` | Delivery status |
| GET | `/campaigns/{id}/captures` | `credentials:read` | Campaign captures |
| GET | `/campaigns/{id}/capture-metrics` | `campaigns:read` | Capture metrics |

### Campaign Endpoints (Infrastructure)

| Method | Path | Permission | Description |
|--------|------|-----------|-------------|
| GET | `/campaigns/{id}/endpoint` | `campaigns:read` | Get endpoint |
| GET | `/campaigns/{id}/endpoint/health` | `campaigns:read` | Endpoint health |
| GET | `/campaigns/{id}/endpoint/logs` | `campaigns:read` | Endpoint logs |
| POST | `/campaigns/{id}/endpoint/stop` | `campaigns:update` | Stop endpoint |
| POST | `/campaigns/{id}/endpoint/restart` | `campaigns:update` | Restart endpoint |
| DELETE | `/campaigns/{id}/endpoint` | `campaigns:update` | Terminate endpoint |
| POST | `/campaigns/{id}/endpoint/retry` | `campaigns:update` | Retry provisioning |
| POST | `/campaigns/{id}/endpoint/redeploy` | `campaigns:update` | Redeploy |
| POST | `/campaigns/{id}/endpoint/tls` | `campaigns:update` | Upload TLS cert |
| POST | `/campaigns/{id}/phishing-reports` | `campaigns:update` | Manual report |

### Campaign Templates

| Method | Path | Permission | Description |
|--------|------|-----------|-------------|
| GET | `/campaign-templates` | `campaigns:read` | List config templates |
| POST | `/campaign-templates` | `campaigns:create` | Create template |
| GET | `/campaign-templates/{id}` | `campaigns:read` | Get template |
| PUT | `/campaign-templates/{id}` | `campaigns:update` | Update template |
| DELETE | `/campaign-templates/{id}` | `campaigns:delete` | Delete template |
| POST | `/campaign-templates/{id}/apply` | `campaigns:update` | Apply to campaign |

---

## Targets

### Target CRUD

| Method | Path | Permission | Description |
|--------|------|-----------|-------------|
| GET | `/targets` | `targets:read` | List targets (paginated) |
| POST | `/targets` | `targets:create` | Create target |
| GET | `/targets/{id}` | `targets:read` | Get target |
| PUT | `/targets/{id}` | `targets:update` | Update target |
| DELETE | `/targets/{id}` | `targets:delete` | Delete target |
| POST | `/targets/{id}/restore` | `targets:update` | Restore deleted |
| GET | `/targets/{id}/history` | `targets:read` | Change history |
| GET | `/targets/{id}/events` | `targets:read` | Target events |
| GET | `/targets/{id}/stats` | `targets:read` | Target statistics |
| GET | `/targets/check-email` | `targets:read` | Check email exists |
| GET | `/targets/departments` | `targets:read` | List departments |

### Target Import (CSV)

| Method | Path | Permission | Description |
|--------|------|-----------|-------------|
| POST | `/targets/import/upload` | `targets:create` | Upload CSV (multipart) |
| GET | `/targets/import/{upload_id}/preview` | `targets:create` | Preview import |
| POST | `/targets/import/{upload_id}/mapping` | `targets:create` | Set field mapping |
| POST | `/targets/import/{upload_id}/validate` | `targets:create` | Validate import |
| POST | `/targets/import/{upload_id}/commit` | `targets:create` | Execute import |

### Import Mapping Templates

| Method | Path | Permission | Description |
|--------|------|-----------|-------------|
| GET | `/targets/import/mapping-templates` | `targets:read` | List templates |
| POST | `/targets/import/mapping-templates` | `targets:create` | Create template |
| PUT | `/targets/import/mapping-templates/{id}` | `targets:update` | Update template |
| DELETE | `/targets/import/mapping-templates/{id}` | `targets:delete` | Delete template |

### Bulk Operations (5 MB body limit)

| Method | Path | Permission | Description |
|--------|------|-----------|-------------|
| POST | `/targets/bulk/delete` | `targets:delete` | Bulk delete |
| POST | `/targets/bulk/edit` | `targets:update` | Bulk edit |
| POST | `/targets/bulk/export` | `targets:read` | Bulk export |
| POST | `/targets/bulk/add-to-group` | `targets:update` | Add to group |
| POST | `/targets/bulk/remove-from-group` | `targets:update` | Remove from group |

---

## Target Groups

| Method | Path | Permission | Description |
|--------|------|-----------|-------------|
| GET | `/target-groups` | `targets:read` | List groups |
| POST | `/target-groups` | `targets:create` | Create group |
| GET | `/target-groups/{id}` | `targets:read` | Get group |
| PUT | `/target-groups/{id}` | `targets:update` | Update group |
| DELETE | `/target-groups/{id}` | `targets:delete` | Delete group |
| POST | `/target-groups/{id}/members` | `targets:update` | Add members (5 MB) |
| DELETE | `/target-groups/{id}/members` | `targets:update` | Remove members |

---

## Blocklist

| Method | Path | Permission | Description |
|--------|------|-----------|-------------|
| GET | `/blocklist` | `targets:read` | List entries |
| POST | `/blocklist` | `targets:create` | Create entry |
| GET | `/blocklist/{id}` | `targets:read` | Get entry |
| PUT | `/blocklist/{id}/deactivate` | `targets:update` | Deactivate |
| PUT | `/blocklist/{id}/reactivate` | `targets:update` | Reactivate |
| GET | `/blocklist/check` | `targets:read` | Check email |
| GET | `/blocklist-overrides` | `targets:read` | List overrides |
| POST | `/blocklist-overrides/{id}/decide` | `targets:update` | Decide override |

---

## Email Templates

| Method | Path | Permission | Description |
|--------|------|-----------|-------------|
| GET | `/email-templates` | `templates:read` | List templates |
| POST | `/email-templates` | `templates:create` | Create template |
| GET | `/email-templates/{id}` | `templates:read` | Get template |
| PUT | `/email-templates/{id}` | `templates:update` | Update template |
| DELETE | `/email-templates/{id}` | `templates:delete` | Delete template |
| POST | `/email-templates/{id}/clone` | `templates:create` | Clone template |
| POST | `/email-templates/{id}/preview` | `templates:read` | Preview |
| POST | `/email-templates/{id}/validate` | `templates:read` | Validate |
| GET | `/email-templates/{id}/versions` | `templates:read` | List versions |
| GET | `/email-templates/{id}/versions/{v}` | `templates:read` | Get version |
| GET | `/email-templates/{id}/export` | `templates:read` | Export |
| POST | `/email-templates/{id}/send-test` | `templates:update` | Send test email |

### Email Template Attachments

| Method | Path | Permission | Description |
|--------|------|-----------|-------------|
| GET | `/email-templates/{id}/attachments` | `templates:read` | List attachments |
| POST | `/email-templates/{id}/attachments` | `templates:update` | Upload (12 MB multipart) |
| DELETE | `/email-templates/{id}/attachments/{aid}` | `templates:update` | Delete attachment |

---

## SMTP Profiles

| Method | Path | Permission | Description |
|--------|------|-----------|-------------|
| GET | `/smtp-profiles` | `smtp:read` | List profiles |
| POST | `/smtp-profiles` | `smtp:create` | Create profile |
| GET | `/smtp-profiles/{id}` | `smtp:read` | Get profile |
| PUT | `/smtp-profiles/{id}` | `smtp:update` | Update profile |
| DELETE | `/smtp-profiles/{id}` | `smtp:delete` | Delete profile |
| POST | `/smtp-profiles/{id}/test` | `smtp:update` | Test connection |
| POST | `/smtp-profiles/{id}/duplicate` | `smtp:create` | Duplicate profile |

---

## Landing Pages

### CRUD

| Method | Path | Permission | Description |
|--------|------|-----------|-------------|
| GET | `/landing-pages` | `landing_pages:read` | List projects |
| POST | `/landing-pages` | `landing_pages:create` | Create project |
| GET | `/landing-pages/{id}` | `landing_pages:read` | Get project |
| PUT | `/landing-pages/{id}` | `landing_pages:update` | Update project |
| DELETE | `/landing-pages/{id}` | `landing_pages:delete` | Delete project |
| POST | `/landing-pages/{id}/duplicate` | `landing_pages:create` | Duplicate |
| POST | `/landing-pages/{id}/preview` | `landing_pages:read` | Preview |
| POST | `/landing-pages/{id}/import` | `landing_pages:update` | Import HTML (5 MB) |
| POST | `/landing-pages/{id}/import-zip` | `landing_pages:update` | Import ZIP |
| POST | `/landing-pages/{id}/clone-url` | `landing_pages:create` | Clone from URL |

### Templates & Components

| Method | Path | Permission | Description |
|--------|------|-----------|-------------|
| GET | `/landing-pages/templates` | `landing_pages:read` | List templates |
| POST | `/landing-pages/templates` | `landing_pages:create` | Save as template |
| PUT | `/landing-pages/templates/{id}` | `landing_pages:update` | Update template |
| DELETE | `/landing-pages/templates/{id}` | `landing_pages:delete` | Delete template |
| GET | `/landing-pages/components` | `landing_pages:read` | List components |
| GET | `/landing-pages/themes` | `landing_pages:read` | List themes |
| GET | `/landing-pages/js-snippets` | `landing_pages:read` | List JS snippets |
| GET | `/landing-pages/starter-templates` | `landing_pages:read` | List starters |

### Builds

| Method | Path | Permission | Description |
|--------|------|-----------|-------------|
| POST | `/landing-pages/{id}/build` | `landing_pages:update` | Trigger build |
| GET | `/landing-pages/{id}/builds` | `landing_pages:read` | List builds |
| GET | `/landing-pages/{id}/builds/{buildId}` | `landing_pages:read` | Get build |
| POST | `/landing-pages/{id}/builds/{buildId}/start` | `landing_pages:update` | Start app |
| POST | `/landing-pages/{id}/builds/{buildId}/stop` | `landing_pages:update` | Stop app |
| GET | `/landing-pages/{id}/builds/{buildId}/health` | `landing_pages:read` | App health |

### Field Categories (Credential Capture)

| Method | Path | Permission | Description |
|--------|------|-----------|-------------|
| GET | `/landing-pages/{id}/field-categories` | `landing_pages:read` | Get rules |
| POST | `/landing-pages/{id}/field-categories` | `landing_pages:update` | Create rule |
| DELETE | `/field-categories/{id}` | `landing_pages:update` | Delete rule |

---

## Credentials / Captures

| Method | Path | Permission | Description |
|--------|------|-----------|-------------|
| GET | `/captures` | `credentials:read` | List captures |
| GET | `/captures/export` | `credentials:read` | Export captures |
| GET | `/captures/{id}` | `credentials:read` | Get capture |
| POST | `/captures/{id}/reveal` | `credentials:reveal` | Decrypt credentials |
| DELETE | `/captures/{id}` | `credentials:delete` | Delete capture |
| POST | `/captures/{id}/associate` | `credentials:update` | Associate with target |
| POST | `/captures/purge` | `credentials:delete` | Purge old captures |

---

## Domains

### Domain CRUD

| Method | Path | Permission | Description |
|--------|------|-----------|-------------|
| GET | `/domains` | `domains:read` | List domains |
| POST | `/domains` | `domains:create` | Create domain |
| GET | `/domains/{id}` | `domains:read` | Get domain |
| PUT | `/domains/{id}` | `domains:update` | Update domain |
| DELETE | `/domains/{id}` | `domains:delete` | Delete domain |
| POST | `/domains/check-availability` | `domains:read` | Check availability |
| POST | `/domains/{id}/renew` | `domains:update` | Renew domain |
| GET | `/domains/{id}/renewal-history` | `domains:read` | Renewal history |

### Domain Registration

| Method | Path | Permission | Description |
|--------|------|-----------|-------------|
| POST | `/domains/registration-requests` | `domains:create` | Submit request |
| POST | `/domains/registration-requests/{id}/approve` | `domains:approve` | Approve |
| POST | `/domains/registration-requests/{id}/reject` | `domains:approve` | Reject |

### DNS Records

| Method | Path | Permission | Description |
|--------|------|-----------|-------------|
| GET | `/domains/{id}/dns-records` | `domains:read` | List DNS records |
| POST | `/domains/{id}/dns-records` | `domains:update` | Create record |
| PUT | `/domains/{id}/dns-records/{rid}` | `domains:update` | Update record |
| DELETE | `/domains/{id}/dns-records/{rid}` | `domains:update` | Delete record |
| GET | `/domains/{id}/dns-records/soa` | `domains:read` | Get SOA record |

### Email Authentication

| Method | Path | Permission | Description |
|--------|------|-----------|-------------|
| GET | `/domains/{id}/email-auth` | `domains:read` | Get email auth config |
| POST | `/domains/{id}/email-auth/spf` | `domains:update` | Configure SPF |
| POST | `/domains/{id}/email-auth/dkim` | `domains:update` | Generate DKIM |
| POST | `/domains/{id}/email-auth/dmarc` | `domains:update` | Configure DMARC |
| POST | `/domains/{id}/email-auth/validate` | `domains:read` | Validate config |
| GET | `/domains/{id}/propagation-checks` | `domains:read` | DNS propagation |

### Domain Health

| Method | Path | Permission | Description |
|--------|------|-----------|-------------|
| POST | `/domains/{id}/health-check` | `domains:read` | Trigger check |
| GET | `/domains/{id}/health-checks` | `domains:read` | List checks |
| GET | `/domains/{id}/health-checks/latest` | `domains:read` | Latest check |
| GET | `/domains/{id}/categorization` | `domains:read` | Categorization |
| GET | `/domains/{id}/categorization/history` | `domains:read` | Category history |
| POST | `/domains/{id}/categorization/check` | `domains:update` | Trigger check |

---

## Domain Providers (Settings)

| Method | Path | Permission | Description |
|--------|------|-----------|-------------|
| GET | `/settings/domain-providers` | `settings:read` | List providers |
| POST | `/settings/domain-providers` | `settings:update` | Create provider |
| GET | `/settings/domain-providers/{id}` | `settings:read` | Get provider |
| PUT | `/settings/domain-providers/{id}` | `settings:update` | Update provider |
| DELETE | `/settings/domain-providers/{id}` | `settings:update` | Delete provider |
| POST | `/settings/domain-providers/{id}/test` | `settings:read` | Test connection |

---

## Cloud Credentials (Settings)

| Method | Path | Permission | Description |
|--------|------|-----------|-------------|
| GET | `/settings/cloud-credentials` | `settings:read` | List credentials |
| POST | `/settings/cloud-credentials` | `settings:update` | Create credential |
| GET | `/settings/cloud-credentials/{id}` | `settings:read` | Get credential |
| PUT | `/settings/cloud-credentials/{id}` | `settings:update` | Update credential |
| DELETE | `/settings/cloud-credentials/{id}` | `settings:update` | Delete credential |
| POST | `/settings/cloud-credentials/{id}/test` | `settings:read` | Test credential |

---

## Instance Templates

| Method | Path | Permission | Description |
|--------|------|-----------|-------------|
| GET | `/instance-templates` | `infrastructure:read` | List templates |
| POST | `/instance-templates` | `infrastructure:create` | Create template |
| GET | `/instance-templates/{id}` | `infrastructure:read` | Get template |
| PUT | `/instance-templates/{id}` | `infrastructure:update` | Update template |
| DELETE | `/instance-templates/{id}` | `infrastructure:delete` | Delete template |
| GET | `/instance-templates/{id}/versions` | `infrastructure:read` | List versions |
| GET | `/instance-templates/{id}/versions/{v}` | `infrastructure:read` | Get version |
| POST | `/instance-templates/validate` | `infrastructure:read` | Validate template |

---

## Endpoints (Infrastructure)

| Method | Path | Permission | Description |
|--------|------|-----------|-------------|
| GET | `/endpoints` | `infrastructure:read` | List all endpoints |
| POST | `/endpoints/provision` | `infrastructure:create` | Provision endpoint |

---

## Auth Providers (Settings)

| Method | Path | Permission | Description |
|--------|------|-----------|-------------|
| GET | `/settings/auth-providers` | `settings:read` | List providers |
| POST | `/settings/auth-providers` | `settings:update` | Create provider |
| GET | `/settings/auth-providers/{id}` | `settings:read` | Get provider |
| PUT | `/settings/auth-providers/{id}` | `settings:update` | Update provider |
| DELETE | `/settings/auth-providers/{id}` | `settings:update` | Delete provider |
| POST | `/settings/auth-providers/{id}/test` | `settings:read` | Test connection |
| POST | `/settings/auth-providers/{id}/enable` | `settings:update` | Enable |
| POST | `/settings/auth-providers/{id}/disable` | `settings:update` | Disable |
| GET | `/settings/auth-providers/{id}/role-mappings` | `settings:read` | Get mappings |
| PUT | `/settings/auth-providers/{id}/role-mappings` | `settings:update` | Set mappings |

---

## Settings

| Method | Path | Permission | Description |
|--------|------|-----------|-------------|
| GET | `/settings` | `settings:read` | Get all settings |
| PUT | `/settings` | `settings:update` | Update settings |

---

## Metrics & Reports

| Method | Path | Permission | Description |
|--------|------|-----------|-------------|
| GET | `/metrics/organization` | `metrics:read` | Org-wide metrics |
| GET | `/metrics/departments` | `metrics:read` | Department metrics |
| GET | `/metrics/trends` | `metrics:read` | Trends data |
| GET | `/report-templates` | `reports:read` | List report templates |
| POST | `/report-templates` | `reports:create` | Create template |
| POST | `/reports/generate` | `reports:create` | Generate report |
| GET | `/reports` | `reports:read` | List reports |
| GET | `/reports/{id}` | `reports:read` | Get report |
| GET | `/reports/{id}/download` | `reports:read` | Download report |
| DELETE | `/reports/{id}` | `reports:delete` | Delete report |

---

## Audit Logs

| Method | Path | Permission | Description |
|--------|------|-----------|-------------|
| GET | `/logs/audit` | `audit:read` | List audit logs |
| GET | `/logs/audit/{id}` | `audit:read` | Get entry |
| POST | `/logs/audit/{id}/verify` | `audit:read` | Verify HMAC integrity |

---

## Alert Rules

| Method | Path | Permission | Description |
|--------|------|-----------|-------------|
| GET | `/alert-rules` | `audit:read` | List alert rules |
| POST | `/alert-rules` | `audit:update` | Create rule |
| PUT | `/alert-rules/{id}` | `audit:update` | Update rule |
| DELETE | `/alert-rules/{id}` | `audit:update` | Delete rule |

---

## Notifications

### REST

| Method | Path | Auth | Description |
|--------|------|------|-------------|
| GET | `/notifications` | JWT | List notifications |
| PUT | `/notifications/{id}/read` | JWT | Mark read |
| POST | `/notifications/read-all` | JWT | Mark all read |
| DELETE | `/notifications/{id}` | JWT | Delete |
| POST | `/notifications/delete-read` | JWT | Delete all read |
| POST | `/notifications/delete-selected` | JWT | Delete selected |
| GET | `/notifications/unread-count` | JWT | Unread count |
| GET | `/notifications/preferences` | JWT | Get preferences |
| PUT | `/notifications/preferences` | JWT | Update preferences |

### Admin

| Method | Path | Permission | Description |
|--------|------|-----------|-------------|
| GET | `/notifications/smtp-config` | `settings:read` | Get SMTP config |
| PUT | `/notifications/smtp-config` | `settings:update` | Update SMTP config |

### Webhooks

| Method | Path | Permission | Description |
|--------|------|-----------|-------------|
| GET | `/webhooks` | `settings:read` | List webhooks |
| POST | `/webhooks` | `settings:update` | Create webhook |
| DELETE | `/webhooks/{id}` | `settings:update` | Delete webhook |
| PUT | `/webhooks/{id}/toggle` | `settings:update` | Toggle webhook |
| GET | `/webhooks/{id}/deliveries` | `settings:read` | Webhook deliveries |

### WebSocket

| Method | Path | Auth | Description |
|--------|------|------|-------------|
| GET | `/ws` | JWT (via message) | WebSocket connection |

**WebSocket Protocol**:
1. Connect to `/api/v1/ws`
2. Send: `{"type":"auth","token":"<JWT>"}`
3. Receive: `{"type":"auth_ok"}`
4. Receive pushed notification events

---

## Search

| Method | Path | Permission | Description |
|--------|------|-----------|-------------|
| GET | `/search` | JWT | Cross-entity search |

---

## Tools

| Method | Path | Permission | Description |
|--------|------|-----------|-------------|
| POST | `/tools/typosquat` | `domains:read` | Generate typosquats |
| POST | `/tools/typosquat/register` | `domains:create` | Register typosquats |

---

## AI Integration (Stubs)

| Method | Path | Permission | Description |
|--------|------|-----------|-------------|
| GET | `/ai/proposals` | JWT | List proposals |
| GET | `/ai/proposals/{id}` | JWT | Get proposal |
| POST | `/ai/proposals/{id}/review` | JWT | Review proposal |
| DELETE | `/ai/proposals/{id}` | JWT | Delete proposal |
| POST | `/ai/generate/email-template` | JWT | Generate email |
| POST | `/ai/generate/subject-lines` | JWT | Generate subjects |
| POST | `/ai/generate/landing-page-content` | JWT | Generate LP content |
| POST | `/ai/generate/personalization` | JWT | Generate personalization |
| POST | `/ai/research/target-org` | JWT | Research org |
| POST | `/ai/research/industry-templates` | JWT | Industry research |
| GET | `/ai/research/{id}` | JWT | Get research |
| GET | `/ai/research` | JWT | List research |

---

## Internal API (Endpoint ↔ Server)

These are used by deployed phishing endpoints to communicate back to the server. Not for frontend use.

| Method | Path | Auth | Description |
|--------|------|------|-------------|
| POST | `/internal/captures` | Build token | Credential capture events |
| POST | `/internal/tracking` | Build token | Tracking events |
| POST | `/internal/telemetry` | Build token | Telemetry |
| POST | `/internal/delivery-result` | Build token | Email delivery results |
| POST | `/internal/session-captures` | Build token | Session captures |
| POST | `/endpoint-data/heartbeat` | Endpoint auth | Health heartbeat |
| POST | `/endpoint-data/logs` | Endpoint auth | Request logs |
| POST | `/webhooks/phishing-reports` | Endpoint auth | Phishing reports |

---

## Health Check

| Method | Path | Auth | Description |
|--------|------|------|-------------|
| GET | `/health` | None | Server health check |
