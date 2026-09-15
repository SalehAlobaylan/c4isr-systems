-- +goose Up
CREATE TABLE alerts (
    id              text PRIMARY KEY,
    type            text NOT NULL,
    severity        text NOT NULL CHECK (severity IN ('low', 'medium', 'high', 'critical')),
    state           text NOT NULL DEFAULT 'ACTIVE' CHECK (state IN ('ACTIVE', 'ACKNOWLEDGED', 'RESOLVED')),
    title           text NOT NULL,
    message         text,
    -- Provenance of the rule that produced this alert.
    source_reference jsonb NOT NULL DEFAULT '{}'::jsonb,
    track_id        text REFERENCES tracks (id),
    asset_id        text REFERENCES assets (id),
    geofence_id     text REFERENCES geofences (id),
    incident_id     text,
    created_at      timestamptz NOT NULL DEFAULT now(),
    updated_at      timestamptz NOT NULL DEFAULT now(),
    acknowledged_at timestamptz,
    acknowledged_by text,
    resolved_at     timestamptz,
    resolved_by     text
);

CREATE INDEX alerts_state_idx ON alerts (state, created_at DESC);
CREATE INDEX alerts_track_idx ON alerts (track_id);
CREATE INDEX alerts_geofence_idx ON alerts (geofence_id);
CREATE INDEX alerts_incident_idx ON alerts (incident_id);

-- +goose Down
DROP TABLE alerts;
