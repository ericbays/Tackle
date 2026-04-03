-- Migration 017: domain_profiles table.

-- Domain lifecycle status enum.
CREATE TYPE domain_status AS ENUM (
    'pending_registration',
    'active',
    'expired',
    'suspended',
    'decommissioned'
);

CREATE TABLE domain_profiles (
    id                          UUID         NOT NULL DEFAULT gen_random_uuid() PRIMARY KEY,
    domain_name                 VARCHAR(255) NOT NULL,
    registrar_connection_id     UUID         REFERENCES domain_provider_connections(id) ON DELETE SET NULL,
    dns_provider_connection_id  UUID         REFERENCES domain_provider_connections(id) ON DELETE SET NULL,
    status                      domain_status NOT NULL DEFAULT 'pending_registration',
    registration_date           DATE,
    expiry_date                 DATE,
    tags                        TEXT[]       NOT NULL DEFAULT '{}',
    notes                       TEXT,
    created_by                  UUID         NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
    created_at                  TIMESTAMPTZ  NOT NULL DEFAULT now(),
    updated_at                  TIMESTAMPTZ  NOT NULL DEFAULT now(),
    CONSTRAINT uq_domain_profiles_domain_name UNIQUE (domain_name)
);

CREATE INDEX idx_domain_profiles_status      ON domain_profiles (status);
CREATE INDEX idx_domain_profiles_expiry_date ON domain_profiles (expiry_date);
CREATE INDEX idx_domain_profiles_tags        ON domain_profiles USING GIN (tags);
CREATE INDEX idx_domain_profiles_registrar   ON domain_profiles (registrar_connection_id);
CREATE INDEX idx_domain_profiles_dns_provider ON domain_profiles (dns_provider_connection_id);

CREATE TRIGGER trg_domain_profiles_updated_at
    BEFORE UPDATE ON domain_profiles
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();
