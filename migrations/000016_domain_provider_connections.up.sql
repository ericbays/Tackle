-- Migration 016: domain_provider_connections table and infrastructure permissions.

-- Provider type enum.
CREATE TYPE provider_type AS ENUM ('namecheap', 'godaddy', 'route53', 'azure_dns');

-- Connection health status enum.
CREATE TYPE provider_status AS ENUM ('untested', 'healthy', 'error');

CREATE TABLE domain_provider_connections (
    id                   UUID         NOT NULL DEFAULT gen_random_uuid() PRIMARY KEY,
    provider_type        provider_type NOT NULL,
    display_name         VARCHAR(255) NOT NULL,
    credentials_encrypted BYTEA       NOT NULL,
    status               provider_status NOT NULL DEFAULT 'untested',
    status_message       TEXT,
    last_tested_at       TIMESTAMPTZ,
    created_by           UUID         NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
    created_at           TIMESTAMPTZ  NOT NULL DEFAULT now(),
    updated_at           TIMESTAMPTZ  NOT NULL DEFAULT now(),
    CONSTRAINT uq_domain_provider_display_name UNIQUE (display_name)
);

CREATE INDEX idx_domain_provider_connections_provider_type ON domain_provider_connections (provider_type);
CREATE INDEX idx_domain_provider_connections_created_by    ON domain_provider_connections (created_by);

-- Trigger to keep updated_at current.
CREATE TRIGGER trg_domain_provider_connections_updated_at
    BEFORE UPDATE ON domain_provider_connections
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();
