-- name: CreateObservation :one
-- The area columns are coalesced to an empty string rather than left NULL:
-- sqlc maps a nullable text column to a Go pointer, but an observation that is
-- not an area has no geometry to return, and scanning a NULL into a plain string
-- fails at runtime. The empty string is already the "no area" value.
INSERT INTO observations (
    id, source_id, observation_type, observed_at, received_at, processed_at,
    position, payload, quality, track_hint, area, area_kind
) VALUES (
    @id, @source_id, @observation_type, @observed_at, @received_at, @processed_at,
    CASE WHEN @has_position::boolean
         THEN ST_SetSRID(ST_MakePoint(@lng::float8, @lat::float8), 4326)::geography
         ELSE NULL END,
    @payload, @quality, @track_hint,
    CASE WHEN @has_area::boolean
         THEN ST_SetSRID(ST_GeomFromGeoJSON(@area_geojson::text), 4326)::geography
         ELSE NULL END,
    @area_kind
)
RETURNING
    id, source_id, observation_type, observed_at, received_at, processed_at,
    payload, quality, track_hint, created_at, area_kind,
    (position IS NOT NULL)::boolean AS has_position,
    COALESCE(ST_Y(position::geometry), 0)::float8 AS lat,
    COALESCE(ST_X(position::geometry), 0)::float8 AS lng,
    (area IS NOT NULL)::boolean AS has_area,
    CAST(COALESCE(ST_AsGeoJSON(area::geometry)::text, '') AS text) AS area_geojson;

-- name: GetObservation :one
SELECT
    id, source_id, observation_type, observed_at, received_at, processed_at,
    payload, quality, track_hint, created_at, area_kind,
    (position IS NOT NULL)::boolean AS has_position,
    COALESCE(ST_Y(position::geometry), 0)::float8 AS lat,
    COALESCE(ST_X(position::geometry), 0)::float8 AS lng,
    (area IS NOT NULL)::boolean AS has_area,
    CAST(COALESCE(ST_AsGeoJSON(area::geometry)::text, '') AS text) AS area_geojson
FROM observations
WHERE id = @id;

-- name: ListObservations :many
SELECT
    id, source_id, observation_type, observed_at, received_at, processed_at,
    payload, quality, track_hint, created_at, area_kind,
    (position IS NOT NULL)::boolean AS has_position,
    COALESCE(ST_Y(position::geometry), 0)::float8 AS lat,
    COALESCE(ST_X(position::geometry), 0)::float8 AS lng,
    (area IS NOT NULL)::boolean AS has_area,
    CAST(COALESCE(ST_AsGeoJSON(area::geometry)::text, '') AS text) AS area_geojson
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
    o.payload, o.quality, o.track_hint, o.created_at, o.area_kind,
    (o.position IS NOT NULL)::boolean AS has_position,
    COALESCE(ST_Y(o.position::geometry), 0)::float8 AS lat,
    COALESCE(ST_X(o.position::geometry), 0)::float8 AS lng,
    (o.area IS NOT NULL)::boolean AS has_area,
    CAST(COALESCE(ST_AsGeoJSON(o.area::geometry)::text, '') AS text) AS area_geojson
FROM observations o
JOIN track_observations t ON t.observation_id = o.id
WHERE t.track_id = @track_id
ORDER BY o.observed_at DESC, o.id DESC
LIMIT @limit_count OFFSET @offset_count;

-- name: MarkObservationProcessed :exec
UPDATE observations SET processed_at = now() WHERE id = @id;

-- name: GeofencesIntersectingArea :many
-- Which active zones a footprint overlaps.
--
-- ST_Intersects, not ST_Contains: a hazard zone may only partially overlap a
-- restricted geofence and that is still an operational fact. ST_Contains would
-- silently drop a wildfire burning across the boundary.
--
-- The overlap area is returned so a rule can distinguish a graze from a
-- headline event instead of treating both identically.
--
-- The footprint is parsed with ST_GeomFromGeoJSON rather than cast directly:
-- a text -> geography cast goes through the WKT parser, which rejects GeoJSON.
SELECT
    id, name, type, severity,
    ST_Area(ST_Intersection(
        geometry::geography,
        ST_GeomFromGeoJSON(@area_geojson::text)::geography
    ))::float8 AS overlap_m2
FROM geofences
WHERE active = true
  AND ST_Intersects(geometry, ST_GeomFromGeoJSON(@area_geojson::text))
ORDER BY overlap_m2 DESC;

-- name: ListAreaObservations :many
-- Recent area observations of one kind, newest first. Used by the operator's
-- situational-awareness view.
--
-- The column list matches the standard observation read shape so the repository
-- reuses one row mapper. The filter has to live here rather than in the
-- browser: point observations outnumber areas by orders of magnitude on a real
-- feed, so "newest N observations" would be almost entirely radar pings.
SELECT
    id, source_id, observation_type, observed_at, received_at, processed_at,
    payload, quality, track_hint, created_at, area_kind,
    (position IS NOT NULL)::boolean AS has_position,
    COALESCE(ST_Y(position::geometry), 0)::float8 AS lat,
    COALESCE(ST_X(position::geometry), 0)::float8 AS lng,
    (area IS NOT NULL)::boolean AS has_area,
    CAST(COALESCE(ST_AsGeoJSON(area::geometry)::text, '') AS text) AS area_geojson
FROM observations
WHERE area IS NOT NULL
  AND (@area_kind::text = '' OR area_kind = @area_kind::text)
ORDER BY observed_at DESC, id DESC
LIMIT @limit_count OFFSET @offset_count;

-- name: CountAreaObservations :one
SELECT count(*)
FROM observations
WHERE area IS NOT NULL
  AND (@area_kind::text = '' OR area_kind = @area_kind::text);