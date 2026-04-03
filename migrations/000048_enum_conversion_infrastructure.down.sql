-- Reverse migration 048: recreate infrastructure ENUMs and restore ENUM columns.

-- Recreate ENUM types
CREATE TYPE provider_type AS ENUM ('namecheap', 'godaddy', 'route53', 'azure_dns');
CREATE TYPE provider_status AS ENUM ('untested', 'healthy', 'error');
CREATE TYPE domain_status AS ENUM ('pending_registration', 'active', 'expired', 'suspended', 'decommissioned');
CREATE TYPE cloud_provider_type AS ENUM ('aws', 'azure', 'proxmox');
CREATE TYPE cloud_credential_status AS ENUM ('untested', 'healthy', 'error');
CREATE TYPE endpoint_state AS ENUM ('requested', 'provisioning', 'configuring', 'active', 'stopped', 'error', 'terminated');

-- 7. Restore endpoint_state on phishing_endpoints and endpoint_state_transitions
DROP INDEX IF EXISTS idx_phishing_endpoints_active;
DROP INDEX IF EXISTS idx_phishing_endpoints_state;
ALTER TABLE phishing_endpoints DROP CONSTRAINT IF EXISTS chk_endpoint_state;

ALTER TABLE phishing_endpoints ADD COLUMN state_old endpoint_state;
UPDATE phishing_endpoints SET state_old = state::endpoint_state;
ALTER TABLE phishing_endpoints ALTER COLUMN state_old SET NOT NULL;
ALTER TABLE phishing_endpoints ALTER COLUMN state_old SET DEFAULT 'requested';
ALTER TABLE phishing_endpoints DROP COLUMN state;
ALTER TABLE phishing_endpoints RENAME COLUMN state_old TO state;
CREATE INDEX idx_phishing_endpoints_state ON phishing_endpoints (state);

ALTER TABLE endpoint_state_transitions ADD COLUMN from_state_old endpoint_state;
ALTER TABLE endpoint_state_transitions ADD COLUMN to_state_old endpoint_state;
UPDATE endpoint_state_transitions SET from_state_old = from_state::endpoint_state, to_state_old = to_state::endpoint_state;
ALTER TABLE endpoint_state_transitions ALTER COLUMN from_state_old SET NOT NULL;
ALTER TABLE endpoint_state_transitions ALTER COLUMN to_state_old SET NOT NULL;
ALTER TABLE endpoint_state_transitions DROP COLUMN from_state;
ALTER TABLE endpoint_state_transitions DROP COLUMN to_state;
ALTER TABLE endpoint_state_transitions RENAME COLUMN from_state_old TO from_state;
ALTER TABLE endpoint_state_transitions RENAME COLUMN to_state_old TO to_state;

-- 6. Restore cloud_credential_status on cloud_credentials
ALTER TABLE cloud_credentials DROP CONSTRAINT IF EXISTS chk_cloud_credential_status;
ALTER TABLE cloud_credentials ADD COLUMN status_old cloud_credential_status;
UPDATE cloud_credentials SET status_old = status::cloud_credential_status;
ALTER TABLE cloud_credentials ALTER COLUMN status_old SET NOT NULL;
ALTER TABLE cloud_credentials ALTER COLUMN status_old SET DEFAULT 'untested';
ALTER TABLE cloud_credentials DROP COLUMN status;
ALTER TABLE cloud_credentials RENAME COLUMN status_old TO status;

-- 5. Restore cloud_provider_type on cloud_credentials
ALTER TABLE cloud_credentials DROP CONSTRAINT IF EXISTS chk_cloud_provider_type;
ALTER TABLE cloud_credentials ADD COLUMN provider_type_old cloud_provider_type;
UPDATE cloud_credentials SET provider_type_old = provider_type::cloud_provider_type;
ALTER TABLE cloud_credentials ALTER COLUMN provider_type_old SET NOT NULL;
ALTER TABLE cloud_credentials DROP COLUMN provider_type;
ALTER TABLE cloud_credentials RENAME COLUMN provider_type_old TO provider_type;

-- 4. Restore cloud_provider_type on phishing_endpoints
ALTER TABLE phishing_endpoints DROP CONSTRAINT IF EXISTS chk_endpoint_cloud_provider;
ALTER TABLE phishing_endpoints ADD COLUMN cloud_provider_old cloud_provider_type;
UPDATE phishing_endpoints SET cloud_provider_old = cloud_provider::cloud_provider_type;
ALTER TABLE phishing_endpoints DROP COLUMN cloud_provider;
ALTER TABLE phishing_endpoints RENAME COLUMN cloud_provider_old TO cloud_provider;
-- Restore NOT NULL since original was NOT NULL
ALTER TABLE phishing_endpoints ALTER COLUMN cloud_provider SET NOT NULL;

-- 3. Restore domain_status on domain_profiles
DROP INDEX IF EXISTS idx_domain_profiles_status;
ALTER TABLE domain_profiles DROP CONSTRAINT IF EXISTS chk_domain_status;
ALTER TABLE domain_profiles ADD COLUMN status_old domain_status;
UPDATE domain_profiles SET status_old = status::domain_status;
ALTER TABLE domain_profiles ALTER COLUMN status_old SET NOT NULL;
ALTER TABLE domain_profiles ALTER COLUMN status_old SET DEFAULT 'pending_registration';
ALTER TABLE domain_profiles DROP COLUMN status;
ALTER TABLE domain_profiles RENAME COLUMN status_old TO status;
CREATE INDEX idx_domain_profiles_status ON domain_profiles (status);

-- 2. Restore provider_status on domain_provider_connections
ALTER TABLE domain_provider_connections DROP CONSTRAINT IF EXISTS chk_provider_status;
ALTER TABLE domain_provider_connections ADD COLUMN status_old provider_status;
UPDATE domain_provider_connections SET status_old = status::provider_status;
ALTER TABLE domain_provider_connections ALTER COLUMN status_old SET NOT NULL;
ALTER TABLE domain_provider_connections ALTER COLUMN status_old SET DEFAULT 'untested';
ALTER TABLE domain_provider_connections DROP COLUMN status;
ALTER TABLE domain_provider_connections RENAME COLUMN status_old TO status;

-- 1. Restore provider_type on domain_provider_connections
DROP INDEX IF EXISTS idx_dpc_provider_type;
ALTER TABLE domain_provider_connections DROP CONSTRAINT IF EXISTS chk_provider_type;
ALTER TABLE domain_provider_connections ADD COLUMN provider_type_old provider_type;
UPDATE domain_provider_connections SET provider_type_old = provider_type::provider_type;
ALTER TABLE domain_provider_connections ALTER COLUMN provider_type_old SET NOT NULL;
ALTER TABLE domain_provider_connections DROP COLUMN provider_type;
ALTER TABLE domain_provider_connections RENAME COLUMN provider_type_old TO provider_type;

-- Recreate partial index from migration 047 (which would have been on the ENUM column)
CREATE INDEX idx_phishing_endpoints_active ON phishing_endpoints (state) WHERE state NOT IN ('terminated', 'error');
