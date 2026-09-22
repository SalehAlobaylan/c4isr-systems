-- name: CreateGeofence :one
INSERT INTO geofences (id, name, type, geometry, severity, active, metadata, created_at, updated_at)
VALUES (
    @id, @name, @type,
    ST_SetSRID(ST_GeomFromGeoJSON(@geojson::text), 4326),
    @severity, @active, @metadata, now(), now()
)
RETURNING
    id, name, type, severity, active, metadata, created_at, updated_at,
    ST_AsGeoJSON(geometry)::text AS geojson;

-- name: GetGeofence :one
SELECT
    id, name, type, severity, active, metadata, created_at, updated_at,
    ST_AsGeoJSON(geometry)::text AS geojson
FROM geofences
WHERE id = @id;

-- name: ListGeofences :many
SELECT
    id, name, type, severity, active, metadata, created_at, updated_at,
    ST_AsGeoJSON(geometry)::text AS geojson
FROM geofences
ORDER BY created_at DESC
LIMIT @limit_count OFFSET @offset_count;

-- name: CountGeofences :one
SELECT count(*) FROM geofences;

-- name: ListActiveGeofencesContainingPoint :many
SELECT
    id, name, type, severity, active, metadata, created_at, updated_at,
    ST_AsGeoJSON(geometry)::text AS geojson
FROM geofences
WHERE active = true
  AND ST_Contains(geometry, ST_SetSRID(ST_MakePoint(@lng::float8, @lat::float8), 4326))
ORDER BY created_at DESC;

-- name: UpdateGeofenceActive :one
UPDATE geofences
SET active = @active, updated_at = now()
WHERE id = @id
RETURNING id, name, type, severity, active, metadata, created_at, updated_at,
    ST_AsGeoJSON(geometry)::text AS geojson;

-- name: GetGeofenceState :one
SELECT geofence_id, track_id, inside, since, updated_at
FROM geofence_states
WHERE geofence_id = @geofence_id AND track_id = @track_id;

-- name: UpsertGeofenceState :exec
INSERT INTO geofence_states (geofence_id, track_id, inside, since, updated_at)
VALUES (@geofence_id, @track_id, @inside, now(), now())
ON CONFLICT (geofence_id, track_id) DO UPDATE SET
    inside = EXCLUDED.inside,
    since = CASE WHEN geofence_states.inside IS DISTINCT FROM EXCLUDED.inside
                 THEN now() ELSE geofence_states.since END,
    updated_at = now();

-- name: ListGeofenceStatesByTrack :many
SELECT geofence_id, track_id, inside, since, updated_at
FROM geofence_states
WHERE track_id = @track_id;

-- name: ReconcileGeofenceStates :many
WITH locked AS (
    SELECT pg_advisory_xact_lock(hashtext(@track_id::text)) AS acquired
), eligible AS (
    SELECT 1
    FROM tracks t
    CROSS JOIN locked
    WHERE t.id = @track_id
      AND @observed_at::timestamptz >= t.last_seen_at
), existing AS (
    SELECT gs.geofence_id, gs.inside
    FROM geofence_states gs
    CROSS JOIN locked
    CROSS JOIN eligible
    WHERE gs.track_id = @track_id
      AND (@scope_prefix::text = '' OR gs.geofence_id LIKE @scope_prefix::text || '%')
    FOR UPDATE
), containing AS (
    SELECT DISTINCT id
    FROM unnest(@containing_ids::text[]) AS ids(id)
), entered AS (
    INSERT INTO geofence_states (geofence_id, track_id, inside, since, updated_at)
    SELECT c.id, @track_id, true, now(), now()
    FROM containing c
    CROSS JOIN locked
    CROSS JOIN eligible
    ON CONFLICT (geofence_id, track_id) DO UPDATE SET
        inside = EXCLUDED.inside,
        since = now(),
        updated_at = now()
    WHERE geofence_states.inside IS DISTINCT FROM EXCLUDED.inside
    RETURNING geofence_id, inside
), exited AS (
    UPDATE geofence_states gs
    SET inside = false,
        since = now(),
        updated_at = now()
    FROM existing e
    WHERE gs.geofence_id = e.geofence_id
      AND gs.track_id = @track_id
      AND e.inside
      AND NOT EXISTS (SELECT 1 FROM containing c WHERE c.id = e.geofence_id)
    RETURNING gs.geofence_id, gs.inside
)
SELECT geofence_id, inside FROM entered
UNION ALL
SELECT geofence_id, inside FROM exited
ORDER BY geofence_id;

-- name: DeleteGeofenceStateForTrack :exec
DELETE FROM geofence_states WHERE track_id = @track_id;
