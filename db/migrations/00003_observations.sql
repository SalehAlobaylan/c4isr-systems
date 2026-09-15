-- +goose Up
-- Observations are the evidence layer: append-oriented statements made by a
-- source about the world at a specific time. They are preserved independently
-- of any track or classification derived from them.
CREATE TABLE observations (
    id               text PRIMARY KEY,
    source_id        text NOT NULL REFERENCES sources (id),
    observation_type text NOT NULL,
    observed_at      timestamptz NOT NULL,
    received_at      timestamptz NOT NULL DEFAULT now(),
    processed_at     timestamptz,
    position         geography (Point, 4326),
    payload          jsonb NOT NULL DEFAULT '{}'::jsonb,
    quality          jsonb,
    track_hint       text,
    created_at       timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX observations_observed_at_idx ON observations (observed_at DESC);
CREATE INDEX observations_source_idx ON observations (source_id, observed_at DESC);
CREATE INDEX observations_position_idx ON observations USING gist (position);
CREATE INDEX observations_track_hint_idx ON observations (track_hint) WHERE track_hint IS NOT NULL;

-- +goose Down
DROP TABLE observations;
