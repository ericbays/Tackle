-- Migration 053: Merge system_config into system_settings and drop system_config.
-- Creates system_settings table if it doesn't exist, copies data from system_config.

CREATE TABLE IF NOT EXISTS system_settings (
    id          UUID        NOT NULL DEFAULT gen_random_uuid() PRIMARY KEY,
    key         TEXT        NOT NULL UNIQUE,
    value       TEXT        NOT NULL,
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- Copy system_config entries into system_settings (skip duplicates).
INSERT INTO system_settings (key, value, updated_at)
SELECT key, value::text, updated_at FROM system_config
ON CONFLICT (key) DO NOTHING;

-- Drop system_config.
DROP TABLE IF EXISTS system_config;
