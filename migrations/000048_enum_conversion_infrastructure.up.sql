-- Migration 048: Convert infrastructure ENUMs to TEXT + CHECK constraints.
-- Pattern: add TEXT column, copy data, set NOT NULL + default, drop ENUM column, rename TEXT column, add CHECK.

-- 1. provider_type on domain_provider_connections
ALTER TABLE domain_provider_connections ADD COLUMN provider_type_new TEXT;
UPDATE domain_provider_connections SET provider_type_new = provider_type::TEXT;
ALTER TABLE domain_provider_connections ALTER COLUMN provider_type_new SET NOT NULL;
ALTER TABLE domain_provider_connections DROP COLUMN provider_type;
ALTER TABLE domain_provider_connections RENAME COLUMN provider_type_new TO provider_type;
ALTER TABLE domain_provider_connections ADD CONSTRAINT chk_provider_type
    CHECK (provider_type IN ('namecheap', 'godaddy', 'route53', 'azure_dns'));
CREATE INDEX idx_dpc_provider_type ON domain_provider_connections (provider_type);

-- 2. provider_status (status column) on domain_provider_connections
ALTER TABLE domain_provider_connections ADD COLUMN status_new TEXT;
UPDATE domain_provider_connections SET status_new = status::TEXT;
ALTER TABLE domain_provider_connections ALTER COLUMN status_new SET NOT NULL;
ALTER TABLE domain_provider_connections ALTER COLUMN status_new SET DEFAULT 'untested';
ALTER TABLE domain_provider_connections DROP COLUMN status;
ALTER TABLE domain_provider_connections RENAME COLUMN status_new TO status;
ALTER TABLE domain_provider_connections ADD CONSTRAINT chk_provider_status
    CHECK (status IN ('untested', 'healthy', 'error'));

-- 3. domain_status on domain_profiles
DROP INDEX IF EXISTS idx_domain_profiles_status;
ALTER TABLE domain_profiles ADD COLUMN status_new TEXT;
UPDATE domain_profiles SET status_new = status::TEXT;
ALTER TABLE domain_profiles ALTER COLUMN status_new SET NOT NULL;
ALTER TABLE domain_profiles ALTER COLUMN status_new SET DEFAULT 'pending_registration';
ALTER TABLE domain_profiles DROP COLUMN status;
ALTER TABLE domain_profiles RENAME COLUMN status_new TO status;
ALTER TABLE domain_profiles ADD CONSTRAINT chk_domain_status
    CHECK (status IN ('pending_registration', 'active', 'expired', 'suspended', 'decommissioned'));
CREATE INDEX idx_domain_profiles_status ON domain_profiles (status);

-- 4. cloud_provider_type on phishing_endpoints
ALTER TABLE phishing_endpoints ADD COLUMN cloud_provider_new TEXT;
UPDATE phishing_endpoints SET cloud_provider_new = cloud_provider::TEXT;
ALTER TABLE phishing_endpoints DROP COLUMN cloud_provider;
ALTER TABLE phishing_endpoints RENAME COLUMN cloud_provider_new TO cloud_provider;
ALTER TABLE phishing_endpoints ADD CONSTRAINT chk_endpoint_cloud_provider
    CHECK (cloud_provider IN ('aws', 'azure', 'proxmox'));

-- 5. cloud_provider_type on cloud_credentials
ALTER TABLE cloud_credentials ADD COLUMN provider_type_new TEXT;
UPDATE cloud_credentials SET provider_type_new = provider_type::TEXT;
ALTER TABLE cloud_credentials ALTER COLUMN provider_type_new SET NOT NULL;
ALTER TABLE cloud_credentials DROP COLUMN provider_type;
ALTER TABLE cloud_credentials RENAME COLUMN provider_type_new TO provider_type;
ALTER TABLE cloud_credentials ADD CONSTRAINT chk_cloud_provider_type
    CHECK (provider_type IN ('aws', 'azure', 'proxmox'));

-- 6. cloud_credential_status on cloud_credentials
ALTER TABLE cloud_credentials ADD COLUMN status_new TEXT;
UPDATE cloud_credentials SET status_new = status::TEXT;
ALTER TABLE cloud_credentials ALTER COLUMN status_new SET NOT NULL;
ALTER TABLE cloud_credentials ALTER COLUMN status_new SET DEFAULT 'untested';
ALTER TABLE cloud_credentials DROP COLUMN status;
ALTER TABLE cloud_credentials RENAME COLUMN status_new TO status;
ALTER TABLE cloud_credentials ADD CONSTRAINT chk_cloud_credential_status
    CHECK (status IN ('untested', 'healthy', 'error'));

-- 7. endpoint_state on phishing_endpoints and endpoint_state_transitions
-- Drop indexes that reference the state ENUM column
DROP INDEX IF EXISTS idx_phishing_endpoints_state;
DROP INDEX IF EXISTS idx_phishing_endpoints_active;

-- Convert endpoint_state_transitions first (it references the same ENUM type)
ALTER TABLE endpoint_state_transitions ADD COLUMN from_state_new TEXT;
ALTER TABLE endpoint_state_transitions ADD COLUMN to_state_new TEXT;
UPDATE endpoint_state_transitions SET from_state_new = from_state::TEXT, to_state_new = to_state::TEXT;
ALTER TABLE endpoint_state_transitions ALTER COLUMN from_state_new SET NOT NULL;
ALTER TABLE endpoint_state_transitions ALTER COLUMN to_state_new SET NOT NULL;
ALTER TABLE endpoint_state_transitions DROP COLUMN from_state;
ALTER TABLE endpoint_state_transitions DROP COLUMN to_state;
ALTER TABLE endpoint_state_transitions RENAME COLUMN from_state_new TO from_state;
ALTER TABLE endpoint_state_transitions RENAME COLUMN to_state_new TO to_state;

-- Convert phishing_endpoints.state
ALTER TABLE phishing_endpoints ADD COLUMN state_new TEXT;
UPDATE phishing_endpoints SET state_new = state::TEXT;
ALTER TABLE phishing_endpoints ALTER COLUMN state_new SET NOT NULL;
ALTER TABLE phishing_endpoints ALTER COLUMN state_new SET DEFAULT 'requested';
ALTER TABLE phishing_endpoints DROP COLUMN state;
ALTER TABLE phishing_endpoints RENAME COLUMN state_new TO state;
ALTER TABLE phishing_endpoints ADD CONSTRAINT chk_endpoint_state
    CHECK (state IN ('requested', 'provisioning', 'configuring', 'active', 'running', 'stopped', 'stopping', 'terminating', 'error', 'terminated', 'unhealthy'));
CREATE INDEX idx_phishing_endpoints_state ON phishing_endpoints (state);
CREATE INDEX idx_phishing_endpoints_active ON phishing_endpoints (state) WHERE state NOT IN ('terminated', 'error');

-- 8. cloud_provider_type on instance_templates
DROP INDEX IF EXISTS idx_instance_templates_provider_type;
ALTER TABLE instance_templates ADD COLUMN provider_type_new TEXT;
UPDATE instance_templates SET provider_type_new = provider_type::TEXT;
ALTER TABLE instance_templates ALTER COLUMN provider_type_new SET NOT NULL;
ALTER TABLE instance_templates DROP COLUMN provider_type;
ALTER TABLE instance_templates RENAME COLUMN provider_type_new TO provider_type;
ALTER TABLE instance_templates ADD CONSTRAINT chk_instance_template_provider_type
    CHECK (provider_type IN ('aws', 'azure', 'proxmox'));
CREATE INDEX idx_instance_templates_provider_type ON instance_templates (provider_type);

-- Drop old ENUM types
DROP TYPE IF EXISTS provider_type;
DROP TYPE IF EXISTS provider_status;
DROP TYPE IF EXISTS domain_status;
DROP TYPE IF EXISTS cloud_provider_type;
DROP TYPE IF EXISTS cloud_credential_status;
DROP TYPE IF EXISTS endpoint_state;
