-- Migration 027: smtp_profiles table.

-- SMTP auth type enum.
CREATE TYPE smtp_auth_type AS ENUM ('none', 'plain', 'login', 'cram_md5', 'xoauth2');

-- SMTP TLS mode enum.
CREATE TYPE smtp_tls_mode AS ENUM ('none', 'starttls', 'tls');

-- SMTP profile status enum.
CREATE TYPE smtp_profile_status AS ENUM ('untested', 'healthy', 'error');

CREATE TABLE smtp_profiles (
    id                  UUID                  NOT NULL DEFAULT gen_random_uuid() PRIMARY KEY,
    name                VARCHAR(255)          NOT NULL,
    description         TEXT,
    host                VARCHAR(255)          NOT NULL,
    port                INTEGER               NOT NULL,
    auth_type           smtp_auth_type        NOT NULL DEFAULT 'plain',
    username_encrypted  BYTEA,
    password_encrypted  BYTEA,
    tls_mode            smtp_tls_mode         NOT NULL DEFAULT 'starttls',
    tls_skip_verify     BOOLEAN               NOT NULL DEFAULT FALSE,
    from_address        VARCHAR(255)          NOT NULL,
    from_name           VARCHAR(255),
    reply_to            VARCHAR(255),
    custom_helo         VARCHAR(255),
    max_send_rate       INTEGER,
    max_connections     INTEGER               NOT NULL DEFAULT 5,
    timeout_connect     INTEGER               NOT NULL DEFAULT 30,
    timeout_send        INTEGER               NOT NULL DEFAULT 60,
    status              smtp_profile_status   NOT NULL DEFAULT 'untested',
    status_message      TEXT,
    last_tested_at      TIMESTAMPTZ,
    created_by          UUID                  NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
    created_at          TIMESTAMPTZ           NOT NULL DEFAULT now(),
    updated_at          TIMESTAMPTZ           NOT NULL DEFAULT now(),
    CONSTRAINT uq_smtp_profiles_name UNIQUE (name)
);

CREATE INDEX idx_smtp_profiles_status ON smtp_profiles (status);

-- Trigger to keep updated_at current.
CREATE TRIGGER trg_smtp_profiles_updated_at
    BEFORE UPDATE ON smtp_profiles
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();
