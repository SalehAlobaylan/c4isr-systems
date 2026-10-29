-- +goose Up
ALTER TABLE analyst_read_snapshots ADD COLUMN snapshot_kind text NOT NULL DEFAULT 'map' CHECK(snapshot_kind IN('map','catalog'));
CREATE TABLE analyst_read_cursors(
 id text PRIMARY KEY,snapshot_id text NOT NULL,
 scope_id text NOT NULL DEFAULT NULLIF(current_setting('emad.scope_id',true),''),
 principal_id text NOT NULL DEFAULT NULLIF(current_setting('emad.principal_id',true),''),
 query_key text NOT NULL,row_offset int NOT NULL CHECK(row_offset>=0),page_size int NOT NULL CHECK(page_size BETWEEN 1 AND 300),
 UNIQUE(snapshot_id,query_key,row_offset,page_size),
 FOREIGN KEY(snapshot_id,scope_id) REFERENCES analyst_read_snapshots(id,scope_id) ON DELETE CASCADE
);
ALTER TABLE analyst_read_cursors ENABLE ROW LEVEL SECURITY;
ALTER TABLE analyst_read_cursors FORCE ROW LEVEL SECURITY;
GRANT SELECT,INSERT ON analyst_read_cursors TO emad_data;
CREATE POLICY read_cursor_owner ON analyst_read_cursors TO emad_data
 USING(scope_id=NULLIF(current_setting('emad.scope_id',true),'') AND principal_id=NULLIF(current_setting('emad.principal_id',true),''))
 WITH CHECK(scope_id=NULLIF(current_setting('emad.scope_id',true),'') AND principal_id=NULLIF(current_setting('emad.principal_id',true),'') AND EXISTS(SELECT 1 FROM analyst_read_snapshots s WHERE s.id=snapshot_id AND s.scope_id=analyst_read_cursors.scope_id));
CREATE TABLE imagery_request_intents(
 scope_id text NOT NULL DEFAULT NULLIF(current_setting('emad.scope_id',true),''),operation text NOT NULL CHECK(operation IN('capture','analysis')),
 requester_id text NOT NULL,request_key text NOT NULL,input_hash text NOT NULL,entity_id text NOT NULL,
 PRIMARY KEY(scope_id,operation,requester_id,request_key),UNIQUE(scope_id,operation,entity_id)
);
ALTER TABLE imagery_request_intents ENABLE ROW LEVEL SECURITY;
ALTER TABLE imagery_request_intents FORCE ROW LEVEL SECURITY;
GRANT SELECT,INSERT ON imagery_request_intents TO emad_data;
CREATE POLICY imagery_intent_owner ON imagery_request_intents TO emad_data
 USING(scope_id=NULLIF(current_setting('emad.scope_id',true),'') AND requester_id=COALESCE(NULLIF(current_setting('emad.human_id',true),''),NULLIF(current_setting('emad.principal_id',true),'')))
 WITH CHECK(scope_id=NULLIF(current_setting('emad.scope_id',true),'') AND requester_id=COALESCE(NULLIF(current_setting('emad.human_id',true),''),NULLIF(current_setting('emad.principal_id',true),'')));
CREATE TRIGGER imagery_intent_immutable BEFORE UPDATE OR DELETE ON imagery_request_intents FOR EACH ROW EXECUTE FUNCTION emad_immutable_public_evidence();
-- Bounded input processing does not disclose human case text or grant sources.
-- Human readers of linked work must still have effective case membership.
-- +goose StatementBegin
CREATE FUNCTION emad_case_work_visible(case_key text) RETURNS boolean LANGUAGE sql STABLE SECURITY INVOKER AS $$
 SELECT case_key IS NULL OR case_key='' OR current_setting('emad.imagery_worker',true)='true' OR emad_investigation_visible(case_key,NULLIF(current_setting('emad.scope_id',true),''))
$$;
-- +goose StatementEnd
CREATE POLICY capture_case_link ON imagery_capture_jobs AS RESTRICTIVE TO emad_data
 USING(emad_case_work_visible(input#>>'{caseRevision,caseId}')) WITH CHECK(emad_case_work_visible(input#>>'{caseRevision,caseId}'));
CREATE POLICY job_case_link ON imagery_jobs AS RESTRICTIVE TO emad_data
 USING(emad_case_work_visible(input#>>'{caseRevision,caseId}')) WITH CHECK(emad_case_work_visible(input#>>'{caseRevision,caseId}'));
-- Separate current queue capacity from immutable lifetime history. Resource
-- byte reservations and the stricter per-source byte budgets remain installed.
-- +goose StatementBegin
CREATE OR REPLACE FUNCTION emad_imagery_queue_available(queue_kind text, measurement_source text) RETURNS boolean
 LANGUAGE sql STABLE SECURITY DEFINER SET search_path=pg_catalog,public SET row_security=off AS $$
 SELECT CASE WHEN NULLIF(current_setting('emad.scope_id',true),'') IS NULL THEN false
 WHEN COALESCE(current_setting('emad.source_ids',true),'null') NOT IN ('null','[]') AND NOT current_setting('emad.source_ids',true)::jsonb @> jsonb_build_array(measurement_source) THEN false
 WHEN (SELECT COALESCE(sum(size_bytes),0) FROM public.artifacts WHERE scope_id=current_setting('emad.scope_id',true) AND state<>'DELETED')
    +(SELECT COALESCE(sum(octet_length(document::text)+COALESCE(octet_length(original_record),0)),0) FROM public.public_evidence WHERE scope_id=current_setting('emad.scope_id',true))
    +(SELECT COALESCE(sum(estimated_bytes),0) FROM public.resource_capacity_reservations WHERE scope_id=current_setting('emad.scope_id',true) AND settled_at IS NULL AND lease_until>clock_timestamp())>=8589934592 THEN false
 WHEN queue_kind='capture' THEN (SELECT count(*)<4 FROM public.imagery_capture_jobs WHERE scope_id=current_setting('emad.scope_id',true) AND status IN('QUEUED','RUNNING'))
 WHEN queue_kind='job' THEN (SELECT count(*)<32 FROM public.imagery_jobs WHERE scope_id=current_setting('emad.scope_id',true) AND source_id=measurement_source AND status IN('QUEUED','RUNNING','CANCEL_REQUESTED'))
 ELSE false END
$$;
-- +goose StatementEnd
CREATE INDEX imagery_scene_scope_time ON imagery_scenes(scope_id,acquired_at DESC,id);
CREATE INDEX imagery_capture_scope_history ON imagery_capture_jobs(scope_id,created_at DESC,id);
CREATE INDEX imagery_job_scope_history ON imagery_jobs(scope_id,created_at DESC,id);
CREATE INDEX investigation_head_scope ON investigations(scope_id,head_revision_id);
-- +goose Down
-- +goose StatementBegin
DO $$ BEGIN RAISE EXCEPTION 'catalog cursors and imagery intent history require forward recovery'; END $$;
-- +goose StatementEnd
