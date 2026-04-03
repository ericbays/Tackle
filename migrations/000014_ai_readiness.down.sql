-- Rollback 014.
DROP INDEX IF EXISTS idx_ai_proposals_status;
DROP INDEX IF EXISTS idx_ai_proposals_provider_id;
DROP TRIGGER IF EXISTS trg_ai_proposals_updated_at ON ai_proposals;
DROP TABLE IF EXISTS ai_proposals;
DROP INDEX IF EXISTS idx_ai_providers_enabled;
DROP TRIGGER IF EXISTS trg_ai_providers_updated_at ON ai_providers;
DROP TABLE IF EXISTS ai_providers;
