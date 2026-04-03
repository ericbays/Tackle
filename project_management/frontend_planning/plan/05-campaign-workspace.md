# 05 — Campaign Workspace

This document specifies the **Campaign List View** and the **Campaign Workspace** — the tabbed full-page environment where operators build, manage, and monitor phishing simulation campaigns. Campaigns are long-lived projects assembled over days or weeks, not quick forms. The workspace reflects this by providing a persistent, tab-organized interface where each section saves independently and readiness is tracked holistically.

---

## 1. Campaign List View

### 1.1 Purpose

The Campaign List is the entry point to all campaign operations. It provides a filterable, sortable table of all campaigns with status visibility, bulk operations, and an alternative calendar view for schedule-oriented planning.

### 1.2 Layout

```
┌──────────────────────────────────────────────────────────────────────┐
│  Campaigns                                    [+ New Campaign]      │
├──────────────────────────────────────────────────────────────────────┤
│  [Search...                ] [Status ▾] [Created ▾] [📋 Table|📅 Cal]│
├──┬────────────────┬──────────┬──────────┬─────────┬────────┬────────┤
│☐ │ Name           │ Status   │ Targets  │ Launch  │ Owner  │  ···   │
├──┼────────────────┼──────────┼──────────┼─────────┼────────┼────────┤
│☐ │ Q1 Exec Spear  │ ACTIVE   │ 142      │ Mar 15  │ jdoe   │  ···   │
│☐ │ IT Dept Recon  │ DRAFT    │ —        │ —       │ asmith │  ···   │
│☐ │ HR Benefits    │ BUILDING │ 89       │ Apr 01  │ jdoe   │  ···   │
│☐ │ Sales Q4       │ COMPLETED│ 230      │ Dec 01  │ mwong  │  ···   │
├──┴────────────────┴──────────┴──────────┴─────────┴────────┴────────┤
│  Showing 1–25 of 47                              [← 1  2  →]       │
└──────────────────────────────────────────────────────────────────────┘

                    ┌──── Floating Action Bar (on selection) ────┐
                    │  3 selected   [Archive] [Delete]  [✕]     │
                    └───────────────────────────────────────────┘
```

### 1.3 Table Columns

| Column | Width | Content | Sortable |
|--------|-------|---------|----------|
| Checkbox | 40px | Row selection checkbox | No |
| Name | flex | Campaign name, truncated with tooltip at 40 chars | Yes |
| Status | 120px | Status badge using `--status-*` colors (design system 1.5) | Yes |
| Targets | 80px | Count of assigned targets, or "—" if none | Yes |
| Launch Date | 100px | Scheduled launch date, or "—" if unset | Yes |
| Owner | 100px | Username of the campaign creator, truncated | Yes |
| Actions | 48px | Kebab menu (`···`) | No |

- Default sort: **Created date descending** (newest first).
- Clicking a row (outside the checkbox or kebab) navigates to that campaign's workspace.
- Row hover: `--bg-hover` background.
- Active campaign rows show a subtle left border in `--status-active` color (3px).

### 1.4 Search and Filters

**Search Bar:**
- Placeholder: "Search campaigns..."
- Searches against campaign name and description.
- Debounced at 300ms, minimum 2 characters.
- Sends `search` query parameter to `GET /api/v1/campaigns`.

**Status Filter (Multi-Select Dropdown):**
- Lists all 10 campaign statuses with their colored dots.
- Multiple statuses can be selected simultaneously.
- Selected statuses appear as removable chips below the search bar.
- Default: all statuses except Archived are shown.

**Date Filter (Dropdown):**
- Options: "Created date", "Launch date", "Completed date".
- When selected, a date range picker appears inline with From/To fields.
- Sends `created_after`, `created_before` (or corresponding date fields) to the API.

**Filter Persistence:**
- Active filters are reflected in URL query parameters.
- Navigating back to the list restores the last-used filters from the URL.

### 1.5 Kebab Menu Actions

The kebab menu on each row provides context-sensitive actions based on campaign state:

| Action | Available States | Behavior |
|--------|-----------------|----------|
| Open Workspace | All | Navigate to workspace (same as row click) |
| Duplicate | All except Building | Creates a new draft campaign copying all configuration |
| Submit for Approval | Draft, Rejected | Transitions to `pending_approval` |
| Approve | Pending Approval | Transitions to `approved` (requires `campaigns:approve` permission) |
| Reject | Pending Approval | Opens rejection reason modal, transitions to `rejected` |
| Build | Approved | Transitions to `building`, starts build process |
| Launch | Ready | Transitions to `active` |
| Pause | Active | Transitions to `paused` |
| Resume | Paused | Transitions to `active` |
| Complete | Active, Paused | Confirmation modal, transitions to `completed` |
| Archive | Completed | Transitions to `archived` |
| Delete | Draft, Rejected | Destructive confirmation modal, permanently deletes |

- Destructive actions (Delete) use `--danger` text color.
- Actions requiring confirmation show a confirmation modal with the campaign name and a description of the consequence.
- State transition actions that are not available for the current state are not rendered (they are absent, not disabled).

### 1.6 Bulk Operations

When one or more rows are selected via checkboxes:

- A **floating action bar** appears at the bottom center of the viewport, `--z-sticky`.
- The bar shows: selected count, available bulk actions, and a dismiss (✕) button.
- Available bulk actions depend on the intersection of states of all selected campaigns:
  - **Archive**: Available if all selected campaigns are in `completed` state.
  - **Delete**: Available if all selected campaigns are in `draft` or `rejected` state.
- If the selection spans incompatible states, no bulk actions are shown — only the count and dismiss.
- Bulk delete shows a confirmation modal listing all campaign names that will be deleted.
- The floating bar uses `--bg-tertiary` background, `--shadow-lg`, `--radius-lg`, and `--duration-smooth` slide-up animation.

### 1.7 Calendar View

Toggling to the calendar view replaces the table with a month-view calendar:

```
┌──────────────────────────────────────────────────────────────────────┐
│  Campaigns                                    [+ New Campaign]      │
├──────────────────────────────────────────────────────────────────────┤
│  [Search...                ] [Status ▾]              [📋 Table|📅 Cal]│
├──────────────────────────────────────────────────────────────────────┤
│                      ◀  March 2026  ▶                               │
│  Mon    Tue    Wed    Thu    Fri    Sat    Sun                       │
│ ┌──────┬──────┬──────┬──────┬──────┬──────┬──────┐                  │
│ │      │      │      │      │      │      │  1   │                  │
│ │      │      │      │      │      │      │      │                  │
│ ├──────┼──────┼──────┼──────┼──────┼──────┼──────┤                  │
│ │  2   │  3   │  4   │  5   │  6   │  7   │  8   │                  │
│ │      │      │      │      │      │      │      │                  │
│ ├──────┼──────┼──────┼──────┼──────┼──────┼──────┤                  │
│ │  9   │  10  │  11  │  12  │  13  │  14  │  15  │                  │
│ │      │      │      │██Q1█ │══════│══════│══════ │                  │
│ ├──────┼──────┼──────┼──────┼──────┼──────┼──────┤                  │
│ │══════│══════│══════│══════│══════│  21  │  22  │                  │
│ │      │      │      │      │      │      │      │                  │
│ └──────┴──────┴──────┴──────┴──────┴──────┴──────┘                  │
└──────────────────────────────────────────────────────────────────────┘
```

- Campaigns are displayed as horizontal bars spanning their start-to-end dates.
- Bar color corresponds to the campaign's status color.
- Clicking a campaign bar navigates to the workspace.
- Campaigns without dates are not shown on the calendar (only on the table view).
- Month navigation via left/right arrows. "Today" button jumps to the current month.
- View toggle state is persisted in `localStorage`.

### 1.8 Empty State

When no campaigns exist:

```
┌──────────────────────────────────────────────────────────────────────┐
│  Campaigns                                    [+ New Campaign]      │
├──────────────────────────────────────────────────────────────────────┤
│                                                                      │
│                      (Crosshair icon, xl size)                       │
│                                                                      │
│                    No campaigns yet                                  │
│             Create your first phishing campaign                      │
│             to start testing your organization.                      │
│                                                                      │
│                      [+ New Campaign]                                │
│                                                                      │
└──────────────────────────────────────────────────────────────────────┘
```

- The "New Campaign" button in the empty state is a Primary button.
- When filters are active but return no results, the message reads "No campaigns match your filters" with a "Clear filters" link.

### 1.9 New Campaign

Clicking "New Campaign" opens a **modal** with minimal required fields:

| Field | Type | Validation |
|-------|------|------------|
| Campaign Name | Text input | Required, 3–100 characters, unique |
| Description | Textarea | Optional, max 500 characters |

- On submit: `POST /api/v1/campaigns` with state `draft`.
- On success: navigate to the new campaign's workspace.
- On error: show inline validation errors in the modal.
- The modal uses `--duration-smooth` scale-up animation.

---

## 2. Campaign Workspace — Structure

### 2.1 Purpose

The Campaign Workspace is the full-page detail view for a single campaign. It serves as the project hub where operators configure targets, email templates, landing pages, infrastructure, and schedule across multiple sessions over days or weeks. Each tab saves independently — the campaign is gradually assembled, not filled out in a single sitting.

### 2.2 URL Structure

```
/campaigns/:id                → Overview tab (default)
/campaigns/:id/targets        → Targets tab
/campaigns/:id/email          → Email Templates tab
/campaigns/:id/landing-page   → Landing Page tab
/campaigns/:id/infrastructure → Infrastructure tab
/campaigns/:id/schedule       → Schedule tab
/campaigns/:id/approval       → Approval History tab
```

Each tab has its own URL, enabling deep linking and browser back/forward navigation.

### 2.3 Page Header

```
┌──────────────────────────────────────────────────────────────────────┐
│  ← Campaigns  /  Q1 Executive Spear Phish                          │
├──────────────────────────────────────────────────────────────────────┤
│                                                                      │
│  Q1 Executive Spear Phish                          [ACTIVE]         │
│  Test executive team resilience to credential       ┌─────────────┐ │
│  harvesting attacks using spoofed board comms.      │ Pause  ▾    │ │
│                                                     └─────────────┘ │
│  Created Mar 1 by jdoe  ·  Last edited 2h ago                      │
│                                                                      │
├──────────────────────────────────────────────────────────────────────┤
│  [Overview] [Targets] [Email Templates] [Landing Page]              │
│  [Infrastructure] [Schedule] [Approval History]                     │
└──────────────────────────────────────────────────────────────────────┘
```

**Elements:**
- **Breadcrumb**: "Campaigns / {Campaign Name}" — "Campaigns" is a link back to the list.
- **Title**: Campaign name in `--text-h1`. Editable inline (pencil icon on hover) when campaign is in `draft` or `rejected` state. Clicking the pencil converts the title to an input field. Pressing Enter or blurring saves. Pressing Escape reverts.
- **Status Badge**: Current state badge using `--status-*` colors, positioned top-right of the title row.
- **Description**: Campaign description in `--text-secondary`, below the title. Editable inline when in `draft` or `rejected` state.
- **Metadata Line**: Created date, creator, and last-edited relative timestamp. Uses `--text-muted`.
- **Primary Action Button**: A split button (primary action + dropdown chevron) positioned to the right. The primary action changes based on campaign state (see section 8).
- **Tab Bar**: Horizontal tabs below the header. Active tab uses `--accent-primary` underline (3px) with `--accent-primary` text. Inactive tabs use `--text-secondary`. Tabs transition underline position with `--duration-normal`.

### 2.4 Read-Only Mode

When a campaign is in an **active, paused, building, completed, or archived** state, all configuration tabs (Targets, Email Templates, Landing Page, Infrastructure, Schedule) switch to read-only presentation:

- All form inputs are replaced with static text displays.
- Selection controls (dropdowns, checkboxes) show their selected values as text.
- "Edit" buttons, "Add" buttons, and "Remove" icons are hidden.
- The tab content uses the same layout as the editable version for visual consistency, but inputs are rendered as text on a transparent background.
- A subtle banner appears below the tab bar: "Configuration is locked while the campaign is {state}." in `--text-secondary` with an info icon.
- The Overview tab is **never** read-only — it always shows live data and remains interactive for its display controls (chart toggles, filters, etc.).

### 2.5 Tab Saving Behavior

Each tab saves independently. The saving pattern varies by tab:

| Tab | Save Trigger | API Call |
|-----|-------------|----------|
| Overview | No save — display only | N/A |
| Targets | Explicit "Save" button | `PUT /api/v1/campaigns/:id` (target_group_ids) |
| Email Templates | Explicit "Save" button | `PUT /api/v1/campaigns/:id` (template config) |
| Landing Page | Explicit "Save" button | `PUT /api/v1/campaigns/:id` (landing_page_id) |
| Infrastructure | Explicit "Save" button | `PUT /api/v1/campaigns/:id` (infra config) |
| Schedule | Explicit "Save" button | `PUT /api/v1/campaigns/:id` (schedule config) |

- Each tab's Save button is positioned at the bottom-right of the tab content area.
- Save button states: default "Save", loading "Saving..." (spinner, disabled), success "Saved" (checkmark, auto-reverts to "Save" after 2 seconds).
- The Save button is only enabled when the tab's form data differs from the last-saved state (dirty detection).
- If the user navigates away from a tab with unsaved changes, a confirmation modal appears: "You have unsaved changes on this tab. Discard changes?" with "Stay" (secondary) and "Discard" (danger) buttons.
- Save errors show a toast notification with the error message and the Save button reverts to its default state.

---

## 3. Overview Tab

### 3.1 Purpose

The Overview tab is the campaign's command center. For draft campaigns, it shows readiness status across all configuration sections. For active/completed campaigns, it shows real-time metrics and the state transition timeline.

### 3.2 Layout — Draft/Configuration Phase

```
┌──────────────────────────────────────────────────────────────────────┐
│  READINESS                                                          │
│                                                                      │
│  ┌──────────────┐ ┌──────────────┐ ┌──────────────┐ ┌─────────────┐│
│  │ ● Targets    │ │ ○ Email      │ │ ● Landing    │ │ ○ Infra     ││
│  │              │ │   Templates  │ │   Page       │ │              ││
│  │  3 groups    │ │  No template │ │  Selected    │ │  Not         ││
│  │  142 targets │ │  configured  │ │  "O365 Login"│ │  configured  ││
│  │              │ │              │ │              │ │              ││
│  │  [View →]    │ │  [Configure] │ │  [View →]    │ │  [Configure] ││
│  └──────────────┘ └──────────────┘ └──────────────┘ └─────────────┘│
│                                                                      │
│  ┌──────────────┐ ┌──────────────┐                                  │
│  │ ○ Schedule   │ │ ● Approval   │                                  │
│  │              │ │              │                                  │
│  │  No dates    │ │  Not         │                                  │
│  │  set         │ │  required    │                                  │
│  │              │ │  yet         │                                  │
│  │  [Configure] │ │  [—]         │                                  │
│  └──────────────┘ └──────────────┘                                  │
│                                                                      │
├──────────────────────────────────────────────────────────────────────┤
│  CAMPAIGN SUMMARY                                                    │
│                                                                      │
│  Name:           Q1 Executive Spear Phish                           │
│  Description:    Test executive team resilience...                   │
│  State:          Draft                                               │
│  Created:        March 1, 2026 by jdoe                              │
│  Last Modified:  March 3, 2026                                      │
│                                                                      │
└──────────────────────────────────────────────────────────────────────┘
```

### 3.3 Readiness Cards

Six readiness cards are displayed in a responsive grid (3 columns on desktop, 2 on tablet, 1 on mobile). Each card represents a required configuration section.

**Card Structure:**
- **Status Indicator**: Filled circle (●) in `--success` when configured/ready; hollow circle (○) in `--text-muted` when incomplete.
- **Section Name**: Card title in `--text-h4`.
- **Summary**: 1–2 lines describing current state of that section in `--text-secondary`.
- **Action Link**: "View →" if configured (navigates to that tab), "Configure" if incomplete (navigates to that tab). Uses `--accent-primary` text.
- Card background: `--bg-secondary`. Border: `--border-default`. Incomplete cards have a dashed border style instead of solid.

**Readiness Rules:**

| Card | Ready When |
|------|-----------|
| Targets | At least one target group is assigned |
| Email Templates | At least one email template variant is configured |
| Landing Page | A landing page is selected |
| Infrastructure | Cloud provider, region, instance type, and endpoint domain are all set |
| Schedule | Start date is set |
| Approval | Not a readiness gate — shows "Not required yet" in draft, or approval status when applicable |

**Overall Readiness:**
- Below the readiness cards, a summary line states: "4 of 5 sections ready" (excluding Approval from the count) with a progress bar in `--accent-primary`.
- When all 5 operational sections are ready, the line changes to "All sections ready — this campaign can be submitted for approval" in `--success` color.

### 3.4 Layout — Active/Completed Phase

When the campaign is in `active`, `paused`, `completed`, or `archived` state, the Overview tab transforms to show metrics:

```
┌──────────────────────────────────────────────────────────────────────┐
│  CAMPAIGN METRICS                                    [7d ▾] [Export]│
│                                                                      │
│  ┌──────────────┐ ┌──────────────┐ ┌──────────────┐ ┌─────────────┐│
│  │  Emails Sent │ │  Opened      │ │  Clicked     │ │  Credentials││
│  │     142      │ │     89       │ │     34       │ │     12      ││
│  │              │ │    62.7%     │ │    23.9%     │ │     8.5%    ││
│  └──────────────┘ └──────────────┘ └──────────────┘ └─────────────┘│
│                                                                      │
│  ┌───────────────────────────────────────────────────────────────┐  │
│  │  Engagement Over Time (line chart)                            │  │
│  │                                                               │  │
│  │  ───── Opens ───── Clicks ───── Captures                     │  │
│  │  ═══════════════════════════════════════════                   │  │
│  │                                                               │  │
│  └───────────────────────────────────────────────────────────────┘  │
│                                                                      │
│  ┌──────────────────────────────┐ ┌──────────────────────────────┐  │
│  │  A/B Variant Performance    │ │  Funnel                      │  │
│  │                              │ │                              │  │
│  │  Variant A  ████████ 28%    │ │  Sent     ████████████ 142   │  │
│  │  Variant B  ██████████ 35%  │ │  Opened   ████████     89   │  │
│  │                              │ │  Clicked  ████           34   │  │
│  │  (click-through rate)       │ │  Captured ██            12   │  │
│  └──────────────────────────────┘ └──────────────────────────────┘  │
│                                                                      │
├──────────────────────────────────────────────────────────────────────┤
│  STATE TIMELINE                                                      │
│                                                                      │
│  ●──────●──────●──────●──────●──────●                               │
│  Draft  Pending Approved Building Ready  Active                     │
│  Mar 1  Mar 5   Mar 6   Mar 6   Mar 6  Mar 7                       │
│                                                                      │
├──────────────────────────────────────────────────────────────────────┤
│  RECENT CREDENTIAL CAPTURES                          [View All →]   │
│                                                                      │
│  ┌──────┬──────────────┬─────────────┬────────┬──────────────────┐  │
│  │ Time │ Target       │ Variant     │ Field  │ Value            │  │
│  ├──────┼──────────────┼─────────────┼────────┼──────────────────┤  │
│  │ 2m   │ j.smith@...  │ Variant A   │ pass   │ ●●●●●● [Reveal] │  │
│  │ 8m   │ m.jones@...  │ Variant B   │ pass   │ ●●●●●● [Reveal] │  │
│  │ 1h   │ k.lee@...    │ Variant A   │ pass   │ ●●●●●● [Reveal] │  │
│  └──────┴──────────────┴─────────────┴────────┴──────────────────┘  │
│                                                                      │
└──────────────────────────────────────────────────────────────────────┘
```

### 3.5 Metric Stat Cards

Four stat cards in a 4-column grid (same pattern as the dashboard stat cards):

| Card | Value | Subvalue |
|------|-------|----------|
| Emails Sent | Total emails dispatched | Count only |
| Opened | Count of unique opens | Percentage of sent |
| Clicked | Count of unique link clicks | Percentage of sent |
| Credentials | Count of credential submissions | Percentage of sent |

- Values update in real-time via WebSocket when the campaign is active.
- Use `font-variant-numeric: tabular-nums` to prevent layout shift on counter updates.
- Source: `GET /api/v1/campaigns/:id/metrics`.

### 3.6 Engagement Over Time Chart

- Multi-line chart with three series: Opens (blue), Clicks (teal), Captures (green).
- X-axis: time since campaign launch (hourly for campaigns under 7 days, daily otherwise).
- Y-axis: cumulative count.
- Date range selector: "All time", "7 days", "24 hours", "1 hour".
- Tooltip on hover shows exact counts for all three series at that time point.
- For active campaigns, the chart updates in real-time — the rightmost data point extends as new events arrive.

### 3.7 A/B Variant Performance

- Horizontal bar chart comparing click-through rates across configured A/B variants.
- Each bar shows the variant label, bar, and percentage.
- Bar color: `--accent-primary` for all variants, with the winning variant highlighted slightly brighter.
- Only displayed if the campaign has more than one email template variant.
- If only one variant exists, this chart is replaced by a simple "Single template — no A/B comparison" note.

### 3.8 Engagement Funnel

- Horizontal funnel chart showing progression: Sent → Opened → Clicked → Captured.
- Each bar shrinks proportionally to represent drop-off.
- Bars use a gradient from `--accent-primary` (top) to `--status-active` (bottom).
- Each row shows the label, bar, and count.

### 3.9 State Transition Timeline

A horizontal timeline showing every state transition the campaign has undergone:

- Each state is a node (filled circle) on a horizontal line.
- The current state's node is larger and uses the corresponding `--status-*` color.
- Past states use `--text-muted` color.
- Below each node: state name and timestamp (date, or relative time if within 24 hours).
- If a rejection occurred, the rejection node is shown in `--status-rejected` with a branch down showing the rejection reason on hover (tooltip).
- The timeline scrolls horizontally if it exceeds the container width (campaigns with many state changes).

### 3.10 Credential Capture Feed

- A compact table showing the most recent 10 credential captures.
- Columns: Relative time, Target email (truncated), Variant, Field name, Masked value.
- **Masked-with-reveal pattern**: Credential values are displayed as `●●●●●●` with a "Reveal" button.
  - Clicking "Reveal" calls the server-side decryption endpoint and displays the plaintext value.
  - The reveal action creates an entry in the audit log (server-side).
  - After 30 seconds, the value automatically re-masks with a fade-out animation (`--duration-normal`).
  - While revealed, the button text changes to "Hide" for manual re-masking.
  - Only users with `credentials:decrypt` permission see the Reveal button. Others see only the masked dots.
- "View All" link navigates to the campaign's metrics page with the credential capture tab pre-selected.
- For active campaigns, new captures prepend to the table in real-time via WebSocket with a subtle slide-in animation.

### 3.11 Campaign Summary Section

Displayed below the readiness cards (draft phase) or below the timeline (active/completed phase):

- Shows: Name, Description, State, Created date + creator, Last modified date.
- In draft/rejected states, the name and description are inline-editable (pencil icon on hover).
- In all other states, values are static text.

---

## 4. Targets Tab

### 4.1 Purpose

Configure which target groups will receive the phishing emails in this campaign. Targets are organized into reusable groups managed elsewhere (Targets section of the app); this tab handles assignment to the campaign.

### 4.2 Layout

```
┌──────────────────────────────────────────────────────────────────────┐
│  TARGET GROUPS                                    [+ Add Group]     │
│                                                                      │
│  ┌────────────────────────────────────────────────────────────────┐  │
│  │  ┌──────────────────────────────────────────────────────────┐ │  │
│  │  │ ✕  Executive Team              42 targets    Added Mar 1 │ │  │
│  │  └──────────────────────────────────────────────────────────┘ │  │
│  │  ┌──────────────────────────────────────────────────────────┐ │  │
│  │  │ ✕  Marketing Department        67 targets    Added Mar 2 │ │  │
│  │  └──────────────────────────────────────────────────────────┘ │  │
│  │  ┌──────────────────────────────────────────────────────────┐ │  │
│  │  │ ✕  IT Support Staff            33 targets    Added Mar 3 │ │  │
│  │  └──────────────────────────────────────────────────────────┘ │  │
│  └────────────────────────────────────────────────────────────────┘  │
│                                                                      │
│  Total: 142 unique targets (0 duplicates removed)                   │
│                                                                      │
├──────────────────────────────────────────────────────────────────────┤
│  BLOCKLIST CHECK                                                     │
│                                                                      │
│  ⚠ 3 targets match blocklist entries:                               │
│    • ceo@example.com — "CEO exclusion per policy" (added Jan 15)    │
│    • cfo@example.com — "CFO exclusion per policy" (added Jan 15)    │
│    • vp.legal@example.com — "Legal exclusion" (added Feb 10)        │
│                                                                      │
│  Blocklist matches do not prevent the campaign from proceeding.     │
│  An additional Administrator approval will be required.             │
│                                                                      │
├──────────────────────────────────────────────────────────────────────┤
│  CANARY TARGETS                                   [+ Add Canary]    │
│                                                                      │
│  Canary targets receive the phishing email but are controlled       │
│  accounts used to verify delivery and rendering.                    │
│                                                                      │
│  • canary1@internal.example.com                              [✕]    │
│  • canary2@internal.example.com                              [✕]    │
│                                                                      │
├──────────────────────────────────────────────────────────────────────┤
│                                                         [Save]      │
└──────────────────────────────────────────────────────────────────────┘
```

### 4.3 Target Group Selection

**Add Group Button:**
- Opens a slide-over panel from the right listing all available target groups.
- The slide-over includes a search bar to filter groups by name.
- Each group row shows: group name, target count, and an "Add" button.
- Groups already assigned to this campaign show "Added" (disabled) instead of the "Add" button.
- Adding a group closes the slide-over and adds the group card to the list with a slide-in animation.

**Group Cards:**
- Each assigned group is displayed as a card showing: group name, target count, date added.
- A remove button (✕) on the right removes the group from the assignment (not from the system).
- Clicking the group name opens the group's detail in a new browser tab (target groups are managed independently).

**Deduplication:**
- The system calculates unique targets across all selected groups.
- The summary line shows: "Total: {unique} unique targets ({duplicates} duplicates removed)".
- Deduplication count is fetched from the API when groups change: `GET /api/v1/campaigns/:id/target-preview?group_ids=1,2,3`.

### 4.4 Blocklist Check

- When target groups are assigned, the system checks for blocklist matches via `POST /api/v1/campaigns/:id/blocklist-check`.
- If matches are found, a warning section appears with `--warning-muted` background and `--warning` left border (3px).
- Each matched target is listed with their email and the blocklist entry reason.
- An explanatory note clarifies: blocklist matches do **not** block the campaign. They trigger an additional Administrator approval requirement when the campaign is submitted.
- If no blocklist matches exist, this section is hidden entirely.

### 4.5 Canary Targets

- Canary targets are individual email addresses (not groups) added directly.
- "Add Canary" opens a small inline form with an email input and "Add" button.
- Canary emails are validated for format before adding.
- Each canary target has a remove button (✕).
- Canary targets are sent the phishing email alongside real targets but are excluded from metrics calculations.

### 4.6 Read-Only State

When the campaign is not in a configurable state:
- Group cards show without the ✕ remove button.
- "Add Group" and "Add Canary" buttons are hidden.
- Canary target remove buttons are hidden.
- The Save button is hidden.
- The blocklist section, if present, is still displayed for informational reference.

---

## 5. Email Templates Tab

### 5.1 Purpose

Configure one or more email template variants for the campaign. The campaign supports A/B testing by assigning multiple templates with percentage-based traffic splits.

### 5.2 Layout

```
┌──────────────────────────────────────────────────────────────────────┐
│  EMAIL TEMPLATE VARIANTS                         [+ Add Variant]    │
│                                                                      │
│  ┌────────────────────────────────────────────────────────────────┐  │
│  │  VARIANT A                                     50%    [✕]     │  │
│  │                                                                │  │
│  │  Template: [Select template...              ▾]                 │  │
│  │            or [Create New Template →]                          │  │
│  │                                                                │  │
│  │  ┌─ Preview ────────────────────────────────────────────────┐  │  │
│  │  │ From: hr-benefits@company-updates.com                    │  │  │
│  │  │ Subject: Action Required: Update Your Benefits Selection │  │  │
│  │  │                                                          │  │  │
│  │  │ Dear {{first_name}},                                     │  │  │
│  │  │                                                          │  │  │
│  │  │ Your benefits enrollment window closes on...             │  │  │
│  │  └──────────────────────────────────────────────────────────┘  │  │
│  └────────────────────────────────────────────────────────────────┘  │
│                                                                      │
│  ┌────────────────────────────────────────────────────────────────┐  │
│  │  VARIANT B                                     50%    [✕]     │  │
│  │                                                                │  │
│  │  Template: [Quarterly Review Notification    ▾]                │  │
│  │                                                                │  │
│  │  ┌─ Preview ────────────────────────────────────────────────┐  │  │
│  │  │ From: reviews@hr-portal.com                              │  │  │
│  │  │ Subject: Your Q1 Performance Review is Ready             │  │  │
│  │  │                                                          │  │  │
│  │  │ Hi {{first_name}},                                       │  │  │
│  │  │                                                          │  │  │
│  │  │ Your manager has completed your quarterly review...      │  │  │
│  │  └──────────────────────────────────────────────────────────┘  │  │
│  └────────────────────────────────────────────────────────────────┘  │
│                                                                      │
│  Traffic Split:  Variant A [====50%====|====50%====] Variant B      │
│  (drag handle to adjust, or edit percentages directly)              │
│                                                                      │
├──────────────────────────────────────────────────────────────────────┤
│  SMTP PROFILE                                                        │
│                                                                      │
│  Profile: [Company SMTP Relay ▾]                                    │
│  Sending address will be taken from the selected email template.    │
│                                                                      │
├──────────────────────────────────────────────────────────────────────┤
│                                                         [Save]      │
└──────────────────────────────────────────────────────────────────────┘
```

### 5.3 Variant Management

**Add Variant:**
- Maximum 5 variants per campaign.
- When adding a new variant, traffic percentages are automatically rebalanced equally (e.g., adding a third variant sets all to 33/33/34).
- The new variant card appears with an empty template selector.

**Remove Variant (✕):**
- Requires at least 1 variant to remain — the remove button is hidden when only one variant exists.
- Removing a variant triggers automatic rebalancing of remaining variants.
- A confirmation tooltip appears on click: "Remove this variant?" with "Remove" and "Cancel" buttons.

**Traffic Split:**
- Displayed as a segmented bar below all variant cards.
- Each segment is labeled with the variant letter and percentage.
- Percentages can be edited by:
  1. Clicking a percentage value to type a number directly.
  2. Dragging the divider between segments.
- Percentages must sum to 100%. Adjusting one variant automatically adjusts the last variant to compensate.
- Minimum 5% per variant.

### 5.4 Template Selection

Each variant has a template selector dropdown:

- **Dropdown**: Lists all email templates from `GET /api/v1/email-templates`. Shows template name and a truncated subject line.
- **"Create New Template" link**: Opens the email template editor in a **new browser tab** (`/email-templates/new`). When the user returns to this tab, a "Refresh templates" link appears below the dropdown to reload the template list.
- When a template is selected, a read-only preview renders below the dropdown showing: From address, Subject line, and a truncated body preview (first 200 characters of the plaintext version).
- Clicking the preview area opens a full preview modal showing the complete rendered email (HTML version) at approximate email client dimensions (600px wide).

### 5.5 SMTP Profile Selection

- A dropdown listing all SMTP profiles from `GET /api/v1/smtp-profiles`.
- Each option shows: profile name and server address.
- One SMTP profile is selected per campaign (shared across all variants).
- The sending address (From field) comes from each variant's selected email template, not the SMTP profile.

### 5.6 Read-Only State

- Template dropdowns are replaced with static text showing the selected template name (clickable to view template detail in new tab).
- Traffic split bar is displayed without drag handles or editable percentages.
- SMTP profile dropdown is replaced with static text.
- "Add Variant" and remove (✕) buttons are hidden.
- Save button is hidden.

---

## 6. Landing Page Tab

### 6.1 Purpose

Select or create the credential harvesting landing page that targets will see when they click the phishing link.

### 6.2 Layout

```
┌──────────────────────────────────────────────────────────────────────┐
│  LANDING PAGE                                                        │
│                                                                      │
│  ┌────────────────────────────────────────────────────────────────┐  │
│  │  Landing Page: [Select landing page...           ▾]            │  │
│  │                or [Create New Landing Page →]                   │  │
│  └────────────────────────────────────────────────────────────────┘  │
│                                                                      │
│  ┌─ Preview ──────────────────────────────────────────────────────┐  │
│  │                                                                │  │
│  │  ┌──────────────────────────────────────────────────────────┐  │  │
│  │  │                                                          │  │  │
│  │  │         (Rendered landing page preview)                  │  │  │
│  │  │         Scaled-down iframe or screenshot                 │  │  │
│  │  │         of the selected landing page                     │  │  │
│  │  │                                                          │  │  │
│  │  │  ┌─────────────┐  ┌──────────────────┐                  │  │  │
│  │  │  │ Email       │  │                  │                  │  │  │
│  │  │  └─────────────┘  │  Password        │                  │  │  │
│  │  │                    └──────────────────┘                  │  │  │
│  │  │                    [Sign In]                             │  │  │
│  │  │                                                          │  │  │
│  │  └──────────────────────────────────────────────────────────┘  │  │
│  │                                                                │  │
│  │  Page: O365 Login Clone                                        │  │
│  │  Type: Credential Capture                                      │  │
│  │  Fields: email, password                                       │  │
│  │  Redirect: https://real-login.example.com (after capture)     │  │
│  │                                                                │  │
│  │  [Open Full Preview →]              [Edit Landing Page →]     │  │
│  └────────────────────────────────────────────────────────────────┘  │
│                                                                      │
├──────────────────────────────────────────────────────────────────────┤
│                                                         [Save]      │
└──────────────────────────────────────────────────────────────────────┘
```

### 6.3 Landing Page Selection

- **Dropdown**: Lists all landing pages from `GET /api/v1/landing-pages`. Shows page name and type.
- **"Create New Landing Page" link**: Opens the landing page editor in a **new browser tab** (`/landing-pages/new`). A "Refresh pages" link appears on return.
- One landing page per campaign (shared across all email template variants).

### 6.4 Preview

When a landing page is selected:

- A scaled-down visual preview renders below the dropdown. This is a sandboxed iframe at 50% scale, or a server-rendered screenshot if iframe rendering is not feasible.
- Below the preview, metadata is displayed: page name, type (Credential Capture, Click-Only, Awareness), captured fields, and post-capture redirect URL.
- "Open Full Preview" opens a modal showing the landing page at full size in an iframe.
- "Edit Landing Page" opens the landing page editor in a new browser tab for the selected page.

### 6.5 Read-Only State

- Dropdown is replaced with static text showing the selected landing page name.
- Preview remains visible.
- "Edit Landing Page" link is hidden.
- Save button is hidden.

---

## 7. Infrastructure Tab

### 7.1 Purpose

Configure the cloud infrastructure that will host the phishing landing page for this campaign. Infrastructure is provisioned per-campaign during the build phase and terminated after campaign completion. Endpoints are never reused across campaigns.

### 7.2 Layout

```
┌──────────────────────────────────────────────────────────────────────┐
│  CLOUD INFRASTRUCTURE                                                │
│                                                                      │
│  ┌────────────────────────────────────────────────────────────────┐  │
│  │  Cloud Provider                                                │  │
│  │  [AWS ▾]                                                       │  │
│  │                                                                │  │
│  │  Region                                                        │  │
│  │  [us-east-1 (N. Virginia) ▾]                                   │  │
│  │                                                                │  │
│  │  Instance Type                                                 │  │
│  │  [t3.micro ▾]                                                  │  │
│  │  Recommended for campaigns under 500 targets.                  │  │
│  └────────────────────────────────────────────────────────────────┘  │
│                                                                      │
├──────────────────────────────────────────────────────────────────────┤
│  ENDPOINT CONFIGURATION                                              │
│                                                                      │
│  ┌────────────────────────────────────────────────────────────────┐  │
│  │  Endpoint Domain                                               │  │
│  │  [login-portal.example-corp.com        ]                       │  │
│  │  Domain must be pre-registered in the Domains section.         │  │
│  │                                                                │  │
│  │  TLS Certificate                                               │  │
│  │  Auto-provisioned via Let's Encrypt during build.              │  │
│  └────────────────────────────────────────────────────────────────┘  │
│                                                                      │
├──────────────────────────────────────────────────────────────────────┤
│  ENDPOINT STATUS                    (visible after build)            │
│                                                                      │
│  ┌────────────────────────────────────────────────────────────────┐  │
│  │  Status:      ● Online                                         │  │
│  │  IP Address:  54.23.101.42                                     │  │
│  │  Domain:      login-portal.example-corp.com                    │  │
│  │  TLS:         Valid (expires Jun 15, 2026)                     │  │
│  │  Provisioned: Mar 6, 2026 14:23 UTC                           │  │
│  │  Health:      Last check 30s ago — 200 OK (142ms)             │  │
│  └────────────────────────────────────────────────────────────────┘  │
│                                                                      │
├──────────────────────────────────────────────────────────────────────┤
│                                                         [Save]      │
└──────────────────────────────────────────────────────────────────────┘
```

### 7.3 Cloud Configuration Fields

| Field | Type | Options | Validation |
|-------|------|---------|------------|
| Cloud Provider | Dropdown | AWS, GCP, Azure, DigitalOcean | Required |
| Region | Dropdown | Dynamic based on provider selection | Required |
| Instance Type | Dropdown | Dynamic based on provider selection | Required |

- Region and Instance Type dropdowns are populated from `GET /api/v1/cloud-providers/:provider/regions` and `GET /api/v1/cloud-providers/:provider/instance-types`.
- Changing the Cloud Provider resets Region and Instance Type to empty.
- A helper text line below Instance Type shows a recommendation based on target count (if targets are already configured): "Recommended for campaigns under {n} targets."

### 7.4 Endpoint Configuration

**Endpoint Domain:**
- A text input for the domain name.
- Autocomplete suggestions from `GET /api/v1/domains` (pre-registered domains).
- If the entered domain is not in the registered domains list, a warning appears: "This domain is not registered in Tackle. Add it in the Domains section first." with a link to the Domains page.
- Domain validation: must be a valid FQDN format.

**TLS Certificate:**
- Static informational text: "Auto-provisioned via Let's Encrypt during the build process."
- No user input required — TLS is always automatic.

### 7.5 Endpoint Status (Post-Build)

Once the campaign has been built (state is `ready`, `active`, `paused`, `completed`), an "Endpoint Status" section appears showing:

- **Status**: Online/Degraded/Offline with a colored dot using infrastructure status colors (design system 1.6).
- **IP Address**: Displayed in monospace font (`--text-code`).
- **Domain**: The configured domain.
- **TLS**: Certificate validity and expiration date.
- **Provisioned**: Timestamp of when the endpoint was created.
- **Health**: Result of the most recent health check (HTTP status code, response time).

For active campaigns, the endpoint status updates in real-time via WebSocket. The health check timestamp refreshes automatically.

For `completed` and `archived` campaigns, the status shows "Terminated" with the termination timestamp. The IP address and health check are no longer relevant and are replaced with: "Infrastructure terminated on {date}."

### 7.6 Read-Only State

- Cloud provider, region, instance type, and domain inputs are replaced with static text.
- Endpoint Status section (if visible) remains interactive — it's a live monitoring display, not a configuration.
- Save button is hidden.

---

## 8. Schedule Tab

### 8.1 Purpose

Configure when the campaign runs: launch timing, send windows, email pacing, and campaign end date.

### 8.2 Layout

```
┌──────────────────────────────────────────────────────────────────────┐
│  LAUNCH                                                              │
│                                                                      │
│  ┌────────────────────────────────────────────────────────────────┐  │
│  │  Scheduled Launch                                              │  │
│  │  [○ Launch manually]                                           │  │
│  │  [● Schedule for specific date/time]                           │  │
│  │     Date: [2026-04-01    ]  Time: [09:00    ]  TZ: [UTC ▾]    │  │
│  │                                                                │  │
│  │  Grace Period (minutes before first email)                     │  │
│  │  [15        ]                                                  │  │
│  │  Allows canary targets to verify delivery before main send.   │  │
│  └────────────────────────────────────────────────────────────────┘  │
│                                                                      │
├──────────────────────────────────────────────────────────────────────┤
│  SEND WINDOWS                                                        │
│                                                                      │
│  ┌────────────────────────────────────────────────────────────────┐  │
│  │  Emails will only be sent during these windows:                │  │
│  │                                                                │  │
│  │  ┌─ Window 1 ──────────────────────────────────────────┐      │  │
│  │  │  Days: [☑ Mon] [☑ Tue] [☑ Wed] [☑ Thu] [☑ Fri]    │      │  │
│  │  │        [☐ Sat] [☐ Sun]                              │      │  │
│  │  │  Hours: [08:00] to [17:00]  TZ: [Target Local ▾]   │      │  │
│  │  └────────────────────────────────────────────────────────┘      │  │
│  │                                                                │  │
│  │  [+ Add Send Window]                                           │  │
│  └────────────────────────────────────────────────────────────────┘  │
│                                                                      │
├──────────────────────────────────────────────────────────────────────┤
│  EMAIL PACING                                                        │
│                                                                      │
│  ┌────────────────────────────────────────────────────────────────┐  │
│  │  Send Order                                                    │  │
│  │  [● Random] [○ Alphabetical] [○ Group-sequential]              │  │
│  │                                                                │  │
│  │  Throttle Rate (emails per minute)                             │  │
│  │  [10        ]                                                  │  │
│  │                                                                │  │
│  │  Inter-Email Delay (seconds)                                   │  │
│  │  Min: [3     ]   Max: [8     ]                                 │  │
│  │  Random delay between min and max is applied between sends.   │  │
│  └────────────────────────────────────────────────────────────────┘  │
│                                                                      │
├──────────────────────────────────────────────────────────────────────┤
│  CAMPAIGN DURATION                                                   │
│                                                                      │
│  ┌────────────────────────────────────────────────────────────────┐  │
│  │  Start Date: [2026-04-01    ]                                  │  │
│  │  End Date:   [2026-04-15    ]                                  │  │
│  │                                                                │  │
│  │  The campaign will automatically complete on the end date.    │  │
│  │  Duration: 14 days                                             │  │
│  └────────────────────────────────────────────────────────────────┘  │
│                                                                      │
├──────────────────────────────────────────────────────────────────────┤
│                                                         [Save]      │
└──────────────────────────────────────────────────────────────────────┘
```

### 8.3 Launch Configuration

**Launch Mode (Radio):**
- **Manual**: The campaign will be launched by clicking the "Launch" button when in `ready` state. No scheduled time.
- **Scheduled**: A date/time picker appears. The campaign will transition from `ready` to `active` automatically at the specified time. Date must be in the future at the time of save. Time zone selector defaults to UTC.

**Grace Period:**
- Numeric input, value in minutes. Default: 0.
- When set, canary targets receive their emails first, followed by a pause of the grace period duration before the main send begins.
- Helper text explains the purpose.

### 8.4 Send Windows

Send windows define when emails are allowed to be dispatched. Outside these windows, sending pauses and resumes when the next window opens.

**Window Configuration:**
- Day-of-week checkboxes (Mon–Sun). Weekdays pre-checked by default.
- Start time and end time inputs (24-hour format).
- Timezone selector: "UTC" or "Target Local" (sends each email during the window relative to that target's timezone).
- Multiple windows can be added (e.g., different hours for different days).
- Each window can be removed with a ✕ icon (minimum 1 window required).

### 8.5 Email Pacing

**Send Order (Radio):**
- **Random**: Targets are shuffled randomly (default).
- **Alphabetical**: Targets are sorted by email address.
- **Group-sequential**: All targets in the first group are sent before the second group, etc.

**Throttle Rate:**
- Numeric input: maximum emails per minute. Default: 10. Range: 1–100.
- Helper text: "Higher rates may trigger spam filters. Recommended: 5–15 per minute."

**Inter-Email Delay:**
- Two numeric inputs: minimum and maximum delay in seconds.
- A random delay in this range is applied between each individual email send.
- Default: min 2, max 5. Min must be less than or equal to max.

### 8.6 Campaign Duration

- **Start Date**: Date picker. This is the earliest date the campaign can be active. Used in calendar view and reporting.
- **End Date**: Date picker. The campaign will automatically transition from `active` to `completed` at the end of this date (23:59:59 in the campaign's timezone).
- A calculated "Duration: {n} days" line appears between the fields.
- End date must be after start date.

### 8.7 Read-Only State

- All inputs are replaced with static text.
- Send window day checkboxes show as a comma-separated list (e.g., "Mon, Tue, Wed, Thu, Fri").
- Radio buttons show the selected option as text.
- Save button is hidden.

---

## 9. Approval History Tab

### 9.1 Purpose

Shows the complete approval history for the campaign — all submissions, reviews, approvals, rejections, and escalation events.

### 9.2 Layout

```
┌──────────────────────────────────────────────────────────────────────┐
│  APPROVAL HISTORY                                                    │
│                                                                      │
│  ┌────────────────────────────────────────────────────────────────┐  │
│  │  ● Mar 6, 14:00 — Submitted for approval by jdoe             │  │
│  │    Submission note: "Ready for Q1 exec test. All configs      │  │
│  │    verified against the playbook."                             │  │
│  │                                                                │  │
│  │  ⚠ Mar 6, 14:00 — Blocklist escalation triggered             │  │
│  │    3 targets match blocklist entries. Administrator            │  │
│  │    approval required in addition to standard review.          │  │
│  │    Matched: ceo@example.com, cfo@example.com,                 │  │
│  │    vp.legal@example.com                                        │  │
│  │                                                                │  │
│  │  ✕ Mar 6, 16:30 — Rejected by admin1                         │  │
│  │    Reason: "Landing page redirect URL is pointing to staging. │  │
│  │    Update to production URL and resubmit."                     │  │
│  │                                                                │  │
│  │  ● Mar 7, 09:15 — Resubmitted by jdoe                        │  │
│  │    Note: "Fixed redirect URL. No other changes."              │  │
│  │                                                                │  │
│  │  ✓ Mar 7, 11:00 — Approved by admin1                         │  │
│  │    Note: "Looks good. Approved including blocklist targets."  │  │
│  │                                                                │  │
│  │  ✓ Mar 7, 11:05 — Administrator approval by superadmin       │  │
│  │    Note: "CEO/CFO targeting confirmed with board."            │  │
│  │    (Required due to blocklist escalation)                     │  │
│  └────────────────────────────────────────────────────────────────┘  │
│                                                                      │
└──────────────────────────────────────────────────────────────────────┘
```

### 9.3 Timeline Entries

Each entry in the approval history is displayed as a timeline node:

| Event Type | Icon | Color |
|-----------|------|-------|
| Submitted | ● (filled circle) | `--accent-primary` |
| Blocklist Escalation | ⚠ (warning) | `--warning` |
| Approved | ✓ (checkmark) | `--success` |
| Rejected | ✕ (cross) | `--danger` |
| Resubmitted | ● (filled circle) | `--accent-primary` |
| Admin Override Approval | ✓ (checkmark) | `--success` |

**Entry Structure:**
- Timestamp (absolute date/time), event type description, actor username.
- Optional note/reason text displayed indented below the event line in `--text-secondary`.
- A vertical line connects consecutive timeline entries on the left side.

### 9.4 Tab Visibility

- This tab is always visible in the tab bar regardless of campaign state.
- For draft campaigns that have never been submitted, the tab shows: "No approval history. This campaign has not been submitted for review."
- The tab is purely informational — it has no editable fields and no Save button in any state.

---

## 10. Campaign Lifecycle Actions

### 10.1 Primary Action Button

The workspace header contains a split button whose primary action changes based on campaign state. The dropdown chevron reveals additional actions.

| State | Primary Action | Dropdown Actions |
|-------|---------------|-----------------|
| Draft | Submit for Approval | Duplicate, Delete |
| Rejected | Resubmit for Approval | Duplicate, Delete |
| Pending Approval | — (no primary action for submitter) | Withdraw Submission |
| Pending Approval (reviewer) | Approve | Reject |
| Approved | Build | — |
| Building | — (disabled, shows "Building...") | — |
| Ready | Launch | — |
| Active | Pause | Complete |
| Paused | Resume | Complete |
| Completed | Archive | Export Results |
| Archived | — (no primary action) | Export Results |

### 10.2 Submit for Approval

**Preconditions:**
- Campaign must be in `draft` or `rejected` state.
- All 5 readiness sections must be marked as ready (targets, email templates, landing page, infrastructure, schedule).
- If any section is incomplete, the button is disabled with a tooltip: "Complete all configuration sections before submitting."

**Flow:**
1. User clicks "Submit for Approval".
2. A modal appears with:
   - Summary of the campaign configuration (name, target count, launch date).
   - A "Submission Note" textarea (optional, max 500 characters).
   - Blocklist warning (if applicable): "This campaign has {n} blocklist matches. An additional Administrator approval will be required."
   - "Submit" (primary) and "Cancel" (secondary) buttons.
3. On submit: `POST /api/v1/campaigns/:id/submit`.
4. On success: state transitions to `pending_approval`, toast notification "Campaign submitted for approval", approval banner appears (section 11).
5. On error: error toast, modal remains open.

### 10.3 Approve

**Preconditions:**
- User must have `campaigns:approve` permission.
- Campaign must be in `pending_approval` state.
- If blocklist escalation is active, user must also have `campaigns:admin_approve` permission (Administrator role).

**Flow:**
1. User clicks "Approve".
2. A modal appears with:
   - Campaign summary.
   - An optional "Approval Note" textarea.
   - If blocklist escalation: "This approval also covers {n} blocklist-matched targets."
   - "Approve" (primary, green) and "Cancel" buttons.
3. On submit: `POST /api/v1/campaigns/:id/approve`.
4. On success: state transitions to `approved`, toast "Campaign approved".

### 10.4 Reject

**Flow:**
1. User clicks "Reject" from the dropdown or the approval banner.
2. A modal appears with:
   - Campaign summary.
   - A **required** "Rejection Reason" textarea (min 10 characters).
   - "Reject" (danger) and "Cancel" buttons.
3. On submit: `POST /api/v1/campaigns/:id/reject` with reason.
4. On success: state transitions to `rejected`, toast "Campaign rejected". The campaign returns to an editable state so the submitter can address the feedback and resubmit.

### 10.5 Build

**Preconditions:**
- Campaign must be in `approved` state.
- User must have `campaigns:build` permission.

**Flow:**
1. User clicks "Build".
2. A confirmation modal appears: "This will provision cloud infrastructure and prepare the campaign for launch. Proceed?" with "Build" (primary) and "Cancel" buttons.
3. On submit: `POST /api/v1/campaigns/:id/build`.
4. On success: state transitions to `building`, the build progress UI appears (section 12).

### 10.6 Launch

**Preconditions:**
- Campaign must be in `ready` state.
- If the campaign is scheduled for a future time, the primary button shows "Scheduled: {date}" (disabled) instead of "Launch".

**Flow:**
1. User clicks "Launch".
2. A confirmation modal appears: "This will begin sending phishing emails to {n} targets. This action cannot be undone. Proceed?" with "Launch" (primary, green) and "Cancel" buttons.
3. On submit: `POST /api/v1/campaigns/:id/launch`.
4. On success: state transitions to `active`, Overview tab switches to metrics view, toast "Campaign launched".

### 10.7 Pause / Resume

**Pause:**
1. "Pause" button click — no confirmation modal (immediate action).
2. `POST /api/v1/campaigns/:id/pause`.
3. Email sending stops. Endpoint remains online. Metrics continue to be collected (targets may still interact with already-delivered emails).
4. Toast: "Campaign paused. No new emails will be sent."

**Resume:**
1. "Resume" button click — no confirmation modal (immediate action).
2. `POST /api/v1/campaigns/:id/resume`.
3. Email sending resumes from where it left off.
4. Toast: "Campaign resumed."

### 10.8 Complete

**Flow:**
1. User clicks "Complete" from the dropdown.
2. A confirmation modal appears: "This will end the campaign. The landing page endpoint will remain online for the grace period ({n} hours), then be terminated. This cannot be undone."
3. On submit: `POST /api/v1/campaigns/:id/complete`.
4. On success: state transitions to `completed`. Endpoint teardown begins after the configured grace period.

### 10.9 Archive

**Flow:**
1. User clicks "Archive" as the primary action.
2. No confirmation modal — archiving is a lightweight organizational action.
3. `POST /api/v1/campaigns/:id/archive`.
4. On success: state transitions to `archived`. Toast: "Campaign archived."
5. Archived campaigns are hidden from the default list view (visible only when the "Archived" status filter is explicitly selected).

### 10.10 Delete

**Preconditions:**
- Campaign must be in `draft` or `rejected` state.

**Flow:**
1. User clicks "Delete" from the dropdown.
2. A destructive confirmation modal appears with `--danger-muted` background: "Permanently delete '{campaign name}'? This action cannot be undone." with "Delete" (danger) and "Cancel" buttons.
3. On submit: `DELETE /api/v1/campaigns/:id`.
4. On success: navigate to the campaign list, toast "Campaign deleted."

### 10.11 Duplicate

Available in all states except `building`:

1. User clicks "Duplicate" from the dropdown.
2. `POST /api/v1/campaigns/:id/duplicate`.
3. Creates a new campaign in `draft` state with all configuration copied except: name (appended with " (Copy)"), dates (cleared), and state (set to draft).
4. On success: navigate to the new campaign's workspace, toast "Campaign duplicated."

---

## 11. Approval Workflow — Inline Banner

### 11.1 Banner Display

When a campaign is in `pending_approval` state, an inline banner appears between the page header and the tab bar:

```
┌──────────────────────────────────────────────────────────────────────┐
│  ⏳ PENDING APPROVAL                                                │
│                                                                      │
│  Submitted by jdoe on Mar 6 at 14:00.                               │
│  "Ready for Q1 exec test. All configs verified."                    │
│                                                                      │
│  ⚠ Blocklist escalation: 3 targets match blocklist entries.        │
│    Administrator approval required.                                  │
│                                                                      │
│  [Approve]  [Reject]                    (visible to reviewers only) │
│                                                                      │
└──────────────────────────────────────────────────────────────────────┘
```

### 11.2 Banner Styling

- Background: `--warning-muted` (subtle amber tint).
- Left border: 4px solid `--status-pending`.
- Icon: hourglass icon in `--status-pending` color.
- Full width, spanning the content area.
- The banner is rendered as a distinct section — it does not overlap or obscure tab content.

### 11.3 Submitter View

The campaign submitter sees:
- Submission details (who submitted, when, with what note).
- Blocklist escalation notice (if applicable).
- No Approve/Reject buttons — submitters cannot approve their own campaigns.
- A "Withdraw Submission" link in `--text-secondary` that returns the campaign to `draft` state (via `POST /api/v1/campaigns/:id/withdraw`). This link is styled as a text link, not a button, to discourage accidental use.

### 11.4 Reviewer View

Users with `campaigns:approve` permission see:
- All the same information as the submitter view.
- "Approve" (primary button) and "Reject" (secondary button with `--danger` text) buttons.
- If blocklist escalation is active and the reviewer does not have `campaigns:admin_approve` permission, the "Approve" button is disabled with tooltip: "Administrator approval required due to blocklist matches."

### 11.5 Rejected Banner

When a campaign transitions to `rejected` state, the banner changes:

```
┌──────────────────────────────────────────────────────────────────────┐
│  ✕ REJECTED                                                        │
│                                                                      │
│  Rejected by admin1 on Mar 6 at 16:30.                              │
│  Reason: "Landing page redirect URL is pointing to staging.         │
│  Update to production URL and resubmit."                             │
│                                                                      │
│  Configuration tabs are now editable. Address the feedback and      │
│  resubmit when ready.                                                │
│                                                                      │
└──────────────────────────────────────────────────────────────────────┘
```

- Background: `--danger-muted`.
- Left border: 4px solid `--status-rejected`.
- The banner persists until the campaign is resubmitted.

---

## 12. Build Progress UI

### 12.1 Display Context

When a campaign enters the `building` state, the Overview tab replaces its normal content with the build progress view. This is a full-tab-width display showing each build step in sequence.

### 12.2 Layout

```
┌──────────────────────────────────────────────────────────────────────┐
│  BUILD PROGRESS                                                      │
│                                                                      │
│  ┌────────────────────────────────────────────────────────────────┐  │
│  │  ✓ Snapshot target list                           2s    14:23 │  │
│  │    142 targets locked for this campaign.                       │  │
│  │                                                                │  │
│  │  ✓ Assign A/B variants                            1s    14:23 │  │
│  │    71 targets → Variant A, 71 targets → Variant B.            │  │
│  │                                                                │  │
│  │  ✓ Compile landing page                           3s    14:23 │  │
│  │    O365 Login compiled successfully.                           │  │
│  │                                                                │  │
│  │  ✓ Start landing page application                 8s    14:23 │  │
│  │    Application listening on port 443.                          │  │
│  │                                                                │  │
│  │  ◐ Provision endpoint                            ...    14:23 │  │
│  │    Creating t3.micro in us-east-1...                           │  │
│  │    Instance ID: i-0abc123def456                                │  │
│  │                                                                │  │
│  │  ○ Deploy reverse proxy                           —      —    │  │
│  │  ○ Configure DNS                                  —      —    │  │
│  │  ○ Obtain TLS certificate                         —      —    │  │
│  │  ○ Health check                                   —      —    │  │
│  └────────────────────────────────────────────────────────────────┘  │
│                                                                      │
│  Elapsed: 1m 14s                                                    │
│                                                                      │
└──────────────────────────────────────────────────────────────────────┘
```

### 12.3 Build Steps

The build process consists of 9 sequential steps:

| Step | Description |
|------|------------|
| 1. Snapshot target list | Lock the target list for this campaign |
| 2. Assign A/B variants | Randomly distribute targets across variants |
| 3. Compile landing page | Build the landing page HTML/app |
| 4. Start landing page app | Boot the LP application process |
| 5. Provision endpoint | Create cloud VM instance |
| 6. Deploy reverse proxy | Install and configure the proxy on the instance |
| 7. Configure DNS | Point the endpoint domain to the instance IP |
| 8. Obtain TLS certificate | Provision a Let's Encrypt certificate |
| 9. Health check | Verify the full stack is responding correctly |

### 12.4 Step States

| State | Icon | Duration Column | Behavior |
|-------|------|----------------|----------|
| Pending | ○ (hollow circle, `--text-muted`) | "—" | Step has not started |
| In Progress | ◐ (half-filled, `--status-building`, animated spin) | "..." (animated dots) | Step is currently executing |
| Completed | ✓ (checkmark, `--success`) | "{n}s" | Step finished successfully |
| Failed | ✕ (cross, `--danger`) | "{n}s" | Step failed |

- Each step shows: status icon, step name, duration, and timestamp.
- Below the step name, a detail line in `--text-secondary` `--text-small` shows contextual information from the build log.
- The in-progress step has a subtle pulsing background animation using `--status-building` at 5% opacity.

### 12.5 Real-Time Updates

- Build progress is streamed via WebSocket. Each build log event updates the corresponding step.
- Steps transition from pending → in-progress → completed/failed in sequence.
- An elapsed timer at the bottom counts up from the build start time.
- When all steps complete: the elapsed timer stops, the Overview tab transitions to the readiness/metrics view, and a toast appears: "Build complete. Campaign is ready to launch."

### 12.6 Build Failure

If any step fails:

- The failed step shows the error icon and message in `--danger` color.
- All subsequent pending steps remain as pending (they are not attempted).
- A failure banner appears below the step list:

```
┌──────────────────────────────────────────────────────────────────────┐
│  ✕ Build failed at step "Provision endpoint"                        │
│                                                                      │
│  Error: InsufficientCapacity — no t3.micro instances available      │
│  in us-east-1a. Try a different region or instance type.            │
│                                                                      │
│  All provisioned resources have been rolled back.                   │
│  The campaign has been returned to Draft state.                     │
│                                                                      │
│  [Return to Infrastructure Tab]                                     │
└──────────────────────────────────────────────────────────────────────┘
```

- On build failure, the API rolls back all resources and returns the campaign to `draft` state.
- The "Return to Infrastructure Tab" link navigates to the Infrastructure tab so the user can adjust configuration and retry.
- Build failure details are preserved in the Approval History tab as an informational event.

---

## 13. Active Campaign Behavior

### 13.1 Overview Tab — Live Metrics

When a campaign is `active`:

- The stat cards (Emails Sent, Opened, Clicked, Credentials) update in real-time via WebSocket.
- Counter animations: numbers increment with a brief counter-roll animation (`--duration-fast`).
- The engagement chart's rightmost data point extends in real-time.
- The credential capture feed prepends new entries as they arrive (slide-in animation, `--duration-normal`).

### 13.2 Configuration Tabs — Read-Only

All configuration tabs (Targets, Email Templates, Landing Page, Infrastructure, Schedule) display their saved values in read-only format as specified in section 2.4. A banner at the top of each tab reads: "Configuration is locked while the campaign is active."

### 13.3 Infrastructure Tab — Live Monitoring

The Infrastructure tab's Endpoint Status section provides live monitoring:

- Health status updates via WebSocket.
- If the endpoint goes degraded or offline, the status dot changes color and a warning/error banner appears.
- Response time is shown from the most recent health check.

### 13.4 Credential Capture Feed

The credential capture feed on the Overview tab is the primary real-time display of incoming credentials:

- New captures appear at the top with a slide-in animation.
- Each row shows: relative timestamp, target email (truncated to 20 chars), A/B variant, captured field name, masked value with Reveal button.
- **Reveal flow**:
  1. User clicks "Reveal" on a masked value.
  2. Frontend calls `GET /api/v1/campaigns/:id/credentials/:cred_id/decrypt`.
  3. The server decrypts the value, returns it, and logs the access in the audit trail.
  4. The plaintext value replaces the masked dots with a fade-in animation.
  5. A 30-second countdown begins (not visible to the user).
  6. After 30 seconds, the value fades back to masked dots automatically.
  7. The "Reveal" button reappears, allowing repeated reveals (each logged independently).
- If the user clicks "Hide" before the 30-second timeout, the value re-masks immediately.
- If the user navigates away from the tab, all revealed values are re-masked immediately (no timeout needed).

---

## 14. Completed and Archived Campaign Behavior

### 14.1 Full Read-Only

When a campaign is in `completed` or `archived` state:

- All tabs are read-only (same as active state, but without live updates).
- The Overview tab shows final metrics (frozen at campaign completion).
- No real-time WebSocket connections are established — all data is fetched once and cached.
- The Infrastructure tab shows "Terminated" status with the termination date.

### 14.2 Export Options

An "Export Results" action is available in the primary action dropdown:

- Clicking "Export Results" opens a modal with export configuration:
  - **Format**: CSV or JSON radio buttons.
  - **Content checkboxes**:
    - Campaign summary (metadata, configuration).
    - Target list (all targets with their A/B variant assignment).
    - Engagement data (per-target: sent, opened, clicked, captured timestamps).
    - Credential captures (masked by default — checkbox to include plaintext, requires `credentials:export` permission and creates audit log entry).
    - Metrics summary (aggregate stats).
  - "Export" (primary) and "Cancel" buttons.
- Export calls `POST /api/v1/campaigns/:id/export` with the selected options.
- The export is generated server-side. A toast appears: "Export started. You will be notified when it's ready."
- When the export is ready (delivered via WebSocket event or polling), a download link toast appears.

### 14.3 Archived Distinction

Archived campaigns differ from completed campaigns only in:
- They are hidden from the default campaign list view.
- The status badge shows "ARCHIVED" in `--status-archived`.
- No "Archive" action in the dropdown (already archived).
- An "Unarchive" action appears in the dropdown, which returns the campaign to `completed` state.

---

## 15. Error States and Edge Cases

### 15.1 Concurrent Edit Conflict

If two users have the same campaign workspace open and one saves a tab:
- The second user's next save attempt receives a `409 Conflict` response.
- An error banner appears on the tab: "This configuration was updated by {username} at {time}. Reload to see their changes, or overwrite with your version."
- Two buttons: "Reload" (discards local changes, fetches latest) and "Overwrite" (force-saves with `If-Match` header override).
- This uses optimistic concurrency control via the `ETag` / `If-Match` header pattern. Each save response includes an `ETag` that is sent with the next save request.

### 15.2 Campaign Deleted While Viewing

If the campaign is deleted by another user (or via API) while the workspace is open:
- The next API call returns `404 Not Found`.
- A full-page error state appears: "This campaign no longer exists. It may have been deleted." with a "Back to Campaigns" button.
- All WebSocket subscriptions for this campaign are unsubscribed.

### 15.3 Permission Denied

If the user's permissions change while viewing the workspace (e.g., role downgraded):
- The next API call returns `403 Forbidden`.
- A toast appears: "You no longer have permission to access this campaign."
- The workspace becomes fully read-only.
- Action buttons are hidden.

### 15.4 State Changed Externally

If the campaign state changes while the user is viewing the workspace (e.g., another user approves it):
- A WebSocket event triggers a notification banner at the top of the workspace: "Campaign state changed to {new_state} by {user}. [Refresh]"
- Clicking "Refresh" reloads the workspace data.
- The status badge in the header updates immediately.
- If the state change makes the current tab read-only (e.g., campaign transitioned from draft to pending_approval), the read-only mode is applied immediately.

### 15.5 Build Timeout

If the build process does not complete within 10 minutes:
- The build progress UI shows a timeout warning: "Build is taking longer than expected. The system will continue trying."
- After 15 minutes, the build is considered failed. The failure banner appears with a generic timeout error.
- The campaign is rolled back to `draft` state.

### 15.6 WebSocket Disconnection

If the WebSocket connection drops during an active campaign:
- A subtle warning banner appears below the tab bar: "Live updates disconnected. Reconnecting..." with a pulsing dot animation.
- The system attempts automatic reconnection with exponential backoff (1s, 2s, 4s, 8s, max 30s).
- On reconnection, the system fetches the latest data to reconcile any missed events.
- If reconnection fails after 5 attempts, the banner changes to: "Live updates unavailable. [Retry] or refresh the page for latest data."

### 15.7 Large Target Count Warning

If the total target count exceeds 1,000:
- A notice appears on the Targets tab: "This campaign targets {n} users. Large campaigns may take longer to build and send. Consider splitting into smaller campaigns for better deliverability."
- This is informational only — it does not block any actions.

### 15.8 Scheduled Launch in the Past

If a campaign in `ready` state has a scheduled launch time that has already passed:
- The primary action button shows "Launch Now" instead of the scheduled time.
- A notice appears: "The scheduled launch time has passed. Launch manually or update the schedule."
- The scheduled launch automation will not fire for past dates.

### 15.9 Empty Tab States

Each tab has an empty state when no configuration exists:

| Tab | Empty State Message |
|-----|-------------------|
| Targets | "No target groups assigned. Add groups to define who will receive the phishing email." |
| Email Templates | "No email template configured. Select or create a template to define the phishing email." |
| Landing Page | "No landing page selected. Choose a page to capture credentials or track clicks." |
| Infrastructure | "Infrastructure not configured. Set up cloud hosting for the phishing landing page." |
| Schedule | "Schedule not configured. Define when and how the campaign will send emails." |
| Approval History | "No approval history. This campaign has not been submitted for review." |

Each empty state includes the section's icon from Lucide (at `xl` size), the message in `--text-secondary`, and a "Configure" button (primary) that focuses the first input on that tab.

### 15.10 Network Errors

For any API failure during tab save:
- Toast notification: "Failed to save {tab name}. Check your connection and try again."
- The Save button reverts to its default enabled state.
- Form data is preserved — no data is lost.
- If the error is a `422 Unprocessable Entity` with validation errors, inline field-level errors are shown instead of a generic toast.

### 15.11 Browser Navigation with Unsaved Changes

If the user attempts to leave the workspace entirely (browser back, URL change, tab close) with unsaved changes on any tab:
- The browser's native `beforeunload` confirmation dialog is triggered.
- This is intentionally the browser's native dialog (not a custom modal) because custom modals cannot intercept `beforeunload` reliably.
- Navigating between tabs within the workspace uses the custom in-app confirmation modal described in section 2.5.
