-- name: CreateScenarioRunEvent :exec
INSERT INTO scenario_run_events (
    run_id, sequence, virtual_time_ms, action_name, status, error, created_at, updated_at
) VALUES (
    @run_id, @sequence, @virtual_time_ms, @action_name, @status, @error, now(), now()
)
ON CONFLICT (run_id, sequence) DO UPDATE SET
    virtual_time_ms = EXCLUDED.virtual_time_ms,
    action_name = EXCLUDED.action_name,
    status = EXCLUDED.status,
    error = EXCLUDED.error,
    updated_at = now();

-- name: UpdateScenarioRunEvent :exec
UPDATE scenario_run_events
SET status = @status,
    error = @error,
    updated_at = now()
WHERE run_id = @run_id AND sequence = @sequence;

-- name: ListScenarioRunEvents :many
SELECT run_id, sequence, virtual_time_ms, action_name, status, error, created_at, updated_at
FROM scenario_run_events
WHERE run_id = @run_id
ORDER BY virtual_time_ms, sequence;

-- name: SkipPendingScenarioRunEvents :exec
UPDATE scenario_run_events
SET status = 'skipped',
    error = COALESCE(NULLIF(@error, ''), error),
    updated_at = now()
WHERE run_id = @run_id AND status IN ('pending', 'running');

-- name: UpdateScenarioRunCursor :one
UPDATE scenario_runs
SET virtual_time_ms = @virtual_time_ms,
    last_action = @last_action,
    last_action_at_ms = @last_action_at_ms,
    action_error = @action_error
WHERE id = @id
RETURNING *;
