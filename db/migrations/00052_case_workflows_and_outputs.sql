-- +goose Up
-- Retained assets need their own explicit purpose; no provenance-name inference.
ALTER TABLE content_purpose_revisions DROP CONSTRAINT content_purpose_revisions_entity_kind_check;
ALTER TABLE content_purpose_revisions ADD CONSTRAINT content_purpose_revisions_entity_kind_check CHECK(entity_kind IN ('reference','scene','capture','job','assessment','package','artifact'));
CREATE TABLE investigation_save_intents(
 scope_id text NOT NULL DEFAULT NULLIF(current_setting('emad.scope_id',true),''),human_id text NOT NULL,request_key text NOT NULL,
 input_hash text NOT NULL,case_id text NOT NULL,revision_id text NOT NULL,
 PRIMARY KEY(scope_id,human_id,request_key),
 FOREIGN KEY(case_id,scope_id) REFERENCES investigations(id,scope_id),
 FOREIGN KEY(revision_id,scope_id) REFERENCES investigation_revisions(id,scope_id)
);
CREATE TABLE investigation_finding_revisions(
 id text NOT NULL,scope_id text NOT NULL DEFAULT NULLIF(current_setting('emad.scope_id',true),''),case_id text NOT NULL,finding_id text NOT NULL,document jsonb NOT NULL,
 PRIMARY KEY(id,scope_id),FOREIGN KEY(case_id,scope_id) REFERENCES investigations(id,scope_id)
);
CREATE INDEX finding_case_history ON investigation_finding_revisions(scope_id,case_id,finding_id);
CREATE TABLE investigation_review_events(
 id text NOT NULL,scope_id text NOT NULL DEFAULT NULLIF(current_setting('emad.scope_id',true),''),case_id text NOT NULL,revision_id text NOT NULL,
 sequence bigint GENERATED ALWAYS AS IDENTITY,document jsonb NOT NULL,
 PRIMARY KEY(id,scope_id),FOREIGN KEY(case_id,scope_id) REFERENCES investigations(id,scope_id),
 FOREIGN KEY(revision_id,scope_id) REFERENCES investigation_revisions(id,scope_id)
);
CREATE INDEX investigation_review_latest ON investigation_review_events(scope_id,case_id,sequence DESC);
CREATE TABLE investigation_dependencies(
 scope_id text NOT NULL DEFAULT NULLIF(current_setting('emad.scope_id',true),''),case_id text NOT NULL,revision_id text NOT NULL,
 dependency_kind text NOT NULL,dependency_id text NOT NULL,source_id text NOT NULL,digest text NOT NULL,
 PRIMARY KEY(scope_id,revision_id,dependency_kind,dependency_id),FOREIGN KEY(case_id,scope_id) REFERENCES investigations(id,scope_id),
 FOREIGN KEY(revision_id,scope_id) REFERENCES investigation_revisions(id,scope_id)
);
CREATE INDEX investigation_reverse_dependency ON investigation_dependencies(scope_id,dependency_kind,dependency_id);
CREATE TABLE composite_outputs(
 id text NOT NULL,scope_id text NOT NULL DEFAULT NULLIF(current_setting('emad.scope_id',true),''),case_id text NOT NULL,revision_id text NOT NULL,
 content_digest text NOT NULL,creator_human_id text NOT NULL,request_key text NOT NULL,input_hash text NOT NULL,document jsonb NOT NULL,
 state text NOT NULL DEFAULT 'FROZEN' CHECK(state IN('FROZEN','READY','DELETE_PENDING','TOMBSTONED')),
 render_human_id text NOT NULL DEFAULT '',render_sha256 text NOT NULL DEFAULT '',media_type text NOT NULL DEFAULT '',size_bytes bigint NOT NULL DEFAULT 0,
 retain_until timestamptz NOT NULL,unavailable_reason text NOT NULL DEFAULT '',object_key text NOT NULL DEFAULT '',
 PRIMARY KEY(id,scope_id),UNIQUE(scope_id,creator_human_id,request_key),
 FOREIGN KEY(case_id,scope_id) REFERENCES investigations(id,scope_id),FOREIGN KEY(revision_id,scope_id) REFERENCES investigation_revisions(id,scope_id),
 CHECK(size_bytes BETWEEN 0 AND 8388608)
);
CREATE TABLE composite_output_events(
 id text NOT NULL,scope_id text NOT NULL DEFAULT NULLIF(current_setting('emad.scope_id',true),''),case_id text NOT NULL,output_id text NOT NULL,document jsonb NOT NULL,
 PRIMARY KEY(id,scope_id),FOREIGN KEY(case_id,scope_id) REFERENCES investigations(id,scope_id),FOREIGN KEY(output_id,scope_id) REFERENCES composite_outputs(id,scope_id)
);
-- +goose StatementBegin
DO $$ DECLARE t text; BEGIN
 FOREACH t IN ARRAY ARRAY['investigation_save_intents','investigation_finding_revisions','investigation_review_events','investigation_dependencies','composite_outputs','composite_output_events'] LOOP
  EXECUTE format('ALTER TABLE %I ENABLE ROW LEVEL SECURITY',t);
  EXECUTE format('ALTER TABLE %I FORCE ROW LEVEL SECURITY',t);
  EXECUTE format('GRANT SELECT,INSERT ON %I TO emad_data',t);
  EXECUTE format('CREATE POLICY case_content ON %I TO emad_data USING(emad_investigation_visible(case_id,scope_id)) WITH CHECK(emad_investigation_visible(case_id,scope_id))',t);
 END LOOP;
 FOREACH t IN ARRAY ARRAY['investigation_save_intents','investigation_finding_revisions','investigation_review_events','investigation_dependencies','composite_output_events'] LOOP
  EXECUTE format('CREATE TRIGGER immutable BEFORE UPDATE OR DELETE ON %I FOR EACH ROW EXECUTE FUNCTION emad_immutable_public_evidence()',t);
 END LOOP;
END $$;
-- +goose StatementEnd
GRANT USAGE ON SEQUENCE investigation_review_events_sequence_seq TO emad_data;
GRANT UPDATE ON composite_outputs TO emad_data;
CREATE POLICY save_intent_owner ON investigation_save_intents AS RESTRICTIVE TO emad_data
 USING(human_id=NULLIF(current_setting('emad.human_id',true),'')) WITH CHECK(human_id=NULLIF(current_setting('emad.human_id',true),''));
-- Preserve frozen manifests and render identity. Retention transitions only
-- remove bytes; approval is an append-only manifest-bound review event.
-- +goose StatementBegin
CREATE FUNCTION emad_composite_immutable() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN
 IF NEW.id<>OLD.id OR NEW.scope_id<>OLD.scope_id OR NEW.case_id<>OLD.case_id OR NEW.revision_id<>OLD.revision_id OR NEW.content_digest<>OLD.content_digest OR NEW.retain_until<>OLD.retain_until OR NEW.creator_human_id<>OLD.creator_human_id OR NEW.input_hash<>OLD.input_hash OR NEW.request_key<>OLD.request_key THEN RAISE EXCEPTION 'immutable output manifest'; END IF;
 IF NEW.document<>OLD.document AND NOT (NEW.state='TOMBSTONED' AND NEW.document=jsonb_build_object('contentDigest',OLD.content_digest,'caseRevisionId',OLD.revision_id,'tombstone',true)) THEN RAISE EXCEPTION 'immutable output content'; END IF;
 IF OLD.state<>'FROZEN' AND (NEW.render_human_id<>OLD.render_human_id OR NEW.render_sha256<>OLD.render_sha256 OR NEW.media_type<>OLD.media_type OR NEW.size_bytes<>OLD.size_bytes OR NEW.object_key<>OLD.object_key) THEN RAISE EXCEPTION 'immutable rendered identity'; END IF;
 IF OLD.state='READY' AND NEW.state NOT IN('READY','DELETE_PENDING') OR OLD.state='FROZEN' AND NEW.state NOT IN('FROZEN','READY','DELETE_PENDING') THEN RAISE EXCEPTION 'invalid output lifecycle transition'; END IF;
 IF OLD.state='TOMBSTONED' AND NEW.state<>'TOMBSTONED' OR OLD.state='DELETE_PENDING' AND NEW.state NOT IN('DELETE_PENDING','TOMBSTONED') THEN RAISE EXCEPTION 'output bytes cannot be restored'; END IF;
 RETURN NEW;
END $$;
-- +goose StatementEnd
CREATE TRIGGER composite_immutable BEFORE UPDATE ON composite_outputs FOR EACH ROW EXECUTE FUNCTION emad_composite_immutable();
-- +goose Down
-- +goose StatementBegin
DO $$ BEGIN RAISE EXCEPTION 'case history, review and output manifests require forward recovery'; END $$;
-- +goose StatementEnd
