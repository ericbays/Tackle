-- Migration 014: AI readiness tables (schema only, no application code in v1).
-- Reference: Section 4.8 of 14-database-schema.md, REQ-DB-080, REQ-DB-081.

-- ai_providers: configuration for external AI provider integrations.
CREATE TABLE ai_providers (
    id            UUID        NOT NULL DEFAULT gen_random_uuid() PRIMARY KEY,
    type          TEXT        NOT NULL,
    name          TEXT        NOT NULL UNIQUE,
    configuration BYTEA       NOT NULL,
    enabled       BOOLEAN     NOT NULL DEFAULT FALSE,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at    TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TRIGGER trg_ai_providers_updated_at
    BEFORE UPDATE ON ai_providers
    FOR EACH ROW
    EXECUTE FUNCTION set_updated_at();

CREATE INDEX idx_ai_providers_enabled ON ai_providers (enabled) WHERE enabled = TRUE;

-- ai_proposals: AI-generated content proposals awaiting human review.
-- campaign_id has no FK yet; campaigns table is Phase 2+.
CREATE TABLE ai_proposals (
    id            UUID        NOT NULL DEFAULT gen_random_uuid() PRIMARY KEY,
    provider_id   UUID        NOT NULL REFERENCES ai_providers(id) ON DELETE RESTRICT,
    campaign_id   UUID,
    proposal_type TEXT        NOT NULL,
    content       JSONB       NOT NULL,
    status        TEXT        NOT NULL DEFAULT 'pending'
                      CHECK (status IN ('pending', 'accepted', 'rejected', 'modified')),
    reviewed_by   UUID        REFERENCES users(id) ON DELETE SET NULL,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at    TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TRIGGER trg_ai_proposals_updated_at
    BEFORE UPDATE ON ai_proposals
    FOR EACH ROW
    EXECUTE FUNCTION set_updated_at();

CREATE INDEX idx_ai_proposals_provider_id ON ai_proposals (provider_id);
CREATE INDEX idx_ai_proposals_status      ON ai_proposals (status) WHERE status = 'pending';
