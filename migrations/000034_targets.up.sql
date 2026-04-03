-- Migration 034: targets, campaign_targets, and campaign_target_events tables.

-- REQ-DB-030: Core target table for phishing campaign recipients.
CREATE TABLE targets (
    id            UUID            NOT NULL DEFAULT gen_random_uuid() PRIMARY KEY,
    email         TEXT            NOT NULL,
    first_name    TEXT,
    last_name     TEXT,
    department    TEXT,
    title         TEXT,
    custom_fields JSONB           NOT NULL DEFAULT '{}'::jsonb,
    created_by    UUID            NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
    deleted_at    TIMESTAMPTZ,
    created_at    TIMESTAMPTZ     NOT NULL DEFAULT now(),
    updated_at    TIMESTAMPTZ     NOT NULL DEFAULT now()
);

-- Case-insensitive unique index on email for active (non-deleted) targets.
CREATE UNIQUE INDEX idx_targets_email_active ON targets (LOWER(email)) WHERE deleted_at IS NULL;

-- Indexes for common filters.
CREATE INDEX idx_targets_department ON targets (department) WHERE deleted_at IS NULL;
CREATE INDEX idx_targets_last_name ON targets (last_name) WHERE deleted_at IS NULL;
CREATE INDEX idx_targets_deleted_at ON targets (deleted_at) WHERE deleted_at IS NULL;
CREATE INDEX idx_targets_created_at ON targets (created_at);
CREATE INDEX idx_targets_created_by ON targets (created_by);

-- Trigger to keep updated_at current.
CREATE TRIGGER trg_targets_updated_at
    BEFORE UPDATE ON targets
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();

-- Campaign-target join table for per-campaign targeting and status tracking.
CREATE TABLE campaign_targets (
    id          UUID            NOT NULL DEFAULT gen_random_uuid() PRIMARY KEY,
    campaign_id UUID            NOT NULL,
    target_id   UUID            NOT NULL REFERENCES targets(id) ON DELETE RESTRICT,
    status      TEXT            NOT NULL DEFAULT 'pending'
                CHECK (status IN ('pending', 'email_sent', 'email_opened', 'link_clicked', 'credential_submitted')),
    reported    BOOLEAN         NOT NULL DEFAULT false,
    assigned_at TIMESTAMPTZ     NOT NULL DEFAULT now(),
    assigned_by UUID            NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
    removed_at  TIMESTAMPTZ,
    created_at  TIMESTAMPTZ     NOT NULL DEFAULT now(),
    updated_at  TIMESTAMPTZ     NOT NULL DEFAULT now(),

    CONSTRAINT uq_campaign_targets UNIQUE (campaign_id, target_id)
);

CREATE INDEX idx_campaign_targets_campaign_id ON campaign_targets (campaign_id);
CREATE INDEX idx_campaign_targets_target_id ON campaign_targets (target_id);
CREATE INDEX idx_campaign_targets_status ON campaign_targets (status);

CREATE TRIGGER trg_campaign_targets_updated_at
    BEFORE UPDATE ON campaign_targets
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();

-- Per-target event timeline for campaign interaction tracking.
CREATE TABLE campaign_target_events (
    id          UUID            NOT NULL DEFAULT gen_random_uuid() PRIMARY KEY,
    campaign_id UUID            NOT NULL,
    target_id   UUID            NOT NULL REFERENCES targets(id) ON DELETE RESTRICT,
    event_type  TEXT            NOT NULL
                CHECK (event_type IN ('email_sent', 'email_bounced', 'email_opened', 'link_clicked', 'page_visited', 'credential_submitted', 'reported')),
    event_data  JSONB           NOT NULL DEFAULT '{}'::jsonb,
    ip_address  TEXT,
    user_agent  TEXT,
    created_at  TIMESTAMPTZ     NOT NULL DEFAULT now()
);

CREATE INDEX idx_cte_campaign_target ON campaign_target_events (campaign_id, target_id);
CREATE INDEX idx_cte_event_type ON campaign_target_events (event_type);
CREATE INDEX idx_cte_created_at ON campaign_target_events (created_at);

-- CSV import uploads tracking table.
CREATE TABLE target_imports (
    id            UUID            NOT NULL DEFAULT gen_random_uuid() PRIMARY KEY,
    filename      TEXT            NOT NULL,
    row_count     INTEGER         NOT NULL DEFAULT 0,
    column_count  INTEGER         NOT NULL DEFAULT 0,
    headers       JSONB           NOT NULL DEFAULT '[]'::jsonb,
    preview_rows  JSONB           NOT NULL DEFAULT '[]'::jsonb,
    raw_data      BYTEA,
    mapping       JSONB,
    status        TEXT            NOT NULL DEFAULT 'uploaded'
                  CHECK (status IN ('uploaded', 'mapped', 'validated', 'committed', 'failed')),
    validation_result JSONB,
    imported_count INTEGER        NOT NULL DEFAULT 0,
    rejected_count INTEGER        NOT NULL DEFAULT 0,
    uploaded_by   UUID            NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
    created_at    TIMESTAMPTZ     NOT NULL DEFAULT now(),
    updated_at    TIMESTAMPTZ     NOT NULL DEFAULT now()
);

CREATE TRIGGER trg_target_imports_updated_at
    BEFORE UPDATE ON target_imports
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();

-- CSV import mapping templates (reusable across imports).
CREATE TABLE import_mapping_templates (
    id          UUID            NOT NULL DEFAULT gen_random_uuid() PRIMARY KEY,
    name        TEXT            NOT NULL,
    mapping     JSONB           NOT NULL,
    created_by  UUID            NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
    created_at  TIMESTAMPTZ     NOT NULL DEFAULT now(),
    updated_at  TIMESTAMPTZ     NOT NULL DEFAULT now(),

    CONSTRAINT uq_import_mapping_templates_name UNIQUE (name)
);

CREATE TRIGGER trg_import_mapping_templates_updated_at
    BEFORE UPDATE ON import_mapping_templates
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();
