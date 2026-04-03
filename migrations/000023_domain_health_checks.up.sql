-- Migration 000023: domain_health_checks table
-- Stores per-domain health check results with full per-check detail.

CREATE TYPE domain_health_check_type AS ENUM (
    'full',
    'propagation_only',
    'blocklist_only',
    'email_auth_only',
    'mx_only'
);

CREATE TYPE domain_health_overall_status AS ENUM (
    'healthy',
    'warning',
    'critical'
);

CREATE TYPE domain_health_trigger AS ENUM (
    'manual',
    'scheduled',
    'dns_change'
);

CREATE TABLE domain_health_checks (
    id                  UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    domain_profile_id   UUID NOT NULL REFERENCES domain_profiles(id) ON DELETE CASCADE,
    check_type          domain_health_check_type NOT NULL DEFAULT 'full',
    overall_status      domain_health_overall_status NOT NULL,
    results_json        JSONB NOT NULL DEFAULT '{}',
    triggered_by        domain_health_trigger NOT NULL DEFAULT 'manual',
    created_at          TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_domain_health_checks_profile_created
    ON domain_health_checks (domain_profile_id, created_at DESC);

CREATE INDEX idx_domain_health_checks_status
    ON domain_health_checks (overall_status);
