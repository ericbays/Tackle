-- Migration 036: Campaign lifecycle tables.
-- REQ-DB-020: Core campaign table with lifecycle state machine.

CREATE TABLE campaigns (
    id                    UUID            NOT NULL DEFAULT gen_random_uuid() PRIMARY KEY,
    name                  VARCHAR(255)    NOT NULL,
    description           TEXT            NOT NULL DEFAULT '',
    current_state         TEXT            NOT NULL DEFAULT 'draft'
                          CHECK (current_state IN ('draft', 'pending_approval', 'approved', 'building', 'ready', 'active', 'paused', 'completed', 'archived')),
    state_changed_at      TIMESTAMPTZ     NOT NULL DEFAULT now(),
    landing_page_id       UUID,
    cloud_provider        TEXT            CHECK (cloud_provider IS NULL OR cloud_provider IN ('aws', 'azure')),
    region                VARCHAR(64),
    instance_type         VARCHAR(128),
    endpoint_domain_id    UUID,
    throttle_rate         INTEGER,
    inter_email_delay_min INTEGER,
    inter_email_delay_max INTEGER,
    send_order            TEXT            NOT NULL DEFAULT 'default'
                          CHECK (send_order IN ('default', 'alphabetical', 'department', 'custom', 'randomized')),
    scheduled_launch_at   TIMESTAMPTZ,
    grace_period_hours    INTEGER         NOT NULL DEFAULT 72,
    start_date            TIMESTAMPTZ,
    end_date              TIMESTAMPTZ,
    approved_by           UUID            REFERENCES users(id) ON DELETE SET NULL,
    approval_comment      TEXT,
    launched_at           TIMESTAMPTZ,
    completed_at          TIMESTAMPTZ,
    archived_at           TIMESTAMPTZ,
    configuration         JSONB           NOT NULL DEFAULT '{}'::jsonb,
    created_by            UUID            NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
    deleted_at            TIMESTAMPTZ,
    created_at            TIMESTAMPTZ     NOT NULL DEFAULT now(),
    updated_at            TIMESTAMPTZ     NOT NULL DEFAULT now(),

    CONSTRAINT chk_campaigns_dates CHECK (end_date IS NULL OR start_date IS NULL OR end_date > start_date)
);

-- Unique name among non-archived, non-deleted campaigns.
CREATE UNIQUE INDEX idx_campaigns_name_active
    ON campaigns (LOWER(name))
    WHERE current_state != 'archived' AND deleted_at IS NULL;

CREATE INDEX idx_campaigns_state ON campaigns (current_state) WHERE deleted_at IS NULL;
CREATE INDEX idx_campaigns_created_by ON campaigns (created_by);
CREATE INDEX idx_campaigns_launched_at ON campaigns (launched_at) WHERE launched_at IS NOT NULL;
CREATE INDEX idx_campaigns_deleted_at ON campaigns (deleted_at) WHERE deleted_at IS NULL;
CREATE INDEX idx_campaigns_scheduled_launch ON campaigns (scheduled_launch_at)
    WHERE scheduled_launch_at IS NOT NULL AND current_state = 'ready';

CREATE TRIGGER trg_campaigns_updated_at
    BEFORE UPDATE ON campaigns
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();

-- Add FK from campaign_targets to campaigns now that campaigns table exists.
ALTER TABLE campaign_targets
    ADD CONSTRAINT fk_campaign_targets_campaign FOREIGN KEY (campaign_id) REFERENCES campaigns(id) ON DELETE CASCADE;

-- Add FK from campaign_target_events to campaigns.
ALTER TABLE campaign_target_events
    ADD CONSTRAINT fk_campaign_target_events_campaign FOREIGN KEY (campaign_id) REFERENCES campaigns(id) ON DELETE CASCADE;

-- Add FK from campaign_target_groups to campaigns.
ALTER TABLE campaign_target_groups
    ADD CONSTRAINT fk_campaign_target_groups_campaign FOREIGN KEY (campaign_id) REFERENCES campaigns(id) ON DELETE CASCADE;

-- Add FK from campaign_canary_targets to campaigns.
ALTER TABLE campaign_canary_targets
    ADD CONSTRAINT fk_campaign_canary_targets_campaign FOREIGN KEY (campaign_id) REFERENCES campaigns(id) ON DELETE CASCADE;

-- Add FK from blocklist_overrides to campaigns.
ALTER TABLE blocklist_overrides
    ADD CONSTRAINT fk_blocklist_overrides_campaign FOREIGN KEY (campaign_id) REFERENCES campaigns(id) ON DELETE CASCADE;

-- REQ-CAMP-005: Campaign template variants for A/B testing.
CREATE TABLE campaign_template_variants (
    id            UUID            NOT NULL DEFAULT gen_random_uuid() PRIMARY KEY,
    campaign_id   UUID            NOT NULL REFERENCES campaigns(id) ON DELETE CASCADE,
    template_id   UUID            NOT NULL REFERENCES email_templates(id) ON DELETE RESTRICT,
    split_ratio   INTEGER         NOT NULL DEFAULT 100
                  CHECK (split_ratio >= 1 AND split_ratio <= 100),
    label         VARCHAR(64)     NOT NULL DEFAULT 'Variant A',
    created_at    TIMESTAMPTZ     NOT NULL DEFAULT now(),

    CONSTRAINT uq_campaign_template_variant UNIQUE (campaign_id, template_id)
);

CREATE INDEX idx_ctv_campaign_id ON campaign_template_variants (campaign_id);
CREATE INDEX idx_ctv_template_id ON campaign_template_variants (template_id);

-- Comment: split_ratio sum per campaign must equal 100 — enforced at application level.

-- REQ-CAMP-006: Campaign send windows.
CREATE TABLE campaign_send_windows (
    id          UUID            NOT NULL DEFAULT gen_random_uuid() PRIMARY KEY,
    campaign_id UUID            NOT NULL REFERENCES campaigns(id) ON DELETE CASCADE,
    days        JSONB           NOT NULL DEFAULT '[]'::jsonb,
    start_time  TIME            NOT NULL,
    end_time    TIME            NOT NULL,
    timezone    VARCHAR(64)     NOT NULL DEFAULT 'UTC',
    created_at  TIMESTAMPTZ     NOT NULL DEFAULT now(),

    CONSTRAINT chk_send_window_times CHECK (end_time > start_time)
);

CREATE INDEX idx_csw_campaign_id ON campaign_send_windows (campaign_id);

-- Campaign targets snapshot: frozen target set at build time.
CREATE TABLE campaign_targets_snapshot (
    id                  UUID            NOT NULL DEFAULT gen_random_uuid() PRIMARY KEY,
    campaign_id         UUID            NOT NULL REFERENCES campaigns(id) ON DELETE CASCADE,
    target_id           UUID            NOT NULL REFERENCES targets(id) ON DELETE RESTRICT,
    variant_label       VARCHAR(64),
    send_order_position INTEGER,
    is_canary           BOOLEAN         NOT NULL DEFAULT false,
    snapshotted_at      TIMESTAMPTZ     NOT NULL DEFAULT now(),

    CONSTRAINT uq_snapshot_campaign_target UNIQUE (campaign_id, target_id)
);

CREATE INDEX idx_cts_campaign_id ON campaign_targets_snapshot (campaign_id);
CREATE INDEX idx_cts_send_order ON campaign_targets_snapshot (campaign_id, send_order_position);

-- Campaign target variant assignments (A/B split assignments after build).
CREATE TABLE campaign_target_variant_assignments (
    campaign_id UUID            NOT NULL REFERENCES campaigns(id) ON DELETE CASCADE,
    target_id   UUID            NOT NULL REFERENCES targets(id) ON DELETE RESTRICT,
    variant_id  UUID            NOT NULL REFERENCES campaign_template_variants(id) ON DELETE CASCADE,
    assigned_at TIMESTAMPTZ     NOT NULL DEFAULT now(),

    CONSTRAINT pk_ctva PRIMARY KEY (campaign_id, target_id)
);

CREATE INDEX idx_ctva_variant ON campaign_target_variant_assignments (variant_id);

-- Campaign state transition history (append-only log).
CREATE TABLE campaign_state_transitions (
    id          UUID            NOT NULL DEFAULT gen_random_uuid() PRIMARY KEY,
    campaign_id UUID            NOT NULL REFERENCES campaigns(id) ON DELETE CASCADE,
    from_state  TEXT            NOT NULL,
    to_state    TEXT            NOT NULL,
    actor_id    UUID            REFERENCES users(id) ON DELETE SET NULL,
    reason      TEXT            NOT NULL DEFAULT '',
    created_at  TIMESTAMPTZ     NOT NULL DEFAULT now()
);

CREATE INDEX idx_cst_campaign_id ON campaign_state_transitions (campaign_id);
CREATE INDEX idx_cst_created_at ON campaign_state_transitions (created_at);

-- Campaign build logs (append-only, step-by-step progress).
CREATE TABLE campaign_build_logs (
    id            UUID            NOT NULL DEFAULT gen_random_uuid() PRIMARY KEY,
    campaign_id   UUID            NOT NULL REFERENCES campaigns(id) ON DELETE CASCADE,
    step_name     VARCHAR(128)    NOT NULL,
    step_order    INTEGER         NOT NULL,
    status        TEXT            NOT NULL DEFAULT 'pending'
                  CHECK (status IN ('pending', 'running', 'completed', 'failed', 'rolled_back')),
    started_at    TIMESTAMPTZ,
    completed_at  TIMESTAMPTZ,
    error_details TEXT,
    created_at    TIMESTAMPTZ     NOT NULL DEFAULT now()
);

CREATE INDEX idx_cbl_campaign_id ON campaign_build_logs (campaign_id);
CREATE INDEX idx_cbl_status ON campaign_build_logs (campaign_id, status);

-- Campaign emails: per-target email dispatch tracking.
CREATE TABLE campaign_emails (
    id              UUID            NOT NULL DEFAULT gen_random_uuid() PRIMARY KEY,
    campaign_id     UUID            NOT NULL REFERENCES campaigns(id) ON DELETE CASCADE,
    target_id       UUID            NOT NULL REFERENCES targets(id) ON DELETE RESTRICT,
    variant_id      UUID            REFERENCES campaign_template_variants(id) ON DELETE SET NULL,
    smtp_config_id  UUID,
    status          TEXT            NOT NULL DEFAULT 'queued'
                    CHECK (status IN ('queued', 'sending', 'sent', 'delivered', 'deferred', 'bounced', 'failed', 'cancelled')),
    message_id      TEXT,
    sent_at         TIMESTAMPTZ,
    delivered_at    TIMESTAMPTZ,
    bounced_at      TIMESTAMPTZ,
    bounce_type     TEXT,
    bounce_message  TEXT,
    retry_count     INTEGER         NOT NULL DEFAULT 0,
    send_order_position INTEGER,
    created_at      TIMESTAMPTZ     NOT NULL DEFAULT now(),
    updated_at      TIMESTAMPTZ     NOT NULL DEFAULT now()
);

CREATE INDEX idx_ce_campaign_id ON campaign_emails (campaign_id);
CREATE INDEX idx_ce_target_status ON campaign_emails (campaign_id, status);
CREATE INDEX idx_ce_target_id ON campaign_emails (target_id);
CREATE INDEX idx_ce_sent_at ON campaign_emails (campaign_id, sent_at) WHERE sent_at IS NOT NULL;
CREATE INDEX idx_ce_send_order ON campaign_emails (campaign_id, send_order_position);

CREATE TRIGGER trg_campaign_emails_updated_at
    BEFORE UPDATE ON campaign_emails
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();

-- Reusable campaign templates (saved configurations).
CREATE TABLE campaign_config_templates (
    id          UUID            NOT NULL DEFAULT gen_random_uuid() PRIMARY KEY,
    name        VARCHAR(255)    NOT NULL,
    description TEXT            NOT NULL DEFAULT '',
    config_json JSONB           NOT NULL DEFAULT '{}'::jsonb,
    created_by  UUID            NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
    created_at  TIMESTAMPTZ     NOT NULL DEFAULT now(),
    updated_at  TIMESTAMPTZ     NOT NULL DEFAULT now()
);

CREATE UNIQUE INDEX idx_cct_name ON campaign_config_templates (LOWER(name));

CREATE TRIGGER trg_campaign_config_templates_updated_at
    BEFORE UPDATE ON campaign_config_templates
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();
