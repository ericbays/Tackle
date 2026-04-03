-- Migration 051: Create approval comments table for discussion threads on campaign approvals.

CREATE TABLE approval_comments (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    campaign_id     UUID NOT NULL REFERENCES campaigns(id) ON DELETE CASCADE,
    submission_id   UUID NOT NULL,
    author_id       UUID NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
    body            TEXT NOT NULL,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_approval_comments_campaign_submission ON approval_comments (campaign_id, submission_id);
CREATE INDEX idx_approval_comments_author ON approval_comments (author_id);
CREATE INDEX idx_approval_comments_created_at ON approval_comments (created_at);

CREATE TRIGGER trg_approval_comments_updated_at BEFORE UPDATE ON approval_comments
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();
