-- +goose Up

-- A retry may arrive while the original observation is still being projected.
-- The durable owner key makes the correlation decision idempotent even when
-- those requests race.
-- Existing installations may contain rows written before this guard existed.
-- Keep the newest durable owner and remove the key from older copies so the
-- upgrade is deterministic instead of failing while building the index.
WITH duplicate_tracks AS (
    SELECT id,
           ROW_NUMBER() OVER (
               PARTITION BY metadata->>'initialObservationId'
               ORDER BY created_at DESC, id DESC
           ) AS row_number
    FROM tracks
    WHERE metadata ? 'initialObservationId'
)
UPDATE tracks AS t
SET metadata = t.metadata - 'initialObservationId',
    updated_at = now()
FROM duplicate_tracks AS d
WHERE t.id = d.id
  AND d.row_number > 1;

CREATE UNIQUE INDEX tracks_initial_observation_id_key
    ON tracks ((metadata->>'initialObservationId'))
    WHERE metadata ? 'initialObservationId';

-- Geofence breach alerts are unique while unresolved. Resolving an alert
-- intentionally permits a later re-entry to raise a new alert.
-- Older duplicates are retained as resolved history; the newest alert remains
-- the active representative for each geofence/track pair.
WITH duplicate_alerts AS (
    SELECT id,
           ROW_NUMBER() OVER (
               PARTITION BY geofence_id, track_id
               ORDER BY created_at DESC, id DESC
           ) AS row_number
    FROM alerts
    WHERE type = 'geofence.breach'
      AND state <> 'RESOLVED'
      AND geofence_id IS NOT NULL
      AND track_id IS NOT NULL
)
UPDATE alerts AS a
SET state = 'RESOLVED',
    resolved_at = COALESCE(a.resolved_at, now()),
    resolved_by = COALESCE(a.resolved_by, 'migration:00017'),
    updated_at = now()
FROM duplicate_alerts AS d
WHERE a.id = d.id
  AND d.row_number > 1;

CREATE UNIQUE INDEX alerts_active_geofence_track_key
    ON alerts (geofence_id, track_id)
    WHERE type = 'geofence.breach' AND state <> 'RESOLVED';

-- +goose Down
DROP INDEX alerts_active_geofence_track_key;
DROP INDEX tracks_initial_observation_id_key;
