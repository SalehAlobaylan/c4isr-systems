-- name: ListAssetsWithinRadius :many
SELECT
    a.id, a.name, a.type, a.status,
    s.connection_state, s.last_seen_at,
    (s.position IS NOT NULL)::boolean AS has_position,
    COALESCE(ST_Y(s.position::geometry), 0)::float8 AS lat,
    COALESCE(ST_X(s.position::geometry), 0)::float8 AS lng,
    ST_Distance(
        s.position,
        ST_SetSRID(ST_MakePoint(@lng::float8, @lat::float8), 4326)::geography
    )::float8 AS distance_m
FROM assets a
JOIN asset_state s ON s.asset_id = a.id
WHERE s.position IS NOT NULL
  AND ST_DWithin(
        s.position,
        ST_SetSRID(ST_MakePoint(@lng::float8, @lat::float8), 4326)::geography,
        @radius_m::float8
      )
ORDER BY distance_m ASC
LIMIT @limit_count;

-- name: NearestAssets :many
SELECT
    a.id, a.name, a.type, a.status,
    s.connection_state, s.last_seen_at,
    (s.position IS NOT NULL)::boolean AS has_position,
    COALESCE(ST_Y(s.position::geometry), 0)::float8 AS lat,
    COALESCE(ST_X(s.position::geometry), 0)::float8 AS lng,
    ST_Distance(
        s.position,
        ST_SetSRID(ST_MakePoint(@lng::float8, @lat::float8), 4326)::geography
    )::float8 AS distance_m
FROM assets a
JOIN asset_state s ON s.asset_id = a.id
WHERE s.position IS NOT NULL
ORDER BY distance_m ASC
LIMIT @limit_count;
