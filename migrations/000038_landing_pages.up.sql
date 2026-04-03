-- Migration 038: Landing page builder tables.
-- REQ-DB-023: Landing page projects, templates, builds, health checks.

CREATE TABLE landing_page_projects (
    id              UUID            NOT NULL DEFAULT gen_random_uuid() PRIMARY KEY,
    name            VARCHAR(255)    NOT NULL,
    description     TEXT            NOT NULL DEFAULT '',
    definition_json JSONB           NOT NULL DEFAULT '{}'::jsonb,
    created_by      UUID            NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
    deleted_at      TIMESTAMPTZ,
    created_at      TIMESTAMPTZ     NOT NULL DEFAULT now(),
    updated_at      TIMESTAMPTZ     NOT NULL DEFAULT now()
);

CREATE INDEX idx_lpp_created_by ON landing_page_projects (created_by);
CREATE INDEX idx_lpp_deleted_at ON landing_page_projects (deleted_at) WHERE deleted_at IS NULL;

CREATE UNIQUE INDEX idx_lpp_name_active
    ON landing_page_projects (LOWER(name))
    WHERE deleted_at IS NULL;

CREATE TRIGGER trg_landing_page_projects_updated_at
    BEFORE UPDATE ON landing_page_projects
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();

-- Landing page templates: reusable starter templates and user-saved templates.
CREATE TABLE landing_page_templates (
    id              UUID            NOT NULL DEFAULT gen_random_uuid() PRIMARY KEY,
    name            VARCHAR(255)    NOT NULL,
    description     TEXT            NOT NULL DEFAULT '',
    category        VARCHAR(64)     NOT NULL DEFAULT 'custom',
    definition_json JSONB           NOT NULL DEFAULT '{}'::jsonb,
    created_by      UUID            NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
    is_shared       BOOLEAN         NOT NULL DEFAULT false,
    created_at      TIMESTAMPTZ     NOT NULL DEFAULT now()
);

CREATE INDEX idx_lpt_created_by ON landing_page_templates (created_by);
CREATE INDEX idx_lpt_shared ON landing_page_templates (is_shared) WHERE is_shared = true;

-- Landing page builds: compiled artifacts per project/campaign.
CREATE TABLE landing_page_builds (
    id                  UUID            NOT NULL DEFAULT gen_random_uuid() PRIMARY KEY,
    project_id          UUID            NOT NULL REFERENCES landing_page_projects(id) ON DELETE CASCADE,
    campaign_id         UUID            REFERENCES campaigns(id) ON DELETE SET NULL,
    seed                BIGINT          NOT NULL DEFAULT 0,
    strategy            VARCHAR(64)     NOT NULL DEFAULT 'default',
    build_manifest_json JSONB           NOT NULL DEFAULT '{}'::jsonb,
    build_log           TEXT            NOT NULL DEFAULT '',
    binary_path         VARCHAR(512),
    binary_hash         VARCHAR(128),
    status              TEXT            NOT NULL DEFAULT 'pending'
                        CHECK (status IN ('pending', 'building', 'built', 'starting', 'running', 'stopping', 'stopped', 'failed', 'cleaned_up')),
    port                INTEGER         CHECK (port IS NULL OR (port >= 1024 AND port <= 65535)),
    build_token         VARCHAR(256),
    created_at          TIMESTAMPTZ     NOT NULL DEFAULT now()
);

CREATE INDEX idx_lpb_project_id ON landing_page_builds (project_id);
CREATE INDEX idx_lpb_campaign_id ON landing_page_builds (campaign_id) WHERE campaign_id IS NOT NULL;
CREATE INDEX idx_lpb_status ON landing_page_builds (status);

-- Landing page health checks.
CREATE TABLE landing_page_health_checks (
    id              UUID            NOT NULL DEFAULT gen_random_uuid() PRIMARY KEY,
    build_id        UUID            NOT NULL REFERENCES landing_page_builds(id) ON DELETE CASCADE,
    status          VARCHAR(32)     NOT NULL,
    response_time_ms INTEGER        NOT NULL DEFAULT 0,
    checked_at      TIMESTAMPTZ     NOT NULL DEFAULT now()
);

CREATE INDEX idx_lphc_build_id ON landing_page_health_checks (build_id);

-- Add FK from campaigns to landing_page_projects now that the table exists.
ALTER TABLE campaigns
    ADD CONSTRAINT fk_campaigns_landing_page FOREIGN KEY (landing_page_id)
    REFERENCES landing_page_projects(id) ON DELETE SET NULL;

-- Seed landing page permissions.
INSERT INTO permissions (resource_type, action, description) VALUES
    ('landing_pages', 'read',   'View landing page projects'),
    ('landing_pages', 'create', 'Create landing page projects'),
    ('landing_pages', 'update', 'Update landing page projects'),
    ('landing_pages', 'delete', 'Delete landing page projects')
ON CONFLICT (resource_type, action) DO NOTHING;

-- Grant landing page permissions to Operator, Engineer, and Admin roles.
INSERT INTO role_permissions (role_id, permission_id)
SELECT r.id, p.id
FROM roles r, permissions p
WHERE r.name IN ('operator', 'engineer', 'admin')
  AND p.resource_type = 'landing_pages'
  AND p.action IN ('read', 'create', 'update', 'delete')
ON CONFLICT DO NOTHING;
