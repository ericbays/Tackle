# 04 — Email & SMTP Configuration

## 1. Purpose

This document defines the requirements for configuring external SMTP servers, composing phishing emails, managing email authentication DNS records, and controlling email delivery within the Tackle platform. Email is the sole delivery channel in v1 — SMS, voice phishing, and other channels are out of scope.

## 2. Architectural Context

```
┌──────────────────────────────────────────────────────────────────────┐
│                     TACKLE FRAMEWORK SERVER                          │
│                                                                      │
│  ┌──────────────────┐  ┌──────────────────┐  ┌──────────────────┐    │
│  │  SMTP Config UI  │  │  Email Template  │  │ DNS Record Mgmt  │    │
│  │  (CRUD, test)    │  │  Editor & Preview│  │ (DKIM/SPF/DMARC) │    │
│  └────────┬─────────┘  └────────┬─────────┘  └─────────┬────────┘    │
│           │                     │                      │             │
│           └─────────┬───────────┴──────────────────────┘             │
│                     │                                                │
│                     ▼                                                │
│           ┌──────────────────┐                                       │
│           │ Campaign Engine  │                                       │
│           │ (orchestration)  │                                       │
│           └────────┬─────────┘                                       │
│                    │  sends campaign payload                         │
└────────────────────┼─────────────────────────────────────────────────┘
                     │
                     ▼
      ┌──────────────────────────────┐
      │   PHISHING ENDPOINT (VM)     │
      │                              │
      │  ┌────────────────────────┐  │
      │  │   SMTP Relay Module    │  │
      │  │                        │  │
      │  │  - Receives campaign   │  │
      │  │    payload from        │  │
      │  │    framework           │  │
      │  │  - Opens connections   │  │
      │  │    to external SMTP    │  │
      │  │    servers             │  │
      │  │  - Sends emails per    │  │
      │  │    campaign schedule   │  │
      │  │  - Reports delivery    │  │
      │  │    status back to      │  │
      │  │    framework           │  │
      │  └───────────┬────────────┘  │
      └──────────────┼───────────────┘
                     │
                     ▼
      ┌──────────────────────────────┐
      │   EXTERNAL SMTP SERVER(S)    │
      │   (e.g., SES, Mailgun,       │
      │    self-hosted Postfix)      │
      └──────────────┬───────────────┘
                     │
                     ▼
      ┌──────────────────────────────┐
      │   TARGET INBOX               │
      └──────────────────────────────┘
```

### 2.1 Key Architecture Constraints

1. **No built-in SMTP server.** Tackle does not run its own MTA. All SMTP servers are external and user-configured.
2. **Phishing endpoint is the sender.** The phishing endpoint VM opens SMTP connections to the configured external servers. This ensures the sending IP address seen in email headers belongs to the phishing endpoint, not the framework server.
3. **Framework orchestrates, endpoint executes.** The framework server compiles campaign payloads (rendered templates, SMTP credentials, schedules, target lists) and transmits them to the phishing endpoint. The endpoint performs all SMTP transactions.
4. **Email only in v1.** No SMS, voice, or messaging-app delivery channels.

## 3. SMTP Configuration

### 3.1 SMTP Server Profiles

SMTP configurations are first-class, reusable entities that define connection parameters for an external SMTP server.

**REQ-SMTP-001** — The system SHALL allow users to create, read, update, and delete SMTP server profiles.

**REQ-SMTP-002** — Each SMTP server profile SHALL store the following fields:

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `name` | string | yes | Human-readable label (unique per tenant) |
| `description` | string | no | Free-text notes |
| `host` | string | yes | SMTP server hostname or IP address |
| `port` | integer | yes | SMTP server port (common: 25, 465, 587, 2525) |
| `auth_type` | enum | yes | One of: `none`, `plain`, `login`, `cram_md5`, `xoauth2` |
| `username` | string | conditional | Required when `auth_type` is not `none` |
| `password` | string (encrypted) | conditional | Required when `auth_type` is not `none`; stored encrypted at rest |
| `tls_mode` | enum | yes | One of: `none`, `starttls`, `tls` (implicit TLS) |
| `tls_skip_verify` | boolean | yes | Whether to skip TLS certificate verification (default `false`) |
| `from_address` | string | yes | Default envelope sender (RFC 5321 MAIL FROM) |
| `from_name` | string | no | Default display name for the From header |
| `reply_to` | string | no | Default Reply-To header value |
| `custom_helo` | string | no | Custom HELO/EHLO hostname; defaults to endpoint's reverse DNS if blank |
| `max_send_rate` | integer | no | Maximum emails per minute for this SMTP profile (0 = unlimited) |
| `max_connections` | integer | no | Maximum concurrent SMTP connections to this server (default 5) |
| `timeout_connect` | integer | no | Connection timeout in seconds (default 30) |
| `timeout_send` | integer | no | Per-message send timeout in seconds (default 60) |

**REQ-SMTP-003** — SMTP profile passwords and OAuth tokens SHALL be encrypted at rest using the application-level encryption described in the System Overview (01). They SHALL never appear in logs, API responses, or UI displays after initial creation.

**REQ-SMTP-004** — The system SHALL allow duplicating an existing SMTP profile to create a new profile pre-populated with the source profile's settings (excluding credentials).

**REQ-SMTP-005** — The system SHALL prevent deletion of an SMTP profile that is currently associated with an active (running or scheduled) campaign. The UI SHALL display which campaigns reference the profile.

### 3.2 SMTP Connection Testing

**REQ-SMTP-006** — The system SHALL provide a "Test Connection" action for any SMTP profile. This test SHALL:
- Establish a TCP connection to the configured host and port
- Negotiate TLS if configured
- Authenticate with the provided credentials
- Issue an SMTP NOOP command (or equivalent) to verify the session is valid
- Report success or a structured error message indicating the failure stage (connection, TLS, authentication, command)

**REQ-SMTP-007** — The connection test SHALL execute from the framework server for configuration validation purposes. A separate "Send Test Email" function (REQ-EMAIL-012) executes from the phishing endpoint.

**REQ-SMTP-008** — Connection test results SHALL be logged in the audit log with the SMTP profile ID, test timestamp, result (pass/fail), and error details if applicable.

### 3.3 Campaign SMTP Assignment

**REQ-SMTP-009** — Each campaign SHALL be associated with one or more SMTP profiles.

**REQ-SMTP-010** — When multiple SMTP profiles are assigned to a campaign, the user SHALL configure a sending strategy from the following options:

| Strategy | Behavior |
|----------|----------|
| `round_robin` | Rotate through SMTP profiles sequentially per message |
| `random` | Select a random SMTP profile per message |
| `weighted` | Distribute messages according to user-defined weight percentages (must total 100%) |
| `failover` | Use profiles in priority order; advance to the next only when the current profile fails or reaches its rate limit |
| `segment` | Assign specific SMTP profiles to specific target segments (defined by target list attributes) |

**REQ-SMTP-011** — Each campaign-SMTP association SHALL allow overriding the profile-level `from_address`, `from_name`, and `reply_to` fields for that campaign.

**REQ-SMTP-012** — The system SHALL validate that all SMTP profiles assigned to a campaign are reachable (via REQ-SMTP-006 connection test) before allowing the campaign to transition to the `ready` state.

### 3.4 Sending Rate Limits and Schedules

**REQ-SMTP-013** — Each campaign SHALL support the following delivery schedule parameters:

| Parameter | Type | Description |
|-----------|------|-------------|
| `send_window_start` | time (HH:MM, timezone-aware) | Earliest time of day emails may be sent |
| `send_window_end` | time (HH:MM, timezone-aware) | Latest time of day emails may be sent |
| `send_window_timezone` | string (IANA tz) | Timezone for the send window (e.g., `America/New_York`) |
| `send_window_days` | []weekday | Days of the week when sending is allowed (e.g., Mon-Fri) |
| `campaign_rate_limit` | integer | Maximum emails per minute across all SMTP profiles for this campaign |
| `min_delay_ms` | integer | Minimum delay in milliseconds between consecutive emails |
| `max_delay_ms` | integer | Maximum delay in milliseconds between consecutive emails (randomized between min and max) |
| `batch_size` | integer | Number of emails to send per batch before pausing |
| `batch_pause_seconds` | integer | Pause duration in seconds between batches |

**REQ-SMTP-014** — The effective send rate for a campaign SHALL be the most restrictive of: the campaign-level rate limit, the sum of individual SMTP profile rate limits, and any rate limits imposed by the sending strategy.

**REQ-SMTP-015** — When the send window closes (time-of-day or day-of-week boundary), the phishing endpoint SHALL pause sending and resume when the next valid window opens. No emails SHALL be queued or sent outside the configured window.

**REQ-SMTP-016** — The randomized inter-message delay (between `min_delay_ms` and `max_delay_ms`) SHALL use a cryptographically secure random number generator to avoid predictable timing patterns.

## 4. Email Composition

### 4.1 Email Templates

**REQ-EMAIL-001** — The system SHALL support creating, reading, updating, and deleting email templates independently of campaigns. Templates are reusable entities.

**REQ-EMAIL-002** — Each email template SHALL contain the following fields:

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `name` | string | yes | Human-readable label (unique per tenant) |
| `description` | string | no | Free-text notes |
| `subject` | string | yes | Email subject line (supports variable substitution) |
| `body_html` | text | yes | HTML email body (supports variable substitution) |
| `body_text` | text | no | Plain-text email body (supports variable substitution); auto-generated from HTML if omitted |
| `from_name_override` | string | no | Override the SMTP profile's display name for this template |
| `from_address_override` | string | no | Override the SMTP profile's sender address for this template |
| `reply_to_override` | string | no | Override the SMTP profile's Reply-To for this template |
| `custom_headers` | []key-value | no | Additional email headers (e.g., `X-Mailer`, `List-Unsubscribe`) |
| `attachments` | []attachment | no | File attachments (see REQ-EMAIL-006) |
| `priority` | enum | no | Email priority header: `high`, `normal`, `low` |

**REQ-EMAIL-003** — The template engine SHALL support the following variable substitution tokens, delimited by `{{` and `}}`:

| Token | Resolves To |
|-------|-------------|
| `{{target.first_name}}` | Target's first name |
| `{{target.last_name}}` | Target's last name |
| `{{target.full_name}}` | Target's full name |
| `{{target.email}}` | Target's email address |
| `{{target.position}}` | Target's job title |
| `{{target.department}}` | Target's department |
| `{{target.custom.<field>}}` | Any custom field from the target record |
| `{{campaign.name}}` | Campaign name |
| `{{campaign.from_name}}` | Sender display name |
| `{{sender.email}}` | Effective sender email address |
| `{{tracking.pixel}}` | Inserts a 1x1 tracking pixel `<img>` tag (HTML body only) |
| `{{tracking.link}}` | Base URL of the phishing endpoint for this campaign |
| `{{phishing.url}}` | Full phishing URL unique to this target (includes tracking token) |
| `{{current.date}}` | Current date (formatted per campaign locale) |
| `{{current.year}}` | Current four-digit year |

**REQ-EMAIL-004** — Variable substitution SHALL fail gracefully: if a token references a field that is empty or missing on the target record, the system SHALL substitute an empty string and log a warning (not abort the send).

**REQ-EMAIL-005** — The template engine SHALL sanitize substituted values to prevent header injection attacks. Any value containing newline characters (`\r`, `\n`) SHALL have those characters stripped before insertion into email headers.

### 4.2 Attachments

**REQ-EMAIL-006** — The system SHALL support attaching files to email templates. Each attachment SHALL have:

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `filename` | string | yes | Filename as it appears to the recipient |
| `content_type` | string | yes | MIME type (e.g., `application/pdf`) |
| `content` | binary | yes | File content (stored encrypted at rest) |
| `inline` | boolean | no | If `true`, embed as inline attachment with `Content-ID` for HTML body references |

**REQ-EMAIL-007** — The system SHALL enforce a configurable maximum attachment size per email (default: 10 MB). Emails exceeding this limit SHALL be rejected at template save time with a descriptive error.

**REQ-EMAIL-008** — The system SHALL enforce a configurable allowlist of permitted attachment MIME types. By default, the allowlist SHALL include common document types (`application/pdf`, `application/vnd.openxmlformats-officedocument.*`, `image/*`, `text/plain`, `text/html`, `text/csv`). An administrator SHALL be able to modify this allowlist.

### 4.3 A/B Testing

**REQ-EMAIL-009** — A campaign SHALL support associating multiple email templates, each with a configurable weight (percentage). The weights across all templates in a campaign SHALL total 100%.

**REQ-EMAIL-010** — When a campaign has multiple templates, the system SHALL assign each target to a template based on the configured weights using randomized selection. The assignment SHALL be recorded and immutable once the email is sent, enabling per-template performance analysis.

**REQ-EMAIL-011** — The system SHALL report per-template metrics (open rate, click rate, credential submission rate) on the campaign dashboard to facilitate A/B comparison.

### 4.4 Preview and Test Send

**REQ-EMAIL-012** — The system SHALL provide a "Send Test Email" function that:
- Accepts a target email address (or selects from configured test accounts)
- Renders the selected template with user-supplied or sample variable values
- Sends the email through the campaign's assigned phishing endpoint and SMTP profile
- Reports delivery success or failure with SMTP transaction details (response codes, relay info)

**REQ-EMAIL-013** — The system SHALL provide an in-UI preview of a rendered email template showing:
- The fully substituted HTML body rendered in an isolated iframe
- The fully substituted plain-text body
- All headers (From, Reply-To, Subject, custom headers)
- Attachment listing with file sizes
- The preview SHALL accept a target record (or sample data) for variable resolution

**REQ-EMAIL-014** — Test emails SHALL be clearly distinguishable in the system logs and SHALL NOT count toward campaign metrics or target interaction records.

### 4.5 Email Construction and Encoding

**REQ-EMAIL-015** — All outbound emails SHALL be constructed as valid MIME messages conforming to RFC 5322 (Internet Message Format) and RFC 2045-2049 (MIME).

**REQ-EMAIL-016** — Email subjects and header values containing non-ASCII characters SHALL be encoded using RFC 2047 encoded-word syntax.

**REQ-EMAIL-017** — HTML email bodies SHALL be encoded as `Content-Type: text/html; charset=utf-8` with `Content-Transfer-Encoding: quoted-printable` or `base64`.

**REQ-EMAIL-018** — When both HTML and plain-text bodies are present, the email SHALL be constructed as a `multipart/alternative` message. When attachments are also present, the message SHALL use `multipart/mixed` with a nested `multipart/alternative` part.

**REQ-EMAIL-019** — Each outbound email SHALL include a unique `Message-ID` header generated in the format `<uuid@sending-domain>` where `sending-domain` matches the domain of the From address.

**REQ-EMAIL-020** — The system SHALL insert a tracking pixel into the HTML body (if `{{tracking.pixel}}` is present in the template or if automatic tracking is enabled for the campaign). The tracking pixel URL SHALL be unique per target and resolve to the phishing endpoint.

## 5. DNS Records for Email Authentication

### 5.1 SPF (Sender Policy Framework)

**REQ-EMAIL-021** — The system SHALL generate SPF TXT records for campaign sending domains. The generated record SHALL include the IP address(es) of the phishing endpoint(s) assigned to the campaign and, optionally, any SMTP relay servers that need to be authorized.

**REQ-EMAIL-022** — The system SHALL integrate with the domain management module (03) to create or update SPF records in the DNS zone of the sending domain.

**REQ-EMAIL-023** — The system SHALL validate that the SPF record for a campaign's sending domain authorizes the phishing endpoint's IP before allowing the campaign to transition to the `ready` state. If validation fails, the system SHALL display a warning with corrective instructions. The operator SHALL be able to override the warning and proceed.

### 5.2 DKIM (DomainKeys Identified Mail)

**REQ-EMAIL-024** — The system SHALL generate DKIM key pairs (RSA 2048-bit minimum or Ed25519) for each sending domain. The private key SHALL be stored encrypted at rest.

**REQ-EMAIL-025** — The system SHALL generate the DKIM DNS TXT record (`<selector>._domainkey.<domain>`) containing the public key and publish it via the domain management module.

**REQ-EMAIL-026** — The phishing endpoint SHALL DKIM-sign all outbound emails using the private key associated with the sending domain. The signing algorithm, selector, and canonicalization method SHALL be configurable (defaults: `rsa-sha256`, relaxed/relaxed).

**REQ-EMAIL-027** — The system SHALL support per-campaign DKIM selectors to enable key rotation and prevent selector reuse across campaigns.

### 5.3 DMARC (Domain-based Message Authentication, Reporting & Conformance)

**REQ-EMAIL-028** — The system SHALL generate a DMARC TXT record (`_dmarc.<domain>`) for each sending domain with configurable policy (`none`, `quarantine`, `reject`), percentage, and reporting addresses.

**REQ-EMAIL-029** — The default DMARC policy for newly registered domains SHALL be `none` (monitoring only) to avoid blocking emails during initial setup.

**REQ-EMAIL-030** — The system SHALL publish the DMARC record via the domain management module.

### 5.4 Email Authentication Validation

**REQ-EMAIL-031** — The system SHALL provide a "Validate Email Auth" action for each campaign that checks:
- SPF record exists and includes the phishing endpoint IP
- DKIM public key record exists and matches the stored private key
- DMARC record exists and is syntactically valid
- MX records for the sending domain are configured (if applicable)

**REQ-EMAIL-032** — Validation results SHALL be displayed in the campaign setup UI with per-check pass/fail/warning status and human-readable remediation steps for failures.

**REQ-EMAIL-033** — The system SHALL re-validate email authentication records whenever a campaign's phishing endpoint IP changes (e.g., endpoint redeployment) and notify the operator if records are stale.

## 6. Delivery Tracking and Status

**REQ-SMTP-017** — The phishing endpoint SHALL report the SMTP transaction result for each email back to the framework server. The result SHALL include:

| Field | Description |
|-------|-------------|
| `target_id` | Target who was sent the email |
| `campaign_id` | Campaign the email belongs to |
| `smtp_profile_id` | SMTP profile used for this send |
| `template_id` | Email template used |
| `status` | One of: `queued`, `sending`, `sent`, `deferred`, `bounced`, `failed` |
| `smtp_response_code` | SMTP response code from the server (e.g., 250, 550) |
| `smtp_response_text` | Full SMTP response text |
| `message_id` | Message-ID header value |
| `sent_at` | Timestamp of successful send |
| `error_detail` | Error description for `deferred`, `bounced`, or `failed` statuses |
| `attempt_count` | Number of send attempts for this email |

**REQ-SMTP-018** — The system SHALL implement automatic retry logic for deferred emails with configurable parameters:
- Maximum retry attempts (default: 3)
- Retry interval (default: 5 minutes, exponential backoff)
- Retries SHALL respect the campaign send window (REQ-SMTP-015)

**REQ-SMTP-019** — The system SHALL provide a real-time delivery status dashboard showing:
- Total emails queued, sent, deferred, bounced, and failed
- Per-SMTP-profile send counts and error rates
- Delivery rate over time (emails per minute)
- Current position in the target list

**REQ-SMTP-020** — The system SHALL support pausing and resuming email delivery for a running campaign. When paused, no further SMTP transactions SHALL be initiated. When resumed, delivery SHALL continue from where it stopped.

## 7. Security Considerations

**SEC-SMTP-001** — SMTP credentials (passwords, OAuth tokens) SHALL be encrypted at rest using AES-256-GCM (or the application-level encryption standard defined in 01). Credentials SHALL never be logged, included in API responses (except masked), or transmitted in plaintext between the framework and phishing endpoint.

**SEC-SMTP-002** — Communication between the framework server and phishing endpoints (for transmitting campaign payloads including SMTP credentials) SHALL occur over a mutually authenticated TLS channel.

**SEC-SMTP-003** — DKIM private keys SHALL be encrypted at rest and transmitted to phishing endpoints only over the secure channel described in SEC-SMTP-002.

**SEC-SMTP-004** — The system SHALL not store the content of sent emails in the database by default. Only metadata (template ID, target ID, timestamp, delivery status) SHALL be persisted. An optional "archive sent emails" flag may be enabled per campaign for debugging purposes, with automatic purge after a configurable retention period (default: 7 days).

**SEC-SMTP-005** — Test emails (REQ-EMAIL-012) SHALL use the same secure transmission path as production sends. Test SMTP credentials SHALL not be handled differently from production credentials.

**SEC-SMTP-006** — The system SHALL sanitize all user-supplied values (target fields, template content) before insertion into email headers to prevent SMTP header injection and email spoofing beyond the configured sender identity.

**SEC-SMTP-007** — All SMTP profile CRUD operations, connection tests, test sends, and configuration changes SHALL be recorded in the audit log with the acting user, timestamp, and details of the change.

## 8. Acceptance Criteria

### SMTP Configuration
- [ ] An operator can create an SMTP profile with all fields defined in REQ-SMTP-002 and successfully test the connection
- [ ] SMTP passwords are not visible in the UI after initial entry, not present in API GET responses (masked as `********`), and not present in any log output
- [ ] Deleting an SMTP profile associated with an active campaign is blocked with a clear error message listing the affected campaigns
- [ ] A campaign with three SMTP profiles using `weighted` strategy distributes emails within a 5% tolerance of the configured weights over a sample of 1,000 sends
- [ ] Sending respects both per-profile and per-campaign rate limits, never exceeding the most restrictive limit

### Email Composition
- [ ] An operator can create an email template with HTML body, plain-text body, custom headers, and attachments
- [ ] Variable substitution correctly resolves all tokens listed in REQ-EMAIL-003 for a target with all fields populated
- [ ] Variable substitution for a target with missing fields produces an empty string (not an error) and logs a warning
- [ ] A/B test campaigns correctly assign templates according to configured weights and report per-template metrics
- [ ] The preview function renders the email identically to how it would appear when actually sent (same headers, same body, same attachments)
- [ ] Test emails are sent through the phishing endpoint (not the framework server) and delivery result is reported back to the UI

### Email Authentication DNS
- [ ] The system generates valid SPF, DKIM, and DMARC records and publishes them via the domain management module
- [ ] DKIM signatures on sent emails pass verification when checked against the published DNS record
- [ ] The "Validate Email Auth" action correctly identifies missing or misconfigured records and provides remediation guidance
- [ ] Changing a campaign's phishing endpoint triggers re-validation of email auth records

### Delivery Tracking
- [ ] Every sent email has a corresponding delivery status record with SMTP response details
- [ ] Failed emails are retried according to the configured retry policy
- [ ] The real-time dashboard accurately reflects delivery progress within 5 seconds of status changes
- [ ] Pausing a campaign halts email delivery within 30 seconds; resuming continues from the correct position

## 9. WYSIWYG Email Template Editor

**REQ-EMAIL-034** — The system SHALL provide a WYSIWYG (What You See Is What You Get) visual editor for composing HTML email templates, in addition to a raw HTML code editor.

| Feature | Behavior |
|---------|----------|
| **Visual editing** | Rich-text editing with formatting toolbar (bold, italic, underline, headings, lists, links, images, tables, alignment, colors, fonts). |
| **Code editor** | Raw HTML code editor with syntax highlighting, available as a toggle alongside the WYSIWYG editor. |
| **Bidirectional sync** | Changes in the WYSIWYG editor update the raw HTML and vice versa. |
| **Variable insertion** | A template variable picker (dropdown or autocomplete) allows inserting `{{variable}}` tokens directly from the WYSIWYG toolbar. |
| **Preview** | Inline preview pane showing the rendered email as it would appear in a mail client. |

Acceptance Criteria:
- [ ] The WYSIWYG editor produces clean, email-client-compatible HTML (tables-based layout for broad compatibility)
- [ ] Switching between WYSIWYG and code editor preserves content without data loss
- [ ] The variable picker lists all available template variables (from REQ-EMAIL-003) with descriptions
- [ ] The editor supports image embedding via URL reference and inline base64
- [ ] Undo/redo is supported with a minimum 30-level undo stack

---

## 10. Email Client Analytics

**REQ-EMAIL-035** — The system SHALL detect and record which email client and platform the target used to open a phishing email, based on tracking pixel request metadata.

| Data Point | Detection Method |
|------------|-----------------|
| **Email client** | User-Agent header analysis from tracking pixel requests (e.g., Outlook, Thunderbird, Apple Mail, Gmail web, Yahoo web). |
| **Platform/OS** | User-Agent parsing for operating system (Windows, macOS, iOS, Android, Linux). |
| **Device type** | Classification as desktop, mobile, or tablet based on User-Agent and viewport heuristics. |
| **Image loading behavior** | Whether images were loaded automatically (immediate pixel hit) or manually (delayed pixel hit after user action). |

**REQ-EMAIL-036** — Email client analytics SHALL be aggregated per campaign and per template variant, and surfaced on the campaign metrics dashboard.

| Metric | Display |
|--------|---------|
| **Client distribution** | Pie chart showing percentage breakdown by email client. |
| **Platform distribution** | Pie chart showing percentage breakdown by OS/platform. |
| **Device type breakdown** | Bar chart showing desktop vs. mobile vs. tablet opens. |
| **Client-specific engagement** | Table showing open rate, click rate, and capture rate segmented by email client. |

Acceptance Criteria:
- [ ] Email client detection correctly identifies at least the top 10 email clients by market share
- [ ] Unknown or unrecognizable User-Agent strings are categorized as "Other" with the raw string preserved
- [ ] Analytics are available per campaign and per template variant
- [ ] The detection logic accounts for image proxy services (e.g., Gmail image proxy) and does not double-count
- [ ] Email client data is available in campaign reports and CSV exports

---

## 11. Built-In Email Template Library

**REQ-EMAIL-037** — The system SHALL include a built-in library of email template presets covering common phishing scenarios.

| Category | Template Examples |
|----------|------------------|
| **Credential harvesting** | Password reset notification, account verification required, suspicious login alert, IT policy acknowledgment. |
| **Document/file lure** | Shared document notification, invoice attached, contract review request, file download available. |
| **IT/system notifications** | System maintenance notice, mailbox storage full, software update required, VPN reconfiguration. |
| **HR/corporate** | Benefits enrollment reminder, mandatory training notification, policy update acknowledgment, org announcement. |
| **Social engineering** | Meeting invitation, calendar event, delivery notification, payment confirmation. |

**REQ-EMAIL-038** — Template library entries SHALL include:
- Pre-written subject line and HTML body with variable substitution tokens
- Matching plain-text body
- Recommended sender name and address patterns
- Difficulty rating (how convincing the template is likely to be)
- Notes on which target demographics the template works best for

Acceptance Criteria:
- [ ] At least 20 built-in templates are available across the five categories
- [ ] Library templates are read-only but can be duplicated and customized
- [ ] Templates use proper variable substitution tokens for personalization
- [ ] Each template includes both HTML and plain-text versions
- [ ] New templates can be added to the library by Administrators

---

## 12. URL Redirect Chains

**REQ-EMAIL-039** — The system SHALL support configuring redirect chains in phishing URLs, where the target is redirected through one or more intermediate URLs before reaching the final landing page.

| Feature | Behavior |
|---------|----------|
| **Chain configuration** | The Operator defines an ordered list of redirect URLs. The phishing URL in the email points to the first URL in the chain. Each intermediate URL redirects (HTTP 302) to the next. The final redirect lands on the phishing endpoint. |
| **Use case** | Redirect chains obscure the final destination from email security gateways that inspect only the first URL in a chain. They also enable use of legitimate URL shorteners or redirect services as intermediaries. |
| **Tracking** | Each hop in the chain can optionally include tracking parameters so the system knows the target traversed the full chain. |
| **Chain types** | Support for: framework-managed redirects (using framework-controlled intermediate domains) and external redirects (using third-party URL shorteners or open redirects specified by the Operator). |

Acceptance Criteria:
- [ ] Redirect chains of up to 5 hops are supported
- [ ] Each redirect in the chain uses HTTP 302 (temporary redirect) by default, with configurable status codes (301, 302, 303, 307)
- [ ] The phishing URL template variable (`{{phishing.url}}`) resolves to the first URL in the chain (or directly to the endpoint if no chain is configured)
- [ ] Tracking parameters are preserved and forwarded through the redirect chain
- [ ] The redirect chain configuration is visible in the campaign approval review

---

## 13. Out of Scope for v1

- SMS phishing (smishing)
- Voice phishing (vishing)
- Messaging platform delivery (Slack, Teams, etc.)
- Built-in SMTP server / MTA
- Bounce processing via webhook integrations with ESP providers
- Email warm-up automation
- Inbox placement testing / seed list monitoring
