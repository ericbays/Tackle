# 09 — Target Management

## 1. Overview

Target Management governs how phishing targets (individual people identified by email address) are created, organized, imported, tracked, and protected within Tackle. Targets are first-class entities that persist across campaigns and carry a complete engagement history. A system-wide block list provides a critical safety mechanism to prevent unauthorized phishing of protected individuals or domains.

This module interacts with:
- **Campaign Management (06)** — targets and target groups are assigned to campaigns
- **Credential Capture (08)** — captured credentials are linked back to the originating target
- **Metrics & Reporting (10)** — per-target and aggregate engagement data feeds dashboards and reports
- **Authentication & Authorization (02)** — RBAC governs who can manage targets, groups, and the block list

---

## 2. Target Data Model

### REQ-TGT-001: Core Target Entity

The system SHALL store an individual target record with the following fields:

| Field | Type | Required | Constraints |
|-------|------|----------|-------------|
| `id` | UUID | Yes | System-generated, immutable |
| `email` | String | Yes | Valid RFC 5321 email address, unique across all targets, case-insensitive comparison |
| `first_name` | String | No | Max 255 characters |
| `last_name` | String | No | Max 255 characters |
| `department` | String | No | Max 255 characters |
| `title` | String | No | Max 255 characters (job title) |
| `custom_fields` | JSONB | No | Arbitrary key-value pairs, max 50 keys, each key max 64 characters, each value max 1024 characters |
| `created_at` | Timestamp | Yes | System-generated, immutable |
| `updated_at` | Timestamp | Yes | System-managed |
| `created_by` | UUID (FK) | Yes | Reference to the user who created the target |

**Acceptance Criteria:**
- [ ] Email addresses are validated against RFC 5321 on creation and update
- [ ] Email uniqueness is enforced at the database level (case-insensitive unique index)
- [ ] Custom fields reject payloads exceeding the key/value limits
- [ ] `created_at` and `id` cannot be modified after creation

**Security Considerations:**
- Target email addresses and personal information are considered sensitive data. Access is restricted by RBAC.
- Custom fields MUST NOT be used to store credentials, passwords, or API keys. The UI should display a warning if custom field values resemble secret material.

---

### REQ-TGT-002: Target Persistence Across Campaigns

Targets SHALL persist as independent entities that exist outside the lifecycle of any single campaign. A target record is NOT deleted when a campaign referencing it is deleted or archived.

**Acceptance Criteria:**
- [ ] Deleting a campaign does not delete associated target records
- [ ] A target can exist with zero campaign associations
- [ ] Archiving a campaign preserves the target's historical engagement data for that campaign

---

### REQ-TGT-003: Target-Campaign Association

Targets SHALL be associable with one or more campaigns through a many-to-many relationship. The association record SHALL include:

| Field | Type | Description |
|-------|------|-------------|
| `target_id` | UUID (FK) | Reference to the target |
| `campaign_id` | UUID (FK) | Reference to the campaign |
| `status` | Enum | Current status within this campaign (see REQ-TGT-024) |
| `assigned_at` | Timestamp | When the target was added to the campaign |
| `assigned_by` | UUID (FK) | User who assigned the target |

**Acceptance Criteria:**
- [ ] The same target can be assigned to multiple concurrent campaigns
- [ ] Duplicate target-campaign associations are rejected at the database level
- [ ] Removing a target from a campaign that has not yet launched deletes the association
- [ ] Removing a target from a launched campaign marks the association as `removed` but preserves historical data

---

## 3. Import & Creation

### REQ-TGT-004: CSV Import — File Upload

The system SHALL allow users with the Operator or Engineer role to upload CSV files containing target data. The upload endpoint SHALL:

- Accept files up to 50 MB in size
- Accept `.csv` and `.txt` file extensions
- Reject files that are not valid CSV (malformed quoting, inconsistent column counts)
- Return a parse preview before committing the import

**Acceptance Criteria:**
- [ ] Files exceeding 50 MB are rejected with a clear error message
- [ ] Non-CSV files are rejected during validation, not after processing
- [ ] The upload is processed server-side; the browser does not parse the CSV

**Security Considerations:**
- CSV parsing MUST use a hardened parser that handles CSV injection vectors (cells beginning with `=`, `+`, `-`, `@`, `\t`, `\r`). These characters SHALL be escaped or stripped on import.
- File uploads MUST be scanned for size and type before processing. The server MUST NOT trust the `Content-Type` header alone.

---

### REQ-TGT-005: CSV Import — Column Mapping

After upload, the system SHALL present a column mapping interface allowing the user to:

- View the first 10 rows of the uploaded CSV as a preview
- Map each CSV column to a target field (`email`, `first_name`, `last_name`, `department`, `title`, or a custom field)
- Mark columns as "ignored" (not imported)
- Designate which column contains the email address (required mapping)
- Save column mapping configurations as named templates for reuse

**Acceptance Criteria:**
- [ ] The mapping UI displays actual data from the CSV for each column
- [ ] Import cannot proceed without an email column mapping
- [ ] Saved mapping templates persist across sessions and are available to all users
- [ ] Mapping templates can be edited and deleted by the user who created them or by Administrators

---

### REQ-TGT-006: CSV Import — Validation

Before committing an import, the system SHALL validate all rows and present a validation report:

| Check | Behavior |
|-------|----------|
| **Email format** | Rows with invalid email addresses are flagged as errors |
| **Duplicate (within file)** | Rows with duplicate email addresses within the same CSV are flagged; user chooses which to keep |
| **Duplicate (existing targets)** | Rows matching existing targets are flagged; user chooses to skip, update existing, or create duplicate warning |
| **Block list match** | Rows matching block list entries (see Section 5) are flagged with a warning |
| **Empty required field** | Rows with an empty email field are flagged as errors |

The validation report SHALL display:
- Total rows parsed
- Count of valid rows ready for import
- Count of rows with errors (with details)
- Count of rows with warnings (with details)
- Count of rows matching the block list

The user SHALL be able to:
- Proceed with only the valid rows
- Download a CSV of rejected rows for correction
- Cancel the entire import

**Acceptance Criteria:**
- [ ] Validation processes all rows before presenting results (no partial validation)
- [ ] Block list matches are visually distinct from other warnings
- [ ] The rejected-rows CSV includes a column indicating the rejection reason
- [ ] Import of valid rows is atomic — either all valid rows are imported or none are

---

### REQ-TGT-007: Manual Target Entry

The system SHALL provide a form-based UI for creating individual targets. The form SHALL:

- Include all fields from the target data model (REQ-TGT-001)
- Validate email format in real-time (client-side) with server-side confirmation on submit
- Check for duplicate email addresses on blur and display a warning if a match exists
- Check the block list on blur and display a warning if the email or domain matches
- Allow adding custom field key-value pairs dynamically

**Acceptance Criteria:**
- [ ] Real-time validation provides feedback within 500ms of the user finishing input
- [ ] Duplicate and block list warnings do not prevent creation but are clearly visible
- [ ] Successfully created targets appear in the target list immediately
- [ ] The form resets after successful creation, with an option to keep values for rapid entry

---

### REQ-TGT-008: Bulk Operations — Selection

The system SHALL support selecting multiple targets in the target list for bulk operations. Selection mechanisms:

- Checkbox selection of individual rows
- "Select all on this page" checkbox
- "Select all matching current filter/search" option (may exceed current page)
- Selection count displayed prominently in the UI

**Acceptance Criteria:**
- [ ] Selection persists across page navigation within the target list
- [ ] "Select all matching filter" works correctly with any active search or filter criteria
- [ ] The UI clearly displays the total count of selected targets

---

### REQ-TGT-009: Bulk Operations — Actions

The following bulk actions SHALL be available on selected targets:

| Action | Roles | Behavior |
|--------|-------|----------|
| **Add to group** | Operator, Engineer, Admin | Add selected targets to one or more existing groups, or create a new group |
| **Remove from group** | Operator, Engineer, Admin | Remove selected targets from a specified group |
| **Edit field** | Operator, Engineer, Admin | Set a single field (department, title, or a custom field) to the same value for all selected targets |
| **Delete** | Engineer, Admin | Soft-delete selected targets (see REQ-TGT-010) |
| **Export** | Operator, Engineer, Admin | Download a CSV of selected targets |

**Acceptance Criteria:**
- [ ] Bulk operations on more than 1,000 targets execute asynchronously with a progress indicator
- [ ] Bulk delete requires a confirmation dialog displaying the count of targets to be deleted
- [ ] Bulk operations produce an audit log entry listing all affected target IDs
- [ ] If any individual operation within a bulk action fails, the system completes the remaining operations and reports failures separately

---

### REQ-TGT-010: Target Deletion

Target deletion SHALL be a soft delete. Soft-deleted targets:

- Are excluded from all target lists and search results by default
- Cannot be assigned to new campaigns
- Retain all historical campaign association and engagement data
- Can be restored by an Administrator within 90 days
- Are permanently purged after 90 days by an automated cleanup process

**Acceptance Criteria:**
- [ ] Soft-deleted targets do not appear in any target selection UI
- [ ] Historical reports referencing soft-deleted targets still display correct data
- [ ] The restore function is available only to Administrators
- [ ] The 90-day retention period is configurable by an Administrator (minimum 30 days, maximum 365 days)

**Security Considerations:**
- Permanent deletion must be irreversible and must remove all personally identifiable information (PII) from the database, including custom fields.

---

## 4. Target Groups

### REQ-TGT-011: Group Entity

The system SHALL support named target groups with the following fields:

| Field | Type | Required | Constraints |
|-------|------|----------|-------------|
| `id` | UUID | Yes | System-generated, immutable |
| `name` | String | Yes | Max 255 characters, unique across groups |
| `description` | String | No | Max 1024 characters |
| `created_at` | Timestamp | Yes | System-generated |
| `updated_at` | Timestamp | Yes | System-managed |
| `created_by` | UUID (FK) | Yes | Reference to creating user |

**Acceptance Criteria:**
- [ ] Group names are unique (case-insensitive)
- [ ] Groups can be created, renamed, and deleted
- [ ] Deleting a group does not delete its member targets

---

### REQ-TGT-012: Group Membership

Targets SHALL be associable with groups through a many-to-many relationship. A target MAY belong to zero, one, or many groups. A group MAY contain zero or more targets.

**Acceptance Criteria:**
- [ ] Adding a target to a group it already belongs to is idempotent (no error, no duplicate)
- [ ] Removing a target from a group it does not belong to is idempotent (no error)
- [ ] The group membership list displays the current member count
- [ ] Groups with zero members are allowed and visible in the group list

---

### REQ-TGT-013: Group Assignment to Campaigns

When assigning targets to a campaign, the system SHALL allow selection of entire groups in addition to (or instead of) individual targets. The resolved target list for a campaign is the union of:

- All individually assigned targets
- All members of all assigned groups (at the time of resolution)

Duplicate targets (appearing both individually and via group, or in multiple groups) SHALL be deduplicated — each target appears at most once in a campaign's target list.

**Acceptance Criteria:**
- [ ] Assigning a group to a campaign adds all current members of that group
- [ ] The campaign target list shows the source of each target (direct assignment, group name, or both)
- [ ] Deduplication is applied automatically and reported to the user (e.g., "15 targets added, 3 duplicates removed")

---

### REQ-TGT-014: Dynamic Group Membership

Group membership is dynamic with respect to future campaign assignments. The following rules apply:

| Scenario | Behavior |
|----------|----------|
| Target added to group AFTER group assigned to a draft campaign | Target IS included when the campaign launches (resolution happens at launch) |
| Target removed from group AFTER group assigned to a draft campaign | Target is NOT included when the campaign launches |
| Target added to group AFTER campaign has launched | Target is NOT added to the already-launched campaign |
| Target removed from group AFTER campaign has launched | Target is NOT removed from the already-launched campaign |

**Acceptance Criteria:**
- [ ] Target list resolution for a campaign occurs at launch time, not at assignment time
- [ ] The pre-launch campaign view shows a "Resolve Now" preview that computes the current target list
- [ ] Post-launch campaign target lists are immutable snapshots
- [ ] The UI clearly communicates that group membership changes affect only future/draft campaigns

---

## 5. Global Block List

### REQ-TGT-015: Block List Entity (CRITICAL)

The system SHALL maintain a global block list of email addresses and domain patterns that represent targets who MUST NOT be phished without explicit Administrator override. Block list entries SHALL have the following fields:

| Field | Type | Required | Constraints |
|-------|------|----------|-------------|
| `id` | UUID | Yes | System-generated, immutable |
| `pattern` | String | Yes | Email address or domain wildcard (see REQ-TGT-016) |
| `reason` | String | Yes | Human-readable explanation for why this entry exists, max 2048 characters |
| `added_by` | UUID (FK) | Yes | Reference to the Administrator who added the entry |
| `added_at` | Timestamp | Yes | System-generated |
| `is_active` | Boolean | Yes | Defaults to `true`. Inactive entries are retained for audit but not enforced. |

**Acceptance Criteria:**
- [ ] Block list entries are unique by pattern (case-insensitive)
- [ ] Every block list entry requires a non-empty reason
- [ ] Block list entries cannot be permanently deleted, only deactivated
- [ ] All block list changes (add, deactivate, reactivate) are recorded in the audit log with the acting user and timestamp

**Security Considerations:**
- The block list is a critical safety control. Its integrity must be protected at all times. Only Administrators may modify the block list. This restriction MUST be enforced server-side, not merely in the UI.

---

### REQ-TGT-016: Block List Pattern Matching

Block list entries SHALL support two pattern types:

| Pattern Type | Format | Example | Matches |
|-------------|--------|---------|---------|
| **Individual email** | `user@domain.com` | `ceo@company.com` | Exact email match (case-insensitive) |
| **Domain wildcard** | `*@domain.com` | `*@legal.company.com` | All email addresses at the specified domain |

Pattern matching rules:
- Matching is case-insensitive
- Domain wildcards match only the exact domain specified — `*@company.com` does NOT match `user@sub.company.com`
- Subdomain wildcards are supported: `*@*.company.com` matches `user@legal.company.com` and `user@hr.company.com` but NOT `user@company.com`
- No other wildcard patterns are supported (no partial-name matching)

**Acceptance Criteria:**
- [ ] `*@legal.company.com` matches `anyone@legal.company.com` but not `anyone@company.com`
- [ ] `*@*.company.com` matches `user@sub.company.com` but not `user@company.com`
- [ ] `ceo@company.com` matches only that exact address (case-insensitive)
- [ ] Invalid patterns (e.g., `*@*`, `*`, `@@`, empty string) are rejected on creation
- [ ] The pattern `*@*` is explicitly rejected to prevent accidentally blocking all targets

---

### REQ-TGT-017: Block List Enforcement on Campaign Assignment

When targets are assigned to a campaign (individually or via group), the system SHALL check each target against the active block list. If any target matches a block list entry:

- The matching targets SHALL be flagged in the campaign's target list with a visual indicator (e.g., red warning icon)
- Each flagged target SHALL display the matching block list pattern and the reason
- The campaign status SHALL be set to `requires_block_list_approval` (in addition to any other required approvals)
- A summary banner SHALL be displayed: "X target(s) match the global block list. Administrator approval is required to proceed."

**Acceptance Criteria:**
- [ ] Block list checking occurs at assignment time AND at campaign launch time (targets may be added to the block list between assignment and launch)
- [ ] The block list check is performed server-side and cannot be bypassed by the client
- [ ] Flagged targets are visually distinct from non-flagged targets in the campaign target list
- [ ] The block list reason is displayed alongside each flagged target

---

### REQ-TGT-018: Block List Override Approval (CRITICAL)

If a campaign includes targets that match the block list, the campaign CANNOT launch without explicit Administrator approval. The approval workflow SHALL operate as follows:

1. **Approval request**: An Operator or Engineer initiates the campaign launch. The system detects block list matches and generates an approval request.
2. **Approval UI**: The Administrator is presented with an approval screen that shows:
   - The campaign name and ID
   - A list of ALL blocked targets with their matching block list pattern and the associated reason
   - A mandatory acknowledgment checkbox: "I understand that launching this campaign will send phishing emails to targets on the global block list. I accept responsibility for this override."
   - A required text field for the Administrator to provide a justification for the override
3. **Approval action**: The Administrator must check the acknowledgment AND provide a justification to approve.
4. **Rejection action**: The Administrator may reject the override, optionally providing a reason. The campaign returns to draft status.

**Constraints:**
- Only users with the **Administrator** role may approve block list overrides. The **Engineer** role is NOT sufficient.
- The approval is specific to the exact set of blocked targets at the time of approval. If additional blocked targets are added to the campaign after approval, a new approval is required.
- The approval, including the Administrator's identity, timestamp, justification text, and the list of overridden targets, SHALL be recorded in the audit log as a distinct, queryable event.

**Acceptance Criteria:**
- [ ] Engineers cannot approve block list overrides even if they can approve other campaign aspects
- [ ] The approval screen lists every blocked target individually — not just a count
- [ ] The acknowledgment checkbox and justification text are both required for approval
- [ ] Approval is invalidated if the campaign's blocked target set changes after approval
- [ ] The audit log entry for a block list override includes the full list of overridden target emails, the matching patterns, and the Administrator's justification
- [ ] Rejected overrides are also logged with the Administrator's rejection reason

---

### REQ-TGT-019: Block List Management

Block list management capabilities SHALL be restricted by role:

| Action | Administrator | Engineer | Operator | Viewer |
|--------|:---:|:---:|:---:|:---:|
| View block list | Yes | Yes | Yes | No |
| Add entry | Yes | No | No | No |
| Deactivate entry | Yes | No | No | No |
| Reactivate entry | Yes | No | No | No |
| View audit history | Yes | Yes | No | No |

The block list management UI SHALL:
- Display all entries (active and inactive) with filtering options
- Show the reason, added-by user, and timestamp for each entry
- Provide search/filter by pattern, reason text, or status
- Display a count of campaigns currently affected by each active entry

**Acceptance Criteria:**
- [ ] RBAC for block list operations is enforced at the API level
- [ ] Non-Administrator users receive a `403 Forbidden` response when attempting to modify the block list
- [ ] Deactivated entries are visually distinguished from active entries
- [ ] The "affected campaigns" count reflects only active (non-archived) campaigns

---

### REQ-TGT-020: Block List Notifications

The system SHALL generate notifications when block list events occur:

| Event | Notification Recipients | Channel |
|-------|------------------------|---------|
| Target assigned to campaign matches block list | Campaign owner, all Administrators | In-app notification, WebSocket push |
| Block list entry added that matches targets in active campaigns | Affected campaign owners, all Administrators | In-app notification, WebSocket push |
| Block list override approved | Campaign owner, all Administrators | In-app notification, WebSocket push |
| Block list override rejected | Campaign owner | In-app notification, WebSocket push |

**Acceptance Criteria:**
- [ ] Notifications are delivered within 5 seconds of the triggering event via WebSocket
- [ ] Each notification includes a direct link to the relevant campaign or block list entry
- [ ] Notifications persist in the user's notification inbox until dismissed

---

## 6. Target Tracking

### REQ-TGT-021: Per-Target Campaign Status

Each target within a campaign SHALL have a tracked status reflecting the furthest stage of engagement. The status values are:

| Status | Description | Trigger |
|--------|-------------|---------|
| `pending` | Target is assigned but no email has been sent | Default on assignment |
| `email_sent` | Phishing email has been delivered (or delivery attempted) | SMTP confirmation or send event |
| `email_opened` | Target opened the phishing email | Tracking pixel loaded |
| `link_clicked` | Target clicked a link in the phishing email | Landing page request logged |
| `credential_submitted` | Target submitted data on the landing page | Form submission captured |
| `reported` | Target reported the phishing email (via internal reporting mechanism) | Report webhook or manual entry |

Status progression rules:
- Status always reflects the **highest-engagement** action observed. Once a target reaches `credential_submitted`, it does not revert to `link_clicked` even if additional link clicks occur.
- The `reported` status is independent and can coexist with any other status. A target can be both `credential_submitted` and `reported`. The system SHALL track `reported` as a separate boolean flag in addition to the engagement status.
- All status transitions are timestamped.

**Acceptance Criteria:**
- [ ] Status progresses forward only (except `reported`, which is independent)
- [ ] Each status transition is recorded with a timestamp
- [ ] The campaign target list can be filtered and sorted by status
- [ ] Status updates are reflected in the UI in real-time via WebSocket

---

### REQ-TGT-022: Target Interaction Timeline

For each target within a campaign, the system SHALL maintain a chronological timeline of all interactions:

| Event Type | Data Captured |
|-----------|---------------|
| `email_sent` | Timestamp, sending server, message ID |
| `email_bounced` | Timestamp, bounce type (hard/soft), bounce message |
| `email_opened` | Timestamp, user-agent (if available), IP address |
| `link_clicked` | Timestamp, URL clicked, user-agent, IP address |
| `page_visited` | Timestamp, landing page URL, user-agent, IP address, referrer |
| `credential_submitted` | Timestamp, fields submitted (field names only in timeline — values stored separately in Credential Capture module), IP address |
| `reported` | Timestamp, report source (e.g., phishing button, IT ticket), reporter notes |

The timeline SHALL be viewable in the UI for each target within a campaign, displayed in reverse chronological order (most recent first) with the option to sort ascending.

**Acceptance Criteria:**
- [ ] All event types listed above are captured and displayed
- [ ] The timeline loads within 2 seconds for targets with up to 100 events
- [ ] Credential values are NOT displayed in the timeline (only field names)
- [ ] The timeline supports filtering by event type
- [ ] IP addresses and user-agents are displayed but also available for bulk export

**Security Considerations:**
- IP addresses and user-agent strings are considered sensitive operational data. They are visible to Operators, Engineers, and Administrators but NOT to Viewers.
- Credential submission events in the timeline MUST NOT display submitted values. The credential data itself is managed by the Credential Capture module (08) with its own access controls.

---

### REQ-TGT-023: Cross-Campaign Target History

The system SHALL provide a view of a target's engagement across all campaigns they have participated in. This cross-campaign history SHALL display:

- A list of all campaigns the target has been assigned to, ordered by campaign date (most recent first)
- For each campaign: campaign name, campaign status, the target's engagement status, and a link to the campaign-specific timeline
- Summary statistics:
  - Total campaigns participated in
  - Number of times email was opened
  - Number of times link was clicked
  - Number of times credentials were submitted
  - Number of times the target reported the phishing email
- A trend indicator showing whether the target's susceptibility is increasing, decreasing, or stable over time

**Acceptance Criteria:**
- [ ] The cross-campaign view is accessible from the target detail screen
- [ ] The view includes campaigns from all time, including archived campaigns
- [ ] Summary statistics are accurate and update when new campaign data is recorded
- [ ] The trend indicator uses at least the last 3 campaigns to determine direction
- [ ] Campaigns where the target was `removed` before launch are excluded from statistics but listed separately

---

### REQ-TGT-024: Target Search and Filtering

The target list SHALL support the following search and filter capabilities:

| Criterion | Type | Description |
|-----------|------|-------------|
| Email address | Text search | Partial match, case-insensitive |
| First name / Last name | Text search | Partial match, case-insensitive |
| Department | Filter (dropdown) | Populated from existing department values |
| Title | Text search | Partial match, case-insensitive |
| Group membership | Filter (multi-select) | Show targets belonging to selected groups |
| Block list status | Filter (boolean) | Show only targets matching the block list |
| Campaign participation | Filter (campaign select) | Show targets assigned to a specific campaign |
| Engagement status | Filter (multi-select) | Show targets with a specific status in a selected campaign |
| Custom field | Key-value search | Search by custom field key and value |
| Created date | Date range | Filter by target creation date |

**Acceptance Criteria:**
- [ ] Search results update within 500ms for datasets up to 100,000 targets
- [ ] Multiple filters can be combined (AND logic)
- [ ] Active filters are displayed as removable chips/tags in the UI
- [ ] Filter state is preserved in the URL (shareable/bookmarkable)
- [ ] The target list supports pagination with configurable page sizes (25, 50, 100)

---

## 7. API Endpoints

The following REST API endpoints SHALL be implemented for target management:

### Targets

| Method | Path | Description | Roles |
|--------|------|-------------|-------|
| `GET` | `/api/v1/targets` | List targets (paginated, filterable) | Operator, Engineer, Admin |
| `POST` | `/api/v1/targets` | Create a single target | Operator, Engineer, Admin |
| `GET` | `/api/v1/targets/:id` | Get target detail | Operator, Engineer, Admin |
| `PUT` | `/api/v1/targets/:id` | Update a target | Operator, Engineer, Admin |
| `DELETE` | `/api/v1/targets/:id` | Soft-delete a target | Engineer, Admin |
| `POST` | `/api/v1/targets/:id/restore` | Restore a soft-deleted target | Admin |
| `GET` | `/api/v1/targets/:id/history` | Cross-campaign history | Operator, Engineer, Admin |

### Import

| Method | Path | Description | Roles |
|--------|------|-------------|-------|
| `POST` | `/api/v1/targets/import/upload` | Upload CSV file | Operator, Engineer, Admin |
| `GET` | `/api/v1/targets/import/:upload_id/preview` | Get preview and validation results | Operator, Engineer, Admin |
| `POST` | `/api/v1/targets/import/:upload_id/mapping` | Submit column mapping | Operator, Engineer, Admin |
| `POST` | `/api/v1/targets/import/:upload_id/commit` | Commit the import | Operator, Engineer, Admin |
| `GET` | `/api/v1/targets/import/:upload_id/rejected` | Download rejected rows CSV | Operator, Engineer, Admin |

### Bulk Operations

| Method | Path | Description | Roles |
|--------|------|-------------|-------|
| `POST` | `/api/v1/targets/bulk/add-to-group` | Add targets to group(s) | Operator, Engineer, Admin |
| `POST` | `/api/v1/targets/bulk/remove-from-group` | Remove targets from group(s) | Operator, Engineer, Admin |
| `POST` | `/api/v1/targets/bulk/edit` | Bulk edit a single field | Operator, Engineer, Admin |
| `POST` | `/api/v1/targets/bulk/delete` | Bulk soft-delete | Engineer, Admin |
| `POST` | `/api/v1/targets/bulk/export` | Export selected targets as CSV | Operator, Engineer, Admin |

### Groups

| Method | Path | Description | Roles |
|--------|------|-------------|-------|
| `GET` | `/api/v1/target-groups` | List all groups | Operator, Engineer, Admin |
| `POST` | `/api/v1/target-groups` | Create a group | Operator, Engineer, Admin |
| `GET` | `/api/v1/target-groups/:id` | Get group detail with member list | Operator, Engineer, Admin |
| `PUT` | `/api/v1/target-groups/:id` | Update group (name, description) | Operator, Engineer, Admin |
| `DELETE` | `/api/v1/target-groups/:id` | Delete a group | Engineer, Admin |
| `POST` | `/api/v1/target-groups/:id/members` | Add targets to group | Operator, Engineer, Admin |
| `DELETE` | `/api/v1/target-groups/:id/members` | Remove targets from group | Operator, Engineer, Admin |

### Block List

| Method | Path | Description | Roles |
|--------|------|-------------|-------|
| `GET` | `/api/v1/blocklist` | List all block list entries | Operator, Engineer, Admin |
| `POST` | `/api/v1/blocklist` | Add a block list entry | Admin |
| `GET` | `/api/v1/blocklist/:id` | Get block list entry detail | Operator, Engineer, Admin |
| `PUT` | `/api/v1/blocklist/:id/deactivate` | Deactivate entry | Admin |
| `PUT` | `/api/v1/blocklist/:id/reactivate` | Reactivate entry | Admin |
| `GET` | `/api/v1/blocklist/check` | Check an email against the block list | Operator, Engineer, Admin |

### Block List Override

| Method | Path | Description | Roles |
|--------|------|-------------|-------|
| `GET` | `/api/v1/campaigns/:id/blocklist-review` | Get blocked targets for a campaign | Engineer, Admin |
| `POST` | `/api/v1/campaigns/:id/blocklist-override` | Approve or reject override | Admin |

**Acceptance Criteria:**
- [ ] All endpoints enforce RBAC as specified
- [ ] All endpoints return appropriate HTTP status codes (200, 201, 400, 403, 404, 409, 422)
- [ ] All list endpoints support pagination (`page`, `per_page` query parameters)
- [ ] All endpoints produce audit log entries for state-changing operations
- [ ] All endpoints validate input and return structured error responses

---

## 7A. Canary Targets

### REQ-TGT-028: Canary Target Designation

The system SHALL support designating targets as "canary targets" at the campaign level. Canary targets are internal team members included in a campaign to verify that the phishing infrastructure is functioning correctly in production.

| Feature | Behavior |
|---------|----------|
| **Designation** | Any target can be flagged as a canary for a specific campaign during campaign configuration (Draft state). Canary status is per-campaign, not global — the same target can be a canary in one campaign and a regular target in another. |
| **Auto-verification** | When a canary target interacts with the campaign (opens email, clicks link, submits credentials), the system records a "canary verification" event confirming the campaign pipeline is working end-to-end. |
| **Priority sending** | Canary targets receive emails before any non-canary targets when the campaign launches. |
| **Metric exclusion** | Canary target interactions are tracked but excluded from campaign aggregate metrics (open rate, click rate, capture rate). |
| **Dashboard indicator** | The campaign dashboard includes a "Canary Status" panel showing each canary target, whether they have been sent an email, and whether they have verified (interacted). |

**Acceptance Criteria:**
- [ ] Canary targets can be designated and undesignated during campaign configuration
- [ ] Canary target emails are sent before non-canary target emails during Active state
- [ ] Canary interactions are recorded as normal events but flagged as `is_canary = true`
- [ ] Aggregate campaign metrics exclude canary-flagged events
- [ ] The campaign dashboard "Canary Status" panel updates in real time
- [ ] Campaign reports include a separate canary section when canary targets are present

---

## 7B. Organizational Hierarchy (v2 Design Note)

> **v2 Feature — Design Note Only**
>
> A future version of Tackle will support a hierarchical organizational data model for targets, enabling:
>
> - **Organization → Division → Department → Team** hierarchy
> - Targets associated with positions in the hierarchy
> - Metrics and reporting segmented by organizational unit at any level
> - Campaigns scoped to specific organizational units
> - Automatic group creation from organizational units
>
> The v1 data model should accommodate this by:
> - Using the existing flat `department` field on targets (REQ-TGT-001) for v1
> - Designing the `custom_fields` JSONB to support future structured hierarchy fields (e.g., `division`, `team`, `org_unit`)
> - Not introducing database constraints that would prevent adding a `target_org_positions` join table in v2
> - Ensuring the reporting queries can be extended to support multi-level group-by without schema changes
>
> No v1 code needs to implement the organizational hierarchy itself. This note ensures schema and API decisions do not preclude it.

---

## 8. Performance Requirements

### REQ-TGT-025: Scalability

| Metric | Requirement |
|--------|-------------|
| Total targets in system | Support up to 500,000 targets without degradation |
| Targets per campaign | Support up to 50,000 targets per campaign |
| Groups in system | Support up to 1,000 groups |
| Targets per group | Support up to 100,000 targets per group |
| CSV import size | Process 50,000-row CSV files within 60 seconds |
| Block list entries | Support up to 10,000 entries with sub-100ms matching |

**Acceptance Criteria:**
- [ ] Target list pagination loads within 500ms at maximum dataset sizes
- [ ] Block list matching against a single email completes in under 10ms
- [ ] CSV import of 50,000 rows completes within 60 seconds including validation
- [ ] Bulk operations on 10,000 targets complete within 30 seconds

---

## 9. Data Retention and Privacy

### REQ-TGT-026: Data Retention

- Active targets are retained indefinitely
- Soft-deleted targets are permanently purged after the configurable retention period (REQ-TGT-010)
- Target interaction data (timelines) is retained as long as the associated campaign exists
- When a target is permanently purged, all associated interaction data across all campaigns is also purged
- Block list entries are never permanently deleted (deactivation only) for audit completeness

**Acceptance Criteria:**
- [ ] Permanent purge removes all PII including email, name, department, title, and custom fields
- [ ] Purged targets in historical campaign data are replaced with a placeholder (e.g., "[purged target]") to maintain campaign statistics accuracy
- [ ] An automated job runs the purge process daily and logs all purged records to the audit log

**Security Considerations:**
- The purge process must be thorough. PII must not remain in database backups beyond the backup retention period. Document the interaction between target purge and backup policy.

---

## 10. Audit Requirements

### REQ-TGT-027: Audit Trail

All target management operations SHALL produce audit log entries per the Audit Logging specification (11). The following events are mandatory:

| Event | Severity | Data Captured |
|-------|----------|---------------|
| Target created | INFO | Target ID, email (masked), created-by user |
| Target updated | INFO | Target ID, changed fields (old and new values), updated-by user |
| Target soft-deleted | WARN | Target ID, email (masked), deleted-by user |
| Target restored | WARN | Target ID, email (masked), restored-by user |
| Target permanently purged | WARN | Target ID, purge reason (retention expiry) |
| CSV import completed | INFO | Upload ID, total rows, imported count, rejected count, importing user |
| Group created/updated/deleted | INFO | Group ID, group name, acting user |
| Group membership changed | INFO | Group ID, target IDs added/removed, acting user |
| Block list entry added | WARN | Entry ID, pattern, reason, added-by user |
| Block list entry deactivated | WARN | Entry ID, pattern, deactivated-by user |
| Block list override approved | CRITICAL | Campaign ID, overridden target list, approving admin, justification |
| Block list override rejected | WARN | Campaign ID, rejecting admin, rejection reason |
| Bulk operation executed | INFO | Operation type, target count, acting user |

**Acceptance Criteria:**
- [ ] All listed events produce audit log entries
- [ ] Audit log entries for block list overrides are queryable as a distinct event type
- [ ] Email addresses in audit logs are masked (e.g., `c**@company.com`) except in block list override entries where full addresses are required for accountability
- [ ] Audit log entries include correlation IDs linking related operations (e.g., a bulk operation and its individual target updates)
