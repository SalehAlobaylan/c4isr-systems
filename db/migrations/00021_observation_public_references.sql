-- +goose Up
-- A random publication reference cannot be used to test guesses about a
-- scenario's private actor names. Existing primary IDs/evidence links remain.
ALTER TABLE observations ADD COLUMN public_id text NOT NULL
    DEFAULT ('view_obs_'||replace(gen_random_uuid()::text,'-',''));
CREATE UNIQUE INDEX observations_public_id_key ON observations(scope_id,public_id);

-- +goose StatementBegin
CREATE FUNCTION emad_observation_reference_immutable() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
    IF NEW.public_id IS DISTINCT FROM OLD.public_id THEN
        RAISE EXCEPTION 'observation publication reference is immutable'
            USING ERRCODE='23514',CONSTRAINT='emad_scope_relation';
    END IF;
    RETURN NEW;
END $$;
-- +goose StatementEnd
CREATE TRIGGER observations_public_reference_immutable BEFORE UPDATE ON observations
FOR EACH ROW EXECUTE FUNCTION emad_observation_reference_immutable();

-- +goose Down
-- Publication references may already be retained by evidence consumers.
-- +goose StatementBegin
DO $$ BEGIN RAISE EXCEPTION 'publication references require an explicit forward recovery plan'; END $$;
-- +goose StatementEnd
