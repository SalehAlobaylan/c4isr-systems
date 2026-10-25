-- +goose Up
ALTER TABLE assessments ADD COLUMN review_source_id text;
CREATE POLICY audit_review_source_visibility ON audit_events AS RESTRICTIVE TO emad_data USING(action<>'assessment.revision.recorded' OR COALESCE(current_setting('emad.source_ids',true),'null') IN ('null','[]') OR data->>'sourceId' IN(SELECT jsonb_array_elements_text(current_setting('emad.source_ids',true)::jsonb))) WITH CHECK(action<>'assessment.revision.recorded' OR COALESCE(current_setting('emad.source_ids',true),'null') IN ('null','[]') OR data->>'sourceId' IN(SELECT jsonb_array_elements_text(current_setting('emad.source_ids',true)::jsonb)));
ALTER TABLE assessments ADD CONSTRAINT assessments_id_scope_unique UNIQUE(id,scope_id);
ALTER TABLE assessments ADD CONSTRAINT assessment_review_source_fk FOREIGN KEY(review_source_id,scope_id)REFERENCES sources(id,scope_id);
CREATE POLICY assessment_review_source_visibility ON assessments AS RESTRICTIVE TO emad_data USING(review_source_id IS NULL OR COALESCE(current_setting('emad.source_ids',true),'null') IN ('null','[]') OR review_source_id IN(SELECT jsonb_array_elements_text(current_setting('emad.source_ids',true)::jsonb))) WITH CHECK(review_source_id IS NULL OR COALESCE(current_setting('emad.source_ids',true),'null') IN ('null','[]') OR review_source_id IN(SELECT jsonb_array_elements_text(current_setting('emad.source_ids',true)::jsonb)));
CREATE TABLE assessment_revisions(
 id text PRIMARY KEY,assessment_id text NOT NULL,scope_id text NOT NULL DEFAULT NULLIF(current_setting('emad.scope_id',true),''),source_id text NOT NULL,
 version integer NOT NULL CHECK(version>0),status text NOT NULL CHECK(status IN ('DRAFT','PROPOSED','IN_REVIEW','PUBLISHED','REJECTED')),document jsonb NOT NULL,
 UNIQUE(assessment_id,version),UNIQUE(id,assessment_id,source_id,scope_id),
 FOREIGN KEY(assessment_id,scope_id)REFERENCES assessments(id,scope_id),FOREIGN KEY(source_id,scope_id)REFERENCES sources(id,scope_id),
 CHECK(split_part(scope_id,'/',3)='PUBLIC_EVIDENCE'),CHECK(octet_length(document::text)<=262144)
);
CREATE TABLE assessment_review_heads(
 assessment_id text PRIMARY KEY,scope_id text NOT NULL DEFAULT NULLIF(current_setting('emad.scope_id',true),''),source_id text NOT NULL,
 latest_id text NOT NULL,published_id text,
 FOREIGN KEY(latest_id,assessment_id,source_id,scope_id)REFERENCES assessment_revisions(id,assessment_id,source_id,scope_id),
 FOREIGN KEY(published_id,assessment_id,source_id,scope_id)REFERENCES assessment_revisions(id,assessment_id,source_id,scope_id)
);
-- +goose StatementBegin
DO $$ DECLARE t text; BEGIN
 FOREACH t IN ARRAY ARRAY['assessment_revisions','assessment_review_heads'] LOOP
  EXECUTE format('ALTER TABLE %I ENABLE ROW LEVEL SECURITY',t);EXECUTE format('ALTER TABLE %I FORCE ROW LEVEL SECURITY',t);
  EXECUTE format('GRANT SELECT,INSERT ON %I TO emad_data',t);
  EXECUTE format($policy$CREATE POLICY %I ON %I TO emad_data USING(scope_id=NULLIF(current_setting('emad.scope_id',true),'')) WITH CHECK(scope_id=NULLIF(current_setting('emad.scope_id',true),''))$policy$,t||'_scope',t);
  EXECUTE format('CREATE POLICY %I ON %I AS RESTRICTIVE TO emad_data USING(COALESCE(current_setting(''emad.source_ids'',true),''null'') IN (''null'',''[]'') OR source_id IN(SELECT jsonb_array_elements_text(current_setting(''emad.source_ids'',true)::jsonb))) WITH CHECK(COALESCE(current_setting(''emad.source_ids'',true),''null'') IN (''null'',''[]'') OR source_id IN(SELECT jsonb_array_elements_text(current_setting(''emad.source_ids'',true)::jsonb)))',t||'_source',t);
 END LOOP;
END $$;
-- +goose StatementEnd
GRANT UPDATE(latest_id,published_id) ON assessment_review_heads TO emad_data;
CREATE TRIGGER assessment_revisions_immutable BEFORE UPDATE OR DELETE ON assessment_revisions FOR EACH ROW EXECUTE FUNCTION emad_immutable_public_evidence();
-- +goose StatementBegin
CREATE FUNCTION emad_assessment_review_source_immutable()RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN
 IF NEW.review_source_id IS DISTINCT FROM OLD.review_source_id THEN RAISE EXCEPTION 'assessment review source is immutable';END IF;RETURN NEW;
END $$;
-- +goose StatementEnd
CREATE TRIGGER assessment_review_source_immutable BEFORE UPDATE ON assessments FOR EACH ROW EXECUTE FUNCTION emad_assessment_review_source_immutable();
-- +goose Down
-- +goose StatementBegin
DO $$ BEGIN RAISE EXCEPTION 'review history requires forward recovery';END $$;
-- +goose StatementEnd
