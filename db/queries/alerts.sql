-- name: CreateAlert :one
INSERT INTO alerts (
    id, type, severity, state, title, message, source_reference,
    track_id, asset_id, geofence_id, incident_id, created_at, updated_at
) VALUES (
    @id, @type, @severity, 'ACTIVE', @title, @message, @source_reference,
    @track_id, @asset_id, @geofence_id, @incident_id, now(), now()
)
RETURNING *;

-- name: GetAlert :one
SELECT * FROM alerts WHERE id = @id;

-- name: ListAlerts :many
SELECT * FROM alerts
WHERE (sqlc.narg('state')::text IS NULL OR state = sqlc.narg('state'))
  AND (sqlc.narg('severity')::text IS NULL OR severity = sqlc.narg('severity'))
  AND (sqlc.narg('track_id')::text IS NULL OR track_id = sqlc.narg('track_id'))
  AND (sqlc.narg('incident_id')::text IS NULL OR incident_id = sqlc.narg('incident_id'))
ORDER BY created_at DESC
LIMIT @limit_count OFFSET @offset_count;

-- name: CountAlerts :one
SELECT count(*) FROM alerts
WHERE (sqlc.narg('state')::text IS NULL OR state = sqlc.narg('state'))
  AND (sqlc.narg('severity')::text IS NULL OR severity = sqlc.narg('severity'))
  AND (sqlc.narg('track_id')::text IS NULL OR track_id = sqlc.narg('track_id'))
  AND (sqlc.narg('incident_id')::text IS NULL OR incident_id = sqlc.narg('incident_id'));

-- name: CountActiveAlerts :one
SELECT count(*) FROM alerts WHERE state = 'ACTIVE';

-- name: AcknowledgeAlert :one
UPDATE alerts
SET state = 'ACKNOWLEDGED', acknowledged_at = now(), acknowledged_by = @acknowledged_by, updated_at = now()
WHERE id = @id AND state = 'ACTIVE'
RETURNING *;

-- name: ResolveAlert :one
UPDATE alerts
SET state = 'RESOLVED', resolved_at = now(), resolved_by = @resolved_by, updated_at = now()
WHERE id = @id AND state IN ('ACTIVE', 'ACKNOWLEDGED')
RETURNING *;

-- name: SetAlertIncident :exec
UPDATE alerts SET incident_id = @incident_id, updated_at = now() WHERE id = @id;

-- name: FindUnresolvedAlertForGeofenceTrack :one
SELECT * FROM alerts
WHERE geofence_id = @geofence_id AND track_id = @track_id AND state <> 'RESOLVED'
ORDER BY created_at DESC
LIMIT 1;

-- name: FindUnresolvedAlertForObservationGeofence :one
-- Area alerts have no track: their evidence is the observation that reported
-- the footprint. The observation id lives in source_reference, which is where
-- provenance for this alert type is recorded. The ::text casts are explicit so
-- the comparison is a plain string match rather than a jsonb equality, which
-- would need the parameter typed as jsonb and would not use the index.
SELECT * FROM alerts
WHERE type = 'geofence.area.overlap'
  AND geofence_id = @geofence_id
  AND source_reference->>'observationId'::text = @observation_id::text
  AND state <> 'RESOLVED'
ORDER BY created_at DESC
LIMIT 1;

-- name: ListAlertsByIncident :many
SELECT * FROM alerts WHERE incident_id = @incident_id ORDER BY created_at DESC;
