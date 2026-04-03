# 05 — Landing Page Builder

## 1. Purpose

The Landing Page Builder is Tackle's integrated application builder for creating phishing landing pages. Rather than cloning existing websites, operators build fully functional web applications from scratch using a visual drag-and-drop interface within the framework UI. The framework then compiles each design into a standalone React frontend + Go backend binary, hosts it on an arbitrary unused port on the framework server, and phishing endpoints transparently proxy target traffic to it. Every compiled build incorporates anti-fingerprinting techniques so that no two campaign landing pages share detectable structural similarities when examined by defensive security tooling.

## 2. Architectural Context

```
┌─────────────────────────────────────────────────────────────────┐
│                    TACKLE FRAMEWORK SERVER                      │
│                                                                 │
│  ┌────────────────────┐      ┌────────────────────────────────┐ │
│  │  Admin UI (React)  │      │ Landing Page App (Campaign A)  │ │
│  │                    │      │ :rand_port_1                   │ │
│  │  ┌──────────────┐  │      └────────────────────────────────┘ │
│  │  │ Landing Page │  │      ┌────────────────────────────────┐ │
│  │  │   Builder    │  │      │ Landing Page App (Campaign B)  │ │
│  │  │  (drag-drop) │  │      │ :rand_port_2                   │ │
│  │  └──────┬───────┘  │      └────────────────────────────────┘ │
│  └─────────┼──────────┘      ┌────────────────────────────────┐ │
│            │ save            │ Landing Page App (Campaign C)  │ │
│            ▼                 │ :rand_port_3                   │ │
│  ┌─────────────────┐         └──────────┬─────────────────────┘ │
│  │  Compilation    │──── build ────────►│                       │
│  │  Engine (Go)    │                    │                       │
│  └─────────────────┘                    │                       │
└─────────────────────────────────────────┼───────────────────────┘
                                          │ transparent proxy
                                          ▼
                             ┌──────────────────────┐
                             │  Phishing Endpoint   │
                             │  (target-facing)     │
                             └──────────────────────┘
```

**Data flow:** Builder UI -> JSON page definition (stored in DB) -> Compilation Engine -> React+Go binary -> Hosted on unused port -> Proxied through phishing endpoint -> Served to targets.

## 3. Builder Features

### 3.1 Visual Drag-and-Drop Page Builder

| ID | Requirement | Details |
|----|-------------|---------|
| **REQ-LPB-001** | The builder shall provide a visual drag-and-drop interface for constructing landing pages from scratch. | No URL cloning or website mirroring functionality. All pages are designed by the operator within the builder. |
| **REQ-LPB-002** | The builder shall use a canvas-based WYSIWYG editor where components are placed by dragging from a library panel onto the canvas. | Canvas must support grid/freeform layout modes. Snapping and alignment guides are required. |
| **REQ-LPB-003** | The builder shall support undo/redo for all canvas operations. | Minimum 50-level undo stack. |
| **REQ-LPB-004** | The builder shall persist page definitions as a structured JSON document stored in PostgreSQL. | The JSON schema must capture all component properties, layout, styles, event bindings, and page routing. |
| **REQ-LPB-005** | The builder shall support saving and loading page definitions as reusable templates. | Templates are stored per-user and can optionally be shared across the team. |

### 3.2 Component Library

| ID | Requirement | Details |
|----|-------------|---------|
| **REQ-LPB-006** | The builder shall provide a component library panel containing all available building blocks. | The panel must be searchable and organized by category. |
| **REQ-LPB-007** | The component library shall include the following component categories and types: | See table below. |

**Component Categories:**

| Category | Components |
|----------|------------|
| **Layout** | Container (div), Row, Column, Section, Spacer, Divider, Card |
| **Navigation** | Navbar, Footer, Breadcrumb, Tabs, Sidebar |
| **Text** | Heading (H1-H6), Paragraph, Span, Label, Blockquote, Code Block |
| **Media** | Image, Icon, Video embed, Logo placeholder |
| **Form Elements** | Text Input, Password Input, Email Input, Textarea, Select/Dropdown, Checkbox, Radio Button, File Upload, Hidden Field |
| **Interactive** | Button, Link, Submit Button, Toggle/Switch |
| **Feedback** | Alert/Banner, Loading Spinner, Progress Bar, Toast/Notification |

| ID | Requirement | Details |
|----|-------------|---------|
| **REQ-LPB-008** | Every component shall expose a properties panel when selected on the canvas. | Properties include: content/value, CSS classes, inline styles, ID, name attribute, placeholder text, visibility conditions, and event bindings. |
| **REQ-LPB-009** | Components shall support nesting — any layout component may contain child components. | The builder must display a component tree (DOM tree view) alongside the canvas for precise hierarchy management. |

### 3.3 Form Builder & Credential Capture Configuration

| ID | Requirement | Details |
|----|-------------|---------|
| **REQ-LPB-010** | The builder shall include a dedicated form builder mode for constructing credential capture forms. | Form builder is accessible as a subset of the main builder, focused on form element configuration. |
| **REQ-LPB-011** | Each form field shall be configurable with: field name, field type, placeholder, label text, required/optional flag, validation rules (regex pattern, min/max length), and a **capture tag**. | The capture tag maps the field to a credential capture category (e.g., `username`, `password`, `email`, `mfa_token`, `custom`). |
| **REQ-LPB-012** | The form builder shall support configuring the form submission behavior: target URL (always the Go backend's capture endpoint), HTTP method (POST), redirect-after-submit URL, and optional delay before redirect. | The Go backend capture endpoint is auto-generated during compilation. Operators configure only the user-facing behavior. |
| **REQ-LPB-013** | The form builder shall support multi-step forms (e.g., username on step 1, password on step 2) with configurable progression logic. | Each step is a distinct form state. Progression can be immediate or conditional (e.g., "after 1 second delay to simulate authentication"). |
| **REQ-LPB-014** | The form builder shall allow operators to add hidden fields with static or dynamic values. | Dynamic values include: target tracking token, timestamp, user-agent string, client IP (injected at runtime by the Go backend). |

### 3.4 Multi-Page Support

| ID | Requirement | Details |
|----|-------------|---------|
| **REQ-LPB-015** | The builder shall support multi-page landing page applications. | Each page is a separate canvas within the same project. Minimum support: 10 pages per project. |
| **REQ-LPB-016** | The builder shall provide a page management panel listing all pages, their routes, and navigation order. | Operators can add, remove, reorder, and rename pages. |
| **REQ-LPB-017** | The builder shall support configuring navigation flows between pages. | Navigation is defined as: automatic redirect (with configurable delay), button/link click, form submission result, or conditional (based on input values). |
| **REQ-LPB-018** | Common multi-page flows shall be available as starter templates: | Templates include: (1) Login -> Loading -> Success, (2) Login -> MFA -> Dashboard, (3) SSO Login -> Consent -> Redirect, (4) File Share Login -> Download Page, (5) Password Reset -> Confirmation. |
| **REQ-LPB-019** | Each page shall have configurable metadata: page title (browser tab), favicon reference, and meta tags. | These values feed into the compiled application's HTML head section. |

### 3.5 CSS/Styling Editor

| ID | Requirement | Details |
|----|-------------|---------|
| **REQ-LPB-020** | The builder shall include a CSS/styling editor accessible per-component and at the project level. | Per-component styles are applied inline or via generated classes. Project-level styles are applied globally. |
| **REQ-LPB-021** | The styling editor shall support both visual property controls (color pickers, spacing sliders, font selectors) and a raw CSS code editor. | Visual controls set properties; raw editor allows arbitrary CSS. Raw CSS takes precedence where conflicts exist. |
| **REQ-LPB-022** | The builder shall support theme presets that set coordinated colors, fonts, spacing, and border styles across all components. | Operators can create, save, and load custom themes. |
| **REQ-LPB-023** | The builder shall include a set of built-in themes mimicking common enterprise application styles. | Themes must include: (1) Microsoft/Office 365 style, (2) Google Workspace style, (3) Generic corporate, (4) Cloud service provider, (5) Banking/financial. These are visual approximations, not clones. |
| **REQ-LPB-024** | The builder shall support responsive design controls — operators can toggle between desktop, tablet, and mobile viewport previews and set breakpoint-specific styles. | Compiled applications must be responsive according to the configured breakpoints. |

### 3.6 JavaScript Injection Points

| ID | Requirement | Details |
|----|-------------|---------|
| **REQ-LPB-025** | The builder shall allow operators to inject custom JavaScript at three levels: (1) project-level (loaded on every page), (2) page-level (loaded on a specific page), and (3) component-level (attached to a specific component's events). | JavaScript is embedded in the compiled application exactly as entered. |
| **REQ-LPB-026** | Component-level JavaScript shall support binding to DOM events: `onClick`, `onSubmit`, `onFocus`, `onBlur`, `onInput`, `onChange`, `onLoad`, `onMouseEnter`, `onMouseLeave`. | Event bindings are configured in the component properties panel. |
| **REQ-LPB-027** | The builder shall provide JavaScript snippet templates for common phishing behaviors. | Snippets include: (1) keylogger capture (send keystrokes to backend), (2) clipboard capture, (3) browser fingerprint collection, (4) session token extraction, (5) simulated loading delay, (6) redirect after timeout, (7) viewport/screen resolution reporting. |
| **REQ-LPB-028** | The JavaScript editor shall include syntax highlighting, basic autocompletion, and error checking (linting). | Use a code editor component (e.g., Monaco/CodeMirror equivalent embedded in the builder). |

> **Security Consideration:** Custom JavaScript runs in the target's browser within the context of the landing page origin. Operators must understand that injected JS is visible in page source. The builder should display a warning when JS is added, reminding operators that targets or defenders can view it.

### 3.7 Preview Mode

| ID | Requirement | Details |
|----|-------------|---------|
| **REQ-LPB-029** | The builder shall provide a preview mode that renders the current page definition as it would appear to a target. | Preview must render within an iframe in the builder UI. |
| **REQ-LPB-030** | Preview mode shall support switching between desktop, tablet, and mobile viewports. | Viewport sizes must match the responsive design breakpoints configured in the styling editor. |
| **REQ-LPB-031** | Preview mode shall support navigating between pages within the multi-page flow. | Form submissions in preview mode must simulate the configured redirect behavior without actually submitting data. |
| **REQ-LPB-032** | Preview mode shall display a banner indicating "PREVIEW MODE" to prevent confusion with a live deployment. | The banner must not appear in compiled production builds. |

## 4. Anti-Fingerprinting System

> **CRITICAL:** Defensive security teams use automated tooling to fingerprint and signature phishing pages. If two campaigns produce structurally similar HTML, CSS, or HTTP responses, defenders can write a single detection rule that blocks all campaigns. The anti-fingerprinting system ensures that every build is structurally unique, even when built from the same page definition.

### 4.1 Requirements

| ID | Requirement | Details |
|----|-------------|---------|
| **REQ-LPB-033** | The compilation engine shall produce structurally unique output for every build, even when compiling the same page definition multiple times. | Two builds of the same project must differ in HTML structure, CSS class names, asset paths, and HTTP behavior sufficiently that no single static signature can match both. |
| **REQ-LPB-034** | The compilation engine shall randomize the HTML DOM structure. | Techniques: vary nesting depth (add/remove semantically neutral wrapper elements), alternate between equivalent HTML tags (e.g., `<div>` vs `<section>` vs `<article>` for containers), randomize element ordering where visually irrelevant (e.g., hidden elements, head meta tags), vary attribute ordering on elements. |
| **REQ-LPB-035** | The compilation engine shall generate procedurally unique CSS class names per build. | Class names must: (a) be random alphanumeric strings of variable length (4-12 chars), (b) optionally use a randomized prefix pattern (e.g., `c4x_`, `el-`, `_s`), (c) never reuse the same naming scheme across builds. No two builds should share a class-naming pattern. |
| **REQ-LPB-036** | The compilation engine shall randomize all asset paths (filenames and directory structures) per build. | Static assets (CSS files, JS bundles, images, fonts) must have unique filenames and be served from unique directory paths. Examples: `/assets/main.css` in one build might be `/res/a7f3e2.css` in another, or `/static/styles/f829c.css` in a third. Directory depth and naming must vary. |
| **REQ-LPB-037** | The compilation engine shall inject randomized decoy content into each build. | Decoy content includes: (a) HTML comments with varied content (plausible code comments, TODO notes, version strings), (b) hidden/invisible DOM elements with varied content, (c) unused CSS rules, (d) no-op JavaScript statements or unused variables, (e) varied `<meta>` tags in `<head>`. Decoy volume and placement must be randomized per build. |
| **REQ-LPB-038** | The compilation engine shall vary its technical approach across builds where feasible. | Variation strategies: (a) alternate between inline styles, `<style>` blocks, and external CSS files, (b) vary between CSS Grid and Flexbox for layout, (c) vary JavaScript patterns (IIFE vs module vs inline), (d) vary HTML doctype/charset declarations where possible, (e) alternate between different CSS reset/normalize approaches. |
| **REQ-LPB-039** | The compiled Go backend shall randomize HTTP response headers per build. | Randomized headers include: (a) `Server` header (random or omitted), (b) `X-Powered-By` (random value or omitted), (c) custom decoy headers with plausible names/values, (d) `Cache-Control` and `Expires` values (varied but functionally equivalent), (e) header ordering. The header profile must be generated at build time and remain consistent for the lifetime of that build. |
| **REQ-LPB-040** | The compilation engine shall implement multiple distinct code generation strategies (templates). | The engine must maintain at least 3 fundamentally different code generation templates. Each template produces a different overall application structure (e.g., SPA with hash routing vs. multi-file with server routing vs. single inlined HTML page). The engine randomly selects a template at build time, then applies all other randomization on top of it. |
| **REQ-LPB-041** | The anti-fingerprinting system shall log all randomization decisions made during a build. | The build log must record: selected template, class-naming scheme, asset path scheme, decoy injection summary, header profile, and DOM variation strategy. This log is stored in the database associated with the campaign for forensic review. |

### 4.2 Anti-Fingerprinting Acceptance Criteria

- [ ] Two consecutive builds of the same page definition produce HTML output where no common substring of 200+ characters exists (excluding actual user-visible content).
- [ ] CSS class names share zero overlap between two builds of the same project.
- [ ] Asset file paths (URLs) share zero overlap between two builds of the same project.
- [ ] HTTP response headers differ in at least 3 header names or values between two builds of the same project.
- [ ] A YARA rule written to match Build A does not match Build B when both are built from the same page definition.
- [ ] Decoy content quantity varies by at least 20% between consecutive builds.
- [ ] Build logs capture all randomization parameters for every build.

### 4.3 Cline Delegation — Anti-Fingerprinting Implementation

The following anti-fingerprinting tasks are candidates for Cline delegation. Each includes a ready-to-paste prompt.

---

**Cline Task AF-1: HTML DOM Randomization Engine**

```
CONTEXT: You are implementing the HTML DOM randomization engine for Tackle's
landing page compilation pipeline. The compilation engine takes a JSON page
definition (component tree with styles and properties) and produces HTML output.

TASK: Implement the DOM randomization layer that sits between the page
definition parser and the HTML serializer. This layer must:

1. Wrap content elements in randomized neutral container elements (<div>,
   <section>, <article>, <main>, <aside>, <span>) with configurable nesting
   depth (1-4 extra levels).
2. Randomize the ordering of attributes on every HTML element.
3. Randomize the ordering of <meta>, <link>, and <script> tags in <head>.
4. Insert semantically neutral wrapper elements at random points in the DOM
   tree (ensure they do not affect visual layout — use CSS to maintain
   rendering).
5. Vary whitespace and indentation patterns in the serialized HTML.

INPUT: A parsed component tree (Go struct) representing the page.
OUTPUT: An HTML AST (or string) with randomized structure.

CONSTRAINTS:
- Must not alter visual rendering of the page.
- Must be deterministic given a seed (use a seeded PRNG so builds are
  reproducible if the same seed is provided).
- Must produce measurably different output across different seeds.
- Go implementation, no external dependencies beyond the standard library.
- Include unit tests that verify: (a) visual equivalence (same text content),
  (b) structural divergence (different DOM structure) across 10 different seeds.

FILE LOCATION: internal/compiler/randomizer/dom_randomizer.go
TEST LOCATION: internal/compiler/randomizer/dom_randomizer_test.go
```

---

**Cline Task AF-2: CSS Class Name Randomization Engine**

```
CONTEXT: You are implementing the CSS class name randomization engine for
Tackle's landing page compilation pipeline.

TASK: Implement a class name generator that produces unique, unpredictable
CSS class names for every build. This generator must:

1. Accept a seed value and produce a deterministic (but unique) set of class
   names for that seed.
2. Generate class names of variable length (4-12 characters).
3. Randomly select a naming convention per build:
   - All lowercase (e.g., "axqfm")
   - Camel-like (e.g., "kRtPx")
   - Hyphenated prefix (e.g., "el-a7f3")
   - Underscore prefix (e.g., "_sK29x")
   - BEM-like (e.g., "blk__elem--mod" with random block/element/modifier strings)
4. Maintain a mapping from logical component names to generated class names.
5. Apply the mapping consistently: update both HTML class attributes and CSS
   selectors in a single pass.

INPUT: Logical class names (from the page definition) + seed.
OUTPUT: A bidirectional mapping (logical -> generated) and functions to
transform HTML and CSS strings using the mapping.

CONSTRAINTS:
- Generated names must be valid CSS identifiers (no leading digits, no
  special characters except hyphens and underscores).
- No two logical names may map to the same generated name within a build.
- No two builds (different seeds) should produce the same mapping.
- Go implementation, standard library only.
- Include unit tests verifying uniqueness across 100 different seeds and
  validity of all generated names as CSS identifiers.

FILE LOCATION: internal/compiler/randomizer/css_randomizer.go
TEST LOCATION: internal/compiler/randomizer/css_randomizer_test.go
```

---

**Cline Task AF-3: Asset Path Randomization Engine**

```
CONTEXT: You are implementing the asset path randomization engine for
Tackle's landing page compilation pipeline. Compiled landing pages serve
static assets (CSS, JS, images, fonts) via the Go backend.

TASK: Implement an asset path generator that produces unique file paths and
directory structures for every build:

1. Generate random filenames for each asset (variable length, alphanumeric,
   with appropriate extensions).
2. Generate a random directory structure per build:
   - Vary root directory name (e.g., "assets", "static", "res", "pub", "dist",
     "media", "files", or random string).
   - Vary subdirectory depth (0-3 levels).
   - Vary subdirectory names.
3. Maintain a mapping from logical asset references to generated paths.
4. Update all references in HTML and CSS to use the generated paths.
5. Generate the Go route registrations for serving each asset at its
   randomized path.

INPUT: List of logical asset references + seed.
OUTPUT: Asset path mapping + updated HTML/CSS + Go route registration code.

CONSTRAINTS:
- Paths must be valid URL paths (no special characters beyond alphanumeric,
  hyphens, underscores, forward slashes, and dots).
- File extensions must be preserved (.css, .js, .png, etc.).
- Go implementation, standard library only.
- Include unit tests verifying: path validity, uniqueness across seeds,
  correct reference updates in sample HTML/CSS, and valid Go route output.

FILE LOCATION: internal/compiler/randomizer/asset_randomizer.go
TEST LOCATION: internal/compiler/randomizer/asset_randomizer_test.go
```

---

**Cline Task AF-4: Decoy Content Injection Engine**

```
CONTEXT: You are implementing the decoy content injection engine for Tackle's
landing page compilation pipeline. Decoy content is non-functional content
injected to make each build's source code unique when analyzed by defenders.

TASK: Implement a decoy injection system that adds plausible but non-functional
content to each build:

1. HTML comments: Inject 5-20 comments at random positions in the DOM.
   Content should be plausible developer comments (generate from a pool of
   templates: TODO notes, version annotations, developer names, date stamps,
   section markers, lint suppression markers).
2. Hidden DOM elements: Inject 3-10 invisible elements (display:none,
   visibility:hidden, off-screen positioning, aria-hidden, or zero-size).
   Content should be plausible (lorem ipsum, form fields, navigation links).
3. Unused CSS rules: Inject 10-30 CSS rules that target non-existent
   selectors. Rules should be plausible (common property sets, varied
   selectors).
4. JavaScript decoys: Inject 3-10 no-op or dead-code JS statements (unused
   variables with plausible names, empty function declarations, commented
   code blocks, console.debug statements wrapped in false conditionals).
5. Meta tag variation: Add/vary 3-8 meta tags (viewport variations, theme
   color, description, generator, robots with varied values).

INPUT: Compiled HTML/CSS/JS + seed.
OUTPUT: Modified HTML/CSS/JS with injected decoy content.

CONSTRAINTS:
- Decoy content must not alter visual rendering or functional behavior.
- Injection quantity and placement must be randomized per build.
- Decoy content must be plausible enough to resist simple "strip known
  decoys" filtering.
- Go implementation, standard library only.
- Include unit tests verifying: no visual impact (content text unchanged),
  measurable output difference across seeds, decoy count within specified
  ranges.

FILE LOCATION: internal/compiler/randomizer/decoy_injector.go
TEST LOCATION: internal/compiler/randomizer/decoy_injector_test.go
```

---

**Cline Task AF-5: HTTP Response Header Randomization**

```
CONTEXT: You are implementing HTTP response header randomization for Tackle's
compiled landing page Go backends. Each compiled landing page app is a Go
HTTP server. Its response headers must be unique per build to resist
header-based fingerprinting.

TASK: Implement a header profile generator and middleware:

1. At build time, generate a "header profile" — a deterministic set of
   HTTP response headers for the build:
   - Server header: randomly selected from a pool of plausible values
     ("nginx/1.x.x", "Apache/2.x.x", "cloudflare", "Microsoft-IIS/10.0",
     or omitted entirely). Version numbers should vary.
   - X-Powered-By: randomly selected ("Express", "ASP.NET", "PHP/x.x",
     "Next.js", or omitted).
   - Cache-Control: varied but functionally similar values.
   - X-Content-Type-Options, X-Frame-Options, X-XSS-Protection: include
     or omit randomly (these are security headers defenders might check).
   - Custom decoy headers: 1-3 plausible custom headers (e.g.,
     "X-Request-ID", "X-Correlation-ID", "X-Trace", "X-Edge-Location")
     with randomized values.
   - Header ordering: randomize the order headers are written.
2. Generate a Go middleware function that applies this header profile to
   every HTTP response.
3. The profile is baked into the compiled binary (not recalculated per
   request).

INPUT: Seed value.
OUTPUT: Go source code for the header middleware.

CONSTRAINTS:
- Headers must be valid HTTP headers.
- The same build seed must produce the same header profile (deterministic).
- Different seeds must produce measurably different profiles.
- Go implementation, standard library net/http only.
- Include unit tests verifying: header validity, determinism (same seed =
  same output), divergence (different seeds = different output), and that
  the middleware correctly applies headers to HTTP responses.

FILE LOCATION: internal/compiler/randomizer/header_randomizer.go
TEST LOCATION: internal/compiler/randomizer/header_randomizer_test.go
```

---

**Cline Task AF-6: Template Diversity — Multiple Code Generation Strategies**

```
CONTEXT: You are implementing the template diversity system for Tackle's
landing page compilation engine. The engine must support multiple fundamentally
different code generation strategies so that builds are structurally diverse
at the application architecture level — not just at the superficial
HTML/CSS level.

TASK: Design and implement the template strategy interface and at least 3
concrete strategy implementations:

STRATEGY 1 — Single-Page Application (SPA):
- All pages compiled into a single HTML file.
- Client-side hash routing (#/page1, #/page2).
- All CSS inlined in <style> tags.
- All JS inlined in <script> tags.
- Go backend serves the single HTML file + handles form POSTs.

STRATEGY 2 — Multi-File Server-Routed:
- Each page is a separate HTML file.
- Go backend serves each page at its own route (/login, /success, etc.).
- CSS in external stylesheet(s).
- JS in external script file(s).
- Server-side redirects between pages.

STRATEGY 3 — Hybrid Inlined:
- Single entry HTML file with JS-driven page transitions (no hash routing —
  uses DOM manipulation to swap page content).
- CSS split: critical CSS inlined, non-critical in external file.
- JS bundled into a single external file.
- Go backend serves HTML + static assets + handles form POSTs.

Each strategy must:
- Accept the same JSON page definition as input.
- Produce a functional set of files + Go backend code.
- Integrate with all other randomization engines (DOM, CSS, assets, decoys,
  headers).

INTERFACE: Define a Go interface `CodeGenerator` with method
  `Generate(definition PageDefinition, seed int64) (*BuildOutput, error)`

CONSTRAINTS:
- Each strategy must produce visually identical results for the same page
  definition.
- Strategies must be selectable randomly at build time (default) or
  manually by the operator.
- Go implementation, standard library only (+ React for frontend where
  applicable).
- Include integration tests that compile the same page definition with
  each strategy and verify: (a) functional equivalence (same pages, same
  forms, same navigation), (b) structural divergence (fundamentally
  different file layouts and code patterns).

FILE LOCATION: internal/compiler/strategy/strategy.go (interface)
                internal/compiler/strategy/spa.go
                internal/compiler/strategy/multifile.go
                internal/compiler/strategy/hybrid.go
TEST LOCATION:  internal/compiler/strategy/strategy_test.go
```

## 5. Compilation & Hosting

### 5.1 Compilation Engine

| ID | Requirement | Details |
|----|-------------|---------|
| **REQ-LPB-042** | The framework shall include a compilation engine that transforms a JSON page definition into a standalone React frontend + Go backend application. | The compilation engine runs on the framework server. It is invoked via the API when an operator triggers a build. |
| **REQ-LPB-043** | The compilation engine shall accept as input: (a) the JSON page definition, (b) an optional seed value (auto-generated if omitted), (c) an optional template strategy override, and (d) campaign-specific configuration (tracking tokens, capture endpoint behavior, redirect URLs). | All inputs are validated before compilation begins. |
| **REQ-LPB-044** | The compilation engine shall produce as output: (a) a compiled Go binary embedding the React frontend as static assets, (b) a build manifest (JSON) listing all generated files, paths, and randomization decisions, and (c) a build log. | The Go binary is a single self-contained executable with no external dependencies. |
| **REQ-LPB-045** | The compilation engine shall compile the Go binary using `go build` with the target OS/architecture matching the framework server. | Cross-compilation is not required in the initial implementation. The binary targets the same OS/arch as the framework server (linux/amd64). |
| **REQ-LPB-046** | The compilation engine shall embed the React frontend into the Go binary using Go's `embed` package. | No external file serving — all static assets are embedded in the binary. |
| **REQ-LPB-047** | The compilation pipeline shall complete within 60 seconds for a standard landing page project (up to 5 pages, 20 components, 5 form fields). | Build time is measured from API call to binary ready. Performance budgets must be validated during testing. |
| **REQ-LPB-048** | The compilation engine shall integrate the credential capture backend into the generated Go binary. | The generated backend handles form POST submissions, extracts field values by capture tag, and forwards them to the framework server's credential capture API (see [08-credential-capture.md](08-credential-capture.md)). |
| **REQ-LPB-049** | The generated Go backend shall embed a tracking pixel endpoint that records page visits. | The tracking pixel URL is unique per build. Visit data (timestamp, IP, user-agent, tracking token) is forwarded to the framework server. |

### 5.2 Application Hosting

| ID | Requirement | Details |
|----|-------------|---------|
| **REQ-LPB-050** | The framework shall host compiled landing page applications on the framework server, each listening on a randomly selected unused port. | Port selection must check availability before binding. Ports must be in a configurable range (default: 10000-60000). |
| **REQ-LPB-051** | The framework shall support running multiple landing page applications concurrently (one per active campaign). | Resource limits must be enforced: maximum concurrent applications is configurable (default: 20). |
| **REQ-LPB-052** | The framework shall manage the full lifecycle of each landing page application. | Lifecycle states: `built` -> `starting` -> `running` -> `stopping` -> `stopped` -> `cleaned_up`. State transitions are logged. |
| **REQ-LPB-053** | The framework shall monitor the health of each running landing page application via periodic HTTP health checks. | Health check interval: configurable (default: 30 seconds). An application that fails 3 consecutive health checks is flagged as unhealthy and an alert is generated. |
| **REQ-LPB-054** | The framework shall automatically restart a landing page application that crashes or becomes unhealthy, up to a configurable retry limit (default: 3). | Each restart is logged with the failure reason. After the retry limit is exhausted, the application enters a `failed` state and an alert is sent. |
| **REQ-LPB-055** | The framework shall clean up all resources when a landing page application is stopped: (a) kill the process, (b) release the port, (c) delete the binary from disk. | The compiled binary and any temporary build artifacts are removed. The page definition and build logs are retained in the database. |
| **REQ-LPB-056** | The framework shall expose the assigned port of a running landing page application to the phishing endpoint configuration module. | The phishing endpoint's reverse proxy is configured to forward traffic to `localhost:{assigned_port}` on the framework server. |

### 5.3 Generated Application Communication

| ID | Requirement | Details |
|----|-------------|---------|
| **REQ-LPB-057** | The generated landing page application shall communicate with the framework server over an internal HTTP API (localhost only). | Communication is not exposed outside the framework server. No authentication is required for localhost-only communication, but a shared build-specific token must be included in all requests as a header (`X-Build-Token`). |
| **REQ-LPB-058** | The generated application shall forward captured credentials to the framework server via `POST /api/v1/internal/captures`. | Payload: `{ "campaign_id": "...", "build_token": "...", "fields": { "username": "...", "password": "..." }, "metadata": { "ip": "...", "user_agent": "...", "timestamp": "..." } }` |
| **REQ-LPB-059** | The generated application shall forward tracking pixel hits to the framework server via `POST /api/v1/internal/tracking`. | Payload: `{ "campaign_id": "...", "build_token": "...", "tracking_token": "...", "event": "page_view", "metadata": { ... } }` |
| **REQ-LPB-060** | The generated application shall forward JavaScript-collected data (keystrokes, clipboard, browser fingerprints) to the framework server via `POST /api/v1/internal/telemetry`. | Payload structure is flexible — the framework stores it as JSON associated with the campaign and tracking token. |

## 5A. HTML Import and Raw Code Editor

### 5A.1 HTML Import Modes

| ID | Requirement | Details |
|----|-------------|---------|
| **REQ-LPB-061** | The builder SHALL support importing existing HTML files into the visual builder. | Imported HTML is parsed and converted into the builder's component model. Elements that cannot be mapped to builder components are preserved as raw HTML blocks. |
| **REQ-LPB-062** | The builder SHALL provide a raw code editor mode as an alternative to the visual drag-and-drop interface. | The raw code editor provides direct access to the page's HTML, CSS, and JavaScript with syntax highlighting and validation. Changes in the raw editor update the visual builder and vice versa. |
| **REQ-LPB-063** | The HTML import function SHALL accept single HTML files (up to 5 MB), ZIP archives containing HTML with associated assets (CSS, JS, images — up to 50 MB total), and copy-pasted HTML source code. | Assets referenced in imported HTML are extracted and stored in the project. Relative paths are preserved. |
| **REQ-LPB-064** | The system SHALL support both import-to-builder (parsed into visual components) and import-as-raw (preserved as raw HTML blocks within the builder). | The operator selects the import mode during the import dialog. Import-to-builder mode performs best-effort mapping; unmapped elements become raw HTML blocks. |

Acceptance Criteria:
- [ ] HTML files are successfully parsed and rendered in the visual builder
- [ ] The raw code editor supports syntax highlighting for HTML, CSS, and JavaScript
- [ ] Changes in the raw editor are reflected in the visual builder canvas within 2 seconds
- [ ] ZIP imports correctly extract and reference associated assets
- [ ] The import dialog clearly explains the two import modes and their trade-offs

### 5A.2 Iframe and Embedded Content Support

| ID | Requirement | Details |
|----|-------------|---------|
| **REQ-LPB-065** | The builder SHALL support embedding iframe elements that load external content within the landing page. | Iframes are available as a component in the builder's component library under the "Media" category. |
| **REQ-LPB-066** | Iframe components SHALL support configurable properties: source URL, width, height, sandbox attributes, allow attributes (camera, microphone, etc.), and border/styling options. | Source URLs can be static or dynamically constructed using template variables. |
| **REQ-LPB-067** | The compiled landing page SHALL support transparent iframe overlays for credential interception scenarios. | The iframe can be positioned as a full-page overlay or targeted overlay on specific page elements. Opacity and pointer-event settings are configurable. |

Acceptance Criteria:
- [ ] Iframe components render correctly in both the builder preview and the compiled landing page
- [ ] Iframe sandbox attributes are configurable to control embedded content capabilities
- [ ] Transparent iframe overlays can be positioned over form elements for credential interception
- [ ] Iframe source URLs support variable substitution

### 5A.3 Page Cloning (Visual Clone)

| ID | Requirement | Details |
|----|-------------|---------|
| **REQ-LPB-068** | The builder SHALL support visually cloning an existing public web page to use as a starting point for a landing page project. | The clone function fetches the target URL, extracts the HTML/CSS/assets, and imports them into the builder. |
| **REQ-LPB-069** | The visual clone function SHALL capture: the page's HTML structure, inline and external CSS styles, images and media assets, favicon, and page metadata (title, meta tags). | JavaScript is optionally included (operator toggle). External API calls and tracking scripts in the original page are stripped by default. |
| **REQ-LPB-070** | The cloned content SHALL be imported into the builder as editable components where possible, with complex or unmappable elements preserved as raw HTML blocks. | The operator can edit, rearrange, and extend the cloned content using the full builder feature set. |
| **REQ-LPB-071** | The clone function SHALL rewrite all asset references (images, CSS, fonts) to point to locally stored copies, eliminating external dependencies. | No requests to the original site are made when the compiled landing page is served to targets. |

Acceptance Criteria:
- [ ] A URL input triggers the clone process and displays a preview before committing
- [ ] Cloned pages render visually identical to the original in the builder preview
- [ ] All external assets are downloaded and stored locally within the project
- [ ] The clone process completes within 30 seconds for standard web pages
- [ ] The operator can edit any element of the cloned page in the visual builder

### 5A.4 Evilginx-Style Reverse Proxy Mode (v2 Design Note)

> **v2 Feature — Design Note Only**
>
> A future version of Tackle will support an evilginx-style transparent reverse proxy mode where the phishing endpoint acts as a man-in-the-middle proxy to a real target application. In this mode:
>
> - The landing page is NOT built with the page builder. Instead, the phishing endpoint proxies all requests to the real target application (e.g., login.microsoft.com) in real time.
> - The proxy intercepts and captures session tokens, cookies, OAuth tokens, and credentials as they flow between the target's browser and the real application.
> - This enables full session hijacking, not just credential capture.
> - The v1 data model and endpoint architecture should not preclude this mode. Specifically:
>   - The phishing endpoint entity should support a `mode` field (values: `proxy_to_framework` for v1, `reverse_proxy_to_target` for v2).
>   - The credential capture schema should support storing session tokens and cookies (see [08-credential-capture.md](08-credential-capture.md)).
>   - The landing page association on a campaign should be nullable to support campaigns that use reverse proxy mode instead of a built landing page.
>
> Implementation of this feature is deferred to v2. No v1 code needs to implement the reverse proxy mode itself, but schema and data model decisions should accommodate it.

## 6. API Endpoints

The following REST API endpoints support the Landing Page Builder:

| Method | Endpoint | Description |
|--------|----------|-------------|
| `GET` | `/api/v1/landing-pages` | List all landing page projects |
| `POST` | `/api/v1/landing-pages` | Create a new landing page project |
| `GET` | `/api/v1/landing-pages/{id}` | Get a landing page project (including full JSON definition) |
| `PUT` | `/api/v1/landing-pages/{id}` | Update a landing page project |
| `DELETE` | `/api/v1/landing-pages/{id}` | Delete a landing page project |
| `POST` | `/api/v1/landing-pages/{id}/build` | Trigger compilation of a landing page project |
| `GET` | `/api/v1/landing-pages/{id}/builds` | List all builds for a project |
| `GET` | `/api/v1/landing-pages/{id}/builds/{build_id}` | Get build details (manifest, logs, status) |
| `POST` | `/api/v1/landing-pages/{id}/builds/{build_id}/start` | Start a compiled landing page application |
| `POST` | `/api/v1/landing-pages/{id}/builds/{build_id}/stop` | Stop a running landing page application |
| `GET` | `/api/v1/landing-pages/{id}/builds/{build_id}/health` | Get health status of a running application |
| `GET` | `/api/v1/landing-pages/templates` | List available project templates |
| `POST` | `/api/v1/landing-pages/templates` | Save current project as a template |
| `GET` | `/api/v1/landing-pages/components` | List available builder components (for the UI) |
| `POST` | `/api/v1/landing-pages/{id}/preview` | Generate a preview render (returns HTML) |
| `POST` | `/api/v1/landing-pages/{id}/import` | Import HTML file or ZIP archive into a project |
| `POST` | `/api/v1/landing-pages/{id}/clone-url` | Clone a public web page by URL |
| `POST` | `/api/v1/landing-pages/{id}/duplicate` | Duplicate an existing landing page project |

## 7. Database Entities

The following entities support the Landing Page Builder (detailed schema in [14-database-schema.md](14-database-schema.md)):

| Entity | Key Fields | Purpose |
|--------|------------|---------|
| `landing_page_projects` | `id`, `name`, `description`, `definition_json`, `created_by`, `created_at`, `updated_at` | Stores the page builder project and its JSON definition. |
| `landing_page_builds` | `id`, `project_id`, `campaign_id`, `seed`, `strategy`, `build_manifest_json`, `build_log`, `binary_path`, `status`, `port`, `build_token`, `created_at` | Stores each compilation build and its runtime state. |
| `landing_page_templates` | `id`, `name`, `description`, `definition_json`, `created_by`, `is_shared`, `created_at` | Stores reusable project templates. |
| `landing_page_health_checks` | `id`, `build_id`, `status`, `response_time_ms`, `checked_at` | Stores health check history for running applications. |

## 8. Security Considerations

| Concern | Mitigation |
|---------|------------|
| **Binary execution** | Compiled landing page binaries are generated from controlled input (the JSON page definition). The compilation engine must sanitize all inputs before embedding them in generated code. No arbitrary code execution beyond the operator-provided JavaScript snippets. |
| **Port binding** | Landing page apps bind to localhost only. Only the framework's reverse proxy (and by extension, the phishing endpoint) can route traffic to them. Firewall rules should block direct external access to the landing page port range. |
| **Build token exposure** | Build tokens are used for internal framework-to-app communication only. They must not appear in any content served to targets (HTML, JS, headers). |
| **Operator JavaScript** | Operators can inject arbitrary JavaScript. This is by design (phishing functionality requires it). However, injected JS must not be able to access the internal framework API — the build token is not exposed to the frontend. |
| **Resource exhaustion** | Concurrent app limit, process monitoring, and automatic cleanup prevent runaway resource usage. Each app process should have memory and CPU limits enforced by the framework (e.g., via OS-level cgroups or process resource limits). |
| **Build artifacts** | Compiled binaries are deleted on cleanup. Build logs and manifests are retained in the database. Temporary build directories are cleaned up immediately after compilation completes (success or failure). |

## 9. Acceptance Criteria

### Builder UI
- [ ] Operators can create a new landing page project and design pages using drag-and-drop.
- [ ] All component categories listed in REQ-LPB-007 are available in the component library.
- [ ] Components can be nested inside layout components, and the component tree is accurately displayed.
- [ ] Form fields are configurable with capture tags that map to credential capture categories.
- [ ] Multi-step forms work correctly in preview mode.
- [ ] Multi-page projects support at least 10 pages with configurable navigation flows.
- [ ] CSS styling can be applied via visual controls and raw CSS, with raw CSS taking precedence.
- [ ] Theme presets apply consistent styling across all components.
- [ ] Custom JavaScript can be injected at project, page, and component levels.
- [ ] Preview mode renders pages accurately and supports viewport switching.
- [ ] Undo/redo works for all canvas operations.

### HTML Import, Raw Code Editor, and Page Cloning
- [ ] HTML files can be imported into the builder via file upload, ZIP archive, or paste.
- [ ] The raw code editor supports HTML, CSS, and JS with syntax highlighting and bidirectional sync with the visual builder.
- [ ] Iframe components are available and configurable with sandbox/allow attributes.
- [ ] Visual page cloning successfully captures and localizes a target web page's appearance.
- [ ] Cloned pages have no external dependencies to the original site.

### Anti-Fingerprinting
- [ ] All acceptance criteria in Section 4.2 are met.
- [ ] Build logs record all randomization decisions.
- [ ] Operators can optionally specify a seed for reproducible builds.
- [ ] Operators can optionally override the template strategy selection.

### Compilation & Hosting
- [ ] A standard project compiles to a working binary within 60 seconds.
- [ ] The compiled binary serves the landing page correctly when started.
- [ ] Credential capture form submissions are forwarded to the framework server.
- [ ] Tracking pixel hits are forwarded to the framework server.
- [ ] Multiple landing page apps can run concurrently on different ports.
- [ ] Health checks detect unhealthy applications and trigger restarts.
- [ ] Stopped applications are fully cleaned up (process killed, port released, binary deleted).
- [ ] The assigned port is correctly exposed to the phishing endpoint configuration.

### API
- [ ] All API endpoints listed in Section 6 are implemented and return correct responses.
- [ ] API endpoints enforce RBAC permissions (see [02-authentication-authorization.md](02-authentication-authorization.md)).
- [ ] Build and lifecycle operations produce audit log entries (see [11-audit-logging.md](11-audit-logging.md)).

## 10. Dependencies

| Dependency | Document | Nature |
|------------|----------|--------|
| Authentication & RBAC | [02-authentication-authorization.md](02-authentication-authorization.md) | Builder access controlled by role permissions. |
| Campaign Management | [06-campaign-management.md](06-campaign-management.md) | Landing pages are associated with campaigns. Builds are triggered in campaign context. |
| Phishing Endpoints | [07-phishing-endpoints.md](07-phishing-endpoints.md) | Endpoints proxy traffic to hosted landing page apps. |
| Credential Capture | [08-credential-capture.md](08-credential-capture.md) | Generated apps forward captured credentials to the capture pipeline. |
| Metrics & Reporting | [10-metrics-reporting.md](10-metrics-reporting.md) | Tracking data feeds into campaign metrics. |
| Audit Logging | [11-audit-logging.md](11-audit-logging.md) | All builder and lifecycle actions are logged. |
| Database Schema | [14-database-schema.md](14-database-schema.md) | Entity definitions for landing page tables. |
| Frontend Architecture | [16-frontend-architecture.md](16-frontend-architecture.md) | Builder UI is part of the Admin UI React application. |
