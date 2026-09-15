-- +goose Up
-- +goose StatementBegin
CREATE EXTENSION IF NOT EXISTS postgis;
-- +goose StatementEnd

-- +goose Down
-- PostGIS is required by the platform; the extension is intentionally not dropped.
