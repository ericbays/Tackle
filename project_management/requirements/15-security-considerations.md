# 15 — Security Considerations

## 1. Purpose

This document defines the security requirements for the Tackle platform. As an authorized red team phishing simulation framework, Tackle occupies a unique position: it is itself an offensive tool, but it also handles highly sensitive data (captured credentials, infrastructure secrets, campaign intelligence) and must be protected against unauthorized access, data leakage, and tampering. Additionally, the campaigns it produces must resist detection by the blue team — operational security (OpSec) is a first-class concern.

Security considerations in this document fall into two categories:

1. **Framework Security** — Protecting the Tackle application, its data, and its infrastructure from unauthorized access and misuse.
2. **Operational Security (OpSec)** — Ensuring that phishing campaigns produced by Tackle resist detection, fingerprinting, and attribution by defenders.

---

## 2. Threat Model

### 2.1 Threats to the Framework

| ID | Threat | Impact | Likelihood |
|----|--------|--------|------------|
| TF-01 | **Unauthorized access to the admin UI** | Full control of campaigns, infrastructure, and captured data | Medium — mitigated by VPN/lab network, but insider access possible |
| TF-02 | **Credential theft from the database** | Exposure of captured phishing credentials, SMTP passwords, cloud provider keys | High impact if DB is compromised |
| TF-03 | **API key / secret exposure** | Unauthorized programmatic access to framework functions; cloud provider abuse | High impact — keys grant persistent access |
| TF-04 | **Log tampering** | Loss of audit trail integrity; concealment of unauthorized actions | Medium — undermines accountability |
| TF-05 | **Session hijacking** | Impersonation of authenticated users | Medium — mitigated by token rotation and secure cookie flags |
| TF-06 | **Insider threat** | Malicious or compromised team member exfiltrating data or sabotaging campaigns | High impact — authenticated access bypasses perimeter controls |
| TF-07 | **Backup data exposure** | Unencrypted database backups leaking sensitive campaign data | High impact if backups are accessed |
| TF-08 | **Supply chain attack on dependencies** | Compromised Go modules or npm packages introducing backdoors | Medium — mitigated by minimal dependencies and audit |

### 2.2 Operational Security Threats (Campaign OpSec)

| ID | Threat | Impact | Likelihood |
|----|--------|--------|------------|
| TO-01 | **Blue team detecting infrastructure patterns** | Campaign infrastructure burned; phishing endpoints blocklisted | High — sophisticated defenders actively look for this |
| TO-02 | **Landing page fingerprinting / signature detection** | Phishing pages blocked by URL filtering or WAF rules | High — automated scanners match known signatures |
| TO-03 | **Email header analysis revealing framework origin** | Emails flagged or quarantined; framework attribution exposed | Medium — email security gateways inspect headers |
| TO-04 | **DNS pattern analysis** | Bulk domain registration or naming patterns detected and blocklisted | Medium — threat intel feeds monitor new domains |
| TO-05 | **Binary / asset fingerprinting** | Phishing endpoint binary identified by EDR or network inspection | Medium — static binaries have stable hashes |
| TO-06 | **Network traffic analysis** | Communication patterns between endpoints and framework server identified | Low-Medium — encrypted traffic is harder to fingerprint but metadata is visible |

---

## 3. Security Requirements — Framework Protection

### 3.1 Encryption at Rest

#### REQ-SEC-001 — Secret Encryption at Rest

All secrets stored in the database MUST be encrypted at rest using AES-256-GCM with application-managed encryption keys.

**Scope of encrypted fields:**
- SMTP passwords and OAuth tokens
- Cloud provider credentials (AWS Secret Access Key, Azure Client Secret)
- Domain registrar API credentials (Namecheap, GoDaddy API keys)
- Captured credentials from phishing campaigns
- DKIM private keys
- OIDC / FusionAuth client secrets
- LDAP bind passwords
- Any user-configured webhook secrets or integration tokens

**Acceptance Criteria:**
- [ ] Every field listed above is encrypted before writing to PostgreSQL and decrypted only at the point of use in application code
- [ ] Raw secret values are never present in database query results accessible outside the encryption service layer
- [ ] Encryption uses AES-256-GCM with a unique nonce per encryption operation
- [ ] Encrypted fields in the database are stored as opaque byte arrays (not recognizable as the original value format)

#### REQ-SEC-002 — Encryption Key Management

The master encryption key MUST be provided to the application via an environment variable (`TACKLE_ENCRYPTION_KEY`) at startup. The key MUST NOT be stored in the database, in configuration files committed to version control, or in application logs.

**Acceptance Criteria:**
- [ ] The application refuses to start if `TACKLE_ENCRYPTION_KEY` is not set or is shorter than 32 bytes
- [ ] The encryption key is not logged at any log level, including debug
- [ ] The encryption key is not exposed via any API endpoint, health check, or diagnostic output
- [ ] A key rotation procedure is documented that re-encrypts all secrets with a new key without downtime (can be a CLI tool)

#### REQ-SEC-003 — Database Backup Encryption

All database backups produced by or for the Tackle framework MUST be encrypted.

**Acceptance Criteria:**
- [ ] Backup documentation specifies that `pg_dump` output must be encrypted (e.g., piped through `gpg` or `age`) before storage
- [ ] The backup encryption key is managed separately from the application encryption key
- [ ] Unencrypted backup files are never written to persistent storage

### 3.2 Encryption in Transit

#### REQ-SEC-004 — TLS for All Network Communication

All network communication between Tackle components MUST use TLS 1.2 or higher.

**Scope:**
- Browser to framework admin UI (HTTPS)
- API client to framework REST API (HTTPS)
- Framework server to phishing endpoints (mutually authenticated TLS)
- Phishing endpoints to external SMTP servers (STARTTLS or implicit TLS)
- Framework server to external provider APIs (HTTPS — enforced by provider SDKs)
- Framework server to PostgreSQL (TLS with `sslmode=require` or higher)

**Acceptance Criteria:**
- [ ] The framework server does not serve HTTP on any port (only HTTPS, or HTTP with mandatory redirect to HTTPS)
- [ ] TLS 1.0 and 1.1 are explicitly disabled in the server TLS configuration
- [ ] The framework-to-endpoint channel uses mutual TLS (mTLS) with certificates generated per endpoint deployment
- [ ] Database connections enforce TLS via connection string parameter

#### REQ-SEC-005 — Certificate Management for Framework-Endpoint Communication

The framework MUST generate a unique TLS certificate pair (CA-signed by an internal Tackle CA) for each phishing endpoint deployment. These certificates are used for mTLS between the framework server and the endpoint.

**Acceptance Criteria:**
- [ ] Each endpoint deployment receives a unique client certificate and the framework CA certificate
- [ ] The framework validates the endpoint's client certificate on every API call from the endpoint
- [ ] Certificates are revoked (added to an internal revocation list) when an endpoint is terminated
- [ ] Private keys for endpoint certificates are encrypted at rest in the database (REQ-SEC-001)

### 3.3 Authentication Security

#### REQ-SEC-006 — Password Hashing

All local account passwords MUST be hashed using bcrypt with a minimum cost factor of 12, or Argon2id with parameters meeting OWASP recommendations (minimum: 19 MiB memory, 2 iterations, 1 parallelism).

**Acceptance Criteria:**
- [ ] The password hashing algorithm is configurable (bcrypt or Argon2id) with bcrypt as the default
- [ ] Bcrypt cost factor is configurable with a minimum of 12
- [ ] Argon2id parameters are configurable with minimums enforced
- [ ] Plaintext passwords are never stored, logged, or returned in API responses
- [ ] Password comparison uses constant-time functions to prevent timing attacks

#### REQ-SEC-007 — Account Lockout

The system MUST lock user accounts after a configurable number of consecutive failed authentication attempts.

**Acceptance Criteria:**
- [ ] Per-account lockout threshold is configurable (default: 5 failed attempts)
- [ ] Lockout duration is configurable (default: 15 minutes)
- [ ] Per-IP rate limiting is also enforced (default: 10 failed attempts per minute, 5-minute lockout)
- [ ] Successful login resets the failure counter
- [ ] Lockout events are recorded in the audit log with IP, username, and timestamp
- [ ] The initial administrator account is exempt from permanent lockout (REQ-AUTH-003)

#### REQ-SEC-008 — Session Token Security

Session tokens (refresh tokens, access tokens) MUST follow secure token handling practices.

**Acceptance Criteria:**
- [ ] Refresh tokens are stored in `httpOnly`, `Secure`, `SameSite=Strict` cookies for browser clients
- [ ] Access tokens have a maximum lifetime of 60 minutes (configurable, default 15 minutes)
- [ ] Session tokens are rotated on any privilege-level change (role change, permission modification)
- [ ] All active sessions for a user are invalidated when their password is changed
- [ ] Refresh token rotation is enforced — reuse of an old token invalidates all tokens for that user (see REQ-AUTH-071)

#### REQ-SEC-009 — CSRF Protection

All state-changing API endpoints MUST be protected against Cross-Site Request Forgery (CSRF) attacks.

**Acceptance Criteria:**
- [ ] The framework implements the synchronizer token pattern or double-submit cookie pattern for CSRF protection
- [ ] All `POST`, `PUT`, `PATCH`, and `DELETE` endpoints validate the CSRF token
- [ ] CSRF tokens are bound to the user session and rotated on login
- [ ] OIDC/OAuth2 flows use the `state` parameter for CSRF protection (see REQ-AUTH-031)

### 3.4 API Security

#### REQ-SEC-010 — Input Validation and Sanitization

All API endpoints MUST validate and sanitize input data.

**Acceptance Criteria:**
- [ ] Every API endpoint defines an explicit input schema (Go struct with validation tags)
- [ ] Request body size is limited (configurable, default: 10 MB for standard endpoints, 50 MB for file upload endpoints)
- [ ] String inputs are validated for maximum length, character set, and format where applicable
- [ ] All validation errors return structured error responses with field-level detail (without revealing internal implementation)
- [ ] No user input is passed to shell commands, system calls, or file system operations without sanitization

#### REQ-SEC-011 — SQL Injection Prevention

All database queries MUST use parameterized queries or prepared statements. String concatenation for SQL query construction is explicitly prohibited.

**Acceptance Criteria:**
- [ ] The Go codebase uses parameterized queries exclusively (via `database/sql` placeholders or an ORM/query builder that enforces parameterization)
- [ ] Code review checklist includes SQL injection verification
- [ ] No raw SQL string concatenation with user-supplied values exists anywhere in the codebase
- [ ] The `.clinerules` file explicitly prohibits string-concatenated SQL in delegated code

#### REQ-SEC-012 — Cross-Site Scripting (XSS) Prevention

All API responses that include user-supplied data MUST apply appropriate output encoding to prevent XSS.

**Acceptance Criteria:**
- [ ] API responses use `Content-Type: application/json` with proper JSON encoding (which inherently escapes HTML entities)
- [ ] Any endpoint that returns HTML content (e.g., landing page previews) applies contextual output encoding
- [ ] The React admin UI uses JSX (which auto-escapes by default) and does not use `dangerouslySetInnerHTML` without explicit sanitization
- [ ] Content Security Policy (CSP) headers are set on the admin UI to restrict inline scripts and external resource loading

#### REQ-SEC-013 — Rate Limiting

All API endpoints MUST enforce rate limiting to prevent abuse and denial-of-service.

**Acceptance Criteria:**
- [ ] Rate limits are configurable per endpoint category (authentication, standard CRUD, bulk operations, file uploads)
- [ ] Default rate limits: authentication endpoints — 10 requests/minute per IP; standard API — 100 requests/minute per user; bulk operations — 10 requests/minute per user
- [ ] Rate limit exceeded responses return HTTP `429 Too Many Requests` with a `Retry-After` header
- [ ] Rate limit state is maintained in-memory (or Redis if deployed) and survives individual request failures

#### REQ-SEC-014 — Correlation IDs and Audit Trail

Every API request MUST be assigned a unique correlation ID that is propagated through all downstream operations for that request.

**Acceptance Criteria:**
- [ ] A `X-Correlation-ID` header is generated for each request (or accepted from the client if provided)
- [ ] The correlation ID appears in all log entries, audit log records, and error responses generated during the request lifecycle
- [ ] Correlation IDs are UUIDs (v4) to prevent guessing or enumeration

#### REQ-SEC-015 — No Sensitive Data in URLs

Sensitive data MUST NOT appear in URLs, query parameters, or URL path segments.

**Acceptance Criteria:**
- [ ] Authentication tokens are never sent as query parameters (use `Authorization` header or cookies)
- [ ] Captured credential data is never included in URL paths
- [ ] API endpoints that retrieve sensitive data use `POST` with request body rather than `GET` with query parameters where necessary
- [ ] Server access logs do not contain sensitive data (URLs are logged, so sensitive data in URLs would be logged)

### 3.5 Data Protection

#### REQ-SEC-016 — Captured Credential Access Control

Captured credentials from phishing campaigns MUST be accessible only to users with the `credentials:read` permission (Operator and Admin roles by default).

**Acceptance Criteria:**
- [ ] Captured credentials are not returned in campaign summary endpoints — a separate, explicitly authorized endpoint is required
- [ ] Credential values are not displayed by default in the UI; the user must click an explicit "Reveal" action
- [ ] Every credential view or export action generates an audit log entry identifying the user, timestamp, and which credentials were accessed
- [ ] The Defender role cannot access captured credentials under any circumstances (REQ-RBAC-004)
- [ ] Credential export (CSV, JSON) is gated by the `credentials:export` permission and logged

#### REQ-SEC-017 — No Plaintext Secrets in Logs

Application logs MUST NOT contain plaintext secrets at any log level.

**Acceptance Criteria:**
- [ ] SMTP passwords, API keys, cloud credentials, captured credentials, encryption keys, and JWT signing keys are never logged
- [ ] Log sanitization is applied at the logging framework level (not per-callsite) to catch accidental leakage
- [ ] Structured log fields containing potentially sensitive data are automatically redacted (e.g., any field named `password`, `secret`, `token`, `key`, `credential`)
- [ ] A test exists that scans log output during integration tests for patterns matching known secret formats

#### REQ-SEC-018 — Audit Log Integrity

The audit log MUST resist tampering by application-level attackers (including compromised admin accounts).

**Acceptance Criteria:**
- [ ] Audit log entries are append-only — no API endpoint exists to modify or delete audit log records
- [ ] Audit log entries include a chained hash (each entry includes the hash of the previous entry) to detect retroactive insertion or deletion
- [ ] The database user used by the application has `INSERT` and `SELECT` permissions on the audit log table but not `UPDATE` or `DELETE`
- [ ] Audit logs are included in database backups and subject to the same encryption requirements (REQ-SEC-003)

#### REQ-SEC-019 — Insider Threat Mitigation

The system MUST implement controls to limit the impact of a compromised or malicious team member.

**Acceptance Criteria:**
- [ ] Role-based access control limits each user to the minimum permissions required for their function (see 02-authentication-authorization.md)
- [ ] All privileged actions (user management, infrastructure provisioning, credential access) are audited
- [ ] Credential export triggers a notification to all Admin users
- [ ] Session duration limits ensure that stolen tokens expire quickly (default: 15-minute access tokens)
- [ ] Administrators can force-terminate any user's sessions immediately

---

## 4. Security Requirements — Operational Security (OpSec)

### 4.1 Landing Page Anti-Fingerprinting

#### REQ-SEC-020 — Unique Landing Page Builds

Every campaign landing page deployment MUST produce output that is structurally unique, resisting signature-based detection.

**`[CLINE-DELEGATE]`** — The anti-fingerprinting engine (randomized HTML/CSS/JS generation) is delegated to the local LLM for implementation.

**Acceptance Criteria:**
- [ ] No two landing page builds produce identical HTML, CSS, or JavaScript output
- [ ] CSS class names, element IDs, and JavaScript variable names are randomized per build
- [ ] HTML structure (nesting depth, element order of non-visual elements, whitespace) varies per build
- [ ] Static analysis of two builds from the same template must not produce a stable signature (fuzzy hash similarity below 30%)
- [ ] The visual rendering of the page remains identical despite structural differences

#### REQ-SEC-021 — No Framework-Identifying Strings

No target-facing content (landing pages, emails, endpoint HTTP responses) MUST contain strings that identify the Tackle framework.

**Acceptance Criteria:**
- [ ] The strings "tackle", "Tackle", "TACKLE" do not appear in any landing page HTML/CSS/JS output
- [ ] No comments, metadata, or hidden fields reference the framework
- [ ] HTTP response headers from phishing endpoints do not include `X-Powered-By`, `Server`, or any custom header that identifies the framework
- [ ] Error pages served by phishing endpoints do not reveal the framework identity or version

### 4.2 Phishing Endpoint Binary Security

#### REQ-SEC-022 — Per-Deployment Binary Compilation

The phishing endpoint binary MUST be compiled fresh for each deployment, producing a unique binary hash.

**`[CLINE-DELEGATE]`** — The phishing endpoint transparent proxy binary is delegated to the local LLM for implementation.

**Acceptance Criteria:**
- [ ] Each compilation produces a binary with a different SHA-256 hash than previous compilations
- [ ] Compilation incorporates build-time randomization (e.g., randomized string literals, build ID, nonce embedded in binary)
- [ ] The binary does not contain debug symbols or source file paths in release builds
- [ ] The binary does not contain the string "tackle" or any framework-identifying marker
- [ ] Binary stripping (`-ldflags "-s -w"`) is applied by default

#### REQ-SEC-023 — Randomized HTTP Response Headers

Phishing endpoint HTTP responses MUST use randomized or configurable headers to avoid fingerprinting.

**Acceptance Criteria:**
- [ ] The `Server` header is either omitted or set to a common, benign value (e.g., "nginx", "Apache", "Microsoft-IIS") configurable per deployment
- [ ] Response header ordering varies across deployments
- [ ] No custom `X-` headers that are consistent across deployments are included in target-facing responses
- [ ] TLS cipher suite preference order is configurable to match common server profiles

### 4.3 Email OpSec

#### REQ-SEC-024 — Email Header Sanitization

All outbound phishing emails MUST have their headers sanitized to remove any traces of the Tackle framework.

**`[CLINE-DELEGATE]`** — Email header sanitization for OpSec is delegated to the local LLM for implementation.

**Acceptance Criteria:**
- [ ] No `X-Mailer`, `X-Originating-IP`, or `User-Agent` headers reference the Tackle framework or its internal infrastructure
- [ ] The `Received` header chain does not include the framework server's hostname or internal IP
- [ ] `Message-ID` domain portions match the sending domain (not the framework server domain)
- [ ] MIME boundary strings are randomized and do not follow a predictable pattern
- [ ] Header ordering mimics common mail clients (configurable: Outlook, Gmail, Apple Mail profiles)

#### REQ-SEC-025 — Tracking Pixel Implementation

Tracking pixels embedded in emails MUST be implemented to resist detection by email security gateways.

**`[CLINE-DELEGATE]`** — Tracking pixel implementation is delegated to the local LLM for implementation.

**Acceptance Criteria:**
- [ ] Tracking pixel URLs are unique per target and per campaign
- [ ] The pixel URL path does not follow a predictable pattern (randomized path components)
- [ ] The pixel response returns a valid 1x1 transparent image with correct `Content-Type` header
- [ ] The pixel endpoint supports both GIF and PNG formats
- [ ] The pixel URL domain matches the campaign's phishing domain (no third-party tracking domains)

### 4.4 DNS OpSec

#### REQ-SEC-026 — DNS Pattern Avoidance

DNS records associated with campaigns MUST NOT follow detectable patterns.

**Acceptance Criteria:**
- [ ] Domain names used across campaigns do not share a recognizable naming convention (no sequential numbering, no common prefix/suffix)
- [ ] DNS record TTL values vary across campaigns and domains (not all set to the same value)
- [ ] Domains are registered across multiple registrars and use different DNS providers where possible
- [ ] A and AAAA records point to phishing endpoints in different cloud regions and providers across campaigns (as supported by infrastructure availability)
- [ ] The framework provides guidance (UI warning or documentation) when an operator reuses patterns that could be correlated

### 4.5 Infrastructure OpSec

#### REQ-SEC-027 — Multi-Provider and Multi-Region Deployment

The framework MUST support deploying phishing endpoints across multiple cloud providers and regions to resist infrastructure correlation.

**Acceptance Criteria:**
- [ ] Campaigns can use endpoints deployed on different cloud providers (AWS and Azure)
- [ ] Campaigns can use endpoints deployed in different regions within the same provider
- [ ] The instance template system (REQ-INFRA-006) supports templates for multiple providers and regions
- [ ] The framework does not enforce any default that concentrates all endpoints in a single provider or region

#### REQ-SEC-028 — Network Traffic Obfuscation

Communication between the framework server and phishing endpoints MUST resist traffic pattern analysis.

**Acceptance Criteria:**
- [ ] Framework-to-endpoint communication uses standard HTTPS (port 443) to blend with normal web traffic
- [ ] Heartbeat/health check intervals are configurable and support jitter (randomized variation) to avoid predictable timing patterns
- [ ] The communication protocol does not use custom ports or protocols that would stand out in network traffic analysis
- [ ] Payload sizes are padded to avoid distinctive content-length patterns where feasible

---

## 5. Offensive Code Delegation (Cline)

Certain implementation tasks involve offensive security techniques that require specialized domain knowledge. These tasks are delegated to the local LLM (Cline) for implementation. Each delegated item is marked with `[CLINE-DELEGATE]` in the requirement where it appears and is accompanied by a ready-to-paste prompt below.

### 5.1 Inventory of Cline-Delegated Items

| ID | Item | Source Requirement | Primary Document |
|----|------|--------------------|-----------------|
| CLINE-01 | Anti-fingerprinting engine (randomized HTML/CSS/JS generation) | REQ-SEC-020 | This document (15) and 05-landing-page-builder.md |
| CLINE-02 | Phishing endpoint transparent proxy binary | REQ-SEC-022 | This document (15) and 07-phishing-endpoints.md |
| CLINE-03 | Email header sanitization for OpSec | REQ-SEC-024 | This document (15) and 04-email-smtp.md |
| CLINE-04 | Credential capture form interception | — | 08-credential-capture.md |
| CLINE-05 | Tracking pixel implementation | REQ-SEC-025 | This document (15) and 04-email-smtp.md |
| CLINE-06 | SMTP relay with header manipulation | — | 04-email-smtp.md and 07-phishing-endpoints.md |

### 5.2 Cline Prompts

#### CLINE-01: Anti-Fingerprinting Engine

```
You are implementing the anti-fingerprinting engine for a phishing simulation platform called Tackle. This is an authorized red team tool used in a private lab.

TASK:
Build a Go package at `tackle/internal/antifingerprint` that takes a compiled React landing page (HTML + CSS + JS bundle) and produces a structurally unique variant on every invocation.

REQUIREMENTS:
- Randomize all CSS class names to unique strings (6-10 char alphanumeric). Maintain a mapping and update all references in HTML and JS.
- Randomize all JavaScript variable and function names (except browser-provided globals). Use AST-level transformation, not regex replacement.
- Randomize HTML element IDs and `data-*` attribute names. Update all CSS and JS references accordingly.
- Vary non-visual HTML structure: insert benign invisible `<span>` or `<div>` elements with randomized attributes at random insertion points. Vary whitespace and indentation patterns.
- Randomize CSS property ordering within each rule block.
- Inject decoy CSS rules targeting the inserted invisible elements.
- Randomize the order of `<script>` and `<style>` blocks where reordering is safe (no dependency ordering violations).
- Randomize MIME boundary strings if content is served as multipart.
- The visual rendering of the page MUST remain pixel-identical to the original.
- The function signature should be: `func Transform(htmlContent []byte, cssContent []byte, jsContent []byte) ([]byte, []byte, []byte, error)`

CONSTRAINTS:
- Go 1.21+, standard library preferred. Minimal external dependencies.
- All import paths must be local (`tackle/internal/...`), no `github.com` imports.
- Write clean, self-documenting code with thorough comments explaining each transformation.
- Include unit tests that verify: (1) output differs between invocations, (2) all original CSS classes/IDs are replaced, (3) no "tackle" string appears in output.
- Do NOT include any framework-identifying strings ("tackle", "Tackle", etc.) in the output content. The package name and import path may reference tackle but the generated output must not.
```

#### CLINE-02: Phishing Endpoint Transparent Proxy Binary

```
You are implementing the phishing endpoint binary for a phishing simulation platform called Tackle. This is an authorized red team tool used in a private lab.

TASK:
Build a Go binary at `tackle/cmd/endpoint` that acts as a transparent reverse proxy deployed to cloud VMs (AWS EC2 / Azure). The binary proxies HTTPS traffic from targets to the framework server's landing page application, making the framework server invisible to the target.

REQUIREMENTS:
- TLS termination on port 443 using a provided certificate and key (paths via CLI flags or environment variables).
- Transparent reverse proxy: all HTTPS requests from the target are forwarded to the framework server's landing page app (configurable upstream URL). Responses are relayed back without modification except for header sanitization.
- Remove or rewrite hop-by-hop headers. Strip any headers that reveal the framework server's identity.
- Add configurable `Server` response header (default: omit). Support preset profiles: "nginx", "apache", "iis".
- Randomize response header ordering on each response.
- Health check endpoint on a separate management port (configurable, default 8443) protected by mTLS. Endpoint: `GET /healthz` returns 200 with JSON `{"status":"ok"}`.
- Graceful shutdown on SIGTERM/SIGINT: drain active connections for up to 30 seconds.
- Structured JSON logging to stdout. No framework-identifying strings in any log output or error message that could be seen by a target.
- Build-time nonce: embed a random string at build time (`-ldflags "-X main.buildNonce=..."`) so each compilation produces a unique binary hash.
- Strip debug symbols and source paths in release builds (`-ldflags "-s -w"`).

CONSTRAINTS:
- Go 1.21+, standard library only (net/http, crypto/tls, etc.). No external dependencies.
- All import paths must be local (`tackle/internal/...`), no `github.com` imports.
- Write clean, self-documenting code with thorough comments.
- Include unit tests for: proxy behavior, header sanitization, health check endpoint, and graceful shutdown.
- The compiled binary must not contain the string "tackle" in any user-visible output. Internal package paths in the binary are acceptable.
```

#### CLINE-03: Email Header Sanitization

```
You are implementing email header sanitization for a phishing simulation platform called Tackle. This is an authorized red team tool used in a private lab.

TASK:
Build a Go package at `tackle/internal/email/sanitize` that processes outbound email headers before SMTP transmission, removing or replacing any headers that could identify the framework or its infrastructure.

REQUIREMENTS:
- Remove headers: `X-Mailer`, `X-Originating-IP`, `X-Originating-Email`, `X-Source`, `X-Source-Args`, `X-Source-Dir`, any header containing the framework server hostname or internal IP range.
- Rewrite `Received` headers: remove any entries that reference the framework server. The first `Received` header should appear to originate from the phishing endpoint, not the framework.
- Ensure `Message-ID` domain portion matches the configured sending domain.
- Randomize MIME boundary strings using cryptographically secure random generation. Boundary format should mimic common mail clients.
- Support configurable header ordering profiles that mimic specific mail clients:
  - "outlook": From, To, Subject, Date, Message-ID, MIME-Version, Content-Type, ...
  - "gmail": MIME-Version, Date, From, Message-ID, Subject, To, Content-Type, ...
  - "apple_mail": From, Content-Type, MIME-Version, Subject, Message-ID, Date, To, ...
- Strip any `X-` headers not explicitly whitelisted by the campaign configuration.
- Validate that the final header set does not contain any string from a configurable blocklist (default: framework server hostname, internal IPs, "tackle").

FUNCTION SIGNATURE:
`func Sanitize(headers map[string][]string, config SanitizeConfig) (map[string][]string, error)`

CONSTRAINTS:
- Go 1.21+, standard library only (net/mail, strings, crypto/rand, etc.).
- All import paths must be local (`tackle/internal/...`), no `github.com` imports.
- Write clean, self-documenting code with thorough comments.
- Include unit tests verifying: (1) framework-identifying headers are removed, (2) Message-ID domain is correct, (3) header ordering matches the selected profile, (4) MIME boundaries are random and unique per call, (5) blocklisted strings are absent from the output.
```

#### CLINE-04: Credential Capture Form Interception

```
You are implementing credential capture form interception for a phishing simulation platform called Tackle. This is an authorized red team tool used in a private lab.

TASK:
Build a JavaScript module (ES module) at `tackle/internal/landingpage/assets/capture.js` that intercepts form submissions on a phishing landing page and transmits the captured data to the Tackle framework via the phishing endpoint.

REQUIREMENTS:
- Attach event listeners to all `<form>` elements on the page (and any dynamically added forms via MutationObserver).
- On form submission, capture all form field name-value pairs.
- Transmit captured data to the phishing endpoint via a `POST` request to a configurable callback URL path (default: `/api/c`).
- The POST body should be JSON: `{"fields": {"username": "...", "password": "..."}, "url": "<current page URL>", "ts": <unix timestamp>, "tk": "<tracking token>"}`.
- The tracking token (`tk`) is embedded in the page by the framework at build time (unique per target).
- After successful transmission, allow the original form action to proceed (redirect the user to the legitimate site or a configured redirect URL) so the target does not notice the interception.
- If the callback request fails, still allow the form submission to proceed (fail open — do not alert the target).
- The JavaScript must be minified and obfuscated before deployment.
- Variable names, function names, and the callback URL path must all be randomized per-build by the anti-fingerprinting engine (CLINE-01).

CONSTRAINTS:
- Vanilla JavaScript (ES2020+). No frameworks, no npm dependencies.
- The script must work in all modern browsers (Chrome, Firefox, Edge, Safari — last 2 major versions).
- Write clean, well-commented source code (obfuscation is applied as a build step, not in the source).
- Include unit tests (using jsdom or similar) that verify: (1) form interception captures all fields, (2) callback POST is sent with correct payload, (3) original form submission proceeds after capture, (4) failure of callback does not block form submission.
- No string in the source or output may contain "tackle" or any framework-identifying marker.
```

#### CLINE-05: Tracking Pixel Implementation

```
You are implementing a tracking pixel system for a phishing simulation platform called Tackle. This is an authorized red team tool used in a private lab.

TASK:
Build a Go HTTP handler at `tackle/internal/tracking/pixel.go` that serves unique tracking pixels embedded in phishing emails and records email open events.

REQUIREMENTS:
- Serve a 1x1 transparent image at a configurable URL path (default: randomized per campaign, e.g., `/img/<random>/<token>.gif`).
- Support both GIF and PNG formats (based on file extension in the URL or `Accept` header).
- The URL path must include a per-target tracking token that maps to a target ID and campaign ID.
- On each request, record an open event: target ID, campaign ID, timestamp, source IP, User-Agent, and the tracking token.
- Return appropriate `Cache-Control` headers (`no-cache, no-store, must-revalidate`) to prevent caching that would suppress repeated open tracking.
- The response must include correct `Content-Type` (`image/gif` or `image/png`) and `Content-Length` headers.
- The tracking pixel image bytes must be valid (a real 1x1 transparent GIF89a or PNG, not just empty bytes).
- URL path structure must be randomized per campaign deployment (the path prefix is configured at deployment time, not hardcoded).
- No framework-identifying strings in response headers or response body.

FUNCTION SIGNATURE:
Handler constructor: `func NewPixelHandler(store EventStore, config PixelConfig) http.Handler`

CONSTRAINTS:
- Go 1.21+, standard library only (net/http, image, image/gif, image/png, etc.).
- All import paths must be local (`tackle/internal/...`), no `github.com` imports.
- Write clean, self-documenting code with thorough comments.
- Include unit tests verifying: (1) valid GIF and PNG responses, (2) open event is recorded with correct data, (3) cache headers prevent caching, (4) invalid tokens return a valid image (fail gracefully — do not return errors that reveal the tracking system).
```

#### CLINE-06: SMTP Relay with Header Manipulation

```
You are implementing the SMTP relay module for the phishing endpoint binary of a phishing simulation platform called Tackle. This is an authorized red team tool used in a private lab.

TASK:
Build a Go package at `tackle/internal/endpoint/smtprelay` that receives campaign email payloads from the framework server and transmits them to external SMTP servers, applying header manipulation for operational security.

REQUIREMENTS:
- Accept email payloads from the framework server over the mTLS management channel (internal API, not exposed to targets).
- Each payload contains: rendered email (RFC 5322 message), SMTP server credentials, envelope sender, envelope recipient, and campaign configuration.
- Open connections to the configured external SMTP server(s) with support for: no auth, PLAIN, LOGIN, CRAM-MD5, and XOAUTH2 authentication; STARTTLS, implicit TLS, and no-TLS connection modes.
- Before sending, pass the email headers through the sanitization module (CLINE-03) to remove framework traces.
- Apply DKIM signing using the campaign's private key (provided in the payload). Use the `crypto/rsa` package for RSA-SHA256 signing.
- Implement connection pooling: maintain a configurable number of persistent connections per SMTP server (default: 5).
- Implement rate limiting per SMTP profile: respect the `max_send_rate` from the SMTP profile configuration.
- Report delivery status (sent, deferred, bounced, failed) back to the framework server for each email via the mTLS channel.
- Support graceful draining: when a campaign is paused or stopped, finish in-progress sends but do not start new ones.

CONSTRAINTS:
- Go 1.21+, standard library only (net/smtp, crypto/tls, crypto/rsa, etc.).
- All import paths must be local (`tackle/internal/...`), no `github.com` imports.
- Write clean, self-documenting code with thorough comments.
- Include unit tests using a mock SMTP server that verify: (1) emails are sent with sanitized headers, (2) DKIM signature is present and valid, (3) rate limiting is respected, (4) connection pooling reuses connections, (5) delivery status is reported correctly.
- No framework-identifying strings in any SMTP transaction (EHLO hostname should be configurable, defaulting to the phishing endpoint's reverse DNS).
```

---

## 6. `.clinerules` Specification

The project MUST maintain a `.clinerules` file at the repository root that guides the local LLM (Cline) on project conventions, security requirements, and code quality standards. This file is referenced by all Cline-delegated prompts and ensures consistency across delegated implementations.

### REQ-SEC-029 — `.clinerules` File Contents

The `.clinerules` file MUST include the following sections:

**Project Conventions:**
- Language: Go 1.21+ for backend, TypeScript/React for frontend
- All Go import paths must be local (`tackle/internal/...`, `tackle/cmd/...`, `tackle/pkg/...`) — no `github.com` in import paths
- Follow standard Go project layout (`cmd/`, `internal/`, `pkg/`)
- Use `gofmt` formatting; no custom style overrides
- Error handling: always return errors, never panic in library code
- Naming: follow Go conventions (exported names are PascalCase, unexported are camelCase)

**Security Requirements:**
- All secrets must be encrypted at rest using AES-256-GCM
- Never log plaintext secrets at any log level
- Never hardcode secrets, API keys, or credentials in source code
- All SQL queries must use parameterized queries — no string concatenation
- All user input must be validated and sanitized before use
- No framework-identifying strings ("tackle", "Tackle") in any target-facing output
- All network communication must use TLS 1.2+

**Code Quality Standards:**
- Code must be self-documenting with clear, descriptive names
- All exported functions must have Go doc comments
- Complex logic must include inline comments explaining the "why"
- Every package must have unit tests with at least 80% line coverage
- Tests must include both positive and negative cases
- Error messages must be descriptive but not reveal internal implementation details to end users

**Delegated Code Constraints:**
- Generated code must compile and pass tests before delivery
- Generated code must not introduce any external dependencies
- Generated code must not include TODO/FIXME/HACK comments — complete the implementation fully
- Generated code must handle all error paths explicitly
- Generated code must not use `os.Exit()` or `log.Fatal()` in library packages (only in `main`)

**Acceptance Criteria:**
- [ ] The `.clinerules` file exists at the repository root
- [ ] The file covers all four sections listed above
- [ ] All Cline-delegated prompts reference the `.clinerules` file as the project convention guide
- [ ] The file is version-controlled and updated when project conventions change

---

## 7. Acceptance Criteria — Summary

### Framework Security

- [ ] All secrets listed in REQ-SEC-001 are encrypted at rest with AES-256-GCM
- [ ] The application refuses to start without a valid `TACKLE_ENCRYPTION_KEY` environment variable
- [ ] All inter-component communication uses TLS 1.2+ (REQ-SEC-004)
- [ ] mTLS is enforced between the framework server and phishing endpoints (REQ-SEC-005)
- [ ] Password hashing uses bcrypt (cost 12+) or Argon2id with OWASP-recommended parameters (REQ-SEC-006)
- [ ] Account lockout is enforced after configurable failed attempts (REQ-SEC-007)
- [ ] Session tokens use httpOnly/Secure/SameSite cookies and rotate on privilege changes (REQ-SEC-008)
- [ ] CSRF protection is active on all state-changing endpoints (REQ-SEC-009)
- [ ] All API inputs are validated with explicit schemas and size limits (REQ-SEC-010)
- [ ] No SQL string concatenation exists in the codebase (REQ-SEC-011)
- [ ] XSS prevention via output encoding and CSP headers is in place (REQ-SEC-012)
- [ ] Rate limiting is enforced on all API endpoints (REQ-SEC-013)
- [ ] Every API request carries a correlation ID through all operations (REQ-SEC-014)
- [ ] No sensitive data appears in URLs or query parameters (REQ-SEC-015)
- [ ] Captured credentials require explicit action to view and are audit-logged (REQ-SEC-016)
- [ ] No plaintext secrets appear in logs at any level (REQ-SEC-017)
- [ ] Audit log is append-only with chained hashing for integrity (REQ-SEC-018)
- [ ] Insider threat controls (RBAC, audit notifications, session limits) are in place (REQ-SEC-019)

### Operational Security

- [ ] Landing page builds are structurally unique per deployment (REQ-SEC-020)
- [ ] No framework-identifying strings in target-facing content (REQ-SEC-021)
- [ ] Phishing endpoint binaries are compiled fresh per deployment with unique hashes (REQ-SEC-022)
- [ ] HTTP response headers from endpoints are randomized (REQ-SEC-023)
- [ ] Email headers are sanitized to remove framework traces (REQ-SEC-024)
- [ ] Tracking pixels use randomized paths and per-target unique tokens (REQ-SEC-025)
- [ ] DNS records do not follow detectable patterns across campaigns (REQ-SEC-026)
- [ ] Multi-provider/multi-region deployment is supported (REQ-SEC-027)
- [ ] Framework-endpoint communication resists traffic pattern analysis (REQ-SEC-028)

### Cline Integration

- [ ] All six Cline-delegated items have ready-to-paste prompts in this document
- [ ] The `.clinerules` file exists at the repo root with all required sections (REQ-SEC-029)
- [ ] Delegated code output is reviewed and integrated by the team before merging

---

## 8. Dependencies

| Dependency | Document | Relationship |
|------------|----------|-------------|
| **System Overview** | [01-system-overview.md](01-system-overview.md) | Defines the encryption key management model and architectural principles that security requirements build upon |
| **Authentication & Authorization** | [02-authentication-authorization.md](02-authentication-authorization.md) | Defines RBAC, session management, and authentication flows referenced by framework security requirements |
| **Domain & Infrastructure** | [03-domain-infrastructure.md](03-domain-infrastructure.md) | Infrastructure provisioning and DNS management must follow OpSec requirements for multi-provider deployment and pattern avoidance |
| **Email & SMTP** | [04-email-smtp.md](04-email-smtp.md) | Email header sanitization, DKIM signing, and SMTP relay security are governed by OpSec requirements here |
| **Landing Page Builder** | [05-landing-page-builder.md](05-landing-page-builder.md) | Anti-fingerprinting engine applies to landing page output; detailed landing page OpSec covered there and here |
| **Campaign Management** | [06-campaign-management.md](06-campaign-management.md) | Campaign lifecycle must enforce security checks (email auth validation, endpoint health) before launch |
| **Phishing Endpoints** | [07-phishing-endpoints.md](07-phishing-endpoints.md) | Endpoint binary security, header randomization, and proxy behavior are specified here and implemented in the endpoint |
| **Credential Capture** | [08-credential-capture.md](08-credential-capture.md) | Credential access control, encryption, and audit logging requirements originate here |
| **Audit Logging** | [11-audit-logging.md](11-audit-logging.md) | Audit log integrity requirements (append-only, chained hashing) extend the audit logging specification |
| **Database Schema** | [14-database-schema.md](14-database-schema.md) | Encryption at rest applies to specific columns; audit log table permissions are defined here |
