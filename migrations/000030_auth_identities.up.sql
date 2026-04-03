-- Migration 030: auth_identities table.
-- Links external provider identities to local user accounts.
-- A user may have multiple linked identities (one per provider config + external_subject pair).

CREATE TABLE auth_identities (
    id                  UUID        NOT NULL DEFAULT gen_random_uuid() PRIMARY KEY,
    user_id             UUID        NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    provider_type       TEXT        NOT NULL CHECK (provider_type IN ('oidc', 'fusionauth', 'ldap')),
    provider_config_id  UUID        NOT NULL REFERENCES auth_providers(id) ON DELETE CASCADE,
    external_subject    TEXT        NOT NULL,
    external_email      TEXT,
    created_at          TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT uq_auth_identities_provider_subject UNIQUE (provider_config_id, external_subject)
);

CREATE INDEX idx_auth_identities_user_id           ON auth_identities (user_id);
CREATE INDEX idx_auth_identities_provider_config_id ON auth_identities (provider_config_id);
