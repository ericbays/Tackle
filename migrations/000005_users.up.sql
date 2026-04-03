-- Migration 005: users table.

CREATE TABLE users (
    id                    UUID        NOT NULL DEFAULT gen_random_uuid() PRIMARY KEY,
    email                 TEXT        NOT NULL UNIQUE,
    username              TEXT        NOT NULL UNIQUE,
    password_hash         TEXT,
    display_name          TEXT        NOT NULL,
    is_initial_admin      BOOLEAN     NOT NULL DEFAULT FALSE,
    auth_provider         TEXT        NOT NULL DEFAULT 'local',
    external_id           TEXT,
    status                TEXT        NOT NULL DEFAULT 'active'
                              CHECK (status IN ('active', 'inactive', 'locked')),
    force_password_change BOOLEAN     NOT NULL DEFAULT FALSE,
    created_at            TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at            TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TRIGGER trg_users_updated_at
    BEFORE UPDATE ON users
    FOR EACH ROW
    EXECUTE FUNCTION set_updated_at();

CREATE UNIQUE INDEX idx_users_email    ON users (email);
CREATE UNIQUE INDEX idx_users_username ON users (username);
CREATE        INDEX idx_users_auth_provider ON users (auth_provider);
CREATE        INDEX idx_users_status ON users (status) WHERE status = 'active';
