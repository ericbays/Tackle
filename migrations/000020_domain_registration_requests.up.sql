-- Migration 020: domain_registration_requests table (approval-gated registration workflow).

CREATE TYPE registration_request_status AS ENUM ('pending', 'approved', 'rejected');

CREATE TABLE domain_registration_requests (
    id                      UUID                        NOT NULL DEFAULT gen_random_uuid() PRIMARY KEY,
    domain_name             VARCHAR(255)                NOT NULL,
    registrar_connection_id UUID                        NOT NULL REFERENCES domain_provider_connections(id) ON DELETE RESTRICT,
    years                   INTEGER                     NOT NULL DEFAULT 1 CHECK (years > 0),
    registrant_info         JSONB                       NOT NULL DEFAULT '{}',
    status                  registration_request_status NOT NULL DEFAULT 'pending',
    requested_by            UUID                        NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
    reviewed_by             UUID                        REFERENCES users(id) ON DELETE SET NULL,
    reviewed_at             TIMESTAMPTZ,
    rejection_reason        TEXT,
    domain_profile_id       UUID                        REFERENCES domain_profiles(id) ON DELETE SET NULL,
    created_at              TIMESTAMPTZ                 NOT NULL DEFAULT now(),
    updated_at              TIMESTAMPTZ                 NOT NULL DEFAULT now()
);

CREATE INDEX idx_domain_reg_requests_status       ON domain_registration_requests (status);
CREATE INDEX idx_domain_reg_requests_requested_by ON domain_registration_requests (requested_by);

CREATE TRIGGER trg_domain_registration_requests_updated_at
    BEFORE UPDATE ON domain_registration_requests
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();
