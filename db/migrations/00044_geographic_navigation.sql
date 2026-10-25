-- +goose Up
-- Navigation reference geometry is admitted independently of bounded provider
-- retrieval policies. Existing evidence/import limits are deliberately retained.
CREATE TABLE geographic_editions (
 id text PRIMARY KEY, scope_id text NOT NULL DEFAULT NULLIF(current_setting('emad.scope_id',true),''),
 source_id text NOT NULL, document jsonb NOT NULL, artifact bytea NOT NULL CHECK(octet_length(artifact) BETWEEN 1 AND 8388608),
 spec_hash text NOT NULL, UNIQUE(id,source_id,scope_id),
 FOREIGN KEY(source_id,scope_id) REFERENCES sources(id,scope_id), CHECK(split_part(scope_id,'/',3)='PUBLIC_EVIDENCE')
);
CREATE TABLE geographic_regions (
 id text NOT NULL, edition_id text NOT NULL, scope_id text NOT NULL DEFAULT NULLIF(current_setting('emad.scope_id',true),''),
 source_id text NOT NULL, region_key text NOT NULL, parent_key text NOT NULL, kind text NOT NULL CHECK(kind IN('country','region','governorate','place','study')),
 name text NOT NULL, aliases text NOT NULL, geometry geometry(Geometry,4326) NOT NULL, properties jsonb NOT NULL,
 PRIMARY KEY(id,scope_id), UNIQUE(edition_id,region_key),
 FOREIGN KEY(edition_id,source_id,scope_id) REFERENCES geographic_editions(id,source_id,scope_id),
 CHECK(ST_IsValid(geometry) AND ST_NDims(geometry)=2 AND ST_NPoints(geometry)<=100000 AND ST_XMin(geometry)>=-180 AND ST_XMax(geometry)<=180 AND ST_YMin(geometry)>=-90 AND ST_YMax(geometry)<=90)
);
CREATE INDEX geographic_regions_geometry ON geographic_regions USING gist(geometry);
CREATE INDEX geographic_regions_parent ON geographic_regions(scope_id,parent_key);
-- +goose StatementBegin
DO $$ DECLARE t text; BEGIN
 FOREACH t IN ARRAY ARRAY['geographic_editions','geographic_regions'] LOOP
  EXECUTE format('ALTER TABLE %I ENABLE ROW LEVEL SECURITY',t);
  EXECUTE format('ALTER TABLE %I FORCE ROW LEVEL SECURITY',t);
  EXECUTE format('GRANT SELECT,INSERT ON %I TO emad_data',t);
  EXECUTE format('CREATE POLICY %I ON %I TO emad_data USING(scope_id=NULLIF(current_setting(''emad.scope_id'',true),'''')) WITH CHECK(scope_id=NULLIF(current_setting(''emad.scope_id'',true),''''))',t||'_scope',t);
  EXECUTE format('CREATE POLICY %I ON %I AS RESTRICTIVE TO emad_data USING(COALESCE(current_setting(''emad.source_ids'',true),''null'') IN (''null'',''[]'') OR source_id IN(SELECT jsonb_array_elements_text(current_setting(''emad.source_ids'',true)::jsonb))) WITH CHECK(COALESCE(current_setting(''emad.source_ids'',true),''null'') IN (''null'',''[]'') OR source_id IN(SELECT jsonb_array_elements_text(current_setting(''emad.source_ids'',true)::jsonb)))',t||'_source',t);
  EXECUTE format('CREATE TRIGGER %I BEFORE UPDATE OR DELETE ON %I FOR EACH ROW EXECUTE FUNCTION emad_immutable_public_evidence()',t||'_immutable',t);
 END LOOP;
END $$;
-- +goose StatementEnd
-- +goose Down
-- +goose StatementBegin
DO $$ BEGIN RAISE EXCEPTION 'admitted geography requires forward recovery'; END $$;
-- +goose StatementEnd
