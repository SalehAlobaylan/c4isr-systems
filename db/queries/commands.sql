-- name: CreateCommand :one
INSERT INTO commands (
    id, asset_id, mission_id, incident_id, type, payload, state,
    created_by, correlation_id, created_at, updated_at
) VALUES (
    @id, @asset_id, @mission_id, @incident_id, @type, @payload, @state,
    @created_by, @correlation_id, now(), now()
)
RETURNING *;

-- name: GetCommand :one
SELECT * FROM commands WHERE id = @id;

-- name: ListCommands :many
SELECT * FROM commands
WHERE (sqlc.narg('asset_id')::text IS NULL OR asset_id = sqlc.narg('asset_id'))
  AND (sqlc.narg('state')::text IS NULL OR state = sqlc.narg('state'))
  AND (sqlc.narg('mission_id')::text IS NULL OR mission_id = sqlc.narg('mission_id'))
ORDER BY created_at DESC
LIMIT @limit_count OFFSET @offset_count;

-- name: CountCommands :one
SELECT count(*) FROM commands
WHERE (sqlc.narg('asset_id')::text IS NULL OR asset_id = sqlc.narg('asset_id'))
  AND (sqlc.narg('state')::text IS NULL OR state = sqlc.narg('state'))
  AND (sqlc.narg('mission_id')::text IS NULL OR mission_id = sqlc.narg('mission_id'));

-- name: TransitionCommand :one
UPDATE commands
SET state = @state,
    updated_at = now(),
    queued_at = CASE WHEN @state::text = 'QUEUED' AND queued_at IS NULL THEN now() ELSE queued_at END,
    sent_at = CASE WHEN @state::text = 'SENT' AND sent_at IS NULL THEN now() ELSE sent_at END,
    acknowledged_at = CASE WHEN @state::text = 'ACKNOWLEDGED' AND acknowledged_at IS NULL THEN now() ELSE acknowledged_at END,
    completed_at = CASE WHEN @state::text = 'COMPLETED' AND completed_at IS NULL THEN now() ELSE completed_at END,
    failure_reason = CASE WHEN @state::text IN ('REJECTED', 'FAILED', 'TIMED_OUT')
                          THEN @failure_reason ELSE failure_reason END
WHERE id = @id AND state = @from_state
RETURNING *;
