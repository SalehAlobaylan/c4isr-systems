# Implementation Decisions

Material decisions made while implementing the C2 + Operational Awareness milestone. Each entry records the choice, the reason, and known limits. Architectural intent lives in `ARCHITECTURE.md`; this file records how that intent was realized.

## D1. Modular monolith with a synchronous in-process dispatcher

**Decision:** Modules integrate through `internal/events`, a synchronous dispatcher that invokes subscribers in registration order during the publishing call stack.

**Why:** The architecture calls for in-process events (Section 22) and deterministic behavior. Synchronous delivery means a track update is durable before geofence evaluation runs, and geofence state is durable before an alert is created. Tests can assert ordering without sleeps or brokers. No broker is introduced before a concrete requirement exists.

**Limits:** Subscriber failure does not roll back the publisher's transaction; handlers log and continue. Cross-module transactions are therefore eventual at module boundaries, which is acceptable for the current scope. When durability across process restarts becomes required, the dispatcher is the single place to introduce an outbox.

## D2. PostGIS column types: geography for points, geometry for polygons

**Decision:** Positions (observations, track state, asset state, telemetry) are `geography(Point, 4326)`. Geofences are `geometry(Geometry, 4326)`. Queries convert with `position::geometry` for `ST_Contains` and use geography for `ST_DWithin`/`ST_Distance` (meters).

**Why:** Geography points give accurate meter-based proximity; `ST_Contains` operates on geometry and is used against polygon geofences. Keeping authoritative operations in SQL satisfies the GIS rules in the architecture.

**Limits:** The `geography` type is deliberately never selected raw through sqlc; repositories read `ST_X/ST_Y` and expose `geo.Point` in the domain.

## D3. Nullable result columns are made explicit in SQL

**Decision:** Computed position columns are selected as `(position IS NOT NULL)::boolean` plus `COALESCE(ST_Y(...), 0)::float8` / `COALESCE(ST_X(...), 0)::float8`, and repositories rebuild an optional point.

**Why:** sqlc cannot infer nullability through PostGIS function calls on a type it does not model; this keeps generated Go types concrete and avoids `interface{}` scans while remaining fully SQL-side.

## D4. Filters use `sqlc.narg` rather than empty-string sentinels

**Decision:** Optional list filters are declared with `sqlc.narg('name')` so generated parameters are `*string` and empty filters become SQL NULL.

**Why:** `@name::text IS NULL OR ...` typed as `string` forced an adapter layer to translate `""` to NULL. `narg` removes that class of bug and deletes the workaround entirely.

## D5. Track correlation v1: explicit hints, no fusion

**Decision:** An observation carries an optional `trackHint` (scenario-supplied). The track service resolves it against `tracks.external_ref`, creating the track on first sight. Observations without a hint create their own track. Evidence attaches idempotently through `track_observations`; stale observations attach as evidence but never regress current state.

**Why:** The implementation plan explicitly defers sensor fusion. The model preserves the observation-to-track relation so future correlation can replace the strategy without changing the information model.

**Limits:** No automatic association of unhinted observations; no merge/split of tracks.

## D6. Time model: scenario inputs use wall-clock `observed_at`

**Decision:** Scenario-emitted observations use `time.Now()` as `observed_at` (minus `stale_for` when requested). Scenario virtual time drives scheduling, pause/resume, and progress, not timestamps.

**Why:** Under a playback speed multiplier, virtual time runs ahead of wall time; stamping `observed_at` from virtual time would create future-dated observations that the ingestion validation correctly rejects. Wall-clock timestamps keep data realistic while replay still reproduces the same observation sequence and geometry because scheduling, ordering, movement, and jitter are virtual-time/seeded.

**Replay guarantee:** With the same seed, the sequence of observation positions is bit-identical between runs; this is asserted by the acceptance test. Absolute timestamps differ between replays by design.

## D7. Realtime contract is an explicit mapping

**Decision:** `internal/realtime/mapper.go` maps each event type to a wire payload map. Internal event structs are not marshaled directly.

**Why:** Required by the architecture: WebSocket payloads are a versioned browser contract (`{id, type, version, occurredAt, data}`). The explicit switch makes any event change a compile-time decision instead of a silent wire change.

## D8. Hub disconnects slow clients

**Decision:** Each WebSocket client has a bounded send buffer (256 messages). If it fills, the client is closed rather than blocking event processing.

**Why:** Event publication happens inside request/ingestion paths; backpressure from one slow operator must never stall ingestion. Clients reconnect and re-fetch authoritative state via REST.

## D9. Operator identity via header until Phase 17

**Decision:** `X-Operator-ID` (default `operator-01`) identifies the actor; it flows into events with actor fields and into the audit log. A default operator row is ensured at startup.

**Why:** RBAC is Phase 17. Attribute-before-authenticate keeps operator actions auditable now and leaves one seam (the middleware and `Append`) to enforce later.

## D10. Audit is event-derived plus explicit attribution

**Decision:** The audit service subscribes to every domain topic and writes an entry per event, deriving `OPERATOR` attribution from events that carry an operator/actor field and `SCENARIO` for scenario run events. Services do not call the audit service directly.

**Why:** One subscription point keeps audit coverage total and prevents services from forgetting to record actions. The event actor fields were added exactly for this purpose.

## D11. Command lifecycle: issued means sent; transport is simulated

**Decision:** `Issue` creates a command in `SENT` (with `sent_at`) because there is no external transport yet. Transitions are validated by an explicit state machine; the scenario runner subscribes to `command.issued` for assets it owns and schedules acknowledgements according to `command_simulation` (`normal`, `reject`, `fail`, `timeout`).

**Why:** Models control before hardware exists, keeps commands auditable, and exercises the failure modes the plan requires.

## D12. Scenario runner registers entities through application services

**Decision:** Scenario setup calls `sources.Ensure`, `assets.Ensure`, and `geospatial.Ensure`; emissions call `observations.Ingest` and `telemetry.Ingest`. The runner never writes to the database.

**Why:** Required by the implementation plan (Section 16). It turns every scenario run into an integration test of the real ingestion path and makes replacing the runner with a real simulator a wiring change, not a domain change.

## D13. Multi-arch PostGIS image

**Decision:** `deployments/docker-compose.yml` and Testcontainers use `imresamu/postgis:17-3.5`, a community multi-arch build of the official image.

**Why:** `postgis/postgis` does not publish a `linux/arm64` manifest, so Apple Silicon developers and CI runners cannot pull it. The image is built from the official Dockerfile and provides the same PostGIS 3.5 / PostgreSQL 17 stack.

## D14. Integration tests run real PostGIS and are opt-in

**Decision:** `internal/integration` starts a PostGIS container via Testcontainers, applies migrations, assembles the real `app.App`, and exercises the HTTP surface. Tests skip unless `C4ISR_TEST_INTEGRATION=1`.

**Why:** Spatial behavior, transactions, and migrations must not be mocked. Gating keeps `go test ./...` fast and Docker-free for day-to-day work.

## D15. Assessment evidence is generic references

**Decision:** `assessment_evidence` stores `(evidence_type, evidence_id)` pairs (observation, track, asset, incident, source, classification, alert) instead of typed foreign keys.

**Why:** Assessments may reference any combination of evidence across modules; a typed FK per evidence kind would require schema changes whenever a new evidence source appears. The trade-off (no referential integrity on evidence ids) is acceptable while assessments remain operator-facing, and it is the seam where a future intelligence layer adds validation or a knowledge graph.
