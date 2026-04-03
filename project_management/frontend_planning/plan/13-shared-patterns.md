# 13. Shared UI Patterns

Behavioral specifications for reusable UI patterns applied across the entire Tackle phishing simulation platform. Every feature screen composes from these building blocks. Implementations must conform to these specs to maintain consistency.

---

## 1. Data Table Pattern

Tables are the primary way users browse campaigns, targets, templates, logs, and other collections. All tables use TanStack Table (v8) as the headless engine.

### Column Definition

```tsx
// Every table defines its columns as a typed array.
const columns: ColumnDef<Campaign>[] = [
  { accessorKey: "name", header: "Name", enableSorting: true },
  { accessorKey: "status", header: "Status", cell: StatusBadge },
  { accessorKey: "createdAt", header: "Created", cell: RelativeTime },
  { id: "actions", header: "", cell: KebabMenu, enableSorting: false },
];
```

- The first column is never the checkbox column when rendered visually; a dedicated selection column is prepended automatically when row selection is enabled.
- The last column is reserved for the kebab action menu. It has no header label, is not sortable, and is right-aligned.
- Column widths use a combination of `size`, `minSize`, and `maxSize` constraints. Text columns default to flexible width. Timestamp and status columns use fixed widths.

### Sorting

- Clicking a column header cycles: unsorted -> ascending -> descending -> unsorted.
- A small arrow icon (chevron-up / chevron-down) appears in the header to indicate sort direction. When unsorted, the icon is hidden (not grayed out).
- Only one column is sortable at a time (no multi-sort). Sorting a new column clears the previous sort.
- Sort state is synced to URL query params: `?sort=name&order=asc`. Navigating back restores the sort.
- Default sort is defined per table (e.g., campaigns sort by `createdAt desc`, targets sort by `name asc`).

### Filtering

- Filters live in a filter bar above the table (see section 14 for the filter bar pattern).
- The table re-fetches data whenever filter state changes. Filters are debounced (see section 14).
- Active filters are reflected in URL query params: `?status=active&search=phishing`.

### Pagination

- Default page size: 25 rows. Configurable per table (10, 25, 50, 100).
- Pagination controls are rendered below the table, right-aligned.
- Layout: `Showing 1-25 of 312 results` (left) | `Rows per page: [25 v]` | `< 1 2 3 ... 13 >` (right).
- Current page is synced to URL: `?page=2&pageSize=25`.
- When filters change, page resets to 1.
- When the current page exceeds total pages (e.g., after a delete), automatically navigate to the last valid page.
- Page numbers show: first page, last page, current page, and one page on each side of current. Gaps are shown as `...`.

### Row Selection

- A checkbox column is prepended when bulk actions are available.
- Header checkbox: unchecked = none selected, checked = all on current page selected, indeterminate = some selected.
- Clicking the header checkbox toggles between select-all-on-page and select-none.
- A "Select all 312 results" link appears above the table when all rows on the current page are selected, enabling cross-page selection.
- Selected row IDs are stored in component state (not URL params). Navigating away clears selection.
- Selected rows have a light highlight background (`bg-primary-50`).

### Kebab Menu (Row Actions)

- Trigger: a vertical three-dot icon button (`...`) in the last column of each row.
- Clicking opens a dropdown menu anchored to the button, aligned to the right edge.
- Menu items are text labels with optional leading icons. Destructive actions (Delete, Archive) are rendered in red text.
- The menu includes a visual separator (horizontal rule) before destructive actions.
- The menu closes on: clicking an item, clicking outside, pressing Escape, scrolling the table.
- Example menu for a campaign row:

```
  View Details
  Edit
  Duplicate
  ─────────────
  Archive        (red text)
  Delete         (red text)
```

- Keyboard: when the menu is open, arrow keys navigate items, Enter activates, Escape closes and returns focus to the trigger button.

### Table States

**Loading state:**
- Render the table header row normally (real column headers).
- Render 10 skeleton rows matching the column layout. Each cell contains a shimmer placeholder bar.
- Pagination controls show skeleton placeholders.
- Do not show a spinner overlaid on the table.

**Empty state (no data matches filters):**
- Render the table header row normally.
- Below the header, render a single centered cell spanning all columns.
- Content: a muted icon (e.g., search icon), text: "No results found", subtext: "Try adjusting your filters or search term", and a "Clear Filters" button if any filters are active.

**Empty state (no data exists at all):**
- Do not render the table header.
- Show the full empty state pattern (see section 10) with an action button to create the first item.

**Error state:**
- Render the table header row normally.
- Below the header, render a centered error message spanning all columns.
- Content: alert icon, text: "Failed to load data", a "Retry" button.
- Do not display raw API error messages.

### URL Parameter Synchronization

All table state is encoded in URL search params to enable shareable links and browser back/forward navigation.

```
/campaigns?search=quarterly&status=active&sort=createdAt&order=desc&page=2&pageSize=25
```

- Use `useSearchParams` (React Router) to read and write.
- Default values are omitted from the URL (e.g., page=1 is not shown).
- Invalid URL params (e.g., page=999 when only 5 pages exist) are silently corrected to valid values.
- Updating table state replaces the current history entry (not push) to avoid polluting browser history with every filter change. Exception: explicit pagination clicks use push so back-button returns to the previous page.

---

## 2. Floating Action Bar Pattern

The floating action bar (FAB) appears when one or more table rows are selected. It provides bulk actions.

### Appearance

- Position: fixed to the bottom center of the viewport, 24px above the bottom edge.
- Shape: rounded rectangle with shadow (`shadow-lg`), background `bg-gray-900`, text white.
- Height: 48px. Horizontal padding: 16px.
- Entrance animation: slides up from below the viewport over 200ms with ease-out.
- Exit animation: slides down over 150ms with ease-in when selection is cleared.

### Layout

```
[ X selected ] | [ Action 1 ] [ Action 2 ] [ Danger Action ] | [ X ]
```

- Left: selection count label (e.g., "3 selected" or "All 312 selected").
- Center: action buttons, separated by 8px gaps. Non-destructive actions use ghost-white buttons. Destructive actions use a red-tinted button.
- Right: dismiss button (X icon) that clears the selection.
- A vertical divider (thin white line, 50% opacity) separates the count, actions, and dismiss button.

### Behavior

- Appears when `selectedRows.length >= 1`.
- Disappears when selection is cleared (dismiss button, completing an action, or navigating away).
- Clicking a bulk action opens the appropriate confirmation dialog (see section 6).
- The bar does not scroll with the page; it remains fixed at the bottom.
- On small viewports (below 640px), action labels collapse to icon-only with tooltips.

---

## 3. Form Pattern

All forms use React Hook Form for state management and Zod for schema validation.

### Layout

- Forms use a single-column layout by default. Two-column layouts are used only for short related fields (e.g., first name / last name side by side).
- Max width of a form: 560px. Centered within its container (modal, slide-over, or page).
- Vertical spacing between fields: 24px.
- Vertical spacing between form sections (groups of related fields with a section heading): 32px.
- Section headings: `text-sm font-semibold text-gray-900` with a bottom border.

### Labels

- Position: above the input field.
- Typography: `text-sm font-medium text-gray-700`.
- Spacing between label and input: 6px.
- Required fields: a red asterisk (`*`) after the label text with `aria-hidden="true"`. Screen readers hear "required" via the `required` attribute on the input.
- Optional fields: append "(optional)" in regular weight gray text after the label. If most fields are required, mark only optional fields. If most are optional, mark only required fields.
- Helper text: below the label, above the input, in `text-xs text-gray-500`. Example: "Must be at least 8 characters."

### Field Spacing (Vertical Stack)

```
Label .................. text-sm font-medium
Helper text ............ text-xs text-gray-500 (optional, 2px below label)
[  Input field  ] ...... 6px below label/helper
Error message .......... text-xs text-red-600, 4px below input
                         24px gap to next field
```

### Input Fields

- Height: 40px for text inputs, selects, and date pickers.
- Border: 1px solid `border-gray-300`. On focus: `ring-2 ring-primary-500 border-primary-500`.
- Border radius: 8px.
- Padding: 12px horizontal.
- Placeholder text: `text-gray-400`, descriptive but not a substitute for labels.
- Disabled state: `bg-gray-50 text-gray-500 cursor-not-allowed`, with reduced opacity on the border.

### Validation Timing

1. **On mount:** No validation. Fields are pristine. No errors, no success indicators.
2. **On change (while typing):** No validation is triggered. Do not block or interrupt the user.
3. **On blur (leaving a field):** Validate the field. If invalid, show the error message. If valid, show the success checkmark.
4. **On subsequent changes (after a blur-triggered error):** Re-validate on every keystroke so the error clears as soon as the input becomes valid. This is the only case where on-change validation fires.
5. **On submit:** Validate all fields. Focus the first invalid field. Scroll it into view if necessary.

### Error Display

- Inline error message appears directly below the field, 4px gap.
- Text: `text-xs text-red-600`. Prefixed with a small alert-circle icon (14px).
- The field border changes to `border-red-500`. The ring on focus becomes `ring-red-500`.
- Error messages are specific and actionable: "Email address is not valid" not "Invalid input."
- Only one error message per field at a time (show the most relevant).

### Success Indicators

- After a field passes blur validation, a green checkmark icon (16px) appears inside the input field, right-aligned with 12px right padding.
- The field border changes to `border-green-500` momentarily (500ms), then returns to the default border.
- The checkmark remains visible until the field is edited again.

### Required Field Marking

- All required fields have `required` set in the HTML attribute for accessibility.
- The Zod schema is the source of truth for which fields are required.
- On submit, missing required fields show: "This field is required."

### Form Actions (Submit / Cancel)

- Positioned at the bottom of the form, right-aligned.
- Button order: `[ Cancel ]  [ Submit ]` (cancel on the left, submit on the right).
- Cancel is a ghost/secondary button. Submit is a primary button.
- While submitting: the submit button shows a spinner icon replacing the label text, is disabled, and the form fields are not disabled (the user can still read what they entered).
- After successful submission: the form closes (modal/slide-over) or navigates away, and a success toast appears.
- After failed submission: a form-level error banner appears above the form actions, and field-level errors from the server are mapped to specific fields.

---

## 4. Modal Pattern (Centered Overlay)

Used for confirmations, small forms (1-5 fields), and focused decisions.

### Sizes

| Size   | Width   | Use case                          |
|--------|---------|-----------------------------------|
| `sm`   | 400px   | Simple confirmations              |
| `md`   | 480px   | Short forms, info display         |
| `lg`   | 640px   | Longer forms, preview content     |

- Height is determined by content, up to a maximum of `calc(100vh - 96px)`. If content exceeds this, the modal body scrolls while header and footer remain fixed.
- Centered vertically and horizontally in the viewport.

### Backdrop

- Background: `bg-black/50` (black at 50% opacity).
- Clicking the backdrop closes the modal **only** if there are no unsaved changes. If a form has been modified, clicking the backdrop does nothing (the user must explicitly cancel or close).
- The backdrop fades in over 150ms.

### Close Behavior

- X button in the top-right corner of the modal header.
- Escape key closes the modal (same unsaved-changes guard as backdrop click).
- Cancel button in the footer.
- On close: if the form has unsaved changes, show a confirmation: "You have unsaved changes. Discard changes?" with "Keep Editing" and "Discard" buttons.

### Animation

- Entrance: the modal scales from 95% to 100% and fades in over 200ms with ease-out. The backdrop fades in simultaneously.
- Exit: the modal scales from 100% to 95% and fades out over 150ms with ease-in. The backdrop fades out simultaneously.

### Structure

```
┌─────────────────────────────────────┐
│ Title                          [X]  │  <- header, sticky
├─────────────────────────────────────┤
│                                     │
│  Modal body content                 │  <- scrollable if needed
│                                     │
├─────────────────────────────────────┤
│              [ Cancel ] [ Action ]  │  <- footer, sticky
└─────────────────────────────────────┘
```

- Header: `px-6 py-4`, title in `text-lg font-semibold`.
- Body: `px-6 py-4`, overflow-y auto.
- Footer: `px-6 py-4`, border-top, buttons right-aligned.

### Focus Management

- On open: focus is trapped within the modal. The first focusable element receives focus (or the primary action button for confirmations).
- On close: focus returns to the element that triggered the modal.
- Tab order cycles within the modal; it never escapes to the page behind.

---

## 5. Slide-Over Panel Pattern

Used for larger forms, detail views, and multi-step workflows that need more vertical space.

### Sizes

| Size   | Width   | Use case                          |
|--------|---------|-----------------------------------|
| `md`   | 480px   | Standard forms                    |
| `lg`   | 640px   | Complex forms, detail views       |
| `xl`   | 800px   | Multi-section forms, previews     |

- Height: full viewport height (`100vh`).
- Anchored to the right edge of the viewport.

### Backdrop

- Same as modal: `bg-black/50`, click-to-close with unsaved-changes guard.
- The backdrop fades in over 200ms.

### Close Behavior

- X button in the top-right corner of the panel header.
- Escape key (with unsaved-changes guard).
- Cancel button at the bottom.
- Same unsaved-changes confirmation as modals.

### Animation

- Entrance: the panel slides in from the right edge over 250ms with ease-out. It translates from `translateX(100%)` to `translateX(0)`.
- Exit: the panel slides out to the right over 200ms with ease-in.
- The backdrop fades in/out simultaneously.

### Structure

```
┌─────────────────────────┐
│ Title              [X]  │  <- header, sticky top
├─────────────────────────┤
│                         │
│  Panel body content     │  <- scrollable
│                         │
│                         │
│                         │
│                         │
├─────────────────────────┤
│    [ Cancel ] [ Save ]  │  <- footer, sticky bottom
└─────────────────────────┘
```

- Header: `px-6 py-4`, border-bottom. Title in `text-lg font-semibold`. Optional subtitle/description below the title in `text-sm text-gray-500`.
- Body: `px-6 py-6`, overflow-y auto.
- Footer: `px-6 py-4`, border-top, buttons right-aligned.

### Focus Management

- Same as modal: focus trapped, returns on close.

### Nesting

- Slide-over panels can open a centered modal on top (e.g., a delete confirmation while editing). The slide-over remains visible behind the modal backdrop.
- Slide-over panels do not nest within other slide-over panels. If a sub-form is needed, it is rendered inline within the existing panel.

---

## 6. Confirmation Dialog Pattern

A specialized modal used before irreversible or significant actions.

### Standard Confirmation

Used for: archiving, bulk actions, status changes.

```
┌─────────────────────────────────────────┐
│ ⚠ Archive Campaign                     │
├─────────────────────────────────────────┤
│                                         │
│  Are you sure you want to archive       │
│  "Q4 Security Awareness"?              │
│                                         │
│  This campaign will no longer send      │
│  emails. You can restore it later       │
│  from the archive.                      │
│                                         │
├─────────────────────────────────────────┤
│                [ Cancel ] [ Archive ]   │
└─────────────────────────────────────────┘
```

- Title: action-specific ("Archive Campaign", "Remove 3 Targets").
- Body: describe the specific item(s) affected and the consequences.
- Cancel button: secondary/ghost style. This is the default focused button (safe default).
- Action button: matches the intent. Destructive actions use `bg-red-600 text-white`. Non-destructive confirmations use the primary color.
- Escape and backdrop click trigger cancel.
- Size: `sm` (400px).

### High-Impact Confirmation (Typed Phrase)

Used for: deleting items permanently, removing all targets, actions that cannot be undone.

```
┌─────────────────────────────────────────┐
│ ⚠ Delete Campaign Permanently          │
├─────────────────────────────────────────┤
│                                         │
│  This will permanently delete           │
│  "Q4 Security Awareness" and all        │
│  associated data including:             │
│                                         │
│  - 1,247 tracking events               │
│  - 86 captured credentials             │
│  - 3 landing page submissions          │
│                                         │
│  This action cannot be undone.          │
│                                         │
│  Type "delete Q4 Security Awareness"    │
│  to confirm:                            │
│                                         │
│  [ _________________________________ ]  │
│                                         │
├─────────────────────────────────────────┤
│            [ Cancel ] [ Delete ]        │
└─────────────────────────────────────────┘
```

- The danger button is disabled until the user types the exact confirmation phrase.
- The confirmation phrase is shown in a monospace code-styled inline span.
- Matching is case-insensitive but whitespace-sensitive.
- The input field has no autocomplete (`autocomplete="off"`).
- Size: `md` (480px) to accommodate the extra content.

---

## 7. Toast Notification Pattern

Toasts provide non-blocking feedback about completed actions or system events.

### Types and Timing

| Type      | Icon           | Auto-dismiss | Color accent        |
|-----------|----------------|--------------|---------------------|
| `success` | check-circle   | 5 seconds    | Green left border   |
| `info`    | info-circle    | 5 seconds    | Blue left border    |
| `warning` | alert-triangle | 10 seconds   | Amber left border   |
| `error`   | x-circle       | Persistent   | Red left border     |

### Positioning and Stacking

- Position: fixed to the bottom-right corner of the viewport, 24px from the right edge, 24px from the bottom.
- Maximum visible toasts: 5. If a 6th toast arrives, the oldest auto-dismissable toast is removed immediately.
- Stacking order: newest toast appears at the bottom of the stack. Older toasts shift upward.
- Vertical gap between stacked toasts: 8px.

### Layout

```
┌──────────────────────────────────────────┐
│ ●  Campaign created successfully  [ X ] │
│    "Q4 Awareness" is now in draft.      │
│                              [ Undo ]   │
└──────────────────────────────────────────┘
```

- Width: 360px (fixed).
- Left border: 4px solid, colored by type.
- Background: white with `shadow-lg`.
- Border radius: 8px.
- Padding: 12px 16px.
- Title: `text-sm font-semibold text-gray-900`. Required.
- Description: `text-sm text-gray-600`. Optional, appears below the title.
- Close button: X icon in top-right, always visible.
- Action button (e.g., "Undo"): text-only link style, right-aligned below the description.

### Undo Action

- When a toast supports undo (e.g., after archiving, deleting, moving):
  - The toast auto-dismiss timer extends to 10 seconds (regardless of type) to give the user time to click Undo.
  - Clicking Undo: triggers the reverse API call, removes the toast, shows a new success toast ("Action undone").
  - If the undo window expires, the undo action is no longer available.

### Animation

- Entrance: slide in from the right over 200ms with ease-out, fading from 0 to 100% opacity.
- Exit: fade out over 150ms with ease-in. Remaining toasts shift down smoothly over 200ms.
- The auto-dismiss timer pauses when the user hovers over the toast and resumes on mouse leave.

### Programmatic API

```tsx
toast.success("Campaign created", {
  description: '"Q4 Awareness" is now in draft.',
  action: { label: "Undo", onClick: handleUndo },
});

toast.error("Failed to save template", {
  description: "Please try again or contact support.",
  // No auto-dismiss; user must close manually.
});
```

---

## 8. Loading State Pattern

No page or component should ever show a blank white area or a lone spinner. Loading states use skeleton placeholders.

### Skeleton Specifications

- Shape: rounded rectangles matching the approximate dimensions of the content they replace.
- Color: `bg-gray-200` base.
- Shimmer animation: a linear gradient highlight sweeps left to right continuously over 1.5 seconds.

```css
@keyframes shimmer {
  0% { background-position: -200% 0; }
  100% { background-position: 200% 0; }
}

.skeleton {
  background: linear-gradient(
    90deg,
    theme('colors.gray.200') 25%,
    theme('colors.gray.100') 50%,
    theme('colors.gray.200') 75%
  );
  background-size: 200% 100%;
  animation: shimmer 1.5s ease-in-out infinite;
  border-radius: 4px;
}
```

### Skeleton Variants

**Text line:** height 12px (for body text), 16px (for headings). Width varies (60%-90% of container) to look organic. Multiple lines stack with 8px gaps.

**Avatar/icon:** circle, 32px or 40px diameter.

**Button:** rounded rectangle matching button dimensions, 40px height.

**Card:** full card outline with skeleton elements inside (title bar, 2-3 text lines, a button-shaped bar at the bottom).

**Table row:** matches column layout. Each cell contains a skeleton bar sized to its content type (short for status, medium for name, narrow for date).

**Chart area:** a rectangular skeleton with a subtle wave shape inside to suggest a chart.

### Page-Level Loading

- The application shell (sidebar, top bar) is rendered immediately and never shows a skeleton. It is always present.
- The main content area shows skeletons shaped like the expected content.
- Page title and breadcrumbs render immediately if they can be derived from the route.

### Component-Level Loading

- Individual cards or widgets show their own skeletons independently.
- If the table is loading but the filter bar is ready, the filter bar renders normally and only the table body shows skeletons.

### Refetch / Background Loading

- When data is already displayed and a refetch happens (e.g., after a mutation, polling, or filter change), do NOT replace the existing content with skeletons.
- Instead, show a subtle progress indicator: a thin 2px progress bar at the very top of the content area (below the page header), using primary color, with an indeterminate animation.
- The existing data remains visible and interactive during the refetch.

---

## 9. Error State Pattern

### Page-Level Error

Shown when the primary API call for a page fails entirely.

```
         ┌─────────────────────────────┐
         │                             │
         │      [  ! alert icon  ]     │
         │                             │
         │  Something went wrong       │
         │                             │
         │  We couldn't load this      │
         │  page. Please try again.    │
         │                             │
         │       [ Retry ]             │
         │                             │
         └─────────────────────────────┘
```

- Centered vertically and horizontally within the content area.
- Icon: alert-triangle, 48px, `text-gray-400`.
- Title: `text-lg font-semibold text-gray-900`.
- Description: `text-sm text-gray-500`, max-width 320px, centered.
- Retry button: primary style.
- No raw error messages, status codes, or stack traces in production.
- In development mode only: a collapsible "Technical Details" section shows the error message and status code.

### Component-Level Error

When a widget or section within a page fails but the rest of the page is fine.

- The failing component renders an inline error within its boundaries.
- Layout: an alert icon, a short message ("Failed to load recent activity"), and a "Retry" link.
- Other components on the page remain functional.
- Typography: `text-sm text-gray-500`.
- Minimum height: the component maintains its expected height (or at least 120px) to prevent layout shift.

### Error Boundaries

- A React Error Boundary wraps each major page section.
- If a component throws a rendering error, the boundary catches it and shows the component-level error state.
- The error boundary includes a "Retry" button that remounts the failed component.
- Errors are logged to the application's error reporting service (e.g., Sentry) automatically.

### Network Error Handling

- **401 Unauthorized:** redirect to login page, clear auth state.
- **403 Forbidden:** show an inline message: "You don't have permission to view this." No retry button (retrying won't help).
- **404 Not Found:** show: "This [resource] doesn't exist or has been removed." with a link back to the list page.
- **429 Too Many Requests:** show: "You're making too many requests. Please wait a moment." with a retry button that has a countdown timer.
- **500+ Server Error:** show the generic page-level error with retry.
- **Network failure (no response):** show: "Unable to connect. Check your internet connection and try again."

---

## 10. Empty State Pattern

### Empty with Action

Shown when a collection has zero items and the user has permission to create one.

```
         ┌──────────────────────────────┐
         │                              │
         │     [ illustration/icon ]    │
         │                              │
         │    No campaigns yet          │
         │                              │
         │    Create your first         │
         │    phishing campaign to      │
         │    start testing your        │
         │    organization's security.  │
         │                              │
         │    [ + Create Campaign ]     │
         │                              │
         └──────────────────────────────┘
```

- Centered vertically and horizontally within the content area.
- Icon or illustration: 64px, muted colors (`text-gray-300`). Use a contextual icon (e.g., inbox for campaigns, users for targets, mail for templates).
- Title: `text-lg font-semibold text-gray-900`.
- Description: `text-sm text-gray-500`, max-width 360px, centered. Explains what the resource is and what to do next.
- Action button: primary style. Uses a `+` icon prefix. Only shown if the user has the `create` permission for this resource type.

### Empty Without Action

Shown when the user doesn't have permission to create, or when it's a read-only collection.

- Same layout as above, but omit the action button.
- Description adjusts: "No campaigns have been created yet. Contact your administrator to get started."

### Filtered Empty

Shown when a search/filter returns zero results (but data does exist in the unfiltered state).

- This is handled within the table (see section 1, empty state for no filter matches).
- It is not the same as the full empty state pattern. The table header remains visible, and a "Clear Filters" button is the primary action.

---

## 11. WebSocket Integration Pattern

WebSocket connections provide real-time updates for campaign status changes, new submissions, and system events.

### Connection Management

```tsx
// Single WebSocket connection per authenticated session.
// Managed by a top-level WebSocketProvider.
const ws = useWebSocket({
  url: `${WS_BASE_URL}/ws`,
  token: authToken,
  reconnect: true,
});
```

- Connect on successful authentication. Disconnect on logout.
- Authentication: pass the JWT as a query param or in the first message after connection opens.
- The connection is established once and shared across all components via React context.

### Reconnection Strategy

- On disconnect: attempt reconnection immediately, then with exponential backoff.
- Backoff schedule: 1s, 2s, 4s, 8s, 16s, 30s (max). Jitter of +/- 500ms is added to each interval.
- Maximum reconnection attempts: unlimited. The connection retries indefinitely.
- During reconnection: show a subtle banner at the top of the page: "Reconnecting..." with a pulsing dot indicator. This banner disappears once the connection is re-established.
- After reconnection: the client sends a `sync` message requesting any events missed during the disconnection window. The server responds with a batch of missed events.

### Event Handling

Events arrive as JSON messages with a standard envelope:

```json
{
  "type": "campaign.status_changed",
  "payload": { "campaignId": "abc-123", "status": "active" },
  "timestamp": "2026-04-03T10:30:00Z"
}
```

**Silent data updates (default behavior):**
- When a WebSocket event arrives that affects data currently displayed on screen, update the data in place silently.
- Use React Query's `queryClient.setQueryData` to patch the cache directly, or call `queryClient.invalidateQueries` to trigger a background refetch.
- The UI updates without any user-visible notification. No toast, no flash, no animation.
- Example: a campaign status changes from "Scheduled" to "Active" -- the status badge in the table updates in place.

**Toast for critical events only:**
- Show a toast notification for events that require user attention or are unexpected.
- Critical events include: campaign errors, infrastructure failures, credential captures (if configured), scheduled send failures.
- The toast uses the appropriate type (warning, error) and includes a link to the relevant detail page.

### Cache Invalidation

```tsx
// Map WebSocket event types to React Query cache keys.
const eventToQueryMap: Record<string, string[]> = {
  "campaign.status_changed":  ["campaigns"],
  "campaign.event_received":  ["campaigns", "dashboard-stats"],
  "target.imported":          ["targets", "target-groups"],
  "template.updated":         ["templates"],
};
```

- When an event arrives, invalidate all related query keys.
- Use `invalidateQueries` (background refetch) rather than manually patching the cache, unless the event payload contains the complete updated object.
- If the user is on a detail page for the specific resource that was updated, prefer `setQueryData` to update instantly.

### Heartbeat

- The client sends a `ping` message every 30 seconds.
- If no `pong` is received within 5 seconds, consider the connection dead and begin reconnection.

---

## 12. Form Validation Pattern

### Client-Side Validation (Zod)

Every form has a Zod schema that defines all validation rules.

```tsx
const campaignSchema = z.object({
  name: z
    .string()
    .min(1, "Campaign name is required")
    .max(100, "Campaign name must be 100 characters or fewer"),
  templateId: z
    .string()
    .min(1, "Please select an email template"),
  scheduledAt: z
    .date()
    .min(new Date(), "Scheduled date must be in the future")
    .optional(),
  targetGroupIds: z
    .array(z.string())
    .min(1, "Select at least one target group"),
});

type CampaignFormData = z.infer<typeof campaignSchema>;
```

- Zod schemas are the single source of truth. No additional validation logic outside the schema.
- Custom refinements handle cross-field validation (e.g., "end date must be after start date").
- Error messages are defined inline in the schema, not in the component.

### Validation Timing Integration

React Hook Form is configured with `mode: "onBlur"` and `reValidateMode: "onChange"`:

```tsx
const form = useForm<CampaignFormData>({
  resolver: zodResolver(campaignSchema),
  mode: "onBlur",
  reValidateMode: "onChange",
  defaultValues: { name: "", templateId: "", targetGroupIds: [] },
});
```

- `mode: "onBlur"`: first validation of a field occurs when the user leaves it.
- `reValidateMode: "onChange"`: once a field has an error, it re-validates on every change so the error clears immediately when fixed.

### Server-Side Error Mapping

When the server returns validation errors (HTTP 422), map them to form fields:

```tsx
// Server returns:
// { errors: [{ field: "name", message: "Campaign name already exists" }] }

const onSubmit = async (data: CampaignFormData) => {
  try {
    await createCampaign(data);
  } catch (error) {
    if (error.status === 422 && error.body.errors) {
      error.body.errors.forEach(({ field, message }) => {
        form.setError(field as keyof CampaignFormData, {
          type: "server",
          message,
        });
      });
    } else {
      // Non-field-specific error: show in form-level error banner.
      form.setError("root", {
        type: "server",
        message: "An unexpected error occurred. Please try again.",
      });
    }
  }
};
```

### Field-Level vs Form-Level Errors

- **Field-level:** errors that map to a specific input (e.g., "Name already exists"). Displayed inline below the field.
- **Form-level:** errors that don't map to a specific field (e.g., "You have exceeded your campaign limit"). Displayed as a banner above the form action buttons.

```
┌─ Form-level error banner ──────────────────────┐
│  ! You have exceeded your monthly campaign     │
│    limit. Upgrade your plan to continue.       │
└────────────────────────────────────────────────┘
                          [ Cancel ] [ Submit ]
```

- The banner uses `bg-red-50 border border-red-200 text-red-800 rounded-lg p-4`.
- An alert-circle icon appears at the left of the message.

---

## 13. Pagination Pattern

### Page-Based Pagination (Default)

Used for most tables where the total count is known.

```tsx
// API request:
GET /api/campaigns?page=2&pageSize=25&sort=createdAt&order=desc

// API response:
{
  "data": [...],
  "meta": {
    "page": 2,
    "pageSize": 25,
    "totalCount": 312,
    "totalPages": 13
  }
}
```

- The UI renders page controls below the table (see section 1 for layout).
- `page` and `pageSize` are synced to URL search params.
- When `totalCount` is expensive to compute, the server may return `totalCount: null`. In this case, show "next" and "previous" buttons without page numbers.

### Cursor-Based Pagination (Infinite Scroll)

Used for: activity feeds, audit logs, real-time event streams.

```tsx
// API request:
GET /api/audit-logs?cursor=eyJpZCI6MTIzfQ&limit=50

// API response:
{
  "data": [...],
  "meta": {
    "nextCursor": "eyJpZCI6MTczfQ",
    "hasMore": true
  }
}
```

- Managed by React Query's `useInfiniteQuery`.
- A "Load More" button or an intersection observer triggers fetching the next page.
- When using an intersection observer: place a sentinel div at the bottom of the list. When it enters the viewport, fetch the next page automatically.
- Show a spinner (inline, small) below the last item while the next page loads.
- The URL does not reflect cursor state (cursors are opaque and not user-facing).

### URL Synchronization

- Page-based tables sync `page` and `pageSize` to URL params.
- Default values (page=1, pageSize=25) are omitted from the URL.
- When URL params are manually edited to invalid values, silently correct:
  - `page=0` or `page=-1` becomes page 1.
  - `page=999` (beyond total pages) becomes the last page.
  - `pageSize=7` (not in the allowed list) becomes the nearest valid option (10).

---

## 14. Search and Filter Bar Pattern

The search/filter bar sits between the page header and the table.

### Layout

```
┌──────────────────────────────────────────────────────────┐
│ [ 🔍 Search targets...          ] [ Status v ] [ + Filter ] │
└──────────────────────────────────────────────────────────┘
  Active filters: [ Status: Active  x ] [ Group: IT  x ]  [ Clear all ]
```

- Search input: left-aligned, takes available space (flex-grow), max-width 320px. A magnifying glass icon is inside the input on the left.
- Predefined filter dropdowns: to the right of search, each as a dropdown button showing the filter name. When a value is selected, the button updates to show the selected value.
- "+ Filter" button: opens a dropdown listing additional filterable fields not shown by default.
- Active filter pills: rendered below the bar when any filter is active.

### Search Debounce

- Debounce delay: 300ms after the user stops typing.
- While debouncing, no loading indicator is shown in the search field.
- After debounce fires, the table enters its refetch state (thin progress bar, not skeletons).
- Minimum search length: 1 character. Empty search returns unfiltered results.
- The search input has a clear button (X icon) when text is present.

### Filter Dropdowns

- Each filter dropdown shows its available options as a list.
- Options are fetched from the API (e.g., available statuses, groups).
- Multi-select filters use checkboxes. Single-select filters use radio-style selection.
- Dropdowns close when a selection is made (single-select) or when clicking outside (multi-select).
- Keyboard: arrow keys navigate options, Enter/Space toggles selection, Escape closes.

### Active Filter Pills

- Each active filter is shown as a pill/chip: `[ Label: Value  x ]`.
- The `x` button removes that specific filter.
- "Clear all" link appears after the pills and removes all active filters at once.
- Pills appear on a row below the filter bar. If no filters are active, this row is not rendered (no empty space).

### URL Synchronization

- All filter state is reflected in URL search params.
- Search: `?search=john`.
- Filters: `?status=active,paused&group=engineering`.
- Changing any filter resets pagination to page 1.

---

## 15. Tag Input Pattern

Used for: adding tags to campaigns, assigning categories, adding email addresses to a list.

### Layout

```
┌─────────────────────────────────────────────────────┐
│ [ Phishing x ] [ Awareness x ] [ type to add... ] │
└─────────────────────────────────────────────────────┘
```

- Tags are rendered as inline pills within the input container.
- Each tag has a label and an `x` remove button.
- The text input cursor appears after the last tag.
- The container grows vertically to accommodate multiple rows of tags.

### Autocomplete

- As the user types, a dropdown appears below the input showing matching existing tags.
- Matching is case-insensitive and matches anywhere in the tag name (not just prefix).
- Maximum dropdown items: 10. If more exist, show "and 5 more..." at the bottom.
- Keyboard: arrow keys navigate suggestions, Enter selects the highlighted suggestion, Escape closes the dropdown.

### Creating New Tags

- If the user types a value that doesn't match any existing tag and presses Enter or comma:
  - The new tag is created inline immediately.
  - The tag appears with a subtle "new" indicator (e.g., a dotted border) until the form is saved.
  - Alternatively, if new tag creation is not allowed, show a message: "No matching tags found."

### Removing Tags

- Click the `x` button on a tag to remove it.
- Press Backspace when the text input is empty to remove the last tag (with a visual highlight on the tag before removal as a confirmation cue -- first Backspace highlights, second Backspace removes).
- Removed tags return to the autocomplete suggestions.

### Validation

- Duplicate tags are not allowed. If the user tries to add a duplicate, the existing tag briefly flashes/pulses to indicate it already exists.
- Maximum number of tags: configurable per field (default: no limit). When the limit is reached, the input is disabled and a helper message appears: "Maximum of 10 tags reached."

---

## 16. Date and Time Display Pattern

### Relative Time

Used when recency matters more than the exact timestamp.

| Age               | Display        |
|-------------------|----------------|
| < 1 minute        | "Just now"     |
| 1-59 minutes      | "5m ago"       |
| 1-23 hours        | "3h ago"       |
| 1-6 days          | "2d ago"       |
| 7-29 days         | "2w ago"       |
| 30+ days          | Absolute date  |

- Used in: table cells (Created, Last Modified), activity feeds, audit logs.
- Hover tooltip shows the full absolute timestamp (see below).
- Relative times do NOT live-update in the browser. They are calculated on render and when the component re-renders due to data changes.

### Absolute Time

Used when precision matters or the date is more than 30 days old.

| Context              | Format                        | Example                    |
|----------------------|-------------------------------|----------------------------|
| Table cell           | `MMM D, YYYY`                 | Apr 3, 2026                |
| Detail view          | `MMM D, YYYY [at] h:mm A`    | Apr 3, 2026 at 2:30 PM    |
| Tooltip on relative  | `MMM D, YYYY h:mm A (z)`     | Apr 3, 2026 2:30 PM (EDT) |
| Audit log            | `MMM D, YYYY h:mm:ss A (z)`  | Apr 3, 2026 2:30:45 PM (EDT) |
| ISO export/API       | `YYYY-MM-DDTHH:mm:ssZ`       | 2026-04-03T18:30:00Z      |

### Timezone Handling

- **Display:** All times are displayed in the user's local timezone by default. The timezone abbreviation is shown in tooltips and detail views.
- **Storage:** All times are stored and transmitted as UTC (ISO 8601 with Z suffix).
- **Conversion:** Use `Intl.DateTimeFormat` or a lightweight library (e.g., `date-fns` with `date-fns-tz`) for timezone conversion. Do not use Moment.js.
- **Campaign scheduling:** When scheduling a campaign, the user selects date and time in their local timezone. The UI shows their timezone name next to the time picker: "Schedule for: [Apr 5, 2026] [2:30 PM] EDT". The value is converted to UTC before sending to the API.
- **Timezone preference:** The user's timezone is detected from the browser (`Intl.DateTimeFormat().resolvedOptions().timeZone`). There is no manual timezone setting in user preferences; the browser timezone is always used.

### Date Picker

- Use a dropdown calendar component.
- Supports both date-only and date-time selection.
- For date-time: the calendar has a time input (hours:minutes, 12-hour format with AM/PM toggle) below the date grid.
- Min/max date constraints are visually communicated: dates outside the range are grayed out and unclickable.
- Today's date is highlighted with a subtle ring.
- The input field shows the selected date in the absolute format for the context (typically `MMM D, YYYY`).

---

## Cross-Cutting Concerns

### Animation Defaults

All transitions use the following defaults unless otherwise specified:

| Property                | Value            |
|-------------------------|------------------|
| Duration (enter)        | 200ms            |
| Duration (exit)         | 150ms            |
| Easing (enter)          | ease-out         |
| Easing (exit)           | ease-in          |
| Duration (slide panels) | 250ms enter, 200ms exit |
| Reduced motion          | Respect `prefers-reduced-motion`: disable all animations, use instant show/hide instead. |

### Keyboard Navigation

- All interactive patterns are fully keyboard accessible.
- Focus indicators: a visible 2px ring (`ring-2 ring-primary-500 ring-offset-2`) on all focusable elements.
- Tab order follows visual order (left-to-right, top-to-bottom).
- Dropdown menus, modals, and slide-overs trap focus within their boundaries.
- Escape closes the topmost overlay (modal > slide-over > dropdown).

### Responsive Behavior

- All patterns are designed for desktop-first (minimum viewport: 1024px).
- Tables: below 1024px, less critical columns are hidden. Below 768px, tables may convert to a card-based list layout.
- Modals: below 640px, modals become full-screen.
- Slide-overs: below 768px, slide-overs become full-screen.
- Toasts: below 640px, toasts span the full width, positioned at the bottom.
- Floating action bar: below 640px, labels collapse to icons with tooltips.

### Dark Mode Considerations

- All color references in this document (e.g., `bg-gray-200`, `text-red-600`) refer to light mode values.
- Dark mode equivalents are defined in the design system (document 01) and are applied via Tailwind's `dark:` variant.
- Patterns do not define separate dark-mode specs; they inherit from the design system's dark palette.
