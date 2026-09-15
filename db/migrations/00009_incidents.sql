-- +goose Up
CREATE TABLE incidents (
    id                text PRIMARY KEY,
    title             text NOT NULL,
    description       text NOT NULL DEFAULT '',
    priority          text NOT NULL DEFAULT 'medium' CHECK (priority IN ('low', 'medium', 'high', 'critical')),
    status            text NOT NULL DEFAULT 'OPEN'
                      CHECK (status IN ('OPEN', 'ACKNOWLEDGED', 'INVESTIGATING', 'RESPONDING', 'RESOLVED', 'CLOSED')),
    assigned_operator text,
    created_at        timestamptz NOT NULL DEFAULT now(),
    updated_at        timestamptz NOT NULL DEFAULT now(),
    resolved_at       timestamptz,
    closed_at         timestamptz
);

CREATE INDEX incidents_status_idx ON incidents (status, created_at DESC);

ALTER TABLE alerts
    ADD CONSTRAINT alerts_incident_fk FOREIGN KEY (incident_id) REFERENCES incidents (id);

-- Evidence relations. An incident is an operational workspace over evidence;
-- the underlying records are never owned by it.
CREATE TABLE incident_alerts (
    incident_id text NOT NULL REFERENCES incidents (id) ON DELETE CASCADE,
    alert_id    text NOT NULL REFERENCES alerts (id) ON DELETE CASCADE,
    added_at    timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (incident_id, alert_id)
);

CREATE TABLE incident_tracks (
    incident_id text NOT NULL REFERENCES incidents (id) ON DELETE CASCADE,
    track_id    text NOT NULL REFERENCES tracks (id) ON DELETE CASCADE,
    added_at    timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (incident_id, track_id)
);

CREATE TABLE incident_assets (
    incident_id text NOT NULL REFERENCES incidents (id) ON DELETE CASCADE,
    asset_id    text NOT NULL REFERENCES assets (id) ON DELETE CASCADE,
    added_at    timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (incident_id, asset_id)
);

CREATE TABLE incident_observations (
    incident_id    text NOT NULL REFERENCES incidents (id) ON DELETE CASCADE,
    observation_id text NOT NULL REFERENCES observations (id) ON DELETE CASCADE,
    added_at       timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (incident_id, observation_id)
);

-- +goose Down
DROP TABLE incident_observations;
DROP TABLE incident_assets;
DROP TABLE incident_tracks;
DROP TABLE incident_alerts;
ALTER TABLE alerts DROP CONSTRAINT alerts_incident_fk;
DROP TABLE incidents;
