-- +goose Up
CREATE EXTENSION IF NOT EXISTS unaccent;
-- Display/source aliases stay unchanged. This versioned derived expression
-- supports accent/case-insensitive discovery without rewriting retained rows.
-- +goose StatementBegin
CREATE FUNCTION emad_geographic_search(value text) RETURNS text LANGUAGE sql IMMUTABLE PARALLEL SAFE AS $$
 SELECT lower(public.unaccent('public.unaccent',value))
$$;
-- +goose StatementEnd
CREATE INDEX geographic_names_normalized ON geographic_regions(scope_id,emad_geographic_search(name),id);
CREATE TABLE geography_reference_requests (
 id text PRIMARY KEY, scope_id text NOT NULL DEFAULT NULLIF(current_setting('emad.scope_id',true),''),
 source_id text NOT NULL, request_key text NOT NULL, requested_by text NOT NULL,
 region_id text NOT NULL, document jsonb NOT NULL, resolution jsonb,
 UNIQUE(scope_id,requested_by,request_key), FOREIGN KEY(region_id,scope_id) REFERENCES geographic_regions(id,scope_id),
 FOREIGN KEY(source_id,scope_id) REFERENCES sources(id,scope_id)
);
ALTER TABLE geography_reference_requests ENABLE ROW LEVEL SECURITY;
ALTER TABLE geography_reference_requests FORCE ROW LEVEL SECURITY;
GRANT SELECT,INSERT,UPDATE ON geography_reference_requests TO emad_data;
CREATE POLICY geographic_request_scope ON geography_reference_requests TO emad_data
 USING(scope_id=NULLIF(current_setting('emad.scope_id',true),'') AND emad_scope_sources_visible(jsonb_build_array(source_id)))
 WITH CHECK(scope_id=NULLIF(current_setting('emad.scope_id',true),'') AND emad_scope_sources_visible(jsonb_build_array(source_id)));
-- +goose Down
-- +goose StatementBegin
DO $$ BEGIN RAISE EXCEPTION 'geographic requests and retained catalog require forward recovery'; END $$;
-- +goose StatementEnd
