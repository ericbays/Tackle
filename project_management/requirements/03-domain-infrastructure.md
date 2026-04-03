# 03 — Domain & Infrastructure Management

## 1. Purpose

This document specifies the requirements for managing external domains and cloud infrastructure within the Tackle framework. Domains are used as the sender identity for phishing emails and as the host names that targets see in URLs. Cloud infrastructure (AWS EC2, Azure VMs) provides the phishing endpoints that sit between the framework server and targets. Together, these subsystems give operators full lifecycle control over the externally-facing attack surface from a single UI.

---

## 2. Domain Management

### 2.1 Domain Registrar Integrations

The framework must integrate with the following domain registrar and DNS provider APIs:

| Provider | Capabilities | SDK / API |
|----------|-------------|-----------|
| **Namecheap** | Domain registration, renewal status, DNS record management | Namecheap XML API |
| **GoDaddy** | Domain registration, renewal status, DNS record management | GoDaddy REST API v1 |
| **AWS Route 53** | Hosted zone management, DNS record management (no registration) | AWS SDK for Go v2 |
| **Azure DNS** | DNS zone management, DNS record management (no registration) | Azure SDK for Go |

> **Note:** Route 53 and Azure DNS are DNS-only providers in this context. Domain registration itself is handled through Namecheap or GoDaddy. A single domain may use one registrar for registration and a different provider for DNS hosting (e.g., registered at Namecheap, DNS hosted in Route 53).

### 2.2 Provider Connection Management

**REQ-DOM-001** — The framework UI must provide a dedicated settings area where Admins and Engineers can create, edit, test, and delete API connections for each supported domain provider (Namecheap, GoDaddy, AWS Route 53, Azure DNS).

**REQ-DOM-002** — Each provider connection record must store: provider type, display name, API credentials (encrypted at rest), date created, date last tested, connection status (untested / healthy / error), and the user who created the connection.

**REQ-DOM-003** — The framework must provide a "Test Connection" action for each provider connection that validates the stored credentials against the provider's API and reports success or a specific, actionable error message.

**REQ-DOM-004** — The framework must support multiple connections per provider type (e.g., two separate Namecheap accounts) to accommodate operational segmentation.

**REQ-DOM-005** — Provider connections must be referenceable by other subsystems (domain profiles, campaign configuration) via a stable internal identifier that does not change if the connection's display name is edited.

### 2.3 Domain Registration

**REQ-DOM-006** — Engineers must be able to initiate a domain registration through the framework UI by selecting a registrar connection (Namecheap or GoDaddy), entering a desired domain name, and confirming the registration.

**REQ-DOM-007** — Before executing a registration, the framework must perform an availability check against the registrar API and present the result (available, unavailable, premium pricing) to the user.

**REQ-DOM-008** — Upon successful registration, the framework must automatically create a domain profile (see REQ-DOM-016) and record the registration event in the audit log.

**REQ-DOM-009** — Domain registration must be an approval-gated operation: if initiated by a user without the Engineer role, it must be queued for Engineer approval before execution.

### 2.4 Domain Renewal Tracking

**REQ-DOM-010** — The framework must periodically query each registrar connection to synchronize the expiry date of all tracked domains. The sync interval must be configurable (default: every 24 hours).

**REQ-DOM-011** — The framework UI must display a domain inventory view sorted by expiry date, with visual indicators for domains expiring within 30 days (warning) and 7 days (critical).

**REQ-DOM-012** — The framework must generate system notifications when a domain enters the 30-day or 7-day expiry window. Notifications must be visible in the UI and logged.

**REQ-DOM-013** — Engineers must be able to trigger a manual renewal for a domain through the framework UI, subject to the registrar API supporting renewal operations.

**REQ-DOM-014** — If a registrar API does not support programmatic renewal, the framework must display a clear message directing the user to renew through the registrar's web console, and provide a direct link where possible.

**REQ-DOM-015** — The framework must track renewal history (date, duration, cost if available) for each domain.

### 2.5 Domain Profiles

**REQ-DOM-016** — Every domain managed by the framework must have a domain profile containing the following fields:

| Field | Description |
|-------|-------------|
| Domain name | The fully qualified domain name |
| Registrar connection | Reference to the provider connection used for registration |
| DNS provider connection | Reference to the provider connection used for DNS hosting (may differ from registrar) |
| Status | One of: `pending_registration`, `active`, `expired`, `suspended`, `decommissioned` |
| Registration date | Date the domain was registered |
| Expiry date | Current expiry date, synced from registrar |
| Associated campaigns | List of campaigns that have used or are using this domain |
| Tags | Free-form tags for organizational grouping (e.g., "finance-theme", "Q1-2026") |
| Notes | Free-text field for operator notes |
| Created by | User who added the domain to the framework |
| Created at / Updated at | Timestamps |

**REQ-DOM-017** — Domain profiles must be searchable and filterable by status, registrar, DNS provider, tag, expiry date range, and campaign association.

**REQ-DOM-018** — Engineers must be able to import an existing domain (one already registered outside the framework) by manually creating a domain profile and linking it to the appropriate registrar and DNS provider connections.

**REQ-DOM-019** — Deleting a domain profile must be a soft-delete that sets the status to `decommissioned`. Hard deletion must not be permitted if the domain has any campaign associations. Decommissioned domains must remain queryable for historical reporting.

### 2.6 DNS Record Management

**REQ-DOM-020** — For each domain profile, the framework must provide full CRUD (create, read, update, delete) operations for the following DNS record types via the associated DNS provider connection:

- **A** — IPv4 address
- **AAAA** — IPv6 address
- **CNAME** — Canonical name alias
- **MX** — Mail exchange
- **TXT** — Text records (including SPF, DKIM, DMARC values)
- **NS** — Name server delegation
- **SOA** — Start of authority (read-only display where the provider API supports it)

**REQ-DOM-021** — The framework UI must present DNS records in a tabular view grouped by record type, with inline editing and a confirmation step before committing changes to the provider API.

**REQ-DOM-022** — All DNS record mutations must be recorded in the audit log with the before-and-after values and the identity of the user who made the change.

**REQ-DOM-023** — The framework must validate DNS record inputs client-side and server-side before submission (e.g., A records must be valid IPv4, AAAA must be valid IPv6, CNAME targets must be valid hostnames, MX records must include a priority value).

**REQ-DOM-024** — When a DNS record change is committed, the framework must automatically trigger a propagation check (see REQ-DOM-031) and display the result to the user.

### 2.7 Email Authentication Record Management (DKIM / SPF / DMARC)

**REQ-DOM-025** — The framework must provide a dedicated "Email Authentication" panel within each domain profile that consolidates DKIM, SPF, and DMARC configuration into a guided workflow, separate from the raw DNS record editor.

**REQ-DOM-026** — For SPF, the framework must offer a builder UI that lets the user specify authorized sending sources (IP addresses, `include:` directives, `a`/`mx` mechanisms) and automatically generate the correct TXT record value. The builder must enforce the single-SPF-record-per-domain rule and warn if the generated record exceeds the 10-lookup limit.

**REQ-DOM-027** — For DKIM, the framework must accept a DKIM selector name and public key value, generate the correct TXT record at `<selector>._domainkey.<domain>`, and publish it via the DNS provider API. The framework must also support displaying the expected DKIM record so operators can verify alignment with their SMTP sending configuration.

**REQ-DOM-028** — For DMARC, the framework must offer a builder UI that lets the user configure the policy (`none`, `quarantine`, `reject`), percentage, `rua` (aggregate report URI), `ruf` (forensic report URI), and alignment modes (`aspf`, `adkim`). The builder must generate the correct TXT record at `_dmarc.<domain>` and publish it via the DNS provider API.

**REQ-DOM-029** — After publishing any email authentication record, the framework must run an automated validation check that queries the published record via public DNS and confirms it matches the intended value. Mismatches must be reported with a clear diff.

**REQ-DOM-030** — The Email Authentication panel must display a summary status for each mechanism (SPF, DKIM, DMARC) using indicators: `configured`, `misconfigured`, `missing`. This status must update after every change and on each domain health check.

### 2.8 Domain Health Checking

**REQ-DOM-031** — The framework must provide a "Check Health" action on each domain profile that performs the following checks and reports results:

| Check | Description |
|-------|-------------|
| **DNS Propagation** | Query the domain's A/AAAA records from multiple public DNS resolvers (minimum three geographically distributed resolvers) and confirm consistent responses |
| **Blacklist Status** | Query the domain against common DNS-based blocklists (e.g., Spamhaus DBL, SURBL, URIBL) and report if the domain is listed |
| **SPF Validity** | Parse and validate the published SPF record |
| **DKIM Validity** | Verify the published DKIM record for each known selector |
| **DMARC Validity** | Parse and validate the published DMARC record |
| **MX Resolution** | Confirm that MX records resolve to reachable mail servers |

**REQ-DOM-032** — Health check results must be persisted and timestamped so operators can review historical health over time.

**REQ-DOM-033** — The framework must support scheduled health checks at a configurable interval (default: every 6 hours) for all domains with status `active`.

**REQ-DOM-034** — If a health check detects a domain on a blocklist, the framework must generate a high-priority notification visible in the UI dashboard and recorded in the audit log.

**REQ-DOM-035** — The domain inventory view must include a health summary column showing the most recent overall health status (`healthy`, `warning`, `critical`, `unchecked`) for each domain.

---

## 3. Cloud Infrastructure Management

### 3.1 Cloud Provider Credential Management

**REQ-INFRA-001** — The framework UI must provide a settings area where Admins and Engineers can create, edit, test, and delete API credential sets for AWS and Azure.

**REQ-INFRA-002** — Each AWS credential set must store: display name, AWS Access Key ID (encrypted at rest), AWS Secret Access Key (encrypted at rest), default region, and optionally an IAM Role ARN for assume-role workflows.

**REQ-INFRA-003** — Each Azure credential set must store: display name, Tenant ID, Client ID (encrypted at rest), Client Secret (encrypted at rest), Subscription ID, and default region.

**REQ-INFRA-004** — The framework must provide a "Test Connection" action for each credential set that validates permissions by performing a non-mutating API call (e.g., `DescribeInstances` for AWS, list VMs for Azure) and reports success or a specific error.

**REQ-INFRA-005** — Credential sets must be referenceable by instance templates and campaign configurations via a stable internal identifier.

### 3.2 Instance Templates

**REQ-INFRA-006** — Engineers must be able to define reusable instance templates (also referred to as instance profiles) that capture the full specification for provisioning a phishing endpoint VM.

**REQ-INFRA-007** — Each instance template must contain the following fields:

| Field | Description |
|-------|-------------|
| Display name | Human-readable name for the template |
| Cloud provider | `aws` or `azure` |
| Credential set | Reference to the cloud provider credential set to use |
| Region | The cloud region for deployment (e.g., `us-east-1`, `eastus`) |
| Instance size | Provider-specific size identifier (e.g., `t3.micro`, `Standard_B1s`) |
| OS image | AMI ID (AWS) or image reference (Azure) |
| Security group(s) | One or more security group IDs (AWS) or NSG references (Azure) |
| SSH key reference | Key pair name (AWS) or SSH public key (Azure) for emergency access |
| User data / cloud-init | Optional startup script executed on first boot |
| Tags | Key-value pairs applied to the provisioned instance for cloud-side identification |
| Notes | Free-text field for operator documentation |

**REQ-INFRA-008** — Instance templates must be versioned. Editing a template must create a new version; previous versions must remain associated with any instances that were provisioned using them, ensuring accurate historical records.

**REQ-INFRA-009** — The framework must validate template fields before saving (e.g., confirm the specified region is valid for the selected provider, confirm the credential set matches the selected provider).

### 3.3 Instance Lifecycle Management

**REQ-INFRA-010** — The framework must manage the full lifecycle of phishing endpoint instances through the following states:

```
provisioning  -->  configuring  -->  running  -->  stopping  -->  stopped  -->  terminating  -->  terminated
                                       |                            |
                                       +------- stopping -----------+
                                       |
                                       +------- terminating  -->  terminated
```

| State | Description |
|-------|-------------|
| `provisioning` | VM creation API call has been issued; waiting for the instance to reach a running state in the cloud provider |
| `configuring` | Instance is running; the framework is deploying the phishing endpoint binary, TLS certificates, and proxy configuration |
| `running` | Endpoint is operational and passing health checks |
| `stopping` | A stop command has been issued to the cloud provider |
| `stopped` | Instance exists but is not running; can be restarted or terminated |
| `terminating` | A terminate/delete command has been issued |
| `terminated` | Instance has been destroyed in the cloud provider; record retained for audit |
| `error` | Any lifecycle transition that fails; includes an error message and the failed-from state |

**REQ-INFRA-011** — Provisioning a new instance must be an approval-gated operation. If initiated by a user without the Engineer role, it must be queued for Engineer approval before the cloud API call is executed.

**REQ-INFRA-012** — When provisioning, the framework must: (1) call the cloud provider API to create the instance using the selected template; (2) wait for the instance to reach a running state; (3) deploy the phishing endpoint binary to the instance via SSH or cloud-init; (4) configure the endpoint with campaign-specific settings (target domain, TLS certificate, proxy rules, SMTP relay configuration); (5) verify the endpoint is healthy before marking it `running`.

**REQ-INFRA-013** — The framework must support the following operator-initiated lifecycle actions from the UI:

- **Provision** — Create a new instance from a template, associated with a specific campaign
- **Stop** — Shut down a running instance without destroying it
- **Start** — Restart a stopped instance
- **Terminate** — Permanently destroy an instance and release all cloud resources
- **Redeploy** — Re-run the configuration step on a running instance (e.g., to push updated endpoint binary or configuration)

**REQ-INFRA-014** — Terminate must require an explicit confirmation dialog that names the instance and its associated campaign. Termination must not be reversible.

**REQ-INFRA-015** — All lifecycle state transitions must be recorded in the audit log with timestamps, the initiating user, the cloud provider's instance ID, and any error details.

**REQ-INFRA-016** — The framework must handle partial failures gracefully. If provisioning fails after the cloud instance is created but before configuration completes, the framework must record the cloud instance ID, set the state to `error`, and provide the operator with options to retry configuration or terminate the orphaned instance.

### 3.4 Health Monitoring

**REQ-INFRA-017** — The framework must perform periodic health checks on all instances in the `running` state. The health check interval must be configurable (default: every 60 seconds).

**REQ-INFRA-018** — Each health check must verify:

| Check | Method |
|-------|--------|
| **Instance reachable** | ICMP ping or TCP connect to the instance's public IP |
| **Endpoint process running** | HTTP(S) request to the endpoint's health endpoint (e.g., `GET /healthz` on a management port) |
| **TLS certificate valid** | Confirm the TLS certificate presented on port 443 is not expired and matches the expected domain |
| **SMTP relay functional** | Verify the SMTP relay port is accepting connections (TCP connect check) |

**REQ-INFRA-019** — Health check results must be stored with timestamps and associated with the instance record. The framework must maintain a rolling window of health history (configurable, default: 7 days).

**REQ-INFRA-020** — If an instance fails health checks for a configurable number of consecutive intervals (default: 3), the framework must mark the instance status as `unhealthy`, generate a high-priority notification, and display the failure details in the UI.

**REQ-INFRA-021** — The framework UI must provide a real-time infrastructure dashboard showing all active instances with their current health status, uptime, associated campaign, public IP, region, and provider. This dashboard must update via WebSocket.

### 3.5 Domain Categorization and Reputation

**REQ-INFRA-022** — The framework must support checking domain categorization status against common web filtering services (e.g., Bluecoat/Symantec, Zscaler, Fortiguard, Palo Alto URL Filtering). The check must query these services and display the current categorization (e.g., "Business", "Uncategorized", "Newly Registered", "Suspicious") in the domain profile.

**REQ-INFRA-023** — The domain profile must display the categorization status with a visual indicator: `categorized` (domain has a benign category), `uncategorized` (not yet categorized — common for new domains), `flagged` (categorized as suspicious/malicious), `unknown` (check failed or unsupported service).

**REQ-INFRA-024** — The framework must support scheduled categorization checks at a configurable interval (default: every 24 hours) for all domains with status `active`.

**REQ-INFRA-025** — The framework must generate a notification when a domain's categorization changes (especially transitions to a negative category).

**REQ-INFRA-026** — Categorization check results must be persisted with timestamps so operators can review categorization history over time.

### 3.6 Domain Reconnaissance — Typosquat Generator

**REQ-INFRA-027** — The framework must include a typosquat domain generator tool accessible from the domain management UI. Given a target domain name (e.g., `example.com`), the tool must generate candidate lookalike domains using the following techniques:

| Technique | Example (for `example.com`) |
|-----------|---------------------------|
| **Character substitution** | `examp1e.com`, `exarnple.com` (rn→m), `examp|e.com` |
| **Homoglyph replacement** | `ехаmple.com` (Cyrillic е/а), `exaṁple.com` |
| **Adjacent key typos** | `examplr.com`, `exanple.com`, `exampke.com` |
| **Character omission** | `examle.com`, `exmple.com` |
| **Character insertion** | `exaample.com`, `examplee.com` |
| **Character transposition** | `exmaple.com`, `exampel.com` |
| **TLD variations** | `example.net`, `example.org`, `example.co`, `example.io` |
| **Hyphenation** | `ex-ample.com`, `exam-ple.com` |
| **Subdomain spoofing** | `example.com.attacker.com` format suggestions |

**REQ-INFRA-028** — For each generated candidate domain, the tool must check registration availability via the configured registrar APIs (Namecheap, GoDaddy) and display the result (available, taken, premium).

**REQ-INFRA-029** — The tool must allow an operator to select one or more available candidate domains and initiate registration directly from the results view (subject to the standard approval workflow for domain registration).

**REQ-INFRA-030** — The generated candidates must be sortable and filterable by technique type, availability status, and visual similarity score (a heuristic rating of how closely the candidate resembles the original domain).

---

## 4. Security Considerations

**SEC-DOM-001** — All domain registrar and DNS provider API credentials must be encrypted at rest using the application's encryption key (see 01-system-overview.md, Section 7). Credentials must never appear in logs, API responses, or frontend payloads. When displayed in the UI, credentials must be masked (e.g., `****abcd`).

**SEC-DOM-002** — All cloud provider credentials (AWS access keys, Azure client secrets) must follow the same encryption-at-rest and masking requirements as domain provider credentials.

**SEC-DOM-003** — API calls to external providers must use TLS. The framework must validate server certificates and must not disable certificate verification.

**SEC-DOM-004** — Provider API credentials must follow the principle of least privilege. The framework documentation must specify the minimum IAM permissions (AWS) and RBAC roles (Azure) required for each integration feature.

**SEC-DOM-005** — All infrastructure lifecycle operations (provisioning, termination, DNS changes, domain registration) must require authentication and appropriate RBAC authorization as defined in [02-authentication-authorization.md](02-authentication-authorization.md). At minimum, the Engineer role is required for mutating operations.

**SEC-DOM-006** — The phishing endpoint binary deployed to cloud instances must not contain embedded secrets. Runtime secrets (e.g., callback tokens for communicating with the framework server) must be injected at configuration time and stored in memory or in a file with restrictive permissions (mode 0600) on the instance.

**SEC-DOM-007** — SSH keys used for instance configuration must be generated per-provisioning operation and stored encrypted in the database. The framework should support automatic key rotation and cleanup of keys after an instance is terminated.

**SEC-DOM-008** — The framework must rate-limit API calls to external providers to prevent accidental abuse or credential exhaustion. Rate limits must be configurable per provider connection.

---

## 5. Acceptance Criteria

### Domain Management

- [ ] An Admin or Engineer can create, test, edit, and delete provider connections for Namecheap, GoDaddy, AWS Route 53, and Azure DNS
- [ ] An Engineer can register a new domain through the framework UI using a Namecheap or GoDaddy connection, and the resulting domain profile is created automatically
- [ ] An Engineer can import an existing domain by manually creating a domain profile
- [ ] The domain inventory view displays all domains with status, expiry, health, and associated campaigns
- [ ] Domain renewal tracking syncs expiry dates on a configurable schedule and generates notifications at 30-day and 7-day thresholds
- [ ] Full CRUD operations for A, AAAA, CNAME, MX, TXT, NS records are functional through the UI for all four DNS providers
- [ ] The Email Authentication panel provides builder UIs for SPF, DKIM, and DMARC that generate and publish correct DNS records
- [ ] After publishing email authentication records, automated validation confirms the published records match intended values
- [ ] Domain health checks execute on demand and on a configurable schedule, covering DNS propagation, blocklist status, and email authentication validity
- [ ] Blocklist detection generates a high-priority notification
- [ ] All DNS record changes and domain lifecycle events are recorded in the audit log with before/after values

### Cloud Infrastructure Management

- [ ] An Admin or Engineer can create, test, edit, and delete AWS and Azure credential sets
- [ ] An Engineer can define, version, and manage instance templates for both AWS and Azure
- [ ] An Engineer can provision a new phishing endpoint instance from a template, associated with a campaign
- [ ] The framework deploys the phishing endpoint binary and configuration to the instance automatically after provisioning
- [ ] Stop, start, terminate, and redeploy actions work correctly from the UI for both AWS and Azure instances
- [ ] Partial provisioning failures result in an `error` state with recovery options (retry or terminate)
- [ ] Health checks run on the configured interval and detect unreachable instances, failed endpoint processes, invalid TLS, and SMTP relay failures
- [ ] Consecutive health check failures trigger an `unhealthy` status and a high-priority notification
- [ ] The real-time infrastructure dashboard displays all instances with live health status via WebSocket
- [ ] All lifecycle transitions are recorded in the audit log

### Domain Categorization and Reconnaissance

- [ ] Domain profiles display categorization status from web filtering services with visual indicators
- [ ] Categorization checks run on a configurable schedule and generate notifications on status changes
- [ ] The typosquat generator produces candidate domains using all specified techniques (substitution, homoglyph, typo, omission, insertion, transposition, TLD, hyphenation)
- [ ] Generated candidates show real-time availability from registrar APIs
- [ ] Operators can initiate domain registration directly from the typosquat results view

---

## 6. Dependencies

| Dependency | Document | Relationship |
|------------|----------|-------------|
| **Authentication & Authorization** | [02-authentication-authorization.md](02-authentication-authorization.md) | RBAC roles (Admin, Engineer) gate access to provider connections, domain operations, and infrastructure lifecycle actions |
| **Campaign Management** | [06-campaign-management.md](06-campaign-management.md) | Campaigns reference domain profiles for sender identity and phishing URLs; campaigns reference instances as their phishing endpoints |
| **Phishing Endpoints** | [07-phishing-endpoints.md](07-phishing-endpoints.md) | The endpoint binary specification and its configuration contract define what the infrastructure subsystem deploys to provisioned instances |
| **SMTP Configuration** | [04-smtp-configuration.md](04-smtp-configuration.md) | Email authentication records (SPF, DKIM, DMARC) must align with the SMTP sending configuration for a campaign |
| **Audit Logging** | [11-audit-logging.md](11-audit-logging.md) | All domain and infrastructure operations emit audit log events |
| **Database Schema** | [14-database-schema.md](14-database-schema.md) | Domain profiles, provider connections, instance records, instance templates, health check history, and categorization data require persistent storage |
| **Frontend Architecture** | [16-frontend-architecture.md](16-frontend-architecture.md) | Domain management UI, infrastructure dashboard, and health monitoring views are implemented in the React Admin UI |
