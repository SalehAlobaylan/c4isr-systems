-- +goose Up
CREATE TABLE simulation_versions (
 id text NOT NULL, scope_id text NOT NULL DEFAULT NULLIF(current_setting('emad.scope_id',true),''),
 document bytea NOT NULL CHECK(octet_length(document) BETWEEN 1 AND 4194304), PRIMARY KEY(scope_id,id),
 CHECK(split_part(scope_id,'/',3)='EXERCISE_INPUT' AND split_part(scope_id,'/',4)='')
);
ALTER TABLE simulation_versions ENABLE ROW LEVEL SECURITY;
ALTER TABLE simulation_versions FORCE ROW LEVEL SECURITY;
GRANT SELECT,INSERT ON simulation_versions TO emad_data;
CREATE POLICY simulation_versions_scope ON simulation_versions TO emad_data USING(scope_id=NULLIF(current_setting('emad.scope_id',true),'')) WITH CHECK(scope_id=NULLIF(current_setting('emad.scope_id',true),''));
CREATE TRIGGER simulation_versions_immutable BEFORE UPDATE OR DELETE ON simulation_versions FOR EACH ROW EXECUTE FUNCTION emad_immutable_public_evidence();
ALTER TABLE scenario_runs ADD COLUMN frozen_run jsonb NOT NULL DEFAULT '{}'::jsonb;
-- +goose StatementBegin
CREATE FUNCTION emad_immutable_run_input() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN
 IF NEW.frozen_run IS DISTINCT FROM OLD.frozen_run OR NEW.seed IS DISTINCT FROM OLD.seed OR NEW.scenario_name IS DISTINCT FROM OLD.scenario_name THEN RAISE EXCEPTION 'frozen run input is immutable'; END IF;
 RETURN NEW;
END $$;
-- +goose StatementEnd
CREATE TRIGGER scenario_runs_frozen BEFORE UPDATE ON scenario_runs FOR EACH ROW EXECUTE FUNCTION emad_immutable_run_input();
CREATE TABLE context_packages (
 id text NOT NULL,scope_id text NOT NULL DEFAULT NULLIF(current_setting('emad.scope_id',true),''),
 document jsonb NOT NULL, bundle bytea NOT NULL CHECK(octet_length(bundle) BETWEEN 1 AND 12582912),
 PRIMARY KEY(scope_id,id),CHECK(split_part(scope_id,'/',3)='PUBLIC_EVIDENCE')
);
ALTER TABLE context_packages ENABLE ROW LEVEL SECURITY;
ALTER TABLE context_packages FORCE ROW LEVEL SECURITY;
GRANT SELECT,INSERT ON context_packages TO emad_data;
CREATE POLICY context_packages_scope ON context_packages TO emad_data USING(scope_id=NULLIF(current_setting('emad.scope_id',true),'')) WITH CHECK(scope_id=NULLIF(current_setting('emad.scope_id',true),''));
CREATE POLICY context_packages_sources ON context_packages AS RESTRICTIVE TO emad_data USING (
 COALESCE(current_setting('emad.source_ids',true),'null') IN ('null','[]') OR NOT EXISTS(SELECT 1 FROM jsonb_array_elements(document->'assets') a WHERE NOT (a->>'sourceId' IN (SELECT jsonb_array_elements_text(current_setting('emad.source_ids',true)::jsonb))))
) WITH CHECK (
 COALESCE(current_setting('emad.source_ids',true),'null') IN ('null','[]') OR NOT EXISTS(SELECT 1 FROM jsonb_array_elements(document->'assets') a WHERE NOT (a->>'sourceId' IN (SELECT jsonb_array_elements_text(current_setting('emad.source_ids',true)::jsonb))))
);
CREATE TRIGGER context_packages_immutable BEFORE UPDATE OR DELETE ON context_packages FOR EACH ROW EXECUTE FUNCTION emad_immutable_public_evidence();
-- +goose Down
-- +goose StatementBegin
DO $$ BEGIN RAISE EXCEPTION 'frozen simulation inputs require forward recovery'; END $$;
-- +goose StatementEnd
