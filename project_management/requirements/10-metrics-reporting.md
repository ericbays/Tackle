# 10 — Metrics & Reporting

## 1. Purpose

This document defines the requirements for Tackle's metrics collection, real-time dashboard, and enterprise reporting subsystem. Metrics and reporting sit at the end of the campaign data pipeline — consuming events produced by email delivery (04), landing page interactions (05/08), credential capture (08), and target engagement (09) — and presenting them through a WebSocket-powered live dashboard and configurable report generation engine. All metrics flow into PostgreSQL and are surfaced through the Go backend API to the React admin UI.

## 2. Scope

This module covers:

- Real-time dashboard with WebSocket-powered live updates
- Comprehensive metric tracking across the full campaign lifecycle
- Reusable report templates with on-demand and scheduled generation
- Export to PDF and CSV
- Role-based dashboard access
- Archived campaign data handling in reporting
- Chart and visualization requirements

Out of scope:

- Target-facing analytics or tracking beyond what is captured by credential capture (08) and landing page apps (05)
- External BI tool integrations (Tackle is self-contained)
- Network-layer monitoring of phishing endpoints (covered in 07)

---

## 3. Real-Time Dashboard

### 3.1 WebSocket Event Stream

| ID | Requirement | Acceptance Criteria |
|----|-------------|---------------------|
| **REQ-MET-001** | The backend SHALL maintain a persistent WebSocket connection to each authenticated admin UI session for pushing real-time metric updates. | 1. WebSocket endpoint is available at `/api/v1/ws/metrics`. 2. Connection requires a valid session token. 3. Connection survives brief network interruptions with automatic reconnect (client-side, max 5 retries with exponential backoff). |
| **REQ-MET-002** | The backend SHALL broadcast metric events to connected WebSocket clients within 2 seconds of the underlying event being persisted to the database. | 1. Timestamp delta between DB write and WebSocket delivery is measurable. 2. 95th-percentile latency is under 2 seconds under normal load (fewer than 50 concurrent dashboard sessions). |
| **REQ-MET-003** | WebSocket messages SHALL be scoped by role — clients only receive events for campaigns and data they are authorized to view per RBAC rules defined in [02-authentication-authorization.md](02-authentication-authorization.md). | 1. A Defender-role session never receives infrastructure health events. 2. An Operator-role session does not receive events for campaigns they are not assigned to (if campaign-level scoping is enforced). 3. Unit tests verify event filtering per role. |
| **REQ-MET-004** | The WebSocket connection SHALL support a subscription model where the client can subscribe to specific campaigns or metric categories to reduce unnecessary traffic. | 1. Client can send a `subscribe` message specifying campaign IDs and/or event categories. 2. Server filters outbound messages accordingly. 3. Client can update subscriptions without reconnecting. |

### 3.2 Campaign-Level Metrics (Live)

| ID | Requirement | Acceptance Criteria |
|----|-------------|---------------------|
| **REQ-MET-005** | The dashboard SHALL display the following campaign-level counters in real time: emails sent, emails delivered, emails bounced, emails opened, links clicked, credentials captured, and phishing emails reported (by targets to their security team). | 1. Each counter updates within 2 seconds of the underlying event. 2. Counters are accurate to within eventual consistency (no double-counting). 3. All seven counters are visible on the campaign detail view. |
| **REQ-MET-006** | The dashboard SHALL compute and display the following derived rates in real time: open rate (opened / delivered), click-through rate (clicked / delivered), and credential capture rate (credentials captured / clicked). | 1. Rates are displayed as percentages with one decimal place. 2. Division-by-zero cases display "N/A" rather than an error. 3. Rates update automatically when underlying counters change. |
| **REQ-MET-007** | The dashboard SHALL present a chronological activity feed showing individual campaign events (e.g., "Target X opened email", "Target Y submitted credentials") as they occur. | 1. Feed updates in real time via WebSocket. 2. Each entry shows: timestamp (local timezone), event type, anonymized or full target identifier (configurable per role), and campaign name. 3. Feed is scrollable and retains at least the most recent 500 events in the browser. 4. New events appear at the top of the feed. |

### 3.3 Filtering and Segmentation

| ID | Requirement | Acceptance Criteria |
|----|-------------|---------------------|
| **REQ-MET-008** | All dashboard views SHALL support filtering by: campaign (single or multi-select), target group, template variant (A/B identifier), time range (preset and custom), and event status. | 1. Filters are combinable (AND logic). 2. Changing a filter updates all visible charts, counters, and the activity feed within 3 seconds. 3. Active filters are visually indicated and clearable. |
| **REQ-MET-009** | The dashboard SHALL support a global date-range picker with presets: "Last 1 hour", "Last 24 hours", "Last 7 days", "Last 30 days", "Campaign lifetime", and a custom range selector. | 1. Preset selection immediately applies the filter. 2. Custom range allows selection of start and end timestamps with minute granularity. 3. The selected range persists across page navigation within the same session. |
| **REQ-MET-010** | The dashboard SHALL display a geographic distribution view of target interactions when IP geolocation data is available. | 1. Interactions with resolved geolocation are plotted on a map or displayed in a sortable table by country/region. 2. If geolocation is unavailable for an interaction, it is categorized as "Unknown" rather than omitted. 3. Geolocation resolution uses a local GeoIP database (e.g., MaxMind GeoLite2) — no external API calls at query time. |

### 3.4 Security Considerations — Dashboard

| ID | Requirement | Acceptance Criteria |
|----|-------------|---------------------|
| **REQ-MET-011** | WebSocket connections SHALL enforce the same authentication and session expiry rules as REST API endpoints. | 1. An expired session token causes the server to close the WebSocket with code 4401. 2. No metric data is sent after the session expires. 3. The client handles this closure by redirecting to the login page. |
| **REQ-MET-012** | All WebSocket messages SHALL be transmitted over TLS (wss://). Plaintext ws:// connections SHALL be rejected by the server. | 1. Server listener refuses upgrade requests on non-TLS connections. 2. Integration test confirms ws:// is rejected. |

---

## 4. Metrics Tracked

### 4.1 Email Metrics

| ID | Requirement | Acceptance Criteria |
|----|-------------|---------------------|
| **REQ-MET-013** | The system SHALL track and store the following per-email events: **sent** (email handed to SMTP relay), **delivered** (no bounce received within configurable window, default 30 minutes), **bounced** (DSN received or SMTP rejection), **opened** (tracking pixel loaded), and **link clicked** (target visited tracked URL). | 1. Each event is stored with: event type, timestamp (UTC), campaign ID, target ID, email template ID, and phishing endpoint ID. 2. A single email can generate multiple "opened" and "link clicked" events (each recorded). 3. "Delivered" is inferred — the system marks sent emails as delivered after the bounce window expires without a corresponding bounce. |
| **REQ-MET-014** | Tracking pixel events (email opens) SHALL record the source IP address and User-Agent header of the requesting client. | 1. IP and User-Agent are stored alongside the open event. 2. If the request lacks a User-Agent, the field is stored as an empty string, not null. 3. These fields are available for filtering and geolocation resolution. |

### 4.2 Landing Page Metrics

| ID | Requirement | Acceptance Criteria |
|----|-------------|---------------------|
| **REQ-MET-015** | The system SHALL track the following per-visit landing page metrics: **page view** (page loaded), **time on page** (duration from load to unload/navigation, reported via beacon or heartbeat), **form interaction** (any input field focused or changed), and **credential submission** (form submitted with captured data). | 1. Page view is recorded on initial page load with: timestamp, campaign ID, target ID (from tracking token), source IP, User-Agent. 2. Time on page is reported to the nearest second. 3. Form interaction is a boolean flag per visit — not a count of individual field interactions. 4. Credential submission links to the credential record in the credential capture module (08). |
| **REQ-MET-016** | Landing page metric collection SHALL NOT introduce detectable artifacts (additional script tags, unique DOM attributes, or recognizable beacon URLs) that could be used to fingerprint the page as a phishing simulation tool. | 1. Metric collection code is embedded during the landing page build process and obfuscated per build. 2. Beacon/heartbeat URLs do not contain identifiable strings such as "tackle", "metric", or "track". 3. Review of generated page source by a security-aware reviewer confirms no obvious fingerprinting artifacts. |

### 4.3 Target Engagement Metrics

| ID | Requirement | Acceptance Criteria |
|----|-------------|---------------------|
| **REQ-MET-017** | The system SHALL compute and store per-target engagement metrics: **first interaction time** (elapsed time from email delivery to first open or click), **total interactions** (count of all tracked events for the target in a campaign), and **conversion funnel stage** (furthest stage reached: delivered -> opened -> clicked -> submitted credentials). | 1. First interaction time is stored in seconds. 2. Funnel stage is updated automatically as new events arrive. 3. Metrics are queryable per target and per campaign. |
| **REQ-MET-018** | The system SHALL support conversion funnel analysis showing the number and percentage of targets at each stage: Emails Sent -> Delivered -> Opened -> Clicked -> Credentials Submitted. | 1. Funnel data is available via API endpoint and rendered as a funnel chart on the dashboard. 2. Each stage shows absolute count and percentage relative to the previous stage and relative to total sent. 3. Funnel can be filtered by target group and template variant. |

### 4.4 Campaign-Level Aggregates

| ID | Requirement | Acceptance Criteria |
|----|-------------|---------------------|
| **REQ-MET-019** | The system SHALL compute campaign-level aggregate metrics: **overall success rate** (credentials captured / emails delivered), **engagement rate** (any interaction / emails delivered), and **engagement rate by department or target group**. | 1. Aggregates are recomputed within 30 seconds of a new event arriving. 2. Department/group breakdown is available if targets have group metadata (see 09). 3. Aggregates are exposed via a dedicated API endpoint. |
| **REQ-MET-020** | The system SHALL track temporal engagement patterns: **engagement by hour of day** (0-23) and **engagement by day of week** (Monday-Sunday), aggregated across the campaign duration. | 1. Temporal data uses the timezone configured for the campaign (or UTC if none). 2. Data is presented as bar charts or heatmaps on the dashboard. 3. Both "opened" and "clicked" events are counted for temporal analysis. |
| **REQ-MET-021** | The system SHALL support template comparison metrics for campaigns using A/B testing: per-variant open rate, click-through rate, credential capture rate, and average time to first interaction. | 1. Comparison is only available for campaigns with two or more template variants. 2. Metrics are displayed side-by-side in a comparison table and/or chart. 3. Statistical significance is NOT required in v1 — raw rates are sufficient. |

---

## 5. Enterprise Reporting

### 5.1 Report Templates

| ID | Requirement | Acceptance Criteria |
|----|-------------|---------------------|
| **REQ-RPT-001** | Engineers and Operators SHALL be able to create, edit, duplicate, and delete reusable report templates. | 1. Template CRUD operations are available via API and admin UI. 2. Defenders cannot create or modify templates (read-only access to generated reports only). 3. Templates are stored in the database and associated with the creating user. |
| **REQ-RPT-002** | A report template SHALL define: a human-readable name, a description, which metric categories to include (email, landing page, engagement, aggregates, temporal, template comparison), chart types per section, default date range, default filter criteria (campaigns, groups, statuses), and layout order of sections. | 1. All fields are persisted and editable after creation. 2. At least one metric category must be selected — the system rejects empty templates. 3. Layout order is a sortable list of section identifiers. |
| **REQ-RPT-003** | The system SHALL provide at least three built-in (system) report templates that cannot be deleted: **Campaign Summary** (all key metrics for a single campaign), **Executive Overview** (high-level success/engagement rates across multiple campaigns), and **Trend Analysis** (campaign-over-campaign comparison over time). | 1. Built-in templates are available immediately after system initialization. 2. Built-in templates can be duplicated but not modified or deleted. 3. Users can create custom templates based on duplicated built-in templates. |

### 5.2 Report Generation

| ID | Requirement | Acceptance Criteria |
|----|-------------|---------------------|
| **REQ-RPT-004** | Reports SHALL be generable on-demand by any authorized user (Operator, Engineer, Administrator) by selecting a template and specifying or overriding the date range and filter criteria. | 1. Generation is initiated from the admin UI or via API. 2. The user can override any default filter from the template at generation time. 3. Generation completes within 30 seconds for campaigns with up to 10,000 targets. |
| **REQ-RPT-005** | Reports SHALL be schedulable for automatic generation at recurring intervals: daily, weekly, or monthly. | 1. Schedule is configured per report template with a specific time of day (UTC). 2. Scheduled reports are generated by a backend background worker. 3. Generated reports are stored and accessible from the admin UI. 4. If generation fails, the failure is logged and the schedule continues on the next interval. |
| **REQ-RPT-006** | Generated reports SHALL be exportable in **PDF** format (formatted with charts, tables, and Tackle branding) and **CSV** format (raw tabular data). | 1. PDF export produces a readable, professional document with embedded charts rendered as images. 2. CSV export includes column headers and one row per data point — no charts. 3. Both formats include a generation timestamp and the filter criteria used. 4. PDF generation uses a server-side rendering library (no headless browser dependency). |
| **REQ-RPT-007** | Generated reports SHALL be stored in the database with metadata: template used, generation timestamp, generating user (or "system" for scheduled), filter criteria applied, and file size. | 1. Reports are retrievable by ID via API. 2. Reports list view supports sorting by date and filtering by template. 3. Old reports can be manually deleted by Operators, Engineers, or Administrators. |

### 5.3 Charts and Visualizations

| ID | Requirement | Acceptance Criteria |
|----|-------------|---------------------|
| **REQ-RPT-008** | The reporting engine SHALL support the following chart types: **bar charts** (vertical and horizontal), **line charts** (single and multi-series), **pie charts**, **trend lines** (overlaid on bar or line charts), and **funnel charts**. | 1. Each chart type renders correctly in both the dashboard (interactive, via React charting library) and PDF exports (static image). 2. Charts include axis labels, legends, and data labels where appropriate. 3. The charting library is consistent across dashboard and export — no visual discrepancies between live view and PDF. |
| **REQ-RPT-009** | Reports SHALL support comparison views: **A/B test comparison** (side-by-side metrics for template variants within a campaign) and **campaign-over-campaign trends** (the same metrics plotted across multiple campaigns over time). | 1. A/B comparison displays variant labels and all six core rates (open, click, credential capture, bounce, engagement, first-interaction time). 2. Campaign trend view allows selection of 2 to 20 campaigns on the same time-series chart. 3. Both views are available as dashboard widgets and as report template sections. |

### 5.4 Security Considerations — Reporting

| ID | Requirement | Acceptance Criteria |
|----|-------------|---------------------|
| **REQ-RPT-010** | Generated report files (PDF, CSV) SHALL be accessible only to users whose role permits viewing the underlying data. The API SHALL enforce RBAC checks on report download endpoints. | 1. A Defender can download reports containing only metrics data (not infrastructure health). 2. An unauthenticated request to the report download endpoint returns 401. 3. A role-insufficient request returns 403. |
| **REQ-RPT-011** | Report files SHALL NOT contain raw captured credentials. Credential data in reports SHALL be limited to: count of submissions, anonymized or hashed usernames (first and last character visible, remainder masked), and no passwords. | 1. PDF and CSV outputs are auditable for absence of plaintext passwords. 2. Username masking follows the pattern: `j***n@example.com`. 3. If a credential has no username, it is represented as "[no username]". |
| **REQ-RPT-012** | Scheduled report generation SHALL execute with a system service account that has read-only access to metric data. The service account SHALL NOT have write access to campaign or infrastructure resources. | 1. The service account's permissions are defined in the RBAC model and cannot be escalated via the admin UI. 2. Scheduled generation logs include the service account identity for audit purposes. |

### 4.5 Email Client Analytics

| ID | Requirement | Acceptance Criteria |
|----|-------------|---------------------|
| **REQ-MET-036** | The system SHALL track and display email client detection data based on tracking pixel request metadata, including email client name, platform/OS, and device type (desktop/mobile/tablet). | 1. Email client detection uses User-Agent analysis from tracking pixel requests. 2. At least the top 10 email clients by market share are correctly identified. 3. Unknown clients are categorized as "Other" with the raw User-Agent preserved. |
| **REQ-MET-037** | The dashboard SHALL display email client analytics as: client distribution pie chart, platform distribution pie chart, device type bar chart, and a client-specific engagement table showing open rate, click rate, and capture rate per client. | 1. Charts render on the campaign detail metrics view. 2. Data is filterable by template variant for A/B analysis. 3. Email client data is included in exported reports. |
| **REQ-MET-038** | The system SHALL account for email client image proxy services (e.g., Gmail Image Proxy, Apple Privacy Protection) that can obscure the true client identity or inflate open counts. | 1. Known image proxy User-Agents are flagged and categorized separately. 2. The dashboard displays a note when proxy-attributed opens exceed a configurable threshold (default: 20%). 3. Proxy-attributed opens are included in total counts but visually distinguished in charts. |

### 4.6 Auto-Summary on Campaign Completion

| ID | Requirement | Acceptance Criteria |
|----|-------------|---------------------|
| **REQ-MET-039** | The system SHALL automatically generate a lightweight campaign summary within 60 seconds of a campaign reaching Completed state. | 1. The summary includes: total emails sent/delivered/bounced, open rate, click-through rate, credential capture rate, top 5 targets by engagement, campaign duration, and A/B variant winner (if applicable). 2. The summary is displayed prominently on the campaign detail view. 3. Summary generation does not block the completion state transition. |
| **REQ-MET-040** | The system SHALL support on-demand generation of a comprehensive campaign report from the campaign detail view, in addition to the auto-generated lightweight summary. | 1. A "Generate Full Report" button is available on the campaign detail view for Completed campaigns. 2. The comprehensive report includes all metrics, charts, per-target breakdown, timeline, geographic distribution, and template comparison. 3. The comprehensive report uses the standard report generation pipeline (REQ-RPT-004 through REQ-RPT-007). |

---

## 6. Dashboard Access by Role

| ID | Requirement | Acceptance Criteria |
|----|-------------|---------------------|
| **REQ-MET-022** | The **Defender** role SHALL have read-only access to the metrics dashboard via a dedicated Defender Dashboard view, limited to: campaign-level aggregate metrics, conversion funnel, temporal patterns, organizational risk metrics, and generated reports. Defenders SHALL NOT see per-target detail, infrastructure health data, or raw event feeds with target identifiers. | 1. A dedicated Defender Dashboard is available as a distinct navigation item, separate from the Operator/Engineer dashboards. 2. API endpoints enforce field-level filtering — per-target data is stripped from responses. 3. Attempting to access restricted endpoints returns 403. |
| **REQ-MET-023** | The **Operator** role SHALL have full access to all campaign metrics including per-target detail, activity feeds, and all chart types. Operators SHALL also have campaign-specific filtered views scoped to campaigns they manage. | 1. Operator dashboard includes all widgets. 2. Campaign selector defaults to campaigns assigned to the operator (with option to view all if permitted). 3. Per-target data is fully visible to Operators. |
| **REQ-MET-024** | The **Engineer** role SHALL have full access to all campaign metrics plus infrastructure health metrics: phishing endpoint status, uptime, request throughput, error rates, and resource utilization. | 1. Engineer dashboard includes an "Infrastructure" tab not visible to other roles (except Administrator). 2. Infrastructure metrics update in real time via the same WebSocket connection. 3. Endpoint health data includes: status (healthy/degraded/down), last health check timestamp, and error count in the last hour. |
| **REQ-MET-025** | The **Administrator** role SHALL have unrestricted access to all metrics, reports, templates, and dashboard views including those defined for Defender, Operator, and Engineer roles. | 1. No dashboard widget, API endpoint, or report is restricted for the Administrator. 2. Administrator can view and manage all report templates regardless of creator. |

### 6.1 Dedicated Defender Dashboard

| ID | Requirement | Acceptance Criteria |
|----|-------------|---------------------|
| **REQ-MET-041** | The system SHALL provide a dedicated Defender Dashboard as a distinct view optimized for security awareness program management, separate from the operational dashboards used by Operators and Engineers. | 1. The Defender Dashboard is accessible from the main navigation for users with the Defender, Engineer, or Administrator role. 2. The dashboard focuses on organizational risk posture and awareness trends, not individual campaign operations. 3. The layout and widgets are designed for presentation to security leadership. |
| **REQ-MET-042** | The Defender Dashboard SHALL display: overall organizational susceptibility score (percentage of targets who submitted credentials across all campaigns), susceptibility trend over time (line chart across campaigns), department/group-level risk heatmap, campaign effectiveness comparison, and phishing report rates. | 1. Susceptibility score updates within 30 seconds of new campaign data. 2. Department/group breakdowns use target metadata from the Target Management module (09). 3. Phishing report rates show the percentage of targets who correctly identified and reported the phishing email. |
| **REQ-MET-043** | The Defender Dashboard SHALL support date range filtering and campaign selection to scope the displayed metrics. | 1. Defenders can filter by date range (presets and custom). 2. Defenders can select specific campaigns or view aggregate data across all campaigns. 3. An "Include archived" toggle is available to include historical data. |

---

## 7. Archived Campaign Data

| ID | Requirement | Acceptance Criteria |
|----|-------------|---------------------|
| **REQ-MET-026** | Metric data associated with archived campaigns SHALL be immutable. No new events SHALL be recorded against an archived campaign, and existing metric records SHALL NOT be modified or deleted. | 1. API rejects POST/PUT/DELETE requests targeting metric records of archived campaigns with HTTP 409 (Conflict). 2. Database triggers or application-level guards prevent modification. 3. Attempting to record a new event against an archived campaign logs a warning and discards the event. |
| **REQ-MET-027** | Dashboard and report filters SHALL support explicit inclusion or exclusion of archived campaign data. The default filter behavior SHALL exclude archived campaigns. | 1. A toggle or filter option labeled "Include archived campaigns" is available on the dashboard and in report template filter criteria. 2. When excluded (default), archived campaign data does not appear in any aggregate computation. 3. When included, archived data is visually distinguished (e.g., muted color, label badge). |
| **REQ-MET-028** | Historical trend analysis reports SHALL be able to include archived campaign data when the user explicitly requests it, enabling long-term trend visibility across the full history of campaigns. | 1. The "Trend Analysis" built-in template includes an "Include archived" option. 2. Trend charts render archived and active campaign data points on the same axis. 3. The report output clearly labels which data points come from archived campaigns. |

---

## 8. Performance Requirements

| ID | Requirement | Acceptance Criteria |
|----|-------------|---------------------|
| **REQ-MET-029** | Dashboard initial load (all widgets, current campaign) SHALL complete within 3 seconds for campaigns with up to 10,000 targets. | 1. Measured from navigation to full widget render. 2. Tested with a seeded database of 10,000 targets and 50,000 events. 3. Backend API response time for the initial data payload is under 1 second. |
| **REQ-MET-030** | The WebSocket event stream SHALL support at least 50 concurrent dashboard sessions without degradation in event delivery latency (95th percentile under 2 seconds). | 1. Load test with 50 concurrent WebSocket clients receiving events from an active campaign. 2. Event delivery latency is logged and measurable. |
| **REQ-MET-031** | Report generation for campaigns with up to 10,000 targets SHALL complete within 30 seconds for PDF output and 10 seconds for CSV output. | 1. Timed from API request to file availability. 2. Tested with the "Campaign Summary" built-in template. 3. For larger campaigns, the system returns an estimated completion time and notifies the user when the report is ready. |

---

## 9. Data Integrity and Security

| ID | Requirement | Acceptance Criteria |
|----|-------------|---------------------|
| **REQ-MET-032** | All metric events SHALL be written to PostgreSQL with ACID guarantees. No metric event SHALL be lost due to application crashes during write operations. | 1. Events use database transactions. 2. The write-ahead log ensures durability. 3. A crash-recovery test (kill backend mid-write, restart) confirms no data loss for committed transactions. |
| **REQ-MET-033** | Metric data at rest SHALL be protected by PostgreSQL's encryption-at-rest configuration as defined in [14-database-schema.md](14-database-schema.md). Sensitive fields (IP addresses, User-Agent strings) SHALL follow the same encryption policy as other PII. | 1. IP address and User-Agent fields are stored in encrypted columns or the database volume is encrypted. 2. A database dump does not expose plaintext IP addresses. |
| **REQ-MET-034** | All metric API endpoints SHALL require authentication and enforce RBAC as defined in [02-authentication-authorization.md](02-authentication-authorization.md). There SHALL be no unauthenticated access to any metric data. | 1. Every `/api/v1/metrics/*` and `/api/v1/reports/*` endpoint returns 401 for unauthenticated requests. 2. Role-based access is verified per the rules in Section 6. 3. Integration tests cover all endpoints for each role. |
| **REQ-MET-035** | Metric collection (tracking pixels, landing page beacons) SHALL NOT leak information about the Tackle framework to targets. Tracking endpoints SHALL return generic responses (1x1 transparent GIF for pixels, 204 for beacons) with no identifying headers. | 1. Response headers do not include "X-Powered-By", "Server: Tackle", or any framework-identifying string. 2. Tracking pixel response is exactly a 1x1 transparent GIF with `Content-Type: image/gif`. 3. Beacon responses return HTTP 204 with no body. |

---

## 10. API Endpoints (Summary)

The following API endpoints support this module. Detailed request/response schemas will be defined during implementation.

| Method | Endpoint | Description | Minimum Role |
|--------|----------|-------------|--------------|
| GET | `/api/v1/metrics/campaigns/{id}` | Campaign-level aggregate metrics | Defender |
| GET | `/api/v1/metrics/campaigns/{id}/events` | Paginated event list for a campaign | Operator |
| GET | `/api/v1/metrics/campaigns/{id}/funnel` | Conversion funnel data | Defender |
| GET | `/api/v1/metrics/campaigns/{id}/temporal` | Temporal engagement patterns | Defender |
| GET | `/api/v1/metrics/campaigns/{id}/geo` | Geographic distribution of interactions | Operator |
| GET | `/api/v1/metrics/campaigns/{id}/comparison` | A/B variant comparison | Operator |
| GET | `/api/v1/metrics/trends` | Cross-campaign trend data | Operator |
| GET | `/api/v1/metrics/infrastructure` | Phishing endpoint health metrics | Engineer |
| WS | `/api/v1/ws/metrics` | WebSocket stream for real-time events | Defender |
| GET | `/api/v1/reports/templates` | List report templates | Defender |
| POST | `/api/v1/reports/templates` | Create a report template | Operator |
| PUT | `/api/v1/reports/templates/{id}` | Update a report template | Operator |
| DELETE | `/api/v1/reports/templates/{id}` | Delete a report template | Operator |
| POST | `/api/v1/reports/generate` | Generate a report on-demand | Operator |
| GET | `/api/v1/reports` | List generated reports | Defender |
| GET | `/api/v1/reports/{id}` | Download a generated report | Defender |
| DELETE | `/api/v1/reports/{id}` | Delete a generated report | Operator |
| POST | `/api/v1/reports/schedules` | Create a report schedule | Operator |
| GET | `/api/v1/reports/schedules` | List report schedules | Operator |
| PUT | `/api/v1/reports/schedules/{id}` | Update a report schedule | Operator |
| DELETE | `/api/v1/reports/schedules/{id}` | Delete a report schedule | Operator |

---

## 11. Dependencies

| Dependency | Document | Nature |
|------------|----------|--------|
| Authentication & RBAC | [02-authentication-authorization.md](02-authentication-authorization.md) | Role enforcement on all endpoints and WebSocket connections |
| SMTP / Email Delivery | [04-smtp-configuration.md](04-smtp-configuration.md) | Source of email sent/delivered/bounced events |
| Landing Page Builder | [05-landing-page-builder.md](05-landing-page-builder.md) | Landing pages embed metric collection code during build |
| Campaign Management | [06-campaign-management.md](06-campaign-management.md) | Campaigns are the primary organizational unit for all metrics |
| Phishing Endpoints | [07-phishing-endpoints.md](07-phishing-endpoints.md) | Endpoints relay tracking events; infrastructure health metrics originate here |
| Credential Capture | [08-credential-capture.md](08-credential-capture.md) | Credential submission events feed into metrics |
| Target Management | [09-target-management.md](09-target-management.md) | Target metadata (groups, departments) used for segmentation and filtering |
| Database Schema | [14-database-schema.md](14-database-schema.md) | Metric and report tables, indexes, encryption-at-rest policy |
| Audit Logging | [11-audit-logging.md](11-audit-logging.md) | Report generation and template changes are auditable events |
