-- +goose Up
CREATE TABLE geographic_environment_displays (
 id text NOT NULL,
 scope_id text NOT NULL DEFAULT NULLIF(current_setting('emad.scope_id',true),''),
 release_id text NOT NULL,
 asset_id text NOT NULL,
 owner_id text NOT NULL,
 request_key text NOT NULL,
 required_sources jsonb NOT NULL,
 document jsonb NOT NULL CHECK(octet_length(document::text)<=32768),
 created_at timestamptz NOT NULL DEFAULT clock_timestamp(),
 PRIMARY KEY(id,scope_id),
 UNIQUE(scope_id,release_id,owner_id,request_key),
 UNIQUE(asset_id,scope_id),
 FOREIGN KEY(release_id,scope_id) REFERENCES geographic_environment_releases(id,scope_id),
 FOREIGN KEY(asset_id,scope_id) REFERENCES geographic_assets(id,scope_id)
);
CREATE INDEX geographic_environment_display_release
 ON geographic_environment_displays(scope_id,release_id,id);
ALTER TABLE geographic_environment_displays ENABLE ROW LEVEL SECURITY;
ALTER TABLE geographic_environment_displays FORCE ROW LEVEL SECURITY;
GRANT SELECT,INSERT ON geographic_environment_displays TO emad_data;
CREATE POLICY geographic_scope ON geographic_environment_displays TO emad_data
 USING(scope_id=NULLIF(current_setting('emad.scope_id',true),'')
  AND split_part(scope_id,'/',3)='PUBLIC_EVIDENCE' AND emad_scope_sources_visible(required_sources));
CREATE TRIGGER geographic_immutable BEFORE UPDATE OR DELETE ON geographic_environment_displays
 FOR EACH ROW EXECUTE FUNCTION emad_immutable_public_evidence();

-- +goose Down
-- +goose StatementBegin
DO $$ BEGIN RAISE EXCEPTION 'geographic display history requires forward recovery'; END $$;
-- +goose StatementEnd
