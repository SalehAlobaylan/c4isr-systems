-- +goose Up
CREATE INDEX public_evidence_catalog_known_idx ON public_evidence_catalog(scope_id,published_at,evidence_id);
CREATE INDEX public_event_revision_history_idx ON public_event_revisions(scope_id,source_id,provider_record_id,revision_at DESC,evidence_id DESC);
CREATE INDEX observations_public_position_idx ON observations USING gist((position::geometry)) WHERE position IS NOT NULL;
-- +goose Down
DROP INDEX IF EXISTS observations_public_position_idx;
DROP INDEX IF EXISTS public_event_revision_history_idx;
DROP INDEX IF EXISTS public_evidence_catalog_known_idx;
