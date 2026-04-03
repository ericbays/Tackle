# 10 — Metrics and Reporting

This document specifies the **Metrics Dashboard** and **Report Generation** interfaces. The Metrics Dashboard provides real-time, filterable visualizations of campaign performance, email delivery, target engagement, and comparative analytics. The Report Generation system allows operators to produce downloadable reports in multiple formats from configurable templates. Together these views transform raw phishing simulation data into actionable intelligence.

---

## 1. Metrics Dashboard

### 1.1 Purpose

The Metrics Dashboard is the central analytics interface. It presents campaign and engagement data through interactive chart widgets arranged in a filterable, scrollable layout. Data refreshes automatically via WebSocket events for live campaigns and via polling (30-second interval) for historical views. Role-based access controls determine data granularity: Operators see per-target detail; Defenders see only aggregates.

### 1.2 Page Layout

```
┌──────────────────────────────────────────────────────────────────────┐
│  Metrics                                          [Generate Report] │
├──────────────────────────────────────────────────────────────────────┤
│  Filters                                                            │
│  [Campaign ▾] [Date Range ▾] [Department ▾] [Template ▾] [↻ Live]  │
├─────────────────┬────────────┬────────────────┬──────────────────────┤
│  Emails Sent    │  Delivered │  Opened        │  Credentials         │
│     1,248       │   1,190    │    412  (34.6%) │  Captured  87 (7.3%)│
│   ↑12% vs prev │  95.4% del │   ↑8% vs prev  │  ↑2.1% vs prev     │
├─────────────────┴────────────┴────────────────┴──────────────────────┤
│                                                                      │
│  ┌──────────────────────────────────────────────────────────────────┐│
│  │  Email Funnel                                    [PNG][SVG][⋯]  ││
│  │  Sent ████████████████████████████████████████████  1,248       ││
│  │  Delivered ██████████████████████████████████████░░  1,190      ││
│  │  Opened ████████████████░░░░░░░░░░░░░░░░░░░░░░░░░    412      ││
│  │  Clicked ████████░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░    198      ││
│  │  Captured ████░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░     87      ││
│  └──────────────────────────────────────────────────────────────────┘│
│                                                                      │
│  ┌───────────────────────────────┐ ┌────────────────────────────────┐│
│  │  Campaign Comparison (bar)   │ │  Time to Capture (histogram)  ││
│  │  ████                        │ │       ▄                       ││
│  │  ██████                      │ │     ▄ █ ▄                     ││
│  │  ████████                    │ │   ▄ █ █ █ ▄                   ││
│  │  ██████████                  │ │  ▄█ █ █ █ █▄                  ││
│  │                     [⋯]     │ │                        [⋯]    ││
│  └───────────────────────────────┘ └────────────────────────────────┘│
│                                                                      │
│  ┌───────────────────────────────┐ ┌────────────────────────────────┐│
│  │  Temporal Analysis           │ │  Template Performance          ││
│  │  (heatmap: hour x day)      │ │  (grouped bar)                ││
│  │  Mon ░░▒▒▓▓██▓▓▒▒░░░░░░░░  │ │  ████  ████                  ││
│  │  Tue ░░▒▒▓▓████▓▓▒▒░░░░░░  │ │  ██████  ██████              ││
│  │  Wed ░░░▒▒▓▓██▓▓▒▒░░░░░░░  │ │  ████████  ████████          ││
│  │                     [⋯]     │ │                        [⋯]    ││
│  └───────────────────────────────┘ └────────────────────────────────┘│
│                                                                      │
│  ┌──────────────────────────────────────────────────────────────────┐│
│  │  Department Breakdown (horizontal bar)                    [⋯]   ││
│  │  Engineering  ████████████████████  42%                         ││
│  │  Sales        ████████████░░░░░░░░  28%                         ││
│  │  HR           ████████░░░░░░░░░░░░  18%                         ││
│  │  Finance      █████░░░░░░░░░░░░░░░  12%                         ││
│  └──────────────────────────────────────────────────────────────────┘│
└──────────────────────────────────────────────────────────────────────┘
```

### 1.3 Filter Bar

The filter bar spans the full width below the page header and controls which data is reflected across all dashboard widgets simultaneously.

**Campaign Filter (Multi-Select Dropdown):**
- Lists all campaigns the user has permission to view.
- Options show campaign name plus status badge.
- Default: all active campaigns selected.
- When a single campaign is selected, the dashboard enters "single campaign mode" which unlocks additional per-campaign widgets (A/B comparison, per-target tables).
- Sends `campaign_ids[]` parameter to all metric endpoints.

**Date Range Filter (Preset Dropdown + Custom Range):**
- Preset options: Last 24 hours, Last 7 days, Last 30 days, Last 90 days, Year to date, All time.
- "Custom range" opens a date range picker with From/To date inputs.
- Default: Last 30 days.
- Sends `start_date` and `end_date` parameters.

**Department Filter (Multi-Select Dropdown):**
- Populated from `GET /api/v1/metrics/targets` department breakdown.
- Default: all departments.
- Sends `departments[]` parameter.

**Template Filter (Multi-Select Dropdown):**
- Lists email templates used in selected campaigns.
- Default: all templates.
- Sends `template_ids[]` parameter.

**Live Toggle:**
- Toggle button labeled "Live" with a pulsing indicator dot when active.
- When enabled: subscribes to WebSocket channel for real-time metric updates. The indicator dot pulses in `--success` color (`#38a169`).
- When disabled: data refreshes only on filter change or manual refresh.
- Default: enabled when any selected campaign is active, disabled otherwise.
- Live mode disables the date range filter end date (locks to "now").

**Filter Behavior:**
- Changing any filter immediately triggers data reload for all widgets.
- Active filters are serialized to URL query parameters for shareability.
- A "Clear filters" link appears when any non-default filter is active.
- Each widget shows a brief skeleton/shimmer during data reload (see section 13).

### 1.4 Stat Cards (Top Row)

Four stat cards span the top of the dashboard in a 4-column grid (2x2 on tablet, single column on mobile).

| Card | Value | Subtext | Color |
|------|-------|---------|-------|
| Emails Sent | Total count | % change vs. previous period | `--accent-primary` |
| Delivered | Total count | Delivery rate percentage | `--success` |
| Opened | Total count with open rate | % change vs. previous period | `--warning` |
| Credentials Captured | Total count with capture rate | % change vs. previous period | `--danger` |

- "Previous period" is the equivalent duration immediately before the selected date range. For example, "Last 7 days" compares to the 7 days before that.
- The % change arrow is `--success` for increases in capture rate (from operator perspective), `--text-muted` for no change.
- Clicking a stat card scrolls to the most relevant chart (e.g., clicking "Opened" scrolls to the email funnel).
- Source: `GET /api/v1/metrics/email-delivery` with active filters.

---

## 2. Email Funnel Visualization

### 2.1 Purpose

The email funnel is the primary engagement visualization. It shows the progressive narrowing from sent emails through to credential capture, giving operators immediate insight into where targets drop off.

### 2.2 Chart Specification

**Chart Type:** Horizontal funnel bar chart (Recharts `BarChart` or Nivo `Bar`).

**Stages (top to bottom):**

| Stage | Data Field | Bar Color | Label |
|-------|-----------|-----------|-------|
| Sent | `emails_sent` | `#4a7ab5` (accent) | "{count} sent" |
| Delivered | `emails_delivered` | `#5a8ac5` | "{count} ({pct}% of sent)" |
| Opened | `emails_opened` | `#d69e2e` (warning) | "{count} ({pct}% of delivered)" |
| Clicked | `links_clicked` | `#e8853a` | "{count} ({pct}% of opened)" |
| Captured | `credentials_captured` | `#e53e3e` (danger) | "{count} ({pct}% of clicked)" |

- Bars are rendered left-aligned with width proportional to count relative to "Sent".
- Each bar shows count and conversion percentage as inline labels to the right of the bar.
- Between each pair of bars, a small annotation shows the drop-off: "↓ {lost_count} lost ({drop_pct}%)".
- Source: `GET /api/v1/metrics/email-delivery` filtered by active selections.

### 2.3 Funnel Interactions

**Hover:** Tooltip shows:
```
Opened: 412
Rate: 34.6% of delivered
Drop from previous: 778 (65.4%)
```

**Click on a funnel stage:** When in single-campaign mode, clicking a stage opens a slide-over panel listing targets at that stage.
- Operator role: shows target name, email, department, timestamp of stage entry.
- Defender role: click is disabled (aggregates only).

**Bounce Breakdown:** Below the funnel, a small inline breakdown shows bounce detail when bounced count > 0:
```
Bounced: 58 (4.6% of sent) — Hard: 12  Soft: 46
```

### 2.4 Role-Based Display

| Element | Operator/Admin | Defender |
|---------|---------------|----------|
| Funnel bars and counts | Visible | Visible |
| Click-through to target list | Enabled | Disabled (cursor: default) |
| Per-target hover details | Available | Not available |
| Department split on hover | Available | Shows only counts, no names |

---

## 3. Campaign Comparison Charts

### 3.1 Purpose

Compare performance across multiple campaigns to identify trends, measure improvement over time, and evaluate which campaign configurations produce the best results.

### 3.2 Layout

```
┌──────────────────────────────────────────────────────────────────────┐
│  Campaign Comparison                          [Metric ▾] [PNG][SVG] │
│                                                                      │
│  ┌─ Grouped Bar Chart ──────────────────────────────────────────────┐│
│  │                                                                  ││
│  │        ┌──┐                                                      ││
│  │   ┌──┐ │  │ ┌──┐         ┌──┐                                   ││
│  │   │  │ │  │ │  │    ┌──┐ │  │ ┌──┐    ┌──┐ ┌──┐ ┌──┐           ││
│  │   │  │ │  │ │  │    │  │ │  │ │  │    │  │ │  │ │  │           ││
│  │   │  │ │  │ │  │    │  │ │  │ │  │    │  │ │  │ │  │           ││
│  │   └──┘ └──┘ └──┘    └──┘ └──┘ └──┘    └──┘ └──┘ └──┘           ││
│  │   Q1 Exec Spear     HR Benefits       IT Dept Recon             ││
│  │                                                                  ││
│  │   ■ Open Rate  ■ Click Rate  ■ Capture Rate                     ││
│  └──────────────────────────────────────────────────────────────────┘│
│                                                                      │
│  ┌─ Comparison Table ───────────────────────────────────────────────┐│
│  │ Campaign        │ Sent │ Delivered │ Opened │ Clicked │ Captured ││
│  │ Q1 Exec Spear   │  142 │  138 97%  │ 62 45% │  28 20% │  12  9% ││
│  │ HR Benefits      │   89 │   86 97%  │ 41 48% │  19 22% │   8  9% ││
│  │ IT Dept Recon    │  230 │  221 96%  │ 78 35% │  31 14% │   9  4% ││
│  └──────────────────────────────────────────────────────────────────┘│
└──────────────────────────────────────────────────────────────────────┘
```

### 3.3 Chart Specification

**Chart Type:** Grouped vertical bar chart (Recharts `BarChart` with multiple `Bar` components).

**Metric Selector Dropdown:**
- Options: Open Rate, Click Rate, Capture Rate, Delivery Rate, All Rates (grouped).
- Default: All Rates (shows open, click, and capture as grouped bars per campaign).
- Switching metric re-renders the chart with transition animation (300ms ease).

**Bar Colors:**
- Open Rate: `#d69e2e` (warning)
- Click Rate: `#e8853a`
- Capture Rate: `#e53e3e` (danger)
- Delivery Rate: `#38a169` (success)

**Axis Configuration:**
- X-axis: campaign names (truncated at 20 chars with tooltip for full name).
- Y-axis: percentage (0-100%), with gridlines at 25% intervals.
- Y-axis gridlines: `#1e2a3a` (`--border-default`), 1px dashed.

**Hover Tooltip:** Shows campaign name, metric name, percentage value, and raw count (e.g., "Q1 Exec Spear — Open Rate: 45% (62/138)").

### 3.4 Comparison Table

Rendered below the chart as a data table. All percentage cells are color-coded:
- Top-performing value in each column: bold text in `--success`.
- Bottom-performing value: text in `--danger`.
- Other values: `--text-primary`.

Clicking a campaign name in the table navigates to that campaign's workspace Results tab.

Source: `GET /api/v1/metrics/campaigns` with `campaign_ids[]` from filter.

---

## 4. Time-to-Capture Histogram

### 4.1 Purpose

Visualize the distribution of time elapsed between email delivery and credential capture. This reveals how quickly targets fall for the simulation and helps operators understand urgency patterns.

### 4.2 Chart Specification

**Chart Type:** Vertical bar histogram (Recharts `BarChart`).

**Bins:**
- < 1 min, 1-5 min, 5-15 min, 15-30 min, 30-60 min, 1-2 hr, 2-4 hr, 4-8 hr, 8-24 hr, 1-3 days, 3-7 days, > 7 days.
- Bar height represents count of credential captures in that time bucket.

**Bar Color:** Gradient from `#4a7ab5` (fast captures) to `#e53e3e` (slow captures), applied per-bin.

**Annotations:**
- A vertical dashed line marks the median time-to-capture, labeled "Median: {value}".
- A second dashed line (different dash pattern) marks the mean, labeled "Mean: {value}".
- Annotation lines use `--text-secondary` color.

**Axis Configuration:**
- X-axis: time bucket labels.
- Y-axis: count of captures with auto-scaled max.

**Hover Tooltip:**
```
5-15 minutes
Captures: 23
Percentage: 26.4% of all captures
Cumulative: 58.6%
```

### 4.3 Interactions

- Click on a histogram bar (Operator only): opens a slide-over panel listing the targets who were captured in that time window, with columns: Target Name, Email, Department, Exact Time-to-Capture, Campaign.
- Defender role: click disabled, tooltip only.

Source: `GET /api/v1/metrics/targets` with `metric=time_to_capture` and active filters.

---

## 5. Template and Landing Page Performance Comparison

### 5.1 Purpose

Compare the effectiveness of different email templates and landing pages to identify which content and designs produce the highest engagement and capture rates.

### 5.2 Layout

```
┌──────────────────────────────────────────────────────────────────────┐
│  Template Performance                    [Email|Landing] [PNG][SVG] │
├──────────────────────────────────────────────────────────────────────┤
│                                                                      │
│  ┌─ Horizontal Bar Chart ───────────────────────────────────────────┐│
│  │                                                                  ││
│  │  Password Reset  ██████████████████████████████  42%  (62/148)   ││
│  │  IT Alert        █████████████████████████░░░░░  35%  (41/117)   ││
│  │  Benefits Update ██████████████████░░░░░░░░░░░░  28%  (28/100)   ││
│  │  Shipping Notice ██████████████░░░░░░░░░░░░░░░░  22%  (19/86)    ││
│  │                                                                  ││
│  │  Showing: Open Rate ▾                                            ││
│  └──────────────────────────────────────────────────────────────────┘│
│                                                                      │
│  ┌─ Detail Table ───────────────────────────────────────────────────┐│
│  │ Template         │ Times Used │ Open Rate │ Click Rate │ Capture ││
│  │ Password Reset   │     4      │  42%      │   21%      │  12%   ││
│  │ IT Alert         │     3      │  35%      │   18%      │   9%   ││
│  │ Benefits Update  │     2      │  28%      │   14%      │   7%   ││
│  │ Shipping Notice  │     2      │  22%      │   11%      │   5%   ││
│  └──────────────────────────────────────────────────────────────────┘│
└──────────────────────────────────────────────────────────────────────┘
```

### 5.3 Segment Toggle

A segmented control at the top right switches between:
- **Email Templates** — compares email template performance.
- **Landing Pages** — compares landing page performance (page views, time on page, form interactions, credential submissions).

Switching the segment reloads chart data with a crossfade animation (200ms).

### 5.4 Email Template Chart

**Chart Type:** Horizontal bar chart, sorted by the selected metric descending.

**Metric Dropdown:** Open Rate (default), Click Rate, Capture Rate, Delivery Rate.

**Bar Color:** `--accent-primary` (`#4a7ab5`) with opacity varying by rank (top bar 100%, diminishing by 10% per rank, minimum 40%).

**Label Format:** "{template_name} {bar} {pct}% ({count}/{total})"

**Hover Tooltip:**
```
Password Reset
Used in: 4 campaigns
Sent: 148 | Delivered: 145 (98%)
Opened: 62 (42%) | Clicked: 31 (21%) | Captured: 18 (12%)
```

### 5.5 Landing Page Chart

**Chart Type:** Same horizontal bar layout.

**Metrics Available:** Page Views, Avg. Time on Page, Form Interaction Rate, Credential Submission Rate.

**Hover Tooltip:**
```
Office 365 Login
Used in: 3 campaigns
Page Views: 89
Avg. Time on Page: 47s
Form Interactions: 62 (69.7%)
Credential Submissions: 31 (34.8%)
```

### 5.6 Detail Table

Below both chart types, a sortable table provides the full numeric breakdown. Clicking a column header sorts by that metric. The top-performing row in each column is highlighted with bold `--success` text.

Clicking a template or landing page name navigates to the template editor or landing page builder respectively.

Source: `GET /api/v1/metrics/campaigns` with `group_by=template` or `group_by=landing_page`.

---

## 6. A/B Variant Comparison View

### 6.1 Purpose

When a campaign uses A/B testing (multiple email variants or landing page variants), this widget provides a direct statistical comparison of variant performance. Available only in single-campaign mode.

### 6.2 Layout

```
┌──────────────────────────────────────────────────────────────────────┐
│  A/B Variant Comparison — Q1 Exec Spear            [PNG][SVG][⋯]  │
├──────────────────────────────────────────────────────────────────────┤
│                                                                      │
│  ┌─ Variant A ──────────────────┐  ┌─ Variant B ──────────────────┐ │
│  │  "Urgent: Password Expiry"  │  │  "IT: Account Verification" │ │
│  │                              │  │                              │ │
│  │  Sent:      74               │  │  Sent:      74               │ │
│  │  Opened:    38  (51.4%)      │  │  Opened:    24  (32.4%)      │ │
│  │  Clicked:   19  (25.7%)      │  │  Clicked:    9  (12.2%)      │ │
│  │  Captured:   9  (12.2%)      │  │  Captured:   3   (4.1%)      │ │
│  └──────────────────────────────┘  └──────────────────────────────┘ │
│                                                                      │
│  ┌─ Side-by-Side Bar ──────────────────────────────────────────────┐│
│  │              Variant A          Variant B                       ││
│  │  Open Rate   █████████████████  ███████████                     ││
│  │  Click Rate  ██████████         █████                           ││
│  │  Capture     ██████             ██                               ││
│  └──────────────────────────────────────────────────────────────────┘│
│                                                                      │
│  ┌─ Statistical Significance ───────────────────────────────────────┐│
│  │  Open Rate:    Variant A leads by +19.0pp  │  p=0.021  ● Sig.  ││
│  │  Click Rate:   Variant A leads by +13.5pp  │  p=0.034  ● Sig.  ││
│  │  Capture Rate: Variant A leads by  +8.1pp  │  p=0.082  ○ N.S.  ││
│  │                                                                  ││
│  │  ● Statistically significant (p < 0.05)                         ││
│  │  ○ Not statistically significant                                 ││
│  └──────────────────────────────────────────────────────────────────┘│
└──────────────────────────────────────────────────────────────────────┘
```

### 6.3 Variant Cards

Two cards side by side, each showing:
- Variant label (e.g., "Variant A") and the email subject line or landing page name.
- Key metrics: Sent, Opened (with rate), Clicked (with rate), Captured (with rate).
- The winning variant's card has a subtle left border in `--success` color (3px) on the metric where it leads.

If more than two variants exist, the layout switches to a scrollable horizontal card list.

### 6.4 Side-by-Side Bar Chart

**Chart Type:** Butterfly/mirror bar chart or paired horizontal bars (Recharts `BarChart`).

**Bar Colors:**
- Variant A: `#4a7ab5` (accent primary)
- Variant B: `#7c5cbf` (purple, a secondary chart color)
- Additional variants: `#38a169`, `#d69e2e`, `#e53e3e` (cycling through semantic palette).

### 6.5 Statistical Significance Panel

Displays the result of a two-proportion z-test for each metric:
- Shows the leading variant, the difference in percentage points (pp), and the p-value.
- Significance indicator: filled circle (`--success`) if p < 0.05, hollow circle (`--text-muted`) if p >= 0.05.
- If sample size is too small for a valid test (< 30 per variant), the panel shows: "Insufficient sample size for significance testing. Minimum 30 targets per variant required."

### 6.6 Visibility

- This widget only appears when a single campaign is selected in the filter and that campaign has A/B variants configured.
- If no A/B variants exist, this section is not rendered (no empty state).

Source: `GET /api/v1/metrics/campaigns/{id}` with variant breakdown.

---

## 7. Temporal Analysis

### 7.1 Purpose

Visualize engagement patterns by time of day and day of week to help operators optimize send times for future campaigns.

### 7.2 Layout Options

The temporal analysis widget supports two visualization modes toggled by a segmented control:

**Heatmap Mode (Default):**
```
┌──────────────────────────────────────────────────────────────────────┐
│  Temporal Analysis                    [Heatmap|Line] [Metric ▾] [⋯]│
├──────────────────────────────────────────────────────────────────────┤
│                                                                      │
│  Engagement by Hour and Day of Week                                  │
│                                                                      │
│       00 01 02 03 04 05 06 07 08 09 10 11 12 13 14 15 16 17 ...    │
│  Mon  ░░ ░░ ░░ ░░ ░░ ░░ ▒▒ ▓▓ ██ ██ ▓▓ ▓▓ ▒▒ ▓▓ ██ ▓▓ ▒▒ ░░    │
│  Tue  ░░ ░░ ░░ ░░ ░░ ░░ ▒▒ ▓▓ ██ ▓▓ ▓▓ ▒▒ ▒▒ ▓▓ ▓▓ ▓▓ ▒▒ ░░    │
│  Wed  ░░ ░░ ░░ ░░ ░░ ░░ ▒▒ ▓▓ ▓▓ ██ ██ ▓▓ ▒▒ ▓▓ ██ ██ ▒▒ ░░    │
│  Thu  ░░ ░░ ░░ ░░ ░░ ░░ ▒▒ ▓▓ ██ ██ ▓▓ ▓▓ ▒▒ ▓▓ ▓▓ ▓▓ ▒▒ ░░    │
│  Fri  ░░ ░░ ░░ ░░ ░░ ░░ ▒▒ ▒▒ ▓▓ ▓▓ ▒▒ ▒▒ ▒▒ ▒▒ ▒▒ ▒▒ ░░ ░░    │
│  Sat  ░░ ░░ ░░ ░░ ░░ ░░ ░░ ░░ ░░ ░░ ░░ ░░ ░░ ░░ ░░ ░░ ░░ ░░    │
│  Sun  ░░ ░░ ░░ ░░ ░░ ░░ ░░ ░░ ░░ ░░ ░░ ░░ ░░ ░░ ░░ ░░ ░░ ░░    │
│                                                                      │
│  ░ Low  ▒ Medium  ▓ High  █ Peak                                    │
└──────────────────────────────────────────────────────────────────────┘
```

### 7.3 Heatmap Specification

**Chart Type:** Heatmap grid (Nivo `HeatMap` or custom Recharts implementation).

**Axes:**
- Y-axis: days of week (Mon-Sun).
- X-axis: hours of day (00-23), displayed in user's local timezone.

**Color Scale:** Sequential scale from `#141b2d` (surface, no activity) through `#1e3a5a` (low) to `#4a7ab5` (medium) to `#e53e3e` (peak). Five discrete steps.

**Metric Selector:** Opens, Clicks, Captures (default: Opens).

**Hover Tooltip:**
```
Wednesday, 09:00-10:00
Opens: 28
% of total opens: 6.8%
Rank: 3rd highest hour
```

**Click:** Clicking a cell filters the funnel and other charts to that specific day+hour window (adds a temporary time-of-day filter). A dismissible chip appears in the filter bar: "Filtered: Wed 09:00-10:00 [x]".

### 7.4 Line Chart Mode

**Chart Type:** Multi-line chart (Recharts `LineChart`).

Displays one line per day of week, with the x-axis as hour of day (00-23) and y-axis as the selected metric count. A legend identifies each day by color. This mode is useful for comparing daily patterns.

**Line Colors:** Seven distinct colors from the chart palette, one per weekday.

Source: `GET /api/v1/metrics/email-delivery` with `group_by=hour_of_day,day_of_week`.

---

## 8. Report Template Management

### 8.1 Purpose

Report templates define reusable configurations for report generation. Operators create templates that specify which sections, charts, and data to include, then use them to generate reports across different campaigns and time periods.

### 8.2 Report Templates List

Accessed via the "Reports" navigation item in the sidebar, which shows a list/management view.

```
┌──────────────────────────────────────────────────────────────────────┐
│  Report Templates                               [+ New Template]    │
├──────────────────────────────────────────────────────────────────────┤
│  [Search templates...                        ]                       │
├──┬──────────────────────┬──────────┬──────────────┬──────────┬──────┤
│  │ Name                 │ Type     │ Last Used    │ Created  │  ··· │
├──┼──────────────────────┼──────────┼──────────────┼──────────┼──────┤
│  │ Executive Summary    │ Executive│ Apr 01, 2026 │ Jan 15   │  ··· │
│  │ Campaign Deep Dive   │ Campaign │ Mar 28, 2026 │ Feb 02   │  ··· │
│  │ Quarterly Comparison │ Compare  │ Mar 31, 2026 │ Mar 01   │  ··· │
│  │ Compliance Report    │ Complianc│ Mar 15, 2026 │ Jan 20   │  ··· │
│  │ Custom Analytics     │ Custom   │ —            │ Mar 25   │  ··· │
├──┴──────────────────────┴──────────┴──────────────┴──────────┴──────┤
│  Showing 1–5 of 5                                                    │
└──────────────────────────────────────────────────────────────────────┘
```

### 8.3 Table Columns

| Column | Width | Content | Sortable |
|--------|-------|---------|----------|
| Name | flex | Template name, clickable to open editor | Yes |
| Type | 100px | Report type badge: Campaign, Comparison, Executive, Compliance, Custom | Yes |
| Last Used | 120px | Date template was last used to generate a report, or "—" | Yes |
| Created | 100px | Creation date | Yes |
| Actions | 48px | Kebab menu | No |

**Kebab Menu Actions:**
- Edit — opens the template editor.
- Duplicate — creates a copy named "{original name} (Copy)" via `POST /api/v1/reports/templates` with duplicated fields.
- Generate Report — opens the report generation dialog pre-populated with this template.
- Delete — confirmation dialog: "Delete template '{name}'? This cannot be undone." Calls `DELETE /api/v1/reports/templates/{id}`.

Source: `GET /api/v1/reports/templates`.

### 8.4 Template Editor

Clicking "New Template" or editing an existing template opens a full-page editor.

```
┌──────────────────────────────────────────────────────────────────────┐
│  ← Back to Templates          Edit Report Template       [Save]     │
├──────────────────────────────────────────────────────────────────────┤
│                                                                      │
│  Template Name    [Executive Summary Report              ]           │
│  Report Type      [Executive ▾]                                      │
│  Description      [High-level overview for leadership... ]           │
│                                                                      │
│  ── Sections ────────────────────────────────────────────────────── │
│                                                                      │
│  ☰ ☑ Executive Summary        Narrative overview of results         │
│  ☰ ☑ Email Funnel             Sent → Delivered → Opened → Captured  │
│  ☰ ☑ Campaign Comparison      Side-by-side campaign metrics         │
│  ☰ ☐ Target Detail            Per-target engagement breakdown       │
│  ☰ ☑ Department Breakdown     Results grouped by department         │
│  ☰ ☑ Temporal Analysis        Engagement by time of day/week        │
│  ☰ ☐ Template Performance     Email template comparison             │
│  ☰ ☐ Landing Page Performance Landing page metrics                  │
│  ☰ ☑ A/B Variant Results      Variant comparison (if applicable)    │
│  ☰ ☐ Raw Data Tables          Full event-level data export          │
│  ☰ ☑ Recommendations          Auto-generated improvement notes      │
│                                                                      │
│  ── Default Settings ────────────────────────────────────────────── │
│                                                                      │
│  Default Format   [PDF ▾]                                            │
│  Include Charts   [☑ Yes]                                            │
│  Branding         [☑ Include company logo and header]                │
│                                                                      │
└──────────────────────────────────────────────────────────────────────┘
```

### 8.5 Template Editor Fields

**Template Name:** Required, max 100 characters. Validated on blur.

**Report Type (Dropdown):**
- Campaign — single campaign deep dive.
- Comparison — multi-campaign side-by-side.
- Executive — high-level summary with minimal technical detail.
- Compliance — structured output aligned to compliance frameworks.
- Custom — freeform section selection.

Selecting a type pre-selects a default set of sections. Users can override the defaults.

**Description:** Optional, max 500 characters. Shown in the template list as a tooltip.

**Sections (Reorderable Checklist):**
- Each section has a drag handle (☰), a checkbox (include/exclude), a name, and a brief description.
- Drag and drop to reorder sections. Order determines the section order in the generated report.
- At least one section must be checked. If user unchecks the last section, show inline validation: "At least one section is required."

**Default Format:** PDF (default), CSV, JSON, HTML.

**Include Charts:** Checkbox. When unchecked, reports contain only tabular data (relevant for CSV/JSON).

**Branding:** Checkbox. When checked, report includes organization logo and header styling.

### 8.6 Save Behavior

- "Save" button creates (`POST /api/v1/reports/templates`) or updates (`PUT /api/v1/reports/templates/{id}`).
- On success: toast "Template saved" and navigate back to template list.
- On validation error (e.g., duplicate name): inline error under the offending field.
- Unsaved changes trigger the standard navigation guard: "You have unsaved changes. Leave anyway?"

---

## 9. Report Generation Workflow

### 9.1 Purpose

Report generation transforms filtered metric data into downloadable documents. The workflow guides the user through campaign selection, section configuration, and format choice, then initiates async generation.

### 9.2 Entry Points

Reports can be generated from three locations:
1. **Metrics Dashboard** — "Generate Report" button in the page header. Pre-populates with current filter selections.
2. **Report Templates List** — "Generate Report" action in kebab menu. Pre-populates with the template's default sections and format.
3. **Campaign Workspace Results Tab** — "Export Report" button. Pre-populates with that single campaign.

### 9.3 Report Generation Dialog

A multi-step modal dialog (3 steps):

**Step 1 — Select Data:**
```
┌──────────────────────────────────────────────────────────────────────┐
│  Generate Report                                            [✕]     │
│  ── Step 1 of 3: Select Data ────────────────────────────────────── │
│                                                                      │
│  Template       [Executive Summary ▾]    or [Start from scratch]    │
│                                                                      │
│  Campaigns      ┌──────────────────────────────────────────────┐    │
│                 │ ☑ Q1 Exec Spear (Active)                    │    │
│                 │ ☑ HR Benefits (Completed)                    │    │
│                 │ ☐ IT Dept Recon (Completed)                  │    │
│                 │ ☐ Sales Q4 (Completed)                       │    │
│                 └──────────────────────────────────────────────┘    │
│                                                                      │
│  Date Range     [Last 30 days ▾]  or  [Custom: _____ to _____]     │
│  Departments    [All ▾]                                              │
│                                                                      │
│                                              [Cancel]  [Next →]     │
└──────────────────────────────────────────────────────────────────────┘
```

**Step 2 — Configure Sections:**
```
┌──────────────────────────────────────────────────────────────────────┐
│  Generate Report                                            [✕]     │
│  ── Step 2 of 3: Configure Sections ─────────────────────────────── │
│                                                                      │
│  ☰ ☑ Executive Summary                                              │
│  ☰ ☑ Email Funnel                                                   │
│  ☰ ☑ Campaign Comparison                                            │
│  ☰ ☑ Department Breakdown                                           │
│  ☰ ☑ Temporal Analysis                                              │
│  ☰ ☑ A/B Variant Results                                           │
│  ☰ ☑ Recommendations                                               │
│                                                                      │
│  Sections are pre-selected from template. Drag to reorder.          │
│                                                                      │
│                                     [← Back]  [Cancel]  [Next →]   │
└──────────────────────────────────────────────────────────────────────┘
```

**Step 3 — Output Settings:**
```
┌──────────────────────────────────────────────────────────────────────┐
│  Generate Report                                            [✕]     │
│  ── Step 3 of 3: Output Settings ────────────────────────────────── │
│                                                                      │
│  Report Title   [Q1 Executive Summary - April 2026       ]          │
│                                                                      │
│  Format         ○ PDF  ○ HTML  ○ CSV  ○ JSON                        │
│                 (PDF selected)                                       │
│                                                                      │
│  ☑ Include charts and visualizations                                 │
│  ☑ Include company branding                                          │
│  ☐ Include raw data appendix                                         │
│                                                                      │
│  Estimated size: ~2.4 MB                                             │
│                                                                      │
│                               [← Back]  [Cancel]  [Generate →]     │
└──────────────────────────────────────────────────────────────────────┘
```

### 9.4 Step Validation

**Step 1:**
- At least one campaign must be selected. "Next" button disabled until a campaign is checked.
- Template selection is optional. "Start from scratch" clears all section pre-selections.

**Step 2:**
- At least one section must be checked.
- Section availability depends on data: "A/B Variant Results" is greyed out with a tooltip "No A/B variants in selected campaigns" if no selected campaign has variants.
- "Target Detail" section is hidden for Defender role.

**Step 3:**
- Report title is required, max 200 characters. Auto-generated from template name + date, editable.
- "Include charts" is disabled and unchecked when format is CSV or JSON.
- "Include company branding" is disabled when format is CSV or JSON.

### 9.5 Generation Request

Clicking "Generate" sends `POST /api/v1/reports` with body:
```json
{
  "template_id": "uuid-or-null",
  "title": "Q1 Executive Summary - April 2026",
  "format": "pdf",
  "campaign_ids": ["uuid-1", "uuid-2"],
  "date_range": { "start": "2026-03-04", "end": "2026-04-03" },
  "departments": [],
  "sections": ["executive_summary", "email_funnel", "campaign_comparison", ...],
  "options": {
    "include_charts": true,
    "include_branding": true,
    "include_raw_data": false
  }
}
```

### 9.6 Generation Progress

After submission, the dialog transitions to a progress view:

```
┌──────────────────────────────────────────────────────────────────────┐
│  Generating Report                                          [✕]     │
├──────────────────────────────────────────────────────────────────────┤
│                                                                      │
│  Q1 Executive Summary - April 2026                                   │
│                                                                      │
│  ████████████████████░░░░░░░░░░░░  62%                               │
│                                                                      │
│  Processing: Campaign Comparison section...                          │
│                                                                      │
│  Estimated time remaining: ~15 seconds                               │
│                                                                      │
│  You can close this dialog. The report will continue generating      │
│  in the background and will appear in your Downloads when ready.     │
│                                                                      │
│                                          [Close]  [Cancel Generation]│
└──────────────────────────────────────────────────────────────────────┘
```

**Progress Polling:**
- After `POST /api/v1/reports` returns the report ID, the frontend polls `GET /api/v1/reports/{id}` every 3 seconds.
- The response includes `status` (queued, processing, completed, failed), `progress` (0-100), and `current_section` (name of section being processed).
- The progress bar animates smoothly between polled values using CSS transitions.

**Completion:**
- On status "completed": progress bar fills to 100%, message changes to "Report ready!", and a "Download" button appears.
- On status "failed": progress bar turns `--danger` color, message shows error detail from the API response, and a "Retry" button replaces the download action.

**Background Generation:**
- If the user closes the dialog during generation, a toast appears: "Report generating in background. We'll notify you when it's ready."
- When generation completes in background, a toast notification appears: "Report '{title}' is ready. [Download]".
- The download link in the toast calls `GET /api/v1/reports/{id}/download`.

---

## 10. Report Download and Format Options

### 10.1 Generated Reports List

A "Generated Reports" tab within the Reports section shows previously generated reports.

```
┌──────────────────────────────────────────────────────────────────────┐
│  Reports                                                             │
│  [Templates] [Generated Reports]                                     │
├──────────────────────────────────────────────────────────────────────┤
│  [Search reports...                          ] [Status ▾] [Format ▾] │
├──┬────────────────────────────┬────────┬────────┬───────┬──────┬────┤
│  │ Title                      │ Format │ Status │ Size  │ Date │ ···│
├──┼────────────────────────────┼────────┼────────┼───────┼──────┼────┤
│  │ Q1 Executive Summary       │ PDF    │ ● Done │ 2.4MB │ Apr 3│  ↓ │
│  │ March Campaign Comparison  │ PDF    │ ● Done │ 1.8MB │ Mar 31│ ↓ │
│  │ Compliance Export Q1       │ CSV    │ ● Done │ 340KB │ Mar 31│ ↓ │
│  │ Weekly Digest              │ HTML   │ ● Done │ 1.1MB │ Mar 28│ ↓ │
│  │ Full Data Export           │ JSON   │ ◐ 45%  │ —     │ Apr 3│  ⏳ │
├──┴────────────────────────────┴────────┴────────┴───────┴──────┴────┤
│  Showing 1–5 of 23                                 [← 1  2  →]     │
└──────────────────────────────────────────────────────────────────────┘
```

### 10.2 Table Columns

| Column | Width | Content | Sortable |
|--------|-------|---------|----------|
| Title | flex | Report title, clickable to view details | Yes |
| Format | 70px | Format badge: PDF, CSV, JSON, HTML | Yes |
| Status | 80px | ● Done (`--success`), ◐ {pct}% (`--warning`), ● Failed (`--danger`) | Yes |
| Size | 70px | File size, or "—" if not yet complete | Yes |
| Date | 80px | Generation date | Yes |
| Download | 48px | Download icon button (↓) for completed, spinner (⏳) for in-progress | No |

### 10.3 Download Behavior

- Clicking the download icon triggers `GET /api/v1/reports/{id}/download`.
- The browser initiates a file download with the appropriate MIME type and filename.
- Filename format: `{title}_{date}.{ext}` (e.g., `Q1_Executive_Summary_2026-04-03.pdf`).

### 10.4 Format-Specific Details

| Format | MIME Type | Contents |
|--------|-----------|----------|
| PDF | `application/pdf` | Full formatted report with charts rendered as images, branding, page numbers, table of contents |
| HTML | `text/html` | Self-contained HTML file with inline styles and SVG charts, viewable in any browser |
| CSV | `text/csv` | Tabular data only, one CSV file per section (delivered as ZIP if multiple sections), no charts |
| JSON | `application/json` | Structured data matching the report sections, machine-readable for integration |

### 10.5 Kebab Menu Actions on Generated Reports

- **Download** — same as the download button.
- **Preview** (PDF and HTML only) — opens the report in a new browser tab for viewing before download.
- **Regenerate** — opens the generation dialog pre-populated with the same configuration. Does not overwrite the existing report.
- **Delete** — confirmation dialog: "Delete report '{title}'? The file will be permanently removed." Calls `DELETE /api/v1/reports/{id}` (if supported) or marks as hidden client-side.

### 10.6 Report Detail View

Clicking a report title opens a detail panel (slide-over from right):

- Report title, format, status, file size, generation date.
- Configuration summary: which campaigns, date range, sections, and options were used.
- "Download" button.
- "Regenerate with same settings" link.
- If the report is still in progress, the progress bar and status from section 9.6 are shown.

---

## 11. Chart Interactions

### 11.1 Common Chart Behaviors

All chart widgets across the metrics dashboard share these interaction patterns:

**Tooltips:**
- Appear on hover with a 100ms delay.
- Background: `--bg-tertiary` (`#141b2d`) with `--border-default` border and subtle box shadow.
- Text: `--text-primary` for values, `--text-secondary` for labels.
- Positioned to avoid viewport overflow (automatically flip above/below/left/right).
- Dismiss on mouse-out with no delay.

**Chart Header Bar:**
- Every chart widget has a header row with the chart title on the left and action icons on the right.
- Action icons: export menu (`[⋯]` kebab or explicit `[PNG][SVG]` buttons).
- Hover on action icons: `--bg-hover` background with border-radius 4px.

### 11.2 Export Functionality

Each chart widget supports export via the header action menu:

**Export Options:**
- **PNG** — rasterized image at 2x resolution for clarity. Includes chart title, legend, and a subtle watermark "Generated by Tackle" in the bottom-right corner.
- **SVG** — vector format, scalable. Same content as PNG but as vector paths.

**Export Behavior:**
- Clicking PNG or SVG immediately triggers a browser download.
- Filename format: `{chart_title}_{date}.{ext}` (e.g., `Email_Funnel_2026-04-03.png`).
- The exported image uses the same dark theme colors as the on-screen chart.

**Kebab Menu (⋯) Additional Options:**
- Copy to clipboard (PNG only) — copies the chart image to the system clipboard. Toast: "Chart copied to clipboard."
- Full screen — expands the chart to a fullscreen modal overlay for presentation. Press Escape or click the close button to exit.
- View data — opens a modal showing the raw data table behind the chart, with a "Copy as CSV" button.

### 11.3 Click-Through Drill-Down

Charts that support click-through follow a consistent pattern:

1. User clicks a data element (bar, point, funnel stage, heatmap cell).
2. A slide-over panel opens from the right (480px wide) showing the underlying data.
3. The panel header shows what was clicked (e.g., "Opened — Q1 Exec Spear").
4. The panel body shows a sortable, paginated table of relevant records.
5. A "View all in Targets" link at the bottom navigates to the Targets page pre-filtered.

**Operator role:** Table shows target name, email, department, timestamp, and additional relevant fields.
**Defender role:** Click-through is disabled. Cursor remains `default` on data elements. No visual affordance suggesting clickability.

### 11.4 Chart Animations

- Initial render: bars grow from zero, lines draw from left to right. Duration 600ms, ease-out.
- Data update (filter change): crossfade transition, 300ms.
- Hover highlight: non-hovered elements dim to 40% opacity, 150ms transition.
- No animation when `prefers-reduced-motion` is enabled.

### 11.5 Chart Responsiveness

| Breakpoint | Behavior |
|------------|----------|
| Desktop (>1280px) | Charts in 2-column grid where specified |
| Tablet (768-1280px) | Charts stack to single column, maintain aspect ratio |
| Mobile (<768px) | Charts fill full width, legends collapse to horizontal scroll, heatmap shows abbreviated labels |

---

## 12. Real-Time Data Refresh

### 12.1 WebSocket Integration

When the "Live" toggle is enabled and at least one active campaign is in the current filter, the dashboard subscribes to WebSocket channels for real-time updates.

**WebSocket Channels:**
- `metrics.campaign.{campaign_id}` — campaign-level aggregate updates.
- `metrics.email.{campaign_id}` — email delivery events (sent, delivered, bounced, opened, clicked).
- `metrics.capture.{campaign_id}` — credential capture events.

**Event Handling:**
- On receiving a WebSocket event, the affected chart widget updates incrementally (no full reload).
- Stat cards increment their counters with a brief count-up animation (200ms).
- Funnel bars extend smoothly (CSS transition 300ms).
- New data points append to time-series charts with smooth animation.
- A subtle pulse effect (single `--accent-primary` ring expanding outward, 500ms) appears on the updated widget to draw attention.

### 12.2 Polling Fallback

When WebSocket connection is unavailable or "Live" mode is disabled:

- Dashboard polls `GET /api/v1/metrics/campaigns` (and other metric endpoints as needed) every 30 seconds.
- A small timestamp in the filter bar shows "Last updated: {time}" in `--text-muted`.
- On poll failure: the timestamp text changes to `--warning` color and appends "(stale)". Polling continues with exponential backoff: 30s, 60s, 120s, max 300s.
- On reconnection: all data reloads fully and polling interval resets to 30s.

### 12.3 WebSocket Connection Status

The Live toggle indicator reflects connection state:

| State | Indicator | Behavior |
|-------|-----------|----------|
| Connected | Pulsing green dot | Data streaming actively |
| Connecting | Pulsing amber dot | Attempting connection or reconnection |
| Disconnected | Static red dot | Connection lost, falls back to polling. Tooltip: "Live connection lost. Using 30s polling." |
| Disabled | No dot (grey toggle) | User has toggled Live off |

**Reconnection:** On WebSocket disconnect, the client attempts to reconnect with exponential backoff (1s, 2s, 4s, 8s, max 30s). After 5 failed attempts, it falls back to polling and shows a toast: "Live connection unavailable. Dashboard will refresh every 30 seconds."

### 12.4 Data Consistency

- When switching from live mode to historical (changing the date range to a past period), the WebSocket subscription is automatically dropped and live toggle is disabled.
- When a campaign transitions from active to completed (received via WebSocket), the live indicator for that campaign stops, and a toast is shown: "Campaign '{name}' has completed."
- If multiple browser tabs are open, each tab maintains its own WebSocket connection. No cross-tab coordination is attempted.

---

## 13. Error States and Loading Patterns

### 13.1 Initial Page Load

**Skeleton Loading:**
- On initial page load, all chart widgets display skeleton placeholders.
- Stat cards show animated shimmer rectangles matching the card dimensions.
- Chart areas show a skeleton with a faint grid pattern and shimmer bars/lines suggesting chart shape.
- Skeleton animation: left-to-right shimmer gradient using `--bg-secondary` to `--bg-hover` to `--bg-secondary`, 1.5s duration, infinite loop.
- Skeleton state lasts until the corresponding API response is received. Each widget loads independently.

### 13.2 Widget-Level Loading

When filters change, each widget independently enters a loading state:

- The widget content dims to 40% opacity.
- A small spinner appears centered over the dimmed content.
- The widget header and export buttons remain fully opaque and interactive (export exports the currently displayed data).
- Duration: until the API response is received, minimum 200ms to prevent flash.

### 13.3 Error States

**Per-Widget Error:**
If a single API call fails, the affected widget shows an error state while other widgets continue to function.

```
┌──────────────────────────────────────────────────────────────────────┐
│  Campaign Comparison                                          [⋯]  │
│                                                                      │
│              ┌───────────────────────────┐                           │
│              │     ⚠ Unable to load      │                           │
│              │  campaign comparison data  │                           │
│              │                           │                           │
│              │       [Retry]             │                           │
│              └───────────────────────────┘                           │
│                                                                      │
└──────────────────────────────────────────────────────────────────────┘
```

- Error icon: warning triangle in `--warning` color.
- Message: "Unable to load {widget_name} data".
- "Retry" button: re-fetches the specific API call for this widget.
- If retry also fails: message appends "Check your connection or try again later."
- Error state preserves the widget's place in the layout (no layout shift).

**Full Page Error:**
If all API calls fail (e.g., network down), the page shows a centered error state:
- Icon: warning triangle, large (48px).
- Heading: "Unable to load metrics".
- Subtext: "Please check your connection and try again."
- "Retry" button reloads all data.

### 13.4 Empty States

**No Campaigns:**
When no campaigns match the selected filters:
```
No data to display

Adjust your filters or select different campaigns to see metrics.
[Clear Filters]
```

**No Data for Widget:**
When a specific widget has no data (e.g., no A/B variants, no credential captures):
- The widget renders at half height with centered text: "No {data type} data available for the selected filters."
- Text in `--text-muted`.
- No retry button (this is an expected state, not an error).

**No Report Templates:**
```
No report templates yet

Create a template to define reusable report configurations.
[+ Create Template]
```

**No Generated Reports:**
```
No reports generated yet

Select a template and generate your first report.
[Generate Report]
```

### 13.5 API Error Handling

| HTTP Status | Behavior |
|-------------|----------|
| 401 | Redirect to login (session expired). |
| 403 | Widget shows "You don't have permission to view this data." No retry button. |
| 404 | Widget shows "Data not found." This may occur if a campaign was deleted. |
| 422 | Filter validation error. Toast: "Invalid filter parameters. Resetting to defaults." Filters reset. |
| 429 | Toast: "Too many requests. Data will refresh shortly." Polling interval doubles temporarily. |
| 500 | Widget error state with retry button (section 13.3). |
| Network error | Full page error if initial load; widget-level error with retry for subsequent loads. |

### 13.6 Report Generation Errors

| Error | Displayed Message | Recovery |
|-------|-------------------|----------|
| Generation timeout (>5 min) | "Report generation is taking longer than expected." | "Continue waiting" or "Cancel" buttons. |
| Generation failure | "Report generation failed: {api_error_message}" | "Retry" button re-submits with same configuration. |
| Download failure | Toast: "Download failed. Please try again." | Download button remains active for retry. |
| Invalid template | "This template references data that is no longer available." | "Edit Template" link. |

### 13.7 Permissions and Role-Based Restrictions

When a user lacks permission for certain data, the UI degrades gracefully rather than showing errors:

**Defender Role Restrictions:**
- Per-target detail widgets (target list, click-through panels) are not rendered at all.
- The "Target Detail" section in report templates is hidden.
- Funnel stages and chart data points do not show click affordances (no pointer cursor, no hover underline).
- Stat cards show aggregate numbers only; the "vs. previous" comparison may be hidden if it requires per-target data.
- A subtle banner at the top of the metrics page reads: "Showing aggregate data. Per-target details require Operator access." in `--text-muted`, dismissible.

**Template/Report Access:**
- Users can only see report templates they created or that are marked as shared/organization-wide.
- Generated reports are visible only to the user who generated them and to Admin role users.
- Attempting to access another user's report via direct URL returns 403, handled per section 13.5.

---

## 14. Chart Color Palette

All charts use the following dark-theme-compatible color palette for data series:

| Index | Hex | Usage |
|-------|-----|-------|
| 0 | `#4a7ab5` | Primary data series, Variant A |
| 1 | `#7c5cbf` | Secondary data series, Variant B |
| 2 | `#38a169` | Tertiary series, success indicators |
| 3 | `#d69e2e` | Quaternary series, warning indicators |
| 4 | `#e53e3e` | Quinary series, danger indicators |
| 5 | `#e8853a` | Orange, transition metrics |
| 6 | `#63b3ed` | Light blue, supplementary |
| 7 | `#b794f4` | Light purple, supplementary |
| 8 | `#68d391` | Light green, supplementary |
| 9 | `#f6ad55` | Light orange, supplementary |

- Charts with more than 10 series cycle back through the palette with reduced opacity (80%).
- All chart colors meet WCAG AA contrast requirements against `--bg-secondary` (`#0f1525`).
- Chart gridlines: `#1e2a3a` (`--border-default`), 1px.
- Chart axis labels: `--text-muted` (`#64748b`), 12px.
- Chart value labels: `--text-primary` (`#e2e8f0`), 14px semibold.
- Tooltip background: `--bg-tertiary` (`#141b2d`) with 1px `--border-default` border.

---

## 15. API Integration Summary

### 15.1 Metrics Endpoints

| Endpoint | Used By | Polling | WebSocket |
|----------|---------|---------|-----------|
| `GET /api/v1/metrics/campaigns` | Campaign comparison, stat cards | 30s | `metrics.campaign.*` |
| `GET /api/v1/metrics/endpoints` | Infrastructure health (dashboard) | 30s | — |
| `GET /api/v1/metrics/targets` | Time-to-capture, department breakdown | 30s | `metrics.capture.*` |
| `GET /api/v1/metrics/email-delivery` | Email funnel, temporal analysis, stat cards | 30s | `metrics.email.*` |

All metric endpoints accept common query parameters:
- `campaign_ids[]` — filter by campaign(s).
- `start_date`, `end_date` — date range.
- `departments[]` — filter by department.
- `template_ids[]` — filter by email template.
- `group_by` — grouping dimension (e.g., `template`, `landing_page`, `hour_of_day`, `day_of_week`, `department`).

### 15.2 Report Endpoints

| Endpoint | Method | Used By |
|----------|--------|---------|
| `/api/v1/reports/templates` | GET | Template list |
| `/api/v1/reports/templates` | POST | Create template |
| `/api/v1/reports/templates/{id}` | GET | Template editor (load) |
| `/api/v1/reports/templates/{id}` | PUT | Template editor (save) |
| `/api/v1/reports/templates/{id}` | DELETE | Delete template |
| `/api/v1/reports` | POST | Generate report |
| `/api/v1/reports/{id}` | GET | Report status/progress polling |
| `/api/v1/reports/{id}/download` | GET | Download report file |

### 15.3 Caching Strategy

- Metric data for completed campaigns is cached client-side for the duration of the session. Cache key: `{endpoint}:{filters_hash}`.
- Metric data for active campaigns is never cached (always fetched fresh or updated via WebSocket).
- Report template list is cached for 5 minutes, invalidated on any create/update/delete operation.
- Generated report list is cached for 1 minute.
- Chart export (PNG/SVG) is generated client-side from the rendered chart; no server call needed.
