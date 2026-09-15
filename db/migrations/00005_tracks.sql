-- +goose Up
-- Tracks are operational interpretations built from observations. The track
-- state is a projection; the supporting observations remain primary evidence.

CREATE TABLE tracks (
    id            text PRIMARY KEY,
    external_ref  text,
    status        text NOT NULL DEFAULT 'active' CHECK (status IN ('active', 'lost', 'closed')),
    first_seen_at timestamptz NOT NULL,
    last_seen_at  timestamptz NOT NULL,
    metadata      jsonb NOT NULL DEFAULT '{}'::jsonb,
    created_at    timestamptz NOT NULL DEFAULT now(),
    updated_at    timestamptz NOT NULL DEFAULT now(),
    closed_at     timestamptz
);

CREATE UNIQUE INDEX tracks_external_ref_key ON tracks (external_ref) WHERE external_ref IS NOT NULL;
CREATE INDEX tracks_status_idx ON tracks (status, last_seen_at DESC);

CREATE TABLE track_state (
    track_id   text PRIMARY KEY REFERENCES tracks (id) ON DELETE CASCADE,
    position   geography (Point, 4326),
    speed      double precision,
    heading    double precision,
    updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX track_state_position_idx ON track_state USING gist (position);

-- Provenance: which observations support which track. One observation belongs
-- to at most one track; the observation row itself is never mutated by this.
CREATE TABLE track_observations (
    track_id      text NOT NULL REFERENCES tracks (id) ON DELETE CASCADE,
    observation_id text NOT NULL UNIQUE REFERENCES observations (id) ON DELETE CASCADE,
    source_id     text NOT NULL REFERENCES sources (id),
    observed_at   timestamptz NOT NULL,
    position      geography (Point, 4326),
    created_at    timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (track_id, observation_id)
);

CREATE INDEX track_observations_track_idx ON track_observations (track_id, observed_at);

-- Movement history for path rendering and replay.
CREATE TABLE track_history (
    id         text PRIMARY KEY,
    track_id   text NOT NULL REFERENCES tracks (id) ON DELETE CASCADE,
    observed_at timestamptz NOT NULL,
    position   geography (Point, 4326),
    speed      double precision,
    heading    double precision,
    created_at timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX track_history_track_idx ON track_history (track_id, observed_at DESC);

-- +goose Down
DROP TABLE track_history;
DROP TABLE track_observations;
DROP TABLE track_state;
DROP TABLE tracks;
