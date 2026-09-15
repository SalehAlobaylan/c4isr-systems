-- +goose Up
-- Audit events record what the system and operators did, when, and why it was
-- allowed to happen. Derived facts remain traceable to their origin.
CREATE TABLE audit_events (
    id             text PRIMARY KEY,
    occurred_at    timestamptz NOT NULL,
    actor_type     text NOT NULL CHECK (actor_type IN ('SYSTEM', 'OPERATOR', 'SCENARIO', 'AI')),
    actor_id       text,
    action         text NOT NULL,
    subject_type   text,
    subject_id     text,
    correlation_id text,
    data           jsonb NOT NULL DEFAULT '{}'::jsonb,
    created_at     timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX audit_events_occurred_idx ON audit_events (occurred_at DESC);
CREATE INDEX audit_events_subject_idx ON audit_events (subject_type, subject_id, occurred_at DESC);
CREATE INDEX audit_events_action_idx ON audit_events (action, occurred_at DESC);
CREATE INDEX audit_events_correlation_idx ON audit_events (correlation_id) WHERE correlation_id IS NOT NULL;

-- +goose Down
DROP TABLE audit_events;
