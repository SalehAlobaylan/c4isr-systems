-- +goose Up
-- Assets are entities we control or coordinate. They are structurally distinct
-- from tracks (entities being observed).

CREATE TABLE assets (
    id           text PRIMARY KEY,
    name         text NOT NULL,
    type         text NOT NULL,
    status       text NOT NULL DEFAULT 'available'
                 CHECK (status IN ('available', 'assigned', 'unavailable', 'offline', 'maintenance')),
    capabilities jsonb NOT NULL DEFAULT '[]'::jsonb,
    metadata     jsonb NOT NULL DEFAULT '{}'::jsonb,
    created_at   timestamptz NOT NULL DEFAULT now(),
    updated_at   timestamptz NOT NULL DEFAULT now()
);

-- Current state projection: one row per asset, optimized for operational reads.
CREATE TABLE asset_state (
    asset_id         text PRIMARY KEY REFERENCES assets (id) ON DELETE CASCADE,
    position         geography (Point, 4326),
    speed            double precision,
    heading          double precision,
    health           text,
    connection_state text NOT NULL DEFAULT 'connected'
                     CHECK (connection_state IN ('connected', 'degraded', 'disconnected', 'unknown')),
    last_seen_at     timestamptz,
    updated_at       timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX asset_state_position_idx ON asset_state USING gist (position);

-- Telemetry history: append-oriented raw samples.
CREATE TABLE asset_telemetry (
    id               text PRIMARY KEY,
    message_id       text,
    asset_id         text NOT NULL REFERENCES assets (id) ON DELETE CASCADE,
    source_id        text REFERENCES sources (id),
    observed_at      timestamptz NOT NULL,
    received_at      timestamptz NOT NULL DEFAULT now(),
    position         geography (Point, 4326),
    speed            double precision,
    heading          double precision,
    health           text,
    connection_state text,
    payload          jsonb NOT NULL DEFAULT '{}'::jsonb,
    created_at       timestamptz NOT NULL DEFAULT now()
);

CREATE UNIQUE INDEX asset_telemetry_message_id_key
    ON asset_telemetry (message_id) WHERE message_id IS NOT NULL;
CREATE INDEX asset_telemetry_asset_time_idx ON asset_telemetry (asset_id, observed_at DESC);
CREATE INDEX asset_telemetry_source_idx ON asset_telemetry (source_id);

-- +goose Down
DROP TABLE asset_telemetry;
DROP TABLE asset_state;
DROP TABLE assets;
