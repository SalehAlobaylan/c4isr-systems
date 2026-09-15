-- +goose Up
CREATE TABLE commands (
    id             text PRIMARY KEY,
    asset_id       text NOT NULL REFERENCES assets (id),
    mission_id     text REFERENCES missions (id),
    incident_id    text REFERENCES incidents (id),
    type           text NOT NULL,
    payload        jsonb NOT NULL DEFAULT '{}'::jsonb,
    state          text NOT NULL DEFAULT 'CREATED'
                   CHECK (state IN ('CREATED', 'QUEUED', 'SENT', 'ACKNOWLEDGED', 'COMPLETED',
                                    'REJECTED', 'FAILED', 'TIMED_OUT', 'CANCELLED')),
    created_by     text,
    correlation_id text,
    created_at     timestamptz NOT NULL DEFAULT now(),
    updated_at     timestamptz NOT NULL DEFAULT now(),
    queued_at      timestamptz,
    sent_at        timestamptz,
    acknowledged_at timestamptz,
    completed_at   timestamptz,
    failure_reason text
);

CREATE INDEX commands_asset_idx ON commands (asset_id, created_at DESC);
CREATE INDEX commands_state_idx ON commands (state, created_at DESC);
CREATE INDEX commands_mission_idx ON commands (mission_id);

-- +goose Down
DROP TABLE commands;
