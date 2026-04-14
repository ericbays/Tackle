# 08 — Server Generation (servergen) Pipeline

## 8.1 Overview

The `servergen` pipeline is the compilation system that transforms an operator's builder configuration (the JSON definition) into a standalone Go binary. This binary is a complete web application — it serves React pages, handles form submissions, captures data, reports telemetry, serves assets, and executes navigation flows. The pipeline replaces the previous `gogen`, `htmlgen`, and `reactgen` modules with a unified code generation system.

## 8.2 Pipeline Architecture

```
                    ┌─────────────────────┐
                    │   Builder JSON       │
                    │   Definition         │
                    │   (from database)    │
                    └──────────┬──────────┘
                               │
                               ▼
                    ┌─────────────────────┐
                    │  1. VALIDATION       │
                    │                      │
                    │  Schema validation   │
                    │  Component tree      │
                    │  Navigation rules    │
                    │  Capture config      │
                    │  Asset references    │
                    └──────────┬──────────┘
                               │
                               ▼
                    ┌─────────────────────┐
                    │  2. WORKSPACE        │
                    │     GENERATION       │
                    │                      │
                    │  Create temp dir     │
                    │  Initialize go.mod   │
                    │  Scaffold structure  │
                    └──────────┬──────────┘
                               │
              ┌────────────────┼────────────────┐
              ▼                ▼                 ▼
   ┌──────────────┐ ┌──────────────┐  ┌──────────────┐
   │ 3a. FRONTEND  │ │ 3b. BACKEND  │  │ 3c. ASSETS   │
   │   GENERATION  │ │   GENERATION │  │   STAGING    │
   │               │ │              │  │              │
   │ React comps   │ │ main.go      │  │ Images       │
   │ Page routing  │ │ handlers.go  │  │ Fonts        │
   │ Behavioral JS │ │ capture.go   │  │ Payloads     │
   │ CSS bundle    │ │ telemetry.go │  │ Favicons     │
   │ esbuild       │ │ server.go    │  │              │
   └──────┬───────┘ └──────┬───────┘  └──────┬───────┘
          │                │                  │
          └────────────────┼──────────────────┘
                           │
                           ▼
                ┌─────────────────────┐
                │  4. EMBEDDING       │
                │                     │
                │  go:embed for all   │
                │  static assets,     │
                │  JS/CSS bundles,    │
                │  payload files      │
                └──────────┬──────────┘
                           │
                           ▼
                ┌─────────────────────┐
                │  5. COMPILATION     │
                │                     │
                │  go build           │
                │  → standalone       │
                │    binary           │
                └──────────┬──────────┘
                           │
                           ▼
                ┌─────────────────────┐
                │  6. CLEANUP         │
                │                     │
                │  Remove workspace   │
                │  Record manifest    │
                │  Store binary       │
                └─────────────────────┘
```

## 8.3 Stage 1: Validation

Before any code generation begins, the pipeline validates the entire definition.

### Schema Validation

- `schema_version` is a supported version
- `pages` array is non-empty and within the maximum page limit (50)
- `global_styles` and `global_js` are valid strings (not objects or arrays)
- `navigation` array contains valid flow definitions

### Component Tree Validation

For each page:
- Component tree nesting depth does not exceed 20
- All `component_id` values are unique within the project
- All `type` values are recognized component types
- Container components have `children` arrays; leaf components do not
- Forms are not nested inside other forms
- Required properties are present for each component type

### Navigation Validation

- All `source_page` references point to existing pages
- All `target_page` references point to existing pages or are valid external URLs
- `trigger_target` DOM IDs exist on the referenced source page (warning, not error)
- No infinite timer/page_load loops without user interaction

### Capture Validation

- Form components have valid `action` paths
- Action paths are unique across the project (two forms cannot POST to the same path, unless intentional)
- Input components within forms have `name` attributes
- Capture tags are valid values

### Asset Validation

- All `asset://` references resolve to assets that exist in the project's asset store
- Asset total size is within limits

### Validation Output

Validation produces either:
- **Success**: Proceed to workspace generation
- **Errors**: Hard stop — invalid definition cannot be compiled (e.g., missing required properties, invalid schema)
- **Warnings**: Non-blocking issues logged in the build manifest (e.g., orphaned pages, missing DOM IDs for flow triggers)

## 8.4 Stage 2: Workspace Generation

Creates a temporary build directory with the Go project structure:

```
workspace/
├── go.mod                  // Module definition (no external imports)
├── main.go                 // Entry point, server setup, registration
├── server.go               // HTTP server, routing, middleware
├── handlers.go             // Page serving and form capture handlers
├── capture.go              // Capture processing and upstream forwarding
├── telemetry.go            // Metrics collection and upstream reporting
├── navigation.go           // Flow evaluation and routing logic
├── static/                 // Embedded static assets
│   ├── index.js            // Bundled React application
│   ├── index.css           // Bundled CSS
│   ├── favicon.ico         // (if configured)
│   └── assets/             // Uploaded images, fonts, files
│       ├── a1b2c3d4.png
│       ├── e5f6a7b8.woff2
│       └── ...
└── embed.go                // go:embed directives
```

### go.mod

```go
module landing-app

go 1.23
```

No external dependencies. The generated application uses only the Go standard library.

## 8.5 Stage 3a: Frontend Generation

### React Component Generation

The pipeline transforms the component tree into React JSX source code:

1. **Page Components**: Each page becomes a React component that renders its component tree
2. **Component Mapping**: Each builder component type maps to a React element (see component system doc)
3. **Style Application**: Component `style` properties are rendered as inline styles
4. **Event Binding**: Click actions, form submissions, and behavioral capabilities are wired to JavaScript handlers
5. **Client-Side Router**: A React router component manages page navigation based on URL paths

### Behavioral JavaScript Generation

For each page that has behavioral capabilities enabled:

1. **Page-level behaviors**: Generate initialization code that runs on page load (fingerprinting, session extraction, page view tracking, etc.)
2. **Component-level behaviors**: Generate event listeners attached to specific DOM elements (keystroke capture, clipboard capture, etc.)
3. **Metrics sender**: Generate a utility function that POSTs behavioral data to the landing app's internal metrics endpoint

All behavioral JavaScript is bundled into the main React application bundle — not loaded as separate scripts.

### CSS Generation

1. **Global styles**: The operator's `global_styles` CSS is included as-is
2. **Page styles**: Each page's `page_styles` CSS is scoped and included
3. **Component styles**: Inline styles from the component tree are rendered as inline style attributes (not extracted into CSS)
4. **Font faces**: `@font-face` rules for uploaded custom fonts

### esbuild Transpilation

The generated React JSX source is transpiled and bundled using esbuild:

- **Input**: JSX source files
- **Output**: Single ES module bundle (`index.js`) + CSS bundle (`index.css`)
- **Externals**: React and ReactDOM are loaded from a CDN import map (reducing binary size)
- **Minification**: Enabled in production builds, disabled in development builds
- **Source maps**: Included in development builds, excluded in production

## 8.6 Stage 3b: Backend Generation

### main.go

The entry point for the compiled binary:

```
1. Parse command-line flags (Tackle host URL, development mode flag)
2. Request available ports from the OS (net.Listen on :0)
3. Start HTTP server on the allocated port
4. Register with Tackle (POST registration webhook with port, metadata)
5. Start heartbeat goroutine (periodic POST to Tackle)
6. Block until shutdown signal (SIGINT, SIGTERM, or heartbeat failure)
7. Graceful shutdown
```

### server.go

HTTP server setup and route registration:

```
Routes:
  GET  /                          → Serve React SPA (index.html with JS/CSS)
  GET  /{page-routes}             → Serve React SPA (client-side routing handles page)
  GET  /assets/{filename}         → Serve embedded static assets
  GET  /assets/download/{file_id} → Serve payload file downloads (tracked)
  POST /{form-action-paths}       → Form capture handlers (one per unique form action)
  POST /{metrics-endpoint}        → Internal metrics receiver (behavioral data from JS)
```

**Middleware**:
- Recovery (panic handler)
- Request logging (to stdout in dev, suppressed in production)
- CORS (configured for the phishing endpoint's domain)
- Content-Type enforcement

### handlers.go

One handler function is generated per unique form action path in the project:

```
For each Form component in the definition:
  Generate handler for POST {action_path}:
    1. Parse request body (form-urlencoded or JSON)
    2. Map field names to capture tags (based on builder config)
    3. Collect request metadata (IP, headers, User-Agent)
    4. Build CaptureEvent payload
    5. POST to Tackle: /api/v1/internal/captures
    6. POST to Tackle: /api/v1/internal/metrics (form_submission event)
    7. Evaluate navigation flows for this form
    8. Return navigation response (redirect URL, page route, or message)
```

**Page serving handler**:
```
For all GET requests that match page routes:
  Serve the React SPA HTML shell (with embedded JS/CSS references)
  React client-side router handles rendering the correct page
```

### capture.go

Shared capture utilities:

- `buildCapturePayload()`: Constructs the CaptureEvent from parsed form data and metadata
- `forwardCapture()`: POSTs the capture payload to Tackle's internal API
- `categorizeField()`: Applies capture tag rules (auto-detect + operator overrides)

### telemetry.go

Shared telemetry utilities:

- `reportMetric()`: POSTs a metric event to Tackle's internal API
- `handleBehavioralData()`: Handler for the internal metrics endpoint that receives behavioral data from the page's JavaScript and forwards it to Tackle

### navigation.go

Flow evaluation logic:

- `evaluateFlows()`: Given a page route, trigger type, and form data, evaluates all matching flows and returns the navigation target
- `evaluateConditions()`: Checks a flow's condition rules against form data
- `buildNavigationResponse()`: Constructs the appropriate HTTP response (redirect, JSON route instruction, etc.)

## 8.7 Stage 3c: Asset Staging

All project assets are fetched from the database and written to the workspace:

1. Query all assets associated with the project
2. Write each asset to `workspace/static/assets/{hash}.{extension}`
3. Write payload files to `workspace/static/assets/download/{hash}.{extension}`
4. Write favicons to `workspace/static/favicon.{extension}`

## 8.8 Stage 4: Embedding

Generate `embed.go` with Go embed directives:

```go
package main

import "embed"

//go:embed static/*
var staticFiles embed.FS
```

This embeds the entire `static/` directory (React bundles, CSS, images, fonts, payload files) into the compiled binary.

## 8.9 Stage 5: Compilation

Execute `go build` against the workspace:

```
GOOS={target_os} GOARCH={target_arch} go build -o landing-app{.exe} .
```

### Build Targets

| Target | Use Case |
|--------|----------|
| `linux/amd64` | Default — Tackle typically runs on Linux |
| `windows/amd64` | Windows Tackle deployments |
| `darwin/amd64` | macOS development |
| `darwin/arm64` | Apple Silicon development |

The target OS/architecture matches the Tackle server's platform (since the binary runs locally on the same host).

### Build Output

- **Binary path**: Stored in a build artifacts directory
- **Binary hash**: SHA-256 hash of the compiled binary (stored in build record)
- **Build size**: Logged for operator awareness

## 8.10 Stage 6: Cleanup and Recording

### Workspace Cleanup

The temporary build workspace is deleted after successful compilation. On failure, the workspace is preserved for debugging (configurable retention).

### Build Manifest

A manifest is recorded for each build:

```
BuildManifest {
    build_id          : string
    project_id        : string
    seed              : integer       // Random seed used for any procedural generation
    mode              : string        // "development" or "production"
    pages_count       : integer
    components_count  : integer
    forms_count       : integer
    assets_count      : integer
    assets_total_size : integer
    binary_size       : integer
    binary_hash       : string
    target_os         : string
    target_arch       : string
    build_duration_ms : integer
    warnings          : array         // Validation warnings
    timestamp         : datetime
}
```

### Build Record

The build is recorded in the database with:
- Status: `pending` → `building` → `built` (or `failed`)
- Binary path
- Binary hash
- Build manifest (JSONB)
- Build log (text — compilation output, warnings, errors)

## 8.11 Development vs. Production Builds

| Aspect | Development Build | Production Build |
|--------|------------------|-----------------|
| **Minification** | Disabled | Enabled |
| **Source maps** | Included | Excluded |
| **Logging** | Verbose to stdout | Minimal |
| **Registration** | Reports as dev instance | Reports as production instance |
| **Anti-fingerprinting** | None | Applied by evasion pipeline (separate system) |
| **Hot reload support** | Enabled (WebSocket endpoint) | Disabled |
| **Metrics reporting** | Tagged as dev | Tagged as production |

### Development-Only Additions

In development mode, the binary includes:

1. **WebSocket endpoint**: Accepts AST update pushes from Tackle for hot reload (see doc 09)
2. **Verbose logging**: Logs all requests, captures, and telemetry to stdout
3. **CORS permissiveness**: Allows requests from the Tackle admin frontend origin
4. **Unminified output**: React bundle is readable for debugging

## 8.12 Error Handling

### Compilation Failures

If `go build` fails:
- Build status is set to `failed`
- The full compiler error output is stored in the build log
- The workspace is preserved for inspection
- The operator is notified via the builder UI

### Runtime Failures

If the compiled binary crashes at runtime:
- The process exit code and stderr output are captured
- The build status is updated to `failed`
- Tackle's app manager detects the failure via health check
- The operator can view crash logs in the builder UI

### Graceful Degradation

If the binary cannot reach Tackle's internal API (captures/metrics endpoints):
- Captures and metrics are buffered in-memory (bounded queue)
- The binary retries with exponential backoff
- If the buffer fills, oldest entries are dropped (captures are prioritized over metrics)
- The binary does NOT stop serving the landing page — the target experience is unaffected

## 8.13 No External Dependencies

The generated Go source code uses only the Go standard library:

- `net/http` for the HTTP server
- `encoding/json` for JSON processing
- `html/template` for HTML rendering (if needed)
- `embed` for static file embedding
- `os/signal` for shutdown handling
- `net` for port allocation
- `crypto/sha256` for hashing
- `time` for timing
- `log` for logging
- `io`, `bytes`, `strings`, `fmt` for utilities

No third-party Go modules are imported. This ensures:
- No supply chain dependencies
- No network access needed during compilation (after initial Go toolchain setup)
- Minimal binary size
- No version compatibility issues
