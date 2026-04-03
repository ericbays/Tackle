-- Migration 032: add default_role_id and auto_provision to auth_providers.
-- default_role_id: the role assigned to auto-provisioned users when no group mapping matches.
-- auto_provision:  whether new users are automatically created on first login.
-- auth_order:      'local_first' or 'ldap_first' — controls LDAP fallback order.

ALTER TABLE auth_providers
    ADD COLUMN default_role_id UUID REFERENCES roles(id) ON DELETE SET NULL,
    ADD COLUMN auto_provision  BOOLEAN     NOT NULL DEFAULT TRUE,
    ADD COLUMN auth_order      TEXT        NOT NULL DEFAULT 'local_first'
                                   CHECK (auth_order IN ('local_first', 'ldap_first'));
