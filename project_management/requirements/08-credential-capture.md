# 08 — Credential Capture

## 1. Overview

Credential capture is a core operational capability of the Tackle platform. When phishing targets interact with landing page forms, all submitted data — usernames, passwords, MFA tokens, security question answers, and any other form fields — is captured, transmitted back to the framework server, and stored securely in PostgreSQL with encryption at rest.

The capture mechanism is designed to be invisible to the target. From the target's perspective, the form submission behaves normally: the page may redirect to a legitimate login portal, display a "session expired" message, or perform any other configurable post-submission action.

Credential capture ties directly into campaign management, feeding real-time data to the operator dashboard, populating campaign metrics, and providing the raw material for post-campaign reporting.

## 2. Capture Flow

```
Target's Browser
      │
      │  1. Target visits phishing URL
      ▼
┌─────────────────────────┐
│   Phishing Endpoint     │
│   (transparent proxy)   │
│   - TLS termination     │
│   - Forwards to         │
│     framework server    │
└───────────┬─────────────┘
            │  2. Proxied to landing page app
            ▼
┌─────────────────────────┐
│   Landing Page App      │
│   (React + Go, per-     │
│    campaign, framework  │
│    server)              │
│                         │
│   3. Target fills form  │
│   4. Form submission    │
│      intercepted        │
│   5. Captured data sent │
│      to Go backend      │
└───────────┬─────────────┘
            │  6. Data transmitted to framework API
            ▼
┌─────────────────────────┐
│   Tackle Framework      │
│   (Go Backend)          │
│                         │
│   7. Data validated &   │
│      normalized         │
│   8. Encrypted and      │
│      stored in          │
│      PostgreSQL         │
│   9. WebSocket event    │
│      pushed to          │
│      campaign dashboard │
│  10. Audit log entry    │
│      created            │
└───────────┬─────────────┘
            │
            ▼
┌─────────────────────────┐
│   Post-Capture Action   │
│   (configurable per     │
│    landing page)        │
│                         │
│   - Redirect to real    │
│     login page          │
│   - "Session expired"   │
│   - Custom HTML page    │
│   - Delayed redirect    │
└─────────────────────────┘
```

### 2.1 Detailed Step Sequence

1. **Target visits landing page** — The target clicks a link in the phishing email, which resolves to the phishing endpoint. The endpoint transparently proxies the request to the landing page app running on the framework server.
2. **Landing page renders** — The React-based landing page app serves the phishing page. The page is unique per campaign build (anti-fingerprinting).
3. **Target submits form** — The target enters credentials and submits the form.
4. **Form submission intercepted** — The landing page app's Go backend receives the form POST. All form fields are captured regardless of field name or type.
5. **Data transmitted to framework** — The landing page app sends the captured data to the Tackle framework backend API via an internal HTTP call.
6. **Framework processes capture** — The framework backend validates the submission, associates it with the correct campaign/target/template variant, encrypts sensitive fields, and persists to PostgreSQL.
7. **Real-time notification** — A WebSocket event is emitted to all connected operator sessions viewing the campaign dashboard.
8. **Post-capture action** — The landing page app executes the configured post-capture behavior (redirect, page swap, etc.) from the target's perspective.

## 3. Data Captured

### 3.1 Form Data

All form fields submitted by the target are captured. The system does not require predefined field names — it captures whatever the form contains. However, the following field categories receive special handling for reporting and analysis:

| Category | Examples | Handling |
|----------|----------|----------|
| **Username/email** | `username`, `email`, `login`, `user_id` | Flagged as identity field; used for target correlation |
| **Password** | `password`, `passwd`, `pass`, `pwd` | Flagged as sensitive; encrypted at rest; masked in UI by default |
| **MFA/OTP tokens** | `otp`, `mfa_code`, `token`, `verification_code` | Flagged as sensitive; time-sensitive notation added |
| **Custom fields** | Security questions, PINs, account numbers | Captured as generic key-value pairs |
| **Hidden fields** | CSRF tokens, session IDs, tracking values | Captured for completeness and analysis |

Field categorization is configurable per landing page template. Operators can define custom field-name-to-category mappings in the landing page builder.

### 3.2 Request Metadata

In addition to form data, each capture event records:

| Data Point | Description |
|------------|-------------|
| **Source IP address** | IP address of the target (as seen by the phishing endpoint) |
| **User-Agent** | Full `User-Agent` header string |
| **Accept-Language** | Browser language preferences |
| **Referer** | `Referer` header if present |
| **Other HTTP headers** | Configurable set of additional headers to capture |
| **Timestamp** | Server-side UTC timestamp of the capture event |
| **Submission sequence number** | Ordinal number for this target (1st attempt, 2nd attempt, etc.) |
| **Landing page URL path** | The specific URL path the target submitted from |
| **Request method** | POST, GET, or other HTTP method used |

### 3.3 Contextual Associations

Each capture event is linked to:

- **Campaign** — The campaign that generated the phishing email
- **Target** — The specific target individual (matched via tracking token in the URL)
- **Template variant** — For A/B testing, which landing page variant the target saw
- **Phishing endpoint** — The endpoint instance that proxied the request
- **Email send record** — The specific email delivery that led to this interaction

## 4. Storage and Security

### 4.1 Encryption at Rest

All captured credential data is encrypted before being written to PostgreSQL. The encryption scheme applies to form field values (especially passwords, tokens, and other sensitive fields). Request metadata (IP, User-Agent, timestamps) is stored in plaintext for efficient querying.

### 4.2 Access Control

Captured credentials are restricted by the platform's RBAC model:

| Role | Permission |
|------|-----------|
| **Admin** | Full access — view, decrypt, export, delete |
| **Operator** | View capture events, decrypt credentials (with audit), export |
| **Engineer** | View capture statistics only (counts, timestamps). No access to credential values |
| **Viewer** | View campaign-level aggregate statistics only. No access to individual capture events |

### 4.3 Credential Display Behavior

- Captured credentials are **never displayed in plaintext by default** in the UI
- Password and sensitive fields show as masked values (e.g., `••••••••`)
- Decryption requires an explicit operator action ("Reveal Credential" button or similar)
- Every reveal/decrypt action creates an audit log entry recording who viewed what and when
- Revealed values are displayed temporarily and re-masked after a configurable timeout or on navigation away

### 4.4 No Automated Credential Validation

The framework does **not** automatically test captured credentials against Active Directory, LDAP, or any other authentication system. Credential validation against live systems is an operator responsibility performed outside the Tackle platform. This boundary exists to limit the platform's attack surface and prevent unintended account lockouts.

### 4.5 Data Retention and Immutability

- When a campaign is **Active**, captured credentials can be viewed and exported
- When a campaign is **Archived**, credential data becomes **immutable** — no modifications, no deletions
- Credential data can only be permanently deleted by an Admin via an explicit purge action, which requires confirmation and creates an audit log entry
- Data retention policies are configurable per-organization (e.g., auto-purge after N days)

## 5. Capture in Campaign Context

### 5.1 Real-Time Dashboard Integration

- Each capture event triggers a WebSocket message to connected operator sessions
- The campaign dashboard displays a live feed of capture events (timestamp, target name/email, fields captured — values masked)
- Operators can see capture events as they happen without page refresh

### 5.2 Campaign Metrics Integration

Capture data feeds the following campaign metrics:

- **Capture rate** — Percentage of targets who submitted credentials vs. total targets
- **Capture rate by variant** — Per-template-variant capture rates (A/B testing analysis)
- **Time to capture** — Elapsed time between email delivery and credential submission
- **Repeat submission rate** — Percentage of targets who submitted more than once
- **Field completion rate** — Which fields targets filled vs. left empty
- **Capture timeline** — Time-series view of captures over the campaign duration

### 5.3 Reporting

Captured data is included in campaign reports with appropriate access controls:

- **Summary reports** — Aggregate statistics (capture rates, timelines) without credential values
- **Detailed reports** — Include credential values; require Operator/Admin role; report generation creates an audit log entry
- **Export formats** — CSV, JSON, and PDF; all exports create audit log entries
- **Redacted exports** — Export capture metadata without credential values for sharing with stakeholders who should not see raw credentials

### 5.4 A/B Testing Support

When a campaign uses multiple landing page variants:

- Each capture is tagged with the template variant identifier
- Metrics are computed per-variant for comparison
- Reports include variant-level breakdowns
- Operators can assess which landing page design yields higher capture rates

## 6. Post-Capture Behavior

After a successful credential capture, the landing page executes a configurable post-capture action:

| Action | Description |
|--------|-------------|
| **Redirect to URL** | HTTP redirect (302) to a specified URL (e.g., the real login page of the impersonated service) |
| **Display page** | Render an inline HTML page (e.g., "Your session has expired, please try again later") |
| **Redirect with delay** | Show a brief message, then redirect after N seconds |
| **Replay submission** | Forward the captured credentials to the real login endpoint so the target is logged in seamlessly (advanced, high-risk) |
| **No action** | Form appears to submit normally; page remains as-is |

The post-capture action is configured per landing page in the landing page builder.

## 7. Requirements

### Capture Mechanism

---

**REQ-CRED-001: Universal Form Field Capture**

The landing page app MUST capture all form fields submitted by the target, regardless of field name, type, or quantity. No prior knowledge of field names is required.

**Acceptance Criteria:**
- Given a landing page with any combination of form fields, when the target submits the form, then every field name and value is captured and transmitted to the framework.
- Hidden fields, disabled fields, and dynamically generated fields are all captured.
- File upload fields are excluded from capture (only text-based fields).

> **Cline Delegation Flag:** Form interception implementation — including JavaScript-level form hooking, field enumeration, and submission interception techniques — is a candidate for Cline delegation (offensive security technique).

---

**REQ-CRED-002: Form Submission Interception**

The landing page app MUST intercept form submissions transparently so that the target sees normal form behavior (loading indicators, page transitions) while data is exfiltrated to the framework backend.

**Acceptance Criteria:**
- Given a target submitting a form, the submission is intercepted before the browser performs its default action.
- The target experiences no visible errors, unexpected delays (> 2 seconds), or unusual behavior.
- The interception works across all modern browsers (Chrome, Firefox, Edge, Safari — latest two major versions).

> **Cline Delegation Flag:** Transparent form interception and data exfiltration to the framework backend — including anti-detection techniques and seamless UX preservation — is a candidate for Cline delegation (offensive security technique).

---

**REQ-CRED-003: Data Transmission to Framework**

The landing page app MUST transmit captured form data to the Tackle framework backend API via an authenticated internal HTTP call.

**Acceptance Criteria:**
- Given a captured form submission, the landing page app's Go backend sends the data to the framework API within 1 second.
- The transmission uses TLS encryption in transit.
- The request includes the campaign ID, target tracking token, and template variant identifier.
- If the framework API is unreachable, the landing page app queues the data locally and retries with exponential backoff (max 5 retries over 5 minutes).
- Failed transmissions are logged on the landing page app and eventually reconciled when connectivity is restored.

---

**REQ-CRED-004: Target Identification via Tracking Token**

Each capture event MUST be associated with the specific target by resolving the tracking token embedded in the URL the target visited.

**Acceptance Criteria:**
- Given a target clicking a phishing link containing a unique tracking token, when the form is submitted, the capture event is linked to the correct target record in the database.
- If the tracking token is missing or invalid, the capture event is still stored but flagged as "unattributed" for manual review.
- Tracking tokens are opaque (not reversible to target identity without database lookup).

> **Cline Delegation Flag:** Tracking token generation, embedding, and extraction techniques — designed to be undetectable to the target and resilient to URL manipulation — are candidates for Cline delegation (offensive security technique).

---

**REQ-CRED-005: Request Metadata Capture**

Each capture event MUST include HTTP request metadata as defined in Section 3.2.

**Acceptance Criteria:**
- Given a form submission, the capture event record includes: source IP, User-Agent, Accept-Language, Referer (if present), timestamp (UTC), submission sequence number, URL path, and HTTP method.
- Source IP reflects the target's actual IP address as seen at the phishing endpoint (not the framework server's internal IP).
- The set of additional HTTP headers to capture is configurable per landing page.

---

**REQ-CRED-006: Submission Sequence Tracking**

The system MUST track the number of form submissions per target per campaign and record the ordinal sequence number on each capture event.

**Acceptance Criteria:**
- Given a target who submits a form 3 times, the capture events are numbered 1, 2, and 3 respectively.
- The campaign dashboard displays the submission count per target.
- Repeated submissions from the same target are flagged in the UI for operator awareness.

---

### Storage and Security

---

**REQ-CRED-007: Encryption at Rest**

All captured form field values classified as sensitive (passwords, tokens, MFA codes) MUST be encrypted at rest in PostgreSQL using AES-256-GCM or equivalent authenticated encryption.

**Acceptance Criteria:**
- Given a captured credential stored in the database, the raw database contents are ciphertext, not plaintext.
- Encryption uses a unique initialization vector (IV/nonce) per record.
- The encryption key is derived from an application-level master key provided via environment variable, not stored in the database.
- Key rotation is supported without requiring re-encryption of all existing records (key versioning).

**Security Consideration:** The master encryption key MUST NOT be committed to source control, stored in the database, or logged. It is provided exclusively via environment variable at application startup.

---

**REQ-CRED-008: RBAC-Restricted Credential Access**

Access to captured credential data MUST be restricted by the platform's RBAC model as defined in Section 4.2.

**Acceptance Criteria:**
- Given a user with the Engineer role, they can view capture event counts and timestamps but cannot view or decrypt credential field values.
- Given a user with the Viewer role, they can view campaign-level aggregate statistics but cannot access individual capture events.
- Given a user with the Operator or Admin role, they can view capture events and explicitly decrypt credential values.
- API endpoints that return credential data enforce role checks and return 403 Forbidden for unauthorized roles.

---

**REQ-CRED-009: Credential Masking by Default**

Captured sensitive field values (passwords, tokens) MUST be masked in the UI by default and only revealed via explicit operator action.

**Acceptance Criteria:**
- Given a capture event displayed in the campaign dashboard, password and token fields show masked values (e.g., `********`).
- An explicit "Reveal" action (button click, keyboard shortcut) is required to decrypt and display the value.
- Revealed values are re-masked after a configurable timeout (default: 30 seconds) or when the operator navigates away.
- The API does not include decrypted credential values in default list/detail responses; a separate decrypt endpoint is used.

---

**REQ-CRED-010: Audit Logging for Credential Access**

Every action that decrypts, reveals, or exports captured credentials MUST create an entry in the audit log.

**Acceptance Criteria:**
- Given an operator clicking "Reveal" on a captured password, an audit log entry is created with: user ID, timestamp, action ("credential_reveal"), campaign ID, target ID, and capture event ID.
- Given an operator exporting credential data, an audit log entry records: user ID, timestamp, action ("credential_export"), campaign ID, export format, and number of records exported.
- Audit log entries for credential access are immutable and cannot be deleted by any role.
- Audit log entries are queryable via the framework API and viewable in the admin UI.

---

**REQ-CRED-011: No Automated Credential Validation**

The Tackle framework MUST NOT automatically test or validate captured credentials against any external authentication system (Active Directory, LDAP, OAuth providers, or any other service).

**Acceptance Criteria:**
- No code path exists in the framework that submits captured credentials to an external authentication endpoint.
- The UI does not provide a "Test Credential" or "Validate" button.
- Documentation explicitly states that credential validation is an operator responsibility performed outside Tackle.

---

### Campaign Context and Real-Time Features

---

**REQ-CRED-012: Campaign-Target-Variant Association**

Each capture event MUST be associated with a campaign, target, and template variant (if A/B testing is active).

**Acceptance Criteria:**
- Given a capture event, the database record includes foreign keys to the campaign, target, and template variant tables.
- Capture events can be queried by any combination of campaign, target, and variant.
- If A/B testing is not active for the campaign, the variant field is null.

---

**REQ-CRED-013: Real-Time Capture Notifications**

Capture events MUST be pushed to connected operator sessions in real time via WebSocket.

**Acceptance Criteria:**
- Given an operator viewing the campaign dashboard with a WebSocket connection, when a new credential is captured, a notification appears within 3 seconds of the capture event.
- The notification includes: timestamp, target identifier (name or email), and the list of fields captured (names only, not values).
- Multiple operators viewing the same campaign all receive the notification simultaneously.
- WebSocket messages for capture events do not include decrypted credential values.

---

**REQ-CRED-014: Capture Statistics for Campaign Metrics**

The framework MUST compute and expose capture statistics as defined in Section 5.2.

**Acceptance Criteria:**
- The campaign metrics API returns: capture rate, capture rate by variant, average time to capture, repeat submission rate, field completion rate, and a capture timeline series.
- Metrics update within 10 seconds of a new capture event.
- Metrics are available to all roles that can view the campaign (values are aggregate counts/percentages, not credential data).

---

**REQ-CRED-015: Capture Data in Campaign Reports**

Capture data MUST be included in campaign reports as defined in Section 5.3.

**Acceptance Criteria:**
- Summary reports include aggregate capture statistics without any credential values.
- Detailed reports include credential values and require Operator or Admin role to generate.
- Generating a detailed report creates an audit log entry.
- Export is available in CSV, JSON, and PDF formats.
- A "redacted export" option is available that includes capture metadata (timestamps, target, variant) without credential values.

---

**REQ-CRED-016: Immutability on Campaign Archive**

When a campaign is moved to the Archived state, all associated capture data MUST become immutable.

**Acceptance Criteria:**
- Given an archived campaign, attempts to modify or delete capture events via the API return 403 Forbidden with an explanatory error message.
- Archived capture data can still be read, decrypted (with audit logging), and exported.
- Only an Admin can permanently purge capture data from an archived campaign, and the purge action requires explicit confirmation and creates an audit log entry.

---

### Post-Capture Behavior

---

**REQ-CRED-017: Configurable Post-Capture Action**

Each landing page MUST support a configurable post-capture action executed after the form submission is processed.

**Acceptance Criteria:**
- The landing page builder provides a "Post-Capture Action" configuration with the following options: Redirect to URL, Display page, Redirect with delay, Replay submission, No action.
- The configured action executes after the credential capture is confirmed (data transmitted to framework successfully).
- If the framework API is unreachable, the post-capture action still executes (capture queuing and post-capture action are independent).

---

**REQ-CRED-018: Credential Replay to Real Service**

The landing page app MUST support an advanced post-capture action that replays the captured credentials to the real login endpoint of the impersonated service, enabling transparent pass-through authentication.

**Acceptance Criteria:**
- Given a landing page configured with "Replay submission," when the target submits credentials, the credentials are captured AND forwarded to the configured real login URL.
- The response from the real login endpoint is relayed back to the target's browser (cookies, redirects, session tokens).
- The target experiences a seamless login to the real service.
- This option is marked as "Advanced / High Risk" in the UI with a warning about potential detection.
- This option is disabled by default and requires Operator or Admin role to enable.

> **Cline Delegation Flag:** Credential replay and session passthrough implementation — including cookie/token relay, redirect chain handling, and transparent proxy behavior to the real service — is a candidate for Cline delegation (offensive security technique).

---

### Field Categorization and Configuration

---

**REQ-CRED-019: Configurable Field Categorization**

Operators MUST be able to configure field-name-to-category mappings in the landing page builder to control which fields are flagged as username, password, MFA token, or custom.

**Acceptance Criteria:**
- The landing page builder provides a UI for mapping form field names to categories (identity, sensitive, MFA, custom).
- Default mappings exist for common field names (e.g., `password` -> sensitive, `email` -> identity).
- Custom mappings override defaults.
- Field categorization determines encryption treatment, UI masking behavior, and report column placement.

---

**REQ-CRED-020: Unattributed Capture Handling**

The system MUST handle form submissions that cannot be attributed to a specific target.

**Acceptance Criteria:**
- Given a form submission with a missing or invalid tracking token, the capture event is stored with a null target association and flagged as "unattributed."
- Unattributed captures appear in the campaign dashboard with a distinct visual indicator.
- Operators can manually associate an unattributed capture with a target via the UI.
- Unattributed captures are included in aggregate campaign metrics (capture count) but excluded from target-level metrics (capture rate).

---

## 7A. Full Session Capture (Cookies, OAuth Tokens, Session Tokens)

**REQ-CRED-021: Full Session Capture**

Beyond username/password credential capture, the system SHALL support capturing full session data including browser cookies, OAuth tokens, session tokens, and other authentication artifacts.

| Data Type | Capture Method | Storage |
|-----------|---------------|---------|
| **Browser cookies** | JavaScript injection captures `document.cookie` and cookie-related headers. The compiled landing page app's Go backend intercepts `Set-Cookie` headers from proxied responses. | Encrypted at rest alongside credential data. |
| **OAuth tokens** | When the landing page intercepts an OAuth flow (authorization codes, access tokens, refresh tokens), these are captured from URL parameters, form fields, or response bodies. | Encrypted at rest. Token type and scope are recorded as metadata. |
| **Session tokens** | Tokens stored in `localStorage`, `sessionStorage`, or custom headers are captured via JavaScript injection at the page level. | Encrypted at rest. |
| **Authentication headers** | Bearer tokens, API keys, or other authentication headers sent by the target's browser are captured from proxied requests. | Encrypted at rest. |

**REQ-CRED-022: Session Capture Configuration**

| Feature | Behavior |
|---------|----------|
| **Per-landing-page toggle** | Session capture is configurable per landing page. It is disabled by default and must be explicitly enabled by the Operator. |
| **Capture scope** | The Operator configures which session data types to capture (cookies, OAuth tokens, session storage, authentication headers). |
| **Automatic detection** | The system automatically detects and flags common authentication artifacts (tokens matching JWT format, OAuth parameter names, common session cookie names). |
| **Time-sensitivity marking** | Captured tokens are marked with a flag indicating they are time-sensitive (may expire) with the capture timestamp prominently displayed. |

**REQ-CRED-023: Session Capture Data Model**

Each session capture event SHALL store:

| Field | Type | Description |
|-------|------|-------------|
| `capture_event_id` | UUID (FK) | Link to the parent credential capture event |
| `data_type` | ENUM | One of: `cookie`, `oauth_token`, `session_token`, `auth_header`, `local_storage`, `session_storage` |
| `key` | TEXT (encrypted) | The name/key of the captured item (e.g., cookie name, header name) |
| `value` | TEXT (encrypted) | The captured value (encrypted at rest) |
| `metadata` | JSONB | Additional context: domain, path, expiry, httpOnly flag, secure flag, token type, scope |
| `captured_at` | TIMESTAMP | When the capture occurred |
| `is_time_sensitive` | BOOLEAN | Whether the captured data has a known expiration |

Acceptance Criteria:
- [ ] Session capture data is encrypted at rest with the same AES-256-GCM scheme used for credential data
- [ ] Session capture is disabled by default and requires explicit Operator enablement per landing page
- [ ] Captured cookies include all attributes (domain, path, expiry, httpOnly, secure, sameSite)
- [ ] OAuth tokens are automatically classified by type (authorization code, access token, refresh token) based on parameter naming conventions
- [ ] Session capture data is accessible through the same RBAC model as credential data
- [ ] The UI displays session capture data separately from form field credentials, with clear labeling
- [ ] Time-sensitive captures display the elapsed time since capture and a warning if likely expired
- [ ] JavaScript-based capture code is embedded during the landing page build process and obfuscated per build (anti-fingerprinting)

> **Cline Delegation Flag:** Client-side session capture techniques — including cookie extraction, localStorage/sessionStorage enumeration, OAuth flow interception, and authentication header capture — are candidates for Cline delegation (offensive security technique).

---

## 8. Security Considerations Summary

| Concern | Mitigation |
|---------|-----------|
| Credential data exposure via database compromise | AES-256-GCM encryption at rest with external master key (REQ-CRED-007) |
| Unauthorized credential access by framework users | RBAC enforcement with role-appropriate access tiers (REQ-CRED-008) |
| Casual credential exposure in UI | Masking by default with explicit reveal (REQ-CRED-009) |
| Unaudited credential access | Mandatory audit log for every decrypt/reveal/export action (REQ-CRED-010) |
| Accidental credential validation causing lockouts | No automated validation — explicit architectural boundary (REQ-CRED-011) |
| Credential data in transit between landing page and framework | TLS encryption on internal API calls (REQ-CRED-003) |
| Stale credential data retention | Configurable retention policies and explicit purge workflow (REQ-CRED-016) |
| Encryption key compromise | Key versioning and rotation support without full re-encryption (REQ-CRED-007) |
| Session token exposure via capture | Same AES-256-GCM encryption at rest; RBAC-restricted access; time-sensitivity warnings (REQ-CRED-021, REQ-CRED-022) |

## 9. Cline Delegation Summary

The following requirements involve offensive security techniques and are candidates for delegation to Cline:

| Requirement | Technique |
|-------------|-----------|
| REQ-CRED-001 | Form field enumeration and universal capture |
| REQ-CRED-002 | Transparent form interception and data exfiltration |
| REQ-CRED-004 | Tracking token generation, embedding, and stealth extraction |
| REQ-CRED-018 | Credential replay and session passthrough to real services |
| REQ-CRED-021 | Full session capture (cookies, OAuth tokens, session tokens, auth headers) |

These items require specialized knowledge of browser behavior, anti-detection techniques, and offensive web application patterns. Implementation should be reviewed by the red team lead before deployment.

## 10. Dependencies

| Dependency | Document |
|------------|----------|
| Authentication & RBAC | [02-authentication-authorization.md](02-authentication-authorization.md) |
| Landing Page Builder | [05-landing-page-builder.md](05-landing-page-builder.md) |
| Campaign Management | [06-campaign-management.md](06-campaign-management.md) |
| Phishing Endpoints | [07-phishing-endpoints.md](07-phishing-endpoints.md) |
| Target Management | [09-target-management.md](09-target-management.md) |
| Metrics & Reporting | [10-metrics-reporting.md](10-metrics-reporting.md) |
| Audit Logging | [11-audit-logging.md](11-audit-logging.md) |
| Database Schema | [14-database-schema.md](14-database-schema.md) |
