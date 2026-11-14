# EMAD Implementation Plan v2

> **Current scope correction — 25 October 2026:** This plan retains implementation history. New work must follow [EMAD_CURRENT_SCOPE.md](EMAD_CURRENT_SCOPE.md): **map/Public Context for the Middle East plus worldwide support; environment preparation only for future simulation**. Do not automatically continue S1–S4 or a parallel simulation pilot. Detailed simulation design is deferred to a later discussion with the user. See the [map-first gaps and proposed sequence](docs/implementation/MAP_FIRST_PLAN.md#gap-report).

**Version:** 2.0  
**Date:** 2026-10-20  
**Status:** Bounded recovery installed — 2026-10-23. The [recovery acceptance](docs/implementation/HISTORICAL_RECOVERY.md#recovery-acceptance) and [delivery ledger](docs/implementation/RECOVERY_STATUS.json) record both persistent product workflows and their evidence. Canonical review identity, real geography preparation, automatic runtime launch and archive/replay are installed. Restricted live providers and actual human review retain explicit gates; further S4 remains future work. Per-task reports below remain historical evidence for their own finite profiles.
**Primary inputs:** [Architecture and Direction v2](EMAD_ARCHITECTURE_AND_DIRECTION_v2.md) and [Final Data Strategy v2](EMAD_FINAL_DATA_STRATEGY_v2.md).  
**Historical baseline:** [IMPLEMENTATION_PLAN.md](IMPLEMENTATION_PLAN.md), which reports Phases 0–21 complete.

**Usability follow-up, 2026-10-23:** The task statuses above describe bounded implementation/acceptance profiles. They do not mean that the default development account has public/authoring access, that sources/assets/workers are configured, or that the advanced simulation vision is finished. The normal-startup review exposed navigation and activation gaps; see [local startup and workspace access](docs/implementation/MAP_FIRST_OPERATIONS.md#historical-startup). A configured, visible end-to-end installation is a separate delivery requirement.

## 1. Purpose and precedence

Evolve the existing EMAD platform into two independently useful, deliberately interoperable product pillars:

1. **Public Context & Intelligence:** selected public geographic data, observations, imagery, evidence, external track hypotheses, and reviewed assessments.
2. **3D Modeling & Simulation:** a native EMAD module for scene and scenario authoring, virtual actors, synthetic sensing, execution, recording, replay, and evaluation.

Existing Assets, Missions, Commands, Telemetry, Incidents, Operators, and Audit provide supporting infrastructure. Controlled Operations serves exercises; it is not a third product pillar.

The v2 architecture and data strategy govern product scope and data semantics. This plan translates them into implementation increments without superseding their decisions. Earlier architecture and product documents remain useful where compatible. Their C2-first positioning and deferred hardware roadmap do not determine new work.

The old implementation phases retain their historical numbering. New work uses **F0, P1, P2, S0, P3, S1, S2, S3, and S4**, with stable task IDs within each workstream. The data strategy's D0–D5 labels are cross-referenced below; they are not a second execution schedule.

This is a dependency-driven plan, not a calendar commitment. Staffing, provider access, dataset rights, and measured workload will determine estimates. Detail is greatest for F0/P1/S0; later workstreams must be refined against evidence from preceding milestones.

## 2. Starting point: reuse and verification

The following observations come from targeted static inspection on 2026-10-20. They establish implementation touchpoints, not certification that the entire baseline works. Tests were not run when preparing this plan.

| Area | Evidence in the repository | Consequence for this plan |
| --- | --- | --- |
| Application composition | [internal/app/app.go](internal/app/app.go) wires domain services, repositories, events, HTTP, and realtime | Extend the modular monolith and preserve application interfaces |
| Evidence and interpretation | [observations](internal/observations/domain.go), [tracks](internal/tracks/service.go), and [assessments](internal/assessments/domain.go) are separate | Extend provenance, revision, and review semantics rather than replace these modules |
| Area observations | [Area rules](internal/arearule/service.go) and [migration 00018](db/migrations/00018_area_observations.sql) support footprints without track correlation | Reuse for appropriate spatial phenomena; do not manufacture footprints for point-only provider data |
| Point observations | The [track handler](internal/tracks/service.go) excludes areas, while other observations enter correlation | P1 must distinguish stationary point events from moving observations explicitly |
| Scenario execution | [RunScope](internal/scenarios/scope.go), [engine](internal/scenarios/engine.go), and persisted run events support namespacing, seeded execution, and virtual ordering | Reuse the runner as the first simulation adapter |
| Simulation time | The engine has virtual time, but emitted observations and telemetry use wall time | S0 must introduce explicit simulation-time semantics without rewriting original evidence time |
| Identity | [HTTP authentication](internal/platform/httpx/middleware.go) resolves bearer tokens to operator records and role permissions | Add service principals and resource grants; a source record is not an authenticated principal |
| Run isolation | Scenario resource IDs are namespaced; [picture queries](db/queries/picture.sql) use optional prefixes for some entities | Prefixes are not authorization. Enforce ownership on every read, write, relation, and subscription |
| Realtime | The [hub](internal/realtime/hub.go) broadcasts mapped events to connected clients; the [envelope](internal/realtime/envelope.go) has no partition binding | Add authorized routing and explicit audience-safe payloads before introducing public feeds |
| COP and replay | [Picture service](internal/picture/service.go) reconstructs selected state, but alerts/geofences and current attributes need further temporal/scope review | Reuse the implementation; do not equate the current scrubber with complete historical or simulation replay |
| Contracts | [OpenAPI](api/openapi/openapi.yaml), generated UI types, and [protobuf ingestion definitions](api/proto/c4isr/ingestion/v1/ingestion.proto) exist | Evolve contracts compatibly. Protobuf declarations alone do not establish an implemented gRPC listener |
| Verification | [CI](.github/workflows/ci.yml), [Taskfile](Taskfile.yml), and [integration tests](internal/integration/acceptance_test.go) exist | Re-run and record the baseline before changing behavior; retain regression coverage |
| New capabilities | Targeted inspection did not locate the four explicit partitions, scoped connector identities, public provider adapters, frozen context packages, or a native 3D runtime | Treat these as planned work, subject to the F0 inventory |

The first baseline report must record the commit, local changes, environment, executed checks, skipped checks, and known failures. A phase can be accepted only with evidence for its own criteria; the presence of a test file is insufficient.

**Execution update — 2026-10-20:** [F0.1 baseline verification and migration inventory](docs/implementation/HISTORICAL_DELIVERY.md#f0-1) is complete. It records executed checks, environment limitations, all current boundary families, and a read-only local provenance audit. The original static observations above remain planning history; the report supplies subsequent verification evidence.

## 3. Non-negotiable implementation rules

- Preserve functioning modules, existing identifiers, evidence links, and compatible APIs.
- Public Context must remain usable with simulation disabled. A frozen or hand-authored exercise must run with public feeds disabled.
- Keep `PUBLIC_EVIDENCE`, `EXERCISE_INPUT`, `EXERCISE_OBSERVATION`, and `EXERCISE_TRUTH` distinct throughout storage, authorization, projections, subscriptions, artifacts, and browser caches.
- External aircraft and vessel tracks remain hypotheses about observed entities. They never acquire command authority.
- Separate geographic reference features, raw provider records, observations, detections, tracks, claims, assessments, model assets, and simulation actors.
- AI outputs remain attributed proposals or analytical results. Human review must be explicit before publishing a reviewed assessment.
- Keep source reliability, measurement quality, model score, and assessment confidence separate. Shared upstream data is not independent corroboration.
- PostGIS owns authoritative spatial database queries. Scene-space calculations belong to the simulator and carry their declared limitations.
- The Go API owns metadata, authorization, evidence, orchestration, and bounded control traffic. Rendering, physics, large raster processing, and simulation ticks must not execute inside synchronous HTTP requests.
- Define stable run, time, coordinate, and artifact contracts before selecting a 3D engine.
- Introduce workers, object storage, queues, and additional processes when a deliverable needs them. Do not scaffold the entire future repository layout.
- Real hardware control, live engagement, operational weapon guidance, and autonomous engagement are outside this roadmap. Military-shaped visual assets do not change that scope.

## 4. Delivery sequence and dependencies

Public Intelligence receives the main implementation focus. S0 begins alongside P1 after F0; it does not wait for all intelligence features or public feeds.

| Workstream | Outcome | Entry dependencies | Data strategy mapping |
| --- | --- | --- | --- |
| **F0** | Product separation, service identities, scope enforcement, compatible migration | Baseline inventory | D0 |
| **P1** | Bounded reference GIS and terrain catalogue; USGS/FIRMS events with evidence drill-down | F0 accepted | D1 |
| **S0** | Engine-independent contracts, explicit clocks/coordinates, frozen input, existing runner adapter | F0 accepted; reference-backed package uses P1.3 | D0, foundation of D2/D4/D5 |
| **P2** | Copernicus imagery, reproducible analysis, review, integrated context export | P1 accepted; export uses S0.3 manifest | D2 |
| **P3** | Bounded external moving tracks, weather, selected report evidence | P1/P2 accepted and operationally stable | D3 |
| **S1** | Model pipeline, evaluated engine choice, first georeferenced 3D scene | S0 accepted and one eligible reference/terrain package | Foundation of D5 |
| **S2** | Separate simulation runtime, actors, synthetic sensors, Exercise COP | S0/S1 accepted | D4, part of D5 |
| **S3** | Exercise tasking, durable recordings, replay, restricted evaluation | S2 accepted | D4/D5 |
| **S4** | Broader assets, authoring, fidelity, and measured scale | S3 accepted; each increment has a specific use case | Continuing D5 |

Dependency details:

1. F0 schema and identity work may be developed in small increments, but **public-feed activation is gated on the complete F0 isolation gate**.
2. P1 ingestion semantics and S0 time/coordinate contracts can progress independently after F0.
3. S0 defines the context manifest and validates a hand-authored fixture first. Its reference-backed acceptance uses a small, permitted P1 dataset.
4. P2's export UI and imagery enrichment consume that same manifest. They do not create a competing package format.
5. S1 can begin with the accepted reference/terrain package; it does not require P3 or completion of every P2 analysis feature.
6. P3 and the simulation runtime workstream may progress independently. Neither must become a prerequisite for the other.
7. D4/D5 describe overlapping data capabilities. Their numbering does not postpone S0 contracts or the S1 asset proof until after moving feeds.

### Milestone checkpoints

| Milestone | Required evidence |
| --- | --- |
| **M0 — Safe foundation** | F0: existing exercise workflow preserved; unauthorized cross-scope and cross-run operations denied |
| **M1 — Public evidence workspace** | P1: reference layers and real point/thermal events with revisions, provenance, and zero accidental tracks |
| **M2 — Simulation-ready inputs** | S0: frozen reference package, verified coordinate transforms, explicit virtual time, headless runner adapter |
| **M3 — Intelligence demonstration** | P2: two scenes → reproducible environmental change result → human-reviewed assessment → approved export |
| **M4 — 3D scene proof** | S1: licensed models and terrain load at declared coordinates within a recorded resource budget |
| **M5 — Perception-driven exercise** | S2: hidden world state produces synthetic observations consumed by the Exercise COP |
| **M6 — Integrated exercise demonstration** | S3: virtual task → simulated outcome → recording/replay → privileged evaluation |

M2 may finish before M3. The milestone numbering groups outcomes and does not override the dependency table.

## 5. Contract and data boundary foundation

### 5.1 Ownership and access matrix

| Partition | Normal writers | Normal readers | Required boundary |
| --- | --- | --- | --- |
| `PUBLIC_EVIDENCE` | Approved source connectors; attributed analysis/review services | Authorized public viewers and analysts | Public principals cannot task exercise actors; imports never mutate the source evidence |
| `EXERCISE_INPUT` | Scenario authors and approved importers | Authorized authors and runners | Immutable versions bind runs to selected inputs; ordinary exercise operators do not receive hidden scenario definitions |
| `EXERCISE_OBSERVATION` | Run-bound synthetic publishers and exercise workflow services | Authorized operators for that run; evaluators | No hidden actor identity or truth state in observation payloads, history, metadata, or notifications |
| `EXERCISE_TRUTH` | Authorized runtime and evaluation services | Explicitly privileged evaluator/designer/controller grants | Separate access path and storage policy; no ordinary COP, evidence, or shared realtime delivery |

Permissions are explicit grants over resources, not implications of a broad role name. Platform administration does not silently grant truth access. An authorized designer may inspect world state through a privileged view; that privilege does not carry into the operator view.

Use a workspace as the initial ownership root. A single-workspace installation is supported without building a multi-tenant product, but every applicable record and request still has explicit ownership. Environment and run membership are validated by the server.

Reusable scenario versions and context packages belong to an exercise environment and may predate any run. Runs reference immutable inputs; actor instances, observations, telemetry, commands, and truth outputs are run-bound. Do not force a reusable package to belong to one run.

### 5.2 Contract families and first implementation owner

| Contract family from architecture v2 §9 | First increment |
| --- | --- |
| `PublicSourceIngestion` | F0 identity/envelope foundation; P1 provider normalization |
| `EvidenceAndImagery` | P1 provenance; P2 artifacts, analysis, and review |
| `ContextSnapshotExport` | S0 manifest/loader; P2 integrated analyst export |
| `SimulationScenario`, `SimulationRun` | S0 versions, clock, run lifecycle, adapter |
| `SyntheticObservationStream`, `ExerciseTelemetry` | S0 minimal adapter contract; S2 native runtime implementation |
| `ExerciseTasking` | F0 protects current command paths; S3 extends native exercise tasking |
| `SimulationRecording` | S0 defines replay identity; S3 implements durable recordings/checkpoints |
| `AfterActionEvaluation` | F0 reserves privilege boundary; S2 supplies sealed truth; S3 delivers evaluation |

These are application contracts, not mandatory network services. Use in-process interfaces until an executable boundary exists. Add protobuf services and generated code only with their implemented transport.

Common envelope requirements:

- Contract version, stable event/message ID, producer identity, workspace, partition, environment, and run where applicable.
- Explicit observed, received, and processed times for public evidence; provider publish/revision times when supplied.
- Explicit simulation elapsed time/tick/sequence and separate wall receipt time for synthetic output.
- Provenance and immutable artifact references; protected evaluator mappings are absent from ordinary messages.
- Idempotency and ordering policy per stream. Event IDs must survive retry rather than be regenerated as new domain events.
- Scope comes from authorized server context. Client fields may request a scope but cannot expand the principal's grants.

The two proposals use illustrative permission and payload names. F0 records one canonical permission vocabulary and wire naming convention in an architecture decision. Avoid implementing several synonymous permission families.

**Current delivery:** The [nine recovery stages](docs/implementation/HISTORICAL_RECOVERY.md#recovery-acceptance) address the audited software and installation gaps for the agreed bounded Saudi/Jeddah installation. Consult the [operations guide](docs/implementation/MAP_FIRST_OPERATIONS.md#historical-recovery-operations) and ledger for activation and external prerequisites. The original audit and implementation reports remain traceability history. Further S4 expansion requires its own defined increment and acceptance.

**Review closeout — 2026-10-22:** The user reports migrations 37 and 38 applied. [Recording-worker recovery, retry status, evaluation v2 correction and build/check coverage](docs/implementation/HISTORICAL_RECOVERY.md#review-readiness) complete the concrete gaps found before review. No additional migration is required; rebuild/restart the affected executables and UI. [Closeout evidence](docs/implementation/evidence/REVIEW_READINESS_ACCEPTANCE.json) supplements the original milestone records.

**Full plan review — 2026-10-22:** [Task traceability, findings fixed and final evidence](docs/implementation/HISTORICAL_RECOVERY.md#plan-review) cover all 32 original tasks and S4.1. Live refresh, delayed recording reads, reproducible evaluation sums, browser history and Windows staging checks were corrected without changing applied migrations. Rebuild/restart affected components; further S4 and provider activation remain separate work.

## 6. F0 — Scope, identity, and trust boundaries

**Status:** Complete — F0.1–F0.6 accepted, 2026-10-21. [Workspace behavior and F0 gate evidence](docs/implementation/HISTORICAL_DELIVERY.md#f0-6).
**Goal:** Safely introduce the two product workspaces while preserving the existing exercise workflow.  
**Entry:** No new infrastructure required.

### F0.1 — Verify the baseline and publish the migration inventory

**Status:** Complete — 2026-10-20. [Verification and acceptance evidence](docs/implementation/HISTORICAL_DELIVERY.md#f0-1); [source inventory](docs/implementation/F0_1_INVENTORY.json); [read-only legacy audit](docs/implementation/F0_1_LEGACY_AUDIT.sql).

- Execute the existing relevant checks; record failures separately from new work.
- Inventory routes, background actions, tables, relationship tables, events, UI queries, artifacts, and existing scenario fixtures.
- Map each data path to its intended partition, owner, reader, writer, and compatibility behavior.
- Classify legacy records from recorded provenance. Do not assume that every existing row is synthetic or public.
- Reconcile old product positioning in documentation while preserving historical phase records.

**Acceptance:** A reviewed inventory identifies every current ingress, read model, command path, and broadcast path; no unknown data is automatically published as public evidence.

### F0.2 — Define principals, resource context, and contract vocabulary

**Status:** Complete — 2026-10-20. [Implementation, rollout, and acceptance evidence](docs/implementation/HISTORICAL_DELIVERY.md#f0-2-4).

- Add distinct human and service principal types while reusing existing operator records and authentication where practical.
- Define grants for source-bound public ingestion, scenario authoring, run publication/control, exercise operation, review, and privileged evaluation.
- Define expiry/revocation and credential rotation behavior; connectors and runners do not use human supervisor tokens.
- Introduce an application resource context carrying validated workspace/environment/partition/run bindings.
- Audit denied cross-boundary attempts without copying sensitive payloads into ordinary logs.

**Acceptance:** A connector is limited to its approved sources and operations; a runner cannot publish to another run; an analyst does not inherit exercise command or truth access.

### F0.3 — Add ownership persistence and backfill safely

**Status:** Complete — 2026-10-20. [Implementation, rollout, and acceptance evidence](docs/implementation/HISTORICAL_DELIVERY.md#f0-2-4).

- Add explicit ownership/partition/run fields or ownership relations to affected aggregates, evidence links, current projections, histories, and audit records.
- Scope uniqueness and idempotency to the owning resource domain. A provider ID or scenario-local ID is not globally unique.
- Add indexed scope predicates and referential checks that prevent unauthorized cross-run links.
- Preserve legitimate run-to-input references through validated application operations. Cross-partition references require explicit semantics.
- Backfill known legacy scenario resources using run records, resource namespaces, and audit provenance; preserve original IDs.
- Keep ambiguous records inaccessible to new public/exercise views until classified. This is a migration quarantine state, not a fifth product partition.

**Acceptance:** Migration tests preserve row counts and evidence links, report unresolved records, reject invalid ownership relations, and exercise retry/resume of the backfill.

### F0.4 — Enforce boundaries in application services and queries

**Status:** Complete — 2026-10-20. [Implementation, rollout, and acceptance evidence](docs/implementation/HISTORICAL_DELIVERY.md#f0-2-4).

- Carry validated context through HTTP, scenario calls, background work, and repository methods.
- Enforce scope on list/get/search/count/history/replay queries, spatial evaluation, attachments, command recipients, and state transitions.
- Validate related objects under the same authorized context; caller-supplied IDs cannot bypass ownership.
- Scope incidents and alerts appropriately in both pillars. Missions, commands, and controlled asset telemetry stay within exercises.
- Make public observed entities ineligible for command dispatch at the application boundary.

**Acceptance:** Two runs and one public workspace can coexist. Cross-run reads, joins, mutations, attachments, and commands fail; legitimate existing exercise workflows still succeed.

### F0.5 — Separate realtime delivery and protect metadata

**Status:** Complete — 2026-10-20. [Realtime contract, disclosure policy, migration, and verification](docs/implementation/HISTORICAL_DELIVERY.md#f0-5).

- Add scope-aware event context and authorize subscriptions before delivery.
- Remove the global broadcast assumption; reconnects and permission revocation must preserve the same grants.
- Keep explicit payload mapping. Audit observation payloads, `trackHint`, external references, resource names, errors, and scenario event inspection for hidden identity disclosure.
- Reserve truth/input-sensitive event types for separate privileged delivery; ordinary subscriptions reject them by construction.
- Apply bounded buffering and a documented reconnect/resynchronization policy using authorized snapshots.

**Acceptance:** Simultaneous clients receive only permitted public or run events. Tests inspect serialized messages, denied subscriptions, reconnects, and sensitive metadata, not only rendered UI.

### F0.6 — Separate workspaces and close the compatibility gate

**Status:** Complete — 2026-10-21. [Implementation, compatibility, and real-backend browser acceptance](docs/implementation/HISTORICAL_DELIVERY.md#f0-6).

- Add explicit Public Context and Exercise COP navigation within the existing React application.
- Keep workspace, environment, run, source mode, and time basis visible.
- Scope TanStack Query keys and realtime handlers; clear inaccessible selection/cache state when identity or workspace changes.
- Keep current scenario controls in the exercise workspace. A full 3D Studio is not required at this stage.
- Preserve existing endpoint behavior through documented, server-bound compatibility mapping where possible; version unavoidable contract changes.
- Build the negative authorization matrix and a representative real-backend browser flow.

**F0 exit gate:** Existing scenario → observation → alert → incident → mission → simulated command → audit works inside an authorized exercise. Public records remain separate. All active reads/writes/subscriptions enforce the new boundary. Truth payload fixtures cannot reach ordinary users. No real public connector is enabled before this gate.

**Deferred:** Full imagery workflow, general-purpose identity administration, physical database separation for every module, a 3D engine, and broad infrastructure replacement.

## 7. P1 — Public reference data, events, and provenance

**Status:** Complete — P1.1–P1.6 and M1 accepted, 2026-10-21. [FIRMS, public history, acceptance and activation](docs/implementation/HISTORICAL_DELIVERY.md#s0-4-p1-5-6). Native USGS live acceptance is retained from P1.4; FIRMS fixtures and the same-AOI M1 gate pass. FIRMS live retrieval requires a map key. No permanent provider schedule is enabled by default.
**Goal:** Deliver a useful Public Context workspace for one bounded region.  
**Entry:** F0 accepted; select a small AOI and approved provider/dataset configurations.

### P1.1 — Source admission, provenance, and connector execution

**Status:** Complete — 2026-10-21. [Implementation, operating contract, migration, and verification](docs/implementation/HISTORICAL_DELIVERY.md#p1-1). Includes source-bound service retrieval, immutable rights/provenance, revision deduplication, persistent quotas/backoff/checkpoints, recoverable publication, and visible coverage. Five focused API/PostGIS tests and the full compatibility regressions passed. Provider-specific native adapters remain P1.4/P1.5; semantic routing remains P1.2.

- Extend the source catalogue with original producer, upstream provider, connector identity, dataset/product version, rights record, coverage, freshness expectations, and AOI/quota policy.
- Distinguish connector execution history from evidence identity and from an analysis that consumes the evidence.
- Implement bounded retrieval with timeouts, backoff, rate limits, checkpoints/watermarks, deduplication, and visible health.
- Define provider record ID/revision normalization and content hashing; retain original records or permitted references with retrieval metadata.
- Publish a revision only after its durable evidence is committed. Recover projection failures without duplicating evidence.
- Use vetted provider locations and artifact retrieval rules; credentials remain server-side.

**Acceptance:** Retries do not duplicate evidence; interruptions resume safely; provider outages show stale/unavailable coverage rather than an empty world interpreted as absence.

### P1.2 — Separate point events, areas, and moving observations

**Status:** Complete — 2026-10-21. [Implementation and acceptance evidence](docs/implementation/HISTORICAL_DELIVERY.md#p1-2-4). Explicit semantics/geometry roles, immutable provider revision history, current reported-event projections, atomic publication recovery, PostGIS point/footprint rules, and correction/withdrawal alert provenance. Earthquake/thermal fixtures create zero tracks; existing moving/scenario regressions pass.

- Introduce explicit semantic routing independent of geometry: a point can be an earthquake event, a thermal detection, or a moving observation.
- Preserve existing track behavior for compatible moving/scenario observations.
- Store public point events and valid area observations without creating tracks.
- Reuse PostGIS spatial rules for relevant event/area alerts and preserve the triggering record and rule version.
- Retain provider revisions as new evidence versions/supersession links; update projections and affected alerts with visible correction history.
- Treat measured footprints, derived buffers, and actual phenomenon boundaries as distinct.

**Acceptance:** Earthquake and FIRMS fixtures create zero tracks, including repeated ingestion and revisions. Existing moving-track scenarios still work. Event/area spatial rules use PostGIS.

### P1.3 — Reference GIS and terrain catalogue

**Status:** Complete — 2026-10-21. [Reference artifacts, limits, import procedure, and verification](docs/implementation/HISTORICAL_DELIVERY.md#p1-2-4). Retained Natural Earth Jeddah/Makkah features and NOAA ETOPO elevation samples have versioned rights/attribution, coordinates/datum, quality, transformations and checked hashes. Scoped import/catalog/artifact APIs, MapLibre reference views, and eligible immutable versions are available to S0.

- Import a compact, permitted selection of reference features from the proposed reference sources, plus one suitable elevation dataset.
- Record source/version, AOI, CRS, vertical datum, resolution, validity, quality, rights, attribution, and artifact checksums.
- Separate reference features from observations and exercise actors.
- Preserve source artifacts or reproducible authorized retrieval references; introduce minimal artifact storage when size requires it.
- Expose reference layers on MapLibre and make eligible dataset versions available to S0.

**Acceptance:** One reference region and terrain asset have reproducible metadata, bounded storage, visible attribution, and sufficient coordinate/rights information for a frozen package. This is not yet a 3D terrain editor.

### P1.4 — USGS event integration

**Status:** Complete — 2026-10-21. [Adapter contract, recorded fixtures, live smoke, and UI evidence](docs/implementation/HISTORICAL_DELIVERY.md#p1-2-4). The bounded native query preserves reported origin/depth/revision/quality and raw provenance, handles explicit withdrawals, publishes event/spatial projections, and supports source revision/original inspection. An authorized live service invocation retained five reports and produced zero tracks. Persistent source activation remains an explicit deployment operation.

- Implement one bounded earthquake feed/query adapter using the admitted source contract.
- Preserve provider event identity, reported origin time, location/depth semantics, revision time, quality, and raw provenance.
- Handle updated and withdrawn records according to verified provider behavior without deleting their history.
- Add point overlays, relevant spatial alerts, and drill-down to source revisions.

**Acceptance:** Recorded provider fixtures cover first ingest, duplicates, corrections, invalid data, and unavailable service. An authorized live smoke confirms the adapter against the selected provider configuration.

### P1.5 — FIRMS thermal detection integration

**Status:** Complete — 2026-10-21. [Implementation, contracts, rollout and verification](docs/implementation/HISTORICAL_DELIVERY.md#s0-4-p1-5-6). Reviewed NOAA-20 VIIRS NRT adapter, bounded resumable UTC-day retrieval, exact CSV provenance, categorical confidence and honest point/resolution representation; fixture and same-AOI integration gates pass. Live activation remains an explicit keyed deployment operation.

- Select one VIIRS product after checking coverage, access conditions, quality fields, and usage rights.
- Preserve acquisition time, instrument/product, reported quality, source identity, and footprint/resolution information when available.
- Represent thermal detections as reported. Do not label them confirmed fires, fire perimeters, identities, or intent.
- Use declared, separately attributed spatial rules if proximity or derived areas are needed.

**Acceptance:** Fixtures exercise duplicates, overlap, time handling, quality filtering, and malformed rows. The UI distinguishes detection location, measurement footprint, and any derived geometry.

### P1.6 — Public evidence drill-down and historical view

**Status:** Complete — 2026-10-21. [Implementation, contracts, rollout and verification](docs/implementation/HISTORICAL_DELIVERY.md#s0-4-p1-5-6). Source/phenomenon/AOI/observed-time/quality/revision filters, exact originals and lineage, labeled known/publication-time versus retrospective effective-time views, validated bookmarks, safe historical map presentation, and M1 acceptance pass.

- Add filters for source, phenomenon, AOI, observed interval, quality, and revision.
- Show original producer/provider, observed/received times, original record, correction lineage, rights, and source health.
- Keep raw records separate from interpretation. A source outage or filtered view must be understandable.
- Define historical view semantics explicitly: evidence effective at a time versus evidence known to EMAD at a time. Preserve both timestamps and label the selected interpretation.

**P1 exit gate:** M1 passes using reference geography, USGS, and FIRMS for one AOI; no fake moving tracks appear. Idempotency, revisions, source health, scoped evidence access, and spatial correctness are verified.

**Deferred:** Global ingestion, ADS-B/AIS, general sensor fusion, inferred fire perimeters, and automated intelligence conclusions.

## 8. S0 — Simulation contracts and frozen inputs

**Status:** Complete — S0.1–S0.4 and M2 accepted, 2026-10-21. [Frozen versions, time, coordinates and packages](docs/implementation/HISTORICAL_DELIVERY.md#s0-1-3); [scoped headless adapter, protected mappings and M2 verification](docs/implementation/HISTORICAL_DELIVERY.md#s0-4-p1-5-6).
**Goal:** Make the existing runner a safe, reproducible integration foundation without choosing a 3D engine.

### S0.1 — Scenario versions, runs, and explicit time

**Status:** Complete — 2026-10-21. [Implementation, contracts, limits, and verification](docs/implementation/HISTORICAL_DELIVERY.md#s0-1-3).

- Separate mutable scenario authoring from immutable scenario versions and run instances.
- Freeze scenario definition, actor templates, context references, seed, adapter/runtime build, and clock configuration at run creation.
- Define start/pause/resume/stop/failure/restart semantics. Restart creates a new run and preserves previous evidence.
- Define virtual epoch/elapsed time, tick or event ordering, sequence identity, time scale, and source-to-scenario time mapping.
- Keep receipt/processing wall time separate. Simulation freshness must follow the exercise clock; public freshness follows public time semantics.
- Define recorded event replay separately from re-simulation. State the reproducibility guarantee and any excluded fields.

**Acceptance:** A paused run does not advance virtual time; speed changes preserve event order; the same frozen fixture and seed produce equivalent declared outputs after excluding identified wall-time/run-ID fields.

### S0.2 — Coordinate and fidelity contracts

**Status:** Complete — 2026-10-21. [Implementation, contracts, limits, and verification](docs/implementation/HISTORICAL_DELIVERY.md#s0-1-3).

- Define WGS84 input, longitude/latitude order, scene origin, local axes, units, orientation convention, and transform version.
- Implement/test the chosen Earth-fixed/local-frame conversion independently of the future renderer.
- Record vertical datum and distinguish ellipsoidal, MSL, and above-ground heights; unsupported conversions must be rejected or explicitly unresolved.
- Attach declared fidelity and simplifications to actor/sensor templates.

**Acceptance:** Known coordinate fixtures and round trips meet a recorded tolerance, with axis/unit/vertical-datum errors detected. Zero altitude is not substituted for missing height.

### S0.3 — Context package manifest and integrity

**Status:** Complete — 2026-10-21. [Implementation, contracts, limits, and verification](docs/implementation/HISTORICAL_DELIVERY.md#s0-1-3).

- Define one versioned manifest covering AOI, selected time window, source references, dataset versions, retrieval times, asset hashes, CRS/datum, transform/processing versions, quality, rights, attribution, creator, approval, and reproduction status.
- Implement a minimal package builder/validator and content integrity check.
- Store immutable authorized bytes for offline reproduction where rights permit. If only references may be retained, mark the package dependency and weaker reproduction guarantee explicitly.
- Validate a hand-authored fixture, then one permitted reference/terrain package from P1.3.
- Keep authored actors and hidden scenario instructions outside ordinary public metadata and operator-visible package listings.

**Acceptance:** A corrupt or incomplete package fails validation; source updates cannot mutate an existing package; an approved retained package loads with live feeds disabled.

### S0.4 — Existing runner adapter and minimal synthetic ingress

**Status:** Complete — 2026-10-21. [Implementation, contracts, rollout and verification](docs/implementation/HISTORICAL_DELIVERY.md#s0-4-p1-5-6). Authorized typed synthetic observation/telemetry boundary, opaque contact/message identities, protected immutable evaluator mappings, frozen-context headless consumer, seeded failure and operational compatibility, and M2 acceptance pass.

- Adapt existing YAML scenarios and seeded failure actions to the new version/run/time contracts.
- Route synthetic observations and controlled telemetry through the authorized exercise application boundary.
- Expose observation identities independently of hidden actor identities. Keep evaluator mappings in a protected structure.
- Retain existing mission/command simulations as compatibility fixtures; the native runtime extension arrives in S2/S3.
- Add a headless fixture that consumes frozen context and emits synthetic observations without requiring graphics.

**S0 exit gate:** M2 passes: a frozen scenario and approved context package run headlessly, produce run-scoped synthetic evidence, preserve public data, enforce truth isolation, and demonstrate the declared replay guarantee.

**Deferred:** Engine selection, rich rendering, full physics, recording infrastructure, federation, and a GPU deployment.

## 9. P2 — Imagery intelligence and approved context export

**Status:** Complete - P2.1-P2.4 and M3 accepted, 2026-10-21. M3 uses recorded catalogue metadata and explicitly synthetic spectral fixtures; no live raster analysis is claimed.  
**Goal:** Deliver the first reproducible evidence-to-assessment workflow and an analyst-facing export into simulation.

### P2.1 — Artifact storage and imagery catalogue

**Status:** Complete — 2026-10-21. [Delivered behavior, acceptance evidence and operating limits](docs/implementation/HISTORICAL_DELIVERY.md#p2-1-3).

- Extend the artifact abstraction introduced as needed in P1/S0 to S3-compatible storage for imagery and derived outputs.
- Enforce partition-aware object keys/access, hashes, sizes, retention, rights, download authorization, and deletion policy.
- Implement bounded Copernicus STAC discovery and scene metadata: acquisition time, geometry, product/version, cloud/quality, asset references, and processing lineage.
- Track partial retrieval and retry without exposing incomplete scenes as analysis-ready.

**Acceptance:** An analyst can discover and inspect two eligible scenes for the same AOI. Unauthorized artifact access fails even with a known object identifier.

### P2.2 — Reproducible environmental analysis

**Status:** Complete — 2026-10-21. [Delivered behavior, acceptance evidence and operating limits](docs/implementation/HISTORICAL_DELIVERY.md#p2-1-3).

- Choose one limited environmental change use case, such as vegetation or flood-related change, with a documented deterministic baseline.
- Validate scene comparability, spatial alignment, resolution, masks, cloud/no-data conditions, and method limitations before comparison.
- Record input hashes, algorithm/build version, parameters, coordinate transforms, quality checks, and output artifacts.
- Run expensive raster processing asynchronously outside synchronous API handlers. A separate Python worker is justified here if needed.
- Persist job lifecycle, cancellation, retries, resource limits, and failures. Select the simplest durable job mechanism that meets the measured workload.

**Acceptance:** Repeating a job with pinned inputs/configuration reproduces results within its declared tolerance. Unsuitable scenes produce an explicit failure/limitation, not a confident change result.

### P2.3 — Reviewable assessments and intelligence workspace

**Status:** Complete — 2026-10-21. [Delivered behavior, acceptance evidence and operating limits](docs/implementation/HISTORICAL_DELIVERY.md#p2-1-3).

- Extend existing assessments with revisions, evidence versions, proposal/review/publish/reject states, reviewer identity, and rationale.
- Preserve source claims, algorithm/AI proposals, and analyst conclusions as distinct records or explicit types.
- Link a finding to both original scenes, derived artifacts, method version, quality limitations, and confidence rationale.
- Add imagery comparison, finding geometry, assessment history, and review controls.
- Keep model scores separate from analyst confidence; shared source lineage must remain visible.

**Acceptance:** An analyst traces a published assessment to its exact evidence and review history. Unreviewed AI output cannot silently become a reviewed fact or mutate track identity.

### P2.4 — Approved public-to-simulation export

**Status:** Complete - 2026-10-21. M3 preview/approval/offline validation and headless integration passed. [Acceptance and operating limits](docs/implementation/HISTORICAL_DELIVERY.md#p2-4-p3-1-2).

- Add AOI/time/layer selection and preview of the package contents, rights, quality, transforms, and omissions.
- Use S0's existing manifest/validator; add imagery references/assets without changing its trust model.
- Require an authorized user to approve and freeze the selected version.
- Label converted geometry as derived/synthetic and preserve source lineage. Public tracks, if used later as inspiration, require a new explicitly synthetic actor definition.
- Keep export one-way; scenario edits cannot modify original public evidence.

**P2 exit gate:** M3 / Demonstration A: select an AOI and time interval → inspect public context → compare two scenes → produce reproducible environmental analysis → publish a human-reviewed assessment → export an eligible approved context package.

**Deferred:** General-purpose intelligence automation, automatic identity/affiliation inference, unrestricted imagery archives, full knowledge graphs, and mandatory ML.

## 10. P3 — Bounded moving sources, weather, and reports

**Status:** P3.1-P3.3 implemented and individually accepted within documented fixture/protocol scopes. Live-provider activation/exit acceptance remains subject to the documented agreement, key and rights prerequisites.  
**Goal:** Add selected analytical value without turning the platform into an unbounded feed aggregator.

### P3.1 — External moving track projection

**Status:** Complete - 2026-10-21. [Implementation, native protocol/budget/isolation acceptance and operating limits](docs/implementation/HISTORICAL_DELIVERY.md#p2-4-p3-1-2).

- Extend scoped observation normalization for moving entities, with evidence links, source time, identity hypotheses, uncertainty, and staleness.
- Keep classification, claimed identity, affiliation claim, and confidence separate.
- Define track creation/update/expiry/reassociation behavior and preserve historical correlation decisions.
- Keep public tracks structurally ineligible for exercise commands and controlled asset registration.

### P3.2 — One ADS-B provider and AISStream

**Status:** Complete - 2026-10-21. [Implementation, native protocol/budget/isolation acceptance and operating limits](docs/implementation/HISTORICAL_DELIVERY.md#p2-4-p3-1-2). Live feeds are not enabled: OpenSky requires its written operational agreement; AISStream requires its server key and reviewed use/retention rights.

- Evaluate and choose one of the proposed ADS-B providers for the bounded AOI; implement AISStream as the maritime source.
- Check current access, coverage, rights, retention, and quotas before activation.
- Reuse connector health, deduplication, revisions, and provenance facilities.
- Bound stream subscriptions, reconnect, buffering, out-of-order handling, and retention.

### P3.3 — Weather context and selected reports

**Status:** Complete - bounded NOAA METAR station samples and GDACS EQ bulletins, evidence history, common-source lineage and independent report assessment review. [Acceptance and rollout](docs/implementation/HISTORICAL_DELIVERY.md#p3-3). Migration 33 is required. Disabled source templates do not claim live activation.

- Add METAR or one suitable official weather feed where the use case requires it; retain station/area coverage and uncertainty.
- Pilot selected GDELT/UCDP/official reports only against an explicit analyst question and admission decision.
- Preserve source documents, claims, cited locations/time uncertainty, extraction method, and analyst review.
- Identify shared upstream sources and syndicated reports. Multiple copies do not increase corroboration automatically.

**P3 exit gate:** One ADS-B integration and AISStream work within budgets; stale/missing coverage is visible; external entities remain non-controllable. Weather/report pilots are accepted individually and cannot block the simulation workstream.

**Deferred:** Global continuous tracking, generic news aggregation, composite threat scores, and automatic conclusions about intent.

## 11. S1 — Asset pipeline and georeferenced 3D proof

**Status:** Complete — S1.1-S1.3 and M4 accepted on 2026-10-22 within the declared visual-only reference/synthetic-terrain profile. [Engine decision](docs/implementation/HISTORICAL_DELIVERY.md#s1-2); [scene proof and rollout](docs/implementation/HISTORICAL_DELIVERY.md#s1-3-s2-1).
**Goal:** Prove scene interoperability and choose an engine from measured evidence.

### S1.1 — Model and environment asset registry

**Status:** Complete - immutable scoped visual models, separate actor templates/environment frames, retained source/native artifacts and a bounded standalone validation worker. [Acceptance and rollout](docs/implementation/HISTORICAL_DELIVERY.md#s1-1). Migration 34 is required. This task does not instantiate runtime actors or select an engine.

- Separate visual `ModelAsset`, behavioral `ActorTemplate`, and runtime `SimulationActor`.
- Support a small set of licensed GLB/glTF assets, source files/references, checksums, import versions, units, axes, materials, bounds, and fidelity declarations.
- Validate imports and bound decompression, mesh/texture size, external references, and processing time.
- Keep large binaries in artifact storage and retain generated engine-native variants separately.

### S1.2 — Engine evaluation

**Status:** Complete — shared-scene Three.js/Babylon hardware, software and no-GPU comparisons; requirements, budgets and scoped renderer selection recorded in [the decision and raw evidence](docs/implementation/HISTORICAL_DELIVERY.md#s1-2). Native physics/GPU engines remain future workload-specific decisions.

- Record requirements for geospatial precision, headless execution, scripting, native/web embedding, Windows/Linux support, licensing/distribution, maintenance, and performance.
- Shortlist candidates and run the same small scene, asset set, coordinate fixtures, and headless behavior test.
- Measure load time, memory, CPU/GPU use, positional error, and integration complexity on a declared machine.
- Record the selection and tradeoffs in an architecture decision. The original plan did not preselect an engine; the completed evaluation now selects the initial viewport adapter.

### S1.3 — First georeferenced scene and Studio shell

**Status:** Complete — immutable scoped scenes, explicit S0 terrain/actor coordinates, 3D/2D views, authoring properties, rights/fidelity and input-only Studio shell. [Acceptance and rollout](docs/implementation/HISTORICAL_DELIVERY.md#s1-3-s2-1). Migration 35 required.

- Load the accepted terrain/reference package and a few abstract actors using S0 transforms.
- Add a scene viewport, asset selection, actor placement/properties, attribution, and 2D geographic overview where useful.
- Separate privileged authoring visibility from exercise operator perception.
- Keep authoring integrated into EMAD's repository, identity, and navigation even if a native executable supplies the viewport.

**S1 exit gate:** M4: the same versioned scene loads reproducibly, aligns with coordinate fixtures, displays rights/fidelity information, and stays within a recorded resource budget. Engine selection is supported by the comparison evidence.

**Deferred:** Broad model libraries, federation, high-fidelity dynamics/sensors, weapon guidance, and production GPU clusters. Evaluate 3D Tiles only when the scene-streaming workload needs it.

## 12. S2 — Native runtime, synthetic perception, and Exercise COP

**Status:** Complete — S2.1-S2.3 accepted, 2026-10-22. M5 passes within the declared abstract native perception profile. [Delivery and verification](docs/implementation/HISTORICAL_DELIVERY.md#s2-2-s2-3-s3-1).
**Goal:** Run the native simulation as a separate executable while the Go core administers the exercise.

### S2.1 — Runtime lifecycle and actor behavior

**Status:** Complete — independent compiled Windows/Linux headless runtime, version/build negotiation, exclusive service lease, virtual clock, static/waypoint actors, stop/cancellation/failure, idempotent bounded snapshots and separately authorized truth. [Acceptance, limits and rollout](docs/implementation/HISTORICAL_DELIVERY.md#s1-3-s2-1). No native sensor/COP projection is claimed by this task.

- Implement authenticated run start/attach/pause/resume/stop, runtime version negotiation, heartbeat, cancellation, and failure reporting.
- Own virtual clocks, actor lifecycle, private world state, and simple declared behaviors in the runtime.
- Support headless execution using the same scenario contract.
- Publish bounded snapshots/events; keep render frames and high-frequency physics out of relational observation ingestion.

### S2.2 — Synthetic sensor adapters

**Delivered:** 2026-10-22. [Implementation and acceptance](docs/implementation/HISTORICAL_DELIVERY.md#s2-2-s2-3-s3-1).

- Convert private truth into explicit synthetic observations with configurable sample rate, missing samples, delay, noise, and quality.
- Maintain observation/track identities independently of hidden actor identities.
- Publish controlled exercise asset telemetry only to authorized operators for those assets and runs.
- Keep truth storage and inspection under separate grants, artifact paths, and payload types.

### S2.3 — Exercise ingestion and operational picture

**Delivered:** 2026-10-22. [Implementation and acceptance](docs/implementation/HISTORICAL_DELIVERY.md#s2-2-s2-3-s3-1).

- Implement the cross-process gateway using the accepted contracts and required transport.
- Enforce run status, principal binding, sequence/idempotency, bounded ingestion, duplicate/reordered events, reconnect, and cancellation.
- Project synthetic evidence into exercise tracks, alerts, and permitted asset state.
- Update the Exercise COP with explicit virtual time and observed-versus-controlled semantics.
- Let viewports interpolate display snapshots without becoming authoritative simulation state.

**S2 exit gate:** M5: an exercise runs with public feeds disabled; synthetic sensors can miss or delay an actor while the evaluator still sees its sealed truth. Ordinary APIs, streams, logs, and artifacts reveal only the permitted perception. Restart/failure cannot contaminate another run.

**Deferred:** Detailed specialist physics, distributed simulation federation, unbounded per-frame archives, and global infrastructure scaling.

## 13. S3 — Exercise operations, recording, replay, and evaluation

**Status:** Complete — S3.1–S3.3 and M6 accepted, 2026-10-22, within the declared abstract profile. [Delivery and verification](docs/implementation/HISTORICAL_DELIVERY.md#s3-2-s3-3-s4-1).
**Goal:** Complete Demonstration B using existing control infrastructure and durable evaluation evidence.

### S3.1 — Exercise-controlled workflows

**Delivered:** 2026-10-22. [Implementation and acceptance](docs/implementation/HISTORICAL_DELIVERY.md#s2-2-s2-3-s3-1).

- Reuse Assets, Missions, Commands, Telemetry, Incidents, Operators, and Audit.
- Bind virtual recipients, issuer permissions, task/command IDs, and outcomes to one run.
- Extend the runtime adapter for acknowledgment, completion, rejection, timeout, and failure.
- Preserve idempotency and correlate outcomes with the original command; stopped runs cannot receive new tasking.
- Test delayed acknowledgment and reconnect without cross-run dispatch or duplicate execution.

### S3.2 — Recording and replay

**Delivered:** 2026-10-22. Separate durable operator/truth journals, content-addressed artifacts, bounded read-only historical replay and compatible selected-state checkpoints. [Implementation and acceptance](docs/implementation/HISTORICAL_DELIVERY.md#s3-2-s3-3-s4-1).

- Record bounded event logs, selected state snapshots, checkpoints, runtime build, scenario version, asset/context hashes, seed, and clock configuration.
- Store large histories as appropriate artifact files; keep queryable metadata in PostgreSQL.
- Separate operator-visible recordings from privileged truth recordings and indexes.
- Replay recorded output without reissuing commands, ingesting it as new public evidence, or mutating the original run.
- Apply current requester authorization to every replay read; historical permission records do not grant present access.
- Preserve state transitions needed for historical status, classification, alerts, permissions, and time interpretation; do not silently read present-day state into a historical frame.
- Define and test checkpoint compatibility and storage/retention limits.

### S3.3 — After-action evaluation

**Delivered:** 2026-10-22. Versioned metrics over recorded perception/truth, explicit audited evaluator grants and deliberate release of numeric summaries. [Implementation and acceptance](docs/implementation/HISTORICAL_DELIVERY.md#s3-2-s3-3-s4-1).

- Give authorized evaluators an audited comparison of recorded observations against sealed truth.
- Start with defined perception and workflow metrics: missed/late observations, track association errors, duplicate handling, command lifecycle timing, and scenario completion.
- Record metric definitions, time basis, denominator, versions, and uncertainty.
- Separate privileged results from any intentionally released operator report; sanitize and authorize that release explicitly.

**S3 exit gate:** M6 / Demonstration B: approved geographic package → 3D scene → virtual actors and synthetic observations → Exercise COP → permitted virtual task and simulated outcome → recording/replay → privileged evaluation. Public evidence and unrelated runs remain unchanged.

**Deferred:** Claims of universal bit-level physics determinism, operational weapon effectiveness evaluation, and external hardware actuation.

## 14. S4 — Sustained simulation expansion

**Status:** S4.1 delivered; further expansion remains a long-term backlog refined against concrete scenarios and S3 evidence.  
**Goal:** Grow simulation depth while preserving contracts, independence, and maintainability.

Candidate increments:

- Broader aircraft, UAV, helicopter, vehicle, vessel, radar-representation, and abstract projectile visual libraries.
- Richer scene/scenario authoring, terrain tools, environmental presets, and reusable behavior templates.
- Larger geographic environments, asset streaming/LOD, and rendering optimization.
- More varied abstract sensing and information-quality scenarios.
- Validated specialist models only under separately scoped fidelity requirements, known limitations, and domain-specific verification.
- Runtime placement, concurrency, archival formats, and compute scaling driven by measured limits.
- HLA/DIS evaluation only if interoperability with independent simulators becomes an actual requirement.

**Acceptance for every increment:** A concrete scenario, versioned assets/contracts, fidelity statement, baseline comparison, regression/isolation evidence, and CPU/GPU/storage budget. Visual plausibility alone is not model validation.

### S4.1 — Reusable sensor-quality presets

**Delivered:** 2026-10-22, selected explicitly by the user as the third task in this increment. [Implementation and acceptance](docs/implementation/HISTORICAL_DELIVERY.md#s3-2-s3-3-s4-1).

- Save immutable, content-addressed abstract observer profiles with explicit limitations.
- Reapply profiles across scenes; pin their documents and exact values into frozen versions.
- Treat edits as custom settings or a new profile version; preserve existing scene hashes and behavior.
- Bound preset count and observer settings, retain author audit and environment isolation, and compare sampled behavior with the prior custom-settings baseline.

Further S4 increments remain unimplemented and require a concrete acceptance scenario. This task does not claim a larger visual library, new sensing physics, global terrain streaming, federation or unlimited scale.

S4 does not reopen live hardware or engagement control.

## 15. Migration and rollout strategy

Apply this sequence to F0 and later schema/contract expansions:

1. **Inventory and characterize.** Record current consumers, migrations, permission behavior, fixtures, and retained data. Run baseline checks before modifying their assumptions.
2. **Expand compatibly.** Add ownership/version fields, contract variants, and server-side adapters. Regenerate sqlc/OpenAPI/protobuf outputs from their source definitions as applicable.
3. **Backfill with provenance.** Classify known data and report ambiguity. Retain old IDs, links, and timestamps. Re-running a migration/backfill must not duplicate or relabel evidence.
4. **Enforce before exposure.** Enable complete scope validation and truth-safe mapping across REST, background services, SQL, realtime, artifacts, and caches. Add constraints after verified backfill.
5. **Cut over consumers.** Update UI queries, worker credentials, fixtures, and scenario adapters together. Legacy callers may use an explicitly configured server-bound exercise context; omitted scope never means unrestricted access.
6. **Enable bounded capabilities.** Activate one admitted provider/AOI or exercise runtime at a time, with quotas and health checks.
7. **Retire compatibility deliberately.** Remove obsolete paths only after consumer verification and documented migration. Preserve historical evidence under retention policy.

If rollout fails, stop the new connector/runtime or revert compatible application behavior while retaining ownership enforcement and committed evidence. Do not recover by turning off authorization, exposing quarantined rows, resetting the database, or destructively replacing public history.

Unknown legacy records must not block use of correctly migrated data, but cannot be silently assigned a default public partition. Rollback/restore procedures must be tested on a disposable copy before use with retained evidence.

## 16. Verification and operational acceptance

### 16.1 Existing checks to retain

The [Taskfile](Taskfile.yml) and [CI workflow](.github/workflows/ci.yml) are the source of executable commands. At F0.1 record actual outcomes for:

- Go tests and vet; concurrency-sensitive race tests.
- Real PostgreSQL/PostGIS integration tests, enabled through `task test:integration`.
- Frontend type/build checks and existing Playwright workflows.
- Generated OpenAPI types and protobuf lint/compatibility checks.
- Relevant migration and staging operations validation.

Plain `go test ./...` does not prove PostGIS integration acceptance: the integration harness is opt-in. Existing mocked browser coverage also does not replace a real-backend workflow for new authorization and data boundaries.

### 16.2 New acceptance matrix

| Concern | Required verification |
| --- | --- |
| Scope and principals | Allow/deny matrix for humans and services; two workspaces, two environments, and at least two runs; lists, counts, details, histories, relations, spatial queries, and commands |
| Streaming and browser state | Cross-scope subscriptions denied; reconnect/revocation checked; no stale cache disclosure after identity/workspace changes |
| Truth confidentiality | Inspect serialized REST/WS responses, errors, payloads, actor hints, scenario inputs, artifact URLs, ordinary logs, and replay output |
| Migration | Fresh schema and populated legacy database; known/ambiguous provenance, relation integrity, idempotent backfill, compatibility flow, and restore rehearsal |
| Provider ingestion | Recorded fixtures for duplicates, corrections, withdrawal when supported, bad timestamps/geometry, outages, quotas, and source lineage |
| Spatial correctness | PostGIS point/area rules and AOI queries; no fabricated geometry; coordinate/vertical-datum fixtures for simulation |
| Imagery and assessments | Input comparability, masked/no-data behavior, reproducible result, reviewer history, evidence versions, and denied artifact access |
| Context packages | Hash/schema/rights validation, corruption/missing input, immutable revisions, approved import, and offline reproduction claim |
| Runtime and replay | Virtual-time ordering, pause/resume, seeded faults, reconnect/cancel/failure, headless execution, recording replay without side effects |
| Pillar independence | Public workflow with simulator disabled; retained-package exercise with live providers disabled |

CI should use deterministic provider fixtures and require no live provider credentials or GPU for core checks. Live provider smoke tests verify current integrations separately. Add optional engine visual checks after S1; headless scenario and package-integrity checks remain required.

### 16.3 Resource and reliability gates

Before accepting a workstream, record its test machine/deployment profile, AOI, input volume, concurrency, duration, measurement method, and configured ceilings.

| Workload | Measures and limits to record |
| --- | --- |
| Public connectors | Request quota, ingestion lag, duplicate/revision counts, rejection rate, outage recovery, retained bytes/day |
| COP/API | Bounded result size, query latency, projection delay, subscription fanout, reconnect recovery, slow-client behavior |
| Imagery/jobs | Scene/asset size, job duration, CPU/memory, retry count, cancellation, concurrent jobs, storage/egress |
| Context/3D import | Bundle size, validation/import time, mesh/texture limits, coordinate tolerance, failure recovery |
| Simulation | Tick budget, observation rate, viewport update rate, run concurrency, CPU/GPU/memory, recording bytes/minute |
| Evaluation | Metric reproducibility, truth access audit, report size, retained dependencies |

Select numerical ceilings when the fixture and hardware are known, before the acceptance run. Do not claim scale from an unbounded demo. Record source freshness, provenance completeness, false conflation, review traceability, scene-load success, and replay validity rather than treating feed count as success.

## 17. Decisions to close at the relevant gate

| Decision | Responsible capability | Due | Evidence |
| --- | --- | --- | --- |
| Canonical scope/principal/permission vocabulary and legacy mapping | Platform + domain owners | F0.2/F0.3 | Route/table inventory and negative authorization cases |
| Scope constraints and any additional DB role/RLS defense | Platform/storage | F0.3 | Query coverage, migration compatibility, threat boundaries |
| Initial AOI and source admission policy | Public Context | P1 entry | Coverage, analytical use, rights, quota, storage budget |
| Elevation/reference datasets | GIS Context | P1.3 | Resolution, CRS/datum, permitted derivatives, frozen-package feasibility |
| Artifact backend and retention | Platform + data owners | First artifact increment; expanded P2 | Size, access policy, recovery, licensing, reproduction needs |
| Clock, coordinates, replay guarantee | Simulation + GIS | S0.1/S0.2 | Runner compatibility and coordinate/time fixtures |
| Environmental analysis method | Intelligence | P2.2 | Comparable scenes, reproducible baseline, limitations |
| ADS-B provider | Public integrations | P3.2 | Current access/coverage/rights and independent value |
| 3D engine and Studio delivery | Simulation | S1.2 | Shared-scene evaluation, headless support, distribution and resource measurements |
| Queue/broker or independent worker scaling | Owning workload | Only when needed | Measured durability, buffering, throughput, or process-isolation requirement |
| Federation and specialist fidelity | Simulation | S4 only if required | Concrete interoperability/validation use case |

Provider endpoints, quotas, terms, and engine capabilities must be checked against current official documentation during implementation. The source proposals' reference lists are research starting points, not permanent access or license guarantees.

## 18. First execution backlog

Start with small changes that can be reviewed and accepted independently. The entries below describe scope boundaries, not promises that each fits one pull request.

| Order | Work item | Concrete deliverable | Required dependency |
| --- | --- | --- | --- |
| 1 | F0.1 | Baseline verification report and route/table/event migration inventory | None |
| 2 | F0.2 | Principal/resource-context decision and tested authorization primitives | Inventory |
| 3 | F0.3 | Additive ownership migration, backfill report, and migration fixtures | Context contract |
| 4 | F0.4, first slice | Scoped source → observation → projection → read path, including relations/history | Ownership persistence |
| 5 | F0.4, remaining paths | Scoped assets, telemetry, spatial rules, alerts, incidents, missions, commands, audit, and scenario access | First slice pattern |
| 6 | F0.5 | Authorized realtime routing, payload review, reconnect/deny tests | Scoped domain paths |
| 7 | F0.6 | Workspace-aware UI/cache behavior, compatibility workflow, complete M0 gate | All active boundaries enforced |
| 8A | P1.1–P1.2 | Source/revision foundation and point-event path with zero fake tracks | M0 |
| 8B | S0.1–S0.2 | Frozen scenario/run/time and coordinate contracts with runner fixtures | M0; may proceed alongside 8A |
| 9 | P1.3–P1.4 | Compact reference/terrain catalogue and first USGS evidence slice | P1 foundations |
| 10 | S0.3–S0.4 | Frozen reference package, integrity validation, scoped headless adapter | S0 contracts + eligible P1.3 data |
| 11 | P1.5–P1.6 | FIRMS, full public drill-down, source health, M1 gate | P1 foundations and first provider |
| 12 | P2.1–P2.4 | Imagery catalogue, deterministic analysis, review, integrated export | M1; export also needs S0 manifest |

S1 can start after M2 and the eligible terrain package, independently of P3. Keep S0 visible in active planning so intelligence's open-ended backlog cannot indefinitely postpone the simulation foundation.

Each implementation task must state its user-visible outcome, task ID, touched interfaces, migration/compatibility impact, acceptance cases, resource limits where applicable, and explicit non-goals. Do not mark a whole workstream complete because its first endpoint or prototype exists.

## 19. Definition of done

A task is complete when:

- Its behavior is implemented through the required backend, storage, contract, and UI paths.
- Existing compatible behavior remains covered; intentional changes have a documented migration.
- Applicable scope, provenance, temporal, coordinate, and truth-protection tests pass.
- Generated artifacts match contract/query sources.
- Failure, retry, and recovery paths have been exercised where applicable.
- Required rights, attribution, and retention records exist for introduced data/assets.
- The relevant user workflow has been demonstrated with recorded validation evidence.
- Documentation distinguishes delivered behavior from the remaining roadmap.

A workstream is complete only when its exit gate and applicable independence/resource checks pass. Demonstrations A and B are cumulative product proofs; neither authorizes implementing all future modules at once.

## 20. Source traceability

| Plan content | Primary design basis |
| --- | --- |
| Two pillars and exercise-support operations | Architecture v2 §§1–4; Data Strategy v2 §§1–3 |
| Four partitions, ownership, and protected truth | Architecture v2 §§5, 9–10, 14; Data Strategy v2 §§2, 7, 10 |
| P1 reference GIS, USGS, FIRMS | Architecture v2 §15 P1; Data Strategy v2 §§4, 11–13 D1 |
| P2 imagery, review, context export | Architecture v2 §§4.3, 5, 15 P2; Data Strategy v2 §§5–9, 13 D2 |
| S0 clocks, coordinates, snapshots, runner reuse | Architecture v2 §§6–9, 15 S0; Data Strategy v2 §§7–8, 11 |
| P3 moving sources and reports | Architecture v2 §15 P3; Data Strategy v2 §§4, 13 D3 |
| S1/S2 native asset pipeline and runtime | Architecture v2 §§4.4–4.6, 7–8, 12–15; Data Strategy v2 §§6–10, 13 D4/D5 |
| S3/S4 exercise operation, replay, evaluation, growth | Architecture v2 §§13–17; Data Strategy v2 §§8, 11, 13–14 |
| End-to-end milestones | Architecture v2 §16 Demonstrations A/B; Data Strategy v2 §13 acceptance criteria |

Source documents: [Architecture and Direction v2](EMAD_ARCHITECTURE_AND_DIRECTION_v2.md) · [Final Data Strategy v2](EMAD_FINAL_DATA_STRATEGY_v2.md).
