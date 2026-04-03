-- Alert rules for audit log event monitoring.
CREATE TABLE alert_rules (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name TEXT NOT NULL,
    description TEXT,
    conditions JSONB NOT NULL,
    actions JSONB NOT NULL,
    is_enabled BOOLEAN NOT NULL DEFAULT TRUE,
    cooldown_minutes INT NOT NULL DEFAULT 60,
    last_triggered_at TIMESTAMPTZ,
    created_by UUID NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_alert_rules_enabled ON alert_rules (is_enabled) WHERE is_enabled = TRUE;

-- Seed default alert rules. created_by references a system placeholder that will be
-- replaced with the initial admin user during setup. Use a subquery for safety.
INSERT INTO alert_rules (name, description, conditions, actions, is_enabled, cooldown_minutes, created_by)
SELECT
    'Failed Login Surge',
    '5+ failed login attempts within 5 minutes from the same source',
    '{"severity": "warning", "action_pattern": "auth.login.failure", "threshold": 5, "window_minutes": 5}'::jsonb,
    '{"notify": true}'::jsonb,
    TRUE,
    5,
    id
FROM users
WHERE is_initial_admin = TRUE
LIMIT 1;

INSERT INTO alert_rules (name, description, conditions, actions, is_enabled, cooldown_minutes, created_by)
SELECT
    'Credential Purge',
    'Any credential purge operation triggers an admin notification',
    '{"action_pattern": "credentials.purge"}'::jsonb,
    '{"notify": true}'::jsonb,
    TRUE,
    60,
    id
FROM users
WHERE is_initial_admin = TRUE
LIMIT 1;

INSERT INTO alert_rules (name, description, conditions, actions, is_enabled, cooldown_minutes, created_by)
SELECT
    'Endpoint Error',
    'Phishing endpoint transitions to error state',
    '{"category": "infrastructure", "action_pattern": "endpoint.state_change", "severity": "error"}'::jsonb,
    '{"notify": true}'::jsonb,
    TRUE,
    15,
    id
FROM users
WHERE is_initial_admin = TRUE
LIMIT 1;

INSERT INTO alert_rules (name, description, conditions, actions, is_enabled, cooldown_minutes, created_by)
SELECT
    'Unauthorized Access Attempt',
    'Any 403 forbidden response is tracked',
    '{"action_pattern": "auth.forbidden"}'::jsonb,
    '{"notify": false}'::jsonb,
    TRUE,
    0,
    id
FROM users
WHERE is_initial_admin = TRUE
LIMIT 1;
