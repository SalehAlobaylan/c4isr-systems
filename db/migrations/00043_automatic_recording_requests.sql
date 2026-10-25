-- +goose Up
-- Queue the operator journal automatically when a native exercise closes.
-- Existing manual requests remain idempotent. No cross-partition privilege is
-- needed here: the archive worker separately opens the sealed truth journal.
-- +goose StatementBegin
CREATE FUNCTION emad_request_closed_recording() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
 IF NEW.status IN ('COMPLETED','STOPPED','FAILED') AND OLD.status IS DISTINCT FROM NEW.status THEN
  INSERT INTO simulation_recordings(scope_id,status,requested_by)
   VALUES(NEW.scope_id,'QUEUED','internal:auto-archive') ON CONFLICT DO NOTHING;
 END IF;
 RETURN NULL;
END $$;
-- +goose StatementEnd
CREATE TRIGGER native_automatic_recording AFTER UPDATE OF status ON native_exercises
 FOR EACH ROW EXECUTE FUNCTION emad_request_closed_recording();

-- +goose Down
DROP TRIGGER native_automatic_recording ON native_exercises;
DROP FUNCTION emad_request_closed_recording();
