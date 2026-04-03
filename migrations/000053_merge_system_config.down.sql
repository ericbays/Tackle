-- Reverse migration 053: Re-create system_config from system_settings data.

CREATE TABLE IF NOT EXISTS system_config (
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

-- Copy relevant keys back.
INSERT INTO system_config (key, value, updated_at)
SELECT key, value::jsonb, updated_at FROM system_settings
WHERE key IN (
    'jwt_access_token_lifetime_minutes',
    'jwt_refresh_token_lifetime_days',
    'password_min_length',
    'password_require_uppercase',
    'password_require_lowercase',
    'password_require_digit',
    'password_require_special',
    'password_history_count',
    'login_rate_limit_per_ip',
    'login_rate_limit_per_account',
    'account_lockout_duration_minutes',
    'max_concurrent_sessions',
    'notification_retention_days',
    'idle_timeout_minutes'
)
ON CONFLICT (key) DO NOTHING;
