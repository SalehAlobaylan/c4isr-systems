-- +goose Up
CREATE TABLE platform_worker_status(
 scope_id text NOT NULL REFERENCES data_scopes(id),principal_id text NOT NULL,
 feature text NOT NULL CHECK(feature IN('connectors','imagery','assets','runtime','recordings','preparation')),
 runtime_build text NOT NULL DEFAULT '',last_seen_at timestamptz NOT NULL DEFAULT now(),
 PRIMARY KEY(scope_id,principal_id,feature)
);
ALTER TABLE platform_worker_status ENABLE ROW LEVEL SECURITY;
ALTER TABLE platform_worker_status FORCE ROW LEVEL SECURITY;
GRANT SELECT,INSERT,UPDATE ON platform_worker_status TO emad_data;
CREATE POLICY worker_status_scope ON platform_worker_status TO emad_data
 USING(scope_id=NULLIF(current_setting('emad.scope_id',true),''))
 WITH CHECK(scope_id=NULLIF(current_setting('emad.scope_id',true),''));
-- +goose Down
DROP TABLE platform_worker_status;
