-- Down migration 039: Drop phishing endpoint provisioning tables.

-- Remove endpoint permissions from roles.
DELETE FROM role_permissions
WHERE permission_id IN (
    SELECT id FROM permissions WHERE resource_type = 'endpoint'
);
DELETE FROM permissions WHERE resource_type = 'endpoint';

-- Revert campaigns cloud_provider check to exclude proxmox.
ALTER TABLE campaigns DROP CONSTRAINT IF EXISTS campaigns_cloud_provider_check;
ALTER TABLE campaigns ADD CONSTRAINT campaigns_cloud_provider_check
    CHECK (cloud_provider IS NULL OR cloud_provider IN ('aws', 'azure'));

-- Drop FK on phishing_endpoints.ssh_key_id before dropping ssh_keys table.
ALTER TABLE phishing_endpoints DROP CONSTRAINT IF EXISTS fk_phishing_endpoints_ssh_key;

-- Drop tables in dependency order.
DROP TABLE IF EXISTS proxmox_ip_allocations;
DROP TABLE IF EXISTS endpoint_ssh_keys;
DROP TABLE IF EXISTS endpoint_state_transitions;
DROP TABLE IF EXISTS phishing_endpoints;

-- Drop endpoint_state enum.
DROP TYPE IF EXISTS endpoint_state;

-- Note: Cannot remove 'proxmox' from cloud_provider_type enum in PostgreSQL without recreating the type.
-- This is intentional — enum values cannot be removed once added.
