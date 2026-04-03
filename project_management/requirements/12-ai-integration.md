# 12 — AI Integration (Future State)

## 1. Overview

This is a **future-state** requirements document. AI features described here are **not implemented in v1**. The v1 implementation is predominantly manual — operators create campaigns, write templates, select targets, and schedule delivery by hand.

However, the architecture, data models, and interfaces defined in v1 **must** be designed now to accommodate AI integration without architectural changes. This document specifies what must exist in v1 (schema fields, interfaces, queue structures) so that AI capabilities can be layered in progressively during v2 and beyond.

All AI features follow one principle: **AI is advisory, humans decide**. No AI agent can execute a campaign action without explicit operator approval.

### 1.1 Relationship to Other Requirements

| Dependency | Interaction |
|-----------|-------------|
| **Campaign Management (06)** | AI proposals create draft campaigns that enter the standard campaign workflow |
| **Landing Page Builder (05)** | AI content suggestions feed into the existing template/builder system |
| **Target Management (09)** | AI targeting recommendations reference the existing target data model |
| **Metrics & Reporting (10)** | AI training data derives from campaign outcome metrics |
| **Audit Logging (11)** | All AI actions are logged through the standard audit system |
| **Database Schema (14)** | v1 schema must include AI-readiness fields and tables defined in this document |

## 2. AI Capabilities (Planned)

### 2.1 AI-Assisted Research

AI research agents gather intelligence to inform campaign design. These agents connect to local or remote AI APIs and MCP (Model Context Protocol) servers to:

- Research current phishing trends, successful techniques, and industry-specific social engineering approaches
- Gather publicly available intelligence on target organizations (corporate news, press releases, organizational structure, technology stack)
- Identify optimal pretexts based on current events, seasonal patterns, and organizational context
- Summarize defensive posture indicators (published security blog posts, job listings for security roles, conference talks)

The v1 framework must define interfaces and APIs that AI research agents can call when they are implemented in v2.

### 2.2 Automated Campaign Drafting

AI agents generate complete campaign proposals that include:

- Email template drafts (subject lines, body content, sender persona)
- Landing page type and content suggestions
- Target group recommendations with justification
- Optimal send timing based on historical data and organizational patterns
- Infrastructure recommendations (domain similarity, endpoint region)

Proposals queue in a **backlog** for operator review and scheduling. Operators can:

- **Accept** a proposal as-is, promoting it to a draft campaign
- **Modify** a proposal, editing any field before promotion
- **Reject** a proposal, optionally providing feedback that enters the AI feedback loop
- **Archive** a proposal for future reference without acting on it

AI learns from campaign outcomes and operator decisions to improve future proposals through a structured feedback loop.

### 2.3 Content Generation

AI assists operators with content creation tasks:

- **Email template writing** — Generate phishing email body text with persuasion optimization, adjustable for tone, urgency, and pretext type
- **Subject line generation** — Produce multiple subject line variants with predicted open-rate rankings
- **Landing page content** — Suggest page copy, form field labels, error messages, and branding elements that align with the chosen pretext
- **Personalization recommendations** — Suggest merge fields and personalization strategies based on available target data (name, role, department, manager)
- **A/B variant generation** — Produce content variants for split testing with rationale for each variant

## 3. Architecture Requirements for AI Readiness

### 3.1 Plugin/Agent Interface

The framework must define a standard interface that AI agents implement. This interface is the contract between the core platform and any AI agent — whether it is a local LLM, a remote API-backed agent, or an MCP-connected tool.

The agent interface must support:

| Capability | Direction | Description |
|-----------|-----------|-------------|
| **Campaign data read** | Agent reads from framework | Query targets, templates, campaign configurations, historical metrics |
| **Proposal submission** | Agent writes to framework | Submit campaign proposals, content suggestions, research findings |
| **Research storage** | Agent writes to framework | Store research results (OSINT findings, trend analysis, pretext ideas) |
| **Feedback retrieval** | Agent reads from framework | Retrieve operator decisions and campaign outcomes for learning |
| **Configuration** | Framework configures agent | API endpoints, model selection, MCP server connections, rate limits, context windows |

### 3.2 MCP Server Support

The data layer must be designed so that an MCP (Model Context Protocol) server can be built as a thin adapter over existing APIs. The MCP server would expose:

- Campaign data (read-only): targets, templates, schedules, status
- Metrics data (read-only): open rates, click rates, credential capture rates, time-series data
- Configuration data (read-only): available domains, SMTP configurations, landing page types
- Proposal submission (write): submit campaign drafts, content suggestions
- Research storage (write): store and retrieve intelligence findings

This does **not** require implementing an MCP server in v1. It requires that the underlying REST API and data model are structured such that an MCP server can be layered on top without modifying core platform code.

### 3.3 Backlog/Queue System

AI-generated proposals require a dedicated review queue with an approval workflow:

- Proposals exist in a distinct state from operator-created campaigns
- Each proposal has a lifecycle: `pending_review` -> `approved` / `rejected` / `archived`
- Approved proposals promote to draft campaigns in the standard campaign workflow
- The queue supports filtering, sorting, and bulk actions
- Operators receive notifications when new proposals are queued

### 3.4 Feedback Loop Data Model

Campaign outcomes must be stored in a structured format that can be used for AI training and tuning:

- Every campaign records structured outcome metrics (not just aggregates)
- Operator decisions on AI proposals (accept/modify/reject) are stored with context
- Modification diffs are captured — what the operator changed and why
- A/B test results are linked to the content variants that produced them
- Feedback is queryable by campaign type, pretext category, target demographic, and time period

### 3.5 API Key Management

The framework must support managing connections to multiple AI providers:

- Store API keys for external AI services (OpenAI, Anthropic, local LLM endpoints)
- Support multiple concurrent provider configurations
- Provider credentials are encrypted at rest using the same mechanism as other secrets (see 01-system-overview.md, Section 7)
- Configuration includes: endpoint URL, API key, model identifier, rate limits, timeout values
- Support for MCP server connection strings and authentication tokens
- Health check mechanism to verify provider connectivity

## 4. Design Principles

1. **AI is always advisory** — No AI agent can send an email, deploy infrastructure, or modify a live campaign. Every AI output is a suggestion that requires human approval before it affects the real world.

2. **AI actions are fully audited** — Every AI interaction (query, proposal, content generation) is logged through the standard audit logging system (see 11-audit-logging.md). Logs include: the agent identity, the action taken, the input provided, and the output received.

3. **AI-generated content is clearly labeled** — The UI must distinguish AI-generated content from operator-created content. Labels persist through the campaign lifecycle. If an operator modifies AI content, the label updates to "AI-assisted" rather than being removed.

4. **No architectural changes for AI** — AI features layer on top of existing APIs, data models, and workflows. The v1 platform must not require schema migrations, API changes, or workflow modifications to enable AI features in v2.

5. **Graceful degradation** — If AI services are unavailable, the platform operates normally. AI features are additive — the manual workflow is always the fallback.

6. **Provider agnosticism** — The agent interface abstracts AI provider details. Switching from one provider to another requires only configuration changes, not code changes.

## 5. Requirements

### 5.1 Agent Interface Requirements

**REQ-AI-001: Agent Interface Definition** (v1-design)

The v1 codebase must define a Go interface type for AI agents that specifies the contract any future agent implementation must fulfill.

Acceptance Criteria:
- [ ] A Go interface `AIAgent` (or equivalent) exists in the codebase under `tackle/internal/ai/`
- [ ] The interface defines methods for: submitting proposals, querying campaign data, storing research results, and retrieving feedback
- [ ] The interface is documented with Go doc comments explaining each method's purpose, parameters, and expected behavior
- [ ] At least one no-op (stub) implementation of the interface exists for testing and as a reference for future implementors
- [ ] The interface is imported by zero production code paths in v1 — it exists solely as a contract definition

**REQ-AI-002: Agent Registration and Discovery** (v2-implement)

The framework supports registering multiple AI agents at runtime. Each agent declares its capabilities (research, content generation, campaign drafting) and the framework routes requests accordingly.

**REQ-AI-003: Agent Configuration Schema** (v1-design)

The v1 database schema and configuration system must include structures for storing AI agent configurations.

Acceptance Criteria:
- [ ] A database table `ai_agent_configs` exists in the v1 schema with columns for: id, name, agent_type, provider, endpoint_url, model_identifier, api_key_encrypted, rate_limit, timeout_seconds, mcp_server_url, enabled, created_at, updated_at
- [ ] The `api_key_encrypted` column uses the same encryption mechanism as other secret fields in the database
- [ ] The `agent_type` column supports an extensible set of values (enum or reference table) with initial values: `research`, `content_generation`, `campaign_drafting`
- [ ] The schema supports multiple configurations per agent type (e.g., multiple research agents with different providers)
- [ ] A configuration validation function exists that checks required fields based on agent type

**REQ-AI-004: Agent Rate Limiting** (v2-implement)

AI agent API calls are rate-limited per provider configuration to prevent runaway costs. Rate limits are configurable per agent and enforced at the framework level.

### 5.2 Proposal and Queue Requirements

**REQ-AI-005: Proposal Data Model** (v1-design)

The v1 database schema must define a table structure for AI-generated campaign proposals.

Acceptance Criteria:
- [ ] A database table `ai_proposals` exists in the v1 schema with columns for: id, agent_config_id (FK to ai_agent_configs), proposal_type, status, title, proposal_data (JSONB), operator_feedback, promoted_campaign_id (nullable FK to campaigns), created_at, reviewed_at, reviewed_by (FK to users)
- [ ] The `status` column supports values: `pending_review`, `approved`, `rejected`, `archived`
- [ ] The `proposal_type` column supports values: `full_campaign`, `email_template`, `landing_page`, `target_selection`, `timing_recommendation`
- [ ] The `proposal_data` JSONB column has a documented schema for each proposal type (documented in code comments or a schema definition file)
- [ ] Foreign key relationships are defined to campaigns (for promoted proposals) and users (for reviewer tracking)
- [ ] An index exists on `(status, created_at)` to support queue queries

**REQ-AI-006: Proposal Review Queue API** (v1-design)

The v1 REST API must define endpoint stubs for the proposal review queue, returning appropriate "not implemented" responses.

Acceptance Criteria:
- [ ] API route definitions exist for: `GET /api/v1/ai/proposals` (list), `GET /api/v1/ai/proposals/:id` (detail), `PUT /api/v1/ai/proposals/:id/review` (approve/reject), `DELETE /api/v1/ai/proposals/:id` (archive)
- [ ] Each endpoint returns HTTP 501 (Not Implemented) with a JSON body: `{"error": "AI features are not yet available", "status": "planned"}`
- [ ] The routes are registered in the router and protected by authentication and RBAC middleware
- [ ] API documentation (OpenAPI/Swagger annotations or equivalent) describes the planned request/response schemas for each endpoint

**REQ-AI-007: Proposal Promotion Workflow** (v2-implement)

When an operator approves a proposal, the framework creates a draft campaign pre-populated with the proposal data. The operator can then edit and schedule the campaign through the standard workflow.

**REQ-AI-008: Bulk Proposal Management** (v2-implement)

Operators can select multiple proposals and perform bulk actions: approve all, reject all, archive all. Bulk operations are logged as individual audit entries per proposal.

### 5.3 Content Generation Requirements

**REQ-AI-009: Content Origin Tracking** (v1-design)

All content entities (email templates, landing page templates, subject lines) must track their origin — whether they were created manually by an operator or generated/assisted by AI.

Acceptance Criteria:
- [ ] Email template, landing page template, and related content tables include a `content_origin` column with values: `manual`, `ai_generated`, `ai_assisted`
- [ ] The `content_origin` column defaults to `manual` for all v1 operations
- [ ] If a `content_origin` is `ai_generated` or `ai_assisted`, an `ai_agent_config_id` column (nullable FK) references the agent that produced the content
- [ ] The admin UI displays the content origin as a visual indicator (badge, label, or icon) on all content listing and detail views
- [ ] Content origin is included in campaign report exports

**REQ-AI-010: Content Generation API Stubs** (v1-design)

The v1 REST API must define endpoint stubs for AI content generation.

Acceptance Criteria:
- [ ] API route definitions exist for: `POST /api/v1/ai/generate/email-template`, `POST /api/v1/ai/generate/subject-lines`, `POST /api/v1/ai/generate/landing-page-content`, `POST /api/v1/ai/generate/personalization-suggestions`
- [ ] Each endpoint returns HTTP 501 (Not Implemented) with the standard planned-feature response body
- [ ] The routes are registered, authenticated, and RBAC-protected
- [ ] API documentation describes the planned request schemas (input parameters: pretext type, tone, target demographic, etc.) and response schemas (generated content, confidence scores, variant count)

**REQ-AI-011: Content Generation with Variants** (v2-implement)

AI content generation endpoints return multiple variants with confidence scores. Operators select preferred variants or request regeneration with modified parameters.

**REQ-AI-012: Persuasion Optimization Controls** (v2-implement)

Content generation supports configurable persuasion parameters: urgency level, authority impersonation level, social proof emphasis, scarcity framing, and reciprocity triggers. Parameters are stored with the generated content for outcome correlation.

### 5.4 Research and Intelligence Requirements

**REQ-AI-013: Research Results Data Model** (v1-design)

The v1 database schema must define a table structure for storing AI research findings.

Acceptance Criteria:
- [ ] A database table `ai_research_results` exists in the v1 schema with columns for: id, agent_config_id (FK to ai_agent_configs), research_type, target_org (nullable), summary, findings (JSONB), sources (JSONB array of URLs/references), confidence_score (decimal), created_at, expires_at, reviewed (boolean, default false)
- [ ] The `research_type` column supports values: `phishing_trends`, `org_intelligence`, `pretext_ideas`, `defensive_posture`, `industry_analysis`
- [ ] The `findings` JSONB column has a documented schema per research type
- [ ] An index exists on `(research_type, created_at)` and `(target_org, created_at)` to support lookup queries
- [ ] A TTL mechanism exists (via `expires_at`) to flag stale research for re-evaluation

**REQ-AI-014: Research API Stubs** (v1-design)

The v1 REST API must define endpoint stubs for AI research operations.

Acceptance Criteria:
- [ ] API route definitions exist for: `POST /api/v1/ai/research/initiate`, `GET /api/v1/ai/research/results` (list), `GET /api/v1/ai/research/results/:id` (detail), `DELETE /api/v1/ai/research/results/:id`
- [ ] Each endpoint returns HTTP 501 (Not Implemented) with the standard planned-feature response body
- [ ] The routes are registered, authenticated, and RBAC-protected
- [ ] API documentation describes the planned request schemas (research type, target organization, scope constraints) and response schemas

**REQ-AI-015: Automated Research Scheduling** (v2-implement)

AI research agents can be scheduled to run periodically (e.g., weekly trend analysis, pre-campaign org intelligence). Schedules are configurable per agent and managed through the admin UI.

**REQ-AI-016: Research-to-Campaign Linking** (v2-implement)

Research results can be linked to campaigns and proposals. Campaign reports include references to the research that informed campaign design.

### 5.5 Feedback Loop Requirements

**REQ-AI-017: Campaign Outcome Feedback Model** (v1-design)

Campaign outcome data must be stored in a structured format suitable for AI consumption. This extends the existing metrics model (see 10-metrics-reporting.md) with AI-specific structures.

Acceptance Criteria:
- [ ] A database table `ai_feedback_entries` exists in the v1 schema with columns for: id, campaign_id (FK to campaigns), proposal_id (nullable FK to ai_proposals), feedback_type, outcome_data (JSONB), operator_notes (text, nullable), created_at
- [ ] The `feedback_type` column supports values: `campaign_outcome`, `proposal_decision`, `content_modification`, `operator_override`
- [ ] For `proposal_decision` entries, the outcome_data includes: original proposal snapshot, operator decision (accept/modify/reject), modification diff (if modified), rejection reason (if rejected)
- [ ] For `campaign_outcome` entries, the outcome_data includes: structured metrics (open rate, click rate, credential capture rate), target demographic breakdown, timing analysis, comparison to predicted outcomes
- [ ] An index exists on `(feedback_type, created_at)` to support training data queries

**REQ-AI-018: Feedback Collection Automation** (v1-design)

When a campaign completes, the framework automatically generates a feedback entry summarizing outcomes. In v1, this is a database trigger or application-level hook that creates a skeleton `ai_feedback_entries` record.

Acceptance Criteria:
- [ ] When a campaign transitions to a terminal status (`completed`, `cancelled`), an `ai_feedback_entries` record with `feedback_type = 'campaign_outcome'` is automatically created
- [ ] The auto-generated record includes all available structured metrics from the campaign
- [ ] The record creation does not block the campaign status transition (asynchronous or non-failing)
- [ ] The `operator_notes` field is initially null and can be updated by an operator through the campaign detail view

**REQ-AI-019: Feedback Export for Training** (v2-implement)

Feedback data can be exported in formats suitable for AI model fine-tuning (JSONL, CSV). Export supports filtering by date range, campaign type, and outcome metrics.

**REQ-AI-020: Feedback-Driven Improvement Metrics** (v2-implement)

The system tracks AI proposal quality over time — acceptance rate, modification frequency, and outcome correlation — to measure whether the AI is improving.

### 5.6 MCP Server Requirements

**REQ-AI-021: MCP-Compatible Data Access Layer** (v1-design)

The v1 data access layer must be structured to support MCP server integration without modification.

Acceptance Criteria:
- [ ] All campaign, template, target, and metrics data is accessible through well-defined Go service interfaces (not directly through SQL in handlers)
- [ ] Service interfaces use data transfer objects (DTOs) that can be serialized to JSON without transformation
- [ ] Read operations support pagination, filtering, and field selection through interface parameters
- [ ] The service layer does not depend on HTTP-specific types (no `http.Request` or `http.ResponseWriter` in service method signatures)
- [ ] Service interfaces are documented with Go doc comments describing each method's behavior and access semantics

**REQ-AI-022: MCP Server Implementation** (v2-implement)

An MCP server binary can be built that imports the framework's service layer and exposes campaign data, metrics, and configuration to MCP-compatible AI agents. The MCP server runs as a separate process alongside the framework.

### 5.7 Audit and Labeling Requirements

**REQ-AI-023: AI Action Audit Logging** (v1-design)

The v1 audit logging system must define event types and log structures for AI actions.

Acceptance Criteria:
- [ ] The audit log event type enumeration includes AI-specific values: `ai_proposal_submitted`, `ai_proposal_reviewed`, `ai_content_generated`, `ai_research_completed`, `ai_agent_configured`, `ai_agent_error`
- [ ] The audit log schema supports an `ai_agent_config_id` field (nullable) for correlating log entries to specific AI agents
- [ ] AI audit event documentation exists describing what data each event type captures
- [ ] In v1, these event types exist in the enumeration but are never emitted (no AI agents are active)

**REQ-AI-024: AI Content Labeling in UI** (v1-design)

The v1 admin UI must include visual components for displaying AI-origin indicators, even though they will only show "manual" in v1.

Acceptance Criteria:
- [ ] A reusable UI component exists for displaying content origin badges (`manual`, `ai_generated`, `ai_assisted`)
- [ ] The component is integrated into email template list/detail views, landing page template list/detail views, and campaign detail views
- [ ] The component renders a neutral state for `manual` origin (subtle or hidden) and a distinct visual state for AI origins (visible badge with AI indicator)
- [ ] The component accepts an optional `ai_agent_name` prop to display which agent produced the content

### 5.8 API Key and Provider Management Requirements

**REQ-AI-025: AI Provider Configuration Storage** (v1-design)

The v1 system must support storing and managing AI provider configurations.

Acceptance Criteria:
- [ ] The `ai_agent_configs` table (defined in REQ-AI-003) is the single source of truth for AI provider connections
- [ ] A settings UI section exists under administration for managing AI provider configurations (add, edit, disable, delete)
- [ ] The settings UI section displays a "coming soon" or "future feature" state in v1, with the form fields visible but disabled
- [ ] API key values are never returned in API responses — only a masked indicator (e.g., `sk-...7x9z`) confirming a key is stored
- [ ] Provider configurations support a `test_connection` action (v1 stub returns 501)

**REQ-AI-026: Multi-Provider Support** (v2-implement)

The framework supports simultaneous connections to multiple AI providers. Different agent types can be routed to different providers (e.g., content generation to OpenAI, research to Anthropic, specialized tasks to a local LLM). Provider failover and fallback chains are configurable.

**REQ-AI-027: MCP Server Connection Management** (v2-implement)

AI agents can connect to external MCP servers for tool access. MCP server connections are managed through the same configuration interface as AI provider connections, with support for authentication tokens, connection health monitoring, and capability discovery.

## 6. Data Model Summary

The following tables are added to the v1 schema to support AI readiness. All tables follow the conventions defined in [14-database-schema.md](14-database-schema.md).

```
┌─────────────────────┐     ┌──────────────────────┐
│  ai_agent_configs   │     │    ai_proposals      │
│─────────────────────│     │──────────────────────│
│  id (PK)            │◄────│  agent_config_id (FK)│
│  name               │     │  id (PK)             │
│  agent_type         │     │  proposal_type       │
│  provider           │     │  status              │
│  endpoint_url       │     │  title               │
│  model_identifier   │     │  proposal_data       │
│  api_key_encrypted  │     │  operator_feedback   │
│  rate_limit         │     │  promoted_campaign_id│
│  timeout_seconds    │     │  created_at          │
│  mcp_server_url     │     │  reviewed_at         │
│  enabled            │     │  reviewed_by (FK)    │
│  created_at         │     └──────────────────────┘
│  updated_at         │
└─────────┬───────────┘     ┌──────────────────────┐
          │                 │ ai_research_results  │
          │                 │──────────────────────│
          ├────────────────►│  agent_config_id (FK)│
          │                 │  id (PK)             │
          │                 │  research_type       │
          │                 │  target_org          │
          │                 │  summary             │
          │                 │  findings (JSONB)    │
          │                 │  sources (JSONB)     │
          │                 │  confidence_score    │
          │                 │  created_at          │
          │                 │  expires_at          │
          │                 │  reviewed            │
          │                 └──────────────────────┘
          │
          │                 ┌──────────────────────┐
          │                 │ ai_feedback_entries  │
          │                 │──────────────────────│
          └────────────────►│  id (PK)             │
                            │  campaign_id (FK)    │
                            │  proposal_id (FK)    │
                            │  feedback_type       │
                            │  outcome_data (JSONB)│
                            │  operator_notes      │
                            │  created_at          │
                            └──────────────────────┘
```

Additionally, the following columns are added to existing tables:

| Existing Table | New Column | Type | Purpose |
|---------------|-----------|------|---------|
| `email_templates` | `content_origin` | varchar, default `'manual'` | Tracks whether content is manual, AI-generated, or AI-assisted |
| `email_templates` | `ai_agent_config_id` | FK (nullable) | Links to the AI agent that produced the content |
| `landing_page_templates` | `content_origin` | varchar, default `'manual'` | Tracks whether content is manual, AI-generated, or AI-assisted |
| `landing_page_templates` | `ai_agent_config_id` | FK (nullable) | Links to the AI agent that produced the content |

## 7. API Route Summary

All AI API routes are registered in v1 and return 501 (Not Implemented) until v2 activates them.

| Method | Route | Purpose | v1 Status |
|--------|-------|---------|-----------|
| GET | `/api/v1/ai/proposals` | List AI proposals | 501 stub |
| GET | `/api/v1/ai/proposals/:id` | Get proposal detail | 501 stub |
| PUT | `/api/v1/ai/proposals/:id/review` | Approve/reject proposal | 501 stub |
| DELETE | `/api/v1/ai/proposals/:id` | Archive proposal | 501 stub |
| POST | `/api/v1/ai/generate/email-template` | Generate email template | 501 stub |
| POST | `/api/v1/ai/generate/subject-lines` | Generate subject lines | 501 stub |
| POST | `/api/v1/ai/generate/landing-page-content` | Generate landing page content | 501 stub |
| POST | `/api/v1/ai/generate/personalization-suggestions` | Generate personalization ideas | 501 stub |
| POST | `/api/v1/ai/research/initiate` | Start research task | 501 stub |
| GET | `/api/v1/ai/research/results` | List research results | 501 stub |
| GET | `/api/v1/ai/research/results/:id` | Get research result detail | 501 stub |
| DELETE | `/api/v1/ai/research/results/:id` | Delete research result | 501 stub |
| GET | `/api/v1/ai/config/providers` | List AI provider configs | 501 stub |
| POST | `/api/v1/ai/config/providers` | Create provider config | 501 stub |
| PUT | `/api/v1/ai/config/providers/:id` | Update provider config | 501 stub |
| DELETE | `/api/v1/ai/config/providers/:id` | Delete provider config | 501 stub |
| POST | `/api/v1/ai/config/providers/:id/test` | Test provider connection | 501 stub |
| GET | `/api/v1/ai/feedback` | List feedback entries | 501 stub |
| GET | `/api/v1/ai/feedback/:id` | Get feedback entry detail | 501 stub |

## 8. RBAC Considerations

AI features introduce the following permissions that must be defined in the v1 RBAC model (see [02-authentication-authorization.md](02-authentication-authorization.md)):

| Permission | Description | Default Role |
|-----------|-------------|-------------|
| `ai.proposals.read` | View AI-generated proposals | Operator, Engineer, Admin |
| `ai.proposals.review` | Approve, reject, or archive proposals | Operator, Admin |
| `ai.content.generate` | Trigger AI content generation | Operator, Admin |
| `ai.research.read` | View AI research results | Operator, Engineer, Admin |
| `ai.research.initiate` | Trigger AI research tasks | Operator, Admin |
| `ai.config.read` | View AI provider configurations | Engineer, Admin |
| `ai.config.manage` | Create, edit, delete AI provider configurations | Admin |
| `ai.feedback.read` | View AI feedback entries | Operator, Engineer, Admin |

In v1, these permissions exist in the RBAC model but are associated with endpoints that return 501. They must still be enforced by middleware so that RBAC coverage is complete when v2 activates the features.

## 9. Acceptance Criteria (Document-Level)

These criteria validate that v1 is AI-ready without implementing AI features:

- [ ] All four AI database tables (`ai_agent_configs`, `ai_proposals`, `ai_research_results`, `ai_feedback_entries`) exist in the v1 schema with the columns specified in this document
- [ ] Content origin columns exist on `email_templates` and `landing_page_templates` with default value `'manual'`
- [ ] The `AIAgent` Go interface is defined and documented in `tackle/internal/ai/`
- [ ] All 19 AI API routes are registered, authenticated, RBAC-protected, and return 501
- [ ] All 8 AI RBAC permissions are defined in the authorization model
- [ ] All 6 AI audit event types are defined in the audit log enumeration
- [ ] The AI content origin UI component exists and is integrated into template views
- [ ] The AI provider settings UI section exists in administration (disabled/placeholder state)
- [ ] Campaign completion automatically creates a skeleton `ai_feedback_entries` record
- [ ] The service layer for campaigns, templates, targets, and metrics uses provider-agnostic interfaces with serializable DTOs (no HTTP types in service signatures)
- [ ] Zero v1 code paths invoke AI agent methods or connect to AI providers — all AI functionality is inert
