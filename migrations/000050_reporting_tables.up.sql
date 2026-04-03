-- Migration 050: Create reporting tables.

CREATE TABLE report_templates (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name            TEXT NOT NULL,
    description     TEXT,
    template_type   TEXT NOT NULL DEFAULT 'campaign',
    template_config JSONB NOT NULL DEFAULT '{}',
    layout_config   JSONB NOT NULL DEFAULT '{}',
    is_default      BOOLEAN NOT NULL DEFAULT FALSE,
    created_by      UUID NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
    deleted_at      TIMESTAMPTZ,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT now(),

    CONSTRAINT chk_report_template_type CHECK (template_type IN ('campaign', 'comparison', 'executive', 'compliance', 'custom'))
);

CREATE UNIQUE INDEX idx_report_templates_name_active ON report_templates (name) WHERE deleted_at IS NULL;
CREATE INDEX idx_report_templates_type ON report_templates (template_type);
CREATE INDEX idx_report_templates_created_by ON report_templates (created_by);

CREATE TRIGGER trg_report_templates_updated_at BEFORE UPDATE ON report_templates
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();

CREATE TABLE generated_reports (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    template_id     UUID REFERENCES report_templates(id) ON DELETE SET NULL,
    campaign_ids    UUID[] NOT NULL DEFAULT '{}',
    title           TEXT NOT NULL,
    format          TEXT NOT NULL DEFAULT 'pdf',
    status          TEXT NOT NULL DEFAULT 'pending',
    file_path       TEXT,
    file_size_bytes BIGINT,
    parameters      JSONB NOT NULL DEFAULT '{}',
    error_message   TEXT,
    generated_by    UUID NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
    started_at      TIMESTAMPTZ,
    completed_at    TIMESTAMPTZ,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT now(),

    CONSTRAINT chk_report_format CHECK (format IN ('pdf', 'csv', 'json', 'html')),
    CONSTRAINT chk_report_status CHECK (status IN ('pending', 'generating', 'completed', 'failed'))
);

CREATE INDEX idx_generated_reports_campaign_ids ON generated_reports USING GIN (campaign_ids);
CREATE INDEX idx_generated_reports_status ON generated_reports (status);
CREATE INDEX idx_generated_reports_generated_by ON generated_reports (generated_by);
CREATE INDEX idx_generated_reports_template ON generated_reports (template_id) WHERE template_id IS NOT NULL;

CREATE TRIGGER trg_generated_reports_updated_at BEFORE UPDATE ON generated_reports
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();
