# 09 — Development Server & Hot Reload

## 9.1 Overview

The development server allows operators to see their landing application running live as they build it. This is not a static preview or a rendering of the current page — it is the actual compiled landing application running with full backend functionality (form capture, navigation flows, telemetry). When the operator changes something in the builder, the change is reflected in the running application in real-time through a full hot-reload system that updates both the frontend (React) and backend (Go) components.

## 9.2 Architecture

```
┌─────────────────────────────────────────────────────────────┐
│                     TACKLE SERVER                            │
│                                                              │
│  ┌─────────────────┐         ┌─────────────────────────┐    │
│  │ Builder Frontend │         │ Landing App Dev Binary   │    │
│  │ (React SPA)      │         │                          │    │
│  │                  │         │  ┌───────────────────┐   │    │
│  │  Operator makes  │ ──(1)──│  │ Go Backend         │   │    │
│  │  change in       │        │  │ (Port A - OS       │   │    │
│  │  builder         │        │  │  assigned)          │   │    │
│  │                  │        │  │                     │   │    │
│  │                  │        │  │ - Form handlers     │   │    │
│  └────────┬─────────┘        │  │ - Capture logic     │   │    │
│           │                  │  │ - Asset serving      │   │    │
│           │ (2) POST         │  │ - Telemetry          │   │    │
│           │ definition       │  │ - HMR WebSocket      │   │    │
│           │ to Tackle API    │  │   endpoint           │   │    │
│           ▼                  │  └───────────────────┘   │    │
│  ┌─────────────────┐        │                           │    │
│  │ Tackle Backend   │        │  ┌───────────────────┐   │    │
│  │ (Go API)         │──(3)──│  │ React Dev Server   │   │    │
│  │                  │ push  │  │ (Port B - OS       │   │    │
│  │                  │ AST   │  │  assigned)          │   │    │
│  │                  │       │  │                     │   │    │
│  └─────────────────┘       │  │ - Serves React app  │   │    │
│                             │  │ - WebSocket for HMR │   │    │
│                             │  │ - Receives AST      │   │    │
│                             │  │   updates           │   │    │
│                             │  └───────────────────┘   │    │
│                             └─────────────────────────┘    │
│                                                              │
│  Operator views running app at:                              │
│    http://localhost:{Port A}  (full app with backend)        │
│    http://localhost:{Port B}  (React frontend only, HMR)     │
└─────────────────────────────────────────────────────────────┘
```

## 9.3 Dual-Port Architecture

Each development landing application runs on two ports, both requested from the OS (no pre-assigned ports):

### Port A — Application Backend

The full Go backend server:
- Serves the React application (HTML shell + bundled JS/CSS)
- Handles form POST endpoints (with capture and telemetry)
- Serves embedded assets
- Evaluates navigation flows
- Reports captures and metrics to Tackle
- Exposes a WebSocket endpoint for receiving AST updates from Tackle

### Port B — React Dev Server

A lightweight development server for the React frontend:
- Serves the React application with HMR (Hot Module Replacement) support
- Maintains WebSocket connections with the browser for live updates
- When an AST update is received, re-transpiles the React components and pushes the update to connected browsers
- Provides faster iteration than full backend recompilation for frontend-only changes

### When to Use Each Port

| Port | Use Case |
|------|----------|
| Port A | Testing full application flow: form submissions, capture, navigation, backend logic |
| Port B | Rapid visual iteration: seeing layout/style changes instantly without backend recompile |

The operator typically has the Port B URL open in a browser tab during active building. When they want to test form submissions and navigation, they switch to Port A.

## 9.4 Development Server Lifecycle

### Starting the Dev Server

1. Operator clicks "Start Dev Server" in the builder toolbar
2. Builder frontend POSTs to Tackle: `POST /api/v1/landing-pages/{id}/dev-server/start`
3. Tackle triggers a development build via the `servergen` pipeline
4. `servergen` compiles the binary with development mode enabled
5. Tackle launches the binary as a child process
6. Binary requests two ports from the OS (Port A and Port B)
7. Binary starts both servers (Go backend + React dev server)
8. Binary registers with Tackle via webhook: `{ port_a: N, port_b: M, build_id: "..." }`
9. Tackle records the registration and updates the dev server status
10. Builder UI updates to show "Online" with links to both ports

### Stopping the Dev Server

1. Operator clicks "Stop Dev Server" in the builder toolbar
2. Builder frontend POSTs to Tackle: `POST /api/v1/landing-pages/{id}/dev-server/stop`
3. Tackle sends a shutdown signal to the binary process
4. Binary performs graceful shutdown (closes connections, stops servers)
5. Tackle updates the dev server status to "Offline"

### Automatic Shutdown

The dev server binary self-terminates if:
- Tackle becomes unreachable (heartbeat failure for 30 seconds)
- The operator closes the builder (Tackle detects session end and sends shutdown)
- The builder switches to a different project

## 9.5 Hot Reload Mechanism

### Full Hot-Reload Flow

When the operator makes a change in the builder:

```
1. Operator adds/modifies/removes a component
        │
        ▼
2. Builder debounces changes (600ms)
        │
        ▼
3. Builder serializes the full definition_json
        │
        ▼
4. Builder POSTs to Tackle API:
   POST /api/v1/landing-pages/{id}/dev-server/push-ast
   Body: { definition_json: {...} }
        │
        ▼
5. Tackle forwards the AST to the dev binary:
   POST http://localhost:{Port A}/__hmr/update
   Body: { definition_json: {...} }
        │
        ▼
6. Dev binary processes the update:
   a. Frontend changes → Re-transpile React components via esbuild
                        → Push update to React Dev Server (Port B)
                        → Browser receives WebSocket update
                        → React components hot-reload without page refresh
   b. Backend changes  → Re-generate Go handlers
                        → Restart Go backend server on same Port A
                        → ~1-2 second interruption
```

### What Constitutes a "Frontend Change"

Changes that only affect the React frontend and can be hot-reloaded without backend restart:

- Adding/removing/moving components in the tree
- Changing component properties (text, style, src, etc.)
- Modifying page styles or global styles
- Adding/removing pages (route registration happens on both frontend and backend)

### What Constitutes a "Backend Change"

Changes that affect the Go backend and require a backend restart:

- Changing a form's action path (POST endpoint changes)
- Modifying navigation flow rules (condition evaluation logic changes)
- Changing post-capture actions
- Adding/removing forms (new handlers needed)
- Changing behavioral capability configuration (new JS generation + new metrics endpoint logic)
- Modifying page routes (affects Go route registration)

### Smart Rebuild Detection

The dev binary compares the incoming definition with the current running definition to determine the minimal rebuild scope:

1. **Diff the definitions**: Compare the new definition against the currently running one
2. **Classify changes**: Determine if changes are frontend-only, backend-only, or both
3. **Minimal rebuild**:
   - Frontend-only → Hot-reload React via WebSocket (sub-second)
   - Backend changes → Recompile and restart Go backend (~1-2 seconds)
   - Both → Restart backend, then push frontend update

## 9.6 Dev Server Registration (Bottom-Up)

### Registration Webhook

When the dev binary starts, it POSTs a registration payload to Tackle:

```
POST http://{tackle_host}/api/v1/internal/dev-server/register

{
    "project_id": "uuid",
    "build_id": "uuid",
    "port_a": 38291,        // Go backend port
    "port_b": 41055,        // React dev server port
    "mode": "development",
    "pid": 12345,           // Process ID
    "started_at": "2026-04-14T..."
}
```

### Heartbeat

The dev binary sends periodic heartbeat POSTs to Tackle:

```
POST http://{tackle_host}/api/v1/internal/dev-server/heartbeat

{
    "project_id": "uuid",
    "build_id": "uuid",
    "port_a": 38291,
    "port_b": 41055,
    "uptime_seconds": 120,
    "requests_served": 45
}
```

**Interval**: Every 10 seconds
**Failure threshold**: If 3 consecutive heartbeats fail (30 seconds), the binary self-terminates

### Deregistration

On graceful shutdown, the binary POSTs a deregistration:

```
POST http://{tackle_host}/api/v1/internal/dev-server/deregister

{
    "project_id": "uuid",
    "build_id": "uuid",
    "reason": "operator_stop"  // or "heartbeat_failure", "signal"
}
```

## 9.7 Multiple Simultaneous Dev Servers

Tackle supports multiple landing applications running dev servers simultaneously:

- Each dev binary allocates its own ports from the OS (no collision)
- Each binary self-registers with Tackle independently
- Tackle tracks all active dev instances in memory
- The builder UI shows the status of the current project's dev server only
- AST updates are routed to the correct dev instance based on project ID

### Resource Limits

| Limit | Value |
|-------|-------|
| Maximum simultaneous dev servers | Configurable (default: 5) |
| Memory per dev instance | Monitored, no hard limit |
| CPU per dev instance | Monitored, no hard limit |

If the operator tries to start a dev server when the limit is reached, the builder shows an error indicating which projects have running dev servers and offers to stop one.

## 9.8 Dev Server Status API

### Get Status

```
GET /api/v1/landing-pages/{id}/dev-server/status

Response:
{
    "status": "online",        // online, offline, building, restarting
    "port_a": 38291,
    "port_b": 41055,
    "url_a": "http://localhost:38291",
    "url_b": "http://localhost:41055",
    "build_id": "uuid",
    "uptime_seconds": 120,
    "last_update": "2026-04-14T...",
    "last_rebuild_reason": "frontend_change"
}
```

### Builder UI Status Indicator

The toolbar shows:

| Status | Display |
|--------|---------|
| Offline | Gray dot, "Start Dev Server" button |
| Building | Yellow dot, "Building..." (disabled) |
| Online | Green dot, "Open App" link + "Stop" button |
| Restarting | Orange dot, "Restarting..." (disabled) |

## 9.9 Development Mode Telemetry

When the dev server captures form data or behavioral events, it reports them to Tackle with a `mode: "development"` flag. This allows:

- Tackle to distinguish dev traffic from production campaign traffic
- The operator to verify captures are working correctly (visible in a dev capture log)
- Dev captures to be excluded from campaign reports and dashboards

## 9.10 Zombie Process Cleanup

Tackle runs a background goroutine that periodically checks for orphaned dev server processes:

1. **Heartbeat timeout**: If a registered dev server misses 3 heartbeats, Tackle marks it as dead and attempts to kill the process (by PID)
2. **Startup cleanup**: On Tackle server start, any previously registered dev servers are assumed dead. Tackle attempts to kill any lingering processes by PID and clears the registration table.
3. **Project deletion**: If a landing page project is deleted while its dev server is running, Tackle stops the dev server first.

## 9.11 Dev Server WebSocket Protocol

### Tackle → Dev Binary (AST Push)

```
POST http://localhost:{Port A}/__hmr/update

{
    "type": "ast_update",
    "definition_json": { ... },
    "changed_pages": ["page-abc123"],      // Which pages changed
    "change_type": "frontend"              // frontend, backend, or full
}
```

### Dev Binary → Browser (HMR Push via Port B)

WebSocket message format:

```json
{
    "type": "hmr_update",
    "modules": ["Page_signin", "Page_mfa"],  // Which React modules updated
    "css": "...",                              // Updated CSS bundle (if changed)
    "timestamp": 1713100000
}
```

### Browser → Dev Binary (HMR Client)

The React dev server injects a small HMR client script that:
1. Maintains a WebSocket connection to Port B
2. Listens for `hmr_update` messages
3. Replaces the affected React module code in-memory
4. Triggers a React re-render without losing component state (when possible)
5. Falls back to a full page reload if hot-replacement fails
