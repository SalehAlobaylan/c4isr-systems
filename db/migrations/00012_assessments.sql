-- +goose Up
-- Assessments are analytical conclusions. They carry method, confidence, and
-- explicit evidence links, and never silently mutate operational state.
CREATE TABLE assessments (
    id           text PRIMARY KEY,
    subject_type text NOT NULL CHECK (subject_type IN ('track', 'asset', 'incident', 'observation', 'source')),
    subject_id   text NOT NULL,
    type         text NOT NULL,
    conclusion   text NOT NULL,
    confidence   double precision CHECK (confidence IS NULL OR (confidence >= 0 AND confidence <= 1)),
    method       text NOT NULL CHECK (method IN ('OPERATOR', 'RULE', 'ALGORITHM', 'AI')),
    created_by   text,
    created_at   timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX assessments_subject_idx ON assessments (subject_type, subject_id, created_at DESC);

CREATE TABLE assessment_evidence (
    assessment_id text NOT NULL REFERENCES assessments (id) ON DELETE CASCADE,
    evidence_type text NOT NULL,
    evidence_id   text NOT NULL,
    added_at      timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (assessment_id, evidence_type, evidence_id)
);

CREATE TABLE incident_assessments (
    incident_id   text NOT NULL REFERENCES incidents (id) ON DELETE CASCADE,
    assessment_id text NOT NULL REFERENCES assessments (id) ON DELETE CASCADE,
    added_at      timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (incident_id, assessment_id)
);

-- +goose Down
DROP TABLE incident_assessments;
DROP TABLE assessment_evidence;
DROP TABLE assessments;
