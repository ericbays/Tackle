-- Migration 026: instance_templates and instance_template_versions tables.

CREATE TABLE instance_templates (
    id                   UUID         NOT NULL DEFAULT gen_random_uuid() PRIMARY KEY,
    display_name         VARCHAR(255) NOT NULL,
    cloud_credential_id  UUID         NOT NULL REFERENCES cloud_credentials(id) ON DELETE RESTRICT,
    provider_type        cloud_provider_type NOT NULL,
    current_version      INTEGER      NOT NULL DEFAULT 1,
    created_by           UUID         NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
    created_at           TIMESTAMPTZ  NOT NULL DEFAULT now(),
    updated_at           TIMESTAMPTZ  NOT NULL DEFAULT now(),
    CONSTRAINT uq_instance_templates_display_name UNIQUE (display_name)
);

CREATE INDEX idx_instance_templates_cloud_credential_id ON instance_templates (cloud_credential_id);
CREATE INDEX idx_instance_templates_provider_type       ON instance_templates (provider_type);

-- Trigger to keep updated_at current.
CREATE TRIGGER trg_instance_templates_updated_at
    BEFORE UPDATE ON instance_templates
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();

CREATE TABLE instance_template_versions (
    id               UUID         NOT NULL DEFAULT gen_random_uuid() PRIMARY KEY,
    template_id      UUID         NOT NULL REFERENCES instance_templates(id) ON DELETE CASCADE,
    version_number   INTEGER      NOT NULL,
    region           VARCHAR(100) NOT NULL,
    instance_size    VARCHAR(100) NOT NULL,
    os_image         VARCHAR(255) NOT NULL,
    security_groups  TEXT[]       NOT NULL DEFAULT '{}',
    ssh_key_reference VARCHAR(255),
    user_data        TEXT,
    tags             JSONB        NOT NULL DEFAULT '{}',
    notes            TEXT,
    created_by       UUID         NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
    created_at       TIMESTAMPTZ  NOT NULL DEFAULT now(),
    CONSTRAINT uq_instance_template_versions UNIQUE (template_id, version_number)
);

CREATE INDEX idx_instance_template_versions_template_id ON instance_template_versions (template_id);
