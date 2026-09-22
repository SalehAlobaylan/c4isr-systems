-- name: CreateAsset :one
INSERT INTO assets (id, name, type, status, capabilities, metadata, created_at, updated_at)
VALUES (@id, @name, @type, @status, @capabilities, @metadata, now(), now())
RETURNING *;

-- name: GetAsset :one
SELECT * FROM assets WHERE id = @id;

-- name: ListAssets :many
SELECT * FROM assets
ORDER BY created_at DESC
LIMIT @limit_count OFFSET @offset_count;

-- name: CountAssets :one
SELECT count(*) FROM assets;

-- name: UpdateAssetStatus :one
UPDATE assets
SET status = @status, updated_at = now()
WHERE id = @id
RETURNING *;

-- name: AssetExists :one
SELECT EXISTS (SELECT 1 FROM assets WHERE id = @id);

-- name: EnsureAssetState :exec
INSERT INTO asset_state (asset_id, connection_state)
VALUES (@asset_id, 'unknown')
ON CONFLICT (asset_id) DO NOTHING;

-- name: GetAssetState :one
SELECT
    asset_id, speed, heading, health, connection_state, last_seen_at, updated_at,
    (position IS NOT NULL)::boolean AS has_position,
    COALESCE(ST_Y(position::geometry), 0)::float8 AS lat,
    COALESCE(ST_X(position::geometry), 0)::float8 AS lng
FROM asset_state
WHERE asset_id = @asset_id;

-- name: UpsertAssetState :one
WITH applied AS (
INSERT INTO asset_state (
    asset_id, position, speed, heading, health, connection_state, last_seen_at, updated_at
) VALUES (
    @asset_id,
    CASE WHEN @has_position::boolean
         THEN ST_SetSRID(ST_MakePoint(@lng::float8, @lat::float8), 4326)::geography
         ELSE NULL END,
    @speed, @heading, @health, @connection_state, @observed_at, now()
)
ON CONFLICT (asset_id) DO UPDATE SET
    position = CASE WHEN @has_position::boolean THEN EXCLUDED.position ELSE asset_state.position END,
    speed = COALESCE(EXCLUDED.speed, asset_state.speed),
    heading = COALESCE(EXCLUDED.heading, asset_state.heading),
    health = COALESCE(EXCLUDED.health, asset_state.health),
    connection_state = COALESCE(EXCLUDED.connection_state, asset_state.connection_state),
    last_seen_at = EXCLUDED.last_seen_at,
    updated_at = now()
WHERE (
    (
        @expected_last_seen_at::timestamptz IS NULL
        AND asset_state.last_seen_at IS NULL
    )
    OR asset_state.last_seen_at = @expected_last_seen_at::timestamptz
)
AND (
    asset_state.last_seen_at IS NULL
    OR asset_state.last_seen_at < EXCLUDED.last_seen_at
)
RETURNING asset_id
)
SELECT EXISTS (SELECT 1 FROM applied)::boolean AS applied;

-- name: ListStaleAssetIDs :many
SELECT a.id AS asset_id
FROM assets AS a
LEFT JOIN asset_state AS s ON s.asset_id = a.id
WHERE a.created_at < @cutoff
  AND (s.last_seen_at IS NULL OR s.last_seen_at < @cutoff);

-- name: ListAssetsWithState :many
SELECT
    a.id, a.name, a.type, a.status, a.capabilities, a.metadata, a.created_at, a.updated_at,
    s.health, s.connection_state, s.last_seen_at, s.speed AS state_speed, s.heading AS state_heading,
    (s.position IS NOT NULL)::boolean AS has_position,
    COALESCE(ST_Y(s.position::geometry), 0)::float8 AS lat,
    COALESCE(ST_X(s.position::geometry), 0)::float8 AS lng
FROM assets a
LEFT JOIN asset_state s ON s.asset_id = a.id
ORDER BY a.name
LIMIT @limit_count OFFSET @offset_count;

-- name: GetAssetDetail :one
SELECT
    a.id, a.name, a.type, a.status, a.capabilities, a.metadata, a.created_at, a.updated_at,
    s.health, s.connection_state, s.last_seen_at, s.speed AS state_speed, s.heading AS state_heading,
    (s.position IS NOT NULL)::boolean AS has_position,
    COALESCE(ST_Y(s.position::geometry), 0)::float8 AS lat,
    COALESCE(ST_X(s.position::geometry), 0)::float8 AS lng
FROM assets a
LEFT JOIN asset_state s ON s.asset_id = a.id
WHERE a.id = @id;
