# EMAD Data Strategy, Public Context + Simulation

> **Current phase correction — 25 October 2026:** [EMAD_CURRENT_SCOPE.md](EMAD_CURRENT_SCOPE.md) governs new work: map/Public Context across the **Middle East plus the world**, with **geographic environment preparation only** for later simulation. Preserve provenance, rights, coordinates, versioning and existing isolation; do not read future simulation contracts below as authorization to implement runtime or exercise features now.

**Version:** 2.0  
**Status:** Proposed authoritative strategy, supersedes `EMAD_FINAL_DATA_STRATEGY.md` v1.0 after project adoption  
**Date:** 2026-10-19  
**Companion:** `EMAD_ARCHITECTURE_AND_DIRECTION_v2.md`  
**Inputs:** the user-supplied v1.0 strategy and architecture/direction, `PRODUCT.md`, `ARCHITECTURE.md`, `IMPLEMENTATION_PLAN.md`  
**Delivery truth:** the implementation plan reports phases 0–21 complete. This strategy does not independently verify repository implementation or imply that the integrations below already exist.

## 1. Product decision and data thesis

EMAD now has **two product pillars**:

1. **Public Context & Intelligence:** assemble an evidence-backed, geospatially coherent view of the real world using selected public sources; expose uncertainty and human-reviewed interpretation rather than pretending to know more than the data supports.
2. **3D Modeling & Simulation:** author, run, visualize, replay and evaluate synthetic environments containing aircraft, UAVs, vehicles, ships, radar representations, simulated missiles/projectiles, terrain and abstract sensor behavior. The simulation module is native to EMAD but may require independent compute and graphics processes.

**Controlled Operations is no longer a third independent product/data pillar.** Existing assets, telemetry, assignments, missions and commands are **shared exercise-control infrastructure** primarily serving simulation. There is **no planned live fleet-control or real-weapon integration** without a new, explicit product and safety decision.

The architectural consequence: public evidence and synthetic exercise data can interact through *deliberate, versioned transformations*, but they do not share an authority model. External tracks are not controlled assets. Simulated entities can be controlled within an exercise only. Raw truth inside a simulator cannot masquerade as observed evidence.

**Optimization goal:** quality, provenance, repeatability, geographic correctness, and simulation utility, **not** feed count, global coverage or a generic C2 product.

## 2. Four logical data partitions under two pillars

These are **trust/isolation partitions**, not four product pillars:

| Partition | Contents | Who can write? | May affect what? |
|---|---|---|---|
| `PUBLIC_EVIDENCE` | Source records, GIS reference layers, imagery, reports, public observed tracks and reviewed assessments | Scoped external connectors, analysis workers, accredited reviewers | Public-context COP and explicitly exported context snapshots |
| `EXERCISE_INPUT` | Frozen imported context, authored scenes, scenarios, exercise actors, injected events, abstract behaviors | Exercise author and approved importer | One specified simulation run |
| `EXERCISE_OBSERVATION` | Synthetic sensor observations, exercise track hypotheses, simulated asset telemetry, exercise tasks/commands | Scoped simulation gateway / exercise controllers | Exercise COP, evaluation inputs and operator workflow |
| `EXERCISE_TRUTH` | Simulator-internal world state, actual actor identities, exact states, hidden ground truth | Simulation runtime/evaluator only | Evaluation, privileged debugging and after-action review only |

`PUBLIC_EVIDENCE` stays separate from the three exercise partitions. `EXERCISE_INPUT` may **reference** a licensed copy or snapshot of public evidence but cannot rewrite it. A fifth live-controlled-asset partition is **out of scope**, not a silent default.

Each relevant record carries `tenant_or_workspace_id`, `scope`, `environment_id`, and where applicable `scenario_id`, `run_id`; enforcement occurs in application authorization, queries, subscriptions and storage paths. Never rely on a UI label alone for isolation.

## 3. Explicit distinctions: truth, evidence, rendering, control

- **Reference feature:** road, building, coastline, elevation, facility or boundary. It describes geographic context; it is not a time-indexed observation or a controllable actor.
- **Raw external observation:** a public provider's time-scoped report or measurement, subject to reception gaps, false reports, revisions and upstream quality limitations.
- **Public observed track:** a hypothesis combining external moving observations; it is *not* the underlying real-world object's authoritative state.
- **Evidence artifact:** retained or referenced image, scene, report, data file or model output with source and rights metadata.
- **Detection / spatial change:** machine or analyst interpretation derived from source evidence; it can concern a point, area or object.
- **Assessment:** reviewable inference and its evidence, uncertainty, method, revision and author.
- **3D asset:** geometry, materials, animations and level-of-detail data to render an entity. A mesh is **not** evidence, physical behavior, identity or a sensor model.
- **Simulation actor:** a synthetic entity with scenario identity, state and behavior. Its visual model is replaceable. An actor is not an external track and need not represent a real asset.
- **Simulated controlled asset:** an exercise actor for which the exercise controller has authorized tasking rights. Existing EMAD asset/mission/command machinery is re-scoped here.
- **Synthetic observation:** deliberately imperfect information emitted by a simulated source. This, not the world's exact state, is what the exercise COP sees.
- **Ground truth:** privileged simulation world state for evaluation, unavailable to ordinary analyst/COP pipelines.

Identity axes must remain independent: `object_classification`, `entity_identity_hypothesis`, `affiliation_claim`, `source_reliability`, `assessment_confidence`, `simulation_actor_id`. Avoid a single overloaded `identity/status` field.

## 4. What public data EMAD actually needs

The data catalogue serves **two uses**: a trustworthy Public Context workspace and optional, licensed **world-building/reference inputs** for simulations. That second use elevates terrain and basemap data above many speculative news indicators.

### 4.1 Source prioritization, revised

| Tier | Sources | Why now | Primary information type | Simulation reuse |
|---|---|---|---|---|
| **P0 foundation** | OpenStreetMap/Geofabrik, Natural Earth and **one appropriately licensed elevation/terrain dataset** | The 2D COP and future 3D terrain need stable geographic reference | Reference GIS and geospatial asset catalogue | Frozen terrain/road/building/landcover scene assets, only when rights permit |
| **P0 observation** | USGS Earthquakes | Low-volume point events, provider revisions, spatial alerts without tracks | Event observations | Optional historical environmental event injectors |
| **P0 observation** | NASA FIRMS, appropriately selected VIIRS product | Thermal-detection points/footprints, source-quality and area-overlay logic | Satellite-derived detections / observations | Optional historical context; never pretend hotspots are exact fires or targets |
| **P0 intelligence** | Copernicus Data Space satellite imagery | First reproducible evidence-to-detection-to-assessment workflow | `ImageryScene`, evidence and spatial-change results | Background/context imagery only with resolution/date/rights qualifiers |
| **P1 moving entities** | Choose **one** of OpenSky or ADS-B.lol | External airborne track quality, stale signals, identity uncertainty | Moving observations / tracks | Historical read-only overlay or clearly transformed, synthetic exercise actor |
| **P1 moving entities** | AISStream | Independent maritime motion source, streaming lifecycle | Moving observations / tracks | As above; never treat AIS identity as authority |
| **P1 context** | METAR and carefully selected official weather feeds | Environmental conditions for COP; coarse scenario setup context | Station/area observations | Scenario weather presets with declared interpolation / uncertainty |
| **P2 intelligence** | GDELT, UCDP, official public reports | Narrative evidence, historical/candidate event analysis and review | Reports / claims / assessments | Scenario design references, never automatic truth |
| **P2 exploratory** | CelesTrak, GEOSA/official layers and regional public datasets | Only on specific use case, validation and license review | Orbital reference / GIS / documentation | Explicit approved context exports only |

This **reorders** v1.0 intentionally: terrain/reference assets and satellite imagery are part of the first main product increment; aircraft/AIS integrations remain important, but are not prerequisites for building Intelligence or preparing a 3D environment.

Do **not** start by globally archiving all aircraft or ships. Start with a small region of interest, such as a limited area of Saudi Arabia, the Red Sea or the Gulf, subject to provider coverage and rights.

### 4.2 Non-priority sources

Reddit, Wikipedia pageviews, generic news mirrors, prediction markets, DOUGHCON, duplicate ADS-B-based dashboards, generic popularity scores and third-party composite threat scores remain excluded from authoritative ingestion. They can become clearly marked analyst research references if a concrete case justifies them.

### 4.3 Source independence rule

Separate `original_producer`, `upstream_provider`, `connector`, and `derived_analysis`. Multiple analyses of the same provider stream are **not independent corroboration**. Treat syndicated media stories and aggregators likewise.

## 5. Three canonical information flows

### A. Public situational awareness

```text
Public source / licensed reference dataset
  -> Source registry + rights/quota/AOI policy
  -> Provider adapter + normalized event/scene/feature
  -> Immutable/versioned public evidence and spatial metadata
  -> {ReferenceLayer | EventObservation | Detection | TrackHypothesis}
  -> Reviewable assessment where interpretation is needed
  -> Public-context COP, optional rule-based alert and timeline
```

An earthquake or spatial thermal detection may trigger an alert **without a Track**. A news report or satellite change never silently becomes an authoritative observation of motive, identity or affiliation.

### B. Scene creation from public context

```text
Selected public layers + imagery + optional historical records
  -> License/usage validation + AOI/time/quality selection
  -> Source snapshot manifest (original references, hashes, versions)
  -> Terrain/scene conversion pipeline
  -> Versioned ScenarioContextPackage
  -> Exercise author approves and freezes it
  -> 3D environment loader
```

This is a **one-way, reviewable import**. The package describes *what was selected*, not the exact historical world state. Coarse satellite pixels cannot be used to assert vehicle-sized structures or infer hidden properties. A generated geometry is labeled derived/synthetic even when based on public geography.

### C. Simulation to exercise COP

```text
Frozen scenario package + synthetic actor definitions + seed
  -> Simulation runtime (private world state)
  -> Synthetic sensor adapter(s) and simulated-asset telemetry
  -> Exercise-only ingress (scope/run authorization)
  -> Exercise observations + tracks + exercise-controlled assets
  -> Exercise COP, tasks, acknowledgments and audit

Simulation truth -> sealed evaluation store -> after-action evaluator
                 (never direct normal evidence or COP updates)
```

A user-visible public track does not become an exercise-controlled entity by selecting it. An **explicit transformation** makes a *new synthetic actor*, with `derived_from_public_record` retained and a clearly synthetic label.

## 6. Data ownership and minimal domain catalogue

| Aggregate / object | Authority | Key characteristics |
|---|---|---|
| `Source`, `ConnectorRun`, `SourceHealth` | Public Context ingestion | Provider identity, source lineage, rate limits, AOI, health, schema version |
| `ReferenceDataset`, `ReferenceFeature`, `ElevationAsset` | GIS Context | Provider, CRS, vertical datum, spatial/temporal validity, license |
| `Observation`, `EventObservation`, `AreaObservation`, `Detection` | Public Evidence or exercise observations (segregated) | Raw references, time, geometry/quality, version and input provenance |
| `ExternalTrack`, `TrackObservationLink` | Public Context analytical projection | Hypothesis ID, supporting observations, staleness, revisions |
| `EvidenceArtifact`, `ImageryScene`, `SpatialChange`, `AnalysisJob` | Intelligence | Storage URI, STAC scene metadata, algorithms and reproducibility |
| `Claim`, `Assessment`, `AssessmentEvidence`, `AssessmentRevision` | Intelligence | Distinguish source statement, model suggestion, analyst conclusion, review |
| `Scenario`, `ScenarioContextPackage`, `ScenarioVersion` | Simulation authoring | Frozen context manifest, assets, expected behavior, objectives, rights |
| `ModelAsset`, `AssetVariant`, `EnvironmentAsset` | Simulation asset pipeline | GLB/glTF, engine-native versions, textures, LOD, authoring provenance |
| `SimulationRun`, `SimulationClock`, `RunCheckpoint` | Simulation orchestration | `run_id`, lifecycle, seed, clock, runtime version, reproducibility manifest |
| `SimulationActor`, `ActorTemplate`, `SimulatedSensor` | Simulation world | Actor-scoped identity, components, declared fidelity and capability limits |
| `SyntheticObservation`, `ExerciseTrack`, `ExerciseAssetState` | Exercise projection | Scoped to run, reports not truth, separate observed and controlled identities |
| `ExerciseTask`, `ExerciseCommand`, `CommandOutcome` | **Enabling controlled-operations infrastructure** | Authorized, simulated-only tasking and lifecycle with audit |
| `GroundTruthSnapshot`, `EvaluationMetric`, `AfterActionReport` | Restricted evaluator | Privileged exact states, comparisons and non-public results |

Use existing tables/modules where they fit. These names are intended bounded contexts and candidate types, **not** a directive to create every table now.

## 7. Minimal record envelopes and model separation

### 7.1 Public normalized observation

```yaml
observation_id: uuid
scope: PUBLIC_EVIDENCE
source_id: USGS
provider_record_id: example-id
provider_record_version: example-revision
origin_producer_id: USGS
connector_run_id: uuid
observed_at: RFC3339
received_at: RFC3339
processed_at: RFC3339
geometry: GeoJSON_or_reference
crs: EPSG:4326
quality_flags: []
raw_artifact_ref: optional-uri
content_hash: sha256
source_rights_id: rights-record-id
```

The above is **illustrative**, not a final wire schema. `observed_at` may be a time interval or be unknown for a historical claim; do not create false precision.

### 7.2 Simulation run and observation envelope

```yaml
run_id: uuid
scenario_version_id: immutable-id
context_package_id: immutable-id
runtime_version: version
seed: integer
simulation_time: duration-or-scenario-instant
wall_received_at: RFC3339
sequence: run-scoped-integer
scope: EXERCISE_OBSERVATION
source_id: run-scoped-simulated-sensor
actor_id: hidden-from-normal-COP-if-ground-truth
observation_payload: domain-specific-normalized-data
```

Visible exercise messages must not expose hidden actor identity through metadata. Maintain internal `sim_actor_id -> exercise_track_id` mappings only in evaluator or controller scopes as appropriate. Controlled exercise asset identifiers can be visible to authorized operators, but other hidden actor truth stays sealed.

### 7.3 Snapshot manifest

A `ScenarioContextPackage` should contain: `package_id`, `created_at`, `AOI`, `original_source_refs`, `dataset_versions`, `retrieval_timestamps`, `time_window`, `asset_hashes`, `projection_and_vertical_datum`, `processing_steps`, `rights_and_attribution`, `transform_versions`, `geometry_quality`, `imagery_quality`, `creator`, `approval`, and `reproduction_status`.

Never silently refresh a frozen scenario with newer imagery or roads. Create a **new package revision** instead.

## 8. Time, space and fidelity are first-class data

### 8.1 Time semantics

- External: `observed_at`, `published_at`, `received_at`, `processed_at`, `valid_from/to`, `revision_at` where applicable.
- Simulation: `scenario_time`, `sim_elapsed`, `wall_time`, `tick_index`, `time_scale`, `run_id`, `event_sequence`.
- Import/replay: an explicit `source_to_scenario_time_map`; historical real timestamps remain preserved.
- Pausing or accelerating simulation does not rewrite source observation time. Out-of-order real data is not an invalid simulation clock tick.
- **Recorded replay** (playing stored output) and **re-simulation** (rerunning engine logic) are different and must have separate reproducibility claims. Cross-platform physics or GPU results are not assumed bit-identical.

### 8.2 Geospatial semantics

- External API/display: declare WGS84 (`EPSG:4326`) and longitude/latitude order explicitly.
- 3D engine: support a documented Earth-fixed to local-frame transform, e.g. ECEF to local ENU; record origin, axes, units, heading convention and transform version.
- Altitude: preserve vertical datum and source quality; never confuse height above ellipsoid, MSL and above-ground elevation.
- PostGIS remains authoritative for spatial database relations; a rendering engine's coordinates are not automatically GIS truth.
- Reprojection, terrain mesh generation, simplification and imagery draping must preserve source and transformation provenance.

### 8.3 Fidelity semantics

Attach `fidelity_profile` to every simulated component, such as `visual_only`, `abstract_behavior`, `kinematic`, or `validated_specialized_model` (future, subject to separate review). Physics visuals and detailed 3D models do not certify physical accuracy. Synthetic radar display artifacts must be labeled as modeled signals, not actual military sensor measurements.

## 9. Artifacts, formats, storage and rights

- **PostgreSQL/PostGIS:** source registry; structured metadata; spatial reference features; bounded observations/tracks; scenario and run manifests; audits; assessments and evaluation indexes.
- **S3-compatible object storage:** raster scenes/COGs, model meshes and textures, 3D world bundles, run recordings and provenance-preserved reports. Store checksums, access policies, provenance and retention metadata in the DB.
- **STAC / STAC API:** suitable catalogue metadata for earth-observation scenes and related geospatial assets. Use it where appropriate; it is not a real-time track protocol.
- **GeoJSON / GeoParquet / COG (as warranted):** interoperability/efficient exchange for vectors and imagery, not one universal canonical DB format.
- **glTF 2.0/GLB:** recommended exchange/publishing format for many 3D model assets; keep authoring source and engine-native import artifacts separately. glTF does not define simulation physics or world streaming.
- **3D Tiles:** evaluate for large georeferenced city/terrain scene streaming, not for per-frame simulation-actor messaging.
- **Protobuf/gRPC:** only for actual cross-process simulation or high-volume ingestion contracts; do not use it to encode giant 3D binaries into operational-event streams.

**Licensing gate:** public visibility is not the same as redistribution or commercial-use permission. Retain dataset-specific use terms, derivative/share-alike duties, attribution, storage/retention conditions and prohibited re-use. In particular, OpenStreetMap is ODbL; a rendered or bundled simulation environment may carry attribution and database obligations. Check upstream terms *before* repackaging assets, not at launch.

## 10. Identity, access and security model

- Create non-human connector identities with source/AOI/operation-specific permissions. Never reuse a supervisor bearer token.
- Simulation workers receive **exercise-scoped** `simulations.run`, `simulations.publish_observation`, `simulation_assets.read`, and limited `exercise_telemetry.ingest`; not live asset or admin authority.
- Ground-truth access is restricted to evaluator, test harness and specially authorized run controllers. Do not send truth fields in normal browser payloads, even if hidden in UI.
- Scenario authors may import *approved* public context snapshots. They may not rewrite public evidence. Public reviewers may publish assessments, not submit simulation control commands by default.
- Enforce tenant/workspace + environment + run scope in PostgreSQL operations, API authorization and pub/sub routing. Test negative cases, including WebSocket cross-run leaks.
- Malware scanning, quotas, SSRF protection, source URL allowlists, signed artifact references, size and decompression limits apply to external data and imported 3D assets.
- Simulated tasking remains **within the virtual environment**. Any bridge to real-world hardware or weapon effects is excluded from this strategy and would require a new architectural/security approval.

## 11. Reliability, quality and costs

- No global high-frequency ingestion initially; prioritize bounded AOI, connector quotas, watermarks, idempotency, revision handling, deduplication and coverage visibility.
- Clearly distinguish **unknown/no coverage** from **confirmed absence**. Last observed position is not current truth.
- Preserve original source record ID and version; late corrections append a new revision or supersession link, not destructive replacement.
- Separate **ML score**, **source signal quality**, **analyst confidence** and **simulated sensor error profile**; never average them without a validated model.
- Archive reproducible *manifests* and bounded recorded output. Define content-addressed storage and quota/retention policies before capturing large imagery or high-rate 3D state.
- Sim runtime should stream/select only visible actor state to viewers; high-frequency transforms should not be stored as individual rows in Postgres nor broadcast unfiltered through the existing COP WebSocket.
- Introduce a queue/job system for long-running raster preparation or model conversion only when synchronous processing becomes unsuitable. Avoid mandatory Kafka, Kubernetes, Redis or service mesh.

## 12. Admission gates

A new public source must satisfy: documented legal basis, original producer, retrieval/API terms, geography/time coverage, normalization, revision semantics, quality/error indicators, reasonable operational cost, independent analytical value, and a defined COP/scene use.

A new simulation model must satisfy: asset provenance, engine/import support, coordinate/unit conventions, license, validated scope of behavior, scenario-version compatibility, reproducibility strategy, CPU/GPU budget and evaluator visibility policy.

A public-to-simulation export additionally requires: AOI/time-selection, accepted accuracy/resolution, rights to create a derived scene, frozen package manifest, transform chain, user approval, and an explicit **synthetic/derived** marker for generated geometry.

## 13. Implementation order and acceptance

### D0. Identity, scopes and data contracts

Deliver integration principals, partition guards, schema versioning, provider registry, record provenance and rights catalogue. **Pass:** wrong-scope ingestion/subscription is denied and audited; legacy C2 APIs remain functional under documented compatibility tests.

### D1. Public events, GIS and evidence drill-down

One compact reference GIS region; bounded USGS + FIRMS feeds; events and areas shown without fake tracks; source and revision drill-down. **Pass:** repeat ingestion is idempotent, event corrections visible, FIRMS not labeled as fire perimeter.

### D2. Imagery intelligence and snapshot export

Copernicus STAC discovery, scene metadata and a reproducible before/after environmental-change analysis. Build `ScenarioContextPackage` export from a licensed reference geography snapshot. **Pass:** analysts can review both original scenes and derived result; an exercise can load the frozen context, including attribution and transform provenance.

### D3. External moving tracks and reports

One bounded ADS-B provider and AISStream, then weather and targeted GDELT/UCDP pilot. **Pass:** stale tracks and evidence independence are handled; external moving entities remain non-controllable.

### D4. Synthetic observations / exercise control

Simulation scenario generates run-scoped synthetic observations and telemetric asset states, supports seeded failure cases, and seals truth separately. Reuse existing mission/command lifecycles for **virtual** exercise tasks only. **Pass:** exercise operations never mutate public evidence, and hidden truth cannot leak to exercise operator COP.

### D5. Durable 3D data pipeline

Model asset registry, GLB import/validation, terrain-derived scene, versioned package, 3D recording and after-action export. **Pass:** scene loads reproducibly, model types are visibly identified as synthetic, geometry and recording budgets are bounded.

**Success measurements:** provenance completeness, source freshness/coverage, review quality, false conflation rate, scene-load success, deterministic event replay, scope isolation, evaluation validity and bounded recurring cost. Do not use raw feed count as a success metric.

## 14. Decision register and explicit non-goals

**Accepted now:** two product pillars; existing C2 retained as exercise-support infrastructure; public and synthetic namespaces isolated; metadata/run/version first; reference GIS + imagery promoted; contracts before engine; no hardware dependency.

**Defer:** engine choice, distributed simulation federation, advanced physical realism, high-fidelity sensor models, globally continuous ingestion, full knowledge graph, production GPU cluster, edge control, real-world command authority.

**Excluded absent a fresh project decision:** using publicly observed aircraft/vessels as controllable assets; automatically constructing militarily significant conclusions from imagery; publishing simulator truth as collected evidence; any live weapon or actuator command bridge; autonomous engagement actions.

## 15. Verified documentation and references

Check current access conditions when implementing, since quotas, products and provider terms may change.

- OGC STAC (standardized geospatial asset metadata): https://www.ogc.org/standards/stac/
- OGC 3D Tiles 1.1 (massive geospatial 3D streaming): https://docs.ogc.org/cs/22-025r4/22-025r4.html
- OGC GeoPose (geospatial pose exchange): https://www.ogc.org/standards/geopose/
- Khronos glTF 2.0 (3D model runtime asset format): https://github.com/KhronosGroup/glTF
- USGS earthquake GeoJSON feeds: https://earthquake.usgs.gov/earthquakes/feed/v1.0/geojson.php
- NASA FIRMS area API: https://firms.modaps.eosdis.nasa.gov/api/area/
- Copernicus Data Space STAC: https://documentation.dataspace.copernicus.eu/APIs/STAC.html
- OpenStreetMap data/license: https://www.openstreetmap.org/copyright
- AISStream API and server-side auth: https://aisstream.io/documentation/
- OpenSky REST docs: https://github.com/openskynetwork/opensky-api/blob/master/docs/free/rest.rst
- ADS-B.lol docs: https://api.adsb.lol/docs
- UCDP downloads and citation: https://ucdp.uu.se/downloads/
- GDELT catalogue: https://www.gdeltproject.org/data.html
