-- Migration 018: domain_renewal_history table.

CREATE TABLE domain_renewal_history (
    id                      UUID          NOT NULL DEFAULT gen_random_uuid() PRIMARY KEY,
    domain_profile_id       UUID          NOT NULL REFERENCES domain_profiles(id) ON DELETE CASCADE,
    renewal_date            DATE          NOT NULL,
    duration_years          INTEGER       NOT NULL CHECK (duration_years > 0),
    cost_amount             DECIMAL(10,2),
    cost_currency           VARCHAR(3),
    registrar_connection_id UUID          REFERENCES domain_provider_connections(id) ON DELETE SET NULL,
    initiated_by            UUID          REFERENCES users(id) ON DELETE SET NULL,
    created_at              TIMESTAMPTZ   NOT NULL DEFAULT now()
);

CREATE INDEX idx_domain_renewal_history_domain ON domain_renewal_history (domain_profile_id);
