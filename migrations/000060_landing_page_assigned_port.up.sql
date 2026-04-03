-- Migration 060: Add assigned_port to landing_page_projects.
-- Persists the port across rebuilds so phishing endpoints have a stable target.

ALTER TABLE landing_page_projects
    ADD COLUMN assigned_port INTEGER CHECK (assigned_port IS NULL OR (assigned_port >= 1024 AND assigned_port <= 65535));

CREATE UNIQUE INDEX idx_lpp_assigned_port
    ON landing_page_projects (assigned_port)
    WHERE assigned_port IS NOT NULL AND deleted_at IS NULL;
