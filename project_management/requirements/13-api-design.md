# 13 — API Design

## 1. Purpose

This document defines the complete REST API and WebSocket API surface for the Tackle platform. It specifies every endpoint, its HTTP method, URL path, required permissions, request/response contracts, and security controls. The API serves as the contract between the Go backend and the React admin UI, and it is the sole interface through which all operations are performed. No direct database access is permitted from the frontend.

---

## 2. Architectural Context

```
┌──────────────────────────────────────────────────────────────────────┐
│                      REACT ADMIN UI (SPA)                            │
│                                                                      │
│  ┌──────────────┐  ┌──────────────┐  ┌─────────────┐  ┌───────────┐  │
│  │  REST Calls  │  │  WebSocket   │  │  File       │  │  Auth     │  │
│  │  (axios/     │  │  Connections │  │  Uploads    │  │  Tokens   │  │
│  │   fetch)     │  │  (ws://)     │  │  (multipart)│  │  (JWT)    │  │
│  └──────┬───────┘  └──────┬───────┘  └──────┬──────┘  └─────┬─────┘  │
└─────────┼─────────────────┼─────────────────┼────────────────┼───────┘
          │                 │                 │                │
          ▼                 ▼                 ▼                ▼
┌──────────────────────────────────────────────────────────────────────┐
│                      GO BACKEND API SERVER                           │
│                                                                      │
│  ┌────────────────────────────────────────────────────────────────┐  │
│  │                     MIDDLEWARE CHAIN                           │  │
│  │  Rate Limiter → CORS → Auth (JWT) → RBAC → Request Logger      │  |
│  └────────────────────────────────────────────────────────────────┘  │
│                                                                      │
│  ┌──────────────┐  ┌──────────────┐  ┌──────────────────────────┐    │
│  │  REST Router │  │  WebSocket   │  │  Background Workers      │    │
│  │  /api/v1/... │  │  Upgrader    │  │  (email, infra, health)  │    │
│  └──────┬───────┘  └──────┬───────┘  └──────────────────────────┘    │
│         │                 │                                          │
│         ▼                 ▼                                          │
│  ┌──────────────────────────────┐                                    │
│  │        PostgreSQL            │                                    │
│  └──────────────────────────────┘                                    │
└──────────────────────────────────────────────────────────────────────┘
```

### 2.1 Key Design Principles

1. **Versioned routes.** All REST endpoints live under `/api/v1/`. Future breaking changes increment the version.
2. **JSON everywhere.** All request and response bodies use `application/json` unless explicitly noted (file uploads use `multipart/form-data`).
3. **Stateless.** The API server does not maintain session state. Authentication state is carried in JWT tokens.
4. **Consistent patterns.** Every resource follows the same CRUD conventions, error format, pagination scheme, and filtering syntax.
5. **Security by default.** Every endpoint requires authentication and RBAC authorization unless explicitly marked as public.
6. **Correlation IDs.** Every request is assigned a unique correlation ID (returned in the `X-Correlation-ID` response header) that links the request to all log entries, audit records, and downstream operations it triggers.

---

## 3. General API Conventions

### 3.1 Request and Response Format

**REQ-API-001** — All API endpoints SHALL accept and return `application/json` content type unless the endpoint explicitly handles file uploads (`multipart/form-data`) or file downloads (`application/octet-stream`, `text/csv`).

**REQ-API-002** — All API responses SHALL include the following standard headers:

| Header | Description |
|--------|-------------|
| `X-Correlation-ID` | Unique UUID for the request, used in all logs and audit records |
| `X-Request-ID` | Alias for correlation ID (for compatibility with common observability tools) |
| `Content-Type` | `application/json` for all JSON responses |
| `X-RateLimit-Limit` | Maximum requests allowed in the current window |
| `X-RateLimit-Remaining` | Requests remaining in the current window |
| `X-RateLimit-Reset` | Unix timestamp when the rate limit window resets |

**REQ-API-003** — All successful responses SHALL use the appropriate HTTP status code:

| Status Code | Usage |
|-------------|-------|
| `200 OK` | Successful GET, PUT, PATCH, DELETE |
| `201 Created` | Successful POST that creates a resource |
| `202 Accepted` | Request accepted for async processing (e.g., provisioning) |
| `204 No Content` | Successful DELETE with no response body |

**REQ-API-004** — All successful list responses SHALL use a consistent envelope format:

```json
{
  "data": [ ... ],
  "pagination": {
    "cursor": "eyJpZCI6MTAwfQ==",
    "has_more": true,
    "total_count": 1542
  }
}
```

**REQ-API-005** — All successful single-resource responses SHALL use a consistent envelope format:

```json
{
  "data": { ... }
}
```

### 3.2 Error Response Format

**REQ-API-006** — All error responses SHALL use a consistent JSON structure:

```json
{
  "error": {
    "code": "VALIDATION_ERROR",
    "message": "Human-readable description of the error",
    "details": [
      {
        "field": "email",
        "message": "Must be a valid email address",
        "code": "INVALID_FORMAT"
      }
    ],
    "correlation_id": "550e8400-e29b-41d4-a716-446655440000"
  }
}
```

**REQ-API-007** — The API SHALL use the following standard error codes and HTTP status codes:

| HTTP Status | Error Code | Description |
|-------------|-----------|-------------|
| `400` | `VALIDATION_ERROR` | Request body or parameters failed validation |
| `400` | `BAD_REQUEST` | Malformed request syntax |
| `401` | `UNAUTHORIZED` | Missing or invalid authentication token |
| `401` | `TOKEN_EXPIRED` | JWT has expired; client should refresh |
| `403` | `FORBIDDEN` | Authenticated but lacks required permission |
| `404` | `NOT_FOUND` | Resource does not exist or is not accessible |
| `409` | `CONFLICT` | Resource state conflict (e.g., duplicate name, invalid state transition) |
| `413` | `PAYLOAD_TOO_LARGE` | Request body exceeds size limit |
| `422` | `UNPROCESSABLE_ENTITY` | Request is well-formed but semantically invalid |
| `429` | `RATE_LIMITED` | Too many requests; retry after the time indicated in headers |
| `500` | `INTERNAL_ERROR` | Unexpected server error (details logged, not exposed to client) |
| `502` | `UPSTREAM_ERROR` | External service call failed (cloud provider, SMTP, DNS) |
| `503` | `SERVICE_UNAVAILABLE` | Server is temporarily unable to handle the request |

**REQ-API-008** — Internal error details (stack traces, internal paths, SQL errors) SHALL never be exposed in error responses. The `correlation_id` SHALL be the mechanism for operators to look up detailed error information in the logs.

### 3.3 Pagination

**REQ-API-009** — All list endpoints returning potentially large result sets SHALL support cursor-based pagination using the following query parameters:

| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| `cursor` | string | (empty) | Opaque cursor from a previous response; omit for the first page |
| `limit` | integer | 25 | Number of items to return (max 100) |

**REQ-API-010** — The cursor SHALL be an opaque, base64-encoded token that encodes the position in the result set. Clients SHALL NOT parse or construct cursors; they SHALL only pass cursors received from previous responses.

**REQ-API-011** — For endpoints where total count is requested, the client SHALL pass `include_count=true` as a query parameter. The `total_count` field in the pagination response SHALL only be populated when this parameter is present, to avoid expensive count queries by default.

### 3.4 Filtering and Sorting

**REQ-API-012** — List endpoints SHALL support filtering via query parameters using the following conventions:

| Pattern | Example | Description |
|---------|---------|-------------|
| Exact match | `?status=active` | Field equals value |
| Multiple values | `?status=active,paused` | Field equals any of the comma-separated values |
| Date range | `?created_after=2026-01-01&created_before=2026-02-01` | Field within date range |
| Search | `?search=keyword` | Full-text search across relevant fields |
| Nested filter | `?campaign_id=uuid` | Filter by related resource ID |

**REQ-API-013** — List endpoints SHALL support sorting via query parameters:

| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| `sort` | string | `created_at` | Field name to sort by |
| `order` | string | `desc` | Sort direction: `asc` or `desc` |

**REQ-API-014** — The API SHALL return a `400 VALIDATION_ERROR` if a client specifies a sort field or filter field that is not supported by the endpoint.

### 3.5 Rate Limiting

**REQ-API-015** — All API endpoints SHALL be rate-limited. Rate limits SHALL be configurable per endpoint group and per authentication context (user or API key).

**REQ-API-016** — Default rate limits SHALL be:

| Endpoint Group | Rate Limit |
|---------------|------------|
| Authentication (login, refresh) | 10 requests per minute per IP |
| Read operations (GET) | 120 requests per minute per user |
| Write operations (POST, PUT, PATCH, DELETE) | 60 requests per minute per user |
| File uploads | 10 requests per minute per user |
| WebSocket connections | 5 concurrent connections per user |

**REQ-API-017** — When a rate limit is exceeded, the API SHALL return a `429 RATE_LIMITED` response with a `Retry-After` header indicating the number of seconds before the client may retry.

### 3.6 Request Size Limits

**REQ-API-018** — The API SHALL enforce the following request size limits:

| Request Type | Maximum Size |
|-------------|-------------|
| Standard JSON body | 1 MB |
| File upload (single file) | 25 MB |
| CSV import (target lists) | 50 MB |
| Batch operations | 5 MB |

---

## 4. Authentication API

### 4.1 Endpoints

**REQ-API-019** — The API SHALL expose the following authentication endpoints:

| # | Method | Path | Description | Auth Required | Required Permission |
|---|--------|------|-------------|---------------|-------------------|
| 1 | `POST` | `/api/v1/auth/login` | Authenticate with local credentials | No | None |
| 2 | `POST` | `/api/v1/auth/logout` | Invalidate the current refresh token | Yes | Any authenticated user |
| 3 | `POST` | `/api/v1/auth/refresh` | Exchange a refresh token for a new access token | No (refresh token in body) | None |
| 4 | `POST` | `/api/v1/auth/setup` | Create the initial admin account (first-run only) | No | None (disabled after first use) |
| 5 | `GET` | `/api/v1/auth/providers` | List enabled authentication providers | No | None |
| 6 | `POST` | `/api/v1/auth/oidc/callback` | Handle OIDC provider callback | No | None |
| 7 | `POST` | `/api/v1/auth/ldap/login` | Authenticate via LDAP | No | None |
| 8 | `GET` | `/api/v1/auth/me` | Get the current authenticated user's profile and permissions | Yes | Any authenticated user |

**REQ-API-020** — The login endpoint (`POST /api/v1/auth/login`) SHALL accept:

```json
{
  "username": "string",
  "password": "string"
}
```

And return on success:

```json
{
  "data": {
    "access_token": "eyJhbGciOi...",
    "refresh_token": "dGhpcyBpcyBh...",
    "token_type": "Bearer",
    "expires_in": 900,
    "user": {
      "id": "uuid",
      "username": "string",
      "display_name": "string",
      "roles": ["admin"],
      "permissions": ["campaigns.create", "campaigns.read", ...]
    }
  }
}
```

**REQ-API-021** — The setup endpoint (`POST /api/v1/auth/setup`) SHALL only be callable when no users exist in the system. After the initial admin is created, the endpoint SHALL return `403 FORBIDDEN` on all subsequent calls.

**REQ-API-022** — JWT access tokens SHALL have a configurable expiry (default: 15 minutes). Refresh tokens SHALL have a configurable expiry (default: 7 days). Refresh tokens SHALL be single-use; exchanging a refresh token SHALL invalidate it and issue a new refresh token alongside the new access token.

**REQ-API-023** — The `GET /api/v1/auth/me` endpoint SHALL return the full user profile including all assigned roles, resolved permissions (including permissions inherited from roles), and active authentication provider.

---

## 5. User Management API

### 5.1 Endpoints

**REQ-API-024** — The API SHALL expose the following user management endpoints:

| # | Method | Path | Description | Auth Required | Required Permission |
|---|--------|------|-------------|---------------|-------------------|
| 1 | `GET` | `/api/v1/users` | List all users (paginated, filterable) | Yes | `users.read` |
| 2 | `POST` | `/api/v1/users` | Create a new user | Yes | `users.create` |
| 3 | `GET` | `/api/v1/users/{id}` | Get user by ID | Yes | `users.read` |
| 4 | `PUT` | `/api/v1/users/{id}` | Update a user | Yes | `users.update` |
| 5 | `DELETE` | `/api/v1/users/{id}` | Deactivate a user (soft delete) | Yes | `users.delete` |
| 6 | `PUT` | `/api/v1/users/{id}/roles` | Assign roles to a user | Yes | `users.assign_roles` |
| 7 | `PUT` | `/api/v1/users/{id}/password` | Change a user's password | Yes | `users.update` or self |
| 8 | `GET` | `/api/v1/users/{id}/activity` | Get user activity/audit log | Yes | `users.read` |
| 9 | `PUT` | `/api/v1/users/me/profile` | Update own profile | Yes | Any authenticated user |
| 10 | `PUT` | `/api/v1/users/me/password` | Change own password | Yes | Any authenticated user |

**REQ-API-025** — The user list endpoint SHALL support filtering by `status` (active, inactive), `role`, and `search` (searches username, display name, email). It SHALL support sorting by `username`, `display_name`, `created_at`, and `last_login_at`.

**REQ-API-026** — A user SHALL NOT be able to delete their own account or remove their own Admin role. The API SHALL return a `409 CONFLICT` error if this is attempted.

**REQ-API-027** — Password change endpoints SHALL require the current password when the user is changing their own password. Admin-initiated password resets SHALL NOT require the current password.

---

## 6. Roles & Permissions API

### 6.1 Endpoints

**REQ-API-028** — The API SHALL expose the following roles and permissions endpoints:

| # | Method | Path | Description | Auth Required | Required Permission |
|---|--------|------|-------------|---------------|-------------------|
| 1 | `GET` | `/api/v1/roles` | List all roles | Yes | `roles.read` |
| 2 | `POST` | `/api/v1/roles` | Create a custom role | Yes | `roles.create` |
| 3 | `GET` | `/api/v1/roles/{id}` | Get role by ID with permissions | Yes | `roles.read` |
| 4 | `PUT` | `/api/v1/roles/{id}` | Update a role (name, description, permissions) | Yes | `roles.update` |
| 5 | `DELETE` | `/api/v1/roles/{id}` | Delete a custom role | Yes | `roles.delete` |
| 6 | `GET` | `/api/v1/permissions` | List all available permissions | Yes | `roles.read` |
| 7 | `GET` | `/api/v1/roles/{id}/users` | List users assigned to a role | Yes | `roles.read` |

**REQ-API-029** — Built-in roles (Admin, Engineer, Operator, Viewer) SHALL NOT be deletable. The API SHALL return a `409 CONFLICT` if deletion of a built-in role is attempted.

**REQ-API-030** — Deleting a custom role SHALL fail with a `409 CONFLICT` if any users are currently assigned to that role. The response SHALL include the count and IDs of affected users.

**REQ-API-031** — The permissions list endpoint (`GET /api/v1/permissions`) SHALL return all available permissions grouped by resource area (e.g., `campaigns.*`, `users.*`, `infrastructure.*`) with human-readable descriptions for each permission.

---

## 7. Campaign Management API

### 7.1 Endpoints

**REQ-API-032** — The API SHALL expose the following campaign management endpoints:

| # | Method | Path | Description | Auth Required | Required Permission |
|---|--------|------|-------------|---------------|-------------------|
| 1 | `GET` | `/api/v1/campaigns` | List all campaigns (paginated, filterable) | Yes | `campaigns.read` |
| 2 | `POST` | `/api/v1/campaigns` | Create a new campaign | Yes | `campaigns.create` |
| 3 | `GET` | `/api/v1/campaigns/{id}` | Get campaign by ID (full detail) | Yes | `campaigns.read` |
| 4 | `PUT` | `/api/v1/campaigns/{id}` | Update campaign configuration | Yes | `campaigns.update` |
| 5 | `DELETE` | `/api/v1/campaigns/{id}` | Soft-delete a campaign | Yes | `campaigns.delete` |
| 6 | `POST` | `/api/v1/campaigns/{id}/submit` | Submit campaign for approval | Yes | `campaigns.submit` |
| 7 | `POST` | `/api/v1/campaigns/{id}/approve` | Approve a submitted campaign | Yes | `campaigns.approve` |
| 8 | `POST` | `/api/v1/campaigns/{id}/reject` | Reject a submitted campaign (with reason) | Yes | `campaigns.approve` |
| 9 | `POST` | `/api/v1/campaigns/{id}/build` | Trigger the campaign build process (compile landing page, package payload) | Yes | `campaigns.build` |
| 10 | `POST` | `/api/v1/campaigns/{id}/launch` | Launch the campaign (begin sending) | Yes | `campaigns.launch` |
| 11 | `POST` | `/api/v1/campaigns/{id}/pause` | Pause a running campaign | Yes | `campaigns.launch` |
| 12 | `POST` | `/api/v1/campaigns/{id}/resume` | Resume a paused campaign | Yes | `campaigns.launch` |
| 13 | `POST` | `/api/v1/campaigns/{id}/complete` | Mark a campaign as complete (stop all activity) | Yes | `campaigns.complete` |
| 14 | `POST` | `/api/v1/campaigns/{id}/archive` | Archive a completed campaign | Yes | `campaigns.update` |
| 15 | `POST` | `/api/v1/campaigns/{id}/unlock` | Unlock a campaign for editing (returns to draft from rejected/approved) | Yes | `campaigns.approve` |
| 16 | `GET` | `/api/v1/campaigns/{id}/status` | Get current campaign status and progress summary | Yes | `campaigns.read` |
| 17 | `GET` | `/api/v1/campaigns/{id}/timeline` | Get campaign event timeline (state transitions, actions) | Yes | `campaigns.read` |
| 18 | `POST` | `/api/v1/campaigns/{id}/clone` | Clone a campaign (deep copy configuration, templates, targets) | Yes | `campaigns.create` |

**REQ-API-033** — The campaign list endpoint SHALL support filtering by `status` (draft, submitted, approved, rejected, building, ready, running, paused, completed, archived), `created_by`, `date range`, `domain_id`, and `search` (searches campaign name and description).

**REQ-API-034** — Campaign state transition endpoints (submit, approve, reject, build, launch, pause, resume, complete, archive, unlock) SHALL enforce the campaign state machine. Invalid transitions SHALL return a `409 CONFLICT` error with the current state and the list of valid transitions from that state.

**REQ-API-035** — The reject endpoint SHALL require a `reason` field in the request body. The reason SHALL be stored and visible in the campaign timeline.

**REQ-API-036** — The build endpoint SHALL return `202 Accepted` with a build ID. The client SHALL poll `GET /api/v1/campaigns/{id}/status` or subscribe to the `ws/campaign/{id}` WebSocket for build progress updates.

**REQ-API-037** — The launch endpoint SHALL perform pre-flight validation before initiating the campaign:

- All required fields are populated (template, targets, SMTP, domain, endpoint)
- SMTP profiles are reachable
- Email authentication records are valid
- Phishing endpoint is healthy
- Landing page is compiled and deployed

If any pre-flight check fails, the endpoint SHALL return a `422 UNPROCESSABLE_ENTITY` response with details of each failed check.

---

## 8. Email Template API

### 8.1 Endpoints

**REQ-API-038** — The API SHALL expose the following email template endpoints:

| # | Method | Path | Description | Auth Required | Required Permission |
|---|--------|------|-------------|---------------|-------------------|
| 1 | `GET` | `/api/v1/templates` | List all email templates (paginated) | Yes | `templates.read` |
| 2 | `POST` | `/api/v1/templates` | Create a new email template | Yes | `templates.create` |
| 3 | `GET` | `/api/v1/templates/{id}` | Get template by ID | Yes | `templates.read` |
| 4 | `PUT` | `/api/v1/templates/{id}` | Update a template | Yes | `templates.update` |
| 5 | `DELETE` | `/api/v1/templates/{id}` | Delete a template | Yes | `templates.delete` |
| 6 | `POST` | `/api/v1/templates/{id}/preview` | Render a template with sample/target data | Yes | `templates.read` |
| 7 | `POST` | `/api/v1/templates/{id}/test-send` | Send a test email using the template | Yes | `templates.test` |
| 8 | `POST` | `/api/v1/templates/{id}/clone` | Clone a template | Yes | `templates.create` |
| 9 | `POST` | `/api/v1/templates/{id}/attachments` | Upload an attachment to a template | Yes | `templates.update` |
| 10 | `DELETE` | `/api/v1/templates/{id}/attachments/{attachment_id}` | Remove an attachment from a template | Yes | `templates.update` |

**REQ-API-039** — The preview endpoint SHALL accept an optional target record (or sample data) in the request body and return the fully rendered email including subject, HTML body, plain-text body, and all headers with variable substitution applied.

**REQ-API-040** — The test-send endpoint SHALL accept:

```json
{
  "recipient_email": "test@example.com",
  "smtp_profile_id": "uuid",
  "sample_data": {
    "target": {
      "first_name": "John",
      "last_name": "Doe",
      "email": "john.doe@target.com"
    }
  }
}
```

And SHALL return the SMTP transaction result (success/failure, response code, response text).

**REQ-API-041** — Deleting a template that is currently associated with a running or scheduled campaign SHALL return a `409 CONFLICT` with the list of affected campaigns.

---

## 9. Landing Page API

### 9.1 Endpoints

**REQ-API-042** — The API SHALL expose the following landing page endpoints:

| # | Method | Path | Description | Auth Required | Required Permission |
|---|--------|------|-------------|---------------|-------------------|
| 1 | `GET` | `/api/v1/landing-pages` | List all landing pages (paginated) | Yes | `landing_pages.read` |
| 2 | `POST` | `/api/v1/landing-pages` | Create a new landing page | Yes | `landing_pages.create` |
| 3 | `GET` | `/api/v1/landing-pages/{id}` | Get landing page by ID | Yes | `landing_pages.read` |
| 4 | `PUT` | `/api/v1/landing-pages/{id}` | Update a landing page | Yes | `landing_pages.update` |
| 5 | `DELETE` | `/api/v1/landing-pages/{id}` | Delete a landing page | Yes | `landing_pages.delete` |
| 6 | `POST` | `/api/v1/landing-pages/{id}/compile` | Compile the landing page into a deployable artifact | Yes | `landing_pages.build` |
| 7 | `POST` | `/api/v1/landing-pages/{id}/deploy` | Deploy the compiled artifact to the framework server | Yes | `landing_pages.deploy` |
| 8 | `GET` | `/api/v1/landing-pages/{id}/status` | Get build/deploy status | Yes | `landing_pages.read` |
| 9 | `POST` | `/api/v1/landing-pages/{id}/clone` | Clone a landing page | Yes | `landing_pages.create` |
| 10 | `GET` | `/api/v1/landing-pages/{id}/preview` | Get a preview URL for the landing page | Yes | `landing_pages.read` |

**REQ-API-043** — The compile endpoint SHALL return `202 Accepted` and process the build asynchronously. The build status SHALL be available via the status endpoint and the campaign WebSocket.

**REQ-API-044** — The deploy endpoint SHALL only be callable on a landing page that has been successfully compiled. Calling deploy on an uncompiled landing page SHALL return a `409 CONFLICT`.

**REQ-API-045** — Deleting a landing page that is deployed or associated with an active campaign SHALL return a `409 CONFLICT`.

---

## 10. Target Management API

### 10.1 Endpoints

**REQ-API-046** — The API SHALL expose the following target management endpoints:

| # | Method | Path | Description | Auth Required | Required Permission |
|---|--------|------|-------------|---------------|-------------------|
| 1 | `GET` | `/api/v1/targets` | List all targets (paginated, filterable) | Yes | `targets.read` |
| 2 | `POST` | `/api/v1/targets` | Create a single target | Yes | `targets.create` |
| 3 | `GET` | `/api/v1/targets/{id}` | Get target by ID | Yes | `targets.read` |
| 4 | `PUT` | `/api/v1/targets/{id}` | Update a target | Yes | `targets.update` |
| 5 | `DELETE` | `/api/v1/targets/{id}` | Delete a target | Yes | `targets.delete` |
| 6 | `POST` | `/api/v1/targets/import` | Import targets from CSV file | Yes | `targets.import` |
| 7 | `GET` | `/api/v1/targets/import/{job_id}` | Get import job status and results | Yes | `targets.import` |
| 8 | `GET` | `/api/v1/targets/export` | Export targets as CSV | Yes | `targets.export` |
| 9 | `GET` | `/api/v1/target-groups` | List target groups | Yes | `targets.read` |
| 10 | `POST` | `/api/v1/target-groups` | Create a target group | Yes | `targets.create` |
| 11 | `GET` | `/api/v1/target-groups/{id}` | Get target group by ID with members | Yes | `targets.read` |
| 12 | `PUT` | `/api/v1/target-groups/{id}` | Update a target group | Yes | `targets.update` |
| 13 | `DELETE` | `/api/v1/target-groups/{id}` | Delete a target group | Yes | `targets.delete` |
| 14 | `POST` | `/api/v1/target-groups/{id}/members` | Add targets to a group | Yes | `targets.update` |
| 15 | `DELETE` | `/api/v1/target-groups/{id}/members` | Remove targets from a group | Yes | `targets.update` |
| 16 | `GET` | `/api/v1/blocklist` | List blocked email addresses/domains | Yes | `targets.read` |
| 17 | `POST` | `/api/v1/blocklist` | Add entries to the block list | Yes | `targets.blocklist` |
| 18 | `DELETE` | `/api/v1/blocklist/{id}` | Remove an entry from the block list | Yes | `targets.blocklist` |
| 19 | `POST` | `/api/v1/blocklist/import` | Import block list entries from CSV | Yes | `targets.blocklist` |

**REQ-API-047** — The target list endpoint SHALL support filtering by `group_id`, `department`, `position`, `email domain`, `campaign_id` (targets associated with a campaign), and `search` (searches name, email, department, position).

**REQ-API-048** — The CSV import endpoint SHALL accept `multipart/form-data` with a CSV file and return `202 Accepted` with a job ID. The import SHALL be processed asynchronously. The job status endpoint SHALL report:

```json
{
  "data": {
    "job_id": "uuid",
    "status": "processing",
    "total_rows": 5000,
    "processed_rows": 3200,
    "created_count": 3100,
    "updated_count": 50,
    "skipped_count": 30,
    "error_count": 20,
    "errors": [
      {
        "row": 145,
        "field": "email",
        "message": "Invalid email format"
      }
    ]
  }
}
```

**REQ-API-049** — The block list SHALL be enforced at campaign launch time and during email sending. Targets whose email address or email domain matches a block list entry SHALL be excluded from sends. The block list check SHALL be performed server-side; the client SHALL NOT be responsible for enforcement.

---

## 11. Domain Management API

### 11.1 Endpoints

**REQ-API-050** — The API SHALL expose the following domain management endpoints:

| # | Method | Path | Description | Auth Required | Required Permission |
|---|--------|------|-------------|---------------|-------------------|
| 1 | `GET` | `/api/v1/domains` | List all domains (paginated, filterable) | Yes | `domains.read` |
| 2 | `POST` | `/api/v1/domains` | Create/register a new domain | Yes | `domains.create` |
| 3 | `GET` | `/api/v1/domains/{id}` | Get domain by ID (full profile) | Yes | `domains.read` |
| 4 | `PUT` | `/api/v1/domains/{id}` | Update domain profile | Yes | `domains.update` |
| 5 | `DELETE` | `/api/v1/domains/{id}` | Decommission a domain (soft delete) | Yes | `domains.delete` |
| 6 | `POST` | `/api/v1/domains/{id}/check-health` | Trigger an on-demand health check | Yes | `domains.read` |
| 7 | `GET` | `/api/v1/domains/{id}/health-history` | Get health check history | Yes | `domains.read` |
| 8 | `GET` | `/api/v1/domains/{id}/dns-records` | List DNS records for a domain | Yes | `domains.dns.read` |
| 9 | `POST` | `/api/v1/domains/{id}/dns-records` | Create a DNS record | Yes | `domains.dns.manage` |
| 10 | `PUT` | `/api/v1/domains/{id}/dns-records/{record_id}` | Update a DNS record | Yes | `domains.dns.manage` |
| 11 | `DELETE` | `/api/v1/domains/{id}/dns-records/{record_id}` | Delete a DNS record | Yes | `domains.dns.manage` |
| 12 | `GET` | `/api/v1/domains/{id}/email-auth` | Get email authentication status (SPF, DKIM, DMARC) | Yes | `domains.dns.read` |
| 13 | `PUT` | `/api/v1/domains/{id}/email-auth/spf` | Configure/publish SPF record | Yes | `domains.dns.manage` |
| 14 | `PUT` | `/api/v1/domains/{id}/email-auth/dkim` | Configure/publish DKIM record | Yes | `domains.dns.manage` |
| 15 | `PUT` | `/api/v1/domains/{id}/email-auth/dmarc` | Configure/publish DMARC record | Yes | `domains.dns.manage` |
| 16 | `POST` | `/api/v1/domains/{id}/email-auth/validate` | Validate all email auth records against live DNS | Yes | `domains.dns.read` |
| 17 | `GET` | `/api/v1/domain-providers` | List configured domain provider connections | Yes | `domains.providers.read` |
| 18 | `POST` | `/api/v1/domain-providers` | Create a domain provider connection | Yes | `domains.providers.manage` |
| 19 | `GET` | `/api/v1/domain-providers/{id}` | Get provider connection details | Yes | `domains.providers.read` |
| 20 | `PUT` | `/api/v1/domain-providers/{id}` | Update a provider connection | Yes | `domains.providers.manage` |
| 21 | `DELETE` | `/api/v1/domain-providers/{id}` | Delete a provider connection | Yes | `domains.providers.manage` |
| 22 | `POST` | `/api/v1/domain-providers/{id}/test` | Test a provider connection | Yes | `domains.providers.read` |

**REQ-API-051** — The domain list endpoint SHALL support filtering by `status` (pending_registration, active, expired, suspended, decommissioned), `registrar`, `dns_provider`, `tag`, `expiry_before`, `expiry_after`, and `search` (searches domain name, tags, notes).

**REQ-API-052** — The health check endpoint SHALL return `202 Accepted` and execute the health check asynchronously. Results SHALL be available via the health-history endpoint and pushed via the dashboard WebSocket.

**REQ-API-053** — DNS record mutation endpoints SHALL validate inputs server-side (valid IPv4 for A records, valid IPv6 for AAAA, valid hostname for CNAME, MX priority present, etc.) before submitting to the provider API.

---

## 12. Infrastructure Management API

### 12.1 Endpoints

**REQ-API-054** — The API SHALL expose the following infrastructure management endpoints:

| # | Method | Path | Description | Auth Required | Required Permission |
|---|--------|------|-------------|---------------|-------------------|
| 1 | `GET` | `/api/v1/infrastructure/providers` | List configured cloud provider credential sets | Yes | `infrastructure.providers.read` |
| 2 | `POST` | `/api/v1/infrastructure/providers` | Create a cloud provider credential set | Yes | `infrastructure.providers.manage` |
| 3 | `GET` | `/api/v1/infrastructure/providers/{id}` | Get provider credential set details (secrets masked) | Yes | `infrastructure.providers.read` |
| 4 | `PUT` | `/api/v1/infrastructure/providers/{id}` | Update a provider credential set | Yes | `infrastructure.providers.manage` |
| 5 | `DELETE` | `/api/v1/infrastructure/providers/{id}` | Delete a provider credential set | Yes | `infrastructure.providers.manage` |
| 6 | `POST` | `/api/v1/infrastructure/providers/{id}/test` | Test cloud provider credentials | Yes | `infrastructure.providers.read` |
| 7 | `GET` | `/api/v1/infrastructure/templates` | List instance templates | Yes | `infrastructure.templates.read` |
| 8 | `POST` | `/api/v1/infrastructure/templates` | Create an instance template | Yes | `infrastructure.templates.manage` |
| 9 | `GET` | `/api/v1/infrastructure/templates/{id}` | Get instance template by ID | Yes | `infrastructure.templates.read` |
| 10 | `PUT` | `/api/v1/infrastructure/templates/{id}` | Update an instance template (creates new version) | Yes | `infrastructure.templates.manage` |
| 11 | `DELETE` | `/api/v1/infrastructure/templates/{id}` | Delete an instance template | Yes | `infrastructure.templates.manage` |
| 12 | `GET` | `/api/v1/infrastructure/instances` | List all instances (paginated, filterable) | Yes | `infrastructure.instances.read` |
| 13 | `POST` | `/api/v1/infrastructure/instances` | Provision a new instance | Yes | `infrastructure.instances.provision` |
| 14 | `GET` | `/api/v1/infrastructure/instances/{id}` | Get instance details (status, health, config) | Yes | `infrastructure.instances.read` |
| 15 | `POST` | `/api/v1/infrastructure/instances/{id}/stop` | Stop a running instance | Yes | `infrastructure.instances.manage` |
| 16 | `POST` | `/api/v1/infrastructure/instances/{id}/start` | Start a stopped instance | Yes | `infrastructure.instances.manage` |
| 17 | `POST` | `/api/v1/infrastructure/instances/{id}/terminate` | Terminate an instance permanently | Yes | `infrastructure.instances.terminate` |
| 18 | `POST` | `/api/v1/infrastructure/instances/{id}/redeploy` | Redeploy endpoint binary and configuration | Yes | `infrastructure.instances.manage` |
| 19 | `GET` | `/api/v1/infrastructure/instances/{id}/health` | Get instance health check history | Yes | `infrastructure.instances.read` |
| 20 | `GET` | `/api/v1/infrastructure/instances/{id}/logs` | Get instance provisioning/lifecycle logs | Yes | `infrastructure.instances.read` |
| 21 | `GET` | `/api/v1/infrastructure/costs` | Get cost summary (filterable by date, provider, campaign) | Yes | `infrastructure.costs.read` |
| 22 | `GET` | `/api/v1/infrastructure/costs/export` | Export cost data as CSV | Yes | `infrastructure.costs.read` |

**REQ-API-055** — The instance provisioning endpoint SHALL return `202 Accepted` and process provisioning asynchronously. The response SHALL include the instance ID for subsequent status polling:

```json
{
  "data": {
    "id": "uuid",
    "status": "provisioning",
    "message": "Instance provisioning initiated",
    "campaign_id": "uuid",
    "template_id": "uuid",
    "provider": "aws"
  }
}
```

**REQ-API-056** — The instance list endpoint SHALL support filtering by `status` (provisioning, configuring, running, stopping, stopped, terminating, terminated, error), `provider` (aws, azure), `campaign_id`, `region`, and `health_status` (healthy, unhealthy, unknown).

**REQ-API-057** — The terminate endpoint SHALL require a confirmation field in the request body:

```json
{
  "confirm_instance_id": "uuid"
}
```

The `confirm_instance_id` SHALL match the instance ID in the URL path. This prevents accidental termination through automated tooling or errant API calls.

**REQ-API-058** — Cloud provider credential sets SHALL never return unmasked secrets in API responses. Secret fields (access keys, client secrets) SHALL be returned as masked values (e.g., `"****abcd"`).

---

## 13. SMTP Configuration API

### 13.1 Endpoints

**REQ-API-059** — The API SHALL expose the following SMTP configuration endpoints:

| # | Method | Path | Description | Auth Required | Required Permission |
|---|--------|------|-------------|---------------|-------------------|
| 1 | `GET` | `/api/v1/smtp-profiles` | List all SMTP profiles | Yes | `smtp.read` |
| 2 | `POST` | `/api/v1/smtp-profiles` | Create an SMTP profile | Yes | `smtp.create` |
| 3 | `GET` | `/api/v1/smtp-profiles/{id}` | Get SMTP profile by ID (password masked) | Yes | `smtp.read` |
| 4 | `PUT` | `/api/v1/smtp-profiles/{id}` | Update an SMTP profile | Yes | `smtp.update` |
| 5 | `DELETE` | `/api/v1/smtp-profiles/{id}` | Delete an SMTP profile | Yes | `smtp.delete` |
| 6 | `POST` | `/api/v1/smtp-profiles/{id}/test` | Test SMTP connection | Yes | `smtp.test` |
| 7 | `POST` | `/api/v1/smtp-profiles/{id}/clone` | Clone an SMTP profile (excluding credentials) | Yes | `smtp.create` |

**REQ-API-060** — SMTP profile GET responses SHALL mask the `password` field. The password SHALL never be returned in plaintext. When updating an SMTP profile, omitting the `password` field SHALL leave the existing password unchanged.

**REQ-API-061** — The test connection endpoint SHALL return a structured result:

```json
{
  "data": {
    "success": true,
    "stages": {
      "tcp_connect": { "success": true, "duration_ms": 45 },
      "tls_handshake": { "success": true, "duration_ms": 120 },
      "authentication": { "success": true, "duration_ms": 30 },
      "smtp_command": { "success": true, "duration_ms": 15 }
    },
    "server_banner": "220 smtp.example.com ESMTP",
    "tls_version": "TLS 1.3",
    "tls_cipher": "TLS_AES_256_GCM_SHA384"
  }
}
```

**REQ-API-062** — Deleting an SMTP profile that is associated with a running or scheduled campaign SHALL return a `409 CONFLICT` with the list of affected campaign IDs and names.

---

## 14. Credential Capture API

### 14.1 Endpoints

**REQ-API-063** — The API SHALL expose the following credential capture endpoints:

| # | Method | Path | Description | Auth Required | Required Permission |
|---|--------|------|-------------|---------------|-------------------|
| 1 | `GET` | `/api/v1/credentials` | List captured credentials (paginated, filterable) | Yes | `credentials.read` |
| 2 | `GET` | `/api/v1/credentials/{id}` | Get a single captured credential record | Yes | `credentials.read` |
| 3 | `GET` | `/api/v1/campaigns/{id}/credentials` | List credentials captured for a specific campaign | Yes | `credentials.read` |
| 4 | `GET` | `/api/v1/credentials/export` | Export credentials as CSV (filtered by campaign, date range) | Yes | `credentials.export` |
| 5 | `DELETE` | `/api/v1/credentials/{id}` | Delete a captured credential record | Yes | `credentials.delete` |
| 6 | `POST` | `/api/v1/credentials/purge` | Bulk purge credentials by campaign or date range | Yes | `credentials.purge` |

**REQ-API-064** — The `credentials.read` permission SHALL be restricted to the Admin and Engineer roles by default. Operators and Viewers SHALL NOT have access to captured credential data unless explicitly granted the permission.

**REQ-API-065** — Credential list responses SHALL include the following fields per record: `id`, `campaign_id`, `target_id`, `target_email`, `submitted_at`, `landing_page_id`, `source_ip`, `user_agent`, `fields_captured` (list of field names), and `raw_data` (the actual submitted data, encrypted at rest). The `raw_data` field SHALL only be included when the requesting user has `credentials.read` permission and explicitly requests it via `?include_raw=true`.

**REQ-API-066** — The export endpoint SHALL support filtering by `campaign_id`, `date_after`, `date_before`, and `target_group_id`. The CSV SHALL contain field names, submitted values, target identifiers, and timestamps. Exported CSV files SHALL be generated server-side and streamed to the client; they SHALL NOT be stored on disk beyond the lifetime of the request.

**REQ-API-067** — The purge endpoint SHALL require a confirmation field and SHALL hard-delete credential data (not soft-delete). Purge operations SHALL be recorded in the audit log with the acting user, timestamp, and count of records purged.

---

## 15. Metrics & Reporting API

### 15.1 Endpoints

**REQ-API-068** — The API SHALL expose the following metrics and reporting endpoints:

| # | Method | Path | Description | Auth Required | Required Permission |
|---|--------|------|-------------|---------------|-------------------|
| 1 | `GET` | `/api/v1/campaigns/{id}/metrics` | Get campaign metrics (opens, clicks, submissions, delivery stats) | Yes | `metrics.read` |
| 2 | `GET` | `/api/v1/campaigns/{id}/metrics/timeline` | Get campaign metrics over time (time-series data) | Yes | `metrics.read` |
| 3 | `GET` | `/api/v1/campaigns/{id}/metrics/by-template` | Get per-template metrics for A/B campaigns | Yes | `metrics.read` |
| 4 | `GET` | `/api/v1/campaigns/{id}/metrics/by-department` | Get metrics broken down by target department | Yes | `metrics.read` |
| 5 | `GET` | `/api/v1/dashboard/summary` | Get dashboard summary (active campaigns, totals, recent activity) | Yes | `metrics.read` |
| 6 | `GET` | `/api/v1/dashboard/campaigns` | Get dashboard campaign cards (status, progress, key metrics) | Yes | `metrics.read` |
| 7 | `GET` | `/api/v1/reports/templates` | List available report templates | Yes | `reports.read` |
| 8 | `POST` | `/api/v1/reports/generate` | Generate a report from a template | Yes | `reports.generate` |
| 9 | `GET` | `/api/v1/reports/{id}` | Get a generated report (metadata) | Yes | `reports.read` |
| 10 | `GET` | `/api/v1/reports/{id}/download` | Download the generated report file (PDF/CSV) | Yes | `reports.read` |
| 11 | `GET` | `/api/v1/reports` | List previously generated reports | Yes | `reports.read` |
| 12 | `DELETE` | `/api/v1/reports/{id}` | Delete a generated report | Yes | `reports.delete` |

**REQ-API-069** — The campaign metrics endpoint SHALL return a comprehensive metrics object:

```json
{
  "data": {
    "campaign_id": "uuid",
    "total_targets": 500,
    "emails": {
      "sent": 480,
      "delivered": 475,
      "bounced": 5,
      "failed": 0,
      "pending": 20
    },
    "interactions": {
      "opened": 312,
      "open_rate": 0.6568,
      "clicked": 198,
      "click_rate": 0.4168,
      "submitted_credentials": 87,
      "submission_rate": 0.1831
    },
    "timing": {
      "first_open_at": "2026-02-20T10:15:00Z",
      "last_open_at": "2026-02-24T16:45:00Z",
      "first_click_at": "2026-02-20T10:17:00Z",
      "last_click_at": "2026-02-24T16:50:00Z",
      "median_time_to_click_seconds": 342
    },
    "updated_at": "2026-02-25T12:00:00Z"
  }
}
```

**REQ-API-070** — The timeline metrics endpoint SHALL accept `interval` (hour, day, week) and `date_after`/`date_before` parameters and return time-series data suitable for charting in the frontend.

**REQ-API-071** — The report generation endpoint SHALL return `202 Accepted` with a report ID. Report generation SHALL be asynchronous. The client SHALL poll the report status endpoint or listen on the WebSocket for completion.

---

## 16. Logging API

### 16.1 Endpoints

**REQ-API-072** — The API SHALL expose the following logging endpoints:

| # | Method | Path | Description | Auth Required | Required Permission |
|---|--------|------|-------------|---------------|-------------------|
| 1 | `GET` | `/api/v1/logs` | Query logs (paginated, filterable) | Yes | `logs.read` |
| 2 | `GET` | `/api/v1/logs/export` | Export logs as CSV or JSON (filtered) | Yes | `logs.export` |
| 3 | `GET` | `/api/v1/logs/sources` | List available log sources | Yes | `logs.read` |
| 4 | `GET` | `/api/v1/audit-logs` | Query audit log entries (paginated, filterable) | Yes | `audit.read` |
| 5 | `GET` | `/api/v1/audit-logs/export` | Export audit logs | Yes | `audit.export` |

**REQ-API-073** — The logs endpoint SHALL support filtering by:

| Parameter | Type | Description |
|-----------|------|-------------|
| `level` | string | Log level: `debug`, `info`, `warn`, `error`, `fatal` |
| `source` | string | Log source: `api`, `campaign`, `smtp`, `infrastructure`, `endpoint`, `system` |
| `campaign_id` | uuid | Logs associated with a specific campaign |
| `instance_id` | uuid | Logs associated with a specific infrastructure instance |
| `correlation_id` | uuid | All logs linked to a specific request |
| `search` | string | Full-text search across log message content |
| `after` | timestamp | Logs after this timestamp |
| `before` | timestamp | Logs before this timestamp |

**REQ-API-074** — The audit log endpoint SHALL support filtering by `user_id`, `action` (e.g., `campaign.created`, `user.login`, `instance.terminated`), `resource_type`, `resource_id`, `after`, and `before`.

**REQ-API-075** — Audit logs SHALL be immutable. The API SHALL NOT expose any endpoint to modify or delete audit log entries.

---

## 17. Application Settings API

### 17.1 Endpoints

**REQ-API-076** — The API SHALL expose the following application settings endpoints:

| # | Method | Path | Description | Auth Required | Required Permission |
|---|--------|------|-------------|---------------|-------------------|
| 1 | `GET` | `/api/v1/settings` | Get all application settings (grouped by category) | Yes | `settings.read` |
| 2 | `PUT` | `/api/v1/settings` | Update application settings (partial update supported) | Yes | `settings.update` |
| 3 | `GET` | `/api/v1/settings/{category}` | Get settings for a specific category | Yes | `settings.read` |
| 4 | `PUT` | `/api/v1/settings/{category}` | Update settings for a specific category | Yes | `settings.update` |
| 5 | `GET` | `/api/v1/settings/auth/providers` | Get authentication provider configurations | Yes | `settings.auth.read` |
| 6 | `PUT` | `/api/v1/settings/auth/providers/{provider}` | Configure an authentication provider (OIDC, LDAP, FusionAuth) | Yes | `settings.auth.manage` |
| 7 | `POST` | `/api/v1/settings/auth/providers/{provider}/test` | Test an authentication provider connection | Yes | `settings.auth.manage` |

**REQ-API-077** — Settings categories SHALL include at minimum: `general` (application name, timezone, retention policies), `auth` (authentication provider configurations), `notifications` (notification preferences, alert thresholds), `security` (password policies, session timeouts, rate limits), and `email` (default attachment size limits, allowed MIME types).

**REQ-API-078** — Settings update operations SHALL be recorded in the audit log with before/after values for each changed setting. Sensitive settings (secrets, credentials) SHALL be masked in audit records.

---

## 18. Health Check Endpoint

**REQ-API-079** — The API SHALL expose the following public health check endpoint:

| # | Method | Path | Description | Auth Required | Required Permission |
|---|--------|------|-------------|---------------|-------------------|
| 1 | `GET` | `/api/v1/health` | Application health check | No | None |
| 2 | `GET` | `/api/v1/health/detailed` | Detailed health with dependency status | Yes | `settings.read` |

**REQ-API-080** — The public health endpoint SHALL return:

```json
{
  "status": "healthy",
  "version": "1.0.0",
  "timestamp": "2026-02-25T12:00:00Z"
}
```

**REQ-API-081** — The detailed health endpoint SHALL include the status of all dependencies:

```json
{
  "status": "healthy",
  "version": "1.0.0",
  "timestamp": "2026-02-25T12:00:00Z",
  "dependencies": {
    "database": { "status": "healthy", "latency_ms": 2 },
    "websocket_hub": { "status": "healthy", "active_connections": 5 },
    "background_workers": { "status": "healthy", "active_jobs": 3, "queued_jobs": 12 }
  }
}
```

---

## 18A. Extended API Endpoints

This section defines API endpoints for features added across the requirements documents that extend the core resource endpoints defined in sections 7–17.

### 18A.1 Campaign Extensions

**REQ-API-114** — The API SHALL expose the following campaign extension endpoints:

| # | Method | Path | Description | Auth Required | Required Permission |
|---|--------|------|-------------|---------------|-------------------|
| 1 | `POST` | `/api/v1/campaigns/{id}/clone` | Clone a campaign with selective component inclusion | Yes | `campaigns.create` |
| 2 | `GET` | `/api/v1/campaigns/calendar` | Get campaigns for calendar view (date-range query) | Yes | `campaigns.read` |
| 3 | `POST` | `/api/v1/campaigns/{id}/dry-run` | Execute a dry run simulation of the campaign | Yes | `campaigns.launch` |
| 4 | `GET` | `/api/v1/campaigns/{id}/dry-run` | Get dry run results | Yes | `campaigns.read` |
| 5 | `GET` | `/api/v1/campaign-templates` | List campaign templates | Yes | `campaigns.read` |
| 6 | `POST` | `/api/v1/campaign-templates` | Create a campaign template from an existing campaign | Yes | `campaigns.create` |
| 7 | `GET` | `/api/v1/campaign-templates/{id}` | Get campaign template by ID | Yes | `campaigns.read` |
| 8 | `PUT` | `/api/v1/campaign-templates/{id}` | Update a campaign template | Yes | `campaigns.update` |
| 9 | `DELETE` | `/api/v1/campaign-templates/{id}` | Delete a campaign template | Yes | `campaigns.delete` |
| 10 | `POST` | `/api/v1/campaign-templates/{id}/apply` | Create a new campaign from a template | Yes | `campaigns.create` |
| 11 | `POST` | `/api/v1/campaigns/{id}/canary-targets` | Designate canary targets for a campaign | Yes | `campaigns.update` |
| 12 | `GET` | `/api/v1/campaigns/{id}/canary-targets` | List canary targets for a campaign | Yes | `campaigns.read` |
| 13 | `DELETE` | `/api/v1/campaigns/{id}/canary-targets/{target_id}` | Remove a canary target designation | Yes | `campaigns.update` |

> **Note:** The clone endpoint (`POST /api/v1/campaigns/{id}/clone`) already exists in section 7.1. The entry here adds the **selective clone** behavior detail — the request body accepts a `components` array specifying which parts to clone (targets, templates, landing_page, smtp, domain, schedule, endpoint).

**REQ-API-115** — The clone endpoint SHALL accept a request body with a `components` array. Valid component values: `targets`, `templates`, `landing_page`, `smtp_profile`, `domain`, `schedule`, `endpoint`. If `components` is omitted, all components are cloned. The new campaign is created in `draft` status.

**REQ-API-116** — The calendar endpoint SHALL accept `start_date` and `end_date` query parameters and return campaigns with their `scheduled_launch_at`, `started_at`, `completed_at` dates and current status, suitable for rendering in a calendar view.

**REQ-API-117** — The dry run endpoint SHALL return `202 Accepted` and simulate campaign execution without sending real emails or provisioning infrastructure. Results include: target count, email rendering validation, SMTP connectivity check, DNS record validation, landing page build status, and estimated timeline.

**REQ-API-118** — Campaign template CRUD follows standard conventions. Templates store a snapshot of campaign configuration that can be applied to create new campaigns. The apply endpoint creates a new campaign in `draft` status with the template's configuration pre-populated.

**REQ-API-119** — Canary target endpoints manage the designation of specific targets as canary targets for a campaign. Canary targets are sent emails first and their interactions are excluded from campaign metrics.

### 18A.2 Landing Page Extensions

**REQ-API-120** — The API SHALL expose the following landing page extension endpoints:

| # | Method | Path | Description | Auth Required | Required Permission |
|---|--------|------|-------------|---------------|-------------------|
| 1 | `POST` | `/api/v1/landing-pages/import` | Import a landing page from HTML file/ZIP/paste | Yes | `landing_pages.create` |
| 2 | `POST` | `/api/v1/landing-pages/clone-url` | Clone a landing page from a URL | Yes | `landing_pages.create` |
| 3 | `POST` | `/api/v1/landing-pages/{id}/duplicate` | Duplicate an existing landing page project | Yes | `landing_pages.create` |

**REQ-API-121** — The import endpoint SHALL accept `multipart/form-data` with an HTML file, ZIP archive, or raw HTML in a JSON body. It SHALL support two import modes: `builder` (parse HTML into builder components) and `raw` (preserve original HTML with a code editor). The response includes a preview of the parsed result before committing.

**REQ-API-122** — The clone-url endpoint SHALL accept a URL, fetch the page and all linked assets (CSS, JS, images, fonts), localize all external references, and create a new landing page project. Asset download follows a configurable timeout (default 30s) and maximum total size (default 50MB).

**REQ-API-123** — The duplicate endpoint creates a full copy of an existing landing page project including all pages, components, styles, and assets, with a new name.

### 18A.3 Email Template Library

**REQ-API-124** — The API SHALL expose the following email template library endpoints:

| # | Method | Path | Description | Auth Required | Required Permission |
|---|--------|------|-------------|---------------|-------------------|
| 1 | `GET` | `/api/v1/email-template-library` | List built-in email templates (filterable by category, difficulty) | Yes | `templates.read` |
| 2 | `GET` | `/api/v1/email-template-library/{id}` | Get a library template with full content | Yes | `templates.read` |
| 3 | `POST` | `/api/v1/email-template-library/{id}/copy-to-templates` | Copy a library template to the user's template collection | Yes | `templates.create` |
| 4 | `POST` | `/api/v1/email-template-library` | Add a custom template to the library (Admin only) | Yes | `templates.library.manage` |
| 5 | `DELETE` | `/api/v1/email-template-library/{id}` | Remove a custom template from the library (Admin only) | Yes | `templates.library.manage` |

**REQ-API-125** — The library list endpoint SHALL support filtering by `category` (credential_harvesting, document_lure, it_notification, hr_corporate, social_engineering), `difficulty` (beginner, intermediate, advanced), and `search`. Library templates marked `is_system=true` cannot be deleted.

### 18A.4 Session Capture

**REQ-API-126** — The API SHALL expose the following session capture endpoints:

| # | Method | Path | Description | Auth Required | Required Permission |
|---|--------|------|-------------|---------------|-------------------|
| 1 | `GET` | `/api/v1/campaigns/{id}/session-captures` | List session capture events for a campaign | Yes | `credentials.read` |
| 2 | `GET` | `/api/v1/session-captures/{id}` | Get a single session capture event | Yes | `credentials.read` |
| 3 | `DELETE` | `/api/v1/session-captures/{id}` | Delete a session capture event | Yes | `credentials.delete` |
| 4 | `POST` | `/api/v1/session-captures/purge` | Bulk purge session captures by campaign or date range | Yes | `credentials.purge` |

**REQ-API-127** — Session capture endpoints follow the same access control as credential endpoints (`credentials.read` permission restricted to Admin and Engineer by default). The list endpoint supports filtering by `data_type` (cookie, oauth_token, session_token, auth_header), `target_id`, `is_time_sensitive`, and date range.

### 18A.5 Notification System

**REQ-API-128** — The API SHALL expose the following notification and webhook endpoints:

| # | Method | Path | Description | Auth Required | Required Permission |
|---|--------|------|-------------|---------------|-------------------|
| 1 | `GET` | `/api/v1/notifications` | List notifications for the current user | Yes | Any authenticated user |
| 2 | `GET` | `/api/v1/notifications/unread-count` | Get unread notification count | Yes | Any authenticated user |
| 3 | `PUT` | `/api/v1/notifications/{id}/read` | Mark a notification as read | Yes | Any authenticated user |
| 4 | `PUT` | `/api/v1/notifications/read-all` | Mark all notifications as read | Yes | Any authenticated user |
| 5 | `DELETE` | `/api/v1/notifications/{id}` | Delete a notification | Yes | Any authenticated user |
| 6 | `GET` | `/api/v1/notifications/preferences` | Get current user's notification preferences | Yes | Any authenticated user |
| 7 | `PUT` | `/api/v1/notifications/preferences` | Update notification preferences | Yes | Any authenticated user |
| 8 | `GET` | `/api/v1/webhooks` | List webhook endpoints | Yes | `settings.update` |
| 9 | `POST` | `/api/v1/webhooks` | Create a webhook endpoint | Yes | `settings.update` |
| 10 | `GET` | `/api/v1/webhooks/{id}` | Get webhook endpoint details | Yes | `settings.update` |
| 11 | `PUT` | `/api/v1/webhooks/{id}` | Update a webhook endpoint | Yes | `settings.update` |
| 12 | `DELETE` | `/api/v1/webhooks/{id}` | Delete a webhook endpoint | Yes | `settings.update` |
| 13 | `POST` | `/api/v1/webhooks/{id}/test` | Send a test webhook delivery | Yes | `settings.update` |
| 14 | `GET` | `/api/v1/webhooks/{id}/deliveries` | List webhook delivery history | Yes | `settings.update` |

**REQ-API-129** — Notification list supports filtering by `category`, `severity` (info, warning, critical), `is_read`, and date range. Notifications are user-scoped; each user sees only their own notifications.

**REQ-API-130** — Webhook endpoints support `auth_type` values of `hmac_sha256`, `bearer_token`, `basic_auth`, and `none`. Webhook secrets are encrypted at rest and masked in GET responses.

**REQ-API-131** — The test webhook endpoint sends a sample payload to verify connectivity and authentication. The delivery history endpoint shows recent deliveries with status codes, response times, and retry counts.

### 18A.6 Universal Tags

**REQ-API-132** — The API SHALL expose the following universal tag endpoints:

| # | Method | Path | Description | Auth Required | Required Permission |
|---|--------|------|-------------|---------------|-------------------|
| 1 | `GET` | `/api/v1/tags` | List all tags (with usage counts) | Yes | Any authenticated user |
| 2 | `POST` | `/api/v1/tags` | Create a tag | Yes | Any authenticated user |
| 3 | `PUT` | `/api/v1/tags/{id}` | Rename a tag | Yes | `settings.update` |
| 4 | `DELETE` | `/api/v1/tags/{id}` | Delete a tag (removes all associations) | Yes | `settings.update` |
| 5 | `POST` | `/api/v1/{entity_type}/{id}/tags` | Add tags to an entity | Yes | `{entity_type}.update` |
| 6 | `DELETE` | `/api/v1/{entity_type}/{id}/tags/{tag_id}` | Remove a tag from an entity | Yes | `{entity_type}.update` |

**REQ-API-133** — Universal tags can be applied to any primary entity type: `campaigns`, `templates`, `landing-pages`, `domains`, `smtp-profiles`, `targets`, `target-groups`. The `{entity_type}` path parameter matches the resource's existing route prefix. Tag names are case-insensitive and unique. Autocomplete is supported via `?search=` on the tags list endpoint.

### 18A.7 Alert Rules

**REQ-API-134** — The API SHALL expose the following alert rule endpoints:

| # | Method | Path | Description | Auth Required | Required Permission |
|---|--------|------|-------------|---------------|-------------------|
| 1 | `GET` | `/api/v1/alert-rules` | List all alert rules | Yes | `audit.read` |
| 2 | `POST` | `/api/v1/alert-rules` | Create an alert rule | Yes | `settings.update` |
| 3 | `GET` | `/api/v1/alert-rules/{id}` | Get alert rule by ID | Yes | `audit.read` |
| 4 | `PUT` | `/api/v1/alert-rules/{id}` | Update an alert rule | Yes | `settings.update` |
| 5 | `DELETE` | `/api/v1/alert-rules/{id}` | Delete an alert rule | Yes | `settings.update` |
| 6 | `POST` | `/api/v1/alert-rules/{id}/test` | Test an alert rule against recent audit log entries | Yes | `settings.update` |
| 7 | `GET` | `/api/v1/alert-rules/templates` | List built-in alert rule templates | Yes | `audit.read` |
| 8 | `POST` | `/api/v1/alert-rules/templates/{id}/duplicate` | Duplicate a built-in template for customization | Yes | `settings.update` |

**REQ-API-135** — Alert rules define conditions (audit log category, severity, action pattern match, threshold count within time window, absence of expected event) and actions (in-app notification, email, webhook, severity escalation). Rules have a configurable cooldown period to prevent alert storms. Built-in templates (is_system=true) cannot be modified or deleted; they must be duplicated first.

### 18A.8 User Preferences

**REQ-API-136** — The API SHALL expose the following user preference endpoints:

| # | Method | Path | Description | Auth Required | Required Permission |
|---|--------|------|-------------|---------------|-------------------|
| 1 | `GET` | `/api/v1/users/me/preferences` | Get current user's preferences | Yes | Any authenticated user |
| 2 | `PUT` | `/api/v1/users/me/preferences` | Update current user's preferences | Yes | Any authenticated user |

**REQ-API-137** — User preferences include: `timezone`, `date_format` (ISO, US, EU), `time_format` (12h, 24h), `table_page_size` (10, 25, 50, 100), `dashboard_default_view`, and `notification_preferences` (per-category in-app/email toggles). Preferences are stored server-side and applied by the frontend on load.

### 18A.9 Domain Extensions

**REQ-API-138** — The API SHALL expose the following domain extension endpoints:

| # | Method | Path | Description | Auth Required | Required Permission |
|---|--------|------|-------------|---------------|-------------------|
| 1 | `POST` | `/api/v1/domains/{id}/check-categorization` | Trigger categorization check across security vendors | Yes | `domains.read` |
| 2 | `GET` | `/api/v1/domains/{id}/categorizations` | Get categorization results history | Yes | `domains.read` |
| 3 | `POST` | `/api/v1/domains/generate-typosquats` | Generate typosquat domain suggestions from a seed domain | Yes | `domains.create` |

**REQ-API-139** — The categorization check endpoint returns `202 Accepted` and asynchronously queries security vendor categorization services. Results include vendor name, detected category, and check timestamp. History is retained for trend analysis.

**REQ-API-140** — The typosquat generator accepts a `seed_domain` and returns a list of candidate typosquat domains with the technique used (homoglyph, transposition, insertion, omission, hyphenation, TLD swap). Results include optional availability check via the domain provider API.

### 18A.10 Phishing Report Webhooks

**REQ-API-141** — The API SHALL expose the following phishing report endpoints:

| # | Method | Path | Description | Auth Required | Required Permission |
|---|--------|------|-------------|---------------|-------------------|
| 1 | `POST` | `/api/v1/webhooks/phishing-reports` | Receive phishing report notifications from email gateways | Configurable | Configurable auth (API key, HMAC, none) |
| 2 | `GET` | `/api/v1/campaigns/{id}/phishing-reports` | List phishing reports for a campaign | Yes | `campaigns.read` |
| 3 | `POST` | `/api/v1/campaigns/{id}/phishing-reports` | Manually flag a phishing report for a campaign | Yes | `campaigns.update` |

**REQ-API-142** — The phishing report webhook accepts POST requests from email security gateways when a target reports a phishing email. The webhook matches reports to campaigns via the `Message-ID` header. Authentication is configurable per webhook instance (API key, HMAC signature, or none). Reports are displayed in campaign metrics and the Defender Dashboard.

### 18A.11 Configuration Export/Import

**REQ-API-143** — The API SHALL expose the following configuration export/import endpoints:

| # | Method | Path | Description | Auth Required | Required Permission |
|---|--------|------|-------------|---------------|-------------------|
| 1 | `POST` | `/api/v1/settings/export` | Export framework configuration as ZIP | Yes | `settings.export` |
| 2 | `POST` | `/api/v1/settings/import/preview` | Preview an import (dry run) | Yes | `settings.import` |
| 3 | `POST` | `/api/v1/settings/import` | Import framework configuration from ZIP | Yes | `settings.import` |

**REQ-API-144** — The export endpoint generates a ZIP archive containing JSON files for all exportable configuration categories (see 01-system-overview.md Section 9.1). All credentials, secrets, and PII are excluded. The response streams the ZIP file directly.

**REQ-API-145** — The import preview endpoint accepts a ZIP archive and returns a detailed preview of what would change: entities to create, entities that conflict with existing data (with conflict resolution options: skip, overwrite, create_as_new), and validation errors. No changes are applied.

**REQ-API-146** — The import endpoint accepts the ZIP archive plus conflict resolution selections and applies the import. The request body includes `categories` (which categories to import) and `conflict_resolution` (per-entity decisions). Import requires Administrator role and is fully audit-logged.

### 18A.12 Data Retention

**REQ-API-147** — The API SHALL expose the following data retention endpoints:

| # | Method | Path | Description | Auth Required | Required Permission |
|---|--------|------|-------------|---------------|-------------------|
| 1 | `GET` | `/api/v1/settings/retention` | Get global retention policy settings | Yes | `settings.read` |
| 2 | `PUT` | `/api/v1/settings/retention` | Update global retention policy settings | Yes | `settings.update` |
| 3 | `GET` | `/api/v1/campaigns/{id}/retention` | Get per-campaign retention overrides | Yes | `campaigns.read` |
| 4 | `PUT` | `/api/v1/campaigns/{id}/retention` | Set per-campaign retention overrides | Yes | `campaigns.update` |
| 5 | `GET` | `/api/v1/settings/retention/preview` | Preview retention policy impact (projected storage savings) | Yes | `settings.read` |

**REQ-API-148** — Retention settings cover 7 data categories: campaign data, captured credentials, audit logs, target interaction data, notifications, report files, and landing page builds. Each category has a `retention_days` value (null = indefinite) with documented minimum values. Per-campaign settings override global defaults.

**REQ-API-149** — The preview endpoint calculates the projected impact of current retention policies: how many records would be affected, estimated storage to be reclaimed, and next scheduled enforcement run time.

### 18A.13 Defender Dashboard

**REQ-API-150** — The API SHALL expose the following Defender Dashboard endpoints:

| # | Method | Path | Description | Auth Required | Required Permission |
|---|--------|------|-------------|---------------|-------------------|
| 1 | `GET` | `/api/v1/dashboard/defender` | Get Defender Dashboard summary metrics | Yes | `metrics.read` |
| 2 | `GET` | `/api/v1/dashboard/defender/trends` | Get organizational susceptibility trends over time | Yes | `metrics.read` |
| 3 | `GET` | `/api/v1/dashboard/defender/departments` | Get department risk heatmap data | Yes | `metrics.read` |
| 4 | `GET` | `/api/v1/dashboard/defender/campaign-comparison` | Get side-by-side campaign comparison data | Yes | `metrics.read` |

**REQ-API-151** — The Defender Dashboard endpoints provide aggregate organizational security metrics. The summary endpoint returns: overall susceptibility score, total campaigns, total targets, phishing report rate, and aggregate interaction rates. No individual target data is exposed through these endpoints.

**REQ-API-152** — The trends endpoint accepts `interval` (week, month, quarter) and date range parameters and returns time-series data for susceptibility score, click rate, submission rate, and report rate.

**REQ-API-153** — The department endpoint returns per-department metrics: target count, average click rate, average submission rate, report rate, and a risk score. The campaign comparison endpoint accepts an array of campaign IDs and returns normalized metrics for side-by-side comparison.

---

## 19. WebSocket API

### 19.1 WebSocket Endpoints

**REQ-API-082** — The API SHALL expose the following WebSocket endpoints:

| # | Path | Description | Required Permission |
|---|------|-------------|-------------------|
| 1 | `/ws/dashboard` | Real-time dashboard metrics updates (all active campaigns) | `metrics.read` |
| 2 | `/ws/logs` | Real-time log streaming with server-side filtering | `logs.read` |
| 3 | `/ws/campaign/{id}` | Campaign-specific event stream (status changes, build progress, delivery updates, interactions) | `campaigns.read` |
| 4 | `/ws/notifications` | Real-time notification delivery for the connected user | Any authenticated user |

### 19.2 Connection Handshake

**REQ-API-083** — WebSocket connections SHALL authenticate using the JWT access token passed as a query parameter during the connection handshake: `ws://host/ws/dashboard?token=<jwt>`. The server SHALL validate the token before upgrading the connection. If the token is invalid or expired, the server SHALL reject the upgrade with HTTP `401`.

**REQ-API-084** — Upon successful connection, the server SHALL send an initial state message containing the current snapshot of relevant data (e.g., current dashboard metrics, current campaign status) so the client does not need to make a separate REST call to hydrate its state.

### 19.3 Message Format

**REQ-API-085** — All WebSocket messages SHALL use the following JSON envelope:

```json
{
  "type": "event_type",
  "timestamp": "2026-02-25T12:00:00Z",
  "data": { ... }
}
```

**REQ-API-086** — The following event types SHALL be supported on the dashboard WebSocket (`/ws/dashboard`):

| Event Type | Description |
|-----------|-------------|
| `dashboard.snapshot` | Full dashboard state (sent on connection) |
| `campaign.metrics_updated` | Updated metrics for a specific campaign |
| `campaign.status_changed` | Campaign transitioned to a new state |
| `infrastructure.health_updated` | Instance health status changed |
| `infrastructure.status_changed` | Instance lifecycle state changed |
| `notification.new` | New system notification (blocklist alert, health check failure, etc.) |
| `notification.created` | New notification for the connected user |

**REQ-API-087** — The following event types SHALL be supported on the campaign WebSocket (`/ws/campaign/{id}`):

| Event Type | Description |
|-----------|-------------|
| `campaign.snapshot` | Full campaign state (sent on connection) |
| `campaign.status_changed` | Campaign state transition |
| `campaign.build_progress` | Landing page build progress update |
| `campaign.email_sent` | Individual email delivery result |
| `campaign.email_opened` | Target opened the email |
| `campaign.link_clicked` | Target clicked the phishing link |
| `campaign.credentials_submitted` | Target submitted credentials |
| `campaign.metrics_updated` | Aggregate metrics updated |

**REQ-API-088** — The following event types SHALL be supported on the logs WebSocket (`/ws/logs`):

| Event Type | Description |
|-----------|-------------|
| `log.entry` | New log entry matching the client's filter subscription |
| `log.filter_applied` | Confirmation that the server applied the client's filter request |

**REQ-API-154** — The following event types SHALL be supported on the notifications WebSocket (`/ws/notifications`):

| Event Type | Description |
|-----------|-------------|
| `notification.snapshot` | Unread notification count and recent notifications (sent on connection) |
| `notification.created` | New notification created for this user |
| `notification.read` | Notification marked as read (sync across tabs/devices) |

### 19.4 Client-to-Server Messages

**REQ-API-089** — Clients SHALL be able to send the following messages to WebSocket endpoints:

| Message Type | WebSocket | Description |
|-------------|-----------|-------------|
| `subscribe` | `/ws/logs` | Set or update log filter criteria (level, source, campaign_id, search) |
| `unsubscribe` | `/ws/logs` | Remove log filter (stop receiving log entries) |
| `ping` | All | Client heartbeat |

### 19.5 Heartbeat and Reconnection

**REQ-API-090** — The server SHALL send a `ping` frame to each connected WebSocket client at a configurable interval (default: 30 seconds). If the client does not respond with a `pong` within the configured timeout (default: 10 seconds), the server SHALL close the connection.

**REQ-API-091** — The client-side WebSocket implementation (in the React SPA) SHALL implement automatic reconnection with exponential backoff: initial delay of 1 second, doubling on each attempt, capped at 30 seconds. Upon reconnection, the client SHALL re-authenticate and re-subscribe to its previous channels.

**REQ-API-092** — When a WebSocket connection is re-established, the server SHALL send the latest snapshot for the subscribed resource so the client can reconcile any missed events during the disconnection window.

---

## 20. API Security Requirements

### 20.1 Authentication and Authorization

**REQ-API-093** — All API endpoints SHALL require a valid JWT Bearer token in the `Authorization` header, except for the following explicitly public endpoints:

- `POST /api/v1/auth/login`
- `POST /api/v1/auth/setup`
- `POST /api/v1/auth/refresh`
- `GET /api/v1/auth/providers`
- `POST /api/v1/auth/oidc/callback`
- `POST /api/v1/auth/ldap/login`
- `GET /api/v1/health`

**REQ-API-094** — RBAC middleware SHALL intercept every authenticated request and verify the requesting user has the required permission for the target endpoint. Insufficient permissions SHALL result in a `403 FORBIDDEN` response. The middleware SHALL NOT reveal which permission was required (to prevent permission enumeration).

**REQ-API-095** — The JWT payload SHALL include at minimum: `sub` (user ID), `iat` (issued at), `exp` (expiration), `roles` (list of role IDs), and `jti` (unique token identifier for revocation support).

**REQ-API-096** — The API SHALL support token revocation. Logging out SHALL invalidate the user's refresh token. A compromised token SHALL be revocable by an Admin through the user management API without requiring the user's participation.

### 20.2 Input Validation

**REQ-API-097** — All request bodies SHALL be validated against a schema before processing. The API SHALL reject requests with unknown fields (strict mode) to prevent mass-assignment vulnerabilities.

**REQ-API-098** — All string inputs SHALL be validated for maximum length. Path parameters and query parameters SHALL be validated for correct type and format (e.g., UUID format for ID parameters).

**REQ-API-099** — All user-supplied strings that will be rendered in HTML contexts (e.g., template content, campaign names in reports) SHALL be sanitized to prevent stored XSS. The API SHALL encode HTML special characters on output.

### 20.3 SQL Injection Prevention

**REQ-API-100** — All database queries SHALL use parameterized queries or a query builder that enforces parameterization. Raw string concatenation in SQL queries is prohibited.

**REQ-API-101** — Sort and filter field names received from clients SHALL be validated against an allowlist of permitted column names. Dynamic column names SHALL never be interpolated directly into SQL.

### 20.4 CSRF Protection

**REQ-API-102** — The API SHALL implement CSRF protection for all state-changing operations. Since the API uses JWT Bearer tokens (not cookies) for authentication, the primary CSRF defense is the `Authorization` header requirement (which cannot be set by cross-origin form submissions). Additionally, the API SHALL:

- Set the `SameSite=Strict` attribute on any cookies used
- Validate the `Origin` header on all state-changing requests
- Reject requests from unexpected origins

### 20.5 CORS Configuration

**REQ-API-103** — The API SHALL implement strict CORS headers:

| Header | Value |
|--------|-------|
| `Access-Control-Allow-Origin` | Configured origin of the React SPA (not `*`) |
| `Access-Control-Allow-Methods` | `GET, POST, PUT, PATCH, DELETE, OPTIONS` |
| `Access-Control-Allow-Headers` | `Authorization, Content-Type, X-Correlation-ID` |
| `Access-Control-Expose-Headers` | `X-Correlation-ID, X-RateLimit-Limit, X-RateLimit-Remaining, X-RateLimit-Reset` |
| `Access-Control-Max-Age` | `86400` (24 hours) |

### 20.6 Audit Trail

**REQ-API-104** — Every state-changing API request (POST, PUT, PATCH, DELETE) SHALL generate an audit log entry containing: correlation ID, user ID, timestamp, endpoint path, HTTP method, request summary (excluding sensitive fields), response status code, and IP address.

**REQ-API-105** — Audit log entries for sensitive operations (credential access, password changes, role modifications, infrastructure lifecycle actions) SHALL include additional detail: the resource type, resource ID, and a before/after diff of changed fields (with sensitive values masked).

### 20.7 Request Logging

**REQ-API-106** — All API requests SHALL be logged with: correlation ID, timestamp, HTTP method, path, response status, response time in milliseconds, user ID (if authenticated), and client IP address. Request and response bodies SHALL NOT be logged by default (to prevent credential leakage); body logging SHALL be available as a configurable debug option.

---

## 21. API Versioning

**REQ-API-107** — All API routes SHALL be prefixed with a version identifier: `/api/v1/`. The version prefix is part of the URL path, not a header.

**REQ-API-108** — When a breaking change is introduced, a new version SHALL be created (e.g., `/api/v2/`). The previous version SHALL continue to function for a documented deprecation period (minimum 6 months). Non-breaking changes (new optional fields, new endpoints) SHALL be added to the current version without incrementing.

**REQ-API-109** — Deprecated endpoints SHALL return a `Deprecation` header with the date the endpoint will be removed and a `Link` header pointing to the replacement endpoint.

---

## 22. Go Implementation Guidelines

**REQ-API-110** — All API handler code SHALL use local import paths (e.g., `tackle/internal/api/v1/handlers`, `tackle/internal/middleware`). No import paths SHALL reference `github.com`.

**REQ-API-111** — The API layer SHALL be structured as follows:

```
tackle/
  internal/
    api/
      v1/
        handlers/       # HTTP handler functions (one file per resource)
        middleware/      # Auth, RBAC, rate limiting, CORS, logging, correlation ID
        routes.go       # Route registration
        errors.go       # Error response helpers
    models/             # Request/response structs, validation
    service/            # Business logic layer (handlers delegate to services)
    repository/         # Database access layer (parameterized queries)
    websocket/          # WebSocket hub, client management, event broadcasting
```

**REQ-API-112** — Each handler function SHALL: (1) parse and validate the request; (2) delegate to a service function for business logic; (3) return the response using the standard envelope format. Handlers SHALL NOT contain business logic or direct database access.

**REQ-API-113** — All request structs SHALL use struct tags for validation (e.g., `validate:"required,email"`) and be validated in the handler before passing to the service layer.

---

## 23. Security Considerations

**SEC-API-001** — JWT signing keys SHALL be cryptographically strong (minimum 256-bit for HMAC, 2048-bit for RSA). Keys SHALL be configurable via environment variables and SHALL NOT be hardcoded or stored in the database.

**SEC-API-002** — All API responses SHALL include security headers:

| Header | Value |
|--------|-------|
| `X-Content-Type-Options` | `nosniff` |
| `X-Frame-Options` | `DENY` |
| `X-XSS-Protection` | `0` (disabled in favor of CSP) |
| `Content-Security-Policy` | Strict policy appropriate for API responses |
| `Strict-Transport-Security` | `max-age=31536000; includeSubDomains` |
| `Cache-Control` | `no-store` (for authenticated responses) |
| `Pragma` | `no-cache` (for authenticated responses) |

**SEC-API-003** — Failed authentication attempts SHALL be logged with the attempted username, client IP, and timestamp. After a configurable number of failed attempts (default: 5) within a time window (default: 15 minutes), the API SHALL temporarily lock the account and return `429 RATE_LIMITED`.

**SEC-API-004** — API responses SHALL NOT include sensitive data beyond what is explicitly needed. Specifically:
- Passwords SHALL never be returned
- API keys and secrets SHALL be masked
- Internal IDs or paths that reveal system architecture SHALL not be included in error messages
- Stack traces SHALL never be exposed to clients

**SEC-API-005** — File upload endpoints SHALL validate file content (not just the extension or MIME type header) to prevent upload of malicious files. Uploaded files SHALL be stored outside the web root and served through a controlled endpoint.

**SEC-API-006** — The API SHALL implement request timeout enforcement. Long-running requests SHALL be terminated after a configurable timeout (default: 30 seconds for synchronous requests). Asynchronous operations SHALL use the `202 Accepted` pattern with background processing.

---

## 24. Acceptance Criteria

### General API Behavior
- [ ] All endpoints return responses matching the standard envelope format (REQ-API-004, REQ-API-005, REQ-API-006)
- [ ] Every response includes a `X-Correlation-ID` header that matches the correlation ID in corresponding log entries
- [ ] Pagination works correctly with cursor-based navigation; forward and backward traversal returns consistent results
- [ ] Filtering and sorting produce correct results for all documented filter parameters
- [ ] Rate limiting returns `429` responses with correct `Retry-After` headers when limits are exceeded
- [ ] Request size limits are enforced; oversized requests return `413`

### Authentication
- [ ] Login with valid credentials returns JWT access and refresh tokens
- [ ] Login with invalid credentials returns `401` and does not reveal whether the username or password was incorrect
- [ ] Expired access tokens return `401 TOKEN_EXPIRED`; the client can use the refresh token to obtain a new access token
- [ ] The setup endpoint creates the initial admin and is disabled for all subsequent calls
- [ ] Logout invalidates the refresh token; subsequent refresh attempts fail

### Authorization
- [ ] Every authenticated endpoint enforces RBAC; requests without the required permission receive `403`
- [ ] Permission changes take effect immediately on the next request (no caching of stale permissions)
- [ ] The permission check does not reveal which permission was required in the error response

### WebSocket
- [ ] WebSocket connections require a valid JWT token; connections with invalid tokens are rejected with `401`
- [ ] The dashboard WebSocket delivers real-time metrics updates within 2 seconds of the underlying event
- [ ] The logs WebSocket streams log entries matching the client's subscribed filter
- [ ] The campaign WebSocket delivers all campaign-specific events (status changes, email events, interactions)
- [ ] Heartbeat/keepalive correctly detects and closes stale connections
- [ ] Reconnected clients receive a state snapshot to reconcile missed events

### Security
- [ ] All database queries use parameterized queries; no SQL injection is possible via any API parameter
- [ ] HTML special characters in user input are properly encoded on output; no stored XSS is possible
- [ ] CORS headers restrict access to the configured SPA origin only
- [ ] CSRF attacks via cross-origin form submissions are blocked
- [ ] Sensitive data (passwords, API keys, secrets) never appears in API responses (except masked), logs, or error messages
- [ ] Failed login attempts trigger account lockout after the configured threshold
- [ ] All state-changing operations produce audit log entries with correlation IDs

### Resource-Specific
- [ ] Campaign state transitions enforce the state machine; invalid transitions return `409`
- [ ] Campaign launch pre-flight validation checks all dependencies and returns actionable errors for failures
- [ ] CSV target import processes asynchronously and reports progress, counts, and per-row errors
- [ ] Infrastructure provisioning returns `202` and provisions asynchronously; status is available via polling and WebSocket
- [ ] Instance termination requires confirmation matching the instance ID
- [ ] Credential data is access-controlled; only users with `credentials.read` can view captured credentials
- [ ] Report generation is asynchronous; generated reports are downloadable via a stable URL

### Extended Features
- [ ] Campaign cloning creates a new draft campaign with selected components
- [ ] Campaign templates can be created, listed, and applied to create new campaigns
- [ ] Dry run simulation completes without sending real emails or provisioning infrastructure
- [ ] Calendar view returns campaigns with correct date ranges and status
- [ ] Canary targets are sent emails first and excluded from campaign metrics
- [ ] Landing page HTML import parses content into builder-compatible format
- [ ] Landing page URL cloning downloads and localizes all assets
- [ ] Email template library returns built-in templates filterable by category and difficulty
- [ ] Session capture data is access-controlled at the same level as credentials
- [ ] Notifications are delivered in real-time via WebSocket and persist in the inbox
- [ ] Webhook endpoints can be configured, tested, and receive deliveries with retry logic
- [ ] Universal tags can be applied to and searched across all primary entity types
- [ ] Alert rules trigger notifications when audit log conditions are met
- [ ] User preferences are stored server-side and returned on authentication
- [ ] Domain categorization checks return vendor classifications
- [ ] Typosquat generator produces candidate domains with technique labels
- [ ] Phishing report webhooks match reports to campaigns via Message-ID
- [ ] Configuration export produces a credential-free ZIP archive
- [ ] Configuration import preview shows accurate change predictions
- [ ] Data retention policies are enforceable at global and per-campaign levels
- [ ] Defender Dashboard returns aggregate metrics without exposing individual target data

---

## 25. Dependencies

| Dependency | Document | Relationship |
|------------|----------|-------------|
| **System Overview** | [01-system-overview.md](01-system-overview.md) | Defines the technology stack (Go, React, PostgreSQL) and architectural principles that shape the API design |
| **Authentication & Authorization** | [02-authentication-authorization.md](02-authentication-authorization.md) | Defines the authentication providers, RBAC model, roles, and permissions enforced by the API middleware |
| **Domain Management** | [03-domain-infrastructure.md](03-domain-infrastructure.md) | Domain and infrastructure resource models served by the API |
| **Email & SMTP** | [04-email-smtp.md](04-email-smtp.md) | SMTP profile and email template resource models served by the API |
| **Landing Page Builder** | [05-landing-page-builder.md](05-landing-page-builder.md) | Landing page resource model and build/deploy lifecycle exposed via the API |
| **Campaign Management** | [06-campaign-management.md](06-campaign-management.md) | Campaign state machine and lifecycle operations exposed via the API |
| **Phishing Endpoints** | [07-phishing-endpoints.md](07-phishing-endpoints.md) | Endpoint health and lifecycle management exposed via the infrastructure API |
| **Credential Capture** | [08-credential-capture.md](08-credential-capture.md) | Credential data model and access control requirements served by the API |
| **Target Management** | [09-target-management.md](09-target-management.md) | Target and target group resource models served by the API |
| **Metrics & Reporting** | [10-metrics-reporting.md](10-metrics-reporting.md) | Metrics aggregation and report generation triggered via the API |
| **Audit Logging** | [11-audit-logging.md](11-audit-logging.md) | Audit log entries generated by every state-changing API operation |
| **Database Schema** | [14-database-schema.md](14-database-schema.md) | Database tables and relationships that back all API resources |
| **Frontend Architecture** | [16-frontend-architecture.md](16-frontend-architecture.md) | The React SPA that consumes this API |
| **Notification System** | [18-notification-system.md](18-notification-system.md) | Notification delivery channels, user preferences, and webhook configuration served by the API |
