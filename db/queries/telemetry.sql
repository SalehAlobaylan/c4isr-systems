-- name: CreateTelemetry :one
INSERT INTO asset_telemetry (
    id, message_id, asset_id, source_id, observed_at, received_at,
    position, speed, heading, health, connection_state, payload
) VALUES (
    @id, @message_id, @asset_id, @source_id, @observed_at, @received_at,
    CASE WHEN @has_position::boolean
         THEN ST_SetSRID(ST_MakePoint(@lng::float8, @lat::float8), 4326)::geography
         ELSE NULL END,
    @speed, @heading, @health, @connection_state, @payload
)
ON CONFLICT (message_id) WHERE message_id IS NOT NULL DO NOTHING
RETURNING id;

-- name: ListTelemetryByAsset :many
SELECT
    id, message_id, asset_id, source_id, observed_at, received_at,
    speed, heading, health, connection_state, payload, created_at,
    (position IS NOT NULL)::boolean AS has_position,
    COALESCE(ST_Y(position::geometry), 0)::float8 AS lat,
    COALESCE(ST_X(position::geometry), 0)::float8 AS lng
FROM asset_telemetry
WHERE asset_id = @asset_id
ORDER BY observed_at DESC, id DESC
LIMIT @limit_count OFFSET @offset_count;

-- name: CountTelemetryByAsset :one
SELECT count(*) FROM asset_telemetry WHERE asset_id = @asset_id;

-- name: LatestTelemetryForAsset :one
SELECT
    id, message_id, asset_id, source_id, observed_at, received_at,
    speed, heading, health, connection_state, payload, created_at,
    (position IS NOT NULL)::boolean AS has_position,
    COALESCE(ST_Y(position::geometry), 0)::float8 AS lat,
    COALESCE(ST_X(position::geometry), 0)::float8 AS lng
FROM asset_telemetry
WHERE asset_id = @asset_id
ORDER BY observed_at DESC, id DESC
LIMIT 1;
