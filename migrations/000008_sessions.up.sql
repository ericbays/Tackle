-- Migration 008: sessions table (merges session + refresh token into one row).
-- Only hashes of tokens are stored; never plaintext (REQ-AUTH-027).

CREATE TABLE sessions (
    id                  UUID        NOT NULL DEFAULT gen_random_uuid() PRIMARY KEY,
    user_id             UUID        NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    token_hash          TEXT        NOT NULL UNIQUE,
    refresh_token_hash  TEXT        UNIQUE,
    ip_address          INET,
    user_agent          TEXT,
    expires_at          TIMESTAMPTZ NOT NULL,
    last_used_at        TIMESTAMPTZ,
    revoked             BOOLEAN     NOT NULL DEFAULT FALSE,
    created_at          TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_sessions_user_id            ON sessions (user_id);
CREATE UNIQUE INDEX idx_sessions_token_hash  ON sessions (token_hash);
CREATE UNIQUE INDEX idx_sessions_refresh_token_hash ON sessions (refresh_token_hash)
    WHERE refresh_token_hash IS NOT NULL;
CREATE INDEX idx_sessions_expires_at         ON sessions (expires_at);
