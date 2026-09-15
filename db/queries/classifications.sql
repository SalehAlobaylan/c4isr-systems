-- name: CreateClassification :one
INSERT INTO classifications (
    id, track_id, label, confidence, method, source_reference, created_by, created_at
) VALUES (
    @id, @track_id, @label, @confidence, @method, @source_reference, @created_by, now()
)
RETURNING *;

-- name: GetClassification :one
SELECT * FROM classifications WHERE id = @id;

-- name: ListClassificationsByTrack :many
SELECT * FROM classifications
WHERE track_id = @track_id
ORDER BY created_at DESC
LIMIT @limit_count OFFSET @offset_count;

-- name: LatestClassificationForTrack :one
SELECT * FROM classifications
WHERE track_id = @track_id
ORDER BY created_at DESC
LIMIT 1;
