# 17 — Implementation Roadmap

## 1. Purpose

This document defines the phased implementation plan for the Tackle platform. It sequences all development work across seven phases, identifies dependencies between items, calls out parallelization opportunities, and assigns relative complexity ratings to guide planning. Time estimates are deliberately omitted — this is a sequencing and dependency document, not a schedule.

Items tagged with **[CLINE]** involve offensive security techniques and are delegated to the local LLM via Cline prompts. These items require red team lead review before integration.

---

## 2. Phased Implementation Plan

### Phase 1: Foundation

**Objective:** Establish a working application skeleton with authentication, authorization, user management, and audit logging. At the end of this phase, an administrator can log in, manage users, and all actions are audited.

**Complexity: High**

| ID | Requirement | Description | Complexity | Dependencies | Notes |
|----|-------------|-------------|------------|--------------|-------|
| REQ-ROAD-001 | Project scaffolding | Initialize Go backend project structure (`tackle/internal/...`, local import paths per [01-system-overview.md](01-system-overview.md)), React frontend project (dark theme shell), PostgreSQL database container, Docker Compose for local development, CI pipeline skeleton. | Medium | None | All subsequent work depends on this. No `github.com` import paths. |
| REQ-ROAD-002 | Database schema and migration system | Implement schema-first migration tooling. Define initial tables: `users`, `roles`, `role_permissions`, `auth_identities`, `refresh_tokens`, `password_history`, `api_keys`, `auth_provider_configs`, `role_mappings`, `audit_log`. See [14-database-schema.md](14-database-schema.md). | Medium | REQ-ROAD-001 | Migration system must support forward-only versioned migrations. |
| REQ-ROAD-003 | Local account authentication | Implement local account login, registration, password hashing (bcrypt cost 12+), JWT access tokens, refresh token rotation, session management. Covers REQ-AUTH-010 through REQ-AUTH-014, REQ-AUTH-070 through REQ-AUTH-073. See [02-authentication-authorization.md](02-authentication-authorization.md). | High | REQ-ROAD-002 | Local auth must be fully functional before adding external providers. |
| REQ-ROAD-004 | Initial admin setup flow | Implement first-launch detection, setup endpoint (`POST /api/v1/setup`), permanent deactivation after first use, initial administrator immutability. Covers REQ-AUTH-001 through REQ-AUTH-003. | Medium | REQ-ROAD-003 | Setup wizard in frontend must block all other navigation until complete. |
| REQ-ROAD-005 | RBAC system with built-in roles | Implement permission model (`resource:action`), four built-in roles (Administrator, Engineer, Operator, Defender), permission middleware on all API routes, frontend permission-based rendering. Covers REQ-RBAC-001 through REQ-RBAC-013, REQ-RBAC-030 through REQ-RBAC-032. | High | REQ-ROAD-003 | Custom roles (REQ-RBAC-020 through REQ-RBAC-023) included here. |
| REQ-ROAD-006 | User management | Full CRUD for user accounts, role assignment, account locking/unlocking, password reset, session management UI. Covers REQ-AUTH-011, REQ-AUTH-014, REQ-AUTH-026. | Medium | REQ-ROAD-005 | Admin-only and self-service operations. |
| REQ-ROAD-007 | OIDC authentication provider | Generic OIDC Authorization Code Flow, provider configuration CRUD, user provisioning and account linking. Covers REQ-AUTH-030 through REQ-AUTH-032, REQ-AUTH-060, REQ-AUTH-061. | High | REQ-ROAD-003 | Can be parallelized with REQ-ROAD-008 and REQ-ROAD-009. |
| REQ-ROAD-008 | FusionAuth authentication provider | FusionAuth-specific OAuth2/OIDC flow, group-to-role mapping, configuration validation. Covers REQ-AUTH-040 through REQ-AUTH-042. | Medium | REQ-ROAD-003 | Can be parallelized with REQ-ROAD-007 and REQ-ROAD-009. |
| REQ-ROAD-009 | LDAP authentication provider | LDAP bind authentication, user search, attribute mapping, group-to-role mapping, connection pooling, StartTLS support. Covers REQ-AUTH-050 through REQ-AUTH-052. | High | REQ-ROAD-003 | Can be parallelized with REQ-ROAD-007 and REQ-ROAD-008. |
| REQ-ROAD-010 | Password policy and security controls | Password complexity enforcement, password history, login rate limiting (per-IP and per-account), account lockout. Covers REQ-AUTH-020 through REQ-AUTH-028. | Medium | REQ-ROAD-003 | Breached password list (10K+ entries) must be embedded. |
| REQ-ROAD-011 | API key authentication | API key generation, hashing, `X-API-Key` header authentication, RBAC enforcement for API key requests. Covers REQ-AUTH-080. | Low | REQ-ROAD-005 | |
| REQ-ROAD-012 | Basic audit logging | Audit log table, structured event recording for all auth events, user management events, and configuration changes. Log query API with filtering. See [11-audit-logging.md](11-audit-logging.md). | Medium | REQ-ROAD-002 | Audit logging is consumed by every subsequent phase. |
| REQ-ROAD-013 | Core API framework with middleware | Implement middleware chain: authentication, RBAC, request logging, correlation IDs, error handling, rate limiting. Versioned routes (`/api/v1/...`). Structured error responses. | Medium | REQ-ROAD-003, REQ-ROAD-005 | Every API endpoint uses this middleware stack. |
| REQ-ROAD-014 | `.clinerules` file | Create the `.clinerules` configuration file defining Cline delegation boundaries, code style rules, and project conventions for the local LLM. | Low | REQ-ROAD-001 | Must be in place before any [CLINE] tasks are executed. |

#### Phase 1 Gate

The following MUST be complete before proceeding to Phase 2:

- [ ] A fresh installation presents the setup wizard; no other endpoints are accessible
- [ ] The initial administrator can log in via local auth and has unrestricted access
- [ ] All four built-in roles enforce their documented permission sets on every API endpoint
- [ ] At least one external auth provider (OIDC, FusionAuth, or LDAP) is functional alongside local auth
- [ ] All authentication and user management events are recorded in the audit log
- [ ] The API middleware chain (auth, RBAC, logging, error handling) is applied to all protected routes
- [ ] The `.clinerules` file is committed and validated

#### Phase 1 Parallelization

```
REQ-ROAD-001 (scaffolding)
    │
    ├──► REQ-ROAD-002 (DB schema) ──► REQ-ROAD-003 (local auth) ──┐
    │                                                             │
    └──► REQ-ROAD-014 (.clinerules)                               │
                                                                  │
    ┌─────────────────────────────────────────────────────────────┘
    │
    ├──► REQ-ROAD-004 (setup flow)
    ├──► REQ-ROAD-005 (RBAC) ──► REQ-ROAD-006 (user mgmt) ──► REQ-ROAD-011 (API keys)
    ├──► REQ-ROAD-010 (password policy)
    ├──► REQ-ROAD-012 (audit logging)
    ├──► REQ-ROAD-013 (API middleware)
    │
    │   (these three are parallelizable with each other)
    ├──► REQ-ROAD-007 (OIDC)
    ├──► REQ-ROAD-008 (FusionAuth)
    └──► REQ-ROAD-009 (LDAP)
```

---

### Phase 2: Infrastructure

**Objective:** Enable the platform to manage external cloud providers, domains, DNS records, email authentication, and SMTP configurations. At the end of this phase, an operator can register domains, configure DNS, set up SMTP, and provision cloud instances — but cannot yet run campaigns.

**Complexity: High**

| ID | Requirement | Description | Complexity | Dependencies | Notes |
|----|-------------|-------------|------------|--------------|-------|
| REQ-ROAD-020 | Cloud provider credential management | CRUD for AWS and Azure credential sets, encrypted storage, "Test Connection" validation, RBAC enforcement (Admin/Engineer). Covers REQ-INFRA-001 through REQ-INFRA-005. See [03-domain-infrastructure.md](03-domain-infrastructure.md). | Medium | Phase 1 complete | |
| REQ-ROAD-021 | Domain provider integrations | API integrations for Namecheap, GoDaddy, Route 53, and Azure DNS. Provider connection CRUD, "Test Connection" action, multiple connections per provider type. Covers REQ-DOM-001 through REQ-DOM-005. | High | Phase 1 complete | Four distinct API integrations. Can parallelize provider implementations. |
| REQ-ROAD-022 | Domain registration and management | Domain registration workflow (availability check, approval gate, registration execution), domain profiles, domain inventory UI, import existing domains, soft-delete lifecycle. Covers REQ-DOM-006 through REQ-DOM-019. | High | REQ-ROAD-021 | |
| REQ-ROAD-023 | DNS record management | Full CRUD for A, AAAA, CNAME, MX, TXT, NS records across all four DNS providers. Tabular UI with inline editing, propagation checking, input validation. Covers REQ-DOM-020 through REQ-DOM-024. | High | REQ-ROAD-021 | |
| REQ-ROAD-024 | DKIM/SPF/DMARC record management | Email Authentication panel with builder UIs for SPF (authorized sources, lookup limit warnings), DKIM (key pair generation, selector management), DMARC (policy builder). Automated validation after publishing. Covers REQ-DOM-025 through REQ-DOM-030 and REQ-EMAIL-021 through REQ-EMAIL-030. See [04-email-smtp.md](04-email-smtp.md). | High | REQ-ROAD-023 | Shared between domain module (03) and email module (04). |
| REQ-ROAD-025 | Domain health checking | Automated health checks (DNS propagation, blocklist status, SPF/DKIM/DMARC validity, MX resolution), scheduled checks, health history, notifications for blocklist detection. Covers REQ-DOM-031 through REQ-DOM-035. | Medium | REQ-ROAD-023, REQ-ROAD-024 | |
| REQ-ROAD-026 | SMTP configuration management | SMTP server profile CRUD, connection testing, encrypted credential storage, duplication, deletion protection for active campaigns, sending strategy configuration. Covers REQ-SMTP-001 through REQ-SMTP-012. | Medium | Phase 1 complete | Can be parallelized with domain work. |
| REQ-ROAD-027 | Instance templates | Instance template CRUD with versioning for both AWS and Azure. Template fields: region, instance size, OS image, security groups, SSH keys, user data, tags. Validation against cloud provider. Covers REQ-INFRA-006 through REQ-INFRA-009. | Medium | REQ-ROAD-020 | |
| REQ-ROAD-028 | Instance provisioning and lifecycle management | Full instance lifecycle state machine (provisioning through terminated), approval-gated provisioning, SSH-based configuration deployment, lifecycle actions (provision, stop, start, terminate, redeploy), partial failure handling. Covers REQ-INFRA-010 through REQ-INFRA-016. | Very High | REQ-ROAD-020, REQ-ROAD-027 | Most complex item in Phase 2. Two cloud providers with different APIs. |
| REQ-ROAD-029 | Infrastructure health monitoring | Periodic health checks for running instances (reachability, endpoint process, TLS cert, SMTP relay), health history, consecutive failure alerting, real-time infrastructure dashboard via WebSocket. Covers REQ-INFRA-017 through REQ-INFRA-021. | High | REQ-ROAD-028 | |
| REQ-ROAD-031 | Domain categorization checking | Asynchronous categorization checks against security vendor services, categorization history tracking, trend monitoring. Covers REQ-INFRA-027 through REQ-INFRA-029. See [03-domain-infrastructure.md](03-domain-infrastructure.md). | Medium | REQ-ROAD-022 | |
| REQ-ROAD-032 | Typosquat domain generator | Typosquat generation algorithms (homoglyph, transposition, insertion, omission, hyphenation, TLD swap), bulk availability checking via domain provider API, one-click registration. Covers REQ-INFRA-030 through REQ-INFRA-032. | Medium | REQ-ROAD-021 | |

#### Phase 2 Gate

The following MUST be complete before proceeding to Phase 3:

- [ ] AWS and Azure credential sets can be created, tested, and used for provisioning
- [ ] Domains can be registered (Namecheap/GoDaddy) and DNS records managed (all four providers)
- [ ] SPF, DKIM, and DMARC records can be generated, published, and validated
- [ ] SMTP profiles can be created, tested, and configured with sending strategies
- [ ] Cloud instances can be provisioned, configured, and terminated through their full lifecycle
- [ ] Health monitoring is active for running instances with alerting on failures
- [ ] All infrastructure operations are recorded in the audit log

#### Phase 2 Parallelization

```
Phase 1 complete
    │
    ├──► REQ-ROAD-020 (cloud creds) ──► REQ-ROAD-027 (templates) ──► REQ-ROAD-028 (lifecycle) ──► REQ-ROAD-029 (health)
    │
    ├──► REQ-ROAD-021 (domain providers) ──► REQ-ROAD-022 (registration) ──► REQ-ROAD-031 (categorization)
    │                                  └──► REQ-ROAD-023 (DNS mgmt) ──► REQ-ROAD-024 (DKIM/SPF/DMARC) ──► REQ-ROAD-025 (health)
    │                                  └──► REQ-ROAD-032 (typosquat generator)
    │
    └──► REQ-ROAD-026 (SMTP config)    [parallel with domain and cloud tracks]
```

Three independent tracks (cloud, domain, SMTP) can proceed in parallel.

---

### Phase 3: Campaign Core

**Objective:** Build the campaign management foundation — targets, email templates, landing page builder, compilation engine, campaign configuration, state machine, and approval workflow. At the end of this phase, an operator can design a complete campaign but cannot yet execute it.

**Complexity: Very High**

| ID | Requirement | Description | Complexity | Dependencies | Notes |
|----|-------------|-------------|------------|--------------|-------|
| REQ-ROAD-040 | Target management | Target CRUD, CSV import with field mapping and validation, target groups, block list, search and filtering, PII handling, bulk operations. See [09-target-management.md](09-target-management.md). | Medium | Phase 1 complete | Can begin as soon as Phase 1 is complete (no Phase 2 dependency). |
| REQ-ROAD-041 | Email template system | Template CRUD, variable substitution engine (`{{token}}` syntax), HTML and plaintext body support, custom headers, attachment handling (MIME types, size limits), A/B test weight configuration, inline preview with sample data. Covers REQ-EMAIL-001 through REQ-EMAIL-014. See [04-email-smtp.md](04-email-smtp.md). | High | Phase 1 complete | Can begin as soon as Phase 1 is complete. |
| REQ-ROAD-042 | Landing page builder — visual editor | Drag-and-drop canvas (WYSIWYG), component library (all categories from REQ-LPB-007), component properties panel, component tree view, undo/redo (50 levels), grid/freeform layout modes, snap and alignment guides. Covers REQ-LPB-001 through REQ-LPB-009. See [05-landing-page-builder.md](05-landing-page-builder.md). | Very High | Phase 1 complete | Most complex frontend feature in the entire project. |
| REQ-ROAD-043 | Landing page builder — form builder | Dedicated form builder mode, field configuration with capture tags, form submission behavior, multi-step forms with progression logic, hidden fields with static/dynamic values. Covers REQ-LPB-010 through REQ-LPB-014. | High | REQ-ROAD-042 | Extends the visual editor with form-specific capabilities. |
| REQ-ROAD-044 | Landing page builder — multi-page support | Multi-page projects (10+ pages), page management panel, navigation flow configuration (redirect, click, form submit, conditional), starter templates (Login->Loading->Success, Login->MFA->Dashboard, etc.), page metadata. Covers REQ-LPB-015 through REQ-LPB-019. | High | REQ-ROAD-042 | |
| REQ-ROAD-045 | Landing page builder — styling and themes | CSS/styling editor (visual controls + raw CSS), theme presets, built-in enterprise themes (Microsoft, Google, corporate, cloud, banking), responsive design controls (desktop/tablet/mobile). Covers REQ-LPB-020 through REQ-LPB-024. | Medium | REQ-ROAD-042 | |
| REQ-ROAD-046 | Landing page builder — JavaScript injection | Project/page/component-level JS injection, event bindings (onClick, onSubmit, etc.), JS snippet templates for phishing behaviors, syntax-highlighted code editor. Covers REQ-LPB-025 through REQ-LPB-028. | Medium | REQ-ROAD-042 | Snippet templates include keylogger, clipboard capture, fingerprint collection. |
| REQ-ROAD-047 | Landing page builder — preview mode | In-builder preview rendering (iframe), viewport switching (desktop/tablet/mobile), multi-page navigation in preview, "PREVIEW MODE" banner. Covers REQ-LPB-029 through REQ-LPB-032. | Medium | REQ-ROAD-042 through REQ-ROAD-046 | Depends on all builder sub-features being at least partially implemented. |
| REQ-ROAD-048 | Landing page compilation engine | JSON-to-binary compilation pipeline, React frontend generation, Go backend generation, `embed` package for static assets, credential capture endpoint generation, tracking pixel endpoint generation, build manifest and logging. Covers REQ-LPB-042 through REQ-LPB-049. | Very High | REQ-ROAD-042, REQ-ROAD-043 | Core compilation without anti-fingerprinting (that is Phase 4). |
| REQ-ROAD-049 | Landing page hosting and lifecycle | Host compiled apps on random ports, concurrent app support (configurable limit), lifecycle management (built->running->stopped->cleaned_up), health checks, automatic restart, resource cleanup, port exposure to phishing endpoint config. Covers REQ-LPB-050 through REQ-LPB-060. | High | REQ-ROAD-048 | |
| REQ-ROAD-050 | Campaign creation and configuration | Campaign CRUD, association with targets/groups, email template assignment (with A/B weights), landing page assignment, SMTP profile assignment (with strategy selection), phishing endpoint assignment, domain assignment, sending schedule and rate limit configuration. See [06-campaign-management.md](06-campaign-management.md). | High | REQ-ROAD-040, REQ-ROAD-041, REQ-ROAD-048, Phase 2 complete | Ties together all previous subsystems. |
| REQ-ROAD-051 | Campaign state machine and lifecycle | Campaign states (draft, ready, pending_approval, approved, running, paused, stopping, stopped, completed, archived), state transition rules, pre-transition validation (SMTP reachability, DNS records, endpoint health), campaign ownership and sharing. | High | REQ-ROAD-050 | |
| REQ-ROAD-052 | Approval workflow | Infrastructure request queue, Engineer approval/rejection flow, Operator request submission, approval notifications, audit logging for all approval actions. Covers REQ-RBAC-002 (Engineer approval role), REQ-INFRA-011. | Medium | REQ-ROAD-051 | Operators cannot approve their own requests. |
| REQ-ROAD-053 | Campaign cloning and templates | Selective campaign cloning (choose components to copy), campaign template CRUD, apply template to create new campaign. Covers REQ-CAMP-035 through REQ-CAMP-036. See [06-campaign-management.md](06-campaign-management.md). | Medium | REQ-ROAD-050 | |
| REQ-ROAD-054 | Email template WYSIWYG editor | Rich text editor with formatting toolbar, bidirectional HTML/code sync, variable insertion picker, inline preview. Covers REQ-EMAIL-034. See [04-email-smtp.md](04-email-smtp.md). | High | REQ-ROAD-041 | Extends the basic template system with visual editing. |
| REQ-ROAD-055 | Built-in email template library | 20+ pre-built templates across 5 categories (credential harvesting, document lure, IT notification, HR/corporate, social engineering), difficulty ratings, copy-to-templates workflow. Covers REQ-EMAIL-037, REQ-EMAIL-038. | Low | REQ-ROAD-041 | Template content is static seed data. |
| REQ-ROAD-056 | Landing page HTML import and cloning | HTML import (file/ZIP/paste) with builder and raw modes, raw HTML code editor with bidirectional sync, visual page cloning from URL with asset localization. Covers REQ-LPB-061 through REQ-LPB-071. See [05-landing-page-builder.md](05-landing-page-builder.md). | High | REQ-ROAD-042 | Alternative path into the builder (import vs. build from scratch). |
| REQ-ROAD-057 | Campaign scheduling and send order | Scheduled auto-launch with `scheduled_launch_at` field, custom send order options (default, alphabetical, department, custom, randomized). Covers REQ-CAMP-037, REQ-CAMP-038. | Medium | REQ-ROAD-050 | |
| REQ-ROAD-058 | Parallel approval workflow | Configurable approver count (1-5), parallel approval (all must approve), any rejection voids all approvals, approver notification integration. Covers REQ-CAMP-039. | Medium | REQ-ROAD-052 | Extends basic approval workflow. |

#### Phase 3 Gate

The following MUST be complete before proceeding to Phase 4:

- [ ] Targets can be created, imported via CSV, grouped, and filtered
- [ ] Email templates support variable substitution, A/B weights, attachments, and inline preview
- [ ] The landing page builder produces a functional JSON page definition via drag-and-drop
- [ ] The compilation engine generates a working React+Go binary from a page definition
- [ ] Compiled landing page apps run on the framework server and respond to HTTP requests
- [ ] A campaign can be fully configured (targets, templates, landing page, SMTP, endpoint, domain, schedule)
- [ ] The campaign state machine enforces all documented transition rules
- [ ] The approval workflow routes infrastructure requests from Operators to Engineers

#### Phase 3 Parallelization

```
Phase 1 complete (Phase 2 can proceed in parallel for infrastructure items)
    │
    ├──► REQ-ROAD-040 (targets)            ─────────────────────┐
    ├──► REQ-ROAD-041 (email templates) ──────────────────────┐ │
    │                                  └──► REQ-ROAD-054 ──┐  │ │
    │                                  └──► REQ-ROAD-055   │  │ │
    │                                       (template lib) │  │ │
    │                                       (WYSIWYG)      │  │ │
    │                                                      │  │ │
    └──► REQ-ROAD-042 (visual editor) ──┐                  │  │ │
         ├──► REQ-ROAD-043 (forms)      │                  │  │ │
         ├──► REQ-ROAD-044 (multi-page) ├──►  REQ-ROAD-047 │  │ │
         ├──► REQ-ROAD-045 (styling)    │     (preview)    │  │ │
         ├──► REQ-ROAD-046 (JS inject)  │                  │  │ │
         └──► REQ-ROAD-056 (import/clone)                  │  │ │
                                         │                 │  │ │
                                         └──► REQ-ROAD-048 ┘  │ │
                                              (compilation)    │ │
                                                   │           │ │
                                              REQ-ROAD-049     │ │
                                              (hosting)        │ │
                                                   │           │ │
                                                   └─────┬─────┘ │
                                                         │       │
                                                    REQ-ROAD-050 (campaign config) [+ Phase 2 complete]
                                                         │
                                                    ├──► REQ-ROAD-053 (cloning/templates)
                                                    ├──► REQ-ROAD-057 (scheduling/send order)
                                                    │
                                                    REQ-ROAD-051 (state machine)
                                                         │
                                                    REQ-ROAD-052 (approval workflow)
                                                         │
                                                    REQ-ROAD-058 (parallel approval)
```

Targets (REQ-ROAD-040) and email templates (REQ-ROAD-041) have no dependency on the landing page builder track and can proceed fully in parallel. The landing page builder sub-features (REQ-ROAD-043 through REQ-ROAD-046) can be developed in parallel after the core editor (REQ-ROAD-042) is established. REQ-ROAD-054 (WYSIWYG editor) and REQ-ROAD-055 (template library) extend the email template system. REQ-ROAD-056 (import/cloning) extends the visual editor. REQ-ROAD-053, REQ-ROAD-057, and REQ-ROAD-058 extend campaign configuration and approval workflows respectively.

---

### Phase 4: Campaign Execution

**Objective:** Build the systems that make campaigns actually run — phishing endpoint deployment, SMTP relay, email sending, credential capture, interaction tracking, and operational security measures. This is where offensive capabilities come online.

**Complexity: Very High**

| ID | Requirement | Description | Complexity | Dependencies | Notes |
|----|-------------|-------------|------------|--------------|-------|
| REQ-ROAD-060 | Phishing endpoint binary — transparent reverse proxy | **[CLINE]** Lightweight Go binary deployed to EC2/Azure VMs. TLS termination on port 443, transparent reverse proxy to framework server landing page app (no HTTP redirects visible to target), management health endpoint. See [07-phishing-endpoints.md](07-phishing-endpoints.md). | Very High | REQ-ROAD-028, REQ-ROAD-049 | Core of the target-facing infrastructure. Must be indistinguishable from a normal web server. |
| REQ-ROAD-061 | SMTP relay through phishing endpoints | **[CLINE]** SMTP relay module in the phishing endpoint binary. Receives campaign payloads from framework, opens connections to external SMTP servers, sends emails with phishing endpoint IP as source, reports delivery status back to framework. See [04-email-smtp.md](04-email-smtp.md), Section 2. | Very High | REQ-ROAD-060, REQ-ROAD-026 | Emails must originate from endpoint IP for sender attribution. |
| REQ-ROAD-062 | Email sending engine | Campaign email orchestration: target queue management, sending schedule enforcement (time windows, day-of-week), rate limiting (campaign-level and per-SMTP-profile), inter-message delay randomization (CSPRNG), batch processing with pauses, send/pause/resume controls, retry logic with exponential backoff. Covers REQ-SMTP-013 through REQ-SMTP-020. | High | REQ-ROAD-061, REQ-ROAD-051 | Framework orchestrates; endpoint executes. |
| REQ-ROAD-063 | Credential capture system | **[CLINE]** Form interception in landing page apps (JS-level form hooking, universal field capture), transparent submission interception, data transmission to framework API, target identification via tracking tokens, request metadata capture, submission sequence tracking, post-capture actions (redirect, display page, replay). Covers REQ-CRED-001 through REQ-CRED-006, REQ-CRED-017, REQ-CRED-018. See [08-credential-capture.md](08-credential-capture.md). | Very High | REQ-ROAD-048 | REQ-CRED-018 (credential replay) is advanced/high-risk. |
| REQ-ROAD-064 | Credential storage and access control | Encryption at rest (AES-256-GCM), RBAC-restricted access, credential masking by default, explicit reveal workflow with audit logging, immutability on campaign archive, data retention policies, purge workflow. Covers REQ-CRED-007 through REQ-CRED-011, REQ-CRED-016. | High | REQ-ROAD-063 | Framework-side processing of captured credentials. |
| REQ-ROAD-065 | Tracking system | Email open tracking (tracking pixel), link click tracking, landing page view tracking, form submission tracking, per-target interaction timeline, real-time event forwarding to framework via internal API. Covers REQ-EMAIL-020, REQ-LPB-049, REQ-LPB-057 through REQ-LPB-060. | High | REQ-ROAD-060, REQ-ROAD-048 | Tracking tokens must be opaque and non-reversible without DB lookup. |
| REQ-ROAD-066 | Email header sanitization for OpSec | **[CLINE]** Sanitize outbound email headers to remove any indicators of the framework server's identity, internal IPs, or tooling signatures. Custom HELO/EHLO hostnames, `Message-ID` domain alignment, removal of framework-specific headers, `Received` header chain management. See [04-email-smtp.md](04-email-smtp.md), REQ-EMAIL-015 through REQ-EMAIL-019. | High | REQ-ROAD-061 | OpSec-critical. Headers are the first thing defenders analyze. |
| REQ-ROAD-067 | Anti-fingerprinting engine for landing pages | **[CLINE]** Full anti-fingerprinting pipeline integrated into the compilation engine: HTML DOM randomization, CSS class name randomization, asset path randomization, decoy content injection, HTTP response header randomization, multiple code generation strategies (SPA, multi-file, hybrid). Covers REQ-LPB-033 through REQ-LPB-041. See [05-landing-page-builder.md](05-landing-page-builder.md), Section 4. | Very High | REQ-ROAD-048 | Six distinct Cline tasks (AF-1 through AF-6) defined in 05-landing-page-builder.md. |
| REQ-ROAD-068 | Email authentication validation | Pre-launch validation of SPF, DKIM, and DMARC records against campaign configuration. Automatic re-validation on endpoint IP change. Warning and override workflow. Covers REQ-EMAIL-031 through REQ-EMAIL-033. | Medium | REQ-ROAD-024, REQ-ROAD-060 | |
| REQ-ROAD-069 | Session capture system | **[CLINE]** Full session capture — cookies via document.cookie and Set-Cookie interception, OAuth tokens from URL/form/response, session tokens from localStorage/sessionStorage, auth headers. Per-landing-page toggle. Covers REQ-CRED-021 through REQ-CRED-023. See [08-credential-capture.md](08-credential-capture.md). | High | REQ-ROAD-063 | Extends credential capture with session/token capture. |
| REQ-ROAD-069A | URL redirect chains | Configurable redirect chains (up to 5 hops) between phishing link click and landing page, configurable status codes (301/302/303/307), framework-managed and external redirects, tracking parameter forwarding. Covers REQ-EMAIL-039. See [04-email-smtp.md](04-email-smtp.md). | Medium | REQ-ROAD-060, REQ-ROAD-065 | |
| REQ-ROAD-069B | TLS certificate upload for endpoints | Manual PEM certificate upload form, domain/chain/expiry validation, encrypted storage, certificate replacement without endpoint restart. Covers REQ-PHEP-023. See [07-phishing-endpoints.md](07-phishing-endpoints.md). | Low | REQ-ROAD-060 | Alternative to automated Let's Encrypt provisioning. |

#### Phase 4 Gate

The following MUST be complete before proceeding to Phase 5:

- [ ] Phishing endpoints transparently proxy HTTPS traffic to framework landing pages without exposing redirects
- [ ] Emails are sent through phishing endpoints with correct sender IP attribution
- [ ] The sending engine respects all rate limits, schedules, and sending strategies
- [ ] Credential capture works for arbitrary form fields across all modern browsers
- [ ] Captured credentials are encrypted at rest with RBAC-enforced access
- [ ] Tracking captures email opens, link clicks, page views, and form submissions
- [ ] Email headers contain no indicators of framework identity
- [ ] Two builds of the same landing page definition produce structurally unique output that defeats signature matching
- [ ] A complete campaign can be configured, approved, launched, and captures credentials from test targets

#### Phase 4 Parallelization

```
Phase 3 complete
    │
    ├──► REQ-ROAD-060 (proxy binary) [CLINE] ──► REQ-ROAD-061 (SMTP relay) [CLINE] ──► REQ-ROAD-062 (sending engine)
    │                                        └──► REQ-ROAD-066 (header sanitization) [CLINE]
    │                                        └──► REQ-ROAD-065 (tracking) ──► REQ-ROAD-069A (redirect chains)
    │                                        └──► REQ-ROAD-068 (email auth validation)
    │                                        └──► REQ-ROAD-069B (TLS cert upload)
    │
    ├──► REQ-ROAD-063 (credential capture) [CLINE] ──► REQ-ROAD-064 (credential storage)
    │                                              └──► REQ-ROAD-069 (session capture) [CLINE]
    │
    └──► REQ-ROAD-067 (anti-fingerprinting) [CLINE]  [parallel — depends only on compilation engine]
```

The credential capture track (REQ-ROAD-063/064) and the anti-fingerprinting track (REQ-ROAD-067) can proceed independently of the phishing endpoint track (REQ-ROAD-060/061/062). REQ-ROAD-069 (session capture) extends credential capture. REQ-ROAD-069A (redirect chains) depends on the proxy binary and tracking system. REQ-ROAD-069B (TLS cert upload) depends only on the proxy binary. End-to-end integration testing requires all tracks to converge.

---

### Phase 5: Monitoring & Reporting

**Objective:** Build the operator-facing monitoring, metrics, and reporting systems. At the end of this phase, operators have full visibility into campaign execution and can generate deliverable reports.

**Complexity: High**

| ID | Requirement | Description | Complexity | Dependencies | Notes |
|----|-------------|-------------|------------|--------------|-------|
| REQ-ROAD-070 | Real-time WebSocket dashboard | WebSocket server for live campaign updates (capture events, email delivery status, tracking events, endpoint health), operator session management, multi-campaign overview, per-campaign drill-down. | High | Phase 4 complete | Foundation WebSocket plumbing may exist from Phase 2 (REQ-ROAD-029). Expand here. |
| REQ-ROAD-071 | Campaign metrics aggregation | Compute and expose: capture rate, open rate, click rate, per-variant metrics, time-to-capture, delivery success/failure rates, funnel analysis (sent->opened->clicked->submitted), metric time series. Covers REQ-EMAIL-011, REQ-CRED-014. See [10-metrics-reporting.md](10-metrics-reporting.md). | High | Phase 4 complete | Metrics must update within 10 seconds of new events. |
| REQ-ROAD-072 | Report template system | Configurable report templates defining sections, data sources, chart types, access control scoping (summary vs. detailed vs. redacted). | Medium | REQ-ROAD-071 | |
| REQ-ROAD-073 | Report generation | PDF generation with charts and tables, CSV export for raw data, report scheduling, redacted export option (no credential values), audit logging for all report generation and export actions. Covers REQ-CRED-015. | High | REQ-ROAD-072 | PDF generation with embedded charts is non-trivial. |
| REQ-ROAD-074 | Comprehensive logging UI | Unified logging interface for audit logs, campaign activity logs, and system event logs. Filtering by date range, user, event type, campaign, severity. Full-text search. Real-time log streaming via WebSocket. Log export. See [11-audit-logging.md](11-audit-logging.md). | High | REQ-ROAD-012 (extends basic audit logging from Phase 1) | |
| REQ-ROAD-075 | Infrastructure health monitoring dashboard | Consolidated view of all phishing endpoints with live health status, campaign association, uptime, region, provider. Alert management UI. Historical health trends. Extends REQ-ROAD-029. | Medium | REQ-ROAD-029, REQ-ROAD-070 | Builds on Phase 2 health monitoring; adds UI polish and WebSocket streaming. |
| REQ-ROAD-076 | Email client analytics | User-Agent detection from tracking pixel requests, email client/platform/device classification, image proxy service detection, per-campaign and per-variant aggregation. Covers REQ-EMAIL-035, REQ-EMAIL-036, REQ-MET-036 through REQ-MET-038. | Medium | REQ-ROAD-065, REQ-ROAD-071 | Extends tracking and metrics systems. |
| REQ-ROAD-077 | Dedicated Defender Dashboard | Organizational susceptibility score, trend lines, department risk heatmap, campaign comparison, phishing report rate tracking, date range filtering. Covers REQ-MET-041 through REQ-MET-043, REQ-FE-062. See [10-metrics-reporting.md](10-metrics-reporting.md). | High | REQ-ROAD-071 | Dedicated view for Defender role. |
| REQ-ROAD-078 | Campaign auto-summary | Lightweight auto-summary generated within 60 seconds of campaign completion, on-demand comprehensive summary via standard report pipeline. Covers REQ-CAMP-040, REQ-MET-039, REQ-MET-040. | Medium | REQ-ROAD-071, REQ-ROAD-073 | |
| REQ-ROAD-079 | Phishing report tracking | Phishing report webhook receiver (`POST /api/v1/webhooks/phishing-reports`), Message-ID matching, configurable auth, manual flagging, report metrics in Defender Dashboard. Covers REQ-PHEP-024. See [07-phishing-endpoints.md](07-phishing-endpoints.md). | Medium | REQ-ROAD-071 | |
| REQ-ROAD-079A | Campaign dry run mode | Dry run simulation without real emails or infrastructure, validation checks (target count, email rendering, SMTP connectivity, DNS, landing page build), estimated timeline. Covers REQ-CAMP-041. | Medium | REQ-ROAD-050 | Could also be Phase 3, but simulation requires Phase 4 subsystems for validation. |
| REQ-ROAD-079B | Campaign calendar view | Calendar UI component (month/week/day views), campaign date-range bars, state color-coding, overlap visualization, click-to-create. Covers REQ-CAMP-043, REQ-FE-061. | Medium | REQ-ROAD-070 | Frontend feature building on dashboard. |
| REQ-ROAD-079C | Canary targets in campaign execution | Priority sending for canary targets (sent first), auto-verification on interaction, metric exclusion, canary status panel on dashboard. Covers REQ-CAMP-042, REQ-TGT-028. | Medium | REQ-ROAD-062, REQ-ROAD-065 | |
| REQ-ROAD-079D | Notification system | Multi-channel notification delivery: in-app (WebSocket persistent inbox), email (opt-in with digest mode), webhooks (HMAC/bearer/basic auth, retry logic). 9 notification categories. See [18-notification-system.md](18-notification-system.md). | High | REQ-ROAD-070 | Cross-cutting system consumed by all feature modules. |
| REQ-ROAD-079E | Universal tagging system | Free-form tags on all primary entities, autocomplete, polymorphic entity_tags join table, search/filter integration, admin tag management. Covers REQ-FE-063. See [16-frontend-architecture.md](16-frontend-architecture.md). | Medium | Phase 1 complete | Can be implemented anytime; listed here for integration with search/filter. |
| REQ-ROAD-079F | Alert rule system | Configurable audit log alert rules with conditions (category, severity, pattern, threshold, absence) and actions (in-app, email, webhook, severity escalation). 6 built-in templates. Covers REQ-LOG-026, REQ-LOG-027. See [11-audit-logging.md](11-audit-logging.md). | High | REQ-ROAD-074, REQ-ROAD-079D | Depends on logging UI and notification system. |
| REQ-ROAD-079G | User preferences | Server-side user preferences (timezone, date/time format, table page size, dashboard defaults, notification preferences). Covers REQ-FE-060. | Low | Phase 1 complete | Simple CRUD; can be done anytime. |
| REQ-ROAD-079H | Data retention enforcement | Daily background worker for retention policy enforcement, configurable global and per-campaign retention, retention preview in admin settings. Covers 01-system-overview.md Section 8. | Medium | Phase 5 complete | Must be after all data stores exist. |
| REQ-ROAD-079I | Configuration export/import | Export framework config as ZIP (credential-free), import with conflict resolution (skip/overwrite/create new), selective import, dry run preview. Covers 01-system-overview.md Section 9. | High | Phase 5 complete | All exportable entities must exist before export/import can be built. |

#### Phase 5 Gate

The following MUST be complete before proceeding to Phase 6:

- [ ] The real-time dashboard displays live campaign events via WebSocket within 3 seconds
- [ ] All documented campaign metrics are computed and displayed accurately
- [ ] Reports can be generated in PDF, CSV, and JSON formats with appropriate access controls
- [ ] The logging UI supports filtering, full-text search, and real-time streaming
- [ ] Infrastructure health is visible in a consolidated dashboard with alerting
- [ ] Notifications are delivered via in-app WebSocket, email, and webhooks within documented timeframes
- [ ] Alert rules trigger notifications when audit log conditions match
- [ ] Universal tags can be applied and searched across all primary entity types
- [ ] User preferences persist across sessions and are applied consistently
- [ ] Email client analytics are captured and displayed in campaign metrics
- [ ] The Defender Dashboard displays organizational susceptibility metrics
- [ ] Campaign auto-summary generates within 60 seconds of campaign completion

#### Phase 5 Parallelization

```
Phase 4 complete
    │
    ├──► REQ-ROAD-070 (WebSocket dashboard) ──► REQ-ROAD-075 (infra health dashboard)
    │                                       └──► REQ-ROAD-079B (calendar view)
    │                                       └──► REQ-ROAD-079D (notification system) ──► REQ-ROAD-079F (alert rules)
    │
    ├──► REQ-ROAD-071 (metrics) ──► REQ-ROAD-072 (report templates) ──► REQ-ROAD-073 (report generation)
    │                           └──► REQ-ROAD-076 (email client analytics)        └──► REQ-ROAD-078 (auto-summary)
    │                           └──► REQ-ROAD-077 (Defender Dashboard)
    │                           └──► REQ-ROAD-079 (phishing report tracking)
    │
    ├──► REQ-ROAD-074 (logging UI)
    │
    ├──► REQ-ROAD-079E (universal tags)     [parallel — no Phase 4+ dependency]
    ├──► REQ-ROAD-079G (user preferences)   [parallel — no Phase 4+ dependency]
    │
    └──► After Phase 5 core complete:
         ├──► REQ-ROAD-079H (data retention)
         └──► REQ-ROAD-079I (config export/import)
```

All three core tracks (dashboard, metrics/reporting, logging) can proceed in parallel. Universal tags and user preferences have no Phase 4+ dependency and can be built at any time. Data retention and config export/import require all Phase 5 features to be complete.

---

### Phase 6: Polish & Hardening

**Objective:** Ensure production readiness through comprehensive testing, performance optimization, security hardening, and operational documentation.

**Complexity: High**

| ID | Requirement | Description | Complexity | Dependencies | Notes |
|----|-------------|-------------|------------|--------------|-------|
| REQ-ROAD-080 | End-to-end testing | Full campaign lifecycle tests (create targets, design template, build landing page, configure campaign, provision endpoint, launch, capture credentials, generate report). Cross-browser testing for landing pages. Multi-auth-provider testing. | Very High | Phase 5 complete | Must cover all critical paths documented across all requirement documents. |
| REQ-ROAD-081 | Performance optimization | Database query optimization (indexes for metric queries, log queries), WebSocket connection scaling, concurrent campaign load testing, compilation engine performance (60-second budget per REQ-LPB-047), email sending throughput testing. | High | Phase 5 complete | Performance targets are defined in individual requirement docs. |
| REQ-ROAD-082 | Security audit and hardening | Review all encrypted-at-rest fields, validate RBAC enforcement on every endpoint, verify no secrets in logs/responses, SMTP header sanitization audit, anti-fingerprinting effectiveness review, penetration testing of framework API, review Cline-generated offensive code. | Very High | Phase 5 complete | Cline-generated code requires red team lead review. |
| REQ-ROAD-083 | Documentation | API documentation (OpenAPI/Swagger), operator manual (campaign workflow), engineer manual (infrastructure management), administrator manual (system configuration, auth providers), architecture decision records. | Medium | Phase 5 complete | |
| REQ-ROAD-084 | Operational runbooks | Procedures for: initial deployment, backup and restore, encryption key rotation, disaster recovery, adding new auth providers, troubleshooting endpoint health failures, manual credential purge, database maintenance. | Medium | Phase 5 complete | |

#### Phase 6 Gate

The following MUST be complete before the platform is considered production-ready:

- [ ] End-to-end tests pass for a complete campaign lifecycle on both AWS and Azure
- [ ] No API endpoint returns data or allows actions that violate its documented RBAC restrictions
- [ ] Performance meets all documented targets (compilation <60s, metrics update <10s, WebSocket events <3s)
- [ ] Security audit finds no high-severity issues in the framework or Cline-generated code
- [ ] Operational runbooks cover all documented maintenance scenarios

#### Phase 6 Parallelization

All Phase 6 items can proceed in parallel after Phase 5 is complete.

---

### Phase 7: AI Readiness (Future)

**Objective:** Prepare the platform for AI agent integration. This phase adds the infrastructure for an AI agent to propose campaign configurations, draft templates, and suggest optimizations — with human review gates.

**Complexity: High**

| ID | Requirement | Description | Complexity | Dependencies | Notes |
|----|-------------|-------------|------------|--------------|-------|
| REQ-ROAD-090 | AI provider configuration | Settings UI for configuring AI model providers (API keys, endpoints, model selection), encrypted credential storage, connection testing. See [12-ai-integration.md](12-ai-integration.md). | Low | Phase 6 complete | |
| REQ-ROAD-091 | MCP server interface | Model Context Protocol server implementation allowing an AI agent to interact with Tackle's API. Structured tool definitions for campaign operations, template management, target queries, and metric retrieval. | High | REQ-ROAD-090 | Defines the contract between AI agent and Tackle. |
| REQ-ROAD-092 | AI proposal queue and review workflow | Queue for AI-generated proposals (campaign configs, template drafts, schedule suggestions). Operator review, approval, and rejection workflow. Diff view showing AI-proposed changes. Audit logging for all AI actions. | High | REQ-ROAD-091 | AI never executes without human approval. |
| REQ-ROAD-093 | Campaign drafting AI integration | AI-assisted campaign creation: generate email templates based on target context, suggest landing page designs, recommend sending schedules based on historical metrics, optimize A/B test configurations. All outputs are proposals requiring operator approval. | Medium | REQ-ROAD-091, REQ-ROAD-092 | AI generates; humans approve. |

#### Phase 7 Gate

- [ ] An AI provider can be configured and tested from the settings UI
- [ ] The MCP server exposes all documented tool definitions and the AI agent can invoke them
- [ ] AI-generated proposals appear in the review queue and require explicit operator approval
- [ ] No AI action modifies platform state without passing through the approval workflow
- [ ] All AI interactions are recorded in the audit log

---

## 3. Cross-Phase Dependency Map

The following diagram shows major dependencies between phases and the critical path through the implementation.

```
Phase 1: Foundation
    │
    ├──────────────────────────► Phase 2: Infrastructure ──────┐
    │                                                          │
    ├──► Phase 3: Campaign Core (targets, templates,           │
    │    builder start in parallel with Phase 2) ──────────────┤
    │                                                          │
    │    ┌─────────────────────────────────────────────────────┘
    │    │  Phase 3 campaign config requires Phase 2 outputs
    │    │
    │    ▼
    │  Phase 4: Campaign Execution
    │    │
    │    ▼
    │  Phase 5: Monitoring & Reporting
    │    │
    │    ▼
    │  Phase 6: Polish & Hardening
    │    │
    │    ▼
    │  Phase 7: AI Readiness (Future)
    │
    └──► Note: Phases 2 and 3 overlap. Target management,
         email templates, and the landing page builder can
         begin as soon as Phase 1 completes, even while
         Phase 2 infrastructure work continues.
```

### Critical Path

The longest dependency chain determining minimum project duration:

```
Project scaffolding (REQ-ROAD-001)
    → DB schema (REQ-ROAD-002)
    → Local auth (REQ-ROAD-003)
    → RBAC (REQ-ROAD-005)
    → Landing page builder (REQ-ROAD-042)
    → Compilation engine (REQ-ROAD-048)
    → Landing page hosting (REQ-ROAD-049)
    → Campaign configuration (REQ-ROAD-050)  [also needs Phase 2 outputs]
    → Campaign state machine (REQ-ROAD-051)
    → Phishing endpoint binary (REQ-ROAD-060)
    → Email sending engine (REQ-ROAD-062)
    → WebSocket dashboard (REQ-ROAD-070)
    → Metrics aggregation (REQ-ROAD-071)
    → End-to-end testing (REQ-ROAD-080)
```

### Detailed Cross-Phase Dependencies

| Downstream Item | Depends On | Phase Boundary |
|----------------|-----------|----------------|
| REQ-ROAD-050 (campaign config) | REQ-ROAD-040 (targets), REQ-ROAD-041 (templates), REQ-ROAD-048 (compilation), REQ-ROAD-026 (SMTP), REQ-ROAD-022 (domains), REQ-ROAD-028 (instances) | Phase 3 depends on Phase 2 |
| REQ-ROAD-060 (proxy binary) | REQ-ROAD-028 (instance lifecycle), REQ-ROAD-049 (landing page hosting) | Phase 4 depends on Phases 2+3 |
| REQ-ROAD-061 (SMTP relay) | REQ-ROAD-060 (proxy binary), REQ-ROAD-026 (SMTP config) | Phase 4 depends on Phases 2+4 |
| REQ-ROAD-063 (credential capture) | REQ-ROAD-048 (compilation engine) | Phase 4 depends on Phase 3 |
| REQ-ROAD-067 (anti-fingerprinting) | REQ-ROAD-048 (compilation engine) | Phase 4 depends on Phase 3 |
| REQ-ROAD-070 (dashboard) | REQ-ROAD-065 (tracking), REQ-ROAD-064 (credential storage), REQ-ROAD-062 (sending engine) | Phase 5 depends on Phase 4 |
| REQ-ROAD-073 (report generation) | REQ-ROAD-071 (metrics aggregation) | Internal Phase 5 dependency |
| REQ-ROAD-069 (session capture) | REQ-ROAD-063 (credential capture) | Phase 4 internal dependency |
| REQ-ROAD-079D (notification system) | REQ-ROAD-070 (WebSocket dashboard) | Phase 5 internal dependency |
| REQ-ROAD-079F (alert rules) | REQ-ROAD-074 (logging UI), REQ-ROAD-079D (notifications) | Phase 5 internal dependency |
| REQ-ROAD-079I (config export/import) | All Phases 1-5 feature modules | Phase 5 depends on all prior |
| REQ-ROAD-080 (E2E testing) | All Phases 1-5 | Phase 6 depends on all prior phases |

---

## 4. Risk Areas

### REQ-RISK-001 — Landing Page Builder Complexity

**Risk Level: Very High**

The visual drag-and-drop builder (REQ-ROAD-042) is the most complex frontend feature in the project. It requires a canvas-based WYSIWYG editor, component library, property panels, CSS editing, undo/redo, and responsive design — essentially building a simplified web development IDE.

**Mitigations:**
- Evaluate established React-based builder frameworks (e.g., GrapesJS, Craft.js) as a foundation rather than building from scratch
- Prioritize a minimal viable builder (basic components, single-page) before adding multi-page support and advanced styling
- The builder is on the critical path — delays here cascade to the compilation engine, campaign configuration, and all subsequent phases

### REQ-RISK-002 — Anti-Fingerprinting Effectiveness

**Risk Level: High**

The anti-fingerprinting system (REQ-ROAD-067) must produce output that defeats real-world signature matching (YARA rules, HTML similarity analysis). Theoretical randomization may not survive contact with production defensive tooling.

**Mitigations:**
- Test against actual YARA rules and similarity detection tools during development, not just in Phase 6
- The six Cline tasks (AF-1 through AF-6) are independently testable — validate each randomization layer before integrating
- Plan for iterative refinement based on red team testing results

### REQ-RISK-003 — Transparent Proxy Reliability

**Risk Level: High**

The phishing endpoint's transparent reverse proxy (REQ-ROAD-060) must handle TLS termination, WebSocket passthrough (if needed), and all HTTP methods without any behavioral differences visible to the target or their browser's developer tools.

**Mitigations:**
- Build comprehensive test suites covering edge cases: large POST bodies, chunked transfer encoding, WebSocket upgrade, TLS renegotiation, HTTP/2
- Test across all target browsers (Chrome, Firefox, Edge, Safari) with developer tools open to verify no proxy artifacts leak
- Include fallback behavior for malformed requests that avoids revealing the proxy architecture

### REQ-RISK-004 — Multi-Auth Provider Complexity

**Risk Level: Medium**

Supporting four simultaneous authentication providers (Local, OIDC, FusionAuth, LDAP) with account linking introduces significant edge cases: email conflicts, account merge scenarios, provider outages, inconsistent group-to-role mappings.

**Mitigations:**
- Implement local auth first (REQ-ROAD-003) as the stable baseline
- Add external providers incrementally (REQ-ROAD-007/008/009 are parallelizable but should each be integration-tested independently)
- Maintain comprehensive test cases for account linking and conflict resolution

### REQ-RISK-005 — Real-Time WebSocket Scaling

**Risk Level: Medium**

Multiple operators viewing live campaign dashboards while campaigns are actively sending and capturing creates sustained WebSocket load. Each capture event, delivery status update, and tracking event generates a broadcast.

**Mitigations:**
- Implement topic-based subscriptions so operators only receive events for campaigns they are viewing
- Use event batching (aggregate events over 1-2 second windows) to reduce broadcast frequency during high-volume campaigns
- Load test with simulated campaign volumes before Phase 5 gate

### REQ-RISK-006 — Compilation Engine Performance

**Risk Level: Medium**

The compilation engine (REQ-ROAD-048) must produce a working Go binary within 60 seconds (REQ-LPB-047). Adding anti-fingerprinting (REQ-ROAD-067) increases compilation complexity significantly.

**Mitigations:**
- Profile the compilation pipeline early and establish a performance baseline before adding anti-fingerprinting layers
- Consider pre-compiled component libraries to reduce per-build work
- The 60-second budget is for standard projects (5 pages, 20 components); complex projects may need a longer allowance

---

## 5. Complexity Summary

| Phase | Overall Complexity | Key High-Complexity Items |
|-------|-------------------|--------------------------|
| **Phase 1: Foundation** | High | Multi-auth provider support, RBAC permission system |
| **Phase 2: Infrastructure** | High | Instance lifecycle management (two cloud providers), four domain provider integrations |
| **Phase 3: Campaign Core** | Very High | Landing page builder (visual editor), compilation engine |
| **Phase 4: Campaign Execution** | Very High | Transparent reverse proxy [CLINE], anti-fingerprinting engine [CLINE], credential capture [CLINE], SMTP relay [CLINE] |
| **Phase 5: Monitoring & Reporting** | High | Real-time WebSocket dashboard, PDF report generation with charts |
| **Phase 6: Polish & Hardening** | High | End-to-end testing across all subsystems, security audit of offensive code |
| **Phase 7: AI Readiness** | High | MCP server interface, proposal review workflow |

### Item-Level Complexity Ratings

| Complexity | Items |
|------------|-------|
| **Very High** | Landing page visual editor (REQ-ROAD-042), compilation engine (REQ-ROAD-048), instance lifecycle management (REQ-ROAD-028), transparent reverse proxy [CLINE] (REQ-ROAD-060), SMTP relay [CLINE] (REQ-ROAD-061), credential capture [CLINE] (REQ-ROAD-063), anti-fingerprinting engine [CLINE] (REQ-ROAD-067), end-to-end testing (REQ-ROAD-080), security audit (REQ-ROAD-082) |
| **High** | Local auth (REQ-ROAD-003), RBAC (REQ-ROAD-005), OIDC (REQ-ROAD-007), LDAP (REQ-ROAD-009), domain provider integrations (REQ-ROAD-021), domain registration (REQ-ROAD-022), DNS management (REQ-ROAD-023), DKIM/SPF/DMARC (REQ-ROAD-024), infrastructure health monitoring (REQ-ROAD-029), email templates (REQ-ROAD-041), form builder (REQ-ROAD-043), multi-page support (REQ-ROAD-044), landing page hosting (REQ-ROAD-049), campaign configuration (REQ-ROAD-050), campaign state machine (REQ-ROAD-051), WYSIWYG email editor (REQ-ROAD-054), landing page import/cloning (REQ-ROAD-056), email sending engine (REQ-ROAD-062), credential storage (REQ-ROAD-064), tracking system (REQ-ROAD-065), email header sanitization [CLINE] (REQ-ROAD-066), session capture [CLINE] (REQ-ROAD-069), WebSocket dashboard (REQ-ROAD-070), metrics aggregation (REQ-ROAD-071), report generation (REQ-ROAD-073), logging UI (REQ-ROAD-074), Defender Dashboard (REQ-ROAD-077), notification system (REQ-ROAD-079D), alert rule system (REQ-ROAD-079F), config export/import (REQ-ROAD-079I), performance optimization (REQ-ROAD-081), MCP server (REQ-ROAD-091), AI proposal workflow (REQ-ROAD-092) |
| **Medium** | DB schema (REQ-ROAD-002), setup flow (REQ-ROAD-004), user management (REQ-ROAD-006), FusionAuth (REQ-ROAD-008), password policy (REQ-ROAD-010), audit logging (REQ-ROAD-012), API middleware (REQ-ROAD-013), cloud credentials (REQ-ROAD-020), SMTP config (REQ-ROAD-026), instance templates (REQ-ROAD-027), domain health checking (REQ-ROAD-025), domain categorization (REQ-ROAD-031), typosquat generator (REQ-ROAD-032), target management (REQ-ROAD-040), styling/themes (REQ-ROAD-045), JS injection (REQ-ROAD-046), preview mode (REQ-ROAD-047), approval workflow (REQ-ROAD-052), campaign cloning/templates (REQ-ROAD-053), campaign scheduling/send order (REQ-ROAD-057), parallel approval (REQ-ROAD-058), email auth validation (REQ-ROAD-068), URL redirect chains (REQ-ROAD-069A), report templates (REQ-ROAD-072), infra health dashboard (REQ-ROAD-075), email client analytics (REQ-ROAD-076), campaign auto-summary (REQ-ROAD-078), phishing report tracking (REQ-ROAD-079), dry run mode (REQ-ROAD-079A), calendar view (REQ-ROAD-079B), canary targets (REQ-ROAD-079C), universal tags (REQ-ROAD-079E), data retention (REQ-ROAD-079H), documentation (REQ-ROAD-083), operational runbooks (REQ-ROAD-084), AI campaign drafting (REQ-ROAD-093) |
| **Low** | API key auth (REQ-ROAD-011), `.clinerules` (REQ-ROAD-014), built-in email template library (REQ-ROAD-055), TLS cert upload (REQ-ROAD-069B), user preferences (REQ-ROAD-079G), AI provider config (REQ-ROAD-090) |

---

## 6. Cline Delegation Summary

The following items are delegated to the local LLM via Cline prompts. Each involves offensive security techniques that must be reviewed by the red team lead before integration.

| ID | Phase | Description | Cline Tasks |
|----|-------|-------------|-------------|
| REQ-ROAD-060 | 4 | Transparent reverse proxy binary | Proxy implementation, TLS termination, header passthrough |
| REQ-ROAD-061 | 4 | SMTP relay through phishing endpoints | SMTP client, relay logic, delivery status reporting |
| REQ-ROAD-063 | 4 | Credential capture system | Form interception (REQ-CRED-001, REQ-CRED-002), tracking tokens (REQ-CRED-004), credential replay (REQ-CRED-018) |
| REQ-ROAD-066 | 4 | Email header sanitization | Header stripping, HELO customization, Received chain management |
| REQ-ROAD-067 | 4 | Anti-fingerprinting engine | AF-1 (DOM randomization), AF-2 (CSS class names), AF-3 (asset paths), AF-4 (decoy injection), AF-5 (HTTP headers), AF-6 (code generation strategies) |
| REQ-ROAD-069 | 4 | Session capture system | Cookie interception, OAuth token capture, localStorage/sessionStorage reading, auth header capture |

**Prerequisite:** The `.clinerules` file (REQ-ROAD-014) MUST be complete and validated before any Cline tasks are initiated.

**Review gate:** All Cline-generated code MUST pass a security review by the red team lead before merging into the main codebase (enforced in Phase 6, REQ-ROAD-082, but reviewed incrementally during Phase 4).

---

## 7. Dependencies on Requirement Documents

This roadmap references and sequences work defined across all Tackle requirement documents:

| Document | Phases Affected |
|----------|----------------|
| [01-system-overview.md](01-system-overview.md) | All phases (architecture, tech stack, import path rules) |
| [02-authentication-authorization.md](02-authentication-authorization.md) | Phase 1 (auth, RBAC, session management) |
| [03-domain-infrastructure.md](03-domain-infrastructure.md) | Phase 2 (domains, cloud, instances) |
| [04-email-smtp.md](04-email-smtp.md) | Phase 2 (SMTP config), Phase 3 (templates), Phase 4 (sending, DKIM signing) |
| [05-landing-page-builder.md](05-landing-page-builder.md) | Phase 3 (builder, compilation), Phase 4 (anti-fingerprinting) |
| [06-campaign-management.md](06-campaign-management.md) | Phase 3 (campaign config, state machine), Phase 4 (execution) |
| [07-phishing-endpoints.md](07-phishing-endpoints.md) | Phase 2 (provisioning), Phase 4 (proxy binary, deployment) |
| [08-credential-capture.md](08-credential-capture.md) | Phase 4 (capture mechanism, storage, access control) |
| [09-target-management.md](09-target-management.md) | Phase 3 (target CRUD, CSV import, groups) |
| [10-metrics-reporting.md](10-metrics-reporting.md) | Phase 5 (metrics, reports) |
| [11-audit-logging.md](11-audit-logging.md) | Phase 1 (basic logging), Phase 5 (logging UI) |
| [12-ai-integration.md](12-ai-integration.md) | Phase 7 (AI readiness) |
| [14-database-schema.md](14-database-schema.md) | All phases (schema definitions for each module) |
| [16-frontend-architecture.md](16-frontend-architecture.md) | All phases (React admin UI) |
| [18-notification-system.md](18-notification-system.md) | Phase 5 (notification system, alert integration) |
