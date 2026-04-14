# 02 — Application Builder Interface

## 2.1 Overview

The Application Builder is the primary interface through which operators design landing applications. It is a full-screen, no-code structural editor within the Tackle admin frontend. The builder provides drag-and-drop component placement, property editing, page management, workflow configuration, and real-time development server integration.

The canvas is a **structural representation** of the component tree — it shows the hierarchy, nesting, and order of components, not a pixel-accurate rendering of how the page will look. The actual visual output is seen by running the development server, which compiles and serves the real application.

## 2.2 Builder Layout

```
┌─────────────────────────────────────────────────────────────────────────┐
│ TOOLBAR                                                                 │
│ [← Back] [Project Name]       [Undo|Redo]                              │
│                          [Dev Server: Start|Stop]  [Save]               │
└─────────────────────────────────────────────────────────────────────────┘
┌────┬───────────┬──────────────────────────────────┬─────────────────────┐
│Icon│ Left      │                                  │ Right               │
│Bar │ Panel     │         CANVAS                   │ Inspector           │
│    │           │                                  │                     │
│ C  │ Component │  ┌──────────────────────────┐    │ [Content|Style|Adv] │
│ P  │ Palette   │  │                          │    │                     │
│ L  │   or      │  │   Structural Component   │    │  Property editors   │
│ W  │ Pages     │  │   Tree Representation    │    │  for selected       │
│    │   or      │  │                          │    │  component          │
│    │ Layers    │  │   (drag-and-drop,        │    │                     │
│    │   or      │  │    select, reorder)      │    │                     │
│    │ Workflows │  │                          │    │                     │
│    │           │  └──────────────────────────┘    │                     │
└────┴───────────┴──────────────────────────────────┴─────────────────────┘
```

### Icon Bar (Left Edge)

A narrow vertical bar with icons that switch the left panel content:

| Icon | Panel | Purpose |
|------|-------|---------|
| **C** | Component Palette | Drag components onto the canvas |
| **P** | Page Manager | Create, edit, delete, and switch between pages |
| **L** | Layers Panel | Navigate the component tree hierarchy (DOM tree view) |
| **W** | Workflow Editor | Configure navigation flows and event triggers |

### Left Panel (~280px)

Displays the content corresponding to the selected icon bar tab. Each panel is described in detail below.

### Canvas (Center)

The structural editing area. Displays the active page's component tree as a visual block representation with builder overlays (selection outlines, drop indicators, component type labels). The canvas shows the **structure and hierarchy** of components — not a pixel-accurate rendering of the final page. Supports:

- Drag-and-drop from the palette and within the tree
- Click-to-select components
- Visual drop position indicators (top, bottom, left, right, inside)

### Right Inspector (~340px)

Property editor for the currently selected component. Three tabs:

- **Content**: Component-specific properties (text, image source, input name, placeholder, heading level, etc.)
- **Style**: Visual CSS properties (layout, spacing, typography, borders, backgrounds)
- **Advanced**: Capture configuration, click actions, behavioral capabilities, custom CSS classes, DOM IDs

### Toolbar (Top)

- **Back**: Return to the landing pages list
- **Project Name**: Editable inline
- **Undo / Redo**: Step through edit history (max 50 states)
- **Dev Server**: Start/Stop the development server, with status indicator (online/offline/building)
- **Save**: Persist the current definition to Tackle's database

## 2.3 Component Palette

The palette presents all available component types organized by category. Each item is draggable onto the canvas.

### Categories

| Category | Components |
|----------|-----------|
| **Layout** | Container, Row, Column, Section, Card, Divider, Spacer |
| **Navigation** | Navbar, Footer, Sidebar, Tabs, Breadcrumb |
| **Text** | Heading, Paragraph, Text, Span, Label, Blockquote, Code Block |
| **Media** | Image, Icon, Video, Logo, Iframe |
| **Forms** | Form, Text Input, Password Input, Email Input, Textarea, Select, Checkbox, Radio, File Upload, Hidden Field |
| **Interactive** | Button, Submit Button, Link, Toggle |
| **Feedback** | Alert, Spinner, Progress Bar, Toast |
| **Special** | Raw HTML |

### Drag Behavior

- Dragging a palette item creates a new component instance (new unique ID, default properties)
- Dragging an existing canvas component moves it within the tree
- Drop position is determined by cursor location relative to the target component (top, bottom, left, right, inside)
- Container-type components accept drops inside them; non-container components only accept adjacent drops
- Auto-wrapping: dropping left/right of a non-row component automatically wraps both in a new Row; dropping top/bottom inside a Row wraps in a Column

## 2.4 Canvas Rendering

The canvas displays the active page's component tree as a structural block layout. Each component is represented as a labeled block showing its type, nesting depth, and relationship to other components. This is **not** a visual preview — the operator uses the development server to see the actual rendered output.

### Component Blocks

Each component is displayed as a block element on the canvas showing:
- The component type label (e.g., "heading", "form", "container")
- The component's custom name (if set by the operator)
- A brief content preview for text-bearing components (e.g., "Sign In" for a heading)
- Nesting indicators for parent-child relationships

### Selection State

- **Hover**: Subtle outline indicating the component boundary
- **Selected**: Blue outline (2px) with component type badge in the top-left corner
- **Drop target**: Blue highlight indicating valid drop zones with directional indicators (top/bottom/left/right bars, or filled highlight for "inside")

### Void Elements

Components that cannot have children (Image, Input, Divider, Spacer, etc.) are rendered without drop-inside capability. They only accept adjacent drops.

### Root Container

Every page has an implicit root container. Components dropped directly on the canvas are placed inside this root.

## 2.5 Page Manager

The Page Manager panel allows operators to manage the multi-page structure of the landing application.

### Page Properties

Each page has:

| Property | Description | Example |
|----------|-------------|---------|
| **Name** | Internal display name (shown in builder only) | "Login Page" |
| **Route** | URL path the target sees in the browser | `/signin` |
| **Title** | Browser tab title | "Sign In - Contoso" |
| **Favicon** | Optional favicon URL or embedded asset | (upload) |
| **Meta Tags** | Optional meta tags for the page | viewport, description |
| **Page Styles** | CSS specific to this page | (editor) |
| **Page JavaScript** | JavaScript specific to this page | (editor) |

### Page Operations

- **Add Page**: Creates a new blank page with default name, route, and empty component tree
- **Edit Page**: Modify page metadata (name, route, title, favicon, meta tags)
- **Delete Page**: Remove a page (blocked if it's the only page remaining)
- **Switch Page**: Click a page to make it the active page in the canvas
- **Reorder Pages**: Drag to reorder (first page is the default/landing route)

### Route Constraints

- Routes must start with `/`
- Routes must be unique within the project
- Routes are auto-slugified (spaces → hyphens, lowercased)
- The first page's route is the entry point for the landing application

## 2.6 Layers Panel

The Layers panel displays the component tree as a hierarchical tree view, similar to a DOM inspector.

### Features

- Displays all components on the active page in their nesting hierarchy
- Click a node to select it on the canvas and open its properties in the inspector
- Collapse/expand nodes with children
- Shows component type as tag name (e.g., `<heading>`, `<form>`, `<container>`)
- Shows truncated text content for text-bearing components (headings, paragraphs, buttons)
- Visual tree lines connecting parent to children
- Depth-based indentation

### Interaction

- Clicking a layer node selects the corresponding component on the canvas
- The canvas scrolls to bring the selected component into view
- Selection state is synchronized bidirectionally (canvas ↔ layers)

## 2.7 Workflow Editor

The Workflow Editor allows operators to define event-driven navigation flows and interaction triggers. This is detailed fully in [04 - Page Navigation & Workflow Engine](04-page-navigation-workflows.md).

## 2.8 Right Inspector: Content Tab

The Content tab displays component-specific editable properties. The fields shown depend on the selected component's type.

### Universal Properties

All components expose:
- **Component Name**: Optional friendly name shown in the layers panel and component badge

### Type-Specific Properties

| Component Type | Content Properties |
|---------------|-------------------|
| **Heading** | Text content, Heading level (h1–h6) |
| **Paragraph / Text / Span** | Text content |
| **Label** | Text content, `for` attribute |
| **Button / Submit Button** | Button text, Button type (button/submit/reset) |
| **Link** | Link text, href, target (_blank/_self) |
| **Image / Logo** | Source (upload or URL), Alt text, Width, Height |
| **Icon** | Icon name/set, Size, Color |
| **Video** | Source URL, Autoplay, Muted, Loop, Controls |
| **Iframe** | Source URL, Width, Height, Sandbox attributes |
| **Text Input** | Name, Placeholder, Label text, Required, Autocomplete |
| **Password Input** | Name, Placeholder, Label text, Required |
| **Email Input** | Name, Placeholder, Label text, Required |
| **Textarea** | Name, Placeholder, Rows, Required |
| **Select** | Name, Options (label/value pairs), Placeholder, Required |
| **Checkbox** | Name, Label text, Checked default, Value |
| **Radio** | Name, Label text, Value, Checked default |
| **File Upload** | Name, Accept types, Multiple, Label text |
| **Hidden Field** | Name, Value |
| **Form** | Action (POST endpoint path), Method (POST/GET) |
| **Navbar** | Brand text/logo, Navigation items |
| **Tabs** | Tab labels, Active tab index |
| **Alert** | Message text, Severity (info/warning/error/success), Dismissible |
| **Blockquote** | Quote text, Citation |
| **Code Block** | Code content, Language |
| **Raw HTML** | Raw HTML string |

## 2.9 Right Inspector: Style Tab

The Style tab provides visual controls for CSS properties. All components share the same style interface.

### Layout & Flexbox

| Property | Control | Values |
|----------|---------|--------|
| Display | Select | block, flex, inline-block, inline, grid, none |
| Flex Direction | Select (if flex) | row, column, row-reverse, column-reverse |
| Align Items | Select (if flex) | flex-start, center, flex-end, stretch, baseline |
| Justify Content | Select (if flex) | flex-start, center, flex-end, space-between, space-around, space-evenly |
| Flex Wrap | Select (if flex) | nowrap, wrap |
| Gap | Input | CSS value (e.g., "10px", "1rem") |
| Flex Grow | Input | Number |
| Width | Input | CSS value |
| Height | Input | CSS value |
| Min Width | Input | CSS value |
| Min Height | Input | CSS value |
| Max Width | Input | CSS value |
| Max Height | Input | CSS value |
| Overflow | Select | visible, hidden, scroll, auto |
| Position | Select | static, relative, absolute, fixed |

### Spacing (Box Model)

| Property | Control |
|----------|---------|
| Padding (top, right, bottom, left) | Individual inputs |
| Margin (top, right, bottom, left) | Individual inputs |

### Typography

| Property | Control |
|----------|---------|
| Font Family | Input / Select |
| Font Size | Input |
| Font Weight | Select (normal, 500, 600, bold) |
| Line Height | Input |
| Text Color | Color picker + hex input |
| Text Align | Toggle (left, center, right, justify) |
| Text Transform | Select (none, uppercase, lowercase, capitalize) |
| Text Decoration | Select (none, underline, line-through) |
| Letter Spacing | Input |

### Borders & Backgrounds

| Property | Control |
|----------|---------|
| Background Color | Color picker + hex input |
| Background Image | URL input |
| Border Width | Input |
| Border Style | Select (none, solid, dashed, dotted) |
| Border Color | Color picker + hex input |
| Border Radius | Input |
| Box Shadow | Input (CSS string) |
| Opacity | Slider / Input (0–1) |

### Dimension Auto-Suffixing

When an operator enters a bare number (e.g., "16") for dimension properties (width, height, padding, margin, font-size, etc.), the builder automatically appends "px". Operators can explicitly enter other units (rem, %, em, vh, vw).

## 2.10 Right Inspector: Advanced Tab

The Advanced tab contains configuration for capture behavior, click actions, behavioral capabilities, and DOM attributes.

### For Form Components

| Property | Description |
|----------|-------------|
| **Action Path** | The URL path the form POSTs to (e.g., `/signin`, `/api/auth`) |
| **Method** | HTTP method (POST or GET) |
| **Post-Submit Action** | What happens after form submission (see §2.10.1) |

### For Input Components

| Property | Description |
|----------|-------------|
| **Capture Tag** | Auto-detected from input type/name, operator can override. Values: username, password, email, mfa_token, credit_card, custom, generic |

### For Button / Link Components

| Property | Description |
|----------|-------------|
| **Click Action** | What happens on click (see §2.10.2) |

### For All Components

| Property | Description |
|----------|-------------|
| **Custom CSS Class** | Additional CSS class names |
| **DOM ID** | Explicit HTML id attribute (used for workflow targeting) |
| **Page-Level Behaviors** | Toggle behavioral capabilities (see [06 - Behavioral Capabilities](06-behavioral-capabilities.md)) |

### 2.10.1 Post-Submit Actions

When a form is submitted, the operator configures what happens next:

| Action | Description | Configuration |
|--------|-------------|---------------|
| **Navigate to Page** | Route the target to another page in the application | Target page (dropdown from page list) |
| **External Redirect** | Redirect the target to an external URL | Target URL |
| **Display Message** | Show a success/loading/error message on the current page | Message text, style |
| **Delayed Redirect** | Show a loading animation, then redirect after N ms | Delay (ms), target (page or URL) |
| **Replay to Real Service** | Forward the form data to the real service being spoofed | Real service URL |
| **No Action** | Form submits but nothing visible happens | — |

### 2.10.2 Click Actions

For buttons and links that are NOT submit buttons:

| Action | Description | Configuration |
|--------|-------------|---------------|
| **No Action** | Default browser behavior | — |
| **Navigate to Page** | Internal page navigation | Target page |
| **External Redirect** | Open external URL | URL, target (_blank/_self) |

## 2.11 Project Lifecycle

### Creating a Project

- Operator navigates to the Landing Pages list
- Clicks "New Landing Page"
- Tackle creates a new project with a default name, single blank page, and empty component tree
- Builder opens immediately

### Saving

- Manual save via toolbar button
- Serializes the full component tree, page list, global styles/JS, navigation rules, and theme into the `definition_json` payload
- PUTs the updated definition to Tackle's API

### Opening Existing Projects

- Operator selects a project from the Landing Pages list
- Builder loads the project's `definition_json` and hydrates the component tree
- Dev server status is checked on load

## 2.12 Undo / Redo

- Every mutation to the component tree, page list, or configuration pushes a new state onto the history stack
- Maximum 50 history states
- Undo steps backward through history; Redo steps forward
- History is per-session (cleared on page load, not persisted)
- The toolbar displays enabled/disabled state for Undo and Redo based on position in history

## 2.13 Keyboard Shortcuts

| Shortcut | Action |
|----------|--------|
| Ctrl+Z | Undo |
| Ctrl+Shift+Z / Ctrl+Y | Redo |
| Ctrl+S | Save project |
| Delete / Backspace | Delete selected component |
| Escape | Deselect current component |
