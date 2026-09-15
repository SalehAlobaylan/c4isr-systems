-- name: CreateSource :one
INSERT INTO sources (id, name, type, status, metadata, created_at, updated_at)
VALUES (@id, @name, @type, @status, @metadata, now(), now())
RETURNING *;

-- name: GetSource :one
SELECT * FROM sources WHERE id = @id;

-- name: ListSources :many
SELECT * FROM sources
ORDER BY created_at DESC
LIMIT @limit_count OFFSET @offset_count;

-- name: CountSources :one
SELECT count(*) FROM sources;

-- name: SourceExists :one
SELECT EXISTS (SELECT 1 FROM sources WHERE id = @id);

-- name: UpdateSourceStatus :one
UPDATE sources
SET status = @status, updated_at = now()
WHERE id = @id
RETURNING *;
