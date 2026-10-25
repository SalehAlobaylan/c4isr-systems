# EMAD Architecture and Development Direction

> **Current phase correction — 25 October 2026:** Preserve this long-term design reference where compatible with [EMAD_CURRENT_SCOPE.md](EMAD_CURRENT_SCOPE.md). Current delivery is map/Public Context for the **Middle East and the rest of the world**. Simulation execution, actors, sensing and exercise workflows require a later user-led design discussion; only reusable geographic environment preparation belongs to the current phase. Existing implementation does not settle that future design.

**Version:** 2.0  
**Status:** Proposed target architecture and engineering roadmap, supersedes `EMAD_ARCHITECTURE_AND_DIRECTION.md` v1.0 after adoption  
**Date:** 2026-10-19  
**Companion:** `EMAD_FINAL_DATA_STRATEGY_v2.md`  
**Inputs:** the user-provided data strategy, existing EMAD architecture/product/implementation-plan documents, and the prior two v1.0 consolidation documents  
**Implementation status:** existing `IMPLEMENTATION_PLAN.md` **reports** Phases 0–21 complete. This is a document claim, not a code audit.

## 1. Executive architectural decision

**EMAD is one integrated software system with two product pillars:**

1. **Public Context & Intelligence (PC):** an evidence-rich geospatial platform presenting selected public sources, time-aware observations, real-world tracks, satellite scenes, analytical findings and human-reviewed assessments.
2. **3D Modeling & Simulation (SIM):** an incrementally built, eventually heavyweight native module for authoring and exercising simulated environments with terrain, aircraft, UAVs, ships, vehicles, radar representations, abstract synthetic sensors and projectiles/missile visual models; it owns simulated world state, time, playback and evaluation.

**Controlled Operations (CO) is a supporting capability, not a third pillar.** The existing C2-related modules are repurposed to support exercise ownership, scenario tasking, simulated-asset telemetry, operator permissions, task/command acknowledgments and audit. Real hardware and live operational command are **not** assumed future requirements.

The architecture preserves C4ISR information-model discipline (source, observation, detection, classification, track, assessment, operational picture, reviewed decision) while honestly describing the product as **OSINT/Intelligence plus a simulation and evaluation environment**. EMAD is not claiming to be a deployable defense C2 system.

**Hard constraint:** The 3D module is inside the EMAD product, repository and user experience, but the simulation compute, physics, asset processing and rendering may run as their **own executables**. “Module” does not mean “load a 3D engine into the Go HTTP process.”

## 2. What changes versus the former direction

| Design question | Prior direction v1.0 | Decision in v2.0 |
|---|---|---|
| Product center | Broad C4ISR core with Public Context, Controlled Operations, Simulation | **Two pillars only:** Public Context/Intelligence and 3D Simulation |
| Controlled assets / commands | Independent private operations world and potential hardware bridge | **Exercise infrastructure:** simulated controlled actors and commands; real hardware out of scope |
| Simulation | Later module using C2 facilities | **Long-term strategic pillar**, with contracts and data/asset foundations started early |
| Data model | Three equally emphasized data worlds | Public evidence plus separately isolated exercise input, exercise observations and sealed truth |
| Public data priorities | USGS/FIRMS, aircraft/AIS, then imagery | GIS/terrain + USGS/FIRMS + Copernicus early; moving feeds follow as bounded sources |
| COP | Combined operational picture including controlled assets | Explicit **Public Context COP** and **Exercise COP**; no unmarked mixture |
| Intelligence | Feature set of broad C4ISR | First-class product workflow, with evidentiary review and scene export |
| External hardware | Deferred but on the eventual roadmap | **Not planned** unless the user explicitly reopens this product decision |
| Performance | Real-time operator updates | Split data-processing, sim-tick, rendering and dashboard update rates |
| Simulation standards | Normalized contracts; engine undecided | GLB/glTF + georeferencing + simulation contracts first; HLA/DIS studied only if federation required |

**Preserve, do not discard:** the existing Go application and its modular principles; PostGIS; TanStack/MapLibre operator UI; existing observations/tracks/evidence provenance, missions/commands/audit, deterministic scenario runner, OpenAPI/protobuf, auth/RBAC, observability and tests. **Change their product ownership and expose their scope explicitly.**

## 3. Capability map and system-level architecture

```text
                         EMAD PRODUCT / IAM / AUDIT
                                      |
            +-------------------------+-------------------------+
            |                                                   |
 PUBLIC CONTEXT & INTELLIGENCE                        3D SIMULATION MODULE
 +-------------------------------+                    +------------------------------+
 | Reference GIS / source feeds  |                    | Scenario + scene authoring   |
 | USGS / FIRMS / Copernicus     |                    | Terrain + model asset library|
 | ADS-B / AIS / weather         |                    | Aircraft/UAV/ship/vehicle     |
 | Imagery + evidence artifacts  |                    | Radar / projectile visuals    |
 | Detections / external tracks  |                    | Virtual clock + simulation   |
 | Claims / assessments / review |                    | Synthetic sensor generation  |
 | Public COP / history          |                    | Exercise-controlled actors   |
 +---------------+---------------+                    | Evaluation + after-action     |
                 |                                    +---------------+--------------+
       approved and licensed                                           |
       context snapshot export                                synthetic observations
                 |                                             / exercise telemetry
                 v                                                     v
       +---------------------+                            +--------------------------+
       | Frozen Context      |--------------------------->| Exercise Gateway         |
       | Package + manifest  |                            | scoped/run-only ingestion |
       +---------------------+                            +-----------+--------------+
                                                                      |
                                                        +-------------v--------------+
                                                        | EXERCISE COP + SUPPORT C2   |
                                                        | assets / tasks / commands   |
                                                        | simulated acknowledgments  |
                                                        +-------------+--------------+
                                                                      |
                                             privileged truth --------+--> evaluator ONLY

                        COMMON PLATFORM INFRASTRUCTURE
          Go application / auth / contracts / PostGIS / object storage
                 jobs / observability / WebSocket read models

 PUBLIC CONTEXT UI: real evidence   ||   SIMULATION STUDIO: synthetic exercises

      No automatic public-track -> controllable-asset promotion.
      No direct simulation-ground-truth -> exercise COP shortcut.
```

**Architectural test:** if the user disables the simulator, the Public Context product still works. If the user disables all external live feeds, a synthetic exercise can run using a frozen/hand-authored scene. The two pillars enhance each other but neither depends on the other's uptime.

## 4. Internal boundaries and ownership

### 4.1 Shared platform kernel, deliberately small

Owns authentication, workspaces, roles and permissions, audit, user configuration, source catalogue metadata, contract/version registry, storage references, event publication and health monitoring. It **does not** become a giant domain package. New modules depend on explicit application interfaces, not arbitrary cross-module SQL joins.

### 4.2 Public Context domain

Owns authoritative public *records* as received, normalized interpretations, external-tracking read models, source freshness/coverage, reference GIS and evidence workspaces. It handles errors and revisions without silently manufacturing facts.

The fundamental pipeline remains:

```text
source record -> observation/evidence -> detection / track hypothesis
              -> source-linked analytical assessment
              -> optional human review -> Public Context COP
```

Report extraction and AI model output are **proposals**, not new independent evidence producers. Classification, exact identity, affiliation and confidence remain distinct.

### 4.3 Geospatial imagery and analysis

Bounded `ImageryScene`/`SpatialChange`/`AnalysisJob` capabilities initially: catalogue by STAC; preserve acquisition time, geometry, cloud/quality and processing lineage; compare scenes in a reproducible environmental use case (e.g., flood/vegetation change). Keep original binaries in object storage and only references/metadata in PostGIS.

As workload grows, Python or another suitable raster-processing runtime can run independently. This does not force the Go core into microservices.

### 4.4 Simulation authoring and scene preparation

Simulation authoring owns scenario versions, actor templates, event scripts, environment selection and editable scene structures. Scene preparation owns terrain/reference transforms, model import, tiling/LOD, materials, asset versions, processing jobs and source licenses. **Authoring a model does not assert that its dynamics are accurate**.

Separate `ModelAsset` (render geometry), `ActorTemplate` (configurable synthetic identity/behaviors), `SimulationActor` (a run-time instance), and `ExternalTrack` (public observed hypothesis).

### 4.5 Runtime and engine boundary

A future simulation executable owns the virtual clock, actor lifecycle, state update loop, simulation-world state, and rendering adapter, or exposes world snapshots to an independent viewport. Allow pluggable fidelity levels. Basic visual/kinematic behaviors are enough for initial integration; physically validated specialist models are not a prerequisite.

EMAD Go owns **run administration**, not per-frame physical calculations. It records scenario/run identity, published exercise observations, simulated task lifecycle, checkpoints/recordings metadata, permission checks and after-action results.

### 4.6 Synthetic sensing and exercise perception

Simulation truth produces **synthetic observations through explicit sensor adapters** that model missing samples, delays, quality indicators and uncertainty. The normal Exercise COP sees those observations, its derived tracks and authorized exercise asset telemetry, not all underlying actor positions.

Even when the 3D viewport intentionally shows full world state to a scenario designer, the exercise operator UI must remain scoped to the permitted perception model. Separate UI roles (`exercise_designer`, `exercise_operator`, `exercise_evaluator`) and data contracts.

### 4.7 Controlled Operations as support infrastructure

Reassign existing modules as follows:

| Existing capability | New principal role | What changes |
|---|---|---|
| `assets` | Exercise-controlled entities | Namespaced per run; may retain historical app API but default UI has no live-hardware semantics |
| `telemetry` | Simulated controlled actor health/pose | Scoped to exercise; not automatically a real GPS/device feed |
| `missions` | Scenario tasks and exercise workflows | Goals/status within exercise, not live mission command authority |
| `commands` | Simulation task messages and acknowledgments | Only run-scoped virtual recipients; audit and no cross-run dispatch |
| `operators` / RBAC | Public analysts, scene authors, exercise operators, evaluators | Distinct permissions for analysis, scenario authoring, run control and sealed truth |
| `incidents`, `alerts` | Public evidence workflows *and* run-scoped exercise events | Separate namespace, severity and life cycles; never commingle read models |
| `tracks` | External track hypotheses and exercise track hypotheses | Shared abstraction, partitioned storage/projections and explicit source origin |
| `scenarios` | Foundation of simulation lifecycle | Extend existing deterministic runner into richer 3D-specific integration later |

**Migration rule:** don't rename/delete existing endpoints or repurpose database rows overnight. Add `scope` and new semantics at service boundaries, migrate with explicit backward-compatible versions and scenario acceptance tests. Decommission unsupported live-control assumptions only after audit.

## 5. Trust boundaries and public-to-simulation interactions

### Flow 1. Public events to analysis

```text
USGS/FIRMS/ADS-B/AIS/Copernicus + approved reference GIS
  -> connectors + producer/provenance registry
  -> normalized source records and evidence
  -> event/area overlays, external tracks, imagery analyses
  -> reviewer findings and timeline
  -> Public Context COP
```

### Flow 2. Geospatial intelligence to 3D scene

```text
Selected Area of Interest (AOI) + reference geography + optional imagery
  -> check rights / coordinate systems / quality / date
  -> versioned ContextSnapshotManifest
  -> terrain / imagery drape / scene-asset conversion
  -> human-approved ScenarioContextPackage
  -> 3D scene load
```

**A frozen scene is not automatically a ground-truth replica of a real location.** Source accuracy, resolution, asset generalization, timestamp mismatch and renderer choices must be declared. Historical tracks added as scenario inspiration become explicitly synthetic actors; raw public tracks remain separately available as read-only evidence layers.

### Flow 3. Virtual environment to operator/evaluator

```text
SimulationRunner: run_id + clock + scene + actor templates + seed
         |
         +--> private WorldState ----------------------> Evaluator
         |
         +--> synthetic observations -> ExerciseGateway -> Exercise COP
         |
         +--> controlled exercise actor telemetry ----> Exercise Asset State
         |                                         |
         +<---- authorized exercise task/command <-+-- Exercise Operator
         |
         +--> run recording / checkpoint manifest ----> After-action review
```

At no point can the public data connector issue an exercise command. An exercise operator cannot write to live external records. Privileged truth is not carried inside ordinary WebSocket observation envelopes.

## 6. Four different clocks and performance layers

An architecture that sends every simulation actor update through Postgres and dashboard WebSockets will fail long before a complex simulator becomes visually convincing. Separate:

1. **External evidence time:** provider observed/published/received/processed time; delayed/revised data is expected.
2. **Simulation virtual time:** deterministic step identity, time scaling, pause, restart and scenario event order; does not equate to wall time.
3. **Render frame time:** viewer refresh cadence controlled by graphics engine and GPU budget, not database transaction rate.
4. **Control/evaluation time:** tasks, acknowledgments, checkpoints and after-action metrics at meaningful state changes.

Do not store every frame as a relational observation or publish every transform to the operator REST/WebSocket read model. Support selective state snapshots and bounded-rate broadcast for display; recorded binary/columnar simulation histories are separate artifacts. A live viewer may interpolate from received snapshots without becoming the authoritative physics engine.

**Reproducibility:** define event-log replay (stored events reproduce the same operator experience) separately from physical re-simulation (which may differ across hardware/physics engines). Store scenario version, engine build, assets version, seed, inputs, clock config and output hashes. State a reproducibility guarantee precisely rather than promising universal bit-level physics determinism.

## 7. Georeferencing and 3D data standards

### GIS-to-3D transform boundary

External layers predominantly use geodetic coordinates; a simulation scene has a local Cartesian reference. Require explicit WGS84/CRS metadata, origin/axes, meters vs feet, heading/pitch/roll conventions and vertical datum. Transform via a defined georeferencing component, potentially through ECEF and local ENU frames. Prevent silent height-above-ellipsoid vs MSL vs above-ground confusion.

Use PostGIS for authoritative intersections in public/exercise geospatial **queries**, while the renderer and simulation runtime own scene-space transforms. The 3D viewport is not an alternative GIS database.

### Asset and interchange choices

- **glTF 2.0/GLB:** portable mesh/material/animation delivery baseline; preserve original authoring assets and engine-native builds. Not a physics, sensor or streaming protocol.
- **OGC 3D Tiles:** evaluate for large georeferenced environment streaming or rich geospatial 3D. Not required for a small local test scene.
- **OGC STAC:** imagery/geospatial asset discovery/catalogue. Not a 3D motion-stream or command format.
- **OGC GeoPose:** useful reference when defining geospatial object location/orientation contracts; evaluate, don't force early adoption.
- **Protobuf/gRPC:** internal cross-process simulation envelopes where justified. Use existing Buf compatibility checks.
- **HLA IEEE 1516 family / DIS IEEE 1278:** possible future interoperability with multiple independent simulations. Don't deploy a federation runtime for a single engine. Current IEEE HLA framework standard is **1516-2025**, not only the superseded 2010 edition.

A future simulation engine evaluation should compare geospatial precision and large-world support, headless execution, scripting/extensibility, asset pipeline, Linux/Windows support, runtime footprint, open-source/commercial license, maintainability and interoperability. **Do not select Unreal/Godot/Unity from aesthetics alone.** Cesium engine connectors can be evaluated as tools, not as platform architecture decisions.

## 8. Data storage, service boundaries and practical deployment

```text
Operator React/Vite/MapLibre          Simulation Studio (embedded UI or native view)
       |                                             |
 REST/OpenAPI + WebSocket                        Studio control APIs
       |                                             |
       +----------------- EMAD Go Core ---------------+
                         PostGIS metadata
                           |        |
                      Object Store  Background Jobs / Workers
                           |        |
                   Imagery processing / Scene preparation
                                    |
                       Simulation Runtime (later)
                       (headless worker and/or GPU client)
                                    |
                       Exercise Gateway (typed contracts)
                                    |
                  Exercise projections / recording artifacts
```

**Go core:** keep modular-monolith application for metadata, evidence, assessments, run lifecycle, auth, audits, read models and low-frequency exercise tasking.

**Processing workers:** introduce Python for substantial remote-sensing raster analysis; model preprocessing/mesh conversion worker only when required. Run asynchronously with artifact references rather than shipping huge binaries in JSON.

**Simulation runtime:** one separate process at the point graphics/physics/clock needs demand it; potentially native desktop rendering for interactive authoring plus a headless runner for CI. This remains an EMAD module. A dedicated GPU server is not necessary for the first contract milestone.

**Datastores:** PostgreSQL/PostGIS for structured spatial metadata; object storage for imagery/terrain/3D assets, frozen scenario bundles and recordings. Redis/NATS/Kafka/TimescaleDB/Kubernetes are **not required by architectural principle**, and should be chosen only after measured volume/reliability needs.

**Distribution:** local dev can keep Go, Vite and PostGIS as currently documented. Simulation runtime and asset workers launch on demand, avoiding a continuously running expensive 3D service. Future simulation-host placement is a separate deployment decision.

## 9. Contract families, schemas and ownership

Proposed logical application contracts; **they do not imply one network service each**:

| Contract family | Payload/operations | Primary owner |
|---|---|---|
| `PublicSourceIngestion` | provider identity, source revisions, AOI, raw references, event/track candidates | Public Context |
| `EvidenceAndImagery` | artefact/scene registration, processing provenance, classification/assessment proposal | Intelligence |
| `ContextSnapshotExport` | eligible public layers, AOI, time range, rights and frozen package manifest | Public Context + scene preparation |
| `SimulationScenario` | scenario/version, actor templates, model and environment asset refs | Simulation authoring |
| `SimulationRun` | create, authorize, start, pause, resume, stop, reset, replay reference | Simulation orchestration |
| `SyntheticObservationStream` | run/environment scoped, quality/time/version and simulated producer | Exercise gateway |
| `ExerciseTelemetry` | virtual controlled actor state/health, not real hardware telemetry | Exercise support C2 |
| `ExerciseTasking` | virtual exercise command, expected ack/outcome, audit and authorization | Exercise support C2 |
| `SimulationRecording` | snapshots, event-log files, checkpoint manifests and playback metadata | Simulation runtime |
| `AfterActionEvaluation` | privileged truth comparisons, scenario quality metrics, report access | Evaluator |

**All cross-module messages** must encode `scope`, version, source identity, event ID, environment/run binding where applicable, and timestamp semantics. Explicit message types prevent confusing `EXTERNAL_TRACK`, `EXERCISE_TRACK`, `SIMULATION_ACTOR`, and `CONTROLLED_EXERCISE_ASSET`.

Example of a **minimal, safe run descriptor**:

```json
{
  "runId": "run-uuid",
  "scenarioVersionId": "scenario-hash-or-id",
  "contextPackageId": "context-hash-or-id",
  "environmentId": "exercise-lab",
  "scope": "EXERCISE_INPUT",
  "seed": 4217,
  "runtimeVersion": "sim-engine-build-id",
  "clock": {"mode": "fixed-step", "timeScale": 1.0},
  "status": "CREATED"
}
```

**This is an illustrative product schema, not a claim the API exists.** Define generated contracts only as the implementation arrives. Separate privileged evaluator truth from operator-visible events structurally, not just by front-end conditional rendering.

## 10. Identity, permissions, and isolation

Role families:

- `public_viewer`, `analyst`, `public_source_integrator`: public read, findings/reviews, or scoped ingestion.
- `scenario_author`, `exercise_operator`, `simulation_runner`: scene creation, exercise tasking or run publication, never broad admin by default.
- `evaluator`, `platform_admin`: privileged truth/results or platform management; evaluator access is explicitly audited and not implied by analyst role.

Example permissions:

```text
public.sources.read       public.observations.read    public.assessments.propose
public.assessments.review public.context.export      imagery.jobs.submit
sim.scenarios.author      sim.assets.manage          sim.runs.start
sim.runs.control          sim.synthetic.publish      sim.exercise_assets.telemetry
sim.exercise_commands.issue sim.evaluation.read       sim.truth.read [restricted]
```

Use non-human service identities for public connectors and simulation workers. Enforce resource ownership and `run_id` scope through backend application logic and DB queries; reject mismatched API/stream tokens; audit every override. **No live external actuator command bridge exists in the endorsed architecture.**

## 11. Operator product architecture

### Public Context Workspace

A geospatial dashboard with 2D MapLibre map, evidence filter, time scrubber, original source labels, imagery compare, observed tracks, event and area layers, source-health indicator, assessment authoring and source/provenance drill-down. Distinguish raw, derived, reviewed and uncertain information visually and semantically. Avoid turning it into an undifferentiated headline feed.

### Simulation Studio

A native workspace within EMAD with a 3D scene viewport (engine choice later), asset library, environment configuration, scenario authoring timeline, actor properties, simulation controls, synthetic sensor toggles, run diagnostics, performance budget, and recording/replay controls. Support 2D geographic overview alongside 3D, where useful. Headless scenario execution should not require an interactive browser.

### Exercise COP

A distinct workspace for synthetic exercise observations, exercise track hypotheses, exercise-controlled assets, tasks, acknowledgments, alerts and incidents. Exercise operators never automatically see hidden world truth. A separate evaluation interface overlays truth after a run only for permitted users.

### Shared design pattern

Allow a user to select an evidence item and explicitly **Export to Scenario Context**, with licensing and approval; do not provide a button that silently changes a public track into a controllable simulation actor. Visible workspace name, environment, time basis and source mode remain persistent in every view.

## 12. Repository plan: module inside EMAD, separate heavyweight runtimes

Target directory sketch, **not an instruction to restructure completed modules right now**:

```text
apps/
  c4isr-server/                # existing Go modular monolith
  operator-ui/                 # public-context + exercise UIs
  simulation-studio/           # later interactive 3D tool or engine project
  simulation-runtime/          # later headless / standalone simulation executable
  analysis-worker/             # later raster / ML worker
internal/
  sources/ observations/ tracks/ geospatial/ alerts/ assessments/
  assets/ missions/ commands/ incidents/ operators/ audit/ realtime/
  integrations/               # public source adapters + scoped identities
  evidence/ imagery/          # metadata, review, image jobs
  context_packages/           # licensed/frozen scenario exports
  simulation/                 # authoring, run orchestration, typed adapters
  evaluation/                 # evaluator metadata and authorization
api/
  openapi/
  proto/ingestion/
  proto/simulation/
  proto/evaluation/
assets/
  models/                     # source/model manifests; binaries may be external
  environments/
scenarios/                    # existing deterministic fixtures + richer scenes
storage/                      # manifests and storage conventions, not large binaries
```

Preserve Go's module boundaries and avoid cross-domain database coupling. `simulation-studio` or `simulation-runtime` can use different languages and build systems under this single project repository. Source binaries need not be committed; use object-store references or versioned artifact packages.

## 13. Model fidelity and project limits

Future 3D assets may depict military aircraft, UAVs, radars, ground vehicles, naval vessels and missile/projectile shapes. That **does not** imply implementing operational weapon guidance, classified profiles, realistic sensor performance or live engagement functionality. Stage the simulator's capabilities deliberately:

1. **Visual fidelity:** geometry, animation, materials, maps, cameras, labels, and asset management.
2. **Scenario fidelity:** actor spawn/lifecycle, scheduling, generic trajectories, environment state and synthetic event playback.
3. **Observation fidelity:** abstract sensor coverage, noise, delay and disagreements useful for evaluating EMAD perception.
4. **Scientific/engineering fidelity:** model validation, accuracy limits and domain-specific testing, considered only in a separately scoped program.

No engine or renderer should be advertised as realistic purely because its graphics look realistic. Every simulated component declares a fidelity tier, known simplifications and version.

## 14. Security, safety and engineering-quality gates

- Data provenance and provider rights checked before public-to-scene conversion; user-supplied 3D assets go through import validation and safe file handling.
- Simulation truth and normal exercise observations use different authorization paths and payloads. Unit, integration and streaming cross-scope tests should confirm non-disclosure.
- Simulation tasks bound to known run, exercise actor, operator authorization, idempotency and audit. Exclude external hardware routing.
- Handle clock drift, duplicate/reordered synthetic events, reconnect, message loss and run cancellation without corrupting scenario audit.
- PostGIS-backed spatial queries remain authoritative for geospatial records, with separate local-engine spatial calculations declared as simulation estimates.
- Resource ceilings for scene preparation, massive meshes/textures, concurrent runs, graphics CPU/GPU/memory, storage and data-egress cost.
- CI must include server-only deterministic run tests, one scenario-bundle integrity test, and exercised permission-negative cases. Interactive GPU visual tests can be optional until runner technology stabilizes.
- Repeated exercise results measured against defined metrics and tagged with simulator version and scenario manifest; avoid relying only on screenshot/visual inspection.

## 15. Roadmap, linked to existing implementation plan

Phases **0–21** of `IMPLEMENTATION_PLAN.md` remain a document-reported delivered baseline. Rather than renumber or claim them obsolete, implement new **product workstreams** with clear gates. The original future phases 22–26 are inputs, **not obligations in their prior order**; real-device integration and edge/hardware work are removed from the default product roadmap.

| Stage | Priority | Core deliverables | Exit criteria |
|---|---|---|---|
| **F0 Scope and trust boundary** | Immediate | New product taxonomy, separate Public/Exercise COP, service identities, `scope/run` authorization, compatibility tests | Existing flows work; no public/exercise/truth data leak |
| **P1 Public reference & events** | Highest | Bounded GIS reference and terrain catalogue, USGS + FIRMS events, source drill-down | Real events/area alerts operate with provenance, no fake tracks |
| **P2 Imagery Intelligence** | Highest | Copernicus STAC, artifact storage, before/after analysis, reviewable assessment | Reviewer traces finding to both scenes, algorithm version and quality |
| **S0 Simulation contract & snapshot** | Parallel foundation | Run/scope/time/coordinate schemas, frozen context package, existing scenario-runner adapter | Replayable scenario imports public context without leaking truth |
| **P3 Moving sources and Intel** | After P1/P2 stabilization | One ADS-B provider, AISStream, optional METAR, later GDELT/UCDP | Separate external track hypotheses; clear data quality and provider health |
| **S1 3D model pipeline proof** | Next simulation increment | Sample GLB models, one small licensed terrain region, scene validation and 3D prototype | Visual 3D scene loads with correct coordinates/licensing and bounded resources |
| **S2 Native simulation runtime** | After S0/S1 | Scenario lifecycle, virtual clock, actor world state, synthetic observations, exercise telemetry, headless tests | Real exercise COP consumes synthetic observations, not world truth |
| **S3 Exercise control & evaluation** | After S2 | Reuse assets/missions/commands, role-bound virtual tasking, recordings, restricted after-action evaluator | End-to-end virtual action lifecycle, reproducible audit, evaluator metrics |
| **S4 Long-horizon 3D evolution** | Long term | Broader 3D models, environments, richer scene tooling, fidelity benchmarks and runtime scaling | Repeatable and maintainable complex exercises without corrupting core data |

**Parallelism:** F0 then P1/P2 can proceed while a smaller S0 effort defines shared contracts. S1 begins once one frozen context package and coordinate transform have passed tests, not necessarily after every public feed exists. Avoid postponing all simulator work until “Intelligence is finished,” because Intelligence is open-ended.

### Mapping to old deferred phases

- **Old Phase 22 ISR expansion:** feeds/reference/imagery ingestion in P1/P2/P3.
- **Old Phase 23 Intelligence expansion:** assessments, conflicts, confidence, review, evidence lineage in P2/P3.
- **Old Phase 24 AI assistance:** optional analysis jobs inside P2/P3 after deterministic baselines exist.
- **Old Phase 25 External Integration:** *reinterpreted* as simulator ingress and public provider adapters (F0/S0/S2), not mandatory live defense integrations.
- **Old Phase 26 Edge and Hardware:** removed from endorsed roadmap until a new product decision.

## 16. Two end-to-end demonstrations to guide engineering

### Demonstration A: Public Context intelligence

An analyst selects a small region and historical date interval; EMAD shows public reference layers, ingests earthquake/thermal detections with source timestamps and revisions, discovers two public satellite scenes, produces a reproducible environmental-change candidate, and records a human-reviewed assessment. The assessment is traceable to original artifacts, limitations and analysis version. No fabricated track or unwarranted cause inference.

### Demonstration B: Integrated 3D simulation

A scenario author exports an approved, licensed geographic snapshot from Public Context, loads it into a simple 3D scene with one or more abstract aircraft/vehicle/UAV actors and synthetic observation generators, runs it using an explicit virtual clock, watches an **Exercise COP** update from synthetic observations, issues a permitted virtual task, receives a simulated acknowledgment, and then compares the observed exercise information to hidden ground truth in a privileged after-action workspace. Public data and other runs stay unchanged.

Demonstration B is the **architectural proof** that the two pillars collaborate without being coupled into one fragile data/compute subsystem. It does not require high-fidelity weapons or specialist physics.

## 17. Decision log: committed, deferred and rejected

### Committed

- Two product pillars, Public Context/Intelligence and 3D Simulation.
- Controlled Operations as exercise infrastructure, not an independent live-control program.
- One EMAD ecosystem/repository and user identity model, but runtime boundaries based on workload.
- Separate Public Evidence / Exercise Input / Exercise Observation / Exercise Truth.
- Preserve C4ISR evidence/provenance semantics and current Go/PostGIS/React architecture.
- Terrain/reference and satellite imagery have near-term priority due to their value in **both** pillars.
- Freeze versioned public-context packages before importing them into simulation.
- External tracks and synthetic actors are different domain entities.
- Engine-independent run/time/coordinate/identity contracts come before heavy 3D engine choice.

### Deferred decisions with evaluation trigger

| Decision | Trigger/evidence needed |
|---|---|
| Unreal vs Godot vs Unity vs alternative | Small georeferenced 3D prototype, engine constraints, platform licensing, headless support and performance measurements |
| Native GPU rendering vs web 3D studio | Required graphics fidelity, interaction design, deployment costs, operator devices |
| Standalone simulation process location | Measured scene complexity, CPU/GPU and network overhead |
| NATS/Kafka, broker or event log technology | Proven throughput, buffering, replay, reliability and independent scaling needs |
| HLA/DIS federation | Need to interoperate with multiple independent simulators or standards-bound partners; HLA 1516-2025 is a relevant current reference |
| Full physics, detailed sensor modeling | Independently scoped fidelity/validation requirements, credible ground truth and test resources |
| High-frequency data archive/time-series DB | Confirmed ingestion/storage cost and query performance limits |

### Rejected by default

- Generic operational C2/hardware control as the project's third pillar.
- Merging live public feeds into simulation truth with no versioned export.
- Using scene rendering/visual plausibility as proof of real-world accuracy.
- Streaming per-frame engine transforms through Postgres/standard COP WebSockets.
- GPU-heavy simulation inside the synchronous Go API process.
- Treating actor 3D mesh category as identity/affiliation or ML assessment.
- Automatic weapon-target selection, live engagement control or hardware dispatch.

## 18. Deliverable and governance changes

Recommended documentation precedence after adopting v2.0:

1. `EMAD_ARCHITECTURE_AND_DIRECTION_v2.md` states **product pillars**, module boundaries and roadmap.
2. `EMAD_FINAL_DATA_STRATEGY_v2.md` states dataset/evidence/scene/simulation contracts, trust partitions and source admission.
3. Existing `ARCHITECTURE.md`, `PRODUCT.md`, `IMPLEMENTATION_PLAN.md` remain useful **historical implementation baselines**; revise them separately to remove former assumptions that real controlled operations are a main future pillar.

Before requesting a coding agent to implement everything, create small, reviewed tasks at F0/P1/P2/S0 with explicit scope, non-goals, migration and test gates. Do not simultaneously scaffold every imagined microservice and simulation subsystem.

## 19. Verified technical references (recheck versions and rights at implementation)

- OGC SpatioTemporal Asset Catalog (STAC): https://www.ogc.org/standards/stac/
- OGC 3D Tiles 1.1: https://docs.ogc.org/cs/22-025r4/22-025r4.html
- OGC GeoPose: https://www.ogc.org/standards/geopose/
- Khronos glTF runtime asset format: https://github.com/KhronosGroup/glTF
- IEEE HLA architecture, active 1516-2025: https://standards.ieee.org/ieee/1516/6687/
- Cesium georeferencing concepts (technology example, not engine selection): https://cesium.com/learn/unreal/ref-doc/classACesiumGeoreference.html
- Copernicus Data Space STAC: https://documentation.dataspace.copernicus.eu/APIs/STAC.html
- OpenStreetMap data licensing: https://www.openstreetmap.org/copyright
- NASA FIRMS: https://firms.modaps.eosdis.nasa.gov/api/area/
- USGS earthquake GeoJSON feeds: https://earthquake.usgs.gov/earthquakes/feed/v1.0/geojson.php
