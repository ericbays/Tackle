-- Migration 003: permissions table and full permission matrix seed.

CREATE TABLE permissions (
    id            UUID NOT NULL DEFAULT gen_random_uuid() PRIMARY KEY,
    resource_type TEXT NOT NULL,
    action        TEXT NOT NULL,
    description   TEXT,
    UNIQUE (resource_type, action)
);

-- Seed all applicable permissions from the permission matrix (REQ-RBAC-012).
-- Format: resource_type:action per the matrix table in the requirements.
INSERT INTO permissions (resource_type, action, description) VALUES
    -- users
    ('users', 'create', 'Create user accounts'),
    ('users', 'read',   'View user accounts'),
    ('users', 'update', 'Modify user accounts'),
    ('users', 'delete', 'Delete user accounts'),
    ('users', 'export', 'Export user account data'),

    -- roles
    ('roles', 'create', 'Create roles'),
    ('roles', 'read',   'View roles and permissions'),
    ('roles', 'update', 'Modify role permission assignments'),
    ('roles', 'delete', 'Delete custom roles'),

    -- campaigns
    ('campaigns', 'create',  'Create phishing campaigns'),
    ('campaigns', 'read',    'View campaign details and status'),
    ('campaigns', 'update',  'Modify campaign configuration'),
    ('campaigns', 'delete',  'Delete campaigns'),
    ('campaigns', 'execute', 'Launch, pause, and stop campaigns'),
    ('campaigns', 'export',  'Export campaign data'),

    -- targets
    ('targets', 'create', 'Create target entries and lists'),
    ('targets', 'read',   'View target details'),
    ('targets', 'update', 'Modify target records'),
    ('targets', 'delete', 'Delete targets'),
    ('targets', 'export', 'Export target data'),

    -- templates.email
    ('templates.email', 'create', 'Create email templates'),
    ('templates.email', 'read',   'View email templates'),
    ('templates.email', 'update', 'Modify email templates'),
    ('templates.email', 'delete', 'Delete email templates'),
    ('templates.email', 'export', 'Export email templates'),

    -- templates.landing
    ('templates.landing', 'create',  'Create landing page templates'),
    ('templates.landing', 'read',    'View landing page templates'),
    ('templates.landing', 'update',  'Modify landing page templates'),
    ('templates.landing', 'delete',  'Delete landing page templates'),
    ('templates.landing', 'execute', 'Build and deploy landing pages'),
    ('templates.landing', 'export',  'Export landing page templates'),

    -- domains
    ('domains', 'create', 'Register and add domains'),
    ('domains', 'read',   'View domain configuration'),
    ('domains', 'update', 'Modify DNS records and certificates'),
    ('domains', 'delete', 'Remove domains'),

    -- endpoints
    ('endpoints', 'create',  'Provision phishing endpoints'),
    ('endpoints', 'read',    'View endpoint status and configuration'),
    ('endpoints', 'update',  'Modify endpoint configuration'),
    ('endpoints', 'delete',  'Terminate endpoints'),
    ('endpoints', 'execute', 'Deploy, start, and stop endpoints'),

    -- smtp
    ('smtp', 'create',  'Add SMTP server configurations'),
    ('smtp', 'read',    'View SMTP configurations'),
    ('smtp', 'update',  'Modify SMTP configurations'),
    ('smtp', 'delete',  'Remove SMTP configurations'),
    ('smtp', 'execute', 'Send test emails via SMTP'),

    -- credentials
    ('credentials', 'read',   'View captured credentials'),
    ('credentials', 'delete', 'Delete captured credential records'),
    ('credentials', 'export', 'Export captured credentials'),

    -- reports
    ('reports', 'create', 'Generate reports'),
    ('reports', 'read',   'View reports'),
    ('reports', 'delete', 'Delete reports'),
    ('reports', 'export', 'Export reports to PDF/CSV'),

    -- metrics
    ('metrics', 'read',   'View metrics dashboards'),
    ('metrics', 'export', 'Export metrics data'),

    -- logs.audit
    ('logs.audit', 'read',   'View all audit log entries'),
    ('logs.audit', 'export', 'Export audit logs'),

    -- logs.campaign
    ('logs.campaign', 'read',   'View campaign activity logs'),
    ('logs.campaign', 'export', 'Export campaign logs'),

    -- logs.system
    ('logs.system', 'read',   'View system event logs'),
    ('logs.system', 'export', 'Export system logs'),

    -- settings
    ('settings', 'read',   'View system-wide configuration'),
    ('settings', 'update', 'Modify system-wide configuration'),

    -- settings.auth
    ('settings.auth', 'read',   'View authentication provider configuration'),
    ('settings.auth', 'update', 'Modify authentication provider configuration'),

    -- cloud
    ('cloud', 'create', 'Add cloud provider credentials'),
    ('cloud', 'read',   'View cloud provider configuration'),
    ('cloud', 'update', 'Modify cloud provider credentials'),
    ('cloud', 'delete', 'Remove cloud provider credentials'),

    -- infrastructure.requests
    ('infrastructure.requests', 'create',  'Submit infrastructure change requests'),
    ('infrastructure.requests', 'read',    'View infrastructure change requests'),
    ('infrastructure.requests', 'update',  'Modify infrastructure change requests'),
    ('infrastructure.requests', 'approve', 'Approve or reject infrastructure change requests'),

    -- schedules
    ('schedules', 'create',  'Create campaign schedules'),
    ('schedules', 'read',    'View campaign schedules'),
    ('schedules', 'update',  'Modify campaign schedules'),
    ('schedules', 'delete',  'Delete campaign schedules'),
    ('schedules', 'execute', 'Trigger scheduled campaigns manually'),

    -- notifications
    ('notifications', 'create', 'Create system notifications'),
    ('notifications', 'read',   'View notifications'),
    ('notifications', 'update', 'Mark notifications as read or configure preferences'),
    ('notifications', 'delete', 'Delete notifications'),

    -- api_keys
    ('api_keys', 'create', 'Generate API keys'),
    ('api_keys', 'read',   'View API key metadata'),
    ('api_keys', 'delete', 'Revoke API keys');
