-- +goose Up
CREATE TABLE native_environment_views(
 scope_id text PRIMARY KEY REFERENCES data_scopes(id),document jsonb NOT NULL CHECK(octet_length(document::text)<=65536)
);
ALTER TABLE native_environment_views ENABLE ROW LEVEL SECURITY;
ALTER TABLE native_environment_views FORCE ROW LEVEL SECURITY;
GRANT SELECT,INSERT ON native_environment_views TO emad_data;
CREATE POLICY native_environment_scope ON native_environment_views TO emad_data
 USING(scope_id=NULLIF(current_setting('emad.scope_id',true),''))WITH CHECK(scope_id=NULLIF(current_setting('emad.scope_id',true),''));
CREATE TRIGGER native_environment_immutable BEFORE UPDATE OR DELETE ON native_environment_views FOR EACH ROW EXECUTE FUNCTION emad_immutable_public_evidence();
-- +goose StatementBegin
CREATE FUNCTION emad_record_environment_view()RETURNS trigger LANGUAGE plpgsql AS $$
DECLARE ms bigint;
BEGIN
 IF split_part(NEW.scope_id,'/',3)<>'EXERCISE_OBSERVATION' THEN RAISE EXCEPTION 'operator environment requires exercise observation scope';END IF;
 SELECT elapsed_ms INTO ms FROM native_exercises WHERE scope_id=NEW.scope_id;
 IF NOT FOUND THEN RAISE EXCEPTION 'operator environment requires an admitted native exercise';END IF;
 PERFORM emad_record_event(NEW.scope_id,ms,'environment','base','INSERT',NEW.document);
 RETURN NULL;
END $$;
-- +goose StatementEnd
CREATE TRIGGER record_environment AFTER INSERT ON native_environment_views FOR EACH ROW EXECUTE FUNCTION emad_record_environment_view();
-- +goose Down
-- +goose StatementBegin
DO $$ BEGIN RAISE EXCEPTION 'recorded environment provenance requires forward recovery';END $$;
-- +goose StatementEnd
