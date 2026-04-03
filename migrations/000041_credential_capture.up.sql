-- 000041: Credential capture tables for form submissions, field storage, and session capture.

-- Field category enum.
CREATE TYPE field_category AS ENUM ('identity', 'sensitive', 'mfa', 'custom', 'hidden');

-- Session capture data type enum.
CREATE TYPE session_data_type AS ENUM (
    'cookie', 'oauth_token', 'session_token',
    'auth_header', 'local_storage', 'session_storage'
);

-- Post-capture action type enum.
CREATE TYPE post_capture_action AS ENUM (
    'redirect', 'display_page', 'redirect_with_delay',
    'replay_submission', 'no_action'
);

-- Capture events: one row per form submission.
CREATE TABLE capture_events (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    campaign_id UUID NOT NULL REFERENCES campaigns(id) ON DELETE RESTRICT,
    target_id UUID REFERENCES targets(id) ON DELETE SET NULL,
    template_variant_id UUID,
    endpoint_id UUID REFERENCES phishing_endpoints(id) ON DELETE SET NULL,
    email_send_id UUID,
    tracking_token VARCHAR(255),
    source_ip INET,
    user_agent TEXT,
    accept_language VARCHAR(255),
    referer VARCHAR(2048),
    url_path VARCHAR(2048),
    http_method VARCHAR(10) DEFAULT 'POST',
    submission_sequence INT NOT NULL DEFAULT 1,
    is_unattributed BOOLEAN NOT NULL DEFAULT false,
    is_canary BOOLEAN NOT NULL DEFAULT false,
    captured_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_capture_events_campaign_id ON capture_events(campaign_id);
CREATE INDEX idx_capture_events_target_id ON capture_events(target_id);
CREATE INDEX idx_capture_events_captured_at ON capture_events(captured_at);
CREATE INDEX idx_capture_events_tracking_token ON capture_events(tracking_token);
CREATE INDEX idx_capture_events_campaign_target ON capture_events(campaign_id, target_id);

-- Capture fields: individual form field values (encrypted).
CREATE TABLE capture_fields (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    capture_event_id UUID NOT NULL REFERENCES capture_events(id) ON DELETE CASCADE,
    field_name VARCHAR(255) NOT NULL,
    field_value_encrypted BYTEA NOT NULL,
    field_category field_category NOT NULL DEFAULT 'custom',
    encryption_key_version INT NOT NULL DEFAULT 1,
    iv BYTEA NOT NULL
);

CREATE INDEX idx_capture_fields_event_id ON capture_fields(capture_event_id);

-- Session captures: cookies, OAuth tokens, session tokens, etc.
CREATE TABLE session_captures (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    capture_event_id UUID NOT NULL REFERENCES capture_events(id) ON DELETE CASCADE,
    data_type session_data_type NOT NULL,
    key_encrypted BYTEA NOT NULL,
    value_encrypted BYTEA NOT NULL,
    metadata JSONB,
    captured_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    is_time_sensitive BOOLEAN NOT NULL DEFAULT false
);

CREATE INDEX idx_session_captures_event_id ON session_captures(capture_event_id);

-- Field categorization defaults: configurable per landing page.
CREATE TABLE field_categorization_rules (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    landing_page_id UUID REFERENCES landing_page_projects(id) ON DELETE CASCADE,
    field_pattern VARCHAR(255) NOT NULL,
    category field_category NOT NULL,
    is_default BOOLEAN NOT NULL DEFAULT false,
    priority INT NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_field_cat_rules_landing_page ON field_categorization_rules(landing_page_id);

-- Insert default field categorization rules (global, no landing page).
INSERT INTO field_categorization_rules (id, landing_page_id, field_pattern, category, is_default, priority) VALUES
    (gen_random_uuid(), NULL, 'password', 'sensitive', true, 100),
    (gen_random_uuid(), NULL, 'passwd', 'sensitive', true, 100),
    (gen_random_uuid(), NULL, 'pass', 'sensitive', true, 90),
    (gen_random_uuid(), NULL, 'secret', 'sensitive', true, 90),
    (gen_random_uuid(), NULL, 'pin', 'sensitive', true, 90),
    (gen_random_uuid(), NULL, 'token', 'sensitive', true, 80),
    (gen_random_uuid(), NULL, 'otp', 'mfa', true, 100),
    (gen_random_uuid(), NULL, 'mfa', 'mfa', true, 100),
    (gen_random_uuid(), NULL, 'totp', 'mfa', true, 100),
    (gen_random_uuid(), NULL, '2fa', 'mfa', true, 100),
    (gen_random_uuid(), NULL, 'verification_code', 'mfa', true, 90),
    (gen_random_uuid(), NULL, 'auth_code', 'mfa', true, 90),
    (gen_random_uuid(), NULL, 'email', 'identity', true, 100),
    (gen_random_uuid(), NULL, 'username', 'identity', true, 100),
    (gen_random_uuid(), NULL, 'user', 'identity', true, 80),
    (gen_random_uuid(), NULL, 'login', 'identity', true, 80),
    (gen_random_uuid(), NULL, 'name', 'identity', true, 70),
    (gen_random_uuid(), NULL, 'phone', 'identity', true, 70),
    (gen_random_uuid(), NULL, 'account', 'identity', true, 70);

-- Add post-capture action config to landing pages.
ALTER TABLE landing_page_projects ADD COLUMN IF NOT EXISTS post_capture_action post_capture_action NOT NULL DEFAULT 'no_action';
ALTER TABLE landing_page_projects ADD COLUMN IF NOT EXISTS post_capture_config JSONB;
ALTER TABLE landing_page_projects ADD COLUMN IF NOT EXISTS session_capture_enabled BOOLEAN NOT NULL DEFAULT false;
ALTER TABLE landing_page_projects ADD COLUMN IF NOT EXISTS session_capture_scope JSONB;

-- Data retention configuration per organization (stored in system_config).
-- Handled via system_config table, no new table needed.
