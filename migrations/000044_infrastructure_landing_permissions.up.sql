-- Migration 044: Add infrastructure:* and landing_pages:* permissions.
-- server.go routes use "infrastructure:read" etc. for cloud credentials, SMTP profiles,
-- instance templates, and phishing endpoint routes. The original permission seeds used
-- separate "endpoints:*", "cloud:*", "smtp:*" resource types. This migration adds the
-- unified "infrastructure" resource type used by the actual route middleware.
-- Similarly, landing page routes use "landing_pages:*" but seeds used "templates.landing:*".

INSERT INTO permissions (resource_type, action, description) VALUES
    ('infrastructure', 'create', 'Create infrastructure resources (cloud credentials, SMTP profiles, instance templates, endpoints)'),
    ('infrastructure', 'read',   'View infrastructure resources'),
    ('infrastructure', 'update', 'Modify infrastructure resources'),
    ('infrastructure', 'delete', 'Delete infrastructure resources'),
    ('landing_pages',  'create', 'Create landing page projects'),
    ('landing_pages',  'read',   'View landing page projects'),
    ('landing_pages',  'update', 'Modify landing page projects'),
    ('landing_pages',  'delete', 'Delete landing page projects')
ON CONFLICT (resource_type, action) DO NOTHING;

-- Engineer role: full infrastructure access (had endpoints:*, cloud:*, smtp:* separately).
INSERT INTO role_permissions (role_id, permission_id)
SELECT r.id, p.id
FROM roles r
CROSS JOIN permissions p
WHERE r.name = 'engineer'
  AND p.resource_type = 'infrastructure'
  AND p.action IN ('create', 'read', 'update', 'delete')
ON CONFLICT DO NOTHING;

-- Engineer role: full landing page access (had templates.landing:* separately).
INSERT INTO role_permissions (role_id, permission_id)
SELECT r.id, p.id
FROM roles r
CROSS JOIN permissions p
WHERE r.name = 'engineer'
  AND p.resource_type = 'landing_pages'
  AND p.action IN ('create', 'read', 'update', 'delete')
ON CONFLICT DO NOTHING;

-- Operator role: read-only infrastructure (had endpoints:read, smtp:read).
INSERT INTO role_permissions (role_id, permission_id)
SELECT r.id, p.id
FROM roles r
CROSS JOIN permissions p
WHERE r.name = 'operator'
  AND p.resource_type = 'infrastructure'
  AND p.action = 'read'
ON CONFLICT DO NOTHING;

-- Operator role: full landing page access (had templates.landing:* separately).
INSERT INTO role_permissions (role_id, permission_id)
SELECT r.id, p.id
FROM roles r
CROSS JOIN permissions p
WHERE r.name = 'operator'
  AND p.resource_type = 'landing_pages'
  AND p.action IN ('create', 'read', 'update', 'delete')
ON CONFLICT DO NOTHING;
