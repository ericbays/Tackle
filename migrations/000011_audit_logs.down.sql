-- Rollback 011.
DROP INDEX IF EXISTS idx_audit_logs_details_gin;
DROP INDEX IF EXISTS idx_audit_logs_campaign_id;
DROP INDEX IF EXISTS idx_audit_logs_action;
DROP INDEX IF EXISTS idx_audit_logs_severity;
DROP INDEX IF EXISTS idx_audit_logs_correlation_id;
DROP INDEX IF EXISTS idx_audit_logs_resource;
DROP INDEX IF EXISTS idx_audit_logs_actor_id;
DROP INDEX IF EXISTS idx_audit_logs_category;
DROP INDEX IF EXISTS idx_audit_logs_timestamp;

-- Drop partitions first, then the parent.
DROP TABLE IF EXISTS audit_logs_2026_06;
DROP TABLE IF EXISTS audit_logs_2026_05;
DROP TABLE IF EXISTS audit_logs_2026_04;
DROP TABLE IF EXISTS audit_logs_2026_03;
DROP TABLE IF EXISTS audit_logs;
