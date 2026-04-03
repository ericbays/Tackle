-- Rollback migration 000023
DROP TABLE IF EXISTS domain_health_checks;
DROP TYPE IF EXISTS domain_health_trigger;
DROP TYPE IF EXISTS domain_health_overall_status;
DROP TYPE IF EXISTS domain_health_check_type;
