# 06 — Email Templates

This document specifies the Email Template management feature: listing, creating, editing, previewing, versioning, and sending test emails. Email templates define the phishing email content used in campaigns, supporting both WYSIWYG and raw HTML editing, variable interpolation, attachment management, and live preview with device simulation.

---

## 1. Template List View

### 1.1 Purpose

The template list is the primary management surface for all email templates. It displays a paginated, filterable, sortable table of templates with quick actions via kebab menus and a slide-over panel for quick preview without leaving the list.

### 1.2 Layout

```
┌──────────────────────────────────────────────────────────────────┐
│  Email Templates                          [+ New Template]       │
├──────────────────────────────────────────────────────────────────┤
│  ┌──────────────────────┐ ┌────────────┐ ┌────────────┐         │
│  │ 🔍 Search templates  │ │ Category ▾ │ │ Tags ▾     │         │
│  └──────────────────────┘ └────────────┘ └────────────┘         │
├──────────────────────────────────────────────────────────────────┤
│  Name ▾        │ Subject      │ Category │ Tags   │ Updated  │ ⋮│
│  ─────────────────────────────────────────────────────────────── │
│  IT Password   │ Urgent: Pas..│ IT       │ urgent │ 2h ago   │ ⋮│
│  HR Benefits   │ Open Enroll..│ HR       │ social │ 1d ago   │ ⋮│
│  CEO Wire      │ Wire Transf..│ Finance  │ bec    │ 3d ago   │ ⋮│
│  Delivery Not..│ Your packag..│ Shipping │ lure   │ 1w ago   │ ⋮│
│  ─────────────────────────────────────────────────────────────── │
│                                                                  │
│  Showing 1-20 of 47              [← Prev]  1  2  3  [Next →]   │
└──────────────────────────────────────────────────────────────────┘
```

### 1.3 Table Columns

| Column | Content | Default Sort | Sortable |
|--------|---------|-------------|----------|
| Name | Template name, truncated to 40 chars. Below the name in `--text-muted`: description truncated to 60 chars (if present). | — | Yes |
| Subject | Email subject line, truncated to 35 chars. | — | Yes |
| Category | Category badge with `--bg-tertiary` background. | — | Yes |
| Tags | Up to 3 tag chips displayed; overflow shown as "+N". | — | No |
| Updated | Relative timestamp (`2h ago`, `3d ago`). Tooltip shows absolute datetime. | Default descending | Yes |
| Shared | Lock/globe icon indicating `is_shared` status. Tooltip: "Shared" or "Private". | — | Yes |
| Actions | Kebab menu (⋮) — see section 1.6. | — | No |

### 1.4 Filters

All filters are applied as query parameters to `GET /api/v1/email-templates` and are AND-combined.

- **Search**: Text input with debounce (300ms). Searches across `name`, `subject`, and `description`. Maps to `?search=<term>`.
- **Category**: Dropdown multi-select. Options populated from a static list defined in config (e.g., IT, HR, Finance, Shipping, Social Media, Generic). Maps to `?category=<value>`.
- **Tags**: Dropdown multi-select with search-within-dropdown. Displays existing tags from the template corpus. Maps to `?tags=<tag1>,<tag2>`.

Filter state is persisted in the URL query string so that filtered views are shareable and survive page refresh.

### 1.5 Sorting

Clicking a column header cycles: unsorted -> ascending -> descending -> unsorted. The active sort column shows an arrow indicator (▲/▼). Default sort: `updated_at` descending. Maps to `?sort=<field>&order=asc|desc`.

### 1.6 Kebab Menu Actions

Clicking the kebab icon (⋮) on a row opens a dropdown menu with the following actions:

| Action | Icon | Behavior |
|--------|------|----------|
| Edit | Pencil | Navigate to full-page template editor (`/email-templates/{id}/edit`). |
| Quick Preview | Eye | Open the slide-over preview panel (section 1.7). |
| Duplicate | Copy | `POST /api/v1/email-templates/{id}/clone`. Creates a copy named `"{original name} (Copy)"`. On success, the new template appears at the top of the list and a success toast is shown. |
| Send Test | Paper plane | Opens the test email modal (section 7). |
| Export | Download | `POST /api/v1/email-templates/{id}/export`. Downloads the exported template file. |
| Delete | Trash (red) | Opens a confirmation modal (section 1.8). |

### 1.7 Slide-Over Quick Preview

Clicking "Quick Preview" or clicking a template row (anywhere except the kebab or name link) opens a slide-over panel from the right edge of the viewport.

```
┌───────────────────────────┬──────────────────────────────────┐
│                           │  ┌──────────────────────────────┐│
│  (List continues dimmed   │  │  IT Password Reset           ││
│   behind the overlay)     │  │  ──────────────────────────  ││
│                           │  │  Subject: Urgent: Password...││
│                           │  │  From: IT Support <it@...>   ││
│                           │  │  Category: IT  Tags: urgent  ││
│                           │  │  ──────────────────────────  ││
│                           │  │                              ││
│                           │  │  ┌──────────────────────────┐││
│                           │  │  │                          │││
│                           │  │  │  (Rendered HTML preview  │││
│                           │  │  │   with sample data)      │││
│                           │  │  │                          │││
│                           │  │  └──────────────────────────┘││
│                           │  │                              ││
│                           │  │  [Edit Template] [Send Test] ││
│                           │  └──────────────────────────────┘│
└───────────────────────────┴──────────────────────────────────┘
```

**Slide-over specifications:**
- Width: 520px on desktop; full-width on viewports below 768px.
- Background: `--bg-secondary`.
- Header: template name as title, close (X) button top-right.
- Metadata section: subject, sender name/email, category badge, tags, created/updated timestamps.
- Preview area: rendered HTML from `POST /api/v1/email-templates/{id}/preview` with sample data, displayed inside a sandboxed iframe.
- Footer buttons: "Edit Template" (navigates to editor page), "Send Test" (opens test email modal).
- Clicking outside the slide-over or pressing Escape closes it.

### 1.8 Delete Confirmation Modal

- Title: "Delete Template"
- Body: "Are you sure you want to delete **{template name}**? This action can be undone by an administrator."
- Buttons: [Cancel] (secondary) and [Delete] (danger red).
- On confirm: `DELETE /api/v1/email-templates/{id}` (soft delete). The row is removed from the list and a success toast is shown: "Template deleted."
- If the template is currently in use by an active campaign, the API returns a 409 Conflict. The frontend shows an error toast: "This template is in use by an active campaign and cannot be deleted."

### 1.9 Pagination

- 20 items per page (fixed).
- Pagination control at the bottom of the table: previous/next buttons plus page number links.
- When fewer than 20 total templates exist, the pagination control is hidden.
- Summary text: "Showing {start}-{end} of {total}".
- Page state is stored in the URL query string (`?page=2`).

### 1.10 Empty State

When no templates exist (or no templates match the current filters):

- **No templates at all**: Centered illustration with text "No email templates yet" and a primary button "Create Your First Template" that navigates to the editor.
- **No filter results**: Text "No templates match your filters" with a link "Clear all filters".

### 1.11 Loading State

- Table body shows 5 skeleton rows matching the column layout.
- Filter inputs are interactive immediately (skeleton only applies to data rows).

---

## 2. Template Editor Page

### 2.1 Purpose

The full-page editor is the primary interface for creating and editing email templates. It combines metadata fields, a WYSIWYG/HTML editor, a live preview panel, attachment management, and version history into a single workspace.

### 2.2 Route

- Create: `/email-templates/new`
- Edit: `/email-templates/{id}/edit`

### 2.3 Layout

```
┌──────────────────────────────────────────────────────────────────────────┐
│  ← Back to Templates    Email Template Editor       [Save Draft] [Save] │
│                          Auto-saved 30s ago                              │
├──────────────────────────────────────────────────────────────────────────┤
│  ┌────────────────────────────────────────┐ ┌──────────────────────────┐ │
│  │  Template Name*  [____________________]│ │  Sender Name*            │ │
│  │  Description     [____________________]│ │  [____________________]  │ │
│  │  Subject Line*   [____________________]│ │  Sender Email*           │ │
│  │  Category*       [Category ▾         ] │ │  [____________________]  │ │
│  │  Tags            [+ Add tag          ] │ │  ☐ Share with all users  │ │
│  └────────────────────────────────────────┘ └──────────────────────────┘ │
├──────────────────────────────────────────────────────────────────────────┤
│  ┌─────────────────────────────────────────────┐┌───────────────────────┐│
│  │  [WYSIWYG] [HTML]               [Toolbar...] ││  Preview              ││
│  │  ──────────────────────────────────────────── ││  [Desktop] [Mobile]   ││
│  │                                               ││  ────────────────────-││
│  │                                               ││  ┌──────────────────┐││
│  │  (Editor area — WYSIWYG or raw HTML           ││  │                  │││
│  │   depending on active tab)                    ││  │  (Live rendered   │││
│  │                                               ││  │   preview with    │││
│  │                                               ││  │   sample data)    │││
│  │                                               ││  │                  │││
│  │                                               ││  └──────────────────┘││
│  │                                               ││                      ││
│  └─────────────────────────────────────────────┘└───────────────────────┘│
├──────────────────────────────────────────────────────────────────────────┤
│  ┌────────────────────────────────┐  ┌──────────────────────────────────┐│
│  │  Attachments                  │  │  Text Body (Plain Text Fallback) ││
│  │  [+ Upload File]             │  │  ┌──────────────────────────────┐││
│  │  ──────────────────────────── │  │  │                              │││
│  │  report.pdf  245KB  [🗑]     │  │  │  (plain text editor area)    │││
│  │  invoice.xlsx  89KB  [🗑]    │  │  │                              │││
│  │  ──────────────────────────── │  │  └──────────────────────────────┘││
│  │  2 files, 334KB of 12MB used │  │  [Auto-generate from HTML]       ││
│  └────────────────────────────────┘  └──────────────────────────────────┘│
└──────────────────────────────────────────────────────────────────────────┘
```

### 2.4 Metadata Fields

The metadata section sits above the editor area in a two-column layout (single column on mobile).

| Field | Type | Required | Validation | API Field |
|-------|------|----------|------------|-----------|
| Template Name | Text input | Yes | 1-150 characters | `name` |
| Description | Text input | No | Max 500 characters | `description` |
| Subject Line | Text input | Yes | 1-255 characters. Supports template variables (rendered in preview). | `subject` |
| Category | Select dropdown | Yes | Must select one from predefined list | `category` |
| Tags | Tag input (comma-separated, Enter to add) | No | Each tag: 1-50 chars, alphanumeric + hyphens. Max 10 tags. | `tags` |
| Sender Name | Text input | Yes | 1-100 characters | `sender_name` |
| Sender Email | Text input | Yes | Must be a valid email address format | `sender_email` |
| Share with all users | Checkbox | No | — | `is_shared` |

**Subject Line Variable Support:** The subject line input supports template variables. A small "Insert Variable" button to the right of the input opens the same variable insertion dropdown used in the editor (section 3.4). The inserted variable text (e.g., `{{target.first_name}}`) is placed at the cursor position in the subject input.

### 2.5 Editor / Preview Split

The editor and preview sit side by side in a resizable split layout:

- Default split: 55% editor / 45% preview.
- A draggable divider between the two panels allows resizing. Minimum width for either panel: 300px.
- On viewports below 1024px, the split collapses to a tabbed layout with "Editor" and "Preview" tabs.
- The preview panel can be collapsed entirely via a toggle button on the divider, giving the editor full width.

### 2.6 Page Header

- **Back link**: "← Back to Templates" navigates to the list view. If there are unsaved changes, an "Unsaved Changes" confirmation modal appears first (see section 9.3).
- **Title**: "Email Template Editor" (create mode) or the template name (edit mode).
- **Auto-save indicator**: Displays "Auto-saved {time}" or "Saving..." or "Unsaved changes" (see section 9).
- **Save Draft button** (secondary): Saves the template with its current state without running validation. Calls `PUT /api/v1/email-templates/{id}`.
- **Save button** (primary): Validates the template (`POST /api/v1/email-templates/{id}/validate`) and then saves. If validation fails, errors are shown inline and the save is blocked.

---

## 3. WYSIWYG Editor & Toolbar

### 3.1 Editor Engine

The editor uses a modern WYSIWYG framework (TipTap or Lexical) configured for email-compatible HTML output. The editor produces clean HTML suitable for email clients — no JavaScript, no external stylesheets, inline styles only.

### 3.2 Editor Tabs

Two tabs sit above the editor area:

- **WYSIWYG** (default): Rich text editing with the toolbar.
- **HTML**: Raw HTML code editor with syntax highlighting (monospace font, line numbers, bracket matching).

Switching between tabs syncs content bidirectionally:
- WYSIWYG -> HTML: The current WYSIWYG content is serialized to HTML and displayed in the code editor.
- HTML -> WYSIWYG: The raw HTML is parsed and loaded into the WYSIWYG editor. If the HTML contains constructs that cannot be represented in WYSIWYG (e.g., complex tables, custom CSS), a warning banner appears above the WYSIWYG editor: "Some HTML elements may not display correctly in the visual editor. Use HTML mode for full control."

### 3.3 Toolbar (WYSIWYG Mode)

The toolbar is displayed only when the WYSIWYG tab is active. It is hidden in HTML mode.

```
┌──────────────────────────────────────────────────────────────────────┐
│ B  I  U  S  │ H1 H2 H3 │ 🔗  📷  │ ≡  ≡  ≡  │ •  1. │ {x} ▾ │ ⊞ │
│              │          │         │ L  C  R  │      │       │   │
└──────────────────────────────────────────────────────────────────────┘
```

**Toolbar groups (left to right, separated by vertical dividers):**

| Group | Controls | Behavior |
|-------|----------|----------|
| **Inline Formatting** | Bold (B), Italic (I), Underline (U), Strikethrough (S) | Toggle buttons. Active state highlighted with `--accent-primary-muted` background. Keyboard shortcuts: Ctrl+B, Ctrl+I, Ctrl+U. |
| **Headings** | H1, H2, H3 | Mutually exclusive toggle buttons. Clicking the active heading button removes the heading (reverts to paragraph). |
| **Insert** | Link (🔗), Image (📷) | Link: opens a popover with URL input and "Open in new tab" checkbox. Image: opens a popover with URL input for image source (no file upload — email images must be hosted). Alt text field included. |
| **Alignment** | Left, Center, Right | Mutually exclusive. Default: Left. Applied to the current block element. |
| **Lists** | Unordered (•), Ordered (1.) | Toggle between list types. Pressing the active list button removes the list. Tab/Shift+Tab for nesting. |
| **Variables** | Insert Variable ({x} ▾) dropdown | See section 3.4. |
| **Table** | Table (⊞) | Opens a grid selector (up to 6x6) for inserting a table. Once inserted, table controls appear in a floating toolbar: add/remove row, add/remove column, merge cells, delete table. |

### 3.4 Variable Insertion Dropdown

The "Insert Variable" button ({x} icon with a dropdown caret) opens a categorized dropdown menu.

```
┌──────────────────────────────┐
│  Insert Variable             │
│  ────────────────────────    │
│  TARGET                      │
│    First Name                │
│    Last Name                 │
│    Email                     │
│    Department                │
│    Title                     │
│  ────────────────────────    │
│  TRACKING                    │
│    Tracking Pixel            │
│    Phishing URL              │
│  ────────────────────────    │
│  CAMPAIGN                    │
│    Campaign Name             │
│  ────────────────────────    │
│  SENDER                      │
│    Sender Name               │
│    Sender Email              │
│  ────────────────────────    │
│  ⓘ Variables are replaced    │
│    with actual values when   │
│    the email is sent.        │
└──────────────────────────────┘
```

**Variable mapping:**

| Display Name | Inserted Text | Category |
|-------------|---------------|----------|
| First Name | `{{target.first_name}}` | Target |
| Last Name | `{{target.last_name}}` | Target |
| Email | `{{target.email}}` | Target |
| Department | `{{target.department}}` | Target |
| Title | `{{target.title}}` | Target |
| Tracking Pixel | `{{tracking.pixel}}` | Tracking |
| Phishing URL | `{{tracking.url}}` | Tracking |
| Campaign Name | `{{campaign.name}}` | Campaign |
| Sender Name | `{{sender.name}}` | Sender |
| Sender Email | `{{sender.email}}` | Sender |

**Insertion behavior:**
- In WYSIWYG mode: the variable is inserted as a styled inline chip (pill-shaped, `--accent-primary-muted` background, `--accent-primary` text) at the cursor position. The chip is non-editable as a unit — Backspace deletes the entire chip. The chip displays the friendly name (e.g., "First Name") but serializes to `{{target.first_name}}` in the HTML output.
- In HTML mode: the raw variable text (e.g., `{{target.first_name}}`) is inserted at the cursor position as plain text.
- In the Subject Line input: the raw variable text is inserted at the cursor position.

### 3.5 Editor Behavior Details

- **Paste handling**: Pasting from external sources (Word, web pages) strips non-email-safe markup. Classes, external stylesheets, scripts, and iframes are removed. Basic formatting (bold, italic, links, images, tables) is preserved.
- **Undo/Redo**: Ctrl+Z / Ctrl+Shift+Z with a deep undo history (100+ levels).
- **Focus ring**: The editor area shows a `--accent-primary` focus ring when focused.
- **Minimum height**: 400px. The editor area grows vertically with content (no internal scrollbar until the page itself scrolls).
- **Character count**: A subtle character count is displayed below the editor in `--text-muted`: "{count} characters".

---

## 4. Live Preview Panel

### 4.1 Purpose

The preview panel renders the email template with sample data in real time, giving the template author immediate visual feedback. It supports desktop and mobile viewport simulation.

### 4.2 Rendering

- Preview is rendered by calling `POST /api/v1/email-templates/{id}/preview` with the current `html_body` and `subject` values.
- For unsaved templates (create mode), the preview is rendered client-side using a local variable substitution engine that replaces `{{variable}}` placeholders with sample data.
- The preview is debounced: it re-renders 500ms after the user stops typing in the editor.
- The rendered HTML is displayed inside a sandboxed iframe (`sandbox="allow-same-origin"` — no scripts) to prevent template content from affecting the application UI.

### 4.3 Sample Data

The preview uses hardcoded sample data for variable substitution:

| Variable | Sample Value |
|----------|-------------|
| `{{target.first_name}}` | Jane |
| `{{target.last_name}}` | Smith |
| `{{target.email}}` | jane.smith@example.com |
| `{{target.department}}` | Marketing |
| `{{target.title}}` | Senior Manager |
| `{{tracking.pixel}}` | (1x1 transparent pixel placeholder) |
| `{{tracking.url}}` | `https://example.com/track/sample` |
| `{{campaign.name}}` | Sample Campaign |
| `{{sender.name}}` | (value from the Sender Name metadata field) |
| `{{sender.email}}` | (value from the Sender Email metadata field) |

### 4.4 Device Simulation

Two toggle buttons at the top of the preview panel control viewport simulation:

- **Desktop** (default): Preview iframe renders at the full width of the preview panel.
- **Mobile**: Preview iframe is constrained to 375px width, centered within the panel with a subtle device-frame border (`--border-default`). The panel itself maintains its width; only the iframe within it narrows.

```
Desktop mode:                    Mobile mode:
┌──────────────────────────┐    ┌──────────────────────────┐
│  Preview                 │    │  Preview                 │
│  [Desktop] [Mobile]      │    │  [Desktop] [Mobile]      │
│  ────────────────────    │    │  ────────────────────    │
│  ┌──────────────────────┐│    │      ┌───────────┐      │
│  │                      ││    │      │           │      │
│  │  Dear Jane,          ││    │      │ Dear Jane,│      │
│  │                      ││    │      │           │      │
│  │  Your password will  ││    │      │ Your pass │      │
│  │  expire in 24 hours. ││    │      │ word will │      │
│  │                      ││    │      │ expire... │      │
│  │  [Reset Password]    ││    │      │           │      │
│  │                      ││    │      │ [Reset]   │      │
│  └──────────────────────┘│    │      └───────────┘      │
└──────────────────────────┘    └──────────────────────────┘
```

### 4.5 Preview Header

Above the iframe, the preview panel shows a simulated email header:

```
┌──────────────────────────────────────┐
│  From: IT Support <it@company.com>   │
│  To: Jane Smith <jane.smith@...>     │
│  Subject: Urgent: Password Expiry    │
│  ──────────────────────────────────  │
│  (rendered email body below)         │
└──────────────────────────────────────┘
```

The From, To, and Subject values use the metadata fields and sample data. This gives the author a realistic impression of how the email will appear in a recipient's inbox.

### 4.6 Preview Error State

If the preview rendering fails (e.g., malformed HTML that crashes the parser), the preview panel shows:

- A warning icon with the message "Preview could not be rendered."
- The specific error message from the API (if available) in `--text-muted`.
- The previous successfully rendered preview remains visible (dimmed at 50% opacity) behind the error overlay, so the author does not lose context.

---

## 5. Attachment Management Panel

### 5.1 Layout

The attachment panel sits below the editor/preview split. It displays a list of currently attached files and provides upload/delete controls.

### 5.2 Upload

- **Upload button**: "+ Upload File" opens the system file picker. Multiple files can be selected at once.
- **Drag and drop**: Files can be dragged onto the attachment panel area. A dashed border highlight (`--accent-primary`) appears when a file is dragged over.
- **Multipart upload**: Files are uploaded via multipart form data to the attachments endpoint.
- **Size limit**: Individual file limit is 12MB. Total attachment size for a template is also 12MB. If an upload would exceed the limit, the upload is rejected with an inline error: "Total attachment size cannot exceed 12MB. Current usage: {used}MB."
- **Progress**: Each file shows an upload progress bar during upload.

### 5.3 Attachment List

Each attachment is displayed as a row:

```
┌──────────────────────────────────────────────────┐
│  📎 report.pdf                  245 KB      [🗑] │
│  📎 invoice.xlsx                 89 KB      [🗑] │
│  ────────────────────────────────────────────     │
│  2 files, 334 KB of 12 MB used                   │
└──────────────────────────────────────────────────┘
```

- **File icon**: Generic paperclip icon (no file-type-specific icons needed).
- **File name**: Truncated to 40 characters with tooltip showing full name.
- **File size**: Formatted in KB or MB.
- **Delete button**: Trash icon. Clicking immediately deletes the attachment (no confirmation modal — attachments are easily re-uploaded). Calls the delete attachment endpoint. On success, the row is removed and the usage summary updates.
- **Usage summary**: Shows total file count and total size vs. the 12MB limit.

### 5.4 File Type Restrictions

No file type restrictions are enforced on the frontend. The backend may reject certain file types — if so, the error message from the API response is displayed as an inline error below the upload button.

---

## 6. Version History

### 6.1 Access

Version history is accessible from the template editor page via a "Version History" button in the page header (visible only in edit mode, not create mode). Clicking it opens a slide-over panel from the right.

### 6.2 Version List

```
┌──────────────────────────────────────┐
│  Version History              [X]    │
│  ────────────────────────────────    │
│  v5 (current)                        │
│  Saved 10 minutes ago                │
│  by operator@tackle.io               │
│  ────────────────────────────────    │
│  v4                                  │
│  Saved 2 hours ago                   │
│  by operator@tackle.io               │
│  [Preview] [Restore]                 │
│  ────────────────────────────────    │
│  v3                                  │
│  Saved yesterday                     │
│  by admin@tackle.io                  │
│  [Preview] [Restore]                 │
│  ────────────────────────────────    │
│  v2                                  │
│  Saved 3 days ago                    │
│  by operator@tackle.io               │
│  [Preview] [Restore]                 │
│  ────────────────────────────────    │
│  v1 (original)                       │
│  Created 1 week ago                  │
│  by admin@tackle.io                  │
│  [Preview] [Restore]                 │
└──────────────────────────────────────┘
```

- Source: `GET /api/v1/email-templates/{id}/versions`.
- Each version entry shows: version number, relative timestamp, author email.
- The current version is labeled "(current)" and has no action buttons.
- The original version is labeled "(original)".

### 6.3 Preview a Version

Clicking "Preview" on a past version opens a modal displaying the rendered HTML body of that version. The modal includes the version's subject line and metadata at the top. The modal has a single "Close" button.

### 6.4 Restore a Version

Clicking "Restore" opens a confirmation modal:
- Title: "Restore Version"
- Body: "This will replace the current template content with version {N}. A new version will be created with the current content before restoring. Continue?"
- Buttons: [Cancel] (secondary), [Restore] (primary).
- On confirm: The frontend saves the current state as a new version, then applies the selected version's content to the editor. Both the WYSIWYG editor and metadata fields update. The version list refreshes to show the new version entry.

---

## 7. Test Email Send Flow

### 7.1 Access

The "Send Test" action is available from:
- The kebab menu on the template list view.
- The slide-over quick preview panel footer.
- A "Send Test" button in the template editor page header.

### 7.2 Test Email Modal

```
┌──────────────────────────────────────────┐
│  Send Test Email                   [X]   │
│  ────────────────────────────────────    │
│  Recipient Email*                        │
│  [____________________________________]  │
│                                          │
│  ☐ Include attachments                   │
│                                          │
│  ⓘ The test email will use sample data   │
│    for all template variables. Tracking   │
│    links will be disabled.               │
│                                          │
│  [Cancel]                  [Send Test]   │
└──────────────────────────────────────────┘
```

### 7.3 Fields

| Field | Type | Required | Default | Notes |
|-------|------|----------|---------|-------|
| Recipient Email | Email input | Yes | Pre-populated with the current user's email address | Must be a valid email format. |
| Include attachments | Checkbox | No | Checked | When checked, the test email includes all template attachments. |

### 7.4 Send Behavior

1. User fills in the recipient email and clicks "Send Test".
2. The button shows a loading spinner and the text changes to "Sending...". The button is disabled.
3. Frontend calls `POST /api/v1/email-templates/{id}/send-test` with `{ recipient_email, include_attachments }`.
4. **On success (200)**: The modal closes and a success toast is shown: "Test email sent to {email}."
5. **On error (4xx/5xx)**: The modal remains open. An inline error message appears above the buttons: the error message from the API response (e.g., "SMTP configuration error" or "Invalid recipient address").

### 7.5 Unsaved Content Warning

If "Send Test" is triggered from the editor and there are unsaved changes, a warning line appears in the modal: "You have unsaved changes. The test email will use the last saved version." The warning uses `--warning` color.

---

## 8. Template Creation from Campaign Workspace

### 8.1 Context

When building a campaign, the user selects an email template. The campaign workspace provides two paths:
1. **Select existing template**: A dropdown/searchable select listing available templates.
2. **Create new template**: A button that opens the template editor.

### 8.2 "Create New Template" Flow

When the user clicks "Create New Template" from within the campaign workspace:

1. The application navigates to `/email-templates/new?campaign_id={id}&return_to=campaign`.
2. The template editor page renders normally but with two differences:
   - The "Back" link reads "← Back to Campaign" and navigates to the campaign workspace (not the template list).
   - On successful save, after the save completes, the user is prompted: "Template saved. Return to campaign?" with [Stay Here] and [Return to Campaign] buttons in a toast-like banner at the top of the editor.
3. If the user clicks "Return to Campaign", they are navigated back to the campaign workspace. The newly created template is automatically selected in the campaign's template picker.

### 8.3 "Edit Template" from Campaign

If the user clicks "Edit" on the currently selected template within the campaign workspace, the same flow applies: the editor opens with a "← Back to Campaign" link and the return-to-campaign prompt on save.

### 8.4 Quick Select Existing Template

The campaign workspace template selector is a searchable dropdown:
- Displays template name and subject line as the two-line option label.
- Search filters by name and subject.
- A "Preview" icon button next to the selected template opens the slide-over quick preview (same component as the list view slide-over, section 1.7).
- A "Create New" option is always pinned at the top of the dropdown list.

---

## 9. Auto-Save Behavior

### 9.1 Auto-Save Cycle

- Auto-save triggers every 30 seconds if there are unsaved changes (dirty state).
- Auto-save calls `PUT /api/v1/email-templates/{id}` with all current field values.
- For new templates (create mode), the first auto-save performs a `POST /api/v1/email-templates` to create the template, and subsequent auto-saves use `PUT` with the returned ID. The URL updates from `/email-templates/new` to `/email-templates/{id}/edit` via `history.replaceState` (no navigation event).

### 9.2 Auto-Save Indicator

A subtle text indicator in the page header communicates auto-save status:

| State | Display | Style |
|-------|---------|-------|
| Saved / clean | "All changes saved" | `--text-muted`, no icon |
| Unsaved changes pending | "Unsaved changes" | `--warning`, dot icon |
| Saving in progress | "Saving..." | `--text-muted`, spinner icon |
| Auto-save just completed | "Auto-saved {relative time}" | `--text-muted`, check icon. The relative time updates live ("just now" -> "10s ago" -> "30s ago"). |
| Save failed | "Auto-save failed. [Retry]" | `--danger`, warning icon. Retry link triggers an immediate save attempt. |

### 9.3 Unsaved Changes Guard

If the user attempts to navigate away from the editor (via the Back link, browser back button, or any sidebar navigation) while there are unsaved changes:

- A confirmation modal appears:
  - Title: "Unsaved Changes"
  - Body: "You have unsaved changes that will be lost. Do you want to save before leaving?"
  - Buttons: [Discard] (secondary), [Save & Leave] (primary), [Cancel] (ghost/text).
- "Save & Leave" saves the template and then navigates.
- "Discard" navigates without saving.
- "Cancel" closes the modal and stays on the editor.
- The browser `beforeunload` event is also hooked to show the native browser prompt if the user closes the tab or refreshes.

### 9.4 Conflict Detection

If the API returns a 409 Conflict on save (another user has modified the template since the last fetch), the editor shows an inline banner:

- "This template has been modified by another user. [Load Latest] or [Force Save]."
- "Load Latest" fetches the latest version and reloads the editor (current changes are lost — the user is warned).
- "Force Save" overwrites with the current content.

---

## 10. Error States & Validation

### 10.1 Template Validation

When the user clicks "Save" (not "Save Draft"), the frontend calls `POST /api/v1/email-templates/{id}/validate` before saving. The validation endpoint checks:

- Required fields are present (name, subject, sender_name, sender_email, html_body).
- Template variables used in the body/subject are valid (no typos like `{{target.fist_name}}`).
- HTML body is not empty.
- The `{{tracking.url}}` variable is present in the body (warning, not blocking — the template is still saveable but the user is warned that the phishing link is missing).

### 10.2 Validation Error Display

Validation errors are displayed in two places simultaneously:

1. **Inline on fields**: Each field with an error shows a red border (`--danger`) and an error message below the field in `--danger` text. The first field with an error is scrolled into view and focused.
2. **Summary banner**: A collapsible error banner appears at the top of the editor area:

```
┌──────────────────────────────────────────────────────────────┐
│  ⚠ 3 issues found                                    [▾]    │
│  • Template name is required                                 │
│  • Subject line is required                                  │
│  • Unknown variable: {{target.fist_name}}                    │
└──────────────────────────────────────────────────────────────┘
```

### 10.3 Validation Warnings

Warnings are non-blocking. They are displayed in the summary banner with a yellow warning icon instead of red:

```
┌──────────────────────────────────────────────────────────────┐
│  ⓘ 1 warning                                         [▾]    │
│  • No phishing URL ({{tracking.url}}) found in template      │
│    body. Recipients will not have a clickable link.          │
└──────────────────────────────────────────────────────────────┘
```

The user can proceed to save despite warnings. Warnings are dismissed when the user clicks "Save Anyway" or adds the missing element.

### 10.4 Field-Level Validation (Client-Side)

Client-side validation runs on blur for each field:

| Field | Validation | Error Message |
|-------|-----------|---------------|
| Template Name | Non-empty, max 150 chars | "Template name is required." / "Template name must be under 150 characters." |
| Subject Line | Non-empty, max 255 chars | "Subject line is required." / "Subject line must be under 255 characters." |
| Sender Name | Non-empty, max 100 chars | "Sender name is required." |
| Sender Email | Non-empty, valid email format | "Sender email is required." / "Please enter a valid email address." |
| Category | Must have a selection | "Please select a category." |
| Tags | Each tag: 1-50 chars, max 10 tags | "Tag is too long (max 50 characters)." / "Maximum 10 tags allowed." |

### 10.5 API Error Handling

| HTTP Status | Scenario | Frontend Behavior |
|-------------|----------|-------------------|
| 400 | Validation errors from server | Display errors inline per field (mapped by field name in the response body). |
| 401 | Session expired | Redirect to login page. |
| 403 | Insufficient permissions | Toast: "You do not have permission to perform this action." |
| 404 | Template not found (deleted by another user) | Full-page error: "Template not found. It may have been deleted." with a "Back to Templates" link. |
| 409 | Conflict (concurrent edit) | Conflict banner (see section 9.4). |
| 413 | Attachment too large | Inline error on the attachment panel: "File exceeds the 12MB size limit." |
| 422 | Unprocessable entity | Display the `message` field from the response as a toast error. |
| 500 | Server error | Toast: "Something went wrong. Please try again." with a "Retry" action on the toast. |

### 10.6 Network Error States

- **Offline**: If the browser goes offline, the auto-save indicator changes to "You are offline. Changes will be saved when reconnected." in `--warning`. Auto-save attempts are queued and retried when connectivity is restored (detected via the `online` event).
- **Timeout**: If a save request times out (10 seconds), the auto-save indicator shows "Save timed out. [Retry]." in `--danger`.

---

## 11. Keyboard Shortcuts

The following keyboard shortcuts are active when the template editor is focused:

| Shortcut | Action |
|----------|--------|
| Ctrl+S | Save the template (triggers validation + save). |
| Ctrl+Shift+S | Save as draft (no validation). |
| Ctrl+B | Bold (WYSIWYG mode). |
| Ctrl+I | Italic (WYSIWYG mode). |
| Ctrl+U | Underline (WYSIWYG mode). |
| Ctrl+K | Insert/edit link (WYSIWYG mode). |
| Ctrl+Z | Undo. |
| Ctrl+Shift+Z | Redo. |
| Ctrl+Shift+P | Toggle preview panel visibility. |
| Escape | Close any open modal, dropdown, or slide-over. |

### 11.1 Shortcut Conflicts

- Ctrl+S overrides the browser's native save-page behavior when the editor is mounted.
- All shortcuts use `preventDefault()` to avoid browser defaults.
- Shortcuts are disabled when a modal is open (focus is trapped within the modal).

---

## 12. Export Template

### 12.1 Trigger

Export is available from the kebab menu on the list view and can also be accessed from the editor page header (as a secondary action in a dropdown).

### 12.2 Behavior

1. Frontend calls `POST /api/v1/email-templates/{id}/export`.
2. The API returns a downloadable file (content-disposition: attachment).
3. The browser initiates a file download. No modal or confirmation is needed.
4. On error, a toast is shown: "Export failed. Please try again."

---

## 13. Permissions

Template actions are gated by the user's role and permissions:

| Action | Required Permission |
|--------|-------------------|
| View template list | `email_templates:read` |
| View template detail | `email_templates:read` |
| Create template | `email_templates:create` |
| Edit template | `email_templates:update` (own templates) or `email_templates:update_any` (shared/others') |
| Delete template | `email_templates:delete` (own) or `email_templates:delete_any` |
| Duplicate template | `email_templates:create` |
| Send test email | `email_templates:test` |
| Export template | `email_templates:read` |

UI elements for unauthorized actions are hidden entirely (not shown as disabled). If an unauthorized action is attempted via direct URL navigation, the application shows a 403 error page.
