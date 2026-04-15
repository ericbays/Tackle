CREATE TABLE app_logs (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    timestamp TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    level VARCHAR(20) NOT NULL,
    message TEXT NOT NULL,
    attributes JSONB NOT NULL DEFAULT '{}'::jsonb
);

CREATE INDEX idx_app_logs_timestamp ON app_logs(timestamp DESC);
CREATE INDEX idx_app_logs_level ON app_logs(level);
