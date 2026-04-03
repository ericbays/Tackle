-- Migration 019: domain_campaign_associations table.
-- Note: campaigns table does not exist yet (Phase 3); campaign_id is a plain UUID with no FK.

CREATE TYPE domain_association_type AS ENUM ('sender_domain', 'phishing_url');

CREATE TABLE domain_campaign_associations (
    id                UUID                   NOT NULL DEFAULT gen_random_uuid() PRIMARY KEY,
    domain_profile_id UUID                   NOT NULL REFERENCES domain_profiles(id) ON DELETE CASCADE,
    campaign_id       UUID                   NOT NULL,
    association_type  domain_association_type NOT NULL,
    created_at        TIMESTAMPTZ            NOT NULL DEFAULT now()
);

CREATE INDEX idx_domain_campaign_assoc_domain   ON domain_campaign_associations (domain_profile_id);
CREATE INDEX idx_domain_campaign_assoc_campaign  ON domain_campaign_associations (campaign_id);
CREATE INDEX idx_domain_campaign_assoc_composite ON domain_campaign_associations (domain_profile_id, campaign_id);
