-- +goose Up
CREATE TABLE reference_datasets (
 id text PRIMARY KEY,scope_id text NOT NULL DEFAULT NULLIF(current_setting('emad.scope_id',true),''),source_id text NOT NULL,
 dataset_key text NOT NULL,version text NOT NULL,kind text NOT NULL CHECK(kind IN ('vector','elevation')),
 document jsonb NOT NULL,artifact bytea NOT NULL CHECK(octet_length(artifact) BETWEEN 1 AND 524288),spec_hash text NOT NULL,
 UNIQUE(id,source_id,scope_id),UNIQUE(scope_id,source_id,dataset_key,version),
 FOREIGN KEY(source_id,scope_id) REFERENCES sources(id,scope_id),
 CHECK(split_part(scope_id,'/',3)='PUBLIC_EVIDENCE')
);
CREATE TABLE reference_features (
 dataset_id text NOT NULL,scope_id text NOT NULL DEFAULT NULLIF(current_setting('emad.scope_id',true),''),source_id text NOT NULL,
 feature_id text NOT NULL,geometry geometry(Geometry,4326) NOT NULL,properties jsonb NOT NULL,
 PRIMARY KEY(dataset_id,feature_id),FOREIGN KEY(dataset_id,source_id,scope_id) REFERENCES reference_datasets(id,source_id,scope_id),
 CHECK(ST_IsValid(geometry) AND ST_NDims(geometry)=2 AND ST_NPoints(geometry)<=20000),
 CHECK(split_part(scope_id,'/',3)='PUBLIC_EVIDENCE')
);
CREATE INDEX reference_features_geometry_idx ON reference_features USING gist(geometry);
-- +goose StatementBegin
DO $$ DECLARE t text; BEGIN
 FOREACH t IN ARRAY ARRAY['reference_datasets','reference_features'] LOOP
  EXECUTE format('ALTER TABLE %I ENABLE ROW LEVEL SECURITY',t);
  EXECUTE format('ALTER TABLE %I FORCE ROW LEVEL SECURITY',t);
  EXECUTE format('GRANT SELECT,INSERT ON %I TO emad_data',t);
  EXECUTE format('CREATE POLICY %I ON %I TO emad_data USING (scope_id=NULLIF(current_setting(''emad.scope_id'',true),'''')) WITH CHECK (scope_id=NULLIF(current_setting(''emad.scope_id'',true),''''))',t||'_scope',t);
  EXECUTE format('CREATE POLICY %I ON %I AS RESTRICTIVE TO emad_data USING (COALESCE(current_setting(''emad.source_ids'',true),''null'') IN (''null'',''[]'') OR source_id IN (SELECT jsonb_array_elements_text(current_setting(''emad.source_ids'',true)::jsonb))) WITH CHECK (COALESCE(current_setting(''emad.source_ids'',true),''null'') IN (''null'',''[]'') OR source_id IN (SELECT jsonb_array_elements_text(current_setting(''emad.source_ids'',true)::jsonb)))',t||'_source',t);
  EXECUTE format('CREATE TRIGGER %I BEFORE UPDATE OR DELETE ON %I FOR EACH ROW EXECUTE FUNCTION emad_immutable_public_evidence()',t||'_immutable',t);
 END LOOP;
END $$;
-- +goose StatementEnd
-- +goose Down
-- +goose StatementBegin
DO $$ BEGIN RAISE EXCEPTION 'reference snapshots require forward recovery'; END $$;
-- +goose StatementEnd
