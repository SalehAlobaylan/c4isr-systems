-- +goose Up
CREATE TABLE simulation_scenes(
 id text NOT NULL,scope_id text NOT NULL REFERENCES data_scopes(id) DEFAULT NULLIF(current_setting('emad.scope_id',true),''),
 name text NOT NULL,document bytea NOT NULL CHECK(octet_length(document)<=4194304),created_at timestamptz NOT NULL DEFAULT now(),
 PRIMARY KEY(id,scope_id),CHECK(split_part(scope_id,'/',3)='EXERCISE_INPUT' AND split_part(scope_id,'/',4)='')
);
CREATE TABLE simulation_runtime_runs(
 id text NOT NULL,scope_id text NOT NULL DEFAULT NULLIF(current_setting('emad.scope_id',true),''),scene_id text NOT NULL,
 configuration jsonb NOT NULL CHECK(octet_length(configuration::text)<=8192),
 status text NOT NULL CHECK(status IN('QUEUED','RUNNING','PAUSED','STOPPING','STOPPED','COMPLETED','FAILED')),
 desired text NOT NULL CHECK(desired IN('RUNNING','PAUSED','STOPPED')),
 worker_id text,lease_hash text,lease_until timestamptz,sequence integer NOT NULL DEFAULT 0,last_hash text NOT NULL DEFAULT '',
 elapsed_ms bigint NOT NULL DEFAULT 0 CHECK(elapsed_ms BETWEEN 0 AND 3600000),heartbeat_at timestamptz,
 failure_code text NOT NULL DEFAULT '',created_at timestamptz NOT NULL DEFAULT now(),
 PRIMARY KEY(id,scope_id),FOREIGN KEY(scene_id,scope_id) REFERENCES simulation_scenes(id,scope_id)
);
CREATE TABLE simulation_runtime_events(
 id bigserial PRIMARY KEY,scope_id text NOT NULL DEFAULT NULLIF(current_setting('emad.scope_id',true),''),run_id text NOT NULL,
 kind text NOT NULL CHECK(kind IN('START','ATTACH','PAUSE','RESUME','STOP','STOPPED','COMPLETED','FAILED')),
 at timestamptz NOT NULL DEFAULT now(),FOREIGN KEY(run_id,scope_id) REFERENCES simulation_runtime_runs(id,scope_id)
);
CREATE TABLE simulation_runtime_truth(
 scope_id text PRIMARY KEY REFERENCES data_scopes(id) DEFAULT NULLIF(current_setting('emad.scope_id',true),''),
 document jsonb NOT NULL CHECK(octet_length(document::text)<=65536),updated_at timestamptz NOT NULL DEFAULT now(),
 CHECK(split_part(scope_id,'/',3)='EXERCISE_TRUTH')
);
CREATE INDEX simulation_runtime_active ON simulation_runtime_runs(scope_id,status,lease_until);
-- +goose StatementBegin
DO $$ DECLARE t text; BEGIN
 FOREACH t IN ARRAY ARRAY['simulation_scenes','simulation_runtime_runs','simulation_runtime_events','simulation_runtime_truth'] LOOP
  EXECUTE format('ALTER TABLE %I ENABLE ROW LEVEL SECURITY',t);EXECUTE format('ALTER TABLE %I FORCE ROW LEVEL SECURITY',t);
  EXECUTE format('GRANT SELECT,INSERT ON %I TO emad_data',t);
  EXECUTE format($p$CREATE POLICY %I ON %I TO emad_data USING(scope_id=NULLIF(current_setting('emad.scope_id',true),'')) WITH CHECK(scope_id=NULLIF(current_setting('emad.scope_id',true),''))$p$,t||'_scope',t);
 END LOOP;
END $$;
-- +goose StatementEnd
GRANT USAGE,SELECT ON SEQUENCE simulation_runtime_events_id_seq TO emad_data;
GRANT UPDATE(status,desired,worker_id,lease_hash,lease_until,sequence,last_hash,elapsed_ms,heartbeat_at,failure_code) ON simulation_runtime_runs TO emad_data;
GRANT UPDATE(document,updated_at) ON simulation_runtime_truth TO emad_data;
CREATE TRIGGER simulation_scene_immutable BEFORE UPDATE OR DELETE ON simulation_scenes FOR EACH ROW EXECUTE FUNCTION emad_immutable_public_evidence();
CREATE TRIGGER simulation_runtime_event_immutable BEFORE UPDATE OR DELETE ON simulation_runtime_events FOR EACH ROW EXECUTE FUNCTION emad_immutable_public_evidence();
-- +goose Down
-- +goose StatementBegin
DO $$ BEGIN RAISE EXCEPTION 'frozen scenes and runtime evidence require forward recovery';END $$;
-- +goose StatementEnd
