-- Migration 045: Add soft deletes to email_templates, target_groups, domain_profiles, smtp_profiles.
-- Each table gets a deleted_at column, a partial index for filtering, and
-- the existing unique name constraint is replaced with a partial unique index
-- so that names are unique only among active (non-deleted) records.

-- email_templates: add soft delete
ALTER TABLE email_templates ADD COLUMN deleted_at TIMESTAMPTZ;
CREATE INDEX idx_email_templates_deleted_at ON email_templates (deleted_at) WHERE deleted_at IS NULL;
ALTER TABLE email_templates DROP CONSTRAINT uq_email_templates_name;
CREATE UNIQUE INDEX idx_email_templates_name_active ON email_templates (name) WHERE deleted_at IS NULL;

-- target_groups: add soft delete
ALTER TABLE target_groups ADD COLUMN deleted_at TIMESTAMPTZ;
CREATE INDEX idx_target_groups_deleted_at ON target_groups (deleted_at) WHERE deleted_at IS NULL;
DROP INDEX IF EXISTS idx_target_groups_name;
CREATE UNIQUE INDEX idx_target_groups_name_active ON target_groups (LOWER(name)) WHERE deleted_at IS NULL;

-- domain_profiles: add soft delete
ALTER TABLE domain_profiles ADD COLUMN deleted_at TIMESTAMPTZ;
CREATE INDEX idx_domain_profiles_deleted_at ON domain_profiles (deleted_at) WHERE deleted_at IS NULL;
ALTER TABLE domain_profiles DROP CONSTRAINT uq_domain_profiles_domain_name;
CREATE UNIQUE INDEX idx_domain_profiles_name_active ON domain_profiles (domain_name) WHERE deleted_at IS NULL;

-- smtp_profiles: add soft delete
ALTER TABLE smtp_profiles ADD COLUMN deleted_at TIMESTAMPTZ;
CREATE INDEX idx_smtp_profiles_deleted_at ON smtp_profiles (deleted_at) WHERE deleted_at IS NULL;
ALTER TABLE smtp_profiles DROP CONSTRAINT uq_smtp_profiles_name;
CREATE UNIQUE INDEX idx_smtp_profiles_name_active ON smtp_profiles (name) WHERE deleted_at IS NULL;
