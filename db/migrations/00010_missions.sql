-- +goose Up
CREATE TABLE missions (
    id          text PRIMARY KEY,
    name        text NOT NULL,
    objective   text NOT NULL DEFAULT '',
    priority    text NOT NULL DEFAULT 'medium' CHECK (priority IN ('low', 'medium', 'high', 'critical')),
    status      text NOT NULL DEFAULT 'PLANNED'
                CHECK (status IN ('PLANNED', 'ACTIVE', 'COMPLETED', 'ABORTED')),
    incident_id text REFERENCES incidents (id),
    created_at  timestamptz NOT NULL DEFAULT now(),
    updated_at  timestamptz NOT NULL DEFAULT now(),
    started_at  timestamptz,
    ended_at    timestamptz
);

CREATE INDEX missions_status_idx ON missions (status, created_at DESC);
CREATE INDEX missions_incident_idx ON missions (incident_id);

CREATE TABLE mission_assets (
    mission_id  text NOT NULL REFERENCES missions (id) ON DELETE CASCADE,
    asset_id    text NOT NULL REFERENCES assets (id) ON DELETE CASCADE,
    assigned_at timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (mission_id, asset_id)
);

CREATE TABLE mission_tasks (
    id              text PRIMARY KEY,
    mission_id      text NOT NULL REFERENCES missions (id) ON DELETE CASCADE,
    type            text NOT NULL,
    description     text NOT NULL DEFAULT '',
    status          text NOT NULL DEFAULT 'PENDING'
                    CHECK (status IN ('PENDING', 'ACTIVE', 'COMPLETED', 'FAILED', 'CANCELLED')),
    target_position geography (Point, 4326),
    created_at      timestamptz NOT NULL DEFAULT now(),
    updated_at      timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX mission_tasks_mission_idx ON mission_tasks (mission_id);

-- +goose Down
DROP TABLE mission_tasks;
DROP TABLE mission_assets;
DROP TABLE missions;
