CREATE TABLE links (
    id               UUID PRIMARY KEY,
    short_code       TEXT NOT NULL,
    original_url     TEXT NOT NULL,
    title            TEXT,
    created_by       UUID,
    created_at       TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at       TIMESTAMPTZ NOT NULL DEFAULT now(),
    expires_at       TIMESTAMPTZ,
    max_clicks       BIGINT NOT NULL DEFAULT 0,
    redirect_count   BIGINT NOT NULL DEFAULT 0,
    disabled_at      TIMESTAMPTZ,
    deleted_at       TIMESTAMPTZ,
    last_accessed_at TIMESTAMPTZ
);

CREATE UNIQUE INDEX idx_links_short_code_active
    ON links (short_code) WHERE deleted_at IS NULL;

CREATE INDEX idx_links_created_at
    ON links (created_at DESC);
