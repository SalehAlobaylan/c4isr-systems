-- name: CreateScenarioRun :one
INSERT INTO scenario_runs (
    id, scenario_name, seed, status, playback_speed, virtual_time_ms, started_at, created_at
) VALUES (
    @id, @scenario_name, @seed, @status, @playback_speed, 0, now(), now()
)
RETURNING *;

-- name: GetScenarioRun :one
SELECT * FROM scenario_runs WHERE id = @id;

-- name: ListScenarioRuns :many
SELECT * FROM scenario_runs
ORDER BY started_at DESC
LIMIT @limit_count OFFSET @offset_count;

-- name: CountScenarioRuns :one
SELECT count(*)::bigint FROM scenario_runs;

-- name: UpdateScenarioRunStatus :one
UPDATE scenario_runs
SET status = @status,
    error = @error,
    ended_at = CASE WHEN @status IN ('COMPLETED', 'STOPPED', 'FAILED') THEN now() ELSE ended_at END
WHERE id = @id
RETURNING *;

-- name: UpdateScenarioRunSpeed :one
UPDATE scenario_runs
SET playback_speed = @playback_speed
WHERE id = @id
RETURNING *;

-- name: UpdateScenarioRunProgress :exec
UPDATE scenario_runs
SET virtual_time_ms = @virtual_time_ms
WHERE id = @id;

-- name: ListActiveScenarioRuns :many
SELECT * FROM scenario_runs WHERE status IN ('RUNNING', 'PAUSED') ORDER BY started_at DESC;
