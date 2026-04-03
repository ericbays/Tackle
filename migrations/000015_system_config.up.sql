-- Migration 015: system_config table for runtime configuration key-value store.

CREATE TABLE system_config (
    id          UUID        NOT NULL DEFAULT gen_random_uuid() PRIMARY KEY,
    key         TEXT        NOT NULL UNIQUE,
    value       JSONB       NOT NULL,
    description TEXT,
    updated_by  UUID        REFERENCES users(id) ON DELETE SET NULL,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TRIGGER trg_system_config_updated_at
    BEFORE UPDATE ON system_config
    FOR EACH ROW
    EXECUTE FUNCTION set_updated_at();

-- Seed default configuration values.
INSERT INTO system_config (key, value, description) VALUES
    ('jwt_access_token_lifetime_minutes', '15',    'JWT access token lifetime in minutes (REQ-AUTH-070).'),
    ('jwt_refresh_token_lifetime_days',   '7',     'JWT refresh token lifetime in days (REQ-AUTH-071).'),
    ('password_min_length',               '12',    'Minimum password length (REQ-AUTH-020).'),
    ('password_require_uppercase',        'true',  'Password must contain at least one uppercase letter.'),
    ('password_require_lowercase',        'true',  'Password must contain at least one lowercase letter.'),
    ('password_require_digit',            'true',  'Password must contain at least one digit.'),
    ('password_require_special',          'true',  'Password must contain at least one special character.'),
    ('password_history_count',            '5',     'Number of previous passwords to check against reuse (REQ-AUTH-021).'),
    ('login_rate_limit_per_ip',           '10',    'Max login attempts per IP before rate limiting (REQ-AUTH-025).'),
    ('login_rate_limit_per_account',      '5',     'Max failed login attempts per account before lockout.'),
    ('account_lockout_duration_minutes',  '15',    'Duration in minutes an account is locked after failed attempts.'),
    ('max_concurrent_sessions',           '0',     'Maximum concurrent sessions per user; 0 = unlimited (REQ-AUTH-073).'),
    ('notification_retention_days',       '90',    'Days to retain notifications before purging (REQ-NOTIF-002).');
