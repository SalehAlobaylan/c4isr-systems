# Environment and 3D infrastructure plans

These 26 system plans turn the Environment foundation installation evidence and the 3D infrastructure review into implementation work. Each defines the outcome, existing code to extend, data and API behavior, failure handling, acceptance checks and delivery evidence.

Prepared on 12 November 2026 against repository baseline `b3c0962` and the current working tree. World Monitor references are pinned to `1eb90a98bda21fc8c4f4ce070c3ef7e25b3873cc`. Each plan records its implementation status and acceptance evidence.

Read the [implementation sequence and shared contracts](IMPLEMENTATION_SEQUENCE.md) before selecting a system. The [point-by-point coverage map](SOURCE_COVERAGE.md) traces every substantive item in the two source reviews to these plans.

See the [implementation record](IMPLEMENTATION_PROGRESS.md) for worktree changes, verified behavior and the remaining acceptance work.

## Geographic priorities

1. **Riyadh first for detailed preparation and 3D acceptance.** Define a bounded Riyadh study area, retain its exact source inputs and prepare a versioned reference environment. Use it to qualify terrain, imagery and vector display, feature inspection and the return to Atlas.
2. **Saudi Arabia first for national coverage.** Use the retained country and first-order region references for discovery and show source and layer coverage by area. Expand detailed environments through bounded Saudi study areas selected by user need and source readiness.
3. **The wider Middle East for regional coverage.** Use the existing 17-country working profile for navigation, document coverage and gaps by source, and add detailed study areas where a workflow and source coverage justify them. [Middle East working profile](../scripts/data/region_profiles/middle-east-v1.json)

The first **Riyadh reference release** is staged. Private staging serves its terrain, imagery, feature layers, local streets and scoped observation context. Riyadh passed the desktop and compact workstation viewer budgets with external requests blocked. Al Ain passed the second local-frame check. Independent release review and source permission for Riyadh portable issuance and frozen handoff remain open. [Riyadh evidence](IMPLEMENTATION_PROGRESS.md#riyadh-reference-release) · [Context and viewer qualification](IMPLEMENTATION_PROGRESS.md#context-handoff-and-viewer-qualification)

## Next acceptance steps

All 26 plans have implementation evidence, a qualified fixture or a bounded pilot. Accept the installed Riyadh workflow before expanding detailed coverage.

1. **Complete F09 on the pinned installation.** Verify the 24-hour metadata observation, worker restart and storage trend together. The latest migration 58 run began at 23:06 Riyadh time on 14 November 2026 after host recovery. Use the readiness ledger to record which gates passed for that exact API process generation. [Current observation and ledger procedure](../docs/implementation/COASTAL_MOVING_AND_READINESS.md#f09--installed-readiness)
2. **Record F03 decisions and the Riyadh walkthrough.** Authorized reviewers verify the three pinned releases and record their decisions. An independent receiver discovers the released Riyadh package, verifies the ZIP and display identities, and records its usable layers and limits. These steps can run alongside the observation. [Review and receiver procedure](../docs/implementation/RIYADH_RELEASE_RECOVERY.md#f03-release-review-and-receiving)
3. **Qualify migration 59 on shared staging.** After the pinned window closes, take a current paired backup, roll out the candidate and recheck the installed hashes, queue, storage and viewer. Bind the new acceptance record to the deployed images and configuration. Record any remaining portable viewing, frozen context or physical mobile checks against their own prerequisites. [Migration 59 rehearsal](../docs/implementation/GEOGRAPHIC_SOURCE_QUALITY_AND_STORAGE.md#rollout-and-evidence) · [Viewer acceptance](3d-infrastructure/08-viewer-performance-and-acceptance.md)

The next detailed study area is selected within Saudi Arabia by intended use and source readiness. The wider Middle East remains the regional discovery and coverage scope. [Implementation sequence](IMPLEMENTATION_SEQUENCE.md)

## Environment foundation

- [F01 — Source admission, lineage and retention](environment-foundation/01-source-admission-and-lineage.md).
- [F02 — Geographic profiles, coordinates and quality](environment-foundation/02-geographic-profiles-and-quality.md).
- [F03 — Release review and independent handoff](environment-foundation/03-release-review-and-handoff.md).
- [F04 — Durable preparation workers and execution transport](environment-foundation/04-durable-workers-and-transport.md).
- [F05 — Artifact delivery, integrity and storage lifecycle](environment-foundation/05-artifact-delivery-and-lifecycle.md).
- [F06 — Portable verification and reproducible rebuild](environment-foundation/06-portable-verification-and-rebuild.md).
- [F07 — Paired backup, restore and forward recovery](environment-foundation/07-backup-restore-and-forward-recovery.md).
- [F08 — Operations, metrics, alerts and commissioning](environment-foundation/08-operations-metrics-and-commissioning.md).
- [F09 — Readiness evidence and operational qualification](environment-foundation/09-readiness-and-operational-qualification.md).
- [F10 — Contracts, CI and upgrade compatibility](environment-foundation/10-contracts-ci-and-upgrade-compatibility.md).

F01, F02 and F05 passed isolated migration 59 qualification. Riyadh source impact, profile evidence, resumable downloads and purge retries are ready for staging rollout after the F09 observation. The rebuilt Riyadh package matched all ten installed member hashes. [Source, quality and storage qualification](../docs/implementation/GEOGRAPHIC_SOURCE_QUALITY_AND_STORAGE.md)

## 3D infrastructure

- [V01 — Browser asset delivery and derivative publication](3d-infrastructure/01-browser-assets-and-publication.md).
- [V02 — Terrain mesh generation and coordinate rendering](3d-infrastructure/02-terrain-and-coordinate-rendering.md).
- [V03 — Imagery and vector display preparation](3d-infrastructure/03-imagery-and-vector-display.md).
- [V04 — Shared layer catalog and Atlas-to-3D handoff](3d-infrastructure/04-layer-catalog-and-atlas-handoff.md).
- [V05 — Local basemaps and offline viewing packages](3d-infrastructure/05-local-basemaps-and-offline-packages.md).
- [V06 — Context overlays, freshness and coverage](3d-infrastructure/06-context-overlays-and-freshness.md).
- [V07 — Frozen environment and observation handoff](3d-infrastructure/07-frozen-environment-context-handoff.md).
- [V08 — Viewer performance, accessibility and acceptance](3d-infrastructure/08-viewer-performance-and-acceptance.md).

V01–V03 now support durable terrain and overlay generation, native-grid seams, and terrain inspection and handoff to Atlas. The isolated Riyadh run reproduced the five installed artifact hashes. [Riyadh durable geographic preview](../docs/implementation/RIYADH_DURABLE_GEOGRAPHIC_PREVIEW.md)

## Environment data

- [D01 — Categorical land-cover data](environment-data/01-land-cover.md).
- [D02 — Bare-earth terrain and surface comparison](environment-data/02-bare-earth-terrain.md).
- [D03 — Radar imagery acquisition and display](environment-data/03-radar-imagery.md).
- [D04 — Natural hazards and official event context](environment-data/04-natural-hazards-and-official-events.md).
- [D05 — Historical weather fields and station context](environment-data/05-weather-fields.md).
- [D06 — Coastal geography and bathymetric context](environment-data/06-coastal-and-bathymetric-data.md).
- [D07 — Moving observations and orbital context](environment-data/07-moving-and-orbital-context.md).
- [D08 — Infrastructure features and dimensional evidence](environment-data/08-infrastructure-features-and-dimensions.md).

D01, D02 and D08 now have a versioned Riyadh preparation profile and 3D inspection workflow. The profile retains WorldCover classes, modeled bare-earth terrain and infrastructure dimensions with their source limitations. Independent release review and local physical accuracy remain open. [Riyadh enrichment evidence and rollout](../docs/implementation/RIYADH_ENVIRONMENT_ENRICHMENT.md) Apply the same coverage and quality gates to subsequent Saudi areas and then to other Middle Eastern areas.

D03 radar, D04 historical flood reports and D05 pressure fields have bounded pilots. [Observation sources, qualification and rollout](../docs/implementation/RIYADH_OBSERVATION_PILOT.md)

D06 passed a bounded Jubail Gulf preparation, viewer and offline rebuild in disposable staging. D07 added historical moving revision reads and handoff verification using owned records, plus a private NOAA-20 orbital sample. F09 records exact installed readiness gates. [D06, D07 and F09 evidence](../docs/implementation/COASTAL_MOVING_AND_READINESS.md)

F03 added exact archive verification and a receiver workflow. F06 verified compatibility across seven packages. F07 restored the current installed Riyadh workload and checked the previous API and forward recovery in an isolated stack. The 24-hour observation, installed release decisions and independent receiving walkthrough remain open. [Riyadh release, verification and recovery](../docs/implementation/RIYADH_RELEASE_RECOVERY.md)

F04 and F08 run on shared staging with two database-free workers, one display-only worker and authenticated Prometheus. F10 verified unchanged package and display bytes across migration 58. [Transport, operations and compatibility](../docs/implementation/GEOGRAPHIC_OPERATIONS_AND_UPGRADE.md)

## Architectural decisions

Implement these systems as bounded modules and workflows in the existing Go API, PostgreSQL/PostGIS, object storage, preparation workers and React UI. Reuse current access scopes, catalog, acquisition, durable jobs and audit. A separate plan represents an independently reviewable capability and does not imply a new network service.

Use MapLibre for Atlas and the existing Three.js stack for the first local geographic preview. Begin with bounded GLB/texture derivatives. Evaluate deck.gl, 3D Tiles or a different geospatial renderer only against a measured workload and documented consumer requirements.

Preserve original native data and geographic manifests. Derived meshes, display textures, basemaps and observation bundles have their own identities and explicit parent links. Geographic preparation and data inspection are the delivery scope. Simulation execution, physical models and behavior remain later consumer responsibilities.

The plans use real reviewer decisions where the installed release workflow requires them. Staged technical work can proceed under existing authorized roles. Actual human verification and release must be recorded by the responsible participant.

## Regression evidence

The staged Jeddah, Al Ain and Avignon packages provide regression evidence for preparation, recovery and rendering. Riyadh is the detailed viewer performance candidate. Its staging test records frame time and resource counts for desktop and compact workstation profiles. [Terrain measurements](IMPLEMENTATION_PROGRESS.md#native-grid-terrain-results) · [Viewer measurements](IMPLEMENTATION_PROGRESS.md#context-handoff-and-viewer-qualification)

## Source documents

- [Environment foundation installation evidence](../docs/implementation/ENVIRONMENT_FOUNDATION_EVIDENCE_2026_11_12.md).
- [Original environment implementation plan](../docs/implementation/ENVIRONMENT_FOUNDATION_IMPLEMENTATION_PLAN_2026_11_11.md).
- [Environment operations](../docs/implementation/ENVIRONMENT_LIBRARY_OPERATIONS.md).
- [3D infrastructure review](C:/Users/saleh/.cursor/projects/c-Users-saleh-Desktop-c4isr-systems/canvases/EMAD-world-monitor-3d-infrastructure.canvas.tsx).
- [Reviewed World Monitor commit](https://github.com/koala73/worldmonitor/commit/1eb90a98bda21fc8c4f4ce070c3ef7e25b3873cc).

The repository plans are self-contained implementation documents. The canvas link provides the original review on this workstation.

## Completion records

Each system’s evidence should bind the implementation commit, build/configuration digests, migrations, input releases and source revisions, commands and results, failed/skipped checks, participant decisions where applicable and evidence artifact hashes. Store credentials and protected raw payloads privately. Link the evidence from the corresponding plan when its status changes.

Track systems as proposed, implementing, verification pending, accepted, or blocked with a concrete reason. Acceptance requires the named system tests and the shared policy and compatibility checks. Optional source unavailability leaves that capability explicitly unsupported.
