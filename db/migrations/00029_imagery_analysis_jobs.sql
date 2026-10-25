-- +goose Up
CREATE TABLE imagery_jobs (
 id text PRIMARY KEY,scope_id text NOT NULL DEFAULT NULLIF(current_setting('emad.scope_id',true),''),source_id text NOT NULL,
 kind text NOT NULL CHECK(kind IN ('PREPARE','NDVI_CHANGE')),status text NOT NULL CHECK(status IN ('QUEUED','RUNNING','CANCEL_REQUESTED','CANCELED','FAILED','SUCCEEDED')),
 request_key text NOT NULL,request_hash text NOT NULL,input jsonb NOT NULL,created_by text NOT NULL,created_at timestamptz NOT NULL DEFAULT now(),
 attempt integer NOT NULL DEFAULT 0 CHECK(attempt BETWEEN 0 AND 3),lease_id text,lease_until timestamptz,
 worker_build text,error_code text NOT NULL DEFAULT '',result jsonb,finished_at timestamptz,
 UNIQUE(scope_id,source_id,request_key),UNIQUE(id,source_id,scope_id),FOREIGN KEY(source_id,scope_id)REFERENCES sources(id,scope_id),
 CHECK(split_part(scope_id,'/',3)='PUBLIC_EVIDENCE'),CHECK(octet_length(input::text)<=65536)
);
CREATE INDEX imagery_jobs_queue_idx ON imagery_jobs(scope_id,status,created_at);
CREATE TABLE imagery_job_attempts (
 job_id text NOT NULL,scope_id text NOT NULL DEFAULT NULLIF(current_setting('emad.scope_id',true),''),source_id text NOT NULL,
 attempt integer NOT NULL,document jsonb NOT NULL,PRIMARY KEY(job_id,attempt),
 FOREIGN KEY(job_id,source_id,scope_id) REFERENCES imagery_jobs(id,source_id,scope_id)
);
CREATE TABLE imagery_preparations (
 job_id text PRIMARY KEY,scope_id text NOT NULL DEFAULT NULLIF(current_setting('emad.scope_id',true),''),source_id text NOT NULL,
 scene_id text NOT NULL,artifact_id text NOT NULL,document jsonb NOT NULL,
 FOREIGN KEY(job_id,source_id,scope_id)REFERENCES imagery_jobs(id,source_id,scope_id),
 FOREIGN KEY(scene_id,source_id,scope_id)REFERENCES imagery_scenes(id,source_id,scope_id),
 FOREIGN KEY(artifact_id,source_id,scope_id)REFERENCES artifacts(id,source_id,scope_id)
);
-- +goose StatementBegin
DO $$ DECLARE t text; BEGIN
 FOREACH t IN ARRAY ARRAY['imagery_jobs','imagery_job_attempts','imagery_preparations'] LOOP
  EXECUTE format('ALTER TABLE %I ENABLE ROW LEVEL SECURITY',t);EXECUTE format('ALTER TABLE %I FORCE ROW LEVEL SECURITY',t);
  EXECUTE format('GRANT SELECT,INSERT ON %I TO emad_data',t);
  EXECUTE format($policy$CREATE POLICY %I ON %I TO emad_data USING(scope_id=NULLIF(current_setting('emad.scope_id',true),'')) WITH CHECK(scope_id=NULLIF(current_setting('emad.scope_id',true),''))$policy$,t||'_scope',t);
  EXECUTE format('CREATE POLICY %I ON %I AS RESTRICTIVE TO emad_data USING(COALESCE(current_setting(''emad.source_ids'',true),''null'') IN (''null'',''[]'') OR source_id IN(SELECT jsonb_array_elements_text(current_setting(''emad.source_ids'',true)::jsonb))) WITH CHECK(COALESCE(current_setting(''emad.source_ids'',true),''null'') IN (''null'',''[]'') OR source_id IN(SELECT jsonb_array_elements_text(current_setting(''emad.source_ids'',true)::jsonb)))',t||'_source',t);
 END LOOP;
END $$;
-- +goose StatementEnd
GRANT UPDATE(status,attempt,lease_id,lease_until,worker_build,error_code,result,finished_at) ON imagery_jobs TO emad_data;
CREATE TRIGGER imagery_preparations_immutable BEFORE UPDATE OR DELETE ON imagery_preparations FOR EACH ROW EXECUTE FUNCTION emad_immutable_public_evidence();
CREATE TRIGGER imagery_attempts_immutable BEFORE UPDATE OR DELETE ON imagery_job_attempts FOR EACH ROW EXECUTE FUNCTION emad_immutable_public_evidence();
-- +goose StatementBegin
CREATE FUNCTION emad_imagery_job_immutable() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN
 IF (to_jsonb(NEW)-ARRAY['status','attempt','lease_id','lease_until','worker_build','error_code','result','finished_at']) IS DISTINCT FROM (to_jsonb(OLD)-ARRAY['status','attempt','lease_id','lease_until','worker_build','error_code','result','finished_at']) THEN RAISE EXCEPTION 'job inputs are immutable'; END IF;
 IF OLD.status='SUCCEEDED' OR OLD.status='CANCELED' THEN RAISE EXCEPTION 'terminal job is immutable'; END IF;
 RETURN NEW;
END $$;
-- +goose StatementEnd
CREATE TRIGGER imagery_jobs_immutable BEFORE UPDATE ON imagery_jobs FOR EACH ROW EXECUTE FUNCTION emad_imagery_job_immutable();
-- +goose Down
-- +goose StatementBegin
DO $$ BEGIN RAISE EXCEPTION 'analysis evidence requires forward recovery'; END $$;
-- +goose StatementEnd
