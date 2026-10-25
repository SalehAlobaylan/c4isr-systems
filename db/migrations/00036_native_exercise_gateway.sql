-- +goose Up
CREATE UNIQUE INDEX native_assets_scope_key ON assets(id,scope_id);
CREATE UNIQUE INDEX native_commands_scope_key ON commands(id,scope_id);
CREATE UNIQUE INDEX native_mission_tasks_scope_key ON mission_tasks(id,scope_id);
CREATE TABLE native_exercises(
 scope_id text PRIMARY KEY REFERENCES data_scopes(id) DEFAULT NULLIF(current_setting('emad.scope_id',true),''),
 status text NOT NULL,desired text NOT NULL,epoch timestamptz NOT NULL,elapsed_ms bigint NOT NULL DEFAULT 0,
 runtime_version text NOT NULL,scale double precision NOT NULL,lease_until timestamptz,started_at timestamptz NOT NULL DEFAULT now(),
 CHECK(split_part(scope_id,'/',3)='EXERCISE_OBSERVATION'),
 CHECK(status IN('QUEUED','RUNNING','PAUSED','STOPPING','STOPPED','COMPLETED','FAILED')),
 CHECK(desired IN('RUNNING','PAUSED','STOPPED')),CHECK(elapsed_ms BETWEEN 0 AND 3600000),CHECK(scale BETWEEN 0.1 AND 20),CHECK(runtime_version='emad-kinematic-v2')
);
CREATE TABLE native_controlled_assets(
 scope_id text NOT NULL DEFAULT NULLIF(current_setting('emad.scope_id',true),''),asset_id text NOT NULL,
 PRIMARY KEY(scope_id,asset_id),FOREIGN KEY(asset_id,scope_id)REFERENCES assets(id,scope_id),FOREIGN KEY(scope_id)REFERENCES native_exercises(scope_id)
);
CREATE TABLE native_projection_batches(
 scope_id text NOT NULL REFERENCES data_scopes(id) DEFAULT NULLIF(current_setting('emad.scope_id',true),''),sequence integer NOT NULL,
 document jsonb NOT NULL CHECK(octet_length(document::text)<=262144),projected boolean NOT NULL DEFAULT false,
 PRIMARY KEY(scope_id,sequence),CHECK(split_part(scope_id,'/',3)='EXERCISE_TRUTH')
);
CREATE TABLE native_task_dispatch(
 scope_id text NOT NULL DEFAULT NULLIF(current_setting('emad.scope_id',true),''),command_id text NOT NULL,
 issued_ms bigint NOT NULL,deadline_ms bigint NOT NULL,task_id text,finished boolean NOT NULL DEFAULT false,
 PRIMARY KEY(scope_id,command_id),FOREIGN KEY(command_id,scope_id)REFERENCES commands(id,scope_id),
 FOREIGN KEY(task_id,scope_id)REFERENCES mission_tasks(id,scope_id),FOREIGN KEY(scope_id)REFERENCES native_exercises(scope_id)
);
ALTER TABLE simulation_runtime_runs ADD COLUMN sample_count integer NOT NULL DEFAULT 0 CHECK(sample_count BETWEEN 0 AND 20000);
GRANT UPDATE(sample_count)ON simulation_runtime_runs TO emad_data;
ALTER TABLE commands ADD COLUMN request_key text;
ALTER TABLE commands ADD COLUMN request_owner text NOT NULL DEFAULT '';
CREATE UNIQUE INDEX native_command_request_key ON commands(scope_id,request_owner,request_key)WHERE request_key IS NOT NULL;
-- +goose StatementBegin
DO $$ DECLARE t text; BEGIN
 FOREACH t IN ARRAY ARRAY['native_exercises','native_controlled_assets','native_projection_batches','native_task_dispatch'] LOOP
  EXECUTE format('ALTER TABLE %I ENABLE ROW LEVEL SECURITY',t);EXECUTE format('ALTER TABLE %I FORCE ROW LEVEL SECURITY',t);
  EXECUTE format('GRANT SELECT,INSERT ON %I TO emad_data',t);
  EXECUTE format($p$CREATE POLICY %I ON %I TO emad_data USING(scope_id=NULLIF(current_setting('emad.scope_id',true),''))WITH CHECK(scope_id=NULLIF(current_setting('emad.scope_id',true),''))$p$,t||'_scope',t);
 END LOOP;
END $$;
-- +goose StatementEnd
GRANT UPDATE(status,desired,elapsed_ms,lease_until)ON native_exercises TO emad_data;
GRANT UPDATE(projected)ON native_projection_batches TO emad_data;
GRANT UPDATE(finished)ON native_task_dispatch TO emad_data;
CREATE TRIGGER native_controlled_asset_immutable BEFORE UPDATE OR DELETE ON native_controlled_assets FOR EACH ROW EXECUTE FUNCTION emad_immutable_public_evidence();
-- Native command admission is checked under the same scoped transaction as the
-- existing command insert. Metadata cannot make an unrelated asset controllable.
-- +goose StatementBegin
CREATE FUNCTION emad_native_command_admission()RETURNS trigger LANGUAGE plpgsql AS $$
DECLARE meta native_exercises; n integer; task text;
BEGIN
 SELECT * INTO meta FROM native_exercises WHERE scope_id=NEW.scope_id FOR UPDATE;
 IF NOT FOUND THEN RETURN NEW;END IF;
 IF length(NEW.type)NOT BETWEEN 1 AND 80 OR octet_length(NEW.payload::text)>8192 THEN RAISE EXCEPTION 'native command payload exceeds dispatch budget' USING ERRCODE='23514';END IF;
 IF meta.status NOT IN('RUNNING','PAUSED')OR meta.desired='STOPPED'OR meta.lease_until IS NULL OR meta.lease_until<clock_timestamp()THEN RAISE EXCEPTION 'native exercise is not accepting tasking' USING ERRCODE='23514';END IF;
 IF NOT EXISTS(SELECT 1 FROM native_controlled_assets WHERE scope_id=NEW.scope_id AND asset_id=NEW.asset_id)THEN RAISE EXCEPTION 'asset is not exercise-controlled' USING ERRCODE='23514';END IF;
 SELECT count(*) INTO n FROM native_task_dispatch WHERE scope_id=NEW.scope_id;IF n>=128 THEN RAISE EXCEPTION 'native task budget exhausted' USING ERRCODE='23514';END IF;
 IF NEW.request_owner='' OR NEW.request_key IS NULL OR length(NEW.request_key)NOT BETWEEN 1 AND 160 THEN RAISE EXCEPTION 'native command request key required' USING ERRCODE='23514';END IF;
 task:=NEW.payload->>'taskId';
 IF task IS NOT NULL AND NOT EXISTS(SELECT 1 FROM mission_tasks WHERE scope_id=NEW.scope_id AND id=task AND mission_id=NEW.mission_id)THEN RAISE EXCEPTION 'mission task does not belong to command mission' USING ERRCODE='23514';END IF;
 IF NEW.mission_id IS NOT NULL AND NOT EXISTS(SELECT 1 FROM mission_assets WHERE scope_id=NEW.scope_id AND mission_id=NEW.mission_id AND asset_id=NEW.asset_id)THEN RAISE EXCEPTION 'recipient is not assigned to mission' USING ERRCODE='23514';END IF;
 RETURN NEW;
END $$;
CREATE FUNCTION emad_native_queue_command()RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN
 INSERT INTO native_task_dispatch(scope_id,command_id,issued_ms,deadline_ms,task_id)SELECT NEW.scope_id,NEW.id,elapsed_ms,elapsed_ms+GREATEST(10000,ceil(scale*2000)::bigint),NEW.payload->>'taskId' FROM native_exercises WHERE scope_id=NEW.scope_id;
 RETURN NEW;
END $$;
-- +goose StatementEnd
CREATE TRIGGER native_command_admission BEFORE INSERT ON commands FOR EACH ROW EXECUTE FUNCTION emad_native_command_admission();
CREATE TRIGGER native_command_queue AFTER INSERT ON commands FOR EACH ROW EXECUTE FUNCTION emad_native_queue_command();
-- Cancellation is linearized against each evidence insert. A sample already
-- committed before stop remains evidence; unpublished samples are discarded.
-- +goose StatementBegin
CREATE FUNCTION emad_native_evidence_admission()RETURNS trigger LANGUAGE plpgsql AS $$
DECLARE meta native_exercises;
BEGIN
 SELECT * INTO meta FROM native_exercises WHERE scope_id=NEW.scope_id FOR SHARE;
 IF FOUND AND (meta.desired='STOPPED' OR meta.status IN('STOPPING','STOPPED','FAILED') OR meta.lease_until<clock_timestamp()) THEN
  RAISE EXCEPTION 'native exercise publication is closed' USING ERRCODE='23514';
 END IF;
 RETURN NEW;
END $$;
-- +goose StatementEnd
CREATE TRIGGER native_observation_admission BEFORE INSERT ON observations FOR EACH ROW EXECUTE FUNCTION emad_native_evidence_admission();
CREATE TRIGGER native_telemetry_admission BEFORE INSERT ON asset_telemetry FOR EACH ROW EXECUTE FUNCTION emad_native_evidence_admission();
-- +goose Down
-- +goose StatementBegin
DO $$ BEGIN RAISE EXCEPTION 'native exercise evidence requires forward recovery';END $$;
-- +goose StatementEnd
