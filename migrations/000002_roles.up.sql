-- Migration 002: roles table and built-in role seed data.

CREATE TABLE roles (
    id          UUID        NOT NULL DEFAULT gen_random_uuid() PRIMARY KEY,
    name        TEXT        NOT NULL UNIQUE,
    is_builtin  BOOLEAN     NOT NULL DEFAULT FALSE,
    description TEXT,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TRIGGER trg_roles_updated_at
    BEFORE UPDATE ON roles
    FOR EACH ROW
    EXECUTE FUNCTION set_updated_at();

-- Seed the four built-in roles (REQ-RBAC-001 through REQ-RBAC-004).
INSERT INTO roles (name, is_builtin, description) VALUES
    ('admin',    TRUE, 'Full administrative access to all resources.'),
    ('engineer', TRUE, 'Infrastructure and endpoint management; read access to campaign data.'),
    ('operator', TRUE, 'Campaign planning, execution, and target management.'),
    ('defender', TRUE, 'Read-only access to metrics and aggregate campaign results.');
