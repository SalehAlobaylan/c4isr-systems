-- +goose Up
-- Area observations.
--
-- Not every observation describes a moving object. A wildfire, an earthquake,
-- or a weather cell is an *area* at a moment in time: it has extent, it does not
-- have a track, and forcing it through the track pipeline produces either a
-- nonsense track or an alert that never fires because the correlation rules
-- were written for points.
--
-- Rather than introduce a second top-level entity alongside tracks - which would
-- split provenance in two and undercut the Source/Observation spine - an
-- observation may carry an area. Area observations are still observations:
-- same table, same sources, same timestamps, same provenance. What changes is
-- that they raise alerts directly instead of being correlated into a track.
--
-- The area is stored as geography so containment and intersection are decided by
-- PostGIS, consistent with every other spatial decision in the platform.
ALTER TABLE observations
    ADD COLUMN area geography (Polygon, 4326),
    ADD COLUMN area_kind text;

-- kind names the phenomenon class, so alert rules can select the areas they
-- care about without parsing payload keys. Constrained to keep it a small,
-- meaningful vocabulary rather than a free-form string that drifts.
ALTER TABLE observations
    ADD CONSTRAINT observations_area_kind_check CHECK (
        area_kind IS NULL OR area_kind IN ('hazard', 'weather', 'environmental', 'infrastructure', 'crowd')
    );

-- An area observation must carry a kind: an area with no declared class cannot
-- be routed to a rule, so accepting one would store evidence that can never
-- raise anything. Point-only observations keep a null area and null kind.
ALTER TABLE observations
    ADD CONSTRAINT observations_area_requires_kind CHECK (
        (area IS NULL AND area_kind IS NULL) OR (area IS NOT NULL AND area_kind IS NOT NULL)
    );

CREATE INDEX observations_area_idx ON observations USING gist (area);

-- The alert router scans recent area observations of one kind. The partial
-- index keeps that scan off the (much larger) point-observation set.
CREATE INDEX observations_area_kind_time_idx
    ON observations (area_kind, observed_at DESC)
    WHERE area IS NOT NULL;

-- The concurrency guard for area overlap alerts. Two handlers evaluating the
-- same footprint at once must not both insert an unresolved alert, so this is
-- enforced in the database rather than relying on the service's read-then-write
-- check, which has a window between the two statements.
CREATE UNIQUE INDEX alerts_active_area_overlap_key
    ON alerts (geofence_id, (source_reference->>'observationId'))
    WHERE type = 'geofence.area.overlap'
      AND state <> 'RESOLVED'
      AND geofence_id IS NOT NULL
      AND source_reference->>'observationId' IS NOT NULL;

-- +goose Down
DROP INDEX alerts_active_area_overlap_key;
DROP INDEX observations_area_kind_time_idx;
DROP INDEX observations_area_idx;
ALTER TABLE observations DROP CONSTRAINT observations_area_requires_kind;
ALTER TABLE observations DROP CONSTRAINT observations_area_kind_check;
ALTER TABLE observations DROP COLUMN area_kind;
ALTER TABLE observations DROP COLUMN area;