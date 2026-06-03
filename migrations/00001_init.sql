-- +goose Up
-- +goose StatementBegin
CREATE TABLE IF NOT EXISTS operation_log (
    id          BIGSERIAL PRIMARY KEY,
    app_id      TEXT        NOT NULL DEFAULT '',
    version_id  TEXT        NOT NULL DEFAULT '',
    tool        TEXT        NOT NULL,
    input_hash  TEXT        NOT NULL,
    status      TEXT        NOT NULL,
    error       TEXT        NOT NULL DEFAULT '',
    result      TEXT        NOT NULL DEFAULT '',
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);
-- +goose StatementEnd

-- +goose StatementBegin
CREATE INDEX IF NOT EXISTS idx_operation_log_input_hash
    ON operation_log (input_hash, status);
-- +goose StatementEnd

-- +goose StatementBegin
CREATE TABLE IF NOT EXISTS metadata_cache (
    app_id      TEXT        NOT NULL,
    version_id  TEXT        NOT NULL,
    locale      TEXT        NOT NULL,
    payload     JSONB       NOT NULL,
    fetched_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (app_id, version_id, locale)
);
-- +goose StatementEnd

-- +goose StatementBegin
CREATE TABLE IF NOT EXISTS screenshot_uploads (
    id                 BIGSERIAL PRIMARY KEY,
    operation_id       BIGINT      NOT NULL DEFAULT 0,
    asc_reservation_id TEXT        NOT NULL,
    chunk_state        JSONB       NOT NULL DEFAULT '{}'::jsonb,
    status             TEXT        NOT NULL,
    created_at         TIMESTAMPTZ NOT NULL DEFAULT now()
);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS screenshot_uploads;
DROP TABLE IF EXISTS metadata_cache;
DROP TABLE IF EXISTS operation_log;
-- +goose StatementEnd
