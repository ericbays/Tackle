-- Migration 058 rollback: Remove user preferences column.
ALTER TABLE users DROP COLUMN IF EXISTS preferences;
