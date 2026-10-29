-- +goose Up
-- Additive contracts; no retained reference or evidence bytes are rewritten.
CREATE TABLE geographic_releases (
 id text PRIMARY KEY, scope_id text NOT NULL DEFAULT NULLIF(current_setting('emad.scope_id',true),''),
 source_id text NOT NULL, release_key text NOT NULL, version text NOT NULL, document jsonb NOT NULL,
 UNIQUE(id,source_id,scope_id), UNIQUE(scope_id,source_id,release_key,version),
 FOREIGN KEY(source_id,scope_id) REFERENCES sources(id,scope_id), CHECK(split_part(scope_id,'/',3)='PUBLIC_EVIDENCE')
);
CREATE TABLE geographic_release_editions (
 release_id text NOT NULL, edition_id text NOT NULL, source_id text NOT NULL,
 scope_id text NOT NULL DEFAULT NULLIF(current_setting('emad.scope_id',true),''),
 PRIMARY KEY(release_id,edition_id),
 FOREIGN KEY(release_id,source_id,scope_id) REFERENCES geographic_releases(id,source_id,scope_id),
 FOREIGN KEY(edition_id,source_id,scope_id) REFERENCES geographic_editions(id,source_id,scope_id)
);
CREATE TABLE geographic_release_heads (
 scope_id text NOT NULL DEFAULT NULLIF(current_setting('emad.scope_id',true),''), source_id text NOT NULL,
 release_id text NOT NULL, PRIMARY KEY(scope_id,source_id),
 FOREIGN KEY(release_id,source_id,scope_id) REFERENCES geographic_releases(id,source_id,scope_id)
);
-- +goose StatementBegin
DO $$ DECLARE t text; BEGIN
 FOREACH t IN ARRAY ARRAY['geographic_releases','geographic_release_editions','geographic_release_heads'] LOOP
  EXECUTE format('ALTER TABLE %I ENABLE ROW LEVEL SECURITY',t);
  EXECUTE format('ALTER TABLE %I FORCE ROW LEVEL SECURITY',t);
  EXECUTE format('GRANT SELECT,INSERT ON %I TO emad_data',t);
  EXECUTE format('CREATE POLICY %I ON %I TO emad_data USING(scope_id=NULLIF(current_setting(''emad.scope_id'',true),'''')) WITH CHECK(scope_id=NULLIF(current_setting(''emad.scope_id'',true),''''))',t||'_scope',t);
  EXECUTE format('CREATE POLICY %I ON %I AS RESTRICTIVE TO emad_data USING(COALESCE(current_setting(''emad.source_ids'',true),''null'') IN (''null'',''[]'') OR source_id IN(SELECT jsonb_array_elements_text(current_setting(''emad.source_ids'',true)::jsonb))) WITH CHECK(COALESCE(current_setting(''emad.source_ids'',true),''null'') IN (''null'',''[]'') OR source_id IN(SELECT jsonb_array_elements_text(current_setting(''emad.source_ids'',true)::jsonb)))',t||'_source',t);
  IF t<>'geographic_release_heads' THEN
   EXECUTE format('CREATE TRIGGER %I BEFORE UPDATE OR DELETE ON %I FOR EACH ROW EXECUTE FUNCTION emad_immutable_public_evidence()',t||'_immutable',t);
  END IF;
 END LOOP;
END $$;
-- +goose StatementEnd
GRANT UPDATE ON geographic_release_heads TO emad_data;

CREATE TABLE area_revisions (
 id text NOT NULL, scope_id text NOT NULL DEFAULT NULLIF(current_setting('emad.scope_id',true),''),
 area_id text NOT NULL, document jsonb NOT NULL, geometry geometry(Geometry,4326),
 canonical_geometry bytea, original_geometry bytea,
 required_sources jsonb NOT NULL CHECK(jsonb_typeof(required_sources)='array'),
 PRIMARY KEY(id,scope_id),
 CHECK(split_part(scope_id,'/',3)='PUBLIC_EVIDENCE'),
 CHECK(geometry IS NULL OR ST_IsValid(geometry) AND NOT ST_IsEmpty(geometry) AND ST_NDims(geometry)=2)
);
CREATE TABLE region_profile_revisions (
 id text NOT NULL, scope_id text NOT NULL DEFAULT NULLIF(current_setting('emad.scope_id',true),''),
 profile_key text NOT NULL, version text NOT NULL, document jsonb NOT NULL, manifest_bytes bytea NOT NULL, required_sources jsonb NOT NULL,
 PRIMARY KEY(id,scope_id), UNIQUE(scope_id,profile_key,version), CHECK(split_part(scope_id,'/',3)='PUBLIC_EVIDENCE')
);
CREATE TABLE analyst_scope_snapshots (
 id text NOT NULL, scope_id text NOT NULL DEFAULT NULLIF(current_setting('emad.scope_id',true),''),
 area_revision_id text NOT NULL, document jsonb NOT NULL, required_sources jsonb NOT NULL,
 PRIMARY KEY(id,scope_id), FOREIGN KEY(area_revision_id,scope_id) REFERENCES area_revisions(id,scope_id)
);
-- +goose StatementBegin
DO $$ DECLARE t text; BEGIN
 FOREACH t IN ARRAY ARRAY['area_revisions','region_profile_revisions','analyst_scope_snapshots'] LOOP
  EXECUTE format('ALTER TABLE %I ENABLE ROW LEVEL SECURITY',t);
  EXECUTE format('ALTER TABLE %I FORCE ROW LEVEL SECURITY',t);
  EXECUTE format('GRANT SELECT,INSERT ON %I TO emad_data',t);
  EXECUTE format('CREATE POLICY %I ON %I TO emad_data USING(scope_id=NULLIF(current_setting(''emad.scope_id'',true),'''')) WITH CHECK(scope_id=NULLIF(current_setting(''emad.scope_id'',true),''''))',t||'_scope',t);
  EXECUTE format('CREATE POLICY %I ON %I AS RESTRICTIVE TO emad_data USING(COALESCE(current_setting(''emad.source_ids'',true),''null'') IN (''null'',''[]'') OR required_sources <@ current_setting(''emad.source_ids'',true)::jsonb) WITH CHECK(COALESCE(current_setting(''emad.source_ids'',true),''null'') IN (''null'',''[]'') OR required_sources <@ current_setting(''emad.source_ids'',true)::jsonb)',t||'_sources',t);
  EXECUTE format('CREATE TRIGGER %I BEFORE UPDATE OR DELETE ON %I FOR EACH ROW EXECUTE FUNCTION emad_immutable_public_evidence()',t||'_immutable',t);
 END LOOP;
END $$;
-- +goose StatementEnd
CREATE TABLE analyst_read_snapshots (
 id text NOT NULL, scope_id text NOT NULL DEFAULT NULLIF(current_setting('emad.scope_id',true),''),
 principal_id text NOT NULL DEFAULT NULLIF(current_setting('emad.principal_id',true),''),
 authorization_revision text NOT NULL, expires_at timestamptz NOT NULL, document jsonb NOT NULL,
 budget_slot integer NOT NULL CHECK(budget_slot BETWEEN 1 AND 16), UNIQUE(scope_id,principal_id,budget_slot),
 PRIMARY KEY(id,scope_id), CHECK(octet_length(document::text)<=8388608)
);
CREATE INDEX analyst_read_expiry ON analyst_read_snapshots(expires_at);
ALTER TABLE analyst_read_snapshots ENABLE ROW LEVEL SECURITY;
ALTER TABLE analyst_read_snapshots FORCE ROW LEVEL SECURITY;
GRANT SELECT,INSERT,DELETE ON analyst_read_snapshots TO emad_data;
CREATE POLICY analyst_read_owner ON analyst_read_snapshots TO emad_data
 USING(scope_id=NULLIF(current_setting('emad.scope_id',true),'') AND principal_id=NULLIF(current_setting('emad.principal_id',true),''))
 WITH CHECK(scope_id=NULLIF(current_setting('emad.scope_id',true),'') AND principal_id=NULLIF(current_setting('emad.principal_id',true),''));
-- Expired manifests contain no durable evidence; maintenance can reclaim them.
GRANT DELETE ON analyst_read_snapshots TO CURRENT_USER;
-- Embedded scope provenance follows its contributing source grants, including
-- when the geometry source differs from an imagery job's measurement source.
-- +goose StatementBegin
CREATE FUNCTION emad_scope_sources_visible(dependencies jsonb) RETURNS boolean
 LANGUAGE sql STABLE SECURITY INVOKER AS $$
 SELECT COALESCE(current_setting('emad.source_ids',true),'null') IN ('null','[]')
 OR COALESCE(dependencies,'[]'::jsonb) <@ current_setting('emad.source_ids',true)::jsonb
$$;
CREATE POLICY imagery_job_scope_sources ON imagery_jobs AS RESTRICTIVE TO emad_data
 USING(emad_scope_sources_visible(input#>'{scope,requiredSources}')) WITH CHECK(emad_scope_sources_visible(input#>'{scope,requiredSources}'));
CREATE POLICY imagery_capture_scope_sources ON imagery_capture_jobs AS RESTRICTIVE TO emad_data
 USING(emad_scope_sources_visible(input#>'{scope,requiredSources}')) WITH CHECK(emad_scope_sources_visible(input#>'{scope,requiredSources}'));
CREATE POLICY assessment_scope_sources ON assessment_revisions AS RESTRICTIVE TO emad_data
 USING(emad_scope_sources_visible(document#>'{evidence,input,scope,requiredSources}')) WITH CHECK(emad_scope_sources_visible(document#>'{evidence,input,scope,requiredSources}'));
CREATE POLICY context_package_scope_sources ON context_packages AS RESTRICTIVE TO emad_data
 USING(emad_scope_sources_visible(document#>'{scope,requiredSources}')) WITH CHECK(emad_scope_sources_visible(document#>'{scope,requiredSources}'));
CREATE POLICY context_preview_scope_sources ON context_export_previews AS RESTRICTIVE TO emad_data
 USING(emad_scope_sources_visible(document#>'{scope,requiredSources}')) WITH CHECK(emad_scope_sources_visible(document#>'{scope,requiredSources}'));
CREATE POLICY content_purpose_scope_dependencies ON content_purpose_revisions AS RESTRICTIVE TO emad_data
 USING(CASE entity_kind
 WHEN 'capture' THEN EXISTS(SELECT 1 FROM imagery_capture_jobs e WHERE e.id=entity_id)
 WHEN 'job' THEN EXISTS(SELECT 1 FROM imagery_jobs e WHERE e.id=entity_id)
 WHEN 'assessment' THEN EXISTS(SELECT 1 FROM assessment_revisions e WHERE e.assessment_id=entity_id)
 WHEN 'package' THEN EXISTS(SELECT 1 FROM context_packages e WHERE e.id=entity_id)
 ELSE true END)
 WITH CHECK(CASE entity_kind
 WHEN 'capture' THEN EXISTS(SELECT 1 FROM imagery_capture_jobs e WHERE e.id=entity_id)
 WHEN 'job' THEN EXISTS(SELECT 1 FROM imagery_jobs e WHERE e.id=entity_id)
 WHEN 'assessment' THEN EXISTS(SELECT 1 FROM assessment_revisions e WHERE e.assessment_id=entity_id)
 WHEN 'package' THEN EXISTS(SELECT 1 FROM context_packages e WHERE e.id=entity_id)
 ELSE true END);
-- Privacy of nested provenance must not weaken existing queue limits. Return
-- availability only, inside the caller's resource, including hidden jobs.
CREATE FUNCTION emad_imagery_queue_available(queue_kind text, measurement_source text) RETURNS boolean
 LANGUAGE sql STABLE SECURITY DEFINER SET search_path=pg_catalog,public SET row_security=off AS $$
 SELECT CASE WHEN NULLIF(current_setting('emad.scope_id',true),'') IS NULL THEN false
 WHEN COALESCE(current_setting('emad.source_ids',true),'null') NOT IN ('null','[]') AND NOT current_setting('emad.source_ids',true)::jsonb @> jsonb_build_array(measurement_source) THEN false
 WHEN queue_kind='capture' THEN (SELECT count(*)<1000 AND count(*) FILTER(WHERE status IN('QUEUED','RUNNING'))<4 FROM public.imagery_capture_jobs WHERE scope_id=current_setting('emad.scope_id',true))
 WHEN queue_kind='job' THEN (SELECT count(*)<32 FROM public.imagery_jobs WHERE scope_id=current_setting('emad.scope_id',true) AND source_id=measurement_source AND status IN('QUEUED','RUNNING','CANCEL_REQUESTED'))
 ELSE false END
$$;
REVOKE ALL ON FUNCTION emad_imagery_queue_available(text,text) FROM PUBLIC;
GRANT EXECUTE ON FUNCTION emad_imagery_queue_available(text,text) TO emad_data;
-- +goose StatementEnd
-- +goose Down
-- +goose StatementBegin
DO $$ BEGIN RAISE EXCEPTION 'analyst scope history requires forward recovery'; END $$;
-- +goose StatementEnd
