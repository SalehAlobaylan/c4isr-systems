-- +goose Up
ALTER TABLE composite_outputs ADD COLUMN upload_sha256 text NOT NULL DEFAULT '';
ALTER TABLE composite_outputs ADD COLUMN next_cleanup_at timestamptz NOT NULL DEFAULT clock_timestamp();
CREATE INDEX composite_expiry_work ON composite_outputs(scope_id,next_cleanup_at,retain_until) WHERE state<>'TOMBSTONED';
-- +goose StatementBegin
CREATE FUNCTION emad_upload_identity_immutable() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN
 IF OLD.upload_sha256<>'' AND NEW.upload_sha256<>OLD.upload_sha256 THEN RAISE EXCEPTION 'immutable planned render identity'; END IF;
 RETURN NEW;
END $$;
-- +goose StatementEnd
CREATE TRIGGER composite_upload_identity BEFORE UPDATE ON composite_outputs FOR EACH ROW EXECUTE FUNCTION emad_upload_identity_immutable();

CREATE TABLE delivery_jobs(
 id text NOT NULL,scope_id text NOT NULL DEFAULT NULLIF(current_setting('emad.scope_id',true),''),case_id text NOT NULL,output_id text NOT NULL,
 creator_human_id text NOT NULL,kind text NOT NULL CHECK(kind IN('brief','environment')),input_hash text NOT NULL,document jsonb NOT NULL,
 state text NOT NULL DEFAULT 'QUEUED' CHECK(state IN('QUEUED','RUNNING','SUCCEEDED','FAILED','CANCELED')),
 attempt integer NOT NULL DEFAULT 0,lease_id text NOT NULL DEFAULT '',lease_until timestamptz,
 next_attempt_at timestamptz NOT NULL DEFAULT clock_timestamp(),worker_build text NOT NULL DEFAULT '',failure_reason text NOT NULL DEFAULT '',created_at timestamptz NOT NULL DEFAULT clock_timestamp(),
 PRIMARY KEY(id,scope_id),UNIQUE(output_id,scope_id),FOREIGN KEY(case_id,scope_id) REFERENCES investigations(id,scope_id),FOREIGN KEY(output_id,scope_id) REFERENCES composite_outputs(id,scope_id)
);
CREATE INDEX delivery_queue ON delivery_jobs(scope_id,state,created_at);
CREATE TABLE delivery_attempt_events(
 id text NOT NULL,scope_id text NOT NULL DEFAULT NULLIF(current_setting('emad.scope_id',true),''),case_id text NOT NULL,job_id text NOT NULL,document jsonb NOT NULL,
 PRIMARY KEY(id,scope_id),FOREIGN KEY(job_id,scope_id) REFERENCES delivery_jobs(id,scope_id)
);
ALTER TABLE delivery_jobs ENABLE ROW LEVEL SECURITY;
ALTER TABLE delivery_jobs FORCE ROW LEVEL SECURITY;
ALTER TABLE delivery_attempt_events ENABLE ROW LEVEL SECURITY;
ALTER TABLE delivery_attempt_events FORCE ROW LEVEL SECURITY;
GRANT SELECT,INSERT,UPDATE ON delivery_jobs TO emad_data;
GRANT SELECT,INSERT ON delivery_attempt_events TO emad_data;
CREATE POLICY delivery_case ON delivery_jobs TO emad_data USING(emad_investigation_visible(case_id,scope_id)) WITH CHECK(emad_investigation_visible(case_id,scope_id));
CREATE POLICY delivery_event_case ON delivery_attempt_events TO emad_data USING(emad_investigation_visible(case_id,scope_id)) WITH CHECK(emad_investigation_visible(case_id,scope_id));
CREATE TRIGGER delivery_attempt_immutable BEFORE UPDATE OR DELETE ON delivery_attempt_events FOR EACH ROW EXECUTE FUNCTION emad_immutable_public_evidence();
-- +goose StatementBegin
CREATE FUNCTION emad_delivery_input_immutable() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN
 IF NEW.id<>OLD.id OR NEW.scope_id<>OLD.scope_id OR NEW.case_id<>OLD.case_id OR NEW.output_id<>OLD.output_id OR NEW.creator_human_id<>OLD.creator_human_id OR NEW.kind<>OLD.kind OR NEW.input_hash<>OLD.input_hash OR NEW.document<>OLD.document OR NEW.created_at<>OLD.created_at THEN RAISE EXCEPTION 'immutable delivery input'; END IF;
 IF OLD.state IN('SUCCEEDED','CANCELED') AND NEW.state<>OLD.state THEN RAISE EXCEPTION 'terminal delivery job'; END IF;
 RETURN NEW;
END $$;
-- +goose StatementEnd
CREATE TRIGGER delivery_input_immutable BEFORE UPDATE ON delivery_jobs FOR EACH ROW EXECUTE FUNCTION emad_delivery_input_immutable();

CREATE TABLE delivery_intents(
 scope_id text NOT NULL DEFAULT NULLIF(current_setting('emad.scope_id',true),''),human_id text NOT NULL,request_key text NOT NULL,input_hash text NOT NULL,job_id text NOT NULL,
 PRIMARY KEY(scope_id,human_id,request_key),FOREIGN KEY(job_id,scope_id) REFERENCES delivery_jobs(id,scope_id)
);
ALTER TABLE delivery_intents ENABLE ROW LEVEL SECURITY;
ALTER TABLE delivery_intents FORCE ROW LEVEL SECURITY;
GRANT SELECT,INSERT ON delivery_intents TO emad_data;
CREATE POLICY delivery_intent_owner ON delivery_intents TO emad_data USING(scope_id=NULLIF(current_setting('emad.scope_id',true),'') AND human_id=NULLIF(current_setting('emad.human_id',true),'')) WITH CHECK(scope_id=NULLIF(current_setting('emad.scope_id',true),'') AND human_id=NULLIF(current_setting('emad.human_id',true),''));
CREATE TRIGGER delivery_intent_immutable BEFORE UPDATE OR DELETE ON delivery_intents FOR EACH ROW EXECUTE FUNCTION emad_immutable_public_evidence();

-- +goose Down
-- +goose StatementBegin
DO $$ BEGIN RAISE EXCEPTION 'delivery history requires forward recovery'; END $$;
-- +goose StatementEnd
