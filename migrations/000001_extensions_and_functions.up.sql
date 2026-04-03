-- Migration 001: PostgreSQL extensions and shared trigger functions.

CREATE EXTENSION IF NOT EXISTS "pgcrypto";
CREATE EXTENSION IF NOT EXISTS "pg_trgm";

-- set_updated_at sets updated_at to the current timestamp on every UPDATE.
-- Attach this trigger to every table that has an updated_at column.
CREATE OR REPLACE FUNCTION set_updated_at()
RETURNS TRIGGER
LANGUAGE plpgsql
AS $$
BEGIN
    NEW.updated_at = now();
    RETURN NEW;
END;
$$;

-- reject_modification raises an exception, preventing UPDATE and DELETE on
-- tables that must be immutable (e.g., audit_logs).
CREATE OR REPLACE FUNCTION reject_modification()
RETURNS TRIGGER
LANGUAGE plpgsql
AS $$
BEGIN
    RAISE EXCEPTION 'modification of % is not permitted', TG_TABLE_NAME;
END;
$$;
