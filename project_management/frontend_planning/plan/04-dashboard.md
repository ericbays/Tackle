# 04 — Dashboard

This document specifies the two dashboard views: the **Operator Dashboard** (default for Admin, Operator, Engineer roles) and the **Defender Dashboard** (separate navigation item, focused on organizational risk metrics).

---

## 1. Operator Dashboard

### 1.1 Purpose

The Operator Dashboard is the default landing page after login. It provides a high-level overview of system activity: active campaigns, credential captures, infrastructure health, pending approvals, and recent activity.

### 1.2 Layout

The dashboard uses a responsive grid layout:

```
┌──────────────────────────────────────────────────────────────┐
│  Dashboard                                       [7d ▾][↻]  │
├──────────────┬──────────────┬──────────────┬─────────────────┤
│  Active      │  Pending     │  Captures    │  Endpoint       │
│  Campaigns   │  Approvals   │  Today       │  Health         │
│     3        │     2        │    12 ↑23%   │  4●  1◐  0○     │
│              │              │              │  up  deg  down  │
├──────────────┴──────────────┴──────────────┴─────────────────┤
│                                                              │
│  ┌─────────────────────────────────┐ ┌──────────────────────┐│
│  │  Captures Over Time (line)     │ │  Capture Rate by     ││
│  │  ──────────────────────────    │ │  Campaign (bar)      ││
│  │                                │ │  ████                ││
│  │                                │ │  ██████              ││
│  │                                │ │  ████████            ││
│  └─────────────────────────────────┘ └──────────────────────┘│
│                                                              │
│  ┌────────────────────────────┐ ┌────────────────────────────┐│
│  │  Email Delivery Breakdown │ │  Recent Activity           ││
│  │  (donut chart)            │ │                            ││
│  │       ╭───╮               │ │  14:32 j.doe opened email ││
│  │      │     │              │ │  14:31 Campaign launched  ││
│  │       ╰───╯               │ │  14:28 Credential capture ││
│  │  Delivered Bounced Opened │ │  14:25 Endpoint provisioned│
│  └────────────────────────────┘ │  14:20 User created       ││
│                                  │  [View all logs →]       ││
│                                  └────────────────────────────┘│
└──────────────────────────────────────────────────────────────┘
```

### 1.3 Stat Cards (Top Row)

Four stat cards span the top of the dashboard in a 4-column grid (stack to 2x2 on tablet, 1-column on mobile).

**Active Campaigns:**
- Count of campaigns with `current_state = 'active'`.
- Click navigates to campaign list filtered to active campaigns.
- Source: `GET /api/v1/campaigns?state=active&per_page=1` (use `pagination.total` for count).

**Pending Approvals (Admin/Engineer only):**
- Count of campaigns with `current_state = 'pending_approval'`.
- Click navigates to campaign list filtered to pending approval.
- Hidden for roles without `campaigns:approve` permission.
- When hidden, the grid redistributes to 3 columns.

**Captures Today:**
- Count of credential captures in the last 24 hours.
- Trend indicator: percentage change compared to the previous 24-hour period (↑ green if higher, ↓ red if lower, → gray if unchanged).
- Source: Metrics API endpoint.

**Endpoint Health:**
- Three sub-counts: online (green dot), degraded (yellow dot), offline (red dot).
- Click navigates to the Infrastructure > Cloud & Endpoints page.
- Source: `GET /api/v1/endpoints` filtered by status.

### 1.4 Charts

**Captures Over Time (Line Chart):**
- X-axis: time (hourly granularity for 7d, daily for 30d).
- Y-axis: capture count.
- Line color: `--accent-primary`.
- Fill: gradient from `--accent-primary` at 20% opacity to transparent.
- Tooltip on hover shows exact count and timestamp.
- Date range selector in the dashboard header: 7 days (default), 30 days, 90 days, or custom date range.

**Capture Rate by Campaign (Horizontal Bar Chart):**
- Top 5 active campaigns by capture rate (captures / targets).
- Bar color: `--accent-primary`, with the highest rate bar slightly brighter.
- Each bar shows the campaign name (truncated to 25 chars) and the capture rate as a percentage.
- Click a bar to navigate to that campaign's workspace.

**Email Delivery Breakdown (Donut Chart):**
- Segments: Delivered (green), Bounced (red), Opened (blue), Clicked (teal).
- Center text: total emails sent across all active campaigns.
- Legend below the chart with count and percentage per segment.
- Aggregated across all active campaigns.

**Recent Activity (Feed):**
- The 20 most recent audit log entries relevant to the user, displayed as a compact feed.
- Each entry shows: relative timestamp, actor, action description.
- Entries are color-coded by category (campaign events = green dot, infrastructure = blue, system = gray, security = red).
- "View all logs →" link at the bottom navigates to the Audit Logs page.
- Updates in real-time via WebSocket — new entries prepend to the top.

### 1.5 Real-Time Updates

- All stat cards and charts receive real-time updates via WebSocket event subscriptions.
- When a capture event arrives, the "Captures Today" counter increments and the "Captures Over Time" chart's latest data point updates.
- When a campaign state changes, the "Active Campaigns" count adjusts.
- Updates are applied silently in place — no flash animations or toasts for dashboard data changes.
- The Recent Activity feed prepends new events as they arrive.

### 1.6 Loading State

Each dashboard widget independently shows a skeleton placeholder while its data loads. The layout is rendered immediately with skeleton shapes matching the final widget dimensions. No blank page or full-page spinner.

### 1.7 Empty States

- **No active campaigns**: The "Active Campaigns" card shows "0" with a link: "Create your first campaign →" (if user has `campaigns:create` permission).
- **No captures**: The capture chart shows an empty state illustration with "No credential captures yet."
- **No endpoints**: The endpoint health card shows "No endpoints provisioned."

### 1.8 Refresh

- A refresh icon button in the top-right of the dashboard header manually triggers a refetch of all dashboard data.
- All dashboard queries use a 30-second stale time for automatic background refetching.

---

## 2. Defender Dashboard

### 2.1 Purpose

The Defender Dashboard is a separate navigation item accessible to Defender, Engineer, and Administrator roles. It is focused on organizational risk assessment — aggregate metrics for security leadership presentations, with no per-target detail or operational controls.

### 2.2 Navigation

- Appears as "Security Overview" in the sidebar under the Insights group.
- Defender role users see this as their primary dashboard (their sidebar shows "Security Overview" instead of "Dashboard" at the top level).

### 2.3 Layout

```
┌──────────────────────────────────────────────────────────────┐
│  Security Overview                    [Date Range ▾] [↻]    │
│                                       [☐ Include Archived]   │
├──────────────────────────────────────────────────────────────┤
│                                                              │
│  ┌──────────────────────────────────────────────────────────┐│
│  │  Organizational Susceptibility Score                     ││
│  │                                                          ││
│  │              23.4%                                       ││
│  │  (% of targets who submitted credentials)                ││
│  │  ────────────────── trend line ──────────────────        ││
│  └──────────────────────────────────────────────────────────┘│
│                                                              │
│  ┌────────────────────────────┐ ┌────────────────────────────┐│
│  │  Department Risk Heatmap  │ │  Campaign Effectiveness    ││
│  │                           │ │  Comparison (bar chart)    ││
│  │  Engineering    ██ 12%    │ │                            ││
│  │  Marketing      ████ 34% │ │  Q1 Test  ████████         ││
│  │  Sales          ███ 28%  │ │  Q2 Test  ██████           ││
│  │  Executive      █ 5%     │ │  Q3 Test  ████████████     ││
│  │  HR             ██ 18%   │ │                            ││
│  └────────────────────────────┘ └────────────────────────────┘│
│                                                              │
│  ┌──────────────────────────────────────────────────────────┐│
│  │  Phishing Report Rate Trend                              ││
│  │  (% of targets who reported the phishing email)          ││
│  │  ──────────────────── trend line ────────────────────    ││
│  └──────────────────────────────────────────────────────────┘│
│                                                              │
└──────────────────────────────────────────────────────────────┘
```

### 2.4 Widgets

**Organizational Susceptibility Score:**
- Large percentage number showing the overall credential submission rate across all campaigns (or selected date range).
- Trend line below showing how this score has changed across campaigns over time.
- Color-coded: green if decreasing (improving), red if increasing (worsening).

**Department/Group Risk Heatmap:**
- Horizontal bars for each department, colored by capture rate intensity (low = green, medium = yellow, high = red).
- Departments sorted by risk (highest capture rate at top).
- Source: aggregate metrics grouped by target department across campaigns.

**Campaign Effectiveness Comparison:**
- Horizontal bar chart comparing key metrics across campaigns.
- Metrics shown: open rate, click rate, capture rate, report rate.
- Campaigns sorted by capture rate (highest first).
- Click a campaign to see its metrics detail (navigates to Metrics page filtered to that campaign).

**Phishing Report Rate Trend:**
- Line chart showing the percentage of targets who reported the phishing email across campaigns over time.
- An increasing trend (more people reporting) is positive and shown in green.
- A decreasing trend is negative and shown in red.

### 2.5 Data Scope

- All data is **aggregate only** — no individual target names, email addresses, or credential values are displayed anywhere on this dashboard.
- Date range selector filters campaigns by their start date.
- "Include Archived" toggle includes completed and archived campaigns in the calculations.

### 2.6 Presentation Mode

The Defender Dashboard layout is optimized for presenting to security leadership:
- Clean, high-level visualizations.
- Large readable numbers and clear trend indicators.
- No operational controls or action buttons.
- Charts use the same dark theme but with higher contrast for projector readability.
