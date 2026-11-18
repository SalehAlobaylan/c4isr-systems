# Infrastructure preparation for the later simulation phase

Prepared 15 November 2026 from the working tree at `8fe5b1c`, including the current uncommitted planning and operations updates. This is a proposed preparation backlog, not an implementation or acceptance record. F/V/D identifiers refer to the 26 implemented systems now consolidated in the [implementation record](../../IMPLEMENTATION_PROGRESS.md); the original plans remain in Git history.

## Recommendation

Add a bounded **Simulation Preparation** workstream focused on qualified, machine-readable environments and independent consumption. The 26 implemented systems establish substantial geographic preparation, delivery and viewing infrastructure. The next useful investment is making its outputs dependable inputs for another application.

Prioritize three results: a machine-readable statement of supported uses, a small reference area with independently checked data, and an independent consumer that can verify and query the exact permitted release offline. Extend the existing Go/PostGIS, object storage, workers and data tools. The workstream names below describe capabilities, not new services.

This review follows [the current scope](../../../EMAD_CURRENT_SCOPE.md). It proposes reusable geographic/data infrastructure and records decisions for the later simulation discussion. It does not select an engine, design scenarios, implement behavior or launch an exercise. “Prepared for the next phase” means the next team can establish suitability and identify missing inputs without rebuilding the data foundation. It cannot mean suitability for every possible simulation before its purpose and required fidelity are known.

## What is already covered

- **Source and release foundations:** admission, rights, lineage, geographic profiles, immutable releases, durable workers, integrity, rebuild, paired restore, metrics and compatibility are covered by F01–F10. Extend these contracts instead of creating another catalog, permission system, job queue or storage service. [Plan index](../../README.md)
- **Visual infrastructure:** terrain, imagery/vector derivatives, Atlas handoff, local navigation, optional context, frozen membership and bounded viewer performance are covered by V01–V08. The existing Three.js viewer is an inspection consumer. [Implementation record](../../IMPLEMENTATION_PROGRESS.md)
- **Environmental inputs:** land cover, modeled ground, infrastructure dimensions, radar, weather, hazards, coastal and moving/orbital context have implemented capabilities or bounded pilots. Their individual restrictions remain part of the data. [Riyadh enrichment](../../../docs/implementation/RIYADH_ENVIRONMENT_ENRICHMENT.md) · [Observation pilot](../../../docs/implementation/RIYADH_OBSERVATION_PILOT.md)
- **Historical simulation building blocks:** explicit coordinates, virtual time, frozen versions, seeded abstract motion, isolated runtime workers, protected truth, recording and evaluation already exist. Preserve these as future reuse candidates and regression evidence. Their recorded acceptance applies to their finite historical profiles. [Spatial contracts](../../../internal/simulation/spatial.go) · [Frozen runs](../../../internal/scenarios/frozen.go) · [Historical delivery](../../../docs/implementation/HISTORICAL_DELIVERY.md)

The review is based on source and recorded evidence. No fresh installed qualification, physical survey, simulation run or full test suite was performed for this assessment.

## Existing acceptance work to finish

The implementation of the plans and acceptance of the installed workflow are separate milestones. The current index still records these open items:

1. **F09 installed observation:** complete a fresh 24-hour observation bound to the current API process generation, with the required worker restart and storage trend. Earlier interrupted observations remain partial evidence.
2. **F03 release and receiving:** record the responsible reviewers' actual decisions and an independent Riyadh receiving walkthrough. Record permitted portable issuance and frozen handoff separately; successful private viewing does not establish those permissions.
3. **Shared rollout:** qualify migration 59 on shared staging in the documented sequence after the pinned observation, and commission selected enrichment/pilot capabilities against the installed candidate. Disposable qualification is not shared installation evidence.
4. **Target-specific acceptance:** retain physical mobile, separate-host commissioning and external alert delivery as explicit open targets where required. They need not block a desktop-only, local preparation profile; they must not be presented as qualified.

Use the existing [acceptance sequence](../../README.md#next-acceptance-steps), [F09 procedure](../../../docs/implementation/COASTAL_MOVING_AND_READINESS.md#f09--installed-readiness), [release recovery procedure](../../../docs/implementation/RIYADH_RELEASE_RECOVERY.md) and [rollout evidence](../../../docs/implementation/GEOGRAPHIC_SOURCE_QUALITY_AND_STORAGE.md). These are unfinished acceptance items within existing plans, not new infrastructure projects. They can progress alongside new preparation work using appropriately permitted fixtures.

## Findings that change the preparation priorities

### Physical suitability needs its own evidence

Riyadh has strong numerical coordinate and byte-reproducibility evidence, while its physical accuracy remains `UNKNOWN`. Its retained building footprints have no usable building-height tags; road widths are also unknown. GHS-OBAT adds modeled centroid heights separately and does not establish heights for those footprints. The modeled ground and DSM share some ancestry, so their agreement is not independent validation. [Riyadh results](../../IMPLEMENTATION_PROGRESS.md#riyadh-reference-release) · [Dimension and terrain evidence](../../../docs/implementation/RIYADH_ENVIRONMENT_ENRICHMENT.md)

The current release validator explicitly requires `physicalAccuracy == "UNKNOWN"`. Future physical qualification therefore needs a versioned assessment contract or compatible schema evolution. It cannot be delivered by replacing an immutable manifest field with an unsupported accuracy claim. [Release validation](../../../internal/geoenv/releases.go) · [Structured quality](../../../internal/geoenv/profile_quality.go)

### Display preparation does not establish computational geometry

The retained roads, buildings and masks are a useful canonical starting point. Their display derivatives include draping, reductions and offsets. Those choices must remain visible to another application. Road connectivity, grade-separated crossings, occupied building volumes and material properties need their own qualification. D08 preserves source dimensions and separate modeled context without establishing those additional capabilities. [D08 evidence](../../../docs/implementation/RIYADH_ENVIRONMENT_ENRICHMENT.md) · [Display validation](../../../internal/geoenv/display_layers.go)

### The current handoff is a strong foundation with a bounded consumer proof

V07 freezes geographic/display identities and selected observations through `emad.geographic-context.v1`, with expiry and current-rights checks. The standalone reader verifies the handoff and local display dependencies. Historical Studio still loads its older context bundle and separate environment/model definitions. The reviewed paths do not demonstrate that a new Riyadh geographic handoff can be consumed directly by Studio. [Geographic handoff](../../../internal/contextpackage/geographic.go) · [Independent verifier](../../../scripts/ops/verify_geographic_handoff.py) · [Studio input loading](../../../internal/simscene/service.go)

The preparation task is to finish an independent, documented data-consumption path. A simulator adapter belongs to the later phase. Do not make geographic acceptance depend on Studio.

### Historical limits cannot become future scale promises

Historical scene validation limits scenes to 16 actors, a 10 km radius and 4,096 context-terrain samples. The Riyadh source release contains 46,656 valid surface samples. These are different representations and profiles, so the counts do not prove a failed import; they do show that compatibility must be measured explicitly. The historical world model follows static/waypoint paths without forces, collision or terrain following. Recorded checkpoints restore selected recorded output, not an executable simulator state. [Scene validation](../../../internal/simscene/domain.go) · [World model](../../../internal/simruntime/world.go) · [Recording guarantee](../../../internal/recordings/worker.go)

### Exact versions do not establish a consistent historical world

Riyadh combines modeled-ground source periods of 2006–2015, modeled building context for 2020, land cover for 2021, imagery from 2025 and OSM retained in 2026. Keeping these identities frozen is necessary. It does not establish that all layers describe the same date. A future consumer needs a stated time interpretation and a way to reject incompatible combinations. [Source editions](../../../docs/implementation/RIYADH_ENVIRONMENT_ENRICHMENT.md) · [Reference inputs](../../IMPLEMENTATION_PROGRESS.md#riyadh-reference-release)

## Proposed preparation workstreams

All P01–P08 entries below are recommendations. “Core” means broadly reusable preparation; “conditional” means implementation depth depends on an identified consumer. No new capability is marked accepted by this review.

### P01 — Consumer requirements and capability checks

**Priority:** Core, first. **Extend:** F02, F09, V07 and the existing profile/quality contracts. **Owner:** Product and data-contract engineering with geospatial quality review.

Add a versioned consumer requirement profile that states intended analytical use, AOI, time interpretation, required layers, units, accepted source quality, missing-data policy, permitted operation and resource envelope. Match it against an exact release and any later quality assessments. Report `supported`, `unsupported` or `unqualified`, with reasons and evidence; these are proposed eligibility outcomes, separate from release and job states.

The result must distinguish data presence, numerical correctness, source claims, independent physical validation, coverage, temporal compatibility and current permission. An unknown building height must remain unknown. “Layer exists” must not imply “suitable for movement or visibility.” Do not invent universal metre-level tolerances before a consumer requires them.

**Deliverable:** Requirements schema, compatibility/eligibility report and CLI/API preflight using existing identities. **Exit proof:** the same release can pass declared visual inspection while correctly failing a request requiring independently verified building heights or incompatible dates. A permission change invalidates the relevant current delivery decision without rewriting historical evidence.

### P02 — Canonical feature semantics and topology

**Priority:** Core metadata and diagnostics; detailed network preparation conditional. **Extend:** D01, D08, F02 and canonical vectors. **Owner:** Geospatial data engineering.

Preserve source edition and feature ID through canonical geometry, display simplification and later derivatives. Normalize evidenced road direction, junction connectivity, bridge/tunnel/layer separation, restrictions, building footprints/holes and dimension origin. Distinguish an area clipped at the package boundary from a true dead end. Keep uncertain geometry and relationships inspectable.

Prepare topology diagnostics and a versioned static network representation where source evidence supports it. A rendered intersection does not by itself establish a traversable junction. Land-cover labels also do not establish friction, load-bearing strength or a traversable surface. Reserve provenance-bearing property links for future sources; leave absent physical values unresolved.

**Deliverable:** Semantic dictionary, stable feature mapping, coverage and topology report, and a bounded network dataset when justified. **Exit proof:** bridge crossings, one-way links, disconnected segments, polygon holes and unknown dimensions survive independent reads. No invented connection or dimension is needed to make the dataset pass. Route execution and actor navigation remain later work.

### P03 — Reusable environment queries

**Priority:** Core native-data access; specialized geometry queries conditional. **Extend:** F02, V02, D02 and existing spatial code. **Owner:** Geospatial/backend engineering.

Expose a small, versioned library/CLI for coordinate conversion, native surface/ground sampling, land-cover lookup, exact feature lookup and bounded spatial selection. Keep analytical reads separate from display offsets and render simplification. Pin the release, member, transform and interpolation method in each answer; include datum, units, coverage and quality status. Distinguish “outside the package,” “nodata,” “unknown physical value” and “permission denied.”

Document the mapping among geodetic, ECEF, local ENU and model axes, local-origin identity, orientation and supported area limits. Existing transform limits and scene limits differ; document the applicable limit for each consumer. Derive slopes only from a declared suitable ground representation with an explicit method. Additional ray/volume queries require qualified geometry and their own accuracy contract.

**Deliverable:** Shared read interfaces and a bounded offline query tool, reusable by map analysis and future applications. **Exit proof:** independent expected values match at control points, grid seams, edges and nodata holes; unsupported vertical conversions fail clearly. Backend and offline clients return equivalent declared results. A future runtime can load data locally without depending on a remote API call for every update.

### P04 — Reference area and independent validation corpus

**Priority:** Core, first; external data acquisition depends on availability and rights. **Extend:** F02, D02, D08 and existing fixtures. **Owner:** Geodesy/quality engineering and data stewardship.

Select a small Riyadh subarea where independent terrain controls, dimensions or other needed measurements can be acquired. Record positional and vertical uncertainty, acquisition time, spatial coverage and how controls were kept independent of source/model generation. Evaluate geometry, datum conversion, sample interpretation and any claimed physical accuracy separately. Qualify only the uses supported by the controls; a surveyed citywide digital twin is not a universal prerequisite.

Add synthetic reference fixtures for flat and sloped terrain, holes, tile seams, an elevated crossing, narrow gaps and mixed units. Give these analytically known expected values. Synthetic fixtures establish implementation correctness; real independent controls establish physical fitness. Keep the two evidence classes separate.

Store new assessments as immutable, versioned records bound to release/member digests, scope, method, statistic, units, uncertainty and reviewer decision. Preserve previous `UNKNOWN` manifests. If independent control data cannot be obtained, retain an explicit limitation and qualify only uses that accept it.

**Deliverable:** Small reference package, control inventory, expected results and repeatable validation command. **Exit proof:** another reader reproduces the checks, and the suitability report neither generalizes local checks to all Riyadh nor treats correlated datasets as independent evidence.

### P05 — Independent consumer kit and complete dependency handoff

**Priority:** Core, first. **Extend:** F03, F05, F06, F10, V05 and V07. **Owner:** Data-contract and tooling engineering.

Package the existing verification tools behind a documented consumer path: discover an eligible release, obtain the authorized dependency set, verify it, enumerate native layers, resolve feature IDs, read samples and report unsupported capabilities. Compose the prepared-ZIP verifier with the geographic handoff verifier; a manifest/display check alone is not the entire native-data read proof.

Specify the relationship among the release, archive, display derivatives, observation handoff, profile, quality assessment and recipes. Every required transitive dependency must be present or explicitly unavailable. Retain compatibility/version rules, exact digests, masks, terms and expiry. Immutable membership does not mean perpetual entitlement or guaranteed indefinite retention. Define the permitted offline-use window, retention/pinning behavior, updates and required deletion/expiry handling; disconnected clients cannot be promised instant online revocation.

Start with a documented language-neutral contract and one reference CLI/library. Add another language binding when an actual consumer needs it. Use a fresh independent client with no database access and no implicit viewer cache. Keep simulation import outside this deliverable.

**Deliverable:** Consumer guide, executable example, dependency manifest and compatibility fixtures. **Exit proof:** a permitted Riyadh release, or an explicitly labeled permitted fixture while Riyadh authorization is pending, opens in a clean offline process; native values and feature IDs match; altered/missing dependencies and unsupported schemas fail; reopening retains the exact revisions. Final Riyadh acceptance still requires the actual source permission and receiver.

### P06 — Time-aware environmental fields

**Priority:** Extend existing fields for demonstrated analytical needs; additional physical variables conditional. **Extend:** D03–D07 and V07. **Owner:** Environmental data engineering.

Provide consistent native sampling and metadata for time-indexed fields: variable, unit, grid, vertical coordinate, valid time, issue time where applicable, acquisition/model period, source edition and masks. Keep observations, reanalysis, forecasts and modeled context identifiable. Add interpolation only as a versioned derivative with documented validity limits and boundary behavior.

For the current pressure-field pilot, pressure levels must not silently become geometric heights. Mixed source epochs need a compatibility result. Historical moving gaps must remain gaps rather than invented trajectories. Wind, visibility, precipitation, tides or currents should be acquired only after their consumer specifies area, time, levels, resolution and acceptable error.

**Deliverable:** Common field-read contract, native-value reader and temporal compatibility report. **Exit proof:** native samples survive a round trip; outside-window/level requests fail explicitly; interpolation never extrapolates silently; availability cutoff and event time retain their separate meanings. Modeling the effects on vehicles or sensors stays in the simulation phase.

### P07 — Static model metadata and derivative qualification

**Priority:** Core inventory and contracts; additional derivatives conditional. **Extend:** V01, V02, D08, F10 and the historical asset registry. **Owner:** Asset pipeline and geospatial engineering.

Reuse the existing asset registry's rights, units, axes, hashes, validator and engine-variant lineage. Audit model scale, pivot/origin, bounds, semantic feature links and stated purpose. Separate render geometry, potential query/collision geometry, and any physical property set into independently identified representations. A visible model or rendering material is not evidence of physical mass, inertia, friction or sensor response.

Define derivative requests by source members, recipe/tool versions, intended use and measured error. Preserve originals and feature identity across simplification. Prepare collision meshes, navigation representations, articulation or specialist material data only when a named consumer establishes their semantics and tolerances. These are not blanket requirements for geographic preparation.

**Deliverable:** Asset inventory and compatibility report, plus a qualified derivative profile for each activated use. **Exit proof:** model-axis and metre-scale controls pass; original and derivative remain separately addressable; unsupported physical properties stay unknown; a consumer cannot accidentally select a display-offset surface as physical ground.

### P08 — Consumer workloads, operational limits and commissioning

**Priority:** Core for the declared preparation target. **Extend:** F04–F10 and V08. **Owner:** Performance/operations and quality engineering.

Add workload descriptors for geographic reads and offline consumption: AOI size, native samples, features, required accuracy, bytes, query mix, concurrency, cache state and hardware. Benchmark preparation, verification, cold load, repeated queries, peak memory, scratch space and artifact growth separately from viewer frame time. Preserve current Riyadh and second-frame fixtures; introduce larger workloads only for an actual coverage or density need.

Use the results to set CPU/RAM/storage/network budgets and decide whether spatial indexes, chunking, additional levels of detail or another delivery format are justified. GPU purchases, a new renderer, Kubernetes or a new event broker do not follow from the existence of a simulation roadmap. Future actor counts, tick rates, real-time factors, event throughput and runtime concurrency require later workload benchmarks.

Extend existing commissioning with the exact consumer build/configuration, compatibility versions, source rights, dependency retention, reproducible environment, access boundaries, quota behavior and restore evidence. Include corrupt inputs, cancellation/retry, exhausted resources and denied access. Run only the regression checks affected by the eventual implementation; this document does not claim new test passes.

**Deliverable:** Versioned workload pack, measured capacity report and consumer commissioning record. **Exit proof:** the independent consumer completes its agreed workload within declared limits on named hardware, degrades explicitly under overload, and still reads the same permitted data after paired restore and compatible upgrade.

## Delivery order

1. **Close existing acceptance in parallel.** Continue F03/F09 and the documented migration/rollout sequence without changing the workload of an active observation. Use independently permitted fixtures for new tooling.
2. **Establish the consumer contract.** Deliver P01, the P04 control/fixture inventory, and a minimal P05 read/verify example. Start with one bounded Riyadh area and a second frame for coordinate regressions. Record unsupported uses immediately.
3. **Make the data queryable.** Implement the relevant P02 semantics and P03 query interfaces. Integrate their independent controls and native-data consumption into P05. This is the principal new infrastructure increment.
4. **Activate justified capabilities.** Add P06 fields or P07 derivatives only where the requirement profile calls for them. Further sources, geographic expansion and specialist formats follow those requirements.
5. **Qualify the handoff.** Complete P08 and the independent receiving workflow against the actual released candidate. Publish measured limits, known gaps, owners and evidence with the consumer kit.

P01 and P04 discovery can begin together. P05 can verify existing native data before additional queries are built. P02/P03 acceptance depends on the relevant source semantics and control fixtures. P08 measures each delivered increment and provides the final assembled acceptance. This sequence is not a calendar estimate; source access, independent controls and the eventual workload determine duration.

## Decisions reserved for the simulation phase

Carry the following decision list into the later discussion, alongside the existing historical code and measured preparation results:

- **Purpose and fidelity:** visual rehearsal, abstract exercise, terrain-constrained movement, environmental modeling or another concrete question; accepted error and which output will be validated.
- **Runtime ownership and time:** authoritative state, discrete-event versus fixed/variable-step execution, clock mapping, pause/speed behavior, scheduling, deterministic guarantees and supported platforms. A seed alone does not guarantee reproducibility across different engines or builds.
- **Models and behaviors:** required actors, physical dynamics, collision response, navigation, sensors and environmental effects. Choose what is needed from the use case and evidence, then select an engine or adapter.
- **Execution and collaboration:** local/headless/server execution, concurrent runs, participant roles, resource admission, isolation, cancellation, failure recovery and truth/observation separation. Preserve the existing access model when assessing reuse.
- **Results and recovery:** recorded playback, re-simulation, executable checkpoint/resume, run branching, result retention, experiment comparison and evaluation criteria. These require separate guarantees; historical output replay does not establish executable checkpoint recovery.
- **Interoperability:** identify the specific external consumer and contract before implementing federation or specialist adapters. Avoid making every possible interoperability format a preparation prerequisite.

The infrastructure needed beyond the common core varies by use. Ground movement may need qualified connected roads, ground elevations and clearances. Atmospheric use may need height-resolved time-varying fields. Maritime use may need appropriate bathymetry, vertical references and time-dependent water data. Visibility/perception use may need qualified obstruction geometry and properties. The current pilots are starting points for these branches, not validation of the future physical models.

## Preparation exit criteria

The preparation phase is complete for a declared consumer profile when:

1. An eligible released package has exact native/display identities, permitted use and an independent receiving record; applicable installed gates are complete.
2. A fresh independent application verifies and reads the complete permitted package offline without Studio or a database connection.
3. Coordinates, units, sample semantics, feature identities, spatial boundaries and time interpretations are explicit and independently checked.
4. Required quality is demonstrated for the selected area, or the profile explicitly accepts its unresolved limitations. Unknown physical properties are never filled merely to make the import succeed.
5. Required queries and optional activated representations pass reference checks, including boundary/missing-data cases.
6. Versions, rights, expiry, retention and compatibility are enforceable and documented; corruption, unavailable dependencies and denied access are clear outcomes.
7. Consumer performance and recovery are measured on the declared workload and hardware, with evidence linked to the exact installed and packaged identities.
8. The later simulation team receives the consumer kit, reference fixtures, measured limits, capability gaps and the decision list above.

These criteria qualify reusable inputs and the handoff. Simulation execution and physical model validation have their own later acceptance.

## Primary reference checks

These references help define interchange boundaries; they are not new technology selections.

- Khronos defines glTF as a runtime asset-delivery format and leaves runtime behavior to the application. Keep the GLB visual pipeline while qualifying computational representations separately. [glTF 2.0 specification](https://registry.khronos.org/glTF/specs/2.0/glTF-2.0.html)
- OGC GeoPose covers geographically referenced position and orientation. Use it as a mapping reference if an external consumer needs pose exchange; retain the existing tested frame contract until a concrete integration requires a change. [OGC GeoPose](https://www.ogc.org/standards/geopose/)
- ASAM OpenDRIVE describes static road networks and distinguishes them from dynamic scenario content. Assess it when a road-simulation consumer needs that contract; ordinary road display is insufficient evidence of such compatibility. [ASAM OpenDRIVE](https://www.asam.net/standards/detail/opendrive/)
