-- Rollback 010.
DROP INDEX IF EXISTS idx_api_keys_key_hash;
DROP INDEX IF EXISTS idx_api_keys_user_id;
DROP TRIGGER IF EXISTS trg_api_keys_updated_at ON api_keys;
DROP TABLE IF EXISTS api_keys;
