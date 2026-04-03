-- Rollback 009.
DROP INDEX IF EXISTS idx_password_history_user_id;
DROP TABLE IF EXISTS password_history;
