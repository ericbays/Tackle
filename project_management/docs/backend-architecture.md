# Tackle Backend Architecture

This document describes the backend architecture, patterns, and conventions for the Tackle platform. Use this as a reference when building the new frontend.

---

## Technology Stack

| Component | Technology |
|-----------|-----------|
| Language | Go 1.25 |
| HTTP Router | Chi v5 (`github.com/go-chi/chi/v5`) |
| Database | PostgreSQL 16 |
| DB Driver | `database/sql` + `lib/pq` (raw SQL, no ORM) |
| Migrations | `golang-migrate/v4` |
| Auth | JWT (HS256 default, RS256 optional) |
| Encryption | AES-256-GCM with HKDF key derivation |
| WebSocket | `gorilla/websocket` |
| Frontend Dev Server | Vite (port 5173) |

## Server Configuration

- **Default listen address**: `:8080`
- **CORS origins**: `http://localhost:5173` (configurable via `TACKLE_CORS_ORIGINS`)
- **Request body limits**: 1 MB standard, 5 MB for batch/import endpoints, 12 MB for email attachments
- **Timeouts**: Read/Write 15s, Idle 60s
- **Graceful shutdown**: 30 seconds

---

## Architecture Layers

```
HTTP Request
  -> Middleware (auth, RBAC, rate limiting, CSRF, correlation ID)
    -> Handler (request parsing, validation, response formatting)
      -> Service (business logic, orchestration)
        -> Repository (raw SQL queries against PostgreSQL)
          -> Database
```

### Handler Pattern

Handlers are methods on a `Deps` struct that holds injected dependencies:

```go
type Deps struct {
    DB       *sql.DB
    Svc      *campaignsvc.Service
    AuditSvc *auditsvc.AuditService
}

func (d *Deps) ListCampaigns(w http.ResponseWriter, r *http.Request) {
    // 1. Parse query params / decode JSON body
    // 2. Validate input
    // 3. Call service layer
    // 4. Return response via response helpers
}
```

### Response Format

All API responses follow a consistent structure:

**Success (single resource)**:
```json
{
  "data": { ... }
}
```

**Success (list with pagination)**:
```json
{
  "data": [ ... ],
  "pagination": {
    "page": 1,
    "per_page": 25,
    "total": 100,
    "total_pages": 4
  }
}
```

**Error**:
```json
{
  "error": {
    "code": "FORBIDDEN",
    "message": "insufficient permissions",
    "correlation_id": "uuid"
  }
}
```

**Validation Error**:
```json
{
  "errors": [
    { "field": "email", "message": "invalid email format" }
  ],
  "correlation_id": "uuid"
}
```

Response helpers are in `pkg/response/`:
- `response.JSON(w, status, data)` — single resource
- `response.List(w, data, pagination)` — paginated list
- `response.Created(w, data)` — 201 Created
- `response.Error(w, status, code, message, correlationID)` — error
- `response.ValidationFailed(w, errors, correlationID)` — validation errors

---

## Middleware Stack

### Global Middleware (applied to all routes, in order)

1. **Recovery** — panic recovery with audit logging
2. **CorrelationID** — injects `X-Correlation-ID` header for request tracing
3. **SecurityHeaders** — HSTS, CSP, X-Frame-Options, X-Content-Type-Options
4. **RequestLogger** — structured request/response logging via `slog`
5. **CORS** — cross-origin handling (configurable allowed origins)
6. **CSRF** — token validation on state-changing requests (POST/PUT/DELETE)

### Route-level Middleware

| Middleware | Purpose | Applied To |
|-----------|---------|-----------|
| `RequireAuth` | JWT validation from `Authorization: Bearer <token>` | All authenticated routes |
| `RequirePermission(perm)` | RBAC permission check | Specific routes |
| `APIKey` | API key auth from `X-API-Key` header | API key routes |
| `RequireBuildToken` | Internal endpoint communication auth | Internal API routes |
| `EndpointAuth` | Endpoint-specific auth | Endpoint data routes |
| `RateLimit` | Per-IP or per-user rate limiting | See below |

### Rate Limits

| Traffic Class | Limit | Scope |
|--------------|-------|-------|
| Auth endpoints | 10 req/min | Per IP |
| Read operations | 120 req/min | Per user |
| Write operations | 60 req/min | Per user |

---

## Authentication System

### Login Flow
1. `POST /api/v1/auth/login` with `{ username, password }`
2. Server validates credentials (local bcrypt or LDAP)
3. Returns JWT access token (15 min TTL) + refresh token
4. Frontend stores tokens and sends `Authorization: Bearer <token>` on all requests

### Token Refresh
1. `POST /api/v1/auth/refresh` with refresh token
2. Server validates and rotates refresh token
3. Returns new access + refresh tokens

### JWT Claims Structure
```json
{
  "sub": "user-uuid",
  "username": "admin",
  "email": "admin@example.com",
  "role": "admin",
  "permissions": ["users:read", "campaigns:create", ...],
  "jti": "unique-token-id",
  "exp": 1234567890,
  "iat": 1234567890
}
```

### External Auth Providers
- **LDAP** — configured via admin UI, supports group-to-role mapping
- **OIDC** — OpenID Connect (FusionAuth, generic providers)
- Account linking allows users to connect external identities

### Password Policy
- Minimum 8 characters, 1 uppercase, 1 digit
- Breached password checking
- Password history enforcement (no reuse)
- Force change on first login

### Account Security
- Per-IP and per-account login rate limiting
- Account lockout after failed attempts
- Token blacklist for logout/revocation
- 30-second user status cache (detects locked/inactive accounts)

---

## RBAC (Role-Based Access Control)

### Permission Format
Permissions follow the pattern `resource:action`:
- `users:read`, `users:create`, `users:update`, `users:delete`
- `campaigns:read`, `campaigns:create`, `campaigns:approve`
- `credentials:reveal`
- `audit:read`
- etc.

### Built-in Roles
Roles are seeded via migration 000002. The `admin` role bypasses all permission checks.

### How It Works
1. JWT contains full `permissions` array
2. `RequirePermission("campaigns:create")` middleware checks claims
3. Admin role short-circuits to allow
4. Otherwise checks if permission exists in claims array

---

## WebSocket Notifications

### Connection Flow
1. Client connects: `GET /api/v1/ws` (upgrades to WebSocket)
2. Client sends auth: `{"type":"auth","token":"<JWT>"}`
3. Server responds: `{"type":"auth_ok"}`
4. Server pushes notification events to connected clients

### 5-second auth timeout — connection closed if no auth message received.

### REST Notification Endpoints
- `GET /api/v1/notifications` — list notifications
- `PUT /api/v1/notifications/{id}/read` — mark read
- `POST /api/v1/notifications/read-all` — mark all read
- `GET /api/v1/notifications/unread-count` — unread count

---

## Background Workers

- **NotificationCleanupWorker** — periodically cleans expired notifications
- Workers start after migrations complete, before server begins accepting requests
- Context-based lifecycle (shut down with server)

---

## File Structure Reference

```
cmd/tackle/main.go              — Entry point, CLI flags, startup
internal/
  config/config.go              — Environment config loader
  database/db.go                — PostgreSQL connection pool
  migrations/runner.go          — Migration runner
  server/server.go              — HTTP server, router setup
  middleware/                   — All middleware (auth, RBAC, CORS, etc.)
  handlers/                     — HTTP handlers (24 subdirectories)
  services/                     — Business logic (30+ service packages)
  repositories/                 — Raw SQL data access (22+ files)
  models/                       — Shared domain models
  crypto/                       — Encryption, key derivation, rotation
  endpoint/                     — Infrastructure provisioning
  compiler/                     — Landing page compilation
  tracking/                     — Tracking token service
  providers/                    — External provider integrations
  workers/                      — Background workers
pkg/
  response/                     — HTTP response helpers
migrations/                     — SQL migration files (60 files)
scripts/
  seed.sql                      — Development seed data
```
