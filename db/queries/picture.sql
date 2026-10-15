-- Replay: reconstruct the operational picture as it was at a point in time.
--
-- The scrubber is the product's core advantage, and it is only trustworthy if
-- it reads the same evidence the live picture was built from. These queries
-- therefore reconstruct from track_history and asset_telemetry - the append
-- oriented records - never from the current-state projections, which have
-- already moved on.

-- name: TracksAt :many
-- Every track with its state as of a point in time. DISTINCT ON collapses the
-- history to the newest sample at or before the cut. Tracks with no history at
-- or before the cut are excluded: they had not been observed yet, so showing
-- them would invent a presence that never existed.
SELECT DISTINCT ON (t.id)
    t.id AS track_id,
    t.external_ref,
    t.status,
    t.first_seen_at,
    t.last_seen_at,
    h.observed_at,
    h.speed,
    h.heading,
    (h.position IS NOT NULL)::boolean AS has_position,
    COALESCE(ST_Y(h.position::geometry), 0)::float8 AS lat,
    COALESCE(ST_X(h.position::geometry), 0)::float8 AS lng
FROM tracks t
JOIN track_history h ON h.track_id = t.id AND h.observed_at <= @at
WHERE (@scope_prefix::text = '' OR t.id LIKE @scope_prefix::text || '%')
ORDER BY t.id, h.observed_at DESC;

-- name: AssetsAt :many
SELECT DISTINCT ON (a.id)
    a.id AS asset_id,
    a.name,
    a.type,
    a.status,
    t.observed_at,
    t.speed,
    t.heading,
    t.connection_state,
    t.health,
    (t.position IS NOT NULL)::boolean AS has_position,
    COALESCE(ST_Y(t.position::geometry), 0)::float8 AS lat,
    COALESCE(ST_X(t.position::geometry), 0)::float8 AS lng
FROM assets a
JOIN LATERAL (
    SELECT position, speed, heading, observed_at, connection_state, health
    FROM asset_telemetry
    WHERE asset_id = a.id AND observed_at <= @at
    ORDER BY observed_at DESC
    LIMIT 1
) t ON true
WHERE @scope_prefix::text = '' OR a.id LIKE @scope_prefix::text || '%'
ORDER BY a.id;

-- name: AlertsAt :many
SELECT
    id, type, severity, state, title, message, source_reference,
    track_id, asset_id, geofence_id, created_at
FROM alerts
WHERE created_at <= @at
ORDER BY created_at DESC
LIMIT @limit_count;

-- name: GeofencesAt :many
SELECT
    id, name, type, severity, active, ST_AsGeoJSON(geometry)::text AS geojson
FROM geofences
ORDER BY created_at
LIMIT @limit_count;

-- name: TrackHistoryWindow :one
-- The observed-time span a run covers. The scrubber cannot offer a time it
-- cannot reconstruct, so this bounds the control rather than letting it run off
-- the end of the evidence.
SELECT
    COALESCE(MIN(observed_at), now())::timestamptz AS earliest,
    COALESCE(MAX(observed_at), now())::timestamptz AS latest
FROM track_history
WHERE (@scope_prefix::text = '' OR track_id LIKE @scope_prefix::text || '%');