-- Rollback migration 035: target groups, block list, canary targets, overrides.

DROP TABLE IF EXISTS blocklist_overrides;
DROP TABLE IF EXISTS campaign_canary_targets;
DROP TABLE IF EXISTS blocklist_entries;
DROP TABLE IF EXISTS campaign_target_groups;
DROP TABLE IF EXISTS target_group_members;
DROP TABLE IF EXISTS target_groups;
