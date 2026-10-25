-- +goose Up
CREATE TABLE simulation_sensor_presets(
 id text NOT NULL,scope_id text NOT NULL REFERENCES data_scopes(id) DEFAULT NULLIF(current_setting('emad.scope_id',true),''),
 document jsonb NOT NULL CHECK(octet_length(document::text)<=8192),created_at timestamptz NOT NULL DEFAULT now(),
 PRIMARY KEY(id,scope_id),CHECK(split_part(scope_id,'/',3)='EXERCISE_INPUT' AND split_part(scope_id,'/',4)='')
);
ALTER TABLE simulation_sensor_presets ENABLE ROW LEVEL SECURITY;
ALTER TABLE simulation_sensor_presets FORCE ROW LEVEL SECURITY;
GRANT SELECT,INSERT ON simulation_sensor_presets TO emad_data;
CREATE POLICY simulation_sensor_presets_scope ON simulation_sensor_presets TO emad_data USING(scope_id=NULLIF(current_setting('emad.scope_id',true),''))WITH CHECK(scope_id=NULLIF(current_setting('emad.scope_id',true),''));
CREATE TRIGGER sensor_preset_immutable BEFORE UPDATE OR DELETE ON simulation_sensor_presets FOR EACH ROW EXECUTE FUNCTION emad_immutable_public_evidence();
-- +goose Down
-- +goose StatementBegin
DO $$ BEGIN RAISE EXCEPTION 'frozen preset provenance requires forward recovery';END $$;
-- +goose StatementEnd
