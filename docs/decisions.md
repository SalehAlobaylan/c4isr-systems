# Implementation Decisions

Material decisions made while implementing the C2 + Operational Awareness milestone. Each entry records the choice, the reason, and known limits. Current architectural intent lives in [Architecture and Direction v2](../EMAD_ARCHITECTURE_AND_DIRECTION_v2.md) and [Final Data Strategy v2](../EMAD_FINAL_DATA_STRATEGY_v2.md); this file records how the earlier foundation was realized.

**v2 migration note:** Historical decisions remain implementation evidence. D5's default point-to-track behavior, D6's wall-clock scenario timestamps, and the original realtime/role boundaries require deliberate evolution under [Implementation Plan v2](../EMAD_IMPLEMENTATION_PLAN_v2.md). [F0.1](implementation/F0_1_BASELINE.md) records the verified starting point; it does not implement the replacement principal or data-scope model.

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

## D9. Operator identity via header before Phase 17 (superseded)

**Historical decision:** Before bearer authentication was introduced, `X-Operator-ID` (default `operator-01`) identified the actor; it flowed into events with actor fields and into the audit log. A default operator row was ensured at startup.

**Why:** RBAC was introduced in Phase 17. The header remains only as an explicitly disabled-auth compatibility path for development and test harnesses.

**Current state:** Authenticated requests use the operator resolved from the configured bearer token; see D16.

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

## D16. Phase 17 authentication is configuration-backed bearer identity

**Decision:** The server resolves opaque bearer tokens from `C4ISR_AUTH_TOKENS` (`operator-id=token` pairs) to operator records and applies RBAC at the HTTP boundary. Auth is required by `config.Load`; only an explicitly disabled server accepts `X-Operator-ID` for local test harnesses.

**Why:** The first milestone needs trustworthy actor attribution and authorization without coupling the core to an external identity provider. Token verification and operator lookup form a narrow seam for a future OIDC/JWT adapter.

**Limits:** Tokens are static configuration secrets, not a user-facing login system. Production deployments must replace the local development token and should supply secrets through a secret manager.

## D17. Phase 18 faults remain application-service inputs

**Decision:** Scenario actions for duplicate, delayed, stale, conflicting, invalid, and missing observations, asset status changes, assessments, incidents, and command outcomes all call the same application services used by external integrations. Faults are recorded in the runner event cursor when validation rejects them.

**Why:** This keeps the scenario runner deterministic while ensuring it exercises validation, provenance, lifecycle, audit, and realtime behavior instead of creating a test-only state path.

## D18. REST and machine contracts are source-controlled artifacts

**Decision:** OpenAPI is the browser contract and generated TypeScript types are committed. Protobuf defines the first external ingestion contracts and Buf lint is part of CI.

**Why:** Browser clients and external systems need a reviewable compatibility boundary before adding generated server stubs or a separate gateway.

## D19. In-process metrics plus OpenTelemetry HTTP instrumentation

**Decision:** The server exposes a low-cardinality in-memory `/metrics` endpoint and wraps the router with `otelhttp`. Structured logs include request/trace/operator dimensions.

**Why:** This provides useful local and CI diagnostics without introducing a broker, metrics daemon, or exporter-specific domain dependency. Deployments can install an OpenTelemetry provider/exporter later.

**Limits:** Metrics reset on process restart and are intended for the current modular-monolith milestone; durable aggregation belongs to deployment infrastructure.

## D20. Human/service principals and explicit resource grants

**Decision:** F0.2 replaces configuration-only authorization with persisted human/service principals, hashed expiring/revocable credentials, and workspace/environment/partition/run grants. Configuration-backed operators remain an explicit compatibility path with live role ceilings. Source constraints and exact-run publication are separate from exercise control. Headers select existing authorized resources; they never grant access. Truth read/grant and privileged evaluation require explicit capabilities. Synchronous projections and ordinary administration do not imply them.

**Contract:** Retain existing domain permission identifiers; add `sim.scenarios.author`, `sim.truth.read`, `sim.truth.grant`, and `sim.evaluation.read`. Reserved review/context/imagery capabilities introduce no new endpoints yet. OpenAPI documents selection headers and `/auth/me` principal/resource fields; Protobuf resource selectors are additive contracts for a future adapter, not an implemented listener. See [F0.2–F0.4 implementation and rollout](implementation/F0_2_4_IMPLEMENTATION.md).

## D21. Ownership through transaction-local context and restricted-role RLS

**Decision:** F0.3 adds indexed `scope_id` ownership to all 28 domain tables and keeps operators as platform identities. Same-scope FK/polymorphic evidence triggers protect relationships. Unresolved legacy records retain NULL ownership and remain invisible to application views. Recorded run/namespace/audit provenance and graph validation drive the repeatable backfill; unknown records are never presumed public or synthetic. Scoped uniqueness and canonical namespaced caller IDs preserve independent resource idempotency.

**Decision:** F0.4 requires authorization at application-service entry and uses a restricted `emad_data` role plus transaction-local resource/source settings for every domain query or transaction. FORCE RLS enforces ownership even when the pool login owns tables or is a superuser. Pool reuse cannot retain another request's scope. Maintenance/control-plane access is explicit and separate. Missions, commands, controlled assets, and telemetry remain exercise-only.

**Compatibility and operations:** Existing token clients use a stored default resource; authorized scenario start selects its run. Explicit run headers support concurrent clients. Migration 20 requires privileged maintenance authority, excludes concurrent writes during backfill, and refuses automatic rollback. Minimal same-resource hub routing is included to prevent a scoped-read bypass; F0.5 still owns payload/metadata and subscription lifecycle hardening. These decisions supersede D15's unvalidated-reference limitation and D16's configuration-only authorization description while preserving their historical context.

## D22. Ordinary realtime is an authorized COP notification stream

**Decision:** F0.5 introduces envelope version 2 with authenticated resource context, an explicit topic/permission/source catalogue, checks before enqueue and send, idle credential refresh, and bounded buffering. Input/truth and unknown event families are denied by the ordinary socket even for a truth-authorized identity. Ready/resync controls require authorized API snapshots; the stream has no replay cursor. Credential subprotocol offers and legacy query credentials are consumed before instrumentation and never echoed. See [F0.5 implementation and rollout](implementation/F0_5_REALTIME.md).

## D23. Exercise publication references and disclosure are explicit

**Decision:** Exercise transport uses a shared allowlist for extensible metadata and quality, suppresses raw correlation hints and private names/errors, and restricts scheduled scenario inspection to authors. Public evidence and stored originals remain intact. Migration 21 adds stable random immutable observation publication references, preserving original storage IDs and links. References resolve through the existing restricted-role database boundary, including typed evidence writes and legacy track references. Public hashes of predictable actor names are unsuitable for hiding identity. Full UI workspace/cache separation remains F0.6.


## F0.6 — Explicit COP workspace lifetimes (2026-10-21)

Public Context and Exercise COP share the existing application shell and domain APIs. The live identity response offers only authorized ordinary resources and an opaque access revision; neither input packages nor ground truth becomes a selectable COP. Each mounted workspace captures one API resource, network cancellation lifetime, query client/key prefix, and realtime resource. Switching or losing authority discards cached/selected/replay state. Scenario start explicitly selects the new run; omitted-header compatibility remains server-bound for older callers. The real-backend browser gate and negative authorization matrix are required in CI. See [F0.6 acceptance](implementation/F0_6_WORKSPACES.md).

## D24. Public retrieval, immutable evidence, and catalog publication are separate

**Decision:** P1.1 introduces `publicingest` inside the modular monolith with immutable human-reviewed admission versions, source-bound service execution, server-owned HTTPS locations, persistent request budgets/leases, frozen query windows, and atomic evidence/checkpoint commits. Rights snapshots, normalized content hashes, exact original-byte hashes, and shared producer keys preserve provenance independently of a connector run. A transactional receipt publishes catalog visibility only after evidence exists; projection recovery does not refetch the provider. All six new tables use scoped restricted-role RLS and same-resource/source foreign keys.

**Scope:** Execution is a bounded service API invocation, with no polling daemon, external broker, or ML worker. Registration status and provider query coverage remain separate; an outage preserves existing evidence and never proves absence. Source-specific native adapters and event/area/moving projections remain subsequent tasks. No live provider is enabled. See [P1.1 implementation and operation](implementation/P1_1_PUBLIC_INGESTION.md).

## D25. Reported event revisions and geographic reference are separate models

**Decision:** P1.2 makes observation semantics explicit independently of geometry. Public reported-event revisions project from committed evidence, with a latest-known pointer, immutable history, explicit withdrawals, and PostGIS spatial matches. Event publication, its observation/current projection, corrected alerts, catalog visibility, and receipt acknowledgment share a transaction. Named events/detections cannot enter moving-track correlation. Original provider review/quality is not an EMAD-reviewed assessment.

**Decision:** P1.3 stores small immutable reference artifacts and PostGIS features in the existing database, with EPSG:4326, explicit elevation datum/grid registration, rights, transformations and hashes. Public reference versions are eligible inputs to S0 only when their metadata/rights allow; they are not observations, actors, or a second context-package standard. Regional Natural Earth/NOAA artifacts and preparation/import helpers make the first version reproducible. P1.4 adds the vetted native USGS adapter on the existing bounded service boundary. See [P1.2–P1.4 implementation](implementation/P1_2_4_PUBLIC_CONTEXT.md).


## D26. Frozen input versions, information clocks, and one context package contract

**Decision:** S0.1–S0.3 retain YAML as mutable authoring and store immutable, exact serialized scenario input bytes in `EXERCISE_INPUT`. Run configuration freezes version/build/seed/clock/context identities, while progress/lifecycle stay mutable. Restart creates a fresh namespace from those retained inputs. Author permission gates full definitions; ordinary run responses contain no actor templates. Re-simulation requires the original executable identity; reading recorded evidence remains separate.

**Decision:** Exercise information freshness and scheduled source instants follow the selected run clock; receipt, processing, audit, and delivery remain wall time. Public evidence uses public time semantics. Renderer-independent WGS84/ECEF/ENU transforms require resolved ellipsoidal height and recorded axes/units/orientation. Orthometric and AGL inputs are not relabeled. Component templates declare fidelity/version/simplifications without claiming physical validation.

**Decision:** `emad.context.v1` is the single bounded geographic package format. Human `context.export` approval freezes reviewed source versions, provenance, hashes, coverage, datum, processing, rights, and original retained bytes. Reference-only packages explicitly weaken reproduction. Packages contain no authored actors, perform no network retrieval, and are immutable with forced scope/source RLS. PostgreSQL holds these small initial artifacts; a standalone validator loads retained bundles offline. S0.4/M2 and engine selection remain later work. See [S0.1–S0.3 implementation](implementation/S0_1_3_SIMULATION_FOUNDATIONS.md).

## D27. Approved imagery exports extend the existing context contract

**Decision:** P2.4 retains `emad.context.v1` and adds the `context-snapshot-v2` processing variant. A bounded source-restricted preview pins published assessment revisions, prepared subsets, result hashes, original/current rights and transformations. Human approval freezes that exact selection after live authorization and transactional dependency rechecks. Derived finding geometry is clipped in PostGIS and remains explicitly synthetic context. Public originals and simulator truth remain separate. Small enriched bundles use the existing S0 frozen loader; its total snapshot budget remains explicit.

## D28. Moving public evidence projects source-bound hypotheses through native adapters

**Decision:** P3.1-P3.2 reuse admission, durable evidence/outbox, observations, tracks, and history. Immutable association decisions retain late and quarantined reports without rewinding accepted state. Source-bound identity/motion gates do not verify identity, infer affiliation, merge providers, or create controllable assets. Restricted source visibility covers legacy track read endpoints too. Separate bounded connector processes add no scheduler to API startup.

**Decision:** OpenSky is the initial ADS-B adapter because ADS-B.lol's public API is deprecated. Anonymous technical access does not authorize operational use; OpenSky's written agreement and approved archive rights are activation prerequisites. AISStream requires a server key, reviewed rights and compressed bounded captures with explicit gaps. No live provider activation is claimed from synthetic/local protocol acceptance. See [delivery, acceptance and limits](implementation/P2_4_P3_1_2_CONTEXT_AND_MOVING.md).


## P3.3 delivery update

P3.3 selects NOAA METAR and GDACS EQ structured bulletins for a bounded, admission-specific analyst question. Native JSON bulletin features are the retained source documents; linked HTML is not collected. Common producer/event lineage is exposed without automatic corroboration counts. Reports reuse the established assessment review workflow, exact evidence hashes and original/current rights checks. Weather/report projections are immutable, scoped and distinct from tracks/operations. See [implementation, acceptance and rollout](implementation/P3_3_WEATHER_AND_REPORTS.md).


## S1.1 delivery update

S1.1 reuses the existing S3 artifact store with separate input-scoped metadata rather than weakening public imagery/source admission constraints. Immutable model versions pin source/version, rights, worker/importer/Khronos dependency hashes and exact validation manifests. Separate templates and S0 frames remain authored input, not runtime actors. The initial portable profile rejects external resources and unsupported compression; core glTF structure/data and actual bounded image payloads are validated only in an independent worker. Engine-native artifacts remain opaque and separately pinned until an engine integration is evaluated. See [implementation, acceptance and rollout](implementation/S1_1_MODEL_ASSET_REGISTRY.md).


## S1.2-S1.3 / S2.1 delivery update

The first Simulation Studio viewport uses measured Three.js rendering over immutable georeferenced authoring scenes. A separate native Go executable owns abstract actor kinematics, virtual time and authenticated runtime lifecycle. Authoring, latest sealed runtime truth and ordinary operator perception remain separate. Migration 35 is required. M4 is accepted within the documented reference/synthetic-terrain visual-only scope; native sensing and Exercise COP integration remain S2.2-S2.3. See [engine comparison and decision](implementation/S1_2_ENGINE_DECISION.md) and [implementation, acceptance and rollout](implementation/S1_3_S2_1_STUDIO_AND_NATIVE_RUNTIME.md).
