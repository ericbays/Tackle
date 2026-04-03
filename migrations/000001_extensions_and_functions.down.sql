-- Rollback 001.
DROP FUNCTION IF EXISTS reject_modification();
DROP FUNCTION IF EXISTS set_updated_at();
-- Extensions are intentionally NOT dropped on rollback to avoid disrupting
-- other schemas or objects that may depend on them.
