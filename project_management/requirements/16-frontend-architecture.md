# 16 — Frontend Architecture

## 1. Overview

This document defines the frontend architecture for the Tackle platform. There are two fundamentally distinct React efforts in the project, each serving a different audience, with different security constraints, and requiring different technology choices. The framework admin UI is a feature-rich internal tool for the red team. The campaign landing pages are ephemeral, lightweight phishing pages generated per campaign and served to targets through transparent proxies.

These two concerns must never share a component library, CSS framework, or any detectable pattern that would allow a defender to correlate a landing page with the Tackle platform.

---

## 2. Two Distinct React Efforts

### 2.1 Framework Admin UI

| Property | Detail |
|----------|--------|
| **Purpose** | Internal management interface for the red team |
| **Users** | Authenticated team members (Admin, Engineer, Operator, Defender roles) |
| **Lifecycle** | Deployed with the framework server, long-lived, updated with releases |
| **Access** | Accessible only within the private lab network |
| **Theme** | Dark theme (no light mode for v1) |
| **Key capabilities** | Real-time WebSocket updates, complex forms, data dashboards, drag-and-drop builder, role-based views |

### 2.2 Campaign Landing Pages (Generated)

| Property | Detail |
|----------|--------|
| **Purpose** | Phishing pages served to targets via transparent proxy |
| **Users** | Phishing targets (unauthenticated, external) |
| **Lifecycle** | Generated per campaign, ephemeral, unique per build |
| **Access** | Publicly accessible through phishing endpoints |
| **Key constraints** | Lightweight, anti-fingerprinting, no detectable patterns, fast loading, self-contained |

### 2.3 Why Different Stacks

Using the same UI library (e.g., shadcn/ui, Material UI, Ant Design) for both admin and landing pages would create a detectable fingerprint. Defender tooling can scan page source for known library signatures — CSS class naming conventions, DOM structure patterns, bundled font files, and JavaScript runtime markers. If a landing page uses the same component library as the admin UI, a single detection rule could flag every Tackle campaign.

The landing page generator must produce diverse, unique output that does not share any structural or stylistic signature with the admin UI or with other landing page builds.

---

## 3. Admin UI Technology Stack

### 3.1 Recommended Stack

| Layer | Technology | Rationale |
|-------|-----------|-----------|
| **Build tool** | Vite | Fast HMR, modern ESM-native bundling, excellent TypeScript support |
| **Framework** | React 18+ with TypeScript | Type safety, large ecosystem, team familiarity |
| **Routing** | React Router v6+ | Client-side SPA routing, nested routes, lazy loading support |
| **Styling** | Tailwind CSS + shadcn/ui | Utility-first CSS, dark theme support, highly customizable headless components |
| **Server state** | TanStack Query (React Query) v5+ | Server state caching, background refetching, optimistic updates, WebSocket integration |
| **Client state** | Zustand | Lightweight global state for UI concerns (sidebar state, modals, user preferences) |
| **Forms** | React Hook Form + Zod | Performant form handling with schema-based validation |
| **Charts** | Recharts or Nivo | Composable charting for metrics dashboards |
| **WebSocket** | Native WebSocket with thin wrapper | Real-time event streaming for dashboards, logs, notifications |
| **HTTP client** | Axios or native fetch with interceptors | JWT token management, request/response interceptors, error handling |
| **Icons** | Lucide React | Consistent icon set matching shadcn/ui aesthetic |
| **Tables** | TanStack Table | Headless table with sorting, filtering, pagination, column resizing |
| **Drag and drop** | dnd-kit | Accessible drag-and-drop for the landing page builder |
| **Code editor** | Monaco Editor (React wrapper) | Embedded code editor for CSS/HTML editing in the landing page builder |

### 3.2 Project Structure

```
frontend/
  admin/
    src/
      main.tsx                  # Application entry point
      App.tsx                   # Root component, router setup
      routes/                   # Route definitions and lazy-loaded page components
      components/
        ui/                     # shadcn/ui components (Button, Dialog, etc.)
        layout/                 # Shell, Sidebar, TopBar, Breadcrumbs
        shared/                 # Reusable composite components
      features/
        dashboard/              # Dashboard feature module
        campaigns/              # Campaign management feature module
        templates/              # Email template editor feature module
        landing-pages/          # Landing page builder feature module
        targets/                # Target management feature module
        domains/                # Domain management feature module
        infrastructure/         # Cloud infrastructure feature module
        smtp/                   # SMTP configuration feature module
        metrics/                # Metrics and reporting feature module
        logs/                   # Audit/campaign/system log viewer
        settings/               # Application settings feature module
        users/                  # User and role management feature module
      hooks/                    # Custom React hooks
      lib/
        api/                    # API client, endpoint definitions, interceptors
        ws/                     # WebSocket client, event types, reconnection logic
        auth/                   # Auth context, token management, permission helpers
        utils/                  # Utility functions
        validators/             # Zod schemas shared across features
      types/                    # TypeScript type definitions
      stores/                   # Zustand stores for client-side state
      assets/                   # Static assets (images, fonts)
    public/                     # Public static files
    index.html                  # HTML entry point
    vite.config.ts              # Vite configuration
    tailwind.config.ts          # Tailwind configuration
    tsconfig.json               # TypeScript configuration
```

Each feature module follows a consistent internal structure:

```
features/campaigns/
  components/                   # Feature-specific components
    CampaignList.tsx
    CampaignDetail.tsx
    CampaignForm.tsx
    CampaignApprovalWorkflow.tsx
  hooks/                        # Feature-specific hooks
    useCampaigns.ts
    useCampaignMutations.ts
  types/                        # Feature-specific types
    campaign.types.ts
  utils/                        # Feature-specific utilities
  index.ts                      # Public API barrel file
```

---

## 4. Admin UI Requirements

### 4.1 Application Shell and Navigation

---

**REQ-FE-001: Application Shell Layout**

The admin UI MUST implement a persistent application shell consisting of a collapsible sidebar, a top bar, and a main content area.

**Acceptance Criteria:**
- The sidebar is fixed on the left side of the viewport and displays navigation items.
- The sidebar can be collapsed to an icon-only state and expanded via a toggle button or keyboard shortcut.
- The collapsed/expanded state persists across page navigations and browser sessions (stored in localStorage).
- The top bar spans the full width above the content area and displays the user profile menu, notification bell, search trigger, and WebSocket connection status indicator.
- The main content area fills the remaining viewport space and scrolls independently of the sidebar and top bar.

---

**REQ-FE-002: Role-Based Navigation**

Sidebar navigation items MUST be conditionally rendered based on the authenticated user's permissions.

**Acceptance Criteria:**
- Given a user with the Defender role, the sidebar displays only the Dashboard and Metrics navigation items.
- Given a user with the Operator role, the sidebar displays all campaign-related items (Dashboard, Campaigns, Templates, Landing Pages, Targets, Metrics, Logs) but hides infrastructure-specific items (Domains, Infrastructure, SMTP configuration, Users & Roles, Settings).
- Given a user with the Engineer role, the sidebar displays infrastructure and read-only campaign items.
- Given a user with the Admin role, the sidebar displays all navigation items.
- Navigation items that the user cannot access MUST NOT be rendered in the DOM (not merely hidden with CSS).
- Frontend navigation filtering is supplemental — the backend MUST enforce authorization on every API request regardless (see [02-authentication-authorization.md](02-authentication-authorization.md), REQ-RBAC-031).

---

**REQ-FE-003: Breadcrumb Navigation**

Every page MUST display a breadcrumb trail showing the user's current location within the navigation hierarchy.

**Acceptance Criteria:**
- Breadcrumbs render beneath the top bar and above the page content.
- Each breadcrumb segment is a clickable link to that level of the hierarchy (except the current page).
- Dynamic segments (e.g., campaign name, target group name) display the entity's human-readable label, not its ID.
- Breadcrumbs update reactively when navigating between pages.

---

**REQ-FE-004: Responsive Layout**

The admin UI MUST be desktop-first but functional on tablet-sized viewports (minimum 768px width).

**Acceptance Criteria:**
- At viewport widths below 1024px, the sidebar auto-collapses to icon-only mode.
- At viewport widths below 768px, the sidebar becomes an overlay drawer toggled by a hamburger button.
- All data tables switch to a card-based layout or horizontally scroll at narrow widths.
- The landing page builder (drag-and-drop canvas) is desktop-only and displays a message on viewports below 1024px indicating that a larger screen is required.
- No horizontal scrollbar appears on the overall page layout at any supported viewport width.

---

**REQ-FE-005: Global Search**

The top bar MUST include a search interface that allows quick navigation to any entity in the system.

**Acceptance Criteria:**
- Activating search (via click on search icon or `Ctrl+K` / `Cmd+K` keyboard shortcut) opens a command palette-style dialog.
- The search queries campaigns, targets, templates, domains, and users via a backend search API endpoint.
- Results are categorized by entity type and display a name, type badge, and status indicator.
- Selecting a result navigates to the entity's detail page.
- Search is debounced (300ms) to avoid excessive API calls.
- The search dialog supports keyboard navigation (arrow keys, Enter to select, Escape to close).

---

**REQ-FE-006: Notification System**

The top bar MUST display a notification bell with an unread count badge and a dropdown notification panel.

**Acceptance Criteria:**
- Notifications are delivered via WebSocket in real time and also fetched from a REST API endpoint on initial load.
- Notification types include: campaign events (started, completed, credential captured), infrastructure events (endpoint provisioned, endpoint down), approval requests (pending infrastructure requests), system alerts (domain expiring, SMTP failure), and admin actions (user created, role changed).
- Each notification displays a timestamp, message, and action link.
- Notifications can be marked as read individually or all at once.
- The unread count badge displays the count of unread notifications (capped at 99+).

---

### 4.2 Authentication and Authorization UI

---

**REQ-FE-007: Login Page**

The admin UI MUST present a login page for unauthenticated users.

**Acceptance Criteria:**
- The login page displays a local login form (username/email and password fields) and buttons for each configured external authentication provider (OIDC, FusionAuth, LDAP).
- External provider buttons are fetched dynamically from the API (only enabled providers appear).
- The login form performs client-side validation (non-empty fields) before submission.
- On authentication failure, a generic error message is displayed ("Invalid credentials") without revealing whether the username or password was incorrect.
- On success, the JWT access token and refresh token are stored securely and the user is redirected to the dashboard or their originally requested URL.
- The login page applies the dark theme but has no sidebar or top bar — it is a standalone layout.

---

**REQ-FE-008: First-Launch Setup Wizard**

When the application detects a fresh installation (no users exist), the frontend MUST present a setup wizard instead of the login page.

**Acceptance Criteria:**
- The setup wizard collects: username, email address, display name, password, and password confirmation.
- Password strength indicators display in real time as the user types, reflecting the configured password policy.
- On successful setup, the user is automatically logged in and redirected to the dashboard.
- After setup completes, the setup wizard is never shown again — the frontend checks for setup status on load and routes to login if setup is complete.

---

**REQ-FE-009: Token Management**

The frontend MUST manage JWT access tokens and refresh tokens transparently.

**Acceptance Criteria:**
- The access token is stored in memory (not localStorage or sessionStorage) to reduce XSS exposure.
- The refresh token is stored in an HTTP-only cookie set by the backend (the frontend does not directly handle the refresh token value).
- When an API request receives a 401 response, the frontend automatically attempts a token refresh before retrying the request.
- If the token refresh fails (refresh token expired or revoked), the user is redirected to the login page with a "Session expired" message.
- Concurrent requests that trigger 401 responses share a single refresh attempt (request queuing during refresh).

---

**REQ-FE-010: Permission-Based UI Rendering**

All interactive UI elements (buttons, forms, menu items, action links) MUST be conditionally rendered or disabled based on the user's permissions.

**Acceptance Criteria:**
- A `usePermissions()` hook (or equivalent) provides the current user's permission set to any component.
- A `<PermissionGate permission="campaigns:create">` wrapper component conditionally renders children.
- Destructive actions (delete, terminate) are hidden entirely for users without the required permission — not shown as disabled.
- Read-only views for users who lack write permissions display data without edit controls.
- Permission checks are performed against the permission list extracted from the JWT or fetched from `/api/v1/auth/me`.

---

### 4.3 Dashboard

---

**REQ-FE-011: Dashboard Overview Page**

The Dashboard MUST provide a high-level overview of system activity for the authenticated user, scoped to their role permissions.

**Acceptance Criteria:**
- The dashboard displays: count of active campaigns (with links), count of pending approval requests (Engineers and Admins only), total credentials captured today (with trend indicator), phishing endpoint health summary (online / degraded / offline counts), recent activity feed (last 20 events across campaigns, infrastructure, and user actions).
- All metrics update in real time via WebSocket subscriptions.
- Dashboard cards use consistent styling and display loading skeletons while data is fetching.
- Empty states display helpful messages (e.g., "No active campaigns. Create one to get started.") with action links where the user has permission.

---

**REQ-FE-012: Dashboard Charts**

The Dashboard MUST include charting widgets for key metrics.

**Acceptance Criteria:**
- A time-series line chart displays credential captures over time (last 7 days, 30 days, or custom range — selectable via dropdown).
- A bar chart displays capture rates by campaign (top 5 active campaigns).
- A donut chart displays email delivery status breakdown (delivered, bounced, opened, clicked) across all active campaigns.
- Charts render using Recharts or Nivo components with dark-theme-compatible color palettes.
- Charts display tooltips on hover with precise values.
- Charts are responsive and resize with their container.

---

### 4.4 Campaign Management

---

**REQ-FE-013: Campaign List View**

The Campaigns page MUST display a sortable, filterable, paginated table of all campaigns accessible to the user.

**Acceptance Criteria:**
- Columns include: campaign name, status (with color-coded badge), target count, capture count, capture rate, created by, created date, and actions.
- Status badges use consistent color coding: Draft (gray), Pending Approval (yellow), Approved (blue), Active (green), Paused (orange), Completed (teal), Archived (muted).
- Filtering supports: status, date range, created by, and free-text search on campaign name.
- Sorting is available on all columns.
- Pagination supports configurable page sizes (10, 25, 50, 100).
- The table uses TanStack Table for headless functionality.
- A "New Campaign" button is visible only to users with `campaigns:create` permission.

---

**REQ-FE-014: Campaign Create/Edit Form**

The campaign creation and editing workflow MUST use a multi-step form wizard.

**Acceptance Criteria:**
- The wizard steps are: (1) Basic Info — name, description, schedule, (2) Targets — select target groups or individuals, (3) Email Template — select or create email template, (4) Landing Page — select or create landing page, (5) Infrastructure — select endpoint and domain, (6) Review & Submit — summary of all selections with a submit/save button.
- Each step validates its fields before allowing progression to the next step.
- The user can navigate backward without losing entered data.
- Draft campaigns can be saved at any step for later completion.
- The form uses React Hook Form with Zod schemas for validation.
- Required fields are marked with an asterisk. Validation errors display inline beneath the relevant field.

---

**REQ-FE-015: Campaign Lifecycle Management**

The campaign detail view MUST provide lifecycle management controls appropriate to the campaign's current state and the user's permissions.

**Acceptance Criteria:**
- Available actions per state: Draft (Edit, Delete, Submit for Approval), Pending Approval (Approve, Reject — Engineers/Admins only), Approved (Launch, Edit), Active (Pause, Stop), Paused (Resume, Stop), Completed (Archive, Export Report), Archived (Export Report, view only).
- State transition buttons display confirmation dialogs for irreversible actions (Launch, Stop, Archive).
- The campaign detail view displays real-time metrics (target count, emails sent, emails opened, links clicked, credentials captured) updated via WebSocket.
- A campaign activity timeline shows chronological events (created, approved, launched, first capture, paused, completed).

---

### 4.5 Email Template Editor

---

**REQ-FE-016: Email Template List and Editor**

The Templates page MUST provide a list of email templates and an inline editor with preview capability.

**Acceptance Criteria:**
- The template list displays: template name, subject line, last modified date, created by, and usage count (number of campaigns using the template).
- The editor provides: a subject line field, a rich-text editor for the email body (WYSIWYG mode), a raw HTML editor toggle for advanced users, template variable insertion (e.g., `{{target.first_name}}`, `{{tracking_url}}`), and an attachment configuration panel.
- A live preview panel renders the email as it would appear to a target, with template variables replaced by sample data.
- The preview supports toggling between desktop and mobile viewport simulations.
- Templates can be duplicated, creating a copy with a "(Copy)" name suffix.
- The editor auto-saves drafts every 30 seconds to prevent data loss.

---

### 4.6 Landing Page Builder

---

**REQ-FE-017: Landing Page Builder — Canvas Layout**

The landing page builder MUST implement a three-panel layout: component palette, canvas area, and property editor.

**Acceptance Criteria:**
- The left panel displays a categorized component palette with draggable components.
- The center panel displays a live preview canvas where components can be arranged via drag-and-drop.
- The right panel displays the property editor for the currently selected component on the canvas.
- The canvas supports zoom controls (fit to screen, zoom in, zoom out, percentage selector).
- The canvas displays device-width guides (desktop: 1440px, tablet: 768px, mobile: 375px) for responsive design.
- Panels are resizable via drag handles.

---

**REQ-FE-018: Landing Page Builder — Component Palette**

The component palette MUST provide a library of reusable components that can be dragged onto the canvas.

**Acceptance Criteria:**
- Component categories include: Layout (container, grid, columns, spacer, divider), Navigation (navbar, footer, breadcrumb), Content (heading, paragraph, rich text, image, video embed), Forms (form container, text input, email input, password input, select dropdown, checkbox, radio button, submit button), Interactive (button, link, tab group, accordion), and Branding (logo placeholder, hero banner).
- Each component displays a thumbnail preview and label in the palette.
- Components support nesting (e.g., a form container can contain input fields and a submit button).
- Drag-and-drop uses dnd-kit and displays drop zone indicators on the canvas.

---

**REQ-FE-019: Landing Page Builder — Property Editor**

The property editor MUST allow operators to configure the selected component's properties, styles, and behavior.

**Acceptance Criteria:**
- The property editor displays context-sensitive fields based on the selected component type.
- Property categories include: Content (text, placeholder, label, alt text), Styling (colors, fonts, spacing, borders, backgrounds — editable via visual controls and raw CSS), Layout (width, height, alignment, display mode), Behavior (form action, post-capture action, link target), and Validation (required, pattern, min/max length — for form fields).
- Changes in the property editor are reflected immediately on the canvas (live preview).
- A "Custom CSS" section allows operators to write raw CSS for the selected component.

---

**REQ-FE-020: Landing Page Builder — Multi-Page Support**

The landing page builder MUST support creating multi-page landing page flows.

**Acceptance Criteria:**
- A page navigation bar displays at the top of the canvas showing all pages in the flow.
- Operators can add, remove, rename, and reorder pages.
- Each page has its own canvas with independent component layouts.
- Page transitions can be configured (e.g., form submission on page 1 navigates to page 2).
- Post-capture actions (redirect, display page, etc.) can be configured per page.

---

**REQ-FE-021: Landing Page Builder — Preview Mode**

The builder MUST provide a preview mode that renders the landing page exactly as a target would see it.

**Acceptance Criteria:**
- Preview mode hides the component palette, property editor, and builder chrome — only the rendered page is visible.
- Preview mode supports device simulation (desktop, tablet, mobile viewport widths).
- Form submissions in preview mode do not trigger actual credential capture — they display a "Preview: form submitted" toast.
- A toggle button or keyboard shortcut (`Ctrl+P` / `Cmd+P`) switches between builder and preview mode.

---

**REQ-FE-022: Landing Page Builder — CSS/Style Editor**

The builder MUST include an embedded code editor for global page CSS.

**Acceptance Criteria:**
- A global CSS editor (Monaco Editor) is accessible via a "Page CSS" tab in the property panel or a dedicated slide-out panel.
- The CSS editor supports syntax highlighting, auto-completion, and error indicators.
- CSS changes apply immediately to the canvas preview.
- The builder enforces no specific CSS framework — operators can write any valid CSS.

---

**REQ-FE-023: Landing Page Builder — Form Builder**

The form builder within the landing page builder MUST allow operators to create forms with configurable field types and field-to-category mappings.

**Acceptance Criteria:**
- Operators can add form fields by dragging input components from the palette into a form container.
- Each form field can be configured with: field name (HTML name attribute), label, placeholder, field type (text, email, password, hidden, etc.), validation rules, and capture category (identity, sensitive, MFA, custom) as defined in [08-credential-capture.md](08-credential-capture.md), REQ-CRED-019.
- The form builder displays a summary of all fields and their capture categories.
- Operators can configure the form's action endpoint and method (handled by the generated landing page app's Go backend).

---

### 4.7 Target Management

---

**REQ-FE-024: Target List and Group Management**

The Targets page MUST display targets in a searchable, filterable table and support organizing targets into groups.

**Acceptance Criteria:**
- The target list displays: name, email, department, title, group memberships, campaign participation count, and last activity date.
- Targets can be filtered by group, department, campaign association, and activity status.
- Target groups can be created, renamed, and deleted.
- Targets can be assigned to multiple groups via bulk selection.
- A "Block List" tab displays targets who are excluded from all campaigns (opted out, executive protection, etc.).

---

**REQ-FE-025: Target Import**

The Targets page MUST support bulk target import from CSV files.

**Acceptance Criteria:**
- The import wizard accepts a CSV file upload and displays a column mapping interface.
- The mapping interface lets the operator map CSV columns to target fields (name, email, department, title, custom fields).
- A preview step shows the first 10 rows of parsed data before import execution.
- Import validation reports errors (missing email, duplicate email, invalid format) per row.
- Operators can choose to skip or fix invalid rows.
- Import progress is displayed for large files (> 100 targets).
- Import results show a summary: imported count, skipped count, error count with details.

---

### 4.8 Domain Management

---

**REQ-FE-026: Domain Management Page**

The Domains page MUST display domain profiles and provide lifecycle management controls.

**Acceptance Criteria:**
- The domain list displays: domain name, status (color-coded badge), registrar, DNS provider, expiry date (with warning indicators per [03-domain-infrastructure.md](03-domain-infrastructure.md), REQ-DOM-011), campaign associations, and tags.
- Clicking a domain opens a detail view with: domain profile information, DNS record editor (inline table with CRUD controls per REQ-DOM-020–023), campaign association history, and an activity log filtered to this domain.
- A "Register Domain" button opens a modal with registrar selection, domain name input, and availability check (per REQ-DOM-006–007).
- An "Import Domain" button opens a form for adding externally registered domains (per REQ-DOM-018).

---

### 4.9 Infrastructure Management

---

**REQ-FE-027: Infrastructure Overview Page**

The Infrastructure page MUST display the status of all cloud provider configurations and phishing endpoint instances.

**Acceptance Criteria:**
- The page displays: cloud provider connection cards (AWS, Azure) showing connection status, region, and credential health. Phishing endpoint table showing: endpoint name, status (provisioning, running, stopped, terminated, error), IP address, associated campaign, domain, uptime, and actions (start, stop, terminate, view logs).
- Endpoint status updates in real time via WebSocket.
- A "Provision Endpoint" button initiates the provisioning workflow (requires Engineer role or submits an approval request for Operators).
- Endpoint detail view shows: instance metadata, traffic statistics, health check history, and associated campaign links.

---

### 4.10 SMTP Configuration

---

**REQ-FE-028: SMTP Configuration Page**

The SMTP page MUST allow Engineers and Admins to manage SMTP server configurations.

**Acceptance Criteria:**
- The page displays a list of configured SMTP servers with: name, host, port, encryption type (TLS/STARTTLS/None), status (tested / untested / error), and associated campaigns.
- A create/edit form collects: display name, host, port, encryption, username, password (masked), sender address, sender name, and rate limit settings.
- A "Test Connection" button sends a test email to a specified address and reports success or failure with diagnostic details.
- A "Send Test Email" button allows sending a test message using a selected email template to a specified address.

---

### 4.11 Metrics and Reporting

---

**REQ-FE-029: Metrics Dashboard**

The Metrics page MUST display interactive dashboards with campaign performance data.

**Acceptance Criteria:**
- Dashboard widgets include: email funnel visualization (sent -> delivered -> opened -> clicked -> captured), campaign comparison chart (side-by-side capture rates), time-to-capture histogram, geographic distribution of target interactions (if IP geolocation data is available), and top-performing templates/landing pages.
- Dashboards support date range filtering and campaign filtering.
- All charts support export as PNG or SVG.
- Data refreshes automatically via polling (30-second interval) or WebSocket events.

---

**REQ-FE-030: Report Builder**

The Metrics page MUST include a report builder for generating exportable campaign reports.

**Acceptance Criteria:**
- Operators can select one or more campaigns to include in a report.
- Report sections are configurable: executive summary, email delivery statistics, click statistics, credential capture statistics (with option to include or redact credential values), timeline visualizations, and target-level detail (with option to anonymize).
- Reports can be exported in PDF, CSV, and JSON formats.
- Generating a report with credential values requires Operator or Admin role and creates an audit log entry (per [08-credential-capture.md](08-credential-capture.md), REQ-CRED-015).
- Report generation for large campaigns runs asynchronously with a progress indicator and notification on completion.

---

### 4.12 Log Viewer

---

**REQ-FE-031: Audit Log Viewer**

The Logs page MUST display audit logs in a filterable, searchable, real-time streaming view.

**Acceptance Criteria:**
- The log viewer displays entries in a table with: timestamp, user, action, resource type, resource identifier, outcome (success/failure), IP address, and details (expandable).
- Filtering supports: date range, user, action type, resource type, and outcome.
- Free-text search searches across all log entry fields.
- A "Live" toggle enables real-time log streaming via WebSocket — new entries appear at the top of the table without manual refresh.
- When live streaming is active, a visual indicator (pulsing dot) confirms the WebSocket connection is active.
- Log entries are paginated with infinite scroll (loads older entries on scroll-down).
- Operators see only their own audit log entries. Engineers and Admins see all entries.

---

### 4.13 Settings

---

**REQ-FE-032: Application Settings Page**

The Settings page MUST provide configuration interfaces for system-wide settings, accessible to Admins only.

**Acceptance Criteria:**
- Settings sections include: authentication provider configuration (OIDC, FusionAuth, LDAP — create, edit, test, enable/disable per [02-authentication-authorization.md](02-authentication-authorization.md)), session settings (token lifetimes, max sessions per [02-authentication-authorization.md](02-authentication-authorization.md), REQ-AUTH-073), password policy settings (complexity, history per [02-authentication-authorization.md](02-authentication-authorization.md), REQ-AUTH-020–021), cloud provider credentials (AWS, Azure API keys), and general settings (application name, default time zone, data retention policies).
- Each settings section saves independently (not a single global save button).
- Changes that affect active sessions display a warning indicating when changes take effect.
- Sensitive fields (API keys, secrets) display masked values with a reveal toggle and are only sent to the backend when explicitly changed.

---

**REQ-FE-033: User Management Page**

The Users & Roles page MUST provide user account and role management interfaces.

**Acceptance Criteria:**
- The user list displays: username, display name, email, role, auth provider(s), last login, account status (active/locked), and actions.
- Admins can create local accounts, edit user profiles, assign roles, lock/unlock accounts, reset passwords, and delete accounts (subject to constraints in [02-authentication-authorization.md](02-authentication-authorization.md), REQ-AUTH-003).
- The role management tab displays built-in roles (read-only) and custom roles (editable).
- Custom role creation provides a permission matrix UI — a table of resource types vs. action types with checkboxes.
- The initial administrator account displays a visual indicator and its role cannot be changed or deleted from the UI.

---

### 4.14 UI/UX Requirements

---

**REQ-FE-034: Dark Theme**

The admin UI MUST implement a dark theme as the primary and only theme for v1.

**Acceptance Criteria:**
- All pages, components, dialogs, and overlays use a dark color palette.
- The color palette uses: a near-black background (#0a0a0a to #1a1a2e range), muted gray for secondary surfaces, high-contrast white/light gray for text, accent colors for status indicators and interactive elements, and error/warning/success semantic colors with sufficient contrast ratios.
- All text meets WCAG AA contrast requirements (4.5:1 for normal text, 3:1 for large text) against dark backgrounds.
- shadcn/ui components are configured with a dark theme by default.
- No light mode toggle is needed for v1.

---

**REQ-FE-035: Loading, Error, and Empty States**

Every data-driven view MUST implement loading, error, and empty states.

**Acceptance Criteria:**
- **Loading state**: Skeleton placeholders (shimmer effect) that match the shape of the expected content. Never show a blank page or a spinner alone.
- **Error state**: A centered error message with the error description, a "Retry" button, and (for 500 errors) a "Contact Administrator" message. API error details are not exposed to the user in production builds.
- **Empty state**: An illustration or icon, a descriptive message ("No campaigns yet"), and a primary action button ("Create Campaign") if the user has the required permission.
- These states are implemented via reusable wrapper components (e.g., `<DataView loading={...} error={...} empty={...}>`).

---

**REQ-FE-036: Toast Notifications**

Async operations MUST provide feedback via toast notifications.

**Acceptance Criteria:**
- Toasts display in the bottom-right corner of the viewport.
- Toast types: success (green), error (red), warning (yellow), info (blue).
- Toasts auto-dismiss after a configurable duration (default: 5 seconds for success/info, 10 seconds for warnings, persistent until dismissed for errors).
- Toasts support an "Undo" action button for reversible operations (e.g., "Target deleted. Undo").
- Multiple toasts stack vertically with the newest at the bottom.

---

**REQ-FE-037: Confirmation Dialogs**

All destructive actions MUST require explicit confirmation via a dialog.

**Acceptance Criteria:**
- Destructive actions include: deleting a campaign, target, template, domain, endpoint, user, or role; terminating an endpoint; launching a campaign; archiving a campaign; purging credential data.
- The confirmation dialog displays a clear description of the action and its consequences.
- High-impact actions (launching a campaign, purging credentials) require the user to type a confirmation phrase (e.g., the campaign name or "PURGE") before the confirm button becomes active.
- The confirm button uses a danger color (red) and the cancel button is visually prominent as the safe default.

---

**REQ-FE-038: Keyboard Shortcuts**

The admin UI MUST support keyboard shortcuts for common actions.

**Acceptance Criteria:**
- Global shortcuts: `Ctrl+K` / `Cmd+K` (search), `Ctrl+/` (toggle sidebar), `Escape` (close modal/dialog).
- Page-specific shortcuts: `N` (new entity on list pages), `E` (edit on detail pages), `?` (show keyboard shortcut cheat sheet).
- Shortcuts are discoverable via a help dialog triggered by `?`.
- Shortcuts do not conflict with browser defaults or screen reader commands.
- Shortcuts are disabled when a text input or editor has focus.

---

**REQ-FE-039: WebSocket Connection Management**

The admin UI MUST maintain a persistent WebSocket connection for real-time updates and display connection status.

**Acceptance Criteria:**
- On login, the frontend establishes a WebSocket connection to the backend with the JWT access token for authentication.
- A connection status indicator in the top bar displays: green dot for "Connected," yellow dot for "Reconnecting," and red dot for "Disconnected."
- On connection loss, the client implements automatic reconnection with exponential backoff (1s, 2s, 4s, 8s, 16s, 30s max).
- During disconnection, the UI displays a non-blocking banner: "Real-time updates unavailable. Reconnecting..."
- On reconnection, the client re-subscribes to all active topic channels and requests a delta of missed events since the last received event timestamp.
- WebSocket messages are typed using TypeScript discriminated unions for type-safe event handling.

---

### 4.15 State Management Patterns

---

**REQ-FE-040: Server State Management**

All server-sourced data MUST be managed via TanStack Query (React Query).

**Acceptance Criteria:**
- Each API resource has a dedicated query key factory (e.g., `campaignKeys.list(filters)`, `campaignKeys.detail(id)`).
- List queries support pagination, filtering, and sorting parameters that are reflected in the URL query string.
- Mutations (create, update, delete) use `useMutation` with `onSuccess` callbacks that invalidate relevant query caches.
- Optimistic updates are implemented for low-risk mutations (e.g., marking a notification as read).
- Stale time for list queries: 30 seconds. Stale time for detail queries: 60 seconds. These are configurable per query.
- Background refetching occurs on window focus and on WebSocket events that signal data changes.

---

**REQ-FE-041: Client State Management**

UI-only state (not derived from server data) MUST be managed via Zustand stores or React local state.

**Acceptance Criteria:**
- Zustand is used for cross-component UI state: sidebar collapsed state, active theme/preferences, modal/dialog visibility stack, landing page builder selection state (selected component, tool mode).
- React component-local state (`useState`, `useReducer`) is used for component-scoped concerns: form input values (managed by React Hook Form), tooltip visibility, dropdown open state.
- Server-derived data MUST NOT be duplicated into Zustand — TanStack Query is the source of truth for all server state.

---

### 4.16 Performance Requirements

---

**REQ-FE-042: Code Splitting and Lazy Loading**

The admin UI MUST implement route-based code splitting to minimize initial bundle size.

**Acceptance Criteria:**
- Each top-level route (Dashboard, Campaigns, Templates, etc.) is loaded as a separate chunk via `React.lazy()` and `Suspense`.
- The initial bundle (main chunk + vendor chunk) MUST NOT exceed 200 KB gzipped.
- Heavy dependencies (Monaco Editor, charting libraries, dnd-kit) are loaded only when their respective pages are accessed.
- A route-level loading fallback (skeleton or spinner) displays while chunks load.
- Prefetching is implemented for routes reachable from the current page's navigation links (on hover or on viewport idle).

---

**REQ-FE-043: Rendering Performance**

The admin UI MUST maintain responsive interaction under realistic data loads.

**Acceptance Criteria:**
- Tables with up to 1,000 rows render without jank. Tables exceeding 1,000 rows use virtualization (e.g., TanStack Virtual).
- The landing page builder canvas renders up to 100 components without perceptible lag (< 16ms per frame during drag operations).
- React DevTools Profiler shows no unnecessary re-renders on static pages (components re-render only when their data or state changes).
- Large forms (20+ fields) maintain instant input responsiveness (no observable delay between keystroke and display).

---

**REQ-FE-044: Asset Optimization**

All static assets MUST be optimized for production builds.

**Acceptance Criteria:**
- Vite production builds apply: tree shaking, minification (Terser or esbuild), CSS purging (Tailwind's JIT mode), asset hashing for cache busting, and gzip/brotli pre-compression.
- Images are served in WebP format where supported, with PNG/JPEG fallbacks.
- Font files are subset to include only the character sets in use.
- The Lighthouse performance score for the login page is 90+ and for the dashboard is 80+ (measured on a simulated desktop connection).

---

### 4.10 User Preferences

---

**REQ-FE-060: User Preferences Panel**

The admin UI MUST provide a comprehensive user preferences panel accessible from the user profile menu.

**Acceptance Criteria:**
- The preferences panel includes the following configurable settings:
  - **Notification preferences** — Per-category email notification opt-in/out, digest mode selection (immediate, hourly, daily, weekly). See [18-notification-system.md](18-notification-system.md).
  - **Dashboard defaults** — Default campaign filter, default date range, default metric view.
  - **Table preferences** — Default page size (25, 50, 100), default sort order per table, visible column selection per table.
  - **Timezone** — Display timezone for all timestamps in the UI (does not affect stored data, which is always UTC).
  - **Date/time format** — Choice between 12-hour and 24-hour time display, date format (ISO, US, EU).
- Preferences are stored server-side (database) and loaded on session start.
- Preferences are applied immediately without page refresh.
- All preferences have sensible defaults for new users.

---

### 4.11 Campaign Calendar View

---

**REQ-FE-061: Campaign Calendar Component**

The admin UI MUST provide a calendar-based view of campaigns as an alternative to the list view.

**Acceptance Criteria:**
- The calendar view is accessible from the Campaigns navigation section (tab or toggle alongside the list view).
- Month, week, and day views are available with navigation controls (previous/next, today button).
- Campaigns are rendered as date-range bars spanning their `start_date` to `end_date`.
- Campaign bars are color-coded by current state (Draft=gray, Pending Approval=orange, Active=green, Paused=yellow, Completed=blue, Archived=muted).
- Clicking a campaign bar navigates to the campaign detail view.
- Overlapping campaigns are visually stacked so all are visible.
- Scheduled launch times (REQ-CAMP-037) are indicated with a distinct marker on the calendar.
- Clicking an empty date creates a new campaign with that start date pre-populated.
- The calendar respects the same filters as the campaign list (state, Operator, archived toggle).

---

### 4.12 Dedicated Defender Dashboard

---

**REQ-FE-062: Defender Dashboard View**

The admin UI MUST provide a dedicated dashboard view optimized for the Defender role, focused on organizational risk metrics rather than campaign operations.

**Acceptance Criteria:**
- The Defender Dashboard is a distinct navigation item accessible to Defender, Engineer, and Administrator roles.
- The dashboard displays:
  - Overall organizational susceptibility score (% of targets who submitted credentials).
  - Susceptibility trend line across campaigns over time.
  - Department/group risk heatmap (color-coded by capture rate).
  - Campaign effectiveness comparison (bar chart of key metrics across campaigns).
  - Phishing report rate trend (% of targets who reported the email).
- The dashboard supports date range filtering and campaign selection.
- An "Include archived" toggle allows viewing historical data.
- The layout is optimized for presentation to security leadership (clean, high-level, no per-target detail).
- All data displayed is aggregate — no individual target names, emails, or credential values are shown.

---

### 4.13 Universal Tags

---

**REQ-FE-063: Universal Tagging System**

The admin UI MUST provide a universal tagging system that allows users to apply free-form tags to any primary entity in the system.

**Acceptance Criteria:**
- Tags can be applied to: campaigns, email templates, landing page projects, SMTP profiles, domains, target groups, and report templates.
- Tags are displayed as colored chips/badges on entity cards and list views.
- A tag input component allows adding tags via typing with autocomplete from existing tags.
- Tags are searchable — the global search (REQ-FE-005) includes tag matching.
- Tags are filterable — all list views support filtering by one or more tags (AND/OR logic toggle).
- Tag management: an admin view lists all tags in the system with usage counts, and allows renaming or deleting tags.
- Deleting a tag removes it from all associated entities.
- Tags are lowercase, alphanumeric with hyphens and underscores allowed, max 50 characters.
- Each entity can have up to 20 tags.
- Tags are stored in a dedicated `tags` table with a polymorphic join table (`entity_tags`) supporting any entity type.

---

## 5. Campaign Landing Pages — Generated Frontend

### 5.1 Generation Architecture

---

**REQ-FE-050: Landing Page Code Generation Engine**

The backend MUST include a code generation engine that produces unique, self-contained landing page applications from the landing page builder's output.

**Acceptance Criteria:**
- The generator accepts a landing page definition (component tree, styles, form configuration, post-capture actions) and produces a complete React + Go application.
- Each build produces structurally unique output — two builds from the same landing page definition MUST produce different source code (different variable names, different CSS approaches, different DOM structures).
- The generated output compiles into a standalone Go binary that embeds the built React frontend.
- The generated Go binary exposes an HTTP server that serves the landing page and handles form submissions (credential capture and transmission to the framework API).
- Build artifacts are stored in the database or filesystem and associated with the campaign.

---

**REQ-FE-051: Anti-Fingerprinting — CSS Diversity**

Each landing page build MUST use a randomized CSS strategy to prevent pattern-based detection.

**Acceptance Criteria:**
- The generator randomly selects from the following CSS approaches per build: inline styles (`style` attributes), CSS Modules (generated class names), utility classes (custom-generated, not Tailwind), styled-components (CSS-in-JS), plain CSS with randomized class names, and CSS custom properties (variables).
- The selected approach is applied consistently within a single build but varies between builds.
- No build produces CSS class names or patterns that match any known framework (Tailwind, Bootstrap, Material, Ant Design, shadcn/ui).
- Generated class names use random strings that vary in length and character set per build.

---

**REQ-FE-052: Anti-Fingerprinting — HTML Structure Diversity**

Each landing page build MUST produce a unique HTML structure.

**Acceptance Criteria:**
- The generator introduces structural variation: randomized nesting depth for wrapper elements, varied use of semantic vs. generic HTML elements (`<section>` vs. `<div>`), randomized `id` and `data-*` attribute names, varied comment insertion (or absence), and randomized whitespace and formatting in the output.
- Two builds from the same landing page definition produce HTML that is not structurally identical when compared at the DOM level.
- No HTML patterns or attributes match the admin UI's output.

---

**REQ-FE-053: Anti-Fingerprinting — JavaScript Minimization**

Generated landing pages MUST minimize their JavaScript footprint to reduce detectable patterns.

**Acceptance Criteria:**
- The generated JavaScript is limited to: form handling (submission interception, validation), page transitions (multi-page flows), and minimal UI interactions (show/hide, toggle).
- React is used in production-compiled form only — no development-mode markers, no React DevTools hook, no identifiable React runtime signatures in the final bundle.
- Alternatively, the generator MAY produce vanilla JavaScript (no React) for simple single-page landing pages, further reducing fingerprint surface.
- Total JavaScript bundle size for a generated landing page MUST NOT exceed 50 KB gzipped.
- No third-party analytics, tracking, or UI library scripts are included.

---

**REQ-FE-054: Self-Contained Build Output**

Each generated landing page MUST compile into a single self-contained Go binary.

**Acceptance Criteria:**
- The Go binary embeds all static assets (HTML, CSS, JS, images, fonts) using Go's `embed` package.
- The binary requires no external runtime dependencies.
- The binary starts an HTTP server on a configurable port.
- The binary communicates with the Tackle framework API for credential transmission and campaign event reporting.
- The binary supports TLS termination or operates behind the phishing endpoint's transparent proxy (TLS handled at the proxy layer).

---

**REQ-FE-055: Landing Page Asset Randomization**

Generated landing pages MUST randomize asset fingerprints.

**Acceptance Criteria:**
- Generated filename hashes are unique per build (not content-based hashes that would be identical across builds with the same content).
- Font files, if included, use subsetted versions with randomized font-family names in the CSS.
- Image assets are re-encoded with slightly varied compression parameters to produce different file hashes.
- The HTML `<head>` contents (meta tags, resource loading order, charset declaration placement) vary between builds.
- No `generator` meta tag or framework-identifying header is present.

---

### 5.2 Landing Page Performance

---

**REQ-FE-056: Landing Page Load Performance**

Generated landing pages MUST load quickly to match the performance expectations of the impersonated service.

**Acceptance Criteria:**
- First Contentful Paint (FCP) under 1.5 seconds on a 4G connection.
- Time to Interactive (TTI) under 2.5 seconds on a 4G connection.
- Total transfer size (all assets) under 500 KB for a typical landing page.
- The landing page is fully functional with JavaScript disabled for basic content display (progressive enhancement where feasible — forms require JavaScript).
- No render-blocking resources except the primary CSS file.

---

## 6. Shared Concerns

### 6.1 TypeScript and Code Quality

---

**REQ-FE-060: TypeScript Strict Mode**

The admin UI MUST use TypeScript in strict mode.

**Acceptance Criteria:**
- `tsconfig.json` enables `strict: true` (includes `strictNullChecks`, `noImplicitAny`, `strictFunctionTypes`, etc.).
- No `@ts-ignore` or `@ts-expect-error` comments are permitted without an accompanying explanation comment.
- All API response types are defined as TypeScript interfaces or types.
- All component props are typed (no `any` props).
- Event handlers and callback props use specific function signatures, not `Function` or `any`.

---

**REQ-FE-061: Linting and Formatting**

The admin UI codebase MUST enforce consistent code style via automated tooling.

**Acceptance Criteria:**
- ESLint is configured with: `eslint-config-react-app` or equivalent, `@typescript-eslint/recommended` rules, React Hooks rules (`eslint-plugin-react-hooks`), import ordering rules, and no unused variable warnings treated as errors.
- Prettier is configured for consistent formatting (single quotes, trailing commas, 2-space indent, 100 character line width).
- A pre-commit hook runs lint and format checks on staged files.
- CI builds fail on lint errors.

---

### 6.2 Testing

---

**REQ-FE-062: Unit and Component Testing**

The admin UI MUST include unit and component tests for critical functionality.

**Acceptance Criteria:**
- Testing uses Vitest (aligned with Vite) and React Testing Library.
- Minimum coverage targets: 80% line coverage for utility functions in `lib/`, 70% coverage for custom hooks, and component tests for all shared components in `components/ui/` and `components/shared/`.
- Tests follow the "Testing Library" philosophy — test user interactions and visible output, not implementation details.
- API calls in tests are mocked using MSW (Mock Service Worker) for realistic network simulation.
- Tests run in CI on every pull request.

---

**REQ-FE-063: End-to-End Testing**

Critical user workflows MUST be covered by end-to-end tests.

**Acceptance Criteria:**
- E2E tests use Playwright.
- Critical workflows tested: login flow (local and simulated OIDC), campaign creation wizard (full flow), landing page builder (add component, configure, preview), target import (CSV upload, mapping, import), and report generation and export.
- E2E tests run against a test environment with a seeded database.
- E2E tests are included in the CI pipeline but may run on a separate schedule (not on every commit).

---

### 6.3 Accessibility

---

**REQ-FE-064: Accessibility Baseline**

The admin UI MUST meet a baseline level of accessibility.

**Acceptance Criteria:**
- All interactive elements are keyboard-navigable (tab order, Enter/Space activation).
- All form fields have associated `<label>` elements or `aria-label` attributes.
- All images have `alt` attributes.
- Focus is managed correctly for modals (focus trap) and page transitions (focus moves to the main content area).
- Color is not the sole means of conveying information — status indicators use icons or text labels in addition to color.
- ARIA landmarks are used for the sidebar (`<nav>`), main content (`<main>`), and top bar (`<header>`).
- Axe accessibility audits report no critical or serious violations on key pages (login, dashboard, campaign list).

---

## 7. Security Considerations

| Concern | Mitigation |
|---------|-----------|
| XSS via user input | All user-provided content rendered via React's default escaping. `dangerouslySetInnerHTML` is used only for the email template preview and landing page preview, within sandboxed iframes. |
| JWT exposure | Access tokens stored in memory only, not localStorage. Refresh tokens are HTTP-only cookies. |
| CSRF | SameSite=Strict cookies. All state-changing requests require the JWT Bearer token. |
| Sensitive data in URL | Campaign IDs, target IDs, and credential IDs use UUIDs. No PII appears in URL paths or query strings. |
| Admin UI fingerprinting | The admin UI is served only within the private lab network. No external users see admin UI assets. |
| Landing page fingerprinting | Generated pages use randomized CSS, HTML, JavaScript, and asset hashes per build (REQ-FE-051 through REQ-FE-055). |
| Dependency supply chain | Lock files (`pnpm-lock.yaml` or `package-lock.json`) are committed. Dependency audit runs in CI. No CDN-loaded dependencies in production. |
| Source maps in production | Source maps are NOT included in production builds of the admin UI. |

---

## 8. Acceptance Criteria Summary

### Admin UI

- [ ] The application shell renders with sidebar, top bar, and content area on all supported viewport widths.
- [ ] Navigation items are correctly filtered by role — Defenders see only Dashboard and Metrics; Operators see campaign-related items; Engineers see infrastructure items; Admins see everything.
- [ ] Login, token refresh, and logout flows work correctly for local and external auth providers.
- [ ] The dashboard displays live-updating metrics via WebSocket.
- [ ] Campaign creation wizard validates each step and allows draft saving at any point.
- [ ] The landing page builder supports drag-and-drop placement, property editing, multi-page flows, and preview mode.
- [ ] The email template editor provides WYSIWYG and HTML editing with live preview.
- [ ] All data tables support sorting, filtering, and pagination.
- [ ] Toast notifications, confirmation dialogs, and loading/error/empty states are consistent across all pages.
- [ ] User preferences (timezone, date format, table defaults, notification settings) are configurable and persist across sessions.
- [ ] Campaign calendar view displays campaigns as date-range bars with state color-coding across month/week/day views.
- [ ] Dedicated Defender Dashboard displays organizational risk metrics without per-target detail.
- [ ] Universal tags can be applied to, searched by, and filtered on all primary entities.
- [ ] Keyboard shortcuts function as documented and do not conflict with browser defaults.
- [ ] WebSocket reconnection works automatically with exponential backoff.
- [ ] Initial bundle size is under 200 KB gzipped with lazy-loaded feature chunks.
- [ ] TypeScript strict mode is enabled with no `any` types in component props.
- [ ] Unit test coverage meets minimum thresholds and E2E tests pass for critical workflows.

### Landing Pages

- [ ] The code generation engine produces unique output for each build from the same definition.
- [ ] Generated CSS uses a randomized approach per build with no detectable framework signatures.
- [ ] Generated HTML structure varies between builds.
- [ ] Generated JavaScript is under 50 KB gzipped with no identifiable library markers.
- [ ] Generated output compiles into a self-contained Go binary with embedded static assets.
- [ ] Landing pages achieve FCP under 1.5s and TTI under 2.5s on a 4G connection.
- [ ] No shared code, class names, or structural patterns exist between the admin UI and generated landing pages.

---

## 9. Dependencies

| Dependency | Document | Nature |
|------------|----------|--------|
| Authentication & RBAC | [02-authentication-authorization.md](02-authentication-authorization.md) | Login UI, permission-based rendering, role-gated navigation, session management |
| Domain Management | [03-domain-infrastructure.md](03-domain-infrastructure.md) | Domain management UI, DNS record editor |
| SMTP Configuration | [04-email-smtp.md](04-email-smtp.md) | SMTP configuration forms, test email sending |
| Landing Page Builder | [05-landing-page-builder.md](05-landing-page-builder.md) | Landing page builder is a major feature module in the admin UI |
| Campaign Management | [06-campaign-management.md](06-campaign-management.md) | Campaign lifecycle UI, approval workflows, dashboard metrics |
| Phishing Endpoints | [07-phishing-endpoints.md](07-phishing-endpoints.md) | Infrastructure management UI, endpoint status display |
| Credential Capture | [08-credential-capture.md](08-credential-capture.md) | Real-time capture display, credential masking, audit-logged reveal, field categorization |
| Target Management | [09-target-management.md](09-target-management.md) | Target list UI, import wizard, group management |
| Metrics & Reporting | [10-metrics-reporting.md](10-metrics-reporting.md) | Dashboard charts, report builder, export functionality |
| Audit Logging | [11-audit-logging.md](11-audit-logging.md) | Log viewer with real-time streaming, filtering |
| Database Schema | [14-database-schema.md](14-database-schema.md) | API response types align with database entities |
| System Overview | [01-system-overview.md](01-system-overview.md) | Overall architecture context, component responsibilities |
| Implementation Roadmap | [17-implementation-roadmap.md](17-implementation-roadmap.md) | Implementation sequencing and dependencies |

---

## 10. Cline Delegation Summary

The following requirements involve offensive security techniques or anti-detection engineering and are candidates for delegation to Cline:

| Requirement | Technique |
|-------------|-----------|
| REQ-FE-050 | Landing page code generation engine architecture and implementation |
| REQ-FE-051 | CSS randomization and anti-fingerprinting strategies |
| REQ-FE-052 | HTML structure randomization techniques |
| REQ-FE-053 | JavaScript minimization and React runtime signature removal |
| REQ-FE-055 | Asset fingerprint randomization (re-encoding, hash variation) |

These items require specialized knowledge of web fingerprinting techniques, detection evasion, and adversarial web development. Implementation should be reviewed by the red team lead before deployment.
