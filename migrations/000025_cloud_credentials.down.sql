-- Migration 025 rollback: drop cloud_credentials table and related permissions.

DELETE FROM role_permissions
WHERE permission_id IN (
    SELECT id FROM permissions WHERE resource_type = 'infrastructure'
);
DELETE FROM permissions WHERE resource_type = 'infrastructure';

DROP TABLE IF EXISTS cloud_credentials;
DROP TYPE IF EXISTS cloud_credential_status;
DROP TYPE IF EXISTS cloud_provider_type;
