-- Reverse migration 044: Remove infrastructure:* and landing_pages:* permissions.

DELETE FROM role_permissions
WHERE permission_id IN (
    SELECT id FROM permissions
    WHERE resource_type IN ('infrastructure', 'landing_pages')
);

DELETE FROM permissions
WHERE resource_type IN ('infrastructure', 'landing_pages');
