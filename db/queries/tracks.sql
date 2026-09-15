-- name: CreateTrack :one
INSERT INTO tracks (id, external_ref, status, first_seen_at, last_seen_at, metadata, created_at, updated_at)
VALUES (@id, @external_ref, @status, @first_seen_at, @last_seen_at, @metadata, now(), now())
RETURNING *;

-- name: GetTrackDetail :one
SELECT
    t.id, t.external_ref, t.status, t.first_seen_at, t.last_seen_at,
    t.metadata, t.created_at, t.updated_at, t.closed_at,
    s.speed, s.heading, s.updated_at AS state_updated_at,
    (s.position IS NOT NULL)::boolean AS has_position,
    COALESCE(ST_Y(s.position::geometry), 0)::float8 AS lat,
    COALESCE(ST_X(s.position::geometry), 0)::float8 AS lng
FROM tracks t
LEFT JOIN track_state s ON s.track_id = t.id
WHERE t.id = @id;

-- name: ListTrackDetails :many
SELECT
    t.id, t.external_ref, t.status, t.first_seen_at, t.last_seen_at,
    t.metadata, t.created_at, t.updated_at, t.closed_at,
    s.speed, s.heading, s.updated_at AS state_updated_at,
    (s.position IS NOT NULL)::boolean AS has_position,
    COALESCE(ST_Y(s.position::geometry), 0)::float8 AS lat,
    COALESCE(ST_X(s.position::geometry), 0)::float8 AS lng
FROM tracks t
LEFT JOIN track_state s ON s.track_id = t.id
ORDER BY t.last_seen_at DESC
LIMIT @limit_count OFFSET @offset_count;

-- name: CountTracks :one
SELECT count(*) FROM tracks;

-- name: FindTrackByExternalRef :one
SELECT
    t.id, t.external_ref, t.status, t.first_seen_at, t.last_seen_at,
    t.metadata, t.created_at, t.updated_at, t.closed_at,
    s.speed, s.heading, s.updated_at AS state_updated_at,
    (s.position IS NOT NULL)::boolean AS has_position,
    COALESCE(ST_Y(s.position::geometry), 0)::float8 AS lat,
    COALESCE(ST_X(s.position::geometry), 0)::float8 AS lng
FROM tracks t
LEFT JOIN track_state s ON s.track_id = t.id
WHERE t.external_ref = @external_ref;

-- name: TouchTrack :exec
UPDATE tracks
SET last_seen_at = GREATEST(last_seen_at, @observed_at), updated_at = now()
WHERE id = @id;

-- name: CloseTrack :one
UPDATE tracks
SET status = 'closed', closed_at = now(), updated_at = now()
WHERE id = @id
RETURNING *;

-- name: UpsertTrackState :exec
INSERT INTO track_state (track_id, position, speed, heading, updated_at)
VALUES (
    @track_id,
    CASE WHEN @has_position::boolean
         THEN ST_SetSRID(ST_MakePoint(@lng::float8, @lat::float8), 4326)::geography
         ELSE NULL END,
    @speed, @heading, now()
)
ON CONFLICT (track_id) DO UPDATE SET
    position = CASE WHEN @has_position::boolean THEN EXCLUDED.position ELSE track_state.position END,
    speed = COALESCE(EXCLUDED.speed, track_state.speed),
    heading = COALESCE(EXCLUDED.heading, track_state.heading),
    updated_at = now();

-- name: AttachObservationToTrack :execrows
INSERT INTO track_observations (track_id, observation_id, source_id, observed_at, position)
SELECT @track_id, o.id, o.source_id, o.observed_at, o.position
FROM observations o
WHERE o.id = @observation_id
ON CONFLICT (observation_id) DO NOTHING;

-- name: CreateTrackHistoryEntry :exec
INSERT INTO track_history (id, track_id, observed_at, position, speed, heading)
VALUES (
    @id, @track_id, @observed_at,
    CASE WHEN @has_position::boolean
         THEN ST_SetSRID(ST_MakePoint(@lng::float8, @lat::float8), 4326)::geography
         ELSE NULL END,
    @speed, @heading
);

-- name: ListTrackHistory :many
SELECT
    id, observed_at, speed, heading,
    (position IS NOT NULL)::boolean AS has_position,
    COALESCE(ST_Y(position::geometry), 0)::float8 AS lat,
    COALESCE(ST_X(position::geometry), 0)::float8 AS lng
FROM track_history
WHERE track_id = @track_id
ORDER BY observed_at ASC, id ASC
LIMIT @limit_count;

-- name: CountTrackObservations :one
SELECT count(*) FROM track_observations WHERE track_id = @track_id;

-- name: CountObservationsBySourceForTrack :many
SELECT source_id, count(*) AS observation_count
FROM track_observations
WHERE track_id = @track_id
GROUP BY source_id
ORDER BY observation_count DESC;
