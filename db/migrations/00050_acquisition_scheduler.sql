-- +goose Up
-- Stop old processes before this forward cutover. Fence their outstanding work.
UPDATE connector_runs SET document=document||jsonb_build_object('status','INTERRUPTED','failureCode','scheduler_cutover','finishedAt',clock_timestamp()) WHERE document->>'status'='RUNNING';
UPDATE retrieval_profile_state SET lease_id=NULL,lease_until=NULL;
UPDATE imagery_capture_jobs SET status='FAILED',error_code='scheduler_cutover',progress='Upgrade fenced this attempt; inspect retained inputs before retry',lease_id=NULL,lease_until=NULL,worker_id=NULL,finished_at=clock_timestamp() WHERE status='RUNNING';
UPDATE imagery_jobs SET status='FAILED',error_code='scheduler_cutover',lease_id=NULL,lease_until=NULL,finished_at=clock_timestamp() WHERE status='RUNNING';
ALTER TABLE retrieval_profile_state ADD COLUMN query_request_id text;
ALTER TABLE retrieval_profile_state ADD COLUMN query_aoi jsonb;
ALTER TABLE retrieval_profile_state ADD COLUMN last_work_kind text NOT NULL DEFAULT 'CURRENT';
ALTER TABLE retrieval_profile_state ADD COLUMN scheduled_at timestamptz NOT NULL DEFAULT '-infinity';
CREATE OR REPLACE VIEW source_connector_state WITH(security_invoker=true) AS
 SELECT scope_id,source_id,admission_id,checkpoint,next_attempt_at,last_attempt_at,last_success_at,last_failure_code,failures,lease_id,lease_until,quota_day,requests_today,coverage_from,coverage_to,query_from,query_to,query_request_id,query_aoi,scheduled_at
 FROM retrieval_profile_state WHERE profile_key=COALESCE(NULLIF(current_setting('emad.profile_key',true),''),'default') WITH LOCAL CHECK OPTION;
CREATE TABLE acquisition_requests (
 id text PRIMARY KEY,scope_id text NOT NULL DEFAULT NULLIF(current_setting('emad.scope_id',true),''),source_id text NOT NULL,
 requester_id text NOT NULL,request_key text NOT NULL,input_hash text NOT NULL,profile_key text NOT NULL,profile_revision_id text NOT NULL,
 scope_snapshot_id text NOT NULL,required_sources jsonb NOT NULL,kind text NOT NULL CHECK(kind IN('ACQUISITION','COVERAGE_REVIEW')),
 status text NOT NULL,document jsonb NOT NULL,created_at timestamptz NOT NULL DEFAULT clock_timestamp(),updated_at timestamptz NOT NULL DEFAULT clock_timestamp(),
 UNIQUE(scope_id,requester_id,request_key),UNIQUE(id,source_id,scope_id),
 FOREIGN KEY(profile_revision_id,source_id,scope_id) REFERENCES retrieval_profile_revisions(id,source_id,scope_id),
 FOREIGN KEY(scope_snapshot_id,scope_id) REFERENCES analyst_scope_snapshots(id,scope_id)
);
CREATE TABLE acquisition_parts (
 id text PRIMARY KEY,scope_id text NOT NULL DEFAULT NULLIF(current_setting('emad.scope_id',true),''),source_id text NOT NULL,request_id text NOT NULL,
 ordinal int NOT NULL,document jsonb NOT NULL,status text NOT NULL DEFAULT 'QUEUED',attempts int NOT NULL DEFAULT 0,
 next_attempt_at timestamptz NOT NULL DEFAULT clock_timestamp(),lease_id text,lease_until timestamptz,run_id text,
 UNIQUE(request_id,ordinal),FOREIGN KEY(request_id,source_id,scope_id) REFERENCES acquisition_requests(id,source_id,scope_id)
);
CREATE INDEX acquisition_due ON acquisition_parts(scope_id,source_id,next_attempt_at) WHERE status IN('QUEUED','WAITING','RUNNING');
CREATE TABLE retrieval_receipts (
 run_id text PRIMARY KEY,scope_id text NOT NULL DEFAULT NULLIF(current_setting('emad.scope_id',true),''),source_id text NOT NULL,profile_revision_id text NOT NULL,
 request_id text,document jsonb NOT NULL,
 FOREIGN KEY(run_id,source_id,scope_id) REFERENCES connector_runs(id,source_id,scope_id),
 FOREIGN KEY(profile_revision_id,source_id,scope_id) REFERENCES retrieval_profile_revisions(id,source_id,scope_id)
);
-- Opaque account budgets are a control plane. emad_data has no direct access,
-- including to aggregate usage from another resource or hidden source.
CREATE TABLE provider_budget_state(group_key text PRIMARY KEY,day date NOT NULL,spent bigint NOT NULL DEFAULT 0,next_at timestamptz NOT NULL DEFAULT 'epoch',policy_hash text NOT NULL);
ALTER TABLE provider_budget_state ADD COLUMN reported_remaining bigint;
CREATE TABLE provider_cost_reservations(id text PRIMARY KEY,group_key text NOT NULL,run_id text NOT NULL,source_id text NOT NULL,scope_id text NOT NULL,cost bigint NOT NULL,created_at timestamptz NOT NULL,lease_until timestamptz NOT NULL,state text NOT NULL,credits_remaining bigint);
CREATE INDEX provider_cost_inflight ON provider_cost_reservations(group_key,lease_until) WHERE state='UNCERTAIN';
CREATE TABLE resource_capacity_reservations(id text PRIMARY KEY,scope_id text NOT NULL,source_id text NOT NULL,kind text NOT NULL,estimated_bytes bigint NOT NULL CHECK(estimated_bytes>=0),lease_until timestamptz NOT NULL,settled_at timestamptz,actual_bytes bigint);
CREATE INDEX capacity_pending ON resource_capacity_reservations(scope_id,lease_until) WHERE settled_at IS NULL;
-- +goose StatementBegin
DO $$ DECLARE t text; BEGIN
 FOREACH t IN ARRAY ARRAY['acquisition_requests','acquisition_parts','retrieval_receipts'] LOOP
  EXECUTE format('ALTER TABLE %I ENABLE ROW LEVEL SECURITY',t);
  EXECUTE format('ALTER TABLE %I FORCE ROW LEVEL SECURITY',t);
  EXECUTE format('GRANT SELECT,INSERT ON %I TO emad_data',t);
  EXECUTE format('CREATE POLICY %I ON %I TO emad_data USING(scope_id=NULLIF(current_setting(''emad.scope_id'',true),'''') AND emad_scope_sources_visible(jsonb_build_array(source_id))) WITH CHECK(scope_id=NULLIF(current_setting(''emad.scope_id'',true),'''') AND emad_scope_sources_visible(jsonb_build_array(source_id)))',t||'_scope',t);
 END LOOP;
END $$;
-- +goose StatementEnd
GRANT UPDATE ON acquisition_requests,acquisition_parts TO emad_data;
CREATE POLICY acquisition_dependencies ON acquisition_requests AS RESTRICTIVE TO emad_data USING(emad_scope_sources_visible(required_sources)) WITH CHECK(emad_scope_sources_visible(required_sources));
CREATE POLICY acquisition_part_dependencies ON acquisition_parts AS RESTRICTIVE TO emad_data USING(EXISTS(SELECT 1 FROM acquisition_requests r WHERE r.id=request_id)) WITH CHECK(EXISTS(SELECT 1 FROM acquisition_requests r WHERE r.id=request_id));
CREATE TRIGGER retrieval_receipt_immutable BEFORE UPDATE OR DELETE ON retrieval_receipts FOR EACH ROW EXECUTE FUNCTION emad_immutable_public_evidence();
-- An older binary cannot increment a provider counter after the new account
-- budget is installed. Reads and historical receipts remain compatible.
-- +goose StatementBegin
CREATE FUNCTION emad_scheduler_fence() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN
 IF current_user='emad_data' AND NEW.requests_today>OLD.requests_today AND current_setting('emad.scheduler_policy',true) IS DISTINCT FROM 'durable-v1' THEN
  RAISE EXCEPTION 'scheduler-aware worker required' USING ERRCODE='42501';
 END IF;
 RETURN NEW;
END $$;
-- +goose StatementEnd
CREATE TRIGGER scheduler_counter_fence BEFORE UPDATE ON retrieval_profile_state FOR EACH ROW EXECUTE FUNCTION emad_scheduler_fence();
-- +goose StatementBegin
CREATE FUNCTION emad_scheduler_artifact_fence() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN
 IF current_user='emad_data' AND current_setting('emad.scheduler_policy',true) IS DISTINCT FROM 'durable-v1' THEN
  RAISE EXCEPTION 'capacity-aware artifact reservation required' USING ERRCODE='42501';
 END IF;
 RETURN NEW;
END $$;
CREATE FUNCTION emad_scheduler_job_fence() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN
 IF current_user='emad_data' AND NEW.status='RUNNING' AND current_setting('emad.scheduler_policy',true) IS DISTINCT FROM 'durable-v1' THEN
  RAISE EXCEPTION 'scheduler-aware worker required' USING ERRCODE='42501';
 END IF;
 RETURN NEW;
END $$;
CREATE FUNCTION emad_acquisition_frozen() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN
 IF TG_TABLE_NAME='acquisition_requests' THEN
  IF (NEW.scope_id,NEW.source_id,NEW.requester_id,NEW.request_key,NEW.input_hash,NEW.profile_key,NEW.profile_revision_id,NEW.scope_snapshot_id,NEW.required_sources,NEW.kind,NEW.document,NEW.created_at)
   IS DISTINCT FROM (OLD.scope_id,OLD.source_id,OLD.requester_id,OLD.request_key,OLD.input_hash,OLD.profile_key,OLD.profile_revision_id,OLD.scope_snapshot_id,OLD.required_sources,OLD.kind,OLD.document,OLD.created_at) THEN RAISE EXCEPTION 'acquisition intent is immutable'; END IF;
 ELSE
  IF (NEW.scope_id,NEW.source_id,NEW.request_id,NEW.ordinal,NEW.document-'reason') IS DISTINCT FROM (OLD.scope_id,OLD.source_id,OLD.request_id,OLD.ordinal,OLD.document-'reason') THEN RAISE EXCEPTION 'acquisition part input is immutable'; END IF;
 END IF;
 RETURN NEW;
END $$;
-- +goose StatementEnd
CREATE TRIGGER scheduler_artifact_fence BEFORE INSERT ON artifacts FOR EACH ROW EXECUTE FUNCTION emad_scheduler_artifact_fence();
CREATE TRIGGER capture_scheduler_fence BEFORE UPDATE ON imagery_capture_jobs FOR EACH ROW EXECUTE FUNCTION emad_scheduler_job_fence();
CREATE TRIGGER imagery_scheduler_fence BEFORE UPDATE ON imagery_jobs FOR EACH ROW EXECUTE FUNCTION emad_scheduler_job_fence();
CREATE TRIGGER acquisition_input_frozen BEFORE UPDATE ON acquisition_requests FOR EACH ROW EXECUTE FUNCTION emad_acquisition_frozen();
CREATE TRIGGER acquisition_part_frozen BEFORE UPDATE ON acquisition_parts FOR EACH ROW EXECUTE FUNCTION emad_acquisition_frozen();
-- +goose Down
-- +goose StatementBegin
DO $$ BEGIN RAISE EXCEPTION 'durable requests and charges require forward recovery'; END $$;
-- +goose StatementEnd
