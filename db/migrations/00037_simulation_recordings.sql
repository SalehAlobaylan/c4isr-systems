-- +goose Up
-- Shared identity audit, excluded from ordinary COP grants. Never stores tokens.
CREATE TABLE simulation_authorization_history(
 id bigserial PRIMARY KEY,principal_id text NOT NULL,workspace_id text NOT NULL,environment_id text NOT NULL,
 partition text NOT NULL,run_id text NOT NULL,kind text NOT NULL,occurred_at timestamptz NOT NULL DEFAULT clock_timestamp(),
 document jsonb NOT NULL CHECK(octet_length(document::text)<=32768)
);
CREATE INDEX simulation_authorization_history_scope ON simulation_authorization_history(workspace_id,environment_id,run_id,id);
-- +goose StatementBegin
CREATE FUNCTION emad_record_authorization_change()RETURNS trigger LANGUAGE plpgsql AS $$
DECLARE record jsonb;s text;doc jsonb;p text;w text;env text;part text;run text;
BEGIN
 IF TG_OP='DELETE' THEN record:=to_jsonb(OLD);ELSE record:=to_jsonb(NEW);END IF;
 p:=record->>'principal_id';
 IF TG_TABLE_NAME='principal_grants' THEN
  w:=record->>'workspace_id';env:=record->>'environment_id';part:=record->>'partition';run:=record->>'run_id';
  doc:=jsonb_build_object('grantId',record->'id','permissions',record->'permissions','sourceIds',record->'source_ids');
 ELSE
  IF TG_OP='UPDATE' AND NEW.revoked_at IS NOT DISTINCT FROM OLD.revoked_at AND NEW.expires_at IS NOT DISTINCT FROM OLD.expires_at THEN RETURN NULL;END IF;
  s:=record->>'default_scope_id';w:=split_part(s,'/',1);env:=split_part(s,'/',2);part:=split_part(s,'/',3);run:=split_part(s,'/',4);
  doc:=jsonb_build_object('credentialId',record->'id','createdAt',record->'created_at','expiresAt',record->'expires_at','revokedAt',record->'revoked_at');
 END IF;
 IF octet_length(doc::text)>32768 THEN doc:=jsonb_build_object('captureTruncated',true,'reason','identity audit document budget exceeded');END IF;
 INSERT INTO simulation_authorization_history(principal_id,workspace_id,environment_id,partition,run_id,kind,document)VALUES(p,w,env,part,run,TG_TABLE_NAME||'.'||TG_OP,doc);
 RETURN NULL;
END $$;
-- +goose StatementEnd
CREATE TRIGGER recording_grant_history AFTER INSERT OR UPDATE OR DELETE ON principal_grants FOR EACH ROW EXECUTE FUNCTION emad_record_authorization_change();
CREATE TRIGGER recording_credential_history AFTER INSERT OR UPDATE OR DELETE ON principal_credentials FOR EACH ROW EXECUTE FUNCTION emad_record_authorization_change();
INSERT INTO simulation_authorization_history(principal_id,workspace_id,environment_id,partition,run_id,kind,document)
 SELECT principal_id,workspace_id,environment_id,partition,run_id,'principal_grants.BASELINE',CASE WHEN octet_length(doc::text)<=32768 THEN doc ELSE '{"captureTruncated":true,"reason":"migration baseline exceeds identity audit budget"}'::jsonb END
 FROM principal_grants CROSS JOIN LATERAL(SELECT jsonb_build_object('grantId',id,'permissions',permissions,'sourceIds',source_ids,'effectiveBeforeCapture','unknown; migration baseline only')AS doc)policy;
INSERT INTO simulation_authorization_history(principal_id,workspace_id,environment_id,partition,run_id,kind,document)
 SELECT principal_id,split_part(default_scope_id,'/',1),split_part(default_scope_id,'/',2),split_part(default_scope_id,'/',3),split_part(default_scope_id,'/',4),'principal_credentials.BASELINE',jsonb_build_object('credentialId',id,'createdAt',created_at,'expiresAt',expires_at,'revokedAt',revoked_at,'capture','migration baseline')FROM principal_credentials;
-- +goose StatementBegin
CREATE FUNCTION emad_record_principal_state()RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN
 INSERT INTO simulation_authorization_history(principal_id,workspace_id,environment_id,partition,run_id,kind,document)
 SELECT NEW.id,workspace_id,environment_id,partition,run_id,'principal.disabled_state',jsonb_build_object('disabledAt',NEW.disabled_at)FROM principal_grants WHERE principal_id=NEW.id;
 RETURN NULL;
END $$;
-- +goose StatementEnd
CREATE TRIGGER recording_principal_state AFTER UPDATE OF disabled_at ON principals FOR EACH ROW WHEN(NEW.disabled_at IS DISTINCT FROM OLD.disabled_at)EXECUTE FUNCTION emad_record_principal_state();
CREATE TRIGGER authorization_recording_immutable BEFORE UPDATE OR DELETE ON simulation_authorization_history FOR EACH ROW EXECUTE FUNCTION emad_immutable_public_evidence();
-- Journals are separate from mutable COP projections and the gateway outbox.
CREATE TABLE simulation_recording_limits(
 scope_id text PRIMARY KEY REFERENCES data_scopes(id) DEFAULT NULLIF(current_setting('emad.scope_id',true),''),
 event_count integer NOT NULL DEFAULT 0,bytes bigint NOT NULL DEFAULT 0,truncated boolean NOT NULL DEFAULT false,
 CHECK(split_part(scope_id,'/',3) IN('EXERCISE_OBSERVATION','EXERCISE_TRUTH'))
);
CREATE TABLE simulation_recording_events(
 scope_id text NOT NULL REFERENCES simulation_recording_limits(scope_id) DEFAULT NULLIF(current_setting('emad.scope_id',true),''),
 sequence integer NOT NULL,elapsed_ms bigint NOT NULL,received_at timestamptz NOT NULL DEFAULT clock_timestamp(),
 kind text NOT NULL,entity text NOT NULL,operation text NOT NULL CHECK(operation IN('INSERT','UPDATE','DELETE')),
 document jsonb NOT NULL CHECK(octet_length(document::text)<=4194304),PRIMARY KEY(scope_id,sequence),
 CHECK(elapsed_ms BETWEEN 0 AND 3600000)
);
CREATE TABLE simulation_recordings(
 scope_id text PRIMARY KEY REFERENCES data_scopes(id) DEFAULT NULLIF(current_setting('emad.scope_id',true),''),
 status text NOT NULL CHECK(status IN('QUEUED','PROCESSING','READY','FAILED')),
 requested_by text NOT NULL,requested_at timestamptz NOT NULL DEFAULT now(),lease_until timestamptz,
 attempts integer NOT NULL DEFAULT 0 CHECK(attempts BETWEEN 0 AND 3),
 manifest jsonb,error_code text NOT NULL DEFAULT '',retain_until timestamptz NOT NULL DEFAULT now()+interval '90 days',
 CHECK(split_part(scope_id,'/',3) IN('EXERCISE_OBSERVATION','EXERCISE_TRUTH'))
);
CREATE TABLE simulation_evaluations(
 scope_id text PRIMARY KEY REFERENCES data_scopes(id) DEFAULT NULLIF(current_setting('emad.scope_id',true),''),
 document jsonb NOT NULL CHECK(octet_length(document::text)<=262144),created_at timestamptz NOT NULL DEFAULT now(),
 CHECK(split_part(scope_id,'/',3)='EXERCISE_TRUTH')
);
CREATE TABLE simulation_released_reports(
 scope_id text PRIMARY KEY REFERENCES data_scopes(id) DEFAULT NULLIF(current_setting('emad.scope_id',true),''),
 document jsonb NOT NULL CHECK(octet_length(document::text)<=32768),created_at timestamptz NOT NULL DEFAULT now(),
 CHECK(split_part(scope_id,'/',3)='EXERCISE_OBSERVATION')
);
-- +goose StatementBegin
DO $$ DECLARE t text; BEGIN
 FOREACH t IN ARRAY ARRAY['simulation_recording_limits','simulation_recording_events','simulation_recordings','simulation_evaluations','simulation_released_reports'] LOOP
  EXECUTE format('ALTER TABLE %I ENABLE ROW LEVEL SECURITY',t);EXECUTE format('ALTER TABLE %I FORCE ROW LEVEL SECURITY',t);
  EXECUTE format('GRANT SELECT,INSERT ON %I TO emad_data',t);
  EXECUTE format($p$CREATE POLICY %I ON %I TO emad_data USING(scope_id=NULLIF(current_setting('emad.scope_id',true),''))WITH CHECK(scope_id=NULLIF(current_setting('emad.scope_id',true),''))$p$,t||'_scope',t);
 END LOOP;
END $$;
CREATE FUNCTION emad_record_event(s text,ms bigint,k text,entity text,op text,doc jsonb)RETURNS void LANGUAGE plpgsql AS $$
DECLARE n integer;b bigint;stop boolean;size integer;
BEGIN
 INSERT INTO simulation_recording_limits(scope_id)VALUES(s)ON CONFLICT DO NOTHING;
 SELECT event_count,bytes,truncated INTO n,b,stop FROM simulation_recording_limits WHERE scope_id=s FOR UPDATE;
 IF stop THEN RETURN;END IF;
 size:=octet_length(doc::text);
 IF n>=100000 OR b+size>134217728 OR size>(CASE WHEN k='configuration' THEN 4194304 ELSE 262144 END) THEN
  UPDATE simulation_recording_limits SET truncated=true WHERE scope_id=s;RETURN;
 END IF;
 INSERT INTO simulation_recording_events(scope_id,sequence,elapsed_ms,kind,entity,operation,document)VALUES(s,n+1,ms,k,entity,op,doc);
 UPDATE simulation_recording_limits SET event_count=n+1,bytes=b+size WHERE scope_id=s;
END $$;
-- A fixed allowlist prevents newly added domain fields from leaking into replay.
CREATE FUNCTION emad_record_operator_change()RETURNS trigger LANGUAGE plpgsql AS $$
DECLARE raw jsonb;safe jsonb;s text;ms bigint;k text;v jsonb;obs text;entity text;
BEGIN
 IF TG_OP='DELETE' THEN raw:=to_jsonb(OLD);ELSE raw:=to_jsonb(NEW);END IF;
 s:=raw->>'scope_id';
 IF split_part(s,'/',3)<>'EXERCISE_OBSERVATION' THEN RETURN NULL;END IF;
 SELECT elapsed_ms INTO ms FROM native_exercises WHERE scope_id=s;
 IF NOT FOUND THEN RETURN NULL;END IF;
 SELECT COALESCE(jsonb_object_agg(key,value),'{}')INTO safe FROM jsonb_each(raw)WHERE key=ANY(ARRAY[
 'id','public_id','source_id','track_id','asset_id','mission_id','incident_id','command_id','task_id','alert_id','geofence_id','observation_id',
 'type','observation_type','semantic_kind','status','state','severity','priority','name','title','description','objective','label','confidence','method',
 'speed','heading','health','connection_state','first_seen_at','last_seen_at','observed_at','received_at','processed_at','created_at','updated_at','closed_at',
 'queued_at','sent_at','acknowledged_at','completed_at','resolved_at','started_at','ended_at','assigned_at','epoch','elapsed_ms','desired','scale','runtime_version']);
 IF raw->>'position' IS NOT NULL THEN safe:=safe||jsonb_build_object('position',ST_AsGeoJSON((raw->>'position')::geography::geometry)::jsonb);END IF;
 IF raw->>'geometry' IS NOT NULL THEN safe:=safe||jsonb_build_object('geometry',ST_AsGeoJSON((raw->>'geometry')::geometry)::jsonb);END IF;
 IF raw->>'target_position' IS NOT NULL THEN safe:=safe||jsonb_build_object('target_position',ST_AsGeoJSON((raw->>'target_position')::geography::geometry)::jsonb);END IF;
 IF TG_TABLE_NAME='observations' THEN safe:=(safe-'public_id')||jsonb_build_object('id',raw->>'public_id');END IF;
 IF safe ? 'observation_id' THEN
  SELECT public_id INTO obs FROM observations WHERE scope_id=s AND id=raw->>'observation_id';
  safe:=safe||jsonb_build_object('observation_id',obs);
 END IF;
 IF TG_TABLE_NAME='sources' THEN safe:=safe||'{"name":"Synthetic exercise observer"}';END IF;
 IF TG_TABLE_NAME='geofences' THEN safe:=safe||'{"name":"Exercise area"}';END IF;
 IF TG_TABLE_NAME='alerts' THEN safe:=safe||'{"title":"Reported evidence matched Exercise area"}';END IF;
 -- Quality is numeric only; arbitrary retained input maps never enter this log.
 IF TG_TABLE_NAME='observations' THEN
  SELECT COALESCE(jsonb_object_agg(key,value),'{}')INTO v FROM jsonb_each(COALESCE(raw->'quality','{}'))WHERE key IN('confidence','horizontalAccuracyM')AND jsonb_typeof(value)='number';
  safe:=safe||jsonb_build_object('quality',v);
 END IF;
 entity:=COALESCE(safe->>'id',concat_ws('|',safe->>'track_id',safe->>'asset_id',safe->>'mission_id',safe->>'incident_id',safe->>'observation_id',safe->>'alert_id'));
 -- Native sample IDs are lease-keyed random references, never hidden actor IDs.
 IF TG_TABLE_NAME='observations' THEN entity:=raw->>'id';END IF;
 IF TG_TABLE_NAME='native_exercises' THEN entity:='clock';END IF;
 PERFORM emad_record_event(s,ms,TG_TABLE_NAME,entity,TG_OP,safe);
 RETURN NULL;
END $$;
DO $$ DECLARE t text; BEGIN
 FOREACH t IN ARRAY ARRAY['native_exercises','sources','assets','asset_state','asset_telemetry','observations','tracks','track_state','track_observations','track_history','classifications','geofences','alerts','incidents','incident_alerts','incident_tracks','incident_assets','incident_observations','missions','mission_assets','mission_tasks','commands'] LOOP
  EXECUTE format('CREATE TRIGGER recording_change AFTER INSERT OR UPDATE OR DELETE ON %I FOR EACH ROW EXECUTE FUNCTION emad_record_operator_change()',t);
 END LOOP;
END $$;
-- Existing runs cannot acquire complete history retroactively. Start a new run.
DO $$ DECLARE t text; BEGIN
 FOREACH t IN ARRAY ARRAY['simulation_recording_events','simulation_evaluations','simulation_released_reports'] LOOP
  EXECUTE format('CREATE TRIGGER recording_immutable BEFORE UPDATE OR DELETE ON %I FOR EACH ROW EXECUTE FUNCTION emad_immutable_public_evidence()',t);
 END LOOP;
END $$;
-- +goose StatementEnd
GRANT UPDATE(event_count,bytes,truncated)ON simulation_recording_limits TO emad_data;
GRANT UPDATE(status,lease_until,attempts,manifest,error_code)ON simulation_recordings TO emad_data;
-- +goose Down
-- +goose StatementBegin
DO $$ BEGIN RAISE EXCEPTION 'retained recordings require forward recovery';END $$;
-- +goose StatementEnd
