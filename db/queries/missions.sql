-- name: CreateMission :one
INSERT INTO missions (
    id, name, objective, priority, status, incident_id, created_at, updated_at
) VALUES (
    @id, @name, @objective, @priority, @status, @incident_id, now(), now()
)
RETURNING *;

-- name: GetMission :one
SELECT * FROM missions WHERE id = @id;

-- name: ListMissions :many
SELECT * FROM missions
WHERE (sqlc.narg('status')::text IS NULL OR status = sqlc.narg('status'))
ORDER BY created_at DESC
LIMIT @limit_count OFFSET @offset_count;

-- name: CountMissions :one
SELECT count(*) FROM missions
WHERE (sqlc.narg('status')::text IS NULL OR status = sqlc.narg('status'));

-- name: UpdateMissionStatus :one
UPDATE missions
SET status = @status,
    updated_at = now(),
    started_at = CASE WHEN @status::text = 'ACTIVE' AND started_at IS NULL THEN now() ELSE started_at END,
    ended_at = CASE WHEN @status::text IN ('COMPLETED', 'ABORTED') THEN now() ELSE ended_at END
WHERE id = @id
RETURNING *;

-- name: AssignAssetToMission :exec
INSERT INTO mission_assets (mission_id, asset_id)
VALUES (@mission_id, @asset_id)
ON CONFLICT DO NOTHING;

-- name: ListMissionAssets :many
SELECT
    a.id, a.name, a.type, a.status, a.capabilities, a.created_at, a.updated_at,
    s.health, s.connection_state, s.last_seen_at, s.speed AS state_speed, s.heading AS state_heading,
    (s.position IS NOT NULL)::boolean AS has_position,
    COALESCE(ST_Y(s.position::geometry), 0)::float8 AS lat,
    COALESCE(ST_X(s.position::geometry), 0)::float8 AS lng
FROM assets a
JOIN mission_assets ma ON ma.asset_id = a.id
LEFT JOIN asset_state s ON s.asset_id = a.id
WHERE ma.mission_id = @mission_id
ORDER BY a.name;

-- name: CreateMissionTask :one
INSERT INTO mission_tasks (id, mission_id, type, description, status, target_position)
VALUES (
    @id, @mission_id, @type, @description, @status,
    CASE WHEN @has_target::boolean
         THEN ST_SetSRID(ST_MakePoint(@lng::float8, @lat::float8), 4326)::geography
         ELSE NULL END
)
RETURNING id, mission_id, type, description, status, created_at, updated_at,
    (target_position IS NOT NULL)::boolean AS has_position,
    COALESCE(ST_Y(target_position::geometry), 0)::float8 AS lat,
    COALESCE(ST_X(target_position::geometry), 0)::float8 AS lng;

-- name: ListMissionTasks :many
SELECT
    id, mission_id, type, description, status, created_at, updated_at,
    (target_position IS NOT NULL)::boolean AS has_position,
    COALESCE(ST_Y(target_position::geometry), 0)::float8 AS lat,
    COALESCE(ST_X(target_position::geometry), 0)::float8 AS lng
FROM mission_tasks
WHERE mission_id = @mission_id
ORDER BY created_at ASC;

-- name: UpdateMissionTaskStatus :one
UPDATE mission_tasks
SET status = @status, updated_at = now()
WHERE id = @id
RETURNING id, mission_id, type, description, status, created_at, updated_at,
    (target_position IS NOT NULL)::boolean AS has_position,
    COALESCE(ST_Y(target_position::geometry), 0)::float8 AS lat,
    COALESCE(ST_X(target_position::geometry), 0)::float8 AS lng;

-- name: ListMissionsForAsset :many
SELECT m.* FROM missions m
JOIN mission_assets ma ON ma.mission_id = m.id
WHERE ma.asset_id = @asset_id
ORDER BY m.created_at DESC;
