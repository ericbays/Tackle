-- Migration 039: Phishing endpoint provisioning tables.
-- REQ-DB-044: Endpoint schema, state machine, SSH keys, Proxmox IP allocations.

-- Extend cloud_provider_type enum with 'proxmox'.
ALTER TYPE cloud_provider_type ADD VALUE IF NOT EXISTS 'proxmox';

-- Endpoint state enum.
CREATE TYPE endpoint_state AS ENUM (
    'requested',
    'provisioning',
    'configuring',
    'active',
    'stopped',
    'error',
    'terminated'
);

-- Core phishing endpoints table.
CREATE TABLE phishing_endpoints (
    id                UUID            NOT NULL DEFAULT gen_random_uuid() PRIMARY KEY,
    campaign_id       UUID            REFERENCES campaigns(id) ON DELETE SET NULL,
    cloud_provider    cloud_provider_type NOT NULL,
    region            VARCHAR(100)    NOT NULL,
    instance_id       VARCHAR(255),
    public_ip         INET,
    domain            VARCHAR(255),
    state             endpoint_state  NOT NULL DEFAULT 'requested',
    binary_hash       VARCHAR(128),
    control_port      INTEGER,
    auth_token        BYTEA,
    ssh_key_id        UUID,
    error_message     TEXT,
    last_heartbeat_at TIMESTAMPTZ,
    provisioned_at    TIMESTAMPTZ,
    activated_at      TIMESTAMPTZ,
    terminated_at     TIMESTAMPTZ,
    created_at        TIMESTAMPTZ     NOT NULL DEFAULT now(),
    updated_at        TIMESTAMPTZ     NOT NULL DEFAULT now()
);

CREATE INDEX idx_phishing_endpoints_campaign_id ON phishing_endpoints (campaign_id) WHERE campaign_id IS NOT NULL;
CREATE INDEX idx_phishing_endpoints_state ON phishing_endpoints (state);
CREATE INDEX idx_phishing_endpoints_cloud_provider ON phishing_endpoints (cloud_provider);

CREATE TRIGGER trg_phishing_endpoints_updated_at
    BEFORE UPDATE ON phishing_endpoints
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();

-- Endpoint state transition history (append-only audit log).
CREATE TABLE endpoint_state_transitions (
    id            UUID        NOT NULL DEFAULT gen_random_uuid() PRIMARY KEY,
    endpoint_id   UUID        NOT NULL REFERENCES phishing_endpoints(id) ON DELETE CASCADE,
    from_state    endpoint_state NOT NULL,
    to_state      endpoint_state NOT NULL,
    actor         VARCHAR(255) NOT NULL,
    reason        TEXT        NOT NULL DEFAULT '',
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_est_endpoint_id ON endpoint_state_transitions (endpoint_id);
CREATE INDEX idx_est_created_at ON endpoint_state_transitions (created_at);

-- Per-campaign SSH key pairs for endpoint configuration deployment.
CREATE TABLE endpoint_ssh_keys (
    id                    UUID        NOT NULL DEFAULT gen_random_uuid() PRIMARY KEY,
    campaign_id           UUID        NOT NULL REFERENCES campaigns(id) ON DELETE CASCADE,
    public_key            TEXT        NOT NULL,
    private_key_encrypted BYTEA       NOT NULL,
    created_at            TIMESTAMPTZ NOT NULL DEFAULT now(),
    destroyed_at          TIMESTAMPTZ
);

CREATE INDEX idx_endpoint_ssh_keys_campaign ON endpoint_ssh_keys (campaign_id);

-- Proxmox IP pool allocation tracking.
-- REQ-DB-044a: Static IP pool per credential, atomic allocation, no double-allocation.
CREATE TABLE proxmox_ip_allocations (
    id                  UUID        NOT NULL DEFAULT gen_random_uuid() PRIMARY KEY,
    cloud_credential_id UUID        NOT NULL REFERENCES cloud_credentials(id) ON DELETE RESTRICT,
    ip_address          INET        NOT NULL,
    endpoint_id         UUID        REFERENCES phishing_endpoints(id) ON DELETE SET NULL,
    allocated_at        TIMESTAMPTZ,
    released_at         TIMESTAMPTZ,
    created_at          TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- Prevent double-allocation: only one active allocation per IP per credential.
CREATE UNIQUE INDEX idx_proxmox_ip_unique_active
    ON proxmox_ip_allocations (cloud_credential_id, ip_address)
    WHERE released_at IS NULL;

CREATE INDEX idx_proxmox_ip_credential ON proxmox_ip_allocations (cloud_credential_id);
CREATE INDEX idx_proxmox_ip_endpoint ON proxmox_ip_allocations (endpoint_id) WHERE endpoint_id IS NOT NULL;

-- Add the FK for ssh_key_id now that endpoint_ssh_keys exists.
ALTER TABLE phishing_endpoints
    ADD CONSTRAINT fk_phishing_endpoints_ssh_key
    FOREIGN KEY (ssh_key_id) REFERENCES endpoint_ssh_keys(id) ON DELETE SET NULL;

-- Update campaigns table to allow 'proxmox' as cloud_provider value.
ALTER TABLE campaigns DROP CONSTRAINT IF EXISTS campaigns_cloud_provider_check;
ALTER TABLE campaigns ADD CONSTRAINT campaigns_cloud_provider_check
    CHECK (cloud_provider IS NULL OR cloud_provider IN ('aws', 'azure', 'proxmox'));

-- Add endpoint permissions.
INSERT INTO permissions (resource_type, action, description) VALUES
    ('endpoint', 'read',   'View phishing endpoint status and details'),
    ('endpoint', 'create', 'Provision new phishing endpoints'),
    ('endpoint', 'update', 'Manage phishing endpoint lifecycle'),
    ('endpoint', 'delete', 'Terminate phishing endpoints')
ON CONFLICT (resource_type, action) DO NOTHING;

-- Grant endpoint permissions to admin and engineer roles.
INSERT INTO role_permissions (role_id, permission_id)
SELECT r.id, p.id
FROM roles r
CROSS JOIN permissions p
WHERE r.name IN ('admin', 'engineer')
  AND p.resource_type = 'endpoint'
ON CONFLICT DO NOTHING;
