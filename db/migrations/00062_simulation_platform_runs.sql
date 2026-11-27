-- +goose Up
-- Durable run registry, cross-simulation sensor-network handoff layouts, and
-- audited live interactive session commands for the EMAD Simulation Platform.
CREATE TABLE simulation_platform_runs (
 id text NOT NULL,
 scope_id text NOT NULL DEFAULT NULLIF(current_setting('emad.scope_id',true),''),
 engine text NOT NULL CHECK(engine IN('sim','coverage','salvo','bench')),
 environment_id text NOT NULL,
 title text NOT NULL DEFAULT '',
 manifest_sha256 text NOT NULL DEFAULT '',
 scenario_sha256 text NOT NULL DEFAULT '',
 seed bigint NOT NULL DEFAULT 0,
 status text NOT NULL CHECK(status IN('queued','running','completed','failed','cancelled','interrupted')),
 stage text NOT NULL DEFAULT '',
 worker_job_id text NOT NULL DEFAULT '',
 created_by text NOT NULL,
 created_at timestamptz NOT NULL DEFAULT clock_timestamp(),
 completed_at timestamptz,
 error_message text NOT NULL DEFAULT '',
 summary jsonb NOT NULL DEFAULT '{}'::jsonb,
 result_overlay jsonb,
 replay_obs_blob_key text NOT NULL DEFAULT '',
 replay_obs_bytes bigint NOT NULL DEFAULT 0,
 replay_debrief_blob_key text NOT NULL DEFAULT '',
 replay_debrief_bytes bigint NOT NULL DEFAULT 0,
 PRIMARY KEY(id,scope_id)
);
CREATE INDEX simulation_platform_runs_recent
 ON simulation_platform_runs(scope_id,engine,created_at DESC);

ALTER TABLE simulation_platform_runs ENABLE ROW LEVEL SECURITY;
ALTER TABLE simulation_platform_runs FORCE ROW LEVEL SECURITY;
GRANT SELECT,INSERT,UPDATE ON simulation_platform_runs TO emad_data;
CREATE POLICY sim_platform_runs_scope ON simulation_platform_runs TO emad_data
 USING(scope_id=NULLIF(current_setting('emad.scope_id',true),''))
 WITH CHECK(scope_id=NULLIF(current_setting('emad.scope_id',true),''));

CREATE TABLE simulation_sensor_networks (
 id text NOT NULL,
 scope_id text NOT NULL DEFAULT NULLIF(current_setting('emad.scope_id',true),''),
 name text NOT NULL,
 environment_id text NOT NULL,
 manifest_sha256 text NOT NULL DEFAULT '',
 source_job_id text NOT NULL DEFAULT '',
 sensor_count integer NOT NULL DEFAULT 0,
 document jsonb NOT NULL,
 created_by text NOT NULL,
 created_at timestamptz NOT NULL DEFAULT clock_timestamp(),
 PRIMARY KEY(id,scope_id)
);
CREATE INDEX simulation_sensor_networks_env
 ON simulation_sensor_networks(scope_id,environment_id,created_at DESC);

ALTER TABLE simulation_sensor_networks ENABLE ROW LEVEL SECURITY;
ALTER TABLE simulation_sensor_networks FORCE ROW LEVEL SECURITY;
GRANT SELECT,INSERT,UPDATE ON simulation_sensor_networks TO emad_data;
CREATE POLICY sim_sensor_networks_scope ON simulation_sensor_networks TO emad_data
 USING(scope_id=NULLIF(current_setting('emad.scope_id',true),''))
 WITH CHECK(scope_id=NULLIF(current_setting('emad.scope_id',true),''));

CREATE TABLE simulation_session_commands (
 id text NOT NULL,
 scope_id text NOT NULL DEFAULT NULLIF(current_setting('emad.scope_id',true),''),
 session_id text NOT NULL,
 tick integer NOT NULL,
 virtual_time_s double precision NOT NULL,
 command_kind text NOT NULL,
 payload jsonb NOT NULL DEFAULT '{}'::jsonb,
 actor_id text NOT NULL,
 created_at timestamptz NOT NULL DEFAULT clock_timestamp(),
 PRIMARY KEY(id,scope_id)
);
CREATE INDEX simulation_session_commands_session
 ON simulation_session_commands(scope_id,session_id,tick,created_at);

ALTER TABLE simulation_session_commands ENABLE ROW LEVEL SECURITY;
ALTER TABLE simulation_session_commands FORCE ROW LEVEL SECURITY;
GRANT SELECT,INSERT ON simulation_session_commands TO emad_data;
CREATE POLICY sim_session_commands_scope ON simulation_session_commands TO emad_data
 USING(scope_id=NULLIF(current_setting('emad.scope_id',true),''))
 WITH CHECK(scope_id=NULLIF(current_setting('emad.scope_id',true),''));

-- +goose Down
DROP TABLE IF EXISTS simulation_session_commands;
DROP TABLE IF EXISTS simulation_sensor_networks;
DROP TABLE IF EXISTS simulation_platform_runs;
