-- Migration 025: cloud_credentials table.

-- Cloud provider type enum.
CREATE TYPE cloud_provider_type AS ENUM ('aws', 'azure');

-- Cloud credential status enum.
CREATE TYPE cloud_credential_status AS ENUM ('untested', 'healthy', 'error');

CREATE TABLE cloud_credentials (
    id                    UUID                    NOT NULL DEFAULT gen_random_uuid() PRIMARY KEY,
    provider_type         cloud_provider_type     NOT NULL,
    display_name          VARCHAR(255)            NOT NULL,
    credentials_encrypted BYTEA                   NOT NULL,
    default_region        VARCHAR(100)            NOT NULL DEFAULT '',
    status                cloud_credential_status NOT NULL DEFAULT 'untested',
    status_message        TEXT,
    last_tested_at        TIMESTAMPTZ,
    created_by            UUID                    NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
    created_at            TIMESTAMPTZ             NOT NULL DEFAULT now(),
    updated_at            TIMESTAMPTZ             NOT NULL DEFAULT now(),
    CONSTRAINT uq_cloud_credentials_display_name UNIQUE (display_name)
);

CREATE INDEX idx_cloud_credentials_provider_type ON cloud_credentials (provider_type);
CREATE INDEX idx_cloud_credentials_status        ON cloud_credentials (status);

-- Trigger to keep updated_at current.
CREATE TRIGGER trg_cloud_credentials_updated_at
    BEFORE UPDATE ON cloud_credentials
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();

-- Add infrastructure:read/create/update/delete permissions.
INSERT INTO permissions (resource_type, action, description) VALUES
    ('infrastructure', 'read',   'View cloud credentials and instance templates'),
    ('infrastructure', 'create', 'Create cloud credentials and instance templates'),
    ('infrastructure', 'update', 'Update cloud credentials and instance templates'),
    ('infrastructure', 'delete', 'Delete cloud credentials and instance templates')
ON CONFLICT (resource_type, action) DO NOTHING;

-- Grant to admin and engineer roles.
INSERT INTO role_permissions (role_id, permission_id)
SELECT r.id, p.id
FROM roles r
CROSS JOIN permissions p
WHERE r.name IN ('admin', 'engineer')
  AND p.resource_type = 'infrastructure'
ON CONFLICT DO NOTHING;
