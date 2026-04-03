-- Migration 049: Convert remaining ENUMs to TEXT + CHECK constraints.
-- SMTP, domain health, domain associations, registration, categorization, capture, landing page.
-- NOTE: campaign_smtp_strategies table does not exist — skipped.

-- 1. smtp_auth_type on smtp_profiles
ALTER TABLE smtp_profiles ADD COLUMN auth_type_new TEXT;
UPDATE smtp_profiles SET auth_type_new = auth_type::TEXT;
ALTER TABLE smtp_profiles ALTER COLUMN auth_type_new SET NOT NULL;
ALTER TABLE smtp_profiles ALTER COLUMN auth_type_new SET DEFAULT 'plain';
ALTER TABLE smtp_profiles DROP COLUMN auth_type;
ALTER TABLE smtp_profiles RENAME COLUMN auth_type_new TO auth_type;
ALTER TABLE smtp_profiles ADD CONSTRAINT chk_smtp_auth_type
    CHECK (auth_type IN ('none', 'plain', 'login', 'cram_md5', 'xoauth2'));

-- 2. smtp_tls_mode on smtp_profiles
ALTER TABLE smtp_profiles ADD COLUMN tls_mode_new TEXT;
UPDATE smtp_profiles SET tls_mode_new = tls_mode::TEXT;
ALTER TABLE smtp_profiles ALTER COLUMN tls_mode_new SET NOT NULL;
ALTER TABLE smtp_profiles ALTER COLUMN tls_mode_new SET DEFAULT 'starttls';
ALTER TABLE smtp_profiles DROP COLUMN tls_mode;
ALTER TABLE smtp_profiles RENAME COLUMN tls_mode_new TO tls_mode;
ALTER TABLE smtp_profiles ADD CONSTRAINT chk_smtp_tls_mode
    CHECK (tls_mode IN ('none', 'starttls', 'tls'));

-- 3. smtp_profile_status on smtp_profiles
DROP INDEX IF EXISTS idx_smtp_profiles_status;
ALTER TABLE smtp_profiles ADD COLUMN status_new TEXT;
UPDATE smtp_profiles SET status_new = status::TEXT;
ALTER TABLE smtp_profiles ALTER COLUMN status_new SET NOT NULL;
ALTER TABLE smtp_profiles ALTER COLUMN status_new SET DEFAULT 'untested';
ALTER TABLE smtp_profiles DROP COLUMN status;
ALTER TABLE smtp_profiles RENAME COLUMN status_new TO status;
ALTER TABLE smtp_profiles ADD CONSTRAINT chk_smtp_profile_status
    CHECK (status IN ('untested', 'healthy', 'error'));
CREATE INDEX idx_smtp_profiles_status ON smtp_profiles (status);

-- 4. domain_health_check_type on domain_health_checks
ALTER TABLE domain_health_checks ADD COLUMN check_type_new TEXT;
UPDATE domain_health_checks SET check_type_new = check_type::TEXT;
ALTER TABLE domain_health_checks ALTER COLUMN check_type_new SET NOT NULL;
ALTER TABLE domain_health_checks ALTER COLUMN check_type_new SET DEFAULT 'full';
ALTER TABLE domain_health_checks DROP COLUMN check_type;
ALTER TABLE domain_health_checks RENAME COLUMN check_type_new TO check_type;
ALTER TABLE domain_health_checks ADD CONSTRAINT chk_health_check_type
    CHECK (check_type IN ('full', 'propagation_only', 'blocklist_only', 'email_auth_only', 'mx_only',
                          'dns_resolution', 'whois_expiry', 'ssl_certificate', 'http_response', 'blacklist', 'dkim', 'spf', 'dmarc'));

-- 5. domain_health_overall_status on domain_health_checks
ALTER TABLE domain_health_checks ADD COLUMN overall_status_new TEXT;
UPDATE domain_health_checks SET overall_status_new = overall_status::TEXT;
ALTER TABLE domain_health_checks ALTER COLUMN overall_status_new SET NOT NULL;
ALTER TABLE domain_health_checks DROP COLUMN overall_status;
ALTER TABLE domain_health_checks RENAME COLUMN overall_status_new TO overall_status;
ALTER TABLE domain_health_checks ADD CONSTRAINT chk_health_overall_status
    CHECK (overall_status IN ('healthy', 'warning', 'critical', 'unknown'));

-- 6. domain_health_trigger (triggered_by column) on domain_health_checks
ALTER TABLE domain_health_checks ADD COLUMN triggered_by_new TEXT;
UPDATE domain_health_checks SET triggered_by_new = triggered_by::TEXT;
ALTER TABLE domain_health_checks ALTER COLUMN triggered_by_new SET NOT NULL;
ALTER TABLE domain_health_checks ALTER COLUMN triggered_by_new SET DEFAULT 'manual';
ALTER TABLE domain_health_checks DROP COLUMN triggered_by;
ALTER TABLE domain_health_checks RENAME COLUMN triggered_by_new TO triggered_by;
ALTER TABLE domain_health_checks ADD CONSTRAINT chk_health_trigger
    CHECK (triggered_by IN ('manual', 'scheduled', 'dns_change', 'event'));

-- 7. domain_association_type on domain_campaign_associations
ALTER TABLE domain_campaign_associations ADD COLUMN association_type_new TEXT;
UPDATE domain_campaign_associations SET association_type_new = association_type::TEXT;
ALTER TABLE domain_campaign_associations ALTER COLUMN association_type_new SET NOT NULL;
ALTER TABLE domain_campaign_associations DROP COLUMN association_type;
ALTER TABLE domain_campaign_associations RENAME COLUMN association_type_new TO association_type;
ALTER TABLE domain_campaign_associations ADD CONSTRAINT chk_domain_association_type
    CHECK (association_type IN ('sender_domain', 'phishing_url', 'registrar', 'dns_provider'));

-- 8. registration_request_status on domain_registration_requests
ALTER TABLE domain_registration_requests ADD COLUMN status_new TEXT;
UPDATE domain_registration_requests SET status_new = status::TEXT;
ALTER TABLE domain_registration_requests ALTER COLUMN status_new SET NOT NULL;
ALTER TABLE domain_registration_requests ALTER COLUMN status_new SET DEFAULT 'pending';
ALTER TABLE domain_registration_requests DROP COLUMN status;
ALTER TABLE domain_registration_requests RENAME COLUMN status_new TO status;
ALTER TABLE domain_registration_requests ADD CONSTRAINT chk_registration_status
    CHECK (status IN ('pending', 'approved', 'rejected', 'submitted', 'processing', 'completed', 'failed', 'cancelled'));

-- 9. domain_categorization_status on domain_categorizations
ALTER TABLE domain_categorizations ADD COLUMN status_new TEXT;
UPDATE domain_categorizations SET status_new = status::TEXT;
ALTER TABLE domain_categorizations ALTER COLUMN status_new SET NOT NULL;
ALTER TABLE domain_categorizations ALTER COLUMN status_new SET DEFAULT 'unknown';
ALTER TABLE domain_categorizations DROP COLUMN status;
ALTER TABLE domain_categorizations RENAME COLUMN status_new TO status;
ALTER TABLE domain_categorizations ADD CONSTRAINT chk_categorization_status
    CHECK (status IN ('categorized', 'uncategorized', 'flagged', 'unknown', 'pending', 'submitted', 'failed', 'recategorization_needed'));

-- 10. field_category on capture_fields
ALTER TABLE capture_fields ADD COLUMN field_category_new TEXT;
UPDATE capture_fields SET field_category_new = field_category::TEXT;
ALTER TABLE capture_fields ALTER COLUMN field_category_new SET NOT NULL;
ALTER TABLE capture_fields ALTER COLUMN field_category_new SET DEFAULT 'custom';
ALTER TABLE capture_fields DROP COLUMN field_category;
ALTER TABLE capture_fields RENAME COLUMN field_category_new TO field_category;
ALTER TABLE capture_fields ADD CONSTRAINT chk_field_category
    CHECK (field_category IN ('identity', 'sensitive', 'mfa', 'custom', 'hidden'));

-- 11. field_category on field_categorization_rules
ALTER TABLE field_categorization_rules ADD COLUMN category_new TEXT;
UPDATE field_categorization_rules SET category_new = category::TEXT;
ALTER TABLE field_categorization_rules ALTER COLUMN category_new SET NOT NULL;
ALTER TABLE field_categorization_rules DROP COLUMN category;
ALTER TABLE field_categorization_rules RENAME COLUMN category_new TO category;
ALTER TABLE field_categorization_rules ADD CONSTRAINT chk_rule_category
    CHECK (category IN ('identity', 'sensitive', 'mfa', 'custom', 'hidden'));

-- 12. session_data_type on session_captures
ALTER TABLE session_captures ADD COLUMN data_type_new TEXT;
UPDATE session_captures SET data_type_new = data_type::TEXT;
ALTER TABLE session_captures ALTER COLUMN data_type_new SET NOT NULL;
ALTER TABLE session_captures DROP COLUMN data_type;
ALTER TABLE session_captures RENAME COLUMN data_type_new TO data_type;
ALTER TABLE session_captures ADD CONSTRAINT chk_session_data_type
    CHECK (data_type IN ('cookie', 'oauth_token', 'session_token', 'auth_header', 'local_storage', 'session_storage'));

-- 13. post_capture_action on landing_page_projects
ALTER TABLE landing_page_projects ADD COLUMN post_capture_action_new TEXT;
UPDATE landing_page_projects SET post_capture_action_new = post_capture_action::TEXT;
ALTER TABLE landing_page_projects ALTER COLUMN post_capture_action_new SET NOT NULL;
ALTER TABLE landing_page_projects ALTER COLUMN post_capture_action_new SET DEFAULT 'no_action';
ALTER TABLE landing_page_projects DROP COLUMN post_capture_action;
ALTER TABLE landing_page_projects RENAME COLUMN post_capture_action_new TO post_capture_action;
ALTER TABLE landing_page_projects ADD CONSTRAINT chk_post_capture_action
    CHECK (post_capture_action IN ('redirect', 'display_page', 'redirect_with_delay', 'replay_submission', 'no_action'));

-- Drop old ENUM types
DROP TYPE IF EXISTS smtp_auth_type;
DROP TYPE IF EXISTS smtp_tls_mode;
DROP TYPE IF EXISTS smtp_profile_status;
DROP TYPE IF EXISTS domain_health_check_type;
DROP TYPE IF EXISTS domain_health_overall_status;
DROP TYPE IF EXISTS domain_health_trigger;
DROP TYPE IF EXISTS domain_association_type;
DROP TYPE IF EXISTS registration_request_status;
DROP TYPE IF EXISTS domain_categorization_status;
DROP TYPE IF EXISTS field_category;
DROP TYPE IF EXISTS session_data_type;
DROP TYPE IF EXISTS post_capture_action;
