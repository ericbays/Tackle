-- Migration 033: email_templates and email_template_versions tables.

CREATE TABLE email_templates (
    id              UUID            NOT NULL DEFAULT gen_random_uuid() PRIMARY KEY,
    name            VARCHAR(255)    NOT NULL,
    description     TEXT,
    subject         VARCHAR(998)    NOT NULL,
    html_body       TEXT            NOT NULL DEFAULT '',
    text_body       TEXT            NOT NULL DEFAULT '',
    category        VARCHAR(100)    NOT NULL DEFAULT 'general',
    tags            TEXT[]          NOT NULL DEFAULT '{}',
    is_shared       BOOLEAN         NOT NULL DEFAULT FALSE,
    created_by      UUID            NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
    created_at      TIMESTAMPTZ     NOT NULL DEFAULT now(),
    updated_at      TIMESTAMPTZ     NOT NULL DEFAULT now(),
    CONSTRAINT uq_email_templates_name UNIQUE (name)
);

CREATE INDEX idx_email_templates_category ON email_templates (category);
CREATE INDEX idx_email_templates_created_by ON email_templates (created_by);
CREATE INDEX idx_email_templates_is_shared ON email_templates (is_shared);

-- Trigger to keep updated_at current.
CREATE TRIGGER trg_email_templates_updated_at
    BEFORE UPDATE ON email_templates
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();

CREATE TABLE email_template_versions (
    id              UUID            NOT NULL DEFAULT gen_random_uuid() PRIMARY KEY,
    template_id     UUID            NOT NULL REFERENCES email_templates(id) ON DELETE CASCADE,
    version_number  INTEGER         NOT NULL,
    subject         VARCHAR(998)    NOT NULL,
    html_body       TEXT            NOT NULL DEFAULT '',
    text_body       TEXT            NOT NULL DEFAULT '',
    change_note     TEXT,
    created_by      UUID            NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
    created_at      TIMESTAMPTZ     NOT NULL DEFAULT now(),
    CONSTRAINT uq_email_template_versions UNIQUE (template_id, version_number)
);

CREATE INDEX idx_email_template_versions_template_id ON email_template_versions (template_id);
