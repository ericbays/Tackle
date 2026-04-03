-- Rollback 004.
DROP INDEX IF EXISTS idx_role_permissions_permission_id;
DROP TABLE IF EXISTS role_permissions;
