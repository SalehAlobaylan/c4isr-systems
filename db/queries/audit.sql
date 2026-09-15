-- name: CreateAuditEvent :one
INSERT INTO audit_events (
    id, occurred_at, actor_type, actor_id, action,
    subject_type, subject_id, correlation_id, data, created_at
) VALUES (
    @id, @occurred_at, @actor_type, @actor_id, @action,
    @subject_type, @subject_id, @correlation_id, @data, now()
)
RETURNING *;

-- name: ListAuditEvents :many
SELECT * FROM audit_events
WHERE (sqlc.narg('subject_type')::text IS NULL OR subject_type = sqlc.narg('subject_type'))
  AND (sqlc.narg('subject_id')::text IS NULL OR subject_id = sqlc.narg('subject_id'))
  AND (sqlc.narg('action')::text IS NULL OR action = sqlc.narg('action'))
  AND (@since::timestamptz IS NULL OR occurred_at >= @since)
ORDER BY occurred_at DESC, id DESC
LIMIT @limit_count OFFSET @offset_count;

-- name: CountAuditEvents :one
SELECT count(*) FROM audit_events
WHERE (sqlc.narg('subject_type')::text IS NULL OR subject_type = sqlc.narg('subject_type'))
  AND (sqlc.narg('subject_id')::text IS NULL OR subject_id = sqlc.narg('subject_id'))
  AND (sqlc.narg('action')::text IS NULL OR action = sqlc.narg('action'))
  AND (@since::timestamptz IS NULL OR occurred_at >= @since);
