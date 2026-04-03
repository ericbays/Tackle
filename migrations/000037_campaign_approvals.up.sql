-- Migration 037: Campaign approval workflow tables.
-- REQ-CAMP-009, REQ-CAMP-030, REQ-CAMP-039, REQ-DB-090, REQ-DB-091.

-- Campaign approval records: tracks every approval-related action (submit, approve, reject, unlock).
CREATE TABLE campaign_approvals (
    id                      UUID            NOT NULL DEFAULT gen_random_uuid() PRIMARY KEY,
    campaign_id             UUID            NOT NULL REFERENCES campaigns(id) ON DELETE CASCADE,
    submission_id           UUID            NOT NULL,
    actor_id                UUID            NOT NULL REFERENCES users(id) ON DELETE SET NULL,
    action                  TEXT            NOT NULL
                            CHECK (action IN ('submitted', 'approved', 'rejected', 'unlock', 'blocklist_override_approved', 'blocklist_override_rejected')),
    comments                TEXT            NOT NULL DEFAULT '',
    block_list_acknowledged BOOLEAN         NOT NULL DEFAULT false,
    block_list_justification TEXT,
    config_snapshot_json    JSONB,
    created_at              TIMESTAMPTZ     NOT NULL DEFAULT now()
);

CREATE INDEX idx_ca_campaign_submission ON campaign_approvals (campaign_id, submission_id);
CREATE INDEX idx_ca_actor ON campaign_approvals (actor_id);
CREATE INDEX idx_ca_campaign_action ON campaign_approvals (campaign_id, action);

-- Campaign approval requirements: tracks multi-approver progress per submission cycle.
CREATE TABLE campaign_approval_requirements (
    campaign_id              UUID        NOT NULL REFERENCES campaigns(id) ON DELETE CASCADE,
    submission_id            UUID        NOT NULL,
    required_approver_count  INTEGER     NOT NULL DEFAULT 1 CHECK (required_approver_count >= 1 AND required_approver_count <= 5),
    requires_admin_approval  BOOLEAN     NOT NULL DEFAULT false,
    current_approval_count   INTEGER     NOT NULL DEFAULT 0,
    created_at               TIMESTAMPTZ NOT NULL DEFAULT now(),

    PRIMARY KEY (campaign_id, submission_id)
);

-- Seed the campaigns:approve permission for Engineer and Administrator roles.
INSERT INTO permissions (resource_type, action, description) VALUES
    ('campaigns', 'approve', 'Approve or reject campaign submissions')
ON CONFLICT (resource_type, action) DO NOTHING;

-- Grant campaigns:approve to Engineer role.
INSERT INTO role_permissions (role_id, permission_id)
SELECT r.id, p.id
FROM roles r, permissions p
WHERE r.name = 'engineer'
  AND p.resource_type = 'campaigns'
  AND p.action = 'approve'
ON CONFLICT DO NOTHING;
