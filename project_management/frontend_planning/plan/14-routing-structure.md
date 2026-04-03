# 14 — Routing Structure

This document specifies the complete URL routing structure, breadcrumb configuration, code-splitting boundaries, auth/permission guards, redirect rules, and route parameter validation for the Tackle admin UI. The application uses React Router v6+ with client-side SPA routing. Each top-level route boundary is code-split via `React.lazy()` wrapped in `<Suspense>` with a shared loading fallback.

---

## 1. Route Architecture Overview

### 1.1 Layout Hierarchy

Routes are organized into two layout groups:

| Layout | Description | Routes |
|--------|-------------|--------|
| **AuthLayout** | Standalone full-page layout. No sidebar, no top bar. `--bg-primary` background with centered card. | `/login`, `/setup`, `/auth/callback/:provider` |
| **AppShell** | Authenticated layout with sidebar, top bar, breadcrumb strip, and scrollable content area (see doc 02). | All other routes |

The `AppShell` layout wraps all authenticated routes and provides the persistent navigation frame. The `AuthLayout` is a separate component tree with no shared state with the shell.

### 1.2 Router Structure (Conceptual)

```
<BrowserRouter>
  <Routes>
    {/* Public routes — AuthLayout */}
    <Route element={<AuthLayout />}>
      <Route path="/login" ... />
      <Route path="/setup" ... />
      <Route path="/auth/callback/:provider" ... />
    </Route>

    {/* Protected routes — AppShell */}
    <Route element={<AuthGuard><AppShell /></AuthGuard>}>
      <Route path="/" ... />
      <Route path="/dashboard" ... />
      <Route path="/security-overview" ... />
      <Route path="/campaigns" ...>
        <Route path=":campaignId" ...>
          <Route path="targets" ... />
          ...
        </Route>
      </Route>
      ...
    </Route>

    {/* Catch-all */}
    <Route path="*" element={<NotFound />} />
  </Routes>
</BrowserRouter>
```

---

## 2. Complete Route Table

### 2.1 Public Routes (No Auth Required)

These routes are accessible without authentication. If an authenticated user navigates to `/login` or `/setup`, they are redirected to `/dashboard`.

| Path | Component Chunk | Description | Query Parameters |
|------|----------------|-------------|-----------------|
| `/login` | `LoginPage` | Login form with local and external auth providers. | `redirect` — URL to navigate to after successful login (URL-encoded). `expired=1` — shows "Session expired" message. |
| `/setup` | `SetupPage` | Initial admin account creation wizard. Only accessible when `GET /api/v1/setup/status` returns `setup_complete: false`. Redirects to `/login` after setup is complete. | None |
| `/auth/callback/:provider` | `AuthCallback` | OAuth/OIDC callback handler. Receives tokens from external provider redirect, stores access token, then redirects to dashboard or stored redirect URL. | `code` — authorization code from provider. `state` — CSRF state token. `error` — error code from provider. |

### 2.2 Authenticated Routes — Top Level

All routes below require authentication. The `AuthGuard` wrapper checks for a valid access token, attempts silent refresh if expired, and redirects to `/login?redirect={currentPath}` if authentication fails.

| Path | Component Chunk | Page Title | Description |
|------|----------------|------------|-------------|
| `/` | N/A (redirect) | N/A | Redirects to `/dashboard` (302 client-side redirect). |
| `/dashboard` | `DashboardPage` | Dashboard | Operator dashboard with campaign overview, capture metrics, infrastructure health, and activity feed. Default landing page for Admin, Operator, and Engineer roles. |
| `/security-overview` | `SecurityOverviewPage` | Security Overview | Defender dashboard with organizational susceptibility score, department risk heatmap, and aggregate campaign effectiveness. Default landing page for Defender role users. |

### 2.3 Campaigns

| Path | Component Chunk | Page Title | Description | Query Parameters |
|------|----------------|------------|-------------|-----------------|
| `/campaigns` | `CampaignsPage` | Campaigns | Campaign list table with search, filters, and bulk actions. | `search` — search string. `status` — comma-separated status filter (e.g., `active,draft`). `date_field` — one of `created`, `launched`, `completed`. `date_from` — ISO date. `date_to` — ISO date. `page` — page number (default `1`). `sort` — column name. `order` — `asc` or `desc`. `view` — `table` (default) or `calendar`. |
| `/campaigns/calendar` | `CampaignsPage` | Campaigns — Calendar | Calendar view of campaigns by scheduled/actual launch dates. Same component as list, toggled via `view=calendar` query parameter. Alias route that sets `view=calendar`. | `month` — `YYYY-MM` (default: current month). `status` — comma-separated status filter. |
| `/campaigns/:campaignId` | `CampaignWorkspace` | {Campaign Name} | Campaign workspace — Overview tab (default). Displays campaign summary, readiness checklist, status, and quick metrics. | None |
| `/campaigns/:campaignId/targets` | `CampaignWorkspace` | {Campaign Name} — Targets | Campaign workspace — Targets tab. Target group assignment, deduplication preview, blocklist check. | None |
| `/campaigns/:campaignId/email` | `CampaignWorkspace` | {Campaign Name} — Email | Campaign workspace — Email Templates tab. Template selection, sender profile configuration, A/B testing setup. | None |
| `/campaigns/:campaignId/landing-page` | `CampaignWorkspace` | {Campaign Name} — Landing Page | Campaign workspace — Landing Page tab. Landing page selection and preview. | None |
| `/campaigns/:campaignId/infrastructure` | `CampaignWorkspace` | {Campaign Name} — Infrastructure | Campaign workspace — Infrastructure tab. Domain, SMTP, and endpoint configuration. | None |
| `/campaigns/:campaignId/schedule` | `CampaignWorkspace` | {Campaign Name} — Schedule | Campaign workspace — Schedule tab. Launch timing, send windows, throttling, and timezone configuration. | None |
| `/campaigns/:campaignId/approval` | `CampaignWorkspace` | {Campaign Name} — Approval History | Campaign workspace — Approval History tab. Submission, approval, and rejection history with comments. | None |

### 2.4 Targets

| Path | Component Chunk | Page Title | Description | Query Parameters |
|------|----------------|------------|-------------|-----------------|
| `/targets` | `TargetsPage` | Targets | Target list with search, filtering, and bulk operations. Default tab: Targets. | `search` — search string. `department` — department filter. `group` — group ID filter. `activity` — activity status filter. `page` — page number. `sort` — column name. `order` — `asc` or `desc`. `tab` — `targets` (default), `groups`, or `blocklist`. |
| `/targets?tab=groups` | `TargetsPage` | Targets — Groups | Target groups management tab. Group list with member counts, create/edit/delete groups. | `search` — search groups. `page` — page number. |
| `/targets?tab=blocklist` | `TargetsPage` | Targets — Blocklist | Global blocklist management. Email addresses and patterns excluded from all campaigns. | `search` — search blocklist entries. `page` — page number. |

### 2.5 Email Templates

| Path | Component Chunk | Page Title | Description | Query Parameters |
|------|----------------|------------|-------------|-----------------|
| `/email-templates` | `EmailTemplatesPage` | Email Templates | Template list with search, category/tag filtering, and preview slide-over. | `search` — search string. `category` — category filter. `tags` — comma-separated tag filter. `page` — page number. `sort` — column name. `order` — `asc` or `desc`. |
| `/email-templates/new` | `EmailTemplateEditor` | New Email Template | Template editor for creating a new template. WYSIWYG and HTML editing, variable insertion, attachment management, device preview. | None |
| `/email-templates/:templateId` | `EmailTemplateEditor` | {Template Name} | Template editor for an existing template. Same editor component as new, pre-populated with template data. | `version` — optional version number to load a specific version. |

### 2.6 Landing Pages

| Path | Component Chunk | Page Title | Description | Query Parameters |
|------|----------------|------------|-------------|-----------------|
| `/landing-pages` | `LandingPagesPage` | Landing Pages | Landing page list with search, filtering, and preview. | `search` — search string. `page` — page number. `sort` — column name. `order` — `asc` or `desc`. |
| `/landing-pages/new` | `LandingPageBuilder` | New Landing Page | Full-screen landing page builder. Application shell sidebar is hidden (see doc 07). | None |
| `/landing-pages/:pageId` | `LandingPageBuilder` | {Page Name} | Full-screen landing page builder for an existing page. | None |

### 2.7 Infrastructure

| Path | Component Chunk | Page Title | Description | Query Parameters |
|------|----------------|------------|-------------|-----------------|
| `/infrastructure` | N/A (redirect) | N/A | Redirects to `/infrastructure/domains`. | N/A |
| `/infrastructure/domains` | `InfrastructurePage` | Infrastructure — Domains | Domain list with health status, DNS records, and email authentication status. Detail via slide-over panel. | `search` — search string. `status` — health status filter. `provider` — domain provider filter. `page` — page number. `sort` — column name. `order` — `asc` or `desc`. |
| `/infrastructure/smtp-profiles` | `InfrastructurePage` | Infrastructure — SMTP Profiles | SMTP profile list with connection status. Detail and create/edit via slide-over panel. | `search` — search string. `status` — connection status filter. `page` — page number. |
| `/infrastructure/cloud-credentials` | `InfrastructurePage` | Infrastructure — Cloud Credentials | Cloud provider credential management (AWS, Azure, GCP). | `search` — search string. `provider` — cloud provider filter. `page` — page number. |
| `/infrastructure/instance-templates` | `InfrastructurePage` | Infrastructure — Instance Templates | Compute instance template definitions for endpoint provisioning. | `search` — search string. `provider` — cloud provider filter. `page` — page number. |
| `/infrastructure/domain-providers` | `InfrastructurePage` | Infrastructure — Domain Providers | Domain registrar connection management. | `search` — search string. `page` — page number. |
| `/infrastructure/tools` | `InfrastructurePage` | Infrastructure — Tools | Typosquat domain generator and other infrastructure utilities. | Tool-specific query parameters vary. |

### 2.8 Insights

| Path | Component Chunk | Page Title | Description | Query Parameters |
|------|----------------|------------|-------------|-----------------|
| `/metrics` | `MetricsPage` | Metrics | Metrics dashboard with interactive charts, filterable by campaign, date range, department, and template. | `campaign_ids` — comma-separated campaign IDs. `start_date` — ISO date. `end_date` — ISO date. `departments` — comma-separated department names. `template_ids` — comma-separated template IDs. `live` — `1` or `0` (live mode toggle). |
| `/reports` | `ReportsPage` | Reports | Report list with generated reports and generation controls. | `search` — search string. `page` — page number. `sort` — column name. `order` — `asc` or `desc`. |
| `/reports/:reportId` | `ReportDetailPage` | {Report Name} | Report detail view with rendered report content and export options. | `format` — `pdf`, `csv`, or `html` (for export trigger). |

### 2.9 Audit Logs

| Path | Component Chunk | Page Title | Description | Query Parameters |
|------|----------------|------------|-------------|-----------------|
| `/audit-logs` | `AuditLogsPage` | Audit Logs | Full-page log viewer with real-time tail mode, filtering, search, and correlation chain tracing. | `category` — event category filter. `severity` — severity level filter. `actor` — actor username filter. `campaign` — campaign ID filter. `action` — action type filter. `resource` — resource type filter. `start` — ISO datetime. `end` — ISO datetime. `search` — full-text search. `correlation_id` — show only entries matching this correlation chain. `live` — `1` or `0` (live tail mode). |

### 2.10 Administration

| Path | Component Chunk | Page Title | Description | Query Parameters |
|------|----------------|------------|-------------|-----------------|
| `/users` | `UsersRolesPage` | Users & Roles | User management list with role assignment. Default tab: Users. | `tab` — `users` (default) or `roles`. `search` — search string. `role` — role filter. `status` — active/inactive filter. `page` — page number. |
| `/users?tab=roles` | `UsersRolesPage` | Users & Roles — Roles | Role management tab. Role list with permission matrix display, create/edit roles. | `search` — search roles. |
| `/settings` | `SettingsPage` | Settings | System settings with tabbed sections. Default tab: General. | `tab` — `general` (default), `auth-providers`, `session`, `password`, `webhooks`, or `notification-smtp`. |
| `/settings?tab=auth-providers` | `SettingsPage` | Settings — Auth Providers | OIDC, LDAP, and FusionAuth provider configuration. | None additional. |
| `/settings?tab=session` | `SettingsPage` | Settings — Session | Session timeout, concurrent session limits, token lifetime configuration. | None additional. |
| `/settings?tab=password` | `SettingsPage` | Settings — Password | Password policy requirements (length, complexity, history, expiry). | None additional. |
| `/settings?tab=general` | `SettingsPage` | Settings — General | Application name, timezone, retention policies, and general system configuration. | None additional. |
| `/settings?tab=webhooks` | `SettingsPage` | Settings — Webhooks | Webhook endpoint configuration for external integrations. | None additional. |
| `/settings?tab=notification-smtp` | `SettingsPage` | Settings — Notification SMTP | SMTP server configuration for system notification emails (not phishing emails). | None additional. |

### 2.11 User Profile

| Path | Component Chunk | Page Title | Description | Query Parameters |
|------|----------------|------------|-------------|-----------------|
| `/preferences` | `UserPreferencesPage` | Preferences | User-specific preferences: theme, timezone, notification settings, sidebar pinned items. Accessible from the profile dropdown menu in the top bar. | `section` — optional scroll-to section anchor (`theme`, `notifications`, `timezone`). |

### 2.12 Error Pages

| Path | Component Chunk | Page Title | Description |
|------|----------------|------------|-------------|
| `/403` | `ForbiddenPage` | Access Denied | Displayed when a user navigates to a route they lack permission for. Shows "Access Denied" message with a "Go to Dashboard" button. Not a real navigable route — rendered inline by `PermissionGuard`. |
| `*` (catch-all) | `NotFoundPage` | Page Not Found | Displayed for any URL that does not match a defined route. Shows "404 — Page Not Found" with a "Go to Dashboard" button. |

---

## 3. Breadcrumb Configuration

Breadcrumbs are rendered in the 36px breadcrumb strip between the top bar and the content area (within the content column, not spanning the sidebar). Each segment except the last is a clickable link. The last segment is static text in `--text-primary`. Separator: `/` in `--text-muted`.

Dynamic segments (entity names) are resolved from the TanStack Query cache. If the entity is not yet in the cache, the breadcrumb segment displays a 60px-wide skeleton shimmer until the data loads. Entity names longer than 30 characters are truncated with an ellipsis and the full name is shown in a tooltip.

### 3.1 Breadcrumb Trail by Route

| Route | Breadcrumb Trail |
|-------|-----------------|
| `/dashboard` | **Dashboard** |
| `/security-overview` | **Security Overview** |
| `/campaigns` | **Campaigns** |
| `/campaigns/calendar` | Campaigns / **Calendar** |
| `/campaigns/:campaignId` | Campaigns / **{Campaign Name}** |
| `/campaigns/:campaignId/targets` | Campaigns / {Campaign Name} / **Targets** |
| `/campaigns/:campaignId/email` | Campaigns / {Campaign Name} / **Email** |
| `/campaigns/:campaignId/landing-page` | Campaigns / {Campaign Name} / **Landing Page** |
| `/campaigns/:campaignId/infrastructure` | Campaigns / {Campaign Name} / **Infrastructure** |
| `/campaigns/:campaignId/schedule` | Campaigns / {Campaign Name} / **Schedule** |
| `/campaigns/:campaignId/approval` | Campaigns / {Campaign Name} / **Approval History** |
| `/targets` | **Targets** |
| `/targets?tab=groups` | Targets / **Groups** |
| `/targets?tab=blocklist` | Targets / **Blocklist** |
| `/email-templates` | **Email Templates** |
| `/email-templates/new` | Email Templates / **New Template** |
| `/email-templates/:templateId` | Email Templates / **{Template Name}** |
| `/landing-pages` | **Landing Pages** |
| `/landing-pages/new` | Landing Pages / **New Landing Page** |
| `/landing-pages/:pageId` | Landing Pages / **{Page Name}** |
| `/infrastructure/domains` | Infrastructure / **Domains** |
| `/infrastructure/smtp-profiles` | Infrastructure / **SMTP Profiles** |
| `/infrastructure/cloud-credentials` | Infrastructure / **Cloud Credentials** |
| `/infrastructure/instance-templates` | Infrastructure / **Instance Templates** |
| `/infrastructure/domain-providers` | Infrastructure / **Domain Providers** |
| `/infrastructure/tools` | Infrastructure / **Tools** |
| `/metrics` | **Metrics** |
| `/reports` | **Reports** |
| `/reports/:reportId` | Reports / **{Report Name}** |
| `/audit-logs` | **Audit Logs** |
| `/users` | **Users & Roles** |
| `/users?tab=roles` | Users & Roles / **Roles** |
| `/settings` | **Settings** |
| `/preferences` | **Preferences** |

### 3.2 Breadcrumb Data Resolution

Dynamic breadcrumb segments resolve entity names from these TanStack Query cache keys:

| Dynamic Segment | Query Key | API Endpoint | Display Field |
|----------------|-----------|--------------|---------------|
| `:campaignId` | `['campaigns', campaignId]` | `GET /api/v1/campaigns/:id` | `name` |
| `:templateId` | `['email-templates', templateId]` | `GET /api/v1/templates/email/:id` | `name` |
| `:pageId` | `['landing-pages', pageId]` | `GET /api/v1/landing-pages/:id` | `name` |
| `:reportId` | `['reports', reportId]` | `GET /api/v1/reports/:id` | `name` |

If the entity detail query has not been fetched yet (e.g., the user navigated directly to a deep URL), the breadcrumb component triggers a lightweight fetch for just the entity name. This fetch uses `staleTime: Infinity` to avoid refetching if the entity is already cached from a list or detail load.

### 3.3 Tab-Driven Breadcrumbs

For pages that use query-parameter-based tabs (`/targets?tab=groups`, `/users?tab=roles`, `/settings?tab=auth-providers`), the breadcrumb trail adds the tab label as a second segment only when the tab is not the default. When the default tab is active, only the top-level breadcrumb is shown.

| Tab Query | Breadcrumb Behavior |
|-----------|-------------------|
| `?tab=targets` (default) | Targets (no second segment) |
| `?tab=groups` | Targets / **Groups** |
| `?tab=blocklist` | Targets / **Blocklist** |
| `?tab=users` (default) | Users & Roles (no second segment) |
| `?tab=roles` | Users & Roles / **Roles** |
| `?tab=general` (default) | Settings (no second segment) |
| `?tab=auth-providers` | Settings / **Auth Providers** |
| `?tab=session` | Settings / **Session** |
| `?tab=password` | Settings / **Password** |
| `?tab=webhooks` | Settings / **Webhooks** |
| `?tab=notification-smtp` | Settings / **Notification SMTP** |

---

## 4. Code-Splitting Boundaries

Each entry in the table below is a separate chunk loaded via `React.lazy()` wrapped in `<Suspense>`. The `<Suspense>` fallback is a consistent loading skeleton: the AppShell frame (sidebar + top bar) with a centered spinner in the content area.

| Chunk Name | Routes Covered | Estimated Size Justification |
|------------|---------------|------------------------------|
| `LoginPage` | `/login` | Standalone auth form, minimal dependencies. |
| `SetupPage` | `/setup` | One-time wizard, separate from login. |
| `AuthCallback` | `/auth/callback/:provider` | Minimal — token exchange logic only. |
| `DashboardPage` | `/dashboard` | Chart libraries (Recharts), stat cards, activity feed. |
| `SecurityOverviewPage` | `/security-overview` | Separate chart set from operator dashboard. |
| `CampaignsPage` | `/campaigns`, `/campaigns/calendar` | List table, calendar view (large calendar component), filters. |
| `CampaignWorkspace` | `/campaigns/:campaignId` and all sub-tabs | Largest chunk. Workspace tabs are **not** further split — they share too much state (campaign entity, readiness tracker, save handlers). All 7 tabs load together. |
| `TargetsPage` | `/targets` (all tabs) | Table, group management, blocklist, CSV import wizard. |
| `EmailTemplatesPage` | `/email-templates` | List view only. |
| `EmailTemplateEditor` | `/email-templates/new`, `/email-templates/:templateId` | WYSIWYG editor, HTML editor, preview renderer, attachment manager. Separate from list due to size. |
| `LandingPagesPage` | `/landing-pages` | List view only. |
| `LandingPageBuilder` | `/landing-pages/new`, `/landing-pages/:pageId` | Largest chunk after CampaignWorkspace. Full drag-and-drop builder, iframe canvas, property editor, code panel. Separate from list due to size. |
| `InfrastructurePage` | `/infrastructure/*` (all tabs) | Domain, SMTP, cloud, template, provider, and tools tabs. Single chunk because tabs share the infrastructure layout and summary bar. |
| `MetricsPage` | `/metrics` | Chart-heavy page with Recharts, filter bar, live WebSocket. |
| `ReportsPage` | `/reports` | Report list, generation controls. |
| `ReportDetailPage` | `/reports/:reportId` | Report renderer, export functionality. Separate from list because report rendering may be heavyweight. |
| `AuditLogsPage` | `/audit-logs` | Virtualized log table, filter bar, correlation chain viewer, live tail WebSocket. |
| `UsersRolesPage` | `/users` (all tabs) | User table, role management, permission matrix. |
| `SettingsPage` | `/settings` (all tabs) | Settings forms across 6 tabs. |
| `UserPreferencesPage` | `/preferences` | User preference forms. |
| `NotFoundPage` | `*` (catch-all) | Minimal — static error page. |
| `ForbiddenPage` | (inline, rendered by PermissionGuard) | Minimal — static error page. |

### 4.1 Prefetching Strategy

To reduce perceived load time on navigation, the following prefetch rules apply:

- **Sidebar hover prefetch**: When a user hovers over a sidebar navigation item for more than 200ms, the chunk for that route begins loading. This uses `React.lazy()` with a manual `import()` call triggered by the hover event.
- **Dashboard prefetch**: On initial authenticated load, after the dashboard chunk loads and renders, the `CampaignsPage` and `MetricsPage` chunks are prefetched in the background (these are the most commonly navigated-to pages).
- **List-to-detail prefetch**: When hovering over a campaign row in the campaign list, the `CampaignWorkspace` chunk begins loading. Same pattern for email template rows (prefetch `EmailTemplateEditor`) and landing page rows (prefetch `LandingPageBuilder`).

---

## 5. Auth and Permission Guards

### 5.1 Auth Guard

The `AuthGuard` component wraps all non-public routes. It performs the following check sequence on every route transition:

1. Check if an access token exists in the Zustand auth store.
2. If yes, allow rendering of the child route.
3. If no, attempt silent refresh via `POST /api/v1/auth/refresh`.
4. If refresh succeeds, store the new token and allow rendering.
5. If refresh fails, redirect to `/login?redirect={encodeURIComponent(currentPath)}`.

During steps 3-4, a full-screen loading state is shown (Tackle logo with pulse animation on `--bg-primary` background).

### 5.2 Setup Guard

Before any route renders (including `/login`), the application checks `GET /api/v1/setup/status`. If `setup_complete` is `false`, all routes redirect to `/setup`. If setup is complete and the user navigates to `/setup`, they are redirected to `/login`.

### 5.3 Permission Guard

The `PermissionGuard` component wraps individual routes that require specific permissions. It reads the user's permission set from the Zustand auth store (populated from JWT claims) and either renders the child route or renders the `ForbiddenPage` inline.

| Route | Required Permission | Accessible Roles |
|-------|-------------------|-----------------|
| `/dashboard` | None (always accessible) | All authenticated users |
| `/security-overview` | `metrics:read` | Admin, Operator, Engineer, Defender |
| `/campaigns` | `campaigns:read` | Admin, Operator, Engineer |
| `/campaigns/calendar` | `campaigns:read` | Admin, Operator, Engineer |
| `/campaigns/:campaignId` | `campaigns:read` | Admin, Operator, Engineer |
| `/campaigns/:campaignId/*` | `campaigns:read` | Admin, Operator, Engineer |
| `/targets` | `targets:read` | Admin, Operator |
| `/targets?tab=groups` | `targets:read` | Admin, Operator |
| `/targets?tab=blocklist` | `targets:read` | Admin, Operator |
| `/email-templates` | `templates.email:read` | Admin, Operator |
| `/email-templates/new` | `templates.email:create` | Admin, Operator |
| `/email-templates/:templateId` | `templates.email:read` | Admin, Operator |
| `/landing-pages` | `landing_pages:read` | Admin, Operator, Engineer |
| `/landing-pages/new` | `landing_pages:create` | Admin, Operator, Engineer |
| `/landing-pages/:pageId` | `landing_pages:read` | Admin, Operator, Engineer |
| `/infrastructure/*` | `domains:read` OR `smtp:read` OR `cloud:read` | Admin, Engineer |
| `/infrastructure/domains` | `domains:read` | Admin, Engineer |
| `/infrastructure/smtp-profiles` | `smtp:read` | Admin, Engineer |
| `/infrastructure/cloud-credentials` | `cloud:read` | Admin, Engineer |
| `/infrastructure/instance-templates` | `cloud:read` | Admin, Engineer |
| `/infrastructure/domain-providers` | `domains:read` | Admin, Engineer |
| `/infrastructure/tools` | `domains:read` | Admin, Engineer |
| `/metrics` | `metrics:read` | Admin, Operator, Engineer, Defender |
| `/reports` | `reports:read` | Admin, Operator, Engineer, Defender |
| `/reports/:reportId` | `reports:read` | Admin, Operator, Engineer, Defender |
| `/audit-logs` | `logs.audit:read` | Admin, Engineer |
| `/users` | `users:read` | Admin |
| `/settings` | `settings:read` | Admin |
| `/preferences` | None (always accessible) | All authenticated users |

### 5.4 Write Permission Enforcement

Write permissions are not enforced at the route level — they are enforced at the component level via `PermissionGate` and `usePermissions()` (see doc 03, section 5). A user with `campaigns:read` but not `campaigns:create` can view `/campaigns` but the "New Campaign" button is not rendered. A user with `templates.email:read` but not `templates.email:update` can open `/email-templates/:templateId` but the editor is in read-only mode.

---

## 6. Redirect Rules

| Condition | From | To | Type |
|-----------|------|----|------|
| Root path | `/` | `/dashboard` | Client-side redirect (React Router `<Navigate>`) |
| Infrastructure index | `/infrastructure` | `/infrastructure/domains` | Client-side redirect |
| Authenticated user visits login | `/login` | `/dashboard` | Redirect in AuthLayout (if token is valid) |
| Authenticated user visits setup | `/setup` | `/dashboard` | Redirect in setup guard (if setup is complete) |
| Setup incomplete | Any route | `/setup` | Redirect in setup guard |
| Unauthenticated user visits protected route | Any protected route | `/login?redirect={currentPath}` | Redirect in AuthGuard |
| Defender role user default | `/dashboard` | `/security-overview` | Redirect based on role (Defender role users are redirected from `/dashboard` to `/security-overview`) |
| Post-login redirect | `/login` (after success) | Stored `redirect` param or `/dashboard` | Redirect after successful authentication |
| Campaign workspace index | `/campaigns/:campaignId` (no sub-path) | Renders Overview tab directly | Not a redirect — the default tab renders at the base workspace URL |

---

## 7. Route Parameter Types and Validation

### 7.1 Parameter Definitions

| Parameter | Pattern | Description | Validation |
|-----------|---------|-------------|------------|
| `:campaignId` | UUID v4 | Campaign entity identifier | Must match `/^[0-9a-f]{8}-[0-9a-f]{4}-4[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$/i`. If invalid, render 404 immediately without making an API call. |
| `:templateId` | UUID v4 | Email template entity identifier | Same UUID v4 validation as above. |
| `:pageId` | UUID v4 | Landing page entity identifier | Same UUID v4 validation as above. |
| `:reportId` | UUID v4 | Report entity identifier | Same UUID v4 validation as above. |
| `:provider` | String (slug) | Auth provider identifier for OAuth callback | Must match `/^[a-z0-9-]+$/` (lowercase alphanumeric with hyphens). Max length 64 characters. |

### 7.2 Validation Behavior

Route parameter validation occurs in a `RouteParamValidator` wrapper component that sits between the route definition and the page component:

1. **Invalid UUID format**: If a dynamic segment expected to be a UUID does not match the UUID v4 regex, the `NotFoundPage` is rendered immediately. No API call is made.
2. **Valid UUID, entity not found**: If the UUID format is valid but the API returns 404, the page component handles this by rendering a "not found" state specific to the entity type (e.g., "Campaign not found" with a link back to `/campaigns`).
3. **Valid UUID, no permission**: If the API returns 403, the `ForbiddenPage` is rendered.

### 7.3 Query Parameter Validation

Query parameters are validated with lenient parsing. Invalid values are silently replaced with defaults rather than showing an error:

| Parameter | Type | Default | Validation |
|-----------|------|---------|------------|
| `page` | Positive integer | `1` | Parsed with `parseInt`. If NaN or < 1, reset to `1`. |
| `sort` | String (enum) | Varies by page | Must be one of the allowed column names for that page. If invalid, use default sort. |
| `order` | `asc` or `desc` | `desc` | If not `asc` or `desc`, reset to `desc`. |
| `search` | String | `""` | Trimmed. Max 200 characters (truncated silently). |
| `tab` | String (enum) | First tab | Must match one of the valid tab names for that page. If invalid, reset to default tab. |
| `view` | `table` or `calendar` | `table` | If not `table` or `calendar`, reset to `table`. |
| `live` | `0` or `1` | `0` | If not `0` or `1`, reset to `0`. |
| `date_from`, `date_to`, `start_date`, `end_date` | ISO 8601 date | None | Parsed with `Date.parse()`. If invalid, the parameter is ignored. |
| `redirect` | URL path | `/dashboard` | Must start with `/`. External URLs are rejected (prevents open redirect). Validated with URL constructor — if parsing fails, defaults to `/dashboard`. |

---

## 8. 404 Handling

### 8.1 Catch-All Route

A catch-all route (`path="*"`) is defined as the last route in the router configuration. Any URL that does not match a defined route renders the `NotFoundPage` component within the `AppShell` layout (if the user is authenticated) or within the `AuthLayout` (if not).

### 8.2 Not Found Page Layout

```
┌──────────────────────────────────────────┐
│                                          │
│            (FileQuestion icon)           │
│                                          │
│         404 — Page Not Found             │
│                                          │
│   The page you're looking for doesn't    │
│   exist or has been moved.               │
│                                          │
│         [Go to Dashboard]               │
│                                          │
└──────────────────────────────────────────┘
```

- Icon: `FileQuestion` from Lucide, 64px, `--text-muted` color.
- Heading: "404 — Page Not Found" in `--text-h2`.
- Body text: `--text-secondary`.
- Button: Primary variant, navigates to `/dashboard`.
- The page renders within the AppShell (sidebar and top bar remain visible) so the user can navigate via the sidebar.

### 8.3 Entity Not Found (API 404)

Distinct from the catch-all 404. When a valid route with a valid UUID parameter results in an API 404 response, the page component renders an entity-specific not-found state:

| Entity | Message | Navigation |
|--------|---------|------------|
| Campaign | "Campaign not found. It may have been deleted." | "Back to Campaigns" link to `/campaigns` |
| Email Template | "Template not found. It may have been deleted." | "Back to Email Templates" link to `/email-templates` |
| Landing Page | "Landing page not found. It may have been deleted." | "Back to Landing Pages" link to `/landing-pages` |
| Report | "Report not found. It may have been deleted." | "Back to Reports" link to `/reports` |

These entity-not-found states use the same visual style as the 404 page (centered icon, heading, body, navigation button) but with entity-specific copy.

---

## 9. Landing Page Builder Route Behavior

The landing page builder routes (`/landing-pages/new` and `/landing-pages/:pageId`) have special layout behavior:

- The AppShell sidebar is automatically hidden when these routes activate.
- The AppShell top bar is replaced by the builder's own toolbar (see doc 07).
- Breadcrumbs are not shown — the builder toolbar includes its own "Back" button that navigates to `/landing-pages`.
- When navigating away from the builder, the sidebar returns to its previous expanded/collapsed state.
- If there are unsaved changes, a confirmation dialog is shown before navigation proceeds.

---

## 10. URL Synchronization

### 10.1 Filter State in URL

All list pages synchronize their filter and pagination state with URL query parameters. This enables:

- **Shareable URLs**: Copying a URL with filters produces the same filtered view for another user.
- **Browser history**: Using browser back/forward navigates through filter states.
- **Bookmarkable views**: Users can bookmark specific filtered views.

Filter-to-URL synchronization uses `useSearchParams()` from React Router. Changes to filters update the URL without a full navigation (using `replace` mode to avoid polluting browser history with every keystroke).

### 10.2 Tab State in URL

Tab-based pages (`/targets`, `/users`, `/settings`) use the `tab` query parameter rather than nested routes. This is intentional — tabs on these pages share a common layout and header, and switching tabs should not trigger a chunk load or a full route transition.

The `tab` parameter is updated via `setSearchParams` with `replace: true` so that switching between tabs does not create back-button history entries for each tab switch.

---

## 11. Route Transition Behavior

### 11.1 Loading States

When a lazy-loaded chunk is loading (first visit to a route group):

- The AppShell frame (sidebar, top bar) remains visible and interactive.
- The content area shows a centered spinner with the text "Loading..." in `--text-muted`.
- The breadcrumb strip updates immediately to show the target route's breadcrumb (with skeleton shimmer for dynamic segments if entity data is not yet cached).

### 11.2 Navigation Guards

Before navigating away from a page with unsaved changes, a confirmation dialog is shown:

```
┌────────────────────────────────────────┐
│  Unsaved Changes                       │
│                                        │
│  You have unsaved changes that will    │
│  be lost if you leave this page.       │
│                                        │
│  [Discard Changes]     [Stay on Page]  │
└────────────────────────────────────────┘
```

This applies to:
- Campaign workspace tabs with unsaved modifications.
- Email template editor with unsaved content.
- Landing page builder with unsaved content.
- Settings page with unsaved form changes.

The guard is implemented via React Router's `useBlocker` hook (v6.4+), combined with the `beforeunload` browser event for tab/window close attempts.
