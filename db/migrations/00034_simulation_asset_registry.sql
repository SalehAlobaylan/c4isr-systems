-- +goose Up
CREATE TABLE model_asset_uploads(
 id text NOT NULL,scope_id text NOT NULL REFERENCES data_scopes(id) DEFAULT NULLIF(current_setting('emad.scope_id',true),''),kind text NOT NULL,
 document jsonb NOT NULL CHECK(octet_length(document::text)<=32768),object_key text NOT NULL,size_bytes bigint NOT NULL CHECK(size_bytes BETWEEN 1 AND 16777216),
 state text NOT NULL CHECK(state IN('PENDING','STAGED','RUNNING','READY','STORED','REJECTED','FAILED')),attempts integer NOT NULL DEFAULT 0,
 lease_id text,lease_until timestamptz,error_code text NOT NULL DEFAULT '',
 source_file_id text,parent_version_id text,
 PRIMARY KEY(id,scope_id),CHECK(split_part(scope_id,'/',3)='EXERCISE_INPUT' AND split_part(scope_id,'/',4)=''),
 CHECK(kind IN('MODEL','SOURCE','ENGINE_VARIANT')),FOREIGN KEY(source_file_id,scope_id) REFERENCES model_asset_uploads(id,scope_id)
);
CREATE TABLE model_asset_versions(
 id text NOT NULL,scope_id text NOT NULL DEFAULT NULLIF(current_setting('emad.scope_id',true),''),upload_id text NOT NULL,manifest_key text NOT NULL,document jsonb NOT NULL CHECK(octet_length(document::text)<=262144),
 PRIMARY KEY(id,scope_id),UNIQUE(upload_id,scope_id),FOREIGN KEY(upload_id,scope_id) REFERENCES model_asset_uploads(id,scope_id)
);
ALTER TABLE model_asset_uploads ADD FOREIGN KEY(parent_version_id,scope_id) REFERENCES model_asset_versions(id,scope_id);
CREATE TABLE model_asset_attempts(
 id text NOT NULL,scope_id text NOT NULL DEFAULT NULLIF(current_setting('emad.scope_id',true),''),upload_id text NOT NULL,document jsonb NOT NULL,
 PRIMARY KEY(id,scope_id),FOREIGN KEY(upload_id,scope_id) REFERENCES model_asset_uploads(id,scope_id)
);
CREATE TABLE actor_templates(
 id text NOT NULL,scope_id text NOT NULL DEFAULT NULLIF(current_setting('emad.scope_id',true),''),model_version_id text NOT NULL,document jsonb NOT NULL CHECK(octet_length(document::text)<=32768),
 PRIMARY KEY(id,scope_id),FOREIGN KEY(model_version_id,scope_id) REFERENCES model_asset_versions(id,scope_id)
);
CREATE TABLE environment_assets(
 id text NOT NULL,scope_id text NOT NULL DEFAULT NULLIF(current_setting('emad.scope_id',true),''),model_version_id text NOT NULL,document jsonb NOT NULL CHECK(octet_length(document::text)<=32768),
 PRIMARY KEY(id,scope_id),FOREIGN KEY(model_version_id,scope_id) REFERENCES model_asset_versions(id,scope_id)
);
CREATE INDEX model_asset_work ON model_asset_uploads(scope_id,state,lease_until,id) WHERE kind='MODEL';
-- +goose StatementBegin
DO $$ DECLARE t text; BEGIN
 FOREACH t IN ARRAY ARRAY['model_asset_uploads','model_asset_versions','model_asset_attempts','actor_templates','environment_assets'] LOOP
  EXECUTE format('ALTER TABLE %I ENABLE ROW LEVEL SECURITY',t);EXECUTE format('ALTER TABLE %I FORCE ROW LEVEL SECURITY',t);
  EXECUTE format('GRANT SELECT,INSERT ON %I TO emad_data',t);
  EXECUTE format($p$CREATE POLICY %I ON %I TO emad_data USING(scope_id=NULLIF(current_setting('emad.scope_id',true),'')) WITH CHECK(scope_id=NULLIF(current_setting('emad.scope_id',true),''))$p$,t||'_scope',t);
  IF t<>'model_asset_uploads' THEN EXECUTE format('CREATE TRIGGER %I BEFORE UPDATE OR DELETE ON %I FOR EACH ROW EXECUTE FUNCTION emad_immutable_public_evidence()',t||'_immutable',t);END IF;
 END LOOP;
END $$;
-- +goose StatementEnd
GRANT UPDATE(state,attempts,lease_id,lease_until,error_code) ON model_asset_uploads TO emad_data;
-- +goose StatementBegin
CREATE FUNCTION emad_immutable_asset_input() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN
 IF NEW.id IS DISTINCT FROM OLD.id OR NEW.scope_id IS DISTINCT FROM OLD.scope_id OR NEW.document IS DISTINCT FROM OLD.document OR NEW.object_key IS DISTINCT FROM OLD.object_key OR NEW.size_bytes IS DISTINCT FROM OLD.size_bytes OR NEW.kind IS DISTINCT FROM OLD.kind OR NEW.source_file_id IS DISTINCT FROM OLD.source_file_id OR NEW.parent_version_id IS DISTINCT FROM OLD.parent_version_id THEN RAISE EXCEPTION 'model source input is immutable';END IF;RETURN NEW;
END $$;
-- +goose StatementEnd
CREATE TRIGGER model_asset_input_immutable BEFORE UPDATE ON model_asset_uploads FOR EACH ROW EXECUTE FUNCTION emad_immutable_asset_input();
CREATE TRIGGER model_asset_upload_no_delete BEFORE DELETE ON model_asset_uploads FOR EACH ROW EXECUTE FUNCTION emad_immutable_public_evidence();
-- +goose Down
-- +goose StatementBegin
DO $$ BEGIN RAISE EXCEPTION 'simulation asset versions require forward recovery';END $$;
-- +goose StatementEnd
