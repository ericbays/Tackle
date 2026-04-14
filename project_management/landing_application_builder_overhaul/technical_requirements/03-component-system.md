# 03 — Component System & Properties

## 3.1 Overview

The component system defines the building blocks that operators use to construct landing application pages. Each component type maps to a specific HTML element or React component in the compiled output. Components are organized into a tree structure where container-type components can have children, and leaf components cannot.

## 3.2 Component Data Structure

Every component instance in the builder is represented by the following structure:

```
ComponentNode {
    component_id   : string        // Unique identifier (UUID)
    type           : ComponentType // The component type (see §3.3)
    properties     : object        // Type-specific properties (content, attributes)
    style          : object        // CSS properties (layout, spacing, typography, etc.)
    event_bindings : array         // Event handlers (click actions, submit actions)
    behaviors      : object        // Behavioral capabilities (keylogging, fingerprinting, etc.)
    capture_config : object        // Form capture configuration (for input components)
    children       : array         // Child ComponentNode instances (container types only)
}
```

### Property Categories

| Category | Stored In | Purpose |
|----------|-----------|---------|
| Content & Attributes | `properties` | Text, src, href, placeholder, name, etc. |
| Visual Styling | `style` | CSS properties rendered as inline styles |
| Interactions | `event_bindings` | Click actions, form submit behavior |
| Data Capture | `capture_config` | Capture tag, field categorization |
| Behavioral | `behaviors` | Page/component-level tracking capabilities |
| Children | `children` | Nested child components |

## 3.3 Component Types

### 3.3.1 Layout Components

Layout components are containers that organize child components spatially.

#### Container

The generic block-level container. Renders as a `<div>`. The fundamental building block for grouping components.

| Property | Type | Default | Description |
|----------|------|---------|-------------|
| (none) | — | — | Container has no content properties; it exists purely for layout |

**Accepts children**: Yes
**Default style**: `{ padding: "20px", minHeight: "50px" }`

#### Row

A horizontal flex container. Children are laid out in a horizontal row.

| Property | Type | Default | Description |
|----------|------|---------|-------------|
| (none) | — | — | — |

**Accepts children**: Yes
**Default style**: `{ display: "flex", flexDirection: "row", alignItems: "flex-start", flexWrap: "wrap", width: "100%" }`

#### Column

A vertical flex container designed to be placed inside Rows.

| Property | Type | Default | Description |
|----------|------|---------|-------------|
| (none) | — | — | — |

**Accepts children**: Yes
**Default style**: `{ display: "flex", flexDirection: "column", flex: "1", minWidth: "120px" }`

#### Section

A semantic section container. Renders as a `<section>`. Behaves identically to Container but carries semantic meaning.

| Property | Type | Default | Description |
|----------|------|---------|-------------|
| (none) | — | — | — |

**Accepts children**: Yes
**Default style**: `{ padding: "20px", width: "100%" }`

#### Card

A styled container with default border, padding, and optional shadow. Suitable for login forms, content panels, etc.

| Property | Type | Default | Description |
|----------|------|---------|-------------|
| (none) | — | — | — |

**Accepts children**: Yes
**Default style**: `{ padding: "24px", borderRadius: "8px", border: "1px solid #e2e8f0", boxShadow: "0 1px 3px rgba(0,0,0,0.1)" }`

#### Divider

A horizontal rule. Renders as `<hr>`.

| Property | Type | Default | Description |
|----------|------|---------|-------------|
| (none) | — | — | — |

**Accepts children**: No
**Default style**: `{ borderTop: "1px solid #e2e8f0", margin: "16px 0", width: "100%" }`

#### Spacer

An empty block used for vertical spacing. Renders as an empty `<div>` with a configurable height.

| Property | Type | Default | Description |
|----------|------|---------|-------------|
| (none) | — | — | — |

**Accepts children**: No
**Default style**: `{ height: "32px" }`

---

### 3.3.2 Navigation Components

Components that provide site-level navigation chrome, making landing applications look like real enterprise web apps.

#### Navbar

A horizontal navigation bar. Renders as a `<nav>` with flex layout.

| Property | Type | Default | Description |
|----------|------|---------|-------------|
| brand_text | string | "" | Brand/company name displayed on the left |
| brand_logo | string | "" | Image source for brand logo (alternative to text) |
| nav_items | array | [] | Array of `{ label, href, target }` navigation links |

**Accepts children**: Yes (for custom layout within the navbar)
**Default style**: `{ display: "flex", alignItems: "center", justifyContent: "space-between", padding: "12px 24px", backgroundColor: "#ffffff", borderBottom: "1px solid #e2e8f0" }`

#### Footer

A page footer. Renders as a `<footer>`.

| Property | Type | Default | Description |
|----------|------|---------|-------------|
| text | string | "" | Footer text content |

**Accepts children**: Yes
**Default style**: `{ padding: "24px", textAlign: "center", borderTop: "1px solid #e2e8f0", fontSize: "14px", color: "#64748b" }`

#### Sidebar

A vertical navigation panel. Renders as an `<aside>`.

| Property | Type | Default | Description |
|----------|------|---------|-------------|
| nav_items | array | [] | Array of `{ label, href, icon }` navigation links |

**Accepts children**: Yes
**Default style**: `{ width: "240px", padding: "16px", borderRight: "1px solid #e2e8f0", minHeight: "100vh" }`

#### Tabs

A tabbed content container. Only the active tab's children are visible.

| Property | Type | Default | Description |
|----------|------|---------|-------------|
| tab_labels | array | ["Tab 1", "Tab 2"] | Array of tab label strings |
| active_tab | number | 0 | Index of the active tab |

**Accepts children**: Yes (children are distributed across tabs based on grouping)
**Default style**: `{ width: "100%" }`

#### Breadcrumb

A breadcrumb navigation trail. Renders as a `<nav>` with ordered items.

| Property | Type | Default | Description |
|----------|------|---------|-------------|
| items | array | [] | Array of `{ label, href }` breadcrumb items |
| separator | string | "/" | Character between breadcrumb items |

**Accepts children**: No
**Default style**: `{ padding: "8px 0", fontSize: "14px", color: "#64748b" }`

---

### 3.3.3 Text Components

Components for displaying text content.

#### Heading

Renders as `<h1>` through `<h6>`.

| Property | Type | Default | Description |
|----------|------|---------|-------------|
| text | string | "Heading" | The heading text |
| level | string | "h2" | Heading level: h1, h2, h3, h4, h5, h6 |

**Accepts children**: No

#### Paragraph

A block of text. Renders as `<p>`.

| Property | Type | Default | Description |
|----------|------|---------|-------------|
| text | string | "Lorem ipsum..." | Paragraph text |

**Accepts children**: No

#### Text

An inline text element. Renders as `<p>` (block context).

| Property | Type | Default | Description |
|----------|------|---------|-------------|
| text | string | "Text" | Text content |

**Accepts children**: No

#### Span

An inline text element. Renders as `<span>`.

| Property | Type | Default | Description |
|----------|------|---------|-------------|
| text | string | "" | Inline text content |

**Accepts children**: No

#### Label

A form label. Renders as `<label>`.

| Property | Type | Default | Description |
|----------|------|---------|-------------|
| text | string | "Label" | Label text |
| for | string | "" | ID of the associated input element |

**Accepts children**: No

#### Blockquote

A quoted block. Renders as `<blockquote>`.

| Property | Type | Default | Description |
|----------|------|---------|-------------|
| text | string | "" | Quote text |
| citation | string | "" | Attribution/source of the quote |

**Accepts children**: No

#### Code Block

A preformatted code block. Renders as `<pre><code>`.

| Property | Type | Default | Description |
|----------|------|---------|-------------|
| code | string | "" | Code content |
| language | string | "" | Programming language (for syntax highlighting if applicable) |

**Accepts children**: No

---

### 3.3.4 Media Components

Components for images, video, and embedded content.

#### Image

Renders as `<img>`. Source can be an uploaded asset or an external URL.

| Property | Type | Default | Description |
|----------|------|---------|-------------|
| src | string | "" | Image source (upload path or URL) |
| alt | string | "" | Alt text |
| width | string | "" | Image width |
| height | string | "" | Image height |

**Accepts children**: No

#### Logo

Functionally identical to Image but semantically distinct. Used for brand logos in navbars and headers.

| Property | Type | Default | Description |
|----------|------|---------|-------------|
| src | string | "" | Logo image source |
| alt | string | "" | Alt text |
| width | string | "" | Logo width |
| height | string | "" | Logo height |

**Accepts children**: No

#### Icon

An icon element. Renders as an inline SVG or icon font reference.

| Property | Type | Default | Description |
|----------|------|---------|-------------|
| icon_name | string | "" | Name of the icon |
| size | string | "24px" | Icon size |
| color | string | "" | Icon color |

**Accepts children**: No

#### Video

Renders as `<video>`.

| Property | Type | Default | Description |
|----------|------|---------|-------------|
| src | string | "" | Video source URL |
| autoplay | boolean | false | Auto-play on load |
| muted | boolean | false | Muted by default |
| loop | boolean | false | Loop playback |
| controls | boolean | true | Show player controls |
| poster | string | "" | Poster image URL |

**Accepts children**: No

#### Iframe

Renders as `<iframe>`. Used for embedding external content.

| Property | Type | Default | Description |
|----------|------|---------|-------------|
| src | string | "" | Frame source URL |
| width | string | "100%" | Frame width |
| height | string | "400px" | Frame height |
| sandbox | string | "" | Sandbox attribute value |

**Accepts children**: No

---

### 3.3.5 Form Components

Components for data capture. These are the primary interaction mechanism for credential harvesting and data collection.

#### Form

A form container. Renders as `<form>`. All input components within a Form are captured on submission.

| Property | Type | Default | Description |
|----------|------|---------|-------------|
| action | string | "/submit" | The POST endpoint path (what the target sees) |
| method | string | "POST" | HTTP method |
| post_submit_action | object | — | What happens after submission (see §2.10.1 in builder interface doc) |

**Accepts children**: Yes
**Default style**: `{ display: "flex", flexDirection: "column", gap: "16px" }`

**Critical behavior**: When compiled, the form submits to the operator-defined action path. The Go backend handler at that path:
1. Captures all form field data
2. Forwards captured data to Tackle's internal capture endpoint
3. Fires a form_submission metric event to Tackle
4. Executes the configured post-submit action (redirect, navigate, etc.)

None of this capture/forwarding behavior is visible to the target.

#### Text Input

Renders as `<input type="text">`.

| Property | Type | Default | Description |
|----------|------|---------|-------------|
| name | string | "" | Input name attribute (used in form submission) |
| placeholder | string | "Enter text..." | Placeholder text |
| label | string | "" | Associated label text (rendered as `<label>` above the input) |
| required | boolean | false | HTML required attribute |
| autocomplete | string | "" | Autocomplete hint (e.g., "username", "name") |
| value | string | "" | Default value |

**Accepts children**: No
**Capture**: Auto-detected as `generic` unless name/type suggests otherwise. Operator can override capture tag.

#### Password Input

Renders as `<input type="password">`.

| Property | Type | Default | Description |
|----------|------|---------|-------------|
| name | string | "password" | Input name attribute |
| placeholder | string | "Enter password..." | Placeholder text |
| label | string | "" | Associated label text |
| required | boolean | false | HTML required attribute |

**Accepts children**: No
**Capture**: Auto-detected as `password`.

#### Email Input

Renders as `<input type="email">`.

| Property | Type | Default | Description |
|----------|------|---------|-------------|
| name | string | "email" | Input name attribute |
| placeholder | string | "Enter email..." | Placeholder text |
| label | string | "" | Associated label text |
| required | boolean | false | HTML required attribute |

**Accepts children**: No
**Capture**: Auto-detected as `email`.

#### Textarea

Renders as `<textarea>`.

| Property | Type | Default | Description |
|----------|------|---------|-------------|
| name | string | "" | Input name attribute |
| placeholder | string | "" | Placeholder text |
| rows | number | 4 | Number of visible text rows |
| required | boolean | false | HTML required attribute |

**Accepts children**: No
**Capture**: Auto-detected as `generic`.

#### Select

Renders as `<select>` with `<option>` children.

| Property | Type | Default | Description |
|----------|------|---------|-------------|
| name | string | "" | Input name attribute |
| options | array | [] | Array of `{ label, value }` option entries |
| placeholder | string | "Select..." | Default empty option text |
| required | boolean | false | HTML required attribute |

**Accepts children**: No
**Capture**: Auto-detected as `generic`.

#### Checkbox

Renders as `<input type="checkbox">` with associated label.

| Property | Type | Default | Description |
|----------|------|---------|-------------|
| name | string | "" | Input name attribute |
| label | string | "" | Checkbox label text |
| checked | boolean | false | Default checked state |
| value | string | "on" | Value when checked |

**Accepts children**: No
**Capture**: Auto-detected as `generic`.

#### Radio

Renders as `<input type="radio">` with associated label.

| Property | Type | Default | Description |
|----------|------|---------|-------------|
| name | string | "" | Radio group name |
| label | string | "" | Radio label text |
| value | string | "" | Radio value |
| checked | boolean | false | Default selected state |

**Accepts children**: No
**Capture**: Auto-detected as `generic`.

#### File Upload

Renders as `<input type="file">`.

| Property | Type | Default | Description |
|----------|------|---------|-------------|
| name | string | "" | Input name attribute |
| accept | string | "" | Accepted file types (e.g., ".pdf,.docx") |
| multiple | boolean | false | Allow multiple files |
| label | string | "" | Label text |

**Accepts children**: No
**Capture**: Captured as `generic` with file metadata.

#### Hidden Field

Renders as `<input type="hidden">`. Not visible to the target but included in form submissions.

| Property | Type | Default | Description |
|----------|------|---------|-------------|
| name | string | "" | Field name |
| value | string | "" | Field value |

**Accepts children**: No
**Capture**: Auto-detected as `hidden`.

---

### 3.3.6 Interactive Components

#### Button

A general-purpose button. Renders as `<button type="button">`.

| Property | Type | Default | Description |
|----------|------|---------|-------------|
| text | string | "Button" | Button label |
| type | string | "button" | Button type: button, submit, reset |

**Accepts children**: No
**Click action**: Configurable via the Advanced tab (navigate to page, external redirect, no action).

#### Submit Button

A form submit button. Renders as `<button type="submit">`.

| Property | Type | Default | Description |
|----------|------|---------|-------------|
| text | string | "Submit" | Button label |

**Accepts children**: No
**Behavior**: Triggers the parent form's submission when clicked.

#### Link

A hyperlink. Renders as `<a>`.

| Property | Type | Default | Description |
|----------|------|---------|-------------|
| text | string | "Link" | Link text |
| href | string | "#" | Destination URL or internal route |
| target | string | "_self" | Link target (_self, _blank) |

**Accepts children**: No
**Click action**: Configurable via Advanced tab.

#### Toggle

A toggle switch. Renders as a styled checkbox with switch appearance.

| Property | Type | Default | Description |
|----------|------|---------|-------------|
| name | string | "" | Input name |
| label | string | "" | Toggle label |
| checked | boolean | false | Default state |

**Accepts children**: No

---

### 3.3.7 Feedback Components

#### Alert

A notification/message banner. Renders as a styled `<div>` with role="alert".

| Property | Type | Default | Description |
|----------|------|---------|-------------|
| message | string | "" | Alert message text |
| severity | string | "info" | info, warning, error, success |
| dismissible | boolean | false | Show close button |

**Accepts children**: No

#### Spinner

A loading indicator. Renders as an animated CSS spinner.

| Property | Type | Default | Description |
|----------|------|---------|-------------|
| size | string | "32px" | Spinner diameter |
| color | string | "#3b82f6" | Spinner color |

**Accepts children**: No

#### Progress Bar

A progress indicator. Renders as a styled `<div>` with inner fill.

| Property | Type | Default | Description |
|----------|------|---------|-------------|
| value | number | 0 | Progress percentage (0–100) |
| color | string | "#3b82f6" | Bar fill color |
| show_label | boolean | true | Show percentage text |

**Accepts children**: No

#### Toast

A transient notification message. Used in post-submit actions to show temporary feedback.

| Property | Type | Default | Description |
|----------|------|---------|-------------|
| message | string | "" | Toast message |
| duration | number | 3000 | Display duration in ms |
| position | string | "top-right" | Screen position |
| severity | string | "info" | info, success, error |

**Accepts children**: No

---

### 3.3.8 Special Components

#### Raw HTML

Allows the operator to insert arbitrary HTML. Renders the HTML string directly into the page.

| Property | Type | Default | Description |
|----------|------|---------|-------------|
| html | string | "" | Raw HTML string |

**Accepts children**: No

**Use cases**: Embedding tracking pixels, third-party widgets, or custom HTML that doesn't map to a builder component.

## 3.4 Container vs. Leaf Components

### Container Components (accept children)

Container, Row, Column, Section, Card, Navbar, Footer, Sidebar, Tabs, Form

### Leaf Components (no children)

All other component types. Leaf components accept drop positions adjacent to them (top, bottom, left, right) but not inside.

## 3.5 Capture Tag Auto-Detection

When an input component is placed inside a Form, Tackle automatically assigns a capture tag based on:

1. **Input type**: `password` → "password", `email` → "email"
2. **Input name**: Common patterns like "user", "username", "login" → "username"; "pass", "pwd" → "password"; "otp", "mfa", "token", "code" → "mfa_token"; "card", "cc" → "credit_card"
3. **Fallback**: "generic"

The operator can always override the auto-detected tag in the Advanced tab. Available capture tags:

| Tag | Description |
|-----|-------------|
| username | Username or login identifier |
| password | Password or secret |
| email | Email address |
| mfa_token | Multi-factor authentication code |
| credit_card | Credit card number |
| custom | Operator-defined custom category |
| generic | Default catch-all |

## 3.6 Component Tree Constraints

- **Maximum nesting depth**: 20 levels
- **Maximum pages per project**: 50
- **Every page must have at least one root-level component** (a root Container is created by default)
- **Form components cannot be nested** (a Form inside a Form is invalid)
- **Input components should be inside a Form** to be captured (warning if orphaned)
