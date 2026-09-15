-- name: CreateObservation :one
INSERT INTO observations (
    id, source_id, observation_type, observed_at, received_at, processed_at,
    position, payload, quality, track_hint
) VALUES (
    @id, @source_id, @observation_type, @observed_at, @received_at, @processed_at,
    CASE WHEN @has_position::boolean
         THEN ST_SetSRID(ST_MakePoint(@lng::float8, @lat::float8), 4326)::geography
         ELSE NULL END,
    @payload, @quality, @track_hint
)
RETURNING
    id, source_id, observation_type, observed_at, received_at, processed_at,
    payload, quality, track_hint, created_at,
    (position IS NOT NULL)::boolean AS has_position,
    COALESCE(ST_Y(position::geometry), 0)::float8 AS lat,
    COALESCE(ST_X(position::geometry), 0)::float8 AS lng;

-- name: GetObservation :one
SELECT
    id, source_id, observation_type, observed_at, received_at, processed_at,
    payload, quality, track_hint, created_at,
    (position IS NOT NULL)::boolean AS has_position,
    COALESCE(ST_Y(position::geometry), 0)::float8 AS lat,
    COALESCE(ST_X(position::geometry), 0)::float8 AS lng
FROM observations
WHERE id = @id;

-- name: ListObservations :many
SELECT
    id, source_id, observation_type, observed_at, received_at, processed_at,
    payload, quality, track_hint, created_at,
    (position IS NOT NULL)::boolean AS has_position,
    COALESCE(ST_Y(position::geometry), 0)::float8 AS lat,
    COALESCE(ST_X(position::geometry), 0)::float8 AS lng
FROM observations
ORDER BY observed_at DESC, id DESC
LIMIT @limit_count OFFSET @offset_count;

-- name: CountObservations :one
SELECT count(*) FROM observations;

-- name: CountObservationsByTrack :one
SELECT count(*)
FROM track_observations
WHERE track_id = @track_id;

-- name: ListObservationsByTrack :many
SELECT
    o.id, o.source_id, o.observation_type, o.observed_at, o.received_at, o.processed_at,
    o.payload, o.quality, o.track_hint, o.created_at,
    (o.position IS NOT NULL)::boolean AS has_position,
    COALESCE(ST_Y(o.position::geometry), 0)::float8 AS lat,
    COALESCE(ST_X(o.position::geometry), 0)::float8 AS lng
FROM observations o
JOIN track_observations t ON t.observation_id = o.id
WHERE t.track_id = @track_id
ORDER BY o.observed_at DESC, o.id DESC
LIMIT @limit_count OFFSET @offset_count;

-- name: MarkObservationProcessed :exec
UPDATE observations SET processed_at = now() WHERE id = @id;
