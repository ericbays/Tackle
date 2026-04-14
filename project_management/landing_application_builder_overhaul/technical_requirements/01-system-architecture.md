# 01 — Landing Application Builder: System Architecture & Paradigm

## 1.1 Purpose

The Landing Application Builder is a core subsystem of Tackle that enables operators to visually design, configure, and deploy fully functional web applications used during phishing campaigns. The builder is a no-code tool — operators configure all application behavior (page layout, form capture, navigation flows, interaction tracking, file serving) entirely through the builder's visual interface. No code writing is required at any point.

The output of the builder is a **standalone Go binary** that serves a complete web application (React frontend + Go backend) capable of:

- Serving multi-page React applications to targets
- Capturing form submissions and forwarding them to Tackle
- Tracking behavioral interactions (page views, clicks, downloads, keystrokes, etc.)
- Executing operator-defined navigation flows with conditional routing
- Serving files and payloads to targets
- Reporting all events and captured data upstream to Tackle via direct POST

## 1.2 Architectural Boundaries

### What the Landing Application Builder IS

- A visual, drag-and-drop application builder within the Tackle admin UI
- A configuration tool that produces a complete JSON definition of the landing application
- The orchestrator of a compilation pipeline (`servergen`) that transforms JSON definitions into standalone Go binaries
- The manager of a development server lifecycle for real-time testing during the build process

### What the Landing Application Builder is NOT

- It is NOT a code editor — operators never write Go, JavaScript, HTML, or CSS directly
- It is NOT a campaign management tool — campaign attachment, phishing endpoint routing, and deployment lifecycle are handled by the Campaign subsystem
- It is NOT responsible for anti-fingerprinting or evasion — those are production/campaign-level build concerns handled by a separate evasion pipeline during campaign deployment
- It is NOT a static page previewer — the development server runs the actual compiled application with full backend functionality

## 1.3 System Topology

```
┌─────────────────────────────────────────────────────────────────┐
│                        TACKLE SERVER                            │
│                                                                 │
│  ┌──────────────────┐    ┌──────────────────┐                   │
│  │  Admin Frontend   │    │  Tackle Backend   │                  │
│  │  (React SPA)      │◄──►│  (Go API Server)  │                  │
│  │                   │    │                   │                  │
│  │  ┌─────────────┐  │    │  ┌─────────────┐  │                  │
│  │  │ Application  │  │    │  │  servergen   │  │                  │
│  │  │ Builder UI   │──┼────┼─►│  Pipeline    │  │                  │
│  │  └─────────────┘  │    │  └──────┬──────┘  │                  │
│  └──────────────────┘    │         │         │                  │
│                          │         ▼         │                  │
│                          │  ┌─────────────┐  │                  │
│                          │  │ Compiled Go  │  │                  │
│                          │  │ Binary       │  │                  │
│                          │  └──────┬──────┘  │                  │
│                          └────────┼─────────┘                  │
│                                   │                             │
│  ┌────────────────────────────────┼──────────────────────────┐  │
│  │        LANDING APPLICATION RUNTIME ENVIRONMENT             │  │
│  │                                                            │  │
│  │  ┌──────────────┐  ┌──────────────┐  ┌──────────────┐     │  │
│  │  │ Landing App  │  │ Landing App  │  │ Landing App  │     │  │
│  │  │ Instance A   │  │ Instance B   │  │ Instance C   │     │  │
│  │  │ (Port 38291) │  │ (Port 41055) │  │ (Port 29847) │     │  │
│  │  │              │  │              │  │              │     │  │
│  │  │ React ← Go   │  │ React ← Go   │  │ React ← Go   │     │  │
│  │  └──────┬───────┘  └──────┬───────┘  └──────┬───────┘     │  │
│  │         │                 │                 │              │  │
│  │         └────────────┬────┴────────┬────────┘              │  │
│  │                      ▼             ▼                       │  │
│  │              POST /captures   POST /metrics                │  │
│  │                   to Tackle Internal API                    │  │
│  └────────────────────────────────────────────────────────────┘  │
└─────────────────────────────────────────────────────────────────┘
                              │
                    ┌─────────┼─────────┐
                    ▼                   ▼
           ┌──────────────┐    ┌──────────────┐
           │   Phishing    │    │   Phishing    │
           │  Endpoint A   │    │  Endpoint B   │
           │  (Internet)   │    │  (Internet)   │
           │               │    │               │
           │  Proxies to   │    │  Proxies to   │
           │  Landing App  │    │  Landing App  │
           │  Instance A   │    │  Instance B   │
           └──────────────┘    └──────────────┘
                 ▲                    ▲
                 │                    │
            Target HTTP          Target HTTP
             Traffic              Traffic
```

## 1.4 Core Principles

### 1.4.1 Operator-Driven Configuration

Every aspect of the landing application's behavior is configured by the operator through the builder UI:

- **Page structure**: Layout, components, styling — all visual drag-and-drop
- **Form behavior**: Which fields to capture, what endpoint the form POSTs to, what happens after submission
- **Navigation flows**: Conditional and event-driven routing between pages
- **Interaction tracking**: Page-level and component-level behavioral capabilities
- **Asset management**: Upload and embed images, fonts, files, and payloads
- **URL structure**: The operator defines all visible URL paths the target sees

### 1.4.2 Invisible Backend Operations

From the target's perspective (and from any defensive tooling inspecting the browser), the landing application behaves like a normal website:

- Forms POST to normal-looking endpoints defined by the operator (e.g., `/signin`, `/api/v1/auth`)
- Page navigation uses standard browser routing
- Assets load from the same origin
- No visible JavaScript interception or exfiltration

What happens invisibly on the backend:

- The Go server captures all form data and forwards it to Tackle's internal capture endpoint
- Behavioral events (page views, clicks, keystrokes, etc.) are packaged and sent to Tackle's metrics endpoint
- File download events are tracked and reported
- All upstream communication stays within the local network — never exposed to the internet

### 1.4.3 Standalone Binary Architecture

Each landing application compiles into a single Go binary that:

- Embeds all React/HTML/CSS/JS assets via `go:embed`
- Embeds all uploaded assets (images, fonts, files) via `go:embed`
- Contains all route handlers, middleware, and business logic
- Requires no external dependencies at runtime (no database, no file system access, no network dependencies other than Tackle's internal API)
- Self-registers with Tackle on startup (bottom-up registration model)
- Self-terminates via heartbeat if Tackle becomes unreachable

### 1.4.4 Bottom-Up Self-Registration

Landing application binaries do not receive port assignments from Tackle. Instead:

1. The binary starts and requests available ports from the OS
2. The binary binds to the OS-assigned ports
3. The binary fires a registration webhook to Tackle with its assigned ports and metadata
4. Tackle records the registration and begins routing traffic (or configuring phishing endpoints)
5. The binary maintains a heartbeat with Tackle — if Tackle becomes unreachable for a configurable duration, the binary self-terminates

This model supports:
- Multiple simultaneous landing applications on the same Tackle server
- No port collision management needed in Tackle
- Future distributed deployment (binaries could run on separate hosts)

### 1.4.5 Dual-Mode Operation

The builder supports two operational modes for landing applications:

| Aspect | Development Mode | Production Mode |
|--------|-----------------|-----------------|
| **Purpose** | Real-time testing during build | Live campaign deployment |
| **Triggered by** | Operator in the builder UI | Campaign management subsystem |
| **Hot reload** | Full hot-reload (frontend + backend) | No hot reload |
| **Evasion** | None — clean, readable output | Full anti-fingerprinting pipeline |
| **Telemetry** | Reports to Tackle (dev-flagged) | Reports to Tackle (production) |
| **Lifecycle** | Tied to builder session | Tied to campaign lifecycle |

## 1.5 Compilation Pipeline Overview

The `servergen` pipeline transforms an operator's builder configuration into a running application:

```
Builder JSON Definition
        │
        ▼
┌─────────────────┐
│   Validation     │  Validate definition schema, component tree,
│                  │  navigation rules, capture configuration
└────────┬────────┘
         │
         ▼
┌─────────────────┐
│   React/Frontend │  Generate React components from component tree,
│   Generation     │  transpile JSX → JS via esbuild, bundle CSS
└────────┬────────┘
         │
         ▼
┌─────────────────┐
│   Go Backend     │  Generate main.go with route handlers, form
│   Generation     │  capture logic, telemetry reporting, asset
│                  │  serving, navigation routing
└────────┬────────┘
         │
         ▼
┌─────────────────┐
│   Asset          │  Embed React bundles, uploaded assets,
│   Embedding      │  payload files into Go source via go:embed
└────────┬────────┘
         │
         ▼
┌─────────────────┐
│   Compilation    │  go build → standalone binary
│                  │
└────────┬────────┘
         │
         ▼
   Standalone Go Binary
   (ready to run)
```

## 1.6 Security Boundaries

### Network Isolation

- Landing application binaries run on the Tackle server within the private lab network
- Upstream communication (captures, metrics) occurs exclusively over the local network
- Landing applications never initiate outbound connections to the internet
- Only phishing endpoints (separate infrastructure) are internet-facing
- Phishing endpoints transparently proxy HTTP traffic to landing applications

### No Authentication on Internal Channel

- Landing applications communicate with Tackle's internal API without authentication
- This is safe because both processes run on the same host within a firewalled network
- The internal API endpoints are not exposed on any public-facing interface

### Target-Facing Surface

- The only surface visible to targets and defenders is the HTTP response served through the phishing endpoint proxy
- No internal telemetry, capture forwarding, or Tackle metadata is ever included in target-facing responses
- The landing application's Go binary, source code, and internal API are never accessible to targets or defenders
