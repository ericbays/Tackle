-- Migration 055: Add previous_checksum column to audit_logs for HMAC chain integrity.
-- Each entry's HMAC now includes the previous entry's checksum, creating a
-- tamper-evident hash chain.

ALTER TABLE audit_logs ADD COLUMN IF NOT EXISTS previous_checksum TEXT;

COMMENT ON COLUMN audit_logs.previous_checksum IS 'HMAC of the previous audit log entry, forming a tamper-evident chain';
