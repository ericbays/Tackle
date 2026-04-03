-- Rollback migration 037: remove campaign approval tables.

-- Remove the campaigns:approve permission assignment.
DELETE FROM role_permissions
WHERE permission_id IN (
    SELECT id FROM permissions WHERE resource_type = 'campaigns' AND action = 'approve'
);

DELETE FROM permissions WHERE resource_type = 'campaigns' AND action = 'approve';

DROP TABLE IF EXISTS campaign_approval_requirements;
DROP TABLE IF EXISTS campaign_approvals;
