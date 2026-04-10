CREATE DATABASE IF NOT EXISTS analytics;

CREATE TABLE IF NOT EXISTS analytics.click_events_raw
(
    event_id UUID,
    request_id String,
    link_id UUID,
    short_code String,
    occurred_at DateTime64(3, 'UTC'),
    ip String,
    user_agent String,
    referer String
)
ENGINE = MergeTree
PARTITION BY toYYYYMM(occurred_at)
ORDER BY (link_id, occurred_at, event_id);
