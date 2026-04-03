-- Migration 027 rollback.
DROP TABLE IF EXISTS smtp_profiles;
DROP TYPE IF EXISTS smtp_profile_status;
DROP TYPE IF EXISTS smtp_tls_mode;
DROP TYPE IF EXISTS smtp_auth_type;
