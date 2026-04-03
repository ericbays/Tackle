-- Reverse migration 043: Remove the three permissions seeded in the up migration.

DELETE FROM role_permissions
WHERE permission_id IN (
    SELECT id FROM permissions
    WHERE (resource_type, action) IN (
        ('credentials', 'reveal'),
        ('credentials', 'purge'),
        ('blocklist',   'manage')
    )
);

DELETE FROM permissions
WHERE (resource_type, action) IN (
    ('credentials', 'reveal'),
    ('credentials', 'purge'),
    ('blocklist',   'manage')
);
