-- Rollback migration 055: Remove previous_checksum column from audit_logs.
ALTER TABLE audit_logs DROP COLUMN IF EXISTS previous_checksum;
