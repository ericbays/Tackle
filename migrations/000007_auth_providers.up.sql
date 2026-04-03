-- Migration 007: auth_providers table.
-- The configuration column stores AES-256-GCM encrypted JSONB (BYTEA).
-- Application code is responsible for encryption/decryption.

CREATE TABLE auth_providers (
    id            UUID        NOT NULL DEFAULT gen_random_uuid() PRIMARY KEY,
    type          TEXT        NOT NULL
                      CHECK (type IN ('local', 'oidc', 'fusionauth', 'ldap')),
    name          TEXT        NOT NULL UNIQUE,
    configuration BYTEA       NOT NULL,
    enabled       BOOLEAN     NOT NULL DEFAULT TRUE,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at    TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TRIGGER trg_auth_providers_updated_at
    BEFORE UPDATE ON auth_providers
    FOR EACH ROW
    EXECUTE FUNCTION set_updated_at();

CREATE INDEX idx_auth_providers_type    ON auth_providers (type);
CREATE INDEX idx_auth_providers_enabled ON auth_providers (enabled) WHERE enabled = TRUE;
