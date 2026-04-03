-- Migration 009: password_history table.
-- Stores bcrypt hashes of previous passwords to enforce reuse policy (REQ-AUTH-021).

CREATE TABLE password_history (
    id            UUID        NOT NULL DEFAULT gen_random_uuid() PRIMARY KEY,
    user_id       UUID        NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    password_hash TEXT        NOT NULL,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_password_history_user_id ON password_history (user_id);
