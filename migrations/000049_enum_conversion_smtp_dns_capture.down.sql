-- Reverse migration 049: recreate SMTP/DNS/capture ENUMs and restore ENUM columns.

-- Recreate ENUM types
CREATE TYPE smtp_auth_type AS ENUM ('none', 'plain', 'login', 'cram_md5', 'xoauth2');
CREATE TYPE smtp_tls_mode AS ENUM ('none', 'starttls', 'tls');
CREATE TYPE smtp_profile_status AS ENUM ('untested', 'healthy', 'error');
CREATE TYPE domain_health_check_type AS ENUM ('full', 'propagation_only', 'blocklist_only', 'email_auth_only', 'mx_only');
CREATE TYPE domain_health_overall_status AS ENUM ('healthy', 'warning', 'critical');
CREATE TYPE domain_health_trigger AS ENUM ('manual', 'scheduled', 'dns_change');
CREATE TYPE domain_association_type AS ENUM ('sender_domain', 'phishing_url');
CREATE TYPE registration_request_status AS ENUM ('pending', 'approved', 'rejected');
CREATE TYPE domain_categorization_status AS ENUM ('categorized', 'uncategorized', 'flagged', 'unknown');
CREATE TYPE field_category AS ENUM ('identity', 'sensitive', 'mfa', 'custom', 'hidden');
CREATE TYPE session_data_type AS ENUM ('cookie', 'oauth_token', 'session_token', 'auth_header', 'local_storage', 'session_storage');
CREATE TYPE post_capture_action AS ENUM ('redirect', 'display_page', 'redirect_with_delay', 'replay_submission', 'no_action');

-- 13. Restore post_capture_action on landing_page_projects
ALTER TABLE landing_page_projects DROP CONSTRAINT IF EXISTS chk_post_capture_action;
ALTER TABLE landing_page_projects ADD COLUMN post_capture_action_old post_capture_action;
UPDATE landing_page_projects SET post_capture_action_old = post_capture_action::post_capture_action;
ALTER TABLE landing_page_projects ALTER COLUMN post_capture_action_old SET NOT NULL;
ALTER TABLE landing_page_projects ALTER COLUMN post_capture_action_old SET DEFAULT 'no_action';
ALTER TABLE landing_page_projects DROP COLUMN post_capture_action;
ALTER TABLE landing_page_projects RENAME COLUMN post_capture_action_old TO post_capture_action;

-- 12. Restore session_data_type on session_captures
ALTER TABLE session_captures DROP CONSTRAINT IF EXISTS chk_session_data_type;
ALTER TABLE session_captures ADD COLUMN data_type_old session_data_type;
UPDATE session_captures SET data_type_old = data_type::session_data_type;
ALTER TABLE session_captures ALTER COLUMN data_type_old SET NOT NULL;
ALTER TABLE session_captures DROP COLUMN data_type;
ALTER TABLE session_captures RENAME COLUMN data_type_old TO data_type;

-- 11. Restore field_category on field_categorization_rules
ALTER TABLE field_categorization_rules DROP CONSTRAINT IF EXISTS chk_rule_category;
ALTER TABLE field_categorization_rules ADD COLUMN category_old field_category;
UPDATE field_categorization_rules SET category_old = category::field_category;
ALTER TABLE field_categorization_rules ALTER COLUMN category_old SET NOT NULL;
ALTER TABLE field_categorization_rules DROP COLUMN category;
ALTER TABLE field_categorization_rules RENAME COLUMN category_old TO category;

-- 10. Restore field_category on capture_fields
ALTER TABLE capture_fields DROP CONSTRAINT IF EXISTS chk_field_category;
ALTER TABLE capture_fields ADD COLUMN field_category_old field_category;
UPDATE capture_fields SET field_category_old = field_category::field_category;
ALTER TABLE capture_fields ALTER COLUMN field_category_old SET NOT NULL;
ALTER TABLE capture_fields ALTER COLUMN field_category_old SET DEFAULT 'custom';
ALTER TABLE capture_fields DROP COLUMN field_category;
ALTER TABLE capture_fields RENAME COLUMN field_category_old TO field_category;

-- 9. Restore domain_categorization_status on domain_categorizations
ALTER TABLE domain_categorizations DROP CONSTRAINT IF EXISTS chk_categorization_status;
ALTER TABLE domain_categorizations ADD COLUMN status_old domain_categorization_status;
UPDATE domain_categorizations SET status_old = status::domain_categorization_status;
ALTER TABLE domain_categorizations ALTER COLUMN status_old SET NOT NULL;
ALTER TABLE domain_categorizations ALTER COLUMN status_old SET DEFAULT 'unknown';
ALTER TABLE domain_categorizations DROP COLUMN status;
ALTER TABLE domain_categorizations RENAME COLUMN status_old TO status;

-- 8. Restore registration_request_status on domain_registration_requests
ALTER TABLE domain_registration_requests DROP CONSTRAINT IF EXISTS chk_registration_status;
ALTER TABLE domain_registration_requests ADD COLUMN status_old registration_request_status;
UPDATE domain_registration_requests SET status_old = status::registration_request_status;
ALTER TABLE domain_registration_requests ALTER COLUMN status_old SET NOT NULL;
ALTER TABLE domain_registration_requests ALTER COLUMN status_old SET DEFAULT 'pending';
ALTER TABLE domain_registration_requests DROP COLUMN status;
ALTER TABLE domain_registration_requests RENAME COLUMN status_old TO status;

-- 7. Restore domain_association_type on domain_campaign_associations
ALTER TABLE domain_campaign_associations DROP CONSTRAINT IF EXISTS chk_domain_association_type;
ALTER TABLE domain_campaign_associations ADD COLUMN association_type_old domain_association_type;
UPDATE domain_campaign_associations SET association_type_old = association_type::domain_association_type;
ALTER TABLE domain_campaign_associations ALTER COLUMN association_type_old SET NOT NULL;
ALTER TABLE domain_campaign_associations DROP COLUMN association_type;
ALTER TABLE domain_campaign_associations RENAME COLUMN association_type_old TO association_type;

-- 6. Restore domain_health_trigger (triggered_by column) on domain_health_checks
ALTER TABLE domain_health_checks DROP CONSTRAINT IF EXISTS chk_health_trigger;
ALTER TABLE domain_health_checks ADD COLUMN triggered_by_old domain_health_trigger;
UPDATE domain_health_checks SET triggered_by_old = triggered_by::domain_health_trigger;
ALTER TABLE domain_health_checks ALTER COLUMN triggered_by_old SET NOT NULL;
ALTER TABLE domain_health_checks ALTER COLUMN triggered_by_old SET DEFAULT 'manual';
ALTER TABLE domain_health_checks DROP COLUMN triggered_by;
ALTER TABLE domain_health_checks RENAME COLUMN triggered_by_old TO triggered_by;

-- 5. Restore domain_health_overall_status on domain_health_checks
ALTER TABLE domain_health_checks DROP CONSTRAINT IF EXISTS chk_health_overall_status;
ALTER TABLE domain_health_checks ADD COLUMN overall_status_old domain_health_overall_status;
UPDATE domain_health_checks SET overall_status_old = overall_status::domain_health_overall_status;
ALTER TABLE domain_health_checks ALTER COLUMN overall_status_old SET NOT NULL;
ALTER TABLE domain_health_checks DROP COLUMN overall_status;
ALTER TABLE domain_health_checks RENAME COLUMN overall_status_old TO overall_status;

-- 4. Restore domain_health_check_type on domain_health_checks
ALTER TABLE domain_health_checks DROP CONSTRAINT IF EXISTS chk_health_check_type;
ALTER TABLE domain_health_checks ADD COLUMN check_type_old domain_health_check_type;
UPDATE domain_health_checks SET check_type_old = check_type::domain_health_check_type;
ALTER TABLE domain_health_checks ALTER COLUMN check_type_old SET NOT NULL;
ALTER TABLE domain_health_checks ALTER COLUMN check_type_old SET DEFAULT 'full';
ALTER TABLE domain_health_checks DROP COLUMN check_type;
ALTER TABLE domain_health_checks RENAME COLUMN check_type_old TO check_type;

-- 3. Restore smtp_profile_status on smtp_profiles
DROP INDEX IF EXISTS idx_smtp_profiles_status;
ALTER TABLE smtp_profiles DROP CONSTRAINT IF EXISTS chk_smtp_profile_status;
ALTER TABLE smtp_profiles ADD COLUMN status_old smtp_profile_status;
UPDATE smtp_profiles SET status_old = status::smtp_profile_status;
ALTER TABLE smtp_profiles ALTER COLUMN status_old SET NOT NULL;
ALTER TABLE smtp_profiles ALTER COLUMN status_old SET DEFAULT 'untested';
ALTER TABLE smtp_profiles DROP COLUMN status;
ALTER TABLE smtp_profiles RENAME COLUMN status_old TO status;
CREATE INDEX idx_smtp_profiles_status ON smtp_profiles (status);

-- 2. Restore smtp_tls_mode on smtp_profiles
ALTER TABLE smtp_profiles DROP CONSTRAINT IF EXISTS chk_smtp_tls_mode;
ALTER TABLE smtp_profiles ADD COLUMN tls_mode_old smtp_tls_mode;
UPDATE smtp_profiles SET tls_mode_old = tls_mode::smtp_tls_mode;
ALTER TABLE smtp_profiles ALTER COLUMN tls_mode_old SET NOT NULL;
ALTER TABLE smtp_profiles ALTER COLUMN tls_mode_old SET DEFAULT 'starttls';
ALTER TABLE smtp_profiles DROP COLUMN tls_mode;
ALTER TABLE smtp_profiles RENAME COLUMN tls_mode_old TO tls_mode;

-- 1. Restore smtp_auth_type on smtp_profiles
ALTER TABLE smtp_profiles DROP CONSTRAINT IF EXISTS chk_smtp_auth_type;
ALTER TABLE smtp_profiles ADD COLUMN auth_type_old smtp_auth_type;
UPDATE smtp_profiles SET auth_type_old = auth_type::smtp_auth_type;
ALTER TABLE smtp_profiles ALTER COLUMN auth_type_old SET NOT NULL;
ALTER TABLE smtp_profiles ALTER COLUMN auth_type_old SET DEFAULT 'plain';
ALTER TABLE smtp_profiles DROP COLUMN auth_type;
ALTER TABLE smtp_profiles RENAME COLUMN auth_type_old TO auth_type;
