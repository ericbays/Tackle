-- Migration 010: api_keys table.
-- Only the SHA-256 hash of the full key is stored (REQ-AUTH-027).
-- key_prefix stores the first 8 characters for UI display.

CREATE TABLE api_keys (
    id           UUID        NOT NULL DEFAULT gen_random_uuid() PRIMARY KEY,
    user_id      UUID        NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    name         TEXT        NOT NULL,
    key_prefix   TEXT        NOT NULL,
    key_hash     TEXT        NOT NULL UNIQUE,
    expires_at   TIMESTAMPTZ,
    last_used_at TIMESTAMPTZ,
    revoked      BOOLEAN     NOT NULL DEFAULT FALSE,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at   TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TRIGGER trg_api_keys_updated_at
    BEFORE UPDATE ON api_keys
    FOR EACH ROW
    EXECUTE FUNCTION set_updated_at();

CREATE INDEX idx_api_keys_user_id  ON api_keys (user_id);
CREATE UNIQUE INDEX idx_api_keys_key_hash ON api_keys (key_hash);
