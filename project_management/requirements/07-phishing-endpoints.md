# 07 — Phishing Endpoints

## 1. Overview

Phishing endpoints are the target-facing infrastructure components of the Tackle platform. Each endpoint is a cloud VM (AWS EC2, Azure VM, or Proxmox VM) provisioned and managed by the framework server. The endpoint runs a lightweight Go binary that serves two roles:

1. **Transparent reverse proxy** — Accepts all HTTPS traffic on port 443 and forwards it to the framework server, where the campaign's landing page application is hosted on an arbitrary port. The proxy is completely transparent to the target's browser.
2. **SMTP relay** — Sends phishing emails on behalf of the framework through external SMTP servers, ensuring the endpoint's IP address (not the framework's) appears as the sender IP.

In v1, the relationship is strictly one endpoint per campaign. Endpoint sharing across campaigns is a future consideration.

## 2. Architecture

```
┌──────────────────────────────────────────────────────────┐
│              FRAMEWORK SERVER (Private Lab)              │
│                                                          │
│  ┌────────────────┐   ┌────────────────────────────────┐ │
│  │  Go Backend    │   │  Landing Page App              │ │
│  │  (Orchestrator)│   │  (campaign-specific, :PORT)    │ │
│  └───────┬────────┘   └──────────────┬─────────────────┘ │
│          │                           │                   │
│          │  Control plane            │  Data plane       │
│          │  (provisioning,           │  (proxied HTTP    │
│          │   health checks,          │   requests from   │
│          │   commands)               │   endpoint)       │
└──────────┼───────────────────────────┼───────────────────┘
           │                           ▲
           │                           │
           ▼                           │
┌──────────────────────────────────────────────────────────┐
│        PHISHING ENDPOINT (AWS EC2 / Azure VM / Proxmox)  │
│                                                          │
│  ┌────────────────────────────────────────────────────┐  │
│  │              Go Proxy Binary                       │  │
│  │                                                    │  │
│  │  - TLS termination (:443)                          │  │
│  │  - Transparent reverse proxy → framework:PORT      │  │
│  │  - SMTP relay → external SMTP servers              │  │
│  │  - Request logging → framework                     │  │
│  │  - Health heartbeat → framework                    │  │
│  └──────────────────────┬─────────────────────────────┘  │
│                         │                                │
└─────────────────────────┼────────────────────────────────┘
                          │
              ┌───────────┴───────────┐
              ▼                       ▼
┌──────────────────┐    ┌──────────────────────┐
│  TARGETS         │    │  EXTERNAL SMTP       │
│  (Web browsers)  │    │  SERVERS             │
│  HTTPS → :443    │    │  (email delivery)    │
└──────────────────┘    └──────────────────────┘
```

### 2.1 Transparent Proxy — Critical Design Constraint

The proxy **MUST** behave as a transparent reverse proxy (comparable to nginx `proxy_pass`). This means:

- **NO HTTP redirects** — The proxy must never respond with 301, 302, 303, 307, or 308 status codes to route the target to another server.
- **NO `Location` headers** — No response header may reveal the framework server's address.
- **NO evidence of proxying** — The target's browser must see the endpoint as the origin server. Standard proxy headers (`X-Forwarded-For`, `Via`, etc.) must NOT be sent to the target.
- **All request/response data is streamed** — The proxy reads from the target, forwards to the framework landing app, reads the response, and streams it back. The target's TCP connection terminates at the endpoint.
- **Cookie domains, asset paths, and all embedded URLs** in responses from the landing page app must resolve correctly against the endpoint's domain. The landing page app is responsible for generating relative URLs or URLs using the campaign domain (provided at build time).

### 2.2 SMTP Relay

The endpoint accepts email-send commands from the framework server over the authenticated control channel and relays them to configured external SMTP servers. This ensures:

- The sending IP visible to the recipient's mail server is the endpoint's public IP, not the framework's.
- SPF, DKIM, and DMARC alignment is achievable because DNS records for the campaign domain point to the endpoint's IP.
- The framework never directly connects to external SMTP servers for campaign emails.

### 2.3 One Endpoint Per Campaign (v1)

- Each campaign provisions exactly one endpoint.
- The endpoint is tied to the campaign's lifecycle (created during Build, terminated during campaign teardown).
- Future versions may support endpoint pooling or sharing — the data model should not preclude this (endpoint has a foreign key to campaign, but the schema should support nullable or many-to-many in the future).

---

## 3. Endpoint Lifecycle

The endpoint progresses through a defined set of states managed by the framework server.

```
                 ┌──────────────┐
                 │  Requested   │  Campaign enters Build phase
                 └──────┬───────┘
                        │
                        ▼
                 ┌──────────────┐
                 │ Provisioning │  Cloud API called (EC2 RunInstances / Azure VM create)
                 └──────┬───────┘
                        │
                        ▼
                 ┌──────────────┐
                 │ Configuring  │  Binary deployed, TLS cert installed, proxy rules set
                 └──────┬───────┘
                        │
                        ▼
                 ┌──────────────┐
                 │   Active     │  Proxying traffic, relaying SMTP, reporting health
                 └──────┬───────┘
                        │
              ┌─────────┼─────────┐
              ▼                   ▼
       ┌──────────────┐    ┌──────────────┐
       │   Stopped    │    │    Error     │
       │ (restartable)│    │  (requires   │
       └──────┬───────┘    │   attention) │
              │            └──────┬───────┘
              │                   │
              ▼                   ▼
       ┌──────────────────────────────┐
       │          Terminated          │  Instance destroyed, resources released
       └──────────────────────────────┘
```

### 3.1 State Definitions

| State | Description | Cloud Instance State | Transitions To |
|-------|-------------|---------------------|----------------|
| **Requested** | Campaign build initiated; endpoint creation queued | Does not exist yet | Provisioning |
| **Provisioning** | Cloud API called; waiting for instance to reach running state | Pending / Starting | Configuring, Error |
| **Configuring** | Instance running; framework deploying binary, TLS cert, and proxy config | Running | Active, Error |
| **Active** | Fully operational; proxying HTTPS and relaying SMTP; heartbeats healthy | Running | Stopped, Error, Terminated |
| **Stopped** | Instance stopped (not terminated); can be restarted | Stopped | Active (restart), Terminated |
| **Error** | Endpoint in a failed state; requires operator intervention or automatic retry | Varies | Configuring (retry), Terminated |
| **Terminated** | Instance destroyed; all cloud resources released | Terminated / Deleted | (terminal state) |

### 3.2 State Transitions

All state transitions are recorded in the audit log with timestamp, actor (system or user), and reason.

---

## 4. Proxy Binary Specification

The proxy binary is the sole software deployed to the phishing endpoint. It must be minimal, single-purpose, and leave no unnecessary fingerprint.

### 4.1 Core Capabilities

- **TLS termination** on port 443 using Let's Encrypt (ACME) or operator-provided certificates
- **Transparent reverse proxying** of all HTTP methods (GET, POST, PUT, DELETE, PATCH, OPTIONS, HEAD) to the framework landing page application
- **WebSocket proxying** — Upgrade requests must be handled correctly for landing pages that use WebSocket connections
- **Chunked transfer encoding** and streaming response support
- **SMTP relay** — Accepts structured email-send commands from the framework and delivers them to external SMTP servers
- **Request/response logging** — Every proxied request (method, path, headers, timing, response status) is sent back to the framework for storage
- **Heartbeat reporting** — Periodic health status sent to the framework (CPU, memory, disk, active connections, uptime)
- **Graceful shutdown** — On stop/terminate signal, drain active connections before exiting

### 4.2 What the Binary Must NOT Do

- Expose any admin interface, debug endpoint, or management API on any port
- Write logs to the local filesystem (all logs are shipped to the framework)
- Run any services beyond the proxy and SMTP relay
- Include hardcoded framework addresses in the compiled binary (configuration is injected at deployment time)
- Add, modify, or remove headers in a way that reveals the proxy's existence to the target

### 4.3 Binary Compilation

> **Cline Delegation Candidate** — The proxy binary implementation is an offensive security technique and is flagged for potential delegation to Cline for implementation.

- The binary must be compiled fresh for each deployment to avoid binary fingerprinting by defenders.
- Each compilation should produce a unique binary hash (achieved via build-time variable injection: campaign ID, timestamp, nonce).
- The binary must be statically compiled with no external runtime dependencies.
- Cross-compilation target: Linux amd64 (primary), Linux arm64 (secondary).
- Build output is uploaded to the endpoint during the Configuring phase.

---

## 5. Communication Between Framework and Endpoint

### 5.1 Control Channel (Framework → Endpoint)

- Persistent, authenticated connection from the framework to the endpoint
- Used for: configuration updates, SMTP send commands, stop/restart signals, status queries
- Authentication: mutual TLS (mTLS) or pre-shared token over TLS
- The endpoint never initiates outbound connections to the framework on the control channel — the framework connects to a control port on the endpoint (not port 443)

### 5.2 Data Channel (Endpoint → Framework)

- Request logs and heartbeats are pushed from the endpoint to the framework
- Uses HTTPS POST to a dedicated framework ingestion endpoint
- Authentication: bearer token unique to the endpoint, issued during Configuring phase
- Batched delivery with configurable flush interval (default: 5 seconds or 100 records, whichever comes first)

### 5.3 Proxy Data Plane (Target → Endpoint → Framework)

- Target's HTTPS request arrives at endpoint port 443
- Endpoint terminates TLS, proxies the raw HTTP request to `framework_host:landing_page_port`
- Framework landing page app processes the request and returns a response
- Endpoint streams the response back to the target over the original TLS connection
- Connection between endpoint and framework uses TLS (separate from the target-facing TLS)

---

## 6. Requirements

### 6.1 Endpoint Provisioning

**REQ-PHEP-001: Cloud Instance Provisioning**
The framework MUST provision phishing endpoint instances via AWS EC2, Azure VM, and Proxmox VM APIs during the campaign Build phase.

*Acceptance Criteria:*
- [ ] Framework can create an EC2 instance in a specified region and VPC using configured AWS credentials
- [ ] Framework can create an Azure VM in a specified resource group and region using configured Azure credentials
- [ ] Framework can clone a Proxmox VM from a cloud-init enabled template on a specified node using configured Proxmox API token credentials
- [ ] The cloud provider and region are configurable per campaign
- [ ] Instance type/size is configurable with sensible defaults (e.g., `t3.micro` for AWS, `Standard_B1s` for Azure, `2c-4g` for Proxmox)
- [ ] Provisioning is performed asynchronously via background worker; the UI is not blocked
- [ ] Provisioning failure is surfaced to the operator with the cloud provider error message

*Security Considerations:*
- Cloud API credentials are read from the encrypted configuration store, never hardcoded or logged
- Provisioned instances must be tagged with campaign ID and framework instance ID for audit traceability
- Proxmox VMs are tagged in the VM description/notes and PVE tags with `managed-by: tackle`, campaign ID, and endpoint ID

---

**REQ-PHEP-002: Endpoint State Machine**
The framework MUST track each endpoint through the defined lifecycle states (Requested → Provisioning → Configuring → Active → Stopped → Terminated, with Error as a possible state from Provisioning, Configuring, or Active).

*Acceptance Criteria:*
- [ ] Endpoint state is persisted in the database and updated atomically on each transition
- [ ] Every state transition is recorded in the audit log with timestamp, previous state, new state, actor, and reason
- [ ] The admin UI displays the current endpoint state and full state history
- [ ] Invalid state transitions are rejected by the backend (e.g., cannot go from Terminated to Active)
- [ ] The Error state includes a diagnostic message and supports manual or automatic retry to the appropriate prior state

---

**REQ-PHEP-003: Endpoint Configuration Deployment**
Upon successful instance provisioning, the framework MUST deploy the proxy binary and all required configuration to the endpoint.

*Acceptance Criteria:*
- [ ] The proxy binary is compiled fresh for the specific campaign deployment (unique hash per build)
- [ ] The binary, TLS certificate/key, and proxy configuration file are transferred to the endpoint securely (SCP/SFTP over SSH or equivalent)
- [ ] The proxy configuration includes: framework host, landing page port, control channel credentials, data channel endpoint URL, campaign ID
- [ ] The binary is started as a systemd service (or equivalent) with automatic restart on crash
- [ ] The framework waits for the first successful heartbeat from the endpoint before transitioning to Active state
- [ ] Configuration deployment timeout is configurable (default: 5 minutes); timeout triggers Error state

*Security Considerations:*
- SSH keys for endpoint access are generated per-campaign and stored encrypted in the database
- SSH keys are destroyed when the endpoint is terminated

---

**REQ-PHEP-004: Static IP and DNS Association**
The framework MUST associate a static/elastic IP address with each endpoint and configure DNS records for the campaign domain to point to it.

*Acceptance Criteria:*
- [ ] An Elastic IP (AWS), static public IP (Azure), or pool-allocated static IP (Proxmox) is allocated and associated with the endpoint instance
- [ ] For Proxmox, static IPs are allocated from a per-credential IP pool and configured on the VM via cloud-init
- [ ] The campaign domain's A record is updated to point to the endpoint's static IP via the configured DNS provider (Route 53, Namecheap, GoDaddy, Azure DNS)
- [ ] DNS propagation is verified before the endpoint transitions to Active state (or a warning is surfaced)
- [ ] The static IP is released when the endpoint is terminated (for Proxmox, returned to the IP pool)
- [ ] IP allocation and DNS changes are logged in the audit trail

---

**REQ-PHEP-025: Proxmox VM Provisioning**
The framework MUST support provisioning phishing endpoints on Proxmox VE clusters via the Proxmox REST API. Proxmox is intended for development and testing environments; it is treated identically to AWS/Azure in the codebase and UI.

*Acceptance Criteria:*
- [ ] Proxmox credentials store: API token (token_id + token_secret), host, port, target node name, template VMID, bridge interface, and an IP pool range (start IP, end IP, gateway, subnet mask)
- [ ] VM provisioning clones from a cloud-init enabled template VMID on the specified node
- [ ] Cloud-init configuration injects: static IP (from pool), gateway, DNS, SSH public key, and hostname
- [ ] Static IPs are allocated from the per-credential IP pool; the framework tracks which IPs are in use and releases them on endpoint termination
- [ ] Proxmox VMs are tagged with `managed-by: tackle`, campaign ID, and endpoint ID in the VM description/notes and PVE tags
- [ ] Tackle only manages VMs it has created; it does not list, modify, or interact with other VMs on the Proxmox node
- [ ] Authentication uses PVE API tokens (user@realm!tokenid = secret); no session ticket management
- [ ] Node name is stored in the credential configuration; the "region" concept maps to the Proxmox node name
- [ ] Instance size maps to a CPU/memory profile (e.g., `2c-4g` = 2 cores, 4GB RAM) configurable in the instance template
- [ ] VM stop, start, and terminate operations use the Proxmox API (qemu stop/start/destroy)
- [ ] TestConnection validates API token by querying the Proxmox cluster status endpoint

*Cloud-Init Template VM Requirements:*
- Base OS: Ubuntu Server 22.04+ minimal or Debian 12+ minimal
- Packages: `cloud-init`, `qemu-guest-agent`
- cloud-init configured with `NoCloud` or `configdrive2` datasource
- VM converted to template after setup (right-click → Convert to Template in PVE UI)
- Template VMID recorded in the Proxmox credential configuration

---

**REQ-PHEP-026: Proxmox IP Pool Management**
The framework MUST manage a pool of static IP addresses for Proxmox endpoint provisioning, defined per Proxmox credential.

*Acceptance Criteria:*
- [ ] Each Proxmox credential defines an IP range (start IP, end IP), gateway, and subnet mask
- [ ] When provisioning a Proxmox endpoint, the framework allocates the next available IP from the pool
- [ ] Allocated IPs are tracked in the database with a reference to the endpoint that holds them
- [ ] When an endpoint is terminated, its IP is released back to the pool
- [ ] The framework prevents double-allocation of IPs (atomic allocation with DB-level uniqueness)
- [ ] The API exposes IP pool utilization (allocated/total) for each Proxmox credential
- [ ] If the pool is exhausted, provisioning fails with a clear error message

---

### 6.2 Transparent Reverse Proxy

**REQ-PHEP-005: Transparent HTTPS Reverse Proxy**
The proxy binary MUST transparently forward all HTTPS traffic received on port 443 to the framework server's landing page application without any indication to the target that proxying is occurring.

*Acceptance Criteria:*
- [ ] All HTTP methods (GET, POST, PUT, DELETE, PATCH, OPTIONS, HEAD) are proxied correctly
- [ ] Request headers from the target are forwarded to the framework landing app without modification (except adding internal-only headers for framework use, such as the target's real IP)
- [ ] Response headers from the framework landing app are forwarded to the target without adding proxy-revealing headers (`Via`, `X-Forwarded-For`, `X-Proxy-*`, etc.)
- [ ] Response bodies are streamed without modification
- [ ] The proxy NEVER returns 301, 302, 303, 307, or 308 status codes as part of the proxying mechanism itself
- [ ] The proxy NEVER sends a `Location` header as part of the proxying mechanism — only if the upstream landing app includes one in its response
- [ ] HTTP request/response bodies of any size are supported (streaming, no buffering entire bodies in memory)
- [ ] The target browser's developer tools show the endpoint domain as the origin for all requests

> **Cline Delegation Candidate** — Proxy implementation is an offensive security technique.

---

**REQ-PHEP-006: TLS Termination**
The proxy binary MUST terminate TLS on port 443 using valid certificates for the campaign domain.

*Acceptance Criteria:*
- [ ] Supports Let's Encrypt certificates obtained via ACME (HTTP-01 or DNS-01 challenge)
- [ ] Supports operator-provided certificates (PEM-encoded certificate and key files)
- [ ] TLS 1.2 and TLS 1.3 are supported; TLS 1.0 and 1.1 are disabled
- [ ] Strong cipher suites only (no RC4, no DES, no export ciphers)
- [ ] OCSP stapling is enabled when available
- [ ] Certificate renewal is handled automatically for Let's Encrypt certificates (at least 30 days before expiry)
- [ ] TLS configuration does not reveal the proxy software identity (no identifying `Server` header)

*Security Considerations:*
- Private keys must never leave the endpoint VM
- Let's Encrypt account keys are generated per-endpoint and not reused

---

**REQ-PHEP-007: WebSocket Proxying**
The proxy binary MUST support transparent proxying of WebSocket connections for landing pages that require real-time communication.

*Acceptance Criteria:*
- [ ] HTTP Upgrade requests with `Connection: Upgrade` and `Upgrade: websocket` headers are correctly forwarded
- [ ] The WebSocket connection is established between the target and the framework through the proxy
- [ ] Bidirectional message framing is preserved (text and binary frames)
- [ ] WebSocket ping/pong frames are proxied correctly to maintain connection liveness
- [ ] Connection close frames are propagated in both directions
- [ ] The proxy does not impose a timeout on idle WebSocket connections shorter than the framework's own timeout

> **Cline Delegation Candidate** — WebSocket proxy implementation is part of the offensive proxy capability.

---

**REQ-PHEP-008: Request Logging**
The proxy binary MUST log every proxied HTTP request and send the logs to the framework server for storage.

*Acceptance Criteria:*
- [ ] Each log entry includes: timestamp, source IP, HTTP method, request path, query string, request headers, response status code, response size (bytes), response time (milliseconds), TLS protocol version, and campaign ID
- [ ] Request and response bodies are NOT logged by default (configurable for debugging with size limits)
- [ ] Logs are batched and sent to the framework data channel endpoint at the configured flush interval
- [ ] If the framework is unreachable, logs are buffered in memory (up to a configurable limit, default 10,000 entries) and retried with exponential backoff
- [ ] Log delivery failures are reported in the heartbeat status
- [ ] No logs are written to the endpoint's local filesystem

---

### 6.3 SMTP Relay

**REQ-PHEP-009: SMTP Relay**
The proxy binary MUST relay phishing emails from the framework to external SMTP servers, ensuring the endpoint's IP address is the originating sender IP.

*Acceptance Criteria:*
- [ ] The framework sends structured email-send commands to the endpoint over the control channel (recipient, subject, body, SMTP server address, SMTP port, SMTP credentials, envelope sender)
- [ ] The endpoint connects to the specified external SMTP server and delivers the email using the provided credentials
- [ ] The SMTP transaction originates from the endpoint's public IP address
- [ ] STARTTLS is used when supported by the target SMTP server
- [ ] The endpoint reports delivery success or failure (including SMTP error codes and messages) back to the framework for each email
- [ ] SMTP delivery is rate-limited per the campaign configuration to avoid triggering mail server throttling
- [ ] Multiple emails can be queued and delivered concurrently (configurable concurrency limit, default: 5 simultaneous connections)

*Security Considerations:*
- SMTP credentials are provided per email-send command and are never persisted on the endpoint
- SMTP transaction logs (sender, recipient, status, SMTP server) are sent to the framework for storage

> **Cline Delegation Candidate** — SMTP relay implementation for phishing email delivery is an offensive security technique.

---

### 6.4 Health Monitoring

**REQ-PHEP-010: Endpoint Health Heartbeat**
The proxy binary MUST send periodic health heartbeats to the framework server.

*Acceptance Criteria:*
- [ ] Heartbeats are sent at a configurable interval (default: 30 seconds)
- [ ] Each heartbeat includes: endpoint ID, campaign ID, uptime, CPU usage percentage, memory usage (used/total), disk usage (used/total), active proxy connections count, total requests proxied since start, total emails relayed since start, log buffer depth, current timestamp
- [ ] Heartbeats are sent over the authenticated data channel
- [ ] If a heartbeat cannot be delivered, the binary retries with exponential backoff and continues operating normally

---

**REQ-PHEP-011: Framework-Side Health Monitoring**
The framework MUST continuously monitor endpoint health and alert operators to issues.

*Acceptance Criteria:*
- [ ] The framework tracks the last heartbeat timestamp for each active endpoint
- [ ] If no heartbeat is received within a configurable threshold (default: 3 missed intervals = 90 seconds), the endpoint state transitions to Error with reason "heartbeat timeout"
- [ ] The framework performs periodic HTTP health checks against the endpoint's port 443 (using a predefined health check path that the proxy recognizes and responds to without forwarding to the landing app)
- [ ] Health check failures are logged and contribute to the error detection logic
- [ ] The admin UI displays a real-time health dashboard for all active endpoints with color-coded status indicators
- [ ] Notifications (in-app and optionally via configured channels) are sent when an endpoint enters Error state
- [ ] Resource usage trends (CPU, memory) from heartbeat data are stored and viewable in the UI

---

### 6.5 Endpoint Lifecycle Management

**REQ-PHEP-012: Endpoint Stop and Restart**
The framework MUST support stopping and restarting a phishing endpoint without terminating the cloud instance.

*Acceptance Criteria:*
- [ ] Stop command halts the proxy binary gracefully (drains active connections within a timeout, default 30 seconds)
- [ ] The cloud instance is stopped (AWS StopInstances / Azure VM deallocate) to avoid ongoing compute charges
- [ ] Restart command starts the cloud instance and restarts the proxy binary
- [ ] The endpoint retains its static IP across stop/restart cycles
- [ ] State transitions (Active → Stopped, Stopped → Active) are recorded in the audit log
- [ ] The UI provides stop and restart controls with confirmation dialogs

---

**REQ-PHEP-013: Endpoint Termination**
The framework MUST support full termination of a phishing endpoint, destroying the cloud instance and releasing all associated resources.

*Acceptance Criteria:*
- [ ] The proxy binary is shut down gracefully before instance termination
- [ ] The cloud instance is terminated (AWS TerminateInstances / Azure VM delete)
- [ ] The static/elastic IP address is released
- [ ] DNS records pointing to the endpoint IP are removed or updated
- [ ] SSH keys associated with the endpoint are deleted from the database
- [ ] All endpoint metadata and logs remain in the database for historical reference (only the cloud resources are destroyed)
- [ ] Termination can be triggered manually from the UI or automatically as part of campaign teardown
- [ ] A confirmation dialog with a summary of resources to be destroyed is presented before manual termination

---

**REQ-PHEP-014: Automatic Cleanup on Campaign Completion**
When a campaign is completed or cancelled, the framework MUST automatically terminate the associated endpoint unless the operator explicitly opts to retain it.

*Acceptance Criteria:*
- [ ] Campaign completion triggers an automatic endpoint termination workflow
- [ ] The operator is prompted (or a campaign-level setting controls) whether to auto-terminate or retain the endpoint
- [ ] If retained, the endpoint transitions to Stopped (not actively proxying, but instance preserved)
- [ ] Retained endpoints are surfaced in the UI with a reminder to terminate them to avoid ongoing cloud costs
- [ ] An administrative view lists all non-terminated endpoints across all campaigns with their current cloud cost implications

---

### 6.6 Security

**REQ-PHEP-015: Per-Deployment Binary Compilation**
The proxy binary MUST be compiled uniquely for each endpoint deployment to prevent binary fingerprinting by defenders.

*Acceptance Criteria:*
- [ ] Each compilation injects unique build-time variables (campaign ID, deployment timestamp, random nonce) that alter the binary hash
- [ ] The resulting binary has a unique SHA-256 hash for every deployment
- [ ] No two endpoints in the system's history share the same binary hash
- [ ] The compilation is automated as part of the Configuring lifecycle phase
- [ ] Compilation logs (including the resulting hash) are stored in the database
- [ ] The binary is statically compiled for Linux amd64 with no external dependencies

*Security Considerations:*
- The build environment must be clean and reproducible (no leftover artifacts from prior builds)
- The Go source code for the proxy binary must not be present on the endpoint — only the compiled binary

> **Cline Delegation Candidate** — Anti-fingerprinting compilation strategy is an offensive security technique.

---

**REQ-PHEP-016: Authenticated Framework-Endpoint Communication**
All communication between the framework server and the phishing endpoint MUST be authenticated and encrypted.

*Acceptance Criteria:*
- [ ] The control channel uses TLS with mutual authentication (mTLS) or a pre-shared secret over TLS
- [ ] The data channel (logs, heartbeats) uses HTTPS with a bearer token unique to the endpoint
- [ ] Authentication credentials are generated during the Configuring phase and stored encrypted in the database
- [ ] Credentials are invalidated when the endpoint is terminated
- [ ] Unauthenticated requests to the control channel or data channel are rejected and logged
- [ ] The endpoint rejects control commands that do not pass authentication

---

**REQ-PHEP-017: Endpoint Hardening**
The phishing endpoint VM MUST be hardened to minimize attack surface and forensic exposure.

*Acceptance Criteria:*
- [ ] Only port 443 (HTTPS proxy) and the control channel port are open; all other ports are firewalled
- [ ] No SSH access is left open after the Configuring phase completes (SSH is used only during deployment, then the firewall rule is removed)
- [ ] No unnecessary services, packages, or daemons run on the VM
- [ ] The proxy binary runs as a non-root user with minimal filesystem permissions
- [ ] No logs, temporary files, or cached data persist on the endpoint's local filesystem
- [ ] The VM base image is minimal (e.g., Amazon Linux 2 minimal, Ubuntu Server minimal; for Proxmox, the cloud-init template VM)
- [ ] Filesystem is mounted with `noexec` on non-essential partitions where feasible

*Security Considerations:*
- The endpoint should be assumed to be potentially captured and inspected by defenders. It must reveal as little as possible about the framework server, the campaign, or the red team.

---

### 6.7 API Endpoints (Framework Backend)

**REQ-PHEP-018: Endpoint Management API**
The framework backend MUST expose RESTful API endpoints for managing phishing endpoints.

*Acceptance Criteria:*
- [ ] `POST /api/v1/campaigns/{campaign_id}/endpoint` — Provision a new endpoint for the campaign
- [ ] `GET /api/v1/campaigns/{campaign_id}/endpoint` — Get the endpoint's current status, configuration, and health
- [ ] `POST /api/v1/campaigns/{campaign_id}/endpoint/stop` — Stop the endpoint
- [ ] `POST /api/v1/campaigns/{campaign_id}/endpoint/restart` — Restart the endpoint
- [ ] `DELETE /api/v1/campaigns/{campaign_id}/endpoint` — Terminate the endpoint
- [ ] `GET /api/v1/campaigns/{campaign_id}/endpoint/logs` — Retrieve proxied request logs (paginated, filterable)
- [ ] `GET /api/v1/campaigns/{campaign_id}/endpoint/health` — Retrieve health history (heartbeat data, time-series)
- [ ] `GET /api/v1/endpoints` — List all endpoints across campaigns (admin view, filterable by state)
- [ ] All endpoints require authentication and appropriate RBAC permissions
- [ ] All mutating operations are gated by the approval workflow when configured
- [ ] API responses follow the standard Tackle API response envelope

---

### 6.8 Admin UI

**REQ-PHEP-019: Endpoint Status in Campaign View**
The admin UI MUST display the phishing endpoint's status within the campaign detail view.

*Acceptance Criteria:*
- [ ] The campaign detail view includes an endpoint panel showing: current state, cloud provider, region, instance ID, public IP, domain, uptime, resource usage (CPU/memory), request count, email count
- [ ] State is updated in real-time via WebSocket
- [ ] The panel includes action buttons for Stop, Restart, and Terminate with confirmation dialogs
- [ ] State history (all transitions with timestamps) is viewable in an expandable section
- [ ] Error states display the diagnostic message and a retry button

---

**REQ-PHEP-020: Endpoint Request Log Viewer**
The admin UI MUST provide a log viewer for all HTTP requests proxied through the endpoint.

*Acceptance Criteria:*
- [ ] Logs are displayed in a sortable, filterable table with columns: timestamp, source IP, method, path, status code, response size, response time
- [ ] Filters include: time range, HTTP method, status code range, source IP, path pattern (substring or regex)
- [ ] Logs are paginated with configurable page size
- [ ] Clicking a log entry expands to show full request and response headers
- [ ] Logs can be exported as CSV or JSON
- [ ] The viewer updates in near-real-time as new requests are proxied (via WebSocket or polling)

---

**REQ-PHEP-021: Endpoint Health Dashboard**
The admin UI MUST provide a health overview for all active endpoints.

*Acceptance Criteria:*
- [ ] A dedicated page (or section in the infrastructure view) lists all non-terminated endpoints
- [ ] Each endpoint shows: campaign name, domain, state (color-coded), last heartbeat time, CPU usage, memory usage, active connections
- [ ] Endpoints in Error state are highlighted and sorted to the top
- [ ] Clicking an endpoint navigates to the campaign detail view's endpoint panel
- [ ] The dashboard auto-refreshes at a configurable interval (default: 10 seconds)

---

## 7. Data Model

The following fields represent the minimum required schema for the endpoint entity. Full schema is defined in [14-database-schema.md](14-database-schema.md).

| Field | Type | Description |
|-------|------|-------------|
| `id` | UUID | Primary key |
| `campaign_id` | UUID (FK) | Associated campaign |
| `cloud_provider` | ENUM (aws, azure, proxmox) | Cloud provider used |
| `region` | VARCHAR | Cloud region (e.g., `us-east-1`, `eastus`) |
| `instance_id` | VARCHAR | Cloud provider instance identifier |
| `public_ip` | INET | Static/elastic public IP address |
| `domain` | VARCHAR | Campaign domain served by this endpoint |
| `state` | ENUM | Current lifecycle state |
| `binary_hash` | VARCHAR | SHA-256 hash of the deployed binary |
| `control_port` | INTEGER | Port for framework→endpoint control channel |
| `auth_token` | VARCHAR (encrypted) | Bearer token for data channel authentication |
| `ssh_key_id` | UUID (FK) | Reference to encrypted SSH key used during deployment |
| `error_message` | TEXT | Diagnostic message when in Error state |
| `last_heartbeat_at` | TIMESTAMP | Timestamp of last received heartbeat |
| `provisioned_at` | TIMESTAMP | When the cloud instance was created |
| `activated_at` | TIMESTAMP | When the endpoint entered Active state |
| `terminated_at` | TIMESTAMP | When the endpoint was terminated |
| `created_at` | TIMESTAMP | Record creation timestamp |
| `updated_at` | TIMESTAMP | Record last update timestamp |

---

## 8. Error Handling

| Scenario | Behavior |
|----------|----------|
| Cloud API provisioning failure | Transition to Error; log cloud error; surface to operator; allow retry |
| Binary deployment failure (SSH timeout, transfer error) | Transition to Error; log details; allow retry from Configuring state |
| TLS certificate acquisition failure | Transition to Error; log ACME error; allow retry or manual certificate upload |
| Heartbeat timeout | Transition to Error; attempt cloud-level health check; notify operator |
| SMTP relay failure | Report per-email failure to framework; do NOT transition endpoint state (individual email failure is not an endpoint failure) |
| Framework unreachable (from endpoint) | Buffer logs, continue proxying, retry connection with exponential backoff |
| Endpoint binary crash | Systemd restarts the binary; if crash loops (>3 in 5 minutes), framework detects via heartbeat loss |
| DNS propagation failure | Warn operator; do not block Active transition but surface warning prominently |

---

## 9. Non-Functional Requirements

| Concern | Requirement |
|---------|-------------|
| **Latency** | Proxy-added latency must be < 50ms per request (excluding network transit to framework) |
| **Throughput** | Endpoint must handle at least 100 concurrent connections without degradation |
| **Memory** | Proxy binary must use < 128MB RSS under normal load |
| **Startup** | Binary must be ready to proxy within 5 seconds of process start |
| **Reliability** | Binary must run unattended for at least 30 days without memory leaks or performance degradation |
| **Binary size** | Compiled binary should be < 20MB |

---

## 10. Dependencies

| Dependency | Requirement Document |
|------------|---------------------|
| Campaign Management (lifecycle, Build phase) | [06-campaign-management.md](06-campaign-management.md) |
| Domain Management (DNS records) | [03-domain-management.md](03-domain-management.md) |
| SMTP Configuration (external server details) | [04-smtp-configuration.md](04-smtp-configuration.md) |
| Landing Page Apps (proxy target) | [05-landing-page-builder.md](05-landing-page-builder.md) |
| Credential Capture (proxied POST data) | [08-credential-capture.md](08-credential-capture.md) |
| Audit Logging (state transitions, operations) | [11-audit-logging.md](11-audit-logging.md) |
| Database Schema (endpoint table) | [14-database-schema.md](14-database-schema.md) |
| Cloud Provider Configuration | [01-system-overview.md](01-system-overview.md) |

---

## 11. Cline Delegation Summary

The following requirements involve offensive security techniques and are candidates for delegation to Cline for implementation:

| Requirement | Description |
|-------------|-------------|
| REQ-PHEP-005 | Transparent HTTPS reverse proxy implementation |
| REQ-PHEP-007 | WebSocket proxying |
| REQ-PHEP-009 | SMTP relay for phishing email delivery |
| REQ-PHEP-015 | Per-deployment anti-fingerprinting binary compilation |

These items require careful implementation to ensure the endpoint is indistinguishable from a legitimate web server to both human analysts and automated security tools.

---

## 12. Notify-Only Endpoint Recovery

**REQ-PHEP-022: Notify-Only Error Recovery**

When an endpoint enters the Error state, the system SHALL notify the Operator but NOT automatically attempt recovery actions. Recovery actions are manual.

*Acceptance Criteria:*
- [ ] When an endpoint enters Error state, the system sends an immediate notification to the campaign Operator and all Administrators (in-app + email if configured)
- [ ] The notification includes: campaign name, endpoint domain, error type, error details, and a direct link to the endpoint status panel
- [ ] No automatic restart, reprovisioning, or infrastructure changes are attempted without Operator action
- [ ] The Operator can manually trigger recovery actions (restart, reprovision, or terminate) from the endpoint status panel
- [ ] A "Retry" button is available in the UI for the Operator to manually initiate recovery from the Error state

---

**REQ-PHEP-023: Manual TLS Certificate Upload**

In addition to ACME/Let's Encrypt automatic certificate provisioning (REQ-PHEP-006), the system SHALL support manual upload of TLS certificates by the Operator.

*Acceptance Criteria:*
- [ ] The endpoint configuration UI provides a certificate upload form accepting PEM-encoded certificate and private key files
- [ ] The system validates that the uploaded certificate matches the campaign domain
- [ ] The system validates that the certificate chain is complete (includes intermediate certificates)
- [ ] The system validates that the certificate is not expired and has a remaining validity of at least 24 hours
- [ ] Uploaded certificates are stored encrypted at rest in the database
- [ ] The Operator can choose between ACME automatic provisioning and manual upload during endpoint configuration
- [ ] Certificate replacement (uploading a new certificate for an active endpoint) is supported without endpoint restart

---

**REQ-PHEP-024: Phishing Report Webhook Tracking**

The system SHALL support receiving phishing report notifications via webhook from external reporting systems (e.g., email phishing report buttons that forward to a webhook URL).

*Acceptance Criteria:*
- [ ] The framework exposes a webhook endpoint (`POST /api/v1/webhooks/phishing-reports`) that accepts phishing report submissions
- [ ] The webhook payload must include at minimum: the reporter's email address and the original phishing email's Message-ID or subject line
- [ ] The system matches the report to the correct campaign and target based on the Message-ID or email metadata
- [ ] Successfully matched reports update the target's status to include the `reported` flag (REQ-TGT-021)
- [ ] Webhook authentication is configurable: API key header, HMAC signature, or no authentication (for testing)
- [ ] In addition to webhooks, Operators can manually flag a target as "reported" from the campaign target list UI
- [ ] All phishing report events (webhook and manual) are recorded in the audit log

---

## 13. Acceptance Criteria (Document-Level)

- [ ] Endpoints can be provisioned on AWS EC2, Azure VM, and Proxmox VM from the framework UI
- [ ] The proxy binary transparently forwards HTTPS traffic with zero evidence of proxying visible in the target's browser (no redirects, no proxy headers, no `Location` headers)
- [ ] Emails sent through the endpoint's SMTP relay originate from the endpoint's public IP as observed by the recipient's mail server
- [ ] Endpoint lifecycle (provision → configure → active → stop → restart → terminate) is fully managed from the UI
- [ ] Health monitoring detects endpoint failures within 90 seconds and alerts the operator
- [ ] Each deployment produces a unique binary hash
- [ ] All framework-endpoint communication is authenticated and encrypted
- [ ] Request logs from the endpoint are visible in the admin UI within 10 seconds of the request occurring
- [ ] Endpoint termination releases all cloud resources (instance, IP, DNS records)
- [ ] No sensitive information about the framework is recoverable from a captured endpoint VM
- [ ] Endpoint error recovery is notify-only; no automatic recovery without Operator action
- [ ] TLS certificates can be manually uploaded as an alternative to ACME
- [ ] Phishing report webhooks correctly match reports to campaigns and update target status
