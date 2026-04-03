-- Migration 006: user_roles junction table.
-- Application logic enforces single-role assignment per user (REQ-RBAC-023),
-- but the junction table keeps the schema flexible.

CREATE TABLE user_roles (
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    role_id UUID NOT NULL REFERENCES roles(id) ON DELETE CASCADE,
    PRIMARY KEY (user_id, role_id)
);

CREATE INDEX idx_user_roles_role_id ON user_roles (role_id);
