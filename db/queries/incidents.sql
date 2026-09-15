-- name: CreateIncident :one
INSERT INTO incidents (
    id, title, description, priority, status, assigned_operator, created_at, updated_at
) VALUES (
    @id, @title, @description, @priority, @status, @assigned_operator, now(), now()
)
RETURNING *;

-- name: GetIncident :one
SELECT * FROM incidents WHERE id = @id;

-- name: ListIncidents :many
SELECT * FROM incidents
WHERE (sqlc.narg('status')::text IS NULL OR status = sqlc.narg('status'))
ORDER BY created_at DESC
LIMIT @limit_count OFFSET @offset_count;

-- name: CountIncidents :one
SELECT count(*) FROM incidents
WHERE (sqlc.narg('status')::text IS NULL OR status = sqlc.narg('status'));

-- name: UpdateIncidentStatus :one
UPDATE incidents
SET status = @status,
    updated_at = now(),
    resolved_at = CASE WHEN @status::text = 'RESOLVED' THEN now() ELSE resolved_at END,
    closed_at = CASE WHEN @status::text = 'CLOSED' THEN now() ELSE closed_at END
WHERE id = @id
RETURNING *;

-- name: UpdateIncident :one
UPDATE incidents
SET title = @title, description = @description, priority = @priority,
    assigned_operator = @assigned_operator, updated_at = now()
WHERE id = @id
RETURNING *;

-- name: AddIncidentAlert :exec
INSERT INTO incident_alerts (incident_id, alert_id)
VALUES (@incident_id, @alert_id)
ON CONFLICT DO NOTHING;

-- name: AddIncidentTrack :exec
INSERT INTO incident_tracks (incident_id, track_id)
VALUES (@incident_id, @track_id)
ON CONFLICT DO NOTHING;

-- name: AddIncidentAsset :exec
INSERT INTO incident_assets (incident_id, asset_id)
VALUES (@incident_id, @asset_id)
ON CONFLICT DO NOTHING;

-- name: AddIncidentObservation :exec
INSERT INTO incident_observations (incident_id, observation_id)
VALUES (@incident_id, @observation_id)
ON CONFLICT DO NOTHING;

-- name: AddIncidentAssessment :exec
INSERT INTO incident_assessments (incident_id, assessment_id)
VALUES (@incident_id, @assessment_id)
ON CONFLICT DO NOTHING;

-- name: ListIncidentAlerts :many
SELECT a.* FROM alerts a
JOIN incident_alerts ia ON ia.alert_id = a.id
WHERE ia.incident_id = @incident_id
ORDER BY a.created_at DESC;

-- name: ListIncidentTracks :many
SELECT
    t.id, t.external_ref, t.status, t.first_seen_at, t.last_seen_at,
    t.metadata, t.created_at, t.updated_at, t.closed_at,
    s.speed, s.heading, s.updated_at AS state_updated_at,
    (s.position IS NOT NULL)::boolean AS has_position,
    COALESCE(ST_Y(s.position::geometry), 0)::float8 AS lat,
    COALESCE(ST_X(s.position::geometry), 0)::float8 AS lng
FROM tracks t
JOIN incident_tracks it ON it.track_id = t.id
LEFT JOIN track_state s ON s.track_id = t.id
WHERE it.incident_id = @incident_id
ORDER BY t.last_seen_at DESC;

-- name: ListIncidentAssets :many
SELECT
    a.id, a.name, a.type, a.status, a.capabilities, a.created_at, a.updated_at,
    s.health, s.connection_state, s.last_seen_at, s.speed AS state_speed, s.heading AS state_heading,
    (s.position IS NOT NULL)::boolean AS has_position,
    COALESCE(ST_Y(s.position::geometry), 0)::float8 AS lat,
    COALESCE(ST_X(s.position::geometry), 0)::float8 AS lng
FROM assets a
JOIN incident_assets ia ON ia.asset_id = a.id
LEFT JOIN asset_state s ON s.asset_id = a.id
WHERE ia.incident_id = @incident_id
ORDER BY a.name;

-- name: ListIncidentObservations :many
SELECT
    o.id, o.source_id, o.observation_type, o.observed_at, o.received_at, o.processed_at,
    o.payload, o.quality, o.track_hint, o.created_at,
    (o.position IS NOT NULL)::boolean AS has_position,
    COALESCE(ST_Y(o.position::geometry), 0)::float8 AS lat,
    COALESCE(ST_X(o.position::geometry), 0)::float8 AS lng
FROM observations o
JOIN incident_observations io ON io.observation_id = o.id
WHERE io.incident_id = @incident_id
ORDER BY o.observed_at DESC
LIMIT @limit_count;

-- name: ListIncidentAssessments :many
SELECT a.id, a.subject_type, a.subject_id, a.type, a.conclusion, a.confidence, a.method, a.created_by, a.created_at
FROM assessments a
JOIN incident_assessments ia ON ia.assessment_id = a.id
WHERE ia.incident_id = @incident_id
ORDER BY a.created_at DESC;
