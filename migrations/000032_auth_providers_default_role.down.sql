ALTER TABLE auth_providers
    DROP COLUMN IF EXISTS auth_order,
    DROP COLUMN IF EXISTS auto_provision,
    DROP COLUMN IF EXISTS default_role_id;
