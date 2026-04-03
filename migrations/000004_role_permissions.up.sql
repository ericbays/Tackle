-- Migration 004: role_permissions junction table and built-in role assignments.
-- Administrator role has implicit all-permissions; no rows seeded for admin.

CREATE TABLE role_permissions (
    role_id       UUID NOT NULL REFERENCES roles(id) ON DELETE CASCADE,
    permission_id UUID NOT NULL REFERENCES permissions(id) ON DELETE CASCADE,
    PRIMARY KEY (role_id, permission_id)
);

CREATE INDEX idx_role_permissions_permission_id ON role_permissions (permission_id);

-- Helper: look up role and permission UUIDs by name to seed assignments.

-- Engineer permissions (REQ-RBAC-013).
INSERT INTO role_permissions (role_id, permission_id)
SELECT r.id, p.id
FROM roles r, permissions p
WHERE r.name = 'engineer'
  AND (r.name, p.resource_type, p.action) IN (
    ('engineer', 'campaigns',               'read'),
    ('engineer', 'targets',                 'read'),
    ('engineer', 'templates.email',         'read'),
    ('engineer', 'templates.landing',       'create'),
    ('engineer', 'templates.landing',       'read'),
    ('engineer', 'templates.landing',       'update'),
    ('engineer', 'templates.landing',       'delete'),
    ('engineer', 'templates.landing',       'execute'),
    ('engineer', 'templates.landing',       'export'),
    ('engineer', 'domains',                 'create'),
    ('engineer', 'domains',                 'read'),
    ('engineer', 'domains',                 'update'),
    ('engineer', 'domains',                 'delete'),
    ('engineer', 'endpoints',               'create'),
    ('engineer', 'endpoints',               'read'),
    ('engineer', 'endpoints',               'update'),
    ('engineer', 'endpoints',               'delete'),
    ('engineer', 'endpoints',               'execute'),
    ('engineer', 'smtp',                    'create'),
    ('engineer', 'smtp',                    'read'),
    ('engineer', 'smtp',                    'update'),
    ('engineer', 'smtp',                    'delete'),
    ('engineer', 'smtp',                    'execute'),
    ('engineer', 'credentials',             'read'),
    ('engineer', 'credentials',             'delete'),
    ('engineer', 'credentials',             'export'),
    ('engineer', 'reports',                 'read'),
    ('engineer', 'metrics',                 'read'),
    ('engineer', 'metrics',                 'export'),
    ('engineer', 'logs.audit',              'read'),
    ('engineer', 'logs.audit',              'export'),
    ('engineer', 'logs.campaign',           'read'),
    ('engineer', 'logs.campaign',           'export'),
    ('engineer', 'logs.system',             'read'),
    ('engineer', 'logs.system',             'export'),
    ('engineer', 'settings',                'read'),
    ('engineer', 'cloud',                   'create'),
    ('engineer', 'cloud',                   'read'),
    ('engineer', 'cloud',                   'update'),
    ('engineer', 'cloud',                   'delete'),
    ('engineer', 'infrastructure.requests', 'create'),
    ('engineer', 'infrastructure.requests', 'read'),
    ('engineer', 'infrastructure.requests', 'update'),
    ('engineer', 'infrastructure.requests', 'approve'),
    ('engineer', 'schedules',               'read'),
    ('engineer', 'notifications',           'read'),
    ('engineer', 'notifications',           'create'),
    ('engineer', 'notifications',           'update'),
    ('engineer', 'notifications',           'delete'),
    ('engineer', 'api_keys',                'create'),
    ('engineer', 'api_keys',                'read'),
    ('engineer', 'api_keys',                'delete')
  );

-- Operator permissions (REQ-RBAC-013).
INSERT INTO role_permissions (role_id, permission_id)
SELECT r.id, p.id
FROM roles r, permissions p
WHERE r.name = 'operator'
  AND (r.name, p.resource_type, p.action) IN (
    ('operator', 'campaigns',               'create'),
    ('operator', 'campaigns',               'read'),
    ('operator', 'campaigns',               'update'),
    ('operator', 'campaigns',               'delete'),
    ('operator', 'campaigns',               'execute'),
    ('operator', 'campaigns',               'export'),
    ('operator', 'targets',                 'create'),
    ('operator', 'targets',                 'read'),
    ('operator', 'targets',                 'update'),
    ('operator', 'targets',                 'delete'),
    ('operator', 'targets',                 'export'),
    ('operator', 'templates.email',         'create'),
    ('operator', 'templates.email',         'read'),
    ('operator', 'templates.email',         'update'),
    ('operator', 'templates.email',         'delete'),
    ('operator', 'templates.email',         'export'),
    ('operator', 'templates.landing',       'create'),
    ('operator', 'templates.landing',       'read'),
    ('operator', 'templates.landing',       'update'),
    ('operator', 'templates.landing',       'delete'),
    ('operator', 'templates.landing',       'execute'),
    ('operator', 'templates.landing',       'export'),
    ('operator', 'endpoints',               'read'),
    ('operator', 'smtp',                    'read'),
    ('operator', 'smtp',                    'execute'),
    ('operator', 'credentials',             'read'),
    ('operator', 'credentials',             'delete'),
    ('operator', 'credentials',             'export'),
    ('operator', 'reports',                 'create'),
    ('operator', 'reports',                 'read'),
    ('operator', 'reports',                 'delete'),
    ('operator', 'reports',                 'export'),
    ('operator', 'metrics',                 'read'),
    ('operator', 'metrics',                 'export'),
    ('operator', 'logs.campaign',           'read'),
    ('operator', 'logs.campaign',           'export'),
    ('operator', 'infrastructure.requests', 'create'),
    ('operator', 'infrastructure.requests', 'read'),
    ('operator', 'infrastructure.requests', 'update'),
    ('operator', 'schedules',               'create'),
    ('operator', 'schedules',               'read'),
    ('operator', 'schedules',               'update'),
    ('operator', 'schedules',               'delete'),
    ('operator', 'schedules',               'execute'),
    ('operator', 'notifications',           'read'),
    ('operator', 'notifications',           'create'),
    ('operator', 'notifications',           'update'),
    ('operator', 'notifications',           'delete'),
    ('operator', 'api_keys',                'create'),
    ('operator', 'api_keys',                'read'),
    ('operator', 'api_keys',                'delete')
  );

-- Defender permissions (REQ-RBAC-013): metrics:read and notifications:read only.
INSERT INTO role_permissions (role_id, permission_id)
SELECT r.id, p.id
FROM roles r, permissions p
WHERE r.name = 'defender'
  AND (r.name, p.resource_type, p.action) IN (
    ('defender', 'metrics',        'read'),
    ('defender', 'notifications',  'read')
  );
