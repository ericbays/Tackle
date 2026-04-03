# 02 — Application Shell

This document specifies the persistent application shell — the layout frame that wraps every authenticated page in the Tackle admin UI. The shell consists of a collapsible sidebar, a top bar, a breadcrumb trail, and a main content area.

---

## 1. Overall Layout

The application shell uses a CSS Grid layout with three regions:

```
┌──────────────────────────────────────────────────────────┐
│                      Top Bar (56px)                      │
├──────────┬───────────────────────────────────────────────┤
│          │  Breadcrumbs (36px)                           │
│ Sidebar  │──────────────────────────────────────────────│
│ (240px   │                                               │
│  or 64px │  Main Content Area                            │
│  when    │  (scrolls independently)                      │
│ collapsed│                                               │
│  )       │                                               │
│          │                                               │
└──────────┴───────────────────────────────────────────────┘
```

- The sidebar is fixed on the left and does not scroll with page content.
- The top bar spans the full width above everything.
- The main content area fills the remaining viewport and scrolls independently.
- The breadcrumb strip sits between the top bar and the content area, within the content column.

---

## 2. Sidebar

### 2.1 States

The sidebar has two states:

| State | Width | Content |
|-------|-------|---------|
| Expanded | 240px | Logo + app name, grouped navigation items with text labels, pinned favorites, collapse toggle |
| Collapsed | 64px | Logo icon only, icon-only navigation items, expand toggle |

### 2.2 Toggle Behavior

- Toggle via a button at the bottom of the sidebar (chevron icon).
- Toggle via keyboard shortcut `Ctrl+/`.
- At viewport widths below 1024px, the sidebar auto-collapses to icon-only.
- At viewport widths below 768px, the sidebar is hidden entirely and becomes an overlay drawer toggled by a hamburger button in the top bar.
- Collapsed/expanded state is persisted in `localStorage` and restored on page load.
- Transition: `--duration-smooth` (300ms) with `--ease-in-out`. Text labels fade out before the width animates.

### 2.3 Header

- **Expanded**: The Tackle logo (from `supporting_images/logo.png`) at 32px height, with "TACKLE" text beside it in `--text-h4` weight.
- **Collapsed**: Logo only at 32px, centered horizontally.

### 2.4 Navigation Structure

Navigation items are organized into collapsible groups. Groups expand/collapse with a chevron toggle. Group collapse state is persisted in `localStorage`.

```
★ Pinned                         (user-configurable favorites)
  [Pinned items appear here]

Dashboard                        (LayoutDashboard icon)

▼ Operations
  Campaigns                      (Crosshair icon)
  Targets                        (Users icon)
  Email Templates                (Mail icon)
  Landing Pages                  (Globe icon)

▼ Infrastructure
  Domains                        (Link icon)
  SMTP Profiles                  (Server icon)
  Cloud & Endpoints              (Cloud icon)

▼ Insights
  Metrics                        (BarChart3 icon)
  Reports                        (FileText icon)
  Audit Logs                     (ScrollText icon)

▼ Administration
  Users & Roles                  (ShieldCheck icon)
  Settings                       (Settings icon)
```

### 2.5 Pinned Favorites

- Users can pin any navigation item to the "Pinned" section at the top of the sidebar.
- Pin/unpin via right-click context menu on any nav item → "Pin to sidebar" / "Unpin".
- Pinned items are stored in user preferences (server-side via `/api/v1/users/me/preferences`).
- The Pinned section is only visible when at least one item is pinned.
- Maximum 5 pinned items.

### 2.6 Role-Based Visibility

Navigation items are conditionally rendered based on the user's permissions from the JWT. Items the user cannot access are **not rendered in the DOM** (not hidden with CSS).

| Navigation Item | Required Permission | Visible To |
|----------------|-------------------|------------|
| Dashboard | (always visible) | All roles |
| Campaigns | `campaigns:read` | Admin, Operator, Engineer |
| Targets | `targets:read` | Admin, Operator |
| Email Templates | `templates.email:read` | Admin, Operator |
| Landing Pages | `landing_pages:read` | Admin, Operator, Engineer |
| Domains | `domains:read` | Admin, Engineer |
| SMTP Profiles | `smtp:read` | Admin, Engineer |
| Cloud & Endpoints | `cloud:read` | Admin, Engineer |
| Metrics | `metrics:read` | Admin, Operator, Engineer, Defender |
| Reports | `reports:read` | Admin, Operator, Engineer, Defender |
| Audit Logs | `logs.audit:read` | Admin, Engineer |
| Users & Roles | `users:read` | Admin |
| Settings | `settings:read` | Admin |

If a group section has no visible items for the current user, the entire group heading is also not rendered.

**The Defender role** sees only: Dashboard (Defender variant), Metrics, and Reports.

### 2.7 Active State

- The currently active navigation item has:
  - `--accent-primary` color for icon and text.
  - `--accent-primary-muted` background.
  - A 3px `--accent-primary` left border indicator.
- In collapsed mode, the active item shows the same left border and accent icon color.

### 2.8 Collapsed Behavior

- In collapsed mode, hovering over a nav icon shows a tooltip with the item label (500ms delay).
- Hovering over a group icon shows a flyout menu listing all items in that group.
- The pinned section shows only icons in collapsed mode.

---

## 3. Top Bar

### 3.1 Layout

The top bar is 56px tall, spans the full viewport width (above the sidebar), and uses `--bg-secondary` background with a `1px solid --border-default` bottom border.

```
┌──────────────────────────────────────────────────────────────┐
│  [☰]  TACKLE          [🔍 Search]  [🔔 3]  [● WS]  [Avatar ▾] │
└──────────────────────────────────────────────────────────────┘
```

**Left section:**
- Hamburger menu button (visible only at `< 768px` viewport — toggles overlay sidebar drawer).

**Center section:**
- Empty (reserved for future use).

**Right section (right-aligned, 8px gap between items):**
- **Search trigger**: A search icon button with "Search" label and `Ctrl+K` keyboard hint. Clicking opens the command palette.
- **Notification bell**: Bell icon with unread count badge. Clicking opens the notification panel.
- **WebSocket status indicator**: A small colored dot indicating connection state (see section 3.3).
- **User profile menu**: Avatar circle (initials-based, `--accent-primary` background) + username text + chevron-down icon. Clicking opens a dropdown.

### 3.2 Notification Panel

Clicking the notification bell opens a dropdown panel anchored to the bell icon, aligned to the right edge of the viewport.

**Panel specifications:**
- Width: 400px.
- Max height: 500px with scroll.
- Header: "Notifications" title + "Mark all read" link button.
- Each notification item displays:
  - Category icon (campaign, infrastructure, system, approval).
  - Title text (bold).
  - Body text (secondary color, truncated to 2 lines).
  - Relative timestamp ("2m ago", "1h ago").
  - Action link (navigates to related entity on click).
  - Unread indicator (blue dot on left edge).
- Clicking a notification marks it as read and navigates to the action link.
- "Mark all read" calls `POST /api/v1/notifications/read-all`.
- Empty state: "No notifications" with a bell-off icon.
- The unread count badge on the bell displays the count from `GET /api/v1/notifications/unread-count`, capped at "99+".

**Real-time updates:**
- New notifications arrive via WebSocket and are prepended to the list.
- The unread count badge updates instantly without polling.
- A brief pulse animation plays on the bell icon when a new notification arrives.

**Notification categories and their icons:**
- Campaign events (play-circle): started, completed, credential captured
- Infrastructure events (server): endpoint provisioned, endpoint down
- Approval requests (check-circle): pending approval, approved, rejected
- System alerts (alert-triangle): domain expiring, SMTP failure
- Admin actions (shield): user created, role changed

### 3.3 WebSocket Status Indicator

A small dot (8px diameter) next to the user avatar indicates the WebSocket connection state:

| State | Color | Tooltip |
|-------|-------|---------|
| Connected | `--success` (green) | "Real-time updates active" |
| Reconnecting | `--warning` (yellow) | "Reconnecting..." |
| Disconnected | `--danger` (red) | "Real-time updates unavailable" |

When disconnected, a non-blocking banner appears at the top of the content area: "Real-time updates unavailable. Reconnecting..." with `--warning-muted` background.

### 3.4 User Profile Dropdown

Clicking the avatar area opens a dropdown menu:

```
┌─────────────────────────┐
│  John Operator          │
│  john@company.com       │
│  Role: Operator         │
├─────────────────────────┤
│  Profile & Preferences  │
│  Active Sessions        │
│  Change Password        │
├─────────────────────────┤
│  Keyboard Shortcuts     │
├─────────────────────────┤
│  Log Out                │
└─────────────────────────┘
```

- "Profile & Preferences" navigates to the user preferences page.
- "Active Sessions" navigates to session management.
- "Change Password" opens a modal with current password, new password, and confirm password fields.
- "Keyboard Shortcuts" opens a modal listing all available shortcuts.
- "Log Out" calls `POST /api/v1/auth/logout`, clears tokens, and redirects to the login page.

---

## 4. Breadcrumb Navigation

### 4.1 Position

Breadcrumbs render immediately below the top bar, within the content column (not spanning the sidebar). Height: 36px. Background: transparent (content area background shows through). Bottom border: `1px solid --border-subtle`.

### 4.2 Structure

```
Dashboard  /  Campaigns  /  Operation Sunrise  /  Targets
```

- Each segment except the last is a clickable link (`--accent-primary` color on hover).
- The last segment displays the current page name in `--text-primary` (not clickable).
- Separator character: `/` in `--text-muted`.
- Dynamic segments (campaign names, target group names, template names) display the entity's human-readable name, not the UUID.
- If the entity name is longer than 30 characters, it is truncated with an ellipsis and the full name is shown in a tooltip.

### 4.3 Breadcrumb Updates

- Breadcrumbs update reactively on route change.
- The breadcrumb data is derived from the current route path and enriched with entity names from TanStack Query cache (no additional API call needed if the entity has been previously loaded).

---

## 5. Global Search (Command Palette)

### 5.1 Activation

- Click the search trigger in the top bar.
- Press `Ctrl+K` (Windows/Linux) or `Cmd+K` (macOS).
- The search dialog renders as a centered modal at `--z-command` (80) with a backdrop overlay.

### 5.2 Dialog Layout

```
┌──────────────────────────────────────────────┐
│  🔍 [Search campaigns, targets, templates...]│
├──────────────────────────────────────────────┤
│                                              │
│  Recent                                      │
│  ⏱ Operation Sunrise        Campaign        │
│  ⏱ john.doe@company.com     Target          │
│  ⏱ IT Password Reset        Email Template  │
│                                              │
│  ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─  │
│  When typing, results replace "Recent":      │
│                                              │
│  Campaigns                                   │
│  → Operation Sunrise         Active          │
│  → Q2 Security Test          Draft           │
│                                              │
│  Targets                                     │
│  → john.doe@company.com      Engineering     │
│                                              │
├──────────────────────────────────────────────┤
│  ↑↓ Navigate   ↵ Open   Esc Close           │
└──────────────────────────────────────────────┘
```

### 5.3 Behavior

- **Debounce**: 300ms after the user stops typing before sending the search query.
- **API endpoint**: `GET /api/v1/search?q={query}`.
- **Results grouping**: Results are categorized by entity type (Campaigns, Targets, Templates, Domains, Users) with a type badge next to each result.
- **Result display**: Each result shows the entity name, a type badge, and a status indicator (for campaigns: status badge color).
- **Keyboard navigation**: Arrow keys move the highlight, Enter opens the selected result, Escape closes the dialog.
- **Empty search**: Shows the 5 most recently visited pages (stored in `localStorage`).
- **No results**: "No results for '{query}'" with a search-x icon.
- **Loading**: A spinner replaces results while the search API is in flight.
- **Selecting a result**: Navigates to the entity's detail page and closes the dialog.

### 5.4 Search Scope

The search queries across all entity types the user has permission to read. The backend filters results based on the user's RBAC permissions.

---

## 6. Keyboard Shortcuts

### 6.1 Global Shortcuts

| Shortcut | Action |
|----------|--------|
| `Ctrl+K` / `Cmd+K` | Open search command palette |
| `Ctrl+/` | Toggle sidebar collapse |
| `Escape` | Close the topmost modal, dialog, or dropdown |
| `?` | Open keyboard shortcuts help dialog |

### 6.2 List Page Shortcuts

| Shortcut | Action |
|----------|--------|
| `N` | Create new entity (campaigns, targets, templates — if user has create permission) |

### 6.3 Detail Page Shortcuts

| Shortcut | Action |
|----------|--------|
| `E` | Enter edit mode (if user has update permission) |

### 6.4 Shortcut Rules

- Shortcuts are disabled when any text input, textarea, select, or code editor has focus.
- Shortcuts do not conflict with browser defaults (Ctrl+C, Ctrl+V, Ctrl+T, etc.).
- The `?` shortcut opens a modal listing all available shortcuts, organized by context (Global, List Pages, Detail Pages, Landing Page Builder).

---

## 7. Error Boundary

### 7.1 Page-Level Errors

If a page component throws an unhandled error, a React Error Boundary catches it and displays:

```
┌──────────────────────────────────────────────┐
│                                              │
│        (AlertTriangle icon, 48px)            │
│                                              │
│     Something went wrong                     │
│     An unexpected error occurred while       │
│     loading this page.                       │
│                                              │
│     [Try Again]   [Go to Dashboard]          │
│                                              │
│     Error ID: abc-123-def                    │
│     (small, muted, for support reference)    │
│                                              │
└──────────────────────────────────────────────┘
```

- "Try Again" resets the error boundary and remounts the component.
- "Go to Dashboard" navigates to `/dashboard`.
- The correlation ID from the failed request (if available) is displayed for support reference.
- The full error stack is logged to the browser console in development mode only.

### 7.2 Component-Level Errors

Individual components (charts, widgets, tables) that fail do not crash the entire page. They display a compact error state within their container:

```
┌─────────────────────────────┐
│  ⚠ Failed to load           │
│  [Retry]                    │
└─────────────────────────────┘
```
