-- +goose Up
CREATE TABLE exercise_messages (
 id text NOT NULL,scope_id text NOT NULL DEFAULT NULLIF(current_setting('emad.scope_id',true),''),
 kind text NOT NULL CHECK(kind IN ('observation','telemetry')), sequence integer NOT NULL CHECK(sequence>0),
 elapsed_ms bigint NOT NULL CHECK(elapsed_ms BETWEEN 0 AND 2678400000), version_id text NOT NULL,
 input_hash text NOT NULL,document jsonb NOT NULL,status text NOT NULL CHECK(status IN ('PENDING','DELIVERED','REJECTED')),
 received_at timestamptz NOT NULL DEFAULT now(),processed_at timestamptz,error_code text NOT NULL DEFAULT '',
 PRIMARY KEY(scope_id,id),CHECK(split_part(scope_id,'/',3)='EXERCISE_OBSERVATION'),
 CHECK(octet_length(document::text)<=65536)
);
CREATE INDEX exercise_messages_order_idx ON exercise_messages(scope_id,sequence,id);
CREATE TABLE simulation_actor_mappings (
 scope_id text NOT NULL DEFAULT NULLIF(current_setting('emad.scope_id',true),''),
 message_id text NOT NULL,actor_id text NOT NULL,contact_ref text NOT NULL,kind text NOT NULL,
 PRIMARY KEY(scope_id,message_id),CHECK(split_part(scope_id,'/',3)='EXERCISE_TRUTH')
);
-- +goose StatementBegin
DO $$ DECLARE t text; BEGIN
 FOREACH t IN ARRAY ARRAY['exercise_messages','simulation_actor_mappings'] LOOP
  EXECUTE format('ALTER TABLE %I ENABLE ROW LEVEL SECURITY',t);
  EXECUTE format('ALTER TABLE %I FORCE ROW LEVEL SECURITY',t);
  EXECUTE format('GRANT SELECT,INSERT ON %I TO emad_data',t);
  EXECUTE format('CREATE POLICY %I ON %I TO emad_data USING(scope_id=NULLIF(current_setting(''emad.scope_id'',true),'''')) WITH CHECK(scope_id=NULLIF(current_setting(''emad.scope_id'',true),''''))',t||'_scope',t);
 END LOOP;
END $$;
-- +goose StatementEnd
GRANT UPDATE(status,processed_at,error_code) ON exercise_messages TO emad_data;
CREATE TRIGGER simulation_actor_mappings_immutable BEFORE UPDATE OR DELETE ON simulation_actor_mappings FOR EACH ROW EXECUTE FUNCTION emad_immutable_public_evidence();
-- +goose StatementBegin
CREATE FUNCTION emad_exercise_message_immutable() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN
 IF ROW(NEW.id,NEW.scope_id,NEW.kind,NEW.sequence,NEW.elapsed_ms,NEW.version_id,NEW.input_hash,NEW.document,NEW.received_at) IS DISTINCT FROM ROW(OLD.id,OLD.scope_id,OLD.kind,OLD.sequence,OLD.elapsed_ms,OLD.version_id,OLD.input_hash,OLD.document,OLD.received_at) THEN RAISE EXCEPTION 'synthetic message input is immutable'; END IF;
 RETURN NEW;
END $$;
-- +goose StatementEnd
CREATE TRIGGER exercise_messages_immutable BEFORE UPDATE ON exercise_messages FOR EACH ROW EXECUTE FUNCTION emad_exercise_message_immutable();
-- +goose Down
-- +goose StatementBegin
DO $$ BEGIN RAISE EXCEPTION 'synthetic evidence requires forward recovery'; END $$;
-- +goose StatementEnd
