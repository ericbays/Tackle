-- Migration 043: Seed missing RBAC permissions.
-- credentials:reveal, credentials:purge, and blocklist:manage are used in
-- server.go requirePerm() middleware but were never seeded in any migration.
-- Non-admin users receive 403 because the permission rows don't exist.

INSERT INTO permissions (resource_type, action, description) VALUES
    ('credentials', 'reveal', 'Reveal captured credential values'),
    ('credentials', 'purge',  'Purge captured credentials'),
    ('blocklist',   'manage', 'Manage the global target block list')
ON CONFLICT (resource_type, action) DO NOTHING;

-- Grant credentials:reveal to Engineer role.
-- credentials:purge and blocklist:manage remain Admin-only (admin bypasses all checks).
INSERT INTO role_permissions (role_id, permission_id)
SELECT r.id, p.id
FROM roles r
CROSS JOIN permissions p
WHERE r.name = 'engineer'
  AND p.resource_type = 'credentials'
  AND p.action = 'reveal'
ON CONFLICT DO NOTHING;
