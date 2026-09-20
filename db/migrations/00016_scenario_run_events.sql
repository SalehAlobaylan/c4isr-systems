-- +goose Up
-- Durable scenario action state makes terminal inspection independent of the
-- in-memory engine and preserves evidence for stopped or failed runs.
ALTER TABLE scenario_runs
    ADD COLUMN last_action text,
    ADD COLUMN last_action_at_ms bigint NOT NULL DEFAULT 0,
    ADD COLUMN action_error text;

CREATE TABLE scenario_run_events (
    run_id          text NOT NULL REFERENCES scenario_runs (id) ON DELETE CASCADE,
    sequence        integer NOT NULL CHECK (sequence > 0),
    virtual_time_ms bigint NOT NULL CHECK (virtual_time_ms >= 0),
    action_name     text NOT NULL,
    status          text NOT NULL CHECK (status IN ('pending', 'running', 'completed', 'failed', 'skipped')),
    error           text,
    created_at      timestamptz NOT NULL DEFAULT now(),
    updated_at      timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (run_id, sequence)
);

CREATE INDEX scenario_run_events_order_idx
    ON scenario_run_events (run_id, virtual_time_ms, sequence);

-- +goose Down
DROP TABLE scenario_run_events;
ALTER TABLE scenario_runs
    DROP COLUMN action_error,
    DROP COLUMN last_action_at_ms,
    DROP COLUMN last_action;
