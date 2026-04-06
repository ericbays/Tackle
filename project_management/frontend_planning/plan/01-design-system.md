# 01 — Design System

This document defines the visual design language for the Tackle admin UI. Every component, page, and interaction in the application adheres to these specifications. The design system is built around the Tackle brand identity — a dark, professional aesthetic derived from the logo's slate blue and navy palette with steel blue accents.

## Technology Stack Principles
- **Styling Utility**: Tailwind CSS is the absolute source of truth for styling. We avoid raw CSS and prioritize Tailwind utility classes for rapid, maintainable design.
- **Micro-Interactions**: Framer Motion powers all complex layout transitions, modal appearances, and micro-animations (spring physics).
- **Aesthetic Vibe**: Glassmorphism (using backdrop blurs), ambient light glows behind critical elements, and layered gradient borders to upgrade the flat UI to a premium hacker/security tool feel.

---

## 1. Color Palette

### 1.1 Core Colors

| Token | Hex | Usage |
|-------|-----|-------|
| `--bg-primary` | `#0a0f1a` | Page background, deepest layer |
| `--bg-secondary` | `#0f1525` | Card/panel backgrounds, sidebar |
| `--bg-tertiary` | `#141b2d` | Elevated surfaces (modals, dropdowns, tooltips) |
| `--bg-hover` | `#1a2340` | Hover state for interactive surfaces |
| `--bg-active` | `#1e2a4a` | Active/pressed state for interactive surfaces |
| `--border-default` | `#1e2a3a` | Default borders on cards, inputs, dividers |
| `--border-subtle` | `#162030` | Subtle borders for grouping within panels |
| `--border-strong` | `#2a3a52` | Emphasized borders (focused inputs, selected items) |

### 1.2 Text Colors

| Token | Hex | Usage |
|-------|-----|-------|
| `--text-primary` | `#e2e8f0` | Primary body text, headings |
| `--text-secondary` | `#94a3b8` | Secondary text, labels, descriptions |
| `--text-muted` | `#64748b` | Placeholder text, disabled text, timestamps |
| `--text-inverse` | `#0a0f1a` | Text on light/accent backgrounds |

### 1.3 Accent Colors

| Token | Hex | Usage |
|-------|-----|-------|
| `--accent-primary` | `#4a7ab5` | Primary buttons, links, active nav items, focus rings |
| `--accent-primary-hover` | `#3a6a9f` | Hover state for primary accent |
| `--accent-primary-muted` | `#4a7ab520` | Accent backgrounds (selected row, active tab indicator) |

### 1.4 Semantic Colors

| Token | Hex | Usage |
|-------|-----|-------|
| `--success` | `#38a169` | Success toasts, healthy status, completed states |
| `--success-muted` | `#38a16920` | Success backgrounds |
| `--warning` | `#d69e2e` | Warning toasts, expiring indicators, caution states |
| `--warning-muted` | `#d69e2e20` | Warning backgrounds |
| `--danger` | `#e53e3e` | Destructive buttons, error toasts, critical alerts, failed states |
| `--danger-muted` | `#e53e3e20` | Error/danger backgrounds |
| `--info` | `#4a90d9` | Informational toasts, help text, neutral indicators |
| `--info-muted` | `#4a90d920` | Info backgrounds |

### 1.5 Campaign Status Colors

| Status | Color | Token |
|--------|-------|-------|
| Draft | `#64748b` | `--status-draft` |
| Pending Approval | `#d69e2e` | `--status-pending` |
| Approved | `#4a7ab5` | `--status-approved` |
| Building | `#8b5cf6` | `--status-building` |
| Ready | `#06b6d4` | `--status-ready` |
| Active | `#38a169` | `--status-active` |
| Paused | `#f59e0b` | `--status-paused` |
| Completed | `#14b8a6` | `--status-completed` |
| Archived | `#475569` | `--status-archived` |
| Rejected | `#e53e3e` | `--status-rejected` |

### 1.6 Infrastructure Status Colors

| Status | Color |
|--------|-------|
| Online / Healthy | `--success` |
| Degraded / Warning | `--warning` |
| Offline / Error | `--danger` |
| Provisioning | `--status-building` |
| Terminated | `--status-archived` |

---

## 2. Typography

### 2.1 Font Stack

- **Primary font**: `Inter` — used for all UI text. Load weights 400 (regular), 500 (medium), 600 (semibold), 700 (bold).
- **Monospace font**: `JetBrains Mono` — used for code editors, log entries, correlation IDs, API keys, and technical values.
- **Fallback stack**: `Inter, -apple-system, BlinkMacSystemFont, 'Segoe UI', sans-serif`
- **Monospace fallback**: `'JetBrains Mono', 'Fira Code', 'Cascadia Code', 'Consolas', monospace`

### 2.2 Type Scale

| Token | Size | Weight | Line Height | Usage |
|-------|------|--------|-------------|-------|
| `--text-h1` | 28px | 700 | 1.2 | Page titles (e.g., "Campaigns", "Dashboard") |
| `--text-h2` | 22px | 600 | 1.3 | Section headings within pages |
| `--text-h3` | 18px | 600 | 1.4 | Card titles, modal titles |
| `--text-h4` | 15px | 600 | 1.4 | Subsection headings, tab labels |
| `--text-body` | 14px | 400 | 1.5 | Default body text |
| `--text-body-medium` | 14px | 500 | 1.5 | Emphasized body text, table headers |
| `--text-small` | 12px | 400 | 1.5 | Timestamps, helper text, badges |
| `--text-tiny` | 11px | 500 | 1.4 | Status badges, compact labels |
| `--text-code` | 13px | 400 | 1.6 | Code blocks, log entries, technical values |

### 2.3 Usage Guidelines

- Page titles use `--text-h1` and are the only element at that size on the page.
- Headings never skip levels (H1 → H2 → H3, never H1 → H3).
- Table column headers use `--text-body-medium` with `--text-secondary` color and uppercase letter-spacing of `0.05em`.
- All numbers in metrics/dashboards use tabular figures (`font-variant-numeric: tabular-nums`) to prevent layout shift on updates.

---

## 3. Spacing System

### 3.1 Base Unit

All spacing uses a **4px base unit** (`0.25rem`). Every margin, padding, and gap value is a multiple of 4px.

### 3.2 Spacing Scale

| Token | Value | Usage |
|-------|-------|-------|
| `--space-1` | 4px | Tight gaps (between icon and label, badge padding) |
| `--space-2` | 8px | Default gap between inline elements, small padding |
| `--space-3` | 12px | Input padding, compact card padding |
| `--space-4` | 16px | Default card padding, section gaps |
| `--space-5` | 20px | Medium section spacing |
| `--space-6` | 24px | Large section spacing, modal padding |
| `--space-8` | 32px | Page section dividers |
| `--space-10` | 40px | Major section breaks |
| `--space-12` | 48px | Page top padding |

### 3.3 Layout Spacing

- **Page content padding**: `--space-6` (24px) on all sides.
- **Card internal padding**: `--space-4` (16px).
- **Gap between cards/sections on a page**: `--space-6` (24px).
- **Gap between form fields**: `--space-4` (16px).
- **Gap between a label and its input**: `--space-1` (4px).
- **Table cell padding**: `--space-3` (12px) horizontal, `--space-2` (8px) vertical.

---

## 4. Border Radius

| Token | Value | Usage |
|-------|-------|-------|
| `--radius-sm` | 4px | Badges, small chips |
| `--radius-md` | 6px | Buttons, inputs, dropdowns |
| `--radius-lg` | 8px | Cards, panels, modals |
| `--radius-xl` | 12px | Large cards, hero sections |
| `--radius-full` | 9999px | Circular elements (avatars, status dots) |

---

## 5. Shadows

| Token | Value | Usage |
|-------|-------|-------|
| `--shadow-sm` | `0 1px 2px rgba(0, 0, 0, 0.3)` | Subtle elevation (buttons, inputs) |
| `--shadow-md` | `0 4px 12px rgba(0, 0, 0, 0.4)` | Cards, dropdowns |
| `--shadow-lg` | `0 8px 24px rgba(0, 0, 0, 0.5)` | Modals, slide-over panels |
| `--shadow-xl` | `0 16px 48px rgba(0, 0, 0, 0.6)` | Floating builder toolbar, elevated overlays |

---

## 6. Animation and Transitions

All animations are powered by **Framer Motion** to ensure buttery-smooth physics-based interactions that feel tangibly alive.

### 6.1 Timing (Framer Motion Defaults)

| Token | Value | Usage |
|-------|-------|-------|
| `--duration-fast` | `150ms` | Hover color changes, opacity toggles |
| `--duration-normal` | `200ms` | Button transitions, input focus, accordion open/close |
| `--duration-smooth` | `300ms` | Modal appear/disappear, slide-over, sidebar collapse |
| `--duration-slow` | `500ms` | Page-level transitions, complex animations |

### 6.2 Easing

| Token | Value | Usage |
|-------|-------|-------|
| `--ease-out` | `cubic-bezier(0.16, 1, 0.3, 1)` | Elements entering the viewport (modals, toasts, slide-overs) |
| `--ease-in` | `cubic-bezier(0.7, 0, 0.84, 0)` | Elements leaving the viewport |
| `--ease-in-out` | `cubic-bezier(0.45, 0, 0.55, 1)` | Elements repositioning (sidebar collapse, accordion) |
| `--ease-spring` | `cubic-bezier(0.34, 1.56, 0.64, 1)` | Playful interactions (drag snap, badge pop) |

### 6.3 Transition Behaviors (Framer Motion)

- **Modals**: Framer Motion `AnimatePresence`. Fade in backdrop, scale from 95% to 100% with spring physics (`type: spring, bounce: 0.3`).
- **Slide-over panels**: Slide in from the right edge (`--duration-smooth`, `--ease-out`). Background overlay fades in simultaneously.
- **Sidebar collapse**: Width transition (`--duration-smooth`, `--ease-in-out`). Text labels fade out before width animates. Icons remain stationary.
- **Toasts**: Slide in from the right + fade in (`--duration-normal`, `--ease-out`). Slide out + fade out on dismiss.
- **Accordions**: Height transition with `--duration-normal`, `--ease-in-out`. Content fades in slightly delayed.
- **Hover states**: Color transitions use `--duration-fast` with linear easing.
- **Focus rings**: Appear instantly (no transition delay). Use `2px solid --accent-primary` with `2px offset`.
- **Drag-and-drop snap**: Components snap to drop position with `100ms` `--ease-spring`.

### 6.4 Reduced Motion

When `prefers-reduced-motion: reduce` is active:
- All transitions set to `0ms` duration.
- Modals and slide-overs appear/disappear instantly.
- Toasts appear without animation.
- Drag-and-drop remains functional but without snap animation.

---

## 7. Iconography

### 7.1 Icon Library

Use **Lucide React** as the primary icon set. It aligns with the shadcn/ui aesthetic and provides consistent line-weight icons.

### 7.2 Icon Sizes

| Size | Pixels | Usage |
|------|--------|-------|
| `sm` | 16px | Inline with small text, table cells, badges |
| `md` | 20px | Buttons, nav items, form field icons |
| `lg` | 24px | Page headers, empty state illustrations, sidebar expanded items |
| `xl` | 32px | Dashboard stat cards, hero sections |

### 7.3 Icon Colors

- Icons inherit the text color of their context by default.
- Interactive icons use `--text-secondary` and transition to `--text-primary` on hover.
- Status icons use semantic colors (green for success, red for error, etc.).
- Nav icons use `--text-muted` when inactive, `--accent-primary` when active.

---

## 8. Component Design Tokens

### 8.1 Buttons

| Variant | Background | Text | Border | Hover BG |
|---------|-----------|------|--------|----------|
| Primary | `--accent-primary` | `--text-inverse` | none | `--accent-primary-hover` |
| Secondary | transparent | `--text-primary` | `--border-default` | `--bg-hover` |
| Danger | `--danger` | white | none | `#c53030` |
| Ghost | transparent | `--text-secondary` | none | `--bg-hover` |

- **Button padding**: `--space-2` (8px) vertical, `--space-4` (16px) horizontal.
- **Button border radius**: `--radius-md` (6px).
- **Button font**: `--text-body-medium` (14px/500).
- **Disabled state**: 40% opacity, `cursor: not-allowed`, no hover effect.
- **Loading state**: Text replaced by a spinner icon, button width preserved (no layout shift).

### 8.2 Inputs

- **Background**: `--bg-primary`.
- **Border**: `1px solid --border-default`.
- **Focus border**: `--accent-primary` with a `0 0 0 2px --accent-primary-muted` box shadow.
- **Error border**: `--danger` with `0 0 0 2px --danger-muted` box shadow.
- **Padding**: `--space-2` (8px) vertical, `--space-3` (12px) horizontal.
- **Border radius**: `--radius-md` (6px).
- **Placeholder color**: `--text-muted`.
- **Height**: 36px (default), 32px (compact for tables/filters).

### 8.3 Cards

- **Background**: `--bg-secondary`.
- **Border**: `1px solid --border-default`.
- **Border radius**: `--radius-lg` (8px).
- **Padding**: `--space-4` (16px).
- **Shadow**: none by default. `--shadow-sm` on hover for interactive cards.

### 8.4 Badges / Status Chips

- **Border radius**: `--radius-sm` (4px).
- **Padding**: `2px 8px`.
- **Font**: `--text-tiny` (11px/500), uppercase.
- **Background**: The status color at 15% opacity.
- **Text color**: The status color at full opacity.

### 8.5 Tables

- **Header row**: `--bg-tertiary` background, `--text-body-medium` font, `--text-secondary` color.
- **Body rows**: `--bg-secondary` background.
- **Alternating rows**: Not used (rely on border separators).
- **Row border**: `1px solid --border-subtle` between rows.
- **Row hover**: `--bg-hover` background.
- **Selected row**: `--accent-primary-muted` background with `--accent-primary` left border (3px).
- **Checkbox column**: 40px width, centered.
- **Actions column (kebab)**: 48px width, right-aligned.

### 8.6 Tooltips

- **Background**: `--bg-tertiary`.
- **Text**: `--text-primary`, `--text-small` (12px).
- **Border**: `1px solid --border-default`.
- **Border radius**: `--radius-md` (6px).
- **Padding**: `--space-1` (4px) vertical, `--space-2` (8px) horizontal.
- **Shadow**: `--shadow-md`.
- **Delay**: 500ms before appearing, instant dismiss.
- **Max width**: 240px.

---

## 9. Z-Index Scale

| Token | Value | Usage |
|-------|-------|-------|
| `--z-base` | 0 | Default page content |
| `--z-sticky` | 10 | Sticky table headers, floating action bar |
| `--z-sidebar` | 20 | Application sidebar |
| `--z-topbar` | 25 | Application top bar |
| `--z-dropdown` | 30 | Dropdown menus, select poppers |
| `--z-overlay` | 40 | Modal/slide-over backdrop |
| `--z-modal` | 50 | Modal content, slide-over panels |
| `--z-toast` | 60 | Toast notifications |
| `--z-tooltip` | 70 | Tooltips |
| `--z-command` | 80 | Command palette (Ctrl+K search) |

---

## 10. Responsive Breakpoints

| Token | Value | Behavior |
|-------|-------|----------|
| `--bp-mobile` | `< 768px` | Sidebar becomes overlay drawer. Tables switch to card layout or horizontal scroll. |
| `--bp-tablet` | `768px – 1023px` | Sidebar auto-collapses to icon-only. Content fills available space. |
| `--bp-desktop` | `1024px – 1439px` | Full layout with collapsible sidebar. Default experience. |
| `--bp-wide` | `≥ 1440px` | Content max-width applied. Extra space used for wider panels. |

---

## 11. WCAG Compliance

- All text meets **WCAG AA** contrast requirements:
  - `--text-primary` (#e2e8f0) on `--bg-primary` (#0a0f1a): contrast ratio **13.5:1** (passes AAA).
  - `--text-secondary` (#94a3b8) on `--bg-primary` (#0a0f1a): contrast ratio **6.8:1** (passes AA).
  - `--text-muted` (#64748b) on `--bg-primary` (#0a0f1a): contrast ratio **4.6:1** (passes AA for normal text).
- All interactive elements have visible focus indicators (focus ring).
- Color is never the sole indicator of state — status badges include text labels alongside color.
- All icons used as buttons have `aria-label` attributes.
