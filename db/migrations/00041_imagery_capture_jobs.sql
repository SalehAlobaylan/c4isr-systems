-- +goose Up
CREATE TABLE imagery_capture_jobs(
 id text PRIMARY KEY,scope_id text NOT NULL DEFAULT NULLIF(current_setting('emad.scope_id',true),''),source_id text NOT NULL,
 input jsonb NOT NULL CHECK(octet_length(input::text)<=65536),created_by text NOT NULL,created_at timestamptz NOT NULL DEFAULT now(),
 status text NOT NULL DEFAULT 'QUEUED' CHECK(status IN('QUEUED','RUNNING','SUCCEEDED','FAILED','CANCELLED')),
 attempt integer NOT NULL DEFAULT 0 CHECK(attempt BETWEEN 0 AND 3),worker_id text,lease_id text,lease_until timestamptz,
 progress text NOT NULL DEFAULT 'Awaiting capture worker',result jsonb,error_code text NOT NULL DEFAULT '',finished_at timestamptz,
 FOREIGN KEY(source_id,scope_id)REFERENCES sources(id,scope_id),CHECK(split_part(scope_id,'/',3)='PUBLIC_EVIDENCE')
);
ALTER TABLE imagery_capture_jobs ENABLE ROW LEVEL SECURITY;
ALTER TABLE imagery_capture_jobs FORCE ROW LEVEL SECURITY;
GRANT SELECT,INSERT,UPDATE ON imagery_capture_jobs TO emad_data;
CREATE POLICY imagery_capture_scope ON imagery_capture_jobs TO emad_data USING(scope_id=NULLIF(current_setting('emad.scope_id',true),''))WITH CHECK(scope_id=NULLIF(current_setting('emad.scope_id',true),''));
CREATE POLICY imagery_capture_source ON imagery_capture_jobs AS RESTRICTIVE TO emad_data
 USING(COALESCE(current_setting('emad.source_ids',true),'null') IN ('null','[]') OR source_id IN(SELECT jsonb_array_elements_text(current_setting('emad.source_ids',true)::jsonb)))
 WITH CHECK(COALESCE(current_setting('emad.source_ids',true),'null') IN ('null','[]') OR source_id IN(SELECT jsonb_array_elements_text(current_setting('emad.source_ids',true)::jsonb)));
-- +goose StatementBegin
CREATE FUNCTION emad_capture_input_immutable()RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN
 IF NEW.id IS DISTINCT FROM OLD.id OR NEW.input IS DISTINCT FROM OLD.input OR NEW.scope_id IS DISTINCT FROM OLD.scope_id OR NEW.source_id IS DISTINCT FROM OLD.source_id OR NEW.created_by IS DISTINCT FROM OLD.created_by OR NEW.created_at IS DISTINCT FROM OLD.created_at THEN RAISE EXCEPTION 'capture input is immutable'; END IF;RETURN NEW;END $$;
-- +goose StatementEnd
CREATE TRIGGER capture_input_immutable BEFORE UPDATE ON imagery_capture_jobs FOR EACH ROW EXECUTE FUNCTION emad_capture_input_immutable();
-- +goose Down
-- +goose StatementBegin
DO $$ BEGIN RAISE EXCEPTION 'capture provenance requires forward recovery';END $$;
-- +goose StatementEnd
