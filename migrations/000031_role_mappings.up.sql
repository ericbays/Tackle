-- Migration 031: role_mappings table.
-- Maps external group names/claims from an auth provider to Tackle roles.

CREATE TABLE role_mappings (
    id                  UUID        NOT NULL DEFAULT gen_random_uuid() PRIMARY KEY,
    provider_config_id  UUID        NOT NULL REFERENCES auth_providers(id) ON DELETE CASCADE,
    external_group      TEXT        NOT NULL,
    role_id             UUID        NOT NULL REFERENCES roles(id) ON DELETE CASCADE,
    created_at          TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at          TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT uq_role_mappings_provider_group UNIQUE (provider_config_id, external_group)
);

CREATE TRIGGER trg_role_mappings_updated_at
    BEFORE UPDATE ON role_mappings
    FOR EACH ROW
    EXECUTE FUNCTION set_updated_at();

CREATE INDEX idx_role_mappings_provider_config_id ON role_mappings (provider_config_id);
