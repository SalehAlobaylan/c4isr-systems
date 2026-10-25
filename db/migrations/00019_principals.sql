-- +goose Up
ALTER TABLE audit_events DROP CONSTRAINT audit_events_actor_type_check;
ALTER TABLE audit_events ADD CONSTRAINT audit_events_actor_type_check CHECK(actor_type IN ('SYSTEM','OPERATOR','SCENARIO','AI','SERVICE'));
CREATE TABLE workspaces (id text PRIMARY KEY);
CREATE TABLE environments (
    workspace_id text NOT NULL REFERENCES workspaces(id),
    id text NOT NULL,
    PRIMARY KEY (workspace_id,id)
);
CREATE TABLE data_scopes (
    id text PRIMARY KEY,
    workspace_id text NOT NULL,
    environment_id text NOT NULL,
    partition text NOT NULL CHECK (partition IN ('PUBLIC_EVIDENCE','EXERCISE_INPUT','EXERCISE_OBSERVATION','EXERCISE_TRUTH')),
    run_id text NOT NULL DEFAULT '',
    FOREIGN KEY(workspace_id,environment_id) REFERENCES environments(workspace_id,id),
    UNIQUE(workspace_id,environment_id,partition,run_id),
    CHECK ((partition='PUBLIC_EVIDENCE' AND run_id='') OR partition='EXERCISE_INPUT' OR (partition IN ('EXERCISE_OBSERVATION','EXERCISE_TRUTH') AND run_id<>'')),
    CHECK (id=workspace_id||'/'||environment_id||'/'||partition||'/'||run_id)
);
INSERT INTO workspaces VALUES ('default');
INSERT INTO environments VALUES ('default','local');
INSERT INTO data_scopes VALUES ('default/local/EXERCISE_OBSERVATION/legacy','default','local','EXERCISE_OBSERVATION','legacy');

CREATE TABLE principals (
    id text PRIMARY KEY,
    kind text NOT NULL CHECK(kind IN ('human','service')),
    operator_id text REFERENCES operators(id),
    name text NOT NULL,
    disabled_at timestamptz,
    created_at timestamptz NOT NULL DEFAULT now(),
    CHECK ((kind='human' AND operator_id IS NOT NULL) OR (kind='service' AND operator_id IS NULL))
);
CREATE TABLE principal_grants (
    id bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    principal_id text NOT NULL REFERENCES principals(id),
    workspace_id text NOT NULL,
    environment_id text NOT NULL,
    partition text NOT NULL CHECK (partition IN ('PUBLIC_EVIDENCE','EXERCISE_INPUT','EXERCISE_OBSERVATION','EXERCISE_TRUTH')),
    run_id text NOT NULL,
    permissions text[] NOT NULL,
    source_ids text[] NOT NULL DEFAULT '{}',
    FOREIGN KEY(workspace_id,environment_id) REFERENCES environments(workspace_id,id)
);
CREATE INDEX principal_grants_principal_idx ON principal_grants(principal_id);
CREATE TABLE principal_credentials (
    id text PRIMARY KEY,
    principal_id text NOT NULL REFERENCES principals(id),
    token_hash bytea NOT NULL UNIQUE,
    default_scope_id text NOT NULL REFERENCES data_scopes(id),
    expires_at timestamptz NOT NULL,
    revoked_at timestamptz,
    created_at timestamptz NOT NULL DEFAULT now()
);
CREATE TABLE authorization_denials (
    id bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    principal_id text NOT NULL,
    permission text NOT NULL,
    scope_id text NOT NULL,
    reason text NOT NULL,
    occurred_at timestamptz NOT NULL DEFAULT now()
);
CREATE TABLE authorization_changes (
    id bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    actor_id text NOT NULL,
    subject_id text NOT NULL,
    action text NOT NULL,
    occurred_at timestamptz NOT NULL DEFAULT now()
);

-- +goose Down
DROP TABLE authorization_changes;
DROP TABLE authorization_denials;
DROP TABLE principal_credentials;
DROP TABLE principal_grants;
DROP TABLE principals;
DROP TABLE data_scopes;
DROP TABLE environments;
DROP TABLE workspaces;
ALTER TABLE audit_events DROP CONSTRAINT audit_events_actor_type_check;
ALTER TABLE audit_events ADD CONSTRAINT audit_events_actor_type_check CHECK(actor_type IN ('SYSTEM','OPERATOR','SCENARIO','AI'));
