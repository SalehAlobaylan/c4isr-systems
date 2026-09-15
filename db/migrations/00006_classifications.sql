-- +goose Up
-- Classifications are hypotheses about a track, not absolute truth. Multiple
-- classifications may exist over time (and competing labels may coexist).
CREATE TABLE classifications (
    id               text PRIMARY KEY,
    track_id         text NOT NULL REFERENCES tracks (id) ON DELETE CASCADE,
    label            text NOT NULL,
    confidence       double precision CHECK (confidence IS NULL OR (confidence >= 0 AND confidence <= 1)),
    method           text NOT NULL CHECK (method IN ('SCENARIO', 'OPERATOR', 'RULE', 'ALGORITHM', 'AI')),
    source_reference text,
    created_by       text,
    created_at       timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX classifications_track_idx ON classifications (track_id, created_at DESC);

-- +goose Down
DROP TABLE classifications;
