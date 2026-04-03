-- Migration 035: target groups, group membership, campaign-group assignment,
-- block list, campaign canary targets.

-- REQ-DB-031: Named collections of targets for campaign assignment.
CREATE TABLE target_groups (
    id          UUID        NOT NULL DEFAULT gen_random_uuid() PRIMARY KEY,
    name        VARCHAR(255) NOT NULL,
    description VARCHAR(1024) NOT NULL DEFAULT '',
    created_by  UUID        NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- Case-insensitive unique index on name.
CREATE UNIQUE INDEX idx_target_groups_name ON target_groups (LOWER(name));

CREATE TRIGGER trg_target_groups_updated_at
    BEFORE UPDATE ON target_groups
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();

-- REQ-DB-032: Junction table linking targets to groups.
CREATE TABLE target_group_members (
    group_id  UUID        NOT NULL REFERENCES target_groups(id) ON DELETE CASCADE,
    target_id UUID        NOT NULL REFERENCES targets(id) ON DELETE CASCADE,
    added_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    added_by  UUID        NOT NULL REFERENCES users(id) ON DELETE RESTRICT,

    CONSTRAINT pk_target_group_members PRIMARY KEY (group_id, target_id)
);

CREATE INDEX idx_tgm_target_id ON target_group_members (target_id);
CREATE INDEX idx_tgm_group_id ON target_group_members (group_id);

-- Campaign-group assignment table.
CREATE TABLE campaign_target_groups (
    campaign_id UUID        NOT NULL,
    group_id    UUID        NOT NULL REFERENCES target_groups(id) ON DELETE RESTRICT,
    assigned_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    assigned_by UUID        NOT NULL REFERENCES users(id) ON DELETE RESTRICT,

    CONSTRAINT pk_campaign_target_groups PRIMARY KEY (campaign_id, group_id)
);

CREATE INDEX idx_ctg_campaign_id ON campaign_target_groups (campaign_id);
CREATE INDEX idx_ctg_group_id ON campaign_target_groups (group_id);

-- REQ-DB-034: Block list patterns.
CREATE TABLE blocklist_entries (
    id        UUID          NOT NULL DEFAULT gen_random_uuid() PRIMARY KEY,
    pattern   VARCHAR(512)  NOT NULL,
    reason    VARCHAR(2048) NOT NULL,
    is_active BOOLEAN       NOT NULL DEFAULT true,
    added_by  UUID          NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
    added_at  TIMESTAMPTZ   NOT NULL DEFAULT now()
);

-- Case-insensitive unique index on pattern.
CREATE UNIQUE INDEX idx_blocklist_pattern ON blocklist_entries (LOWER(pattern));

-- Index for efficient active-only queries.
CREATE INDEX idx_blocklist_active ON blocklist_entries (is_active) WHERE is_active = true;

-- Campaign canary targets.
CREATE TABLE campaign_canary_targets (
    campaign_id   UUID        NOT NULL,
    target_id     UUID        NOT NULL REFERENCES targets(id) ON DELETE CASCADE,
    designated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    designated_by UUID        NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
    verified_at   TIMESTAMPTZ,

    CONSTRAINT pk_campaign_canary_targets PRIMARY KEY (campaign_id, target_id)
);

CREATE INDEX idx_cct_campaign_id ON campaign_canary_targets (campaign_id);
CREATE INDEX idx_cct_target_id ON campaign_canary_targets (target_id);

-- Block list override approvals for campaigns.
CREATE TABLE blocklist_overrides (
    id              UUID          NOT NULL DEFAULT gen_random_uuid() PRIMARY KEY,
    campaign_id     UUID          NOT NULL,
    status          TEXT          NOT NULL DEFAULT 'pending'
                    CHECK (status IN ('pending', 'approved', 'rejected')),
    blocked_targets JSONB         NOT NULL DEFAULT '[]'::jsonb,
    target_hash     TEXT          NOT NULL,
    acknowledgment  BOOLEAN       NOT NULL DEFAULT false,
    justification   TEXT,
    rejection_reason TEXT,
    decided_by      UUID          REFERENCES users(id) ON DELETE RESTRICT,
    decided_at      TIMESTAMPTZ,
    created_at      TIMESTAMPTZ   NOT NULL DEFAULT now()
);

CREATE INDEX idx_blocklist_overrides_campaign ON blocklist_overrides (campaign_id);
CREATE INDEX idx_blocklist_overrides_status ON blocklist_overrides (status);
