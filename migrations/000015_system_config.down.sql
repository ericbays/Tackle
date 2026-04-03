-- Rollback 015.
DROP TRIGGER IF EXISTS trg_system_config_updated_at ON system_config;
DROP TABLE IF EXISTS system_config;
