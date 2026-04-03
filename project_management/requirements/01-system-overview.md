# 01 — System Overview & Architecture

## 1. Purpose

Tackle is an enterprise phishing simulation and security testing platform built for an internal red team conducting authorized security assessments. It manages the full lifecycle of phishing campaigns — from domain registration and infrastructure provisioning through email delivery, credential capture, and post-campaign reporting.

## 2. High-Level Architecture

```
┌───────────────────────────────────────────────────────────────────────┐
│                         PRIVATE LAB (AWS)                             │
│                                                                       │
│  ┌─────────────────────────────────────────────────────────────────┐  │
│  │                   TACKLE FRAMEWORK SERVER                       │  │
│  │                                                                 │  │
│  │  ┌──────────────┐  ┌──────────────┐  ┌───────────────────────┐  │  │
│  │  │  Go Backend  │  │  PostgreSQL  │  │  Landing Page Apps    │  │  │
│  │  │  (REST API + │  │  Database    │  │  (per-campaign,       │  │  │
│  │  │   WebSocket) │  │              │  │   arbitrary ports)    │  │  │
│  │  └──────┬───────┘  └──────────────┘  └───────────┬───────────┘  │  │
│  │         │                                        │              │  │
│  │  ┌──────┴───────┐                                │              │  │
│  │  │ React SPA    │                                │              │  │
│  │  │ (Admin UI)   │                                │              │  │
│  │  └──────────────┘                                │              │  │
│  └──────────────────────────────────────────────────┼──────────────┘  │
│                                                     │                 │
└─────────────────────────────────────────────────────┼─────────────────┘
                                                      │
                    ┌─────────────────────────────────┘
                    │ (transparent proxy — no redirects)
                    ▼
    ┌────────────────────────────────────┐
    │   PHISHING ENDPOINT (AWS/Azure)    │
    │                                    │
    │  - TLS termination (443)           │
    │  - Transparent reverse proxy       │
    │    to framework landing app        │
    │  - SMTP relay to external          │
    │    SMTP servers                    │
    │  - IP attribution for emails       │
    └─────────────────┬──────────────────┘
                      │
                      ▼
    ┌────────────────────────────────────┐
    │   TARGETS (End Users)              │
    │   - Receive phishing emails        │
    │   - Interact with landing          │
    │     pages via endpoint IP          │
    └────────────────────────────────────┘
```

## 3. Component Summary

| Component | Technology | Responsibility |
|-----------|-----------|----------------|
| **Backend API** | Go (Golang) | REST API, WebSocket server, business logic, campaign orchestration, cloud provider integration, compilation engine |
| **Frontend (Admin UI)** | React (dark theme) | Framework management interface — campaigns, infrastructure, reporting, logging |
| **Database** | PostgreSQL | All persistent state — users, campaigns, targets, credentials, logs, metrics, configurations |
| **Landing Page Apps** | React + Go (generated) | Per-campaign web applications built by the landing page builder, hosted on the framework server |
| **Phishing Endpoints** | Lightweight Go binary | Transparent reverse proxy + SMTP relay, deployed to AWS EC2 / Azure VMs |
| **Cloud Integrations** | AWS SDK, Azure SDK | Instance provisioning/lifecycle, Route 53 DNS, domain management |
| **Domain Provider Integrations** | Namecheap API, GoDaddy API | Domain registration, DNS record management |
| **SMTP** | External SMTP servers | Email delivery (configured per campaign, relayed through phishing endpoints) |

## 4. Tech Stack Detail

### 4.1 Backend — Go

- Standard library + minimal, well-maintained dependencies
- All import paths must be local (e.g., `tackle/internal/...`) — no `github.com` in import paths
- RESTful API with versioned routes (`/api/v1/...`)
- WebSocket support for real-time dashboard updates
- Background workers for async operations (email sending, infrastructure provisioning, log aggregation)

### 4.2 Frontend — React

Two distinct React efforts exist in this project:

| Concern | Framework Admin UI | Campaign Landing Pages |
|---------|-------------------|----------------------|
| **Purpose** | Internal tool for red team operators | Phishing pages served to targets |
| **Users** | Red team members (authenticated) | Phishing targets (unauthenticated) |
| **Lifecycle** | Deployed once, updated with framework | Generated per-campaign, ephemeral |
| **Requirements** | Dark theme, feature-rich, WebSocket | Lightweight, unique per build, anti-fingerprint |

Detailed frontend architecture decisions are in [16-frontend-architecture.md](16-frontend-architecture.md).

### 4.3 Database — PostgreSQL

- Schema-first design with migration tooling
- Enforced referential integrity (foreign keys)
- Indexed for query patterns identified in metrics and logging requirements
- Encryption at rest for sensitive fields (credentials, API keys, SMTP passwords)
- Detailed schema in [14-database-schema.md](14-database-schema.md)

### 4.4 Authentication

- Local application accounts (always available)
- OIDC (generic provider support)
- FusionAuth
- LDAP
- All providers can be active simultaneously
- Detailed in [02-authentication-authorization.md](02-authentication-authorization.md)

## 5. Deployment Model

### 5.1 Framework Server

- Runs on an AWS instance within a private lab environment
- Network security (VPN, firewalling) is handled at the lab infrastructure level, not by the application
- The application provides configuration UI for AWS/Azure API credentials
- Single-tenant deployment (one red team)

### 5.2 Phishing Endpoints

- Provisioned dynamically by the framework via AWS EC2 and Azure VM APIs
- Framework manages full lifecycle: provision → configure → deploy → monitor → stop → terminate
- Each endpoint is associated with one campaign
- Endpoint acts as transparent reverse proxy (no HTTP redirects visible to target)
- Endpoint handles outbound SMTP to external servers (sender IP attribution)

## 6. Key Architectural Principles

1. **Security-first** — Every component assumes it may be inspected by defenders. Secrets are encrypted at rest. Audit trails are comprehensive.
2. **Clean separation** — Framework server never directly contacts targets. All target-facing traffic flows through phishing endpoints.
3. **Uniqueness** — Every campaign deployment must be distinct. Landing pages incorporate anti-fingerprinting techniques to resist signature-based detection.
4. **Comprehensive logging** — Every user action, system event, email transaction, and target interaction is logged and queryable.
5. **Approval gates** — Infrastructure-affecting operations require explicit Engineer approval before execution.
6. **Future-ready** — Data models and interfaces are designed to accommodate AI agent integration without architectural changes.

## 7. Cross-Cutting Concerns

| Concern | Approach |
|---------|----------|
| **Configuration** | Database-stored for runtime config (cloud credentials, SMTP, domain providers). Environment variables for bootstrap config (DB connection, listen address). |
| **Secrets Management** | All secrets (API keys, SMTP passwords, captured credentials) encrypted at rest in PostgreSQL using application-level encryption. Encryption key managed via environment variable. |
| **Error Handling** | Structured error types in Go. All errors logged with correlation IDs. User-facing errors are sanitized. |
| **Background Processing** | Worker pool architecture for async tasks (email sending, instance provisioning, log ingestion). |
| **Health Monitoring** | Framework monitors health of deployed phishing endpoints. Automatic alerting on endpoint failure. |

## 8. Data Retention Policies

### 8.1 Global and Per-Campaign Retention

The system SHALL support configurable data retention policies at two levels:

| Level | Scope | Configurable By |
|-------|-------|-----------------|
| **Global** | Default retention settings applied to all campaigns and system data. | Administrator |
| **Per-campaign** | Override global settings for specific campaigns. Per-campaign settings take precedence over global defaults. | Operator (at campaign creation), Administrator |

### 8.2 Retention Categories

| Data Category | Default Retention | Configurable Range | Behavior on Expiry |
|---------------|-------------------|-------------------|-------------------|
| **Campaign data** (config, metrics, approvals) | Indefinite | 90 days – indefinite | Campaign is auto-archived, then data is purged |
| **Captured credentials** | Indefinite | 30 days – indefinite | Credential values are purged; metadata (counts, timestamps) is retained |
| **Audit logs** | Indefinite | 1 year – indefinite | Logs are archived to cold storage (see [11-audit-logging.md](11-audit-logging.md)) |
| **Target interaction data** (timelines, events) | Indefinite | 90 days – indefinite | Events are purged; aggregate metrics are retained |
| **Notifications** | 90 days | 30 days – 365 days | Notifications are permanently deleted |
| **Report files** (generated PDFs, CSVs) | 365 days | 30 days – indefinite | Files are deleted; report metadata is retained |
| **Landing page builds** (compiled binaries) | Deleted on campaign teardown | N/A | Binaries are always deleted on teardown; build logs and manifests are retained |

### 8.3 Retention Enforcement

- A daily background worker identifies and processes expired data according to retention policies.
- All retention-related deletions are recorded in the audit log.
- A retention policy preview is available in admin settings showing projected storage impact.
- Changing a retention policy applies prospectively only — a confirmation dialog is required if the change would retroactively affect existing data.

---

## 9. Configuration Export / Import

### 9.1 Exportable Configuration

The system SHALL support exporting the framework's configuration as a portable ZIP archive (containing JSON files) for backup, migration, or disaster recovery.

| Exportable | Excluded |
|------------|----------|
| SMTP profiles (sans credentials) | SMTP passwords, OAuth tokens |
| Email templates (including attachments) | Captured credentials |
| Landing page project definitions | Target PII |
| Landing page and campaign templates | API keys and secrets |
| Target group structures (not member PII) | Cloud provider credentials |
| Domain configurations (sans API creds) | DKIM private keys |
| Block list entries | Webhook secrets |
| User roles and permission configs | Encryption keys |
| Alert rule configurations | Audit logs |
| Report templates | Generated reports |
| System settings | Session data |

### 9.2 Import Behavior

| Feature | Behavior |
|---------|----------|
| **Conflict resolution** | When imported entities conflict with existing ones, the user chooses: skip, overwrite, or create as new. |
| **Validation** | All entities are validated before committing. Invalid entities are reported and skipped. |
| **Selective import** | Users select which categories to import from the archive. |
| **Dry run** | A preview mode shows what would change without making modifications. |
| **Security** | Import requires Administrator role. All import operations are logged in the audit trail. |

---

## 10. Acceptance Criteria

- [ ] Architecture supports all four authentication providers simultaneously
- [ ] Framework server can host multiple landing page applications on distinct ports concurrently
- [ ] Phishing endpoints transparently proxy HTTPS traffic without exposing redirects
- [ ] All cloud infrastructure is manageable (provision through terminate) from the framework UI
- [ ] Real-time metrics are delivered via WebSocket to the admin dashboard
- [ ] The system can handle multiple concurrent campaigns without performance degradation
- [ ] All components produce structured logs that aggregate to the central database
- [ ] No import paths reference `github.com` — all paths are local
- [ ] Data retention policies are configurable at global and per-campaign levels
- [ ] Configuration export produces a credential-free archive
- [ ] Configuration import with conflict resolution works correctly on a fresh instance

## 11. Dependencies Between Components

```
Authentication (02) ──► RBAC ──► All feature modules
                                      │
Domain Management (03) ───────────────┤
SMTP Configuration (04) ──────────────┤
Landing Page Builder (05) ────────────┤
                                      ▼
                            Campaign Management (06)
                                      │
                                      ├──► Phishing Endpoints (07)
                                      ├──► Credential Capture (08)
                                      ├──► Target Management (09)
                                      │
                                      ▼
                            Metrics & Reporting (10)
                            Audit Logging (11)
                            Notification System (18)
                                      │
                                      ▼
                            AI Integration (12) [future]
```

Implementation order follows this dependency graph — see [17-implementation-roadmap.md](17-implementation-roadmap.md).
