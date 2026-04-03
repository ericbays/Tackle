-- Migration 029: campaign_send_schedules table.

-- Sending strategy enum.
CREATE TYPE sending_strategy AS ENUM ('round_robin', 'random', 'weighted', 'failover', 'segment');

CREATE TABLE campaign_send_schedules (
    id                    UUID              NOT NULL DEFAULT gen_random_uuid() PRIMARY KEY,
    campaign_id           UUID              NOT NULL,
    sending_strategy      sending_strategy  NOT NULL DEFAULT 'round_robin',
    send_window_start     TIME,
    send_window_end       TIME,
    send_window_timezone  VARCHAR(100),
    send_window_days      INTEGER[],
    campaign_rate_limit   INTEGER,
    min_delay_ms          INTEGER           NOT NULL DEFAULT 0,
    max_delay_ms          INTEGER           NOT NULL DEFAULT 0,
    batch_size            INTEGER,
    batch_pause_seconds   INTEGER,
    created_at            TIMESTAMPTZ       NOT NULL DEFAULT now(),
    updated_at            TIMESTAMPTZ       NOT NULL DEFAULT now(),
    CONSTRAINT uq_campaign_send_schedules_campaign UNIQUE (campaign_id)
);

-- Trigger to keep updated_at current.
CREATE TRIGGER trg_campaign_send_schedules_updated_at
    BEFORE UPDATE ON campaign_send_schedules
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();
