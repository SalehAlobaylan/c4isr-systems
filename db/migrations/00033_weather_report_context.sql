-- +goose Up
CREATE TABLE public_context_records(
 evidence_id text PRIMARY KEY,scope_id text NOT NULL DEFAULT NULLIF(current_setting('emad.scope_id',true),''),source_id text NOT NULL,
 kind text NOT NULL CHECK(kind IN('WEATHER','REPORT')),record_key text NOT NULL,
 valid_from timestamptz NOT NULL,valid_to timestamptz NOT NULL,revision_at timestamptz NOT NULL,known_at timestamptz NOT NULL,
 position geometry(Point,4326) NOT NULL,
 FOREIGN KEY(evidence_id,source_id,scope_id) REFERENCES public_evidence(id,source_id,scope_id),
 CHECK(valid_to>=valid_from),CHECK(split_part(scope_id,'/',3)='PUBLIC_EVIDENCE')
);
CREATE INDEX public_context_current ON public_context_records(scope_id,source_id,record_key,revision_at DESC,known_at DESC,evidence_id);
CREATE INDEX public_context_position ON public_context_records USING gist(position);
ALTER TABLE public_context_records ENABLE ROW LEVEL SECURITY;
ALTER TABLE public_context_records FORCE ROW LEVEL SECURITY;
GRANT SELECT,INSERT ON public_context_records TO emad_data;
CREATE POLICY public_context_scope ON public_context_records TO emad_data USING(scope_id=NULLIF(current_setting('emad.scope_id',true),'')) WITH CHECK(scope_id=NULLIF(current_setting('emad.scope_id',true),''));
CREATE POLICY public_context_source ON public_context_records AS RESTRICTIVE TO emad_data USING(COALESCE(current_setting('emad.source_ids',true),'null') IN ('null','[]') OR source_id IN(SELECT jsonb_array_elements_text(current_setting('emad.source_ids',true)::jsonb))) WITH CHECK(COALESCE(current_setting('emad.source_ids',true),'null') IN ('null','[]') OR source_id IN(SELECT jsonb_array_elements_text(current_setting('emad.source_ids',true)::jsonb)));
CREATE TRIGGER public_context_immutable BEFORE UPDATE OR DELETE ON public_context_records FOR EACH ROW EXECUTE FUNCTION emad_immutable_public_evidence();
-- +goose Down
-- +goose StatementBegin
DO $$ BEGIN RAISE EXCEPTION 'weather and report evidence requires forward recovery';END $$;
-- +goose StatementEnd
