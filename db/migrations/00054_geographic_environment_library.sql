-- +goose Up
CREATE TABLE geographic_assets (
 id text NOT NULL, scope_id text NOT NULL DEFAULT NULLIF(current_setting('emad.scope_id',true),''),
 source_id text NOT NULL, admission_id text NOT NULL, owner_id text NOT NULL,
 required_sources jsonb NOT NULL CHECK(jsonb_typeof(required_sources)='array' AND required_sources @> jsonb_build_array(source_id)),
 document jsonb NOT NULL CHECK(octet_length(document::text)<=32768),
 object_key text NOT NULL UNIQUE, size_bytes bigint NOT NULL CHECK(size_bytes BETWEEN 1 AND 1073741824),
 state text NOT NULL DEFAULT 'PENDING' CHECK(state IN('PENDING','READY','EXPIRED')),
 retain_until timestamptz NOT NULL, created_at timestamptz NOT NULL DEFAULT clock_timestamp(),
 upload_until timestamptz, purged_at timestamptz,
 PRIMARY KEY(id,scope_id),
 FOREIGN KEY(admission_id,source_id,scope_id) REFERENCES source_admissions(id,source_id,scope_id)
);
CREATE INDEX geographic_assets_inventory ON geographic_assets(scope_id,id);
CREATE INDEX geographic_assets_expiry ON geographic_assets(scope_id,retain_until) WHERE state<>'EXPIRED';
CREATE TABLE geographic_environment_definitions (
 id text NOT NULL, scope_id text NOT NULL DEFAULT NULLIF(current_setting('emad.scope_id',true),''),
 revision integer NOT NULL CHECK(revision>0), area_revision_id text NOT NULL, profile_id text NOT NULL,
 required_sources jsonb NOT NULL, document jsonb NOT NULL CHECK(octet_length(document::text)<=32768),
 PRIMARY KEY(id,scope_id,revision),
 FOREIGN KEY(area_revision_id,scope_id) REFERENCES area_revisions(id,scope_id)
);
CREATE TABLE geographic_environment_builds (
 id text NOT NULL, scope_id text NOT NULL DEFAULT NULLIF(current_setting('emad.scope_id',true),''),
 definition_id text NOT NULL, definition_revision integer NOT NULL, owner_id text NOT NULL, credential_id text NOT NULL,
 request_key text NOT NULL, input_hash text NOT NULL, recipe jsonb NOT NULL CHECK(octet_length(recipe::text)<=8388608),
 required_sources jsonb NOT NULL, state text NOT NULL DEFAULT 'QUEUED' CHECK(state IN('QUEUED','RUNNING','SUCCEEDED','FAILED','CANCELED')),
 attempt integer NOT NULL DEFAULT 0 CHECK(attempt BETWEEN 0 AND 3), lease_id text NOT NULL DEFAULT '', worker_id text NOT NULL DEFAULT '',
 lease_until timestamptz, next_attempt_at timestamptz NOT NULL DEFAULT clock_timestamp(),
 failure_code text NOT NULL DEFAULT '', created_at timestamptz NOT NULL DEFAULT clock_timestamp(),
 PRIMARY KEY(id,scope_id), UNIQUE(scope_id,owner_id,request_key),
 FOREIGN KEY(definition_id,scope_id,definition_revision) REFERENCES geographic_environment_definitions(id,scope_id,revision)
);
CREATE INDEX geographic_environment_queue ON geographic_environment_builds(scope_id,next_attempt_at,id) WHERE state IN('QUEUED','RUNNING');
CREATE TABLE geographic_environment_build_events (
 id text NOT NULL, scope_id text NOT NULL DEFAULT NULLIF(current_setting('emad.scope_id',true),''),
 build_id text NOT NULL, required_sources jsonb NOT NULL, attempt integer NOT NULL, state text NOT NULL, code text NOT NULL,
 occurred_at timestamptz NOT NULL DEFAULT clock_timestamp(),
 PRIMARY KEY(id,scope_id), FOREIGN KEY(build_id,scope_id) REFERENCES geographic_environment_builds(id,scope_id)
);
CREATE INDEX geographic_environment_attempts ON geographic_environment_build_events(scope_id,build_id,occurred_at,id);
CREATE TABLE geographic_environment_releases (
 id text NOT NULL, scope_id text NOT NULL DEFAULT NULLIF(current_setting('emad.scope_id',true),''),
 build_id text NOT NULL, required_sources jsonb NOT NULL, manifest_bytes bytea NOT NULL CHECK(octet_length(manifest_bytes) BETWEEN 1 AND 8388608),
 manifest_hash text NOT NULL, bundle_id text NOT NULL, created_at timestamptz NOT NULL DEFAULT clock_timestamp(),
 PRIMARY KEY(id,scope_id), UNIQUE(build_id,scope_id),
 FOREIGN KEY(build_id,scope_id) REFERENCES geographic_environment_builds(id,scope_id),
 FOREIGN KEY(bundle_id,scope_id) REFERENCES geographic_assets(id,scope_id)
);
CREATE TABLE geographic_environment_release_events (
 id text NOT NULL, scope_id text NOT NULL DEFAULT NULLIF(current_setting('emad.scope_id',true),''),
 release_id text NOT NULL, required_sources jsonb NOT NULL,
 state text NOT NULL CHECK(state IN('STAGED','VERIFIED','RELEASED','SUPERSEDED','WITHDRAWN','EXPIRED')),
 human_id text NOT NULL, note text NOT NULL, occurred_at timestamptz NOT NULL DEFAULT clock_timestamp(),
 PRIMARY KEY(id,scope_id), FOREIGN KEY(release_id,scope_id) REFERENCES geographic_environment_releases(id,scope_id)
);
CREATE INDEX geographic_environment_release_history ON geographic_environment_release_events(scope_id,release_id,occurred_at DESC,id DESC);
-- +goose StatementBegin
DO $$ DECLARE t text; BEGIN
 FOREACH t IN ARRAY ARRAY['geographic_assets','geographic_environment_definitions','geographic_environment_builds','geographic_environment_build_events','geographic_environment_releases','geographic_environment_release_events'] LOOP
  EXECUTE format('ALTER TABLE %I ENABLE ROW LEVEL SECURITY',t);
  EXECUTE format('ALTER TABLE %I FORCE ROW LEVEL SECURITY',t);
  EXECUTE format('GRANT SELECT,INSERT ON %I TO emad_data',t);
  EXECUTE format('CREATE POLICY geographic_scope ON %I TO emad_data USING(scope_id=NULLIF(current_setting(''emad.scope_id'',true),'''') AND split_part(scope_id,''/'',3)=''PUBLIC_EVIDENCE'' AND emad_scope_sources_visible(required_sources))',t);
  IF t NOT IN ('geographic_assets','geographic_environment_builds') THEN
   EXECUTE format('CREATE TRIGGER geographic_immutable BEFORE UPDATE OR DELETE ON %I FOR EACH ROW EXECUTE FUNCTION emad_immutable_public_evidence()',t);
  END IF;
 END LOOP;
END $$;
-- +goose StatementEnd
GRANT UPDATE(state,upload_until,purged_at) ON geographic_assets TO emad_data;
GRANT UPDATE(state,attempt,lease_id,worker_id,lease_until,next_attempt_at,failure_code) ON geographic_environment_builds TO emad_data;
ALTER TABLE platform_worker_status DROP CONSTRAINT platform_worker_status_feature_check;
ALTER TABLE platform_worker_status ADD CHECK(feature IN('connectors','imagery','assets','runtime','recordings','preparation','environment-library'));
-- +goose Down
-- +goose StatementBegin
DO $$ BEGIN RAISE EXCEPTION 'geographic environment history requires forward recovery'; END $$;
-- +goose StatementEnd
