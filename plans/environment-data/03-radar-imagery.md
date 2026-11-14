# D03 — Radar imagery acquisition and display

[Plan index](../README.md)

**Status** Riyadh radar pilot implemented, 13 November 2026. Independent release review and separate layover/shadow coverage remain open. [Pilot evidence](../../docs/implementation/RIYADH_OBSERVATION_PILOT.md#d03-sentinel-1-radar).

**Implementation owner** Remote-sensing data engineering.

**Dependencies** [F01 — Source admission, lineage and retention](../environment-foundation/01-source-admission-and-lineage.md), [F02 — Geographic profiles, coordinates and quality](../environment-foundation/02-geographic-profiles-and-quality.md), [V03 — Imagery and vector display preparation](../3d-infrastructure/03-imagery-and-vector-display.md)

## Outcome and activation

A selected environment can display qualified radar observations for a declared date and footprint. Activate this plan when the receiving app needs radar context or optical coverage is insufficient. The existing imagery pipeline handles optical scenes, while World Monitor supplies a useful bounded STAC discovery pattern.

## Features and contracts

Add a provider-specific adapter and a distinct radar representation. Retain exact product ID, acquisition interval, orbit, mode, polarization, processing level, native measurement units and source assets. Record calibration, terrain correction, masks and the processor recipe used for any derivative.

A discovery result represents scene metadata. Geographic output becomes available only after acquiring, validating and retaining the required pixels and processing dependencies.

## Implementation sequence

1. Choose one real scene covering a supported AOI and validate the provider collection and usable assets.
2. Admit the source and add bounded paging, rate budgets and acquisition receipts.
3. Qualify a documented processing recipe with pinned dependencies and units.
4. Generate a radar preview with explicit display scaling and available layover/shadow/nodata information.
5. Publish through V01/V03 and expose acquisition and processing metadata in the inspector.
6. Rebuild the derivative offline from retained inputs.

## Acceptance

- Scene metadata and retained pixels refer to the same product revision.
- Unsupported mode or polarization fails preflight.
- A provider error remains an acquisition failure and never becomes complete empty coverage.
- Output georeferencing and mask behavior pass independent checks.
- Radiometric values and display scaling are documented and distinguishable.
- Reprocessing reproduces the declared content identity within the pinned recipe.
- Optical and radar products remain separately selectable and dated.

## Rollout and recovery

Enable the adapter for the bounded pilot area and quota. Preserve raw retained input only within its admission. A recipe correction creates a new derived edition. Keep failed processing diagnostic evidence.

## Deliverable evidence

Deliver discovery/acquisition receipts, product and toolchain hashes, processing validation and one reviewed preview. If no suitable authorized product exists, record the unmet capability without substituting a thumbnail.

## Implementation and source references

- [internal/publicingest/earthsearch.go](../../internal/publicingest/earthsearch.go)
- [internal/publicingest/stac.go](../../internal/publicingest/stac.go)
- [internal/imagery/preflight.go](../../internal/imagery/preflight.go)
- [internal/imagery/worker.go](../../internal/imagery/worker.go)
- [World Monitor server/worldmonitor/imagery/v1/search-imagery.ts](https://github.com/koala73/worldmonitor/blob/1eb90a98bda21fc8c4f4ce070c3ef7e25b3873cc/server/worldmonitor/imagery/v1/search-imagery.ts)
- [Copernicus Sentinel-1](https://documentation.dataspace.copernicus.eu/Data/SentinelMissions/Sentinel1.html)
