# 09 — Infrastructure Management

This document specifies the **Infrastructure Management** section of the Tackle platform: domain providers, domains, DNS records, email authentication, SMTP profiles, cloud credentials, instance templates, endpoints, and the typosquat domain generation tool. Infrastructure is the operational backbone of phishing campaigns — domains host landing pages, SMTP profiles send emails, cloud credentials provision infrastructure, and instance templates define compute resources. Endpoints are provisioned per-campaign during the campaign build phase and are never reused between campaigns; they are viewed from the campaign context, not managed as standalone entities.

---

## 1. Infrastructure Overview Page

### 1.1 Purpose

The infrastructure overview serves as the central hub for managing all infrastructure components. It uses a tabbed layout to separate each infrastructure type into its own management surface while providing a unified navigation experience.

### 1.2 Layout

```
┌──────────────────────────────────────────────────────────────────────────┐
│  Infrastructure                                                          │
├──────────────────────────────────────────────────────────────────────────┤
│  [Domains]  [SMTP Profiles]  [Cloud Credentials]  [Instance Templates]  │
│  [Domain Providers]  [Tools]                                             │
├──────────────────────────────────────────────────────────────────────────┤
│                                                                          │
│  (Active tab content renders here)                                       │
│                                                                          │
└──────────────────────────────────────────────────────────────────────────┘
```

### 1.3 Top-Level Tabs

| Tab | Route | Content |
|-----|-------|---------|
| Domains | `/infrastructure/domains` | Domain list and management (section 3) |
| SMTP Profiles | `/infrastructure/smtp-profiles` | SMTP configuration list (section 7) |
| Cloud Credentials | `/infrastructure/cloud-credentials` | Cloud provider credential management (section 8) |
| Instance Templates | `/infrastructure/instance-templates` | Compute instance template management (section 9) |
| Domain Providers | `/infrastructure/domain-providers` | Domain registrar connections (section 2) |
| Tools | `/infrastructure/tools` | Typosquat generator and other tools (section 11) |

- The active tab is indicated by a bottom border in `--accent` color.
- Tab switching does not reset filters on any tab — filters are preserved in URL query parameters.
- Default route `/infrastructure` redirects to `/infrastructure/domains`.
- Endpoints are NOT listed as a tab here. They are viewed exclusively from the campaign workspace (section 10).

### 1.4 Summary Bar

Below the tab row, each tab displays a summary bar with key counts relevant to that infrastructure type. For the Domains tab:

```
┌──────────────────────────────────────────────────────────────────────────┐
│  ┌──────────────┐ ┌──────────────┐ ┌──────────────┐ ┌──────────────┐   │
│  │ Total        │ │ Healthy      │ │ Needs Review  │ │ Pending      │   │
│  │     24       │ │     18       │ │      4        │ │     2        │   │
│  └──────────────┘ └──────────────┘ └──────────────┘ └──────────────┘   │
└──────────────────────────────────────────────────────────────────────────┘
```

- Each card is clickable and applies a filter to the list below.
- Cards use `--bg-secondary` background, `--text-primary` for the count, `--text-muted` for the label.
- Healthy count uses `--status-success` text color. Needs Review uses `--status-warning`. Pending uses `--text-muted`.

---

## 2. Domain Provider Management

### 2.1 Purpose

Domain providers represent connections to domain registrar APIs (GoDaddy, Namecheap, Route53, Azure DNS). These connections enable automated domain registration, DNS management, and renewal operations. Providers are displayed as connection cards rather than table rows.

### 2.2 Layout

```
┌──────────────────────────────────────────────────────────────────────────┐
│  Domain Providers                              [+ Add Provider]          │
├──────────────────────────────────────────────────────────────────────────┤
│                                                                          │
│  ┌─────────────────────────┐  ┌─────────────────────────┐               │
│  │  GoDaddy           ···  │  │  AWS Route53        ··· │               │
│  │  ─────────────────────  │  │  ─────────────────────  │               │
│  │  API Key: ••••••••3f2a  │  │  Access Key: ••••••4b1c │               │
│  │  Status: ● Connected    │  │  Status: ● Connected    │               │
│  │  Domains: 12            │  │  Domains: 6             │               │
│  │  Last tested: 2h ago    │  │  Last tested: 1d ago    │               │
│  │                         │  │                         │               │
│  │  [Test Connection]      │  │  [Test Connection]      │               │
│  └─────────────────────────┘  └─────────────────────────┘               │
│                                                                          │
│  ┌─────────────────────────┐  ┌─────────────────────────┐               │
│  │  Namecheap         ···  │  │  Azure DNS          ··· │               │
│  │  ─────────────────────  │  │  ─────────────────────  │               │
│  │  API Key: ••••••••9d3e  │  │  Client ID: ••••••8f2d  │               │
│  │  Status: ● Connected    │  │  Status: ○ Error        │               │
│  │  Domains: 3             │  │  Domains: 0             │               │
│  │  Last tested: 5h ago    │  │  Last tested: 3d ago    │               │
│  │                         │  │                         │               │
│  │  [Test Connection]      │  │  [Test Connection]      │               │
│  └─────────────────────────┘  └─────────────────────────┘               │
│                                                                          │
└──────────────────────────────────────────────────────────────────────────┘
```

### 2.3 Provider Card Specifications

- Cards use `--bg-secondary` background with `--border-primary` border.
- Cards are arranged in a responsive grid: 2 columns on desktop (>1024px), 1 column on tablet/mobile.
- Card width: fill available space within the grid column.
- The provider logo/icon appears in the card header alongside the provider name.
- Kebab menu (`···`) in the top-right corner of each card.
- Status indicator uses a colored dot: `--status-success` green for "Connected", `--status-error` red for "Error", `--text-muted` gray for "Untested".
- Credential values are always masked with bullet characters, showing only the last 4 characters.
- "Domains" count indicates how many domains in the system use this provider.
- "Last tested" shows a relative timestamp with tooltip for absolute datetime.

### 2.4 Kebab Menu Actions

| Action | Icon | Behavior |
|--------|------|----------|
| Edit | Pencil | Opens the provider edit slide-over (section 2.6) |
| Test Connection | Refresh | Triggers connection test (section 2.7) |
| Delete | Trash (red) | Opens delete confirmation modal (section 2.8) |

### 2.5 Add Provider Slide-Over

Clicking "[+ Add Provider]" opens a slide-over from the right (520px width on desktop, full-width below 768px).

```
┌───────────────────────────┬──────────────────────────────────┐
│                           │  Add Domain Provider       [✕]  │
│                           │  ────────────────────────────── │
│                           │                                  │
│                           │  Provider Type *                 │
│                           │  ┌────────────────────────────┐  │
│                           │  │ Select provider...       ▾ │  │
│                           │  └────────────────────────────┘  │
│                           │                                  │
│                           │  (Fields change based on type)   │
│                           │                                  │
│                           │  ── GoDaddy / Namecheap ──       │
│                           │  Name *                          │
│                           │  [________________________]      │
│                           │  API Key *                       │
│                           │  [________________________]      │
│                           │  API Secret *                    │
│                           │  [________________________]      │
│                           │                                  │
│                           │  ── Route53 ──                   │
│                           │  Name *                          │
│                           │  [________________________]      │
│                           │  AWS Access Key ID *             │
│                           │  [________________________]      │
│                           │  AWS Secret Access Key *         │
│                           │  [________________________]      │
│                           │  Region                          │
│                           │  [________________________]      │
│                           │                                  │
│                           │  ── Azure DNS ──                 │
│                           │  Name *                          │
│                           │  [________________________]      │
│                           │  Tenant ID *                     │
│                           │  [________________________]      │
│                           │  Client ID *                     │
│                           │  [________________________]      │
│                           │  Client Secret *                 │
│                           │  [________________________]      │
│                           │  Subscription ID *               │
│                           │  [________________________]      │
│                           │                                  │
│                           │  [Cancel]  [Test & Save]         │
└───────────────────────────┴──────────────────────────────────┘
```

**Form Behavior:**
- Selecting a provider type dynamically renders the appropriate credential fields.
- All credential fields use `type="password"` with a toggle-visibility eye icon.
- "Test & Save" first calls `POST /api/v1/domain-providers/test` with the entered credentials. On success, it then calls `POST /api/v1/domain-providers` to save. On test failure, a toast error is shown: "Connection test failed: {error_message}" and the provider is NOT saved.
- The user may also click "Cancel" to discard without saving. If the form has unsaved changes, a confirmation dialog appears: "Discard unsaved changes?"
- On successful save: toast "Provider added successfully.", slide-over closes, card grid refreshes.

### 2.6 Edit Provider Slide-Over

- Same layout as the Add Provider slide-over, but pre-populated with existing data.
- Provider type is displayed as read-only text (cannot be changed after creation).
- Credential fields show masked placeholder text ("••••••••"). The user must re-enter credentials to change them; leaving them blank preserves the existing values.
- API call: `PUT /api/v1/domain-providers/{id}`.
- "Test & Save" button behaves identically to the add flow.

### 2.7 Connection Test

- Clicking "Test Connection" on a card calls `POST /api/v1/domain-providers/{id}/test`.
- During the test: the button shows a spinner and text changes to "Testing...". The button is disabled.
- On success: toast "Connection successful." Status dot updates to green "Connected". "Last tested" timestamp updates.
- On failure: toast "Connection failed: {error_message}" (error variant). Status dot updates to red "Error". A detail modal opens with the full error response:

```
┌─────────────────────────────────────────┐
│  Connection Test Failed           [✕]   │
├─────────────────────────────────────────┤
│                                         │
│  Provider: GoDaddy                      │
│  Tested: Apr 3, 2026 at 2:15 PM        │
│                                         │
│  Error:                                 │
│  ┌─────────────────────────────────┐    │
│  │ 401 Unauthorized                │    │
│  │ Invalid API key or secret.      │    │
│  │ Please verify your credentials  │    │
│  │ in the GoDaddy developer        │    │
│  │ portal.                         │    │
│  └─────────────────────────────────┘    │
│                                         │
│                          [Close]        │
└─────────────────────────────────────────┘
```

### 2.8 Delete Provider Confirmation

```
┌─────────────────────────────────────────┐
│  Delete Provider                  [✕]   │
├─────────────────────────────────────────┤
│                                         │
│  Are you sure you want to delete the    │
│  provider "GoDaddy"?                    │
│                                         │
│  12 domains are currently associated    │
│  with this provider. Deleting it will   │
│  NOT delete those domains, but DNS      │
│  management via this provider will no   │
│  longer be available.                   │
│                                         │
│              [Cancel]  [Delete]         │
└─────────────────────────────────────────┘
```

- If the provider has associated domains, the count is shown as a warning.
- On confirm: `DELETE /api/v1/domain-providers/{id}`.
- On success: toast "Provider deleted.", card is removed from the grid.
- If the API returns a 409 (provider in use by active campaign), show error toast: "This provider is in use by an active campaign and cannot be deleted."

---

## 3. Domain Management

### 3.1 Purpose

The domain list is the primary management surface for all domains used in phishing campaigns. Domains host landing pages, receive email replies, and must be properly configured with DNS records and email authentication to avoid detection.

### 3.2 Layout

```
┌──────────────────────────────────────────────────────────────────────────┐
│  Domains                           [Register Domain]  [+ Add Domain]    │
├──────────────────────────────────────────────────────────────────────────┤
│  ┌────────────────────────┐ ┌──────────────┐ ┌──────────────┐           │
│  │ 🔍 Search domains...   │ │ Provider ▾   │ │ Status ▾     │           │
│  └────────────────────────┘ └──────────────┘ └──────────────┘           │
│  ┌──────────────┐                                                       │
│  │ Health ▾     │  [Clear Filters]                                      │
│  └──────────────┘                                                       │
├──────────────┬────────────┬──────────┬─────────┬──────────┬──────┬──────┤
│ Domain       │ Provider   │ Status   │ Health  │ Category │ Exp. │ ···  │
├──────────────┼────────────┼──────────┼─────────┼──────────┼──────┼──────┤
│ login-hr.com │ GoDaddy    │ Active   │ ● Good  │ Business │ 241d │ ···  │
│ secure-it.io │ Route53    │ Active   │ ▲ Warn  │ Uncateg. │ 89d  │ ···  │
│ hr-portal.co │ Namecheap  │ Pending  │ — N/A   │ —        │ —    │ ···  │
│ mail-sys.net │ GoDaddy    │ Active   │ ● Good  │ Tech     │ 180d │ ···  │
│ acme-sso.com │ Route53    │ Expired  │ ✕ Down  │ Business │ -14d │ ···  │
├──────────────┴────────────┴──────────┴─────────┴──────────┴──────┴──────┤
│  Showing 1–25 of 24                               [← 1  →]             │
└──────────────────────────────────────────────────────────────────────────┘
```

### 3.3 Table Columns

| Column | Width | Content | Sortable |
|--------|-------|---------|----------|
| Domain | flex | Domain name. Clickable — opens domain detail slide-over (section 3.7). | Yes |
| Provider | 120px | Provider name. If no provider associated, show `—`. | Yes |
| Status | 100px | Badge: "Active" (`--status-success`), "Pending" (`--status-warning`), "Expired" (`--status-error`), "Suspended" (`--status-error`). | Yes |
| Health | 80px | Composite health indicator from last health check (section 6). Green dot = Good, yellow triangle = Warning, red X = Down, dash = N/A. | Yes |
| Category | 100px | Domain categorization result. "Uncateg." if checked but uncategorized. Dash if never checked. | No |
| Expiration | 70px | Days until expiration. Positive shows as "241d". Negative (expired) shows as "-14d" in `--status-error` color. Dash if unknown. | Yes |
| Actions | 48px | Kebab menu (`···`) | No |

- Default sort: **Domain ascending** (alphabetical).
- Row hover: `--bg-hover` background.
- Clicking a row (outside kebab) opens the domain detail slide-over.

### 3.4 Filters

All filters are applied as query parameters to `GET /api/v1/domains` and are AND-combined.

- **Search**: Text input, debounced at 300ms, minimum 2 characters. Searches across domain name. Maps to `?search=<term>`.
- **Provider** (multi-select dropdown): Options populated from `GET /api/v1/domain-providers`. Maps to `?provider_id=1,2`.
- **Status** (multi-select dropdown): Options: Active, Pending, Expired, Suspended. Maps to `?status=active,pending`.
- **Health** (single-select dropdown): Options: All (default), Good, Warning, Down, Unchecked. Maps to `?health=good`.

**Clear Filters** link appears when any non-default filter is active.

### 3.5 Kebab Menu Actions

| Action | Icon | Behavior |
|--------|------|----------|
| View Details | Eye | Opens domain detail slide-over (section 3.7) |
| Edit | Pencil | Opens domain edit slide-over (section 3.8) |
| Check Health | Activity | Triggers a health check (section 6.1) |
| Check Categorization | Tag | Triggers categorization check (section 6.3) |
| Renew | Refresh | Initiates renewal (section 3.10) |
| Delete | Trash (red) | Opens delete confirmation modal |

"Renew" is only visible for domains with an associated provider that supports renewal. "Check Health" and "Check Categorization" are disabled (grayed, with tooltip "Health check in progress") while a check is already running.

### 3.6 Add Domain Slide-Over

Clicking "[+ Add Domain]" opens a slide-over for manually adding a domain that is already registered and owned.

```
┌───────────────────────────┬──────────────────────────────────┐
│                           │  Add Domain                [✕]  │
│                           │  ────────────────────────────── │
│                           │                                  │
│                           │  Domain Name *                   │
│                           │  [________________________]      │
│                           │                                  │
│                           │  Domain Provider                 │
│                           │  ┌────────────────────────────┐  │
│                           │  │ Select provider...       ▾ │  │
│                           │  └────────────────────────────┘  │
│                           │  (Optional — for automated DNS)  │
│                           │                                  │
│                           │  Notes                           │
│                           │  ┌────────────────────────────┐  │
│                           │  │                            │  │
│                           │  │                            │  │
│                           │  └────────────────────────────┘  │
│                           │                                  │
│                           │  [Cancel]  [Add Domain]          │
└───────────────────────────┴──────────────────────────────────┘
```

- Domain name is validated: must be a valid domain format (letters, numbers, hyphens, dots), no protocol prefix.
- If a provider is selected, the system attempts to verify the domain exists in the provider account after saving.
- API: `POST /api/v1/domains`.
- On success: toast "Domain added.", slide-over closes, list refreshes. The domain detail slide-over opens automatically so the user can configure DNS and email auth.

### 3.7 Domain Detail Slide-Over

Clicking a domain row opens the detail slide-over. This is the most complex slide-over in the system, containing tabbed sub-sections for DNS, email auth, and health.

```
┌───────────────────────────┬──────────────────────────────────────────┐
│                           │  login-hr.com                      [✕]  │
│                           │  ──────────────────────────────────────  │
│                           │  Provider: GoDaddy    Status: Active    │
│                           │  Created: Jan 15, 2026                  │
│                           │  Expires: Dec 2, 2026 (241 days)        │
│                           │  ──────────────────────────────────────  │
│                           │                                          │
│                           │  [DNS Records] [Email Auth] [Health]     │
│                           │  ──────────────────────────────────────  │
│                           │                                          │
│                           │  (Sub-tab content renders here —         │
│                           │   see sections 4, 5, and 6)             │
│                           │                                          │
│                           │  ──────────────────────────────────────  │
│                           │  [Edit Domain]  [Delete]                 │
└───────────────────────────┴──────────────────────────────────────────┘
```

**Slide-over specifications:**
- Width: 640px on desktop (wider than standard to accommodate DNS tables); full-width below 768px.
- Background: `--bg-secondary`.
- Header: domain name as title, close (X) button top-right.
- Metadata section: provider name (linked), status badge, created date, expiration with days remaining.
- Sub-tabs: "DNS Records" (default), "Email Auth", "Health". Sub-tab switching does not close the slide-over.
- Footer: "Edit Domain" navigates to the edit slide-over (replaces current slide-over content). "Delete" opens delete confirmation modal.

### 3.8 Edit Domain Slide-Over

- Same slide-over shell as the detail view, but metadata fields become editable.
- Domain name is read-only (cannot be changed after creation).
- Editable fields: provider (dropdown), notes (textarea).
- API: `PUT /api/v1/domains/{id}`.
- "Save" and "Cancel" buttons replace the footer. Cancel returns to the detail view.

### 3.9 Domain Registration Workflow

Clicking "[Register Domain]" on the domain list opens a multi-step registration flow in a slide-over.

**Step 1 — Domain Search:**

```
┌──────────────────────────────────────────┐
│  Register Domain                   [✕]  │
│  ──────────────────────────────────────  │
│  Step 1 of 3: Search                     │
│                                          │
│  Domain Name *                           │
│  ┌────────────────────────┐              │
│  │ example-domain         │ [Check]      │
│  └────────────────────────┘              │
│                                          │
│  Provider *                              │
│  ┌────────────────────────────┐          │
│  │ GoDaddy                 ▾ │           │
│  └────────────────────────────┘          │
│                                          │
│  Results:                                │
│  ┌──────────────────────────────────┐    │
│  │ example-domain.com    Available  │    │
│  │   $12.99/yr            [Select] │    │
│  │ example-domain.net    Available  │    │
│  │   $10.99/yr            [Select] │    │
│  │ example-domain.org    Taken     │    │
│  │   —                             │    │
│  │ example-domain.io     Available │    │
│  │   $39.99/yr            [Select] │    │
│  └──────────────────────────────────┘    │
│                                          │
│                              [Next →]    │
└──────────────────────────────────────────┘
```

- "Check" calls `POST /api/v1/domains/check-availability` with the entered base domain and selected provider.
- Results show TLD variants with availability and pricing.
- Taken domains are shown grayed out with no select button.
- The user must select exactly one domain to proceed.

**Step 2 — Registration Details:**

```
┌──────────────────────────────────────────┐
│  Register Domain                   [✕]  │
│  ──────────────────────────────────────  │
│  Step 2 of 3: Details                    │
│                                          │
│  Selected: example-domain.com            │
│  Price: $12.99/yr                        │
│                                          │
│  Registration Period                     │
│  ┌────────────────────────────┐          │
│  │ 1 year                  ▾ │           │
│  └────────────────────────────┘          │
│                                          │
│  Privacy Protection                      │
│  [✓] Enable WHOIS privacy ($0.00)        │
│                                          │
│  Auto-Renew                              │
│  [ ] Enable auto-renewal                 │
│                                          │
│  Notes                                   │
│  [_______________________________]       │
│                                          │
│           [← Back]  [Submit Request]     │
└──────────────────────────────────────────┘
```

**Step 3 — Confirmation:**

```
┌──────────────────────────────────────────┐
│  Register Domain                   [✕]  │
│  ──────────────────────────────────────  │
│  Step 3 of 3: Submitted                  │
│                                          │
│     ┌─────┐                              │
│     │  ✓  │                              │
│     └─────┘                              │
│                                          │
│  Registration request submitted for      │
│  example-domain.com                      │
│                                          │
│  An administrator must approve this      │
│  request before the domain is            │
│  registered. You will be notified        │
│  when the request is processed.          │
│                                          │
│  Request ID: REQ-2026-0403-001           │
│  Status: Pending Approval                │
│                                          │
│                            [Done]        │
└──────────────────────────────────────────┘
```

- "Submit Request" calls `POST /api/v1/domains/registration-requests`.
- Registration requests require admin approval. The request enters "pending" state.
- Admins see pending requests in their notification center and can approve/reject via `POST /api/v1/domains/registration-requests/{id}/approve` or `/reject`.
- On approval, the domain is automatically registered and appears in the domain list with "Active" status.
- On rejection, the requester receives a notification with the rejection reason.

### 3.10 Domain Renewal

- Clicking "Renew" from the kebab menu opens a confirmation modal:

```
┌─────────────────────────────────────────┐
│  Renew Domain                     [✕]   │
├─────────────────────────────────────────┤
│                                         │
│  Renew login-hr.com for an additional   │
│  year through GoDaddy?                  │
│                                         │
│  Current expiration: Dec 2, 2026        │
│  New expiration: Dec 2, 2027            │
│                                         │
│              [Cancel]  [Renew]          │
└─────────────────────────────────────────┘
```

- On confirm: `POST /api/v1/domains/{id}/renew`.
- On success: toast "Domain renewed. New expiration: Dec 2, 2027."
- On failure: error toast with the provider's error message.

### 3.11 Delete Domain Confirmation

```
┌─────────────────────────────────────────┐
│  Delete Domain                    [✕]   │
├─────────────────────────────────────────┤
│                                         │
│  Are you sure you want to delete        │
│  login-hr.com?                          │
│                                         │
│  This will remove the domain from       │
│  Tackle. It will NOT cancel the         │
│  registration with the provider.        │
│                                         │
│  ⚠  This domain has 4 DNS records      │
│     and active email authentication     │
│     configuration that will be lost.    │
│                                         │
│              [Cancel]  [Delete]         │
└─────────────────────────────────────────┘
```

- On confirm: `DELETE /api/v1/domains/{id}`.
- If the domain is in use by an active campaign: API returns 409. Error toast: "This domain is in use by an active campaign and cannot be deleted."
- On success: toast "Domain deleted.", row removed from list, slide-over closes if open.

---

## 4. DNS Record Management

### 4.1 Purpose

DNS records are managed within the domain detail slide-over's "DNS Records" sub-tab. Records are edited via modals (not slide-overs) for quick, focused edits. This section covers A, AAAA, CNAME, MX, TXT, NS, and SRV record types.

### 4.2 DNS Records Sub-Tab Layout

```
┌──────────────────────────────────────────────────────────┐
│  DNS Records                              [+ Add Record] │
│  ────────────────────────────────────────────────────── │
│  SOA Record                                              │
│  ┌──────────────────────────────────────────────────┐    │
│  │ Primary NS: ns1.godaddy.com                      │    │
│  │ Admin: admin@login-hr.com                        │    │
│  │ Serial: 2026040301   Refresh: 3600               │    │
│  │ Retry: 900   Expire: 1209600   Min TTL: 300      │    │
│  └──────────────────────────────────────────────────┘    │
│                                                          │
│  ┌──────┬──────────────┬──────────────────┬─────┬─────┐  │
│  │ Type │ Name         │ Value            │ TTL │ ··· │  │
│  ├──────┼──────────────┼──────────────────┼─────┼─────┤  │
│  │ A    │ @            │ 93.184.216.34    │ 300 │ ··· │  │
│  │ A    │ www          │ 93.184.216.34    │ 300 │ ··· │  │
│  │ CNAME│ mail         │ mail.provider.co │ 300 │ ··· │  │
│  │ MX   │ @            │ 10 mail.login-hr │ 300 │ ··· │  │
│  │ TXT  │ @            │ v=spf1 include:..│ 300 │ ··· │  │
│  │ TXT  │ _dmarc       │ v=DMARC1; p=rej..│ 300 │ ··· │  │
│  │ TXT  │ default._dom │ v=DKIM1; k=rsa;..│ 300 │ ··· │  │
│  └──────┴──────────────┴──────────────────┴─────┴─────┘  │
│                                                          │
│  [Check Propagation]                                     │
└──────────────────────────────────────────────────────────┘
```

### 4.3 SOA Record Display

- The SOA record is displayed in a read-only card at the top of the DNS sub-tab.
- Data fetched from `GET /api/v1/domains/{id}/dns/soa`.
- Fields: Primary NS, Admin email, Serial, Refresh, Retry, Expire, Minimum TTL.
- The SOA record is not directly editable from this interface — it is managed by the provider. Displayed for informational purposes.

### 4.4 DNS Record Table

| Column | Width | Content | Sortable |
|--------|-------|---------|----------|
| Type | 60px | Record type badge (A, AAAA, CNAME, MX, TXT, NS, SRV). Color-coded: A/AAAA in blue, CNAME in green, MX in purple, TXT in amber, NS in gray, SRV in teal. | Yes |
| Name | flex | Record name / hostname. "@" represents the root domain. Truncated at 15 chars with tooltip. | Yes |
| Value | flex | Record value. Truncated at 20 chars with tooltip showing the full value. For MX records, priority is shown as a prefix ("10 mail..."). | No |
| TTL | 60px | TTL value in seconds. | Yes |
| Actions | 48px | Kebab menu (`···`) | No |

- Records fetched from `GET /api/v1/domains/{id}/dns`.
- No pagination — all records are displayed (domains typically have fewer than 50 DNS records).
- Default sort: Type ascending, then Name ascending.

### 4.5 Kebab Menu Actions (Per Record)

| Action | Icon | Behavior |
|--------|------|----------|
| Edit | Pencil | Opens edit record modal (section 4.7) |
| Duplicate | Copy | Opens add record modal pre-populated with this record's values (name cleared) |
| Delete | Trash (red) | Opens delete confirmation modal |

### 4.6 Add Record Modal

Clicking "[+ Add Record]" opens a centered modal.

```
┌─────────────────────────────────────────────┐
│  Add DNS Record                       [✕]   │
├─────────────────────────────────────────────┤
│                                             │
│  Record Type *                              │
│  ┌─────────────────────────────────────┐    │
│  │ A                                 ▾ │    │
│  └─────────────────────────────────────┘    │
│                                             │
│  Name *                                     │
│  ┌─────────────────────────────────────┐    │
│  │ @                                   │    │
│  └─────────────────────────────────────┘    │
│  Relative to login-hr.com                   │
│                                             │
│  Value *                                    │
│  ┌─────────────────────────────────────┐    │
│  │                                     │    │
│  └─────────────────────────────────────┘    │
│                                             │
│  TTL                                        │
│  ┌─────────────────────────────────────┐    │
│  │ 300                               ▾ │    │
│  └─────────────────────────────────────┘    │
│                                             │
│                    [Cancel]  [Add Record]    │
└─────────────────────────────────────────────┘
```

**Dynamic Fields by Record Type:**

| Type | Fields |
|------|--------|
| A | Name, IPv4 Address (validated format), TTL |
| AAAA | Name, IPv6 Address (validated format), TTL |
| CNAME | Name (cannot be "@"), Target hostname, TTL |
| MX | Name, Mail server hostname, Priority (0–65535), TTL |
| TXT | Name, Text value (up to 4096 chars, multi-line allowed), TTL |
| NS | Name, Nameserver hostname, TTL |
| SRV | Name (auto-formatted as `_service._protocol`), Target, Port (0–65535), Priority (0–65535), Weight (0–65535), TTL |

**TTL Dropdown Options:** 60 (1 min), 300 (5 min, default), 900 (15 min), 1800 (30 min), 3600 (1 hr), 14400 (4 hr), 86400 (1 day). Custom entry also allowed.

**Validation:**
- IPv4: regex `^(\d{1,3}\.){3}\d{1,3}$` with octet range 0–255.
- IPv6: standard IPv6 format validation.
- Hostnames: valid hostname characters only (letters, digits, hyphens, dots).
- CNAME name cannot be "@" (root) — show inline error "CNAME records cannot be created for the root domain."
- Name field cannot contain the full domain — show hint "Relative to {domain}" below the field.

**API:** `POST /api/v1/domains/{id}/dns`.

On success: toast "DNS record added.", modal closes, record table refreshes.

### 4.7 Edit Record Modal

- Same layout as the Add Record modal, but pre-populated with existing values.
- Record type is read-only (displayed as text, not a dropdown).
- API: `PUT /api/v1/domains/{id}/dns/{record_id}`.
- On success: toast "DNS record updated.", modal closes, record table refreshes.

### 4.8 Delete Record Confirmation

```
┌─────────────────────────────────────────┐
│  Delete DNS Record                [✕]   │
├─────────────────────────────────────────┤
│                                         │
│  Delete A record "@" pointing to        │
│  93.184.216.34?                         │
│                                         │
│  This change will propagate to DNS      │
│  servers. Active campaigns using this   │
│  domain may be affected.                │
│                                         │
│              [Cancel]  [Delete]         │
└─────────────────────────────────────────┘
```

- On confirm: `DELETE /api/v1/domains/{id}/dns/{record_id}`.
- On success: toast "DNS record deleted."

### 4.9 DNS Propagation Check

Clicking "[Check Propagation]" at the bottom of the DNS sub-tab triggers a propagation check for all records.

- API: `POST /api/v1/domains/{id}/dns/propagation-check`.
- During check: button shows spinner and "Checking propagation..."
- Results are displayed inline below the DNS table:

```
┌──────────────────────────────────────────────────────┐
│  Propagation Status          Checked: 2 minutes ago  │
│  ──────────────────────────────────────────────────  │
│  A  @  →  93.184.216.34                              │
│    ● Google DNS (8.8.8.8)         Propagated         │
│    ● Cloudflare (1.1.1.1)        Propagated         │
│    ○ OpenDNS (208.67.222.222)    Pending             │
│                                                      │
│  MX @  →  10 mail.login-hr.com                       │
│    ● Google DNS                   Propagated         │
│    ● Cloudflare                   Propagated         │
│    ● OpenDNS                      Propagated         │
└──────────────────────────────────────────────────────┘
```

- Green dot = propagated, yellow circle = pending.
- Results auto-dismiss after 60 seconds, or the user can close them.

---

## 5. Email Authentication

### 5.1 Purpose

The "Email Auth" sub-tab within the domain detail slide-over provides configuration panels for SPF, DKIM, and DMARC records. These are critical for email deliverability — improperly configured email authentication is the primary cause of phishing emails being flagged or rejected.

### 5.2 Email Auth Sub-Tab Layout

```
┌──────────────────────────────────────────────────────────────────┐
│  Email Authentication                                            │
│  ──────────────────────────────────────────────────────────────  │
│                                                                  │
│  ┌────────────────────────────────────────────────────────────┐  │
│  │  SPF Record                              Status: ● Valid   │  │
│  │  ────────────────────────────────────────────────────────  │  │
│  │  Current Record:                                          │  │
│  │  v=spf1 include:_spf.google.com include:sendgrid.net ~all │  │
│  │                                                            │  │
│  │  [Configure]  [Validate]                                   │  │
│  └────────────────────────────────────────────────────────────┘  │
│                                                                  │
│  ┌────────────────────────────────────────────────────────────┐  │
│  │  DKIM Record                         Status: ● Configured  │  │
│  │  ────────────────────────────────────────────────────────  │  │
│  │  Selector: default                                        │  │
│  │  Key Type: RSA 2048-bit                                   │  │
│  │  Record: default._domainkey.login-hr.com                  │  │
│  │                                                            │  │
│  │  [Configure]  [Validate]  [Rotate Key]                     │  │
│  └────────────────────────────────────────────────────────────┘  │
│                                                                  │
│  ┌────────────────────────────────────────────────────────────┐  │
│  │  DMARC Record                    Status: ▲ Not Configured  │  │
│  │  ────────────────────────────────────────────────────────  │  │
│  │  No DMARC record found for this domain.                   │  │
│  │                                                            │  │
│  │  [Configure]  [Validate]                                   │  │
│  └────────────────────────────────────────────────────────────┘  │
│                                                                  │
│  ┌────────────────────────────────────────────────────────────┐  │
│  │  Overall Email Auth Score: 2/3                             │  │
│  │  ██████████████████████░░░░░░░░  67%                       │  │
│  │  Recommendation: Configure DMARC to improve deliverability │  │
│  └────────────────────────────────────────────────────────────┘  │
└──────────────────────────────────────────────────────────────────┘
```

### 5.3 Status Indicators

Each authentication method shows a status:

| Status | Indicator | Meaning |
|--------|-----------|---------|
| Valid | ● (green) | Record exists and passes validation |
| Configured | ● (blue) | Record exists but has not been validated yet |
| Invalid | ✕ (red) | Record exists but fails validation |
| Not Configured | ▲ (yellow) | No record found |

### 5.4 SPF Configuration Modal

Clicking "Configure" on the SPF panel opens a modal.

```
┌─────────────────────────────────────────────────────┐
│  Configure SPF                                [✕]   │
├─────────────────────────────────────────────────────┤
│                                                     │
│  SPF Mechanism Builder                              │
│                                                     │
│  Include Domains:                                   │
│  ┌─────────────────────────────────┐  [+ Add]       │
│  │ _spf.google.com            [✕] │                 │
│  │ sendgrid.net               [✕] │                 │
│  └─────────────────────────────────┘                │
│                                                     │
│  IP4 Addresses:                                     │
│  ┌─────────────────────────────────┐  [+ Add]       │
│  │ 192.168.1.0/24             [✕] │                 │
│  └─────────────────────────────────┘                │
│                                                     │
│  IP6 Addresses:                                     │
│  ┌─────────────────────────────────┐  [+ Add]       │
│  │ (none)                         │                 │
│  └─────────────────────────────────┘                │
│                                                     │
│  Policy *                                           │
│  ○ ~all (Soft Fail - recommended)                   │
│  ○ -all (Hard Fail)                                 │
│  ○ ?all (Neutral)                                   │
│                                                     │
│  Generated Record:                                  │
│  ┌─────────────────────────────────────────────┐    │
│  │ v=spf1 include:_spf.google.com              │    │
│  │ include:sendgrid.net ip4:192.168.1.0/24 ~all│    │
│  └─────────────────────────────────────────────┘    │
│  (Auto-generated. Editable for advanced users.)     │
│                                                     │
│                       [Cancel]  [Save & Validate]   │
└─────────────────────────────────────────────────────┘
```

- The mechanism builder provides a structured UI for constructing SPF records.
- The "Generated Record" field auto-updates as mechanisms are added/removed.
- Advanced users can directly edit the generated record text.
- "Save & Validate" calls `POST /api/v1/domains/{id}/email-auth/spf` to save, then `POST /api/v1/domains/{id}/email-auth/spf/validate` to validate.
- Validation checks: syntax correctness, DNS lookup limit (max 10), duplicate includes.
- On validation success: toast "SPF record saved and validated." Status updates to "Valid".
- On validation failure: toast warning "SPF record saved but validation failed." The validation errors are displayed inline in the modal (e.g., "Too many DNS lookups (12/10 max)").

### 5.5 DKIM Configuration Modal

```
┌─────────────────────────────────────────────────────┐
│  Configure DKIM                               [✕]   │
├─────────────────────────────────────────────────────┤
│                                                     │
│  Selector *                                         │
│  ┌─────────────────────────────────────────────┐    │
│  │ default                                     │    │
│  └─────────────────────────────────────────────┘    │
│                                                     │
│  Key Type *                                         │
│  ○ RSA 2048-bit (recommended)                       │
│  ○ RSA 1024-bit                                     │
│  ○ Ed25519                                          │
│                                                     │
│  Public Key                                         │
│  ┌─────────────────────────────────────────────┐    │
│  │ MIIBIjANBgkqhkiG9w0BAQEFAAOCAQ8AMIIBCg... │    │
│  │                                             │    │
│  └─────────────────────────────────────────────┘    │
│  [Generate New Key Pair]                            │
│                                                     │
│  DNS Record to Create:                              │
│  ┌─────────────────────────────────────────────┐    │
│  │ Name: default._domainkey.login-hr.com       │    │
│  │ Type: TXT                                   │    │
│  │ Value: v=DKIM1; k=rsa; p=MIIBIjANBgk...    │    │
│  └─────────────────────────────────────────────┘    │
│  [Auto-Create DNS Record]                           │
│                                                     │
│                       [Cancel]  [Save & Validate]   │
└─────────────────────────────────────────────────────┘
```

- "Generate New Key Pair" calls the backend to generate a key pair. The public key is displayed; the private key is stored server-side (never sent to the frontend).
- "Auto-Create DNS Record" automatically creates the required TXT record in the domain's DNS if the domain has a connected provider. If no provider is connected, this button is disabled with tooltip "Connect a domain provider to auto-create DNS records."
- "Rotate Key" (from the panel) generates a new key pair, updates the DNS record, and invalidates the old key. A confirmation modal appears first: "Rotating the DKIM key will invalidate all currently signed emails. Continue?"
- API: `POST /api/v1/domains/{id}/email-auth/dkim`.

### 5.6 DMARC Configuration Modal

```
┌─────────────────────────────────────────────────────┐
│  Configure DMARC                              [✕]   │
├─────────────────────────────────────────────────────┤
│                                                     │
│  Policy *                                           │
│  ○ none (Monitor only)                              │
│  ○ quarantine (Send to spam)                        │
│  ○ reject (Block delivery - strictest)              │
│                                                     │
│  Subdomain Policy                                   │
│  ┌─────────────────────────────────────────────┐    │
│  │ Same as domain policy                     ▾ │    │
│  └─────────────────────────────────────────────┘    │
│                                                     │
│  Aggregate Report Email (rua)                       │
│  ┌─────────────────────────────────────────────┐    │
│  │ dmarc-reports@login-hr.com                  │    │
│  └─────────────────────────────────────────────┘    │
│                                                     │
│  Forensic Report Email (ruf)                        │
│  ┌─────────────────────────────────────────────┐    │
│  │                                             │    │
│  └─────────────────────────────────────────────┘    │
│                                                     │
│  Percentage (pct)                                   │
│  ┌────────────────┐                                 │
│  │ 100          ▾ │  (% of messages to apply to)    │
│  └────────────────┘                                 │
│                                                     │
│  DKIM Alignment                                     │
│  ○ Relaxed (r)   ○ Strict (s)                       │
│                                                     │
│  SPF Alignment                                      │
│  ○ Relaxed (r)   ○ Strict (s)                       │
│                                                     │
│  Generated Record:                                  │
│  ┌─────────────────────────────────────────────┐    │
│  │ v=DMARC1; p=reject; sp=reject; pct=100;    │    │
│  │ rua=mailto:dmarc-reports@login-hr.com;      │    │
│  │ adkim=r; aspf=r                             │    │
│  └─────────────────────────────────────────────┘    │
│                                                     │
│                       [Cancel]  [Save & Validate]   │
└─────────────────────────────────────────────────────┘
```

- Generated record auto-updates as options change.
- API: `POST /api/v1/domains/{id}/email-auth/dmarc`.
- Validation checks: syntax, valid policy, report email format.

### 5.7 Validate Button

Each panel's "Validate" button triggers a standalone validation without saving changes.

- API: `POST /api/v1/domains/{id}/email-auth/{type}/validate` where type is `spf`, `dkim`, or `dmarc`.
- During validation: button shows spinner.
- On success: toast "SPF record is valid." (or DKIM/DMARC). Status updates.
- On failure: toast warning with the specific validation error.

### 5.8 Overall Email Auth Score

The score card at the bottom provides an at-a-glance assessment:

- Score is X/3 based on how many of SPF, DKIM, DMARC are configured AND validated.
- Progress bar fills proportionally in `--accent` color.
- Recommendations are auto-generated based on missing or invalid configurations:
  - No SPF: "Configure SPF to specify authorized senders."
  - No DKIM: "Configure DKIM to sign outgoing emails."
  - No DMARC: "Configure DMARC to improve deliverability."
  - SPF invalid: "Fix SPF record — currently failing validation."

---

## 6. Domain Health and Categorization

### 6.1 Health Check

The "Health" sub-tab in the domain detail slide-over displays the results of health checks.

```
┌──────────────────────────────────────────────────────────────────┐
│  Domain Health                    Last checked: 15 minutes ago  │
│  ──────────────────────────────────────────────────────────────  │
│                                                                  │
│  Overall: ● Healthy                                              │
│                                                                  │
│  ┌──────────────────────────────────┬────────┬─────────────────┐ │
│  │ Check                           │ Status │ Detail          │ │
│  ├──────────────────────────────────┼────────┼─────────────────┤ │
│  │ DNS Resolution                  │ ● Pass │ Resolves OK     │ │
│  │ HTTP Reachability               │ ● Pass │ 200 OK (143ms)  │ │
│  │ HTTPS/TLS Certificate           │ ● Pass │ Valid, 89 days  │ │
│  │ SPF Record                      │ ● Pass │ Valid           │ │
│  │ DKIM Record                     │ ● Pass │ Valid           │ │
│  │ DMARC Record                    │ ▲ Warn │ Not configured  │ │
│  │ Domain Expiration               │ ● Pass │ 241 days        │ │
│  │ Blacklist Check                 │ ● Pass │ Not listed      │ │
│  └──────────────────────────────────┴────────┴─────────────────┘ │
│                                                                  │
│  [Run Health Check]                                              │
└──────────────────────────────────────────────────────────────────┘
```

**Health Check Behavior:**
- "Run Health Check" calls `POST /api/v1/domains/{id}/health-check`.
- During the check: button shows spinner, text changes to "Checking..."
- Results update inline as each check completes (the API may return results progressively via SSE, or the frontend polls until complete).
- Overall status is the worst status among all checks: all Pass = Healthy, any Warn = Warning, any Fail = Down.
- "Last checked" timestamp updates after completion.

**Status indicators:**
- ● Pass: green dot, `--status-success`.
- ▲ Warn: yellow triangle, `--status-warning`.
- ✕ Fail: red X, `--status-error`.

### 6.2 Health Check Details

Clicking any row in the health check table expands an inline detail panel below the row:

```
│ HTTPS/TLS Certificate           │ ● Pass │ Valid, 89 days  │
│  ┌──────────────────────────────────────────────────────┐  │
│  │ Certificate: Let's Encrypt Authority X3              │  │
│  │ Subject: login-hr.com                                │  │
│  │ Issued: Jan 3, 2026                                  │  │
│  │ Expires: Apr 3, 2026 (89 days)                       │  │
│  │ SANs: login-hr.com, www.login-hr.com                 │  │
│  └──────────────────────────────────────────────────────┘  │
```

- Detail content varies by check type.
- Only one row can be expanded at a time (accordion behavior).

### 6.3 Domain Categorization

Categorization checks determine how web filtering services classify the domain. This is crucial for phishing simulations — domains categorized as "Phishing" or "Malicious" will be blocked by corporate firewalls.

```
┌──────────────────────────────────────────────────────────────────┐
│  Categorization                   Last checked: 2 days ago      │
│  ──────────────────────────────────────────────────────────────  │
│                                                                  │
│  ┌──────────────────────────────┬────────────────────────┐       │
│  │ Service                     │ Category               │       │
│  ├──────────────────────────────┼────────────────────────┤       │
│  │ Symantec/Bluecoat           │ Business/Economy       │       │
│  │ McAfee/TrustedSource        │ Minimal Risk           │       │
│  │ Fortinet/FortiGuard         │ Information Technology  │       │
│  │ Palo Alto URL Filtering     │ Business and Economy   │       │
│  │ Cisco Talos                 │ Uncategorized          │       │
│  └──────────────────────────────┴────────────────────────┘       │
│                                                                  │
│  [Check Categorization]                                          │
└──────────────────────────────────────────────────────────────────┘
```

- Categorization data is part of the Health sub-tab, displayed below the health check table.
- "Check Categorization" calls `POST /api/v1/domains/{id}/categorization-check`.
- Results show how each major web filtering service categorizes the domain.
- Categories flagged as suspicious (Phishing, Malware, Spam, etc.) are highlighted in `--status-error` color with a warning icon.
- "Uncategorized" is shown in `--text-muted` — this is generally acceptable for new domains.

---

## 7. SMTP Profile Management

### 7.1 Purpose

SMTP profiles define the email sending configuration used by campaigns. Each profile specifies the mail server connection, authentication, TLS settings, sending rate limits, and default sender information. Profiles are reusable across campaigns.

### 7.2 Layout

```
┌──────────────────────────────────────────────────────────────────────────┐
│  SMTP Profiles                                  [+ New SMTP Profile]    │
├──────────────────────────────────────────────────────────────────────────┤
│  ┌────────────────────────┐ ┌──────────────┐                            │
│  │ 🔍 Search profiles...  │ │ Status ▾     │  [Clear Filters]           │
│  └────────────────────────┘ └──────────────┘                            │
├──────────────────┬───────────────┬──────────┬──────────┬────────┬───────┤
│ Name             │ Host          │ From     │ Status   │Updated │  ···  │
├──────────────────┼───────────────┼──────────┼──────────┼────────┼───────┤
│ Primary Relay    │ smtp.relay.co │ it@hr.co │ ● Healthy│ 2h ago │  ···  │
│ Backup Server    │ mail.bak.com  │ no-reply │ ● Healthy│ 1d ago │  ···  │
│ Dev SMTP         │ smtp.dev.io   │ test@dev │ ○ Error  │ 3d ago │  ···  │
│ New Profile      │ smtp.new.com  │ —        │ — Untest.│ 5m ago │  ···  │
├──────────────────┴───────────────┴──────────┴──────────┴────────┴───────┤
│  Showing 1–25 of 4                                                      │
└──────────────────────────────────────────────────────────────────────────┘
```

### 7.3 Table Columns

| Column | Width | Content | Sortable |
|--------|-------|---------|----------|
| Name | flex | Profile name. Clickable — opens detail slide-over. | Yes |
| Host | 140px | SMTP server hostname. Truncated at 20 chars with tooltip. | Yes |
| From | 120px | `from_address` truncated. Dash if not set. | No |
| Status | 90px | "Healthy" (green dot), "Error" (red circle), "Untested" (gray dash). | Yes |
| Updated | 80px | Relative timestamp. Tooltip for absolute datetime. | Yes (default descending) |
| Actions | 48px | Kebab menu (`···`) | No |

- Default sort: `updated_at` descending.
- Clicking a row (outside kebab) opens the SMTP profile detail slide-over.

### 7.4 Filters

- **Search**: Debounced 300ms, minimum 2 characters. Searches `name`, `host`, `from_address`. Maps to `?search=<term>`.
- **Status** (single-select dropdown): All (default), Healthy, Error, Untested. Maps to `?status=<value>`.

### 7.5 Kebab Menu Actions

| Action | Icon | Behavior |
|--------|------|----------|
| View Details | Eye | Opens SMTP profile detail slide-over |
| Edit | Pencil | Opens SMTP profile edit slide-over |
| Test Connection | Refresh | Triggers SMTP connection test (section 7.8) |
| Send Test Email | Paper plane | Opens test email modal (section 7.9) |
| Duplicate | Copy | `POST /api/v1/smtp-profiles/{id}/duplicate`. Creates a copy named "{name} (Copy)". Toast: "Profile duplicated." |
| Delete | Trash (red) | Opens delete confirmation modal |

### 7.6 Create SMTP Profile Slide-Over

Clicking "[+ New SMTP Profile]" opens a slide-over.

```
┌───────────────────────────┬──────────────────────────────────────────┐
│                           │  New SMTP Profile                  [✕]  │
│                           │  ──────────────────────────────────────  │
│                           │                                          │
│                           │  ── General ──                           │
│                           │  Profile Name *                          │
│                           │  [________________________]              │
│                           │                                          │
│                           │  ── Server Settings ──                   │
│                           │  SMTP Host *                             │
│                           │  [________________________]              │
│                           │  Port *                                  │
│                           │  [  587  ]                               │
│                           │                                          │
│                           │  TLS Mode *                              │
│                           │  ○ STARTTLS (recommended)                │
│                           │  ○ TLS/SSL                               │
│                           │  ○ None                                  │
│                           │                                          │
│                           │  ── Authentication ──                    │
│                           │  Auth Type *                              │
│                           │  ┌────────────────────────────┐          │
│                           │  │ PLAIN                    ▾ │          │
│                           │  └────────────────────────────┘          │
│                           │  Username                                │
│                           │  [________________________]              │
│                           │  Password                                │
│                           │  [________________________]   👁          │
│                           │                                          │
│                           │  ── Sender Defaults ──                   │
│                           │  From Address *                          │
│                           │  [________________________]              │
│                           │  From Name                               │
│                           │  [________________________]              │
│                           │  Reply-To Address                        │
│                           │  [________________________]              │
│                           │                                          │
│                           │  ── Rate Limiting ──                     │
│                           │  Max Send Rate (emails/min)              │
│                           │  [  60   ]                               │
│                           │  Max Connections                         │
│                           │  [   5   ]                               │
│                           │                                          │
│                           │  ── Timeouts ──                          │
│                           │  Connect Timeout (seconds)               │
│                           │  [  30   ]                               │
│                           │  Read Timeout (seconds)                  │
│                           │  [  60   ]                               │
│                           │                                          │
│                           │        [Cancel]  [Test & Save]           │
└───────────────────────────┴──────────────────────────────────────────┘
```

**Auth Type Options:**

| Auth Type | Additional Fields |
|-----------|-------------------|
| None | No username/password fields shown |
| PLAIN | Username, Password |
| LOGIN | Username, Password |
| CRAM-MD5 | Username, Password |
| XOAUTH2 | Token endpoint URL, Client ID, Client Secret, Scope |

- When "None" is selected, the Username and Password fields are hidden.
- When "XOAUTH2" is selected, the standard username/password fields are replaced with OAuth2-specific fields.
- Password field uses `type="password"` with toggle-visibility eye icon.

**Port Defaults by TLS Mode:**
- STARTTLS: 587 (auto-filled when TLS mode changes, user can override).
- TLS/SSL: 465.
- None: 25.

**Validation:**
- Profile Name: required, 1–100 characters.
- SMTP Host: required, valid hostname or IP.
- Port: required, 1–65535.
- From Address: required, valid email format.
- Max Send Rate: optional, positive integer.
- Max Connections: optional, 1–100.
- Timeouts: optional, 1–300 seconds.

**"Test & Save" Behavior:**
1. Client-side validation first.
2. Call `POST /api/v1/smtp-profiles/test` with the entered configuration.
3. If test succeeds: call `POST /api/v1/smtp-profiles` to save. Toast: "SMTP profile created and connection verified." Status set to "Healthy".
4. If test fails: show error toast "SMTP connection test failed." and open a detail modal with the failure reason (see section 7.8). The profile is NOT saved.

**Unsaved Changes:**
- If the form has unsaved changes and the user clicks Cancel or the close button, a confirmation dialog appears: "Discard unsaved changes?"

### 7.7 Edit SMTP Profile Slide-Over

- Same layout as Create, pre-populated with existing data.
- Password field shows masked placeholder ("••••••••"). Leaving it blank preserves the existing password. Entering a new value replaces it.
- API: `PUT /api/v1/smtp-profiles/{id}`.
- "Test & Save" behavior is identical to Create.

### 7.8 SMTP Connection Test

Triggered from the kebab menu "Test Connection" or during "Test & Save".

- API: `POST /api/v1/smtp-profiles/{id}/test` (for existing profiles) or `POST /api/v1/smtp-profiles/test` (for unsaved profiles with inline config).
- During test: if triggered from kebab, the status column shows a spinner. If triggered from the slide-over, the "Test & Save" button shows a spinner.
- On success: toast "SMTP connection verified." Profile status updates to "Healthy".
- On failure: error toast "SMTP connection test failed: {brief_error}". A detail modal opens:

```
┌─────────────────────────────────────────────────┐
│  SMTP Connection Test Failed              [✕]   │
├─────────────────────────────────────────────────┤
│                                                 │
│  Profile: Primary Relay                         │
│  Host: smtp.relay.co:587                        │
│  Tested: Apr 3, 2026 at 2:15 PM                │
│                                                 │
│  Steps Completed:                               │
│  ● DNS resolution                      Pass     │
│  ● TCP connection (port 587)           Pass     │
│  ● STARTTLS negotiation                Pass     │
│  ✕ SMTP AUTH (PLAIN)                   Failed   │
│                                                 │
│  Error Details:                                 │
│  ┌─────────────────────────────────────────┐    │
│  │ 535 5.7.8 Authentication credentials    │    │
│  │ invalid. Learn more at                  │    │
│  │ https://support.google.com/mail/?p=...  │    │
│  └─────────────────────────────────────────┘    │
│                                                 │
│                                    [Close]      │
└─────────────────────────────────────────────────┘
```

- The modal shows which steps in the connection process passed and which failed, enabling targeted troubleshooting.
- Profile status updates to "Error".

### 7.9 Send Test Email Modal

```
┌─────────────────────────────────────────────────┐
│  Send Test Email                          [✕]   │
├─────────────────────────────────────────────────┤
│                                                 │
│  Using Profile: Primary Relay                   │
│                                                 │
│  Recipient Email *                              │
│  ┌─────────────────────────────────────────┐    │
│  │ admin@company.com                       │    │
│  └─────────────────────────────────────────┘    │
│                                                 │
│  Subject                                        │
│  ┌─────────────────────────────────────────┐    │
│  │ Tackle SMTP Test                        │    │
│  └─────────────────────────────────────────┘    │
│                                                 │
│  Body                                           │
│  ┌─────────────────────────────────────────┐    │
│  │ This is a test email sent from Tackle   │    │
│  │ to verify SMTP profile configuration.   │    │
│  └─────────────────────────────────────────┘    │
│                                                 │
│                        [Cancel]  [Send Test]    │
└─────────────────────────────────────────────────┘
```

- Subject and Body have default values that the user can override.
- "Send Test" calls `POST /api/v1/smtp-profiles/{id}/test` with the recipient, subject, and body as additional parameters (or a separate `/send-test` endpoint).
- During send: button shows spinner and "Sending..."
- On success: toast "Test email sent to admin@company.com."
- On failure: error toast with the error message. The SMTP test failure detail modal opens (same as section 7.8) with the send-specific error.

### 7.10 Delete SMTP Profile Confirmation

```
┌─────────────────────────────────────────┐
│  Delete SMTP Profile              [✕]   │
├─────────────────────────────────────────┤
│                                         │
│  Are you sure you want to delete        │
│  "Primary Relay"?                       │
│                                         │
│  This profile will be permanently       │
│  removed.                               │
│                                         │
│              [Cancel]  [Delete]         │
└─────────────────────────────────────────┘
```

- On confirm: `DELETE /api/v1/smtp-profiles/{id}`.
- If in use by an active campaign: 409. Error toast: "This SMTP profile is in use by an active campaign and cannot be deleted."
- On success: toast "SMTP profile deleted.", row removed from list.

---

## 8. Cloud Credential Management

### 8.1 Purpose

Cloud credentials store encrypted authentication details for AWS and Azure accounts used to provision campaign infrastructure. Credentials are encrypted at rest using AES-256-GCM and are never returned in full from the API — only masked representations are available in the frontend.

### 8.2 Layout

Cloud credentials are displayed as provider-grouped cards rather than a flat table.

```
┌──────────────────────────────────────────────────────────────────────────┐
│  Cloud Credentials                            [+ Add Credential]        │
├──────────────────────────────────────────────────────────────────────────┤
│                                                                          │
│  AWS                                                                     │
│  ┌─────────────────────────────┐  ┌─────────────────────────────┐       │
│  │  Production AWS        ···  │  │  Dev/Test AWS          ···  │       │
│  │  ─────────────────────────  │  │  ─────────────────────────  │       │
│  │  Access Key: ••••••••4B1C   │  │  Access Key: ••••••••9F3D   │       │
│  │  Region: us-east-1          │  │  Region: us-west-2          │       │
│  │  Status: ● Connected        │  │  Status: ○ Error            │       │
│  │  Last tested: 1h ago        │  │  Last tested: 5d ago        │       │
│  │  Instances: 3 active        │  │  Instances: 0               │       │
│  │                             │  │                             │       │
│  │  [Test Connection]          │  │  [Test Connection]          │       │
│  └─────────────────────────────┘  └─────────────────────────────┘       │
│                                                                          │
│  Azure                                                                   │
│  ┌─────────────────────────────┐                                        │
│  │  Azure Production      ···  │                                        │
│  │  ─────────────────────────  │                                        │
│  │  Client ID: ••••••••8F2D    │                                        │
│  │  Tenant: ••••••••2A4E       │                                        │
│  │  Status: ● Connected        │                                        │
│  │  Last tested: 3h ago        │                                        │
│  │  Instances: 1 active        │                                        │
│  │                             │                                        │
│  │  [Test Connection]          │                                        │
│  └─────────────────────────────┘                                        │
│                                                                          │
└──────────────────────────────────────────────────────────────────────────┘
```

### 8.3 Card Specifications

- Cards grouped under provider headings ("AWS", "Azure").
- Responsive grid: 2 columns on desktop (>1024px), 1 column below.
- Card background: `--bg-secondary`, border: `--border-primary`.
- All credential values are masked with bullets showing only the last 4 characters. These are returned pre-masked from the API — the frontend never receives full credentials.
- "Instances" count shows the number of active instances provisioned using this credential.
- Status dot: green "Connected", red "Error", gray "Untested".

### 8.4 Kebab Menu Actions

| Action | Icon | Behavior |
|--------|------|----------|
| Edit | Pencil | Opens edit slide-over (section 8.6) |
| Test Connection | Refresh | Triggers connection test (section 8.7) |
| Delete | Trash (red) | Opens delete confirmation modal (section 8.8) |

### 8.5 Add Credential Slide-Over

```
┌───────────────────────────┬──────────────────────────────────────────┐
│                           │  Add Cloud Credential              [✕]  │
│                           │  ──────────────────────────────────────  │
│                           │                                          │
│                           │  Cloud Provider *                        │
│                           │  ┌────────────────────────────┐          │
│                           │  │ Select provider...       ▾ │          │
│                           │  └────────────────────────────┘          │
│                           │                                          │
│                           │  ── AWS ──                               │
│                           │  Credential Name *                       │
│                           │  [________________________]              │
│                           │  Access Key ID *                         │
│                           │  [________________________]              │
│                           │  Secret Access Key *                     │
│                           │  [________________________]   👁          │
│                           │  Default Region *                        │
│                           │  ┌────────────────────────────┐          │
│                           │  │ us-east-1                ▾ │          │
│                           │  └────────────────────────────┘          │
│                           │                                          │
│                           │  ── Azure ──                             │
│                           │  Credential Name *                       │
│                           │  [________________________]              │
│                           │  Tenant ID *                             │
│                           │  [________________________]              │
│                           │  Client ID *                             │
│                           │  [________________________]              │
│                           │  Client Secret *                         │
│                           │  [________________________]   👁          │
│                           │  Subscription ID *                       │
│                           │  [________________________]              │
│                           │                                          │
│                           │        [Cancel]  [Test & Save]           │
└───────────────────────────┴──────────────────────────────────────────┘
```

- Provider selection dynamically renders the appropriate credential fields.
- Secret fields use `type="password"` with toggle-visibility.
- AWS region dropdown populated from a static list of AWS regions.

**"Test & Save" Behavior:**
1. Client-side validation.
2. Call `POST /api/v1/cloud-credentials/test` with the entered credentials.
   - AWS: uses `sts:GetCallerIdentity` to validate credentials.
   - Azure: attempts to list DNS zones to validate credentials.
3. On success: save via `POST /api/v1/cloud-credentials`. Toast: "Cloud credential added and verified." Credentials are encrypted server-side using AES-256-GCM before storage.
4. On failure: error toast "Connection test failed: {error}". Detail modal opens (similar to provider test failure in section 2.7). Credential is NOT saved.

### 8.6 Edit Credential Slide-Over

- Same layout as Add, pre-populated.
- Provider type is read-only.
- Secret fields show masked placeholder ("••••••••"). Leaving blank preserves existing value.
- API: `PUT /api/v1/cloud-credentials/{id}`.

### 8.7 Connection Test

- API: `POST /api/v1/cloud-credentials/{id}/test`.
- During test: button shows spinner, "Testing..."
- On success: toast "Connection verified." Card status updates to "Connected".
- On failure: toast "Connection test failed." Status updates to "Error". Detail modal:

```
┌─────────────────────────────────────────────────┐
│  Cloud Credential Test Failed             [✕]   │
├─────────────────────────────────────────────────┤
│                                                 │
│  Credential: Production AWS                     │
│  Provider: AWS                                  │
│  Tested: Apr 3, 2026 at 2:15 PM                │
│                                                 │
│  Test Method: sts:GetCallerIdentity             │
│                                                 │
│  Error:                                         │
│  ┌─────────────────────────────────────────┐    │
│  │ InvalidClientTokenId: The security      │    │
│  │ token included in the request is        │    │
│  │ invalid.                                │    │
│  └─────────────────────────────────────────┘    │
│                                                 │
│                                    [Close]      │
└─────────────────────────────────────────────────┘
```

### 8.8 Delete Credential Confirmation

```
┌─────────────────────────────────────────┐
│  Delete Cloud Credential          [✕]   │
├─────────────────────────────────────────┤
│                                         │
│  Are you sure you want to delete        │
│  "Production AWS"?                      │
│                                         │
│  ⚠  3 active instances are using this  │
│     credential. They will continue to   │
│     run but cannot be managed through   │
│     Tackle.                             │
│                                         │
│              [Cancel]  [Delete]         │
└─────────────────────────────────────────┘
```

- Warning shows the count of active instances using this credential.
- On confirm: `DELETE /api/v1/cloud-credentials/{id}`.
- If instances are active, the API still allows deletion but shows the warning. The instances themselves are not terminated.
- On success: toast "Credential deleted.", card removed.

---

## 9. Instance Template Management

### 9.1 Purpose

Instance templates define the compute resource configuration used to provision campaign infrastructure. Templates specify the cloud region, instance size, OS image, security groups, SSH keys, user data scripts, and tags. Templates support versioning so that changes are tracked and previous configurations can be referenced.

### 9.2 Layout

```
┌──────────────────────────────────────────────────────────────────────────┐
│  Instance Templates                            [+ New Template]          │
├──────────────────────────────────────────────────────────────────────────┤
│  ┌────────────────────────┐ ┌──────────────┐                            │
│  │ 🔍 Search templates... │ │ Provider ▾   │  [Clear Filters]           │
│  └────────────────────────┘ └──────────────┘                            │
├──────────────────┬──────────┬───────────┬─────────┬──────────┬──────────┤
│ Name             │ Provider │ Region    │ Size    │ Version  │   ···    │
├──────────────────┼──────────┼───────────┼─────────┼──────────┼──────────┤
│ Standard Landing │ AWS      │ us-east-1 │ t3.micro│ v3       │   ···    │
│ Heavy Traffic    │ AWS      │ us-west-2 │ t3.small│ v1       │   ···    │
│ Azure Standard   │ Azure    │ eastus    │ B1s     │ v2       │   ···    │
│ Redirect Server  │ AWS      │ eu-west-1 │ t3.nano │ v5       │   ···    │
├──────────────────┴──────────┴───────────┴─────────┴──────────┴──────────┤
│  Showing 1–25 of 4                                                      │
└──────────────────────────────────────────────────────────────────────────┘
```

### 9.3 Table Columns

| Column | Width | Content | Sortable |
|--------|-------|---------|----------|
| Name | flex | Template name. Clickable — opens detail slide-over. | Yes |
| Provider | 80px | "AWS" or "Azure" badge. | Yes |
| Region | 100px | Cloud region identifier. | Yes |
| Size | 90px | Instance size (e.g., t3.micro, B1s). | Yes |
| Version | 70px | Current version number (e.g., "v3"). | Yes |
| Actions | 48px | Kebab menu (`···`) | No |

- Default sort: Name ascending.
- Row click opens the template detail slide-over.

### 9.4 Filters

- **Search**: Debounced 300ms, minimum 2 characters. Searches `name`. Maps to `?search=<term>`.
- **Provider** (single-select dropdown): All (default), AWS, Azure. Maps to `?provider=aws`.

### 9.5 Kebab Menu Actions

| Action | Icon | Behavior |
|--------|------|----------|
| View Details | Eye | Opens template detail slide-over |
| Edit (New Version) | Pencil | Opens edit slide-over, creates new version on save |
| Duplicate | Copy | Creates a copy named "{name} (Copy)" at v1 |
| View Version History | History | Opens version history modal (section 9.8) |
| Delete | Trash (red) | Opens delete confirmation modal |

### 9.6 Create Template Slide-Over

```
┌───────────────────────────┬──────────────────────────────────────────┐
│                           │  New Instance Template             [✕]  │
│                           │  ──────────────────────────────────────  │
│                           │                                          │
│                           │  Template Name *                         │
│                           │  [________________________]              │
│                           │                                          │
│                           │  Cloud Credential *                      │
│                           │  ┌────────────────────────────┐          │
│                           │  │ Production AWS           ▾ │          │
│                           │  └────────────────────────────┘          │
│                           │  (Determines AWS vs Azure fields)        │
│                           │                                          │
│                           │  Region *                                │
│                           │  ┌────────────────────────────┐          │
│                           │  │ us-east-1                ▾ │          │
│                           │  └────────────────────────────┘          │
│                           │                                          │
│                           │  Instance Size *                         │
│                           │  ┌────────────────────────────┐          │
│                           │  │ t3.micro                 ▾ │          │
│                           │  └────────────────────────────┘          │
│                           │                                          │
│                           │  OS Image *                              │
│                           │  ┌────────────────────────────┐          │
│                           │  │ Ubuntu 22.04 LTS         ▾ │          │
│                           │  └────────────────────────────┘          │
│                           │                                          │
│                           │  Security Groups                         │
│                           │  ┌────────────────────────────┐          │
│                           │  │ sg-0abc1234  (web-server)▾ │          │
│                           │  └────────────────────────────┘          │
│                           │  (Multi-select)                          │
│                           │                                          │
│                           │  SSH Key Reference                       │
│                           │  ┌────────────────────────────┐          │
│                           │  │ tackle-prod-key          ▾ │          │
│                           │  └────────────────────────────┘          │
│                           │                                          │
│                           │  User Data (startup script)              │
│                           │  ┌────────────────────────────┐          │
│                           │  │ #!/bin/bash               │          │
│                           │  │ apt-get update             │          │
│                           │  │ apt-get install -y nginx   │          │
│                           │  │                            │          │
│                           │  └────────────────────────────┘          │
│                           │  (Monospace font, syntax highlighting)    │
│                           │                                          │
│                           │  Tags                                    │
│                           │  ┌──────────────┬──────────────┐         │
│                           │  │ Key          │ Value        │  [+ Add]│
│                           │  ├──────────────┼──────────────┤         │
│                           │  │ Environment  │ production   │  [✕]    │
│                           │  │ Team         │ security     │  [✕]    │
│                           │  └──────────────┴──────────────┘         │
│                           │                                          │
│                           │         [Cancel]  [Save Template]        │
└───────────────────────────┴──────────────────────────────────────────┘
```

**Dynamic Field Loading:**
- Selecting a Cloud Credential determines the provider (AWS or Azure), which affects available regions, instance sizes, OS images, security groups, and SSH keys.
- Region dropdown is populated after credential selection. Changing the region may affect available instance sizes and OS images.
- Security Groups are populated from `GET /api/v1/cloud-credentials/{id}/security-groups?region=<region>`.
- SSH Keys are populated from `GET /api/v1/cloud-credentials/{id}/ssh-keys?region=<region>`.
- OS Images are populated from a curated list maintained by the backend.

**User Data Field:**
- Uses a monospace code editor with basic syntax highlighting for bash/shell scripts.
- Max 16KB (AWS user data limit).

**Tags:**
- Key-value pairs displayed in a dynamic table.
- "[+ Add]" appends a new empty row.
- Each row has a remove button.
- Keys must be unique — duplicate key validation with inline error.

**API:** `POST /api/v1/instance-templates`. The template is created at version 1.

### 9.7 Edit Template Slide-Over

- Same layout as Create, pre-populated with current version data.
- All fields are editable.
- On save: `PUT /api/v1/instance-templates/{id}` — the backend automatically increments the version number.
- Toast: "Template updated (v{new_version})."
- The previous version is preserved in version history.

### 9.8 Version History Modal

```
┌─────────────────────────────────────────────────────┐
│  Version History: Standard Landing            [✕]   │
├─────────────────────────────────────────────────────┤
│                                                     │
│  ┌─────┬───────────────┬────────────┬─────────────┐ │
│  │ Ver │ Changed By    │ Date       │ Action      │ │
│  ├─────┼───────────────┼────────────┼─────────────┤ │
│  │ v3  │ Jane Smith    │ Apr 2, 26  │ [View] ★    │ │
│  │ v2  │ Jane Smith    │ Mar 15, 26 │ [View]      │ │
│  │ v1  │ Bob Lee       │ Feb 1, 26  │ [View]      │ │
│  └─────┴───────────────┴────────────┴─────────────┘ │
│                                                     │
│  ★ = Current active version                         │
│                                                     │
│                                        [Close]      │
└─────────────────────────────────────────────────────┘
```

- "[View]" opens a read-only slide-over showing the template configuration as it was at that version.
- The current (latest) version is marked with a star.
- Versions cannot be deleted or reverted — they are an immutable audit trail.
- If needed, a user can view an old version and then manually re-create its configuration as a new version via the edit flow.

### 9.9 Delete Template Confirmation

```
┌─────────────────────────────────────────┐
│  Delete Instance Template         [✕]   │
├─────────────────────────────────────────┤
│                                         │
│  Are you sure you want to delete        │
│  "Standard Landing" and all 3           │
│  versions?                              │
│                                         │
│  This will not affect any currently     │
│  running instances that were            │
│  provisioned from this template.        │
│                                         │
│              [Cancel]  [Delete]         │
└─────────────────────────────────────────┘
```

- On confirm: `DELETE /api/v1/instance-templates/{id}`.
- Deletes all versions.
- On success: toast "Template deleted."

---

## 10. Endpoint Monitoring (Campaign Context)

### 10.1 Purpose

Endpoints are campaign infrastructure instances (VMs, containers) provisioned to host landing pages and collect campaign data. They are provisioned per-campaign during the campaign build phase and terminated after campaign completion. Endpoints are NEVER reused between campaigns and are NOT managed as standalone entities — they are always viewed and controlled from within the campaign workspace.

### 10.2 Endpoint Lifecycle

```
  ┌───────────┐    ┌──────────────┐    ┌─────────────┐    ┌────────┐
  │ Requested │ ─→ │ Provisioning │ ─→ │ Configuring │ ─→ │ Active │
  └───────────┘    └──────────────┘    └─────────────┘    └────────┘
                          │                                    │
                          ▼                                    ▼
                     ┌─────────┐                         ┌─────────┐
                     │  Error  │                         │ Stopped │
                     └─────────┘                         └─────────┘
                                                              │
                                                              ▼
                                                        ┌────────────┐
                                                        │ Terminated │
                                                        └────────────┘
```

**State Descriptions:**

| State | Description | Color |
|-------|-------------|-------|
| Requested | Campaign build has requested an endpoint | `--text-muted` (gray) |
| Provisioning | Cloud provider is creating the instance | `--status-warning` (amber) |
| Configuring | Instance is running startup scripts and configuring services | `--status-warning` (amber) |
| Active | Endpoint is live and serving traffic | `--status-success` (green) |
| Stopped | Endpoint has been manually stopped | `--text-muted` (gray) |
| Terminated | Endpoint has been permanently shut down | `--text-muted` (gray) |
| Error | Provisioning or configuration failed | `--status-error` (red) |

### 10.3 Endpoint Panel in Campaign Workspace

Within the campaign workspace (documented in 05-campaign-workspace.md), the "Infrastructure" tab shows endpoint status:

```
┌──────────────────────────────────────────────────────────────────┐
│  Campaign Infrastructure                                         │
│  ──────────────────────────────────────────────────────────────  │
│                                                                  │
│  Endpoints                                                       │
│  ┌──────────────┬──────────┬───────────┬──────────┬────────────┐ │
│  │ Hostname     │ IP       │ Template  │ Status   │ Actions    │ │
│  ├──────────────┼──────────┼───────────┼──────────┼────────────┤ │
│  │ lp-01.login- │ 93.184.  │ Standard  │ ● Active │ [···]      │ │
│  │ hr.com       │ 216.34   │ Landing   │          │            │ │
│  │ lp-02.login- │ 93.184.  │ Standard  │ ● Active │ [···]      │ │
│  │ hr.com       │ 216.35   │ Landing   │          │            │ │
│  └──────────────┴──────────┴───────────┴──────────┴────────────┘ │
│                                                                  │
│  ┌────────────────────────────────────────────────────────────┐  │
│  │  Endpoint Health Summary                                   │  │
│  │  Active: 2  │  Stopped: 0  │  Error: 0  │  Total: 2      │  │
│  └────────────────────────────────────────────────────────────┘  │
└──────────────────────────────────────────────────────────────────┘
```

### 10.4 Endpoint Kebab Menu Actions

| Action | Icon | Behavior |
|--------|------|----------|
| View Health | Activity | Opens endpoint health detail modal (section 10.5) |
| View Logs | Terminal | Opens endpoint log viewer modal (section 10.6) |
| Upload TLS Cert | Lock | Opens TLS certificate upload modal (section 10.7) |
| Stop | Square (amber) | Stops the endpoint (confirmation required). Only available when status is "Active". |
| Restart | Refresh | Restarts the endpoint (confirmation required). Only available when status is "Stopped" or "Active". |
| Terminate | Trash (red) | Terminates the endpoint (confirmation required). Not available when already "Terminated". |

**Stop Confirmation:**
- "Stop endpoint lp-01.login-hr.com? The landing page will become unreachable. You can restart it later."
- On confirm: `POST /api/v1/phishing-endpoints/{id}/stop`.

**Restart Confirmation:**
- "Restart endpoint lp-01.login-hr.com? This will briefly interrupt service."
- On confirm: `POST /api/v1/phishing-endpoints/{id}/restart`.

**Terminate Confirmation:**
- "Permanently terminate endpoint lp-01.login-hr.com? This action cannot be undone. The instance will be destroyed."
- On confirm: `POST /api/v1/phishing-endpoints/{id}/terminate`.

### 10.5 Endpoint Health Detail Modal

```
┌─────────────────────────────────────────────────────┐
│  Endpoint Health: lp-01.login-hr.com          [✕]   │
├─────────────────────────────────────────────────────┤
│                                                     │
│  Status: ● Active       Uptime: 4d 7h 23m          │
│  IP: 93.184.216.34      Region: us-east-1           │
│  Template: Standard Landing (v3)                    │
│                                                     │
│  ┌───────────────────┬────────┬───────────────────┐ │
│  │ Check             │ Status │ Detail            │ │
│  ├───────────────────┼────────┼───────────────────┤ │
│  │ HTTP Response     │ ● Pass │ 200 OK (89ms)     │ │
│  │ HTTPS/TLS         │ ● Pass │ Valid, 87 days    │ │
│  │ Landing Page      │ ● Pass │ Serving correctly │ │
│  │ Disk Usage        │ ● Pass │ 23% used          │ │
│  │ Memory            │ ▲ Warn │ 78% used          │ │
│  │ CPU               │ ● Pass │ 12% avg           │ │
│  └───────────────────┴────────┴───────────────────┘ │
│                                                     │
│  Last checked: 5 minutes ago                        │
│                                                     │
│                    [Run Check]  [Close]              │
└─────────────────────────────────────────────────────┘
```

- "Run Check" calls `GET /api/v1/phishing-endpoints/{id}/health`.
- Health data is fetched from `GET /api/v1/phishing-endpoint-reports/{id}/health`.

### 10.6 Endpoint Log Viewer Modal

```
┌─────────────────────────────────────────────────────────────┐
│  Logs: lp-01.login-hr.com                             [✕]  │
├─────────────────────────────────────────────────────────────┤
│  ┌──────────────────────────────┐                           │
│  │ Log Source ▾: Application    │  [Auto-refresh: ON]       │
│  └──────────────────────────────┘                           │
│  ┌─────────────────────────────────────────────────────┐    │
│  │ 2026-04-03 14:23:01 [INFO] Request: GET /           │    │
│  │ 2026-04-03 14:23:01 [INFO] Response: 200 (23ms)    │    │
│  │ 2026-04-03 14:23:15 [INFO] Request: POST /submit   │    │
│  │ 2026-04-03 14:23:15 [INFO] Credential captured     │    │
│  │ 2026-04-03 14:23:15 [INFO] Response: 302 → /done   │    │
│  │ 2026-04-03 14:24:02 [WARN] High memory usage: 78%  │    │
│  │                                                     │    │
│  │                                                     │    │
│  └─────────────────────────────────────────────────────┘    │
│                                                             │
│  Showing last 100 lines         [Load More]  [Download]     │
└─────────────────────────────────────────────────────────────┘
```

- Log source dropdown: Application, Nginx, System.
- Auto-refresh toggles automatic log polling (every 10 seconds when enabled).
- "Load More" fetches older log entries.
- "Download" exports the full log file.
- Log viewer uses monospace font, `--bg-primary` background (dark).
- API: `GET /api/v1/phishing-endpoint-reports/{id}/logs?source=application&lines=100`.

### 10.7 TLS Certificate Upload Modal

```
┌─────────────────────────────────────────────────────┐
│  Upload TLS Certificate                       [✕]   │
├─────────────────────────────────────────────────────┤
│                                                     │
│  Endpoint: lp-01.login-hr.com                       │
│                                                     │
│  Certificate (PEM) *                                │
│  ┌─────────────────────────────────────────────┐    │
│  │ Paste certificate or drag file here         │    │
│  │                                             │    │
│  └─────────────────────────────────────────────┘    │
│  [Browse File]                                      │
│                                                     │
│  Private Key (PEM) *                                │
│  ┌─────────────────────────────────────────────┐    │
│  │ Paste private key or drag file here         │    │
│  │                                             │    │
│  └─────────────────────────────────────────────┘    │
│  [Browse File]                                      │
│                                                     │
│  CA Bundle (optional)                               │
│  ┌─────────────────────────────────────────────┐    │
│  │ Paste CA bundle or drag file here           │    │
│  │                                             │    │
│  └─────────────────────────────────────────────┘    │
│  [Browse File]                                      │
│                                                     │
│                     [Cancel]  [Upload & Apply]      │
└─────────────────────────────────────────────────────┘
```

- Fields accept both pasted text and file drag-and-drop/browse.
- "Upload & Apply" calls `POST /api/v1/phishing-endpoints/{id}/tls-certificate`.
- Client-side validation: certificate and key must begin with `-----BEGIN` markers.
- On success: toast "TLS certificate applied. HTTPS is now active."
- On failure (key mismatch, invalid cert, etc.): error toast with specific error message.

### 10.8 Provisioning During Campaign Build

Endpoints are not created from the Infrastructure section. They are provisioned automatically during campaign build:

1. Campaign build selects a domain, instance template, and cloud credential.
2. The build process calls `POST /api/v1/phishing-endpoints` with the campaign context.
3. The endpoint enters "Requested" state and progresses through the lifecycle automatically.
4. The campaign workspace polls endpoint status and displays real-time progress.

The Infrastructure section's role is limited to managing the building blocks (credentials, templates) that campaigns reference during provisioning.

---

## 11. Typosquat Domain Tool

### 11.1 Purpose

The typosquat domain generation tool helps operators discover and optionally register domains that are typographical variations of a target organization's domain. This is used to create convincing phishing simulation domains.

### 11.2 Layout

```
┌──────────────────────────────────────────────────────────────────────────┐
│  Typosquat Domain Generator                                              │
├──────────────────────────────────────────────────────────────────────────┤
│                                                                          │
│  Target Domain *                                                         │
│  ┌────────────────────────────────────┐                                  │
│  │ example-company.com                │   [Generate]                     │
│  └────────────────────────────────────┘                                  │
│                                                                          │
│  Techniques:                                                             │
│  [✓] Character swap    [✓] Missing character    [✓] Double character    │
│  [✓] Homoglyph         [✓] TLD variation        [✓] Hyphenation        │
│  [✓] Subdomain         [ ] Bitsquatting                                 │
│                                                                          │
│  ┌──────────────────────────────────────────────────────────────────┐    │
│  │ Results: 47 variations generated              [Check All Avail.] │    │
│  ├──────────────────────────┬────────────┬──────────────┬───────────┤    │
│  │ Domain                  │ Technique  │ Available    │ Action    │    │
│  ├──────────────────────────┼────────────┼──────────────┼───────────┤    │
│  │ examp1e-company.com     │ Homoglyph  │ ● Available  │ [Register]│    │
│  │ example-compnay.com     │ Char Swap  │ ● Available  │ [Register]│    │
│  │ examplecompany.com      │ Hyphen     │ ✕ Taken      │ —         │    │
│  │ example-company.net     │ TLD        │ ● Available  │ [Register]│    │
│  │ example-compaany.com    │ Double     │ ● Available  │ [Register]│    │
│  │ exmple-company.com      │ Missing    │ — Unchecked  │ [Check]   │    │
│  │ ...                     │            │              │           │    │
│  └──────────────────────────┴────────────┴──────────────┴───────────┘    │
│                                                                          │
│  Showing 1–25 of 47                               [← 1  2  →]          │
└──────────────────────────────────────────────────────────────────────────┘
```

### 11.3 Generation

- "Generate" calls `POST /api/v1/tools/typosquat/generate` with the target domain and selected techniques.
- During generation: button shows spinner, "Generating..."
- Results are displayed in a paginated table (25 per page).
- Each result shows the generated domain, the technique used, and availability status.
- Availability is initially "Unchecked" for all results.

### 11.4 Availability Checking

- "[Check All Avail.]" calls `POST /api/v1/tools/typosquat/check-availability` with all generated domains. This may take time — a progress indicator shows "Checking 12/47..."
- Individual "[Check]" buttons check a single domain's availability.
- Results update in-place: green dot "Available", red X "Taken", gray dash "Unchecked", yellow spinner "Checking".
- Availability checks use the domain provider API selected in the target domain field (if a provider is associated) or a generic WHOIS lookup.

### 11.5 Registration

- "[Register]" button appears only for available domains.
- Clicking "Register" opens the domain registration workflow (section 3.9) pre-populated with the selected typosquat domain.
- After successful registration, the domain appears in the main domain list and the typosquat result updates to show "Registered" with a green check badge.

### 11.6 Export Results

- An "[Export CSV]" button (not shown in the layout for brevity) appears when results are present.
- Exports all generated domains with their technique and availability status.

---

## 12. Error States and Edge Cases

### 12.1 Empty States

Each list/grid has a specific empty state:

| Section | Empty State Message | Action |
|---------|---------------------|--------|
| Domains | "No domains configured yet. Add your first domain to get started." | [+ Add Domain] button |
| Domain Providers | "No domain providers connected. Connect a registrar to enable automated DNS management." | [+ Add Provider] button |
| SMTP Profiles | "No SMTP profiles configured. Create a profile to send phishing emails." | [+ New SMTP Profile] button |
| Cloud Credentials | "No cloud credentials configured. Add credentials to provision campaign infrastructure." | [+ Add Credential] button |
| Instance Templates | "No instance templates created. Templates define the compute resources for campaign endpoints." | [+ New Template] button |
| DNS Records | "No DNS records found for this domain." | [+ Add Record] button |
| Typosquat Results | "Enter a target domain and click Generate to find typosquat variations." | No button (input focused) |

Empty states use the standard illustration style (line art, `--text-muted` color) centered in the content area.

### 12.2 Loading States

- **Table loading**: Skeleton shimmer rows (5 rows) matching the column layout. No spinner.
- **Card grid loading**: Skeleton shimmer cards (2–4 cards) matching the card dimensions.
- **Slide-over loading**: Skeleton shimmer blocks for each field while data is fetched.
- **Modal loading**: Content area shows a centered spinner for data-dependent modals.

### 12.3 API Error Handling

| Error Code | Behavior |
|------------|----------|
| 400 Bad Request | Inline field errors are shown below the relevant form fields. Toast: "Please fix the errors below." |
| 401 Unauthorized | Redirect to login page. Session expired. |
| 403 Forbidden | Toast: "You do not have permission to perform this action." Disable the triggering control. |
| 404 Not Found | Toast: "The requested resource was not found." If in a slide-over, close it and refresh the list. |
| 409 Conflict | Context-specific message (e.g., "This resource is in use by an active campaign and cannot be deleted."). |
| 422 Unprocessable Entity | Inline field validation errors returned from the backend are displayed below the relevant fields. |
| 429 Too Many Requests | Toast: "Too many requests. Please wait a moment and try again." |
| 500 Internal Server Error | Toast: "An unexpected error occurred. Please try again." Include a "Report Issue" link if error tracking is enabled. |
| Network Error | Toast: "Unable to connect to the server. Check your network connection." Retry button included. |

### 12.4 Concurrent Edit Conflicts

- When a user opens an edit slide-over, the frontend records the resource's `updated_at` timestamp.
- On save, the frontend sends the `updated_at` value in the `If-Unmodified-Since` header (or `expected_version` field).
- If another user has modified the resource, the API returns 409 Conflict.
- The frontend shows a modal: "This {resource type} has been modified by another user. Would you like to reload the latest version (your changes will be lost) or overwrite their changes?"
- Options: [Reload] and [Overwrite]. Reload fetches fresh data. Overwrite re-submits with force flag.

### 12.5 Credential Security Edge Cases

- **Clipboard exposure**: Password/secret fields disable paste-to-clipboard for the masked values. The copy button is not available on masked credential displays.
- **Browser autofill**: Credential forms use `autocomplete="off"` on secret fields to prevent browser autofill from leaking credentials across sessions.
- **Session timeout during edit**: If a session expires while the user is editing credentials, the form data is NOT preserved. The user is redirected to login and must re-enter credentials after re-authentication.
- **Failed encryption**: If the backend fails to encrypt credentials (AES-256-GCM failure), the API returns 500. The frontend shows: "Unable to securely store credentials. Please try again."

### 12.6 Long-Running Operations

Several operations in the infrastructure section are long-running:

| Operation | Expected Duration | UX Pattern |
|-----------|-------------------|------------|
| Domain availability check | 2–5 seconds | Inline spinner on the Check button |
| DNS propagation check | 5–30 seconds | Progress indicator showing resolvers checked |
| Typosquat bulk availability | 10–60 seconds | Progress bar with "Checking X/Y..." |
| SMTP connection test | 5–15 seconds | Button spinner with "Testing..." |
| Cloud credential test | 3–10 seconds | Button spinner with "Testing..." |
| Endpoint provisioning | 2–10 minutes | State badge updates via polling (every 10s) |
| Domain registration | 10–60 seconds | Step 3 of wizard shows pending state |

- For operations exceeding 15 seconds, a "Taking longer than expected..." message appears below the spinner.
- For operations exceeding 60 seconds, a "Cancel" link appears to abort the request.
- Endpoint provisioning progress is polled via `GET /api/v1/phishing-endpoints/{id}` every 10 seconds until the state reaches "Active" or "Error".

### 12.7 Domain Provider Unavailability

- If a domain provider API is unreachable when attempting DNS operations, the error is surfaced with a specific message: "Unable to reach {provider name}. DNS changes cannot be applied at this time."
- A "Retry" button is provided.
- Previously fetched DNS records are still displayed from cache/database — the UI does not go blank on provider failure.
- The domain health check marks the DNS check as "Fail" if the provider is unreachable.

### 12.8 Endpoint Error Recovery

When an endpoint enters the "Error" state:

- The error message from the cloud provider is displayed in the endpoint health detail modal.
- Common errors and suggested fixes are shown:
  - "InsufficientInstanceCapacity": "The selected region does not have available capacity for this instance type. Try a different region or instance size."
  - "UnauthorizedAccess": "The cloud credential does not have sufficient permissions. Verify IAM/RBAC configuration."
  - "InvalidAMI": "The specified OS image is not available in this region. Update the instance template."
- The user may retry provisioning (which creates a new endpoint request) or terminate the failed endpoint.
- Failed endpoints are not automatically cleaned up — they remain visible until explicitly terminated.

### 12.9 Bulk DNS Record Import

- For domains with many records, the DNS sub-tab supports a "Paste Zone File" option (accessible via a "[Import]" link next to "[+ Add Record]").
- The import modal accepts BIND zone file format pasted as text.
- On paste, records are parsed and displayed in a preview table for review before import.
- Duplicate or conflicting records are highlighted with warnings.
- On confirm: records are created via batch `POST /api/v1/domains/{id}/dns/batch`.

### 12.10 Rate Limit Awareness

- SMTP profile "Max Send Rate" and "Max Connections" values are validated against the backend's global limits. If the user enters a value exceeding the system maximum, an inline warning appears: "Value exceeds system maximum of {limit}. It will be capped at {limit}."
- The backend enforces the cap regardless of frontend validation.
