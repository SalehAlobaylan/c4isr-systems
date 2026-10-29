-- +goose Up
-- Quiescent cutover: preserve checkpoints, windows, quota and admission bytes;
-- fence every outstanding lease so an older worker cannot publish after upgrade.
UPDATE connector_runs SET document=document||jsonb_build_object('status','INTERRUPTED','failureCode','profile_cutover','finishedAt',clock_timestamp()) WHERE document->>'status'='RUNNING';
UPDATE source_connector_state SET lease_id=NULL,lease_until=NULL;
ALTER TABLE source_connector_state RENAME TO retrieval_profile_state;
ALTER TABLE retrieval_profile_state ADD COLUMN profile_key text NOT NULL DEFAULT 'default';
ALTER TABLE retrieval_profile_state DROP CONSTRAINT source_connector_state_pkey;
ALTER TABLE retrieval_profile_state ADD PRIMARY KEY(scope_id,source_id,profile_key);
CREATE POLICY retrieval_state_profile ON retrieval_profile_state AS RESTRICTIVE TO emad_data
 USING(true)
 WITH CHECK(profile_key=COALESCE(NULLIF(current_setting('emad.profile_key',true),''),'default'));
-- Simple security-invoker compatibility view: all source-only callers address
-- default unless the new authenticated server explicitly binds a profile.
CREATE VIEW source_connector_state WITH(security_invoker=true) AS
 SELECT scope_id,source_id,admission_id,checkpoint,next_attempt_at,last_attempt_at,last_success_at,last_failure_code,failures,lease_id,lease_until,quota_day,requests_today,coverage_from,coverage_to,query_from,query_to
 FROM retrieval_profile_state WHERE profile_key=COALESCE(NULLIF(current_setting('emad.profile_key',true),''),'default') WITH LOCAL CHECK OPTION;
GRANT SELECT,INSERT,UPDATE ON source_connector_state TO emad_data;
CREATE TABLE retrieval_profiles (
 scope_id text NOT NULL DEFAULT NULLIF(current_setting('emad.scope_id',true),''),source_id text NOT NULL,profile_key text NOT NULL,head_revision_id text NOT NULL,
 PRIMARY KEY(scope_id,source_id,profile_key), FOREIGN KEY(source_id,scope_id) REFERENCES sources(id,scope_id)
);
CREATE TABLE retrieval_profile_revisions (
 id text PRIMARY KEY,scope_id text NOT NULL DEFAULT NULLIF(current_setting('emad.scope_id',true),''),source_id text NOT NULL,profile_key text NOT NULL,
 admission_id text NOT NULL UNIQUE,document jsonb NOT NULL,
 UNIQUE(id,source_id,scope_id), FOREIGN KEY(admission_id,source_id,scope_id) REFERENCES source_admissions(id,source_id,scope_id)
);
CREATE TABLE retrieval_execution_grants (
 scope_id text NOT NULL DEFAULT NULLIF(current_setting('emad.scope_id',true),''),source_id text NOT NULL,profile_revision_id text NOT NULL,
 principal_id text NOT NULL REFERENCES principals(id),enabled boolean NOT NULL,actor_id text NOT NULL,recorded_at timestamptz NOT NULL DEFAULT clock_timestamp(),
 PRIMARY KEY(profile_revision_id,principal_id),FOREIGN KEY(profile_revision_id,source_id,scope_id) REFERENCES retrieval_profile_revisions(id,source_id,scope_id)
);
CREATE TABLE retrieval_execution_events (
 id text PRIMARY KEY,scope_id text NOT NULL DEFAULT NULLIF(current_setting('emad.scope_id',true),''),source_id text NOT NULL,document jsonb NOT NULL,
 FOREIGN KEY(source_id,scope_id) REFERENCES sources(id,scope_id)
);
CREATE TABLE retrieval_occurrences (
 scope_id text NOT NULL DEFAULT NULLIF(current_setting('emad.scope_id',true),''),source_id text NOT NULL,run_id text NOT NULL,evidence_id text NOT NULL,profile_revision_id text NOT NULL,
 PRIMARY KEY(run_id,evidence_id),FOREIGN KEY(run_id,source_id,scope_id) REFERENCES connector_runs(id,source_id,scope_id),
 FOREIGN KEY(evidence_id,source_id,scope_id) REFERENCES public_evidence(id,source_id,scope_id),
 FOREIGN KEY(profile_revision_id,source_id,scope_id) REFERENCES retrieval_profile_revisions(id,source_id,scope_id)
);
-- Every legacy admission maps exactly once; no historical JSON is rewritten.
INSERT INTO retrieval_profile_revisions(id,scope_id,source_id,profile_key,admission_id,document)
 SELECT 'rpr_'||a.id,a.scope_id,a.source_id,'default',a.id,jsonb_build_object('id','rpr_'||a.id,'sourceId',a.source_id,'key','default','admissionId',a.id,'version',a.version,'meaning','Exact legacy admission mapping') FROM source_admissions a;
INSERT INTO retrieval_profiles(scope_id,source_id,profile_key,head_revision_id)
 SELECT scope_id,source_id,'default','rpr_'||admission_id FROM retrieval_profile_state;
INSERT INTO retrieval_execution_grants(scope_id,source_id,profile_revision_id,principal_id,enabled,actor_id)
 SELECT a.scope_id,a.source_id,'rpr_'||a.id,a.document->>'connectorPrincipalId',true,a.document->>'approvedBy' FROM source_admissions a JOIN principals p ON p.id=a.document->>'connectorPrincipalId';
INSERT INTO retrieval_occurrences(scope_id,source_id,run_id,evidence_id,profile_revision_id)
 SELECT e.scope_id,e.source_id,e.connector_run_id,e.id,'rpr_'||e.admission_id FROM public_evidence e;
-- +goose StatementBegin
DO $$ DECLARE t text; BEGIN
 FOREACH t IN ARRAY ARRAY['retrieval_profiles','retrieval_profile_revisions','retrieval_execution_grants','retrieval_execution_events','retrieval_occurrences'] LOOP
  EXECUTE format('ALTER TABLE %I ENABLE ROW LEVEL SECURITY',t);
  EXECUTE format('ALTER TABLE %I FORCE ROW LEVEL SECURITY',t);
  EXECUTE format('GRANT SELECT,INSERT ON %I TO emad_data',t);
  EXECUTE format('CREATE POLICY %I ON %I TO emad_data USING(scope_id=NULLIF(current_setting(''emad.scope_id'',true),'''') AND emad_scope_sources_visible(jsonb_build_array(source_id))) WITH CHECK(scope_id=NULLIF(current_setting(''emad.scope_id'',true),'''') AND emad_scope_sources_visible(jsonb_build_array(source_id)))',t||'_scope',t);
  IF t IN('retrieval_profile_revisions','retrieval_execution_events','retrieval_occurrences') THEN
   EXECUTE format('CREATE TRIGGER %I BEFORE UPDATE OR DELETE ON %I FOR EACH ROW EXECUTE FUNCTION emad_immutable_public_evidence()',t||'_immutable',t);
  END IF;
 END LOOP;
END $$;
-- +goose StatementEnd
GRANT UPDATE ON retrieval_profiles,retrieval_execution_grants TO emad_data;
-- Older policy-unaware processes may continue legacy reads, but cannot use
-- named-profile raster/raw inputs through a default-source rights decision.
-- +goose StatementBegin
CREATE FUNCTION emad_profile_reader(admission_key text) RETURNS boolean LANGUAGE sql STABLE SECURITY INVOKER AS $$
 SELECT current_setting('emad.profile_policy',true)='retrieval-v1'
 OR EXISTS(SELECT 1 FROM retrieval_profile_revisions WHERE admission_id=admission_key AND profile_key='default')
$$;
CREATE POLICY artifact_profile_reader ON artifacts AS RESTRICTIVE TO emad_data
 USING(emad_profile_reader(admission_id)) WITH CHECK(emad_profile_reader(admission_id));
CREATE POLICY evidence_profile_reader ON public_evidence AS RESTRICTIVE TO emad_data
 USING(emad_profile_reader(admission_id)) WITH CHECK(emad_profile_reader(admission_id));
CREATE POLICY scene_profile_reader ON imagery_scenes AS RESTRICTIVE TO emad_data
 USING(emad_profile_reader(admission_id)) WITH CHECK(emad_profile_reader(admission_id));
-- +goose StatementEnd
-- +goose Down
-- +goose StatementBegin
DO $$ BEGIN RAISE EXCEPTION 'profile-aware policy requires forward recovery; keep legacy workers fenced'; END $$;
-- +goose StatementEnd
