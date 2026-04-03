-- Rollback 007.
DROP INDEX IF EXISTS idx_auth_providers_enabled;
DROP INDEX IF EXISTS idx_auth_providers_type;
DROP TRIGGER IF EXISTS trg_auth_providers_updated_at ON auth_providers;
DROP TABLE IF EXISTS auth_providers;
