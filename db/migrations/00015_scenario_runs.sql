-- +goose Up
-- Scenario runs are tracked so the operator can inspect and control the
-- deterministic scenario engine. The runner is an external information source
-- from the core's perspective; this table tracks control-plane state only.
CREATE TABLE scenario_runs (
    id             text PRIMARY KEY,
    scenario_name  text NOT NULL,
    seed           bigint NOT NULL,
    status         text NOT NULL DEFAULT 'RUNNING'
                   CHECK (status IN ('RUNNING', 'PAUSED', 'COMPLETED', 'STOPPED', 'FAILED')),
    playback_speed double precision NOT NULL DEFAULT 1 CHECK (playback_speed > 0),
    virtual_time_ms bigint NOT NULL DEFAULT 0,
    started_at     timestamptz NOT NULL DEFAULT now(),
    ended_at       timestamptz,
    error          text,
    created_at     timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX scenario_runs_status_idx ON scenario_runs (status, started_at DESC);

-- +goose Down
DROP TABLE scenario_runs;
