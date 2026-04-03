# 11 — Audit Logging

## 1. Overview

Tackle requires comprehensive, immutable audit logging of every action, event, and request that occurs within the framework and its deployed infrastructure. The guiding principle is absolute visibility: **every action every user performs in the framework, all events related to email sending and user activity, all logs related to deployed infrastructure, and all requests sent to the deployed infrastructure** must be captured, stored, and queryable.

Audit logs serve three purposes:

1. **Operational awareness** — Real-time visibility into what is happening across campaigns, infrastructure, and user sessions.
2. **Post-engagement accountability** — A complete, tamper-evident record of every action taken during a red team engagement, suitable for executive review or legal inquiry.
3. **Debugging and forensics** — Detailed event trails that allow engineers to reconstruct any sequence of events after the fact.

## 2. Logging Philosophy

Nothing is optional. The system must log at a level of detail that allows full reconstruction of any user session, any campaign lifecycle, any email delivery chain, and any request to any phishing endpoint. Silence in the log is a bug.

Every log entry must be attributable to either a human actor (authenticated user) or a system actor (background worker, scheduler, external callback). Every log entry must carry a correlation ID that links it to related entries across subsystems.

## 3. Log Categories

### 3.1 User Activity Logs

All actions performed by authenticated users within the framework UI and API.

| Event Class | Examples |
|-------------|----------|
| **Navigation** | Page views, tab switches, modal opens |
| **Data mutation** | Form submissions, inline edits, configuration changes, bulk operations |
| **UI interaction** | Button clicks that trigger actions (not passive UI state like resizing) |
| **Authentication** | Login (success/failure), logout, session refresh, token issuance, password change, MFA challenge |
| **RBAC** | Role assignment, role revocation, permission changes, user creation, user deactivation |
| **Campaign lifecycle** | Create, edit, submit for approval, approve, reject, build, launch, pause, resume, complete, archive, unlock, delete |
| **Data access** | Viewing captured credentials, exporting reports, downloading CSV, accessing target PII, viewing SMTP credentials or API keys |

### 3.2 Email Event Logs

All events in the email delivery pipeline, from queue insertion through target interaction.

| Event Class | Examples |
|-------------|----------|
| **Queue events** | Email queued, dequeued, retry scheduled, expired from queue |
| **SMTP connection** | Connect, authenticate, STARTTLS, MAIL FROM, RCPT TO, DATA, QUIT, connection error, timeout |
| **Delivery status** | Sent (accepted by relay), delivered (accepted by target MX), bounced (hard/soft), deferred, rejected |
| **Tracking events** | Email opened (pixel hit), link clicked (with URL and timestamp), attachment downloaded |
| **Per-recipient detail** | Every event tied to a specific recipient record with campaign context |

### 3.3 Infrastructure Logs

All events related to the lifecycle and health of deployed phishing endpoint instances.

| Event Class | Examples |
|-------------|----------|
| **Instance lifecycle** | Provision requested, instance creating, instance running, configuration pushed, application deployed, health check pass/fail, stop requested, instance stopping, instance terminated |
| **Cloud API calls** | AWS/Azure API request (method, parameters), response (status, IDs), errors (code, message) |
| **Health monitoring** | Health check scheduled, health check result, status transition (healthy to unhealthy, etc.), alert triggered |
| **DNS operations** | Record creation, record modification, record deletion, propagation check, TTL changes |
| **Network events** | Security group changes, IP allocation, IP release, TLS certificate provisioned |

### 3.4 Request Logs (Phishing Endpoints)

Every HTTP request received by every phishing endpoint must be captured and aggregated back to the framework database.

| Event Class | Examples |
|-------------|----------|
| **HTTP requests** | Method, path, query string, headers (full), source IP, User-Agent, TLS version, timestamp, response status code, response size, response time |
| **Form submissions** | POST body data (linked to credential capture record), field names and values |
| **Tracking hits** | Pixel requests, redirect chain events, JavaScript beacon data |
| **Error responses** | 4xx/5xx responses served, with request context |

These logs originate on the phishing endpoint and must be transmitted to the framework database via an authenticated, encrypted channel. The endpoint must buffer logs locally if the framework is temporarily unreachable and flush them when connectivity is restored.

### 3.5 System Logs

Internal application events not directly triggered by a user action.

| Event Class | Examples |
|-------------|----------|
| **Application lifecycle** | Startup (with version, config summary), graceful shutdown, crash/panic (with stack trace) |
| **Database** | Migration applied, migration rolled back, connection pool events, slow query warnings |
| **Configuration** | Runtime configuration changed (with before/after values), feature flag toggled |
| **Background workers** | Worker started, task picked up, task completed, task failed (with error), worker pool resized |
| **Errors** | Unhandled errors with full stack traces, dependency failures (external API unreachable, DNS resolution failure) |
| **Security events** | Rate limit triggered, invalid token presented, unauthorized access attempt, IP blocklist match |

## 4. Log Schema

### 4.1 Core Fields

Every log entry, regardless of category, must contain the following fields:

| Field | Type | Description |
|-------|------|-------------|
| `id` | UUID | Unique identifier for the log entry |
| `timestamp` | TIMESTAMPTZ | When the event occurred (UTC, microsecond precision) |
| `category` | ENUM | One of: `user_activity`, `email_event`, `infrastructure`, `request`, `system` |
| `severity` | ENUM | One of: `debug`, `info`, `warn`, `error`, `critical` |
| `actor_type` | ENUM | One of: `user`, `system`, `endpoint`, `external` |
| `actor_id` | UUID (nullable) | User ID if actor_type is `user`; worker/service ID if `system`; endpoint ID if `endpoint` |
| `actor_label` | TEXT | Human-readable actor name (username, worker name, endpoint hostname) |
| `action` | TEXT | Machine-readable action identifier (e.g., `campaign.create`, `email.sent`, `endpoint.provision`) |
| `resource_type` | TEXT (nullable) | Type of resource affected (e.g., `campaign`, `endpoint`, `user`, `email`) |
| `resource_id` | UUID (nullable) | ID of the affected resource |
| `details` | JSONB | Structured payload with event-specific data (variable schema per action) |
| `correlation_id` | UUID | Links related events across subsystems (e.g., all events in a single campaign launch) |
| `source_ip` | INET (nullable) | IP address of the request origin (user session or phishing target) |
| `session_id` | UUID (nullable) | Session ID for user activity events |
| `campaign_id` | UUID (nullable) | Campaign context, when applicable |

### 4.2 Indexes

The following indexes are required for query performance:

- `(timestamp DESC)` — default chronological listing
- `(category, timestamp DESC)` — category-filtered views
- `(actor_id, timestamp DESC)` — user activity audit trail
- `(campaign_id, timestamp DESC)` — campaign-scoped event log
- `(resource_type, resource_id, timestamp DESC)` — resource history
- `(correlation_id)` — correlated event lookup
- `(action, timestamp DESC)` — action-type filtering
- `(severity, timestamp DESC)` — severity-filtered views
- GIN index on `details` — full JSON field querying
- Full-text search index on `action` and `details` — text search support

### 4.3 Partitioning

The `audit_logs` table must be partitioned by time (monthly range partitions on `timestamp`). This supports future archival processes without requiring application-level changes.

## 5. Logging UI

### 5.1 Dedicated Log Viewer

The framework admin UI must include a dedicated log viewer accessible from the main navigation. This is a first-class feature, not an afterthought.

### 5.2 Filtering and Search

| Capability | Description |
|------------|-------------|
| **Category filter** | Filter by one or more log categories (checkboxes) |
| **Severity filter** | Filter by one or more severity levels |
| **User filter** | Filter by actor (dropdown of known users) |
| **Campaign filter** | Filter by campaign (dropdown of campaigns) |
| **Time range** | Absolute range picker (from/to) and relative presets (last 1h, 6h, 24h, 7d, 30d) |
| **Action type filter** | Filter by action identifier (autocomplete from known actions) |
| **Resource filter** | Filter by resource type and optionally resource ID |
| **Full-text search** | Free-text search across action names and JSON details |
| **Combined filters** | All filters composable with AND logic |

### 5.3 Log Entry Detail View

Clicking any log entry must open a detail panel or modal showing:

- All core fields with human-readable labels
- Full `details` JSON rendered as a formatted, syntax-highlighted tree
- Correlation ID as a clickable link that applies a filter showing all correlated events
- Resource ID as a clickable link navigating to the resource (campaign, endpoint, user, etc.)
- Actor ID as a clickable link navigating to the user profile or actor detail

### 5.4 Real-Time Streaming

- Live tail mode using WebSocket connection
- New log entries appear at the top of the list without manual refresh
- Visual indicator when live mode is active
- Pause/resume live streaming without losing buffered entries
- Active filters apply to the live stream (server-side filtered before transmission)

### 5.5 Export

- Export currently filtered log view to CSV
- Export includes all core fields plus flattened details JSON
- Large exports (>10,000 entries) handled as background jobs with download notification

## 6. Data Retention

### 6.1 v1 Retention Policy

All logs are retained indefinitely in PostgreSQL. No automatic purging or archival.

### 6.2 Future Archival Design (Not Implemented in v1)

The schema and partitioning strategy must support a future S3 archival process:

- Monthly partitions can be detached, exported to Parquet/CSV, uploaded to S3, and dropped from PostgreSQL
- An `archive_metadata` table must exist (even if unused in v1) to track which partitions have been archived and their S3 locations
- The API must accept a time-range parameter so the UI can indicate when the requested range extends beyond the live database

## 7. Log Ingestion from Phishing Endpoints

### 7.1 Transport

Phishing endpoints transmit logs to the framework via an authenticated HTTPS channel:

- Endpoint authenticates using a per-instance API token issued during provisioning
- Logs are sent in batches (configurable batch size, default 100 entries or 5 seconds, whichever comes first)
- Request body is a JSON array of log entries conforming to the core schema
- Framework returns acknowledgment; endpoint removes acknowledged entries from local buffer

### 7.2 Buffering

- Endpoints must buffer logs locally (on-disk queue) if the framework is unreachable
- Buffer must survive endpoint process restart
- On reconnection, buffered logs are flushed in chronological order
- Maximum local buffer size: configurable, default 50 MB; oldest entries are dropped if exceeded

### 7.3 Validation

- Framework validates each ingested log entry against the schema before insertion
- Invalid entries are logged as system errors (meta-logging) and rejected with specific error messages
- Duplicate detection based on `id` field (idempotent ingestion)

## 8. Requirements

### REQ-LOG-001: Universal Action Logging

**Description:** Every user-initiated action in the framework UI and API must produce an audit log entry.

**Acceptance Criteria:**
- Every API endpoint that mutates state produces at least one log entry before returning.
- Every authenticated page view produces a navigation log entry.
- Log entries include the authenticated user ID, session ID, and source IP.
- No user action exists that does not appear in the audit log when filtered by that user and time range.

---

### REQ-LOG-002: Authentication Event Logging

**Description:** All authentication-related events must be logged, including failures.

**Acceptance Criteria:**
- Successful login produces a log entry with `action = auth.login.success`, user ID, authentication provider used, and source IP.
- Failed login produces a log entry with `action = auth.login.failure`, attempted username, failure reason (invalid credentials, account locked, provider error), and source IP.
- Logout produces a log entry with `action = auth.logout`.
- Session refresh produces a log entry with `action = auth.session.refresh`.
- Password change produces a log entry with `action = auth.password.change` (old password hash is NOT logged).
- All auth events are severity `info` except failures, which are severity `warn`.

---

### REQ-LOG-003: RBAC Event Logging

**Description:** All changes to roles, permissions, and user access levels must be logged.

**Acceptance Criteria:**
- Role assignment produces a log entry identifying the target user, the assigned role, and the actor who performed the assignment.
- Role revocation produces a log entry with equivalent detail.
- Permission changes include before and after values in the `details` JSON.
- User creation and deactivation are logged with full context.

---

### REQ-LOG-004: Campaign Lifecycle Logging

**Description:** Every state transition in a campaign's lifecycle must be logged.

**Acceptance Criteria:**
- Each of the following actions produces a distinct log entry: create, edit, submit for approval, approve, reject, build, launch, pause, resume, complete, archive, unlock, delete.
- Each entry includes `campaign_id`, the previous state, the new state, and the actor.
- Approval and rejection entries include the reason/comment provided by the approver.
- All campaign lifecycle events share a `correlation_id` scoped to the campaign.

---

### REQ-LOG-005: Sensitive Data Access Logging

**Description:** Accessing sensitive data (credentials, PII, API keys) must produce an explicit log entry.

**Acceptance Criteria:**
- Viewing captured credentials produces a log entry with `action = data.credentials.view`, the campaign ID, and the user who accessed the data.
- Exporting reports produces a log entry with the export format, filter parameters, and row count.
- Viewing SMTP configuration passwords (even masked) produces a log entry.
- Accessing target lists with PII produces a log entry.
- These entries are severity `info` with a security flag in the details JSON (`"security_relevant": true`).

---

### REQ-LOG-006: Email Pipeline Logging

**Description:** Every event in the email delivery pipeline must be logged with per-recipient granularity.

**Acceptance Criteria:**
- Each email queued for sending produces a log entry with campaign ID, recipient ID, template ID, and scheduled send time.
- SMTP connection events (connect, auth, STARTTLS, send, disconnect, error) each produce a log entry with the SMTP server hostname and port.
- Delivery status changes (sent, delivered, bounced, deferred, rejected) produce log entries with the status reason from the remote server.
- Tracking events (open, click) produce log entries with timestamp, source IP, and User-Agent.
- All email events for a single send operation share a `correlation_id`.

---

### REQ-LOG-007: Infrastructure Lifecycle Logging

**Description:** All phishing endpoint infrastructure events must be logged.

**Acceptance Criteria:**
- Instance provisioning produces a log entry with cloud provider, region, instance type, and campaign association.
- Each cloud API call produces a log entry with the API method, key request parameters, response status, and response identifiers (instance ID, request ID).
- Cloud API errors produce log entries with severity `error`, the error code, and the full error message.
- Health check results produce log entries; status transitions (healthy to unhealthy) produce entries with severity `warn`.
- Instance termination produces a log entry with the reason (user-initiated, campaign completed, error).
- DNS record operations (create, modify, delete) produce log entries with the record type, name, value, and TTL.

---

### REQ-LOG-008: Phishing Endpoint Request Logging

**Description:** Every HTTP request received by a phishing endpoint must be logged and aggregated to the framework database.

**Acceptance Criteria:**
- Each HTTP request produces a log entry with: method, full path, query string, all request headers, source IP, User-Agent, TLS version, request timestamp, response status code, response body size, and response time in milliseconds.
- Form submission requests (POST) include the submitted field names and values in the details JSON, linked to the corresponding credential capture record.
- Log entries are transmitted from the endpoint to the framework via the authenticated log ingestion channel (see REQ-LOG-015).
- Request logs are queryable by campaign, source IP, time range, and response code in the framework UI.

---

### REQ-LOG-009: System Event Logging

**Description:** Internal application events must be logged with sufficient detail for debugging and forensics.

**Acceptance Criteria:**
- Application startup produces a log entry with the application version, Go version, startup configuration summary (without secrets), and listening address.
- Graceful shutdown produces a log entry with the shutdown reason and duration.
- Unhandled panics produce a log entry with severity `critical`, the panic message, and the full stack trace.
- Database migrations produce log entries with the migration version, direction (up/down), and result.
- Background worker lifecycle events (start, task pickup, completion, failure) produce log entries with the worker name and task identifier.
- Configuration changes (runtime) produce log entries with the setting name, previous value, and new value (secrets are masked as `[REDACTED]`).

---

### REQ-LOG-010: Log Schema Compliance

**Description:** Every log entry must conform to the defined core schema.

**Acceptance Criteria:**
- All log entries contain the mandatory fields: `id`, `timestamp`, `category`, `severity`, `actor_type`, `action`, `details`, `correlation_id`.
- `timestamp` is stored in UTC with microsecond precision.
- `category` is one of the five defined categories.
- `severity` is one of the five defined levels.
- `details` is valid JSONB.
- `correlation_id` is present and non-null on every entry; entries that are not part of a correlated sequence use a self-referencing correlation ID (equal to their own `id`).
- A database constraint or application-level validation rejects entries missing mandatory fields.

---

### REQ-LOG-011: Log Viewer UI

**Description:** The framework admin UI must include a dedicated, full-featured log viewer.

**Acceptance Criteria:**
- The log viewer is accessible from the main navigation sidebar.
- The log viewer loads with a default view of the most recent 100 entries across all categories.
- The log viewer supports pagination (infinite scroll or explicit page controls) for large result sets.
- Response time for filtered queries returning up to 1,000 entries is under 2 seconds.
- The log viewer is accessible to users with the `admin` or `engineer` role (RBAC-controlled).

---

### REQ-LOG-012: Log Filtering and Search

**Description:** The log viewer must support powerful, composable filtering and full-text search.

**Acceptance Criteria:**
- Users can filter by: category (multi-select), severity (multi-select), actor/user (dropdown), campaign (dropdown), time range (absolute and relative), action type (autocomplete), resource type, and resource ID.
- All filters are combinable (AND logic).
- Full-text search queries the `action` field and the `details` JSONB field.
- Search results highlight matching terms.
- Active filters are reflected in the URL query string (shareable/bookmarkable filter states).
- Clearing all filters resets to the default view.

---

### REQ-LOG-013: Log Entry Detail View

**Description:** Each log entry must be expandable to show its full context.

**Acceptance Criteria:**
- Clicking a log entry opens a detail panel showing all core fields with human-readable labels.
- The `details` JSON is rendered as a formatted, syntax-highlighted, collapsible tree.
- The `correlation_id` is displayed as a clickable link that filters the log view to all entries sharing that correlation ID.
- The `resource_id` is displayed as a clickable link that navigates to the resource detail page (e.g., campaign detail, endpoint detail).
- The `actor_id` is displayed as a clickable link that navigates to the user profile or filters logs to that actor.

---

### REQ-LOG-014: Real-Time Log Streaming

**Description:** The log viewer must support live, real-time log streaming.

**Acceptance Criteria:**
- A "Live" toggle activates real-time streaming via WebSocket.
- New log entries appear at the top of the list without page refresh.
- A visual indicator (e.g., pulsing dot, banner) shows when live mode is active.
- Users can pause and resume the live stream; entries received while paused are buffered and displayed on resume.
- Active filters apply server-side to the WebSocket stream (only matching entries are transmitted).
- Live mode automatically disconnects after 30 minutes of inactivity with a prompt to reconnect.

---

### REQ-LOG-015: Phishing Endpoint Log Ingestion

**Description:** The framework must provide an authenticated API endpoint for phishing endpoints to submit logs.

**Acceptance Criteria:**
- The ingestion endpoint accepts POST requests at `/api/v1/endpoints/{endpoint_id}/logs`.
- Authentication uses a per-instance API token issued during endpoint provisioning.
- Request body is a JSON array of log entries conforming to the core schema.
- The framework validates each entry, inserts valid entries, and returns a response indicating accepted count and rejected count (with per-entry error details for rejections).
- Duplicate entries (same `id`) are silently accepted (idempotent).
- The endpoint processes batches of up to 500 entries per request.
- Response time for a batch of 100 entries is under 500 milliseconds.

---

### REQ-LOG-016: Endpoint Log Buffering

**Description:** Phishing endpoints must buffer logs locally when the framework is unreachable.

**Acceptance Criteria:**
- The endpoint writes log entries to an on-disk queue when the framework ingestion endpoint is unreachable.
- The on-disk queue survives process restarts.
- On reconnection, buffered entries are flushed in chronological order.
- The maximum buffer size is configurable (default: 50 MB).
- When the buffer exceeds the maximum size, the oldest entries are dropped and a warning is logged locally.

---

### REQ-LOG-017: Log Export

**Description:** Users must be able to export filtered log data to CSV.

**Acceptance Criteria:**
- An "Export CSV" button is available in the log viewer toolbar.
- The export respects all currently active filters.
- Exported CSV includes all core fields; the `details` JSON is flattened to a single column containing the raw JSON string.
- Exports of 10,000 entries or fewer are generated and downloaded inline.
- Exports exceeding 10,000 entries are processed as background jobs; the user receives a notification when the file is ready for download.
- Export actions are themselves logged (REQ-LOG-005).

---

### REQ-LOG-018: Data Retention — Indefinite Storage

**Description:** All audit logs must be retained indefinitely in the v1 implementation.

**Acceptance Criteria:**
- No automated purge, truncation, or archival process exists in v1.
- The `audit_logs` table is partitioned by month on the `timestamp` column to support future archival.
- An `archive_metadata` table exists with columns: `partition_name`, `time_range_start`, `time_range_end`, `archived_at`, `s3_bucket`, `s3_key`, `row_count`, `status`. This table is empty in v1 but the schema is present.
- The log viewer API accepts a `time_range` parameter and the UI displays a notice if the requested range extends beyond available partitions (preparing for future archival).

---

### REQ-LOG-019: Correlation ID Propagation

**Description:** Correlation IDs must be propagated across all subsystems to enable end-to-end event tracing.

**Acceptance Criteria:**
- When a user initiates an action that triggers multiple subsystem events (e.g., launching a campaign triggers infrastructure provisioning, email sending, and DNS configuration), all resulting log entries share the same `correlation_id`.
- Correlation IDs are generated at the point of user action and passed through all downstream function calls, background jobs, and endpoint communications.
- The log detail view allows one-click filtering to all entries sharing a correlation ID.
- Correlation IDs are included in error responses to support debugging ("Reference ID: {correlation_id}").

---

### REQ-LOG-020: Log Immutability

**Description:** Audit log entries must be immutable after creation.

**Acceptance Criteria:**
- The `audit_logs` table has no UPDATE or DELETE grants for the application database user; only INSERT and SELECT are permitted.
- The application code does not contain any UPDATE or DELETE queries against the `audit_logs` table.
- An attempt to modify or delete a log entry via the API returns a `405 Method Not Allowed` response.
- Database triggers reject any UPDATE or DELETE operation on the `audit_logs` table (defense in depth beyond application-level controls).

---

### REQ-LOG-021: Log Tampering Prevention

**Description:** The system must provide mechanisms to detect log tampering.

**Acceptance Criteria:**
- Each log entry includes a `checksum` field containing an HMAC-SHA256 of the entry's core fields (timestamp, category, action, actor_id, resource_id, details), keyed with a server-side secret.
- The log viewer UI provides a "Verify Integrity" action on any entry that recomputes and validates the checksum.
- A background job periodically (configurable, default daily) verifies checksums for all entries created in the previous period and logs a system event with the result.
- Checksum verification failures produce a `critical` severity log entry.

---

### REQ-LOG-022: PII Handling in Logs

**Description:** Logs must handle personally identifiable information (PII) with appropriate care.

**Acceptance Criteria:**
- Email addresses in log details are stored in full (required for operational tracing) but marked with a `"contains_pii": true` flag in the details JSON.
- Passwords, API keys, and secret tokens are never stored in log details; they are replaced with `[REDACTED]` before logging.
- The log export function includes a header warning: "This export contains personally identifiable information. Handle according to your organization's data handling policy."
- Log entries containing PII are identified by the `contains_pii` flag to support future data subject access request (DSAR) compliance tooling.

---

### REQ-LOG-023: Structured Logging Format

**Description:** Application-level logging (stdout/stderr) must use structured JSON format.

**Acceptance Criteria:**
- All application log output (Go backend) is in JSON format with fields: `timestamp`, `level`, `message`, `component`, `correlation_id`, and optional additional fields.
- No unstructured (plain text) log lines are emitted by the application in production mode.
- Log output is compatible with standard log aggregation tools (e.g., can be piped to `jq` for local debugging).
- A development mode flag enables human-readable formatted output for local development only.

---

### REQ-LOG-024: Log Severity Guidelines

**Description:** Log severity levels must be applied consistently according to defined guidelines.

**Acceptance Criteria:**
- `debug`: Detailed diagnostic information. Not written to the audit log table in production; only to stdout when debug mode is enabled.
- `info`: Normal operational events. All successful user actions, system operations, and state transitions.
- `warn`: Abnormal but non-critical events. Failed login attempts, health check failures, retry attempts, approaching resource limits.
- `error`: Failures requiring attention. Cloud API errors, SMTP delivery failures, unhandled application errors, data validation failures.
- `critical`: Conditions requiring immediate attention. Application panics, data corruption detected, log tampering detected, security violations.
- Severity assignment guidelines are documented in a developer reference and enforced via code review.

---

### REQ-LOG-025: Performance Requirements

**Description:** The logging subsystem must not degrade application performance.

**Acceptance Criteria:**
- Log writes are asynchronous; the act of logging does not block the request/response cycle.
- An in-memory buffer (configurable, default 10,000 entries) absorbs write bursts; entries are flushed to PostgreSQL in batches.
- If the buffer is full, new entries are written synchronously (back-pressure) rather than dropped.
- The logging subsystem adds no more than 5 milliseconds of latency to any API request under normal operating conditions.
- The log viewer query endpoint returns results for filtered queries (up to 1,000 rows) within 2 seconds, measured at the 95th percentile.
- Bulk ingestion from phishing endpoints (batch of 100 entries) completes within 500 milliseconds.

## 8A. Rule-Based Alert System

### REQ-LOG-026: Configurable Alert Rules

**Description:** The system SHALL support configurable alert rules that trigger notifications when specific log patterns or conditions are detected.

| Feature | Behavior |
|---------|----------|
| **Rule definition** | Administrators can create alert rules that match log entries based on: category, severity, action pattern (exact or regex), actor, resource type, and custom conditions on the `details` JSON. |
| **Rule conditions** | Rules support: single-event matching (trigger on any matching entry), threshold matching (trigger when N matching entries occur within T minutes), and absence matching (trigger when an expected event does NOT occur within T minutes). |
| **Alert actions** | When a rule triggers, the system can: send an in-app notification to specified users/roles, send an email notification, fire a webhook to an external URL, and/or escalate severity (log a new entry with elevated severity). |
| **Rule management** | Rules can be created, edited, enabled, disabled, and deleted by Administrators. Each rule has a name, description, enabled/disabled status, and cooldown period (minimum time between repeated alerts from the same rule). |

**Acceptance Criteria:**
- [ ] At least 10 concurrent alert rules can be active simultaneously
- [ ] Rule evaluation does not add more than 10ms of latency to log write operations
- [ ] Alert rules are evaluated asynchronously (log writes are not blocked by alert delivery)
- [ ] Each triggered alert creates its own audit log entry for traceability
- [ ] Rules support a cooldown period (default: 5 minutes) to prevent alert flooding
- [ ] The alert rule configuration UI provides a test/preview function that shows which recent log entries would match the rule

---

### REQ-LOG-027: Built-In Alert Rule Templates

**Description:** The system SHALL include a set of pre-configured alert rule templates for common security-relevant events.

| Template | Trigger Condition | Default Action |
|----------|-------------------|----------------|
| **Failed login surge** | More than 5 failed login attempts from the same IP within 10 minutes | Notify all Administrators |
| **Endpoint error** | Any endpoint state transition to Error | Notify campaign Operator + all Engineers |
| **Block list override** | Any block list override approval | Notify all Administrators |
| **Credential access** | Any credential reveal or export action | Notify all Administrators |
| **Build failure** | Any campaign build failure | Notify campaign Operator |
| **Log tampering detected** | Any HMAC checksum verification failure | Notify all Administrators (critical severity) |

**Acceptance Criteria:**
- [ ] All six template rules are available on system initialization
- [ ] Templates can be duplicated and customized but the originals cannot be deleted
- [ ] Templates are disabled by default and must be explicitly enabled by an Administrator
- [ ] Each template's trigger conditions and actions are fully editable after duplication

---

## 9. Security Considerations

### 9.1 Log Access Control

- Log viewer access is restricted by RBAC: only users with `admin` or `engineer` roles can access the log viewer.
- Operators can view logs scoped to their own campaigns only (unless they hold `admin` or `engineer` role).
- Log export capability requires `admin` or `engineer` role.

### 9.2 Log Integrity

- Immutability is enforced at both the application and database layers (REQ-LOG-020).
- HMAC checksums provide tamper detection (REQ-LOG-021).
- The HMAC key is stored as an environment variable, separate from the database encryption key.

### 9.3 Secrets in Logs

- Secrets (passwords, tokens, API keys) must never appear in log entries in cleartext.
- Application code must sanitize all log payloads before writing. A centralized sanitization function strips known secret field names (`password`, `token`, `secret`, `api_key`, `private_key`, and variations) from the details JSON.
- Code review checklists must include verification that new log statements do not leak secrets.

### 9.4 Log Injection Prevention

- All user-supplied values included in log entries must be sanitized to prevent log injection attacks.
- Newline characters, control characters, and escape sequences in user input are encoded before inclusion in log details.
- The structured JSON format inherently mitigates many log injection vectors, but explicit sanitization is still required for defense in depth.

## 10. Dependencies

| Dependency | Description |
|------------|-------------|
| [02 — Authentication & Authorization](02-authentication-authorization.md) | User identity and session context for actor attribution |
| [06 — Campaign Management](06-campaign-management.md) | Campaign IDs and lifecycle states for campaign-scoped logging |
| [07 — Phishing Endpoints](07-phishing-endpoints.md) | Endpoint log ingestion channel and instance identification |
| [08 — Credential Capture](08-credential-capture.md) | Linking request logs to credential capture records |
| [14 — Database Schema](14-database-schema.md) | `audit_logs` table definition, partitioning, and indexes |
| [16 — Frontend Architecture](16-frontend-architecture.md) | Log viewer UI components and WebSocket integration |

## 11. Acceptance Criteria (Module Level)

- [ ] Every API endpoint that mutates state produces an audit log entry
- [ ] Every UI page view produces a navigation log entry
- [ ] Every email lifecycle event is logged with per-recipient granularity
- [ ] Every infrastructure operation (provision, configure, deploy, health check, terminate) is logged with cloud API details
- [ ] Every HTTP request to a phishing endpoint is logged and aggregated to the framework database
- [ ] The log viewer displays entries with filtering by category, severity, user, campaign, time range, action type, and free text
- [ ] The log detail view shows full context including formatted JSON details
- [ ] Real-time log streaming works via WebSocket with server-side filtering
- [ ] Filtered logs are exportable to CSV
- [ ] Log entries are immutable (no UPDATE/DELETE at database or API level)
- [ ] HMAC checksums are present and verifiable on all log entries
- [ ] Secrets never appear in cleartext in any log entry
- [ ] Phishing endpoints buffer logs locally during framework unavailability and flush on reconnection
- [ ] The audit_logs table is partitioned by month with the archive_metadata table present
- [ ] Correlation IDs link related events across all subsystems
- [ ] Log write overhead does not exceed 5ms per API request under normal conditions
- [ ] Alert rules can be created, edited, enabled/disabled, and deleted by Administrators
- [ ] Alert rules trigger notifications correctly based on configured conditions
- [ ] Built-in alert rule templates are available and customizable
- [ ] Alert evaluation does not block log write operations
