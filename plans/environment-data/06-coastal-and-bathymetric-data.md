# D06 — Coastal geography and bathymetric context

[Plan index](../README.md)

**Status** The Jubail regional pilot passed disposable staging, browser inspection and offline rebuild. Shared installation rollout and independent release are pending. [Evidence and limits](../../docs/implementation/COASTAL_MOVING_AND_READINESS.md#d06--saudi-gulf-coastal-context)

**Implementation owner** Marine geospatial data engineering.

**Dependencies** [F02 — Geographic profiles, coordinates and quality](../environment-foundation/02-geographic-profiles-and-quality.md), [V02 — Terrain mesh generation and coordinate rendering](../3d-infrastructure/02-terrain-and-coordinate-rendering.md)

## Outcome and activation

A coastal environment can display an explicitly qualified depth layer alongside land surface and coastline. Jeddah currently retains water/coast features and declares bathymetry unsupported. Activate this plan for a defined regional seabed-viewing need.

## Features and contracts

Evaluate a bounded GEBCO_2026 subset and its matching Type Identifier Grid. The nominal spacing is 15 arc-seconds. Preserve signed elevation/depth convention, registration, native spacing, source-type cells, data gaps and the provider’s mixed-source vertical-datum caveats.

Keep coastline, inland water, ground/surface elevation and seabed depth as distinct semantic layers. Declare any shoreline joining or vertical transformation as a display derivation. A port-scale accuracy requirement needs a separately sourced and qualified survey.

## Implementation sequence

1. Record the intended view scale, AOI and source terms.
2. Retain the exact elevation and source-type subsets from the same edition.
3. Extend profile, processor and verifier for bathymetry and categorical source-type masks.
4. Validate sign, cell registration, datum and geographic overlap with the land inputs.
5. Produce a regional display derivative with visible seam/coverage limitations.
6. Review whether the requested scale can be supported by these actual inputs.

## Acceptance

- Source type remains inspectable for sampled depth cells.
- Elevation/depth sign and units are consistent across source, manifest and viewer.
- Coastline overlap reveals gaps and datum disagreement.
- Subsetting preserves all selected valid samples and nodata.
- The viewer exposes native spacing and qualified uses.
- A finer-scale consumer request yields an explicit additional-source requirement.
- Export/rebuild retains both elevation and source-type parents.

## Rollout and recovery

Add the capability through a new environment revision. Keep existing water geometry available independently. Withdraw an invalid depth derivative and rebuild from retained source after correction.

## Deliverable evidence

Deliver the bounded paired subsets, source-type checks, shoreline comparison and scale-suitability decision. GEBCO’s stated use limitations travel with its representation.

## Implementation and source references

- [scripts/data/environment_library.py](../../scripts/data/environment_library.py)
- [internal/geoenv/catalog.go](../../internal/geoenv/catalog.go)
- [internal/reference/map.go](../../internal/reference/map.go)
- [GEBCO_2026 grid](https://www.gebco.net/data-products/gridded-bathymetry-data/gebco2026-grid)
