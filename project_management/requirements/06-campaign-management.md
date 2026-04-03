# 06 — Campaign Management

## 1. Purpose

Campaign Management is the central orchestration module of the Tackle framework. It ties together all upstream components — domains, SMTP configurations, landing pages, email templates, and target lists — into a single executable unit: the phishing campaign. This document specifies the campaign lifecycle state machine, configuration requirements, approval workflow, A/B testing capabilities, and all associated business rules.

Campaign Management is the primary workflow that Operators interact with day-to-day. Getting the state machine right is critical because it governs what actions are permitted, what infrastructure is provisioned, and when emails are sent to real targets.

## 2. Roles Referenced

| Role | Relevant Permissions |
|------|---------------------|
| **Operator** | Create, configure, submit, build, test, launch, pause, complete, archive campaigns |
| **Engineer** | Approve or reject campaigns; Unlock is triggered by Operator but was previously approved by Engineer |
| **Administrator** | All Engineer permissions; required approver when campaign targets include global block list entries |

See [02-authentication-authorization.md](02-authentication-authorization.md) for full RBAC definitions.

## 3. Campaign Lifecycle State Machine

### 3.1 State Diagram

```
                                    ┌─────────────────────────────┐
                                    │                             │
                           Reject   │         ┌───Unlock────┐     │
                        (w/feedback)│         │             │     │
                                    │         │             ▼     │
┌───────────┐  Submit   ┌───────────┴─┐  Approve  ┌───────────┐   │
│           ├──────────►│   Pending   ├──────────►│           │   │
│   Draft   │           │  Approval   │           │  Approved │───┘
│           │◄──────────┤             │           │(read-only)│
└─────▲─────┘           └─────────────┘           └─────┬─────┘
      │                                                 │
      │                                          Build  │
      │                                                 ▼
      │                                          ┌───────────┐
      │                         Unlock           │           │
      └──────────────────────────────────────────┤ Building  │
                                                 │           │
                                                 └─────┬─────┘
                                                       │
                                                Build  │
                                              Complete │
                                                       ▼
                                                 ┌───────────┐
                         Unlock                  │           │
      ┌──────────────────────────────────────────┤   Ready   │
      │                                          │           │
      │                                          └─────┬─────┘
      │                                                │
      │                                        Launch  │
      │                                                ▼
      │                                          ┌───────────┐
      │                              Pause       │           │  Resume
      │                           ┌──────────────┤  Active   │◄──────────┐
      │                           │              │           │           │
      │                           │              └─────┬─────┘           │
      │                           ▼                    │                 │
      │                     ┌───────────┐              │                 │
      │                     │           ├──────────────┘                 │
      │                     │  Paused   │                                │
      │                     │           ├────────────────────────────────┘
      │                     └─────┬─────┘
      │                           │
      │                  Complete │ (manual)
      │                           │
      │                           ▼
      │      Complete      ┌───────────┐
      │   (all sent or     │           │
      │    manual)         │ Completed │◄─── (also from Active when
      └───────────────────►│           │      all emails sent)
                           └─────┬─────┘
                                 │
                          Archive│
                                 ▼
                           ┌───────────┐
                           │           │
                           │ Archived  │  ← TERMINAL (no transitions out)
                           │           │
                           └───────────┘
```

### 3.2 State Definitions

| # | State | Description | Operator Can Edit Config? | Infrastructure Active? |
|---|-------|-------------|--------------------------|----------------------|
| 1 | **Draft** | Campaign is being configured. Operator assembles all components (landing page, targets, SMTP, templates, schedule, endpoint config). | Yes | No |
| 2 | **Pending Approval** | Operator has submitted the campaign for Engineer review. Configuration is frozen pending decision. | No | No |
| 3 | **Approved** | Engineer has approved the campaign. Configuration is read-only to the Operator. Operator may now trigger the Build process. | No (read-only) | No |
| 4 | **Building** | The framework is provisioning infrastructure: spinning up the phishing endpoint (cloud VM), deploying the transparent reverse proxy to that endpoint, starting the campaign-specific landing page application on the framework server. | No | Provisioning |
| 5 | **Ready** | Build is complete. All infrastructure is live. Operator can visit the landing page via the phishing endpoint, send test emails, and verify everything works before launching to real targets. | No | Yes (idle) |
| 6 | **Active** | Campaign is live. Emails are being sent according to the configured schedule. The phishing endpoint is serving the landing page. All target interactions are tracked. | No | Yes (active) |
| 7 | **Paused** | Email sending is paused. No new emails are dispatched. However, the phishing endpoint remains active and continues to serve the landing page and track any target interactions (e.g., targets who already received emails and click later). | No | Yes (partial) |
| 8 | **Completed** | Campaign is finished. Either all emails have been sent and the campaign ended naturally, or the Operator manually completed it. Infrastructure may be torn down or kept briefly for late interactions (configurable). | No | Tear-down / Grace period |
| 9 | **Archived** | Campaign data is frozen and immutable. The campaign can be filtered out of active reports and dashboards but all data remains in the database. No infrastructure is running. | No | No |

### 3.3 Transition Rules

| # | From State | To State | Trigger | Actor | Conditions |
|---|-----------|----------|---------|-------|------------|
| T1 | Draft | Pending Approval | Submit | Operator | All required configuration fields are populated and valid (see REQ-CAMP-012) |
| T2 | Pending Approval | Approved | Approve | Engineer (or Administrator if block list targets present) | Reviewer has examined all configuration |
| T3 | Pending Approval | Draft | Reject | Engineer / Administrator | Rejection comment is mandatory |
| T4 | Approved | Building | Build | Operator | Operator triggers build; system begins provisioning |
| T5 | Building | Ready | Build Complete | System (automatic) | All infrastructure provisioned and health-checked successfully |
| T6 | Building | Draft | Build Failed | System (automatic) | Infrastructure provisioning failed; Operator must fix and re-submit |
| T7 | Ready | Active | Launch | Operator | Operator confirms launch; email sending begins |
| T8 | Active | Paused | Pause | Operator | Sending halted; endpoint stays up |
| T9 | Paused | Active | Resume | Operator | Sending resumes from where it left off |
| T10 | Active | Completed | Complete (manual) | Operator | Operator manually ends the campaign |
| T11 | Active | Completed | All Sent | System (automatic) | All scheduled emails have been sent and the campaign end date/time has passed |
| T12 | Paused | Completed | Complete (manual) | Operator | Operator ends a paused campaign |
| T13 | Completed | Archived | Archive | Operator | Infrastructure is fully torn down; data becomes immutable |
| T14 | Approved | Draft | Unlock | Operator | Campaign returns to Draft; previous approval is voided; must re-submit and re-approve |
| T15 | Ready | Draft | Unlock | Operator | Infrastructure is torn down; campaign returns to Draft; must re-submit, re-approve, and re-build |

### 3.4 Terminal State

**Archived** is the terminal state. Once a campaign is archived:
- No state transitions are permitted.
- All campaign data (configuration, emails sent, credentials captured, metrics, logs) is immutable.
- The campaign can be filtered out of default list views and reports but remains queryable.
- Infrastructure associated with the campaign has been fully terminated.

### 3.5 Build Failure Handling

If the build process fails (T6), the system must:
1. Roll back any partially provisioned infrastructure (terminate VMs, release IPs, clean up DNS).
2. Log the failure with full error context.
3. Transition the campaign back to **Draft** state.
4. Notify the Operator of the failure with actionable error details.
5. The Operator must fix the issue, re-submit for approval, and rebuild.

## 4. Campaign Configuration

### 4.1 Required Fields

**REQ-CAMP-001** — Campaign Basic Information

A campaign SHALL have the following basic fields:

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `name` | string | Yes | Human-readable campaign name. Must be unique across non-archived campaigns. Max 255 characters. |
| `description` | text | No | Free-text description of the campaign purpose and scope. |
| `start_date` | datetime | Yes | Earliest date/time the campaign may begin sending emails (in Active state). |
| `end_date` | datetime | Yes | Date/time after which no more emails will be sent. Must be after `start_date`. |
| `created_by` | user_id (FK) | Yes (auto) | The Operator who created the campaign. Set automatically. |
| `current_state` | enum | Yes (auto) | Current lifecycle state. Initialized to `Draft`. |
| `state_changed_at` | datetime | Yes (auto) | Timestamp of the most recent state transition. |

Acceptance Criteria:
- [ ] Campaign name uniqueness is enforced at the database level (unique constraint scoped to non-archived campaigns)
- [ ] `end_date` must be strictly after `start_date`; the API rejects invalid date ranges with a descriptive error
- [ ] All datetime fields are stored and transmitted in UTC

---

**REQ-CAMP-002** — Landing Page Association

A campaign SHALL be associated with exactly one landing page (built via the Landing Page Builder, see [05-landing-page-builder.md](05-landing-page-builder.md)).

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `landing_page_id` | FK → landing_pages | Yes | Reference to the landing page that will be compiled and deployed for this campaign. |

Acceptance Criteria:
- [ ] The referenced landing page must exist and be in a deployable state
- [ ] Changing the landing page after approval requires an Unlock back to Draft
- [ ] The landing page is compiled into a standalone application during the Build phase

---

**REQ-CAMP-003** — Target Association

A campaign SHALL be associated with one or more target lists and/or target groups.

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `target_list_ids` | FK[] → target_lists | At least one list or group | References to target lists to include. |
| `target_group_ids` | FK[] → target_groups | At least one list or group | References to target groups to include. |

The effective target set for a campaign is the **union** of all targets from all associated lists and groups, **deduplicated** by email address. If the same email address appears in multiple lists/groups, it receives exactly one set of campaign emails.

Acceptance Criteria:
- [ ] At least one target list or target group must be associated before submission
- [ ] Deduplication by email address is applied at build/launch time
- [ ] The total effective target count is displayed in the campaign configuration UI
- [ ] Targets added to associated lists/groups after the campaign reaches Approved state are NOT included (target set is snapshotted at build time)

---

**REQ-CAMP-004** — SMTP Configuration Association

A campaign SHALL be associated with one or more SMTP configurations (see [04-smtp-configuration.md](04-smtp-configuration.md)).

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `smtp_config_ids` | FK[] → smtp_configs | Yes (at least one) | SMTP server configurations to use for sending. |

When multiple SMTP configurations are associated, the sending engine distributes emails across them in a round-robin pattern to spread sending volume.

Acceptance Criteria:
- [ ] At least one SMTP configuration must be associated before submission
- [ ] All referenced SMTP configurations must have passed a connectivity test
- [ ] Round-robin distribution is implemented when multiple configs are present
- [ ] If an SMTP config fails during sending, the system retries with the next available config and logs the failure

---

**REQ-CAMP-005** — Email Template Association and A/B Testing Configuration

A campaign SHALL be associated with one or more email templates.

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `template_variants` | array | Yes (at least one entry) | List of template-variant objects, each containing a template reference and a split ratio. |
| `template_variants[].template_id` | FK → email_templates | Yes | Reference to an email template. |
| `template_variants[].split_ratio` | integer (1–100) | Yes | Percentage of the target set that receives this variant. |

Acceptance Criteria:
- [ ] At least one template variant must be configured before submission
- [ ] The sum of all `split_ratio` values must equal exactly 100
- [ ] Each template variant is independently trackable in metrics (open rate, click rate, credential capture rate)
- [ ] If only one template is used, its split_ratio must be 100
- [ ] Template variants are labeled (Variant A, Variant B, etc.) for identification in reports

---

**REQ-CAMP-006** — Sending Schedule Configuration

A campaign SHALL have a configurable sending schedule that controls when and how fast emails are dispatched.

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `send_windows` | array | No (defaults to 24/7 within date range) | Time windows during which sending is permitted (e.g., business hours only). |
| `send_windows[].days` | enum[] | Yes per window | Days of the week (Monday–Sunday). |
| `send_windows[].start_time` | time | Yes per window | Start of the window (HH:MM, in the configured timezone). |
| `send_windows[].end_time` | time | Yes per window | End of the window (HH:MM). |
| `send_windows[].timezone` | string (IANA) | Yes per window | Timezone for interpreting start/end times (e.g., `America/New_York`). |
| `throttle_rate` | integer | No (default: 0 = unlimited) | Maximum number of emails sent per minute. 0 means no throttle. |
| `inter_email_delay_min` | integer (ms) | No (default: 0) | Minimum delay in milliseconds between consecutive emails. |
| `inter_email_delay_max` | integer (ms) | No (default: 0) | Maximum delay in milliseconds. Actual delay is randomized between min and max to avoid detection. |

Acceptance Criteria:
- [ ] When no send windows are configured, emails can be sent at any time within the campaign date range
- [ ] When send windows are configured, the sending engine only dispatches emails during permitted windows
- [ ] Throttle rate is enforced globally across all SMTP configurations
- [ ] Randomized inter-email delay is applied when both min and max are set (min <= max validated)
- [ ] Send windows, throttle, and delay settings are visible in the approval review

---

**REQ-CAMP-007** — Phishing Endpoint Configuration

A campaign SHALL have configuration for the phishing endpoint that will be provisioned during the Build phase.

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `cloud_provider` | enum (AWS, Azure) | Yes | Cloud provider where the endpoint VM is deployed. |
| `region` | string | Yes | Cloud region (e.g., `us-east-1`, `eastus`). Must be a valid region for the selected provider. |
| `instance_type` | string | Yes | VM instance type (e.g., `t3.micro`, `Standard_B1s`). |
| `endpoint_domain` | FK → domains | Yes | The domain that will resolve to this endpoint. |

Acceptance Criteria:
- [ ] Region and instance type are validated against the selected cloud provider's available options
- [ ] The selected domain must be registered and have DNS managed by the framework
- [ ] During Build, the framework provisions the VM, configures the reverse proxy, and points the domain's DNS A record at the endpoint IP
- [ ] The endpoint configuration is displayed in the approval review

---

**REQ-CAMP-008** — Domain Association

A campaign SHALL be associated with a domain that is used for the phishing endpoint.

Acceptance Criteria:
- [ ] The domain must be in an active/managed state in the Domain Management module (see [03-domain-management.md](03-domain-management.md))
- [ ] A domain can only be actively used by one campaign at a time (campaigns in states Active, Paused, Ready, or Building)
- [ ] The domain is released when the campaign reaches Completed or Archived state, or when an Unlock tears down infrastructure

## 5. Approval Workflow

**REQ-CAMP-009** — Submission for Approval

When an Operator submits a campaign (Draft -> Pending Approval), the system SHALL:

1. Validate that all required configuration fields are populated (REQ-CAMP-001 through REQ-CAMP-008).
2. Snapshot the current configuration so reviewers see exactly what was submitted.
3. Check the effective target list against the **global block list**.
4. If any targets match the global block list, flag the campaign as requiring **Administrator** approval (not just Engineer).
5. Create an approval request record with a timestamp and the submitting Operator's identity.
6. Notify eligible reviewers (Engineers, or Administrators if block list flag is set) via the UI notification system.

Acceptance Criteria:
- [ ] Submission is rejected with specific validation errors if any required field is missing or invalid
- [ ] The configuration snapshot is immutable — subsequent changes to referenced entities (e.g., editing a target list) do not affect the submitted snapshot
- [ ] Block list matches are clearly enumerated in the approval view (which targets, which block list entries)
- [ ] Notification is delivered to all users with the appropriate reviewer role

---

**REQ-CAMP-010** — Approval Review Interface

The approval review interface SHALL present the following information to the reviewer:

1. Campaign name, description, and date range.
2. Full landing page preview (rendered).
3. Complete target list with count and any block list flags highlighted in red.
4. SMTP configuration(s) summary (server, port, from address — no passwords displayed).
5. Email template(s) preview with rendered personalization placeholders and A/B split ratios.
6. Sending schedule details (windows, throttle, delays).
7. Phishing endpoint configuration (provider, region, instance type, domain).
8. Submission metadata (who submitted, when, any prior rejection history for this campaign).

Acceptance Criteria:
- [ ] All eight information sections are displayed on the approval review screen
- [ ] Block list matches are visually prominent (e.g., red highlight, warning banner)
- [ ] The reviewer can view full email template renders with sample target data
- [ ] Prior rejection comments are visible if the campaign was previously rejected

---

**REQ-CAMP-011** — Approve Action

When a reviewer approves the campaign (Pending Approval -> Approved):

1. The campaign transitions to **Approved** state.
2. The approval is recorded with the reviewer's identity, timestamp, and optional comments.
3. The Operator is notified that the campaign is approved and ready for build.
4. Campaign configuration becomes read-only to the Operator.

Acceptance Criteria:
- [ ] Only users with Engineer role (or Administrator role if block list flag is set) can approve
- [ ] The approval record is stored in the audit log
- [ ] The Operator receives a UI notification upon approval

---

**REQ-CAMP-012** — Reject Action

When a reviewer rejects the campaign (Pending Approval -> Draft):

1. The campaign transitions back to **Draft** state.
2. The rejection is recorded with the reviewer's identity, timestamp, and **mandatory** rejection comments.
3. The Operator is notified of the rejection.
4. Rejection feedback is displayed prominently in the campaign editor when the Operator re-opens it.

Acceptance Criteria:
- [ ] Rejection comments are mandatory; the API rejects a reject action with empty comments
- [ ] The Operator can see all historical rejection comments for the campaign, ordered chronologically
- [ ] After rejection, the Operator can modify the campaign and re-submit

---

**REQ-CAMP-013** — Block List Escalation

If the effective target list for a campaign contains one or more email addresses that appear on the global block list:

1. The campaign MUST be flagged as requiring **Administrator** approval at submission time.
2. Engineer-level users cannot approve the campaign; only Administrators can.
3. The approval review interface MUST enumerate every blocked target with the matching block list entry and the reason recorded in the block list.
4. The Administrator must explicitly acknowledge the block list override in the approval action.

Acceptance Criteria:
- [ ] Block list check runs automatically during the Submit transition
- [ ] Engineer users see a "Requires Administrator Approval" indicator and cannot click Approve
- [ ] Administrator approval records the explicit block list override acknowledgment
- [ ] If targets are modified (via Unlock) and block list entries are removed, the campaign reverts to standard Engineer approval

---

**REQ-CAMP-014** — Unlock Action

An Operator can Unlock a campaign from **Approved** or **Ready** state to return it to **Draft**:

1. The previous approval is voided.
2. If the campaign is in **Ready** state, all provisioned infrastructure (phishing endpoint VM, DNS records, landing page app) is torn down before transitioning.
3. The campaign transitions to **Draft** state.
4. The Operator can now modify the configuration.
5. The campaign must go through the full approval and build cycle again (Submit -> Approve -> Build).

Acceptance Criteria:
- [ ] Unlock from Ready state tears down all infrastructure before transitioning
- [ ] The voided approval is recorded in the audit log with a reason
- [ ] After Unlock, the campaign shows a "Previously approved, now unlocked" indicator
- [ ] The full approval history (approvals, rejections, unlocks) is preserved and visible

## 6. A/B Testing

**REQ-CAMP-015** — A/B Test Configuration

A campaign MAY be configured with multiple email template variants for A/B testing purposes. Configuration details are specified in REQ-CAMP-005.

When multiple variants are configured:
1. The target set is partitioned according to the split ratios.
2. Target assignment to variants is randomized but deterministic (seeded by campaign ID + target ID) so that re-sends go to the same variant.
3. Each target receives exactly one variant.

Acceptance Criteria:
- [ ] Targets are assigned to variants proportionally to the configured split ratios
- [ ] Assignment is deterministic: given the same campaign and target, the same variant is always selected
- [ ] The assignment algorithm handles non-even splits correctly (e.g., 70/30 with 10 targets = 7 and 3)

---

**REQ-CAMP-016** — A/B Test Tracking

Each template variant SHALL be tracked independently:

| Metric | Tracked Per Variant |
|--------|-------------------|
| Emails sent | Yes |
| Emails delivered | Yes |
| Emails bounced | Yes |
| Opens (unique and total) | Yes |
| Clicks (unique and total) | Yes |
| Credentials captured | Yes |
| Time-to-first-click (median) | Yes |

Acceptance Criteria:
- [ ] All metrics listed above are tracked and stored per variant
- [ ] The metrics dashboard can display variant comparison side-by-side
- [ ] Export/report functionality includes variant breakdowns
- [ ] Statistical significance indicators are displayed when sample sizes are sufficient (using chi-squared test or equivalent)

---

**REQ-CAMP-017** — A/B Test Results Comparison

The campaign metrics view SHALL include an A/B comparison panel that displays:

1. Side-by-side metric cards for each variant.
2. Percentage difference for key metrics (click rate, credential capture rate).
3. A "winner" indicator when one variant statistically outperforms the other.
4. Timeline chart overlay showing variant performance over time.

Acceptance Criteria:
- [ ] Comparison panel is visible whenever a campaign has 2+ variants
- [ ] Statistical significance is calculated using a minimum sample size threshold (configurable, default: 30 per variant)
- [ ] The comparison view is included in exported campaign reports

## 7. Campaign Build Process

**REQ-CAMP-018** — Build Orchestration

When the Operator triggers Build (Approved -> Building), the system SHALL execute the following steps in order:

1. **Snapshot targets**: Resolve the effective target set (union of all lists/groups, deduplicated). This snapshot is immutable for the campaign's lifetime.
2. **Assign A/B variants**: Partition the snapshotted target set according to split ratios (REQ-CAMP-015).
3. **Compile landing page**: Build the campaign-specific landing page application from the associated landing page definition (see [05-landing-page-builder.md](05-landing-page-builder.md)).
4. **Start landing page app**: Launch the compiled landing page application on the framework server on an assigned port.
5. **Provision phishing endpoint**: Create the cloud VM using the configured provider, region, and instance type.
6. **Deploy proxy**: Deploy and configure the transparent reverse proxy on the phishing endpoint, pointing back to the framework's landing page app.
7. **Configure DNS**: Point the campaign domain's A record to the phishing endpoint's public IP.
8. **Provision TLS**: Obtain and install a TLS certificate for the campaign domain on the phishing endpoint.
9. **Health check**: Verify end-to-end connectivity — HTTPS request to the domain resolves to the endpoint, proxy serves the landing page, TLS is valid.
10. **Transition to Ready**: If all steps succeed, transition to Ready. If any step fails, execute rollback (REQ-CAMP-019).

Acceptance Criteria:
- [ ] Each build step is logged with start time, end time, and status
- [ ] Build progress is reported to the Operator in real-time via WebSocket
- [ ] The build process is idempotent — if retried after a failure-and-rollback, it starts from scratch cleanly
- [ ] The health check verifies TLS validity, HTTP 200 from the landing page, and correct domain resolution

---

**REQ-CAMP-019** — Build Rollback

If any step in the build process fails:

1. All previously completed steps are rolled back in reverse order.
2. Cloud VMs are terminated. DNS records are reverted. Landing page apps are stopped. TLS certificates are cleaned up.
3. The campaign transitions to **Draft** state (not Approved) so the Operator can investigate and fix the issue.
4. A detailed error report is generated and attached to the campaign, visible to both Operator and Engineer.

Acceptance Criteria:
- [ ] Rollback completes fully even if individual rollback steps fail (best-effort with logging)
- [ ] No orphaned cloud resources remain after rollback
- [ ] The error report includes the failed step, error message, and any relevant cloud provider error codes
- [ ] The campaign's approval is voided on build failure (must re-submit)

## 8. Campaign Execution

**REQ-CAMP-020** — Launch Behavior

When the Operator launches a campaign (Ready -> Active):

1. The sending engine begins dispatching emails according to the configured schedule.
2. Emails are personalized per target using the email template's variable substitution.
3. Each email contains a unique tracking pixel URL and unique phishing link (tied to the specific target).
4. The sending engine respects all schedule constraints (send windows, throttle rate, inter-email delays).

Acceptance Criteria:
- [ ] Emails begin sending within 60 seconds of launch (assuming current time is within a send window)
- [ ] Each email's tracking URLs are unique per target and per campaign
- [ ] The sending engine halts sending outside configured send windows and resumes when the next window opens
- [ ] Throttle rate is enforced accurately (within 10% tolerance)

---

**REQ-CAMP-021** — Pause and Resume Behavior

When a campaign is paused (Active -> Paused):

1. The sending engine immediately stops dispatching new emails (in-flight emails may complete).
2. The phishing endpoint remains active and continues to serve the landing page.
3. Tracking continues for any target interactions (clicks, credential submissions).
4. The sending queue position is preserved so that Resume continues from where it stopped.

When a campaign is resumed (Paused -> Active):

1. The sending engine resumes from the saved queue position.
2. Schedule constraints are re-evaluated (if resumed outside a send window, sending waits for the next window).

Acceptance Criteria:
- [ ] Pause takes effect within 5 seconds (no new emails dispatched after pause completes)
- [ ] Endpoint remains accessible during pause
- [ ] Target interactions during pause are tracked normally
- [ ] Resume continues from the exact queue position — no emails are skipped or duplicated

---

**REQ-CAMP-022** — Completion Behavior

A campaign reaches **Completed** state when:
- All emails in the queue have been sent AND the campaign `end_date` has passed (automatic), OR
- The Operator manually triggers completion from Active or Paused state.

Upon completion:

1. The sending engine stops (any remaining unsent emails are marked as `cancelled`).
2. A configurable **grace period** (default: 72 hours) begins, during which the phishing endpoint remains active to capture late interactions.
3. After the grace period expires, infrastructure teardown begins automatically.
4. A completion summary is generated (total sent, delivered, bounced, opens, clicks, credentials).

Acceptance Criteria:
- [ ] Automatic completion triggers when the last email is sent and `end_date` has passed
- [ ] Manual completion is available from both Active and Paused states
- [ ] The grace period is configurable per campaign (default: 72 hours)
- [ ] Unsent emails are marked as `cancelled` with a reason
- [ ] Infrastructure teardown after grace period is automatic and logged

---

**REQ-CAMP-023** — Archive Behavior

When an Operator archives a campaign (Completed -> Archived):

1. All campaign data is marked as immutable (enforced at the application and database level).
2. The campaign is excluded from default list views and dashboards (but can be shown with an "include archived" filter).
3. No further state transitions are permitted.
4. All associated infrastructure must already be terminated (enforced precondition).
5. Campaign data remains in the database indefinitely and is queryable for historical reporting.

Acceptance Criteria:
- [ ] Archived campaigns cannot be modified via any API endpoint (returns 403 with explanation)
- [ ] Default campaign list views exclude archived campaigns
- [ ] An "include archived" filter toggle is available in the UI
- [ ] Archive action is rejected if any infrastructure is still running
- [ ] Archived campaign data is accessible for reporting and export

## 9. Security Considerations

**REQ-CAMP-024** — State Transition Authorization

Every state transition SHALL be authorized based on the actor's role:

| Transition | Required Role |
|-----------|--------------|
| Submit, Build, Launch, Pause, Resume, Complete, Unlock, Archive | Operator (or higher) |
| Approve, Reject | Engineer (or Administrator if block list flag) |
| Approve (with block list targets) | Administrator only |

Acceptance Criteria:
- [ ] Unauthorized state transition attempts return 403 Forbidden
- [ ] Every state transition is recorded in the audit log with actor, timestamp, from-state, to-state, and any comments
- [ ] The API validates both the actor's role and the campaign's current state before permitting any transition

---

**REQ-CAMP-025** — Configuration Immutability Enforcement

Campaign configuration MUST be immutable in all states except **Draft**:

1. The API SHALL reject any configuration modification request when the campaign is not in Draft state, returning HTTP 409 Conflict with an explanation.
2. This is enforced at the API layer, not just the UI.
3. The only way to modify a non-Draft campaign is to Unlock it back to Draft (which voids approval and tears down infrastructure).

Acceptance Criteria:
- [ ] PUT/PATCH requests to campaign configuration endpoints return 409 when campaign is not in Draft
- [ ] The error response includes the current state and instructions to Unlock if modification is needed
- [ ] UI disables all configuration editing controls for non-Draft campaigns

---

**REQ-CAMP-026** — Audit Trail

All campaign lifecycle events SHALL be recorded in the audit log:

| Event | Logged Fields |
|-------|--------------|
| State transition | Actor, timestamp, from_state, to_state, comments |
| Configuration change | Actor, timestamp, field changed, old value, new value |
| Approval/rejection | Reviewer, timestamp, decision, comments, block list acknowledgment (if applicable) |
| Build step progress | Step name, start time, end time, status, error details (if failed) |
| Email sent | Target ID, template variant, SMTP config used, timestamp, message ID |
| Target interaction | Target ID, interaction type (open/click/submit), timestamp, source IP |

Acceptance Criteria:
- [ ] All events listed above are recorded with the specified fields
- [ ] Audit log entries are append-only and cannot be modified or deleted via the application
- [ ] Audit log is queryable by campaign ID, actor, event type, and date range

---

**REQ-CAMP-027** — Concurrent Campaign Isolation

Multiple campaigns MAY run concurrently. The system SHALL ensure:

1. Each campaign operates on independent infrastructure (separate VM, separate landing page app instance).
2. Tracking data is isolated per campaign — no cross-campaign data leakage.
3. A domain cannot be used by more than one campaign simultaneously (enforced at state transition time).
4. Resource limits are configurable to prevent a single campaign from exhausting system resources.

Acceptance Criteria:
- [ ] Two campaigns cannot share a phishing endpoint or domain simultaneously
- [ ] Campaign A's tracking events never appear in Campaign B's metrics
- [ ] The system enforces a configurable maximum number of concurrent active campaigns

## 10. API Endpoints

**REQ-CAMP-028** — Campaign REST API

The following REST API endpoints SHALL be implemented:

| Method | Endpoint | Description | Auth |
|--------|----------|-------------|------|
| POST | `/api/v1/campaigns` | Create a new campaign (Draft) | Operator+ |
| GET | `/api/v1/campaigns` | List campaigns (filterable by state, date, name; paginated) | Operator+ |
| GET | `/api/v1/campaigns/:id` | Get campaign detail (config, state, metrics summary) | Operator+ |
| PUT | `/api/v1/campaigns/:id` | Update campaign configuration (Draft only) | Operator+ |
| DELETE | `/api/v1/campaigns/:id` | Delete a Draft campaign permanently | Operator+ |
| POST | `/api/v1/campaigns/:id/submit` | Submit for approval (Draft -> Pending Approval) | Operator+ |
| POST | `/api/v1/campaigns/:id/approve` | Approve (Pending Approval -> Approved) | Engineer+ / Admin if block list |
| POST | `/api/v1/campaigns/:id/reject` | Reject with comments (Pending Approval -> Draft) | Engineer+ / Admin if block list |
| POST | `/api/v1/campaigns/:id/build` | Trigger build (Approved -> Building) | Operator+ |
| POST | `/api/v1/campaigns/:id/launch` | Launch (Ready -> Active) | Operator+ |
| POST | `/api/v1/campaigns/:id/pause` | Pause (Active -> Paused) | Operator+ |
| POST | `/api/v1/campaigns/:id/resume` | Resume (Paused -> Active) | Operator+ |
| POST | `/api/v1/campaigns/:id/complete` | Complete (Active/Paused -> Completed) | Operator+ |
| POST | `/api/v1/campaigns/:id/archive` | Archive (Completed -> Archived) | Operator+ |
| POST | `/api/v1/campaigns/:id/unlock` | Unlock (Approved/Ready -> Draft) | Operator+ |
| GET | `/api/v1/campaigns/:id/metrics` | Get campaign metrics (overall + per-variant) | Operator+ |
| GET | `/api/v1/campaigns/:id/build-log` | Get build process log | Operator+ |
| GET | `/api/v1/campaigns/:id/approval-history` | Get approval/rejection/unlock history | Operator+ |
| POST | `/api/v1/campaigns/:id/clone` | Clone a campaign (selective) | Operator+ |
| POST | `/api/v1/campaigns/:id/dry-run` | Execute a dry run simulation | Operator+ |
| GET | `/api/v1/campaigns/:id/dry-run` | Get dry run simulation results | Operator+ |
| GET | `/api/v1/campaigns/calendar` | Get campaigns formatted for calendar view | Operator+ |
| GET | `/api/v1/campaign-templates` | List campaign templates | Operator+ |
| POST | `/api/v1/campaign-templates` | Create a campaign template | Operator+ |
| GET | `/api/v1/campaign-templates/:id` | Get campaign template detail | Operator+ |
| PUT | `/api/v1/campaign-templates/:id` | Update a campaign template | Operator+ |
| DELETE | `/api/v1/campaign-templates/:id` | Delete a campaign template | Operator+ |
| POST | `/api/v1/campaign-templates/:id/apply` | Apply a template to a new campaign | Operator+ |

Acceptance Criteria:
- [ ] All endpoints enforce role-based access control
- [ ] State transition endpoints validate the current state before executing
- [ ] Invalid state transitions return 409 Conflict with the current state and valid transitions
- [ ] List endpoint supports filtering by state (single or multiple), date range, name (substring search), and pagination
- [ ] DELETE is only permitted for Draft campaigns; returns 409 otherwise

---

**REQ-CAMP-029** — WebSocket Real-Time Updates

The campaign management module SHALL emit real-time updates via WebSocket for:

1. Build progress (step-by-step updates during Building state).
2. Sending progress during Active state (emails sent count, delivery confirmations).
3. Live target interactions (opens, clicks, credential captures) during Active and Paused states.
4. State transition notifications.

Acceptance Criteria:
- [ ] WebSocket channel is scoped per campaign (`/ws/campaigns/:id`)
- [ ] Updates are delivered within 2 seconds of the underlying event
- [ ] WebSocket connections require authentication and role authorization
- [ ] Connection loss is handled gracefully with automatic reconnection in the frontend

## 11. Data Model Summary

**REQ-CAMP-030** — Database Entities

The following entities (at minimum) SHALL exist to support campaign management. Full schema is defined in [14-database-schema.md](14-database-schema.md).

| Entity | Purpose |
|--------|---------|
| `campaigns` | Core campaign record (config, state, dates) |
| `campaign_target_lists` | Join table: campaign <-> target lists |
| `campaign_target_groups` | Join table: campaign <-> target groups |
| `campaign_smtp_configs` | Join table: campaign <-> SMTP configs |
| `campaign_template_variants` | Template + split ratio per campaign |
| `campaign_send_windows` | Sending schedule windows |
| `campaign_targets_snapshot` | Snapshotted target set (created at Build time) |
| `campaign_target_variant_assignments` | Target-to-variant assignment for A/B testing |
| `campaign_approvals` | Approval/rejection/unlock history |
| `campaign_build_logs` | Build step execution log |
| `campaign_emails` | Per-email sending record (target, variant, status, timestamps) |
| `campaign_state_transitions` | Full state transition audit trail |
| `campaign_templates` | Reusable campaign configuration templates |
| `campaign_canary_targets` | Canary target designations per campaign |
| `campaign_dry_runs` | Dry run simulation results |

Acceptance Criteria:
- [ ] All entities use UUID primary keys
- [ ] Foreign key constraints are enforced at the database level
- [ ] Appropriate indexes exist for common query patterns (campaign list by state, email by target, metrics aggregation)
- [ ] Soft-delete is NOT used — Draft campaigns can be hard-deleted; all other states are retained

## 12. UI Requirements

**REQ-CAMP-031** — Campaign List View

The campaign list view SHALL display:

1. Campaign name, current state (with color-coded badge), date range, target count, Operator name.
2. Filterable by state, date range, and Operator.
3. Sortable by name, state, start date, creation date.
4. Quick-action buttons appropriate to each campaign's current state (e.g., "Launch" for Ready, "Pause" for Active).
5. Archived campaigns are hidden by default with an "Include Archived" toggle.

---

**REQ-CAMP-032** — Campaign Detail / Configuration View

The campaign detail view SHALL provide:

1. A state indicator bar showing the full lifecycle with the current state highlighted.
2. Tabbed sections for: Configuration, Targets, Templates, Schedule, Infrastructure, Approval History, Build Log, Metrics.
3. In Draft state: all fields are editable inline.
4. In non-Draft states: all fields are displayed as read-only with a lock icon.
5. An Unlock button (visible in Approved and Ready states) with a confirmation dialog that explains the consequences (approval voided, infrastructure torn down, must re-approve and re-build).

---

**REQ-CAMP-033** — Approval Review View

See REQ-CAMP-010 for detailed requirements. This is a dedicated view (not the standard campaign detail view) optimized for the reviewer workflow with approve/reject action buttons.

## 13. Error Handling

**REQ-CAMP-034** — Graceful Error Handling

The campaign management module SHALL handle the following error conditions gracefully:

| Error Condition | Behavior |
|----------------|----------|
| Cloud provider API failure during build | Rollback all provisioned resources; transition to Draft; notify Operator with error details |
| SMTP failure during sending | Retry with next SMTP config (if available); log failure; continue sending; alert Operator if all SMTP configs fail |
| Landing page app crash during Active campaign | Restart landing page app automatically; log the crash; alert Operator if restart fails |
| Phishing endpoint becomes unreachable during Active campaign | Alert Operator immediately; continue email sending (targets who click will see an error until endpoint recovers) |
| Database connection loss | Queue state transitions in memory; replay when connection restores; never lose a state transition |
| Concurrent state transition attempts | Use database-level locking (SELECT FOR UPDATE); reject the slower request with 409 |

Acceptance Criteria:
- [ ] Each error condition listed above is handled as specified
- [ ] All errors are logged with full context (campaign ID, error type, stack trace)
- [ ] Operator-facing error messages are actionable (not raw stack traces)
- [ ] Concurrent state transition conflicts are handled without data corruption

## 14. Campaign Cloning

**REQ-CAMP-035** — Selective Campaign Cloning

The system SHALL support cloning an existing campaign to create a new campaign pre-populated with the source campaign's configuration.

| Feature | Behavior |
|---------|----------|
| **Selective clone** | The Operator selects which configuration elements to carry over from the source campaign. Each element is independently selectable. |
| **Cloneable elements** | Landing page association, target lists/groups, SMTP configurations, email template variants (with split ratios), sending schedule, phishing endpoint configuration (provider, region, instance type), and tags. |
| **Non-cloneable data** | Campaign name (must be set manually), dates (must be set manually), approval history, build logs, metrics, captured credentials, and state (always starts as Draft). |
| **Source campaign states** | Any non-Draft campaign can be used as a clone source. Draft campaigns can also be cloned. |

Acceptance Criteria:
- [ ] The clone action is available from the campaign list view and the campaign detail view
- [ ] A clone configuration dialog allows the Operator to select/deselect each cloneable element
- [ ] The cloned campaign is created in Draft state with a default name of "Copy of {source name}"
- [ ] Cloning does not modify the source campaign in any way
- [ ] The clone action is recorded in the audit log with the source campaign ID

---

## 15. Campaign Templates

**REQ-CAMP-036** — Reusable Campaign Templates

The system SHALL support saving campaign configurations as reusable templates that can be applied when creating new campaigns.

| Feature | Behavior |
|---------|----------|
| **Save as template** | An Operator can save a campaign's current configuration as a named template. |
| **Template contents** | Templates store: landing page reference, SMTP configurations, email template variants (with split ratios), sending schedule configuration, phishing endpoint configuration (provider, region, instance type), and tags. |
| **Template management** | Templates can be created, renamed, duplicated, and deleted. Templates are visible to all users. |
| **Apply template** | When creating a new campaign, the Operator can select a template to pre-populate configuration fields. All pre-populated fields remain editable. |

Acceptance Criteria:
- [ ] Templates are stored as independent entities with a name, description, and configuration JSON
- [ ] Applying a template pre-populates all stored configuration fields in the new campaign
- [ ] The Operator can modify any pre-populated field after applying the template
- [ ] Templates can reference landing pages, SMTP configs, and email templates by ID; the system validates that referenced entities still exist when a template is applied
- [ ] If a referenced entity has been deleted, the template application warns the Operator and leaves that field empty

---

## 16. Scheduled Auto-Launch

**REQ-CAMP-037** — Scheduled Campaign Launch

The system SHALL support configuring a campaign to launch automatically at a scheduled date/time, in addition to manual launch.

| Feature | Behavior |
|---------|----------|
| **Schedule setting** | During campaign configuration (Draft state), the Operator can set an optional `scheduled_launch_at` datetime. |
| **Auto-launch trigger** | When the campaign is in Ready state and the scheduled launch time arrives, the system automatically transitions the campaign to Active state and begins sending emails. |
| **Manual override** | The Operator can manually launch the campaign before the scheduled time, or cancel the scheduled launch. |
| **Validation** | The `scheduled_launch_at` must be within the campaign's `start_date` and `end_date` range. |

Acceptance Criteria:
- [ ] A background worker checks for campaigns in Ready state with a `scheduled_launch_at` that has passed
- [ ] Auto-launch transitions are recorded in the audit log with actor = "system"
- [ ] The campaign detail view displays the scheduled launch time prominently when set
- [ ] The Operator can clear the scheduled launch time to revert to manual-only launch
- [ ] If the campaign is not in Ready state when the scheduled time arrives, the scheduled launch is skipped and a notification is sent to the Operator

---

## 17. Custom Send Order

**REQ-CAMP-038** — Custom Target Send Priority

The system SHALL support configuring a custom send order for targets within a campaign, in addition to the default order.

| Send Order Option | Behavior |
|-------------------|----------|
| **Default** | Targets are sent in the order they appear in the resolved target list (no specific ordering). |
| **Alphabetical** | Sort by target email address (ascending or descending). |
| **Department** | Group by department, with configurable department ordering. |
| **Custom sort** | Operator manually reorders targets or defines a priority ranking per target. |
| **Randomized** | Targets are shuffled using a seeded random order (reproducible given the campaign seed). |

Acceptance Criteria:
- [ ] The campaign configuration UI provides a send order selector with all options listed above
- [ ] Custom sort allows drag-and-drop reordering or numeric priority assignment
- [ ] The selected send order is applied at launch time when building the email queue
- [ ] Send order is visible in the approval review
- [ ] Send order configuration is preserved across Unlock/re-submit cycles

---

## 18. Parallel Approval Workflow

**REQ-CAMP-039** — Parallel Approver Configuration

The system SHALL support requiring approval from multiple reviewers before a campaign can proceed.

| Feature | Behavior |
|---------|----------|
| **Approver count** | Campaigns can be configured to require approval from N approvers (configurable, default: 1). |
| **Parallel approval** | All required approvers must approve independently. Any single rejection returns the campaign to Draft. |
| **Approver selection** | The system notifies all eligible approvers (Engineers, or Administrators if block list flag is set). Any eligible user can act as an approver. |
| **Approval tracking** | Each approval is recorded independently with the approver's identity, timestamp, and optional comments. |

Acceptance Criteria:
- [ ] The system setting for required approver count is configurable by Administrators (minimum 1, maximum 5)
- [ ] The campaign remains in Pending Approval until all required approvals are received
- [ ] Each approver can only approve or reject once per submission cycle
- [ ] If any approver rejects, the campaign returns to Draft immediately and all other pending approvals are voided
- [ ] The approval review interface shows the current approval count (e.g., "2 of 3 approvals received")
- [ ] A single rejection voids all existing approvals and the campaign returns to Draft

---

## 19. Auto-Summary on Completion

**REQ-CAMP-040** — Campaign Completion Summary

The system SHALL automatically generate a lightweight summary report when a campaign reaches Completed state, and support on-demand generation of a comprehensive report.

| Summary Type | Trigger | Contents |
|-------------|---------|----------|
| **Lightweight (auto)** | Automatic on campaign completion | Total emails sent/delivered/bounced, open rate, click-through rate, credential capture rate, top 5 targets by engagement, campaign duration, A/B variant winner (if applicable). |
| **Comprehensive (on-demand)** | Operator clicks "Generate Full Report" | Full PDF report with all metrics, charts, per-target breakdown, timeline, geographic distribution, template comparison, and recommendations. |

Acceptance Criteria:
- [ ] The lightweight summary is generated automatically within 60 seconds of campaign completion
- [ ] The lightweight summary is displayed prominently on the campaign detail view when in Completed state
- [ ] The comprehensive report is generated on-demand and follows the standard report generation pipeline (see [10-metrics-reporting.md](10-metrics-reporting.md))
- [ ] Both summary types are included in the campaign's exportable data
- [ ] The lightweight summary includes a "Generate Full Report" button for the comprehensive version

---

## 20. Dry Run / Sandbox Mode

**REQ-CAMP-041** — Campaign Dry Run Simulation

The system SHALL support a dry run mode that simulates campaign execution without sending emails to real targets or provisioning real infrastructure.

| Feature | Behavior |
|---------|----------|
| **Dry run trigger** | Available from Ready state as an alternative to Launch. |
| **Simulated sending** | The system simulates email delivery for all targets, generating synthetic delivery events (sent, delivered, opened, clicked) based on configurable simulation parameters. |
| **No real emails** | No actual SMTP connections are made. No real emails are sent. |
| **No real infrastructure** | If infrastructure is already provisioned (Ready state), the dry run does not use it for email delivery. The landing page can still be tested manually. |
| **Simulation results** | Dry run produces a simulation report showing the projected timeline, send order, template distribution, and estimated completion time. |
| **State** | Dry run does not change the campaign state. The campaign remains in Ready state after the dry run completes. |

Acceptance Criteria:
- [ ] The dry run action is available alongside the Launch button in Ready state
- [ ] Dry run events are clearly marked as simulated and do NOT appear in real campaign metrics
- [ ] The simulation report is stored and viewable from the campaign detail view
- [ ] Dry run completes within 30 seconds for campaigns with up to 10,000 targets
- [ ] Simulation parameters (e.g., simulated open rate, click rate) are configurable per dry run

---

## 21. Canary Targets

**REQ-CAMP-042** — Canary Target Designation

The system SHALL support designating specific targets as "canary targets" — internal team members who receive the phishing email alongside real targets to verify campaign functionality in production.

| Feature | Behavior |
|---------|----------|
| **Canary designation** | Any target can be flagged as a canary target at the campaign level. |
| **Auto-verification** | When a canary target's email is delivered and the canary interacts (opens, clicks, submits), the system logs a verification event confirming the campaign is functioning correctly. |
| **Priority sending** | Canary targets are sent emails first, before any real targets. |
| **Separate metrics** | Canary target interactions are tracked separately and excluded from campaign success metrics (open rate, click rate, capture rate). |
| **Dashboard indicator** | The campaign dashboard shows a canary status panel indicating which canary targets have verified and which have not. |

Acceptance Criteria:
- [ ] Canary targets can be designated during campaign configuration (Draft state)
- [ ] Canary targets receive emails before any non-canary targets when the campaign launches
- [ ] Canary interactions are recorded but excluded from aggregate campaign metrics
- [ ] The campaign dashboard includes a "Canary Status" section showing verification status per canary target
- [ ] Canary target data is available in reports but clearly labeled as canary data
- [ ] A campaign can have zero or more canary targets (canary designation is optional)

---

## 22. Campaign Calendar View

**REQ-CAMP-043** — Calendar View

The system SHALL provide a calendar-based view of campaigns in addition to the list view.

| Feature | Behavior |
|---------|----------|
| **Calendar display** | Campaigns are displayed as date-range bars on a calendar (month, week, and day views). |
| **Color coding** | Campaign bars are color-coded by current state (Draft = gray, Active = green, Paused = yellow, Completed = blue, etc.). |
| **Interaction** | Clicking a campaign bar navigates to the campaign detail view. |
| **Overlap detection** | The calendar visually highlights date ranges where multiple campaigns overlap, helping operators plan scheduling. |
| **Create from calendar** | The Operator can click on a date to create a new campaign with that start date pre-populated. |

Acceptance Criteria:
- [ ] The campaign calendar is accessible from the main campaigns navigation
- [ ] Month, week, and day views are available with navigation controls
- [ ] Campaigns spanning multiple days/weeks are rendered as continuous bars
- [ ] The calendar respects campaign list filters (state, Operator, archived toggle)
- [ ] Scheduled launch times are indicated with a distinct marker on the calendar

---

## 23. Dependencies

This module depends on:

| Module | Dependency |
|--------|-----------|
| [02-authentication-authorization.md](02-authentication-authorization.md) | Role-based access control for all operations |
| [03-domain-management.md](03-domain-management.md) | Domain availability and DNS management |
| [04-smtp-configuration.md](04-smtp-configuration.md) | SMTP server configs for email delivery |
| [05-landing-page-builder.md](05-landing-page-builder.md) | Landing page definitions for compilation and deployment |
| [07-phishing-endpoints.md](07-phishing-endpoints.md) | Endpoint provisioning and proxy deployment |
| [09-target-management.md](09-target-management.md) | Target lists, groups, and global block list |
| [11-audit-logging.md](11-audit-logging.md) | Audit trail for all campaign events |
| [14-database-schema.md](14-database-schema.md) | Full schema definitions for campaign entities |
