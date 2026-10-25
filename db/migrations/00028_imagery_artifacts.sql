-- +goose Up
CREATE TABLE artifacts (
 id text PRIMARY KEY,scope_id text NOT NULL DEFAULT NULLIF(current_setting('emad.scope_id',true),''),source_id text NOT NULL,
 admission_id text NOT NULL,sha256 text NOT NULL CHECK(sha256~'^[0-9a-f]{64}$'),size_bytes bigint NOT NULL CHECK(size_bytes BETWEEN 1 AND 16777216),
 media_type text NOT NULL,kind text NOT NULL,object_key text NOT NULL UNIQUE,document jsonb NOT NULL,
 state text NOT NULL CHECK(state IN ('PENDING','READY','DELETING','DELETED')),retain_until timestamptz NOT NULL,
 created_at timestamptz NOT NULL DEFAULT now(),UNIQUE(id,source_id,scope_id),
 FOREIGN KEY(source_id,scope_id) REFERENCES sources(id,scope_id),
 FOREIGN KEY(admission_id,source_id,scope_id) REFERENCES source_admissions(id,source_id,scope_id),
 CHECK(split_part(scope_id,'/',3)='PUBLIC_EVIDENCE'),CHECK(octet_length(document::text)<=32768)
);
CREATE INDEX artifacts_scope_idx ON artifacts(scope_id,source_id,created_at DESC);
CREATE TABLE imagery_scenes (
 id text PRIMARY KEY,scope_id text NOT NULL DEFAULT NULLIF(current_setting('emad.scope_id',true),''),source_id text NOT NULL,
 evidence_id text NOT NULL UNIQUE,admission_id text NOT NULL,provider_id text NOT NULL,acquired_at timestamptz NOT NULL,
 geometry geometry(Geometry,4326) NOT NULL,document jsonb NOT NULL,
 UNIQUE(id,source_id,scope_id),FOREIGN KEY(evidence_id,source_id,scope_id) REFERENCES public_evidence(id,source_id,scope_id),
 CHECK(split_part(scope_id,'/',3)='PUBLIC_EVIDENCE'),CHECK(ST_IsValid(geometry) AND ST_NDims(geometry)=2 AND ST_NPoints(geometry)<=1000)
);
CREATE INDEX imagery_scenes_geo_idx ON imagery_scenes USING gist(geometry);
CREATE INDEX imagery_scenes_scope_idx ON imagery_scenes(scope_id,source_id,acquired_at DESC);
-- +goose StatementBegin
DO $$ DECLARE t text; BEGIN
 FOREACH t IN ARRAY ARRAY['artifacts','imagery_scenes'] LOOP
  EXECUTE format('ALTER TABLE %I ENABLE ROW LEVEL SECURITY',t);EXECUTE format('ALTER TABLE %I FORCE ROW LEVEL SECURITY',t);
  EXECUTE format('GRANT SELECT,INSERT ON %I TO emad_data',t);
  EXECUTE format($policy$CREATE POLICY %I ON %I TO emad_data USING(scope_id=NULLIF(current_setting('emad.scope_id',true),'')) WITH CHECK(scope_id=NULLIF(current_setting('emad.scope_id',true),''))$policy$,t||'_scope',t);
  EXECUTE format('CREATE POLICY %I ON %I AS RESTRICTIVE TO emad_data USING(COALESCE(current_setting(''emad.source_ids'',true),''null'') IN (''null'',''[]'') OR source_id IN(SELECT jsonb_array_elements_text(current_setting(''emad.source_ids'',true)::jsonb))) WITH CHECK(COALESCE(current_setting(''emad.source_ids'',true),''null'') IN (''null'',''[]'') OR source_id IN(SELECT jsonb_array_elements_text(current_setting(''emad.source_ids'',true)::jsonb)))',t||'_source',t);
 END LOOP;
END $$;
-- +goose StatementEnd
GRANT UPDATE(state) ON artifacts TO emad_data;
CREATE TRIGGER imagery_scenes_immutable BEFORE UPDATE OR DELETE ON imagery_scenes FOR EACH ROW EXECUTE FUNCTION emad_immutable_public_evidence();
-- +goose StatementBegin
CREATE FUNCTION emad_artifact_immutable() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN
 IF (to_jsonb(NEW)-'state') IS DISTINCT FROM (to_jsonb(OLD)-'state') THEN RAISE EXCEPTION 'artifact metadata is immutable'; END IF;
 IF NEW.state<>OLD.state AND NOT ((OLD.state='PENDING' AND NEW.state IN ('READY','DELETING')) OR (OLD.state='READY' AND NEW.state='DELETING') OR (OLD.state='DELETING' AND NEW.state='DELETED')) THEN RAISE EXCEPTION 'invalid artifact transition'; END IF;
 RETURN NEW;
END $$;
-- +goose StatementEnd
CREATE TRIGGER artifacts_immutable BEFORE UPDATE ON artifacts FOR EACH ROW EXECUTE FUNCTION emad_artifact_immutable();
-- +goose Down
-- +goose StatementBegin
DO $$ BEGIN RAISE EXCEPTION 'imagery evidence requires forward recovery'; END $$;
-- +goose StatementEnd
