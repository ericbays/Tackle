# 07 — Landing Page Builder

This document specifies the Landing Page Builder: a full-screen, visual, drag-and-drop web page builder that enables operators to create phishing landing pages for Tackle campaigns. The builder is the centerpiece of the platform and the most complex frontend feature. It provides Webflow-inspired editing with an iframe-based canvas, icon toolbar with flyout panels, a right-side property editor, a bottom code editor panel, multi-page management, responsive breakpoints, a form builder with field categorization, template galleries, HTML import, and a full preview mode.

This document is the definitive reference for implementation. Every interaction, edge case, animation timing, keyboard shortcut, and API call is specified below.

---

## 1. Builder Layout

### 1.1 Full-Screen Experience

The landing page builder launches as a full-screen experience. The application shell sidebar is hidden automatically when the builder route activates. The sidebar remains collapsible via `Ctrl+/` (the global sidebar toggle), but defaults to hidden. The top application bar is replaced with a builder-specific toolbar.

When the user navigates away from the builder (via breadcrumb, back button, or route change), the sidebar returns to its previous state.

### 1.2 Layout Structure

```
┌─────────────────────────────────────────────────────────────────────────────┐
│  CANVAS TOOLBAR (top bar)                                                   │
│  [Back] Landing Page Name ▾  │ Undo Redo │ Zoom │ Device │ Preview │ Save  │
├──┬──────────────────────────────────────────────────────────────┬───────────┤
│  │                                                              │           │
│I │                                                              │ PROPERTY  │
│C │                                                              │ EDITOR    │
│O │                    CANVAS (iframe)                           │           │
│N │                                                              │ [Content] │
│  │              ┌─────────────────────┐                         │ [Style]   │
│T │              │                     │                         │ [Advanced]│
│O │              │   Page Content      │                         │           │
│O │              │                     │                         │  ... form │
│L │              │                     │                         │  fields   │
│B │              │                     │                         │  here ... │
│A │              └─────────────────────┘                         │           │
│R │                                                              │           │
│  │  [Breadcrumb: Page > Container > Form > Input]               │           │
│  ├──────────────────────────────────────────────────────────────┤           │
│  │  CODE EDITOR (bottom panel, resizable)                       │           │
│  │  [Page CSS] [Page JS] [Component CSS] [Component JS]         │           │
│  │  ┌──────────────────────────────────────────────────────┐    │           │
│  │  │ .login-form {                                        │    │           │
│  │  │   max-width: 400px;                                  │    │           │
│  │  │ }                                                    │    │           │
│  │  └──────────────────────────────────────────────────────┘    │           │
└──┴──────────────────────────────────────────────────────────────┴───────────┘
```

### 1.3 Panel Dimensions

| Panel | Default Width/Height | Min | Max | Resizable |
|-------|---------------------|-----|-----|-----------|
| Left icon toolbar | 48px | 48px | 48px | No |
| Left flyout panel | 280px | 240px | 400px | Yes (drag right edge) |
| Right property editor | 320px | 280px | 480px | Yes (drag left edge) |
| Bottom code editor | 200px (collapsed: 0) | 120px | 50% viewport | Yes (drag top edge) |
| Canvas | Fills remaining space | — | — | Responsive |

### 1.4 Panel Interactions

- The left flyout panel opens when an icon toolbar button is clicked and closes when the same button is clicked again, or when a different flyout is activated (only one flyout visible at a time).
- The right property editor is always visible when a component is selected. When nothing is selected, it shows page-level properties.
- The bottom code editor is collapsed by default. It opens via the `</>` button in the canvas toolbar or via `Ctrl+Shift+E`. It can be collapsed by dragging its top edge to the bottom, or by pressing `Ctrl+Shift+E` again.
- All resize handles show a `col-resize` or `row-resize` cursor on hover. Resize operations apply a 1px `--border-primary` divider line that brightens to `--accent` (#4a7ab5) during drag.

### 1.5 Z-Index Layering

| Layer | Z-Index | Element |
|-------|---------|---------|
| 1 | 1 | Canvas iframe |
| 2 | 10 | Left flyout panel |
| 3 | 20 | Property editor |
| 4 | 30 | Bottom code editor |
| 5 | 40 | Canvas toolbar (top bar) |
| 6 | 50 | Floating action toolbar (on selection) |
| 7 | 100 | Modals (form builder, template gallery) |
| 8 | 110 | Toasts / notifications |
| 9 | 200 | Command palette (Ctrl+K) |

---

## 2. Left Icon Toolbar

### 2.1 Structure

The left icon toolbar is a 48px-wide vertical strip pinned to the left edge of the builder viewport. It uses `--bg-secondary` background with `--border-primary` right border. Icons are 24px, centered in 40px hit targets, stacked vertically with 4px gaps.

```
┌──────┐
│  +   │  Components
│  ☰   │  Layers
│  ◻◻  │  Pages
│  🎨  │  Templates
│      │
│      │
│      │
│      │
│  ⚙   │  Settings
└──────┘
```

### 2.2 Toolbar Buttons

| Position | Icon | Label (tooltip) | Flyout | Shortcut |
|----------|------|-----------------|--------|----------|
| 1 | Plus-square | Components | Component palette | `A` |
| 2 | Layers | Layers | Layer tree | `L` |
| 3 | File-stack | Pages | Page manager | `P` |
| 4 | Layout-template | Templates | Template gallery | `T` |
| Bottom | Settings | Settings | Page settings | `S` |

### 2.3 Active State

The active toolbar button receives:
- Left 2px accent border (`--accent`)
- Icon color changes from `--text-muted` to `--accent`
- Background: `--bg-tertiary`
- Transition: `150ms ease-out`

### 2.4 Flyout Panel Behavior

- Flyout slides out from the right edge of the icon toolbar, pushing the canvas area.
- Animation: `200ms ease-out` width transition from 0 to 280px.
- Flyout has its own header with title and close (X) button.
- Clicking the active toolbar icon again closes the flyout with reverse animation.
- Opening a different flyout crossfades content within the same panel width (no close-then-open flicker).

---

## 3. Components Flyout

### 3.1 Layout

```
┌─────────────────────────────┐
│  Components            [X]  │
├─────────────────────────────┤
│  ┌───────────────────────┐  │
│  │ Search components...  │  │
│  └───────────────────────┘  │
├─────────────────────────────┤
│  ▼ Layout                   │
│    ┌─────┐ ┌─────┐ ┌─────┐ │
│    │ ▭▭  │ │ ⊞   │ │ ▭▭▭ │ │
│    │Cont.│ │Grid │ │Cols │ │
│    └─────┘ └─────┘ └─────┘ │
│    ┌─────┐ ┌─────┐         │
│    │ ─── │ │ │   │         │
│    │Space│ │Divid│         │
│    └─────┘ └─────┘         │
│  ▼ Navigation               │
│    ┌─────┐ ┌─────┐ ┌─────┐ │
│    │ ≡   │ │ ▭   │ │ > > │ │
│    │Navbr│ │Footr│ │Bread│ │
│    └─────┘ └─────┘ └─────┘ │
│  ▼ Content                  │
│    ...                      │
│  ▼ Forms                    │
│    ...                      │
│  ▼ Interactive              │
│    ...                      │
│  ▼ Branding                 │
│    ...                      │
└─────────────────────────────┘
```

### 3.2 Component Categories and Items

**Layout**
| Component | Description | Default Children | Drop Constraints |
|-----------|-------------|-----------------|------------------|
| Container | Generic block wrapper | Empty | Accepts all |
| Grid | CSS Grid layout | Empty | Accepts all |
| Columns | Flexbox column layout | 2 Column children | Accepts Column only |
| Column | Single column within Columns | Empty | Must be inside Columns |
| Spacer | Vertical spacing element | — | Accepts none (leaf) |
| Divider | Horizontal rule | — | Accepts none (leaf) |

**Navigation**
| Component | Description | Default Children | Drop Constraints |
|-----------|-------------|-----------------|------------------|
| Navbar | Top navigation bar | Logo + nav links | Accepts limited set |
| Footer | Page footer | Text + links | Accepts limited set |
| Breadcrumb | Breadcrumb trail | 3 items | Accepts breadcrumb items |

**Content**
| Component | Description | Default Children | Drop Constraints |
|-----------|-------------|-----------------|------------------|
| Heading | H1-H6 text | "Heading" text | Accepts none (leaf) |
| Paragraph | Body text block | "Lorem ipsum..." | Accepts none (leaf) |
| Rich Text | Multi-paragraph formatted text | Sample content | Accepts none (leaf) |
| Image | Image element with src/alt | Placeholder | Accepts none (leaf) |
| Video Embed | iframe video embed | YouTube placeholder | Accepts none (leaf) |

**Forms**
| Component | Description | Default Children | Drop Constraints |
|-----------|-------------|-----------------|------------------|
| Form Container | `<form>` wrapper | Empty | Accepts form fields + layout |
| Text Input | `<input type="text">` | — | Recommended inside Form |
| Email Input | `<input type="email">` | — | Recommended inside Form |
| Password Input | `<input type="password">` | — | Recommended inside Form |
| Select | `<select>` dropdown | 3 options | Recommended inside Form |
| Checkbox | `<input type="checkbox">` | — | Recommended inside Form |
| Radio Group | Radio button group | 3 options | Recommended inside Form |
| Submit Button | `<button type="submit">` | "Submit" text | Must be inside Form |

**Interactive**
| Component | Description | Default Children | Drop Constraints |
|-----------|-------------|-----------------|------------------|
| Button | Clickable button | "Click me" text | Accepts none (leaf) |
| Link | Anchor element | "Link text" | Accepts none (leaf) |
| Tab Group | Tabbed content panels | 3 tabs | Accepts Tab Panels |
| Accordion | Collapsible sections | 3 sections | Accepts Accordion Items |

**Branding**
| Component | Description | Default Children | Drop Constraints |
|-----------|-------------|-----------------|------------------|
| Logo Placeholder | Image with logo role | Generic logo | Accepts none (leaf) |
| Hero Banner | Full-width hero section | Heading + subtext + CTA | Accepts all |

### 3.3 Search Behavior

- The search input at the top filters components across all categories in real time (debounced 150ms).
- Matching is case-insensitive against component name and description.
- When a search is active, category headers are hidden and results display as a flat filtered list.
- When search is cleared, the categorized view restores.
- If no results match, display: "No components match '[query]'" in `--text-muted`.

### 3.4 Drag from Palette

- Components can be dragged from the palette directly onto the canvas.
- On drag start: the palette item gets `opacity: 0.5`, a ghost image (component name + icon) follows the cursor.
- While dragging over the canvas, insertion indicators appear (see Section 6).
- On drop: the component is instantiated at the drop position with default properties.
- On drop outside a valid target: the drag cancels with a `200ms` snap-back animation on the ghost.

---

## 4. Layers Flyout

### 4.1 Layout

```
┌─────────────────────────────┐
│  Layers                [X]  │
├─────────────────────────────┤
│  ┌───────────────────────┐  │
│  │ Search layers...      │  │
│  └───────────────────────┘  │
├─────────────────────────────┤
│  ▾ Page 1                   │
│    ▾ Container              │
│      ▾ Navbar         👁 🔒 │
│        Logo           👁    │
│        Nav Links      👁    │
│      ▾ Form Container 👁    │
│        Email Input    👁    │
│        Password Input 👁    │
│        Submit Button  👁    │
│      ▾ Footer         👁    │
│        Copyright      👁    │
└─────────────────────────────┘
```

### 4.2 Tree Behavior

- Each node displays: indentation (16px per level) + expand/collapse chevron (if has children) + component type icon (12px) + label + action icons.
- Action icons (right side, visible on hover): eye (visibility toggle), lock (lock toggle).
- Labels default to the component type name. Double-click a label to rename it (inline edit with `Enter` to confirm, `Escape` to cancel).
- Renamed labels are stored in the component's `label` property and shown in place of the type name throughout the builder.

### 4.3 Selection Sync

- Clicking a layer node selects the corresponding component on the canvas (scrolling the canvas to bring it into view if necessary).
- Selecting a component on the canvas scrolls the layers panel to reveal and highlight the corresponding node.
- Multi-select via `Shift+click` (range) or `Ctrl+click` (toggle) mirrors canvas multi-select behavior.
- The selected node receives `--bg-tertiary` background with `--accent` left border (2px).

### 4.4 Drag Reorder in Layers

- Nodes can be dragged to reorder within the tree.
- Drop indicators: a thin blue line between nodes for sibling insertion, or a blue highlight on a node for child insertion (dropping into a container).
- Drop constraints are enforced (same rules as canvas drag-and-drop). Invalid drop targets show a red "no-drop" cursor.
- Drag animation: `100ms ease-out` for center-out reshuffling of sibling nodes.
- Reordering in layers immediately updates the canvas.

### 4.5 Visibility Toggle

- Clicking the eye icon toggles component visibility.
- Hidden components: rendered in canvas with `opacity: 0.3` and dashed outline, not rendered in preview/publish.
- Eye icon changes to eye-off (struck-through) when hidden.
- Hiding a parent hides all children visually but preserves their individual visibility states.

### 4.6 Virtual Scrolling

- For pages with more than 100 layer nodes, the layers panel uses virtual scrolling (only rendering visible nodes + 10-node buffer above and below).
- Expand/collapse state is preserved across virtual scroll recycling.

---

## 5. Pages Flyout

### 5.1 Layout

```
┌─────────────────────────────┐
│  Pages              [+ Add] │
├─────────────────────────────┤
│                             │
│  ┌─────────────────────┐    │
│  │  ● Page 1: Login    │ ⋮  │
│  │    (currently open)  │    │
│  └─────────────────────┘    │
│  ┌─────────────────────┐    │
│  │  ○ Page 2: Loading  │ ⋮  │
│  └─────────────────────┘    │
│  ┌─────────────────────┐    │
│  │  ○ Page 3: Error    │ ⋮  │
│  └─────────────────────┘    │
│                             │
│ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ │
│  PAGE FLOW                  │
│  ┌────────┐   ┌────────┐   │
│  │ Login  │──>│Loading │   │
│  └────────┘   └────────┘   │
│                  │          │
│              ┌────────┐    │
│              │ Error  │    │
│              └────────┘    │
└─────────────────────────────┘
```

### 5.2 Page List

- Each page is a card showing: active indicator (filled/empty circle), page name, kebab menu.
- The currently active page (shown in canvas) has a filled blue circle and `--bg-tertiary` background.
- Click a page card to switch the canvas to that page. Switching is instant (the component tree for each page is held in memory).
- Pages can be reordered via drag (same drag mechanics as layers).

### 5.3 Page Actions (Kebab Menu)

| Action | Behavior |
|--------|----------|
| Rename | Inline edit on the page name |
| Duplicate | Creates a copy with " (Copy)" suffix |
| Set as Start Page | Marks this page as the entry point (first page shown to targets) |
| Delete | Confirmation dialog. Cannot delete the last remaining page. |

### 5.4 Add Page

- The [+ Add] button creates a new blank page named "Page N" (where N is the next sequential number).
- The new page becomes active immediately.
- Maximum 20 pages per landing page project. After 20, the add button is disabled with tooltip: "Maximum 20 pages reached."

### 5.5 Page Flow Diagram

- Below the page list, a mini flow diagram visualizes how pages connect.
- Each page is a small rounded rectangle node. Arrows between nodes represent navigation actions (form submit, button click, redirect).
- The start page has a small green left-border indicator.
- Hovering a flow arrow shows a tooltip: "Form submit on 'Login' navigates to 'Loading'".
- The diagram is read-only in the Pages flyout. Navigation connections are configured in the Form Builder (Section 10) and the property editor's Advanced tab on buttons/links.
- If no flow connections are configured, the diagram shows pages as unconnected nodes with a muted label: "Configure page flow in form settings."

---

## 6. Drag and Drop

### 6.1 Drag Sources

Components can be dragged from three sources:
1. **Component palette** (flyout) — creates a new component instance.
2. **Canvas** — moves an existing component to a new position.
3. **Layers panel** — moves an existing component (same as canvas drag, but initiated from tree).

### 6.2 Insertion Indicators

When dragging over the canvas, the builder renders insertion indicators to show the exact drop position:

```
  ┌──────────────────────────────┐
  │        Heading               │
  ├──────────────────────────────┤  <── Thin blue line (2px, --accent)
  │        Paragraph             │
  │                              │
  └──────────────────────────────┘
```

- **Between siblings**: A horizontal blue line (2px height, `--accent` color) spans the full width of the parent container, positioned between two sibling components.
- **Inside empty container**: The entire container gets a dashed blue border and a centered label: "Drop here".
- **Before first child**: Blue line at the top inside the container.
- **After last child**: Blue line at the bottom inside the container.

### 6.3 Ghost Image

- While dragging, a semi-transparent ghost (40% opacity) of the component or a simplified representation (name + icon badge, 120x36px) follows the cursor offset 12px right and 12px below.
- The ghost is rendered outside the iframe in the builder's overlay layer.

### 6.4 Center-Out Reshuffling

When a component is dragged between siblings, existing siblings animate to make room:
- Animation: `100ms ease-out` (using `--ease-spring: cubic-bezier(0.34, 1.56, 0.64, 1)` for the final snap).
- Siblings above the insertion point remain in place. Siblings below shift downward by the estimated height of the dragged component.
- Once dropped, all siblings animate to their final positions.

### 6.5 Drop Constraints

Each component type defines which children it accepts and which parents it requires:

| Component | Allowed Parents | Allowed Children |
|-----------|----------------|-----------------|
| Column | Columns only | Any |
| Submit Button | Form Container only | None |
| Tab Panel | Tab Group only | Any |
| Accordion Item | Accordion only | Any |
| Form fields | Any (recommended: Form) | None |
| Container, Grid | Any | Any |
| Navbar, Footer | Root or Container | Limited set |

When a drag target violates constraints:
- The insertion indicator does not appear.
- The cursor shows `not-allowed`.
- A subtle red flash (100ms) on the container border indicates the rejection.

### 6.6 Keyboard Drag Alternative

Full keyboard-based drag-and-drop for accessibility:

| Key | Action |
|-----|--------|
| `Tab` / `Shift+Tab` | Navigate between components in document order |
| `Space` or `Enter` | Pick up the focused component (enters drag mode) |
| `Arrow Up` / `Arrow Down` | Move the picked-up component up/down among siblings |
| `Arrow Left` | Move out of current parent (unwrap one level) |
| `Arrow Right` | Move into the next sibling container (wrap into) |
| `Space` or `Enter` | Drop the component at current position |
| `Escape` | Cancel the drag, return component to original position |

While in keyboard drag mode:
- The component has a pulsing blue outline.
- An ARIA live region announces: "Moving [component name]. Use arrows to reposition. Press Space to drop."
- The insertion indicator (blue line) is visible and follows keyboard navigation.

---

## 7. Canvas

### 7.1 Iframe Rendering

The canvas renders the landing page inside an `<iframe>` element for complete style isolation. The builder's CSS, theme variables, and component styles do not leak into the page content, and page CSS does not affect the builder UI.

**Iframe setup:**
- The iframe `src` is set to `about:blank`. Content is injected via `iframe.contentDocument.write()` or by setting `iframe.srcdoc`.
- The iframe document includes a minimal doctype, a `<head>` with the page's CSS (both theme CSS and user-authored CSS), and a `<body>` containing the rendered component tree.
- A lightweight JavaScript bridge is injected into the iframe to handle:
  - Click events (forwarded to builder for component selection)
  - Hover events (forwarded for highlight outlines)
  - Drag events (forwarded for drag-and-drop)
  - Scroll position (synced for overlay alignment)
  - Component bounding box queries (for selection outlines, insertion indicators)
  - Resize observer (to detect content size changes for auto-scroll)

### 7.2 Rendering Pipeline

1. The component tree (in-memory JSON) is the single source of truth.
2. On any change (property edit, drag-drop, style change), the component tree is diffed against the previous state.
3. Only affected DOM nodes in the iframe are updated (not a full re-render).
4. CSS changes are applied by updating a `<style>` tag in the iframe `<head>`.
5. The rendering pipeline targets < 16ms per update (60fps) for interactive operations.

### 7.3 Canvas Background

- The canvas area surrounding the page frame uses `--bg-primary` (the darkest background).
- The page frame itself sits centered in the canvas area with a subtle `box-shadow: 0 0 20px rgba(0,0,0,0.3)` to create a "paper on desk" effect.
- Page frame background defaults to `#ffffff` (white) unless overridden by page CSS.

### 7.4 Zoom and Pan

**Zoom:**
- CSS `transform: scale()` applied to the iframe container (not the iframe content).
- Zoom range: 25% to 200%.
- Zoom controls in canvas toolbar: Fit (fits page to viewport), Zoom In (+25%), Zoom Out (-25%), percentage dropdown (25, 50, 75, 100, 125, 150, 200).
- `Ctrl+Mouse Wheel` zooms in/out in 10% increments, centered on cursor position.
- `Ctrl+0` resets zoom to 100%.
- `Ctrl+1` fits to viewport width.
- Zoom transitions animate over `200ms ease-out`.

**Pan:**
- When zoomed in beyond the viewport, `Space+Drag` pans the canvas (cursor changes to grab/grabbing).
- Mouse wheel scrolls vertically. `Shift+Mouse Wheel` scrolls horizontally.
- Pan is also available via two-finger trackpad gesture.

### 7.5 Device Preview

Device preview resizes the page frame within the canvas:

| Device | Frame Width | Toolbar Icon |
|--------|------------|-------------|
| Desktop | 1440px | Monitor |
| Tablet | 768px | Tablet |
| Mobile | 375px | Smartphone |

- Switching device preview: click the device button in the canvas toolbar. Active device button receives `--accent` color.
- The page frame animates to the new width over `300ms ease-out`, centered in the canvas.
- Height is always auto (content height).
- Switching devices also activates the corresponding responsive breakpoint (see Section 16).

### 7.6 Selection Overlay

Selection overlays are rendered in a transparent `<div>` positioned absolutely over the iframe, matching the iframe's coordinate space (accounting for zoom and scroll). This overlay div captures no pointer events except on the overlay elements themselves.

---

## 8. Component Selection and Manipulation

### 8.1 Hover Highlight

- When the cursor hovers over a component in the canvas (while not dragging), a highlight appears:
  - Blue outline: 1px dashed `--accent` around the component's bounding box.
  - Component name label: a small pill positioned at the top-left corner of the bounding box, showing the component type (or custom label). Background: `--accent`, text: white, font-size: 10px, padding: 1px 6px, border-radius: 2px.
- The hover highlight disappears immediately when the cursor leaves the component.
- Hover highlights do not appear on the currently selected component (it already has a selection outline).

### 8.2 Click to Select

- Clicking a component selects it:
  - Solid blue outline: 2px solid `--accent` around the bounding box.
  - Component name pill at top-left (same style as hover, but solid background).
  - Floating action toolbar appears above the component (see Section 8.5).
  - The right property editor populates with the selected component's properties.
  - The corresponding layer node highlights in the layers panel.

### 8.3 Double-Click to Drill In

- Double-clicking a container component drills into it, selecting the first child instead.
- Subsequent clicks within the container now target children directly.
- This enables selecting deeply nested components without multiple clicks.
- Double-clicking a text component (Heading, Paragraph, Rich Text) enters inline text editing mode (see Section 8.6).

### 8.4 Breadcrumb Navigation

Below the canvas (above the bottom code editor), a breadcrumb bar shows the hierarchy path to the currently selected component:

```
Page 1  >  Container  >  Form Container  >  Email Input
```

- Each segment is clickable — clicking selects that ancestor component.
- The breadcrumb updates instantly on selection change.
- Background: `--bg-secondary`, height: 28px, font-size: 12px.
- Segments use `--text-muted` color; the final (active) segment uses `--text-primary`.

### 8.5 Floating Action Toolbar

When a component is selected, a floating toolbar appears 8px above the component's top edge, horizontally centered:

```
      ┌──────────────────────────────────┐
      │  ⬆ ⬇  │  ↕  │  ⧉  │  🗑  │  ⋮  │
      └──────────────────────────────────┘
      ┌──────────────────────────────────┐
      │      Selected Component          │
      │                                  │
      └──────────────────────────────────┘
```

| Button | Icon | Action | Shortcut |
|--------|------|--------|----------|
| Move Up | Arrow up | Move component before previous sibling | `Ctrl+Arrow Up` |
| Move Down | Arrow down | Move component after next sibling | `Ctrl+Arrow Down` |
| Drag Handle | Grip dots | Initiate drag (click-and-hold) | — |
| Duplicate | Copy | Duplicate component (insert after) | `Ctrl+D` |
| Delete | Trash | Delete component with confirmation for containers with children | `Delete` or `Backspace` |
| More | Kebab | Additional actions menu | — |

**More menu items:**
- Copy (`Ctrl+C`)
- Cut (`Ctrl+X`)
- Paste Inside (`Ctrl+V` — pastes as last child)
- Paste Before
- Paste After
- Wrap in Container
- Unwrap (replaces parent with children)
- Copy Styles
- Paste Styles

**Toolbar positioning edge cases:**
- If the component is near the top of the viewport, the toolbar renders below the component instead.
- If the component is wider than the toolbar, the toolbar left-aligns with the component.
- The toolbar follows the component during scroll (repositioned via `requestAnimationFrame`).

### 8.6 Inline Text Editing

- Double-clicking a text component (Heading, Paragraph, Button label) enters inline edit mode.
- The component's text becomes `contenteditable` inside the iframe.
- A minimal floating text toolbar appears above with: Bold, Italic, Link, Clear Formatting.
- Clicking outside the text or pressing `Escape` exits inline edit mode and saves the text.
- Text changes are immediately reflected in the component tree and property editor.

### 8.7 Multi-Select

- `Shift+Click` adds/removes components from the selection (toggle).
- `Ctrl+A` selects all siblings within the current parent.
- When multiple components are selected:
  - Each selected component shows a solid blue outline.
  - The floating action toolbar shows only: Move Up, Move Down, Duplicate, Delete (actions that apply to groups).
  - The property editor shows "N components selected" with common properties (properties shared by all selected types). Changing a common property applies to all selected components.
  - Drag moves all selected components together.

### 8.8 Deselection

- Clicking empty space on the canvas (not on any component) deselects all.
- Pressing `Escape` deselects the current selection (if not in drag mode or text edit mode).

---

## 9. Property Editor (Right Panel)

### 9.1 Structure

The property editor is a 320px panel on the right side of the builder. It has three tabs at the top and scrollable content below.

```
┌─────────────────────────┐
│  [Content] [Style] [Adv]│
├─────────────────────────┤
│                         │
│  ▼ General              │
│    Label: [Email      ] │
│    Placeholder:         │
│    [Enter your email  ] │
│    Name: [email       ] │
│    Type: [email     ▾]  │
│    Required: [✓]        │
│                         │
│  ▼ Validation           │
│    Pattern: [         ] │
│    Min Length: [      ] │
│    Max Length: [      ] │
│    Error Msg: [       ] │
│                         │
│  ▼ Autocomplete         │
│    Autocomplete:        │
│    [email          ▾]   │
│                         │
└─────────────────────────┘
```

### 9.2 Tab Navigation

| Tab | Shortcut | Content |
|-----|----------|---------|
| Content | `Alt+1` | Component-specific content properties |
| Style | `Alt+2` | Visual styling controls |
| Advanced | `Alt+3` | CSS overrides, responsive, custom code |

- Active tab: underlined with `--accent`, text `--text-primary`.
- Inactive tabs: text `--text-muted`.
- Tab switch transition: `150ms` crossfade on content area.

### 9.3 Accordion Sections

Within each tab, properties are organized into collapsible accordion sections:
- Section header: 32px height, `--text-secondary` label, chevron icon.
- Click header to expand/collapse. Collapse animation: `200ms ease-out` height.
- Multiple sections can be open simultaneously.
- Sections default to expanded on first view; collapsed state persists during the session.

### 9.4 Content Tab — Component-Specific Properties

The Content tab shows different fields depending on the selected component type. Below are the field specifications for each component:

**Heading**
| Section | Field | Control | Default |
|---------|-------|---------|---------|
| General | Text | Text input | "Heading" |
| General | Level | Dropdown (H1-H6) | H2 |
| General | ID | Text input | auto-generated |

**Paragraph**
| Section | Field | Control | Default |
|---------|-------|---------|---------|
| General | Text | Textarea (4 rows) | "Lorem ipsum..." |
| General | ID | Text input | auto-generated |

**Image**
| Section | Field | Control | Default |
|---------|-------|---------|---------|
| General | Source URL | Text input + upload button | Placeholder |
| General | Alt Text | Text input | "" |
| General | Width | Number + unit dropdown (px/%) | auto |
| General | Height | Number + unit dropdown (px/%) | auto |
| General | Object Fit | Dropdown (cover/contain/fill/none) | cover |
| Link | Link URL | Text input | "" |
| Link | Open in New Tab | Toggle | Off |

**Text Input / Email Input / Password Input**
| Section | Field | Control | Default |
|---------|-------|---------|---------|
| General | Label | Text input | "Label" |
| General | Placeholder | Text input | "" |
| General | Name | Text input (auto from label) | "" |
| General | Required | Toggle | Off |
| General | Autocomplete | Dropdown | "off" |
| Validation | Pattern | Text input (regex) | "" |
| Validation | Min Length | Number input | — |
| Validation | Max Length | Number input | — |
| Validation | Error Message | Text input | "" |

**Select**
| Section | Field | Control | Default |
|---------|-------|---------|---------|
| General | Label | Text input | "Select" |
| General | Name | Text input | "" |
| General | Required | Toggle | Off |
| Options | Option list | Repeatable row (value + label) with add/remove/reorder | 3 defaults |

**Checkbox / Radio Group**
| Section | Field | Control | Default |
|---------|-------|---------|---------|
| General | Label | Text input | "Option" |
| General | Name | Text input | "" |
| General | Required | Toggle | Off |
| Options (Radio) | Option list | Repeatable row (value + label) | 3 defaults |

**Button / Submit Button**
| Section | Field | Control | Default |
|---------|-------|---------|---------|
| General | Text | Text input | "Submit" / "Click me" |
| General | Type | Dropdown (submit/button/reset) | submit/button |
| General | Disabled | Toggle | Off |
| Action | On Click | Dropdown (none/navigate to page/open URL/custom JS) | none |
| Action | Target Page | Dropdown (list of pages) | — (shown if navigate) |
| Action | Target URL | Text input | — (shown if open URL) |
| Action | New Tab | Toggle | — (shown if open URL) |

**Form Container**
| Section | Field | Control | Default |
|---------|-------|---------|---------|
| General | Form Name | Text input | "Form" |
| General | Method | Dropdown (POST/GET) | POST |
| Form Settings | [Configure Form] | Button — opens Form Builder modal | — |

**Link**
| Section | Field | Control | Default |
|---------|-------|---------|---------|
| General | Text | Text input | "Link text" |
| General | URL | Text input | "#" |
| General | Target | Dropdown (_self/_blank) | _self |
| Action | On Click | Dropdown (navigate to page/open URL/custom JS) | open URL |
| Action | Target Page | Dropdown (list of pages) | — |

**Container / Grid**
| Section | Field | Control | Default |
|---------|-------|---------|---------|
| General | ID | Text input | auto |
| General | Tag | Dropdown (div/section/article/aside/main/header/footer) | div |
| Layout (Grid) | Columns | Number | 2 |
| Layout (Grid) | Rows | Number | auto |
| Layout (Grid) | Gap | Number + unit | 16px |

### 9.5 Style Tab

The Style tab provides visual controls for CSS styling. It is identical in structure for all component types, though some sections are context-sensitive (e.g., typography only appears on text elements, grid layout only on Grid components).

```
┌─────────────────────────┐
│  [Content] [Style] [Adv]│
├─────────────────────────┤
│                         │
│  ▼ Layout               │
│    Display: [block   ▾] │
│    Position: [static ▾] │
│    Overflow: [visible▾] │
│    Width:  [auto      ] │
│    Height: [auto      ] │
│                         │
│  ▼ Spacing              │
│    ┌─── margin ────────┐│
│    │  ┌── padding ──┐  ││
│    │  │   [16]       │  ││
│    │[0]│  content  │[0]│││
│    │  │   [16]       │  ││
│    │  └──────────────┘  ││
│    │    [0]              ││
│    └────────────────────┘│
│                         │
│  ▼ Typography           │
│    Font: [Inter      ▾] │
│    Weight: [400      ▾] │
│    Size: [16px       ▾] │
│    Line H: [1.5      ▾] │
│    Color: [■ #333333 ] │
│    Align: [L] [C] [R] [J]│
│    Transform: [none  ▾] │
│    Decoration: [none ▾] │
│                         │
│  ▼ Backgrounds          │
│    Type: [color      ▾] │
│    Color: [■ #ffffff  ] │
│    Image: [          📁] │
│    Size: [cover      ▾] │
│    Position: [center ▾] │
│                         │
│  ▼ Borders              │
│    Width: [0px        ] │
│    Style: [none      ▾] │
│    Color: [■ #cccccc  ] │
│    Radius: [0px       ] │
│    ┌ ─ ─ ┐              │
│    │ TL TR│  Per-corner │
│    │ BL BR│  toggleable │
│    └ ─ ─ ┘              │
│                         │
│  ▼ Effects              │
│    Opacity: [────●───] 1│
│    Box Shadow:          │
│    [+ Add shadow]       │
│    Cursor: [pointer  ▾] │
│    Transition: [all 0.2s]│
│                         │
└─────────────────────────┘
```

### 9.6 Style Controls — Detailed Specifications

**Box Model Spacing Diagram:**
- Interactive visual diagram showing nested rectangles for margin (outer) and padding (inner).
- Each edge has an editable number input (top, right, bottom, left).
- Click a number to edit directly. Tab cycles through values clockwise (top -> right -> bottom -> left).
- Drag a number vertically to scrub values (increment/decrement by 1px per pixel of mouse movement; hold Shift for 10px increments).
- The diagram updates in real-time to show relative proportions.
- A "link" toggle chains all four values (editing one changes all).
- Units supported: px, %, em, rem, auto. Selectable via suffix dropdown.

**Color Picker:**
- Clicking a color swatch opens a popover color picker.
- The picker includes: hue/saturation gradient square, hue slider, opacity slider, hex input, RGB inputs, a row of recently used colors (last 8), and preset swatches from the design system.
- Color changes apply live to the canvas (no "apply" button needed).
- Closing the popover commits the color. `Escape` reverts to the previous color.

**Typography Controls:**
- Font Family: searchable dropdown listing system fonts + Google Fonts subset (configured in page settings). Default: "Inter".
- Font Weight: dropdown (100-900 in increments of 100, plus named weights). Only available weights for the selected font are shown.
- Font Size: number input with unit dropdown (px, em, rem). Drag to scrub.
- Line Height: number input (unitless multiplier or with unit). Default: 1.5.
- Letter Spacing: number input (px/em). Default: normal.
- Text Color: color swatch + picker.
- Text Align: segmented button group (Left, Center, Right, Justify).
- Text Transform: dropdown (none, uppercase, lowercase, capitalize).
- Text Decoration: dropdown (none, underline, line-through).

**Background Controls:**
- Type selector: dropdown (None, Color, Gradient, Image).
- Color: color swatch + picker.
- Gradient: type (linear/radial), angle (number input), stops (color + position list with add/remove).
- Image: URL text input + file upload button. Opens file picker for JPG/PNG/SVG/WebP (max 5MB).
- Background Size: dropdown (cover, contain, auto, custom).
- Background Position: dropdown (center, top, bottom, left, right, custom XY).
- Background Repeat: dropdown (no-repeat, repeat, repeat-x, repeat-y).

**Border Controls:**
- Width: number input (px).
- Style: dropdown (none, solid, dashed, dotted, double).
- Color: color swatch + picker.
- Radius: single number input for uniform radius OR per-corner inputs (toggled by a "per-corner" button showing a small four-corner diagram).
- Each corner (top-left, top-right, bottom-right, bottom-left) gets its own number input when per-corner mode is active.

**Effects Controls:**
- Opacity: slider (0 to 1) with number input.
- Box Shadow: list of shadow entries. Each entry has: X offset, Y offset, blur, spread, color, inset toggle. [+ Add shadow] appends a new entry. Each entry has a delete button.
- Cursor: dropdown (auto, pointer, text, grab, not-allowed, etc.).
- Transition: text input for shorthand CSS transition value (e.g., "all 0.2s ease").

### 9.7 Advanced Tab

```
┌─────────────────────────┐
│  [Content] [Style] [Adv]│
├─────────────────────────┤
│                         │
│  ▼ Responsive Overrides │
│    Breakpoint:          │
│    [Desktop▾][Tab▾][Mob▾]│
│    Show/Hide per BP:    │
│    Desktop [✓]          │
│    Tablet  [✓]          │
│    Mobile  [ ]          │
│                         │
│  ▼ Custom CSS           │
│    (Mini code editor)   │
│    ┌───────────────────┐│
│    │ .this {           ││
│    │   /* custom CSS */││
│    │ }                 ││
│    └───────────────────┘│
│                         │
│  ▼ Custom JS            │
│    (Mini code editor)   │
│    ┌───────────────────┐│
│    │ // runs on mount  ││
│    │                   ││
│    └───────────────────┘│
│                         │
│  ▼ Attributes           │
│    data-* attributes:   │
│    [+ Add attribute]    │
│    Key:   [data-track ] │
│    Value: [login-form ] │
│                         │
│  ▼ Accessibility        │
│    ARIA Role: [        ]│
│    ARIA Label: [       ]│
│    Tab Index: [        ]│
│                         │
└─────────────────────────┘
```

**Responsive Overrides:**
- Per-breakpoint visibility toggles (show/hide at each breakpoint).
- Per-breakpoint style overrides are edited by switching the device preview in the canvas toolbar — the Style tab then shows overrides for that breakpoint (indicated by a colored breakpoint badge next to "Style").
- Style values that differ from the desktop base are shown with a small dot indicator next to the field.

**Custom CSS:**
- A mini Monaco editor (6 lines default height, expandable) for per-component CSS.
- The selector `.this` is a magic placeholder that resolves to the component's unique scoped class.
- CSS is validated on blur; errors are shown as red underlines with tooltip messages.

**Custom JS:**
- A mini Monaco editor for per-component JavaScript.
- The script runs when the component mounts in the rendered page.
- Available globals: `this.element` (the DOM element), `this.page` (page API).
- JS is not executed in the builder canvas (only in preview and published pages).

**Custom Attributes:**
- Repeatable key-value rows for adding `data-*` attributes, `id`, or any HTML attribute.
- [+ Add attribute] appends a new row.
- Each row has: key text input, value text input, delete button.

---

## 10. Form Builder

### 10.1 Entry Point

The Form Builder is accessed by selecting a Form Container component and clicking the [Configure Form] button in the Content tab of the property editor. This opens a full-screen slide-over panel (from the right, 80% viewport width) or a modal overlay.

### 10.2 Form Builder Layout

```
┌─────────────────────────────────────────────────────────────────┐
│  Configure Form: "Login Form"                           [Done] │
├────────────────────┬────────────────────────────────────────────┤
│                    │                                            │
│  FIELD LIST        │  FIELD DETAIL / SETTINGS                  │
│                    │                                            │
│  ┌──────────────┐  │  ┌──────────────────────────────────────┐  │
│  │ ✉ Email      │◄─│──│ Field: Email                        │  │
│  │   identity   │  │  │ Category: [identity          ▾]     │  │
│  ├──────────────┤  │  │ Name Attribute: email                │  │
│  │ 🔒 Password  │  │  │ Label: Email Address                 │  │
│  │   sensitive   │  │  │ Required: [✓]                       │  │
│  ├──────────────┤  │  │ Validation: email                    │  │
│  │ 🔑 MFA Code  │  │  │                                      │  │
│  │   mfa         │  │  │ ▼ Capture Settings                  │  │
│  ├──────────────┤  │  │   Log Keystrokes: [ ]                │  │
│  │ 🏷 Token     │  │  │   Mask in Logs: [ ]                  │  │
│  │   hidden      │  │  │                                      │  │
│  └──────────────┘  │  └──────────────────────────────────────┘  │
│                    │                                            │
│  [+ Hidden Field]  │  ┌──────────────────────────────────────┐  │
│                    │  │ ▼ Post-Capture Action                │  │
├────────────────────┤  │   Action: [Redirect with delay  ▾]  │  │
│                    │  │   Redirect URL: [https://...]        │  │
│  ▼ POST-CAPTURE   │  │   Delay (ms): [3000]                 │  │
│    ACTION          │  │   Loading Message: [Verifying...]    │  │
│  [Redirect w/ ▾]   │  └──────────────────────────────────────┘  │
│                    │                                            │
│  ▼ MULTI-STEP     │  ┌──────────────────────────────────────┐  │
│  Step 1: Login     │  │ ▼ Multi-Step Flow                    │  │
│  Step 2: MFA       │  │   Step 1: Login → Step 2: MFA        │  │
│  [+ Add Step]      │  │   Step 2: MFA → Page: Loading        │  │
│                    │  │                                      │  │
└────────────────────┴──└──────────────────────────────────────┘──┘
```

### 10.3 Field List

- Auto-populated from form field components that exist within the Form Container on the canvas.
- Each field shows: icon (based on field type), field name/label, category badge (colored).
- Click a field to show its detail panel on the right.
- Hidden fields appear at the bottom of the list with a distinct "hidden" badge.

### 10.4 Field Categories

Each form field is assigned exactly one category that determines how captured data is processed:

| Category | Badge Color | Description | Examples |
|----------|-------------|-------------|----------|
| `identity` | Blue | Identifies the target (used for tracking) | Email, username, employee ID |
| `sensitive` | Red | Sensitive credential data | Password, PIN, SSN, credit card |
| `mfa` | Orange | Multi-factor authentication codes | OTP, SMS code, authenticator code |
| `custom` | Gray | Custom data fields | Comments, survey answers |
| `hidden` | Dark gray | Invisible to user, auto-populated | Tracking token, timestamp, user-agent, IP |

Category assignment is via dropdown in the field detail panel. The category determines:
- How the captured value is stored (encrypted for `sensitive`, hashed for `identity`).
- How the value appears in campaign reports (masked for `sensitive`, shown for `identity`).
- Which analytics are generated (credential reuse detection for `sensitive`).

### 10.5 Hidden Fields

Hidden fields are not visible on the page but are submitted with the form:

| Hidden Field Type | Auto-Populated Value |
|-------------------|---------------------|
| Tracking Token | Campaign-generated unique token per target |
| Timestamp | ISO 8601 submission timestamp |
| User-Agent | Browser user-agent string |
| Client IP | Target's IP address (captured server-side) |
| Custom | Operator-defined static value |

- [+ Hidden Field] in the field list opens a dropdown to select the hidden field type.
- Custom hidden fields require a name and a static value.
- Hidden fields are rendered as `<input type="hidden">` in the page.

### 10.6 Post-Capture Actions

After a form submission is captured, the landing page executes a post-capture action:

| Action | Description | Configuration Fields |
|--------|-------------|---------------------|
| Redirect | Immediately redirect to a URL | Redirect URL |
| Display Page | Navigate to another page in the landing page project | Target Page (dropdown) |
| Redirect with Delay | Show a loading/processing message, then redirect | Redirect URL, Delay (ms), Loading Message |
| Replay Submission | Forward the form data to the real service | Target URL, Method (POST/GET), Forward Headers toggle |
| No Action | Do nothing (page stays on current state) | — |

- Only one post-capture action per form.
- "Replay Submission" is the most complex: it re-submits the captured credentials to the legitimate service so the target doesn't notice the interception. A warning banner displays: "Replay submission forwards credentials to the real service. Use with caution."
- The post-capture action configuration is stored in `capture_config.post_capture_action` in the landing page data model.

### 10.7 Multi-Step Forms

Some phishing scenarios require multi-step forms (e.g., email on step 1, password on step 2, MFA on step 3):

**Configuration:**
- The multi-step section of the Form Builder lists ordered steps.
- Each step corresponds to a page in the landing page project.
- [+ Add Step] adds a new step, prompting to select or create a page.
- Steps can be reordered via drag.
- Each step shows: step number, page name, arrow to next step.

**Runtime Behavior:**
- Step 1 form submits -> data is captured -> page navigates to Step 2 page.
- Step 2 form submits -> data is captured (appended to same session) -> page navigates to Step 3 or post-capture action.
- Session data is keyed by the tracking token hidden field.
- If a target abandons mid-flow, partial data is still captured and flagged as "incomplete" in reports.

**Flow Visualization:**
- A small horizontal flow diagram shows: `[Step 1] -> [Step 2] -> [Step 3] -> [Post-Capture]`.
- Each node is clickable to configure that step.

---

## 11. Page Flow and Navigation Configuration

### 11.1 Purpose

The page flow system allows operators to define how targets navigate between pages in a multi-page landing page. This is critical for phishing simulations that mimic multi-step login flows (credential entry -> MFA -> success/error).

### 11.2 Navigation Triggers

Navigation between pages can be triggered by:

| Trigger | Configuration Location | Description |
|---------|----------------------|-------------|
| Form Submit | Form Builder (Section 10) | Form submission navigates to next step/page |
| Button Click | Property Editor > Content > Action | Button click navigates to a page |
| Link Click | Property Editor > Content > Action | Link click navigates to a page |
| Timer | Property Editor > Advanced > Custom JS | Page auto-redirects after N seconds |
| Custom JS | Code Editor / Advanced tab | Programmatic navigation via `page.navigateTo('page-id')` |

### 11.3 Page Navigation API (Client-Side)

Within the rendered landing page, a lightweight JavaScript API is available:

```javascript
// Navigate to another page by name or ID
TacklePage.navigateTo('page-2');

// Navigate with delay
TacklePage.navigateAfter('page-2', 3000);

// Get current page info
TacklePage.current(); // { id, name, index }
```

This API is injected into the page at build time. It handles page transitions within the single-page application structure.

### 11.4 Visual Flow in Pages Flyout

The page flow diagram in the Pages flyout (Section 5.5) visualizes all configured navigation connections:

```
  ┌──────────┐       ┌──────────┐       ┌──────────┐
  │  Login   │──────>│  MFA     │──────>│  Success  │
  │  (start) │ form  │          │ form  │           │
  └──────────┘ submit└──────────┘ submit└──────────┘
                                    │
                                    │ error
                                    ▼
                              ┌──────────┐
                              │  Error   │
                              └──────────┘
```

- Arrows are labeled with the trigger type (form submit, button click, etc.).
- The diagram auto-layouts using a simple left-to-right flow for sequential steps and downward branches for error/alternate paths.
- The diagram is view-only in the Pages flyout. Connections are configured in the Form Builder and property editors.
- If the diagram becomes too complex (more than 8 nodes), it switches to a simplified list view with indentation.

---

## 12. Multi-Page Management

### 12.1 Data Model

Each landing page project contains an ordered array of pages:

```json
{
  "id": "lp_abc123",
  "name": "Microsoft O365 Login",
  "pages": [
    {
      "id": "page_1",
      "name": "Login",
      "is_start": true,
      "component_tree": { ... },
      "css": "...",
      "js": "..."
    },
    {
      "id": "page_2",
      "name": "MFA Verification",
      "is_start": false,
      "component_tree": { ... },
      "css": "...",
      "js": "..."
    }
  ],
  "capture_config": { ... },
  "global_css": "...",
  "global_js": "..."
}
```

### 12.2 Page Switching

- Clicking a page in the Pages flyout switches the canvas to render that page's component tree.
- The switch is instant (no loading state) because all page trees are held in memory.
- The layers panel updates to show the new page's tree.
- The property editor deselects (shows page-level properties).
- Undo/redo history is per-project (not per-page) — undoing a change on Page 2 that was made while on Page 1 will switch back to Page 1 and undo.

### 12.3 Page Settings

Clicking the Settings icon in the left toolbar (or selecting no component) shows page-level settings in the property editor:

| Setting | Control | Description |
|---------|---------|-------------|
| Page Title | Text input | `<title>` tag content |
| Meta Description | Textarea | Meta description tag |
| Favicon | URL input + upload | Favicon URL |
| Custom Fonts | Multi-select | Google Fonts to include |
| Global CSS | Link to code editor | Opens bottom code editor on Global CSS tab |
| Global JS | Link to code editor | Opens bottom code editor on Global JS tab |
| Background Color | Color picker | Page body background |
| Max Width | Number + unit | Content max-width (default: none) |
| Center Content | Toggle | Centers content horizontally |

---

## 13. Canvas Toolbar

### 13.1 Layout

```
┌─────────────────────────────────────────────────────────────────────────────┐
│ [←] Microsoft O365 Login ▾  │ ↩ ↪ │ [-] 100% [+] [Fit]│ 🖥 📱 📱│ ▶ │💾 Saved│
└─────────────────────────────────────────────────────────────────────────────┘
```

### 13.2 Toolbar Elements (Left to Right)

| Element | Description | Behavior |
|---------|-------------|----------|
| Back arrow [←] | Returns to landing pages list or campaign workspace | Prompts to save if unsaved changes |
| Landing page name | Editable name (click to edit inline) | Updates via API on blur |
| Dropdown chevron ▾ | Opens menu: Rename, Duplicate, Export, Delete | Standard kebab actions |
| Undo ↩ | Undo last action | Disabled when at start of history. Shortcut: `Ctrl+Z` |
| Redo ↪ | Redo next action | Disabled when at end of history. Shortcut: `Ctrl+Shift+Z` |
| Zoom Out [-] | Decrease zoom by 25% | Min: 25% |
| Zoom Level | Dropdown showing current zoom percentage | Click for preset values |
| Zoom In [+] | Increase zoom by 25% | Max: 200% |
| Fit [Fit] | Zoom to fit page in viewport | Shortcut: `Ctrl+1` |
| Device: Desktop | 1440px frame | Active state: `--accent` icon color |
| Device: Tablet | 768px frame | Active state: `--accent` icon color |
| Device: Mobile | 375px frame | Active state: `--accent` icon color |
| Preview ▶ | Enter preview mode | Shortcut: `Ctrl+P` |
| Code </> | Toggle bottom code editor | Shortcut: `Ctrl+Shift+E` |
| Save indicator | Shows save state | See Section 20 |

### 13.3 Save Status Indicator

| State | Display | Behavior |
|-------|---------|----------|
| Saved | "Saved" in `--text-muted` + check icon | Default state after successful save |
| Saving | Spinner + "Saving..." | During API call |
| Unsaved | Orange dot + "Unsaved" | Changes pending auto-save |
| Error | Red dot + "Save failed" | Click to retry. Tooltip shows error. |

---

## 14. Code Editor (Bottom Panel)

### 14.1 Structure

The code editor is a resizable panel that slides up from the bottom of the canvas area, similar to browser DevTools.

```
┌─────────────────────────────────────────────────────────────────────┐
│  [Page CSS] [Page JS] [Component CSS] [Component JS] [Global CSS]  │  ▼
├─────────────────────────────────────────────────────────────────────┤
│  1  .login-form {                                                   │
│  2    max-width: 400px;                                             │
│  3    margin: 0 auto;                                               │
│  4    padding: 2rem;                                                │
│  5  }                                                               │
│  6                                                                  │
│  7  .login-form input {                                             │
│  8    width: 100%;                                                  │
│  9    margin-bottom: 1rem;                                          │
│ 10  }                                                               │
└─────────────────────────────────────────────────────────────────────┘
```

### 14.2 Editor Tabs

| Tab | Scope | Description |
|-----|-------|-------------|
| Page CSS | Current page | CSS that applies to the current page only |
| Page JS | Current page | JavaScript that runs on the current page only |
| Component CSS | Selected component | CSS scoped to the selected component (`.this` selector) |
| Component JS | Selected component | JavaScript for the selected component |
| Global CSS | All pages | CSS that applies to all pages in the project |
| Global JS | All pages | JavaScript that runs on all pages |

- Component CSS/JS tabs are only enabled when a component is selected. Otherwise, they show "Select a component to edit its CSS/JS" in `--text-muted`.
- Switching between Page/Global tabs does not require a component selection.

### 14.3 Monaco Editor Configuration

- Language: CSS for CSS tabs, JavaScript for JS tabs.
- Theme: Dark theme matching the builder's `--bg-secondary` palette.
- Features enabled: syntax highlighting, autocomplete, bracket matching, error indicators (red underlines), minimap (off by default, toggleable), word wrap (on by default).
- Font: `JetBrains Mono` or `Fira Code`, 13px, ligatures enabled.
- Line numbers: shown.
- Tab size: 2 spaces.

### 14.4 Live Preview

- CSS changes in any tab are immediately reflected in the canvas iframe (no save/apply step needed).
- Changes are applied by updating the corresponding `<style>` tag in the iframe document.
- Debounce: 300ms after last keystroke before applying CSS to avoid layout thrashing.
- JS changes are NOT live-applied (would require re-executing scripts). A subtle banner at the top of JS tabs reads: "JavaScript changes are applied in Preview mode and published pages."

### 14.5 Resize Behavior

- The top edge of the code editor panel has a drag handle (3px hit area, `row-resize` cursor).
- Dragging up increases the panel height; dragging down decreases it.
- Double-clicking the drag handle toggles between collapsed (0px) and default height (200px).
- Panel state (open/closed, height) persists in `localStorage` across sessions.

---

## 15. Preview Mode

### 15.1 Entry and Exit

- Enter: click Preview button in toolbar or press `Ctrl+P`.
- Exit: click "Exit Preview" floating button or press `Ctrl+P` or `Escape`.

### 15.2 Layout

Preview mode hides all builder chrome (left toolbar, property editor, code editor, canvas toolbar, breadcrumbs, selection overlays). The iframe expands to fill the entire viewport.

```
┌─────────────────────────────────────────────────────────────────┐
│                                                                 │
│                    ┌────────────────────┐                       │
│                    │                    │     [Exit Preview]    │
│                    │   Page Content     │     [Device: 🖥 📱 📱]│
│                    │   (interactive)    │                       │
│                    │                    │                       │
│                    │   [Login Form]     │                       │
│                    │   Email: [_______] │                       │
│                    │   Pass:  [_______] │                       │
│                    │   [Submit]         │                       │
│                    │                    │                       │
│                    └────────────────────┘                       │
│                                                                 │
└─────────────────────────────────────────────────────────────────┘
```

### 15.3 Preview Behavior

- The page is fully interactive: form fields accept input, buttons trigger actions, page navigation works, custom JS executes.
- Form submissions in preview mode do NOT capture data. Instead, a toast appears: "Preview: Form submitted successfully" with the submitted field names listed.
- The replay submission action shows a toast: "Preview: Replay submission would forward data to [URL]".
- JavaScript errors in preview mode are caught and shown in a collapsible error console at the bottom of the preview (red bar with error count, click to expand).

### 15.4 Device Simulation in Preview

- A floating device switcher in the bottom-right corner allows switching between Desktop, Tablet, and Mobile previews.
- The page frame resizes with animation (`300ms ease-out`).
- The surrounding area uses `--bg-primary` background.

---

## 16. Responsive Design and Breakpoints

### 16.1 Desktop-First Cascade

The builder uses a desktop-first approach:
- **Desktop (1440px)** is the base breakpoint. All styles are authored here first.
- **Tablet (768px)** inherits all desktop styles and allows overrides.
- **Mobile (375px)** inherits all tablet styles (which include desktop styles) and allows further overrides.

Overrides cascade downward only: a font-size change on Tablet applies to Tablet and Mobile, but not Desktop. A font-size change on Mobile applies only to Mobile.

### 16.2 Breakpoint Switching

- Switching the device preview in the canvas toolbar activates the corresponding breakpoint for editing.
- A colored badge appears in the property editor's Style tab header:
  - Desktop: no badge (default).
  - Tablet: blue badge "768px".
  - Mobile: green badge "375px".
- Style changes made while a breakpoint is active are stored as overrides for that breakpoint, not as changes to the desktop base styles.

### 16.3 Override Indicators

- In the Style tab, any property that has a breakpoint override shows a small colored dot next to the field label.
- Hovering the dot shows a tooltip: "Overridden at 768px" or "Overridden at 768px and 375px".
- Right-clicking the dot offers: "Reset to desktop value" (removes the override).

### 16.4 Structure vs. Style

- **Structure** (component hierarchy and ordering) is GLOBAL across all breakpoints. You cannot add, remove, or reorder components per breakpoint.
- **Visibility** can be toggled per breakpoint (Advanced tab > Responsive Overrides > Show/Hide per BP). This allows hiding a desktop navigation on mobile and showing a hamburger menu instead.
- **Style** (all CSS properties) can be overridden per breakpoint.

### 16.5 CSS Output

The generated CSS uses `@media` queries with `max-width`:

```css
/* Desktop (base - no media query) */
.component-abc { font-size: 24px; }

/* Tablet */
@media (max-width: 768px) {
  .component-abc { font-size: 20px; }
}

/* Mobile */
@media (max-width: 375px) {
  .component-abc { font-size: 16px; }
}
```

---

## 17. Template Gallery and Insertion

### 17.1 Template Gallery on New Project

When creating a new landing page (from the landing pages list or from a campaign workspace), a full-screen modal presents the template gallery:

```
┌─────────────────────────────────────────────────────────────────────┐
│  Choose a Template                                        [X]      │
├─────────────────────────────────────────────────────────────────────┤
│  ┌─────────────────────┐                                           │
│  │ Search templates...  │  [All] [Office] [Google] [Banking] [Corp]│
│  └─────────────────────┘                                           │
├─────────────────────────────────────────────────────────────────────┤
│                                                                     │
│  ┌──────────────┐  ┌──────────────┐  ┌──────────────┐              │
│  │ ┌──────────┐ │  │ ┌──────────┐ │  │ ┌──────────┐ │              │
│  │ │  Preview  │ │  │ │  Preview  │ │  │ │  Preview  │ │              │
│  │ │  Image    │ │  │ │  Image    │ │  │ │  Image    │ │              │
│  │ └──────────┘ │  │ └──────────┘ │  │ └──────────┘ │              │
│  │ O365 Login   │  │ Gmail Login  │  │ AWS Console  │              │
│  │ Microsoft    │  │ Google       │  │ Cloud        │              │
│  │ [Use]        │  │ [Use]        │  │ [Use]        │              │
│  └──────────────┘  └──────────────┘  └──────────────┘              │
│                                                                     │
│  ┌──────────────┐  ┌──────────────┐  ┌──────────────┐              │
│  │ ┌──────────┐ │  │              │  │              │              │
│  │ │  Blank   │ │  │  Bank Login  │  │  VPN Portal  │              │
│  │ │  Page    │ │  │  Banking     │  │  Corporate   │              │
│  │ └──────────┘ │  │              │  │              │              │
│  │ Blank        │  │              │  │              │              │
│  │ Start Fresh  │  │              │  │              │              │
│  │ [Use]        │  │              │  │              │              │
│  └──────────────┘  └──────────────┘  └──────────────┘              │
│                                                                     │
└─────────────────────────────────────────────────────────────────────┘
```

### 17.2 Template Categories

| Category | Description | Examples |
|----------|-------------|---------|
| Microsoft Office | O365, Outlook, SharePoint login clones | O365 Login, SharePoint Invite, Teams Join |
| Google Workspace | Gmail, Drive, Calendar clones | Gmail Login, Drive Share, Meet Join |
| Banking | Online banking portals | Generic Bank Login, Wire Transfer Confirmation |
| Corporate | Internal portal templates | VPN Login, Intranet Portal, IT Reset Password |
| Cloud Service | AWS, Azure, Salesforce | AWS Console, Azure AD, Salesforce Login |
| Social Engineering | Package delivery, invoice, survey | Delivery Notification, Invoice Payment, Survey |
| Blank | Empty starting point | Blank Page |

### 17.3 Template Selection Flow

1. User clicks a template card.
2. A preview modal shows a larger screenshot and template details (name, description, included pages, form fields).
3. User clicks [Use This Template].
4. A new landing page project is created with the template's component tree, pages, and pre-configured form fields.
5. The builder opens with the new project.

### 17.4 Template Insertion in Builder

The Templates flyout in the left toolbar allows inserting template sections into the current page (not replacing the entire project):

- The flyout shows a categorized list of section templates (headers, login forms, footers, hero sections, error pages).
- Clicking a section template inserts it at the bottom of the current page's component tree.
- The user can then drag it to the desired position.

### 17.5 Save as Template

From the canvas toolbar dropdown menu, "Save as Template" saves the current landing page as a reusable template:
- Prompts for: template name, category, description, preview screenshot (auto-generated from canvas).
- API: `POST /api/v1/landing-pages/templates` with the full `definition_json`.

---

## 18. HTML/ZIP Import

### 18.1 Import Entry Point

From the landing pages list view, the [+ New Landing Page] button offers a dropdown:
- Blank Page
- From Template
- Import HTML/ZIP
- Clone from URL

### 18.2 HTML Import

- User uploads a `.html` file.
- The parser performs best-effort conversion of the HTML into the builder's component tree format:
  - `<div>` -> Container
  - `<h1>` through `<h6>` -> Heading (with correct level)
  - `<p>` -> Paragraph
  - `<img>` -> Image
  - `<form>` -> Form Container
  - `<input>` -> appropriate input component based on `type` attribute
  - `<button>` -> Button
  - `<a>` -> Link
  - `<nav>` -> Navbar (best effort)
  - `<footer>` -> Footer (best effort)
- Unrecognized elements become **HTML Block** components — generic containers that preserve the raw HTML verbatim and render it via `innerHTML`.
- CSS from `<style>` tags and inline `style` attributes is extracted and placed in the Page CSS code editor.
- `<script>` tags are extracted and placed in the Page JS code editor.
- External CSS/JS links are preserved as-is in the page head configuration.

### 18.3 ZIP Import

- User uploads a `.zip` file containing an `index.html` and optionally: CSS files, JS files, images, fonts.
- The parser extracts `index.html` and processes it per Section 18.2.
- Asset files (images, fonts) are uploaded to the project's asset storage and their URLs are rewritten in the component tree.
- Multiple HTML files in the ZIP are imported as separate pages.

### 18.4 Clone from URL

- User enters a URL.
- API: `POST /api/v1/landing-pages/clone-url` with `{ "url": "https://..." }`.
- The backend fetches the page, downloads assets, and returns a `definition_json`.
- The builder opens with the cloned page.
- A warning banner displays: "This page was cloned from [URL]. Review and customize before use."
- External resources (fonts, CDN scripts) are preserved as external links; images are downloaded and re-hosted.

### 18.5 Import Validation

After import, a validation report is shown:
- Total elements parsed: N
- Successfully converted: N
- Converted to HTML Block (raw): N
- Warnings: list of issues (e.g., "External script at cdn.example.com preserved as external link")
- User clicks [Open in Builder] to proceed.

---

## 19. Undo/Redo System

### 19.1 Implementation

- The undo system uses a linear history stack of state snapshots.
- Maximum stack depth: 50 entries.
- Each entry stores: a JSON patch (RFC 6902) representing the forward delta from the previous state.
- Undo applies the reverse patch. Redo applies the forward patch.

### 19.2 Tracked Operations

Every user-visible change creates a history entry:
- Component add, delete, move, duplicate.
- Property changes (content, style, advanced).
- Page add, delete, reorder.
- Layer rename, visibility toggle.
- Code editor changes (debounced — a single history entry is created 500ms after the user stops typing in the code editor).
- Drag-and-drop (one entry per completed drag).

### 19.3 Undo Grouping

Related changes are grouped into a single undo entry:
- Typing in a text input: characters are grouped into one entry (committed when the field loses focus or after 1 second of inactivity).
- Drag scrubbing a numeric value: one entry for the entire scrub gesture.
- Multi-select operations: one entry for the batch change.

### 19.4 History Limits

- When the stack exceeds 50 entries, the oldest entries are dropped (the earliest state becomes the new "beginning of time").
- Performing a new action after undoing clears the redo stack (standard linear undo behavior).

### 19.5 Controls

- `Ctrl+Z`: Undo.
- `Ctrl+Shift+Z` or `Ctrl+Y`: Redo.
- Toolbar undo/redo buttons with disabled state when at stack boundaries.
- Tooltip on undo button shows the action name: "Undo: Move Email Input".

---

## 20. Auto-Save

### 20.1 Trigger Conditions

Auto-save triggers on:
1. **Debounced change**: 3 seconds after the last user action (debounce timer resets on each action).
2. **Before navigation**: when the user attempts to leave the builder (route change, back button, browser close).
3. **Periodic**: every 60 seconds if there are unsaved changes.

### 20.2 Save Mechanism

- API: `PUT /api/v1/landing-pages/{id}` with the full `definition_json`, `capture_config`, page CSS/JS, and global CSS/JS.
- The save is non-blocking — the user can continue editing while the save is in progress.
- If a save fails (network error, server error), the save indicator shows "Save failed" with a retry option.
- If a save fails, the next auto-save retry occurs after 10 seconds.
- After 3 consecutive failures, a persistent error banner appears: "Unable to save. Check your connection." with a manual [Retry] button.

### 20.3 Conflict Resolution

- The API uses optimistic concurrency via an `etag` or `updated_at` timestamp.
- If a save returns `409 Conflict` (another session modified the same landing page), a dialog appears:
  - "This landing page was modified in another session. Your changes may conflict."
  - Options: [Overwrite with My Changes] [Reload Latest Version] [Download My Version as JSON]
- This scenario is rare since landing pages are typically edited by one user at a time.

### 20.4 Browser Close Protection

- When there are unsaved changes and the user attempts to close the browser tab, the `beforeunload` event triggers a confirmation dialog: "You have unsaved changes. Are you sure you want to leave?"

---

## 21. Landing Page Creation from Campaign Workspace

### 21.1 Entry Point

From the Campaign Workspace (document 05), the Landing Pages section allows:
- Selecting an existing landing page from a searchable dropdown.
- Creating a new landing page (opens the template gallery, then the builder).
- Editing the assigned landing page (opens the builder).

### 21.2 Campaign Context

When a landing page is opened from a campaign workspace:
- The back button [←] in the canvas toolbar returns to the campaign workspace (not the landing pages list).
- The canvas toolbar shows a campaign context badge: "Campaign: [campaign name]".
- The landing page is automatically linked to the campaign on creation.
- The tracking token hidden field is auto-configured with the campaign's tracking token format.

### 21.3 Landing Page Preview in Campaign

- The campaign workspace shows a thumbnail preview of the assigned landing page.
- A [Preview] button in the campaign workspace opens the landing page in Preview Mode without entering the full builder.

---

## 22. Error States, Edge Cases, and Performance

### 22.1 Error States

| Scenario | Behavior |
|----------|----------|
| Failed to load landing page | Full-page error: "Failed to load landing page. [Retry]" |
| Save conflict (409) | Conflict resolution dialog (Section 20.3) |
| Save failure (5xx/network) | Save indicator shows error + retry |
| Import parse failure | Error dialog with details + option to retry or cancel |
| Clone URL failure | Error dialog: "Failed to clone page from URL. The page may be protected or unavailable." |
| Component tree corruption | Detection on load: if `definition_json` fails schema validation, show "This landing page has errors. [Attempt Repair] [Open JSON Editor]" |
| Iframe rendering failure | Fallback message inside canvas: "Unable to render page. Check for JavaScript errors in your custom code." |
| Image upload failure | Toast: "Failed to upload image. [Retry]" with the image component showing a broken-image placeholder. |
| Monaco editor load failure | Fallback to plain `<textarea>` with a warning: "Code editor failed to load. Using basic editor." |

### 22.2 Edge Cases

| Case | Behavior |
|------|----------|
| Empty page (no components) | Canvas shows centered placeholder: "Drag components here or choose a template to get started" with a [Browse Templates] button |
| Very deep nesting (10+ levels) | Allowed up to 15 levels. Attempting to nest deeper shows a toast: "Maximum nesting depth reached (15 levels)." |
| Very large component tree (500+ nodes) | Performance warning toast: "This page has many components. Performance may be affected." Layers panel activates virtual scrolling. |
| Pasting HTML from clipboard | If `Ctrl+V` is pressed on the canvas (not in a text input), pasted HTML is parsed and inserted as an HTML Block component. |
| Dragging from external file manager | Image files dropped on the canvas create an Image component with the file uploaded to asset storage. |
| Screen width < 1024px | Builder shows a warning overlay: "The builder requires a screen width of at least 1024px. Please use a larger screen or increase your browser window size." |
| No form fields in form container | Form Builder shows: "Add form fields to the form container on the canvas, then return here to configure them." |

### 22.3 Performance Targets

| Metric | Target |
|--------|--------|
| Time to interactive (builder load) | < 2 seconds |
| Canvas re-render after property change | < 16ms (60fps) |
| Drag-and-drop frame rate | 60fps (insertion indicator updates) |
| Auto-save payload size | < 500KB (compressed) |
| Component palette search | < 50ms response |
| Page switch time | < 100ms |
| Maximum components per page | 500 (soft limit with warning) |
| Maximum pages per project | 20 |
| Maximum image asset size | 5MB per image |
| Maximum total project asset size | 50MB |

### 22.4 Memory Management

- Component trees are held in memory as plain JSON objects (not deep React component trees).
- Undo history uses JSON patches (not full state copies) to minimize memory usage.
- Images in the canvas use lazy loading — only images within the viewport (plus 500px buffer) are loaded.
- The Monaco editor instances are reused across tab switches (not destroyed/recreated).

---

## 23. Keyboard Shortcuts

### 23.1 Builder-Wide Shortcuts

| Shortcut | Action |
|----------|--------|
| `Ctrl+Z` | Undo |
| `Ctrl+Shift+Z` / `Ctrl+Y` | Redo |
| `Ctrl+S` | Force save (even if auto-save hasn't triggered) |
| `Ctrl+P` | Toggle preview mode |
| `Ctrl+/` | Toggle application sidebar |
| `Ctrl+K` | Open command palette |
| `Ctrl+Shift+E` | Toggle bottom code editor |
| `Escape` | Deselect / Exit preview / Cancel drag / Close flyout |
| `Ctrl+0` | Reset zoom to 100% |
| `Ctrl+1` | Fit to viewport |

### 23.2 Component Shortcuts

| Shortcut | Action |
|----------|--------|
| `Delete` / `Backspace` | Delete selected component(s) |
| `Ctrl+D` | Duplicate selected component(s) |
| `Ctrl+C` | Copy selected component(s) |
| `Ctrl+X` | Cut selected component(s) |
| `Ctrl+V` | Paste component(s) as last child of selected container, or after selected sibling |
| `Ctrl+Arrow Up` | Move selected component before previous sibling |
| `Ctrl+Arrow Down` | Move selected component after next sibling |
| `Ctrl+A` | Select all siblings |
| `Arrow Up` / `Arrow Down` | Select previous/next sibling |
| `Arrow Left` | Select parent |
| `Arrow Right` | Select first child |
| `Enter` | Enter inline edit mode (on text components) |

### 23.3 Flyout Shortcuts

| Shortcut | Action |
|----------|--------|
| `A` | Toggle Components flyout |
| `L` | Toggle Layers flyout |
| `P` | Toggle Pages flyout |
| `T` | Toggle Templates flyout |
| `S` | Toggle Settings flyout |

Note: Single-letter shortcuts are suppressed when a text input, textarea, or the code editor is focused.

### 23.4 Zoom Shortcuts

| Shortcut | Action |
|----------|--------|
| `Ctrl+Mouse Wheel Up` | Zoom in (10% increments) |
| `Ctrl+Mouse Wheel Down` | Zoom out (10% increments) |
| `Ctrl+=` / `Ctrl++` | Zoom in 25% |
| `Ctrl+-` | Zoom out 25% |
| `Space+Drag` | Pan canvas |
| `Shift+Mouse Wheel` | Horizontal scroll |

---

## 24. Backend API Reference

### 24.1 Landing Pages CRUD

| Method | Endpoint | Description |
|--------|----------|-------------|
| `GET` | `/api/v1/landing-pages` | List landing pages (paginated, filterable) |
| `POST` | `/api/v1/landing-pages` | Create new landing page |
| `GET` | `/api/v1/landing-pages/{id}` | Get landing page details + `definition_json` |
| `PUT` | `/api/v1/landing-pages/{id}` | Update landing page (full save) |
| `DELETE` | `/api/v1/landing-pages/{id}` | Delete landing page |
| `POST` | `/api/v1/landing-pages/{id}/duplicate` | Duplicate a landing page |

### 24.2 Preview and Import

| Method | Endpoint | Description |
|--------|----------|-------------|
| `POST` | `/api/v1/landing-pages/{id}/preview` | Render a preview (returns HTML) |
| `POST` | `/api/v1/landing-pages/import` | Import HTML or ZIP file (multipart) |
| `POST` | `/api/v1/landing-pages/clone-url` | Clone from URL |

### 24.3 Templates

| Method | Endpoint | Description |
|--------|----------|-------------|
| `GET` | `/api/v1/landing-pages/templates` | List templates (filterable by category) |
| `POST` | `/api/v1/landing-pages/templates` | Save current project as template |
| `GET` | `/api/v1/landing-pages/templates/{id}` | Get template details |
| `DELETE` | `/api/v1/landing-pages/templates/{id}` | Delete template |
| `GET` | `/api/v1/landing-pages/components` | List available components (registry) |
| `GET` | `/api/v1/landing-pages/themes` | List theme presets |
| `GET` | `/api/v1/landing-pages/js-snippets` | List reusable JS snippets |

### 24.4 Builds and Deployment

| Method | Endpoint | Description |
|--------|----------|-------------|
| `POST` | `/api/v1/landing-pages/{id}/builds` | Trigger a build (compile to deployable HTML) |
| `GET` | `/api/v1/landing-pages/{id}/builds` | List builds |
| `GET` | `/api/v1/landing-pages/{id}/builds/{build_id}` | Get build status/details |
| `POST` | `/api/v1/landing-pages/{id}/start` | Start serving the landing page |
| `POST` | `/api/v1/landing-pages/{id}/stop` | Stop serving the landing page |
| `GET` | `/api/v1/landing-pages/{id}/health` | Health check for running page |

### 24.5 Field Categories

| Method | Endpoint | Description |
|--------|----------|-------------|
| `GET` | `/api/v1/landing-pages/{id}/field-categories` | Get field categorization rules |
| `POST` | `/api/v1/landing-pages/{id}/field-categories` | Create/update field categorization |
| `DELETE` | `/api/v1/landing-pages/{id}/field-categories/{rule_id}` | Delete a categorization rule |

### 24.6 Assets

| Method | Endpoint | Description |
|--------|----------|-------------|
| `POST` | `/api/v1/landing-pages/{id}/assets` | Upload an asset (image, font, file) |
| `GET` | `/api/v1/landing-pages/{id}/assets` | List assets |
| `DELETE` | `/api/v1/landing-pages/{id}/assets/{asset_id}` | Delete an asset |

### 24.7 Landing Page Data Model

```json
{
  "id": "lp_abc123",
  "name": "Microsoft O365 Login",
  "description": "Office 365 login clone for credential harvesting simulation",
  "definition_json": {
    "pages": [
      {
        "id": "page_1",
        "name": "Login",
        "is_start": true,
        "component_tree": {
          "id": "root",
          "type": "root",
          "children": [
            {
              "id": "comp_1",
              "type": "container",
              "label": "Main Wrapper",
              "props": { "tag": "main" },
              "styles": {
                "desktop": { "maxWidth": "400px", "margin": "0 auto", "padding": "2rem" },
                "tablet": {},
                "mobile": { "padding": "1rem" }
              },
              "children": [
                {
                  "id": "comp_2",
                  "type": "image",
                  "label": "Logo",
                  "props": { "src": "/assets/ms-logo.png", "alt": "Microsoft" },
                  "styles": { "desktop": { "width": "108px", "marginBottom": "1.5rem" } },
                  "children": []
                },
                {
                  "id": "comp_3",
                  "type": "heading",
                  "props": { "text": "Sign in", "level": "h1" },
                  "styles": { "desktop": { "fontSize": "24px", "fontWeight": "600" } },
                  "children": []
                },
                {
                  "id": "comp_4",
                  "type": "form-container",
                  "label": "Login Form",
                  "props": { "name": "login-form", "method": "POST" },
                  "styles": {},
                  "children": [
                    {
                      "id": "comp_5",
                      "type": "email-input",
                      "props": {
                        "label": "Email, phone, or Skype",
                        "name": "email",
                        "placeholder": "",
                        "required": true,
                        "autocomplete": "email"
                      },
                      "styles": {},
                      "children": []
                    },
                    {
                      "id": "comp_6",
                      "type": "submit-button",
                      "props": { "text": "Next" },
                      "styles": {
                        "desktop": {
                          "backgroundColor": "#0067b8",
                          "color": "#ffffff",
                          "padding": "8px 24px"
                        }
                      },
                      "children": []
                    }
                  ]
                }
              ]
            }
          ]
        },
        "css": ".login-form input { width: 100%; margin-bottom: 1rem; }",
        "js": ""
      }
    ],
    "global_css": "body { font-family: 'Segoe UI', sans-serif; }",
    "global_js": ""
  },
  "capture_config": {
    "fields": [
      { "name": "email", "category": "identity" },
      { "name": "password", "category": "sensitive" }
    ],
    "hidden_fields": [
      { "name": "tracking_token", "type": "tracking_token" },
      { "name": "timestamp", "type": "timestamp" }
    ],
    "post_capture_action": {
      "type": "redirect_with_delay",
      "redirect_url": "https://login.microsoftonline.com",
      "delay_ms": 3000,
      "loading_message": "Verifying your credentials..."
    }
  },
  "session_capture_enabled": true,
  "theme": "microsoft-office",
  "status": "draft",
  "created_at": "2026-03-15T10:30:00Z",
  "updated_at": "2026-03-15T14:22:00Z",
  "created_by": "user_123",
  "campaign_ids": ["camp_456"]
}
```

---

## 25. Component Tree JSON Schema

### 25.1 Component Node

Every component in the tree follows this schema:

```json
{
  "id": "string (UUID, auto-generated)",
  "type": "string (component type identifier)",
  "label": "string (optional custom label)",
  "props": "object (component-specific properties)",
  "styles": {
    "desktop": "object (CSS properties as camelCase key-value pairs)",
    "tablet": "object (override styles for tablet breakpoint)",
    "mobile": "object (override styles for mobile breakpoint)"
  },
  "customCSS": "string (raw CSS with .this selector)",
  "customJS": "string (raw JavaScript)",
  "attributes": "object (custom HTML attributes, e.g., data-*, aria-*)",
  "visibility": {
    "desktop": true,
    "tablet": true,
    "mobile": true
  },
  "hidden": false,
  "locked": false,
  "children": "array of component nodes"
}
```

### 25.2 Component Type Registry

Each component type defines:
- `type`: unique string identifier (e.g., `"email-input"`).
- `name`: display name (e.g., `"Email Input"`).
- `icon`: icon identifier for palette and layers.
- `category`: palette category (e.g., `"forms"`).
- `defaultProps`: default property values.
- `defaultStyles`: default desktop styles.
- `allowedChildren`: array of allowed child types (empty array = leaf node, `["*"]` = any).
- `allowedParents`: array of allowed parent types (`["*"]` = any).
- `propsSchema`: JSON Schema defining the Content tab fields for this component.

---

## 26. Animation and Transition Specifications

### 26.1 Standard Timings

| Animation | Duration | Easing | Usage |
|-----------|----------|--------|-------|
| Panel open/close | 200ms | `ease-out` | Flyout panels, code editor |
| Flyout content switch | 150ms | `ease-out` | Switching between flyout types |
| Device frame resize | 300ms | `ease-out` | Desktop/tablet/mobile switch |
| Drag snap | 100ms | `--ease-spring` | Components snapping to position after drag |
| Selection outline | 0ms (instant) | — | Hover/click outlines appear immediately |
| Property value scrub | 0ms (realtime) | — | Dragging number inputs updates in real-time |
| Toast enter | 300ms | `ease-out` | Toast notification slide-in from top-right |
| Toast exit | 200ms | `ease-in` | Toast notification fade-out |
| Zoom transition | 200ms | `ease-out` | Canvas zoom in/out |
| Color picker open | 150ms | `ease-out` | Popover scale-in from anchor |
| Modal overlay | 200ms | `ease-out` | Fade-in backdrop + scale-in content |
| Accordion expand | 200ms | `ease-out` | Property section expand/collapse |

### 26.2 Easing Definitions

```css
--ease-out: cubic-bezier(0.16, 1, 0.3, 1);
--ease-in: cubic-bezier(0.7, 0, 0.84, 0);
--ease-spring: cubic-bezier(0.34, 1.56, 0.64, 1);
--ease-standard: cubic-bezier(0.4, 0, 0.2, 1);
```

---

## 27. State Management

### 27.1 Builder State Shape

The builder maintains the following top-level state:

```typescript
interface BuilderState {
  // Project data
  project: LandingPage;              // Full project including all pages
  activePageId: string;              // Currently displayed page ID
  isDirty: boolean;                  // Unsaved changes flag
  lastSavedAt: Date | null;          // Timestamp of last successful save
  saveStatus: 'saved' | 'saving' | 'unsaved' | 'error';

  // Selection
  selectedComponentIds: string[];    // Currently selected component IDs
  hoveredComponentId: string | null; // Component under cursor

  // UI state
  activeBreakpoint: 'desktop' | 'tablet' | 'mobile';
  zoom: number;                      // 0.25 to 2.0
  panOffset: { x: number; y: number };
  activeFlyout: 'components' | 'layers' | 'pages' | 'templates' | 'settings' | null;
  codeEditorOpen: boolean;
  codeEditorHeight: number;
  codeEditorTab: 'page-css' | 'page-js' | 'component-css' | 'component-js' | 'global-css' | 'global-js';
  propertyEditorTab: 'content' | 'style' | 'advanced';
  previewMode: boolean;

  // History
  undoStack: HistoryEntry[];
  redoStack: HistoryEntry[];

  // Drag state
  dragState: {
    active: boolean;
    sourceId: string | null;         // Component being dragged (null if from palette)
    sourceType: string | null;       // Component type (for palette drags)
    targetId: string | null;         // Drop target container
    insertionIndex: number | null;   // Position within target
  } | null;

  // Clipboard
  clipboard: ComponentNode[] | null;
}
```

### 27.2 State Persistence

| State | Persisted | Storage |
|-------|-----------|---------|
| Project data | Yes | API (auto-save) |
| Active page | No | Memory only |
| Selected components | No | Memory only |
| Breakpoint | No | Memory only (resets to desktop) |
| Zoom / pan | Yes | `localStorage` per project |
| Active flyout | Yes | `localStorage` |
| Code editor state | Yes | `localStorage` |
| Undo/redo history | No | Memory only (lost on navigation) |

---

## 28. Accessibility

### 28.1 ARIA Roles and Labels

| Element | Role | ARIA Attributes |
|---------|------|-----------------|
| Icon toolbar | `toolbar` | `aria-label="Builder tools"` |
| Flyout panel | `complementary` | `aria-label="[Panel name] panel"` |
| Canvas iframe | `application` | `aria-label="Page canvas"` |
| Property editor | `complementary` | `aria-label="Property editor"` |
| Layers tree | `tree` | `aria-label="Component layers"` |
| Layer node | `treeitem` | `aria-expanded`, `aria-selected`, `aria-level` |
| Component palette | `listbox` | `aria-label="Component palette"` |
| Palette item | `option` | `aria-grabbed` (during drag) |
| Breadcrumb bar | `navigation` | `aria-label="Component hierarchy"` |

### 28.2 Focus Management

- When a flyout opens, focus moves to the flyout's first interactive element.
- When a flyout closes, focus returns to the toolbar button that triggered it.
- When preview mode activates, focus moves to the iframe content.
- When preview mode exits, focus returns to the Preview button.
- The property editor traps tab focus within its panel when editing (not modal-style trapping, but logical focus flow).

### 28.3 Screen Reader Announcements

- Component selection: ARIA live region announces "Selected [component name]".
- Drag and drop: announces "Picked up [name]", "Moved to position [N] in [parent]", "Dropped [name]".
- Save status changes: announces "Changes saved" / "Save failed".
- Breakpoint switch: announces "Editing [Desktop/Tablet/Mobile] breakpoint".

---

## 29. Design System Integration

### 29.1 Builder-Specific Tokens

The builder uses the global design system tokens (document 01) plus these builder-specific additions:

| Token | Value | Usage |
|-------|-------|-------|
| `--builder-canvas-bg` | `var(--bg-primary)` | Canvas surrounding area |
| `--builder-selection` | `var(--accent)` (#4a7ab5) | Selection outlines, insertion lines |
| `--builder-hover` | `rgba(74, 122, 181, 0.3)` | Hover highlight fill |
| `--builder-toolbar-bg` | `var(--bg-secondary)` | Icon toolbar, canvas toolbar |
| `--builder-panel-bg` | `var(--bg-secondary)` | Flyout panels, property editor |
| `--builder-panel-border` | `var(--border-primary)` | Panel divider lines |
| `--builder-drag-ghost` | `rgba(74, 122, 181, 0.4)` | Drag ghost opacity |
| `--builder-insertion-line` | `var(--accent)` | Drop insertion indicator |
| `--builder-error` | `var(--status-error)` | Constraint violation flash |

### 29.2 Component Rendering Theme

Components rendered inside the canvas iframe use their own CSS — they do NOT inherit the builder's dark theme. The page content typically uses:
- White or light backgrounds (to simulate real login pages).
- Template-specific fonts and colors.
- The builder does not inject any theme variables into the iframe.

---

## 30. Route Structure

| Route | View |
|-------|------|
| `/landing-pages` | Landing page list view |
| `/landing-pages/new` | Template gallery (creation flow) |
| `/landing-pages/{id}/edit` | Builder (full-screen editor) |
| `/landing-pages/{id}/preview` | Standalone preview (no builder chrome) |
| `/campaigns/{id}/landing-page/new` | Template gallery with campaign context |
| `/campaigns/{id}/landing-page/{lp_id}/edit` | Builder with campaign context |

When navigating to the builder route, the application:
1. Fetches the landing page data via `GET /api/v1/landing-pages/{id}`.
2. Parses `definition_json` into the in-memory component tree.
3. Renders the builder layout with the first (or start) page active.
4. Loading state: skeleton of the builder layout with a spinner in the canvas area.

---

*This document is the authoritative specification for the Landing Page Builder. All implementation decisions, component behaviors, API integrations, and interaction patterns are defined here. Deviations from this specification require documented justification and approval.*
