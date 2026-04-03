-- Reverse migration 045: remove soft deletes and restore original unique constraints.

-- smtp_profiles
DROP INDEX IF EXISTS idx_smtp_profiles_name_active;
ALTER TABLE smtp_profiles ADD CONSTRAINT uq_smtp_profiles_name UNIQUE (name);
DROP INDEX IF EXISTS idx_smtp_profiles_deleted_at;
ALTER TABLE smtp_profiles DROP COLUMN deleted_at;

-- domain_profiles
DROP INDEX IF EXISTS idx_domain_profiles_name_active;
ALTER TABLE domain_profiles ADD CONSTRAINT uq_domain_profiles_domain_name UNIQUE (domain_name);
DROP INDEX IF EXISTS idx_domain_profiles_deleted_at;
ALTER TABLE domain_profiles DROP COLUMN deleted_at;

-- target_groups
DROP INDEX IF EXISTS idx_target_groups_name_active;
CREATE UNIQUE INDEX idx_target_groups_name ON target_groups (LOWER(name));
DROP INDEX IF EXISTS idx_target_groups_deleted_at;
ALTER TABLE target_groups DROP COLUMN deleted_at;

-- email_templates
DROP INDEX IF EXISTS idx_email_templates_name_active;
ALTER TABLE email_templates ADD CONSTRAINT uq_email_templates_name UNIQUE (name);
DROP INDEX IF EXISTS idx_email_templates_deleted_at;
ALTER TABLE email_templates DROP COLUMN deleted_at;
