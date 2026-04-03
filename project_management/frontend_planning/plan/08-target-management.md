# 08 — Target Management

This document specifies the **Target Management** section of the Tackle platform: listing, creating, editing, importing, grouping, exporting, and blocklist management for phishing simulation targets. Targets are persistent entities that exist independently of campaigns — they are never deleted when a campaign is removed. The target management interface provides a primary table view with search and filtering, slide-over panels for detail and editing, a multi-step CSV import wizard, group management, blocklist configuration, and comprehensive bulk operations.

---

## 1. Target List View

### 1.1 Purpose

The target list is the primary management surface for all targets in the system. It displays a paginated, searchable, filterable table of targets with inline status indicators, kebab menus for per-row actions, checkbox selection for bulk operations, and a floating action bar for batch processing.

### 1.2 Layout

```
┌──────────────────────────────────────────────────────────────────────────┐
│  Targets                                [Import CSV]  [+ Add Target]    │
├──────────────────────────────────────────────────────────────────────────┤
│  [Targets]  [Groups]  [Blocklist]                                       │
├──────────────────────────────────────────────────────────────────────────┤
│  ┌────────────────────────┐ ┌──────────────┐ ┌──────────────┐           │
│  │ 🔍 Search targets...   │ │ Department ▾ │ │ Group ▾      │           │
│  └────────────────────────┘ └──────────────┘ └──────────────┘           │
│  ┌──────────────┐                                                       │
│  │ Activity ▾   │  [Clear Filters]                                      │
│  └──────────────┘                                                       │
├──┬───────────────┬───────────────┬────────────┬──────────┬──────┬───────┤
│☐ │ Name          │ Email         │ Department │ Title    │ Grps │  ···  │
├──┼───────────────┼───────────────┼────────────┼──────────┼──────┼───────┤
│☐ │ Jane Smith    │ jsmith@co.com │ Marketing  │ Director │ 3    │  ···  │
│☐ │ Bob Lee       │ blee@co.com   │ IT         │ Engineer │ 1    │  ···  │
│☐ │ Carol Diaz    │ cdiaz@co.com  │ Finance    │ Analyst  │ 2    │  ···  │
│☐ │ Dan Kim       │ dkim@co.com   │ HR         │ Manager  │ 0    │  ···  │
│☐ │ Eve Johnson   │ ejohn@co.com  │ Sales      │ Rep      │ 1    │  ···  │
├──┴───────────────┴───────────────┴────────────┴──────────┴──────┴───────┤
│  Showing 1–25 of 1,247                             [← 1  2  3 ... →]   │
└──────────────────────────────────────────────────────────────────────────┘

                  ┌──── Floating Action Bar (on selection) ──────────┐
                  │  5 selected  [Add to Group] [Edit] [Export]      │
                  │              [Remove from Group] [Delete]  [✕]   │
                  └──────────────────────────────────────────────────┘
```

### 1.3 Top-Level Tabs

The target management section has three tabs at the top:

| Tab | Route | Content |
|-----|-------|---------|
| Targets | `/targets` | Target list table (this section) |
| Groups | `/targets/groups` | Target group management (section 5) |
| Blocklist | `/targets/blocklist` | Blocklist entry management (section 9) |

The active tab is indicated by a bottom border in `--accent` color. Tab switching does not reset filters on the Targets tab — filters are preserved in URL query parameters.

### 1.4 Table Columns

| Column | Width | Content | Sortable |
|--------|-------|---------|----------|
| Checkbox | 40px | Row selection checkbox | No |
| Name | flex | `first_name last_name` combined. If either is missing, show available name only. If both missing, show `—`. Truncated at 30 chars with tooltip. | Yes (sorts by `last_name` then `first_name`) |
| Email | flex | Email address. Truncated at 30 chars with tooltip. | Yes |
| Department | 130px | Department name. Truncated at 20 chars with tooltip. If null, show `—`. | Yes |
| Title | 130px | Job title. Truncated at 20 chars with tooltip. If null, show `—`. | Yes |
| Groups | 60px | Count of groups the target belongs to. Clickable — opens a tooltip listing group names (max 10, then "+N more"). | Yes |
| Actions | 48px | Kebab menu (`···`) | No |

- Default sort: **Email ascending** (alphabetical).
- Clicking a row (outside the checkbox or kebab) opens the target detail slide-over (section 3).
- Row hover: `--bg-hover` background.
- Soft-deleted targets are NOT shown in the default view. They are accessible only via the "Activity" filter set to "Deleted" (section 1.6).

### 1.5 Search

- Placeholder: "Search targets..."
- Searches against `email`, `first_name`, `last_name`, `department`, and `title`.
- Debounced at 300ms, minimum 2 characters.
- Sends `search` query parameter to `GET /api/v1/targets`.
- While searching, the table shows a subtle loading shimmer on existing rows (no full-page spinner).
- When search returns zero results: show an empty state illustration with text "No targets match your search." and a "Clear search" link.

### 1.6 Filters

All filters are applied as query parameters to `GET /api/v1/targets` and are AND-combined.

**Department Filter (Multi-Select Dropdown):**
- Options populated dynamically from `GET /api/v1/targets/departments`.
- Multiple departments can be selected simultaneously.
- Selected departments appear as removable chips below the filter bar.
- Maps to `?department=Marketing,IT,Finance`.
- The dropdown includes a search input at the top for large department lists (>10 items).

**Group Filter (Multi-Select Dropdown):**
- Options populated from `GET /api/v1/target-groups`.
- Multiple groups can be selected. Targets matching ANY selected group are shown (OR logic within this filter).
- Maps to `?group_id=1,2,3`.

**Activity Filter (Single-Select Dropdown):**
- Options: "Active" (default), "Deleted", "All".
- "Active" shows only targets where `deleted_at IS NULL`.
- "Deleted" shows only soft-deleted targets (`deleted_at IS NOT NULL`). When this filter is active, rows display in a muted style with `--text-muted` text color and a subtle strikethrough on the email column.
- "All" shows both active and deleted targets.
- Maps to `?status=active|deleted|all`.

**Clear Filters:**
- A "Clear Filters" link appears whenever any filter is active.
- Clicking it resets all filters to defaults and clears URL parameters (except pagination).

**Filter Persistence:**
- Active filters are reflected in URL query parameters.
- Navigating back to the target list restores filters from the URL.

### 1.7 Sorting

- Clicking a column header cycles: unsorted -> ascending -> descending -> unsorted.
- The active sort column shows an arrow indicator (▲/▼).
- Default sort: `email` ascending.
- Maps to `?sort=<field>&order=asc|desc` in the API request.
- Only one column can be sorted at a time.

### 1.8 Pagination

- 25 items per page (fixed).
- Pagination control at the bottom of the table shows: "Showing X–Y of Z" on the left, page navigation on the right.
- Page navigation: previous/next arrow buttons plus up to 5 page number links with ellipsis for large sets.
- Maps to `?page=N&per_page=25`.
- Changing filters or search resets to page 1.

### 1.9 Kebab Menu Actions

Clicking the kebab icon (`···`) on a row opens a dropdown menu:

| Action | Icon | Behavior |
|--------|------|----------|
| View Details | Eye | Opens the target detail slide-over (section 3) |
| Edit | Pencil | Opens the target edit slide-over (section 4) |
| Add to Group | Folder+ | Opens group assignment modal (section 1.10) |
| Delete | Trash (red) | Opens delete confirmation modal (section 1.11) |

For soft-deleted targets (visible when Activity filter is "Deleted" or "All"), the menu shows:

| Action | Icon | Behavior |
|--------|------|----------|
| View Details | Eye | Opens the target detail slide-over (read-only) |
| Restore | Undo | Restores the soft-deleted target (section 10) |
| — | — | No edit or delete options for deleted targets |

### 1.10 Quick Add to Group Modal

When "Add to Group" is selected from the kebab menu for a single target:

```
┌─────────────────────────────────────────┐
│  Add to Group                     [✕]   │
├─────────────────────────────────────────┤
│                                         │
│  Target: jsmith@company.com             │
│                                         │
│  ┌─────────────────────────────────┐    │
│  │ 🔍 Search groups...             │    │
│  └─────────────────────────────────┘    │
│                                         │
│  ☐  Executive Team          (42)        │
│  ☑  Marketing Department    (67)        │
│  ☐  IT Support Staff        (33)        │
│  ☐  New Hires Q1            (18)        │
│                                         │
│  Groups already assigned are checked    │
│  and listed first.                      │
│                                         │
│              [Cancel]  [Save]           │
└─────────────────────────────────────────┘
```

- The modal lists all target groups with checkboxes.
- Groups the target already belongs to are pre-checked and sorted to the top.
- The search input filters the group list in real-time (client-side filter).
- Clicking "Save" computes the diff: newly checked groups get `POST /api/v1/target-groups/{id}/members`, newly unchecked groups get `DELETE /api/v1/target-groups/{id}/members`.
- On success: toast "Group membership updated." and the Groups column count refreshes.

### 1.11 Delete Confirmation Modal

```
┌─────────────────────────────────────────┐
│  Delete Target                    [✕]   │
├─────────────────────────────────────────┤
│                                         │
│  Are you sure you want to delete        │
│  jsmith@company.com?                    │
│                                         │
│  This target will be soft-deleted and   │
│  can be restored later. Campaign        │
│  history will be preserved.             │
│                                         │
│              [Cancel]  [Delete]         │
└─────────────────────────────────────────┘
```

- "Delete" button uses `--danger` background color.
- On confirm: `DELETE /api/v1/targets/{id}`.
- On success: row animates out with a fade-and-collapse (200ms, `--duration-smooth`), toast "Target deleted. [Undo]". The "Undo" link in the toast calls `POST /api/v1/targets/{id}/restore`.
- Toast duration: 8 seconds (extended from default to give time for undo).

### 1.12 Empty State

When no targets exist in the system at all (not due to filtering):

```
┌──────────────────────────────────────────────────────────────────────┐
│                                                                      │
│                    ┌─────────────────────┐                           │
│                    │   (Users icon,      │                           │
│                    │    muted color)     │                           │
│                    └─────────────────────┘                           │
│                                                                      │
│                    No targets yet                                     │
│                                                                      │
│           Add targets individually or import a CSV                   │
│           file to get started with your first campaign.              │
│                                                                      │
│              [Import CSV]    [+ Add Target]                          │
│                                                                      │
└──────────────────────────────────────────────────────────────────────┘
```

---

## 2. Single Target Creation

### 2.1 Trigger

Clicking the "[+ Add Target]" button in the page header opens a slide-over panel from the right side of the viewport.

### 2.2 Layout

```
┌───────────────────────────┬─────────────────────────────────────────┐
│                           │  ┌─────────────────────────────────────┐│
│  (List continues dimmed   │  │  Add Target                   [✕]  ││
│   behind the overlay)     │  ├─────────────────────────────────────┤│
│                           │  │                                     ││
│                           │  │  Email *                             ││
│                           │  │  ┌─────────────────────────────────┐││
│                           │  │  │ e.g. jsmith@company.com        │││
│                           │  │  └─────────────────────────────────┘││
│                           │  │                                     ││
│                           │  │  ┌────────────────┐ ┌──────────────┐││
│                           │  │  │ First Name     │ │ Last Name    │││
│                           │  │  │ ┌────────────┐ │ │ ┌──────────┐│││
│                           │  │  │ │            │ │ │ │          ││││
│                           │  │  │ └────────────┘ │ │ └──────────┘│││
│                           │  │  └────────────────┘ └──────────────┘││
│                           │  │                                     ││
│                           │  │  Department                         ││
│                           │  │  ┌─────────────────────────────────┐││
│                           │  │  │ e.g. Marketing                 │││
│                           │  │  └─────────────────────────────────┘││
│                           │  │                                     ││
│                           │  │  Title                              ││
│                           │  │  ┌─────────────────────────────────┐││
│                           │  │  │ e.g. Senior Director            │││
│                           │  │  └─────────────────────────────────┘││
│                           │  │                                     ││
│                           │  │  ── Custom Fields ──────────────    ││
│                           │  │                                     ││
│                           │  │  ┌──────────┐ ┌──────────────────┐  ││
│                           │  │  │ Key      │ │ Value            │  ││
│                           │  │  └──────────┘ └──────────────────┘  ││
│                           │  │  [+ Add Custom Field]               ││
│                           │  │                                     ││
│                           │  │              [Cancel]  [Add Target] ││
│                           │  └─────────────────────────────────────┘│
└───────────────────────────┴─────────────────────────────────────────┘
```

### 2.3 Slide-Over Specifications

- Width: 520px on desktop; full-width on viewports below 768px.
- Background: `--bg-secondary`.
- Header: "Add Target" as title, close (✕) button top-right.
- Clicking outside the slide-over or pressing Escape closes it (with unsaved changes confirmation if dirty — see 2.7).

### 2.4 Form Fields

| Field | Type | Required | Validation | Notes |
|-------|------|----------|------------|-------|
| Email | Text input | Yes | Valid email format; case-insensitive uniqueness check | Auto-lowercased on blur |
| First Name | Text input | No | Max 255 chars | — |
| Last Name | Text input | No | Max 255 chars | — |
| Department | Combobox | No | Max 255 chars | Autocomplete from existing departments via `GET /api/v1/targets/departments`. User can type a new value. |
| Title | Text input | No | Max 255 chars | — |
| Custom Fields | Key-value pairs | No | Max 50 keys; key max 255 chars; value max 1024 chars | Dynamic rows, add/remove |

### 2.5 Real-Time Validation

**Email field:**
- On blur, validate format client-side using a standard email regex.
- If format is valid, check uniqueness by calling `GET /api/v1/targets?search={email}` and checking if any result has an exact case-insensitive match.
- If duplicate found: show inline error "A target with this email already exists." with a link "View existing target" that opens the detail slide-over for that target.
- If the email matches a blocklist entry: show inline warning (not error) "This email matches a blocklist entry. Campaign sends to this address will require administrator approval." Warning does NOT prevent creation.
- Loading state during uniqueness check: show a subtle spinner icon inside the email input.

**Custom fields:**
- Each custom field row has a key input, value input, and a remove (✕) button.
- Key validation: must be non-empty if a value is entered; must not duplicate an existing key in the form.
- Duplicate key: highlight the row with `--danger` border and inline error "Duplicate key."
- Counter below the custom fields section: "N / 50 custom fields" — turns `--danger` at 50.
- Value character count displayed below each value input: "N / 1024" — turns `--warning` at 900, `--danger` at 1024.

### 2.6 Department Combobox Behavior

- On focus, show a dropdown of existing departments fetched from `GET /api/v1/targets/departments`.
- As the user types, filter the dropdown list in real-time (client-side).
- If the typed value does not match any existing department, the dropdown shows "Use '{typed value}'" at the bottom, allowing new department creation.
- Selecting from the dropdown fills the input; typing a custom value is also accepted.

### 2.7 Submission

- "Add Target" button is disabled until at least the email field is filled and all validations pass.
- On submit: `POST /api/v1/targets` with the form data.
- Payload shape:
  ```json
  {
    "email": "jsmith@company.com",
    "first_name": "Jane",
    "last_name": "Smith",
    "department": "Marketing",
    "title": "Director",
    "custom_fields": {
      "office_location": "Building A",
      "employee_id": "EMP-1234"
    }
  }
  ```
- On success: close slide-over, show toast "Target added successfully.", prepend the new target to the table (or refresh if sorted/filtered).
- On 409 Conflict (duplicate email): show inline error on email field "A target with this email already exists."
- On 422 Validation Error: map field-level errors to inline error messages on the corresponding fields.

### 2.8 Unsaved Changes Guard

If the user has entered data in any field and attempts to close the slide-over (via ✕, Escape, or outside click):

```
┌─────────────────────────────────────────┐
│  Unsaved Changes                  [✕]   │
├─────────────────────────────────────────┤
│                                         │
│  You have unsaved changes. Are you      │
│  sure you want to discard them?         │
│                                         │
│          [Keep Editing]  [Discard]       │
└─────────────────────────────────────────┘
```

---

## 3. Target Detail View

### 3.1 Trigger

Clicking a target row in the list (outside checkbox and kebab) opens the detail slide-over from the right.

### 3.2 Layout

```
┌───────────────────────────┬─────────────────────────────────────────┐
│                           │  ┌─────────────────────────────────────┐│
│  (List continues dimmed   │  │  Jane Smith                  [✕]   ││
│   behind the overlay)     │  │  jsmith@company.com                ││
│                           │  ├─────────────────────────────────────┤│
│                           │  │                                     ││
│                           │  │  DETAILS                            ││
│                           │  │  ─────────────────────────────────  ││
│                           │  │  Department    Marketing            ││
│                           │  │  Title         Director             ││
│                           │  │  Groups        Executive Team,      ││
│                           │  │                Marketing Dept       ││
│                           │  │  Created       Jan 15, 2026         ││
│                           │  │  Updated       Mar 22, 2026         ││
│                           │  │                                     ││
│                           │  │  CUSTOM FIELDS                      ││
│                           │  │  ─────────────────────────────────  ││
│                           │  │  office_location   Building A       ││
│                           │  │  employee_id       EMP-1234         ││
│                           │  │                                     ││
│                           │  │  CAMPAIGN HISTORY                   ││
│                           │  │  ─────────────────────────────────  ││
│                           │  │  Q1 Exec Spear    ACTIVE   Mar 15  ││
│                           │  │    ● Sent ● Opened ● Clicked       ││
│                           │  │  HR Benefits      COMPLETED Feb 01 ││
│                           │  │    ● Sent ● Opened ○ Clicked       ││
│                           │  │  IT Recon Q4      COMPLETED Nov 10 ││
│                           │  │    ● Sent ○ Opened ○ Clicked       ││
│                           │  │                                     ││
│                           │  │           [Edit Target]             ││
│                           │  └─────────────────────────────────────┘│
└───────────────────────────┴─────────────────────────────────────────┘
```

### 3.3 Slide-Over Specifications

- Width: 520px on desktop; full-width below 768px.
- Background: `--bg-secondary`.
- Header: target's full name (or email if no name) as title, close (✕) button top-right.
- Sub-header: email address in `--text-muted`.
- Clicking outside or pressing Escape closes the panel.

### 3.4 Details Section

Displays all target metadata in a two-column label-value layout:

| Label | Value |
|-------|-------|
| Department | Department name or `—` if null |
| Title | Title or `—` if null |
| Groups | Comma-separated list of group names. Each group name is a link that navigates to the Groups tab filtered to that group. If no groups, show `—`. |
| Created | Absolute date in `MMM DD, YYYY` format. Tooltip shows full datetime with timezone. |
| Updated | Relative timestamp (`2h ago`, `3d ago`). Tooltip shows absolute datetime. |

### 3.5 Custom Fields Section

- Displayed as a two-column label-value list, sorted alphabetically by key.
- If no custom fields, the section heading is still shown with text "No custom fields defined." in `--text-muted`.
- Long values wrap to multiple lines.

### 3.6 Campaign History Section

- Displays all campaigns this target has participated in, sorted by campaign start date descending (most recent first).
- Data source: included in the `GET /api/v1/targets/{id}` response as a nested `campaign_history` array.
- Each campaign entry shows:
  - Campaign name (clickable — navigates to that campaign's workspace in a new tab).
  - Campaign status badge (using `--status-*` colors from the design system).
  - Campaign start date.
  - Activity indicators: a row of status dots for Sent, Opened, Clicked, and Submitted (if applicable). Filled dots (`●`) indicate the action occurred; hollow dots (`○`) indicate it did not.
- If the target has no campaign history, show "This target has not participated in any campaigns." in `--text-muted`.
- Maximum 20 campaigns shown initially. If more exist, a "Show all (N)" link expands the list.

### 3.7 Footer

- "Edit Target" button (primary style) — closes the detail slide-over and opens the edit slide-over (section 4).
- For soft-deleted targets, the footer shows a "Restore Target" button (primary) instead of "Edit Target".

---

## 4. Target Editing

### 4.1 Trigger

- Clicking "Edit" from the kebab menu on a target row.
- Clicking "Edit Target" button in the detail slide-over footer.

### 4.2 Layout

The edit slide-over is identical in structure to the create slide-over (section 2.2) with the following differences:

- Header title: "Edit Target" instead of "Add Target".
- All fields are pre-populated with the target's current values.
- The email field is **read-only** — displayed as plain text with a subtle `--bg-tertiary` background. Email changes are not supported to maintain referential integrity with campaign history.
- The submit button reads "Save Changes" instead of "Add Target".
- Custom fields are pre-populated. Existing keys cannot be renamed — only values can be edited or the entire key-value pair can be removed.

### 4.3 Submission

- On submit: `PUT /api/v1/targets/{id}` with the updated fields.
- Only changed fields are included in the request payload (delta update).
- On success: close slide-over, show toast "Target updated.", refresh the row in the table.
- On 422 Validation Error: map errors to fields as in create flow.

### 4.4 Unsaved Changes Guard

Same behavior as the create flow (section 2.8). The guard compares current form state against the initial loaded values to detect changes.

---

## 5. Target Group Management

### 5.1 Purpose

Target groups are reusable collections of targets assigned to campaigns. Groups are managed on a dedicated tab and can be created, renamed, populated, and deleted independently of campaigns.

### 5.2 Groups Tab Layout

```
┌──────────────────────────────────────────────────────────────────────────┐
│  Targets                                                                 │
├──────────────────────────────────────────────────────────────────────────┤
│  [Targets]  [Groups]  [Blocklist]                                       │
├──────────────────────────────────────────────────────────────────────────┤
│                                                     [+ Create Group]     │
│  ┌────────────────────────┐                                              │
│  │ 🔍 Search groups...     │                                              │
│  └────────────────────────┘                                              │
├──┬──────────────────────────┬──────────┬────────────┬─────────┬─────────┤
│☐ │ Group Name               │ Members  │ Campaigns  │ Updated │  ···    │
├──┼──────────────────────────┼──────────┼────────────┼─────────┼─────────┤
│☐ │ Executive Team           │ 42       │ 3          │ 2h ago  │  ···    │
│☐ │ Marketing Department     │ 67       │ 2          │ 1d ago  │  ···    │
│☐ │ IT Support Staff         │ 33       │ 1          │ 3d ago  │  ···    │
│☐ │ New Hires Q1 2026        │ 18       │ 0          │ 1w ago  │  ···    │
├──┴──────────────────────────┴──────────┴────────────┴─────────┴─────────┤
│  Showing 1–25 of 12                                                      │
└──────────────────────────────────────────────────────────────────────────┘
```

### 5.3 Group Table Columns

| Column | Width | Content | Sortable |
|--------|-------|---------|----------|
| Checkbox | 40px | Row selection checkbox | No |
| Group Name | flex | Group name. Truncated at 40 chars with tooltip. | Yes |
| Members | 80px | Count of targets in the group | Yes |
| Campaigns | 80px | Count of campaigns this group is assigned to | Yes |
| Updated | 100px | Relative timestamp. Tooltip shows absolute datetime. | Yes |
| Actions | 48px | Kebab menu (`···`) | No |

- Default sort: **Group Name ascending**.
- Clicking a row opens the group detail slide-over (section 5.6).

### 5.4 Group Kebab Menu

| Action | Icon | Behavior |
|--------|------|----------|
| View Members | Eye | Opens group detail slide-over (section 5.6) |
| Rename | Pencil | Opens inline rename or rename modal (section 5.5) |
| Delete | Trash (red) | Opens delete confirmation modal (section 5.8) |

### 5.5 Create / Rename Group

**Create Group Modal** (triggered by "[+ Create Group]" button):

```
┌─────────────────────────────────────────┐
│  Create Group                     [✕]   │
├─────────────────────────────────────────┤
│                                         │
│  Group Name *                           │
│  ┌─────────────────────────────────┐    │
│  │ e.g. Marketing Department       │    │
│  └─────────────────────────────────┘    │
│                                         │
│  Description                            │
│  ┌─────────────────────────────────┐    │
│  │                                 │    │
│  │                                 │    │
│  └─────────────────────────────────┘    │
│                                         │
│              [Cancel]  [Create]         │
└─────────────────────────────────────────┘
```

- Group name is required, max 255 chars.
- On submit: `POST /api/v1/target-groups` with `{ "name": "...", "description": "..." }`.
- On success: toast "Group created.", new group appears in the table, and the group detail slide-over opens automatically so the user can begin adding members.
- On 409 Conflict (duplicate name): inline error "A group with this name already exists."

**Rename Group Modal** (triggered from kebab menu):

- Same modal layout but with header "Rename Group", pre-populated name and description, submit button reads "Save".
- On submit: `PUT /api/v1/target-groups/{id}`.

### 5.6 Group Detail Slide-Over

```
┌───────────────────────────┬─────────────────────────────────────────┐
│                           │  ┌─────────────────────────────────────┐│
│  (Groups tab dimmed)      │  │  Executive Team              [✕]   ││
│                           │  │  42 members                        ││
│                           │  ├─────────────────────────────────────┤│
│                           │  │  ┌───────────────────┐             ││
│                           │  │  │ 🔍 Search members  │ [+ Add]    ││
│                           │  │  └───────────────────┘             ││
│                           │  │                                     ││
│                           │  │  ┌─────────────────────────────────┐││
│                           │  │  │ Jane Smith                  [✕]│││
│                           │  │  │ jsmith@company.com              │││
│                           │  │  ├─────────────────────────────────┤││
│                           │  │  │ Bob Lee                     [✕]│││
│                           │  │  │ blee@company.com                │││
│                           │  │  ├─────────────────────────────────┤││
│                           │  │  │ Carol Diaz                  [✕]│││
│                           │  │  │ cdiaz@company.com               │││
│                           │  │  └─────────────────────────────────┘││
│                           │  │                                     ││
│                           │  │  Showing 1–25 of 42    [← 1 2 →]  ││
│                           │  │                                     ││
│                           │  │  ASSIGNED CAMPAIGNS                 ││
│                           │  │  ─────────────────────────────────  ││
│                           │  │  Q1 Exec Spear    ACTIVE   Mar 15  ││
│                           │  │  CEO Wire Test    DRAFT    —       ││
│                           │  │                                     ││
│                           │  │        [Rename]  [Delete Group]    ││
│                           │  └─────────────────────────────────────┘│
└───────────────────────────┴─────────────────────────────────────────┘
```

### 5.7 Adding / Removing Members

**Adding Members** (triggered by "[+ Add]" button in group detail):

Opens a modal with a searchable, paginated target list:

```
┌─────────────────────────────────────────────────┐
│  Add Members to "Executive Team"          [✕]   │
├─────────────────────────────────────────────────┤
│                                                  │
│  ┌─────────────────────────────────────────┐    │
│  │ 🔍 Search targets by name or email...   │    │
│  └─────────────────────────────────────────┘    │
│                                                  │
│  ☐  Alice Wong        awong@company.com          │
│  ☐  Frank Torres      ftorres@company.com        │
│  ☐  Grace Park        gpark@company.com          │
│  ☑  Helen Cho         hcho@company.com           │
│  ☐  Ivan Petrov       ipetrov@company.com        │
│                                                  │
│  Showing 1–25 of 1,205     [← 1 2 3 ... →]     │
│                                                  │
│  ────────────────────────────────────────────    │
│  1 target selected                               │
│                                                  │
│                      [Cancel]  [Add Selected]    │
└─────────────────────────────────────────────────┘
```

- Targets already in the group are excluded from the list.
- Search debounced at 300ms, calls `GET /api/v1/targets?search=...&exclude_group={group_id}`.
- Multiple targets can be selected. The selected count is shown at the bottom.
- On submit: `POST /api/v1/target-groups/{id}/members` with `{ "target_ids": [1, 2, 3] }`.
- On success: toast "N target(s) added to group.", member list and count refresh.

**Removing Members:**
- Each member row in the group detail has a remove (✕) button.
- Clicking it shows an inline confirmation: the row transforms to show "Remove from group? [Yes] [No]".
- On confirm: `DELETE /api/v1/target-groups/{id}/members` with `{ "target_ids": [id] }`.
- On success: row animates out, member count decrements, toast "Target removed from group."

### 5.8 Group Deletion

```
┌─────────────────────────────────────────┐
│  Delete Group                     [✕]   │
├─────────────────────────────────────────┤
│                                         │
│  Are you sure you want to delete the    │
│  group "Executive Team"?                │
│                                         │
│  This will NOT delete the 42 targets    │
│  in this group — only the group         │
│  itself will be removed.                │
│                                         │
│  ⚠ This group is assigned to 3 active  │
│  campaigns. Deleting it will remove     │
│  it from those campaigns.              │
│                                         │
│              [Cancel]  [Delete]         │
└─────────────────────────────────────────┘
```

- "Delete" button uses `--danger` styling.
- The warning about active campaigns only appears if the group is currently assigned to campaigns with status `active`, `building`, `approved`, or `pending_approval`. The campaign count is fetched from the group detail API response.
- On confirm: `DELETE /api/v1/target-groups/{id}`.
- On success: toast "Group deleted.", row removed from table.

---

## 6. CSV Import Wizard

### 6.1 Purpose

The CSV import wizard provides a guided, multi-step flow for bulk-importing targets from a CSV file. The import is backend-driven: the frontend uploads the file, then walks through column mapping, validation, and commit steps using the backend's import session API.

### 6.2 Wizard Steps

The wizard is presented as a full-width modal overlay with a step indicator:

```
┌──────────────────────────────────────────────────────────────────────────┐
│  Import Targets                                                    [✕]  │
│                                                                          │
│  ● Upload  ──────  ○ Map Columns  ──────  ○ Validate  ──────  ○ Import  │
│                                                                          │
├──────────────────────────────────────────────────────────────────────────┤
│                                                                          │
│                        (Step content here)                               │
│                                                                          │
├──────────────────────────────────────────────────────────────────────────┤
│                                          [Cancel]  [Back]  [Next →]      │
└──────────────────────────────────────────────────────────────────────────┘
```

- Step indicator at the top shows progress: filled circles for completed/current steps, hollow for upcoming.
- Connecting lines between steps fill with `--accent` color as steps complete.
- Footer has Cancel, Back, and Next buttons. Back is hidden on step 1. Next is disabled until the current step's requirements are met.
- Cancel shows a confirmation: "Cancel import? All progress will be lost." with [Keep Going] and [Cancel Import] buttons.

### 6.3 Step 1: Upload

```
┌──────────────────────────────────────────────────────────────────────────┐
│                                                                          │
│  ┌────────────────────────────────────────────────────────────────┐      │
│  │                                                                │      │
│  │             ┌──────────────────┐                               │      │
│  │             │  (Upload icon)   │                               │      │
│  │             └──────────────────┘                               │      │
│  │                                                                │      │
│  │        Drag and drop a CSV file here, or click to browse       │      │
│  │                                                                │      │
│  │        Accepted format: .csv — Maximum file size: 5 MB         │      │
│  │                                                                │      │
│  └────────────────────────────────────────────────────────────────┘      │
│                                                                          │
│  ┌────────────────────────────────────────────────────────────────┐      │
│  │  💡 Tips:                                                      │      │
│  │  • First row should contain column headers                    │      │
│  │  • Required column: email                                     │      │
│  │  • Optional: first_name, last_name, department, title         │      │
│  │  • Additional columns can be mapped to custom fields          │      │
│  │  • Duplicate emails will be updated (upsert behavior)         │      │
│  └────────────────────────────────────────────────────────────────┘      │
│                                                                          │
└──────────────────────────────────────────────────────────────────────────┘
```

**Upload area behavior:**
- Dashed border area supports drag-and-drop and click-to-browse.
- On drag hover: border becomes solid `--accent`, background shifts to `--bg-hover`.
- Accepted file types: `.csv` only. Attempting to drop other file types shows inline error "Only CSV files are accepted."
- Maximum file size: 5 MB. Files exceeding this show error "File exceeds the 5 MB size limit."
- On file selection: show filename, file size, and a "Remove" link to clear the selection.

**After file selected:**
```
│  ┌────────────────────────────────────────────────────────────────┐      │
│  │  📄 employees_q1.csv (2.3 MB)                     [Remove]    │      │
│  └────────────────────────────────────────────────────────────────┘      │
```

- The "Next" button becomes enabled.
- On "Next": `POST /api/v1/targets/import` with the file as multipart form data.
- The backend responds with `import_id`, `headers`, `preview_rows` (first 5 rows), `row_count`, and `column_count`.
- Show a loading indicator on the "Next" button during upload: "Uploading..." with spinner.
- On upload error (413 Payload Too Large, 422 Unprocessable): show error message inline below the upload area.

### 6.4 Step 2: Map Columns

```
┌──────────────────────────────────────────────────────────────────────────┐
│                                                                          │
│  Map your CSV columns to target fields. 1,247 rows detected.            │
│                                                                          │
│  ┌─────────────────────┬────────────────────┬──────────────────────┐     │
│  │ CSV Column          │ Maps To            │ Preview              │     │
│  ├─────────────────────┼────────────────────┼──────────────────────┤     │
│  │ Email Address       │ [email ▾]          │ jsmith@company.com   │     │
│  │ First              │ [first_name ▾]     │ Jane                 │     │
│  │ Last               │ [last_name ▾]      │ Smith                │     │
│  │ Dept               │ [department ▾]     │ Marketing            │     │
│  │ Job Title          │ [title ▾]          │ Director             │     │
│  │ Office             │ [custom: office ▾] │ Building A           │     │
│  │ Employee ID        │ [custom: emp_id ▾] │ EMP-1234             │     │
│  │ Notes              │ [-- Skip --  ▾]    │ Some notes here      │     │
│  └─────────────────────┴────────────────────┴──────────────────────┘     │
│                                                                          │
│  ── Saved Templates ──                                                   │
│  ┌──────────────────────────┐                                            │
│  │ Load template...     ▾   │  [Save Current as Template]               │
│  └──────────────────────────┘                                            │
│                                                                          │
│  ── Data Preview ──                                                      │
│  ┌────────────────────────────────────────────────────────────────┐      │
│  │ Email Address  │ First │ Last  │ Dept      │ Job Title │ ...  │      │
│  │ jsmith@co.com  │ Jane  │ Smith │ Marketing │ Director  │ ...  │      │
│  │ blee@co.com    │ Bob   │ Lee   │ IT        │ Engineer  │ ...  │      │
│  │ cdiaz@co.com   │ Carol │ Diaz  │ Finance   │ Analyst   │ ...  │      │
│  │ (showing 3 of 1,247 rows)                                     │      │
│  └────────────────────────────────────────────────────────────────┘      │
│                                                                          │
└──────────────────────────────────────────────────────────────────────────┘
```

**Mapping table:**
- One row per CSV column header detected by the backend.
- "Maps To" is a dropdown with options:
  - `email` (required — exactly one column must be mapped to email)
  - `first_name`
  - `last_name`
  - `department`
  - `title`
  - `custom: {column_name}` — automatically generates a custom field key from the CSV column name (lowercased, spaces replaced with underscores).
  - `-- Skip --` — ignore this column.
- The system auto-maps columns whose headers closely match field names (case-insensitive, partial match). For example, "Email Address" auto-maps to `email`, "First" to `first_name`.
- Each target field (email, first_name, last_name, department, title) can only be mapped once. Once selected in one row, it is disabled in other rows' dropdowns.
- The "Preview" column shows the value from the first data row for that CSV column to help the user confirm correct mapping.

**Validation for this step:**
- "Next" is disabled until at least one column is mapped to `email`.
- If no column is mapped to `email`, show an inline info message: "You must map at least one column to the Email field."

**On "Next":** `PUT /api/v1/targets/import/{id}/mapping` with the column mapping configuration.

### 6.5 Step 3: Validate

After mapping is submitted, the wizard automatically triggers validation:

- `POST /api/v1/targets/import/{id}/validate`
- While validating, show a progress indicator: "Validating 1,247 rows..." with an indeterminate progress bar.

**Validation Results — Success:**

```
┌──────────────────────────────────────────────────────────────────────────┐
│                                                                          │
│  ✓ Validation Complete                                                  │
│                                                                          │
│  ┌────────────────────────────────────────────────────────────────┐      │
│  │  1,240 rows ready to import                                    │      │
│  │      7 rows with warnings (will be imported)                   │      │
│  │      0 rows with errors (will be skipped)                      │      │
│  └────────────────────────────────────────────────────────────────┘      │
│                                                                          │
│  ── Warnings (7) ──                                                      │
│  ┌──────┬──────────────────┬──────────────────────────────────────┐      │
│  │ Row  │ Email            │ Warning                              │      │
│  ├──────┼──────────────────┼──────────────────────────────────────┤      │
│  │ 45   │ akim@co.com      │ Existing target will be updated      │      │
│  │ 102  │ bpark@co.com     │ Existing target will be updated      │      │
│  │ 233  │ cwu@co.com       │ Matches blocklist entry *@co.com     │      │
│  └──────┴──────────────────┴──────────────────────────────────────┘      │
│                                                                          │
│  "Next" to proceed with import.                                          │
│                                                                          │
└──────────────────────────────────────────────────────────────────────────┘
```

**Validation Results — With Errors:**

```
┌──────────────────────────────────────────────────────────────────────────┐
│                                                                          │
│  ⚠ Validation Complete — Issues Found                                   │
│                                                                          │
│  ┌────────────────────────────────────────────────────────────────┐      │
│  │  1,200 rows ready to import                                    │      │
│  │     12 rows with warnings (will be imported)                   │      │
│  │     35 rows with errors (will be skipped)                      │      │
│  └────────────────────────────────────────────────────────────────┘      │
│                                                                          │
│  ── Errors (35) ──                                                       │
│  ┌──────┬──────────────────┬──────────────────────────────────────┐      │
│  │ Row  │ Email            │ Error                                │      │
│  ├──────┼──────────────────┼──────────────────────────────────────┤      │
│  │ 12   │ (empty)          │ Email is required                    │      │
│  │ 44   │ not-an-email     │ Invalid email format                 │      │
│  │ 88   │ (empty)          │ Email is required                    │      │
│  │ ...showing 3 of 35                                             │      │
│  └──────┴──────────────────┴──────────────────────────────────────┘      │
│  [Show All Errors]                                                       │
│                                                                          │
│  ── Warnings (12) ──                                                     │
│  (collapsed by default, click to expand)                                 │
│                                                                          │
│  Rows with errors will be skipped. Continue to import valid rows.        │
│                                                                          │
└──────────────────────────────────────────────────────────────────────────┘
```

- Errors are shown first, warnings second.
- Initially show up to 10 error/warning rows. "Show All" expands the full list in a scrollable container (max-height: 300px).
- "Next" button text changes to "Import {N} Targets" where N is the count of valid rows.
- If ALL rows have errors (0 valid), "Next" is disabled and the message reads: "All rows have errors. Please fix your CSV and try again." with a "Start Over" button that resets to step 1.

### 6.6 Step 4: Commit (Import)

On clicking "Import {N} Targets":

- `POST /api/v1/targets/import/{id}/commit`
- The button shows a loading spinner with "Importing..." text.
- The step indicator advances to the final step.

**Success result:**

```
┌──────────────────────────────────────────────────────────────────────────┐
│                                                                          │
│  ● Upload  ──────  ● Map Columns  ──────  ● Validate  ──────  ● Import │
│                                                                          │
├──────────────────────────────────────────────────────────────────────────┤
│                                                                          │
│                    ┌─────────────────────┐                               │
│                    │   (Checkmark icon,  │                               │
│                    │    accent color)    │                               │
│                    └─────────────────────┘                               │
│                                                                          │
│                    Import Complete!                                       │
│                                                                          │
│                 1,200 targets imported                                    │
│                    35 rows skipped                                        │
│                                                                          │
│               [Import Another File]  [Done]                              │
│                                                                          │
└──────────────────────────────────────────────────────────────────────────┘
```

- "Done" closes the wizard and refreshes the target list.
- "Import Another File" resets the wizard to step 1.
- On commit failure (500): show error "Import failed. Please try again or contact support." with a "Retry" button that re-calls the commit endpoint.

---

## 7. Import Mapping Templates

### 7.1 Purpose

Users who regularly import CSVs with the same column structure can save their column mappings as reusable templates, avoiding repetitive manual mapping.

### 7.2 Loading a Template

In step 2 (Map Columns) of the import wizard, the "Load template..." dropdown:

- Populated from `GET /api/v1/targets/import/mapping-templates`.
- Each option shows the template name and the date it was last used.
- On selection: the column mapping dropdowns are automatically filled with the saved mapping. If the current CSV has columns not present in the template, those columns default to `-- Skip --`. If the template references columns not in the CSV, those mappings are silently ignored.
- After loading a template, a subtle info bar appears: "Template '{name}' applied. Review and adjust mappings as needed."

### 7.3 Saving a Template

Clicking "[Save Current as Template]" opens a small inline form:

```
┌─────────────────────────────────────────────────────┐
│  Save Mapping Template                              │
│                                                      │
│  Template Name *                                     │
│  ┌─────────────────────────────────────────────┐    │
│  │ e.g. HR System Export Format                 │    │
│  └─────────────────────────────────────────────┘    │
│                                                      │
│                        [Cancel]  [Save Template]     │
└─────────────────────────────────────────────────────┘
```

- On submit: `POST /api/v1/targets/import/mapping-templates` with `{ "name": "...", "mapping": { ... } }`.
- On success: toast "Mapping template saved." The template immediately appears in the "Load template..." dropdown.
- Duplicate name: 409 Conflict, inline error "A template with this name already exists."

---

## 8. Bulk Operations

### 8.1 Selection Mechanism

- Each target row has a checkbox in the first column.
- A "select all" checkbox in the table header selects/deselects all targets on the current page.
- When at least one target is selected, the floating action bar appears at the bottom center of the viewport.
- The header checkbox has three states: unchecked (none selected), checked (all on page selected), indeterminate (some selected).
- Selection state is maintained across pages. If the user selects items on page 1, navigates to page 2, and selects more items, all selections are preserved.
- Maximum selection: governed by practical UI limits. The selected target IDs are tracked client-side.

### 8.2 Floating Action Bar

```
┌────────────────────────────────────────────────────────────────┐
│  12 selected  [Add to Group] [Remove from Group] [Edit Field] │
│               [Export] [Delete]                          [✕]   │
└────────────────────────────────────────────────────────────────┘
```

- Background: `--bg-tertiary`.
- Shadow: `--shadow-lg`.
- Border radius: `--radius-lg`.
- Animation: slides up from bottom with `--duration-smooth` (200ms) ease-out.
- Z-index: `--z-sticky`.
- The ✕ button deselects all targets and dismisses the bar.

### 8.3 Bulk Add to Group

Opens a modal:

```
┌─────────────────────────────────────────┐
│  Add to Group                     [✕]   │
├─────────────────────────────────────────┤
│                                         │
│  Add 12 targets to:                     │
│                                         │
│  ┌─────────────────────────────────┐    │
│  │ Select a group...           ▾   │    │
│  └─────────────────────────────────┘    │
│                                         │
│  Or create a new group:                 │
│  ┌─────────────────────────────────┐    │
│  │ New group name...               │    │
│  └─────────────────────────────────┘    │
│                                         │
│              [Cancel]  [Add to Group]   │
└─────────────────────────────────────────┘
```

- The dropdown lists existing groups from `GET /api/v1/target-groups`.
- Alternatively, the user can type a new group name to create a group and add targets in one operation.
- If a new group name is entered: `POST /api/v1/target-groups` first, then `POST /api/v1/target-groups/{id}/members`.
- If an existing group is selected: `POST /api/v1/target-groups/{id}/members` with `{ "target_ids": [...] }`.
- On success: toast "12 targets added to '{group name}'.", selections cleared, floating bar dismissed.
- Targets already in the selected group are silently skipped (no error).

### 8.4 Bulk Remove from Group

Opens a modal:

```
┌─────────────────────────────────────────┐
│  Remove from Group                [✕]   │
├─────────────────────────────────────────┤
│                                         │
│  Remove 12 targets from:               │
│                                         │
│  ┌─────────────────────────────────┐    │
│  │ Select a group...           ▾   │    │
│  └─────────────────────────────────┘    │
│                                         │
│        [Cancel]  [Remove from Group]    │
└─────────────────────────────────────────┘
```

- Dropdown lists only groups that at least one of the selected targets belongs to.
- On submit: `DELETE /api/v1/target-groups/{id}/members` with `{ "target_ids": [...] }`.
- On success: toast "12 targets removed from '{group name}'.", selections cleared.

### 8.5 Bulk Edit Field

Opens a modal allowing the user to set a single field value across all selected targets:

```
┌─────────────────────────────────────────┐
│  Edit Field                       [✕]   │
├─────────────────────────────────────────┤
│                                         │
│  Update 12 targets:                     │
│                                         │
│  Field                                  │
│  ┌─────────────────────────────────┐    │
│  │ Select field...             ▾   │    │
│  └─────────────────────────────────┘    │
│                                         │
│  New Value                              │
│  ┌─────────────────────────────────┐    │
│  │                                 │    │
│  └─────────────────────────────────┘    │
│                                         │
│  ☐ Clear this field (set to empty)      │
│                                         │
│             [Cancel]  [Apply Changes]   │
└─────────────────────────────────────────┘
```

- Field dropdown options: `first_name`, `last_name`, `department`, `title`. Email is NOT editable in bulk.
- When a field is selected and "Clear this field" is checked, the value input is disabled and the field will be set to `null`.
- On submit: `POST /api/v1/targets/bulk-edit` with `{ "target_ids": [...], "field": "department", "value": "Engineering" }`.
- On success: toast "12 targets updated.", selections cleared, table refreshes.

### 8.6 Bulk Export

- On click: `POST /api/v1/targets/export` with `{ "target_ids": [...] }`.
- The export request is subject to a 5 MB response limit.
- While exporting, the "Export" button shows a spinner.
- On success: browser downloads a CSV file named `targets_export_YYYY-MM-DD.csv`.
- On 413 (payload too large): toast "Export is too large. Please select fewer targets or refine your filters."
- If no targets are selected but filters are active, the export includes all targets matching current filters (not just the current page).

### 8.7 Bulk Delete

Opens a confirmation modal:

```
┌─────────────────────────────────────────┐
│  Delete Targets                   [✕]   │
├─────────────────────────────────────────┤
│                                         │
│  Are you sure you want to delete        │
│  12 targets?                            │
│                                         │
│  These targets will be soft-deleted     │
│  and can be restored later. Campaign    │
│  history will be preserved.             │
│                                         │
│              [Cancel]  [Delete All]     │
└─────────────────────────────────────────┘
```

- "Delete All" button uses `--danger` styling.
- On confirm: `POST /api/v1/targets/bulk-delete` with `{ "target_ids": [...] }`.
- Request body is subject to the 5 MB limit. If the payload exceeds this (extremely large selection), show error toast "Too many targets selected for a single operation. Please select fewer targets."
- On success: toast "12 targets deleted.", rows animate out, selections cleared.

---

## 9. Blocklist Management

### 9.1 Purpose

The blocklist prevents accidental phishing of sensitive individuals or domains. Blocklist entries do NOT silently block sends — instead, matching targets require explicit Administrator approval before a campaign can proceed. This tab manages blocklist entries: adding, deactivating, reactivating, and removing them.

### 9.2 Blocklist Tab Layout

```
┌──────────────────────────────────────────────────────────────────────────┐
│  Targets                                                                 │
├──────────────────────────────────────────────────────────────────────────┤
│  [Targets]  [Groups]  [Blocklist]                                       │
├──────────────────────────────────────────────────────────────────────────┤
│                                                         [+ Add Entry]    │
│  ┌────────────────────────────┐                                          │
│  │ 🔍 Search blocklist...      │                                          │
│  └────────────────────────────┘                                          │
├──────────────────────┬──────────┬───────────┬───────────────┬────────────┤
│ Pattern              │ Type     │ Status    │ Reason        │  ···       │
├──────────────────────┼──────────┼───────────┼───────────────┼────────────┤
│ ceo@company.com      │ Exact    │ ● Active  │ C-suite prot. │  ···       │
│ *@legal.company.com  │ Domain   │ ● Active  │ Legal dept    │  ···       │
│ *@*.vendor.com       │ Subdomain│ ○ Inactive│ Former vendor │  ···       │
│ cfo@company.com      │ Exact    │ ● Active  │ C-suite prot. │  ···       │
├──────────────────────┴──────────┴───────────┴───────────────┴────────────┤
│  Showing 1–25 of 8                                                       │
└──────────────────────────────────────────────────────────────────────────┘
```

### 9.3 Table Columns

| Column | Width | Content | Sortable |
|--------|-------|---------|----------|
| Pattern | flex | The blocklist pattern (email or wildcard) | Yes |
| Type | 100px | Pattern type: "Exact", "Domain", or "Subdomain" — auto-detected from pattern format | Yes |
| Status | 100px | Active (green dot) or Inactive (gray dot) | Yes |
| Reason | flex | Free-text reason for the blocklist entry. Truncated at 40 chars with tooltip. | No |
| Actions | 48px | Kebab menu (`···`) | No |

- Default sort: **Pattern ascending**.
- The Type column is derived from the pattern format:
  - `user@domain.com` → "Exact"
  - `*@domain.com` → "Domain"
  - `*@*.domain.com` → "Subdomain"

### 9.4 Kebab Menu Actions

| Action | Available When | Behavior |
|--------|----------------|----------|
| Deactivate | Status is Active | Sets the entry to inactive (section 9.6) |
| Reactivate | Status is Inactive | Sets the entry to active (section 9.6) |
| Edit Reason | Always | Opens inline edit for the reason field |
| Delete | Always | Opens delete confirmation modal (section 9.7) |

### 9.5 Add Blocklist Entry

Clicking "[+ Add Entry]" opens a modal:

```
┌─────────────────────────────────────────────────┐
│  Add Blocklist Entry                      [✕]   │
├─────────────────────────────────────────────────┤
│                                                  │
│  Pattern *                                       │
│  ┌─────────────────────────────────────────┐    │
│  │ e.g. ceo@company.com or *@domain.com    │    │
│  └─────────────────────────────────────────┘    │
│                                                  │
│  Detected type: Exact Email                      │
│                                                  │
│  Reason                                          │
│  ┌─────────────────────────────────────────┐    │
│  │ e.g. C-suite protection                 │    │
│  └─────────────────────────────────────────┘    │
│                                                  │
│  ℹ Matching targets will require Administrator  │
│  approval before campaigns can proceed.          │
│                                                  │
│                    [Cancel]  [Add to Blocklist]   │
└─────────────────────────────────────────────────┘
```

**Pattern validation:**
- Required field.
- Must match one of three formats:
  - Exact email: standard email format (e.g., `ceo@company.com`).
  - Domain wildcard: `*@domain.com` format.
  - Subdomain wildcard: `*@*.domain.com` format.
- Invalid patterns show inline error: "Enter a valid email address or wildcard pattern (*@domain.com or *@*.domain.com)."
- As the user types, the "Detected type" line updates in real-time to show which pattern type was recognized, or shows "Invalid pattern" in `--danger` color if the format is not recognized.

**Duplicate check:**
- On blur of the pattern field, call `POST /api/v1/blocklist/check` with the pattern.
- If an existing entry matches, show inline warning: "This pattern overlaps with an existing blocklist entry: '{existing_pattern}'."

**Submission:**
- On submit: `POST /api/v1/blocklist` with `{ "pattern": "...", "reason": "..." }`.
- On success: toast "Blocklist entry added.", modal closes, table refreshes.
- On 409 Conflict (exact duplicate pattern): inline error "This exact pattern already exists in the blocklist."

### 9.6 Deactivate / Reactivate

**Deactivate** (from kebab menu on an active entry):
- No confirmation modal needed — immediate action.
- `PUT /api/v1/blocklist/{id}` with `{ "active": false }`.
- On success: status dot changes from green to gray, toast "Blocklist entry deactivated."
- Row does NOT move or disappear — it remains in place with updated status.

**Reactivate** (from kebab menu on an inactive entry):
- No confirmation modal needed — immediate action.
- `PUT /api/v1/blocklist/{id}` with `{ "active": true }`.
- On success: status dot changes from gray to green, toast "Blocklist entry reactivated."

### 9.7 Delete Blocklist Entry

```
┌─────────────────────────────────────────┐
│  Delete Blocklist Entry           [✕]   │
├─────────────────────────────────────────┤
│                                         │
│  Are you sure you want to delete the    │
│  blocklist entry for:                   │
│                                         │
│  ceo@company.com                        │
│                                         │
│  This entry will be permanently         │
│  removed and the pattern will no        │
│  longer require approval.               │
│                                         │
│              [Cancel]  [Delete]         │
└─────────────────────────────────────────┘
```

- Note: Despite the backend using soft delete for blocklist entries, the UI presents this as permanent removal since there is no restore mechanism exposed for blocklist entries.
- On confirm: `DELETE /api/v1/blocklist/{id}`.
- On success: toast "Blocklist entry deleted.", row removed from table.

### 9.8 Empty State

When no blocklist entries exist:

```
┌──────────────────────────────────────────────────────────────────────┐
│                                                                      │
│                    ┌─────────────────────┐                           │
│                    │   (Shield icon,     │                           │
│                    │    muted color)     │                           │
│                    └─────────────────────┘                           │
│                                                                      │
│                    No blocklist entries                               │
│                                                                      │
│         Add email addresses or domain patterns to require            │
│         administrator approval before those targets can              │
│         receive simulated phishing emails.                           │
│                                                                      │
│                       [+ Add Entry]                                  │
│                                                                      │
└──────────────────────────────────────────────────────────────────────┘
```

---

## 10. Target Restoration

### 10.1 Accessing Deleted Targets

Soft-deleted targets are visible when the Activity filter (section 1.6) is set to "Deleted" or "All".

### 10.2 Visual Treatment of Deleted Targets

When displayed in the table, soft-deleted targets have:

- All text in `--text-muted` color (reduced opacity).
- A subtle strikethrough decoration on the email column.
- A small "Deleted" badge in `--danger` background color next to the name, showing the deletion date on hover.
- The checkbox is still available for bulk selection.

### 10.3 Single Target Restoration

From the kebab menu of a deleted target, clicking "Restore":

- No confirmation modal — immediate action.
- `POST /api/v1/targets/{id}/restore`.
- On success: toast "Target restored.", the row's visual treatment updates to normal (muted style removed). If the Activity filter is set to "Deleted", the row animates out since it no longer matches the filter.

### 10.4 Bulk Restoration

When deleted targets are selected (Activity filter is "Deleted"), the floating action bar shows a "Restore" button instead of the standard bulk actions:

```
┌─────────────────────────────────────────────────────┐
│  5 selected (deleted)       [Restore All]     [✕]   │
└─────────────────────────────────────────────────────┘
```

- On click: calls `POST /api/v1/targets/{id}/restore` for each selected target (or a bulk restore endpoint if available).
- On success: toast "5 targets restored.", rows update or disappear based on active filter.

### 10.5 Mixed Selection

If the user has a mix of active and deleted targets selected (Activity filter is "All"), the floating action bar shows only actions applicable to the intersection:

- If all selected are active: standard bulk actions (section 8.2).
- If all selected are deleted: restore action only (section 10.4).
- If mixed: only "Export" is available. Other actions are disabled with tooltip explanations (e.g., "Cannot delete — selection includes already-deleted targets").

---

## 11. Error States and Edge Cases

### 11.1 Network Errors

- If `GET /api/v1/targets` fails due to network error or 5xx response: show a full-table error state with an illustration and "Failed to load targets. [Retry]" message.
- If a mutation (create, edit, delete) fails due to network error: show an error toast "Action failed. Please check your connection and try again." The modal or slide-over remains open so the user can retry.
- Retry behavior: the "Retry" button re-fires the last failed API call. No automatic retry.

### 11.2 Concurrent Editing

- If a user opens the edit slide-over for a target that another user has modified since it was loaded: the `PUT` request may return a 409 Conflict.
- On 409: show error toast "This target was modified by another user. Please close and reopen to see the latest version." The slide-over remains open but the "Save Changes" button is disabled.

### 11.3 Stale Data

- The target list auto-refreshes when the browser tab regains focus (visibility change event) if more than 60 seconds have elapsed since the last fetch.
- No WebSocket or polling for real-time updates.

### 11.4 Large Custom Field Values

- Custom field values up to 1024 characters are supported.
- In the detail slide-over, long values are displayed with word-wrap.
- In the edit slide-over, the value input expands to a textarea if the value exceeds 100 characters.
- If a custom field value exceeds 1024 characters on input, the textarea border turns `--danger` and an inline error "Value exceeds maximum length (1024 characters)" is shown. The submit button is disabled.

### 11.5 Empty and Null States

| Scenario | Display |
|----------|---------|
| Target with no name fields | Email is displayed as the primary identifier everywhere |
| Target with no department | "—" in table column, "Not set" in detail view |
| Target with no title | "—" in table column, "Not set" in detail view |
| Target with no groups | "0" in table column, "No groups assigned" in detail view |
| Target with no campaign history | "This target has not participated in any campaigns." in detail view |
| Target with no custom fields | "No custom fields defined." in detail view |
| Department filter with no departments | Dropdown shows "No departments found" (disabled state) |
| Group filter with no groups | Dropdown shows "No groups found" with a "Create Group" link |

### 11.6 Import Edge Cases

| Scenario | Behavior |
|----------|----------|
| CSV with no header row | Backend detects this and returns an error. Wizard shows: "No column headers detected. Ensure the first row contains column names." |
| CSV with only headers, no data | Backend returns `row_count: 0`. Wizard shows: "The CSV file contains no data rows." with a "Go Back" button. |
| CSV with >50 columns mapped to custom fields | Validation step returns errors for excess custom fields. Only the first 50 custom-mapped columns are accepted. |
| Import session timeout | If the user takes more than 30 minutes between steps, the backend may expire the import session. The wizard shows: "Your import session has expired. Please start over." with a "Start Over" button. |
| Browser closed mid-import | The import session remains on the server but the frontend state is lost. The next time the user opens import, a fresh session starts. Orphaned sessions are cleaned up by the backend. |
| Extremely large CSV (>5 MB) | Rejected at upload step with "File exceeds the 5 MB size limit." |

### 11.7 Blocklist Interaction

- When a target is created or edited, the system checks the blocklist but does NOT prevent the operation. The blocklist only affects campaign sends.
- If a target's email matches a blocklist entry, the detail slide-over shows an info banner: "This target matches a blocklist entry ({pattern}). Campaign sends to this target will require administrator approval."

---

## 12. Integration with Campaign Workspace

### 12.1 Target Selection from Campaign Targets Tab

As described in document 05 (Campaign Workspace, section 4), the campaign's Targets tab manages group assignment. This section describes how the target management UI integrates with that flow.

### 12.2 Group Selection Slide-Over (Campaign Context)

When the user clicks "[+ Add Group]" on the campaign's Targets tab, a slide-over opens listing available target groups:

```
┌───────────────────────────┬─────────────────────────────────────────┐
│                           │  ┌─────────────────────────────────────┐│
│  (Campaign workspace      │  │  Add Target Groups           [✕]   ││
│   dimmed behind overlay)  │  ├─────────────────────────────────────┤│
│                           │  │                                     ││
│                           │  │  ┌───────────────────────────────┐  ││
│                           │  │  │ 🔍 Search groups...            │  ││
│                           │  │  └───────────────────────────────┘  ││
│                           │  │                                     ││
│                           │  │  ☐  Executive Team        42 tgts  ││
│                           │  │  ☑  Marketing Dept        67 tgts  ││
│                           │  │  ☐  IT Support Staff      33 tgts  ││
│                           │  │  ☐  New Hires Q1          18 tgts  ││
│                           │  │  ☐  Sales Team            55 tgts  ││
│                           │  │                                     ││
│                           │  │  ─────────────────────────────────  ││
│                           │  │  Groups already assigned to this    ││
│                           │  │  campaign are checked and listed    ││
│                           │  │  first.                             ││
│                           │  │                                     ││
│                           │  │  Need a new group?                  ││
│                           │  │  [Open Target Management →]         ││
│                           │  │                                     ││
│                           │  │            [Cancel]  [Add Groups]   ││
│                           │  └─────────────────────────────────────┘│
└───────────────────────────┴─────────────────────────────────────────┘
```

- Groups already assigned to the campaign are pre-checked and sorted to the top.
- Checking/unchecking groups does not immediately modify the campaign — changes are applied when "Add Groups" is clicked.
- The "[Open Target Management]" link opens `/targets/groups` in a new browser tab so users can create or modify groups without leaving the campaign workspace.

### 12.3 Blocklist Check on Group Assignment

When groups are assigned to a campaign (via the campaign's Targets tab), the system performs an automatic blocklist check:

- The campaign workspace calls `POST /api/v1/campaigns/:id/blocklist-check` with the currently assigned group IDs.
- If any targets in the assigned groups match blocklist entries, a warning section appears on the campaign's Targets tab (as described in document 05, section 4.4).
- The warning lists each matching target with their email and the blocklist pattern that matched.
- The warning does NOT prevent campaign progression — it adds an Administrator approval requirement to the campaign's launch readiness checklist.

### 12.4 Cross-Navigation

The following cross-navigation paths exist between target management and the campaign workspace:

| From | To | Mechanism |
|------|-----|-----------|
| Campaign Targets tab → group name | Target Groups tab, filtered to that group | Link opens in new tab |
| Target detail slide-over → campaign name | Campaign workspace | Link opens in new tab |
| Campaign Targets tab → "Open Target Management" | Target management page | Link opens in new tab |
| Target list → Groups column tooltip → group name | Groups tab, filtered to that group | Client-side navigation |

All cross-navigation between the campaign workspace and target management opens in new browser tabs to prevent loss of campaign workspace state (which may contain unsaved changes).

---

## 13. Keyboard Shortcuts and Accessibility

### 13.1 Keyboard Navigation

| Key | Context | Action |
|-----|---------|--------|
| `/` | Target list (no input focused) | Focus the search input |
| `Escape` | Slide-over or modal open | Close the panel/modal (with unsaved changes guard if applicable) |
| `Escape` | Search input focused | Clear search and blur input |
| `Enter` | Target row focused | Open target detail slide-over |
| `Space` | Target row focused | Toggle row checkbox |
| `Tab` | Within slide-over forms | Move focus between form fields in tab order |

### 13.2 ARIA Labels

- Target table: `aria-label="Targets list"`.
- Search input: `aria-label="Search targets"`.
- Kebab menu buttons: `aria-label="Actions for {target email}"`.
- Floating action bar: `aria-label="Bulk actions for selected targets"` with `role="toolbar"`.
- Slide-over panels: `role="dialog"` with `aria-label` matching the panel title.
- Modals: `role="alertdialog"` for confirmation modals, `role="dialog"` for forms.

### 13.3 Focus Management

- When a slide-over opens, focus moves to the first interactive element inside it.
- When a slide-over closes, focus returns to the element that triggered it (the kebab menu button or the table row).
- When a modal opens, focus moves to the first interactive element. Focus is trapped within the modal until it is dismissed.
- When the floating action bar appears, focus is NOT moved to it (to avoid disorienting the user mid-selection).

---

## 14. Loading and Transition States

### 14.1 Initial Page Load

- The target table shows 8 skeleton rows (shimmer animation) while `GET /api/v1/targets` is in flight.
- Filters are rendered immediately (department dropdown shows a loading spinner inside until its data arrives from `GET /api/v1/targets/departments`).

### 14.2 Filter and Search Updates

- While a new API request is in flight due to filter/search changes, the existing table data remains visible with a subtle opacity reduction (0.6) and a thin progress bar at the top of the table area.
- When the response arrives, rows crossfade in with `--duration-smooth` (200ms).

### 14.3 Slide-Over Transitions

- Slide-over panels animate in from the right edge: translate-x from 100% to 0%, duration `--duration-smooth` (200ms), ease-out.
- On close: reverse animation (slide right to off-screen), then unmount.
- A semi-transparent overlay (`--bg-overlay`) covers the list behind the slide-over.

### 14.4 Row Mutations

- When a target is deleted: row fades out and collapses vertically over 200ms.
- When a target is created: if the new target would appear on the current page/sort, it slides in at the appropriate position. Otherwise, the table refreshes.
- When a target is edited: the row briefly flashes with `--bg-hover` background (200ms) to indicate the update.

### 14.5 Import Wizard Transitions

- Step transitions within the wizard use a horizontal slide: current step slides left while the next step slides in from the right, duration 250ms.
- The step indicator's connecting lines animate their fill color from left to right as steps complete.
