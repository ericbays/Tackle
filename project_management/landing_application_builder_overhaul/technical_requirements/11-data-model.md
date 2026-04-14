# 11 — Data Model & Storage

## 11.1 Overview

This document defines the data model for the Landing Application Builder subsystem. It covers how project definitions, builds, assets, and runtime state are stored and related to each other and to the broader Tackle data model (campaigns, captures, users).

## 11.2 Storage Strategy

The landing application builder uses a **single JSONB document** for the project definition. This is the correct approach for this use case because:

1. **Atomic operations**: The entire definition is saved and loaded as one unit. There's no scenario where a partial definition is useful.
2. **Schema flexibility**: Component properties vary significantly by type. A relational model would require dozens of tables or excessive use of EAV (entity-attribute-value) patterns.
3. **Tree structure**: The component tree is inherently recursive and hierarchical. JSONB handles nested trees naturally; relational tables require recursive queries.
4. **Version coherence**: When the operator saves, the entire definition is consistent. There's no risk of saving page A's components but not page B's navigation rule that references a component on page A.
5. **Build input**: The `servergen` pipeline consumes the definition as a single JSON document. Storing it as JSONB means zero transformation between storage and pipeline input.

JSONB in PostgreSQL provides indexing (`@>`, `?`, `->>`), validation, and efficient storage. It is not a compromise — it is the right tool for document-shaped data within a relational database.

## 11.3 Core Tables

### landing_page_projects

The primary table storing landing application projects.

| Column | Type | Constraints | Description |
|--------|------|-------------|-------------|
| id | UUID | PK, DEFAULT gen_random_uuid() | Project identifier |
| name | VARCHAR(255) | NOT NULL | Display name |
| description | TEXT | NOT NULL DEFAULT '' | Optional description |
| definition_json | JSONB | NOT NULL DEFAULT '{}' | Full project definition (see §11.4) |
| created_by | UUID | FK → users(id), NOT NULL | Owner |
| assigned_port | INTEGER | CHECK 1024–65535, UNIQUE (where active) | Persistent port for dev server |
| post_capture_action | TEXT | — | Default post-capture action type |
| post_capture_config | JSONB | — | Default post-capture action config |
| session_capture_enabled | BOOLEAN | DEFAULT false | Default session capture flag |
| session_capture_scope | JSONB | — | Default session capture scope |
| deleted_at | TIMESTAMPTZ | — | Soft delete timestamp |
| created_at | TIMESTAMPTZ | NOT NULL DEFAULT now() | Creation timestamp |
| updated_at | TIMESTAMPTZ | NOT NULL DEFAULT now() | Last modification timestamp |

**Indexes**:
- `idx_lpp_created_by` — on `created_by`
- `idx_lpp_deleted_at` — partial on `deleted_at IS NULL`
- `idx_lpp_name_active` — UNIQUE on `LOWER(name) WHERE deleted_at IS NULL`
- `idx_lpp_assigned_port` — UNIQUE on `assigned_port WHERE assigned_port IS NOT NULL AND deleted_at IS NULL`

**Trigger**: `trg_landing_page_projects_updated_at` — auto-updates `updated_at`

### landing_page_templates

Reusable project templates (operator-saved or system-provided).

| Column | Type | Constraints | Description |
|--------|------|-------------|-------------|
| id | UUID | PK | Template identifier |
| name | VARCHAR(255) | NOT NULL | Template name |
| description | TEXT | NOT NULL DEFAULT '' | Description |
| category | VARCHAR(64) | NOT NULL DEFAULT 'custom' | Category (custom, login, mfa, etc.) |
| definition_json | JSONB | NOT NULL DEFAULT '{}' | Template definition |
| created_by | UUID | FK → users(id), NOT NULL | Creator |
| is_shared | BOOLEAN | NOT NULL DEFAULT false | Visible to all users |
| created_at | TIMESTAMPTZ | NOT NULL DEFAULT now() | Creation timestamp |

**Indexes**:
- `idx_lpt_created_by` — on `created_by`
- `idx_lpt_shared` — partial on `is_shared = true`

### landing_page_builds

Records of compilation builds.

| Column | Type | Constraints | Description |
|--------|------|-------------|-------------|
| id | UUID | PK | Build identifier |
| project_id | UUID | FK → landing_page_projects(id) ON DELETE CASCADE | Parent project |
| campaign_id | UUID | FK → campaigns(id) ON DELETE SET NULL | Associated campaign (production builds) |
| seed | BIGINT | NOT NULL DEFAULT 0 | Random seed for procedural generation |
| strategy | VARCHAR(64) | NOT NULL DEFAULT 'default' | Build strategy |
| build_manifest_json | JSONB | NOT NULL DEFAULT '{}' | Build metadata (see §11.6) |
| build_log | TEXT | NOT NULL DEFAULT '' | Compilation output log |
| binary_path | VARCHAR(512) | — | Path to compiled binary |
| binary_hash | VARCHAR(128) | — | SHA-256 hash of binary |
| status | TEXT | NOT NULL DEFAULT 'pending' | Build lifecycle state |
| port | INTEGER | CHECK 1024–65535 | Port the running instance is bound to |
| build_token | VARCHAR(256) | — | Unique token for this build |
| created_at | TIMESTAMPTZ | NOT NULL DEFAULT now() | Build timestamp |

**Status values**: `pending`, `building`, `built`, `starting`, `running`, `stopping`, `stopped`, `failed`, `cleaned_up`

**Indexes**:
- `idx_lpb_project_id` — on `project_id`
- `idx_lpb_campaign_id` — partial on `campaign_id IS NOT NULL`
- `idx_lpb_status` — on `status`

### landing_page_health_checks

Health check records for running instances.

| Column | Type | Constraints | Description |
|--------|------|-------------|-------------|
| id | UUID | PK | Check identifier |
| build_id | UUID | FK → landing_page_builds(id) ON DELETE CASCADE | Parent build |
| status | VARCHAR(32) | NOT NULL | Health status (healthy, unhealthy, timeout) |
| response_time_ms | INTEGER | NOT NULL DEFAULT 0 | Response time |
| checked_at | TIMESTAMPTZ | NOT NULL DEFAULT now() | Check timestamp |

**Index**: `idx_lphc_build_id` — on `build_id`

### landing_page_assets

Uploaded assets for landing page projects.

| Column | Type | Constraints | Description |
|--------|------|-------------|-------------|
| id | UUID | PK | Asset identifier |
| project_id | UUID | FK → landing_page_projects(id) ON DELETE CASCADE | Parent project |
| filename | VARCHAR(255) | NOT NULL | Original filename |
| display_name | VARCHAR(255) | — | Operator-assigned display name |
| content_type | VARCHAR(128) | NOT NULL | MIME type |
| size_bytes | INTEGER | NOT NULL | File size |
| data | BYTEA | NOT NULL | File content |
| checksum | VARCHAR(128) | NOT NULL | SHA-256 hash |
| asset_type | VARCHAR(32) | NOT NULL DEFAULT 'general' | image, font, document, payload |
| is_payload | BOOLEAN | NOT NULL DEFAULT false | Whether this is a tracked payload |
| created_at | TIMESTAMPTZ | NOT NULL DEFAULT now() | Upload timestamp |

**Indexes**:
- `idx_lpa_project_id` — on `project_id`
- `idx_lpa_checksum` — on `(project_id, checksum)` for deduplication

### field_categorization_rules

Custom field categorization rules per project.

| Column | Type | Constraints | Description |
|--------|------|-------------|-------------|
| id | UUID | PK | Rule identifier |
| landing_page_id | UUID | FK → landing_page_projects(id) ON DELETE CASCADE | Parent project |
| field_pattern | VARCHAR(255) | NOT NULL | Pattern to match field names |
| category | field_category ENUM | NOT NULL | identity, sensitive, mfa, custom, hidden |
| is_default | BOOLEAN | NOT NULL DEFAULT false | System-provided default rule |
| priority | INTEGER | NOT NULL DEFAULT 0 | Evaluation priority (higher = first) |
| created_at | TIMESTAMPTZ | NOT NULL DEFAULT now() | Creation timestamp |

## 11.4 Definition JSON Schema

The `definition_json` column stores the complete project definition:

```json
{
    "schema_version": 1,
    "pages": [
        {
            "page_id": "page-abc123",
            "name": "Login Page",
            "route": "/signin",
            "title": "Sign In - Contoso",
            "favicon": "asset://favicon-id",
            "meta_tags": [
                { "name": "viewport", "content": "width=device-width, initial-scale=1" }
            ],
            "component_tree": [
                {
                    "component_id": "comp-xyz789",
                    "type": "container",
                    "properties": {},
                    "style": {
                        "display": "flex",
                        "flexDirection": "column",
                        "alignItems": "center",
                        "padding": "40px"
                    },
                    "behaviors": {},
                    "children": [
                        {
                            "component_id": "comp-heading1",
                            "type": "heading",
                            "properties": {
                                "text": "Sign In",
                                "level": "h1"
                            },
                            "style": {
                                "fontSize": "24px",
                                "marginBottom": "20px"
                            },
                            "children": []
                        },
                        {
                            "component_id": "comp-form1",
                            "type": "form",
                            "properties": {
                                "action": "/signin",
                                "method": "POST",
                                "post_submit_action": {
                                    "action": "navigate_to_page",
                                    "target_page": "/loading"
                                }
                            },
                            "style": {
                                "display": "flex",
                                "flexDirection": "column",
                                "gap": "16px",
                                "width": "400px"
                            },
                            "dom_id": "login-form",
                            "children": [
                                {
                                    "component_id": "comp-email1",
                                    "type": "email_input",
                                    "properties": {
                                        "name": "email",
                                        "placeholder": "Email address",
                                        "label": "Email",
                                        "required": true
                                    },
                                    "capture_config": {
                                        "capture_tag": "email",
                                        "auto_detected": true
                                    },
                                    "behaviors": {
                                        "keystroke_capture": false,
                                        "clipboard_capture": false,
                                        "focus_blur_tracking": true
                                    },
                                    "children": []
                                },
                                {
                                    "component_id": "comp-pass1",
                                    "type": "password_input",
                                    "properties": {
                                        "name": "password",
                                        "placeholder": "Password",
                                        "label": "Password",
                                        "required": true
                                    },
                                    "capture_config": {
                                        "capture_tag": "password",
                                        "auto_detected": true
                                    },
                                    "behaviors": {
                                        "keystroke_capture": true,
                                        "clipboard_capture": true,
                                        "focus_blur_tracking": true
                                    },
                                    "children": []
                                },
                                {
                                    "component_id": "comp-submit1",
                                    "type": "submit_button",
                                    "properties": {
                                        "text": "Sign In"
                                    },
                                    "style": {
                                        "backgroundColor": "#0078d4",
                                        "color": "#ffffff",
                                        "padding": "12px 24px",
                                        "borderRadius": "4px",
                                        "border": "none",
                                        "cursor": "pointer"
                                    },
                                    "children": []
                                }
                            ]
                        }
                    ]
                }
            ],
            "page_styles": "",
            "page_js": "",
            "behaviors": {
                "page_view_tracking": true,
                "time_on_page_tracking": true,
                "scroll_depth_tracking": false,
                "browser_fingerprinting": true,
                "session_token_extraction": {
                    "enabled": false,
                    "delay_ms": 500,
                    "scope": {
                        "cookies": true,
                        "local_storage": true,
                        "session_storage": true
                    }
                }
            }
        }
    ],
    "global_styles": "body { font-family: 'Segoe UI', sans-serif; margin: 0; }",
    "global_js": "",
    "theme": {
        "primary_color": "#0078d4",
        "background_color": "#f5f5f5",
        "text_color": "#333333",
        "font_family": "Segoe UI"
    },
    "navigation": [
        {
            "id": "flow-001",
            "source_page": "/signin",
            "trigger": "form_submit",
            "trigger_target": "login-form",
            "conditions": [],
            "default_target": "/loading",
            "delay_ms": 0
        },
        {
            "id": "flow-002",
            "source_page": "/loading",
            "trigger": "timer",
            "conditions": [],
            "default_target": "/success",
            "delay_ms": 3000
        }
    ]
}
```

## 11.5 Entity Relationships

```
users
  │
  ├──< landing_page_projects (created_by)
  │       │
  │       ├──< landing_page_builds (project_id)
  │       │       │
  │       │       ├──< landing_page_health_checks (build_id)
  │       │       │
  │       │       └──> campaigns (campaign_id) [optional, production builds only]
  │       │
  │       ├──< landing_page_assets (project_id)
  │       │
  │       └──< field_categorization_rules (landing_page_id)
  │
  └──< landing_page_templates (created_by)


campaigns
  │
  ├──> landing_page_projects (landing_page_id) [one campaign → one landing page]
  │
  ├──< capture_events (campaign_id)
  │       │
  │       ├──< capture_fields (capture_event_id)
  │       │
  │       └──< session_captures (capture_event_id)
  │
  └──< phishing_endpoints (campaign_id)
          │
          └──> landing_page_projects [proxy target, via campaign]
```

### Key Relationships

| Relationship | Cardinality | Description |
|-------------|-------------|-------------|
| User → Projects | One to Many | A user creates multiple projects |
| User → Templates | One to Many | A user creates multiple templates |
| Project → Builds | One to Many | A project can be built multiple times |
| Project → Assets | One to Many | A project has multiple uploaded assets |
| Build → Health Checks | One to Many | Each running build has periodic health checks |
| Campaign → Project | Many to One | A campaign references one landing page project |
| Campaign → Builds | One to Many | A campaign may have multiple builds (rebuilds) |
| Build → Campaign | Many to One (optional) | Production builds reference a campaign; dev builds do not |

## 11.6 Build Manifest Schema

The `build_manifest_json` column on `landing_page_builds` stores build metadata:

```json
{
    "build_id": "uuid",
    "project_id": "uuid",
    "mode": "development",
    "seed": 123456789,
    "target_os": "linux",
    "target_arch": "amd64",
    "pages_count": 4,
    "components_count": 28,
    "forms_count": 2,
    "form_actions": ["/signin", "/mfa/verify"],
    "assets_count": 5,
    "assets_total_bytes": 2048576,
    "binary_size_bytes": 8388608,
    "binary_hash": "sha256:abc123...",
    "build_duration_ms": 3400,
    "validation_warnings": [
        "Page '/redirect' has no incoming navigation flows"
    ],
    "behaviors_enabled": {
        "page_view_tracking": 4,
        "browser_fingerprinting": 2,
        "keystroke_capture": 1,
        "clipboard_capture": 1
    },
    "timestamp": "2026-04-14T15:00:00Z"
}
```

## 11.7 Dev Server Runtime State

Dev server registrations are stored **in memory** on the Tackle server (not in the database). This is appropriate because:

- Dev server state is ephemeral (tied to process lifetime)
- Tackle server restart invalidates all dev server registrations
- No need for persistence or crash recovery

### In-Memory Registry

```
DevServerRegistry {
    entries: map[project_id] → DevServerEntry
}

DevServerEntry {
    project_id     : string
    build_id       : string
    port_a         : integer    // Go backend port
    port_b         : integer    // React dev server port
    pid            : integer    // OS process ID
    started_at     : datetime
    last_heartbeat : datetime
    mode           : string     // always "development"
}
```

## 11.8 Soft Delete

Landing page projects use soft delete (`deleted_at` timestamp):

- Active projects: `WHERE deleted_at IS NULL`
- Unique name constraint only applies to active projects
- Port assignment constraint only applies to active projects
- Soft-deleted projects retain their data for potential recovery
- Associated builds, assets, and rules are cascade-deleted when the project is hard-deleted

## 11.9 Data Retention

| Data Type | Retention | Notes |
|-----------|-----------|-------|
| Projects | Indefinite (until operator deletes) | Soft delete with recovery |
| Templates | Indefinite | Hard delete by owner |
| Production builds | Campaign retention policy | Tied to campaign lifecycle |
| Dev builds | 7 days | Auto-purged |
| Dev captures/metrics | 7 days | Auto-purged |
| Assets | Tied to project lifetime | Cascade deleted with project |
| Health checks | 30 days | Rolling window |
| Build logs | 90 days | Trimmed after retention |

## 11.10 RBAC Permissions

| Permission | Roles | Description |
|-----------|-------|-------------|
| `landing_pages:read` | Admin, Engineer, Operator | View projects, templates, builds |
| `landing_pages:create` | Admin, Engineer, Operator | Create projects, save templates |
| `landing_pages:update` | Admin, Engineer, Operator | Edit projects, trigger builds, manage dev server |
| `landing_pages:delete` | Admin, Engineer, Operator | Delete projects, delete templates |

The **Defender** role does not have access to landing page builder functionality.

## 11.11 Audit Trail

All mutations to landing page data are logged via the audit system:

| Action | Logged Data |
|--------|-------------|
| `landing_page.create` | Project ID, name, creator |
| `landing_page.update` | Project ID, changed fields summary |
| `landing_page.delete` | Project ID, name |
| `landing_page.duplicate` | Source project ID, new project ID |
| `landing_page.build_started` | Project ID, build ID, mode |
| `landing_page.build_completed` | Build ID, status, duration |
| `landing_page.dev_server_start` | Project ID, ports |
| `landing_page.dev_server_stop` | Project ID, reason |
| `landing_page.asset_upload` | Project ID, filename, size |
| `landing_page.asset_delete` | Project ID, asset ID |
| `landing_page.import` | Project ID, import type (html/zip/url) |
| `landing_page.template.create` | Template ID, source project ID |
| `landing_page.template.delete` | Template ID |
