-- Migration 054: campaign_shares table for campaign ownership sharing.
-- Operators can only see campaigns they created or that were shared with them.

CREATE TABLE campaign_shares (
    campaign_id UUID NOT NULL REFERENCES campaigns(id) ON DELETE CASCADE,
    user_id     UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    shared_by   UUID NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
    shared_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (campaign_id, user_id)
);

CREATE INDEX idx_campaign_shares_user ON campaign_shares (user_id);
