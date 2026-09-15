-- +goose Up
-- Geofences are authoritative PostGIS geometry. Severity drives alerting.
CREATE TABLE geofences (
    id         text PRIMARY KEY,
    name       text NOT NULL,
    type       text NOT NULL CHECK (type IN ('restricted', 'patrol', 'surveillance', 'exclusion', 'protected')),
    geometry   geometry (Geometry, 4326) NOT NULL,
    severity   text NOT NULL DEFAULT 'medium' CHECK (severity IN ('low', 'medium', 'high', 'critical')),
    active     boolean NOT NULL DEFAULT true,
    metadata   jsonb NOT NULL DEFAULT '{}'::jsonb,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX geofences_geometry_idx ON geofences USING gist (geometry);
CREATE INDEX geofences_active_idx ON geofences (active);

-- Materialized containment state per (geofence, track) so entry/exit events are
-- derived from persisted transitions rather than re-computed heuristically.
CREATE TABLE geofence_states (
    geofence_id text NOT NULL REFERENCES geofences (id) ON DELETE CASCADE,
    track_id    text NOT NULL REFERENCES tracks (id) ON DELETE CASCADE,
    inside      boolean NOT NULL,
    since       timestamptz NOT NULL DEFAULT now(),
    updated_at  timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (geofence_id, track_id)
);

CREATE INDEX geofence_states_track_idx ON geofence_states (track_id);

-- +goose Down
DROP TABLE geofence_states;
DROP TABLE geofences;
