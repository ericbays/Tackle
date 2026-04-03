-- Rollback migration 016: drop domain_provider_connections table and enums.

DROP TABLE IF EXISTS domain_provider_connections;
DROP TYPE  IF EXISTS provider_status;
DROP TYPE  IF EXISTS provider_type;
