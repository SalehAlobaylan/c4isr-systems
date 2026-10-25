-- +goose Up
ALTER TABLE observations ADD COLUMN semantic_kind text NOT NULL DEFAULT 'moving';
ALTER TABLE observations ADD COLUMN geometry_role text NOT NULL DEFAULT 'reported_location';
UPDATE observations SET semantic_kind='area',geometry_role='reported_extent' WHERE area IS NOT NULL;
UPDATE observations SET semantic_kind='point_event' WHERE area IS NULL AND
 (observation_type IN ('earthquake','thermal.detection','event.earthquake','detection.thermal') OR observation_type LIKE 'event.%' OR observation_type LIKE 'detection.%');
ALTER TABLE observations ADD CONSTRAINT observation_semantic_kind CHECK(semantic_kind IN ('moving','point_event','area'));
ALTER TABLE observations ADD CONSTRAINT observation_geometry_role CHECK(geometry_role IN ('reported_location','reported_extent','measurement_footprint','derived_buffer','phenomenon_boundary'));
CREATE UNIQUE INDEX observations_id_scope_unique ON observations(id,scope_id);
CREATE TABLE public_event_revisions (
 evidence_id text PRIMARY KEY, scope_id text NOT NULL DEFAULT NULLIF(current_setting('emad.scope_id',true),''), source_id text NOT NULL,
 provider_record_id text NOT NULL, revision_at timestamptz NOT NULL, observation_id text,
 supersedes_id text, document jsonb NOT NULL,
 UNIQUE(evidence_id,source_id,scope_id),
 FOREIGN KEY(evidence_id,source_id,scope_id) REFERENCES public_evidence(id,source_id,scope_id),
 FOREIGN KEY(observation_id,scope_id) REFERENCES observations(id,scope_id),
 FOREIGN KEY(supersedes_id,source_id,scope_id) REFERENCES public_event_revisions(evidence_id,source_id,scope_id),
 CHECK(split_part(scope_id,'/',3)='PUBLIC_EVIDENCE')
);
CREATE INDEX public_events_origin_idx ON public_event_revisions(scope_id,source_id,provider_record_id,revision_at DESC,evidence_id DESC);
CREATE TABLE public_event_current (
 scope_id text NOT NULL DEFAULT NULLIF(current_setting('emad.scope_id',true),''), source_id text NOT NULL,provider_record_id text NOT NULL,evidence_id text NOT NULL,
 PRIMARY KEY(scope_id,source_id,provider_record_id),
 FOREIGN KEY(evidence_id,source_id,scope_id) REFERENCES public_event_revisions(evidence_id,source_id,scope_id),
 CHECK(split_part(scope_id,'/',3)='PUBLIC_EVIDENCE')
);
-- +goose StatementBegin
DO $$ DECLARE t text; BEGIN
 FOREACH t IN ARRAY ARRAY['public_event_revisions','public_event_current'] LOOP
  EXECUTE format('ALTER TABLE %I ENABLE ROW LEVEL SECURITY',t);
  EXECUTE format('ALTER TABLE %I FORCE ROW LEVEL SECURITY',t);
  EXECUTE format('GRANT SELECT,INSERT,UPDATE ON %I TO emad_data',t);
  EXECUTE format('CREATE POLICY %I ON %I TO emad_data USING (scope_id=NULLIF(current_setting(''emad.scope_id'',true),'''')) WITH CHECK (scope_id=NULLIF(current_setting(''emad.scope_id'',true),''''))',t||'_scope',t);
  EXECUTE format('CREATE POLICY %I ON %I AS RESTRICTIVE TO emad_data USING (COALESCE(current_setting(''emad.source_ids'',true),''null'') IN (''null'',''[]'') OR source_id IN (SELECT jsonb_array_elements_text(current_setting(''emad.source_ids'',true)::jsonb))) WITH CHECK (COALESCE(current_setting(''emad.source_ids'',true),''null'') IN (''null'',''[]'') OR source_id IN (SELECT jsonb_array_elements_text(current_setting(''emad.source_ids'',true)::jsonb)))',t||'_source',t);
 END LOOP;
END $$;
CREATE TRIGGER public_event_revisions_immutable BEFORE UPDATE OR DELETE ON public_event_revisions FOR EACH ROW EXECUTE FUNCTION emad_immutable_public_evidence();
-- +goose StatementEnd
REVOKE UPDATE ON public_event_revisions FROM emad_data;

-- +goose Down
-- +goose StatementBegin
DO $$ BEGIN RAISE EXCEPTION 'public event evidence requires forward recovery'; END $$;
-- +goose StatementEnd
