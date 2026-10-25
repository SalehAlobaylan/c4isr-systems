-- +goose Up
ALTER TABLE tracks ADD COLUMN external_source_id text;
ALTER TABLE tracks ADD CONSTRAINT tracks_id_scope_unique UNIQUE(id,scope_id);
ALTER TABLE tracks ADD CONSTRAINT tracks_external_source_scope_fk FOREIGN KEY(external_source_id,scope_id)REFERENCES sources(id,scope_id);
ALTER TABLE tracks ADD CONSTRAINT tracks_external_public CHECK(external_source_id IS NULL OR split_part(scope_id,'/',3)='PUBLIC_EVIDENCE');
CREATE POLICY tracks_external_source_visibility ON tracks AS RESTRICTIVE TO emad_data USING(external_source_id IS NULL OR COALESCE(current_setting('emad.source_ids',true),'null') IN ('null','[]') OR external_source_id IN(SELECT jsonb_array_elements_text(current_setting('emad.source_ids',true)::jsonb))) WITH CHECK(external_source_id IS NULL OR COALESCE(current_setting('emad.source_ids',true),'null') IN ('null','[]') OR external_source_id IN(SELECT jsonb_array_elements_text(current_setting('emad.source_ids',true)::jsonb)));
CREATE INDEX tracks_external_expiry ON tracks(scope_id,external_source_id,last_seen_at) WHERE external_source_id IS NOT NULL;
CREATE TABLE external_track_segments(
 track_id text PRIMARY KEY,scope_id text NOT NULL DEFAULT NULLIF(current_setting('emad.scope_id',true),''),source_id text NOT NULL,kind text NOT NULL,identity_claim text NOT NULL,
 first_seen_at timestamptz NOT NULL,last_seen_at timestamptz NOT NULL,latest_evidence_id text NOT NULL,
 document jsonb NOT NULL,UNIQUE(track_id,source_id,scope_id),
 FOREIGN KEY(track_id,scope_id)REFERENCES tracks(id,scope_id),FOREIGN KEY(source_id,scope_id)REFERENCES sources(id,scope_id),
 FOREIGN KEY(latest_evidence_id,source_id,scope_id)REFERENCES public_evidence(id,source_id,scope_id),CHECK(split_part(scope_id,'/',3)='PUBLIC_EVIDENCE')
);
CREATE INDEX external_track_identity_idx ON external_track_segments(scope_id,source_id,kind,identity_claim,first_seen_at DESC);
CREATE TABLE external_track_associations(
 evidence_id text PRIMARY KEY,scope_id text NOT NULL DEFAULT NULLIF(current_setting('emad.scope_id',true),''),source_id text NOT NULL,track_id text NOT NULL,observation_id text NOT NULL,
 observed_at timestamptz NOT NULL,document jsonb NOT NULL,
 FOREIGN KEY(evidence_id,source_id,scope_id)REFERENCES public_evidence(id,source_id,scope_id),
 FOREIGN KEY(track_id,source_id,scope_id)REFERENCES external_track_segments(track_id,source_id,scope_id),FOREIGN KEY(observation_id,scope_id)REFERENCES observations(id,scope_id)
);
CREATE INDEX external_track_associations_history ON external_track_associations(scope_id,track_id,observed_at DESC);
-- +goose StatementBegin
DO $$ DECLARE t text;BEGIN
 FOREACH t IN ARRAY ARRAY['external_track_segments','external_track_associations'] LOOP
  EXECUTE format('ALTER TABLE %I ENABLE ROW LEVEL SECURITY',t);EXECUTE format('ALTER TABLE %I FORCE ROW LEVEL SECURITY',t);
  EXECUTE format('GRANT SELECT,INSERT ON %I TO emad_data',t);
  EXECUTE format($p$CREATE POLICY %I ON %I TO emad_data USING(scope_id=NULLIF(current_setting('emad.scope_id',true),'')) WITH CHECK(scope_id=NULLIF(current_setting('emad.scope_id',true),''))$p$,t||'_scope',t);
  EXECUTE format($p$CREATE POLICY %I ON %I AS RESTRICTIVE TO emad_data USING(COALESCE(current_setting('emad.source_ids',true),'null') IN ('null','[]') OR source_id IN(SELECT jsonb_array_elements_text(current_setting('emad.source_ids',true)::jsonb))) WITH CHECK(COALESCE(current_setting('emad.source_ids',true),'null') IN ('null','[]') OR source_id IN(SELECT jsonb_array_elements_text(current_setting('emad.source_ids',true)::jsonb)))$p$,t||'_source',t);
 END LOOP;
 FOREACH t IN ARRAY ARRAY['track_state','track_history','track_observations'] LOOP
  EXECUTE format('CREATE POLICY %I ON %I AS RESTRICTIVE TO emad_data USING(EXISTS(SELECT 1 FROM tracks t WHERE t.id=track_id)) WITH CHECK(EXISTS(SELECT 1 FROM tracks t WHERE t.id=track_id))',t||'_external_visibility',t);
 END LOOP;
END $$;
-- +goose StatementEnd
GRANT UPDATE(first_seen_at,last_seen_at,latest_evidence_id,document) ON external_track_segments TO emad_data;
CREATE TRIGGER external_track_associations_immutable BEFORE UPDATE OR DELETE ON external_track_associations FOR EACH ROW EXECUTE FUNCTION emad_immutable_public_evidence();
-- +goose Down
-- +goose StatementBegin
DO $$ BEGIN RAISE EXCEPTION 'external evidence and association history require forward recovery';END $$;
-- +goose StatementEnd
