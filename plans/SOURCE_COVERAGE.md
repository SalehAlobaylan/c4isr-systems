# Source coverage

[Plan index](README.md)

This mapping covers the substantive findings, features and acceptance points in both named reviews. Repeated statements map to the same owning system so implementation has one authoritative contract for that capability. Related system links identify the producer, consumer or verification responsibility.

## Environment foundation installation evidence

The source is the [12 November installation report](../docs/implementation/ENVIRONMENT_FOUNDATION_EVIDENCE_2026_11_12.md). Existing successful checks are baseline evidence. Each linked plan says how to preserve the behavior or close the remaining gap.

- **Installed Windows and Linux scopes, role-specific credentials, two workers and source-policy expiry** — [F01](environment-foundation/01-source-admission-and-lineage.md), [F04](environment-foundation/04-durable-workers-and-transport.md), [F08](environment-foundation/08-operations-metrics-and-commissioning.md).
- **Seventeen-country navigation and retained Natural Earth originals** — [F01](environment-foundation/01-source-admission-and-lineage.md), [F07](environment-foundation/07-backup-restore-and-forward-recovery.md), [V05](3d-infrastructure/05-local-basemaps-and-offline-packages.md).
- **Copernicus DSM, EGM2008, Sentinel-2 and OSM provenance, dates, resolution and UNKNOWN physical accuracy** — [F01](environment-foundation/01-source-admission-and-lineage.md), [F02](environment-foundation/02-geographic-profiles-and-quality.md), [V02](3d-infrastructure/02-terrain-and-coordinate-rendering.md), [V03](3d-infrastructure/03-imagery-and-vector-display.md).
- **Three named staging releases, full manifest digests and prepared package sizes** — [F03](environment-foundation/03-release-review-and-handoff.md), [F09](environment-foundation/09-readiness-and-operational-qualification.md), [V01](3d-infrastructure/01-browser-assets-and-publication.md).
- **First-attempt builds, authorized 206/full downloads and distinct Windows/Linux toolchains** — [F04](environment-foundation/04-durable-workers-and-transport.md), [F05](environment-foundation/05-artifact-delivery-and-lifecycle.md), [F06](environment-foundation/06-portable-verification-and-rebuild.md).
- **Byte-identical Linux reconstruction from frozen inputs with provider networking disabled** — [F06](environment-foundation/06-portable-verification-and-rebuild.md).
- **Paired restore of database, security definitions and object bytes, pending-build recovery and separately retained navigation originals** — [F07](environment-foundation/07-backup-restore-and-forward-recovery.md), [F01](environment-foundation/01-source-admission-and-lineage.md).
- **Two concurrent preparations, queue pickup, killed worker, LEASE_EXPIRED and single publication** — [F04](environment-foundation/04-durable-workers-and-transport.md), [F08](environment-foundation/08-operations-metrics-and-commissioning.md), [F09](environment-foundation/09-readiness-and-operational-qualification.md).
- **Five readers over 10,000 metadata rows and 100,000-feature processing measurements** — [F09](environment-foundation/09-readiness-and-operational-qualification.md), [V08](3d-infrastructure/08-viewer-performance-and-acceptance.md).
- **Desktop/390px/740px browser review, offline-matched download and 27 relevant browser regressions** — [F03](environment-foundation/03-release-review-and-handoff.md), [F10](environment-foundation/10-contracts-ci-and-upgrade-compatibility.md), [V04](3d-infrastructure/04-layer-catalog-and-atlas-handoff.md), [V08](3d-infrastructure/08-viewer-performance-and-acceptance.md).
- **Enabled PostGIS, Go/vet/race, Python/frontend checks and the clean-checkout ignore defect** — [F10](environment-foundation/10-contracts-ci-and-upgrade-compatibility.md).
- **Final review metadata, interrupted transfer behavior, rights revocation and current-definition filtering** — [F03](environment-foundation/03-release-review-and-handoff.md), [F05](environment-foundation/05-artifact-delivery-and-lifecycle.md), [F10](environment-foundation/10-contracts-ci-and-upgrade-compatibility.md).
- **Human verification of exact STAGED manifests and independent human workflow still open** — [F03](environment-foundation/03-release-review-and-handoff.md), [F09](environment-foundation/09-readiness-and-operational-qualification.md).
- **24-hour observation, gap detection, real elapsed time and preserved initial run** — [F09](environment-foundation/09-readiness-and-operational-qualification.md).
- **Separate HTTP worker transport remains open** — [F04](environment-foundation/04-durable-workers-and-transport.md).
- **Dedicated environment metrics remain open** — [F08](environment-foundation/08-operations-metrics-and-commissioning.md).
- **Separate-host staging requires an actual deployment target and scoped credentials** — [F08](environment-foundation/08-operations-metrics-and-commissioning.md).
- **Repository-wide OpenAPI lint errors and warnings** — [F10](environment-foundation/10-contracts-ci-and-upgrade-compatibility.md).

## 3D infrastructure review

The source is the [expanded 3D infrastructure review](C:/Users/saleh/.cursor/projects/c-Users-saleh-Desktop-c4isr-systems/canvases/EMAD-world-monitor-3d-infrastructure.canvas.tsx). The titles below preserve its point names. Its World Monitor references are pinned to the reviewed commit.

- **Shared layer catalog and portable map selection** — [V04](3d-infrastructure/04-layer-catalog-and-atlas-handoff.md).
- **Self-hosted navigation and small startup payloads** — [V01](3d-infrastructure/01-browser-assets-and-publication.md), [V05](3d-infrastructure/05-local-basemaps-and-offline-packages.md), [V08](3d-infrastructure/08-viewer-performance-and-acceptance.md).
- **Coverage and freshness visible on each layer** — [V06](3d-infrastructure/06-context-overlays-and-freshness.md).
- **Display derivatives built from exact releases** — [V01](3d-infrastructure/01-browser-assets-and-publication.md), [V02](3d-infrastructure/02-terrain-and-coordinate-rendering.md), [V03](3d-infrastructure/03-imagery-and-vector-display.md).
- **Frozen data inputs for replay** — [V07](3d-infrastructure/07-frozen-environment-context-handoff.md).
- **Viewport budgets before new rendering dependencies** — [V08](3d-infrastructure/08-viewer-performance-and-acceptance.md).
- **Terrain, imagery, roads and buildings** — [F02](environment-foundation/02-geographic-profiles-and-quality.md), [V02](3d-infrastructure/02-terrain-and-coordinate-rendering.md), [V03](3d-infrastructure/03-imagery-and-vector-display.md), [D02](environment-data/02-bare-earth-terrain.md), [D08](environment-data/08-infrastructure-features-and-dimensions.md).
- **Weather and natural hazards** — [D04](environment-data/04-natural-hazards-and-official-events.md), [D05](environment-data/05-weather-fields.md), [V06](3d-infrastructure/06-context-overlays-and-freshness.md).
- **Traffic and orbital context** — [D07](environment-data/07-moving-and-orbital-context.md).
- **Infrastructure and cartographic features** — [D08](environment-data/08-infrastructure-features-and-dimensions.md), [V05](3d-infrastructure/05-local-basemaps-and-offline-packages.md).
- **Release an exact environment before consumer handoff** — [F03](environment-foundation/03-release-review-and-handoff.md), [V07](3d-infrastructure/07-frozen-environment-context-handoff.md).
- **Prove a read-only Atlas-to-3D handoff** — [V04](3d-infrastructure/04-layer-catalog-and-atlas-handoff.md), [V08](3d-infrastructure/08-viewer-performance-and-acceptance.md).
- **Measure the viewer and exercise an unavailable feed** — [V06](3d-infrastructure/06-context-overlays-and-freshness.md), [V08](3d-infrastructure/08-viewer-performance-and-acceptance.md).
- **1. Publish the data a browser can open** — [V01](3d-infrastructure/01-browser-assets-and-publication.md).
- **2. Carry the same selection and layer meaning into 3D** — [V04](3d-infrastructure/04-layer-catalog-and-atlas-handoff.md), [V07](3d-infrastructure/07-frozen-environment-context-handoff.md).
- **3. Make disconnected mode work from a clean browser** — [V05](3d-infrastructure/05-local-basemaps-and-offline-packages.md).
- **4. Read from admitted evidence, with deliberate refresh** — [V06](3d-infrastructure/06-context-overlays-and-freshness.md), [V07](3d-infrastructure/07-frozen-environment-context-handoff.md).
- **5. Measure viewer limits and expose worker health** — [V08](3d-infrastructure/08-viewer-performance-and-acceptance.md), [F08](environment-foundation/08-operations-metrics-and-commissioning.md).
- **Visual land cover and a terrain type comparison** — [D01](environment-data/01-land-cover.md), [D02](environment-data/02-bare-earth-terrain.md).
- **Imagery footprints, then a qualified radar preview** — [V03](3d-infrastructure/03-imagery-and-vector-display.md), [D03](environment-data/03-radar-imagery.md).
- **Regional event coverage and historical weather inputs** — [D04](environment-data/04-natural-hazards-and-official-events.md), [D05](environment-data/05-weather-fields.md).
- **Coastal coverage at the scale the data supports** — [D06](environment-data/06-coastal-and-bathymetric-data.md).
- **Package evidence for Jeddah, Al Ain and Avignon, stored ZIP entries and byte accounting** — [V01](3d-infrastructure/01-browser-assets-and-publication.md), [V08](3d-infrastructure/08-viewer-performance-and-acceptance.md).
- **Code and data rights, AGPL platform, MIT thin clients and hosted service terms** — [F01](environment-foundation/01-source-admission-and-lineage.md), [F10](environment-foundation/10-contracts-ci-and-upgrade-compatibility.md).

## Reuse decisions

F01/F02/F05 extend the existing catalog, profiles and storage. F04 closes the worker transport gap using current lease transactions. V01–V04 connect that foundation to geographic inspection. V05/V06 make disconnected operation and source health usable. V07 preserves exact data for later consumers. F09 and V08 own their separate acceptance evidence.

D01–D08 make every proposed data family an independently qualified capability. Shared pipeline changes stay with F02, delivery stays with V01–V03, and permissions stay with F01. These dependencies avoid duplicating storage or admission systems across datasets.

World Monitor is a pinned architectural reference. Proposed implementation uses the current EMAD stack. Any actual source-code import needs its specific license recorded, and any hosted-data integration needs its effective service/output terms recorded. The [project license explanation](https://www.worldmonitor.app/docs/license) distinguishes the platform, clients and service outputs.
