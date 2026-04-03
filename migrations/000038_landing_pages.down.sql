-- Rollback migration 038: Landing page builder tables.

-- Remove FK from campaigns.
ALTER TABLE campaigns DROP CONSTRAINT IF EXISTS fk_campaigns_landing_page;

-- Remove role-permission grants for landing page permissions.
DELETE FROM role_permissions
WHERE permission_id IN (
    SELECT id FROM permissions WHERE name LIKE 'landing_pages:%'
);

-- Remove permissions.
DELETE FROM permissions WHERE name LIKE 'landing_pages:%';

-- Drop tables in dependency order.
DROP TABLE IF EXISTS landing_page_health_checks;
DROP TABLE IF EXISTS landing_page_builds;
DROP TABLE IF EXISTS landing_page_templates;
DROP TABLE IF EXISTS landing_page_projects;
