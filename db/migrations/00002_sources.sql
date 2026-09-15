-- +goose Up
CREATE TABLE sources (
    id         text PRIMARY KEY,
    name       text NOT NULL,
    type       text NOT NULL CHECK (type IN ('synthetic', 'operator', 'external', 'sensor')),
    status     text NOT NULL DEFAULT 'active' CHECK (status IN ('active', 'degraded', 'offline')),
    metadata   jsonb NOT NULL DEFAULT '{}'::jsonb,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX sources_type_idx ON sources (type);
CREATE INDEX sources_status_idx ON sources (status);

-- +goose Down
DROP TABLE sources;
