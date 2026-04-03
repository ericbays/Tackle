-- Migration 029 rollback.
DROP TABLE IF EXISTS campaign_send_schedules;
DROP TYPE IF EXISTS sending_strategy;
