-- Migration 000024: domain_categorizations table
-- Stores per-service URL categorization results for each domain.

CREATE TYPE domain_categorization_status AS ENUM (
    'categorized',
    'uncategorized',
    'flagged',
    'unknown'
);

CREATE TABLE domain_categorizations (
    id                  UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    domain_profile_id   UUID NOT NULL REFERENCES domain_profiles(id) ON DELETE CASCADE,
    service             VARCHAR(64) NOT NULL,
    category            VARCHAR(128) NOT NULL DEFAULT '',
    status              domain_categorization_status NOT NULL DEFAULT 'unknown',
    raw_response        TEXT,
    checked_at          TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_domain_categorizations_profile_service_checked
    ON domain_categorizations (domain_profile_id, service, checked_at DESC);
