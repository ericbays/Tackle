# 11 — Audit Log Viewer

This document specifies the **Audit Log Viewer** — a full-page, real-time log inspection tool for reviewing all system activity within the Tackle platform. The viewer is designed for high-volume log environments and provides filtering, full-text search, correlation chain tracing, HMAC integrity verification, and live tail mode via WebSocket. Log entries are immutable; the UI enforces read-only presentation throughout.

---

## 1. Page Layout

### 1.1 Purpose

The Audit Log Viewer provides a chronological, filterable view of every recorded event in the system. It serves both operational monitoring (real-time tail) and forensic investigation (search, filter, correlation tracing). Access is scoped by role: Operators see only entries where `actor_id` matches their own user ID; Engineers and Admins see all entries.

### 1.2 Navigation

- Sidebar location: **System > Audit Logs**.
- Requires permission: `logs:read`.
- Page title: "Audit Logs".

### 1.3 Full-Page Layout

The viewer occupies the full content area. No cards or dashboard grid — the log table is the primary element, maximizing vertical space for log entries.

```
┌──────────────────────────────────────────────────────────────────────────┐
│  Audit Logs                                  [Verify Integrity] [Export]│
├──────────────────────────────────────────────────────────────────────────┤
│  Filter Bar                                                             │
│  [Category ▾] [Severity ▾] [Actor ▾] [Campaign ▾] [Action ▾]          │
│  [Resource ▾] [Date range ···] [Search...                        🔍]   │
├──────────────────────────────────────────────────────────────────────────┤
│  ● Live  │ 12,847 entries matching filters          [Clear filters]     │
├──────────┴───────────────────────────────────────────────────────────────┤
│  Timestamp              Sev  Category        Actor     Action  Resource │
├──────────────────────────────────────────────────────────────────────────┤
│  2026-04-03 14:32:01.4  INF  user_activity   j.doe    login   session  │
│  2026-04-03 14:31:58.2  INF  email_event     system   sent    email    │
│  2026-04-03 14:31:55.0  WRN  infrastructure  system   restart endpoint │
│  2026-04-03 14:31:50.1  INF  user_activity   a.smith  update  campaign │
│  2026-04-03 14:31:48.7  ERR  system          system   fail    job      │
│  ...                                                                    │
│  (infinite scroll — older entries load on scroll down)                  │
└──────────────────────────────────────────────────────────────────────────┘
```

When a correlation chain is open, the layout splits:

```
┌──────────────────────────────────────────────────────────────────────────┐
│  Audit Logs                                  [Verify Integrity] [Export]│
├──────────────────────────────────────────────────────────────────────────┤
│  Filter Bar (same as above)                                             │
├──────────────────────────────────────────┬───────────────────────────────┤
│  Log Table (left, ~60%)                  │  Correlation Chain (right)   │
│                                          │  correlation_id: abc-123     │
│  2026-04-03 14:32:01.4 INF login ...    │  ┌─ 14:31:48.7 request      │
│  2026-04-03 14:31:58.2 INF sent  ...    │  │  POST /api/v1/campaigns   │
│  2026-04-03 14:31:55.0 WRN restart ...  │  ├─ 14:31:48.9 user_activity│
│ >2026-04-03 14:31:50.1 INF update ...   │  │  campaign.update          │
│  2026-04-03 14:31:48.7 ERR fail   ...   │  ├─ 14:31:49.1 email_event  │
│  ...                                     │  │  notification.sent        │
│                                          │  └─ 14:31:49.3 system       │
│                                          │     job.enqueued             │
│                                          │                    [✕ Close] │
├──────────────────────────────────────────┴───────────────────────────────┤
```

### 1.4 Header Bar

- **Title**: "Audit Logs" — left-aligned, `text-primary`, heading size from design system.
- **Verify Integrity** button: secondary style. Opens the integrity verification flow (section 8).
- **Export** button: secondary style. Exports the current filtered result set to CSV (section 9).

---

## 2. Column Display

### 2.1 Table Columns

The log table uses a monospace font (`font-family: var(--font-mono)`) for all cell content. The table header is sticky and remains visible during scroll.

| Column | Width | Content | Sortable |
|--------|-------|---------|----------|
| Timestamp | 200px | UTC timestamp, formatted `YYYY-MM-DD HH:mm:ss.f` (1 fractional digit). Full microsecond precision shown in tooltip on hover. | No (always chronological) |
| Severity | 48px | Three-letter abbreviation badge: `DBG`, `INF`, `WRN`, `ERR`, `CRT`. | No |
| Category | 130px | Category label, sentence case: "User Activity", "Email Event", "Infrastructure", "Request", "System". | No |
| Actor | 100px | `actor_label` value, truncated with tooltip at 12 chars. System actors display as "system" in `--text-muted`. | No |
| Action | 120px | The `action` field value, monospace. | No |
| Resource | 120px | `resource_type`, truncated with tooltip. If `resource_id` exists, displayed as `type/id` (e.g., `campaign/42`). | No |
| IP | 120px | `source_ip` value. Hidden by default on viewports below 1440px. | No |

### 2.2 Severity Badge Colors

Each severity level uses a colored badge with the abbreviation text:

| Severity | Badge BG | Text | Abbreviation |
|----------|----------|------|-------------|
| debug | `--bg-tertiary` | `--text-muted` | DBG |
| info | `--info-muted` | `--info` | INF |
| warning | `--warning-muted` | `--warning` | WRN |
| error | `--danger-muted` | `--danger` | ERR |
| critical | `--danger` | `--text-inverse` | CRT |

Critical entries additionally receive a full-row left border of 3px in `--danger`.

### 2.3 Row Behavior

- **Hover**: `--bg-hover` background.
- **Click**: Expands the row to show full detail (section 4). A second click collapses it.
- **Selected/expanded row**: `--accent-primary-muted` background.
- **Row density**: 36px row height. Compact enough to fit ~20 rows in a typical 900px content area.
- **Alternating row shading**: Not used. Borders between rows use `--border-subtle` (1px solid) for readability.
- **Keyboard focus**: Visible focus ring using `--accent-primary` on the currently focused row.

### 2.4 Timestamp Display

- All timestamps display in UTC by default.
- A small toggle in the filter bar area (icon: clock with "UTC" / "Local") allows switching between UTC and the browser's local timezone.
- When local timezone is active, the column header reads "Timestamp (local)" and a subtle label below the toggle shows the detected timezone (e.g., "America/New_York").
- Relative time ("2 min ago") is shown as a tooltip when hovering the timestamp in either mode.

---

## 3. Filter Controls

### 3.1 Filter Bar Layout

The filter bar sits directly below the page header and above the result summary row. It wraps onto two lines if needed. All filters apply immediately on change (no "Apply" button). Active filters are visually indicated by the dropdown trigger showing the selected value in `--accent-primary` text.

```
┌──────────────────────────────────────────────────────────────────────────┐
│ [Category ▾] [Severity ▾] [Actor ▾] [Campaign ▾] [Action ▾]           │
│ [Resource ▾] [2026-03-01 → 2026-04-03] [Search...                  🔍]│
└──────────────────────────────────────────────────────────────────────────┘
```

### 3.2 Filter Definitions

**Category** (multi-select dropdown):
- Options: User Activity, Email Event, Infrastructure, Request, System.
- Default: all selected (no filter applied).
- Selecting one or more limits results to those categories.
- Chip display: when fewer than all are selected, show selected count on trigger, e.g., "Category (2)".

**Severity** (multi-select dropdown):
- Options: Debug, Info, Warning, Error, Critical.
- Default: Info, Warning, Error, Critical selected (debug hidden by default).
- Severity options are color-coded with their badge colors.

**Actor** (searchable single-select dropdown):
- Typeahead search against `actor_label` values.
- Shows recent actors first, then search results.
- For Operators, this filter is locked to their own user and hidden (since they can only see their own entries).
- Selecting an actor filters to all entries where `actor_id` matches.
- Options include a system/endpoint/external grouping header.

**Campaign** (searchable single-select dropdown):
- Typeahead search against campaign names.
- Filters to entries where `campaign_id` matches the selected campaign.
- Shows only campaigns the current user has access to.
- Populated from `GET /api/v1/campaigns?per_page=50` with search.

**Action** (searchable multi-select dropdown):
- Typeahead search against known action values.
- Options are populated from a distinct list of actions seen in recent logs.
- Source: `GET /api/v1/logs/audit?distinct=action`.

**Resource** (combo filter with two parts):
- Resource Type: dropdown with known resource types (campaign, endpoint, email, user, template, landing_page, session, job).
- Resource ID: text input, visible only when a resource type is selected.
- When both are set, filters to `resource_type=X&resource_id=Y`.

**Date Range** (date-time range picker):
- Two date-time inputs: start and end.
- Preset buttons: "Last 1h", "Last 24h", "Last 7d", "Last 30d", "Custom".
- Default: Last 24 hours.
- Date-time picker uses the platform's standard date picker component with time precision to the minute.
- The range is displayed inline on the filter bar as `YYYY-MM-DD → YYYY-MM-DD` when set.

**Full-Text Search** (text input):
- Searches across: `action`, `actor_label`, `resource_type`, `resource_id`, `source_ip`, and the serialized `details` JSONB field.
- Debounced: 400ms after the user stops typing.
- Minimum 3 characters to trigger search.
- Matching terms are highlighted in `--warning` within the result rows.
- The search icon button (or pressing Enter) triggers an immediate search, bypassing the debounce.

### 3.3 Filter State Management

- All active filters are serialized to URL query parameters. Bookmarking or sharing the URL reproduces the exact filter state.
- The "Clear filters" link (visible in the result summary row when any filter is active) resets all filters to defaults and clears the URL parameters.
- Filter changes reset the scroll position to the top (newest entries).
- Each filter change triggers a new `GET /api/v1/logs/audit` request with the combined filter parameters.

### 3.4 Result Summary Row

Between the filter bar and the table sits a single-line summary:

```
● Live  │ 12,847 entries matching filters          [Clear filters]
```

- **Live indicator**: green pulsing dot when tail mode is active (section 6). Gray dot with "Paused" when tail mode is off.
- **Entry count**: total number of matching entries from the API's `pagination.total` field. Formatted with thousands separators.
- **Clear filters**: text link, `--accent-primary`, visible only when at least one non-default filter is active.

---

## 4. Expandable Row Detail

### 4.1 Expansion Behavior

Clicking any row expands an inline detail panel directly below that row. Only one row can be expanded at a time — expanding a new row collapses the previous one. The expansion animates with a 150ms slide-down transition.

### 4.2 Detail Panel Layout

The expanded detail panel spans the full table width and has a `--bg-secondary` background with `--border-subtle` top and bottom borders.

```
┌──────────────────────────────────────────────────────────────────────────┐
│  2026-04-03 14:31:50.123456 UTC                                         │
├──────────────────────────────────┬───────────────────────────────────────┤
│  Field          Value            │  Details (JSON)                      │
│  ─────          ─────            │  ─────────────                       │
│  ID             a1b2c3d4...      │  {                                   │
│  Severity       info             │    "previous_state": "building",     │
│  Category       user_activity    │    "new_state": "ready",             │
│  Action         update           │    "fields_changed": [               │
│  Actor          j.doe (user)     │      "state",                        │
│  Resource       campaign/42      │      "updated_at"                    │
│  Source IP      192.168.1.100    │    ],                                │
│  Session        sess_8f3a...     │    "readiness": {                    │
│  Campaign       campaign/42      │      "score": 100,                   │
│  Correlation    corr_abc123  →   │      "checks_passed": 5             │
│  Checksum       hmac_sha256:...  │    }                                 │
│                                  │  }                                   │
├──────────────────────────────────┴───────────────────────────────────────┤
│  [Copy JSON]  [Copy Entry ID]  [View Correlation Chain →]               │
└──────────────────────────────────────────────────────────────────────────┘
```

### 4.3 Field Display (Left Column)

All fields from the log entry schema are displayed as a key-value list:

| Field | Display | Interaction |
|-------|---------|-------------|
| ID | Full UUID, monospace, `--text-muted` | Click to copy |
| Timestamp | Full microsecond precision, UTC | None |
| Severity | Badge (same as table column) | None |
| Category | Sentence case label | None |
| Action | Monospace text | None |
| Actor | `actor_label` with `actor_type` in parentheses | Clickable — navigates to user profile if `actor_type=user` |
| Resource | `resource_type/resource_id` | Clickable — navigates to the resource page (e.g., campaign workspace) if the resource type has a known route |
| Source IP | Monospace | Click to copy |
| Session ID | Truncated with tooltip, monospace | Click to filter logs by this session ID |
| Campaign ID | `campaign/ID` | Clickable — navigates to campaign workspace |
| Correlation ID | Truncated with arrow icon (→) | Clickable — opens correlation chain side panel (section 5) |
| Checksum | Truncated HMAC value, monospace, `--text-muted` | Tooltip shows full value |

### 4.4 Details JSON Tree (Right Column)

- The `details` JSONB field is rendered as a syntax-highlighted, collapsible JSON tree.
- Root-level keys are expanded by default. Nested objects/arrays are collapsed by default if they contain more than 5 keys/items.
- Syntax highlighting uses the design system colors: keys in `--accent-primary`, strings in `--success`, numbers in `--warning`, booleans in `--info`, null in `--text-muted`.
- If `details` is null or an empty object, the right column displays "No additional details" in `--text-muted`.

### 4.5 Action Buttons (Bottom)

- **Copy JSON**: copies the entire log entry as formatted JSON to the clipboard. Toast: "Log entry copied to clipboard."
- **Copy Entry ID**: copies the entry UUID. Toast: "Entry ID copied."
- **View Correlation Chain**: opens the correlation chain side panel (section 5). Only visible if `correlation_id` is non-null.

---

## 5. Correlation Chain View

### 5.1 Purpose

A correlation chain groups all log entries that share the same `correlation_id` into a timeline view. This allows tracing a single user action (e.g., a campaign launch) through all the system events it triggered — the initial request, state changes, email dispatches, background jobs, and any errors.

### 5.2 Triggering

The correlation chain panel opens when the user:
- Clicks a `correlation_id` value in an expanded row detail.
- Clicks "View Correlation Chain" in the row detail action bar.
- Uses the keyboard shortcut `C` while a row with a correlation ID is focused.

### 5.3 Side Panel Layout

The panel slides in from the right edge, occupying approximately 40% of the page width (min 400px, max 600px). The log table compresses to fill the remaining space. The panel has a `--bg-secondary` background and `--border-default` left border.

```
┌─────────────────────────────────────────┐
│  Correlation Chain               [✕]    │
│  corr_abc-123-def-456                   │
│  4 events · span: 1.2s                  │
├─────────────────────────────────────────┤
│                                         │
│  ┌─ 14:31:48.712345 ──────────────────  │
│  │  ● request                           │
│  │  POST /api/v1/campaigns/42/launch    │
│  │  Actor: j.doe · IP: 192.168.1.100   │
│  │                                      │
│  ├─ 14:31:48.923456 ──────────────────  │
│  │  ● user_activity                     │
│  │  campaign.state_change               │
│  │  building → active                   │
│  │                                      │
│  ├─ 14:31:49.101234 ──────────────────  │
│  │  ● email_event                       │
│  │  notification.dispatch               │
│  │  142 recipients queued               │
│  │                                      │
│  └─ 14:31:49.312456 ──────────────────  │
│     ● system                            │
│     job.enqueued                         │
│     email_batch_worker                   │
│                                         │
├─────────────────────────────────────────┤
│  [Verify Chain Integrity]               │
└─────────────────────────────────────────┘
```

### 5.4 Panel Header

- **Title**: "Correlation Chain" in heading style.
- **Close button**: `✕` in the top-right corner. Pressing `Escape` also closes the panel.
- **Correlation ID**: full value displayed in monospace, `--text-muted`. Click to copy.
- **Summary line**: event count and time span between first and last event (e.g., "4 events, span: 1.2s").

### 5.5 Timeline Entries

Each entry in the timeline is rendered as a node on a vertical line:

- **Connector**: a vertical line in `--border-default` connects the nodes. The first node uses `┌─`, middle nodes use `├─`, and the last node uses `└─`.
- **Timestamp**: full microsecond precision, displayed in `--text-muted` monospace.
- **Category dot**: a small colored circle using the severity badge color for the entry's severity.
- **Category label**: sentence case, `--text-secondary`.
- **Action**: the `action` field, `--text-primary`, monospace.
- **Context line**: a brief summary extracted from `details` — up to 40 characters. For state changes, shows `old → new`. For requests, shows the HTTP method and path. For email events, shows recipient count. Falls back to resource type/ID if no meaningful summary can be extracted.
- **Actor line**: `actor_label` and `source_ip`, `--text-muted`, shown only if the actor differs from the previous entry in the chain.

### 5.6 Timeline Interactions

- Clicking a timeline entry scrolls the main log table to that entry and expands its detail panel.
- The currently selected entry (the one that triggered the chain view) is highlighted with `--accent-primary-muted` background.
- Hovering a timeline entry shows a subtle `--bg-hover` background.
- If the chain contains more than 50 entries, the timeline is paginated with a "Load more" button at the bottom.

### 5.7 Chain Integrity Verification

A "Verify Chain Integrity" button at the bottom of the panel triggers HMAC verification for all entries in the chain (see section 8.3). Results are displayed inline on each timeline node.

### 5.8 Data Loading

- Source: `GET /api/v1/logs/audit?correlation_id={id}&per_page=100&sort=timestamp:asc`.
- Loading state: skeleton shimmer on 4 placeholder timeline nodes.
- Error state: inline error message with retry button.

---

## 6. Real-Time Tail Mode

### 6.1 Purpose

Tail mode streams new log entries to the viewer in real time via WebSocket, replicating the experience of `tail -f` on a server log. It is useful for live monitoring during active campaigns or infrastructure changes.

### 6.2 WebSocket Connection

- Endpoint: `wss://{host}/api/v1/logs/audit/stream`.
- Authentication: the WebSocket upgrade request includes the same auth token used for REST API calls (sent as a query parameter `token` or via the `Sec-WebSocket-Protocol` header).
- The connection sends the current filter state as the initial message so the server only pushes matching entries.
- Filter changes while tail mode is active send an updated filter message over the existing connection (no reconnect).

### 6.3 Toggle Control

- The tail mode toggle is located in the result summary row, replacing the gray paused indicator.
- **Off state**: gray dot, text "Paused". Clicking the dot or text activates tail mode.
- **On state**: green pulsing dot (CSS animation: `opacity` oscillating between 0.4 and 1.0 over 2 seconds), text "Live". Clicking deactivates tail mode.
- Keyboard shortcut: `T` toggles tail mode on/off.

### 6.4 Entry Insertion Behavior

- New entries appear at the top of the log table (newest first).
- Each new entry slides in from the top with a 200ms animation.
- New entries have a brief `--accent-primary-muted` background highlight that fades over 3 seconds.
- If the user has scrolled down (away from the top), new entries still accumulate at the top but the scroll position is not disturbed. A floating badge appears at the top of the table: "↑ 12 new entries" (count updates in real time). Clicking this badge scrolls to the top.
- If the user is at the top of the table (scroll position 0), new entries push older entries down automatically.

### 6.5 Rate Limiting and Batching

- If entries arrive faster than 10 per second, they are batched and inserted in groups every 200ms to prevent DOM thrashing.
- If more than 500 entries accumulate in the buffer without being rendered (e.g., the user scrolled far down), the buffer is truncated to the newest 500 and a toast appears: "High log volume — some entries were skipped in the live view. Use filters to narrow results."

### 6.6 Connection State

- **Connecting**: the dot is yellow, text "Connecting...".
- **Connected**: green pulsing dot, text "Live".
- **Disconnected**: red dot, text "Disconnected". Automatic reconnection with exponential backoff (1s, 2s, 4s, 8s, max 30s). After 5 failed attempts, show inline error: "Live connection lost. [Retry]".
- **Reconnected**: on successful reconnect, fetch any entries missed during the gap via REST API (`GET /api/v1/logs/audit?after={last_seen_id}`) and insert them into the table.

### 6.7 Tail Mode and Filters

- Activating tail mode while filters are applied streams only entries matching those filters.
- Changing filters while tail mode is active updates the stream filter in real time.
- Clearing all filters while in tail mode streams all entries (subject to role-based scoping).

---

## 7. Infinite Scroll Pagination

### 7.1 Scroll Direction

The log table loads the newest entries first. Scrolling **down** loads **older** entries. This matches the natural reading direction — the most recent events are at the top.

### 7.2 Page Loading

- Initial page: `GET /api/v1/logs/audit?per_page=50&sort=timestamp:desc` with any active filters.
- Next page trigger: when the user scrolls within 300px of the bottom of the current entries, the next page is requested.
- Cursor-based pagination: use the `after` parameter with the ID of the last loaded entry, not offset-based pagination, to ensure consistency as new entries arrive.
- Request: `GET /api/v1/logs/audit?per_page=50&sort=timestamp:desc&after={last_entry_id}`.

### 7.3 Loading Indicator

- While a page is loading, a row of skeleton shimmer placeholders (5 rows) appears at the bottom of the table.
- The loading indicator is centered below the last real row.

### 7.4 End of Results

- When the API returns fewer than `per_page` results, no further requests are made on scroll.
- A subtle end marker appears: a horizontal line with centered text "Beginning of log" in `--text-muted`.

### 7.5 DOM Recycling

To maintain performance with large log volumes, the viewer implements virtual scrolling:

- Only rows within the viewport plus a 500px buffer above and below are rendered as actual DOM nodes.
- Rows outside the buffer are replaced with empty spacer elements of the correct height.
- Scroll position and expanded row state are preserved during virtualization.
- Target: maintain 60fps scroll performance with 100,000+ loaded entries.

---

## 8. HMAC Integrity Verification

### 8.1 Purpose

Each log entry has a `checksum` field containing an HMAC-SHA256 value computed from the entry's content. The verification UI lets users confirm that entries have not been tampered with — that the `reject_modification` database trigger is intact and no direct database manipulation has occurred.

### 8.2 Single Entry Verification

- In the expanded row detail, the checksum field displays a small "Verify" link next to the truncated HMAC value.
- Clicking "Verify" sends `POST /api/v1/logs/audit/{id}/verify`.
- While verifying: the link text changes to "Verifying..." with a spinner.
- **Valid response**: a green checkmark icon replaces the link, with text "Integrity verified" in `--success`. Persists until the row is collapsed.
- **Invalid response**: a red warning icon with text "INTEGRITY FAILURE — entry may have been tampered with" in `--danger`. This state also triggers a toast notification: "Integrity check failed for entry {id}. Investigate immediately."
- **Error response** (e.g., network failure): gray text "Verification failed — retry" as a clickable link.

### 8.3 Bulk Verification (Correlation Chain)

- The "Verify Chain Integrity" button in the correlation chain panel verifies all entries in the chain sequentially.
- Each timeline node receives an inline status indicator as its verification completes: green checkmark, red warning, or gray error icon.
- A summary appears above the button after completion: "4/4 entries verified" (green) or "3/4 entries verified — 1 FAILED" (red, bold on the failure count).

### 8.4 Global Verification

- The "Verify Integrity" button in the page header opens a confirmation modal:
  - Title: "Verify Log Integrity"
  - Body: "This will verify the HMAC chain integrity of the current filtered result set ({count} entries). This may take several minutes for large result sets. Continue?"
  - Actions: "Cancel" (secondary), "Start Verification" (primary).
- Verification proceeds in batches of 50 entries via repeated `POST /api/v1/logs/audit/{id}/verify` calls.
- A progress bar appears in the result summary row during verification: "Verifying... 230/12,847 (1.8%)" with a "Cancel" link.
- On completion: toast "Integrity verification complete. {passed}/{total} entries verified." If any failed, the toast is styled as an error with the failure count.
- Failed entries are flagged in the table with a red left-border indicator (same as critical severity styling) and a small warning icon in the timestamp column.

---

## 9. Export to CSV

### 9.1 Trigger

The "Export" button in the page header exports the current filtered result set.

### 9.2 Behavior

- Clicking "Export" opens a confirmation popover below the button:
  - "Export {count} entries as CSV?"
  - If count exceeds 100,000: warning text "Large export — this may take a moment."
  - Actions: "Cancel", "Export CSV".
- Export is performed server-side. The frontend sends the current filter parameters to `GET /api/v1/logs/audit?format=csv` (same filters as the current view).
- The browser initiates a file download with filename: `tackle_audit_log_{YYYYMMDD_HHmmss}.csv`.

### 9.3 CSV Columns

All schema fields are included as columns: `id`, `timestamp`, `category`, `severity`, `actor_type`, `actor_id`, `actor_label`, `action`, `resource_type`, `resource_id`, `details`, `correlation_id`, `source_ip`, `session_id`, `campaign_id`, `checksum`.

- The `details` field is serialized as a JSON string within the CSV cell.
- Timestamps are in ISO 8601 format with full microsecond precision.

### 9.4 Progress

- While the export is being generated, the "Export" button shows a spinner and is disabled.
- If the export takes longer than 10 seconds, a toast appears: "Generating export... This may take a moment for large result sets."
- On completion, the download starts automatically and a toast confirms: "Export complete — {count} entries."

---

## 10. Error States and Edge Cases

### 10.1 Initial Load Failure

- If `GET /api/v1/logs/audit` fails on page load, the table area shows a centered error state:
  - Icon: warning triangle in `--danger`.
  - Heading: "Failed to load audit logs."
  - Detail: the HTTP error message or "Network error — check your connection."
  - Action: "Retry" button (primary).

### 10.2 Empty Results

- When filters return zero entries, the table area shows:
  - Icon: search icon in `--text-muted`.
  - Heading: "No log entries match your filters."
  - Detail: "Try adjusting your filters or expanding the date range."
  - The filter bar remains active above this message.

### 10.3 Page Load Failure (During Scroll)

- If a subsequent page request fails during infinite scroll, the skeleton loading indicator is replaced with an inline error row:
  - "Failed to load more entries. [Retry]"
  - Clicking "Retry" re-requests the same page.
  - The error row does not prevent scrolling back up through already-loaded entries.

### 10.4 WebSocket Errors

- Handled in section 6.6 (connection state management).
- If the WebSocket endpoint returns a 403 (insufficient permissions), tail mode is permanently disabled with a tooltip: "Live mode is not available for your role."

### 10.5 Stale Data

- If the user leaves the tab and returns after more than 5 minutes (detected via `document.visibilitychange`), the viewer:
  - If tail mode was active: reconnects the WebSocket and backfills missed entries.
  - If tail mode was off: shows a subtle banner at the top of the table: "Log data may be stale. [Refresh]". Clicking "Refresh" reloads the current filter set from the API.

### 10.6 Operator Scoping

- Operators see only their own log entries. The API enforces this server-side.
- The Actor filter is hidden for Operators (it would always be themselves).
- The page subtitle for Operators reads: "Showing your activity" in `--text-muted` below the heading.

### 10.7 Concurrent Integrity Verification

- If the user triggers a global verification while a chain verification is in progress, a toast warns: "A verification is already in progress." The second request is ignored.

### 10.8 Entry Detail for Deleted Resources

- When a resource link in the expanded detail points to a resource that no longer exists (returns 404 on navigation), the link still displays but appends "(deleted)" in `--text-muted` after the resource identifier if the API returns metadata indicating deletion.
- If the resource type has no known route (e.g., a custom resource type), the value is displayed as plain text rather than a link.

---

## 11. Keyboard Shortcuts

All shortcuts are active when the log viewer page has focus. They are disabled when a modal or text input is focused.

| Key | Action |
|-----|--------|
| `T` | Toggle tail mode on/off |
| `C` | Open correlation chain for the focused row (if it has a correlation ID) |
| `Enter` | Expand/collapse the focused row detail |
| `Escape` | Close the correlation chain panel; if already closed, collapse the expanded row |
| `↑` / `↓` | Move focus between rows |
| `J` / `K` | Move focus down / up (vim-style alternative) |
| `G` then `G` | Jump to the top of the log (newest entries) — two-key chord |
| `Shift+G` | Jump to the bottom of the currently loaded entries |
| `F` | Focus the full-text search input |
| `/` | Focus the full-text search input (vim-style alternative) |
| `X` | Export current filter set to CSV |
| `Ctrl+C` | Copy the focused row's entry ID to clipboard |

A keyboard shortcut reference is accessible via the `?` key, which opens a popover listing all shortcuts.

---

## 12. Performance Considerations

### 12.1 Virtual Scrolling

As described in section 7.5, virtual scrolling is mandatory. The viewer must handle 100,000+ entries without degradation. Implementation requirements:

- Use a virtual list library (e.g., `@tanstack/react-virtual` or equivalent).
- Fixed row height of 36px enables O(1) scroll position calculations.
- Expanded row detail panels have dynamic height — measure once on expand, cache for re-render.

### 12.2 API Request Optimization

- Debounce all filter changes by 300ms (except severity and category toggles, which apply immediately as they do not require a network-expensive text search).
- Full-text search is debounced at 400ms (section 3.2).
- Cancel in-flight requests when filters change before the previous request completes (use `AbortController`).
- Cache the distinct action list for the Action filter dropdown (refresh on page load, not on every open).

### 12.3 WebSocket Efficiency

- The WebSocket message format is compact JSON: only the fields needed for table display are sent initially. Full entry details are fetched on-demand via `GET /api/v1/logs/audit/{id}` when a row is expanded.
- Batch DOM insertions for high-frequency streams (section 6.5).
- Pause WebSocket processing when the browser tab is hidden (`document.hidden === true`). Buffer messages and process them when the tab becomes visible, up to the 500-entry buffer limit.

### 12.4 JSON Tree Rendering

- The `details` JSON tree in the expanded row detail is rendered lazily — only when the row is expanded.
- Deeply nested JSON (more than 10 levels) is truncated with a "Show full JSON" toggle that renders a plain formatted code block.
- Large JSON values (strings longer than 500 characters) are truncated with an expand control.

### 12.5 Correlation Chain Caching

- Correlation chain results are cached by `correlation_id`. If the user opens the same chain again (within the same session), the cached data is used and a background refresh is triggered.
- Cache invalidation: when a new entry arrives via WebSocket with a `correlation_id` that matches a cached chain, the cache is invalidated.

### 12.6 Memory Management

- Entries loaded via infinite scroll beyond the 10,000 mark are eligible for eviction from the in-memory store. The newest 10,000 entries are retained; older entries are dropped and re-fetched on scroll.
- Expanded row detail data (the full entry including `details` JSON) is held in a separate LRU cache limited to 100 entries.

---

## 13. Accessibility

### 13.1 Table Semantics

- The log table uses proper `<table>`, `<thead>`, `<tbody>`, `<tr>`, `<th>`, and `<td>` elements (not `div`-based grids) to ensure screen reader compatibility.
- Column headers have `scope="col"`.
- Expanded row details use `aria-expanded` on the parent row and `role="region"` on the detail panel.

### 13.2 Live Region for Tail Mode

- When tail mode is active, new entries are announced to screen readers via an `aria-live="polite"` region. Announcements are throttled to at most one per 5 seconds to avoid overwhelming the user: "{count} new log entries received."

### 13.3 Severity Communication

- Severity badges use both color and text labels, never color alone.
- Critical entries additionally use an `aria-label` of "Critical severity" on the badge element.

### 13.4 Focus Management

- Opening the correlation chain side panel moves focus to the panel header.
- Closing the panel returns focus to the row that triggered it.
- Modal dialogs (verification confirmation, export confirmation) trap focus within the modal.
